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

	"github.com/google/uuid"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom"
)

var zeroUUID = uuid.UUID{}

// TableDragData holds the data from a table row drag.
type TableDragData[T TableRowConstraint[T]] struct {
	Table *Table[T]
	Rows  []T
}

// ColumnSize holds the column sizing information.
type ColumnSize struct {
	Current     float32
	Minimum     float32
	Maximum     float32
	AutoMinimum float32
	AutoMaximum float32
}

type tableCache[T TableRowConstraint[T]] struct {
	row    T
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
	InteriorDividerInk:     InteriorDividerColor,
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
	InteriorDividerInk     Ink
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
type Table[T TableRowConstraint[T]] struct {
	Panel
	TableTheme
	SelectionChangedCallback func()
	DoubleClickCallback      func()
	DragRemovedRowsCallback  func() // Called whenever a drag removes one or more rows from a model, but only if the source and destination tables were different.
	DropOccurredCallback     func() // Called whenever a drop occurs that modifies the model.
	ColumnSizes              []ColumnSize
	Model                    TableModel[T]
	filteredRows             []T // Note that we use the difference between nil and an empty slice here
	header                   *TableHeader[T]
	selMap                   map[uuid.UUID]bool
	selAnchor                uuid.UUID
	lastSel                  uuid.UUID
	hitRects                 []tableHitRect
	rowCache                 []tableCache[T]
	interactionRow           int
	interactionColumn        int
	lastMouseMotionRow       int
	lastMouseMotionColumn    int
	startRow                 int
	endBeforeRow             int
	columnResizeStart        float32
	columnResizeBase         float32
	columnResizeOverhead     float32
	PreventUserColumnResize  bool
	awaitingSizeColumnsToFit bool
	awaitingSyncToModel      bool
	selNeedsPrune            bool
	wasDragged               bool
}

// NewTable creates a new Table control.
func NewTable[T TableRowConstraint[T]](model TableModel[T]) *Table[T] {
	t := &Table[T]{
		TableTheme:            DefaultTableTheme,
		Model:                 model,
		selMap:                make(map[uuid.UUID]bool),
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
	t.KeyDownCallback = t.DefaultKeyDown
	t.InstallCmdHandlers(SelectAllItemID, AlwaysEnabled, func(_ any) { t.SelectAll() })
	t.wasDragged = false
	return t
}

// SetDrawRowRange sets a restricted range for sizing and drawing the table. This is intended primarily to be able to
// draw different sections of the table on separate pages of a display and should not be used for anything requiring
// interactivity.
func (t *Table[T]) SetDrawRowRange(start, endBefore int) {
	t.startRow = start
	t.endBeforeRow = endBefore
}

// ClearDrawRowRange clears any restricted range for sizing and drawing the table.
func (t *Table[T]) ClearDrawRowRange() {
	t.startRow = 0
	t.endBeforeRow = 0
}

// CurrentDrawRowRange returns the range of rows that are considered for sizing and drawing.
func (t *Table[T]) CurrentDrawRowRange() (start, endBefore int) {
	if t.startRow < t.endBeforeRow && t.startRow >= 0 && t.endBeforeRow <= len(t.rowCache) {
		return t.startRow, t.endBeforeRow
	}
	return 0, len(t.rowCache)
}

// DefaultDraw provides the default drawing.
func (t *Table[T]) DefaultDraw(canvas *Canvas, dirty Rect) {
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

	startRow, endBeforeRow := t.CurrentDrawRowRange()
	y := insets.Top
	for i := startRow; i < endBeforeRow; i++ {
		y1 := y + t.rowCache[i].height
		if t.ShowRowDivider {
			y1++
		}
		if y1 >= dirty.Y {
			break
		}
		y = y1
		startRow = i + 1
	}

	lastY := dirty.Bottom()
	rect := dirty
	rect.Y = y
	for r := startRow; r < endBeforeRow && rect.Y < lastY; r++ {
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
		if t.ShowRowDivider && r != endBeforeRow-1 {
			rect.Height = 1
			canvas.DrawRect(rect, t.InteriorDividerInk.Paint(canvas, rect, Fill))
			rect.Y++
		}
	}

	if t.ShowColumnDivider {
		rect = dirty
		rect.X = x
		rect.Width = 1
		for c := firstCol; c < len(t.ColumnSizes)-1; c++ {
			rect.X += t.ColumnSizes[c].Current
			canvas.DrawRect(rect, t.InteriorDividerInk.Paint(canvas, rect, Fill))
			rect.X++
		}
	}

	rect = dirty
	rect.Y = y
	lastX := dirty.Right()
	t.hitRects = nil
	for r := startRow; r < endBeforeRow && rect.Y < lastY; r++ {
		rect.X = x
		rect.Height = t.rowCache[r].height
		for c := firstCol; c < len(t.ColumnSizes) && rect.X < lastX; c++ {
			fg, bg, selected, indirectlySelected, focused := t.cellParams(r, c)
			rect.Width = t.ColumnSizes[c].Current
			cellRect := rect
			cellRect.Inset(t.Padding)
			row := t.rowCache[r].row
			if c == t.HierarchyColumnIndex {
				if row.CanHaveChildren() {
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

func (t *Table[T]) cellParams(row, col int) (fg, bg Ink, selected, indirectlySelected, focused bool) {
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

func (t *Table[T]) cell(row, col int) *Panel {
	fg, bg, selected, indirectlySelected, focused := t.cellParams(row, col)
	return t.rowCache[row].row.ColumnCell(row, col, fg, bg, selected, indirectlySelected, focused).AsPanel()
}

func (t *Table[T]) installCell(cell *Panel, frame Rect) {
	cell.SetFrameRect(frame)
	cell.ValidateLayout()
	cell.parent = t.AsPanel()
}

func (t *Table[T]) uninstallCell(cell *Panel) {
	cell.parent = nil
}

// RowHeights returns the heights of each row.
func (t *Table[T]) RowHeights() []float32 {
	heights := make([]float32, len(t.rowCache))
	for i := range t.rowCache {
		heights[i] = t.rowCache[i].height
	}
	return heights
}

// OverRow returns the row index that the y coordinate is over, or -1 if it isn't over any row.
func (t *Table[T]) OverRow(y float32) int {
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
func (t *Table[T]) OverColumn(x float32) int {
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
func (t *Table[T]) OverColumnDivider(x float32) int {
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
func (t *Table[T]) CellWidth(row, col int) float32 {
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
func (t *Table[T]) ColumnEdges(col int) (left, right float32) {
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
func (t *Table[T]) CellFrame(row, col int) Rect {
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
func (t *Table[T]) RowFrame(row int) Rect {
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

func (t *Table[T]) newTableHitRect(rect Rect, row T) tableHitRect {
	return tableHitRect{
		Rect: rect,
		handler: func(where Point, button, clickCount int, mod Modifiers) {
			open := !row.IsOpen()
			row.SetOpen(open)
			t.SyncToModel()
			if !open {
				t.PruneSelectionOfUndisclosedNodes()
			}
		},
	}
}

// DefaultFocusGained provides the default focus gained handling.
func (t *Table[T]) DefaultFocusGained() {
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
func (t *Table[T]) DefaultUpdateCursorCallback(where Point) *Cursor {
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
func (t *Table[T]) DefaultUpdateTooltipCallback(where Point, suggestedAvoidInRoot Rect) Rect {
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
func (t *Table[T]) DefaultMouseEnter(where Point, mod Modifiers) bool {
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
func (t *Table[T]) DefaultMouseExit() bool {
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
func (t *Table[T]) DefaultMouseMove(where Point, mod Modifiers) bool {
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
func (t *Table[T]) DefaultMouseDown(where Point, button, clickCount int, mod Modifiers) bool {
	if t.Window().InDrag() {
		return false
	}
	t.RequestFocus()
	t.wasDragged = false
	t.lastSel = zeroUUID

	t.interactionRow = -1
	t.interactionColumn = -1
	if button == ButtonLeft {
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
		id := rowData.UUID()
		switch {
		case mod&ShiftModifier != 0: // Extend selection from anchor
			selAnchorIndex := -1
			if t.selAnchor != zeroUUID {
				for i, c := range t.rowCache {
					if c.row.UUID() == t.selAnchor {
						selAnchorIndex = i
						break
					}
				}
			}
			if selAnchorIndex != -1 {
				last := xmath.Max(selAnchorIndex, row)
				for i := xmath.Min(selAnchorIndex, row); i <= last; i++ {
					t.selMap[t.rowCache[i].row.UUID()] = true
				}
				t.notifyOfSelectionChange()
			} else if !t.selMap[id] { // No anchor, so behave like a regular click
				t.selMap = make(map[uuid.UUID]bool)
				t.selMap[id] = true
				t.selAnchor = id
				t.notifyOfSelectionChange()
			}
		case mod&(OptionModifier|CommandModifier) != 0: // Toggle single row
			if t.selMap[id] {
				delete(t.selMap, id)
			} else {
				t.selMap[id] = true
			}
			t.notifyOfSelectionChange()
		case t.selMap[id]: // Sets lastClick so that on mouse up, we can treat a click and click and hold differently
			t.lastSel = id
		default: // If not already selected, replace selection with current row and make it the anchor
			t.selMap = make(map[uuid.UUID]bool)
			t.selMap[id] = true
			t.selAnchor = id
			t.notifyOfSelectionChange()
		}
		t.MarkForRedraw()
		if button == ButtonLeft && clickCount == 2 && t.DoubleClickCallback != nil && len(t.selMap) != 0 {
			toolbox.Call(t.DoubleClickCallback)
		}
	}
	return stop
}

func (t *Table[T]) notifyOfSelectionChange() {
	if t.SelectionChangedCallback != nil {
		toolbox.Call(t.SelectionChangedCallback)
	}
}

// DefaultMouseDrag provides the default mouse drag handling.
func (t *Table[T]) DefaultMouseDrag(where Point, button int, mod Modifiers) bool {
	t.wasDragged = true
	stop := false
	if t.interactionColumn != -1 {
		if t.interactionRow == -1 {
			if button == ButtonLeft && !t.PreventUserColumnResize {
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
func (t *Table[T]) DefaultMouseUp(where Point, button int, mod Modifiers) bool {
	if !t.wasDragged && t.lastSel != zeroUUID {
		t.ClearSelection()
		t.selMap[t.lastSel] = true
		t.selAnchor = t.lastSel
		t.MarkForRedraw()
		t.notifyOfSelectionChange()
	}

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

// DefaultKeyDown provides the default key down handling.
func (t *Table[T]) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		if t.DoubleClickCallback != nil && len(t.selMap) != 0 {
			toolbox.Call(t.DoubleClickCallback)
		}
		return true
	}
	switch keyCode {
	case KeyLeft:
		if t.HasSelection() {
			altered := false
			for _, row := range t.SelectedRows(false) {
				if row.IsOpen() {
					row.SetOpen(false)
					altered = true
				}
			}
			if altered {
				t.SyncToModel()
				t.PruneSelectionOfUndisclosedNodes()
			}
		}
	case KeyRight:
		if t.HasSelection() {
			altered := false
			for _, row := range t.SelectedRows(false) {
				if !row.IsOpen() {
					row.SetOpen(true)
					altered = true
				}
			}
			if altered {
				t.SyncToModel()
			}
		}
	case KeyUp:
		var i int
		if t.HasSelection() {
			i = xmath.Max(t.FirstSelectedRowIndex()-1, 0)
		} else {
			i = len(t.rowCache) - 1
		}
		if !mod.ShiftDown() {
			t.ClearSelection()
		}
		t.SelectByIndex(i)
		t.ScrollRowCellIntoView(i, 0)
	case KeyDown:
		i := xmath.Min(t.LastSelectedRowIndex()+1, len(t.rowCache)-1)
		if !mod.ShiftDown() {
			t.ClearSelection()
		}
		t.SelectByIndex(i)
		t.ScrollRowCellIntoView(i, 0)
	case KeyHome:
		if mod.ShiftDown() && t.HasSelection() {
			t.SelectRange(0, t.FirstSelectedRowIndex())
		} else {
			t.ClearSelection()
			t.SelectByIndex(0)
		}
		t.ScrollRowCellIntoView(0, 0)
	case KeyEnd:
		if mod.ShiftDown() && t.HasSelection() {
			t.SelectRange(t.LastSelectedRowIndex(), len(t.rowCache)-1)
		} else {
			t.ClearSelection()
			t.SelectByIndex(len(t.rowCache) - 1)
		}
		t.ScrollRowCellIntoView(len(t.rowCache)-1, 0)
	default:
		return false
	}
	return true
}

// PruneSelectionOfUndisclosedNodes removes any nodes in the selection map that are no longer disclosed from the
// selection map.
func (t *Table[T]) PruneSelectionOfUndisclosedNodes() {
	if !t.selNeedsPrune {
		return
	}
	t.selNeedsPrune = false
	if len(t.selMap) == 0 {
		return
	}
	needsNotify := false
	selMap := make(map[uuid.UUID]bool, len(t.selMap))
	for _, entry := range t.rowCache {
		id := entry.row.UUID()
		if t.selMap[id] {
			selMap[id] = true
		} else {
			needsNotify = true
		}
	}
	t.selMap = selMap
	if needsNotify {
		t.notifyOfSelectionChange()
	}
}

// FirstSelectedRowIndex returns the first selected row index, or -1 if there is no selection.
func (t *Table[T]) FirstSelectedRowIndex() int {
	if len(t.selMap) == 0 {
		return -1
	}
	for i, entry := range t.rowCache {
		if t.selMap[entry.row.UUID()] {
			return i
		}
	}
	return -1
}

// LastSelectedRowIndex returns the last selected row index, or -1 if there is no selection.
func (t *Table[T]) LastSelectedRowIndex() int {
	if len(t.selMap) == 0 {
		return -1
	}
	for i := len(t.rowCache) - 1; i >= 0; i-- {
		if t.selMap[t.rowCache[i].row.UUID()] {
			return i
		}
	}
	return -1
}

// IsRowOrAnyParentSelected returns true if the specified row index or any of its parents are selected.
func (t *Table[T]) IsRowOrAnyParentSelected(index int) bool {
	if index < 0 || index >= len(t.rowCache) {
		return false
	}
	for index >= 0 {
		if t.selMap[t.rowCache[index].row.UUID()] {
			return true
		}
		index = t.rowCache[index].parent
	}
	return false
}

// IsRowSelected returns true if the specified row index is selected.
func (t *Table[T]) IsRowSelected(index int) bool {
	if index < 0 || index >= len(t.rowCache) {
		return false
	}
	return t.selMap[t.rowCache[index].row.UUID()]
}

// SelectedRows returns the currently selected rows. If 'minimal' is true, then children of selected rows that may also
// be selected are not returned, just the topmost row that is selected in any given hierarchy.
func (t *Table[T]) SelectedRows(minimal bool) []T {
	t.PruneSelectionOfUndisclosedNodes()
	if len(t.selMap) == 0 {
		return nil
	}
	rows := make([]T, 0, len(t.selMap))
	for _, entry := range t.rowCache {
		if t.selMap[entry.row.UUID()] && (!minimal || entry.parent == -1 || !t.IsRowOrAnyParentSelected(entry.parent)) {
			rows = append(rows, entry.row)
		}
	}
	return rows
}

// CopySelectionMap returns a copy of the current selection map.
func (t *Table[T]) CopySelectionMap() map[uuid.UUID]bool {
	t.PruneSelectionOfUndisclosedNodes()
	return copySelMap(t.selMap)
}

// SetSelectionMap sets the current selection map.
func (t *Table[T]) SetSelectionMap(selMap map[uuid.UUID]bool) {
	t.selMap = copySelMap(selMap)
	t.selNeedsPrune = true
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

func copySelMap(selMap map[uuid.UUID]bool) map[uuid.UUID]bool {
	result := make(map[uuid.UUID]bool, len(selMap))
	for k, v := range selMap {
		result[k] = v
	}
	return result
}

// HasSelection returns true if there is a selection.
func (t *Table[T]) HasSelection() bool {
	t.PruneSelectionOfUndisclosedNodes()
	return len(t.selMap) != 0
}

// SelectionCount returns the number of rows explicitly selected.
func (t *Table[T]) SelectionCount() int {
	t.PruneSelectionOfUndisclosedNodes()
	return len(t.selMap)
}

// ClearSelection clears the selection.
func (t *Table[T]) ClearSelection() {
	if len(t.selMap) == 0 {
		return
	}
	t.selMap = make(map[uuid.UUID]bool)
	t.selNeedsPrune = false
	t.selAnchor = zeroUUID
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// SelectAll selects all rows.
func (t *Table[T]) SelectAll() {
	t.selMap = make(map[uuid.UUID]bool, len(t.rowCache))
	t.selNeedsPrune = false
	t.selAnchor = zeroUUID
	for _, cache := range t.rowCache {
		id := cache.row.UUID()
		t.selMap[id] = true
		if t.selAnchor == zeroUUID {
			t.selAnchor = id
		}
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// SelectByIndex selects the given indexes. The first one will be considered the anchor selection if no existing anchor
// selection exists.
func (t *Table[T]) SelectByIndex(indexes ...int) {
	for _, index := range indexes {
		if index >= 0 && index < len(t.rowCache) {
			id := t.rowCache[index].row.UUID()
			t.selMap[id] = true
			t.selNeedsPrune = true
			if t.selAnchor == zeroUUID {
				t.selAnchor = id
			}
		}
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// SelectRange selects the given range. The start will be considered the anchor selection if no existing anchor
// selection exists.
func (t *Table[T]) SelectRange(start, end int) {
	start = xmath.Max(start, 0)
	end = xmath.Min(end, len(t.rowCache)-1)
	if start > end {
		return
	}
	for i := start; i <= end; i++ {
		id := t.rowCache[i].row.UUID()
		t.selMap[id] = true
		t.selNeedsPrune = true
		if t.selAnchor == zeroUUID {
			t.selAnchor = id
		}
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// DeselectByIndex deselects the given indexes.
func (t *Table[T]) DeselectByIndex(indexes ...int) {
	for _, index := range indexes {
		if index >= 0 && index < len(t.rowCache) {
			delete(t.selMap, t.rowCache[index].row.UUID())
		}
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// DeselectRange deselects the given range.
func (t *Table[T]) DeselectRange(start, end int) {
	start = xmath.Max(start, 0)
	end = xmath.Min(end, len(t.rowCache)-1)
	if start > end {
		return
	}
	for i := start; i <= end; i++ {
		delete(t.selMap, t.rowCache[i].row.UUID())
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// DiscloseRow ensures the given row can be viewed by opening all parents that lead to it. Returns true if any
// modification was made.
func (t *Table[T]) DiscloseRow(row T, delaySync bool) bool {
	modified := false
	p := row.Parent()
	var zero T
	for p != zero {
		if !p.IsOpen() {
			p.SetOpen(true)
			modified = true
		}
		p = p.Parent()
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

// RootRowCount returns the number of top-level rows.
func (t *Table[T]) RootRowCount() int {
	if t.filteredRows != nil {
		return len(t.filteredRows)
	}
	return t.Model.RootRowCount()
}

// RootRows returns the top-level rows. Do not alter the returned list.
func (t *Table[T]) RootRows() []T {
	if t.filteredRows != nil {
		return t.filteredRows
	}
	return t.Model.RootRows()
}

// SetRootRows sets the top-level rows this table will display. This will call SyncToModel() automatically.
func (t *Table[T]) SetRootRows(rows []T) {
	t.filteredRows = nil
	t.Model.SetRootRows(rows)
	t.selMap = make(map[uuid.UUID]bool)
	t.selNeedsPrune = false
	t.selAnchor = zeroUUID
	t.SyncToModel()
}

// SyncToModel causes the table to update its internal caches to reflect the current model.
func (t *Table[T]) SyncToModel() {
	rowCount := 0
	roots := t.RootRows()
	if t.filteredRows != nil {
		rowCount = len(t.filteredRows)
	} else {
		for _, row := range roots {
			rowCount += t.countOpenRowChildrenRecursively(row)
		}
	}
	t.rowCache = make([]tableCache[T], rowCount)
	j := 0
	for _, row := range roots {
		j = t.buildRowCacheEntry(row, -1, j, 0)
	}
	t.selNeedsPrune = true
	_, pref, _ := t.DefaultSizes(Size{})
	rect := t.FrameRect()
	rect.Size = pref
	t.SetFrameRect(rect)
	t.MarkForRedraw()
	t.MarkForLayoutRecursivelyUpward()
}

func (t *Table[T]) countOpenRowChildrenRecursively(row T) int {
	count := 1
	if row.CanHaveChildren() && row.IsOpen() {
		for _, child := range row.Children() {
			count += t.countOpenRowChildrenRecursively(child)
		}
	}
	return count
}

func (t *Table[T]) buildRowCacheEntry(row T, parentIndex, index, depth int) int {
	t.rowCache[index].row = row
	t.rowCache[index].parent = parentIndex
	t.rowCache[index].depth = depth
	t.rowCache[index].height = t.heightForColumns(row, index, depth)
	parentIndex = index
	index++
	if t.filteredRows == nil && row.CanHaveChildren() && row.IsOpen() {
		for _, child := range row.Children() {
			index = t.buildRowCacheEntry(child, parentIndex, index, depth+1)
		}
	}
	return index
}

func (t *Table[T]) heightForColumns(rowData T, row, depth int) float32 {
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

func (t *Table[T]) cellPrefSize(rowData T, row, col int, widthConstraint float32) geom.Size[float32] {
	fg, bg, selected, indirectlySelected, focused := t.cellParams(row, col)
	cell := rowData.ColumnCell(row, col, fg, bg, selected, indirectlySelected, focused).AsPanel()
	_, size, _ := cell.Sizes(Size{Width: widthConstraint})
	return size
}

// SizeColumnsToFitWithExcessIn sizes each column to its preferred size, with the exception of the 'excessColumnIndex',
// which gets set to any remaining width left over. Pass in -1 for the 'excessColumnIndex' to use the
// HierarchyColumnIndex or 0, if the HierarchyColumnIndex is less than 0
func (t *Table[T]) SizeColumnsToFitWithExcessIn(excessColumnIndex int) {
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
func (t *Table[T]) SizeColumnsToFit(adjust bool) {
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
func (t *Table[T]) SizeColumnToFit(col int, adjust bool) {
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
func (t *Table[T]) EventuallySizeColumnsToFit(adjust bool) {
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
func (t *Table[T]) EventuallySyncToModel() {
	if !t.awaitingSyncToModel {
		t.awaitingSyncToModel = true
		InvokeTaskAfter(func() {
			t.SyncToModel()
			t.awaitingSyncToModel = false
		}, 20*time.Millisecond)
	}
}

// DefaultSizes provides the default sizing.
func (t *Table[T]) DefaultSizes(hint Size) (min, pref, max Size) {
	for col := range t.ColumnSizes {
		pref.Width += t.ColumnSizes[col].Current
	}
	startRow, endBeforeRow := t.CurrentDrawRowRange()
	for _, cache := range t.rowCache[startRow:endBeforeRow] {
		pref.Height += cache.height
	}
	if t.ShowColumnDivider {
		pref.Width += float32(len(t.ColumnSizes) - 1)
	}
	if t.ShowRowDivider {
		pref.Height += float32((endBeforeRow - startRow) - 1)
	}
	if border := t.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	return pref, pref, pref
}

// RowFromIndex returns the row data for the given index.
func (t *Table[T]) RowFromIndex(index int) T {
	if index < 0 || index >= len(t.rowCache) {
		var zero T
		return zero
	}
	return t.rowCache[index].row
}

// RowToIndex returns the row's index within the displayed data, or -1 if it isn't currently in the disclosed rows.
func (t *Table[T]) RowToIndex(rowData T) int {
	id := rowData.UUID()
	for row, data := range t.rowCache {
		if data.row.UUID() == id {
			return row
		}
	}
	return -1
}

// LastRowIndex returns the index of the last row. Will be -1 if there are no rows.
func (t *Table[T]) LastRowIndex() int {
	return len(t.rowCache) - 1
}

// ScrollRowIntoView scrolls the row at the given index into view.
func (t *Table[T]) ScrollRowIntoView(row int) {
	if frame := t.RowFrame(row); !frame.IsEmpty() {
		t.ScrollRectIntoView(frame)
	}
}

// ScrollRowCellIntoView scrolls the cell from the row and column at the given indexes into view.
func (t *Table[T]) ScrollRowCellIntoView(row, col int) {
	if frame := t.CellFrame(row, col); !frame.IsEmpty() {
		t.ScrollRectIntoView(frame)
	}
}

// IsFiltered returns true if a filter is currently applied. When a filter is applied, no hierarchy is display and no
// modifications to the row data should be performed.
func (t *Table[T]) IsFiltered() bool {
	return t.filteredRows != nil
}

// ApplyFilter applies a filter to the data. When a non-nil filter is applied, all rows (recursively) are passed through
// the filter. Only those that the filter returns false for will be visible in the table. When a filter is applied, no
// hierarchy is display and no modifications to the row data should be performed.
func (t *Table[T]) ApplyFilter(filter func(row T) bool) {
	if filter == nil {
		if t.filteredRows == nil {
			return
		}
		t.filteredRows = nil
	} else {
		t.filteredRows = make([]T, 0)
		for _, row := range t.Model.RootRows() {
			t.applyFilter(row, filter)
		}
	}
	t.SyncToModel()
	if t.header != nil && t.header.HasSort() {
		t.header.ApplySort()
	}
}

func (t *Table[T]) applyFilter(row T, filter func(row T) bool) {
	if !filter(row) {
		t.filteredRows = append(t.filteredRows, row)
	}
	if row.CanHaveChildren() {
		for _, child := range row.Children() {
			t.applyFilter(child, filter)
		}
	}
}

// InstallDragSupport installs default drag support into a table. This will chain a function to any existing
// MouseDragCallback.
func (t *Table[T]) InstallDragSupport(svg *SVG, dragKey, singularName, pluralName string) {
	orig := t.MouseDragCallback
	t.MouseDragCallback = func(where Point, button int, mod Modifiers) bool {
		if orig != nil && orig(where, button, mod) {
			return true
		}
		if button == ButtonLeft && t.HasSelection() && t.IsDragGesture(where) {
			data := &TableDragData[T]{
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
// DrawOverCallback. The shouldMoveDataCallback is called when a drop is about to occur to determine if the data should
// be moved (i.e. removed from the source) or copied to the destination. The willDropCallback is called before the
// actual data changes are made, giving an opportunity to start an undo event, which should be returned. The
// didDropCallback is called after data changes are made and is passed the undo event (if any) returned by the
// willDropCallback, so that the undo event can be completed and posted.
func InstallDropSupport[T TableRowConstraint[T], U any](t *Table[T], dragKey string, shouldMoveDataCallback func(from, to *Table[T]) bool, willDropCallback func(from, to *Table[T], move bool) *UndoEdit[U], didDropCallback func(undo *UndoEdit[U], from, to *Table[T], move bool)) *TableDrop[T, U] {
	drop := &TableDrop[T, U]{
		Table:                  t,
		DragKey:                dragKey,
		originalDrawOver:       t.DrawOverCallback,
		shouldMoveDataCallback: shouldMoveDataCallback,
		willDropCallback:       willDropCallback,
		didDropCallback:        didDropCallback,
	}
	t.DataDragOverCallback = drop.DataDragOverCallback
	t.DataDragExitCallback = drop.DataDragExitCallback
	t.DataDragDropCallback = drop.DataDragDropCallback
	t.DrawOverCallback = drop.DrawOverCallback
	return drop
}

// CountTableRows returns the number of table rows, including all descendants, whether open or not.
func CountTableRows[T TableRowConstraint[T]](rows []T) int {
	count := len(rows)
	for _, row := range rows {
		if row.CanHaveChildren() {
			count += CountTableRows(row.Children())
		}
	}
	return count
}

// RowContainsRow returns true if 'descendant' is in fact a descendant of 'ancestor'.
func RowContainsRow[T TableRowConstraint[T]](ancestor, descendant T) bool {
	var zero T
	for descendant != zero && descendant != ancestor {
		descendant = descendant.Parent()
	}
	return descendant == ancestor
}
