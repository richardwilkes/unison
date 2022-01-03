// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"time"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

const (
	// DefaultHierarchyIndent is the default amount of space to indent for each level of depth in the hierarchy.
	DefaultHierarchyIndent = 16
	// DefaultMinimumRowHeight is the default minimum height a row is permitted to have.
	DefaultMinimumRowHeight = 16
	// DefaultColumnResizeSlop is the default number of logical pixels on either side of the column divider where the
	// user can grab the divider and resize it.
	DefaultColumnResizeSlop = 4
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
	ColumnCell(index int, selected bool) Paneler
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
	// CellDataForSort returns the string that represents the data in the specified cell.
	CellDataForSort(index int) string
}

// ColumnSize holds the column sizing information.
type ColumnSize struct {
	Current float32
	Minimum float32
	Maximum float32
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
	DividerColor             Ink
	RowColor                 Ink
	OnRowColor               Ink
	AltRowColor              Ink
	OnAltRowColor            Ink
	SelectionColor           Ink
	OnSelectionColor         Ink
	Padding                  geom32.Insets
	topLevelRows             []TableRowData
	selMap                   map[TableRowData]bool
	HierarchyColumnIndex     int // The column index that will display the hierarchy
	ColumnSizes              []ColumnSize
	hitRects                 []tableHitRect
	rowCache                 []tableCache
	interactionRow           int
	interactionColumn        int
	columnResizeStart        float32
	columnResizeBase         float32
	columnResizeOverhead     float32
	HierarchyIndent          float32
	MinimumRowHeight         float32
	ColumnResizeSlop         float32
	ShowRowDivider           bool
	ShowColumnDivider        bool
	awaitingSizeColumnsToFit bool
}

// NewTable creates a new Table control.
func NewTable() *Table {
	t := &Table{
		selMap:            make(map[TableRowData]bool),
		Padding:           geom32.NewUniformInsets(4),
		HierarchyIndent:   DefaultHierarchyIndent,
		MinimumRowHeight:  DefaultMinimumRowHeight,
		ColumnResizeSlop:  DefaultColumnResizeSlop,
		ShowRowDivider:    true,
		ShowColumnDivider: true,
	}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	t.UpdateCursorCallback = t.DefaultUpdateCursorCallback
	t.UpdateTooltipCallback = t.DefaultUpdateTooltipCallback
	t.MouseMoveCallback = t.DefaultMouseMove
	t.MouseDownCallback = t.DefaultMouseDown
	t.MouseDragCallback = t.DefaultMouseDrag
	t.MouseUpCallback = t.DefaultMouseUp
	return t
}

// DefaultDraw provides the default drawing.
func (t *Table) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	canvas.DrawRect(dirty, ChooseInk(t.RowColor, ContentColor).Paint(canvas, dirty, Fill))

	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}

	var firstCol int
	x := insets.Left
	for i := range t.ColumnSizes {
		x1 := x + t.ColumnSizes[i].Current
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

	lastY := dirty.Bottom()
	rect := dirty
	rect.Y = y
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		rect.Height = t.rowCache[r].height
		if t.IsRowOrAnyParentSelected(r) {
			canvas.DrawRect(rect, ChooseInk(t.SelectionColor, SelectionColor).Paint(canvas, rect, Fill))
		} else if r%2 == 1 {
			canvas.DrawRect(rect, ChooseInk(t.AltRowColor, BandingColor).Paint(canvas, rect, Fill))
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
		for c := firstCol; c < len(t.ColumnSizes)-1; c++ {
			rect.X += t.ColumnSizes[c].Current
			canvas.DrawRect(rect, ChooseInk(t.DividerColor, DividerColor).Paint(canvas, rect, Fill))
			rect.X++
		}
	}

	rect = dirty
	rect.Y = y
	lastX := dirty.Right()
	t.hitRects = nil
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		row := t.rowCache[r].row
		selected := t.IsRowOrAnyParentSelected(r)
		var fg Ink
		switch {
		case selected:
			fg = ChooseInk(t.OnSelectionColor, OnSelectionColor)
		case row.IsOpen():
			fg = ChooseInk(t.OnAltRowColor, OnBandingColor)
		default:
			fg = ChooseInk(t.OnRowColor, OnContentColor)
		}
		rect.X = x
		rect.Height = t.rowCache[r].height
		for c := firstCol; c < len(t.ColumnSizes) && rect.X < lastX; c++ {
			rect.Width = t.ColumnSizes[c].Current
			cellRect := rect
			cellRect.Inset(t.Padding)
			if c == t.HierarchyColumnIndex {
				if row.CanHaveChildRows() {
					const disclosureIndent = 2
					disclosureSize := mathf32.Min(t.HierarchyIndent, t.MinimumRowHeight) - disclosureIndent*2
					canvas.Save()
					left := cellRect.X + t.HierarchyIndent*float32(t.rowCache[r].depth) + disclosureIndent
					top := cellRect.Y + disclosureIndent - 1 // Unclear why the -1 is needed...
					t.hitRects = append(t.hitRects, t.newTableHitRect(geom32.NewRect(left, top, disclosureSize,
						disclosureSize), row))
					canvas.Translate(left, top)
					if row.IsOpen() {
						offset := disclosureSize / 2
						canvas.Translate(offset, offset)
						canvas.RotateDegrees(90)
						canvas.Translate(-offset, -offset)
					}
					canvas.DrawPath(CircledChevronRightSVG().PathForSize(geom32.NewSize(disclosureSize, disclosureSize)),
						fg.Paint(canvas, cellRect, Fill))
					canvas.Restore()
				}
				indent := t.HierarchyIndent*float32(t.rowCache[r].depth+1) + t.Padding.Left
				cellRect.X += indent
				cellRect.Width -= indent
			}
			cell := row.ColumnCell(c, selected).AsPanel()
			t.installCell(cell, cellRect)
			canvas.Save()
			canvas.Translate(cellRect.X, cellRect.Y)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, cellRect)
			t.uninstallCell(cell)
			canvas.Restore()
			rect.X += t.ColumnSizes[c].Current
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
	cell.ValidateLayout()
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
	for i := range t.ColumnSizes {
		start := end
		end += t.ColumnSizes[i].Current
		if t.ShowColumnDivider {
			end++
		}
		if x >= start && x < end {
			return i
		}
	}
	return -1
}

// OverColumnDivider returns the column index of the column divider that the x coordinate is over, or -1 if it isn't
// over any column divider.
func (t *Table) OverColumnDivider(x float32) int {
	if len(t.ColumnSizes) < 2 {
		return -1
	}
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	pos := insets.Left
	for i := range t.ColumnSizes[:len(t.ColumnSizes)-1] {
		pos += t.ColumnSizes[i].Current
		if t.ShowColumnDivider {
			pos++
		}
		if mathf32.Abs(pos-x) < t.ColumnResizeSlop {
			return i
		}
	}
	return -1
}

// CellFrame returns the frame of the given cell.
func (t *Table) CellFrame(row, col int) geom32.Rect {
	if row < 0 || col < 0 || row >= len(t.rowCache) || col >= len(t.ColumnSizes) {
		return geom32.Rect{}
	}
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	x := insets.Left
	for c := 0; c < col; c++ {
		x += t.ColumnSizes[c].Current
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
	rect := geom32.NewRect(x, y, t.rowCache[row].height, t.ColumnSizes[col].Current)
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

// DefaultUpdateCursorCallback provides the default cursor update handling.
func (t *Table) DefaultUpdateCursorCallback(where geom32.Point) *Cursor {
	if over := t.OverColumnDivider(where.X); over != -1 {
		if t.ColumnSizes[over].Minimum <= 0 || t.ColumnSizes[over].Minimum < t.ColumnSizes[over].Maximum {
			return ArrowsHorizontalCursor()
		}
	}
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.rowCache[row].row.ColumnCell(col, t.IsRowOrAnyParentSelected(row)).AsPanel()
			if cell.UpdateCursorCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				cursor := cell.UpdateCursorCallback(where)
				t.uninstallCell(cell)
				return cursor
			}
		}
	}
	return nil
}

// DefaultUpdateTooltipCallback provides the default tooltip update handling.
func (t *Table) DefaultUpdateTooltipCallback(where geom32.Point, suggestedAvoid geom32.Rect) geom32.Rect {
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.rowCache[row].row.ColumnCell(col, t.IsRowOrAnyParentSelected(row)).AsPanel()
			if cell.UpdateTooltipCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				avoid := cell.UpdateTooltipCallback(where, suggestedAvoid)
				t.Tooltip = cell.Tooltip
				t.uninstallCell(cell)
				return avoid
			}
		}
	}
	t.Tooltip = nil
	return geom32.Rect{}
}

// DefaultMouseMove provides the default mouse move handling.
func (t *Table) DefaultMouseMove(where geom32.Point, mod Modifiers) bool {
	stop := false
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.rowCache[row].row.ColumnCell(col, t.IsRowOrAnyParentSelected(row)).AsPanel()
			if cell.MouseMoveCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				stop = cell.MouseMoveCallback(where, mod)
				t.uninstallCell(cell)
			}
		}
	}
	return stop
}

// DefaultMouseDown provides the default mouse down handling.
func (t *Table) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	t.interactionRow = -1
	t.interactionColumn = -1
	if over := t.OverColumnDivider(where.X); over != -1 {
		if t.ColumnSizes[over].Minimum <= 0 || t.ColumnSizes[over].Minimum < t.ColumnSizes[over].Maximum {
			if clickCount == 2 {
				t.SizeColumnToFit(over, true)
				t.MarkForRedraw()
				t.Window().UpdateCursorNow()
				return true
			}
			t.interactionColumn = over
			t.columnResizeStart = where.X
			t.columnResizeBase = t.ColumnSizes[over].Current
			t.columnResizeOverhead = t.Padding.Left + t.Padding.Right
			if over == t.HierarchyColumnIndex {
				depth := 0
				for _, cache := range t.rowCache {
					if depth < cache.depth {
						depth = cache.depth
					}
				}
				t.columnResizeOverhead += t.Padding.Left + t.HierarchyIndent*float32(depth+1)
			}
			return true
		}
	}
	for _, one := range t.hitRects {
		if one.ContainsPoint(where) {
			one.handler(where, button, clickCount, mod)
			return true
		}
	}
	stop := true
	if row := t.OverRow(where.Y); row != -1 {
		if mod&(ShiftModifier|OptionModifier) == 0 {
			t.selMap = make(map[TableRowData]bool)
		}
		rowData := t.rowCache[row].row
		if mod&OptionModifier != 0 {
			t.selMap[rowData] = !t.selMap[rowData]
		} else {
			t.selMap[rowData] = true
		}
		t.MarkForRedraw()
		if col := t.OverColumn(where.X); col != -1 {
			cell := rowData.ColumnCell(col, t.IsRowOrAnyParentSelected(row)).AsPanel()
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
	if t.interactionColumn != -1 {
		if t.interactionRow == -1 {
			width := t.columnResizeBase + where.X - t.columnResizeStart
			if width < t.columnResizeOverhead {
				width = t.columnResizeOverhead
			}
			min := t.ColumnSizes[t.interactionColumn].Minimum
			if min > 0 && width < min+t.columnResizeOverhead {
				width = min + t.columnResizeOverhead
			} else {
				max := t.ColumnSizes[t.interactionColumn].Maximum
				if max > 0 && width > max+t.columnResizeOverhead {
					width = max + t.columnResizeOverhead
				}
			}
			if t.ColumnSizes[t.interactionColumn].Current != width {
				t.ColumnSizes[t.interactionColumn].Current = width
				t.SyncToModel()
				t.MarkForRedraw()
			}
			stop = true
		} else {
			cell := t.rowCache[t.interactionRow].row.ColumnCell(t.interactionColumn, t.IsRowOrAnyParentSelected(t.interactionRow)).AsPanel()
			if cell.MouseDragCallback != nil {
				rect := t.CellFrame(t.interactionRow, t.interactionColumn)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				stop = cell.MouseDragCallback(where, button, mod)
				t.uninstallCell(cell)
			}
		}
	}
	return stop
}

// DefaultMouseUp provides the default mouse up handling.
func (t *Table) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	stop := false
	if t.interactionRow != -1 && t.interactionColumn != -1 {
		cell := t.rowCache[t.interactionRow].row.ColumnCell(t.interactionColumn, t.IsRowOrAnyParentSelected(t.interactionRow)).AsPanel()
		if cell.MouseUpCallback != nil {
			rect := t.CellFrame(t.interactionRow, t.interactionColumn)
			t.installCell(cell, rect)
			where.Subtract(rect.Point)
			stop = cell.MouseUpCallback(where, button, mod)
			t.uninstallCell(cell)
		}
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
	t.rowCache[index].height = t.heightForColumns(row, depth, t.IsRowOrAnyParentSelected(index))
	parentIndex = index
	index++
	if row.CanHaveChildRows() && row.IsOpen() {
		for _, child := range row.ChildRows() {
			index = t.buildRowCacheEntry(child, parentIndex, index, depth+1)
		}
	}
	return index
}

func (t *Table) heightForColumns(row TableRowData, depth int, selected bool) float32 {
	var height float32
	for i := range t.ColumnSizes {
		w := t.ColumnSizes[i].Current
		if w <= 0 {
			continue
		}
		w -= t.Padding.Left + t.Padding.Right
		if i == t.HierarchyColumnIndex {
			w -= t.Padding.Left + t.HierarchyIndent*float32(depth+1)
		}
		_, cpref, _ := row.ColumnCell(i, selected).AsPanel().Sizes(geom32.Size{Width: w})
		cpref.Height += t.Padding.Top + t.Padding.Bottom
		if height < cpref.Height {
			height = cpref.Height
		}
	}
	return mathf32.Max(mathf32.Ceil(height), t.MinimumRowHeight)
}

// SizeColumnsToFit sizes each column to its preferred size. If 'adjust' is true, the Table's FrameRect will be set to
// its preferred size as well.
func (t *Table) SizeColumnsToFit(adjust bool) {
	for i := range t.ColumnSizes {
		initial := t.ColumnSizes[i].Minimum
		if initial < 0 {
			initial = 0
		}
		t.ColumnSizes[i].Current = initial
	}
	for rowIndex, cache := range t.rowCache {
		selected := t.IsRowOrAnyParentSelected(rowIndex)
		for i := range t.ColumnSizes {
			_, pref, _ := cache.row.ColumnCell(i, selected).AsPanel().Sizes(geom32.Size{})
			min := t.ColumnSizes[i].Minimum
			if min > 0 && pref.Width < min {
				pref.Width = min
			} else {
				max := t.ColumnSizes[i].Maximum
				if max > 0 && pref.Width > max {
					pref.Width = max
				}
			}
			pref.Width += t.Padding.Left + t.Padding.Right
			if i == t.HierarchyColumnIndex {
				pref.Width += t.Padding.Left + t.HierarchyIndent*float32(cache.depth+1)
			}
			if t.ColumnSizes[i].Current < pref.Width {
				t.ColumnSizes[i].Current = pref.Width
			}
		}
	}
	for i, cache := range t.rowCache {
		t.rowCache[i].height = t.heightForColumns(cache.row, cache.depth, t.IsRowOrAnyParentSelected(i))
	}
	if adjust {
		_, pref, _ := t.DefaultSizes(geom32.Size{})
		rect := t.FrameRect()
		rect.Size = pref
		t.SetFrameRect(rect)
	}
}

// SizeColumnToFit sizes the specified column to its preferred size. If 'adjust' is true, the Table's FrameRect will be
// set to its preferred size as well.
func (t *Table) SizeColumnToFit(index int, adjust bool) {
	if index < 0 || index >= len(t.ColumnSizes) {
		return
	}
	t.ColumnSizes[index].Current = t.ColumnSizes[index].Minimum
	for rowIndex, cache := range t.rowCache {
		_, pref, _ := cache.row.ColumnCell(index, t.IsRowOrAnyParentSelected(rowIndex)).AsPanel().Sizes(geom32.Size{})
		min := t.ColumnSizes[index].Minimum
		if min > 0 && pref.Width < min {
			pref.Width = min
		} else {
			max := t.ColumnSizes[index].Maximum
			if max > 0 && pref.Width > max {
				pref.Width = max
			}
		}
		pref.Width += t.Padding.Left + t.Padding.Right
		if index == t.HierarchyColumnIndex {
			pref.Width += t.Padding.Left + t.HierarchyIndent*float32(cache.depth+1)
		}
		if t.ColumnSizes[index].Current < pref.Width {
			t.ColumnSizes[index].Current = pref.Width
		}
	}
	for i, cache := range t.rowCache {
		t.rowCache[i].height = t.heightForColumns(cache.row, cache.depth, t.IsRowOrAnyParentSelected(i))
	}
	if adjust {
		_, pref, _ := t.DefaultSizes(geom32.Size{})
		rect := t.FrameRect()
		rect.Size = pref
		t.SetFrameRect(rect)
	}
}

// EventuallySizeColumnsToFit sizes each column to its preferred size after a short delay, allowing multiple
// back-to-back calls to this function to only do work once. If 'adjust' is true, the Table's FrameRect will be set to
// its preferred size as well.
func (t *Table) EventuallySizeColumnsToFit(adjust bool) {
	if !t.awaitingSizeColumnsToFit {
		t.awaitingSizeColumnsToFit = true
		InvokeTaskAfter(func() {
			t.SizeColumnsToFit(adjust)
			t.awaitingSizeColumnsToFit = false
		}, 20*time.Millisecond)
	}
}

// DefaultSizes provides the default sizing.
func (t *Table) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	for i := range t.ColumnSizes {
		pref.Width += t.ColumnSizes[i].Current
	}
	for _, cache := range t.rowCache {
		pref.Height += cache.height
	}
	if t.ShowColumnDivider {
		pref.Width += float32(len(t.ColumnSizes) - 1)
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
