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

// spanRef says that one node occupies a range of another node's text: which node's text it is, and where within it the
// range sits. It is what org.a11y.atspi.Hyperlink's StartIndex and EndIndex report, and the text a link sits in is the
// only place those offsets mean anything, so both ends of the relationship are held together.
type spanRef struct {
	container accessibility.NodeID
	start     int
	end       int
}

// buildSpanIndex returns, for every node that another node's text says occupies part of it, where that range is. It is
// built once per published snapshot, since answering it from the tree would otherwise mean walking every text-bearing
// node's spans to find out whether one names this object.
//
// Only a node's own text is read. A document composes the text of everything beneath it into one stream with a span per
// node, and that stream is never exposed on AT-SPI — the document object is given no org.a11y.atspi.Text at all, since
// Orca reads a document by walking the objects within it and would otherwise read the whole of it twice — so the
// offsets that mean anything here are the block-local ones. [accessibility.Node] keeps a document's own Text nil, so a
// document contributes nothing without anything having to check.
func buildSpanIndex(t *accessibility.Tree) map[accessibility.NodeID]spanRef {
	var spans map[accessibility.NodeID]spanRef
	t.Walk(func(n *accessibility.Node) bool {
		if n.Ignored || n.Text == nil {
			return true
		}
		for _, span := range n.Text.Spans {
			target := t.Node(span.Node)
			if target == nil || target.Ignored || span.Node == n.ID {
				// A span naming a node that has no object of its own, or naming the very node whose text it is, has
				// nothing an assistive technology could be pointed at.
				continue
			}
			if _, exists := spans[span.Node]; exists {
				// The first text to claim a node keeps it. Nothing a snapshot builder produces puts one node into two
				// texts, and answering with either of them would be arbitrary.
				continue
			}
			if spans == nil {
				spans = make(map[accessibility.NodeID]spanRef)
			}
			spans[span.Node] = spanRef{container: n.ID, start: span.Start, end: span.End}
		}
		return true
	})
	return spans
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
	headers   map[accessibility.NodeID][]accessibility.NodeID
	spans     map[accessibility.NodeID]spanRef
	geometry  Geometry
}

// newWindowData indexes a tree so that the questions AT-SPI asks most often — the children of a node, where a node sits
// among its siblings, what a node is related to, and which headers describe a table's columns — can be answered without
// walking anything.
func newWindowData(t *accessibility.Tree, g Geometry) *windowData {
	d := &windowData{
		tree:      t,
		children:  make(map[accessibility.NodeID][]accessibility.NodeID),
		indexes:   make(map[accessibility.NodeID]int),
		relations: buildRelations(t),
		headers:   buildTableHeaders(t),
		spans:     buildSpanIndex(t),
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

// positionInSet returns where a node sits among the reported siblings that share its role and how many such siblings
// there are, which is what [appendPositionAttributes] writes the posinset and setsize attributes of a tab, a menu item
// or a radio button from. Both are zero for a node that is not reported, or that has no reported parent.
//
// It answers exactly what [accessibility.Tree.PositionInSet] does, from the sibling list this snapshot indexed rather
// than by deriving one from the tree. That matters where a set is asked about once per member: an
// org.a11y.atspi.Collection search whose rule names an attribute reads the attributes of every candidate it considers,
// and over a menu or a radio group the tree-derived answer rebuilds and reallocates the whole sibling list for each of
// them, which is one D-Bus call doing quadratic work.
func (d *windowData) positionInSet(id accessibility.NodeID) (pos, size int) {
	n := d.node(id)
	if n == nil || n.Ignored {
		return 0, 0
	}
	for _, siblingID := range d.unignoredChildren(d.parent(id)) {
		if sibling := d.node(siblingID); sibling != nil && sibling.Role == n.Role {
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

// attributesOf returns a node's AT-SPI object attributes, counting any set it belongs to from this snapshot's own
// indexes; see [Attributes], which is the same answer worked out from a tree alone.
func (d *windowData) attributesOf(n *accessibility.Node) dbus.Dict {
	return nodeAttributes(n, d.node(d.parent(n.ID)), d.positionInSet)
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

// columnHeaders returns the nodes that describe the columns of a table or a tree, in the order the header panel holding
// them publishes them, or nil when the snapshot holds no header for it. See [buildTableHeaders].
func (d *windowData) columnHeaders(id accessibility.NodeID) []accessibility.NodeID {
	return d.headers[id]
}

// spanOf returns the range of another node's text that a node occupies, and whether any node's text says it occupies
// one. See [buildSpanIndex].
func (d *windowData) spanOf(id accessibility.NodeID) (span spanRef, ok bool) {
	span, ok = d.spans[id]
	return span, ok
}

// isSpanTarget reports whether another node's text says that this node occupies part of it, which is what gives a link
// or an image within a paragraph org.a11y.atspi.Hyperlink.
func (d *windowData) isSpanTarget(id accessibility.NodeID) bool {
	_, ok := d.spans[id]
	return ok
}

// walkReported calls fn for every reported descendant of a node, in pre-order: a node is visited before its own
// children, and the children in the order their parent reports them, which is the order an assistive technology reads
// them in. The node the walk starts at is not visited, and neither is an ignored node: its children stand in for it,
// exactly as they do everywhere else. Returning false from fn stops the walk, so a search that has found what it needs
// can leave.
//
// Each node is visited at most once, however many places in the tree point at it, which is what keeps a malformed tree
// — one whose children links form a cycle — from being walked forever. The node the walk starts at is one of those: a
// cycle that leads back to it must not hand it to a caller that asked for what is inside it.
func (d *windowData) walkReported(id accessibility.NodeID, fn func(n *accessibility.Node) bool) {
	d.walkReportedChildren(id, map[accessibility.NodeID]bool{id: true}, fn)
}

// walkReportedChildren implements [windowData.walkReported] from one node, reporting whether the walk should go on.
// visited holds the ids already reached.
func (d *windowData) walkReportedChildren(id accessibility.NodeID, visited map[accessibility.NodeID]bool,
	fn func(n *accessibility.Node) bool,
) bool {
	for _, child := range d.unignoredChildren(id) {
		if visited[child] {
			continue
		}
		visited[child] = true
		n := d.node(child)
		if n == nil {
			continue
		}
		if !fn(n) {
			return false
		}
		if !d.walkReportedChildren(child, visited, fn) {
			return false
		}
	}
	return true
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
