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
	"github.com/richardwilkes/toolbox/v2/tid"
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
