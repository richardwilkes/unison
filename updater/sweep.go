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
	"path/filepath"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
)

const (
	// staleAge is how old a leftover has to be before the startup sweep will remove it on sight. Anything younger might
	// belong to an update being applied right now by a helper this instance knows nothing about.
	staleAge = 7 * 24 * time.Hour
	// abandonedStagingAge is the shorter bound used just before a new update is prepared. By then any helper still
	// working would have had to survive since before the application started, and the wait it performs gives up long
	// before this, so anything older is from an attempt that did not clean up after itself. Without it, each failed
	// attempt would leave a copy of the application in the user's applications folder for a week.
	abandonedStagingAge = time.Hour
)

// removeAttempts and removeDelay bound the retries when clearing leftovers. On Windows the helper may still be exiting
// as the newly installed application starts, and its image cannot be deleted until it has.
const (
	removeAttempts = 5
	removeDelay    = 500 * time.Millisecond
)

// outcome reports what became of an update that was in progress when the application last exited. A nil outcome means
// there was nothing to report.
type outcome struct {
	Reason    reason
	Detail    string
	ToVersion string
	Applied   bool
}

// sweepDir is a directory an update may have left things in, along with the base name of the installation in it whose
// displaced copies may be removed.
type sweepDir struct {
	dir        string
	targetBase string
}

// reportAndSweep settles up after an update: it repairs an installation left half-swapped, reports what happened, and
// clears away what the helper deliberately left behind.
//
// The helper never deletes anything; it moves the old installation aside, puts the new one in place, and starts it.
// Removing the old copy happens here, which means it only happens once the replacement has demonstrably started, so a
// version that cannot run is still on disk next to one that can.
//
// This must run only in the instance that owns the installation, after any single-instance handoff has established that
// this process is it. Failures are logged rather than surfaced: the application has already started successfully by
// this point, and a leftover directory is not worth interrupting the user for.
func (u *Updater) reportAndSweep() *outcome {
	statePath := u.statePath()
	s, err := loadState(statePath)
	if err != nil {
		// An unreadable or unrecognized state file cannot be acted on, and keeping it would mean reporting the same
		// confusion on every launch from here on.
		errs.Log(errs.NewWithCause("discarding an unusable update state", err), "path", statePath)
		removeIgnoringErrors(statePath)
		u.sweepStrays(currentSweepDirs(), staleAge)
		return nil
	}
	if s == nil {
		u.sweepStrays(currentSweepDirs(), staleAge)
		return nil
	}
	out := u.settle(s)
	removeIgnoringErrors(statePath)
	dirs := currentSweepDirs()
	if s.Target != "" {
		dirs = append(dirs, sweepDir{dir: filepath.Dir(s.Target), targetBase: filepath.Base(s.Target)})
	}
	u.sweepStrays(dirs, staleAge)
	return out
}

// settle repairs and interprets a state left behind by a previous run, removing whatever is safe to remove.
func (u *Updater) settle(s *state) *outcome {
	// Repair first. Reaching here means the helper died between moving the installation aside and putting the
	// replacement in its place -- a window of microseconds its own rollback normally covers, so this is a net rather
	// than an expected path.
	if s.Backup != "" && s.Target != "" && !exists(s.Target) && exists(s.Backup) {
		if err := os.Rename(s.Backup, s.Target); err != nil {
			errs.Log(errs.NewWithCause("unable to restore the previous version", err), "backup", s.Backup,
				"target", s.Target)
		} else {
			slog.Warn("restored the previous version after an interrupted update", "target", s.Target)
		}
		removeWithRetry(s.WorkDir)
		return &outcome{Reason: reasonSwapFailed, Detail: s.Detail, ToVersion: s.ToVersion}
	}

	switch s.Status {
	case statusApplied:
		if s.ToVersion != "" && s.ToVersion != u.cfg.Version {
			// The swap claimed success, yet this is not the version it installed, so keep the previous version rather
			// than deleting the only other copy on the machine.
			errs.Log(errs.New("the installed version is not the one the update reported installing"),
				"expected", s.ToVersion, "actual", u.cfg.Version, "backup", s.Backup)
			removeWithRetry(s.WorkDir)
			return &outcome{Reason: reasonVersionMismatch, ToVersion: s.ToVersion}
		}
		slog.Info("update applied", "version", s.ToVersion, "from", s.FromVersion)
		removeWithRetry(s.Backup)
		removeWithRetry(s.WorkDir)
		if s.Reason != reasonNone {
			// Installed, but something afterwards went wrong -- the relaunch, most likely, which is why we are here
			// rather than having been started by it.
			return &outcome{Applied: true, Reason: s.Reason, Detail: s.Detail, ToVersion: s.ToVersion}
		}
		return &outcome{Applied: true, ToVersion: s.ToVersion}
	case statusFailed:
		errs.Log(errs.Newf("the update was not applied: %s", s.Reason), "detail", s.Detail)
		removeWithRetry(s.WorkDir)
		// The installation was left untouched by a failure at this point, so any backup is a stray.
		if exists(s.Target) {
			removeWithRetry(s.Backup)
		}
		return &outcome{Reason: s.Reason, Detail: s.Detail, ToVersion: s.ToVersion}
	case statusStaged:
		// Staged but never applied: the helper never ran, or never got as far as recording a result. Nothing was
		// touched, so there is nothing to repair.
		slog.Info("discarding an update that was staged but never applied", "version", s.ToVersion)
		removeWithRetry(s.WorkDir)
		return &outcome{Reason: reasonAbandoned, ToVersion: s.ToVersion}
	default:
		errs.Log(errs.Newf("unrecognized update status %q", s.Status))
		removeWithRetry(s.WorkDir)
		return nil
	}
}

// currentSweepDirs returns where an update to this installation would have left things.
func currentSweepDirs() []sweepDir {
	t, err := currentTarget()
	if err != nil {
		errs.Log(errs.NewWithCause("unable to determine the installation directory", err))
		return nil
	}
	return []sweepDir{{dir: t.Parent, targetBase: filepath.Base(t.Path)}}
}

// sweepStrays removes staging directories and displaced installations old enough that no update in progress could still
// need them. This is the backstop for a helper killed before it could record anything, and it is what eventually clears
// a Windows image that could not be deleted while it was still running.
func (u *Updater) sweepStrays(dirs []sweepDir, olderThan time.Duration) {
	seen := make(map[sweepDir]bool, len(dirs))
	cutoff := time.Now().Add(-olderThan)
	for _, one := range dirs {
		if one.dir == "" || seen[one] {
			continue
		}
		seen[one] = true
		entries, err := os.ReadDir(one.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !u.isLeftover(entry.Name(), one.targetBase) {
				continue
			}
			fi, infoErr := entry.Info()
			if infoErr != nil || fi.ModTime().After(cutoff) {
				continue
			}
			path := filepath.Join(one.dir, entry.Name())
			slog.Info("removing a leftover from an earlier update", "path", path)
			removeWithRetry(path)
		}
	}
}

// removeWithRetry removes a path, retrying briefly. The retries are for Windows, where the helper's own image cannot be
// deleted until it has finished exiting, which may be a moment after it started the replacement.
func removeWithRetry(path string) {
	if path == "" {
		return
	}
	err := retry(removeAttempts, removeDelay, func() error {
		if err := os.RemoveAll(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	})
	if err != nil {
		errs.Log(errs.NewWithCause("unable to remove a leftover from an update", err), "path", path)
	}
}

func exists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Lstat(path)
	return err == nil
}
