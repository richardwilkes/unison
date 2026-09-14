// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
)

// These tests cover the one path that paints a window without the event loop having decided that it is on the screen:
// Window.FlushDrawing, which draws whatever is pending on the spot. A window that is hidden or minimized is a permanent
// resident of the pending set, and the library flushes on its own account from a menu closing, a list flashing its
// selection and a drag finishing, so a flush landing on a window a person cannot see is ordinary rather than exotic. A
// session owns most of the package's mutable globals while it runs, so neither of these may call t.Parallel.

// axFlushWindow is a window holding one describable panel that counts the times it is painted, along with the session
// running it.
type axFlushWindow struct {
	screen *unison.HeadlessScreen
	wnd    *unison.Window
	panel  *unison.Panel
	draws  int // UI thread only, since that is where drawing happens
}

// newAXFlushWindow starts a session showing an axFlushWindow that has been described once, so that there is something
// an assistive technology holds for the tests below to watch being withdrawn.
func newAXFlushWindow(t *testing.T, c check.Checker, title string) *axFlushWindow {
	t.Helper()
	out := &axFlushWindow{}
	out.screen = startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			out.panel = axFocusablePanel("Content")
			// Given a size of its own, since a panel the layout hands nothing to is never drawn and its draw callback
			// would then never run.
			out.panel.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
				size := geom.NewSize(80, 40)
				return size, size, size
			})
			out.panel.DrawCallback = func(_ *unison.Canvas, _ geom.Rect) { out.draws++ }
			out.wnd = newHeadlessWindow(t, title, geom.NewRect(20, 20, 240, 120), axColumn(out.panel))
		}))
	c.NotNil(out.wnd)
	c.NotNil(out.panel)
	c.True(out.screen.AccessibilityTree(out.wnd) != nil, "the window should describe itself")
	c.True(out.screen.AccessibilityNodeFor(out.panel) != nil, "its panel should be part of that description")
	return out
}

// flush marks the window for redraw and draws it on the spot, which is what every in-library caller of FlushDrawing
// does. Asked for through the panel afterwards rather than through AccessibilityTree, which would describe the window
// again and put back the very thing being checked for.
func (a *axFlushWindow) flush() {
	a.screen.Do(func() {
		a.wnd.MarkForRedraw()
		a.wnd.FlushDrawing()
	})
}

// drawCount returns how many times the panel has been painted, read on the UI thread that owns the count.
func (a *axFlushWindow) drawCount() int {
	var count int
	a.screen.Do(func() { count = a.draws })
	return count
}

// TestAccessibilityFlushingAHiddenWindowLeavesItWithdrawn verifies that a window drawn by FlushDrawing while it is
// hidden is not described again. The event loop withdraws a window it finds off the screen, but FlushDrawing draws
// whatever is pending with no visibility test of its own, so a publish that was not gated on the window being visible
// would put the withdrawn description straight back — and permanently, since the draw takes the window out of the
// pending set the withdrawal branch works from. On AT-SPI, where the application lists its own windows, that is a
// window a person cannot see reported as showing and visible for the rest of its life.
func TestAccessibilityFlushingAHiddenWindowLeavesItWithdrawn(t *testing.T) {
	c := check.New(t)
	a := newAXFlushWindow(t, c, "flush hidden")

	var visible bool
	a.screen.Do(func() {
		a.wnd.Hide()
		visible = a.wnd.IsVisible()
	})
	c.False(visible, "a hidden window is off the screen")
	c.True(a.screen.AccessibilityNodeFor(a.panel) == nil, "hiding the window should have withdrawn its description")

	a.flush()
	c.True(a.screen.AccessibilityNodeFor(a.panel) == nil,
		"flushing a hidden window must not describe it again")

	// Twice, because the failure this guards against is not a description that comes back for one pass but one that
	// comes back and stays: the draw removes the window from the pending set, so the withdrawal never runs again.
	a.screen.Sync()
	a.screen.Sync()
	a.screen.Do(func() { visible = a.wnd.IsVisible() })
	c.False(visible, "the window is still off the screen")
	c.True(a.screen.AccessibilityNodeFor(a.panel) == nil,
		"a window that is still hidden must stay withdrawn")

	// The redraw a flush of an off-screen window consumed has to go back, or the withdrawal branch of the event loop
	// never sees the window again and the draw that describes it when it returns to the screen has to be asked for by
	// something else. A flush with nothing marked draws only a window that is still pending, so this says that it is.
	before := a.drawCount()
	a.screen.Do(func() { a.wnd.FlushDrawing() })
	c.True(a.drawCount() > before, "the redraw of a window that could not be described must stay pending")

	a.screen.Do(func() { a.wnd.Show() })
	c.True(a.screen.AccessibilityNodeFor(a.panel) != nil,
		"showing the window again should draw it and describe it afresh")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestAccessibilityFlushingAMinimizedWindowLeavesItWithdrawn verifies the same for a minimized window, which is off the
// screen for exactly the same reasons — X11 unmaps an iconified window and AppKit reports a miniaturized one as not
// visible — and which an application is every bit as free to flush.
func TestAccessibilityFlushingAMinimizedWindowLeavesItWithdrawn(t *testing.T) {
	c := check.New(t)
	a := newAXFlushWindow(t, c, "flush minimized")

	var visible bool
	a.screen.Do(func() {
		a.wnd.Minimize()
		visible = a.wnd.IsVisible()
	})
	c.False(visible, "a minimized window is off the screen")
	c.True(a.screen.AccessibilityNodeFor(a.panel) == nil,
		"minimizing the window should have withdrawn its description")

	a.flush()
	a.screen.Sync()
	a.screen.Sync()
	a.screen.Do(func() { visible = a.wnd.IsVisible() })
	c.False(visible, "the window is still off the screen")
	c.True(a.screen.AccessibilityNodeFor(a.panel) == nil,
		"flushing a minimized window must not describe it again")

	// Minimize() toggles, so this restores it.
	a.screen.Do(func() { a.wnd.Minimize() })
	a.screen.Do(func() { visible = a.wnd.IsVisible() })
	c.True(visible, "a restored window is back on the screen")
	c.True(a.screen.AccessibilityNodeFor(a.panel) != nil,
		"restoring the window should draw it and describe it afresh")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}
