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
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// This file holds the COM object a UI Automation client talks to. One Go struct implements every interface the provider
// needs, laid out the way a C++ object with multiple base classes is: it begins with one virtual method table pointer
// per interface, so the pointer a client holds for an interface is the address of that interface's slot rather than the
// address of the object, and each method subtracts its own interface's offset to find the object again. See
// layout.go for the interface order and the arithmetic.
//
// Every method answers from the window's immutable snapshot, never from a live panel, which is what lets the providers
// declare themselves free-threaded: UI Automation calls in on whichever thread it likes while the UI thread carries on,
// possibly sitting in a modal loop where it could not pump COM messages even if it wanted to. The only traffic in the
// other direction is an action request, which goes to the root package to be run on the UI thread.
//
// A provider outlives the node it describes. When a node leaves the tree its provider is marked stale and answers
// E_ELEMENTNOTAVAILABLE, which is what a client holding an element for something that has been destroyed must be
// told, and it stays alive — anchored against collection and pinned against movement, since UI Automation holds a raw
// pointer to it — until the last COM reference is released. See liveProviders.

// ifaceIIDs holds the interface identifier a client asks for each interface by, indexed by iface. IID_IUnknown is
// deliberately absent: it is answered by handing out the IRawElementProviderSimple pointer, since that table's first
// three slots are the IUnknown methods.
var ifaceIIDs = [ifaceCount]windows.GUID{
	ifaceSimple:         xos.Must(windows.GUIDFromString("{d6dd68d1-86fd-4332-8666-9abedea2d24c}")),
	ifaceFragment:       xos.Must(windows.GUIDFromString("{f7063da8-8359-439c-9297-bbc5299a7d87}")),
	ifaceFragmentRoot:   xos.Must(windows.GUIDFromString("{620ce2a5-ab8f-40a9-86cb-de3c75599b58}")),
	ifaceAdviseEvents:   xos.Must(windows.GUIDFromString("{a407b27b-0f6d-4427-9292-473c7bf93258}")),
	ifaceWindow:         xos.Must(windows.GUIDFromString("{987df77b-db06-4d77-8f8a-86a9c3bb90b9}")),
	ifaceInvoke:         xos.Must(windows.GUIDFromString("{54fcb24b-e18e-47a2-b4d3-eccbe77599a2}")),
	ifaceToggle:         xos.Must(windows.GUIDFromString("{56d00bd0-c4f4-433c-a836-1a52a57e0892}")),
	ifaceValue:          xos.Must(windows.GUIDFromString("{c7935180-6fb3-4201-b174-7df73adbf64a}")),
	ifaceRangeValue:     xos.Must(windows.GUIDFromString("{36dc7aef-33e6-4691-afe1-2be7274b3d33}")),
	ifaceSelection:      xos.Must(windows.GUIDFromString("{fb8b03af-3bdf-48d4-bd36-1a65793be168}")),
	ifaceSelectionItem:  xos.Must(windows.GUIDFromString("{2acad808-b2d4-452d-a407-91ff1ad167b2}")),
	ifaceExpandCollapse: xos.Must(windows.GUIDFromString("{d847d3a5-cab0-4a98-8c32-ecb45c59ad24}")),
	ifaceScrollItem:     xos.Must(windows.GUIDFromString("{2360c714-4bf1-4b26-ba65-9b21316127eb}")),
	ifaceGrid:           xos.Must(windows.GUIDFromString("{b17d6187-0907-464b-a168-0ef17a1572b1}")),
	ifaceGridItem:       xos.Must(windows.GUIDFromString("{d02541f1-fb81-4d64-ae32-f520f8a6dbd1}")),
	ifaceTable:          xos.Must(windows.GUIDFromString("{9c860395-97b3-490a-b52a-858cc22af166}")),
	ifaceTableItem:      xos.Must(windows.GUIDFromString("{b9734fa6-771f-4d78-9c90-2517999349cd}")),
	ifaceText:           xos.Must(windows.GUIDFromString("{0dc5e6ed-3e16-4bf1-8f9a-a979878bc195}")),
	ifaceTextChild:      xos.Must(windows.GUIDFromString("{4c2de2b9-c88f-4f88-a111-f1d336b7d1a9}")),
}

// ifaceIIDAliases holds the interface identifiers that are answered with a table listed in ifaceIIDs rather than
// with one of their own. There is one: ITextProvider, whose six methods are the first six of ITextProvider2's, so an
// element that implements the later interface implements the earlier one by construction and a client asking for either
// must be handed the same table. It is the same trick IID_IUnknown is answered with, and the reason the Text and Text2
// patterns share an interface; see patternIfaces.
var ifaceIIDAliases = []struct {
	guid  windows.GUID
	iface iface
}{
	{guid: xos.Must(windows.GUIDFromString("{3589c92c-63f3-4367-99bb-ada653b77cf2}")), iface: ifaceText},
}

// The virtual method tables, one per interface, shared by every provider in the process. They are built on first use
// rather than at startup: a window that no assistive technology ever asks about must cost nothing, and building them
// means asking the Go runtime for a little over a hundred callback trampolines.
var (
	vtblOnce           sync.Once
	vtblStarts         [ifaceCount]uintptr
	simpleVtbl         [simpleSlots]uintptr
	fragmentVtbl       [fragmentSlots]uintptr
	fragmentRootVtbl   [fragmentRootSlots]uintptr
	adviseEventsVtbl   [adviseEventsSlots]uintptr
	windowVtbl         [windowSlots]uintptr
	invokeVtbl         [invokeSlots]uintptr
	toggleVtbl         [toggleSlots]uintptr
	valueVtbl          [valueSlots]uintptr
	rangeValueVtbl     [rangeValueSlots]uintptr
	selectionVtbl      [selectionSlots]uintptr
	selectionItemVtbl  [selectionItemSlots]uintptr
	expandCollapseVtbl [expandCollapseSlots]uintptr
	scrollItemVtbl     [scrollItemSlots]uintptr
	gridVtbl           [gridSlots]uintptr
	gridItemVtbl       [gridItemSlots]uintptr
	tableVtbl          [tableSlots]uintptr
	tableItemVtbl      [tableItemSlots]uintptr
	textVtbl           [textSlots]uintptr
	textChildVtbl      [textChildSlots]uintptr
	textRangeVtbl      [textRangeSlots]uintptr
)

// ensureVtbls builds every virtual method table, once per process.
func ensureVtbls() {
	vtblOnce.Do(buildVtbls)
}

// buildVtbls fills in every interface's virtual method table. The order of the methods handed to buildVtbl is the
// order the interface declares them in and is the whole content of the ABI contract with UI Automation: a method in the
// wrong slot is called with another method's arguments.
func buildVtbls() {
	buildVtbl(
		ifaceSimple, simpleVtbl[:],
		simpleProviderOptions,
		simpleGetPatternProvider,
		simpleGetPropertyValue,
		simpleHostRawElementProvider,
	)
	buildVtbl(
		ifaceFragment, fragmentVtbl[:],
		fragmentNavigate,
		fragmentGetRuntimeID,
		fragmentBoundingRectangle,
		fragmentGetEmbeddedFragmentRoots,
		fragmentSetFocus,
		fragmentFragmentRoot,
	)
	// ElementProviderFromPoint takes two doubles, which syscall.NewCallback cannot describe, so its slot holds an
	// assembly thunk that moves the floating-point registers into the integer argument registers the callback reads.
	// See thunk_windows.go, including what to do if a thunk ever misbehaves on a target.
	fromPointCallback = windows.NewCallback(fragmentRootElementProviderFromPoint)
	buildVtbl(
		ifaceFragmentRoot, fragmentRootVtbl[:],
		fromPointSlot(),
		fragmentRootGetFocus,
	)
	buildVtbl(
		ifaceAdviseEvents, adviseEventsVtbl[:],
		adviseEventAdded,
		adviseEventRemoved,
	)
	buildVtbl(
		ifaceWindow, windowVtbl[:],
		windowSetVisualState,
		windowClose,
		windowWaitForInputIdle,
		windowCanMaximize,
		windowCanMinimize,
		windowIsModal,
		windowVisualState,
		windowInteractionStateValue,
		windowIsTopmost,
	)
	buildPatternVtbls()
	buildTextVtbls()
}

// buildVtbl fills in one interface's virtual method table. The three IUnknown slots come first and are built here
// rather than shared, because each needs to know which interface's pointer it will be called through in order to find
// the provider; the rest are the methods, in declaration order. A method may be given either as a function, which is
// wrapped in a callback, or as a uintptr that is already the address of a native entry point.
func buildVtbl(which iface, vtbl []uintptr, methods ...any) {
	vtbl[0] = windows.NewCallback(func(this, riid, out uintptr) uint64 {
		return queryInterface(which, this, riid, out)
	})
	vtbl[1] = windows.NewCallback(func(this uintptr) uintptr {
		return providerFromThis(this, which).addRef()
	})
	vtbl[2] = windows.NewCallback(func(this uintptr) uintptr {
		return providerFromThis(this, which).release()
	})
	for i, method := range methods {
		if address, ok := method.(uintptr); ok {
			vtbl[unknownSlots+i] = address
			continue
		}
		vtbl[unknownSlots+i] = windows.NewCallback(method)
	}
	vtblStarts[which] = uintptr(unsafe.Pointer(&vtbl[0]))
}

// liveProviders holds every provider that still has a COM reference outstanding, which is what keeps one reachable
// for the garbage collector.
//
// A provider cannot anchor itself. Its pinner keeps the collector from moving it and makes it legal to hand its address
// out, but a runtime.Pinner is documented to need keeping alive independently of what it pins, and the provider map is
// not that: retiring a provider drops it from the map while UI Automation may still hold references, leaving nothing
// but the object's own field pointing at the object. That it survives today rests on the collector treating every
// finalized object as a root, which is an implementation detail rather than part of the API. This set is the anchor the
// documented lifetime needs — a provider is entered as it is created and dropped when release sees the last reference
// go — so that what Provider's doc comment promises is guaranteed rather than incidental.
//
// UI Automation releases references from whichever thread it likes, so both ends are under the lock.
var liveProviders = struct {
	set  map[*Provider]struct{}
	lock sync.Mutex
}{set: make(map[*Provider]struct{})}

// holdProvider anchors a provider for as long as anything holds a COM reference to it. See liveProviders.
func holdProvider(p *Provider) {
	liveProviders.lock.Lock()
	defer liveProviders.lock.Unlock()
	liveProviders.set[p] = struct{}{}
}

// dropProvider gives up the anchor holdProvider took, which the release of the last reference does.
func dropProvider(p *Provider) {
	liveProviders.lock.Lock()
	defer liveProviders.lock.Unlock()
	delete(liveProviders.set, p)
}

// liveProviderCount returns how many providers are anchored. It is for tests, which is the only thing that can see
// the set at a moment when it should be empty.
func liveProviderCount() int {
	liveProviders.lock.Lock()
	defer liveProviders.lock.Unlock()
	return len(liveProviders.set)
}

// lookupProvider returns the provider an interface pointer a client handed over belongs to, or nil when no provider
// of this process is at that address.
//
// It exists because one of the Text pattern's methods is given an element rather than asked about one —
// ITextProvider::RangeFromChild — and a pointer from a client is not to be trusted: dereferencing whatever it points at
// would be a crash at best, and a client is perfectly entitled to hand over an element from another provider, which is
// an invalid argument rather than a fault. The answer comes from comparing addresses against the providers this process
// has handed out, never from following the pointer.
//
// ITextProvider2::RangeFromAnnotation is given an element too and does not come here, because it never looks at it:
// nothing in a document here is annotated, so every annotation element there could be is one this provider cannot
// place, and the answer is the same NULL range whichever element a client names.
//
// Any of a provider's interface pointers is recognized, not only the one the method's signature names: a client that
// obtained an element as IRawElementProviderFragment and passes it where IRawElementProviderSimple is asked for is
// passing the same object, and refusing it would be refusing a legitimate call.
func lookupProvider(this uintptr) *Provider {
	if this == 0 {
		return nil
	}
	liveProviders.lock.Lock()
	defer liveProviders.lock.Unlock()
	for p := range liveProviders.set {
		for which := ifaceSimple; which < ifaceCount; which++ {
			if p.ifacePtr(which) == this {
				return p
			}
		}
	}
	return nil
}

// Provider is the UI Automation provider for one node of one window's accessibility snapshot. It is a COM object
// implementing every interface in iface, so it is never used through this Go type by anything but the package's own
// bookkeeping: UI Automation holds one of the interface pointers within it instead.
//
// The lifetime is the COM reference count's to decide. The window's provider map holds one reference; UI Automation
// takes its own through QueryInterface and the out-parameters of the navigation methods. The object stays reachable and
// pinned for the garbage collector until the last of them is released, since UI Automation's pointers are invisible to
// Go: reachable through liveProviders, and pinned through the pinner below.
type Provider struct {
	// vtbls MUST BE FIRST, and must stay an array of exactly one pointer per interface: the pointer a client holds for
	// interface i is &vtbls[i], and every method recovers the provider from that address by subtracting the interface's
	// offset. A field in front of this would shift the whole scheme.
	vtbls    [ifaceCount]uintptr
	window   *Window
	pinner   runtime.Pinner
	node     accessibility.NodeID
	refCount int32
	stale    atomic.Bool
}

// newProvider creates the provider for one node, holding the one reference the window's provider map owns. The
// provider is anchored and pinned here, together, and both are given up by the release of the last reference.
func newProvider(window *Window, node accessibility.NodeID) *Provider {
	ensureVtbls()
	p := &Provider{
		window:   window,
		node:     node,
		refCount: 1,
	}
	p.vtbls = vtblStarts
	p.pinner.Pin(p)
	holdProvider(p)
	return p
}

// Stale reports whether the node this provider describes has left the tree. A stale provider answers every method with
// E_ELEMENTNOTAVAILABLE.
func (p *Provider) Stale() bool {
	return p.stale.Load()
}

// Unknown returns the provider's IRawElementProviderSimple pointer, which is also its IUnknown pointer. It is what
// ReturnRawElementProvider and the Raise* functions are handed. No reference is added: the caller is relying on
// the window's provider map holding one.
func (p *Provider) Unknown() unsafe.Pointer {
	return unsafe.Pointer(&p.vtbls[ifaceSimple])
}

// ifacePtr returns the pointer a client holds for one of this provider's interfaces.
func (p *Provider) ifacePtr(which iface) uintptr {
	return uintptr(unsafe.Pointer(&p.vtbls[which]))
}

// addRef implements IUnknown::AddRef.
func (p *Provider) addRef() uintptr {
	return w32.ComAddRef(&p.refCount)
}

// release implements IUnknown::Release, unpinning the object and letting go of the anchor that keeps it reachable once
// the last reference is gone and not before: UI Automation may still hold pointers to it that Go cannot see. Exactly
// one caller observes the final release, so neither is given up twice.
func (p *Provider) release() uintptr {
	remaining, final := w32.ComRelease(&p.refCount)
	if final {
		p.pinner.Unpin()
		dropProvider(p)
	}
	return remaining
}

// retire marks the provider stale, tells UI Automation to drop every reference it holds to it so that a client asking
// about it is told the element is gone rather than given a stale answer, and releases the reference the window's
// provider map held.
//
// DisconnectProvider calls back into the provider — it asks for the runtime identifier to find its own references —
// which is why GetRuntimeId keeps answering after the provider has been marked stale.
func (p *Provider) retire() {
	p.stale.Store(true)
	disconnectProvider(p.Unknown())
	p.release()
}

// retireRoot retires the fragment root: it marks the provider stale and releases the reference the window's provider
// map held, without asking UI Automation to drop its own references.
//
// The disconnect is deliberately skipped rather than forgotten. DisconnectProvider finds the references it is to
// drop by asking the provider for its runtime identifier, and a fragment root has none to give — it answers
// GetRuntimeId with a NULL array so that UI Automation identifies it by the window handle instead, which is what the
// interface requires of a fragment root; see fragmentGetRuntimeID. The call can therefore only fail, while still
// calling back into a provider that is being torn down. What actually withdraws the root is ReturnRawElementProvider
// with a NULL provider, which Window.Destroy makes before it reaches here, and a client still holding the root
// element is answered E_ELEMENTNOTAVAILABLE from the stale flag.
func (p *Provider) retireRoot() {
	p.stale.Store(true)
	p.release()
}

// isRoot reports whether this provider is the window's fragment root, which is the only one that implements the
// fragment root, advise-events and window interfaces.
func (p *Provider) isRoot() bool {
	return p.node == p.window.rootNode
}

// current returns the snapshot the provider must answer from along with its own node within it. ok is false when the
// provider is stale or its node is not in the current snapshot, which is the same thing from a client's point of view
// and is answered with E_ELEMENTNOTAVAILABLE.
func (p *Provider) current() (tree *accessibility.Tree, node *accessibility.Node, ok bool) {
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
// ProvidedPatterns says the node hands that pattern out.
//
// IWindowProvider is both a pattern interface and a window-level one, and the root-only half is what decides: a nested
// node that reports role.Dialog — a dialog-shaped panel inside a window — is not a window of its own, and answering
// get_CanMaximize, get_WindowVisualState or get_IsTopmost for it would be describing the containing window through an
// element that is not it. Patterns cannot make that distinction, since it knows a node and not which one is the
// root, so ProvidedPatterns makes it from the tree and everything that asks what an element supports — this, the
// property values a raise carries, and the decision to raise one at all — answers from that one set.
//
// Answering from the live snapshot is a knowing deviation from COM, which requires the set of interfaces an object
// implements to be fixed for its lifetime: a client that obtained an IID from a successful QueryInterface is entitled
// to assume the same call keeps succeeding, and here it stops as soon as the node loses the pattern or leaves the tree.
// The alternative is worse. A provider whose node is a slider at one moment and a label at the next would either have
// to hand out IRangeValueProvider forever, with every method on it answering E_NOTSUPPORTED, or fix the set at
// creation and refuse a pattern the element has since gained. UI Automation expects providers to change under a client
// — that is what the property-changed and structure-changed events are for — and defines E_NOTSUPPORTED and
// E_ELEMENTNOTAVAILABLE for exactly this, so a client that re-asks is told something true either way.
func (p *Provider) supports(which iface) bool {
	switch which {
	case ifaceSimple, ifaceFragment:
		return true
	case ifaceFragmentRoot, ifaceAdviseEvents:
		return p.isRoot()
	case ifaceWindow:
		return p.hasPattern(PatternWindow)
	default:
		pattern := patternForIface(which)
		return pattern != 0 && p.hasPattern(pattern)
	}
}

// hasPattern reports whether the node this provider describes hands a pattern out as of the current snapshot. A stale
// provider, and one whose node has left the tree, hand out nothing.
//
// ProvidedPatterns rather than Patterns, so that the root-only half of the Window pattern is applied here and
// wherever else the adapter asks what an element supports — ReportsProperty, which decides what a raised property
// change carries, and decider.patternAvailability, which decides what is raised at all. A nested dialog-shaped panel
// is refused IWindowProvider by that one rule rather than by each caller remembering it.
func (p *Provider) hasPattern(pattern PatternSet) bool {
	tree, node, ok := p.current()
	return ok && ProvidesPattern(tree, node, pattern)
}

// providerFromThis recovers the provider a COM method was called on. this is the address of the interface's virtual
// method table pointer within the provider, so the provider begins iface tables earlier.
func providerFromThis(this uintptr, which iface) *Provider {
	return xruntime.PtrFromUintptr[Provider](this - ifaceOffset(which))
}

// queryInterface implements IUnknown::QueryInterface for every interface: which says which one it was called
// through, so that the provider can be found, and the answer does not otherwise depend on it. Every interface handed
// out is AddRef'd first, as COM requires, and the out-parameter is cleared before anything else can fail.
func queryInterface(which iface, this, riid, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	// Cleared before the interface identifier is validated, not after: QueryInterface must store NULL in the
	// out-parameter on every failure, so a caller that passed a NULL riid is not left holding whatever was there.
	*target = 0
	if riid == 0 {
		return w32.COM_E_POINTER
	}
	wanted, ok := ifaceForIID(xruntime.PtrFromUintptr[windows.GUID](riid))
	if !ok {
		return w32.COM_E_NOINTERFACE
	}
	p := providerFromThis(this, which)
	if !p.supports(wanted) {
		return w32.COM_E_NOINTERFACE
	}
	p.addRef()
	*target = p.ifacePtr(wanted)
	return w32.COM_S_OK
}

// ifaceForIID returns the interface a client is asking for, and whether this package implements it. IID_IUnknown is
// answered with the IRawElementProviderSimple table, whose first three slots are the IUnknown methods, and the one
// interface that is another's base is answered with the derived interface's table; see ifaceIIDAliases.
func ifaceForIID(guid *windows.GUID) (which iface, ok bool) {
	if *guid == w32.IIDUnknown {
		return ifaceSimple, true
	}
	for i := range ifaceIIDs {
		if ifaceIIDs[i] == *guid {
			return iface(i), true
		}
	}
	for _, alias := range ifaceIIDAliases {
		if alias.guid == *guid {
			return alias.iface, true
		}
	}
	return 0, false
}

// simpleProviderOptions implements IRawElementProviderSimple::get_ProviderOptions. Reporting
// ProviderOptions_ServerSideProvider alone, without ProviderOptions_UseComThreading, is what declares the provider
// free-threaded; see the comment on ProviderOptions in constants.go for why that is the only workable answer here.
// It is answered even by a stale provider, since it describes the provider rather than the element.
func simpleProviderOptions(_, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	*xruntime.PtrFromUintptr[ProviderOptions](out) = ProviderOptions_ServerSideProvider
	return w32.COM_S_OK
}

// simpleGetPatternProvider implements IRawElementProviderSimple::GetPatternProvider. A pattern the element does not
// support is answered with a NULL interface and S_OK, which is how a client is told the element simply does not do
// that; an error would say the provider is broken.
//
// Which patterns the element has is decided by supports, the same way QueryInterface decides it, so that a client that
// reaches a pattern by identifier and one that asks for its interface directly cannot be given different answers.
func simpleGetPatternProvider(this, patternID, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := providerFromThis(this, ifaceSimple)
	if _, _, ok := p.current(); !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	pattern := PatternSetForID(PatternID(int32(uint32(patternID))))
	if pattern == 0 {
		return w32.COM_S_OK
	}
	which, ok := ifaceForPattern(pattern)
	if !ok || !p.supports(which) {
		return w32.COM_S_OK
	}
	p.addRef()
	*target = p.ifacePtr(which)
	return w32.COM_S_OK
}

// simpleGetPropertyValue implements IRawElementProviderSimple::GetPropertyValue. A property the element does not
// supply is answered with an empty VARIANT and S_OK, which tells a client to fall back to the host provider or to its
// own default; GetReservedNotSupportedValue exists for the stronger statement that the provider will never supply
// it, which nothing here needs to make.
func simpleGetPropertyValue(this, propertyID, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	value := xruntime.PtrFromUintptr[VARIANT](out)
	*value = VARIANT{}
	p := providerFromThis(this, ifaceSimple)
	tree, node, ok := p.current()
	if !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	p.propertyValue(tree, node, PropertyID(int32(uint32(propertyID))), value)
	return w32.COM_S_OK
}

// propertyValue fills in the VARIANT for one property of one node, leaving it empty for a property this element does
// not supply. Everything it can fill in belongs to UI Automation Core the moment the method returns, so nothing
// allocated here is freed here. See §7.6 of the plan for the list.
func (p *Provider) propertyValue(tree *accessibility.Tree, node *accessibility.Node, propertyID PropertyID,
	value *VARIANT,
) {
	switch propertyID {
	case NamePropertyId:
		// NameString rather than the field, so that the blocks a document is made of are named by their own content
		// when the widget gave them none: Narrator's item navigation speaks the name of each element it steps onto, and
		// a paragraph with no name at all is announced as a bare "text".
		setString(value, NameString(node))
	case HelpTextPropertyId:
		// HelpText carries the watermark of a field that has no description of its own, which is the conventional place
		// for it and the only way an unnamed search field is announced as anything but a bare "edit". Description wins
		// when a node has both: it is what the application said about this control, while the watermark is a hint the
		// widget put in its own empty content, and the two are never concatenated — a client speaks this property as
		// one piece of help text. macOS reports the watermark through accessibilityPlaceholderValue and AT-SPI through
		// the placeholder-text attribute, neither of which has an equivalent here.
		if node.Description != "" {
			setString(value, node.Description)
		} else {
			setString(value, node.Placeholder)
		}
	case FullDescriptionPropertyId:
		setString(value, node.Description)
	case ValueValuePropertyId:
		// The Value pattern's property, answered here as well so that a client reading it through GetPropertyValue —
		// which is how a cell's content is read while walking a row — is told what IValueProvider::get_Value would say.
		// An element without the pattern answers nothing, since the property is not its to report.
		if Patterns(node).Has(PatternValue) {
			// Always a BSTR, even for an empty value, which is what the pattern's getter answers with.
			value.SetBSTR(ValueString(node))
		}
	case ControlTypePropertyId:
		value.SetI4(int32(ControlType(tree, node)))
	case IsEnabledPropertyId:
		value.SetBool(!node.Disabled)
	case IsKeyboardFocusablePropertyId:
		value.SetBool(node.Focusable)
	case HasKeyboardFocusPropertyId:
		value.SetBool(HasKeyboardFocus(tree, node))
	case IsControlElementPropertyId:
		value.SetBool(IsControlElement(node))
	case IsContentElementPropertyId:
		value.SetBool(IsContentElement(tree, node))
	case IsPasswordPropertyId:
		value.SetBool(node.Protected)
	case IsOffscreenPropertyId:
		value.SetBool(node.Offscreen)
	case OrientationPropertyId:
		value.SetI4(int32(Orientation(node)))
	case LevelPropertyId:
		if node.Level > 0 {
			value.SetI4(int32(node.Level))
		}
	case PositionInSetPropertyId:
		// The two halves of one answer, asked for as two properties, so PositionInSet is asked twice — and answers
		// the second time from what it remembered of the first; see snapshotMemo. Neither is reported at all unless
		// the node is one of a numbered set, since a position of zero is not a position.
		if position, _ := PositionInSet(tree, node); position > 0 {
			value.SetI4(int32(position))
		}
	case SizeOfSetPropertyId:
		if position, size := PositionInSet(tree, node); position > 0 {
			value.SetI4(int32(size))
		}
	case LabeledByPropertyId:
		p.setProvider(value, node.LabeledBy)
	case DescribedByPropertyId:
		p.setProviderArray(value, node.DescribedBy)
	case ControllerForPropertyId:
		p.setProviderArray(value, node.Controls)
	case AcceleratorKeyPropertyId:
		setString(value, node.Shortcut)
	case AutomationIdPropertyId:
		value.SetBSTR(strconv.FormatUint(uint64(node.ID), 10))
	case FrameworkIdPropertyId:
		value.SetBSTR(FrameworkID)
	case ItemStatusPropertyId:
		setString(value, ItemStatus(node))
	case IsDataValidForFormPropertyId:
		value.SetBool(!node.Invalid)
	case HeadingLevelPropertyId:
		value.SetI4(int32(HeadingLevel(node)))
	case NativeWindowHandlePropertyId:
		// The property is an int, so a 64-bit handle is reported truncated. That loses nothing: a Windows handle is
		// documented to be significant in its low 32 bits, so that 32-bit and 64-bit processes can hand each other
		// window handles, and this is the only answer a client ever gets — UI Automation asks the fragment first and
		// falls back to the host provider only for a property the fragment leaves empty.
		if p.isRoot() {
			value.SetI4(int32(uint32(uintptr(p.window.HWND()))))
		}
	case IsDialogPropertyId:
		if p.isRoot() {
			value.SetBool(node.Role == role.Dialog)
		}
	default:
		// Including LocalizedControlTypePropertyId, which is deliberately left to UI Automation: it has a
		// localized name for every control type, and ours would be in English only.
		//
		// And LiveSettingPropertyId, which nothing here answers. A client consults it only when it has been given
		// LiveRegionChangedEventId for an element, and this package raises that for nothing: an announcement goes
		// out as a notification event instead, which carries its own ordering — raiseNotification passes
		// NotificationProcessing_All — rather than taking it from a live setting. Declaring the root a polite live
		// region would say that changes inside it are announced by themselves, which is not true of any of them.
	}
}

// setString stores a string property, leaving the VARIANT empty when there is nothing to say. An empty BSTR and no
// answer at all are different things to a client: the first asserts that the element's name really is nothing.
func setString(value *VARIANT, s string) {
	if s != "" {
		value.SetBSTR(s)
	}
}

// setProvider stores the first of the given nodes that has a provider as an interface pointer, which is the shape the
// LabeledBy property takes. The reference Provider handed over becomes the VARIANT's, and through it UI Automation
// Core's.
func (p *Provider) setProvider(value *VARIANT, ids []accessibility.NodeID) {
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
func (p *Provider) setProviderArray(value *VARIANT, ids []accessibility.NodeID) {
	if len(ids) == 0 {
		return
	}
	others := make([]*Provider, 0, len(ids))
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

// simpleHostRawElementProvider implements IRawElementProviderSimple::get_HostRawElementProvider. Only the fragment
// root has a host: the system-supplied provider for the window handle, which fills in the process id, the window
// handle and the window's class name so that the fragment does not have to. Every other element answers NULL, which is
// what says "I am part of a fragment rather than a window of my own".
//
// The staleness check matters more here than anywhere else. Window.Destroy runs before DestroyWindow, so a client
// still holding the fragment root afterwards would have the window handle asked about a window that is being torn down
// — and Windows reuses handles, so once it is gone the same value may name some other window entirely, whose process
// id, handle and class name the client would be handed as this fragment's.
func simpleHostRawElementProvider(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := providerFromThis(this, ifaceSimple)
	if _, _, ok := p.current(); !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	if !p.isRoot() {
		return w32.COM_S_OK
	}
	host, hr := hostProviderFromHwnd(p.window.HWND())
	if !w32.HResultSucceeded(hr) {
		return uint64(hr)
	}
	// The reference HostProviderFromHwnd returned is handed straight to the caller, which is what an out-parameter
	// of an interface type means.
	*target = uintptr(unsafe.Pointer(host))
	return w32.COM_S_OK
}

// fragmentNavigate implements IRawElementProviderFragment::Navigate. Navigation runs over the unignored tree, so
// the layout panels a snapshot marks Ignored are invisible to a client and their children appear in their place. The
// fragment root reports nothing for parent or sibling: a fragment's root is where UI Automation stitches it onto the
// window, and it does that through the host provider instead.
func fragmentNavigate(this, direction, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := providerFromThis(this, ifaceFragment)
	tree, _, ok := p.current()
	if !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	id := Navigate(tree, p.node, NavigateDirection(int32(uint32(direction))))
	if id == 0 {
		return w32.COM_S_OK
	}
	other := p.window.Provider(id)
	if other == nil {
		return w32.COM_S_OK
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*target = other.ifacePtr(ifaceFragment)
	return w32.COM_S_OK
}

// fragmentGetRuntimeID implements IRawElementProviderFragment::GetRuntimeId. The fragment root answers with a NULL
// array, which tells UI Automation to identify it by its window handle; everything else answers with an array that
// begins with AppendRuntimeId, so that UI Automation prepends the root's own identifier and the result stays unique
// across the windows of the process.
//
// This is one of the few methods a stale provider still answers rather than reporting the element as unavailable, and
// it has to be: DisconnectProvider asks a provider for its runtime identifier in order to find the references UI
// Automation holds to it, and that call is made while the provider is being retired. The identifier is derived from the
// node id alone, which never changes, so the answer is right whether or not the node is still in the tree. The other
// two are get_ProviderOptions, which describes the provider rather than the element, and get_FragmentRoot, which a
// disconnect can also bring back — see the comment on Window.Destroy.
func fragmentGetRuntimeID(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[SAFEARRAY](out)
	*target = 0
	p := providerFromThis(this, ifaceFragment)
	if p.isRoot() {
		return w32.COM_S_OK
	}
	array := NewSafeArrayInt32(RuntimeID(p.node))
	if array == 0 {
		return w32.COM_E_OUTOFMEMORY
	}
	*target = array
	return w32.COM_S_OK
}

// fragmentBoundingRectangle implements IRawElementProviderFragment::get_BoundingRectangle, converting the node's
// window-local logical bounds into screen pixels. An element that is scrolled or clipped out of view has no meaningful
// rectangle and reports an empty one, which is what pairs with the IsOffscreen property.
func fragmentBoundingRectangle(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	rect := xruntime.PtrFromUintptr[Rect](out)
	*rect = Rect{}
	p := providerFromThis(this, ifaceFragment)
	_, node, ok := p.current()
	if !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	if node.Offscreen || node.Bounds.Empty() {
		return w32.COM_S_OK
	}
	left, top, width, height := p.window.Geometry().ScreenRect(node.Bounds)
	*rect = Rect{Left: left, Top: top, Width: width, Height: height}
	return w32.COM_S_OK
}

// fragmentGetEmbeddedFragmentRoots implements IRawElementProviderFragment::GetEmbeddedFragmentRoots. Unison draws
// every widget itself, so a window never contains another framework's fragment and the answer is always a NULL array.
func fragmentGetEmbeddedFragmentRoots(_, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = 0
	return w32.COM_S_OK
}

// fragmentSetFocus implements IRawElementProviderFragment::SetFocus by asking the window to give the node the
// keyboard focus. The request is queued onto the UI thread and this returns at once, which is what UI Automation
// expects: the focus change is reported later, as an event.
//
// A disabled node refuses with E_ELEMENTNOTENABLED, as every pattern write path here does, and a node that does not
// offer the Focus action refuses too: the snapshot's action set is its own statement that nothing will happen, and
// Window.dispatchAccessibilityAction drops such a request, so answering S_OK would have a client waiting for a focus
// event that is never coming. The two really can disagree with Focusable — axDisabledActions narrows a disabled node's
// actions while resolveFocus goes on marking an open menu's node focusable, and an Accessibility.Callback that sets
// Disabled leaves Focusable untouched — which is why both are checked. Both other adapters refuse the same request;
// see GrabFocus in internal/atspi and axPerform in internal/cocoa.
func fragmentSetFocus(this uintptr) uint64 {
	p := providerFromThis(this, ifaceFragment)
	_, node, ok := p.current()
	if !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	if !node.Focusable || !node.Actions.Has(accessibility.Focus) {
		return E_INVALIDOPERATION
	}
	if !p.window.dispatch(accessibility.ActionRequest{Node: p.node, Action: accessibility.Focus}) {
		return E_INVALIDOPERATION
	}
	return w32.COM_S_OK
}

// fragmentFragmentRoot implements IRawElementProviderFragment::get_FragmentRoot. Every element in the window,
// including the root itself, reports the same fragment root.
func fragmentFragmentRoot(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := providerFromThis(this, ifaceFragment)
	root := p.window.Root()
	if root == nil {
		return E_ELEMENTNOTAVAILABLE
	}
	// The reference Root handed over becomes the caller's, which is what an interface out-parameter means.
	*target = root.ifacePtr(ifaceFragmentRoot)
	return w32.COM_S_OK
}

// fragmentRootElementProviderFromPoint implements IRawElementProviderFragmentRoot::ElementProviderFromPoint. It is
// reached through an assembly thunk, so the two coordinates arrive as the raw bits of the doubles rather than as
// doubles; see thunk_windows.go. The point is in screen pixels and the snapshot's bounds are in window-local
// logical units, so the window's geometry converts before the tree is asked.
//
// A point over nothing — outside the window, or over an element the snapshot marks offscreen — is answered with a NULL
// element and S_OK, which lets UI Automation fall back to the window itself.
func fragmentRootElementProviderFromPoint(this, xBits, yBits, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := providerFromThis(this, ifaceFragmentRoot)
	tree, _, ok := p.current()
	if !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	id := HitTest(tree, p.window.Geometry().WindowPoint(math.Float64frombits(uint64(xBits)),
		math.Float64frombits(uint64(yBits))))
	if id == 0 {
		return w32.COM_S_OK
	}
	hit := p.window.Provider(id)
	if hit == nil {
		return w32.COM_S_OK
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*target = hit.ifacePtr(ifaceFragment)
	return w32.COM_S_OK
}

// fragmentRootGetFocus implements IRawElementProviderFragmentRoot::GetFocus. It answers only while the window is the
// active one: pointing a client at an element inside a window the user is not looking at makes a screen reader jump
// away from where the user is.
func fragmentRootGetFocus(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	p := providerFromThis(this, ifaceFragmentRoot)
	tree, node, ok := p.current()
	if !ok {
		return E_ELEMENTNOTAVAILABLE
	}
	if !node.Focused || tree.Focus == 0 {
		return w32.COM_S_OK
	}
	focus := p.window.Provider(tree.Focus)
	if focus == nil {
		return w32.COM_S_OK
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*target = focus.ifacePtr(ifaceFragment)
	return w32.COM_S_OK
}

// adviseEventAdded implements IRawElementProviderAdviseEvents::AdviseEventAdded. UI Automation calls it when a
// client starts listening to something within this fragment. Nothing is filtered on it — ClientsAreListening is
// what gates event raising — so it only keeps a count, for diagnostics and for tests.
func adviseEventAdded(this, _, _ uintptr) uint64 {
	providerFromThis(this, ifaceAdviseEvents).window.adviseEvents(1)
	return w32.COM_S_OK
}

// adviseEventRemoved implements IRawElementProviderAdviseEvents::AdviseEventRemoved.
func adviseEventRemoved(this, _, _ uintptr) uint64 {
	providerFromThis(this, ifaceAdviseEvents).window.adviseEvents(-1)
	return w32.COM_S_OK
}

// windowSetVisualState implements IWindowProvider::SetVisualState. Maximizing and minimizing a window through
// accessibility is not offered: the root package has no such API, and a client that cannot do it falls back to the
// window's own system menu.
func windowSetVisualState(_, _ uintptr) uint64 {
	return E_NOTSUPPORTED
}

// windowClose implements IWindowProvider::Close. Closing a window is a decision for the application, which may have
// unsaved work to ask about, so it is not offered here either.
func windowClose(_ uintptr) uint64 {
	return E_NOTSUPPORTED
}

// windowWaitForInputIdle implements IWindowProvider::WaitForInputIdle. Waiting for the UI thread from a provider
// method would deadlock whenever the UI thread is the one waiting on us, so it is never offered.
func windowWaitForInputIdle(_, _, out uintptr) uint64 {
	if out != 0 {
		setBOOL(out, false)
	}
	return E_NOTSUPPORTED
}

// windowCanMaximize implements IWindowProvider::get_CanMaximize. A window the snapshot reports as not resizable has
// no maximize box — w32WindowStyle leaves WS_MAXIMIZEBOX out for one — so Resizable is the answer, rather than a
// constant that tells a client a fixed-size dialog can be maximized and leaves it to discover otherwise when
// SetVisualState refuses.
func windowCanMaximize(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceWindow, out, func(n *accessibility.Node) bool { return n.Resizable })
}

// windowCanMinimize implements IWindowProvider::get_CanMinimize. Every window unison creates has a minimize box,
// including the undecorated ones, so the answer is always yes — but it is still answered through patternNode, so
// that an element whose node has left the tree says so rather than reporting on a window it no longer stands for.
func windowCanMinimize(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceWindow, out, func(_ *accessibility.Node) bool { return true })
}

// windowIsModal implements IWindowProvider::get_IsModal, which is how a client knows to keep the user inside this
// window until it is dealt with. Like every other control-pattern method it goes through patternNode, so that an
// element that no longer has the pattern — or never should have had it — says so rather than answering for a window
// that is not it.
func windowIsModal(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceWindow, out, func(n *accessibility.Node) bool { return n.Modal })
}

// windowVisualState implements IWindowProvider::get_WindowVisualState. The snapshot does not record whether a window
// is maximized or minimized — a minimized window has nothing worth reporting anyway — so every window reports itself
// normal. It still goes through patternNode, so that an element whose node has left the tree is reported as gone
// rather than answered for.
func windowVisualState(this, out uintptr) uint64 {
	return patternInt32(this, ifaceWindow, out, func(_ *accessibility.Node) int32 {
		return int32(WindowVisualState_Normal)
	})
}

// windowInteractionStateValue implements IWindowProvider::get_WindowInteractionState. A window the snapshot reports
// as disabled is disabled because something modal sits in front of it, which is the distinction UI Automation wants
// here.
func windowInteractionStateValue(this, out uintptr) uint64 {
	return patternInt32(this, ifaceWindow, out, func(n *accessibility.Node) int32 {
		return int32(WindowInteractionStateOf(n))
	})
}

// windowIsTopmost implements IWindowProvider::get_IsTopmost, which is the snapshot's Floating flag: a window created
// with FloatingWindowOption is given WS_EX_TOPMOST by w32WindowExStyle, and it really does stay in front of the
// ordinary windows, which is what a client wants to know before it describes the window's place on screen.
func windowIsTopmost(this, out uintptr) uint64 {
	return patternBOOL(this, ifaceWindow, out, func(n *accessibility.Node) bool { return n.Floating })
}

// setBOOL stores a Win32 BOOL into an out-parameter. Unlike a VARIANT_BOOL, true is one rather than every bit set.
// The caller must have checked that out is not NULL.
func setBOOL(out uintptr, value bool) {
	boolean := int32(0)
	if value {
		boolean = 1
	}
	*xruntime.PtrFromUintptr[int32](out) = boolean
}
