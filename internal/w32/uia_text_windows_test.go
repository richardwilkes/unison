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
	"runtime"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"golang.org/x/sys/windows"
)

// These tests cover the two interfaces that let a client read a document as text: ITextProvider2 on the document and
// ITextChildProvider on everything inside it. What the answers are supposed to be is settled by the portable tests in
// uia_text_test.go; these check the COM around them.

// TestUIATextIIDs pins the interface identifiers a client asks for the text interfaces by. Nothing else does: a client
// resolves none of them by name, so a transposed digit would have every QueryInterface refused and the document would
// simply never be read, with nothing to say why.
func TestUIATextIIDs(t *testing.T) {
	c := check.New(t)
	c.Equal(xos.Must(windows.GUIDFromString("{0dc5e6ed-3e16-4bf1-8f9a-a979878bc195}")), uiaIfaceIIDs[uiaIfaceText],
		"ITextProvider2")
	c.Equal(xos.Must(windows.GUIDFromString("{4c2de2b9-c88f-4f88-a111-f1d336b7d1a9}")), uiaIfaceIIDs[uiaIfaceTextChild],
		"ITextChildProvider")
	c.Equal(xos.Must(windows.GUIDFromString("{5347ad7b-c355-46f8-aff5-909033582f63}")), iidITextRangeProvider)
	c.Equal(xos.Must(windows.GUIDFromString("{9bbce42c-1921-4f18-89ca-dba1910a0386}")), iidITextRangeProvider2)
	c.Equal(1, len(uiaIfaceIIDAliases))
	c.Equal(xos.Must(windows.GUIDFromString("{3589c92c-63f3-4367-99bb-ada653b77cf2}")), uiaIfaceIIDAliases[0].guid,
		"ITextProvider, which is answered with ITextProvider2's table")
	c.Equal(uiaIfaceText, uiaIfaceIIDAliases[0].iface)
}

// TestUIAQueryInterfaceTextAliases verifies that a document answers both of the Text pattern's interface identifiers
// with the one table it has — ITextProvider2 derives from ITextProvider, so its first six slots are that interface's —
// and that an element inside the document answers the TextChild identifier while the document itself does not.
func TestUIAQueryInterfaceTextAliases(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, uiaTextFixtureTree())
	document := w.providerFor(uiaTextDocumentID)
	c.NotNil(document)
	link := w.providerFor(uiaTextLinkID)
	c.NotNil(link)

	query := func(p *UIAProvider, guid windows.GUID) uint64 {
		wanted := guid
		pin.Pin(&wanted)
		*out = 0
		return uiaQueryInterface(uiaIfaceSimple, p.ifacePtr(uiaIfaceSimple), uintptr(unsafe.Pointer(&wanted)),
			outAddress)
	}

	for _, guid := range []windows.GUID{uiaIfaceIIDs[uiaIfaceText], uiaIfaceIIDAliases[0].guid} {
		c.Equal(COM_S_OK, query(document, guid))
		c.Equal(document.ifacePtr(uiaIfaceText), *out, "both text identifiers answer with the one table")
		c.Equal(uintptr(1), document.release())
	}

	// The document is the text container rather than a child of one, and an element inside it is the other way around.
	c.Equal(COM_E_NOINTERFACE, query(document, uiaIfaceIIDs[uiaIfaceTextChild]))
	c.Equal(COM_S_OK, query(link, uiaIfaceIIDs[uiaIfaceTextChild]))
	c.Equal(link.ifacePtr(uiaIfaceTextChild), *out)
	c.Equal(uintptr(1), link.release())
	c.Equal(COM_E_NOINTERFACE, query(link, uiaIfaceIIDs[uiaIfaceText]))
	c.Equal(COM_E_NOINTERFACE, query(link, uiaIfaceIIDAliases[0].guid))

	// Both patterns are reachable by identifier as well, and the two ways of asking agree.
	for _, id := range []PatternID{UIA_TextPatternId, UIA_TextPattern2Id} {
		c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(document.ifacePtr(uiaIfaceSimple), uintptr(id), outAddress))
		c.Equal(document.ifacePtr(uiaIfaceText), *out, "pattern %d", id)
		c.Equal(uintptr(1), document.release())
	}
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(link.ifacePtr(uiaIfaceSimple), uintptr(UIA_TextChildPatternId),
		outAddress))
	c.Equal(link.ifacePtr(uiaIfaceTextChild), *out)
	c.Equal(uintptr(1), link.release())

	// A Document with no stream hands out neither pattern, which is what pairs with its reporting the group control
	// type.
	plainTree := uiaTextFixtureTree()
	plainTree.Node(uiaTextDocumentID).Document = nil
	plain := newTestUIAWindow(t, plainTree).providerFor(uiaTextDocumentID)
	c.NotNil(plain)
	c.Equal(COM_E_NOINTERFACE, query(plain, uiaIfaceIIDs[uiaIfaceText]))
	c.Equal(COM_S_OK, uiaSimpleGetPatternProvider(plain.ifacePtr(uiaIfaceSimple), uintptr(UIA_TextPatternId),
		outAddress))
	c.Equal(uintptr(0), *out, "an unsupported pattern is a NULL interface with S_OK rather than a failure")
}

// TestUIATextVtblSlotOrder verifies that method N of ITextProvider2 and of ITextChildProvider really sits in slot N of
// its interface's virtual method table, by calling each one through the table the way UI Automation does. Every other
// test here calls the Go functions the slots were built from, which answer the same however the slots are ordered.
//
// RangeFromPoint is left out: it takes a structure by value, which arrives differently on each architecture and which
// syscall.SyscallN cannot describe on either. TestUIATextRangeFromPoint calls it the way UI Automation does.
func TestUIATextVtblSlotOrder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := uiaSlotScratch(&pin)
	w := newTestUIAWindow(t, uiaTextFixtureTree())
	document := w.providerFor(uiaTextDocumentID)
	c.NotNil(document)

	// 0: GetSelection, which is the caret when nothing is selected and is where NVDA reads it from. The fixture has the
	// link selected.
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 0, out.fresh()))
	ranges := uiaSafeArrayRanges(c, out.array())
	c.Equal(1, len(ranges), "GetSelection")
	if len(ranges) == 1 {
		start, end := ranges[0].offsets()
		c.Equal(12, start)
		c.Equal(16, end)
	}

	// 1: GetVisibleRanges, which is what the scroll area really shows.
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 1, out.fresh()))
	ranges = uiaSafeArrayRanges(c, out.array())
	c.Equal(1, len(ranges), "GetVisibleRanges")
	if len(ranges) == 1 {
		start, end := ranges[0].offsets()
		c.Equal(0, start)
		c.Equal(29, end)
	}

	// 2: RangeFromChild, asked about the image, which occupies the one object-replacement character.
	image := w.providerFor(uiaTextImageID)
	c.NotNil(image)
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 2, image.ifacePtr(uiaIfaceSimple), out.fresh()))
	child := uiaRangeFromOut(c, out.ptr())
	c.NotNil(child)
	if child != nil {
		start, end := child.offsets()
		c.Equal(17, start, "RangeFromChild")
		c.Equal(18, end)
	}

	// 4: get_DocumentRange, the range every other one is reached from.
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 4, out.fresh()))
	whole := uiaRangeFromOut(c, out.ptr())
	c.NotNil(whole)
	if whole != nil {
		start, end := whole.offsets()
		c.Equal(0, start, "get_DocumentRange")
		c.Equal(uiaTextFixtureLength, end)
	}

	// 5: get_SupportedTextSelection.
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 5, out.fresh()))
	c.Equal(int32(SupportedTextSelection_Single), out.i32(), "get_SupportedTextSelection")

	// 6: RangeFromAnnotation, which nothing here can place: a NULL range with S_OK is the documented answer.
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 6, image.ifacePtr(uiaIfaceSimple), out.fresh()))
	c.Equal(uintptr(0), out.ptr(), "RangeFromAnnotation")

	// 7: GetCaretRange, which needs two out-parameters, so the second is allocated separately.
	caretOut, caretAddress := uiaOut[uintptr](&pin)
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 7, out.fresh(), caretAddress))
	c.Equal(int32(1), out.i32(), "GetCaretRange reports the caret as active while the document has the focus")
	caret := uiaRangeFromOut(c, *caretOut)
	c.NotNil(caret)
	if caret != nil {
		start, end := caret.offsets()
		c.Equal(12, start)
		c.Equal(12, end, "the caret is a degenerate range")
	}

	// ITextChildProvider: get_TextContainer, then get_TextRange.
	link := w.providerFor(uiaTextLinkID)
	c.NotNil(link)
	c.Equal(COM_S_OK, uiaCallSlot(link, uiaIfaceTextChild, 0, out.fresh()))
	c.Equal(document.ifacePtr(uiaIfaceSimple), out.ptr(), "get_TextContainer")
	uiaProviderFromThis(out.ptr(), uiaIfaceSimple).release()
	c.Equal(COM_S_OK, uiaCallSlot(link, uiaIfaceTextChild, 1, out.fresh()))
	span := uiaRangeFromOut(c, out.ptr())
	c.NotNil(span)
	if span != nil {
		start, end := span.offsets()
		c.Equal(12, start, "get_TextRange")
		c.Equal(16, end)
	}
}

// TestUIATextProviderAnswers covers what the Text pattern's methods answer in the cases the slot-order test does not: a
// document that allows no selection, a client naming an element that is not in the stream, and an element that has lost
// the pattern since the interface was handed out.
func TestUIATextProviderAnswers(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := uiaSlotScratch(&pin)

	// A document that does not offer the selection action has no caret to place, so it reports no selection at all
	// rather than one at offset zero the user cannot move.
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextDocumentID).Actions = accessibility.ActionSet(0).With(accessibility.ScrollRangeIntoView)
	w := newTestUIAWindow(t, tree)
	document := w.providerFor(uiaTextDocumentID)
	c.NotNil(document)
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 5, out.fresh()))
	c.Equal(int32(SupportedTextSelection_None), out.i32())
	c.Equal(COM_S_OK, uiaCallSlot(document, uiaIfaceText, 0, out.fresh()))
	c.Equal(0, len(uiaSafeArrayRanges(c, out.array())), "an empty array rather than a NULL one")

	// An element outside the stream, and one from another window, are invalid arguments rather than faults — and a
	// pointer that is no provider of ours at all is never followed.
	full := newTestUIAWindow(t, uiaTextFixtureTree())
	fullDocument := full.providerFor(uiaTextDocumentID)
	c.NotNil(fullDocument)
	outside := full.providerFor(uiaTextButtonID)
	c.NotNil(outside)
	c.Equal(COM_E_INVALIDARG, uiaCallSlot(fullDocument, uiaIfaceText, 2, outside.ifacePtr(uiaIfaceSimple), out.fresh()))
	c.Equal(COM_E_INVALIDARG, uiaCallSlot(fullDocument, uiaIfaceText, 2,
		document.ifacePtr(uiaIfaceSimple), out.fresh()), "an element of another window is not this document's child")
	c.Equal(COM_E_INVALIDARG, uiaCallSlot(fullDocument, uiaIfaceText, 2, 0, out.fresh()))
	stranger := &UiaRect{}
	pin.Pin(stranger)
	c.Equal(COM_E_INVALIDARG, uiaCallSlot(fullDocument, uiaIfaceText, 2, uintptr(unsafe.Pointer(stranger)),
		out.fresh()))
	c.Equal(COM_E_INVALIDARG, uiaCallSlot(fullDocument, uiaIfaceText, 2,
		fullDocument.ifacePtr(uiaIfaceSimple), out.fresh()), "and the document is not a child of itself")

	// A document that loses its stream between the interface being handed out and a call arriving says so, which is
	// what UIA_E_NOTSUPPORTED is for: the element is still there, and no longer supports the pattern.
	plain := uiaTextFixtureTree()
	plain.Generation = 2
	plain.Node(uiaTextDocumentID).Document = nil
	full.Publish(plain, nil)
	flag, flagAddress := uiaOut[int32](&pin)
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 0, out.fresh()), "GetSelection")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 1, out.fresh()), "GetVisibleRanges")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 2, outside.ifacePtr(uiaIfaceSimple),
		out.fresh()), "RangeFromChild")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 4, out.fresh()), "get_DocumentRange")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 5, out.fresh()),
		"get_SupportedTextSelection")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 6, 0, out.fresh()), "RangeFromAnnotation")
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(fullDocument, uiaIfaceText, 7, flagAddress, out.fresh()),
		"GetCaretRange")
	c.Equal(int32(0), *flag, "and the caret is reported as inactive rather than left untouched")

	// Every out-parameter is checked rather than written through.
	other := newTestUIAWindow(t, uiaTextFixtureTree())
	otherDocument := other.providerFor(uiaTextDocumentID)
	c.NotNil(otherDocument)
	for _, method := range []int{0, 1, 4, 5} {
		c.Equal(COM_E_POINTER, uiaCallSlot(otherDocument, uiaIfaceText, method, 0), "method %d", method)
	}
	c.Equal(COM_E_POINTER, uiaCallSlot(otherDocument, uiaIfaceText, 2, 0, 0))
	c.Equal(COM_E_POINTER, uiaCallSlot(otherDocument, uiaIfaceText, 6, 0, 0))
	c.Equal(COM_E_POINTER, uiaCallSlot(otherDocument, uiaIfaceText, 7, 0, 0))
	c.Equal(COM_E_POINTER, uiaCallSlot(otherDocument, uiaIfaceText, 7, out.fresh(), 0))
}

// TestUIATextChildProvider covers ITextChildProvider beyond the slot order: an element with no stretch of its own is
// answered with its ancestor's, and an element outside the document has no text pattern to be a child of.
func TestUIATextChildProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := uiaSlotScratch(&pin)
	w := newTestUIAWindow(t, uiaTextFixtureTree())
	document := w.providerFor(uiaTextDocumentID)
	c.NotNil(document)

	// The label inside the paragraph has no stretch of the stream of its own, so it is answered with the paragraph's,
	// which is the smallest stretch that is certainly its.
	label := w.providerFor(uiaTextLabelID)
	c.NotNil(label)
	c.Equal(COM_S_OK, uiaCallSlot(label, uiaIfaceTextChild, 1, out.fresh()))
	span := uiaRangeFromOut(c, out.ptr())
	c.NotNil(span)
	if span != nil {
		start, end := span.offsets()
		c.Equal(6, start)
		c.Equal(22, end)
	}

	// A virtual table row is answered like anything else, which is what lets a client reading a table by row find its
	// text.
	row := w.providerFor(uiaTextRowID)
	c.NotNil(row)
	c.Equal(COM_S_OK, uiaCallSlot(row, uiaIfaceTextChild, 0, out.fresh()))
	c.Equal(document.ifacePtr(uiaIfaceSimple), out.ptr())
	uiaProviderFromThis(out.ptr(), uiaIfaceSimple).release()

	// An element outside the document does not support the pattern, which is what it is told when it is asked anyway.
	outside := w.providerFor(uiaTextButtonID)
	c.NotNil(outside)
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(outside, uiaIfaceTextChild, 0, out.fresh()))
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(outside, uiaIfaceTextChild, 1, out.fresh()))

	// And so does everything inside a document that has lost its stream.
	plain := uiaTextFixtureTree()
	plain.Generation = 2
	plain.Node(uiaTextDocumentID).Document = nil
	w.Publish(plain, nil)
	c.Equal(UIA_E_NOTSUPPORTED, uiaCallSlot(label, uiaIfaceTextChild, 1, out.fresh()))
	c.Equal(COM_E_POINTER, uiaCallSlot(label, uiaIfaceTextChild, 1, 0))
}

// TestUIATextRangeFromPoint verifies the one provider method that takes a structure by value. It is called the way UI
// Automation calls it — through the vtable slot on amd64, and through the shim that fills the floating-point registers
// on arm64 — so that the architecture's own argument passing is what is exercised; see uiaTextRangeFromPointCall.
//
// The point is in screen pixels, which the window's geometry converts: an origin of (100,50) and a scale of two, so the
// document's own top left corner is at (120,90) and one character is twenty screen pixels wide — the fixture's advances
// are ten logical units per rune, and the scale doubles them.
func TestUIATextRangeFromPoint(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := uiaOut[uintptr](&pin)
	w := newTestUIAWindow(t, uiaTextFixtureTree())
	document := w.providerFor(uiaTextDocumentID)
	c.NotNil(document)

	*out = 0
	c.Equal(COM_S_OK, uiaTextRangeFromPointCall(t, document.ifacePtr(uiaIfaceText), 120+50, 90+5, outAddress))
	r := uiaRangeFromOut(c, *out)
	c.NotNil(r)
	if r != nil {
		start, end := r.offsets()
		c.Equal(2, start, "the third character of the first line, which is fifty screen pixels along it")
		c.Equal(2, end, "and a degenerate range, which is a caret position")
	}

	// A point below everything answers with the last line rather than failing: RangeFromPoint is documented to report
	// the nearest range.
	*out = 0
	c.Equal(COM_S_OK, uiaTextRangeFromPointCall(t, document.ifacePtr(uiaIfaceText), 125, 90+1000, outAddress))
	r = uiaRangeFromOut(c, *out)
	c.NotNil(r)
	if r != nil {
		start, _ := r.offsets()
		c.Equal(35, start)
	}

	// A NULL out-parameter is refused rather than written through.
	c.Equal(COM_E_POINTER, uiaTextRangeFromPointCall(t, document.ifacePtr(uiaIfaceText), 120, 90, 0))
}

// TestUIATextBlockNames verifies that the blocks a document is made of answer the Name property with their own content,
// which is what Narrator's item navigation speaks as it steps onto each of them. A paragraph the widget gave no name
// would otherwise be announced as a bare "text".
func TestUIATextBlockNames(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := uiaOut[VARIANT](&pin)
	w := newTestUIAWindow(t, uiaTextFixtureTree())

	name := func(id accessibility.NodeID) string {
		p := w.providerFor(id)
		c.NotNil(p)
		*value = VARIANT{}
		c.Equal(COM_S_OK, uiaSimpleGetPropertyValue(p.ifacePtr(uiaIfaceSimple), uintptr(UIA_NamePropertyId),
			valueAddress))
		defer value.Clear()
		return uiaVariantString(value)
	}
	c.Equal("Hello link ￼ end", name(uiaTextParagraphID), "a paragraph is named by what it says")
	c.Equal("x = 1", name(uiaTextCodeID))
	c.Equal("A", name(uiaTextCellAID))
	c.Equal("Title", name(uiaTextHeadingID), "a heading keeps the name folded from its fragments")
	c.Equal("link", name(uiaTextLinkID))
	c.Equal("Notes", name(uiaTextDocumentID), "and a document keeps its own")
}
