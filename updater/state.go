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
	"encoding/json/v2"
	"io"
	"os"
	"path/filepath"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// stateSchema is the version of the state layout. Three different builds of the application read this file -- the one
// that staged the update, the helper that applies it, and the newly installed one that reports on it -- so the layout
// is versioned and every reader tolerates fields it does not recognize. A file whose schema is not understood is
// discarded rather than acted on.
const stateSchema = 1

// status is where an update has got to.
type status string

const (
	// statusStaged means the replacement has been downloaded and verified but not yet swapped in.
	statusStaged status = "staged"
	// statusApplied means the swap succeeded.
	statusApplied status = "applied"
	// statusFailed means the update was abandoned; the reason says why.
	statusFailed status = "failed"
)

// reason identifies why an update could not be applied. These are stable identifiers rather than messages: the file
// they travel in is written by one build of the application and read by another, possibly running in a different
// language, so translation has to happen at the point of display.
type reason string

const (
	// reasonNone means nothing went wrong.
	reasonNone reason = ""
	// reasonPredecessorRunning means the application that staged the update never exited, or something else, normally a
	// copy the user started, held its handoff port until the wait gave up, so nothing was touched.
	reasonPredecessorRunning reason = "predecessor-still-running"
	// reasonSwapFailed means the installation could not be replaced. The original is still in place.
	reasonSwapFailed reason = "swap-failed"
	// reasonRelaunchFailed means the new version was installed but could not be started again.
	reasonRelaunchFailed reason = "relaunch-failed"
	// reasonVersionMismatch means the swap reported success but the running version is not the one that was installed.
	reasonVersionMismatch reason = "version-mismatch"
	// reasonAbandoned means an update was staged but never applied, and the staging has now been discarded.
	reasonAbandoned reason = "abandoned"
)

// state is the record shared between the application staging an update, the helper applying it, and the application
// that comes back afterwards. It is deliberately flat and made only of scalars, so a build that predates or postdates
// any given field can still read the rest of it. The JSON names are a contract with every build that has shipped it.
type state struct {
	Status      status `json:"status"`
	Reason      reason `json:"reason,omitzero"`
	Detail      string `json:"detail,omitzero"`
	FromVersion string `json:"from_version,omitzero"`
	ToVersion   string `json:"to_version,omitzero"`
	Target      string `json:"target"`
	Exec        string `json:"exec,omitzero"`
	Payload     string `json:"payload,omitzero"`
	Backup      string `json:"backup,omitzero"`
	WorkDir     string `json:"work_dir,omitzero"`
	Helper      string `json:"helper,omitzero"`
	LogPath     string `json:"log_path,omitzero"`
	ParentStart string `json:"parent_start,omitzero"`
	Schema      int    `json:"schema"`
	HandoffPort int    `json:"handoff_port,omitzero"`
	ParentPID   int    `json:"parent_pid,omitzero"`
	HelperPID   int    `json:"helper_pid,omitzero"`
	Bundle      bool   `json:"bundle,omitzero"`
}

// statePath returns where the update state lives. It sits in the application data directory rather than in the staging
// directory so that it survives the staging being swept, which is what lets a failure still be reported afterwards. It
// is resolved on each use, since the application may set xos.AppIdentifier after this package is initialized.
func (u *Updater) statePath() string {
	if u.stateFile != "" {
		return u.stateFile
	}
	return filepath.Join(xos.AppDataDir(true), stateName)
}

// save writes the state atomically. A torn file would be read by a different build than the one that wrote it, with no
// way to tell a truncated file from a stale one, so the write must not be observable half-done.
func (s *state) save(path string) error {
	s.Schema = stateSchema
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return errs.Wrap(err)
	}
	return xos.WriteSafeFile(path, func(w io.Writer) error {
		if err := json.MarshalWrite(w, s); err != nil {
			return errs.Wrap(err)
		}
		return nil
	})
}

// loadState reads the update state. A missing file returns a nil state and no error, since not having an update in
// progress is the normal case rather than a failure.
func loadState(path string) (*state, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errs.Wrap(err)
	}
	defer xio.CloseIgnoringErrors(f)
	var s state
	if err = json.UnmarshalRead(f, &s); err != nil {
		return nil, errs.NewWithCause("unable to read the update state", err)
	}
	if s.Schema != stateSchema {
		return nil, errs.Newf("update state schema %d is not understood", s.Schema)
	}
	return &s, nil
}

// targetInfo reconstructs the installation the state describes as being replaced. Exec is the recorded executable path
// when the state carries one; a state written by a build that did not record it falls back to the target itself for a
// bare executable and to the conventional location inside the bundle otherwise, so that the executable is never
// mistaken for the bundle directory that holds it.
func (u *Updater) targetInfo(s *state) target {
	k := kindExecutable
	exe := s.Exec
	if s.Bundle {
		k = kindBundle
		if exe == "" {
			exe = filepath.Join(s.Target, "Contents", "MacOS", u.cfg.CmdName)
		}
	} else if exe == "" {
		exe = s.Target
	}
	return target{
		Path:   s.Target,
		Parent: filepath.Dir(s.Target),
		Exec:   exe,
		Kind:   k,
	}
}
