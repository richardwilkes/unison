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
	"strconv"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
)

// These tests drive a table whose cells are real Fields the way a person would, through the headless driver: they
// click into a cell, type at it, tab out of it and take the row away from underneath it. What they are all really
// checking is one thing — that a cell holding the keyboard focus stays part of the table for as long as it holds it —
// since that is what lets the window deliver keys, commands, scrolling and caret blinking to a widget the table
// otherwise only borrows for the length of a single event. A session owns the package's mutable globals while it runs,
// so none of them may call t.Parallel.

// editTable is the fixture these tests are built on: a table of Fields, plus the counters needed to tell a focus
// change that really happened from one that only looks as though it did.
type editTable struct {
	model  *unison.SimpleTableModel[*tableTestRow]
	table  *unison.Table[*tableTestRow]
	rows   []*tableTestRow
	fields [][]*unison.Field
	gained [][]int
	lost   [][]int
	built  int
}

// newEditTable builds a table of rowCount rows whose first fieldCols columns each hold a memoized Field carrying the
// given text, followed by one more column holding a plain, unfocusable panel — the cell a traversal has to skip and a
// click has to land in to leave the field being edited. When wrap is true, every ColumnCell call returns a freshly
// created panel wrapped around the memoized field instead of the field itself, which is the shape of row that only
// works if the table adopts the newest wrapper the moment it is handed one.
func newEditTable(rowCount, fieldCols int, text string, wrap bool) *editTable {
	e := &editTable{
		model:  &unison.SimpleTableModel[*tableTestRow]{},
		rows:   make([]*tableTestRow, rowCount),
		fields: make([][]*unison.Field, rowCount),
		gained: make([][]int, rowCount),
		lost:   make([][]int, rowCount),
	}
	for r := range e.rows {
		row := newTableTestRow("r" + strconv.Itoa(r))
		e.fields[r] = make([]*unison.Field, fieldCols)
		e.gained[r] = make([]int, fieldCols)
		e.lost[r] = make([]int, fieldCols)
		for col := range fieldCols {
			field := unison.NewField()
			field.SetText(text)
			field.SetLayoutData(&unison.FlexLayoutData{
				HAlign: align.Fill,
				VAlign: align.Fill,
				HGrab:  true,
				VGrab:  true,
			})
			// The field's own focus callbacks were wrapped when its border was installed, so they are chained rather
			// than replaced; replacing them would leave the field without the border changes it makes for itself and
			// without the select-all and caret handling that go with taking the focus.
			gained := field.GainedFocusCallback
			field.GainedFocusCallback = func() {
				e.gained[r][col]++
				gained()
			}
			lost := field.LostFocusCallback
			field.LostFocusCallback = func() {
				e.lost[r][col]++
				lost()
			}
			e.fields[r][col] = field
		}
		plain := unison.NewPanel()
		row.cellFactory = func(_, col int) unison.Paneler {
			if col >= fieldCols {
				return plain
			}
			field := e.fields[r][col]
			if !wrap {
				return field
			}
			e.built++
			wrapper := unison.NewPanel()
			wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
			wrapper.AddChild(field)
			return wrapper
		}
		e.rows[r] = row
	}
	e.model.SetRootRows(e.rows)
	e.table = unison.NewTable[*tableTestRow](e.model)
	e.table.Columns = make([]unison.ColumnInfo, fieldCols+1)
	for i := range fieldCols {
		e.table.Columns[i] = unison.ColumnInfo{ID: i, Current: 120}
	}
	e.table.Columns[fieldCols] = unison.ColumnInfo{ID: fieldCols, Current: 60}
	e.table.SyncToModel()
	return e
}

// newEditWindow shows the table in a window, at its own preferred size in the top left corner rather than stretched to
// fill the window, so that every cell is where CellFrame says it is. When asked for, a button follows the table in the
// same container, giving a Tab out of the last cell somewhere to go. The window is brought to the front, since a
// window that was merely shown holds no focus and a panel in it would report itself unfocused however the focus was
// given to it.
func newEditWindow(t *testing.T, e *editTable, withButton bool) (*unison.Window, *unison.Button) {
	t.Helper()
	container := unison.NewPanel()
	container.SetLayout(&unison.FlexLayout{
		Columns:  1,
		HSpacing: unison.StdHSpacing,
		VSpacing: unison.StdVSpacing,
	})
	e.table.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
	container.AddChild(e.table)
	var button *unison.Button
	if withButton {
		button = unison.NewButton()
		button.SetTitle("After")
		button.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Start})
		container.AddChild(button)
	}
	wnd := newHeadlessWindow(t, "table edit", geom.NewRect(20, 20, 420, 300), container)
	if wnd != nil {
		wnd.ToFront()
	}
	return wnd, button
}

// newEditScrollWindow shows the table inside a scroll panel, wired as an application would wire it, in a window too
// short to show all of the rows at once, so that reaching a row means scrolling to it.
func newEditScrollWindow(t *testing.T, e *editTable) (*unison.Window, *unison.ScrollPanel) {
	t.Helper()
	scroll := unison.NewScrollPanel()
	scroll.SetContent(e.table, behavior.Fill, behavior.Fill)
	wnd := newHeadlessWindow(t, "table edit scroll", geom.NewRect(20, 20, 400, 240), scroll)
	if wnd != nil {
		wnd.ToFront()
	}
	return wnd, scroll
}

// cellCenter returns the screen point to aim at to hit the middle of a cell. It cannot be had from PanelCenter, since
// a cell that isn't the focused one has no parent between events and therefore no position on the screen at all; the
// table does have one, so the cell's frame is expressed as an offset into the table instead.
func cellCenter(screen *unison.HeadlessScreen, table *unison.Table[*tableTestRow], row, col int) geom.Point {
	var offset geom.Point
	screen.Do(func() { offset = table.CellFrame(row, col).Center().Sub(table.ContentRect(false).Point) })
	return screen.PanelPoint(table, offset)
}

// cellSnapshot is everything an assertion below might want to know after a step. Widget state may only be touched on
// the UI thread, so it is all collected in a single pass rather than one Do per question.
type cellSnapshot struct {
	text          string
	selectedText  string
	focusRow      int
	focusCol      int
	gained        int
	lost          int
	tableSelCount int
	firstSelected int
	selStart      int
	selEnd        int
	tableFocused  bool
	fieldFocused  bool
	installed     bool
	wrapped       bool
	detached      bool
	hasRange      bool
}

// snapshot reads the state of the table and of one of its fields.
func (e *editTable) snapshot(c check.Checker, screen *unison.HeadlessScreen, row, col int) cellSnapshot {
	var s cellSnapshot
	c.True(screen.Do(func() {
		s.focusRow, s.focusCol = e.table.FocusedCell()
		s.tableFocused = e.table.Focused()
		s.tableSelCount = e.table.SelectionCount()
		s.firstSelected = e.table.FirstSelectedRowIndex()
		field := e.fields[row][col]
		s.fieldFocused = field.Focused()
		s.text = field.Text()
		s.selectedText = field.SelectedText()
		s.hasRange = field.HasSelectionRange()
		s.selStart, s.selEnd = field.Selection()
		s.gained = e.gained[row][col]
		s.lost = e.lost[row][col]
		parent := field.Parent()
		s.detached = parent == nil
		s.installed = parent == e.table.AsPanel()
		s.wrapped = parent != nil && parent.Parent() == e.table.AsPanel()
	}))
	return s
}

// texts returns the text of every field in the table, for the assertions that a keystroke went to one field and to no
// other.
func (e *editTable) texts(c check.Checker, screen *unison.HeadlessScreen) [][]string {
	all := make([][]string, len(e.fields))
	c.True(screen.Do(func() {
		for r, row := range e.fields {
			all[r] = make([]string, len(row))
			for col, field := range row {
				all[r][col] = field.Text()
			}
		}
	}))
	return all
}

// TestTableCellFieldClickTypes proves the point of the whole feature: clicking a Field in a table cell puts the
// keyboard focus into that field and leaves it there, so the keys that follow edit the cell instead of driving the
// table, and the cell stays part of the table across the redraws that come after.
func TestTableCellFieldClickTypes(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 1, 0))
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused, "clicking the cell should have given its field the focus")
	c.True(s.installed, "the focused cell should have been left attached to the table")
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	c.False(s.tableFocused, "the table gave the focus to the cell rather than keeping it")
	c.Equal(1, s.gained)

	screen.Type("hello")
	c.Equal([][]string{{"", ""}, {"hello", ""}, {"", ""}}, e.texts(c, screen),
		"only the field that was clicked should have received the text")

	screen.KeyPress(unison.KeyBackspace, 0)
	s = e.snapshot(c, screen, 1, 0)
	c.Equal("hell", s.text, "backspace should have deleted a rune from the field rather than typing one")

	// A redraw is where the table decides all over again what each of its cells is; the one being edited has to come
	// through it still installed and still holding the focus.
	screen.Sync()
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.True(s.installed)
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal(0, s.lost, "nothing should have taken the focus away from the field")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldReclickKeepsFocus verifies that a second click inside the cell being edited moves the caret rather
// than starting over: the table must not bounce the focus out to itself and back again, since the field would then
// treat the click as the one that first focused it and select all of its text.
func TestTableCellFieldReclickKeepsFocus(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "alpha", false)
			for _, row := range e.fields {
				for _, field := range row {
					field.InitialClickSelectsAll = func(_ *unison.Field) bool { return true }
				}
			}
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 1, 0))
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.Equal("alpha", s.selectedText, "the first click into the field should have selected all of its text")

	screen.Type("Z")
	s = e.snapshot(c, screen, 1, 0)
	c.Equal("Z", s.text, "typing over the selection should have replaced it")

	// The field is installed in the table now, so it has a position of its own to aim at.
	at := screen.PanelCenter(e.fields[1][0])
	c.NotEqual(geom.Point{}, at, "a focused cell's field is part of the window and therefore has a screen position")
	screen.Click(at)
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.False(s.hasRange, "the second click should have placed the caret rather than selected everything")
	c.Equal(1, s.gained, "the field never lost the focus, so it never gained it a second time")
	c.Equal(0, s.lost)

	screen.Type("Y")
	s = e.snapshot(c, screen, 1, 0)
	c.Equal("ZY", s.text, "a focus that had been re-grabbed would have selected all and left just the new rune")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldLosesFocusOnClickElsewhere verifies the other half of the bargain: clicking away from the cell
// being edited ends the editing, detaches the cell, hands the focus back to the table and leaves the table taking the
// keys again.
func TestTableCellFieldLosesFocusOnClickElsewhere(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	screen.Type("abc")
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused)
	c.Equal("abc", s.text)

	// The last column holds a plain panel, so the press lands on the table itself and selects the row.
	screen.Click(cellCenter(screen, e.table, 2, 2))
	s = e.snapshot(c, screen, 0, 0)
	c.Equal(1, s.lost, "the field should have been told it lost the focus exactly once")
	c.True(s.tableFocused, "the focus should have come back to the table")
	c.Equal(-1, s.focusRow)
	c.Equal(-1, s.focusCol)
	c.True(s.detached, "the cell that is no longer being edited should have been detached from the table")
	c.Equal("abc", s.text, "ending the editing should not have disturbed what was typed")
	c.Equal(1, s.tableSelCount)
	c.Equal(2, s.firstSelected, "the click should have selected the row it landed in")

	screen.KeyPress(unison.KeyUp, 0)
	s = e.snapshot(c, screen, 0, 0)
	c.Equal(1, s.tableSelCount)
	c.Equal(1, s.firstSelected, "the table owns the keys again, so the arrow should have moved the selection")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldTabTraversal verifies that Tab and Shift-Tab walk the focusable cells in row-major order, skipping
// the cells that can take no focus, and that reaching either end hands the traversal back to the window so that it
// continues out of the table and on to whatever comes next.
func TestTableCellFieldTabTraversal(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var button *unison.Button
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			_, button = newEditWindow(t, e, true)
		}))
	c.NotNil(e)
	c.NotNil(button)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	screen.KeyPress(unison.KeyTab, 0)
	s := e.snapshot(c, screen, 0, 1)
	c.Equal(0, s.focusRow)
	c.Equal(1, s.focusCol)
	c.True(s.fieldFocused)
	c.True(s.installed, "the cell that took the focus should have been installed in the table")
	old := e.snapshot(c, screen, 0, 0)
	c.True(old.detached, "the cell that gave the focus up should have been detached")
	c.Equal(1, old.lost)

	// The last column holds nothing that can take the focus, so the next Tab passes it by and starts the next row.
	screen.KeyPress(unison.KeyTab, 0)
	s = e.snapshot(c, screen, 1, 0)
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	c.True(s.fieldFocused)

	screen.KeyPress(unison.KeyTab, mod.Shift)
	s = e.snapshot(c, screen, 0, 1)
	c.Equal(0, s.focusRow)
	c.Equal(1, s.focusCol)
	c.True(s.fieldFocused)

	// A Tab out of the last focusable cell leaves the table entirely, landing on the panel that follows it.
	var focused bool
	c.True(screen.Do(func() { focused = e.table.FocusCell(2, 1) }))
	c.True(focused)
	screen.KeyPress(unison.KeyTab, 0)
	var onButton bool
	c.True(screen.Do(func() { onButton = button.Focused() }))
	c.True(onButton, "there is no cell after the last one, so the Tab should have continued past the table")
	s = e.snapshot(c, screen, 2, 1)
	c.Equal(-1, s.focusRow)
	c.Equal(-1, s.focusCol)
	c.True(s.detached)

	before := e.texts(c, screen)
	screen.Type("q")
	c.Equal(before, e.texts(c, screen), "with the focus outside the table, typing should reach no field")

	// Shift-Tab from the button lands on the table itself rather than back inside one of its cells: the table takes
	// the focus as a whole, and only a click or FocusCell puts it into a cell.
	screen.KeyPress(unison.KeyTab, mod.Shift)
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.tableFocused)
	c.Equal(-1, s.focusRow)
	c.Equal(-1, s.focusCol)

	// And a Shift-Tab out of the first cell continues the window's own traversal, which wraps around to the end.
	c.True(screen.Do(func() { focused = e.table.FocusCell(0, 0) }))
	c.True(focused)
	screen.KeyPress(unison.KeyTab, mod.Shift)
	c.True(screen.Do(func() { onButton = button.Focused() }))
	c.True(onButton, "the traversal should have left the table at its start and wrapped around to the button")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldReturnEscapeSpace verifies the keys that end an editing session and the one that must not: Return,
// NumPadEnter and Escape each hand the focus back to the table, while a Space typed into the field is text rather than
// the table's own activation shortcut.
func TestTableCellFieldReturnEscapeSpace(t *testing.T) {
	c := check.New(t)
	var e *editTable
	activations := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			e.table.DoubleClickCallback = func() { activations++ }
			// The table only acts on a Space when it has a selection, so there has to be one for the assertions that
			// the Space never reached it to mean anything.
			e.table.SelectByIndex(1)
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	for i, one := range []struct {
		name string
		key  unison.KeyCode
	}{
		{key: unison.KeyReturn, name: "return"},
		{key: unison.KeyEscape, name: "escape"},
		{key: unison.KeyNumPadEnter, name: "numpad enter"},
	} {
		screen.Click(cellCenter(screen, e.table, 1, 0))
		if i == 0 {
			screen.Type("ab")
		}
		s := e.snapshot(c, screen, 1, 0)
		c.True(s.fieldFocused, "the click should have started an editing session for the %s case", one.name)

		screen.KeyPress(one.key, 0)
		s = e.snapshot(c, screen, 1, 0)
		c.True(s.tableFocused, "%s should have handed the focus back to the table", one.name)
		c.Equal(-1, s.focusRow)
		c.Equal(-1, s.focusCol)
		c.Equal(i+1, s.lost, "%s should have ended the editing session", one.name)
		c.Equal(0, activations, "%s must not have reached the table's activation shortcut", one.name)
		c.Equal("ab", s.text, "%s should have left the text alone", one.name)
		c.Equal(1, s.tableSelCount, "%s should have left the selection alone", one.name)
		c.Equal(1, s.firstSelected)
	}

	// A Space belongs to whatever holds the focus, which is the field.
	screen.Click(cellCenter(screen, e.table, 1, 0))
	screen.Type(" ")
	s := e.snapshot(c, screen, 1, 0)
	c.Equal("ab ", s.text, "the space should have been typed into the field")
	c.True(s.fieldFocused, "a space is text, so it should not have ended the editing session")
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	var fired int
	c.True(screen.Do(func() { fired = activations }))
	c.Equal(0, fired, "the table must not have seen the space")

	// The very same key, once the table holds the focus again, is the table's activation shortcut, which is what makes
	// the assertions above worth making.
	screen.KeyPress(unison.KeyEscape, 0)
	screen.KeyPress(unison.KeySpace, 0)
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.tableFocused)
	c.True(screen.Do(func() { fired = activations }))
	c.Equal(1, fired, "a space with the table focused should have run its activation callback")
	c.Equal("ab ", s.text, "the table's space should not have reached the field")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldSelectAllStaysInField verifies that a command from the menu bar is dispatched to the widget being
// edited rather than to the table it sits in: Select All while a cell is focused selects the cell's text and leaves
// the table's selection alone.
func TestTableCellFieldSelectAllStaysInField(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "alpha", false)
			e.table.SelectByIndex(0)
			wnd, _ := newEditWindow(t, e, false)
			if wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(wnd, func(m unison.Menu) {
				unison.InsertStdMenus(m, nil, nil, nil)
			})
		}))
	c.NotNil(e)

	// Clicking places the caret rather than selecting, so what the command does is visible in the result.
	screen.Click(cellCenter(screen, e.table, 1, 0))
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.False(s.hasRange)

	screen.KeyPress(unison.KeyA, mod.OSMenuCommand())
	s = e.snapshot(c, screen, 1, 0)
	c.Equal("alpha", s.selectedText, "the command should have selected the field's text")
	c.Equal(1, s.tableSelCount, "the table's own Select All must not have run")
	c.Equal(0, s.firstSelected)
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldRowRemovedWhileEditing verifies that taking the row being edited out of the model ends the editing
// session cleanly: the focus comes back to the table, the cell is detached, and what was typed is left in the widget
// the application still owns.
func TestTableCellFieldRowRemovedWhileEditing(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(e)
	c.NotNil(wnd)

	screen.Click(cellCenter(screen, e.table, 1, 0))
	screen.Type("abc")
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.Equal("abc", s.text)

	var focusIsTable bool
	c.True(screen.Do(func() {
		e.model.SetRootRows([]*tableTestRow{e.rows[0], e.rows[2]})
		e.table.SyncToModel()
		focusIsTable = wnd.Focus().Is(e.table)
	}))
	c.True(focusIsTable, "the focus should have come back to the table when the row went away")
	s = e.snapshot(c, screen, 1, 0)
	c.Equal(1, s.lost)
	c.Equal(-1, s.focusRow)
	c.Equal(-1, s.focusCol)
	c.True(s.detached)
	c.Equal("abc", s.text, "the row is gone, but the field the application holds is not")

	screen.KeyPress(unison.KeyDown, 0)
	s = e.snapshot(c, screen, 1, 0)
	c.Equal(1, s.tableSelCount)
	c.Equal(0, s.firstSelected, "the table should be taking the keys again")

	before := e.texts(c, screen)
	screen.Type("zzz")
	c.Equal(before, e.texts(c, screen), "no field should still be collecting what is typed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableFocusCellScrollsIntoView verifies the programmatic entry point: FocusCell brings the cell into view and
// starts editing it, refuses the positions it cannot honor without disturbing what is already being edited, and moving
// on to another cell releases the previous one.
func TestTableFocusCellScrollsIntoView(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var scroll *unison.ScrollPanel
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(40, 2, "", false)
			_, scroll = newEditScrollWindow(t, e)
		}))
	c.NotNil(e)
	c.NotNil(scroll)

	var focused bool
	c.True(screen.Do(func() { focused = e.table.FocusCell(30, 1) }))
	c.True(focused)
	screen.Sync()
	var viewRect, fieldRect geom.Rect
	var v float32
	c.True(screen.Do(func() {
		view := scroll.ContentView()
		viewRect = view.RectToRoot(view.ContentRect(false))
		field := e.fields[30][1]
		fieldRect = field.RectToRoot(field.ContentRect(false))
		_, v = scroll.Position()
	}))
	s := e.snapshot(c, screen, 30, 1)
	c.True(s.fieldFocused)
	c.Equal(30, s.focusRow)
	c.Equal(1, s.focusCol)
	c.True(viewRect.Contains(fieldRect), "the focused cell should have been scrolled into view: %v is not within %v",
		fieldRect, viewRect)
	c.True(v > 0, "reaching row 30 in this window means scrolling down to it")

	screen.Type("far")
	s = e.snapshot(c, screen, 30, 1)
	c.Equal("far", s.text, "a cell that was scrolled to should be editable where it landed")

	screen.Click(screen.PanelCenter(e.fields[30][1]))
	s = e.snapshot(c, screen, 30, 1)
	c.True(s.fieldFocused, "clicking the cell that is already being edited should have left it alone")
	c.Equal(1, s.gained)

	// Positions the table cannot focus are refused, and refusing one changes nothing.
	var outOfRange, negative, plainCell bool
	c.True(screen.Do(func() {
		outOfRange = e.table.FocusCell(40, 0)
		negative = e.table.FocusCell(0, -1)
		plainCell = e.table.FocusCell(0, 2)
	}))
	c.False(outOfRange, "there is no row 40")
	c.False(negative, "there is no column -1")
	c.False(plainCell, "the last column holds nothing that can take the focus")
	s = e.snapshot(c, screen, 30, 1)
	c.True(s.fieldFocused, "a refused FocusCell must not disturb the cell being edited")
	c.Equal(30, s.focusRow)
	c.Equal(1, s.focusCol)
	c.Equal(0, s.lost)

	c.True(screen.Do(func() { focused = e.table.FocusCell(0, 0) }))
	c.True(focused)
	screen.Sync()
	var padding float32
	c.True(screen.Do(func() {
		view := scroll.ContentView()
		viewRect = view.RectToRoot(view.ContentRect(false))
		field := e.fields[0][0]
		fieldRect = field.RectToRoot(field.ContentRect(false))
		padding = e.table.Padding.Top
		_, v = scroll.Position()
	}))
	c.True(viewRect.Contains(fieldRect), "the first row should have been scrolled into view: %v is not within %v",
		fieldRect, viewRect)
	// A cell's frame is inset by the table's padding and scrolling stops as soon as the cell is showing, so coming
	// back to the first row leaves the view that padding short of the very top rather than at it.
	c.True(v <= padding, "moving to the first row should have scrolled back to the top, but the view is at %v", v)
	old := e.snapshot(c, screen, 30, 1)
	c.Equal(1, old.lost, "the cell that was being edited should have been released")
	c.True(old.detached)
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused)
	c.True(s.installed)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldWrapperPerCall covers the shape of row that builds a fresh panel around a memoized editor every
// time it is asked for a cell. Each redraw, sync and resize therefore hands the table a different panel holding the
// same field, and the editing has to survive all of them, since the table has to notice and adopt the newest wrapper
// the moment it is given one.
func TestTableCellFieldWrapperPerCall(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", true)
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 1, 0))
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.True(s.wrapped, "the wrapper the row built should be what the table installed")
	c.Equal(1, s.focusRow)
	c.Equal(0, s.focusCol)
	var built int
	c.True(screen.Do(func() { built = e.built }))
	c.True(built > 1, "the row should have been asked for its cells more than once by now")

	screen.Type("ab")
	for _, one := range []struct {
		disturb func()
		typed   string
		want    string
		name    string
	}{
		{disturb: func() { e.table.MarkForRedraw() }, typed: "cd", want: "abcd", name: "a redraw"},
		{disturb: func() { e.table.SyncToModel() }, typed: "ef", want: "abcdef", name: "a sync to the model"},
		{disturb: func() { e.table.SizeColumnsToFit(true) }, typed: "g", want: "abcdefg", name: "a column resize"},
	} {
		c.True(screen.Do(one.disturb))
		screen.Sync()
		screen.Type(one.typed)
		s = e.snapshot(c, screen, 1, 0)
		c.True(s.fieldFocused, "%s should not have ended the editing session", one.name)
		c.True(s.wrapped, "%s should have left the newest wrapper installed", one.name)
		c.Equal(one.want, s.text, "the keys after %s should still have reached the field", one.name)
	}

	screen.Click(screen.PanelCenter(e.fields[1][0]))
	s = e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)
	c.Equal(1, s.gained, "the field held the focus throughout, so it gained it only once")
	c.Equal(0, s.lost)

	screen.KeyPress(unison.KeyTab, 0)
	s = e.snapshot(c, screen, 1, 1)
	c.Equal(1, s.focusRow)
	c.Equal(1, s.focusCol)
	c.True(s.fieldFocused)
	c.True(s.wrapped)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldArrowsDoNotMoveSelection verifies that the keys the table would otherwise navigate with belong to
// the field while a cell is being edited: they move the caret and leave the table's selection where it was.
func TestTableCellFieldArrowsDoNotMoveSelection(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "alpha", false)
			e.table.SelectByIndex(1)
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 1, 0))
	s := e.snapshot(c, screen, 1, 0)
	c.True(s.fieldFocused)

	for _, one := range []struct {
		name  string
		key   unison.KeyCode
		start int
		end   int
	}{
		{name: "down", key: unison.KeyDown, start: 5, end: 5},
		{name: "up", key: unison.KeyUp, start: 0, end: 0},
		{name: "home", key: unison.KeyHome, start: 0, end: 0},
		{name: "end", key: unison.KeyEnd, start: 5, end: 5},
	} {
		screen.KeyPress(one.key, 0)
		s = e.snapshot(c, screen, 1, 0)
		c.True(s.fieldFocused, "%s should have left the editing session running", one.name)
		c.Equal(one.start, s.selStart, "%s should have moved the caret within the field", one.name)
		c.Equal(one.end, s.selEnd)
		c.Equal(1, s.tableSelCount, "%s must not have moved the table's selection", one.name)
		c.Equal(1, s.firstSelected)
		c.Equal("alpha", s.text)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldCaretBlinks verifies that the timer a focused field keeps for its caret runs against a cell in a
// table just as it does anywhere else: it fires several times, finds the field still part of a window each time, and
// leaves the editing session exactly as it was.
func TestTableCellFieldCaretBlinks(t *testing.T) {
	c := check.New(t)
	var e *editTable
	// The default blink rate would make the wait below take seconds, and the rate itself is not what is under test, so
	// the fixture's fields blink much faster. This has to happen before the window is shown, since the first blink is
	// scheduled by the first draw of a focused field.
	const blinkRate = 5 * time.Millisecond
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			for _, row := range e.fields {
				for _, field := range row {
					field.BlinkRate = blinkRate
				}
			}
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused)

	// Sync() waits for the application to go quiet, not for a timer that has yet to fire, so the wait for the blinks
	// has to be a real one.
	time.Sleep(10 * blinkRate)
	screen.Sync()
	s = e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the blinking caret should not have disturbed the focus")
	c.True(s.installed)
	c.Equal(0, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal(0, s.lost)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFocusedCellUsesPlainInks verifies that the cell holding the focus is handed the row's plain inks rather
// than the selection inks while its row is selected, so that it stands out as the cell being edited and so that a
// Field's own text selection, which is drawn with the same ink as a selected row, stays visible within it. The row must
// still be reported as selected, the other cells of the row must keep the selection inks, and the selection inks must
// come back to the cell the moment the focus returns to the table.
func TestTableCellFocusedCellUsesPlainInks(t *testing.T) {
	c := check.New(t)
	type cellInks struct {
		fg, bg   unison.Ink
		selected bool
	}
	var e *editTable
	seen := make(map[[2]int]cellInks)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "alpha", false)
			for _, row := range e.rows {
				row.cellParams = func(row, col int, fg, bg unison.Ink, selected, _, _ bool) {
					seen[[2]int{row, col}] = cellInks{fg: fg, bg: bg, selected: selected}
				}
			}
			newEditWindow(t, e, false)
			e.table.RequestFocus()
			e.table.SelectByIndex(1)
		}))
	c.NotNil(e)
	// inks reads what the row was last told about a cell, after a redraw, along with the theme inks to compare against.
	inks := func(row, col int) (got, plain, sel cellInks) {
		c.True(screen.Do(func() { e.table.MarkForRedraw() }))
		screen.Sync()
		c.True(screen.Do(func() {
			got = seen[[2]int{row, col}]
			if row%2 == 1 {
				plain = cellInks{fg: e.table.OnBandingInk, bg: e.table.BandingInk}
			} else {
				plain = cellInks{fg: e.table.OnBackgroundInk, bg: e.table.BackgroundInk}
			}
			sel = cellInks{fg: e.table.OnSelectionInk, bg: e.table.SelectionInk}
		}))
		return got, plain, sel
	}

	// With no cell focused, every cell of the selected row gets the selection inks.
	got, _, sel := inks(1, 0)
	c.True(got.selected, "row 1 should be selected")
	c.True(got.fg == sel.fg && got.bg == sel.bg, "an unfocused cell of a selected row should get the selection inks")

	var focused bool
	c.True(screen.Do(func() { focused = e.table.FocusCell(1, 0) }))
	c.True(focused, "the field should have taken the focus")

	// The focused cell gets the row's plain inks, but is still told that its row is selected.
	got, plain, sel := inks(1, 0)
	c.True(got.selected, "the focused cell must still be told that its row is selected")
	c.True(got.fg == plain.fg && got.bg == plain.bg, "the focused cell should get the row's plain inks")
	c.False(got.bg == sel.bg, "the focused cell must not get the selection background")

	// The other cell of the same row, and the cells of the unselected rows, are unaffected.
	got, _, sel = inks(1, 1)
	c.True(got.selected)
	c.True(got.fg == sel.fg && got.bg == sel.bg, "the rest of the selected row should keep the selection inks")
	got, plain, _ = inks(0, 0)
	c.False(got.selected)
	c.True(got.fg == plain.fg && got.bg == plain.bg, "an unselected row should get its plain inks")

	// Handing the focus back to the table restores the selection inks to the cell.
	screen.KeyPress(unison.KeyEscape, 0)
	got, _, sel = inks(1, 0)
	c.True(got.selected)
	c.True(got.fg == sel.fg && got.bg == sel.bg, "the selection inks should return once the cell loses the focus")

	// The same holds for a cell on an unbanded row, whose plain inks are the table's background inks.
	c.True(screen.Do(func() {
		e.table.ClearSelection()
		e.table.SelectByIndex(0)
		focused = e.table.FocusCell(0, 1)
	}))
	c.True(focused, "the field should have taken the focus")
	got, plain, sel = inks(0, 1)
	c.True(got.selected)
	c.True(got.fg == plain.fg && got.bg == plain.bg, "the focused cell of an unbanded row should get the background inks")
	c.False(got.bg == sel.bg)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
