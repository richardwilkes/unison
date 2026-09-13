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
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// nodePathPrefix is what the decimal id of a node is appended to in order to make its object path.
const nodePathPrefix = string(AccessiblePrefix) + "/"

// NodePath returns the object path that the node with the given id is exported at. Ids come from one process-wide
// counter, so these paths are unique across every window without the window having to appear in them.
func NodePath(id accessibility.NodeID) dbus.ObjectPath {
	return dbus.ObjectPath(nodePathPrefix + strconv.FormatUint(uint64(id), 10))
}

// ParseNodePath returns the id of the node a path refers to. ok is false for anything else, which includes [RootPath],
// [CachePath] and [NullPath], since none of them names a node.
func ParseNodePath(path dbus.ObjectPath) (id accessibility.NodeID, ok bool) {
	rest, found := strings.CutPrefix(string(path), nodePathPrefix)
	if !found {
		return 0, false
	}
	value, err := strconv.ParseUint(rest, 10, 64)
	if err != nil || value == 0 {
		return 0, false
	}
	return accessibility.NodeID(value), true
}

// relation is one kind of relation together with the nodes it points at.
type relation struct {
	targets []accessibility.NodeID
	kind    Relation
}

// relationPairs lists the relations a node holds directly, each with the relation it implies in the other direction.
// Both directions have to be reported: an assistive technology asks a control what labels it, and asks a label what it
// labels, and expects to be told.
var relationPairs = []struct {
	targets func(n *accessibility.Node) []accessibility.NodeID
	forward Relation
	reverse Relation
}{
	{
		targets: func(n *accessibility.Node) []accessibility.NodeID { return n.LabeledBy },
		forward: RelationLabelledBy,
		reverse: RelationLabelFor,
	},
	{
		targets: func(n *accessibility.Node) []accessibility.NodeID { return n.DescribedBy },
		forward: RelationDescribedBy,
		reverse: RelationDescriptionFor,
	},
	{
		targets: func(n *accessibility.Node) []accessibility.NodeID { return n.Controls },
		forward: RelationControllerFor,
		reverse: RelationControlledBy,
	},
}

// buildRelations returns the relation set of every node in the tree that has one, keyed by node id and ordered by
// relation number so that the reply is the same for the same tree. It is built once per published snapshot, since the
// reverse direction of every relation would otherwise require a walk of the whole tree to answer.
func buildRelations(t *accessibility.Tree) map[accessibility.NodeID][]relation {
	gathered := make(map[accessibility.NodeID]map[Relation][]accessibility.NodeID)
	add := func(from accessibility.NodeID, kind Relation, to accessibility.NodeID) {
		byKind := gathered[from]
		if byKind == nil {
			byKind = make(map[Relation][]accessibility.NodeID)
			gathered[from] = byKind
		}
		if !slices.Contains(byKind[kind], to) {
			byKind[kind] = append(byKind[kind], to)
		}
	}
	// The walk is in pre-order, so the targets of the reverse relations come out in tree order rather than in the
	// order the map happens to hand out.
	t.Walk(func(n *accessibility.Node) bool {
		if n.Ignored {
			// An ignored node has no object, so neither end of a relation involving one can be reported.
			return true
		}
		for _, pair := range relationPairs {
			for _, target := range pair.targets(n) {
				if other := t.Node(target); target == n.ID || other == nil || other.Ignored {
					continue
				}
				add(n.ID, pair.forward, target)
				add(target, pair.reverse, n.ID)
			}
		}
		return true
	})
	if len(gathered) == 0 {
		return nil
	}
	result := make(map[accessibility.NodeID][]relation, len(gathered))
	for id, byKind := range gathered {
		relations := make([]relation, 0, len(byKind))
		for kind, targets := range byKind {
			relations = append(relations, relation{kind: kind, targets: targets})
		}
		slices.SortFunc(relations, func(a, b relation) int { return int(a.kind) - int(b.kind) })
		result[id] = relations
	}
	return result
}

// windowData is everything that answering a query about one window needs: the published tree, the indexes built over
// it, and the geometry that turns its logical coordinates into the physical pixels AT-SPI works in. It is replaced as a
// whole and never modified, so a handler that has loaded the pointer to it may use it without any further locking, even
// while the user interface thread publishes the next snapshot.
type windowData struct {
	tree      *accessibility.Tree
	children  map[accessibility.NodeID][]accessibility.NodeID
	indexes   map[accessibility.NodeID]int
	relations map[accessibility.NodeID][]relation
	geometry  Geometry
}

// newWindowData indexes a tree so that the questions AT-SPI asks most often — the children of a node, where a node sits
// among its siblings, and what a node is related to — can be answered without walking anything.
func newWindowData(t *accessibility.Tree, g Geometry) *windowData {
	d := &windowData{
		tree:      t,
		children:  make(map[accessibility.NodeID][]accessibility.NodeID),
		indexes:   make(map[accessibility.NodeID]int),
		relations: buildRelations(t),
		geometry:  g,
	}
	t.Walk(func(n *accessibility.Node) bool {
		if n.Ignored || len(n.Children) == 0 {
			// An ignored node is not reported at all: its children are spliced into the list of the nearest ancestor
			// that is reported, which is where they are indexed.
			return true
		}
		children := t.UnignoredChildren(n.ID)
		if len(children) == 0 {
			return true
		}
		d.children[n.ID] = children
		for i, child := range children {
			d.indexes[child] = i
		}
		return true
	})
	return d
}

// withGeometry returns a copy of the data that converts coordinates with a different geometry. The indexes are shared
// rather than rebuilt, since a window that has moved, been resized or changed screens still holds the same tree.
func (d *windowData) withGeometry(g Geometry) *windowData {
	copied := *d
	copied.geometry = g
	return &copied
}

// node returns the node with the given id, or nil if this window does not hold one.
func (d *windowData) node(id accessibility.NodeID) *accessibility.Node {
	return d.tree.Node(id)
}

// root returns the window's root node, which is the window itself.
func (d *windowData) root() *accessibility.Node {
	return d.tree.Node(d.tree.Root)
}

// active returns true if this window is the active one. The root of a tree reports that through its Focused field.
func (d *windowData) active() bool {
	if r := d.root(); r != nil {
		return r.Focused
	}
	return false
}

// unignoredChildren returns the ids of the children of a node that are reported to an assistive technology.
func (d *windowData) unignoredChildren(id accessibility.NodeID) []accessibility.NodeID {
	return d.children[id]
}

// indexInParent returns the position of a node among the reported children of its reported parent, or -1 when it has no
// parent, which is the case for the window's root.
func (d *windowData) indexInParent(id accessibility.NodeID) int {
	if index, exists := d.indexes[id]; exists {
		return index
	}
	return -1
}

// parent returns the id of the nearest ancestor of a node that is reported to an assistive technology, or zero if there
// is none.
func (d *windowData) parent(id accessibility.NodeID) accessibility.NodeID {
	return d.tree.UnignoredParent(id)
}

// relationsOf returns the relation set of a node, which is empty for most of them.
func (d *windowData) relationsOf(id accessibility.NodeID) []relation {
	return d.relations[id]
}

// reachableBounds returns the part of a node that can actually be pointed at: its own Bounds confined to those of every
// ancestor, which is the clipping [accessibility.Tree.HitTest] applies and the clipping the drawing applies. ok is
// false when nothing of the node can be reached, either because it or an ancestor is scrolled or clipped entirely out
// of view or because the clipping leaves no area at all.
//
// Node bounds alone will not do. A node's Bounds are the whole of it, unclipped, so that an assistive technology can
// tell how far to scroll to reveal it, which means a row scrolled out of its table's view port still reports the area
// it would occupy. Answering a question about a point from those bounds alone would have the row claim a point that
// belongs to whatever is drawn over it.
func (d *windowData) reachableBounds(n *accessibility.Node) (bounds geom.Rect, ok bool) {
	path := d.tree.Path(n.ID)
	if len(path) == 0 {
		return geom.Rect{}, false
	}
	for i, id := range path {
		ancestor := d.node(id)
		if ancestor == nil || ancestor.Offscreen {
			return geom.Rect{}, false
		}
		if i == 0 {
			bounds = ancestor.Bounds
			continue
		}
		bounds = bounds.Intersect(ancestor.Bounds)
	}
	return bounds, !bounds.Empty()
}

// Geometry conversion. A node's Bounds are window-local, top-left origin, logical units. AT-SPI works in physical
// pixels, either relative to the screen, to the window's content area, or to the object's parent.

// effectiveScale returns the geometry's scale, substituting one for an axis that has not been set. A window that has
// published a tree before its geometry is known would otherwise report every node as a point.
func (g Geometry) effectiveScale() geom.Point {
	scale := g.Scale
	if scale.X <= 0 {
		scale.X = 1
	}
	if scale.Y <= 0 {
		scale.Y = 1
	}
	return scale
}

// originFor returns the point, in physical pixels, that a node's coordinates are relative to in the given coordinate
// space. A node with no reported parent has nothing to be relative to in parent coordinates, so it is given the
// screen's origin instead: its parent is the desktop, and the desktop's children are measured from the screen.
func (d *windowData) originFor(n *accessibility.Node, coord CoordType) geom.Point {
	switch coord {
	case CoordWindow:
		return geom.Point{}
	case CoordParent:
		if parent := d.node(d.parent(n.ID)); parent != nil {
			scale := d.geometry.effectiveScale()
			return geom.Point{
				X: -parent.Bounds.X * scale.X,
				Y: -parent.Bounds.Y * scale.Y,
			}
		}
		return d.geometry.Origin
	default:
		return d.geometry.Origin
	}
}

// extents returns a node's area in physical pixels, in the given coordinate space.
func (d *windowData) extents(n *accessibility.Node, coord CoordType) (x, y, w, h int32) {
	origin := d.originFor(n, coord)
	scale := d.geometry.effectiveScale()
	return pixels(origin.X + n.Bounds.X*scale.X), pixels(origin.Y + n.Bounds.Y*scale.Y),
		pixels(n.Bounds.Width * scale.X), pixels(n.Bounds.Height * scale.Y)
}

// logicalPoint converts a point in the given coordinate space, in physical pixels, into the window-local logical point
// that node bounds are expressed in. n is the node the coordinates were given relative to, which only matters for
// parent coordinates.
func (d *windowData) logicalPoint(n *accessibility.Node, x, y int32, coord CoordType) geom.Point {
	origin := d.originFor(n, coord)
	scale := d.geometry.effectiveScale()
	return geom.Point{
		X: (float32(x) - origin.X) / scale.X,
		Y: (float32(y) - origin.Y) / scale.Y,
	}
}

// pixels rounds a physical pixel coordinate to the nearest whole one.
func pixels(v float32) int32 {
	return int32(math.Round(float64(v)))
}

// windowState is one window's slot in an [Adapter]. The identity of the slot is what matters — it is what the index
// from node to window points at, and its position in the adapter's order is the window's position among the
// application's children — so the only thing in it is the snapshot. That pointer is swapped rather than modified, so a
// query being answered while the user interface thread publishes the next snapshot goes on reading the one it started
// with.
type windowState struct {
	data atomic.Pointer[windowData]
}
