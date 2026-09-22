// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"image"
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
)

// selectedTableIndexes returns the indexes of the selected rows, in order.
func selectedTableIndexes(table *unison.Table[*tableTestRow]) []int {
	var indexes []int
	for i := range table.LastRowIndex() + 1 {
		if table.IsRowSelected(i) {
			indexes = append(indexes, i)
		}
	}
	return indexes
}

// TestTableKeyNavigationIgnoresUnrecognizedModifiers verifies that the arrow, home and end keys only navigate when
// they carry the modifiers the table gives a meaning to. A navigation key with any other modifier is typically a menu
// shortcut that the menu declined because its command was disabled, and treating it as a plain key press moved the
// selection when the user expected nothing at all to happen. Such a key is reported as unhandled, so that whatever is
// above the table gets a look at it.
func TestTableKeyNavigationIgnoresUnrecognizedModifiers(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(4)...)
	table.SelectByIndex(2)

	for _, mods := range []mod.Modifiers{
		mod.OSMenuCommand(),
		mod.Shift | mod.OSMenuCommand(),
		mod.Option,
		mod.Shift | mod.Option,
	} {
		for _, key := range []unison.KeyCode{unison.KeyUp, unison.KeyDown, unison.KeyHome, unison.KeyEnd} {
			c.False(table.DefaultKeyDown(key, mods, false), "%v with %v must be reported as unhandled", key, mods)
			c.Equal([]int{2}, selectedTableIndexes(table), "%v with %v must leave the selection alone", key, mods)
		}
	}

	// The modifiers the table does recognize keep working.
	c.True(table.DefaultKeyDown(unison.KeyUp, 0, false), "an unmodified Up must be handled")
	c.Equal([]int{1}, selectedTableIndexes(table), "an unmodified Up must move the selection up")
	c.True(table.DefaultKeyDown(unison.KeyDown, mod.Shift, false), "a shifted Down must be handled")
	c.Equal([]int{1, 2}, selectedTableIndexes(table), "a shifted Down must extend the selection")
	c.True(table.DefaultKeyDown(unison.KeyEnd, 0, false), "an unmodified End must be handled")
	c.Equal([]int{3}, selectedTableIndexes(table), "an unmodified End must select the last row")
	c.True(table.DefaultKeyDown(unison.KeyHome, mod.Shift, false), "a shifted Home must be handled")
	c.Equal([]int{0, 1, 2, 3}, selectedTableIndexes(table), "a shifted Home must extend the selection to the first row")
}

// TestTableDisclosureKeysIgnoreUnrecognizedModifiers does the same for the left and right arrows, which close and open
// containers and recognize only the option modifier, for doing so recursively.
func TestTableDisclosureKeysIgnoreUnrecognizedModifiers(t *testing.T) {
	c := check.New(t)
	inner := newTableTestRow("inner")
	inner.SetChildren([]*tableTestRow{newTableTestRow("leaf")})
	outer := newTableTestRow("outer")
	outer.SetChildren([]*tableTestRow{inner})
	table := newTestTable(outer)
	table.SelectByIndex(0)
	c.False(outer.IsOpen(), "the container starts out closed")

	for _, mods := range []mod.Modifiers{
		mod.OSMenuCommand(),
		mod.Shift | mod.OSMenuCommand(),
		mod.Shift,
		mod.Option | mod.OSMenuCommand(),
	} {
		c.False(table.DefaultKeyDown(unison.KeyRight, mods, false), "Right with %v must be reported as unhandled", mods)
		c.False(outer.IsOpen(), "Right with %v must not open the container", mods)
	}

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false), "an unmodified Right must be handled")
	c.True(outer.IsOpen(), "an unmodified Right must open the container")
	c.False(inner.IsOpen(), "an unmodified Right must open only the selected container")
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false), "an unmodified Left must be handled")
	c.False(outer.IsOpen(), "an unmodified Left must close the container")
	c.True(table.DefaultKeyDown(unison.KeyRight, mod.Option, false), "an option-modified Right must be handled")
	c.True(outer.IsOpen() && inner.IsOpen(), "an option-modified Right must open the container and its descendants")
	c.False(table.DefaultKeyDown(unison.KeyLeft, mod.OSMenuCommand(), false),
		"Left with the command modifier must be reported as unhandled")
	c.True(outer.IsOpen() && inner.IsOpen(), "Left with the command modifier must not close anything")
}

// TestTableDisclosureKeysAreInertUnderHierarchicalFilter verifies that the left and right arrows leave the open states
// alone while a hierarchical filter is applied, since the filter shows every container it kept as open regardless.
func TestTableDisclosureKeysAreInertUnderHierarchicalFilter(t *testing.T) {
	c := check.New(t)
	inner := newTableTestRow("inner")
	inner.SetChildren([]*tableTestRow{newTableTestRow("leaf")})
	outer := newTableTestRow("outer")
	outer.SetChildren([]*tableTestRow{inner})
	table := newTestTable(outer)
	table.ApplyHierarchicalFilter(func(row *tableTestRow) bool { return row.ID() != "leaf" })
	c.Equal(3, table.LastRowIndex()+1, "the leaf and the containers above it must be shown")
	table.SelectByIndex(0)

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false), "Right is still handled")
	c.False(outer.IsOpen(), "Right must not open the container while the filter is applied")
	c.True(table.DefaultKeyDown(unison.KeyRight, mod.Option, false), "an option-modified Right is still handled")
	c.False(outer.IsOpen() || inner.IsOpen(), "an option-modified Right must not open anything either")
	outer.SetOpen(true)
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false), "Left is still handled")
	c.True(outer.IsOpen(), "Left must not close the container while the filter is applied")
	c.Equal(3, table.LastRowIndex()+1, "the view must be unchanged throughout")

	table.ApplyFilter(nil)
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false))
	c.False(outer.IsOpen(), "Left must close the container again once the filter is gone")
}

// newCursorTable returns a table of the given rows with three columns, for the tests that drive the cell cursor. The
// rows name each of their columns, so that what the cursor is standing on can be told from what it reads as.
func newCursorTable(rows ...*tableTestRow) *unison.Table[*tableTestRow] {
	table := newTestTable(rows...)
	table.Columns = []unison.ColumnInfo{
		{ID: 0, Current: 60},
		{ID: 1, Current: 60},
		{ID: 2, Current: 60},
	}
	table.SyncToModel()
	return table
}

// TestTableCellCursorHorizontalKeys verifies the keys that move the cell cursor across a row: Right on a row with
// nothing left to open moves into the cells, Right moves on to the next column and stops at the last one, and Left
// moves back and steps out onto the row again from the first column.
func TestTableCellCursorHorizontalKeys(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(3)...)
	table.SelectByIndex(1)
	c.Equal(-1, table.LeadColumnIndex(), "a table starts out at row level")

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex(), "Right on a row with nothing to open moves into its cells")
	c.Equal(1, table.LeadRowIndex(), "the cursor stands on the row the person was already on")
	c.Equal([]int{1}, selectedTableIndexes(table), "moving across a row does not change what is selected")

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(1, table.LeadColumnIndex())
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(2, table.LeadColumnIndex())
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(2, table.LeadColumnIndex(), "the last column is the end of the row; there is no wrapping or leaving")

	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false))
	c.Equal(1, table.LeadColumnIndex())
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false))
	c.Equal(0, table.LeadColumnIndex())
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false))
	c.Equal(-1, table.LeadColumnIndex(), "Left off the first column puts the person back on the row")
	c.Equal(1, table.LeadRowIndex())
	c.Equal([]int{1}, selectedTableIndexes(table))
}

// TestTableCellCursorAndContainers verifies that the keys that open and close containers keep doing so, and that the
// cells are what Right reaches once there is nothing left to open.
func TestTableCellCursorAndContainers(t *testing.T) {
	c := check.New(t)
	container := newTableTestRow("container")
	container.SetChildren([]*tableTestRow{newTableTestRow("leaf")})
	table := newCursorTable(container)
	table.SelectByIndex(0)

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(container.IsOpen(), "Right on a closed container still opens it")
	c.Equal(-1, table.LeadColumnIndex(), "opening a container is not entering its cells")

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex(), "Right on a container that is already open moves into its cells")
	c.True(container.IsOpen())

	// From cell level, Left walks back out to the row before the container closes again.
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false))
	c.Equal(-1, table.LeadColumnIndex())
	c.True(container.IsOpen(), "the Left that left the cells must not also close the container")
	c.True(table.DefaultKeyDown(unison.KeyLeft, 0, false))
	c.False(container.IsOpen(), "Left at row level closes the container")

	// The recursive form never enters the cells, whatever there is left to open.
	container.SetOpen(true)
	table.SyncToModel()
	table.SelectByIndex(0)
	c.True(table.DefaultKeyDown(unison.KeyRight, mod.Option, false))
	c.Equal(-1, table.LeadColumnIndex(), "an option-modified Right works on the hierarchy, not on the cells")
}

// TestTableCellCursorRightOpensBeforeItEnters verifies that a Right which opened a container anywhere in the selection
// does not also step into the cells of the row the person is on: what they saw happen was a container opening, and the
// cells are for the next Right.
func TestTableCellCursorRightOpensBeforeItEnters(t *testing.T) {
	c := check.New(t)
	leaf := newTableTestRow("leaf")
	container := newTableTestRow("container")
	container.SetChildren([]*tableTestRow{newTableTestRow("child")})
	table := newCursorTable(leaf, container)
	table.SelectByIndex(1, 0) // The leaf is selected last, so it is the row the person is on.
	c.Equal(0, table.LeadRowIndex())

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(container.IsOpen(), "the selected container opens")
	c.Equal(-1, table.LeadColumnIndex(), "and the Right that opened it does not enter the leaf's cells as well")

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex(), "with nothing left to open, the next Right enters the cells")
}

// TestTableSetLeadCellNotifiesOnce verifies that SetLeadCell reports one selection change when the row changes and
// none when the cursor merely moves within the row that was already the whole of the selection, which is what an
// assistive technology stepping across the cells asks for.
func TestTableSetLeadCellNotifiesOnce(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(3)...)
	table.SelectByIndex(0)
	changes := 0
	table.SelectionChangedCallback = func() { changes++ }

	c.True(table.SetLeadCell(2, 1))
	c.Equal(1, changes, "moving onto another row is one selection change")

	c.True(table.SetLeadCell(2, 2))
	c.True(table.SetLeadCell(2, -1))
	c.Equal(1, changes, "moving within the row that is already the selection is not a selection change at all")
	c.Equal(-1, table.LeadColumnIndex())
	c.Equal(2, table.LeadRowIndex())
}

// TestTableCellCursorVerticalKeysKeepTheColumn verifies that the keys that move through the rows keep the column the
// cursor is in, which is what lets a person read down a column, and that Home and End move the cursor across the row
// unmodified but go on moving through the rows when shifted.
func TestTableCellCursorVerticalKeysKeepTheColumn(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(4)...)
	table.SelectByIndex(1)
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(1, table.LeadColumnIndex())

	c.True(table.DefaultKeyDown(unison.KeyDown, 0, false))
	c.Equal(2, table.LeadRowIndex())
	c.Equal(1, table.LeadColumnIndex(), "moving down a row keeps the person in the column they were reading")
	c.Equal([]int{2}, selectedTableIndexes(table))

	c.True(table.DefaultKeyDown(unison.KeyUp, 0, false))
	c.Equal(1, table.LeadRowIndex())
	c.Equal(1, table.LeadColumnIndex())

	c.True(table.DefaultKeyDown(unison.KeyDown, mod.Shift, false))
	c.Equal([]int{1, 2}, selectedTableIndexes(table), "a shifted Down still extends the selection")
	c.Equal(1, table.LeadColumnIndex())

	// Unmodified Home and End move across the row at cell level.
	c.True(table.DefaultKeyDown(unison.KeyEnd, 0, false))
	c.Equal(2, table.LeadColumnIndex(), "End at cell level goes to the last column")
	c.Equal(2, table.LeadRowIndex(), "and leaves the rows alone")
	c.True(table.DefaultKeyDown(unison.KeyHome, 0, false))
	c.Equal(0, table.LeadColumnIndex(), "Home at cell level goes to the first column")
	c.Equal(2, table.LeadRowIndex())
	c.Equal([]int{1, 2}, selectedTableIndexes(table), "neither touches the selection")

	// Shifted, they keep their row-range meaning and the cursor stays in its column.
	c.True(table.DefaultKeyDown(unison.KeyHome, mod.Shift, false))
	c.Equal([]int{0, 1, 2}, selectedTableIndexes(table))
	c.Equal(0, table.LeadRowIndex())
	c.Equal(0, table.LeadColumnIndex())
}

// TestTableCellCursorEscapeReturnsToRowLevel verifies that Escape puts the person back on the row, which is the way
// out of the cells that does not depend on which column they are in.
func TestTableCellCursorEscapeReturnsToRowLevel(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(2)...)
	table.SelectByIndex(0)
	c.False(table.DefaultKeyDown(unison.KeyEscape, 0, false), "Escape at row level is not the table's to take")

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(1, table.LeadColumnIndex())
	c.True(table.DefaultKeyDown(unison.KeyEscape, 0, false))
	c.Equal(-1, table.LeadColumnIndex(), "Escape leaves the cells from wherever the cursor was")
	c.Equal(0, table.LeadRowIndex())
	c.Equal([]int{0}, selectedTableIndexes(table))
}

// TestTableCellCursorNeedsARowToStandOn verifies that the cursor is given up whenever the row beneath it goes: the
// selection being cleared, the row being filtered away, or the columns it named being taken from the table.
func TestTableCellCursorNeedsARowToStandOn(t *testing.T) {
	c := check.New(t)
	rows := flatRows(3)
	table := newCursorTable(rows...)
	table.SelectByIndex(1)
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex())

	table.ClearSelection()
	c.Equal(-1, table.LeadColumnIndex(), "a cursor cannot stand on a row that is no longer selected")

	// A selection made from code puts the person on the row rather than into the cells of it.
	table.SelectByIndex(2)
	c.Equal(-1, table.LeadColumnIndex())
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex())
	table.SelectRange(0, 1)
	c.Equal(-1, table.LeadColumnIndex())

	// A row the table stops showing takes the cursor with it.
	table.SelectByIndex(0)
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex())
	table.ApplyFilter(func(row *tableTestRow) bool { return row.ID() == "r0" })
	c.Equal(-1, table.LeadColumnIndex())
	table.ApplyFilter(nil)

	// Columns taken away clamp the cursor to the last one that is left, and a table with none has nowhere to put it.
	table.SelectByIndex(1)
	c.True(table.DefaultKeyDown(unison.KeyEnd, 0, false))
	c.Equal(-1, table.LeadColumnIndex(), "End at row level is a row key, not a cell one")
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(table.DefaultKeyDown(unison.KeyEnd, 0, false))
	c.Equal(2, table.LeadColumnIndex())
	table.Columns = table.Columns[:2]
	c.Equal(1, table.LeadColumnIndex(), "a cursor past the last column is clamped to it")
	table.Columns = nil
	c.Equal(-1, table.LeadColumnIndex(), "a table with no columns has no cells to stand in")
	c.False(table.DefaultKeyDown(unison.KeyRight, 0, false) && table.LeadColumnIndex() != -1,
		"and Right cannot put the person in one")
}

// TestTableSetLeadCell verifies the programmatic way onto a cell: it selects the row, puts the cursor where it was
// asked for, and refuses anything the table is not showing.
func TestTableSetLeadCell(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(3)...)
	table.SelectByIndex(0)

	c.True(table.SetLeadCell(2, 1))
	c.Equal(2, table.LeadRowIndex())
	c.Equal(1, table.LeadColumnIndex())
	c.Equal([]int{2}, selectedTableIndexes(table), "the cell's row becomes the whole of the selection")

	c.True(table.SetLeadCell(2, -1), "a column of -1 puts the person on the row itself")
	c.Equal(-1, table.LeadColumnIndex())
	c.Equal(2, table.LeadRowIndex())

	c.False(table.SetLeadCell(3, 0), "a row the table is not showing is refused")
	c.False(table.SetLeadCell(-1, 0))
	c.False(table.SetLeadCell(0, 3), "a column the table does not have is refused")
	c.False(table.SetLeadCell(0, -2))
	c.Equal(2, table.LeadRowIndex(), "a refusal changes nothing")
	c.Equal(-1, table.LeadColumnIndex())
}

// TestTableCellCursorMouseWorksAtRowLevel verifies that the mouse leaves the person on a row rather than in one of its
// cells: a click is how the selection is made, and the cells are somewhere the keyboard goes.
func TestTableCellCursorMouseWorksAtRowLevel(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(3)...)
	table.SelectByIndex(0)
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(1, table.LeadColumnIndex())

	// Aimed well inside the second column of the last row, which is a cell the cursor could have stood in.
	frame := table.CellFrame(2, 1)
	where := geom.NewPoint(frame.CenterX(), frame.CenterY())
	table.DefaultMouseDown(where, unison.ButtonLeft, 1, 0)
	table.DefaultMouseUp(where, unison.ButtonLeft, 0)
	c.Equal([]int{2}, selectedTableIndexes(table), "the click selects the row it landed on")
	c.Equal(2, table.LeadRowIndex(), "and puts the person on it")
	c.Equal(-1, table.LeadColumnIndex(), "clicking a cell is not entering the cells")
}

// TestTableCellCursorReturnEditsTheCell verifies the other half of the treegrid pattern: Return on a cell holding a
// widget the keyboard can work in starts an editing session in it, and Escape from that session hands the focus back to
// the table with the person still standing on the cell, one more Escape away from the row.
func TestTableCellCursorReturnEditsTheCell(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.Do(func() {
		e.table.SelectByIndex(1)
		e.table.RequestFocus()
	})

	// Right twice: into the cells of the row, then across to the second field.
	screen.KeyPress(unison.KeyRight, 0)
	screen.KeyPress(unison.KeyRight, 0)
	var col int
	screen.Do(func() { col = e.table.LeadColumnIndex() })
	c.Equal(1, col)

	screen.KeyPress(unison.KeyReturn, 0)
	s := e.snapshot(c, screen, 1, 1)
	c.True(s.fieldFocused, "Return on a cell holding a field starts an editing session in it")
	c.Equal(1, s.focusRow)
	c.Equal(1, s.focusCol)

	screen.KeyPress(unison.KeyEscape, 0)
	s = e.snapshot(c, screen, 1, 1)
	c.True(s.tableFocused, "Escape hands the focus back to the table")
	c.Equal(-1, s.focusCol, "the editing session is over")
	var row int
	screen.Do(func() {
		row = e.table.LeadRowIndex()
		col = e.table.LeadColumnIndex()
	})
	c.Equal(1, row, "the person is left standing on the cell they were editing")
	c.Equal(1, col)

	// And one more Escape takes them back out to the row.
	screen.KeyPress(unison.KeyEscape, 0)
	screen.Do(func() { col = e.table.LeadColumnIndex() })
	c.Equal(-1, col)

	// The last column holds a plain panel with nothing to edit, so Return there falls back to what it does at row
	// level, which is nothing the table itself handles.
	screen.Do(func() { c.True(e.table.SetLeadCell(1, 2)) })
	screen.KeyPress(unison.KeyReturn, 0)
	s = e.snapshot(c, screen, 1, 1)
	c.True(s.tableFocused, "a cell with nothing to edit leaves the focus where it was")
	c.Equal(-1, s.focusCol)
	screen.Do(func() { col = e.table.LeadColumnIndex() })
	c.Equal(2, col, "and leaves the cursor where it was")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellCursorSpacePressesTheCell verifies that the space key works the cell the cursor is on the way a screen
// reader's press on that cell does — a check box within it is clicked — and that a cell with nothing to press falls
// back to the table's own activation shortcut.
func TestTableCellCursorSpacePressesTheCell(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var box *unison.CheckBox
	var wnd *unison.Window
	clicks := 0
	activations := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			box = unison.NewCheckBox()
			box.SetTitle("Done")
			box.ClickCallback = func() { clicks++ }
			row := newTableTestRow("r0")
			row.cellFactory = func(_, col int) unison.Paneler {
				if col == 1 {
					return box
				}
				return unison.NewPanel()
			}
			table = axNewTable(row)
			table.DoubleClickCallback = func() { activations++ }
			wnd = newHeadlessWindow(t, "cell space", geom.NewRect(10, 10, 400, 200), axColumn(table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.Do(func() {
		table.SelectByIndex(0)
		table.RequestFocus()
	})

	// At row level the space key is the table's own activation shortcut, as it has always been.
	screen.KeyPress(unison.KeySpace, 0)
	var state checkenum.Enum
	screen.Do(func() { state = box.State })
	c.Equal(1, activations)
	c.Equal(0, clicks)
	c.Equal(checkenum.Off, state)

	// On the cell holding the check box, it presses the check box instead.
	screen.Do(func() { c.True(table.SetLeadCell(0, 1)) })
	screen.KeyPress(unison.KeySpace, 0)
	screen.Do(func() { state = box.State })
	c.Equal(1, clicks, "the space should have reached the check box in the cell")
	c.Equal(checkenum.On, state)
	c.Equal(1, activations, "and should not have run the table's shortcut as well")

	// On a cell with nothing in it to press, it falls back to that shortcut.
	screen.Do(func() { c.True(table.SetLeadCell(0, 0)) })
	screen.KeyPress(unison.KeySpace, 0)
	screen.Do(func() { state = box.State })
	c.Equal(2, activations)
	c.Equal(1, clicks)
	c.Equal(checkenum.On, state)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellCursorDrawsARing verifies that the cursor is something a sighted person can see as well: a ring is
// drawn around the cell it is on, and nothing is drawn while the person is at row level.
func TestTableCellCursorDrawsARing(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(3)...)
			wnd = newHeadlessWindow(t, "cursor ring", geom.NewRect(10, 10, 400, 200), axColumn(table))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.Do(func() {
		table.RequestFocus()
		table.SelectByIndex(1)
	})
	screen.Sync()
	atRowLevel := screen.CaptureWindow(wnd)
	c.NotNil(atRowLevel)

	screen.KeyPress(unison.KeyRight, 0)
	screen.Sync()
	onFirstCell := screen.CaptureWindow(wnd)
	c.NotNil(onFirstCell)
	if atRowLevel == nil || onFirstCell == nil {
		return
	}
	c.False(slices.Equal(atRowLevel.Pix, onFirstCell.Pix), "the ring is drawn around the cell the cursor moved onto")

	screen.KeyPress(unison.KeyRight, 0)
	screen.Sync()
	onSecondCell := screen.CaptureWindow(wnd)
	c.NotNil(onSecondCell)
	if onSecondCell != nil {
		c.False(slices.Equal(onFirstCell.Pix, onSecondCell.Pix), "and moves with the cursor")
	}

	screen.KeyPress(unison.KeyEscape, 0)
	screen.Sync()
	again := screen.CaptureWindow(wnd)
	c.NotNil(again)
	if again != nil {
		c.True(slices.Equal(atRowLevel.Pix, again.Pix),
			"nothing is drawn at row level, and nothing else repainted for the cursor either")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellCursorRingShowsOverAWidgetCell verifies that the ring survives a cell whose content paints its own
// background, which is the ordinary case for a table of fields and check boxes: it is drawn after the cells, along the
// cell's edges, where the content cannot cover it, and it does not paint across the content itself.
func TestTableCellCursorRingShowsOverAWidgetCell(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.Do(func() {
		e.table.SelectByIndex(1)
		e.table.RequestFocus()
	})
	screen.Sync()
	atRowLevel := screen.CaptureWindow(wnd)
	c.NotNil(atRowLevel)

	// The cell's full extent between the dividers, and the padded area within it that the field fills, both in the
	// window's coordinates, which are the captured frame's.
	var cell, content geom.Rect
	screen.Do(func() {
		content = e.table.CellFrame(1, 0)
		cell = geom.NewRect(content.X-e.table.Padding.Left, e.table.RowFrame(1).Y,
			e.table.Columns[0].Current, e.table.RowFrame(1).Height)
		cell = e.table.RectToRoot(cell)
		content = e.table.RectToRoot(content)
	})

	screen.KeyPress(unison.KeyRight, 0)
	screen.Sync()
	onFieldCell := screen.CaptureWindow(wnd)
	c.NotNil(onFieldCell)
	if atRowLevel == nil || onFieldCell == nil {
		return
	}
	var col int
	screen.Do(func() { col = e.table.LeadColumnIndex() })
	c.Equal(0, col)
	c.False(slices.Equal(atRowLevel.Pix, onFieldCell.Pix), "the ring shows on a cell holding a field")

	// The stroke runs along the cell's edges: the middle of the top edge and of the left edge, one pixel in from the
	// dividers, are painted by it, while the middle of the field, well inside the content, is left as it was.
	top := image.Pt(int(cell.CenterX()), int(cell.Y)+1)
	left := image.Pt(int(cell.X)+1, int(cell.CenterY()))
	middle := image.Pt(int(content.CenterX()), int(content.CenterY()))
	c.NotEqual(atRowLevel.NRGBAAt(top.X, top.Y), onFieldCell.NRGBAAt(top.X, top.Y),
		"the ring is drawn along the top edge of the cell")
	c.NotEqual(atRowLevel.NRGBAAt(left.X, left.Y), onFieldCell.NRGBAAt(left.X, left.Y),
		"and along its left edge")
	c.Equal(atRowLevel.NRGBAAt(middle.X, middle.Y), onFieldCell.NRGBAAt(middle.X, middle.Y),
		"and is not painted across the field's own content")

	screen.KeyPress(unison.KeyEscape, 0)
	screen.Sync()
	again := screen.CaptureWindow(wnd)
	c.NotNil(again)
	if again != nil {
		c.True(slices.Equal(atRowLevel.Pix, again.Pix), "nothing is drawn once the person is back at row level")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellCursorReturnIsLeftToTheWindow verifies that Return on a cell with nothing to edit is reported as
// untaken, exactly as it is at row level, where the table does not handle it at all and the window's default button is
// what answers it. It is space, not Return, that falls back to the table's own activation shortcut.
func TestTableCellCursorReturnIsLeftToTheWindow(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(2)...)
	activations := 0
	table.DoubleClickCallback = func() { activations++ }
	table.SelectByIndex(0)
	c.False(table.DefaultKeyDown(unison.KeyReturn, 0, false), "Return at row level is not the table's to take")

	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.Equal(0, table.LeadColumnIndex())
	c.False(table.DefaultKeyDown(unison.KeyReturn, 0, false), "nor is Return on a cell with nothing to edit")
	c.False(table.DefaultKeyDown(unison.KeyNumPadEnter, 0, false))
	c.Equal(0, activations, "and neither of them runs the table's activation shortcut")
	c.Equal(0, table.LeadColumnIndex(), "the cursor is left where it was")

	// Space on that same cell does fall back to it, which is what it does at row level.
	c.True(table.DefaultKeyDown(unison.KeySpace, 0, false))
	c.Equal(1, activations)
}

// TestTableCellCursorLeavesWithADeselectedRow verifies that taking the row the person is on out of the selection takes
// the person with it. A cursor that outlived the selection of the row beneath it hopped silently to another selected
// row, while LeadRowIndex went on answering with the row that had just been deselected.
func TestTableCellCursorLeavesWithADeselectedRow(t *testing.T) {
	c := check.New(t)
	table := newCursorTable(flatRows(4)...)
	table.SelectByIndex(0, 2)
	c.True(table.SetLeadCell(2, 0), "the cursor stands on the row it was put on")
	c.Equal([]int{2}, selectedTableIndexes(table), "which becomes the whole of the selection")

	table.SelectByIndex(0)
	c.True(table.SetLeadCell(2, 0))
	table.DeselectByIndex(2)
	c.Equal(-1, table.LeadColumnIndex(), "the cursor goes with the row it was standing on")
	c.Equal(-1, table.LeadRowIndex(), "and so does the row the person was on")

	// A range does the same, but only when it covers the row the person is on. The selection is extended with a
	// shifted Down, which is the gesture that leaves more than one row selected with the cursor still in a column.
	table.SelectByIndex(1)
	c.True(table.DefaultKeyDown(unison.KeyRight, 0, false))
	c.True(table.DefaultKeyDown(unison.KeyDown, mod.Shift, false))
	c.Equal([]int{1, 2}, selectedTableIndexes(table))
	c.Equal(2, table.LeadRowIndex())
	c.Equal(0, table.LeadColumnIndex())

	table.DeselectRange(0, 1)
	c.Equal(2, table.LeadRowIndex(), "a range that leaves the row alone leaves the person on it")
	c.Equal(0, table.LeadColumnIndex())
	table.DeselectRange(2, 3)
	c.Equal(-1, table.LeadRowIndex())
	c.Equal(-1, table.LeadColumnIndex())
}
