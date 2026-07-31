// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

// TestAppDelegateAndLaunch exercises the full app-delegate lifecycle: the Go-registered delegate class, launching
// via [NSApp run] until applicationDidFinishLaunching stops the loop (mirroring unison's startup), delegate dispatch
// for termination and open-files, the Cmd+keyUp monitor block, and install/uninstall/reinstall balance.
func TestAppDelegateAndLaunch(t *testing.T) {
	var willFinish, didFinish, shouldTerminate atomic.Bool
	AppWillFinishLaunchingCallback = func() { willFinish.Store(true) }
	AppDidFinishLaunchingCallback = func() {
		didFinish.Store(true)
		PostEmptyEvent()
		StopMainEventLoop()
	}
	AppShouldTerminateCallback = func() { shouldTerminate.Store(true) }
	defer func() {
		AppWillFinishLaunchingCallback = nil
		AppDidFinishLaunchingCallback = nil
		AppShouldTerminateCallback = nil
		OpenFilesCallback = nil
	}()

	var err error
	var alreadyLaunched bool
	runOnMain(func() {
		err = InstallMacAppDelegate()
		// A process can only launch once, so on reruns in the same process (-count=N) FinishLaunching correctly
		// skips [NSApp run] and the launch notifications cannot be delivered again.
		alreadyLaunched = objc.Send[bool](objc.ID(Cls("NSRunningApplication")).Send(Sel("currentApplication")),
			Sel("isFinishedLaunching"))
	})
	if err != nil {
		t.Fatal(err)
	}

	// FinishLaunching blocks in [NSApp run] until the delegate's applicationDidFinishLaunching callback stops the
	// loop. Guard with a watchdog so a regression fails instead of hanging the whole test binary.
	done := make(chan struct{})
	go func() {
		runOnMain(FinishLaunching)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		StopMainEventLoop()
		PostEmptyEvent()
		t.Fatal("FinishLaunching did not return")
	}
	if !alreadyLaunched {
		if !willFinish.Load() {
			t.Error("applicationWillFinishLaunching was not delivered")
		}
		if !didFinish.Load() {
			t.Error("applicationDidFinishLaunching was not delivered")
		}
	}

	// applicationShouldTerminate: reports NSTerminateCancel to AppKit, so terminate: must invoke the callback and
	// then return control here rather than exiting the process.
	runOnMain(func() { sharedApp().Send(Sel("terminate:"), objc.ID(0)) })
	if !shouldTerminate.Load() {
		t.Error("applicationShouldTerminate was not delivered")
	}

	// Drive application:openURLs: through objc_msgSend to prove the Go implementation and the URL conversion.
	var got []string
	OpenFilesCallback = func(paths []string) { got = paths }
	runOnMain(func() {
		WithPool(func() {
			appDelegate.Send(Sel("application:openURLs:"), sharedApp(),
				NSArrayFromIDs(NSURLFromFilePath("/tmp/mac test file.txt")))
		})
	})
	if len(got) != 1 || got[0] != "/tmp/mac test file.txt" {
		t.Errorf("open-files delegate produced %v", got)
	}

	// The Cmd+keyUp monitor must forward to the (currently nil) key window and return the event unchanged.
	runOnMain(func() {
		WithPool(func() {
			event := cmdKeyUpEvent()
			ret, err2 := objc.InvokeBlock[objc.ID](keyUpBlock, event)
			switch {
			case err2 != nil:
				t.Error(err2)
			case ret != event:
				t.Errorf("key-up monitor returned %#x, want %#x", ret, event)
			}
			// Route the same event through the real queue so AppKit itself invokes the monitor block.
			sharedApp().Send(Sel("postEvent:atStart:"), event, true)
			PollEvents()
		})
	})

	// Uninstall, then a second install/uninstall cycle must work even though the delegate class can only be
	// registered once per process.
	runOnMain(UninstallMacAppDelegate)
	runOnMain(func() { err = InstallMacAppDelegate() })
	if err != nil {
		t.Fatal(err)
	}

	// A repeated install without an intervening uninstall must replace the prior installation — a fresh delegate
	// instance and key-up monitor take over — rather than leaking the old ones (a leaked monitor would forward every
	// Cmd+keyUp event once per leaked install). A single uninstall must then clear everything.
	var firstDelegate, firstMonitor objc.ID
	runOnMain(func() {
		// Pin the first install's objects so their addresses cannot be recycled for the second install's, which
		// would make the distinctness checks below meaningless.
		firstDelegate = Retain(appDelegate)
		firstMonitor = Retain(keyUpMonitor)
		err = InstallMacAppDelegate()
	})
	if err != nil {
		t.Fatal(err)
	}
	runOnMain(func() {
		if appDelegate == firstDelegate {
			t.Error("second install did not create a fresh delegate instance")
		}
		if keyUpMonitor == firstMonitor {
			t.Error("second install did not replace the key-up monitor")
		}
		if got := sharedApp().Send(Sel("delegate")); got != appDelegate {
			t.Errorf("app delegate = %#x after second install, want %#x", got, appDelegate)
		}
		Release(firstDelegate)
		Release(firstMonitor)
	})
	runOnMain(UninstallMacAppDelegate)
	runOnMain(func() {
		if appDelegate != 0 || keyUpMonitor != 0 || keyUpBlock != 0 {
			t.Errorf("uninstall left state behind: delegate %#x, monitor %#x, block %#x",
				appDelegate, keyUpMonitor, keyUpBlock)
		}
		if got := sharedApp().Send(Sel("delegate")); got != 0 {
			t.Errorf("app delegate = %#x after uninstall, want 0", got)
		}
	})
}

// cmdKeyUpEvent synthesizes a key-up NSEvent with the Command modifier set.
func cmdKeyUpEvent() objc.ID {
	return objc.ID(Cls("NSEvent")).Send(
		Sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
		uint64(11), // NSEventTypeKeyUp
		NSPoint{},
		uint64(EventModifierFlagCommand),
		float64(0), int64(0), objc.ID(0),
		NSStringFromGo("a"), NSStringFromGo("a"), false, uint16(0))
}

// TestSetMenus verifies the four application menu setters against their AppKit getters, which also proves that the
// cgo bridge's Menu handles interoperate with the purego side while the two coexist. Once the application has
// launched, AppKit manages the services and windows menus itself and may substitute its own instances, so exact
// read-back is only asserted for those two before launch.
func TestSetMenus(t *testing.T) {
	runOnMain(func() {
		app := sharedApp()
		launched := objc.Send[bool](objc.ID(Cls("NSRunningApplication")).Send(Sel("currentApplication")),
			Sel("isFinishedLaunching"))
		for _, one := range []struct {
			set   func(Menu)
			name  string
			get   objc.SEL
			exact bool
		}{
			{name: "main", set: SetMainMenu, get: Sel("mainMenu"), exact: true},
			{name: "services", set: SetServicesMenu, get: Sel("servicesMenu"), exact: !launched},
			{name: "windows", set: SetWindowsMenu, get: Sel("windowsMenu"), exact: !launched},
			{name: "help", set: SetHelpMenu, get: Sel("helpMenu"), exact: true},
		} {
			m := NewMenu("Test "+one.name, nil)
			one.set(m)
			got := app.Send(one.get)
			if one.exact && got != objc.ID(m) {
				t.Errorf("%s menu: got %#x, want %#x", one.name, got, m)
			} else if got == 0 {
				t.Errorf("%s menu: got nil", one.name)
			}
			m.Release()
		}
	})
}
