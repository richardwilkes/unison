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
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
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
	// Title returns the title of this menu.
	Title() string
	// Count of menu items in this menu.
	Count() int
	// Popup the menu at the specified position within the active window.
	Popup(where geom32.Rect, itemIndex int)
	// Dispose releases any OS resources associated with this menu.
	Dispose()
}

type menu struct {
	factory   *inWindowMenuFactory
	titleItem *menuItem
	items     []*menuItem
	updater   func(Menu)
	popup     *Window
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

func (m *menu) Popup(where geom32.Rect, itemIndex int) {
	if m.popup == nil {
		m.createPopup()
		if m.popup != nil {
			where.Point.Add(ActiveWindow().ContentRect().Point)
			if itemIndex >= 0 && itemIndex < len(m.items) {
				p := m.popup.Content().Children()[itemIndex]
				fr := p.FrameRect()
				where.X += fr.X
				where.Y -= fr.Y
			}
			fr := m.popup.FrameRect()
			where.Height = fr.Height
			where.Width = mathf32.Max(fr.Width, where.Width)
			m.popup.SetFrameRect(where)
			m.popup.Show()
			if itemIndex >= 0 && itemIndex < len(m.items) {
				m.items[itemIndex].mouseEnter(geom32.Point{}, 0) // params are unused
			}
		}
	}
}

func (m *menu) createPopup() {
	if m.popup != nil {
		return
	}
	activeWnd := ActiveWindow()
	m.closeMenuStackStoppingAt(activeWnd, m.titleItem.menu)
	w, err := NewWindow("", NotResizableWindowOption(), FloatingWindowOption(), UndecoratedWindowOption(), TransientWindowOption())
	if err != nil {
		jot.Error(err)
		return
	}
	m.popup = w
	root := m
	for root.titleItem.menu != nil {
		root = root.titleItem.menu
	}
	m.factory.wndRootMenuMap[activeWnd] = root
	w.SetContent(m.newPanel(false))
	w.Pack()
}

func (m *menu) Dispose() {
}

func (m *menu) newPanel(forBar bool) *Panel {
	p := NewPanel()
	if forBar {
		p.SetBorder(NewLineBorder(DividerColor, 0, geom32.Insets{Bottom: 1}, false))
	} else {
		p.SetBorder(NewLineBorder(DividerColor, 0, geom32.NewUniformInsets(1), false))
	}
	for _, mi := range m.items {
		mi.validate()
		child := mi.newPanel()
		p.AddChild(child)
		if !forBar {
			child.SetLayoutData(&FlexLayoutData{
				HSpan:  1,
				VSpan:  1,
				HAlign: FillAlignment,
				VAlign: MiddleAlignment,
				HGrab:  true,
			})
		}
	}
	lay := &FlexLayout{Columns: 1}
	if forBar {
		lay.Columns = len(p.Children())
	}
	p.SetLayout(lay)
	return p
}

func (m *menu) preMoved(w *Window) {
	m.closeMenuStackStoppingAt(w, nil)
}

func (m *menu) postLostFocus(w *Window) {
	// Need to give the event loop a chance to potentially refocus is before we decide to tear it down.
	InvokeTask(func() {
		if ActiveWindow() != w {
			m.closeMenuStackStoppingAt(w, nil)
		}
	})
}

func (m *menu) preMouseDown(w *Window, where geom32.Point) bool {
	if !overMenuBar(w, where) {
		m.closeMenuStackStoppingAt(w, nil)
	}
	return false
}

func (m *menu) preKeyDown(wnd *Window, keyCode KeyCode, mod Modifiers) bool {
	for _, mi := range m.items {
		if mi.keyCode != 0 && mi.keyCode == keyCode && mi.keyModifiers == mod {
			mi.validate()
			if mi.enabled {
				mi.execute()
				return true
			}
			return m.isActiveWindowShowingPopupMenu()
		}
		if mi.subMenu != nil {
			if mi.subMenu.preKeyDown(wnd, keyCode, mod) {
				return true
			}
		}
	}
	return m.isActiveWindowShowingPopupMenu()
}

func (m *menu) preKeyUp(_ *Window, _ KeyCode, _ Modifiers) bool {
	return m.isActiveWindowShowingPopupMenu()
}

func (m *menu) preRuneTyped(_ *Window, _ rune) bool {
	return m.isActiveWindowShowingPopupMenu()
}

func (m *menu) isActiveWindowShowingPopupMenu() bool {
	return m.factory.wndRootMenuMap[ActiveWindow()] != nil
}

func (m *menu) closeMenuStack() bool {
	wnd := ActiveWindow()
	closed := m.closeMenuStackStoppingAt(wnd, nil)
	wnd.ToFront()
	return closed
}

func (m *menu) closeMenuStackStoppingAt(wnd *Window, stopAt *menu) bool {
	list := m.factory.activeMenuList(wnd)
	if len(list) == 0 {
		return false
	}
	for i := len(list) - 1; i >= 0; i-- {
		if list[i] == stopAt {
			return true
		}
		if list[i].popup != nil {
			list[i].popup.Dispose()
			list[i].popup = nil
		}
	}
	delete(m.factory.wndRootMenuMap, wnd)
	return true
}

func (m *menu) collectActiveMenus(list []*menu) []*menu {
	list = append(list, m)
	for _, mi := range m.items {
		if mi.subMenu != nil && mi.subMenu.popup != nil {
			return mi.subMenu.collectActiveMenus(list)
		}
	}
	return list
}
