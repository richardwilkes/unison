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
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

const (
	// testToolkitVersion is the version the adapter under test reports as both its own and the toolkit's.
	testToolkitVersion = "1.2.3"
	// mainWindow and otherWindow are the two windows the adapter tests publish.
	mainWindow  WindowKey = 1
	otherWindow WindowKey = 2
	// testShortcut is the accelerator the slider carries, and testKeyBinding is the AT-SPI key binding triple that
	// org.a11y.atspi.Action reports it as.
	testShortcut   = "Ctrl+V"
	testKeyBinding = ";;" + testShortcut
	// testLabelName is what the main window's label says, and testFieldValue is what the field beside it holds.
	testLabelName  = "Name:"
	testFieldValue = "Fred"
)

// mainTree is the window the adapter tests work over:
//
//	1 window "Test Window"          (0,0 200x150)   active
//	├─ 2 group        [ignored]      (0,0 200x60)
//	│  ├─ 3 label "Name:"            (10,10 40x20)
//	│  └─ 4 text field "Fred"        (60,10 100x20)  focused, labeled by 3, controls 5
//	├─ 5 list         [multi-select] (0,60 200x60)
//	│  ├─ 6 list item "One"          (0,60 200x20)   selected
//	│  └─ 7 list item "Two"          (0,80 200x20)
//	├─ 8 slider "Volume"             (0,120 200x20)  0..10, at 4
//	└─ 9 progress bar "Progress"     (0,140 200x10)  0..1, at 0.5
func mainTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Test Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 150),
			Children: []accessibility.NodeID{2, 5, 8, 9},
		},
		&accessibility.Node{
			ID: 2, Parent: 1, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 60),
			Children: []accessibility.NodeID{3, 4},
		},
		&accessibility.Node{
			ID: 3, Parent: 2, Role: role.Label, Name: testLabelName, Bounds: geom.NewRect(10, 10, 40, 20),
		},
		&accessibility.Node{
			ID: 4, Parent: 2, Role: role.TextField, Value: testFieldValue, Placeholder: "Your name", Focusable: true,
			Focused: true, Bounds: geom.NewRect(60, 10, 100, 20), LabeledBy: []accessibility.NodeID{3},
			Controls: []accessibility.NodeID{5}, Actions: accessibility.ActionSet(0).With(accessibility.Focus),
		},
		&accessibility.Node{
			ID: 5, Parent: 1, Role: role.List, Name: "Items", Multiselectable: true,
			Bounds: geom.NewRect(0, 60, 200, 60), Children: []accessibility.NodeID{6, 7},
		},
		&accessibility.Node{
			ID: 6, Parent: 5, Role: role.ListItem, Name: "One", Selectable: true, Selected: true,
			Bounds:  geom.NewRect(0, 60, 200, 20),
			Actions: selectionActions(),
		},
		&accessibility.Node{
			ID: 7, Parent: 5, Role: role.ListItem, Name: "Two", Selectable: true,
			Bounds:  geom.NewRect(0, 80, 200, 20),
			Actions: selectionActions(),
		},
		&accessibility.Node{
			ID: 8, Parent: 1, Role: role.Slider, Name: "Volume", Value: "4", Shortcut: testShortcut,
			Focusable: true, HasNumber: true, Number: 4, Min: 0, Max: 10, Step: 1,
			Orientation: accessibility.OrientationHorizontal, Bounds: geom.NewRect(0, 120, 200, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Increment,
				accessibility.Decrement, accessibility.SetValue, accessibility.ShowContextMenu,
				accessibility.ScrollIntoView, accessibility.Focus),
		},
		&accessibility.Node{
			ID: 9, Parent: 1, Role: role.ProgressBar, Name: "Progress", Value: "50%", HasNumber: true, Number: 0.5,
			Min: 0, Max: 1, Bounds: geom.NewRect(0, 140, 200, 10),
		},
	)
}

// otherTree is a second window, whose list allows only one selection at a time:
//
//	20 dialog "Pick One"            (0,0 100x60)
//	└─ 21 list                      (0,0 100x60)
//	   └─ 22 list item "Only"       (0,0 100x20)
func otherTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 20, Role: role.Dialog, Name: "Pick One", Bounds: geom.NewRect(0, 0, 100, 60),
			Children: []accessibility.NodeID{21},
		},
		&accessibility.Node{
			ID: 21, Parent: 20, Role: role.List, Bounds: geom.NewRect(0, 0, 100, 60),
			Children: []accessibility.NodeID{22},
		},
		&accessibility.Node{
			ID: 22, Parent: 21, Role: role.ListItem, Name: "Only", Selectable: true,
			Bounds:  geom.NewRect(0, 0, 100, 20),
			Actions: selectionActions(),
		},
	)
}

// selectionActions is what a row that can be selected, added to a selection and taken out of one supports.
func selectionActions() accessibility.ActionSet {
	return accessibility.ActionSet(0).With(accessibility.Select, accessibility.AddToSelection,
		accessibility.RemoveFromSelection)
}

// testAdapter is an adapter whose peer is a fake registry, along with the requests it has handed on.
type testAdapter struct {
	*Adapter
	peer     *testPeer
	c        check.Checker
	requests chan accessibility.ActionRequest
}

// newTestAdapter starts an adapter against a fake registry and publishes the main window.
func newTestAdapter(t *testing.T) *testAdapter {
	t.Helper()
	c := check.New(t)
	p := newTestPeer(t, registryAnswers)
	requests := make(chan accessibility.ActionRequest, 16)
	a, err := Start(Config{
		Action:         func(req accessibility.ActionRequest) { requests <- req },
		ToolkitVersion: testToolkitVersion,
		conn:           p.client,
	})
	c.NoError(err)
	ta := &testAdapter{Adapter: a, peer: p, c: c, requests: requests}
	ta.embedCall()
	a.Publish(mainWindow, mainTree(), nil, sampleGeometry())
	return ta
}

// embedCall returns the Embed call the adapter made when it started, checking that it named the application root.
func (ta *testAdapter) embedCall() *dbus.Message {
	msg := ta.peer.nextCall()
	ta.c.Equal(RegistryDestination, msg.Destination)
	ta.c.Equal(RootPath, msg.Path)
	ta.c.Equal(InterfaceSocket, msg.Interface)
	ta.c.Equal("Embed", msg.Member)
	args, err := msg.Args()
	ta.c.NoError(err)
	ta.c.Equal([]any{rootRef()}, args)
	return msg
}

// nextRequest returns the next request the adapter handed to the action callback.
func (ta *testAdapter) nextRequest(t *testing.T) accessibility.ActionRequest {
	t.Helper()
	select {
	case req := <-ta.requests:
		return req
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for an action request")
		return accessibility.ActionRequest{}
	}
}

// noRequest fails the test if the adapter handed anything to the action callback.
func (ta *testAdapter) noRequest(t *testing.T) {
	t.Helper()
	select {
	case req := <-ta.requests:
		t.Fatalf("nothing should have been requested, but %v of node %d was", req.Action, req.Node)
	default:
	}
}

// values makes a call on one of the adapter's objects and returns the values of the reply.
func (ta *testAdapter) values(path dbus.ObjectPath, iface, member string, sig dbus.Signature,
	args ...any,
) []any {
	return ta.peer.replyValues(ta.peer.call(path, iface, member, sig, args...))
}

// one makes a call that replies with a single value and returns it.
func (ta *testAdapter) one(path dbus.ObjectPath, iface, member string, sig dbus.Signature, args ...any) any {
	values := ta.values(path, iface, member, sig, args...)
	ta.c.Equal(1, len(values), "%s.%s must reply with one value", iface, member)
	return values[0]
}

// errorName makes a call that is expected to fail and returns the name of the error it failed with.
func (ta *testAdapter) errorName(path dbus.ObjectPath, iface, member string, sig dbus.Signature,
	args ...any,
) string {
	reply := ta.peer.call(path, iface, member, sig, args...)
	ta.c.Equal(dbus.TypeError, reply.Type, "the call should have failed, but got %s", reply)
	return reply.ErrorName
}

// rootRef is the reference to the application root as the fake bus's name makes it.
func rootRef() dbus.ObjectRef {
	return dbus.ObjectRef{Name: testBusName, Path: RootPath}
}

// nodeRef is the reference to one node as the fake bus's name makes it.
func nodeRef(id accessibility.NodeID) dbus.ObjectRef {
	return dbus.ObjectRef{Name: testBusName, Path: NodePath(id)}
}

// desktopRef is the reference the fake registry hands back from Embed.
func desktopRef() dbus.ObjectRef {
	return dbus.ObjectRef{Name: testPeerName, Path: testDesktopPath}
}

func TestStartExportsTheApplication(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(desktopRef(), ta.peer.getProperty(RootPath, InterfaceAccessible, "Parent"),
		"the desktop the registry handed back becomes the application's parent")
	c.Equal(xos.AppName, ta.peer.getProperty(RootPath, InterfaceAccessible, "Name"))
	c.Equal(int32(1), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"))
	c.Equal([]dbus.ObjectRef{nodeRef(1)}, ta.one(RootPath, InterfaceAccessible, "GetChildren", ""))
	c.Equal(nodeRef(1), ta.one(RootPath, InterfaceAccessible, "GetChildAtIndex", "i", int32(0)))
	c.Equal(nullReference(), ta.one(RootPath, InterfaceAccessible, "GetChildAtIndex", "i", int32(1)))
	c.Equal(int32(-1), ta.one(RootPath, InterfaceAccessible, "GetIndexInParent", ""))
	c.Equal(uint32(RoleApplication), ta.one(RootPath, InterfaceAccessible, "GetRole", ""))
	c.Equal("application", ta.one(RootPath, InterfaceAccessible, "GetRoleName", ""))
	c.Equal("application", ta.one(RootPath, InterfaceAccessible, "GetLocalizedRoleName", ""))
	c.Equal(rootRef(), ta.one(RootPath, InterfaceAccessible, "GetApplication", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceApplication},
		ta.one(RootPath, InterfaceAccessible, "GetInterfaces", ""))
	c.Equal([]any{}, ta.one(RootPath, InterfaceAccessible, "GetRelationSet", ""))
	c.Equal(dbus.Dict{{Key: toolkitAttribute, Value: toolkitName}},
		ta.one(RootPath, InterfaceAccessible, "GetAttributes", ""))
	states, ok := ta.one(RootPath, InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	c.Equal(rootStates().Words(), states)
}

func TestApplicationInterface(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(toolkitName, ta.peer.getProperty(RootPath, InterfaceApplication, "ToolkitName"))
	c.Equal(testToolkitVersion, ta.peer.getProperty(RootPath, InterfaceApplication, "Version"))
	c.Equal(testToolkitVersion, ta.peer.getProperty(RootPath, InterfaceApplication, "ToolkitVersion"))
	c.Equal(atspiVersion, ta.peer.getProperty(RootPath, InterfaceApplication, "AtspiVersion"))

	// The registry sets the application's id as soon as it has been embedded, and expects to be able to read it back.
	c.Equal(int32(0), ta.peer.getProperty(RootPath, InterfaceApplication, "Id"))
	reply := ta.peer.setProperty(RootPath, InterfaceApplication, "Id", dbus.Variant{Sig: "i", Value: int32(17)})
	c.Equal(dbus.TypeMethodReturn, reply.Type, "unexpected reply: %s", reply)
	c.Equal(int32(17), ta.peer.getProperty(RootPath, InterfaceApplication, "Id"))

	c.Equal(currentLocale(), ta.one(RootPath, InterfaceApplication, "GetLocale", "u", uint32(2)))
	// libatspi asks every application it sees for a private bus of its own, and an empty string is how at-spi2-atk and
	// GTK both say there is none. Answering with an error instead would turn a routine probe into a failed call and a
	// logged warning for every Unison process on the desktop.
	c.Equal("", ta.one(RootPath, InterfaceApplication, "GetApplicationBusAddress", ""))
}

func TestNodeAccessible(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal("Test Window", ta.peer.getProperty(NodePath(1), InterfaceAccessible, "Name"))
	c.Equal(testLabelName, ta.peer.getProperty(NodePath(3), InterfaceAccessible, "Name"))
	c.Equal(rootRef(), ta.peer.getProperty(NodePath(1), InterfaceAccessible, "Parent"),
		"a window's parent is the application")
	c.Equal(nodeRef(1), ta.peer.getProperty(NodePath(4), InterfaceAccessible, "Parent"),
		"the ignored group between them is passed over")
	c.Equal(nodeRef(5), ta.peer.getProperty(NodePath(6), InterfaceAccessible, "Parent"))
	c.Equal(int32(5), ta.peer.getProperty(NodePath(1), InterfaceAccessible, "ChildCount"))
	c.Equal(int32(0), ta.peer.getProperty(NodePath(3), InterfaceAccessible, "ChildCount"))
	c.Equal("4", ta.peer.getProperty(NodePath(4), InterfaceAccessible, "AccessibleId"))
	c.Equal("", ta.peer.getProperty(NodePath(4), InterfaceAccessible, "HelpText"))
	c.Equal(currentLocale(), ta.peer.getProperty(NodePath(4), InterfaceAccessible, "Locale"))

	c.Equal([]dbus.ObjectRef{nodeRef(3), nodeRef(4), nodeRef(5), nodeRef(8), nodeRef(9)},
		ta.one(NodePath(1), InterfaceAccessible, "GetChildren", ""),
		"the ignored group is replaced by its own children")
	c.Equal(nodeRef(4), ta.one(NodePath(1), InterfaceAccessible, "GetChildAtIndex", "i", int32(1)))
	c.Equal(nullReference(), ta.one(NodePath(1), InterfaceAccessible, "GetChildAtIndex", "i", int32(99)))
	c.Equal(nullReference(), ta.one(NodePath(1), InterfaceAccessible, "GetChildAtIndex", "i", int32(-1)))
	c.Equal(int32(1), ta.one(NodePath(4), InterfaceAccessible, "GetIndexInParent", ""))
	c.Equal(int32(0), ta.one(NodePath(1), InterfaceAccessible, "GetIndexInParent", ""),
		"a window reports where it sits among the application's windows")

	c.Equal(uint32(RoleEntry), ta.one(NodePath(4), InterfaceAccessible, "GetRole", ""))
	c.Equal("entry", ta.one(NodePath(4), InterfaceAccessible, "GetRoleName", ""))
	c.Equal(uint32(RoleFrame), ta.one(NodePath(1), InterfaceAccessible, "GetRole", ""))
	c.Equal(rootRef(), ta.one(NodePath(6), InterfaceAccessible, "GetApplication", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceAction, InterfaceComponent, InterfaceValue},
		ta.one(NodePath(8), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: placeholderTextAttribute, Value: "Your name"},
	}, ta.one(NodePath(4), InterfaceAccessible, "GetAttributes", ""))
}

func TestNodeState(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	states, ok := ta.one(NodePath(4), InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	c.Equal(2, len(states), "a state set is always two words")
	expected := States(mainTree().Node(4), true, false)
	c.Equal(expected.Words(), states)
	var set StateSet
	set[0], set[1] = states[0], states[1]
	c.True(set.Has(StateFocused), "the focused field of the active window is focused")
	c.False(set.Has(StateEditable), "a field with no text yet has no text states to claim")
	c.False(set.Has(StateSingleLine))

	// The states that describe text arrive with the text itself, which is also when the object gains the
	// org.a11y.atspi.Text interface an assistive technology would read it through.
	typed := mainTree()
	typed.Generation++
	typed.Node(4).Text = &accessibility.TextInfo{Text: testFieldValue, SelStart: 4, SelEnd: 4}
	ta.Publish(mainWindow, typed, nil, sampleGeometry())
	states, ok = ta.one(NodePath(4), InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	set[0], set[1] = states[0], states[1]
	c.True(set.Has(StateEditable))
	c.True(set.Has(StateSingleLine))
	// The interface that does the editing arrives with the state, which is what one predicate for the two is for. This
	// field offers nothing but Focus — the snapshot strips the actions of a field that cannot be used — so what it
	// hands back is a refusal rather than no interface at all, exactly as an insensitive GtkEntry does.
	advertised, ok := ta.one(NodePath(4), InterfaceAccessible, "GetInterfaces", "").([]string)
	c.True(ok)
	c.True(slices.Contains(advertised, InterfaceEditableText), "an editable field has an interface to edit it through")
	c.Equal(false, ta.one(NodePath(4), InterfaceEditableText, "DeleteText", "ii", int32(0), int32(1)))
	ta.noRequest(t)

	// The same field in a window that is not the active one is focusable but not focused.
	ta.Publish(otherWindow, otherTree(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	dialogStates, ok := ta.one(NodePath(20), InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	set[0], set[1] = dialogStates[0], dialogStates[1]
	c.False(set.Has(StateActive), "the second window was not published as the active one")
}

func TestNodeRelationSet(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal([]any{
		dbus.Struct{uint32(RelationLabelledBy), []dbus.ObjectRef{nodeRef(3)}},
		dbus.Struct{uint32(RelationControllerFor), []dbus.ObjectRef{nodeRef(5)}},
	}, ta.one(NodePath(4), InterfaceAccessible, "GetRelationSet", ""))
	c.Equal([]any{dbus.Struct{uint32(RelationLabelFor), []dbus.ObjectRef{nodeRef(4)}}},
		ta.one(NodePath(3), InterfaceAccessible, "GetRelationSet", ""),
		"the label is told what it labels")
	c.Equal([]any{dbus.Struct{uint32(RelationControlledBy), []dbus.ObjectRef{nodeRef(4)}}},
		ta.one(NodePath(5), InterfaceAccessible, "GetRelationSet", ""))
	c.Equal([]any{}, ta.one(NodePath(6), InterfaceAccessible, "GetRelationSet", ""))
}

func TestPathsThatHaveNoObject(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	for _, path := range []dbus.ObjectPath{
		NodePath(2),   // An ignored node is not reported at all
		NodePath(404), // A node that was never published
		NullPath,
		"/org/a11y/atspi/accessible/nonsense",
	} {
		c.Equal(dbus.UnknownObject, ta.errorName(path, InterfaceAccessible, "GetRole", ""),
			"%s must have no object", path)
	}
}

func TestComponentExtents(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(dbus.Struct{int32(220), int32(70), int32(200), int32(40)},
		ta.one(NodePath(4), InterfaceComponent, "GetExtents", "u", uint32(CoordScreen)))
	c.Equal(dbus.Struct{int32(120), int32(20), int32(200), int32(40)},
		ta.one(NodePath(4), InterfaceComponent, "GetExtents", "u", uint32(CoordWindow)))
	c.Equal(dbus.Struct{int32(0), int32(0), int32(400), int32(40)},
		ta.one(NodePath(6), InterfaceComponent, "GetExtents", "u", uint32(CoordParent)))
	c.Equal([]any{int32(220), int32(70)},
		ta.values(NodePath(4), InterfaceComponent, "GetPosition", "u", uint32(CoordScreen)))
	c.Equal([]any{int32(120), int32(20)},
		ta.values(NodePath(4), InterfaceComponent, "GetPosition", "u", uint32(CoordWindow)))
	c.Equal([]any{int32(200), int32(40)}, ta.values(NodePath(4), InterfaceComponent, "GetSize", ""))

	// A window that has moved reports its nodes somewhere else on the screen, without being published again.
	ta.SetGeometry(mainWindow, Geometry{Origin: geom.NewPoint(0, 0), Scale: geom.NewPoint(1, 1)})
	c.Equal(dbus.Struct{int32(60), int32(10), int32(100), int32(20)},
		ta.one(NodePath(4), InterfaceComponent, "GetExtents", "u", uint32(CoordScreen)))
	ta.SetGeometry(otherWindow, sampleGeometry()) // A window that is not there changes nothing
}

func TestComponentMiscellany(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(uint32(LayerWindow), ta.one(NodePath(1), InterfaceComponent, "GetLayer", ""))
	c.Equal(uint32(LayerWidget), ta.one(NodePath(4), InterfaceComponent, "GetLayer", ""))
	c.Equal(int16(0), ta.one(NodePath(4), InterfaceComponent, "GetMDIZOrder", ""))
	c.Equal(1.0, ta.one(NodePath(4), InterfaceComponent, "GetAlpha", ""))
	c.Equal(true, ta.one(NodePath(4), InterfaceComponent, "Contains", "iiu", int32(220), int32(70),
		uint32(CoordScreen)))
	c.Equal(false, ta.one(NodePath(4), InterfaceComponent, "Contains", "iiu", int32(0), int32(0),
		uint32(CoordScreen)))
	for _, member := range []string{"SetExtents", "SetPosition", "SetSize", "ScrollToPoint"} {
		var sig dbus.Signature
		var args []any
		switch member {
		case "SetExtents":
			sig, args = "iiiiu", []any{int32(0), int32(0), int32(10), int32(10), uint32(CoordScreen)}
		case "SetPosition":
			sig, args = "iiu", []any{int32(0), int32(0), uint32(CoordScreen)}
		case "SetSize":
			sig, args = "ii", []any{int32(10), int32(10)}
		default:
			sig, args = "uii", []any{uint32(CoordScreen), int32(0), int32(0)}
		}
		c.Equal(false, ta.one(NodePath(4), InterfaceComponent, member, sig, args...),
			"%s must report that it did nothing", member)
	}
}

func TestComponentAccessibleAtPoint(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	// The middle of the first row, in each of the three coordinate spaces.
	c.Equal(nodeRef(6), ta.one(NodePath(1), InterfaceComponent, "GetAccessibleAtPoint", "iiu", int32(300),
		int32(190), uint32(CoordScreen)))
	c.Equal(nodeRef(6), ta.one(NodePath(1), InterfaceComponent, "GetAccessibleAtPoint", "iiu", int32(200),
		int32(140), uint32(CoordWindow)))
	c.Equal(nodeRef(6), ta.one(NodePath(5), InterfaceComponent, "GetAccessibleAtPoint", "iiu", int32(200),
		int32(140), uint32(CoordParent)), "the list's own coordinates are relative to its parent, the window")
	c.Equal(nullReference(), ta.one(NodePath(6), InterfaceComponent, "GetAccessibleAtPoint", "iiu", int32(300),
		int32(190), uint32(CoordScreen)), "a row has nothing inside it")
	c.Equal(nullReference(), ta.one(NodePath(5), InterfaceComponent, "GetAccessibleAtPoint", "iiu", int32(220),
		int32(70), uint32(CoordScreen)), "the text field is not inside the list")
	c.Equal(nullReference(), ta.one(NodePath(1), InterfaceComponent, "GetAccessibleAtPoint", "iiu", int32(-1),
		int32(-1), uint32(CoordScreen)))
}

func TestComponentGrabFocusAndScrollTo(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(true, ta.one(NodePath(8), InterfaceComponent, "GrabFocus", ""))
	c.Equal(accessibility.ActionRequest{Node: 8, Action: accessibility.Focus}, ta.nextRequest(t))
	c.Equal(false, ta.one(NodePath(3), InterfaceComponent, "GrabFocus", ""), "a label cannot take the focus")
	ta.noRequest(t)
	c.Equal(true, ta.one(NodePath(8), InterfaceComponent, "ScrollTo", "u", uint32(0)))
	c.Equal(accessibility.ActionRequest{Node: 8, Action: accessibility.ScrollIntoView}, ta.nextRequest(t))
	c.Equal(false, ta.one(NodePath(4), InterfaceComponent, "ScrollTo", "u", uint32(0)),
		"a node that cannot be scrolled into view says so")
	ta.noRequest(t)
}

func TestActionInterface(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(int32(4), ta.peer.getProperty(NodePath(8), InterfaceAction, "NActions"))
	// A key binding is the semicolon-separated triple mnemonic;full-shortcut;accelerator that every AT-SPI producer
	// hands back, and a Unison shortcut is only ever the accelerator. Orca splits the string on the semicolons, so a
	// bare "Ctrl+V" would be taken for a mnemonic path rather than for the accelerator it is.
	c.Equal([]any{
		dbus.Struct{"click", "", testKeyBinding},
		dbus.Struct{"increment", "", ""},
		dbus.Struct{"decrement", "", ""},
		dbus.Struct{"menu", "", ""},
	}, ta.one(NodePath(8), InterfaceAction, "GetActions", ""))
	c.Equal("click", ta.one(NodePath(8), InterfaceAction, "GetName", "i", int32(0)))
	c.Equal("click", ta.one(NodePath(8), InterfaceAction, "GetLocalizedName", "i", int32(0)))
	c.Equal("", ta.one(NodePath(8), InterfaceAction, "GetDescription", "i", int32(0)))
	c.Equal(testKeyBinding, ta.one(NodePath(8), InterfaceAction, "GetKeyBinding", "i", int32(0)))
	c.Equal("", ta.one(NodePath(8), InterfaceAction, "GetKeyBinding", "i", int32(1)))
	c.Equal("menu", ta.one(NodePath(8), InterfaceAction, "GetName", "i", int32(3)))
	c.Equal("", keyBinding(""), "a node with no shortcut reports nothing rather than two bare separators")

	c.Equal(true, ta.one(NodePath(8), InterfaceAction, "DoAction", "i", int32(1)))
	c.Equal(accessibility.ActionRequest{Node: 8, Action: accessibility.Increment}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(8), InterfaceAction, "DoAction", "i", int32(0)))
	c.Equal(accessibility.ActionRequest{Node: 8, Action: accessibility.Press}, ta.nextRequest(t))

	for _, index := range []int32{-1, 4, 99} {
		c.Equal(dbus.InvalidArgs, ta.errorName(NodePath(8), InterfaceAction, "DoAction", "i", index),
			"action %d is out of range", index)
	}
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(3), InterfaceAction, "DoAction", "i", int32(0)),
		"a label has nothing to do")
}

func TestValueInterface(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(0.0, ta.peer.getProperty(NodePath(8), InterfaceValue, "MinimumValue"))
	c.Equal(10.0, ta.peer.getProperty(NodePath(8), InterfaceValue, "MaximumValue"))
	c.Equal(1.0, ta.peer.getProperty(NodePath(8), InterfaceValue, "MinimumIncrement"))
	c.Equal(4.0, ta.peer.getProperty(NodePath(8), InterfaceValue, "CurrentValue"))
	c.Equal("4", ta.peer.getProperty(NodePath(8), InterfaceValue, "Text"))

	reply := ta.peer.setProperty(NodePath(8), InterfaceValue, "CurrentValue", dbus.Variant{Sig: "d", Value: 7.5})
	c.Equal(dbus.TypeMethodReturn, reply.Type, "unexpected reply: %s", reply)
	c.Equal(accessibility.ActionRequest{Node: 8, Action: accessibility.SetValue, Number: 7.5}, ta.nextRequest(t))
	c.Equal(4.0, ta.peer.getProperty(NodePath(8), InterfaceValue, "CurrentValue"),
		"the published value only changes when the next snapshot arrives")

	// A progress bar has a value, but nothing can be done about it, which it says by having no setter at all rather
	// than by taking the write and then refusing it.
	c.Equal(0.5, ta.peer.getProperty(NodePath(9), InterfaceValue, "CurrentValue"))
	reply = ta.peer.setProperty(NodePath(9), InterfaceValue, "CurrentValue", dbus.Variant{Sig: "d", Value: 0.9})
	c.Equal(dbus.TypeError, reply.Type, "unexpected reply: %s", reply)
	c.Equal(dbus.PropertyReadOnly, reply.ErrorName)
	ta.noRequest(t)
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(3), dbusPropertiesInterface, "Get", "ss", InterfaceValue,
		"CurrentValue"), "a label has no value at all")
}

func TestSelectionInterface(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.Equal(int32(1), ta.peer.getProperty(NodePath(5), InterfaceSelection, "NSelectedChildren"))
	c.Equal(nodeRef(6), ta.one(NodePath(5), InterfaceSelection, "GetSelectedChild", "i", int32(0)))
	c.Equal(nullReference(), ta.one(NodePath(5), InterfaceSelection, "GetSelectedChild", "i", int32(1)))
	c.Equal(true, ta.one(NodePath(5), InterfaceSelection, "IsChildSelected", "i", int32(0)))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "IsChildSelected", "i", int32(1)))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "IsChildSelected", "i", int32(99)))

	// The list allows more than one selection, so asking for another child adds it.
	c.Equal(true, ta.one(NodePath(5), InterfaceSelection, "SelectChild", "i", int32(1)))
	c.Equal(accessibility.ActionRequest{Node: 7, Action: accessibility.AddToSelection}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(5), InterfaceSelection, "DeselectChild", "i", int32(0)))
	c.Equal(accessibility.ActionRequest{Node: 6, Action: accessibility.RemoveFromSelection}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(5), InterfaceSelection, "DeselectSelectedChild", "i", int32(0)))
	c.Equal(accessibility.ActionRequest{Node: 6, Action: accessibility.RemoveFromSelection}, ta.nextRequest(t))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "SelectChild", "i", int32(99)))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "DeselectChild", "i", int32(99)))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "DeselectSelectedChild", "i", int32(99)))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "SelectAll", ""))
	c.Equal(false, ta.one(NodePath(5), InterfaceSelection, "ClearSelection", ""))
	ta.noRequest(t)

	// A list that allows only one selection at a time replaces it instead.
	ta.Publish(otherWindow, otherTree(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	c.Equal(int32(0), ta.peer.getProperty(NodePath(21), InterfaceSelection, "NSelectedChildren"))
	c.Equal(true, ta.one(NodePath(21), InterfaceSelection, "SelectChild", "i", int32(0)))
	c.Equal(accessibility.ActionRequest{Node: 22, Action: accessibility.Select}, ta.nextRequest(t))
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(6), InterfaceSelection, "SelectAll", ""),
		"a row is not a container of selectable things")
}

func TestCacheGetItems(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	items, ok := ta.one(CachePath, InterfaceCache, "GetItems", "").([]any)
	c.True(ok)
	// The application root plus every reported node of the one published window: the ignored group is left out.
	c.Equal(9, len(items))
	c.Equal(dbus.Struct{
		rootRef(), rootRef(), desktopRef(), int32(-1), int32(1),
		[]string{InterfaceAccessible, InterfaceApplication},
		xos.AppName, uint32(RoleApplication), "",
		rootStates().Words(),
	}, items[0])
	c.Equal(dbus.Struct{
		nodeRef(1), rootRef(), rootRef(), int32(0), int32(5),
		[]string{InterfaceAccessible, InterfaceComponent},
		"Test Window", uint32(RoleFrame), "",
		States(mainTree().Node(1), true, true).Words(),
	}, items[1], "the window itself comes first, and its parent is the application")
	c.Equal(dbus.Struct{
		nodeRef(4), rootRef(), nodeRef(1), int32(1), int32(0),
		// The field carries no text of its own yet, but its value is textual, so what it hands over is the read-only
		// text synthesized from that value.
		[]string{InterfaceAccessible, InterfaceComponent, InterfaceText},
		"", uint32(RoleEntry), "",
		States(mainTree().Node(4), true, false).Words(),
	}, items[3], "the text field's parent is the window, since the group between them is ignored")

	// A second window adds its own nodes, and the root reports one more child.
	ta.Publish(otherWindow, otherTree(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	items, ok = ta.one(CachePath, InterfaceCache, "GetItems", "").([]any)
	c.True(ok)
	c.Equal(12, len(items))
	first, ok := items[0].(dbus.Struct)
	c.True(ok)
	c.Equal(int32(2), first[4], "the application now has two windows")
}

func TestIntrospection(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	for _, one := range []struct {
		path     dbus.ObjectPath
		expected []string
	}{
		{
			path: RootPath,
			expected: []string{
				InterfaceAccessible, InterfaceApplication, "GetApplicationBusAddress",
				"org.freedesktop.DBus.Introspectable", dbusPropertiesInterface, "org.freedesktop.DBus.Peer",
			},
		},
		{
			path: CachePath,
			expected: []string{
				InterfaceCache, "GetItems", "AddAccessible", "RemoveAccessible",
				`<arg type="a((so)(so)(so)iiassusau)" direction="out"/>`,
			},
		},
		{
			path: NodePath(8),
			expected: []string{
				InterfaceAccessible, InterfaceAction, InterfaceComponent, InterfaceValue,
				`<property name="CurrentValue" type="d" access="readwrite"/>`,
				`<method name="GetExtents">`,
			},
		},
		{
			path:     NodePath(5),
			expected: []string{InterfaceSelection, "NSelectedChildren"},
		},
	} {
		xml, ok := ta.one(one.path, "org.freedesktop.DBus.Introspectable", "Introspect", "").(string)
		c.True(ok)
		for _, want := range one.expected {
			c.True(strings.Contains(xml, want), "the introspection of %s must mention %s", one.path, want)
		}
	}
}

// TestTheObjectsAgreeWithWhatIsAdvertised walks every node of every tree the package's tests build, since the
// membership rules of the interfaces that come and go are the ones most likely to drift: the main window covers
// Accessible, Action, Component, Selection, Text and Value, the table window covers Table and TableCell, and the text
// window covers EditableText along with the text states.
func TestTheObjectsAgreeWithWhatIsAdvertised(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(tableWindow, tableTree(), nil, sampleGeometry())
	ta.Publish(textWindow, textTree(), nil, sampleGeometry())
	for _, tree := range []*accessibility.Tree{mainTree(), tableTree(), textTree()} {
		tree.Walk(func(n *accessibility.Node) bool {
			if n.Ignored {
				return true
			}
			advertised, ok := ta.one(NodePath(n.ID), InterfaceAccessible, "GetInterfaces", "").([]string)
			c.True(ok)
			xml, ok := ta.one(NodePath(n.ID), "org.freedesktop.DBus.Introspectable", "Introspect", "").(string)
			c.True(ok)
			// Whatever a node says it implements must actually be there, in the same order, so that a client that walks
			// the introspection and one that asks the shorter question see the same object.
			c.Equal(advertised, atspiInterfacesIn(xml), "node %d advertises interfaces it does not implement", n.ID)
			return true
		})
	}
}

func TestPublishReplacesTheSnapshot(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	// The same window again, with the second row gone and the first one renamed.
	tree := mainTree()
	list := tree.Node(5)
	list.Children = []accessibility.NodeID{6}
	delete(tree.Nodes, 7)
	tree.Node(6).Name = "Only"
	tree.Generation = 2
	ta.Publish(mainWindow, tree, nil, sampleGeometry())

	c.Equal("Only", ta.peer.getProperty(NodePath(6), InterfaceAccessible, "Name"))
	c.Equal(int32(1), ta.peer.getProperty(NodePath(5), InterfaceAccessible, "ChildCount"))
	c.Equal(dbus.UnknownObject, ta.errorName(NodePath(7), InterfaceAccessible, "GetRole", ""),
		"a node that has left the tree has no object")
	c.Equal(int32(1), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"),
		"publishing a window again does not add it twice")

	ta.Publish(mainWindow, nil, nil, sampleGeometry()) // Publishing nothing changes nothing
	c.Equal("Only", ta.peer.getProperty(NodePath(6), InterfaceAccessible, "Name"))
}

func TestRemoveWindow(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(otherWindow, otherTree(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	c.Equal(int32(2), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"))
	c.Equal(int32(1), ta.one(NodePath(20), InterfaceAccessible, "GetIndexInParent", ""))

	ta.RemoveWindow(mainWindow)
	c.Equal(int32(1), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"))
	c.Equal([]dbus.ObjectRef{nodeRef(20)}, ta.one(RootPath, InterfaceAccessible, "GetChildren", ""))
	c.Equal(int32(0), ta.one(NodePath(20), InterfaceAccessible, "GetIndexInParent", ""),
		"the window that is left has moved up")
	for _, id := range []accessibility.NodeID{1, 4, 5, 6} {
		c.Equal(dbus.UnknownObject, ta.errorName(NodePath(id), InterfaceAccessible, "GetRole", ""),
			"node %d went away with its window", id)
	}
	ta.RemoveWindow(mainWindow) // Removing a window that is not there changes nothing
	c.Equal(int32(1), ta.peer.getProperty(RootPath, InterfaceAccessible, "ChildCount"))
}

// windowThatTookTheSlider returns the second window's tree with the main window's slider in it, which is what a panel
// reparented from one window into another looks like in the window that now holds it.
func windowThatTookTheSlider() *accessibility.Tree {
	tree := otherTree()
	tree.Node(21).Children = append(tree.Node(21).Children, 8)
	tree.Nodes[8] = &accessibility.Node{
		ID: 8, Parent: 21, Role: role.Slider, Name: "Volume", HasNumber: true, Number: 4, Max: 10,
		Bounds: geom.NewRect(0, 20, 100, 20),
	}
	return tree
}

// mainWindowWithoutTheSlider returns the main window's tree without the slider, which is the snapshot it publishes once
// the node has moved to another window.
func mainWindowWithoutTheSlider() *accessibility.Tree {
	tree := mainTree()
	tree.Generation++
	tree.Node(1).Children = []accessibility.NodeID{2, 5, 9}
	delete(tree.Nodes, 8)
	return tree
}

// TestPublishingAfterANodeHasMovedToAnotherWindow covers a panel being reparented. The window it joined publishes first
// — it is the one whose layout changed — and the window it left publishes a snapshot without it afterwards. Dropping
// the node from the index then would leave its object answering UnknownObject until the window that actually holds it
// published again, which for a window nothing is happening in may be never.
func TestPublishingAfterANodeHasMovedToAnotherWindow(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(otherWindow, windowThatTookTheSlider(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	c.Equal(nodeRef(21), ta.peer.getProperty(NodePath(8), InterfaceAccessible, "Parent"),
		"the slider belongs to the window that took it")

	without := mainWindowWithoutTheSlider()
	ta.Publish(mainWindow, without, accessibility.Diff(mainTree(), without), sampleGeometry())
	c.Equal("Volume", ta.peer.getProperty(NodePath(8), InterfaceAccessible, "Name"),
		"the node goes on answering for the window that holds it now")
	c.Equal(nodeRef(21), ta.peer.getProperty(NodePath(8), InterfaceAccessible, "Parent"))
}

// TestRemovingAWindowANodeHasLeftKeepsTheNode covers the same reparenting the other way round: the window the node came
// from is closed rather than republished, and the node has to go on answering for the window that took it.
func TestRemovingAWindowANodeHasLeftKeepsTheNode(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(otherWindow, windowThatTookTheSlider(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	ta.RemoveWindow(mainWindow)
	c.Equal("Volume", ta.peer.getProperty(NodePath(8), InterfaceAccessible, "Name"),
		"a node that moved on does not go away with the window it came from")
	for _, id := range []accessibility.NodeID{1, 4, 5, 6} {
		c.Equal(dbus.UnknownObject, ta.errorName(NodePath(id), InterfaceAccessible, "GetRole", ""),
			"node %d did go away with its window", id)
	}
}

// TestTheRegistryComingBackRejoinsTheAccessibilityTree covers at-spi2-registryd being restarted, which happens on any
// desktop where the session outlives the assistive technology stack. The connection to the accessibility bus survives
// it, so nothing else reports it: the application is simply no longer among the desktop's children, everything it sends
// afterwards reaches nobody, and the desktop reference it holds names a bus name that no longer exists.
func TestTheRegistryComingBackRejoinsTheAccessibilityTree(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	c.True(slices.Contains(ta.peer.matchRules(), registryMatchRule),
		"the bus has been asked to deliver the signal that says the registry is back")

	// A signal from anything but the bus itself is another peer trying to make the application talk to a registry of
	// its choosing, and a registry losing its name is not one coming back.
	ta.peer.emitFrom(testPeerName, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", RegistryDestination, "",
		":1.99")
	ta.peer.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", RegistryDestination,
		testPeerName, "")
	ta.peer.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", "org.example.Other", "",
		":1.99")
	// A call of the test's own is answered on the same goroutine the signals are handled on, so by the time it comes
	// back every one of them has been dealt with.
	c.Equal(desktopRef(), ta.peer.getProperty(RootPath, InterfaceAccessible, "Parent"))
	ta.peer.noCall()

	// The registry taking its name again is the one thing that means it is back.
	ta.peer.emitFrom(dbusDestination, dbusObjectPath, dbusInterface, nameOwnerChanged, "sss", RegistryDestination, "",
		":1.99")
	msg := ta.peer.nextCall()
	c.Equal(RegistryDestination, msg.Destination)
	c.Equal(InterfaceSocket, msg.Interface)
	c.Equal("Embed", msg.Member)
	args, err := msg.Args()
	c.NoError(err)
	c.Equal([]any{rootRef()}, args, "the application root is what joins the tree again")
	c.Equal(desktopRef(), ta.peer.getProperty(RootPath, InterfaceAccessible, "Parent"),
		"and the desktop the new registry handed back is the application's parent")
}

func TestAnnounceAndStop(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	// An announcement is a signal, which TestAnnouncementsComeFromTheApplication looks at; here it only has to be
	// harmless, as does one with nothing to say.
	ta.Announce("Saved")
	ta.Announce("")

	ta.Stop()
	msg := ta.peer.nextCall()
	c.Equal("Unembed", msg.Member)
	c.Equal(InterfaceSocket, msg.Interface)
	args, err := msg.Args()
	c.NoError(err)
	c.Equal([]any{rootRef()}, args)
}

func TestStartWithoutAnAddress(t *testing.T) {
	clearAccessibilityEnvironment(t)
	c := check.New(t)
	a, err := Start(Config{})
	c.HasError(err, "there is nowhere to connect to")
	c.Nil(a)
}

// TestTheConnectionDyingIsReported covers the one failure nothing else on the desktop announces: the accessibility bus
// going away while the launcher keeps its name on the session bus, which produces no signal for the status watch to
// hear. Everything published afterwards is dropped by the connection without a word, so the adapter has to say that it
// has become useless, or accessibility stays dead for the rest of the process.
func TestTheConnectionDyingIsReported(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	p := newTestPeer(t, registryAnswers)
	lost := make(chan error, 1)
	a, err := Start(Config{
		ToolkitVersion: testToolkitVersion,
		Lost:           func(reason error) { lost <- reason },
		conn:           p.client,
	})
	c.NoError(err)
	c.NoError(a.Err(), "an adapter whose connection is alive has nothing to report")

	xio.CloseIgnoringErrors(p.side) // The accessibility bus goes away
	select {
	case reason := <-lost:
		c.HasError(reason, "the loss is reported with the reason the connection ended")
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the connection loss to be reported")
	}
	c.HasError(a.Err(), "and can be asked about afterwards by anyone that did not want a callback")

	// Everything the root package would go on calling has to remain harmless on an adapter that can no longer reach
	// anyone, since it only learns of the loss when the user interface thread next runs.
	a.Publish(mainWindow, mainTree(), nil, sampleGeometry())
	a.SetGeometry(mainWindow, Geometry{Scale: geom.NewPoint(1, 1)})
	a.Announce("nobody is listening")
	a.RemoveWindow(mainWindow)
	a.Stop()
}

// TestStartRefusesAConnectionThatHasAlreadyGone covers the gap between the Embed reply and the disconnect handler being
// registered. An accessibility bus that dies in it leaves nothing to publish over, and an adapter handed back in that
// state would be installed by the root package and never report anything again, since the loss it would have reported
// happened before the caller held it. Whichever side of the race wins, the caller has to learn of the loss exactly
// once: as the error Start returns, or through the callback.
func TestStartRefusesAConnectionThatHasAlreadyGone(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for range 20 {
		p := newTestPeer(t, func(peer *testPeer, msg *dbus.Message) bool {
			if msg.Interface != InterfaceSocket || msg.Member != embedMember {
				return registryAnswers(peer, msg)
			}
			peer.replyTo(msg, objectRefSignature, desktopRef())
			xio.CloseIgnoringErrors(peer.side) // The bus goes away the moment the application has joined the tree
			return true
		})
		lost := make(chan error, 1)
		a, err := Start(Config{
			ToolkitVersion: testToolkitVersion,
			Lost:           func(reason error) { lost <- reason },
			conn:           p.client,
		})
		if err != nil {
			c.Nil(a, "nothing is handed back when the connection has already gone")
			// The callback belongs to an adapter its owner holds. Reporting a loss for one that was never handed back
			// would have the root package deal with an adapter it has never seen, which is exactly the state the
			// error it is given instead describes.
			select {
			case reason := <-lost:
				t.Fatalf("a start that failed must not report a loss as well, but %v was reported", reason)
			default:
			}
			continue
		}
		// The other side of the race: the handler was registered while the connection was still alive, so the loss is
		// the callback's to report.
		select {
		case reason := <-lost:
			c.HasError(reason, "an adapter that was handed back reports the loss itself")
		case <-time.After(testTimeout):
			t.Fatal("timed out waiting for the connection loss to be reported")
		}
		c.HasError(a.Err())
	}
}

// TestStopIsNotReportedAsALoss verifies that the adapter's own shutdown does not look like the bus dying: the root
// package would otherwise tear down and rebuild an adapter every time a screen reader was switched off.
func TestStopIsNotReportedAsALoss(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	p := newTestPeer(t, registryAnswers)
	lost := make(chan error, 1)
	a, err := Start(Config{
		ToolkitVersion: testToolkitVersion,
		Lost:           func(reason error) { lost <- reason },
		conn:           p.client,
	})
	c.NoError(err)
	c.Equal(embedMember, p.nextCall().Member, "the application joined the accessibility tree")

	// The connection's disconnect callbacks run one after another on a goroutine of their own, so this one is not
	// reached until the adapter's has returned: waiting for it is waiting for the decision about whether the loss is
	// worth reporting. Waiting for Err to report the closure instead would say nothing at all, since Stop sets it
	// before it returns, and a regression that dropped the flag Stop sets would still pass.
	handled := make(chan struct{})
	a.conn.OnDisconnect(func(_ error) { close(handled) })

	a.Stop()
	c.Equal("Unembed", p.nextCall().Member, "the registry is told the application is leaving before it goes")
	select {
	case <-handled:
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the connection to end")
	}
	select {
	case reason := <-lost:
		t.Fatalf("a connection closed by Stop must not be reported as lost, but %v was", reason)
	default:
	}
}
