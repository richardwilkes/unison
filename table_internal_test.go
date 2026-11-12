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
	"slices"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
)

// panicCellRow is a minimal TableRowData whose ColumnCell panics while its shared 'boom' flag is set, standing in for
// user cell code that fails partway through a table sync.
type panicCellRow struct {
	parent    *panicCellRow
	boom      *bool
	cellCalls *int
	id        tid.TID
}

func (r *panicCellRow) CloneForTarget(_ Paneler, newParent *panicCellRow) *panicCellRow {
	return &panicCellRow{id: r.id, parent: newParent, boom: r.boom, cellCalls: r.cellCalls}
}
func (r *panicCellRow) ID() tid.TID                    { return r.id }
func (r *panicCellRow) Parent() *panicCellRow          { return r.parent }
func (r *panicCellRow) SetParent(parent *panicCellRow) { r.parent = parent }
func (r *panicCellRow) CanHaveChildren() bool          { return false }
func (r *panicCellRow) Children() []*panicCellRow      { return nil }
func (r *panicCellRow) SetChildren(_ []*panicCellRow)  {}
func (r *panicCellRow) CellDataForSort(_ int) string   { return string(r.id) }
func (r *panicCellRow) ColumnCell(_, _ int, _, _ Ink, _, _, _ bool) Paneler {
	*r.cellCalls++
	if *r.boom {
		panic("cell exploded")
	}
	return NewPanel()
}
func (r *panicCellRow) IsOpen() bool   { return false }
func (r *panicCellRow) SetOpen(_ bool) {}

// newPanicCellTable returns a single-column table backed by a row that panics from ColumnCell whenever *boom is true,
// along with a counter of the ColumnCell calls made. The table starts out synced with the row behaving.
func newPanicCellTable(boom *bool, cellCalls *int) *Table[*panicCellRow] {
	model := &SimpleTableModel[*panicCellRow]{}
	model.SetRootRows([]*panicCellRow{{id: "a", boom: boom, cellCalls: cellCalls}})
	table := NewTable[*panicCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}}
	table.SyncToModel()
	return table
}

// runTasksUntil runs queued tasks through the same recovery path the UI loop uses until done reports that the work the
// test is waiting on has happened. It drains the queue instead of assuming the next task to arrive is the one the test
// scheduled, since the global queue is shared. These tests share the global task queue and therefore must not call
// t.Parallel.
func runTasksUntil(t *testing.T, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the scheduled task to run")
		}
		if length, head := taskQueueState(); length > head {
			processNextTask()
		} else {
			time.Sleep(time.Millisecond)
		}
	}
}

// TestTableEventuallySyncToModelClearsFlagOnPanic verifies that a panicking sync doesn't wedge the table. The pending
// flag used to be cleared only after the work completed, so a panic in user cell code (recovered by processNextTask)
// left it set forever and every later EventuallySyncToModel was silently ignored.
func TestTableEventuallySyncToModelClearsFlagOnPanic(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	var recovered error
	withRecoveryCallback(t, func(err error) { recovered = err })

	boom := false
	cellCalls := 0
	table := newPanicCellTable(&boom, &cellCalls)

	boom = true
	table.EventuallySyncToModel()
	c.True(table.awaitingSyncToModel)
	runTasksUntil(t, func() bool { return !table.awaitingSyncToModel })
	c.NotNil(recovered, "the scheduled sync was expected to panic")
	c.False(table.awaitingSyncToModel, "a panicking sync must still clear the pending flag")

	// A later request must actually do the work rather than being dropped.
	boom = false
	recovered = nil
	before := cellCalls
	table.EventuallySyncToModel()
	runTasksUntil(t, func() bool { return !table.awaitingSyncToModel })
	c.Nil(recovered)
	c.True(cellCalls > before, "the table must sync again after a panicking sync")
}

// TestTableEventuallySizeColumnsToFitClearsFlagOnPanic verifies the same for the column sizing counterpart.
func TestTableEventuallySizeColumnsToFitClearsFlagOnPanic(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	var recovered error
	withRecoveryCallback(t, func(err error) { recovered = err })

	boom := false
	cellCalls := 0
	table := newPanicCellTable(&boom, &cellCalls)

	boom = true
	table.EventuallySizeColumnsToFit(false)
	c.True(table.awaitingSizeColumnsToFit)
	runTasksUntil(t, func() bool { return !table.awaitingSizeColumnsToFit })
	c.NotNil(recovered, "the scheduled sizing was expected to panic")
	c.False(table.awaitingSizeColumnsToFit, "a panicking sizing pass must still clear the pending flag")

	boom = false
	recovered = nil
	before := cellCalls
	table.EventuallySizeColumnsToFit(false)
	runTasksUntil(t, func() bool { return !table.awaitingSizeColumnsToFit })
	c.Nil(recovered)
	c.True(cellCalls > before, "the table must size its columns again after a panicking sizing pass")
}

// TestColumnInfoResizable verifies the gate that decides whether the user may drag a column divider. A Maximum of 0 or
// less means "no maximum" to the resize clamping, so only a Minimum and Maximum that pin the column to a single width
// may block a resize.
func TestColumnInfoResizable(t *testing.T) {
	c := check.New(t)
	c.True((&ColumnInfo{}).resizable(), "an unconstrained column is resizable")
	c.True((&ColumnInfo{Minimum: 50}).resizable(), "a column with only a minimum is resizable")
	c.True((&ColumnInfo{Maximum: 50}).resizable(), "a column with only a maximum is resizable")
	c.True((&ColumnInfo{Minimum: 50, Maximum: 100}).resizable(), "a column with room between its bounds is resizable")
	c.False((&ColumnInfo{Minimum: 50, Maximum: 50}).resizable(), "a column pinned to one width is not resizable")
	c.False((&ColumnInfo{Minimum: 100, Maximum: 50}).resizable(), "an inverted range is not resizable")
}

// focusCellColumnCount is how many columns the tables built by newFocusCellFixture() have. The rows memoize one panel
// per column, so they have to know how much room to make for them.
const focusCellColumnCount = 3

// focusCellRow is a TableRowData whose cells are memoized panels, which is what a row has to do for a cell that holds
// state -- the keyboard focus, in this case -- to survive the table asking for its cells over and over again. The first
// 'focusableCols' columns stand in for an editable widget such as a Field: they are focusable and take the keyboard
// focus when they are pressed. The remaining columns are plain panels that can never be focused. The per-column gained
// and lost counters, along with the recorded value of ColumnCell's 'focused' argument, are how the tests see what the
// table did to the cells without having to reach into the table itself.
type focusCellRow struct {
	parent         *focusCellRow
	id             tid.TID
	cells          []*Panel
	gained         []int
	lost           []int
	children       []*focusCellRow
	focusableCols  int
	lastFocusedArg bool
	open           bool
	// wrap makes ColumnCell hand back a fresh panel around the memoized one on every call.
	wrap bool
}

func newFocusCellRow(id string, focusableCols int) *focusCellRow {
	return &focusCellRow{
		cells:         make([]*Panel, focusCellColumnCount),
		gained:        make([]int, focusCellColumnCount),
		lost:          make([]int, focusCellColumnCount),
		id:            tid.TID(id),
		focusableCols: focusableCols,
	}
}

func (r *focusCellRow) CloneForTarget(_ Paneler, newParent *focusCellRow) *focusCellRow {
	// Deliberately doesn't copy the cells: a widget that holds state must never be shared between rows.
	clone := newFocusCellRow(string(r.id), r.focusableCols)
	clone.parent = newParent
	clone.open = r.open
	return clone
}
func (r *focusCellRow) ID() tid.TID                    { return r.id }
func (r *focusCellRow) Parent() *focusCellRow          { return r.parent }
func (r *focusCellRow) SetParent(parent *focusCellRow) { r.parent = parent }
func (r *focusCellRow) CanHaveChildren() bool          { return len(r.children) > 0 }
func (r *focusCellRow) Children() []*focusCellRow      { return r.children }
func (r *focusCellRow) SetChildren(children []*focusCellRow) {
	r.children = children
	for _, child := range children {
		child.parent = r
	}
}
func (r *focusCellRow) CellDataForSort(_ int) string { return string(r.id) }
func (r *focusCellRow) IsOpen() bool                 { return r.open }
func (r *focusCellRow) SetOpen(open bool)            { r.open = open }

func (r *focusCellRow) ColumnCell(_, col int, _, _ Ink, _, _, focused bool) Paneler {
	r.lastFocusedArg = focused
	if col < 0 || col >= len(r.cells) {
		return NewPanel()
	}
	if r.cells[col] == nil {
		cell := NewPanel()
		if col < r.focusableCols {
			cell.SetFocusable(true)
			cell.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
				cell.RequestFocus()
				return true
			}
			// These deliberately don't chain: a bare panel has no callbacks of its own to preserve.
			cell.GainedFocusCallback = func() { r.gained[col]++ }
			cell.LostFocusCallback = func() { r.lost[col]++ }
		}
		r.cells[col] = cell
	}
	if r.wrap {
		wrapper := NewPanel()
		wrapper.AddChild(r.cells[col])
		return wrapper
	}
	return r.cells[col]
}

// focusCellFixture bundles the pieces the cell focus tests drive: a window that reports itself as focused but has no
// platform resources behind it, a three column table inside that window whose first 'cols' columns hold focusable
// cells, and the rows backing it.
type focusCellFixture struct {
	w     *Window
	model *SimpleTableModel[*focusCellRow]
	table *Table[*focusCellRow]
	rows  []*focusCellRow
}

// newFocusCellFixture builds a table with one row per entry in 'ids', whose first 'cols' columns hold focusable cells.
// The window is marked as focused because both Panel.Focused() and the table's own focus checks report false for every
// panel in a window that doesn't itself hold the keyboard focus, no matter where the focus within it points.
func newFocusCellFixture(t *testing.T, ids []string, cols int) *focusCellFixture {
	t.Helper()
	swapRedrawSet(t)
	w := newRedrawTestWindow()
	w.focused = true
	rows := make([]*focusCellRow, len(ids))
	for i, id := range ids {
		rows[i] = newFocusCellRow(id, cols)
	}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}, {ID: 2, Current: 50}}
	w.Content().AddChild(table)
	table.SyncToModel()
	w.SetFocus(table)
	return &focusCellFixture{w: w, model: model, table: table, rows: rows}
}

// pressCell sends a left button press to the center of the given cell, which is how the tests hand the keyboard focus
// to a focusable cell the way a user would.
func (f *focusCellFixture) pressCell(row, col int) {
	f.table.DefaultMouseDown(f.table.CellFrame(row, col).Center(), ButtonLeft, 1, 0)
}

// TestTableFocusedCellStaysInstalled verifies the core of the mechanism: a cell whose widget takes the keyboard focus
// stays attached to the table instead of being detached again the moment the event that focused it has been forwarded,
// and none of the other traffic through the table's cells disturbs it.
func TestTableFocusedCellStaysInstalled(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
	f.pressCell(1, 0)
	cell := f.rows[1].cells[0]
	c.True(f.w.CurrentFocus() == cell, "the press must leave the focus on the cell that asked for it")
	c.True(cell.Parent() == f.table.AsPanel(), "the focused cell must remain attached to the table")
	c.True(cell.Window() == f.w, "an attached cell can find its window, which is what makes key dispatch work")
	row, col := f.table.FocusedCell()
	c.Equal(1, row)
	c.Equal(0, col)
	c.Equal(1, f.rows[1].gained[0])
	c.Equal(0, f.rows[1].lost[0])

	// Moving the mouse over other cells installs and detaches those cells, but must leave the focused one alone.
	f.table.DefaultMouseEnter(f.table.CellFrame(0, 1).Center(), 0)
	f.table.DefaultMouseEnter(f.table.CellFrame(2, 0).Center(), 0)
	f.table.DefaultMouseExit()
	c.NotNil(f.rows[0].cells[1], "the hover was expected to land on a cell and create it")
	c.NotNil(f.rows[2].cells[0], "the hover was expected to land on a cell and create it")
	c.Nil(f.rows[0].cells[1].Parent(), "a hovered cell must not be left attached")
	c.Nil(f.rows[2].cells[0].Parent(), "a hovered cell must not be left attached")
	c.True(cell.Parent() == f.table.AsPanel(), "hovering elsewhere must not detach the focused cell")
	c.True(f.w.CurrentFocus() == cell)

	// uninstallCell() is the one place that detaches cells, and it has to make an exception for the focused one.
	f.table.uninstallCell(cell, 1, 0)
	c.True(cell.Parent() == f.table.AsPanel(), "uninstalling the focused cell must not detach it")

	// Pressing the cell that already has the focus must not bounce the focus out to the table and back again, which a
	// widget would see as a fresh click into itself rather than a click that just moves the caret.
	f.pressCell(1, 0)
	c.True(f.w.CurrentFocus() == cell)
	c.Equal(1, f.rows[1].gained[0], "re-pressing the focused cell must not re-fire GainedFocusCallback")
	c.Equal(0, f.rows[1].lost[0], "re-pressing the focused cell must not fire LostFocusCallback")
	row, col = f.table.FocusedCell()
	c.Equal(1, row)
	c.Equal(0, col)
}

// TestTableFocusedCellReleasedWhenRowGone verifies that a focused cell whose row or column stops being displayed hands
// the keyboard focus back to the table rather than keeping it in a widget the user can no longer see.
func TestTableFocusedCellReleasedWhenRowGone(t *testing.T) {
	t.Run("row removed", func(t *testing.T) {
		c := check.New(t)
		f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
		f.pressCell(1, 0)
		cell := f.rows[1].cells[0]
		f.model.SetRootRows([]*focusCellRow{f.rows[0], f.rows[2]})
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.CurrentFocus() == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[0])
	})

	t.Run("row removed from another view", func(t *testing.T) {
		c := check.New(t)
		f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
		c.True(f.table.FocusCell(3, 0))
		cell := f.rows[3].cells[0]
		f.model.SetRootRows(f.rows[:3])
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.CurrentFocus() == views[1].AsPanel(), "the view the cell was in takes the focus back")
		c.Equal(1, f.rows[3].lost[0])
	})

	t.Run("column removed", func(t *testing.T) {
		c := check.New(t)
		f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
		f.pressCell(1, 1)
		cell := f.rows[1].cells[1]
		c.True(f.w.CurrentFocus() == cell)
		f.table.Columns = f.table.Columns[:1]
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.CurrentFocus() == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[1])
	})

	t.Run("column removed with its view out of another window", func(t *testing.T) {
		c := check.New(t)
		f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 4}, [2]int{2, 4}, [2]int{0, 2})
		other := newFocusablePanel()
		f.w.Content().AddChild(other)
		f.w.SetFocus(other)
		wnd := newRedrawTestWindow()
		wnd.Content().AddChild(views[1])
		wnd.Content().AddChild(views[2])
		views[1].DefaultMouseDown(views[1].CellFrame(3, 1).Center(), ButtonLeft, 1, 0)
		cell := f.rows[3].cells[1]
		c.True(wnd.CurrentFocus() == cell)
		views[1].RemoveFromParent()
		f.table.Columns = f.table.Columns[:1]
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(wnd.CurrentFocus() == views[2].AsPanel(), "the view left in the cell's window takes the focus back")
		c.True(f.w.CurrentFocus() == other, "the other window's focus stayed where it was")
		c.Equal(1, f.rows[3].lost[1])
	})

	t.Run("parent collapsed", func(t *testing.T) {
		c := check.New(t)
		f := newFocusCellFixture(t, []string{"p"}, 2)
		child := newFocusCellRow("c", 2)
		f.rows[0].SetChildren([]*focusCellRow{child})
		f.rows[0].SetOpen(true)
		f.table.SyncToModel()
		f.pressCell(1, 0)
		cell := child.cells[0]
		c.True(f.w.CurrentFocus() == cell, "the child row's cell should have taken the focus")
		f.rows[0].SetOpen(false)
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.CurrentFocus() == f.table.AsPanel())
		c.Equal(1, child.lost[0])
	})

	t.Run("filtered out", func(t *testing.T) {
		c := check.New(t)
		f := newFocusCellFixture(t, []string{"a", "b"}, 2)
		f.pressCell(1, 0)
		cell := f.rows[1].cells[0]
		// A filter keeps only the rows it returns false for, so this one leaves just "a" showing.
		f.table.ApplyFilter(func(row *focusCellRow) bool { return row.id != "a" })
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.CurrentFocus() == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[0])

		// Dropping the filter brings the row back, but the focus stays where it was put.
		f.table.ApplyFilter(nil)
		row, col = f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.True(f.w.CurrentFocus() == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[0])
	})
}

// TestTableFocusedCellFollowsRowIndexShift verifies that a focused cell survives a sync that moves its row to a
// different index: the row is located again by its ID, the same panel keeps the focus and its frame is brought up to
// date so it is drawn and hit-tested at its new position.
func TestTableFocusedCellFollowsRowIndexShift(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
	f.pressCell(1, 0)
	cell := f.rows[1].cells[0]

	inserted := newFocusCellRow("z", 2)
	f.model.SetRootRows(append([]*focusCellRow{inserted}, f.rows...))
	f.table.SyncToModel()
	row, col := f.table.FocusedCell()
	c.Equal(2, row)
	c.Equal(0, col)
	c.True(f.w.CurrentFocus() == cell, "the same panel must still hold the focus")
	c.True(cell.Parent() == f.table.AsPanel())
	c.Equal(0, f.rows[1].lost[0], "shifting the row must not end the editing")
	c.Equal(f.table.CellFrame(2, 0), cell.FrameRect())

	// Taking the inserted row back out shifts the focused cell back to where it started.
	f.model.SetRootRows(f.rows)
	f.table.SyncToModel()
	row, col = f.table.FocusedCell()
	c.Equal(1, row)
	c.Equal(0, col)
	c.True(f.w.CurrentFocus() == cell)
	c.Equal(0, f.rows[1].lost[0])
	c.Equal(f.table.CellFrame(1, 0), cell.FrameRect())
}

// TestTableCellPressThatChangesTheRowsActsOnThePressedRow verifies that a press whose cell MouseDownCallback changes
// the rows selects the row that was pressed wherever it now is, and nothing once it is gone, that a cell whose widget
// took the focus during the callback is tracked under that row, that the table taking the focus during the callback
// scrolls to that row, and that the drag and release that follow go to the pressed cell where it now is, in whichever
// view shows it, and nowhere once its row is gone. The press is delivered through SafeCall, as the window delivers it.
func TestTableCellPressThatChangesTheRowsActsOnThePressedRow(t *testing.T) {
	for _, tc := range []struct {
		name     string
		order    []int // Indexes into the original rows of the rows the callback leaves, in their new order
		selected []int
		ranges   [][2]int // The ranges of the views, when there is more than one; the press goes to the first
		press    int
		lead     int
		focusRow int
		focus    bool // The callback also takes the focus into the cell and reports the press as handled
		refocus  bool // The cell holds the focus when pressed, and the callback hands the focus back to the table
	}{
		{name: "row moved", order: []int{2, 1, 0}, press: 0, selected: []int{2}, lead: 2, focusRow: -1},
		{name: "row removed with another taking its index", order: []int{0, 2}, press: 1, lead: -1, focusRow: -1},
		{name: "last row removed", order: []int{0, 1}, press: 2, lead: -1, focusRow: -1},
		{
			name:     "row moved by a cell taking the focus",
			order:    []int{2, 1, 0},
			press:    0,
			selected: []int{2},
			lead:     2,
			focusRow: 2,
			focus:    true,
		},
		{
			name:     "row moved by a cell handing the focus back to the table",
			order:    []int{2, 1, 0},
			press:    0,
			selected: []int{2},
			lead:     2,
			focusRow: -1,
			refocus:  true,
		},
		{
			name:     "row moved to another view",
			order:    []int{1, 2, 0},
			press:    0,
			selected: []int{2},
			lead:     2,
			focusRow: -1,
			ranges:   [][2]int{{0, 2}, {2, 3}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			var recovered error
			withRecoveryCallback(t, func(err error) { recovered = err })
			ids := []string{"a", "b", "c"}
			var f *focusCellFixture
			if len(tc.ranges) == 0 {
				f = newFocusCellFixture(t, ids, 1)
			} else {
				f, _ = newTableViewsFixture(t, ids, 1, tc.ranges...)
			}
			cell := f.rows[tc.press].ColumnCell(tc.press, 0, nil, nil, false, false, false).AsPanel()
			cell.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
				rows := make([]*focusCellRow, 0, len(tc.order))
				for _, i := range tc.order {
					rows = append(rows, f.rows[i])
				}
				f.model.SetRootRows(rows)
				f.table.SyncToModel()
				if tc.focus {
					cell.RequestFocus()
				}
				if tc.refocus {
					f.table.RequestFocus()
				}
				return tc.focus
			}
			var drags, ups []geom.Point
			cell.MouseDragCallback = func(where geom.Point, _ int, _ mod.Modifiers) bool {
				drags = append(drags, where)
				return true
			}
			cell.MouseUpCallback = func(where geom.Point, _ int, _ mod.Modifiers) bool {
				ups = append(ups, where)
				return true
			}
			if tc.refocus {
				c.True(f.table.FocusCell(tc.press, 0))
			}
			var scrolled []geom.Rect
			f.w.Content().ScrollRectIntoViewCallback = func(rect geom.Rect) bool {
				scrolled = append(scrolled, rect)
				return true
			}
			SafeCall(func() { f.pressCell(tc.press, 0) })
			c.NoError(recovered)
			var selected []int
			for i := range tc.order {
				if f.table.IsRowSelected(i) {
					selected = append(selected, i)
				}
			}
			c.Equal(tc.selected, selected)
			c.Equal(tc.lead, f.table.LeadRowIndex())
			row, _ := f.table.FocusedCell()
			c.Equal(tc.focusRow, row)
			now := f.table.RowToIndex(f.rows[tc.press])
			var wantScrolled []geom.Rect
			if tc.refocus {
				wantScrolled = append(wantScrolled, f.table.RectTo(f.table.RowFrame(now), f.w.Content()))
			}
			c.Equal(wantScrolled, scrolled, "the table taking the focus scrolls to the pressed row")

			var want []geom.Point
			where := f.table.CellFrame(0, 0).Center()
			if now != -1 {
				v := f.table.ViewForRow(now)
				frame := v.CellFrame(now, 0)
				where = v.PointTo(frame.Center(), f.table.AsPanel())
				want = append(want, frame.Center().Sub(frame.Point))
			}
			f.table.DefaultMouseDrag(where, ButtonLeft, 0)
			f.table.DefaultMouseUp(where, ButtonLeft, 0)
			c.Equal(want, drags, "the drag goes to the pressed cell where it now is")
			c.Equal(want, ups, "the release goes to the pressed cell where it now is")
		})
	}
}

// TestTableFocusCellAPI verifies the programmatic entry point: which arguments it accepts, that it hands the focus over
// from one cell to the next, and that giving the focus back to the table releases the cell.
func TestTableFocusCellAPI(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b"}, 2)
	c.True(f.table.FocusCell(0, 0))
	c.True(f.w.CurrentFocus() == f.rows[0].cells[0])
	c.Equal(1, f.rows[0].gained[0])
	row, col := f.table.FocusedCell()
	c.Equal(0, row)
	c.Equal(0, col)

	// Out of range indexes are rejected without disturbing the cell that has the focus.
	c.False(f.table.FocusCell(-1, 0))
	c.False(f.table.FocusCell(99, 0))
	c.False(f.table.FocusCell(0, -1))
	c.False(f.table.FocusCell(0, 99))
	row, col = f.table.FocusedCell()
	c.Equal(0, row)
	c.Equal(0, col)

	// The last column holds a plain panel, which has nothing in it that can take the focus.
	c.False(f.table.FocusCell(1, 2))
	c.True(f.w.CurrentFocus() == f.rows[0].cells[0])
	row, col = f.table.FocusedCell()
	c.Equal(0, row)
	c.Equal(0, col)

	// Moving to another cell detaches the old one and hands the focus over.
	c.True(f.table.FocusCell(1, 1))
	c.Equal(1, f.rows[0].lost[0])
	c.Nil(f.rows[0].cells[0].Parent())
	c.Equal(1, f.rows[1].gained[1])
	c.True(f.rows[1].cells[1].Parent() == f.table.AsPanel())
	row, col = f.table.FocusedCell()
	c.Equal(1, row)
	c.Equal(1, col)

	// Handing the focus to the table itself ends the editing.
	f.w.SetFocus(f.table)
	c.Equal(1, f.rows[1].lost[1])
	row, col = f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(f.rows[1].cells[1].Parent())
}

// TestTableFocusLeavesCellWhenFocusMovesElsewhere verifies that a focused cell is released when the keyboard focus goes
// to some unrelated panel, or is removed from the window altogether. In neither case may the table try to pull the
// focus back to itself.
func TestTableFocusLeavesCellWhenFocusMovesElsewhere(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b"}, 2)
	other := newFocusablePanel()
	f.w.Content().AddChild(other)

	f.pressCell(0, 0)
	cell := f.rows[0].cells[0]
	f.w.SetFocus(other)
	c.Equal(1, f.rows[0].lost[0])
	c.True(f.w.CurrentFocus() == other)
	row, col := f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(cell.Parent())
	c.True(f.w.CurrentFocus() == other, "releasing the cell must not steal the focus back")

	// Removing the focus entirely is the other way the cell can lose it.
	f.pressCell(0, 0)
	c.True(f.w.CurrentFocus() == cell)
	c.Equal(2, f.rows[0].gained[0])
	f.w.SetFocus(nil)
	c.Equal(2, f.rows[0].lost[0])
	row, col = f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(cell.Parent())
	c.Nil(f.w.CurrentFocus(), "the window must be left with no focus at all")
}

// TestTableCellParamsFocusedWhenCellFocused verifies the 'focused' argument the table passes to ColumnCell(): it is
// true when the table itself has the keyboard focus and stays true while one of its cells is being edited, so a
// selected row keeps its focused look, and is false once the focus has moved somewhere else entirely.
func TestTableCellParamsFocusedWhenCellFocused(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b"}, 2)
	f.table.cell(0, 0)
	c.True(f.rows[0].lastFocusedArg, "the table itself holds the focus")

	f.pressCell(0, 0)
	f.table.cell(1, 0)
	c.True(f.rows[1].lastFocusedArg, "a focused cell must keep the table's cells looking focused")

	other := newFocusablePanel()
	f.w.Content().AddChild(other)
	f.w.SetFocus(other)
	f.table.cell(0, 0)
	c.False(f.rows[0].lastFocusedArg)
	f.table.cell(1, 0)
	c.False(f.rows[1].lastFocusedArg)
}

// TestTableKeyFromFocusedCell verifies the table's handling of the keys that bubble up out of a focused cell. The keys
// are pushed through Window.keyPressed() rather than straight into the table so that the window's own Tab traversal
// gets its turn after the table declines the key, which is what carries the focus out of the table.
func TestTableKeyFromFocusedCell(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
	after := newFocusablePanel()
	f.w.Content().AddChild(after)
	doubleClicks := 0
	f.table.DoubleClickCallback = func() { doubleClicks++ }
	f.table.SelectByIndex(1)

	// Arrow keys that a cell's widget didn't want are not the table's to act on: moving the selection out from under
	// something that is being edited would be wrong, so they are ignored and the cell keeps the focus. The focus
	// arriving in the cell has made its row the whole of the selection, which is where the cursor now stands.
	f.pressCell(0, 0)
	cell := f.rows[0].cells[0]
	c.True(f.w.CurrentFocus() == cell)
	c.True(f.table.IsRowSelected(0), "the focus arriving in a cell selects its row")
	f.w.keyPressed(KeyDown, 0)
	f.w.keyPressed(KeyUp, 0)
	c.Equal(1, f.table.SelectionCount())
	c.True(f.table.IsRowSelected(0), "the selection must not move while a cell is focused")
	c.True(f.w.CurrentFocus() == cell)

	// Tab moves to the next focusable cell, in row-major order.
	f.w.keyPressed(KeyTab, 0)
	row, col := f.table.FocusedCell()
	c.Equal(0, row)
	c.Equal(1, col)
	c.True(f.w.CurrentFocus() == f.rows[0].cells[1])
	c.Equal(1, f.rows[0].lost[0])
	c.Equal(1, f.rows[0].gained[1])
	c.Nil(cell.Parent(), "the cell that was left behind must be detached again")

	// Tab from the last focusable cell of the last row leaves the table, landing on the panel that follows it.
	c.True(f.table.FocusCell(2, 1))
	f.w.keyPressed(KeyTab, 0)
	c.True(f.w.CurrentFocus() == after)
	row, col = f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(f.rows[2].cells[1].Parent())
	c.Equal(1, f.rows[2].lost[1])

	// Return and Escape end the editing by handing the focus back to the table, and neither of them may reach the
	// table's own Space-to-DoubleClickCallback shortcut.
	for _, key := range []KeyCode{KeyReturn, KeyEscape} {
		f.pressCell(1, 0)
		c.True(f.w.CurrentFocus() == f.rows[1].cells[0])
		f.w.keyPressed(key, 0)
		c.True(f.w.CurrentFocus() == f.table.AsPanel())
		row, col = f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(f.rows[1].cells[0].Parent())
		c.Equal(0, doubleClicks)
	}

	// A Space that bubbles out of a focused cell belongs to whatever is being edited, not to the table.
	f.pressCell(1, 0)
	f.w.keyPressed(KeySpace, 0)
	c.Equal(0, doubleClicks)
	c.True(f.w.CurrentFocus() == f.rows[1].cells[0], "an unwanted key must not cost the cell its focus")

	// Sanity check that the table does still run its DoubleClickCallback for a Space when it holds the focus itself.
	f.w.SetFocus(f.table)
	f.w.keyPressed(KeySpace, 0)
	c.Equal(1, doubleClicks)
}

// newTableViewsFixture builds the table of newFocusCellFixture and splits it into views, one per range, the primary
// showing the first. The views are placed side by side in the window, so that each one's rows start at its top.
func newTableViewsFixture(t *testing.T, ids []string, cols int, ranges ...[2]int) (*focusCellFixture,
	[]*Table[*focusCellRow],
) {
	t.Helper()
	f := newFocusCellFixture(t, ids, cols)
	f.table.SetDrawRowRange(ranges[0][0], ranges[0][1])
	views := make([]*Table[*focusCellRow], 0, len(ranges))
	views = append(views, f.table)
	for _, r := range ranges[1:] {
		v := f.table.NewView(r[0], r[1])
		f.w.Content().AddChild(v)
		views = append(views, v)
	}
	f.table.SyncToModel()
	var x float32
	for _, v := range views {
		frame := v.FrameRect()
		frame.Point = geom.NewPoint(x, 0)
		v.SetFrameRect(frame)
		x = frame.Right() + 10
	}
	return f, views
}

var (
	fourTableViewIDs = []string{"a", "b", "c", "d"}
	sixTableViewIDs  = []string{"a", "b", "c", "d", "e", "f"}
)

// TestTableViewsShareStateButNotGestures verifies what a view shares with the table it was made from, which is
// everything but the state of a gesture, and that a new view starts with no gesture under way.
func TestTableViewsShareStateButNotGestures(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
	p, v := views[0], views[1]
	c.True(v.Primary() == p)
	c.True(slices.Equal(views, p.Views()))
	c.True(SameTable(p, v))
	c.False(SameTable(p, NewTable[*focusCellRow](f.model)), "a second table over the same model is another table")

	v.SelectByIndex(3)
	c.True(p.IsRowSelected(3), "the selection is shared")
	p.Columns[1].Current = 77
	c.Equal(float32(77), v.Columns[1].Current, "the columns are shared")
	p.MinimumRowHeight = 30
	c.Equal(float32(30), v.MinimumRowHeight, "the theme is shared")
	p.ClientData()["key"] = 1
	c.Equal(1, v.ClientData()["key"], "the client data is shared")

	c.Equal(-1, v.interactionRow, "a new view starts with no gesture under way")
	c.Equal(-1, v.lastMouseMotionRow)
	v.DefaultMouseDown(v.CellFrame(3, 0).Center(), ButtonLeft, 1, 0)
	c.Equal(3, v.interactionRow)
	c.Equal(-1, p.interactionRow, "a press in one view leaves the gestures of the others alone")
}

// TestTableViewGeometryIsRelativeToItsRange verifies that a view measures its rows from the first row of its range and
// has no position for a row it does not show.
func TestTableViewGeometryIsRelativeToItsRange(t *testing.T) {
	c := check.New(t)
	_, views := newTableViewsFixture(t, sixTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4}, [2]int{4, 6})
	for i, v := range views {
		first := i * 2
		frame := v.RowFrame(first)
		c.Equal(v.ContentRect(false).Y, frame.Y, "view %d shows its first row at its top", i)
		c.Equal(frame.Y+v.Padding.Top, v.CellFrame(first, 1).Y, "view %d", i)
		c.Equal(first, v.OverRow(frame.CenterY()), "view %d", i)
		c.Equal(first+1, v.OverRow(v.RowFrame(first+1).CenterY()), "view %d", i)
		c.Equal(-1, v.OverRow(v.RowFrame(first+1).Bottom()+1), "view %d has no row below its last", i)
		other := (first + 2) % 6
		c.True(v.RowFrame(other).Empty(), "view %d has no frame for a row it does not show", i)
		c.True(v.CellFrame(other, 0).Empty(), "view %d has no frame for a cell it does not show", i)
	}
}

// TestTableViewForRowIgnoresDetachedAndHiddenViews verifies which view shows a row: the first one in a window,
// neither hidden nor beneath a hidden panel, whose range holds it, with the primary standing in when there is none.
func TestTableViewForRowIgnoresDetachedAndHiddenViews(t *testing.T) {
	for _, test := range []struct {
		disable func(v *Table[*focusCellRow])
		name    string
	}{
		{name: "view out of the window", disable: func(v *Table[*focusCellRow]) { v.RemoveFromParent() }},
		{name: "hidden view", disable: func(v *Table[*focusCellRow]) { v.Hidden = true }},
		{
			name: "view beneath a hidden panel",
			disable: func(v *Table[*focusCellRow]) {
				hidden := NewPanel()
				hidden.Hidden = true
				v.Parent().AddChild(hidden)
				hidden.AddChild(v)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			_, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4}, [2]int{2, 4})
			p := views[0]
			c.True(p.ViewForRow(3) == views[1], "the first view showing the row is the one it goes to")
			test.disable(views[1])
			c.True(p.ViewForRow(3) == views[2], "a %s does not show the row", test.name)
			test.disable(views[2])
			c.True(p.ViewForRow(3) == p, "the primary stands in when no view shows the row")
		})
	}
}

// TestTableViewWithoutARangeShowsEveryRow verifies that a view with no range set shows every row, so that a cell is
// focused in it even while other views show some of the rows.
func TestTableViewWithoutARangeShowsEveryRow(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
	p := views[0]
	p.ClearDrawRowRange()
	start, end := p.CurrentDrawRowRange()
	c.Equal(0, start)
	c.Equal(4, end)
	c.True(p.FocusCell(0, 0), "the cell is focused in the view showing its row")
	c.True(f.rows[0].cells[0].Parent() == p.AsPanel())
}

// TestTableRangeIsClampedNotWidened verifies that a view shows the part of its range the table has rows for, even
// nothing at all, rather than every row of the table, whether the range was set that way or rows removed later cut it
// back.
func TestTableRangeIsClampedNotWidened(t *testing.T) {
	for _, test := range []struct {
		name               string
		set                []int // The range given to the view in place of [2, 4), if any
		keep               int
		start, end, topRow int
	}{
		{name: "cut back", keep: 3, start: 2, end: 3, topRow: 2},
		{name: "emptied", keep: 1, start: 1, end: 1, topRow: -1},
		{name: "set empty", set: []int{4, 4}, keep: 4, start: 4, end: 4, topRow: -1},
		{name: "set from below zero", set: []int{-1, 1}, keep: 4, start: 0, end: 1, topRow: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
			v := views[1]
			if test.set != nil {
				v.SetDrawRowRange(test.set[0], test.set[1])
			}
			f.model.SetRootRows(f.rows[:test.keep])
			f.table.SyncToModel()
			start, end := v.CurrentDrawRowRange()
			c.Equal(test.start, start)
			c.Equal(test.end, end)
			c.Equal(test.topRow, v.OverRow(v.ContentRect(false).Y+1))
		})
	}
}

// TestTableFocusedCellLivesInTheViewShowingItsRow verifies that a cell given the keyboard focus through any view is
// attached to the view showing its row, at its frame there, and is let go of when the focus moves to a cell another
// view shows.
func TestTableFocusedCellLivesInTheViewShowingItsRow(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
	p, v := views[0], views[1]
	c.True(p.FocusCell(3, 0), "the primary hands the focus to a cell the other view shows")
	cell := f.rows[3].cells[0]
	c.True(f.w.CurrentFocus() == cell)
	c.True(cell.Parent() == v.AsPanel(), "the cell is attached to the view showing its row")
	c.Equal(v.CellFrame(3, 0), cell.FrameRect())
	c.True(p.FocusedView() == v)

	p.DefaultMouseDown(p.CellFrame(1, 1).Center(), ButtonLeft, 1, 0)
	c.True(f.rows[1].cells[1].Parent() == p.AsPanel(), "a press in the primary focuses a cell of its own")
	c.Nil(cell.Parent(), "the cell that lost the focus is detached from its view")
	c.True(p.FocusedView() == p)
}

// TestTableFocusedCellMovesWithRepagination verifies that a focused cell whose row passes to another view, through new
// ranges, through its view being taken out of the window, or through its view being removed, moves to that view
// without losing the focus, even when it is looked at while no view shows its row, and that it stays in the window
// whose focus it holds, at its frame there, when a view of another window shows its row, even once that view drew it.
func TestTableFocusedCellMovesWithRepagination(t *testing.T) {
	newRanges := func(_ check.Checker, _ *Table[*focusCellRow], views []*Table[*focusCellRow]) {
		views[1].SetDrawRowRange(2, 3)
		views[2].SetDrawRowRange(3, 6)
	}
	draw := func(c check.Checker, v *Table[*focusCellRow]) {
		size := v.FrameRect().Size
		img, err := NewImageFromDrawing(int(size.Width), int(size.Height), 72, func(canvas *Canvas) {
			v.DefaultDraw(canvas, v.ContentRect(true))
		})
		c.NoError(err)
		if err == nil {
			img.Dispose()
		}
	}
	for _, test := range []struct {
		repaginate  func(c check.Checker, table *Table[*focusCellRow], views []*Table[*focusCellRow])
		name        string
		otherWindow bool // Puts the last view in a window of its own
	}{
		{name: "new ranges", repaginate: newRanges},
		{
			name: "view taken out of the window",
			repaginate: func(_ check.Checker, table *Table[*focusCellRow], views []*Table[*focusCellRow]) {
				views[1].RemoveFromParent()
				table.FocusedCell()
				views[2].SetDrawRowRange(2, 6)
			},
		},
		{
			name: "view removed",
			repaginate: func(_ check.Checker, table *Table[*focusCellRow], views []*Table[*focusCellRow]) {
				table.RemoveView(views[1])
				views[1].RemoveFromParent()
				table.FocusedCell()
				views[2].SetDrawRowRange(2, 6)
			},
		},
		{name: "new ranges passing the row to another window", repaginate: newRanges, otherWindow: true},
		{
			name: "new ranges passing the row to another window that draws it",
			repaginate: func(c check.Checker, table *Table[*focusCellRow], views []*Table[*focusCellRow]) {
				newRanges(c, table, views)
				draw(c, views[2])
			},
			otherWindow: true,
		},
		{
			name: "row also shown by another window that draws it",
			repaginate: func(c check.Checker, table *Table[*focusCellRow], views []*Table[*focusCellRow]) {
				views[2].SetDrawRowRange(3, 6)
				draw(c, views[2])
				c.Equal(views[1].CellFrame(3, 0), table.focusedCell.FrameRect(), "the cell is back at its frame")
			},
			otherWindow: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, sixTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4}, [2]int{4, 6})
			if test.otherWindow {
				newRedrawTestWindow().Content().AddChild(views[2])
			}
			c.True(f.table.FocusCell(3, 0))
			cell := f.rows[3].cells[0]
			test.repaginate(c, f.table, views)
			row, col := f.table.FocusedCell()
			c.Equal(3, row)
			c.Equal(0, col)
			if !test.otherWindow {
				c.True(cell.Parent() == views[2].AsPanel(), "the cell moved to the view now showing its row")
				c.Equal(views[2].CellFrame(3, 0), cell.FrameRect())
			}
			c.True(f.w.CurrentFocus() == cell, "the cell kept the focus in its window")
			c.Equal(0, f.rows[3].lost[0], "moving the cell did not take the focus from it")
		})
	}
}

// TestTableRemoveViewHandsOnTheFocusTheViewHeld verifies that removing a view holding the keyboard focus, whether it is
// taken out of its parent before or after, hands the focus to another view of the table rather than leaving it on a
// panel that is no longer in the window, and that a cell the view shows that holds the focus of another window moves to
// a view left in that window, even when only a view of the first window shows its row.
func TestTableRemoveViewHandsOnTheFocusTheViewHeld(t *testing.T) {
	for _, test := range []struct {
		name              string
		detachFirst       bool
		cellInOtherWindow bool
	}{
		{name: "detached after"},
		{name: "detached before", detachFirst: true},
		{name: "cell in another window, detached after", cellInOtherWindow: true},
		{name: "cell in another window, detached before", detachFirst: true, cellInOtherWindow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
			f.table.SelectByIndex(3)
			removed := views[1]
			var wnd *Window
			var left *Table[*focusCellRow]
			if test.cellInOtherWindow {
				wnd = newRedrawTestWindow()
				left = f.table.NewView(0, 2)
				removed = f.table.NewView(2, 4)
				wnd.Content().AddChild(left)
				wnd.Content().AddChild(removed)
				f.table.SyncToModel()
				removed.DefaultMouseDown(removed.CellFrame(3, 0).Center(), ButtonLeft, 1, 0)
				c.True(wnd.CurrentFocus() == f.rows[3].cells[0])
			} else {
				f.w.SetFocus(views[1])
				c.True(f.w.CurrentFocus() == views[1].AsPanel())
			}
			if test.detachFirst {
				removed.RemoveFromParent()
			}
			c.True(f.table.RemoveView(removed))
			removed.RemoveFromParent()
			if test.cellInOtherWindow {
				cell := f.rows[3].cells[0]
				c.True(wnd.CurrentFocus() == cell, "the cell kept the focus of its window")
				c.True(cell.Parent() == left.AsPanel(), "the cell moved to the view left in its window")
				return
			}
			f.table.SetDrawRowRange(0, 4)
			c.True(f.w.CurrentFocus() == f.table.AsPanel(), "the focus moved to the view left showing the person's row")
		})
	}
}

// TestTableSizingAdoptsAWrapperForACellFocusedInAnotherView verifies that sizing the columns through a primary that is
// in no window, or in another window, while the editor of a row that wraps it afresh on every call is focused in
// another view, attaches the newest wrapper to that view rather than leaving the editor cut off from its window.
func TestTableSizingAdoptsAWrapperForACellFocusedInAnotherView(t *testing.T) {
	for _, test := range []struct {
		name        string
		otherWindow bool
	}{
		{name: "primary in no window"},
		{name: "primary in another window", otherWindow: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
			for _, row := range f.rows {
				row.wrap = true
			}
			wnd := f.w
			if test.otherWindow {
				wnd = newRedrawTestWindow()
				wnd.Content().AddChild(views[1])
			} else {
				f.table.RemoveFromParent()
			}
			c.True(views[1].FocusCell(3, 0))
			editor := f.rows[3].cells[0]
			c.True(wnd.CurrentFocus() == editor)

			f.table.SizeColumnsToFit(false)
			c.True(wnd.CurrentFocus() == editor, "the focused editor is still in its window")
			c.True(editor.Parent().Parent() == views[1].AsPanel(), "the newest wrapper is attached to the view showing it")
			row, col := views[1].FocusedCell()
			c.Equal(3, row)
			c.Equal(0, col)
		})
	}
}

// TestTableSyncSizesEveryViewToItsOwnRange verifies that a sync through any view sizes every view of the table to the
// rows of its own range.
func TestTableSyncSizesEveryViewToItsOwnRange(t *testing.T) {
	c := check.New(t)
	_, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 1}, [2]int{1, 4})
	views[0].SetDrawRowRange(0, 3)
	views[1].SetDrawRowRange(3, 4)
	views[1].SyncToModel()
	for i, v := range views {
		_, pref, _ := v.DefaultSizes(geom.Size{})
		c.Equal(pref, v.FrameRect().Size, "view %d", i)
	}
	c.True(views[0].FrameRect().Height > views[1].FrameRect().Height, "each view is sized to its own range")
}

// TestTableColumnFitLaysTheViewsOutAgain verifies that fitting the columns with the views adjusted to them has the
// views laid out again, so that views side by side neither overlap nor leave a gap once their widths change.
func TestTableColumnFitLaysTheViewsOutAgain(t *testing.T) {
	for _, test := range []struct {
		fit  func(table *Table[*focusCellRow])
		name string
	}{
		{name: "every column", fit: func(table *Table[*focusCellRow]) { table.SizeColumnsToFit(true) }},
		{name: "one column", fit: func(table *Table[*focusCellRow]) { table.SizeColumnToFit(0, true) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
			content := f.w.Content()
			content.SetLayout(&FlexLayout{Columns: 2})
			content.ValidateLayout()
			width := views[0].FrameRect().Width
			test.fit(f.table)
			c.NotEqual(width, views[0].FrameRect().Width, "the fit changed the width of the views")
			content.ValidateLayout()
			c.Equal(views[0].FrameRect().Right(), views[1].FrameRect().X, "the views are side by side again")
		})
	}
}

// TestTableTabCrossesViewsAndLeavesFromTheLastCell verifies that Tab moves from the last focusable cell one view shows
// to the first one the next view shows, and that a Tab from the last focusable cell of the table leaves the table
// rather than landing on a view after it.
func TestTableTabCrossesViewsAndLeavesFromTheLastCell(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, sixTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4}, [2]int{4, 6})
	after := newFocusablePanel()
	f.w.Content().AddChild(after)
	for _, row := range f.rows[4:] {
		for _, cell := range row.cells {
			cell.SetFocusable(false)
		}
	}

	c.True(f.table.FocusCell(1, 1))
	f.w.keyPressed(KeyTab, 0)
	cell := f.rows[2].cells[0]
	c.True(f.w.CurrentFocus() == cell, "Tab moved to the first focusable cell of the next view")
	c.True(cell.Parent() == views[1].AsPanel())
	c.Equal(views[1].CellFrame(2, 0), cell.FrameRect())

	c.True(f.table.FocusCell(3, 1))
	f.w.keyPressed(KeyTab, 0)
	c.True(f.w.CurrentFocus() == after, "Tab from the last focusable cell left the table")
}

// TestTableCellIsFocusedInTheWindowOfTheViewAsked verifies that a cell given the keyboard focus through a view, from
// code, by a Tab from the cell before it, by a click, by a right press on a widget with a menu, or by an assistive
// technology, takes the focus in that view's window, even though a view of another window that comes first shows its
// row too, and that the other window's focus is left alone, or handed back to its view when a cell held it.
func TestTableCellIsFocusedInTheWindowOfTheViewAsked(t *testing.T) {
	for _, test := range []struct {
		focus func(c check.Checker, f *focusCellFixture, v *Table[*focusCellRow], wnd *Window)
		name  string
		row   int
	}{
		{
			name: "focus cell",
			focus: func(c check.Checker, _ *focusCellFixture, v *Table[*focusCellRow], _ *Window) {
				c.True(v.FocusCell(3, 0))
			},
			row: 3,
		},
		{
			name: "tab into the next row",
			focus: func(c check.Checker, f *focusCellFixture, v *Table[*focusCellRow], wnd *Window) {
				v.DefaultMouseDown(v.CellFrame(1, 1).Center(), ButtonLeft, 1, 0)
				c.True(wnd.CurrentFocus() == f.rows[1].cells[1])
				wnd.keyPressed(KeyTab, 0)
				c.Equal(1, f.rows[1].lost[1], "the editing the Tab left ended")
			},
			row: 2,
		},
		{
			name: "click on the cell edited in the other window",
			focus: func(c check.Checker, f *focusCellFixture, v *Table[*focusCellRow], _ *Window) {
				f.pressCell(3, 0)
				c.True(f.w.CurrentFocus() == f.rows[3].cells[0])
				v.DefaultMouseDown(v.CellFrame(3, 0).Center(), ButtonLeft, 1, 0)
				c.Equal(1, f.rows[3].lost[0], "the editing in the other window ended")
			},
			row: 3,
		},
		{
			name: "click on another cell while one is edited in the other window",
			focus: func(c check.Checker, f *focusCellFixture, v *Table[*focusCellRow], _ *Window) {
				f.pressCell(1, 0)
				c.True(f.w.CurrentFocus() == f.rows[1].cells[0])
				v.DefaultMouseDown(v.CellFrame(3, 0).Center(), ButtonLeft, 1, 0)
				c.Equal(1, f.rows[1].lost[0], "the editing in the other window ended")
			},
			row: 3,
		},
		{
			name: "right press on a widget with a menu in the cell edited in the other window",
			focus: func(c check.Checker, f *focusCellFixture, v *Table[*focusCellRow], _ *Window) {
				f.pressCell(3, 0)
				cell := f.rows[3].cells[0]
				c.True(f.w.CurrentFocus() == cell)
				cell.ContextMenuCallback = func(geom.Point) Menu { return nil }
				c.True(v.contextMenuOwnerWithin(v.CellFrame(3, 0).Center()) == cell)
				c.Equal(1, f.rows[3].lost[0], "the editing in the other window ended")
			},
			row: 3,
		},
		{
			name: "assistive technology focusing a widget in the cell",
			focus: func(c check.Checker, _ *focusCellFixture, v *Table[*focusCellRow], _ *Window) {
				c.True(v.PerformAccessibilityAction(accessibility.ActionRequest{
					Key:    axCellPanelKey{Cell: accessibility.CellKey{Row: "d", Col: 0}},
					Action: accessibility.Focus,
				}))
			},
			row: 3,
		},
		{
			name: "assistive technology pressing the cell",
			focus: func(c check.Checker, _ *focusCellFixture, v *Table[*focusCellRow], _ *Window) {
				v.cell(3, 0).MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool { return true }
				c.True(v.PerformAccessibilityAction(accessibility.ActionRequest{
					Key:    accessibility.CellKey{Row: "d", Col: 0},
					Action: accessibility.Press,
				}))
			},
			row: 3,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 4}, [2]int{0, 4})
			wnd := newRedrawTestWindow()
			wnd.Content().AddChild(views[1])
			test.focus(c, f, views[1], wnd)
			cell := f.rows[test.row].cells[0]
			c.True(wnd.CurrentFocus() == cell, "the cell took the focus of the view's window")
			c.True(cell.Parent() == views[1].AsPanel(), "the cell is attached to the view in that window")
			c.True(f.w.CurrentFocus() == f.table.AsPanel(), "the other window's focus is on its view")
		})
	}
}

// TestTableKeysLeavingACellFocusTheViewNowShowingItsRow verifies that Return, Escape and a Tab out of the last
// focusable cell hand the focus to the view showing the cell's row, even when the row has passed to another view since
// the cell took the focus, so that a Tab traverses on from there.
func TestTableKeysLeavingACellFocusTheViewNowShowingItsRow(t *testing.T) {
	for _, test := range []struct {
		name string
		key  KeyCode
	}{
		{name: "return", key: KeyReturn},
		{name: "escape", key: KeyEscape},
		{name: "tab out", key: KeyTab},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, sixTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4}, [2]int{4, 6})
			f.w.Content().AddChildAtIndex(newFocusablePanel(), 2)
			after := newFocusablePanel()
			f.w.Content().AddChild(after)
			for _, row := range f.rows[4:] {
				for _, cell := range row.cells {
					cell.SetFocusable(false)
				}
			}
			c.True(f.table.FocusCell(3, 1))
			views[1].SetDrawRowRange(2, 3)
			views[2].SetDrawRowRange(3, 6)
			f.w.keyPressed(test.key, 0)
			want := views[2].AsPanel()
			if test.key == KeyTab {
				want = after
			}
			c.True(f.w.CurrentFocus() == want, "the focus left the cell from the view now showing its row")
		})
	}
}

// TestTableArrowAcrossAViewBoundaryMovesTheFocusToThatView verifies that a key moving the person onto a row another
// view shows hands that view the keyboard focus, since only the view holding it can report it on the row, and leaves
// the focus where it is, in both windows, when that view cannot take the focus or is in another window.
func TestTableArrowAcrossAViewBoundaryMovesTheFocusToThatView(t *testing.T) {
	for _, test := range []struct {
		setup func(views []*Table[*focusCellRow]) (verify func(c check.Checker))
		name  string
		key   KeyCode
		from  int // The view holding the focus before the key
		row   int // The row selected before the key
		focus int // The view holding the focus after the key
	}{
		{name: "down onto the next view", key: KeyDown, row: 1, focus: 1},
		{name: "up onto the primary", key: KeyUp, from: 1, row: 2},
		{
			name: "down onto a disabled view",
			setup: func(views []*Table[*focusCellRow]) func(c check.Checker) {
				views[1].SetEnabled(false)
				return nil
			},
			key: KeyDown,
			row: 1,
		},
		{
			name: "up onto a disabled primary",
			setup: func(views []*Table[*focusCellRow]) func(c check.Checker) {
				views[0].SetEnabled(false)
				return nil
			},
			key:   KeyUp,
			from:  1,
			row:   2,
			focus: 1,
		},
		{
			name: "down onto a view in another window",
			setup: func(views []*Table[*focusCellRow]) func(c check.Checker) {
				wnd := newRedrawTestWindow()
				wnd.Content().AddChild(views[1])
				field := newFocusablePanel()
				wnd.Content().AddChild(field)
				wnd.SetFocus(field)
				lost := 0
				field.LostFocusCallback = func() { lost++ }
				return func(c check.Checker) {
					c.True(wnd.CurrentFocus() == field, "the other window's focus stayed where it was")
					c.Equal(0, lost, "the other window's focus did not leave its panel")
				}
			},
			key: KeyDown,
			row: 1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
			var verify func(c check.Checker)
			if test.setup != nil {
				verify = test.setup(views)
			}
			f.w.SetFocus(views[test.from])
			f.table.SelectByIndex(test.row)
			f.w.keyPressed(test.key, 0)
			lead := test.row + 1
			if test.key == KeyUp {
				lead = test.row - 1
			}
			c.Equal(lead, f.table.LeadRowIndex())
			c.True(f.w.CurrentFocus() == views[test.focus].AsPanel(), "view %d holds the focus", test.focus)
			if verify != nil {
				verify(c)
			}
		})
	}
}

// TestTableMovingOntoARowFollowsItOnceSelected verifies that moving onto a row, from code or for an assistive
// technology, scrolls to the row and puts the focus on the view showing it where the row is once selecting it is done,
// when the SelectionChangedCallback that ran has moved the row to another view.
func TestTableMovingOntoARowFollowsItOnceSelected(t *testing.T) {
	for _, test := range []struct {
		move func(table *Table[*focusCellRow]) bool
		name string
		col  int
	}{
		{
			name: "assistive technology focus",
			col:  -1,
			move: func(table *Table[*focusCellRow]) bool {
				return table.PerformAccessibilityAction(accessibility.ActionRequest{
					Key:    tid.TID("a"),
					Action: accessibility.Focus,
				})
			},
		},
		{
			name: "assistive technology select",
			col:  -1,
			move: func(table *Table[*focusCellRow]) bool {
				return table.PerformAccessibilityAction(accessibility.ActionRequest{
					Key:    tid.TID("a"),
					Action: accessibility.Select,
				})
			},
		},
		{
			name: "assistive technology add to selection",
			col:  -1,
			move: func(table *Table[*focusCellRow]) bool {
				return table.PerformAccessibilityAction(accessibility.ActionRequest{
					Key:    tid.TID("a"),
					Action: accessibility.AddToSelection,
				})
			},
		},
		{name: "lead row", col: -1, move: func(table *Table[*focusCellRow]) bool { return table.SetLeadCell(0, -1) }},
		{name: "lead cell", col: 1, move: func(table *Table[*focusCellRow]) bool { return table.SetLeadCell(0, 1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
			f.table.SelectionChangedCallback = func() {
				f.model.SetRootRows([]*focusCellRow{f.rows[1], f.rows[2], f.rows[3], f.rows[0]})
				f.table.SyncToModel()
			}
			var scrolled []geom.Rect
			f.w.Content().ScrollRectIntoViewCallback = func(rect geom.Rect) bool {
				scrolled = append(scrolled, rect)
				return true
			}
			c.True(test.move(f.table))
			c.Equal(3, f.table.axCurrentRow(), "the callback moved the row to the other view")
			v := views[1]
			want := v.RowFrame(3)
			if test.col >= 0 {
				want = v.CellFrame(3, test.col)
			}
			c.Equal([]geom.Rect{v.RectTo(want, f.w.Content())}, scrolled, "the row was scrolled to where it is now")
			c.True(f.w.CurrentFocus() == v.AsPanel(), "the view now showing the row holds the focus")
		})
	}
}

// TestTableAccessibilityFocusStaysInTheWindowOfTheViewAsked verifies that an assistive technology putting the focus on
// a row hands the keyboard focus to the view that shows the row in the window of the view asked and can take it, and
// that with no such view the request is refused, leaving the focus of each window where it was.
func TestTableAccessibilityFocusStaysInTheWindowOfTheViewAsked(t *testing.T) {
	for _, test := range []struct {
		setup   func(views []*Table[*focusCellRow], other *Window)
		name    string
		focus   int // The view holding the focus of the first window afterwards
		handled bool
	}{
		{
			name:    "behind a view of another window",
			setup:   func(views []*Table[*focusCellRow], other *Window) { other.Content().AddChild(views[1]) },
			focus:   2,
			handled: true,
		},
		{
			name: "shown only in another window",
			setup: func(views []*Table[*focusCellRow], other *Window) {
				other.Content().AddChild(views[1])
				other.Content().AddChild(views[2])
			},
		},
		{
			name: "shown only by disabled views",
			setup: func(views []*Table[*focusCellRow], _ *Window) {
				views[1].SetEnabled(false)
				views[2].SetEnabled(false)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 0, [2]int{0, 2}, [2]int{2, 4}, [2]int{2, 4})
			other := newRedrawTestWindow()
			field := newFocusablePanel()
			other.Content().AddChild(field)
			other.SetFocus(field)
			test.setup(views, other)
			c.Equal(test.handled, views[0].PerformAccessibilityAction(accessibility.ActionRequest{
				Key:    tid.TID("d"),
				Action: accessibility.Focus,
			}))
			c.True(f.w.CurrentFocus() == views[test.focus].AsPanel(), "view %d holds the focus", test.focus)
			c.True(other.CurrentFocus() == field, "the other window's focus stayed where it was")
		})
	}
}

// TestTableCursorRingDrawnOnlyInTheLeadRowsView verifies that the ring around the cell the cursor is on is drawn by
// the view showing the cell's row, and by no other.
func TestTableCursorRingDrawnOnlyInTheLeadRowsView(t *testing.T) {
	c := check.New(t)
	_, views := newTableViewsFixture(t, fourTableViewIDs, 0, [2]int{0, 2}, [2]int{2, 4})
	render := func(v *Table[*focusCellRow]) []byte {
		size := v.FrameRect().Size
		img, err := NewImageFromDrawing(int(size.Width), int(size.Height), 72, func(canvas *Canvas) {
			v.DefaultDraw(canvas, v.ContentRect(true))
		})
		c.NoError(err)
		if err != nil {
			return nil
		}
		defer img.Dispose()
		pixels, err := img.ToNRGBA()
		c.NoError(err)
		if err != nil {
			return nil
		}
		return pixels.Pix
	}
	c.True(views[0].SetLeadCell(1, -1))
	primary := render(views[0])
	other := render(views[1])
	c.True(views[0].SetLeadCell(1, 0))
	c.False(slices.Equal(primary, render(views[0])), "the view showing the cursor's row draws the ring")
	c.True(slices.Equal(other, render(views[1])), "the view not showing the cursor's row draws no ring")
}

// TestTableCellsLookFocusedWhenAnotherViewHoldsTheFocus verifies that a view hands its cells the focused look while
// another view of the table holds the keyboard focus.
func TestTableCellsLookFocusedWhenAnotherViewHoldsTheFocus(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4})
	views[1].cell(3, 0)
	c.True(f.rows[3].lastFocusedArg, "the primary holds the focus")
	other := newFocusablePanel()
	f.w.Content().AddChild(other)
	f.w.SetFocus(other)
	views[1].cell(3, 0)
	c.False(f.rows[3].lastFocusedArg, "no view holds the focus")
}

// TestTableContextMenuAnchorIsInTheFocusedViewsCoordinates verifies that a view asked where a contextual menu opened
// from the keyboard goes answers with the point beneath the row the person is on, wherever the view showing that row
// is, in its own coordinates, or with its default anchor when no view shows the row.
func TestTableContextMenuAnchorIsInTheFocusedViewsCoordinates(t *testing.T) {
	c := check.New(t)
	_, views := newTableViewsFixture(t, sixTableViewIDs, 0, [2]int{0, 2}, [2]int{2, 4})
	p, v := views[0], views[1]
	p.SetSelectionMap(map[tid.TID]bool{"d": true})
	frame := v.RowFrame(3)
	c.Equal(v.PointTo(geom.NewPoint(frame.X, frame.Bottom()), p.AsPanel()), p.ContextMenuAnchor())
	p.SetSelectionMap(map[tid.TID]bool{"f": true})
	c.Equal(p.DefaultContextMenuAnchor(), p.ContextMenuAnchor(), "no view shows the row")
}

// TestTableAccessibilityContextMenuOpensWhereTheFocusMoveLeftTheRow verifies that a contextual menu an assistive
// technology asks for opens beneath the row or cell in the view showing it once the view that showed it has taken the
// focus, when the panel losing the focus moved the row to another view or cut the columns back.
func TestTableAccessibilityContextMenuOpensWhereTheFocusMoveLeftTheRow(t *testing.T) {
	for _, test := range []struct {
		lostFocus func(f *focusCellFixture)
		frame     func(views []*Table[*focusCellRow]) geom.Rect
		key       any
		name      string
		view      int // The view asked for its menu
	}{
		{
			// As a field committing on losing the focus might.
			name: "row moved into the view losing the focus",
			lostFocus: func(f *focusCellFixture) {
				f.model.SetRootRows([]*focusCellRow{f.rows[3], f.rows[0], f.rows[1], f.rows[2]})
				f.table.SyncToModel()
			},
			key:   tid.TID("d"),
			frame: func(views []*Table[*focusCellRow]) geom.Rect { return views[0].RowFrame(0) },
			view:  0,
		},
		{
			name:      "cell's column cut away",
			lostFocus: func(f *focusCellFixture) { f.table.Columns = f.table.Columns[:2] },
			key:       accessibility.CellKey{Row: "d", Col: 2},
			frame:     func(views []*Table[*focusCellRow]) geom.Rect { return views[1].CellFrame(3, 1) },
			view:      1,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := check.New(t)
			f, views := newTableViewsFixture(t, fourTableViewIDs, 0, [2]int{0, 2}, [2]int{2, 4})
			activateForContextMenu(t, f.w)
			asked := make(map[int]geom.Point)
			for i, v := range views {
				v.ContextMenuCallback = func(where geom.Point) Menu {
					asked[i] = where
					return nil
				}
			}
			f.table.LostFocusCallback = func() { test.lostFocus(f) }
			f.table.PerformAccessibilityAction(accessibility.ActionRequest{
				Key:    test.key,
				Action: accessibility.ShowContextMenu,
			})
			frame := test.frame(views)
			c.Equal(map[int]geom.Point{test.view: geom.NewPoint(frame.X, frame.Bottom())}, asked,
				"only view %d was asked for its menu, beneath the row or cell where it is now", test.view)
		})
	}
}

// TestTableViewWithoutAHeaderUsesAnotherViewsHeader verifies that a view with no header of its own falls back on the
// header of another view of the table, both for the titles its rows are named with and for the sort a change to the
// filter keeps.
func TestTableViewWithoutAHeaderUsesAnotherViewsHeader(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, []string{"b", "d", "a", "c"}, 0, [2]int{0, 2}, [2]int{2, 4})
	first := NewTableColumnHeader[*focusCellRow]("Name", "", nil)
	first.SetSortState(SortState{Order: 0, Ascending: true, Sortable: true})
	NewTableHeader(f.table, first, NewTableColumnHeader[*focusCellRow]("Size", "", nil),
		NewTableColumnHeader[*focusCellRow]("Kind", "", nil))
	v := views[1]
	c.Contains(v.axRowName(v.RowFromIndex(3), v.axColumnTitles()), "Size", "the row is named with the column titles")
	v.ApplyFilter(func(*focusCellRow) bool { return false })
	ids := make([]tid.TID, 0, 4)
	for i := range v.LastRowIndex() + 1 {
		ids = append(ids, v.RowFromIndex(i).ID())
	}
	c.Equal([]tid.TID{"a", "b", "c", "d"}, ids, "the filtered rows are sorted")
}

// TestTableScrollRowIntoViewScrollsTheViewShowingTheRow verifies that bringing a row or one of its cells into view,
// whichever view is asked, scrolls to it in the view showing the row in the window of the view asked.
func TestTableScrollRowIntoViewScrollsTheViewShowingTheRow(t *testing.T) {
	c := check.New(t)
	f, views := newTableViewsFixture(t, fourTableViewIDs, 0, [2]int{0, 2}, [2]int{2, 4})
	var scrolled []geom.Rect
	f.w.Content().ScrollRectIntoViewCallback = func(rect geom.Rect) bool {
		scrolled = append(scrolled, rect)
		return true
	}
	views[0].ScrollRowIntoView(3)
	views[0].ScrollRowCellIntoView(3, 1)
	v := views[1]
	c.Equal([]geom.Rect{
		v.RectTo(v.RowFrame(3), f.w.Content()),
		v.RectTo(v.CellFrame(3, 1), f.w.Content()),
	}, scrolled)

	// A view of another window showing the row first is not the one scrolled.
	f, views = newTableViewsFixture(t, fourTableViewIDs, 0, [2]int{0, 4}, [2]int{0, 4})
	wnd := newRedrawTestWindow()
	wnd.Content().AddChild(views[1])
	scrolled = nil
	var scrolledThere []geom.Rect
	f.w.Content().ScrollRectIntoViewCallback = func(rect geom.Rect) bool {
		scrolled = append(scrolled, rect)
		return true
	}
	wnd.Content().ScrollRectIntoViewCallback = func(rect geom.Rect) bool {
		scrolledThere = append(scrolledThere, rect)
		return true
	}
	v = views[1]
	v.ScrollRowIntoView(3)
	v.ScrollRowCellIntoView(3, 1)
	c.Equal(0, len(scrolled), "the other window was not scrolled")
	c.Equal([]geom.Rect{
		v.RectTo(v.RowFrame(3), wnd.Content()),
		v.RectTo(v.CellFrame(3, 1), wnd.Content()),
	}, scrolledThere)
}

// TestTableMouseExitIsDeliveredWhereverTheRowWent verifies that the pointer leaving a cell whose row has passed to
// another view since the pointer entered it tells the cell so with the cell in the view now showing it, leaving a
// focused cell where it lives, and tells it so through the view the pointer was in when no view shows the row now. A
// view of another window showing the row first is passed over. Rows re-sorted or removed under the pointer are followed
// too: the exit goes to the cell of the row the pointer was over, installed where that row is now or nowhere once it is
// gone, never to the row that took its index; the pointer moving on to where the row went is no exit; and the table
// taking the focus meanwhile scrolls to where the row went.
func TestTableMouseExitIsDeliveredWhereverTheRowWent(t *testing.T) {
	chk := check.New(t)
	vf, views := newTableViewsFixture(t, sixTableViewIDs, 2, [2]int{0, 2}, [2]int{2, 4}, [2]int{4, 6})
	chk.True(vf.table.FocusCell(3, 0))
	focused := vf.rows[3].cells[0]
	focusedExits := 0
	focused.MouseExitCallback = func() bool {
		focusedExits++
		return true
	}
	views[1].DefaultMouseEnter(views[1].CellFrame(3, 0).Center(), 0)
	views[1].SetDrawRowRange(2, 3)
	views[2].SetDrawRowRange(3, 6)
	views[1].DefaultMouseExit()
	chk.Equal(1, focusedExits, "the cell was told the pointer left it")
	chk.True(focused.Parent() == views[2].AsPanel(), "the focused cell is in the view now showing its row")
	chk.Equal(views[2].CellFrame(3, 0), focused.FrameRect())

	views[2].DefaultMouseEnter(views[2].CellFrame(4, 1).Center(), 0)
	unshownExits := 0
	vf.rows[4].cells[1].MouseExitCallback = func() bool {
		unshownExits++
		return true
	}
	views[2].SetDrawRowRange(3, 4)
	views[2].DefaultMouseExit()
	chk.Equal(1, unshownExits, "the cell no view shows now was told the pointer left it")

	vf, views = newTableViewsFixture(t, fourTableViewIDs, 2, [2]int{0, 4}, [2]int{0, 4})
	wnd := newRedrawTestWindow()
	wnd.Content().AddChild(views[1])
	views[1].DefaultMouseEnter(views[1].CellFrame(3, 1).Center(), 0)
	var exitWnd *Window
	vf.rows[3].cells[1].MouseExitCallback = func() bool {
		exitWnd = vf.rows[3].cells[1].Window()
		return true
	}
	views[1].DefaultMouseExit()
	chk.True(exitWnd == wnd, "the cell was told the pointer left it in the window the pointer was in")

	for _, tc := range []struct {
		name  string
		order []int // Indexes into the original rows of the rows left, in their new order
		move  int   // The row the pointer moves on to, or -1 for the pointer leaving the table
		exits int
	}{
		{name: "row moved and pointer leaves", order: []int{0, 2, 1}, move: -1, exits: 1},
		{name: "row moved and pointer stays", order: []int{0, 2, 1}, move: 1, exits: 1},
		{name: "row moved and pointer follows it", order: []int{0, 2, 1}, move: 2},
		{name: "row removed and pointer leaves", order: []int{0, 2}, move: -1, exits: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
			hovered := f.rows[1]
			cell := hovered.ColumnCell(1, 0, nil, nil, false, false, false).AsPanel()
			exits := 0
			cell.MouseExitCallback = func() bool {
				exits++
				if now := f.table.RowToIndex(hovered); now == -1 {
					c.Nil(cell.Parent(), "the cell of a row that is gone is not installed")
				} else {
					c.True(cell.Parent() == f.table.AsPanel(), "the cell is installed")
					c.Equal(f.table.CellFrame(now, 0), cell.FrameRect(), "the cell is where its row is now")
				}
				for _, r := range f.rows {
					if r != hovered && r.cells[0] != nil {
						c.Nil(r.cells[0].Parent(), "no other row's cell is installed")
					}
				}
				return true
			}
			f.table.DefaultMouseEnter(f.table.CellFrame(1, 0).Center(), 0)
			rows := make([]*focusCellRow, 0, len(tc.order))
			for _, i := range tc.order {
				rows = append(rows, f.rows[i])
			}
			f.model.SetRootRows(rows)
			f.table.SyncToModel()
			var scrolled, want []geom.Rect
			f.w.Content().ScrollRectIntoViewCallback = func(rect geom.Rect) bool {
				scrolled = append(scrolled, rect)
				return true
			}
			f.table.DefaultFocusGained()
			if now := f.table.RowToIndex(hovered); now != -1 {
				want = append(want, f.table.RectTo(f.table.RowFrame(now), f.w.Content()))
			}
			c.Equal(want, scrolled, "the table taking the focus scrolls to where the row went")
			if tc.move == -1 {
				f.table.DefaultMouseExit()
			} else {
				f.table.DefaultMouseMove(f.table.CellFrame(tc.move, 0).Center(), 0)
			}
			c.Equal(tc.exits, exits)
		})
	}
}

// TestTableHoverWithinACellEntersThePanelInstalled verifies that the pointer moving from one panel of a cell to another
// enters the second with the cell installed, once the first has been told the pointer left it.
func TestTableHoverWithinACellEntersThePanelInstalled(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b"}, 0)
	cell := f.rows[0].ColumnCell(0, 0, nil, nil, false, false, false).AsPanel()
	frame := f.table.CellFrame(0, 0)
	left := NewPanel()
	left.SetFrameRect(geom.NewRect(0, 0, frame.Width/2, frame.Height))
	right := NewPanel()
	right.SetFrameRect(geom.NewRect(frame.Width/2, 0, frame.Width/2, frame.Height))
	cell.AddChild(left)
	cell.AddChild(right)
	exits := 0
	left.MouseExitCallback = func() bool {
		exits++
		return true
	}
	var enteredIn *Window
	right.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
		enteredIn = right.Window()
		return true
	}
	f.table.DefaultMouseMove(frame.Point.Add(geom.NewPoint(frame.Width/4, frame.Height/2)), 0)
	f.table.DefaultMouseMove(frame.Point.Add(geom.NewPoint(frame.Width*3/4, frame.Height/2)), 0)
	c.Equal(1, exits, "the panel the pointer left was told so")
	c.True(enteredIn == f.w, "the panel the pointer moved on to was entered with its cell installed")
	c.Nil(cell.Parent(), "the cell is detached afterwards")
}

// TestTableCellHoverThatChangesTheRowsActsOnTheHoveredRow verifies that a cell's enter, move, cursor or tooltip
// callback that moves the rows and takes the focus leaves the focused cell on its own row wherever that row went, that
// a move goes only to the cell the pointer entered, so to none once the enter has changed what is under the pointer,
// and that the cell the pointer moves on to is the one under it once the exit from the cell it left has changed the
// rows.
func TestTableCellHoverThatChangesTheRowsActsOnTheHoveredRow(t *testing.T) {
	// The callback of the hovered cell that changes the rows.
	type hoverEvent int
	const (
		onEnter hoverEvent = iota
		onMove
		onCursor
		onTooltip
		onExit
	)
	for _, tc := range []struct {
		name     string
		order    []int // Indexes into the original rows of the rows the callback leaves, in their new order
		moves    []tid.TID
		ranges   [][2]int // The row ranges of the table's views, when it has more than one
		event    hoverEvent
		hover    int
		to       int // The row the pointer moves on to from the hovered one, for an exit
		focusRow int
		focus    bool // The callback also takes the focus into the cell
		// focusFirst makes the callback take the focus before it changes the rows rather than after.
		focusFirst bool
	}{
		{name: "enter takes the focus", event: onEnter, order: []int{2, 1, 0}, focusRow: 2, focus: true},
		{
			name:     "move takes the focus",
			event:    onMove,
			order:    []int{2, 1, 0},
			moves:    []tid.TID{"a"},
			focusRow: 2,
			focus:    true,
		},
		{name: "cursor takes the focus", event: onCursor, order: []int{2, 1, 0}, focusRow: 2, focus: true},
		{name: "tooltip takes the focus", event: onTooltip, order: []int{2, 1, 0}, focusRow: 2, focus: true},
		{
			name:       "tooltip takes the focus and moves its row into another view",
			event:      onTooltip,
			order:      []int{0, 2, 1},
			ranges:     [][2]int{{0, 2}, {2, 3}},
			hover:      1,
			focusRow:   2,
			focus:      true,
			focusFirst: true,
		},
		{name: "enter moves another row under the pointer", event: onEnter, order: []int{2, 1, 0}, focusRow: -1},
		{name: "enter removes the row under the pointer", event: onEnter, order: []int{0, 1}, hover: 2, focusRow: -1},
		{
			name:     "exit removes the row the pointer moves on to",
			event:    onExit,
			order:    []int{0},
			moves:    []tid.TID{"a"},
			to:       2,
			focusRow: -1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			var recovered error
			withRecoveryCallback(t, func(err error) { recovered = err })
			var f *focusCellFixture
			if tc.ranges != nil {
				f, _ = newTableViewsFixture(t, []string{"a", "b", "c"}, 1, tc.ranges...)
			} else {
				f = newFocusCellFixture(t, []string{"a", "b", "c"}, 1)
			}
			var moves []tid.TID
			for i, r := range f.rows {
				p := r.ColumnCell(i, 0, nil, nil, false, false, false).AsPanel()
				p.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool {
					moves = append(moves, r.id)
					return true
				}
			}
			cell := f.rows[tc.hover].cells[0]
			change := func() {
				if tc.focus && tc.focusFirst {
					cell.RequestFocus()
				}
				rows := make([]*focusCellRow, 0, len(tc.order))
				for _, i := range tc.order {
					rows = append(rows, f.rows[i])
				}
				f.model.SetRootRows(rows)
				f.table.SyncToModel()
				if tc.focus && !tc.focusFirst {
					cell.RequestFocus()
				}
			}
			switch tc.event {
			case onEnter:
				cell.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
					change()
					return true
				}
			case onMove:
				cell.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool {
					moves = append(moves, f.rows[tc.hover].id)
					change()
					return true
				}
			case onCursor:
				cell.UpdateCursorCallback = func(_ geom.Point) *Cursor {
					change()
					return nil
				}
			case onTooltip:
				cell.UpdateTooltipCallback = func(_ geom.Point, avoid geom.Rect) geom.Rect {
					change()
					return avoid
				}
			case onExit:
				cell.MouseExitCallback = func() bool {
					change()
					return true
				}
			}
			where := f.table.CellFrame(tc.hover, 0).Center()
			to := f.table.CellFrame(tc.to, 0).Center()
			SafeCall(func() {
				switch tc.event {
				case onCursor:
					f.table.DefaultUpdateCursorCallback(where)
				case onTooltip:
					f.table.DefaultUpdateTooltipCallback(where, geom.Rect{})
				case onExit:
					f.table.DefaultMouseMove(where, 0)
					f.table.DefaultMouseMove(to, 0)
				default:
					f.table.DefaultMouseMove(where, 0)
				}
			})
			c.NoError(recovered)
			row, _ := f.table.FocusedCell()
			c.Equal(tc.focusRow, row, "the focused cell is on the hovered row")
			c.Equal(tc.moves, moves)
		})
	}
}

// TestTableChangesThroughADetachedPrimaryRedrawTheViews verifies that a change made through a primary view that is in
// no window still redraws the views that are.
func TestTableChangesThroughADetachedPrimaryRedrawTheViews(t *testing.T) {
	c := check.New(t)
	f, _ := newTableViewsFixture(t, fourTableViewIDs, 0, [2]int{0, 2}, [2]int{2, 4})
	f.table.RemoveFromParent()
	clear(redrawSet)
	f.table.SelectByIndex(3)
	_, pending := redrawSet[f.w]
	c.True(pending, "the window holding the other view was marked for redraw")
}
