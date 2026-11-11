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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
)

// testExitID is the identifier the stand-in for xos.RunAtExit hands out.
const testExitID = 7

// installWait is a generous bound on the wait for install, so that a progress dialog nothing closes fails the test
// rather than hanging it.
const installWait = 10 * time.Second

// installFixture is an Updater whose every effect outside its own process is recorded rather than performed.
type installFixture struct {
	u *Updater
	// onStage, when set, runs on the staging goroutine before the update is staged; an error it returns is staging's.
	onStage   func(ctx context.Context) error
	plan      *plan
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
	// It runs on the progress dialog's goroutine, where t.Fatal is not allowed, so failures are returned as staging's.
	f.u.stageFn = func(ctx context.Context, _ *plan, _ func(float64), _ func(phase)) (*staged, error) {
		if f.onStage != nil {
			if err := f.onStage(ctx); err != nil {
				return nil, err
			}
		}
		payload := filepath.Join(f.workDir, testCmd)
		if err := os.MkdirAll(f.workDir, 0o755); err != nil {
			t.Errorf("unable to stage the payload: %v", err)
			return nil, err
		}
		if err := os.WriteFile(payload, []byte("new"), 0o600); err != nil {
			t.Errorf("unable to stage the payload: %v", err)
			return nil, err
		}
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

// install runs install on the UI thread and waits for it. It is posted rather than run with Do, since install runs the
// progress dialog's modal loop, and Do would never return were nothing to end it.
func (f *installFixture) install(t *testing.T, screen *unison.HeadlessScreen) {
	t.Helper()
	finished := make(chan struct{})
	if !screen.Post(func() {
		defer close(finished)
		f.u.install(f.plan)
	}) {
		t.Fatal("the headless session has ended")
	}
	select {
	case <-finished:
	case <-time.After(installWait):
		t.Fatal("install did not return")
	}
}

// closeProgressDialog asks the progress dialog to close, as Alt+F4 does, which the dialog takes as a press of its
// Cancel button. UI thread only.
func closeProgressDialog() {
	for _, w := range unison.Windows() {
		if w.Title() == "Updating…" {
			w.AttemptClose()
		}
	}
}

// TestInstall verifies the hand-off around the application quitting. The helper is started only by the application
// actually exiting, and finds the staged update recorded on disk when it is. A quit that is refused throws the prepared
// update away, tells the user, and leaves nothing for a later exit to apply. A failure to prepare the update is
// reported and never asks the application to quit, and a cancel does the same without the report, however late it
// comes.
func TestInstall(t *testing.T) {
	screen := startHeadless(t)

	t.Run("the application exits", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		f.onQuit = func() { f.exitHook() }
		f.install(t, screen)
		c.Equal(1, f.quits)
		c.Equal([]string{string(statusStaged)}, f.spawned)
		c.Equal(4321, f.spawnPort, "the helper needs the application's handoff port to wait on")
		c.Equal(0, len(f.errors))
	})

	t.Run("the quit is refused", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		f.install(t, screen)
		c.Equal(1, f.quits)
		c.Equal(0, len(f.spawned), "nothing exited, so the helper must not have been started")
		c.Equal([]int{testExitID}, f.canceled, "the exit hook must be withdrawn")
		c.False(exists(f.u.stateFile), "the record of the prepared update must be removed")
		c.False(exists(f.workDir), "the prepared update must be removed")
		c.Equal("old", read(t, f.plan.Target.Path), "the installation must be untouched")
		c.Equal(1, len(f.warnings), "the user must be told the update was not installed")
		if f.exitHook == nil {
			t.Fatal("no exit hook was registered")
		}
		f.exitHook()
		c.Equal(0, len(f.spawned), "a later exit must find nothing left to apply")
		c.Equal(0, len(f.errors))
	})

	t.Run("the update cannot be prepared", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		f.onStage = func(context.Context) error { return errors.New("the download failed") }
		f.install(t, screen)
		c.Equal(1, len(f.errors), "the failure must be reported")
		c.Equal(0, f.quits, "the application must not be asked to quit")
		c.False(exists(f.u.stateFile))
	})

	t.Run("the update takes too long to prepare", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		saved := stageTimeout
		stageTimeout = time.Millisecond
		t.Cleanup(func() { stageTimeout = saved })
		f.onStage = func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}
		f.install(t, screen)
		c.Equal(1, len(f.errors), "running out of time is a failure, which must be reported")
		c.Equal(0, f.quits, "the application must not be asked to quit")
		c.False(exists(f.u.stateFile))
	})

	// Cancel lands after the last step that watches for it, as it can while the helper is copied or before the dialog
	// closes. The close request is queued ahead of the task that closes the dialog, so it is handled first.
	t.Run("Cancel is pressed once staging has stopped looking", func(t *testing.T) {
		c := check.New(t)
		f := newInstallFixture(t)
		f.onStage = func(context.Context) error {
			unison.InvokeTask(closeProgressDialog)
			return nil
		}
		f.install(t, screen)
		c.Equal(0, f.quits, "a canceled update must not quit the application")
		c.False(exists(f.u.stateFile), "a canceled update must not be recorded")
		c.False(exists(f.workDir), "the prepared update must be removed")
		c.Equal(0, len(f.errors), "a cancel is not a failure")
		c.Equal(0, len(f.warnings))
	})

	check.New(t).Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
