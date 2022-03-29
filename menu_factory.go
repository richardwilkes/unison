// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

var (
	// DisableMenus overrides all application menus when set to true, causing them to become disabled. This is primarily
	// provided to allow a way to disable menu key capture temporarily. This will not allow system keys to be captured,
	// but will prevent the menus from capturing keys while it is true.
	DisableMenus       bool
	defaultMenuFactory MenuFactory
	_                  MenuFactory = &inWindowMenuFactory{}
)

// MenuFactory provides methods for creating a menu bar and its menus.
type MenuFactory interface {
	// BarForWindow returns the menu bar for the given window. If this is the first time the menu bar has been returned
	// from this call, initializer will be called so that your code can configure the menus.
	BarForWindow(window *Window, initializer func(Menu)) Menu
	// BarForWindowNoCreate returns the menu bar for the given window. May return nil if no menu bar for the window has
	// been created yet.
	BarForWindowNoCreate(window *Window) Menu
	// BarIsPerWindow returns true if the menu bar returned from this MenuFactory is per-window instead of global.
	BarIsPerWindow() bool
	// NewMenu creates a new Menu. updater is optional and, if present, will be called prior to showing the Menu, giving
	// a chance to modify it.
	NewMenu(id int, title string, updater func(Menu)) Menu
	// NewItem creates a new MenuItem. Both validator and handler may be nil for default behavior.
	NewItem(id int, title string, keyBinding KeyBinding, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem
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
	showInProgress bool
}

// NewInWindowMenuFactory creates a new MenuFactory for in-window usage. This is the fallback Go-only version of menus
// used when a non-platform-native version doesn't exist.
func NewInWindowMenuFactory() MenuFactory {
	return &inWindowMenuFactory{}
}

func (f *inWindowMenuFactory) BarForWindowNoCreate(window *Window) Menu {
	return window.root.menuBar
}

func (f *inWindowMenuFactory) BarForWindow(window *Window, initializer func(Menu)) Menu {
	if window.root.menuBar != nil {
		return window.root.menuBar
	}
	b := f.newMenu(RootMenuID, "", nil)
	initializer(b)
	window.root.setMenuBar(b)
	return b
}

func (f *inWindowMenuFactory) BarIsPerWindow() bool {
	return true
}

func (f *inWindowMenuFactory) NewMenu(id int, title string, updater func(Menu)) Menu {
	return f.newMenu(id, title, updater)
}

func (f *inWindowMenuFactory) newMenu(id int, title string, updater func(Menu)) *menu {
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

func (f *inWindowMenuFactory) NewItem(id int, title string, keyBinding KeyBinding, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	return &menuItem{
		factory:    f,
		id:         id,
		title:      title,
		validator:  validator,
		handler:    handler,
		keyBinding: keyBinding,
	}
}
