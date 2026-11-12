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
	"os"
	"os/exec"
	"path/filepath"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
)

// spawnDetached starts a program that keeps running after this process exits, and returns its process identifier.
//
// Standard input comes from the null device and output is appended to logPath, if one is given. Leaving them attached
// to this process's own streams would keep pipes open that whoever started this is waiting to see closed, and would
// lose the child's output exactly when it is most wanted -- an application started from the Finder has no terminal.
func spawnDetached(exePath string, args []string, logPath, workingDir string) (pid int, err error) {
	cmd := exec.Command(exePath, args...)
	detach(cmd)
	// Never the installation directory: a working directory is an open handle, and holding one inside the directory
	// being replaced is the sort of thing that makes a rename fail on Windows.
	cmd.Dir = workingDir

	// Opened for writing too, since it also stands in for the output when there is no log.
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return 0, errs.Wrap(err)
	}
	defer xio.CloseIgnoringErrors(devNull)
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull

	if logPath != "" {
		if mkErr := os.MkdirAll(filepath.Dir(logPath), 0o750); mkErr == nil {
			if logFile, logErr := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600); logErr == nil {
				defer xio.CloseIgnoringErrors(logFile)
				cmd.Stdout = logFile
				cmd.Stderr = logFile
			}
		}
	}

	if err = cmd.Start(); err != nil {
		return 0, errs.NewWithCause("unable to start "+filepath.Base(exePath), err)
	}
	// Read before Release, which clears it outside Windows.
	pid = cmd.Process.Pid
	// The descriptors above are duplicated into the child by the time Start returns, so closing this process's copies
	// on the way out is correct. Release drops the child from this process's bookkeeping: nothing here will ever wait
	// for it, and this process is about to exit.
	if err = cmd.Process.Release(); err != nil {
		return 0, errs.Wrap(err)
	}
	return pid, nil
}
