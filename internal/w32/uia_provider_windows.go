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
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"golang.org/x/sys/windows"
)

// This file holds the COM object a UI Automation client talks to. One Go struct implements every interface the provider
// needs, laid out the way a C++ object with multiple base classes is: it begins with one virtual method table pointer
// per interface, so the pointer a client holds for an interface is the address of that interface's slot rather than the
// address of the object, and each method subtracts its own interface's offset to find the object again. See
// uia_layout.go for the interface order and the arithmetic.
//
// Every method answers from the window's immutable snapshot, never from a live panel, which is what lets the providers
// declare themselves free-threaded: UI Automation calls in on whichever thread it likes while the UI thread carries on,
// possibly sitting in a modal loop where it could not pump COM messages even if it wanted to. The only traffic in the
// other direction is an action request, which goes to the root package to be run on the UI thread.
//
// A provider outlives the node it describes. When a node leaves the tree its provider is marked stale and answers
// UIA_E_ELEMENTNOTAVAILABLE, which is what a client holding an element for something that has been destroyed must be
// told, and it stays alive — pinned, since UI Automation holds a raw pointer to it — until the last COM reference is
// released.

// uiaIfaceIIDs holds the interface identifier a client asks for each interface by, indexed by uiaIface. IID_IUnknown is
// deliberately absent: it is answered by handing out the IRawElementProviderSimple pointer, since that table's first
// three slots are the IUnknown methods.
var uiaIfaceIIDs = [uiaIfaceCount]windows.GUID{
	uiaIfaceSimple:         xos.Must(windows.GUIDFromString("{d6dd68d1-86fd-4332-8666-9abedea2d24c}")),
	uiaIfaceFragment:       xos.Must(windows.GUIDFromString("{f7063da8-8359-439c-9297-bbc5299a7d87}")),
	uiaIfaceFragmentRoot:   xos.Must(windows.GUIDFromString("{620ce2a5-ab8f-40a9-86cb-de3c75599b58}")),
	uiaIfaceAdviseEvents:   xos.Must(windows.GUIDFromString("{a407b27b-0f6d-4427-9292-473c7bf93258}")),
	uiaIfaceWindow:         xos.Must(windows.GUIDFromString("{987df77b-db06-4d77-8f8a-86a9c3bb90b9}")),
	uiaIfaceInvoke:         xos.Must(windows.GUIDFromString("{54fcb24b-e18e-47a2-b4d3-eccbe77599a2}")),
	uiaIfaceToggle:         xos.Must(windows.GUIDFromString("{56d00bd0-c4f4-433c-a836-1a52a57e0892}")),
	uiaIfaceValue:          xos.Must(windows.GUIDFromString("{c7935180-6fb3-4201-b174-7df73adbf64a}")),
	uiaIfaceRangeValue:     xos.Must(windows.GUIDFromString("{36dc7aef-33e6-4691-afe1-2be7274b3d33}")),
	uiaIfaceSelection:      xos.Must(windows.GUIDFromString("{fb8b03af-3bdf-48d4-bd36-1a65793be168}")),
	uiaIfaceSelectionItem:  xos.Must(windows.GUIDFromString("{2acad808-b2d4-452d-a407-91ff1ad167b2}")),
	uiaIfaceExpandCollapse: xos.Must(windows.GUIDFromString("{d847d3a5-cab0-4a98-8c32-ecb45c59ad24}")),
	uiaIfaceScrollItem:     xos.Must(windows.GUIDFromString("{2360c714-4bf1-4b26-ba65-9b21316127eb}")),
	uiaIfaceGrid:           xos.Must(windows.GUIDFromString("{b17d6187-0907-464b-a168-0ef17a1572b1}")),
	uiaIfaceGridItem:       xos.Must(windows.GUIDFromString("{d02541f1-fb81-4d64-ae32-f520f8a6dbd1}")),
	uiaIfaceTable:          xos.Must(windows.GUIDFromString("{9c860395-97b3-490a-b52a-858cc22af166}")),
	uiaIfaceTableItem:      xos.Must(windows.GUIDFromString("{b9734fa6-771f-4d78-9c90-2517999349cd}")),
}

// The virtual method tables, one per interface, shared by every provider in the process. They are built on first use
// rather than at startup: a window that no assistive technology ever asks about must cost nothing, and building them
// means asking the Go runtime for a little over a hundred callback trampolines.
var (
	uiaVtblOnce           sync.Once
	uiaVtblStarts         [uiaIfaceCount]uintptr
	uiaSimpleVtbl         [uiaSimpleSlots]uintptr
	uiaFragmentVtbl       [uiaFragmentSlots]uintptr
	uiaFragmentRootVtbl   [uiaFragmentRootSlots]uintptr
	uiaAdviseEventsVtbl   [uiaAdviseEventsSlots]uintptr
	uiaWindowVtbl         [uiaWindowSlots]uintptr
	uiaInvokeVtbl         [uiaInvokeSlots]uintptr
	uiaToggleVtbl         [uiaToggleSlots]uintptr
	uiaValueVtbl          [uiaValueSlots]uintptr
	uiaRangeValueVtbl     [uiaRangeValueSlots]uintptr
	uiaSelectionVtbl      [uiaSelectionSlots]uintptr
	uiaSelectionItemVtbl  [uiaSelectionItemSlots]uintptr
	uiaExpandCollapseVtbl [uiaExpandCollapseSlots]uintptr
	uiaScrollItemVtbl     [uiaScrollItemSlots]uintptr
	uiaGridVtbl           [uiaGridSlots]uintptr
	uiaGridItemVtbl       [uiaGridItemSlots]uintptr
	uiaTableVtbl          [uiaTableSlots]uintptr
	uiaTableItemVtbl      [uiaTableItemSlots]uintptr
)

// uiaEnsureVtbls builds every virtual method table, once per process.
func uiaEnsureVtbls() {
	uiaVtblOnce.Do(uiaBuildVtbls)
}

// uiaBuildVtbls fills in every interface's virtual method table. The order of the methods handed to uiaBuildVtbl is the
// order the interface declares them in and is the whole content of the ABI contract with UI Automation: a method in the
// wrong slot is called with another method's arguments.
func uiaBuildVtbls() {
	uiaBuildVtbl(uiaIfaceSimple, uiaSimpleVtbl[:],
		uiaSimpleProviderOptions,
		uiaSimpleGetPatternProvider,
		uiaSimpleGetPropertyValue,
		uiaSimpleHostRawElementProvider,
	)
	uiaBuildVtbl(uiaIfaceFragment, uiaFragmentVtbl[:],
		uiaFragmentNavigate,
		uiaFragmentGetRuntimeID,
		uiaFragmentBoundingRectangle,
		uiaFragmentGetEmbeddedFragmentRoots,
		uiaFragmentSetFocus,
		uiaFragmentFragmentRoot,
	)
	// ElementProviderFromPoint takes two doubles, which syscall.NewCallback cannot describe, so its slot holds an
	// assembly thunk that moves the floating-point registers into the integer argument registers the callback reads.
	// See uia_thunk_windows.go, including what to do if a thunk ever misbehaves on a target.
	uiaFromPointCallback = windows.NewCallback(uiaFragmentRootElementProviderFromPoint)
	uiaBuildVtbl(uiaIfaceFragmentRoot, uiaFragmentRootVtbl[:],
		uiaFromPointSlot(),
		uiaFragmentRootGetFocus,
	)
	uiaBuildVtbl(uiaIfaceAdviseEvents, uiaAdviseEventsVtbl[:],
		uiaAdviseEventAdded,
		uiaAdviseEventRemoved,
	)
	uiaBuildVtbl(uiaIfaceWindow, uiaWindowVtbl[:],
		uiaWindowSetVisualState,
		uiaWindowClose,
		uiaWindowWaitForInputIdle,
		uiaWindowCanMaximize,
		uiaWindowCanMinimize,
		uiaWindowIsModal,
		uiaWindowVisualState,
		uiaWindowInteractionStateValue,
		uiaWindowIsTopmost,
	)
	uiaBuildPatternVtbls()
}

// uiaBuildVtbl fills in one interface's virtual method table. The three IUnknown slots come first and are built here
// rather than shared, because each needs to know which interface's pointer it will be called through in order to find
// the provider; the rest are the methods, in declaration order. A method may be given either as a function, which is
// wrapped in a callback, or as a uintptr that is already the address of a native entry point.
func uiaBuildVtbl(iface uiaIface, vtbl []uintptr, methods ...any) {
	vtbl[0] = windows.NewCallback(func(this, riid, out uintptr) uint64 {
		return uiaQueryInterface(iface, this, riid, out)
	})
	vtbl[1] = windows.NewCallback(func(this uintptr) uintptr {
		return uiaProviderFromThis(this, iface).addRef()
	})
	vtbl[2] = windows.NewCallback(func(this uintptr) uintptr {
		return uiaProviderFromThis(this, iface).release()
	})
	for i, method := range methods {
		if address, ok := method.(uintptr); ok {
			vtbl[uiaUnknownSlots+i] = address
			continue
		}
		vtbl[uiaUnknownSlots+i] = windows.NewCallback(method)
	}
	uiaVtblStarts[iface] = uintptr(unsafe.Pointer(&vtbl[0]))
}

// UIAProvider is the UI Automation provider for one node of one window's accessibility snapshot. It is a COM object
// implementing every interface in uiaIface, so it is never used through this Go type by anything but the package's own
// bookkeeping: UI Automation holds one of the interface pointers within it instead.
//
// The lifetime is the COM reference count's to decide. The window's provider map holds one reference; UI Automation
// takes its own through QueryInterface and the out-parameters of the navigation methods. The object stays pinned for
// the garbage collector until the last of them is released, since UI Automation's pointers are invisible to Go.
type UIAProvider struct {
	// vtbls MUST BE FIRST, and must stay an array of exactly one pointer per interface: the pointer a client holds for
	// interface i is &vtbls[i], and every method recovers the provider from that address by subtracting the interface's
	// offset. A field in front of this would shift the whole scheme.
	vtbls    [uiaIfaceCount]uintptr
	window   *UIAWindow
	pinner   runtime.Pinner
	node     accessibility.NodeID
	refCount int32
	stale    atomic.Bool
}

// newUIAProvider creates the provider for one node, holding the one reference the window's provider map owns.
func newUIAProvider(window *UIAWindow, node accessibility.NodeID) *UIAProvider {
	uiaEnsureVtbls()
	p := &UIAProvider{
		window:   window,
		node:     node,
		refCount: 1,
	}
	p.vtbls = uiaVtblStarts
	p.pinner.Pin(p)
	return p
}

// Node returns the id of the node this provider describes. The node may no longer be in the window's tree, in which
// case the provider is stale.
func (p *UIAProvider) Node() accessibility.NodeID {
	return p.node
}

// Stale reports whether the node this provider describes has left the tree. A stale provider answers every method with
// UIA_E_ELEMENTNOTAVAILABLE.
func (p *UIAProvider) Stale() bool {
	return p.stale.Load()
}

// Unknown returns the provider's IRawElementProviderSimple pointer, which is also its IUnknown pointer. It is what
// UiaReturnRawElementProvider and the UiaRaise* functions are handed. No reference is added: the caller is relying on
// the window's provider map holding one.
func (p *UIAProvider) Unknown() unsafe.Pointer {
	return unsafe.Pointer(&p.vtbls[uiaIfaceSimple])
}

// ifacePtr returns the pointer a client holds for one of this provider's interfaces.
func (p *UIAProvider) ifacePtr(iface uiaIface) uintptr {
	return uintptr(unsafe.Pointer(&p.vtbls[iface]))
}

// addRef implements IUnknown::AddRef.
func (p *UIAProvider) addRef() uintptr {
	return comAddRef(&p.refCount)
}

// release implements IUnknown::Release, unpinning the object once the last reference is gone and not before: UI
// Automation may still hold pointers to it that Go cannot see.
func (p *UIAProvider) release() uintptr {
	remaining, final := comRelease(&p.refCount)
	if final {
		p.pinner.Unpin()
	}
	return remaining
}

// retire marks the provider stale, tells UI Automation to drop every reference it holds to it so that a client asking
// about it is told the element is gone rather than given a stale answer, and releases the reference the window's
// provider map held.
//
// UiaDisconnectProvider calls back into the provider — it asks for the runtime identifier to find its own references —
// which is why GetRuntimeId keeps answering after the provider has been marked stale.
func (p *UIAProvider) retire() {
	p.stale.Store(true)
	uiaDisconnectProvider(p.Unknown())
	p.release()
}

// retireRoot retires the fragment root: it marks the provider stale and releases the reference the window's provider
// map held, without asking UI Automation to drop its own references.
//
// The disconnect is deliberately skipped rather than forgotten. UiaDisconnectProvider finds the references it is to
// drop by asking the provider for its runtime identifier, and a fragment root has none to give — it answers
// GetRuntimeId with a NULL array so that UI Automation identifies it by the window handle instead, which is what the
// interface requires of a fragment root; see uiaFragmentGetRuntimeID. The call can therefore only fail, while still
// calling back into a provider that is being torn down. What actually withdraws the root is UiaReturnRawElementProvider
// with a NULL provider, which UIAWindow.Destroy makes before it reaches here, and a client still holding the root
// element is answered UIA_E_ELEMENTNOTAVAILABLE from the stale flag.
func (p *UIAProvider) retireRoot() {
	p.stale.Store(true)
	p.release()
}

// isRoot reports whether this provider is the window's fragment root, which is the only one that implements the
// fragment root, advise-events and window interfaces.
func (p *UIAProvider) isRoot() bool {
	return p.node == p.window.rootNode
}

// current returns the snapshot the provider must answer from along with its own node within it. ok is false when the
// provider is stale or its node is not in the current snapshot, which is the same thing from a client's point of view
// and is answered with UIA_E_ELEMENTNOTAVAILABLE.
func (p *UIAProvider) current() (tree *accessibility.Tree, node *accessibility.Node, ok bool) {
	if p.stale.Load() {
		return nil, nil, false
	}
	tree = p.window.Tree()
	if node = tree.Node(p.node); node == nil {
		return nil, nil, false
	}
	return tree, node, true
}

// supports reports whether this provider hands out one of the interfaces, which is what QueryInterface,
// GetPatternProvider and every control-pattern method answer from. Every element implements the two provider
// interfaces; the fragment root alone implements the three window-level ones; and a pattern interface exists only when
// UIAPatterns says the node supports that pattern.
//
// IWindowProvider is both a pattern interface and a window-level one, and the root-only half is what decides: a nested
// node that reports role.Dialog — a dialog-shaped panel inside a window — is not a window of its own, and answering
// get_CanMaximize, get_WindowVisualState or get_IsTopmost for it would be describing the containing window through an
// element that is not it. UIAPatterns cannot make that distinction, since it knows a node and not which one is the
// root, so it is made here and everything that hands out an interface goes through this.
//
// Answering from the live snapshot is a knowing deviation from COM, which requires the set of interfaces an object
// implements to be fixed for its lifetime: a client that obtained an IID from a successful QueryInterface is entitled
// to assume the same call keeps succeeding, and here it stops as soon as the node loses the pattern or leaves the tree.
// The alternative is worse. A provider whose node is a slider at one moment and a label at the next would either have
// to hand out IRangeValueProvider forever, with every method on it answering UIA_E_NOTSUPPORTED, or fix the set at
// creation and refuse a pattern the element has since gained. UI Automation expects providers to change under a client
// — that is what the property-changed and structure-changed events are for — and defines UIA_E_NOTSUPPORTED and
// UIA_E_ELEMENTNOTAVAILABLE for exactly this, so a client that re-asks is told something true either way.
func (p *UIAProvider) supports(iface uiaIface) bool {
	switch iface {
	case uiaIfaceSimple, uiaIfaceFragment:
		return true
	case uiaIfaceFragmentRoot, uiaIfaceAdviseEvents:
		return p.isRoot()
	case uiaIfaceWindow:
		return p.isRoot() && p.hasPattern(PatternWindow)
	default:
		pattern := uiaPatternForIface(iface)
		return pattern != 0 && p.hasPattern(pattern)
	}
}

// hasPattern reports whether the node this provider describes supports a pattern as of the current snapshot. A stale
// provider, and one whose node has left the tree, support nothing.
func (p *UIAProvider) hasPattern(pattern PatternSet) bool {
	_, node, ok := p.current()
	return ok && UIAPatterns(node).Has(pattern)
}

// uiaProviderFromThis recovers the provider a COM method was called on. this is the address of the interface's virtual
// method table pointer within the provider, so the provider begins iface tables earlier.
func uiaProviderFromThis(this uintptr, iface uiaIface) *UIAProvider {
	return xruntime.PtrFromUintptr[UIAProvider](this - uiaIfaceOffset(iface))
}

// uiaQueryInterface implements IUnknown::QueryInterface for every interface: iface says which one it was called
// through, so that the provider can be found, and the answer does not otherwise depend on it. Every interface handed
// out is AddRef'd first, as COM requires, and the out-parameter is cleared before anything else can fail.
func uiaQueryInterface(iface uiaIface, this, riid, out uintptr) uint64 {
	if out == 0 || riid == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	wanted, ok := uiaIfaceForIID(xruntime.PtrFromUintptr[windows.GUID](riid))
	if !ok {
		return COM_E_NOINTERFACE
	}
	p := uiaProviderFromThis(this, iface)
	if !p.supports(wanted) {
		return COM_E_NOINTERFACE
	}
	p.addRef()
	*target = p.ifacePtr(wanted)
	return COM_S_OK
}

// uiaIfaceForIID returns the interface a client is asking for, and whether this package implements it. IID_IUnknown is
// answered with the IRawElementProviderSimple table, whose first three slots are the IUnknown methods.
func uiaIfaceForIID(guid *windows.GUID) (iface uiaIface, ok bool) {
	if *guid == iidUnknown {
		return uiaIfaceSimple, true
	}
	for i := range uiaIfaceIIDs {
		if uiaIfaceIIDs[i] == *guid {
			return uiaIface(i), true
		}
	}
	return 0, false
}

// uiaSimpleProviderOptions implements IRawElementProviderSimple::get_ProviderOptions. Reporting
// ProviderOptions_ServerSideProvider alone, without ProviderOptions_UseComThreading, is what declares the provider
// free-threaded; see the comment on ProviderOptions in uia_constants.go for why that is the only workable answer here.
// It is answered even by a stale provider, since it describes the provider rather than the element.
func uiaSimpleProviderOptions(_, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	*xruntime.PtrFromUintptr[ProviderOptions](out) = ProviderOptions_ServerSideProvider
	return COM_S_OK
}

// uiaSimpleGetPatternProvider implements IRawElementProviderSimple::GetPatternProvider. A pattern the element does not
// support is answered with a NULL interface and S_OK, which is how a client is told the element simply does not do
// that; an error would say the provider is broken.
//
// Which patterns the element has is decided by supports, the same way QueryInterface decides it, so that a client that
// reaches a pattern by identifier and one that asks for its interface directly cannot be given different answers.
func uiaSimpleGetPatternProvider(this, patternID, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceSimple)
	if _, _, ok := p.current(); !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	pattern := PatternSetForID(PatternID(int32(uint32(patternID))))
	if pattern == 0 {
		return COM_S_OK
	}
	iface, ok := uiaIfaceForPattern(pattern)
	if !ok || !p.supports(iface) {
		return COM_S_OK
	}
	p.addRef()
	*target = p.ifacePtr(iface)
	return COM_S_OK
}

// uiaSimpleGetPropertyValue implements IRawElementProviderSimple::GetPropertyValue. A property the element does not
// supply is answered with an empty VARIANT and S_OK, which tells a client to fall back to the host provider or to its
// own default; UiaGetReservedNotSupportedValue exists for the stronger statement that the provider will never supply
// it, which nothing here needs to make.
func uiaSimpleGetPropertyValue(this, propertyID, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	value := xruntime.PtrFromUintptr[VARIANT](out)
	*value = VARIANT{}
	p := uiaProviderFromThis(this, uiaIfaceSimple)
	tree, node, ok := p.current()
	if !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	p.propertyValue(tree, node, PropertyID(int32(uint32(propertyID))), value)
	return COM_S_OK
}

// propertyValue fills in the VARIANT for one property of one node, leaving it empty for a property this element does
// not supply. Everything it can fill in belongs to UI Automation Core the moment the method returns, so nothing
// allocated here is freed here. See §7.6 of the plan for the list.
func (p *UIAProvider) propertyValue(tree *accessibility.Tree, node *accessibility.Node, propertyID PropertyID,
	value *VARIANT,
) {
	switch propertyID {
	case UIA_NamePropertyId:
		uiaSetString(value, node.Name)
	case UIA_HelpTextPropertyId, UIA_FullDescriptionPropertyId:
		uiaSetString(value, node.Description)
	case UIA_ControlTypePropertyId:
		value.SetI4(int32(UIAControlType(node)))
	case UIA_IsEnabledPropertyId:
		value.SetBool(!node.Disabled)
	case UIA_IsKeyboardFocusablePropertyId:
		value.SetBool(node.Focusable)
	case UIA_HasKeyboardFocusPropertyId:
		value.SetBool(node.Focused && uiaRootFocused(tree))
	case UIA_IsControlElementPropertyId:
		value.SetBool(UIAIsControlElement(node))
	case UIA_IsContentElementPropertyId:
		value.SetBool(UIAIsContentElement(tree, node))
	case UIA_IsPasswordPropertyId:
		value.SetBool(node.Protected)
	case UIA_IsOffscreenPropertyId:
		value.SetBool(node.Offscreen)
	case UIA_OrientationPropertyId:
		value.SetI4(int32(UIAOrientation(node)))
	case UIA_LevelPropertyId:
		if node.Level > 0 {
			value.SetI4(int32(node.Level))
		}
	case UIA_PositionInSetPropertyId:
		if position, _ := UIAPositionInSet(tree, node); position > 0 {
			value.SetI4(int32(position))
		}
	case UIA_SizeOfSetPropertyId:
		if position, size := UIAPositionInSet(tree, node); position > 0 {
			value.SetI4(int32(size))
		}
	case UIA_LabeledByPropertyId:
		p.setProvider(value, node.LabeledBy)
	case UIA_DescribedByPropertyId:
		p.setProviderArray(value, node.DescribedBy)
	case UIA_ControllerForPropertyId:
		p.setProviderArray(value, node.Controls)
	case UIA_AcceleratorKeyPropertyId:
		uiaSetString(value, node.Shortcut)
	case UIA_AutomationIdPropertyId:
		value.SetBSTR(strconv.FormatUint(uint64(node.ID), 10))
	case UIA_FrameworkIdPropertyId:
		value.SetBSTR(UiaFrameworkID)
	case UIA_ItemStatusPropertyId:
		uiaSetString(value, UIAItemStatus(node))
	case UIA_IsDataValidForFormPropertyId:
		value.SetBool(!node.Invalid)
	case UIA_HeadingLevelPropertyId:
		value.SetI4(int32(UIAHeadingLevel(node)))
	case UIA_NativeWindowHandlePropertyId:
		// The property is an int, so a 64-bit handle is reported truncated. Clients that care use the host provider,
		// which the root also supplies and which reports the handle itself.
		if p.isRoot() {
			value.SetI4(int32(uint32(uintptr(p.window.HWND()))))
		}
	case UIA_IsDialogPropertyId:
		if p.isRoot() {
			value.SetBool(node.Role == role.Dialog)
		}
	case UIA_LiveSettingPropertyId:
		// The root is where an announcement is raised from, and a polite live region is what tells a client to speak
		// one after whatever it is already saying rather than interrupting.
		if p.isRoot() {
			value.SetI4(int32(LiveSetting_Polite))
		}
	default:
		// Including UIA_LocalizedControlTypePropertyId, which is deliberately left to UI Automation: it has a
		// localized name for every control type, and ours would be in English only.
	}
}

// uiaSetString stores a string property, leaving the VARIANT empty when there is nothing to say. An empty BSTR and no
// answer at all are different things to a client: the first asserts that the element's name really is nothing.
func uiaSetString(value *VARIANT, s string) {
	if s != "" {
		value.SetBSTR(s)
	}
}

// uiaRootFocused reports whether the window a snapshot describes is the active one, which is the second half of the
// HasKeyboardFocus property: a node holds the focus within its window whether or not that window has it.
func uiaRootFocused(tree *accessibility.Tree) bool {
	root := tree.Node(tree.Root)
	return root != nil && root.Focused
}

// setProvider stores the first of the given nodes that has a provider as an interface pointer, which is the shape the
// LabeledBy property takes. The reference Provider handed over becomes the VARIANT's, and through it UI Automation
// Core's.
func (p *UIAProvider) setProvider(value *VARIANT, ids []accessibility.NodeID) {
	for _, id := range ids {
		if other := p.window.Provider(id); other != nil {
			value.SetUnknown(other.Unknown())
			return
		}
	}
}

// setProviderArray stores the given nodes' providers as a SAFEARRAY of interface pointers, which is the shape the
// DescribedBy and ControllerFor properties take. Storing an element into the array adds a reference of its own, so the
// array ends up owning one per element and the references Provider handed over are given back once it is built.
func (p *UIAProvider) setProviderArray(value *VARIANT, ids []accessibility.NodeID) {
	if len(ids) == 0 {
		return
	}
	others := make([]*UIAProvider, 0, len(ids))
	defer func() {
		for _, other := range others {
			other.release()
		}
	}()
	pointers := make([]unsafe.Pointer, 0, len(ids))
	for _, id := range ids {
		if other := p.window.Provider(id); other != nil {
			others = append(others, other)
			pointers = append(pointers, other.Unknown())
		}
	}
	if len(pointers) == 0 {
		return
	}
	if array := NewSafeArrayUnknown(pointers); array != 0 {
		value.SetArray(VT_UNKNOWN, array)
	}
}

// uiaSimpleHostRawElementProvider implements IRawElementProviderSimple::get_HostRawElementProvider. Only the fragment
// root has a host: the system-supplied provider for the window handle, which fills in the process id, the window
// handle and the window's class name so that the fragment does not have to. Every other element answers NULL, which is
// what says "I am part of a fragment rather than a window of my own".
func uiaSimpleHostRawElementProvider(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceSimple)
	if !p.isRoot() {
		return COM_S_OK
	}
	host, hr := UiaHostProviderFromHwnd(p.window.HWND())
	if !hresultSucceeded(hr) {
		return uint64(hr)
	}
	// The reference UiaHostProviderFromHwnd returned is handed straight to the caller, which is what an out-parameter
	// of an interface type means.
	*target = uintptr(unsafe.Pointer(host))
	return COM_S_OK
}

// uiaFragmentNavigate implements IRawElementProviderFragment::Navigate. Navigation runs over the unignored tree, so
// the layout panels a snapshot marks Ignored are invisible to a client and their children appear in their place. The
// fragment root reports nothing for parent or sibling: a fragment's root is where UI Automation stitches it onto the
// window, and it does that through the host provider instead.
func uiaFragmentNavigate(this, direction, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceFragment)
	tree, _, ok := p.current()
	if !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	id := UIANavigate(tree, p.node, NavigateDirection(int32(uint32(direction))))
	if id == 0 {
		return COM_S_OK
	}
	other := p.window.Provider(id)
	if other == nil {
		return COM_S_OK
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*target = other.ifacePtr(uiaIfaceFragment)
	return COM_S_OK
}

// uiaFragmentGetRuntimeID implements IRawElementProviderFragment::GetRuntimeId. The fragment root answers with a NULL
// array, which tells UI Automation to identify it by its window handle; everything else answers with an array that
// begins with UiaAppendRuntimeId, so that UI Automation prepends the root's own identifier and the result stays unique
// across the windows of the process.
//
// This is the one method a stale provider still answers rather than reporting the element as unavailable, and it has to
// be: UiaDisconnectProvider asks a provider for its runtime identifier in order to find the references UI Automation
// holds to it, and that call is made while the provider is being retired. The identifier is derived from the node id
// alone, which never changes, so the answer is right whether or not the node is still in the tree.
func uiaFragmentGetRuntimeID(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[SAFEARRAY](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceFragment)
	if p.isRoot() {
		return COM_S_OK
	}
	array := NewSafeArrayInt32(UIARuntimeID(p.node))
	if array == 0 {
		return COM_E_OUTOFMEMORY
	}
	*target = array
	return COM_S_OK
}

// uiaFragmentBoundingRectangle implements IRawElementProviderFragment::get_BoundingRectangle, converting the node's
// window-local logical bounds into screen pixels. An element that is scrolled or clipped out of view has no meaningful
// rectangle and reports an empty one, which is what pairs with the IsOffscreen property.
func uiaFragmentBoundingRectangle(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	rect := xruntime.PtrFromUintptr[UiaRect](out)
	*rect = UiaRect{}
	p := uiaProviderFromThis(this, uiaIfaceFragment)
	_, node, ok := p.current()
	if !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	if node.Offscreen || node.Bounds.Empty() {
		return COM_S_OK
	}
	left, top, width, height := p.window.Geometry().ScreenRect(node.Bounds)
	*rect = UiaRect{Left: left, Top: top, Width: width, Height: height}
	return COM_S_OK
}

// uiaFragmentGetEmbeddedFragmentRoots implements IRawElementProviderFragment::GetEmbeddedFragmentRoots. Unison draws
// every widget itself, so a window never contains another framework's fragment and the answer is always a NULL array.
func uiaFragmentGetEmbeddedFragmentRoots(_, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = 0
	return COM_S_OK
}

// uiaFragmentSetFocus implements IRawElementProviderFragment::SetFocus by asking the window to give the node the
// keyboard focus. The request is queued onto the UI thread and this returns at once, which is what UI Automation
// expects: the focus change is reported later, as an event.
func uiaFragmentSetFocus(this uintptr) uint64 {
	p := uiaProviderFromThis(this, uiaIfaceFragment)
	_, node, ok := p.current()
	if !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	if !node.Focusable {
		return UIA_E_INVALIDOPERATION
	}
	if !p.window.dispatch(accessibility.ActionRequest{Node: p.node, Action: accessibility.Focus}) {
		return UIA_E_INVALIDOPERATION
	}
	return COM_S_OK
}

// uiaFragmentFragmentRoot implements IRawElementProviderFragment::get_FragmentRoot. Every element in the window,
// including the root itself, reports the same fragment root.
func uiaFragmentFragmentRoot(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceFragment)
	root := p.window.Root()
	if root == nil {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	// The reference Root handed over becomes the caller's, which is what an interface out-parameter means.
	*target = root.ifacePtr(uiaIfaceFragmentRoot)
	return COM_S_OK
}

// uiaFragmentRootElementProviderFromPoint implements IRawElementProviderFragmentRoot::ElementProviderFromPoint. It is
// reached through an assembly thunk, so the two coordinates arrive as the raw bits of the doubles rather than as
// doubles; see uia_thunk_windows.go. The point is in screen pixels and the snapshot's bounds are in window-local
// logical units, so the window's geometry converts before the tree is asked.
//
// A point over nothing — outside the window, or over an element the snapshot marks offscreen — is answered with a NULL
// element and S_OK, which lets UI Automation fall back to the window itself.
func uiaFragmentRootElementProviderFromPoint(this, xBits, yBits, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceFragmentRoot)
	tree, _, ok := p.current()
	if !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	id := UIAHitTest(tree, p.window.Geometry().WindowPoint(math.Float64frombits(uint64(xBits)),
		math.Float64frombits(uint64(yBits))))
	if id == 0 {
		return COM_S_OK
	}
	hit := p.window.Provider(id)
	if hit == nil {
		return COM_S_OK
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*target = hit.ifacePtr(uiaIfaceFragment)
	return COM_S_OK
}

// uiaFragmentRootGetFocus implements IRawElementProviderFragmentRoot::GetFocus. It answers only while the window is the
// active one: pointing a client at an element inside a window the user is not looking at makes a screen reader jump
// away from where the user is.
func uiaFragmentRootGetFocus(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := uiaProviderFromThis(this, uiaIfaceFragmentRoot)
	tree, node, ok := p.current()
	if !ok {
		return UIA_E_ELEMENTNOTAVAILABLE
	}
	if !node.Focused || tree.Focus == 0 {
		return COM_S_OK
	}
	focus := p.window.Provider(tree.Focus)
	if focus == nil {
		return COM_S_OK
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*target = focus.ifacePtr(uiaIfaceFragment)
	return COM_S_OK
}

// uiaAdviseEventAdded implements IRawElementProviderAdviseEvents::AdviseEventAdded. UI Automation calls it when a
// client starts listening to something within this fragment. Nothing is filtered on it — UiaClientsAreListening is
// what gates event raising — so it only keeps a count, for diagnostics and for tests.
func uiaAdviseEventAdded(this, _, _ uintptr) uint64 {
	uiaProviderFromThis(this, uiaIfaceAdviseEvents).window.adviseEvents(1)
	return COM_S_OK
}

// uiaAdviseEventRemoved implements IRawElementProviderAdviseEvents::AdviseEventRemoved.
func uiaAdviseEventRemoved(this, _, _ uintptr) uint64 {
	uiaProviderFromThis(this, uiaIfaceAdviseEvents).window.adviseEvents(-1)
	return COM_S_OK
}

// uiaWindowSetVisualState implements IWindowProvider::SetVisualState. Maximizing and minimizing a window through
// accessibility is not offered: the root package has no such API, and a client that cannot do it falls back to the
// window's own system menu.
func uiaWindowSetVisualState(_, _ uintptr) uint64 {
	return UIA_E_NOTSUPPORTED
}

// uiaWindowClose implements IWindowProvider::Close. Closing a window is a decision for the application, which may have
// unsaved work to ask about, so it is not offered here either.
func uiaWindowClose(_ uintptr) uint64 {
	return UIA_E_NOTSUPPORTED
}

// uiaWindowWaitForInputIdle implements IWindowProvider::WaitForInputIdle. Waiting for the UI thread from a provider
// method would deadlock whenever the UI thread is the one waiting on us, so it is never offered.
func uiaWindowWaitForInputIdle(_, _, out uintptr) uint64 {
	if out != 0 {
		uiaSetBOOL(out, false)
	}
	return UIA_E_NOTSUPPORTED
}

// uiaWindowCanMaximize implements IWindowProvider::get_CanMaximize. A window the snapshot reports as not resizable has
// no maximize box — w32WindowStyle leaves WS_MAXIMIZEBOX out for one — so Resizable is the answer, rather than a
// constant that tells a client a fixed-size dialog can be maximized and leaves it to discover otherwise when
// SetVisualState refuses.
func uiaWindowCanMaximize(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceWindow, out, func(n *accessibility.Node) bool { return n.Resizable })
}

// uiaWindowCanMinimize implements IWindowProvider::get_CanMinimize. Every window unison creates has a minimize box,
// including the undecorated ones, so the answer is always yes — but it is still answered through uiaPatternNode, so
// that an element whose node has left the tree says so rather than reporting on a window it no longer stands for.
func uiaWindowCanMinimize(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceWindow, out, func(_ *accessibility.Node) bool { return true })
}

// uiaWindowIsModal implements IWindowProvider::get_IsModal, which is how a client knows to keep the user inside this
// window until it is dealt with. Like every other control-pattern method it goes through uiaPatternNode, so that an
// element that no longer has the pattern — or never should have had it — says so rather than answering for a window
// that is not it.
func uiaWindowIsModal(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceWindow, out, func(n *accessibility.Node) bool { return n.Modal })
}

// uiaWindowVisualState implements IWindowProvider::get_WindowVisualState. The snapshot does not record whether a window
// is maximized or minimized — a minimized window has nothing worth reporting anyway — so every window reports itself
// normal. It still goes through uiaPatternNode, so that an element whose node has left the tree is reported as gone
// rather than answered for.
func uiaWindowVisualState(this, out uintptr) uint64 {
	return uiaPatternInt32(this, uiaIfaceWindow, out, func(_ *accessibility.Node) int32 {
		return int32(WindowVisualState_Normal)
	})
}

// uiaWindowInteractionStateValue implements IWindowProvider::get_WindowInteractionState. A window the snapshot reports
// as disabled is disabled because something modal sits in front of it, which is the distinction UI Automation wants
// here.
func uiaWindowInteractionStateValue(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	*xruntime.PtrFromUintptr[WindowInteractionState](out) = WindowInteractionState_ReadyForUserInteraction
	_, _, node, hr := uiaPatternNode(this, uiaIfaceWindow)
	if hr != COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[WindowInteractionState](out) = UIAWindowInteractionState(node)
	return COM_S_OK
}

// uiaWindowIsTopmost implements IWindowProvider::get_IsTopmost, which is the snapshot's Floating flag: a window created
// with FloatingWindowOption is given WS_EX_TOPMOST by w32WindowExStyle, and it really does stay in front of the
// ordinary windows, which is what a client wants to know before it describes the window's place on screen.
func uiaWindowIsTopmost(this, out uintptr) uint64 {
	return uiaPatternBOOL(this, uiaIfaceWindow, out, func(n *accessibility.Node) bool { return n.Floating })
}

// uiaSetBOOL stores a Win32 BOOL into an out-parameter. Unlike a VARIANT_BOOL, true is one rather than every bit set.
// The caller must have checked that out is not NULL.
func uiaSetBOOL(out uintptr, value bool) {
	boolean := int32(0)
	if value {
		boolean = 1
	}
	*xruntime.PtrFromUintptr[int32](out) = boolean
}
