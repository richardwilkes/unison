// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// macUnknownWindow is a window handle no unison window can have, for exercising the callbacks' unknown-window paths
// without going anywhere near AppKit. Zero would not do: a headless window's native handle is zero, so macFindWindow
// would match one.
const macUnknownWindow cocoa.Window = 0xdead0000

// TestMacAccessibilityCallbacksInstalled proves the two Cocoa accessibility callbacks are wired up, which is what
// nativeLateInit does at startup, and that both refuse a window they cannot resolve rather than acting on the wrong
// one. Everything the adapter itself does needs a real screen and is tested in internal/cocoa. This test mutates
// global state and therefore must not call t.Parallel.
func TestMacAccessibilityCallbacksInstalled(t *testing.T) {
	c := check.New(t)
	savedActivate := cocoa.AccessibilityActivateCallback
	savedAction := cocoa.AccessibilityActionCallback
	t.Cleanup(func() {
		cocoa.AccessibilityActivateCallback = savedActivate
		cocoa.AccessibilityActionCallback = savedAction
	})
	cocoa.AccessibilityActivateCallback = nil
	cocoa.AccessibilityActionCallback = nil

	macInitAccessibilityCallbacks()
	c.NotNil(cocoa.AccessibilityActivateCallback)
	c.NotNil(cocoa.AccessibilityActionCallback)

	// A window that does not exist activates nothing, so nothing is built and no adapter appears.
	before := axSnapshotCount
	c.False(cocoa.AccessibilityActivateCallback(macUnknownWindow))
	c.Equal(before, axSnapshotCount)
	c.False(IsAccessibilityActive())

	// An action for a window that does not exist is dropped rather than queued.
	resetTaskQueue()
	cocoa.AccessibilityActionCallback(macUnknownWindow, accessibility.ActionRequest{Action: accessibility.Press})
}
