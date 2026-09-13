// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility

import "github.com/richardwilkes/toolbox/v2/geom"

// maxTreeDepth bounds how far the tree helpers will descend. Real hierarchies are orders of magnitude shallower than
// this, so the limit never comes into play; it exists only so that a malformed tree — one whose Children links form a
// cycle — cannot make these functions recurse until the stack is exhausted.
const maxTreeDepth = 512

// Tree is an immutable snapshot of one window's accessibility hierarchy. Nothing in a tree, including the nodes the
// Nodes map points at, is modified after the tree has been published, so a platform adapter may read it from whatever
// thread its assistive technology calls in on.
type Tree struct {
	// Nodes holds every node in the tree, keyed by its ID.
	Nodes map[NodeID]*Node
	// Root is the id of the window node every other node descends from.
	Root NodeID
	// Focus is the id of the node holding the keyboard focus, or zero when nothing in the window has it.
	Focus NodeID
	// Generation counts the snapshots taken of this window, starting at one. It increases by one per published tree,
	// which lets an adapter tell a stale tree from a current one.
	Generation uint64
}

// Node returns the node with the given id, or nil if the tree does not hold one. It is safe to call on a nil tree.
func (t *Tree) Node(id NodeID) *Node {
	if t == nil {
		return nil
	}
	return t.Nodes[id]
}

// Walk calls fn for every node reachable from Root, in pre-order: a node is visited before its children, and children
// are visited in the order they appear in Children. Returning false from fn stops the walk entirely, so a search can
// leave early. Child ids that the tree has no node for are skipped. It is safe to call on a nil tree.
func (t *Tree) Walk(fn func(n *Node) bool) {
	if t == nil || fn == nil {
		return
	}
	t.walk(t.Root, 0, fn)
}

// walk implements Walk from one starting node, reporting whether the traversal should continue.
func (t *Tree) walk(id NodeID, depth int, fn func(n *Node) bool) bool {
	n := t.Nodes[id]
	if n == nil {
		return true
	}
	if !fn(n) {
		return false
	}
	if depth >= maxTreeDepth {
		return true
	}
	for _, child := range n.Children {
		if !t.walk(child, depth+1, fn) {
			return false
		}
	}
	return true
}

// HitTest returns the id of the deepest node under pt, or zero when nothing is. Among overlapping siblings the earlier
// one wins, since children are ordered with the topmost first; Offscreen nodes are skipped. A node's Bounds are not
// clipped by its ancestors, but what it draws is, so a point counts as inside a node only when it is also inside every
// ancestor: the rows of a table that has been scrolled partly out of its view port cannot be hit through the widgets
// that cover them.
func (t *Tree) HitTest(pt geom.Point) NodeID {
	if t == nil {
		return 0
	}
	root := t.Nodes[t.Root]
	if root == nil {
		return 0
	}
	return t.hitTest(t.Root, pt, root.Bounds, 0)
}

// hitTest implements HitTest from one starting node, returning zero when nothing at or below it contains pt. clip is
// the intersection of the bounds of every node above this one.
func (t *Tree) hitTest(id NodeID, pt geom.Point, clip geom.Rect, depth int) NodeID {
	n := t.Nodes[id]
	if n == nil || n.Offscreen {
		return 0
	}
	clip = clip.Intersect(n.Bounds)
	if !pt.In(clip) {
		return 0
	}
	if depth < maxTreeDepth {
		for _, child := range n.Children {
			if hit := t.hitTest(child, pt, clip, depth+1); hit != 0 {
				return hit
			}
		}
	}
	return id
}

// UnignoredChildren returns the ids of the children an assistive technology should see beneath the node with the given
// id. Each Ignored child is replaced by its own unignored children, recursively, so a control buried under several
// layers of anonymous grouping panels appears as a direct child. The result is nil when there is nothing to report.
func (t *Tree) UnignoredChildren(id NodeID) []NodeID {
	n := t.Node(id)
	if n == nil || len(n.Children) == 0 {
		return nil
	}
	return t.appendUnignoredChildren(nil, n, 0)
}

// appendUnignoredChildren appends the unignored children of n to ids, splicing in the children of any Ignored child.
func (t *Tree) appendUnignoredChildren(ids []NodeID, n *Node, depth int) []NodeID {
	for _, childID := range n.Children {
		child := t.Nodes[childID]
		switch {
		case child == nil:
		case !child.Ignored:
			ids = append(ids, childID)
		case depth < maxTreeDepth:
			ids = t.appendUnignoredChildren(ids, child, depth+1)
		}
	}
	return ids
}

// UnignoredParent returns the id of the nearest ancestor of the node with the given id that is not Ignored, or zero if
// there is none. It is the inverse of UnignoredChildren: for any id that UnignoredChildren(p) returns, this returns p.
func (t *Tree) UnignoredParent(id NodeID) NodeID {
	n := t.Node(id)
	for i := 0; n != nil && i < maxTreeDepth; i++ {
		parent := t.Nodes[n.Parent]
		if parent == nil {
			return 0
		}
		if !parent.Ignored {
			return parent.ID
		}
		n = parent
	}
	return 0
}

// Path returns the ids of the node with the given id and each of its ancestors, ordered from Root down to and including
// that node. Ignored nodes are included, since the path describes the tree's actual shape. It returns nil when the tree
// holds no such node.
func (t *Tree) Path(id NodeID) []NodeID {
	n := t.Node(id)
	if n == nil {
		return nil
	}
	path := []NodeID{n.ID}
	for i := 0; i < maxTreeDepth; i++ {
		parent := t.Nodes[n.Parent]
		if parent == nil {
			break
		}
		path = append(path, parent.ID)
		n = parent
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// PositionInSet returns the one-based position of the node with the given id among the unignored children of its
// unignored parent that share its Role, together with how many such siblings there are. Restricting the count to one
// role is what makes the numbers useful: a list whose items are interleaved with separators still reports its items as
// one through n. Both results are zero when the node is not in the tree, is Ignored, or has no unignored parent.
func (t *Tree) PositionInSet(id NodeID) (pos, size int) {
	n := t.Node(id)
	if n == nil || n.Ignored {
		return 0, 0
	}
	for _, siblingID := range t.UnignoredChildren(t.UnignoredParent(id)) {
		if sibling := t.Nodes[siblingID]; sibling != nil && sibling.Role == n.Role {
			size++
			if siblingID == id {
				pos = size
			}
		}
	}
	if pos == 0 {
		return 0, 0
	}
	return pos, size
}
