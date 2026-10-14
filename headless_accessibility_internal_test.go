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

// TestAccessibilityOfPopupMenuWindowIsItsOwners verifies that a popup menu window's description and events are its
// owner's, since the menu it shows is described as part of the owner, and that a disposed popup, like any other
// disposed window, has no description at all rather than its former owner's.
func TestAccessibilityOfPopupMenuWindowIsItsOwners(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 200, 100))
	var chosen []string
	var m, sub *menu
	var popup *Window
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One", "Two")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
		}
	}))
	c.NotNil(popup)
	if popup == nil {
		return
	}
	tree := screen.AccessibilityTree(popup)
	c.NotNil(tree)
	ownerTree := screen.AccessibilityTree(owner)
	c.NotNil(ownerTree)
	if tree == nil || ownerTree == nil {
		return
	}
	c.Equal(ownerTree.Root, tree.Root)
	screen.AccessibilityEvents(popup)
	c.True(screen.Do(func() { sub.titleItem.showSubMenu() }))
	c.NotNil(screen.AccessibilityTree(popup))
	c.True(len(screen.AccessibilityEvents(popup)) != 0, "opening the sub-menu is reported through the popup")
	c.Equal(0, len(screen.AccessibilityEvents(owner)), "and those events were the owner's, asked for once")

	c.True(screen.Do(func() { m.closeMenuStack() }))
	c.False(popup.IsValid())
	c.Nil(screen.AccessibilityTree(popup), "a disposed popup has no description")
	c.Equal(0, len(screen.AccessibilityEvents(popup)))
}
