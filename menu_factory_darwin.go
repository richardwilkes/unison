// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/unison/internal/cocoa"

type macMenuFactory struct {
	bar *macMenu
}

func apiNewDefaultMenuFactory() MenuFactory {
	return &macMenuFactory{}
}

func (f *macMenuFactory) BarForWindow(_ *Window, initializer func(Menu)) Menu {
	if f.bar == nil {
		f.bar = f.macNewMenu(RootMenuID, "", nil)
		initializer(f.bar)
		InvokeTask(func() { cocoa.SetMainMenu(f.bar.menu) })
	}
	return f.bar
}

func (f *macMenuFactory) BarForWindowNoCreate(_ *Window) Menu {
	if f.bar == nil {
		return nil
	}
	return f.bar
}

func (f *macMenuFactory) BarIsPerWindow() bool {
	return false
}

func (f *macMenuFactory) NewMenu(id int, title string, updater func(Menu)) Menu {
	return f.macNewMenu(id, title, updater)
}

func (f *macMenuFactory) macNewMenu(id int, title string, updater func(Menu)) *macMenu {
	m := cocoa.NewMenu(title, f.wrapUpdater(id, updater))
	return &macMenu{
		factory: f,
		id:      id,
		menu:    m,
		owned:   true,
	}
}

func (f *macMenuFactory) wrapUpdater(id int, updater func(Menu)) func(cocoa.Menu) {
	if updater == nil {
		return nil
	}
	return func(m cocoa.Menu) {
		updater(&macMenu{factory: f, id: id, menu: m})
	}
}

func (f *macMenuFactory) NewItem(id int, title string, keyBinding KeyBinding, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	var h func(cocoa.MenuItem)
	if handler != nil {
		h = func(mi cocoa.MenuItem) {
			SafeCall(func() { handler(&macMenuItem{factory: f, item: mi}) })
		}
	}
	mi := cocoa.NewMenuItem(id, title, macKeyCodeToMenuEquivalentMap[keyBinding.KeyCode],
		macEventModifierFlagsFromModifiers(keyBinding.Modifiers), func(mi cocoa.MenuItem) bool {
			if DisableMenus {
				return false
			}
			if validator != nil {
				var result bool
				SafeCall(func() { result = validator(&macMenuItem{factory: f, item: mi}) })
				return result
			}
			return true
		}, h)
	return &macMenuItem{
		factory: f,
		item:    mi,
	}
}
