// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// This file holds what the widgets in this package share when they describe themselves: the additions to
// AccessibilityBuilder that only a widget needs, and the helpers that more than one widget reaches for.

// axMaxSelectedRows bounds how many rows a list or table describes solely because they are selected. Rows that can be
// seen are always described, and the selection is worth describing whether or not it can be seen — an assistive
// technology asks what is selected — but "select all" in a table of a million rows must not turn into a million nodes.
const axMaxSelectedRows = 200

// axReach widens the part of a list or table that can be seen by one screenful above and one below it. An assistive
// technology moves through rows one at a time and can only move onto a row that has been described, so the rows just
// past either edge have to be there for it to step onto at all; selecting one scrolls it into view, and the description
// that follows reaches a screenful further still. A screenful in each direction keeps the count bounded while leaving
// room for the pause between one description and the next.
func axReach(visible geom.Rect) geom.Rect {
	if visible.Empty() {
		return visible
	}
	visible.Y -= visible.Height
	visible.Height *= 3
	return visible
}

// DescribeChildren describes the children of the panel being described as children of its node.
//
// It exists for the roles the snapshot builder treats as a single element: a heading and a label are static text,
// however many panels that text is drawn with, so their children are folded into the name rather than described. A
// heading that holds a link or an image is the exception — such a thing has to be reachable in its own right, since an
// assistive technology moves to it, says what it is and presses it — so the widget describing one asks for its children
// here. The name is still gathered from the whole of the text afterwards, exactly as it is for a heading of nothing but
// text, which is what an assistive technology reads the heading itself as.
//
// Nothing happens for any other role: the builder describes those children itself once ProvideAccessibility returns,
// and describing them here as well would list every one of them beneath its parent twice.
//
// Having described them is recorded on the snapshot rather than being left to be inferred from the role afterwards. A
// node's role can still move between here and the point the builder decides whether to describe the children — an
// Accessibility.Callback runs after ProvideAccessibility and is entitled to write any role it likes — so a node that
// asked here and is no longer static text by then would otherwise have its children described a second time.
//
// That same record is what makes asking twice within one description harmless. A widget that calls this from more than
// one place — a heading that describes its content and then defers to an embedded label that does the same — would
// otherwise append every child's id to the node a second time, leaving the parent listing each of its children twice,
// every position an assistive technology counts out of that list wrong, and the node's own PositionInSet and SizeOfSet
// with it. It is the failure AddVirtualChildOf refuses a repeated key for, and it is refused here the same way.
func (b *AccessibilityBuilder) DescribeChildren() {
	if b.node.Role != role.Heading && b.node.Role != role.Label {
		return
	}
	if b.snapshot.childrenDescribed[b.node.ID] {
		return
	}
	b.snapshot.markChildrenDescribed(b.node.ID)
	b.snapshot.visitChildren(b.panel, b.node.ID, b.clip)
}

// axChildAt is one panel a widget is describing beneath a node it invented, together with the index that panel occupies
// within its own parent. See AccessibilityBuilder.describeChildrenUnder.
type axChildAt struct {
	panel *Panel
	index int
}

// describeChildrenUnder describes real panels beneath a node the widget invented, which is how a virtual row of a
// document's table gains the cells a person reads it by: the cells are ordinary panels the widget holds as children,
// but the row between them and the widget exists only because the widget described it, so nothing else would put the
// two together. The panels must be the widget's own descendants, and each keeps its own identity, so a request about
// one reaches it exactly as it would have if it had been described where it actually sits.
//
// The caller records that it has described its children — see AccessibilityBuilder.DescribeChildren and
// axSnapshot.markChildrenDescribed — since otherwise the builder describes them a second time as children of the
// widget itself, and every position counted out of the widget's list of children would then be wrong.
//
// Each panel is visited at the index its entry names, which must be the index it occupies within its own parent rather
// than its position in the list handed here, so that a widget doing this from inside a table cell — where panels are
// keyed by where they sit, since they may not exist a moment later — is described under ids a request can find its way
// back through. See axCellContext. The caller states that index rather than leaving it to be derived, since a widget
// walks its own children in order anyway and searching its child list for each of them again is what would make
// describing a table of n cells cost n² rather than n.
//
// An entry with no panel, or one whose index is negative — which is what a panel the widget does not actually hold
// amounts to — is skipped, as is a panel that already has a node in this tree. Describing one twice would replace what
// it said the first time and list its id beneath two parents, which is what every position counted out of those lists —
// the index within the parent, what Tree.PositionInSet answers, and the ChildrenChanged events the next diff produces —
// would then be wrong about. It is the failure AddVirtualChildOf refuses a repeated key for, refused here the same way.
func (b *AccessibilityBuilder) describeChildrenUnder(parent accessibility.NodeID, children []axChildAt) {
	if b.snapshot.tree.Nodes[parent] == nil {
		return
	}
	for _, child := range children {
		p := child.panel
		if p == nil || child.index < 0 {
			continue
		}
		if p.Accessibility.owner == p && b.snapshot.tree.Nodes[p.Accessibility.id] != nil {
			continue
		}
		b.snapshot.visitChild(p, child.index, parent, b.clip)
	}
}

// addColumnHeaderPanel describes the panel a table header shows for one of its columns, and everything inside it,
// beneath the node for that column. The panel must be installed at the column's frame for the duration of the call, as
// it is for drawing.
//
// A column header is in the same position as a table cell: it is a panel, but the header does not hold it as a child,
// so nothing else would place it in the hierarchy, and it is detached again the moment the call returns, which leaves
// it with no window for a request from an assistive technology to reach it through. Both are solved the same way — the
// nodes are keyed by the column and the panel's position within it, and requests are sent to the header, which installs
// the panel again and finds what the request named. See axCellContext.
func (b *AccessibilityBuilder) addColumnHeaderPanel(parent accessibility.NodeID, col int, p *Panel) {
	b.addCellPanel(parent, accessibility.CellKey{Col: col}, p, false)
}

// axDescribeStaticContent fills in the name, role and ignored flag of a widget whose whole content is a piece of static
// text, a drawable, or both: a Label, a Tag or a DrawablePanel. text is whatever text it shows, and hasDrawable says
// whether it also shows a drawable.
//
// The text is the name, since that is what an assistive technology reads such a widget as. A widget that shows only a
// drawable has nothing in it that says what it shows, so its name comes from its tooltip, which is the usual way one is
// described — markdown hangs an image's alt text there. That has to be consulted here rather than being left to the
// description the snapshot would otherwise take from the tooltip, since a node marked ignored is never reached to hear
// it.
//
// A widget that nothing describes is skipped rather than announced as an image of nothing or as an empty piece of
// static text, both of which are worse than not being mentioned at all — a label used purely for spacing is the
// ordinary case. Skipping is only right for the role derived here, though: a widget an application has given a role of
// its own is something it means to be found — a drawable made focusable and given a press to stand for a button, say —
// and splicing that out of the tree would take a live control away from the person using one, so an explicit role is
// left exposed however little the widget has to say for itself.
func axDescribeStaticContent(b *AccessibilityBuilder, text string, hasDrawable bool) {
	node := b.Node()
	p := b.Panel()
	if node.Name == "" {
		node.Name = text
	}
	if node.Name == "" && hasDrawable {
		node.Name = axTooltipText(p)
	}
	if node.Role != role.Auto {
		return
	}
	if text == "" && hasDrawable {
		node.Role = role.Image
	} else {
		node.Role = role.Label
	}
	node.Ignored = text == "" && node.Name == "" && xreflect.IsNil(p.Accessibility.LabeledBy)
}

// axLabelOf returns the Label a panel is built around, which is the panel itself for a Label and the embedded one for a
// widget that is written by embedding a *Label and pointing Self at itself, as a table column header is. It returns nil
// for everything else, including the many widgets that merely pair a SetTitle with the String() that every panel
// answers with the name of its own type.
func axLabelOf(p *Panel) *Label {
	if p == nil {
		return nil
	}
	if labeler, ok := p.Self.(interface{ axLabel() *Label }); ok {
		return labeler.axLabel()
	}
	return nil
}
