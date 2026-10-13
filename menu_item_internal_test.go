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
)

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
// open it, since the popup is inserted into the active window and would otherwise land there at coordinates taken from
// the item's own window, and that the same item opens its menu below its title once its window is the active one.
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
			openedIn = backMenu.popupPanel.Window()
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

	var fr geom.Rect
	var want geom.Point
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
			fr = backMenu.popupPanel.FrameRect()
		}
		title := backMenu.titleItem.panel
		pr := title.RectToRoot(title.ContentRect(true))
		want = geom.NewPoint(pr.X, pr.Bottom())
	}))
	c.True(active == back, "the test needs the item's window active")
	c.True(openedIn == back, "the item opens its menu in its own window once that is the active one")
	c.Equal(1, backOpen)
	c.Equal(0, frontOpen)
	c.Equal(want, fr.Point, "placed below its title")
}
