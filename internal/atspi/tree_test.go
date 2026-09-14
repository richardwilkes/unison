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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// treeOf assembles a tree from its nodes. The first node is the root, and the tree's focus is whichever node says it is
// focused.
func treeOf(generation uint64, nodes ...*accessibility.Node) *accessibility.Tree {
	t := &accessibility.Tree{
		Nodes:      make(map[accessibility.NodeID]*accessibility.Node, len(nodes)),
		Generation: generation,
	}
	for i, n := range nodes {
		if i == 0 {
			t.Root = n.ID
		}
		if n.Focused && n.ID != t.Root {
			t.Focus = n.ID
		}
		t.Nodes[n.ID] = n
	}
	return t
}

// sampleTree is the hierarchy the tree tests work over:
//
//	1 window                       (0,0 200x150)
//	├─ 2 group      [ignored]       (0,0 200x60)
//	│  ├─ 3 label "Name:"           (10,10 40x20)
//	│  └─ 4 text field              (60,10 100x20)   labeled by 3, controls 5
//	└─ 5 list                       (0,60 200x90)
//	   ├─ 6 list item "One"         (0,60 200x20)    selected
//	   └─ 7 list item "Two"         (0,80 200x20)
func sampleTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Sample", Focused: true, Bounds: geom.NewRect(0, 0, 200, 150),
			Children: []accessibility.NodeID{2, 5},
		},
		&accessibility.Node{
			ID: 2, Parent: 1, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 60),
			Children: []accessibility.NodeID{3, 4},
		},
		&accessibility.Node{
			ID: 3, Parent: 2, Role: role.Label, Name: testLabelName, Bounds: geom.NewRect(10, 10, 40, 20),
		},
		&accessibility.Node{
			ID: 4, Parent: 2, Role: role.TextField, Value: testFieldValue, Bounds: geom.NewRect(60, 10, 100, 20),
			LabeledBy: []accessibility.NodeID{3}, Controls: []accessibility.NodeID{5},
		},
		&accessibility.Node{
			ID: 5, Parent: 1, Role: role.List, Bounds: geom.NewRect(0, 60, 200, 90),
			Children: []accessibility.NodeID{6, 7},
		},
		&accessibility.Node{
			ID: 6, Parent: 5, Role: role.ListItem, Name: "One", Selectable: true, Selected: true,
			Bounds: geom.NewRect(0, 60, 200, 20),
		},
		&accessibility.Node{
			ID: 7, Parent: 5, Role: role.ListItem, Name: "Two", Selectable: true,
			Bounds: geom.NewRect(0, 80, 200, 20),
		},
	)
}

// sampleGeometry puts the window's content area at (100,50) on a screen with two physical pixels per logical unit.
func sampleGeometry() Geometry {
	return Geometry{Origin: geom.NewPoint(100, 50), Scale: geom.NewPoint(2, 2)}
}

func TestNodePath(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(dbus.ObjectPath("/org/a11y/atspi/accessible/1"), NodePath(1))
	c.Equal(dbus.ObjectPath("/org/a11y/atspi/accessible/18446744073709551615"), NodePath(^accessibility.NodeID(0)))
	for _, id := range []accessibility.NodeID{1, 7, 42, 1 << 40, ^accessibility.NodeID(0)} {
		parsed, ok := ParseNodePath(NodePath(id))
		c.True(ok, "%d must parse back", id)
		c.Equal(id, parsed)
	}
	c.NoError(NodePath(12345).Validate())
}

func TestParseNodePathRejectsEverythingElse(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, path := range []dbus.ObjectPath{
		RootPath,
		CachePath,
		NullPath,
		AccessiblePrefix,
		"/org/a11y/atspi/accessible/",
		"/org/a11y/atspi/accessible/0",
		"/org/a11y/atspi/accessible/12x",
		"/org/a11y/atspi/accessible/-1",
		"/org/a11y/atspi/accessible/1/2",
		"/",
		"",
	} {
		_, ok := ParseNodePath(path)
		c.False(ok, "%s must not parse as a node path", path)
	}
}

func TestWindowDataIndexesTheReportedChildren(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(sampleTree(), sampleGeometry())
	// The ignored group is replaced by its own children, so the window reports three of them.
	c.Equal([]accessibility.NodeID{3, 4, 5}, d.unignoredChildren(1))
	c.Equal([]accessibility.NodeID{6, 7}, d.unignoredChildren(5))
	c.Nil(d.unignoredChildren(2), "an ignored node reports no children of its own")
	c.Nil(d.unignoredChildren(3))
	c.Equal(0, d.indexInParent(3))
	c.Equal(1, d.indexInParent(4))
	c.Equal(2, d.indexInParent(5))
	c.Equal(0, d.indexInParent(6))
	c.Equal(1, d.indexInParent(7))
	c.Equal(-1, d.indexInParent(1), "the window's root has no parent within the window")
	c.Equal(accessibility.NodeID(1), d.parent(4), "the ignored group is passed over")
	c.Equal(accessibility.NodeID(5), d.parent(6))
	c.Equal(accessibility.NodeID(0), d.parent(1))
	c.True(d.active(), "the sample window is the active one")
	c.Equal(accessibility.NodeID(1), d.root().ID)
}

func TestWindowDataBuildsBothDirectionsOfEveryRelation(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(sampleTree(), sampleGeometry())
	c.Equal([]relation{
		{kind: RelationLabelledBy, targets: []accessibility.NodeID{3}},
		{kind: RelationControllerFor, targets: []accessibility.NodeID{5}},
	}, d.relationsOf(4))
	c.Equal([]relation{{kind: RelationLabelFor, targets: []accessibility.NodeID{4}}}, d.relationsOf(3))
	c.Equal([]relation{{kind: RelationControlledBy, targets: []accessibility.NodeID{4}}}, d.relationsOf(5))
	c.Nil(d.relationsOf(6))
}

func TestWindowDataRelationsAreOrderedAndFiltered(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(treeOf(1,
		&accessibility.Node{
			ID: 1, Role: role.Window, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []accessibility.NodeID{2, 3, 4},
		},
		&accessibility.Node{ID: 2, Parent: 1, Role: role.Label, Name: "Label"},
		&accessibility.Node{ID: 3, Parent: 1, Role: role.Group, Ignored: true},
		&accessibility.Node{
			ID: 4, Parent: 1, Role: role.TextField,
			// The describing node is ignored and the controlled one is not in the tree at all, so neither can be
			// reported; pointing at itself is meaningless and is dropped too.
			LabeledBy:   []accessibility.NodeID{2, 2},
			DescribedBy: []accessibility.NodeID{3},
			Controls:    []accessibility.NodeID{99, 4},
		},
	), Geometry{})
	c.Equal([]relation{{kind: RelationLabelledBy, targets: []accessibility.NodeID{2}}}, d.relationsOf(4))
	c.Equal([]relation{{kind: RelationLabelFor, targets: []accessibility.NodeID{4}}}, d.relationsOf(2))
	c.Nil(d.relationsOf(3))
	c.Nil(d.relationsOf(99))
}

func TestWindowDataRelationsFromAnIgnoredNodeAreDropped(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(treeOf(1,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 3}},
		&accessibility.Node{ID: 2, Parent: 1, Role: role.Label},
		&accessibility.Node{
			ID: 3, Parent: 1, Role: role.Group, Ignored: true,
			LabeledBy: []accessibility.NodeID{2},
		},
	), Geometry{})
	c.Nil(d.relationsOf(2))
	c.Nil(d.relationsOf(3))
}

func TestWindowDataExtents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(sampleTree(), sampleGeometry())
	for _, one := range []struct {
		name  string
		node  accessibility.NodeID
		coord CoordType
		x     int32
		y     int32
		w     int32
		h     int32
	}{
		{name: "the text field on the screen", node: 4, coord: CoordScreen, x: 220, y: 70, w: 200, h: 40},
		{name: "the text field in the window", node: 4, coord: CoordWindow, x: 120, y: 20, w: 200, h: 40},
		{
			name: "the text field within its parent, which is the window, since the group is ignored",
			node: 4, coord: CoordParent, x: 120, y: 20, w: 200, h: 40,
		},
		{name: "the first row on the screen", node: 6, coord: CoordScreen, x: 100, y: 170, w: 400, h: 40},
		{name: "the first row in the window", node: 6, coord: CoordWindow, x: 0, y: 120, w: 400, h: 40},
		{name: "the first row within the list", node: 6, coord: CoordParent, x: 0, y: 0, w: 400, h: 40},
		{name: "the second row within the list", node: 7, coord: CoordParent, x: 0, y: 40, w: 400, h: 40},
		{name: "the window on the screen", node: 1, coord: CoordScreen, x: 100, y: 50, w: 400, h: 300},
		{name: "the window in the window", node: 1, coord: CoordWindow, x: 0, y: 0, w: 400, h: 300},
		{
			name: "the window within its parent, which is the desktop, so the screen",
			node: 1, coord: CoordParent, x: 100, y: 50, w: 400, h: 300,
		},
	} {
		x, y, w, h := d.extents(d.node(one.node), one.coord)
		c.Equal([]int32{one.x, one.y, one.w, one.h}, []int32{x, y, w, h}, one.name)
	}
}

func TestWindowDataExtentsWithoutAGeometry(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// A window that has published a tree before anything has told the adapter where it is must still report the sizes
	// it knows, rather than turning every node into a point.
	d := newWindowData(sampleTree(), Geometry{})
	x, y, w, h := d.extents(d.node(4), CoordScreen)
	c.Equal([]int32{60, 10, 100, 20}, []int32{x, y, w, h})
}

func TestWindowDataExtentsRound(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(treeOf(1, &accessibility.Node{
		ID: 1, Role: role.Window, Bounds: geom.NewRect(0.4, 0.6, 10.5, 10.5),
	}), Geometry{Scale: geom.NewPoint(1.5, 1.5)})
	x, y, w, h := d.extents(d.node(1), CoordWindow)
	c.Equal([]int32{1, 1, 16, 16}, []int32{x, y, w, h})
}

func TestWindowDataLogicalPoint(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(sampleTree(), sampleGeometry())
	field := d.node(4)
	c.Equal(geom.NewPoint(60, 10), d.logicalPoint(field, 220, 70, CoordScreen))
	c.Equal(geom.NewPoint(60, 10), d.logicalPoint(field, 120, 20, CoordWindow))
	c.Equal(geom.NewPoint(60, 10), d.logicalPoint(field, 120, 20, CoordParent))
	row := d.node(6)
	c.Equal(geom.NewPoint(0, 60), d.logicalPoint(row, 0, 0, CoordParent))
	c.Equal(geom.NewPoint(0, 60), d.logicalPoint(row, 100, 170, CoordScreen))
	// Every conversion is the inverse of the one extents makes.
	for _, coord := range []CoordType{CoordScreen, CoordWindow, CoordParent} {
		for _, id := range []accessibility.NodeID{1, 4, 6, 7} {
			n := d.node(id)
			x, y, _, _ := d.extents(n, coord)
			c.Equal(n.Bounds.Point, d.logicalPoint(n, x, y, coord), "node %d in space %d", id, coord)
		}
	}
}

func TestWindowDataWithGeometry(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	d := newWindowData(sampleTree(), sampleGeometry())
	moved := d.withGeometry(Geometry{Origin: geom.NewPoint(0, 0), Scale: geom.NewPoint(1, 1)})
	x, y, w, h := moved.extents(moved.node(4), CoordScreen)
	c.Equal([]int32{60, 10, 100, 20}, []int32{x, y, w, h})
	c.Equal(d.tree, moved.tree, "the tree is shared rather than rebuilt")
	original, _, _, _ := d.extents(d.node(4), CoordScreen)
	c.Equal(int32(220), original, "the original is untouched")
}
