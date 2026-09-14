// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// nodeAction is one of the things an assistive technology can ask a node to do, under the name AT-SPI knows it by.
type nodeAction struct {
	name   string
	action accessibility.Action
}

// actionOrder is the order org.a11y.atspi.Action reports the actions of a node in. The order is fixed rather than
// derived from the node so that a client that remembers "action 0" of a kind of widget is not surprised by another one
// of the same kind, and the names are the ones assistive technologies already recognize: "click" is what they announce
// as the default action of a button.
var actionOrder = []nodeAction{
	{name: "click", action: accessibility.Press},
	{name: "toggle", action: accessibility.Toggle},
	{name: "expand", action: accessibility.Expand},
	{name: "collapse", action: accessibility.Collapse},
	{name: "increment", action: accessibility.Increment},
	{name: "decrement", action: accessibility.Decrement},
	{name: "menu", action: accessibility.ShowContextMenu},
}

// nodeActions returns the actions a node supports, in the order AT-SPI reports them.
func nodeActions(n *accessibility.Node) []nodeAction {
	actions := make([]nodeAction, 0, len(actionOrder))
	for _, one := range actionOrder {
		if n.Actions.Has(one.action) {
			actions = append(actions, one)
		}
	}
	return actions
}

// hasActions returns true if a node supports at least one of the actions AT-SPI can ask for, which is what decides
// whether it implements org.a11y.atspi.Action at all. It answers without allocating, since it is asked for every node
// of every snapshot.
func hasActions(n *accessibility.Node) bool {
	for _, one := range actionOrder {
		if n.Actions.Has(one.action) {
			return true
		}
	}
	return false
}

// actionInterface returns the org.a11y.atspi.Action interface of a node. The actions are the ones the caller has
// already worked out, so that the list is built once per call rather than once per method.
func (o *nodeObject) actionInterface(actions []nodeAction) *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceAction,
		Methods: []*dbus.Method{
			{Name: "GetName", In: "i", Out: "s", Handle: o.actionName(actions)},
			{Name: "GetLocalizedName", In: "i", Out: "s", Handle: o.actionName(actions)},
			{Name: "GetDescription", In: "i", Out: "s", Handle: o.actionDescription(actions)},
			{Name: "GetKeyBinding", In: "i", Out: "s", Handle: o.actionKeyBinding(actions)},
			{Name: "GetActions", Out: "a(sss)", Handle: o.getActions(actions)},
			{Name: "DoAction", In: "i", Out: "b", Handle: o.doAction(actions)},
		},
		Properties: []*dbus.Property{
			{Name: "NActions", Sig: "i", Get: func() (any, error) { return int32(len(actions)), nil }},
		},
	}
}

// actionName returns the handler for org.a11y.atspi.Action.GetName and GetLocalizedName. There is nothing to localize:
// the names are the ones AT-SPI defines, and an assistive technology decides for itself what to say about them.
func (o *nodeObject) actionName(actions []nodeAction) func(*dbus.Call) {
	return func(call *dbus.Call) {
		index, ok := actionIndex(call, actions)
		if !ok {
			return
		}
		call.Reply(actions[index].name)
	}
}

// actionDescription returns the handler for org.a11y.atspi.Action.GetDescription. The name says everything there is to
// say about these actions, so there is no description to add.
func (o *nodeObject) actionDescription(actions []nodeAction) func(*dbus.Call) {
	return func(call *dbus.Call) {
		if _, ok := actionIndex(call, actions); !ok {
			return
		}
		call.Reply("")
	}
}

// actionKeyBinding returns the handler for org.a11y.atspi.Action.GetKeyBinding. Only the default action has a key
// binding, and only when the node has one to report, such as a menu item's accelerator.
func (o *nodeObject) actionKeyBinding(actions []nodeAction) func(*dbus.Call) {
	return func(call *dbus.Call) {
		index, ok := actionIndex(call, actions)
		if !ok {
			return
		}
		if actions[index].action == accessibility.Press {
			call.Reply(keyBinding(o.node.Shortcut))
			return
		}
		call.Reply("")
	}
}

// keyBinding returns a node's shortcut in the form every AT-SPI producer reports one: the three semicolon-separated
// fields mnemonic;full-shortcut;accelerator, of which a Unison shortcut is only ever the last. An assistive technology
// splits the string on the semicolons — Orca's mnemonicShortcutAccelerator does exactly that — so a bare accelerator
// lands in the mnemonic's field, which has a menu item's Ctrl+V announced, or suppressed, as though it were the
// underlined letter of a menu path. GTK's gtkatspiaction.c builds "%s;;%s" for the same reason. A node with no shortcut
// reports nothing at all rather than two bare separators.
func keyBinding(shortcut string) string {
	if shortcut == "" {
		return ""
	}
	return ";;" + shortcut
}

// getActions returns the handler for org.a11y.atspi.Action.GetActions, which hands over every action in one call as
// name, description and key binding. The key binding is the same triple GetKeyBinding reports; see [keyBinding].
func (o *nodeObject) getActions(actions []nodeAction) func(*dbus.Call) {
	return func(call *dbus.Call) {
		list := make([]any, 0, len(actions))
		for _, one := range actions {
			binding := ""
			if one.action == accessibility.Press {
				binding = keyBinding(o.node.Shortcut)
			}
			list = append(list, dbus.Struct{one.name, "", binding})
		}
		call.Reply(list)
	}
}

// doAction returns the handler for org.a11y.atspi.Action.DoAction. The answer is optimistic: the request is handed to
// the user interface thread, and "yes" means it was accepted rather than that it has happened.
func (o *nodeObject) doAction(actions []nodeAction) func(*dbus.Call) {
	return func(call *dbus.Call) {
		index, ok := actionIndex(call, actions)
		if !ok {
			return
		}
		call.Reply(o.a.dispatch(accessibility.ActionRequest{
			Node:   o.node.ID,
			Action: actions[index].action,
		}))
	}
}

// actionIndex returns the action index a call names, answering it and returning false if the index is out of range.
// Answering with an error rather than a default is what libatspi expects: it asks for indexes it has already been told
// about, so one that is out of range means the client is confused.
func actionIndex(call *dbus.Call, actions []nodeAction) (index int, ok bool) {
	args, ok := callArgs(call)
	if !ok {
		return 0, false
	}
	index = int(int32Arg(args, 0))
	if index < 0 || index >= len(actions) {
		call.Error(dbus.InvalidArgs, "the action index is out of range")
		return 0, false
	}
	return index, true
}
