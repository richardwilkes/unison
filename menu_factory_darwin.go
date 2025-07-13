// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/ns"
)

type macMenuFactory struct {
	bar *macMenu
}

func platformNewDefaultMenuFactory() MenuFactory {
	return &macMenuFactory{}
}

func (f *macMenuFactory) BarForWindowNoCreate(_ *Window) Menu {
	if f.bar == nil {
		return nil
	}
	return f.bar
}

func (f *macMenuFactory) BarForWindow(_ *Window, initializer func(Menu)) Menu {
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

func (f *macMenuFactory) NewItem(id int, title string, keyBinding KeyBinding, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	var h func(ns.MenuItem)
	if handler != nil {
		h = func(mi ns.MenuItem) {
			xos.SafeCall(func() { handler(&macMenuItem{factory: f, item: mi}) }, nil)
		}
	}
	mi := ns.NewMenuItem(id, title, macKeyCodeToMenuEquivalentMap[keyBinding.KeyCode],
		keyBinding.Modifiers.eventModifierFlags(), func(mi ns.MenuItem) bool {
			if DisableMenus {
				return false
			}
			if validator != nil {
				var result bool
				xos.SafeCall(func() { result = validator(&macMenuItem{factory: f, item: mi}) }, nil)
				return result
			}
			return true
		}, h)
	return &macMenuItem{
		factory: f,
		item:    mi,
	}
}
