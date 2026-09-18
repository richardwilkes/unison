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
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
)

// This file holds the two interfaces that let a client read a document as text rather than as a pile of elements:
// ITextProvider2 on the document itself, and ITextChildProvider on everything inside it. The ranges they hand out are
// objects of their own; see uia_textrange_windows.go.
//
// Every answer is worked out by uia_text.go, which is portable and tested on any platform, so what is left here is the
// COM: checking out-parameters, turning stretches of the stream into range objects with the right reference counts, and
// getting each HRESULT right. The three answers a method gives instead of an answer are the ones listed at the top of
// uia_patterns_windows.go, with one addition: a client that names an element from another provider, or one that is not
// in this document's stream, is refused with E_INVALIDARG, since that is a mistake on its side rather than a state of
// ours.
//
// Narrator is the client all of this is for. Its scan mode reads a document by line, word and character through these
// methods, and NVDA's caret reading follows the selection through them; neither can read a Unison document at all
// without them.

// The two reserved-value entry points, held in variables for the reason the ones in uia_events_windows.go are: a test
// has no UI Automation Core to get a singleton from, so it stands in for these and checks what the provider did with
// what it was given. Nothing but a test ever replaces them.
var (
	uiaGetReservedNotSupportedValue   = UiaGetReservedNotSupportedValue
	uiaGetReservedMixedAttributeValue = UiaGetReservedMixedAttributeValue
)

// uiaBuildTextVtbls fills in the virtual method tables of the Text pattern's interface, of the TextChild pattern's, and
// of the text range object. The order of the methods in each call is the order the interface declares them in; see the
// interface declarations in the Windows SDK's uiautomationcore.idl.
//
// ITextProvider2's table is also ITextProvider's: the first six slots are that interface's six methods, in its own
// order, and RangeFromAnnotation and GetCaretRange follow. One table answering both is what lets a client that asks for
// either identifier be handed the same object; see uiaIfaceIIDAliases.
func uiaBuildTextVtbls() {
	// RangeFromPoint takes a UiaPoint by value, which arrives differently on each architecture — by reference on amd64
	// and in a pair of floating-point registers on arm64 — so which callback or thunk belongs in its slot is decided
	// per architecture. See uia_text_point_windows_amd64.go and uia_text_point_windows_arm64.go.
	uiaBuildVtbl(uiaIfaceText, uiaTextVtbl[:],
		uiaTextProviderGetSelection,
		uiaTextProviderGetVisibleRanges,
		uiaTextProviderRangeFromChild,
		uiaTextRangeFromPointSlot(),
		uiaTextProviderDocumentRange,
		uiaTextProviderSupportedTextSelection,
		uiaTextProviderRangeFromAnnotation,
		uiaTextProviderGetCaretRange,
	)
	uiaBuildVtbl(uiaIfaceTextChild, uiaTextChildVtbl[:],
		uiaTextChildTextContainer,
		uiaTextChildTextRange,
	)
	uiaBuildTextRangeVtbl()
}

// uiaTextProviderDocument recovers what one of the Text pattern's methods needs to answer: the provider it was called
// on and the view of the document's stream it answers from. hr is COM_S_OK when the method may go ahead and otherwise
// is what it must return instead.
//
// A node that no longer carries a stream is reported as no longer supporting the pattern, which is what it is: the
// pattern is granted by Node.Document being there, and a publish can take it away while a client holds the interface.
func uiaTextProviderDocument(this uintptr, iface uiaIface) (p *UIAProvider, doc *uiaTextDocument, hr uint64) {
	p, tree, node, hr := uiaPatternNode(this, iface)
	if hr != COM_S_OK {
		return p, nil, hr
	}
	if doc = uiaMemoizedTextDocument(tree, node.ID); doc == nil {
		return p, nil, UIA_E_NOTSUPPORTED
	}
	return p, doc, COM_S_OK
}

// uiaStoreRange hands one stretch of a document's stream to a client as a text range. The reference the new range holds
// becomes the caller's, which is what an interface out-parameter means.
func uiaStoreRange(p *UIAProvider, doc *uiaTextDocument, start, end int, out uintptr) uint64 {
	r := newUIATextRange(p.window, doc.Node().ID, start, end)
	*xruntime.PtrFromUintptr[uintptr](out) = r.this()
	return COM_S_OK
}

// uiaStoreRangeArray hands several stretches of a document's stream to a client as a SAFEARRAY of text ranges. An empty
// set yields a valid empty array rather than a NULL one: "there is none" is an answer, while a NULL array is
// indistinguishable from an allocation failure to most clients.
//
// Storing a range into the array adds a reference of its own, so each range is created holding one, handed to the
// array, and then released here — leaving the array holding exactly the one reference per element that UI Automation
// Core will release when it destroys the array.
func uiaStoreRangeArray(p *UIAProvider, doc *uiaTextDocument, stretches [][2]int, out uintptr) uint64 {
	ranges := make([]*UIATextRange, 0, len(stretches))
	defer func() {
		for _, r := range ranges {
			r.release()
		}
	}()
	pointers := make([]unsafe.Pointer, 0, len(stretches))
	for _, stretch := range stretches {
		r := newUIATextRange(p.window, doc.Node().ID, stretch[0], stretch[1])
		ranges = append(ranges, r)
		pointers = append(pointers, r.Unknown())
	}
	array := NewSafeArrayUnknown(pointers)
	if array == 0 {
		return COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return COM_S_OK
}

// uiaTextProviderGetSelection implements ITextProvider::GetSelection, which is where a client reads the caret from: a
// document with nothing selected reports the degenerate range the caret sits at, which is what NVDA follows after every
// arrow key.
//
// A document that allows no selection at all reports an empty array. It has no caret to place — a Markdown view accepts
// one only while it can take the focus — and get_SupportedTextSelection says as much, so an array holding a range at
// offset zero would have a client announce a caret the user cannot move.
func uiaTextProviderGetSelection(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	var stretches [][2]int
	if doc.SupportedSelection() != SupportedTextSelection_None {
		start, end := doc.Selection()
		stretches = [][2]int{{start, end}}
	}
	return uiaStoreRangeArray(p, doc, stretches, out)
}

// uiaTextProviderGetVisibleRanges implements ITextProvider::GetVisibleRanges. A document is one continuous stream laid
// out in one column, so what is visible of it is one stretch rather than several; a document scrolled out of view, or
// clipped away by an ancestor, reports an empty array.
func uiaTextProviderGetVisibleRanges(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	var stretches [][2]int
	if start, end, ok := doc.VisibleRange(); ok {
		stretches = [][2]int{{start, end}}
	}
	return uiaStoreRangeArray(p, doc, stretches, out)
}

// uiaTextProviderRangeFromChild implements ITextProvider::RangeFromChild, which is how a client that has found an
// element — a link, an image, a heading — asks which part of the document it is.
//
// An element this document's stream does not hold is an invalid argument, as UI Automation defines it, and so is one
// from another window: the pointer is validated against the providers this process has handed out rather than followed,
// since a client may pass anything at all. See uiaLookupProvider.
func uiaTextProviderRangeFromChild(this, child, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	other := uiaLookupProvider(child)
	if other == nil || other.window != p.window {
		return COM_E_INVALIDARG
	}
	start, end, ok := doc.SpanFor(other.node)
	if !ok {
		return COM_E_INVALIDARG
	}
	return uiaStoreRange(p, doc, start, end, out)
}

// uiaTextProviderRangeFromPointAt implements ITextProvider::RangeFromPoint, once the point has been taken out of
// whatever registers the architecture delivered it in. The point is in screen pixels and the stream's lines are in
// window-local logical units, so the window's geometry converts before the document is asked.
//
// Every point has an answer, since the method is documented to report the range nearest the point rather than to fail:
// a point above the text answers with its beginning and one below it with its end. The range is degenerate — a caret
// position — which is what a client placing the caret with the mouse wants.
func uiaTextProviderRangeFromPointAt(this uintptr, x, y float64, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	offset := doc.OffsetAt(p.window.Geometry().WindowPoint(x, y))
	return uiaStoreRange(p, doc, offset, offset, out)
}

// uiaTextProviderDocumentRange implements ITextProvider::get_DocumentRange, the range every other one is reached from:
// a client reading a document starts here, expands to a unit, and walks.
func uiaTextProviderDocumentRange(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	return uiaStoreRange(p, doc, 0, doc.Len(), out)
}

// uiaTextProviderSupportedTextSelection implements ITextProvider::get_SupportedTextSelection.
func uiaTextProviderSupportedTextSelection(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearInt32(out)
	_, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[int32](out) = int32(doc.SupportedSelection())
	return COM_S_OK
}

// uiaTextProviderRangeFromAnnotation implements ITextProvider2::RangeFromAnnotation. Nothing in a document here is
// annotated — no comments, no tracked changes, no spelling errors, and the AnnotationTypes attribute answers that it is
// not supported — so there is no annotation element a client could have to ask about. A NULL range with S_OK is the
// documented answer for an annotation the provider cannot place, which is every one of them.
func uiaTextProviderRangeFromAnnotation(this, _, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	_, _, hr := uiaTextProviderDocument(this, uiaIfaceText)
	return hr
}

// uiaTextProviderGetCaretRange implements ITextProvider2::GetCaretRange, which is how a client finds the reading caret
// without a selection to go by. isActive says whether the caret is in the element the keyboard is in, which is what
// decides whether a client follows it: a caret in a document the user is not typing into is still where reading would
// resume, but it is not where the user is.
func uiaTextProviderGetCaretRange(this, isActive, out uintptr) uint64 {
	if out == 0 || isActive == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	uiaSetBOOL(isActive, false)
	p, doc, hr := uiaTextProviderDocument(this, uiaIfaceText)
	if hr != COM_S_OK {
		return hr
	}
	uiaSetBOOL(isActive, doc.Focused())
	caret := doc.Caret()
	return uiaStoreRange(p, doc, caret, caret, out)
}

// uiaTextChildTextContainer implements ITextChildProvider::get_TextContainer, answering with the document whose stream
// this element sits in. See uiaTextContainerFor, which is also what grants the pattern, so an element that reports the
// pattern always has a container to name.
func uiaTextChildTextContainer(this, out uintptr) uint64 {
	return uiaPatternProvider(this, uiaIfaceTextChild, out,
		func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
			return uiaTextContainerFor(t, n)
		})
}

// uiaTextChildTextRange implements ITextChildProvider::get_TextRange, answering with the stretch of the document's text
// this element occupies. It is the other half of what makes an element inside a document readable: a client that has
// walked the element tree to a link uses it to read the text around that link.
func uiaTextChildTextRange(this, out uintptr) uint64 {
	if out == 0 {
		return COM_E_POINTER
	}
	uiaClearPointer(out)
	p, tree, node, hr := uiaPatternNode(this, uiaIfaceTextChild)
	if hr != COM_S_OK {
		return hr
	}
	doc := uiaMemoizedTextDocument(tree, uiaTextContainerFor(tree, node))
	if doc == nil {
		return UIA_E_NOTSUPPORTED
	}
	start, end, ok := doc.SpanFor(node.ID)
	if !ok {
		return UIA_E_NOTSUPPORTED
	}
	return uiaStoreRange(p, doc, start, end, out)
}
