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
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
)

// TestSubMenuOriginConvertsTheCorner verifies that a sub-menu is placed from the converted far corner of its item rather
// than from the converted origin plus the item's logical extent, which on Windows at 150%, with the owner's content at
// raw (1000, 100), would put a 200-wide item's sub-menu at x 1200 rather than 1300 and a 24-high bar title's menu at
// y 124 rather than 136.
func TestSubMenuOriginConvertsTheCorner(t *testing.T) {
	c := check.New(t)
	toScreen := func(pt geom.Point) geom.Point { return geom.NewPoint(1000, 100).Add(pt.Mul(1.5)) }
	pr := geom.NewRect(0, 0, 200, 24)
	c.Equal(geom.NewPoint(1300, 100), subMenuOrigin(pr, false, toScreen))
	c.Equal(geom.NewPoint(1000, 136), subMenuOrigin(pr, true, toScreen))
	pr = geom.NewRect(10, 20, 200, 24)
	c.Equal(geom.NewPoint(1315, 130), subMenuOrigin(pr, false, toScreen))
	c.Equal(geom.NewPoint(1015, 166), subMenuOrigin(pr, true, toScreen))
}

// TestMenuItemMenuReturnsTrueNil verifies that Menu() on an item that has not been inserted into a menu returns a
// true nil interface rather than a typed-nil *menu, which would pass != nil checks and then crash on the first method
// call. This matches the behavior of the macOS implementation.
func TestMenuItemMenuReturnsTrueNil(t *testing.T) {
	c := check.New(t)
	var item MenuItem = &menuItem{}
	c.True(item.Menu() == nil)
}

// TestShowSubMenuOnlyInTheActiveWindow verifies that a menu bar item in a background window opens nothing, whether
// asked directly or on behalf of an assistive technology, which is refused and is not offered the actions that would
// open it, since the popup would be opened for the active window, and that the same item opens its menu below its title
// once its window is the active one.
func TestShowSubMenuOnlyInTheActiveWindow(t *testing.T) {
	c := check.New(t)
	const testMenuID = UserBaseID + 1
	installBar := func(wnd *Window) *menu {
		var m *menu
		DefaultMenuFactory().BarForWindow(wnd, func(bar Menu) {
			f := bar.Factory()
			sub := f.NewMenu(testMenuID, "Test", nil)
			sub.InsertItem(-1, f.NewItem(testMenuID+1, "Do It", KeyBinding{}, nil, nil))
			bar.InsertMenu(-1, sub)
			if inWindow, ok := sub.(*menu); ok {
				m = inWindow
			}
		})
		return m
	}
	var front, back *Window
	var backMenu *menu
	screen := startHeadlessTest(t, HeadlessConfig{Width: 500, Height: 400},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(10, 10, 300, 200))
			front = newHeadlessTestWindow(t, "front", geom.NewRect(50, 50, 300, 200))
			if back == nil || front == nil {
				return
			}
			backMenu = installBar(back)
			installBar(front)
			front.ToFront()
		}))
	c.NotNil(back)
	c.NotNil(front)
	c.NotNil(backMenu)
	if back == nil || front == nil || backMenu == nil {
		return
	}
	screen.Sync() // the bars have to have been laid out before there is a title to open from

	var active, openedIn *Window
	var frontOpen, backOpen int
	look := func() {
		openedIn = nil
		if backMenu.popupPanel != nil {
			openedIn = backMenu.popupPanel.Window().menuOwner()
		}
		frontOpen = len(front.root.openMenuPanels)
		backOpen = len(back.root.openMenuPanels)
	}
	c.True(screen.Do(func() {
		active = ActiveWindow()
		backMenu.titleItem.showSubMenu()
		look()
	}))
	c.True(active == front, "the test needs the item's window in the background")
	c.True(openedIn == nil, "a bar item in a background window opens nothing")
	c.Equal(0, frontOpen, "and nothing lands in the active window")
	c.Equal(0, backOpen)

	// What is refused is not advertised either, so an assistive technology is never offered an expansion that would
	// open nothing. See axSnapshot.narrowMenuActions.
	screen.AccessibilityTree(back)
	backgrounded := screen.AccessibilityNodeFor(backMenu.titleItem.panel)
	c.NotNil(backgrounded)
	if backgrounded == nil {
		return
	}
	c.True(backgrounded.Expandable, "the item still has a menu; what it has lost is anywhere to show it")
	c.False(backgrounded.Actions.Has(accessibility.Expand), "an item that would refuse to expand must not offer to")
	c.False(backgrounded.Actions.Has(accessibility.Press), "nor the press that is the same thing by another name")
	var expanded, pressed bool
	c.True(screen.Do(func() {
		item := backMenu.titleItem.panel
		expanded = item.axDispatchAction(accessibility.ActionRequest{Action: accessibility.Expand}, false)
		pressed = item.axDispatchAction(accessibility.ActionRequest{Action: accessibility.Press}, false)
		look()
	}))
	c.False(expanded, "expanding a bar item in a background window is refused, in case what was published offered it")
	c.False(pressed, "and so is pressing it")
	c.True(openedIn == nil, "an assistive technology's request on a bar item in a background window opens nothing")
	c.Equal(0, frontOpen)
	c.Equal(0, backOpen)

	var at, want geom.Point
	c.True(screen.Do(func() { back.ToFront() }))
	screen.AccessibilityTree(back)
	restored := screen.AccessibilityNodeFor(backMenu.titleItem.panel)
	c.NotNil(restored)
	if restored == nil {
		return
	}
	c.True(restored.Actions.Has(accessibility.Expand), "the active window's bar item offers its menu again")
	c.True(restored.Actions.Has(accessibility.Press))
	c.True(screen.Do(func() {
		active = ActiveWindow()
		backMenu.titleItem.showSubMenu()
		look()
		if backMenu.popupPanel != nil {
			at = backMenu.popupPanel.Window().ContentRect().Point
		}
		title := backMenu.titleItem.panel
		pr := title.RectToRoot(title.ContentRect(true))
		want = back.apiPointToScreen(geom.NewPoint(pr.X, pr.Bottom()))
	}))
	c.True(active == back, "the test needs the item's window active")
	c.True(openedIn == back, "the item opens its menu for its own window once that is the active one")
	c.Equal(1, backOpen)
	c.Equal(0, frontOpen)
	c.Equal(want, at, "placed below its title")
}

// TestDisabledItemLeavesTheMenuOpen verifies that clicking a disabled item, or pressing Return or Space on it, runs
// nothing and leaves the menu open, as the platforms' own menus do, while choosing an enabled item still closes it.
func TestDisabledItemLeavesTheMenuOpen(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m *menu
	var opened bool
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = f.newMenu(UserBaseID, "Test", nil)
		record := func(mi MenuItem) { chosen = append(chosen, mi.Title()) }
		m.InsertItem(-1, f.NewItem(UserBaseID+1, "Off", KeyBinding{}, func(MenuItem) bool { return false }, record))
		m.InsertItem(-1, f.NewItem(UserBaseID+2, "On", KeyBinding{}, nil, record))
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		opened = m.popupPanel != nil
	}))
	c.True(opened)
	if !opened {
		return
	}
	state := func() (open int, picked []string) {
		c.True(screen.Do(func() {
			open = len(owner.root.openMenuPanels)
			picked = append(picked, chosen...)
		}))
		return open, picked
	}
	screen.Click(screen.PanelCenter(m.items[0].panel))
	open, picked := state()
	c.Equal(1, open, "a click on a disabled item leaves the menu open")
	c.Equal(0, len(picked))
	screen.KeyPress(KeyDown, 0)
	for _, key := range []KeyCode{KeyReturn, KeySpace} {
		screen.KeyPress(key, 0)
		open, picked = state()
		c.Equal(1, open, "%v on a disabled item leaves the menu open", key)
		c.Equal(0, len(picked))
	}
	screen.KeyPress(KeyDown, 0)
	screen.KeyPress(KeyReturn, 0)
	open, picked = state()
	c.Equal(0, open, "choosing an enabled item closes the menu")
	c.Equal([]string{"On"}, picked)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHighlightClearsARemovedItemStillShown verifies that moving the highlight takes it off an item removed from the
// menu while its popup still shows it, so that the popup never shows two items highlighted and the item the highlight
// moved to is the one reported as the focus.
func TestHighlightClearsARemovedItemStillShown(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m *menu
	var removed *menuItem
	var last *Panel
	c.True(screen.Do(func() {
		m = newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two", "Three")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			removed = m.items[1]
			last = m.items[2].panel
			m.RemoveItem(1)
		}
	}))
	c.NotNil(removed)
	if removed == nil {
		return
	}
	screen.MouseMove(screen.PanelCenter(removed.panel), mod.None)
	screen.AccessibilityTree(owner)
	node := screen.AccessibilityNodeFor(last)
	c.NotNil(node)
	if node == nil {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Focus}))
	var highlighted []string
	c.True(screen.Do(func() {
		for _, item := range m.popupPanel.items {
			if item.over {
				highlighted = append(highlighted, item.title)
			}
		}
	}))
	c.Equal([]string{"Three"}, highlighted, "the removed item the pointer highlighted loses the highlight")
	tree := screen.AccessibilityTree(owner)
	c.NotNil(tree)
	if tree != nil {
		c.Equal(node.ID, tree.Focus, "the item the highlight moved to is the focus")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
