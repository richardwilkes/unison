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

// TestHeadlessRaiseKeepsTiers verifies that raising a window puts it at the top of its own tier of the z-order, with
// normal windows below floating ones and floating ones below popup menu windows, so that a newer popup sits above an
// older one and every popup above a floating dialog. An application's own window of the menu kind is not a popup and
// stays with the normal windows, as it does on the platforms.
func TestHeadlessRaiseKeepsTiers(t *testing.T) {
	c := check.New(t)
	s := &headlessState{}
	normal1 := &Window{title: "normal1"}
	normal2 := &Window{title: "normal2"}
	floating := &Window{title: "floating", floating: true}
	menu1 := &Window{title: "menu1", kind: WindowKindMenu, owner: normal1}
	menu2 := &Window{title: "menu2", kind: WindowKindMenu, owner: normal1}
	menuKind := &Window{title: "menuKind", kind: WindowKindMenu}
	c.Equal(0, windowStackLevel(normal1))
	c.Equal(1, windowStackLevel(floating))
	c.Equal(2, windowStackLevel(menu1))
	c.Equal(0, windowStackLevel(menuKind))
	s.raise(menu1)
	s.raise(floating)
	s.raise(normal1)
	c.Equal([]*Window{normal1, floating, menu1}, s.stack)
	s.raise(menu2)
	s.raise(normal2)
	c.Equal([]*Window{normal1, normal2, floating, menu1, menu2}, s.stack)
	s.raise(normal1)
	s.raise(menu1)
	s.raise(floating)
	c.Equal([]*Window{normal2, normal1, floating, menu2, menu1}, s.stack)
	s.raise(menuKind)
	c.Equal([]*Window{normal2, normal1, menuKind, floating, menu2, menu1}, s.stack)
}

// TestHeadlessPressOutsideMenuStackClosesIt verifies that a press outside an open menu stack, on no window or on
// another of the application's windows, here a transient one that takes no focus, closes the stack and is then
// delivered, as it is on macOS.
func TestHeadlessPressOutsideMenuStackClosesIt(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	openMenu := func() *Window {
		var popup *Window
		c.True(screen.Do(func() {
			var chosen []string
			m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two")
			m.Popup(geom.NewRect(20, 20, 0, 0), -1)
			if m.popupPanel != nil {
				popup = m.popupPanel.Window()
			}
		}))
		c.NotNil(popup)
		return popup
	}
	checkClosed := func(popup *Window) {
		t.Helper()
		var open int
		var popupValid bool
		c.True(screen.Do(func() {
			open = len(owner.root.openMenuPanels)
			popupValid = popup.IsValid()
		}))
		c.Equal(0, open)
		c.False(popupValid)
		c.True(screen.FocusedWindow() == owner)
	}

	popup := openMenu()
	screen.Click(geom.NewPoint(450, 350))
	checkClosed(popup)

	var openAtPress []int
	c.True(screen.Do(func() {
		other, err := NewWindow("other", TransientWindowOption())
		if err != nil {
			t.Errorf("unable to create transient window: %v", err)
			return
		}
		other.SetContentRect(geom.NewRect(370, 50, 100, 100))
		other.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			openAtPress = append(openAtPress, len(owner.root.openMenuPanels))
			return true
		}
		other.Show()
	}))
	popup = openMenu()
	screen.Click(geom.NewPoint(420, 100))
	checkClosed(popup)
	var pressed []int
	c.True(screen.Do(func() { pressed = openAtPress }))
	c.Equal([]int{0}, pressed, "the press reaches the other window once the menus have closed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
