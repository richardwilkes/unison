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
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/internal/dbus"
)

// The shape of every signal in this file — which interface it belongs to, what its detail string says, what its two
// integers mean and what its value holds — is the one at-spi2-atk's atk-adaptor/event.c sends for the same change,
// since that is the bridge every assistive technology on the desktop was written against. The cache signals follow
// atk-adaptor/adaptors/cache-adaptor.c, and the announcement, which postdates ATK, follows GTK's
// gtk/a11y/gtkatspicontext.c. libatspi refuses an event whose body is not "siiva{sv}" outright, so the signature is not
// negotiable; the properties dictionary at the end of it is where a bridge may pass a few of the sender's properties
// along to save a round trip, and leaving it empty is allowed.
//
// Two things are deliberately not what atk-adaptor does. Its accessible-value property change carries an unused integer
// where this one carries the new value, as GTK's does, and its announcement comes from the object concerned where this
// one comes from the application, since a Unison announcement belongs to no object. Neither is anything a client reads
// the changed value from: it asks the object.

// The members of the AT-SPI event interfaces that this package sends. An AT-SPI event is named class:major:minor, which
// on D-Bus becomes interface:member:detail, so these are the major parts, in the CamelCase that D-Bus requires of a
// signal name and that an assistive technology converts back to the hyphenated form it matches its listeners against.
const (
	signalActivate             = "Activate"
	signalAnnouncement         = "Announcement"
	signalAttributesChanged    = "AttributesChanged"
	signalBoundsChanged        = "BoundsChanged"
	signalChildrenChanged      = "ChildrenChanged"
	signalCreate               = "Create"
	signalDeactivate           = "Deactivate"
	signalDestroy              = "Destroy"
	signalFocus                = "Focus"
	signalPropertyChange       = "PropertyChange"
	signalStateChanged         = "StateChanged"
	signalTextCaretMoved       = "TextCaretMoved"
	signalTextChanged          = "TextChanged"
	signalTextSelectionChanged = "TextSelectionChanged"
)

// The members of the cache object's signals, which are not events and carry a cache item rather than the usual event
// body.
const (
	signalAddAccessible    = "AddAccessible"
	signalRemoveAccessible = "RemoveAccessible"
)

// The detail strings of the structural and text events, which are the minor part of the AT-SPI event name.
const (
	detailAdd    = "add"
	detailDelete = "delete"
	detailInsert = "insert"
	detailRemove = "remove"
)

// The detail strings of the state changes, which are AT-SPI's own names for the members of AtspiStateType: lowercase,
// with words separated by hyphens. An assistive technology listens for "object:state-changed:focused" and compares the
// detail against the part after the last colon, so these have to match exactly.
const (
	stateNameActive          = "active"
	stateNameBusy            = "busy"
	stateNameChecked         = "checked"
	stateNameEditable        = "editable"
	stateNameEnabled         = "enabled"
	stateNameExpandable      = "expandable"
	stateNameExpanded        = "expanded"
	stateNameFocusable       = "focusable"
	stateNameFocused         = "focused"
	stateNameIndeterminate   = "indeterminate"
	stateNameInvalidEntry    = "invalid-entry"
	stateNameModal           = "modal"
	stateNameMultiselectable = "multiselectable"
	stateNamePressed         = "pressed"
	stateNameReadOnly        = "read-only"
	stateNameSelectable      = "selectable"
	stateNameSelected        = "selected"
	stateNameSensitive       = "sensitive"
	stateNameShowing         = "showing"
)

// The detail strings of the property changes, which are the names of the ATK properties the values belong to. AT-SPI
// has no other spelling for them: the names are part of the protocol, and an assistive technology matches on
// "object:property-change:accessible-name" and its like.
const (
	propertyAccessibleName        = "accessible-name"
	propertyAccessibleDescription = "accessible-description"
	propertyAccessibleValue       = "accessible-value"
)

// livePolite is ATSPI_LIVE_POLITE, from AtspiLive. An announcement carries it as detail1 to say that whatever the
// assistive technology is saying now may finish first.
const livePolite = 1

// trueValue is what a boolean state change holds in the event's New and Old, which is what strconv.FormatBool writes.
const trueValue = "true"

// emitEvents turns the events that came with a published tree into the AT-SPI signals on the nodes they concern. It is
// called after [Adapter.Publish] has released the lock, and never blocks: every signal is queued rather than written.
//
// prior is the snapshot the window had before this one and is nil the first time a window is published, which is what
// the whole window has to be announced from. It is also what the events that describe something that is no longer in
// the tree — a node that has been removed, or the node that used to hold the focus — are read from.
func (a *Adapter) emitEvents(ws *windowState, prior, data *windowData, events []accessibility.Event) {
	if data == nil {
		return
	}
	if prior == nil {
		// A window nobody has been told about yet needs its whole hierarchy announced, which covers everything the
		// events could say: the only event a first snapshot produces is the focus it already has.
		a.emitWindowAdded(ws, data)
		return
	}
	for i := range events {
		a.emitEvent(prior, data, &events[i])
	}
}

// emitEvent sends the signals for one event.
func (a *Adapter) emitEvent(prior, data *windowData, ev *accessibility.Event) {
	switch ev.Kind {
	case accessibility.FocusChanged:
		a.emitFocusChanged(prior, data, ev)
	case accessibility.NameChanged:
		a.emitPropertyChange(data, ev.Node, propertyAccessibleName, variantString(ev.New))
	case accessibility.DescriptionChanged:
		a.emitPropertyChange(data, ev.Node, propertyAccessibleDescription, variantString(ev.New))
	case accessibility.ValueChanged:
		// The textual value is what the widget itself shows, which is what an assistive technology says out loud.
		a.emitPropertyChange(data, ev.Node, propertyAccessibleValue, variantString(ev.New))
	case accessibility.NumberChanged:
		if n := reportedNode(data, ev.Node); n != nil {
			a.emitPropertyChange(data, ev.Node, propertyAccessibleValue, variantDouble(n.Number))
		}
	case accessibility.StateChanged:
		a.emitStateChanged(data, ev)
	case accessibility.TextInserted:
		a.emitTextChanged(data, ev, detailInsert, ev.New)
	case accessibility.TextDeleted:
		a.emitTextChanged(data, ev, detailDelete, ev.Old)
	case accessibility.TextSelectionChanged:
		a.emitTextSelectionChanged(data, ev)
	case accessibility.NodeAdded:
		a.emitNodeAdded(data, ev.Node)
	case accessibility.NodeRemoved:
		a.emitNodeRemoved(prior, ev.Node)
	case accessibility.BoundsChanged:
		a.emitBoundsChanged(data, ev.Node)
	case accessibility.SortChanged:
		// AT-SPI reports a node's sort direction as an object attribute, so a change to it is a change to the
		// attributes rather than to anything with an interface of its own.
		if reportedNode(data, ev.Node) != nil {
			a.emit(NodePath(ev.Node), InterfaceEventObject, signalAttributesChanged, "", 0, 0, variantInt32(0))
		}
	case accessibility.WindowActivated:
		a.emitWindowActivated(data)
	case accessibility.WindowDeactivated:
		a.emitWindowDeactivated(data)
	case accessibility.Announcement:
		a.emitAnnouncement(ev.New)
	case accessibility.ChildrenChanged:
		// Nothing: the additions and removals that changed the list are reported one by one as NodeAdded and
		// NodeRemoved, each with the index it happened at, which is what AT-SPI's ChildrenChanged carries. A list whose
		// membership is the same but whose order is not goes unreported in V1.
	default:
	}
}

// emitWindowAdded announces a window that has just joined the accessibility tree: the window's own Create signal, the
// application root's ChildrenChanged, one cache item per reported node, and then, if the window is already the active
// one, everything that says so.
func (a *Adapter) emitWindowAdded(ws *windowState, data *windowData) {
	root := data.root()
	if root == nil {
		return
	}
	index := a.windowIndex(ws)
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalCreate, "", 0, 0, variantString(root.Name))
	a.emit(RootPath, InterfaceEventObject, signalChildrenChanged, detailAdd, int32(index), 0,
		variantRef(a.reference(root.ID)))
	data.tree.Walk(func(n *accessibility.Node) bool {
		if n.Ignored {
			return true
		}
		at := data.indexInParent(n.ID)
		if n.ID == root.ID {
			at = index
		}
		a.emitCacheAdd((&nodeObject{a: a, data: data, node: n}).cacheItem(at))
		return true
	})
	if data.active() {
		a.emitWindowActivated(data)
	}
}

// emitWindowRemoved announces that a window has left the accessibility tree: the window's own Destroy signal, the
// application root's ChildrenChanged, and one cache removal per reported node. index is where the window was among the
// application root's children, which the signal reports and which cannot be worked out once the window has been
// removed.
func (a *Adapter) emitWindowRemoved(ws *windowState, index int) {
	data := ws.data.Load()
	if data == nil {
		return
	}
	root := data.root()
	if root == nil {
		return
	}
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalDestroy, "", 0, 0, variantString(root.Name))
	a.emit(RootPath, InterfaceEventObject, signalChildrenChanged, detailRemove, int32(index), 0,
		variantRef(a.reference(root.ID)))
	data.tree.Walk(func(n *accessibility.Node) bool {
		if !n.Ignored {
			a.emitCacheRemove(a.reference(n.ID))
		}
		return true
	})
}

// emitAnnouncement asks an assistive technology to say something that belongs to the application as a whole rather than
// to any one object, which is why it comes from the application root.
func (a *Adapter) emitAnnouncement(text string) {
	if text == "" {
		return
	}
	a.emit(RootPath, InterfaceEventObject, signalAnnouncement, "", livePolite, 0, variantString(text))
}

// emitWindowActivated announces that a window has become the active one, which is also when the node inside it that
// holds the keyboard focus becomes the focus an assistive technology follows.
func (a *Adapter) emitWindowActivated(data *windowData) {
	root := data.root()
	if root == nil {
		return
	}
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalActivate, "", 0, 0, variantString(root.Name))
	a.emit(NodePath(root.ID), InterfaceEventObject, signalStateChanged, stateNameActive, 1, 0, variantInt32(0))
	a.emitFocusGained(data, data.tree.Focus)
}

// emitWindowDeactivated announces that a window has stopped being the active one, and that its focus is therefore no
// longer the focus.
func (a *Adapter) emitWindowDeactivated(data *windowData) {
	root := data.root()
	if root == nil {
		return
	}
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalDeactivate, "", 0, 0, variantString(root.Name))
	a.emit(NodePath(root.ID), InterfaceEventObject, signalStateChanged, stateNameActive, 0, 0, variantInt32(0))
	a.emitFocusLost(data, data.tree.Focus)
}

// emitFocusChanged announces that the keyboard focus has moved within a window, which is the pair of signals an
// assistive technology relies on most: the node that had the focus loses the state, and the one that has it now gains
// it.
//
// Nothing is sent while the window is not the active one. AT-SPI's FOCUSED state belongs to the active window's focus
// alone, and an assistive technology that is told about a focus move in a window the user is not looking at follows it
// there.
func (a *Adapter) emitFocusChanged(prior, data *windowData, ev *accessibility.Event) {
	if !data.active() {
		return
	}
	// Which node had the focus is only in the snapshot before this one, but whether it is still worth telling anyone
	// about depends on the new one: a node that has left the tree has already been reported as gone, and saying that
	// something that no longer exists has lost the focus would only send an assistive technology looking for it.
	if old := prior.tree.Focus; old != 0 && old != ev.Node {
		a.emitFocusLost(data, old)
	}
	a.emitFocusGained(data, ev.Node)
}

// emitFocusGained sends the two signals that say a node has taken the keyboard focus: the state change, and the legacy
// focus event that predates it and that assistive technologies still listen for.
func (a *Adapter) emitFocusGained(data *windowData, id accessibility.NodeID) {
	if reportedNode(data, id) == nil {
		return
	}
	a.emit(NodePath(id), InterfaceEventObject, signalStateChanged, stateNameFocused, 1, 0, variantInt32(0))
	a.emit(NodePath(id), InterfaceEventFocus, signalFocus, "", 0, 0, variantInt32(0))
}

// emitFocusLost sends the signal that says a node no longer holds the keyboard focus. There is no legacy event for
// losing the focus: the old one only ever announced where the focus went.
func (a *Adapter) emitFocusLost(data *windowData, id accessibility.NodeID) {
	if reportedNode(data, id) == nil {
		return
	}
	a.emit(NodePath(id), InterfaceEventObject, signalStateChanged, stateNameFocused, 0, 0, variantInt32(0))
}

// emitPropertyChange announces that one of the pieces of information an object reports through a property has changed.
func (a *Adapter) emitPropertyChange(data *windowData, id accessibility.NodeID, property string, value dbus.Variant) {
	if reportedNode(data, id) == nil {
		return
	}
	a.emit(NodePath(id), InterfaceEventObject, signalPropertyChange, property, 0, 0, value)
}

// emitStateChanged announces that one of a node's states has changed. A single change in the schema can be more than
// one AT-SPI state, since AT-SPI splits some of what Unison holds in one flag.
func (a *Adapter) emitStateChanged(data *windowData, ev *accessibility.Event) {
	if reportedNode(data, ev.Node) == nil {
		return
	}
	for _, one := range stateChanges(ev) {
		var detail1 int32
		if one.on {
			detail1 = 1
		}
		a.emit(NodePath(ev.Node), InterfaceEventObject, signalStateChanged, one.name, detail1, 0, variantInt32(0))
	}
}

// stateChange is one AT-SPI state that a schema state change is reported as, and whether the node now has it.
type stateChange struct {
	name string
	on   bool
}

// stateChanges returns the AT-SPI states a StateChanged event is reported as. Some of Unison's flags are the opposite
// of AT-SPI's state, and some are two states at once; a flag AT-SPI has no state for is reported as nothing at all.
func stateChanges(ev *accessibility.Event) []stateChange {
	on := ev.New == trueValue
	switch ev.State {
	case accessibility.StateDisabled:
		// AT-SPI says the same thing twice: ENABLED is "can be used" and SENSITIVE is "will respond to input".
		return []stateChange{{name: stateNameEnabled, on: !on}, {name: stateNameSensitive, on: !on}}
	case accessibility.StateFocusable:
		return []stateChange{{name: stateNameFocusable, on: on}}
	case accessibility.StateSelectable:
		return []stateChange{{name: stateNameSelectable, on: on}}
	case accessibility.StateSelected:
		return []stateChange{{name: stateNameSelected, on: on}}
	case accessibility.StateMultiselectable:
		return []stateChange{{name: stateNameMultiselectable, on: on}}
	case accessibility.StatePressed:
		return []stateChange{{name: stateNamePressed, on: on}}
	case accessibility.StateReadOnly:
		// A control whose value can no longer be changed has gained READ_ONLY and lost EDITABLE.
		return []stateChange{{name: stateNameReadOnly, on: on}, {name: stateNameEditable, on: !on}}
	case accessibility.StateModal:
		return []stateChange{{name: stateNameModal, on: on}}
	case accessibility.StateBusy:
		return []stateChange{{name: stateNameBusy, on: on}}
	case accessibility.StateInvalid:
		return []stateChange{{name: stateNameInvalidEntry, on: on}}
	case accessibility.StateOffscreen:
		// A node scrolled or clipped out of view is still VISIBLE in AT-SPI's sense, but it is no longer SHOWING.
		return []stateChange{{name: stateNameShowing, on: !on}}
	case accessibility.StateExpandable:
		return []stateChange{{name: stateNameExpandable, on: on}}
	case accessibility.StateExpanded:
		return []stateChange{{name: stateNameExpanded, on: on}}
	case accessibility.StateChecked:
		return checkedStateChanges(ev)
	case accessibility.StateIgnored, accessibility.StateProtected, accessibility.StateNone:
		// A node becoming ignored, or stopping being ignored, joins or leaves the reported hierarchy, which arrives as
		// a node added or removed rather than as a state change, and AT-SPI has no state for a password field: it has
		// the PASSWORD_TEXT role instead, which is part of the cache item rather than of the state set.
		return nil
	default:
		return nil
	}
}

// checkedStateChanges returns the states a change to a node's check state is reported as. CHECKED is the on state and
// INDETERMINATE is the mixed one, so moving between them changes both.
func checkedStateChanges(ev *accessibility.Event) []stateChange {
	was := check.Extract(ev.Old)
	now := check.Extract(ev.New)
	changes := []stateChange{{name: stateNameChecked, on: now == check.On}}
	if was == check.Mixed || now == check.Mixed {
		changes = append(changes, stateChange{name: stateNameIndeterminate, on: now == check.Mixed})
	}
	return changes
}

// emitTextChanged announces that runes have been inserted into or deleted from a node's text. The offsets are rune
// indexes, which is what AT-SPI means by characters.
func (a *Adapter) emitTextChanged(data *windowData, ev *accessibility.Event, detail, text string) {
	if reportedNode(data, ev.Node) == nil {
		return
	}
	a.emit(NodePath(ev.Node), InterfaceEventObject, signalTextChanged, detail, int32(ev.Start), int32(ev.Length),
		variantString(text))
}

// emitTextSelectionChanged announces that the caret has moved, and, when there is a range rather than just a caret,
// that the selection has changed as well. An assistive technology reads the selection back from the object, so the
// second signal carries nothing.
func (a *Adapter) emitTextSelectionChanged(data *windowData, ev *accessibility.Event) {
	if reportedNode(data, ev.Node) == nil {
		return
	}
	a.emit(NodePath(ev.Node), InterfaceEventObject, signalTextCaretMoved, "", int32(ev.Start+ev.Length), 0,
		variantInt32(0))
	if ev.Length != 0 {
		a.emit(NodePath(ev.Node), InterfaceEventObject, signalTextSelectionChanged, "", 0, 0, variantString(""))
	}
}

// emitNodeAdded announces a node that has joined a published window: the cache item first, so that an assistive
// technology already knows what the node is when it is told where it went.
func (a *Adapter) emitNodeAdded(data *windowData, id accessibility.NodeID) {
	n := reportedNode(data, id)
	if n == nil {
		return
	}
	parent := data.parent(id)
	if parent == 0 {
		// A node with no reported parent is the window's own root, which joins the tree as a window rather than as a
		// child of anything inside one.
		return
	}
	index := data.indexInParent(id)
	a.emitCacheAdd((&nodeObject{a: a, data: data, node: n}).cacheItem(index))
	a.emit(NodePath(parent), InterfaceEventObject, signalChildrenChanged, detailAdd, int32(index), 0,
		variantRef(a.reference(id)))
}

// emitNodeRemoved announces a node that has left a published window. Everything it says has to come from the snapshot
// the node was last in, since the new one no longer holds it: where it was, and which node it was under.
func (a *Adapter) emitNodeRemoved(prior *windowData, id accessibility.NodeID) {
	if reportedNode(prior, id) == nil {
		return
	}
	if parent := prior.parent(id); parent != 0 {
		a.emit(NodePath(parent), InterfaceEventObject, signalChildrenChanged, detailRemove,
			int32(prior.indexInParent(id)), 0, variantRef(a.reference(id)))
	}
	a.emitCacheRemove(a.reference(id))
}

// emitBoundsChanged announces that a window has moved or been resized. Only window roots report it: the nodes inside a
// window move whenever anything is scrolled or relaid out, and an assistive technology asks for the extents it needs
// rather than remembering them.
func (a *Adapter) emitBoundsChanged(data *windowData, id accessibility.NodeID) {
	n := reportedNode(data, id)
	if n == nil || id != data.tree.Root {
		return
	}
	x, y, w, h := data.extents(n, CoordScreen)
	a.emit(NodePath(id), InterfaceEventObject, signalBoundsChanged, "", 0, 0, variantRect(x, y, w, h))
}

// emit queues one AT-SPI event signal. Every event has the same shape — the detail string, two integers whose meaning
// depends on the event, the value the event carries, and a dictionary of the sender's properties that this package
// always leaves empty — and is sent from the path of the object it concerns.
func (a *Adapter) emit(path dbus.ObjectPath, iface, member, detail string, detail1, detail2 int32, data dbus.Variant) {
	a.conn.EmitWithSignature(path, iface, member, eventSignature, detail, detail1, detail2, data, dbus.Dict{})
}

// emitCacheAdd queues the cache object's signal that says a node has joined the hierarchy, carrying everything an
// assistive technology would otherwise have to ask for.
func (a *Adapter) emitCacheAdd(item dbus.Struct) {
	a.conn.EmitWithSignature(CachePath, InterfaceCache, signalAddAccessible, cacheItemSignature, item)
}

// emitCacheRemove queues the cache object's signal that says a node is gone.
func (a *Adapter) emitCacheRemove(ref dbus.ObjectRef) {
	a.conn.EmitWithSignature(CachePath, InterfaceCache, signalRemoveAccessible, objectRefSignature, ref)
}

// reportedNode returns the node an event names, or nil if the snapshot has no object for it, which is the case for a
// node that is not there at all and for one that is ignored.
func reportedNode(d *windowData, id accessibility.NodeID) *accessibility.Node {
	if d == nil || id == 0 {
		return nil
	}
	n := d.node(id)
	if n == nil || n.Ignored {
		return nil
	}
	return n
}

// variantString returns a string as the value an event carries.
func variantString(s string) dbus.Variant {
	return dbus.Variant{Sig: "s", Value: s}
}

// variantInt32 returns an integer as the value an event carries. Zero is what the events that carry nothing use, since
// the value is not optional.
func variantInt32(v int32) dbus.Variant {
	return dbus.Variant{Sig: "i", Value: v}
}

// variantDouble returns a floating point number as the value an event carries.
func variantDouble(v float64) dbus.Variant {
	return dbus.Variant{Sig: "d", Value: v}
}

// variantRef returns an object reference as the value an event carries.
func variantRef(ref dbus.ObjectRef) dbus.Variant {
	return dbus.Variant{Sig: objectRefSignature, Value: ref}
}

// variantRect returns a rectangle, in physical pixels, as the value an event carries.
func variantRect(x, y, w, h int32) dbus.Variant {
	return dbus.Variant{Sig: extentsSignature, Value: dbus.Struct{x, y, w, h}}
}
