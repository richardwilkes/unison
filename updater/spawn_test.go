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
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// reportWritesEnv, when set, turns a run of this test binary into a process that writes to its standard output and
// error, and records at the path it names what became of each write, for TestSpawnDetachedDiscardsOutputWithoutALog.
const reportWritesEnv = "UPDATER_TEST_REPORT_WRITES"

// Without a log, the child's output goes to the null device, which accepts it: a write that fails can make a program
// give up, or report a failure that never happened.
func TestSpawnDetachedDiscardsOutputWithoutALog(t *testing.T) {
	if report := os.Getenv(reportWritesEnv); report != "" {
		_, outErr := os.Stdout.WriteString("output\n")
		_, errErr := os.Stderr.WriteString("output\n")
		tmp := report + ".tmp"
		//nolint:gosec // The path is one the test running this chose
		if err := os.WriteFile(tmp, fmt.Appendf(nil, "%v\n%v", outErr, errErr), 0o600); err == nil {
			_ = os.Rename(tmp, report) //nolint:errcheck,gosec // The test waiting for the report fails without it
		}
		return
	}
	c := check.New(t)
	report := filepath.Join(t.TempDir(), "report")
	t.Setenv(reportWritesEnv, report)
	exe, err := os.Executable()
	c.NoError(err)
	c.NoError(spawnDetached(exe, []string{"-test.run", "^TestSpawnDetachedDiscardsOutputWithoutALog$"}, "",
		os.TempDir()))
	for deadline := time.Now().Add(10 * time.Second); !exists(report); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the child did not report")
		}
	}
	c.Equal("<nil>\n<nil>", read(t, report), "both writes must succeed")
}
