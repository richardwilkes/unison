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
	"strconv"
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

// newTestPopupWindow hand-builds a popup menu window of owner whose content is panel, for the tests that run without a
// session.
func newTestPopupWindow(owner *Window, panel *menuPanel) *Window {
	w := &Window{owner: owner, kind: WindowKindMenu}
	w.root = newRootPanel(w)
	w.root.setContent(panel)
	return w
}

// TestPopupClickInsideClosesOnlyNewerMenus verifies that a press in a popup menu window closes only the menus opened
// after the one it shows, and that a press reaching the owner, which landed outside every menu, closes them all.
func TestPopupClickInsideClosesOnlyNewerMenus(t *testing.T) {
	c := check.New(t)
	owner := &Window{}
	owner.root = newRootPanel(owner)
	older := newTestMenuPanel(0, 0, 100, 100)
	newer := newTestMenuPanel(0, 0, 100, 100)
	olderWnd := newTestPopupWindow(owner, older)
	newerWnd := newTestPopupWindow(owner, newer)
	owner.root.insertMenu(older)
	owner.root.insertMenu(newer)

	// A press in the newest popup leaves the whole stack alone.
	c.False(newerWnd.root.preMouseDown(newerWnd, geom.NewPoint(50, 50)))
	c.Equal(2, len(owner.root.openMenuPanels))

	// A press in the older popup closes only the newer one.
	c.False(olderWnd.root.preMouseDown(olderWnd, geom.NewPoint(50, 50)))
	c.Equal([]*menuPanel{older}, owner.root.openMenuPanels)
	c.Nil(newer.menu.popupPanel)
	c.NotNil(older.menu.popupPanel)
	c.Nil(newer.Parent(), "a hand-built popup window is not disposed of, so its panel is merely detached")

	// A press that reached the owner closes everything.
	c.False(owner.root.preMouseDown(owner, geom.NewPoint(150, 150)))
	c.Equal(0, len(owner.root.openMenuPanels))
	c.Nil(older.menu.popupPanel)
}

// TestPopupClosesOnWindowMoveWithoutMenuBar verifies that moving a window without an in-window menu bar still closes
// any open popup menus, while moving the popup menu window itself, which placing it does, leaves them open.
func TestPopupClosesOnWindowMoveWithoutMenuBar(t *testing.T) {
	c := check.New(t)
	owner := &Window{}
	owner.root = newRootPanel(owner)
	mp := newTestMenuPanel(10, 10, 100, 100)
	popup := newTestPopupWindow(owner, mp)
	owner.root.insertMenu(mp)
	popup.root.preMoved(popup)
	c.Equal(1, len(owner.root.openMenuPanels))
	owner.root.preMoved(owner)
	c.Equal(0, len(owner.root.openMenuPanels))
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

// startMenuTest starts a session with a screen of the given size holding one window at rect, which is shown and
// active, and returns the screen and that window.
func startMenuTest(t *testing.T, screenSize geom.Size, rect geom.Rect) (*HeadlessScreen, *Window) {
	t.Helper()
	var owner *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: screenSize.Width, Height: screenSize.Height},
		StartupFinishedCallback(func() {
			if owner = newHeadlessTestWindow(t, "owner", rect); owner != nil {
				owner.ToFront()
			}
		}))
	if owner == nil {
		t.FailNow()
	}
	return screen, owner
}

// newTestMenu builds an in-window menu holding an item for each title, each of which appends its title to chosen when
// it is chosen.
func newTestMenu(f *inWindowMenuFactory, id int, chosen *[]string, titles ...string) *menu {
	m := f.newMenu(id, "Test", nil)
	for i, title := range titles {
		m.InsertItem(-1, f.NewItem(id+1+i, title, KeyBinding{}, nil, func(mi MenuItem) {
			*chosen = append(*chosen, mi.Title())
		}))
	}
	return m
}

// menuItemTitles returns the titles of n items, numbered from one.
func menuItemTitles(n int) []string {
	titles := make([]string, n)
	for i := range titles {
		titles[i] = "Item " + strconv.Itoa(i+1)
	}
	return titles
}

// TestEnsureOnDisplayKeepsMenuBelowTop verifies that a menu too tall for the space under the menu bar is shrunk to fit
// there rather than moved up over the bar, while a menu with no such limit may move up to use the whole display.
func TestEnsureOnDisplayKeepsMenuBelowTop(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(400, 300), geom.NewRect(0, 0, 400, 300))
	var rects []geom.Rect
	var created bool
	c.True(screen.Do(func() {
		var chosen []string
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One")
		if created = m.createPopup(owner); !created {
			return
		}
		place := func(where geom.Rect, fit menuFit) {
			m.ensureOnDisplay(where, fit, owner, -1)
			rects = append(rects, m.popupPanel.Window().ContentRect())
		}
		bar := menuFit{bar: true, barTop: 10, barBottom: 30}
		// Below the bar: no room to grow, so it shrinks and stays put.
		place(geom.NewRect(10, 30, 100, 500), bar)
		// A menu that fits under the bar is left where it was asked to go.
		place(geom.NewRect(10, 30, 100, 200), bar)
		// One that nearly fits is moved up to fit.
		place(geom.NewRect(10, 60, 100, 260), bar)
		// One that would have to move above the bar to fit is shrunk instead, and moved up only as far as the bar.
		place(geom.NewRect(10, 60, 100, 280), bar)
		// Without a limit, the same menu may cover the whole display height.
		place(geom.NewRect(10, 30, 100, 500), menuFit{})
		// A sub-menu with no limit still slides up to fit.
		place(geom.NewRect(10, 100, 100, 250), menuFit{})
		// One that overlaps no display is placed on its anchor's.
		place(geom.NewRect(500, 400, 100, 50), menuFit{})
		m.closeMenuStack()
	}))
	c.True(created)
	c.Equal([]geom.Rect{
		geom.NewRect(10, 30, 100, 270),
		geom.NewRect(10, 30, 100, 200),
		geom.NewRect(10, 40, 100, 260),
		geom.NewRect(10, 30, 100, 270),
		geom.NewRect(10, 0, 100, 300),
		geom.NewRect(10, 50, 100, 250),
		geom.NewRect(300, 250, 100, 50),
	}, rects)
}

// TestFitMenuRectInDisplaySpace verifies the fitting in the raw pixels of a Windows display, which is the space
// ensureOnDisplay converts into: on a 2880x1620 display at 150%, a 200x150 logical drop-down at raw (2400, 1200)
// covers 300x225 raw pixels and fits where it is, and on a 3840x2160 display at 200% a menu-bar menu below a bar at raw
// y 1100 keeps its height, or is shrunk to the space left rather than collapsed.
func TestFitMenuRectInDisplaySpace(t *testing.T) {
	c := check.New(t)
	usable := geom.NewRect(0, 0, 2880, 1620)
	c.Equal(geom.NewRect(2400, 1200, 300, 225), fitMenuRect(geom.NewRect(2400, 1200, 300, 225), usable, menuFit{}))
	c.Equal(geom.NewRect(2580, 1395, 300, 225), fitMenuRect(geom.NewRect(2700, 1500, 300, 225), usable, menuFit{}))
	usable = geom.NewRect(0, 0, 3840, 2160)
	bar := menuFit{bar: true, barTop: 1050, barBottom: 1100, minHeight: 100}
	c.Equal(geom.NewRect(100, 1100, 400, 800), fitMenuRect(geom.NewRect(100, 1100, 400, 800), usable, bar))
	c.Equal(geom.NewRect(100, 1100, 400, 1060), fitMenuRect(geom.NewRect(100, 1100, 400, 2000), usable, bar))
}

// TestPopupWindowStacksAboveItsOwner verifies that a menu opens in a popup window of its own, placed from the owner's
// coordinates and stacked above it, which never becomes the active window or takes the focus.
func TestPopupWindowStacksAboveItsOwner(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var m *menu
	var chosen []string
	var popup, active, ownedBy *Window
	var rect geom.Rect
	var center geom.Point
	c.True(screen.Do(func() {
		m = newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two")
		m.Popup(geom.NewRect(20, 30, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
			rect = popup.ContentRect()
			ownedBy = popup.owner
			center = screen.PanelCenter(m.items[0].panel)
		}
		active = ActiveWindow()
	}))
	c.NotNil(popup)
	if popup == nil {
		return
	}
	c.True(popup != owner)
	c.True(ownedBy == owner)
	c.Equal(geom.NewPoint(70, 80), rect.Point, "placed at the point asked for, taken from the owner's coordinates")
	c.True(active == owner, "the popup never becomes the active window")
	c.True(screen.FocusedWindow() == owner, "nor takes the focus")
	c.True(screen.WindowAt(center) == popup, "and is stacked above its owner")
	c.NotNil(screen.CaptureWindow(popup), "and is drawn")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPressInOwnerClosesEveryMenu verifies that a click in the owner, outside every menu, closes the menus and disposes
// of their popup windows.
func TestPressInOwnerClosesEveryMenu(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var m *menu
	var chosen []string
	var popup *Window
	c.True(screen.Do(func() {
		m = newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
		}
	}))
	c.NotNil(popup)
	screen.Click(geom.NewPoint(340, 240))
	var open int
	var popupValid, menuOpen bool
	var picked []string
	c.True(screen.Do(func() {
		open = len(owner.root.openMenuPanels)
		popupValid = popup.IsValid()
		menuOpen = m.popupPanel != nil
		picked = slices.Clone(chosen)
	}))
	c.Equal(0, open)
	c.False(popupValid, "the popup window is disposed of")
	c.False(menuOpen)
	c.Equal(0, len(picked))
	c.True(screen.FocusedWindow() == owner)
}

// TestEscapeClosesTheMenu verifies that Escape, which the owner receives, closes a menu open in a popup window.
func TestEscapeClosesTheMenu(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var popup *Window
	c.True(screen.Do(func() {
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two")
		m.Popup(geom.NewRect(20, 20, 0, 0), 0)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
		}
	}))
	c.NotNil(popup)
	screen.KeyPress(KeyEscape, 0)
	var open int
	var popupValid bool
	var picked []string
	c.True(screen.Do(func() {
		open = len(owner.root.openMenuPanels)
		popupValid = popup.IsValid()
		picked = slices.Clone(chosen)
	}))
	c.Equal(0, open)
	c.False(popupValid)
	c.Equal(0, len(picked))
}

// TestMenuKeyboardNavigationRoutesThroughTheOwner verifies that the arrow keys and Return, which arrive at the owner
// since the popup never holds the focus, drive the menu in the popup window and choose from it.
func TestMenuKeyboardNavigationRoutesThroughTheOwner(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var popup *Window
	c.True(screen.Do(func() {
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two", "Three")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
		}
	}))
	c.NotNil(popup)
	screen.KeyPress(KeyDown, 0)
	screen.KeyPress(KeyDown, 0)
	c.True(screen.FocusedWindow() == owner)
	screen.KeyPress(KeyReturn, 0)
	var open int
	var popupValid bool
	var picked []string
	c.True(screen.Do(func() {
		open = len(owner.root.openMenuPanels)
		popupValid = popup.IsValid()
		picked = slices.Clone(chosen)
	}))
	c.Equal([]string{"Two"}, picked)
	c.Equal(0, open)
	c.False(popupValid)
}

// TestSubMenuOpensBesideItsParentPopup verifies that a sub-menu opens in a popup window of its own beside the item in
// its parent's popup, stacked above it and belonging to the same owner, and that clicks in the popups never move the
// focus or the activation away from the owner.
func TestSubMenuOpensBesideItsParentPopup(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m, sub *menu
	var opened bool
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep", "Deeper")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		opened = m.popupPanel != nil
	}))
	c.True(opened)
	if !opened {
		return
	}
	screen.Click(screen.PanelCenter(sub.titleItem.panel))
	var parent, child, active, childOwner *Window
	var want geom.Point
	var at geom.Rect
	var stack int
	c.True(screen.Do(func() {
		parent = m.popupPanel.Window()
		if sub.popupPanel != nil {
			child = sub.popupPanel.Window()
			at = child.ContentRect()
			childOwner = child.owner
		}
		item := sub.titleItem.panel
		pr := item.RectToRoot(item.ContentRect(true))
		want = parent.apiPointToScreen(geom.NewPoint(pr.Right(), pr.Y))
		stack = len(owner.root.openMenuPanels)
		active = ActiveWindow()
	}))
	c.NotNil(child)
	if child == nil {
		return
	}
	c.Equal(2, stack)
	c.True(childOwner == owner, "a sub-menu's popup belongs to the top-level window")
	c.Equal(want, at.Point, "placed beside its item")
	c.True(screen.WindowAt(at.Center()) == child, "and stacked above its parent")
	c.True(active == owner)
	c.True(screen.FocusedWindow() == owner)

	screen.Click(screen.PanelCenter(sub.items[1].panel))
	var parentValid, childValid bool
	var picked []string
	c.True(screen.Do(func() {
		stack = len(owner.root.openMenuPanels)
		active = ActiveWindow()
		parentValid = parent.IsValid()
		childValid = child.IsValid()
		picked = slices.Clone(chosen)
	}))
	c.Equal([]string{"Deeper"}, picked)
	c.Equal(0, stack)
	c.False(parentValid)
	c.False(childValid)
	c.True(active == owner)
	c.True(screen.FocusedWindow() == owner)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLongMenuShrinksToTheDisplayAndScrolls verifies that a menu taller than the display, though not limited by its
// small owner, is shrunk to the display and scrolled by the arrow keys.
func TestLongMenuShrinksToTheDisplayAndScrolls(t *testing.T) {
	c := check.New(t)
	screen, _ := startMenuTest(t, geom.NewSize(400, 300), geom.NewRect(50, 50, 200, 100))
	var chosen []string
	var m *menu
	var rect geom.Rect
	itemBounds := func() geom.Rect {
		last := m.items[len(m.items)-1].panel
		return last.RectToRoot(last.ContentRect(true))
	}
	var before, after geom.Rect
	var opened bool
	c.True(screen.Do(func() {
		m = newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, menuItemTitles(40)...)
		m.Popup(geom.NewRect(10, 10, 0, 0), -1)
		if opened = m.popupPanel != nil; opened {
			rect = m.popupPanel.Window().ContentRect()
			before = itemBounds()
		}
	}))
	c.True(opened)
	if !opened {
		return
	}
	c.Equal(float32(60), rect.X)
	c.Equal(float32(0), rect.Y, "moved up to the top of the display")
	c.Equal(float32(300), rect.Height, "and shrunk to its height, well beyond the owner's")
	c.True(before.Y >= rect.Height, "the last item starts out of sight")
	for range 40 {
		screen.KeyPress(KeyDown, 0)
	}
	c.True(screen.Do(func() { after = itemBounds() }))
	c.True(after.Y >= 0 && after.Bottom() <= rect.Height, "and is scrolled into view by the arrow keys: %v", after)
}

// TestBarMenuStaysBelowTheBar verifies that a menu opened from the menu bar of a window low on the screen, with room
// for a few of its items beneath the bar, is placed below the bar, in screen coordinates, and shrinks to the space left
// beneath it rather than covering the bar.
func TestBarMenuStaysBelowTheBar(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 250, 300, 80))
	var chosen []string
	var m *menu
	c.True(screen.Do(func() {
		DefaultMenuFactory().BarForWindow(owner, func(bar Menu) {
			if f, ok := bar.Factory().(*inWindowMenuFactory); ok {
				m = newTestMenu(f, UserBaseID, &chosen, menuItemTitles(40)...)
				bar.InsertMenu(-1, m)
			}
		})
	}))
	c.NotNil(m, "the test needs an in-window menu bar")
	if m == nil {
		return
	}
	screen.Sync() // the bar has to have been laid out before there is a title to open from
	screen.Click(screen.PanelCenter(m.titleItem.panel))
	var rect geom.Rect
	var barBottom float32
	var opened bool
	c.True(screen.Do(func() {
		if opened = m.popupPanel != nil; opened {
			rect = m.popupPanel.Window().ContentRect()
		}
		title := m.titleItem.panel
		barBottom = owner.apiPointToScreen(geom.NewPoint(0, title.RectToRoot(title.ContentRect(true)).Bottom())).Y
	}))
	c.True(opened)
	c.True(barBottom > 250 && barBottom < 300, "the test needs the bar low on the screen: %v", barBottom)
	c.Equal(barBottom, rect.Y, "the menu starts at the bottom of the bar")
	c.Equal(400-barBottom, rect.Height, "and shrinks to the space left beneath it")
}

// TestOwnerMoveClosesTheMenus verifies that moving the owner closes its menus.
func TestOwnerMoveClosesTheMenus(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var popup *Window
	var open int
	var popupValid bool
	c.True(screen.Do(func() {
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
		}
		owner.SetContentRect(geom.NewRect(60, 60, 300, 200))
		open = len(owner.root.openMenuPanels)
		popupValid = popup.IsValid()
	}))
	c.NotNil(popup)
	c.Equal(0, open)
	c.False(popupValid)
}

// TestPopupMenuInModalDialog verifies that the menu of a PopupMenu in a modal dialog opens above the dialog, which is a
// floating window, and that a choice made from it is carried out.
func TestPopupMenuInModalDialog(t *testing.T) {
	c := check.New(t)
	screen, _ := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var dialog *Dialog
	var popup *PopupMenu[string]
	c.True(screen.Post(func() {
		popup = NewPopupMenu[string]()
		popup.AddItem("Light", "Regular", "Bold")
		popup.SelectIndex(0)
		d, err := NewDialog(nil, nil, popup, []*DialogButtonInfo{NewOKButtonInfo()})
		if err != nil {
			t.Errorf("unable to create dialog: %v", err)
			return
		}
		dialog = d
		d.RunModal()
	}))
	screen.Sync()
	c.NotNil(dialog)
	if dialog == nil {
		return
	}
	screen.Click(screen.PanelCenter(popup))
	var m *menu
	var menuWnd, ownedBy, dialogWnd *Window
	var pt geom.Point
	c.True(screen.Do(func() {
		if inWindow, ok := popup.openMenu.(*menu); ok && inWindow.popupPanel != nil {
			m = inWindow
			menuWnd = inWindow.popupPanel.Window()
			ownedBy = menuWnd.owner
			dialogWnd = dialog.Window()
			pt = screen.PanelCenter(m.items[2].panel)
		}
	}))
	c.NotNil(m, "the popup's menu opens")
	if m == nil {
		return
	}
	c.True(ownedBy == dialogWnd, "for the dialog")
	c.True(screen.WindowAt(pt) == menuWnd, "above the dialog")
	screen.Click(pt)
	var selected int
	c.True(screen.Do(func() {
		selected = popup.SelectedIndex()
		dialog.StopModal(ModalResponseOK)
	}))
	screen.Sync()
	var menuWndValid bool
	c.True(screen.Do(func() { menuWndValid = menuWnd.IsValid() }))
	c.Equal(2, selected, "the choice is carried out")
	c.False(menuWndValid)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestBarTitlePressTogglesItsMenu verifies that a press on the title of the open menu closes it, rather than closing
// it and opening it again, whether a press or the pointer passing over the title opened it.
func TestBarTitlePressTogglesItsMenu(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var first, second *menu
	c.True(screen.Do(func() {
		DefaultMenuFactory().BarForWindow(owner, func(bar Menu) {
			if f, ok := bar.Factory().(*inWindowMenuFactory); ok {
				first = newTestMenu(f, UserBaseID, &chosen, "One", "Two")
				second = newTestMenu(f, UserBaseID+100, &chosen, "Three")
				bar.InsertMenu(-1, first)
				bar.InsertMenu(-1, second)
			}
		})
	}))
	c.NotNil(first, "the test needs an in-window menu bar")
	if first == nil {
		return
	}
	screen.Sync()
	open := func() (count int, firstOpen, secondOpen bool) {
		c.True(screen.Do(func() {
			count = len(owner.root.openMenuPanels)
			firstOpen = first.popupPanel != nil
			secondOpen = second.popupPanel != nil
		}))
		return count, firstOpen, secondOpen
	}
	screen.Click(screen.PanelCenter(first.titleItem.panel))
	count, firstOpen, secondOpen := open()
	c.Equal(1, count)
	c.True(firstOpen)
	c.False(secondOpen)
	var popup *Window
	c.True(screen.Do(func() { popup = first.popupPanel.Window() }))

	screen.Click(screen.PanelCenter(first.titleItem.panel))
	count, firstOpen, secondOpen = open()
	c.Equal(0, count, "a press on the open menu's title closes it")
	c.False(firstOpen)
	c.False(secondOpen)
	var popupValid bool
	c.True(screen.Do(func() { popupValid = popup.IsValid() }))
	c.False(popupValid)

	screen.Click(screen.PanelCenter(first.titleItem.panel))
	count, firstOpen, _ = open()
	c.Equal(1, count, "and the next press opens it again")
	c.True(firstOpen)

	screen.MouseMove(screen.PanelCenter(second.titleItem.panel), 0)
	count, firstOpen, secondOpen = open()
	c.Equal(1, count, "the pointer arriving over another title switches menus")
	c.False(firstOpen)
	c.True(secondOpen)

	screen.Click(screen.PanelCenter(second.titleItem.panel))
	count, _, secondOpen = open()
	c.Equal(0, count, "and a press on the title of the menu the pointer opened closes it too")
	c.False(secondOpen)
	var picked []string
	c.True(screen.Do(func() { picked = slices.Clone(chosen) }))
	c.Equal(0, len(picked))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPressOnItemWithOpenSubMenuKeepsIt verifies that a press on an item whose sub-menu is open leaves that sub-menu
// up, in the same popup window, rather than closing it and opening it again.
func TestPressOnItemWithOpenSubMenuKeepsIt(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m, sub *menu
	var opened bool
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep", "Deeper")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		opened = m.popupPanel != nil
	}))
	c.True(opened)
	if !opened {
		return
	}
	screen.Click(screen.PanelCenter(sub.titleItem.panel))
	var child *Window
	var stack int
	c.True(screen.Do(func() {
		if sub.popupPanel != nil {
			child = sub.popupPanel.Window()
		}
		stack = len(owner.root.openMenuPanels)
	}))
	c.NotNil(child)
	c.Equal(2, stack)
	if child == nil {
		return
	}
	screen.Click(screen.PanelCenter(sub.titleItem.panel))
	var again *Window
	var childValid bool
	var picked []string
	c.True(screen.Do(func() {
		if sub.popupPanel != nil {
			again = sub.popupPanel.Window()
		}
		stack = len(owner.root.openMenuPanels)
		childValid = child.IsValid()
		picked = slices.Clone(chosen)
	}))
	c.Equal(2, stack)
	c.True(again == child, "the sub-menu stays in the popup window it was in")
	c.True(childValid)
	c.Equal(0, len(picked))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestRemoveAllClearsItemMenus verifies that RemoveAll, like RemoveItem, leaves each removed item with no menu, so that
// Menu and Index agree that the item is in none.
func TestRemoveAllClearsItemMenus(t *testing.T) {
	c := check.New(t)
	f := &inWindowMenuFactory{}
	m := f.newMenu(UserBaseID, "Test", nil)
	one := f.NewItem(UserBaseID+1, "One", KeyBinding{}, nil, nil)
	two := f.NewItem(UserBaseID+2, "Two", KeyBinding{}, nil, nil)
	m.InsertItem(-1, one)
	m.InsertItem(-1, two)
	c.True(one.Menu() == m)
	m.RemoveAll()
	c.Equal(0, m.Count())
	c.True(one.Menu() == nil)
	c.True(two.Menu() == nil)
	c.Equal(-1, one.Index())
}

// TestCommandKeyLeavesTheOpenMenuAlone verifies that a key press with the command modifier down, which runs the
// updaters of the menu bar's menus, leaves the open menu's updater alone, so that the items its popup shows are still
// the menu's own and can be navigated to and chosen.
func TestCommandKeyLeavesTheOpenMenuAlone(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m *menu
	var updates int
	c.True(screen.Do(func() {
		DefaultMenuFactory().BarForWindow(owner, func(bar Menu) {
			if f, ok := bar.Factory().(*inWindowMenuFactory); ok {
				m = f.newMenu(UserBaseID, "Test", func(menu Menu) {
					updates++
					menu.RemoveAll()
					for i, title := range []string{"One", "Two"} {
						menu.InsertItem(-1, f.NewItem(UserBaseID+1+i, title, KeyBinding{}, nil, func(mi MenuItem) {
							chosen = append(chosen, mi.Title())
						}))
					}
				})
				bar.InsertMenu(-1, m)
			}
		})
	}))
	c.NotNil(m, "the test needs an in-window menu bar")
	if m == nil {
		return
	}
	screen.Sync()
	screen.Click(screen.PanelCenter(m.titleItem.panel))
	var shown []*menuItem
	var opened bool
	c.True(screen.Do(func() {
		if opened = m.popupPanel != nil; opened {
			shown = slices.Clone(m.items)
		}
	}))
	c.True(opened)
	if !opened {
		return
	}
	screen.KeyPress(KeyB, mod.OSMenuCommand())
	screen.KeyPress(KeyDown, 0)
	var items []*menuItem
	var updated, highlighted int
	c.True(screen.Do(func() {
		items = slices.Clone(m.items)
		updated = updates
		highlighted = m.popupPanel.itemIndex
	}))
	c.Equal(shown, items, "the open menu's items are left as its popup shows them")
	c.Equal(1, updated, "the updater ran only when the menu opened")
	c.Equal(0, highlighted, "and the arrow keys move through them")
	screen.Click(screen.PanelCenter(shown[0].panel))
	var open int
	var picked []string
	c.True(screen.Do(func() {
		open = len(owner.root.openMenuPanels)
		picked = slices.Clone(chosen)
	}))
	c.Equal([]string{"One"}, picked, "and a click chooses one")
	c.Equal(0, open)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMenuDisplayAmongPicksTheMostOverlapped verifies that a menu belongs on the display it overlaps most, rather than
// on the one holding its top-left corner, so that one hanging off the bottom of a display above another is placed on
// the lower one, where it fits, and one wholly within a display is placed there.
func TestMenuDisplayAmongPicksTheMostOverlapped(t *testing.T) {
	c := check.New(t)
	upper := &Display{Usable: geom.NewRect(0, 0, 1920, 1080), headless: true}
	lower := &Display{Usable: geom.NewRect(0, 1080, 1920, 1080), headless: true}
	displays := []*Display{upper, lower}
	c.True(menuDisplayAmong(displays, geom.NewRect(100, 1075, 300, 300)) == lower,
		"a menu whose corner is on the upper display but which mostly hangs down onto the lower one belongs there")
	c.True(menuDisplayAmong(displays, geom.NewRect(100, 1000, 300, 100)) == upper,
		"one that overlaps the upper display more belongs there")
	c.True(menuDisplayAmong(displays, geom.NewRect(100, 100, 300, 300)) == upper, "one wholly within a display")
	c.True(menuDisplayAmong(displays, geom.NewRect(100, 1200, 300, 300)) == lower)
	c.True(menuDisplayAmong(displays, geom.NewRect(2000, 100, 300, 300)) == nil, "one overlapping none")
}

// TestPopupHighlightsTheItemBeforeShowing verifies that a menu popped up on an item has that item highlighted, and
// scrolled into view, by the time its window is shown, so that the window's first draw, which on X11 happens as it is
// shown and describes the owner's menus to assistive technologies, already reports the focus on the item rather than on
// the menu. The item's sub-menu, which has to stack above the menu, is opened only afterwards.
func TestPopupHighlightsTheItemBeforeShowing(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(400, 300), geom.NewRect(0, 0, 400, 300))
	var highlightedWhenShown, visibleWhenShown, subMenuOpenWhenShown, placed bool
	var itemBounds geom.Rect
	var height float32
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		var chosen []string
		m := newTestMenu(f, UserBaseID, &chosen, menuItemTitles(40)...)
		sub := newTestMenu(f, UserBaseID+100, &chosen, "Deep")
		m.InsertMenu(-1, sub)
		if placed = m.createPopup(owner); !placed {
			return
		}
		popup := m.popupPanel.Window()
		last := len(m.items) - 1
		m.ensureOnDisplay(geom.NewRect(10, 10, 100, 2000), menuFit{}, owner, last)
		highlightedWhenShown = m.items[last].over
		visibleWhenShown = popup.IsVisible()
		subMenuOpenWhenShown = sub.popupPanel != nil
		itemBounds = m.items[last].panel.RectToRoot(m.items[last].panel.ContentRect(true))
		height = popup.ContentRect().Height
		m.setKeyIndex(last)
		m.closeMenuStack()
	}))
	c.True(placed)
	c.True(visibleWhenShown)
	c.True(highlightedWhenShown, "the item is highlighted by the time the window is shown")
	c.True(itemBounds.Y >= 0 && itemBounds.Bottom() <= height, "and scrolled into view: %v within %v", itemBounds,
		height)
	c.False(subMenuOpenWhenShown, "its sub-menu is not opened before the menu is shown")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestRemovedItemsKeepWorkingWhileShown verifies that an item removed from a menu, singly or with the rest, while the
// menu's popup still shows its panel, which is how application code refreshing a menu while it is open changes it, goes
// on drawing and being chosen from the popup, while Menu and Index say it is in no menu.
func TestRemovedItemsKeepWorkingWhileShown(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m *menu
	var shown []*menuItem
	var popup *Window
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One", "Two", "Three")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel == nil {
			return
		}
		popup = m.popupPanel.Window()
		shown = slices.Clone(m.items)
		m.RemoveItem(0)
		m.RemoveAll()
		for i, title := range []string{"Four", "Five"} {
			m.InsertItem(-1, f.NewItem(UserBaseID+10+i, title, KeyBinding{}, nil, nil))
		}
	}))
	c.NotNil(popup)
	if popup == nil {
		return
	}
	c.NotNil(screen.CaptureWindow(popup))
	c.Equal(0, len(screen.Errors()), "drawing the removed items must not panic: %v", screen.Errors())
	var inNone bool
	var index int
	c.True(screen.Do(func() {
		inNone = shown[0].Menu() == nil && shown[1].Menu() == nil
		index = shown[1].Index()
	}))
	c.True(inNone, "a removed item is in no menu")
	c.Equal(-1, index)
	screen.Click(screen.PanelCenter(shown[1].panel))
	var open int
	var picked []string
	c.True(screen.Do(func() {
		open = len(owner.root.openMenuPanels)
		picked = slices.Clone(chosen)
	}))
	c.Equal([]string{"Two"}, picked, "a removed item the popup still shows can be chosen")
	c.Equal(0, open, "and choosing it closes the menu")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMenuWindowRectWithoutADisplay verifies that a menu is fitted onto its display when it has one and left where it
// was asked to go when it has none, as on macOS while every display is asleep, rather than crashing on the display.
func TestMenuWindowRectWithoutADisplay(t *testing.T) {
	c := check.New(t)
	where := geom.NewRect(500, 400, 100, 50)
	c.Equal(where, menuWindowRect(where, menuFit{}, nil))
	d := &Display{Usable: geom.NewRect(0, 0, 400, 300), headless: true}
	c.Equal(geom.NewRect(300, 250, 100, 50), menuWindowRect(where, menuFit{}, d))
}

// TestBarMenuOpensAboveABarWithNoRoomBelow verifies that a menu opened from a menu bar title with too little room
// beneath it for its first three items is placed above the bar, with its bottom at the bar's top and shrunk to the
// space above when it has to be, while one with room for them stays below, shrunk to that room, and one whose bar has
// even less room above it than below stays below as well.
func TestBarMenuOpensAboveABarWithNoRoomBelow(t *testing.T) {
	c := check.New(t)
	usable := geom.NewRect(0, 0, 400, 300)
	fit := func(barTop, barBottom float32) menuFit {
		return menuFit{bar: true, barTop: barTop, barBottom: barBottom, minHeight: 60}
	}
	c.Equal(geom.NewRect(10, 80, 100, 200), fitMenuRect(geom.NewRect(10, 300, 100, 200), usable, fit(280, 300)),
		"a bar at the bottom of the display")
	c.Equal(geom.NewRect(10, 50, 100, 200), fitMenuRect(geom.NewRect(10, 270, 100, 200), usable, fit(250, 270)),
		"a bar with room for only a couple of items beneath it")
	c.Equal(geom.NewRect(10, 0, 100, 250), fitMenuRect(geom.NewRect(10, 270, 100, 400), usable, fit(250, 270)),
		"a menu taller than the space above the bar is shrunk to it")
	c.Equal(geom.NewRect(10, 220, 100, 80), fitMenuRect(geom.NewRect(10, 220, 100, 200), usable, fit(200, 220)),
		"a bar with room for three items keeps its menu below, shrunk to the room")
	c.Equal(geom.NewRect(10, 30, 100, 200), fitMenuRect(geom.NewRect(10, 30, 100, 200), usable, fit(10, 30)),
		"a bar at the top")
	usable = geom.NewRect(0, 0, 400, 50)
	c.Equal(geom.NewRect(10, 20, 100, 30), fitMenuRect(geom.NewRect(10, 20, 100, 200), usable, fit(0, 20)),
		"a bar with less room above it than below keeps its menu below")
}

// TestBarMenuAtTheScreenBottomOpensAbove verifies, end to end, that a menu opened from the menu bar of a window whose
// bar sits at the bottom of the screen is placed above the bar rather than squeezed into the sliver beneath it.
func TestBarMenuAtTheScreenBottomOpensAbove(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 372, 300, 80))
	var chosen []string
	var m *menu
	c.True(screen.Do(func() {
		DefaultMenuFactory().BarForWindow(owner, func(bar Menu) {
			if f, ok := bar.Factory().(*inWindowMenuFactory); ok {
				m = newTestMenu(f, UserBaseID, &chosen, menuItemTitles(5)...)
				bar.InsertMenu(-1, m)
			}
		})
	}))
	c.NotNil(m, "the test needs an in-window menu bar")
	if m == nil {
		return
	}
	screen.Sync()
	screen.Click(screen.PanelCenter(m.titleItem.panel))
	var rect geom.Rect
	var barTop, barBottom, height float32
	var opened bool
	c.True(screen.Do(func() {
		if opened = m.popupPanel != nil; opened {
			rect = m.popupPanel.Window().ContentRect()
			height = m.popupPanel.FrameRect().Height
		}
		title := m.titleItem.panel
		tr := title.RectToRoot(title.ContentRect(true))
		barTop = owner.apiPointToScreen(tr.Point).Y
		barBottom = owner.apiPointToScreen(geom.NewPoint(0, tr.Bottom())).Y
	}))
	c.True(opened)
	c.True(barBottom > 390 && barTop < 400, "the test needs the bar at the bottom of the screen: %v-%v", barTop,
		barBottom)
	c.Equal(barTop, rect.Bottom(), "the menu ends at the top of the bar")
	c.Equal(height, rect.Height, "and keeps its height")
}

// TestSubMenuOpensOnTheLeftWhenItDoesNotFit verifies that a sub-menu that does not fit to the right of its item is placed
// to the left of the item, with its right edge at the item's left edge, rather than shifted left over the parent menu,
// and that one that does fit stays on the right.
func TestSubMenuOpensOnTheLeftWhenItDoesNotFit(t *testing.T) {
	c := check.New(t)
	usable := geom.NewRect(0, 0, 400, 300)
	fit := menuFit{sub: true, leftOf: 250}
	c.Equal(geom.NewRect(350, 50, 40, 100), fitMenuRect(geom.NewRect(350, 50, 40, 100), usable, fit), "fits")
	c.Equal(geom.NewRect(150, 50, 100, 100), fitMenuRect(geom.NewRect(350, 50, 100, 100), usable, fit),
		"does not fit, so goes to the left of the item")
	c.Equal(geom.NewRect(0, 50, 300, 100), fitMenuRect(geom.NewRect(350, 50, 300, 100), usable, fit),
		"one too wide for the left as well is kept on the display")
	c.Equal(geom.NewRect(300, 50, 100, 100), fitMenuRect(geom.NewRect(350, 50, 100, 100), usable, menuFit{}),
		"a menu that is not a sub-menu is shifted left to fit")

	screen, _ := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m, sub *menu
	var opened bool
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep", "Deeper")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(380, 20, 0, 0), -1)
		opened = m.popupPanel != nil
	}))
	c.True(opened)
	if !opened {
		return
	}
	screen.Click(screen.PanelCenter(sub.titleItem.panel))
	var parent, child *Window
	var at, parentRect geom.Rect
	var itemLeft float32
	c.True(screen.Do(func() {
		parent = m.popupPanel.Window()
		parentRect = parent.ContentRect()
		if sub.popupPanel != nil {
			child = sub.popupPanel.Window()
			at = child.ContentRect()
		}
		item := sub.titleItem.panel
		itemLeft = parent.apiPointToScreen(item.RectToRoot(item.ContentRect(true)).Point).X
	}))
	c.NotNil(child)
	if child == nil {
		return
	}
	c.True(parentRect.Right()+at.Width > 500, "the test needs a sub-menu that does not fit on the right")
	c.Equal(itemLeft, at.Right(), "placed to the left of its item")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
