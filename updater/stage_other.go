// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build !darwin

package updater

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// stagePayload unpacks the downloaded archive. The Linux and Windows distributions hold exactly one file, the
// executable, so this extracts that and refuses anything else.
func (u *Updater) stagePayload(_ context.Context, archivePath, dstPath string) error {
	wantName, err := u.payloadName(runtime.GOOS)
	if err != nil {
		return err
	}
	if runtime.GOOS == xos.WindowsOS {
		return extractSingleZip(archivePath, wantName, dstPath)
	}
	f, err := os.Open(archivePath)
	if err != nil {
		return errs.Wrap(err)
	}
	defer xio.CloseIgnoringErrors(f)
	return extractSingleTGZ(f, wantName, dstPath)
}

// helperFileName returns the name to give the staged copy of the running executable.
//
// On Windows the ".exe" extension is required, not cosmetic. Windows resolves a program against the PATHEXT list before
// starting it, and os/exec applies that rule even when handed an absolute path, so an extensionless copy is refused
// outright -- "executable file not found in %PATH%" -- despite being a perfectly good image at exactly the path named.
// Without it, staging succeeds and the helper that would finish the job cannot be started, so no Windows update can
// ever be applied.
func helperFileName() string {
	if runtime.GOOS == xos.WindowsOS {
		return helperName + ".exe"
	}
	return helperName
}

// stageHelper places a copy of the running executable in the staging directory and returns the path to run. See stage
// for why the helper is a copy at all.
func stageHelper(_ context.Context, t *target, workDir string) (string, error) {
	dst := filepath.Join(workDir, helperFileName())
	if err := xos.Copy(t.Exec, dst); err != nil {
		return "", errs.NewWithCause("unable to prepare the update helper", err)
	}
	//nolint:gosec // The helper is an executable, so it must be executable by the account that runs it
	if err := os.Chmod(dst, executableModePerm); err != nil {
		return "", errs.NewWithCause("unable to prepare the update helper", err)
	}
	return dst, nil
}

// verifyPayload has nothing to check on these platforms. The distributions carry no signature -- upack signs only on
// macOS -- so the SHA-256 recorded by GitHub, checked during the download, is the whole of the integrity guarantee
// here. Running the staged executable afterwards is what catches the rest.
func verifyPayload(_ context.Context, _, _ string) error {
	return nil
}
