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
	"github.com/richardwilkes/toolbox/log/jot"
)

var (
	defaultMenuFactory MenuFactory
	_                  MenuFactory = &inWindowMenuFactory{}
)

// MenuFactory provides methods for creating a menu bar and its menus.
type MenuFactory interface {
	// BarForWindow returns the menu bar for the given window. If this is the first time the menu bar has been returned
	// from this call, initializer will be called so that your code can configure the menus.
	BarForWindow(window *Window, initializer func(Menu)) Menu
	// BarIsPerWindow returns true if the menu bar returned from this MenuFactory is per-window instead of global.
	BarIsPerWindow() bool
	// NewMenu creates a new Menu. updater is optional and, if present, will be called prior to showing the Menu, giving
	// a chance to modify it.
	NewMenu(id int, title string, updater func(Menu)) Menu
	// NewItem creates a new MenuItem. Both validator and handler may be nil for default behavior. If keyCode is 0, no key
	// accelerator will be attached to the menuItem.
	NewItem(id int, title string, keyCode KeyCode, keyModifiers Modifiers, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem
}

// DefaultMenuFactory returns the default MenuFactory for the platform. Multiple calls always return the same object.
func DefaultMenuFactory() MenuFactory {
	if defaultMenuFactory == nil {
		if noGlobalMenuBar {
			defaultMenuFactory = NewInWindowMenuFactory()
		} else {
			defaultMenuFactory = platformNewDefaultMenuFactory()
		}
	}
	return defaultMenuFactory
}

type inWindowMenuFactory struct {
	wndBarMap      map[*Window]*menu
	wndRootMenuMap map[*Window]*menu
	showInProgress bool
}

// NewInWindowMenuFactory creates a new MenuFactory for in-window usage. This is the fallback Go-only version of menus
// used when a non-platform-native version doesn't exist.
// TODO: Consider doing something similar to Java and only create native windows for menu content when absolutely
//       necessary. While the current approach works extremely well on macOS, it is slow on both Windows and Linux,
//       thanks to gratuitous animations upon window creation that I don't see a way to disable.
func NewInWindowMenuFactory() MenuFactory {
	return &inWindowMenuFactory{
		wndBarMap:      make(map[*Window]*menu),
		wndRootMenuMap: make(map[*Window]*menu),
	}
}

func (f *inWindowMenuFactory) BarForWindow(window *Window, initializer func(Menu)) Menu {
	b, exists := f.wndBarMap[window]
	if exists {
		return b
	}
	b = f.NewMenu(RootMenuID, "", nil).(*menu) //nolint:errcheck // Can only be used with this type
	initializer(b)
	f.wndBarMap[window] = b
	if holder := barHolderFromWindow(window); holder != nil {
		holder.SetMenuBar(b.newPanel(true), b.preMoved, b.postLostFocus, b.preMouseDown, b.preKeyDown, b.preKeyUp, b.preRuneTyped)
	} else {
		// This should be impossible
		jot.Error("unable to obtain menu bar holder")
	}
	return b
}

func (f *inWindowMenuFactory) BarIsPerWindow() bool {
	return true
}

func (f *inWindowMenuFactory) NewMenu(id int, title string, updater func(Menu)) Menu {
	m := &menu{
		factory: f,
		titleItem: &menuItem{
			factory: f,
			id:      id,
			title:   title,
		},
		updater: updater,
	}
	m.titleItem.subMenu = m
	return m
}

func (f *inWindowMenuFactory) NewItem(id int, title string, keyCode KeyCode, keyModifiers Modifiers, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	return &menuItem{
		factory:      f,
		id:           id,
		title:        title,
		validator:    validator,
		handler:      handler,
		keyCode:      keyCode,
		keyModifiers: keyModifiers,
	}
}

func (f *inWindowMenuFactory) activeMenuList(wnd *Window) []*menu {
	root := f.wndRootMenuMap[wnd]
	if root == nil {
		return nil
	}
	return root.collectActiveMenus(nil)
}
