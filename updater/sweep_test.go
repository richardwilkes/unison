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
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
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
// moment the replacement has been shown to run. The version running shows the swap happened even when the helper never
// recorded it, because its save failed or it was killed first.
func TestSettleRemovesTheBackupOnlyAfterASuccessfulStart(t *testing.T) {
	for _, one := range []struct {
		name   string
		status status
	}{
		{name: "the helper recorded the swap", status: statusApplied},
		{name: "the helper did not record the swap", status: statusStaged},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			u := newTestUpdater(t)
			s := installation(t, u, one.status, "target", "backup", "workdir")

			out := u.settle(s)

			c.True(exists(s.Target))
			c.False(exists(s.Backup), "the previous version is removed once the new one has started")
			c.False(exists(s.WorkDir))
			c.NotNil(out)
			if out != nil {
				c.True(out.Applied)
				c.Equal(reasonNone, out.Reason)
			}
		})
	}
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

// An update prepared but never applied -- the helper never ran, or died before recording anything -- is cleaned up
// without touching the installation, but only once neither the application that prepared it nor the helper it started
// can still act on it: until then, that copy may yet quit and have its helper apply the update, or the helper may still
// be waiting to. Once the application has recorded its helper, which it does on its way out, only the helper can act on
// it. This process having the identifier that application had means only that the identifier was reused. Either is told
// from a process given its identifier later by the start the state records, which a change to the clock since cannot
// disturb. A state that records none counts any process with the application's identifier, and one with the helper's
// only until helperRunLimit has passed.
func TestReportAndSweepDiscardsOnlyAnAbandonedStaging(t *testing.T) {
	parentStart := processStartID(os.Getppid())
	if parentStart == "" {
		t.Fatal("unable to establish when the parent process started")
	}
	for _, one := range []struct {
		name        string
		parentStart string
		helperStart string
		parentPID   int
		helperPID   int
		age         time.Duration
		wantKept    bool
	}{
		{name: "the application that prepared it has exited", parentPID: exitedPID(t)},
		{name: "no application was recorded"},
		{name: "this process has the application's identifier", parentPID: os.Getpid()},
		{
			name:        "the application that prepared it is still running",
			parentPID:   os.Getppid(),
			parentStart: parentStart,
			wantKept:    true,
		},
		{
			// The state's time on the clock is earlier than the application's start, as when the clock was stepped.
			name:        "the application that prepared it is still running and the clock has been stepped",
			parentPID:   os.Getppid(),
			parentStart: parentStart,
			age:         10 * 365 * 24 * time.Hour,
			wantKept:    true,
		},
		{
			name:      "the application that prepared it is still running and its start was not recorded",
			parentPID: os.Getppid(),
			wantKept:  true,
		},
		{
			name:        "a process that started later has the application's identifier",
			parentPID:   os.Getppid(),
			parentStart: "another process",
		},
		{name: "its helper has exited", parentPID: exitedPID(t), helperPID: exitedPID(t)},
		{
			name:      "its helper has exited and another process has the application's identifier",
			parentPID: os.Getppid(),
			helperPID: exitedPID(t),
		},
		{
			name:        "its helper is still running and the clock has been stepped",
			parentPID:   exitedPID(t),
			helperPID:   os.Getppid(),
			helperStart: parentStart,
			age:         10 * 365 * 24 * time.Hour,
			wantKept:    true,
		},
		{
			name:        "a process that started later has its helper's identifier",
			parentPID:   exitedPID(t),
			helperPID:   os.Getppid(),
			helperStart: "another process",
		},
		{
			name:      "its helper is still running and its start was not recorded",
			parentPID: exitedPID(t),
			helperPID: os.Getppid(),
			wantKept:  true,
		},
		{
			name:      "another process has its helper's identifier and its start was not recorded",
			parentPID: exitedPID(t),
			helperPID: os.Getppid(),
			age:       helperRunLimit + time.Minute,
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			u := newTestUpdater(t)
			u.stateFile = filepath.Join(t.TempDir(), stateName)
			s := installation(t, u, statusStaged, "target", "workdir")
			s.ToVersion = testNextVersion
			s.ParentPID = one.parentPID
			s.ParentStart = one.parentStart
			s.HelperPID = one.helperPID
			s.HelperStart = one.helperStart
			c.NoError(s.save(u.stateFile))
			if one.age != 0 {
				written := time.Now().Add(-one.age)
				c.NoError(os.Chtimes(u.stateFile, written, written))
			}

			out := u.reportAndSweep()

			c.Equal("new", read(t, s.Target), "the installation must be untouched")
			c.Equal(one.wantKept, exists(s.WorkDir), "the staging directory")
			c.Equal(one.wantKept, exists(u.stateFile), "the record of the prepared update")
			if one.wantKept {
				c.Nil(out)
			} else {
				c.NotNil(out)
				if out != nil {
					c.Equal(reasonAbandoned, out.Reason)
				}
			}
		})
	}
}

// The helper's log is in the staging directory, which settling removes, so when the update did not simply succeed, what
// the log holds is carried into the application log first. An update in place that the helper never recorded as applied
// did not simply succeed: the log is the only record of why.
func TestReportAndSweepKeepsTheHelperLog(t *testing.T) {
	for _, one := range []struct {
		name       string
		status     status
		reason     reason
		wantLogged bool
	}{
		{name: "the swap failed", status: statusFailed, reason: reasonSwapFailed, wantLogged: true},
		{name: "the swap was never recorded", status: statusStaged, wantLogged: true},
		{name: "the swap was recorded", status: statusApplied},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			u := newTestUpdater(t)
			u.stateFile = filepath.Join(t.TempDir(), stateName)
			s := installation(t, u, one.status, "target", "workdir")
			s.Reason = one.reason
			s.LogPath = filepath.Join(s.WorkDir, logName)
			write(t, s.LogPath, "helper-diagnostics")
			c.NoError(s.save(u.stateFile))
			var logged bytes.Buffer
			prevLogger, prevOutput, prevFlags := slog.Default(), log.Writer(), log.Flags()
			slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
			t.Cleanup(func() {
				slog.SetDefault(prevLogger)
				log.SetOutput(prevOutput)
				log.SetFlags(prevFlags)
			})

			u.reportAndSweep()

			c.False(exists(s.WorkDir), "the staging directory")
			if one.wantLogged {
				c.Contains(logged.String(), "helper-diagnostics", "the helper's log must reach the application log")
			} else {
				c.NotContains(logged.String(), "helper-diagnostics", "a recorded success must stay quiet")
			}
		})
	}
}

// Both halves of the sweep: it clears what an interrupted update leaves behind, and it leaves alone anything recent
// enough to belong to an update being applied right now by a helper this process knows nothing about. It must also not
// touch the user's own files, nor a displaced copy of some other application sharing the directory. The shorter bound
// used just before a new update is prepared (see abandonedStagingAge) clears a staging directory the startup sweep
// leaves alone, while still sparing one a helper could be using, and any backup the startup sweep would spare.
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

	// Moving an installation aside keeps the modification time of what it holds, so a backup made just now can look a
	// month old.
	displaced := (&target{Path: filepath.Join(dir, testBundle), Parent: dir}).freeBackupPath()
	write(t, displaced, "just moved aside")
	monthOld := time.Now().Add(-30 * 24 * time.Hour)
	c.NoError(os.Chtimes(displaced, monthOld, monthOld))

	// Kept after a version mismatch, where it is the only other copy of the application.
	keptAt := strconv.FormatInt(time.Now().Add(-2*abandonedStagingAge).UnixNano(), 36)
	kept := (&target{Path: filepath.Join(dir, testBundle), Parent: dir}).backupPath(keptAt)
	write(t, kept, "kept")
	c.NoError(os.Chtimes(kept, monthOld, monthOld))

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
	c.True(exists(displaced), "a backup must be aged from when it was moved aside, not from what it holds")
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
	c.True(exists(displaced), "a backup moved aside just now must be left alone")
	c.True(exists(kept), "a backup younger than staleAge must survive the sweep before a new attempt")
}

// The sweep must be harmless when the installation directory cannot be determined or no longer exists. It runs on every
// launch, so it must never be able to fail the startup path, nor take an unknown directory for the working directory.
func TestSweepStraysToleratesMissingDirectories(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	wd := t.TempDir()
	t.Chdir(wd)
	old := time.Now().Add(-staleAge - time.Hour)
	leftovers := []string{
		filepath.Join(wd, u.workDirPrefix()+"aaa"),
		filepath.Join(wd, "."+testCmd+backupSuffix+"111"),
	}
	for _, path := range leftovers {
		write(t, path, "leftover")
		c.NoError(os.Chtimes(path, old, old))
	}

	u.sweepStrays(nil, staleAge)
	u.sweepStrays([]sweepDir{{}, {targetBase: testCmd}}, staleAge)
	u.sweepStrays([]sweepDir{{dir: filepath.Join(t.TempDir(), "gone"), targetBase: testCmd}}, staleAge)

	for _, path := range leftovers {
		c.True(exists(path), "a sweep with no directory removed something from the working directory: %s", path)
	}
}
