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
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
)

// tableTestRow is a minimal TableRowData implementation for exercising selection logic. The table is built with no
// columns, so ColumnCell is never invoked and no font/graphics work is required.
type tableTestRow struct {
	parent   *tableTestRow
	id       tid.TID
	children []*tableTestRow
	open     bool
}

func newTableTestRow(id string) *tableTestRow {
	return &tableTestRow{id: tid.TID(id)}
}

func (r *tableTestRow) CloneForTarget(_ unison.Paneler, newParent *tableTestRow) *tableTestRow {
	return &tableTestRow{id: r.id, parent: newParent, open: r.open}
}
func (r *tableTestRow) ID() tid.TID                    { return r.id }
func (r *tableTestRow) Parent() *tableTestRow          { return r.parent }
func (r *tableTestRow) SetParent(parent *tableTestRow) { r.parent = parent }
func (r *tableTestRow) CanHaveChildren() bool          { return len(r.children) > 0 }
func (r *tableTestRow) Children() []*tableTestRow      { return r.children }
func (r *tableTestRow) SetChildren(children []*tableTestRow) {
	r.children = children
	for _, child := range children {
		child.parent = r
	}
}
func (r *tableTestRow) CellDataForSort(_ int) string { return string(r.id) }
func (r *tableTestRow) ColumnCell(_, _ int, _, _ unison.Ink, _, _, _ bool) unison.Paneler {
	return unison.NewPanel()
}
func (r *tableTestRow) IsOpen() bool      { return r.open }
func (r *tableTestRow) SetOpen(open bool) { r.open = open }

// newTestTable builds a synced table from the supplied root rows.
func newTestTable(rows ...*tableTestRow) *unison.Table[*tableTestRow] {
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows(rows)
	table := unison.NewTable[*tableTestRow](model)
	table.SyncToModel()
	return table
}

// flatRows returns count root rows named r0..r(count-1).
func flatRows(count int) []*tableTestRow {
	rows := make([]*tableTestRow, count)
	for i := range rows {
		rows[i] = newTableTestRow("r" + string(rune('0'+i)))
	}
	return rows
}

func TestTableSelectByIndex(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	table.SelectByIndex(1, 3)
	c.Equal(2, table.SelectionCount())
	c.True(table.IsRowSelected(1))
	c.True(table.IsRowSelected(3))
	c.False(table.IsRowSelected(0))
	c.True(table.HasSelection())
	c.Equal(1, table.FirstSelectedRowIndex())
	c.Equal(3, table.LastSelectedRowIndex())
}

func TestTableSelectByIndexIgnoresOutOfRange(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	table.SelectByIndex(-1, 99, 2)
	c.Equal(1, table.SelectionCount())
	c.True(table.IsRowSelected(2))
}

func TestTableSelectRangeClamps(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	table.SelectRange(1, 3)
	c.Equal(3, table.SelectionCount())
	c.True(table.IsRowSelected(1))
	c.True(table.IsRowSelected(2))
	c.True(table.IsRowSelected(3))

	// Out-of-bounds bounds are clamped to the available rows.
	table.ClearSelection()
	table.SelectRange(-5, 99)
	c.Equal(5, table.SelectionCount())
}

func TestTableSelectRangeInvertedIsNoOp(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	table.SelectRange(3, 1)
	c.False(table.HasSelection())
}

func TestTableDeselect(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	table.SelectAll()
	c.Equal(5, table.SelectionCount())
	table.DeselectByIndex(0, 2)
	c.Equal(3, table.SelectionCount())
	c.False(table.IsRowSelected(0))
	c.False(table.IsRowSelected(2))
	c.True(table.IsRowSelected(1))

	table.DeselectRange(1, 4)
	c.False(table.HasSelection())
}

func TestTableClearSelection(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	table.SelectAll()
	c.True(table.HasSelection())
	table.ClearSelection()
	c.Equal(0, table.SelectionCount())
	c.Equal(-1, table.FirstSelectedRowIndex())
	c.Equal(-1, table.LastSelectedRowIndex())
}

func TestTableSelectionChangedCallbackFires(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(5)...)
	calls := 0
	table.SelectionChangedCallback = func() { calls++ }
	table.SelectByIndex(1)
	c.Equal(1, calls)
	table.ClearSelection()
	c.Equal(2, calls)
}

func TestTableHierarchySelection(t *testing.T) {
	c := check.New(t)
	parent := newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), newTableTestRow("c1")})
	parent.SetOpen(true)
	sibling := newTableTestRow("s")
	table := newTestTable(parent, sibling)

	// Disclosed order is p(0), c0(1), c1(2), s(3).
	c.Equal(4, table.LastRowIndex()+1)

	// Selecting the parent makes its descendants report as indirectly selected.
	table.SelectByIndex(0)
	c.True(table.IsRowOrAnyParentSelected(1))
	c.False(table.IsRowSelected(1))

	// With both the parent and a child explicitly selected, the minimal set collapses to just the parent.
	table.SelectByIndex(1)
	c.Equal(2, len(table.SelectedRows(false)))
	minimal := table.SelectedRows(true)
	c.Equal(1, len(minimal))
	c.Equal(parent, minimal[0])
}
