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
	signalActivate                = "Activate"
	signalActiveDescendantChanged = "ActiveDescendantChanged"
	signalAnnouncement            = "Announcement"
	signalAttributesChanged       = "AttributesChanged"
	signalBoundsChanged           = "BoundsChanged"
	signalChildrenChanged         = "ChildrenChanged"
	signalCreate                  = "Create"
	signalDeactivate              = "Deactivate"
	signalDestroy                 = "Destroy"
	signalFocus                   = "Focus"
	signalPropertyChange          = "PropertyChange"
	signalStateChanged            = "StateChanged"
	signalTextCaretMoved          = "TextCaretMoved"
	signalTextChanged             = "TextChanged"
	signalTextSelectionChanged    = "TextSelectionChanged"
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

// publication is one publish's worth of context: the snapshot the window had before it, the snapshot it has now, and
// what the signals sent so far have already said.
//
// The events of a publish are not independent, which is why they cannot each be turned into signals on their own. A
// window that becomes active and moves its focus in the same publish produces both a WindowActivated and a
// FocusChanged naming the same node, and the focus has to be announced once rather than twice. One logical value
// change arrives as both a ValueChanged and a NumberChanged, and AT-SPI has one property for the two. And a client
// applies each children-changed as it arrives, so the second removal from one parent has to be numbered against the
// list the first one left behind rather than against the snapshot both came from.
type publication struct {
	prior *windowData
	data  *windowData
	// removed holds, per parent, the indexes within the prior snapshot of the children already announced as removed.
	removed map[accessibility.NodeID][]int
	// valueAnnounced holds the nodes whose value has already been announced.
	valueAnnounced map[accessibility.NodeID]bool
	// active holds the containers that manage their own descendants along with which descendant has become the
	// current one, which is sent once per container after every other signal of the publish.
	active []activeDescendant
	// focusAnnounced reports that where the focus has gone has already been said.
	focusAnnounced bool
}

// activeDescendant is one container that manages its own descendants together with the descendant within it that has
// become the current one.
type activeDescendant struct {
	container  accessibility.NodeID
	descendant accessibility.NodeID
}

// priorFocus returns the node that held the keyboard focus before this publish, or zero if none did or there is no
// snapshot before this one.
func (p *publication) priorFocus() accessibility.NodeID {
	if p.prior == nil {
		return 0
	}
	return p.prior.tree.Focus
}

// removalIndex returns the index a children-changed:remove has to carry for a child of the given parent, which is
// where the child sits in the list the client has now rather than where it sat in the snapshot before this publish.
// index is the latter. One publish can take several children away from the same parent, and a client applies each
// signal as it arrives, so every removal after the first has to be numbered against what the earlier ones left.
func (p *publication) removalIndex(parent accessibility.NodeID, index int) int {
	adjusted := index
	for _, done := range p.removed[parent] {
		if done < index {
			adjusted--
		}
	}
	if p.removed == nil {
		p.removed = make(map[accessibility.NodeID][]int)
	}
	p.removed[parent] = append(p.removed[parent], index)
	return adjusted
}

// noteActiveDescendant records which descendant of a container that manages its own descendants is now the current one,
// replacing whatever was recorded for that container before while keeping the order the containers were first noted
// in, so that a publish that says several things about the same container still sends one signal and a publish that
// touches several containers sends them in a fixed order.
func (p *publication) noteActiveDescendant(container, descendant accessibility.NodeID) {
	for i := range p.active {
		if p.active[i].container == container {
			p.active[i].descendant = descendant
			return
		}
	}
	p.active = append(p.active, activeDescendant{container: container, descendant: descendant})
}

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
	pub := &publication{prior: prior, data: data}
	if prior == nil {
		// A window nobody has been told about yet needs its whole hierarchy announced, which covers everything the
		// events could say: the only event a first snapshot produces is the focus it already has.
		a.emitWindowAdded(ws, pub)
	} else {
		for i := range events {
			a.emitEvent(pub, &events[i])
		}
	}
	a.emitActiveDescendants(pub)
}

// emitEvent sends the signals for one event.
func (a *Adapter) emitEvent(pub *publication, ev *accessibility.Event) {
	data := pub.data
	switch ev.Kind {
	case accessibility.FocusChanged:
		a.emitFocusChanged(pub, ev)
	case accessibility.NameChanged:
		a.emitPropertyChange(data, ev.Node, propertyAccessibleName, variantString(ev.New))
	case accessibility.DescriptionChanged:
		a.emitPropertyChange(data, ev.Node, propertyAccessibleDescription, variantString(ev.New))
	case accessibility.ValueChanged, accessibility.NumberChanged:
		a.emitValueChanged(pub, ev.Node)
	case accessibility.StateChanged:
		a.emitStateChanged(pub, ev)
	case accessibility.TextInserted:
		a.emitTextChanged(data, ev, detailInsert, ev.New)
	case accessibility.TextDeleted:
		a.emitTextChanged(data, ev, detailDelete, ev.Old)
	case accessibility.TextSelectionChanged:
		a.emitTextSelectionChanged(data, ev)
	case accessibility.NodeAdded:
		a.emitNodeAdded(data, ev.Node)
	case accessibility.NodeRemoved:
		a.emitNodeRemoved(pub, ev.Node)
	case accessibility.BoundsChanged:
		a.emitBoundsChanged(data, ev.Node)
	case accessibility.SortChanged:
		// AT-SPI reports a node's sort direction as an object attribute, so a change to it is a change to the
		// attributes rather than to anything with an interface of its own.
		if reportedNode(data, ev.Node) != nil {
			a.emit(NodePath(ev.Node), InterfaceEventObject, signalAttributesChanged, "", 0, 0, variantInt32(0))
		}
	case accessibility.WindowActivated:
		a.emitWindowActivated(pub)
	case accessibility.WindowDeactivated:
		a.emitWindowDeactivated(pub)
	case accessibility.Announcement:
		a.emitAnnouncement(ev.New)
	case accessibility.ChildrenChanged:
		// Nothing: the additions and removals that changed the list are reported one by one as NodeAdded and
		// NodeRemoved, each with the index it happened at, which is what AT-SPI's ChildrenChanged carries. A list whose
		// membership is the same but whose order is not goes unreported in V1.
	default:
	}
}

// emitValueChanged announces that a node's value has changed. Unison reports one such change twice — as the textual
// value the widget shows and as the number behind it — but AT-SPI has a single accessible-value property for both, and
// ATK, which every assistive technology was written against, defines it as a number. So the number is what is sent,
// once per node however many of the two events arrived, and a node that has no number sends nothing: it has no
// org.a11y.atspi.Value interface for a client to read a value back from, and what such a control shows is its name or
// its text instead.
func (a *Adapter) emitValueChanged(pub *publication, id accessibility.NodeID) {
	n := reportedNode(pub.data, id)
	if n == nil || !n.HasNumber || pub.valueAnnounced[id] {
		return
	}
	if pub.valueAnnounced == nil {
		pub.valueAnnounced = make(map[accessibility.NodeID]bool)
	}
	pub.valueAnnounced[id] = true
	a.emitPropertyChange(pub.data, id, propertyAccessibleValue, variantDouble(n.Number))
}

// emitWindowAdded announces a window that has just joined the accessibility tree: one cache item per reported node, the
// window's own Create signal, the application root's ChildrenChanged, and then, if the window is already the active
// one, everything that says so.
//
// The cache items come first for the same reason [Adapter.emitNodeAdded] sends a node's ahead of where it went: an
// assistive technology should already know what an object is by the time it is pointed at it. A window:create naming a
// window the client has never heard of leaves it to ask, over the bus, everything it was about to be told.
func (a *Adapter) emitWindowAdded(ws *windowState, pub *publication) {
	data := pub.data
	root := data.root()
	if root == nil {
		return
	}
	index := a.windowIndex(ws)
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
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalCreate, "", 0, 0, variantString(root.Name))
	a.emit(RootPath, InterfaceEventObject, signalChildrenChanged, detailAdd, int32(index), 0,
		variantRef(a.reference(root.ID)))
	if data.active() {
		a.emitWindowActivated(pub)
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
func (a *Adapter) emitWindowActivated(pub *publication) {
	root := pub.data.root()
	if root == nil {
		return
	}
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalActivate, "", 0, 0, variantString(root.Name))
	a.emit(NodePath(root.ID), InterfaceEventObject, signalStateChanged, stateNameActive, 1, 0, variantInt32(0))
	a.emitFocusMoved(pub, pub.data.tree.Focus)
}

// emitWindowDeactivated announces that a window has stopped being the active one, and that its focus is therefore no
// longer the focus.
//
// Which node loses the state is read from the snapshot before this one. A window very often loses the focus and moves
// it in the same publish — clicking a control in another window does both — and it is the node that actually held the
// focus while the window was active that has to be told it no longer does. The focus is announced here rather than by
// the FocusChanged of the same publish, which says nothing at all about a window that is no longer active.
func (a *Adapter) emitWindowDeactivated(pub *publication) {
	root := pub.data.root()
	if root == nil {
		return
	}
	a.emit(NodePath(root.ID), InterfaceEventWindow, signalDeactivate, "", 0, 0, variantString(root.Name))
	a.emit(NodePath(root.ID), InterfaceEventObject, signalStateChanged, stateNameActive, 0, 0, variantInt32(0))
	pub.focusAnnounced = true
	a.emitFocusLost(pub.data, pub.priorFocus())
}

// emitFocusChanged announces that the keyboard focus has moved within a window, which is the pair of signals an
// assistive technology relies on most: the node that had the focus loses the state, and the one that has it now gains
// it.
//
// Nothing is sent while the window is not the active one. AT-SPI's FOCUSED state belongs to the active window's focus
// alone, and an assistive technology that is told about a focus move in a window the user is not looking at follows it
// there. Nothing is sent either when the window became active in this same publish, since activating it has already
// said where its focus is; announcing it twice has a client hear the same object named twice and, worse, receive the
// old node's focused 0 after the new one's focused 1.
func (a *Adapter) emitFocusChanged(pub *publication, ev *accessibility.Event) {
	if !pub.data.active() || pub.focusAnnounced {
		return
	}
	a.emitFocusMoved(pub, ev.Node)
}

// emitFocusMoved announces that a node has taken the keyboard focus from whichever node held it before this publish,
// and records that the publish has now said where the focus is.
func (a *Adapter) emitFocusMoved(pub *publication, id accessibility.NodeID) {
	pub.focusAnnounced = true
	// Which node had the focus is only in the snapshot before this one, but whether it is still worth telling anyone
	// about depends on the new one: a node that has left the tree has already been reported as gone, and saying that
	// something that no longer exists has lost the focus would only send an assistive technology looking for it.
	if old := pub.priorFocus(); old != 0 && old != id {
		a.emitFocusLost(pub.data, old)
	}
	a.emitFocusGained(pub.data, id)
	a.noteActiveDescendant(pub, id)
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
// one AT-SPI state, since AT-SPI splits some of what Unison holds in one flag, and two of them are not states to an
// assistive technology at all but changes to the shape of the hierarchy or to where the user is within it.
func (a *Adapter) emitStateChanged(pub *publication, ev *accessibility.Event) {
	if ev.State == accessibility.StateIgnored {
		a.emitIgnoredChanged(pub, ev)
		return
	}
	if reportedNode(pub.data, ev.Node) == nil {
		return
	}
	if ev.State == accessibility.StateSelected && ev.New == trueValue {
		// Selecting a row is how the current one moves in a table that manages its own descendants, which is the one
		// thing a client that honors that state has no other way of learning.
		a.noteActiveDescendant(pub, ev.Node)
	}
	for _, one := range stateChanges(ev) {
		var detail1 int32
		if one.on {
			detail1 = 1
		}
		a.emit(NodePath(ev.Node), InterfaceEventObject, signalStateChanged, one.name, detail1, 0, variantInt32(0))
	}
}

// emitIgnoredChanged announces a node whose Ignored flag has flipped. [accessibility.Diff] reports that as nothing but
// a state change, since an ignored node stays in the tree so that hit testing and coordinate clipping go on working,
// but to an assistive technology an ignored node has no object at all: the flip is exactly a node joining or leaving
// the reported hierarchy, and it reaches one in normal use, as a scroll bar appears and disappears or a group is given
// a name. Reporting nothing leaves the client's cached hierarchy permanently at odds with what GetChildren answers.
//
// The node's own reported children move with it, from the nearest reported ancestor onto the node or the other way
// about, and are told where they are now. Nothing deeper has to be: only the direct children change parents.
func (a *Adapter) emitIgnoredChanged(pub *publication, ev *accessibility.Event) {
	if ev.New == trueValue {
		// The node has left the reported hierarchy, so its children are placed under the ancestor that holds them now
		// after it has gone, rather than being left pointing at an object the client has just been told to forget.
		a.emitNodeRemoved(pub, ev.Node)
		for _, id := range pub.prior.unignoredChildren(ev.Node) {
			a.emitNodeAdded(pub.data, id)
		}
		return
	}
	a.emitNodeAdded(pub.data, ev.Node)
	for _, id := range pub.data.unignoredChildren(ev.Node) {
		a.emitNodeAdded(pub.data, id)
	}
}

// noteActiveDescendant records that a node has become the current one within the nearest container above it that
// claims ATSPI_STATE_MANAGES_DESCENDANTS, if there is one. Nothing is sent yet: a publish can say several things about
// the same container, and only the last of them is the answer.
func (a *Adapter) noteActiveDescendant(pub *publication, id accessibility.NodeID) {
	if container := managesDescendantsAncestor(pub.data, id); container != 0 {
		pub.noteActiveDescendant(container, id)
	}
}

// managesDescendantsAncestor returns the nearest reported ancestor of a node that claims
// ATSPI_STATE_MANAGES_DESCENDANTS, or zero when none does.
func managesDescendantsAncestor(data *windowData, id accessibility.NodeID) accessibility.NodeID {
	for parent := data.parent(id); parent != 0; parent = data.parent(parent) {
		n := data.node(parent)
		if n == nil {
			return 0
		}
		if ManagesDescendants(n) {
			return parent
		}
	}
	return 0
}

// emitActiveDescendants announces, for each container that manages its own descendants and whose current descendant
// moved during this publish, which one it is now. A container claiming ATSPI_STATE_MANAGES_DESCENDANTS has told its
// client not to walk or cache what is inside it, so this signal is the only way the client can follow the user through
// it; without it, exactly the large tables that state exists to make usable would be the ones a client could learn
// nothing about. It carries the descendant's place among its parent's children and a reference to it, which is what
// ATK's own bridge sends.
//
// These go out after everything else the publish has to say, so that the row an assistive technology is sent to has
// already been described and placed.
func (a *Adapter) emitActiveDescendants(pub *publication) {
	for _, one := range pub.active {
		a.emit(NodePath(one.container), InterfaceEventObject, signalActiveDescendantChanged, "",
			int32(pub.data.indexInParent(one.descendant)), 0, variantRef(a.reference(one.descendant)))
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
		// A node becoming ignored, or stopping being ignored, joins or leaves the reported hierarchy rather than
		// changing a state, and is turned into the signals that say so by [Adapter.emitIgnoredChanged] before it ever
		// gets here. AT-SPI has no state for a password field: it has the PASSWORD_TEXT role instead, which is part of
		// the cache item rather than of the state set.
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
// the node was last in, since the new one no longer holds it: where it was, and which node it was under. The index is
// then corrected for the removals this publish has already announced from the same parent — see
// [publication.removalIndex] — because a client applies each one as it arrives and the second of three children to go
// is no longer where the old snapshot had it by the time it is told about.
func (a *Adapter) emitNodeRemoved(pub *publication, id accessibility.NodeID) {
	prior := pub.prior
	if reportedNode(prior, id) == nil {
		return
	}
	if parent := prior.parent(id); parent != 0 {
		a.emit(NodePath(parent), InterfaceEventObject, signalChildrenChanged, detailRemove,
			int32(pub.removalIndex(parent, prior.indexInParent(id))), 0, variantRef(a.reference(id)))
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
