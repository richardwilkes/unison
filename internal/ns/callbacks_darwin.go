// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

/*
#import <Cocoa/Cocoa.h>

typedef CFTypeRef NSMenuRef;
typedef CFTypeRef NSMenuItemRef;
*/
import "C"

var (
	menuUpdaters               = make(map[Menu]func(Menu))
	menuItemValidators         = make(map[MenuItem]func(item MenuItem) bool)
	menuItemHandlers           = make(map[MenuItem]func(item MenuItem))
	openFilesCallback          func([]string)
	systemThemeChangedCallback func()
)

//export updateMenuCallback
func updateMenuCallback(m C.NSMenuRef) {
	menu := Menu(m)
	if updater, ok := menuUpdaters[menu]; ok && updater != nil {
		updater(menu)
	}
}

//export menuItemValidateCallback
func menuItemValidateCallback(mi C.NSMenuItemRef) bool {
	item := MenuItem(mi)
	if validator, ok := menuItemValidators[item]; ok && validator != nil {
		return validator(item)
	}
	return true
}

//export menuItemHandleCallback
func menuItemHandleCallback(mi C.NSMenuItemRef) {
	item := MenuItem(mi)
	if handler, ok := menuItemHandlers[item]; ok && handler != nil {
		handler(item)
	}
}

//export appOpenURLsCallback
func appOpenURLsCallback(a C.CFArrayRef) {
	if openFilesCallback != nil {
		if urls := Array(a).ArrayOfURLToStringSlice(); len(urls) > 0 {
			openFilesCallback(urls)
		}
	}
}

//export themeChangedCallback
func themeChangedCallback() {
	if systemThemeChangedCallback != nil {
		systemThemeChangedCallback()
	}
}
