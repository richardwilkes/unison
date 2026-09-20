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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// These tests cover the two interfaces that let a client read a document as text: ITextProvider2 on the document and
// ITextChildProvider on everything inside it. What the answers are supposed to be is settled by the portable tests in
// text_test.go; these check the COM around them.

// TestTextIIDs pins the interface identifiers a client asks for the text interfaces by. Nothing else does: a client
// resolves none of them by name, so a transposed digit would have every QueryInterface refused and the document would
// simply never be read, with nothing to say why.
func TestTextIIDs(t *testing.T) {
	c := check.New(t)
	c.Equal(xos.Must(windows.GUIDFromString("{0dc5e6ed-3e16-4bf1-8f9a-a979878bc195}")), ifaceIIDs[ifaceText],
		"ITextProvider2")
	c.Equal(xos.Must(windows.GUIDFromString("{4c2de2b9-c88f-4f88-a111-f1d336b7d1a9}")), ifaceIIDs[ifaceTextChild],
		"ITextChildProvider")
	c.Equal(xos.Must(windows.GUIDFromString("{5347ad7b-c355-46f8-aff5-909033582f63}")), iidITextRangeProvider)
	c.Equal(xos.Must(windows.GUIDFromString("{9bbce42c-1921-4f18-89ca-dba1910a0386}")), iidITextRangeProvider2)
	c.Equal(1, len(ifaceIIDAliases))
	c.Equal(xos.Must(windows.GUIDFromString("{3589c92c-63f3-4367-99bb-ada653b77cf2}")), ifaceIIDAliases[0].guid,
		"ITextProvider, which is answered with ITextProvider2's table")
	c.Equal(ifaceText, ifaceIIDAliases[0].iface)
}

// TestQueryInterfaceTextAliases verifies that a document answers both of the Text pattern's interface identifiers
// with the one table it has — ITextProvider2 derives from ITextProvider, so its first six slots are that interface's —
// and that an element inside the document answers the TextChild identifier while the document itself does not.
func TestQueryInterfaceTextAliases(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, textFixtureTree())
	document := w.providerFor(textDocumentID)
	c.NotNil(document)
	link := w.providerFor(textLinkID)
	c.NotNil(link)

	query := func(p *Provider, guid windows.GUID) uint64 {
		wanted := guid
		pin.Pin(&wanted)
		*out = 0
		return queryInterface(ifaceSimple, p.ifacePtr(ifaceSimple), uintptr(unsafe.Pointer(&wanted)),
			outAddress)
	}

	for _, guid := range []windows.GUID{ifaceIIDs[ifaceText], ifaceIIDAliases[0].guid} {
		c.Equal(w32.COM_S_OK, query(document, guid))
		c.Equal(document.ifacePtr(ifaceText), *out, "both text identifiers answer with the one table")
		c.Equal(uintptr(1), document.release())
	}

	// The document is the text container rather than a child of one, and an element inside it is the other way around.
	c.Equal(w32.COM_E_NOINTERFACE, query(document, ifaceIIDs[ifaceTextChild]))
	c.Equal(w32.COM_S_OK, query(link, ifaceIIDs[ifaceTextChild]))
	c.Equal(link.ifacePtr(ifaceTextChild), *out)
	c.Equal(uintptr(1), link.release())
	c.Equal(w32.COM_E_NOINTERFACE, query(link, ifaceIIDs[ifaceText]))
	c.Equal(w32.COM_E_NOINTERFACE, query(link, ifaceIIDAliases[0].guid))

	// Both patterns are reachable by identifier as well, and the two ways of asking agree.
	for _, id := range []PatternID{TextPatternId, TextPattern2Id} {
		c.Equal(w32.COM_S_OK, simpleGetPatternProvider(document.ifacePtr(ifaceSimple), uintptr(id), outAddress))
		c.Equal(document.ifacePtr(ifaceText), *out, "pattern %d", id)
		c.Equal(uintptr(1), document.release())
	}
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(link.ifacePtr(ifaceSimple), uintptr(TextChildPatternId),
		outAddress))
	c.Equal(link.ifacePtr(ifaceTextChild), *out)
	c.Equal(uintptr(1), link.release())

	// A Document with no stream hands out neither pattern, which is what pairs with its reporting the group control
	// type.
	plainTree := textFixtureTree()
	plainTree.Node(textDocumentID).Document = nil
	plain := newTestWindow(t, plainTree).providerFor(textDocumentID)
	c.NotNil(plain)
	c.Equal(w32.COM_E_NOINTERFACE, query(plain, ifaceIIDs[ifaceText]))
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(plain.ifacePtr(ifaceSimple), uintptr(TextPatternId),
		outAddress))
	c.Equal(uintptr(0), *out, "an unsupported pattern is a NULL interface with S_OK rather than a failure")
}

// TestTextVtblSlotOrder verifies that method N of ITextProvider2 and of ITextChildProvider really sits in slot N of
// its interface's virtual method table, by calling each one through the table the way UI Automation does. Every other
// test here calls the Go functions the slots were built from, which answer the same however the slots are ordered.
//
// RangeFromPoint is left out: it takes a structure by value, which arrives differently on each architecture and which
// syscall.SyscallN cannot describe on either. TestTextRangeFromPoint calls it the way UI Automation does.
func TestTextVtblSlotOrder(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w := newTestWindow(t, textFixtureTree())
	document := w.providerFor(textDocumentID)
	c.NotNil(document)

	// 0: GetSelection, which is the caret when nothing is selected and is where NVDA reads it from. The fixture has the
	// link selected.
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 0, out.fresh()))
	ranges := safeArrayRanges(c, out.array())
	c.Equal(1, len(ranges), "GetSelection")
	if len(ranges) == 1 {
		start, end := ranges[0].offsets()
		c.Equal(12, start)
		c.Equal(16, end)
	}

	// 1: GetVisibleRanges, which is what the scroll area really shows.
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 1, out.fresh()))
	ranges = safeArrayRanges(c, out.array())
	c.Equal(1, len(ranges), "GetVisibleRanges")
	if len(ranges) == 1 {
		start, end := ranges[0].offsets()
		c.Equal(0, start)
		c.Equal(29, end)
	}

	// 2: RangeFromChild, asked about the image, which occupies the one object-replacement character.
	image := w.providerFor(textImageID)
	c.NotNil(image)
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 2, image.ifacePtr(ifaceSimple), out.fresh()))
	child := rangeFromOut(c, out.ptr())
	c.NotNil(child)
	if child != nil {
		start, end := child.offsets()
		c.Equal(17, start, "RangeFromChild")
		c.Equal(18, end)
	}

	// 4: get_DocumentRange, the range every other one is reached from.
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 4, out.fresh()))
	whole := rangeFromOut(c, out.ptr())
	c.NotNil(whole)
	if whole != nil {
		start, end := whole.offsets()
		c.Equal(0, start, "get_DocumentRange")
		c.Equal(textFixtureLength, end)
	}

	// 5: get_SupportedTextSelection.
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 5, out.fresh()))
	c.Equal(int32(SupportedTextSelection_Single), out.i32(), "get_SupportedTextSelection")

	// 6: RangeFromAnnotation, which nothing here can place: a NULL range with S_OK is the documented answer.
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 6, image.ifacePtr(ifaceSimple), out.fresh()))
	c.Equal(uintptr(0), out.ptr(), "RangeFromAnnotation")

	// 7: GetCaretRange, which needs two out-parameters, so the second is allocated separately.
	caretOut, caretAddress := pinnedOut[uintptr](&pin)
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 7, out.fresh(), caretAddress))
	c.Equal(int32(1), out.i32(), "GetCaretRange reports the caret as active while the document has the focus")
	caret := rangeFromOut(c, *caretOut)
	c.NotNil(caret)
	if caret != nil {
		start, end := caret.offsets()
		c.Equal(12, start)
		c.Equal(12, end, "the caret is a degenerate range")
	}

	// ITextChildProvider: get_TextContainer, then get_TextRange.
	link := w.providerFor(textLinkID)
	c.NotNil(link)
	c.Equal(w32.COM_S_OK, callSlot(link, ifaceTextChild, 0, out.fresh()))
	c.Equal(document.ifacePtr(ifaceSimple), out.ptr(), "get_TextContainer")
	providerFromThis(out.ptr(), ifaceSimple).release()
	c.Equal(w32.COM_S_OK, callSlot(link, ifaceTextChild, 1, out.fresh()))
	rng := rangeFromOut(c, out.ptr())
	c.NotNil(rng)
	if rng != nil {
		start, end := rng.offsets()
		c.Equal(12, start, "get_TextRange")
		c.Equal(16, end)
	}
}

// TestTextProviderAnswers covers what the Text pattern's methods answer in the cases the slot-order test does not: a
// document that allows no selection, a client naming an element that is not in the stream, and an element that has lost
// the pattern since the interface was handed out.
func TestTextProviderAnswers(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)

	// A document that does not offer the selection action has no caret to place, so it reports no selection at all
	// rather than one at offset zero the user cannot move.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Actions = accessibility.ActionSet(0).With(accessibility.ScrollRangeIntoView)
	w := newTestWindow(t, tree)
	document := w.providerFor(textDocumentID)
	c.NotNil(document)
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 5, out.fresh()))
	c.Equal(int32(SupportedTextSelection_None), out.i32())
	c.Equal(w32.COM_S_OK, callSlot(document, ifaceText, 0, out.fresh()))
	c.Equal(0, len(safeArrayRanges(c, out.array())), "an empty array rather than a NULL one")

	// An element outside the stream, and one from another window, are invalid arguments rather than faults — and a
	// pointer that is no provider of ours at all is never followed.
	full := newTestWindow(t, textFixtureTree())
	fullDocument := full.providerFor(textDocumentID)
	c.NotNil(fullDocument)
	outside := full.providerFor(textButtonID)
	c.NotNil(outside)
	c.Equal(w32.COM_E_INVALIDARG, callSlot(fullDocument, ifaceText, 2, outside.ifacePtr(ifaceSimple), out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callSlot(fullDocument, ifaceText, 2,
		document.ifacePtr(ifaceSimple), out.fresh()), "an element of another window is not this document's child")
	c.Equal(w32.COM_E_INVALIDARG, callSlot(fullDocument, ifaceText, 2, 0, out.fresh()))
	stranger := &Rect{}
	pin.Pin(stranger)
	c.Equal(w32.COM_E_INVALIDARG, callSlot(fullDocument, ifaceText, 2, uintptr(unsafe.Pointer(stranger)),
		out.fresh()))
	c.Equal(w32.COM_E_INVALIDARG, callSlot(fullDocument, ifaceText, 2,
		fullDocument.ifacePtr(ifaceSimple), out.fresh()), "and the document is not a child of itself")

	// A document that loses its stream between the interface being handed out and a call arriving says so, which is
	// what E_NOTSUPPORTED is for: the element is still there, and no longer supports the pattern.
	plain := textFixtureTree()
	plain.Generation = 2
	plain.Node(textDocumentID).Document = nil
	full.Publish(plain, nil)
	flag, flagAddress := pinnedOut[int32](&pin)
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 0, out.fresh()), "GetSelection")
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 1, out.fresh()), "GetVisibleRanges")
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 2, outside.ifacePtr(ifaceSimple),
		out.fresh()), "RangeFromChild")
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 4, out.fresh()), "get_DocumentRange")
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 5, out.fresh()),
		"get_SupportedTextSelection")
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 6, 0, out.fresh()), "RangeFromAnnotation")
	c.Equal(E_NOTSUPPORTED, callSlot(fullDocument, ifaceText, 7, flagAddress, out.fresh()),
		"GetCaretRange")
	c.Equal(int32(0), *flag, "and the caret is reported as inactive rather than left untouched")

	// Every out-parameter is checked rather than written through.
	other := newTestWindow(t, textFixtureTree())
	otherDocument := other.providerFor(textDocumentID)
	c.NotNil(otherDocument)
	for _, method := range []int{0, 1, 4, 5} {
		c.Equal(w32.COM_E_POINTER, callSlot(otherDocument, ifaceText, method, 0), "method %d", method)
	}
	c.Equal(w32.COM_E_POINTER, callSlot(otherDocument, ifaceText, 2, 0, 0))
	c.Equal(w32.COM_E_POINTER, callSlot(otherDocument, ifaceText, 6, 0, 0))
	c.Equal(w32.COM_E_POINTER, callSlot(otherDocument, ifaceText, 7, 0, 0))
	c.Equal(w32.COM_E_POINTER, callSlot(otherDocument, ifaceText, 7, out.fresh(), 0))
}

// TestTextChildProvider covers ITextChildProvider beyond the slot order: an element with no stretch of its own is
// answered with its ancestor's, and an element outside the document has no text pattern to be a child of.
func TestTextChildProvider(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	w := newTestWindow(t, textFixtureTree())
	document := w.providerFor(textDocumentID)
	c.NotNil(document)

	// The label inside the paragraph has no stretch of the stream of its own, so it is answered with the paragraph's,
	// which is the smallest stretch that is certainly its.
	label := w.providerFor(textLabelID)
	c.NotNil(label)
	c.Equal(w32.COM_S_OK, callSlot(label, ifaceTextChild, 1, out.fresh()))
	rng := rangeFromOut(c, out.ptr())
	c.NotNil(rng)
	if rng != nil {
		start, end := rng.offsets()
		c.Equal(6, start)
		c.Equal(22, end)
	}

	// A virtual table row is answered like anything else, which is what lets a client reading a table by row find its
	// text.
	row := w.providerFor(textRowID)
	c.NotNil(row)
	c.Equal(w32.COM_S_OK, callSlot(row, ifaceTextChild, 0, out.fresh()))
	c.Equal(document.ifacePtr(ifaceSimple), out.ptr())
	providerFromThis(out.ptr(), ifaceSimple).release()

	// An element outside the document does not support the pattern, which is what it is told when it is asked anyway.
	outside := w.providerFor(textButtonID)
	c.NotNil(outside)
	c.Equal(E_NOTSUPPORTED, callSlot(outside, ifaceTextChild, 0, out.fresh()))
	c.Equal(E_NOTSUPPORTED, callSlot(outside, ifaceTextChild, 1, out.fresh()))

	// And so does everything inside a document that has lost its stream.
	plain := textFixtureTree()
	plain.Generation = 2
	plain.Node(textDocumentID).Document = nil
	w.Publish(plain, nil)
	c.Equal(E_NOTSUPPORTED, callSlot(label, ifaceTextChild, 1, out.fresh()))
	c.Equal(w32.COM_E_POINTER, callSlot(label, ifaceTextChild, 1, 0))
}

// TestTextRangeFromPoint verifies the one provider method that takes a structure by value. It is called the way UI
// Automation calls it — through the vtable slot on amd64, and through the shim that fills the floating-point registers
// on arm64 — so that the architecture's own argument passing is what is exercised; see textRangeFromPointCall.
//
// The point is in screen pixels, which the window's geometry converts: an origin of (100,50) and a scale of two, so the
// document's own top left corner is at (120,90) and one character is twenty screen pixels wide — the fixture's advances
// are ten logical units per rune, and the scale doubles them.
func TestTextRangeFromPoint(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, textFixtureTree())
	document := w.providerFor(textDocumentID)
	c.NotNil(document)

	*out = 0
	c.Equal(w32.COM_S_OK, textRangeFromPointCall(t, document.ifacePtr(ifaceText), 120+50, 90+5, outAddress))
	r := rangeFromOut(c, *out)
	c.NotNil(r)
	if r != nil {
		start, end := r.offsets()
		c.Equal(2, start, "the third character of the first line, which is fifty screen pixels along it")
		c.Equal(2, end, "and a degenerate range, which is a caret position")
	}

	// A point below everything answers with the last line rather than failing: RangeFromPoint is documented to report
	// the nearest range.
	*out = 0
	c.Equal(w32.COM_S_OK, textRangeFromPointCall(t, document.ifacePtr(ifaceText), 125, 90+1000, outAddress))
	r = rangeFromOut(c, *out)
	c.NotNil(r)
	if r != nil {
		start, _ := r.offsets()
		c.Equal(35, start)
	}

	// A NULL out-parameter is refused rather than written through.
	c.Equal(w32.COM_E_POINTER, textRangeFromPointCall(t, document.ifacePtr(ifaceText), 120, 90, 0))
}

// TestTextBlockNames verifies that the blocks a document is made of answer the Name property with their own content,
// which is what Narrator's item navigation speaks as it steps onto each of them. A paragraph the widget gave no name
// would otherwise be announced as a bare "text".
func TestTextBlockNames(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	value, valueAddress := pinnedOut[VARIANT](&pin)
	w := newTestWindow(t, textFixtureTree())

	name := func(id accessibility.NodeID) string {
		p := w.providerFor(id)
		c.NotNil(p)
		*value = VARIANT{}
		c.Equal(w32.COM_S_OK, simpleGetPropertyValue(p.ifacePtr(ifaceSimple), uintptr(NamePropertyId),
			valueAddress))
		defer value.Clear()
		return variantString(value)
	}
	c.Equal("Hello link ￼ end", name(textParagraphID), "a paragraph is named by what it says")
	c.Equal("x = 1", name(textCodeID))
	c.Equal("A", name(textCellAID))
	c.Equal("Title", name(textHeadingID), "a heading keeps the name folded from its fragments")
	c.Equal("link", name(textLinkID))
	c.Equal("Notes", name(textDocumentID), "and a document keeps its own")

	// A column header drawn as plain text is named by that text for the same reason a cell is: a client stepping onto
	// one would otherwise be told a column's heading is a bare "header". A label always has a name of its own, and a
	// header the widget named keeps that name.
	plain := newTestWindow(t, newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2, 3, 4}},
		&accessibility.Node{ID: 2, Role: role.ColumnHeader, Text: &accessibility.TextInfo{Text: "Name"}},
		&accessibility.Node{
			ID: 3, Role: role.ColumnHeader, Name: "Full name", Text: &accessibility.TextInfo{Text: "Name"},
		},
		&accessibility.Node{ID: 4, Role: role.Label, Name: "Note", Text: &accessibility.TextInfo{Text: "Note"}},
	))
	plainName := func(id accessibility.NodeID) string {
		p := plain.providerFor(id)
		c.NotNil(p)
		*value = VARIANT{}
		c.Equal(w32.COM_S_OK, simpleGetPropertyValue(p.ifacePtr(ifaceSimple), uintptr(NamePropertyId),
			valueAddress))
		defer value.Clear()
		return variantString(value)
	}
	c.Equal("Name", plainName(2))
	c.Equal("Full name", plainName(3))
	c.Equal("Note", plainName(4))
}

// TestTextProviderAnswersForField covers the same interface over an element that carries text of its own rather than
// a composed stream. Nothing below textInfoOf knows the difference, so what is checked here is that the COM around it
// reaches a field at all: the pattern is obtainable, the caret and the selection are the field's own, and a label —
// which accepts no selection — reports None and hands back an empty array rather than a caret the user cannot move.
func TestTextProviderAnswersForField(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	ptr, ptrAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, fieldFixtureTree())
	field := w.providerFor(fieldID)
	c.NotNil(field)
	label := w.providerFor(fieldCellLabelID)
	c.NotNil(label)

	query := func(p *Provider, guid windows.GUID) uint64 {
		wanted := guid
		pin.Pin(&wanted)
		*ptr = 0
		return queryInterface(ifaceSimple, p.ifacePtr(ifaceSimple), uintptr(unsafe.Pointer(&wanted)), ptrAddress)
	}

	// Both of the Text pattern's identifiers reach the one table, by QueryInterface and by pattern identifier alike,
	// and the field is no child of anyone's text.
	for _, guid := range []windows.GUID{ifaceIIDs[ifaceText], ifaceIIDAliases[0].guid} {
		c.Equal(w32.COM_S_OK, query(field, guid))
		c.Equal(field.ifacePtr(ifaceText), *ptr)
		c.Equal(uintptr(1), field.release())
	}
	c.Equal(w32.COM_E_NOINTERFACE, query(field, ifaceIIDs[ifaceTextChild]))
	for _, id := range []PatternID{TextPatternId, TextPattern2Id} {
		c.Equal(w32.COM_S_OK, simpleGetPatternProvider(field.ifacePtr(ifaceSimple), uintptr(id), ptrAddress))
		c.Equal(field.ifacePtr(ifaceText), *ptr, "pattern %d", id)
		c.Equal(uintptr(1), field.release())
	}

	// GetSelection is where NVDA reads the caret from after every arrow key, and here it is the field's own.
	c.Equal(w32.COM_S_OK, callSlot(field, ifaceText, 0, out.fresh()))
	ranges := safeArrayRanges(c, out.array())
	c.Equal(1, len(ranges))
	if len(ranges) == 1 {
		start, end := ranges[0].offsets()
		c.Equal(6, start)
		c.Equal(11, end)
	}

	// get_DocumentRange is the whole of the field's content, which is where a client reading it starts.
	c.Equal(w32.COM_S_OK, callSlot(field, ifaceText, 4, out.fresh()))
	whole := rangeFromOut(c, out.ptr())
	c.NotNil(whole)
	if whole != nil {
		start, end := whole.offsets()
		c.Equal(0, start)
		c.Equal(fieldFixtureLength, end)
	}

	c.Equal(w32.COM_S_OK, callSlot(field, ifaceText, 5, out.fresh()))
	c.Equal(int32(SupportedTextSelection_Single), out.i32())

	// GetCaretRange reports the caret as active, since the field is where the keyboard is.
	caretOut, caretAddress := pinnedOut[uintptr](&pin)
	c.Equal(w32.COM_S_OK, callSlot(field, ifaceText, 7, out.fresh(), caretAddress))
	c.Equal(int32(1), out.i32())
	caret := rangeFromOut(c, *caretOut)
	c.NotNil(caret)
	if caret != nil {
		start, end := caret.offsets()
		c.Equal(11, start)
		c.Equal(11, end)
	}

	// A label hands out the pattern too — that is what lets Narrator's scan mode read it by line, word and character
	// — but it accepts no selection, so it reports None and an empty array rather than a caret at offset zero.
	c.Equal(w32.COM_S_OK, query(label, ifaceIIDs[ifaceText]))
	c.Equal(uintptr(1), label.release())
	c.Equal(w32.COM_S_OK, callSlot(label, ifaceText, 5, out.fresh()))
	c.Equal(int32(SupportedTextSelection_None), out.i32())
	c.Equal(w32.COM_S_OK, callSlot(label, ifaceText, 0, out.fresh()))
	c.Equal(0, len(safeArrayRanges(c, out.array())), "an empty array rather than a NULL one")
	c.Equal(w32.COM_S_OK, callSlot(label, ifaceText, 7, out.fresh(), caretAddress))
	c.Equal(int32(0), out.i32(), "and the caret is not where the keyboard is")

	// A field that becomes Protected publishes no text, which takes the pattern away while a client holds the
	// interface: every method says E_NOTSUPPORTED and QueryInterface stops answering. The Value pattern stays, and
	// answers with the empty string every protected node reports.
	protected := fieldFixtureTree()
	protected.Generation = 2
	protected.Node(fieldID).Protected = true
	protected.Node(fieldID).Text = nil
	w.Publish(protected, nil)
	for _, method := range []int{0, 1, 4, 5} {
		c.Equal(E_NOTSUPPORTED, callSlot(field, ifaceText, method, out.fresh()), "method %d", method)
	}
	c.Equal(w32.COM_E_NOINTERFACE, query(field, ifaceIIDs[ifaceText]))
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(field.ifacePtr(ifaceSimple), uintptr(TextPatternId), ptrAddress))
	c.Equal(uintptr(0), *ptr, "an unsupported pattern is a NULL interface with S_OK rather than a failure")
	c.Equal(w32.COM_S_OK, simpleGetPatternProvider(field.ifacePtr(ifaceSimple), uintptr(ValuePatternId),
		ptrAddress))
	c.Equal(field.ifacePtr(ifaceValue), *ptr, "the Value pattern is not what the flag takes away")
	c.Equal(uintptr(1), field.release())
}

// TestTextChildProviderAbsentOutsideDocument verifies the other half of the exchange between the two patterns. An
// element carrying text of its own that no document has claimed is a text container rather than a child of one, so it
// answers the Text identifier and refuses the TextChild one — and nothing in a window without a document has any
// business answering ITextChildProvider at all.
func TestTextChildProviderAbsentOutsideDocument(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out := slotScratch(&pin)
	ptr, ptrAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, fieldFixtureTree())

	for _, id := range []accessibility.NodeID{
		fieldID, fieldHeadingID, fieldCellLabelID, fieldCellID, fieldTableID, fieldButtonID,
	} {
		p := w.providerFor(id)
		c.NotNil(p)
		wanted := ifaceIIDs[ifaceTextChild]
		pin.Pin(&wanted)
		*ptr = 0
		c.Equal(w32.COM_E_NOINTERFACE, queryInterface(ifaceSimple, p.ifacePtr(ifaceSimple),
			uintptr(unsafe.Pointer(&wanted)), ptrAddress), "node %d", id)
		c.Equal(E_NOTSUPPORTED, callSlot(p, ifaceTextChild, 0, out.fresh()), "node %d", id)
		c.Equal(E_NOTSUPPORTED, callSlot(p, ifaceTextChild, 1, out.fresh()), "node %d", id)
	}

	// The blocks of a real document are the ones that do answer it, which is what TestTextChildProvider covers; here
	// the point is that gaining it costs them the Text pattern, so the two are never both obtainable.
	doc := newTestWindow(t, textFixtureTree())
	paragraph := doc.providerFor(textParagraphID)
	c.NotNil(paragraph)
	wanted := ifaceIIDs[ifaceText]
	pin.Pin(&wanted)
	*ptr = 0
	c.Equal(w32.COM_E_NOINTERFACE, queryInterface(ifaceSimple, paragraph.ifacePtr(ifaceSimple),
		uintptr(unsafe.Pointer(&wanted)), ptrAddress), "the document owns the paragraph's words")
	c.Equal(w32.COM_S_OK, callSlot(paragraph, ifaceTextChild, 0, out.fresh()))
	providerFromThis(out.ptr(), ifaceSimple).release()
}

// TestTextRangeFromPointOnField verifies the hit test over an owner whose lines sit at its own origin. The point is
// in screen pixels, which the window's geometry converts: an origin of (100,50) and a scale of two, so the field's own
// top left corner is at (120,90) and one character is twenty screen pixels wide.
func TestTextRangeFromPointOnField(t *testing.T) {
	c := check.New(t)
	var pin runtime.Pinner
	defer pin.Unpin()
	out, outAddress := pinnedOut[uintptr](&pin)
	w := newTestWindow(t, fieldFixtureTree())
	field := w.providerFor(fieldID)
	c.NotNil(field)

	*out = 0
	c.Equal(w32.COM_S_OK, textRangeFromPointCall(t, field.ifacePtr(ifaceText), 120+50, 90+5, outAddress))
	r := rangeFromOut(c, *out)
	c.NotNil(r)
	if r != nil {
		start, end := r.offsets()
		c.Equal(2, start, "the third character of the first line, which is fifty screen pixels along it")
		c.Equal(2, end, "and a degenerate range, which is a caret position")
	}

	// A point below everything answers with the last line, clipped away by the scroll area or not: a hit test is
	// about where the text is, not about what is visible.
	*out = 0
	c.Equal(w32.COM_S_OK, textRangeFromPointCall(t, field.ifacePtr(ifaceText), 125, 90+1000, outAddress))
	r = rangeFromOut(c, *out)
	c.NotNil(r)
	if r != nil {
		start, _ := r.offsets()
		c.Equal(11, start, "the third line begins at offset 11")
	}
}
