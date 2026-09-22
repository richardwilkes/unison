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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison/accessibility"
)

// axHeaderRow is the least a table needs from its rows, so that a header has a table with columns to describe.
type axHeaderRow struct {
	parent *axHeaderRow
	id     tid.TID
}

func (r *axHeaderRow) CloneForTarget(_ Paneler, newParent *axHeaderRow) *axHeaderRow {
	return &axHeaderRow{id: r.id, parent: newParent}
}
func (r *axHeaderRow) ID() tid.TID                   { return r.id }
func (r *axHeaderRow) Parent() *axHeaderRow          { return r.parent }
func (r *axHeaderRow) SetParent(parent *axHeaderRow) { r.parent = parent }
func (r *axHeaderRow) CanHaveChildren() bool         { return false }
func (r *axHeaderRow) Children() []*axHeaderRow      { return nil }
func (r *axHeaderRow) SetChildren(_ []*axHeaderRow)  {}
func (r *axHeaderRow) CellDataForSort(_ int) string  { return string(r.id) }
func (r *axHeaderRow) IsOpen() bool                  { return false }
func (r *axHeaderRow) SetOpen(_ bool)                {}
func (r *axHeaderRow) ColumnCell(_, _ int, _, _ Ink, _, _, _ bool) Paneler {
	return NewLabel()
}

// axShadowedColumnHeader is a column header written the documented way — by embedding a header built around a *Label
// and pointing Self at itself — which draws words of its own rather than the ones the label beneath it holds, and
// shadows axTextLine to say so, exactly as DefaultTableColumnHeader does for a title shrunk by the sort indicator.
type axShadowedColumnHeader struct {
	*DefaultTableColumnHeader[*axHeaderRow]
	drawn string
}

// axShadowedHeaderBounds is where axShadowedColumnHeader claims its text was drawn.
var axShadowedHeaderBounds = geom.NewRect(3, 5, 17, 19)

func (h *axShadowedColumnHeader) axTextLine() (runes []rune, decorations []*TextDecoration,
	line accessibility.Line,
) {
	runes, decorations, line = axStaticTextLine(h.ContentRect(false), h.HAlign, h.VAlign, h.Font,
		NewText(h.drawn, &TextDecoration{Font: h.Font}), nil, h.Side, h.Gap)
	line.Bounds = axShadowedHeaderBounds
	return runes, decorations, line
}

// axNewShadowedColumnHeader builds a column header whose embedded label holds title while what it draws is drawn.
func axNewShadowedColumnHeader(title, drawn string) *axShadowedColumnHeader {
	inner := NewTableColumnHeader[*axHeaderRow](title, "", nil)
	shadowed := &axShadowedColumnHeader{DefaultTableColumnHeader: inner, drawn: drawn}
	inner.Self = shadowed
	return shadowed
}

// TestTableHeaderPublishesShadowedText verifies that a column header's text and the line it was measured over come
// from the same call through Self. The lines, the advances and the styled runs all index the runes axTextLine handed
// back, so publishing the embedded label's own string beside them would leave every offset an adapter derives
// addressing a different set of characters; and a header drawing words of its own is described by them even when the
// label it embeds holds nothing at all. See TableHeader.axDescribeColumnHeaderText. A session owns most of the
// package's mutable globals while it runs, so this may not call t.Parallel.
func TestTableHeaderPublishesShadowedText(t *testing.T) {
	c := check.New(t)
	var header *TableHeader[*axHeaderRow]
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 400},
		StartupFinishedCallback(func() {
			model := &SimpleTableModel[*axHeaderRow]{}
			model.SetRootRows([]*axHeaderRow{{id: "a"}})
			table := NewTable[*axHeaderRow](model)
			table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
			table.SyncToModel()
			header = NewTableHeader[*axHeaderRow](table,
				TableColumnHeader[*axHeaderRow](axNewShadowedColumnHeader("Label", "Drawn")),
				TableColumnHeader[*axHeaderRow](axNewShadowedColumnHeader("", "Nameless")))
			wnd = axNewTestWindow(t, "shadowed header text", geom.NewRect(10, 10, 500, 300), header)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)

	node := screen.AccessibilityNodeFor(header)
	c.NotNil(node)
	if node == nil {
		return
	}
	c.Equal(2, len(node.Children), "the header describes one element per column header")
	if len(node.Children) != 2 {
		return
	}
	for i, expected := range []string{"Drawn", "Nameless"} {
		col := tree.Node(node.Children[i])
		c.NotNil(col)
		if col == nil {
			continue
		}
		c.NotNil(col.Text, "a header drawing text reports the line it was drawn on")
		if col.Text == nil {
			continue
		}
		c.Equal(expected, col.Text.Text,
			"the text is the runes the header says it drew, not the ones the label beneath it holds")
		c.Equal(1, len(col.Text.Lines), "static text occupies exactly one line")
		if len(col.Text.Lines) != 1 {
			continue
		}
		line := col.Text.Lines[0]
		c.Equal(axShadowedHeaderBounds, line.Bounds, "the line is the one the header measured, not the label's")
		c.Equal(0, line.Start)
		c.Equal(len([]rune(expected)), line.End, "the line covers the whole of the text that was published")
		c.Equal(len([]rune(expected))+1, len(line.Advances),
			"the advances index the same runes the text is built from")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
