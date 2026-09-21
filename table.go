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
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
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

// resizable returns true if the user should be permitted to resize the column, i.e. its Minimum and Maximum don't pin
// it to a single width. A Maximum of 0 or less means "no maximum", matching how the resize clamping treats it, so a
// column with only a Minimum set remains resizable.
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
	filteredRows             []T              // Note that we use the difference between nil and an empty slice here
	filterMatches            map[tid.TID]bool // The rows that passed a hierarchical filter, by ID
	filterChildren           map[tid.TID][]T  // The children a hierarchical filter kept, by the ID of their parent
	filterRoots              []T              // The root rows a hierarchical filter kept
	header                   *TableHeader[T]
	selMap                   map[tid.TID]bool
	selAnchor                tid.TID
	lead                     tid.TID
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
	hierarchicalFilter       bool // Set while the filter in force keeps the hierarchy (see ApplyHierarchicalFilter)
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
						// A hierarchical filter shows every container it kept as open, whatever the container's own
						// open state, so the disclosure triangle is drawn open and is not offered for toggling.
						if !t.hierarchicalFilter {
							t.hitRects = append(t.hitRects,
								t.newTableHitRect(geom.NewRect(left, top, disclosureSize, disclosureSize), row))
						}
						canvas.Translate(geom.NewPoint(left, top))
						if t.isRowDisclosed(row) {
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
						if t.IsFilterContextRow(row) {
							// The row is only there to show where the rows that passed the filter sit, so its
							// disclosure triangle is dimmed the way a disabled control is, for the row's cells to
							// match if they choose to.
							chevronPaint.SetColorFilter(Grayscale30Filter())
						}
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

// hasFocus returns true if the table itself or the cell that is currently holding the keyboard focus has it.
func (t *Table[T]) hasFocus() bool {
	wnd := t.Window()
	if wnd == nil || !wnd.Focused() {
		return false
	}
	focus := wnd.CurrentFocus()
	return focus == t.AsPanel() || (t.focusedCell != nil && panelContains(t.focusedCell, focus))
}

// adoptCellIfFocused makes cell the table's focused cell if the window's keyboard focus currently lies within it. Cells
// are acquired fresh from the row on every call, so a row that wraps a memoized editor in a newly created panel each
// time hands back a different cell on each call, and AddChild() will have moved the editor into that newest wrapper.
// The newest wrapper must therefore be adopted the moment it is acquired, no matter which code path asked for it,
// because otherwise the focused editor is left hanging off a panel with no window and the next call to Window.Focus()
// takes the focus away from it.
//
// The window's raw focus pointer is consulted here deliberately, rather than Window.CurrentFocus(). CurrentFocus()
// reports nil whenever the focused panel can't find its window, and that is precisely the state a focused editor is in
// at this point when its row has just moved it into a wrapper that hasn't been attached to the table yet. Adopting the
// wrapper is what reattaches it, so asking CurrentFocus() first would refuse the adoption in the one case it exists
// for.
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
		return
	}
	inside := panelContains(t.focusedCell, wnd.CurrentFocus())
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
	y := insets.Top
	for r := range row {
		y += t.rowCache[r].height
		if t.ShowRowDivider {
			y++
		}
	}
	return t.cellFrameAtY(row, col, y)
}

// cellFrameAtY returns the frame of the given cell, given the y coordinate of the top of its row. Walking the rows
// before a row is the expensive half of finding a cell, so a caller that is already walking the rows in order — the
// accessibility snapshot describing the visible ones — hands in the y it has instead of paying for it again. The row
// and column must be in range.
func (t *Table[T]) cellFrameAtY(row, col int, y float32) geom.Rect {
	rect := geom.NewRect(t.columnLeft(col), y, t.Columns[col].Current, t.rowCache[row].height).Inset(t.Padding)
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

// DefaultUpdateTooltipCallback provides the default tooltip update handling. The tooltip of the cell the pointer is
// over is handed to the window through Panel.borrowedTooltip rather than through the table's own Tooltip: the cell is
// not a panel of the table's, so what it has to say is not the table's to keep, and a description of the table built
// while the borrowed tooltip sat in Tooltip would be a description of one of its cells.
func (t *Table[T]) DefaultUpdateTooltipCallback(where geom.Point, avoid geom.Rect) geom.Rect {
	if row := t.OverRow(where.Y); row != -1 {
		if col := t.OverColumn(where.X); col != -1 {
			cell := t.cell(row, col)
			if cell.HasInSelfOrDescendants(func(p *Panel) bool { return p.UpdateTooltipCallback != nil || p.Tooltip != nil }) {
				rect := t.CellFrame(row, col)
				t.installCell(cell, rect)
				where = where.Sub(rect.Point)
				target := cell.PanelAt(where)
				t.borrowedTooltip = nil
				t.TooltipImmediate = false
				for target != t.AsPanel() {
					avoid = target.RectToRoot(target.ContentRect(true)).Align()
					if target.UpdateTooltipCallback != nil {
						SafeCall(func() { avoid = target.UpdateTooltipCallback(cell.PointTo(where, target), avoid) })
					}
					if target.Tooltip != nil {
						t.borrowedTooltip = target.Tooltip
						t.TooltipImmediate = target.TooltipImmediate
						break
					}
					target = target.parent
				}
				t.uninstallCell(cell, row, col)
				return avoid
			}
			if cell.Tooltip != nil {
				t.borrowedTooltip = cell.Tooltip
				t.TooltipImmediate = cell.TooltipImmediate
				return t.RectToRoot(t.CellFrame(row, col)).Align()
			}
		}
	}
	t.borrowedTooltip = nil
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

// DefaultMouseExit provides the default mouse exit handling. The tooltip borrowed from the cell the pointer was over
// is given up here as well: the window asks for a tooltip only while the pointer is within the table, so nothing would
// otherwise take back what was borrowed once the pointer had left, and the table would go on holding the cell's
// tooltip panel alive until the pointer next passed over a cell.
func (t *Table[T]) DefaultMouseExit() bool {
	t.borrowedTooltip = nil
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
		// Every branch below acts on the selection, including the one that only notes the row for the mouse up to
		// settle, so the row the person is on is this one whichever way the press is modified.
		t.setLead(id)
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
		t.setLead(t.lastSel)
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
	if !tableNavigationModifiersRecognized(keyCode, mods) {
		// A navigation key carrying a modifier the table gives no meaning to isn't navigation, so it isn't acted on.
		// The usual way one arrives here is as a menu shortcut -- a command bound to cmd+shift+up, say -- that the
		// menu declined because the command was disabled at the time; treating it as a plain arrow would move the
		// selection when the user expected nothing to happen. It is reported as unhandled rather than swallowed,
		// leaving it to anything above the table that does have a use for it.
		return false
	}
	switch keyCode {
	case KeyLeft:
		// The keys that open and close containers are left alone while any filter is applied, rather than changing
		// open states beneath the filter without anything to show for it. A hierarchical filter shows every container
		// it kept as open, whatever the container's own open state, and a flat one shows nothing beneath any row at
		// all; the disclosure triangle a person would click is drawn without a hit rect under the one and not drawn at
		// all under the other. This is the same refusal axSetRowOpen makes for the equivalent request from an
		// assistive technology, and it is what ApplyFilter's promise that no modifications to the row data are
		// performed requires.
		if !repeat && !t.IsFiltered() && t.HasSelection() {
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
		// Refused while a filter is applied, for the reasons given for KeyLeft above.
		if !repeat && !t.IsFiltered() && t.HasSelection() {
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
		t.setLeadByIndex(i)
		t.ScrollRowCellIntoView(i, 0)
	case KeyDown:
		i := min(t.LastSelectedRowIndex()+1, len(t.rowCache)-1)
		if !mods.ShiftDown() {
			t.ClearSelection()
		}
		t.SelectByIndex(i)
		t.setLeadByIndex(i)
		t.ScrollRowCellIntoView(i, 0)
	case KeyHome:
		if mods.ShiftDown() && t.HasSelection() {
			t.SelectRange(0, t.FirstSelectedRowIndex())
		} else {
			t.ClearSelection()
			t.SelectByIndex(0)
		}
		// The row the keys arrived at is the first one, whether the selection was replaced or extended back to it:
		// SelectRange leaves the lead at the end of the range it was given, which for an extension is where the
		// selection already was.
		t.setLeadByIndex(0)
		t.ScrollRowCellIntoView(0, 0)
	case KeyEnd:
		if mods.ShiftDown() && t.HasSelection() {
			t.SelectRange(t.LastSelectedRowIndex(), len(t.rowCache)-1)
		} else {
			t.ClearSelection()
			t.SelectByIndex(len(t.rowCache) - 1)
		}
		t.setLeadByIndex(len(t.rowCache) - 1)
		t.ScrollRowCellIntoView(len(t.rowCache)-1, 0)
	default:
		return false
	}
	return true
}

// tableNavigationModifiersRecognized returns false if the key is one of the table's navigation keys and the modifiers
// include one the table attaches no meaning to for that key: only shift (extending the selection) means anything with
// the vertical keys, and only option (recursing into containers) with the horizontal ones. Any other key is not the
// table's concern here, so it is reported as recognized and left to the rest of the key handling.
func tableNavigationModifiersRecognized(keyCode KeyCode, mods mod.Modifiers) bool {
	var recognized mod.Modifiers
	switch keyCode {
	case KeyUp, KeyDown, KeyHome, KeyEnd:
		recognized = mod.Shift
	case KeyLeft, KeyRight:
		recognized = mod.Option
	default:
		return true
	}
	return mods&mod.NonSticky&^recognized == 0
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
	if i, focusables := collectFocusables(t.focusedCell, wnd.CurrentFocus(), nil); i != -1 {
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
	t.pruneLead()
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

// LeadRowIndex returns the index of the lead row — the row the person is on, which is the last one a gesture or a call
// moved the selection to. Will be -1 if there is no lead row or the table is no longer showing it. It is what the
// keyboard focus is reported on, so that a screen reader lands on the row the person moved to rather than on the table
// as a whole, while it is still one of the selected rows; see Table.axCurrentRow.
func (t *Table[T]) LeadRowIndex() int {
	if t.lead == "" {
		return -1
	}
	return t.axRowIndexForID(t.lead)
}

// setLead moves the row the person is on. A change to it that leaves the selection alone — an arrow key onto a row that
// was selected already — still has to reach an assistive technology, and a window is described again only once it has
// been drawn, so the redraw is what republishes it.
func (t *Table[T]) setLead(id tid.TID) {
	if t.lead == id {
		return
	}
	t.lead = id
	t.MarkForRedraw()
}

// setLeadByIndex moves the row the person is on to the row at the given index, and does nothing when the index names no
// row the table is showing.
func (t *Table[T]) setLeadByIndex(index int) {
	if index >= 0 && index < len(t.rowCache) {
		t.setLead(t.rowCache[index].row.ID())
	}
}

// pruneLead lets go of a lead row the table is no longer showing or no longer holds in the selection. Keeping a stale
// one would do no harm — axCurrentRow tests both before the focus is reported on it — but the row cache has just been
// rebuilt or walked here, so this is where it costs nothing.
func (t *Table[T]) pruneLead() {
	if t.lead != "" && (!t.selMap[t.lead] || t.axRowIndexForID(t.lead) < 0) {
		t.setLead("")
	}
}

// axCurrentRow returns the index of the row the person is on, which is the row the keyboard focus is reported on: the
// lead row, while the table is still showing it and it is still selected, and otherwise the first selected row. The
// result is -1 when nothing is selected, since the selection is what a table is "on" — an assistive technology reads
// the rows it holds — and there is then nowhere within the table to report the focus.
func (t *Table[T]) axCurrentRow() int {
	if t.lead != "" && t.selMap[t.lead] {
		if row := t.axRowIndexForID(t.lead); row >= 0 {
			return row
		}
	}
	return t.FirstSelectedRowIndex()
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
	t.setLead("")
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
	// The row the person is on is left where it was, since selecting everything does not move anyone: what it changes
	// is what else is selected around them. A lead that named no row the table is showing lands on the first row, which
	// is where a table with everything selected and nowhere in particular to be reads from.
	if !t.selMap[t.lead] {
		var first tid.TID
		if len(t.rowCache) != 0 {
			first = t.rowCache[0].row.ID()
		}
		t.setLead(first)
	}
	t.MarkForRedraw()
	t.notifyOfSelectionChange()
}

// SelectByIndex selects the given indexes. The first one will be considered the anchor selection if no existing anchor
// selection exists.
func (t *Table[T]) SelectByIndex(indexes ...int) {
	for _, index := range indexes {
		if index < 0 || index >= len(t.rowCache) {
			continue
		}
		id := t.rowCache[index].row.ID()
		t.selMap[id] = true
		t.selNeedsPrune = true
		if t.selAnchor == "" {
			t.selAnchor = id
		}
		// The last row actually selected is the one the person has arrived on, which is what the row the keyboard
		// focus is reported on is worked out from.
		t.setLead(id)
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
	// The end of the range is where a selection made by dragging or by shift-arrowing has arrived, so that is the row
	// the person is on.
	t.setLeadByIndex(end)
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
// modification was made. A hierarchical filter already shows every container it kept as open, so nothing is done while
// one is applied.
func (t *Table[T]) DiscloseRow(row T, delaySync bool) bool {
	if t.hierarchicalFilter {
		return false
	}
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

// RootRowCount returns the number of top-level rows. While a filter is applied, this is the number of rows the filter
// shows at the top level: every row that passed, for ApplyFilter, or the root rows kept, for ApplyHierarchicalFilter.
func (t *Table[T]) RootRowCount() int {
	switch {
	case t.hierarchicalFilter:
		return len(t.filterRoots)
	case t.filteredRows != nil:
		return len(t.filteredRows)
	default:
		return t.Model.RootRowCount()
	}
}

// RootRows returns the top-level rows. Do not alter the returned list. While a filter is applied, these are the rows
// the filter shows at the top level: every row that passed, for ApplyFilter, or the root rows kept, for
// ApplyHierarchicalFilter.
func (t *Table[T]) RootRows() []T {
	switch {
	case t.hierarchicalFilter:
		return t.filterRoots
	case t.filteredRows != nil:
		return t.filteredRows
	default:
		return t.Model.RootRows()
	}
}

// SetRootRows sets the top-level rows this table will display. This will call SyncToModel() automatically.
func (t *Table[T]) SetRootRows(rows []T) {
	t.clearFilter()
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
	// A filter applied by ApplyFilter presents the rows that passed as a flat list: disclosedChildren shows nothing
	// beneath any of them, whatever they are, so the table has no hierarchy while one is applied, however many
	// containers passed the filter. Saying otherwise would draw a disclosure triangle, with a hit rect, that opens onto
	// nothing and would describe such a row to an assistive technology as expandable, expanded and openable when there
	// is nothing there to open.
	t.hasHierarchy = false
	if !t.flatFilter() {
		for _, row := range roots {
			if row.CanHaveChildren() {
				t.hasHierarchy = true
				break
			}
		}
	}
	for _, row := range roots {
		rowCount += t.countDisclosedRowsRecursively(row)
	}
	t.rowCache = make([]tableCache[T], rowCount)
	j := 0
	for _, row := range roots {
		j = t.buildRowCacheEntry(row, -1, j, 0)
	}
	t.selNeedsPrune = true
	// The row cache was just thrown away and rebuilt, so the row the person was on may no longer be among the rows the
	// table shows — a row that was filtered out, or whose container was closed.
	t.pruneLead()
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

// countDisclosedRowsRecursively returns the number of rows the table shows for the row: the row itself plus every row
// disclosed beneath it.
func (t *Table[T]) countDisclosedRowsRecursively(row T) int {
	count := 1
	for _, child := range t.disclosedChildren(row) {
		count += t.countDisclosedRowsRecursively(child)
	}
	return count
}

// disclosedChildren returns the children the table shows beneath the row. A filter applied by ApplyFilter shows none,
// since it presents the rows that passed as a flat list. One applied by ApplyHierarchicalFilter shows the children it
// kept, whether or not the row is open. Otherwise, the row's children are shown when it is open.
func (t *Table[T]) disclosedChildren(row T) []T {
	if !row.CanHaveChildren() {
		return nil
	}
	switch {
	case t.hierarchicalFilter:
		return t.filterChildren[row.ID()]
	case t.filteredRows != nil:
		return nil
	case row.IsOpen():
		return row.Children()
	default:
		return nil
	}
}

// isRowDisclosed returns true if the table shows the row's children, which a hierarchical filter always does for the
// containers it keeps.
func (t *Table[T]) isRowDisclosed(row T) bool {
	return t.hierarchicalFilter || row.IsOpen()
}

func (t *Table[T]) buildRowCacheEntry(row T, parentIndex, index, depth int) int {
	t.rowCache[index].row = row
	t.rowCache[index].parent = parentIndex
	t.rowCache[index].depth = depth
	t.rowCache[index].height = t.heightForColumns(row, index, depth)
	parentIndex = index
	index++
	for _, child := range t.disclosedChildren(row) {
		index = t.buildRowCacheEntry(child, parentIndex, index, depth+1)
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

// IsFiltered returns true if a filter is currently applied, whether by ApplyFilter or by ApplyHierarchicalFilter. While
// a filter is applied, no modifications to the row data should be performed.
func (t *Table[T]) IsFiltered() bool {
	return t.filteredRows != nil
}

// flatFilter reports whether the filter in force is one that shows the rows that passed as a flat list, which is what
// ApplyFilter applies and ApplyHierarchicalFilter does not. Nothing is shown beneath any row while such a filter is
// applied, so the table has no hierarchy to draw, to describe or to change.
func (t *Table[T]) flatFilter() bool {
	return t.filteredRows != nil && !t.hierarchicalFilter
}

// ApplyFilter applies a filter to the data. When a non-nil filter is applied, all rows (recursively) are passed through
// the filter. Only those that the filter returns false for will be visible in the table. When a filter is applied, no
// hierarchy is display and no modifications to the row data should be performed. See ApplyHierarchicalFilter for a
// filter that keeps the hierarchy. A nil filter removes the filter, whichever of the two applied it.
func (t *Table[T]) ApplyFilter(filter func(row T) bool) {
	if filter == nil {
		if t.filteredRows == nil {
			return
		}
		t.clearFilter()
	} else {
		t.collectFilteredRows(filter)
		t.hierarchicalFilter = false
		t.filterMatches = nil
		t.filterChildren = nil
		t.filterRoots = nil
	}
	t.finishFilterChange()
}

// ApplyHierarchicalFilter applies a filter to the data the way ApplyFilter does, but keeps the hierarchy: each row the
// filter keeps is shown beneath the rows above it, and every container shown is shown open, whatever its own open
// state, so that the rows that passed are always in view. A row shown only because a row beneath it passed is context
// rather than a match, which IsFilterContextRow reports so that such a row can be drawn differently. The disclosure
// triangles, the keys that open and close containers and DiscloseRow are all left alone while the filter is applied,
// since the open states they would change have nothing to show for it. A nil filter removes the filter, as it does for
// ApplyFilter. As with ApplyFilter, no modifications to the row data should be performed while the filter is applied.
func (t *Table[T]) ApplyHierarchicalFilter(filter func(row T) bool) {
	if filter == nil {
		t.ApplyFilter(nil)
		return
	}
	t.collectFilteredRows(filter)
	t.hierarchicalFilter = true
	t.rebuildFilterHierarchy()
	t.finishFilterChange()
}

// IsFilterContextRow returns true if the row is one a hierarchical filter shows only as context: the row did not pass
// the filter itself, but a row beneath it did. It is false for every row while no hierarchical filter is applied.
func (t *Table[T]) IsFilterContextRow(row T) bool {
	return t.hierarchicalFilter && !t.filterMatches[row.ID()]
}

// collectFilteredRows puts every row of the model through the filter and records those that passed, in the model's
// order.
func (t *Table[T]) collectFilteredRows(filter func(row T) bool) {
	t.filteredRows = make([]T, 0)
	for _, row := range t.Model.RootRows() {
		t.applyFilter(row, filter)
	}
}

// clearFilter removes whatever filter is applied.
func (t *Table[T]) clearFilter() {
	t.filteredRows = nil
	t.hierarchicalFilter = false
	t.filterMatches = nil
	t.filterChildren = nil
	t.filterRoots = nil
}

// finishFilterChange brings the table in line with a change to the filter, keeping the rows sorted when the header has
// a sort in force.
func (t *Table[T]) finishFilterChange() {
	t.SyncToModel()
	if t.header != nil && t.header.HasSort() {
		t.header.ApplySort()
	}
}

// rebuildFilterHierarchy derives the rows a hierarchical filter shows from those that passed it. The model is walked so
// that the rows come out in its order, keeping a row when it passed or when any row beneath it did.
func (t *Table[T]) rebuildFilterHierarchy() {
	t.filterMatches = make(map[tid.TID]bool, len(t.filteredRows))
	for _, row := range t.filteredRows {
		t.filterMatches[row.ID()] = true
	}
	t.filterChildren = make(map[tid.TID][]T)
	t.filterRoots = t.keptFilterRows(t.Model.RootRows())
}

// keptFilterRows returns the rows of the list that a hierarchical filter keeps, recording the children kept beneath
// each of them along the way.
func (t *Table[T]) keptFilterRows(rows []T) []T {
	kept := make([]T, 0, len(rows))
	for _, row := range rows {
		var children []T
		if row.CanHaveChildren() {
			children = t.keptFilterRows(row.Children())
		}
		if t.filterMatches[row.ID()] || len(children) > 0 {
			kept = append(kept, row)
			if len(children) > 0 {
				t.filterChildren[row.ID()] = children
			}
		}
	}
	return kept
}

// removeRowsFromFilter takes the rows and their descendants out of the filtered view, for when they have been moved out
// of the model while a filter is applied.
func (t *Table[T]) removeRowsFromFilter(rows []T) {
	if t.filteredRows == nil {
		return
	}
	t.filteredRows = slices.DeleteFunc(slices.Clone(t.filteredRows), func(row T) bool {
		for _, r := range rows {
			if RowContainsRow(r, row) {
				return true
			}
		}
		return false
	})
	if t.hierarchicalFilter {
		t.rebuildFilterHierarchy()
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

// ProvideAccessibility describes the table to assistive technologies. Neither the rows nor the cells have panels of
// their own — a cell is borrowed from its row to draw it and handed back again — so each one is described directly,
// as a virtual child. Rows are keyed by their model id rather than by their index, so that an assistive technology's
// notion of a row survives the rows around it being sorted, filtered, inserted or removed.
//
// Only the rows that can be seen are described, plus the ones that are selected and the one holding the cell that has
// the keyboard focus. A table may hold far more rows than it shows; what an assistive technology asks about is what is
// on the screen, what the selection is, and where the focus is.
func (t *Table[T]) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		if t.hasHierarchy {
			node.Role = role.Tree
		} else {
			node.Role = role.Table
		}
	}
	node.Multiselectable = true
	node.RowCount = len(t.rowCache)
	node.ColumnCount = len(t.Columns)
	if t.header != nil {
		// A table and its header are separate panels — the header goes into the column-header slot of the scroll panel
		// whose content is the table — so the header is nowhere within the table's own subtree and nothing else would
		// say that the two belong together. All three platform adapters look here first and fall back to a proximity
		// search that gives up when an ancestor holds more than one table, so a window that puts two tables under one
		// common ancestor would otherwise report no column headers for either of them.
		node.Controls = append(node.Controls, b.IDFor(t.header))
	}
	// Pressing the table is not activating it; the default behavior would synthesize a click at the center of the
	// table's whole frame, which for a table in a scroll panel lands on a row somewhere near the middle of the model,
	// far from anything that can be seen, and replaces the selection with it.
	node.Actions = node.Actions.Without(accessibility.Press)
	if len(t.rowCache) == 0 {
		return
	}
	focusedRow := -1
	if t.focusedCell != nil {
		focusedRow = t.focusedCellRowIndex
	}
	// The row the keyboard focus is reported on, which is where the person is. While a cell holds the focus there is
	// nothing to report on a row: the panel inside that cell really does hold it and is described as holding it by the
	// ordinary path, so the table asks for no delegation at all.
	currentRow := -1
	if t.focusedCell == nil {
		currentRow = t.axCurrentRow()
	}
	var currentID accessibility.NodeID
	reach := axReach(b.VisibleRect())
	// The rows are walked in order, accumulating the y coordinate as their heights go by, since asking each row where
	// it is would walk the rows before it all over again. Nothing is described until the walk reaches the rows worth
	// describing, and it stops as soon as none can be left, so what a description costs is bounded by what is on the
	// screen and what is selected rather than by the size of the model — but the rows above the view port are still
	// stepped over one at a time, since neither this nor the draw path keeps the running heights that would let the
	// first visible row be found by search, and a table scrolled to the bottom of a million rows pays for that walk on
	// every description. Stepping over a row costs an addition, a comparison and, while anything is selected, a map
	// lookup.
	rect := t.ContentRect(false)
	described := 0
	// An upper bound on how many selected rows are still to come. It counts any rows the selection holds that are no
	// longer showing as well, which costs the walk nothing but its early exit.
	selectedLeft := len(t.selMap)
	// The top of the most recent row seen at each depth, which is where the ancestors of a row described out of
	// sequence sit: a row's ancestors are always the last rows seen above it at each shallower depth. A table whose
	// rows cannot have children has no ancestors to describe and does not pay for any of this.
	var depthTops []float32
	// The last row described, so that a row arriving without the one before it can bring its ancestors with it.
	last := -1
	for row := range t.rowCache {
		entry := &t.rowCache[row]
		rect.Height = entry.height
		if t.hasHierarchy {
			for len(depthTops) <= entry.depth {
				depthTops = append(depthTops, rect.Y)
			}
			depthTops[entry.depth] = rect.Y
		}
		selected := selectedLeft > 0 && t.IsRowSelected(row)
		if selected {
			selectedLeft--
		}
		outOfSequence := t.hasHierarchy && last != row-1
		switch {
		case rect.Intersects(reach), row == focusedRow:
			if outOfSequence {
				t.axAddRowAncestors(b, row, depthTops)
			}
			id := t.axAddRow(b, row, rect, false)
			if row == currentRow {
				currentID = id
			}
			last = row
		case row == currentRow:
			// The row the focus is to be reported on is described however far out of sight it is and whatever the cap
			// on selected rows has reached: it is where the person is. It is not counted against that cap, so the rest
			// of the selection is described exactly as much as it would have been without it.
			if outOfSequence {
				t.axAddRowAncestors(b, row, depthTops)
			}
			currentID = t.axAddRow(b, row, rect, true)
			last = row
		case selected && described < axMaxSelectedRows:
			if outOfSequence {
				t.axAddRowAncestors(b, row, depthTops)
			}
			t.axAddRow(b, row, rect, true)
			last = row
			described++
		}
		rect.Y += rect.Height
		if t.ShowRowDivider {
			rect.Y++
		}
		if rect.Y > reach.Bottom() && row >= focusedRow && row >= currentRow &&
			(selectedLeft <= 0 || described >= axMaxSelectedRows) {
			// Every row from here down starts below the reach, so none of them can be seen; the focused row and the
			// row the person is on have gone by; and either the selection holds nothing further or as much of it as
			// will be described has been.
			break
		}
	}
	if currentID != 0 {
		// The keyboard focus the table holds is reported on the row the person is on, which is where every native
		// table puts it and the only thing a screen reader follows: a container that keeps the focus to itself and says
		// nothing but "the selection changed" leaves the reader where it was. The table keeps the focus when there is
		// no current row, as a native table with nothing selected does. See AccessibilityBuilder.FocusChild, which
		// refuses the delegation unless the table really is the panel holding the focus.
		b.FocusChild(currentID)
	}
}

// axAddRowAncestors describes the rows a row hangs beneath, for a row that is being described although the row before
// it was not.
//
// An assistive technology reads the nesting of a table's rows from their levels and the order they arrive in: the rows
// disclosed by a container are the ones after it with a deeper level, up to the next row at the container's own level.
// So a row that turned up without its container — one described only because it is selected, or the first row of what
// can be seen — would be read as disclosed by whatever container came before it instead, which is the wrong row
// whenever the two belong to different parts of the model. Bringing its ancestors along keeps the flat list honest;
// they are described as the rows they are, with nothing in them, exactly as any other row that cannot be seen is.
//
// depthTops holds the top of the most recent row seen at each depth, which is where each of the ancestors sits.
func (t *Table[T]) axAddRowAncestors(b *AccessibilityBuilder, row int, depthTops []float32) {
	var chain []int
	for parent := t.rowCache[row].parent; parent >= 0 && parent < len(t.rowCache); parent = t.rowCache[parent].parent {
		chain = append(chain, parent)
	}
	rect := t.ContentRect(false)
	for i := len(chain) - 1; i >= 0; i-- {
		ancestor := chain[i]
		if depth := t.rowCache[ancestor].depth; depth < len(depthTops) {
			rect.Y = depthTops[depth]
		}
		rect.Height = t.rowCache[ancestor].height
		// An ancestor that has already been described is left where it is: adding it again would list it among the
		// table's children twice, which is what every position counted out of that list would then be wrong about.
		t.axAddRow(b, ancestor, rect, true)
	}
}

// columnLeft returns the x coordinate at which the given column begins, in the table's own coordinates.
func (t *Table[T]) columnLeft(col int) float32 {
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
	return x
}

// disclosureFrameAtY returns the frame of the disclosure triangle drawn for the row whose top is at y, in the table's
// own coordinates, or an empty rect when the row is drawn without one. It is the arithmetic DefaultDraw places the
// triangle with, so that what an assistive technology is told can be pressed is what a person can click.
func (t *Table[T]) disclosureFrameAtY(row int, y float32) geom.Rect {
	hierarchyIndent := t.CurrentHierarchyIndent()
	if hierarchyIndent <= 0 || t.hierarchicalFilter || !t.rowCache[row].row.CanHaveChildren() {
		return geom.Rect{}
	}
	col := -1
	for c := range t.Columns {
		if t.Columns[c].ID == t.HierarchyColumnID {
			col = c
			break
		}
	}
	if col < 0 {
		return geom.Rect{}
	}
	const disclosureIndent = 2
	size := min(hierarchyIndent, t.MinimumRowHeight) - disclosureIndent*2
	if size <= 0 {
		return geom.Rect{}
	}
	cellRect := geom.NewRect(t.columnLeft(col), y, t.Columns[col].Current, t.rowCache[row].height).Inset(t.Padding)
	return geom.NewRect(cellRect.X+hierarchyIndent*float32(t.rowCache[row].depth)+disclosureIndent,
		cellRect.Y+(t.MinimumRowHeight-size)/2, size, size)
}

// axDisclosureKey is the key under which the disclosure triangle of a row is described.
type axDisclosureKey struct {
	Row tid.TID
}

// axAddRow describes one row of the table, along with each of its cells. rect is the row's frame in the table's own
// coordinates.
//
// A row described with nameOnly is described as itself and nothing more: what it is, where it is, and whether it is
// selected, open or can be opened, without the disclosure triangle and the cells beneath it. That is what is left of a
// row nobody can see — one described because the selection holds it, or because a row further down hangs beneath it —
// and it is all an assistive technology asks of such a row, since what it does with the selection is read out what is
// in it. Building the rest would mean asking the model for a panel per column, laying each one out and walking it,
// over and over: several hundred panels on every description of a table where everything has just been selected.
//
// The node id of the row is returned, or zero when nothing was added — which is what a row that has already been
// described during this description answers with — and is what ProvideAccessibility reports the keyboard focus on for
// the current row.
func (t *Table[T]) axAddRow(b *AccessibilityBuilder, row int, rect geom.Rect, nameOnly bool) accessibility.NodeID {
	entry := t.rowCache[row]
	id := entry.row.ID()
	depth := entry.depth
	selected := t.IsRowSelected(row)
	// A container is only something that expands while the table is showing a hierarchy at all. Under a flat filter it
	// is not: nothing is shown beneath it, the table draws it without a disclosure triangle, and it is described as the
	// plain row it appears as. See SyncToModel.
	expandable := t.hasHierarchy && entry.row.CanHaveChildren()
	expanded := expandable && t.isRowDisclosed(entry.row)
	// A hierarchical filter shows every container it kept as open, whatever the container's own open state, so the row
	// is reported as expanded but is not offered for opening and closing, exactly as the disclosure triangle is drawn
	// open but given no hit rect and the keys that would change the open states are left alone.
	toggleable := expandable && !t.hierarchicalFilter
	var name string
	if len(t.Columns) != 0 {
		// The name comes from the first column, so a table with no columns has nothing to ask the row for. Asking
		// anyway would have the model produce the data of a column that does not exist.
		name = entry.row.CellDataForSort(0)
	}
	rowID := b.AddVirtualChild(id, func(n *accessibility.Node) {
		n.Role = role.Row
		n.Name = name
		n.Bounds = rect
		n.Level = depth + 1
		n.RowIndex = row
		n.Selectable = true
		n.Selected = selected
		n.Expandable = expandable
		n.Expanded = expanded
		// The rows are where the keyboard focus within a table is, as they are in a native one: the table reports the
		// focus it holds on the row the person is on, and an assistive technology asking for the focus to be put on
		// another row is asking to move to it, which selects it. See PerformAccessibilityAction.
		n.Focusable = true
		n.Actions = n.Actions.With(accessibility.Select, accessibility.AddToSelection,
			accessibility.RemoveFromSelection, accessibility.ScrollIntoView, accessibility.Focus)
		if toggleable {
			n.Actions = n.Actions.With(accessibility.Expand, accessibility.Collapse)
		}
		if t.DoubleClickCallback != nil {
			// Pressing a row is opening it, which is what a double-click and the Return key stand for, so it is offered
			// only when there is something for it to do.
			n.Actions = n.Actions.With(accessibility.Press)
		}
	})
	if rowID == 0 || nameOnly {
		return rowID
	}
	// The triangle that opens and closes a row is drawn by the table rather than by any cell, so it is described here
	// as the row's first child: something a screen reader can land on and press, as a person can click it.
	if frame := t.disclosureFrameAtY(row, rect.Y); !frame.Empty() {
		b.AddVirtualChildOf(rowID, axDisclosureKey{Row: id}, func(n *accessibility.Node) {
			n.Role = role.DisclosureTriangle
			n.Name = i18n.Text("Disclosure Triangle")
			n.Bounds = frame
			n.RowIndex = row
			n.Pressed = expanded
			n.Expandable = true
			n.Expanded = expanded
			n.Actions = n.Actions.With(accessibility.Press, accessibility.Toggle, accessibility.Expand,
				accessibility.Collapse, accessibility.ScrollIntoView)
		})
	}
	for col := range t.Columns {
		frame := t.cellFrameAtY(row, col, rect.Y)
		text := entry.row.CellDataForSort(col)
		key := accessibility.CellKey{Row: id, Col: col}
		cellID := b.AddVirtualChildOf(rowID, key, func(n *accessibility.Node) {
			n.Role = role.Cell
			n.Name = text
			n.Bounds = frame
			n.RowIndex = row
			n.ColumnIndex = col
			n.Actions = n.Actions.With(accessibility.ScrollIntoView)
		})
		if cellID == 0 {
			continue
		}
		// Whatever the row hands back for the cell — a check box, a button, a field, a wrapper full of labels — is
		// described within it, attached and laid out for the moment exactly as it is for drawing. A cell with content
		// of its own to describe has no need of the sort text as a name on top of that; a cell whose content amounts to
		// nothing keeps it.
		cell := t.cell(row, col)
		t.installCell(cell, frame)
		b.addCellPanel(cellID, key, cell, cell == t.focusedCell)
		t.uninstallCell(cell, row, col)
		if content := b.snapshot.tree.UnignoredChildren(cellID); len(content) != 0 {
			cellNode := b.snapshot.tree.Nodes[cellID]
			cellNode.Name = ""
			// A screen reader moving through a table treats the cell as the unit: it presses the cell, and it reports a
			// change by reading the cell's value again. So a cell whose content can be pressed offers the press itself
			// and passes it on (see axActOnCellContent), and a cell holding a single widget with a state reports that
			// state as its own value, so that a change to it is a change to the cell.
			for _, action := range []accessibility.Action{accessibility.Press, accessibility.Toggle} {
				if axDescendantOffers(b.snapshot.tree, cellID, action) {
					cellNode.Actions = cellNode.Actions.With(action)
				}
			}
			if len(content) == 1 {
				cellNode.Value = axContentValue(b.snapshot.tree.Node(content[0]))
			}
		}
	}
	return rowID
}

// axContentValue returns the value a cell reports for the one widget it holds: the state of a check box or radio
// button in words, otherwise the widget's own value, textual or numeric.
func axContentValue(n *accessibility.Node) string {
	switch {
	case n == nil:
		return ""
	case n.HasCheck:
		switch n.Checked {
		case check.On:
			return i18n.Text("Checked")
		case check.Mixed:
			return i18n.Text("Mixed")
		default:
			return i18n.Text("Unchecked")
		}
	case n.Value != "":
		return n.Value
	case n.HasNumber:
		return strconv.FormatFloat(n.Number, 'g', -1, 64)
	default:
		return ""
	}
}

// axDescendantOffers reports whether any node beneath the given one advertises the action.
func axDescendantOffers(tree *accessibility.Tree, id accessibility.NodeID, action accessibility.Action) bool {
	for _, child := range tree.UnignoredChildren(id) {
		if n := tree.Node(child); n != nil && (n.Actions.Has(action) || axDescendantOffers(tree, child, action)) {
			return true
		}
	}
	return false
}

// PerformAccessibilityAction carries out a request from an assistive technology. Every request that reaches here names
// one of the rows or cells described by ProvideAccessibility, which arrives as the key that row or cell was described
// under; acting on a cell acts on the row it belongs to, apart from scrolling, which brings the cell itself into view.
// Selecting a row also scrolls it into view, as the arrow keys do, since an assistive technology moving through the
// rows selects each one as it goes and expects to see where it has got to. Pressing a row opens it, which is the
// gesture a double-click and the Return key stand for. Putting the focus on a row moves the person onto it: the table
// takes the keyboard focus and the row becomes the whole of the selection, exactly as an arrow key onto that row would
// leave things, since the selection is where a table's cursor is. Only a row offers the focus; a cell does not, since
// the table is the one tab stop and a cell is not a place the keyboard can be left.
func (t *Table[T]) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	var id tid.TID
	col := -1
	switch key := req.Key.(type) {
	case tid.TID:
		id = key
	case accessibility.CellKey:
		id = key.Row
		col = key.Col
	case axCellPanelKey:
		return t.axPerformInCell(key, req)
	case axDisclosureKey:
		return t.axActOnDisclosure(key, req)
	default:
		return false
	}
	row := t.axRowIndexForID(id)
	if row < 0 {
		return false
	}
	if col >= 0 && (req.Action == accessibility.Press || req.Action == accessibility.Toggle) {
		return t.axActOnCellContent(row, col, req)
	}
	switch req.Action {
	case accessibility.Press:
		if t.DoubleClickCallback == nil {
			return false
		}
		// Both of the gestures this stands for act on the selection: a double-click has already selected the row with
		// its first click, and the Return key runs against whatever is selected. So the row is selected first when it
		// was not already, and the callback then finds what it expects.
		if !t.IsRowSelected(row) {
			t.axSelectOnly(id)
		}
		SafeCall(t.DoubleClickCallback)
	case accessibility.Select:
		t.axSelectOnly(id)
		t.ScrollRowIntoView(row)
	case accessibility.AddToSelection:
		if !t.IsRowSelected(row) {
			// A row that is already in the selection has nothing to be added to it. SelectByIndex would tell the
			// application its selection had changed regardless, and an assistive technology acts on the row it is
			// already on often enough — landing on it, then acting on it — that an application would set about whatever
			// it does on a change to the selection over and over. The mouse path counts the same way.
			t.SelectByIndex(row)
		}
		t.ScrollRowIntoView(row)
	case accessibility.RemoveFromSelection:
		if t.IsRowSelected(row) {
			// As above: a row that is not selected has nothing to be taken out of the selection, and the row is left
			// exactly as it was asked to be.
			t.DeselectByIndex(row)
		}
	case accessibility.Focus:
		if col >= 0 {
			// A cell is not somewhere the focus can be put: a cell that holds a widget the keyboard can work in
			// publishes that widget as the real panel it is, and the focus goes there through the ordinary path. Cells
			// are not described as offering this, but this method is exported and the dispatcher does not check that an
			// action was advertised, so one that arrives anyway is refused here.
			return false
		}
		// Replacing the selection with the row, rather than adding to it, is what the arrow keys do, and this stands
		// for the same movement; SelectByIndex is also what puts the row the focus is reported on onto this one.
		t.ClearSelection()
		t.SelectByIndex(row)
		t.ScrollRowCellIntoView(row, 0)
		t.RequestFocusWithoutScroll()
		if wnd := t.Window(); wnd == nil || !t.Is(wnd.CurrentFocus()) {
			// A focus request that did not leave the focus where it said it would must be reported as refused rather
			// than as carried out, exactly as axDispatchAction checks for a real panel: the window may decline it, and
			// a table with no window cannot take it at all.
			return false
		}
	case accessibility.Expand:
		return t.axSetRowOpen(row, true)
	case accessibility.Collapse:
		return t.axSetRowOpen(row, false)
	case accessibility.ScrollIntoView:
		if col >= 0 && col < len(t.Columns) {
			t.ScrollRowCellIntoView(row, col)
		} else {
			t.ScrollRowIntoView(row)
		}
	default:
		return false
	}
	return true
}

// axSelectOnly makes the row with the given id the whole of the selection, and the anchor a later shift-click extends
// from. A plain click does both — setting the selection alone leaves the anchor wherever it last was, on a row that may
// not even be selected any more — and a person who has just moved the selection with an assistive technology and then
// shift-clicks expects the same thing to happen as if they had clicked the row themselves.
//
// The application is told the selection changed only when it did, exactly as DefaultMouseDown says nothing for a click
// on the row that is already the whole of the selection. An assistive technology re-selects the row it is already on
// constantly, and an application told its selection had changed does whatever it does when that happens. The selection
// is made either way, since it also puts the anchor on the row.
func (t *Table[T]) axSelectOnly(id tid.TID) {
	changed := t.SelectionCount() != 1 || !t.selMap[id]
	// Nothing needs pruning: the caller found the row in the row cache, so the one id the selection now holds is known
	// to be in it, exactly as DefaultMouseDown's equivalent branch leaves the flag alone. Setting it would make the
	// next HasSelection, SelectionCount or SelectedRows walk the whole row cache and rebuild the map — once for every
	// row an assistive technology steps onto.
	t.selMap = map[tid.TID]bool{id: true}
	t.selAnchor = id
	t.MarkForRedraw()
	if changed {
		t.notifyOfSelectionChange()
	}
}

// axPerformInCell carries out a request aimed at a panel inside one of the table's cells. The cell is built and
// attached again, exactly as it is to hand it a mouse event, so that the panel the request is for exists and can reach
// its window; the panel is then found at the position within the cell it was described at. A widget that takes the
// keyboard focus while handling the request stays attached as the focused cell, as one that took it from a click would.
//
// A key naming a cell of something other than a table — a list keys its rows by index — is refused: the key travels
// with the request from whatever described the panel, and only the widget that put it there knows how to read it.
func (t *Table[T]) axPerformInCell(key axCellPanelKey, req accessibility.ActionRequest) bool {
	cellKey, ok := key.Cell.(accessibility.CellKey)
	if !ok {
		return false
	}
	row := t.axRowIndexForID(cellKey.Row)
	col := cellKey.Col
	if row < 0 || col < 0 || col >= len(t.Columns) {
		return false
	}
	// The key that brought the request here named one of the table's virtual children. What it is being handed to is a
	// real panel, for which ActionRequest.Key is nil: an application's own action callback on a panel within a cell
	// would otherwise be given a key it never handed out, and a widget that keys virtual children of its own — a table
	// header nested in a cell — would take the table's key for one of its own and act at whatever position it matched.
	req.Key = nil
	cell := t.cell(row, col)
	t.installCell(cell, t.CellFrame(row, col))
	handled := false
	if target := axPanelAtPath(cell, key.Path); target != nil {
		handled = target.axDispatchAction(req, false)
	}
	t.uninstallCell(cell, row, col)
	t.MarkForRedraw()
	return handled
}

// axActOnDisclosure carries out a request aimed at a row's disclosure triangle: pressing or toggling it turns the row
// the other way, and expanding or collapsing it turns the row that way.
func (t *Table[T]) axActOnDisclosure(key axDisclosureKey, req accessibility.ActionRequest) bool {
	row := t.axRowIndexForID(key.Row)
	if row < 0 {
		return false
	}
	switch req.Action {
	case accessibility.Press, accessibility.Toggle:
		return t.axSetRowOpen(row, !t.rowCache[row].row.IsOpen())
	case accessibility.Expand:
		return t.axSetRowOpen(row, true)
	case accessibility.Collapse:
		return t.axSetRowOpen(row, false)
	case accessibility.ScrollIntoView:
		t.ScrollRowIntoView(row)
		return true
	default:
		return false
	}
}

// axActOnCellContent passes a press or toggle aimed at a cell on to the content within the cell that can carry it out,
// with the cell built and attached as it is for a click. A screen reader moving through a table treats the cell as the
// unit and presses that, expecting the check box or button within it to respond.
//
// Each candidate is offered the request in turn until one of them reports having carried it out, because what a widget
// does with a request is only known from handing it over: a cell that holds a button and a check box advertises both a
// press and a toggle, since between them they offer both, yet only the check box has a state to move on, so a toggle
// handed to the button would be refused and the cell would have advertised something it then would not do.
//
// A candidate that was described as not offering the action is passed over rather than being handed it. The candidates
// are picked by the shape of the panel — one with an action callback or actor of its own, or one that handles both
// halves of a click — while what the cell advertises comes from what its content was actually described as offering,
// and the two part company for a widget that withdrew the action: a Field takes Press out of its own description, yet
// still looks pressable here because it handles both halves of a click. A press on a cell holding a field ahead of a
// button — advertised only because of the button — would otherwise be handed to the field, which does not act on a
// press but does not refuse one either, since the fallback synthesizes a click that merely drops the caret in it; the
// button would never see the press, and the field would be left as the table's focused cell, quietly starting an
// editing session nobody asked for.
func (t *Table[T]) axActOnCellContent(row, col int, req accessibility.ActionRequest) bool {
	if col >= len(t.Columns) {
		return false
	}
	cellKey := accessibility.CellKey{Row: t.rowCache[row].row.ID(), Col: col}
	action := req.Action
	// The request named the cell, whose key is the table's own; the content it is being passed to is a real panel, for
	// which ActionRequest.Key is nil. See axPerformInCell.
	req.Key = nil
	cell := t.cell(row, col)
	t.installCell(cell, t.CellFrame(row, col))
	handled := false
	for _, target := range t.axCellTargetsOffering(cell, cellKey, action) {
		if target.axDispatchAction(req, false) {
			handled = true
			break
		}
	}
	t.uninstallCell(cell, row, col)
	t.MarkForRedraw()
	return handled
}

// axCellTargetsOffering returns the panels within a cell that a press or a toggle may be handed to, in the order they
// should be tried: the ones whose description advertises the action first, then the ones that were not described at
// all. A panel that was described without the action is left out entirely, since the cell only advertises what its
// content was described as offering, and handing the request to something that was described as not offering it is how
// the cell comes to answer for a widget that never saw it.
//
// A panel with no description of its own is still tried, after the rest. The cell is built afresh for every request, so
// a row that builds something different from what it built when the window was last described has nothing to match
// against, and refusing everything then would be worse than the order the panels are in.
func (t *Table[T]) axCellTargetsOffering(cell *Panel, key accessibility.CellKey,
	action accessibility.Action,
) []*Panel {
	candidates := axPressableTargets(cell)
	offering := make([]*Panel, 0, len(candidates))
	undescribed := make([]*Panel, 0, len(candidates))
	for _, candidate := range candidates {
		path, ok := axPathFromRoot(cell, candidate)
		if !ok {
			undescribed = append(undescribed, candidate)
			continue
		}
		node := t.axDescribedCellNode(axCellPanelKey{Cell: key, Path: path})
		switch {
		case node == nil:
			undescribed = append(undescribed, candidate)
		case node.Actions.Has(action):
			offering = append(offering, candidate)
		}
	}
	return append(offering, undescribed...)
}

// axDescribedCellNode returns the node the table last described for the panel at a position within one of its cells, or
// nil if it did not describe one. The panels inside a cell exist only while the cell is built, so they are described
// under ids the table allocates for their position within it rather than under ids of their own; see axCellPanelKey.
func (t *Table[T]) axDescribedCellNode(key axCellPanelKey) *accessibility.Node {
	id := t.Accessibility.virtual[key].id
	if id == 0 {
		return nil
	}
	wnd := t.Window()
	if wnd == nil || wnd.ax == nil || wnd.ax.last == nil {
		return nil
	}
	return wnd.ax.last.Node(id)
}

// axPathFromRoot returns the position of p beneath root in the form axCellPanelKey.Path holds it: the child indexes on
// the way down from root, joined with dots. Reports false if p is not beneath root at all. This is the inverse of
// axPanelAtPath.
func axPathFromRoot(root, p *Panel) (string, bool) {
	var parts []string
	for one := p; one != root; one = one.parent {
		parent := one.parent
		if parent == nil {
			return "", false
		}
		index := slices.Index(parent.Children(), one)
		if index < 0 {
			return "", false
		}
		parts = append(parts, strconv.Itoa(index))
	}
	slices.Reverse(parts)
	return strings.Join(parts, "."), true
}

// axPressableTargets returns the panels at or beneath p, front to back, that might respond to a press or a toggle: ones
// with an accessibility action callback or actor of their own, and ones that handle both halves of a click.
//
// A hidden panel is left out along with everything beneath it, since nothing within something that is not drawn can be
// reached. A disabled panel is left out too, since every path refuses it — Window.mouseDown and Window.mouseUp pass
// over a panel that is not enabled, and so does axDispatchAction — but what is beneath it is not: being enabled is a
// panel's own property rather than something it passes down, and a click landing on an enabled widget nested inside a
// disabled wrapper is handed to that widget, so a request from an assistive technology must reach it too.
//
// What comes back is the panels that might respond, judged by their shape alone, which is not the same as the panels
// the cell was described as offering the action: a widget that withdrew the action from its own description still
// looks like one that would take it here. axCellTargetsOffering is what reconciles the two before anything is handed
// over.
func axPressableTargets(p *Panel) []*Panel {
	var targets []*Panel
	axAppendPressableTargets(&targets, p)
	return targets
}

// axAppendPressableTargets appends the panels at or beneath p that might respond to a press or a toggle to targets.
// The children come before the panel itself, and in index order, which is front to back: index 0 is drawn last and so
// sits on top, and a press must land where a click would rather than on whatever was drawn underneath.
func axAppendPressableTargets(targets *[]*Panel, p *Panel) {
	if p.Hidden {
		return
	}
	for _, child := range p.Children() {
		axAppendPressableTargets(targets, child)
	}
	if !p.Enabled() {
		return
	}
	if p.Accessibility.ActionCallback != nil || (p.MouseDownCallback != nil && p.MouseUpCallback != nil) {
		*targets = append(*targets, p)
	} else if _, ok := p.Self.(AccessibilityActor); ok {
		*targets = append(*targets, p)
	}
}

// axRowIndexForID returns the index of the row with the given id among the rows the table is currently showing, or -1
// if it is not showing one. A request from an assistive technology names a row by its id, since that is what the row
// was described under, and the rows may have been sorted or filtered since.
func (t *Table[T]) axRowIndexForID(id tid.TID) int {
	for i := range t.rowCache {
		if t.rowCache[i].row.ID() == id {
			return i
		}
	}
	return -1
}

// axSetRowOpen opens or closes a row on behalf of an assistive technology, bringing the table up to date with the
// change exactly as clicking the row's disclosure triangle would. Reports false if the row cannot have children at all,
// or if a filter is applied: a hierarchical filter shows every container it kept as open, whatever the container's own
// open state, and a flat one shows nothing beneath any row at all, so neither has anything to show for a change to an
// open state. The disclosure triangle a person would click is not there under either — drawn without a hit rect under a
// hierarchical filter, not drawn at all under a flat one — so an assistive technology let through would be told a row
// had opened or closed while everything it can observe stayed exactly as it was.
func (t *Table[T]) axSetRowOpen(row int, open bool) bool {
	if t.IsFiltered() {
		return false
	}
	data := t.rowCache[row].row
	if !data.CanHaveChildren() {
		return false
	}
	if data.IsOpen() == open {
		// The row is already the way it was asked to be, which is as good an outcome as having changed it.
		return true
	}
	data.SetOpen(open)
	t.SyncToModel()
	if !open {
		t.PruneSelectionOfUndisclosedNodes()
	}
	return true
}
