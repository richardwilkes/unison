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
)

// These tests exercise the bookkeeping behind ToFront() with hand-built windows that have no platform backing, so
// they must never let a request reach the platform: flushPendingFront is only ever called for a window that it will
// decline to activate. The empty event requestFront posts is harmless, since every backend ignores it before Start()
// has run.

// saveFrontState restores the package-level window state the tests below alter once the test is over.
func saveFrontState(t *testing.T) {
	t.Helper()
	savedList := windowList
	savedPending := pendingFrontWindow
	t.Cleanup(func() {
		windowList = savedList
		pendingFrontWindow = savedPending
	})
}

// TestFrontRequestsCoalesce verifies that requests made within a pass collapse into the most recent one, which is what
// lets a menu closing (which fronts the window underneath) followed by a dialog opening activate only the dialog.
func TestFrontRequestsCoalesce(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	b := newModalInputTestWindow()
	pendingFrontWindow = nil
	requestFront(a)
	c.True(pendingFrontWindow == a)
	requestFront(b)
	c.True(pendingFrontWindow == b, "the most recent request should replace the earlier one")
	requestFront(b)
	c.True(pendingFrontWindow == b)
}

// TestFlushPendingFrontDropsDisposedWindow verifies that a request outstanding for a window that has since been
// disposed of is discarded rather than performed. Reaching the platform for a hand-built window would panic, which is
// the assertion.
func TestFlushPendingFrontDropsDisposedWindow(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	pendingFrontWindow = nil
	requestFront(a)
	a.valid = false
	flushPendingFront()
	c.Nil(pendingFrontWindow, "the request should have been cleared")
	flushPendingFront() // nothing pending: must be a no-op
	c.Nil(pendingFrontWindow)
}

// TestFlushPendingFrontHonorsKeepHidden verifies that a window marked keepHidden is never activated, matching the
// check ToFront() itself makes, since the mark may be applied after the request was made.
func TestFlushPendingFrontHonorsKeepHidden(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	pendingFrontWindow = nil
	requestFront(a)
	a.keepHidden = true
	flushPendingFront()
	c.Nil(pendingFrontWindow)
}

// TestActiveWindowPrefersPendingFront verifies that a window whose activation is still outstanding is reported as the
// active one, so that anything positioned or parented relative to the active window during the same pass — a dialog
// centering itself, for instance — uses the window that is about to hold the focus rather than the one that still
// does. A transient window is never reported, matching the treatment of focused transient windows.
func TestActiveWindowPrefersPendingFront(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	b := newModalInputTestWindow()
	a.focused = true
	windowList = []*Window{a, b}
	pendingFrontWindow = nil
	c.True(ActiveWindow() == a)
	pendingFrontWindow = b
	c.True(ActiveWindow() == b, "the window about to be activated should be reported as active")
	b.transient = true
	c.True(ActiveWindow() == a, "a transient window is never the active window")
	b.transient = false
	b.valid = false
	c.True(ActiveWindow() == a, "a disposed window is never the active window")
}
