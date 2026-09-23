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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// testColumnHeader is a minimal TableColumnHeader built on a plain panel, avoiding the font work a label-based header
// would require.
type testColumnHeader struct {
	*unison.Panel
	state unison.SortState
}

func newTestColumnHeader() *testColumnHeader {
	return &testColumnHeader{Panel: unison.NewPanel()}
}

func (h *testColumnHeader) SortState() unison.SortState         { return h.state }
func (h *testColumnHeader) SetSortState(state unison.SortState) { h.state = state }
func (h *testColumnHeader) Less() func(a, b string) bool        { return nil }

// newHeaderTestTable builds a table with two 100-wide columns.
func newHeaderTestTable() *unison.Table[*tableTestRow] {
	table := newTestTable(flatRows(2)...)
	table.Columns = append(table.Columns,
		unison.ColumnInfo{ID: 0, Current: 100},
		unison.ColumnInfo{ID: 1, Current: 100},
	)
	table.SetFrameRect(geom.NewRect(0, 0, 300, 300))
	return table
}

func TestTableHeaderToleratesFewerHeadersThanColumns(t *testing.T) {
	c := check.New(t)
	table := newHeaderTestTable()
	// The sizing and drawing paths tolerate fewer column headers than columns, so the interaction paths must too. This
	// header has no column headers at all.
	header := unison.NewTableHeader[*tableTestRow](table)
	header.SetFrameRect(geom.NewRect(0, 0, 300, 20))

	pt := geom.NewPoint(50, 10) // Over column 0, away from any divider
	c.True(header.DefaultUpdateCursorCallback(pt) == nil)
	c.True(header.DefaultUpdateTooltipCallback(pt, geom.Rect{}).Empty())
	c.False(header.DefaultMouseMove(pt, 0))
	c.True(header.DefaultMouseDown(pt, unison.ButtonLeft, 1, 0))
	c.False(header.DefaultMouseUp(pt, unison.ButtonLeft, 0))
}

func TestTableHeaderStillDispatchesToPresentHeaders(t *testing.T) {
	c := check.New(t)
	table := newHeaderTestTable()
	colHeader := newTestColumnHeader()
	downCalls := 0
	upCalls := 0
	moveCalls := 0
	colHeader.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { downCalls++; return true }
	colHeader.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool { upCalls++; return true }
	colHeader.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool { moveCalls++; return true }
	// One header for two columns: column 0 dispatches to it, column 1 has no header and must be ignored.
	header := unison.NewTableHeader(table, unison.TableColumnHeader[*tableTestRow](colHeader))
	header.SetFrameRect(geom.NewRect(0, 0, 300, 20))

	over0 := geom.NewPoint(50, 10)
	c.True(header.DefaultMouseMove(over0, 0))
	c.True(header.DefaultMouseDown(over0, unison.ButtonLeft, 1, 0))
	c.True(header.DefaultMouseUp(over0, unison.ButtonLeft, 0))
	c.Equal(1, moveCalls)
	c.Equal(1, downCalls)
	c.Equal(1, upCalls)

	over1 := geom.NewPoint(150, 10)
	c.False(header.DefaultMouseMove(over1, 0))
	c.True(header.DefaultMouseDown(over1, unison.ButtonLeft, 1, 0))
	c.False(header.DefaultMouseUp(over1, unison.ButtonLeft, 0))
	c.Equal(1, moveCalls)
	c.Equal(1, downCalls)
	c.Equal(1, upCalls)
}

// TestTableHeaderColumnWithOnlyMinimumIsResizable verifies that the header's resize gate agrees with the table's: a
// column with a positive Minimum and no Maximum is resizable, while one pinned to a single width is not.
func TestTableHeaderColumnWithOnlyMinimumIsResizable(t *testing.T) {
	c := check.New(t)
	table := newHeaderTestTable()
	table.Columns[0].Minimum = 50
	header := unison.NewTableHeader[*tableTestRow](table)
	header.SetFrameRect(geom.NewRect(0, 0, 300, 20))

	divider := geom.NewPoint(100, 10)
	c.Equal(0, table.OverColumnDivider(divider.X), "expected a divider at the column boundary")
	c.True(header.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0))
	c.True(header.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0))
	c.Equal(float32(120), table.Columns[0].Current)
	header.DefaultMouseUp(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)

	// A column pinned to a single width is not resizable, so the press lands on the header instead of the divider.
	pinned := newHeaderTestTable()
	pinned.Columns[0].Minimum = 100
	pinned.Columns[0].Maximum = 100
	pinnedHeader := unison.NewTableHeader[*tableTestRow](pinned)
	pinnedHeader.SetFrameRect(geom.NewRect(0, 0, 300, 20))
	pinnedHeader.DefaultMouseDown(divider, unison.ButtonLeft, 1, 0)
	pinnedHeader.DefaultMouseDrag(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
	c.Equal(float32(100), pinned.Columns[0].Current)
	pinnedHeader.DefaultMouseUp(divider.Add(geom.NewPoint(20, 0)), unison.ButtonLeft, 0)
}

// TestTableHeaderSortsHierarchicalFilterInPlace verifies that a sort applied while a hierarchical filter is in force
// orders the rows the filter shows, at every level, without reordering the model behind them.
func TestTableHeaderSortsHierarchicalFilterInPlace(t *testing.T) {
	c := check.New(t)
	c1 := newTableTestRow("c1")
	c0 := newTableTestRow("c0")
	parent := newTableTestRow("p")
	parent.SetChildren([]*tableTestRow{c1, c0})
	a := newTableTestRow("a")
	table := newTestTable(parent, a)
	table.Columns = append(table.Columns, unison.ColumnInfo{ID: 0, Current: 100})
	colHeader := newTestColumnHeader()
	colHeader.SetSortState(unison.SortState{Order: 0, Ascending: true, Sortable: true})
	header := unison.NewTableHeader(table, unison.TableColumnHeader[*tableTestRow](colHeader))
	c.True(header.HasSort())

	table.ApplyHierarchicalFilter(func(_ *tableTestRow) bool { return false })
	c.Equal(4, table.LastRowIndex()+1)
	c.Equal(tid.TID("a"), table.RowFromIndex(0).ID(), "the root rows must be sorted")
	c.Equal(tid.TID("p"), table.RowFromIndex(1).ID())
	c.Equal(tid.TID("c0"), table.RowFromIndex(2).ID(), "the children the filter kept must be sorted too")
	c.Equal(tid.TID("c1"), table.RowFromIndex(3).ID())

	c.Equal(tid.TID("p"), table.Model.RootRows()[0].ID(), "the model's order must be left alone")
	c.Equal(tid.TID("a"), table.Model.RootRows()[1].ID())
	c.Equal(tid.TID("c1"), parent.Children()[0].ID(), "the model's children must be left alone")
	c.Equal(tid.TID("c0"), parent.Children()[1].ID())
}

// axCustomColumnHeader is a column header written the way the documentation describes one: it embeds a *unison.Label
// and points Self at itself, so it is neither the library's own header type nor a plain *unison.Label.
type axCustomColumnHeader struct {
	*unison.Label
	state unison.SortState
}

func newAxCustomColumnHeader(title string) *axCustomColumnHeader {
	h := &axCustomColumnHeader{Label: unison.NewLabel()}
	h.Self = h
	h.state = unison.SortState{Order: -1, Ascending: true, Sortable: true}
	h.SetTitle(title)
	return h
}

func (h *axCustomColumnHeader) SortState() unison.SortState         { return h.state }
func (h *axCustomColumnHeader) SetSortState(state unison.SortState) { h.state = state }
func (h *axCustomColumnHeader) Less() func(a, b string) bool        { return nil }

// TestTableHeaderAccessibilityNamesACustomColumnHeader verifies that a column header built around a label but not of
// the library's own type is still named by that label's text. The fallback used to insist on the concrete type, so a
// custom header matched neither it nor the check for a plain label and was handed to a screen reader with no name.
func TestTableHeaderAccessibilityNamesACustomColumnHeader(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var header *unison.TableHeader[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(2)...)
			header = unison.NewTableHeader[*tableTestRow](table,
				unison.TableColumnHeader[*tableTestRow](newAxCustomColumnHeader("Custom")),
				unison.NewTableColumnHeader[*tableTestRow]("Stock", "", nil))
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "custom header", geom.NewRect(10, 10, 400, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(header)
	c.True(node != nil)
	c.False(node.Actions.Has(accessibility.Press),
		"pressing the header would sort the table on whichever column sits in the middle of it")

	columns := axChildNodes(tree, node)
	c.Equal(2, len(columns))
	if len(columns) != 2 {
		return
	}
	c.Equal("Custom", columns[0].Name, "a header built around a label is named by that label's text")
	c.Equal("Stock", columns[1].Name)
	c.True(columns[0].Actions.Has(accessibility.Press), "a sortable column header can still be pressed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableHeaderAccessibilityColumnTooltipDoesNotDescribeTheHeader verifies that the tooltip a header borrows from
// the column header the pointer is over stays out of what the header itself is described as. The borrowed tooltip used
// to be parked in Panel.Tooltip, which is what a node's description falls back to, so the header was announced with
// one column's tooltip as its own description, and that description changed as the pointer moved along the header. The
// column it belongs to is described with it, which is where it was always meant to be heard, and the tooltip itself
// must still appear.
func TestTableHeaderAccessibilityColumnTooltipDoesNotDescribeTheHeader(t *testing.T) {
	c := check.New(t)
	var header *unison.TableHeader[*tableTestRow]
	var away *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table := axNewTable(flatRows(2)...)
			header = unison.NewTableHeader[*tableTestRow](table,
				unison.NewTableColumnHeader[*tableTestRow]("Name", "What the thing is called", nil),
				unison.NewTableColumnHeader[*tableTestRow]("Value", "", nil))
			// The header does not carry a column header's own immediate flag across, so without this the tooltip would
			// not appear until the delay a person's pause has to last, which is not something to wait on in a test.
			header.TooltipImmediate = true
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			// Somewhere outside the header for the pointer to move on to, since the defect was about what was left
			// behind once it had. It sits above the table so that a tooltip, which is shown below what it belongs to,
			// can never be what the pointer lands on instead.
			away = unison.NewLabel()
			away.SetTitle("Away")
			wnd = newHeadlessWindow(t, "column tips", geom.NewRect(10, 10, 400, 400), axColumn(away, scroller))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	tree := screen.AccessibilityTree(wnd)
	headerNode := axMustNode(c, screen.AccessibilityNodeFor(header))
	headerID := headerNode.ID
	c.Equal("", headerNode.Description, "nothing has been hovered yet")
	columns := axChildNodes(tree, headerNode)
	c.Equal(2, len(columns))
	if len(columns) != 2 {
		return
	}
	c.Equal("What the thing is called", columns[0].Description,
		"the tooltip describes the column it was given to")
	c.Equal("", columns[1].Description, "the column with no tooltip has nothing to say about itself")
	c.Equal(0, len(axNodesWithRole(tree, role.Tooltip)), "no tooltip is showing yet")
	screen.AccessibilityEvents(wnd)

	var overColumn geom.Point
	c.True(screen.Do(func() {
		// Aimed well inside the first column: a point within ColumnResizeSlop of a divider is a resize rather than the
		// column header itself.
		frame := header.ColumnFrame(0)
		overColumn = geom.NewPoint(frame.X+5, frame.CenterY())
	}))
	screen.MouseMove(screen.PanelPoint(header, overColumn), mod.None)

	tree = screen.AccessibilityTree(wnd)
	tips := axNodesWithRole(tree, role.Tooltip)
	c.Equal(1, len(tips), "the column header's tooltip should be showing")
	if len(tips) == 1 {
		c.Equal("What the thing is called", tips[0].Name)
	}
	c.Equal("", axMustNode(c, screen.AccessibilityNodeFor(header)).Description,
		"the tooltip belongs to the column the pointer is over, not to the header")
	c.False(axHasEvent(screen.AccessibilityEvents(wnd), accessibility.DescriptionChanged, headerID),
		"borrowing a column header's tooltip must not report that the header's description changed")

	// Off the header altogether, which is where the borrowed tooltip used to stick.
	screen.MouseMove(screen.PanelCenter(away), mod.None)
	tree = screen.AccessibilityTree(wnd)
	c.Equal(0, len(axNodesWithRole(tree, role.Tooltip)), "the tooltip goes away with the pointer")
	c.Equal("", axMustNode(c, screen.AccessibilityNodeFor(header)).Description,
		"the header must not be left described as the column the pointer last crossed")
	c.False(axHasEvent(screen.AccessibilityEvents(wnd), accessibility.DescriptionChanged, headerID))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTableHeaderAccessibilityActionKeepsTheFocusInAColumnHeader verifies that a request carried out inside a column
// header leaves the keyboard focus it moved in there reachable. A column header is installed only for as long as it
// takes to hand it whatever it is being given and is detached again afterwards; a focusable widget in one that took
// the focus while handling the request — which Focus does outright, and Press does on its way to the click it
// synthesizes — was left hanging off a panel with no parent, so the window could no longer find it, the description
// published immediately afterwards reported nothing focused, and the person's focus was silently gone until they
// pressed Tab.
func TestTableHeaderAccessibilityActionKeepsTheFocusInAColumnHeader(t *testing.T) {
	c := check.New(t)
	var header *unison.TableHeader[*tableTestRow]
	var colHeader *axButtonHeader
	var wnd *unison.Window
	clicks := 0
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table := axNewTable(flatRows(3)...)
			// A header built around something other than a label, so that what it holds is described and can be acted
			// on, and focusable, which is what makes the focus something to lose.
			colHeader = newAxButtonHeader("Pick", &clicks)
			header = unison.NewTableHeader[*tableTestRow](table,
				unison.TableColumnHeader[*tableTestRow](colHeader),
				unison.NewTableColumnHeader[*tableTestRow]("Value", "", nil))
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "header focus", geom.NewRect(10, 10, 400, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	tree := screen.AccessibilityTree(wnd)
	inside := axColumnHeaderContent(c, screen, tree, header)
	if inside == nil {
		return
	}
	c.True(inside.Focusable, "the widget the header is built around can take the focus")
	c.True(inside.Actions.Has(accessibility.Focus))

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   inside.ID,
		Action: accessibility.Focus,
	}))
	axCheckColumnHeaderHoldsFocus(c, screen, wnd, header, colHeader)

	// Pressing it focuses it as well, on the way to the click it synthesizes.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   inside.ID,
		Action: accessibility.Press,
	}))
	var count int
	screen.Do(func() { count = clicks })
	c.Equal(1, count, "pressing the button in the header should have clicked it")
	axCheckColumnHeaderHoldsFocus(c, screen, wnd, header, colHeader)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axColumnHeaderContent returns the one node describing what the header's first column header is built around, or nil
// if it was not described.
func axColumnHeaderContent(c check.Checker, screen *unison.HeadlessScreen, tree *accessibility.Tree,
	header *unison.TableHeader[*tableTestRow],
) *accessibility.Node {
	c.Helper()
	columns := axChildNodes(tree, axMustNode(c, screen.AccessibilityNodeFor(header)))
	if len(columns) == 0 {
		c.Error("the header's columns should have been described")
		return nil
	}
	inside := axUnignoredNodes(tree, columns[0])
	if len(inside) != 1 {
		c.Errorf("the widget the column header is built around should have been described, found %d", len(inside))
		return nil
	}
	return inside[0]
}

// axCheckColumnHeaderHoldsFocus asserts that the window's focus is still on the widget the first column header is
// built around, both as the window answers for it and as the next description reports it.
func axCheckColumnHeaderHoldsFocus(c check.Checker, screen *unison.HeadlessScreen, wnd *unison.Window,
	header *unison.TableHeader[*tableTestRow], colHeader unison.Paneler,
) {
	c.Helper()
	var focus *unison.Panel
	screen.Do(func() { focus = wnd.CurrentFocus() })
	c.True(focus != nil && focus.Is(colHeader),
		"the focus must still be reachable rather than left on a panel with no parent")
	tree := screen.AccessibilityTree(wnd)
	inside := axColumnHeaderContent(c, screen, tree, header)
	if inside == nil {
		return
	}
	c.True(inside.Focused, "the widget in the column header should be described as holding the focus")
	c.Equal(inside.ID, tree.Focus, "the description should report where the focus is")
}

// TestTableHeaderAccessibilityNamesAnIconOnlyColumnHeaderByItsTooltip verifies that a column header with no text — one
// whose title is an icon — is named by its tooltip, which is the only thing such a header has to say what the column
// holds. It used to be announced with no name at all, and the tooltip was offered only as a description, so the cells
// beneath it were read with no column title either.
func TestTableHeaderAccessibilityNamesAnIconOnlyColumnHeaderByItsTooltip(t *testing.T) {
	c := check.New(t)
	var table *unison.Table[*tableTestRow]
	var header *unison.TableHeader[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			table = axNewTable(flatRows(2)...)
			iconOnly := unison.NewTableColumnHeader[*tableTestRow]("", "Equipped", nil)
			iconOnly.Drawable = &unison.DrawableSVG{SVG: unison.CheckmarkSVG, Size: geom.NewSize(12, 12)}
			header = unison.NewTableHeader[*tableTestRow](table,
				unison.TableColumnHeader[*tableTestRow](iconOnly),
				unison.NewTableColumnHeader[*tableTestRow]("Stock", "What is in stock", nil))
			scroller := axScroller(table, geom.NewSize(300, 200))
			scroller.SetColumnHeader(header)
			wnd = newHeadlessWindow(t, "icon header", geom.NewRect(10, 10, 400, 400), axColumn(scroller))
		}))
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(header)
	c.True(node != nil)
	columns := axChildNodes(tree, node)
	c.Equal(2, len(columns))
	if len(columns) != 2 {
		return
	}
	c.Equal("Equipped", columns[0].Name, "a header with no text is named by its tooltip")
	c.Equal("", columns[0].Description, "and the tooltip is not repeated as its description")
	c.Equal("Stock", columns[1].Name, "a header with text keeps its text as its name")
	c.Equal("What is in stock", columns[1].Description, "and its tooltip as its description")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
