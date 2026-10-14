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
		place := func(where geom.Rect, top float32) {
			m.ensureOnDisplay(where, top, owner)
			rects = append(rects, m.popupPanel.Window().ContentRect())
		}
		// Below the bar: no room to grow, so it shrinks and stays put.
		place(geom.NewRect(10, 30, 100, 500), 30)
		// A menu that fits under the bar is left where it was asked to go.
		place(geom.NewRect(10, 30, 100, 200), 30)
		// One that nearly fits is moved up, but never above the bar.
		place(geom.NewRect(10, 30, 100, 280), 30)
		// Without a limit, the same menu may cover the whole display height.
		place(geom.NewRect(10, 30, 100, 500), noTopLimit)
		// A sub-menu with no limit still slides up to fit.
		place(geom.NewRect(10, 100, 100, 250), noTopLimit)
		// One that overlaps no display is placed on its anchor's.
		place(geom.NewRect(500, 400, 100, 50), noTopLimit)
		m.closeMenuStack()
	}))
	c.True(created)
	c.Equal([]geom.Rect{
		geom.NewRect(10, 30, 100, 270),
		geom.NewRect(10, 30, 100, 200),
		geom.NewRect(10, 30, 100, 270),
		geom.NewRect(10, 0, 100, 300),
		geom.NewRect(10, 50, 100, 250),
		geom.NewRect(300, 250, 100, 50),
	}, rects)
}

// TestPopupWindowStacksAboveItsOwner verifies that a menu opens in a popup window of its own, placed from the owner's
// coordinates and stacked above it, which never becomes the active window or takes the focus.
func TestPopupWindowStacksAboveItsOwner(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var m *menu
	var chosen []string
	var popup, active *Window
	var rect geom.Rect
	c.True(screen.Do(func() {
		m = newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two")
		m.Popup(geom.NewRect(20, 30, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
			rect = popup.ContentRect()
		}
		active = ActiveWindow()
	}))
	c.NotNil(popup)
	if popup == nil {
		return
	}
	c.True(popup != owner)
	c.True(popup.owner == owner)
	c.Equal(geom.NewPoint(70, 80), rect.Point, "placed at the point asked for, taken from the owner's coordinates")
	c.True(active == owner, "the popup never becomes the active window")
	c.True(screen.FocusedWindow() == owner, "nor takes the focus")
	c.True(screen.WindowAt(screen.PanelCenter(m.items[0].panel)) == popup, "and is stacked above its owner")
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
	c.True(screen.Do(func() { open = len(owner.root.openMenuPanels) }))
	c.Equal(0, open)
	c.False(popup.IsValid(), "the popup window is disposed of")
	c.Nil(m.popupPanel)
	c.Equal(0, len(chosen))
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
	c.True(screen.Do(func() { open = len(owner.root.openMenuPanels) }))
	c.Equal(0, open)
	c.False(popup.IsValid())
	c.Equal(0, len(chosen))
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
	c.True(screen.Do(func() { open = len(owner.root.openMenuPanels) }))
	c.Equal([]string{"Two"}, chosen)
	c.Equal(0, open)
	c.False(popup.IsValid())
}

// TestSubMenuOpensBesideItsParentPopup verifies that a sub-menu opens in a popup window of its own beside the item in
// its parent's popup, stacked above it and belonging to the same owner, and that clicks in the popups never move the
// focus or the activation away from the owner.
func TestSubMenuOpensBesideItsParentPopup(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m, sub *menu
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep", "Deeper")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
	}))
	c.NotNil(m.popupPanel)
	if m.popupPanel == nil {
		return
	}
	screen.Click(screen.PanelCenter(sub.titleItem.panel))
	var parent, child, active *Window
	var want geom.Point
	var at geom.Rect
	var stack int
	c.True(screen.Do(func() {
		parent = m.popupPanel.Window()
		if sub.popupPanel != nil {
			child = sub.popupPanel.Window()
			at = child.ContentRect()
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
	c.True(child.owner == owner, "a sub-menu's popup belongs to the top-level window")
	c.Equal(want, at.Point, "placed beside its item")
	c.True(screen.WindowAt(at.Center()) == child, "and stacked above its parent")
	c.True(active == owner)
	c.True(screen.FocusedWindow() == owner)

	screen.Click(screen.PanelCenter(sub.items[1].panel))
	c.True(screen.Do(func() {
		stack = len(owner.root.openMenuPanels)
		active = ActiveWindow()
	}))
	c.Equal([]string{"Deeper"}, chosen)
	c.Equal(0, stack)
	c.False(parent.IsValid())
	c.False(child.IsValid())
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
	c.True(screen.Do(func() {
		m = newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, menuItemTitles(40)...)
		m.Popup(geom.NewRect(10, 10, 0, 0), -1)
		if m.popupPanel != nil {
			rect = m.popupPanel.Window().ContentRect()
			before = itemBounds()
		}
	}))
	c.NotNil(m.popupPanel)
	if m.popupPanel == nil {
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

// TestBarMenuStaysBelowTheBar verifies that a menu opened from the menu bar of a window low on the screen is placed
// below the bar, in screen coordinates, and shrinks to the space left beneath it rather than covering the bar.
func TestBarMenuStaysBelowTheBar(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 300, 300, 80))
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
	c.True(screen.Do(func() {
		if m.popupPanel != nil {
			rect = m.popupPanel.Window().ContentRect()
		}
		title := m.titleItem.panel
		barBottom = owner.apiPointToScreen(geom.NewPoint(0, title.RectToRoot(title.ContentRect(true)).Bottom())).Y
	}))
	c.NotNil(m.popupPanel)
	c.True(barBottom > 300, "the test needs the bar on the screen")
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
	c.True(screen.Do(func() {
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			popup = m.popupPanel.Window()
		}
		owner.SetContentRect(geom.NewRect(60, 60, 300, 200))
		open = len(owner.root.openMenuPanels)
	}))
	c.NotNil(popup)
	c.Equal(0, open)
	c.False(popup.IsValid())
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
	var menuWnd *Window
	c.True(screen.Do(func() {
		if inWindow, ok := popup.openMenu.(*menu); ok && inWindow.popupPanel != nil {
			m = inWindow
			menuWnd = inWindow.popupPanel.Window()
		}
	}))
	c.NotNil(m, "the popup's menu opens")
	if m == nil {
		return
	}
	c.True(menuWnd.owner == dialog.Window(), "for the dialog")
	pt := screen.PanelCenter(m.items[2].panel)
	c.True(screen.WindowAt(pt) == menuWnd, "above the dialog")
	screen.Click(pt)
	var selected int
	c.True(screen.Do(func() {
		selected = popup.SelectedIndex()
		dialog.StopModal(ModalResponseOK)
	}))
	screen.Sync()
	c.Equal(2, selected, "the choice is carried out")
	c.False(menuWnd.IsValid())
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
	c.False(popup.IsValid())

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
	c.Equal(0, len(chosen))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPressOnItemWithOpenSubMenuKeepsIt verifies that a press on an item whose sub-menu is open leaves that sub-menu
// up, in the same popup window, rather than closing it and opening it again.
func TestPressOnItemWithOpenSubMenuKeepsIt(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m, sub *menu
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep", "Deeper")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
	}))
	c.NotNil(m.popupPanel)
	if m.popupPanel == nil {
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
	c.True(screen.Do(func() {
		if sub.popupPanel != nil {
			again = sub.popupPanel.Window()
		}
		stack = len(owner.root.openMenuPanels)
	}))
	c.Equal(2, stack)
	c.True(again == child, "the sub-menu stays in the popup window it was in")
	c.True(child.IsValid())
	c.Equal(0, len(chosen))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
