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
func (b *AccessibilityBuilder) DescribeChildren() {
	if b.node.Role != role.Heading && b.node.Role != role.Label {
		return
	}
	b.snapshot.visitChildren(b.panel, b.node.ID, b.clip)
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
