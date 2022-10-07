// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath"
)

var _ Menu = &menu{}

// Menu holds a set of menu items.
type Menu interface {
	// Factory returns the MenuFactory that created this Menu.
	Factory() MenuFactory
	// ID returns the id of this Menu.
	ID() int
	// IsSame returns true if the two menus represent the same object. Do not use == to test for equality.
	IsSame(other Menu) bool
	// ItemAtIndex returns the menu item at the specified index within the menu.
	ItemAtIndex(index int) MenuItem
	// Item returns the menu item with the specified id anywhere in the menu and and its sub-menus.
	Item(id int) MenuItem
	// Menu returns the menu with the specified id anywhere in the menu and and its sub-menus.
	Menu(id int) Menu
	// InsertSeparator inserts a menu separator at the specified menuItem index within this menu. Pass in a negative
	// index to append to the end. If onlyIfNeeded is true, then a separator is only inserted if the menuItem that would
	// precede it at the insertion location is not a separator.
	InsertSeparator(atIndex int, onlyIfNeeded bool)
	// InsertItem inserts a menu item at the specified menuItem index within this menu. Pass in a negative index to
	// append to the end. If the menu item is nil, then nothing happens.
	InsertItem(atIndex int, mi MenuItem)
	// InsertMenu inserts a new sub-menu at the specified menuItem index within this menu. Pass in a negative index to
	// append to the end.
	InsertMenu(atIndex int, subMenu Menu)
	// RemoveItem removes the menu item at the specified index from this menu.
	RemoveItem(index int)
	// RemoveAll removes all menu items from the menu.
	RemoveAll()
	// Title returns the title of this menu.
	Title() string
	// Count of menu items in this menu.
	Count() int
	// Popup the menu at the specified position within the active window.
	Popup(where Rect, itemIndex int)
	// Dispose releases any OS resources associated with this menu.
	Dispose()
}

// DefaultMenuTheme holds the default MenuTheme values for Menus. Modifying this data will not alter existing Menus,
// but will alter any Menus created in the future.
var DefaultMenuTheme = MenuTheme{
	BarBorder:  NewLineBorder(DividerColor, 0, Insets{Bottom: 1}, false),
	MenuBorder: NewLineBorder(DividerColor, 0, NewUniformInsets(1), false),
}

// MenuTheme holds theming data for a Menu.
type MenuTheme struct {
	BarBorder  Border
	MenuBorder Border
}

type menuPanel struct {
	Panel
	menu      *menu
	itemIndex int
}

type menu struct {
	factory    *inWindowMenuFactory
	titleItem  *menuItem
	items      []*menuItem
	updater    func(Menu)
	popupPanel *menuPanel
}

func (m *menu) Factory() MenuFactory {
	return m.factory
}

func (m *menu) IsSame(other Menu) bool {
	return m == other
}

func (m *menu) ItemAtIndex(index int) MenuItem {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	return m.items[index]
}

func (m *menu) Item(id int) MenuItem {
	for _, mi := range m.items {
		if mi.IsSeparator() {
			continue
		}
		if id == mi.ID() {
			return mi
		}
		if mi.subMenu != nil {
			if subItem := mi.subMenu.Item(id); subItem != nil {
				return subItem
			}
		}
	}
	return nil
}

func (m *menu) Menu(id int) Menu {
	for _, mi := range m.items {
		if mi.IsSeparator() {
			continue
		}
		if mi.subMenu != nil {
			if id == mi.ID() {
				return mi.subMenu
			}
			if sub := mi.subMenu.Menu(id); sub != nil {
				return sub
			}
		}
	}
	return nil
}

func (m *menu) InsertSeparator(atIndex int, onlyIfNeeded bool) {
	if onlyIfNeeded && atIndex != 0 && len(m.items) != 0 {
		if atIndex < 0 || atIndex > len(m.items) {
			atIndex = len(m.items)
		}
		if m.items[atIndex-1].IsSeparator() {
			return
		}
	}
	m.insertItem(atIndex, &menuItem{
		factory:     m.factory,
		menu:        m,
		isSeparator: true,
	})
}

func (m *menu) InsertItem(atIndex int, mi MenuItem) {
	if mi != nil {
		m.insertItem(atIndex, mi.(*menuItem))
	}
}

func (m *menu) InsertMenu(atIndex int, subMenu Menu) {
	if sub, ok := subMenu.(*menu); ok {
		m.insertItem(atIndex, sub.titleItem)
	}
}

func (m *menu) insertItem(atIndex int, mi *menuItem) {
	mi.menu = m
	if atIndex < 0 || atIndex >= len(m.items) {
		m.items = append(m.items, mi)
	} else {
		m.items = append(m.items, nil)
		copy(m.items[atIndex+1:], m.items[atIndex:])
		m.items[atIndex] = mi
	}
}

func (m *menu) RemoveItem(index int) {
	if index >= 0 && index < len(m.items) {
		m.items[index].menu = nil
		copy(m.items[index:], m.items[index+1:])
		m.items[len(m.items)-1] = nil
		m.items = m.items[:len(m.items)-1]
	}
}

func (m *menu) RemoveAll() {
	m.items = nil
}

func (m *menu) ID() int {
	return m.titleItem.id
}

func (m *menu) Title() string {
	return m.titleItem.title
}

func (m *menu) String() string {
	return m.titleItem.String()
}

func (m *menu) Count() int {
	return len(m.items)
}

func (m *menu) Popup(where Rect, itemIndex int) {
	if m.popupPanel == nil {
		m.createPopup()
		if itemIndex >= 0 && itemIndex < len(m.items) {
			m.popupPanel.ValidateLayout()
			where.Y -= m.popupPanel.Children()[0].Self.(*ScrollPanel).Content().AsPanel().Children()[itemIndex].FrameRect().Y
		}
		fr := m.popupPanel.FrameRect()
		where.Height = fr.Height
		where.Width = xmath.Max(fr.Width, where.Width)
		m.ensureInWindow(where)
		m.popupPanel.ValidateLayout()
		m.setKeyIndex(itemIndex)
	}
}

func (m *menu) ensureInWindow(where Rect) {
	fr := m.popupPanel.Parent().ContentRect(true)
	if where.Width > fr.Width {
		where.Width = fr.Width
	}
	if where.Right() > fr.Right() {
		where.X = fr.Right() - where.Width
	}
	if where.X < 0 {
		where.X = 0
	}
	if where.Height > fr.Height {
		where.Height = fr.Height
	}
	if where.Bottom() > fr.Bottom() {
		where.Y = fr.Bottom() - where.Height
	}
	if where.Y < 0 {
		where.Y = 0
	}
	m.popupPanel.SetFrameRect(where)
}

func (m *menu) createPopup() {
	if m.popupPanel != nil {
		return
	}
	if activeWnd := ActiveWindow(); activeWnd != nil {
		m.closeMenuStackStoppingAt(activeWnd, m.titleItem.menu)
		root := m
		for root.titleItem.menu != nil {
			root = root.titleItem.menu
		}
		m.popupPanel = m.newPanel(false)
		m.popupPanel.Pack()
		activeWnd.root.insertMenu(m.popupPanel)
	}
}

func (m *menu) newPanel(forBar bool) *menuPanel {
	p := &menuPanel{
		menu:      m,
		itemIndex: -1,
	}
	p.Self = p
	if forBar {
		p.SetBorder(DefaultMenuTheme.BarBorder)
	} else {
		p.SetBorder(DefaultMenuTheme.MenuBorder)
	}
	content := NewPanel()
	if m.updater != nil {
		m.updater(m)
	}
	for _, mi := range m.items {
		mi.validate()
		child := mi.newPanel()
		content.AddChild(child)
		if !forBar {
			child.SetLayoutData(&FlexLayoutData{
				HAlign: FillAlignment,
				VAlign: MiddleAlignment,
				HGrab:  true,
			})
		}
	}
	p.KeyDownCallback = func(keyCode KeyCode, mod Modifiers, _ bool) bool {
		if mod != 0 {
			return false
		}
		switch keyCode {
		case KeyDown:
			old := m.popupPanel.itemIndex
			next := old
			for {
				next++
				if next < 0 {
					next = 0
				}
				if next >= len(p.menu.items) {
					break
				}
				if !p.menu.items[next].isSeparator {
					m.popupPanel.itemIndex = next
					break
				}
			}
			m.doExitEnter(old)
			return true
		case KeyUp:
			old := m.popupPanel.itemIndex
			next := old
			for {
				next--
				if next < 0 {
					break
				}
				if next >= len(p.menu.items) {
					next = len(p.menu.items) - 1
					if next < 0 {
						break
					}
				}
				if !p.menu.items[next].isSeparator {
					m.popupPanel.itemIndex = next
					break
				}
			}
			m.doExitEnter(old)
			return true
		case KeyReturn, KeyNumPadEnter:
			if m.popupPanel.itemIndex >= 0 && m.popupPanel.itemIndex < len(p.menu.items) {
				m.items[m.popupPanel.itemIndex].mouseDown(Point{}, 0, 0, 0) // params are unused
				return true
			}
		case KeyEscape, KeyLeft:
			m.closeMenuStackStoppingAt(p.Window(), m.titleItem.menu)
			return true
		case KeyRight:
			if m.popupPanel.itemIndex >= 0 && m.popupPanel.itemIndex < len(m.items) && m.items[m.popupPanel.itemIndex].subMenu != nil {
				m.doExitEnter(-1)
			}
		}
		return false
	}
	lay := &FlexLayout{Columns: 1}
	if forBar {
		lay.Columns = len(content.Children())
	}
	content.SetLayout(lay)
	content.Pack()
	s := NewScrollPanel()
	s.SetContent(content, FollowBehavior, FillBehavior)
	s.SetLayoutData(&FlexLayoutData{
		HAlign: FillAlignment,
		VAlign: FillAlignment,
		HGrab:  true,
		VGrab:  true,
	})
	p.SetLayout(&FlexLayout{Columns: 1})
	p.AddChild(s)
	return p
}

func (m *menu) doExitEnter(previousIndex int) {
	if previousIndex != m.popupPanel.itemIndex && m.popupPanel.itemIndex >= 0 && m.popupPanel.itemIndex < len(m.items) {
		if previousIndex >= 0 && previousIndex < len(m.items) {
			m.items[previousIndex].mouseExit()
		}
		m.items[m.popupPanel.itemIndex].mouseEnter(Point{}, 0) // params are unused
		m.items[m.popupPanel.itemIndex].scrollIntoView()
		if subMenu := m.items[m.popupPanel.itemIndex].subMenu; subMenu != nil {
			subMenu.setKeyIndex(0)
		}
	}
}

func (m *menu) setKeyIndex(index int) {
	m.popupPanel.itemIndex = index
	if index >= 0 && index < len(m.items) {
		m.items[index].mouseEnter(Point{}, 0) // params are unused
		m.items[index].scrollIntoView()
	}
}

func (m *menu) Dispose() {
}

func (m *menu) preMoved(w *Window) {
	m.closeMenuStackStoppingAt(w, nil)
}

func (m *menu) postLostFocus(w *Window) {
	// Need to give the event loop a chance to potentially refocus it before we decide to tear it down.
	InvokeTask(func() {
		if ActiveWindow() != w {
			m.closeMenuStackStoppingAt(w, nil)
		}
	})
}

func (m *menu) preMouseDown(w *Window, where Point) bool {
	if w.root.menuBar != nil {
		for _, one := range w.root.openMenuPanels {
			if one.FrameRect().ContainsPoint(where) {
				m.closeMenuStackStoppingAt(w, one.menu)
				return false
			}
		}
		m.closeMenuStackStoppingAt(w, nil)
	}
	return false
}

func (m *menu) preKeyDown(wnd *Window, keyCode KeyCode, mod Modifiers) bool {
	if m.updater != nil && mod.OSMenuCmdModifierDown() {
		// We only call the updater for key presses with the cmd modifier down. This is because the updater may be doing
		// relatively expensive operations to setup the menu and we don't want to impact every key a user types.
		m.updater(m)
	}
	for _, mi := range m.items {
		if !mi.keyBinding.KeyCode.ShouldOmit() && mi.keyBinding.KeyCode == keyCode && mi.keyBinding.Modifiers == mod {
			mi.validate()
			if mi.enabled {
				mi.execute()
				return true
			}
			return len(wnd.root.openMenuPanels) != 0
		}
		if mi.subMenu != nil {
			if mi.subMenu.preKeyDown(wnd, keyCode, mod) {
				return true
			}
		}
	}
	return len(wnd.root.openMenuPanels) != 0
}

func (m *menu) preKeyUp(wnd *Window, _ KeyCode, _ Modifiers) bool {
	return len(wnd.root.openMenuPanels) != 0
}

func (m *menu) preRuneTyped(wnd *Window, _ rune) bool {
	return len(wnd.root.openMenuPanels) != 0
}

func (m *menu) closeMenuStack() {
	if wnd := ActiveWindow(); wnd != nil {
		m.closeMenuStackStoppingAt(wnd, nil)
		wnd.ToFront()
	}
}

func (m *menu) closeMenuStackStoppingAt(wnd *Window, stopAt *menu) {
	if len(wnd.root.openMenuPanels) == 0 {
		return
	}
	for i := len(wnd.root.openMenuPanels) - 1; i >= 0; i-- {
		if wnd.root.openMenuPanels[i].menu == stopAt {
			return
		}
		wnd.root.removeMenu(wnd.root.openMenuPanels[i])
	}
}
