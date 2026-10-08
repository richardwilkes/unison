// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

func hiddenTableTestRow(id string) *tableTestRow {
	row := newTableTestRow(id)
	row.hidden = true
	return row
}

func rowIDs(rows []*tableTestRow) []tid.TID {
	ids := make([]tid.TID, len(rows))
	for i, row := range rows {
		ids[i] = row.ID()
	}
	return ids
}

func TestTableHiddenRowsAreNotShownButStayInModel(t *testing.T) {
	c := check.New(t)
	parent := hiddenTableTestRow("p")
	parent.SetChildren([]*tableTestRow{newTableTestRow("c")})
	parent.SetOpen(true)
	table := newTestTable(newTableTestRow("a"), hiddenTableTestRow("h"), parent, newTableTestRow("b"))
	c.Equal(2, table.LastRowIndex()+1)
	c.Equal(tid.TID("a"), table.RowFromIndex(0).ID())
	c.Equal(tid.TID("b"), table.RowFromIndex(1).ID())
	c.Equal([]tid.TID{"a", "h", "p", "b"}, rowIDs(table.RootRows()))
}

func TestTableFilterSkipsHiddenRows(t *testing.T) {
	c := check.New(t)
	table := newTestTable(newTableTestRow("a"), hiddenTableTestRow("h"))
	table.ApplyFilter(func(_ *tableTestRow) bool { return false })
	c.Equal([]tid.TID{"a"}, rowIDs(table.RootRows()))
}

func TestTableDropKeepsHiddenRows(t *testing.T) {
	c := check.New(t)
	table := newTestTable(newTableTestRow("a"), hiddenTableTestRow("h"), newTableTestRow("b"))
	x := newTableTestRow("x")
	installMoveDropSupport(t, table, newTestTable(x), x)
	c.True(table.DropCallback(&fakeDragInfo{dataType: tableDropDataType().UTI}, upperHalfOf(table, 1), 0))
	c.Equal([]tid.TID{"a", "h", "x", "b"}, rowIDs(table.RootRows()))
}

func TestTableHiddenRowsMarker(t *testing.T) {
	c := check.New(t)
	table := newTestTable(newTableTestRow("a"), hiddenTableTestRow("h1"), hiddenTableTestRow("h2"),
		newTableTestRow("b"))
	table.SetFrameRect(geom.NewRect(0, 0, 300, 300))
	var tipRows []*tableTestRow
	table.HiddenTooltipCallback = func(rows []*tableTestRow) string {
		tipRows = rows
		return "hidden"
	}
	onBoundary := geom.NewPoint(1, table.RowFrame(1).Y)

	// The marker is off by default.
	c.True(table.DefaultUpdateTooltipCallback(onBoundary, geom.Rect{}).Empty())
	c.Equal(0, len(tipRows))

	table.ShowHiddenRowsMarker = true
	c.False(table.DefaultUpdateTooltipCallback(onBoundary, geom.Rect{}).Empty())
	c.Equal([]tid.TID{"h1", "h2"}, rowIDs(tipRows))
}

func TestTableHiddenRowsMarkerAccessibility(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	var pressed []*tableTestRow
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(newTableTestRow("a"), hiddenTableTestRow("h"), newTableTestRow("b"))
			table.ShowHiddenRowsMarker = true
			table.HiddenTooltipCallback = func(_ []*tableTestRow) string { return "1 hidden" }
			table.HiddenClickCallback = func(rows []*tableTestRow) { pressed = rows }
			if wnd = newHeadlessWindow(t, "Hidden", geom.NewRect(10, 10, 400, 400), table); wnd != nil {
				wnd.ToFront()
			}
		}))
	screen.Sync()
	screen.EnableAccessibility()
	tree := screen.AccessibilityTree(wnd)
	tableNode := screen.AccessibilityNodeFor(table)
	c.Equal(2, tableNode.RowCount)

	// The marker is described within the row above it, and pressing it calls the click callback with the run.
	var marker *accessibility.Node
	for _, child := range axChildNodes(tree, axTableRows(tree, tableNode)["a"]) {
		if child.Name == "1 hidden" {
			marker = child
		}
	}
	c.NotNil(marker)
	if marker == nil {
		return
	}
	c.Equal(role.Button, marker.Role)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: marker.ID, Action: accessibility.Press}))
	c.Equal([]tid.TID{"h"}, rowIDs(pressed))
}
