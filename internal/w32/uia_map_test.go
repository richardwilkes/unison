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
//	│  │  ├─ 4 button                 (0,0 50x20)    focused, pressable
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
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.Press),
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
//
// Every node here is the tree's root, which is what a window-like role needs to be to report the Window control type;
// TestUIAControlTypeNested covers the nested case.
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
		role.Table:              UIA_DataGridControlTypeId,
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
		// A document is a group: the document control type requires the Text pattern, which nothing here implements, so
		// a client would ask for the one pattern every document has and be handed NULL. The Cocoa adapter treats the
		// role the same way.
		role.Document: UIA_GroupControlTypeId,
		role.Toolbar:  UIA_ToolBarControlTypeId,
		role.Unknown:  UIA_CustomControlTypeId,
	}
	c.Equal(len(role.All), len(expected))
	for _, r := range role.All {
		want, ok := expected[r]
		c.True(ok, "no control type expected for role %s", r.Key())
		node := &accessibility.Node{ID: 1, Role: r}
		c.Equal(want, UIAControlType(newTestTree(1, 0, node), node), "role %s", r.Key())
	}
	c.Equal(UIA_CustomControlTypeId, UIAControlType(nil, nil))
}

// TestUIAControlTypeNested verifies that only the fragment root reports the Window control type. The Window control
// type lists IWindowProvider as a required pattern and the provider hands that interface out for the root alone, so a
// dialog-shaped panel inside a window must report Pane instead of advertising a pattern it refuses.
func TestUIAControlTypeNested(t *testing.T) {
	c := check.New(t)
	tree := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 3}},
		&accessibility.Node{ID: 2, Role: role.Dialog},
		&accessibility.Node{ID: 3, Role: role.Window},
	)
	c.Equal(UIA_WindowControlTypeId, UIAControlType(tree, tree.Node(1)))
	c.Equal(UIA_PaneControlTypeId, UIAControlType(tree, tree.Node(2)))
	c.Equal(UIA_PaneControlTypeId, UIAControlType(tree, tree.Node(3)))

	// With no tree to ask, nothing can be said to be the root, so the cautious answer is the nested one.
	c.Equal(UIA_PaneControlTypeId, UIAControlType(nil, tree.Node(1)))
}

// TestUIAPatterns verifies the role-to-patterns table, including the handful of roles whose patterns depend on state.
// Every node here is given no actions at all, so that the role's own answer is what is being checked;
// TestUIAPatternsScrollItem covers the one pattern that comes from the action set instead.
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
		{node: &accessibility.Node{Role: role.CheckBox}, patterns: PatternToggle},
		// A disclosure triangle expands whatever it is attached to, and the state it reports is reachable only through
		// the ExpandCollapse pattern, so the pattern follows the flag exactly as a row's and a menu item's do.
		{node: &accessibility.Node{Role: role.DisclosureTriangle}, patterns: PatternToggle},
		{
			node:     &accessibility.Node{Role: role.DisclosureTriangle, Expandable: true},
			patterns: PatternToggle | PatternExpandCollapse,
		},
		{node: &accessibility.Node{Role: role.RadioButton}, patterns: PatternSelectionItem},
		{node: &accessibility.Node{Role: role.TextField}, patterns: PatternValue},
		{node: &accessibility.Node{Role: role.TextArea}, patterns: PatternValue},
		// A document carries no value: the only thing that produces the role never fills one in, so the pattern would
		// answer a client with an empty string as the whole content of the document.
		{node: &accessibility.Node{Role: role.Document}},
		// A spin button has a range only once it has a number, exactly as a progress bar does: an obscured numeric
		// field fills in none of them, and the pattern would report a PIN field as zero.
		{node: &accessibility.Node{Role: role.SpinButton}, patterns: PatternValue},
		{
			node:     &accessibility.Node{Role: role.SpinButton, HasNumber: true},
			patterns: PatternValue | PatternRangeValue,
		},
		{node: &accessibility.Node{Role: role.ComboBox}, patterns: PatternValue | PatternExpandCollapse},
		{node: &accessibility.Node{Role: role.PopupButton}, patterns: PatternValue | PatternExpandCollapse},
		// A slider and a scroll bar have a range only once they have a number, for the reason a spin button and a
		// progress bar do: the roles are public API, and RangeValue answering zero for the value, the bounds and the
		// increments reads as "0 percent".
		{node: &accessibility.Node{Role: role.Slider}},
		{node: &accessibility.Node{Role: role.Slider, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.ScrollBar}},
		{node: &accessibility.Node{Role: role.ScrollBar, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.ProgressBar}},
		{node: &accessibility.Node{Role: role.ProgressBar, HasNumber: true}, patterns: PatternRangeValue},
		{node: &accessibility.Node{Role: role.List}, patterns: PatternSelection},
		{node: &accessibility.Node{Role: role.TabList}, patterns: PatternSelection},
		{node: &accessibility.Node{Role: role.ListItem}, patterns: PatternSelectionItem},
		{node: &accessibility.Node{Role: role.Tab}, patterns: PatternSelectionItem},
		{node: &accessibility.Node{Role: role.Table}, patterns: PatternGrid | PatternTable | PatternSelection},
		{node: &accessibility.Node{Role: role.Tree}, patterns: PatternGrid | PatternTable | PatternSelection},
		{node: &accessibility.Node{Role: role.Row}, patterns: PatternSelectionItem},
		{
			node:     &accessibility.Node{Role: role.Row, Expandable: true},
			patterns: PatternSelectionItem | PatternExpandCollapse,
		},
		{node: &accessibility.Node{Role: role.Cell}, patterns: PatternGridItem | PatternTableItem},
		// A cell holding one widget reports that widget's state as its own value, which a client can only read through
		// the Value pattern.
		{
			node:     &accessibility.Node{Role: role.Cell, Value: "checked"},
			patterns: PatternGridItem | PatternTableItem | PatternValue,
		},
		{node: &accessibility.Node{Role: role.Menu}},
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

// TestUIAPatternsScrollItem verifies that the ScrollItem pattern follows the ScrollIntoView action rather than the
// role. Every node in a snapshot carries that action — a disabled one keeps it when it keeps nothing else — and the
// pattern's only method does nothing but dispatch it, so anything a client may want to talk about can be brought into
// view: a cell scrolled off to the side, a column header, a tab, a menu item.
func TestUIAPatternsScrollItem(t *testing.T) {
	c := check.New(t)
	scrollable := accessibility.ActionSet(0).With(accessibility.ScrollIntoView)
	for _, r := range []role.Enum{
		role.Cell, role.ColumnHeader, role.Tab, role.MenuItem, role.Row, role.ListItem, role.Button, role.Group,
	} {
		with := UIAPatterns(&accessibility.Node{Role: r, Actions: scrollable})
		c.True(with.Has(PatternScrollItem), "role %s", r.Key())
		c.False(UIAPatterns(&accessibility.Node{Role: r}).Has(PatternScrollItem), "role %s without it", r.Key())
	}

	// The pattern is all a disabled row has left, which is what lets a screen reader scroll through a disabled table.
	c.Equal(PatternSelectionItem|PatternScrollItem,
		UIAPatterns(&accessibility.Node{Role: role.Row, Disabled: true, Actions: scrollable}))
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

// TestUIAIdentifierValues pins every hand-written number in uia_constants.go: the identifiers UI Automation defines,
// the provider options, the VARIANT type tags and VARIANT_BOOL values from wtypes.h, and the HRESULTs a provider
// refuses with. Nothing else does: the mapping tests compare one symbol against another, so a transposed value —
// DataGrid's 50028 written where DataItem's 50029 belongs, or one property id given another's number — would pass the
// whole suite while making a screen reader describe elements as the wrong kind of thing, or read a property no client
// asked for.
//
// A client resolves none of these by name, so the numbers are the entire interface. They are the values in the Windows
// SDK's uiautomationcoreapi.h, uiautomationcore.idl and wtypes.h, which is where a reader checks them.
func TestUIAIdentifierValues(t *testing.T) {
	c := check.New(t)

	// Control types, the value of UIA_ControlTypePropertyId.
	c.Equal(ControlTypeID(50000), UIA_ButtonControlTypeId)
	c.Equal(ControlTypeID(50002), UIA_CheckBoxControlTypeId)
	c.Equal(ControlTypeID(50003), UIA_ComboBoxControlTypeId)
	c.Equal(ControlTypeID(50004), UIA_EditControlTypeId)
	c.Equal(ControlTypeID(50005), UIA_HyperlinkControlTypeId)
	c.Equal(ControlTypeID(50006), UIA_ImageControlTypeId)
	c.Equal(ControlTypeID(50007), UIA_ListItemControlTypeId)
	c.Equal(ControlTypeID(50008), UIA_ListControlTypeId)
	c.Equal(ControlTypeID(50009), UIA_MenuControlTypeId)
	c.Equal(ControlTypeID(50010), UIA_MenuBarControlTypeId)
	c.Equal(ControlTypeID(50011), UIA_MenuItemControlTypeId)
	c.Equal(ControlTypeID(50012), UIA_ProgressBarControlTypeId)
	c.Equal(ControlTypeID(50013), UIA_RadioButtonControlTypeId)
	c.Equal(ControlTypeID(50014), UIA_ScrollBarControlTypeId)
	c.Equal(ControlTypeID(50015), UIA_SliderControlTypeId)
	c.Equal(ControlTypeID(50016), UIA_SpinnerControlTypeId)
	c.Equal(ControlTypeID(50018), UIA_TabControlTypeId)
	c.Equal(ControlTypeID(50019), UIA_TabItemControlTypeId)
	c.Equal(ControlTypeID(50020), UIA_TextControlTypeId)
	c.Equal(ControlTypeID(50021), UIA_ToolBarControlTypeId)
	c.Equal(ControlTypeID(50022), UIA_ToolTipControlTypeId)
	c.Equal(ControlTypeID(50023), UIA_TreeControlTypeId)
	c.Equal(ControlTypeID(50024), UIA_TreeItemControlTypeId)
	c.Equal(ControlTypeID(50025), UIA_CustomControlTypeId)
	c.Equal(ControlTypeID(50026), UIA_GroupControlTypeId)
	c.Equal(ControlTypeID(50028), UIA_DataGridControlTypeId)
	c.Equal(ControlTypeID(50029), UIA_DataItemControlTypeId)
	c.Equal(ControlTypeID(50030), UIA_DocumentControlTypeId)
	c.Equal(ControlTypeID(50032), UIA_WindowControlTypeId)
	c.Equal(ControlTypeID(50033), UIA_PaneControlTypeId)
	c.Equal(ControlTypeID(50034), UIA_HeaderControlTypeId)
	c.Equal(ControlTypeID(50035), UIA_HeaderItemControlTypeId)
	c.Equal(ControlTypeID(50036), UIA_TableControlTypeId)
	c.Equal(ControlTypeID(50038), UIA_SeparatorControlTypeId)

	// Control patterns, which a client asks for by identifier through GetPatternProvider.
	c.Equal(PatternID(10000), UIA_InvokePatternId)
	c.Equal(PatternID(10001), UIA_SelectionPatternId)
	c.Equal(PatternID(10002), UIA_ValuePatternId)
	c.Equal(PatternID(10003), UIA_RangeValuePatternId)
	c.Equal(PatternID(10004), UIA_ScrollPatternId)
	c.Equal(PatternID(10005), UIA_ExpandCollapsePatternId)
	c.Equal(PatternID(10006), UIA_GridPatternId)
	c.Equal(PatternID(10007), UIA_GridItemPatternId)
	c.Equal(PatternID(10009), UIA_WindowPatternId)
	c.Equal(PatternID(10010), UIA_SelectionItemPatternId)
	c.Equal(PatternID(10012), UIA_TablePatternId)
	c.Equal(PatternID(10013), UIA_TableItemPatternId)
	c.Equal(PatternID(10014), UIA_TextPatternId)
	c.Equal(PatternID(10015), UIA_TogglePatternId)
	c.Equal(PatternID(10017), UIA_ScrollItemPatternId)
	c.Equal(PatternID(10024), UIA_TextPattern2Id)

	// Events, every one of which is raised by identifier.
	c.Equal(EventID(20000), UIA_ToolTipOpenedEventId)
	c.Equal(EventID(20001), UIA_ToolTipClosedEventId)
	c.Equal(EventID(20002), UIA_StructureChangedEventId)
	c.Equal(EventID(20003), UIA_MenuOpenedEventId)
	c.Equal(EventID(20004), UIA_AutomationPropertyChangedEventId)
	c.Equal(EventID(20005), UIA_AutomationFocusChangedEventId)
	c.Equal(EventID(20007), UIA_MenuClosedEventId)
	c.Equal(EventID(20009), UIA_Invoke_InvokedEventId)
	c.Equal(EventID(20010), UIA_SelectionItem_ElementAddedToSelectionEventId)
	c.Equal(EventID(20011), UIA_SelectionItem_ElementRemovedFromSelectionEventId)
	c.Equal(EventID(20012), UIA_SelectionItem_ElementSelectedEventId)
	c.Equal(EventID(20014), UIA_Text_TextSelectionChangedEventId)
	c.Equal(EventID(20015), UIA_Text_TextChangedEventId)
	c.Equal(EventID(20016), UIA_Window_WindowOpenedEventId)
	c.Equal(EventID(20017), UIA_Window_WindowClosedEventId)
	c.Equal(EventID(20024), UIA_LiveRegionChangedEventId)
	c.Equal(EventID(20035), UIA_NotificationEventId)

	// Properties, both the element-wide ones and the ones belonging to a control pattern.
	c.Equal(PropertyID(30000), UIA_RuntimeIdPropertyId)
	c.Equal(PropertyID(30001), UIA_BoundingRectanglePropertyId)
	c.Equal(PropertyID(30002), UIA_ProcessIdPropertyId)
	c.Equal(PropertyID(30003), UIA_ControlTypePropertyId)
	c.Equal(PropertyID(30004), UIA_LocalizedControlTypePropertyId)
	c.Equal(PropertyID(30005), UIA_NamePropertyId)
	c.Equal(PropertyID(30006), UIA_AcceleratorKeyPropertyId)
	c.Equal(PropertyID(30007), UIA_AccessKeyPropertyId)
	c.Equal(PropertyID(30008), UIA_HasKeyboardFocusPropertyId)
	c.Equal(PropertyID(30009), UIA_IsKeyboardFocusablePropertyId)
	c.Equal(PropertyID(30010), UIA_IsEnabledPropertyId)
	c.Equal(PropertyID(30011), UIA_AutomationIdPropertyId)
	c.Equal(PropertyID(30012), UIA_ClassNamePropertyId)
	c.Equal(PropertyID(30013), UIA_HelpTextPropertyId)
	c.Equal(PropertyID(30016), UIA_IsControlElementPropertyId)
	c.Equal(PropertyID(30017), UIA_IsContentElementPropertyId)
	c.Equal(PropertyID(30018), UIA_LabeledByPropertyId)
	c.Equal(PropertyID(30019), UIA_IsPasswordPropertyId)
	c.Equal(PropertyID(30020), UIA_NativeWindowHandlePropertyId)
	c.Equal(PropertyID(30022), UIA_IsOffscreenPropertyId)
	c.Equal(PropertyID(30023), UIA_OrientationPropertyId)
	c.Equal(PropertyID(30024), UIA_FrameworkIdPropertyId)
	c.Equal(PropertyID(30026), UIA_ItemStatusPropertyId)
	c.Equal(PropertyID(30103), UIA_IsDataValidForFormPropertyId)
	c.Equal(PropertyID(30104), UIA_ControllerForPropertyId)
	c.Equal(PropertyID(30105), UIA_DescribedByPropertyId)
	c.Equal(PropertyID(30107), UIA_ProviderDescriptionPropertyId)
	c.Equal(PropertyID(30135), UIA_LiveSettingPropertyId)
	c.Equal(PropertyID(30152), UIA_PositionInSetPropertyId)
	c.Equal(PropertyID(30153), UIA_SizeOfSetPropertyId)
	c.Equal(PropertyID(30154), UIA_LevelPropertyId)
	c.Equal(PropertyID(30159), UIA_FullDescriptionPropertyId)
	c.Equal(PropertyID(30173), UIA_HeadingLevelPropertyId)
	c.Equal(PropertyID(30174), UIA_IsDialogPropertyId)
	c.Equal(PropertyID(30045), UIA_ValueValuePropertyId)
	c.Equal(PropertyID(30046), UIA_ValueIsReadOnlyPropertyId)
	c.Equal(PropertyID(30047), UIA_RangeValueValuePropertyId)
	c.Equal(PropertyID(30048), UIA_RangeValueIsReadOnlyPropertyId)
	c.Equal(PropertyID(30049), UIA_RangeValueMinimumPropertyId)
	c.Equal(PropertyID(30050), UIA_RangeValueMaximumPropertyId)
	c.Equal(PropertyID(30051), UIA_RangeValueLargeChangePropertyId)
	c.Equal(PropertyID(30052), UIA_RangeValueSmallChangePropertyId)
	c.Equal(PropertyID(30059), UIA_SelectionSelectionPropertyId)
	c.Equal(PropertyID(30060), UIA_SelectionCanSelectMultiplePropertyId)
	c.Equal(PropertyID(30061), UIA_SelectionIsSelectionRequiredPropertyId)
	c.Equal(PropertyID(30062), UIA_GridRowCountPropertyId)
	c.Equal(PropertyID(30063), UIA_GridColumnCountPropertyId)
	c.Equal(PropertyID(30064), UIA_GridItemRowPropertyId)
	c.Equal(PropertyID(30065), UIA_GridItemColumnPropertyId)
	c.Equal(PropertyID(30066), UIA_GridItemRowSpanPropertyId)
	c.Equal(PropertyID(30067), UIA_GridItemColumnSpanPropertyId)
	c.Equal(PropertyID(30068), UIA_GridItemContainingGridPropertyId)
	c.Equal(PropertyID(30070), UIA_ExpandCollapseExpandCollapseStatePropertyId)
	c.Equal(PropertyID(30073), UIA_WindowCanMaximizePropertyId)
	c.Equal(PropertyID(30074), UIA_WindowCanMinimizePropertyId)
	c.Equal(PropertyID(30075), UIA_WindowWindowVisualStatePropertyId)
	c.Equal(PropertyID(30076), UIA_WindowWindowInteractionStatePropertyId)
	c.Equal(PropertyID(30077), UIA_WindowIsModalPropertyId)
	c.Equal(PropertyID(30078), UIA_WindowIsTopmostPropertyId)
	c.Equal(PropertyID(30079), UIA_SelectionItemIsSelectedPropertyId)
	c.Equal(PropertyID(30080), UIA_SelectionItemSelectionContainerPropertyId)
	c.Equal(PropertyID(30081), UIA_TableRowHeadersPropertyId)
	c.Equal(PropertyID(30082), UIA_TableColumnHeadersPropertyId)
	c.Equal(PropertyID(30083), UIA_TableRowOrColumnMajorPropertyId)
	c.Equal(PropertyID(30084), UIA_TableItemRowHeaderItemsPropertyId)
	c.Equal(PropertyID(30085), UIA_TableItemColumnHeaderItemsPropertyId)
	c.Equal(PropertyID(30086), UIA_ToggleToggleStatePropertyId)

	// The pattern availability properties, which is how a pattern appearing or vanishing is reported.
	c.Equal(PropertyID(30028), UIA_IsExpandCollapsePatternAvailablePropertyId)
	c.Equal(PropertyID(30029), UIA_IsGridItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30030), UIA_IsGridPatternAvailablePropertyId)
	c.Equal(PropertyID(30031), UIA_IsInvokePatternAvailablePropertyId)
	c.Equal(PropertyID(30033), UIA_IsRangeValuePatternAvailablePropertyId)
	c.Equal(PropertyID(30035), UIA_IsScrollItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30036), UIA_IsSelectionItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30037), UIA_IsSelectionPatternAvailablePropertyId)
	c.Equal(PropertyID(30038), UIA_IsTablePatternAvailablePropertyId)
	c.Equal(PropertyID(30039), UIA_IsTableItemPatternAvailablePropertyId)
	c.Equal(PropertyID(30041), UIA_IsTogglePatternAvailablePropertyId)
	c.Equal(PropertyID(30043), UIA_IsValuePatternAvailablePropertyId)
	c.Equal(PropertyID(30044), UIA_IsWindowPatternAvailablePropertyId)

	// Heading levels, which are identifiers of their own rather than plain integers.
	c.Equal(HeadingLevelID(80050), HeadingLevel_None)
	c.Equal(HeadingLevelID(80051), HeadingLevel1)
	c.Equal(HeadingLevelID(80052), HeadingLevel2)
	c.Equal(HeadingLevelID(80053), HeadingLevel3)
	c.Equal(HeadingLevelID(80054), HeadingLevel4)
	c.Equal(HeadingLevelID(80055), HeadingLevel5)
	c.Equal(HeadingLevelID(80056), HeadingLevel6)
	c.Equal(HeadingLevelID(80057), HeadingLevel7)
	c.Equal(HeadingLevelID(80058), HeadingLevel8)
	c.Equal(HeadingLevelID(80059), HeadingLevel9)

	// The two bare integers UI Automation defines: the WM_GETOBJECT lParam and the runtime-id prefix.
	c.Equal(int32(-25), UiaRootObjectId)
	c.Equal(int32(3), UiaAppendRuntimeId)

	// The provider options, which are bit values rather than a sequence: get_ProviderOptions reports
	// ServerSideProvider, and reporting ClientSideProvider or UseComThreading by accident would have UI Automation
	// treat a free-threaded server-side provider as something else entirely.
	c.Equal(ProviderOptions(0x1), ProviderOptions_ClientSideProvider)
	c.Equal(ProviderOptions(0x2), ProviderOptions_ServerSideProvider)
	c.Equal(ProviderOptions(0x4), ProviderOptions_NonClientAreaProvider)
	c.Equal(ProviderOptions(0x8), ProviderOptions_OverrideProvider)
	c.Equal(ProviderOptions(0x10), ProviderOptions_ProviderOwnsSetFocus)
	c.Equal(ProviderOptions(0x20), ProviderOptions_UseComThreading)

	// The VARIANT type tags and the VARIANT_BOOL values, which are wtypes.h's rather than UI Automation's. A wrong tag
	// has a client read a property's bits as the wrong type, and a VARIANT_BOOL of one rather than every bit set is
	// neither true nor false to most COM clients.
	c.Equal(VARTYPE(0), VT_EMPTY)
	c.Equal(VARTYPE(3), VT_I4)
	c.Equal(VARTYPE(5), VT_R8)
	c.Equal(VARTYPE(8), VT_BSTR)
	c.Equal(VARTYPE(11), VT_BOOL)
	c.Equal(VARTYPE(13), VT_UNKNOWN)
	c.Equal(VARTYPE(0x2000), VT_ARRAY)
	c.Equal(int16(-1), VARIANT_TRUE)
	c.Equal(int16(0), VARIANT_FALSE)

	// The UI Automation HRESULTs a provider refuses with. A client turns each into a different error, and the whole
	// distinction between "gone", "never did that" and "not just now" is these four numbers.
	c.Equal(uint64(0x80040200), UIA_E_ELEMENTNOTENABLED)
	c.Equal(uint64(0x80040201), UIA_E_ELEMENTNOTAVAILABLE)
	c.Equal(uint64(0x80040204), UIA_E_NOTSUPPORTED)
	c.Equal(uint64(0x80131509), UIA_E_INVALIDOPERATION)
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

// TestUIAHasKeyboardFocus verifies that at most one element of a fragment claims the keyboard focus, and only while the
// window is the active one.
//
// The root is the trap: its Focused flag says the window is active rather than that the window itself is where typing
// goes, so answering the property from that flag would have the root claim the focus alongside the control that really
// has it — while reporting that it cannot be focused at all, since a root is never focusable. It claims nothing for an
// empty focus either, which is the answer GetFocus gives for one.
func TestUIAHasKeyboardFocus(t *testing.T) {
	c := check.New(t)
	tree := sampleTree()
	c.True(UIAHasKeyboardFocus(tree, tree.Node(4)), "the node the snapshot's Focus names")
	c.False(UIAHasKeyboardFocus(tree, tree.Node(1)), "the root, while something inside the window has the focus")
	c.False(UIAHasKeyboardFocus(tree, tree.Node(5)))

	// An inactive window holds no keyboard focus anywhere, however its nodes are marked.
	inactive := sampleTree()
	inactive.Node(1).Focused = false
	c.False(UIAHasKeyboardFocus(inactive, inactive.Node(4)))
	c.False(UIAHasKeyboardFocus(inactive, inactive.Node(1)))

	// With nothing inside the window focused, no element claims the keyboard: the root would have to report
	// HasKeyboardFocus true alongside IsKeyboardFocusable false, which is a pair a client cannot make sense of, and
	// IRawElementProviderFragmentRoot::GetFocus answers with a NULL element in exactly this case.
	bare := sampleTree()
	bare.Focus = 0
	bare.Node(4).Focused = false
	c.False(UIAHasKeyboardFocus(bare, bare.Node(1)))
	c.False(UIAHasKeyboardFocus(bare, bare.Node(4)), "a stale Focused flag does not decide it; the tree's Focus does")

	// A stale Focused flag on the node the tree still names is not what decides it either.
	stale := sampleTree()
	stale.Focus = 0
	c.False(UIAHasKeyboardFocus(stale, stale.Node(4)))

	c.False(UIAHasKeyboardFocus(nil, tree.Node(4)))
	c.False(UIAHasKeyboardFocus(tree, nil))
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

// TestUIAToggleState verifies that a toggle button reports its pressed state while everything else checkable reports
// its check state, and that a mixed check becomes indeterminate rather than on.
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

// TestUIAExpandCollapseState verifies that a node which cannot expand is reported as a leaf, which is a different
// answer from being collapsed.
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

// TestUIAItemStatus verifies that the item status reports a column header's sort direction, that a node which is
// working says so, and nothing otherwise.
func TestUIAItemStatus(t *testing.T) {
	c := check.New(t)
	c.Equal("", UIAItemStatus(nil))
	c.Equal("", UIAItemStatus(&accessibility.Node{Role: role.ColumnHeader}))
	// The value is free text a screen reader speaks exactly as it is given, so it is a translated phrase rather than
	// the name the SortDirection enumeration goes by.
	c.Equal("Sorted ascending", UIAItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortAscending,
	}))
	c.Equal("Sorted descending", UIAItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortDescending,
	}))

	// An indeterminate progress bar is the reason Busy is reported here at all: it fills in no number, so this is the
	// only thing it has to say for itself.
	indeterminate := &accessibility.Node{Role: role.ProgressBar, Busy: true, ReadOnly: true}
	c.Equal("Busy", UIAItemStatus(indeterminate))
	c.Equal(PatternSet(0), UIAPatterns(indeterminate), "and it has no range for a client to read instead")
	c.Equal("", UIAItemStatus(&accessibility.Node{
		Role: role.ProgressBar, HasNumber: true, Number: 3, Max: 10, ReadOnly: true,
	}), "while a determinate one says nothing, since its value carries the news")

	// Both at once is reachable — Node.Busy is public API, and nothing stops a header that is sorting from setting it —
	// and a client speaks the property as one piece of text.
	c.Equal("Sorted ascending, Busy", UIAItemStatus(&accessibility.Node{
		Role: role.ColumnHeader, Sort: accessibility.SortAscending, Busy: true,
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

	// A snapshot that reports a row past the end of its container — or before the start of it — is not trusted to
	// number it: a stale RowIndex would otherwise have a client announce "row 700 of 500", which is worse than no
	// position at all. Counting siblings is what is left, and the row is the only one of its kind published here.
	for _, rowIndex := range []int{500, 700, -1} {
		stale := newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Table, RowCount: 500, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: rowIndex},
		)
		position, size = UIAPositionInSet(stale, stale.Node(3))
		c.Equal(1, position, "row index %d", rowIndex)
		c.Equal(1, size, "row index %d", rowIndex)
	}

	// A row whose container records no count at all falls back to counting siblings too, which is also what a list
	// does.
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

	// A radio button is numbered the same way, which is the "n of m" a client announces alongside the state and the
	// whole reason the role is given the SelectionItem pattern. Its group is an ignored layout panel, so the siblings
	// counted are the ones the window sees.
	radios := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Group, Ignored: true, Children: []accessibility.NodeID{3, 4, 5}},
		&accessibility.Node{ID: 3, Role: role.RadioButton, HasCheck: true, Checked: checkenum.On},
		&accessibility.Node{ID: 4, Role: role.RadioButton, HasCheck: true},
		&accessibility.Node{ID: 5, Role: role.RadioButton, HasCheck: true},
	)
	position, size = UIAPositionInSet(radios, radios.Node(4))
	c.Equal(2, position)
	c.Equal(3, size)

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

// raiseEvent, raiseProperty, raiseStructure and raiseDisconnect build the expected results of UIADecideRaises, so that
// a test reads as a list of calls rather than as a list of struct literals.
func raiseEvent(node accessibility.NodeID, id EventID) UIARaise {
	return UIARaise{Kind: UIARaiseEvent, Node: node, Event: id}
}

func raiseProperty(node accessibility.NodeID, id PropertyID) UIARaise {
	return UIARaise{Kind: UIARaiseProperty, Node: node, Property: id}
}

func raiseStructure(node, child accessibility.NodeID, change StructureChangeType) UIARaise {
	return UIARaise{Kind: UIARaiseStructure, Node: node, Child: child, Change: change}
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

// TestUIADecideRaisesFocusCleared verifies that the focus moving to nothing is reported on the fragment root. A client
// answers a focus event by moving its cursor to the element the event names, so saying nothing at all would leave it on
// an element that has just stopped being focused; the root is the only element left to name, and a client that follows
// the event up is told by GetFocus that the fragment holds no focus.
func TestUIADecideRaisesFocusCleared(t *testing.T) {
	c := check.New(t)
	cleared := eventTree()
	cleared.Focus = 0
	cleared.Node(2).Focused = false
	c.Equal([]UIARaise{raiseEvent(1, UIA_AutomationFocusChangedEventId)},
		UIADecideRaises(eventTree(), cleared, accessibility.Diff(eventTree(), cleared)))

	// A window becoming active with nothing inside it focused points the client at the window itself for the same
	// reason.
	c.Equal([]UIARaise{raiseEvent(1, UIA_AutomationFocusChangedEventId)},
		UIADecideRaises(eventTree(), cleared, []accessibility.Event{
			{Kind: accessibility.WindowActivated, Node: 1},
		}))

	// An inactive window still says nothing, however its focus moved.
	background := eventTree()
	background.Focus = 0
	background.Node(1).Focused = false
	background.Node(2).Focused = false
	c.Nil(UIADecideRaises(eventTree(), background, []accessibility.Event{
		{Kind: accessibility.FocusChanged, Node: 0},
	}))
}

// TestUIADecideRaisesWindowOpened verifies that the first publish of a window announces that it opened, which is how a
// screen reader knows to read a dialog out as it appears.
//
// Every root raises it, not only a dialog: UIAWindow.Destroy raises Window_WindowClosed for every window, and the
// Window control type lists both events as required, so a client tracking window lifetimes must not be told that a
// window it was never told about has closed.
func TestUIADecideRaisesWindowOpened(t *testing.T) {
	c := check.New(t)
	dialog := eventTree()
	dialog.Node(1).Role = role.Dialog
	c.Equal([]UIARaise{
		raiseEvent(1, UIA_Window_WindowOpenedEventId),
		raiseEvent(2, UIA_AutomationFocusChangedEventId),
	}, UIADecideRaises(nil, dialog, accessibility.Diff(nil, dialog)))

	window := eventTree()
	c.Equal([]UIARaise{
		raiseEvent(1, UIA_Window_WindowOpenedEventId),
		raiseEvent(2, UIA_AutomationFocusChangedEventId),
	}, UIADecideRaises(nil, window, accessibility.Diff(nil, window)))

	// Every publish after the first one has a previous snapshot to compare against, and announces nothing: a window
	// that said it had opened on every redraw would be read out again each time.
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

// TestUIADecideRaisesTextProperties verifies the straightforward one-event-to-one-property translations, and the one
// event that reports two properties: the provider answers both HelpText and FullDescription from Node.Description, so
// a client that cached FullDescription and heard only about HelpText would keep a stale value forever.
func TestUIADecideRaisesTextProperties(t *testing.T) {
	c := check.New(t)
	cur := eventTree()
	c.Equal([]UIARaise{
		raiseProperty(2, UIA_NamePropertyId),
		raiseProperty(2, UIA_HelpTextPropertyId),
		raiseProperty(2, UIA_FullDescriptionPropertyId),
		raiseProperty(3, UIA_ItemStatusPropertyId),
	}, UIADecideRaises(eventTree(), cur, []accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 2, Old: "a", New: "b"},
		{Kind: accessibility.DescriptionChanged, Node: 2},
		{Kind: accessibility.SortChanged, Node: 3},
	}))
}

// TestUIADecideRaisesAttributes verifies that an attributes change reports every property the provider derives from the
// fields that event covers. The event does not say which of them changed — a watermark, a level, a row index, an
// orientation or one of the three relations — so each of the properties GetPropertyValue answers from them is reported
// once, in a fixed order.
func TestUIADecideRaisesAttributes(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{
		raiseProperty(2, UIA_HelpTextPropertyId),
		raiseProperty(2, UIA_LevelPropertyId),
		raiseProperty(2, UIA_HeadingLevelPropertyId),
		raiseProperty(2, UIA_PositionInSetPropertyId),
		raiseProperty(2, UIA_SizeOfSetPropertyId),
		raiseProperty(2, UIA_OrientationPropertyId),
		raiseProperty(2, UIA_LabeledByPropertyId),
		raiseProperty(2, UIA_DescribedByPropertyId),
		raiseProperty(2, UIA_ControllerForPropertyId),
	}, UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: 2},
	}))

	// An ignored node has no provider, so nothing is raised for it at all.
	c.Nil(UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.AttributesChanged, Node: 9},
	}))

	// A description change alongside it reports HelpText once: the two events ask for the same property, and a client
	// hearing about it twice would read the same value twice.
	c.Equal([]UIARaise{
		raiseProperty(2, UIA_HelpTextPropertyId),
		raiseProperty(2, UIA_FullDescriptionPropertyId),
		raiseProperty(2, UIA_LevelPropertyId),
		raiseProperty(2, UIA_HeadingLevelPropertyId),
		raiseProperty(2, UIA_PositionInSetPropertyId),
		raiseProperty(2, UIA_SizeOfSetPropertyId),
		raiseProperty(2, UIA_OrientationPropertyId),
		raiseProperty(2, UIA_LabeledByPropertyId),
		raiseProperty(2, UIA_DescribedByPropertyId),
		raiseProperty(2, UIA_ControllerForPropertyId),
	}, UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.DescriptionChanged, Node: 2},
		{Kind: accessibility.AttributesChanged, Node: 2},
	}))
}

// TestUIADecideRaisesHeadingLevel verifies that a heading whose depth changed reports HeadingLevel. Node.Level answers
// two properties — Level and, through UIAHeadingLevel, HeadingLevel — and a client reads a heading's depth from the
// second of them, so a change that reported only the first would leave it announcing the wrong depth.
func TestUIADecideRaisesHeadingLevel(t *testing.T) {
	c := check.New(t)
	headings := func(level int) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Heading, Name: "Section", Level: level},
		)
	}
	old, cur := headings(2), headings(3)
	c.NotEqual(UIAHeadingLevel(old.Node(2)), UIAHeadingLevel(cur.Node(2)), "the two snapshots must differ")
	c.Equal([]UIARaise{
		raiseProperty(2, UIA_HelpTextPropertyId),
		raiseProperty(2, UIA_LevelPropertyId),
		raiseProperty(2, UIA_HeadingLevelPropertyId),
		raiseProperty(2, UIA_PositionInSetPropertyId),
		raiseProperty(2, UIA_SizeOfSetPropertyId),
		raiseProperty(2, UIA_OrientationPropertyId),
		raiseProperty(2, UIA_LabeledByPropertyId),
		raiseProperty(2, UIA_DescribedByPropertyId),
		raiseProperty(2, UIA_ControllerForPropertyId),
	}, UIADecideRaises(old, cur, []accessibility.Event{{Kind: accessibility.AttributesChanged, Node: 2}}))
}

// TestUIADecideRaisesLabelContent verifies that a change to one node's LabeledBy relation reports the content-view
// change it makes to the label at the other end of it. A label that names another element is left out of the content
// view, so the label's own IsContentElement moves when something starts or stops naming it — and the event names the
// node whose attributes changed rather than the label, so nothing else would report it.
func TestUIADecideRaisesLabelContent(t *testing.T) {
	c := check.New(t)

	// Nodes 3 and 4 are labels; nodes 2 and 5 are fields that may name them.
	labeled := func(first, second []accessibility.NodeID) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{
				ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2, 3, 4, 5},
			},
			&accessibility.Node{ID: 2, Role: role.TextField, LabeledBy: first},
			&accessibility.Node{ID: 3, Role: role.Label, Name: "Name"},
			&accessibility.Node{ID: 4, Role: role.Label, Name: "Other"},
			&accessibility.Node{ID: 5, Role: role.TextField, LabeledBy: second},
		)
	}
	// The attribute properties themselves are pinned by TestUIADecideRaisesAttributes; what matters here is what
	// follows them.
	expected := func(extra ...UIARaise) []UIARaise {
		raises := make([]UIARaise, 0, len(uiaAttributeProperties)+len(extra))
		for _, propertyID := range uiaAttributeProperties {
			raises = append(raises, raiseProperty(2, propertyID))
		}
		return append(raises, extra...)
	}
	for i, one := range []struct {
		old      *accessibility.Tree
		cur      *accessibility.Tree
		expected []UIARaise
		name     string
	}{
		{
			name:     "a label that has just been given something to name leaves the content view",
			old:      labeled(nil, nil),
			cur:      labeled([]accessibility.NodeID{3}, nil),
			expected: expected(raiseProperty(3, UIA_IsContentElementPropertyId)),
		},
		{
			name:     "a label nothing names any more rejoins it",
			old:      labeled([]accessibility.NodeID{3}, nil),
			cur:      labeled(nil, nil),
			expected: expected(raiseProperty(3, UIA_IsContentElementPropertyId)),
		},
		{
			name: "a field that changed which label names it moves both of them",
			old:  labeled([]accessibility.NodeID{3}, nil),
			cur:  labeled([]accessibility.NodeID{4}, nil),
			expected: expected(raiseProperty(3, UIA_IsContentElementPropertyId),
				raiseProperty(4, UIA_IsContentElementPropertyId)),
		},
		{
			name:     "a label another field still names has not moved, so nothing is said about it",
			old:      labeled([]accessibility.NodeID{3}, []accessibility.NodeID{3}),
			cur:      labeled(nil, []accessibility.NodeID{3}),
			expected: expected(),
		},
		{
			name:     "an attributes change that leaves the relation alone reports no label at all",
			old:      labeled([]accessibility.NodeID{3}, nil),
			cur:      labeled([]accessibility.NodeID{3}, nil),
			expected: expected(),
		},
	} {
		c.Equal(one.expected, UIADecideRaises(one.old, one.cur, []accessibility.Event{
			{Kind: accessibility.AttributesChanged, Node: 2},
		}), "case %d (%s)", i, one.name)
	}
}

// TestUIADecideRaisesPatternAvailability verifies that a change which takes a state-gated pattern away, or grants one,
// is reported through that pattern's availability property — and that the pattern's own properties are reported only
// while the current snapshot still hands the pattern out.
//
// UIAPatterns gates ExpandCollapse on Expandable, RangeValue on HasNumber, a menu item's Toggle on HasCheck and a
// cell's Value on there being a value, so a snapshot that has just lost one of those states no longer supports the
// pattern whose property would carry the news. Raising it anyway is what uia_constants.go says must never happen;
// saying nothing at all would leave a client announcing rows as expanded forever after they had become leaves. The
// availability property is the one thing that may be raised on an element without the pattern, so that is what goes
// out, ahead of anything else about the element. Both directions are checked.
func TestUIADecideRaisesPatternAvailability(t *testing.T) {
	c := check.New(t)
	rows := func(expandable bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Tree, RowCount: 1, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Expandable: expandable, Expanded: expandable},
		)
	}
	scrollableRows := func(expandable, scrollable bool) *accessibility.Tree {
		tree := rows(expandable)
		if scrollable {
			tree.Nodes[3].Actions = tree.Nodes[3].Actions.With(accessibility.ScrollIntoView)
		}
		return tree
	}
	cells := func(value string) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Table, RowCount: 1, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Children: []accessibility.NodeID{4}},
			&accessibility.Node{ID: 4, Role: role.Cell, RowIndex: 0, ColumnIndex: 0, Value: value},
		)
	}
	sliders := func(hasNumber bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Slider, HasNumber: hasNumber, Number: 5, Max: 10, Step: 1},
		)
	}
	menuItems := func(hasCheck bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Menu, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.MenuItem, Name: "Wrap", HasCheck: hasCheck},
		)
	}
	stateChanged := func(id accessibility.NodeID, state accessibility.State) accessibility.Event {
		return accessibility.Event{Kind: accessibility.StateChanged, Node: id, State: state}
	}
	for i, one := range []struct {
		old      *accessibility.Tree
		cur      *accessibility.Tree
		expected []UIARaise
		event    accessibility.Event
		name     string
	}{
		{
			name:     "a row that stopped being expandable",
			old:      rows(true),
			cur:      rows(false),
			event:    stateChanged(3, accessibility.StateExpandable),
			expected: []UIARaise{raiseProperty(3, UIA_IsExpandCollapsePatternAvailablePropertyId)},
		},
		{
			name:  "a row that became expandable",
			old:   rows(false),
			cur:   rows(true),
			event: stateChanged(3, accessibility.StateExpandable),
			expected: []UIARaise{
				raiseProperty(3, UIA_IsExpandCollapsePatternAvailablePropertyId),
				raiseProperty(3, UIA_ExpandCollapseExpandCollapseStatePropertyId),
			},
		},
		{
			// Every pattern the node gained or lost is reported, in the order the patterns are defined in, and all of
			// them before whatever else the event asks for.
			name:  "a row that lost one pattern and gained another",
			old:   scrollableRows(true, false),
			cur:   scrollableRows(false, true),
			event: stateChanged(3, accessibility.StateExpandable),
			expected: []UIARaise{
				raiseProperty(3, UIA_IsExpandCollapsePatternAvailablePropertyId),
				raiseProperty(3, UIA_IsScrollItemPatternAvailablePropertyId),
			},
		},
		{
			name:     "a cell whose value became empty",
			old:      cells("Checked"),
			cur:      cells(""),
			event:    accessibility.Event{Kind: accessibility.ValueChanged, Node: 4},
			expected: []UIARaise{raiseProperty(4, UIA_IsValuePatternAvailablePropertyId)},
		},
		{
			name:  "a cell that gained a value",
			old:   cells(""),
			cur:   cells("Checked"),
			event: accessibility.Event{Kind: accessibility.ValueChanged, Node: 4},
			expected: []UIARaise{
				raiseProperty(4, UIA_IsValuePatternAvailablePropertyId),
				raiseProperty(4, UIA_ValueValuePropertyId),
			},
		},
		{
			name:     "a slider that stopped reporting a number",
			old:      sliders(true),
			cur:      sliders(false),
			event:    accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
			expected: []UIARaise{raiseProperty(2, UIA_IsRangeValuePatternAvailablePropertyId)},
		},
		{
			name:  "a slider that started reporting one",
			old:   sliders(false),
			cur:   sliders(true),
			event: accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
			expected: []UIARaise{
				raiseProperty(2, UIA_IsRangeValuePatternAvailablePropertyId),
				raiseProperty(2, UIA_RangeValueValuePropertyId),
			},
		},
		{
			name:     "a menu item that stopped being checkable",
			old:      menuItems(true),
			cur:      menuItems(false),
			event:    stateChanged(3, accessibility.StateChecked),
			expected: []UIARaise{raiseProperty(3, UIA_IsTogglePatternAvailablePropertyId)},
		},
		{
			name:  "a menu item that became checkable",
			old:   menuItems(false),
			cur:   menuItems(true),
			event: stateChanged(3, accessibility.StateChecked),
			expected: []UIARaise{
				raiseProperty(3, UIA_IsTogglePatternAvailablePropertyId),
				raiseProperty(3, UIA_ToggleToggleStatePropertyId),
			},
		},
		{
			name:  "a node that has never had the pattern still reports nothing",
			old:   sliders(false),
			cur:   sliders(false),
			event: accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
		},
		{
			// An attributes change is the only event that reports a change to the action set, and ScrollItem is gated
			// on the ScrollIntoView action alone, so this is the one place the pattern's arrival can be reported.
			name:  "a row that became scrollable",
			old:   scrollableRows(false, false),
			cur:   scrollableRows(false, true),
			event: accessibility.Event{Kind: accessibility.AttributesChanged, Node: 3},
			expected: []UIARaise{
				raiseProperty(3, UIA_IsScrollItemPatternAvailablePropertyId),
				raiseProperty(3, UIA_HelpTextPropertyId),
				raiseProperty(3, UIA_LevelPropertyId),
				raiseProperty(3, UIA_HeadingLevelPropertyId),
				raiseProperty(3, UIA_PositionInSetPropertyId),
				raiseProperty(3, UIA_SizeOfSetPropertyId),
				raiseProperty(3, UIA_OrientationPropertyId),
				raiseProperty(3, UIA_LabeledByPropertyId),
				raiseProperty(3, UIA_DescribedByPropertyId),
				raiseProperty(3, UIA_ControllerForPropertyId),
			},
		},
		{
			// A node only one of the snapshots holds has nothing to compare: its arrival is structural, and a client
			// that has never seen it has nothing cached about it to correct.
			name:  "a node the previous snapshot did not hold",
			old:   newTestTree(1, 0, &accessibility.Node{ID: 1, Role: role.Window, Name: "Window"}),
			cur:   sliders(true),
			event: accessibility.Event{Kind: accessibility.NumberChanged, Node: 2},
			expected: []UIARaise{
				raiseProperty(2, UIA_RangeValueValuePropertyId),
			},
		},
	} {
		raises := UIADecideRaises(one.old, one.cur, []accessibility.Event{one.event})
		if one.expected == nil {
			c.Nil(raises, "case %d (%s)", i, one.name)
			continue
		}
		c.Equal(one.expected, raises, "case %d (%s)", i, one.name)
	}
}

// TestUIAReportsProperty verifies what a raised property change is allowed to carry: a pattern's property only from a
// snapshot in which the element hands that pattern out, and — for the Window pattern — only on the fragment root, which
// is the only element the provider hands IWindowProvider to. UIAProvider.raisedPropertyValue is what consults this, and
// it is the only guard there is: a pattern property has no GetPropertyValue answer to agree with, so nothing else would
// catch a value invented for an element that does not implement the pattern.
func TestUIAReportsProperty(t *testing.T) {
	c := check.New(t)

	// The case that made this necessary. A spin button that becomes a password field stops reporting a number, which
	// takes the RangeValue pattern with it, and Diff still reports the number as changed: the raise goes out, and the
	// new snapshot must hand a client nothing rather than a value for a pattern the element no longer implements. The
	// protected node keeps its Number so that a report worked out from the field rather than from the pattern would
	// show up here as a value instead of as nothing; a real snapshot of a protected field leaves it at zero, which
	// would reach a client as the field's value having become "0".
	spinButtons := func(protected bool) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Children: []accessibility.NodeID{2}},
			&accessibility.Node{
				ID: 2, Role: role.SpinButton, Name: "PIN", HasNumber: !protected, Number: 4, Max: 9,
				Protected: protected,
			},
		)
	}
	open := spinButtons(false)
	hidden := spinButtons(true)
	c.Equal([]UIARaise{raiseProperty(2, UIA_IsRangeValuePatternAvailablePropertyId)},
		UIADecideRaises(open, hidden, []accessibility.Event{{Kind: accessibility.NumberChanged, Node: 2}}),
		"the loss is still reported, since a client holding the old value has to be told it is gone, but through the"+
			" one property that may be raised on an element without the pattern")
	c.True(UIAReportsProperty(open, open.Node(2), UIA_RangeValueValuePropertyId))
	c.False(UIAReportsProperty(hidden, hidden.Node(2), UIA_RangeValueValuePropertyId),
		"while the snapshot that lost the pattern has nothing to report for its property")
	c.True(UIAReportsProperty(hidden, hidden.Node(2), UIA_ValueValuePropertyId),
		"the pattern it kept still answers, with the empty string every protected node gives")
	c.True(UIAReportsProperty(hidden, hidden.Node(2), UIA_NamePropertyId),
		"and a property no pattern owns is answered by every element")
	c.True(UIAReportsProperty(hidden, hidden.Node(2), UIA_IsRangeValuePatternAvailablePropertyId),
		"as is a pattern's availability, which is answered precisely by the element that no longer has the pattern")

	// Modality is the Window pattern's, and the fragment root is the only element that hands that pattern out: a nested
	// node with a window-like role is a dialog-shaped panel that the provider refuses IWindowProvider, so it must be
	// refused a modality to report through it as well. Nothing raises one on such a panel either — the decider asks the
	// same question of the same helper — and both halves are pinned here, since the two used to be independent and only
	// the raising half recorded the rule.
	dialogs := newTestTree(1, 0,
		&accessibility.Node{ID: 1, Role: role.Window, Name: "Window", Modal: true, Children: []accessibility.NodeID{2}},
		&accessibility.Node{ID: 2, Role: role.Dialog, Name: "Sheet", Modal: true},
	)
	c.True(UIAReportsProperty(dialogs, dialogs.Node(1), UIA_WindowIsModalPropertyId),
		"the fragment root is a window of its own")
	c.False(UIAReportsProperty(dialogs, dialogs.Node(2), UIA_WindowIsModalPropertyId),
		"a nested dialog-shaped panel is not, however modal the snapshot says it is")
	c.False(UIAReportsProperty(nil, dialogs.Node(1), UIA_WindowIsModalPropertyId),
		"and with no tree to ask, nothing can be said to be the root")
	c.Equal([]UIARaise{raiseProperty(1, UIA_WindowIsModalPropertyId)},
		UIADecideRaises(dialogs, dialogs, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 1, State: accessibility.StateModal},
		}), "so the root reports a change to it")
	c.Nil(UIADecideRaises(dialogs, dialogs, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 2, State: accessibility.StateModal},
	}), "and the panel reports nothing")

	// Which pattern owns which property. A property answered through a pattern interface is the pattern's to report and
	// nobody else's; everything a client reads through GetPropertyValue is answered by every element.
	for i, one := range []struct {
		property PropertyID
		expected PatternSet
	}{
		{property: UIA_ValueValuePropertyId, expected: PatternValue},
		{property: UIA_ValueIsReadOnlyPropertyId, expected: PatternValue},
		{property: UIA_RangeValueValuePropertyId, expected: PatternRangeValue},
		{property: UIA_RangeValueIsReadOnlyPropertyId, expected: PatternRangeValue},
		{property: UIA_ToggleToggleStatePropertyId, expected: PatternToggle},
		{property: UIA_ExpandCollapseExpandCollapseStatePropertyId, expected: PatternExpandCollapse},
		{property: UIA_SelectionItemIsSelectedPropertyId, expected: PatternSelectionItem},
		{property: UIA_SelectionCanSelectMultiplePropertyId, expected: PatternSelection},
		{property: UIA_WindowIsModalPropertyId, expected: PatternWindow},
		{property: UIA_NamePropertyId},
		{property: UIA_ItemStatusPropertyId},
		{property: UIA_BoundingRectanglePropertyId},
		{property: UIA_IsPasswordPropertyId},
	} {
		c.Equal(one.expected, UIAPropertyPattern(one.property), "case %d (property %d)", i, one.property)
	}

	// Every property an attributes change reports is element-wide, so none of them may be gated: a node with nothing to
	// say for one answers it empty on both sides, which a client reads as no change, and gating them would instead have
	// a node that never had the pattern drop a property it really does answer.
	for _, propertyID := range uiaAttributeProperties {
		c.Equal(PatternSet(0), UIAPropertyPattern(propertyID), "property %d", propertyID)
	}
}

// TestUIAPatternAvailableProperty verifies that every pattern this package implements has an availability property,
// that the two directions of the mapping agree, and that no such property is owned by the pattern it describes — an
// element without the pattern is exactly the element that has to answer one, so gating it would silence the only thing
// that can tell a client the pattern has gone.
func TestUIAPatternAvailableProperty(t *testing.T) {
	c := check.New(t)
	seen := make(map[PropertyID]bool, len(uiaPatternInfos))
	for _, info := range uiaPatternInfos {
		c.NotEqual(PropertyID(0), info.available, "pattern %s has no availability property", info.name)
		c.False(seen[info.available], "pattern %s shares an availability property", info.name)
		seen[info.available] = true
		c.Equal(info.available, UIAPatternAvailableProperty(info.pattern), "pattern %s", info.name)
		c.Equal(info.pattern, UIAAvailabilityPattern(info.available), "pattern %s", info.name)
		c.Equal(PatternSet(0), UIAPropertyPattern(info.available), "pattern %s", info.name)
		c.True(UIAReportsProperty(nil, nil, info.available), "pattern %s", info.name)
	}
	c.Equal(PropertyID(0), UIAPatternAvailableProperty(0))
	c.Equal(PropertyID(0), UIAPatternAvailableProperty(PatternValue|PatternRangeValue),
		"the availability of two patterns at once is not a property")
	c.Equal(PatternSet(0), UIAAvailabilityPattern(UIA_NamePropertyId))
	c.Equal(PatternSet(0), UIAAvailabilityPattern(UIA_ValueValuePropertyId))
}

// TestUIADecideRaisesValue verifies that a value change becomes whichever value property the element actually has. A
// text field reports its value through the Value pattern and a slider through RangeValue, so raising the Value
// pattern's property on a slider would be telling a client about a property the element does not support; a label has
// neither, so its value change is dropped.
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

	// A cell whose one widget changed state reports its value, which is the Value pattern's property: a screen reader
	// reading across a row asks the cell, so a change to the widget has to arrive as a change to the cell.
	cells := func(value string) *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
			&accessibility.Node{ID: 2, Role: role.Table, RowCount: 1, Children: []accessibility.NodeID{3}},
			&accessibility.Node{ID: 3, Role: role.Row, RowIndex: 0, Children: []accessibility.NodeID{4}},
			&accessibility.Node{ID: 4, Role: role.Cell, RowIndex: 0, ColumnIndex: 0, Value: value},
		)
	}
	c.Equal([]UIARaise{raiseProperty(4, UIA_ValueValuePropertyId)}, UIADecideRaises(cells("off"), cells("on"),
		[]accessibility.Event{{Kind: accessibility.ValueChanged, Node: 4}}))

	// A cell whose content is its name has no value to report, so there is no property to raise.
	c.Nil(UIADecideRaises(cells(""), cells(""), []accessibility.Event{
		{Kind: accessibility.ValueChanged, Node: 4},
	}))
}

// TestUIADecideRaisesText verifies that an edit reports the new value once, even though the diff describes it as a
// value change plus the deletion and insertion that made it.
//
// Neither of UI Automation's text events is raised: both belong to the Text control pattern, which this package does
// not implement and which every element answers NULL for, so a client that responded to one by asking for ITextProvider
// would have nothing to read. A caret move reports nothing at all, since it changes no property a client can read back.
func TestUIADecideRaisesText(t *testing.T) {
	c := check.New(t)
	c.Equal([]UIARaise{raiseProperty(2, UIA_ValueValuePropertyId)},
		UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.ValueChanged, Node: 2, Old: "hello", New: "help"},
			{Kind: accessibility.TextDeleted, Node: 2, Start: 3, Length: 2, Old: "lo"},
			{Kind: accessibility.TextInserted, Node: 2, Start: 3, Length: 1, New: "p"},
			{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 4},
		}))

	// An edit with no value change of its own still reports the value, since that is where a client reads the text.
	c.Equal([]UIARaise{raiseProperty(2, UIA_ValueValuePropertyId)},
		UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
			{Kind: accessibility.TextInserted, Node: 2, Start: 5, Length: 1, New: "!"},
		}))

	// A button has no value pattern, so an edit to it reports nothing rather than an event a client cannot follow up.
	c.Nil(UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.TextInserted, Node: 10, Start: 0, Length: 1, New: "x"},
		{Kind: accessibility.TextSelectionChanged, Node: 10},
	}))
}

// TestUIADecideRaisesStates verifies the state flags that map onto a property of their own, and that a flag belonging
// to a pattern the element does not support raises nothing.
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
		// Modality belongs to the Window pattern, so a window reports it and an element with no such pattern does not.
		{state: accessibility.StateModal, node: 1, expected: []UIARaise{
			raiseProperty(1, UIA_WindowIsModalPropertyId),
		}},
		{state: accessibility.StateModal, node: 3},
		// Whether a field hides what is typed into it is answered for every element, so it is reported for every one.
		{state: accessibility.StateProtected, node: 2, expected: []UIARaise{
			raiseProperty(2, UIA_IsPasswordPropertyId),
		}},
		// CanSelectMultiple belongs to the Selection pattern, so a container reports it and an element with no such
		// pattern does not.
		{state: accessibility.StateMultiselectable, node: 4, expected: []UIARaise{
			raiseProperty(4, UIA_SelectionCanSelectMultiplePropertyId),
		}},
		{state: accessibility.StateMultiselectable, node: 2},
		// Being busy is part of the item status, which every element answers, so it is reported for every one. An
		// indeterminate progress bar is the element that needs it: it reports no number at all, so this is the only
		// thing that changes as it starts and stops working.
		{state: accessibility.StateBusy, node: 1, expected: []UIARaise{
			raiseProperty(1, UIA_ItemStatusPropertyId),
		}},
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

// TestUIADecideRaisesSelectionNested verifies that a nested row's selection is judged by the container the provider
// reports — the nearest ancestor supporting the Selection pattern — rather than by its immediate parent. A child row's
// parent is another row, which is never multiselectable, so asking the parent would report a row joining a multiple
// selection with ElementSelected, which tells the client everything else was just deselected.
func TestUIADecideRaisesSelectionNested(t *testing.T) {
	c := check.New(t)
	nested := func() *accessibility.Tree {
		return newTestTree(1, 0,
			&accessibility.Node{ID: 1, Role: role.Window, Focused: true, Children: []accessibility.NodeID{2}},
			&accessibility.Node{
				ID: 2, Role: role.Tree, Multiselectable: true, RowCount: 2,
				Children: []accessibility.NodeID{3},
			},
			&accessibility.Node{
				ID: 3, Role: role.Row, RowIndex: 0, Selectable: true, Selected: true, Expandable: true,
				Expanded: true, Children: []accessibility.NodeID{4},
			},
			&accessibility.Node{ID: 4, Role: role.Row, RowIndex: 1, Selectable: true, Selected: true},
		)
	}
	c.Equal([]UIARaise{
		raiseProperty(4, UIA_SelectionItemIsSelectedPropertyId),
		raiseEvent(4, UIA_SelectionItem_ElementAddedToSelectionEventId),
	}, UIADecideRaises(nested(), nested(), []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateSelected},
	}))

	// The same row in a table that holds one selection at a time reports becoming the selection instead.
	single := nested()
	single.Node(2).Multiselectable = false
	c.Equal([]UIARaise{
		raiseProperty(4, UIA_SelectionItemIsSelectedPropertyId),
		raiseEvent(4, UIA_SelectionItem_ElementSelectedEventId),
	}, UIADecideRaises(nested(), single, []accessibility.Event{
		{Kind: accessibility.StateChanged, Node: 4, State: accessibility.StateSelected},
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

	// Its Selected flag is not what decides it either: ISelectionItemProvider::get_IsSelected answers from the check
	// state for a radio button, so a selected-but-unchecked one must not be reported as having become the selection —
	// the client would be told something the provider then denies. The builder produces no such node today; the two
	// answers are kept in step here rather than relying on that.
	odd := eventTree()
	odd.Node(7).Checked = checkenum.Off
	odd.Node(7).Selected = true
	c.Equal([]UIARaise{raiseProperty(7, UIA_SelectionItemIsSelectedPropertyId)},
		UIADecideRaises(eventTree(), odd, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 7, State: accessibility.StateSelected},
		}))
}

// TestUIADecideRaisesBounds verifies that only the focused node and the root report a new bounding rectangle. Resizing
// a window moves everything in it, and a client that wanted every rectangle would ask for them.
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

// TestUIADecideRaisesIgnored verifies that a node the snapshot marks Ignored never appears, since it has no provider
// for an event to be raised on.
func TestUIADecideRaisesIgnored(t *testing.T) {
	c := check.New(t)
	c.Nil(UIADecideRaises(eventTree(), eventTree(), []accessibility.Event{
		{Kind: accessibility.NameChanged, Node: 9},
		{Kind: accessibility.StateChanged, Node: 9, State: accessibility.StateDisabled},
		{Kind: accessibility.BoundsChanged, Node: 9},
	}))
}

// TestUIADecideRaisesAdded verifies that a new node reports itself as added, and that it says nothing once its parent
// has already reported all of its children invalidated — which is what the diff produces for a real addition, since the
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

// TestUIADecideRaisesAddedSubtree verifies that a whole subtree arriving at once is reported as one invalidation of the
// parent it hangs off and nothing else.
//
// The diff invalidates that parent and then reports every node of the subtree as added, so looking only at a new node's
// immediate parent drops the subtree's top node — whose parent is the invalidated one — and then reports every node
// beneath it, whose parents are the new nodes themselves. A client answers the invalidation by reading the children
// again, so those are events it would only make it do the same work over.
func TestUIADecideRaisesAddedSubtree(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(1).Children = append(cur.Node(1).Children, 11)
	cur.Nodes[11] = &accessibility.Node{ID: 11, Parent: 1, Role: role.Group, Children: []accessibility.NodeID{12, 13}}
	cur.Nodes[12] = &accessibility.Node{ID: 12, Parent: 11, Role: role.Button, Name: "One"}
	cur.Nodes[13] = &accessibility.Node{ID: 13, Parent: 11, Role: role.Button, Name: "Two"}

	c.Equal([]UIARaise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(old, cur, accessibility.Diff(old, cur)))

	// A node added under an invalidated ancestor several levels up is dropped just the same, which is what the walk up
	// the chain is for.
	c.Equal([]UIARaise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(old, cur, []accessibility.Event{
			{Kind: accessibility.ChildrenChanged, Node: 1},
			{Kind: accessibility.NodeAdded, Node: 12},
		}))
}

// TestUIADecideRaisesRemoved verifies that a departed node reports its removal on the parent it left, since it no
// longer exists to raise anything itself, and that the disconnect releasing its provider happens either way.
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

// TestUIADecideRaisesRemovedSubtree verifies that removing a whole subtree reports the invalidation once on the
// surviving parent and disconnects every node that left, rather than trying to raise a removal on a parent that is
// gone too.
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

	// A removal under an ancestor the same batch invalidated higher up is dropped too. A diff never produces that on
	// its own — a real removal changes the immediate parent's list of children as well — so the events are written out
	// here to reach the walk up the chain that removals share with additions.
	gone := eventTree()
	gone.Node(4).Children = []accessibility.NodeID{5}
	delete(gone.Nodes, 6)
	c.Equal([]UIARaise{
		raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated),
		raiseDisconnect(6),
	}, UIADecideRaises(old, gone, []accessibility.Event{
		{Kind: accessibility.ChildrenChanged, Node: 1},
		{Kind: accessibility.NodeRemoved, Node: 6},
	}))
}

// TestUIADecideRaisesIgnoredFlip verifies that a node whose Ignored flag flips has the nearest unignored parent told to
// read its children again.
//
// Nothing else says it happened: an ignored node stays in the snapshot so that hit testing and coordinate clipping go
// on working, so no list of children changed, while to a client the node has just joined or left the tree entirely. It
// reaches a real window — ScrollBar.ProvideAccessibility ignores a scroll bar with nothing to scroll — and a client
// that was told nothing would keep a hierarchy that permanently disagrees with what UIANavigate answers.
func TestUIADecideRaisesIgnoredFlip(t *testing.T) {
	c := check.New(t)
	shown := eventTree()
	hidden := eventTree()
	hidden.Node(3).Ignored = true

	// The check box directly under the window leaves the tree a client sees, and then comes back.
	c.Equal([]UIARaise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(shown, hidden, accessibility.Diff(shown, hidden)))
	c.Equal([]UIARaise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(hidden, shown, accessibility.Diff(hidden, shown)))

	// Node 10 sits under an ignored group, so the invalidation lands on the window rather than on a parent with no
	// provider to raise it on.
	buried := eventTree()
	buried.Node(10).Ignored = true
	c.Equal([]UIARaise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(shown, buried, accessibility.Diff(shown, buried)))

	// Having said the parent's children are all invalid, the additions and removals under it are dropped, exactly as
	// they are for a list of children that changed.
	c.Equal([]UIARaise{raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated)},
		UIADecideRaises(shown, hidden, []accessibility.Event{
			{Kind: accessibility.StateChanged, Node: 3, State: accessibility.StateIgnored, Old: "false", New: "true"},
			{Kind: accessibility.NodeAdded, Node: 2},
		}))
}

// menuTree builds a window with a button, optionally with an open menu or a tooltip hanging off the root the way the
// root panel really holds them: both are children of the window itself rather than of whatever they belong to.
func menuTree(extra ...*accessibility.Node) *accessibility.Tree {
	nodes := []*accessibility.Node{
		{ID: 1, Role: role.Window, Name: "Window", Focused: true},
		{ID: 2, Role: role.Button, Name: "File"},
	}
	children := []accessibility.NodeID{2}
	for _, n := range extra {
		nodes = append(nodes, n)
		children = append(children, n.ID)
	}
	nodes[0].Children = children
	return newTestTree(1, 2, nodes...)
}

// TestUIADecideRaisesMenu verifies that a menu appearing and disappearing raises the two events UI Automation has for
// exactly that, over and above whatever structure change the node asks for. They are what tell a screen reader to enter
// and leave menu mode; a structure change and a focus move say nothing of the kind.
func TestUIADecideRaisesMenu(t *testing.T) {
	c := check.New(t)
	menu := func() []*accessibility.Node {
		return []*accessibility.Node{
			{ID: 3, Role: role.Menu, Children: []accessibility.NodeID{4}},
			{ID: 4, Role: role.MenuItem, Name: "Open"},
		}
	}
	closed := menuTree()
	open := menuTree(menu()...)

	c.Equal([]UIARaise{
		raiseStructure(3, 3, StructureChangeType_ChildAdded),
		raiseEvent(3, UIA_MenuOpenedEventId),
	}, UIADecideRaises(closed, open, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 3}}))

	// The menu closing is raised on the window: the menu itself has left the tree, so there is no provider of its own
	// left for a client to be told about it through.
	c.Equal([]UIARaise{
		raiseEvent(1, UIA_MenuClosedEventId),
		raiseStructure(1, 3, StructureChangeType_ChildRemoved),
		raiseDisconnect(3),
	}, UIADecideRaises(open, closed, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 3}}))

	// The same holds for the real batch a diff produces, where the structure changes collapse into one invalidation of
	// the window but the menu events do not: a client answers an invalidation by reading the children again, which
	// tells it nothing about a menu having opened.
	c.Equal([]UIARaise{
		raiseStructure(1, 0, StructureChangeType_ChildrenInvalidated),
		raiseEvent(3, UIA_MenuOpenedEventId),
	}, UIADecideRaises(closed, open, accessibility.Diff(closed, open)))

	// The menu item inside it is not a menu, and neither is anything else that comes and goes.
	c.Equal([]UIARaise{raiseStructure(4, 4, StructureChangeType_ChildAdded)},
		UIADecideRaises(closed, open, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 4}}))
}

// TestUIADecideRaisesTooltip verifies that a tooltip appearing is announced. Nothing else says it happened: a tip takes
// no focus, names nothing and changes no property, so without the event a client never mentions it.
func TestUIADecideRaisesTooltip(t *testing.T) {
	c := check.New(t)
	without := menuTree()
	with := menuTree(&accessibility.Node{ID: 3, Role: role.Tooltip, Name: "Open a file"})

	c.Equal([]UIARaise{
		raiseStructure(3, 3, StructureChangeType_ChildAdded),
		raiseEvent(3, UIA_ToolTipOpenedEventId),
	}, UIADecideRaises(without, with, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 3}}))

	c.Equal([]UIARaise{
		raiseEvent(1, UIA_ToolTipClosedEventId),
		raiseStructure(1, 3, StructureChangeType_ChildRemoved),
		raiseDisconnect(3),
	}, UIADecideRaises(with, without, []accessibility.Event{{Kind: accessibility.NodeRemoved, Node: 3}}))

	// An ignored node has no provider, so nothing is raised for it at all.
	hidden := menuTree(&accessibility.Node{ID: 3, Role: role.Tooltip, Name: "Open a file", Ignored: true})
	c.Nil(UIADecideRaises(without, hidden, []accessibility.Event{{Kind: accessibility.NodeAdded, Node: 3}}))
}

// TestUIADecideRaisesRole verifies that a node whose role changed reports its control type as changed. A live node
// really can change role — a label becomes an image when its text is swapped for a drawable, a button becomes a toggle
// button when it is made sticky — and the control type is what a client derives the spoken kind of the element, and the
// patterns it bothers looking for, from.
//
// The localized control type is deliberately not reported alongside it: this package never answers that property, since
// UI Automation has a localized name for every control type and ours would be in English only, so it derives that one
// from the control type it has just been told about.
func TestUIADecideRaisesRole(t *testing.T) {
	c := check.New(t)
	old := eventTree()
	cur := eventTree()
	cur.Node(3).Role = role.ToggleButton

	events := accessibility.Diff(old, cur)
	c.Equal(1, len(events))
	c.Equal(accessibility.RoleChanged, events[0].Kind)
	c.Equal([]UIARaise{raiseProperty(3, UIA_ControlTypePropertyId)}, UIADecideRaises(old, cur, events))

	// An ignored node has no provider, so its role change is dropped like everything else about it.
	c.Nil(UIADecideRaises(old, cur, []accessibility.Event{
		{Kind: accessibility.RoleChanged, Node: 9, Old: "group", New: "tool-bar"},
	}))
}

// TestUIADecideRaisesWindowActivation verifies that a window becoming active points the client back at whatever inside
// it has the focus, and that a window losing it says nothing at all.
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

// TestUIARaiseStrings verifies that the diagnostic strings name each kind and carry the field that matters for it,
// since a failing provider test is read through them.
func TestUIARaiseStrings(t *testing.T) {
	c := check.New(t)
	c.Equal("event", UIARaiseEvent.String())
	c.Equal("property", UIARaiseProperty.String())
	c.Equal("structure", UIARaiseStructure.String())
	c.Equal("disconnect", UIARaiseDisconnect.String())
	c.Equal("UIARaiseKind(9)", UIARaiseKind(9).String())
	c.Equal("event{node:2,event:20005}", raiseEvent(2, UIA_AutomationFocusChangedEventId).String())
	c.Equal("property{node:3,property:30005}", raiseProperty(3, UIA_NamePropertyId).String())
	c.Equal("structure{node:4,change:1,child:6}",
		raiseStructure(4, 6, StructureChangeType_ChildRemoved).String())
	c.Equal("structure{node:4,change:2}", raiseStructure(4, 0, StructureChangeType_ChildrenInvalidated).String())
	c.Equal("disconnect{node:6}", raiseDisconnect(6).String())
}
