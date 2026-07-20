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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/cocoa"
)

var _ Menu = &macMenu{}

// macMenu wraps a cocoa.Menu handle. owned records whether this wrapper holds the menu's owned reference: wrappers
// from the factory own the menus they create until InsertMenu transfers that ownership into the parent menu's tree,
// while wrappers that merely navigate existing menus (ItemAtIndex, SubMenu, updater callbacks) never own anything.
// Only an owning wrapper's Dispose releases the underlying menu.
type macMenu struct {
	factory *macMenuFactory
	id      int
	menu    cocoa.Menu
	owned   bool
}

func (m *macMenu) Factory() MenuFactory {
	return m.factory
}

func (m *macMenu) ID() int {
	return m.id
}

func (m *macMenu) IsSame(other Menu) bool {
	if m2, ok := other.(*macMenu); ok {
		return m.menu == m2.menu
	}
	return false
}

func (m *macMenu) ItemAtIndex(index int) MenuItem {
	if index < 0 || index >= m.Count() {
		return nil
	}
	return &macMenuItem{
		factory: m.factory,
		item:    m.menu.ItemAtIndex(index),
	}
}

func (m *macMenu) Item(id int) MenuItem {
	for i := m.Count() - 1; i >= 0; i-- {
		mi := m.ItemAtIndex(i)
		if mi.IsSeparator() {
			continue
		}
		if mi.ID() == id {
			return mi
		}
		if sub := mi.SubMenu(); sub != nil {
			if mi = sub.Item(id); mi != nil {
				return mi
			}
		}
	}
	return nil
}

func (m *macMenu) Menu(id int) Menu {
	for i := m.Count() - 1; i >= 0; i-- {
		mi := m.ItemAtIndex(i)
		if mi.IsSeparator() {
			continue
		}
		if sub := mi.SubMenu(); sub != nil {
			if sub.ID() == id {
				return sub
			}
			if sub = sub.Menu(id); sub != nil {
				return sub
			}
		}
	}
	return nil
}

func (m *macMenu) InsertSeparator(atIndex int, onlyIfNeeded bool) {
	if onlyIfNeeded {
		if count := m.Count(); count != 0 {
			if atIndex < 0 {
				atIndex = count
			}
			if atIndex != 0 {
				if m.ItemAtIndex(atIndex - 1).IsSeparator() {
					return
				}
			}
		}
	}
	m.macInsert(cocoa.NewSeparatorMenuItem(), atIndex)
}

func (m *macMenu) InsertItem(atIndex int, mi MenuItem) {
	if mi != nil {
		if item, ok := mi.(*macMenuItem); ok {
			m.macInsert(item.item, atIndex)
		}
	}
}

func (m *macMenu) InsertMenu(atIndex int, subMenu Menu) {
	if menu, ok := subMenu.(*macMenu); ok {
		// newMacMenuItemForSubMenu transfers ownership of the submenu to the item holding it (and the item to this
		// menu), so from here on the tree owns the submenu and Dispose of its wrapper must become a no-op.
		m.macInsert(newMacMenuItemForSubMenu(m.factory, menu), atIndex)
		menu.owned = false
		switch menu.id {
		case AppMenuID:
			if servicesItem := m.Item(ServicesMenuID); servicesItem != nil {
				if servicesMenu := servicesItem.SubMenu(); servicesMenu != nil {
					if menu, ok = servicesMenu.(*macMenu); ok {
						cocoa.SetServicesMenu(menu.menu)
					}
				}
			}
		case WindowMenuID:
			cocoa.SetWindowsMenu(menu.menu)
		case HelpMenuID:
			cocoa.SetHelpMenu(menu.menu)
		}
	}
}

func (m *macMenu) RemoveItem(index int) {
	if index >= 0 && index < m.Count() {
		m.menu.RemoveItemAtIndex(index)
	}
}

func (m *macMenu) RemoveAll() {
	m.menu.RemoveAll()
}

func (m *macMenu) Title() string {
	return m.menu.Title()
}

func (m *macMenu) Count() int {
	return m.menu.NumberOfItems()
}

func (m *macMenu) Popup(where geom.Rect, itemIndex int) {
	if w := ActiveWindow(); w.IsValid() {
		if mi := m.ItemAtIndex(itemIndex); mi != nil {
			frame := w.wnd.view.Frame()
			where.X += 8
			where.Y = frame.Height - where.Bottom()
			m.menu.Popup(w.wnd.wnd, m.menu, m.menu.ItemAtIndex(itemIndex), where)
		}
	}
}

func (m *macMenu) Dispose() {
	// Only the owning wrapper may release; disposing a wrapper for a menu owned by a tree (e.g. one obtained through
	// SubMenu) is a no-op, since the tree's root cleans it up.
	if m.owned {
		m.owned = false
		m.menu.Release()
	}
}

func (m *macMenu) macInsert(mi cocoa.MenuItem, index int) {
	if index < 0 {
		index = m.Count()
	}
	m.menu.InsertItemAtIndex(mi, index)
}
