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
)

// newCursorTestWindow returns a minimal Window suitable for exercising cursor updates without a live windowing system.
// The window deliberately reports itself as invalid so MouseLocation returns the origin instead of querying the
// platform, and updateCursor skips the platform cursor-image call after recording the new cursor.
func newCursorTestWindow() *Window {
	w := &Window{}
	w.root = newRootPanel(w)
	return w
}

func TestUpdateCursorConvertsRootPointForCallbackOwner(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0, 20, 200, 200))
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(30, 40, 100, 100))
	content.AddChild(child)
	var got []geom.Point
	child.UpdateCursorCallback = func(where geom.Point) *Cursor {
		got = append(got, where)
		return &Cursor{}
	}
	// Root point (50,70) is content-local (50,50) and child-local (20,10).
	w.updateCursor(child.AsPanel(), geom.NewPoint(50, 70))
	c.Equal([]geom.Point{geom.NewPoint(20, 10)}, got)
}

func TestUpdateCursorWalksUpToAncestorWithCallback(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0, 20, 200, 200))
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(30, 40, 100, 100))
	content.AddChild(child)
	var got []geom.Point
	content.UpdateCursorCallback = func(where geom.Point) *Cursor {
		got = append(got, where)
		return &Cursor{}
	}
	// The leaf has no callback, so updateCursor must walk up to the content panel and convert the root point into that
	// panel's coordinates, not the leaf's.
	w.updateCursor(child.AsPanel(), geom.NewPoint(50, 70))
	c.Equal([]geom.Point{geom.NewPoint(50, 50)}, got)
}

func TestUpdateCursorNowDeliversTargetLocalPointExactlyOnce(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	// The invalid test window reports the mouse at the origin, so the target panel is given a negative origin — as a
	// scrolled panel would have — to make its offset from root non-zero. A double conversion would report (50,30)
	// instead of the correct (25,15).
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(-25, -15, 100, 100))
	content.AddChild(child)
	var got []geom.Point
	child.UpdateCursorCallback = func(where geom.Point) *Cursor {
		got = append(got, where)
		return &Cursor{}
	}
	w.UpdateCursorNow()
	c.Equal([]geom.Point{geom.NewPoint(25, 15)}, got)
}
