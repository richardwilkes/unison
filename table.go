// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/toolbox/xmath/geom32"

// DefaultTableIndent is the amount of space to indent for each level of depth in the hierarchy.
const DefaultTableIndent = 20

// TableModel defines the methods a Table needs to access the data it will display.
type TableModel interface {
	// TopLevelRowCount returns the number of top-level rows.
	TopLevelRowCount() int
	// TopLevelRowAt returns the top-level row data for the given index.
	TopLevelRowAt(index int) TableRowData
	// ColumnCount returns the number of columns.
	ColumnCount() int
	// HierarchyColumnIndex returns the column index that will display the hierarchy.
	HierarchyColumnIndex() int
}

// TableRowData provides information about a single row of data.
type TableRowData interface {
	// CanHaveChildRows returns true if this row can have children, even if it currently does not have any.
	CanHaveChildRows() bool
	// ChildRowCount returns the number of direct child rows this row contains.
	ChildRowCount() int
	// ChildRowAt returns the row data for the given direct child index.
	ChildRowAt(index int) TableRowData
	// ColumnCell returns the panel that should be placed at the position of the cell for the given column index.
	ColumnCell(index int) *Panel
}

// Table provides a control that can display data in columns and rows.
type Table struct {
	Panel
	model       TableModel
	tableIndent float32
}

// NewTable creates a new Table control.
func NewTable(model TableModel) *Table {
	t := &Table{
		model:       model,
		tableIndent: DefaultTableIndent,
	}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	return t
}

// DefaultSizes provides the default sizing.
func (t *Table) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	max = MaxSize(max)
	colCount := t.model.ColumnCount()
	count := t.model.TopLevelRowCount()
	hierIndex := t.model.HierarchyColumnIndex()
	minWidth := make([]float32, colCount)
	prefWidth := make([]float32, colCount)
	maxWidth := make([]float32, colCount)
	for i := 0; i < count; i++ {
		row := t.model.TopLevelRowAt(i)
		minH, prefH, maxH := t.sizeForColumns(row, hierIndex, colCount, t.tableIndent, minWidth, prefWidth, maxWidth)
		min.Height += minH
		pref.Height += prefH
		max.Height += maxH
	}
	for i := 0; i < colCount; i++ {
		min.Width += minWidth[i]
		pref.Width += prefWidth[i]
		max.Width += maxWidth[i]
	}
	if border := t.Border(); border != nil {
		insets := border.Insets()
		min.AddInsets(insets)
		pref.AddInsets(insets)
		max.AddInsets(insets)
	}
	min.GrowToInteger()
	pref.GrowToInteger()
	max.GrowToInteger()
	return min, pref, max
}

func (t *Table) sizeForColumns(row TableRowData, hierIndex, colCount int, heirIndent float32, minWidth, prefWidth, maxWidth []float32) (minHeight, prefHeight, maxHeight float32) {
	for i := 0; i < colCount; i++ {
		cmin, cpref, cmax := row.ColumnCell(i).Sizes(geom32.Size{})
		if i == hierIndex && row.CanHaveChildRows() {
			cmin.Width += heirIndent
			cpref.Width += heirIndent
			cmax.Width += heirIndent
		}
		if minWidth[i] < cmin.Width {
			minWidth[i] = cmin.Width
		}
		if minHeight < cmin.Height {
			minHeight = cmin.Height
		}
		if prefWidth[i] < cpref.Width {
			prefWidth[i] = cpref.Width
		}
		if prefHeight < cpref.Height {
			prefHeight = cpref.Height
		}
		if maxWidth[i] < cmax.Width {
			maxWidth[i] = cmax.Width
		}
		if maxHeight < cmax.Height {
			maxHeight = cmax.Height
		}
	}
	if row.CanHaveChildRows() {
		rowCount := row.ChildRowCount()
		heirIndent += t.tableIndent
		for i := 0; i < rowCount; i++ {
			child := row.ChildRowAt(i)
			minH, prefH, maxH := t.sizeForColumns(child, hierIndex, colCount, heirIndent, minWidth, prefWidth, maxWidth)
			minHeight += minH
			prefHeight += prefH
			maxHeight += maxH
		}
	}
	return
}
