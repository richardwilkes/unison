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
	"maps"
	"slices"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// TableDragData holds the data from a table row drag.
type TableDragData[T TableRowConstraint[T]] struct {
	Table *Table[T]
	Rows  []T
}

// ColumnInfo holds column information.
type ColumnInfo struct {
	ID          int
	Current     float32
	Minimum     float32
	Maximum     float32
	AutoMinimum float32
	AutoMaximum float32
}

// resizable returns true if the user should be permitted to resize the column, i.e. its Minimum and Maximum don't pin it
// to a single width. A Maximum of 0 or less means "no maximum", matching how the resize clamping treats it, so a column
// with only a Minimum set remains resizable.
func (c *ColumnInfo) resizable() bool {
	return c.Minimum <= 0 || c.Maximum <= 0 || c.Minimum < c.Maximum
}

type tableCache[T TableRowConstraint[T]] struct {
	row    T
	parent int
	depth  int
	height float32
}

type tableHitRect struct {
	handler func()
	geom.Rect
}

// dragTableData is actually a *TableDragData[T], but cannot be stored with the originating table since it must be
// accessible from the drag data. All access occurs on the UI thread during a single drag & drop operation, so only
// one drag can be in flight at a time and no synchronization is required.
var dragTableData any

// DefaultTableTheme holds the default TableTheme values for Tables. Modifying this data will not alter existing Tables,
// but will alter any Tables created in the future.
var DefaultTableTheme = TableTheme{
	BackgroundInk:          ThemeBelowSurface,
	OnBackgroundInk:        ThemeOnBelowSurface,
	BandingInk:             ThemeBanding,
	OnBandingInk:           ThemeOnBanding,
	InteriorDividerInk:     ThemeAboveSurface,
	SelectionInk:           ThemeFocus,
	OnSelectionInk:         ThemeOnFocus,
	InactiveSelectionInk:   ThemeDeepFocus,
	OnInactiveSelectionInk: ThemeOnDeepFocus,
	IndirectSelectionInk:   ThemeDeeperFocus,
	OnIndirectSelectionInk: ThemeOnDeeperFocus,
	Padding:                geom.NewUniformInsets(4),
	HierarchyIndent:        16,
	MinimumRowHeight:       16,
	ColumnResizeSlop:       4,
	ShowColumnDivider:      true,
	ShowFirstColumnDivider: true,
	ShowLastColumnDivider:  true,
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
	Padding                geom.Insets
	HierarchyColumnID      int
	HierarchyIndent        float32
	MinimumRowHeight       float32
	ColumnResizeSlop       float32
	ShowRowDivider         bool
	ShowColumnDivider      bool
	ShowFirstColumnDivider bool
	ShowLastColumnDivider  bool
}

// Table provides a control that can display data in columns and rows.
//
// A cell may contain a widget that takes the keyboard focus, such as a Field. Clicking such a widget focuses it and
// lets the user work in it in place, Tab and Shift-Tab move the focus to the next or previous focusable cell in
// row-major order and leave the table once there are none left in that direction, and Return, Enter or Escape hand the
// focus back to the table. FocusCell() starts the same thing programmatically and FocusedCell() reports which cell, if
// any, currently holds the focus. A row that supplies such a widget must hand back the same instance from every
// ColumnCell() call, since a freshly created one would have neither the focus nor the state the user put into it.
type Table[T TableRowConstraint[T]] struct {
	SelectionChangedCallback func()
	DoubleClickCallback      func()
	DragRemovedRowsCallback  func() // Called whenever a drag removes one or more rows from a model, but only if the source and destination tables were different.
	DropOccurredCallback     func() // Called whenever a drop occurs that modifies the model.
	Columns                  []ColumnInfo
	Model                    TableModel[T]
	filteredRows             []T // Note that we use the difference between nil and an empty slice here
	header                   *TableHeader[T]
	selMap                   map[tid.TID]bool
	selAnchor                tid.TID
	lastSel                  tid.TID
	hitRects                 []tableHitRect
	rowCache                 []tableCache[T]
	lastMouseEnterCellPanel  *Panel
	lastMouseDownCellPanel   *Panel
	// Cells are normally ephemeral: the table points a cell's parent at itself just long enough to draw it or forward
	// one event to it, then detaches it again. That can't work for a cell whose widget takes the keyboard focus, since
	// the window dispatches keys, redraws and scrolling by walking up the parent chain from Window.focus, and
	// Window.Focus() moves the focus elsewhere the moment it finds it detached. The cell holding the focus is therefore
	// left installed in the table until it loses the focus. These two fields, plus focusedCellRowIndex and
	// focusedCellColumn further down, track which cell that is and where it lives, so it can be recognized again among
	// the panels the row hands back from ColumnCell().
	focusedCell    *Panel
	focusedCellRow T
	TableTheme
	Panel
	pressedHitRect           geom.Rect
	interactionRow           int
	interactionColumn        int
	focusedCellRowIndex      int
	focusedCellColumn        int
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
	dividerDrag              bool
	hasHierarchy             bool
	noScrollOnFocus          bool
}

// NewTable creates a new Table control.
func NewTable[T TableRowConstraint[T]](model TableModel[T]) *Table[T] {
	t := &Table[T]{
		TableTheme:            DefaultTableTheme,
		Model:                 model,
		selMap:                make(map[tid.TID]bool),
		interactionRow:        -1,
		interactionColumn:     -1,
		lastMouseMotionRow:    -1,
		lastMouseMotionColumn: -1,
		focusedCellRowIndex:   -1,
		focusedCellColumn:     -1,
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

// ColumnIndexForID returns the column index with the given ID, or -1 if not found.
func (t *Table[T]) ColumnIndexForID(id int) int {
	for i, c := range t.Columns {
		if c.ID == id {
			return i
		}
	}
	return -1
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

// CurrentHierarchyIndent returns the current hierarchy indent, which will be 0 if the table has no containers.
func (t *Table[T]) CurrentHierarchyIndent() float32 {
	if t.hasHierarchy {
		return t.HierarchyIndent
	}
	return 0
}

func (t *Table[T]) leadingColumnDividerWidth() float32 {
	if t.ShowColumnDivider && t.ShowFirstColumnDivider {
		return 1
	}
	return 0
}

// DefaultDraw provides the default drawing.
func (t *Table[T]) DefaultDraw(canvas *Canvas, dirty geom.Rect) {
	t.validateFocusedCell()
	selectionInk := t.SelectionInk
	if !t.hasFocus() {
		selectionInk = t.InactiveSelectionInk
	}

	backgroundPaint := t.BackgroundInk.Paint(canvas, dirty, paintstyle.Fill)
	canvas.DrawRect(dirty, backgroundPaint)

	var insets geom.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}

	var firstCol int
	x := insets.Left + t.leadingColumnDividerWidth()
	for i := range t.Columns {
		x1 := x + t.Columns[i].Current
		if t.ShowColumnDivider && (t.ShowLastColumnDivider || i < len(t.Columns)-1) {
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
			var rowInk Ink
			if t.IsRowSelected(r) {
				rowInk = selectionInk
			} else {
				rowInk = t.IndirectSelectionInk
			}
			paint := rowInk.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
		} else if r%2 == 1 {
			paint := t.BandingInk.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider && r != endBeforeRow-1 {
			rect.Height = 1
			paint := t.InteriorDividerInk.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
			rect.Y++
		}
	}

	if t.ShowColumnDivider {
		rect = dirty
		rect.Width = 1
		if firstCol == 0 && t.leadingColumnDividerWidth() > 0 {
			rect.X = insets.Left
			paint := t.InteriorDividerInk.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
		}
		rect.X = x
		lastCol := len(t.Columns)
		if !t.ShowLastColumnDivider {
			lastCol--
		}
		for c := firstCol; c < lastCol; c++ {
			rect.X += t.Columns[c].Current
			paint := t.InteriorDividerInk.Paint(canvas, rect, paintstyle.Fill)
			canvas.DrawRect(rect, paint)
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
		for c := firstCol; c < len(t.Columns) && rect.X < lastX; c++ {
			rect.Width = t.Columns[c].Current
			cellRect := rect.Inset(t.Padding)
			row := t.rowCache[r].row
			if t.Columns[c].ID == t.HierarchyColumnID {
				if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
					if row.CanHaveChildren() {
						const disclosureIndent = 2
						disclosureSize := min(hierarchyIndent, t.MinimumRowHeight) - disclosureIndent*2
						canvas.Save()
						left := cellRect.X + hierarchyIndent*float32(t.rowCache[r].depth) + disclosureIndent
						top := cellRect.Y + (t.MinimumRowHeight-disclosureSize)/2
						t.hitRects = append(t.hitRects,
							t.newTableHitRect(geom.NewRect(left, top, disclosureSize, disclosureSize), row))
						canvas.Translate(geom.NewPoint(left, top))
						if row.IsOpen() {
							offset := disclosureSize / 2
							offsetPt := geom.NewPoint(offset, offset)
							canvas.Translate(offsetPt)
							canvas.Rotate(90)
							canvas.Translate(offsetPt.Neg())
						}
						// The disclosure triangle is the only part of the cell the table draws itself, so the cell's
						// foreground ink is only needed here; cellFor() obtains its own copy for the cell.
						fg, _, _, _, _ := t.cellParams(r, c)
						chevronPaint := fg.Paint(canvas, cellRect, paintstyle.Fill)
						CircledChevronRightSVG.DrawInRectPreservingAspectRatio(canvas,
							geom.NewRect(0, 0, disclosureSize, disclosureSize), nil, chevronPaint)
						canvas.Restore()
					}
					indent := hierarchyIndent*float32(t.rowCache[r].depth+1) + t.Padding.Left
					cellRect.X += indent
					cellRect.Width -= indent
				}
			}
			cell := t.cellFor(row, r, c)
			if t.focusedCell != nil && r == t.focusedCellRowIndex && c == t.focusedCellColumn && cell != t.focusedCell {
				// The row presents a different panel at the focused position now, which happens when it creates a new
				// editor on each call rather than memoizing one, or when it swaps the editor out for something else.
				// The panel we are holding the focus for is no longer part of what the user sees, so end the editing
				// session rather than let an invisible widget go on eating the keystrokes.
				t.releaseFocusedCell(true)
			}
			t.installCell(cell, cellRect)
			canvas.Save()
			canvas.Translate(cellRect.Point)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, cellRect)
			t.uninstallCell(cell, r, c)
			canvas.Restore()
			rect.X += t.Columns[c].Current
			if t.ShowColumnDivider && (t.ShowLastColumnDivider || c < len(t.Columns)-1) {
				rect.X++
			}
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider {
			rect.Y++
		}
	}
}

// cellParams returns what a row is told about the cell at the given position: the inks to draw it with and the state
// those inks reflect. The cell holding the keyboard focus is handed the row's plain inks even when its row is selected,
// so that it stands out from the rest of the row as the cell being edited, and so that a Field's own text selection,
// which is drawn with the same ink as a selected row, stays visible within it. The row is still reported as selected,
// since it is.
func (t *Table[T]) cellParams(row, col int) (fg, bg Ink, selected, indirectlySelected, focused bool) {
	focused = t.hasFocus()
	selected = t.IsRowSelected(row)
	indirectlySelected = !selected && t.IsRowOrAnyParentSelected(row)
	editing := t.focusedCell != nil && row == t.focusedCellRowIndex && col == t.focusedCellColumn
	switch {
	case !editing && selected && focused:
		fg = t.OnSelectionInk
		bg = t.SelectionInk
	case !editing && selected:
		fg = t.OnInactiveSelectionInk
		bg = t.InactiveSelectionInk
	case !editing && indirectlySelected:
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
	return t.cellFor(t.rowCache[row].row, row, col)
}

// panelContains returns true if p is ancestor or one of its descendants. This walks the parent pointers directly
// rather than using Panel.AncestorIsOrSelf(), since that compares the panels' Self fields, which are both nil for a
// bare panel that was never given one and would then report a match between two unrelated panels.
func panelContains(ancestor, p *Panel) bool {
	for ; p != nil; p = p.parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

// hasFocus returns true if the table itself or the cell that is currently holding the keyboard focus has it. Unlike
// Panel.Focused(), this asks without disturbing anything: Window.Focus() self-heals a focus it considers stale by
// moving it to another panel, which must never happen as a side effect of drawing or measuring the table.
func (t *Table[T]) hasFocus() bool {
	wnd := t.Window()
	return wnd != nil && wnd.Focused() && wnd.focus != nil &&
		(wnd.focus == t.AsPanel() || (t.focusedCell != nil && panelContains(t.focusedCell, wnd.focus)))
}

// adoptCellIfFocused makes cell the table's focused cell if the window's keyboard focus currently lies within it. Cells
// are acquired fresh from the row on every call, so a row that wraps a memoized editor in a newly created panel each
// time hands back a different cell on each call, and AddChild() will have moved the editor into that newest wrapper.
// The newest wrapper must therefore be adopted the moment it is acquired, no matter which code path asked for it,
// because otherwise the focused editor is left hanging off a panel with no window and the next call to Window.Focus()
// takes the focus away from it.
func (t *Table[T]) adoptCellIfFocused(cell *Panel, rowData T, row, col int) {
	if cell == t.focusedCell {
		return
	}
	wnd := t.Window()
	if wnd == nil || wnd.focus == nil || !panelContains(cell, wnd.focus) {
		return
	}
	if t.focusedCell != nil && t.focusedCell.parent == t.AsPanel() {
		t.focusedCell.parent = nil
	}
	t.focusedCell = cell
	t.focusedCellRow = rowData
	t.focusedCellRowIndex = row
	t.focusedCellColumn = col
	cell.parent = t.AsPanel()
}

// cellFor returns the cell panel for the given row data and position. Every place that asks a row for one of its cells
// funnels through here so that the adoption of a focused cell can't be missed.
func (t *Table[T]) cellFor(rowData T, row, col int) *Panel {
	fg, bg, selected, indirectlySelected, focused := t.cellParams(row, col)
	cell := rowData.ColumnCell(row, col, fg, bg, selected, indirectlySelected, focused).AsPanel()
	t.adoptCellIfFocused(cell, rowData, row, col)
	return cell
}

// releaseFocusedCell stops tracking the focused cell. If 'refocus' is true, the table takes the keyboard focus back,
// which is what triggers the cell widget's LostFocusCallback; that is also the right thing to do when the table isn't
// focusable or is disabled, since the focus is then simply removed, which fires the callback just the same. The order
// here matters. The tracking fields are cleared first so that a LostFocusCallback which turns around and calls back
// into the table (committing an edit with a SyncToModel(), for example) doesn't re-enter this with a half-cleared
// state. The cell is detached last so that a MarkForRedraw() from that same callback can still find the window by
// walking up the parent chain. The guard covers an application that re-parented the panel elsewhere in the meantime.
func (t *Table[T]) releaseFocusedCell(refocus bool) {
	cell := t.focusedCell
	if cell == nil {
		return
	}
	var zero T
	t.focusedCell = nil
	t.focusedCellRow = zero
	t.focusedCellRowIndex = -1
	t.focusedCellColumn = -1
	if refocus {
		t.RequestFocusWithoutScroll()
	}
	if cell.parent == t.AsPanel() {
		cell.parent = nil
	}
}

// validateFocusedCell checks that the focused cell still holds the keyboard focus and still has a row and column to
// live in, releasing it if not and bringing its frame up to date if so. Rather than consuming the table's public
// FocusChangeInHierarchyCallback, which belongs to the application, the state is validated lazily like this from the
// paths that are about to rely on it: drawing, mouse handling and the model syncs.
func (t *Table[T]) validateFocusedCell() {
	if t.focusedCell == nil {
		return
	}
	wnd := t.Window()
	if wnd == nil {
		// A table that isn't in a window has nothing to hand the focus back to, so leave the state alone. It will be
		// validated again once the table is part of a window.
		return
	}
	inside := panelContains(t.focusedCell, wnd.focus)
	row := t.focusedCellRowIndex
	if row < 0 || row >= len(t.rowCache) || t.rowCache[row].row.ID() != t.focusedCellRow.ID() {
		// The row cache was rebuilt underneath us, so fall back to locating the row by its ID.
		row = t.RowToIndex(t.focusedCellRow)
	}
	col := t.focusedCellColumn
	if !inside || row == -1 || col < 0 || col >= len(t.Columns) {
		// Only ask for the focus back if it is still ours to give away; if it has already moved on to another panel,
		// yanking it back would be wrong.
		t.releaseFocusedCell(inside)
		return
	}
	t.focusedCellRowIndex = row
	t.setCellFrame(t.focusedCell, t.CellFrame(row, col))
}

// setCellFrame positions a cell and brings its layout up to date without letting either step propagate up into the
// table. installCell() has always done both before attaching the cell for this reason: SetFrameRect() marks the panel
// for redraw and notifies the ancestors' FrameChangeInChildHierarchyCallback when the frame changes, and
// ValidateLayout() marks it for redraw whenever it actually lays something out. A cell that is merely being positioned
// as part of drawing the table must not schedule yet another draw, and a row that builds a fresh wrapper panel on each
// ColumnCell() call always hands back one that needs to be laid out, so an attached cell would mark the window for
// redraw on every single draw and the table would never stop drawing. The focused cell, and a freshly created wrapper
// that was adopted the moment the row handed it back, are already attached by the time they get here, so the parent is
// unhooked for the duration of the call.
func (t *Table[T]) setCellFrame(cell *Panel, frame geom.Rect) {
	parent := cell.parent
	cell.parent = nil
	cell.SetFrameRect(frame)
	cell.ValidateLayout()
	cell.parent = parent
}

func (t *Table[T]) installCell(cell *Panel, frame geom.Rect) {
	t.setCellFrame(cell, frame)
	cell.parent = t.AsPanel()
}

// uninstallCell detaches a cell that installCell() attached, with one exception. The cell is first offered the chance
// to become the focused cell, since a widget inside it may have taken the keyboard focus while handling the event that
// was just forwarded to it, and the cell that ends up holding the focus is then left attached to the table.
func (t *Table[T]) uninstallCell(cell *Panel, row, col int) {
	if row >= 0 && row < len(t.rowCache) && col >= 0 && col < len(t.Columns) {
		t.adoptCellIfFocused(cell, t.rowCache[row].row, row, col)
	}
	if cell != t.focusedCell {
		cell.parent = nil
	}
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
	var insets geom.Insets
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
	var insets geom.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	end := insets.Left + t.leadingColumnDividerWidth()
	for i := range t.Columns {
		start := end
		end += t.Columns[i].Current
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
// over any column divider. The last column's right edge is also considered a resizable divider.
func (t *Table[T]) OverColumnDivider(x float32) int {
	if len(t.Columns) == 0 {
		return -1
	}
	var insets geom.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	pos := insets.Left + t.leadingColumnDividerWidth()
	for i := range t.Columns {
		pos += t.Columns[i].Current
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
	if row < 0 || col < 0 || row >= len(t.rowCache) || col >= len(t.Columns) {
		return 0
	}
	width := t.Columns[col].Current - (t.Padding.Left + t.Padding.Right)
	if t.Columns[col].ID == t.HierarchyColumnID {
		if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
			width -= hierarchyIndent*float32(t.rowCache[row].depth+1) + t.Padding.Left
		}
	}
	return width
}

// ColumnEdges returns the x-coordinates of the left and right sides of the column.
func (t *Table[T]) ColumnEdges(col int) (left, right float32) {
	if col < 0 || col >= len(t.Columns) {
		return 0, 0
	}
	var insets geom.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	left = insets.Left + t.leadingColumnDividerWidth()
	for c := range col {
		left += t.Columns[c].Current
		if t.ShowColumnDivider {
			left++
		}
	}
	right = left + t.Columns[col].Current
	left += t.Padding.Left
	right -= t.Padding.Right
	if t.Columns[col].ID == t.HierarchyColumnID {
		if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
			left += hierarchyIndent + t.Padding.Left
		}
	}
	if right < left {
		right = left
	}
	return left, right
}

// CellFrame returns the frame of the given cell.
func (t *Table[T]) CellFrame(row, col int) geom.Rect {
	if row < 0 || col < 0 || row >= len(t.rowCache) || col >= len(t.Columns) {
		return geom.Rect{}
	}
	var insets geom.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}
	x := insets.Left + t.leadingColumnDividerWidth()
	for c := range col {
		x += t.Columns[c].Current
		if t.ShowColumnDivider && (t.ShowLastColumnDivider || c < len(t.Columns)-1) {
			x++
		}
	}
	y := insets.Top
	for r := range row {
		y += t.rowCache[r].height
		if t.ShowRowDivider {
			y++
		}
	}
	rect := geom.NewRect(x, y, t.Columns[col].Current, t.rowCache[row].height).Inset(t.Padding)
	if t.Columns[col].ID == t.HierarchyColumnID {
		if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
			indent := hierarchyIndent*float32(t.rowCache[row].depth+1) + t.Padding.Left
			rect.X += indent
			rect.Width -= indent
			if rect.Width < 1 {
				rect.Width = 1
			}
		}
	}
	return rect
}

// RowFrame returns the frame of the row.
func (t *Table[T]) RowFrame(row int) geom.Rect {
	if row < 0 || row >= len(t.rowCache) {
		return geom.Rect{}
	}
	rect := t.ContentRect(false)
	for i := range row {
		rect.Y += t.rowCache[i].height
		if t.ShowRowDivider {
			rect.Y++
		}
	}
	rect.Height = t.rowCache[row].height
	return rect
}

func (t *Table[T]) newTableHitRect(rect geom.Rect, row T) tableHitRect {
	return tableHitRect{
		Rect: rect,
		handler: func() {
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
	if !t.noScrollOnFocus {
		switch {
		case t.interactionRow != -1:
			t.ScrollRowIntoView(t.interactionRow)
		case t.lastMouseMotionRow != -1:
			t.ScrollRowIntoView(t.lastMouseMotionRow)
		default:
			t.ScrollIntoView()
		}
	}
	t.MarkForRedraw()
}

// DefaultUpdateCursorCallback provides the default cursor update handling.
func (t *Table[T]) DefaultUpdateCursorCallback(where geom.Point) *Cursor {
	if !t.PreventUserColumnResize {
		if over := t.OverColumnDivider(where.X); over != -1 {
			if t.Columns[over].resizable() {
				return ResizeHorizontalCursor()
			}
		}
	}
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.HasInSelfOrDescendants(func(p *Panel) bool { return p.UpdateCursorCallback != nil }) {
				var cursor *Cursor
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where = where.Sub(rect.Point)
				target := cell.PanelAt(where)
				for target != t.AsPanel() {
					if target.UpdateCursorCallback == nil {
						target = target.parent
					} else {
						SafeCall(func() { cursor = target.UpdateCursorCallback(cell.PointTo(where, target)) })
						break
					}
				}
				t.uninstallCell(cell, row, col)
				return cursor
			}
		}
	}
	return nil
}

// DefaultUpdateTooltipCallback provides the default tooltip update handling.
func (t *Table[T]) DefaultUpdateTooltipCallback(where geom.Point, avoid geom.Rect) geom.Rect {
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.HasInSelfOrDescendants(func(p *Panel) bool { return p.UpdateTooltipCallback != nil || p.Tooltip != nil }) {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where = where.Sub(rect.Point)
				target := cell.PanelAt(where)
				t.Tooltip = nil
				t.TooltipImmediate = false
				for target != t.AsPanel() {
					avoid = target.RectToRoot(target.ContentRect(true)).Align()
					if target.UpdateTooltipCallback != nil {
						SafeCall(func() { avoid = target.UpdateTooltipCallback(cell.PointTo(where, target), avoid) })
					}
					if target.Tooltip != nil {
						t.Tooltip = target.Tooltip
						t.TooltipImmediate = target.TooltipImmediate
						break
					}
					target = target.parent
				}
				t.uninstallCell(cell, row, col)
				return avoid
			}
			if cell.Tooltip != nil {
				t.Tooltip = cell.Tooltip
				t.TooltipImmediate = cell.TooltipImmediate
				return t.RectToRoot(t.CellFrame(row, col)).Align()
			}
		}
	}
	t.Tooltip = nil
	return geom.Rect{}
}

// DefaultMouseEnter provides the default mouse enter handling.
func (t *Table[T]) DefaultMouseEnter(where geom.Point, mods mod.Modifiers) bool {
	row := t.OverRow(where.Y)
	col := t.OverColumn(where.X)
	if t.lastMouseMotionRow != row || t.lastMouseMotionColumn != col {
		t.DefaultMouseExit()
		t.lastMouseMotionRow = row
		t.lastMouseMotionColumn = col
	}
	if row != -1 && col != -1 {
		cell := t.cell(row, col)
		rect := t.CellFrame(row, col)
		t.installCell(cell, rect)
		where = where.Sub(rect.Point)
		target := cell.PanelAt(where)
		if target != t.lastMouseEnterCellPanel && t.lastMouseEnterCellPanel != nil {
			t.DefaultMouseExit()
			t.lastMouseMotionRow = row
			t.lastMouseMotionColumn = col
		}
		if target.MouseEnterCallback != nil {
			SafeCall(func() { target.MouseEnterCallback(cell.PointTo(where, target), mods) })
		}
		t.uninstallCell(cell, row, col)
		t.lastMouseEnterCellPanel = target
	}
	return true
}

// DefaultMouseMove provides the default mouse move handling.
func (t *Table[T]) DefaultMouseMove(where geom.Point, mods mod.Modifiers) bool {
	t.DefaultMouseEnter(where, mods)
	if t.lastMouseEnterCellPanel != nil {
		row := t.OverRow(where.Y)
		col := t.OverColumn(where.X)
		cell := t.cell(row, col)
		rect := t.CellFrame(row, col)
		t.installCell(cell, rect)
		where = where.Sub(rect.Point)
		if target := cell.PanelAt(where); target.MouseMoveCallback != nil {
			SafeCall(func() { target.MouseMoveCallback(cell.PointTo(where, target), mods) })
		}
		t.uninstallCell(cell, row, col)
	}
	return true
}

// DefaultMouseExit provides the default mouse exit handling.
func (t *Table[T]) DefaultMouseExit() bool {
	if t.lastMouseEnterCellPanel != nil && t.lastMouseEnterCellPanel.MouseExitCallback != nil &&
		t.lastMouseMotionRow >= 0 && t.lastMouseMotionRow < len(t.rowCache) &&
		t.lastMouseMotionColumn >= 0 && t.lastMouseMotionColumn < len(t.Columns) {
		cell := t.cell(t.lastMouseMotionRow, t.lastMouseMotionColumn)
		rect := t.CellFrame(t.lastMouseMotionRow, t.lastMouseMotionColumn)
		t.installCell(cell, rect)
		SafeCall(func() { t.lastMouseEnterCellPanel.MouseExitCallback() })
		t.uninstallCell(cell, t.lastMouseMotionRow, t.lastMouseMotionColumn)
	}
	t.lastMouseEnterCellPanel = nil
	t.lastMouseMotionRow = -1
	t.lastMouseMotionColumn = -1
	return true
}

// DefaultMouseDown provides the default mouse down handling.
func (t *Table[T]) DefaultMouseDown(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
	t.validateFocusedCell()
	row := t.OverRow(where.Y)
	col := t.OverColumn(where.X)
	if t.focusedCell == nil || row != t.focusedCellRowIndex || col != t.focusedCellColumn {
		// Don't take the focus away from a cell that already has it when the press lands inside that same cell.
		// Bouncing the focus out to the table and back again would make the cell's widget treat this as the click that
		// first gave it the focus, so a Field would select all of its text instead of just moving the caret to the
		// spot that was clicked.
		t.RequestFocusWithoutScroll()
	}
	t.wasDragged = false
	t.dividerDrag = false
	t.lastSel = ""
	t.pressedHitRect = geom.Rect{}

	t.interactionRow = -1
	t.interactionColumn = -1
	if button == ButtonLeft {
		if !t.PreventUserColumnResize {
			if over := t.OverColumnDivider(where.X); over != -1 {
				if t.Columns[over].resizable() {
					if clickCount == 2 {
						t.SizeColumnToFit(over, true)
						t.MarkForRedraw()
						t.Window().UpdateCursorNow()
						return true
					}
					t.interactionColumn = over
					t.columnResizeStart = where.X
					t.columnResizeBase = t.Columns[over].Current
					t.columnResizeOverhead = t.Padding.Left + t.Padding.Right
					if t.Columns[over].ID == t.HierarchyColumnID {
						if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
							depth := 0
							for _, cache := range t.rowCache {
								if depth < cache.depth {
									depth = cache.depth
								}
							}
							t.columnResizeOverhead += t.Padding.Left + hierarchyIndent*float32(depth+1)
						}
					}
					return true
				}
			}
		}
		for _, one := range t.hitRects {
			if where.In(one.Rect) {
				t.pressedHitRect = one.Rect
				return true
			}
		}
	}
	if row != -1 {
		if col != -1 {
			cell := t.cell(row, col)
			if cell.HasInSelfOrDescendants(func(p *Panel) bool { return p.MouseDownCallback != nil }) {
				t.interactionRow = row
				t.interactionColumn = col
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where = where.Sub(rect.Point)
				stop := false
				if target := cell.PanelAt(where); target.MouseDownCallback != nil {
					t.lastMouseDownCellPanel = target
					SafeCall(func() {
						stop = target.MouseDownCallback(cell.PointTo(where, target), button, clickCount, mods)
					})
				}
				t.uninstallCell(cell, row, col)
				if stop {
					return stop
				}
			}
		}
		rowData := t.rowCache[row].row
		id := rowData.ID()
		switch {
		case mods&mod.Shift != 0: // Extend selection from anchor
			selAnchorIndex := -1
			if t.selAnchor != "" {
				for i, c := range t.rowCache {
					if c.row.ID() == t.selAnchor {
						selAnchorIndex = i
						break
					}
				}
			}
			if selAnchorIndex != -1 {
				last := max(selAnchorIndex, row)
				for i := min(selAnchorIndex, row); i <= last; i++ {
					t.selMap[t.rowCache[i].row.ID()] = true
				}
				t.notifyOfSelectionChange()
			} else if !t.selMap[id] { // No anchor, so behave like a regular click
				t.selMap = make(map[tid.TID]bool)
				t.selMap[id] = true
				t.selAnchor = id
				t.notifyOfSelectionChange()
			}
		case mods.DiscontiguousSelectionDown(): // Toggle single row
			if t.selMap[id] {
				delete(t.selMap, id)
			} else {
				t.selMap[id] = true
			}
			t.notifyOfSelectionChange()
		case t.selMap[id]: // Sets lastClick so that on mouse up, we can treat a click and click and hold differently
			t.lastSel = id
		default: // If not already selected, replace selection with current row and make it the anchor
			t.selMap = make(map[tid.TID]bool)
			t.selMap[id] = true
			t.selAnchor = id
			t.notifyOfSelectionChange()
		}
		t.MarkForRedraw()
		if button == ButtonLeft && clickCount == 2 && t.DoubleClickCallback != nil && len(t.selMap) != 0 {
			SafeCall(t.DoubleClickCallback)
		}
	}
	return true
}

func (t *Table[T]) notifyOfSelectionChange() {
	SafeCall(t.SelectionChangedCallback)
}

// DefaultMouseDrag provides the default mouse drag handling.
func (t *Table[T]) DefaultMouseDrag(where geom.Point, button int, mods mod.Modifiers) bool {
	t.wasDragged = true
	stop := false
	if t.interactionColumn != -1 {
		if t.interactionRow == -1 {
			if button == ButtonLeft && !t.PreventUserColumnResize {
				width := t.columnResizeBase + where.X - t.columnResizeStart
				if width < t.columnResizeOverhead {
					width = t.columnResizeOverhead
				}
				minimum := t.Columns[t.interactionColumn].Minimum
				if minimum > 0 && width < minimum+t.columnResizeOverhead {
					width = minimum + t.columnResizeOverhead
				} else {
					maximum := t.Columns[t.interactionColumn].Maximum
					if maximum > 0 && width > maximum+t.columnResizeOverhead {
						width = maximum + t.columnResizeOverhead
					}
				}
				if t.Columns[t.interactionColumn].Current != width {
					t.Columns[t.interactionColumn].Current = width
					t.EventuallySyncToModel()
					t.MarkForRedraw()
					t.dividerDrag = true
				}
				stop = true
			}
		} else if t.lastMouseDownCellPanel != nil && t.lastMouseDownCellPanel.MouseDragCallback != nil &&
			t.interactionRow < len(t.rowCache) && t.interactionColumn < len(t.Columns) {
			cell := t.cell(t.interactionRow, t.interactionColumn)
			rect := t.CellFrame(t.interactionRow, t.interactionColumn)
			t.installCell(cell, rect)
			where = where.Sub(rect.Point)
			SafeCall(func() {
				stop = t.lastMouseDownCellPanel.MouseDragCallback(cell.PointTo(where, t.lastMouseDownCellPanel),
					button, mods)
			})
			t.uninstallCell(cell, t.interactionRow, t.interactionColumn)
		}
	}
	return stop
}

// DefaultMouseUp provides the default mouse up handling.
func (t *Table[T]) DefaultMouseUp(where geom.Point, button int, mods mod.Modifiers) bool {
	stop := false
	if !t.dividerDrag && button == ButtonLeft && !t.pressedHitRect.Empty() {
		// Only fire a hit rect's handler when the press began on that same hit rect, so that a gesture started
		// elsewhere (e.g. a row selection drag) releasing over a disclosure triangle does not toggle it.
		for _, one := range t.hitRects {
			if one.Rect == t.pressedHitRect && where.In(one.Rect) {
				one.handler()
				stop = true
				break
			}
		}
		t.pressedHitRect = geom.Rect{}
	}

	if !t.wasDragged && t.lastSel != "" {
		t.ClearSelection()
		t.selMap[t.lastSel] = true
		t.selAnchor = t.lastSel
		t.MarkForRedraw()
		t.notifyOfSelectionChange()
	}

	if !stop && t.interactionRow != -1 && t.interactionColumn != -1 && t.interactionRow < len(t.rowCache) &&
		t.interactionColumn < len(t.Columns) && t.lastMouseDownCellPanel != nil &&
		t.lastMouseDownCellPanel.MouseUpCallback != nil {
		cell := t.cell(t.interactionRow, t.interactionColumn)
		rect := t.CellFrame(t.interactionRow, t.interactionColumn)
		t.installCell(cell, rect)
		where = where.Sub(rect.Point)
		SafeCall(func() {
			stop = t.lastMouseDownCellPanel.MouseUpCallback(cell.PointTo(where, t.lastMouseDownCellPanel), button, mods)
		})
		t.uninstallCell(cell, t.interactionRow, t.interactionColumn)
	}
	t.lastMouseDownCellPanel = nil
	t.interactionRow = -1
	t.interactionColumn = -1
	return stop
}

// DefaultKeyDown provides the default key down handling.
func (t *Table[T]) DefaultKeyDown(keyCode KeyCode, mods mod.Modifiers, repeat bool) bool {
	t.validateFocusedCell()
	if t.focusedCell != nil {
		// The key bubbled up from a widget inside the focused cell, since that is the only way one can arrive here
		// while a cell holds the focus: the window dispatches keys by walking from the focus up the parent chain, and
		// the focused cell's chain runs through the table. The widget didn't want the key, but that doesn't make it
		// ours to act on -- running the row navigation or the Space-to-DoubleClickCallback shortcut below would move
		// the selection out from under something the user is editing -- so only the keys that mean "leave this cell"
		// are handled, and everything else is ignored.
		switch keyCode {
		case KeyTab:
			if mods&(mod.NonSticky&^mod.Shift) == 0 {
				if t.focusAdjacentCell(!mods.ShiftDown()) {
					return true
				}
				// There is no cell left to move to in that direction, so take the focus back and report the key as
				// unhandled. Window.keyPressed() then applies its own Tab fallback, which traverses from the window's
				// focus -- the table, now -- and lands on the panel just after or just before it, which is exactly what
				// a Tab from the table itself would have done.
				t.RequestFocusWithoutScroll()
			}
			return false
		case KeyReturn, KeyNumPadEnter, KeyEscape:
			// Taking the focus back ends the editing; the widget's LostFocusCallback fires from within SetFocus().
			t.RequestFocusWithoutScroll()
			return true
		default:
			return false
		}
	}
	if IsControlAction(keyCode, mods) {
		if t.DoubleClickCallback != nil && len(t.selMap) != 0 {
			SafeCall(t.DoubleClickCallback)
		}
		return true
	}
	switch keyCode {
	case KeyLeft:
		if !repeat && t.HasSelection() {
			altered := false
			for _, row := range t.SelectedRows(false) {
				if mods.OptionDown() {
					if setOpenRecursively(row, false) {
						altered = true
					}
				} else if row.IsOpen() {
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
		if !repeat && t.HasSelection() {
			altered := false
			for _, row := range t.SelectedRows(false) {
				if mods.OptionDown() {
					if setOpenRecursively(row, true) {
						altered = true
					}
				} else if !row.IsOpen() {
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
			i = max(t.FirstSelectedRowIndex()-1, 0)
		} else {
			i = len(t.rowCache) - 1
		}
		if !mods.ShiftDown() {
			t.ClearSelection()
		}
		t.SelectByIndex(i)
		t.ScrollRowCellIntoView(i, 0)
	case KeyDown:
		i := min(t.LastSelectedRowIndex()+1, len(t.rowCache)-1)
		if !mods.ShiftDown() {
			t.ClearSelection()
		}
		t.SelectByIndex(i)
		t.ScrollRowCellIntoView(i, 0)
	case KeyHome:
		if mods.ShiftDown() && t.HasSelection() {
			t.SelectRange(0, t.FirstSelectedRowIndex())
		} else {
			t.ClearSelection()
			t.SelectByIndex(0)
		}
		t.ScrollRowCellIntoView(0, 0)
	case KeyEnd:
		if mods.ShiftDown() && t.HasSelection() {
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

func setOpenRecursively[T TableRowConstraint[T]](row T, open bool) bool {
	altered := false
	if row.IsOpen() != open {
		row.SetOpen(open)
		altered = true
	}
	if row.CanHaveChildren() {
		for _, child := range row.Children() {
			if setOpenRecursively(child, open) {
				altered = true
			}
		}
	}
	return altered
}

// focusCellAt attempts to give the keyboard focus to the cell at the given row and column, scrolling it into view if it
// succeeds. 'forward' says which end of the cell to enter when the cell itself isn't focusable but has focusable
// content within it, so that tabbing through a cell's widgets continues in the direction the user is moving. The cell
// has to be installed in the table before the focus can be handed to it, since Window.SetFocus() only accepts a panel
// that is already part of that window; the matching uninstallCell() is what adopts it as the focused cell if the focus
// did land inside it. Note that acquiring the cell asks the row for it via ColumnCell(), which is expected here.
func (t *Table[T]) focusCellAt(row, col int, forward bool) bool {
	if row < 0 || row >= len(t.rowCache) || col < 0 || col >= len(t.Columns) {
		return false
	}
	wnd := t.Window()
	if wnd == nil {
		return false
	}
	cell := t.cell(row, col)
	if cell.Hidden {
		return false
	}
	target := cell
	if !target.Focusable() {
		if forward {
			target = cell.FirstFocusableChild()
		} else {
			target = cell.LastFocusableChild()
		}
		if target == nil {
			// Nothing in this cell can take the focus. Don't hand a non-focusable panel to SetFocus(), since it would
			// remove the focus entirely rather than leaving it where it was.
			return false
		}
	}
	t.installCell(cell, t.CellFrame(row, col))
	wnd.SetFocus(target)
	t.uninstallCell(cell, row, col)
	t.ScrollRowCellIntoView(row, col)
	return t.focusedCell == cell
}

// focusAdjacentCell moves the keyboard focus from the currently focused cell to the next ('forward' is true) or the
// previous focusable cell, in row-major order. It deliberately doesn't wrap around: returning false at either end is
// what lets a Tab out of the last cell continue on to whatever follows the table.
func (t *Table[T]) focusAdjacentCell(forward bool) bool {
	if t.focusedCell == nil {
		return false
	}
	wnd := t.Window()
	if wnd == nil {
		return false
	}
	// A cell can hold more than one focusable widget, so exhaust the current cell's own focus chain before looking at
	// any other cell.
	if i, focusables := collectFocusables(t.focusedCell, wnd.focus, nil); i != -1 {
		if forward {
			if i+1 < len(focusables) {
				wnd.SetFocus(focusables[i+1])
				return true
			}
		} else if i > 0 {
			wnd.SetFocus(focusables[i-1])
			return true
		}
	}
	row := t.focusedCellRowIndex
	col := t.focusedCellColumn
	if forward {
		for c := col + 1; c < len(t.Columns); c++ {
			if t.focusCellAt(row, c, true) {
				return true
			}
		}
		for r := row + 1; r < len(t.rowCache); r++ {
			for c := range len(t.Columns) {
				if t.focusCellAt(r, c, true) {
					return true
				}
			}
		}
		return false
	}
	for c := col - 1; c >= 0; c-- {
		if t.focusCellAt(row, c, false) {
			return true
		}
	}
	for r := row - 1; r >= 0; r-- {
		for c := range slices.Backward(t.Columns) {
			if t.focusCellAt(r, c, false) {
				return true
			}
		}
	}
	return false
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
	oldLen := len(t.selMap)
	selMap := make(map[tid.TID]bool, oldLen)
	for _, entry := range t.rowCache {
		id := entry.row.ID()
		if t.selMap[id] {
			selMap[id] = true
		}
	}
	t.selMap = selMap
	if len(selMap) != oldLen {
		t.notifyOfSelectionChange()
	}
}

// FirstSelectedRowIndex returns the first selected row index, or -1 if there is no selection.
func (t *Table[T]) FirstSelectedRowIndex() int {
	if len(t.selMap) == 0 {
		return -1
	}
	for i, entry := range t.rowCache {
		if t.selMap[entry.row.ID()] {
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
		if t.selMap[t.rowCache[i].row.ID()] {
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
		if t.selMap[t.rowCache[index].row.ID()] {
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
	return t.selMap[t.rowCache[index].row.ID()]
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
		if t.selMap[entry.row.ID()] && (!minimal || entry.parent == -1 || !t.IsRowOrAnyParentSelected(entry.parent)) {
			rows = append(rows, entry.row)
		}
	}
	return rows
}

// CopySelectionMap returns a copy of the current selection map.
func (t *Table[T]) CopySelectionMap() map[tid.TID]bool {
	t.PruneSelectionOfUndisclosedNodes()
	return copySelMap(t.selMap)
}

// SetSelectionMap sets the current selection map.
func (t *Table[T]) SetSelectionMap(selMap map[tid.TID]bool) {
	t.selMap = copySelMap(selMap)
	t.selNeedsPrune = true
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

func copySelMap(selMap map[tid.TID]bool) map[tid.TID]bool {
	result := make(map[tid.TID]bool, len(selMap))
	maps.Copy(result, selMap)
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
	t.selMap = make(map[tid.TID]bool)
	t.selNeedsPrune = false
	t.selAnchor = ""
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// SelectAll selects all rows.
func (t *Table[T]) SelectAll() {
	t.selMap = make(map[tid.TID]bool, len(t.rowCache))
	t.selNeedsPrune = false
	t.selAnchor = ""
	for _, cache := range t.rowCache {
		id := cache.row.ID()
		t.selMap[id] = true
		if t.selAnchor == "" {
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
			id := t.rowCache[index].row.ID()
			t.selMap[id] = true
			t.selNeedsPrune = true
			if t.selAnchor == "" {
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
	start = max(start, 0)
	end = min(end, len(t.rowCache)-1)
	if start > end {
		return
	}
	for i := start; i <= end; i++ {
		id := t.rowCache[i].row.ID()
		t.selMap[id] = true
		t.selNeedsPrune = true
		if t.selAnchor == "" {
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
			delete(t.selMap, t.rowCache[index].row.ID())
		}
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// DeselectRange deselects the given range.
func (t *Table[T]) DeselectRange(start, end int) {
	start = max(start, 0)
	end = min(end, len(t.rowCache)-1)
	if start > end {
		return
	}
	for i := start; i <= end; i++ {
		delete(t.selMap, t.rowCache[i].row.ID())
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
	t.selMap = make(map[tid.TID]bool)
	t.selNeedsPrune = false
	t.selAnchor = ""
	t.SyncToModel()
}

// SyncToModel causes the table to update its internal caches to reflect the current model.
func (t *Table[T]) SyncToModel() {
	rowCount := 0
	roots := t.RootRows()
	t.hasHierarchy = false
	if t.filteredRows != nil {
		rowCount = len(t.filteredRows)
		for _, row := range t.filteredRows {
			if !t.hasHierarchy && row.CanHaveChildren() {
				t.hasHierarchy = true
			}
		}
	} else {
		for _, row := range roots {
			if !t.hasHierarchy && row.CanHaveChildren() {
				t.hasHierarchy = true
			}
			rowCount += t.countOpenRowChildrenRecursively(row)
		}
	}
	t.rowCache = make([]tableCache[T], rowCount)
	j := 0
	for _, row := range roots {
		j = t.buildRowCacheEntry(row, -1, j, 0)
	}
	t.selNeedsPrune = true
	_, pref, _ := t.DefaultSizes(geom.Size{})
	rect := t.FrameRect()
	rect.Size = pref
	t.SetFrameRect(rect)
	t.MarkForRedraw()
	t.MarkForLayoutRecursivelyUpward()
	// The row cache was just thrown away and rebuilt, so a cell that holds the keyboard focus needs to have its row
	// located again, or be released if its row is no longer being displayed at all.
	t.validateFocusedCell()
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
	for col := range t.Columns {
		w := t.Columns[col].Current
		if w <= 0 {
			continue
		}
		w -= t.Padding.Left + t.Padding.Right
		if t.Columns[col].ID == t.HierarchyColumnID {
			if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
				w -= t.Padding.Left + hierarchyIndent*float32(depth+1)
			}
		}
		size := t.cellPrefSize(rowData, row, col, w)
		size.Height += t.Padding.Top + t.Padding.Bottom
		if height < size.Height {
			height = size.Height
		}
	}
	return max(xmath.Ceil(height), t.MinimumRowHeight)
}

func (t *Table[T]) cellPrefSize(rowData T, row, col int, widthConstraint float32) geom.Size {
	_, size, _ := t.cellFor(rowData, row, col).Sizes(geom.NewSize(widthConstraint, 0))
	return size
}

// SizeColumnsToFitWithExcessIn sizes each column to its preferred size, with the exception of the column with the given
// ID, which gets set to any remaining width left over. If the provided column ID doesn't exist, the first column will
// be used instead.
func (t *Table[T]) SizeColumnsToFitWithExcessIn(columnID int) {
	excessColumnIndex := max(t.ColumnIndexForID(columnID), 0)
	current := make([]float32, len(t.Columns))
	for col := range t.Columns {
		current[col] = max(t.Columns[col].Minimum, 0)
		t.Columns[col].Current = 0
	}
	for row, cache := range t.rowCache {
		for col := range t.Columns {
			if col == excessColumnIndex {
				continue
			}
			pref := t.cellPrefSize(cache.row, row, col, 0)
			minimum := t.Columns[col].AutoMinimum
			if minimum > 0 && pref.Width < minimum {
				pref.Width = minimum
			} else {
				maximum := t.Columns[col].AutoMaximum
				if maximum > 0 && pref.Width > maximum {
					pref.Width = maximum
				}
			}
			pref.Width += t.Padding.Left + t.Padding.Right
			if t.Columns[col].ID == t.HierarchyColumnID {
				if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
					pref.Width += t.Padding.Left + hierarchyIndent*float32(cache.depth+1)
				}
			}
			if current[col] < pref.Width {
				current[col] = pref.Width
			}
		}
	}
	width := t.ContentRect(false).Width
	if t.ShowColumnDivider {
		width -= float32(len(t.Columns)) + t.leadingColumnDividerWidth()
		if !t.ShowLastColumnDivider && len(t.Columns) != 0 {
			width++
		}
	}
	for col := range current {
		if col == excessColumnIndex {
			continue
		}
		t.Columns[col].Current = current[col]
		width -= current[col]
	}
	t.Columns[excessColumnIndex].Current = max(width, t.Columns[excessColumnIndex].Minimum)
	t.SyncRowHeights()
}

// SizeColumnsToFit sizes each column to its preferred size. If 'adjust' is true, the Table's FrameRect will be set to
// its preferred size as well.
func (t *Table[T]) SizeColumnsToFit(adjust bool) {
	current := make([]float32, len(t.Columns))
	for col := range t.Columns {
		current[col] = max(t.Columns[col].Minimum, 0)
		t.Columns[col].Current = 0
	}
	for row, cache := range t.rowCache {
		for col := range t.Columns {
			pref := t.cellPrefSize(cache.row, row, col, 0)
			minimum := t.Columns[col].AutoMinimum
			if minimum > 0 && pref.Width < minimum {
				pref.Width = minimum
			} else {
				maximum := t.Columns[col].AutoMaximum
				if maximum > 0 && pref.Width > maximum {
					pref.Width = maximum
				}
			}
			pref.Width += t.Padding.Left + t.Padding.Right
			if t.Columns[col].ID == t.HierarchyColumnID {
				if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
					pref.Width += t.Padding.Left + hierarchyIndent*float32(cache.depth+1)
				}
			}
			if current[col] < pref.Width {
				current[col] = pref.Width
			}
		}
	}
	for col := range current {
		t.Columns[col].Current = current[col]
	}
	t.SyncRowHeights()
	if adjust {
		_, pref, _ := t.DefaultSizes(geom.Size{})
		rect := t.FrameRect()
		rect.Size = pref
		t.SetFrameRect(rect)
	}
}

// SizeColumnToFit sizes the specified column to its preferred size. If 'adjust' is true, the Table's FrameRect will be
// set to its preferred size as well.
func (t *Table[T]) SizeColumnToFit(col int, adjust bool) {
	if col < 0 || col >= len(t.Columns) {
		return
	}
	current := max(t.Columns[col].Minimum, 0)
	t.Columns[col].Current = 0
	for row, cache := range t.rowCache {
		pref := t.cellPrefSize(cache.row, row, col, 0)
		minimum := t.Columns[col].AutoMinimum
		if minimum > 0 && pref.Width < minimum {
			pref.Width = minimum
		} else {
			maximum := t.Columns[col].AutoMaximum
			if maximum > 0 && pref.Width > maximum {
				pref.Width = maximum
			}
		}
		pref.Width += t.Padding.Left + t.Padding.Right
		if t.Columns[col].ID == t.HierarchyColumnID {
			if hierarchyIndent := t.CurrentHierarchyIndent(); hierarchyIndent > 0 {
				pref.Width += t.Padding.Left + hierarchyIndent*float32(cache.depth+1)
			}
		}
		if current < pref.Width {
			current = pref.Width
		}
	}
	t.Columns[col].Current = current
	t.SyncRowHeights()
	if adjust {
		_, pref, _ := t.DefaultSizes(geom.Size{})
		rect := t.FrameRect()
		rect.Size = pref
		t.SetFrameRect(rect)
	}
}

// SyncRowHeights recalculates the height of each row based on the current column widths. Call this after adjusting
// column widths directly (i.e. outside of the SizeColumns* methods) so that rows whose content wraps to a height that
// depends on the available width are given the correct amount of vertical space.
func (t *Table[T]) SyncRowHeights() {
	for row, cache := range t.rowCache {
		t.rowCache[row].height = t.heightForColumns(cache.row, row, cache.depth)
	}
	// Rows have moved vertically, so a cell that holds the keyboard focus needs its frame brought up to date.
	t.validateFocusedCell()
}

// EventuallySizeColumnsToFit sizes each column to its preferred size after a short delay, allowing multiple
// back-to-back calls to this function to only do work once. If 'adjust' is true, the Table's FrameRect will be set to
// its preferred size as well.
func (t *Table[T]) EventuallySizeColumnsToFit(adjust bool) {
	if !t.awaitingSizeColumnsToFit {
		t.awaitingSizeColumnsToFit = true
		InvokeTaskAfter(func() {
			defer func() { t.awaitingSizeColumnsToFit = false }()
			t.SizeColumnsToFit(adjust)
		}, 20*time.Millisecond)
	}
}

// EventuallySyncToModel syncs the table to its underlying model after a short delay, allowing multiple back-to-back
// calls to this function to only do work once.
func (t *Table[T]) EventuallySyncToModel() {
	if !t.awaitingSyncToModel {
		t.awaitingSyncToModel = true
		InvokeTaskAfter(func() {
			defer func() { t.awaitingSyncToModel = false }()
			t.SyncToModel()
		}, 20*time.Millisecond)
	}
}

// DefaultSizes provides the default sizing.
func (t *Table[T]) DefaultSizes(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
	for col := range t.Columns {
		prefSize.Width += t.Columns[col].Current
	}
	startRow, endBeforeRow := t.CurrentDrawRowRange()
	for _, cache := range t.rowCache[startRow:endBeforeRow] {
		prefSize.Height += cache.height
	}
	if t.ShowColumnDivider {
		prefSize.Width += float32(len(t.Columns)) + t.leadingColumnDividerWidth()
		if !t.ShowLastColumnDivider && len(t.Columns) != 0 {
			prefSize.Width--
		}
	}
	if t.ShowRowDivider && endBeforeRow > startRow {
		prefSize.Height += float32((endBeforeRow - startRow) - 1)
	}
	if border := t.Border(); border != nil {
		prefSize = prefSize.Add(border.Insets().Size())
	}
	prefSize = prefSize.Ceil()
	return prefSize, prefSize, prefSize
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
	id := rowData.ID()
	for row, data := range t.rowCache {
		if data.row.ID() == id {
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
	if frame := t.RowFrame(row); !frame.Empty() {
		t.ScrollRectIntoView(frame)
	}
}

// ScrollRowCellIntoView scrolls the cell from the row and column at the given indexes into view.
func (t *Table[T]) ScrollRowCellIntoView(row, col int) {
	if frame := t.CellFrame(row, col); !frame.Empty() {
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
func (t *Table[T]) InstallDragSupport(svg *SVG, dataType *uti.DataType, singularName, pluralName string) {
	orig := t.MouseDragCallback
	t.MouseDragCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
		if orig != nil && orig(where, button, mods) {
			return true
		}
		if dragTableData == nil && button == ButtonLeft && t.HasSelection() && t.IsDragGesture(where) {
			data := &TableDragData[T]{
				Table: t,
				Rows:  t.SelectedRows(true),
			}
			drawable := NewTableDragDrawable(data, svg, singularName, pluralName)
			size := drawable.LogicalSize()
			img, err := NewImageFromDrawing(int(size.Width), int(size.Height), 144, func(c *Canvas) {
				drawable.DrawInRect(c, geom.Rect{Size: size}, nil, nil)
			})
			if err != nil {
				errs.Log(err)
				return true
			}
			where.X -= size.Width / 2
			where.Y -= size.Height / 2
			dragTableData = data
			t.StartDrag(img, where, func() { dragTableData = nil }, drag.Copy|drag.Move, drag.Data{
				Type: dataType,
				Data: []byte{0},
			})
		}
		return true
	}
}

// InstallDropSupport installs default drop support into this table. This will replace any existing
// CanAcceptDropCallback, DragEnteredCallback, DragUpdatedCallback, DragExitedCallback, and DropCallback functions. It
// will also chain a function to any existing DrawOverCallback. The shouldMoveDataCallback is called when a drop is
// about to occur to determine if the data should be moved (i.e. removed from the source) or copied to the destination.
// The willDropCallback is called before the actual data changes are made, giving an opportunity to start an undo
// event, which should be returned. The didDropCallback is called after data changes are made and is passed the undo
// event (if any) returned by the willDropCallback, so that the undo event can be completed and posted.
func (t *Table[T]) InstallDropSupport[U any](dataType *uti.DataType, shouldMoveDataCallback func(from, to *Table[T]) bool, willDropCallback func(from, to *Table[T], move bool) *UndoEdit[U], didDropCallback func(undo *UndoEdit[U], from, to *Table[T], move bool)) *TableDrop[T, U] {
	drop := &TableDrop[T, U]{
		Table:                  t,
		DataType:               dataType,
		originalDrawOver:       t.DrawOverCallback,
		shouldMoveDataCallback: shouldMoveDataCallback,
		willDropCallback:       willDropCallback,
		didDropCallback:        didDropCallback,
	}
	t.DrawOverCallback = drop.DrawOverCallback
	t.CanAcceptDropCallback = drop.CanAcceptDropCallback
	t.DragEnteredCallback = drop.DragEnterCallback
	t.DragUpdatedCallback = drop.DragUpdatedCallback
	t.DragExitedCallback = drop.DragExitCallback
	t.DropCallback = drop.DropCallback
	return drop
}

// InstallDropSupport installs default drop support into a table.
//
// Deprecated: Use [Table.InstallDropSupport] instead.
func InstallDropSupport[T TableRowConstraint[T], U any](t *Table[T], dataType *uti.DataType, shouldMoveDataCallback func(from, to *Table[T]) bool, willDropCallback func(from, to *Table[T], move bool) *UndoEdit[U], didDropCallback func(undo *UndoEdit[U], from, to *Table[T], move bool)) *TableDrop[T, U] {
	return t.InstallDropSupport(dataType, shouldMoveDataCallback, willDropCallback, didDropCallback)
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

// RequestFocusWithoutScroll requests focus for the table without scrolling to the selection or interaction row.
func (t *Table[T]) RequestFocusWithoutScroll() {
	t.noScrollOnFocus = true
	t.RequestFocus()
	t.noScrollOnFocus = false
}

// FocusCell gives the keyboard focus to the widget in the cell at the given row and column (the cell itself if it is
// focusable, otherwise its first focusable descendant), scrolling the cell into view. It returns false if the indexes
// are out of range, the table is not in a window, or the cell has nothing that can take the focus. The selection is not
// changed.
func (t *Table[T]) FocusCell(row, col int) bool {
	t.validateFocusedCell()
	return t.focusCellAt(row, col, true)
}

// FocusedCell returns the row and column of the cell whose widget currently holds the keyboard focus, or -1, -1 if none
// does.
func (t *Table[T]) FocusedCell() (row, col int) {
	t.validateFocusedCell()
	if t.focusedCell == nil {
		return -1, -1
	}
	return t.focusedCellRowIndex, t.focusedCellColumn
}
