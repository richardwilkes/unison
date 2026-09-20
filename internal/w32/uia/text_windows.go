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
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/w32"
)

// This file holds the two interfaces that let a client read text rather than a pile of elements: ITextProvider2 on
// the element that carries the text — a Markdown view with its composed stream, or a field, a label, a heading or a
// cell with its own — and ITextChildProvider on everything inside a document, which points back at the document and
// at the stretch of it that element occupies. The ranges they hand out are objects of their own; see
// textrange_windows.go.
//
// Every answer is worked out by text.go, which is portable and tested on any platform, so what is left here is the
// COM: checking out-parameters, turning stretches of the stream into range objects with the right reference counts, and
// getting each HRESULT right. The three answers a method gives instead of an answer are the ones listed at the top of
// patterns_windows.go, with one addition: a client that names an element from another provider, or one that is not
// in this element's text, is refused with E_INVALIDARG, since that is a mistake on its side rather than a state of
// ours.
//
// Narrator is the client all of this is for. Its scan mode reads an element by line, word and character through these
// methods, and NVDA's caret reading follows the selection through them; without them neither can read a Unison
// document at all, and arrowing through a field says nothing.

// The two reserved-value entry points, held in variables for the reason the ones in events_windows.go are: a test
// has no UI Automation Core to get a singleton from, so it stands in for these and checks what the provider did with
// what it was given. Nothing but a test ever replaces them.
var (
	getReservedNotSupportedValue   = GetReservedNotSupportedValue
	getReservedMixedAttributeValue = GetReservedMixedAttributeValue
)

// buildTextVtbls fills in the virtual method tables of the Text pattern's interface, of the TextChild pattern's, and
// of the text range object. The order of the methods in each call is the order the interface declares them in; see the
// interface declarations in the Windows SDK's uiautomationcore.idl.
//
// ITextProvider2's table is also ITextProvider's: the first six slots are that interface's six methods, in its own
// order, and RangeFromAnnotation and GetCaretRange follow. One table answering both is what lets a client that asks for
// either identifier be handed the same object; see ifaceIIDAliases.
func buildTextVtbls() {
	// RangeFromPoint takes a Point by value, which arrives differently on each architecture — by reference on amd64
	// and in a pair of floating-point registers on arm64 — so which callback or thunk belongs in its slot is decided
	// per architecture. See text_point_windows_amd64.go and text_point_windows_arm64.go.
	buildVtbl(ifaceText, textVtbl[:],
		textProviderGetSelection,
		textProviderGetVisibleRanges,
		textProviderRangeFromChild,
		textRangeFromPointSlot(),
		textProviderDocumentRange,
		textProviderSupportedTextSelection,
		textProviderRangeFromAnnotation,
		textProviderGetCaretRange,
	)
	buildVtbl(ifaceTextChild, textChildVtbl[:],
		textChildTextContainer,
		textChildTextRange,
	)
	buildTextRangeVtbl()
}

// textProviderDocument recovers what one of the Text pattern's methods needs to answer: the provider it was called
// on and the view of the element's text it answers from. hr is w32.COM_S_OK when the method may go ahead and
// otherwise is what it must return instead.
//
// A node that no longer carries text is reported as no longer supporting the pattern, which is what it is: the
// pattern is granted by the text being there — a field that becomes Protected loses it, as does a block a document
// has begun to claim — and a publish can take it away while a client holds the interface. See textInfoOf.
func textProviderDocument(this uintptr, which iface) (p *Provider, doc *textDocument, hr uint64) {
	p, tree, node, hr := patternNode(this, which)
	if hr != w32.COM_S_OK {
		return p, nil, hr
	}
	if doc = memoizedTextDocument(tree, node.ID); doc == nil {
		return p, nil, E_NOTSUPPORTED
	}
	return p, doc, w32.COM_S_OK
}

// storeRange hands one stretch of an element's text to a client as a text range. The reference the new range holds
// becomes the caller's, which is what an interface out-parameter means.
func storeRange(p *Provider, doc *textDocument, start, end int, out uintptr) uint64 {
	r := newTextRange(p.window, doc.Node().ID, start, end)
	*xruntime.PtrFromUintptr[uintptr](out) = r.this()
	return w32.COM_S_OK
}

// storeRangeArray hands several stretches of an element's text to a client as a SAFEARRAY of text ranges. An empty
// set yields a valid empty array rather than a NULL one: "there is none" is an answer, while a NULL array is
// indistinguishable from an allocation failure to most clients.
//
// Storing a range into the array adds a reference of its own, so each range is created holding one, handed to the
// array, and then released here — leaving the array holding exactly the one reference per element that UI Automation
// Core will release when it destroys the array.
func storeRangeArray(p *Provider, doc *textDocument, stretches [][2]int, out uintptr) uint64 {
	ranges := make([]*TextRange, 0, len(stretches))
	defer func() {
		for _, r := range ranges {
			r.release()
		}
	}()
	pointers := make([]unsafe.Pointer, 0, len(stretches))
	for _, stretch := range stretches {
		r := newTextRange(p.window, doc.Node().ID, stretch[0], stretch[1])
		ranges = append(ranges, r)
		pointers = append(pointers, r.Unknown())
	}
	array := NewSafeArrayUnknown(pointers)
	if array == 0 {
		return w32.COM_E_OUTOFMEMORY
	}
	*xruntime.PtrFromUintptr[SAFEARRAY](out) = array
	return w32.COM_S_OK
}

// textProviderGetSelection implements ITextProvider::GetSelection, which is where a client reads the caret from: an
// element with nothing selected reports the degenerate range the caret sits at, which is what NVDA follows after
// every arrow key in a field.
//
// An element that allows no selection at all reports an empty array. It has no caret to place — a label never accepts
// one — and get_SupportedTextSelection says as much, so an array holding a range at offset zero would have a client
// announce a caret the user cannot move.
func textProviderGetSelection(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	var stretches [][2]int
	if doc.SupportedSelection() != SupportedTextSelection_None {
		start, end := doc.Selection()
		stretches = [][2]int{{start, end}}
	}
	return storeRangeArray(p, doc, stretches, out)
}

// textProviderGetVisibleRanges implements ITextProvider::GetVisibleRanges. The text is one continuous stream laid
// out in one column, so what is visible of it is one stretch rather than several; an element scrolled out of view, or
// clipped away by an ancestor, reports an empty array.
func textProviderGetVisibleRanges(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	var stretches [][2]int
	if start, end, ok := doc.VisibleRange(); ok {
		stretches = [][2]int{{start, end}}
	}
	return storeRangeArray(p, doc, stretches, out)
}

// textProviderRangeFromChild implements ITextProvider::RangeFromChild, which is how a client that has found an
// element — a link, an image, a heading — asks which part of the text it is. Only a document has elements within its
// text; for anything else every child is an invalid argument, since none of them occupies a stretch of it.
//
// An element this text does not hold is an invalid argument, as UI Automation defines it, and so is one from another
// window: the pointer is validated against the providers this process has handed out rather than followed,
// since a client may pass anything at all. See lookupProvider.
func textProviderRangeFromChild(this, child, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	other := lookupProvider(child)
	if other == nil || other.window != p.window {
		return w32.COM_E_INVALIDARG
	}
	start, end, ok := doc.SpanFor(other.node)
	if !ok {
		return w32.COM_E_INVALIDARG
	}
	return storeRange(p, doc, start, end, out)
}

// textProviderRangeFromPointAt implements ITextProvider::RangeFromPoint, once the point has been taken out of
// whatever registers the architecture delivered it in. The point is in screen pixels and the stream's lines are in
// window-local logical units, so the window's geometry converts before the document is asked.
//
// Every point has an answer, since the method is documented to report the range nearest the point rather than to fail:
// a point above the text answers with its beginning and one below it with its end. The range is degenerate — a caret
// position — which is what a client placing the caret with the mouse wants.
func textProviderRangeFromPointAt(this uintptr, x, y float64, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	offset := doc.OffsetAt(p.window.Geometry().WindowPoint(x, y))
	return storeRange(p, doc, offset, offset, out)
}

// textProviderDocumentRange implements ITextProvider::get_DocumentRange, the range every other one is reached from:
// a client reading an element's text starts here, expands to a unit, and walks.
func textProviderDocumentRange(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	return storeRange(p, doc, 0, doc.Len(), out)
}

// textProviderSupportedTextSelection implements ITextProvider::get_SupportedTextSelection.
func textProviderSupportedTextSelection(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearInt32(out)
	_, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	*xruntime.PtrFromUintptr[int32](out) = int32(doc.SupportedSelection())
	return w32.COM_S_OK
}

// textProviderRangeFromAnnotation implements ITextProvider2::RangeFromAnnotation. Nothing here is annotated — no
// comments, no tracked changes, no spelling errors, and the AnnotationTypes attribute answers that it is not
// supported — so there is no annotation element a client could have to ask about. A NULL range with S_OK is the
// documented answer for an annotation the provider cannot place, which is every one of them.
func textProviderRangeFromAnnotation(this, _, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	_, _, hr := textProviderDocument(this, ifaceText)
	return hr
}

// textProviderGetCaretRange implements ITextProvider2::GetCaretRange, which is how a client finds the reading caret
// without a selection to go by. isActive says whether the caret is in the element the keyboard is in, which is what
// decides whether a client follows it: a caret in text the user is not typing into is still where reading would
// resume, but it is not where the user is.
func textProviderGetCaretRange(this, isActive, out uintptr) uint64 {
	if out == 0 || isActive == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	setBOOL(isActive, false)
	p, doc, hr := textProviderDocument(this, ifaceText)
	if hr != w32.COM_S_OK {
		return hr
	}
	setBOOL(isActive, doc.Focused())
	caret := doc.Caret()
	return storeRange(p, doc, caret, caret, out)
}

// textChildTextContainer implements ITextChildProvider::get_TextContainer, answering with the document whose stream
// this element sits in. See textContainerFor, which is also what grants the pattern, so an element that reports the
// pattern always has a container to name.
func textChildTextContainer(this, out uintptr) uint64 {
	return patternProvider(this, ifaceTextChild, out,
		func(t *accessibility.Tree, n *accessibility.Node) accessibility.NodeID {
			return textContainerFor(t, n)
		})
}

// textChildTextRange implements ITextChildProvider::get_TextRange, answering with the stretch of the document's text
// this element occupies. It is the other half of what makes an element inside a document readable: a client that has
// walked the element tree to a link uses it to read the text around that link.
func textChildTextRange(this, out uintptr) uint64 {
	if out == 0 {
		return w32.COM_E_POINTER
	}
	clearPointer(out)
	p, tree, node, hr := patternNode(this, ifaceTextChild)
	if hr != w32.COM_S_OK {
		return hr
	}
	doc := memoizedTextDocument(tree, textContainerFor(tree, node))
	if doc == nil {
		return E_NOTSUPPORTED
	}
	start, end, ok := doc.SpanFor(node.ID)
	if !ok {
		return E_NOTSUPPORTED
	}
	return storeRange(p, doc, start, end, out)
}
