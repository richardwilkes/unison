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
	"strconv"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// accessibleInterface returns the org.a11y.atspi.Accessible interface of a node, which is what every assistive
// technology asks for first: what this thing is, what it is called, where it sits in the hierarchy and what state it is
// in.
func (o *nodeObject) accessibleInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceAccessible,
		Methods: []*dbus.Method{
			{Name: "GetChildAtIndex", In: "i", Out: objectRefSignature, Handle: o.getChildAtIndex},
			{Name: "GetChildren", Out: objectRefArraySignature, Handle: o.getChildren},
			{Name: "GetIndexInParent", Out: "i", Handle: o.getIndexInParent},
			{Name: "GetRelationSet", Out: relationSetSignature, Handle: o.getRelationSet},
			{Name: "GetRole", Out: "u", Handle: o.getRole},
			{Name: "GetRoleName", Out: "s", Handle: o.getRoleName},
			{Name: "GetLocalizedRoleName", Out: "s", Handle: o.getRoleName},
			{Name: "GetState", Out: stateSignature, Handle: o.getState},
			{Name: "GetAttributes", Out: stringDictSignature, Handle: o.getAttributes},
			{Name: "GetApplication", Out: objectRefSignature, Handle: o.getApplication},
			{Name: "GetInterfaces", Out: "as", Handle: o.getInterfaces},
		},
		Properties: []*dbus.Property{
			{Name: "Name", Sig: "s", Get: func() (any, error) { return o.node.Name, nil }},
			{Name: "Description", Sig: "s", Get: func() (any, error) { return o.node.Description, nil }},
			{Name: "Parent", Sig: objectRefSignature, Get: func() (any, error) { return o.parentReference(), nil }},
			{Name: "ChildCount", Sig: "i", Get: func() (any, error) { return int32(len(o.children())), nil }},
			{Name: "Locale", Sig: "s", Get: func() (any, error) { return currentLocale(), nil }},
			{Name: "AccessibleId", Sig: "s", Get: func() (any, error) { return o.accessibleID(), nil }},
			{Name: "HelpText", Sig: "s", Get: func() (any, error) { return "", nil }},
		},
	}
}

// accessibleID returns the identifier an assistive technology can use to recognize this node again, which is the
// decimal form of its node id. Ids come from one process-wide counter and are never reused, so this is stable for as
// long as the node exists.
func (o *nodeObject) accessibleID() string {
	return strconv.FormatUint(uint64(o.node.ID), 10)
}

// getChildAtIndex implements org.a11y.atspi.Accessible.GetChildAtIndex. An index that is out of range is answered with
// the null reference rather than an error, which is what libatspi expects of a hierarchy that may have changed since
// the client counted the children.
func (o *nodeObject) getChildAtIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	children := o.children()
	index := int(int32Arg(args, 0))
	if index < 0 || index >= len(children) {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.a.reference(children[index]))
}

// getChildren implements org.a11y.atspi.Accessible.GetChildren.
func (o *nodeObject) getChildren(call *dbus.Call) {
	call.Reply(o.a.references(o.children()))
}

// getIndexInParent implements org.a11y.atspi.Accessible.GetIndexInParent.
func (o *nodeObject) getIndexInParent(call *dbus.Call) {
	call.Reply(int32(o.indexInParent()))
}

// getRelationSet implements org.a11y.atspi.Accessible.GetRelationSet.
func (o *nodeObject) getRelationSet(call *dbus.Call) {
	relations := o.data.relationsOf(o.node.ID)
	set := make([]any, 0, len(relations))
	for _, one := range relations {
		set = append(set, dbus.Struct{uint32(one.kind), o.a.references(one.targets)})
	}
	call.Reply(set)
}

// getRole implements org.a11y.atspi.Accessible.GetRole.
func (o *nodeObject) getRole(call *dbus.Call) {
	call.Reply(uint32(MapRole(o.node)))
}

// getRoleName implements both org.a11y.atspi.Accessible.GetRoleName and GetLocalizedRoleName. There is nothing to
// localize: the names are the ones AT-SPI defines, and an assistive technology localizes what it says to the user
// itself.
func (o *nodeObject) getRoleName(call *dbus.Call) {
	call.Reply(RoleName(MapRole(o.node)))
}

// getState implements org.a11y.atspi.Accessible.GetState.
func (o *nodeObject) getState(call *dbus.Call) {
	call.Reply(o.states().Words())
}

// getAttributes implements org.a11y.atspi.Accessible.GetAttributes. The tree and the reported parent go along with the
// node, since where a node sits within a set is known to neither: how big the set a row belongs to is known only to the
// container it sits in, and how long a run of tabs or menu items is only to the tree that holds them.
func (o *nodeObject) getAttributes(call *dbus.Call) {
	call.Reply(Attributes(o.data.tree, o.node, o.data.node(o.data.parent(o.node.ID))))
}

// getApplication implements org.a11y.atspi.Accessible.GetApplication.
func (o *nodeObject) getApplication(call *dbus.Call) {
	call.Reply(o.a.rootReference())
}

// getInterfaces implements org.a11y.atspi.Accessible.GetInterfaces.
func (o *nodeObject) getInterfaces(call *dbus.Call) {
	call.Reply(Interfaces(o.node))
}

// selectedChildren returns the ids of the reported children of this node that are selected.
func (o *nodeObject) selectedChildren() []accessibility.NodeID {
	children := o.children()
	selected := make([]accessibility.NodeID, 0, len(children))
	for _, id := range children {
		if n := o.data.node(id); n != nil && n.Selected {
			selected = append(selected, id)
		}
	}
	return selected
}
