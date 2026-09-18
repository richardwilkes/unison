// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"math"
	"runtime"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// No unit test can make UI Automation listen to a provider, so these tests replace the Raise* entry points with a
// recorder and check what the adapter would have said: which provider each call names, which event, property or
// structure change it reports, and what type the VARIANTs carry. What the adapter decides to say is settled by the
// portable tests in map_test.go; these check the calls themselves, and the lifetime around them.

// testHWND stands in for the window handle an adapter covers. Nothing is ever sent to it — the one call that names
// it is recorded rather than made — but it has to be non-zero, since Destroy withdraws the provider only for a real
// window.
const testHWND = windows.HWND(0x4242)

// recordedReturnKind and recordedNotifyKind mark the two calls a recorder holds that no Raise stands for: the
// ReturnRawElementProvider that withdraws a window's provider, and the notification an announcement raises, which
// Window.Announce makes directly rather than deciding from a snapshot. Marking them lets a test check where each
// falls among the events and disconnects around it. Both are deliberately values RaiseKind itself never takes.
const (
	recordedReturnKind = RaiseKind(200)
	recordedNotifyKind = RaiseKind(201)
)

// recordedVariant is what a recorder keeps of a VARIANT it was passed. The VARIANT itself belongs to the adapter,
// which clears it as soon as the call returns, so the type tag and the raw value are copied here and a string is
// decoded at once; a SAFEARRAY is recorded for its type alone, since the handle is dead by the time a test reads it.
type recordedVariant struct {
	Str string
	Val uint64
	VT  VARTYPE
}

// Bool returns the contents of a VT_BOOL variant.
func (v recordedVariant) Bool() bool {
	return int16(uint16(v.Val)) == VARIANT_TRUE
}

// Int32 returns the contents of a VT_I4 variant.
func (v recordedVariant) Int32() int32 {
	return int32(uint32(v.Val))
}

// Float64 returns the contents of a VT_R8 variant.
func (v recordedVariant) Float64() float64 {
	return math.Float64frombits(v.Val)
}

// recordedRaise is one call the adapter made into UI Automation. Which fields matter depends on Kind, exactly as for
// the Raise the call came from.
type recordedRaise struct {
	RuntimeID  []int32
	Text       string
	Activity   string
	Provider   unsafe.Pointer
	Old        recordedVariant
	New        recordedVariant
	HWND       windows.HWND
	Event      EventID
	Property   PropertyID
	Change     StructureChangeType
	Kind       RaiseKind
	Notify     NotificationKind
	Processing NotificationProcessing
}

// recorder stands in for UI Automation Core. It holds every call the adapter made, in order, including the
// disconnects and the withdrawal of the window's provider, so that a test can check both what was said and when.
//
// It needs no lock: a publish, an announcement and a destroy all happen on the goroutine that asked for them, which in
// a test is the test's own.
type recorder struct {
	// onEvent, when set, is called from within RaiseAutomationEvent's stand-in, before the call is recorded. It is
	// how a test looks at the adapter's state at the moment it is talking to a client, which is the only way to check
	// the order the parts of a teardown happen in.
	onEvent   func(id EventID)
	raises    []recordedRaise
	listening bool
}

// record installs a recorder in place of the UI Automation entry points for the duration of one test, restoring them
// afterwards. listening is what ClientsAreListening will report, which is the gate every raise sits behind.
//
// The entry points are package-level variables, so a recorder is installed for the whole package while it is in place;
// no test in this package runs in parallel with another.
func record(t *testing.T, listening bool) *recorder {
	t.Helper()
	r := &recorder{listening: listening}
	savedListening := clientsAreListening
	savedEvent := raiseAutomationEvent
	savedProperty := raiseAutomationPropertyChangedEvent
	savedStructure := raiseStructureChangedEvent
	savedNotification := raiseNotificationEvent
	savedDisconnect := disconnectProvider
	savedReturn := returnRawElementProvider
	t.Cleanup(func() {
		clientsAreListening = savedListening
		raiseAutomationEvent = savedEvent
		raiseAutomationPropertyChangedEvent = savedProperty
		raiseStructureChangedEvent = savedStructure
		raiseNotificationEvent = savedNotification
		disconnectProvider = savedDisconnect
		returnRawElementProvider = savedReturn
	})
	clientsAreListening = r.clientsAreListening
	raiseAutomationEvent = r.automationEvent
	raiseAutomationPropertyChangedEvent = r.propertyChanged
	raiseStructureChangedEvent = r.structureChanged
	raiseNotificationEvent = r.notification
	disconnectProvider = r.disconnect
	returnRawElementProvider = r.returnProvider
	return r
}

// clientsAreListening stands in for ClientsAreListening.
func (r *recorder) clientsAreListening() bool {
	return r.listening
}

// automationEvent stands in for RaiseAutomationEvent.
func (r *recorder) automationEvent(provider unsafe.Pointer, id EventID) uintptr {
	if r.onEvent != nil {
		r.onEvent(id)
	}
	r.raises = append(r.raises, recordedRaise{Kind: RaiseEvent, Provider: provider, Event: id})
	return uintptr(w32.COM_S_OK)
}

// propertyChanged stands in for RaiseAutomationPropertyChangedEvent.
func (r *recorder) propertyChanged(provider unsafe.Pointer, id PropertyID, oldValue, newValue VARIANT) uintptr {
	r.raises = append(r.raises, recordedRaise{
		Kind:     RaiseProperty,
		Provider: provider,
		Property: id,
		Old:      recordVariant(oldValue),
		New:      recordVariant(newValue),
	})
	return uintptr(w32.COM_S_OK)
}

// structureChanged stands in for RaiseStructureChangedEvent. The runtime identifier is copied, since the slice the
// adapter passed is its own.
func (r *recorder) structureChanged(provider unsafe.Pointer, change StructureChangeType, runtimeID []int32) uintptr {
	r.raises = append(r.raises, recordedRaise{
		Kind:      RaiseStructure,
		Provider:  provider,
		Change:    change,
		RuntimeID: append([]int32(nil), runtimeID...),
	})
	return uintptr(w32.COM_S_OK)
}

// notification stands in for RaiseNotificationEvent. Both BSTRs belong to the adapter, which frees them as soon as
// this returns, so both are decoded now.
func (r *recorder) notification(provider unsafe.Pointer, kind NotificationKind,
	processing NotificationProcessing, displayString, activityID BSTR,
) uintptr {
	r.raises = append(r.raises, recordedRaise{
		Kind:       recordedNotifyKind,
		Provider:   provider,
		Notify:     kind,
		Processing: processing,
		Text:       BSTRToString(displayString),
		Activity:   BSTRToString(activityID),
	})
	return uintptr(w32.COM_S_OK)
}

// disconnect stands in for DisconnectProvider.
func (r *recorder) disconnect(provider unsafe.Pointer) uintptr {
	r.raises = append(r.raises, recordedRaise{Kind: RaiseDisconnect, Provider: provider})
	return uintptr(w32.COM_S_OK)
}

// returnProvider stands in for ReturnRawElementProvider.
func (r *recorder) returnProvider(hwnd windows.HWND, _ w32.WPARAM, _ w32.LPARAM, provider unsafe.Pointer) w32.LRESULT {
	r.raises = append(r.raises, recordedRaise{Kind: recordedReturnKind, Provider: provider, HWND: hwnd})
	return 0
}

// recordVariant copies what a test can check about a VARIANT before its owner clears it.
func recordVariant(value VARIANT) recordedVariant {
	recorded := recordedVariant{VT: value.VT, Val: value.Val}
	if value.VT == VT_BSTR {
		recorded.Str = BSTRToString(BSTR(value.Val))
	}
	return recorded
}

// reset forgets every call recorded so far, so that a test can set a window up and then check only what one publish
// said.
func (r *recorder) reset() {
	r.raises = nil
}

// count returns how many calls have been recorded.
func (r *recorder) count() int {
	return len(r.raises)
}

// at returns one recorded call, or the zero call when fewer than that many were recorded, so that a wrong expectation
// is reported as a difference rather than as a panic.
func (r *recorder) at(i int) recordedRaise {
	if i < 0 || i >= len(r.raises) {
		return recordedRaise{}
	}
	return r.raises[i]
}

// newRecordingWindow creates a test adapter whose UI Automation calls are recorded, with the window's first publish
// already forgotten: that publish announces that the window opened, which is TestRaiseWindowOpened's business and
// nothing else's, so a test that asks what one later publish said is not handed it as well.
func newRecordingWindow(t *testing.T, tree *accessibility.Tree, listening bool) (*testWindow, *recorder) {
	t.Helper()
	w, r := newOpeningWindow(t, tree, listening)
	r.reset()
	return w, r
}

// newOpeningWindow creates a test adapter whose UI Automation calls are recorded, including the Window_WindowOpened
// that creating it — the window's first publish — raises.
//
// The window is destroyed when the test finishes, for the reason newTestWindow gives; the recorder is still
// installed at that point, since it was registered first and cleanups run in reverse, so the teardown reaches nothing
// real.
func newOpeningWindow(t *testing.T, tree *accessibility.Tree, listening bool) (*testWindow, *recorder) {
	t.Helper()
	r := record(t, listening)
	w := &testWindow{}
	w.Window = NewWindow(Config{Action: w.record, HWND: testHWND}, tree,
		Geometry{Origin: testOrigin, Scale: testScale})
	t.Cleanup(w.Destroy)
	return w, r
}

// TestRaiseNothingWhenNobodyListens verifies the gate every raise sits behind. A window whose provider was created
// by an inspection tool or the touch keyboard must cost nothing beyond the snapshot the root package was building
// anyway.
//
// Withdrawing the window's provider and disconnecting the ones it held are the exceptions, and deliberately so: they
// are not events but statements about memory, and skipping them would leave UI Automation holding pointers to providers
// that no longer describe anything.
func TestRaiseNothingWhenNobodyListens(t *testing.T) {
	c := check.New(t)
	w, r := newRecordingWindow(t, eventTree(), false)
	button := w.providerFor(10)
	c.NotNil(button)

	cur := eventTree()
	cur.Node(10).Name = "Renamed"
	cur.Node(2).Value = "help"
	cur.Generation = 2
	w.Publish(cur, accessibility.Diff(eventTree(), cur))
	c.Equal(0, r.count())

	w.Announce("Nobody hears this")
	c.Equal(0, r.count())

	w.Destroy()
	for i := range r.count() {
		kind := r.at(i).Kind
		c.True(kind == RaiseDisconnect || kind == recordedReturnKind, "call %d was a %s", i, kind)
	}
	c.True(r.count() > 1, "the providers must still be disconnected and the window's provider withdrawn")
}

// TestRaiseNameChange verifies the shape of a property change: it names the provider for the node that changed, the
// property a client watches for, and the value on either side of the change as a BSTR.
func TestRaiseNameChange(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	old.Node(3).Name = "Ready"
	w, r := newRecordingWindow(t, old, true)

	cur := eventTree()
	cur.Node(3).Name = "Steady"
	cur.Generation = 2
	w.Publish(cur, accessibility.Diff(old, cur))

	c.Equal(1, r.count())
	c.Equal(RaiseProperty, r.at(0).Kind)
	c.Equal(w.providerFor(3).Unknown(), r.at(0).Provider)
	c.Equal(NamePropertyId, r.at(0).Property)
	c.Equal(PropertyID(30005), r.at(0).Property)
	c.Equal(VT_BSTR, r.at(0).Old.VT)
	c.Equal("Ready", r.at(0).Old.Str)
	c.Equal(VT_BSTR, r.at(0).New.VT)
	c.Equal("Steady", r.at(0).New.Str)
}

// TestRaiseToggleState verifies that a check box reporting a new check state reports it as the Toggle pattern's
// property, with both values as integers rather than as the text a name change carries.
func TestRaiseToggleState(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	w, r := newRecordingWindow(t, old, true)

	cur := eventTree()
	cur.Node(3).Checked = checkenum.Off
	cur.Generation = 2
	w.Publish(cur, accessibility.Diff(old, cur))

	c.Equal(1, r.count())
	c.Equal(RaiseProperty, r.at(0).Kind)
	c.Equal(w.providerFor(3).Unknown(), r.at(0).Provider)
	c.Equal(PropertyID(30086), r.at(0).Property)
	c.Equal(VT_I4, r.at(0).Old.VT)
	c.Equal(int32(ToggleState_On), r.at(0).Old.Int32())
	c.Equal(VT_I4, r.at(0).New.VT)
	c.Equal(int32(ToggleState_Off), r.at(0).New.Int32())
}

// TestRaiseTextEdit verifies what an edit to a text field says: the value it changed to, reported as the Value
// pattern's property, since that is where a client reads the text from, and nothing else. Neither of UI Automation's
// text events is raised — they belong to the Text pattern, which a field answers NULL for, so a client that responded
// to one would have nothing to read — and the caret moving reports nothing at all. TestTextEventsRaised covers the
// element that does hand out the pattern: a document.
func TestRaiseTextEdit(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	w, r := newRecordingWindow(t, old, true)

	cur := eventTree()
	cur.Node(2).Value = "help"
	cur.Node(2).Text = &accessibility.TextInfo{Text: "help", SelStart: 4, SelEnd: 4}
	cur.Generation = 2
	w.Publish(cur, accessibility.Diff(old, cur))

	field := w.providerFor(2).Unknown()
	c.Equal(1, r.count())
	c.Equal(RaiseProperty, r.at(0).Kind)
	c.Equal(field, r.at(0).Provider)
	c.Equal(PropertyID(30045), r.at(0).Property)
	c.Equal(VT_BSTR, r.at(0).Old.VT)
	c.Equal("hello", r.at(0).Old.Str)
	c.Equal("help", r.at(0).New.Str)
}

// TestRaiseFocus verifies that the focus moving inside a window is reported on the element that took it, and only
// while the window is the active one. A screen reader answers a focus event by moving its cursor, so raising one for a
// window the user is not looking at pulls them away from the one they are.
func TestRaiseFocus(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	w, r := newRecordingWindow(t, old, true)

	cur := eventTree()
	cur.Focus = 10
	cur.Node(2).Focused = false
	cur.Node(10).Focused = true
	cur.Generation = 2
	w.Publish(cur, accessibility.Diff(old, cur))
	c.Equal(1, r.count())
	c.Equal(RaiseEvent, r.at(0).Kind)
	c.Equal(w.providerFor(10).Unknown(), r.at(0).Provider)
	c.Equal(AutomationFocusChangedEventId, r.at(0).Event)

	// The same move within a window that is not active says nothing.
	r.reset()
	background := eventTree()
	background.Node(1).Focused = false
	background.Generation = 3
	w.Publish(background, accessibility.Diff(cur, background))
	for i := range r.count() {
		c.True(r.at(i).Event != AutomationFocusChangedEventId, "call %d reported the focus", i)
	}

	// Becoming active again points the client back at whatever holds the focus.
	r.reset()
	foreground := eventTree()
	foreground.Generation = 4
	w.Publish(foreground, accessibility.Diff(background, foreground))
	c.Equal(1, r.count())
	c.Equal(w.providerFor(2).Unknown(), r.at(0).Provider)
	c.Equal(AutomationFocusChangedEventId, r.at(0).Event)
}

// TestRaiseStructureAdded verifies that a new element reports itself as added, naming its own runtime identifier,
// and that its provider is created by the raise rather than having to exist first.
func TestRaiseStructureAdded(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	w, r := newRecordingWindow(t, old, true)

	cur := eventTree()
	cur.Node(4).Children = []accessibility.NodeID{5, 6, 11}
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 4, Role: role.ListItem, Selectable: true}
	cur.Generation = 2
	w.Publish(cur, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 11}})

	c.Equal(1, r.count())
	c.Equal(RaiseStructure, r.at(0).Kind)
	c.Equal(StructureChangeType_ChildAdded, r.at(0).Change)
	c.Equal(w.providerFor(11).Unknown(), r.at(0).Provider)
	c.Equal([]int32{AppendRuntimeId, 11, 0}, r.at(0).RuntimeID)

	// The whole diff reports the invalidation on the parent instead, with no runtime identifier at all, and the child
	// event is dropped: a client answers an invalidation by reading the children again.
	r.reset()
	w.Publish(cur, accessibility.Diff(old, cur))
	c.Equal(1, r.count())
	c.Equal(RaiseStructure, r.at(0).Kind)
	c.Equal(StructureChangeType_ChildrenInvalidated, r.at(0).Change)
	c.Equal(w.providerFor(4).Unknown(), r.at(0).Provider)
	c.Equal(0, len(r.at(0).RuntimeID))
}

// TestRaiseStructureRemoved verifies what leaving the tree looks like: the removal is reported on the parent the
// element left, naming the departed element's runtime identifier since the element itself is gone, and only then is its
// provider disconnected and retired.
func TestRaiseStructureRemoved(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	w, r := newRecordingWindow(t, old, true)
	item := w.providerFor(6)
	c.NotNil(item)
	itemUnknown := item.Unknown()

	cur := eventTree()
	cur.Node(4).Children = []accessibility.NodeID{5}
	delete(cur.Nodes, 6)
	cur.Generation = 2
	w.Publish(cur, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 6}})

	c.Equal(2, r.count())
	c.Equal(RaiseStructure, r.at(0).Kind)
	c.Equal(StructureChangeType_ChildRemoved, r.at(0).Change)
	c.Equal(w.providerFor(4).Unknown(), r.at(0).Provider, "a removal is reported on the parent the child left")
	c.Equal([]int32{AppendRuntimeId, 6, 0}, r.at(0).RuntimeID)
	c.Equal(RaiseDisconnect, r.at(1).Kind)
	c.Equal(itemUnknown, r.at(1).Provider)

	c.True(item.Stale())
	c.Nil(w.providerFor(6))
}

// TestRaiseSelection verifies the two halves of a selection change in a container that holds one selection at a
// time: each element reports whether it is selected now, and the one that became the selection reports that it is the
// selection, which implicitly deselects the other.
func TestRaiseSelection(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	old.Node(4).Multiselectable = false
	w, r := newRecordingWindow(t, old, true)

	cur := eventTree()
	cur.Node(4).Multiselectable = false
	cur.Node(5).Selected = false
	cur.Node(6).Selected = true
	cur.Generation = 2
	w.Publish(cur, accessibility.Diff(old, cur))

	c.Equal(3, r.count())
	c.Equal(RaiseProperty, r.at(0).Kind)
	c.Equal(w.providerFor(5).Unknown(), r.at(0).Provider)
	c.Equal(PropertyID(30079), r.at(0).Property)
	c.Equal(VT_BOOL, r.at(0).Old.VT)
	c.True(r.at(0).Old.Bool())
	c.False(r.at(0).New.Bool())
	c.Equal(RaiseProperty, r.at(1).Kind)
	c.Equal(w.providerFor(6).Unknown(), r.at(1).Provider)
	c.Equal(PropertyID(30079), r.at(1).Property)
	c.False(r.at(1).Old.Bool())
	c.True(r.at(1).New.Bool())
	c.Equal(RaiseEvent, r.at(2).Kind)
	c.Equal(w.providerFor(6).Unknown(), r.at(2).Provider)
	c.Equal(SelectionItem_ElementSelectedEventId, r.at(2).Event)
}

// TestRaiseAnnouncement verifies what an announcement asks a client to do: speak one piece of text, from the window
// rather than from any element in it, as one activity. Nothing is announced when there is nothing to say and nothing is
// announced to nobody.
func TestRaiseAnnouncement(t *testing.T) {
	c := check.New(t)
	w, r := newRecordingWindow(t, eventTree(), true)

	w.Announce("Saved")
	c.Equal(1, r.count())
	c.Equal(recordedNotifyKind, r.at(0).Kind)
	c.Equal(w.RootUnknown(), r.at(0).Provider)
	c.Equal(NotificationKind_Other, r.at(0).Notify)
	c.Equal(NotificationProcessing_All, r.at(0).Processing)
	c.Equal("Saved", r.at(0).Text)
	c.Equal("unison", r.at(0).Activity)

	// Saying the same thing twice really does say it twice, which is what an application asking twice means.
	w.Announce("Saved")
	c.Equal(2, r.count())

	w.Announce("")
	c.Equal(2, r.count())

	r.reset()
	r.listening = false
	w.Announce("Saved")
	c.Equal(0, r.count())

	// A window that has been destroyed has no root to announce anything from.
	r.listening = true
	w.Destroy()
	r.reset()
	w.Announce("Saved")
	c.Equal(0, r.count())
}

// TestRaiseWindowOpened verifies that a window announces itself as it appears, which is how a screen reader knows to
// read a dialog out. Every root raises it, dialog or not: Destroy raises Window_WindowClosed for every window, and a
// client tracking window lifetimes must not be told that a window it was never told about has closed.
func TestRaiseWindowOpened(t *testing.T) {
	c := check.New(t)
	dialog := eventTree()
	dialog.Node(1).Role = role.Dialog
	w, r := newOpeningWindow(t, dialog, true)
	c.Equal(1, r.count())
	c.Equal(RaiseEvent, r.at(0).Kind)
	c.Equal(w.RootUnknown(), r.at(0).Provider)
	c.Equal(Window_WindowOpenedEventId, r.at(0).Event)
	c.Equal(EventID(20016), r.at(0).Event)

	plainWindow, plain := newOpeningWindow(t, eventTree(), true)
	c.Equal(1, plain.count())
	c.Equal(plainWindow.RootUnknown(), plain.at(0).Provider)
	c.Equal(Window_WindowOpenedEventId, plain.at(0).Event)

	// Nothing is announced while nobody is listening, as for every other raise.
	_, quiet := newOpeningWindow(t, eventTree(), false)
	c.Equal(0, quiet.count())
}

// TestDestroyRaises verifies the sequence a window's destruction produces: the window reports that it closed while
// its fragment root is still connected and its providers still answer, the window's provider is then withdrawn so that
// a late WM_GETOBJECT is not answered with a fragment being torn down, and every provider is disconnected, retired and
// forgotten. Calling it again says nothing further.
//
// The fragment root is the one provider not disconnected, and deliberately: DisconnectProvider finds what to drop by
// asking for a runtime identifier, which a fragment root has none of, so the call could only fail. Withdrawing the
// window's provider is what takes the root away, and the stale flag answers anything a client still holds.
func TestDestroyRaises(t *testing.T) {
	c := check.New(t)
	w, r := newRecordingWindow(t, eventTree(), true)
	root := w.rootProvider()
	c.NotNil(root)
	rootUnknown := root.Unknown()
	field := w.providerFor(2)
	c.NotNil(field)
	fieldUnknown := field.Unknown()

	w.Destroy()

	c.Equal(3, r.count())
	c.Equal(RaiseEvent, r.at(0).Kind)
	c.Equal(rootUnknown, r.at(0).Provider)
	c.Equal(Window_WindowClosedEventId, r.at(0).Event)
	c.Equal(recordedReturnKind, r.at(1).Kind)
	c.Equal(testHWND, r.at(1).HWND)
	c.Nil(r.at(1).Provider)
	c.Equal(RaiseDisconnect, r.at(2).Kind)
	c.Equal(fieldUnknown, r.at(2).Provider, "every provider but the fragment root must be disconnected")

	c.True(root.Stale())
	c.True(field.Stale())
	c.Nil(w.rootProvider())
	c.Nil(w.RootUnknown())
	c.Nil(w.providerFor(2))

	r.reset()
	w.Destroy()
	c.Equal(0, r.count(), "a second destroy has nothing left to say")
}

// TestDestroyDescribesTheWindowOneLastTime verifies that the window's content is still there to be walked while
// Window_WindowClosed goes out. A client answers that event by asking about the window it names, so a fragment whose
// children had already been given up would make the window look empty at the one moment it is described for the last
// time.
func TestDestroyDescribesTheWindowOneLastTime(t *testing.T) {
	c := check.New(t)
	w, r := newRecordingWindow(t, eventTree(), true)
	field := w.providerFor(2)
	c.NotNil(field)
	var duringClose, rootDuringClose *Provider
	r.onEvent = func(id EventID) {
		if id == Window_WindowClosedEventId {
			duringClose = w.providerFor(2)
			rootDuringClose = w.rootProvider()
		}
	}

	w.Destroy()

	c.Equal(field, duringClose, "the window's providers must still answer while it reports that it closed")
	c.NotNil(rootDuringClose, "and so must the fragment root the event names")
	c.Nil(w.providerFor(2), "every provider is given up once the event has gone out")
	c.Nil(w.rootProvider())
}

// TestRaisedPropertyVariantTypes verifies the type each property a change can be reported for is carried in. A
// client reads a VARIANT by its type tag, so a property reported as the wrong type is a property it cannot read at all,
// and every one of these is a type UI Automation documents for that property rather than a choice.
//
// Every property DecideRaises can ask for belongs here, the ones this method answers itself rather than leaving to
// propertyValue included — the control patterns' own properties, modality among them — since nothing else covers those
// at all.
func TestRaisedPropertyVariantTypes(t *testing.T) {
	c := check.New(t)
	tree := newTestTree(
		1, 2,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{2, 3, 4, 5, 6, 7, 8},
		},
		&accessibility.Node{
			ID: 2, Role: role.TextField, Name: "Field", Description: "Type here", Focusable: true, Focused: true,
			Value: "hi", Bounds: geom.NewRect(0, 0, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue),
			Text:    &accessibility.TextInfo{Text: "hi", SelStart: 2, SelEnd: 2},
		},
		&accessibility.Node{ID: 3, Role: role.CheckBox, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{
			ID: 4, Role: role.Slider, HasNumber: true, Number: 5, Max: 10, Step: 1,
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue),
		},
		&accessibility.Node{ID: 5, Role: role.ListItem, Selectable: true, Selected: true, Expandable: true},
		&accessibility.Node{ID: 6, Role: role.ColumnHeader, Sort: accessibility.SortAscending},
		&accessibility.Node{ID: 7, Role: role.List, Multiselectable: true},
		// A combo box is here for the ExpandCollapse pattern: node 5 is Expandable, but a list item never supports the
		// pattern whatever it says, and a property is answered only by an element whose pattern it is.
		&accessibility.Node{ID: 8, Role: role.ComboBox, Expandable: true},
	)
	w := newTestWindow(t, tree)
	for i, one := range []struct {
		check    func(value *VARIANT)
		node     accessibility.NodeID
		property PropertyID
		expected VARTYPE
	}{
		{node: 2, property: NamePropertyId, expected: VT_BSTR, check: func(value *VARIANT) {
			c.Equal("Field", variantString(value))
		}},
		{node: 2, property: HelpTextPropertyId, expected: VT_BSTR, check: func(value *VARIANT) {
			c.Equal("Type here", variantString(value))
		}},
		{node: 2, property: ValueValuePropertyId, expected: VT_BSTR, check: func(value *VARIANT) {
			c.Equal("hi", variantString(value))
		}},
		{node: 6, property: ItemStatusPropertyId, expected: VT_BSTR, check: func(value *VARIANT) {
			c.Equal(ItemStatus(tree.Node(6)), variantString(value))
		}},
		{node: 2, property: IsEnabledPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.True(variantBool(value))
		}},
		{node: 2, property: IsOffscreenPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.False(variantBool(value))
		}},
		{node: 2, property: IsKeyboardFocusablePropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.True(variantBool(value))
		}},
		{node: 2, property: IsDataValidForFormPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.True(variantBool(value))
		}},
		{node: 2, property: ValueIsReadOnlyPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.False(variantBool(value))
		}},
		{node: 4, property: RangeValueIsReadOnlyPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.False(variantBool(value))
		}},
		{node: 5, property: SelectionItemIsSelectedPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.True(variantBool(value))
		}},
		{node: 1, property: WindowIsModalPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.False(variantBool(value))
		}},
		{node: 2, property: IsPasswordPropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.False(variantBool(value))
		}},
		{node: 7, property: SelectionCanSelectMultiplePropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.True(variantBool(value))
		}},
		{node: 3, property: ToggleToggleStatePropertyId, expected: VT_I4, check: func(value *VARIANT) {
			c.Equal(int32(ToggleState_On), variantInt32(value))
		}},
		{node: 2, property: ControlTypePropertyId, expected: VT_I4, check: func(value *VARIANT) {
			c.Equal(int32(EditControlTypeId), variantInt32(value))
		}},
		{
			node: 8, property: ExpandCollapseExpandCollapseStatePropertyId, expected: VT_I4,
			check: func(value *VARIANT) {
				c.Equal(int32(ExpandCollapseState_Collapsed), variantInt32(value))
			},
		},
		{node: 4, property: RangeValueValuePropertyId, expected: VT_R8, check: func(value *VARIANT) {
			c.Equal(5.0, math.Float64frombits(value.Val))
		}},
		// The pattern availability properties, which a pattern appearing or vanishing is reported through. Every
		// element answers one, so the interesting pair is an element that has the pattern and one that does not: node 5
		// is Expandable, but a list item never carries ExpandCollapse, and false is exactly what a client has to be
		// told.
		{
			node: 4, property: IsRangeValuePatternAvailablePropertyId, expected: VT_BOOL,
			check: func(value *VARIANT) {
				c.True(variantBool(value))
			},
		},
		{
			node: 5, property: IsExpandCollapsePatternAvailablePropertyId, expected: VT_BOOL,
			check: func(value *VARIANT) {
				c.False(variantBool(value))
			},
		},
		{node: 1, property: IsWindowPatternAvailablePropertyId, expected: VT_BOOL, check: func(value *VARIANT) {
			c.True(variantBool(value), "the fragment root is the one element that hands the Window pattern out")
		}},
		{node: 2, property: BoundingRectanglePropertyId, expected: VT_R8 | VT_ARRAY},
	} {
		var pin runtime.Pinner
		value, _ := pinnedOut[VARIANT](&pin)
		w.providerFor(one.node).raisedPropertyValue(tree, one.property, value)
		c.Equal(one.expected, value.VT, "case %d (property %d)", i, one.property)
		if one.check != nil {
			one.check(value)
		}
		value.Clear()
		pin.Unpin()
	}

	// A node the snapshot does not hold has no value for any property, which is what an empty VARIANT says; so does a
	// property nothing reports.
	var pin runtime.Pinner
	defer pin.Unpin()
	value, _ := pinnedOut[VARIANT](&pin)
	w.providerFor(2).raisedPropertyValue(nil, NamePropertyId, value)
	c.Equal(VT_EMPTY, value.VT)
	w.providerFor(2).raisedPropertyValue(tree, LocalizedControlTypePropertyId, value)
	c.Equal(VT_EMPTY, value.VT)

	// So does a pattern's property on a node that does not support the pattern, however much the node itself has to say
	// about it: node 5 is Expandable, but a list item never carries the ExpandCollapse pattern, and a value here would
	// be one a client could not read back through IExpandCollapseProvider. See ReportsProperty, which is what a
	// snapshot that has lost a state-gated pattern is answered through. The pattern's availability is the one thing
	// such an element does answer, which is what tells a client holding the pattern that it has gone.
	w.providerFor(5).raisedPropertyValue(tree, ExpandCollapseExpandCollapseStatePropertyId, value)
	c.Equal(VT_EMPTY, value.VT)
	w.providerFor(5).raisedPropertyValue(tree, IsExpandCollapsePatternAvailablePropertyId, value)
	c.Equal(VT_BOOL, value.VT)
	c.False(variantBool(value))
	value.Clear()
}

// TestTextEventsRaised verifies that an edit to a document and a caret move inside it reach a client as UI
// Automation's two text events, raised on the document itself. They are the only way a client can be told: the
// pattern's text is not a property, so there is nothing to raise a property change on, and a client answers either
// event by reading the document again through ITextProvider.
//
// A caret move is what NVDA reads as the caret having moved, which is the whole of its arrow-key reading, so the event
// has to go out for a selection change that alters nothing else about the snapshot.
func TestTextEventsRaised(t *testing.T) {
	c := check.New(t)
	old := textFixtureTree()
	w, r := newRecordingWindow(t, old, true)

	cur := textFixtureTree()
	cur.Generation = 2
	cur.Node(textDocumentID).Document = &accessibility.DocumentInfo{
		Text: accessibility.TextInfo{
			Text:      "Title!" + textFixtureText[5:],
			Lines:     old.Node(textDocumentID).Document.Text.Lines,
			Runs:      old.Node(textDocumentID).Document.Text.Runs,
			Spans:     old.Node(textDocumentID).Document.Text.Spans,
			SelStart:  20,
			SelEnd:    20,
			Caret:     20,
			Multiline: true,
		},
	}
	w.Publish(cur, accessibility.Diff(old, cur))

	document := w.providerFor(textDocumentID).Unknown()
	c.Equal(2, r.count())
	c.Equal(RaiseEvent, r.at(0).Kind)
	c.Equal(document, r.at(0).Provider)
	c.Equal(Text_TextChangedEventId, r.at(0).Event)
	c.Equal(RaiseEvent, r.at(1).Kind)
	c.Equal(document, r.at(1).Provider)
	c.Equal(Text_TextSelectionChangedEventId, r.at(1).Event)

	// A caret move on its own is still an event, since that is what a client follows the reading cursor by. The stream
	// itself is carried over from the publish above, so that nothing but the caret has changed.
	r.reset()
	moved := textFixtureTree()
	moved.Generation = 3
	moved.Node(textDocumentID).Document = &accessibility.DocumentInfo{
		Text: cur.Node(textDocumentID).Document.Text,
	}
	moved.Node(textDocumentID).Document.Text.SelStart = 25
	moved.Node(textDocumentID).Document.Text.SelEnd = 25
	moved.Node(textDocumentID).Document.Text.Caret = 25
	w.Publish(moved, accessibility.Diff(cur, moved))
	c.Equal(1, r.count())
	c.Equal(Text_TextSelectionChangedEventId, r.at(0).Event)
	c.Equal(document, r.at(0).Provider)
}
