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
	"path/filepath"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// installation builds a fake install directory holding a target, a backup and a staging directory, and returns the
// state describing them. Which of the three actually exist is up to the caller, so each way an update can be
// interrupted can be set up precisely.
func installation(t *testing.T, u *Updater, st status, present ...string) *state {
	t.Helper()
	dir := t.TempDir()
	s := &state{
		Schema:    stateSchema,
		Status:    st,
		Target:    filepath.Join(dir, testCmd),
		Backup:    filepath.Join(dir, "."+testCmd+backupSuffix+"123"),
		WorkDir:   filepath.Join(dir, u.workDirPrefix()+"abc"),
		ToVersion: u.cfg.Version,
	}
	for _, which := range present {
		switch which {
		case "target":
			write(t, s.Target, "new")
		case "backup":
			write(t, s.Backup, "old")
		case "workdir":
			if err := os.MkdirAll(s.WorkDir, 0o755); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(s.WorkDir, helperName), helperName)
		}
	}
	return s
}

// The case that matters most: the helper moved the installation aside and died before putting the replacement in its
// place, so the user has no application at all. The previous version must come back.
func TestSettleRepairsAnInterruptedSwap(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	s := installation(t, u, statusStaged, "backup", "workdir")

	out := u.settle(s)

	c.True(exists(s.Target), "the previous version must be restored")
	c.Equal("old", read(t, s.Target))
	c.False(exists(s.WorkDir), "the staging directory must be cleared")
	c.NotNil(out)
	c.Equal(reasonSwapFailed, out.Reason)
	c.False(out.Applied)
}

// The rollback window: the helper deliberately leaves the previous version on disk, and it is removed here, the first
// moment the replacement has been shown to run.
func TestSettleRemovesTheBackupOnlyAfterASuccessfulStart(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	s := installation(t, u, statusApplied, "target", "backup", "workdir")

	out := u.settle(s)

	c.True(exists(s.Target))
	c.False(exists(s.Backup), "the previous version is removed once the new one has started")
	c.False(exists(s.WorkDir))
	c.NotNil(out)
	c.True(out.Applied)
	c.Equal(reasonNone, out.Reason)
}

// When the swap claims to have installed a version other than the one now running, the previous version is kept.
// Deleting it would leave only a copy already known not to be what it claimed.
func TestSettleKeepsTheBackupOnAVersionMismatch(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	s := installation(t, u, statusApplied, "target", "backup", "workdir")
	s.ToVersion = "99.99.99"

	out := u.settle(s)

	c.True(exists(s.Backup), "the previous version must be kept when the result cannot be trusted")
	c.False(exists(s.WorkDir))
	c.NotNil(out)
	c.Equal(reasonVersionMismatch, out.Reason)
	c.False(out.Applied)
}

// A relaunch failure is still reported as applied: the new version is in place and running -- that is how we got here
// -- so it is a notice, not a failure.
func TestSettleReportsAnInstalledButUnlaunchedUpdate(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	s := installation(t, u, statusApplied, "target", "backup", "workdir")
	s.Reason = reasonRelaunchFailed

	out := u.settle(s)

	c.NotNil(out)
	c.True(out.Applied)
	c.Equal(reasonRelaunchFailed, out.Reason)
	c.False(exists(s.Backup))
}

// A failure recorded before anything was touched clears the staging and leaves the installed application exactly as it
// was.
func TestSettleOnAFailedUpdateLeavesTheInstallationAlone(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	s := installation(t, u, statusFailed, "target", "workdir")
	s.Reason = reasonPredecessorRunning

	out := u.settle(s)

	c.True(exists(s.Target))
	c.Equal("new", read(t, s.Target), "the installed application must be untouched")
	c.False(exists(s.WorkDir))
	c.NotNil(out)
	c.Equal(reasonPredecessorRunning, out.Reason)
	c.False(out.Applied)
}

// An update prepared but never applied -- the user vetoed the quit, or the helper never ran -- is cleaned up without
// touching the installation.
func TestSettleDiscardsAnAbandonedStaging(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	s := installation(t, u, statusStaged, "target", "workdir")

	out := u.settle(s)

	c.True(exists(s.Target))
	c.Equal("new", read(t, s.Target))
	c.False(exists(s.WorkDir))
	c.NotNil(out)
	c.Equal(reasonAbandoned, out.Reason)
}

// Both halves of the sweep: it clears what an interrupted update leaves behind, and it leaves alone anything recent
// enough to belong to an update being applied right now by a helper this process knows nothing about. It must also not
// touch the user's own files, nor a displaced copy of some other application sharing the directory. The shorter bound
// used just before a new update is prepared (see abandonedStagingAge) clears a staging directory the startup sweep
// leaves alone, while still sparing one a helper could be using.
func TestSweepStraysRemovesOnlyOldLeftovers(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	dir := t.TempDir()
	dirs := []sweepDir{{dir: dir, targetBase: testBundle}}
	old := time.Now().Add(-staleAge - time.Hour)

	stale := []string{
		filepath.Join(dir, u.workDirPrefix()+"aaa"),
		filepath.Join(dir, "."+testBundle+backupSuffix+"111"),
	}
	for _, path := range stale {
		write(t, path, "leftover")
		c.NoError(os.Chtimes(path, old, old))
	}

	recent := filepath.Join(dir, u.workDirPrefix()+"bbb")
	write(t, recent, "in progress")

	untouched := []string{
		filepath.Join(dir, testCmd),
		filepath.Join(dir, testBundle),
		filepath.Join(dir, testCmd+"_prefs.json"),
		filepath.Join(dir, "my-notes.old-backup"),
		filepath.Join(dir, ".Other.app"+backupSuffix+"222"),
	}
	for _, path := range untouched {
		write(t, path, "keep")
		c.NoError(os.Chtimes(path, old, old))
	}

	u.sweepStrays(dirs, staleAge)

	for _, path := range stale {
		c.False(exists(path), "a stale leftover survived: %s", path)
	}
	c.True(exists(recent), "a recent staging directory must be left for the update still using it")
	for _, path := range untouched {
		c.True(exists(path), "the sweep removed something it did not create: %s", path)
	}

	abandoned := filepath.Join(dir, u.workDirPrefix()+"old")
	write(t, abandoned, "an attempt that never finished")
	stamp := time.Now().Add(-abandonedStagingAge - time.Minute)
	c.NoError(os.Chtimes(abandoned, stamp, stamp))

	u.sweepStrays(dirs, staleAge)
	c.True(exists(abandoned), "the startup sweep must not remove something this recent")

	u.sweepStrays(dirs, abandonedStagingAge)
	c.False(exists(abandoned), "an abandoned staging directory must be cleared before a new attempt")
	c.True(exists(recent), "a staging directory a helper could still be using must be left alone")
}

// The sweep must be harmless when the installation directory cannot be determined or no longer exists. It runs on every
// launch, so it must never be able to fail the startup path.
func TestSweepStraysToleratesMissingDirectories(t *testing.T) {
	u := newTestUpdater(t)
	u.sweepStrays(nil, staleAge)
	u.sweepStrays([]sweepDir{{}}, staleAge)
	u.sweepStrays([]sweepDir{{dir: filepath.Join(t.TempDir(), "gone"), targetBase: testCmd}}, staleAge)
}
