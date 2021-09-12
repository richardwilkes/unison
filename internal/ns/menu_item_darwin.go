// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

import "github.com/progrium/macdriver/objc"

var menuItemClass = objc.Get("NSMenuItem")

// EventModifierFlags https://developer.apple.com/documentation/appkit/nseventmodifierflags?language=objc
type EventModifierFlags uint

// https://developer.apple.com/documentation/appkit/nseventmodifierflags?language=objc
const (
	EventModifierFlagCapsLock EventModifierFlags = 1 << (16 + iota)
	EventModifierFlagShift
	EventModifierFlagControl
	EventModifierFlagOption
	EventModifierFlagCommand
	EventModifierFlagNumericPad
	EventModifierFlagHelp
	EventModifierFlagFunction
)

// ControlStateValue https://developer.apple.com/documentation/appkit/nscontrolstatevalue?language=objc
type ControlStateValue int

// https://developer.apple.com/documentation/appkit/nscontrolstatevalue?language=objc
const (
	ControlStateValueMixed ControlStateValue = iota - 1
	ControlStateValueOff
	ControlStateValueOn
)

// MenuItem https://developer.apple.com/documentation/appkit/nsmenuitem?language=objc
type MenuItem struct {
	objc.Object
}

// NewMenuItem https://developer.apple.com/documentation/appkit/nsmenuitem/1514858-initwithtitle?language=objc
func NewMenuItem(title string, action objc.Selector, keyEquivalent string) MenuItem {
	titleStr := StringFromString(title)
	defer titleStr.Release()
	keyStr := StringFromString(keyEquivalent)
	defer keyStr.Release()
	obj := menuItemClass.Alloc()
	obj.Send("initWithTitle:action:keyEquivalent:", titleStr, action, keyStr)
	obj.Retain()
	return MenuItem{Object: obj}
}

// NewSeparatorMenuItem https://developer.apple.com/documentation/appkit/nsmenuitem/1514838-separatoritem?language=objc
func NewSeparatorMenuItem() MenuItem {
	return MenuItem{Object: menuItemClass.Send("separatorItem").Retain()}
}

// Tag https://developer.apple.com/documentation/appkit/nsmenuitem/1514840-tag?language=objc
func (m MenuItem) Tag() int {
	return int(m.Send("tag").Int())
}

// SetTag https://developer.apple.com/documentation/appkit/nsmenuitem/1514840-tag?language=objc
func (m MenuItem) SetTag(tag int) {
	m.Send("setTag:", tag)
}

// SetKeyEquivalentModifierMask https://developer.apple.com/documentation/appkit/nsmenuitem/1514815-keyequivalentmodifiermask?language=objc
func (m MenuItem) SetKeyEquivalentModifierMask(modifiers EventModifierFlags) {
	m.Send("setKeyEquivalentModifierMask:", modifiers)
}

// SetTarget https://developer.apple.com/documentation/appkit/nsmenuitem/1514843-target?language=objc
func (m MenuItem) SetTarget(target objc.Object) {
	m.Send("setTarget:", target)
}

// IsSeparatorItem https://developer.apple.com/documentation/appkit/nsmenuitem/1514837-separatoritem?language=objc
func (m MenuItem) IsSeparatorItem() bool {
	return m.Send("isSeparatorItem").Bool()
}

// Title https://developer.apple.com/documentation/appkit/nsmenuitem/1514805-title?language=objc
func (m MenuItem) Title() string {
	return m.Send("title").String()
}

// SetTitle https://developer.apple.com/documentation/appkit/nsmenuitem/1514805-title?language=objc
func (m MenuItem) SetTitle(title string) {
	titleStr := StringFromString(title)
	defer titleStr.Release()
	m.Send("setTitle:", titleStr)
}

// Menu https://developer.apple.com/documentation/appkit/nsmenuitem/1514830-menu?language=objc
func (m MenuItem) Menu() Menu {
	return Menu{Object: m.Send("menu")}
}

// SubMenu https://developer.apple.com/documentation/appkit/nsmenuitem/1514845-submenu?language=objc
func (m MenuItem) SubMenu() Menu {
	return Menu{Object: m.Send("submenu")}
}

// SetSubMenu https://developer.apple.com/documentation/appkit/nsmenuitem/1514845-submenu?language=objc
func (m MenuItem) SetSubMenu(menu Menu) {
	m.Send("setSubmenu:", menu)
}

// State https://developer.apple.com/documentation/appkit/nsmenuitem/1514804-state?language=objc
func (m MenuItem) State() ControlStateValue {
	return ControlStateValue(m.Send("state").Int())
}

// SetState https://developer.apple.com/documentation/appkit/nsmenuitem/1514804-state?language=objc
func (m MenuItem) SetState(state ControlStateValue) {
	m.Send("setState:", int(state))
}
