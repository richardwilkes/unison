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
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/mod"
)

// newMouseButtonTestWindow returns a minimal Window suitable for exercising mouse button bookkeeping without a live
// windowing system. The window deliberately reports itself as invalid so MouseLocation returns the origin instead of
// querying the platform, and cursor updates skip the platform cursor-image call. The content panel is given a frame
// covering the window and a cursor callback so the post-release cursor update never has to fall back to the
// platform-backed ArrowCursor. The window claims the focus so mouse downs are dispatched to panels.
func newMouseButtonTestWindow() *Window {
	w := &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.root = newRootPanel(w)
	w.root.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	w.root.contentPanel.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	w.root.contentPanel.UpdateCursorCallback = func(_ geom.Point) *Cursor { return &Cursor{} }
	w.focused = true
	return w
}

// TestSecondButtonReleaseIsDelivered is the regression test for multi-button presses: with two buttons down, the first
// release must not end the whole mouse-down interaction, and the second release must still reach the panel's
// MouseUpCallback.
func TestSecondButtonReleaseIsDelivered(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	content := w.root.contentPanel
	var downs, ups []int
	content.MouseDownCallback = func(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
		downs = append(downs, button)
		return true
	}
	content.MouseUpCallback = func(_ geom.Point, button int, _ mod.Modifiers) bool {
		ups = append(ups, button)
		return true
	}
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseDown(pt, ButtonRight, 0)
	c.Equal([]int{ButtonLeft, ButtonRight}, downs)
	c.True(w.inMouseDown)
	c.True(w.pressedButtons[ButtonLeft])
	c.True(w.pressedButtons[ButtonRight])

	w.mouseUp(pt, ButtonLeft, 0)
	c.Equal([]int{ButtonLeft}, ups)
	c.True(w.inMouseDown, "the interaction must continue while another button remains down")
	c.NotNil(w.lastMouseDownPanel, "the drag target must be retained until the last button is released")

	w.mouseUp(pt, ButtonRight, 0)
	c.Equal([]int{ButtonLeft, ButtonRight}, ups)
	c.False(w.inMouseDown)
	c.Equal(0, len(w.pressedButtons), "released buttons must be removed from the pressed set, not just marked false")
	c.Nil(w.lastMouseDownPanel)
}

// TestMouseUpWithoutMatchingDownIsIgnored verifies that a release with no matching press — e.g. a click begun before
// the window existed — is dropped rather than dispatched.
func TestMouseUpWithoutMatchingDownIsIgnored(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	ups := 0
	w.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		ups++
		return true
	}
	w.mouseUp(geom.NewPoint(10, 10), ButtonLeft, 0)
	c.Equal(0, ups)
}

// TestSynthesizeMouseUpReleasesAllPressedButtons verifies that the synthetic releases used on focus loss and at drag
// start cover every pressed button and leave the pressed set empty.
func TestSynthesizeMouseUpReleasesAllPressedButtons(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	var ups []int
	w.MouseUpCallback = func(_ geom.Point, button int, _ mod.Modifiers) bool {
		ups = append(ups, button)
		return true
	}
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseDown(pt, ButtonRight, 0)
	w.synthesizeMouseUp()
	c.Equal(2, len(ups))
	c.True(slices.Contains(ups, ButtonLeft))
	c.True(slices.Contains(ups, ButtonRight))
	c.False(w.inMouseDown)
	c.Equal(0, len(w.pressedButtons))
}
