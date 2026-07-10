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

// runOnMain runs f on the process's main thread and waits for it to complete. It must not be called from the main
// thread itself (i.e. from within another runOnMain closure), or it will deadlock.
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
	result := make(chan int, 1)
	go func() {
		result <- m.Run()
	}()
	for {
		select {
		case f := <-mainThreadWork:
			f()
		case code := <-result:
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
