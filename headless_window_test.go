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
// normal windows below floating ones and floating ones below popup windows, so that a newer popup sits above an older
// one and every popup above a floating dialog. A tooltip window shares the popup tier with the menu windows. An
// application's own window of the menu or tooltip kind is not a popup and stays with the normal windows, as it does on
// the platforms.
func TestHeadlessRaiseKeepsTiers(t *testing.T) {
	c := check.New(t)
	s := &headlessState{}
	normal1 := &Window{title: "normal1"}
	normal2 := &Window{title: "normal2"}
	floating := &Window{title: "floating", floating: true}
	menu1 := &Window{title: "menu1", kind: WindowKindMenu, owner: normal1}
	menu2 := &Window{title: "menu2", kind: WindowKindMenu, owner: normal1}
	menuKind := &Window{title: "menuKind", kind: WindowKindMenu}
	tooltip := &Window{title: "tooltip", kind: WindowKindTooltip, owner: normal1}
	tooltipKind := &Window{title: "tooltipKind", kind: WindowKindTooltip}
	c.Equal(0, windowStackLevel(normal1))
	c.Equal(1, windowStackLevel(floating))
	c.Equal(2, windowStackLevel(menu1))
	c.Equal(0, windowStackLevel(menuKind))
	c.Equal(2, windowStackLevel(tooltip))
	c.Equal(0, windowStackLevel(tooltipKind))
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
	s.raise(tooltip)
	s.raise(tooltipKind)
	c.Equal([]*Window{normal2, normal1, menuKind, tooltipKind, floating, menu2, menu1, tooltip}, s.stack)
	s.raise(menu2)
	c.Equal([]*Window{normal2, normal1, menuKind, tooltipKind, floating, menu1, tooltip, menu2}, s.stack)
}

// TestHeadlessWindowAtSkipsTooltipWindows verifies that a tooltip window is never the window under the pointer, so that
// the pointer and presses pass through it to whatever is beneath, while a menu window above the same spot is hit as
// usual, as is an application's own window of the tooltip kind.
func TestHeadlessWindowAtSkipsTooltipWindows(t *testing.T) {
	c := check.New(t)
	s := &headlessState{}
	add := func(title string, kind WindowKind, owner *Window, rect geom.Rect) *Window {
		w := &Window{title: title, kind: kind, owner: owner, wnd: &apiWindow{}}
		w.wnd.hw = &headlessWindow{hs: s, w: w, rect: rect, visible: true}
		s.raise(w)
		return w
	}
	owner := add("owner", WindowKindNormal, nil, geom.NewRect(0, 0, 200, 200))
	tooltipKind := add("tooltipKind", WindowKindTooltip, nil, geom.NewRect(150, 150, 100, 100))
	menu := add("menu", WindowKindMenu, owner, geom.NewRect(100, 0, 50, 50))
	tooltip := add("tooltip", WindowKindTooltip, owner, geom.NewRect(0, 0, 160, 60))
	c.True(s.windowAt(geom.NewPoint(10, 10)) == owner, "the tooltip window is passed over")
	c.True(s.windowAt(geom.NewPoint(120, 10)) == menu, "a menu window is not")
	c.True(s.windowAt(geom.NewPoint(155, 155)) == tooltipKind, "nor is an application's window of the tooltip kind")
	c.Nil(s.windowAt(geom.NewPoint(300, 10)))
	c.True(s.stack[len(s.stack)-1] == tooltip, "the tooltip window is the topmost window all the same")
}

// TestHeadlessHiddenWindowForgetsTooltip verifies that hiding or minimizing the window the pointer is in takes the
// pointer out of it as the pointer leaving does, forgetting the tooltip asked for, so that the tooltip of the panel
// under the pointer shows again once the window is back.
func TestHeadlessHiddenWindowForgetsTooltip(t *testing.T) {
	for _, one := range []struct {
		away func(w *Window)
		back func(w *Window)
		name string
	}{
		{name: "hide", away: func(w *Window) { w.Hide() }, back: func(w *Window) { w.ToFront() }},
		{name: "minimize", away: func(w *Window) { w.Minimize() }, back: func(w *Window) { w.Minimize() }},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var panel *Panel
			screen, owner := startTooltipTest(t, func() []*Panel {
				panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
				return []*Panel{panel}
			})
			screen.MouseMove(screen.PanelCenter(panel), mod.None)
			c.NotNil(screen.TooltipWindow(owner))
			var lastIsNil bool
			c.True(screen.Do(func() {
				one.away(owner)
				lastIsNil = owner.lastTooltip == nil
			}))
			c.Nil(screen.TooltipWindow(owner))
			c.True(lastIsNil, "the tooltip asked for is forgotten")
			c.True(screen.Do(func() { one.back(owner) }))
			c.NotNil(screen.TooltipWindow(owner), "the tooltip of the panel under the pointer shows again")
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
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
