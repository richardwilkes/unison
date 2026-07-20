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
	"github.com/richardwilkes/toolbox/v2/geom"
)

// Menu is a handle to an NSMenu. NewMenu returns an owned reference; balance it with Release. Matching the cgo
// bridge, the reference NewMenu hands back is doubly-retained (alloc/init plus an extra retain), so Release never
// actually deallocates a menu — kept for bring-up parity with the old Objective-C code, which protected AppKit's
// menu-tracking machinery from a Dispose arriving while a menu is still on screen.
type Menu objc.ID

// menuUpdaters holds the updater functions registered via NewMenu, keyed by menu. It is only accessed from the main
// thread (menus are created, updated, and released from the event loop), matching the cgo bridge.
var menuUpdaters = make(map[Menu]func(Menu))

var (
	menuDelegateOnce     sync.Once
	menuDelegateInstance objc.ID
	menuDelegateErr      error
)

// menuDelegate returns the shared MenuDelegate instance, which routes each menu's menuNeedsUpdate: delegate message
// (sent by AppKit at the start of menu tracking) to the updater registered for that menu. The class is registered and
// the instance created on first use; on registration failure it returns 0 with the error logged, leaving menus
// functional but without dynamic updates.
func menuDelegate() objc.ID {
	menuDelegateOnce.Do(func() {
		LoadAppKit()
		var protocols []*objc.Protocol
		if p := objc.GetProtocol("NSMenuDelegate"); p != nil {
			protocols = append(protocols, p)
		}
		cls, err := objc.RegisterClass("MenuDelegate", Cls("NSObject"), protocols, nil, []objc.MethodDef{
			{
				Cmd: Sel("menuNeedsUpdate:"),
				Fn: func(_ objc.ID, _ objc.SEL, menu objc.ID) {
					m := Menu(menu)
					if updater, ok := menuUpdaters[m]; ok && updater != nil {
						updater(m)
					}
				},
			},
		})
		if err != nil {
			menuDelegateErr = errs.NewWithCause("unable to register MenuDelegate class", err)
			return
		}
		menuDelegateInstance = objc.ID(cls).Send(Sel("new"))
	})
	if menuDelegateErr != nil {
		errs.Log(menuDelegateErr)
	}
	return menuDelegateInstance
}

// NewMenu returns a new menu with the given title. If updater is non-nil, it is invoked when AppKit asks the menu to
// update itself just before it is displayed.
func NewMenu(title string, updater func(Menu)) Menu {
	titleStr := NewNSString(title)
	m := Retain(objc.ID(Cls("NSMenu")).Send(Sel("alloc")).Send(Sel("initWithTitle:"), titleStr))
	Release(titleStr)
	if delegate := menuDelegate(); delegate != 0 {
		m.Send(Sel("setDelegate:"), delegate)
	}
	menu := Menu(m)
	if updater != nil {
		menuUpdaters[menu] = updater
	}
	return menu
}

// NumberOfItems returns the number of items in the menu, including separators.
func (m Menu) NumberOfItems() int {
	return int(objc.Send[int64](objc.ID(m), Sel("numberOfItems")))
}

// ItemAtIndex returns the menu item at the given index.
func (m Menu) ItemAtIndex(index int) MenuItem {
	return MenuItem(objc.ID(m).Send(Sel("itemAtIndex:"), int64(index)))
}

// Supermenu returns the menu's parent menu, or 0 if it has none.
func (m Menu) Supermenu() Menu {
	return Menu(objc.ID(m).Send(Sel("supermenu")))
}

// InsertItemAtIndex inserts the given menu item at the given index.
func (m Menu) InsertItemAtIndex(item MenuItem, index int) {
	objc.ID(m).Send(Sel("insertItem:atIndex:"), objc.ID(item), int64(index))
}

// RemoveItemAtIndex removes the menu item at the given index.
func (m Menu) RemoveItemAtIndex(index int) {
	objc.ID(m).Send(Sel("removeItemAtIndex:"), int64(index))
}

// RemoveAll removes all items from the menu.
func (m Menu) RemoveAll() {
	objc.ID(m).Send(Sel("removeAllItems"))
}

// Title returns the menu's title.
func (m Menu) Title() string {
	var title string
	WithPool(func() {
		title = GoStringFromNSString(objc.ID(m).Send(Sel("title")))
	})
	return title
}

// Popup shows the given menu as a popup over the given window's content view, with the given item selected and the
// menu positioned within bounds (in the view's coordinate system). It blocks until the menu is dismissed. The
// receiver is ignored, matching the cgo bridge, which passed the menu to show as a parameter.
func (m Menu) Popup(wnd Window, menu Menu, item MenuItem, bounds geom.Rect) {
	WithPool(func() {
		// popUpMenuPositioningItem:atLocation:inView: is not being used here because it fails to work when a modal
		// dialog is being used.
		title := NewNSString("")
		cell := Retain(objc.ID(Cls("NSPopUpButtonCell")).Send(Sel("alloc")).Send(Sel("initTextCell:pullsDown:"),
			title, false))
		Release(title)
		cell.Send(Sel("setAutoenablesItems:"), false)
		cell.Send(Sel("setAltersStateOfSelectedItem:"), false)
		cell.Send(Sel("setMenu:"), objc.ID(menu))
		cell.Send(Sel("selectItem:"), objc.ID(item))
		cell.Send(Sel("performClickWithFrame:inView:"), NSRectFromRect(bounds), objc.ID(wnd.ContentView()))
		// Matching the cgo bridge, the cell was created with alloc/init plus an extra retain and only released once
		// here, so one reference remains after every popup (pre-existing leak, kept for bring-up parity).
		Release(cell)
	})
}

// Release removes the menu's updater and its items' validators and handlers, then releases the menu. Because of the
// extra retain NewMenu holds (see the Menu type comment), the underlying NSMenu remains alive afterward; the handle
// must nevertheless not be used again.
func (m Menu) Release() {
	delete(menuUpdaters, m)
	for i := m.NumberOfItems() - 1; i >= 0; i-- {
		item := m.ItemAtIndex(i)
		delete(menuItemValidators, item)
		delete(menuItemHandlers, item)
	}
	Release(objc.ID(m))
}
