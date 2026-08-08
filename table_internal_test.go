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

// runPendingTask waits for the task InvokeTaskAfter scheduled to reach the queue, then runs it through the same
// recovery path the UI loop uses. These tests share the global task queue and therefore must not call t.Parallel.
func runPendingTask(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if length, head := taskQueueState(); length > head {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the scheduled task")
		}
		time.Sleep(time.Millisecond)
	}
	processNextTask()
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
	runPendingTask(t)
	c.NotNil(recovered, "the scheduled sync was expected to panic")
	c.False(table.awaitingSyncToModel, "a panicking sync must still clear the pending flag")

	// A later request must actually do the work rather than being dropped.
	boom = false
	recovered = nil
	before := cellCalls
	table.EventuallySyncToModel()
	runPendingTask(t)
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
	runPendingTask(t)
	c.NotNil(recovered, "the scheduled sizing was expected to panic")
	c.False(table.awaitingSizeColumnsToFit, "a panicking sizing pass must still clear the pending flag")

	boom = false
	recovered = nil
	before := cellCalls
	table.EventuallySizeColumnsToFit(false)
	runPendingTask(t)
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
