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
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"golang.org/x/sys/windows"
)

// This file holds the one COM object in the adapter that does not stand for an element: a stretch of one document's
// composed stream, which is what a client reads a document through. It is built on the same template as the data object
// in data_object_windows.go — the virtual method table pointer first, a reference count, and a pinner — rather than on
// the provider's multiple-interface layout, because a range is one interface: ITextRangeProvider2, whose first eighteen
// methods are ITextRangeProvider's.
//
// A range is a pair of offsets and nothing else. It does not hold the snapshot it came from, and every method re-reads
// the document from the window's current snapshot and clamps the offsets to what is there now, because a client keeps a
// range for as long as it likes: Narrator's scan mode holds one while the user reads, and a publish in between may have
// rewritten the document, shortened it, or removed it from the window altogether. Clamping is what turns "the document
// is now shorter than this range" into a shorter range rather than into an out-of-bounds read, and a document that has
// gone reports UIA_E_ELEMENTNOTAVAILABLE, which is what a client holding something that no longer exists must be told.
//
// The offsets are guarded, because a client may move a range from one thread while reading it from another — UI
// Automation calls in on whichever thread it likes, and the providers here declare themselves free-threaded. Nothing
// calls out of the package while holding that lock.

// The two interface identifiers a range answers to. ITextRangeProvider2 derives from ITextRangeProvider, so an object
// that implements the later one implements the earlier one by construction and a client asking for either is handed the
// same object, exactly as the Text pattern's two identifiers are answered with one table.
var (
	iidITextRangeProvider  = xos.Must(windows.GUIDFromString("{5347ad7b-c355-46f8-aff5-909033582f63}"))
	iidITextRangeProvider2 = xos.Must(windows.GUIDFromString("{9bbce42c-1921-4f18-89ca-dba1910a0386}"))
)

// uiaLiveRanges holds every text range that still has a COM reference outstanding, which is what keeps one reachable
// for the garbage collector and what lets a range a client hands back be recognized as one of ours.
//
// A range cannot anchor itself, for the reason uiaLiveProviders gives: a pinner keeps the object from moving and makes
// it legal to hand its address out, but it is documented to need keeping alive independently of what it pins. UI
// Automation releases references from whichever thread it likes, so both ends are under the lock.
var uiaLiveRanges = struct {
	set  map[*UIATextRange]struct{}
	lock sync.Mutex
}{set: make(map[*UIATextRange]struct{})}

// uiaHoldRange anchors a range for as long as anything holds a COM reference to it. See uiaLiveRanges.
func uiaHoldRange(r *UIATextRange) {
	uiaLiveRanges.lock.Lock()
	defer uiaLiveRanges.lock.Unlock()
	uiaLiveRanges.set[r] = struct{}{}
}

// uiaDropRange gives up the anchor uiaHoldRange took, which the release of the last reference does.
func uiaDropRange(r *UIATextRange) {
	uiaLiveRanges.lock.Lock()
	defer uiaLiveRanges.lock.Unlock()
	delete(uiaLiveRanges.set, r)
}

// uiaLiveRangeCount returns how many ranges are anchored. It is for tests, which is the only thing that can see the set
// at a moment when it should be empty.
func uiaLiveRangeCount() int {
	uiaLiveRanges.lock.Lock()
	defer uiaLiveRanges.lock.Unlock()
	return len(uiaLiveRanges.set)
}

// uiaLookupRange returns the range an interface pointer a client handed over belongs to, or nil when no range of this
// process is at that address.
//
// Four of a range's methods are given another range — Compare, CompareEndpoints, MoveEndpointByRange and the
// FindAttribute of a client comparing links — and a pointer from a client is not to be trusted: it may be a range from
// another provider entirely, which is an invalid argument rather than a fault, and following it to find out would be a
// crash at best. Membership is decided by comparing addresses, never by dereferencing.
func uiaLookupRange(this uintptr) *UIATextRange {
	if this == 0 {
		return nil
	}
	uiaLiveRanges.lock.Lock()
	defer uiaLiveRanges.lock.Unlock()
	for r := range uiaLiveRanges.set {
		if r.this() == this {
			return r
		}
	}
	return nil
}

// UIATextRange is the UI Automation text range for a stretch of one document's composed stream: an ITextRangeProvider2,
// which is the only thing a client can read a document's text, attributes and rectangles through.
//
// Like UIAProvider it is never used through this Go type by anything but the package's own bookkeeping — UI Automation
// holds the address of its virtual method table pointer instead — and its lifetime is the COM reference count's to
// decide: it stays reachable and pinned until the last reference is released, since UI Automation's pointers are
// invisible to Go.
type UIATextRange struct {
	// lpVtbl MUST BE FIRST: the pointer a client holds for the range is the address of this field, which is the address
	// of the object, and every method recovers the object from it.
	lpVtbl   uintptr
	window   *UIAWindow
	pinner   runtime.Pinner
	document accessibility.NodeID
	start    int
	end      int
	refCount int32
	lock     sync.Mutex
}

// newUIATextRange creates the range for one stretch of one document's stream, holding the one reference its creator
// owns. The range is anchored and pinned here, together, and both are given up by the release of the last reference.
//
// The offsets are remembered as they are given, and clamped on every use rather than now: the document they name may
// grow or shrink while a client holds the range, and a range clamped at birth would answer from the wrong part of a
// document that has since grown.
func newUIATextRange(window *UIAWindow, document accessibility.NodeID, start, end int) *UIATextRange {
	uiaEnsureVtbls()
	r := &UIATextRange{
		window:   window,
		document: document,
		start:    start,
		end:      end,
		refCount: 1,
	}
	r.lpVtbl = uintptr(unsafe.Pointer(&uiaTextRangeVtbl[0]))
	r.pinner.Pin(r)
	uiaHoldRange(r)
	return r
}

// uiaBuildTextRangeVtbl fills in the text range's virtual method table. The order of the methods is the order
// ITextRangeProvider declares them in, with ITextRangeProvider2's one addition last, and is the whole content of the
// ABI contract with UI Automation: a method in the wrong slot is called with another method's arguments.
//
// The three IUnknown slots are built here rather than shared with uiaBuildVtbl, because a range is a single-interface
// object: its this pointer is the object itself, with no interface offset to subtract.
func uiaBuildTextRangeVtbl() {
	uiaTextRangeVtbl[0] = windows.NewCallback(uiaTextRangeQueryInterface)
	uiaTextRangeVtbl[1] = windows.NewCallback(uiaTextRangeAddRef)
	uiaTextRangeVtbl[2] = windows.NewCallback(uiaTextRangeRelease)
	for i, method := range []any{
		uiaTextRangeClone,
		uiaTextRangeCompare,
		uiaTextRangeCompareEndpoints,
		uiaTextRangeExpandToEnclosingUnit,
		uiaTextRangeFindAttribute,
		uiaTextRangeFindText,
		uiaTextRangeGetAttributeValue,
		uiaTextRangeGetBoundingRectangles,
		uiaTextRangeGetEnclosingElement,
		uiaTextRangeGetText,
		uiaTextRangeMove,
		uiaTextRangeMoveEndpointByUnit,
		uiaTextRangeMoveEndpointByRange,
		uiaTextRangeSelect,
		uiaTextRangeAddToSelection,
		uiaTextRangeRemoveFromSelection,
		uiaTextRangeScrollIntoView,
		uiaTextRangeGetChildren,
		uiaTextRangeShowContextMenu,
	} {
		uiaTextRangeVtbl[uiaUnknownSlots+i] = windows.NewCallback(method)
	}
}

// Unknown returns the range's interface pointer, which is also its IUnknown pointer: the table's first three slots are
// the IUnknown methods. No reference is added.
func (r *UIATextRange) Unknown() unsafe.Pointer {
	return unsafe.Pointer(r)
}

// this returns the address a client holds the range by, for comparing against a pointer a client handed back. It is
// never used to reach the object — a range recovers itself from its own this pointer instead — so nothing here depends
// on the address of an object Go might have moved: the pinner is what makes the address stable in the first place.
func (r *UIATextRange) this() uintptr {
	return uintptr(unsafe.Pointer(r))
}

// addRef implements IUnknown::AddRef.
func (r *UIATextRange) addRef() uintptr {
	return comAddRef(&r.refCount)
}

// release implements IUnknown::Release, unpinning the object and letting go of the anchor that keeps it reachable once
// the last reference is gone and not before: UI Automation may still hold pointers to it that Go cannot see. Exactly
// one caller observes the final release, so neither is given up twice.
func (r *UIATextRange) release() uintptr {
	remaining, final := comRelease(&r.refCount)
	if final {
		r.pinner.Unpin()
		uiaDropRange(r)
	}
	return remaining
}

// offsets returns the stretch the range currently stands for, unclamped. It is under the lock, since the methods that
// move a range may run on another thread than the one reading it.
func (r *UIATextRange) offsets() (start, end int) {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.start, r.end
}

// store records a new stretch for the range, which is what every method that moves one ends with.
func (r *UIATextRange) store(start, end int) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.start, r.end = start, end
}

// resolve reads the document this range stands for out of the window's current snapshot and clamps the range to it. hr
// is COM_S_OK when the method may go ahead, and otherwise UIA_E_ELEMENTNOTAVAILABLE: the window has been destroyed, the
// document has left the tree, or it no longer carries a stream, all of which are the same thing to a client holding a
// range for text that is no longer there.
func (r *UIATextRange) resolve() (doc *uiaTextDocument, start, end int, hr uint64) {
	if r.window == nil {
		return nil, 0, 0, UIA_E_ELEMENTNOTAVAILABLE
	}
	p := r.window.Provider(r.document)
	if p == nil {
		return nil, 0, 0, UIA_E_ELEMENTNOTAVAILABLE
	}
	defer p.release()
	tree, node, ok := p.current()
	if !ok {
		return nil, 0, 0, UIA_E_ELEMENTNOTAVAILABLE
	}
	if doc = uiaMemoizedTextDocument(tree, node.ID); doc == nil {
		return nil, 0, 0, UIA_E_ELEMENTNOTAVAILABLE
	}
	start, end = r.offsets()
	start, end = doc.ClampRange(start, end)
	return doc, start, end, COM_S_OK
}

// dispatch hands one action request to the window on the document's behalf, filling in the document as the node it is
// about. A window with nowhere to send the request reports the operation as impossible rather than as done.
func (r *UIATextRange) dispatch(request accessibility.ActionRequest) uint64 {
	request.Node = r.document
	if r.window == nil || !r.window.dispatch(request) {
		return UIA_E_INVALIDOPERATION
	}
	return COM_S_OK
}

// uiaRangeFromThis recovers the range a COM method was called on. A range is a single-interface object whose table
// pointer is its first field, so the this pointer is the object itself.
func uiaRangeFromThis(this uintptr) *UIATextRange {
	return xruntime.PtrFromUintptr[UIATextRange](this)
}

// uiaTextRangeQueryInterface implements IUnknown::QueryInterface. A range answers to IUnknown and to both range
// interfaces, all with the one table, and refuses everything else. The out-parameter is cleared before anything else
// can fail, as QueryInterface requires.
func uiaTextRangeQueryInterface(this, riid, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	if riid == 0 {
		return COM_E_POINTER
	}
	guid := xruntime.PtrFromUintptr[windows.GUID](riid)
	if *guid != iidUnknown && *guid != iidITextRangeProvider && *guid != iidITextRangeProvider2 {
		return COM_E_NOINTERFACE
	}
	uiaRangeFromThis(this).addRef()
	*target = this
	return COM_S_OK
}

// uiaTextRangeAddRef implements IUnknown::AddRef.
func uiaTextRangeAddRef(this uintptr) uintptr {
	return uiaRangeFromThis(this).addRef()
}

// uiaTextRangeRelease implements IUnknown::Release.
func uiaTextRangeRelease(this uintptr) uintptr {
	return uiaRangeFromThis(this).release()
}

// uiaTextRangeClone implements ITextRangeProvider::Clone, which is how a client keeps hold of where it was while it
// moves a range: the copy stands for the same stretch of the same document and moves independently from then on.
func uiaTextRangeClone(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	r := uiaRangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	// The reference the new range holds becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = newUIATextRange(r.window, r.document, start, end).this()
	return COM_S_OK
}

// uiaTextRangeCompare implements ITextRangeProvider::Compare, which asks whether two ranges stand for the same stretch
// of the same text. A range from another provider is not equal to this one rather than an error, since a client may
// legitimately compare ranges it collected from several places; a pointer that is no range of ours at all is an invalid
// argument.
//
// A range whose document can no longer be read is answered the same way. If the other range's document has left the
// tree or lost its stream while this one is still readable, the two plainly do not stand for the same text, so a client
// — one comparing a range it cached against a fresh one across a publish — is better told FALSE than handed an error
// for a range it passed in good faith. This range's own document is a different matter: a range that cannot say what it
// stands for cannot answer at all.
func uiaTextRangeCompare(this, other, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaSetBOOL(out, false)
	r := uiaRangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	target := uiaLookupRange(other)
	if target == nil {
		return COM_E_INVALIDARG
	}
	_, targetStart, targetEnd, targetHR := target.resolve()
	if targetHR != COM_S_OK {
		return COM_S_OK
	}
	uiaSetBOOL(out, r.document == target.document && r.window == target.window && start == targetStart &&
		end == targetEnd)
	return COM_S_OK
}

// uiaTextRangeCompareEndpoints implements ITextRangeProvider::CompareEndpoints, reporting a negative number when this
// range's endpoint comes first in the text, zero when the two are in the same place, and a positive number when it
// comes later.
//
// Two ranges over different documents cannot be compared: there is no order between the text of one document and the
// text of another, so the answer is E_INVALIDARG rather than a number a client would read as an ordering.
func uiaTextRangeCompareEndpoints(this, endpoint, other, otherEndpoint, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearInt32(out)
	mineEnd := TextPatternRangeEndpoint(int32(uint32(endpoint)))
	theirsEnd := TextPatternRangeEndpoint(int32(uint32(otherEndpoint)))
	if !uiaTextEndpointValid(mineEnd) || !uiaTextEndpointValid(theirsEnd) {
		return COM_E_INVALIDARG
	}
	r := uiaRangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	target := uiaLookupRange(other)
	if target == nil {
		return COM_E_INVALIDARG
	}
	if target.document != r.document || target.window != r.window {
		return COM_E_INVALIDARG
	}
	_, targetStart, targetEnd, hr := target.resolve()
	if hr != COM_S_OK {
		return hr
	}
	mine := uiaEndpointOffset(mineEnd, start, end)
	theirs := uiaEndpointOffset(theirsEnd, targetStart, targetEnd)
	*xruntime.PtrFromUintptr[int32](out) = int32(mine - theirs)
	return COM_S_OK
}

// uiaEndpointOffset returns the offset one end of a range sits at. Anything but the end endpoint is the start, which
// nothing reaches: every method that takes an endpoint from a client refuses a value outside the enumeration with
// E_INVALIDARG first, since silently moving or comparing the start when a client asked for something else is a wrong
// answer it has no way to notice. See uiaTextEndpointValid.
func uiaEndpointOffset(endpoint TextPatternRangeEndpoint, start, end int) int {
	if endpoint == TextPatternRangeEndpoint_End {
		return end
	}
	return start
}

// uiaTextRangeExpandToEnclosingUnit implements ITextRangeProvider::ExpandToEnclosingUnit, which is how a client turns a
// caret position into the line, word or character to read out. See uiaTextDocument.Expand for the eight cases.
func uiaTextRangeExpandToEnclosingUnit(this, unit uintptr) uint64 {
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	textUnit := TextUnit(int32(uint32(unit)))
	if !uiaTextUnitValid(textUnit) {
		return COM_E_INVALIDARG
	}
	r.store(doc.Expand(textUnit, start, end))
	return COM_S_OK
}

// uiaTextRangeFindAttribute implements ITextRangeProvider::FindAttribute, which is how a client jumps to the next link,
// the next heading or the next passage in a different font without reading everything in between.
//
// The VARIANT holding the value to look for is passed by value in the interface's declaration, which on both of the
// architectures this builds for means a pointer to a copy the caller owns — a VARIANT is too large for a register pair
// — so nothing here frees it. A value of a type no stretch of text can have finds nothing, which is reported as a NULL
// range with S_OK: "there is none" is an answer rather than a failure.
func uiaTextRangeFindAttribute(this, attributeID, value, backward, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	if value == 0 {
		return COM_E_POINTER
	}
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	want := uiaAttributeFromVariant(xruntime.PtrFromUintptr[VARIANT](value))
	foundStart, foundEnd, ok := doc.FindAttribute(start, end, TextAttributeID(int32(uint32(attributeID))), want,
		backward != 0)
	if !ok {
		return COM_S_OK
	}
	// The reference the new range holds becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = newUIATextRange(r.window, r.document, foundStart, foundEnd).this()
	return COM_S_OK
}

// uiaTextRangeFindText implements ITextRangeProvider::FindText. The string arrives as a BSTR that belongs to the
// caller, so it is decoded into a Go string and nothing here frees it. Text that is not in the range is reported as a
// NULL range with S_OK, which is how a client is told the search found nothing.
func uiaTextRangeFindText(this, text, backward, ignoreCase, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	if text == 0 {
		return COM_E_INVALIDARG
	}
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	foundStart, foundEnd, ok := doc.FindText(start, end, BSTRToString(BSTR(text)), backward != 0, ignoreCase != 0)
	if !ok {
		return COM_S_OK
	}
	// The reference the new range holds becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = newUIATextRange(r.window, r.document, foundStart, foundEnd).this()
	return COM_S_OK
}

// uiaTextRangeGetAttributeValue implements ITextRangeProvider::GetAttributeValue. The VARIANT it fills in belongs to UI
// Automation Core from the moment this returns, so nothing allocated here is freed here — with the one exception the
// two reserved values are; see uiaStoreReserved.
func uiaTextRangeGetAttributeValue(this, attributeID, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	value := xruntime.PtrFromUintptr[VARIANT](out)
	*value = VARIANT{}
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	answer := doc.Attribute(start, end, TextAttributeID(int32(uint32(attributeID))))
	switch answer.Kind {
	case uiaAttributeUnsupported:
		return uiaStoreReserved(value, uiaGetReservedNotSupportedValue)
	case uiaAttributeMixed:
		return uiaStoreReserved(value, uiaGetReservedMixedAttributeValue)
	case uiaAttributeEmpty:
		// An empty VARIANT, which says the attribute applies to this text and there is nothing of it here — the answer
		// for text that is not part of a link.
	case uiaAttributeBoolean:
		value.SetBool(answer.Bool)
	case uiaAttributeInteger:
		value.SetI4(answer.Int)
	case uiaAttributeNumber:
		value.SetR8(answer.Number)
	case uiaAttributeString:
		// An exhausted allocator would otherwise leave a VT_BSTR VARIANT holding a NULL pointer, which a client reads
		// as an empty font or style name rather than as the failure it is. GetText answers the same failure the same
		// way.
		value.SetBSTR(answer.Text)
		if value.Val == 0 {
			*value = VARIANT{}
			return COM_E_OUTOFMEMORY
		}
	case uiaAttributeRange:
		// The Link attribute's value is a range over the link, which is how a client reads the text of a link it has
		// found without walking the element tree. The reference the new range holds becomes the VARIANT's, and through
		// it UI Automation Core's.
		value.SetUnknown(newUIATextRange(r.window, r.document, answer.Start, answer.End).Unknown())
	}
	return COM_S_OK
}

// uiaAttributeFromVariant turns the VARIANT a client passed to FindAttribute into the shape this package compares text
// against. A type no text attribute here is reported as is answered as unsupported, which matches nothing; the VARIANT
// belongs to the caller, so nothing is freed.
func uiaAttributeFromVariant(value *VARIANT) uiaAttribute {
	switch value.VT {
	case VT_BOOL:
		// Zero is false and every other value is true, which is COM's rule for a VARIANT_BOOL: a client or scripting
		// bridge that marshals true as 1 rather than as VARIANT_TRUE means true, and reading it as false would have its
		// search silently match nothing.
		return uiaAttribute{Kind: uiaAttributeBoolean, Bool: int16(uint16(value.Val)) != VARIANT_FALSE}
	case VT_I4:
		return uiaAttribute{Kind: uiaAttributeInteger, Int: int32(uint32(value.Val))}
	case VT_R8:
		return uiaAttribute{Kind: uiaAttributeNumber, Number: math.Float64frombits(value.Val)}
	case VT_BSTR:
		return uiaAttribute{Kind: uiaAttributeString, Text: BSTRToString(BSTR(value.Val))}
	default:
		return uiaAttribute{Kind: uiaAttributeUnsupported}
	}
}

// uiaStoreReserved stores one of UI Automation's two reserved attribute values into a VARIANT. They are process-wide
// singletons rather than objects with a lifetime, so they are stored without adding a reference and must not be
// released — the one deliberate exception to this package's rule that a reference handed over is a reference added. UI
// Automation's documentation says so, and VariantClear on such a VARIANT is harmless because the singleton's Release
// does nothing.
//
// A system that cannot produce the singleton is answered with UIA_E_NOTSUPPORTED, which is the truth about the
// attribute either way and is a legal answer to GetAttributeValue.
func uiaStoreReserved(value *VARIANT, get func() (*Unknown, uintptr)) uint64 {
	reserved, hr := get()
	if !hresultSucceeded(hr) || reserved == nil {
		return UIA_E_NOTSUPPORTED
	}
	value.SetUnknown(unsafe.Pointer(reserved))
	return COM_S_OK
}

// uiaTextRangeGetBoundingRectangles implements ITextRangeProvider::GetBoundingRectangles, which is what a client draws
// its highlight from. The answer is one rectangle per line the range covers, as four doubles each — left, top, width,
// height — in screen pixels, which is the shape UI Automation wants rather than the UiaRect a bounding rectangle is
// answered with.
//
// A degenerate range covers no text and so has no rectangles, and neither has a range in a document that is scrolled
// out of view: an empty array is the answer, not a NULL one.
func uiaTextRangeGetBoundingRectangles(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	rects := doc.Rectangles(start, end)
	geometry := r.window.Geometry()
	values := make([]float64, 0, 4*len(rects))
	for _, rect := range rects {
		left, top, width, height := geometry.ScreenRect(rect)
		values = append(values, left, top, width, height)
	}
	array := NewSafeArrayFloat64(values)
	if array == 0 {
		return COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return COM_S_OK
}

// uiaTextRangeGetEnclosingElement implements ITextRangeProvider::GetEnclosingElement, which is how a client reading
// with the caret finds out what it has moved into: the innermost element whose own text covers the whole range, or the
// document itself when no element's does.
func uiaTextRangeGetEnclosingElement(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	enclosing := r.window.Provider(doc.EnclosingSpan(start, end))
	if enclosing == nil {
		// Whatever the span named has no provider after all, so the document is what encloses the range. It always has
		// one: this range was reached through it.
		if enclosing = r.window.Provider(r.document); enclosing == nil {
			return UIA_E_ELEMENTNOTAVAILABLE
		}
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = enclosing.ifacePtr(uiaIfaceSimple)
	return COM_S_OK
}

// uiaTextRangeGetText implements ITextRangeProvider::GetText. maxLength is a count of UTF-16 code units, which is what
// a BSTR's length is, and -1 asks for all of it; see UIATextClip. The BSTR handed back belongs to UI Automation Core,
// which frees it, so nothing here does.
func uiaTextRangeGetText(this, maxLength, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	str := NewBSTR(UIATextClip(doc.Text(start, end), int(int32(uint32(maxLength)))))
	if str == 0 {
		return COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[BSTR](out) = str
	return COM_S_OK
}

// uiaTextRangeMove implements ITextRangeProvider::Move, which is how a client walks a document one unit at a time. The
// count it answers with is how many units the range really moved, which is fewer than were asked for at either end of
// the document — that is how a client knows to stop.
func uiaTextRangeMove(this, unit, count, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearInt32(out)
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	textUnit := TextUnit(int32(uint32(unit)))
	if !uiaTextUnitValid(textUnit) {
		return COM_E_INVALIDARG
	}
	movedStart, movedEnd, moved := doc.Move(textUnit, int(int32(uint32(count))), start, end)
	r.store(movedStart, movedEnd)
	*xruntime.PtrFromUintptr[int32](out) = int32(moved)
	return COM_S_OK
}

// uiaTextRangeMoveEndpointByUnit implements ITextRangeProvider::MoveEndpointByUnit, which is how a client grows or
// shrinks a range: extending a selection by a word, or reading from the caret to the end of the line.
func uiaTextRangeMoveEndpointByUnit(this, endpoint, unit, count, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearInt32(out)
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	textUnit := TextUnit(int32(uint32(unit)))
	which := TextPatternRangeEndpoint(int32(uint32(endpoint)))
	if !uiaTextUnitValid(textUnit) || !uiaTextEndpointValid(which) {
		return COM_E_INVALIDARG
	}
	movedStart, movedEnd, moved := doc.MoveEndpoint(textUnit, which, int(int32(uint32(count))), start, end)
	r.store(movedStart, movedEnd)
	*xruntime.PtrFromUintptr[int32](out) = int32(moved)
	return COM_S_OK
}

// uiaTextRangeMoveEndpointByRange implements ITextRangeProvider::MoveEndpointByRange, which moves one end of this range
// to where one end of another range is. An endpoint moved past the other takes it along, leaving the range degenerate,
// since a range whose start had crossed its end would stand for text that runs backwards.
//
// Two ranges over different documents have no common text to move between, so that is E_INVALIDARG, as it is for
// CompareEndpoints.
func uiaTextRangeMoveEndpointByRange(this, endpoint, other, otherEndpoint uintptr) uint64 {
	mineEnd := TextPatternRangeEndpoint(int32(uint32(endpoint)))
	theirsEnd := TextPatternRangeEndpoint(int32(uint32(otherEndpoint)))
	if !uiaTextEndpointValid(mineEnd) || !uiaTextEndpointValid(theirsEnd) {
		return COM_E_INVALIDARG
	}
	r := uiaRangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	target := uiaLookupRange(other)
	if target == nil {
		return COM_E_INVALIDARG
	}
	if target.document != r.document || target.window != r.window {
		return COM_E_INVALIDARG
	}
	_, targetStart, targetEnd, hr := target.resolve()
	if hr != COM_S_OK {
		return hr
	}
	at := uiaEndpointOffset(theirsEnd, targetStart, targetEnd)
	if mineEnd == TextPatternRangeEndpoint_End {
		r.store(min(start, at), at)
		return COM_S_OK
	}
	r.store(at, max(end, at))
	return COM_S_OK
}

// uiaTextRangeSelect implements ITextRangeProvider::Select, which is a client asking for this stretch of the document
// to become the selection — and, when the range is degenerate, for the caret to be placed there. It is the one write
// path a text range has.
//
// The offsets go to the widget as a SetTextSelection action, which is refused when the document does not offer it: a
// Markdown view that cannot take the focus has no caret to place, so answering S_OK would have a client waiting for a
// selection event that is never coming.
func uiaTextRangeSelect(this uintptr) uint64 {
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	node := doc.Node()
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	if !node.Actions.Has(accessibility.SetTextSelection) {
		return UIA_E_INVALIDOPERATION
	}
	return r.dispatch(accessibility.ActionRequest{Action: accessibility.SetTextSelection, Start: start, End: end})
}

// uiaTextRangeAddToSelection implements ITextRangeProvider::AddToSelection. A document here holds one selection at a
// time — get_SupportedTextSelection reports Single — so adding to it means nothing: the widget would replace the
// selection instead, and a client answered S_OK would be told the opposite of what happened. UIA_E_INVALIDOPERATION is
// UI Automation's answer for an operation that cannot be performed on this element, as against UIA_E_NOTSUPPORTED,
// which would deny the whole pattern.
func uiaTextRangeAddToSelection(this uintptr) uint64 {
	return uiaTextRangeNoMultipleSelection(this)
}

// uiaTextRangeRemoveFromSelection implements ITextRangeProvider::RemoveFromSelection, which is refused for the reason
// AddToSelection is.
func uiaTextRangeRemoveFromSelection(this uintptr) uint64 {
	return uiaTextRangeNoMultipleSelection(this)
}

// uiaTextRangeNoMultipleSelection is the answer of the two methods that only mean something in a document holding
// several selections at once. The document is still resolved first, so that a client holding a range for text that is
// gone is told that rather than that the operation is invalid.
func uiaTextRangeNoMultipleSelection(this uintptr) uint64 {
	if _, _, _, hr := uiaRangeFromThis(this).resolve(); hr != COM_S_OK {
		return hr
	}
	return UIA_E_INVALIDOPERATION
}

// uiaTextRangeScrollIntoView implements ITextRangeProvider::ScrollIntoView, which a client calls to bring the text it
// is about to read into view. alignToTop is ignored: the widget scrolls the smallest amount that makes the range
// visible, which is what a person reading wants and what every ScrollIntoView in this package does.
//
// Like IScrollItemProvider::ScrollIntoView it is allowed while the element is disabled — a screen reader reading a
// disabled document must still be able to bring it on screen — and refused when the document does not offer the action.
func uiaTextRangeScrollIntoView(this, _ uintptr) uint64 {
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	if !doc.Node().Actions.Has(accessibility.ScrollRangeIntoView) {
		return UIA_E_INVALIDOPERATION
	}
	return r.dispatch(accessibility.ActionRequest{Action: accessibility.ScrollRangeIntoView, Start: start, End: end})
}

// uiaTextRangeGetChildren implements ITextRangeProvider::GetChildren, answering with the elements one level down from
// the one that encloses the range that occupy any of its text. The array and the references in it belong to UI
// Automation Core, which destroys the array and releases them.
func uiaTextRangeGetChildren(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	r := uiaRangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	array := uiaProviderArray(r.window, doc.Children(start, end))
	if array == 0 {
		return COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return COM_S_OK
}

// uiaTextRangeShowContextMenu implements ITextRangeProvider2::ShowContextMenu, which is what the applications key does
// while a screen reader's reading cursor is in a document: the menu opens where the range begins rather than where the
// mouse is.
//
// The caret is placed there first, which the widget does as part of the request: the menu's Copy applies to the
// selection, so a menu opened at one place while the caret sat at another would copy the wrong text. A document that
// does not offer the action refuses, which is what an unfocusable Markdown view does — it has no menu to open.
func uiaTextRangeShowContextMenu(this uintptr) uint64 {
	r := uiaRangeFromThis(this)
	doc, start, _, hr := r.resolve()
	if hr != COM_S_OK {
		return hr
	}
	node := doc.Node()
	if node.Disabled {
		return UIA_E_ELEMENTNOTENABLED
	}
	if !node.Actions.Has(accessibility.ShowContextMenu) {
		return UIA_E_INVALIDOPERATION
	}
	return r.dispatch(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Start: start, End: start})
}
