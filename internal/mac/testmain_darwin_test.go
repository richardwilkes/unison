// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

func init() {
	// AppKit requires event-loop calls ([NSApp run], nextEventMatchingMask:...) to happen on the process's main
	// thread. Package init runs on the main goroutine while it is still scheduled on the main OS thread, so locking
	// here pins the main goroutine (and therefore TestMain) to it. The tests themselves run on other goroutines and
	// submit main-thread work through runOnMain.
	runtime.LockOSThread()
}

// mainThreadWork carries closures that must execute on the process's main OS thread.
var mainThreadWork = make(chan func())

// testResult receives m.Run's exit code; pumpMainThread exits the process with it once the tests are done.
var testResult = make(chan int, 1)

// runOnMain runs f on the process's main thread and waits for it to complete. It must not be called from the main
// thread itself (i.e. from within another runOnMain closure), or it will deadlock. A closure that aborts with
// runtime.Goexit — which is what testing.T's FailNow/SkipNow (t.Fatal, t.Skip, ...) do, even though calling them off
// the test goroutine is technically misuse — is tolerated: the pump survives (see runPumped) and runOnMain still
// returns, because the deferred close of done runs during the Goexit.
func runOnMain(f func()) {
	done := make(chan struct{})
	mainThreadWork <- func() {
		defer close(done)
		f()
	}
	<-done
}

// TestMain serves two duties on the process's main thread while the tests run on a secondary goroutine: executing
// closures submitted through runOnMain, and pumping the main run loop so run-loop-based delivery keeps working the
// way unison's real event loop keeps it working in production. Notably, NSDistributedNotificationCenter delivery
// requires the default center to be created from and pumped on this thread (verified empirically — creating it from
// any other thread breaks delivery permanently), so the theme-change observer must be installed here, before any
// test can cause another thread to touch the center first.
func TestMain(m *testing.M) {
	InstallSystemThemeChangedCallback(func() { themeFired.Store(true) })
	go func() {
		testResult <- m.Run()
	}()
	pumpMainThread()
}

// pumpMainThread runs the main-thread service loop: it executes runOnMain closures, pumps the main run loop, and
// exits the process when the tests complete. It never returns.
func pumpMainThread() {
	for {
		select {
		case f := <-mainThreadWork:
			runPumped(f)
		case code := <-testResult:
			os.Exit(code)
		default:
			WithPool(func() {
				date := objc.ID(Cls("NSDate")).Send(Sel("dateWithTimeIntervalSinceNow:"), 0.02)
				objc.ID(Cls("NSRunLoop")).Send(Sel("currentRunLoop")).Send(Sel("runMode:beforeDate:"),
					defaultRunLoopMode(), date)
			})
			time.Sleep(time.Millisecond) // ensure we never spin hot if the run loop has no sources to wait on
		}
	}
}

// runPumped runs one runOnMain closure so that the pump survives a runtime.Goexit raised inside it. t.Fatal/t.Skip
// (FailNow/SkipNow) call runtime.Goexit, and any closure that uses test helpers can reach them; unprotected, a single
// Goexit unwinds and kills the whole pump goroutine, deadlocking every later runOnMain caller until the package times
// out. This is exactly what the headless CI runners hit when NewOpenGLPixelFormat legitimately returned 0 (no
// hardware-accelerated GL there) and newTestPixelFormat called t.Fatal — see progress.md, session 7 CI followup.
// Goexit cannot be stopped, but deferred functions still run during it, so the deferred handler re-enters the pump
// loop and never returns: the Goexit stays parked in this frame for the life of the process, the main thread keeps
// servicing work, and the test that Goexited is reported failed/skipped by its own goroutine as usual. Real panics
// are re-raised so genuine crashes stay loud.
func runPumped(f func()) {
	completed := false
	defer func() {
		if r := recover(); r != nil {
			panic(r) // a real panic in a closure must still crash the test binary loudly
		}
		if !completed {
			pumpMainThread() // never returns; the in-flight Goexit stays parked in this frame
		}
	}()
	f()
	completed = true
}

// TestRunOnMainSurvivesGoexit proves the runPumped contract that keeps one broken test from deadlocking the whole
// package: after a closure aborts with runtime.Goexit (the mechanism behind t.Fatal/t.Skip inside runOnMain closures,
// the misuse that deadlocked the headless CI runners), runOnMain must keep working for every later caller.
func TestRunOnMainSurvivesGoexit(t *testing.T) {
	runOnMain(func() { runtime.Goexit() })
	ran := false
	runOnMain(func() { ran = true })
	if !ran {
		t.Fatal("the main-thread pump did not survive a runtime.Goexit inside a runOnMain closure")
	}
}
