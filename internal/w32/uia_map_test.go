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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// newTestTree assembles a tree from nodes given in any order, filling in each node's Parent from the Children lists so
// that a test only has to state the hierarchy once.
func newTestTree(root, focus accessibility.NodeID, nodes ...*accessibility.Node) *accessibility.Tree {
	tree := &accessibility.Tree{
		Nodes:      make(map[accessibility.NodeID]*accessibility.Node, len(nodes)),
		Root:       root,
		Focus:      focus,
		Generation: 1,
	}
	for _, n := range nodes {
		tree.Nodes[n.ID] = n
	}
	for _, n := range nodes {
		for _, childID := range n.Children {
			if child := tree.Nodes[childID]; child != nil {
				child.Parent = n.ID
			}
		}
	}
	return tree
}

// sampleTree builds the hierarchy the navigation, hit-testing and content-view tests work over:
//
//	1 window                          (0,0 200x200)  focused
//	├─ 2 group   [ignored]            (0,0 200x100)
//	│  ├─ 3 group   [ignored]         (0,0 200x50)
//	│  │  ├─ 4 button                 (0,0 50x20)
//	│  │  └─ 5 button                 (50,0 50x20)
//	│  └─ 6 label                     (0,50 100x20)   labels 4
//	└─ 7 group                        (0,100 200x100)
//	   ├─ 8 button                     (0,100 60x30)
//	   └─ 9 button                     (0,100 60x30)
//
// The two layers of ignored grouping mean the window's unignored children are 4, 5, 6 and 7, which is what makes this
// tree worth navigating. Nodes 8 and 9 deliberately occupy the same area, with 8 first so that it is the topmost.
func sampleTree() *accessibility.Tree {
	return newTestTree(1, 4,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true, Bounds: geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{2, 7},
		},
		&accessibility.Node{
			ID: 2, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{3, 6},
		},
		&accessibility.Node{
			ID: 3, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 0, 200, 50),
			Children: []accessibility.NodeID{4, 5},
		},
		&accessibility.Node{
			ID: 4, Role: role.Button, Name: "One", Focusable: true, Focused: true,
			Bounds: geom.NewRect(0, 0, 50, 20), LabeledBy: []accessibility.NodeID{6},
		},
		&accessibility.Node{ID: 5, Role: role.Button, Name: "Two", Bounds: geom.NewRect(50, 0, 50, 20)},
		&accessibility.Node{ID: 6, Role: role.Label, Name: "One", Bounds: geom.NewRect(0, 50, 100, 20)},
		&accessibility.Node{
			ID: 7, Role: role.Group, Bounds: geom.NewRect(0, 100, 200, 100),
			Children: []accessibility.NodeID{8, 9},
		},
		&accessibility.Node{ID: 8, Role: role.Button, Name: "Three", Bounds: geom.NewRect(0, 100, 60, 30)},
		&accessibility.Node{ID: 9, Role: role.Button, Name: "Four", Bounds: geom.NewRect(0, 100, 60, 30)},
	)
}

// TestUIAControlType verifies the role-to-control-type table, and that it covers every role: a role added to the enum
// without a decision here would silently become Custom, which is the wrong answer for anything UI Automation has a
// control type for.
func TestUIAControlType(t *testing.T) {
	c := check.New(t)
	expected := map[role.Enum]ControlTypeID{
		role.Auto:               UIA_CustomControlTypeId,
		role.None:               UIA_CustomControlTypeId,
		role.Window:             UIA_WindowControlTypeId,
		role.Dialog:             UIA_WindowControlTypeId,
		role.Group:              UIA_GroupControlTypeId,
		role.Button:             UIA_ButtonControlTypeId,
		role.ToggleButton:       UIA_ButtonControlTypeId,
		role.DisclosureTriangle: UIA_ButtonControlTypeId,
		role.CheckBox:           UIA_CheckBoxControlTypeId,
		role.RadioButton:        UIA_RadioButtonControlTypeId,
		role.Link:               UIA_HyperlinkControlTypeId,
		role.Label:              UIA_TextControlTypeId,
		role.Heading:            UIA_TextControlTypeId,
		role.TextField:          UIA_EditControlTypeId,
		role.TextArea:           UIA_EditControlTypeId,
		role.SpinButton:         UIA_SpinnerControlTypeId,
		role.ComboBox:           UIA_ComboBoxControlTypeId,
		role.PopupButton:        UIA_ComboBoxControlTypeId,
		role.Slider:             UIA_SliderControlTypeId,
		role.ProgressBar:        UIA_ProgressBarControlTypeId,
		role.ScrollBar:          UIA_ScrollBarControlTypeId,
		role.ScrollArea:         UIA_PaneControlTypeId,
		role.Separator:          UIA_SeparatorControlTypeId,
		role.List:               UIA_ListControlTypeId,
		role.ListItem:           UIA_ListItemControlTypeId,
		role.Table:              UIA_TableControlTypeId,
		role.Tree:               UIA_DataGridControlTypeId,
		role.Row:                UIA_DataItemControlTypeId,
		role.Cell:               UIA_DataItemControlTypeId,
		role.ColumnHeader:       UIA_HeaderItemControlTypeId,
		role.TableHeader:        UIA_HeaderControlTypeId,
		role.TabList:            UIA_TabControlTypeId,
		role.Tab:                UIA_TabItemControlTypeId,
		role.TabPanel:           UIA_PaneControlTypeId,
		role.MenuBar:            UIA_MenuBarControlTypeId,
		role.Menu:               UIA_MenuControlTypeId,
		role.MenuItem:           UIA_MenuItemControlTypeId,
		role.Image:              UIA_ImageControlTypeId,
		role.ColorWell:          UIA_ButtonControlTypeId,
		role.Tooltip:            UIA_ToolTipControlTypeId,
		role.Document:           UIA_DocumentControlTypeId,
		role.Toolbar:            UIA_ToolBarControlTypeId,
		role.Unknown:            UIA_CustomControlTypeId,
	}
	c.Equal(len(role.All), len(expected))
	for _, r := range role.All {
		want, ok := expected[r]
		c.True(ok, "no control type expected for role %s", r.Key())
		c.Equal(want, UIAControlType(&accessibility.Node{ID: 1, Role: r}), "role %s", r.Key())
	}
	c.Equal(UIA_CustomControlTypeId, UIAControlType(nil))
}

// TestUIAPatterns verifies the role-to-patterns table, including the handful of roles whose patterns depend on state.
func TestUIAPatterns(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		node     *accessibility.Node
		patterns PatternSet
	}{
		{node: &accessibility.Node{Role: role.Window}, patterns: PatternWindow},
		{node: &accessibility.Node{Role: role.Dialog}, patterns: PatternWindow},
		{node: &accessibility.Node{Role: role.Group}},
		{node: &accessibility.Node{Role: role.TabPanel}},
		{node: &accessibility.Node{Role: role.ScrollArea}},
		{node: &accessibility.Node{Role: role.TableHeader}},
		{node: &accessibility.Node{Role: role.Label}},
		{node: &accessibility.Node{Role: role.Heading}},
		{node: &accessibility.Node{Role: role.Image}},
		{node: &accessibility.Node{Role: role.Separator}},
		{node: &accessibility.Node{Role: role.MenuBar}},
		{node: &accessibility.Node{Role: role.Tooltip}},
		{node: &accessibility.Node{Role: role.Toolbar}},
		{node: &accessibility.Node{Role: role.Button}, patterns: PatternInvoke},
		{node: &accessibility.Node{Role: role.Link}, patterns: PatternInvoke},
		{node: &accessibility.Node{Role: role.ColumnHeader}, patterns: PatternInvoke},
		{node: &accessibility.Node{Role: role.ColorWell}, patterns: PatternInvoke | PatternValue},
		{node: &accessibility.Node{Role: role.ToggleButton}, patterns: PatternToggle},
		{node: &accessibility.Node{Role: role.DisclosureTriangle}, patterns: PatternToggle},
		{node: &accessibility.Node{Role: role.CheckBox}, patterns: PatternToggle},
		{node: &accessibility.Node{Role: role.RadioButton}, patterns: PatternSelectionItem},
		{node: &accessibility.Node{Role: role.TextField}, patterns: PatternValue},
		{node: &accessibility.Node{Role: role.TextArea}, patterns: PatternValue},
		{node: &accessibility.Node{Role: role.Document}, patterns: PatternValue},
		{node: &accessibility.Node{Role: role.SpinButton}, patterns: PatternValue | PatternRangeValue},
		{node: &accessibility.Node{Role: role.ComboBox}, patterns: PatternValue | PatternExpandCollapse},
		{node: &accessibility.Node{Role: role.PopupButton}, patterns: PatternValue | PatternExpandCollapse},
		{node: &accessibility.Node{Role: role.Slider}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.ScrollBar}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.ProgressBar}},
		{node: &accessibility.Node{Role: role.ProgressBar, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.List}, patterns: PatternSelection},
		{node: &accessibility.Node{Role: role.TabList}, patterns: PatternSelection},
		{node: &accessibility.Node{Role: role.ListItem}, patterns: PatternSelectionItem | PatternScrollItem},
		{node: &accessibility.Node{Role: role.Tab}, patterns: PatternSelectionItem},
		{node: &accessibility.Node{Role: role.Table}, patterns: PatternGrid | PatternTable | PatternSelection},
		{node: &accessibility.Node{Role: role.Tree}, patterns: PatternGrid | PatternTable | PatternSelection},
		{node: &accessibility.Node{Role: role.Row}, patterns: PatternSelectionItem | PatternScrollItem},
		{
			node:     &accessibility.Node{Role: role.Row, Expandable: true},
			patterns: PatternSelectionItem | PatternScrollItem | PatternExpandCollapse,
		},
		{node: &accessibility.Node{Role: role.Cell}, patterns: PatternGridItem | PatternTableItem},
		{node: &accessibility.Node{Role: role.Menu}, patterns: PatternExpandCollapse},
		{node: &accessibility.Node{Role: role.MenuItem}, patterns: PatternInvoke},
		{node: &accessibility.Node{Role: role.MenuItem, HasCheck: true}, patterns: PatternInvoke | PatternToggle},
		{
			node:     &accessibility.Node{Role: role.MenuItem, Expandable: true},
			patterns: PatternInvoke | PatternExpandCollapse,
		},
	} {
		c.Equal(one.patterns, UIAPatterns(one.node), "case %d (%s)", i, one.node.Role.Key())
	}
	c.Equal(PatternSet(0), UIAPatterns(nil))
}

// TestPatternSet verifies the bookkeeping around the pattern bitset: that every pattern a provider implements can be
// looked up by the identifier a client asks for it by, that a pattern this package does not implement looks up to
// nothing, and that Has is an all-of test rather than an any-of one.
func TestPatternSet(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		id      PatternID
		pattern PatternSet
	}{
		{id: UIA_InvokePatternId, pattern: PatternInvoke},
		{id: UIA_TogglePatternId, pattern: PatternToggle},
		{id: UIA_ValuePatternId, pattern: PatternValue},
		{id: UIA_RangeValuePatternId, pattern: PatternRangeValue},
		{id: UIA_SelectionPatternId, pattern: PatternSelection},
		{id: UIA_SelectionItemPatternId, pattern: PatternSelectionItem},
		{id: UIA_ExpandCollapsePatternId, pattern: PatternExpandCollapse},
		{id: UIA_ScrollItemPatternId, pattern: PatternScrollItem},
		{id: UIA_GridPatternId, pattern: PatternGrid},
		{id: UIA_GridItemPatternId, pattern: PatternGridItem},
		{id: UIA_TablePatternId, pattern: PatternTable},
		{id: UIA_TableItemPatternId, pattern: PatternTableItem},
		{id: UIA_WindowPatternId, pattern: PatternWindow},
	} {
		c.Equal(one.pattern, PatternSetForID(one.id))
	}
	c.Equal(PatternSet(0), PatternSetForID(UIA_TextPatternId))
	c.Equal(PatternSet(0), PatternSetForID(UIA_ScrollPatternId))
	c.Equal(PatternSet(0), PatternSetForID(0))

	both := PatternValue | PatternRangeValue
	c.True(both.Has(PatternValue))
	c.True(both.Has(both))
	c.False(both.Has(PatternValue | PatternToggle))
	c.Equal("value,range-value", both.String())
	c.Equal("", PatternSet(0).String())
}

// TestUIAViews verifies which nodes belong to the control and content views. The interesting case is a label: it stays
// in the content view while it names nothing, and drops out once another node says it is labeled by it, because that
// node already reports the label's text as its own name.
func TestUIAViews(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()

	c.False(UIAIsControlElement(nil))
	c.False(UIAIsControlElement(tree.Node(2)))
	c.True(UIAIsControlElement(tree.Node(4)))
	c.True(UIAIsControlElement(tree.Node(7)))

	c.False(UIAIsContentElement(tree, tree.Node(2)))
	c.True(UIAIsContentElement(tree, tree.Node(4)))
	c.False(UIAIsContentElement(tree, tree.Node(6)), "a label that names node 4 is not content")

	lone := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 3, 4, 5}},
		&accessibility.Node{ID: 2, Role: role.Label, Name: "Standing alone"},
		&accessibility.Node{ID: 3, Role: role.Separator},
		&accessibility.Node{ID: 4, Role: role.ScrollBar},
		&accessibility.Node{ID: 5, Role: role.Tooltip},
	)
	c.True(UIAIsContentElement(lone, lone.Node(2)), "a label that names nothing is content")
	c.False(UIAIsContentElement(lone, lone.Node(3)))
	c.False(UIAIsContentElement(lone, lone.Node(4)))
	c.False(UIAIsContentElement(lone, lone.Node(5)))
}

// TestUIAHeadingLevel verifies that only headings report a level, that the nine UI Automation levels are numbered from
// the right place, and that a deeper heading clamps rather than running off the end of the enumeration.
func TestUIAHeadingLevel(t *testing.T) {
	c := check.New(t)
	c.Equal(HeadingLevel_None, UIAHeadingLevel(nil))
	c.Equal(HeadingLevel_None, UIAHeadingLevel(&accessibility.Node{Role: role.Heading}))
	c.Equal(HeadingLevel_None, UIAHeadingLevel(&accessibility.Node{Role: role.Label, Level: 2}))
	c.Equal(HeadingLevel1, UIAHeadingLevel(&accessibility.Node{Role: role.Heading, Level: 1}))
	c.Equal(HeadingLevel3, UIAHeadingLevel(&accessibility.Node{Role: role.Heading, Level: 3}))
	c.Equal(HeadingLevel9, UIAHeadingLevel(&accessibility.Node{Role: role.Heading, Level: 9}))
	c.Equal(HeadingLevel9, UIAHeadingLevel(&accessibility.Node{Role: role.Heading, Level: 12}))
	c.Equal(HeadingLevelID(80053), HeadingLevel3)
}

// TestUIAOrientation verifies the orientation mapping.
func TestUIAOrientation(t *testing.T) {
	c := check.New(t)
	c.Equal(OrientationType_None, UIAOrientation(nil))
	c.Equal(OrientationType_None, UIAOrientation(&accessibility.Node{Role: role.Slider}))
	c.Equal(OrientationType_Horizontal, UIAOrientation(&accessibility.Node{
		Role:        role.Slider,
		Orientation: accessibility.OrientationHorizontal,
	}))
	c.Equal(OrientationType_Vertical, UIAOrientation(&accessibility.Node{
		Role:        role.ScrollBar,
		Orientation: accessibility.OrientationVertical,
	}))
}

// TestUIAToggleState verifies that a toggle button reports its pressed state while everything else checkable reports its
// check state, and that a mixed check becomes indeterminate rather than on.
func TestUIAToggleState(t *testing.T) {
	c := check.New(t)
	c.Equal(ToggleState_Off, UIAToggleState(nil))
	c.Equal(ToggleState_Off, UIAToggleState(&accessibility.Node{Role: role.ToggleButton}))
	c.Equal(ToggleState_On, UIAToggleState(&accessibility.Node{Role: role.ToggleButton, Pressed: true}))
	c.Equal(ToggleState_Off, UIAToggleState(&accessibility.Node{
		Role: role.CheckBox, HasCheck: true, Checked: checkenum.Off,
	}))
	c.Equal(ToggleState_On, UIAToggleState(&accessibility.Node{
		Role: role.CheckBox, HasCheck: true, Checked: checkenum.On,
	}))
	c.Equal(ToggleState_Indeterminate, UIAToggleState(&accessibility.Node{
		Role: role.CheckBox, HasCheck: true, Checked: checkenum.Mixed,
	}))
	c.Equal(ToggleState_On, UIAToggleState(&accessibility.Node{
		Role: role.MenuItem, HasCheck: true, Checked: checkenum.On,
	}))
}

// TestUIAExpandCollapseState verifies that a node which cannot expand is reported as a leaf, which is a different answer
// from being collapsed.
func TestUIAExpandCollapseState(t *testing.T) {
	c := check.New(t)
	c.Equal(ExpandCollapseState_LeafNode, UIAExpandCollapseState(nil))
	c.Equal(ExpandCollapseState_LeafNode, UIAExpandCollapseState(&accessibility.Node{Role: role.Row}))
	c.Equal(ExpandCollapseState_Collapsed, UIAExpandCollapseState(&accessibility.Node{
		Role: role.Row, Expandable: true,
	}))
	c.Equal(ExpandCollapseState_Expanded, UIAExpandCollapseState(&accessibility.Node{
		Role: role.Row, Expandable: true, Expanded: true,
	}))
}

// TestUIAItemStatus verifies that the item status reports a column header's sort direction and nothing otherwise.
func TestUIAItemStatus(t *testing.T) {
	c := check.New(t)
	c.Equal("", UIAItemStatus(nil))
	c.Equal("", UIAItemStatus(&accessibility.Node{Role: role.ColumnHeader}))
	c.Equal("ascending", UIAItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortAscending,
	}))
	c.Equal("descending", UIAItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortDescending,
	}))
}

// TestUIAWindowState verifies the window interaction state and the RangeValue increments.
func TestUIAWindowState(t *testing.T) {
	c := check.New(t)
	c.Equal(WindowInteractionState_ReadyForUserInteraction, UIAWindowInteractionState(nil))
	c.Equal(WindowInteractionState_ReadyForUserInteraction, UIAWindowInteractionState(&accessibility.Node{
		Role: role.Window,
	}))
	c.Equal(WindowInteractionState_BlockedByModalWindow, UIAWindowInteractionState(&accessibility.Node{
		Role: role.Window, Disabled: true,
	}))

	c.Equal(float64(0), UIASmallChange(nil))
	c.Equal(float64(0), UIALargeChange(nil))
	slider := &accessibility.Node{Role: role.Slider, HasNumber: true, Step: 0.5}
	c.Equal(0.5, UIASmallChange(slider))
	c.Equal(5.0, UIALargeChange(slider))
}

// TestUIARuntimeID verifies that a node id survives being split across the two 32-bit halves a runtime identifier is
// made of, including an id large enough to need the high half.
func TestUIARuntimeID(t *testing.T) {
	c := check.New(t)
	c.Equal([]int32{UiaAppendRuntimeId, 1, 0}, UIARuntimeID(1))
	c.Equal([]int32{UiaAppendRuntimeId, -1, 0}, UIARuntimeID(0xFFFFFFFF))
	c.Equal([]int32{UiaAppendRuntimeId, 0, 1}, UIARuntimeID(0x100000000))
	c.Equal([]int32{UiaAppendRuntimeId, 2, 3}, UIARuntimeID(0x300000002))
}

// TestUIANavigate verifies that navigation runs over the unignored tree: the two layers of ignored grouping in
// sampleTree must be invisible, so the window's children are the controls inside them.
func TestUIANavigate(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()
	for i, one := range []struct {
		from      accessibility.NodeID
		direction NavigateDirection
		want      accessibility.NodeID
	}{
		{from: 1, direction: NavigateDirection_FirstChild, want: 4},
		{from: 1, direction: NavigateDirection_LastChild, want: 7},
		{from: 1, direction: NavigateDirection_Parent, want: 0},
		{from: 1, direction: NavigateDirection_NextSibling, want: 0},
		{from: 1, direction: NavigateDirection_PreviousSibling, want: 0},
		{from: 4, direction: NavigateDirection_Parent, want: 1},
		{from: 4, direction: NavigateDirection_PreviousSibling, want: 0},
		{from: 4, direction: NavigateDirection_NextSibling, want: 5},
		{from: 5, direction: NavigateDirection_NextSibling, want: 6},
		{from: 6, direction: NavigateDirection_NextSibling, want: 7},
		{from: 6, direction: NavigateDirection_PreviousSibling, want: 5},
		{from: 7, direction: NavigateDirection_NextSibling, want: 0},
		{from: 7, direction: NavigateDirection_FirstChild, want: 8},
		{from: 7, direction: NavigateDirection_LastChild, want: 9},
		{from: 8, direction: NavigateDirection_Parent, want: 7},
		{from: 8, direction: NavigateDirection_NextSibling, want: 9},
		{from: 9, direction: NavigateDirection_NextSibling, want: 0},
		{from: 4, direction: NavigateDirection_FirstChild, want: 0},
		{from: 0, direction: NavigateDirection_FirstChild, want: 0},
		{from: 99, direction: NavigateDirection_Parent, want: 0},
	} {
		c.Equal(one.want, UIANavigate(tree, one.from, one.direction), "case %d", i)
	}
	c.Equal(accessibility.NodeID(0), UIANavigate(nil, 1, NavigateDirection_FirstChild))
	c.Equal(accessibility.NodeID(0), UIANavigate(tree, 1, NavigateDirection(99)))
}

// TestUIAHitTest verifies that a point resolves to the deepest node covering it, that overlapping siblings resolve to
// the first of them, that an offscreen node is passed over, and that a hit on a node with no provider resolves to the
// nearest ancestor that has one.
func TestUIAHitTest(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()
	c.Equal(accessibility.NodeID(4), UIAHitTest(tree, geom.NewPoint(10, 10)))
	c.Equal(accessibility.NodeID(5), UIAHitTest(tree, geom.NewPoint(60, 10)))
	c.Equal(accessibility.NodeID(6), UIAHitTest(tree, geom.NewPoint(10, 60)))
	c.Equal(accessibility.NodeID(8), UIAHitTest(tree, geom.NewPoint(10, 110)), "the first of two stacked siblings wins")
	c.Equal(accessibility.NodeID(7), UIAHitTest(tree, geom.NewPoint(150, 150)))
	c.Equal(accessibility.NodeID(1), UIAHitTest(tree, geom.NewPoint(150, 10)),
		"a hit on an ignored group reports the nearest unignored ancestor")
	c.Equal(accessibility.NodeID(0), UIAHitTest(tree, geom.NewPoint(500, 500)))
	c.Equal(accessibility.NodeID(0), UIAHitTest(nil, geom.NewPoint(10, 10)))

	tree.Node(8).Offscreen = true
	c.Equal(accessibility.NodeID(9), UIAHitTest(tree, geom.NewPoint(10, 110)), "an offscreen node is passed over")
}

// TestUIAPositionInSet verifies that rows are numbered from what the snapshot recorded rather than from what is in the
// tree, since a table publishes only the rows in its viewport, while tabs and menu items are numbered by counting
// siblings.
func TestUIAPositionInSet(t *testing.T) {
	c := check.New(t)
	table := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{
			ID: 2, Role: role.Table, RowCount: 500, ColumnCount: 2,
			Children: []accessibility.NodeID{3, 4},
		},
		&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 120, Children: []accessibility.NodeID{5}},
		&accessibility.Node{ID: 4, Role: role.Row, RowIndex: 121},
		&accessibility.Node{ID: 5, Role: role.Cell, RowIndex: 120, ColumnIndex: 0},
	)
	position, size := UIAPositionInSet(table, table.Node(3))
	c.Equal(121, position)
	c.Equal(500, size)
	position, size = UIAPositionInSet(table, table.Node(4))
	c.Equal(122, position)
	c.Equal(500, size)
	position, size = UIAPositionInSet(table, table.Node(5))
	c.Equal(0, position, "a cell is not one of a numbered set")
	c.Equal(0, size)
	position, size = UIAPositionInSet(table, table.Node(2))
	c.Equal(0, position)
	c.Equal(0, size)

	// A row whose recorded index is out of step with the container's count falls back to counting siblings, which is
	// also what a list with no recorded count does.
	list := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.List, Children: []accessibility.NodeID{3, 4, 5}},
		&accessibility.Node{ID: 3, Role: role.ListItem},
		&accessibility.Node{ID: 4, Role: role.Separator},
		&accessibility.Node{ID: 5, Role: role.ListItem},
	)
	position, size = UIAPositionInSet(list, list.Node(5))
	c.Equal(2, position, "the separator between the items is not counted")
	c.Equal(2, size)

	tabs := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.TabList, Children: []accessibility.NodeID{3, 4}},
		&accessibility.Node{ID: 3, Role: role.Tab, Selected: true},
		&accessibility.Node{ID: 4, Role: role.Tab},
	)
	position, size = UIAPositionInSet(tabs, tabs.Node(4))
	c.Equal(2, position)
	c.Equal(2, size)

	position, size = UIAPositionInSet(nil, nil)
	c.Equal(0, position)
	c.Equal(0, size)
}

// eventTree builds the hierarchy the event-decision tests work over. Each call produces fresh nodes, so a test may
// mutate one side of a comparison freely.
//
//	1 window [focused]
//	├─ 2 text-field [focused]
//	├─ 3 check-box
//	├─ 4 list [multiselectable]
//	│  ├─ 5 list-item [selected]
//	│  └─ 6 list-item
//	├─ 7 radio-button [checked]
//	├─ 8 slider
//	└─ 9 group [ignored]
//	   └─ 10 button
//
// The radio button hangs directly off the window, which is not multiselectable, so it exercises the single-selection
// path while the list items exercise the multiple-selection one.
func eventTree() *accessibility.Tree {
	return newTestTree(1, 2,
		&accessibility.Node{
			ID: 1, Role: role.Window, Name: "Window", Focused: true,
			Children: []accessibility.NodeID{2, 3, 4, 7, 8, 9},
		},
		&accessibility.Node{
			ID: 2, Role: role.TextField, Focusable: true, Focused: true, Value: "hello",
			Text: &accessibility.TextInfo{Text: "hello", SelStart: 5, SelEnd: 5},
		},
		&accessibility.Node{ID: 3, Role: role.CheckBox, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{
			ID: 4, Role: role.List, Multiselectable: true,
			Children: []accessibility.NodeID{5, 6},
		},
		&accessibility.Node{ID: 5, Role: role.ListItem, Selectable: true, Selected: true},
		&accessibility.Node{ID: 6, Role: role.ListItem, Selectable: true},
		&accessibility.Node{ID: 7, Role: role.RadioButton, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{ID: 8, Role: role.Slider, HasNumber: true, Number: 5, Max: 10, Step: 1},
		&accessibility.Node{ID: 9, Role: role.Group, Ignored: true, Children: []accessibility.NodeID{10}},
		&accessibility.Node{ID: 10, Role: role.Button, Name: "Deep"},
	)
}

// raiseEvent, raiseProperty, raiseStructure, raiseNotify and raiseDisconnect build the expected results of
// UIADecideRaises, so that a test reads as a list of calls rather than as a list of struct literals.
func raiseEvent(node accessibility.NodeID, id EventID) UIARaise {
	return UIARaise{Kind: UIARaiseEvent, Node: node, Event: id}
}

func raiseProperty(node accessibility.NodeID, id PropertyID) UIARaise {
	return UIARaise{Kind: UIARaiseProperty, Node: node, Property: id}
}

func raiseStructure(node, child accessibility.NodeID, change StructureChangeType) UIARaise {
	return UIARaise{Kind: UIARaiseStructure, Node: node, Child: child, Change: change}
}

func raiseNotify(node accessibility.NodeID, text string) UIARaise {
	return UIARaise{Kind: UIARaiseNotify, Node: node, Text: text}
}

func raiseDisconnect(node accessibility.NodeID) UIARaise {
	return UIARaise{Kind: UIARaiseDisconnect, Node: node}
}

// TestUIADecideRaisesNothing verifies the empty cases: no current tree at all, and a batch with no events in it.
func TestUIADecideRaisesNothing(t *testing.T) {
	c := check.New(t)
	c.Nil(UIADecideRaises(nil, nil, nil))
	c.Nil(UIADecideRaises(eventTree(), nil, nil))
	c.Nil(UIADecideRaises(eventTree(), eventTree(), nil))
}

// TestUIADecideRaisesFocus verifies that a focus change is reported only while the window itself is active. A screen
// reader follows a focus event by moving its cursor, so raising one for a window the user is not looking at pulls them
// away from the one they are.
func TestUIADecideRaisesFocus(t *testing.T) {
	c := check.New(t)
	events := []accessibility.Event{{Kind: accessibility.FocusChanged, Node: 2}}

	cur := eventTree()
	c.Equal([]UIARaise{raiseEvent(2, UIA_AutomationFocusChangedEventId)}, UIADecideRaises(eventTree(), cur, events))

	cur = eventTree()
	cur.Node(1).Focused = false
	c.Nil(UIADecideRaises(eventTree(), cur, events))
}

// TestUIADecideRaisesWindowOpened verifies that the first publish of a dialog announces the dialog, which is how a
// screen reader knows to read the whole thing out, and that an ordinary window does not.
func TestUIADecideRaisesWindowOpened(t *testing.T) {
	c := check.New(t)
	dialog := eventTree()
	dialog.Node(1).Role = role.Dialog
	c.Equal([]UIARaise{
		raiseEvent(1, UIA_Window_WindowOpenedEventId),
		raiseEvent(2, UIA_AutomationFocusChangedEventId),
	}, UIADecideRaises(nil, dialog, accessibility.Diff(nil, dialog)))

	window := eventTree()
	c.Equal([]UIARaise{raiseEvent(2, UIA_AutomationFocusChangedEventId)},
		UIADecideRaises(nil, window, accessibility.Diff(nil, window)))

	// Every publish after the first one has a previous snapshot to compare against, and announces nothing: a dialog that
	// said it had opened on every redraw would be read out again each time.
	next := eventTree()
	next.Node(1).Role = role.Dialog
	next.Node(2).Name = "Renamed"
	c.Equal([]UIARaise{raiseProperty(2, UIA_NamePropertyId)},
		UIADecideRaises(dialog, next, accessibility.Diff(dialog, next)))
}

// TestUIADecideRaisesInvalidationOrder verifies that a parent reporting all of its children invalidated drops the
// per-child events under it however the batch is ordered. The diff reports the invalidation first, but the decision
// must not depend on that: it is the presence of the invalidation in the batch that makes the child events redundant,
// not its position.
func TestUIADecideRaisesInvalidationOrder(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(4).Children = []accessibility.NodeID{5, 11}
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 4, Role: role.ListItem, Selectable: true}
	delete(cur.Nodes, 6)

	expected := []UIARaise{
		raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated),
		raiseDisconnect(6),
	}
	c.Equal(expected, UIADecideRaises(old, cur, []accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 4},
		{Kind: accessibility.NodeAdded, Node: 11},
		{Kind: accessibility.NodeRemoved, Node: 6},
	}))
	c.Equal([]UIARaise{
		raiseDisconnect(6),
		raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated),
	}, UIADecideRaises(old, cur, []accessibility.Event{
		{Kind: accessibility.NodeRemoved, Node: 6},
		{Kind: accessibility.NodeAdded, Node: 11},
		{Kind: accessibility.ChildrenChanged, Node: 4},
	}), "the invalidation still drops the child events when it arrives last")
}

// TestUIADecideRaisesTextProperties verifies the straightforward one-event-to-one-property translations.
func TestUIADecideRaisesTextProperties(t *testing.T) {
	c := check.New(t)
	cur := eventTree()
	c.Equal([]UIARaise{
		raiseProperty(2, UIA_NamePropertyId),
		raiseProperty(2, UIA_HelpTextPropertyId),
		raiseProperty(3, UIA_ItemStatusPropertyId),
	}, UIADecideRaises(eventTree(), cur, []accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 2, Old: "a", New: "b"},
		{Kind: accessibility.DescriptionChanged, Node: 2},
		{Kind: accessibility.SortChanged, Node: 3},
	}))
}

// TestUIADecideRaisesValue verifies that a value change becomes whichever value property the element actually has. A
// text field reports its value through the Value pattern and a slider through RangeValue, so raising the Value pattern's
// property on a slider would be telling a client about a property the element does not support; a label has neither, so
// its value change is dropped.
func TestUIADecideRaisesValue(t *testing.T) {
	c := check.New(t)
	cur := eventTree()
	c.Equal([]UIARaise{raiseProperty(2, UIA_ValueValuePropertyId)}, UIADecideRaises(eventTree(), cur,
		[]accessibility.Event{{Kind: accessibility.ValueChanged, Node: 2}}))
	c.Equal([]UIARaise{raiseProperty(8, UIA_RangeValueValuePropertyId)}, UIADecideRaises(eventTree(), cur,
		[]accessibility.Event{{Kind: accessibility.ValueChanged, Node: 8}}))
	c.Equal([]UIARaise{raiseProperty(8, UIA_RangeValueValuePropertyId)}, UIADecideRaises(eventTree(), cur,
		[]accessibility.Event{{Kind: accessibility.NumberChanged, Node: 8}}))
	c.Nil(UIADecideRaises(eventTree(), cur, []accessibility.Event{{Kind: accessibility.NumberChanged, Node: 3}}))

	// A slider reporting both a textual and a numeric change still tells the client once.
	c.Equal([]UIARaise{raiseProperty(8, UIA_RangeValueValuePropertyId)}, UIADecideRaises(eventTree(), cur,
		[]accessibility.Event{
			{Kind: accessibility.ValueChanged, Node: 8},
			{Kind: accessibility.NumberChanged, Node: 8},
		}))
}

// TestUIADecideRaisesText verifies that an edit reports the text change and the new value once each, even though the
// diff describes the edit as a deletion followed by an insertion and reports the value separately. The order follows the
// diff's, which reports the value before the text it came from.
func TestUIADecideRaisesText(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{
		raiseProperty(2, UIA_ValueValuePropertyId),
		raiseEvent(2, UIA_Text_TextChangedEventId),
		raiseEvent(2, UIA_Text_TextSelectionChangedEventId),
	}, UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.ValueChanged, Node: 2, Old: "hello", New: "help"},
		{Kind: accessibility.TextDeleted, Node: 2, Start: 3, Length: 2, Old: "lo"},
		{Kind: accessibility.TextInserted, Node: 2, Start: 3, Length: 1, New: "p"},
		{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 4},
	}))
}

// TestUIADecideRaisesStates verifies the state flags that map onto a property of their own, and that a flag belonging to
// a pattern the element does not support raises nothing.
func TestUIADecideRaisesStates(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		expected []UIARaise
		node     accessibility.NodeID
		state    accessibility.State
	}{
		{state: accessibility.StateDisabled, node: 2, expected: []UIARaise{
			raiseProperty(2, UIA_IsEnabledPropertyId),
		}},
		{state: accessibility.StateFocusable, node: 2, expected: []UIARaise{
			raiseProperty(2, UIA_IsKeyboardFocusablePropertyId),
		}},
		{state: accessibility.StateOffscreen, node: 2, expected: []UIARaise{
			raiseProperty(2, UIA_IsOffscreenPropertyId),
		}},
		{state: accessibility.StateInvalid, node: 2, expected: []UIARaise{
			raiseProperty(2, UIA_IsDataValidForFormPropertyId),
		}},
		{state: accessibility.StateReadOnly, node: 2, expected: []UIARaise{
			raiseProperty(2, UIA_ValueIsReadOnlyPropertyId),
		}},
		{state: accessibility.StateReadOnly, node: 8, expected: []UIARaise{
			raiseProperty(8, UIA_RangeValueIsReadOnlyPropertyId),
		}},
		{state: accessibility.StateChecked, node: 3, expected: []UIARaise{
			raiseProperty(3, UIA_ToggleToggleStatePropertyId),
		}},
		{state: accessibility.StatePressed, node: 3, expected: []UIARaise{
			raiseProperty(3, UIA_ToggleToggleStatePropertyId),
		}},
		{state: accessibility.StateExpanded, node: 3},
		{state: accessibility.StateModal, node: 1},
		{state: accessibility.StateBusy, node: 1},
		{state: accessibility.StateProtected, node: 2},
		{state: accessibility.StateReadOnly, node: 1},
	} {
		raises := UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: one.node, State: one.state},
		})
		if one.expected == nil {
			c.Nil(raises, "case %d (%s)", i, one.state)
			continue
		}
		c.Equal(one.expected, raises, "case %d (%s)", i, one.state)
	}
}

// TestUIADecideRaisesExpandCollapse verifies that a change to either half of the expandable state reports the combined
// ExpandCollapseState property, since that one property carries both.
func TestUIADecideRaisesExpandCollapse(t *testing.T) {
	c := check.New(t)
	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Tree, RowCount: 1, Children: []accessibility.NodeID{3}},
		&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Expandable: true, Expanded: true},
	)
	for _, state := range []accessibility.State{accessibility.StateExpanded, accessibility.StateExpandable} {
		c.Equal([]UIARaise{raiseProperty(3, UIA_ExpandCollapseExpandCollapseStatePropertyId)},
			UIADecideRaises(tree, tree, []accessibility.Event{
				{Kind: accessibility.StateChanged, Node: 3, State: state},
			}), "%s", state)
	}
}

// TestUIADecideRaisesSelection verifies the three shapes a selection change takes: a container that allows several
// selections reports each element joining and leaving, while one that does not reports only the element that became the
// selection, since that implicitly deselects whatever was selected before.
func TestUIADecideRaisesSelection(t *testing.T) {
	c := check.New(t)

	multi := eventTree()
	c.Equal([]UIARaise{
		raiseProperty(5, UIA_SelectionItemIsSelectedPropertyId),
		raiseEvent(5, UIA_SelectionItem_ElementAddedToSelectionEventId),
	}, UIADecideRaises(eventTree(), multi, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
	}))

	deselected := eventTree()
	deselected.Node(5).Selected = false
	c.Equal([]UIARaise{
		raiseProperty(5, UIA_SelectionItemIsSelectedPropertyId),
		raiseEvent(5, UIA_SelectionItem_ElementRemovedFromSelectionEventId),
	}, UIADecideRaises(eventTree(), deselected, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
	}))

	single := eventTree()
	single.Node(4).Multiselectable = false
	c.Equal([]UIARaise{
		raiseProperty(5, UIA_SelectionItemIsSelectedPropertyId),
		raiseEvent(5, UIA_SelectionItem_ElementSelectedEventId),
	}, UIADecideRaises(eventTree(), single, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
	}))

	singleOff := eventTree()
	singleOff.Node(4).Multiselectable = false
	singleOff.Node(5).Selected = false
	c.Equal([]UIARaise{raiseProperty(5, UIA_SelectionItemIsSelectedPropertyId)},
		UIADecideRaises(eventTree(), singleOff, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 5, State: accessibility.StateSelected},
		}))
}

// TestUIADecideRaisesRadioButton verifies that a radio button reports being checked as being selected. It has no Toggle
// pattern, so a toggle-state property change would name a property the element does not support.
func TestUIADecideRaisesRadioButton(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{
		raiseProperty(7, UIA_SelectionItemIsSelectedPropertyId),
		raiseEvent(7, UIA_SelectionItem_ElementSelectedEventId),
	}, UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateChecked},
	}))

	unchecked := eventTree()
	unchecked.Node(7).Checked = checkenum.Off
	c.Equal([]UIARaise{raiseProperty(7, UIA_SelectionItemIsSelectedPropertyId)},
		UIADecideRaises(eventTree(), unchecked, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateChecked},
		}))
}

// TestUIADecideRaisesBounds verifies that only the focused node and the root report a new bounding rectangle. Resizing a
// window moves everything in it, and a client that wanted every rectangle would ask for them.
func TestUIADecideRaisesBounds(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{
		raiseProperty(1, UIA_BoundingRectanglePropertyId),
		raiseProperty(2, UIA_BoundingRectanglePropertyId),
	}, UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.BoundsChanged, Node: 1},
		{Kind: accessibility.BoundsChanged, Node: 2},
		{Kind: accessibility.BoundsChanged, Node: 3},
		{Kind: accessibility.BoundsChanged, Node: 8},
	}))
}

// TestUIADecideRaisesIgnored verifies that a node the snapshot marks Ignored never appears, since it has no provider for
// an event to be raised on.
func TestUIADecideRaisesIgnored(t *testing.T) {
	c := check.New(t)
	c.Nil(UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 9},
		{Kind: accessibility.StateChanged, Node: 9, State: accessibility.StateDisabled},
		{Kind: accessibility.BoundsChanged, Node: 9},
	}))
}

// TestUIADecideRaisesAdded verifies that a new node reports itself as added, and that it says nothing once its parent has
// already reported all of its children invalidated — which is what the diff produces for a real addition, since the
// parent's list of children changed too.
func TestUIADecideRaisesAdded(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(4).Children = append(cur.Node(4).Children, 11)
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 4, Role: role.ListItem, Selectable: true}

	c.Equal([]UIARaise{raiseStructure(11, 11, StructureChangeType_ChildAdded)},
		UIADecideRaises(old, cur, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 11}}))

	c.Equal([]UIARaise{raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestUIADecideRaisesRemoved verifies that a departed node reports its removal on the parent it left, since it no longer
// exists to raise anything itself, and that the disconnect releasing its provider happens either way.
func TestUIADecideRaisesRemoved(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(4).Children = []accessibility.NodeID{5}
	delete(cur.Nodes, 6)

	c.Equal([]UIARaise{
		raiseStructure(4, 6, StructureChangeType_ChildRemoved),
		raiseDisconnect(6),
	}, UIADecideRaises(old, cur, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 6}}))

	c.Equal([]UIARaise{
		raiseDisconnect(6),
		raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated),
	}, UIADecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestUIADecideRaisesRemovedSubtree verifies that removing a whole subtree reports the invalidation once on the surviving
// parent and disconnects every node that left, rather than trying to raise a removal on a parent that is gone too.
func TestUIADecideRaisesRemovedSubtree(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(1).Children = []accessibility.NodeID{2, 3, 7, 8, 9}
	delete(cur.Nodes, 4)
	delete(cur.Nodes, 5)
	delete(cur.Nodes, 6)

	c.Equal([]UIARaise{
		raiseDisconnect(4),
		raiseDisconnect(5),
		raiseDisconnect(6),
		raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated),
	}, UIADecideRaises(old, cur, accessibility.Diff(old, cur)))
}

// TestUIADecideRaisesWindowActivation verifies that a window becoming active points the client back at whatever inside it
// has the focus, and that a window losing it says nothing at all.
func TestUIADecideRaisesWindowActivation(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{raiseEvent(2, UIA_AutomationFocusChangedEventId)},
		UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.WindowActivated, Node: 1},
		}))

	deactivated := eventTree()
	deactivated.Node(1).Focused = false
	c.Nil(UIADecideRaises(eventTree(), deactivated, []accessibility.Event{
		{Kind: accessibility.WindowDeactivated, Node: 1},
	}))
}

// TestUIADecideRaisesAnnouncement verifies that an announcement is aimed at the root and that saying the same thing twice
// really does say it twice, unlike every other kind of call, which is deduplicated.
func TestUIADecideRaisesAnnouncement(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{
		raiseNotify(1, "Saved"),
		raiseNotify(1, "Saved"),
	}, UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.Announcement, New: "Saved"},
		{Kind: accessibility.Announcement, New: "Saved"},
		{Kind: accessibility.Announcement, New: ""},
	}))
}

// TestUIADecideRaisesDeterminism verifies that the same trees and events always produce the same calls in the same
// order, which the map iteration inside the decision logic could otherwise break.
func TestUIADecideRaisesDeterminism(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(2).Value = "help"
	cur.Node(2).Text = &accessibility.TextInfo{Text: "help", SelStart: 4, SelEnd: 4}
	cur.Node(3).Checked = checkenum.Off
	cur.Node(5).Selected = false
	cur.Node(8).Number = 6
	cur.Node(1).Children = []accessibility.NodeID{2, 3, 4, 7, 8}
	delete(cur.Nodes, 9)
	delete(cur.Nodes, 10)
	events := accessibility.Diff(old, cur)
	c.True(len(events) > 4)
	first := UIADecideRaises(old, cur, events)
	c.True(len(first) > 4)
	for range 8 {
		c.Equal(first, UIADecideRaises(old, cur, events))
	}
}

// TestUIARaiseStrings verifies that the diagnostic strings name each kind and carry the field that matters for it, since
// a failing provider test is read through them.
func TestUIARaiseStrings(t *testing.T) {
	c := check.New(t)
	c.Equal("event", UIARaiseEvent.String())
	c.Equal("property", UIARaiseProperty.String())
	c.Equal("structure", UIARaiseStructure.String())
	c.Equal("notify", UIARaiseNotify.String())
	c.Equal("disconnect", UIARaiseDisconnect.String())
	c.Equal("UIARaiseKind(9)", UIARaiseKind(9).String())
	c.Equal("event{node:2,event:20005}", raiseEvent(2, UIA_AutomationFocusChangedEventId).String())
	c.Equal("property{node:3,property:30005}", raiseProperty(3, UIA_NamePropertyId).String())
	c.Equal("structure{node:4,change:1,child:6}",
		raiseStructure(4, 6, StructureChangeType_ChildRemoved).String())
	c.Equal("structure{node:4,change:2}", raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated).String())
	c.Equal(`notify{node:1,text:"Saved"}`, raiseNotify(1, "Saved").String())
	c.Equal("disconnect{node:6}", raiseDisconnect(6).String())
}
