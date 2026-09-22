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
	"github.com/richardwilkes/unison/internal/testenv"
)

func TestIsDarkModeEnabled(_ *testing.T) {
	// The result depends on the user's settings; this exercises the NSUserDefaults path for crashes only.
	_ = IsDarkModeEnabled()
}

// TestRegisterThemeObserverNameCollision proves a class-name collision (a host app or another framework already
// defining the delegate class) degrades to an error instead of panicking during startup: dark-mode tracking is
// non-essential and must not crash the whole app. NSObject is used as a name guaranteed to already exist.
func TestRegisterThemeObserverNameCollision(t *testing.T) {
	if err := registerThemeObserver("NSObject"); err == nil {
		t.Error("registerThemeObserver with an already-registered class name returned nil, want an error")
	}
}

// themeFired records deliveries of the theme-change notification. The observer is installed by TestMain from the
// main thread before any test runs — distributed-notification delivery is bound to the main thread's run loop (see
// TestMain and InstallSystemThemeChangedCallback), and the main thread keeps that run loop pumping for the entire
// test process. The observer registration is process-global (sync.Once in InstallSystemThemeChangedCallback), so
// repeated test runs in one process (-count=N) all share the one registration.
var themeFired atomic.Bool

// TestThemeChangedNotification proves the full macThemeDelegate path: Go-implemented Objective-C class registration,
// distributed-notification observation, and dispatch back into the Go callback.
func TestThemeChangedNotification(t *testing.T) {
	testenv.SkipTimingSensitive(t)
	themeFired.Store(false)
	WithPool(func() {
		objc.ID(Cls("NSDistributedNotificationCenter")).Send(Sel("defaultCenter")).Send(
			Sel("postNotificationName:object:userInfo:deliverImmediately:"),
			NSStringFromGo("AppleInterfaceThemeChangedNotification"), 0, 0, true,
		)
	})
	for deadline := time.Now().Add(5 * time.Second); !themeFired.Load() && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	if !themeFired.Load() {
		t.Error("theme-change notification was not delivered")
	}
}

// TestSetAppAppearance verifies that the application appearance can be set to each of the named appearances and read
// back, and that an empty name clears it so the application follows the system again.
func TestSetAppAppearance(t *testing.T) {
	t.Cleanup(func() { runOnMain(func() { SetAppAppearance("") }) })
	for _, name := range []string{AppearanceNameDarkAqua, AppearanceNameAqua} {
		var got string
		runOnMain(func() {
			SetAppAppearance(name)
			got = AppAppearanceName()
		})
		if got != name {
			t.Errorf("expected appearance %q, got %q", name, got)
		}
	}
	var got string
	runOnMain(func() {
		SetAppAppearance("")
		got = AppAppearanceName()
	})
	if got != "" {
		t.Errorf("expected no appearance after clearing it, got %q", got)
	}
}
