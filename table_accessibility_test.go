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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/mod"
)

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
