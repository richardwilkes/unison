// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"log/slog"
	"os"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// FinishFlag is the command line flag that puts a copy of the application into helper mode, followed by the path of the
// update's state file. It is not meant for anyone to type; the helper is the same program, started again with something
// to do other than open a window.
const FinishFlag = "finish-update"

// spawnFinisher starts the helper that applies a staged update once this process has exited, and records its process
// and start in the state, so that a copy of the application started while the helper is still at work does not take
// the update for abandoned, nor a process later given its identifier for the helper. A helper it cannot record is
// stopped again, and an error returned. It is called from the quitting path, so it must do nothing more; anything slow
// here delays the user's quit for no reason.
func (u *Updater) spawnFinisher(s *state, statePath string) error {
	if s == nil || s.Helper == "" {
		return errs.New("there is no prepared update to apply")
	}
	slog.Info("starting the update helper", "helper", s.Helper, "target", s.Target)
	pid, err := spawnDetached(s.Helper, []string{"--" + FinishFlag, statePath}, s.LogPath, os.TempDir())
	if err != nil {
		return err
	}
	// The helper writes the state only once this process has exited or waitTimeout has passed, so this save comes
	// first. With a handoff port, that relies on the application holding the port until it exits, as Config.HandoffPort
	// requires.
	s.HelperPID = pid
	s.HelperStart = processStartID(pid)
	if err = s.save(statePath); err != nil {
		// Unrecorded, the helper could have its update taken for abandoned and discarded under it by a copy started
		// during its wait, so it is stopped. It is still waiting for this process to exit, so it has touched nothing.
		if proc, findErr := os.FindProcess(pid); findErr == nil {
			if killErr := proc.Kill(); killErr != nil {
				errs.Log(errs.NewWithCause("unable to stop the update helper", killErr), "pid", pid)
			}
		}
		return errs.NewWithCause("unable to record the update helper", err)
	}
	return nil
}

// FinishIfRequested runs Finish and exits if args, normally os.Args, ask for it with FinishFlag. The flag may be given
// with one dash or two, and its path either as the next argument or after an "=". Otherwise it returns, and the
// application starts as usual. Call it at the top of main, before anything that may open a window or claim the
// single-instance handoff.
func (u *Updater) FinishIfRequested(args []string) {
	statePath, ok := finishArg(args)
	if !ok {
		return
	}
	if err := u.Finish(statePath); err != nil {
		errs.Log(err)
		xos.Exit(1)
	}
	xos.Exit(0)
}

// finishArg returns the state file path args give FinishFlag, and whether they give one at all.
func finishArg(args []string) (string, bool) {
	for i, arg := range args {
		name, ok := strings.CutPrefix(arg, "-")
		if !ok {
			continue
		}
		name = strings.TrimPrefix(name, "-")
		if name == FinishFlag {
			if i+1 < len(args) && args[i+1] != "" {
				return args[i+1], true
			}
			return "", false
		}
		if path, found := strings.CutPrefix(name, FinishFlag+"="); found {
			return path, path != ""
		}
	}
	return "", false
}

// Finish applies a staged update. It runs in the helper process, after the application that staged the update has
// exited, and is the only part of an update that touches the installation. Most applications reach it through
// FinishIfRequested; one that parses its own command line calls it with the path given to FinishFlag.
//
// There is no user interface here. Everything it learns is written back into the state file, which the newly installed
// application reads on its next start; that is how a failure at this point ever reaches the user.
//
// It deletes nothing. The previous version stays on disk until the replacement has demonstrably started, so a version
// that turns out not to run is still sitting beside one that does.
func (u *Updater) Finish(statePath string) error {
	s, err := loadState(statePath)
	if err != nil {
		return err
	}
	if s == nil {
		return errs.New("there is no prepared update to apply")
	}
	if s.Status != statusStaged {
		return errs.Newf("the prepared update is %s rather than staged", s.Status)
	}

	if err = waitForPredecessor(s.ParentPID, s.HandoffPort, u.cfg.AppName); err != nil {
		// Nothing has been touched, so the installation is exactly as the user left it.
		return abandon(s, statePath, reasonPredecessorRunning, err)
	}

	t := u.targetInfo(s)
	if err = swap(s.Target, s.Payload, s.Backup); err != nil {
		errs.Log(err)
		// Recorded first, as on success, so the restarted application finds the failure rather than a staged update.
		err = abandon(s, statePath, reasonSwapFailed, err)
		if relaunchErr := u.relaunchFn(&t, s.LogPath); relaunchErr != nil {
			slog.Error("unable to restart after an update that could not be applied", "error", relaunchErr)
		}
		return err
	}

	s.Status = statusApplied
	s.Reason = reasonNone
	s.Detail = ""
	if err = s.save(statePath); err != nil {
		// The update is in place; only the record of it is not. Start the new version anyway; it will simply have
		// nothing to report.
		slog.Error("unable to record that the update was applied", "error", err)
	}

	if err = u.relaunchFn(&t, s.LogPath); err != nil {
		// The new version is installed and verified; it just did not start. Rolling back would replace something known
		// to work with something older for no gain, so record it and leave the user to open it themselves.
		slog.Error("unable to start the updated application", "error", err)
		s.Reason = reasonRelaunchFailed
		s.Detail = err.Error()
		if saveErr := s.save(statePath); saveErr != nil {
			slog.Error("unable to record that the restart failed", "error", saveErr)
		}
		return err
	}
	slog.Info("update applied and restarted", "version", s.ToVersion)
	return nil
}

// abandon records why an update was not applied and returns the original failure. The state file carries this to the
// next run of the application, since there is nobody to tell right now.
func abandon(s *state, statePath string, r reason, cause error) error {
	s.Status = statusFailed
	s.Reason = r
	s.Detail = cause.Error()
	if err := s.save(statePath); err != nil {
		slog.Error("unable to record why the update was not applied", "error", err)
	}
	return cause
}
