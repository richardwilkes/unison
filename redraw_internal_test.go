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

// swapRedrawSet installs a fresh, empty redrawSet for the duration of the test, restoring the previous set when the
// test completes, so these tests neither see nor disturb redraw state left behind by other tests in the package.
func swapRedrawSet(t *testing.T) {
	t.Helper()
	saved := redrawSet
	redrawSet = make(map[*Window]struct{})
	t.Cleanup(func() { redrawSet = saved })
}

// preventQuitOnLastWindowClosed keeps Dispose from invoking quitting() (which would terminate the test process) when
// the window list is empty, restoring the previous callback when the test completes.
func preventQuitOnLastWindowClosed(t *testing.T) {
	t.Helper()
	saved := quitAfterLastWindowClosedCallback
	quitAfterLastWindowClosedCallback = func() bool { return false }
	t.Cleanup(func() { quitAfterLastWindowClosedCallback = saved })
}

// newRedrawTestWindow returns a minimal Window suitable for exercising the redraw bookkeeping without a live windowing
// system. The window reports itself as valid, but has no platform resources, so tests must only drive code paths that
// check validity before touching platform APIs.
func newRedrawTestWindow() *Window {
	w := &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.valid = true
	w.root = newRootPanel(w)
	return w
}

// TestCloseRemovesPendingRedraw covers the leak where a window disposed while a redraw was pending was retained in
// redrawSet forever: e.g. a click handler that marks panels for redraw and then calls AttemptClose(). The valid flag is
// cleared before closing so Dispose skips the platform teardown, which cannot run in a headless test; the redrawSet
// removal it performs is unconditional and is what this test verifies.
func TestCloseRemovesPendingRedraw(t *testing.T) {
	c := check.New(t)
	preventQuitOnLastWindowClosed(t)
	w := newRedrawTestWindow()
	swapRedrawSet(t)
	w.MarkForRedraw()
	_, pending := redrawSet[w]
	c.True(pending, "window should have a pending redraw after MarkForRedraw")
	w.valid = false
	c.True(w.AttemptClose(), "AttemptClose should succeed without an AllowCloseCallback")
	_, pending = redrawSet[w]
	c.False(pending, "disposing a window must remove its pending redraw request")
}

// TestFinishProcessingEventsDropsDisposedWindows verifies that the redraw pass discards windows that were disposed
// while a redraw was pending, rather than re-queuing them on every pass forever.
func TestFinishProcessingEventsDropsDisposedWindows(t *testing.T) {
	c := check.New(t)
	w := newRedrawTestWindow()
	swapRedrawSet(t)
	w.MarkForRedraw()
	w.valid = false // Simulate disposal without platform teardown.
	finishProcessingEvents()
	_, pending := redrawSet[w]
	c.False(pending, "the redraw pass must not re-queue a disposed window")
	c.Equal(0, len(redrawSet))
}

// TestMarkForRedrawIgnoresDisposedWindow verifies that a disposed window can never re-enter redrawSet, since it can
// never be drawn again.
func TestMarkForRedrawIgnoresDisposedWindow(t *testing.T) {
	c := check.New(t)
	w := newRedrawTestWindow()
	w.valid = false
	swapRedrawSet(t)
	w.MarkForRedraw()
	c.Equal(0, len(redrawSet), "MarkForRedraw on a disposed window must be a no-op")
}
