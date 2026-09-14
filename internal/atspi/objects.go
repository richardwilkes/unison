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

// nodeObject is the object that answers for one node of one window's snapshot. It is created for each call, by
// [Adapter.resolve], and holds the snapshot it was resolved against, so every answer it gives describes the same moment
// even if the user interface thread publishes a new snapshot while the call is being answered.
type nodeObject struct {
	a    *Adapter
	data *windowData
	node *accessibility.Node
}

// resolve returns the object that answers calls at a path below [AccessiblePrefix], or nil if nothing is there, which
// the connection turns into an UnknownObject error. An ignored node has no object of its own: it is not reported to an
// assistive technology at all, its children standing in for it.
func (a *Adapter) resolve(path dbus.ObjectPath) dbus.Object {
	id, ok := ParseNodePath(path)
	if !ok {
		return nil
	}
	ws := a.windowFor(id)
	if ws == nil {
		return nil
	}
	data := ws.data.Load()
	if data == nil {
		return nil
	}
	n := data.node(id)
	if n == nil || n.Ignored {
		return nil
	}
	return &nodeObject{a: a, data: data, node: n}
}

// Interfaces implements [dbus.Object]. Both the membership of the list and its order match what
// org.a11y.atspi.Accessible.GetInterfaces reports, which [Interfaces] decides.
func (o *nodeObject) Interfaces() []*dbus.Interface {
	list := make([]*dbus.Interface, 0, 8)
	list = append(list, o.accessibleInterface())
	if actions := nodeActions(o.node); len(actions) != 0 {
		list = append(list, o.actionInterface(actions))
	}
	list = append(list, o.componentInterface())
	if supportsEditableText(o.node) {
		list = append(list, o.editableTextInterface())
	}
	if supportsSelection(o.node.Role) {
		list = append(list, o.selectionInterface())
	}
	if supportsTable(o.node.Role) {
		list = append(list, o.tableInterface())
	}
	if supportsTableCell(o.node.Role) {
		list = append(list, o.tableCellInterface())
	}
	if supportsText(o.node) {
		list = append(list, o.textInterface())
	}
	if o.node.HasNumber {
		list = append(list, o.valueInterface())
	}
	return list
}

// reference returns the reference to this node's object.
func (o *nodeObject) reference() dbus.ObjectRef {
	return o.a.reference(o.node.ID)
}

// parentReference returns the reference to the object that holds this one. The parent of a window's root is the
// application, not a node.
func (o *nodeObject) parentReference() dbus.ObjectRef {
	if o.isRoot() {
		return o.a.rootReference()
	}
	return o.a.reference(o.data.parent(o.node.ID))
}

// children returns the ids of the children that are reported for this node.
func (o *nodeObject) children() []accessibility.NodeID {
	return o.data.unignoredChildren(o.node.ID)
}

// indexInParent returns where this node sits among the reported children of its parent. A window's root reports its
// position among the application's windows instead.
func (o *nodeObject) indexInParent() int {
	if o.isRoot() {
		return o.a.windowIndex(o.window())
	}
	return o.data.indexInParent(o.node.ID)
}

// window returns the window this node belongs to, or nil if it has been removed since the call arrived.
func (o *nodeObject) window() *windowState {
	return o.a.windowFor(o.node.ID)
}

// states returns the node's states, taking into account whether its window is the active one and whether the node is
// the window itself.
func (o *nodeObject) states() StateSet {
	return States(o.node, o.data.active(), o.isRoot())
}

// isRoot reports whether this node is the window its snapshot describes, as distinct from a node within one that
// reports a window role of its own.
func (o *nodeObject) isRoot() bool {
	return o.node.ID == o.data.tree.Root
}

// callArgs unmarshals the arguments of a call, answering it with an InvalidArgs error and returning false if they
// cannot be read. The arguments have already been checked against the method's declared signature, so this only fails
// if a message claimed a signature its body does not hold.
func callArgs(call *dbus.Call) ([]any, bool) {
	args, err := call.Args()
	if err != nil {
		call.Error(dbus.InvalidArgs, err.Error())
		return nil, false
	}
	return args, true
}

// int32Arg returns one argument of a call as an int32. Zero stands in for an argument that is not there or is not the
// type the method's signature promised, neither of which the connection lets through.
func int32Arg(args []any, i int) int32 {
	if i >= len(args) {
		return 0
	}
	value, ok := args[i].(int32)
	if !ok {
		return 0
	}
	return value
}

// uint32Arg returns one argument of a call as a uint32. Zero stands in for an argument that is not there or is not the
// type the method's signature promised, neither of which the connection lets through.
func uint32Arg(args []any, i int) uint32 {
	if i >= len(args) {
		return 0
	}
	value, ok := args[i].(uint32)
	if !ok {
		return 0
	}
	return value
}

// coordArg returns one argument of a call as a coordinate type. An argument that is not there is read as CoordScreen,
// which is zero.
func coordArg(args []any, i int) CoordType {
	return CoordType(uint32Arg(args, i))
}
