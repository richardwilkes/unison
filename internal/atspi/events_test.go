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

// mainWindowSignals is how many signals the first publish of the main window sends: the window's Create, the
// application root gaining a child, one cache item for each of the eight reported nodes, and then the four signals that
// say the window is active and where its focus is.
const mainWindowSignals = 14

// falseValue is what a boolean state change that has been turned off carries, which is what strconv.FormatBool writes.
const falseValue = "false"

// The text the field holds before and after a character is typed into it.
const (
	textBefore = "Will"
	textAfter  = "Willa"
)

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

// activeMainTree returns the main window's tree with the given changes applied, keeping it the active window.
func activeMainTree(apply func(t *accessibility.Tree)) *accessibility.Tree {
	tree := mainTree()
	tree.Generation++
	apply(tree)
	return tree
}

func TestFirstPublishAnnouncesTheWholeWindow(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	signals := ta.peer.nextSignals(mainWindowSignals)

	// The window itself, then its place among the application's children.
	c.Equal(windowEvent(1, signalCreate, "Test Window"), signals[0])
	c.Equal(rootChildrenEvent(detailAdd, 0, 1), signals[1])

	// One cache item per reported node, in the order a walk of the tree reaches them.
	for i, id := range mainWindowNodes {
		c.Equal(nodeRef(id), ta.cachedID(signals[2+i]), "cache item %d", i)
	}
	c.Equal([]any{dbus.Struct{
		nodeRef(1), rootRef(), rootRef(), int32(0), int32(5),
		[]string{InterfaceAccessible, InterfaceComponent},
		"Test Window", uint32(RoleFrame), "",
		States(mainTree().Node(1), true).Words(),
	}}, signals[2].args, "the window's cache item says everything the cache would")

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
	c.Equal(windowEvent(20, signalCreate, "Pick One"), signals[0])
	c.Equal(rootChildrenEvent(detailAdd, 1, 20), signals[1])
	for i, id := range []accessibility.NodeID{20, 21, 22} {
		c.Equal(nodeRef(id), ta.cachedID(signals[2+i]), "cache item %d", i)
	}

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
	ta.Publish(mainWindow, changed, accessibility.Diff(mainTree(), changed), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(3, signalPropertyChange, propertyAccessibleName, 0, 0, variantString("Full name:")),
		objectEvent(5, signalPropertyChange, propertyAccessibleDescription, 0, 0, variantString("The things")),
		objectEvent(8, signalPropertyChange, propertyAccessibleValue, 0, 0, variantString("7")),
		objectEvent(8, signalPropertyChange, propertyAccessibleValue, 0, 0, variantDouble(7)),
		objectEvent(9, signalAttributesChanged, "", 0, 0, variantInt32(0)),
	}, ta.peer.nextSignals(5))
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
			name: "being scrolled out of view is no longer showing",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateOffscreen,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{stateEvent(6, stateNameShowing, false)},
		},
		{
			name: "being checked",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 6, State: accessibility.StateChecked,
				Old: checkenum.Off.Key(), New: checkenum.On.Key(),
			},
			expected: []signalRecord{stateEvent(6, stateNameChecked, true)},
		},
		{
			name: "becoming neither checked nor unchecked",
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
			name: "the simple states",
			event: accessibility.Event{
				Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateSelected,
				Old: falseValue, New: trueValue,
			},
			expected: []signalRecord{stateEvent(7, stateNameSelected, true)},
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
		ta.Publish(mainWindow, mainTree(), []accessibility.Event{one.event}, sampleGeometry())
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
		{state: accessibility.StatePressed, expected: stateNamePressed},
		{state: accessibility.StateModal, expected: stateNameModal},
		{state: accessibility.StateBusy, expected: stateNameBusy},
		{state: accessibility.StateInvalid, expected: stateNameInvalidEntry},
		{state: accessibility.StateExpandable, expected: stateNameExpandable},
		{state: accessibility.StateExpanded, expected: stateNameExpanded},
	} {
		ta.Publish(mainWindow, mainTree(), []accessibility.Event{{
			Kind: accessibility.StateChanged, Node: 5, State: one.state, Old: trueValue, New: falseValue,
		}}, sampleGeometry())
		c.Equal([]signalRecord{stateEvent(5, one.expected, false)}, ta.peer.nextSignals(1), one.state.String())
	}
}

func TestTextEvents(t *testing.T) {
	t.Parallel()
	ta := newEventAdapter(t)
	c := ta.c
	// A field that already holds text, published without events, since a field gaining an interface is not something
	// AT-SPI has an event for.
	before := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textBefore, SelStart: 4, SelEnd: 4}
	})
	ta.Publish(mainWindow, before, nil, sampleGeometry())

	// Typing a character at the end of the field.
	typed := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 5, SelEnd: 5}
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
		tree.Node(4).Text = &accessibility.TextInfo{Text: textAfter, SelStart: 0, SelEnd: 5}
	})
	ta.Publish(mainWindow, selected, accessibility.Diff(typed, selected), sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextCaretMoved, "", 5, 0, variantInt32(0)),
		objectEvent(4, signalTextSelectionChanged, "", 0, 0, variantString("")),
	}, ta.peer.nextSignals(2))

	// Deleting the selection.
	emptied := activeMainTree(func(tree *accessibility.Tree) {
		tree.Node(4).Text = &accessibility.TextInfo{}
	})
	events = accessibility.Diff(selected, emptied)
	c.Equal([]accessibility.Event{
		{Kind: accessibility.TextDeleted, Node: 4, Start: 0, Length: 5, Old: textAfter},
		{Kind: accessibility.TextSelectionChanged, Node: 4},
	}, events)
	ta.Publish(mainWindow, emptied, events, sampleGeometry())
	c.Equal([]signalRecord{
		objectEvent(4, signalTextChanged, detailDelete, 0, 5, variantString(textAfter)),
		objectEvent(4, signalTextCaretMoved, "", 0, 0, variantInt32(0)),
	}, ta.peer.nextSignals(2))
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

	// An announcement that arrives as an event, which only a caller that builds its own events can produce, is the same
	// signal.
	ta.Publish(mainWindow, mainTree(), []accessibility.Event{{
		Kind: accessibility.Announcement,
		New:  "From an event",
	}}, sampleGeometry())
	c.Equal(eventBody("", livePolite, 0, variantString("From an event")), ta.peer.nextSignal().args)
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
