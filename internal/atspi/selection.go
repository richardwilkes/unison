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

// selectionInterface returns the org.a11y.atspi.Selection interface of a container whose children are selected, such as
// a list, a table or a tab list. Every index is over the reported children, so an ignored grouping panel in the middle
// of a list does not shift the numbering.
//
// SelectAll and ClearSelection always report that they did nothing: the schema has no request that means "select
// everything", and turning one into a request per child would send a storm of them at the user interface thread for a
// result no widget promises to produce.
func (o *nodeObject) selectionInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceSelection,
		Methods: []*dbus.Method{
			{Name: "GetSelectedChild", In: "i", Out: objectRefSignature, Handle: o.getSelectedChild},
			{Name: "SelectChild", In: "i", Out: "b", Handle: o.selectChild},
			{Name: "DeselectSelectedChild", In: "i", Out: "b", Handle: o.deselectSelectedChild},
			{Name: "DeselectChild", In: "i", Out: "b", Handle: o.deselectChild},
			{Name: "IsChildSelected", In: "i", Out: "b", Handle: o.isChildSelected},
			{Name: "SelectAll", Out: "b", Handle: replyFalse},
			{Name: "ClearSelection", Out: "b", Handle: replyFalse},
		},
		Properties: []*dbus.Property{
			{
				Name: "NSelectedChildren",
				Sig:  "i",
				Get:  func() (any, error) { return int32(len(o.selectedChildren())), nil },
			},
		},
	}
}

// getSelectedChild implements org.a11y.atspi.Selection.GetSelectedChild, which counts over the selected children rather
// than over all of them.
func (o *nodeObject) getSelectedChild(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	selected := o.selectedChildren()
	index := int(int32Arg(args, 0))
	if index < 0 || index >= len(selected) {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.a.reference(selected[index]))
}

// isChildSelected implements org.a11y.atspi.Selection.IsChildSelected.
func (o *nodeObject) isChildSelected(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	child := o.childAt(int(int32Arg(args, 0)))
	call.Reply(child != nil && child.Selected)
}

// selectChild implements org.a11y.atspi.Selection.SelectChild. In a container that allows more than one selection the
// child is added to the selection, since that is what a client asking for a second one means; anywhere else it becomes
// the selection.
func (o *nodeObject) selectChild(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	child := o.childAt(int(int32Arg(args, 0)))
	if child == nil {
		call.Reply(false)
		return
	}
	action := accessibility.Select
	if o.node.Multiselectable && child.Actions.Has(accessibility.AddToSelection) {
		action = accessibility.AddToSelection
	}
	call.Reply(o.requestOnChild(child, action))
}

// deselectChild implements org.a11y.atspi.Selection.DeselectChild.
func (o *nodeObject) deselectChild(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	child := o.childAt(int(int32Arg(args, 0)))
	if child == nil {
		call.Reply(false)
		return
	}
	call.Reply(o.requestOnChild(child, accessibility.RemoveFromSelection))
}

// deselectSelectedChild implements org.a11y.atspi.Selection.DeselectSelectedChild, whose index counts over the selected
// children rather than over all of them.
func (o *nodeObject) deselectSelectedChild(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	selected := o.selectedChildren()
	index := int(int32Arg(args, 0))
	if index < 0 || index >= len(selected) {
		call.Reply(false)
		return
	}
	call.Reply(o.requestOnChild(o.data.node(selected[index]), accessibility.RemoveFromSelection))
}

// childAt returns the reported child at an index, or nil if there is none there.
func (o *nodeObject) childAt(index int) *accessibility.Node {
	children := o.children()
	if index < 0 || index >= len(children) {
		return nil
	}
	return o.data.node(children[index])
}

// requestOnChild asks a child to change its selection state, returning whether the request was accepted. The answer is
// optimistic, as every request is.
func (o *nodeObject) requestOnChild(child *accessibility.Node, action accessibility.Action) bool {
	if child == nil || !child.Actions.Has(action) {
		return false
	}
	return o.a.dispatch(accessibility.ActionRequest{Node: child.ID, Action: action})
}
