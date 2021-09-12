// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/progrium/macdriver/objc"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison/internal/ns"
)

var _ Menu = &macMenu{}

type macMenu struct {
	factory *macMenuFactory
	id      int
	menu    ns.Menu
}

func (m *macMenu) Factory() MenuFactory {
	return m.factory
}

func (m *macMenu) IsSame(other Menu) bool {
	if m2, ok := other.(*macMenu); ok {
		return m.menu.Equals(m2.menu)
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
	m.insert(ns.NewSeparatorMenuItem(), atIndex)
}

func (m *macMenu) InsertItem(atIndex int, mi MenuItem) {
	if mi != nil {
		m.insert(mi.(*macMenuItem).item, atIndex)
	}
}

func (m *macMenu) InsertMenu(atIndex int, subMenu Menu) {
	m.insertMenu(atIndex, subMenu.(*macMenu))
}

func (m *macMenu) insertMenu(atIndex int, subMenu *macMenu) {
	m.insert(newMacMenuItemForSubMenu(m.factory, subMenu), atIndex)
	switch subMenu.id {
	case AppMenuID:
		if servicesItem := m.Item(ServicesMenuID); servicesItem != nil {
			if servicesMenu := servicesItem.SubMenu(); servicesMenu != nil {
				ns.App().SetServicesMenu(servicesMenu.(*macMenu).menu)
			}
		}
	case WindowMenuID:
		ns.App().SetWindowsMenu(subMenu.menu)
	case HelpMenuID:
		ns.App().SetHelpMenu(subMenu.menu)
	}
}

func (m *macMenu) RemoveItem(index int) {
	if index >= 0 && index < m.Count() {
		m.menu.RemoveItemAtIndex(index)
	}
}

func (m *macMenu) ID() int {
	return m.id
}

func (m *macMenu) Title() string {
	return m.menu.Title()
}

func (m *macMenu) Count() int {
	return m.menu.NumberOfItems()
}

func (m *macMenu) Popup(where geom32.Rect, itemIndex int) {
	w := ActiveWindow()
	if w.IsValid() {
		if mi := m.ItemAtIndex(itemIndex); mi != nil {
			view := ns.Window{Object: objc.ObjectPtr(uintptr(w.wnd.GetCocoaWindow()))}.ContentView()
			frame := view.Frame()
			where.X += 8
			where.Y = float32(frame.Size.Height) - where.Bottom()
			cell := ns.NewPopupButtonCell("", false)
			cell.SetAutoEnablesItems(false)
			cell.SetAltersStateOfSelectedItem(false)
			cell.SetMenu(m.menu)
			cell.SelectItem(mi.(*macMenuItem).item)
			cell.PerformClickWithFrameInView(ns.MakeRect(float64(where.X), float64(where.Y), float64(where.Width),
				float64(where.Height)), view)
			cell.Release()
		}
	}
}

func (m *macMenu) Dispose() {
	m.menu.Release()
}

func (m *macMenu) insert(mi ns.MenuItem, index int) {
	if index < 0 {
		index = m.Count()
	}
	m.menu.InsertItemAtIndex(mi, index)
}
