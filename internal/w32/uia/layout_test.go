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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// fakeProvider mimics the head of a Provider so that the this-pointer arithmetic can be checked without Windows,
// and with something in front of nothing and something behind the tables, which is the layout rule that matters: the
// table pointers are first and there are exactly ifaceCount of them.
type fakeProvider struct {
	vtbls [ifaceCount]uintptr
	extra uint64
}

// TestIfaceOrder verifies the interface indexes, since they are the one thing the virtual method tables, the
// this-pointer arithmetic and the QueryInterface table all have to agree about. The values are written out rather than
// derived so that inserting an interface in the middle of the list fails here instead of silently renumbering the
// tables.
func TestIfaceOrder(t *testing.T) {
	c := check.New(t)
	c.Equal(iface(0), ifaceSimple)
	c.Equal(iface(1), ifaceFragment)
	c.Equal(iface(2), ifaceFragmentRoot)
	c.Equal(iface(3), ifaceAdviseEvents)
	c.Equal(iface(4), ifaceWindow)
	c.Equal(iface(5), ifaceInvoke)
	c.Equal(iface(6), ifaceToggle)
	c.Equal(iface(7), ifaceValue)
	c.Equal(iface(8), ifaceRangeValue)
	c.Equal(iface(9), ifaceSelection)
	c.Equal(iface(10), ifaceSelectionItem)
	c.Equal(iface(11), ifaceExpandCollapse)
	c.Equal(iface(12), ifaceScrollItem)
	c.Equal(iface(13), ifaceGrid)
	c.Equal(iface(14), ifaceGridItem)
	c.Equal(iface(15), ifaceTable)
	c.Equal(iface(16), ifaceTableItem)
	c.Equal(iface(17), ifaceText)
	c.Equal(iface(18), ifaceTextChild)
	c.Equal(iface(19), ifaceCount)
}

// TestIfaceSlots verifies each interface's table size against its method count, and that every interface has one.
func TestIfaceSlots(t *testing.T) {
	c := check.New(t)
	c.Equal(3, unknownSlots)
	for which := ifaceSimple; which < ifaceCount; which++ {
		c.True(ifaceSlots[which] > unknownSlots, "interface %d has no methods of its own", which)
	}
	// Spot-checks of the interfaces whose method counts are easiest to get wrong: the widest one, and the two that
	// consist of nothing but a handful of getters.
	c.Equal(12, ifaceSlots[ifaceWindow])
	c.Equal(11, ifaceSlots[ifaceText])
	c.Equal(10, ifaceSlots[ifaceRangeValue])
	c.Equal(8, ifaceSlots[ifaceGridItem])
	c.Equal(5, ifaceSlots[ifaceTextChild])
	c.Equal(4, ifaceSlots[ifaceInvoke])
	c.Equal(4, ifaceSlots[ifaceScrollItem])

	// The text range is not one of a provider's interfaces but an object of its own, so its table is not in the list
	// above. It is ITextRangeProvider2's nineteen methods, the last of which is ShowContextMenu.
	c.Equal(22, textRangeSlots)
}

// TestIfaceOffset verifies the arithmetic every COM method of a provider depends on: the pointer a client holds for
// interface i is the address of the i-th table pointer within the provider, and subtracting the interface's offset from
// it has to land back on the provider itself.
func TestIfaceOffset(t *testing.T) {
	c := check.New(t)
	c.Equal(uintptr(8), ifacePointerSize)
	p := &fakeProvider{extra: 1}
	base := uintptr(unsafe.Pointer(p))
	for which := ifaceSimple; which < ifaceCount; which++ {
		c.Equal(uintptr(which)*8, ifaceOffset(which))
		this := uintptr(unsafe.Pointer(&p.vtbls[which]))
		c.Equal(base+ifaceOffset(which), this)
		c.Equal(base, this-ifaceOffset(which))
	}
	// The table array has to be the whole head of the object: anything after it must start beyond the last table.
	c.Equal(base+ifaceOffset(ifaceCount), uintptr(unsafe.Pointer(&p.extra)))
}

// TestPatternIfaces verifies that every pattern this package implements has an interface behind it, that no two
// share one except the documented pair that must, and that no non-pattern interface claims a pattern. QueryInterface
// and GetPatternProvider both answer from this mapping, so a gap in it would mean a client could reach a pattern one
// way and not the other.
//
// Text and Text2 are that pair: ITextProvider2 derives from ITextProvider, so one table answers both and a second would
// be the same six methods twice. The interface reports the Text pattern, which is the one an element is checked
// against, and the two are granted together — rolePatterns hands out both or neither — which is what makes the
// sharing safe. Nothing else may share, so every other pattern is still required to have an interface of its own.
func TestPatternIfaces(t *testing.T) {
	c := check.New(t)
	seen := make(map[iface]bool)
	for _, info := range patternInfos {
		which, ok := ifaceForPattern(info.pattern)
		c.True(ok, "pattern %s has no interface", info.name)
		if info.pattern == PatternText2 {
			c.Equal(ifaceText, which, "Text2 must share the Text interface")
			c.Equal(PatternText, patternForIface(which), "the shared interface reports the Text pattern")
			continue
		}
		c.False(seen[which], "pattern %s shares an interface", info.name)
		seen[which] = true
		c.Equal(info.pattern, patternForIface(which))
	}
	c.Equal(len(patternInfos), len(patternIfaces))
	for _, which := range []iface{ifaceSimple, ifaceFragment, ifaceFragmentRoot, ifaceAdviseEvents} {
		c.Equal(PatternSet(0), patternForIface(which))
	}
	// The Window pattern is the one pattern whose interface is also one of the three the fragment root alone
	// implements, so it has to appear in both lists.
	c.Equal(PatternWindow, patternForIface(ifaceWindow))
	_, ok := ifaceForPattern(PatternSet(1) << 20)
	c.False(ok)
}

// TestGeometryScreenRect verifies the conversion from a node's window-local logical bounds to the screen rectangle
// UI Automation asks for.
func TestGeometryScreenRect(t *testing.T) {
	c := check.New(t)
	geometry := Geometry{Origin: geom.NewPoint(100, 50), Scale: geom.NewPoint(2, 2)}
	left, top, width, height := geometry.ScreenRect(geom.NewRect(10, 20, 30, 40))
	c.Equal(120.0, left)
	c.Equal(90.0, top)
	c.Equal(60.0, width)
	c.Equal(80.0, height)

	// A geometry that was never filled in must still answer in logical units rather than with zeros or infinities.
	left, top, width, height = Geometry{}.ScreenRect(geom.NewRect(10, 20, 30, 40))
	c.Equal(10.0, left)
	c.Equal(20.0, top)
	c.Equal(30.0, width)
	c.Equal(40.0, height)

	// Non-square scales and a non-zero origin must not be mixed up with each other.
	geometry = Geometry{Origin: geom.NewPoint(-8, 4), Scale: geom.NewPoint(1.5, 3)}
	left, top, width, height = geometry.ScreenRect(geom.NewRect(2, 2, 2, 2))
	c.Equal(-5.0, left)
	c.Equal(10.0, top)
	c.Equal(3.0, width)
	c.Equal(6.0, height)
}

// TestGeometryWindowPoint verifies the conversion a hit test from UI Automation goes through, which must be the
// exact inverse of the one a bounding rectangle goes through or a screen reader's mouse tracking picks the wrong
// element.
func TestGeometryWindowPoint(t *testing.T) {
	c := check.New(t)
	geometry := Geometry{Origin: geom.NewPoint(100, 50), Scale: geom.NewPoint(2, 2)}
	c.Equal(geom.NewPoint(10, 20), geometry.WindowPoint(120, 90))
	c.Equal(geom.NewPoint(0, 0), geometry.WindowPoint(100, 50))
	c.Equal(geom.NewPoint(-5, -5), geometry.WindowPoint(90, 40))
	c.Equal(geom.NewPoint(120, 90), Geometry{}.WindowPoint(120, 90))

	// Round trip: the top-left corner of a node's screen rectangle must convert back to the node's own origin.
	geometry = Geometry{Origin: geom.NewPoint(37, -11), Scale: geom.NewPoint(1.25, 2.5)}
	bounds := geom.NewRect(16, 24, 8, 8)
	left, top, _, _ := geometry.ScreenRect(bounds)
	c.Equal(bounds.Point, geometry.WindowPoint(left, top))
}
