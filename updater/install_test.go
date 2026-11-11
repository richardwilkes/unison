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
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// testExitID is the identifier the stand-in for xos.RunAtExit hands out.
const testExitID = 7

// installFixture is an Updater whose every effect outside its own process is recorded rather than performed.
type installFixture struct {
	u         *Updater
	plan      *plan
	stageErr  error
	exitHook  func()
	onQuit    func()
	workDir   string
	spawned   []string // The status recorded in the state file each time the helper was started
	canceled  []int
	warnings  []string
	errors    []string
	quits     int
	spawnPort int
}

func newInstallFixture(t *testing.T) *installFixture {
	t.Helper()
	dir := t.TempDir()
	installDir := filepath.Join(dir, "install")
	exe := filepath.Join(installDir, testCmd)
	write(t, exe, "old")
	f := &installFixture{
		workDir: filepath.Join(installDir, ".example-update-1"),
		plan: &plan{
			FromVersion: testVersion,
			ToVersion:   testNextVersion,
			Target:      target{Path: exe, Parent: installDir, Exec: exe, Kind: kindExecutable},
		},
	}
	f.u = New(&Config{
		AppName:      testApp,
		CmdName:      testCmd,
		Version:      testVersion,
		HandoffPort:  4321,
		ErrorHandler: func(title string, _ error) { f.errors = append(f.errors, title) },
	})
	f.u.stateFile = filepath.Join(dir, "data", stateName)
	f.u.stageFn = func(context.Context, *plan, func(float64), func(phase)) (*staged, error) {
		if f.stageErr != nil {
			return nil, f.stageErr
		}
		payload := filepath.Join(f.workDir, testCmd)
		write(t, payload, "new")
		return &staged{
			WorkDir: f.workDir,
			Payload: payload,
			Helper:  filepath.Join(f.workDir, helperName),
			LogPath: filepath.Join(f.workDir, logName),
		}, nil
	}
	f.u.atExitFn = func(hook func()) int {
		f.exitHook = hook
		return testExitID
	}
	f.u.cancelAtExitFn = func(id int) { f.canceled = append(f.canceled, id) }
	f.u.quitFn = func() {
		f.quits++
		if f.onQuit != nil {
			f.onQuit()
		}
	}
	f.u.spawnFn = func(s *state, statePath string) error {
		// What the helper will find when it reads the file, which is all it has to go on.
		saved, err := loadState(statePath)
		if err != nil || saved == nil {
			t.Errorf("the helper was started without a readable state file: %v", err)
			return nil
		}
		f.spawned = append(f.spawned, string(saved.Status))
		f.spawnPort = s.HandoffPort
		return nil
	}
	f.u.warnFn = func(primary, _ string) { f.warnings = append(f.warnings, primary) }
	return f
}

// TestInstall verifies the hand-off around the application quitting. The helper is started only by the application
// actually exiting, and finds the staged update recorded on disk when it is. A quit that is refused throws the prepared
// update away, tells the user, and leaves nothing for a later exit to apply. A failure to prepare the update is
// reported and never asks the application to quit.
func TestInstall(t *testing.T) {
	screen := startHeadless(t)

	t.Run("the application exits", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		f.onQuit = func() { f.exitHook() }
		c.True(screen.Do(func() { f.u.install(f.plan) }))
		c.Equal(1, f.quits)
		c.Equal([]string{string(statusStaged)}, f.spawned)
		c.Equal(4321, f.spawnPort, "the helper needs the application's handoff port to wait on")
		c.Equal(0, len(f.errors))
	})

	t.Run("the quit is refused", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		c.True(screen.Do(func() { f.u.install(f.plan) }))
		c.Equal(1, f.quits)
		c.Equal(0, len(f.spawned), "nothing exited, so the helper must not have been started")
		c.Equal([]int{testExitID}, f.canceled, "the exit hook must be withdrawn")
		c.False(exists(f.u.stateFile), "the record of the prepared update must be removed")
		c.False(exists(f.workDir), "the prepared update must be removed")
		c.Equal("old", read(t, f.plan.Target.Path), "the installation must be untouched")
		c.Equal(1, len(f.warnings), "the user must be told the update was not installed")
		f.exitHook()
		c.Equal(0, len(f.spawned), "a later exit must find nothing left to apply")
		c.Equal(0, len(f.errors))
	})

	t.Run("the update cannot be prepared", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		f.stageErr = errors.New("the download failed")
		c.True(screen.Do(func() { f.u.install(f.plan) }))
		c.Equal(1, len(f.errors), "the failure must be reported")
		c.Equal(0, f.quits, "the application must not be asked to quit")
		c.False(exists(f.u.stateFile))
	})

	check.New(t).Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
