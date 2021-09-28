// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

const (
	// DefaultTableIndent is the default amount of space to indent for each level of depth in the hierarchy.
	DefaultTableIndent = 16
	// DefaultMinimumRowHeight is the default minimum height a row is permitted to have.
	DefaultMinimumRowHeight = 16
)

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
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
}

// Table provides a control that can display data in columns and rows.
type Table struct {
	Panel
	DividerColor      Ink
	model             TableModel
	columnWidths      []float32
	rowHeightCache    []float32
	TableIndent       float32
	MinimumRowHeight  float32
	ShowRowDivider    bool
	ShowColumnDivider bool
}

// NewTable creates a new Table control.
func NewTable(model TableModel) *Table {
	t := &Table{
		model:             model,
		TableIndent:       DefaultTableIndent,
		MinimumRowHeight:  DefaultMinimumRowHeight,
		ShowRowDivider:    true,
		ShowColumnDivider: true,
	}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	return t
}

// ColumnWidths returns the width of each column.
func (t *Table) ColumnWidths() []float32 {
	t.ensureColumnWidths()
	return t.columnWidths
}

// SetColumnWidth sets the width of a column. Will be forced to 0 if a valud less than 0 is passed in.
func (t *Table) SetColumnWidth(index int, width float32) {
	t.ensureColumnWidths()
	t.columnWidths[index] = mathf32.Max(width, 0)
}

// SizeColumnsToFit sizes each column to its preferred size.
func (t *Table) SizeColumnsToFit() {
	hierIndex := t.model.HierarchyColumnIndex()
	colCount := t.model.ColumnCount()
	rowCount := t.model.TopLevelRowCount()
	t.columnWidths = make([]float32, colCount)
	for i := 0; i < rowCount; i++ {
		t.sizeColumns(t.model.TopLevelRowAt(i), hierIndex, colCount, t.TableIndent)
	}
}

func (t *Table) sizeColumns(row TableRowData, hierIndex, colCount int, heirIndent float32) {
	for i := 0; i < colCount; i++ {
		_, pref, _ := row.ColumnCell(i).Sizes(geom32.Size{})
		if i == hierIndex && row.CanHaveChildRows() && row.IsOpen() {
			pref.Width += heirIndent
		}
		if t.columnWidths[i] < pref.Width {
			t.columnWidths[i] = pref.Width
		}
	}
	if row.CanHaveChildRows() && row.IsOpen() {
		rowCount := row.ChildRowCount()
		heirIndent += t.TableIndent
		for i := 0; i < rowCount; i++ {
			t.sizeColumns(row.ChildRowAt(i), hierIndex, colCount, heirIndent)
		}
	}
}

func (t *Table) ensureColumnWidths() {
	colCount := t.model.ColumnCount()
	if len(t.columnWidths) != colCount {
		widths := make([]float32, colCount)
		copy(widths, t.columnWidths)
		for i := len(t.columnWidths); i < colCount; i++ {
			widths[i] = 100
		}
		t.columnWidths = widths
	}
}

func (t *Table) ensureRowCache() {
	t.ensureColumnWidths()
	count := t.model.TopLevelRowCount()
	rowCount := count
	for i := 0; i < count; i++ {
		rowCount += t.countOpenRowChildrenRecursively(t.model.TopLevelRowAt(i))
	}
	if len(t.rowHeightCache) != rowCount {
		heights := make([]float32, rowCount)
		copy(heights, t.rowHeightCache)
		for i := len(t.rowHeightCache); i < rowCount; i++ {
			t.rowHeightCache[i] = 0
		}
		t.rowHeightCache = heights
	}
	colCount := t.model.ColumnCount()
	j := 0
	for i := 0; i < count; i++ {
		j = t.ensureRowCacheRecursively(t.model.TopLevelRowAt(i), j, colCount)
	}
}

func (t *Table) ensureRowCacheRecursively(row TableRowData, index, colCount int) int {
	t.rowHeightCache[index] = t.heightForColumns(row, colCount)
	index++
	if row.CanHaveChildRows() && row.IsOpen() {
		count := row.ChildRowCount()
		for i := 0; i < count; i++ {
			index = t.ensureRowCacheRecursively(row.ChildRowAt(i), index, colCount)
		}
	}
	return index
}

func (t *Table) countOpenRowChildrenRecursively(row TableRowData) int {
	if !row.CanHaveChildRows() || !row.IsOpen() {
		return 0
	}
	count := row.ChildRowCount()
	for i := count - 1; i >= 0; i-- {
		count += t.countOpenRowChildrenRecursively(row.ChildRowAt(i))
	}
	return count
}

// DefaultSizes provides the default sizing.
func (t *Table) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	t.ensureRowCache()
	for _, w := range t.columnWidths {
		pref.Width += w
	}
	for _, h := range t.rowHeightCache {
		pref.Height += h
	}
	if t.ShowColumnDivider {
		pref.Width += float32(len(t.columnWidths) - 1)
	}
	if t.ShowRowDivider {
		pref.Height += float32(len(t.rowHeightCache) - 1)
	}
	if border := t.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	return pref, pref, pref
}

func (t *Table) heightForColumns(row TableRowData, colCount int) float32 {
	var height float32
	for i := 0; i < colCount; i++ {
		_, cpref, _ := row.ColumnCell(i).Sizes(geom32.Size{Width: t.columnWidths[i]})
		h := mathf32.Max(mathf32.Ceil(cpref.Height), t.MinimumRowHeight)
		if height < h {
			height = h
		}
	}
	return height
}

// DefaultDraw provides the default drawing.
func (t *Table) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	t.ensureColumnWidths()
	x := t.columnWidths[0] + insets.Left
	if t.ShowColumnDivider {
		x++
	}
	var firstCol int
	if dirty.X >= x {
		colCount := t.model.ColumnCount()
		for i := 1; i < colCount; i++ {
			x += t.columnWidths[i]
			if t.ShowColumnDivider {
				x++
			}
			if dirty.X >= x {
				firstCol++
			}
		}
	}
}
