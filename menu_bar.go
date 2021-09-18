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
	"fmt"
	"runtime"

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/toolbox/xmath/geom32"
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
	PopupMenuTemporaryBaseID
	ContextMenuIDFlag = 1 << 15 // Should be or'd into IDs for context menus
	UserBaseID        = 5000
	MaxUserBaseID     = ContextMenuIDFlag - 1
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
	m := f.NewMenu(AppMenuID, cmdline.AppName, updater)
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
	m.InsertItem(-1, m.Factory().NewItem(AboutItemID, fmt.Sprintf(i18n.Text("About %s"), cmdline.AppName), KeyNone,
		NoModifiers, func(MenuItem) bool { return aboutHandler != nil }, aboutHandler))
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
	m.InsertItem(atIndex, m.Factory().NewItem(CloseItemID, i18n.Text("Close"), KeyW, OSMenuCmdModifier(),
		func(MenuItem) bool { return ActiveWindow() != nil },
		func(MenuItem) {
			if wnd := ActiveWindow(); wnd != nil {
				wnd.AttemptClose()
			}
		}))
}

// InsertQuitItem creates the standard "Quit"/"Exit" menu item that will issue the Quit command when chosen.
func InsertQuitItem(m Menu, atIndex int) {
	var title string
	if runtime.GOOS == toolbox.MacOS {
		title = i18n.Text("Quit")
	} else {
		title = i18n.Text("Exit")
	}
	m.InsertItem(atIndex, m.Factory().NewItem(QuitItemID, title, KeyQ, OSMenuCmdModifier(), nil,
		func(MenuItem) { AttemptQuit() }))
}

// NewEditMenu creates a standard 'Edit' menu.
func NewEditMenu(f MenuFactory, prefsHandler func(MenuItem), updater func(Menu)) Menu {
	m := f.NewMenu(EditMenuID, i18n.Text("Edit"), updater)
	m.InsertItem(-1, CutAction.NewMenuItem(f))
	m.InsertItem(-1, CopyAction.NewMenuItem(f))
	m.InsertItem(-1, PasteAction.NewMenuItem(f))
	m.InsertItem(-1, DeleteAction.NewMenuItem(f))
	m.InsertItem(-1, SelectAllAction.NewMenuItem(f))
	if prefsHandler != nil && f.BarIsPerWindow() {
		m.InsertSeparator(-1, false)
		InsertPreferencesItem(m, -1, prefsHandler)
	}
	return m
}

// InsertPreferencesItem creates the standard "Preferences…" menu item that will call the provided handler when chosen.
func InsertPreferencesItem(m Menu, atIndex int, prefsHandler func(MenuItem)) {
	m.InsertItem(-1, m.Factory().NewItem(PreferencesItemID, i18n.Text("Preferences…"), KeyComma, OSMenuCmdModifier(),
		func(MenuItem) bool { return prefsHandler != nil }, prefsHandler))
}

// NewWindowMenu creates a standard 'Window' menu.
func NewWindowMenu(f MenuFactory, updater func(Menu)) Menu {
	m := f.NewMenu(WindowMenuID, i18n.Text("Window"), updater)
	InsertMinimizeItem(m, -1)
	InsertZoomItem(m, -1)
	m.InsertSeparator(-1, false)
	InsertBringAllToFrontItem(m, -1)
	return m
}

// InsertMinimizeItem creates the standard "Minimize" menu item that will issue the Minimize command to the currently
// focused window when chosen.
func InsertMinimizeItem(m Menu, atIndex int) {
	m.InsertItem(atIndex, m.Factory().NewItem(MinimizeItemID, i18n.Text("Minimize"), KeyM, OSMenuCmdModifier(),
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
	m.InsertItem(atIndex, m.Factory().NewItem(ZoomItemID, i18n.Text("Zoom"), KeyZ, ShiftModifier|OSMenuCmdModifier(),
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
	m.InsertItem(-1, m.Factory().NewItem(BringAllWindowsToFrontItemID, i18n.Text("Bring All to Front"), KeyNone,
		NoModifiers, func(MenuItem) bool { return WindowCount() > 0 }, func(MenuItem) { AllWindowsToFront() }))
}

// NewHelpMenu creates a standard 'Help' menu.
func NewHelpMenu(f MenuFactory, aboutHandler func(MenuItem), updater func(Menu)) Menu {
	m := f.NewMenu(HelpMenuID, i18n.Text("Help"), updater)
	if f.BarIsPerWindow() {
		InsertAboutItem(m, -1, aboutHandler)
	}
	return m
}

type barHolder interface {
	MenuBar() *Panel
	SetMenuBar(bar *Panel, preMovedCallback, postLostFocusCallback func(*Window),
		preMouseDownCallback func(*Window, geom32.Point) bool,
		preKeyDownCallback func(*Window, KeyCode, Modifiers) bool,
		preKeyUpCallback func(*Window, KeyCode, Modifiers) bool,
		preRuneTypedCallback func(*Window, rune) bool)
}

func barHolderFromWindow(w *Window) barHolder {
	p := w.Content()
	for p.Parent() != nil {
		p = p.Parent()
	}
	if holder, ok := p.Self.(barHolder); ok {
		return holder
	}
	return nil
}

func overMenuBar(w *Window, where geom32.Point) bool {
	if holder := barHolderFromWindow(w); holder != nil {
		if bar := holder.MenuBar(); bar != nil {
			return bar.FrameRect().ContainsPoint(where) && bar.PanelAt(where) != bar
		}
	}
	return false
}
