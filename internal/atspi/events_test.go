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
	"bufio"
	"net"
	"slices"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// toggleWindow is the window the state tests that need a node whose role decides its states publish.
const toggleWindow WindowKey = 6

// mainWindowSignals is how many signals the first publish of the main window sends: one cache item for each of the
// eight reported nodes, the window's Create, the application root gaining a child, and then the four signals that say
// the window is active and where its focus is.
const mainWindowSignals = 14

// falseValue is what a boolean state change that has been turned off carries, which is what strconv.FormatBool writes.
const falseValue = "false"

// groupName is what the ignored group is called in the publishes that make it a reported object, since a group with a
// name is a group worth announcing.
const groupName = "Identity"

// The text the field holds before and after a character is typed into it.
const (
	textBefore = "Will"
	textAfter  = "Willa"
)

// changedFieldValue is what the field's textual value is set to, replacing the testFieldValue it starts with.
const changedFieldValue = "Barney"

// The methods the far end of a stalled connection answers before it stops reading.
const (
	helloMember = "Hello"
	embedMember = "Embed"
)

// mainWindowNodes are the reported nodes of the main window, in the order a walk of the tree reaches them, which is the
// order their cache items are sent in. The ignored group is not among them.
var mainWindowNodes = []accessibility.NodeID{1, 3, 4, 5, 6, 7, 8, 9}

// signalRecord is one signal the fake peer saw, reduced to the parts that identify it.
type signalRecord struct {
	path   dbus.ObjectPath
	iface  string
	member string
	args   []any
}

// nextSignal returns the next signal the connection under test sent.
func (p *testPeer) nextSignal() signalRecord {
	p.t.Helper()
	select {
	case msg := <-p.signals:
		args, err := msg.Args()
		p.c.NoError(err)
		return signalRecord{path: msg.Path, iface: msg.Interface, member: msg.Member, args: args}
	case <-time.After(testTimeout):
		p.t.Fatal("timed out waiting for a signal")
		return signalRecord{}
	}
}

// nextSignals returns the next count signals the connection under test sent, in order.
func (p *testPeer) nextSignals(count int) []signalRecord {
	p.t.Helper()
	records := make([]signalRecord, 0, count)
	for range count {
		records = append(records, p.nextSignal())
	}
	return records
}

// newEventAdapter starts an adapter against a fake registry and takes the signals that announced the main window out of
// the way, so that what follows is only what the test is about.
func newEventAdapter(t *testing.T) *testAdapter {
	t.Helper()
	ta := newTestAdapter(t)
	ta.peer.nextSignals(mainWindowSignals)
	return ta
}

// eventBody is the body of every AT-SPI event: the detail string, the two integers whose meaning depends on the event,
// the value the event carries, and the sender's properties, which this package always leaves empty.
func eventBody(detail string, detail1, detail2 int32, data dbus.Variant) []any {
	return []any{detail, detail1, detail2, data, dbus.Dict{}}
}

// objectEvent is the signal a test expects on the object of one node.
func objectEvent(id accessibility.NodeID, member, detail string, detail1, detail2 int32,
	data dbus.Variant,
) signalRecord {
	return signalRecord{
		path:   NodePath(id),
		iface:  InterfaceEventObject,
		member: member,
		args:   eventBody(detail, detail1, detail2, data),
	}
}

// stateEvent is the signal a test expects when one of a node's states changes.
func stateEvent(id accessibility.NodeID, state string, on bool) signalRecord {
	var detail1 int32
	if on {
		detail1 = 1
	}
	return objectEvent(id, signalStateChanged, state, detail1, 0, variantInt32(0))
}

// windowEvent is the signal a test expects about a window's lifetime.
func windowEvent(id accessibility.NodeID, member, name string) signalRecord {
	return signalRecord{
		path:   NodePath(id),
		iface:  InterfaceEventWindow,
		member: member,
		args:   eventBody("", 0, 0, variantString(name)),
	}
}

// focusEvent is the legacy signal a test expects when a node takes the keyboard focus.
func focusEvent(id accessibility.NodeID) signalRecord {
	return signalRecord{
		path:   NodePath(id),
		iface:  InterfaceEventFocus,
		member: signalFocus,
		args:   eventBody("", 0, 0, variantInt32(0)),
	}
}

// rootChildrenEvent is the signal a test expects when a window joins or leaves the application root.
func rootChildrenEvent(detail string, index int32, id accessibility.NodeID) signalRecord {
	return signalRecord{
		path:   RootPath,
		iface:  InterfaceEventObject,
		member: signalChildrenChanged,
		args:   eventBody(detail, index, 0, variantRef(nodeRef(id))),
	}
}

// cacheRemoval is the signal a test expects when a node leaves the hierarchy.
func cacheRemoval(id accessibility.NodeID) signalRecord {
	return signalRecord{
		path:   CachePath,
		iface:  InterfaceCache,
		member: signalRemoveAccessible,
		args:   []any{nodeRef(id)},
	}
}

// cachedID returns the id of the node a cache item describes, failing the test if the signal is not a cache addition.
func (ta *testAdapter) cachedID(record signalRecord) dbus.ObjectRef {
	ta.c.Equal(CachePath, record.path)
	ta.c.Equal(InterfaceCache, record.iface)
	ta.c.Equal(signalAddAccessible, record.member)
	ta.c.Equal(1, len(record.args))
	item, ok := record.args[0].(dbus.Struct)
	ta.c.True(ok, "a cache item is a structure")
	ta.c.Equal(10, len(item), "a cache item has ten fields")
	ref, ok := item[0].(dbus.ObjectRef)
	ta.c.True(ok, "a cache item starts with the object it describes")
	return ref
}

// childLists is what an assistive technology keeps as it listens: libatspi holds an array of children per object and
// applies every children-changed to it, so replaying what a publish announced against the lists the client had before
// it has to produce the lists the objects themselves now report. Checking the objects alone would pass while the
// announcement was wrong, which is the only thing a client that never asks again would ever see.
type childLists map[dbus.ObjectPath][]dbus.ObjectRef

// apply replays the children-changed events among a run of signals, leaving every other signal alone. Each one has to
// name the child that really is at the index it carries, since the index is all a client has to go on.
func (lists childLists) apply(c check.Checker, records []signalRecord) {
	for _, record := range records {
		if record.iface != InterfaceEventObject || record.member != signalChildrenChanged {
			continue
		}
		detail, ok := record.args[0].(string)
		c.True(ok, "a detail string is a string")
		index, ok := record.args[1].(int32)
		c.True(ok, "the first integer of an event is an integer")
		value, ok := record.args[3].(dbus.Variant)
		c.True(ok, "the value of an event is a variant")
		ref, ok := value.Value.(dbus.ObjectRef)
		c.True(ok, "children-changed carries a reference to the child")
		list := lists[record.path]
		switch detail {
		case detailAdd:
			if int(index) > len(list) {
				c.Fatalf("%s cannot gain a child at %d; it has %d", record.path, index, len(list))
			}
			lists[record.path] = slices.Insert(list, int(index), ref)
		case detailRemove:
			if int(index) >= len(list) {
				c.Fatalf("%s cannot lose the child at %d; it has %d", record.path, index, len(list))
			}
			c.Equal(ref, list[index], "the child leaving %s must be the one at the index announced", record.path)
			lists[record.path] = slices.Delete(list, int(index), int(index)+1)
		default:
			c.Fatalf("children-changed carried the unknown detail %q", detail)
		}
	}
}

// listsOf returns the child lists a client would be holding if it had been told about every object of a tree, which is
// where the replay of a publish starts from and what it has to arrive at.
func listsOf(t *accessibility.Tree) childLists {
	lists := make(childLists)
	t.Walk(func(n *accessibility.Node) bool {
		if n.Ignored {
			return true
		}
		children := t.UnignoredChildren(n.ID)
		if len(children) == 0 {
			return true
		}
		refs := make([]dbus.ObjectRef, 0, len(children))
		for _, id := range children {
			refs = append(refs, nodeRef(id))
		}
		lists[NodePath(n.ID)] = refs
		return true
	})
	return lists
}

// matches fails the test unless the lists a client is holding are the ones every object of the tree now reports. What
// it holds for an object that has left the tree is passed over: a client drops those along with the objects.
func (lists childLists) matches(c check.Checker, t *accessibility.Tree) {
	for path, expected := range listsOf(t) {
		c.Equal(expected, lists[path], "the children a client holds for %s", path)
	}
	t.Walk(func(n *accessibility.Node) bool {
		if !n.Ignored && len(t.UnignoredChildren(n.ID)) == 0 {
			c.Equal(0, len(lists[NodePath(n.ID)]), "%s has nothing left in it", NodePath(n.ID))
		}
		return true
	})
}

// replay applies what a publish announced to the lists a client held before it and checks that the result is what the
// new snapshot says. It is the whole of what a caching client ever sees: the objects would answer correctly however
// wrong the announcement was, and a client that never asks again would live with the difference forever.
func replay(c check.Checker, prior *accessibility.Tree, signals []signalRecord, now *accessibility.Tree) {
	lists := listsOf(prior)
	lists.apply(c, signals)
	lists.matches(c, now)
}

// activeMainTree returns the main window's tree with the given changes applied, keeping it the active window.
func activeMainTree(apply func(t *accessibility.Tree)) *accessibility.Tree {
	tree := mainTree()
	tree.Generation++
	apply(tree)
	return tree
}

// checkableRowTree returns the main window's tree with a check on the first row of the list. Only a node that carries
// one has CHECKED and INDETERMINATE in its state set at all.
func checkableRowTree() *accessibility.Tree {
	return activeMainTree(func(tree *accessibility.Tree) { tree.Node(6).HasCheck = true })
}

func TestFirstPublishAnnouncesTheWholeWindow(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	signals := ta.peer.nextSignals(mainWindowSignals)

	// One cache item per reported node, in the order a walk of the tree reaches them. They come before anything points
	// at the window, so that an assistive technology already knows what it is being shown.
	for i, id := range mainWindowNodes {
		c.Equal(nodeRef(id), ta.cachedID(signals[i]), "cache item %d", i)
	}
	c.Equal([]any{dbus.Struct{
		nodeRef(1), rootRef(), rootRef(), int32(0), int32(5),
		[]string{InterfaceAccessible, InterfaceComponent},
		"Test Window", uint32(RoleFrame), "",
		States(mainTree().Node(1), true, true).Words(),
	}}, signals[0].args, "the window's cache item says everything the cache would")

	// The window itself, then its place among the application's children.
	c.Equal(windowEvent(1, signalCreate, "Test Window"), signals[8])
	c.Equal(rootChildrenEvent(detailAdd, 0, 1), signals[9])

	// The window is the active one, so it says so, and says where its focus is.
	c.Equal(windowEvent(1, signalActivate, "Test Window"), signals[10])
	c.Equal(stateEvent(1, stateNameActive, true), signals[11])
	c.Equal(stateEvent(4, stateNameFocused, true), signals[12])
	c.Equal(focusEvent(4), signals[13])
}

func TestPublishingAnUnchangedTreeSaysNothing(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ta.Publish(mainWindow, mainTree(), nil, sampleGeometry())
	ta.Publish(mainWindow, mainTree(), accessibility.Diff(mainTree(), mainTree()), sampleGeometry())

	// Signals are queued in order, so an announcement arriving next is proof that the two publishes sent nothing.
	ta.Announce("Nothing happened")
	c.Equal(signalRecord{
		path:   RootPath,
		iface:  InterfaceEventObject,
		member: signalAnnouncement,
		args:   eventBody("", livePolite, 0, variantString("Nothing happened")),
	}, ta.peer.nextSignal())
}

func TestFocusMovingWithinAWindow(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	old := mainTree()
	moved := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Focused = false
		tree.Node(8).Focused = true
		tree.Focus = 8
	})
	events := accessibility.Diff(old, moved)
	c.Equal([]accessibility.Event{{Kind: accessibility.FocusChanged, Node: 8}}, events)
	ta.Publish(mainWindow, moved, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(4, stateNameFocused, false),
		stateEvent(8, stateNameFocused, true),
		focusEvent(8),
	}, ta.peer.nextSignals(3))
}

func TestFocusInAWindowThatIsNotActiveSaysNothing(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	// AT-SPI's focused state belongs to the active window alone, so a focus move anywhere else is not reported.
	inactive := mainTree()
	inactive.Node(1).Focused = false
	ta.Publish(mainWindow, inactive, accessibility.Diff(mainTree(), inactive), sampleGeometry())
	ta.peer.nextSignals(3) // The window is no longer the active one, and its focus is no longer the focus

	moved := mainTree()
	moved.Node(1).Focused = false
	moved.Node(4).Focused = false
	moved.Node(8).Focused = true
	moved.Focus = 8
	ta.Publish(mainWindow, moved, []accessibility.Event{{Kind: accessibility.FocusChanged, Node: 8}}, sampleGeometry())

	ta.Announce("Still nothing")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

func TestWindowActivationAndDeactivation(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	// A second window, which is not the active one and whose list row holds the focus within it.
	inactive := otherTree()
	inactive.Node(22).Focused = true
	inactive.Focus = 22
	geometry := Geometry{Scale: geom.NewPoint(1, 1)}
	ta.Publish(otherWindow, inactive, nil, geometry)
	signals := ta.peer.nextSignals(5)
	for i, id := range []accessibility.NodeID{20, 21, 22} {
		c.Equal(nodeRef(id), ta.cachedID(signals[i]), "cache item %d", i)
	}
	c.Equal(windowEvent(20, signalCreate, "Pick One"), signals[3])
	c.Equal(rootChildrenEvent(detailAdd, 1, 20), signals[4])

	activated := otherTree()
	activated.Node(20).Focused = true
	activated.Node(22).Focused = true
	activated.Focus = 22
	events := accessibility.Diff(inactive, activated)
	c.Equal([]accessibility.Event{{Kind: accessibility.WindowActivated, Node: 20}}, events)
	ta.Publish(otherWindow, activated, events, geometry)
	c.Equal([]signalRecord{
		windowEvent(20, signalActivate, "Pick One"),
		stateEvent(20, stateNameActive, true),
		stateEvent(22, stateNameFocused, true),
		focusEvent(22),
	}, ta.peer.nextSignals(4))

	events = accessibility.Diff(activated, inactive)
	c.Equal([]accessibility.Event{{Kind: accessibility.WindowDeactivated, Node: 20}}, events)
	ta.Publish(otherWindow, inactive, events, geometry)
	c.Equal([]signalRecord{
		windowEvent(20, signalDeactivate, "Pick One"),
		stateEvent(20, stateNameActive, false),
		stateEvent(22, stateNameFocused, false),
	}, ta.peer.nextSignals(3), "nothing takes the focus when a window stops being the active one")
}

func TestRemovingAWindowSaysSo(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ta.RemoveWindow(mainWindow)
	signals := ta.peer.nextSignals(2 + len(mainWindowNodes))
	c.Equal(windowEvent(1, signalDestroy, "Test Window"), signals[0])
	c.Equal(rootChildrenEvent(detailRemove, 0, 1), signals[1])
	for i, id := range mainWindowNodes {
		c.Equal(cacheRemoval(id), signals[2+i], "cache removal %d", i)
	}
	ta.RemoveWindow(mainWindow) // A window that is not there says nothing
	ta.Announce("Gone")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestPublishingAWindowAgainAfterRemovingIt covers a window that is hidden and shown again, which the root package
// reports by taking it out of the accessibility tree and publishing it afresh. The second publish has to announce the
// whole window, since nothing has been told anything about it since it went.
func TestPublishingAWindowAgainAfterRemovingIt(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ta.RemoveWindow(mainWindow)
	ta.peer.nextSignals(2 + len(mainWindowNodes))
	c.Equal(int32(0), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"))

	renamed := activeMainTree(func(tree *accessibility.Tree) { tree.Node(1).Name = "Test Window Again" })
	// The events are the ones the window's own snapshots produce, and they are beside the point: to this adapter the
	// window is one it has never heard of, so what it says is what it says about any new window.
	ta.Publish(mainWindow, renamed, accessibility.Diff(mainTree(), renamed), sampleGeometry())
	signals := ta.peer.nextSignals(mainWindowSignals)
	for i, id := range mainWindowNodes {
		c.Equal(nodeRef(id), ta.cachedID(signals[i]), "cache item %d", i)
	}
	c.Equal(windowEvent(1, signalCreate, "Test Window Again"), signals[8])
	c.Equal(rootChildrenEvent(detailAdd, 0, 1), signals[9])
	c.Equal(windowEvent(1, signalActivate, "Test Window Again"), signals[10])
	c.Equal(stateEvent(1, stateNameActive, true), signals[11])
	c.Equal(stateEvent(4, stateNameFocused, true), signals[12])
	c.Equal(focusEvent(4), signals[13])
	c.Equal(int32(1), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"))
	c.Equal("Test Window Again", ta.peer.getProperty(NodePath(1), InterfaceAccessible, "Name"))
	c.Equal([]dbus.ObjectRef{nodeRef(1)}, ta.one(RootPath, InterfaceAccessible, "GetChildren", ""))
}

// TestANodeThatMovedToAnotherWindowIsNotBuried covers the cache signals of a panel that has been reparented from one
// window into another. The window it left says it is gone, which is true of that window's hierarchy, but the object
// itself goes on answering for the window that holds it now — and nothing would ever put it back into a client's cache,
// since that window sees no change of its own.
func TestANodeThatMovedToAnotherWindowIsNotBuried(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	geometry := Geometry{Scale: geom.NewPoint(1, 1)}
	ta.Publish(otherWindow, windowThatTookTheSlider(), nil, geometry)
	ta.peer.nextSignals(6) // A cache item for each of its four nodes, the window's Create, and its place

	// The window the slider came from publishes without it, which to that window is a removal.
	without := mainWindowWithoutTheSlider()
	ta.Publish(mainWindow, without, accessibility.Diff(mainTree(), without), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(1, signalChildrenChanged, detailRemove, 3, 0, variantRef(nodeRef(8))),
	}, ta.peer.nextSignals(1), "the window it left no longer lists it, and nothing says the object has gone")

	// Closing that window says nothing about it either.
	ta.RemoveWindow(mainWindow)
	signals := ta.peer.nextSignals(2 + len(mainWindowNodes) - 1)
	c.Equal(windowEvent(1, signalDestroy, "Test Window"), signals[0])
	c.Equal(rootChildrenEvent(detailRemove, 0, 1), signals[1])
	for _, record := range signals {
		c.False(record.path == CachePath && record.args[0] == nodeRef(8), "the slider must not be reported as gone")
	}
	c.Equal("Volume", ta.peer.getProperty(NodePath(8), InterfaceAccessible, "Name"),
		"and it goes on answering for the window that holds it")
	ta.Announce("Nothing more")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

func TestNodesComingAndGoing(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	changed := activeMainTree(func(tree *accessibility.Tree) {
		list := tree.Node(5)
		list.Children = []accessibility.NodeID{6, 10}
		delete(tree.Nodes, 7)
		tree.Nodes[10] = &accessibility.Node{
			ID: 10, Parent: 5, Role: role.ListItem, Name: "Three", Selectable: true,
			Bounds: geom.NewRect(0, 80, 200, 20),
		}
	})
	events := accessibility.Diff(mainTree(), changed)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.NodeRemoved, Node: 7},
		{Kind: accessibility.ChildrenChanged, Node: 5},
		{Kind: accessibility.NodeAdded, Node: 10},
	}, events)
	ta.Publish(mainWindow, changed, events, sampleGeometry())

	signals := ta.peer.nextSignals(4)
	// What has gone is reported from where it used to be, since the new snapshot no longer holds it.
	c.Equal(objectEvent(5, signalChildrenChanged, detailRemove, 1, 0, variantRef(nodeRef(7))), signals[0])
	c.Equal(cacheRemoval(7), signals[1])
	// What has arrived is described before it is placed.
	c.Equal(nodeRef(10), ta.cachedID(signals[2]))
	c.Equal(objectEvent(5, signalChildrenChanged, detailAdd, 1, 0, variantRef(nodeRef(10))), signals[3])
}

// TestSeveralChildrenOfOneParentLeavingAtOnce covers the numbering of children-changed:remove. A client applies each
// signal as it arrives, so the indexes of a run of removals from the same parent cannot all be read from the snapshot
// they came from: by the time the second is sent, the list it is numbered against is one shorter.
func TestSeveralChildrenOfOneParentLeavingAtOnce(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	// The list gains a third row, so that there are three to take two away from.
	three := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Children = []accessibility.NodeID{6, 7, 10}
		tree.Nodes[10] = &accessibility.Node{
			ID: 10, Parent: 5, Role: role.ListItem, Name: "Three", Selectable: true,
			Bounds: geom.NewRect(0, 100, 200, 20),
		}
	})
	ta.Publish(mainWindow, three, accessibility.Diff(mainTree(), three), sampleGeometry())
	ta.peer.nextSignals(2) // The cache item for the new row and where it went

	one := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Children = []accessibility.NodeID{6}
		delete(tree.Nodes, 7)
	})
	events := accessibility.Diff(three, one)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.NodeRemoved, Node: 7},
		{Kind: accessibility.NodeRemoved, Node: 10},
		{Kind: accessibility.ChildrenChanged, Node: 5},
	}, events)
	ta.Publish(mainWindow, one, events, sampleGeometry())
	// The second row to go was at index 2 of the list the snapshots hold, but the first removal has already taken
	// index 1 away, so index 1 is where it is by the time the client hears about it.
	c.Equal([]signalRecord{
		objectEvent(5, signalChildrenChanged, detailRemove, 1, 0, variantRef(nodeRef(7))),
		cacheRemoval(7),
		objectEvent(5, signalChildrenChanged, detailRemove, 1, 0, variantRef(nodeRef(10))),
		cacheRemoval(10),
	}, ta.peer.nextSignals(4))
}

// TestANodeThatStopsBeingIgnored covers a node joining the reported hierarchy without being added to the tree, which is
// what a scroll bar does when there is finally something to scroll and what a group does when it is given a name.
// accessibility.Diff calls that a state change, since the node was in the tree all along, but an ignored node has no
// object at all, so to an assistive technology it is an addition.
func TestANodeThatStopsBeingIgnored(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	named := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(2).Ignored = false
		tree.Node(2).Name = groupName
	})
	events := accessibility.Diff(mainTree(), named)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 2, New: groupName},
		{Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateIgnored, Old: trueValue, New: falseValue},
	}, events, "the group is still in the tree, so nothing was added or removed there")
	ta.Publish(mainWindow, named, events, sampleGeometry())

	signals := ta.peer.nextSignals(11)
	// The two nodes that were standing in for the group leave the window before anything is put in their place. The
	// second of them was at index 1 of the list the snapshots hold, but the first removal has already taken index 0
	// away by the time the client hears about it.
	c.Equal(objectEvent(1, signalChildrenChanged, detailRemove, 0, 0, variantRef(nodeRef(3))), signals[0])
	c.Equal(objectEvent(1, signalChildrenChanged, detailRemove, 0, 0, variantRef(nodeRef(4))), signals[1])
	// The group then joins the window's children, and the two nodes become its own, each of them told whose child it is
	// now.
	c.Equal(nodeRef(2), ta.cachedID(signals[2]))
	c.Equal(objectEvent(1, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(2))), signals[3])
	c.Equal(nodeRef(3), ta.cachedID(signals[4]))
	c.Equal(objectEvent(2, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(3))), signals[5])
	c.Equal(objectEvent(3, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(2))), signals[6])
	c.Equal(nodeRef(4), ta.cachedID(signals[7]))
	c.Equal(objectEvent(2, signalChildrenChanged, detailAdd, 1, 0, variantRef(nodeRef(4))), signals[8])
	c.Equal(objectEvent(4, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(2))), signals[9])
	// The name change comes last, even though [accessibility.Diff] reports it first. It belongs to an object the client
	// had not been told about when it arrived, and an assistive technology should already know what an object is before
	// anything points at it or reports a change to it.
	c.Equal(objectEvent(2, signalPropertyChange, propertyAccessibleName, 0, 0, variantString(groupName)), signals[10])

	// Replaying what was announced against the lists a client held before the publish has to produce the lists the
	// objects report now. Checking the objects alone would pass even if the window had never been told that the two
	// nodes had left it, which is exactly the mistake a caching client would then live with forever.
	lists := childLists{NodePath(1): {nodeRef(3), nodeRef(4), nodeRef(5), nodeRef(8), nodeRef(9)}}
	lists.apply(c, signals)
	children := ta.one(NodePath(1), InterfaceAccessible, "GetChildren", "")
	c.Equal([]dbus.ObjectRef{nodeRef(2), nodeRef(5), nodeRef(8), nodeRef(9)}, children,
		"what was announced has to be what the window now reports")
	c.Equal(lists[NodePath(1)], children, "a client that applied the announcement has the window's children right")
	c.Equal(lists[NodePath(2)], ta.one(NodePath(2), InterfaceAccessible, "GetChildren", ""), "and the group's")
}

// TestANodeThatBecomesIgnored covers the other direction: a node leaving the reported hierarchy while staying in the
// tree, with its children left to the ancestor that holds them now.
func TestANodeThatBecomesIgnored(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	hidden := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Ignored = true
		tree.Node(5).Name = ""
	})
	events := accessibility.Diff(mainTree(), hidden)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 5, Old: "Items"},
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateIgnored, Old: falseValue, New: trueValue},
	}, events)
	ta.Publish(mainWindow, hidden, events, sampleGeometry())

	// The name change says nothing: a node with no object has no property for anything to have changed on. The list was
	// the window's third reported child, and its two rows take its place there.
	signals := ta.peer.nextSignals(8)
	c.Equal(objectEvent(1, signalChildrenChanged, detailRemove, 2, 0, variantRef(nodeRef(5))), signals[0])
	c.Equal(cacheRemoval(5), signals[1])
	c.Equal(nodeRef(6), ta.cachedID(signals[2]))
	c.Equal(objectEvent(1, signalChildrenChanged, detailAdd, 2, 0, variantRef(nodeRef(6))), signals[3])
	// Nothing is said to the object that held them: the client has just been told to forget the whole of it.
	c.Equal(objectEvent(6, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(1))), signals[4])
	c.Equal(nodeRef(7), ta.cachedID(signals[5]))
	c.Equal(objectEvent(1, signalChildrenChanged, detailAdd, 3, 0, variantRef(nodeRef(7))), signals[6])
	c.Equal(objectEvent(7, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(1))), signals[7])
	lists := childLists{NodePath(1): {nodeRef(3), nodeRef(4), nodeRef(5), nodeRef(8), nodeRef(9)}}
	lists.apply(c, signals)
	children := ta.one(NodePath(1), InterfaceAccessible, "GetChildren", "")
	c.Equal([]dbus.ObjectRef{nodeRef(3), nodeRef(4), nodeRef(6), nodeRef(7), nodeRef(8), nodeRef(9)}, children)
	c.Equal(lists[NodePath(1)], children, "a client that applied the announcement has the window's children right")
}

// TestAGroupAppearingWhileASiblingDisappears covers the numbering of a publish that both adds to and removes from the
// same parent. A client applies each children-changed as it arrives and has nothing to go on but the index it carries,
// so every index has to be an index into the list the client is holding at that moment — the one the signals sent so
// far have left it with — rather than into either snapshot. Numbering a removal against the old snapshot alone names
// the wrong child once anything has been added ahead of it, and a client that trusts the index deletes something else.
func TestAGroupAppearingWhileASiblingDisappears(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	swapped := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(2).Ignored = false
		tree.Node(2).Name = groupName
		tree.Node(5).Ignored = true
	})
	ta.Publish(mainWindow, swapped, accessibility.Diff(mainTree(), swapped), sampleGeometry())

	// The group takes the window's first place, so the list that is on its way out is no longer where either snapshot
	// has it by the time the client is told it has gone.
	signals := ta.peer.nextSignals(19)
	c.Equal(objectEvent(1, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(2))), signals[3])
	c.Equal(objectEvent(1, signalChildrenChanged, detailRemove, 1, 0, variantRef(nodeRef(5))), signals[10])
	c.Equal(cacheRemoval(5), signals[11])
	replay(c, mainTree(), signals, swapped)
	c.Equal([]dbus.ObjectRef{nodeRef(2), nodeRef(6), nodeRef(7), nodeRef(8), nodeRef(9)},
		ta.one(NodePath(1), InterfaceAccessible, "GetChildren", ""))
}

// TestANodeThatStopsBeingIgnoredWhileGainingAChild covers the other way two events can describe one arrival. The new
// child's NodeAdded comes first, naming an object the client has not been told about yet and whose list it does not
// have; announcing it then and again with the group's own children has the same child arrive twice, which is a
// contradiction a client cannot recover from.
func TestANodeThatStopsBeingIgnoredWhileGainingAChild(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	grown := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(2).Ignored = false
		tree.Node(2).Name = groupName
		tree.Node(2).Children = []accessibility.NodeID{3, 4, 10}
		tree.Nodes[10] = &accessibility.Node{
			ID: 10, Parent: 2, Role: role.Label, Name: "New", Bounds: geom.NewRect(10, 30, 40, 20),
		}
	})
	events := accessibility.Diff(mainTree(), grown)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 2},
		{Kind: accessibility.NodeAdded, Node: 10},
		{Kind: accessibility.NameChanged, Node: 2, New: groupName},
		{Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateIgnored, Old: trueValue, New: falseValue},
	}, events, "the new child is reported before anything says its parent has an object at all")
	ta.Publish(mainWindow, grown, events, sampleGeometry())

	signals := ta.peer.nextSignals(13)
	// The group is announced before any of its children, and the child that arrived with it is announced once.
	c.Equal(objectEvent(1, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(2))), signals[3])
	adds := 0
	for _, record := range signals {
		if record.path == NodePath(2) && record.member == signalChildrenChanged {
			adds++
			c.Equal(detailAdd, record.args[0])
		}
	}
	c.Equal(3, adds, "the group gains its two old children and the new one, each of them once")
	replay(c, mainTree(), signals, grown)
	c.Equal([]dbus.ObjectRef{nodeRef(3), nodeRef(4), nodeRef(10)},
		ta.one(NodePath(2), InterfaceAccessible, "GetChildren", ""))
}

// TestAContainerLeavingWithItsChildren covers the order [accessibility.Diff] reports removals in. It reports them in
// ascending id order, and a container's id is lower than its children's, so a list that goes away is announced before
// its rows are. Nothing may then be said from the list's own object: libatspi resolves an event's source with
// ref_accessible, which builds a fresh, parentless object for a path it has just been told to drop and keeps it in the
// application's cache forever, since node ids are never reused.
func TestAContainerLeavingWithItsChildren(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	gone := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(1).Children = []accessibility.NodeID{2, 8, 9}
		for _, id := range []accessibility.NodeID{5, 6, 7} {
			delete(tree.Nodes, id)
		}
	})
	events := accessibility.Diff(mainTree(), gone)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.NodeRemoved, Node: 5},
		{Kind: accessibility.NodeRemoved, Node: 6},
		{Kind: accessibility.NodeRemoved, Node: 7},
		{Kind: accessibility.ChildrenChanged, Node: 1},
	}, events, "the list is reported as gone before either of the rows inside it")
	ta.Publish(mainWindow, gone, events, sampleGeometry())

	// The window is told it has lost the list, and the cache is told about all three objects. The rows say nothing
	// about the list, which the client no longer has.
	signals := ta.peer.nextSignals(4)
	c.Equal([]signalRecord{
		objectEvent(1, signalChildrenChanged, detailRemove, 2, 0, variantRef(nodeRef(5))),
		cacheRemoval(5),
		cacheRemoval(6),
		cacheRemoval(7),
	}, signals)
	replay(c, mainTree(), signals, gone)
	ta.Announce("Nothing from an object that has gone")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// ignoredGroupTree is a window with two nested groups, the inner one ignored, which is the shape the pair of guards in
// [Adapter.emitIgnoredChanged] needs: a node whose reported parent can itself stop being reported in the same publish.
//
//	80 window "Nest"          (0,0 100x100)  active
//	└─ 81 group               (0,0 100x100)
//	   └─ 82 group [ignored]  (0,0 100x50)
//	      └─ 83 label "Deep"  (0,0 100x20)
func ignoredGroupTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 80, Role: role.Window, Name: "Nest", Focused: true, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []accessibility.NodeID{81},
		},
		&accessibility.Node{
			ID: 81, Parent: 80, Role: role.Group, Name: "Outer", Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []accessibility.NodeID{82},
		},
		&accessibility.Node{
			ID: 82, Parent: 81, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 100, 50),
			Children: []accessibility.NodeID{83},
		},
		&accessibility.Node{ID: 83, Parent: 82, Role: role.Label, Name: "Deep", Bounds: geom.NewRect(0, 0, 100, 20)},
	)
}

// TestAGroupJoiningUnderOneThatHasJustLeft covers the other half of the same guard. The inner group joins the reported
// hierarchy in the publish in which the outer one leaves it, so the children that were standing in for the inner group
// have to be taken off the object that reported them — unless that object is the outer group, which the client has
// already been told to forget.
func TestAGroupJoiningUnderOneThatHasJustLeft(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	const nestWindow WindowKey = 7
	nest := ignoredGroupTree()
	ta.Publish(nestWindow, nest, nil, sampleGeometry())
	ta.peer.nextSignals(7) // The three reported nodes, the window's own signals and where its focus is

	swapped := ignoredGroupTree()
	swapped.Generation++
	swapped.Node(81).Ignored = true
	swapped.Node(82).Ignored = false
	events := accessibility.Diff(nest, swapped)
	c.Equal([]accessibility.Event{
		{
			Kind: accessibility.StateChanged, Node: 81, State: accessibility.StateIgnored, Old: falseValue,
			New: trueValue,
		},
		{
			Kind: accessibility.StateChanged, Node: 82, State: accessibility.StateIgnored, Old: trueValue,
			New: falseValue,
		},
	}, events)
	ta.Publish(nestWindow, swapped, events, sampleGeometry())

	signals := ta.peer.nextSignals(7)
	c.Equal([]signalRecord{
		// The outer group leaves the window, taking its object with it.
		objectEvent(80, signalChildrenChanged, detailRemove, 0, 0, variantRef(nodeRef(81))),
		cacheRemoval(81),
		// The inner group takes its place, and the label that was standing in for it becomes its child again. Nothing
		// is said to the outer group, which has gone.
		signals[2],
		objectEvent(80, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(82))),
		signals[4],
		objectEvent(82, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(83))),
		objectEvent(83, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(82))),
	}, signals)
	c.Equal(nodeRef(82), ta.cachedID(signals[2]))
	c.Equal(nodeRef(83), ta.cachedID(signals[4]))
	replay(c, nest, signals, swapped)
	ta.Announce("Nothing from the group that left")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestANodeMovingBetweenParents covers a panel that is reparented within one window. It keeps its id, so
// [accessibility.Diff] has nothing to report but a pair of ChildrenChanged events — no node was added or removed — and
// a client that keeps a child list per object, which libatspi does, would go on holding it under its old parent and
// never under its new one for as long as the window lived.
func TestANodeMovingBetweenParents(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	moved := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(1).Children = []accessibility.NodeID{2, 5, 9}
		tree.Node(5).Children = []accessibility.NodeID{6, 7, 8}
		tree.Node(8).Parent = 5
	})
	events := accessibility.Diff(mainTree(), moved)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 1},
		{Kind: accessibility.ChildrenChanged, Node: 5},
	}, events, "nothing joined or left the window, so this is all there is to go on")
	ta.Publish(mainWindow, moved, events, sampleGeometry())

	signals := ta.peer.nextSignals(4)
	c.Equal(objectEvent(1, signalChildrenChanged, detailRemove, 3, 0, variantRef(nodeRef(8))), signals[0])
	c.Equal(nodeRef(8), ta.cachedID(signals[1]))
	c.Equal(objectEvent(5, signalChildrenChanged, detailAdd, 2, 0, variantRef(nodeRef(8))), signals[2])
	c.Equal(objectEvent(8, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(5))), signals[3])
	replay(c, mainTree(), signals, moved)
	c.Equal([]dbus.ObjectRef{nodeRef(3), nodeRef(4), nodeRef(5), nodeRef(9)},
		ta.one(NodePath(1), InterfaceAccessible, "GetChildren", ""))
	c.Equal([]dbus.ObjectRef{nodeRef(6), nodeRef(7), nodeRef(8)},
		ta.one(NodePath(5), InterfaceAccessible, "GetChildren", ""))
	c.Equal(nodeRef(5), ta.peer.getProperty(NodePath(8), InterfaceAccessible, "Parent"))

	// A node moved into a container that is ignored belongs to the nearest reported ancestor, whose own list of
	// children has not changed: nothing in the publish names it, so the parent that lost the node is where both halves
	// of the move have to be said.
	buried := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(2).Children = []accessibility.NodeID{3, 4, 7}
		tree.Node(5).Children = []accessibility.NodeID{6}
		tree.Node(7).Parent = 2
	})
	buried.Generation++
	// The window goes back to the shape it started in, without a word said about it, so that what follows is only what
	// the move into the ignored group produces.
	ta.Publish(mainWindow, mainTree(), nil, sampleGeometry())
	events = accessibility.Diff(mainTree(), buried)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 2},
		{Kind: accessibility.ChildrenChanged, Node: 5},
	}, events, "the window's own list of children is what it was")
	ta.Publish(mainWindow, buried, events, sampleGeometry())
	signals = ta.peer.nextSignals(4)
	c.Equal(objectEvent(5, signalChildrenChanged, detailRemove, 1, 0, variantRef(nodeRef(7))), signals[0])
	c.Equal(nodeRef(7), ta.cachedID(signals[1]))
	c.Equal(objectEvent(1, signalChildrenChanged, detailAdd, 2, 0, variantRef(nodeRef(7))), signals[2])
	c.Equal(objectEvent(7, signalPropertyChange, propertyAccessibleParent, 0, 0, variantRef(nodeRef(1))), signals[3])
	replay(c, mainTree(), signals, buried)
}

// TestChildrenThatOnlyChangedOrder covers a child list whose membership is what it was but whose order is not, which is
// what sorting a table small enough for every row to be described does: the rows keep their ids and take new places.
// AT-SPI has no signal for a reordering, and libatspi answers GetChildren and GetChildAtIndex from a child array it
// keeps per object, so a publish that says nothing about it leaves an assistive technology reading the rows in the
// order they had before the sort for the life of the window.
func TestChildrenThatOnlyChangedOrder(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	sorted := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Children = []accessibility.NodeID{7, 6}
	})
	events := accessibility.Diff(mainTree(), sorted)
	c.Equal([]accessibility.Event{{Kind: accessibility.ChildrenChanged, Node: 5}}, events,
		"nothing joined or left the list, so this is all there is to go on")
	ta.Publish(mainWindow, sorted, events, sampleGeometry())

	// The row that moved is taken out of the list the client holds and put back where the new snapshot has it, which is
	// the pair of signals ATK's own bridge sends for the same thing.
	signals := ta.peer.nextSignals(2)
	c.Equal([]signalRecord{
		objectEvent(5, signalChildrenChanged, detailRemove, 1, 0, variantRef(nodeRef(7))),
		objectEvent(5, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(7))),
	}, signals)
	replay(c, mainTree(), signals, sorted)
	c.Equal([]dbus.ObjectRef{nodeRef(7), nodeRef(6)},
		ta.one(NodePath(5), InterfaceAccessible, "GetChildren", ""),
		"what was announced has to be what the object now reports")

	// The window's own children are reordered around the ignored group, which has no object of its own: what the client
	// holds is the two nodes standing in for it, and only the child that really moved among them is announced.
	shuffled := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Children = []accessibility.NodeID{7, 6}
		tree.Node(1).Children = []accessibility.NodeID{5, 2, 8, 9}
	})
	shuffled.Generation++
	events = accessibility.Diff(sorted, shuffled)
	c.Equal([]accessibility.Event{{Kind: accessibility.ChildrenChanged, Node: 1}}, events)
	ta.Publish(mainWindow, shuffled, events, sampleGeometry())
	signals = ta.peer.nextSignals(2)
	c.Equal([]signalRecord{
		objectEvent(1, signalChildrenChanged, detailRemove, 2, 0, variantRef(nodeRef(5))),
		objectEvent(1, signalChildrenChanged, detailAdd, 0, 0, variantRef(nodeRef(5))),
	}, signals, "one pair for the child that moved, and nothing for the ones that did not")
	replay(c, sorted, signals, shuffled)
	c.Equal([]dbus.ObjectRef{nodeRef(5), nodeRef(3), nodeRef(4), nodeRef(8), nodeRef(9)},
		ta.one(NodePath(1), InterfaceAccessible, "GetChildren", ""))
	ta.Announce("Nothing more about the order")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestActivationAndAFocusMoveInOneSnapshot covers the pair of events that clicking a control in a window that was not
// the active one produces. Both name the focus, and it has to be announced once: a client that is told twice says the
// same thing twice, and would be handed the old node's focused 0 after the new node's focused 1.
func TestActivationAndAFocusMoveInOneSnapshot(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	inactive := mainTree()
	inactive.Node(1).Focused = false
	ta.Publish(mainWindow, inactive, accessibility.Diff(mainTree(), inactive), sampleGeometry())
	ta.peer.nextSignals(3) // The window is no longer the active one, and its focus is no longer the focus

	reactivated := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Focused = false
		tree.Node(8).Focused = true
		tree.Focus = 8
	})
	events := accessibility.Diff(inactive, reactivated)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.WindowActivated, Node: 1},
		{Kind: accessibility.FocusChanged, Node: 8},
	}, events)
	ta.Publish(mainWindow, reactivated, events, sampleGeometry())
	c.Equal([]signalRecord{
		windowEvent(1, signalActivate, "Test Window"),
		stateEvent(1, stateNameActive, true),
		stateEvent(4, stateNameFocused, false),
		stateEvent(8, stateNameFocused, true),
		focusEvent(8),
	}, ta.peer.nextSignals(5))
	ta.Announce("Nothing more about the focus")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestDeactivationAndAFocusMoveInOneSnapshot covers the mirror image: the node that has to be told it no longer holds
// the focus is the one that held it while the window was still active, not whichever node the new snapshot points at.
func TestDeactivationAndAFocusMoveInOneSnapshot(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	away := mainTree()
	away.Node(1).Focused = false
	away.Node(4).Focused = false
	away.Node(8).Focused = true
	away.Focus = 8
	events := accessibility.Diff(mainTree(), away)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.WindowDeactivated, Node: 1},
		{Kind: accessibility.FocusChanged, Node: 8},
	}, events)
	ta.Publish(mainWindow, away, events, sampleGeometry())
	c.Equal([]signalRecord{
		windowEvent(1, signalDeactivate, "Test Window"),
		stateEvent(1, stateNameActive, false),
		stateEvent(4, stateNameFocused, false),
	}, ta.peer.nextSignals(3), "the field that held the focus is the one that loses it")
	ta.Announce("Nothing about the slider")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestARowThatStopsBeingExpandable covers the pair of states AT-SPI has for a thing that can be opened. [States] gives
// a node that cannot be expanded neither EXPANDED nor COLLAPSED, so losing the ability has to take the side the node
// was on with it rather than putting it on the other one: a client caches a state set until something retracts it, and
// COLLAPSED on an object whose state set has neither is wrong for as long as the window lives.
func TestARowThatStopsBeingExpandable(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	open := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Expandable = true
		tree.Node(5).Expanded = true
	})
	ta.Publish(mainWindow, open, accessibility.Diff(mainTree(), open), sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(5, stateNameExpandable, true),
		stateEvent(5, stateNameExpanded, true),
		stateEvent(5, stateNameCollapsed, false),
	}, ta.peer.nextSignals(3), "the ability and the side it is on arrive together, and each is said once")

	plain := activeMainTree(func(_ *accessibility.Tree) {})
	plain.Generation++
	events := accessibility.Diff(open, plain)
	c.Equal([]accessibility.Event{
		{
			Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpandable,
			Old: trueValue, New: falseValue,
		},
		{
			Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpanded,
			Old: trueValue, New: falseValue,
		},
	}, events)
	ta.Publish(mainWindow, plain, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(5, stateNameExpandable, false),
		stateEvent(5, stateNameExpanded, false),
	}, ta.peer.nextSignals(2), "the row is left with neither side of the pair rather than with the other one")
	ta.Announce("Nothing about being collapsed")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// bigTableTree is a window holding a table with more rows than it is worth walking, which is what makes it claim
// ATSPI_STATE_MANAGES_DESCENDANTS, along with the two of those rows it is showing:
//
//	50 window "Ledger"                (0,0 200x100)  active
//	└─ 51 table "Entries"             (0,0 200x100)  600 rows, 2 columns, multi-select
//	   ├─ 52 row "Opening"            (0,0 200x20)   row 10, selected
//	   └─ 53 row "Closing"            (0,20 200x20)  row 11
func bigTableTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 50, Role: role.Window, Name: "Ledger", Focused: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{51},
		},
		&accessibility.Node{
			ID: 51, Parent: 50, Role: role.Table, Name: "Entries", Multiselectable: true,
			RowCount: manageDescendantsRowThreshold + 100, ColumnCount: 2, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{52, 53},
		},
		&accessibility.Node{
			ID: 52, Parent: 51, Role: role.Row, Name: "Opening", RowIndex: 10, Selectable: true, Selected: true,
			Bounds: geom.NewRect(0, 0, 200, 20), Actions: selectionActions(),
		},
		&accessibility.Node{
			ID: 53, Parent: 51, Role: role.Row, Name: "Closing", RowIndex: 11, Selectable: true,
			Bounds: geom.NewRect(0, 20, 200, 20), Actions: selectionActions(),
		},
	)
}

// TestTheCurrentRowOfATableThatManagesItsDescendants covers the promise a container makes by claiming
// ATSPI_STATE_MANAGES_DESCENDANTS: it has told its client not to walk or cache what is inside it, so the only way the
// client can follow the user through it is object:active-descendant-changed.
func TestTheCurrentRowOfATableThatManagesItsDescendants(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ledger := bigTableTree()
	ta.Publish(tableWindow, ledger, nil, sampleGeometry())
	ta.peer.nextSignals(8) // Four cache items, the window's Create and place, and the two that say it is active
	c.True(States(ledger.Node(51), true, false).Has(StateManagesDescendants))

	moved := bigTableTree()
	moved.Generation++
	moved.Node(52).Selected = false
	moved.Node(53).Selected = true
	events := accessibility.Diff(ledger, moved)
	c.Equal([]accessibility.Event{
		{
			Kind: accessibility.StateChanged, Node: 52, State: accessibility.StateSelected,
			Old: trueValue, New: falseValue,
		},
		{
			Kind: accessibility.StateChanged, Node: 53, State: accessibility.StateSelected,
			Old: falseValue, New: trueValue,
		},
	}, events)
	ta.Publish(tableWindow, moved, events, sampleGeometry())
	// The table is told that its selection moved and which of its rows is current, after the rows themselves have said
	// what happened to them.
	c.Equal([]signalRecord{
		stateEvent(52, stateNameSelected, false),
		stateEvent(53, stateNameSelected, true),
		objectEvent(51, signalSelectionChanged, "", 0, 0, variantInt32(0)),
		objectEvent(51, signalActiveDescendantChanged, "", 1, 0, variantRef(nodeRef(53))),
	}, ta.peer.nextSignals(4))
}

// TestASmallTableSaysNothingAboutItsCurrentRow covers the other side of the same contract: a table a client is expected
// to walk reports its rows one by one and has no current descendant to announce.
func TestASmallTableSaysNothingAboutItsCurrentRow(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	small := bigTableTree()
	small.Node(51).RowCount = manageDescendantsRowThreshold
	ta.Publish(tableWindow, small, nil, sampleGeometry())
	ta.peer.nextSignals(8)
	c.False(States(small.Node(51), true, false).Has(StateManagesDescendants))

	moved := bigTableTree()
	moved.Node(51).RowCount = manageDescendantsRowThreshold
	moved.Generation++
	moved.Node(52).Selected = false
	moved.Node(53).Selected = true
	ta.Publish(tableWindow, moved, accessibility.Diff(small, moved), sampleGeometry())
	// The container still says that its selection moved — every container that can be selected within does — but there
	// is no current descendant to name, since a client is expected to walk this table's rows for itself.
	c.Equal([]signalRecord{
		stateEvent(52, stateNameSelected, false),
		stateEvent(53, stateNameSelected, true),
		objectEvent(51, signalSelectionChanged, "", 0, 0, variantInt32(0)),
	}, ta.peer.nextSignals(3))
	ta.Announce("Nothing about the table")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestFilteringATableMovesManagesDescendants covers a table filtered down to fewer rows than are worth leaving to it,
// which filtering one does in normal use. The row count reaches the adapter as nothing but an attribute change while
// MANAGES_DESCENDANTS is a state, so a client that is not told goes on refusing to walk or cache the rows — and
// [Adapter.emitActiveDescendants], which reads the new snapshot, stops naming the current one, leaving it with neither
// route to the row the user is on.
func TestFilteringATableMovesManagesDescendants(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ledger := bigTableTree()
	ta.Publish(tableWindow, ledger, nil, sampleGeometry())
	ta.peer.nextSignals(8) // Four cache items, the window's Create and place, and the two that say it is active

	filtered := bigTableTree()
	filtered.Generation++
	filtered.Node(51).RowCount = 2
	events := accessibility.Diff(ledger, filtered)
	c.Equal([]accessibility.Event{{Kind: accessibility.AttributesChanged, Node: 51}}, events,
		"a row count moves as nothing but an attribute change")
	ta.Publish(tableWindow, filtered, events, sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(51, signalAttributesChanged, "", 0, 0, variantInt32(0)),
		stateEvent(51, stateNameManagesDescendants, false),
	}, ta.peer.nextSignals(2))
	c.False(States(filtered.Node(51), true, false).Has(StateManagesDescendants),
		"what was announced has to be what the object now reports")

	// Taking the filter off again puts the promise back.
	restored := bigTableTree()
	restored.Generation += 2
	ta.Publish(tableWindow, restored, accessibility.Diff(filtered, restored), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(51, signalAttributesChanged, "", 0, 0, variantInt32(0)),
		stateEvent(51, stateNameManagesDescendants, true),
	}, ta.peer.nextSignals(2))
	c.True(States(restored.Node(51), true, false).Has(StateManagesDescendants))
}

func TestPropertyAndAttributeChanges(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	changed := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(3).Name = "Full name:"
		tree.Node(5).Description = "The things"
		tree.Node(8).Value = "7"
		tree.Node(8).Number = 7
		tree.Node(9).Sort = accessibility.SortAscending
	})
	events := accessibility.Diff(mainTree(), changed)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 3, Old: testLabelName, New: "Full name:"},
		{Kind: accessibility.DescriptionChanged, Node: 5, New: "The things"},
		{Kind: accessibility.ValueChanged, Node: 8, Old: "4", New: "7"},
		{Kind: accessibility.NumberChanged, Node: 8, Old: "4", New: "7"},
		{Kind: accessibility.SortChanged, Node: 9, Old: "unsorted", New: "ascending"},
	}, events, "one change to the slider arrives as both of the value events")
	ta.Publish(mainWindow, changed, events, sampleGeometry())
	// AT-SPI has one accessible-value property, and ATK defines it as a number, so the two value events become one
	// signal carrying a double rather than two carrying different types under the same property name.
	c.Equal([]signalRecord{
		objectEvent(3, signalPropertyChange, propertyAccessibleName, 0, 0, variantString("Full name:")),
		objectEvent(5, signalPropertyChange, propertyAccessibleDescription, 0, 0, variantString("The things")),
		objectEvent(8, signalPropertyChange, propertyAccessibleValue, 0, 0, variantDouble(7)),
		objectEvent(9, signalAttributesChanged, "", 0, 0, variantInt32(0)),
	}, ta.peer.nextSignals(4))

	// A control with a textual value and no number behind it has no org.a11y.atspi.Value interface to read a number
	// back from, so there is no accessible-value to report about it. What it does have is the read-only text
	// synthesized from that value, and a change to it is the old value being replaced by the new one, which is what
	// Orca re-reads a control on.
	textual := activeMainTree(func(tree *accessibility.Tree) { tree.Node(4).Value = changedFieldValue })
	ta.Publish(mainWindow, textual, accessibility.Diff(mainTree(), textual), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextChanged, detailDelete, 0, 4, variantString(testFieldValue)),
		objectEvent(4, signalTextChanged, detailInsert, 0, 6, variantString(changedFieldValue)),
	}, ta.peer.nextSignals(2))

	// A value that is emptied is deleted with nothing put in its place.
	emptied := activeMainTree(func(tree *accessibility.Tree) { tree.Node(4).Value = "" })
	ta.Publish(mainWindow, emptied, accessibility.Diff(textual, emptied), sampleGeometry())
	c.Equal(objectEvent(4, signalTextChanged, detailDelete, 0, 6, variantString(changedFieldValue)),
		ta.peer.nextSignal())

	// A node that carries text of its own says nothing about its value: the text has events of its own, and the value
	// beside it is a description of the same thing. The field gains its text without any event, as in TestTextEvents,
	// since a node gaining an interface is not something AT-SPI has an event for.
	withText := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 5, SelEnd: 5, Caret: 5}
	})
	ta.Publish(mainWindow, withText, nil, sampleGeometry())
	named := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Value = textAfter
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 5, SelEnd: 5, Caret: 5}
	})
	events = accessibility.Diff(withText, named)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.ValueChanged, Node: 4, Old: testFieldValue, New: textAfter},
	}, events)
	ta.Publish(mainWindow, named, events, sampleGeometry())
	ta.Announce("Nothing else about the field")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestAttributeChangesThatHaveNoSignalOfTheirOwn covers the event [accessibility.Diff] reports for the parts of a node
// that AT-SPI carries as object attributes rather than through an interface — its level, where it sits in a set, its
// orientation, its placeholder — along with the relations, which have no AT-SPI signal at all and are reported here
// as well.
func TestAttributeChangesThatHaveNoSignalOfTheirOwn(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	changed := activeMainTree(func(tree *accessibility.Tree) { tree.Node(4).Placeholder = "Your full name" })
	ta.Publish(mainWindow, changed, []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: 4},
		{Kind: accessibility.AttributesChanged, Node: 404},
	}, sampleGeometry())
	c.Equal(objectEvent(4, signalAttributesChanged, "", 0, 0, variantInt32(0)), ta.peer.nextSignal())
	ta.Announce("Nothing about a node with no object")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)

	// [accessibility.Diff] reports a sort change and an attribute change as two events for the same node, and both say
	// the same thing here: the whole attribute set is worth re-reading. Sending the signal twice has an assistive
	// technology re-read it twice, so it is sent once per node however many of the two arrive.
	sorted := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Placeholder = "Your full name"
		tree.Node(9).Sort = accessibility.SortAscending
		tree.Node(9).ColumnIndex = 2
	})
	events := accessibility.Diff(changed, sorted)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.SortChanged, Node: 9, Old: "unsorted", New: "ascending"},
		{Kind: accessibility.AttributesChanged, Node: 9},
	}, events)
	ta.Publish(mainWindow, sorted, events, sampleGeometry())
	c.Equal(objectEvent(9, signalAttributesChanged, "", 0, 0, variantInt32(0)), ta.peer.nextSignal())
	ta.Announce("Nothing more about the attributes")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestAnOrientationChangeIsAStateChange covers the part of [accessibility.AttributesChanged] that AT-SPI does not hold
// as an object attribute at all. A scroll bar whose Horizontal flips and a slider whose Vertical flips both move the
// HORIZONTAL and VERTICAL states that [roleStates] puts in every state set, and a client caches a state set until
// something retracts it, so reporting the attribute change alone leaves the old orientation in place for the life of
// the window.
func TestAnOrientationChangeIsAStateChange(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	turned := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(8).Orientation = accessibility.OrientationVertical
	})
	events := accessibility.Diff(mainTree(), turned)
	c.Equal([]accessibility.Event{{Kind: accessibility.AttributesChanged, Node: 8}}, events,
		"an orientation moves as nothing but an attribute change")
	ta.Publish(mainWindow, turned, events, sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(8, signalAttributesChanged, "", 0, 0, variantInt32(0)),
		stateEvent(8, stateNameHorizontal, false),
		stateEvent(8, stateNameVertical, true),
	}, ta.peer.nextSignals(3))
	states := States(turned.Node(8), true, false)
	c.False(states.Has(StateHorizontal), "what was announced has to be what the object now reports")
	c.True(states.Has(StateVertical))

	// A node that loses its orientation altogether has only the side it was on retracted: the other was never reported,
	// and handing a client a state to retract that it was never given leaves it holding the opposite of the truth.
	plain := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(8).Orientation = accessibility.OrientationNone
	})
	plain.Generation++
	ta.Publish(mainWindow, plain, accessibility.Diff(turned, plain), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(8, signalAttributesChanged, "", 0, 0, variantInt32(0)),
		stateEvent(8, stateNameVertical, false),
	}, ta.peer.nextSignals(2))
	ta.Announce("Nothing about being horizontal")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

func TestBoundsChangesAreOnlyReportedForWindows(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	moved := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(1).Bounds = geom.NewRect(0, 0, 300, 150)
		tree.Node(8).Bounds = geom.NewRect(0, 120, 300, 20)
	})
	events := accessibility.Diff(mainTree(), moved)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.BoundsChanged, Node: 1},
		{Kind: accessibility.BoundsChanged, Node: 8},
	}, events)
	ta.Publish(mainWindow, moved, events, sampleGeometry())
	// The window reports its new area on the screen; the slider inside it reports nothing, since everything inside a
	// window moves whenever anything is scrolled or relaid out.
	c.Equal(objectEvent(1, signalBoundsChanged, "", 0, 0,
		variantRect(100, 50, 600, 300)), ta.peer.nextSignal())
	ta.Announce("Moved")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestSetGeometryReportsTheWindowsNewBounds covers the other way a window's area on the screen changes: not because the
// tree says the window is a different size, but because the window has been moved, has changed screens or has changed
// backing scale, none of which the tree knows anything about. The root package refreshes the geometry on every publish
// as well as on every move and resize, so the signal has to be sent only when something actually changed, or a window
// that redraws continuously would announce its bounds on every frame.
func TestSetGeometryReportsTheWindowsNewBounds(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ta.SetGeometry(mainWindow, sampleGeometry()) // The geometry it already has, so nothing is announced
	ta.SetGeometry(mainWindow, Geometry{Origin: geom.NewPoint(300, 200), Scale: geom.NewPoint(1, 1)})
	c.Equal(objectEvent(1, signalBoundsChanged, "", 0, 0, variantRect(300, 200, 200, 150)), ta.peer.nextSignal())
	// The nodes inside the window report the move too, when they are asked, but nothing is sent about them.
	c.Equal(dbus.Struct{int32(360), int32(210), int32(100), int32(20)},
		ta.one(NodePath(4), InterfaceComponent, "GetExtents", "u", uint32(CoordScreen)))
	ta.SetGeometry(otherWindow, sampleGeometry()) // A window that is not there announces nothing
	ta.Announce("Moved")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

func TestStateChanges(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	for _, one := range []struct {
		tree     *accessibility.Tree
		name     string
		expected []signalRecord
		event    accessibility.Event
	}{
		{
			name: "becoming disabled is the loss of two AT-SPI states",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 8, State: accessibility.StateDisabled,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{
				stateEvent(8, stateNameEnabled, false),
				stateEvent(8, stateNameSensitive, false),
			},
		},
		{
			name: "becoming read only is also the loss of being editable",
			tree: activeMainTree(func(tree *accessibility.Tree) {
				tree.Node(4).Text = &accessibility.TextInfo{Text: textBefore}
			}),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateReadOnly,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{
				stateEvent(4, stateNameReadOnly, true),
				stateEvent(4, stateNameEditable, false),
			},
		},
		{
			// A slider's state set never holds EDITABLE, so there is nothing for it to lose along with the change,
			// and retracting a state it never had would leave a client holding the opposite of the truth.
			name: "a control that was never editable only gains read only",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 8, State: accessibility.StateReadOnly,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{stateEvent(8, stateNameReadOnly, true)},
		},
		{
			// AT-SPI has a state for each side of the pair, and [States] puts COLLAPSED on every unexpanded
			// expandable node, so expanding one has to retract it.
			name: "expanding a row retracts being collapsed",
			tree: activeMainTree(func(tree *accessibility.Tree) {
				tree.Node(5).Expandable = true
				tree.Node(5).Expanded = true
			}),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpanded,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{
				stateEvent(5, stateNameExpanded, true),
				stateEvent(5, stateNameCollapsed, false),
			},
		},
		{
			name: "collapsing it again",
			tree: activeMainTree(func(tree *accessibility.Tree) { tree.Node(5).Expandable = true }),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpanded,
				Old: trueValue, New: falseValue,
			},
			expected: []signalRecord{
				stateEvent(5, stateNameExpanded, false),
				stateEvent(5, stateNameCollapsed, true),
			},
		},
		{
			name: "becoming expandable while already open",
			tree: activeMainTree(func(tree *accessibility.Tree) {
				tree.Node(5).Expandable = true
				tree.Node(5).Expanded = true
			}),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpandable,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{
				stateEvent(5, stateNameExpandable, true),
				stateEvent(5, stateNameExpanded, true),
			},
		},
		{
			// The ability to expand never arrives on its own either: a node that gains it gains the side of the pair
			// it is on at the same moment, and only that side, since the other was never reported.
			name: "becoming expandable says which way it is",
			tree: activeMainTree(func(tree *accessibility.Tree) { tree.Node(5).Expandable = true }),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpandable,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{
				stateEvent(5, stateNameExpandable, true),
				stateEvent(5, stateNameCollapsed, true),
			},
		},
		{
			// This one follows straight on from the case above, so the snapshot it replaces is the collapsed
			// expandable list, and COLLAPSED is what the client holds and what has to be taken back.
			name: "and losing it takes the side it was on away",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateExpandable,
				Old: trueValue, New: falseValue,
			},
			expected: []signalRecord{
				stateEvent(5, stateNameExpandable, false),
				stateEvent(5, stateNameCollapsed, false),
			},
		},
		{
			name: "being scrolled out of view is no longer showing",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateOffscreen,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{stateEvent(6, stateNameShowing, false)},
		},
		{
			// A plain list item carries no check, so its state set holds neither CHECKED nor INDETERMINATE however its
			// Checked field is left. A ProvideAccessibility that fills the field in without setting HasCheck makes
			// [accessibility.Diff] report the change all the same, and announcing it would leave a caching client
			// holding a state this package never reports and never retracts.
			name: "a node that carries no check says nothing about being checked",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.Off.Key(), New: checkenum.On.Key(),
			},
		},
		{
			name: "nor about being neither checked nor unchecked",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.On.Key(), New: checkenum.Mixed.Key(),
			},
		},
		{
			name: "nor about leaving the indeterminate state behind",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.Mixed.Key(), New: checkenum.On.Key(),
			},
		},
		{
			// The same three changes on a row that does carry a check, which is what CHECKED and INDETERMINATE belong
			// to. The check arrives with this snapshot, so the row gains CHECKABLE at the same moment.
			name: "being checked",
			tree: checkableRowTree(),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.Off.Key(), New: checkenum.On.Key(),
			},
			expected: []signalRecord{
				stateEvent(6, stateNameCheckable, true),
				stateEvent(6, stateNameChecked, true),
			},
		},
		{
			name: "becoming neither checked nor unchecked",
			tree: checkableRowTree(),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.On.Key(), New: checkenum.Mixed.Key(),
			},
			expected: []signalRecord{
				stateEvent(6, stateNameChecked, false),
				stateEvent(6, stateNameIndeterminate, true),
			},
		},
		{
			name: "leaving the indeterminate state behind",
			tree: checkableRowTree(),
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.Mixed.Key(), New: checkenum.On.Key(),
			},
			expected: []signalRecord{
				stateEvent(6, stateNameChecked, true),
				stateEvent(6, stateNameIndeterminate, false),
			},
		},
		{
			// The container is told as well as the item, since an assistive technology following a list or a tab list
			// reads the container's signal and nothing else.
			name: "selecting an item also moves its container's selection",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateSelected,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{
				stateEvent(7, stateNameSelected, true),
				objectEvent(5, signalSelectionChanged, "", 0, 0, variantInt32(0)),
			},
		},
		{
			// [roleStates] only ever gives PRESSED to the two roles whose pressed-ness is their check, so announcing it
			// for anything else would move a state the object's own GetState never reports and leave a caching client
			// holding one Unison will never retract.
			name: "a state the role's own state set never holds is not announced",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 5, State: accessibility.StatePressed,
				Old: falseValue, New: trueValue,
			},
		},
		{
			name: "a node that has no object at all",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateBusy,
				Old: falseValue, New: trueValue,
			},
		},
		{
			name: "a state AT-SPI does not have",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateProtected,
				Old: falseValue, New: trueValue,
			},
		},
	} {
		tree := one.tree
		if tree == nil {
			tree = mainTree()
		}
		ta.Publish(mainWindow, tree, []accessibility.Event{one.event}, sampleGeometry())
		if len(one.expected) != 0 {
			c.Equal(one.expected, ta.peer.nextSignals(len(one.expected)), one.name)
		}
		ta.Announce(one.name)
		c.Equal(signalAnnouncement, ta.peer.nextSignal().member, one.name)
	}
	for _, one := range []struct {
		expected string
		state    accessibility.State
	}{
		{state: accessibility.StateFocusable, expected: stateNameFocusable},
		{state: accessibility.StateSelectable, expected: stateNameSelectable},
		{state: accessibility.StateMultiselectable, expected: stateNameMultiselectable},
		{state: accessibility.StateModal, expected: stateNameModal},
		{state: accessibility.StateBusy, expected: stateNameBusy},
		{state: accessibility.StateInvalid, expected: stateNameInvalidEntry},
	} {
		ta.Publish(mainWindow, mainTree(), []accessibility.Event{{
			Kind: accessibility.StateChanged, Node: 5, State: one.state, Old: trueValue, New: falseValue,
		}}, sampleGeometry())
		c.Equal([]signalRecord{stateEvent(5, one.expected, false)}, ta.peer.nextSignals(1), one.state.String())
	}
}

// toggleTree is a window of the controls whose AT-SPI states come from more of the node than the flag that changed:
//
//	90 window "Options"              (0,0 200x80)   active
//	├─ 91 toggle button "Bold"       (0,0 100x20)   pressed
//	├─ 92 disclosure triangle        (0,20 20x20)
//	└─ 93 menu item "Wrap"           (0,40 200x20)  carries a check, currently off
func toggleTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 90, Role: role.Window, Name: "Options", Focused: true, Bounds: geom.NewRect(0, 0, 200, 80),
			Children: []accessibility.NodeID{91, 92, 93},
		},
		&accessibility.Node{
			ID: 91, Parent: 90, Role: role.ToggleButton, Name: "Bold", Pressed: true,
			Bounds: geom.NewRect(0, 0, 100, 20),
		},
		&accessibility.Node{ID: 92, Parent: 90, Role: role.DisclosureTriangle, Bounds: geom.NewRect(0, 20, 20, 20)},
		&accessibility.Node{
			ID: 93, Parent: 90, Role: role.MenuItem, Name: "Wrap", HasCheck: true, Bounds: geom.NewRect(0, 40, 200, 20),
		},
	)
}

// TestStateChangesThatTheNodeDecides covers the two states no flag of its own ever moves: the CHECKED that a toggle's
// pressed-ness stands for, and the CHECKABLE that says whether a node carries a check at all. A client caches a state
// set until something retracts it, so a state this package puts in one and never takes out again is wrong for as long
// as the window lives.
func TestStateChangesThatTheNodeDecides(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	options := toggleTree()
	ta.Publish(toggleWindow, options, nil, sampleGeometry())
	ta.peer.nextSignals(8) // Four cache items, the window's Create and place, and the two that say it is active
	c.True(States(options.Node(91), true, false).Has(StateChecked),
		"a toggle that is down is a control that is checked")

	// Releasing the toggle has to retract that CHECKED as well as the PRESSED it came with.
	released := toggleTree()
	released.Generation++
	released.Node(91).Pressed = false
	events := accessibility.Diff(options, released)
	c.Equal([]accessibility.Event{{
		Kind: accessibility.StateChanged, Node: 91, State: accessibility.StatePressed,
		Old: trueValue, New: falseValue,
	}}, events)
	ta.Publish(toggleWindow, released, events, sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(91, stateNamePressed, false),
		stateEvent(91, stateNameChecked, false),
	}, ta.peer.nextSignals(2))

	// A disclosure triangle is the other role AT-SPI reports as a toggle, so its pressed-ness is its check too.
	opened := toggleTree()
	opened.Generation += 2
	opened.Node(91).Pressed = false
	opened.Node(92).Pressed = true
	ta.Publish(toggleWindow, opened, accessibility.Diff(released, opened), sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(92, stateNamePressed, true),
		stateEvent(92, stateNameChecked, true),
	}, ta.peer.nextSignals(2))

	// A menu item that stops carrying a check loses CHECKABLE, and says so even though its check state has not moved.
	plain := toggleTree()
	plain.Generation += 3
	plain.Node(91).Pressed = false
	plain.Node(92).Pressed = true
	plain.Node(93).HasCheck = false
	events = accessibility.Diff(opened, plain)
	c.Equal([]accessibility.Event{{
		Kind: accessibility.StateChanged, Node: 93, State: accessibility.StateChecked,
		Old: checkenum.Off.Key(), New: checkenum.Off.Key(),
	}}, events, "losing the check is a change even though the check state is what it was")
	ta.Publish(toggleWindow, plain, events, sampleGeometry())
	// It also stops being a check menu item, since that is what this package reports a menu item carrying a check as.
	// The role is decided by more of the node than the schema's role, so a client is told about it here rather than by
	// a role change that never comes.
	c.Equal([]signalRecord{
		stateEvent(93, stateNameCheckable, false),
		objectEvent(93, signalPropertyChange, propertyAccessibleRole, 0, 0, variantUint32(uint32(RoleMenuItem))),
	}, ta.peer.nextSignals(2), "nothing but the checkability and the role it decides moved")

	// Getting it back while already checked gains both states at once.
	ticked := toggleTree()
	ticked.Generation += 4
	ticked.Node(91).Pressed = false
	ticked.Node(92).Pressed = true
	ticked.Node(93).Checked = checkenum.On
	ta.Publish(toggleWindow, ticked, accessibility.Diff(plain, ticked), sampleGeometry())
	c.Equal([]signalRecord{
		stateEvent(93, stateNameCheckable, true),
		stateEvent(93, stateNameChecked, true),
		objectEvent(93, signalPropertyChange, propertyAccessibleRole, 0, 0, variantUint32(uint32(RoleCheckMenuItem))),
	}, ta.peer.nextSignals(3))
}

// TestARoleChangeIsAPropertyChange covers a live control becoming a different kind of thing, which happens whenever a
// label is given a drawable instead of text or a button is made sticky. AT-SPI has no event of its own for it: the role
// is a property, and its value is the role's number rather than its name.
func TestARoleChangeIsAPropertyChange(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	swapped := activeMainTree(func(tree *accessibility.Tree) { tree.Node(3).Role = role.Image })
	events := accessibility.Diff(mainTree(), swapped)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.RoleChanged, Node: 3, Old: role.Label.Key(), New: role.Image.Key()},
	}, events)
	ta.Publish(mainWindow, swapped, events, sampleGeometry())
	c.Equal(objectEvent(3, signalPropertyChange, propertyAccessibleRole, 0, 0, variantUint32(uint32(RoleImage))),
		ta.peer.nextSignal())

	// The role reported is the one this package would answer with rather than the one the event names, since an
	// unnamed group is a panel while a named one is a grouping.
	named := activeMainTree(func(tree *accessibility.Tree) { tree.Node(5).Role = role.Group })
	ta.Publish(mainWindow, named, accessibility.Diff(mainTree(), named), sampleGeometry())
	c.Equal(objectEvent(5, signalPropertyChange, propertyAccessibleRole, 0, 0, variantUint32(uint32(RoleGrouping))),
		ta.peer.nextSignal())

	// A node with no object at all says nothing, as it has nothing for a property to have changed on.
	ignored := activeMainTree(func(tree *accessibility.Tree) { tree.Node(2).Role = role.Toolbar })
	ta.Publish(mainWindow, ignored, accessibility.Diff(mainTree(), ignored), sampleGeometry())
	ta.Announce("Nothing about the group")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

// TestARoleChangeWithNoRoleChangedEvent covers the rest of what decides the role this package reports. A group gaining
// a name, a text field being protected and a menu item gaining a check each move it without the schema's own role
// moving, so [accessibility.Diff] reports something else entirely — or, for the protected field, a state AT-SPI has no
// state for and would otherwise say nothing about at all. A client that is not told goes on believing the old role for
// as long as the window lives.
func TestARoleChangeWithNoRoleChangedEvent(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	// The list becomes an unnamed group, which is layout rather than anything worth announcing.
	unnamed := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Role = role.Group
		tree.Node(5).Name = ""
	})
	ta.Publish(mainWindow, unnamed, accessibility.Diff(mainTree(), unnamed), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(5, signalPropertyChange, propertyAccessibleRole, 0, 0, variantUint32(uint32(RolePanel))),
		objectEvent(5, signalPropertyChange, propertyAccessibleName, 0, 0, variantString("")),
	}, ta.peer.nextSignals(2))

	// Giving it a name turns it into a group worth announcing, and nothing but the name has changed.
	renamed := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(5).Role = role.Group
		tree.Node(5).Name = "Choices"
	})
	renamed.Generation++
	events := accessibility.Diff(unnamed, renamed)
	c.Equal([]accessibility.Event{{Kind: accessibility.NameChanged, Node: 5, New: "Choices"}}, events)
	ta.Publish(mainWindow, renamed, events, sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(5, signalPropertyChange, propertyAccessibleName, 0, 0, variantString("Choices")),
		objectEvent(5, signalPropertyChange, propertyAccessibleRole, 0, 0, variantUint32(uint32(RoleGrouping))),
	}, ta.peer.nextSignals(2))

	// A text field that becomes a password field is AT-SPI's password text role: it has no state for being protected,
	// so the role is the only thing that says what the control now is.
	secret := activeMainTree(func(tree *accessibility.Tree) { tree.Node(4).Protected = true })
	events = accessibility.Diff(mainTree(), secret)
	c.Equal([]accessibility.Event{{
		Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateProtected,
		Old: falseValue, New: trueValue,
	}}, events)
	ta.Publish(mainWindow, secret, events, sampleGeometry())
	c.Equal(objectEvent(4, signalPropertyChange, propertyAccessibleRole, 0, 0,
		variantUint32(uint32(RolePasswordText))), ta.peer.nextSignal())
	ta.Announce("Nothing more about the field")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

func TestTextEvents(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	// A field that already holds text, published without events, since a field gaining an interface is not something
	// AT-SPI has an event for.
	before := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textBefore, SelStart: 4, SelEnd: 4, Caret: 4}
	})
	ta.Publish(mainWindow, before, nil, sampleGeometry())

	// Typing a character at the end of the field.
	typed := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 5, SelEnd: 5, Caret: 5}
	})
	events := accessibility.Diff(before, typed)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.TextInserted, Node: 4, Start: 4, Length: 1, New: "a"},
		{Kind: accessibility.TextSelectionChanged, Node: 4, Start: 5},
	}, events)
	ta.Publish(mainWindow, typed, events, sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextChanged, detailInsert, 4, 1, variantString("a")),
		objectEvent(4, signalTextCaretMoved, "", 5, 0, variantInt32(0)),
	}, ta.peer.nextSignals(2))

	// Selecting the whole field, which moves the caret and adds a range to report.
	selected := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 0, SelEnd: 5, Caret: 5}
	})
	ta.Publish(mainWindow, selected, accessibility.Diff(typed, selected), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextCaretMoved, "", 5, 0, variantInt32(0)),
		objectEvent(4, signalTextSelectionChanged, "", 0, 0, variantString("")),
	}, ta.peer.nextSignals(2))

	// Extending the selection backwards from its end, which leaves the caret at its start rather than at its end. The
	// caret is read from the node, since the event says only where the selection is.
	backwards := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 2, SelEnd: 5, Caret: 2}
	})
	ta.Publish(mainWindow, backwards, accessibility.Diff(selected, backwards), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextCaretMoved, "", 2, 0, variantInt32(0)),
		objectEvent(4, signalTextSelectionChanged, "", 0, 0, variantString("")),
	}, ta.peer.nextSignals(2))

	// Deleting the selection, which both empties the field and collapses a range to a bare caret. Orca replaces the
	// selection it is holding only when it is told the selection changed, so the collapse has to be announced as well
	// as the deletion, or it goes on reading a range the control no longer has.
	emptied := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{}
	})
	events = accessibility.Diff(backwards, emptied)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.TextDeleted, Node: 4, Start: 0, Length: 5, Old: textAfter},
		{Kind: accessibility.TextSelectionChanged, Node: 4},
	}, events)
	ta.Publish(mainWindow, emptied, events, sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextChanged, detailDelete, 0, 5, variantString(textAfter)),
		objectEvent(4, signalTextCaretMoved, "", 0, 0, variantInt32(0)),
		objectEvent(4, signalTextSelectionChanged, "", 0, 0, variantString("")),
	}, ta.peer.nextSignals(3))

	// Moving a bare caret about says nothing about a selection, since there was none and there is none.
	moved := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textBefore, SelStart: 1, SelEnd: 1, Caret: 1}
	})
	ta.Publish(mainWindow, moved, accessibility.Diff(emptied, moved), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextChanged, detailInsert, 0, 4, variantString(textBefore)),
		objectEvent(4, signalTextCaretMoved, "", 1, 0, variantInt32(0)),
	}, ta.peer.nextSignals(2))
	ta.Announce("Nothing about a selection")
	c.Equal(signalAnnouncement, ta.peer.nextSignal().member)
}

func TestAnnouncementsComeFromTheApplication(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	ta.Announce("") // Nothing to say, nothing sent
	ta.Announce("Saved")
	c.Equal(signalRecord{
		path:   RootPath,
		iface:  InterfaceEventObject,
		member: signalAnnouncement,
		args:   eventBody("", livePolite, 0, variantString("Saved")),
	}, ta.peer.nextSignal())
}

func TestPublishNeverWaitsForTheBus(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	a := newStalledAdapter(t, c)
	// Nothing is reading the other end of the connection any more, so every one of these signals has to be queued
	// rather than written. None of it may hold the user interface thread up.
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.Publish(mainWindow, mainTree(), nil, sampleGeometry())
		a.Publish(mainWindow, otherTree(), accessibility.Diff(mainTree(), otherTree()), sampleGeometry())
		a.Announce("Nobody is listening")
		a.RemoveWindow(mainWindow)
		// Shutting down has to be as quick: switching a screen reader off is exactly when the far end stops reading,
		// and telling the registry the application is going is the one thing here that is a method call.
		a.Stop()
	}()
	select {
	case <-done:
	case <-time.After(testTimeout):
		t.Fatal("publishing blocked on a peer that had stopped reading")
	}
}

// newStalledAdapter starts an adapter whose peer answers the handshake and lets the application into the accessibility
// tree, and then stops reading, so that everything sent afterwards has nowhere to go.
func newStalledAdapter(t *testing.T, c check.Checker) *Adapter {
	t.Helper()
	clientSide, peerSide := net.Pipe()
	client, err := dbus.Connect(clientSide)
	c.NoError(err)
	t.Cleanup(func() {
		client.Close()
		xio.CloseIgnoringErrors(peerSide)
	})
	stalled := make(chan struct{})
	go answerUntilEmbedded(peerSide, stalled)
	c.NoError(client.Hello())
	a, err := Start(Config{ToolkitVersion: testToolkitVersion, conn: client})
	c.NoError(err)
	<-stalled
	return a
}

// answerUntilEmbedded answers the handshake and the call that joins the accessibility tree on the far end of a
// connection, closes stalled, and stops reading. Anything the adapter sends from then on stays in its queue.
func answerUntilEmbedded(peerSide net.Conn, stalled chan struct{}) {
	defer close(stalled)
	in := bufio.NewReader(peerSide)
	var serial uint32
	for {
		msg, err := dbus.Decode(in)
		if err != nil {
			return
		}
		if msg.Type != dbus.TypeMethodCall {
			continue
		}
		reply := dbus.NewReply(msg)
		switch msg.Member {
		case helloMember:
			err = reply.SetBodyWithSignature("s", testBusName)
		case embedMember:
			err = reply.SetBodyWithSignature(objectRefSignature,
				dbus.ObjectRef{Name: testPeerName, Path: testDesktopPath})
		default:
		}
		if err != nil {
			return
		}
		serial++
		reply.Serial = serial
		var data []byte
		if data, err = reply.Encode(); err != nil {
			return
		}
		if _, err = peerSide.Write(data); err != nil {
			return
		}
		if msg.Member == embedMember {
			return
		}
	}
}
