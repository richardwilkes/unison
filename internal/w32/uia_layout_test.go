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
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// uiaFakeProvider mimics the head of a UIAProvider so that the this-pointer arithmetic can be checked without Windows,
// and with something in front of nothing and something behind the tables, which is the layout rule that matters: the
// table pointers are first and there are exactly uiaIfaceCount of them.
type uiaFakeProvider struct {
	vtbls [uiaIfaceCount]uintptr
	extra uint64
}

// TestUIAIfaceOrder verifies the interface indexes, since they are the one thing the virtual method tables, the
// this-pointer arithmetic and the QueryInterface table all have to agree about. The values are written out rather than
// derived so that inserting an interface in the middle of the list fails here instead of silently renumbering the
// tables.
func TestUIAIfaceOrder(t *testing.T) {
	c := check.New(t)
	c.Equal(uiaIface(0), uiaIfaceSimple)
	c.Equal(uiaIface(1), uiaIfaceFragment)
	c.Equal(uiaIface(2), uiaIfaceFragmentRoot)
	c.Equal(uiaIface(3), uiaIfaceAdviseEvents)
	c.Equal(uiaIface(4), uiaIfaceWindow)
	c.Equal(uiaIface(5), uiaIfaceInvoke)
	c.Equal(uiaIface(6), uiaIfaceToggle)
	c.Equal(uiaIface(7), uiaIfaceValue)
	c.Equal(uiaIface(8), uiaIfaceRangeValue)
	c.Equal(uiaIface(9), uiaIfaceSelection)
	c.Equal(uiaIface(10), uiaIfaceSelectionItem)
	c.Equal(uiaIface(11), uiaIfaceExpandCollapse)
	c.Equal(uiaIface(12), uiaIfaceScrollItem)
	c.Equal(uiaIface(13), uiaIfaceGrid)
	c.Equal(uiaIface(14), uiaIfaceGridItem)
	c.Equal(uiaIface(15), uiaIfaceTable)
	c.Equal(uiaIface(16), uiaIfaceTableItem)
	c.Equal(uiaIface(17), uiaIfaceCount)
}

// TestUIAIfaceSlots verifies each interface's table size against its method count, and that every interface has one.
func TestUIAIfaceSlots(t *testing.T) {
	c := check.New(t)
	c.Equal(3, uiaUnknownSlots)
	for iface := uiaIfaceSimple; iface < uiaIfaceCount; iface++ {
		c.True(uiaIfaceSlots[iface] > uiaUnknownSlots, "interface %d has no methods of its own", iface)
	}
	// Spot-checks of the interfaces whose method counts are easiest to get wrong: the widest one, and the two that
	// consist of nothing but a handful of getters.
	c.Equal(12, uiaIfaceSlots[uiaIfaceWindow])
	c.Equal(10, uiaIfaceSlots[uiaIfaceRangeValue])
	c.Equal(8, uiaIfaceSlots[uiaIfaceGridItem])
	c.Equal(4, uiaIfaceSlots[uiaIfaceInvoke])
	c.Equal(4, uiaIfaceSlots[uiaIfaceScrollItem])
}

// TestUIAIfaceOffset verifies the arithmetic every COM method of a provider depends on: the pointer a client holds for
// interface i is the address of the i-th table pointer within the provider, and subtracting the interface's offset from
// it has to land back on the provider itself.
func TestUIAIfaceOffset(t *testing.T) {
	c := check.New(t)
	c.Equal(uintptr(8), uiaIfacePointerSize)
	p := &uiaFakeProvider{extra: 1}
	base := uintptr(unsafe.Pointer(p))
	for iface := uiaIfaceSimple; iface < uiaIfaceCount; iface++ {
		c.Equal(uintptr(iface)*8, uiaIfaceOffset(iface))
		this := uintptr(unsafe.Pointer(&p.vtbls[iface]))
		c.Equal(base+uiaIfaceOffset(iface), this)
		c.Equal(base, this-uiaIfaceOffset(iface))
	}
	// The table array has to be the whole head of the object: anything after it must start beyond the last table.
	c.Equal(base+uiaIfaceOffset(uiaIfaceCount), uintptr(unsafe.Pointer(&p.extra)))
}

// TestUIAPatternIfaces verifies that every pattern this package implements has an interface behind it, that no two
// share one, and that no non-pattern interface claims a pattern. QueryInterface and GetPatternProvider both answer from
// this mapping, so a gap in it would mean a client could reach a pattern one way and not the other.
func TestUIAPatternIfaces(t *testing.T) {
	c := check.New(t)
	seen := make(map[uiaIface]bool)
	for _, info := range uiaPatternInfos {
		iface, ok := uiaIfaceForPattern(info.pattern)
		c.True(ok, "pattern %s has no interface", info.name)
		c.False(seen[iface], "pattern %s shares an interface", info.name)
		seen[iface] = true
		c.Equal(info.pattern, uiaPatternForIface(iface))
	}
	c.Equal(len(uiaPatternInfos), len(uiaPatternIfaces))
	for _, iface := range []uiaIface{uiaIfaceSimple, uiaIfaceFragment, uiaIfaceFragmentRoot, uiaIfaceAdviseEvents} {
		c.Equal(PatternSet(0), uiaPatternForIface(iface))
	}
	// The Window pattern is the one pattern whose interface is also one of the three the fragment root alone
	// implements, so it has to appear in both lists.
	c.Equal(PatternWindow, uiaPatternForIface(uiaIfaceWindow))
	_, ok := uiaIfaceForPattern(PatternSet(1) << 20)
	c.False(ok)
}

// TestUIAGeometryScreenRect verifies the conversion from a node's window-local logical bounds to the screen rectangle
// UI Automation asks for.
func TestUIAGeometryScreenRect(t *testing.T) {
	c := check.New(t)
	geometry := UIAGeometry{Origin: geom.NewPoint(100, 50), Scale: geom.NewPoint(2, 2)}
	left, top, width, height := geometry.ScreenRect(geom.NewRect(10, 20, 30, 40))
	c.Equal(120.0, left)
	c.Equal(90.0, top)
	c.Equal(60.0, width)
	c.Equal(80.0, height)

	// A geometry that was never filled in must still answer in logical units rather than with zeros or infinities.
	left, top, width, height = UIAGeometry{}.ScreenRect(geom.NewRect(10, 20, 30, 40))
	c.Equal(10.0, left)
	c.Equal(20.0, top)
	c.Equal(30.0, width)
	c.Equal(40.0, height)

	// Non-square scales and a non-zero origin must not be mixed up with each other.
	geometry = UIAGeometry{Origin: geom.NewPoint(-8, 4), Scale: geom.NewPoint(1.5, 3)}
	left, top, width, height = geometry.ScreenRect(geom.NewRect(2, 2, 2, 2))
	c.Equal(-5.0, left)
	c.Equal(10.0, top)
	c.Equal(3.0, width)
	c.Equal(6.0, height)
}

// TestUIAGeometryWindowPoint verifies the conversion a hit test from UI Automation goes through, which must be the
// exact inverse of the one a bounding rectangle goes through or a screen reader's mouse tracking picks the wrong
// element.
func TestUIAGeometryWindowPoint(t *testing.T) {
	c := check.New(t)
	geometry := UIAGeometry{Origin: geom.NewPoint(100, 50), Scale: geom.NewPoint(2, 2)}
	c.Equal(geom.NewPoint(10, 20), geometry.WindowPoint(120, 90))
	c.Equal(geom.NewPoint(0, 0), geometry.WindowPoint(100, 50))
	c.Equal(geom.NewPoint(-5, -5), geometry.WindowPoint(90, 40))
	c.Equal(geom.NewPoint(120, 90), UIAGeometry{}.WindowPoint(120, 90))

	// Round trip: the top-left corner of a node's screen rectangle must convert back to the node's own origin.
	geometry = UIAGeometry{Origin: geom.NewPoint(37, -11), Scale: geom.NewPoint(1.25, 2.5)}
	bounds := geom.NewRect(16, 24, 8, 8)
	left, top, _, _ := geometry.ScreenRect(bounds)
	c.Equal(bounds.Point, geometry.WindowPoint(left, top))
}
