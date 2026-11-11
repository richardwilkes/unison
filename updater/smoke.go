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
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// smokeTestTimeout bounds the trial run. Printing a version string is near-instant; anything approaching this means the
// executable is not doing what it was asked, which is itself a reason not to install it.
const smokeTestTimeout = 30 * time.Second

// verifyRuns proves the staged application actually runs on this machine before anything is swapped in.
//
// This is the one check that covers the failures nothing else can see: a checksum establishes that the right bytes
// arrived, and a signature that the right developer produced them, but neither says the result will start here. It
// catches a build for the wrong processor, a Linux build wanting a newer glibc than the system has, a macOS bundle the
// kernel refuses to execute, and a corrupted extraction whose checksum passed because the checksum itself was wrong.
// All of those succeed at swap time and fail only at launch, where there is nothing left to roll back to; catching them
// here reports such a release as unusable while the installation is still untouched.
//
// The executable is run with Config.VersionArgs, which make it print its version and exit before it creates any window,
// so this never puts anything on screen.
func (u *Updater) verifyRuns(ctx context.Context, exePath, wantVersion string) error {
	cctx, cancel := context.WithTimeout(ctx, smokeTestTimeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, exePath, u.cfg.VersionArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail != "" {
			return errs.NewWithCause("the downloaded version will not run on this system: "+detail, err)
		}
		return errs.NewWithCause("the downloaded version will not run on this system", err)
	}
	// A clean exit is the requirement. The version is checked only when there is one to check: a Windows build linked
	// as a GUI application has no console to write to, so it exits zero having printed nothing.
	reported := strings.TrimSpace(stdout.String())
	if reported == "" || wantVersion == "" {
		return nil
	}
	// A build made from a modified checkout has a "~" appended, so match on the prefix rather than for equality.
	if !strings.HasPrefix(reported, wantVersion) {
		return errs.Newf("the downloaded version reports itself as %s rather than %s", reported, wantVersion)
	}
	return nil
}
