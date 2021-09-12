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
	"github.com/richardwilkes/unison/internal/ns"
)

type macMenuFactory struct {
	bar                    *macMenu
	menuDelegate           objc.Object
	menuUpdaterMap         map[uintptr]func(Menu)
	menuItemDelegate       objc.Object
	menuItemValidatorMap   map[int]func(MenuItem) bool
	menuItemHandlerMap     map[int]func(MenuItem)
	handleMenuItemSelector objc.Selector
}

func platformNewDefaultMenuFactory() MenuFactory {
	f := &macMenuFactory{
		menuUpdaterMap:         make(map[uintptr]func(Menu)),
		menuItemValidatorMap:   make(map[int]func(MenuItem) bool),
		menuItemHandlerMap:     make(map[int]func(MenuItem)),
		handleMenuItemSelector: objc.Sel("handleMenuItem:"),
	}
	cls := objc.NewClass("MenuDelegate", "NSObject")
	cls.AddMethod("menuNeedsUpdate:", func(_, obj objc.Object) {
		m := ns.Menu{Object: obj}
		if updater, ok := f.menuUpdaterMap[m.Pointer()]; ok {
			updater(&macMenu{factory: f, menu: m})
		}
	})
	f.menuDelegate = cls.Alloc().Init()
	cls = objc.NewClass("MenuItemDelegate", "NSObject")
	cls.AddMethod("validateMenuItem:", func(_, obj objc.Object) bool {
		mi := ns.MenuItem{Object: obj}
		if validator, ok := f.menuItemValidatorMap[mi.Tag()]; ok {
			return validator(&macMenuItem{factory: f, item: mi})
		}
		return true
	})
	cls.AddMethod("handleMenuItem:", func(_, obj objc.Object) {
		mi := ns.MenuItem{Object: obj}
		if handler, ok := f.menuItemHandlerMap[mi.Tag()]; ok {
			handler(&macMenuItem{factory: f, item: mi})
		}
	})
	f.menuItemDelegate = cls.Alloc().Init()
	return f
}

func (f *macMenuFactory) BarForWindow(window *Window, initializer func(Menu)) Menu {
	if f.bar == nil {
		f.bar = f.newMenu(RootMenuID, "", nil)
		initializer(f.bar)
		InvokeTask(func() { ns.App().SetMainMenu(f.bar.menu) })
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
	m := ns.NewMenu(title)
	m.SetDelegate(f.menuDelegate)
	if updater != nil {
		f.menuUpdaterMap[m.Pointer()] = updater
	}
	return &macMenu{
		factory: f,
		id:      id,
		menu:    m,
	}
}

func (f *macMenuFactory) NewItem(id int, title string, keyCode KeyCode, keyModifiers Modifiers, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	mi := ns.NewMenuItem(title, f.handleMenuItemSelector, macKeyCodeToMenuEquivalentMap[keyCode])
	mi.SetTag(id)
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
	mi.SetKeyEquivalentModifierMask(mods)
	mi.SetTarget(f.menuItemDelegate)
	if validator != nil {
		f.menuItemValidatorMap[id] = validator
	} else {
		delete(f.menuItemValidatorMap, id)
	}
	if handler != nil {
		f.menuItemHandlerMap[id] = handler
	} else {
		delete(f.menuItemHandlerMap, id)
	}
	return &macMenuItem{
		factory: f,
		item:    mi,
	}
}
