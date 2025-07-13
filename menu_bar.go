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
	"fmt"

	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/check"
)

// Pre-defined menu IDs. Apps should start their IDs at UserBaseID.
const (
	RootMenuID int = 1 + iota
	AppMenuID
	FileMenuID
	EditMenuID
	WindowMenuID
	HelpMenuID
	ServicesMenuID
	AboutItemID
	PreferencesItemID
	QuitItemID
	CutItemID
	CopyItemID
	PasteItemID
	DeleteItemID
	SelectAllItemID
	MinimizeItemID
	ZoomItemID
	BringAllWindowsToFrontItemID
	CloseItemID
	HideItemID
	HideOthersItemID
	ShowAllItemID
	WindowMenuItemBaseID
	PopupMenuTemporaryBaseID = WindowMenuItemBaseID + maxWindowsListed
	UserBaseID               = 5000
	ContextMenuIDFlag        = 1 << 15 // Should be or'd into IDs for context menus
	MaxUserBaseID            = ContextMenuIDFlag - 1
	maxWindowsListed         = 100
)

// InsertStdMenus adds the standard menus to the menu bar.
func InsertStdMenus(m Menu, aboutHandler, prefsHandler func(MenuItem), updater func(Menu)) {
	f := m.Factory()
	if !f.BarIsPerWindow() {
		m.InsertMenu(-1, NewAppMenu(f, aboutHandler, prefsHandler, updater))
	}
	m.InsertMenu(-1, NewFileMenu(f, updater))
	m.InsertMenu(-1, NewEditMenu(f, prefsHandler, updater))
	m.InsertMenu(-1, NewWindowMenu(f, updater))
	m.InsertMenu(-1, NewHelpMenu(f, aboutHandler, updater))
}

// NewAppMenu creates a standard 'App' menu. Really only intended for macOS, although other platforms can use it if
// desired.
func NewAppMenu(f MenuFactory, aboutHandler, prefsHandler func(MenuItem), updater func(Menu)) Menu {
	m := f.NewMenu(AppMenuID, xos.AppName, updater)
	InsertAboutItem(m, -1, aboutHandler)
	m.InsertSeparator(-1, false)
	InsertPreferencesItem(m, -1, prefsHandler)
	platformAddAppMenuEntries(m)
	m.InsertSeparator(-1, true)
	InsertQuitItem(m, -1)
	return m
}

// InsertAboutItem creates the standard "About" menu item that will call the provided handler when chosen.
func InsertAboutItem(m Menu, atIndex int, aboutHandler func(MenuItem)) {
	m.InsertItem(atIndex, m.Factory().NewItem(AboutItemID, fmt.Sprintf(i18n.Text("About %s"), xos.AppName), KeyBinding{},
		func(MenuItem) bool { return aboutHandler != nil }, aboutHandler))
}

// NewFileMenu creates a standard 'File' menu.
func NewFileMenu(f MenuFactory, updater func(Menu)) Menu {
	m := f.NewMenu(FileMenuID, i18n.Text("File"), updater)
	InsertCloseFocusedWindowItem(m, -1)
	if f.BarIsPerWindow() {
		m.InsertSeparator(-1, false)
		InsertQuitItem(m, -1)
	}
	return m
}

// InsertCloseFocusedWindowItem creates the standard "Close" menu item that will close the currently focused window when
// chosen.
func InsertCloseFocusedWindowItem(m Menu, atIndex int) {
	m.InsertItem(atIndex, m.Factory().NewItem(CloseItemID, i18n.Text("Close"),
		KeyBinding{KeyCode: KeyW, Modifiers: OSMenuCmdModifier()},
		func(MenuItem) bool { return ActiveWindow() != nil },
		func(MenuItem) {
			if wnd := ActiveWindow(); wnd != nil {
				wnd.AttemptClose()
			}
		}))
}

// InsertQuitItem creates the standard "Quit"/"Exit" menu item that will issue the Quit command when chosen.
func InsertQuitItem(m Menu, atIndex int) {
	m.InsertItem(atIndex, m.Factory().NewItem(QuitItemID, quitMenuTitle(),
		KeyBinding{KeyCode: KeyQ, Modifiers: OSMenuCmdModifier()},
		nil,
		func(MenuItem) { AttemptQuit() }))
}

// NewEditMenu creates a standard 'Edit' menu.
func NewEditMenu(f MenuFactory, prefsHandler func(MenuItem), updater func(Menu)) Menu {
	m := f.NewMenu(EditMenuID, i18n.Text("Edit"), updater)
	m.InsertItem(-1, CutAction().NewMenuItem(f))
	m.InsertItem(-1, CopyAction().NewMenuItem(f))
	m.InsertItem(-1, PasteAction().NewMenuItem(f))
	m.InsertItem(-1, DeleteAction().NewMenuItem(f))
	m.InsertItem(-1, SelectAllAction().NewMenuItem(f))
	if prefsHandler != nil && f.BarIsPerWindow() {
		m.InsertSeparator(-1, false)
		InsertPreferencesItem(m, -1, prefsHandler)
	}
	return m
}

// InsertPreferencesItem creates the standard "Preferences…" menu item that will call the provided handler when chosen.
func InsertPreferencesItem(m Menu, atIndex int, prefsHandler func(MenuItem)) {
	m.InsertItem(atIndex, m.Factory().NewItem(PreferencesItemID, i18n.Text("Preferences…"),
		KeyBinding{KeyCode: KeyComma, Modifiers: OSMenuCmdModifier()},
		func(MenuItem) bool { return prefsHandler != nil }, prefsHandler))
}

// NewWindowMenu creates a standard 'Window' menu.
func NewWindowMenu(f MenuFactory, updater func(Menu)) Menu {
	if f.BarIsPerWindow() {
		if updater != nil {
			u := updater
			updater = func(m Menu) {
				windowListUpdater(m)
				u(m)
			}
		} else {
			updater = windowListUpdater
		}
	}
	m := f.NewMenu(WindowMenuID, i18n.Text("Window"), updater)
	InsertMinimizeItem(m, -1)
	InsertZoomItem(m, -1)
	m.InsertSeparator(-1, false)
	InsertBringAllToFrontItem(m, -1)
	return m
}

func windowListUpdater(m Menu) {
	if m.ID() == WindowMenuID {
		for i := m.Count() - 1; i >= 0; i-- {
			mi := m.ItemAtIndex(i)
			if !mi.IsSeparator() {
				if id := mi.ID(); id < WindowMenuItemBaseID || id >= PopupMenuTemporaryBaseID {
					break
				}
			}
			m.RemoveItem(i)
		}
		if len(windowList) != 0 {
			m.InsertSeparator(-1, false)
			id := WindowMenuItemBaseID
			f := m.Factory()
			active := ActiveWindow()
			for _, wnd := range windowList {
				m.InsertItem(-1, createSelectWindowMenuItem(f, id, wnd, active))
				id++
				if id >= PopupMenuTemporaryBaseID {
					break
				}
			}
		}
	}
}

func createSelectWindowMenuItem(f MenuFactory, id int, wnd, active *Window) MenuItem {
	enabled := active != wnd
	mi := f.NewItem(id, wnd.Title(), KeyBinding{},
		func(_ MenuItem) bool { return enabled },
		func(_ MenuItem) { wnd.ToFront() },
	)
	if active == wnd {
		mi.SetCheckState(check.On)
	}
	return mi
}

// InsertMinimizeItem creates the standard "Minimize" menu item that will issue the Minimize command to the currently
// focused window when chosen.
func InsertMinimizeItem(m Menu, atIndex int) {
	m.InsertItem(atIndex, m.Factory().NewItem(MinimizeItemID, i18n.Text("Minimize"),
		KeyBinding{KeyCode: KeyM, Modifiers: OSMenuCmdModifier()},
		func(MenuItem) bool { return ActiveWindow() != nil },
		func(MenuItem) {
			if wnd := ActiveWindow(); wnd != nil {
				wnd.Minimize()
			}
		}))
}

// InsertZoomItem creates the standard "Zoom" menu item that will issue the Zoom command to the currently focused window
// when chosen.
func InsertZoomItem(m Menu, atIndex int) {
	m.InsertItem(atIndex, m.Factory().NewItem(ZoomItemID, i18n.Text("Zoom"),
		KeyBinding{KeyCode: KeyZ, Modifiers: ShiftModifier | OSMenuCmdModifier()},
		func(MenuItem) bool {
			w := ActiveWindow()
			return w != nil && w.Resizable()
		},
		func(MenuItem) {
			if wnd := ActiveWindow(); wnd != nil {
				wnd.Zoom()
			}
		}))
}

// InsertBringAllToFrontItem creates the standard "Bring All to Front" menu item that will call AllWindowsToFront when
// chosen.
func InsertBringAllToFrontItem(m Menu, atIndex int) {
	m.InsertItem(atIndex, m.Factory().NewItem(BringAllWindowsToFrontItemID, i18n.Text("Bring All to Front"), KeyBinding{},
		func(MenuItem) bool { return WindowCount() > 0 }, func(MenuItem) { AllWindowsToFront() }))
}

// NewHelpMenu creates a standard 'Help' menu.
func NewHelpMenu(f MenuFactory, aboutHandler func(MenuItem), updater func(Menu)) Menu {
	m := f.NewMenu(HelpMenuID, i18n.Text("Help"), updater)
	if f.BarIsPerWindow() {
		InsertAboutItem(m, -1, aboutHandler)
	}
	return m
}
