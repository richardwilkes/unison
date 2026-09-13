// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// sampleTree builds the hierarchy every test in this file works over:
//
//	1 window                          (0,0 200x200)
//	├─ 2 group   [ignored]             (0,0 200x100)
//	│  ├─ 3 group   [ignored]          (0,0 200x50)
//	│  │  ├─ 4 button                  (0,0 50x20)
//	│  │  └─ 5 button                  (50,0 50x20)
//	│  └─ 6 label                      (0,50 100x20)
//	└─ 7 group                         (0,100 200x100)
//	   ├─ 8 button  [offscreen]        (0,100 60x30)
//	   ├─ 9 button                     (0,100 60x30)
//	   └─ 10 button                    (0,100 60x30)
//
// Nodes 8, 9 and 10 deliberately occupy exactly the same area: 8 is offscreen so hit testing must pass over it, and 9
// comes before 10 so it must win as the topmost of the two that remain.
func sampleTree() *accessibility.Tree {
	return newTree(4,
		&axNode{
			ID: 1, Role: role.Window, Name: windowName, Focused: true, Bounds: geom.NewRect(0, 0, 200, 200),
			Children: []axID{2, 7},
		},
		&axNode{
			ID: 2, Parent: 1, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []axID{3, 6},
		},
		&axNode{
			ID: 3, Parent: 2, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 50),
			Children: []axID{4, 5},
		},
		&axNode{ID: 4, Parent: 3, Role: role.Button, Name: "A", Bounds: geom.NewRect(0, 0, 50, 20)},
		&axNode{ID: 5, Parent: 3, Role: role.Button, Name: "B", Bounds: geom.NewRect(50, 0, 50, 20)},
		&axNode{ID: 6, Parent: 2, Role: role.Label, Name: "L", Bounds: geom.NewRect(0, 50, 100, 20)},
		&axNode{
			ID: 7, Parent: 1, Role: role.Group, Name: "Bottom", Bounds: geom.NewRect(0, 100, 200, 100),
			Children: []axID{8, 9, 10},
		},
		&axNode{
			ID: 8, Parent: 7, Role: role.Button, Name: "C", Offscreen: true, Bounds: geom.NewRect(0, 100, 60, 30),
		},
		&axNode{ID: 9, Parent: 7, Role: role.Button, Name: "D", Bounds: geom.NewRect(0, 100, 60, 30)},
		&axNode{ID: 10, Parent: 7, Role: role.Button, Name: "E", Bounds: geom.NewRect(0, 100, 60, 30)},
	)
}

func TestTreeNode(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()
	c.Equal("A", tree.Node(4).Name)
	c.Nil(tree.Node(0))
	c.Nil(tree.Node(999))
	var absent *accessibility.Tree
	c.Nil(absent.Node(1))
}

func TestTreeWalk(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()
	var visited []axID
	tree.Walk(func(n *axNode) bool {
		visited = append(visited, n.ID)
		return true
	})
	c.Equal([]axID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, visited)

	// Returning false ends the whole walk, not just the descent into that node's children.
	visited = nil
	tree.Walk(func(n *axNode) bool {
		visited = append(visited, n.ID)
		return n.ID != 3
	})
	c.Equal([]axID{1, 2, 3}, visited)

	// A child id with no node behind it is skipped rather than ending the walk.
	tree.Node(7).Children = []axID{8, 999, 9}
	visited = nil
	tree.Walk(func(n *axNode) bool {
		visited = append(visited, n.ID)
		return true
	})
	c.Equal([]axID{1, 2, 3, 4, 5, 6, 7, 8, 9}, visited)

	var absent *accessibility.Tree
	c.NotPanics(func() { absent.Walk(func(_ *axNode) bool { return true }) })
	c.NotPanics(func() { sampleTree().Walk(nil) })
}

func TestTreeHitTestConfinedByAncestors(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// A view port shows the top half of content that is twice its height; the row near the bottom of the content lies
	// outside the view port and must not be reachable through it, even though its own bounds contain the point.
	tree := &accessibility.Tree{
		Root: 1,
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			1: {ID: 1, Children: []accessibility.NodeID{2}, Bounds: geom.NewRect(0, 0, 100, 100)},
			2: {ID: 2, Parent: 1, Children: []accessibility.NodeID{3}, Bounds: geom.NewRect(0, 0, 100, 50)},
			3: {ID: 3, Parent: 2, Children: []accessibility.NodeID{4, 5}, Bounds: geom.NewRect(0, 0, 100, 200)},
			4: {ID: 4, Parent: 3, Bounds: geom.NewRect(0, 10, 100, 20)},
			5: {ID: 5, Parent: 3, Bounds: geom.NewRect(0, 60, 100, 20)},
		},
	}
	c.Equal(accessibility.NodeID(4), tree.HitTest(geom.NewPoint(50, 20)), "a row inside the view port is hit")
	c.Equal(accessibility.NodeID(1), tree.HitTest(geom.NewPoint(50, 70)),
		"a row outside the view port cannot be hit; the point lands on the root, which is all that is there")
	c.Equal(accessibility.NodeID(3), tree.HitTest(geom.NewPoint(50, 45)), "the content itself is hit between its rows")
}

func TestTreeHitTest(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()

	// Deepest wins: the point is inside the window and two ignored groups, but the button is what is reported.
	c.Equal(axID(4), tree.HitTest(geom.NewPoint(10, 10)))
	c.Equal(axID(5), tree.HitTest(geom.NewPoint(60, 10)))

	// Topmost wins and offscreen nodes are passed over: 8, 9 and 10 all cover this point.
	c.Equal(axID(9), tree.HitTest(geom.NewPoint(30, 110)))

	// A node with no child under the point reports itself, and an ignored node is reported like any other: it still
	// occupies space, and an adapter that needs a reportable element walks up from it.
	c.Equal(axID(7), tree.HitTest(geom.NewPoint(150, 160)))
	c.Equal(axID(2), tree.HitTest(geom.NewPoint(150, 60)))

	// Nothing outside the root.
	c.Equal(axID(0), tree.HitTest(geom.NewPoint(400, 400)))
	c.Equal(axID(0), tree.HitTest(geom.NewPoint(-1, -1)))

	// An offscreen node takes its descendants out of consideration with it.
	tree.Node(7).Offscreen = true
	c.Equal(axID(1), tree.HitTest(geom.NewPoint(30, 110)))

	var absent *accessibility.Tree
	c.Equal(axID(0), absent.HitTest(geom.NewPoint(0, 0)))
}

func TestTreeUnignoredChildren(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()

	// Two levels of ignored grouping collapse away, and the order the leaves were reached in is preserved.
	c.Equal([]axID{4, 5, 6, 7}, tree.UnignoredChildren(1))
	c.Equal([]axID{8, 9, 10}, tree.UnignoredChildren(7))
	c.Nil(tree.UnignoredChildren(4))
	c.Nil(tree.UnignoredChildren(999))

	// An ignored node with nothing but ignored, childless descendants contributes nothing.
	tree.Node(3).Children = nil
	tree.Node(6).Ignored = true
	c.Equal([]axID{7}, tree.UnignoredChildren(1))
}

func TestTreeUnignoredParent(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()
	c.Equal(axID(1), tree.UnignoredParent(4))
	c.Equal(axID(1), tree.UnignoredParent(6))
	c.Equal(axID(7), tree.UnignoredParent(9))
	c.Equal(axID(1), tree.UnignoredParent(7))
	c.Equal(axID(0), tree.UnignoredParent(1))
	c.Equal(axID(0), tree.UnignoredParent(999))

	// UnignoredParent and UnignoredChildren must agree: every child the one reports has the other as its parent.
	for _, id := range tree.UnignoredChildren(1) {
		c.Equal(axID(1), tree.UnignoredParent(id))
	}
}

func TestTreePath(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()
	c.Equal([]axID{1, 2, 3, 4}, tree.Path(4))
	c.Equal([]axID{1, 7, 9}, tree.Path(9))
	c.Equal([]axID{1}, tree.Path(1))
	c.Nil(tree.Path(999))
}

func TestTreePositionInSet(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := sampleTree()

	// The two buttons and the one label sit side by side under the root once the ignored groups are flattened, and each
	// role is counted on its own.
	pos, size := tree.PositionInSet(4)
	c.Equal(1, pos)
	c.Equal(2, size)
	pos, size = tree.PositionInSet(5)
	c.Equal(2, pos)
	c.Equal(2, size)
	pos, size = tree.PositionInSet(6)
	c.Equal(1, pos)
	c.Equal(1, size)
	pos, size = tree.PositionInSet(9)
	c.Equal(2, pos)
	c.Equal(3, size)

	// The root has no parent, an ignored node is not part of any set, and neither is a node that is not in the tree.
	for _, id := range []axID{1, 2, 999} {
		pos, size = tree.PositionInSet(id)
		c.Equal(0, pos, "node %d", id)
		c.Equal(0, size, "node %d", id)
	}
}

// TestTreeSurvivesCycles checks the depth guard: a tree whose Children links loop back on themselves is malformed, but
// the helpers must still return rather than recurse until the stack is gone.
func TestTreeSurvivesCycles(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := newTree(1,
		&axNode{
			ID: 1, Parent: 2, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []axID{2},
		},
		&axNode{
			ID: 2, Parent: 1, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []axID{1},
		},
	)
	c.NotPanics(func() {
		count := 0
		tree.Walk(func(_ *axNode) bool {
			count++
			return true
		})
		c.True(count > 0)
	})
	c.NotPanics(func() { tree.HitTest(geom.NewPoint(50, 50)) })
	c.NotPanics(func() { tree.UnignoredChildren(1) })
	c.NotPanics(func() { tree.UnignoredParent(2) })
	c.NotPanics(func() { tree.Path(2) })
}
