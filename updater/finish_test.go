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
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// The helper is recognized in every form the flag package accepts, so an application that also parses its own command
// line with that package sees the same thing this does. A flag with no path is not a request to finish anything, since
// there is no state to finish it from.
func TestFinishArgRecognizesTheHelperFlag(t *testing.T) {
	c := check.New(t)
	const path = "/data/update-state.json"
	for _, one := range []struct {
		name   string
		want   string
		args   []string
		wantOK bool
	}{
		{name: "one dash", args: []string{testCmd, "-finish-update", path}, want: path, wantOK: true},
		{name: "two dashes", args: []string{testCmd, "--finish-update", path}, want: path, wantOK: true},
		{name: "one dash and =", args: []string{testCmd, "-finish-update=" + path}, want: path, wantOK: true},
		{name: "two dashes and =", args: []string{testCmd, "--finish-update=" + path}, want: path, wantOK: true},
		{
			name:   "among other arguments",
			args:   []string{testCmd, "-x", "--finish-update", path, "file"},
			want:   path,
			wantOK: true,
		},
		{name: "absent", args: []string{testCmd, "-x", path}},
		{name: "no arguments"},
		{name: "no path", args: []string{testCmd, "--finish-update"}},
		{name: "an empty path", args: []string{testCmd, "--finish-update="}},
		{name: "three dashes", args: []string{testCmd, "---finish-update", path}},
		{name: "not a flag", args: []string{testCmd, "finish-update", path}},
	} {
		got, ok := finishArg(one.args)
		c.Equal(one.wantOK, ok, one.name)
		c.Equal(one.want, got, one.name)
	}
}

// The helper is recorded before the application exits, so a copy started while the helper is still at work does not
// take the update for abandoned and discard it.
func TestSpawnFinisherRecordsTheHelper(t *testing.T) {
	c := check.New(t)
	exe, err := os.Executable()
	c.NoError(err)
	statePath := filepath.Join(t.TempDir(), stateName)
	// The test binary stands in for the helper; it rejects the flag it is given and exits.
	s := &state{Status: statusStaged, Helper: exe, ParentPID: os.Getpid()}
	c.NoError(s.save(statePath))

	c.NoError(newTestUpdater(t).spawnFinisher(s, statePath))

	saved, err := loadState(statePath)
	c.NoError(err)
	c.NotNil(saved)
	if saved != nil {
		c.True(saved.HelperPID > 0 && saved.HelperPID != os.Getpid(), "the helper's process must be recorded")
	}
}

// A helper that cannot be recorded is stopped: unrecorded, a copy started during its wait would take its update for
// abandoned and discard it under it. It is still waiting for this process to exit, so stopping it is safe.
func TestSpawnFinisherStopsAHelperItCannotRecord(t *testing.T) {
	if runtime.GOOS == xos.WindowsOS {
		t.Skip("the stand-in for the helper is a shell script")
	}
	c := check.New(t)
	dir := t.TempDir()
	helper := filepath.Join(dir, helperName)
	write(t, helper, "#!/bin/sh\nexec sleep 5\n")
	c.NoError(os.Chmod(helper, executableModePerm)) //nolint:gosec // The stand-in helper has to run
	notADir := filepath.Join(dir, "file")
	write(t, notADir, "")
	s := &state{Status: statusStaged, Helper: helper, ParentPID: os.Getpid()}

	c.HasError(newTestUpdater(t).spawnFinisher(s, filepath.Join(notADir, stateName)))

	c.True(s.HelperPID > 0, "the helper must have been started")
	if s.HelperPID > 0 {
		proc, err := os.FindProcess(s.HelperPID)
		c.NoError(err)
		if err == nil {
			ps, waitErr := proc.Wait()
			c.NoError(waitErr)
			if waitErr == nil {
				c.False(ps.Exited(), "the helper must be stopped rather than left to run to its end: %s", ps)
			}
		}
	}
}

// The helper only ever applies an update the application staged. A missing state, or one that has already been applied
// or has failed, is refused without being touched, so a stray or repeated helper cannot rewrite what the newly
// installed application will report.
func TestFinishRefusesWithoutAStagedUpdate(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)

	c.HasError(u.Finish(filepath.Join(t.TempDir(), stateName)), "a missing state must be refused")

	for _, st := range []status{statusApplied, statusFailed} {
		path := filepath.Join(t.TempDir(), stateName)
		c.NoError((&state{Status: st, Reason: reasonRelaunchFailed}).save(path))
		c.HasError(u.Finish(path), "a %s state must be refused", st)
		got, err := loadState(path)
		c.NoError(err)
		c.Equal(&state{Schema: stateSchema, Status: st, Reason: reasonRelaunchFailed}, got,
			"refusing a %s state must leave it as it was", st)
	}
}

// The outcome is recorded before the application is restarted, whether the swap worked or not, so the restarted
// application reports it rather than finding an update still staged, which it would discard without a word.
func TestFinishRecordsTheOutcomeBeforeRestarting(t *testing.T) {
	for _, one := range []struct {
		name      string
		want      status
		swapFails bool
	}{
		{name: "the swap succeeds", want: statusApplied},
		{name: "the swap fails", want: statusFailed, swapFails: true},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			dir := t.TempDir()
			workDir := filepath.Join(dir, ".example-update-1")
			s := &state{
				Status:    statusStaged,
				ToVersion: testNextVersion,
				Target:    filepath.Join(dir, testCmd),
				Payload:   filepath.Join(workDir, testCmd),
				Backup:    (&target{Path: filepath.Join(dir, testCmd), Parent: dir}).freeBackupPath(),
				WorkDir:   workDir,
			}
			write(t, s.Target, "old")
			if !one.swapFails {
				write(t, s.Payload, "new")
			}
			statePath := filepath.Join(dir, stateName)
			c.NoError(s.save(statePath))
			u := newTestUpdater(t)
			var found []status
			u.relaunchFn = func(*target, string) error {
				saved, err := loadState(statePath)
				c.NoError(err)
				if saved != nil {
					found = append(found, saved.Status)
				}
				return nil
			}
			c.Equal(one.swapFails, u.Finish(statePath) != nil)
			c.Equal([]status{one.want}, found, "what the restarted application finds")
		})
	}
}

// The restarted application's output does not go to the update's log: that lives in the staging directory, which the
// application removes once it has started, and on Windows the handle it would hold for as long as it ran prevents that.
func TestFinishRestartsWithoutTheUpdateLog(t *testing.T) {
	if runtime.GOOS == xos.WindowsOS {
		t.Skip("the stand-in for the application is a shell script")
	}
	c := check.New(t)
	dir := t.TempDir()
	workDir := filepath.Join(dir, ".example-update-1")
	started := filepath.Join(dir, "started")
	s := &state{
		Status:  statusStaged,
		Target:  filepath.Join(dir, testCmd),
		Payload: filepath.Join(workDir, testCmd),
		Backup:  (&target{Path: filepath.Join(dir, testCmd), Parent: dir}).freeBackupPath(),
		WorkDir: workDir,
		LogPath: filepath.Join(workDir, logName),
	}
	write(t, s.Target, "old")
	write(t, s.Payload, "#!/bin/sh\necho output\necho output >&2\ntouch '"+started+"'\n")
	c.NoError(os.Chmod(s.Payload, executableModePerm)) //nolint:gosec // The stand-in application has to run
	statePath := filepath.Join(dir, stateName)
	c.NoError(s.save(statePath))

	c.NoError(newTestUpdater(t).Finish(statePath))

	for deadline := time.Now().Add(10 * time.Second); !exists(started); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the updated application was not started")
		}
	}
	if exists(s.LogPath) {
		c.False(strings.Contains(read(t, s.LogPath), "output"),
			"the restarted application must not write to the update's log")
	}
}
