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
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// A build that predates this package being generic left its staging directories under this name, and the sweep in a
// newer build has to recognize them.
func TestWorkDirPrefixMatchesShippedBuilds(t *testing.T) {
	check.New(t).Equal(".gcs-update-", New(&Config{CmdName: "gcs"}).workDirPrefix())
}

// Every leftover an update can create must be recognized by the startup sweep, or it stays in the user's Applications
// folder forever. The sweep must equally leave alone everything it did not create, including a displaced copy of some
// other application sharing the directory.
func TestIsLeftoverMatchesWhatIsProduced(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	for _, one := range []struct {
		exePath string
		goos    string
	}{
		{hostPath("/Applications/" + testBundle + "/Contents/MacOS/" + testCmd), xos.MacOS},
		{hostPath("/home/someone/bin/" + testCmd), xos.LinuxOS},
	} {
		tgt, err := resolveTarget(one.exePath, one.goos)
		c.NoError(err, one.exePath)
		base := filepath.Base(tgt.Path)
		for _, name := range []string{
			filepath.Base(tgt.backupPath("1700000000000000000")),
			u.workDirPrefix() + "987654321",
			u.workDirPrefix() + "probe-123",
		} {
			c.True(u.isLeftover(name, base), "%s is not recognized as a leftover of %s", name, base)
		}
		for _, name := range []string{
			base,
			".Other.app" + backupSuffix + "123",
			".other" + backupSuffix + "123",
			"my-notes.old-backup",
			"." + testCmd + "-other",
		} {
			c.False(u.isLeftover(name, base), "%s is mistaken for a leftover of %s", name, base)
		}
	}
}
