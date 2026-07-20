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
	"github.com/richardwilkes/unison/enums/mod"
)

// newTestMenuPanel creates a bare open menu panel covering the given frame, suitable for inserting into a rootPanel
// without requiring a live window, fonts, or a menu factory.
func newTestMenuPanel(x, y, width, height float32) *menuPanel {
	m := &menu{}
	p := &menuPanel{
		menu:      m,
		itemIndex: -1,
	}
	p.Self = p
	p.KeyDownCallback = func(_ KeyCode, _ mod.Modifiers, _ bool) bool { return false }
	p.SetFrameRect(geom.NewRect(x, y, width, height))
	m.popupPanel = p
	return p
}

// TestPopupClosesOnOutsideClickWithoutMenuBar verifies that a click outside an open popup menu tears the menu down
// even when the window has no in-window menu bar (e.g. a dialog), rather than leaving it stuck open.
func TestPopupClosesOnOutsideClickWithoutMenuBar(t *testing.T) {
	c := check.New(t)
	root := newRootPanel(nil)
	c.Nil(root.menuBar)
	mp := newTestMenuPanel(10, 10, 100, 100)
	root.insertMenu(mp)
	c.Equal(1, len(root.openMenuPanels))
	c.False(root.preMouseDown(nil, geom.NewPoint(200, 200)))
	c.Equal(0, len(root.openMenuPanels))
	c.Nil(mp.menu.popupPanel)
	c.Nil(mp.Parent())
}

// TestPopupClickInsideClosesOnlyNewerMenusWithoutMenuBar verifies z-order attribution without a menu bar: a click
// inside an older popup closes only the popups opened after it, and a click inside the newest popup closes nothing.
func TestPopupClickInsideClosesOnlyNewerMenusWithoutMenuBar(t *testing.T) {
	c := check.New(t)
	root := newRootPanel(nil)
	older := newTestMenuPanel(10, 10, 100, 100)
	newer := newTestMenuPanel(90, 10, 100, 100)
	root.insertMenu(older)
	root.insertMenu(newer)

	// A click inside the newest popup leaves the whole stack alone.
	c.False(root.preMouseDown(nil, geom.NewPoint(150, 50)))
	c.Equal(2, len(root.openMenuPanels))

	// A click in the non-overlapping part of the older popup closes only the newer one.
	c.False(root.preMouseDown(nil, geom.NewPoint(20, 50)))
	c.Equal(1, len(root.openMenuPanels))
	c.Equal(older, root.openMenuPanels[0])
	c.Nil(newer.menu.popupPanel)
	c.NotNil(older.menu.popupPanel)
}

// TestPopupClosesOnWindowMoveWithoutMenuBar verifies that moving a window without an in-window menu bar still closes
// any open popup menus.
func TestPopupClosesOnWindowMoveWithoutMenuBar(t *testing.T) {
	c := check.New(t)
	root := newRootPanel(nil)
	mp := newTestMenuPanel(10, 10, 100, 100)
	root.insertMenu(mp)
	root.preMoved(nil)
	c.Equal(0, len(root.openMenuPanels))
	c.Nil(mp.menu.popupPanel)
}

// TestKeyEventsSwallowedWhileMenuOpenWithoutMenuBar verifies that key events are consumed while a popup menu is open
// in a window without an in-window menu bar, instead of leaking through to the content underneath.
func TestKeyEventsSwallowedWhileMenuOpenWithoutMenuBar(t *testing.T) {
	c := check.New(t)
	root := newRootPanel(nil)
	c.False(root.preKeyDown(nil, KeyA, 0, false))
	c.False(root.preRuneTyped(nil, 'a'))
	c.False(root.preKeyUp(nil, KeyA, 0))
	root.insertMenu(newTestMenuPanel(10, 10, 100, 100))
	c.True(root.preKeyDown(nil, KeyA, 0, false))
	c.True(root.preRuneTyped(nil, 'a'))
	c.True(root.preKeyUp(nil, KeyA, 0))
}

// TestCloseMenuStackStoppingAt verifies that closing the open menu stack stops when it reaches the given menu,
// leaving that menu and any older ones open.
func TestCloseMenuStackStoppingAt(t *testing.T) {
	c := check.New(t)
	root := newRootPanel(nil)
	first := newTestMenuPanel(0, 0, 50, 50)
	second := newTestMenuPanel(40, 0, 50, 50)
	third := newTestMenuPanel(80, 0, 50, 50)
	root.insertMenu(first)
	root.insertMenu(second)
	root.insertMenu(third)
	root.closeMenuStackStoppingAt(second.menu)
	c.Equal([]*menuPanel{first, second}, root.openMenuPanels)
	root.closeMenuStackStoppingAt(nil)
	c.Equal(0, len(root.openMenuPanels))
}
