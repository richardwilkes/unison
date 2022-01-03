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
	"github.com/richardwilkes/unison/internal/ns"
)

type macMenuFactory struct {
	bar *macMenu
}

func platformNewDefaultMenuFactory() MenuFactory {
	return &macMenuFactory{}
}

func (f *macMenuFactory) BarForWindow(window *Window, initializer func(Menu)) Menu {
	if f.bar == nil {
		f.bar = f.newMenu(RootMenuID, "", nil)
		initializer(f.bar)
		InvokeTask(func() { ns.SetMainMenu(f.bar.menu) })
	}
	return f.bar
}

func (f *macMenuFactory) BarIsPerWindow() bool {
	return false
}

func (f *macMenuFactory) NewMenu(id int, title string, updater func(Menu)) Menu {
	return f.newMenu(id, title, updater)
}

func (f *macMenuFactory) newMenu(id int, title string, updater func(Menu)) *macMenu {
	var u func(ns.Menu)
	if updater != nil {
		u = func(m ns.Menu) {
			updater(&macMenu{factory: f, menu: m})
		}
	}
	m := ns.NewMenu(title, u)
	return &macMenu{
		factory: f,
		id:      id,
		menu:    m,
	}
}

func (f *macMenuFactory) NewItem(id int, title string, keyCode KeyCode, keyModifiers Modifiers, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	var mods ns.EventModifierFlags
	if keyModifiers.ShiftDown() {
		mods |= ns.EventModifierFlagShift
	}
	if keyModifiers.OptionDown() {
		mods |= ns.EventModifierFlagOption
	}
	if keyModifiers.CommandDown() {
		mods |= ns.EventModifierFlagCommand
	}
	if keyModifiers.ControlDown() {
		mods |= ns.EventModifierFlagControl
	}
	if keyModifiers.CapsLockDown() {
		mods |= ns.EventModifierFlagCapsLock
	}
	var v func(ns.MenuItem) bool
	if validator != nil {
		v = func(mi ns.MenuItem) bool {
			return validator(&macMenuItem{factory: f, item: mi})
		}
	}
	var h func(ns.MenuItem)
	if handler != nil {
		h = func(mi ns.MenuItem) {
			handler(&macMenuItem{factory: f, item: mi})
		}
	}
	mi := ns.NewMenuItem(id, title, macKeyCodeToMenuEquivalentMap[keyCode], mods, v, h)
	return &macMenuItem{
		factory: f,
		item:    mi,
	}
}
