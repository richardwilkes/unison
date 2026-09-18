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
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// These tests stand in for the UI Automation client a text range is built for, the way the ones in
// provider_windows_test.go do: they call the methods through the range's virtual method table, with the this
// pointer a client would hold, and check what came back. What the answers are supposed to be is settled by the portable
// tests in text_test.go; these check the COM around them — the out-parameters, the HRESULTs, the reference counts,
// the action requests that reach the window, and what happens to a range the document under it changed.

// iidUnsupportedInterface is an interface identifier a text range does not implement — IDataObject's, chosen only
// because it is a real IID that is not one of the range's — used to check that QueryInterface refuses what it does not
// recognize.
var iidUnsupportedInterface = xos.Must(windows.GUIDFromString("{0000010E-0000-0000-C000-000000000046}"))

// callRangeSlot calls one method of a text range the way UI Automation does: indirectly, through the slot its
// virtual method table holds. method is the index among ITextRangeProvider2's own methods, so method 0 is Clone and
// method 18 is ShowContextMenu.
func callRangeSlot(r *TextRange, method int, args ...uintptr) uint64 {
	ensureVtbls()
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, r.this())
	all = append(all, args...)
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(textRangeVtbl[unknownSlots+method], all...)
	return uint64(result)
}

// textRangeWindow creates the adapter for a window showing the fixture document, along with a range over part of its
// stream. The range holds the one reference its creator owns, which the test gives back when it finishes.
func textRangeWindow(t *testing.T, start, end int) (w *testWindow, r *TextRange) {
	t.Helper()
	w = newTestWindow(t, textFixtureTree())
	r = newTextRange(w.Window, textDocumentID, start, end)
	t.Cleanup(func() { r.release() })
	return w, r
}

// rangeFromOut returns the range an out-parameter holds, which must be one this process handed out, and hands back
// the reference that came with it, since nothing in a test is going to release it later.
//
// What comes back is still a live Go object afterwards even though its last COM reference is gone: releasing a range
// unpins it and drops the package's anchor on it, which lets the collector move or collect it, and the pointer the test
// holds is what keeps it from doing either. A client that did this would be dereferencing a pointer it no longer owns,
// which is why no code outside a test may.
func rangeFromOut(c check.Checker, out uintptr) *TextRange {
	r := lookupRange(out)
	c.NotNil(r, "the out-parameter must hold a range this process created")
	if r != nil {
		r.release()
	}
	return r
}

// safeArrayRanges returns the ranges held in a VT_UNKNOWN SAFEARRAY, releasing both the reference that reading each
// one added and the one the array itself holds, and destroying the array. A range in an array is one a provider method
// handed over, so the array is the caller's to destroy, and destroying it is what releases the references it held.
//
// The ranges that come back are still live Go objects, for the reason rangeFromOut gives, so a test may go on
// reading the offsets they stood for.
func safeArrayRanges(c check.Checker, array SAFEARRAY) []*TextRange {
	c.True(array != 0, "the array must not be NULL")
	if array == 0 {
		return nil
	}
	defer array.Destroy()
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(w32.HResultSucceeded(hr))
	if bound < 0 {
		return nil
	}
	ranges := make([]*TextRange, 0, bound+1)
	for i := int32(0); i <= bound; i++ {
		var element uintptr
		c.True(w32.HResultSucceeded(SafeArrayGetElement(array, i, unsafe.Pointer(&element))))
		r := lookupRange(element)
		c.NotNil(r, "element %d must be a range this process created", i)
		if r != nil {
			// The reference reading the element added; the array still holds its own until it is destroyed.
			r.release()
			ranges = append(ranges, r)
		}
	}
	return ranges
}

// safeArrayFloat64 returns the doubles held in a VT_R8 SAFEARRAY and destroys it, which is what a bounding-rectangle
// answer is.
func safeArrayFloat64(c check.Checker, array SAFEARRAY) []float64 {
	c.True(array != 0, "the array must not be NULL")
	if array == 0 {
		return nil
	}
	defer array.Destroy()
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(w32.HResultSucceeded(hr))
	if bound < 0 {
		return nil
	}
	values := make([]float64, bound+1)
	for i := range values {
		c.True(w32.HResultSucceeded(SafeArrayGetElement(array, int32(i), unsafe.Pointer(&values[i]))))
	}
	return values
}

// TestTextRangeVtblSlotOrder verifies that method N of ITextRangeProvider2 really sits in slot N of the range's
// virtual method table. No other test here can: every other one calls the Go functions the slots were built from, and
// those answer the same however the slots are ordered, so two methods of the same shape swapped — Move for
// MoveEndpointByUnit, say — would pass the whole suite while a real client got one answer where it asked for the other.
//
// AddToSelection and RemoveFromSelection are the one pair the order cannot distinguish: both take nothing but the this
// pointer and both refuse with the same HRESULT, since a document here holds one selection at a time. Swapping them
// would change nothing a client could observe.
func TestTextRangeVtblSlotOrder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w, r := textRangeWindow(t, 12, 16)

	// 0: Clone. The clone is handed back to Compare and CompareEndpoints below, and a range only answers for one it
	// can find among those something still holds a reference to, so the reference the clone came with is given back
	// after those two calls rather than here.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 0, out.fresh()))
	clone := lookupRange(out.ptr())
	c.NotNil(clone, "the out-parameter must hold a range this process created")
	if clone == nil {
		return
	}
	defer clone.release()
	start, end := clone.offsets()
	c.Equal(12, start, "Clone")
	c.Equal(16, end)

	// 1: Compare, against the clone, which stands for the same stretch of the same document.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 1, clone.this(), out.fresh()))
	c.Equal(int32(1), out.i32(), "Compare")

	// 2: CompareEndpoints, this range's start against the clone's end.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 2, uintptr(TextPatternRangeEndpoint_Start), clone.this(),
		uintptr(TextPatternRangeEndpoint_End), out.fresh()))
	c.Equal(int32(-4), out.i32(), "CompareEndpoints")

	// 6: GetAttributeValue, read before anything moves the range. The link is the underlined stretch.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 6, uintptr(UnderlineStyleAttributeId), out.fresh()))
	c.Equal(VT_I4, out.variant().VT, "GetAttributeValue")
	c.Equal(int32(TextDecorationLineStyle_Single), variantInt32(out.variant()))

	// 7: GetBoundingRectangles. The link sits on the second line, which the scroll area shows in full.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 7, out.fresh()))
	c.Equal([]float64{100 + 70*2, 50 + 30*2, 40 * 2, 10 * 2}, safeArrayFloat64(c, out.array()),
		"GetBoundingRectangles")

	// 8: GetEnclosingElement, which for the link's own stretch is the link.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 8, out.fresh()))
	c.Equal(w.providerFor(textLinkID).ifacePtr(ifaceSimple), out.ptr(), "GetEnclosingElement")
	providerFromThis(out.ptr(), ifaceSimple).release()

	// 9: GetText, all of it.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 9, uintptr(^uintptr(0)), out.fresh()))
	c.Equal("link", BSTRToString(BSTR(out.ptr())), "GetText")
	BSTR(out.ptr()).Free()

	// 5: FindText, within the document range rather than within this range.
	document := newTextRange(w.Window, textDocumentID, 0, textFixtureLength)
	defer document.release()
	text := NewBSTR("x = 1")
	defer text.Free()
	c.Equal(w32.COM_S_OK, callRangeSlot(document, 5, uintptr(text), 0, 0, out.fresh()))
	found := rangeFromOut(c, out.ptr())
	c.NotNil(found)
	start, end = found.offsets()
	c.Equal(23, start, "FindText")
	c.Equal(28, end)

	// 4: FindAttribute, which takes the value to look for as a VARIANT passed by reference.
	want, wantAddress := pinnedOut[VARIANT](&pin)
	want.SetI4(int32(StyleId_Heading2))
	c.Equal(w32.COM_S_OK, callRangeSlot(document, 4, uintptr(StyleIdAttributeId), wantAddress, 0, out.fresh()))
	found = rangeFromOut(c, out.ptr())
	c.NotNil(found)
	start, end = found.offsets()
	c.Equal(0, start, "FindAttribute")
	c.Equal(5, end)

	// 17: GetChildren, which for the whole document is its five blocks.
	c.Equal(w32.COM_S_OK, callRangeSlot(document, 17, out.fresh()))
	c.Equal(5, len(safeArrayUnknowns(c, out.array())), "GetChildren")
	out.array().Destroy()

	// 3: ExpandToEnclosingUnit, which moves the range, so it comes after everything that reads it.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 3, uintptr(TextUnit_Line)))
	start, end = r.offsets()
	c.Equal(6, start, "ExpandToEnclosingUnit")
	c.Equal(17, end)

	// 10: Move, by one line from the line the range now covers.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 10, uintptr(TextUnit_Line), 1, out.fresh()))
	c.Equal(int32(1), out.i32(), "Move")
	start, end = r.offsets()
	c.Equal(17, start)
	c.Equal(23, end)

	// 10 again, with a negative count, which is the one place the sign extension of a 32-bit count through a uintptr is
	// checked: a client walking backwards by unit — Narrator's scan mode and NVDA's review cursor both do — passes -1,
	// which arrives with every bit set.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 10, uintptr(TextUnit_Line), uintptr(^uintptr(0)), out.fresh()))
	c.Equal(int32(-1), out.i32(), "Move backwards")
	start, end = r.offsets()
	c.Equal(6, start)
	c.Equal(17, end)

	// And forward again, so that what follows starts from the line the rest of the sequence was written against.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 10, uintptr(TextUnit_Line), 1, out.fresh()))
	c.Equal(int32(1), out.i32())
	start, end = r.offsets()
	c.Equal(17, start)
	c.Equal(23, end)

	// 11: MoveEndpointByUnit, pulling the end back by a line.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 11, uintptr(TextPatternRangeEndpoint_End), uintptr(TextUnit_Line),
		uintptr(^uintptr(0)), out.fresh()))
	c.Equal(int32(-1), out.i32(), "MoveEndpointByUnit")
	start, end = r.offsets()
	c.Equal(17, start)
	c.Equal(17, end)

	// 12: MoveEndpointByRange, moving this range's end to the document range's end.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 12, uintptr(TextPatternRangeEndpoint_End), document.this(),
		uintptr(TextPatternRangeEndpoint_End)))
	start, end = r.offsets()
	c.Equal(17, start, "MoveEndpointByRange")
	c.Equal(textFixtureLength, end)

	// 13, 16 and 18: the three methods that ask the widget for something, told apart by the action each dispatches.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 13))
	c.Equal(accessibility.SetTextSelection, requestAt(w, 0).Action, "Select")
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 16, 1))
	c.Equal(accessibility.ScrollRangeIntoView, requestAt(w, 1).Action, "ScrollIntoView")
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 18))
	c.Equal(accessibility.ShowContextMenu, requestAt(w, 2).Action, "ShowContextMenu")
	c.Equal(3, len(w.recorded()))

	// 14 and 15: the two that mean nothing in a document holding one selection at a time.
	c.Equal(E_INVALIDOPERATION, callRangeSlot(r, 14), "AddToSelection")
	c.Equal(E_INVALIDOPERATION, callRangeSlot(r, 15), "RemoveFromSelection")
	c.Equal(3, len(w.recorded()), "neither asks the widget for anything")
}

// TestTextRangeReferenceCountLifetime verifies that a range lives exactly as long as the references to it: it is
// anchored and pinned while one is outstanding, since UI Automation's pointers are invisible to Go, and both are given
// up by the release of the last one and not before.
func TestTextRangeReferenceCountLifetime(t *testing.T) {
	c := check.New(t)
	w := newTestWindow(t, textFixtureTree())
	before := liveRangeCount()
	r := newTextRange(w.Window, textDocumentID, 0, 5)
	c.Equal(before+1, liveRangeCount(), "a new range is anchored")

	// AddRef and Release through the table, which is how UI Automation holds one.
	ensureVtbls()
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	added, _, _ := syscall.SyscallN(textRangeVtbl[1], r.this())
	c.Equal(uintptr(2), added)
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	remaining, _, _ := syscall.SyscallN(textRangeVtbl[2], r.this())
	c.Equal(uintptr(1), remaining)
	c.Equal(before+1, liveRangeCount(), "and is still anchored while one reference is left")

	c.Equal(uintptr(0), r.release())
	c.Equal(before, liveRangeCount(), "the last release gives up the anchor")
}

// TestTextRangeQueryInterface verifies that a range answers to IUnknown and to both range interfaces with the one
// table it has — ITextRangeProvider2 derives from ITextRangeProvider, so an object that implements the later implements
// the earlier — and that it refuses anything else. Every interface handed out is AddRef'd first, as COM requires.
func TestTextRangeQueryInterface(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	_, r := textRangeWindow(t, 0, 5)
	target, targetAddress := pinnedOut[uintptr](&pin)
	for _, guid := range []windows.GUID{w32.IIDUnknown, iidITextRangeProvider, iidITextRangeProvider2} {
		wanted := guid
		pin.Pin(&wanted)
		*target = 0
		//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
		result, _, _ := syscall.SyscallN(textRangeVtbl[0], r.this(), uintptr(unsafe.Pointer(&wanted)), targetAddress)
		c.Equal(w32.COM_S_OK, uint64(result))
		c.Equal(r.this(), *target, "every interface of a range is the range itself")
		c.Equal(uintptr(1), r.release(), "and each one came with a reference of its own")
	}

	// Anything else is refused, with the out-parameter cleared, as QueryInterface requires on every failure.
	*target = 0xdeadbeef
	other := iidUnsupportedInterface
	pin.Pin(&other)
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ := syscall.SyscallN(textRangeVtbl[0], r.this(), uintptr(unsafe.Pointer(&other)), targetAddress)
	c.Equal(w32.COM_E_NOINTERFACE, uint64(result))
	c.Equal(uintptr(0), *target)

	// A NULL interface identifier and a NULL out-parameter are both pointer errors rather than crashes.
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ = syscall.SyscallN(textRangeVtbl[0], r.this(), 0, targetAddress)
	c.Equal(w32.COM_E_POINTER, uint64(result))
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	result, _, _ = syscall.SyscallN(textRangeVtbl[0], r.this(), uintptr(unsafe.Pointer(&other)), 0)
	c.Equal(w32.COM_E_POINTER, uint64(result))
}

// TestTextRangeSelectDispatches verifies the one write path a range has: the offsets it stands for go to the widget
// as a SetTextSelection request, and are refused when the document does not offer it — an unfocusable Markdown view has
// no caret to place, so answering S_OK would leave a client waiting for a selection event that is never coming.
func TestTextRangeSelectDispatches(t *testing.T) {
	c := check.New(t)
	w, r := textRangeWindow(t, 6, 22)
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 13))
	request := requestAt(w, 0)
	c.Equal(accessibility.SetTextSelection, request.Action)
	c.Equal(textDocumentID, request.Node)
	c.Equal(6, request.Start)
	c.Equal(22, request.End)

	// A document that does not offer the action refuses, and a disabled one refuses as not enabled: being unusable now
	// says nothing about whether the selection could be set.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Actions = accessibility.ActionSet(0).With(accessibility.ScrollRangeIntoView)
	plain := newTestWindow(t, tree)
	plainRange := newTextRange(plain.Window, textDocumentID, 0, 5)
	defer plainRange.release()
	c.Equal(E_INVALIDOPERATION, callRangeSlot(plainRange, 13))
	c.Equal(0, len(plain.recorded()))

	disabledTree := textFixtureTree()
	disabledTree.Node(textDocumentID).Disabled = true
	disabled := newTestWindow(t, disabledTree)
	disabledRange := newTextRange(disabled.Window, textDocumentID, 0, 5)
	defer disabledRange.release()
	c.Equal(E_ELEMENTNOTENABLED, callRangeSlot(disabledRange, 13))
	c.Equal(0, len(disabled.recorded()))
}

// TestTextRangeScrollIntoViewDispatches verifies that bringing a stretch of a document into view reaches the widget
// as a ScrollRangeIntoView request, and that it is the one thing a disabled document still does: a screen reader
// reading a disabled document must still be able to bring it on screen.
func TestTextRangeScrollIntoViewDispatches(t *testing.T) {
	c := check.New(t)
	tree := textFixtureTree()
	tree.Node(textDocumentID).Disabled = true
	w := newTestWindow(t, tree)
	r := newTextRange(w.Window, textDocumentID, 23, 28)
	defer r.release()
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 16, 1))
	request := requestAt(w, 0)
	c.Equal(accessibility.ScrollRangeIntoView, request.Action)
	c.Equal(textDocumentID, request.Node)
	c.Equal(23, request.Start)
	c.Equal(28, request.End)

	// A document that does not offer the action refuses rather than reporting a scroll that never happened.
	without := textFixtureTree()
	without.Node(textDocumentID).Actions = accessibility.ActionSet(0).With(accessibility.SetTextSelection)
	plain := newTestWindow(t, without)
	plainRange := newTextRange(plain.Window, textDocumentID, 0, 5)
	defer plainRange.release()
	c.Equal(E_INVALIDOPERATION, callRangeSlot(plainRange, 16, 0))
	c.Equal(0, len(plain.recorded()))
}

// TestTextRangeShowContextMenuDispatches verifies what the applications key does while a screen reader's reading
// cursor is in a document: the menu opens where the range begins, and the request carries that offset at both ends so
// that the widget places the caret there first — the menu's Copy applies to the selection, so a menu opened at one
// place while the caret sat at another would copy the wrong text.
func TestTextRangeShowContextMenuDispatches(t *testing.T) {
	c := check.New(t)
	w, r := textRangeWindow(t, 12, 16)
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 18))
	request := requestAt(w, 0)
	c.Equal(accessibility.ShowContextMenu, request.Action)
	c.Equal(textDocumentID, request.Node)
	c.Equal(12, request.Start)
	c.Equal(12, request.End, "the caret goes to the start of the range, not around it")

	// A document that does not advertise the action has no menu to open.
	without := textFixtureTree()
	without.Node(textDocumentID).Actions = accessibility.ActionSet(0).With(accessibility.SetTextSelection,
		accessibility.ScrollRangeIntoView)
	plain := newTestWindow(t, without)
	plainRange := newTextRange(plain.Window, textDocumentID, 0, 5)
	defer plainRange.release()
	c.Equal(E_INVALIDOPERATION, callRangeSlot(plainRange, 18))
	c.Equal(0, len(plain.recorded()))
}

// TestTextRangeGetAttributeValueReserved verifies the two answers a range gives instead of a value: UI Automation's
// reserved not-supported singleton for an attribute no document here records, and its reserved mixed singleton for a
// range whose text does not agree about one. Both are stored as interface pointers without a reference being added,
// which is the one place in this package where that is right.
//
// The two entry points are replaced here, since a test has no UI Automation Core to get a singleton from, and what is
// checked is what the provider did with what it was given.
func TestTextRangeGetAttributeValueReserved(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	notSupported := &w32.Unknown{}
	mixed := &w32.Unknown{}
	pin.Pin(notSupported)
	pin.Pin(mixed)
	savedNotSupported := getReservedNotSupportedValue
	savedMixed := getReservedMixedAttributeValue
	t.Cleanup(func() {
		getReservedNotSupportedValue = savedNotSupported
		getReservedMixedAttributeValue = savedMixed
	})
	getReservedNotSupportedValue = func() (*w32.Unknown, uintptr) { return notSupported, uintptr(w32.COM_S_OK) }
	getReservedMixedAttributeValue = func() (*w32.Unknown, uintptr) { return mixed, uintptr(w32.COM_S_OK) }

	w, r := textRangeWindow(t, 0, 12)
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 6, uintptr(CultureAttributeId), out.fresh()))
	c.Equal(VT_UNKNOWN, out.variant().VT)
	c.Equal(uintptr(unsafe.Pointer(notSupported)), uintptr(out.variant().Val),
		"an attribute no document records is refused with the reserved value rather than guessed at")

	c.Equal(w32.COM_S_OK, callRangeSlot(r, 6, uintptr(FontNameAttributeId), out.fresh()))
	c.Equal(VT_UNKNOWN, out.variant().VT)
	c.Equal(uintptr(unsafe.Pointer(mixed)), uintptr(out.variant().Val),
		"a range covering two fonts does not agree about the font")

	// A system that cannot produce the singleton is answered with E_NOTSUPPORTED, which is the truth about the
	// attribute either way.
	getReservedNotSupportedValue = func() (*w32.Unknown, uintptr) { return nil, uintptr(w32.COM_E_NOTIMPL) }
	c.Equal(E_NOTSUPPORTED, callRangeSlot(r, 6, uintptr(CultureAttributeId), out.fresh()))

	// The Link attribute is the one whose value is a range, and text that is not part of a link has an empty answer
	// rather than a refusal.
	link := newTextRange(w.Window, textDocumentID, 12, 16)
	defer link.release()
	c.Equal(w32.COM_S_OK, callRangeSlot(link, 6, uintptr(LinkAttributeId), out.fresh()))
	c.Equal(VT_UNKNOWN, out.variant().VT)
	linkRange := rangeFromOut(c, uintptr(out.variant().Val))
	c.NotNil(linkRange)
	start, end := linkRange.offsets()
	c.Equal(12, start)
	c.Equal(16, end)

	prose := newTextRange(w.Window, textDocumentID, 6, 12)
	defer prose.release()
	c.Equal(w32.COM_S_OK, callRangeSlot(prose, 6, uintptr(LinkAttributeId), out.fresh()))
	c.Equal(VT_EMPTY, out.variant().VT)
}

// TestTextRangeSurvivesPublish verifies what a range does when the document under it changes, which is the whole
// reason it holds a pair of offsets rather than a piece of the snapshot: a client keeps a range for as long as it
// likes, and a publish in between may have rewritten the document, shortened it, or taken it out of the window
// altogether.
func TestTextRangeSurvivesPublish(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w, r := textRangeWindow(t, 23, 28)
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 9, uintptr(^uintptr(0)), out.fresh()))
	c.Equal("x = 1", BSTRToString(BSTR(out.ptr())))
	BSTR(out.ptr()).Free()

	// A shorter document clamps the range rather than reading past the end of it.
	shorter := textFixtureTree()
	shorter.Generation = 2
	shorter.Node(textDocumentID).Document = &accessibility.DocumentInfo{
		Text: accessibility.TextInfo{Text: "Title"},
	}
	w.Publish(shorter, nil)
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 9, uintptr(^uintptr(0)), out.fresh()))
	c.Equal("", BSTRToString(BSTR(out.ptr())), "both ends clamp to the end of what is there now")
	BSTR(out.ptr()).Free()
	start, end := r.offsets()
	c.Equal(23, start, "the range itself is not rewritten: the document may grow back")
	c.Equal(28, end)

	// A document that has lost its stream can no longer be read at all, and neither can one that has left the tree.
	plain := textFixtureTree()
	plain.Generation = 3
	plain.Node(textDocumentID).Document = nil
	w.Publish(plain, nil)
	c.Equal(E_ELEMENTNOTAVAILABLE, callRangeSlot(r, 9, uintptr(^uintptr(0)), out.fresh()))

	gone := textFixtureTree()
	gone.Generation = 4
	gone.Node(textScrollID).Children = nil
	delete(gone.Nodes, textDocumentID)
	w.Publish(gone, nil)
	c.Equal(E_ELEMENTNOTAVAILABLE, callRangeSlot(r, 9, uintptr(^uintptr(0)), out.fresh()))
	c.Equal(E_ELEMENTNOTAVAILABLE, callRangeSlot(r, 0, out.fresh()), "nor cloned")
	c.Equal(E_ELEMENTNOTAVAILABLE, callRangeSlot(r, 13), "nor selected")
	c.Equal(E_ELEMENTNOTAVAILABLE, callRangeSlot(r, 14),
		"and the two selection methods report the document as gone rather than as an invalid operation")
}

// TestTextRangeCompareAcrossDocuments verifies what a range does with another range a client hands it: a pointer
// that is no range of this process at all is an invalid argument rather than something to dereference, and two ranges
// over different documents have no common text, so there is no order between their endpoints to report.
func TestTextRangeCompareAcrossDocuments(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w, r := textRangeWindow(t, 0, 5)

	// Another window, with a document of its own.
	other := newTestWindow(t, textFixtureTree())
	otherRange := newTextRange(other.Window, textDocumentID, 0, 5)
	defer otherRange.release()
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 1, otherRange.this(), out.fresh()))
	c.Equal(int32(0), out.i32(), "the same offsets in another window are not the same text")
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 2, uintptr(TextPatternRangeEndpoint_Start), otherRange.this(),
		uintptr(TextPatternRangeEndpoint_Start), out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 12, uintptr(TextPatternRangeEndpoint_End), otherRange.this(),
		uintptr(TextPatternRangeEndpoint_End)))

	// A pointer that is no range of ours is refused without being followed, and so is a NULL one.
	stranger := &Rect{}
	pin.Pin(stranger)
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 1, uintptr(unsafe.Pointer(stranger)), out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 2, uintptr(TextPatternRangeEndpoint_Start),
		uintptr(unsafe.Pointer(stranger)), uintptr(TextPatternRangeEndpoint_Start), out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 1, 0, out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 12, uintptr(TextPatternRangeEndpoint_End), 0,
		uintptr(TextPatternRangeEndpoint_End)))

	// A range whose document can no longer be read is not equal to one that still can, which is FALSE rather than an
	// error: the two plainly do not stand for the same text. This range's own document going is a different matter,
	// since a range that cannot say what it stands for cannot answer at all.
	lost := textFixtureTree()
	lost.Generation = 2
	lost.Node(textDocumentID).Document = nil
	other.Publish(lost, nil)
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 1, otherRange.this(), out.fresh()))
	c.Equal(int32(0), out.i32(), "a range whose document has gone equals nothing")
	c.Equal(E_ELEMENTNOTAVAILABLE, callRangeSlot(otherRange, 1, r.this(), out.fresh()))

	// A range within the same document compares as it should, which is what says the refusals above are about the
	// document rather than about comparing at all.
	sameDocument := newTextRange(w.Window, textDocumentID, 6, 12)
	defer sameDocument.release()
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 2, uintptr(TextPatternRangeEndpoint_Start), sameDocument.this(),
		uintptr(TextPatternRangeEndpoint_Start), out.fresh()))
	c.Equal(int32(-6), out.i32())
}

// TestTextRangeRefusesValuesOutsideTheEnumerations verifies that a text unit or an endpoint a client passed that is
// not one UI Automation defines is refused with E_INVALIDARG rather than guessed at. A provider that read an unknown
// endpoint as the start would move or compare the wrong end of the range and report success, which a client has no way
// to notice.
func TestTextRangeRefusesValuesOutsideTheEnumerations(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w, r := textRangeWindow(t, 6, 17)
	other := newTextRange(w.Window, textDocumentID, 0, 5)
	defer other.release()
	badUnit, badEndpoint, negative := uintptr(99), uintptr(2), uintptr(^uintptr(0))

	// The unit, in each of the three methods that take one.
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 3, badUnit), "ExpandToEnclosingUnit")
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 10, badUnit, 1, out.fresh()), "Move")
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 11, uintptr(TextPatternRangeEndpoint_End), badUnit, 1, out.fresh()),
		"MoveEndpointByUnit")

	// The endpoint, in each of the three methods that take one — both of them in the two that take another range's as
	// well.
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 11, badEndpoint, uintptr(TextUnit_Word), 1, out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 11, negative, uintptr(TextUnit_Word), 1, out.fresh()),
		"a negative endpoint is no more one end of a range than too large a one is")
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 2, badEndpoint, other.this(),
		uintptr(TextPatternRangeEndpoint_Start), out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 2, uintptr(TextPatternRangeEndpoint_Start), other.this(), badEndpoint,
		out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 12, badEndpoint, other.this(),
		uintptr(TextPatternRangeEndpoint_End)))
	c.Equal(w32.COM_E_INVALIDARG, callRangeSlot(r, 12, uintptr(TextPatternRangeEndpoint_End), other.this(), badEndpoint))

	// Every one of those refusals left the range exactly where it was.
	start, end := r.offsets()
	c.Equal(6, start)
	c.Equal(17, end)

	// The same calls with values the enumerations do hold are answered, which is what says the refusals are about the
	// values rather than about the methods.
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 11, uintptr(TextPatternRangeEndpoint_End), uintptr(TextUnit_Word),
		uintptr(^uintptr(0)), out.fresh()))
	c.Equal(int32(-1), out.i32())
	c.Equal(w32.COM_S_OK, callRangeSlot(r, 2, uintptr(TextPatternRangeEndpoint_Start), other.this(),
		uintptr(TextPatternRangeEndpoint_Start), out.fresh()))
	c.Equal(int32(6), out.i32())
}

// TestTextRangeNullOutParameters verifies that every method a client can pass a NULL out-parameter to says so rather
// than writing through it. UI Automation never does, but a scripting client reaching a provider through a wrapper can.
func TestTextRangeNullOutParameters(t *testing.T) {
	c := check.New(t)
	_, r := textRangeWindow(t, 0, 5)
	for _, one := range []struct {
		name   string
		method int
		args   []uintptr
	}{
		{name: "Clone", method: 0, args: []uintptr{0}},
		{name: "Compare", method: 1, args: []uintptr{r.this(), 0}},
		{name: "CompareEndpoints", method: 2, args: []uintptr{0, r.this(), 0, 0}},
		{name: "FindAttribute", method: 4, args: []uintptr{uintptr(StyleIdAttributeId), 0, 0, 0}},
		{name: "FindText", method: 5, args: []uintptr{0, 0, 0, 0}},
		{name: "GetAttributeValue", method: 6, args: []uintptr{uintptr(StyleIdAttributeId), 0}},
		{name: "GetBoundingRectangles", method: 7, args: []uintptr{0}},
		{name: "GetEnclosingElement", method: 8, args: []uintptr{0}},
		{name: "GetText", method: 9, args: []uintptr{0, 0}},
		{name: "Move", method: 10, args: []uintptr{uintptr(TextUnit_Word), 1, 0}},
		{name: "MoveEndpointByUnit", method: 11, args: []uintptr{0, uintptr(TextUnit_Word), 1, 0}},
		{name: "GetChildren", method: 17, args: []uintptr{0}},
	} {
		c.Equal(w32.COM_E_POINTER, callRangeSlot(r, one.method, one.args...), one.name)
	}
	start, end := r.offsets()
	c.Equal(0, start, "and nothing moved the range while refusing")
	c.Equal(5, end)
}
