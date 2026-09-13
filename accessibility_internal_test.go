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
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests reach the parts of the accessibility core that a widget uses rather than an application: the builder a
// widget describes itself with, the virtual children a collection invents for rows it has no panels for, and the
// geometry the platforms that work in screen pixels convert with. A session owns most of the package's mutable globals
// while it runs, so none of these may call t.Parallel.

const (
	// axTestRowHeight is how tall each of axTestList's rows is, in its own coordinates.
	axTestRowHeight = 10
	// axTestColumns is how many cells each of axTestList's rows holds.
	axTestColumns = 2
)

// axTestCellKey identifies one cell of axTestList, standing in for the accessibility.CellKey a real table uses.
type axTestCellKey struct {
	row int
	col int
}

// axTestList is a widget that has rows but no panel per row, which is how a list or a table is built. It describes each
// row as a virtual child, keyed by the row's index so that the same row keeps the same node id however the rows around it
// change, and gives each row cells of its own so that more than one level of virtual children is exercised.
type axTestList struct {
	window *Window
	rows   []string
	acted  []accessibility.ActionRequest
	Panel
	visible geom.Rect
	focused bool
}

// newAXTestList creates a list of numbered rows.
func newAXTestList(count int) *axTestList {
	l := &axTestList{}
	l.Self = l
	l.rows = make([]string, count)
	for i := range l.rows {
		l.rows[i] = "row " + strconv.Itoa(i)
	}
	l.SetSizer(l.sizes)
	return l
}

func (l *axTestList) sizes(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
	size := geom.NewSize(100, axTestRowHeight*float32(len(l.rows)))
	return size, size, size
}

// ProvideAccessibility describes the list and each of its rows.
func (l *axTestList) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	node.Role = role.List
	node.Name = "Rows"
	node.RowCount = len(l.rows)
	node.ColumnCount = axTestColumns
	node.Multiselectable = true
	// Recorded so the test can check that the builder answers these for the panel being described.
	l.visible = b.VisibleRect()
	l.window = b.Window()
	l.focused = b.Focused()
	for i, text := range l.rows {
		rowID := b.AddVirtualChild(i, func(n *accessibility.Node) {
			n.Role = role.ListItem
			n.Name = text
			n.RowIndex = i
			n.Bounds = geom.NewRect(0, axTestRowHeight*float32(i), 100, axTestRowHeight)
			n.Actions = n.Actions.With(accessibility.Select)
		})
		for col := range axTestColumns {
			b.AddVirtualChildOf(rowID, axTestCellKey{row: i, col: col}, func(n *accessibility.Node) {
				n.Role = role.Cell
				n.Name = text
				n.ColumnIndex = col
				n.Bounds = geom.NewRect(50*float32(col), axTestRowHeight*float32(i), 50, axTestRowHeight)
			})
		}
	}
}

// PerformAccessibilityAction records the selections it is asked for and refuses everything else.
func (l *axTestList) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	if req.Action != accessibility.Select {
		return false
	}
	l.acted = append(l.acted, req)
	return true
}

// axNewTestWindow creates a window whose content is the given panel, laid out to fill it, and shows it.
func axNewTestWindow(t *testing.T, title string, rect geom.Rect, panel Paneler) *Window {
	t.Helper()
	w, err := NewWindow(title)
	if err != nil {
		t.Errorf("unable to create window %q: %v", title, err)
		return nil
	}
	w.Content().SetLayout(&FlexLayout{
		Columns:  1,
		HSpacing: StdHSpacing,
		VSpacing: StdVSpacing,
	})
	w.Content().AddChild(panel)
	w.SetContentRect(rect)
	w.Show()
	return w
}

// TestAccessibilityVirtualChildren verifies what a collection widget can do: describe rows it has no panels for, have
// each row keep its identity from one description to the next, receive requests aimed at one particular row, and have the
// ids of rows it has stopped using reclaimed.
func TestAccessibilityVirtualChildren(t *testing.T) {
	c := check.New(t)
	const initialRows = 30
	var list *axTestList
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 500},
		StartupFinishedCallback(func() {
			list = newAXTestList(initialRows)
			wnd = axNewTestWindow(t, "virtual", geom.NewRect(10, 10, 300, 480), list)
		}))
	c.NotNil(list)
	c.NotNil(wnd)

	tree := screen.AccessibilityTree(wnd)
	c.True(tree != nil)
	listNode := screen.AccessibilityNodeFor(list)
	c.True(listNode != nil)
	c.Equal(role.List, listNode.Role)
	c.Equal("Rows", listNode.Name)
	c.Equal(initialRows, listNode.RowCount)
	c.Equal(initialRows, len(listNode.Children), "every row should be a child of the list")

	firstRow := tree.Node(listNode.Children[0])
	c.True(firstRow != nil)
	c.Equal(role.ListItem, firstRow.Role)
	c.Equal("row 0", firstRow.Name)
	c.Equal(listNode.ID, firstRow.Parent)
	c.Equal(listNode.Bounds.Point, firstRow.Bounds.Point,
		"a virtual child's bounds should have been converted out of the panel's own coordinates")
	c.Equal(float32(axTestRowHeight), firstRow.Bounds.Height)
	c.Equal(axTestColumns, len(firstRow.Children), "the row's cells should be children of the row")
	cell := tree.Node(firstRow.Children[0])
	c.True(cell != nil)
	c.Equal(role.Cell, cell.Role)
	c.Equal(firstRow.ID, cell.Parent)

	var visible geom.Rect
	var describedWindow *Window
	var focusedWhenDescribed bool
	screen.Do(func() {
		visible = list.visible
		describedWindow = list.window
		focusedWhenDescribed = list.focused
	})
	c.False(visible.Empty(), "the list is on the screen, so some of it is visible")
	c.True(describedWindow == wnd, "the builder should report the window being described")
	c.False(focusedWhenDescribed, "nothing gave the list the focus")

	// Asking again must reuse the ids, since an assistive technology's notion of a row has to survive the snapshot it
	// was found in.
	screen.AccessibilityTree(wnd)
	again := screen.AccessibilityNodeFor(list)
	c.True(again != nil)
	c.Equal(listNode.Children, again.Children, "the rows should have kept their ids")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstRow.ID,
		Action: accessibility.Select,
	}), "the list should have handled the selection")
	var acted []accessibility.ActionRequest
	screen.Do(func() { acted = list.acted })
	c.Equal(1, len(acted))
	if len(acted) == 1 {
		c.Equal(0, acted[0].Key, "the request should name the row's own key")
		c.Equal(firstRow.ID, acted[0].Node)
	}
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   firstRow.ID,
		Action: accessibility.Focus,
	}), "a virtual child gets none of the default behaviors, since those act on a panel")

	// Dropping most of the rows leaves the ids of the ones that are gone to be reclaimed, which happens the next time the
	// list is described. Each row accounts for one id of its own plus one per cell.
	screen.Do(func() { list.rows = list.rows[:3] })
	screen.AccessibilityTree(wnd)
	var held int
	screen.Do(func() { held = len(list.Accessibility.virtual) })
	c.Equal(3*(1+axTestColumns), held, "the ids of the rows that are gone should have been reclaimed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityGeometry verifies the conversion the platforms whose accessibility APIs work in physical screen
// pixels apply to the window-local, logical-unit bounds a tree holds.
func TestAccessibilityGeometry(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600, Scale: 2},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "geometry", geom.NewRect(20, 30, 200, 100), NewPanel())
		}))
	c.NotNil(wnd)

	var origin, scale, contentOrigin geom.Point
	screen.Do(func() {
		origin, scale = wnd.accessibilityGeometry()
		contentOrigin = wnd.ContentRect().Point
		// Nothing observable follows from this on a headless window, but the wrapper is what the platform windows call
		// when they move, resize or change backing scale, so it must survive being called.
		wnd.apiAccessibilityGeometryChanged()
	})
	c.Equal(geom.NewPoint(2, 2), scale, "the scale is the window's backing scale")
	c.Equal(geom.NewPoint(20, 30), contentOrigin)
	c.Equal(contentOrigin.MulPt(scale), origin, "the origin is the content area's position in physical pixels")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestAccessibilityDeactivationReleasesState verifies that turning accessibility support off drops everything it was
// holding, so that a screen reader going away costs an application nothing thereafter.
func TestAccessibilityDeactivationReleasesState(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			wnd = axNewTestWindow(t, "deactivate", geom.NewRect(20, 20, 240, 120), NewPanel())
		}))
	c.NotNil(wnd)

	c.True(screen.AccessibilityTree(wnd) != nil)
	var held bool
	screen.Do(func() { held = wnd.ax != nil })
	c.True(held, "the window should be holding the description it published")

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	screen.Do(func() { held = wnd.ax != nil })
	c.False(held, "deactivation should have freed what the window was holding")
	var tree *accessibility.Tree
	screen.Do(func() {
		if hw := headlessWindowFor(wnd); hw != nil {
			tree = hw.axTree
		}
	})
	c.True(tree == nil, "the adapter should have been shut down")

	// Redrawing must not describe anything now that support is off.
	var before, after uint64
	screen.Do(func() {
		before = axSnapshotCount
		wnd.MarkForRedraw()
	})
	screen.Do(func() { after = axSnapshotCount })
	c.Equal(before, after, "no snapshot may be built while accessibility support is off")
}
