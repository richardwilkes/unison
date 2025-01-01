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
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/i18n"
)

var (
	cutAction       *Action
	copyAction      *Action
	pasteAction     *Action
	deleteAction    *Action
	selectAllAction *Action
)

// Action describes an action that can be performed.
type Action struct {
	EnabledCallback func(*Action, any) bool // Should return true if the action can be used. Care should be made to keep this method fast to avoid slowing down the user interface. May be nil, in which case it is assumed to always be enabled.
	ExecuteCallback func(*Action, any)      // Will be called to run the action. May be nil.
	Title           string                  // Typically used in a menu item title or tooltip for a button.
	ID              int                     // Should be unique among all actions and menu items.
	KeyBinding      KeyBinding              // The key binding that will trigger the action.
}

// NewMenuItem returns a newly created menu item using this action.
func (a *Action) NewMenuItem(f MenuFactory) MenuItem {
	return f.NewItem(a.ID, a.Title, a.KeyBinding, a.enabled, a.execute)
}

// NewContextMenuItemFromAction returns a newly created menu item for a context menu using this action. If the menuItem
// would be disabled, nil is returned instead.
func (a *Action) NewContextMenuItemFromAction(f MenuFactory) MenuItem {
	if !a.Enabled(nil) {
		return nil
	}
	return f.NewItem(a.ID|ContextMenuIDFlag, a.Title, KeyBinding{}, a.enabled, a.execute)
}

// Enabled returns true if the action can be used.
func (a *Action) Enabled(src any) bool {
	if a.EnabledCallback == nil {
		return true
	}
	result := false
	toolbox.Call(func() { result = a.EnabledCallback(a, src) })
	return result
}

func (a *Action) enabled(item MenuItem) bool {
	enabled := a.Enabled(item)
	if item.Title() != a.Title {
		item.SetTitle(a.Title)
	}
	return enabled
}

// Execute the action. Calls Enabled() to verify execution is permitted.
func (a *Action) Execute(src any) {
	if a.ExecuteCallback != nil && a.Enabled(src) {
		toolbox.Call(func() { a.ExecuteCallback(a, src) })
	}
}

func (a *Action) execute(item MenuItem) {
	a.Execute(item)
}

// RouteActionToFocusEnabledFunc is intended to be the EnabledCallback for actions that will route to the currently
// focused UI widget and call CanPerformCmd() on it.
func RouteActionToFocusEnabledFunc(action *Action, src any) bool {
	if wnd := ActiveWindow(); wnd != nil {
		return wnd.Focus().CanPerformCmd(src, action.ID)
	}
	return false
}

// RouteActionToFocusExecuteFunc is intended to be the ExecuteCallback for actions that will route to the currently
// focused UI widget and call PerformCmd() on it.
func RouteActionToFocusExecuteFunc(action *Action, src any) {
	if wnd := ActiveWindow(); wnd != nil {
		wnd.Focus().PerformCmd(src, action.ID)
	}
}

// CutAction returns the action that removes the selection and places it on the clipboard.
func CutAction() *Action {
	if cutAction == nil {
		cutAction = &Action{
			ID:              CutItemID,
			Title:           i18n.Text("Cut"),
			KeyBinding:      KeyBinding{KeyCode: KeyX, Modifiers: OSMenuCmdModifier()},
			EnabledCallback: RouteActionToFocusEnabledFunc,
			ExecuteCallback: RouteActionToFocusExecuteFunc,
		}
	}
	return cutAction
}

// CopyAction returns the action that copies the selection and places it on the clipboard.
func CopyAction() *Action {
	if copyAction == nil {
		copyAction = &Action{
			ID:              CopyItemID,
			Title:           i18n.Text("Copy"),
			KeyBinding:      KeyBinding{KeyCode: KeyC, Modifiers: OSMenuCmdModifier()},
			EnabledCallback: RouteActionToFocusEnabledFunc,
			ExecuteCallback: RouteActionToFocusExecuteFunc,
		}
	}
	return copyAction
}

// PasteAction returns the action that pastes the contents of the clipboard, replacing the selection.
func PasteAction() *Action {
	if pasteAction == nil {
		pasteAction = &Action{
			ID:              PasteItemID,
			Title:           i18n.Text("Paste"),
			KeyBinding:      KeyBinding{KeyCode: KeyV, Modifiers: OSMenuCmdModifier()},
			EnabledCallback: RouteActionToFocusEnabledFunc,
			ExecuteCallback: RouteActionToFocusExecuteFunc,
		}
	}
	return pasteAction
}

// DeleteAction returns the action that deletes the selection.
func DeleteAction() *Action {
	if deleteAction == nil {
		deleteAction = &Action{
			ID:              DeleteItemID,
			Title:           i18n.Text("Delete"),
			EnabledCallback: RouteActionToFocusEnabledFunc,
			ExecuteCallback: RouteActionToFocusExecuteFunc,
		}
	}
	return deleteAction
}

// SelectAllAction returns the action that selects everything in the current focus.
func SelectAllAction() *Action {
	if selectAllAction == nil {
		selectAllAction = &Action{
			ID:              SelectAllItemID,
			Title:           i18n.Text("Select All"),
			KeyBinding:      KeyBinding{KeyCode: KeyA, Modifiers: OSMenuCmdModifier()},
			EnabledCallback: RouteActionToFocusEnabledFunc,
			ExecuteCallback: RouteActionToFocusExecuteFunc,
		}
	}
	return selectAllAction
}
