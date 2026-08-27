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
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
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
// scheduled, since the global queue is shared: a timer left running by an earlier test, such as the caret blink a Field
// keeps rearming via InvokeTaskAfter, can enqueue a task at any moment. Those foreign tasks are harmless to run here,
// so they are simply executed and skipped over. These tests share the global task queue and therefore must not call
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
	c.True(f.w.focus == cell, "the press must leave the focus on the cell that asked for it")
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
	c.True(f.w.focus == cell)

	// uninstallCell() is the one place that detaches cells, and it has to make an exception for the focused one.
	f.table.uninstallCell(cell, 1, 0)
	c.True(cell.Parent() == f.table.AsPanel(), "uninstalling the focused cell must not detach it")

	// Pressing the cell that already has the focus must not bounce the focus out to the table and back again, which a
	// widget would see as a fresh click into itself rather than a click that just moves the caret.
	f.pressCell(1, 0)
	c.True(f.w.focus == cell)
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
		c.True(f.w.focus == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[0])
	})

	t.Run("column removed", func(t *testing.T) {
		c := check.New(t)
		f := newFocusCellFixture(t, []string{"a", "b", "c"}, 2)
		f.pressCell(1, 1)
		cell := f.rows[1].cells[1]
		c.True(f.w.focus == cell)
		f.table.Columns = f.table.Columns[:1]
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.focus == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[1])
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
		c.True(f.w.focus == cell, "the child row's cell should have taken the focus")
		f.rows[0].SetOpen(false)
		f.table.SyncToModel()
		row, col := f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.Nil(cell.Parent())
		c.True(f.w.focus == f.table.AsPanel())
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
		c.True(f.w.focus == f.table.AsPanel())
		c.Equal(1, f.rows[1].lost[0])

		// Dropping the filter brings the row back, but the focus stays where it was put.
		f.table.ApplyFilter(nil)
		row, col = f.table.FocusedCell()
		c.Equal(-1, row)
		c.Equal(-1, col)
		c.True(f.w.focus == f.table.AsPanel())
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
	c.True(f.w.focus == cell, "the same panel must still hold the focus")
	c.True(cell.Parent() == f.table.AsPanel())
	c.Equal(0, f.rows[1].lost[0], "shifting the row must not end the editing")
	c.Equal(f.table.CellFrame(2, 0), cell.FrameRect())

	// Taking the inserted row back out shifts the focused cell back to where it started.
	f.model.SetRootRows(f.rows)
	f.table.SyncToModel()
	row, col = f.table.FocusedCell()
	c.Equal(1, row)
	c.Equal(0, col)
	c.True(f.w.focus == cell)
	c.Equal(0, f.rows[1].lost[0])
	c.Equal(f.table.CellFrame(1, 0), cell.FrameRect())
}

// TestTableFocusCellAPI verifies the programmatic entry point: which arguments it accepts, that it hands the focus over
// from one cell to the next, and that giving the focus back to the table releases the cell.
func TestTableFocusCellAPI(t *testing.T) {
	c := check.New(t)
	f := newFocusCellFixture(t, []string{"a", "b"}, 2)
	c.True(f.table.FocusCell(0, 0))
	c.True(f.w.focus == f.rows[0].cells[0])
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
	c.True(f.w.focus == f.rows[0].cells[0])
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
	c.True(f.w.focus == other)
	row, col := f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(cell.Parent())
	c.True(f.w.focus == other, "releasing the cell must not steal the focus back")

	// Removing the focus entirely is the other way the cell can lose it.
	f.pressCell(0, 0)
	c.True(f.w.focus == cell)
	c.Equal(2, f.rows[0].gained[0])
	f.w.SetFocus(nil)
	c.Equal(2, f.rows[0].lost[0])
	row, col = f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(cell.Parent())
	c.Nil(f.w.focus, "the window must be left with no focus at all")
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
	// something that is being edited would be wrong, so they are ignored and the cell keeps the focus.
	f.pressCell(0, 0)
	cell := f.rows[0].cells[0]
	c.True(f.w.focus == cell)
	f.w.keyPressed(KeyDown, 0)
	f.w.keyPressed(KeyUp, 0)
	c.Equal(1, f.table.SelectionCount())
	c.True(f.table.IsRowSelected(1), "the selection must not move while a cell is focused")
	c.True(f.w.focus == cell)

	// Tab moves to the next focusable cell, in row-major order.
	f.w.keyPressed(KeyTab, 0)
	row, col := f.table.FocusedCell()
	c.Equal(0, row)
	c.Equal(1, col)
	c.True(f.w.focus == f.rows[0].cells[1])
	c.Equal(1, f.rows[0].lost[0])
	c.Equal(1, f.rows[0].gained[1])
	c.Nil(cell.Parent(), "the cell that was left behind must be detached again")

	// Tab from the last focusable cell of the last row leaves the table, landing on the panel that follows it.
	c.True(f.table.FocusCell(2, 1))
	f.w.keyPressed(KeyTab, 0)
	c.True(f.w.focus == after)
	row, col = f.table.FocusedCell()
	c.Equal(-1, row)
	c.Equal(-1, col)
	c.Nil(f.rows[2].cells[1].Parent())
	c.Equal(1, f.rows[2].lost[1])

	// Return and Escape end the editing by handing the focus back to the table, and neither of them may reach the
	// table's own Space-to-DoubleClickCallback shortcut.
	for _, key := range []KeyCode{KeyReturn, KeyEscape} {
		f.pressCell(1, 0)
		c.True(f.w.focus == f.rows[1].cells[0])
		f.w.keyPressed(key, 0)
		c.True(f.w.focus == f.table.AsPanel())
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
	c.True(f.w.focus == f.rows[1].cells[0], "an unwanted key must not cost the cell its focus")

	// Sanity check that the table does still run its DoubleClickCallback for a Space when it holds the focus itself.
	f.w.SetFocus(f.table)
	f.w.keyPressed(KeySpace, 0)
	c.Equal(1, doubleClicks)
}
