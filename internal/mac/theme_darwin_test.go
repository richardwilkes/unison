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
	"sync/atomic"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

func TestIsDarkModeEnabled(t *testing.T) {
	// The result depends on the user's settings; this exercises the NSUserDefaults path for crashes only.
	_ = IsDarkModeEnabled()
}

var themeFired atomic.Bool

// TestMain starts the theme-change observer pump before any test runs. Distributed notifications are delivered on
// the run loop of the thread that FIRST created the default NSDistributedNotificationCenter (the addObserver:
// thread is irrelevant — see InstallSystemThemeChangedCallback), and other tests can cause AppKit/Foundation to
// touch the distributed center lazily from their own transient threads, silently binding delivery to a run loop
// nobody pumps. Installing from TestMain guarantees this package's pump thread wins the race, mirroring how unison
// installs the observer on the main thread at startup before anything else runs.
func TestMain(m *testing.M) {
	startThemePump()
	os.Exit(m.Run())
}

// startThemePump installs the theme-change observer from a dedicated, permanently locked OS thread and keeps that
// thread's run loop pumping for the remainder of the test process. The observer registration is process-global
// (sync.Once in InstallSystemThemeChangedCallback), so repeated test runs in one process (-count=N) all share this
// one registering thread.
func startThemePump() {
	ready := make(chan struct{})
	go func() {
		runtime.LockOSThread() // never unlocked; the observer's run loop lives on this thread
		InstallSystemThemeChangedCallback(func() { themeFired.Store(true) })
		close(ready)
		mode := NSStringConstant("Foundation", "NSDefaultRunLoopMode")
		for {
			WithPool(func() {
				date := objc.ID(Cls("NSDate")).Send(Sel("dateWithTimeIntervalSinceNow:"), 0.05)
				objc.ID(Cls("NSRunLoop")).Send(Sel("currentRunLoop")).Send(Sel("runMode:beforeDate:"), mode, date)
			})
		}
	}()
	<-ready
}

// TestThemeChangedNotification proves the full ThemeDelegate path: Go-implemented Objective-C class registration,
// distributed-notification observation, and dispatch back into the Go callback.
func TestThemeChangedNotification(t *testing.T) {
	themeFired.Store(false)
	WithPool(func() {
		objc.ID(Cls("NSDistributedNotificationCenter")).Send(Sel("defaultCenter")).Send(
			Sel("postNotificationName:object:userInfo:deliverImmediately:"),
			NSStringFromGo("AppleInterfaceThemeChangedNotification"), 0, 0, true)
	})
	for deadline := time.Now().Add(5 * time.Second); !themeFired.Load() && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	if !themeFired.Load() {
		t.Error("theme-change notification was not delivered")
	}
}
