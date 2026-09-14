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
	"slices"

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
	signalSelectionChanged        = "SelectionChanged"
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
	stateNameActive             = "active"
	stateNameBusy               = "busy"
	stateNameCheckable          = "checkable"
	stateNameChecked            = "checked"
	stateNameCollapsed          = "collapsed"
	stateNameEditable           = "editable"
	stateNameEnabled            = "enabled"
	stateNameExpandable         = "expandable"
	stateNameExpanded           = "expanded"
	stateNameFocusable          = "focusable"
	stateNameFocused            = "focused"
	stateNameHorizontal         = "horizontal"
	stateNameIndeterminate      = "indeterminate"
	stateNameInvalidEntry       = "invalid-entry"
	stateNameManagesDescendants = "manages-descendants"
	stateNameModal              = "modal"
	stateNameMultiLine          = "multi-line"
	stateNameMultiselectable    = "multiselectable"
	stateNamePressed            = "pressed"
	stateNameReadOnly           = "read-only"
	stateNameSelectable         = "selectable"
	stateNameSelected           = "selected"
	stateNameSensitive          = "sensitive"
	stateNameShowing            = "showing"
	stateNameSingleLine         = "single-line"
	stateNameVertical           = "vertical"
)

// The detail strings of the property changes, which are the names of the ATK properties the values belong to. AT-SPI
// has no other spelling for them: the names are part of the protocol, and an assistive technology matches on
// "object:property-change:accessible-name" and its like.
const (
	propertyAccessibleName        = "accessible-name"
	propertyAccessibleDescription = "accessible-description"
	propertyAccessibleParent      = "accessible-parent"
	propertyAccessibleRole        = "accessible-role"
	propertyAccessibleValue       = "accessible-value"
)

// livePolite is ATSPI_LIVE_POLITE, from AtspiLive. An announcement carries it as detail1 to say that whatever the
// assistive technology is saying now may finish first.
const livePolite = 1

// trueValue is what a boolean state change holds in the event's New and Old, which is what strconv.FormatBool writes.
const trueValue = "true"

// publication is one publish's worth of context: the window being published, the snapshot it had before, the snapshot
// it has now, and what the signals sent so far have already said.
//
// The events of a publish are not independent, which is why they cannot each be turned into signals on their own. A
// window that becomes active and moves its focus in the same publish produces both a WindowActivated and a
// FocusChanged naming the same node, and the focus has to be announced once rather than twice. One logical value
// change arrives as both a ValueChanged and a NumberChanged, and AT-SPI has one property for the two. And a client
// applies each children-changed as it arrives, so every index one carries has to be an index into the list the client
// is holding at that moment rather than into either snapshot; see [publication.children].
type publication struct {
	prior *windowData
	data  *windowData
	ws    *windowState
	// lists holds, per parent, the child list the client has now: the one the parent had before this publish, with
	// every addition and removal announced since applied to it. A parent gains an entry the first time this publish
	// says anything about its children.
	lists map[accessibility.NodeID][]accessibility.NodeID
	// present holds what this publish has said about whether an object exists at all, for the nodes it has said
	// anything about. A node it holds nothing for is the one the snapshot before this publish reported, or did not.
	present map[accessibility.NodeID]bool
	// added holds the nodes whose arrival this publish has already announced, so that two events that both describe
	// the same arrival — a node gaining a child in the same publish in which it stops being ignored — announce it once.
	added map[accessibility.NodeID]bool
	// announcedStates holds the state changes already sent, so that two events that imply the same one do not have an
	// assistive technology announce it twice.
	announcedStates map[stateAnnouncement]bool
	// valueAnnounced holds the nodes whose value has already been announced.
	valueAnnounced map[accessibility.NodeID]bool
	// roleAnnounced holds the nodes whose role has already been announced.
	roleAnnounced map[accessibility.NodeID]bool
	// attributesAnnounced holds the nodes whose attributes have already been announced.
	attributesAnnounced map[accessibility.NodeID]bool
	// childrenAnnounced holds the parents whose child list has already been worked out, so that two events that both
	// describe the same list — one per ignored container under a reported ancestor — walk it once.
	childrenAnnounced map[accessibility.NodeID]bool
	// held holds the signals about a node's own change that could not be sent when they arrived, because the client
	// had not been told that the object exists yet. See [Adapter.emitHeld].
	held []heldSignal
	// waiting holds the nodes whose arrival cannot be announced yet, because the object that is to hold them has not
	// been announced itself. See [Adapter.flushWaiting].
	waiting []accessibility.NodeID
	// selection holds the containers whose selection moved, which is sent once per container after every other signal
	// of the publish.
	selection []accessibility.NodeID
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

// stateAnnouncement is one thing a publish has said about a node's states: which state, on which node, and which way
// it went.
type stateAnnouncement struct {
	state string
	node  accessibility.NodeID
	on    bool
}

// heldSignal is one signal about a node's own change that has been held back until the client has been told that the
// object it is sent from exists. It carries everything [Adapter.emit] needs, since what it describes is what the
// snapshots said when the event was dealt with rather than what they say by the time it goes out.
type heldSignal struct {
	iface   string
	member  string
	detail  string
	value   dbus.Variant
	node    accessibility.NodeID
	detail1 int32
	detail2 int32
}

// priorFocus returns the node that held the keyboard focus before this publish, or zero if none did or there is no
// snapshot before this one.
func (p *publication) priorFocus() accessibility.NodeID {
	if p.prior == nil {
		return 0
	}
	return p.prior.tree.Focus
}

// children returns the reported children the client holds for a parent right now, which is the list it had before this
// publish with every addition and removal announced since applied to it. A client applies each children-changed as it
// arrives and has nothing to go on but the index it carries, so this — rather than either snapshot — is what every one
// of those indexes has to be an index into.
func (p *publication) children(parent accessibility.NodeID) []accessibility.NodeID {
	if list, exists := p.lists[parent]; exists {
		return list
	}
	list := slices.Clone(p.prior.unignoredChildren(parent))
	if p.lists == nil {
		p.lists = make(map[accessibility.NodeID][]accessibility.NodeID)
	}
	p.lists[parent] = list
	return list
}

// holds reports whether the client still has a child in a parent's list.
func (p *publication) holds(parent, id accessibility.NodeID) bool {
	return slices.Contains(p.children(parent), id)
}

// noteAdd records that a child has been announced as joining a parent and returns the index the signal has to carry,
// which is where the child goes in the list the client is holding now. The place is the one that leaves the client's
// list in the order the new snapshot has: after every child that precedes it there and before every child that follows
// it. A sibling the list holds that the new snapshot does not is on its way out and is passed over, so that the child
// lands ahead of it rather than behind it.
func (p *publication) noteAdd(parent, id accessibility.NodeID) int {
	list := p.children(parent)
	order := p.data.unignoredChildren(parent)
	at := slices.Index(order, id)
	index := 0
	for i, existing := range list {
		if position := slices.Index(order, existing); position >= 0 && position < at {
			index = i + 1
		}
	}
	p.lists[parent] = slices.Insert(list, index, id)
	return index
}

// noteRemove records that a child has been announced as leaving a parent and returns the index the signal has to
// carry. ok is false when the client does not hold the child there at all, which is the case for one that an earlier
// signal of this publish has already taken away, and means there is nothing to announce.
func (p *publication) noteRemove(parent, id accessibility.NodeID) (index int, ok bool) {
	list := p.children(parent)
	index = slices.Index(list, id)
	if index < 0 {
		return 0, false
	}
	p.lists[parent] = slices.Delete(list, index, index+1)
	return index, true
}

// knows reports whether the client has an object for a node: one the snapshot before this publish reported, unless a
// signal since has said that it has arrived or gone.
func (p *publication) knows(id accessibility.NodeID) bool {
	if present, said := p.present[id]; said {
		return present
	}
	return reportedNode(p.prior, id) != nil
}

// notePresent records that a signal has said whether a node's object exists.
func (p *publication) notePresent(id accessibility.NodeID, present bool) {
	if p.present == nil {
		p.present = make(map[accessibility.NodeID]bool)
	}
	p.present[id] = present
}

// wait records that a node's arrival has to wait for the object that will hold it to be announced first.
func (p *publication) wait(id accessibility.NodeID) {
	if !slices.Contains(p.waiting, id) {
		p.waiting = append(p.waiting, id)
	}
}

// noteStateAnnounced records that a state of a node has been announced and reports whether the same thing had already
// been said. One change in the schema can imply another that a second event of the same publish reports too — an
// expandable node that opens says so through both its expandability and its expanded-ness — and saying it twice has an
// assistive technology announce something that happened once as though it happened twice.
func (p *publication) noteStateAnnounced(id accessibility.NodeID, state string, on bool) bool {
	one := stateAnnouncement{node: id, state: state, on: on}
	if p.announcedStates[one] {
		return true
	}
	if p.announcedStates == nil {
		p.announcedStates = make(map[stateAnnouncement]bool)
	}
	p.announcedStates[one] = true
	return false
}

// noteSelectionChanged records that the selection of a container has moved, keeping the order the containers were
// first noted in so that a publish that says several things about one container still sends one signal.
func (p *publication) noteSelectionChanged(container accessibility.NodeID) {
	if !slices.Contains(p.selection, container) {
		p.selection = append(p.selection, container)
	}
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
	pub := &publication{prior: prior, data: data, ws: ws}
	if prior == nil {
		// A window nobody has been told about yet needs its whole hierarchy announced, which covers everything the
		// events could say: the only event a first snapshot produces is the focus it already has.
		a.emitWindowAdded(ws, pub)
	} else {
		for i := range events {
			a.emitEvent(pub, &events[i])
		}
		a.emitHeld(pub)
	}
	a.emitSelectionChanges(pub)
	a.emitActiveDescendants(pub)
}

// emitEvent sends the signals for one event.
func (a *Adapter) emitEvent(pub *publication, ev *accessibility.Event) {
	data := pub.data
	switch ev.Kind {
	case accessibility.FocusChanged:
		a.emitFocusChanged(pub, ev)
	case accessibility.NameChanged:
		a.emitPropertyChange(pub, ev.Node, propertyAccessibleName, variantString(ev.New))
		// What this package reports a node as depends on its name, so naming an anonymous group turns it from a panel
		// into a grouping.
		a.emitRoleIfChanged(pub, ev.Node)
	case accessibility.DescriptionChanged:
		a.emitPropertyChange(pub, ev.Node, propertyAccessibleDescription, variantString(ev.New))
	case accessibility.RoleChanged:
		a.emitRoleIfChanged(pub, ev.Node)
	case accessibility.ValueChanged, accessibility.NumberChanged:
		a.emitValueChanged(pub, ev.Node)
	case accessibility.StateChanged:
		a.emitStateChanged(pub, ev)
	case accessibility.TextInserted:
		a.emitTextChanged(pub, ev, detailInsert, ev.New)
	case accessibility.TextDeleted:
		a.emitTextChanged(pub, ev, detailDelete, ev.Old)
	case accessibility.TextSelectionChanged:
		a.emitTextSelectionChanged(pub, ev)
	case accessibility.NodeAdded:
		a.emitNodeAdded(pub, ev.Node)
	case accessibility.NodeRemoved:
		a.emitNodeRemoved(pub, ev.Node)
	case accessibility.BoundsChanged:
		a.emitBoundsChanged(pub, data, ev.Node)
	case accessibility.SortChanged, accessibility.AttributesChanged:
		a.emitAttributesChanged(pub, ev.Node)
	case accessibility.WindowActivated:
		a.emitWindowActivated(pub)
	case accessibility.WindowDeactivated:
		a.emitWindowDeactivated(pub)
	case accessibility.ChildrenChanged:
		a.emitChildrenChanged(pub, ev.Node)
	default:
	}
}

// emitAttributesChanged announces that the secondary facts about a node have moved. AT-SPI reports a node's sort
// direction as an object attribute, and so it does most of what [accessibility.AttributesChanged] covers: the
// placeholder text, the level, the row and column indexes and the position in a set. A change to one of them is
// therefore a change to the attributes rather than to anything with an interface of its own. The relations that event
// also covers — what labels a node, what describes it and what it controls — have no AT-SPI signal of their own to be
// reported through, so this stands in for them as well, and a client that re-reads the attributes will re-read the
// relation set with them.
//
// It is sent once per node however many events arrive for it. [accessibility.Diff] reports a sort change and an
// attribute change as two separate events, so a column header whose sort direction and column index both moved in one
// publish would otherwise be announced twice and have an assistive technology re-read the same attribute set twice.
//
// The two facts that event carries which AT-SPI does not hold as attributes at all are sent as the state changes they
// really are; see [attributeStateChanges].
func (a *Adapter) emitAttributesChanged(pub *publication, id accessibility.NodeID) {
	n := reportedNode(pub.data, id)
	if n == nil || pub.attributesAnnounced[id] {
		return
	}
	if pub.attributesAnnounced == nil {
		pub.attributesAnnounced = make(map[accessibility.NodeID]bool)
	}
	pub.attributesAnnounced[id] = true
	a.emitOwnChange(pub, id, InterfaceEventObject, signalAttributesChanged, "", 0, 0, variantInt32(0))
	if prior := reportedNode(pub.prior, id); prior != nil {
		for _, one := range attributeStateChanges(prior, n) {
			a.emitStateChange(pub, id, one)
		}
	}
}

// attributeStateChanges returns the AT-SPI states that a change to a node's attributes really is. Three of the things
// [accessibility.AttributesChanged] covers are not attributes to AT-SPI at all: the orientation is the HORIZONTAL and
// VERTICAL states that [roleStates] puts in every state set, the number of rows is what [ManagesDescendants] derives
// MANAGES_DESCENDANTS from, and how many lines a control lays its text out over is the SINGLE_LINE and MULTI_LINE pair
// that [lineState] decides. A client caches a state set until something retracts it, so leaving them out leaves a
// scroll bar that has been turned on its side reported the old way for the life of the window, a table filtered down to
// a handful of rows still claiming to manage descendants — while [Adapter.emitActiveDescendants], which reads the new
// snapshot, stops sending anything about it, leaving the client with neither route to the current row — and a
// single-line field that has wrapped onto a second line still read out as one run, with no line-by-line navigation
// through it.
func attributeStateChanges(prior, n *accessibility.Node) []stateChange {
	var changes []stateChange
	if prior.Orientation != n.Orientation {
		// [States] gives a node at most one of the two states and a node with no orientation neither of them, so only
		// the side a snapshot actually reported is moved; the other was never sent and must not be retracted.
		changes = appendOrientationChange(changes, prior.Orientation, false)
		changes = appendOrientationChange(changes, n.Orientation, true)
	}
	if was, now := ManagesDescendants(prior), ManagesDescendants(n); was != now {
		changes = append(changes, stateChange{name: stateNameManagesDescendants, on: now})
	}
	// As with the orientation, a node whose text has no line count at all — one carrying no text, or a role that never
	// has either state — has nothing to retract and nothing to gain, so only the side a snapshot actually reported is
	// moved.
	if was, now := lineStateName(prior), lineStateName(n); was != now {
		if was != "" {
			changes = append(changes, stateChange{name: was, on: false})
		}
		if now != "" {
			changes = append(changes, stateChange{name: now, on: true})
		}
	}
	return changes
}

// lineStateName returns the detail string of the state that says how many lines a node lays its text out over, or an
// empty string when the node's state set holds neither of them.
func lineStateName(n *accessibility.Node) string {
	state, ok := lineState(n)
	switch {
	case !ok:
		return ""
	case state == StateMultiLine:
		return stateNameMultiLine
	default:
		return stateNameSingleLine
	}
}

// appendOrientationChange appends the state change for one side of the orientation pair, if there is one to append: a
// node with no orientation has neither state and so has nothing to gain or lose.
func appendOrientationChange(changes []stateChange, orientation accessibility.Orientation, on bool) []stateChange {
	switch orientation {
	case accessibility.OrientationHorizontal:
		return append(changes, stateChange{name: stateNameHorizontal, on: on})
	case accessibility.OrientationVertical:
		return append(changes, stateChange{name: stateNameVertical, on: on})
	case accessibility.OrientationNone:
	}
	return changes
}

// emitChildrenChanged reports the children a node has gained from, or lost to, another parent of the same window.
//
// A node that joins or leaves the window arrives as NodeAdded or NodeRemoved, each carrying the index it happened at,
// which is what AT-SPI's children-changed says; nothing more is needed for those. A node that merely moves from one
// parent to another keeps its id, so [accessibility.Diff] reports it as nothing but a pair of these events — one for
// the parent it left and one for the parent it joined — and a client that keeps a child list per object, which
// libatspi does, would otherwise hold it under its old parent and never under its new one for as long as the window
// lives. Both ends of the move are therefore announced, along with the node's new parent.
//
// A list whose membership is the same but whose order is not is reported as well; see [Adapter.emitChildrenReordered].
//
// The event often names a node that has no object at all. The root package marks every plain unnamed layout Group
// ignored, so an ordinary panel whose children are rearranged produces one of these for a node an assistive technology
// has never heard of — while the child list that really changed is the one of the nearest reported ancestor, which the
// ignored container's children are spliced into and which [accessibility.Diff] has nothing to say about because its own
// Children field never moved. Saying nothing there would leave libatspi, which keeps a child array per object, reading
// those children in the order they had before for the life of the window, which is exactly what
// [Adapter.emitChildrenReordered] exists to prevent. So the change is worked out against the reported ancestor instead,
// on both sides, since an ignored container that has itself moved has a different one in each snapshot.
func (a *Adapter) emitChildrenChanged(pub *publication, id accessibility.NodeID) {
	switch {
	case reportedNode(pub.prior, id) != nil && reportedNode(pub.data, id) != nil:
		a.emitChildListChanges(pub, id)
	case reportedNode(pub.prior, id) == nil && reportedNode(pub.data, id) == nil:
		// A container with no object in either snapshot has no child list of its own for anything to have moved within.
		// What moved within it moved within the list of the ancestor that stands in for it.
		a.emitChildListChanges(pub, pub.prior.parent(id))
		a.emitChildListChanges(pub, pub.data.parent(id))
	default:
		// A node that has just gained or lost an object is a whole subtree arriving or leaving rather than a move, and
		// its children change parents with it, which [Adapter.emitIgnoredChanged] deals with.
	}
}

// emitChildListChanges announces what has happened to one reported node's list of children: the children it has gained
// from, or lost to, another parent of the same window, and then the ones that are where they were but no longer in the
// order they were in. See [Adapter.emitChildrenChanged], which decides whose list an event is really about.
//
// It is worked out once per parent however many events lead to it. Two ignored containers under one reported ancestor
// produce an event each, and both describe the same list; the second pass would find nothing left to say, since every
// signal it sends is worked out from the list the client is holding by then, but it would walk the whole list again to
// discover that.
func (a *Adapter) emitChildListChanges(pub *publication, id accessibility.NodeID) {
	if reportedNode(pub.prior, id) == nil || reportedNode(pub.data, id) == nil {
		return
	}
	if pub.childrenAnnounced[id] {
		return
	}
	if pub.childrenAnnounced == nil {
		pub.childrenAnnounced = make(map[accessibility.NodeID]bool)
	}
	pub.childrenAnnounced[id] = true
	for _, child := range pub.prior.unignoredChildren(id) {
		if reportedNode(pub.data, child) != nil && pub.data.parent(child) != id {
			// The child is still in the window, under something else, so this parent has lost it rather than the
			// window having done so. Where it has gone is said here as well, since the object that holds it now may
			// have no event of its own: a node moved into an ignored container belongs to the nearest reported
			// ancestor, whose own list of children has not changed and which [accessibility.Diff] therefore says
			// nothing about.
			a.emitChildRemoved(pub, id, child)
			a.emitNodeAdded(pub, child)
		}
	}
	for _, child := range pub.data.unignoredChildren(id) {
		if reportedNode(pub.prior, child) != nil && pub.prior.parent(child) != id {
			a.emitNodeAdded(pub, child)
		}
	}
	a.emitChildrenReordered(pub, id)
}

// emitChildrenReordered announces the children that are where they were but are no longer in the order they were in.
// AT-SPI has no signal for a reordering, so each child that has moved is taken out of the client's list and put back
// where the new snapshot has it, which is the pair of signals ATK's own bridge sends for the same thing.
//
// Saying nothing would be wrong rather than merely incomplete: libatspi keeps an array of children per object and
// answers GetChildren and GetChildAtIndex from it, so sorting a table small enough for every row to be described —
// which keeps the same node ids in a new order — would leave an assistive technology reading the rows in the order they
// had before the sort for the life of the window.
//
// Only the children the client is already holding can have moved within its list. One that is still on its way in is
// left to the signal that announces it, which puts it where the new snapshot has it.
func (a *Adapter) emitChildrenReordered(pub *publication, id accessibility.NodeID) {
	order := pub.data.unignoredChildren(id)
	held := make([]accessibility.NodeID, 0, len(order))
	for _, child := range order {
		if pub.holds(id, child) {
			held = append(held, child)
		}
	}
	for i, child := range held {
		// The list is re-read on every pass, since the signals sent so far have moved the children before this one
		// into place and each index has to be an index into the list the client is holding at that moment.
		if list := pub.children(id); i < len(list) && list[i] == child {
			continue
		}
		a.emitChildRemoved(pub, id, child)
		a.emitChildAdded(pub, id, child)
	}
}

// emitValueChanged announces that a node's value has changed. Unison reports one such change twice — as the textual
// value the widget shows and as the number behind it — but AT-SPI has a single accessible-value property for both, and
// ATK, which every assistive technology was written against, defines it as a number. So the number is what is sent,
// once per node however many of the two events arrived.
//
// A node whose value is text rather than a number has no accessible-value to report at all, and is announced through
// the read-only org.a11y.atspi.Text interface synthesized from that value instead; see [textualValue] and
// [Adapter.emitTextValueChanged].
func (a *Adapter) emitValueChanged(pub *publication, id accessibility.NodeID) {
	n := reportedNode(pub.data, id)
	if n == nil || pub.valueAnnounced[id] {
		return
	}
	if pub.valueAnnounced == nil {
		pub.valueAnnounced = make(map[accessibility.NodeID]bool)
	}
	pub.valueAnnounced[id] = true
	if n.HasNumber {
		a.emitPropertyChange(pub, id, propertyAccessibleValue, variantDouble(n.Number))
		return
	}
	a.emitTextValueChanged(pub, id, n)
}

// emitTextValueChanged announces a change to a node's textual value as the whole of the old value being deleted and the
// whole of the new one inserted, which is what the text an assistive technology can read of such a node actually did.
// Orca re-reads a control when its text changes and ignores it when nothing says anything, so this is what makes the
// item a popup menu has just chosen, or the ink a color well has just been given, announced at all rather than only
// discoverable by asking.
//
// Nothing is sent for a node that carries text of its own: the real text has its own events, and the value beside it is
// a description of the same thing. Both halves are read from the two snapshots rather than from the event, since the
// same node may arrive here from a NumberChanged, whose values are numbers.
func (a *Adapter) emitTextValueChanged(pub *publication, id accessibility.NodeID, n *accessibility.Node) {
	old := textualValue(reportedNode(pub.prior, id))
	now := textualValue(n)
	if old == now {
		return
	}
	if old != "" {
		a.emitOwnChange(pub, id, InterfaceEventObject, signalTextChanged, detailDelete, 0, int32(len([]rune(old))),
			variantString(old))
	}
	if now != "" {
		a.emitOwnChange(pub, id, InterfaceEventObject, signalTextChanged, detailInsert, 0, int32(len([]rune(now))),
			variantString(now))
	}
}

// emitRoleIfChanged announces that a node is no longer the kind of thing it was, which a live control really does
// become: a label turns into an image when its text is swapped for a drawable, and a button turns into a toggle button
// when it is made sticky. AT-SPI carries it as the accessible-role property, whose value is the role's number rather
// than its name, since the number is what the cache item holds and what an assistive technology compares against.
//
// What moved is worked out from the two snapshots rather than from the event, because what this package reports depends
// on more of the node than the schema's role does: an unnamed group is a panel while a named one is a grouping, a
// protected text field is password text, and a menu item that carries a check is a check menu item. Each of those
// arrives as an event about something other than the role, and each of them leaves a client holding the old role
// forever unless it is told. It is sent once per node however many of those events arrive, and nothing is sent for a
// node whose reported role has not actually moved.
func (a *Adapter) emitRoleIfChanged(pub *publication, id accessibility.NodeID) {
	n := reportedNode(pub.data, id)
	prior := reportedNode(pub.prior, id)
	if n == nil || prior == nil || pub.roleAnnounced[id] || MapRole(prior) == MapRole(n) {
		return
	}
	if pub.roleAnnounced == nil {
		pub.roleAnnounced = make(map[accessibility.NodeID]bool)
	}
	pub.roleAnnounced[id] = true
	a.emitPropertyChange(pub, id, propertyAccessibleRole, variantUint32(uint32(MapRole(n))))
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
//
// A node that has been reparented into another window that is still published is left out of the removals: its object
// goes on answering, and telling an assistive technology that a live object is gone would take it out of that client's
// cache with nothing to ever put it back, since the window that holds it now sees no change of its own.
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
		if !n.Ignored && !a.publishedElsewhere(n.ID, ws) {
			a.emitCacheRemove(a.reference(n.ID))
		}
		return true
	})
}

// publishedElsewhere reports whether a node belongs to a published window other than the given one, which is what a
// node that has been reparented from one window into another does. Such a node is still reachable, so nothing may say
// that its object has gone.
func (a *Adapter) publishedElsewhere(id accessibility.NodeID, ws *windowState) bool {
	other := a.windowFor(id)
	return other != nil && other != ws
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
func (a *Adapter) emitPropertyChange(pub *publication, id accessibility.NodeID, property string, value dbus.Variant) {
	if reportedNode(pub.data, id) == nil {
		return
	}
	a.emitOwnChange(pub, id, InterfaceEventObject, signalPropertyChange, property, 0, 0, value)
}

// emitOwnChange queues one signal that reports a change to a node itself, holding it back while the client has not been
// told that the object exists.
//
// [accessibility.Diff] orders a node's own changes ahead of the StateChanged that says it has stopped being ignored,
// which is the event this package turns into the node's arrival, so a node that gains a name in the same publish in
// which it joins the reported hierarchy has its name change reported first. Sending it then contradicts the rule the
// additions follow — an assistive technology should already know what an object is by the time it is pointed at it —
// and has libatspi resolve the path with ref_accessible into a parentless placeholder plus a round trip to fill it in.
// Holding it until the object has been announced is what keeps the order right however the events happen to arrive; see
// [Adapter.emitHeld].
//
// pub is nil for a change that belongs to no publish at all, which is the bounds change a geometry update sends: the
// window's object was announced when the window was first published, long before it was moved.
func (a *Adapter) emitOwnChange(pub *publication, id accessibility.NodeID, iface, member, detail string,
	detail1, detail2 int32, value dbus.Variant,
) {
	if pub == nil || pub.knows(id) {
		a.emit(NodePath(id), iface, member, detail, detail1, detail2, value)
		return
	}
	pub.held = append(pub.held, heldSignal{
		node:    id,
		iface:   iface,
		member:  member,
		detail:  detail,
		detail1: detail1,
		detail2: detail2,
		value:   value,
	})
}

// emitHeld sends the signals that were held back for objects the client has been told about since, in the order they
// were held in, once everything the publish has to say about the shape of the hierarchy has been said. Whatever is
// still held for an object the client never gained is dropped rather than sent into nowhere, exactly as an addition
// that never became announceable is.
func (a *Adapter) emitHeld(pub *publication) {
	for _, one := range pub.held {
		if pub.knows(one.node) {
			a.emit(NodePath(one.node), one.iface, one.member, one.detail, one.detail1, one.detail2, one.value)
		}
	}
	pub.held = nil
}

// emitStateChanged announces that one of a node's states has changed. A single change in the schema can be more than
// one AT-SPI state, since AT-SPI splits some of what Unison holds in one flag, and two of them are not states to an
// assistive technology at all but changes to the shape of the hierarchy or to where the user is within it.
func (a *Adapter) emitStateChanged(pub *publication, ev *accessibility.Event) {
	if ev.State == accessibility.StateIgnored {
		a.emitIgnoredChanged(pub, ev)
		return
	}
	n := reportedNode(pub.data, ev.Node)
	if n == nil {
		return
	}
	if ev.State == accessibility.StateSelected {
		// The container is told that its selection moved, which is what an assistive technology following a combo box
		// or a page tab list listens for, and it is told after the items themselves have said what happened to them.
		if container := selectionAncestor(pub.data, ev.Node); container != 0 {
			pub.noteSelectionChanged(container)
		}
		if ev.New == trueValue {
			// Selecting a row is how the current one moves in a table that manages its own descendants, which is the
			// one thing a client that honors that state has no other way of learning.
			a.noteActiveDescendant(pub, ev.Node)
		}
	}
	for _, one := range stateChanges(reportedNode(pub.prior, ev.Node), n, ev) {
		a.emitStateChange(pub, ev.Node, one)
	}
	// Several of the things this package reports a node as are decided by more than the schema's role, and two of them
	// — whether a text field is protected, and whether a menu item carries a check — arrive as state changes.
	a.emitRoleIfChanged(pub, ev.Node)
}

// emitStateChange announces that one AT-SPI state of a node has moved, unless this publish has already said the same
// thing. One change in the schema can imply another that a second event of the same publish reports too, and saying it
// twice has an assistive technology announce something that happened once as though it happened twice.
func (a *Adapter) emitStateChange(pub *publication, id accessibility.NodeID, one stateChange) {
	if pub.noteStateAnnounced(id, one.name, one.on) {
		return
	}
	var detail1 int32
	if one.on {
		detail1 = 1
	}
	a.emitOwnChange(pub, id, InterfaceEventObject, signalStateChanged, one.name, detail1, 0, variantInt32(0))
}

// selectionAncestor returns the nearest reported ancestor of a node that implements org.a11y.atspi.Selection, or zero
// when none does.
func selectionAncestor(data *windowData, id accessibility.NodeID) accessibility.NodeID {
	for parent := data.parent(id); parent != 0; parent = data.parent(parent) {
		n := data.node(parent)
		if n == nil {
			return 0
		}
		if supportsSelection(n.Role) {
			return parent
		}
	}
	return 0
}

// emitSelectionChanges announces, for each container whose selection moved during this publish, that it did. AT-SPI
// reports a selection twice over — as the SELECTED state of each item, and as this signal from the container they
// belong to — and an assistive technology following a combo box or a page tab list reads the container's signal alone,
// so leaving it out has the user hear nothing at all as the selection moves.
//
// These go out after everything else the publish has to say, so that the items have already said what happened to them
// by the time a client is sent back to the container to ask what is selected now.
func (a *Adapter) emitSelectionChanges(pub *publication) {
	for _, container := range pub.selection {
		a.emit(NodePath(container), InterfaceEventObject, signalSelectionChanged, "", 0, 0, variantInt32(0))
	}
}

// emitIgnoredChanged announces a node whose Ignored flag has flipped. [accessibility.Diff] reports that as nothing but
// a state change, since an ignored node stays in the tree so that hit testing and coordinate clipping go on working,
// but to an assistive technology an ignored node has no object at all: the flip is exactly a node joining or leaving
// the reported hierarchy, and it reaches one in normal use, as a scroll bar appears and disappears or a group is given
// a name. Reporting nothing leaves the client's cached hierarchy permanently at odds with what GetChildren answers.
//
// The node's own reported children move with it, from the nearest reported ancestor onto the node or the other way
// about, and both ends of every move are announced: the ancestor that used to report a child is told it has gone before
// the object that holds it now is told it has arrived. Leaving the first half out would have a client that keeps a
// child list per object — which libatspi does — hold the same children in two places for as long as the window lives.
// Nothing deeper has to be said: only the direct children change parents.
//
// The node itself is announced before any of its children, and a child this publish has already announced is not
// announced again. A node that stops being ignored in the same publish in which it gains a brand-new child produces
// both a NodeAdded for that child and this event, and the child's arrival can only be announced on an object the client
// has already been told about; see [Adapter.emitNodeAdded], which holds such an addition back until it can be made.
func (a *Adapter) emitIgnoredChanged(pub *publication, ev *accessibility.Event) {
	if ev.New == trueValue {
		// The node has left the reported hierarchy, so its children are placed under the ancestor that holds them now
		// after it has gone, rather than being left pointing at an object the client has just been told to forget.
		a.emitNodeRemoved(pub, ev.Node)
		for _, id := range pub.prior.unignoredChildren(ev.Node) {
			a.emitNodeAdded(pub, id)
		}
		return
	}
	// The node has joined the reported hierarchy, and the children that were standing in for it are now its own. They
	// are taken away from the ancestor that used to report them first, so that the node itself lands among what is left
	// where the new snapshot puts it. Nothing is said to an ancestor the client no longer has, which is one that became
	// ignored itself earlier in this publish: it has already been told to forget the whole of what was under it.
	children := pub.data.unignoredChildren(ev.Node)
	if parent := pub.prior.parent(ev.Node); parent != 0 && pub.knows(parent) {
		for _, id := range children {
			if reportedNode(pub.prior, id) != nil {
				a.emitChildRemoved(pub, parent, id)
			}
		}
	}
	a.emitNodeAdded(pub, ev.Node)
	for _, id := range children {
		a.emitNodeAdded(pub, id)
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
//
// The node the event names has to be looked at as well as the flag that changed, in both the snapshot it is in now and
// the one before it, because several of the states [States] reports are derived from more of the node than the flag:
// whether a control can be edited at all, whether its pressed-ness is also its check, and which side of the
// expanded/collapsed pair it is on. A state a node's state set never holds must never be retracted, and one the state
// set does hold must never be left behind for a client to cache forever. prior is nil when the node was not reported
// before this publish.
func stateChanges(prior, n *accessibility.Node, ev *accessibility.Event) []stateChange {
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
		if !pressedIsChecked(n) {
			// [roleStates] only ever puts PRESSED on the two roles whose pressed-ness is their check, while a plain
			// push button carries the flag too — it is set for as long as the mouse is held down on one — so announcing
			// it for anything else would move a state the object's own GetState never reports.
			return nil
		}
		// AT-SPI reports a toggle that is down as a control that is checked, which is where [roleStates] gets CHECKED
		// from for these two roles, so leaving it out would have a client hold a CHECKED nothing ever retracts.
		return []stateChange{{name: stateNamePressed, on: on}, {name: stateNameChecked, on: on}}
	case accessibility.StateReadOnly:
		// A control whose value can no longer be changed has gained READ_ONLY and lost EDITABLE — but only if its
		// state set ever holds EDITABLE, which only the text-bearing roles that actually carry text do. Telling a
		// slider or a button that it has lost a state it never had would have a client retract something it was
		// never given, and hand it back again on the way out.
		changes := []stateChange{{name: stateNameReadOnly, on: on}}
		if editableText(n) {
			changes = append(changes, stateChange{name: stateNameEditable, on: !on})
		}
		return changes
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
		// [States] gives an expandable node exactly one of EXPANDED and COLLAPSED, and a node that cannot be expanded
		// neither of them, so the side it is on arrives and departs with the ability itself. Which side that is comes
		// from whichever snapshot had the ability, since only that one reported either state; the other one is left
		// alone rather than retracted, having never been sent.
		side := n
		if !on {
			side = prior
		}
		changes := []stateChange{{name: stateNameExpandable, on: on}}
		switch {
		case side == nil:
		case side.Expanded:
			changes = append(changes, stateChange{name: stateNameExpanded, on: on})
		default:
			changes = append(changes, stateChange{name: stateNameCollapsed, on: on})
		}
		return changes
	case accessibility.StateExpanded:
		if !n.Expandable {
			// [States] gives neither side of the pair to a node that cannot be expanded, so there is nothing for one to
			// have moved: a row that stops being expandable while it is open has already had both the ability and the
			// side it was on retracted by the StateExpandable branch, and saying anything more here would leave the
			// client holding COLLAPSED on an object whose state set has neither.
			return nil
		}
		// AT-SPI has a state for each side of this one, so expanding a node has to retract COLLAPSED as well, exactly
		// as becoming read-only retracts EDITABLE.
		return []stateChange{{name: stateNameExpanded, on: on}, {name: stateNameCollapsed, on: !on}}
	case accessibility.StateChecked:
		return checkedStateChanges(prior, n, ev)
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

// checkedStateChanges returns the states a change to a node's check is reported as. [accessibility.Diff] sends one
// event for both halves of a check — whether the node carries one at all, and which way it is set if it does — so both
// are worked out here. CHECKED is the on state and INDETERMINATE is the mixed one, so moving between them changes both,
// while a node that has only gained or lost its check changes nothing but CHECKABLE.
//
// Whether the node is checkable is read from the two snapshots rather than from the event, since the event's values are
// the check state alone and a node that cannot be checked reports neither CHECKED nor INDETERMINATE however its Checked
// field is left. That is also why gaining or losing the check carries whichever of the two the node holds with it, and
// why each side is clamped whether or not the checkability moved: a ProvideAccessibility that fills in Checked without
// setting HasCheck has [accessibility.Diff] report a check change on a node whose state set never held CHECKED at all,
// and nothing would ever retract the state this function would otherwise invent for it.
func checkedStateChanges(prior, n *accessibility.Node, ev *accessibility.Event) []stateChange {
	var changes []stateChange
	was := check.Extract(ev.Old)
	now := check.Extract(ev.New)
	wasCheckable, isCheckable := checkable(prior), checkable(n)
	if wasCheckable != isCheckable {
		changes = append(changes, stateChange{name: stateNameCheckable, on: isCheckable})
	}
	// A node that cannot be checked reports neither of the other two, so each side of the change is read as the check
	// the object's own state set would have shown at that moment.
	if !wasCheckable {
		was = check.Off
	}
	if !isCheckable {
		now = check.Off
	}
	if was == now {
		// Nothing a client was ever told about moved: a node as checkable as it was and set the way it was has no
		// state to change, and saying otherwise would have an assistive technology announce what did not happen.
		return changes
	}
	changes = append(changes, stateChange{name: stateNameChecked, on: now == check.On})
	if was == check.Mixed || now == check.Mixed {
		changes = append(changes, stateChange{name: stateNameIndeterminate, on: now == check.Mixed})
	}
	return changes
}

// emitTextChanged announces that runes have been inserted into or deleted from a node's text. The offsets are rune
// indexes, which is what AT-SPI means by characters.
func (a *Adapter) emitTextChanged(pub *publication, ev *accessibility.Event, detail, text string) {
	if reportedNode(pub.data, ev.Node) == nil {
		return
	}
	a.emitOwnChange(pub, ev.Node, InterfaceEventObject, signalTextChanged, detail, int32(ev.Start), int32(ev.Length),
		variantString(text))
}

// emitTextSelectionChanged announces that the caret has moved, and, when the selection is or was a range rather than
// just a caret, that the selection has changed as well. An assistive technology reads the selection back from the
// object, so the second signal carries nothing.
//
// Where the caret is comes from the node rather than from the event, since either end of a selection may be the one it
// sits at: extending a selection backwards with shift+Left leaves it at the start, and reporting the end instead would
// send an assistive technology's review cursor to the wrong end of what it has just announced.
//
// Clearing a selection is announced as well as making one. Orca keeps the last range it was told about and only
// replaces it when this signal arrives, so a collapse that says nothing leaves it reading a selection the control no
// longer has.
func (a *Adapter) emitTextSelectionChanged(pub *publication, ev *accessibility.Event) {
	n := reportedNode(pub.data, ev.Node)
	if n == nil {
		return
	}
	caret := ev.Start + ev.Length
	if n.Text != nil {
		caret = n.Text.Caret
	}
	a.emit(NodePath(ev.Node), InterfaceEventObject, signalTextCaretMoved, "", int32(caret), 0, variantInt32(0))
	if ev.Length != 0 || hadSelection(reportedNode(pub.prior, ev.Node)) {
		a.emit(NodePath(ev.Node), InterfaceEventObject, signalTextSelectionChanged, "", 0, 0, variantString(""))
	}
}

// hadSelection reports whether a node held a range of text rather than a bare caret, which is what makes collapsing
// its selection worth announcing.
func hadSelection(n *accessibility.Node) bool {
	return n != nil && n.Text != nil && n.Text.SelStart != n.Text.SelEnd
}

// emitNodeAdded announces a node that has joined a published window, along with any node whose own arrival was waiting
// for this one.
//
// A node can only be announced on an object the client already has, so an addition to a parent that has not been
// announced yet waits for it: an ignored node that stops being ignored in the same publish in which it gains a child
// produces the child's NodeAdded first, and announcing it then would put the child into a list the client does not have
// and leave the same child announced twice once the parent arrived with its children. Waiting is what makes the order
// right however the events happen to arrive.
func (a *Adapter) emitNodeAdded(pub *publication, id accessibility.NodeID) {
	announced, waiting := a.announceAdd(pub, id)
	if waiting {
		pub.wait(id)
		return
	}
	if announced {
		a.flushWaiting(pub)
	}
}

// flushWaiting announces the nodes whose arrival had to wait for the object that holds them, over and over while any of
// them becomes announceable, since a node that has just arrived may itself be what another was waiting for. Whatever is
// still waiting when nothing more can be said has no object to be announced on at all, and is left unsaid rather than
// announced into nowhere.
func (a *Adapter) flushWaiting(pub *publication) {
	for progress := true; progress && len(pub.waiting) != 0; {
		progress = false
		pending := pub.waiting
		pub.waiting = nil
		for _, id := range pending {
			switch announced, waiting := a.announceAdd(pub, id); {
			case announced:
				progress = true
			case waiting:
				pub.wait(id)
			default:
			}
		}
	}
}

// announceAdd sends the signals that say a node has joined the window: the cache item first, so that an assistive
// technology already knows what the node is when it is told where it went, then the parent's children-changed, and then
// the node's new parent when it had another one before.
//
// announced is false when there is nothing to say, either because the node has no object, because its arrival has
// already been announced, or because it is the window's own root, which joins the tree as a window rather than as a
// child of anything inside one. waiting is true when it cannot be said yet, which is the case while the object that is
// to hold the node has not been announced itself.
func (a *Adapter) announceAdd(pub *publication, id accessibility.NodeID) (announced, waiting bool) {
	data := pub.data
	n := reportedNode(data, id)
	if n == nil || pub.added[id] {
		return false, false
	}
	parent := data.parent(id)
	if parent == 0 {
		return false, false
	}
	if !pub.knows(parent) {
		return false, true
	}
	// A node that was somewhere else is taken away from there first, so that a client that keeps a child list per
	// object is never holding it in two places. Nothing is said to an object that has gone: a node that stops being
	// ignored takes its children with it, and the client has already been told to forget the whole of it.
	if was := pub.prior.parent(id); was != 0 && was != parent && pub.knows(was) && pub.holds(was, id) {
		a.emitChildRemoved(pub, was, id)
	}
	if pub.added == nil {
		pub.added = make(map[accessibility.NodeID]bool)
	}
	pub.added[id] = true
	// The cache item carries where the node sits in the snapshot, which is where it will be once everything this
	// publish has to say has been said, while the signal carries where it goes in the list the client is holding now.
	a.emitCacheAdd((&nodeObject{a: a, data: data, node: n}).cacheItem(data.indexInParent(id)))
	a.emitChildAdded(pub, parent, id)
	pub.notePresent(id, true)
	if prior := reportedNode(pub.prior, id); prior != nil && pub.prior.parent(id) != parent {
		a.emitPropertyChange(pub, id, propertyAccessibleParent, variantRef(a.reference(parent)))
	}
	// A node that was reported somewhere else keeps the children it already had, and they have to be moved onto it as
	// well, since nothing else in the publish says where they went.
	for _, child := range data.unignoredChildren(id) {
		if reportedNode(pub.prior, child) != nil && pub.prior.parent(child) != id {
			pub.wait(child)
		}
	}
	return true, false
}

// emitNodeRemoved announces a node that has left a published window. Everything it says has to come from the snapshot
// the node was last in, since the new one no longer holds it: where it was, and which node it was under.
//
// The cache is only told that the object is gone when it really is. A node that has been reparented into another window
// that is still published goes on answering for that window, and a client told that a live object has gone would drop
// it with nothing to ever put it back, since the window that holds it now sees no change of its own.
//
// Nothing at all is said to a parent the client no longer has, as [Adapter.announceAdd] says nothing to one either.
// [accessibility.Diff] reports the nodes that have left in ascending id order, and a container's id is lower than its
// children's, so closing a list with two rows announces the list's departure and then its rows': a children-changed
// sent from the list at that point would have libatspi resolve the event's source with ref_accessible, which builds a
// fresh, parentless object for the path it has just been told to drop and keeps it in the application's cache forever,
// since node ids are never reused.
func (a *Adapter) emitNodeRemoved(pub *publication, id accessibility.NodeID) {
	prior := pub.prior
	if reportedNode(prior, id) == nil {
		return
	}
	if parent := prior.parent(id); parent != 0 && pub.knows(parent) {
		a.emitChildRemoved(pub, parent, id)
	}
	pub.notePresent(id, false)
	if a.publishedElsewhere(id, pub.ws) {
		return
	}
	a.emitCacheRemove(a.reference(id))
}

// emitChildRemoved announces that a node is no longer among a parent's children, without saying anything about the node
// itself, which may well have gone somewhere else rather than away. The index is where the child sits in the list the
// client is holding now rather than where it sat in either snapshot; see [publication.children]. Nothing is sent for a
// child the client does not hold there, which is one an earlier signal of this publish has already taken away.
func (a *Adapter) emitChildRemoved(pub *publication, parent, id accessibility.NodeID) {
	index, ok := pub.noteRemove(parent, id)
	if !ok {
		return
	}
	a.emit(NodePath(parent), InterfaceEventObject, signalChildrenChanged, detailRemove, int32(index), 0,
		variantRef(a.reference(id)))
}

// emitChildAdded announces that a node is among a parent's children, without saying anything about the node itself,
// which may well have come from somewhere else rather than have just arrived. The index is where the child goes in the
// list the client is holding now rather than where it sits in either snapshot; see [publication.children].
func (a *Adapter) emitChildAdded(pub *publication, parent, id accessibility.NodeID) {
	index := pub.noteAdd(parent, id)
	a.emit(NodePath(parent), InterfaceEventObject, signalChildrenChanged, detailAdd, int32(index), 0,
		variantRef(a.reference(id)))
}

// emitBoundsChanged announces that a window has moved or been resized. Only window roots report it: the nodes inside a
// window move whenever anything is scrolled or relaid out, and an assistive technology asks for the extents it needs
// rather than remembering them.
//
// pub is nil when the change comes from a geometry update rather than from a publish, which is a window that has been
// moved, has changed screens or has changed backing scale without its snapshot changing at all.
func (a *Adapter) emitBoundsChanged(pub *publication, data *windowData, id accessibility.NodeID) {
	n := reportedNode(data, id)
	if n == nil || id != data.tree.Root {
		return
	}
	x, y, w, h := data.extents(n, CoordScreen)
	a.emitOwnChange(pub, id, InterfaceEventObject, signalBoundsChanged, "", 0, 0, variantRect(x, y, w, h))
}

// emit queues one AT-SPI event signal. Every event has the same shape — the detail string, two integers whose meaning
// depends on the event, the value the event carries, and a dictionary of the sender's properties that this package
// always leaves empty — and is sent from the path of the object it concerns.
//
// Every event is queued whether or not anything is listening for its class. The registry keeps a list of the event
// classes clients have registered for, which org.a11y.atspi.Registry.GetRegisteredEvents hands over and the
// EventListenerRegistered and EventListenerDeregistered signals keep up to date, and at-spi2-atk consults it precisely
// to avoid writing what nobody wants — a window that relays out or scrolls costs one bus write per change here.
// Consulting it is deliberately not done: the list is an optimization hint rather than a part of the protocol, since
// libatspi adds match rules of its own and a client may listen without ever registering, so a bridge that trusts it
// drops events that someone was waiting for, and the one thing worse than a wasted write is a screen reader that goes
// quiet. The write costs a queued message on a connection nothing else is using; being told nothing costs the user the
// application. If the traffic ever has to come down, the place to do it is the registration list plus those two
// signals, treating an empty list as "send everything" rather than as "send nothing".
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

// variantUint32 returns an unsigned integer as the value an event carries, which is what the enumerations AT-SPI passes
// by number, such as a role, are sent as.
func variantUint32(v uint32) dbus.Variant {
	return dbus.Variant{Sig: "u", Value: v}
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
