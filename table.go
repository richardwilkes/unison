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

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom"
)

// TableRowData provides information about a single row of data.
type TableRowData interface {
	// ParentRow returns the parent of this row, or nil if it is a root node.
	ParentRow() TableRowData
	// CanHaveChildRows returns true if this row can have children, even if it currently does not have any.
	CanHaveChildRows() bool
	// ChildRows returns the child rows.
	ChildRows() []TableRowData
	// ColumnCell returns the panel that should be placed at the position of the cell for the given column index. If you
	// need for the cell to retain widget state, make sure to return the same widget each time rather than creating a
	// new one.
	ColumnCell(row, col int, foreground, background Ink, selected, indirectlySelected, focused bool) Paneler
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
	// CellDataForSort returns the string that represents the data in the specified cell.
	CellDataForSort(index int) string
}

// TableDragData holds the data from a table row drag.
type TableDragData struct {
	Table *Table
	Rows  []TableRowData
}

// ColumnSize holds the column sizing information.
type ColumnSize struct {
	Current     float32
	Minimum     float32
	Maximum     float32
	AutoMinimum float32
	AutoMaximum float32
}

type tableCache struct {
	row    TableRowData
	parent int
	depth  int
	height float32
}

type tableHitRect struct {
	Rect
	handler func(where Point, button, clickCount int, mod Modifiers)
}

// DefaultTableTheme holds the default TableTheme values for Tables. Modifying this data will not alter existing Tables,
// but will alter any Tables created in the future.
var DefaultTableTheme = TableTheme{
	BackgroundInk:          ContentColor,
	OnBackgroundInk:        OnContentColor,
	BandingInk:             BandingColor,
	OnBandingInk:           OnBandingColor,
	DividerInk:             DividerColor,
	SelectionInk:           SelectionColor,
	OnSelectionInk:         OnSelectionColor,
	InactiveSelectionInk:   InactiveSelectionColor,
	OnInactiveSelectionInk: OnInactiveSelectionColor,
	IndirectSelectionInk:   IndirectSelectionColor,
	OnIndirectSelectionInk: OnIndirectSelectionColor,
	Padding:                NewUniformInsets(4),
	HierarchyColumnIndex:   0,
	HierarchyIndent:        16,
	MinimumRowHeight:       16,
	ColumnResizeSlop:       4,
	ShowRowDivider:         true,
	ShowColumnDivider:      true,
}

// TableTheme holds theming data for a Table.
type TableTheme struct {
	BackgroundInk          Ink
	OnBackgroundInk        Ink
	BandingInk             Ink
	OnBandingInk           Ink
	DividerInk             Ink
	SelectionInk           Ink
	OnSelectionInk         Ink
	InactiveSelectionInk   Ink
	OnInactiveSelectionInk Ink
	IndirectSelectionInk   Ink
	OnIndirectSelectionInk Ink
	Padding                Insets
	HierarchyColumnIndex   int
	HierarchyIndent        float32
	MinimumRowHeight       float32
	ColumnResizeSlop       float32
	ShowRowDivider         bool
	ShowColumnDivider      bool
}

// Table provides a control that can display data in columns and rows.
type Table struct {
	Panel
	TableTheme
	SelectionDoubleClickCallback func()
	ColumnSizes                  []ColumnSize
	topLevelRows                 []TableRowData
	selMap                       map[TableRowData]bool
	selAnchor                    TableRowData
	hitRects                     []tableHitRect
	rowCache                     []tableCache
	interactionRow               int
	interactionColumn            int
	lastMouseMotionRow           int
	lastMouseMotionColumn        int
	columnResizeStart            float32
	columnResizeBase             float32
	columnResizeOverhead         float32
	PreventUserColumnResize      bool
	awaitingSizeColumnsToFit     bool
	awaitingSyncToModel          bool
}

// NewTable creates a new Table control.
func NewTable() *Table {
	t := &Table{
		TableTheme:            DefaultTableTheme,
		selMap:                make(map[TableRowData]bool),
		interactionRow:        -1,
		interactionColumn:     -1,
		lastMouseMotionRow:    -1,
		lastMouseMotionColumn: -1,
	}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	t.GainedFocusCallback = t.DefaultFocusGained
	t.DrawCallback = t.DefaultDraw
	t.UpdateCursorCallback = t.DefaultUpdateCursorCallback
	t.UpdateTooltipCallback = t.DefaultUpdateTooltipCallback
	t.MouseMoveCallback = t.DefaultMouseMove
	t.MouseDownCallback = t.DefaultMouseDown
	t.MouseDragCallback = t.DefaultMouseDrag
	t.MouseUpCallback = t.DefaultMouseUp
	t.MouseEnterCallback = t.DefaultMouseEnter
	t.MouseExitCallback = t.DefaultMouseExit
	t.InstallCmdHandlers(SelectAllItemID, AlwaysEnabled, func(_ any) { t.SelectAll() })
	return t
}

// DefaultDraw provides the default drawing.
func (t *Table) DefaultDraw(canvas *Canvas, dirty Rect) {
	selectionInk := t.SelectionInk
	if !t.Focused() {
		selectionInk = t.InactiveSelectionInk
	}

	canvas.DrawRect(dirty, t.BackgroundInk.Paint(canvas, dirty, Fill))

	var insets Insets
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
			if t.IsRowSelected(r) {
				canvas.DrawRect(rect, selectionInk.Paint(canvas, rect, Fill))
			} else {
				canvas.DrawRect(rect, t.IndirectSelectionInk.Paint(canvas, rect, Fill))
			}
		} else if r%2 == 1 {
			canvas.DrawRect(rect, t.BandingInk.Paint(canvas, rect, Fill))
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider && r != rowCount-1 {
			rect.Height = 1
			canvas.DrawRect(rect, t.DividerInk.Paint(canvas, rect, Fill))
			rect.Y++
		}
	}

	if t.ShowColumnDivider {
		rect = dirty
		rect.X = x
		rect.Width = 1
		for c := firstCol; c < len(t.ColumnSizes)-1; c++ {
			rect.X += t.ColumnSizes[c].Current
			canvas.DrawRect(rect, t.DividerInk.Paint(canvas, rect, Fill))
			rect.X++
		}
	}

	rect = dirty
	rect.Y = y
	lastX := dirty.Right()
	t.hitRects = nil
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		rect.X = x
		rect.Height = t.rowCache[r].height
		for c := firstCol; c < len(t.ColumnSizes) && rect.X < lastX; c++ {
			fg, bg, selected, indirectlySelected, focused := t.cellParams(r, c)
			rect.Width = t.ColumnSizes[c].Current
			cellRect := rect
			cellRect.Inset(t.Padding)
			row := t.rowCache[r].row
			if c == t.HierarchyColumnIndex {
				if row.CanHaveChildRows() {
					const disclosureIndent = 2
					disclosureSize := xmath.Min(t.HierarchyIndent, t.MinimumRowHeight) - disclosureIndent*2
					canvas.Save()
					left := cellRect.X + t.HierarchyIndent*float32(t.rowCache[r].depth) + disclosureIndent
					top := cellRect.Y + (t.MinimumRowHeight-disclosureSize)/2
					t.hitRects = append(t.hitRects, t.newTableHitRect(NewRect(left, top, disclosureSize,
						disclosureSize), row))
					canvas.Translate(left, top)
					if row.IsOpen() {
						offset := disclosureSize / 2
						canvas.Translate(offset, offset)
						canvas.Rotate(90)
						canvas.Translate(-offset, -offset)
					}
					canvas.DrawPath(CircledChevronRightSVG().PathForSize(NewSize(disclosureSize, disclosureSize)),
						fg.Paint(canvas, cellRect, Fill))
					canvas.Restore()
				}
				indent := t.HierarchyIndent*float32(t.rowCache[r].depth+1) + t.Padding.Left
				cellRect.X += indent
				cellRect.Width -= indent
			}
			cell := row.ColumnCell(r, c, fg, bg, selected, indirectlySelected, focused).AsPanel()
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

func (t *Table) cellParams(row, col int) (fg, bg Ink, selected, indirectlySelected, focused bool) {
	focused = t.Focused()
	selected = t.IsRowSelected(row)
	indirectlySelected = !selected && t.IsRowOrAnyParentSelected(row)
	switch {
	case selected && focused:
		fg = t.OnSelectionInk
		bg = t.SelectionInk
	case selected:
		fg = t.OnInactiveSelectionInk
		bg = t.InactiveSelectionInk
	case indirectlySelected:
		fg = t.OnIndirectSelectionInk
		bg = t.IndirectSelectionInk
	case row%2 == 1:
		fg = t.OnBandingInk
		bg = t.BandingInk
	default:
		fg = t.OnBackgroundInk
		bg = t.BackgroundInk
	}
	return fg, bg, selected, indirectlySelected, focused
}

func (t *Table) cell(row, col int) *Panel {
	fg, bg, selected, indirectlySelected, focused := t.cellParams(row, col)
	return t.rowCache[row].row.ColumnCell(row, col, fg, bg, selected, indirectlySelected, focused).AsPanel()
}

func (t *Table) installCell(cell *Panel, frame Rect) {
	cell.SetFrameRect(frame)
	cell.ValidateLayout()
	cell.parent = t.AsPanel()
}

func (t *Table) uninstallCell(cell *Panel) {
	cell.parent = nil
}

// OverRow returns the row index that the y coordinate is over, or -1 if it isn't over any row.
func (t *Table) OverRow(y float32) int {
	var insets Insets
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
	var insets Insets
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
	var insets Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	pos := insets.Left
	for i := range t.ColumnSizes[:len(t.ColumnSizes)-1] {
		pos += t.ColumnSizes[i].Current
		if t.ShowColumnDivider {
			pos++
		}
		if xmath.Abs(pos-x) < t.ColumnResizeSlop {
			return i
		}
	}
	return -1
}

// CellWidth returns the current width of a given cell.
func (t *Table) CellWidth(row, col int) float32 {
	if row < 0 || col < 0 || row >= len(t.rowCache) || col >= len(t.ColumnSizes) {
		return 0
	}
	width := t.ColumnSizes[col].Current - (t.Padding.Left + t.Padding.Right)
	if col == t.HierarchyColumnIndex {
		width -= t.HierarchyIndent*float32(t.rowCache[row].depth+1) + t.Padding.Left
	}
	return width
}

// ColumnEdges returns the x-coordinates of the left and right sides of the column.
func (t *Table) ColumnEdges(col int) (left, right float32) {
	if col < 0 || col >= len(t.ColumnSizes) {
		return 0, 0
	}
	var insets Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	left = insets.Left
	for c := 0; c < col; c++ {
		left += t.ColumnSizes[c].Current
		if t.ShowColumnDivider {
			left++
		}
	}
	right = left + t.ColumnSizes[col].Current
	left += t.Padding.Left
	right -= t.Padding.Right
	if col == t.HierarchyColumnIndex {
		left += t.HierarchyIndent + t.Padding.Left
	}
	if right < left {
		right = left
	}
	return left, right
}

// CellFrame returns the frame of the given cell.
func (t *Table) CellFrame(row, col int) Rect {
	if row < 0 || col < 0 || row >= len(t.rowCache) || col >= len(t.ColumnSizes) {
		return Rect{}
	}
	var insets Insets
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
	rect := NewRect(x, y, t.ColumnSizes[col].Current, t.rowCache[row].height)
	rect.Inset(t.Padding)
	if col == t.HierarchyColumnIndex {
		indent := t.HierarchyIndent*float32(t.rowCache[row].depth+1) + t.Padding.Left
		rect.X += indent
		rect.Width -= indent
		if rect.Width < 1 {
			rect.Width = 1
		}
	}
	return rect
}

// RowFrame returns the frame of the row.
func (t *Table) RowFrame(row int) Rect {
	if row < 0 || row >= len(t.rowCache) {
		return Rect{}
	}
	rect := t.ContentRect(false)
	for i := 0; i < row; i++ {
		rect.Y += t.rowCache[i].height
		if t.ShowRowDivider {
			rect.Y++
		}
	}
	rect.Height = t.rowCache[row].height
	return rect
}

func (t *Table) newTableHitRect(rect Rect, row TableRowData) tableHitRect {
	return tableHitRect{
		Rect: rect,
		handler: func(where Point, button, clickCount int, mod Modifiers) {
			row.SetOpen(!row.IsOpen())
			t.SyncToModel()
			t.MarkForRedraw()
		},
	}
}

// DefaultFocusGained provides the default focus gained handling.
func (t *Table) DefaultFocusGained() {
	switch {
	case t.interactionRow != -1:
		t.ScrollRowIntoView(t.interactionRow)
	case t.lastMouseMotionRow != -1:
		t.ScrollRowIntoView(t.lastMouseMotionRow)
	default:
		t.ScrollIntoView()
	}
	t.MarkForRedraw()
}

// DefaultUpdateCursorCallback provides the default cursor update handling.
func (t *Table) DefaultUpdateCursorCallback(where Point) *Cursor {
	if !t.PreventUserColumnResize {
		if over := t.OverColumnDivider(where.X); over != -1 {
			if t.ColumnSizes[over].Minimum <= 0 || t.ColumnSizes[over].Minimum < t.ColumnSizes[over].Maximum {
				return ResizeHorizontalCursor()
			}
		}
	}
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.UpdateCursorCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				var cursor *Cursor
				toolbox.Call(func() { cursor = cell.UpdateCursorCallback(where) })
				t.uninstallCell(cell)
				return cursor
			}
		}
	}
	return nil
}

// DefaultUpdateTooltipCallback provides the default tooltip update handling.
func (t *Table) DefaultUpdateTooltipCallback(where Point, suggestedAvoidInRoot Rect) Rect {
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.UpdateTooltipCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				var avoid Rect
				toolbox.Call(func() { avoid = cell.UpdateTooltipCallback(where, suggestedAvoidInRoot) })
				t.Tooltip = cell.Tooltip
				t.uninstallCell(cell)
				return avoid
			}
			if cell.Tooltip != nil {
				t.Tooltip = cell.Tooltip
				suggestedAvoidInRoot = t.RectToRoot(t.CellFrame(row, col))
				suggestedAvoidInRoot.Align()
				return suggestedAvoidInRoot
			}
		}
	}
	t.Tooltip = nil
	return Rect{}
}

// DefaultMouseEnter provides the default mouse enter handling.
func (t *Table) DefaultMouseEnter(where Point, mod Modifiers) bool {
	stop := false
	row := t.OverRow(where.Y)
	col := t.OverColumn(where.X)
	if row != -1 && col != -1 {
		if t.lastMouseMotionRow != row || t.lastMouseMotionColumn != col {
			t.DefaultMouseExit()
			t.lastMouseMotionRow = row
			t.lastMouseMotionColumn = col
			cell := t.cell(row, col)
			if cell.MouseEnterCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				toolbox.Call(func() { stop = cell.MouseEnterCallback(where, mod) })
				t.uninstallCell(cell)
			}
		}
	} else {
		t.DefaultMouseExit()
	}
	return stop
}

// DefaultMouseExit provides the default mouse exit handling.
func (t *Table) DefaultMouseExit() bool {
	stop := false
	if t.lastMouseMotionColumn != -1 && t.lastMouseMotionRow >= 0 && t.lastMouseMotionRow < len(t.rowCache) {
		cell := t.cell(t.lastMouseMotionRow, t.lastMouseMotionColumn)
		if cell.MouseExitCallback != nil {
			t.installCell(cell, t.CellFrame(t.lastMouseMotionRow, t.lastMouseMotionColumn))
			toolbox.Call(func() { stop = cell.MouseExitCallback() })
			t.uninstallCell(cell)
		}
	}
	t.lastMouseMotionRow = -1
	t.lastMouseMotionColumn = -1
	return stop
}

// DefaultMouseMove provides the default mouse move handling.
func (t *Table) DefaultMouseMove(where Point, mod Modifiers) bool {
	t.DefaultMouseEnter(where, mod)
	stop := false
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.MouseMoveCallback != nil {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				toolbox.Call(func() { stop = cell.MouseMoveCallback(where, mod) })
				t.uninstallCell(cell)
			}
		}
	}
	return stop
}

// DefaultMouseDown provides the default mouse down handling.
func (t *Table) DefaultMouseDown(where Point, button, clickCount int, mod Modifiers) bool {
	t.interactionRow = -1
	t.interactionColumn = -1
	if !t.PreventUserColumnResize {
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
	}
	for _, one := range t.hitRects {
		if one.ContainsPoint(where) {
			one.handler(where, button, clickCount, mod)
			return true
		}
	}
	stop := true
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.MouseDownCallback != nil {
				t.interactionRow = row
				t.interactionColumn = col
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				toolbox.Call(func() { stop = cell.MouseDownCallback(where, button, clickCount, mod) })
				t.uninstallCell(cell)
				if stop {
					return stop
				}
			}
		}
		rowData := t.rowCache[row].row
		switch {
		case mod&ShiftModifier != 0: // Extend selection from anchor
			selAnchorIndex := -1
			if t.selAnchor != nil {
				for i, c := range t.rowCache {
					if c.row == t.selAnchor {
						selAnchorIndex = i
						break
					}
				}
			}
			if selAnchorIndex != -1 {
				last := xmath.Max(selAnchorIndex, row)
				for i := xmath.Min(selAnchorIndex, row); i <= last; i++ {
					t.selMap[t.rowCache[i].row] = true
				}
			} else if !t.selMap[rowData] { // No anchor, so behave like a regular click
				t.selMap = make(map[TableRowData]bool)
				t.selMap[rowData] = true
				t.selAnchor = rowData
			}
		case mod&(OptionModifier|CommandModifier) != 0: // Toggle single row
			if t.selMap[rowData] {
				delete(t.selMap, rowData)
			} else {
				t.selMap[rowData] = true
			}
		default: // If not already selected, replace selection with current row and make it the anchor
			if !t.selMap[rowData] {
				t.selMap = make(map[TableRowData]bool)
				t.selMap[rowData] = true
				t.selAnchor = rowData
			}
		}
		t.MarkForRedraw()
		if clickCount == 2 && t.SelectionDoubleClickCallback != nil && len(t.selMap) != 0 {
			toolbox.Call(t.SelectionDoubleClickCallback)
		}
	}
	return stop
}

// DefaultMouseDrag provides the default mouse drag handling.
func (t *Table) DefaultMouseDrag(where Point, button int, mod Modifiers) bool {
	stop := false
	if t.interactionColumn != -1 {
		if t.interactionRow == -1 {
			if !t.PreventUserColumnResize {
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
					t.EventuallySyncToModel()
					t.MarkForRedraw()
				}
				stop = true
			}
		} else {
			cell := t.cell(t.interactionRow, t.interactionColumn)
			if cell.MouseDragCallback != nil {
				rect := t.CellFrame(t.interactionRow, t.interactionColumn)
				t.installCell(cell, rect)
				where.Subtract(rect.Point)
				toolbox.Call(func() { stop = cell.MouseDragCallback(where, button, mod) })
				t.uninstallCell(cell)
			}
		}
	}
	return stop
}

// DefaultMouseUp provides the default mouse up handling.
func (t *Table) DefaultMouseUp(where Point, button int, mod Modifiers) bool {
	stop := false
	if t.interactionRow != -1 && t.interactionColumn != -1 {
		cell := t.cell(t.interactionRow, t.interactionColumn)
		if cell.MouseUpCallback != nil {
			rect := t.CellFrame(t.interactionRow, t.interactionColumn)
			t.installCell(cell, rect)
			where.Subtract(rect.Point)
			toolbox.Call(func() { stop = cell.MouseUpCallback(where, button, mod) })
			t.uninstallCell(cell)
		}
	}
	return stop
}

// FirstSelectedRowIndex returns the first selected row index, or -1 if there is no selection.
func (t *Table) FirstSelectedRowIndex() int {
	if len(t.selMap) == 0 {
		return -1
	}
	for i, entry := range t.rowCache {
		if t.selMap[entry.row] {
			return i
		}
	}
	return -1
}

// LastSelectedRowIndex returns the last selected row index, or -1 if there is no selection.
func (t *Table) LastSelectedRowIndex() int {
	if len(t.selMap) == 0 {
		return -1
	}
	for i := len(t.rowCache) - 1; i >= 0; i-- {
		if t.selMap[t.rowCache[i].row] {
			return i
		}
	}
	return -1
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

// IsRowSelected returns true if the specified row index is selected.
func (t *Table) IsRowSelected(index int) bool {
	if index < 0 || index >= len(t.rowCache) {
		return false
	}
	return t.selMap[t.rowCache[index].row]
}

// SelectedRows returns the currently selected rows. If 'minimal' is true, then children of selected rows that may also
// be selected are not returned, just the topmost row that is selected in any given hierarchy.
func (t *Table) SelectedRows(minimal bool) []TableRowData {
	if len(t.selMap) == 0 {
		return nil
	}
	rows := make([]TableRowData, 0, len(t.selMap))
	for _, entry := range t.rowCache {
		if t.selMap[entry.row] && (!minimal || entry.parent == -1 || !t.IsRowOrAnyParentSelected(entry.parent)) {
			rows = append(rows, entry.row)
		}
	}
	return rows
}

// HasSelection returns true if there is a selection.
func (t *Table) HasSelection() bool {
	return len(t.selMap) != 0
}

// ClearSelection clears the selection.
func (t *Table) ClearSelection() {
	if len(t.selMap) == 0 {
		return
	}
	t.selMap = make(map[TableRowData]bool)
	t.selAnchor = nil
	t.MarkForRedraw()
}

// SelectAll selects all rows.
func (t *Table) SelectAll() {
	t.selMap = make(map[TableRowData]bool, len(t.rowCache))
	t.selAnchor = nil
	for _, cache := range t.rowCache {
		t.selMap[cache.row] = true
		if t.selAnchor == nil {
			t.selAnchor = cache.row
		}
	}
	t.MarkForRedraw()
}

// SelectByIndex selects the given indexes. The first one will be considered the anchor selection if no existing anchor
// selection exists.
func (t *Table) SelectByIndex(indexes ...int) {
	for _, index := range indexes {
		if index >= 0 && index < len(t.rowCache) {
			t.selMap[t.rowCache[index].row] = true
			if t.selAnchor == nil {
				t.selAnchor = t.rowCache[index].row
			}
		}
	}
	t.MarkForRedraw()
}

// DeselectByIndex deselects the given indexes.
func (t *Table) DeselectByIndex(indexes ...int) {
	for _, index := range indexes {
		if index >= 0 && index < len(t.rowCache) {
			delete(t.selMap, t.rowCache[index].row)
		}
	}
	t.MarkForRedraw()
}

// DiscloseRow ensures the given row can be viewed by opening all parents that lead to it. Returns true if any
// modification was made.
func (t *Table) DiscloseRow(row TableRowData, delaySync bool) bool {
	modified := false
	p := row.ParentRow()
	for !toolbox.IsNil(p) {
		if !p.IsOpen() {
			p.SetOpen(true)
			modified = true
		}
		p = p.ParentRow()
	}
	if modified {
		if delaySync {
			t.EventuallySyncToModel()
		} else {
			t.SyncToModel()
		}
	}
	return modified
}

// TopLevelRowCount returns the number of top-level rows.
func (t *Table) TopLevelRowCount() int {
	return len(t.topLevelRows)
}

// TopLevelRows returns the top-level rows.
func (t *Table) TopLevelRows() []TableRowData {
	rows := make([]TableRowData, len(t.topLevelRows))
	copy(rows, t.topLevelRows)
	return rows
}

// SetTopLevelRows sets the top-level rows this table will display. This will call SyncToModel() automatically.
func (t *Table) SetTopLevelRows(rows []TableRowData) {
	t.topLevelRows = rows
	t.selMap = make(map[TableRowData]bool)
	t.selAnchor = nil
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
	_, pref, _ := t.DefaultSizes(Size{})
	rect := t.FrameRect()
	rect.Size = pref
	t.SetFrameRect(rect)
	t.MarkForRedraw()
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
	t.rowCache[index].height = t.heightForColumns(row, index, depth)
	parentIndex = index
	index++
	if row.CanHaveChildRows() && row.IsOpen() {
		for _, child := range row.ChildRows() {
			index = t.buildRowCacheEntry(child, parentIndex, index, depth+1)
		}
	}
	return index
}

func (t *Table) heightForColumns(rowData TableRowData, row, depth int) float32 {
	var height float32
	for col := range t.ColumnSizes {
		w := t.ColumnSizes[col].Current
		if w <= 0 {
			continue
		}
		w -= t.Padding.Left + t.Padding.Right
		if col == t.HierarchyColumnIndex {
			w -= t.Padding.Left + t.HierarchyIndent*float32(depth+1)
		}
		size := t.cellPrefSize(rowData, row, col, w)
		size.Height += t.Padding.Top + t.Padding.Bottom
		if height < size.Height {
			height = size.Height
		}
	}
	return xmath.Max(xmath.Ceil(height), t.MinimumRowHeight)
}

func (t *Table) cellPrefSize(rowData TableRowData, row, col int, widthConstraint float32) geom.Size[float32] {
	fg, bg, selected, indirectlySelected, focused := t.cellParams(row, col)
	cell := rowData.ColumnCell(row, col, fg, bg, selected, indirectlySelected, focused).AsPanel()
	_, size, _ := cell.Sizes(Size{Width: widthConstraint})
	return size
}

// SizeColumnsToFitWithExcessIn sizes each column to its preferred size, with the exception of the 'excessColumnIndex',
// which gets set to any remaining width left over. Pass in -1 for the 'excessColumnIndex' to use the
// HierarchyColumnIndex or 0, if the HierarchyColumnIndex is less than 0
func (t *Table) SizeColumnsToFitWithExcessIn(excessColumnIndex int) {
	if excessColumnIndex < 0 {
		excessColumnIndex = t.HierarchyColumnIndex
		if excessColumnIndex < 0 {
			excessColumnIndex = 0
		}
	}
	current := make([]float32, len(t.ColumnSizes))
	for col := range t.ColumnSizes {
		current[col] = xmath.Max(t.ColumnSizes[col].Minimum, 0)
		t.ColumnSizes[col].Current = 0
	}
	for row, cache := range t.rowCache {
		for col := range t.ColumnSizes {
			if col == excessColumnIndex {
				continue
			}
			pref := t.cellPrefSize(cache.row, row, col, 0)
			min := t.ColumnSizes[col].AutoMinimum
			if min > 0 && pref.Width < min {
				pref.Width = min
			} else {
				max := t.ColumnSizes[col].AutoMaximum
				if max > 0 && pref.Width > max {
					pref.Width = max
				}
			}
			pref.Width += t.Padding.Left + t.Padding.Right
			if col == t.HierarchyColumnIndex {
				pref.Width += t.Padding.Left + t.HierarchyIndent*float32(cache.depth+1)
			}
			if current[col] < pref.Width {
				current[col] = pref.Width
			}
		}
	}
	width := t.ContentRect(false).Width
	if t.ShowColumnDivider {
		width -= float32(len(t.ColumnSizes) - 1)
	}
	for col := range current {
		if col == excessColumnIndex {
			continue
		}
		t.ColumnSizes[col].Current = current[col]
		width -= current[col]
	}
	t.ColumnSizes[excessColumnIndex].Current = xmath.Max(width, t.ColumnSizes[excessColumnIndex].Minimum)
	for row, cache := range t.rowCache {
		t.rowCache[row].height = t.heightForColumns(cache.row, row, cache.depth)
	}
}

// SizeColumnsToFit sizes each column to its preferred size. If 'adjust' is true, the Table's FrameRect will be set to
// its preferred size as well.
func (t *Table) SizeColumnsToFit(adjust bool) {
	current := make([]float32, len(t.ColumnSizes))
	for col := range t.ColumnSizes {
		current[col] = xmath.Max(t.ColumnSizes[col].Minimum, 0)
		t.ColumnSizes[col].Current = 0
	}
	for row, cache := range t.rowCache {
		for col := range t.ColumnSizes {
			pref := t.cellPrefSize(cache.row, row, col, 0)
			min := t.ColumnSizes[col].AutoMinimum
			if min > 0 && pref.Width < min {
				pref.Width = min
			} else {
				max := t.ColumnSizes[col].AutoMaximum
				if max > 0 && pref.Width > max {
					pref.Width = max
				}
			}
			pref.Width += t.Padding.Left + t.Padding.Right
			if col == t.HierarchyColumnIndex {
				pref.Width += t.Padding.Left + t.HierarchyIndent*float32(cache.depth+1)
			}
			if current[col] < pref.Width {
				current[col] = pref.Width
			}
		}
	}
	for col := range current {
		t.ColumnSizes[col].Current = current[col]
	}
	for row, cache := range t.rowCache {
		t.rowCache[row].height = t.heightForColumns(cache.row, row, cache.depth)
	}
	if adjust {
		_, pref, _ := t.DefaultSizes(Size{})
		rect := t.FrameRect()
		rect.Size = pref
		t.SetFrameRect(rect)
	}
}

// SizeColumnToFit sizes the specified column to its preferred size. If 'adjust' is true, the Table's FrameRect will be
// set to its preferred size as well.
func (t *Table) SizeColumnToFit(col int, adjust bool) {
	if col < 0 || col >= len(t.ColumnSizes) {
		return
	}
	current := xmath.Max(t.ColumnSizes[col].Minimum, 0)
	t.ColumnSizes[col].Current = 0
	for row, cache := range t.rowCache {
		pref := t.cellPrefSize(cache.row, row, col, 0)
		min := t.ColumnSizes[col].AutoMinimum
		if min > 0 && pref.Width < min {
			pref.Width = min
		} else {
			max := t.ColumnSizes[col].AutoMaximum
			if max > 0 && pref.Width > max {
				pref.Width = max
			}
		}
		pref.Width += t.Padding.Left + t.Padding.Right
		if col == t.HierarchyColumnIndex {
			pref.Width += t.Padding.Left + t.HierarchyIndent*float32(cache.depth+1)
		}
		if current < pref.Width {
			current = pref.Width
		}
	}
	t.ColumnSizes[col].Current = current
	for row, cache := range t.rowCache {
		t.rowCache[row].height = t.heightForColumns(cache.row, row, cache.depth)
	}
	if adjust {
		_, pref, _ := t.DefaultSizes(Size{})
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

// EventuallySyncToModel syncs the table to its underlying model after a short delay, allowing multiple back-to-back
// calls to this function to only do work once.
func (t *Table) EventuallySyncToModel() {
	if !t.awaitingSyncToModel {
		t.awaitingSyncToModel = true
		InvokeTaskAfter(func() {
			t.SyncToModel()
			t.awaitingSyncToModel = false
		}, 20*time.Millisecond)
	}
}

// DefaultSizes provides the default sizing.
func (t *Table) DefaultSizes(hint Size) (min, pref, max Size) {
	for col := range t.ColumnSizes {
		pref.Width += t.ColumnSizes[col].Current
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

// RowFromIndex returns the row data for the given index.
func (t *Table) RowFromIndex(index int) TableRowData {
	if index < 0 || index >= len(t.rowCache) {
		return nil
	}
	return t.rowCache[index].row
}

// RowToIndex returns the row's index within the displayed data, or -1 if it isn't currently in the disclosed rows.
func (t *Table) RowToIndex(rowData TableRowData) int {
	for row, data := range t.rowCache {
		if data.row == rowData {
			return row
		}
	}
	return -1
}

// LastRowIndex returns the index of the last row. Will be -1 if there are no rows.
func (t *Table) LastRowIndex() int {
	return len(t.rowCache) - 1
}

// ScrollRowIntoView scrolls the row at the given index into view.
func (t *Table) ScrollRowIntoView(row int) {
	if frame := t.RowFrame(row); !frame.IsEmpty() {
		t.ScrollRectIntoView(frame)
	}
}

// ScrollRowCellIntoView scrolls the cell from the row and column at the given indexes into view.
func (t *Table) ScrollRowCellIntoView(row, col int) {
	if frame := t.CellFrame(row, col); !frame.IsEmpty() {
		t.ScrollRectIntoView(frame)
	}
}

// InstallDragSupport installs default drag support into a table. This will chain a function to any existing
// MouseDragCallback.
func (t *Table) InstallDragSupport(svg *SVG, dragKey, singularName, pluralName string) {
	orig := t.MouseDragCallback
	t.MouseDragCallback = func(where Point, button int, mod Modifiers) bool {
		if orig != nil && orig(where, button, mod) {
			return true
		}
		if t.HasSelection() && t.IsDragGesture(where) {
			data := &TableDragData{
				Table: t,
				Rows:  t.SelectedRows(true),
			}
			drawable := NewTableDragDrawable(data, svg, singularName, pluralName)
			size := drawable.LogicalSize()
			t.StartDataDrag(&DragData{
				Data:     map[string]any{dragKey: data},
				Drawable: drawable,
				Ink:      t.OnBackgroundInk,
				Offset:   Point{X: 0, Y: -size.Height / 2},
			})
		}
		return false
	}
}

// InstallDropSupport installs default drop support into a table. This will replace any existing DataDragOverCallback,
// DataDragExitCallback, and DataDragDropCallback functions. It will also chain a function to any existing
// DrawOverCallback.
func (t *Table) InstallDropSupport(dragKey string, dropCallback func(*TableDrop)) *TableDrop {
	drop := &TableDrop{
		Table:            t,
		DragKey:          dragKey,
		originalDrawOver: t.DrawOverCallback,
		dropCallback:     dropCallback,
	}
	t.DataDragOverCallback = drop.DataDragOverCallback
	t.DataDragExitCallback = drop.DataDragExitCallback
	t.DataDragDropCallback = drop.DataDragDropCallback
	t.DrawOverCallback = drop.DrawOverCallback
	return drop
}

// CountTableRows returns the number of table rows, including all descendants, whether open or not.
func CountTableRows(rows []TableRowData) int {
	count := len(rows)
	for _, row := range rows {
		if row.CanHaveChildRows() {
			count += CountTableRows(row.ChildRows())
		}
	}
	return count
}
