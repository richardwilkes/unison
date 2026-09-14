// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

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
	"golang.org/x/sys/windows"
)

// These tests stand in for the UI Automation client the provider is built for. There is no way to make UI Automation
// call a provider from a unit test, so they call the Go functions the vtable slots hold directly, with the this pointer
// adjusted to the interface each one belongs to — which is itself worth testing, since getting that adjustment wrong is
// the failure mode this object layout invites.

// uiaTestOrigin and uiaTestScale are the geometry every test window uses: an origin that is not the screen's own, and a
// scale that is not one, so that a conversion that forgets either shows up.
var (
	uiaTestOrigin = geom.NewPoint(100, 50)
	uiaTestScale  = geom.NewPoint(2, 2)
)

// uiaTestWindow is a UIAWindow along with the action requests its providers have asked for.
type uiaTestWindow struct {
	*UIAWindow
	requestLock sync.Mutex
	requests    []accessibility.ActionRequest
}

// newTestUIAWindow creates the adapter for a window that records action requests instead of performing them, with no
// client listening. The window is destroyed when the test finishes, whether or not the test destroys it itself — a
// second Destroy does nothing — so that every test exercises the teardown and none of them leaks its providers: a
// provider is pinned for the garbage collector while the provider map holds its reference, and only retirement gives
// that reference up.
//
// Cutting the window off from UI Automation matters as much as recording the requests: creating an adapter is the
// window's first publish and Destroy raises Window_WindowClosed and disconnects every provider, so without this the
// tests would make real UI Automation calls on any machine where something is listening — with providers for a window
// handle of zero, a fragment UI Automation can neither host nor resolve. A test that wants to see what would have been
// raised installs a recorder of its own with uiaRecord, which replaces the same variables.
func newTestUIAWindow(t *testing.T, tree *accessibility.Tree) *uiaTestWindow {
	t.Helper()
	uiaSilenceClients(t)
	w := &uiaTestWindow{}
	w.UIAWindow = NewUIAWindow(UIAConfig{Action: w.record}, tree,
		UIAGeometry{Origin: uiaTestOrigin, Scale: uiaTestScale})
	t.Cleanup(w.Destroy)
	return w
}

// newActionlessUIAWindow creates an adapter with nowhere to send action requests, which is what a window the root
// package gave no action hook amounts to: every request a client makes of it must be refused rather than reported as
// done. It is destroyed when the test finishes, for the reason newTestUIAWindow gives.
//
// Nobody is listening while it is created, whatever the test has installed, because creating an adapter is the window's
// first publish and that announces the window: a recorder the test set up to watch something else would otherwise be
// handed a Window_WindowOpened it never asked about.
func newActionlessUIAWindow(t *testing.T, tree *accessibility.Tree) *UIAWindow {
	t.Helper()
	saved := uiaClientsAreListening
	uiaClientsAreListening = func() bool { return false }
	w := NewUIAWindow(UIAConfig{}, tree, UIAGeometry{})
	uiaClientsAreListening = saved
	t.Cleanup(w.Destroy)
	return w
}

// uiaSilenceClients cuts a test off from UI Automation for the duration of one test, restoring every entry point it
// replaced afterwards, so that nothing the test does reaches uiautomationcore.dll.
//
// Three of them have to be replaced rather than only the gate. UiaClientsAreListening is what every raise sits behind,
// and reporting that nobody is listening silences all of them; the other two are made whatever it answers.
// UIAProvider.retire calls UiaDisconnectProvider outside the gate, and the t.Cleanup(w.Destroy) of every test goes
// through it for each provider the test created — handing UI Automation providers it has never seen and letting it call
// back into GetRuntimeId from a thread of its own. UIAWindow.Destroy withdraws the window's provider with
// UiaReturnRawElementProvider, which these windows escape only by using a window handle of zero.
func uiaSilenceClients(t *testing.T) {
	t.Helper()
	savedListening := uiaClientsAreListening
	savedDisconnect := uiaDisconnectProvider
	savedReturn := uiaReturnRawElementProvider
	t.Cleanup(func() {
		uiaClientsAreListening = savedListening
		uiaDisconnectProvider = savedDisconnect
		uiaReturnRawElementProvider = savedReturn
	})
	uiaClientsAreListening = func() bool { return false }
	uiaDisconnectProvider = func(_ unsafe.Pointer) uintptr { return uintptr(COM_S_OK) }
	uiaReturnRawElementProvider = func(_ windows.HWND, _ WPARAM, _ LPARAM, _ unsafe.Pointer) LRESULT { return 0 }
}

// record notes one action request.
func (w *uiaTestWindow) record(request accessibility.ActionRequest) {
	w.requestLock.Lock()
	defer w.requestLock.Unlock()
	w.requests = append(w.requests, request)
}

// recorded returns the action requests made so far.
func (w *uiaTestWindow) recorded() []accessibility.ActionRequest {
	w.requestLock.Lock()
	defer w.requestLock.Unlock()
	return append([]accessibility.ActionRequest(nil), w.requests...)
}

// providerFor returns the provider for one node, handing back the reference Provider took on the caller's behalf.
// Provider is what a UI Automation thread calls, so it must return an owned reference; a test holds the window itself
// and never retires a provider behind its own back, so the provider map's reference is enough to keep the pointer good
// for as long as the test needs it. Every test that cares about the counts themselves calls Provider directly.
func (w *UIAWindow) providerFor(id accessibility.NodeID) *UIAProvider {
	p := w.Provider(id)
	if p != nil {
		p.release()
	}
	return p
}

// rootProvider returns the window's fragment root, handing back the reference Root took, for the reason providerFor
// gives.
func (w *UIAWindow) rootProvider() *UIAProvider {
	root := w.Root()
	if root != nil {
		root.release()
	}
	return root
}

// uiaOut allocates an out-parameter for a COM call and returns it along with its address. The allocation is pinned, so
// the address stays good for as long as the caller holds the pointer, which is exactly the guarantee a provider gets
// when UI Automation is the caller.
func uiaOut[T any](pin *runtime.Pinner) (value *T, address uintptr) {
	value = new(T)
	pin.Pin(value)
	return value, uintptr(unsafe.Pointer(value))
}

// uiaVtblsForTest returns every interface's virtual method table as a slice, in interface order.
func uiaVtblsForTest() [uiaIfaceCount][]uintptr {
	return [uiaIfaceCount][]uintptr{
		uiaIfaceSimple:         uiaSimpleVtbl[:],
		uiaIfaceFragment:       uiaFragmentVtbl[:],
		uiaIfaceFragmentRoot:   uiaFragmentRootVtbl[:],
		uiaIfaceAdviseEvents:   uiaAdviseEventsVtbl[:],
		uiaIfaceWindow:         uiaWindowVtbl[:],
		uiaIfaceInvoke:         uiaInvokeVtbl[:],
		uiaIfaceToggle:         uiaToggleVtbl[:],
		uiaIfaceValue:          uiaValueVtbl[:],
		uiaIfaceRangeValue:     uiaRangeValueVtbl[:],
		uiaIfaceSelection:      uiaSelectionVtbl[:],
		uiaIfaceSelectionItem:  uiaSelectionItemVtbl[:],
		uiaIfaceExpandCollapse: uiaExpandCollapseVtbl[:],
		uiaIfaceScrollItem:     uiaScrollItemVtbl[:],
		uiaIfaceGrid:           uiaGridVtbl[:],
		uiaIfaceGridItem:       uiaGridItemVtbl[:],
		uiaIfaceTable:          uiaTableVtbl[:],
		uiaIfaceTableItem:      uiaTableItemVtbl[:],
	}
}

// TestUIAVtbls verifies that every slot of every virtual method table is filled in and that each table is the size its
// interface declares. An empty slot is a jump to address zero the first time a client calls that method.
func TestUIAVtbls(t *testing.T) {
	c := check.New(t)
	uiaEnsureVtbls()
	for iface, vtbl := range uiaVtblsForTest() {
		c.Equal(uiaIfaceSlots[iface], len(vtbl), "interface %d table size", iface)
		c.Equal(uintptr(unsafe.Pointer(&vtbl[0])), uiaVtblStarts[iface], "interface %d table start", iface)
		for slot, method := range vtbl {
			c.True(method != 0, "interface %d slot %d is empty", iface, slot)
		}
	}
}

// uiaSlotOut is the out-parameter the slot-order tests hand every indirect call. It is as wide as the widest
// out-parameter any slot in this package has — UiaRect's four doubles — so that a slot holding the wrong method, which
// is exactly what those tests exist to catch, cannot write past the end of it while the test is finding that out.
type uiaSlotOut struct {
	buf UiaRect
}

// uiaSlotScratch allocates an out-parameter for an indirect call and pins it, for the reason uiaOut gives.
func uiaSlotScratch(pin *runtime.Pinner) *uiaSlotOut {
	out := &uiaSlotOut{}
	pin.Pin(out)
	return out
}

// fresh zeroes the buffer and returns its address, which is what a slot is handed. Zeroing before every call is what
// keeps a method that writes four bytes where another writes eight from being read as having written the difference.
func (o *uiaSlotOut) fresh() uintptr {
	o.buf = UiaRect{}
	return uintptr(unsafe.Pointer(o))
}

// i32 reads the buffer as the 32-bit integer a BOOL, a count, an index or an enumeration-valued property is.
func (o *uiaSlotOut) i32() int32 {
	return *(*int32)(unsafe.Pointer(o))
}

// f64 reads the buffer as the double every RangeValue measurement is.
func (o *uiaSlotOut) f64() float64 {
	return *(*float64)(unsafe.Pointer(o))
}

// ptr reads the buffer as the pointer an interface out-parameter and a BSTR both are.
func (o *uiaSlotOut) ptr() uintptr {
	return *(*uintptr)(unsafe.Pointer(o))
}

// array reads the buffer as a SAFEARRAY handle.
func (o *uiaSlotOut) array() SAFEARRAY {
	return SAFEARRAY(o.ptr())
}

// variant reads the buffer as the VARIANT GetPropertyValue fills in.
func (o *uiaSlotOut) variant() *VARIANT {
	return (*VARIANT)(unsafe.Pointer(o))
}

// rect reads the buffer as the UiaRect get_BoundingRectangle fills in.
func (o *uiaSlotOut) rect() UiaRect {
	return o.buf
}

// uiaCallSlot calls one method of one interface the way UI Automation does: indirectly, through the slot its virtual
// method table holds, with the this pointer for that interface. method is the index among the interface's own methods,
// so method 0 is the one declared first, after IUnknown's three.
func uiaCallSlot(p *UIAProvider, iface uiaIface, method int, args ...uintptr) uint64 {
	uiaEnsureVtbls()
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, p.ifacePtr(iface))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(uiaVtblsForTest()[iface][uiaUnknownSlots+method], all...)
	return uint64(r)
}

// TestUIAVtblSlotOrder verifies the one thing uiaBuildVtbls calls the whole content of the ABI contract: that method N
// of an interface really sits in slot N of its virtual method table. No other test can. Every other test here calls the
// Go functions the slots were built from, and those answer the same however the slots are ordered, so two methods of
// the same shape swapped — get_CanMaximize for get_CanMinimize, say — would pass the entire suite while a real client
// got one answer where it asked for the other.
//
// Two slots are left out. Both hold assembly thunks, because both take doubles, and doubles arrive in floating-point
// registers that syscall.SyscallN cannot fill; TestUIAThunkSlots checks that those two slots hold the thunks, and
// TestFromPointThunk and TestRangeValueSetValueThunk call them the way UI Automation would.
//
// The twelve control-pattern interfaces are covered by TestUIAPatternVtblSlotOrder in uia_patterns_windows_test.go.
func TestUIAVtblSlotOrder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := uiaSlotScratch(&pin)
	w := newTestUIAWindow(t, sampleTree())
	root := w.rootProvider()
	c.NotNil(root)
	button := w.providerFor(4)
	c.NotNil(button)

	// IRawElementProviderSimple: get_ProviderOptions, GetPatternProvider, GetPropertyValue,
	// get_HostRawElementProvider.
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceSimple, 0, out.fresh()))
	c.Equal(int32(ProviderOptions_ServerSideProvider), out.i32(), "get_ProviderOptions")
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceSimple, 1, uintptr(UIA_InvokePatternId), out.fresh()))
	c.Equal(button.ifacePtr(uiaIfaceInvoke), out.ptr(), "GetPatternProvider")
	c.Equal(uintptr(1), button.release())
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceSimple, 2, uintptr(UIA_NamePropertyId), out.fresh()))
	c.Equal(VT_BSTR, out.variant().VT, "GetPropertyValue")
	c.Equal("One", uiaVariantString(out.variant()))
	out.variant().Clear()
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceSimple, 3, out.fresh()))
	c.Equal(uintptr(0), out.ptr(), "get_HostRawElementProvider: only the fragment root has a host")

	// IRawElementProviderFragment: Navigate, GetRuntimeId, get_BoundingRectangle, GetEmbeddedFragmentRoots, SetFocus,
	// get_FragmentRoot. Node 4 is the first of the window's two buttons, at (0,0 50x20) in a window whose content area
	// starts at (100,50) with two pixels per logical unit.
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceFragment, 0, uintptr(NavigateDirection_NextSibling), out.fresh()))
	sibling := w.providerFor(5)
	c.NotNil(sibling)
	c.Equal(sibling.ifacePtr(uiaIfaceFragment), out.ptr(), "Navigate")
	c.Equal(uintptr(1), sibling.release())
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceFragment, 1, out.fresh()))
	c.Equal([]int32{UiaAppendRuntimeId, 4, 0}, safeArrayToInt32(out.array()), "GetRuntimeId")
	out.array().Destroy()
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceFragment, 2, out.fresh()))
	c.Equal(UiaRect{Left: 100, Top: 50, Width: 100, Height: 40}, out.rect(), "get_BoundingRectangle")
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceFragment, 3, out.fresh()))
	c.Equal(uintptr(0), out.ptr(), "GetEmbeddedFragmentRoots: unison draws every widget itself")
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceFragment, 4))
	c.Equal(1, len(w.recorded()), "SetFocus")
	c.Equal(accessibility.Focus, uiaRequestAt(w, 0).Action)
	c.Equal(accessibility.NodeID(4), uiaRequestAt(w, 0).Node)
	c.Equal(COM_S_OK, uiaCallSlot(button, uiaIfaceFragment, 5, out.fresh()))
	c.Equal(root.ifacePtr(uiaIfaceFragmentRoot), out.ptr(), "get_FragmentRoot")
	c.Equal(uintptr(1), root.release())

	// IRawElementProviderFragmentRoot: ElementProviderFromPoint, which holds a thunk, then GetFocus.
	c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceFragmentRoot, 1, out.fresh()))
	focus := w.providerFor(4)
	c.NotNil(focus)
	c.Equal(focus.ifacePtr(uiaIfaceFragment), out.ptr(), "GetFocus")
	c.Equal(uintptr(1), focus.release())

	// IRawElementProviderAdviseEvents: AdviseEventAdded, AdviseEventRemoved. The two are told apart by which way they
	// move the count.
	c.Equal(int32(0), w.Listeners())
	c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceAdviseEvents, 0, uintptr(UIA_AutomationFocusChangedEventId), 0))
	c.Equal(int32(1), w.Listeners(), "AdviseEventAdded")
	c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceAdviseEvents, 1, uintptr(UIA_AutomationFocusChangedEventId), 0))
	c.Equal(int32(0), w.Listeners(), "AdviseEventRemoved")

	uiaCheckWindowSlotOrder(c, w, root, out)
}

// uiaCheckWindowSlotOrder verifies the slot order of IWindowProvider: SetVisualState, Close, WaitForInputIdle,
// get_CanMaximize, get_CanMinimize, get_IsModal, get_WindowVisualState, get_WindowInteractionState, get_IsTopmost.
//
// Six of the nine take nothing but an out-parameter, so telling them apart takes more than one window: each is asked
// about three, and over those three no two of the six answer the same sequence — CanMaximize says 1,0,0; CanMinimize
// 1,1,1; IsModal 0,1,0; WindowVisualState 0,0,0; WindowInteractionState ready, ready, blocked; and IsTopmost 0,0,1. Any
// two of them swapped therefore shows up as a wrong answer for at least one window.
func uiaCheckWindowSlotOrder(c check.Checker, w *uiaTestWindow, root *UIAProvider, out *uiaSlotOut) {
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(root, uiaIfaceWindow, 0, uintptr(WindowVisualState_Maximized)),
		"SetVisualState")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(root, uiaIfaceWindow, 1), "Close")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(root, uiaIfaceWindow, 2, 100, out.fresh()), "WaitForInputIdle")
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
		c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceWindow, 3, out.fresh()))
		c.Equal(one.canMaximize, out.i32(), "get_CanMaximize, window %d", i)
		c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceWindow, 4, out.fresh()))
		c.Equal(int32(1), out.i32(), "get_CanMinimize, window %d", i)
		c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceWindow, 5, out.fresh()))
		c.Equal(one.isModal, out.i32(), "get_IsModal, window %d", i)
		c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceWindow, 6, out.fresh()))
		c.Equal(WindowVisualState_Normal, WindowVisualState(out.i32()), "get_WindowVisualState, window %d", i)
		c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceWindow, 7, out.fresh()))
		c.Equal(one.interaction, WindowInteractionState(out.i32()), "get_WindowInteractionState, window %d", i)
		c.Equal(COM_S_OK, uiaCallSlot(root, uiaIfaceWindow, 8, out.fresh()))
		c.Equal(one.isTopmost, out.i32(), "get_IsTopmost, window %d", i)
	}
}

// TestUIAProviderThisPointers verifies that the pointer a client holds for each interface recovers the provider it
// belongs to, which every COM method here depends on.
func TestUIAProviderThisPointers(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())
	p := w.providerFor(4)
	c.NotNil(p)
	c.Equal(uintptr(unsafe.Pointer(p)), uintptr(p.Unknown()))
	for iface := uiaIfaceSimple; iface < uiaIfaceCount; iface++ {
		this := p.ifacePtr(iface)
		c.Equal(uintptr(unsafe.Pointer(p))+uiaIfaceOffset(iface), this)
		c.Equal(p, uiaProviderFromThis(this, iface))
	}
}

// TestUIAQueryInterface verifies that a provider hands out the interfaces it implements and refuses the rest, that the
// answer does not depend on which interface it was asked through, and that every interface handed out is AddRef'd.
func TestUIAQueryInterface(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())
	root := w.rootProvider()
	c.NotNil(root)
	button := w.providerFor(4)
	c.NotNil(button)

	query := func(p *UIAProvider, through, wanted uiaIface) uint64 {
		guid := uiaIfaceIIDs[wanted]
		pin.Pin(&guid)
		return uiaQueryInterface(through, p.ifacePtr(through), uintptr(unsafe.Pointer(&guid)), outAddress)
	}

	// The root implements the two provider interfaces plus the three only a fragment root has, and the Window pattern.
	rootIfaces := []uiaIface{
		uiaIfaceSimple, uiaIfaceFragment, uiaIfaceFragmentRoot, uiaIfaceAdviseEvents, uiaIfaceWindow,
	}
	for _, iface := range rootIfaces {
		c.Equal(COM_S_OK, query(root, uiaIfaceSimple, iface), "root wants interface %d", iface)
		c.Equal(root.ifacePtr(iface), *out)
		c.Equal(uintptr(1), root.release())
	}

	// IID_IUnknown is answered with the IRawElementProviderSimple table, whose first three slots are IUnknown's.
	unknown := iidUnknown
	pin.Pin(&unknown)
	c.Equal(COM_S_OK, uiaQueryInterface(uiaIfaceFragment, root.ifacePtr(uiaIfaceFragment),
		uintptr(unsafe.Pointer(&unknown)), outAddress))
	c.Equal(root.ifacePtr(uiaIfaceSimple), *out)
	c.Equal(uintptr(1), root.release())

	// A button is not a fragment root, and supports Invoke and nothing else.
	c.Equal(COM_S_OK, query(button, uiaIfaceFragment, uiaIfaceInvoke))
	c.Equal(button.ifacePtr(uiaIfaceInvoke), *out)
	c.Equal(uintptr(1), button.release())
	refused := []uiaIface{uiaIfaceFragmentRoot, uiaIfaceAdviseEvents, uiaIfaceWindow, uiaIfaceToggle, uiaIfaceValue}
	for _, iface := range refused {
		c.Equal(COM_E_NOINTERFACE, query(button, uiaIfaceSimple, iface), "button refuses interface %d", iface)
		c.Equal(uintptr(0), *out)
	}

	// An interface this package does not implement at all, and a NULL out-parameter.
	other := xos.Must(windows.GUIDFromString("{11111111-2222-3333-4444-555555555555}"))
	pin.Pin(&other)
	c.Equal(COM_E_NOINTERFACE, uiaQueryInterface(uiaIfaceSimple, button.ifacePtr(uiaIfaceSimple),
		uintptr(unsafe.Pointer(&other)), outAddress))
	c.Equal(COM_E_POINTER, uiaQueryInterface(uiaIfaceSimple, button.ifacePtr(uiaIfaceSimple),
		uintptr(unsafe.Pointer(&other)), 0))

	// A NULL interface identifier fails too, and the out-parameter is cleared before that is decided: COM requires
	// QueryInterface to store NULL on every failure, so a caller is never left holding what it passed in. UI Automation
	// never passes one, which is exactly why the order has to be right here rather than discovered later.
	*out = 0xDEAD
	c.Equal(COM_E_POINTER, uiaQueryInterface(uiaIfaceSimple, button.ifacePtr(uiaIfaceSimple), 0, outAddress))
	c.Equal(uintptr(0), *out)
}

// TestUIAProviderReferenceCountLifetime verifies that a provider is unpinned only when its last COM reference goes,
// rather than when the window gives up the one its provider map holds: UI Automation's pointers to a provider are
// invisible to Go, so an early unpin would leave it calling into memory the collector may have reused.
func TestUIAProviderReferenceCountLifetime(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())
	p := w.providerFor(4)
	c.NotNil(p)
	c.Equal(int32(1), atomic.LoadInt32(&p.refCount))

	// QueryInterface through one interface, taking a reference of its own.
	guid := uiaIfaceIIDs[uiaIfaceFragment]
	pin.Pin(&guid)
	c.Equal(COM_S_OK, uiaQueryInterface(uiaIfaceSimple, p.ifacePtr(uiaIfaceSimple),
		uintptr(unsafe.Pointer(&guid)), outAddress))
	c.Equal(p.ifacePtr(uiaIfaceFragment), *out)
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
	c.Equal(uintptr(0), uiaProviderFromThis(*out, uiaIfaceFragment).release())

	// AddRef and Release must report the counts IUnknown promises, whichever interface they arrive through. The
	// reference for that arithmetic is the caller's own, taken through Provider and given back at the end: releasing
	// the provider map's instead would leave a zero-count, unpinned provider in the map for Destroy to retire, which is
	// a state the production code has no way of reaching.
	p = w.Provider(5)
	c.NotNil(p)
	c.Equal(int32(2), atomic.LoadInt32(&p.refCount))
	c.Equal(uintptr(3), uiaProviderFromThis(p.ifacePtr(uiaIfaceFragment), uiaIfaceFragment).addRef())
	c.Equal(uintptr(2), uiaProviderFromThis(p.ifacePtr(uiaIfaceSimple), uiaIfaceSimple).release())
	c.Equal(uintptr(1), p.release())
}

// TestUIAProviderAnchor verifies that a provider is anchored for as long as a COM reference to it exists, and is let go
// of by the release of the last one. The anchor is what keeps the object reachable for the collector: UI Automation's
// pointers to it are invisible to Go, and a retired provider is no longer in the window's provider map either, so
// without it the only thing left pointing at the object would be the object's own pinner. Counts are compared as
// differences, since the set is the package's and other windows may be anchored in it.
func TestUIAProviderAnchor(t *testing.T) {
	c := check.New(t)
	before := uiaLiveProviderCount()
	w := newTestUIAWindow(t, sampleTree())
	p := w.Provider(4)
	c.NotNil(p)
	c.Equal(before+2, uiaLiveProviderCount(), "the fragment root and the provider just created")

	// Destroying the window gives up the provider map's reference to each of them. The fragment root has no other, so
	// it goes; the one this test holds keeps its provider anchored, which is exactly the state a client holding an
	// element for a window that has gone away puts the adapter in.
	w.Destroy()
	c.Equal(before+1, uiaLiveProviderCount())
	c.True(p.Stale())
	c.Equal(uintptr(0), p.release())
	c.Equal(before, uiaLiveProviderCount())
}

// TestUIAProviderOptions verifies that providers report themselves as free-threaded server-side providers and nothing
// else. Asking for COM threading would require the UI thread to pump COM messages, which it cannot always do.
func TestUIAProviderOptions(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[ProviderOptions](&pin)
	w := newTestUIAWindow(t, sampleTree())
	c.Equal(COM_S_OK, uiaSimpleProviderOptions(w.rootProvider().ifacePtr(uiaIfaceSimple), outAddress))
	c.Equal(ProviderOptions_ServerSideProvider, *out)
	c.Equal(COM_E_POINTER, uiaSimpleProviderOptions(0, 0))
}

// TestUIAFragmentNavigate verifies navigation over the unignored tree: sampleTree buries its first two buttons under
// two layers of ignored grouping panels, which must be invisible to a client.
func TestUIAFragmentNavigate(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())

	navigate := func(node accessibility.NodeID, direction NavigateDirection) uintptr {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(COM_S_OK, uiaFragmentNavigate(p.ifacePtr(uiaIfaceFragment), uintptr(direction), outAddress))
		if *out != 0 {
			// The reference Navigate handed over is the caller's, which here is the test. Giving it back at once is
			// what lets the counts be checked at the end; the provider map's own reference keeps the pointer good.
			uiaProviderFromThis(*out, uiaIfaceFragment).release()
		}
		return *out
	}
	fragment := func(node accessibility.NodeID) uintptr {
		p := w.providerFor(node)
		c.NotNil(p)
		return p.ifacePtr(uiaIfaceFragment)
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
	c.Equal(COM_E_POINTER, uiaFragmentNavigate(w.rootProvider().ifacePtr(uiaIfaceFragment), 0, 0))
}

// TestUIAGetRuntimeID verifies the runtime identifiers. The fragment root reports none, so that UI Automation
// identifies it by its window handle; everything else reports one that starts with UiaAppendRuntimeId, so that UI
// Automation prepends the root's own identifier and the result stays unique across the process.
func TestUIAGetRuntimeID(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[SAFEARRAY](&pin)
	w := newTestUIAWindow(t, sampleTree())

	c.Equal(COM_S_OK, uiaFragmentGetRuntimeID(w.rootProvider().ifacePtr(uiaIfaceFragment), outAddress))
	c.Equal(SAFEARRAY(0), *out)

	c.Equal(COM_S_OK, uiaFragmentGetRuntimeID(w.providerFor(4).ifacePtr(uiaIfaceFragment), outAddress))
	c.True(*out != 0)
	c.Equal([]int32{UiaAppendRuntimeId, 4, 0}, safeArrayToInt32(*out))
	out.Destroy()
	c.Equal(COM_E_POINTER, uiaFragmentGetRuntimeID(w.rootProvider().ifacePtr(uiaIfaceFragment), 0))
}

// TestUIABoundingRectangle verifies the conversion from a node's window-local logical bounds to the screen rectangle
// UI Automation asks for, and that a node scrolled out of view reports nothing rather than a rectangle somewhere it
// is not.
func TestUIABoundingRectangle(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[UiaRect](&pin)
	tree := sampleTree()
	w := newTestUIAWindow(t, tree)

	// Node 5 is at (50,0 50x20) in the window, and the window's content area starts at (100,50) with two pixels per
	// logical unit.
	c.Equal(COM_S_OK, uiaFragmentBoundingRectangle(w.providerFor(5).ifacePtr(uiaIfaceFragment), outAddress))
	c.Equal(UiaRect{Left: 200, Top: 50, Width: 100, Height: 40}, *out)

	// Moving the window must move the rectangle without a new snapshot.
	w.SetGeometry(UIAGeometry{Origin: geom.NewPoint(0, 0), Scale: geom.NewPoint(1, 1)})
	c.Equal(COM_S_OK, uiaFragmentBoundingRectangle(w.providerFor(5).ifacePtr(uiaIfaceFragment), outAddress))
	c.Equal(UiaRect{Left: 50, Top: 0, Width: 50, Height: 20}, *out)

	tree.Nodes[5].Offscreen = true
	c.Equal(COM_S_OK, uiaFragmentBoundingRectangle(w.providerFor(5).ifacePtr(uiaIfaceFragment), outAddress))
	c.Equal(UiaRect{}, *out)
	c.Equal(COM_E_POINTER, uiaFragmentBoundingRectangle(w.providerFor(5).ifacePtr(uiaIfaceFragment), 0))
}

// TestUIAFragmentRootAndEmbedded verifies that every element reports the same fragment root and that nothing claims an
// embedded fragment, since unison draws every widget itself.
func TestUIAFragmentRootAndEmbedded(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())
	root := w.rootProvider()

	for _, node := range []accessibility.NodeID{1, 4, 7} {
		p := w.providerFor(node)
		c.Equal(COM_S_OK, uiaFragmentFragmentRoot(p.ifacePtr(uiaIfaceFragment), outAddress))
		c.Equal(root.ifacePtr(uiaIfaceFragmentRoot), *out)
		c.Equal(uintptr(1), root.release())
	}
	c.Equal(COM_S_OK, uiaFragmentGetEmbeddedFragmentRoots(root.ifacePtr(uiaIfaceFragment), outAddress))
	c.Equal(uintptr(0), *out)
}

// TestUIASetFocus verifies that SetFocus asks the window for the focus rather than trying to move it here, and that an
// element that cannot take the focus says so instead of quietly doing nothing.
//
// Three things make it impossible, and each is answered the way the pattern write paths answer it: the element is
// disabled, which is UIA_E_ELEMENTNOTENABLED; it cannot take the focus at all, or does not offer the Focus action,
// which is the snapshot's own statement that nothing would happen; or the window has nowhere to send the request.
// Window.dispatchAccessibilityAction drops a request for an action a node does not offer, so answering S_OK would
// leave a client waiting for a focus event that is never coming.
func TestUIASetFocus(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())

	c.Equal(COM_S_OK, uiaFragmentSetFocus(w.providerFor(4).ifacePtr(uiaIfaceFragment)))
	requests := w.recorded()
	c.Equal(1, len(requests))
	c.Equal(accessibility.NodeID(4), requests[0].Node)
	c.Equal(accessibility.Focus, requests[0].Action)

	// Node 5 is a button that cannot take the focus.
	c.Equal(UIA_E_INVALIDOPERATION, uiaFragmentSetFocus(w.providerFor(5).ifacePtr(uiaIfaceFragment)))
	c.Equal(1, len(w.recorded()))

	// A node that reports itself focusable while offering no Focus action refuses too. The pair is reachable:
	// axSnapshot.resolveFocus marks the node an open menu points at focusable after visit has narrowed a disabled
	// node's actions, and an Accessibility.Callback that sets Disabled leaves Focusable alone.
	actionless := sampleTree()
	actionless.Nodes[4].Actions = 0
	actionless.Generation = 2
	w.Publish(actionless, nil)
	c.Equal(UIA_E_INVALIDOPERATION, uiaFragmentSetFocus(w.providerFor(4).ifacePtr(uiaIfaceFragment)))
	c.Equal(1, len(w.recorded()))

	// A disabled node is refused with the answer every pattern write path gives for one, whatever its actions say.
	disabled := sampleTree()
	disabled.Nodes[4].Disabled = true
	disabled.Generation = 3
	w.Publish(disabled, nil)
	c.Equal(UIA_E_ELEMENTNOTENABLED, uiaFragmentSetFocus(w.providerFor(4).ifacePtr(uiaIfaceFragment)))
	c.Equal(1, len(w.recorded()))

	// A window with nowhere to send actions must refuse rather than report success.
	plain := newActionlessUIAWindow(t, sampleTree())
	c.Equal(UIA_E_INVALIDOPERATION, uiaFragmentSetFocus(plain.providerFor(4).ifacePtr(uiaIfaceFragment)))
}

// TestUIAElementProviderFromPoint verifies the hit test a screen reader's mouse tracking goes through, including the
// conversion from screen pixels to the snapshot's window-local logical units.
func TestUIAElementProviderFromPoint(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())
	this := w.rootProvider().ifacePtr(uiaIfaceFragmentRoot)

	fromPoint := func(x, y float64) uintptr {
		c.Equal(COM_S_OK, uiaFragmentRootElementProviderFromPoint(this, uintptr(math.Float64bits(x)),
			uintptr(math.Float64bits(y)), outAddress))
		return *out
	}

	// Window point (60,10) is inside node 5, and lands at screen (220,70) with this window's geometry.
	c.Equal(w.providerFor(5).ifacePtr(uiaIfaceFragment), fromPoint(220, 70))
	c.Equal(uintptr(1), w.providerFor(5).release())

	// Window point (10,10) is inside node 4, which sits under two ignored groups; a hit on an ignored node is reported
	// as a hit on the nearest unignored ancestor, so nothing ignored can ever come back.
	c.Equal(w.providerFor(4).ifacePtr(uiaIfaceFragment), fromPoint(120, 70))
	c.Equal(uintptr(1), w.providerFor(4).release())

	// Nodes 8 and 9 occupy the same area, with 8 first, so the topmost wins.
	c.Equal(w.providerFor(8).ifacePtr(uiaIfaceFragment), fromPoint(140, 270))
	c.Equal(uintptr(1), w.providerFor(8).release())

	// Outside the window there is nothing to report, which lets UI Automation fall back to the window itself.
	c.Equal(uintptr(0), fromPoint(0, 0))
	c.Equal(COM_E_POINTER, uiaFragmentRootElementProviderFromPoint(this, 0, 0, 0))
}

// TestUIAGetFocus verifies that the fragment root reports the focused element only while its window is the active one:
// pointing a client at an element in a window the user is not looking at makes a screen reader jump away from where the
// user is.
func TestUIAGetFocus(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	tree := sampleTree()
	w := newTestUIAWindow(t, tree)
	this := w.rootProvider().ifacePtr(uiaIfaceFragmentRoot)

	c.Equal(COM_S_OK, uiaFragmentRootGetFocus(this, outAddress))
	c.Equal(w.providerFor(4).ifacePtr(uiaIfaceFragment), *out)
	c.Equal(uintptr(1), w.providerFor(4).release())

	// The window is no longer the active one.
	inactive := sampleTree()
	inactive.Nodes[1].Focused = false
	inactive.Generation = 2
	w.Publish(inactive, nil)
	c.Equal(COM_S_OK, uiaFragmentRootGetFocus(this, outAddress))
	c.Equal(uintptr(0), *out)

	// Nothing in the window holds the focus.
	unfocused := sampleTree()
	unfocused.Focus = 0
	unfocused.Generation = 3
	w.Publish(unfocused, nil)
	c.Equal(COM_S_OK, uiaFragmentRootGetFocus(this, outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(COM_E_POINTER, uiaFragmentRootGetFocus(this, 0))
}

// TestUIAAdviseEvents verifies the listener counter, which exists for diagnostics: whether to raise an event is decided
// by UiaClientsAreListening, not by this.
func TestUIAAdviseEvents(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())
	this := w.rootProvider().ifacePtr(uiaIfaceAdviseEvents)
	c.Equal(int32(0), w.Listeners())
	c.Equal(COM_S_OK, uiaAdviseEventAdded(this, uintptr(UIA_AutomationFocusChangedEventId), 0))
	c.Equal(COM_S_OK, uiaAdviseEventAdded(this, uintptr(UIA_AutomationPropertyChangedEventId), 0))
	c.Equal(int32(2), w.Listeners())
	c.Equal(COM_S_OK, uiaAdviseEventRemoved(this, uintptr(UIA_AutomationFocusChangedEventId), 0))
	c.Equal(int32(1), w.Listeners())
}

// TestUIAWindowPattern verifies the Window pattern the fragment root implements, including that every one of its
// answers comes from the snapshot rather than from a constant: a fixed-size dialog must not be reported as something
// that can be maximized, and a window created with FloatingWindowOption really is topmost.
func TestUIAWindowPattern(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	boolean, booleanAddress := uiaOut[int32](&pin)
	visual, visualAddress := uiaOut[WindowVisualState](&pin)
	interaction, interactionAddress := uiaOut[WindowInteractionState](&pin)
	tree := sampleTree()
	w := newTestUIAWindow(t, tree)
	this := w.rootProvider().ifacePtr(uiaIfaceWindow)

	c.Equal(COM_S_OK, uiaWindowCanMaximize(this, booleanAddress))
	c.Equal(int32(0), *boolean, "sampleTree's window is not resizable, so it has no maximize box")
	c.Equal(COM_S_OK, uiaWindowCanMinimize(this, booleanAddress))
	c.Equal(int32(1), *boolean, "every window unison creates has a minimize box")
	c.Equal(COM_S_OK, uiaWindowIsTopmost(this, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(COM_S_OK, uiaWindowIsModal(this, booleanAddress))
	c.Equal(int32(0), *boolean)
	c.Equal(COM_S_OK, uiaWindowVisualState(this, visualAddress))
	c.Equal(WindowVisualState_Normal, *visual)
	c.Equal(COM_S_OK, uiaWindowInteractionStateValue(this, interactionAddress))
	c.Equal(WindowInteractionState_ReadyForUserInteraction, *interaction)

	// A resizable, floating window reports both, which is what the snapshot records for one created without
	// NotResizableWindowOption and with FloatingWindowOption.
	free := sampleTree()
	free.Nodes[1].Resizable = true
	free.Nodes[1].Floating = true
	free.Generation = 2
	w.Publish(free, nil)
	c.Equal(COM_S_OK, uiaWindowCanMaximize(this, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(COM_S_OK, uiaWindowIsTopmost(this, booleanAddress))
	c.Equal(int32(1), *boolean)

	// A modal window says so, and one that is disabled is disabled because something modal is in front of it.
	modal := sampleTree()
	modal.Nodes[1].Modal = true
	modal.Nodes[1].Disabled = true
	modal.Generation = 3
	w.Publish(modal, nil)
	c.Equal(COM_S_OK, uiaWindowIsModal(this, booleanAddress))
	c.Equal(int32(1), *boolean)
	c.Equal(COM_S_OK, uiaWindowInteractionStateValue(this, interactionAddress))
	c.Equal(WindowInteractionState_BlockedByModalWindow, *interaction)

	// The three methods that would act on the window are not offered.
	c.Equal(UIA_E_NOTSUPPORTED, uiaWindowSetVisualState(this, uintptr(WindowVisualState_Maximized)))
	c.Equal(UIA_E_NOTSUPPORTED, uiaWindowClose(this))
	c.Equal(UIA_E_NOTSUPPORTED, uiaWindowWaitForInputIdle(this, 100, booleanAddress))
	c.Equal(COM_E_POINTER, uiaWindowCanMaximize(this, 0))

	// Every getter goes through the element rather than answering blind, so a client holding an IWindowProvider for a
	// root that has since been retired is told the element is gone rather than handed a fabricated answer.
	root := w.Root() // Stand in for the reference such a client would be holding.
	defer root.release()
	w.Destroy()
	c.True(root.Stale())
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaWindowCanMaximize(this, booleanAddress))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaWindowCanMinimize(this, booleanAddress))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaWindowIsModal(this, booleanAddress))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaWindowVisualState(this, visualAddress))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaWindowInteractionStateValue(this, interactionAddress))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaWindowIsTopmost(this, booleanAddress))
}

// TestUIAGetPatternProvider verifies that GetPatternProvider and QueryInterface agree, which a client that reaches a
// pattern both ways depends on, and that the interface it is handed is one the pattern's methods can be called on.
func TestUIAGetPatternProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())
	button := w.providerFor(4)

	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(button.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_InvokePatternId), outAddress))
	c.Equal(button.ifacePtr(uiaIfaceInvoke), *out)
	c.Equal(uintptr(1), button.release())

	// The interface that came back is the one the pattern's methods answer on; see uia_patterns_windows_test.go for
	// what each of them answers.
	c.Equal(COM_S_OK, uiaInvokeInvoke(button.ifacePtr(uiaIfaceInvoke)))
	c.Equal(1, len(w.recorded()))
	c.Equal(accessibility.Press, w.recorded()[0].Action)

	// A pattern the element does not support, and one this package does not implement, are both answered with a NULL
	// interface and S_OK rather than with an error.
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(button.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_TogglePatternId), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(button.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_ScrollPatternId), outAddress))
	c.Equal(uintptr(0), *out)

	// The root is the only element with the Window pattern.
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(w.rootProvider().ifacePtr(uiaIfaceSimple),
		uintptr(UIA_WindowPatternId), outAddress))
	c.Equal(w.rootProvider().ifacePtr(uiaIfaceWindow), *out)
	c.Equal(uintptr(1), w.rootProvider().release())
	c.Equal(COM_E_POINTER, uiaSimpleGetPatternProvider(button.ifacePtr(uiaIfaceSimple), 0, 0))
}

// TestUIAHostRawElementProvider verifies that only the fragment root claims a host provider. Everything else answers
// NULL, which is what says "I am part of a fragment rather than a window of my own".
//
// The root's own answer is the one branch of any provider method that calls a real UI Automation entry point and
// transfers a COM reference out of it, so UiaHostProviderFromHwnd is stood in for here rather than called: a test has
// no window for it to answer about, and the reference it hands back would be a real one to release.
func TestUIAHostRawElementProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, sampleTree())
	c.Equal(COM_S_OK, uiaSimpleHostRawElementProvider(w.providerFor(4).ifacePtr(uiaIfaceSimple), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(COM_E_POINTER, uiaSimpleHostRawElementProvider(w.providerFor(4).ifacePtr(uiaIfaceSimple), 0))

	host := &Unknown{}
	pin.Pin(host)
	asked := windows.HWND(0)
	saved := uiaHostProviderFromHwnd
	t.Cleanup(func() { uiaHostProviderFromHwnd = saved })
	uiaHostProviderFromHwnd = func(hwnd windows.HWND) (*Unknown, uintptr) {
		asked = hwnd
		return host, uintptr(COM_S_OK)
	}

	root := w.rootProvider()
	c.Equal(COM_S_OK, uiaSimpleHostRawElementProvider(root.ifacePtr(uiaIfaceSimple), outAddress))
	c.Equal(uintptr(unsafe.Pointer(host)), *out, "the reference the call returned is handed straight to the caller")
	c.Equal(w.HWND(), asked, "and it is asked about this window")

	// A failure is reported as it came back, with the out-parameter left NULL rather than holding half an answer.
	uiaHostProviderFromHwnd = func(_ windows.HWND) (*Unknown, uintptr) {
		return nil, uintptr(uint32(0x80004005)) // E_FAIL
	}
	c.Equal(uint64(0x80004005), uiaSimpleHostRawElementProvider(root.ifacePtr(uiaIfaceSimple), outAddress))
	c.Equal(uintptr(0), *out)

	// A client still holding the root after the window has been destroyed must be told the element is gone. Windows
	// reuses window handles, so asking about this one afterwards could describe some other window entirely.
	w.Destroy()
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaSimpleHostRawElementProvider(root.ifacePtr(uiaIfaceSimple), outAddress))
	c.Equal(uintptr(0), *out)
}

// uiaVariantString returns the contents of a VT_BSTR VARIANT.
func uiaVariantString(value *VARIANT) string {
	return BSTRToString(BSTR(value.Val))
}

// uiaVariantBool returns the contents of a VT_BOOL VARIANT.
func uiaVariantBool(value *VARIANT) bool {
	return int16(uint16(value.Val)) == VARIANT_TRUE
}

// uiaVariantInt32 returns the contents of a VT_I4 VARIANT.
func uiaVariantInt32(value *VARIANT) int32 {
	return int32(uint32(value.Val))
}

// TestUIAGetPropertyValue verifies the properties a provider answers, including the ones only the fragment root has and
// the rule that an unknown or absent property is an empty VARIANT rather than an error.
func TestUIAGetPropertyValue(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := uiaOut[VARIANT](&pin)
	tree := sampleTree()
	tree.Nodes[4].Description = "The first button"
	tree.Nodes[4].Shortcut = "Ctrl+1"
	tree.Nodes[4].DescribedBy = []accessibility.NodeID{6}
	w := newTestUIAWindow(t, tree)

	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(p.ifacePtr(uiaIfaceSimple), uintptr(id), valueAddress))
		return value
	}

	c.Equal(VT_BSTR, property(4, UIA_NamePropertyId).VT)
	c.Equal("One", uiaVariantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, UIA_HelpTextPropertyId).VT)
	c.Equal("The first button", uiaVariantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, UIA_FullDescriptionPropertyId).VT)
	c.Equal("The first button", uiaVariantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, UIA_AcceleratorKeyPropertyId).VT)
	c.Equal("Ctrl+1", uiaVariantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, UIA_AutomationIdPropertyId).VT)
	c.Equal("4", uiaVariantString(value))
	value.Clear()

	c.Equal(VT_BSTR, property(4, UIA_FrameworkIdPropertyId).VT)
	c.Equal(UiaFrameworkID, uiaVariantString(value))
	value.Clear()

	c.Equal(VT_I4, property(4, UIA_ControlTypePropertyId).VT)
	c.Equal(int32(UIA_ButtonControlTypeId), uiaVariantInt32(value))
	value.Clear()

	c.Equal(VT_BOOL, property(4, UIA_IsEnabledPropertyId).VT)
	c.True(uiaVariantBool(value))
	value.Clear()

	c.Equal(VT_BOOL, property(4, UIA_IsKeyboardFocusablePropertyId).VT)
	c.True(uiaVariantBool(value))
	value.Clear()

	// The node holds the focus and its window is the active one, which is what HasKeyboardFocus means.
	c.Equal(VT_BOOL, property(4, UIA_HasKeyboardFocusPropertyId).VT)
	c.True(uiaVariantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(5, UIA_HasKeyboardFocusPropertyId).VT)
	c.False(uiaVariantBool(value))
	value.Clear()

	// The root must not claim it alongside the control that really has it. Its Focused flag says the window is active,
	// not that the window itself is where typing goes, and it pairs with an IsKeyboardFocusable of false, which is a
	// combination a client cannot make sense of.
	c.Equal(VT_BOOL, property(1, UIA_HasKeyboardFocusPropertyId).VT)
	c.False(uiaVariantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(1, UIA_IsKeyboardFocusablePropertyId).VT)
	c.False(uiaVariantBool(value))
	value.Clear()

	// A label that names another element is left out of the content view, so that a screen reader does not say it
	// twice; everything else that is not ignored is in both views.
	c.Equal(VT_BOOL, property(6, UIA_IsContentElementPropertyId).VT)
	c.False(uiaVariantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(6, UIA_IsControlElementPropertyId).VT)
	c.True(uiaVariantBool(value))
	value.Clear()
	c.Equal(VT_BOOL, property(4, UIA_IsContentElementPropertyId).VT)
	c.True(uiaVariantBool(value))
	value.Clear()

	// LabeledBy is one element; DescribedBy is an array of them. Both add a reference per element handed out, which
	// clearing the VARIANT gives back.
	label := w.providerFor(6)
	c.Equal(VT_UNKNOWN, property(4, UIA_LabeledByPropertyId).VT)
	c.Equal(uintptr(label.Unknown()), uintptr(value.Val))
	c.Equal(int32(2), atomic.LoadInt32(&label.refCount))
	value.Clear()
	c.Equal(int32(1), atomic.LoadInt32(&label.refCount))

	c.Equal(VT_ARRAY|VT_UNKNOWN, property(4, UIA_DescribedByPropertyId).VT)
	c.Equal(int32(2), atomic.LoadInt32(&label.refCount))
	value.Clear()
	c.Equal(int32(1), atomic.LoadInt32(&label.refCount))

	// Only the fragment root answers the window-level properties.
	c.Equal(VT_BOOL, property(1, UIA_IsDialogPropertyId).VT)
	c.False(uiaVariantBool(value))
	value.Clear()
	c.Equal(VT_I4, property(1, UIA_NativeWindowHandlePropertyId).VT)
	c.Equal(int32(0), uiaVariantInt32(value))
	value.Clear()
	c.Equal(VT_EMPTY, property(4, UIA_IsDialogPropertyId).VT)
	c.Equal(VT_EMPTY, property(4, UIA_NativeWindowHandlePropertyId).VT)

	// The live setting is answered by nobody: a client consults it only for an element it has been given a
	// LiveRegionChanged event for, and this package raises that for nothing — an announcement goes out as a
	// notification event, carrying its own ordering.
	c.Equal(VT_EMPTY, property(1, UIA_LiveSettingPropertyId).VT)
	c.Equal(VT_EMPTY, property(4, UIA_LiveSettingPropertyId).VT)

	// Properties with nothing to report, and properties this provider never answers, are empty rather than an error.
	c.Equal(VT_EMPTY, property(5, UIA_NamePropertyId+1000).VT)
	c.Equal(VT_EMPTY, property(5, UIA_HelpTextPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, UIA_LevelPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, UIA_ItemStatusPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, UIA_LabeledByPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, UIA_DescribedByPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, UIA_PositionInSetPropertyId).VT)
	c.Equal(VT_EMPTY, property(5, UIA_LocalizedControlTypePropertyId).VT)

	// The heading level of something that is not a heading is a value of its own rather than nothing.
	c.Equal(VT_I4, property(5, UIA_HeadingLevelPropertyId).VT)
	c.Equal(int32(HeadingLevel_None), uiaVariantInt32(value))
	value.Clear()

	c.Equal(COM_E_POINTER, uiaSimpleGetPropertyValue(w.providerFor(5).ifacePtr(uiaIfaceSimple), 0, 0))
}

// TestUIAHelpTextCarriesPlaceholder verifies that the watermark of a field with no description of its own is reported
// as its help text, which is the conventional carrier for one and the only thing that keeps an unnamed search field
// from being announced as a bare "edit". A description wins when a node has both: it is what the application said
// about the control, while the watermark is a hint the widget put in its own empty content, and FullDescription — the
// other property answered from the description — never carries the watermark at all.
func TestUIAHelpTextCarriesPlaceholder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := uiaOut[VARIANT](&pin)
	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2, 3, 4}},
		&accessibility.Node{ID: 2, Role: role.TextField, Placeholder: "Search"},
		&accessibility.Node{ID: 3, Role: role.TextField, Placeholder: "Search", Description: "Filters the list"},
		&accessibility.Node{ID: 4, Role: role.TextField},
	)
	w := newTestUIAWindow(t, tree)
	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(p.ifacePtr(uiaIfaceSimple), uintptr(id), valueAddress))
		return value
	}

	c.Equal(VT_BSTR, property(2, UIA_HelpTextPropertyId).VT)
	c.Equal("Search", uiaVariantString(value))
	value.Clear()
	c.Equal(VT_EMPTY, property(2, UIA_FullDescriptionPropertyId).VT, "a watermark is not a full description")

	c.Equal(VT_BSTR, property(3, UIA_HelpTextPropertyId).VT)
	c.Equal("Filters the list", uiaVariantString(value), "a description wins over a watermark")
	value.Clear()
	c.Equal(VT_BSTR, property(3, UIA_FullDescriptionPropertyId).VT)
	c.Equal("Filters the list", uiaVariantString(value))
	value.Clear()

	c.Equal(VT_EMPTY, property(4, UIA_HelpTextPropertyId).VT, "a field with neither says nothing")
}

// TestUIAValuePropertyReadsNodeValue verifies that the Value pattern's property is answered through GetPropertyValue
// as well as through IValueProvider::get_Value, and only by an element that has the pattern.
//
// A table cell is what needs it: Table.axAddRow clears the name of a cell that holds one widget and puts that widget's
// state into the cell's value, precisely so that a change to the widget is a change to the cell, and a screen reader
// reading across a row asks the cell for its value.
func TestUIAValuePropertyReadsNodeValue(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := uiaOut[VARIANT](&pin)
	out, outAddress := uiaOut[uintptr](&pin)
	tree := tableTree()
	tree.Nodes[8].Name = ""
	tree.Nodes[8].Value = "checked"
	w := newTestUIAWindow(t, tree)
	property := func(node accessibility.NodeID, id PropertyID) *VARIANT {
		p := w.providerFor(node)
		c.NotNil(p)
		c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(p.ifacePtr(uiaIfaceSimple), uintptr(id), valueAddress))
		return value
	}

	c.Equal(VT_BSTR, property(8, UIA_ValueValuePropertyId).VT)
	c.Equal("checked", uiaVariantString(value))
	value.Clear()

	// The pattern comes with it, both ways a client can reach one, and the value cannot be set through it: a cell
	// offers no SetValue action.
	cell := w.providerFor(8)
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(cell.ifacePtr(uiaIfaceSimple), uintptr(UIA_ValuePatternId),
		outAddress))
	c.Equal(cell.ifacePtr(uiaIfaceValue), *out)
	c.Equal(uintptr(1), cell.release())
	c.Equal(COM_S_OK, uiaValueValue(cell.ifacePtr(uiaIfaceValue), outAddress))
	c.Equal("checked", BSTRToString(BSTR(*out)))
	BSTR(*out).Free()
	boolean, booleanAddress := uiaOut[int32](&pin)
	c.Equal(COM_S_OK, uiaValueIsReadOnly(cell.ifacePtr(uiaIfaceValue), booleanAddress))
	c.Equal(int32(1), *boolean)

	// A cell whose name carries its content has no value, so it has no Value pattern and nothing to report through it.
	c.Equal(VT_EMPTY, property(9, UIA_ValueValuePropertyId).VT)
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(w.providerFor(9).ifacePtr(uiaIfaceSimple),
		uintptr(UIA_ValuePatternId), outAddress))
	c.Equal(uintptr(0), *out)

	// Neither does an element with no value of any kind, however the property is asked for.
	c.Equal(VT_EMPTY, property(7, UIA_ValueValuePropertyId).VT)
}

// TestUIAStaleProvider verifies what a client holding an element for something that has been destroyed is told. Every
// method must report that the element is no longer available rather than answering from a snapshot that no longer holds
// it, and the provider must stay alive while the client still holds it.
func TestUIAStaleProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	value, valueAddress := uiaOut[VARIANT](&pin)
	rect, rectAddress := uiaOut[UiaRect](&pin)
	array, arrayAddress := uiaOut[SAFEARRAY](&pin)
	w := newTestUIAWindow(t, sampleTree())
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

	simple := p.ifacePtr(uiaIfaceSimple)
	fragment := p.ifacePtr(uiaIfaceFragment)
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaSimpleGetPropertyValue(simple, uintptr(UIA_NamePropertyId), valueAddress))
	c.Equal(VT_EMPTY, value.VT)
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaSimpleGetPatternProvider(simple, uintptr(UIA_InvokePatternId), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaFragmentNavigate(fragment, uintptr(NavigateDirection_Parent), outAddress))
	// GetRuntimeId is the one exception: UiaDisconnectProvider asks for it while retiring the provider, so it has to
	// keep answering, and the identifier is derived from the node id alone rather than from the tree.
	c.Equal(COM_S_OK, uiaFragmentGetRuntimeID(fragment, arrayAddress))
	c.Equal([]int32{UiaAppendRuntimeId, 5, 0}, safeArrayToInt32(*array))
	array.Destroy()
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaFragmentBoundingRectangle(fragment, rectAddress))
	c.Equal(UiaRect{}, *rect)
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaFragmentSetFocus(fragment))
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaSimpleHostRawElementProvider(simple, outAddress))
	c.Equal(uintptr(0), *out)

	// A stale element supports no pattern interface, but it is still an IUnknown and an IRawElementProviderSimple, and
	// it still describes how it wants to be called.
	guid := uiaIfaceIIDs[uiaIfaceInvoke]
	pin.Pin(&guid)
	c.Equal(COM_E_NOINTERFACE, uiaQueryInterface(uiaIfaceSimple, simple, uintptr(unsafe.Pointer(&guid)), outAddress))
	simpleGUID := uiaIfaceIIDs[uiaIfaceSimple]
	pin.Pin(&simpleGUID)
	c.Equal(COM_S_OK, uiaQueryInterface(uiaIfaceSimple, simple, uintptr(unsafe.Pointer(&simpleGUID)), outAddress))
	c.Equal(simple, *out)
	c.Equal(uintptr(1), p.release())

	// Nodes that are still in the tree are untouched, and the fragment root is never retired by a publish.
	c.NotNil(w.providerFor(4))
	c.False(w.providerFor(4).Stale())
	c.NotNil(w.rootProvider())
	c.False(w.rootProvider().Stale())
}

// TestUIAProviderLookup verifies which nodes have providers at all. An ignored node is spliced out of the tree a client
// sees, so nothing can ask about one.
func TestUIAProviderLookup(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())
	c.Nil(w.providerFor(0))
	c.Nil(w.providerFor(2), "an ignored node has no provider")
	c.Nil(w.providerFor(3), "an ignored node has no provider")
	c.Nil(w.providerFor(999), "a node that is not in the tree has no provider")
	c.NotNil(w.providerFor(1))
	c.Equal(w.providerFor(4), w.providerFor(4), "a second lookup returns the same provider")
	c.Equal(w.rootProvider(), w.providerFor(1))
	c.Equal(unsafe.Pointer(&w.rootProvider().vtbls[uiaIfaceSimple]), w.RootUnknown())
}

// TestUIADestroy verifies that the adapter gives up every provider as its window is destroyed, and that a client still
// holding one is told the element is gone rather than left pointing at memory nothing owns.
func TestUIADestroy(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())
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
	c.Equal(UIA_E_ELEMENTNOTAVAILABLE, uiaFragmentSetFocus(button.ifacePtr(uiaIfaceFragment)))
	c.Equal(uintptr(0), button.release())

	// The table-header memo is dropped too, whichever window's snapshot it was holding: a strong reference to a tree
	// nothing answers from any more would keep every node in it alive for the rest of the process.
	c.Nil(uiaHeaderMemo.tree)

	// A destroyed adapter must not answer with providers it no longer has, and must not blow up if it is told about
	// another snapshot.
	w.Publish(sampleTree(), nil)
	c.Nil(w.providerFor(4))
}

// TestUIAProviderHandsOutOwnedReferences verifies that Provider and Root take a reference on the caller's behalf, while
// the provider map still holds one of its own.
//
// A caller on a UI Automation thread would otherwise be racing the publish that retires the provider: the retirement
// takes the count to zero and unpins the object, and the caller's own AddRef — made after the window's lock was dropped
// — would resurrect memory Go no longer keeps alive and hand it to UI Automation, which dereferences it later.
func TestUIAProviderHandsOutOwnedReferences(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())

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

// TestUIAWindowPatternIsRootOnly verifies that only the fragment root hands out IWindowProvider, however a client asks
// for it.
//
// A nested node that reports role.Dialog is a dialog-shaped panel rather than a window of its own, so answering
// get_CanMaximize, get_WindowVisualState or get_IsTopmost for it would be describing the window that contains it
// through an element that is not it. QueryInterface, GetPatternProvider and the pattern's own methods all have to agree
// about that: a client that reaches a pattern one way and not the other treats the element as broken.
func TestUIAWindowPatternIsRootOnly(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	value, valueAddress := uiaOut[VARIANT](&pin)
	boolean, booleanAddress := uiaOut[int32](&pin)
	tree := sampleTree()
	tree.Nodes[7].Role = role.Dialog
	w := newTestUIAWindow(t, tree)
	nested := w.providerFor(7)
	c.NotNil(nested)
	guid := uiaIfaceIIDs[uiaIfaceWindow]
	pin.Pin(&guid)

	c.Equal(COM_E_NOINTERFACE, uiaQueryInterface(uiaIfaceSimple, nested.ifacePtr(uiaIfaceSimple),
		uintptr(unsafe.Pointer(&guid)), outAddress))
	c.Equal(uintptr(0), *out)
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(nested.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_WindowPatternId), outAddress))
	c.Equal(uintptr(0), *out, "GetPatternProvider must agree with QueryInterface")
	c.Equal(UIA_E_NOTSUPPORTED, uiaWindowIsModal(nested.ifacePtr(uiaIfaceWindow), booleanAddress))
	c.Equal(UIA_E_NOTSUPPORTED, uiaWindowInteractionStateValue(nested.ifacePtr(uiaIfaceWindow), booleanAddress))

	// IsDialog is a window-level property, and is answered by the same element that answers the pattern.
	c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(nested.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_IsDialogPropertyId), valueAddress))
	c.Equal(VT_EMPTY, value.VT)

	// Nor does it report the Window control type, whose required pattern is the one it has just refused: a
	// dialog-shaped panel is a pane, while the root really is a window.
	c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(nested.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_ControlTypePropertyId), valueAddress))
	c.Equal(int32(UIA_PaneControlTypeId), uiaVariantInt32(value))
	value.Clear()
	c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(w.rootProvider().ifacePtr(uiaIfaceSimple),
		uintptr(UIA_ControlTypePropertyId), valueAddress))
	c.Equal(int32(UIA_WindowControlTypeId), uiaVariantInt32(value))
	value.Clear()

	// The root has the pattern both ways, and every interface it hands out is AddRef'd.
	root := w.rootProvider()
	c.Equal(COM_S_OK, uiaQueryInterface(uiaIfaceSimple, root.ifacePtr(uiaIfaceSimple),
		uintptr(unsafe.Pointer(&guid)), outAddress))
	c.Equal(root.ifacePtr(uiaIfaceWindow), *out)
	c.Equal(uintptr(1), root.release())
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(root.ifacePtr(uiaIfaceSimple),
		uintptr(UIA_WindowPatternId), outAddress))
	c.Equal(root.ifacePtr(uiaIfaceWindow), *out)
	c.Equal(uintptr(1), root.release())
	c.Equal(COM_S_OK, uiaWindowIsModal(root.ifacePtr(uiaIfaceWindow), booleanAddress))
	c.Equal(int32(0), *boolean)
}

// TestUIAPublishKeepsProviders verifies that a publish that changes nothing structural leaves the providers alone: a
// client holding an element across a snapshot must keep holding the same one, or every redraw would invalidate its
// whole view of the window.
func TestUIAPublishKeepsProviders(t *testing.T) {
	c := check.New(t)
	w := newTestUIAWindow(t, sampleTree())
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
