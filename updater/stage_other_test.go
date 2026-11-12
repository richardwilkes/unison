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
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// What the helper needs, whatever shape the copy takes: the path handed back is a copy of the running program, it lives
// in the staging directory rather than in the installation about to be replaced, and it can be started by that path,
// which once failed on Windows for want of an ".exe" extension (see helperFileName). Staging it must also touch nothing
// outside the staging directory, so an abort here can cost the user nothing.
func TestStageHelperProducesAnExecutableCopy(t *testing.T) {
	c := check.New(t)
	installDir := t.TempDir()
	exePath := filepath.Join(installDir, testCmd)
	write(t, exePath, "the application")
	c.NoError(os.Chmod(exePath, executableModePerm)) //nolint:gosec // It stands in for an executable
	tgt, err := resolveTarget(exePath, runtime.GOOS)
	c.NoError(err)
	before := listing(t, installDir)
	workDir := t.TempDir()

	helper, err := stageHelper(t.Context(), &tgt, workDir)
	c.NoError(err)

	c.Equal(before, listing(t, installDir), "staging the helper must leave the installation alone")
	c.True(strings.HasPrefix(helper, workDir+string(filepath.Separator)),
		"the helper must be staged rather than run from the installation being replaced")
	fi, err := os.Stat(helper)
	c.NoError(err)
	c.True(fi.Mode().IsRegular())
	c.Equal("the application", read(t, helper))
	if runtime.GOOS != xos.WindowsOS {
		c.True(fi.Mode().Perm()&0o100 != 0, "the helper must be executable")
	}
	// exec.LookPath applies the same rule os/exec does when starting the helper, without needing a real executable
	// image.
	_, err = exec.LookPath(helper)
	c.NoError(err, "the staged helper must be startable by the path stageHelper hands back")
}
