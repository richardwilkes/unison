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

	"github.com/richardwilkes/toolbox/v2/geom"
)

// This file holds the parts of the UI Automation provider's shape that do not touch the OS: which COM interface each of
// a provider's virtual method tables belongs to, how many slots each of those tables has, where each table pointer sits
// within the provider, which interface implements which control pattern, and how a window's geometry turns the
// snapshot's coordinates into the screen coordinates UI Automation works in.
//
// None of it needs Windows to be meaningful, so the file carries no build constraint and its tests — the ones that
// matter most, since an error in the table layout or in the coordinate conversion is invisible until a screen reader
// misbehaves — run on every platform.

// iface identifies one of the COM interfaces a Provider implements. It is also the index of that interface's
// virtual method table pointer within the provider, which is what makes a single Go object answer to many interfaces:
// the provider begins with one table pointer per interface, in this order, exactly as a C++ object with multiple base
// classes does. The pointer a client holds for interface i is the address of slot i, so a method of interface i
// recovers the provider it was called on by subtracting ifaceOffset(i) from its this pointer.
type iface int

// The interfaces a provider implements, in the order their table pointers appear within it. The order is part of the
// ABI only in the sense that it must stay consistent between the tables, the offsets and the this-pointer arithmetic;
// clients never see it. It follows §7.2 of the plan: the two interfaces every element implements, then the three the
// fragment root alone implements, then one per control pattern.
const (
	ifaceSimple         iface = iota // IRawElementProviderSimple, which also answers IID_IUnknown
	ifaceFragment                    // IRawElementProviderFragment
	ifaceFragmentRoot                // IRawElementProviderFragmentRoot, root only
	ifaceAdviseEvents                // IRawElementProviderAdviseEvents, root only
	ifaceWindow                      // IWindowProvider, root only
	ifaceInvoke                      // IInvokeProvider
	ifaceToggle                      // IToggleProvider
	ifaceValue                       // IValueProvider
	ifaceRangeValue                  // IRangeValueProvider
	ifaceSelection                   // ISelectionProvider
	ifaceSelectionItem               // ISelectionItemProvider
	ifaceExpandCollapse              // IExpandCollapseProvider
	ifaceScrollItem                  // IScrollItemProvider
	ifaceGrid                        // IGridProvider
	ifaceGridItem                    // IGridItemProvider
	ifaceTable                       // ITableProvider
	ifaceTableItem                   // ITableItemProvider
	ifaceText                        // ITextProvider2, which also answers the ITextProvider IID
	ifaceTextChild                   // ITextChildProvider
	ifaceCount                       // Not an interface: how many there are
)

// The number of slots in each interface's virtual method table. Every COM interface here derives directly from
// IUnknown, so each table begins with the same three slots and continues with the interface's own methods in the order
// the interface declares them. Getting one of these counts wrong would leave a slot holding whatever the zero value of
// a uintptr points at, so they are written as the IUnknown count plus the method count of each interface, which can be
// checked against the interface declarations in the Windows SDK's uiautomationcore.idl one line at a time.
const (
	unknownSlots        = 3                // QueryInterface, AddRef, Release
	simpleSlots         = unknownSlots + 4 // get_ProviderOptions .. get_HostRawElementProvider
	fragmentSlots       = unknownSlots + 6 // Navigate .. get_FragmentRoot
	fragmentRootSlots   = unknownSlots + 2 // ElementProviderFromPoint, GetFocus
	adviseEventsSlots   = unknownSlots + 2 // AdviseEventAdded, AdviseEventRemoved
	windowSlots         = unknownSlots + 9 // SetVisualState .. get_IsTopmost
	invokeSlots         = unknownSlots + 1 // Invoke
	toggleSlots         = unknownSlots + 2 // Toggle, get_ToggleState
	valueSlots          = unknownSlots + 3 // SetValue, get_Value, get_IsReadOnly
	rangeValueSlots     = unknownSlots + 7 // SetValue .. get_SmallChange
	selectionSlots      = unknownSlots + 3 // GetSelection, get_CanSelectMultiple, get_IsSelectionRequired
	selectionItemSlots  = unknownSlots + 5 // Select .. get_SelectionContainer
	expandCollapseSlots = unknownSlots + 3 // Expand, Collapse, get_ExpandCollapseState
	scrollItemSlots     = unknownSlots + 1 // ScrollIntoView
	gridSlots           = unknownSlots + 3 // GetItem, get_RowCount, get_ColumnCount
	gridItemSlots       = unknownSlots + 5 // get_Row .. get_ContainingGrid
	tableSlots          = unknownSlots + 3 // GetRowHeaders, GetColumnHeaders, get_RowOrColumnMajor
	tableItemSlots      = unknownSlots + 2 // GetRowHeaderItems, GetColumnHeaderItems
	textSlots           = unknownSlots + 8 // GetSelection .. GetCaretRange
	textChildSlots      = unknownSlots + 2 // get_TextContainer, get_TextRange
)

// textRangeSlots is the size of the virtual method table of a text range, which is the one COM object in this file's
// scheme that is not a provider: a range stands for a stretch of one element's text rather than for the element, so
// it is an object of its own with a single table rather than one of a provider's interfaces. See TextRange.
//
// The table is ITextRangeProvider2's: its first eighteen methods are ITextRangeProvider's, in the order that interface
// declares them, and ShowContextMenu is the nineteenth. One object answers both interface identifiers, exactly as the
// Text pattern's provider answers both of its.
const textRangeSlots = unknownSlots + 19 // Clone .. GetChildren, then ShowContextMenu

// ifaceSlots holds the table size of each interface, indexed by iface, so that a test can check the tables the
// Windows-only code declares against the counts above without repeating them.
var ifaceSlots = [ifaceCount]int{
	ifaceSimple:         simpleSlots,
	ifaceFragment:       fragmentSlots,
	ifaceFragmentRoot:   fragmentRootSlots,
	ifaceAdviseEvents:   adviseEventsSlots,
	ifaceWindow:         windowSlots,
	ifaceInvoke:         invokeSlots,
	ifaceToggle:         toggleSlots,
	ifaceValue:          valueSlots,
	ifaceRangeValue:     rangeValueSlots,
	ifaceSelection:      selectionSlots,
	ifaceSelectionItem:  selectionItemSlots,
	ifaceExpandCollapse: expandCollapseSlots,
	ifaceScrollItem:     scrollItemSlots,
	ifaceGrid:           gridSlots,
	ifaceGridItem:       gridItemSlots,
	ifaceTable:          tableSlots,
	ifaceTableItem:      tableItemSlots,
	ifaceText:           textSlots,
	ifaceTextChild:      textChildSlots,
}

// ifacePointerSize is the size of one virtual method table pointer within a provider. Every platform this package
// builds for has 8-byte pointers, but naming it keeps the arithmetic below readable and correct regardless.
const ifacePointerSize = unsafe.Sizeof(uintptr(0))

// ifaceOffset returns how far into a provider the virtual method table pointer for one interface sits. A method of
// that interface subtracts this from the this pointer it was handed to find the provider itself, which is the whole
// trick that lets one Go object implement every one of those COM interfaces without an object apiece.
func ifaceOffset(which iface) uintptr {
	return uintptr(which) * ifacePointerSize
}

// patternIface pairs one control pattern with the interface that implements it.
type patternIface struct {
	pattern PatternSet
	iface   iface
}

// patternIfaces lists the interface behind each control pattern. QueryInterface and GetPatternProvider both work
// from this list and from Patterns, which is what keeps the two answers consistent: a client that reaches a pattern
// by asking for its interface directly and one that asks for it by pattern identifier get the same answer.
//
// The Text and Text2 patterns are the one place two patterns share an interface, and they must: ITextProvider2 derives
// from ITextProvider, so the eight slots that answer Text2 begin with the six that answer Text, and a second table
// would be the same six methods twice with two ways of getting them out of step. The two patterns are granted and taken
// away together — rolePatterns hands out both or neither — so nothing can ask for one and be handed the other's
// answer. patternForIface reports Text for the shared interface, which is the pattern supports checks the element
// against, and Text2 is present for a client that asks for it by identifier.
var patternIfaces = []patternIface{
	{pattern: PatternInvoke, iface: ifaceInvoke},
	{pattern: PatternToggle, iface: ifaceToggle},
	{pattern: PatternValue, iface: ifaceValue},
	{pattern: PatternRangeValue, iface: ifaceRangeValue},
	{pattern: PatternSelection, iface: ifaceSelection},
	{pattern: PatternSelectionItem, iface: ifaceSelectionItem},
	{pattern: PatternExpandCollapse, iface: ifaceExpandCollapse},
	{pattern: PatternScrollItem, iface: ifaceScrollItem},
	{pattern: PatternGrid, iface: ifaceGrid},
	{pattern: PatternGridItem, iface: ifaceGridItem},
	{pattern: PatternTable, iface: ifaceTable},
	{pattern: PatternTableItem, iface: ifaceTableItem},
	{pattern: PatternWindow, iface: ifaceWindow},
	{pattern: PatternText, iface: ifaceText},
	{pattern: PatternText2, iface: ifaceText},
	{pattern: PatternTextChild, iface: ifaceTextChild},
}

// ifaceForPattern returns the interface that implements a single-bit pattern, and whether there is one.
func ifaceForPattern(pattern PatternSet) (which iface, ok bool) {
	for _, entry := range patternIfaces {
		if entry.pattern == pattern {
			return entry.iface, true
		}
	}
	return 0, false
}

// patternForIface returns the pattern an interface implements, or zero when the interface is not a pattern
// interface. A zero result is what makes QueryInterface refuse an interface the element does not support.
func patternForIface(which iface) PatternSet {
	for _, entry := range patternIfaces {
		if entry.iface == which {
			return entry.pattern
		}
	}
	return 0
}

// Geometry says where a window's content area sits on screen and how large its logical units are, which is
// everything needed to turn the window-local logical coordinates in an accessibility snapshot into the screen
// coordinates UI Automation asks for and answers in.
//
// The root package refreshes it whenever the window moves, resizes or changes display scale, so a provider reads it
// fresh on every call rather than caching a conversion.
type Geometry struct {
	// Origin is the position of the top-left corner of the window's content area on the virtual screen, in physical
	// pixels.
	Origin geom.Point
	// Scale is how many physical pixels one logical unit covers, horizontally and vertically. A zero or negative
	// component is treated as one, so that a geometry that was never filled in still yields usable answers rather than
	// infinities.
	Scale geom.Point
}

// ScreenRect converts a node's bounds — window-local, top-left origin, logical units — into the screen rectangle UI
// Automation wants, in physical pixels, given as an origin plus a size rather than as two corners.
func (g Geometry) ScreenRect(bounds geom.Rect) (left, top, width, height float64) {
	scaleX, scaleY := g.scale()
	return float64(g.Origin.X) + float64(bounds.X)*scaleX,
		float64(g.Origin.Y) + float64(bounds.Y)*scaleY,
		float64(bounds.Width) * scaleX,
		float64(bounds.Height) * scaleY
}

// WindowPoint converts a screen point in physical pixels into the window-local, top-left origin, logical coordinate
// space the snapshot's bounds live in. It is the inverse of ScreenRect's origin conversion, and is what a hit test
// from UI Automation goes through before it reaches the tree.
func (g Geometry) WindowPoint(x, y float64) geom.Point {
	scaleX, scaleY := g.scale()
	return geom.NewPoint(float32((x-float64(g.Origin.X))/scaleX), float32((y-float64(g.Origin.Y))/scaleY))
}

// scale returns the horizontal and vertical scale to use, substituting one for a component that was never filled in.
func (g Geometry) scale() (x, y float64) {
	x, y = float64(g.Scale.X), float64(g.Scale.Y)
	if x <= 0 {
		x = 1
	}
	if y <= 0 {
		y = 1
	}
	return x, y
}
