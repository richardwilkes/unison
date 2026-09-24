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
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// This file holds the one COM object in the adapter that does not stand for an element: a stretch of the text one
// element hands the Text pattern out over, which is what a client reads that text through. The element is a Markdown
// view answering from its composed stream, or a field, a label, a heading or a cell answering from its own text; the
// range calls it its owner and treats the two alike, as UI Automation does in calling either one the pattern's
// document. It is built on the same template as the data object in data_object_windows.go — the virtual method table
// pointer first, a reference count, and a pinner — rather than on the provider's multiple-interface layout, because a
// range is one interface: ITextRangeProvider2, whose first eighteen methods are ITextRangeProvider's.
//
// A range is a pair of offsets and nothing else. It does not hold the snapshot it came from, and every method re-reads
// its owner's text from the window's current snapshot and clamps the offsets to what is there now, because a client
// keeps a range for as long as it likes: Narrator's scan mode holds one while the user reads, and a publish in between
// may have rewritten the text, shortened it, or removed the element from the window altogether. Clamping is what turns
// "the text is now shorter than this range" into a shorter range rather than into an out-of-bounds read, and an owner
// that has gone reports E_ELEMENTNOTAVAILABLE, which is what a client holding something that no longer exists must be
// told.
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

// liveRanges holds every text range that still has a COM reference outstanding, which is what keeps one reachable
// for the garbage collector and what lets a range a client hands back be recognized as one of ours.
//
// A range cannot anchor itself, for the reason liveProviders gives: a pinner keeps the object from moving and makes
// it legal to hand its address out, but it is documented to need keeping alive independently of what it pins. UI
// Automation releases references from whichever thread it likes, so both ends are under the lock.
var liveRanges = struct {
	set  map[*TextRange]struct{}
	lock sync.Mutex
}{set: make(map[*TextRange]struct{})}

// holdRange anchors a range for as long as anything holds a COM reference to it. See liveRanges.
func holdRange(r *TextRange) {
	liveRanges.lock.Lock()
	defer liveRanges.lock.Unlock()
	liveRanges.set[r] = struct{}{}
}

// dropRange gives up the anchor holdRange took, which the release of the last reference does.
func dropRange(r *TextRange) {
	liveRanges.lock.Lock()
	defer liveRanges.lock.Unlock()
	delete(liveRanges.set, r)
}

// liveRangeCount returns how many ranges are anchored. It is for tests, which is the only thing that can see the set
// at a moment when it should be empty.
func liveRangeCount() int {
	liveRanges.lock.Lock()
	defer liveRanges.lock.Unlock()
	return len(liveRanges.set)
}

// lookupRange returns the range an interface pointer a client handed over belongs to, or nil when no range of this
// process is at that address.
//
// Four of a range's methods are given another range — Compare, CompareEndpoints, MoveEndpointByRange and the
// FindAttribute of a client comparing links — and a pointer from a client is not to be trusted: it may be a range from
// another provider entirely, which is an invalid argument rather than a fault, and following it to find out would be a
// crash at best. Membership is decided by comparing addresses, never by dereferencing.
func lookupRange(this uintptr) *TextRange {
	if this == 0 {
		return nil
	}
	liveRanges.lock.Lock()
	defer liveRanges.lock.Unlock()
	for r := range liveRanges.set {
		if r.this() == this {
			return r
		}
	}
	return nil
}

// TextRange is the UI Automation text range for a stretch of one element's text: an ITextRangeProvider2, which is the
// only thing a client can read that text, its attributes and its rectangles through.
//
// Like Provider it is never used through this Go type by anything but the package's own bookkeeping — UI Automation
// holds the address of its virtual method table pointer instead — and its lifetime is the COM reference count's to
// decide: it stays reachable and pinned until the last reference is released, since UI Automation's pointers are
// invisible to Go.
type TextRange struct {
	// lpVtbl MUST BE FIRST: the pointer a client holds for the range is the address of this field, which is the address
	// of the object, and every method recovers the object from it.
	lpVtbl uintptr
	window *Window
	pinner runtime.Pinner
	// owner is the element whose text the range is a stretch of: a Markdown view, a field, a label, a cell. It is
	// what the range is resolved against and what every action it dispatches is addressed to.
	owner    accessibility.NodeID
	start    int
	end      int
	refCount int32
	lock     sync.Mutex
}

// newTextRange creates the range for one stretch of one element's text, holding the one reference its creator owns.
// The range is anchored and pinned here, together, and both are given up by the release of the last reference.
//
// The offsets are remembered as they are given, and clamped on every use rather than now: the text they name may grow
// or shrink while a client holds the range, and a range clamped at birth would answer from the wrong part of text
// that has since grown.
func newTextRange(window *Window, owner accessibility.NodeID, start, end int) *TextRange {
	ensureVtbls()
	r := &TextRange{
		window:   window,
		owner:    owner,
		start:    start,
		end:      end,
		refCount: 1,
	}
	r.lpVtbl = uintptr(unsafe.Pointer(&textRangeVtbl[0]))
	r.pinner.Pin(r)
	holdRange(r)
	return r
}

// buildTextRangeVtbl fills in the text range's virtual method table. The order of the methods is the order
// ITextRangeProvider declares them in, with ITextRangeProvider2's one addition last, and is the whole content of the
// ABI contract with UI Automation: a method in the wrong slot is called with another method's arguments.
//
// The three IUnknown slots are built here rather than shared with buildVtbl, because a range is a single-interface
// object: its this pointer is the object itself, with no interface offset to subtract.
func buildTextRangeVtbl() {
	textRangeVtbl[0] = windows.NewCallback(textRangeQueryInterface)
	textRangeVtbl[1] = windows.NewCallback(textRangeAddRef)
	textRangeVtbl[2] = windows.NewCallback(textRangeRelease)
	for i, method := range []any{
		textRangeClone,
		textRangeCompare,
		textRangeCompareEndpoints,
		textRangeExpandToEnclosingUnit,
		textRangeFindAttribute,
		textRangeFindText,
		textRangeGetAttributeValue,
		textRangeGetBoundingRectangles,
		textRangeGetEnclosingElement,
		textRangeGetText,
		textRangeMove,
		textRangeMoveEndpointByUnit,
		textRangeMoveEndpointByRange,
		textRangeSelect,
		textRangeAddToSelection,
		textRangeRemoveFromSelection,
		textRangeScrollIntoView,
		textRangeGetChildren,
		textRangeShowContextMenu,
	} {
		textRangeVtbl[unknownSlots+i] = windows.NewCallback(method)
	}
}

// Unknown returns the range's interface pointer, which is also its IUnknown pointer: the table's first three slots are
// the IUnknown methods. No reference is added.
func (r *TextRange) Unknown() unsafe.Pointer {
	return unsafe.Pointer(r)
}

// this returns the address a client holds the range by, for comparing against a pointer a client handed back. It is
// never used to reach the object — a range recovers itself from its own this pointer instead — so nothing here depends
// on the address of an object Go might have moved: the pinner is what makes the address stable in the first place.
func (r *TextRange) this() uintptr {
	return uintptr(unsafe.Pointer(r))
}

// addRef implements IUnknown::AddRef.
func (r *TextRange) addRef() uintptr {
	return w32.ComAddRef(&r.refCount)
}

// release implements IUnknown::Release, unpinning the object and letting go of the anchor that keeps it reachable once
// the last reference is gone and not before: UI Automation may still hold pointers to it that Go cannot see. Exactly
// one caller observes the final release, so neither is given up twice.
func (r *TextRange) release() uintptr {
	remaining, final := w32.ComRelease(&r.refCount)
	if final {
		r.pinner.Unpin()
		dropRange(r)
	}
	return remaining
}

// offsets returns the stretch the range currently stands for, unclamped. It is under the lock, since the methods that
// move a range may run on another thread than the one reading it.
func (r *TextRange) offsets() (start, end int) {
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.start, r.end
}

// store records a new stretch for the range, which is what every method that moves one ends with.
func (r *TextRange) store(start, end int) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.start, r.end = start, end
}

// resolve reads the text this range stands for out of the window's current snapshot and clamps the range to it. hr is
// w32.COM_S_OK when the method may go ahead, and otherwise E_ELEMENTNOTAVAILABLE: the window has been destroyed, the
// owner has left the tree, or it no longer carries text — a field that became Protected, a block a document has since
// claimed — all of which are the same thing to a client holding a range for text that is no longer there.
func (r *TextRange) resolve() (doc *textDocument, start, end int, hr uint64) {
	if r.window == nil {
		return nil, 0, 0, E_ELEMENTNOTAVAILABLE
	}
	p := r.window.Provider(r.owner)
	if p == nil {
		return nil, 0, 0, E_ELEMENTNOTAVAILABLE
	}
	defer p.release()
	tree, node, ok := p.current()
	if !ok {
		return nil, 0, 0, E_ELEMENTNOTAVAILABLE
	}
	if doc = memoizedTextDocument(tree, node.ID); doc == nil {
		return nil, 0, 0, E_ELEMENTNOTAVAILABLE
	}
	start, end = r.offsets()
	start, end = doc.ClampRange(start, end)
	return doc, start, end, w32.COM_S_OK
}

// dispatch hands one action request to the window on the owner's behalf, filling in the owner as the node it is
// about. A window with nowhere to send the request reports the operation as impossible rather than as done.
func (r *TextRange) dispatch(request accessibility.ActionRequest) uint64 {
	request.Node = r.owner
	if r.window == nil || !r.window.dispatch(request) {
		return E_INVALIDOPERATION
	}
	return w32.COM_S_OK
}

// rangeFromThis recovers the range a COM method was called on. A range is a single-interface object whose table
// pointer is its first field, so the this pointer is the object itself.
func rangeFromThis(this uintptr) *TextRange {
	return xruntime.PtrFromUintptr[TextRange](this)
}

// textRangeQueryInterface implements IUnknown::QueryInterface. A range answers to IUnknown and to both range
// interfaces, all with the one table, and refuses everything else. The out-parameter is cleared before anything else
// can fail, as QueryInterface requires.
func textRangeQueryInterface(this, riid, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	target := xruntime.PtrFromUintptr[uintptr](out)
	*target = 0
	if riid == 0 {
		return w32.COM_E_POINTER
	}
	guid := xruntime.PtrFromUintptr[windows.GUID](riid)
	if *guid != w32.IIDUnknown && *guid != iidITextRangeProvider && *guid != iidITextRangeProvider2 {
		return w32.COM_E_NOINTERFACE
	}
	rangeFromThis(this).addRef()
	*target = this
	return w32.COM_S_OK
}

// textRangeAddRef implements IUnknown::AddRef.
func textRangeAddRef(this uintptr) uintptr {
	return rangeFromThis(this).addRef()
}

// textRangeRelease implements IUnknown::Release.
func textRangeRelease(this uintptr) uintptr {
	return rangeFromThis(this).release()
}

// textRangeClone implements ITextRangeProvider::Clone, which is how a client keeps hold of where it was while it
// moves a range: the copy stands for the same stretch of the same document and moves independently from then on.
func textRangeClone(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	r := rangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	// The reference the new range holds becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = newTextRange(r.window, r.owner, start, end).this()
	return w32.COM_S_OK
}

// textRangeCompare implements ITextRangeProvider::Compare, which asks whether two ranges stand for the same stretch
// of the same text. A range from another provider is not equal to this one rather than an error, since a client may
// legitimately compare ranges it collected from several places; a pointer that is no range of ours at all is an invalid
// argument.
//
// A range whose owner can no longer be read is answered the same way. If the other range's owner has left the tree or
// lost its text while this one is still readable, the two plainly do not stand for the same text, so a client — one
// comparing a range it cached against a fresh one across a publish — is better told FALSE than handed an error for a
// range it passed in good faith. This range's own owner is a different matter: a range that cannot say what it stands
// for cannot answer at all.
func textRangeCompare(this, other, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	setBOOL(out, false)
	r := rangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	target := lookupRange(other)
	if target == nil {
		return w32.COM_E_INVALIDARG
	}
	_, targetStart, targetEnd, targetHR := target.resolve()
	if targetHR != w32.COM_S_OK {
		return w32.COM_S_OK
	}
	setBOOL(out, r.owner == target.owner && r.window == target.window && start == targetStart &&
		end == targetEnd)
	return w32.COM_S_OK
}

// textRangeCompareEndpoints implements ITextRangeProvider::CompareEndpoints, reporting a negative number when this
// range's endpoint comes first in the text, zero when the two are in the same place, and a positive number when it
// comes later.
//
// Two ranges over different owners cannot be compared: there is no order between the text of one element and the text
// of another, so the answer is E_INVALIDARG rather than a number a client would read as an ordering.
func textRangeCompareEndpoints(this, endpoint, other, otherEndpoint, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearInt32(out)
	mineEnd := TextPatternRangeEndpoint(int32(uint32(endpoint)))
	theirsEnd := TextPatternRangeEndpoint(int32(uint32(otherEndpoint)))
	if !textEndpointValid(mineEnd) || !textEndpointValid(theirsEnd) {
		return w32.COM_E_INVALIDARG
	}
	r := rangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	target := lookupRange(other)
	if target == nil {
		return w32.COM_E_INVALIDARG
	}
	if target.owner != r.owner || target.window != r.window {
		return w32.COM_E_INVALIDARG
	}
	_, targetStart, targetEnd, hr := target.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	mine := endpointOffset(mineEnd, start, end)
	theirs := endpointOffset(theirsEnd, targetStart, targetEnd)
	*xruntime.PtrFromUintptr[int32](out) = int32(mine - theirs)
	return w32.COM_S_OK
}

// endpointOffset returns the offset one end of a range sits at. Anything but the end endpoint is the start, which
// nothing reaches: every method that takes an endpoint from a client refuses a value outside the enumeration with
// E_INVALIDARG first, since silently moving or comparing the start when a client asked for something else is a wrong
// answer it has no way to notice. See textEndpointValid.
func endpointOffset(endpoint TextPatternRangeEndpoint, start, end int) int {
	if endpoint == TextPatternRangeEndpoint_End {
		return end
	}
	return start
}

// textRangeExpandToEnclosingUnit implements ITextRangeProvider::ExpandToEnclosingUnit, which is how a client turns a
// caret position into the line, word or character to read out. See textDocument.Expand for the eight cases.
func textRangeExpandToEnclosingUnit(this, unit uintptr) uint64 {
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	textUnit := TextUnit(int32(uint32(unit)))
	if !textUnitValid(textUnit) {
		return w32.COM_E_INVALIDARG
	}
	r.store(doc.Expand(textUnit, start, end))
	return w32.COM_S_OK
}

// textRangeFindAttribute implements ITextRangeProvider::FindAttribute, which is how a client jumps to the next link,
// the next heading or the next passage in a different font without reading everything in between.
//
// The VARIANT holding the value to look for is passed by value in the interface's declaration, which on both of the
// architectures this builds for means a pointer to a copy the caller owns — a VARIANT is too large for a register pair
// — so nothing here frees it. A value of a type no stretch of text can have finds nothing, which is reported as a NULL
// range with S_OK: "there is none" is an answer rather than a failure.
func textRangeFindAttribute(this, attributeID, value, backward, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	if value == 0 {
		return w32.COM_E_POINTER
	}
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	want := attributeFromVariant(xruntime.PtrFromUintptr[VARIANT](value))
	foundStart, foundEnd, ok := doc.FindAttribute(start, end, TextAttributeID(int32(uint32(attributeID))), want,
		backward != 0)
	if !ok {
		return w32.COM_S_OK
	}
	// The reference the new range holds becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = newTextRange(r.window, r.owner, foundStart, foundEnd).this()
	return w32.COM_S_OK
}

// textRangeFindText implements ITextRangeProvider::FindText. The string arrives as a BSTR that belongs to the
// caller, so it is decoded into a Go string and nothing here frees it. Text that is not in the range is reported as a
// NULL range with S_OK, which is how a client is told the search found nothing.
func textRangeFindText(this, text, backward, ignoreCase, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	if text == 0 {
		return w32.COM_E_INVALIDARG
	}
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	foundStart, foundEnd, ok := doc.FindText(start, end, BSTRToString(BSTR(text)), backward != 0, ignoreCase != 0)
	if !ok {
		return w32.COM_S_OK
	}
	// The reference the new range holds becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = newTextRange(r.window, r.owner, foundStart, foundEnd).this()
	return w32.COM_S_OK
}

// textRangeGetAttributeValue implements ITextRangeProvider::GetAttributeValue. The VARIANT it fills in belongs to UI
// Automation Core from the moment this returns, so nothing allocated here is freed here — with the one exception the
// two reserved values are; see storeReserved.
func textRangeGetAttributeValue(this, attributeID, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	value := xruntime.PtrFromUintptr[VARIANT](out)
	*value = VARIANT{}
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	answer := doc.Attribute(start, end, TextAttributeID(int32(uint32(attributeID))))
	switch answer.Kind {
	case attributeUnsupported:
		return storeReserved(value, getReservedNotSupportedValue)
	case attributeMixed:
		return storeReserved(value, getReservedMixedAttributeValue)
	case attributeEmpty:
		// An empty VARIANT, which says the attribute applies to this text and there is nothing of it here — the answer
		// for text that is not part of a link.
	case attributeBoolean:
		value.SetBool(answer.Bool)
	case attributeInteger:
		value.SetI4(answer.Int)
	case attributeNumber:
		value.SetR8(answer.Number)
	case attributeString:
		// An exhausted allocator would otherwise leave a VT_BSTR VARIANT holding a NULL pointer, which a client reads
		// as an empty font or style name rather than as the failure it is. GetText answers the same failure the same
		// way.
		value.SetBSTR(answer.Text)
		if value.Val == 0 {
			*value = VARIANT{}
			return w32.COM_E_OUTOFMEMORY
		}
	case attributeRange:
		// The Link attribute's value is a range over the link, which is how a client reads the text of a link it has
		// found without walking the element tree. The reference the new range holds becomes the VARIANT's, and through
		// it UI Automation Core's.
		value.SetUnknown(newTextRange(r.window, r.owner, answer.Start, answer.End).Unknown())
	}
	return w32.COM_S_OK
}

// attributeFromVariant turns the VARIANT a client passed to FindAttribute into the shape this package compares text
// against. A type no text attribute here is reported as is answered as unsupported, which matches nothing; the VARIANT
// belongs to the caller, so nothing is freed.
func attributeFromVariant(value *VARIANT) attribute {
	switch value.VT {
	case VT_BOOL:
		// Zero is false and every other value is true, which is COM's rule for a VARIANT_BOOL: a client or scripting
		// bridge that marshals true as 1 rather than as VARIANT_TRUE means true, and reading it as false would have its
		// search silently match nothing.
		return attribute{Kind: attributeBoolean, Bool: int16(uint16(value.Val)) != VARIANT_FALSE}
	case VT_I4:
		return attribute{Kind: attributeInteger, Int: int32(uint32(value.Val))}
	case VT_R8:
		return attribute{Kind: attributeNumber, Number: math.Float64frombits(value.Val)}
	case VT_BSTR:
		return attribute{Kind: attributeString, Text: BSTRToString(BSTR(value.Val))}
	default:
		return attribute{Kind: attributeUnsupported}
	}
}

// storeReserved stores one of UI Automation's two reserved attribute values into a VARIANT. They are process-wide
// singletons rather than objects with a lifetime, so they are stored without adding a reference and must not be
// released — the one deliberate exception to this package's rule that a reference handed over is a reference added. UI
// Automation's documentation says so, and VariantClear on such a VARIANT is harmless because the singleton's Release
// does nothing.
//
// A system that cannot produce the singleton is answered with E_NOTSUPPORTED, which is the truth about the
// attribute either way and is a legal answer to GetAttributeValue.
func storeReserved(value *VARIANT, get func() (*w32.Unknown, uintptr)) uint64 {
	reserved, hr := get()
	if !w32.HResultSucceeded(hr) || reserved == nil {
		return E_NOTSUPPORTED
	}
	value.SetUnknown(unsafe.Pointer(reserved))
	return w32.COM_S_OK
}

// textRangeGetBoundingRectangles implements ITextRangeProvider::GetBoundingRectangles, which is what a client draws
// its highlight from. The answer is one rectangle per line the range covers, as four doubles each — left, top, width,
// height — in screen pixels, which is the shape UI Automation wants rather than the Rect a bounding rectangle is
// answered with.
//
// A degenerate range covers no text and so has no rectangles, and neither has a range whose owner is scrolled out of
// view: an empty array is the answer, not a NULL one.
func textRangeGetBoundingRectangles(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
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
		return w32.COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return w32.COM_S_OK
}

// textRangeGetEnclosingElement implements ITextRangeProvider::GetEnclosingElement, which is how a client reading
// with the caret finds out what it has moved into: the innermost element whose own text covers the whole range, or the
// owner itself when no element's does, which is the only answer there is for text that holds no elements.
func textRangeGetEnclosingElement(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	enclosing := r.window.Provider(doc.EnclosingSpan(start, end))
	if enclosing == nil {
		// Whatever the span named has no provider after all, so the owner is what encloses the range. It always has
		// one: this range was reached through it.
		if enclosing = r.window.Provider(r.owner); enclosing == nil {
			return E_ELEMENTNOTAVAILABLE
		}
	}
	// The reference Provider handed over becomes the caller's, which is what an interface out-parameter means.
	*xruntime.PtrFromUintptr[uintptr](out) = enclosing.ifacePtr(ifaceSimple)
	return w32.COM_S_OK
}

// textRangeGetText implements ITextRangeProvider::GetText. maxLength is a count of UTF-16 code units, which is what
// a BSTR's length is, and -1 asks for all of it; see TextClip. The BSTR handed back belongs to UI Automation Core,
// which frees it, so nothing here does.
func textRangeGetText(this, maxLength, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	str := NewBSTR(TextClip(doc.Text(start, end), int(int32(uint32(maxLength)))))
	if str == 0 {
		return w32.COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[BSTR](out) = str
	return w32.COM_S_OK
}

// textRangeMove implements ITextRangeProvider::Move, which is how a client walks a document one unit at a time. The
// count it answers with is how many units the range really moved, which is fewer than were asked for at either end of
// the document — that is how a client knows to stop.
func textRangeMove(this, unit, count, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearInt32(out)
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	textUnit := TextUnit(int32(uint32(unit)))
	if !textUnitValid(textUnit) {
		return w32.COM_E_INVALIDARG
	}
	movedStart, movedEnd, moved := doc.Move(textUnit, int(int32(uint32(count))), start, end)
	r.store(movedStart, movedEnd)
	*xruntime.PtrFromUintptr[int32](out) = int32(moved)
	return w32.COM_S_OK
}

// textRangeMoveEndpointByUnit implements ITextRangeProvider::MoveEndpointByUnit, which is how a client grows or
// shrinks a range: extending a selection by a word, or reading from the caret to the end of the line.
func textRangeMoveEndpointByUnit(this, endpoint, unit, count, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearInt32(out)
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	textUnit := TextUnit(int32(uint32(unit)))
	which := TextPatternRangeEndpoint(int32(uint32(endpoint)))
	if !textUnitValid(textUnit) || !textEndpointValid(which) {
		return w32.COM_E_INVALIDARG
	}
	movedStart, movedEnd, moved := doc.MoveEndpoint(textUnit, which, int(int32(uint32(count))), start, end)
	r.store(movedStart, movedEnd)
	*xruntime.PtrFromUintptr[int32](out) = int32(moved)
	return w32.COM_S_OK
}

// textRangeMoveEndpointByRange implements ITextRangeProvider::MoveEndpointByRange, which moves one end of this range
// to where one end of another range is. An endpoint moved past the other takes it along, leaving the range degenerate,
// since a range whose start had crossed its end would stand for text that runs backwards.
//
// Two ranges over different owners have no common text to move between, so that is E_INVALIDARG, as it is for
// CompareEndpoints.
func textRangeMoveEndpointByRange(this, endpoint, other, otherEndpoint uintptr) uint64 {
	mineEnd := TextPatternRangeEndpoint(int32(uint32(endpoint)))
	theirsEnd := TextPatternRangeEndpoint(int32(uint32(otherEndpoint)))
	if !textEndpointValid(mineEnd) || !textEndpointValid(theirsEnd) {
		return w32.COM_E_INVALIDARG
	}
	r := rangeFromThis(this)
	_, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	target := lookupRange(other)
	if target == nil {
		return w32.COM_E_INVALIDARG
	}
	if target.owner != r.owner || target.window != r.window {
		return w32.COM_E_INVALIDARG
	}
	_, targetStart, targetEnd, hr := target.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	at := endpointOffset(theirsEnd, targetStart, targetEnd)
	if mineEnd == TextPatternRangeEndpoint_End {
		r.store(min(start, at), at)
		return w32.COM_S_OK
	}
	r.store(at, max(end, at))
	return w32.COM_S_OK
}

// textRangeSelect implements ITextRangeProvider::Select, which is a client asking for this stretch of the owner's
// text to become the selection — and, when the range is degenerate, for the caret to be placed there. It is the one
// write path a text range has.
//
// The offsets go to the widget as a SetTextSelection action, which is refused when the owner does not offer it: a
// label has no caret to place, so answering S_OK would have a client waiting for a selection event that is never
// coming.
func textRangeSelect(this uintptr) uint64 {
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	node := doc.Node()
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	if !node.Actions.Has(accessibility.SetTextSelection) {
		return E_INVALIDOPERATION
	}
	return r.dispatch(accessibility.ActionRequest{Action: accessibility.SetTextSelection, Start: start, End: end})
}

// textRangeAddToSelection implements ITextRangeProvider::AddToSelection. Nothing here holds more than one selection
// at a time — get_SupportedTextSelection reports Single — so adding to it means nothing: the widget would replace the
// selection instead, and a client answered S_OK would be told the opposite of what happened. E_INVALIDOPERATION is
// UI Automation's answer for an operation that cannot be performed on this element, as against E_NOTSUPPORTED,
// which would deny the whole pattern.
func textRangeAddToSelection(this uintptr) uint64 {
	return textRangeNoMultipleSelection(this)
}

// textRangeRemoveFromSelection implements ITextRangeProvider::RemoveFromSelection, which is refused for the reason
// AddToSelection is.
func textRangeRemoveFromSelection(this uintptr) uint64 {
	return textRangeNoMultipleSelection(this)
}

// textRangeNoMultipleSelection is the answer of the two methods that only mean something for text holding several
// selections at once. The owner is still resolved first, so that a client holding a range for text that is gone is
// told that rather than that the operation is invalid.
func textRangeNoMultipleSelection(this uintptr) uint64 {
	if _, _, _, hr := rangeFromThis(this).resolve(); hr != w32.COM_S_OK {
		return hr
	}
	return E_INVALIDOPERATION
}

// textRangeScrollIntoView implements ITextRangeProvider::ScrollIntoView, which a client calls to bring the text it
// is about to read into view. alignToTop is ignored: the widget scrolls the smallest amount that makes the range
// visible, which is what a person reading wants and what every ScrollIntoView in this package does.
//
// Like IScrollItemProvider::ScrollIntoView it is allowed while the element is disabled — a screen reader reading a
// disabled document must still be able to bring it on screen — and refused when the owner offers neither action.
//
// An owner that cannot scroll to a stretch of its text, but can bring itself into view, does that instead. A label
// draws one line and scrolls nowhere within itself, so bringing the whole of it on screen is the whole of what
// "scroll this text into view" can mean for one, and it is what a client asked for: the text it is about to read
// becomes visible. Only an element that can do neither refuses.
func textRangeScrollIntoView(this, _ uintptr) uint64 {
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	actions := doc.Node().Actions
	if !actions.Has(accessibility.ScrollRangeIntoView) {
		if !actions.Has(accessibility.ScrollIntoView) {
			return E_INVALIDOPERATION
		}
		return r.dispatch(accessibility.ActionRequest{Action: accessibility.ScrollIntoView})
	}
	return r.dispatch(accessibility.ActionRequest{Action: accessibility.ScrollRangeIntoView, Start: start, End: end})
}

// textRangeGetChildren implements ITextRangeProvider::GetChildren, answering with the elements one level down from
// the one that encloses the range that occupy any of its text. The array and the references in it belong to UI
// Automation Core, which destroys the array and releases them.
func textRangeGetChildren(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	array := providerArray(r.window, doc.Children(start, end))
	if array == 0 {
		return w32.COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return w32.COM_S_OK
}

// textRangeShowContextMenu implements ITextRangeProvider2::ShowContextMenu, which is what the applications key does
// while a screen reader's reading cursor is in a stretch of text: the request names the range, and a widget with a
// caret selects it first and opens the menu beneath the caret, at the range's end, so that the menu's Cut and Copy
// apply to it; see unison's axPlaceContextMenuRange. A label has no caret, so its menu opens where it does at any
// assistive technology's request. An owner that does not offer the action refuses.
func textRangeShowContextMenu(this uintptr) uint64 {
	r := rangeFromThis(this)
	doc, start, end, hr := r.resolve()
	if hr != w32.COM_S_OK {
		return hr
	}
	node := doc.Node()
	if node.Disabled {
		return E_ELEMENTNOTENABLED
	}
	if !node.Actions.Has(accessibility.ShowContextMenu) {
		return E_INVALIDOPERATION
	}
	return r.dispatch(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Start: start, End: end})
}
