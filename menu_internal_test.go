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

// TestPopupWithoutActiveWindowDoesNotPanic verifies that Menu.Popup degrades to a no-op instead of panicking when
// there is no active window, since createPopup silently declines to build the popup panel in that state.
func TestPopupWithoutActiveWindowDoesNotPanic(t *testing.T) {
	c := check.New(t)
	c.Nil(ActiveWindow())
	f := &inWindowMenuFactory{}
	m := f.newMenu(1, "Test", nil)
	m.InsertItem(-1, f.NewItem(2, "One", KeyBinding{}, nil, nil))
	m.InsertItem(-1, f.NewItem(3, "Two", KeyBinding{}, nil, nil))
	m.Popup(geom.NewRect(10, 10, 100, 20), 0)
	c.Nil(m.popupPanel)
	m.Popup(geom.NewRect(10, 10, 100, 20), -1)
	c.Nil(m.popupPanel)
}

// TestShowSubMenuWithoutActiveWindowDoesNotPanic verifies that opening a sub-menu degrades to a no-op instead of
// panicking when there is no active window to host the popup panel.
func TestShowSubMenuWithoutActiveWindowDoesNotPanic(t *testing.T) {
	c := check.New(t)
	c.Nil(ActiveWindow())
	f := &inWindowMenuFactory{}
	m := f.newMenu(1, "Test", nil)
	sub := f.newMenu(2, "Sub", nil)
	sub.InsertItem(-1, f.NewItem(3, "One", KeyBinding{}, nil, nil))
	m.InsertMenu(-1, sub)
	mi, ok := m.ItemAtIndex(0).(*menuItem)
	c.True(ok)
	mi.showSubMenu()
	c.Nil(sub.popupPanel)
}

// TestSetKeyIndexWithoutPopupDoesNotPanic verifies that keyboard navigation into a menu whose popup panel was never
// created (no active window) is a no-op instead of a panic.
func TestSetKeyIndexWithoutPopupDoesNotPanic(t *testing.T) {
	c := check.New(t)
	f := &inWindowMenuFactory{}
	m := f.newMenu(1, "Test", nil)
	m.InsertItem(-1, f.NewItem(2, "One", KeyBinding{}, nil, nil))
	c.Nil(m.popupPanel)
	m.setKeyIndex(0)
	c.Nil(m.popupPanel)
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
