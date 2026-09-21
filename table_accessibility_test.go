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
	"github.com/richardwilkes/unison/enums/role"
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
