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
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// These tests stand in for the UI Automation client the provider is built for. There is no way to make UI Automation
// call a provider from a unit test, so they call the Go functions the vtable slots hold directly, with the this pointer
// adjusted to the interface each one belongs to — which is itself worth testing, since getting that adjustment wrong is
// the failure mode this object layout invites.

// testOrigin and testScale are the geometry every test window uses: an origin that is not the screen's own, and a
// scale that is not one, so that a conversion that forgets either shows up.
var (
	testOrigin = geom.NewPoint(100, 50)
	testScale  = geom.NewPoint(2, 2)
)

// testWindow is a Window along with the action requests its providers have asked for.
type testWindow struct {
	*Window
	requestLock sync.Mutex
	requests    []accessibility.ActionRequest
}

// newTestWindow creates the adapter for a window that records action requests instead of performing them, with no
// client listening. The window is destroyed when the test finishes, whether or not the test destroys it itself — a
// second Destroy does nothing — so that every test exercises the teardown and none of them leaks its providers: a
// provider is pinned for the garbage collector while the provider map holds its reference, and only retirement gives
// that reference up.
//
// Cutting the window off from UI Automation matters as much as recording the requests: creating an adapter is the
// window's first publish and Destroy raises Window_WindowClosed and disconnects every provider, so without this the
// tests would make real UI Automation calls on any machine where something is listening — with providers for a window
// handle of zero, a fragment UI Automation can neither host nor resolve. A test that wants to see what would have been
// raised installs a recorder of its own with record, which replaces the same variables.
func newTestWindow(t *testing.T, tree *accessibility.Tree) *testWindow {
	t.Helper()
	silenceClients(t)
	w := &testWindow{}
	w.Window = NewWindow(Config{Action: w.record}, tree,
		Geometry{Origin: testOrigin, Scale: testScale})
	t.Cleanup(w.Destroy)
	return w
}

// newActionlessWindow creates an adapter with nowhere to send action requests, which is what a window the root
// package gave no action hook amounts to: every request a client makes of it must be refused rather than reported as
// done. It is destroyed when the test finishes, for the reason newTestWindow gives.
//
// It cuts the test off from UI Automation exactly as newTestWindow does, and for the whole test rather than only
// while the window is created. Creating an adapter is the window's first publish, which announces the window, but the
// teardown matters just as much: the t.Cleanup(w.Destroy) registered here raises Window_WindowClosed and disconnects
// every provider, and silencing only the creation would leave those calls reaching uiautomationcore.dll on any machine
// with a client attached — with providers for a window handle of zero. A test that wants to watch what would have been
// raised installs a recorder of its own with record afterwards, which replaces the same variables.
func newActionlessWindow(t *testing.T, tree *accessibility.Tree) *Window {
	t.Helper()
	silenceClients(t)
	w := NewWindow(Config{}, tree, Geometry{})
	t.Cleanup(w.Destroy)
	return w
}

// silenceClients cuts a test off from UI Automation for the duration of one test, restoring every entry point it
// replaced afterwards, so that nothing the test does reaches uiautomationcore.dll.
//
// Three of them have to be replaced rather than only the gate. ClientsAreListening is what every raise sits behind,
// and reporting that nobody is listening silences all of them; the other two are made whatever it answers.
// Provider.retire calls DisconnectProvider outside the gate, and the t.Cleanup(w.Destroy) of every test goes
// through it for each provider the test created — handing UI Automation providers it has never seen and letting it call
// back into GetRuntimeId from a thread of its own. Window.Destroy withdraws the window's provider with
// ReturnRawElementProvider, which these windows escape only by using a window handle of zero.
func silenceClients(t *testing.T) {
	t.Helper()
	savedListening := clientsAreListening
	savedDisconnect := disconnectProvider
	savedReturn := returnRawElementProvider
	t.Cleanup(func() {
		clientsAreListening = savedListening
		disconnectProvider = savedDisconnect
		returnRawElementProvider = savedReturn
	})
	clientsAreListening = func() bool { return false }
	disconnectProvider = func(_ unsafe.Pointer) uintptr { return uintptr(w32.COM_S_OK) }
	returnRawElementProvider = func(_ windows.HWND, _ w32.WPARAM, _ w32.LPARAM, _ unsafe.Pointer) w32.LRESULT { return 0 }
}

// record notes one action request.
func (w *testWindow) record(request accessibility.ActionRequest) {
	w.requestLock.Lock()
	defer w.requestLock.Unlock()
	w.requests = append(w.requests, request)
}

// recorded returns the action requests made so far.
func (w *testWindow) recorded() []accessibility.ActionRequest {
	w.requestLock.Lock()
	defer w.requestLock.Unlock()
	return append([]accessibility.ActionRequest(nil), w.requests...)
}

// providerFor returns the provider for one node, handing back the reference Provider took on the caller's behalf.
// Provider is what a UI Automation thread calls, so it must return an owned reference; a test holds the window itself
// and never retires a provider behind its own back, so the provider map's reference is enough to keep the pointer good
// for as long as the test needs it. Every test that cares about the counts themselves calls Provider directly.
func (w *Window) providerFor(id accessibility.NodeID) *Provider {
	p := w.Provider(id)
	if p != nil {
		p.release()
	}
	return p
}

// rootProvider returns the window's fragment root, handing back the reference Root took, for the reason providerFor
// gives.
func (w *Window) rootProvider() *Provider {
	root := w.Root()
	if root != nil {
		root.release()
	}
	return root
}

// pinnedOut allocates an out-parameter for a COM call and returns it along with its address. The allocation is
// pinned, so the address stays good for as long as the caller holds the pointer, which is exactly the guarantee a
// provider gets when UI Automation is the caller.
func pinnedOut[T any](pin *runtime.Pinner) (value *T, address uintptr) {
	value = new(T)
	pin.Pin(value)
	return value, uintptr(unsafe.Pointer(value))
}

// vtblsForTest returns every interface's virtual method table as a slice, in interface order.
func vtblsForTest() [ifaceCount][]uintptr {
	return [ifaceCount][]uintptr{
		ifaceSimple:         simpleVtbl[:],
		ifaceFragment:       fragmentVtbl[:],
		ifaceFragmentRoot:   fragmentRootVtbl[:],
		ifaceAdviseEvents:   adviseEventsVtbl[:],
		ifaceWindow:         windowVtbl[:],
		ifaceInvoke:         invokeVtbl[:],
		ifaceToggle:         toggleVtbl[:],
		ifaceValue:          valueVtbl[:],
		ifaceRangeValue:     rangeValueVtbl[:],
		ifaceSelection:      selectionVtbl[:],
		ifaceSelectionItem:  selectionItemVtbl[:],
		ifaceExpandCollapse: expandCollapseVtbl[:],
		ifaceScrollItem:     scrollItemVtbl[:],
		ifaceGrid:           gridVtbl[:],
		ifaceGridItem:       gridItemVtbl[:],
		ifaceTable:          tableVtbl[:],
		ifaceTableItem:      tableItemVtbl[:],
		ifaceText:           textVtbl[:],
		ifaceTextChild:      textChildVtbl[:],
	}
}

// TestVtbls verifies that every slot of every virtual method table is filled in and that each table is the size its
// interface declares. An empty slot is a jump to address zero the first time a client calls that method.
func TestVtbls(t *testing.T) {
	c := check.New(t)
	ensureVtbls()
	for which, vtbl := range vtblsForTest() {
		c.Equal(ifaceSlots[which], len(vtbl), "interface %d table size", which)
		c.Equal(uintptr(unsafe.Pointer(&vtbl[0])), vtblStarts[which], "interface %d table start", which)
		for slot, method := range vtbl {
			c.True(method != 0, "interface %d slot %d is empty", which, slot)
		}
	}
	// The text range's table is not one of a provider's interfaces, so it is not in the list above and has no entry in
	// vtblStarts, but every slot of it has to be filled in just the same.
	for slot, method := range textRangeVtbl {
		c.True(method != 0, "text range slot %d is empty", slot)
	}
}

// slotOut is the out-parameter the slot-order tests hand every indirect call. It is as wide as the widest
// out-parameter any slot in this package has — Rect's four doubles — so that a slot holding the wrong method, which
// is exactly what those tests exist to catch, cannot write past the end of it while the test is finding that out.
type slotOut struct {
	buf Rect
}

// slotScratch allocates an out-parameter for an indirect call and pins it, for the reason pinnedOut gives.
func slotScratch(pin *runtime.Pinner) *slotOut {
	out := &slotOut{}
	pin.Pin(out)
	return out
}

// fresh zeroes the buffer and returns its address, which is what a slot is handed. Zeroing before every call is what
// keeps a method that writes four bytes where another writes eight from being read as having written the difference.
func (o *slotOut) fresh() uintptr {
	o.buf = Rect{}
	return uintptr(unsafe.Pointer(o))
}

// i32 reads the buffer as the 32-bit integer a BOOL, a count, an index or an enumeration-valued property is.
func (o *slotOut) i32() int32 {
	return *(*int32)(unsafe.Pointer(o))
}

// f64 reads the buffer as the double every RangeValue measurement is.
func (o *slotOut) f64() float64 {
	return *(*float64)(unsafe.Pointer(o))
}

// ptr reads the buffer as the pointer an interface out-parameter and a BSTR both are.
func (o *slotOut) ptr() uintptr {
	return *(*uintptr)(unsafe.Pointer(o))
}

// array reads the buffer as a SAFEARRAY handle.
func (o *slotOut) array() SAFEARRAY {
	return SAFEARRAY(o.ptr())
}

// variant reads the buffer as the VARIANT GetPropertyValue fills in.
func (o *slotOut) variant() *VARIANT {
	return (*VARIANT)(unsafe.Pointer(o))
}

// rect reads the buffer as the Rect get_BoundingRectangle fills in.
func (o *slotOut) rect() Rect {
	return o.buf
}

// callSlot calls one method of one interface the way UI Automation does: indirectly, through the slot its virtual
// method table holds, with the this pointer for that interface. method is the index among the interface's own methods,
// so method 0 is the one declared first, after IUnknown's three.
func callSlot(p *Provider, which iface, method int, args ...uintptr) uint64 {
	ensureVtbls()
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, p.ifacePtr(which))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(vtblsForTest()[which][unknownSlots+method], all...)
	return uint64(r)
}

// TestVtblSlotOrder verifies the one thing buildVtbls calls the whole content of the ABI contract: that method N
// of an interface really sits in slot N of its virtual method table. No other test can. Every other test here calls the
// Go functions the slots were built from, and those answer the same however the slots are ordered, so two methods of
// the same shape swapped — get_CanMaximize for get_CanMinimize, say — would pass the entire suite while a real client
// got one answer where it asked for the other.
//
// Two slots are left out. Both hold assembly thunks, because both take doubles, and doubles arrive in floating-point
// registers that syscall.SyscallN cannot fill; TestThunkSlots checks that those two slots hold the thunks, and
// TestFromPointThunk and TestRangeValueSetValueThunk call them the way UI Automation would.
//
// The twelve control-pattern interfaces are covered by TestPatternVtblSlotOrder in patterns_windows_test.go.
func TestVtblSlotOrder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w := newTestWindow(t, sampleTree())
	root := w.rootProvider()
	c.NotNil(root)
	button := w.providerFor(4)
	c.NotNil(button)

	// IRawElementProviderSimple: get_ProviderOptions, GetPatternProvider, GetPropertyValue,
	// get_HostRawElementProvider.
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceSimple, 0, out.fresh()))
	c.Equal(int32(ProviderOptions_ServerSideProvider), out.i32(), "get_ProviderOptions")
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceSimple, 1, uintptr(InvokePatternId), out.fresh()))
	c.Equal(button.ifacePtr(ifaceInvoke), out.ptr(), "GetPatternProvider")
	c.Equal(uintptr(1), button.release())
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceSimple, 2, uintptr(NamePropertyId), out.fresh()))
	c.Equal(VT_BSTR, out.variant().VT, "GetPropertyValue")
	c.Equal("One", variantString(out.variant()))
	out.variant().Clear()
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceSimple, 3, out.fresh()))
	c.Equal(uintptr(0), out.ptr(), "get_HostRawElementProvider: only the fragment root has a host")

	// IRawElementProviderFragment: Navigate, GetRuntimeId, get_BoundingRectangle, GetEmbeddedFragmentRoots, SetFocus,
	// get_FragmentRoot. Node 4 is the first of the window's two buttons, at (0,0 50x20) in a window whose content area
	// starts at (100,50) with two pixels per logical unit.
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceFragment, 0, uintptr(NavigateDirection_NextSibling), out.fresh()))
	sibling := w.providerFor(5)
	c.NotNil(sibling)
	c.Equal(sibling.ifacePtr(ifaceFragment), out.ptr(), "Navigate")
	c.Equal(uintptr(1), sibling.release())
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceFragment, 1, out.fresh()))
	c.Equal([]int32{AppendRuntimeId, 4, 0}, safeArrayToInt32(out.array()), "GetRuntimeId")
	out.array().Destroy()
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceFragment, 2, out.fresh()))
	c.Equal(Rect{Left: 100, Top: 50, Width: 100, Height: 40}, out.rect(), "get_BoundingRectangle")
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceFragment, 3, out.fresh()))
	c.Equal(uintptr(0), out.ptr(), "GetEmbeddedFragmentRoots: unison draws every widget itself")
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceFragment, 4))
	c.Equal(1, len(w.recorded()), "SetFocus")
	c.Equal(accessibility.Focus, requestAt(w, 0).Action)
	c.Equal(accessibility.NodeID(4), requestAt(w, 0).Node)
	c.Equal(w32.COM_S_OK, callSlot(button, ifaceFragment, 5, out.fresh()))
	c.Equal(root.ifacePtr(ifaceFragmentRoot), out.ptr(), "get_FragmentRoot")
	c.Equal(uintptr(1), root.release())

	// IRawElementProviderFragmentRoot: ElementProviderFromPoint, which holds a thunk, then GetFocus.
	c.Equal(w32.COM_S_OK, callSlot(root, ifaceFragmentRoot, 1, out.fresh()))
	focus := w.providerFor(4)
	c.NotNil(focus)
	c.Equal(focus.ifacePtr(ifaceFragment), out.ptr(), "GetFocus")
	c.Equal(uintptr(1), focus.release())

	// IRawElementProviderAdviseEvents: AdviseEventAdded, AdviseEventRemoved. The two are told apart by which way they
	// move the count.
	c.Equal(int32(0), w.Listeners())
	c.Equal(w32.COM_S_OK, callSlot(root, ifaceAdviseEvents, 0, uintptr(AutomationFocusChangedEventId), 0))
	c.Equal(int32(1), w.Listeners(), "AdviseEventAdded")
	c.Equal(w32.COM_S_OK, callSlot(root, ifaceAdviseEvents, 1, uintptr(AutomationFocusChangedEventId), 0))
	c.Equal(int32(0), w.Listeners(), "AdviseEventRemoved")

	checkWindowSlotOrder(c, w, root, out)
}

// checkWindowSlotOrder verifies the slot order of IWindowProvider: SetVisualState, Close, WaitForInputIdle,
// get_CanMaximize, get_CanMinimize, get_IsModal, get_WindowVisualState, get_WindowInteractionState, get_IsTopmost.
//
// Six of the nine take nothing but an out-parameter, so telling them apart takes more than one window: each is asked
// about three, and over those three no two of the six answer the same sequence — CanMaximize says 1,0,0; CanMinimize
// 1,1,1; IsModal 0,1,0; WindowVisualState 0,0,0; WindowInteractionState ready, ready, blocked; and IsTopmost 0,0,1. Any
// two of them swapped therefore shows up as a wrong answer for at least one window.
func checkWindowSlotOrder(c check.Checker, w *testWindow, root *Provider, out *slotOut) {
	c.Equal(E_NOTSUPPORTED, callSlot(root, ifaceWindow, 0, uintptr(WindowVisualState_Maximized)),
		"SetVisualState")
	c.Equal(E_NOTSUPPORTED, callSlot(root, ifaceWindow, 1), "Close")
	c.Equal(E_NOTSUPPORTED, callSlot(root, ifaceWindow, 2, 100, out.fresh()), "WaitForInputIdle")
	for i, one := range []struct {
		interaction WindowInteractionState
		canMaximize int32
		isModal     int32
		isTopmost   int32
		resizable   bool
		modal       bool
		floating    bool
		disabled    bool
	}{
		{resizable: true, canMaximize: 1, interaction: WindowInteractionState_ReadyForUserInteraction},
		{modal: true, isModal: 1, interaction: WindowInteractionState_ReadyForUserInteraction},
		{floating: true, disabled: true, isTopmost: 1, interaction: WindowInteractionState_BlockedByModalWindow},
	} {
		next := sampleTree()
		next.Nodes[1].Resizable = one.resizable
		next.Nodes[1].Modal = one.modal
		next.Nodes[1].Floating = one.floating
		next.Nodes[1].Disabled = one.disabled
		next.Generation = uint64(i + 2)
		w.Publish(next, nil)
		c.Equal(w32.COM_S_OK, callSlot(root, ifaceWindow, 3, out.fresh()))
		c.Equal(one.canMaximize, out.i32(), "get_CanMaximize, window %d", i)
		c.Equal(w32.COM_S_OK, callSlot(root, ifaceWindow, 4, out.fresh()))
		c.Equal(int32(1), out.i32(), "get_CanMinimize, window %d", i)
		c.Equal(w32.COM_S_OK, callSlot(root, ifaceWindow, 5, out.fresh()))
		c.Equal(one.isModal, out.i32(), "get_IsModal, window %d", i)
		c.Equal(w32.COM_S_OK, callSlot(root, ifaceWindow, 6, out.fresh()))
		c.Equal(WindowVisualState_Normal, WindowVisualState(out.i32()), "get_WindowVisualState, window %d", i)
		c.Equal(w32.COM_S_OK, callSlot(root, ifaceWindow, 7, out.fresh()))
		c.Equal(one.interaction, WindowInteractionState(out.i32()), "get_WindowInteractionState, window %d", i)
		c.Equal(w32.COM_S_OK, callSlot(root, ifaceWindow, 8, out.fresh()))
		c.Equal(one.isTopmost, out.i32(), "get_IsTopmost, window %d", i)
	}
}

// TestProviderThisPointers verifies that the pointer a client holds for each interface recovers the provider it
// belongs to, which every COM method here depends on.
func TestProviderThisPointers(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())
	p := w.providerFor(4)
	c.NotNil(p)
	c.Equal(uintptr(unsafe.Pointer(p)), uintptr(p.Unknown()))
	for which := ifaceSimple; which < ifaceCount; which++ {
		this := p.ifacePtr(which)
		c.Equal(uintptr(unsafe.Pointer(p))+ifaceOffset(which), this)
		c.Equal(p, providerFromThis(this, which))
	}
}

// TestQueryInterface verifies that a provider hands out the interfaces it implements and refuses the rest, that the
// answer does not depend on which interface it was asked through, and that every interface handed out is AddRef'd.
func TestQueryInterface(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())
	root := w.rootProvider()
	c.NotNil(root)
	button := w.providerFor(4)
	c.NotNil(button)

	query := func(p *Provider, through, wanted iface) uint64 {
		guid := ifaceIIDs[wanted]
		pin.Pin(&guid)
		return queryInterface(through, p.ifacePtr(through), uintptr(unsafe.Pointer(&guid)), outAddress)
	}

	// The root implements the two provider interfaces plus the three only a fragment root has, and the Window pattern.
	rootIfaces := []iface{
		ifaceSimple, ifaceFragment, ifaceFragmentRoot, ifaceAdviseEvents, ifaceWindow,
	}
	for _, which := range rootIfaces {
		c.Equal(w32.COM_S_OK, query(root, ifaceSimple, which), "root wants interface %d", which)
		c.Equal(root.ifacePtr(which), *out)
		c.Equal(uintptr(1), root.release())
	}

	// IID_IUnknown is answered with the IRawElementProviderSimple table, whose first three slots are IUnknown's.
	unknown := w32.IIDUnknown
	pin.Pin(&unknown)
	c.Equal(w32.COM_S_OK, queryInterface(ifaceFragment, root.ifacePtr(ifaceFragment),
		uintptr(unsafe.Pointer(&unknown)), outAddress))
	c.Equal(root.ifacePtr(ifaceSimple), *out)
	c.Equal(uintptr(1), root.release())

	// A button is not a fragment root, and supports Invoke and nothing else.
	c.Equal(w32.COM_S_OK, query(button, ifaceFragment, ifaceInvoke))
	c.Equal(button.ifacePtr(ifaceInvoke), *out)
	c.Equal(uintptr(1), button.release())
	refused := []iface{ifaceFragmentRoot, ifaceAdviseEvents, ifaceWindow, ifaceToggle, ifaceValue}
	for _, which := range refused {
		c.Equal(w32.COM_E_NOINTERFACE, query(button, ifaceSimple, which), "button refuses interface %d", which)
		c.Equal(uintptr(0), *out)
	}

	// An interface this package does not implement at all, and a NULL out-parameter.
	other := xos.Must(windows.GUIDFromString("{11111111-2222-3333-4444-555555555555}"))
	pin.Pin(&other)
	c.Equal(w32.COM_E_NOINTERFACE, queryInterface(ifaceSimple, button.ifacePtr(ifaceSimple),
		uintptr(unsafe.Pointer(&other)), outAddress))
	c.Equal(w32.COM_E_POINTER, queryInterface(ifaceSimple, button.ifacePtr(ifaceSimple),
		uintptr(unsafe.Pointer(&other)), 0))

	// A NULL interface identifier fails too, and the out-parameter is cleared before that is decided: COM requires
	// QueryInterface to store NULL on every failure, so a caller is never left holding what it passed in. UI Automation
	// never passes one, which is exactly why the order has to be right here rather than discovered later.
	*out = 0xDEAD
	c.Equal(w32.COM_E_POINTER, queryInterface(ifaceSimple, button.ifacePtr(ifaceSimple), 0, outAddress))
	c.Equal(uintptr(0), *out)
}

// TestProviderReferenceCountLifetime verifies that a provider is unpinned only when its last COM reference goes,
// rather than when the window gives up the one its provider map holds: UI Automation's pointers to a provider are
// invisible to Go, so an early unpin would leave it calling into memory the collector may have reused.
func TestProviderReferenceCountLifetime(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())
	p := w.providerFor(4)
	c.NotNil(p)
	c.Equal(int32(1), atomic.LoadInt32(&p.refCount))

	// QueryInterface through one interface, taking a reference of its own.
	guid := ifaceIIDs[ifaceFragment]
	pin.Pin(&guid)
	c.Equal(w32.COM_S_OK, queryInterface(ifaceSimple, p.ifacePtr(ifaceSimple),
		uintptr(unsafe.Pointer(&guid)), outAddress))
	c.Equal(p.ifacePtr(ifaceFragment), *out)
	c.Equal(int32(2), atomic.LoadInt32(&p.refCount))

	// Retiring the provider gives up the provider map's reference alone, leaving the client's intact. It is done the
	// way a publish does it, through the window, so that the map lets go of the entry as well: retiring the provider
	// directly would leave the test's own Destroy to retire an already-retired one.
	pruned := sampleTree()
	delete(pruned.Nodes, 4)
	pruned.Nodes[3].Children = []accessibility.NodeID{5}
	w.retireRemovedProviders(pruned)
	c.True(p.Stale())
	c.Equal(int32(1), atomic.LoadInt32(&p.refCount))

	// The client's release, made through the interface it was handed, drops the last one.
	c.Equal(uintptr(0), providerFromThis(*out, ifaceFragment).release())

	// AddRef and Release must report the counts IUnknown promises, whichever interface they arrive through. The
	// reference for that arithmetic is the caller's own, taken through Provider and given back at the end: releasing
	// the provider map's instead would leave a zero-count, unpinned provider in the map for Destroy to retire, which is
	// a state the production code has no way of reaching.
	p = w.Provider(5)
	c.NotNil(p)
	c.Equal(int32(2), atomic.LoadInt32(&p.refCount))
	c.Equal(uintptr(3), providerFromThis(p.ifacePtr(ifaceFragment), ifaceFragment).addRef())
	c.Equal(uintptr(2), providerFromThis(p.ifacePtr(ifaceSimple), ifaceSimple).release())
	c.Equal(uintptr(1), p.release())
}

// TestProviderAnchor verifies that a provider is anchored for as long as a COM reference to it exists, and is let go
// of by the release of the last one. The anchor is what keeps the object reachable for the collector: UI Automation's
// pointers to it are invisible to Go, and a retired provider is no longer in the window's provider map either, so
// without it the only thing left pointing at the object would be the object's own pinner. Counts are compared as
// differences, since the set is the package's and other windows may be anchored in it.
func TestProviderAnchor(t *testing.T) {
	c := check.New(t)
	before := liveProviderCount()
	w := newTestWindow(t, sampleTree())
	p := w.Provider(4)
	c.NotNil(p)
	c.Equal(before+2, liveProviderCount(), "the fragment root and the provider just created")

	// Destroying the window gives up the provider map's reference to each of them. The fragment root has no other, so
	// it goes; the one this test holds keeps its provider anchored, which is exactly the state a client holding an
	// element for a window that has gone away puts the adapter in.
	w.Destroy()
	c.Equal(before+1, liveProviderCount())
	c.True(p.Stale())
	c.Equal(uintptr(0), p.release())
	c.Equal(before, liveProviderCount())
}

// TestProviderOptions verifies that providers report themselves as free-threaded server-side providers and nothing
// else. Asking for COM threading would require the UI thread to pump COM messages, which it cannot always do.
func TestProviderOptions(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[ProviderOptions](&pin)
	w := newTestWindow(t, sampleTree())
	c.Equal(w32.COM_S_OK, simpleProviderOptions(w.rootProvider().ifacePtr(ifaceSimple), outAddress))
	c.Equal(ProviderOptions_ServerSideProvider, *out)
	c.Equal(w32.COM_E_POINTER, simpleProviderOptions(0, 0))
}

// TestFragmentNavigate verifies navigation over the unignored tree: sampleTree buries its first two buttons under
// two layers of ignored grouping panels, which must be invisible to a client.
func TestFragmentNavigate(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())

	navigate := func(node accessibility.NodeID, direction NavigateDirection) uintptr {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(w32.COM_S_OK, fragmentNavigate(p.ifacePtr(ifaceFragment), uintptr(direction), outAddress))
		if *out != 0 {
			// The reference Navigate handed over is the caller's, which here is the test. Giving it back at once is
			// what lets the counts be checked at the end; the provider map's own reference keeps the pointer good.
			providerFromThis(*out, ifaceFragment).release()
		}
		return *out
	}
	fragment := func(node accessibility.NodeID) uintptr {
		p := w.providerFor(node)
		c.NotNil(p)
		return p.ifacePtr(ifaceFragment)
	}

	// The ignored groups are spliced away, so the window's children are the two buttons, the label and the one group
	// that is not ignored.
	c.Equal(fragment(4), navigate(1, NavigateDirection_FirstChild))
	c.Equal(fragment(7), navigate(1, NavigateDirection_LastChild))
	c.Equal(fragment(5), navigate(4, NavigateDirection_NextSibling))
	c.Equal(fragment(4), navigate(5, NavigateDirection_PreviousSibling))
	c.Equal(uintptr(0), navigate(4, NavigateDirection_PreviousSibling))

	// A node buried under ignored groups reports the nearest unignored ancestor as its parent.
	c.Equal(fragment(1), navigate(4, NavigateDirection_Parent))
	c.Equal(fragment(1), navigate(6, NavigateDirection_Parent))

	// The fragment root has no parent and no siblings: UI Automation stitches the fragment onto the window through the
	// host provider instead.
	c.Equal(uintptr(0), navigate(1, NavigateDirection_Parent))
	c.Equal(uintptr(0), navigate(1, NavigateDirection_NextSibling))
	c.Equal(uintptr(0), navigate(1, NavigateDirection_PreviousSibling))
	c.Equal(uintptr(0), navigate(4, NavigateDirection_FirstChild))

	// Every interface pointer Navigate handed out was AddRef'd and has been released again, so every provider is back
	// to the single reference the window's provider map holds. A method that handed out an unowned pointer, or a
	// caller that forgot to give one back, shows up here as a count of something other than one.
	for _, node := range []accessibility.NodeID{1, 4, 5, 6, 7} {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(int32(1), atomic.LoadInt32(&p.refCount), "node %d", node)
	}
	c.Equal(w32.COM_E_POINTER, fragmentNavigate(w.rootProvider().ifacePtr(ifaceFragment), 0, 0))
}

// TestGetRuntimeID verifies the runtime identifiers. The fragment root reports none, so that UI Automation
// identifies it by its window handle; everything else reports one that starts with AppendRuntimeId, so that UI
// Automation prepends the root's own identifier and the result stays unique across the process.
func TestGetRuntimeID(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[SAFEARRAY](&pin)
	w := newTestWindow(t, sampleTree())

	c.Equal(w32.COM_S_OK, fragmentGetRuntimeID(w.rootProvider().ifacePtr(ifaceFragment), outAddress))
	c.Equal(SAFEARRAY(0), *out)

	c.Equal(w32.COM_S_OK, fragmentGetRuntimeID(w.providerFor(4).ifacePtr(ifaceFragment), outAddress))
	c.True(*out != 0)
	c.Equal([]int32{AppendRuntimeId, 4, 0}, safeArrayToInt32(*out))
	out.Destroy()
	c.Equal(w32.COM_E_POINTER, fragmentGetRuntimeID(w.rootProvider().ifacePtr(ifaceFragment), 0))
}

// TestBoundingRectangle verifies the conversion from a node's window-local logical bounds to the screen rectangle
// UI Automation asks for, and that a node scrolled out of view reports nothing rather than a rectangle somewhere it
// is not.
func TestBoundingRectangle(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[Rect](&pin)
	tree := sampleTree()
	w := newTestWindow(t, tree)

	// Node 5 is at (50,0 50x20) in the window, and the window's content area starts at (100,50) with two pixels per
	// logical unit.
	c.Equal(w32.COM_S_OK, fragmentBoundingRectangle(w.providerFor(5).ifacePtr(ifaceFragment), outAddress))
	c.Equal(Rect{Left: 200, Top: 50, Width: 100, Height: 40}, *out)

	// Moving the window must move the rectangle without a new snapshot.
	w.SetGeometry(Geometry{Origin: geom.NewPoint(0, 0), Scale: geom.NewPoint(1, 1)})
	c.Equal(w32.COM_S_OK, fragmentBoundingRectangle(w.providerFor(5).ifacePtr(ifaceFragment), outAddress))
	c.Equal(Rect{Left: 50, Top: 0, Width: 50, Height: 20}, *out)

	tree.Nodes[5].Offscreen = true
	c.Equal(w32.COM_S_OK, fragmentBoundingRectangle(w.providerFor(5).ifacePtr(ifaceFragment), outAddress))
	c.Equal(Rect{}, *out)
	c.Equal(w32.COM_E_POINTER, fragmentBoundingRectangle(w.providerFor(5).ifacePtr(ifaceFragment), 0))
}

// TestFragmentRootAndEmbedded verifies that every element reports the same fragment root and that nothing claims an
// embedded fragment, since unison draws every widget itself.
func TestFragmentRootAndEmbedded(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())
	root := w.rootProvider()

	for _, node := range []accessibility.NodeID{1, 4, 7} {
		p := w.providerFor(node)
		c.Equal(w32.COM_S_OK, fragmentFragmentRoot(p.ifacePtr(ifaceFragment), outAddress))
		c.Equal(root.ifacePtr(ifaceFragmentRoot), *out)
		c.Equal(uintptr(1), root.release())
	}
	c.Equal(w32.COM_S_OK, fragmentGetEmbeddedFragmentRoots(root.ifacePtr(ifaceFragment), outAddress))
	c.Equal(uintptr(0), *out)
}

// TestSetFocus verifies that SetFocus asks the window for the focus rather than trying to move it here, and that an
// element that cannot take the focus says so instead of quietly doing nothing.
//
// Three things make it impossible, and each is answered the way the pattern write paths answer it: the element is
// disabled, which is E_ELEMENTNOTENABLED; it cannot take the focus at all, or does not offer the Focus action,
// which is the snapshot's own statement that nothing would happen; or the window has nowhere to send the request.
// Window.dispatchAccessibilityAction drops a request for an action a node does not offer, so answering S_OK would
// leave a client waiting for a focus event that is never coming.
func TestSetFocus(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())

	c.Equal(w32.COM_S_OK, fragmentSetFocus(w.providerFor(4).ifacePtr(ifaceFragment)))
	requests := w.recorded()
	c.Equal(1, len(requests))
	c.Equal(accessibility.NodeID(4), requests[0].Node)
	c.Equal(accessibility.Focus, requests[0].Action)

	// Node 5 is a button that cannot take the focus.
	c.Equal(E_INVALIDOPERATION, fragmentSetFocus(w.providerFor(5).ifacePtr(ifaceFragment)))
	c.Equal(1, len(w.recorded()))

	// A node that reports itself focusable while offering no Focus action refuses too. The pair is reachable:
	// axSnapshot.resolveFocus marks the node an open menu points at focusable after visit has narrowed a disabled
	// node's actions, and an Accessibility.Callback that sets Disabled leaves Focusable alone.
	actionless := sampleTree()
	actionless.Nodes[4].Actions = 0
	actionless.Generation = 2
	w.Publish(actionless, nil)
	c.Equal(E_INVALIDOPERATION, fragmentSetFocus(w.providerFor(4).ifacePtr(ifaceFragment)))
	c.Equal(1, len(w.recorded()))

	// A disabled node is refused with the answer every pattern write path gives for one, whatever its actions say.
	disabled := sampleTree()
	disabled.Nodes[4].Disabled = true
	disabled.Generation = 3
	w.Publish(disabled, nil)
	c.Equal(E_ELEMENTNOTENABLED, fragmentSetFocus(w.providerFor(4).ifacePtr(ifaceFragment)))
	c.Equal(1, len(w.recorded()))

	// A window with nowhere to send actions must refuse rather than report success.
	plain := newActionlessWindow(t, sampleTree())
	c.Equal(E_INVALIDOPERATION, fragmentSetFocus(plain.providerFor(4).ifacePtr(ifaceFragment)))
}

// TestElementProviderFromPoint verifies the hit test a screen reader's mouse tracking goes through, including the
// conversion from screen pixels to the snapshot's window-local logical units.
func TestElementProviderFromPoint(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())
	this := w.rootProvider().ifacePtr(ifaceFragmentRoot)

	fromPoint := func(x, y float64) uintptr {
		c.Equal(w32.COM_S_OK, fragmentRootElementProviderFromPoint(this, uintptr(math.Float64bits(x)),
			uintptr(math.Float64bits(y)), outAddress))
		return *out
	}

	// Window point (60,10) is inside node 5, and lands at screen (220,70) with this window's geometry.
	c.Equal(w.providerFor(5).ifacePtr(ifaceFragment), fromPoint(220, 70))
	c.Equal(uintptr(1), w.providerFor(5).release())

	// Window point (10,10) is inside node 4, which sits under two ignored groups; a hit on an ignored node is reported
	// as a hit on the nearest unignored ancestor, so nothing ignored can ever come back.
	c.Equal(w.providerFor(4).ifacePtr(ifaceFragment), fromPoint(120, 70))
	c.Equal(uintptr(1), w.providerFor(4).release())

	// Nodes 8 and 9 occupy the same area, with 8 first, so the topmost wins.
	c.Equal(w.providerFor(8).ifacePtr(ifaceFragment), fromPoint(140, 270))
	c.Equal(uintptr(1), w.providerFor(8).release())

	// Outside the window there is nothing to report, which lets UI Automation fall back to the window itself.
	c.Equal(uintptr(0), fromPoint(0, 0))
	c.Equal(w32.COM_E_POINTER, fragmentRootElementProviderFromPoint(this, 0, 0, 0))
}

// TestGetFocus verifies that the fragment root reports the focused element only while its window is the active one:
// pointing a client at an element in a window the user is not looking at makes a screen reader jump away from where the
// user is.
func TestGetFocus(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	tree := sampleTree()
	w := newTestWindow(t, tree)
	this := w.rootProvider().ifacePtr(ifaceFragmentRoot)

	c.Equal(w32.COM_S_OK, fragmentRootGetFocus(this, outAddress))
	c.Equal(w.providerFor(4).ifacePtr(ifaceFragment), *out)
	c.Equal(uintptr(1), w.providerFor(4).release())

	// The window is no longer the active one.
	inactive := sampleTree()
	inactive.Nodes[1].Focused = false
	inactive.Generation = 2
	w.Publish(inactive, nil)
	c.Equal(w32.COM_S_OK, fragmentRootGetFocus(this, outAddress))
	c.Equal(uintptr(0), *out)

	// Nothing in the window holds the focus.
	unfocused := sampleTree()
	unfocused.Focus = 0
	unfocused.Generation = 3
	w.Publish(unfocused, nil)
	c.Equal(w32.COM_S_OK, fragmentRootGetFocus(this, outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(w32.COM_E_POINTER, fragmentRootGetFocus(this, 0))
}

// TestAdviseEvents verifies the listener counter, which exists for diagnostics: whether to raise an event is decided
// by ClientsAreListening, not by this.
func TestAdviseEvents(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())
	this := w.rootProvider().ifacePtr(ifaceAdviseEvents)
	c.Equal(int32(0), w.Listeners())
	c.Equal(w32.COM_S_OK, adviseEventAdded(this, uintptr(AutomationFocusChangedEventId), 0))
	c.Equal(w32.COM_S_OK, adviseEventAdded(this, uintptr(AutomationPropertyChangedEventId), 0))
	c.Equal(int32(2), w.Listeners())
	c.Equal(w32.COM_S_OK, adviseEventRemoved(this, uintptr(AutomationFocusChangedEventId), 0))
	c.Equal(int32(1), w.Listeners())
}

// TestWindowPattern verifies the Window pattern the fragment root implements, including that every one of its
// answers comes from the snapshot rather than from a constant: a fixed-size dialog must not be reported as something
// that can be maximized, and a window created with FloatingWindowOption really is topmost.
func TestWindowPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	boolean, booleanAddress := pinnedOut[int32](&pin)
	visual, visualAddress := pinnedOut[WindowVisualState](&pin)
	interaction, interactionAddress := pinnedOut[WindowInteractionState](&pin)
	tree := sampleTree()
	w := newTestWindow(t, tree)
	this := w.rootProvider().ifacePtr(ifaceWindow)

	c.Equal(w32.COM_S_OK, windowCanMaximize(this, booleanAddress))
	c.Equal(int32(0), *boolean, "sampleTree's window is not resizable, so it has no maximize box")
	c.Equal(w32.COM_S_OK, windowCanMinimize(this, booleanAddress))
	c.Equal(int32(1), *boolean, "every window unison creates has a minimize box")
	c.Equal(w32.COM_S_OK, windowIsTopmost(this, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(w32.COM_S_OK, windowIsModal(this, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(w32.COM_S_OK, windowVisualState(this, visualAddress))
	c.Equal(WindowVisualState_Normal, *visual)
	c.Equal(w32.COM_S_OK, windowInteractionStateValue(this, interactionAddress))
	c.Equal(WindowInteractionState_ReadyForUserInteraction, *interaction)

	// A resizable, floating window reports both, which is what the snapshot records for one created without
	// NotResizableWindowOption and with FloatingWindowOption.
	free := sampleTree()
	free.Nodes[1].Resizable = true
	free.Nodes[1].Floating = true
	free.Generation = 2
	w.Publish(free, nil)
	c.Equal(w32.COM_S_OK, windowCanMaximize(this, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(w32.COM_S_OK, windowIsTopmost(this, booleanAddress))
	c.Equal(int32(1), *boolean)

	// A modal window says so, and one that is disabled is disabled because something modal is in front of it.
	modal := sampleTree()
	modal.Nodes[1].Modal = true
	modal.Nodes[1].Disabled = true
	modal.Generation = 3
	w.Publish(modal, nil)
	c.Equal(w32.COM_S_OK, windowIsModal(this, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(w32.COM_S_OK, windowInteractionStateValue(this, interactionAddress))
	c.Equal(WindowInteractionState_BlockedByModalWindow, *interaction)

	// The three methods that would act on the window are not offered.
	c.Equal(E_NOTSUPPORTED, windowSetVisualState(this, uintptr(WindowVisualState_Maximized)))
	c.Equal(E_NOTSUPPORTED, windowClose(this))
	c.Equal(E_NOTSUPPORTED, windowWaitForInputIdle(this, 100, booleanAddress))
	c.Equal(w32.COM_E_POINTER, windowCanMaximize(this, 0))

	// Every getter goes through the element rather than answering blind, so a client holding an IWindowProvider for a
	// root that has since been retired is told the element is gone rather than handed a fabricated answer.
	root := w.Root() // Stand in for the reference such a client would be holding.
	defer root.release()
	w.Destroy()
	c.True(root.Stale())
	c.Equal(E_ELEMENTNOTAVAILABLE, windowCanMaximize(this, booleanAddress))
	c.Equal(E_ELEMENTNOTAVAILABLE, windowCanMinimize(this, booleanAddress))
	c.Equal(E_ELEMENTNOTAVAILABLE, windowIsModal(this, booleanAddress))
	c.Equal(E_ELEMENTNOTAVAILABLE, windowVisualState(this, visualAddress))
	c.Equal(E_ELEMENTNOTAVAILABLE, windowInteractionStateValue(this, interactionAddress))
	c.Equal(E_ELEMENTNOTAVAILABLE, windowIsTopmost(this, booleanAddress))
}

// TestGetPatternProvider verifies that GetPatternProvider and QueryInterface agree, which a client that reaches a
// pattern both ways depends on, and that the interface it is handed is one the pattern's methods can be called on.
func TestGetPatternProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())
	button := w.providerFor(4)

	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(button.ifacePtr(ifaceSimple),
		uintptr(InvokePatternId), outAddress))
	c.Equal(button.ifacePtr(ifaceInvoke), *out)
	c.Equal(uintptr(1), button.release())

	// The interface that came back is the one the pattern's methods answer on; see patterns_windows_test.go for
	// what each of them answers.
	c.Equal(w32.COM_S_OK, invokeInvoke(button.ifacePtr(ifaceInvoke)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.Press, w.recorded()[0].Action)

	// A pattern the element does not support, and one this package does not implement, are both answered with a NULL
	// interface and S_OK rather than with an error.
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(button.ifacePtr(ifaceSimple),
		uintptr(TogglePatternId), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(button.ifacePtr(ifaceSimple),
		uintptr(ScrollPatternId), outAddress))
	c.Equal(uintptr(0), *out)

	// The root is the only element with the Window pattern.
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(w.rootProvider().ifacePtr(ifaceSimple),
		uintptr(WindowPatternId), outAddress))
	c.Equal(w.rootProvider().ifacePtr(ifaceWindow), *out)
	c.Equal(uintptr(1), w.rootProvider().release())
	c.Equal(w32.COM_E_POINTER, simpleGetPatternProvider(button.ifacePtr(ifaceSimple), 0, 0))
}

// TestHostRawElementProvider verifies that only the fragment root claims a host provider. Everything else answers
// NULL, which is what says "I am part of a fragment rather than a window of my own".
//
// The root's own answer is the one branch of any provider method that calls a real UI Automation entry point and
// transfers a COM reference out of it, so HostProviderFromHwnd is stood in for here rather than called: a test has
// no window for it to answer about, and the reference it hands back would be a real one to release.
func TestHostRawElementProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, sampleTree())
	c.Equal(w32.COM_S_OK, simpleHostRawElementProvider(w.providerFor(4).ifacePtr(ifaceSimple), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(w32.COM_E_POINTER, simpleHostRawElementProvider(w.providerFor(4).ifacePtr(ifaceSimple), 0))

	host := &w32.Unknown{}
	pin.Pin(host)
	asked := windows.HWND(0)
	saved := hostProviderFromHwnd
	t.Cleanup(func() { hostProviderFromHwnd = saved })
	hostProviderFromHwnd = func(hwnd windows.HWND) (*w32.Unknown, uintptr) {
		asked = hwnd
		return host, uintptr(w32.COM_S_OK)
	}

	root := w.rootProvider()
	c.Equal(w32.COM_S_OK, simpleHostRawElementProvider(root.ifacePtr(ifaceSimple), outAddress))
	c.Equal(uintptr(unsafe.Pointer(host)), *out, "the reference the call returned is handed straight to the caller")
	c.Equal(w.HWND(), asked, "and it is asked about this window")

	// A failure is reported as it came back, with the out-parameter left NULL rather than holding half an answer.
	hostProviderFromHwnd = func(_ windows.HWND) (*w32.Unknown, uintptr) {
		return nil, uintptr(uint32(0x80004005)) // E_FAIL
	}
	c.Equal(uint64(0x80004005), simpleHostRawElementProvider(root.ifacePtr(ifaceSimple), outAddress))
	c.Equal(uintptr(0), *out)

	// A client still holding the root after the window has been destroyed must be told the element is gone. Windows
	// reuses window handles, so asking about this one afterwards could describe some other window entirely.
	w.Destroy()
	c.Equal(E_ELEMENTNOTAVAILABLE, simpleHostRawElementProvider(root.ifacePtr(ifaceSimple), outAddress))
	c.Equal(uintptr(0), *out)
}

// variantString returns the contents of a VT_BSTR VARIANT.
func variantString(value *VARIANT) string {
	return BSTRToString(BSTR(value.Val))
}

// variantBool returns the contents of a VT_BOOL VARIANT.
func variantBool(value *VARIANT) bool {
	return int16(uint16(value.Val)) == VARIANT_TRUE
}

// variantInt32 returns the contents of a VT_I4 VARIANT.
func variantInt32(value *VARIANT) int32 {
	return int32(uint32(value.Val))
}

// TestGetPropertyValue verifies the properties a provider answers, including the ones only the fragment root has and
// the rule that an unknown or absent property is an empty VARIANT rather than an error.
func TestGetPropertyValue(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := pinnedOut[VARIANT](&pin)
	tree := sampleTree()
	tree.Nodes[4].Description = "The first button"
	tree.Nodes[4].Shortcut = "Ctrl+1"
	tree.Nodes[4].DescribedBy = []accessibility.NodeID{6}
	w := newTestWindow(t, tree)

	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(w32.COM_S_OK, simpleGetPropertyValue(p.ifacePtr(ifaceSimple), uintptr(id), valueAddress))
		return value
	}

	c.Equal(VT_BSTR, property(4, NamePropertyId).VT)
	c.Equal("One", variantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, HelpTextPropertyId).VT)
	c.Equal("The first button", variantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, FullDescriptionPropertyId).VT)
	c.Equal("The first button", variantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, AcceleratorKeyPropertyId).VT)
	c.Equal("Ctrl+1", variantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, AutomationIdPropertyId).VT)
	c.Equal("4", variantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, FrameworkIdPropertyId).VT)
	c.Equal(FrameworkID, variantString(value))
	value.Clear()

	c.Equal(VT_I4, property(4, ControlTypePropertyId).VT)
	c.Equal(int32(ButtonControlTypeId), variantInt32(value))
	value.Clear()

	c.Equal(VT_BOOL, property(4, IsEnabledPropertyId).VT)
	c.True(variantBool(value))
	value.Clear()

	c.Equal(VT_BOOL, property(4, IsKeyboardFocusablePropertyId).VT)
	c.True(variantBool(value))
	value.Clear()

	// The node holds the focus and its window is the active one, which is what HasKeyboardFocus means.
	c.Equal(VT_BOOL, property(4, HasKeyboardFocusPropertyId).VT)
	c.True(variantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(5, HasKeyboardFocusPropertyId).VT)
	c.False(variantBool(value))
	value.Clear()

	// The root must not claim it alongside the control that really has it. Its Focused flag says the window is active,
	// not that the window itself is where typing goes, and it pairs with an IsKeyboardFocusable of false, which is a
	// combination a client cannot make sense of.
	c.Equal(VT_BOOL, property(1, HasKeyboardFocusPropertyId).VT)
	c.False(variantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(1, IsKeyboardFocusablePropertyId).VT)
	c.False(variantBool(value))
	value.Clear()

	// A label that names another element is left out of the content view, so that a screen reader does not say it
	// twice; everything else that is not ignored is in both views.
	c.Equal(VT_BOOL, property(6, IsContentElementPropertyId).VT)
	c.False(variantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(6, IsControlElementPropertyId).VT)
	c.True(variantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(4, IsContentElementPropertyId).VT)
	c.True(variantBool(value))
	value.Clear()

	// LabeledBy is one element; DescribedBy is an array of them. Both add a reference per element handed out, which
	// clearing the VARIANT gives back.
	label := w.providerFor(6)
	c.Equal(VT_UNKNOWN, property(4, LabeledByPropertyId).VT)
	c.Equal(uintptr(label.Unknown()), uintptr(value.Val))
	c.Equal(int32(2), atomic.LoadInt32(&label.refCount))
	value.Clear()
	c.Equal(int32(1), atomic.LoadInt32(&label.refCount))

	c.Equal(VT_ARRAY|VT_UNKNOWN, property(4, DescribedByPropertyId).VT)
	c.Equal(int32(2), atomic.LoadInt32(&label.refCount))
	value.Clear()
	c.Equal(int32(1), atomic.LoadInt32(&label.refCount))

	// Only the fragment root answers the window-level properties.
	c.Equal(VT_BOOL, property(1, IsDialogPropertyId).VT)
	c.False(variantBool(value))
	value.Clear()
	c.Equal(VT_I4, property(1, NativeWindowHandlePropertyId).VT)
	c.Equal(int32(0), variantInt32(value))
	value.Clear()
	c.Equal(VT_EMPTY, property(4, IsDialogPropertyId).VT)
	c.Equal(VT_EMPTY, property(4, NativeWindowHandlePropertyId).VT)

	// The live setting is answered by nobody: a client consults it only for an element it has been given a
	// LiveRegionChanged event for, and this package raises that for nothing — an announcement goes out as a
	// notification event, carrying its own ordering.
	c.Equal(VT_EMPTY, property(1, LiveSettingPropertyId).VT)
	c.Equal(VT_EMPTY, property(4, LiveSettingPropertyId).VT)

	// Properties with nothing to report, and properties this provider never answers, are empty rather than an error.
	c.Equal(VT_EMPTY, property(5, NamePropertyId+1000).VT)
	c.Equal(VT_EMPTY, property(5, HelpTextPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, LevelPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, ItemStatusPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, LabeledByPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, DescribedByPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, PositionInSetPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, LocalizedControlTypePropertyId).VT)

	// The heading level of something that is not a heading is a value of its own rather than nothing.
	c.Equal(VT_I4, property(5, HeadingLevelPropertyId).VT)
	c.Equal(int32(HeadingLevel_None), variantInt32(value))
	value.Clear()

	c.Equal(w32.COM_E_POINTER, simpleGetPropertyValue(w.providerFor(5).ifacePtr(ifaceSimple), 0, 0))
}

// TestHelpTextCarriesPlaceholder verifies that the watermark of a field with no description of its own is reported
// as its help text, which is the conventional carrier for one and the only thing that keeps an unnamed search field
// from being announced as a bare "edit". A description wins when a node has both: it is what the application said
// about the control, while the watermark is a hint the widget put in its own empty content, and FullDescription — the
// other property answered from the description — never carries the watermark at all.
func TestHelpTextCarriesPlaceholder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := pinnedOut[VARIANT](&pin)
	tree := newTestTree(
		1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2, 3, 4}},
		&accessibility.Node{ID: 2, Role: role.TextField, Placeholder: "Search"},
		&accessibility.Node{ID: 3, Role: role.TextField, Placeholder: "Search", Description: "Filters the list"},
		&accessibility.Node{ID: 4, Role: role.TextField},
	)
	w := newTestWindow(t, tree)
	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(w32.COM_S_OK, simpleGetPropertyValue(p.ifacePtr(ifaceSimple), uintptr(id), valueAddress))
		return value
	}

	c.Equal(VT_BSTR, property(2, HelpTextPropertyId).VT)
	c.Equal("Search", variantString(value))
	value.Clear()
	c.Equal(VT_EMPTY, property(2, FullDescriptionPropertyId).VT, "a watermark is not a full description")

	c.Equal(VT_BSTR, property(3, HelpTextPropertyId).VT)
	c.Equal("Filters the list", variantString(value), "a description wins over a watermark")
	value.Clear()
	c.Equal(VT_BSTR, property(3, FullDescriptionPropertyId).VT)
	c.Equal("Filters the list", variantString(value))
	value.Clear()

	c.Equal(VT_EMPTY, property(4, HelpTextPropertyId).VT, "a field with neither says nothing")
}

// TestValuePropertyReadsNodeValue verifies that the Value pattern's property is answered through GetPropertyValue
// as well as through IValueProvider::get_Value, and only by an element that has the pattern.
//
// A table cell is what needs it: Table.axAddRow clears the name of a cell that holds one widget and puts that widget's
// state into the cell's value, precisely so that a change to the widget is a change to the cell, and a screen reader
// reading across a row asks the cell for its value.
func TestValuePropertyReadsNodeValue(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := pinnedOut[VARIANT](&pin)
	out, outAddress := pinnedOut[uintptr](&pin)
	tree := tableTree()
	tree.Nodes[8].Name = ""
	tree.Nodes[8].Value = "checked"
	w := newTestWindow(t, tree)
	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(w32.COM_S_OK, simpleGetPropertyValue(p.ifacePtr(ifaceSimple), uintptr(id), valueAddress))
		return value
	}

	c.Equal(VT_BSTR, property(8, ValueValuePropertyId).VT)
	c.Equal("checked", variantString(value))
	value.Clear()

	// The pattern comes with it, both ways a client can reach one, and the value cannot be set through it: a cell
	// offers no SetValue action.
	cell := w.providerFor(8)
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(cell.ifacePtr(ifaceSimple), uintptr(ValuePatternId),
		outAddress))
	c.Equal(cell.ifacePtr(ifaceValue), *out)
	c.Equal(uintptr(1), cell.release())
	c.Equal(w32.COM_S_OK, valueValue(cell.ifacePtr(ifaceValue), outAddress))
	c.Equal("checked", BSTRToString(BSTR(*out)))
	BSTR(*out).Free()
	boolean, booleanAddress := pinnedOut[int32](&pin)
	c.Equal(w32.COM_S_OK, valueIsReadOnly(cell.ifacePtr(ifaceValue), booleanAddress))
	c.Equal(int32(1), *boolean)

	// A cell whose name carries its content has no value, so it has no Value pattern and nothing to report through it.
	c.Equal(VT_EMPTY, property(9, ValueValuePropertyId).VT)
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(w.providerFor(9).ifacePtr(ifaceSimple),
		uintptr(ValuePatternId), outAddress))
	c.Equal(uintptr(0), *out)

	// Neither does an element with no value of any kind, however the property is asked for.
	c.Equal(VT_EMPTY, property(7, ValueValuePropertyId).VT)
}

// TestStaleProvider verifies what a client holding an element for something that has been destroyed is told. Every
// method must report that the element is no longer available rather than answering from a snapshot that no longer holds
// it, and the provider must stay alive while the client still holds it.
func TestStaleProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	value, valueAddress := pinnedOut[VARIANT](&pin)
	rect, rectAddress := pinnedOut[Rect](&pin)
	array, arrayAddress := pinnedOut[SAFEARRAY](&pin)
	w := newTestWindow(t, sampleTree())
	p := w.providerFor(5)
	c.NotNil(p)
	p.addRef() // Stand in for the reference a client would be holding.

	// Node 5 leaves the tree.
	without := sampleTree()
	delete(without.Nodes, 5)
	without.Nodes[3].Children = []accessibility.NodeID{4}
	without.Generation = 2
	w.Publish(without, nil)

	c.True(p.Stale())
	c.Equal(int32(1), atomic.LoadInt32(&p.refCount))
	c.Nil(w.providerFor(5))

	simple := p.ifacePtr(ifaceSimple)
	fragment := p.ifacePtr(ifaceFragment)
	c.Equal(E_ELEMENTNOTAVAILABLE, simpleGetPropertyValue(simple, uintptr(NamePropertyId), valueAddress))
	c.Equal(VT_EMPTY, value.VT)
	c.Equal(E_ELEMENTNOTAVAILABLE, simpleGetPatternProvider(simple, uintptr(InvokePatternId), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(E_ELEMENTNOTAVAILABLE, fragmentNavigate(fragment, uintptr(NavigateDirection_Parent), outAddress))
	// GetRuntimeId is the one exception: DisconnectProvider asks for it while retiring the provider, so it has to
	// keep answering, and the identifier is derived from the node id alone rather than from the tree.
	c.Equal(w32.COM_S_OK, fragmentGetRuntimeID(fragment, arrayAddress))
	c.Equal([]int32{AppendRuntimeId, 5, 0}, safeArrayToInt32(*array))
	array.Destroy()
	c.Equal(E_ELEMENTNOTAVAILABLE, fragmentBoundingRectangle(fragment, rectAddress))
	c.Equal(Rect{}, *rect)
	c.Equal(E_ELEMENTNOTAVAILABLE, fragmentSetFocus(fragment))
	c.Equal(E_ELEMENTNOTAVAILABLE, simpleHostRawElementProvider(simple, outAddress))
	c.Equal(uintptr(0), *out)

	// A stale element supports no pattern interface, but it is still an IUnknown and an IRawElementProviderSimple, and
	// it still describes how it wants to be called.
	guid := ifaceIIDs[ifaceInvoke]
	pin.Pin(&guid)
	c.Equal(w32.COM_E_NOINTERFACE, queryInterface(ifaceSimple, simple, uintptr(unsafe.Pointer(&guid)), outAddress))
	simpleGUID := ifaceIIDs[ifaceSimple]
	pin.Pin(&simpleGUID)
	c.Equal(w32.COM_S_OK, queryInterface(ifaceSimple, simple, uintptr(unsafe.Pointer(&simpleGUID)), outAddress))
	c.Equal(simple, *out)
	c.Equal(uintptr(1), p.release())

	// Nodes that are still in the tree are untouched, and the fragment root is never retired by a publish.
	c.NotNil(w.providerFor(4))
	c.False(w.providerFor(4).Stale())
	c.NotNil(w.rootProvider())
	c.False(w.rootProvider().Stale())
}

// TestProviderLookup verifies which nodes have providers at all. An ignored node is spliced out of the tree a client
// sees, so nothing can ask about one.
func TestProviderLookup(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())
	c.Nil(w.providerFor(0))
	c.Nil(w.providerFor(2), "an ignored node has no provider")
	c.Nil(w.providerFor(3), "an ignored node has no provider")
	c.Nil(w.providerFor(999), "a node that is not in the tree has no provider")
	c.NotNil(w.providerFor(1))
	c.Equal(w.providerFor(4), w.providerFor(4), "a second lookup returns the same provider")
	c.Equal(w.rootProvider(), w.providerFor(1))
	c.Equal(unsafe.Pointer(&w.rootProvider().vtbls[ifaceSimple]), w.RootUnknown())
}

// TestDestroy verifies that the adapter gives up every provider as its window is destroyed, and that a client still
// holding one is told the element is gone rather than left pointing at memory nothing owns.
func TestDestroy(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())
	root := w.rootProvider()
	button := w.providerFor(4)
	button.addRef() // Stand in for the reference a client would be holding.

	w.Destroy()
	c.Nil(w.rootProvider())
	c.Nil(w.RootUnknown())
	c.Nil(w.providerFor(4))
	c.True(root.Stale())
	c.True(button.Stale())
	c.Equal(int32(1), atomic.LoadInt32(&button.refCount))
	c.Equal(E_ELEMENTNOTAVAILABLE, fragmentSetFocus(button.ifacePtr(ifaceFragment)))
	c.Equal(uintptr(0), button.release())

	// The snapshot memo is dropped too, whichever window's snapshot it was holding: a strong reference to a tree
	// nothing answers from any more would keep every node in it alive for the rest of the process.
	c.Nil(snapshotMemo.tree)

	// A destroyed adapter must not answer with providers it no longer has, and must not blow up if it is told about
	// another snapshot.
	w.Publish(sampleTree(), nil)
	c.Nil(w.providerFor(4))
}

// TestProviderHandsOutOwnedReferences verifies that Provider and Root take a reference on the caller's behalf, while
// the provider map still holds one of its own.
//
// A caller on a UI Automation thread would otherwise be racing the publish that retires the provider: the retirement
// takes the count to zero and unpins the object, and the caller's own AddRef — made after the window's lock was dropped
// — would resurrect memory Go no longer keeps alive and hand it to UI Automation, which dereferences it later.
func TestProviderHandsOutOwnedReferences(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())

	p := w.Provider(5)
	c.NotNil(p)
	c.Equal(int32(2), atomic.LoadInt32(&p.refCount), "the provider map's reference plus the caller's")

	// Node 5 leaves the tree while the caller is still holding what Provider handed it.
	without := sampleTree()
	delete(without.Nodes, 5)
	without.Nodes[3].Children = []accessibility.NodeID{4}
	without.Generation = 2
	w.Publish(without, nil)
	c.True(p.Stale())
	c.Equal(int32(1), atomic.LoadInt32(&p.refCount), "the caller's reference outlives the provider map's")
	c.Equal(uintptr(0), p.release())

	root := w.Root()
	c.NotNil(root)
	c.Equal(int32(2), atomic.LoadInt32(&root.refCount))
	c.Equal(uintptr(1), root.release())

	// Destroying the window gives up the last reference to the fragment root, and hands out nothing further.
	w.Destroy()
	c.Equal(int32(0), atomic.LoadInt32(&root.refCount))
	c.Nil(w.Root())
	c.Nil(w.Provider(4))
}

// TestWindowPatternIsRootOnly verifies that only the fragment root hands out IWindowProvider, however a client asks
// for it.
//
// A nested node that reports role.Dialog is a dialog-shaped panel rather than a window of its own, so answering
// get_CanMaximize, get_WindowVisualState or get_IsTopmost for it would be describing the window that contains it
// through an element that is not it. QueryInterface, GetPatternProvider and the pattern's own methods all have to agree
// about that: a client that reaches a pattern one way and not the other treats the element as broken.
func TestWindowPatternIsRootOnly(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	value, valueAddress := pinnedOut[VARIANT](&pin)
	boolean, booleanAddress := pinnedOut[int32](&pin)
	tree := sampleTree()
	tree.Nodes[7].Role = role.Dialog
	w := newTestWindow(t, tree)
	nested := w.providerFor(7)
	c.NotNil(nested)
	guid := ifaceIIDs[ifaceWindow]
	pin.Pin(&guid)

	c.Equal(w32.COM_E_NOINTERFACE, queryInterface(ifaceSimple, nested.ifacePtr(ifaceSimple),
		uintptr(unsafe.Pointer(&guid)), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(nested.ifacePtr(ifaceSimple),
		uintptr(WindowPatternId), outAddress))
	c.Equal(uintptr(0), *out, "GetPatternProvider must agree with QueryInterface")
	c.Equal(E_NOTSUPPORTED, windowIsModal(nested.ifacePtr(ifaceWindow), booleanAddress))
	c.Equal(E_NOTSUPPORTED, windowInteractionStateValue(nested.ifacePtr(ifaceWindow), booleanAddress))

	// IsDialog is a window-level property, and is answered by the same element that answers the pattern.
	c.Equal(w32.COM_S_OK, simpleGetPropertyValue(nested.ifacePtr(ifaceSimple),
		uintptr(IsDialogPropertyId), valueAddress))
	c.Equal(VT_EMPTY, value.VT)

	// Nor does it report the Window control type, whose required pattern is the one it has just refused: a
	// dialog-shaped panel is a pane, while the root really is a window.
	c.Equal(w32.COM_S_OK, simpleGetPropertyValue(nested.ifacePtr(ifaceSimple),
		uintptr(ControlTypePropertyId), valueAddress))
	c.Equal(int32(PaneControlTypeId), variantInt32(value))
	value.Clear()
	c.Equal(w32.COM_S_OK, simpleGetPropertyValue(w.rootProvider().ifacePtr(ifaceSimple),
		uintptr(ControlTypePropertyId), valueAddress))
	c.Equal(int32(WindowControlTypeId), variantInt32(value))
	value.Clear()

	// The root has the pattern both ways, and every interface it hands out is AddRef'd.
	root := w.rootProvider()
	c.Equal(w32.COM_S_OK, queryInterface(ifaceSimple, root.ifacePtr(ifaceSimple),
		uintptr(unsafe.Pointer(&guid)), outAddress))
	c.Equal(root.ifacePtr(ifaceWindow), *out)
	c.Equal(uintptr(1), root.release())
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(root.ifacePtr(ifaceSimple),
		uintptr(WindowPatternId), outAddress))
	c.Equal(root.ifacePtr(ifaceWindow), *out)
	c.Equal(uintptr(1), root.release())
	c.Equal(w32.COM_S_OK, windowIsModal(root.ifacePtr(ifaceWindow), booleanAddress))
	c.Equal(int32(0), *boolean)
}

// TestPublishKeepsProviders verifies that a publish that changes nothing structural leaves the providers alone: a
// client holding an element across a snapshot must keep holding the same one, or every redraw would invalidate its
// whole view of the window.
func TestPublishKeepsProviders(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, sampleTree())
	before := w.providerFor(4)
	c.NotNil(before)

	next := sampleTree()
	next.Nodes[4].Name = "Renamed"
	next.Generation = 2
	w.Publish(next, nil)

	c.Equal(before, w.providerFor(4))
	c.False(before.Stale())
	c.Equal(next, w.Tree())

	// A nil snapshot is ignored rather than leaving the window with none.
	w.Publish(nil, nil)
	c.Equal(next, w.Tree())
}

// TestGetFocusOnCell verifies that a table with its cell cursor on a cell points a client at that cell, that the cell
// answers the two focus properties the way the element the user is on must, and that asking the cell for the focus
// reaches the widget. NVDA follows a focus event by asking the element whether it really has the keyboard —
// shouldAllowUIAFocusEvent — and a client that lost track asks the fragment root instead, so the two have to agree.
func TestGetFocusOnCell(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	value, valueAddress := pinnedOut[VARIANT](&pin)
	tree := cellCursorTree()
	focusCell(tree, 6, 7, 9)
	w := newTestWindow(t, tree)
	this := w.rootProvider().ifacePtr(ifaceFragmentRoot)

	c.Equal(w32.COM_S_OK, fragmentRootGetFocus(this, outAddress))
	c.Equal(w.providerFor(9).ifacePtr(ifaceFragment), *out, "the cell the cursor is on")
	c.Equal(uintptr(1), w.providerFor(9).release())

	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(w32.COM_S_OK, simpleGetPropertyValue(p.ifacePtr(ifaceSimple), uintptr(id), valueAddress))
		return value
	}
	c.Equal(VT_BOOL, property(9, HasKeyboardFocusPropertyId).VT)
	c.True(variantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(9, IsKeyboardFocusablePropertyId).VT)
	c.True(variantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(7, HasKeyboardFocusPropertyId).VT)
	c.False(variantBool(value), "the row that handed the focus on to the cell claims nothing")
	value.Clear()

	// Asking the cell for the focus is asking to move the cursor onto it, which the widget is told to do.
	c.Equal(w32.COM_S_OK, fragmentSetFocus(w.providerFor(9).ifacePtr(ifaceFragment)))
	requests := w.recorded()
	c.Equal(1, len(requests))
	c.Equal(accessibility.NodeID(9), requests[0].Node)
	c.Equal(accessibility.Focus, requests[0].Action)

	// Moving the cursor to the next column moves what the fragment root reports along with it.
	next := cellCursorTree()
	focusCell(next, 6, 7, 8)
	next.Generation = 2
	w.Publish(next, nil)
	c.Equal(w32.COM_S_OK, fragmentRootGetFocus(this, outAddress))
	c.Equal(w.providerFor(8).ifacePtr(ifaceFragment), *out)
	c.Equal(uintptr(1), w.providerFor(8).release())
}
