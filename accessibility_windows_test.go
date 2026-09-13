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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/w32"
)

// These tests cover the two pieces of the Windows accessibility wiring that can be exercised without a real window and
// a real assistive technology: what the WM_GETOBJECT handler does with the messages that are not UI Automation's, and
// the conversion that puts a node's window-local bounds where UI Automation expects to find them. Everything else needs
// UI Automation itself and is checked by hand with Narrator, NVDA and Accessibility Insights.

// TestW32HandleGetObjectIgnoresOtherObjectIDs is the zero-cost-when-inactive test for this platform. WM_GETOBJECT is
// sent for the Microsoft Active Accessibility objects of an ordinary window as a matter of course, and none of those
// requests may build a snapshot, create an adapter or turn accessibility support on: each must be reported as unhandled
// so that DefWindowProc answers it.
func TestW32HandleGetObjectIgnoresOtherObjectIDs(t *testing.T) {
	c := check.New(t)
	wasActive := IsAccessibilityActive()
	snapshots := axSnapshotCount
	w := &Window{}
	for i, objectID := range []int32{
		0,  // OBJID_WINDOW: the window frame
		-4, // OBJID_CLIENT: the client area, which is what MSAA clients ask for
		-5, // OBJID_MENU
		-8, // OBJID_CARET
		1,  // a child id from an MSAA client
		w32.UiaRootObjectId + 1,
		w32.UiaRootObjectId - 1,
	} {
		result, handled := w.w32HandleGetObject(0, w32.LPARAM(objectID))
		c.False(handled, "case %d: object id %d is not UI Automation's and must be left to DefWindowProc", i, objectID)
		c.Equal(uintptr(0), result, "case %d", i)
	}
	// A UI Automation request for a window with nothing to describe is also unhandled, and must not have turned anything
	// on while deciding that. The object id goes through a variable because a negative constant cannot be converted to
	// the unsigned type an lParam is.
	uiaObjectID := w32.UiaRootObjectId
	result, handled := w.w32HandleGetObject(0, w32.LPARAM(uiaObjectID))
	c.False(handled, "a window with no root panel has nothing to describe")
	c.Equal(uintptr(0), result)
	c.Nil(w.wnd.uia, "no adapter should have been created")
	c.Equal(wasActive, IsAccessibilityActive(), "accessibility support should not have been turned on")
	c.Equal(snapshots, axSnapshotCount, "no snapshot should have been built")
}

// TestW32AccessibilityGeometryFor verifies the geometry a window hands its adapter. The origin is the screen position
// of the client area in physical pixels, which is what ClientToScreen answers and what UI Automation works in, and the
// scale is how many of those pixels one logical unit covers.
func TestW32AccessibilityGeometryFor(t *testing.T) {
	c := check.New(t)
	geometry := w32AccessibilityGeometryFor(w32.POINT{X: 100, Y: 50}, geom.NewPoint(2, 2))
	c.Equal(geom.NewPoint(100, 50), geometry.Origin)
	c.Equal(geom.NewPoint(2, 2), geometry.Scale)
	left, top, width, height := geometry.ScreenRect(geom.NewRect(10, 20, 30, 40))
	c.Equal(float64(120), left)
	c.Equal(float64(90), top)
	c.Equal(float64(60), width)
	c.Equal(float64(80), height)
	// A window on a display to the left of the primary one has a negative origin, which must survive the conversion.
	geometry = w32AccessibilityGeometryFor(w32.POINT{X: -1920, Y: 0}, geom.NewPoint(1, 1))
	left, top, _, _ = geometry.ScreenRect(geom.NewRect(4, 8, 1, 1))
	c.Equal(float64(-1916), left)
	c.Equal(float64(8), top)
}
