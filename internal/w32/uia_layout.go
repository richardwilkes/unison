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

// uiaIface identifies one of the COM interfaces a UIAProvider implements. It is also the index of that interface's
// virtual method table pointer within the provider, which is what makes a single Go object answer to many interfaces:
// the provider begins with one table pointer per interface, in this order, exactly as a C++ object with multiple base
// classes does. The pointer a client holds for interface i is the address of slot i, so a method of interface i
// recovers the provider it was called on by subtracting uiaIfaceOffset(i) from its this pointer.
type uiaIface int

// The interfaces a provider implements, in the order their table pointers appear within it. The order is part of the
// ABI only in the sense that it must stay consistent between the tables, the offsets and the this-pointer arithmetic;
// clients never see it. It follows §7.2 of the plan: the two interfaces every element implements, then the three the
// fragment root alone implements, then one per control pattern.
const (
	uiaIfaceSimple         uiaIface = iota // IRawElementProviderSimple, which also answers IID_IUnknown
	uiaIfaceFragment                       // IRawElementProviderFragment
	uiaIfaceFragmentRoot                   // IRawElementProviderFragmentRoot, root only
	uiaIfaceAdviseEvents                   // IRawElementProviderAdviseEvents, root only
	uiaIfaceWindow                         // IWindowProvider, root only
	uiaIfaceInvoke                         // IInvokeProvider
	uiaIfaceToggle                         // IToggleProvider
	uiaIfaceValue                          // IValueProvider
	uiaIfaceRangeValue                     // IRangeValueProvider
	uiaIfaceSelection                      // ISelectionProvider
	uiaIfaceSelectionItem                  // ISelectionItemProvider
	uiaIfaceExpandCollapse                 // IExpandCollapseProvider
	uiaIfaceScrollItem                     // IScrollItemProvider
	uiaIfaceGrid                           // IGridProvider
	uiaIfaceGridItem                       // IGridItemProvider
	uiaIfaceTable                          // ITableProvider
	uiaIfaceTableItem                      // ITableItemProvider
	uiaIfaceText                           // ITextProvider2, which also answers the ITextProvider IID
	uiaIfaceTextChild                      // ITextChildProvider
	uiaIfaceCount                          // Not an interface: how many there are
)

// The number of slots in each interface's virtual method table. Every COM interface here derives directly from
// IUnknown, so each table begins with the same three slots and continues with the interface's own methods in the order
// the interface declares them. Getting one of these counts wrong would leave a slot holding whatever the zero value of
// a uintptr points at, so they are written as the IUnknown count plus the method count of each interface, which can be
// checked against the interface declarations in the Windows SDK's uiautomationcore.idl one line at a time.
const (
	uiaUnknownSlots        = 3                   // QueryInterface, AddRef, Release
	uiaSimpleSlots         = uiaUnknownSlots + 4 // get_ProviderOptions .. get_HostRawElementProvider
	uiaFragmentSlots       = uiaUnknownSlots + 6 // Navigate .. get_FragmentRoot
	uiaFragmentRootSlots   = uiaUnknownSlots + 2 // ElementProviderFromPoint, GetFocus
	uiaAdviseEventsSlots   = uiaUnknownSlots + 2 // AdviseEventAdded, AdviseEventRemoved
	uiaWindowSlots         = uiaUnknownSlots + 9 // SetVisualState .. get_IsTopmost
	uiaInvokeSlots         = uiaUnknownSlots + 1 // Invoke
	uiaToggleSlots         = uiaUnknownSlots + 2 // Toggle, get_ToggleState
	uiaValueSlots          = uiaUnknownSlots + 3 // SetValue, get_Value, get_IsReadOnly
	uiaRangeValueSlots     = uiaUnknownSlots + 7 // SetValue .. get_SmallChange
	uiaSelectionSlots      = uiaUnknownSlots + 3 // GetSelection, get_CanSelectMultiple, get_IsSelectionRequired
	uiaSelectionItemSlots  = uiaUnknownSlots + 5 // Select .. get_SelectionContainer
	uiaExpandCollapseSlots = uiaUnknownSlots + 3 // Expand, Collapse, get_ExpandCollapseState
	uiaScrollItemSlots     = uiaUnknownSlots + 1 // ScrollIntoView
	uiaGridSlots           = uiaUnknownSlots + 3 // GetItem, get_RowCount, get_ColumnCount
	uiaGridItemSlots       = uiaUnknownSlots + 5 // get_Row .. get_ContainingGrid
	uiaTableSlots          = uiaUnknownSlots + 3 // GetRowHeaders, GetColumnHeaders, get_RowOrColumnMajor
	uiaTableItemSlots      = uiaUnknownSlots + 2 // GetRowHeaderItems, GetColumnHeaderItems
	uiaTextSlots           = uiaUnknownSlots + 8 // GetSelection .. GetCaretRange
	uiaTextChildSlots      = uiaUnknownSlots + 2 // get_TextContainer, get_TextRange
)

// uiaTextRangeSlots is the size of the virtual method table of a text range, which is the one COM object in this file's
// scheme that is not a provider: a range stands for a stretch of one document's text rather than for an element, so it
// is an object of its own with a single table rather than one of a provider's interfaces. See UIATextRange.
//
// The table is ITextRangeProvider2's: its first eighteen methods are ITextRangeProvider's, in the order that interface
// declares them, and ShowContextMenu is the nineteenth. One object answers both interface identifiers, exactly as the
// Text pattern's provider answers both of its.
const uiaTextRangeSlots = uiaUnknownSlots + 19 // Clone .. GetChildren, then ShowContextMenu

// uiaIfaceSlots holds the table size of each interface, indexed by uiaIface, so that a test can check the tables the
// Windows-only code declares against the counts above without repeating them.
var uiaIfaceSlots = [uiaIfaceCount]int{
	uiaIfaceSimple:         uiaSimpleSlots,
	uiaIfaceFragment:       uiaFragmentSlots,
	uiaIfaceFragmentRoot:   uiaFragmentRootSlots,
	uiaIfaceAdviseEvents:   uiaAdviseEventsSlots,
	uiaIfaceWindow:         uiaWindowSlots,
	uiaIfaceInvoke:         uiaInvokeSlots,
	uiaIfaceToggle:         uiaToggleSlots,
	uiaIfaceValue:          uiaValueSlots,
	uiaIfaceRangeValue:     uiaRangeValueSlots,
	uiaIfaceSelection:      uiaSelectionSlots,
	uiaIfaceSelectionItem:  uiaSelectionItemSlots,
	uiaIfaceExpandCollapse: uiaExpandCollapseSlots,
	uiaIfaceScrollItem:     uiaScrollItemSlots,
	uiaIfaceGrid:           uiaGridSlots,
	uiaIfaceGridItem:       uiaGridItemSlots,
	uiaIfaceTable:          uiaTableSlots,
	uiaIfaceTableItem:      uiaTableItemSlots,
	uiaIfaceText:           uiaTextSlots,
	uiaIfaceTextChild:      uiaTextChildSlots,
}

// uiaIfacePointerSize is the size of one virtual method table pointer within a provider. Every platform this package
// builds for has 8-byte pointers, but naming it keeps the arithmetic below readable and correct regardless.
const uiaIfacePointerSize = unsafe.Sizeof(uintptr(0))

// uiaIfaceOffset returns how far into a provider the virtual method table pointer for one interface sits. A method of
// that interface subtracts this from the this pointer it was handed to find the provider itself, which is the whole
// trick that lets one Go object implement every one of those COM interfaces without an object apiece.
func uiaIfaceOffset(iface uiaIface) uintptr {
	return uintptr(iface) * uiaIfacePointerSize
}

// uiaPatternIface pairs one control pattern with the interface that implements it.
type uiaPatternIface struct {
	pattern PatternSet
	iface   uiaIface
}

// uiaPatternIfaces lists the interface behind each control pattern. QueryInterface and GetPatternProvider both work
// from this list and from UIAPatterns, which is what keeps the two answers consistent: a client that reaches a pattern
// by asking for its interface directly and one that asks for it by pattern identifier get the same answer.
//
// The Text and Text2 patterns are the one place two patterns share an interface, and they must: ITextProvider2 derives
// from ITextProvider, so the eight slots that answer Text2 begin with the six that answer Text, and a second table
// would be the same six methods twice with two ways of getting them out of step. The two patterns are granted and taken
// away together — uiaRolePatterns hands out both or neither — so nothing can ask for one and be handed the other's
// answer. uiaPatternForIface reports Text for the shared interface, which is the pattern supports checks the element
// against, and Text2 is present for a client that asks for it by identifier.
var uiaPatternIfaces = []uiaPatternIface{
	{pattern: PatternInvoke, iface: uiaIfaceInvoke},
	{pattern: PatternToggle, iface: uiaIfaceToggle},
	{pattern: PatternValue, iface: uiaIfaceValue},
	{pattern: PatternRangeValue, iface: uiaIfaceRangeValue},
	{pattern: PatternSelection, iface: uiaIfaceSelection},
	{pattern: PatternSelectionItem, iface: uiaIfaceSelectionItem},
	{pattern: PatternExpandCollapse, iface: uiaIfaceExpandCollapse},
	{pattern: PatternScrollItem, iface: uiaIfaceScrollItem},
	{pattern: PatternGrid, iface: uiaIfaceGrid},
	{pattern: PatternGridItem, iface: uiaIfaceGridItem},
	{pattern: PatternTable, iface: uiaIfaceTable},
	{pattern: PatternTableItem, iface: uiaIfaceTableItem},
	{pattern: PatternWindow, iface: uiaIfaceWindow},
	{pattern: PatternText, iface: uiaIfaceText},
	{pattern: PatternText2, iface: uiaIfaceText},
	{pattern: PatternTextChild, iface: uiaIfaceTextChild},
}

// uiaIfaceForPattern returns the interface that implements a single-bit pattern, and whether there is one.
func uiaIfaceForPattern(pattern PatternSet) (iface uiaIface, ok bool) {
	for _, entry := range uiaPatternIfaces {
		if entry.pattern == pattern {
			return entry.iface, true
		}
	}
	return 0, false
}

// uiaPatternForIface returns the pattern an interface implements, or zero when the interface is not a pattern
// interface. A zero result is what makes QueryInterface refuse an interface the element does not support.
func uiaPatternForIface(iface uiaIface) PatternSet {
	for _, entry := range uiaPatternIfaces {
		if entry.iface == iface {
			return entry.pattern
		}
	}
	return 0
}

// UIAGeometry says where a window's content area sits on screen and how large its logical units are, which is
// everything needed to turn the window-local logical coordinates in an accessibility snapshot into the screen
// coordinates UI Automation asks for and answers in.
//
// The root package refreshes it whenever the window moves, resizes or changes display scale, so a provider reads it
// fresh on every call rather than caching a conversion.
type UIAGeometry struct {
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
func (g UIAGeometry) ScreenRect(bounds geom.Rect) (left, top, width, height float64) {
	scaleX, scaleY := g.scale()
	return float64(g.Origin.X) + float64(bounds.X)*scaleX,
		float64(g.Origin.Y) + float64(bounds.Y)*scaleY,
		float64(bounds.Width) * scaleX,
		float64(bounds.Height) * scaleY
}

// WindowPoint converts a screen point in physical pixels into the window-local, top-left origin, logical coordinate
// space the snapshot's bounds live in. It is the inverse of ScreenRect's origin conversion, and is what a hit test
// from UI Automation goes through before it reaches the tree.
func (g UIAGeometry) WindowPoint(x, y float64) geom.Point {
	scaleX, scaleY := g.scale()
	return geom.NewPoint(float32((x-float64(g.Origin.X))/scaleX), float32((y-float64(g.Origin.Y))/scaleY))
}

// scale returns the horizontal and vertical scale to use, substituting one for a component that was never filled in.
func (g UIAGeometry) scale() (x, y float64) {
	x, y = float64(g.Scale.X), float64(g.Scale.Y)
	if x <= 0 {
		x = 1
	}
	if y <= 0 {
		y = 1
	}
	return x, y
}
