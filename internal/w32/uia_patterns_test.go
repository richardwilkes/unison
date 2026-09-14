// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover what the control patterns ask of a snapshot. They run on every platform, which is the point of
// keeping the questions in uia_patterns.go: the Windows-only tests then have only the COM left to check.

// patternTree builds a window holding one of every control that supports a pattern:
//
//	1 window                                   focused
//	├─  2 button        "Press me"
//	├─  3 button        "No"                    disabled
//	├─  4 check box     "Mixed"                 check mixed
//	├─  5 toggle button "Bold"                  pressed
//	├─  6 radio button  "First"                 checked
//	├─  7 radio button  "Second"
//	├─  8 text field    "Name"                  text "Gandalf", no separate value
//	├─  9 text field    "Secret"                protected
//	├─ 10 text field    "Fixed"                 value "fixed", read-only
//	├─ 11 slider                                5 of 0..10, step 1
//	├─ 12 slider                                disabled
//	├─ 13 progress bar                          3 of 0..10
//	├─ 14 popup button  "Red"                   expandable
//	├─ 15 combo box     "Text"                  not expandable, so a leaf
//	├─ 16 color well    "#ff0000"
//	└─ 17 label         "Just text"             supports no pattern at all
func patternTree() *accessibility.Tree {
	return newTestTree(1, 8,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 400),
			Children: []accessibility.NodeID{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17},
		},
		&accessibility.Node{
			ID: 2, Role: role.Button, Name: "Press me", Focusable: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{
			ID: 3, Role: role.Button, Name: "No", Disabled: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{
			ID: 4, Role: role.CheckBox, Name: "Mixed", HasCheck: true, Checked: checkenum.Mixed,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Toggle),
		},
		&accessibility.Node{
			ID: 5, Role: role.ToggleButton, Name: "Bold", Pressed: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Toggle),
		},
		&accessibility.Node{
			ID: 6, Role: role.RadioButton, Name: "First", HasCheck: true, Checked: checkenum.On,
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{
			ID: 7, Role: role.RadioButton, Name: "Second", HasCheck: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{
			ID: 8, Role: role.TextField, Name: "Name", Focusable: true, Focused: true,
			Text:    &accessibility.TextInfo{Text: "Gandalf", SelStart: 7, SelEnd: 7},
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetValue),
		},
		&accessibility.Node{
			ID: 9, Role: role.TextField, Name: "Secret", Protected: true, Value: "hunter2",
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue),
		},
		&accessibility.Node{
			ID: 10, Role: role.TextField, Name: "Fixed", Value: "fixed", ReadOnly: true,
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue),
		},
		&accessibility.Node{
			ID: 11, Role: role.Slider, HasNumber: true, Number: 5, Max: 10, Step: 1,
			Orientation: accessibility.OrientationHorizontal,
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue, accessibility.Increment,
				accessibility.Decrement),
		},
		&accessibility.Node{
			ID: 12, Role: role.Slider, Disabled: true, HasNumber: true, Number: 1, Max: 4, Step: 1,
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue),
		},
		&accessibility.Node{ID: 13, Role: role.ProgressBar, HasNumber: true, Number: 3, Max: 10, Step: 1},
		&accessibility.Node{
			ID: 14, Role: role.PopupButton, Name: "Colors", Value: "Red", Expandable: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Expand,
				accessibility.Collapse),
		},
		&accessibility.Node{
			ID: 15, Role: role.ComboBox, Name: "Style", Value: "Text",
			Actions: accessibility.ActionSet(0).With(accessibility.SetValue, accessibility.Expand,
				accessibility.Collapse),
		},
		&accessibility.Node{
			ID: 16, Role: role.ColorWell, Name: "Ink", Value: "#ff0000",
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{ID: 17, Role: role.Label, Name: "Just text"},
	)
}

// listTree builds the two selection containers whose items are not rows:
//
//	1 window
//	├─ 2 list                      multiselectable
//	│  ├─ 3 list item   row 0      selected
//	│  ├─ 4 group [ignored]
//	│  │  └─ 5 list item row 1     selected
//	│  └─ 6 list item   row 2
//	└─ 7 tab list
//	   ├─ 8 tab                    selected
//	   └─ 9 tab
//
// The ignored group is there so that a selection has to be gathered through it: an ignored node has no provider, so its
// children are what a client sees.
func listTree() *accessibility.Tree {
	itemActions := accessibility.ActionSet(0).With(accessibility.Select, accessibility.AddToSelection,
		accessibility.RemoveFromSelection, accessibility.ScrollIntoView)
	return newTestTree(1, 3,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{2, 7},
		},
		&accessibility.Node{
			ID: 2, Role: role.List, Name: "Choices", Multiselectable: true, RowCount: 3,
			Bounds: geom.NewRect(0, 0, 200, 100), Children: []accessibility.NodeID{3, 4, 6},
		},
		&accessibility.Node{
			ID: 3, Role: role.ListItem, Name: "One", RowIndex: 0, Selectable: true, Selected: true,
			Bounds: geom.NewRect(0, 0, 200, 20), Actions: itemActions,
		},
		&accessibility.Node{
			ID: 4, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 20, 200, 20),
			Children: []accessibility.NodeID{5},
		},
		&accessibility.Node{
			ID: 5, Role: role.ListItem, Name: "Two", RowIndex: 1, Selectable: true, Selected: true,
			Bounds: geom.NewRect(0, 20, 200, 20), Actions: itemActions,
		},
		&accessibility.Node{
			ID: 6, Role: role.ListItem, Name: "Three", RowIndex: 2, Selectable: true,
			Bounds: geom.NewRect(0, 40, 200, 20), Actions: itemActions,
		},
		&accessibility.Node{
			ID: 7, Role: role.TabList, Bounds: geom.NewRect(0, 100, 200, 100),
			Children: []accessibility.NodeID{8, 9},
		},
		&accessibility.Node{
			ID: 8, Role: role.Tab, Name: "First", Selectable: true, Selected: true,
			Bounds:  geom.NewRect(0, 100, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Select),
		},
		&accessibility.Node{
			ID: 9, Role: role.Tab, Name: "Second", Selectable: true, Bounds: geom.NewRect(100, 100, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Select),
		},
	)
}

// tableTree builds a table and the header that describes its columns, laid out the way a scroll panel lays them out:
//
//	1 window
//	└─ 2 scroll area
//	   ├─  3 table header
//	   │   ├─ 4 column header  column 0  "Name"
//	   │   └─ 5 column header  column 1  "Size"
//	   └─  6 table                       5 rows, 2 columns, multiselectable
//	       ├─ 7 row     row 1  selected, expandable and expanded
//	       │  ├─  8 cell  row 1 column 0
//	       │  └─  9 cell  row 1 column 1
//	       └─ 10 row    row 2
//	          ├─ 11 cell  row 2 column 0
//	          └─ 12 cell  row 2 column 1
//
// The header is nowhere inside the table, which is the situation UIATableColumnHeaders exists for. Rows 0, 3 and 4 are
// deliberately absent: a table publishes only the rows in its viewport, so most of a large one is not there to be
// handed over.
func tableTree() *accessibility.Tree {
	rowActions := accessibility.ActionSet(0).With(accessibility.Select, accessibility.AddToSelection,
		accessibility.RemoveFromSelection, accessibility.ScrollIntoView, accessibility.Expand,
		accessibility.Collapse)
	return newTestTree(1, 6,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{2},
		},
		&accessibility.Node{
			ID: 2, Role: role.ScrollArea, Bounds: geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{3, 6},
		},
		&accessibility.Node{
			ID: 3, Role: role.TableHeader, Bounds: geom.NewRect(0, 0, 200, 20),
			Children: []accessibility.NodeID{4, 5},
		},
		&accessibility.Node{
			ID: 4, Role: role.ColumnHeader, Name: "Name", ColumnIndex: 0, Sort: accessibility.SortAscending,
			Bounds:  geom.NewRect(0, 0, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{
			ID: 5, Role: role.ColumnHeader, Name: "Size", ColumnIndex: 1, Bounds: geom.NewRect(100, 0, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		},
		&accessibility.Node{
			ID: 6, Role: role.Table, Name: "Files", Multiselectable: true, RowCount: 5, ColumnCount: 2,
			Bounds: geom.NewRect(0, 20, 200, 180), Children: []accessibility.NodeID{7, 10},
		},
		&accessibility.Node{
			ID: 7, Role: role.Row, Name: "unison", RowIndex: 1, Level: 1, Selectable: true, Selected: true,
			Expandable: true, Expanded: true, Bounds: geom.NewRect(0, 20, 200, 20), Actions: rowActions,
			Children: []accessibility.NodeID{8, 9},
		},
		&accessibility.Node{
			ID: 8, Role: role.Cell, Name: "unison", RowIndex: 1, ColumnIndex: 0,
			Bounds: geom.NewRect(0, 20, 100, 20),
		},
		&accessibility.Node{
			ID: 9, Role: role.Cell, Name: "4 KB", RowIndex: 1, ColumnIndex: 1,
			Bounds: geom.NewRect(100, 20, 100, 20),
		},
		&accessibility.Node{
			ID: 10, Role: role.Row, Name: "doc.go", RowIndex: 2, Level: 2, Selectable: true,
			Bounds: geom.NewRect(0, 40, 200, 20), Actions: rowActions,
			Children: []accessibility.NodeID{11, 12},
		},
		&accessibility.Node{
			ID: 11, Role: role.Cell, Name: "doc.go", RowIndex: 2, ColumnIndex: 0,
			Bounds: geom.NewRect(0, 40, 100, 20),
		},
		&accessibility.Node{
			ID: 12, Role: role.Cell, Name: "1 KB", RowIndex: 2, ColumnIndex: 1,
			Bounds: geom.NewRect(100, 40, 100, 20),
		},
	)
}

// buriedHeaderTree builds tableTree with its header moved inside an ignored layout panel, which is where a header can
// genuinely end up, and optionally marks the header itself ignored so that it has no provider to hand a client.
func buriedHeaderTree(ignoreHeader bool) *accessibility.Tree {
	tree := tableTree()
	tree.Nodes[2].Children = []accessibility.NodeID{13, 6}
	tree.Nodes[13] = &accessibility.Node{
		ID: 13, Parent: 2, Role: role.Group, Ignored: true, Children: []accessibility.NodeID{3},
	}
	tree.Nodes[3].Parent = 13
	tree.Nodes[3].Ignored = ignoreHeader
	return tree
}

// twoTableTree builds tableTree with a second table alongside the first under the same scroll area, which is the
// layout proximity cannot resolve: either table could be the one the single header describes.
func twoTableTree() *accessibility.Tree {
	tree := tableTree()
	tree.Nodes[2].Children = []accessibility.NodeID{3, 6, 13}
	tree.Nodes[13] = &accessibility.Node{ID: 13, Parent: 2, Role: role.Table, Name: "Other", RowCount: 1}
	return tree
}

// nestedTableTree builds tableTree with a second table inside cell 12 of the first, holding one row of one cell:
//
//	…
//	└─ 10 row    row 2
//	   ├─ 11 cell  row 2 column 0
//	   └─ 12 cell  row 2 column 1
//	      └─ 13 table                  1 row, 1 column
//	         └─ 14 row   row 0
//	            └─ 15 cell  row 0 column 0
//
// The inner table has no header of its own, which is what the outer table's header must not be mistaken for.
func nestedTableTree() *accessibility.Tree {
	tree := tableTree()
	tree.Nodes[12].Children = []accessibility.NodeID{13}
	tree.Nodes[13] = &accessibility.Node{
		ID: 13, Parent: 12, Role: role.Table, Name: "Inner", RowCount: 1, ColumnCount: 1,
		Bounds: geom.NewRect(100, 40, 100, 20), Children: []accessibility.NodeID{14},
	}
	tree.Nodes[14] = &accessibility.Node{
		ID: 14, Parent: 13, Role: role.Row, Name: "Only", RowIndex: 0,
		Bounds: geom.NewRect(100, 40, 100, 20), Children: []accessibility.NodeID{15},
	}
	tree.Nodes[15] = &accessibility.Node{
		ID: 15, Parent: 14, Role: role.Cell, Name: "Deep", RowIndex: 0, ColumnIndex: 0,
		Bounds: geom.NewRect(100, 40, 100, 20),
	}
	return tree
}

// TestUIASelectionItemRole verifies which role a selection container selects among, which is what decides where
// GetSelection looks.
func TestUIASelectionItemRole(t *testing.T) {
	c := check.New(t)
	c.Equal(role.ListItem, UIASelectionItemRole(role.List))
	c.Equal(role.Tab, UIASelectionItemRole(role.TabList))
	c.Equal(role.Row, UIASelectionItemRole(role.Table))
	c.Equal(role.Row, UIASelectionItemRole(role.Tree))
	for _, r := range []role.Enum{role.Window, role.Group, role.Row, role.ListItem, role.Cell, role.RadioButton} {
		c.Equal(role.None, UIASelectionItemRole(r), "role %s selects nothing", r.Key())
	}
}

// TestUIAIsSelected verifies the state a selection item reports. A radio button has no selected state of its own:
// being the chosen one of its group is being checked.
func TestUIAIsSelected(t *testing.T) {
	c := check.New(t)
	tree := patternTree()
	c.True(UIAIsSelected(tree.Node(6)))
	c.False(UIAIsSelected(tree.Node(7)))
	list := listTree()
	c.True(UIAIsSelected(list.Node(3)))
	c.False(UIAIsSelected(list.Node(6)))
	c.False(UIAIsSelected(nil))

	// A radio button that is selected without being checked, which the builder never produces, still reports what the
	// check state says.
	tree.Nodes[6].Checked = checkenum.Off
	tree.Nodes[6].Selected = true
	c.False(UIAIsSelected(tree.Node(6)))
}

// TestUIASelection verifies the set of elements a container reports as selected, including that an ignored node between
// the container and an item is spliced away rather than hiding it.
func TestUIASelection(t *testing.T) {
	c := check.New(t)
	list := listTree()
	c.Equal([]accessibility.NodeID{3, 5}, UIASelection(list, 2))
	c.Equal([]accessibility.NodeID{8}, UIASelection(list, 7))
	c.Nil(UIASelection(list, 3), "a list item is not a selection container")
	c.Nil(UIASelection(list, 1), "a window is not a selection container")
	c.Nil(UIASelection(list, 999))
	c.Nil(UIASelection(nil, 2))

	// Nothing selected is an empty answer rather than a missing one, which the provider reports as an empty array.
	list.Nodes[3].Selected = false
	list.Nodes[5].Selected = false
	c.Nil(UIASelection(list, 2))

	table := tableTree()
	c.Equal([]accessibility.NodeID{7}, UIASelection(table, 6))
	table.Nodes[10].Selected = true
	c.Equal([]accessibility.NodeID{7, 10}, UIASelection(table, 6))
}

// TestUIASelectionNested verifies that a container reports its own selection and not a nested container's: a table
// inside a cell has rows of its own, and each table has to answer for the rows it holds.
func TestUIASelectionNested(t *testing.T) {
	c := check.New(t)
	tree := tableTree()
	tree.Nodes[12].Children = []accessibility.NodeID{13}
	tree.Nodes[13] = &accessibility.Node{
		ID: 13, Parent: 12, Role: role.Table, Name: "Inner", RowCount: 1, ColumnCount: 1,
		Children: []accessibility.NodeID{14},
	}
	tree.Nodes[14] = &accessibility.Node{
		ID: 14, Parent: 13, Role: role.Row, Name: "Deep", RowIndex: 0, Selectable: true, Selected: true,
	}
	c.Equal([]accessibility.NodeID{7}, UIASelection(tree, 6))
	c.Equal([]accessibility.NodeID{14}, UIASelection(tree, 13))

	// A hierarchical table nests its rows under one another, and every one of them belongs to the same table.
	nested := tableTree()
	nested.Nodes[10].Children = append(nested.Nodes[10].Children, 13)
	nested.Nodes[13] = &accessibility.Node{
		ID: 13, Parent: 10, Role: role.Row, Name: "Child", RowIndex: 3, Level: 3, Selectable: true, Selected: true,
	}
	c.Equal([]accessibility.NodeID{7, 13}, UIASelection(nested, 6))
}

// uiaMalformedTreeDeadline is how long a question asked of a malformed snapshot is given before the test calls it hung.
// The answer takes microseconds, and the failure this guards against is an endless walk rather than a slow one, so the
// deadline is far longer than it needs to be and nothing here is timing-sensitive.
const uiaMalformedTreeDeadline = 10 * time.Second

// TestUIASelectionMalformedTree verifies that the walk down a subtree both terminates and answers on a snapshot whose
// Children links are malformed. A depth bound alone does not bound a downward walk: links that branch and revisit
// double the work at every level, and a link that points back at an ancestor closes a loop that never ends. UI
// Automation asks for a container's selection from whichever thread it likes, so either shape would hang a client
// rather than merely answer it wrongly.
func TestUIASelectionMalformedTree(t *testing.T) {
	c := check.New(t)

	// A chain of groups 22 deep, each listing its one child twice, with the only list item at the bottom. Reaching the
	// item once per path through the chain reports it 2^22 times.
	const bottom = accessibility.NodeID(24)
	nodes := []*accessibility.Node{
		{ID: 1, Role: role.List, Name: "Choices", Children: []accessibility.NodeID{2}},
	}
	for id := accessibility.NodeID(2); id < bottom; id++ {
		nodes = append(nodes, &accessibility.Node{
			ID: id, Role: role.Group, Children: []accessibility.NodeID{id + 1, id + 1},
		})
	}
	nodes = append(nodes, &accessibility.Node{
		ID: bottom, Role: role.ListItem, Name: "Only", Selectable: true, Selected: true,
	})
	c.Equal([]accessibility.NodeID{bottom}, UIASelection(newTestTree(1, 0, nodes...), 1))

	// A four-node cycle: the list holds one group, that group holds two more nodes, and both of them name the group as
	// a child of their own. There is no bottom to reach, so the answer is checked on a goroutine of its own rather than
	// leaving the test to be killed by the package's own timeout.
	cycle := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.List, Name: "Choices", Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Group, Children: []accessibility.NodeID{3, 4}},
		&accessibility.Node{ID: 3, Role: role.Group, Children: []accessibility.NodeID{2}},
		&accessibility.Node{
			ID: 4, Role: role.ListItem, Name: "Only", Selectable: true, Selected: true,
			Children: []accessibility.NodeID{2},
		},
	)
	done := make(chan []accessibility.NodeID, 1)
	go func() { done <- UIASelection(cycle, 1) }()
	select {
	case ids := <-done:
		c.Equal([]accessibility.NodeID{4}, ids)
	case <-time.After(uiaMalformedTreeDeadline):
		t.Fatal("UIASelection followed the cyclic child links rather than stopping at the nodes it had seen")
	}
}

// TestUIATableColumnHeadersMalformedTree verifies that the search for a table's header survives a snapshot that lists
// the same child twice. The search gives up on an ancestor holding more than one table, since widening it could only
// make the ambiguity worse, so counting the one table twice would end the search and leave the header that is really
// there unfound.
func TestUIATableColumnHeadersMalformedTree(t *testing.T) {
	c := check.New(t)
	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.ScrollArea, Children: []accessibility.NodeID{3, 4, 4}},
		&accessibility.Node{ID: 3, Role: role.TableHeader, Children: []accessibility.NodeID{5, 6}},
		&accessibility.Node{ID: 4, Role: role.Table, ColumnCount: 2},
		&accessibility.Node{ID: 5, Role: role.ColumnHeader, Name: "Name", ColumnIndex: 0},
		&accessibility.Node{ID: 6, Role: role.ColumnHeader, Name: "Size", ColumnIndex: 1},
	)
	c.Equal([]accessibility.NodeID{5, 6}, UIATableColumnHeaders(tree, 4))
}

// TestUIASelectionContainer verifies which element a selection item says it belongs to, including that a radio button
// says none: its group is a layout panel with no pattern of its own.
func TestUIASelectionContainer(t *testing.T) {
	c := check.New(t)
	list := listTree()
	c.Equal(accessibility.NodeID(2), UIASelectionContainer(list, 3))
	c.Equal(accessibility.NodeID(2), UIASelectionContainer(list, 5), "through an ignored group")
	c.Equal(accessibility.NodeID(7), UIASelectionContainer(list, 8))
	c.Equal(accessibility.NodeID(0), UIASelectionContainer(list, 1))
	c.Equal(accessibility.NodeID(0), UIASelectionContainer(list, 999))

	table := tableTree()
	c.Equal(accessibility.NodeID(6), UIASelectionContainer(table, 7))

	tree := patternTree()
	c.Equal(accessibility.NodeID(0), UIASelectionContainer(tree, 6), "a radio button reports no container")
	c.Equal(accessibility.NodeID(0), UIASelectionContainer(tree, 2))
}

// TestUIAContainingGrid verifies which element a cell says it belongs to.
func TestUIAContainingGrid(t *testing.T) {
	c := check.New(t)
	table := tableTree()
	c.Equal(accessibility.NodeID(6), UIAContainingGrid(table, 8))
	c.Equal(accessibility.NodeID(6), UIAContainingGrid(table, 12))
	c.Equal(accessibility.NodeID(6), UIAContainingGrid(table, 7), "a row is inside the grid too")
	c.Equal(accessibility.NodeID(0), UIAContainingGrid(table, 6), "a table is not inside itself")
	c.Equal(accessibility.NodeID(0), UIAContainingGrid(table, 4))
	c.Equal(accessibility.NodeID(0), UIAContainingGrid(listTree(), 3))
}

// TestUIAGridItem verifies the cell lookup a client walking a table by row and column goes through. A cell the snapshot
// does not hold is reported as nothing, which is the normal answer for a table that publishes only its viewport.
func TestUIAGridItem(t *testing.T) {
	c := check.New(t)
	table := tableTree()
	c.Equal(accessibility.NodeID(8), UIAGridItem(table, 6, 1, 0))
	c.Equal(accessibility.NodeID(9), UIAGridItem(table, 6, 1, 1))
	c.Equal(accessibility.NodeID(11), UIAGridItem(table, 6, 2, 0))
	c.Equal(accessibility.NodeID(12), UIAGridItem(table, 6, 2, 1))
	c.Equal(accessibility.NodeID(0), UIAGridItem(table, 6, 0, 0), "a row outside the viewport is not published")
	c.Equal(accessibility.NodeID(0), UIAGridItem(table, 6, 1, 5))
	c.Equal(accessibility.NodeID(0), UIAGridItem(table, 6, -1, 0))
	c.Equal(accessibility.NodeID(0), UIAGridItem(table, 6, 1, -1))
	c.Equal(accessibility.NodeID(0), UIAGridItem(table, 999, 1, 0))
	c.Equal(accessibility.NodeID(0), UIAGridItem(nil, 6, 1, 0))

	// The row is found by the index it recorded rather than by its position among the rows published, which is what
	// keeps the answer right while scrolled.
	table.Nodes[7].RowIndex = 30
	table.Nodes[8].RowIndex = 30
	c.Equal(accessibility.NodeID(0), UIAGridItem(table, 6, 1, 0))
	c.Equal(accessibility.NodeID(8), UIAGridItem(table, 6, 30, 0))

	// A hierarchical table nests its rows, and a nested row's cells are reachable the same way.
	nested := tableTree()
	nested.Nodes[10].Children = append(nested.Nodes[10].Children, 13)
	nested.Nodes[13] = &accessibility.Node{
		ID: 13, Parent: 10, Role: role.Row, Name: "Child", RowIndex: 3, Children: []accessibility.NodeID{14},
	}
	nested.Nodes[14] = &accessibility.Node{ID: 14, Parent: 13, Role: role.Cell, Name: "Deep", ColumnIndex: 1}
	c.Equal(accessibility.NodeID(14), UIAGridItem(nested, 6, 3, 1))
}

// TestUIATableColumnHeaders verifies how the header that describes a table's columns is found, which takes a search
// because the two are separate panels with nothing in the snapshot linking them.
func TestUIATableColumnHeaders(t *testing.T) {
	c := check.New(t)
	table := tableTree()
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(table, 6), "found by proximity")
	c.Nil(UIATableColumnHeaders(table, 999))
	c.Nil(UIATableColumnHeaders(nil, 6))
	c.Nil(UIATableColumnHeaders(table, 2), "a scroll area has no columns")

	// An explicit link, in either direction, is used before proximity is considered.
	controlled := tableTree()
	controlled.Nodes[3].Controls = []accessibility.NodeID{6}
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(controlled, 6))
	controlling := tableTree()
	controlling.Nodes[6].Controls = []accessibility.NodeID{3}
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(controlling, 6))

	// A header buried in a layout panel is still the header; an ignored header is not, since it has no provider. Each
	// case gets a tree of its own rather than being made by editing the previous one: a published snapshot is never
	// modified, and UIATableColumnHeaders remembers its answer for the tree it was asked about.
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(buriedHeaderTree(false), 6))
	c.Nil(UIATableColumnHeaders(buriedHeaderTree(true), 6))

	// Two tables sharing an ancestor cannot be told apart by proximity, so neither claims a header. Naming one
	// explicitly settles it.
	two := twoTableTree()
	c.Nil(UIATableColumnHeaders(two, 6))
	c.Nil(UIATableColumnHeaders(two, 13))
	named := twoTableTree()
	named.Nodes[6].Controls = []accessibility.NodeID{3}
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(named, 6))
	c.Nil(UIATableColumnHeaders(named, 13))

	// A table with no header anywhere in the window reports none rather than reaching for something unrelated.
	alone := tableTree()
	delete(alone.Nodes, 3)
	delete(alone.Nodes, 4)
	delete(alone.Nodes, 5)
	alone.Nodes[2].Children = []accessibility.NodeID{6}
	c.Nil(UIATableColumnHeaders(alone, 6))

	// The search widens to the next ancestor when the nearest one holds no header at all.
	sibling := tableTree()
	sibling.Nodes[1].Children = []accessibility.NodeID{3, 2}
	sibling.Nodes[3].Parent = 1
	sibling.Nodes[2].Children = []accessibility.NodeID{6}
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(sibling, 6))
}

// TestUIATableColumnHeadersNested verifies that a table nested in a cell of another table claims no header at all
// rather than the outer table's. Widening the proximity search past the table that contains it would have a screen
// reader announce the outer table's column names over every cell of the inner one.
func TestUIATableColumnHeadersNested(t *testing.T) {
	c := check.New(t)
	nested := nestedTableTree()
	c.Nil(UIATableColumnHeaders(nested, 13), "the inner table's search stops at the table that contains it")
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(nested, 6), "the outer table is unaffected")
	c.Equal(accessibility.NodeID(0), UIAColumnHeaderItem(nested, 15), "so the inner table's cell has no header")
	c.Equal(accessibility.NodeID(5), UIAColumnHeaderItem(nested, 12), "while the cell holding it still has one")
}

// TestUIATableColumnHeadersMemo verifies that the answer remembered for one snapshot is never handed to another. The
// cost of finding a header is what makes remembering it worth doing — a client asks once per cell — but a window that
// publishes a new snapshot must be described by the new one from that moment on. See uiaSnapshotMemo.
func TestUIATableColumnHeadersMemo(t *testing.T) {
	c := check.New(t)
	first := tableTree()
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(first, 6))
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(first, 6), "asking twice answers the same")

	headerless := tableTree()
	delete(headerless.Nodes, 3)
	delete(headerless.Nodes, 4)
	delete(headerless.Nodes, 5)
	headerless.Nodes[2].Children = []accessibility.NodeID{6}
	c.Nil(UIATableColumnHeaders(headerless, 6), "the next snapshot has no header, and is not answered from the last")
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(first, 6), "and going back answers the first again")
}

// TestUIAForgetSnapshotMemo verifies that the memo can be emptied, which is what keeps the last snapshot of a destroyed
// window — and every node in it — from staying reachable for the rest of the process. UIAWindow.Destroy calls it; the
// only thing a client can notice is that the next question is worked out from scratch.
func TestUIAForgetSnapshotMemo(t *testing.T) {
	c := check.New(t)

	// A table whose name comes from a label outside it, so that all three of the answers the memo holds are worked out
	// from the one snapshot it can hold at a time.
	tree := tableTree()
	tree.Nodes[1].Children = []accessibility.NodeID{2, 13}
	tree.Nodes[13] = &accessibility.Node{ID: 13, Parent: 1, Role: role.Label, Name: "Files"}
	tree.Nodes[6].LabeledBy = []accessibility.NodeID{13}
	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(tree, 6))
	c.False(UIAIsContentElement(tree, tree.Node(13)), "the label names the table, so it is not content of its own")
	position, size := UIAPositionInSet(tree, tree.Node(7))
	c.Equal(2, position)
	c.Equal(5, size)

	uiaForgetSnapshotMemo()
	c.Nil(uiaSnapshotMemo.tree, "the snapshot is no longer held")
	c.Nil(uiaSnapshotMemo.headers, "and neither is anything worked out from it")
	c.Nil(uiaSnapshotMemo.named)
	c.False(uiaSnapshotMemo.namedDone)
	c.Nil(uiaSnapshotMemo.positions)

	c.Equal([]accessibility.NodeID{4, 5}, UIATableColumnHeaders(tree, 6), "and the answer is worked out again")
	c.Equal(tree, uiaSnapshotMemo.tree)
	uiaForgetSnapshotMemo()
}

// TestUIASnapshotMemoHoldsOneSnapshot verifies that every answer remembered is dropped the moment a different snapshot
// is asked about, which is what keeps one window's answers from ever being given for another's.
func TestUIASnapshotMemoHoldsOneSnapshot(t *testing.T) {
	c := check.New(t)
	uiaForgetSnapshotMemo()
	t.Cleanup(uiaForgetSnapshotMemo)

	labels := func(labeledBy []accessibility.NodeID) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 3}},
			&accessibility.Node{ID: 2, Role: role.Label, Name: "Name"},
			&accessibility.Node{ID: 3, Role: role.TextField, LabeledBy: labeledBy},
		)
	}
	named := labels([]accessibility.NodeID{2})
	lone := labels(nil)

	c.False(UIAIsContentElement(named, named.Node(2)), "a label another node names is not in the content view")
	c.Equal(named, uiaSnapshotMemo.tree)
	c.True(uiaSnapshotMemo.namedDone, "and the whole snapshot's answer was worked out at once")

	c.True(UIAIsContentElement(lone, lone.Node(2)), "while the next snapshot names nothing, and is not answered from"+
		" the last")
	c.Equal(lone, uiaSnapshotMemo.tree)
	c.Nil(uiaSnapshotMemo.named, "a snapshot in which nothing names anything allocates no set at all")
	c.False(UIAIsContentElement(named, named.Node(2)), "and going back answers the first again")

	// A snapshot the memo has already moved past is the one thing that does not take it over. raisePropertyChanged asks
	// about the previous tree and then about the current one for every property it raises, so an older tree let in here
	// would throw away what a client's own thread had built for the snapshot it is walking — once per property. Two
	// snapshots are of the same window when they name the same root, and Generation orders them.
	previous := labels(nil)
	previous.Generation = 6
	current := labels([]accessibility.NodeID{2})
	current.Generation = 7
	c.False(UIAIsContentElement(current, current.Node(2)), "the label the current snapshot names is not content")
	c.Equal(current, uiaSnapshotMemo.tree)
	c.True(UIAIsContentElement(previous, previous.Node(2)), "the previous snapshot is answered from itself")
	c.Equal(current, uiaSnapshotMemo.tree, "without evicting the snapshot the memo is serving")
	c.True(uiaSnapshotMemo.named[2], "or dropping what had already been worked out from it")

	// A snapshot of another window is not compared by generation at all, however few times that window has published:
	// generations are counted per window, so a newly opened dialog would otherwise be starved of the memo for as long
	// as a window that has been publishing for hours held it.
	other := newTestTree(11, 0,
		&accessibility.Node{ID: 11, Role: role.Window, Children: []accessibility.NodeID{12, 13}},
		&accessibility.Node{ID: 12, Role: role.Label, Name: "Name"},
		&accessibility.Node{ID: 13, Role: role.TextField, LabeledBy: []accessibility.NodeID{12}},
	)
	other.Generation = 1
	c.False(UIAIsContentElement(other, other.Node(12)), "another window's label names something of its own")
	c.Equal(other, uiaSnapshotMemo.tree, "and its snapshot takes the memo over")
}

// TestUIAPositionInSetMemo verifies that the two halves of one node's position are worked out once and answered twice,
// which is what a client reading PositionInSet and SizeOfSet of the same element asks for.
func TestUIAPositionInSetMemo(t *testing.T) {
	c := check.New(t)
	uiaForgetSnapshotMemo()
	t.Cleanup(uiaForgetSnapshotMemo)

	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{3, 4}},
		&accessibility.Node{ID: 3, Role: role.ListItem},
		&accessibility.Node{ID: 4, Role: role.ListItem},
	)
	position, size := UIAPositionInSet(tree, tree.Node(4))
	c.Equal(2, position)
	c.Equal(2, size)
	c.Equal(uiaSetPosition{position: 2, size: 2}, uiaSnapshotMemo.positions[4])
	position, size = UIAPositionInSet(tree, tree.Node(4))
	c.Equal(2, position, "asking again answers the same")
	c.Equal(2, size)

	// An ignored node never reaches the memo: it has no provider to answer a property for, and remembering an answer
	// for it would put an entry in the map for every layout panel a client ever hit tested.
	ignored := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.ListItem, Ignored: true},
	)
	position, size = UIAPositionInSet(ignored, ignored.Node(2))
	c.Equal(0, position)
	c.Equal(0, size)
	c.Equal(tree, uiaSnapshotMemo.tree, "so the snapshot being answered from is the one that was asked about")

	// A different snapshot is never answered from the last, however alike the two are.
	other := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{4}},
		&accessibility.Node{ID: 4, Role: role.ListItem},
	)
	position, size = UIAPositionInSet(other, other.Node(4))
	c.Equal(1, position)
	c.Equal(1, size)

	// A snapshot the memo has already moved past is answered from itself and remembered nowhere, so that the publish
	// path — which asks for both halves of this of the previous snapshot as well as of the current one, for every node
	// whose attributes changed — cannot evict the snapshot a client is walking. See uiaMemoSwitchTo.
	current := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{3, 4}},
		&accessibility.Node{ID: 3, Role: role.ListItem},
		&accessibility.Node{ID: 4, Role: role.ListItem},
	)
	current.Generation = 9
	previous := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{4}},
		&accessibility.Node{ID: 4, Role: role.ListItem},
	)
	previous.Generation = 8
	position, size = UIAPositionInSet(current, current.Node(4))
	c.Equal(2, position)
	c.Equal(2, size)
	c.Equal(current, uiaSnapshotMemo.tree)
	position, size = UIAPositionInSet(previous, previous.Node(4))
	c.Equal(1, position, "the previous snapshot is answered from itself")
	c.Equal(1, size)
	c.Equal(current, uiaSnapshotMemo.tree, "without evicting the snapshot the memo is serving")
	c.Equal(uiaSetPosition{position: 2, size: 2}, uiaSnapshotMemo.positions[4], "or what it had remembered")
	c.Equal(1, len(uiaSnapshotMemo.positions), "and nothing of the older snapshot's is remembered")
}

// TestUIAColumnHeaderItem verifies which header a cell says describes its column, including the fallback for a snapshot
// that never filled the headers' column indexes in.
func TestUIAColumnHeaderItem(t *testing.T) {
	c := check.New(t)
	table := tableTree()
	c.Equal(accessibility.NodeID(4), UIAColumnHeaderItem(table, 8))
	c.Equal(accessibility.NodeID(5), UIAColumnHeaderItem(table, 9))
	c.Equal(accessibility.NodeID(4), UIAColumnHeaderItem(table, 11))
	c.Equal(accessibility.NodeID(5), UIAColumnHeaderItem(table, 12))
	c.Equal(accessibility.NodeID(0), UIAColumnHeaderItem(table, 999))
	c.Equal(accessibility.NodeID(0), UIAColumnHeaderItem(table, 4), "a header has no header")

	// Headers whose column indexes were never filled in are taken by position instead.
	unindexed := tableTree()
	unindexed.Nodes[5].ColumnIndex = 0
	c.Equal(accessibility.NodeID(4), UIAColumnHeaderItem(unindexed, 8))
	c.Equal(accessibility.NodeID(5), UIAColumnHeaderItem(unindexed, 9))

	// A column with no header at all, and a cell whose column is beyond every header.
	short := tableTree()
	delete(short.Nodes, 5)
	short.Nodes[3].Children = []accessibility.NodeID{4}
	c.Equal(accessibility.NodeID(0), UIAColumnHeaderItem(short, 9))
}

// TestUIAValueString verifies the text the Value pattern reports, and that a password reports nothing.
func TestUIAValueString(t *testing.T) {
	c := check.New(t)
	tree := patternTree()
	c.Equal("Gandalf", UIAValueString(tree.Node(8)), "a text control's content is its value")
	c.Equal("fixed", UIAValueString(tree.Node(10)))
	c.Equal("Red", UIAValueString(tree.Node(14)))
	c.Equal("#ff0000", UIAValueString(tree.Node(16)))
	c.Equal("", UIAValueString(tree.Node(9)), "a password reports nothing")
	c.Equal("", UIAValueString(tree.Node(17)))
	c.Equal("", UIAValueString(nil))

	// An explicit value wins over the text, since a widget that fills in both means the value.
	tree.Nodes[8].Value = "Mithrandir"
	c.Equal("Mithrandir", UIAValueString(tree.Node(8)))
}

// TestUIAValueReadOnly verifies when the Value pattern says the value cannot be changed.
func TestUIAValueReadOnly(t *testing.T) {
	c := check.New(t)
	tree := patternTree()
	c.False(UIAIsValueReadOnly(tree.Node(8)), "a text field takes a new value")
	c.False(UIAIsValueReadOnly(tree.Node(15)), "so does a combo box")
	c.True(UIAIsValueReadOnly(tree.Node(10)), "not one the snapshot marks read-only")
	c.True(UIAIsValueReadOnly(tree.Node(14)), "a popup button reports its choice but does not take one")
	c.True(UIAIsValueReadOnly(tree.Node(16)), "nor does a color well")
	c.True(UIAIsValueReadOnly(nil))

	tree.Nodes[8].Disabled = true
	c.False(UIAIsValueReadOnly(tree.Node(8)),
		"a disabled field is not read-only: being unusable now says nothing about whether the value could be set")
	tree.Nodes[8].Disabled = false
	tree.Nodes[8].Actions = tree.Nodes[8].Actions.Without(accessibility.SetValue)
	c.True(UIAIsValueReadOnly(tree.Node(8)), "a node that offers no SetValue action cannot take one")

	document := &accessibility.Node{
		ID: 1, Role: role.Document, Actions: accessibility.ActionSet(0).With(accessibility.SetValue),
	}
	c.True(UIAIsValueReadOnly(document), "a document is there to be read")
}

// TestUIARangeValueReadOnly verifies when the RangeValue pattern says the value cannot be changed.
func TestUIARangeValueReadOnly(t *testing.T) {
	c := check.New(t)
	tree := patternTree()
	c.False(UIAIsRangeValueReadOnly(tree.Node(11)), "a slider takes a new value")
	c.False(UIAIsRangeValueReadOnly(tree.Node(12)),
		"so does a disabled one, in principle: SetValue refuses it as not enabled rather than as read-only")
	c.True(UIAIsRangeValueReadOnly(tree.Node(13)), "nor does a progress bar, ever")
	c.True(UIAIsRangeValueReadOnly(nil))

	// The rule is the action set's rather than the role's: a node that offers only the two steps says, by leaving
	// SetValue out, that nothing will happen if a client sets the value outright. The scroll bar unison actually ships
	// is not such a node — ScrollBar.ProvideAccessibility offers SetValue alongside Increment and Decrement, which is
	// how VoiceOver scrolls something into view — so both sides are checked here.
	scrollBar := &accessibility.Node{
		ID: 1, Role: role.ScrollBar, HasNumber: true, Number: 5, Max: 10,
		Actions: accessibility.ActionSet(0).With(accessibility.Increment, accessibility.Decrement),
	}
	c.True(UIAIsRangeValueReadOnly(scrollBar), "offering no SetValue action is what makes a value read-only")
	scrollBar.Actions = scrollBar.Actions.With(accessibility.SetValue)
	c.False(UIAIsRangeValueReadOnly(scrollBar), "a real scroll bar offers SetValue, so it is not read-only")
}
