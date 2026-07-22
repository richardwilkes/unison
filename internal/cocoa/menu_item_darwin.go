// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
)

// ControlStateValue mirrors AppKit's NSControlStateValue (an NSInteger).
type ControlStateValue int

// Possible ControlStateValue values.
const (
	ControlStateValueMixed ControlStateValue = iota - 1
	ControlStateValueOff
	ControlStateValueOn
)

// MenuItem is a handle to an NSMenuItem. NewMenuItem and NewSeparatorMenuItem return owned (+1) references. Inserting
// an item into a menu transfers that reference to the menu (see Menu.InsertItemAtIndex), which from then on cleans the
// item up when it is removed or the menu is released; only an item that is never inserted needs its own Release.
type MenuItem objc.ID

// menuItemValidators and menuItemHandlers hold the functions registered via NewMenuItem, keyed by item. They are
// only accessed from the main thread (menu items are created, validated, invoked, and cleaned up from the event
// loop), matching the cgo bridge.
var (
	menuItemValidators = make(map[MenuItem]func(item MenuItem) bool)
	menuItemHandlers   = make(map[MenuItem]func(item MenuItem))
)

var (
	menuItemDelegateOnce     sync.Once
	menuItemDelegateInstance objc.ID
	menuItemDelegateErr      error
)

// menuItemDelegate returns the shared macMenuItemDelegate instance, the target of every item created by NewMenuItem. It
// implements handleMenuItem: (the items' action) routing to the registered handler, and NSMenuItemValidation's
// validateMenuItem: routing to the registered validator, so AppKit's menu auto-enablement asks it about each item.
// The class is registered and the instance created on first use; on registration failure it returns 0 with the error
// logged, leaving items constructible but permanently disabled (no target).
func menuItemDelegate() objc.ID {
	menuItemDelegateOnce.Do(func() {
		LoadAppKit()
		var protocols []*objc.Protocol
		if p := objc.GetProtocol("NSMenuItemValidation"); p != nil {
			protocols = append(protocols, p)
		}
		cls, err := objc.RegisterClass("macMenuItemDelegate", Cls("NSObject"), protocols, nil, []objc.MethodDef{
			{
				Cmd: Sel("validateMenuItem:"),
				Fn: func(_ objc.ID, _ objc.SEL, menuItem objc.ID) bool {
					item := MenuItem(menuItem)
					if validator, ok := menuItemValidators[item]; ok && validator != nil {
						return validator(item)
					}
					return true
				},
			},
			{
				Cmd: Sel("handleMenuItem:"),
				Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) {
					item := MenuItem(sender)
					if handler, ok := menuItemHandlers[item]; ok && handler != nil {
						handler(item)
					}
				},
			},
		})
		if err != nil {
			menuItemDelegateErr = errs.NewWithCause("unable to register macMenuItemDelegate class", err)
			return
		}
		menuItemDelegateInstance = objc.ID(cls).Send(Sel("new"))
	})
	if menuItemDelegateErr != nil {
		errs.Log(menuItemDelegateErr)
	}
	return menuItemDelegateInstance
}

// NewMenuItem returns a new menu item as an owned (+1) reference. The item's action is routed through the shared
// macMenuItemDelegate to the given handler, and AppKit's menu validation to the given validator; a nil validator leaves
// the item always enabled.
func NewMenuItem(tag int, title, keyEquivalent string, modifiers EventModifierFlags, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	titleStr := NewNSString(title)
	keyStr := NewNSString(keyEquivalent)
	item := objc.ID(Cls("NSMenuItem")).Send(Sel("alloc")).Send(Sel("initWithTitle:action:keyEquivalent:"),
		titleStr, Sel("handleMenuItem:"), keyStr)
	Release(keyStr)
	Release(titleStr)
	item.Send(Sel("setTag:"), int64(tag))
	item.Send(Sel("setKeyEquivalentModifierMask:"), uint64(modifiers))
	if delegate := menuItemDelegate(); delegate != 0 {
		item.Send(Sel("setTarget:"), delegate)
	}
	mi := MenuItem(item)
	if validator != nil {
		menuItemValidators[mi] = validator
	}
	if handler != nil {
		menuItemHandlers[mi] = handler
	}
	return mi
}

// NewSeparatorMenuItem returns a new separator menu item as an owned (+1) reference (AppKit's separatorItem is
// autoreleased, so the retain converts it to the same ownership NewMenuItem hands back).
func NewSeparatorMenuItem() MenuItem {
	var item objc.ID
	WithPool(func() {
		item = Retain(objc.ID(Cls("NSMenuItem")).Send(Sel("separatorItem")))
	})
	return MenuItem(item)
}

// Tag returns the menu item's tag.
func (m MenuItem) Tag() int {
	return int(objc.Send[int64](objc.ID(m), Sel("tag")))
}

// IsSeparatorItem returns true if the menu item is a separator.
func (m MenuItem) IsSeparatorItem() bool {
	return objc.Send[bool](objc.ID(m), Sel("isSeparatorItem"))
}

// Title returns the menu item's title.
func (m MenuItem) Title() string {
	var title string
	WithPool(func() {
		title = GoStringFromNSString(objc.ID(m).Send(Sel("title")))
	})
	return title
}

// SetTitle sets the menu item's title.
func (m MenuItem) SetTitle(title string) {
	titleStr := NewNSString(title)
	objc.ID(m).Send(Sel("setTitle:"), titleStr)
	Release(titleStr)
}

// KeyBinding returns the menu item's key equivalent and required modifiers.
func (m MenuItem) KeyBinding() (keyEquivalent string, modifiers EventModifierFlags) {
	WithPool(func() {
		keyEquivalent = GoStringFromNSString(objc.ID(m).Send(Sel("keyEquivalent")))
	})
	return keyEquivalent, EventModifierFlags(objc.Send[uint64](objc.ID(m), Sel("keyEquivalentModifierMask")))
}

// SetKeyBinding sets the menu item's key equivalent and required modifiers.
func (m MenuItem) SetKeyBinding(keyEquivalent string, modifiers EventModifierFlags) {
	keyStr := NewNSString(keyEquivalent)
	objc.ID(m).Send(Sel("setKeyEquivalent:"), keyStr)
	Release(keyStr)
	objc.ID(m).Send(Sel("setKeyEquivalentModifierMask:"), uint64(modifiers))
}

// Menu returns the menu the item belongs to, or 0 if it has not been inserted into one.
func (m MenuItem) Menu() Menu {
	return Menu(objc.ID(m).Send(Sel("menu")))
}

// SubMenu returns the item's submenu, or 0 if it has none.
func (m MenuItem) SubMenu() Menu {
	return Menu(objc.ID(m).Send(Sel("submenu")))
}

// SetSubMenu sets the item's submenu, transferring ownership of the caller's reference to the item: the item retains
// the submenu and the owned reference returned by NewMenu is released here. The submenu handle remains usable for as
// long as the item keeps the submenu; releasing the root of the tree it belongs to cleans it up.
func (m MenuItem) SetSubMenu(menu Menu) {
	objc.ID(m).Send(Sel("setSubmenu:"), objc.ID(menu))
	Release(objc.ID(menu))
}

// Release releases a menu item that was never inserted into a menu, removing its validator and handler registrations
// along with those of any submenu tree attached to it. Items that have been inserted must not be released: the menu
// owns them and cleans them up when they are removed or when it is released.
func (m MenuItem) Release() {
	m.forgetRegistrations()
	Release(objc.ID(m))
}

// forgetRegistrations removes the item's validator and handler registrations and, if the item has a submenu, the
// registrations of that entire submenu tree.
func (m MenuItem) forgetRegistrations() {
	delete(menuItemValidators, m)
	delete(menuItemHandlers, m)
	if sub := m.SubMenu(); sub != 0 {
		sub.forgetRegistrations()
	}
}

// State returns the menu item's state (its checked/unchecked/mixed mark).
func (m MenuItem) State() ControlStateValue {
	return ControlStateValue(objc.Send[int64](objc.ID(m), Sel("state")))
}

// SetState sets the menu item's state.
func (m MenuItem) SetState(state ControlStateValue) {
	objc.ID(m).Send(Sel("setState:"), int64(state))
}
