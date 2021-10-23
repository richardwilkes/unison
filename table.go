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
	"github.com/richardwilkes/unison/fa"
)

const (
	// DefaultHierarchyIndent is the default amount of space to indent for each level of depth in the hierarchy.
	DefaultHierarchyIndent = 16
	// DefaultMinimumRowHeight is the default minimum height a row is permitted to have.
	DefaultMinimumRowHeight = 16
)

// TableRowData provides information about a single row of data.
type TableRowData interface {
	// CanHaveChildRows returns true if this row can have children, even if it currently does not have any.
	CanHaveChildRows() bool
	// ChildRows returns the child rows.
	ChildRows() []TableRowData
	// ColumnCell returns the panel that should be placed at the position of the cell for the given column index. If you
	// need for the cell to retain widget state, make sure to return the same widget each time rather than creating a
	// new one.
	ColumnCell(index int) Paneler
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
}

type tableCache struct {
	row    TableRowData
	parent int
	depth  int
	height float32
}

type tableHitRect struct {
	geom32.Rect
	handler func(where geom32.Point, button, clickCount int, mod Modifiers)
}

// Table provides a control that can display data in columns and rows.
type Table struct {
	Panel
	DividerColor         Ink
	RowColor             Ink
	OnRowColor           Ink
	AltRowColor          Ink
	OnAltRowColor        Ink
	SelectionColor       Ink
	OnSelectionColor     Ink
	Padding              geom32.Insets
	topLevelRows         []TableRowData
	selMap               map[TableRowData]bool
	HierarchyColumnIndex int       // The column index that will display the hierarchy
	ColumnWidths         []float32 // The widths of each column
	hitRects             []tableHitRect
	rowCache             []tableCache
	interactionRow       int
	interactionColumn    int
	HierarchyIndent      float32
	MinimumRowHeight     float32
	ShowRowDivider       bool
	ShowColumnDivider    bool
}

// NewTable creates a new Table control.
func NewTable() *Table {
	t := &Table{
		selMap:               make(map[TableRowData]bool),
		Padding:              geom32.NewUniformInsets(4),
		HierarchyIndent:      DefaultHierarchyIndent,
		MinimumRowHeight:     DefaultMinimumRowHeight,
		HierarchyColumnIndex: 1,
		ShowRowDivider:       true,
		ShowColumnDivider:    true,
	}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	t.MouseDownCallback = t.DefaultMouseDown
	t.MouseDragCallback = t.DefaultMouseDrag
	t.MouseUpCallback = t.DefaultMouseUp
	return t
}

// DefaultDraw provides the default drawing.
func (t *Table) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}

	var firstCol int
	x := insets.Left
	for i, w := range t.ColumnWidths {
		x1 := x + w
		if t.ShowColumnDivider {
			x1++
		}
		if x1 >= dirty.X {
			break
		}
		x = x1
		firstCol = i + 1
	}

	var firstRow int
	y := insets.Top
	rowCount := len(t.rowCache)
	for i := 0; i < rowCount; i++ {
		y1 := y + t.rowCache[i].height
		if t.ShowRowDivider {
			y1++
		}
		if y1 >= dirty.Y {
			break
		}
		y = y1
		firstRow = i + 1
	}

	canvas.DrawRect(dirty, ChooseInk(t.RowColor, ListColor).Paint(canvas, dirty, Fill))

	lastY := dirty.Bottom()
	rect := dirty
	rect.Y = y
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		rect.Height = t.rowCache[r].height
		if t.IsRowOrAnyParentSelected(r) {
			canvas.DrawRect(rect, ChooseInk(t.SelectionColor, SelectionColor).Paint(canvas, rect, Fill))
		} else if r%2 == 1 {
			canvas.DrawRect(rect, ChooseInk(t.AltRowColor, ListAltColor).Paint(canvas, rect, Fill))
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider && r != rowCount-1 {
			rect.Height = 1
			canvas.DrawRect(rect, ChooseInk(t.DividerColor, DividerColor).Paint(canvas, rect, Fill))
			rect.Y++
		}
	}

	if t.ShowColumnDivider {
		rect = dirty
		rect.X = x
		rect.Width = 1
		for c := firstCol; c < len(t.ColumnWidths)-1; c++ {
			rect.X += t.ColumnWidths[c]
			canvas.DrawRect(rect, ChooseInk(t.DividerColor, DividerColor).Paint(canvas, rect, Fill))
			rect.X++
		}
	}

	rect = dirty
	rect.Y = y
	lastX := dirty.Right()
	faDesc := FontDescriptor{
		Family:  FontAwesomeFreeFamilyName,
		Size:    t.HierarchyIndent - 6,
		Weight:  BlackFontWeight,
		Spacing: StandardSpacing,
		Slant:   NoSlant,
	}
	faFont := faDesc.Font()
	t.hitRects = nil
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		row := t.rowCache[r].row
		var fg Ink
		switch {
		case t.IsRowOrAnyParentSelected(r):
			fg = ChooseInk(t.OnSelectionColor, OnSelectionColor)
		case row.IsOpen():
			fg = ChooseInk(t.OnAltRowColor, OnListAltColor)
		default:
			fg = ChooseInk(t.OnRowColor, OnListColor)
		}
		rect.X = x
		rect.Height = t.rowCache[r].height
		for c := firstCol; c < len(t.ColumnWidths) && rect.X < lastX; c++ {
			rect.Width = t.ColumnWidths[c]
			cellRect := rect
			cellRect.Inset(t.Padding)
			if c == t.HierarchyColumnIndex {
				if row.CanHaveChildRows() {
					var code string
					if row.IsOpen() {
						code = fa.ChevronCircleDown
					} else {
						code = fa.ChevronCircleRight
					}
					extents := faFont.Extents(code)
					left := cellRect.X + t.HierarchyIndent*float32(t.rowCache[r].depth)
					canvas.DrawSimpleText(code, left+(t.HierarchyIndent-extents.Width)/2,
						cellRect.Y+(cellRect.Height-faDesc.Size)/2+faDesc.Size-0.5, faFont,
						fg.Paint(canvas, cellRect, Fill))
					t.hitRects = append(t.hitRects, t.newTableHitRect(geom32.NewRect(left,
						cellRect.Y+(cellRect.Height-t.HierarchyIndent)/2, t.HierarchyIndent, t.HierarchyIndent), row))
				}
				indent := t.HierarchyIndent*float32(t.rowCache[r].depth+1) + t.Padding.Left
				cellRect.X += indent
				cellRect.Width -= indent
			}
			cell := row.ColumnCell(c).AsPanel()
			t.installCell(cell, cellRect)
			canvas.Save()
			canvas.Translate(cellRect.X, cellRect.Y)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, cellRect)
			t.uninstallCell(cell)
			canvas.Restore()
			rect.X += t.ColumnWidths[c]
			if t.ShowColumnDivider {
				rect.X++
			}
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider {
			rect.Y++
		}
	}
}

func (t *Table) installCell(cell *Panel, frame geom32.Rect) {
	cell.SetFrameRect(frame)
	cell.parent = t.AsPanel()
}

func (t *Table) uninstallCell(cell *Panel) {
	cell.parent = nil
}

// OverRow returns the row index that the y coordinate is over, or -1 if it isn't over any row.
func (t *Table) OverRow(y float32) int {
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	end := insets.Top
	for i := range t.rowCache {
		start := end
		end += t.rowCache[i].height
		if t.ShowRowDivider {
			end++
		}
		if y >= start && y < end {
			return i
		}
	}
	return -1
}

// OverColumn returns the column index that the x coordinate is over, or -1 if it isn't over any column.
func (t *Table) OverColumn(x float32) int {
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	end := insets.Left
	for i := range t.ColumnWidths {
		start := end
		end += t.ColumnWidths[i]
		if t.ShowColumnDivider {
			end++
		}
		if x >= start && x < end {
			return i
		}
	}
	return -1
}

// CellFrame returns the frame of the given cell.
func (t *Table) CellFrame(row, col int) geom32.Rect {
	if row < 0 || col < 0 || row >= len(t.rowCache) || col >= len(t.ColumnWidths) {
		return geom32.Rect{}
	}
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	x := insets.Left
	for c := 0; c < col; c++ {
		x += t.ColumnWidths[c]
		if t.ShowColumnDivider {
			x++
		}
	}
	y := insets.Top
	for r := 0; r < row; r++ {
		y += t.rowCache[r].height
		if t.ShowRowDivider {
			y++
		}
	}
	rect := geom32.NewRect(x, y, t.rowCache[row].height, t.ColumnWidths[col])
	rect.Inset(t.Padding)
	if col == t.HierarchyColumnIndex {
		indent := t.HierarchyIndent*float32(t.rowCache[row].depth+1) + t.Padding.Left
		rect.X += indent
		rect.Width -= indent
	}
	return rect
}

func (t *Table) newTableHitRect(rect geom32.Rect, row TableRowData) tableHitRect {
	return tableHitRect{
		Rect: rect,
		handler: func(where geom32.Point, button, clickCount int, mod Modifiers) {
			row.SetOpen(!row.IsOpen())
			t.SyncToModel()
			t.MarkForRedraw()
		},
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (t *Table) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	t.interactionRow = -1
	t.interactionColumn = -1
	for _, one := range t.hitRects {
		if one.ContainsPoint(where) {
			one.handler(where, button, clickCount, mod)
			return true
		}
	}
	stop := true
	if row := t.OverRow(where.Y); row != -1 {
		if t.IsRowOrAnyParentSelected(row) {
			if mod&OptionModifier != 0 {
				t.selMap[t.rowCache[row].row] = false
				t.MarkForRedraw()
			}
		} else {
			if mod&(ShiftModifier|OptionModifier) == 0 {
				t.selMap = make(map[TableRowData]bool)
			}
			t.selMap[t.rowCache[row].row] = true
			t.MarkForRedraw()
		}
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.rowCache[row].row.ColumnCell(col).AsPanel()
			if cell.MouseDownCallback != nil {
				t.interactionRow = row
				t.interactionColumn = col
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				stop = cell.MouseDownCallback(where, button, clickCount, mod)
				t.uninstallCell(cell)
			}
		}
	}
	return stop
}

// DefaultMouseDrag provides the default mouse drag handling.
func (t *Table) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	stop := false
	if t.interactionRow != -1 && t.interactionColumn != -1 {
		cell := t.rowCache[t.interactionRow].row.ColumnCell(t.interactionColumn).AsPanel()
		rect := t.CellFrame(t.interactionRow, t.interactionColumn)
		t.installCell(cell, rect)
		where.Subtract(rect.Point)
		stop = cell.MouseDragCallback(where, button, mod)
		t.uninstallCell(cell)
	}
	return stop
}

// DefaultMouseUp provides the default mouse up handling.
func (t *Table) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	stop := false
	if t.interactionRow != -1 && t.interactionColumn != -1 {
		cell := t.rowCache[t.interactionRow].row.ColumnCell(t.interactionColumn).AsPanel()
		rect := t.CellFrame(t.interactionRow, t.interactionColumn)
		t.installCell(cell, rect)
		where.Subtract(rect.Point)
		stop = cell.MouseUpCallback(where, button, mod)
		t.uninstallCell(cell)
	}
	return stop
}

// IsRowOrAnyParentSelected returns true if the specified row index or any of its parents are selected.
func (t *Table) IsRowOrAnyParentSelected(index int) bool {
	if index < 0 || index >= len(t.rowCache) {
		return false
	}
	for index >= 0 {
		if t.selMap[t.rowCache[index].row] {
			return true
		}
		index = t.rowCache[index].parent
	}
	return false
}

// SetTopLevelRows sets the top-level rows this table will display. This will call SyncToModel() automatically.
func (t *Table) SetTopLevelRows(rows []TableRowData) {
	t.topLevelRows = rows
	t.selMap = make(map[TableRowData]bool)
	t.SyncToModel()
}

// SyncToModel causes the table to update its internal caches to reflect the current model.
func (t *Table) SyncToModel() {
	rowCount := 0
	for _, row := range t.topLevelRows {
		rowCount += t.countOpenRowChildrenRecursively(row)
	}
	t.rowCache = make([]tableCache, rowCount)
	j := 0
	for _, row := range t.topLevelRows {
		j = t.buildRowCacheEntry(row, -1, j, 0)
	}
	_, pref, _ := t.DefaultSizes(geom32.Size{})
	rect := t.FrameRect()
	rect.Size = pref
	t.SetFrameRect(rect)
}

func (t *Table) countOpenRowChildrenRecursively(row TableRowData) int {
	count := 1
	if row.CanHaveChildRows() && row.IsOpen() {
		for _, child := range row.ChildRows() {
			count += t.countOpenRowChildrenRecursively(child)
		}
	}
	return count
}

func (t *Table) buildRowCacheEntry(row TableRowData, parentIndex, index, depth int) int {
	t.rowCache[index].row = row
	t.rowCache[index].parent = parentIndex
	t.rowCache[index].depth = depth
	t.rowCache[index].height = t.heightForColumns(row, depth)
	parentIndex = index
	index++
	if row.CanHaveChildRows() && row.IsOpen() {
		for _, child := range row.ChildRows() {
			index = t.buildRowCacheEntry(child, parentIndex, index, depth+1)
		}
	}
	return index
}

func (t *Table) heightForColumns(row TableRowData, depth int) float32 {
	var height float32
	for i, w := range t.ColumnWidths {
		if w <= 0 {
			continue
		}
		w -= t.Padding.Left + t.Padding.Right
		if i == t.HierarchyColumnIndex {
			w -= t.Padding.Left + t.HierarchyIndent*float32(depth+1)
		}
		_, cpref, _ := row.ColumnCell(i).AsPanel().Sizes(geom32.Size{Width: w})
		cpref.Height += t.Padding.Top + t.Padding.Bottom
		if height < cpref.Height {
			height = cpref.Height
		}
	}
	return mathf32.Max(mathf32.Ceil(height), t.MinimumRowHeight)
}

// SizeColumnsToFit sizes each column to its preferred size.
func (t *Table) SizeColumnsToFit() {
	t.ColumnWidths = make([]float32, len(t.ColumnWidths))
	for _, cache := range t.rowCache {
		for i := range t.ColumnWidths {
			_, pref, _ := cache.row.ColumnCell(i).AsPanel().Sizes(geom32.Size{})
			pref.Width += t.Padding.Left + t.Padding.Right
			if i == t.HierarchyColumnIndex {
				pref.Width += t.Padding.Left + t.HierarchyIndent*float32(cache.depth+1)
			}
			if t.ColumnWidths[i] < pref.Width {
				t.ColumnWidths[i] = pref.Width
			}
		}
	}
	for i, cache := range t.rowCache {
		t.rowCache[i].height = t.heightForColumns(cache.row, cache.depth)
	}
}

// DefaultSizes provides the default sizing.
func (t *Table) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	for _, w := range t.ColumnWidths {
		pref.Width += w
	}
	for _, cache := range t.rowCache {
		pref.Height += cache.height
	}
	if t.ShowColumnDivider {
		pref.Width += float32(len(t.ColumnWidths) - 1)
	}
	if t.ShowRowDivider {
		pref.Height += float32(len(t.rowCache) - 1)
	}
	if border := t.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	return pref, pref, pref
}
