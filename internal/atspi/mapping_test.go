// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// expectedRoles is the AT-SPI role every one of Unison's roles maps onto, for a node with nothing else set.
var expectedRoles = map[role.Enum]Role{
	role.Auto:               RoleUnknown,
	role.None:               RoleUnknown,
	role.Window:             RoleFrame,
	role.Dialog:             RoleDialog,
	role.Group:              RolePanel,
	role.Button:             RolePushButton,
	role.ToggleButton:       RoleToggleButton,
	role.DisclosureTriangle: RoleToggleButton,
	role.CheckBox:           RoleCheckBox,
	role.RadioButton:        RoleRadioButton,
	role.Link:               RoleLink,
	role.Label:              RoleLabel,
	role.Heading:            RoleHeading,
	role.TextField:          RoleEntry,
	role.TextArea:           RoleText,
	role.SpinButton:         RoleSpinButton,
	role.ComboBox:           RoleComboBox,
	role.PopupButton:        RoleComboBox,
	role.Slider:             RoleSlider,
	role.ProgressBar:        RoleProgressBar,
	role.ScrollBar:          RoleScrollBar,
	role.ScrollArea:         RoleScrollPane,
	role.Separator:          RoleSeparator,
	role.List:               RoleListBox,
	role.ListItem:           RoleListItem,
	role.Table:              RoleTable,
	role.Tree:               RoleTreeTable,
	role.Row:                RoleTableRow,
	role.Cell:               RoleTableCell,
	role.ColumnHeader:       RoleColumnHeader,
	role.TableHeader:        RolePanel,
	role.TabList:            RolePageTabList,
	role.Tab:                RolePageTab,
	role.TabPanel:           RolePanel,
	role.MenuBar:            RoleMenuBar,
	role.Menu:               RoleMenu,
	role.MenuItem:           RoleMenuItem,
	role.Image:              RoleImage,
	role.ColorWell:          RolePushButton,
	role.Tooltip:            RoleToolTip,
	role.Document:           RoleDocumentFrame,
	role.Toolbar:            RoleToolBar,
	role.Unknown:            RoleUnknown,
}

func TestMapRoleCoversEveryRole(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(len(role.All), len(expectedRoles), "every role must have an expected AT-SPI role")
	for _, one := range role.All {
		expected, exists := expectedRoles[one]
		c.True(exists, "%s has no expected AT-SPI role", one.Key())
		mapped := MapRole(&accessibility.Node{Role: one})
		c.Equal(expected, mapped, "%s mapped to %d", one.Key(), mapped)
		c.NotEqual(RoleInvalid, mapped, "%s must not map to an invalid role", one.Key())
		c.NotEqual("", RoleName(mapped), "%s maps to a role with no name", one.Key())
	}
}

func TestMapRoleDependsOnMoreThanTheRole(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(RoleGrouping, MapRole(&accessibility.Node{Role: role.Group, Name: "Options"}))
	c.Equal(RoleGrouping, MapRole(&accessibility.Node{Role: role.TabPanel, Name: "General"}))
	c.Equal(RolePasswordText, MapRole(&accessibility.Node{Role: role.TextField, Protected: true}))
	c.Equal(RoleCheckMenuItem, MapRole(&accessibility.Node{Role: role.MenuItem, HasCheck: true}))
}

func TestRoleName(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal("push button", RoleName(RolePushButton))
	c.Equal("table row", RoleName(RoleTableRow))
	c.Equal("application", RoleName(RoleApplication))
	c.Equal("password text", RoleName(RolePasswordText))
	c.Equal("unknown", RoleName(Role(12345)))
}

func TestInterfaces(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name     string
		expected []string
		node     accessibility.Node
	}{
		{
			name:     "a label has nothing but the basics",
			node:     accessibility.Node{Role: role.Label},
			expected: []string{InterfaceAccessible, InterfaceComponent},
		},
		{
			name: "a button can be pressed",
			node: accessibility.Node{
				Role:    role.Button,
				Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
			},
			expected: []string{InterfaceAccessible, InterfaceAction, InterfaceComponent},
		},
		{
			name: "a slider has a value and can be nudged",
			node: accessibility.Node{
				Role:      role.Slider,
				HasNumber: true,
				Actions:   accessibility.ActionSet(0).With(accessibility.Increment, accessibility.SetValue),
			},
			expected: []string{InterfaceAccessible, InterfaceAction, InterfaceComponent, InterfaceValue},
		},
		{
			name:     "a progress bar has a value it cannot change",
			node:     accessibility.Node{Role: role.ProgressBar, HasNumber: true},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceValue},
		},
		{
			name:     "a list holds a selection",
			node:     accessibility.Node{Role: role.List},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceSelection},
		},
		{
			name:     "a tree holds a selection and is laid out as a grid",
			node:     accessibility.Node{Role: role.Tree},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceSelection, InterfaceTable},
		},
		{
			name:     "a table is a grid as well as a selection",
			node:     accessibility.Node{Role: role.Table, RowCount: 3, ColumnCount: 2},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceSelection, InterfaceTable},
		},
		{
			name:     "a cell says where in the grid it sits",
			node:     accessibility.Node{Role: role.Cell, RowIndex: 1, ColumnIndex: 0},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceTableCell},
		},
		{
			name:     "a row is the container the cells sit in rather than one of them",
			node:     accessibility.Node{Role: role.Row, RowIndex: 1},
			expected: []string{InterfaceAccessible, InterfaceComponent},
		},
		{
			name:     "a list is not a grid",
			node:     accessibility.Node{Role: role.List, RowCount: 3},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceSelection},
		},
		{
			name: "focus alone is not an AT-SPI action",
			node: accessibility.Node{
				Role:    role.TextField,
				Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection),
			},
			expected: []string{InterfaceAccessible, InterfaceComponent},
		},
		{
			name: "a field with content holds navigable text",
			node: accessibility.Node{
				Role: role.TextField,
				Text: &accessibility.TextInfo{Text: "content"},
			},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceText},
		},
		{
			name:     "a password field has no text to navigate",
			node:     accessibility.Node{Role: role.TextField, Protected: true},
			expected: []string{InterfaceAccessible, InterfaceComponent},
		},
		{
			name: "a spin button has both text and a number, in that order",
			node: accessibility.Node{
				Role:      role.SpinButton,
				HasNumber: true,
				Text:      &accessibility.TextInfo{Text: "42"},
			},
			expected: []string{InterfaceAccessible, InterfaceComponent, InterfaceText, InterfaceValue},
		},
	} {
		c.Equal(one.expected, Interfaces(&one.node), one.name)
	}
}

func TestStateSetPacking(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	var set StateSet
	c.Equal([]uint32{0, 0}, set.Words())
	set = set.With(StateFocusable, StateIndeterminate, StateReadOnly)
	// FOCUSABLE is 11, so it belongs in the first word; INDETERMINATE is 32 and READ_ONLY is 43, so they belong in the
	// second one, at bits 0 and 11.
	c.Equal([]uint32{1 << 11, 1<<0 | 1<<11}, set.Words())
	c.True(set.Has(StateFocusable))
	c.True(set.Has(StateIndeterminate))
	c.True(set.Has(StateReadOnly))
	c.False(set.Has(StateFocused))
	c.False(set.Has(StateBit(64)), "a state outside the bitset is never present")
	c.Equal(set, set.With(StateBit(64)), "a state outside the bitset changes nothing")
}

func TestStatesOfEveryRole(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range role.All {
		set := States(&accessibility.Node{Role: one}, true, false)
		c.True(set.Has(StateVisible), "%s must be visible", one.Key())
		c.True(set.Has(StateShowing), "%s must be showing", one.Key())
		c.True(set.Has(StateEnabled), "%s must be enabled", one.Key())
		c.True(set.Has(StateSensitive), "%s must be sensitive", one.Key())
		c.False(set.Has(StateFocused), "%s is not focused", one.Key())
	}
}

func TestStatesOfTheCommonFlags(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	set := States(&accessibility.Node{
		Role:      role.Button,
		Disabled:  true,
		Offscreen: true,
		Busy:      true,
		Invalid:   true,
		ReadOnly:  true,
	}, true, false)
	c.False(set.Has(StateShowing), "an offscreen node is not showing")
	c.True(set.Has(StateVisible), "an offscreen node is still visible")
	c.False(set.Has(StateEnabled))
	c.False(set.Has(StateSensitive))
	c.True(set.Has(StateBusy))
	c.True(set.Has(StateInvalidEntry))
	c.True(set.Has(StateReadOnly))
}

func TestStatesOfFocus(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	focused := &accessibility.Node{Role: role.Button, Focusable: true, Focused: true}
	set := States(focused, true, false)
	c.True(set.Has(StateFocusable))
	c.True(set.Has(StateFocused))
	c.False(States(focused, false, false).Has(StateFocused), "only the active window has a focused object")
	c.True(States(focused, false, false).Has(StateFocusable))
}

func TestStatesOfAWindow(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	set := States(&accessibility.Node{Role: role.Window, Focused: true, Modal: true, Resizable: true}, true, true)
	c.True(set.Has(StateActive))
	c.True(set.Has(StateResizable))
	c.True(set.Has(StateModal))
	c.False(set.Has(StateFocused), "a window reports being active rather than focused")
	c.False(States(&accessibility.Node{Role: role.Dialog}, false, true).Has(StateActive))
	// A window created with NotResizableWindowOption, and a dialog, which is most often one of those.
	c.False(States(&accessibility.Node{Role: role.Window, Focused: true}, true, true).Has(StateResizable),
		"a fixed size window must not claim it can be resized")
	c.False(States(&accessibility.Node{Role: role.Dialog}, true, true).Has(StateResizable))
	c.True(States(&accessibility.Node{Role: role.Dialog, Resizable: true}, true, true).Has(StateResizable))
}

// TestStatesOfADialogWithinAWindow covers the node that is not the window it is in although it says it is one: a panel
// laid out as a dialog inside another window reports role.Dialog, and only the tree knows which node is really the
// window. Reading the role instead would have such a panel claim to be the active window while the focus it holds went
// unreported, which is the one state an assistive technology follows above all others.
func TestStatesOfADialogWithinAWindow(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	nested := &accessibility.Node{Role: role.Dialog, Focusable: true, Focused: true, Resizable: true}
	set := States(nested, true, false)
	c.False(set.Has(StateActive), "only the window itself is the active one")
	c.True(set.Has(StateFocused), "and everything else that is focused within it holds the focus")
	c.True(set.Has(StateResizable))
	c.Equal(LayerWidget, layerFor(false), "a dialog inside a window is in the widget layer")
	c.Equal(LayerWindow, layerFor(true))
}

func TestStatesOfCheckables(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	on := States(&accessibility.Node{Role: role.CheckBox, HasCheck: true, Checked: checkenum.On}, true, false)
	c.True(on.Has(StateCheckable))
	c.True(on.Has(StateChecked))
	c.False(on.Has(StateIndeterminate))
	mixed := States(&accessibility.Node{Role: role.CheckBox, HasCheck: true, Checked: checkenum.Mixed}, true, false)
	c.True(mixed.Has(StateIndeterminate))
	c.False(mixed.Has(StateChecked))
	off := States(&accessibility.Node{Role: role.CheckBox, HasCheck: true, Checked: checkenum.Off}, true, false)
	c.True(off.Has(StateCheckable))
	c.False(off.Has(StateChecked))
	c.False(off.Has(StateIndeterminate))
	pressed := States(&accessibility.Node{Role: role.ToggleButton, Pressed: true}, true, false)
	c.True(pressed.Has(StateCheckable))
	c.True(pressed.Has(StateChecked))
	c.True(pressed.Has(StatePressed))
	up := States(&accessibility.Node{Role: role.ToggleButton}, true, false)
	c.True(up.Has(StateCheckable))
	c.False(up.Has(StateChecked))
	c.False(up.Has(StatePressed))
}

func TestStatesOfTextControls(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	field := States(&accessibility.Node{Role: role.TextField, Text: &accessibility.TextInfo{}}, true, false)
	c.True(field.Has(StateSingleLine))
	c.True(field.Has(StateSelectableText))
	c.True(field.Has(StateEditable))
	c.False(field.Has(StateMultiLine))
	locked := States(&accessibility.Node{Role: role.TextField, ReadOnly: true, Text: &accessibility.TextInfo{}}, true,
		false)
	c.False(locked.Has(StateEditable))
	c.True(locked.Has(StateReadOnly))
	area := States(&accessibility.Node{Role: role.TextArea, Text: &accessibility.TextInfo{}}, true, false)
	c.True(area.Has(StateMultiLine))
	c.True(area.Has(StateEditable))
	c.False(area.Has(StateSingleLine))
	document := States(&accessibility.Node{Role: role.Document, Text: &accessibility.TextInfo{}}, true, false)
	c.True(document.Has(StateMultiLine))
	c.True(document.Has(StateSelectableText))
	c.False(document.Has(StateEditable), "a document is never editable")
	combo := States(&accessibility.Node{Role: role.ComboBox, Text: &accessibility.TextInfo{}}, true, false)
	c.True(combo.Has(StateHasPopup))
	c.True(combo.Has(StateEditable))
	c.True(States(&accessibility.Node{Role: role.PopupButton}, true, false).Has(StateHasPopup))
}

// TestStatesOfTextControlsWithNoText covers the state set of a text control whose content this process will not hand
// over, which is what a password field is: [accessibility.Node] leaves the Text of a protected node nil, so
// [Interfaces] gives the object no org.a11y.atspi.Text. Claiming the text states anyway would have an assistive
// technology ask an object for text through an interface it does not implement.
func TestStatesOfTextControlsWithNoText(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	password := &accessibility.Node{Role: role.TextField, Protected: true}
	c.Equal(RolePasswordText, MapRole(password))
	c.False(slices.Contains(Interfaces(password), InterfaceText), "a protected field hands over no text")
	set := States(password, true, false)
	for _, one := range []struct {
		name  string
		state StateBit
	}{
		{name: "single line", state: StateSingleLine},
		{name: "selectable text", state: StateSelectableText},
		{name: "editable", state: StateEditable},
	} {
		c.False(set.Has(one.state), "a node with no text interface must not claim %s", one.name)
	}
	c.True(set.Has(StateVisible), "everything else about it is still reported")
	spin := States(&accessibility.Node{Role: role.SpinButton, Protected: true}, true, false)
	c.False(spin.Has(StateSelectableText))
	c.False(spin.Has(StateEditable))
}

func TestStatesOfCollections(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	list := States(&accessibility.Node{Role: role.List, Multiselectable: true}, true, false)
	c.True(list.Has(StateMultiselectable))
	item := States(&accessibility.Node{Role: role.ListItem, Selectable: true, Selected: true}, true, false)
	c.True(item.Has(StateSelectable))
	c.True(item.Has(StateSelected))
	row := States(&accessibility.Node{Role: role.Row, Expandable: true}, true, false)
	c.True(row.Has(StateExpandable))
	c.True(row.Has(StateCollapsed))
	c.False(row.Has(StateExpanded))
	open := States(&accessibility.Node{Role: role.Row, Expandable: true, Expanded: true}, true, false)
	c.True(open.Has(StateExpanded))
	c.False(open.Has(StateCollapsed))
	small := States(&accessibility.Node{Role: role.Table, RowCount: manageDescendantsRowThreshold}, true, false)
	c.False(small.Has(StateManagesDescendants), "a table this size is walked child by child")
	big := States(&accessibility.Node{Role: role.Table, RowCount: manageDescendantsRowThreshold + 1}, true, false)
	c.True(big.Has(StateManagesDescendants))
	c.True(States(&accessibility.Node{
		Role:     role.Tree,
		RowCount: manageDescendantsRowThreshold + 1,
	}, true, false).Has(StateManagesDescendants))
}

func TestStatesOfOrientedControls(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	horizontal := States(&accessibility.Node{
		Role:        role.Slider,
		Orientation: accessibility.OrientationHorizontal,
	}, true, false)
	c.True(horizontal.Has(StateHorizontal))
	c.False(horizontal.Has(StateVertical))
	vertical := States(&accessibility.Node{
		Role:        role.ScrollBar,
		Orientation: accessibility.OrientationVertical,
	}, true, false)
	c.True(vertical.Has(StateVertical))
	c.False(vertical.Has(StateHorizontal))
	neither := States(&accessibility.Node{Role: role.Slider}, true, false)
	c.False(neither.Has(StateHorizontal))
	c.False(neither.Has(StateVertical))
}

func TestAttributes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(dbus.Dict{{Key: toolkitAttribute, Value: toolkitName}},
		Attributes(&accessibility.Node{Role: role.Button}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: levelAttribute, Value: "3"},
	}, Attributes(&accessibility.Node{Role: role.Heading, Level: 3}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: sortAttribute, Value: "ascending"},
		{Key: columnIndexAttribute, Value: "3"},
	}, Attributes(&accessibility.Node{
		Role:        role.ColumnHeader,
		Sort:        accessibility.SortAscending,
		ColumnIndex: 2,
	}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: sortAttribute, Value: "descending"},
		{Key: columnIndexAttribute, Value: "1"},
	}, Attributes(&accessibility.Node{Role: role.ColumnHeader, Sort: accessibility.SortDescending}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: placeholderTextAttribute, Value: "Search"},
	}, Attributes(&accessibility.Node{Role: role.TextField, Placeholder: "Search"}, nil))
}

func TestAttributesOfARowAndItsCells(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// A table describes only the rows it is showing, so the size of the set has to come from the table rather than
	// from how many rows happen to have objects: "row 4 of 6000", not "row 4 of 6".
	table := &accessibility.Node{Role: role.Table, RowCount: 6000, ColumnCount: 3}
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: levelAttribute, Value: "2"},
		{Key: rowIndexAttribute, Value: "4"},
		{Key: positionInSetAttribute, Value: "4"},
		{Key: setSizeAttribute, Value: "6000"},
	}, Attributes(&accessibility.Node{Role: role.Row, Level: 2, RowIndex: 3}, table))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: rowIndexAttribute, Value: "4"},
		{Key: columnIndexAttribute, Value: "2"},
	}, Attributes(&accessibility.Node{Role: role.Cell, RowIndex: 3, ColumnIndex: 1}, table),
		"a cell is placed by row and column rather than by a position in a run")
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: rowIndexAttribute, Value: "1"},
		{Key: positionInSetAttribute, Value: "1"},
	}, Attributes(&accessibility.Node{Role: role.ListItem}, &accessibility.Node{Role: role.List}),
		"a container that does not know how many rows it holds reports no size")
}

func TestLayerFor(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(LayerWindow, layerFor(true))
	c.Equal(LayerWidget, layerFor(false))
}

func TestNodeActionOrder(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	n := &accessibility.Node{
		Role: role.Row,
		Actions: accessibility.ActionSet(0).With(accessibility.ShowContextMenu, accessibility.Decrement,
			accessibility.Collapse, accessibility.Press, accessibility.Focus),
	}
	names := make([]string, 0, 5)
	for _, one := range nodeActions(n) {
		names = append(names, one.name)
	}
	c.Equal([]string{"click", "collapse", "decrement", "menu"}, names)
	c.True(hasActions(n))
	c.False(hasActions(&accessibility.Node{Role: role.Label}))
	c.False(hasActions(&accessibility.Node{
		Role:    role.Label,
		Actions: accessibility.ActionSet(0).With(accessibility.Focus),
	}), "the actions AT-SPI has no name for do not make an Action interface")
}
