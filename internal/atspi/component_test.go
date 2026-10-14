// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// clippedWindow is the window the pointer tests publish.
const clippedWindow WindowKey = 5

// clippedTree is a list inside a view port half the list's height, which is the shape that tells [nodeObject.contains]
// and [nodeObject.getAccessibleAtPoint] apart. A node's Bounds are the whole of it, unclipped, so that an assistive
// technology knows how far to scroll to reveal it; what can be pointed at is the part of that the ancestors leave.
//
//	70 window "Ledger"        (0,0 200x100)  active
//	└─ 71 scroll area         (0,0 200x30)   the view port
//	   └─ 72 list             (0,0 200x60)
//	      ├─ 73 row "One"     (0,0 200x20)   wholly within the view port
//	      ├─ 74 row "Two"     (0,20 200x20)  the lower half is below the view port
//	      └─ 75 row "Three"   (0,40 200x20)  wholly below it, and marked offscreen
func clippedTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 70, Role: role.Window, Name: "Ledger", Focused: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{71},
		},
		&accessibility.Node{
			ID: 71, Parent: 70, Role: role.ScrollArea, Bounds: geom.NewRect(0, 0, 200, 30),
			Children: []accessibility.NodeID{72},
		},
		&accessibility.Node{
			ID: 72, Parent: 71, Role: role.List, Name: "Entries", Bounds: geom.NewRect(0, 0, 200, 60),
			Children: []accessibility.NodeID{73, 74, 75},
		},
		&accessibility.Node{
			ID: 73, Parent: 72, Role: role.ListItem, Name: "One", Selectable: true,
			Bounds: geom.NewRect(0, 0, 200, 20),
		},
		&accessibility.Node{
			ID: 74, Parent: 72, Role: role.ListItem, Name: "Two", Selectable: true,
			Bounds: geom.NewRect(0, 20, 200, 20),
		},
		&accessibility.Node{
			ID: 75, Parent: 72, Role: role.ListItem, Name: "Three", Selectable: true, Offscreen: true,
			Bounds: geom.NewRect(0, 40, 200, 20),
		},
	)
}

// TestContainsAgreesWithTheHitTest covers the two questions an assistive technology doing mouse review asks about one
// point: it asks a container what is under the pointer and then asks that object whether the point really is inside it.
// Two different answers leave it with nowhere to go, which is what reading a node's unclipped Bounds used to produce
// for every row scrolled out of its view port.
func TestContainsAgreesWithTheHitTest(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(clippedWindow, clippedTree(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	for _, one := range []struct {
		why      string
		node     accessibility.NodeID
		x        int32
		y        int32
		contains bool
		inList   bool
	}{
		{why: "the first row, wholly in view", node: 73, x: 100, y: 10, contains: true, inList: true},
		{why: "the visible part of the second row", node: 74, x: 100, y: 25, contains: true, inList: true},
		{why: "the part of the second row below the view port", node: 74, x: 100, y: 35},
		{why: "a row scrolled out of view entirely", node: 75, x: 100, y: 45},
		{why: "a point outside the window", node: 73, x: 400, y: 400},
		{why: "the window below its only child", node: 70, x: 100, y: 90, contains: true},
	} {
		c.Equal(one.contains, ta.one(NodePath(one.node), InterfaceComponent, "Contains", "iiu", one.x, one.y,
			uint32(CoordWindow)), one.why)
		// The two have to agree: a point the list places in one of its rows is a point that row claims, and a point it
		// places nowhere is a point no row claims.
		expected := nullReference()
		if one.inList {
			expected = nodeRef(one.node)
		}
		c.Equal(expected, ta.one(NodePath(72), InterfaceComponent, "GetAccessibleAtPoint", "iiu", one.x, one.y,
			uint32(CoordWindow)), "the list must place %s where the row that claims it is", one.why)
	}
}

// overlayWindow is the window the overlay tests publish.
const overlayWindow WindowKey = 11

// overlayTree is a small window with an open menu, shown in a window of its own, that extends beyond its right and
// bottom edges. The menu is an overlay, so what it holds is confined to it rather than to the window.
//
//	90 window "Small"         (0,0 100x100)  active
//	├─ 91 menu  [overlay]     (80,20 60x120)
//	│  ├─ 92 item "Open"      (80,20 60x20)
//	│  └─ 93 item "Close"     (80,120 60x20) below the window
//	└─ 94 button "OK"         (10,10 20x20)
func overlayTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 90, Role: role.Window, Name: "Small", Focused: true, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []accessibility.NodeID{91, 94},
		},
		&accessibility.Node{
			ID: 91, Parent: 90, Role: role.Menu, Name: "File", Overlay: true, Bounds: geom.NewRect(80, 20, 60, 120),
			Children: []accessibility.NodeID{92, 93},
		},
		&accessibility.Node{ID: 92, Parent: 91, Role: role.MenuItem, Name: "Open", Bounds: geom.NewRect(80, 20, 60, 20)},
		&accessibility.Node{
			ID: 93, Parent: 91, Role: role.MenuItem, Name: "Close", Bounds: geom.NewRect(80, 120, 60, 20),
		},
		&accessibility.Node{ID: 94, Parent: 90, Role: role.Button, Name: "OK", Bounds: geom.NewRect(10, 10, 20, 20)},
	)
}

// TestContainsReachesOverlaysBeyondTheWindow covers mouse review over the part of an open menu that extends beyond its
// window. Orca asks the window whether it contains the pointer before asking it what is there, so the window has to
// claim the points its overlays cover, and the menu's items have to claim the points they cover outside the window.
func TestContainsReachesOverlaysBeyondTheWindow(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	ta.Publish(overlayWindow, overlayTree(), nil, Geometry{Scale: geom.NewPoint(1, 1)})
	for _, one := range []struct {
		why      string
		x        int32
		y        int32
		contains bool
		at       accessibility.NodeID
	}{
		{why: "an item right of the window", x: 130, y: 30, contains: true, at: 92},
		{why: "an item below the window", x: 100, y: 130, contains: true, at: 93},
		{why: "the menu between its items, outside the window", x: 130, y: 80, contains: true, at: 91},
		{why: "the part of an item inside the window", x: 90, y: 30, contains: true, at: 92},
		{why: "a button inside the window", x: 20, y: 20, contains: true, at: 94},
		{why: "the window where it holds nothing", x: 50, y: 80, contains: true},
		{why: "a point outside both the window and the menu", x: 300, y: 300},
		{why: "a point beside the menu, below the window", x: 40, y: 130},
	} {
		c.Equal(one.contains, ta.one(NodePath(90), InterfaceComponent, "Contains", "iiu", one.x, one.y,
			uint32(CoordWindow)), "the window must claim %s", one.why)
		expected := nullReference()
		if one.at != 0 {
			expected = nodeRef(one.at)
		}
		c.Equal(expected, ta.one(NodePath(90), InterfaceComponent, "GetAccessibleAtPoint", "iiu", one.x, one.y,
			uint32(CoordWindow)), "the window must place %s", one.why)
	}
	c.Equal(true, ta.one(NodePath(93), InterfaceComponent, "Contains", "iiu", int32(100), int32(130),
		uint32(CoordWindow)), "an item below the window claims the point it covers")
	c.Equal(false, ta.one(NodePath(94), InterfaceComponent, "Contains", "iiu", int32(130), int32(30),
		uint32(CoordWindow)), "a button does not claim a point of the menu")
	c.Equal(false, ta.one(NodePath(91), InterfaceComponent, "Contains", "iiu", int32(20), int32(20),
		uint32(CoordWindow)), "the menu does not claim a point of the window it does not cover")
}
