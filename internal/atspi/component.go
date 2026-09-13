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

// componentInterface returns the org.a11y.atspi.Component interface of a node, which is how an assistive technology
// finds out where the node is on the screen, and how the user's pointer finds out what is under it.
//
// Everything is answered from the published snapshot, converted with the window's geometry. The methods that would
// change the node — the setters and ScrollToPoint — report that they did nothing, since a toolkit that lays its own
// widgets out has nowhere to put an externally imposed position.
func (o *nodeObject) componentInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceComponent,
		Methods: []*dbus.Method{
			{Name: "Contains", In: intPairAndCoordSignature, Out: "b", Handle: o.contains},
			{
				Name: "GetAccessibleAtPoint", In: intPairAndCoordSignature, Out: objectRefSignature,
				Handle: o.getAccessibleAtPoint,
			},
			{Name: "GetExtents", In: "u", Out: extentsSignature, Handle: o.getExtents},
			{Name: "GetPosition", In: "u", Out: "ii", Handle: o.getPosition},
			{Name: "GetSize", Out: "ii", Handle: o.getSize},
			{Name: "GetLayer", Out: "u", Handle: o.getLayer},
			{Name: "GetMDIZOrder", Out: "n", Handle: o.getMDIZOrder},
			{Name: "GrabFocus", Out: "b", Handle: o.grabFocus},
			{Name: "GetAlpha", Out: "d", Handle: o.getAlpha},
			{Name: "SetExtents", In: "iiiiu", Out: "b", Handle: replyFalse},
			{Name: "SetPosition", In: intPairAndCoordSignature, Out: "b", Handle: replyFalse},
			{Name: "SetSize", In: "ii", Out: "b", Handle: replyFalse},
			{Name: "ScrollTo", In: "u", Out: "b", Handle: o.scrollTo},
			{Name: "ScrollToPoint", In: "uii", Out: "b", Handle: replyFalse},
		},
	}
}

// replyFalse answers a method that reports whether it did something with "no".
func replyFalse(call *dbus.Call) {
	call.Reply(false)
}

// contains implements org.a11y.atspi.Component.Contains.
func (o *nodeObject) contains(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	pt := o.data.logicalPoint(o.node, int32Arg(args, 0), int32Arg(args, 1), coordArg(args, 2))
	call.Reply(pt.In(o.node.Bounds))
}

// getAccessibleAtPoint implements org.a11y.atspi.Component.GetAccessibleAtPoint. The answer is the deepest reported
// descendant of this node at the point; the node itself does not count, and neither does anything outside it, both of
// which are answered with the null reference.
func (o *nodeObject) getAccessibleAtPoint(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	pt := o.data.logicalPoint(o.node, int32Arg(args, 0), int32Arg(args, 1), coordArg(args, 2))
	var deepest accessibility.NodeID
	below := false
	for id := o.data.tree.HitTest(pt); id != 0; id = o.data.parent(id) {
		if id == o.node.ID {
			below = true
			break
		}
		if deepest == 0 {
			if n := o.data.node(id); n != nil && !n.Ignored {
				deepest = id
			}
		}
	}
	if !below || deepest == 0 {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.a.reference(deepest))
}

// getExtents implements org.a11y.atspi.Component.GetExtents.
func (o *nodeObject) getExtents(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	x, y, w, h := o.data.extents(o.node, coordArg(args, 0))
	call.Reply(dbus.Struct{x, y, w, h})
}

// getPosition implements org.a11y.atspi.Component.GetPosition.
func (o *nodeObject) getPosition(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	x, y, _, _ := o.data.extents(o.node, coordArg(args, 0))
	call.Reply(x, y)
}

// getSize implements org.a11y.atspi.Component.GetSize. A size is the same in every coordinate space, so the method does
// not take one.
func (o *nodeObject) getSize(call *dbus.Call) {
	_, _, w, h := o.data.extents(o.node, CoordWindow)
	call.Reply(w, h)
}

// getLayer implements org.a11y.atspi.Component.GetLayer.
func (o *nodeObject) getLayer(call *dbus.Call) {
	call.Reply(uint32(layerFor(o.node)))
}

// getMDIZOrder implements org.a11y.atspi.Component.GetMDIZOrder. Unison has no multiple document interface panes, so
// there is no ordering to report.
func (o *nodeObject) getMDIZOrder(call *dbus.Call) {
	call.Reply(int16(0))
}

// getAlpha implements org.a11y.atspi.Component.GetAlpha. Nothing Unison reports is translucent as far as an assistive
// technology is concerned.
func (o *nodeObject) getAlpha(call *dbus.Call) {
	call.Reply(1.0)
}

// grabFocus implements org.a11y.atspi.Component.GrabFocus. The answer is optimistic: the request is handed to the user
// interface thread and "yes" means it was accepted, not that the focus has moved yet. A node that cannot take the focus
// says so immediately.
func (o *nodeObject) grabFocus(call *dbus.Call) {
	if o.node.Disabled || !o.node.Focusable {
		call.Reply(false)
		return
	}
	call.Reply(o.a.dispatch(accessibility.ActionRequest{Node: o.node.ID, Action: accessibility.Focus}))
}

// scrollTo implements org.a11y.atspi.Component.ScrollTo. Where in the view the node ends up is up to the widget: the
// schema has one request for this, and it means "make it visible".
func (o *nodeObject) scrollTo(call *dbus.Call) {
	if !o.node.Actions.Has(accessibility.ScrollIntoView) {
		call.Reply(false)
		return
	}
	call.Reply(o.a.dispatch(accessibility.ActionRequest{Node: o.node.ID, Action: accessibility.ScrollIntoView}))
}
