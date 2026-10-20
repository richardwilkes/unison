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
	"strconv"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/testenv"
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

// TestTableCellFieldClickAfterACommitRemovesTheRows verifies that a click on a row while a field in another cell is
// being edited looks the row up only once the field has committed on losing the focus, so that a click on a row the
// commit removed selects nothing rather than panicking on a stale index.
func TestTableCellFieldClickAfterACommitRemovesTheRows(t *testing.T) {
	c := check.New(t)
	var e *editTable
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table))
	// Where the last row is, before the commit removes it.
	lastRow := cellCenter(screen, e.table, 2, 1)
	screen.Do(func() {
		lost := e.fields[0][0].LostFocusCallback
		e.fields[0][0].LostFocusCallback = func() {
			if lost != nil {
				lost()
			}
			e.model.SetRootRows(e.rows[:2])
			e.table.SyncToModel()
		}
	})
	screen.Click(lastRow)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
	s = e.snapshot(c, screen, 0, 0)
	c.False(s.fieldFocused, "the click takes the focus from the field")
	c.True(s.tableFocused, "and gives it to the table")
	c.Equal(1, s.lost)
	var gone bool
	screen.Do(func() { gone = e.table.RowToIndex(e.rows[2]) == -1 })
	c.True(gone, "the test needs the commit to have removed the row under the pointer")
	c.Equal([]int{0}, axTableSelectedIndexes(screen, e.table),
		"a click where no row is any more leaves the selection as it was")
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
	c.Equal(1, s.firstSelected, "the click into the field made its row the whole of the selection")
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
		focusIsTable = wnd.CurrentFocus().Is(e.table)
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
	testenv.SkipTimingSensitive(t)
	c := check.New(t)
	var e *editTable
	// Every blink redraws the field, so the blinks are counted by counting the times the field is drawn while it holds
	// the focus: the draw that shows it taking the focus is among them, but once the application has reacted to the
	// click nothing except the caret's timer has any reason to draw the field again. The default blink rate would make
	// waiting for that many take seconds, and the rate itself is not what is under test, so the fixture's fields blink
	// much faster. This has to happen before the window is shown, since the first blink is scheduled by the first draw
	// of a focused field, at whatever rate the field has at that moment.
	const blinkRate = 5 * time.Millisecond
	const wantDraws = 8
	draws := 0
	blinked := make(chan struct{})
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "", false)
			for _, row := range e.fields {
				for _, field := range row {
					field.BlinkRate = blinkRate
				}
			}
			field := e.fields[0][0]
			draw := field.DrawCallback
			field.DrawCallback = func(gc *unison.Canvas, rect geom.Rect) {
				draw(gc, rect)
				if field.Focused() {
					draws++
					if draws == wantDraws {
						close(blinked)
					}
				}
			}
			newEditWindow(t, e, false)
		}))
	c.NotNil(e)

	// A field blinking this fast never lets the application go quiet on a slow enough machine: the timer has fired
	// again before a pass of the event loop can draw the caret it toggled and find nothing left outstanding, so a wait
	// for the application to settle would run to its timeout instead. Nothing may wait for that while the field is
	// blinking. The click is therefore injected from the UI thread, where the driver only queues its events, and the
	// blinks themselves are what is waited for.
	pt := cellCenter(screen, e.table, 0, 0)
	c.True(screen.Post(func() { screen.Click(pt) }))
	select {
	case <-blinked:
	case <-time.After(10 * time.Second):
		t.Fatal("the caret never blinked")
	}

	// Pushing the rate out to something that cannot fire within the test lets the application go quiet again: the one
	// blink that may already be in flight reschedules itself an hour out, and from then on there is nothing for Sync()
	// to wait for. The rate is only touched on the UI thread, since that is where the blink reads it.
	c.True(screen.Do(func() {
		for _, row := range e.fields {
			for _, field := range row {
				field.BlinkRate = time.Hour
			}
		}
	}))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the blinking caret should not have disturbed the focus")
	c.True(s.installed)
	c.Equal(0, s.focusRow)
	c.Equal(0, s.focusCol)
	c.Equal(1, s.gained, "the field held the focus throughout, so it gained it only once")
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

// TestTableFocusCellSelectsTheRow verifies that the focus arriving in a cell of a row that is not selected takes the
// person to that cell: the row becomes the whole of the selection and the cursor lands on the column, so that Tab
// carries both along with the focus and Escape hands the focus back to the table on the cell that was being edited.
// Leaving the selection where it was left the ring and the cursor on some other row, and Escape then landed on a cell
// nobody was in.
func TestTableFocusCellSelectsTheRow(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	changes := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}

	// The person starts on the second cell of the middle row, with the selection counting from there.
	var focused bool
	c.True(screen.Do(func() {
		c.True(e.table.SetLeadCell(1, 1))
		e.table.SelectionChangedCallback = func() { changes++ }
		focused = e.table.FocusCell(2, 0)
	}))
	c.True(focused)
	var indexes []int
	var row, col int
	c.True(screen.Do(func() {
		indexes = selectedTableIndexes(e.table)
		row = e.table.LeadRowIndex()
		col = e.table.LeadColumnIndex()
	}))
	c.Equal([]int{2}, indexes, "the row whose cell took the focus becomes the whole of the selection")
	c.Equal(2, row)
	c.Equal(0, col)
	c.Equal(1, changes, "and the application hears of the one change that really happened")

	// Tab moves the focus along the row, and the cursor goes with it.
	screen.KeyPress(unison.KeyTab, 0)
	s := e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused)
	c.Equal(2, s.focusRow)
	c.Equal(1, s.focusCol)
	c.True(screen.Do(func() {
		indexes = selectedTableIndexes(e.table)
		col = e.table.LeadColumnIndex()
	}))
	c.Equal(1, col, "the cursor follows the focus across the row")
	c.Equal([]int{2}, indexes)
	c.Equal(1, changes, "a move within the row that was already the whole of the selection changes nothing")

	// Escape ends the editing and leaves the person standing on the cell they were working in.
	screen.KeyPress(unison.KeyEscape, 0)
	s = e.snapshot(c, screen, 2, 1)
	c.True(s.tableFocused, "Escape hands the focus back to the table")
	c.Equal(-1, s.focusCol, "the editing session is over")
	c.True(screen.Do(func() {
		row = e.table.LeadRowIndex()
		col = e.table.LeadColumnIndex()
	}))
	c.Equal(2, row)
	c.Equal(1, col, "the cursor is left on the cell that was being edited")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldClickSelectsTheRow verifies the same thing for the gesture that starts most editing sessions: a
// click into a Field in a row that is not selected selects that row and puts the cell cursor on the cell that was
// clicked, which is where Escape from the editor leaves the person.
func TestTableCellFieldClickSelectsTheRow(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			e.table.SelectByIndex(0)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}

	screen.Click(cellCenter(screen, e.table, 2, 1))
	s := e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused, "the click should have put the focus into the field")
	var indexes []int
	var row, col int
	c.True(screen.Do(func() {
		indexes = selectedTableIndexes(e.table)
		row = e.table.LeadRowIndex()
		col = e.table.LeadColumnIndex()
	}))
	c.Equal([]int{2}, indexes, "clicking into a widget a cell holds selects the row it belongs to")
	c.Equal(2, row)
	c.Equal(1, col, "and puts the cursor on the cell being worked in")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableDragAndUpAfterModelShrink verifies that a mouse drag or mouse up whose interaction row was captured at
// mouse down does not panic (and does not replay the event into a stale cell) when the model shrinks mid-gesture,
// such as when a cell callback mutates the model and calls SyncToModel before the drag or up event arrives.
func TestTableDragAndUpAfterModelShrink(t *testing.T) {
	c := check.New(t)
	var downCount, dragCount, upCount int
	makeRow := func(id string) *tableTestRow {
		row := newTableTestRow(id)
		panel := unison.NewPanel()
		panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			downCount++
			return false
		}
		panel.MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			dragCount++
			return true
		}
		panel.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			upCount++
			return true
		}
		row.cellFactory = func(_, _ int) unison.Paneler { return panel }
		return row
	}
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows([]*tableTestRow{makeRow("A"), makeRow("B"), makeRow("C")})
	table := unison.NewTable[*tableTestRow](model)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100}}
	table.SyncToModel()

	// Sanity check: a normal down/drag/up gesture over the last row reaches the cell's callbacks.
	pt := table.CellFrame(2, 0).Point.Add(geom.NewPoint(1, 1))
	table.DefaultMouseDown(pt, unison.ButtonLeft, 1, 0)
	c.Equal(1, downCount)
	table.DefaultMouseDrag(pt, unison.ButtonLeft, 0)
	c.Equal(1, dragCount)
	table.DefaultMouseUp(pt, unison.ButtonLeft, 0)
	c.Equal(1, upCount)

	// Start a gesture on the last row, then shrink the model out from under it before the drag and up arrive. The
	// stale interaction row must not be replayed into the now out-of-range row cache.
	table.DefaultMouseDown(pt, unison.ButtonLeft, 1, 0)
	c.Equal(2, downCount)
	model.SetRootRows([]*tableTestRow{makeRow("D")})
	table.SyncToModel()
	table.DefaultMouseDrag(pt, unison.ButtonLeft, 0)
	c.Equal(1, dragCount)
	table.DefaultMouseUp(pt, unison.ButtonLeft, 0)
	c.Equal(1, upCount)
}

// TestTableMouseUpClearsColumnResizeState verifies that a mouse up ends a column-resize interaction. Only the
// interaction row used to be reset, so a drag arriving after the release — which happens whenever a second button is
// still down, since the window keeps routing drags to the table — re-entered the column-resize branch with stale
// resize state and overwrote the column's width with a garbage value.
func TestTableMouseUpClearsColumnResizeState(t *testing.T) {
	c := check.New(t)
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows([]*tableTestRow{newTableTestRow("A"), newTableTestRow("B")})
	table := unison.NewTable[*tableTestRow](model)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
	table.SyncToModel()

	// Grab the divider between the two columns and drag it 20 to the right.
	divider := geom.NewPoint(100, 5)
	c.Equal(0, table.OverColumnDivider(divider.X), "expected a divider at the column boundary")
	table.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0)
	table.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
	c.Equal(float32(120), table.Columns[0].Current)

	// Release, then let a drag from the rest of a multi-button gesture arrive. The resize is over, so the column must
	// keep the width the completed drag left it at.
	table.DefaultMouseUp(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
	table.DefaultMouseDrag(geom.NewPoint(400, 5), unison.ButtonLeft, 0)
	c.Equal(float32(120), table.Columns[0].Current, "a drag after the release must not resize the column")
}

// TestTableDisclosureRequiresPressOnSameHitRect verifies that a disclosure hit rect's handler only fires when the
// mouse-down began on that same hit rect. A gesture that starts elsewhere (e.g. a row selection drag) and releases
// over a disclosure triangle must not toggle it.
func TestTableDisclosureRequiresPressOnSameHitRect(t *testing.T) {
	c := check.New(t)
	parentA := newTableTestRow("A")
	parentA.SetChildren([]*tableTestRow{newTableTestRow("a0")})
	parentA.SetOpen(false)
	parentB := newTableTestRow("B")
	parentB.SetChildren([]*tableTestRow{newTableTestRow("b0")})
	parentB.SetOpen(false)
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows([]*tableTestRow{parentA, parentB})
	table := unison.NewTable[*tableTestRow](model)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100}}
	table.SyncToModel()

	rectA := table.RowFrame(0)
	rectA.Size = geom.NewSize(12, 12)
	rectB := table.RowFrame(1)
	rectB.Size = geom.NewSize(12, 12)
	addHitRects := func() {
		table.AddHitRectForTest(rectA, parentA)
		table.AddHitRectForTest(rectB, parentB)
	}
	addHitRects()

	// A press that starts on row B outside any hit rect, drags, and releases over row A's disclosure triangle must not
	// toggle it.
	table.DefaultMouseDown(geom.NewPoint(50, rectB.CenterY()), unison.ButtonLeft, 1, 0)
	table.DefaultMouseDrag(rectA.Center(), unison.ButtonLeft, 0)
	table.DefaultMouseUp(rectA.Center(), unison.ButtonLeft, 0)
	c.False(parentA.IsOpen())
	c.False(parentB.IsOpen())

	// A press that starts on row A's disclosure triangle but releases over row B's must toggle neither.
	table.DefaultMouseDown(rectA.Center(), unison.ButtonLeft, 1, 0)
	table.DefaultMouseUp(rectB.Center(), unison.ButtonLeft, 0)
	c.False(parentA.IsOpen())
	c.False(parentB.IsOpen())

	// A press that starts on the disclosure triangle and releases outside it must not toggle.
	table.DefaultMouseDown(rectA.Center(), unison.ButtonLeft, 1, 0)
	table.DefaultMouseUp(geom.NewPoint(50, rectA.CenterY()), unison.ButtonLeft, 0)
	c.False(parentA.IsOpen())

	// A press and release on the same disclosure triangle must toggle it, even if drag events occurred in between.
	table.DefaultMouseDown(rectA.Center(), unison.ButtonLeft, 1, 0)
	table.DefaultMouseDrag(rectA.Center().Add(geom.NewPoint(1, 1)), unison.ButtonLeft, 0)
	table.DefaultMouseUp(rectA.Center(), unison.ButtonLeft, 0)
	c.True(parentA.IsOpen())
	c.False(parentB.IsOpen())

	// The toggle resynced the model (and a real redraw would rebuild the hit rects), so re-inject them and verify a
	// second same-rect gesture closes it again.
	addHitRects()
	table.DefaultMouseDown(rectA.Center(), unison.ButtonLeft, 1, 0)
	table.DefaultMouseUp(rectA.Center(), unison.ButtonLeft, 0)
	c.False(parentA.IsOpen())
}

// TestTableMouseExitAfterColumnRemoval verifies that removing columns while the mouse is over a cell doesn't leave the
// exit path addressing a column that no longer exists. The row bound was checked but the column bound wasn't, so the
// exit path handed an out-of-range column index to the row's ColumnCell, which panics in typical implementations.
func TestTableMouseExitAfterColumnRemoval(t *testing.T) {
	c := check.New(t)
	exits := 0
	visibleColumns := 2
	panel := unison.NewPanel()
	panel.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool { return true }
	panel.MouseExitCallback = func() bool { exits++; return true }
	row := newTableTestRow("A")
	row.cellFactory = func(_, col int) unison.Paneler {
		// Stand in for the usual row implementation, which indexes its own per-column data.
		if col < 0 || col >= visibleColumns {
			panic("column index out of range")
		}
		return panel
	}
	model := &unison.SimpleTableModel[*tableTestRow]{}
	model.SetRootRows([]*tableTestRow{row})
	table := unison.NewTable[*tableTestRow](model)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
	table.SyncToModel()

	// Sanity check: entering and exiting a cell that stays put reaches the cell's exit callback.
	over0 := table.CellFrame(0, 0).Point.Add(geom.NewPoint(1, 1))
	table.DefaultMouseEnter(over0, 0)
	table.DefaultMouseExit()
	c.Equal(1, exits)

	// Now hover the last column and drop it out from under the mouse before the exit arrives.
	over1 := table.CellFrame(0, 1).Point.Add(geom.NewPoint(1, 1))
	table.DefaultMouseEnter(over1, 0)
	table.Columns = table.Columns[:1]
	visibleColumns = 1
	table.SyncToModel()
	table.DefaultMouseExit()
	c.Equal(1, exits, "the exit must not be delivered through a column that no longer exists")

	// The table remains usable afterward.
	table.DefaultMouseEnter(over0, 0)
	table.DefaultMouseExit()
	c.Equal(2, exits)
}

// TestTableColumnWithOnlyMinimumIsResizable verifies that a column with a positive Minimum and no Maximum can be
// resized. The gate treated a Maximum of 0 as a hard bound, while the resize clamping treats it as "no maximum", so
// such a column could never be resized at all.
func TestTableColumnWithOnlyMinimumIsResizable(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(2)...)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100, Minimum: 50}, {ID: 1, Current: 100}}
	table.SyncToModel()

	divider := geom.NewPoint(100, 5)
	c.Equal(0, table.OverColumnDivider(divider.X), "expected a divider at the column boundary")
	table.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0)
	table.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
	c.Equal(float32(120), table.Columns[0].Current)

	// The Minimum is still honored: dragging far to the left stops at the minimum plus the cell padding overhead.
	table.DefaultMouseDrag(geom.NewPoint(-500, 5), unison.ButtonLeft, 0)
	c.Equal(50+table.Padding.Left+table.Padding.Right, table.Columns[0].Current)
	table.DefaultMouseUp(geom.NewPoint(-500, 5), unison.ButtonLeft, 0)
}

// TestTableColumnPinnedToOneWidthIsNotResizable verifies that a column whose Minimum and Maximum pin it to a single
// width still refuses to be resized.
func TestTableColumnPinnedToOneWidthIsNotResizable(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(2)...)
	table.Columns = []unison.ColumnInfo{{ID: 0, Current: 100, Minimum: 100, Maximum: 100}, {ID: 1, Current: 100}}
	table.SyncToModel()

	divider := geom.NewPoint(100, 5)
	c.Equal(0, table.OverColumnDivider(divider.X), "expected a divider at the column boundary")
	table.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0)
	table.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
	c.Equal(float32(100), table.Columns[0].Current)
	table.DefaultMouseUp(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
}

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

// TestEmptyTableWithRowDividerSizes verifies that an empty table with ShowRowDivider enabled reports a non-negative
// preferred and minimum height, rather than the -1 that used to result from counting dividers for an empty row range.
func TestEmptyTableWithRowDividerSizes(t *testing.T) {
	c := check.New(t)
	table := newTestTable()
	table.ShowRowDivider = true
	minSize, prefSize, _ := table.DefaultSizes(geom.Size{})
	c.True(minSize.Height >= 0, "minimum height %v must not be negative", minSize.Height)
	c.True(prefSize.Height >= 0, "preferred height %v must not be negative", prefSize.Height)
}

// TestTableAccessibilityPressOpensRow verifies that pressing a row runs the double-click callback, which is the table's
// "open this row" gesture and was unreachable from an assistive technology, and that the table itself offers no press
// of its own: the default behavior would click the center of the table's whole frame, replacing the selection with
// whatever row happened to sit there.
func TestTableAccessibilityPressOpensRow(t *testing.T) {
	c := check.New(t)
	table := newTestTable(flatRows(3)...)
	table.Columns = append(table.Columns, unison.ColumnInfo{ID: 0, Current: 100})
	table.SyncToModel()
	opens := 0
	changes := 0
	table.DoubleClickCallback = func() { opens++ }
	table.SelectionChangedCallback = func() { changes++ }

	c.True(table.PerformAccessibilityAction(accessibility.ActionRequest{
		Key:    table.RowFromIndex(1).ID(),
		Action: accessibility.Press,
	}))
	c.Equal(1, opens, "pressing a row opens it")
	c.True(table.IsRowSelected(1), "the row the callback acts on has to be selected")
	c.Equal(1, table.SelectionCount())
	c.Equal(1, changes)

	// Pressing the row that is already selected leaves the selection alone.
	c.True(table.PerformAccessibilityAction(accessibility.ActionRequest{
		Key:    table.RowFromIndex(1).ID(),
		Action: accessibility.Press,
	}))
	c.Equal(2, opens)
	c.Equal(1, changes, "a row that was already selected is not selected again")

	// A table with nothing to open refuses the press rather than reporting that it did something.
	table.DoubleClickCallback = nil
	c.False(table.PerformAccessibilityAction(accessibility.ActionRequest{
		Key:    table.RowFromIndex(0).ID(),
		Action: accessibility.Press,
	}))
}

// TestTableAccessibilitySelectSetsTheAnchor verifies that selecting a row through an assistive technology makes it the
// anchor a later shift-click extends from, exactly as a plain click does. The selection map used to be replaced on its
// own, leaving the anchor on a row that might not even be selected any more, so the shift-click that followed extended
// from somewhere the person had never been.
func TestTableAccessibilitySelectSetsTheAnchor(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(5)...)
			// An anchor left on a row that is no longer even selected, as an earlier click and a deselection
			// would have left one.
			table.SelectByIndex(4)
			table.DeselectByIndex(4)
			wnd = newHeadlessWindow(t, "anchor", geom.NewRect(10, 10, 400, 400), axColumn(table))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	row := axTableRows(tree, node)["r1"]
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.Select,
	}))
	c.Equal([]int{1}, axTableSelectedIndexes(screen, table))

	// A shift-click three rows down now extends from the row the assistive technology selected.
	screen.Do(func() {
		// Aimed well inside the first column: the center of a row falls on a column divider, which a press starts a
		// column resize from rather than reaching the row at all.
		frame := table.RowFrame(3)
		where := geom.NewPoint(frame.X+10, frame.CenterY())
		table.DefaultMouseDown(where, unison.ButtonLeft, 1, mod.Shift)
		table.DefaultMouseUp(where, unison.ButtonLeft, mod.Shift)
	})
	c.Equal([]int{1, 2, 3}, axTableSelectedIndexes(screen, table),
		"the shift-click should have extended from the row that was selected")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axTableSelectedIndexes returns the indexes of the rows a table currently has selected, in order, read on the thread
// that owns the table.
func axTableSelectedIndexes[T unison.TableRowConstraint[T]](screen *unison.HeadlessScreen,
	table *unison.Table[T],
) []int {
	var indexes []int
	screen.Do(func() {
		for i := range table.LastRowIndex() + 1 {
			if table.IsRowSelected(i) {
				indexes = append(indexes, i)
			}
		}
	})
	return indexes
}

// TestTableAccessibilityRowPressIsOfferedOnlyWhenItDoesSomething verifies what a table and its rows offer to do: the
// table itself is not something to press, and a row offers a press only when the table has been given a double-click
// callback for it to run.
func TestTableAccessibilityRowPressIsOfferedOnlyWhenItDoesSomething(t *testing.T) {
	c := check.New(t)
	var quiet, openable *unison.Table[*tableTestRow]
	var wnd *unison.Window
	opens := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			quiet = axNewTable(flatRows(2)...)
			openable = axNewTable(newTableTestRow("o0"), newTableTestRow("o1"))
			openable.DoubleClickCallback = func() { opens++ }
			wnd = newHeadlessWindow(t, "row press", geom.NewRect(10, 10, 400, 400),
				axColumn(quiet, openable))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	quietNode := screen.AccessibilityNodeFor(quiet)
	c.True(quietNode != nil)
	c.False(quietNode.Actions.Has(accessibility.Press),
		"pressing the table would select whatever row sits in the middle of its frame")
	quietRows := axTableRows(tree, quietNode)
	c.True(quietRows["r0"] != nil)
	if quietRows["r0"] != nil {
		c.False(quietRows["r0"].Actions.Has(accessibility.Press), "nothing has been given to open these rows with")
	}

	openableNode := screen.AccessibilityNodeFor(openable)
	c.True(openableNode != nil)
	c.False(openableNode.Actions.Has(accessibility.Press))
	rows := axTableRows(tree, openableNode)
	row := rows["o1"]
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Actions.Has(accessibility.Press), "the table has something to open its rows with")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = opens })
	c.Equal(1, count, "pressing the row should have opened it")
	var selected bool
	screen.Do(func() { selected = openable.IsRowSelected(1) })
	c.True(selected, "the row the callback acts on has to be selected")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCellActsOnContentThatCanRespond verifies that a request aimed at a cell reaches something that
// can carry it out: a disabled widget is passed over rather than refusing the request on behalf of the enabled one
// behind it, and a toggle goes to the widget that has a state to move on rather than to a button that would refuse it.
func TestTableAccessibilityCellActsOnContentThatCanRespond(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	var behindDisabled, besideButton *unison.CheckBox
	disabledClicks := 0
	buttonClicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			behindDisabled = unison.NewCheckBox()
			behindDisabled.ClickAnimationTime = 0
			behindDisabled.SetTitle("Behind")
			besideButton = unison.NewCheckBox()
			besideButton.ClickAnimationTime = 0
			besideButton.SetTitle("Beside")
			row := newTableTestRow("r0")
			row.cellFactory = func(_, col int) unison.Paneler {
				wrapper := unison.NewPanel()
				wrapper.SetLayout(&unison.FlexLayout{Columns: 2, HSpacing: unison.StdHSpacing})
				button := unison.NewButton()
				button.ClickAnimationTime = 0
				var box *unison.CheckBox
				if col == 0 {
					button.SetTitle("Off")
					button.ClickCallback = func() { disabledClicks++ }
					button.SetEnabled(false)
					box = behindDisabled
				} else {
					button.SetTitle("On")
					button.ClickCallback = func() { buttonClicks++ }
					box = besideButton
				}
				for _, child := range []unison.Paneler{button, box} {
					child.AsPanel().SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Middle})
					wrapper.AddChild(child)
				}
				return wrapper
			}
			table = axNewTable(row)
			wnd = newHeadlessWindow(t, "cell targets", geom.NewRect(10, 10, 500, 200), table)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	rows := axChildNodes(tree, node)
	c.Equal(1, len(rows))
	if len(rows) != 1 {
		return
	}
	cells := axChildNodes(tree, rows[0])
	c.Equal(2, len(cells))
	if len(cells) != 2 {
		return
	}
	c.True(cells[0].Actions.Has(accessibility.Press), "the enabled check box behind the disabled button can be pressed")
	c.True(cells[0].Actions.Has(accessibility.Toggle))

	// The disabled button comes first in drawing order, but a click would pass straight over it, so the press has to
	// land on the check box behind it rather than being refused.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[0].ID,
		Action: accessibility.Press,
	}))
	var state checkenum.Enum
	var clicks int
	screen.Do(func() {
		state = behindDisabled.State
		clicks = disabledClicks
	})
	c.Equal(checkenum.On, state, "the press should have reached the check box behind the disabled button")
	c.Equal(0, clicks, "a disabled button must not be clicked")

	// A toggle offered by the check box must go to the check box, not to the enabled button ahead of it, which has no
	// state to move on and would refuse it.
	c.True(cells[1].Actions.Has(accessibility.Toggle))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[1].ID,
		Action: accessibility.Toggle,
	}))
	screen.Do(func() {
		state = besideButton.State
		clicks = buttonClicks
	})
	c.Equal(checkenum.On, state, "the toggle should have reached the check box")
	c.Equal(0, clicks, "a button cannot be toggled, so it must not have been asked to be")

	// A press on that same cell still goes to the first thing that can take one, which is the button.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[1].ID,
		Action: accessibility.Press,
	}))
	screen.Do(func() { clicks = buttonClicks })
	c.Equal(1, clicks, "the press should have reached the button")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityLongTableKeepsTheSelectionInSight verifies that stopping the walk through a very large table
// does not drop a selected row that lies far below what can be seen, since what is selected is worth describing however
// far out of view it is.
func TestTableAccessibilityLongTableKeepsTheSelectionInSight(t *testing.T) {
	c := check.New(t)
	const (
		rowCount    = 500
		selectedRow = 400
	)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows := make([]*tableTestRow, rowCount)
			for i := range rows {
				rows[i] = newTableTestRow("r" + strconv.Itoa(i))
			}
			table = axNewTable(rows...)
			table.SelectByIndex(selectedRow)
			wnd = newHeadlessWindow(t, "long table", geom.NewRect(10, 10, 400, 400),
				axColumn(axScroller(table, geom.NewSize(300, 100))))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	c.Equal(rowCount, node.RowCount)
	described := axChildNodes(tree, node)
	c.True(len(described) > 0 && len(described) < 40,
		"only the rows worth describing should have been described, got %d", len(described))
	selected := axTableRows(tree, node)["r"+strconv.Itoa(selectedRow)]
	c.True(selected != nil, "the selected row is described however far out of sight it is")
	if selected != nil {
		c.True(selected.Selected)
		c.True(selected.Offscreen)
	}
	c.True(axTableRows(tree, node)["r"+strconv.Itoa(rowCount-1)] == nil,
		"a row that is neither visible nor selected is not described at all")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axIndexOfChild returns where a node sits among a parent's children, or -1 if it is not one of them.
func axIndexOfChild(parent, child *accessibility.Node) int {
	if parent == nil || child == nil {
		return -1
	}
	for i, id := range parent.Children {
		if id == child.ID {
			return i
		}
	}
	return -1
}

// TestTableAccessibilityOutOfSequenceRowsBringTheirAncestors verifies that a row described although the row before it
// was not arrives with the rows it hangs beneath. An assistive technology reads the nesting of a table from the levels
// of the rows and the order they come in, so a selected row that turned up far below everything else described would
// otherwise be read as a child of whatever container happened to precede it. Such a row is described by name alone,
// since nobody can see it: building its cells would mean asking the model for a panel per column for every row of a
// selection that may run to hundreds.
func TestTableAccessibilityOutOfSequenceRowsBringTheirAncestors(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			first := newTableTestRow("first")
			firstChildren := make([]*tableTestRow, 60)
			for i := range firstChildren {
				firstChildren[i] = newTableTestRow("a" + strconv.Itoa(i))
			}
			first.SetChildren(firstChildren)
			first.SetOpen(true)
			second := newTableTestRow("second")
			second.SetChildren([]*tableTestRow{
				newTableTestRow("b0"),
				newTableTestRow("b1"),
				newTableTestRow("b2"),
			})
			second.SetOpen(true)
			table = axNewTable(first, second)
			// The third child of the second container, far below anything that can be seen and in a different part of
			// the model from the rows that can.
			table.SelectByIndex(64)
			wnd = newHeadlessWindow(t, "ancestors", geom.NewRect(10, 10, 400, 400),
				axColumn(axScroller(table, geom.NewSize(300, 100))))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	if node == nil {
		return
	}
	rows := axTableRows(tree, node)
	selected := rows["b2"]
	c.True(selected != nil, "the selected row is described however far out of sight it is")
	if selected == nil {
		return
	}
	c.True(selected.Selected)
	c.True(selected.Offscreen)
	c.Equal(2, selected.Level, "it is a child of the container it belongs to")
	c.Equal(0, len(selected.Children), "a row nobody can see is described by name alone, without its cells")

	container := rows["second"]
	c.True(container != nil, "the container the selected row hangs beneath is described along with it")
	if container == nil {
		return
	}
	c.Equal(1, container.Level)
	c.True(axIndexOfChild(node, container) < axIndexOfChild(node, selected),
		"a container has to come before the rows it discloses")
	c.True(rows["b0"] == nil, "a row that is neither visible nor selected nor an ancestor is not described")

	// A row that can be seen is described in full, cells and all.
	visible := rows["first"]
	c.True(visible != nil)
	if visible != nil {
		c.True(len(visible.Children) > 0, "a visible row still holds its disclosure triangle and cells")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCellReachesContentUnderADisabledWrapper verifies that a cell holding a disabled panel with an
// enabled widget inside it offers what that widget can do and then does it. Being enabled is a panel's own property
// rather than something it passes down, so a click lands on the widget just as it would if nothing around it were
// disabled, and a request from an assistive technology must reach it too rather than being advertised and then refused.
func TestTableAccessibilityCellReachesContentUnderADisabledWrapper(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	var box *unison.CheckBox
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			box = unison.NewCheckBox()
			box.ClickAnimationTime = 0
			box.SetTitle("Done")
			row := newTableTestRow("r0")
			row.cellFactory = func(_, col int) unison.Paneler {
				if col != 0 {
					return unison.NewPanel()
				}
				wrapper := unison.NewPanel()
				wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
				wrapper.SetEnabled(false)
				box.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Middle})
				wrapper.AddChild(box)
				return wrapper
			}
			table = axNewTable(row)
			wnd = newHeadlessWindow(t, "disabled wrapper", geom.NewRect(10, 10, 500, 200), table)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	rows := axChildNodes(tree, node)
	c.Equal(1, len(rows))
	if len(rows) != 1 {
		return
	}
	cells := axChildNodes(tree, rows[0])
	c.True(len(cells) > 0)
	if len(cells) == 0 {
		return
	}
	c.True(cells[0].Actions.Has(accessibility.Press), "the cell offers the press its content can take")
	c.True(cells[0].Actions.Has(accessibility.Toggle))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[0].ID,
		Action: accessibility.Press,
	}), "what the cell offered must be something it will actually do")
	var state checkenum.Enum
	screen.Do(func() { state = box.State })
	c.Equal(checkenum.On, state, "the press should have reached the check box inside the disabled wrapper")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCellPressLandsWhereAClickWould verifies that a press a cell passes on reaches the widget a
// click would have landed on. A panel is drawn before its children, so the widgets within a cell sit on top of the
// panel holding them; a cell panel that handles both halves of a click itself must therefore be offered the press after
// what it holds rather than before it, or a click on the button in a cell would do one thing and a press of that same
// cell another.
func TestTableAccessibilityCellPressLandsWhereAClickWould(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	wrapperPresses := 0
	buttonClicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			row := newTableTestRow("r0")
			row.cellFactory = func(_, col int) unison.Paneler {
				wrapper := unison.NewPanel()
				wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
				wrapper.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
				wrapper.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
					wrapperPresses++
					return true
				}
				if col != 0 {
					return wrapper
				}
				button := unison.NewButton()
				button.ClickAnimationTime = 0
				button.SetTitle("Go")
				button.ClickCallback = func() { buttonClicks++ }
				button.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Start, VAlign: align.Middle})
				wrapper.AddChild(button)
				return wrapper
			}
			table = axNewTable(row)
			wnd = newHeadlessWindow(t, "cell order", geom.NewRect(10, 10, 500, 200), table)
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(table)
	c.True(node != nil)
	rows := axChildNodes(tree, node)
	c.Equal(1, len(rows))
	if len(rows) != 1 {
		return
	}
	cells := axChildNodes(tree, rows[0])
	c.True(len(cells) > 0)
	if len(cells) == 0 {
		return
	}
	c.True(cells[0].Actions.Has(accessibility.Press))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   cells[0].ID,
		Action: accessibility.Press,
	}))
	var clicks, presses int
	screen.Do(func() {
		clicks = buttonClicks
		presses = wrapperPresses
	})
	c.Equal(1, clicks, "the press should have reached the button drawn on top of the cell's own panel")
	c.Equal(0, presses, "the panel beneath it must not have taken the press first")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCellTooltipDoesNotDescribeTheTable verifies that the tooltip a table borrows from the cell the
// pointer is over stays out of what the table itself is described as. The borrowed tooltip used to be parked in
// Panel.Tooltip, which is what a node's description falls back to, so the table was announced with one cell's tooltip
// as its own description — and, since the window asks for a tooltip only while the pointer is within the table, went
// on being announced with it long after the pointer had left. Moving from one cell to the next reported a description
// change for the table each time besides. The tooltip itself must still appear, since that is what the borrowing is
// for.
func TestTableAccessibilityCellTooltipDoesNotDescribeTheTable(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var away *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows := make([]*tableTestRow, 2)
			for i := range rows {
				rows[i] = newTableTestRow("r" + strconv.Itoa(i))
				tip := unison.NewTooltipWithText("tip " + strconv.Itoa(i))
				rows[i].cellFactory = func(_, col int) unison.Paneler {
					cell := unison.NewPanel()
					if col == 0 {
						cell.Tooltip = tip
						// Without this the tooltip would not appear until the delay a person's pause has to last.
						cell.TooltipImmediate = true
					}
					return cell
				}
			}
			table = axNewTable(rows...)
			// Somewhere outside the table for the pointer to move on to, since the defect was about what was left
			// behind once it had. It sits above the table so that a tooltip, which is shown below the cell it belongs
			// to, can never be what the pointer lands on instead.
			away = unison.NewLabel()
			away.SetTitle("Away")
			wnd = newHeadlessWindow(t, "cell tips", geom.NewRect(10, 10, 400, 300), axColumn(away, table))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	tree := screen.AccessibilityTree(wnd)
	tableNode := axMustNode(c, screen.AccessibilityNodeFor(table))
	tableID := tableNode.ID
	c.Equal("", tableNode.Description, "nothing has been hovered yet")
	c.Equal(0, len(axNodesWithRole(tree, role.Tooltip)), "no tooltip is showing yet")
	screen.AccessibilityEvents(wnd)

	var overFirst, overSecond geom.Point
	c.True(screen.Do(func() {
		// Aimed well inside the first column: the center of a cell can fall on a column divider.
		first := table.CellFrame(0, 0)
		second := table.CellFrame(1, 0)
		overFirst = geom.NewPoint(first.X+5, first.CenterY())
		overSecond = geom.NewPoint(second.X+5, second.CenterY())
	}))

	// The second row is hovered first and the first row after it, since the tooltip for a cell is shown directly
	// beneath that cell: going the other way would put the tooltip itself under the pointer.
	screen.MouseMove(screen.PanelPoint(table, overSecond), mod.None)
	tree = screen.AccessibilityTree(wnd)
	tips := axNodesWithRole(tree, role.Tooltip)
	c.Equal(1, len(tips), "the cell's tooltip should be showing")
	if len(tips) == 1 {
		c.Equal("tip 1", tips[0].Name)
	}
	c.Equal("", axMustNode(c, screen.AccessibilityNodeFor(table)).Description,
		"the tooltip belongs to the cell the pointer is over, not to the table")
	c.False(axHasEvent(screen.AccessibilityEvents(wnd), accessibility.DescriptionChanged, tableID),
		"borrowing a cell's tooltip must not report that the table's description changed")

	// Onto a cell in the row above, whose tooltip says something else.
	screen.MouseMove(screen.PanelPoint(table, overFirst), mod.None)
	tree = screen.AccessibilityTree(wnd)
	tips = axNodesWithRole(tree, role.Tooltip)
	c.Equal(1, len(tips), "the other cell's tooltip should be showing now")
	if len(tips) == 1 {
		c.Equal("tip 0", tips[0].Name)
	}
	c.Equal("", axMustNode(c, screen.AccessibilityNodeFor(table)).Description)
	c.False(axHasEvent(screen.AccessibilityEvents(wnd), accessibility.DescriptionChanged, tableID),
		"moving from cell to cell must not report a description change for the table")

	// Out of the table altogether, which is where the borrowed tooltip used to stick.
	screen.MouseMove(screen.PanelCenter(away), mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(0, len(axNodesWithRole(tree, role.Tooltip)), "the tooltip goes away with the pointer")
	c.Equal("", axMustNode(c, screen.AccessibilityNodeFor(table)).Description,
		"the table must not be left described as the cell the pointer last crossed")
	c.False(axHasEvent(screen.AccessibilityEvents(wnd), accessibility.DescriptionChanged, tableID))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axTableWindow is what the focus tests below drive: a table, another control to move the focus away to, and a menu
// bar with one menu, which is what the displacement test opens.
type axTableWindow struct {
	screen  *unison.HeadlessScreen
	wnd     *unison.Window
	table   *unison.Table[*tableTestRow]
	other   *unison.Panel
	tableID accessibility.NodeID
}

// newAXTableWindow starts a session showing the given rows in a table, with nothing selected and nothing holding the
// keyboard focus yet.
func newAXTableWindow(t *testing.T, title string, rows ...*tableTestRow) *axTableWindow {
	t.Helper()
	const (
		menuID = unison.UserBaseID + iota
		cutID
	)
	out := &axTableWindow{}
	out.screen = startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			out.table = axNewTable(rows...)
			out.other = axFocusablePanel("Other")
			out.wnd = newHeadlessWindow(t, title, geom.NewRect(10, 10, 400, 400),
				axColumn(out.table, out.other))
			if out.wnd == nil {
				return
			}
			unison.DefaultMenuFactory().BarForWindow(out.wnd, func(bar unison.Menu) {
				f := bar.Factory()
				edit := f.NewMenu(menuID, "Edit", nil)
				edit.InsertItem(-1, f.NewItem(cutID, "Cut", unison.KeyBinding{}, nil, nil))
				bar.InsertMenu(-1, edit)
			})
			out.wnd.ToFront()
		}))
	out.screen.Sync()
	out.screen.EnableAccessibility()
	if node := out.screen.AccessibilityNodeFor(out.table); node != nil {
		out.tableID = node.ID
	}
	return out
}

// tree describes the window as it is now.
func (a *axTableWindow) tree() *accessibility.Tree {
	return a.screen.AccessibilityTree(a.wnd)
}

// rowNode returns the node describing the row with the given name in the given tree, or nil if it was not described.
func (a *axTableWindow) rowNode(tree *accessibility.Tree, name string) *accessibility.Node {
	return axTableRows(tree, tree.Node(a.tableID))[name]
}

// axTableRowPoint returns the screen point to aim at to hit a row well inside its first column. The center of a row
// falls on a column divider, which a press starts a column resize from rather than reaching the row at all.
func axTableRowPoint(screen *unison.HeadlessScreen, table *unison.Table[*tableTestRow], row int) geom.Point {
	var offset geom.Point
	screen.Do(func() {
		frame := table.RowFrame(row)
		offset = geom.NewPoint(frame.X+10, frame.CenterY()).Sub(table.ContentRect(false).Point)
	})
	return screen.PanelPoint(table, offset)
}

// TestTableAccessibilityFocusFollowsTheCurrentRow verifies that a table reports the keyboard focus on the row the
// person is on, and moves it with every gesture that moves the selection. Reporting it on the table itself and
// publishing nothing but a selection change per arrow key is what left a screen reader sitting on the table, silent,
// while the person moved through it.
func TestTableAccessibilityFocusFollowsTheCurrentRow(t *testing.T) {
	c := check.New(t)
	a := newAXTableWindow(t, "table focus", flatRows(6)...)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() { a.table.RequestFocus() })
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal(a.tableID, tree.Focus, "a table with nothing selected keeps the focus on itself")

	// Down with nothing selected lands on the first row, which is where the person now is.
	previous := tree
	a.screen.KeyPress(unison.KeyDown, mod.None)
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, "r0")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "the focus must be reported on the row the arrow key landed on")
	c.True(row.Focused)
	c.True(row.Focusable)
	c.True(row.Selected)
	c.True(row.Actions.Has(accessibility.Focus), "a row that can be moved to has to offer the move")
	c.False(tree.Node(a.tableID).Focused, "the table itself stops reporting the focus it handed to the row")
	c.Equal(1, axFocusedCount(tree), "exactly one node other than the root may report the focus")
	c.True(axHasEvent(accessibility.Diff(previous, tree), accessibility.FocusChanged, row.ID),
		"the move has to reach an assistive technology as a focus change naming the row")
	var lead int
	a.screen.Do(func() { lead = a.table.LeadRowIndex() })
	c.Equal(0, lead)

	// And again, to the row after it.
	a.screen.KeyPress(unison.KeyDown, mod.None)
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, "r1")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus)
	c.True(a.rowNode(tree, "r0") != nil && !a.rowNode(tree, "r0").Focused, "the row that was left gives the focus up")

	// Shift-Down adds the row below to the selection and moves the person onto it.
	a.screen.KeyPress(unison.KeyDown, mod.Shift)
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal([]int{1, 2}, axTableSelectedIndexes(a.screen, a.table), "shift-down extends the selection")
	row = a.rowNode(tree, "r2")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "the row the selection was extended onto is where the person is")

	// A click puts the person on the row that was clicked.
	a.screen.Click(axTableRowPoint(a.screen, a.table, 4))
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal([]int{4}, axTableSelectedIndexes(a.screen, a.table))
	row = a.rowNode(tree, "r4")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.Equal(row.ID, tree.Focus, "a click moves the reported focus onto the row it landed on")

	// With the selection gone there is no row to be on, so the table takes the focus back.
	a.screen.Do(func() { a.table.ClearSelection() })
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.Equal(a.tableID, tree.Focus, "with nothing selected the focus goes back to the table")
	c.True(tree.Node(a.tableID).Focused)
	c.Equal(1, axFocusedCount(tree))
	a.screen.Do(func() { lead = a.table.LeadRowIndex() })
	c.Equal(-1, lead, "clearing the selection leaves no row to be on")
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestTableAccessibilityFocusActionSelectsTheRow verifies that a request to put the focus on a row does what an arrow
// key onto that row does: the table takes the keyboard focus and the row becomes the whole of the selection, which is
// what the focus being reported on a row means.
func TestTableAccessibilityFocusActionSelectsTheRow(t *testing.T) {
	c := check.New(t)
	a := newAXTableWindow(t, "table focus action", flatRows(6)...)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() {
		a.table.SelectByIndex(0)
		a.wnd.SetFocus(a.other)
	})
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, "r3")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Actions.Has(accessibility.Focus))
	c.True(a.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   row.ID,
		Action: accessibility.Focus,
	}), "a row offering the focus must be able to take it")
	c.Equal([]int{3}, axTableSelectedIndexes(a.screen, a.table),
		"moving onto a row makes it the whole of the selection")
	var holdsFocus bool
	a.screen.Do(func() { holdsFocus = a.table.Is(a.wnd.CurrentFocus()) })
	c.True(holdsFocus, "the table is the one tab stop, so the keyboard focus goes there")

	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, "r3")
	c.True(row != nil && row.Focused)
	if row != nil {
		c.Equal(row.ID, tree.Focus)
	}
	c.Equal(1, axFocusedCount(tree))

	// A cell is where the person is whenever the cell cursor is on it, so it offers the focus and takes one: doing so
	// puts the cursor on that cell, which is what the left and right arrows do.
	cells := axChildNodes(tree, row)
	c.True(len(cells) != 0)
	if len(cells) != 0 {
		c.True(cells[1].Focusable, "a cell the cursor can stand on has to be focusable")
		c.True(cells[1].Actions.Has(accessibility.Focus), "a cell the cursor can stand on has to offer the move")
		var handled bool
		a.screen.Do(func() {
			handled = a.table.PerformAccessibilityAction(accessibility.ActionRequest{
				Key:    accessibility.CellKey{Row: "r3", Col: 1},
				Action: accessibility.Focus,
			})
		})
		c.True(handled, "a focus request naming a cell must move the cursor onto it")
		var leadRow, leadCol int
		a.screen.Do(func() {
			leadRow = a.table.LeadRowIndex()
			leadCol = a.table.LeadColumnIndex()
		})
		c.Equal(3, leadRow)
		c.Equal(1, leadCol)
		c.Equal([]int{3}, axTableSelectedIndexes(a.screen, a.table), "the row named by the cell is the selection")

		// And the focus is now reported on the cell rather than on the row it belongs to.
		tree = a.tree()
		c.True(tree != nil)
		if tree == nil {
			return
		}
		row = a.rowNode(tree, "r3")
		c.True(row != nil)
		if row == nil {
			return
		}
		cells = axChildNodes(tree, row)
		c.Equal(2, len(cells))
		if len(cells) == 2 {
			c.Equal(cells[1].ID, tree.Focus, "the cell the cursor is on is where the person is")
			c.True(cells[1].Focused)
			c.False(row.Focused, "the row gives the focus up to the cell within it")
			c.Equal(1, axFocusedCount(tree))
		}

		// Putting the focus back on the row puts the person back at row level.
		c.True(a.screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   row.ID,
			Action: accessibility.Focus,
		}))
		a.screen.Do(func() { leadCol = a.table.LeadColumnIndex() })
		c.Equal(-1, leadCol, "moving onto the row leaves the cells of it")
	}
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestTableAccessibilityFocusDisplacedByAnOpenMenu verifies that an open menu takes the focus away from a table's
// current row exactly as it takes it away from any other control.
func TestTableAccessibilityFocusDisplacedByAnOpenMenu(t *testing.T) {
	c := check.New(t)
	a := newAXTableWindow(t, "table focus menu", flatRows(6)...)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() {
		a.table.SelectByIndex(1)
		a.table.RequestFocus()
	})
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, "r1")
	c.True(row != nil && row.Focused)
	title := axNamed(tree, "Edit")
	c.True(title != nil)
	if title == nil {
		return
	}

	a.screen.Click(axScreenPoint(a.screen, a.wnd, title))
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row = a.rowNode(tree, "r1")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.False(row.Focused, "an open menu displaces the focus a table handed to its row")
	c.False(tree.Node(a.tableID).Focused)
	c.True(tree.Focus != row.ID && tree.Focus != a.tableID, "the focus must be reported within the menu")
	c.Equal(1, axFocusedCount(tree))
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestTableAccessibilityFocusFallsBackWhenTheCurrentRowGoes verifies what happens to the row the focus is reported on
// when the table stops showing it — its container was closed, or the model no longer holds it: the first row of what
// is still selected takes over, and the focus is never left on a row that is not there.
func TestTableAccessibilityFocusFallsBackWhenTheCurrentRowGoes(t *testing.T) {
	c := check.New(t)
	parent := newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{newTableTestRow("c0"), newTableTestRow("c1")})
	parent.SetOpen(true)
	a := newAXTableWindow(t, "table focus fallback", parent, newTableTestRow("r9"))
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() {
		a.table.SelectByIndex(0, 2)
		a.table.RequestFocus()
	})
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	child := a.rowNode(tree, "c1")
	c.True(child != nil)
	if child == nil {
		return
	}
	c.Equal(child.ID, tree.Focus, "the last row selected is the one the person is on")

	// Closing the container takes the row the focus was on out of the table entirely.
	a.screen.Do(func() {
		parent.SetOpen(false)
		a.table.SyncToModel()
	})
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.True(a.rowNode(tree, "c1") == nil, "a row the table no longer shows is not described")
	parentNode := a.rowNode(tree, "p")
	c.True(parentNode != nil)
	if parentNode == nil {
		return
	}
	c.Equal(parentNode.ID, tree.Focus, "the focus falls back to the first row still selected")
	var lead int
	a.screen.Do(func() { lead = a.table.LeadRowIndex() })
	c.Equal(-1, lead, "the row the person was on is let go of once the table stops showing it")

	// And the same when the model itself stops holding the row.
	a.screen.Do(func() {
		a.table.SelectByIndex(1)
		a.table.Model.SetRootRows([]*tableTestRow{parent})
		a.table.SyncToModel()
	})
	tree = a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	c.True(a.rowNode(tree, "r9") == nil)
	parentNode = a.rowNode(tree, "p")
	c.True(parentNode != nil)
	if parentNode == nil {
		return
	}
	c.Equal(parentNode.ID, tree.Focus, "the focus falls back to the first row still selected")
	c.Equal(1, axFocusedCount(tree))
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestTableAccessibilityDisabledTableReportsNoRow verifies that a disabled table hands the focus to nothing: every
// request about one of its rows would be refused, so a row is no place to say the person is.
func TestTableAccessibilityDisabledTableReportsNoRow(t *testing.T) {
	c := check.New(t)
	a := newAXTableWindow(t, "table focus disabled", flatRows(4)...)
	c.NotNil(a.wnd)
	if a.wnd == nil {
		return
	}
	a.screen.Do(func() {
		a.table.SelectByIndex(1)
		a.table.RequestFocus()
		a.table.SetEnabled(false)
	})
	tree := a.tree()
	c.True(tree != nil)
	if tree == nil {
		return
	}
	row := a.rowNode(tree, "r1")
	c.True(row != nil)
	if row == nil {
		return
	}
	c.True(row.Disabled, "the rows of a disabled table are disabled too")
	c.False(row.Focused, "a row of a disabled table is no place to report the focus")
	c.False(row.Actions.Has(accessibility.Focus), "and nothing may be asked of it")
	c.True(tree.Focus != row.ID)
	c.Equal(0, len(a.screen.Errors()), "nothing should have panicked: %v", a.screen.Errors())
}

// TestTableAccessibilityEditedCellReportsTheEditor verifies that while a cell is being edited the focus is reported on
// the widget the person is actually typing in, rather than on a row: that widget is a real panel of the table for as
// long as it holds the focus, and it is described as holding it by the ordinary path.
func TestTableAccessibilityEditedCellReportsTheEditor(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(4, 2, "typed", false)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	var focused bool
	screen.Do(func() {
		e.table.SelectByIndex(0)
		focused = e.table.FocusCell(2, 1)
	})
	c.True(focused, "the field in the cell should have taken the focus")

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	if tree == nil {
		return
	}
	field := screen.AccessibilityNodeFor(e.fields[2][1])
	c.True(field != nil)
	if field == nil {
		return
	}
	c.Equal(field.ID, tree.Focus, "the widget being typed in is where the person is")
	c.True(field.Focused)
	for _, row := range axNodesWithRole(tree, role.Row) {
		c.False(row.Focused, "no row may claim a focus the cell's own widget holds: %s", row.Name)
	}
	c.Equal(1, axFocusedCount(tree))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityRowNameIsTheWholeRow verifies what a row is called: the whole of it, column by column, with
// each column after the first announced with the title of the header standing over it, so that a person arrowing down
// the rows hears everything the row shows rather than only its first column. A column holding nothing for the row is
// left out, and a table with no header attached has no titles to give.
func TestTableAccessibilityRowNameIsTheWholeRow(t *testing.T) {
	c := check.New(t)
	var withHeader, bare *unison.Table[*tableTestRow]
	var wnd *unison.Window
	data := func(col int) string {
		switch col {
		case 0:
			return "report.docx"
		case 1:
			return "9/1/2024"
		default:
			return ""
		}
	}
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			named := newTableTestRow("r0")
			named.cellData = data
			withHeader = newTestTable(named)
			withHeader.Columns = []unison.ColumnInfo{
				{ID: 0, Current: 100},
				{ID: 1, Current: 100},
				{ID: 2, Current: 100},
			}
			withHeader.SyncToModel()
			unison.NewTableHeader(withHeader,
				unison.TableColumnHeader[*tableTestRow](unison.NewTableColumnHeader[*tableTestRow]("Name", "", nil)),
				unison.NewTableColumnHeader[*tableTestRow]("Modified", "", nil),
				unison.NewTableColumnHeader[*tableTestRow]("Size", "", nil))

			plain := newTableTestRow("r1")
			plain.cellData = data
			bare = newTestTable(plain)
			bare.Columns = []unison.ColumnInfo{
				{ID: 0, Current: 100},
				{ID: 1, Current: 100},
				{ID: 2, Current: 100},
			}
			bare.SyncToModel()

			wnd = newHeadlessWindow(t, "row names", geom.NewRect(10, 10, 500, 400), axColumn(withHeader, bare))
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}

	tree := screen.AccessibilityTree(wnd)
	rows := axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(withHeader)))
	c.Equal(1, len(rows))
	if len(rows) == 1 {
		c.Equal("report.docx, Modified 9/1/2024", rows[0].Name,
			"the first column stands alone and the rest are announced with their titles; an empty column is left out")
	}
	rows = axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(bare)))
	c.Equal(1, len(rows))
	if len(rows) == 1 {
		c.Equal("report.docx, 9/1/2024", rows[0].Name,
			"a table with no header has no titles to announce its columns with")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axTextRow is a row whose sort text carries a marker ahead of the words a cell shows, as a row that must sort its
// containers first or its kinds in a fixed order does, and which hands the bare words to an assistive technology
// through TableRowAccessibleText.
type axTextRow struct {
	parent *axTextRow
	id     tid.TID
}

func (r *axTextRow) CloneForTarget(_ unison.Paneler, newParent *axTextRow) *axTextRow {
	return &axTextRow{id: r.id, parent: newParent}
}
func (r *axTextRow) ID() tid.TID                 { return r.id }
func (r *axTextRow) Parent() *axTextRow          { return r.parent }
func (r *axTextRow) SetParent(parent *axTextRow) { r.parent = parent }
func (r *axTextRow) CanHaveChildren() bool       { return false }
func (r *axTextRow) Children() []*axTextRow      { return nil }
func (r *axTextRow) SetChildren(_ []*axTextRow)  {}
func (r *axTextRow) IsOpen() bool                { return false }
func (r *axTextRow) SetOpen(_ bool)              {}
func (r *axTextRow) ColumnCell(_, _ int, _, _ unison.Ink, _, _, _ bool) unison.Paneler {
	return unison.NewPanel()
}

func (r *axTextRow) CellDataForSort(col int) string {
	return "1/" + r.CellDataForAccessibility(col)
}

func (r *axTextRow) CellDataForAccessibility(col int) string {
	switch col {
	case 0:
		return "report.docx"
	case 1:
		return "9/1/2024"
	default:
		return ""
	}
}

// TestTableAccessibilityRowNameComesFromAccessibleText verifies that a row which says what its cells should be called
// (see TableRowAccessibleText) has those words used for the row and for each cell, rather than the sort text with
// whatever ordering marker the row put ahead of it.
func TestTableAccessibilityRowNameComesFromAccessibleText(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*axTextRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			model := &unison.SimpleTableModel[*axTextRow]{}
			model.SetRootRows([]*axTextRow{{id: "r0"}})
			table = unison.NewTable[*axTextRow](model)
			table.Columns = []unison.ColumnInfo{
				{ID: 0, Current: 100},
				{ID: 1, Current: 100},
				{ID: 2, Current: 100},
			}
			table.SyncToModel()
			unison.NewTableHeader(table,
				unison.TableColumnHeader[*axTextRow](unison.NewTableColumnHeader[*axTextRow]("Name", "", nil)),
				unison.NewTableColumnHeader[*axTextRow]("Modified", "", nil),
				unison.NewTableColumnHeader[*axTextRow]("Size", "", nil))
			wnd = newHeadlessWindow(t, "accessible text", geom.NewRect(10, 10, 500, 400), axColumn(table))
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}

	tree := screen.AccessibilityTree(wnd)
	rows := axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))
	c.Equal(1, len(rows))
	if len(rows) != 1 {
		return
	}
	c.Equal("report.docx, Modified 9/1/2024", rows[0].Name,
		"the row is named from the words the row hands over, not from the sort text and its marker")
	var cells []string
	for _, child := range axChildNodes(tree, rows[0]) {
		if child.Role == role.Cell {
			cells = append(cells, child.Name)
		}
	}
	c.Equal([]string{"report.docx", "9/1/2024", ""}, cells,
		"each cell is named from the words the row hands over, not from the sort text and its marker")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityCursorFocusesTheCellOutOfView verifies that a row the person is on, however far out of sight it
// is, is described with the cells it holds while the cell cursor is on one of them, and that the focus is reported on
// that cell: a cursor the reader cannot be pointed at is a cursor nothing reads out.
func TestTableAccessibilityCursorFocusesTheCellOutOfView(t *testing.T) {
	c := check.New(t)
	const (
		rowCount = 400
		onRow    = 300
	)
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			rows := make([]*tableTestRow, rowCount)
			for i := range rows {
				rows[i] = newTableTestRow("r" + strconv.Itoa(i))
			}
			table = axNewTable(rows...)
			wnd = newHeadlessWindow(t, "cursor out of view", geom.NewRect(10, 10, 400, 400),
				axColumn(axScroller(table, geom.NewSize(300, 100))))
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
		// Put the person on a row far below the view port without scrolling to it, so that the row the cursor stands
		// on is one nobody can see.
		table.SelectByIndex(onRow)
		table.SetLeadCell(onRow, 1)
		table.ScrollRowIntoView(0)
	})

	tree := screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(table))
	row := axTableRows(tree, node)["r"+strconv.Itoa(onRow)]
	c.True(row != nil, "the row the person is on is described however far out of sight it is")
	if row == nil {
		return
	}
	c.True(row.Offscreen)
	cells := axChildNodes(tree, row)
	c.Equal(2, len(cells), "the row the cursor is in has to be described with its cells for the cursor to land on one")
	if len(cells) != 2 {
		return
	}
	c.Equal(cells[1].ID, tree.Focus, "the focus is reported on the cell the cursor is on")
	c.True(cells[1].Focused)
	c.False(row.Focused, "the row hands the focus on to the cell within it")
	c.Equal(1, axFocusedCount(tree), "exactly one node other than the root may report the focus")

	// Back out to the row, and the focus is reported on the row again. Putting the cursor on the row scrolls to it, so
	// the view is taken back to the top to leave the row out of sight, which is where it is described by name alone.
	screen.Do(func() {
		table.SetLeadCell(onRow, -1)
		table.ScrollRowIntoView(0)
	})
	tree = screen.AccessibilityTree(wnd)
	row = axTableRows(tree, axMustNode(c, screen.AccessibilityNodeFor(table)))["r"+strconv.Itoa(onRow)]
	c.True(row != nil)
	if row != nil {
		c.Equal(row.ID, tree.Focus, "at row level the row is where the person is")
		c.True(row.Focused)
		c.Equal(0, len(row.Children), "a row nobody can see is described by name alone again")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableAccessibilityFocusInCellSelectsTheRow verifies the route a screen reader takes into an editable cell: it
// asks for the focus on the widget the cell holds rather than on the cell itself, which reaches the widget through the
// table and leaves it holding the keyboard focus. That is the person moving into that cell, so the row it belongs to
// becomes the whole of the selection and the cell cursor lands on the column, exactly as a click into the widget or a
// call to FocusCell does.
func TestTableAccessibilityFocusInCellSelectsTheRow(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			e.table.SelectByIndex(0)
			wnd, _ = newEditWindow(t, e, false)
		}))
	c.NotNil(wnd)
	if wnd == nil {
		return
	}
	screen.EnableAccessibility()

	tree := screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(e.table))
	rows := axChildNodes(tree, node)
	c.Equal(3, len(rows))
	if len(rows) != 3 {
		return
	}
	cells := axChildNodes(tree, rows[2])
	c.Equal(3, len(cells), "each row is described with one node per column")
	if len(cells) != 3 {
		return
	}
	content := axChildNodes(tree, cells[1])
	c.Equal(1, len(content), "the field the cell holds is described within it")
	if len(content) != 1 {
		return
	}

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   content[0].ID,
		Action: accessibility.Focus,
	}))
	s := e.snapshot(c, screen, 2, 1)
	c.True(s.fieldFocused, "the request should have left the field holding the focus")
	c.Equal(2, s.focusRow)
	c.Equal(1, s.focusCol)
	var indexes []int
	var row, col int
	c.True(screen.Do(func() {
		indexes = selectedTableIndexes(e.table)
		row = e.table.LeadRowIndex()
		col = e.table.LeadColumnIndex()
	}))
	c.Equal([]int{2}, indexes, "the row the cell belongs to becomes the whole of the selection")
	c.Equal(2, row)
	c.Equal(1, col, "and the cursor stands on the cell being worked in")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableCellFieldCommitRemovingItsRowLetsTheFocusLeave verifies that a cell whose field commits on losing the focus,
// and whose commit removes its row, is released without the table taking the focus back: the focus is on its way to
// where the person put it, and that move stands.
func TestTableCellFieldCommitRemovingItsRowLetsTheFocusLeave(t *testing.T) {
	c := check.New(t)
	var e *editTable
	var wnd *unison.Window
	var button *unison.Button
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			e = newEditTable(3, 2, "abc", false)
			wnd, button = newEditWindow(t, e, true)
		}))
	c.NotNil(wnd)
	c.NotNil(button)
	screen.Click(cellCenter(screen, e.table, 0, 0))
	s := e.snapshot(c, screen, 0, 0)
	c.True(s.fieldFocused, "the test needs the field focused")
	var onButton, gone bool
	c.True(screen.Do(func() {
		lost := e.fields[0][0].LostFocusCallback
		e.fields[0][0].LostFocusCallback = func() {
			lost()
			e.model.SetRootRows(e.rows[1:])
			e.table.SyncToModel()
		}
		wnd.SetFocus(button)
		onButton = button.Focused()
		gone = e.table.RowToIndex(e.rows[0]) == -1
	}))
	c.True(gone, "the test needs the commit to have removed the row")
	c.True(onButton, "the focus lands where it was sent")
	s = e.snapshot(c, screen, 0, 0)
	c.False(s.tableFocused, "the table does not take back a focus that is already leaving")
	c.Equal(1, s.lost)
	c.Equal(-1, s.focusRow)
	c.Equal(-1, s.focusCol)
	c.True(s.detached)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
