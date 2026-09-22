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
	role.Paragraph:          RoleParagraph,
	role.BlockQuote:         RoleBlockQuote,
	role.Code:               RoleParagraph,
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
	role.List:               RoleList,
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
	c.Equal("paragraph", RoleName(RoleParagraph))
	c.Equal("list", RoleName(RoleList))
	c.Equal("list box", RoleName(RoleListBox))
	c.Equal("block quote", RoleName(RoleBlockQuote))
	c.Equal("unknown", RoleName(Role(12345)))
}

func TestInterfaces(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name       string
		expected   []string
		node       accessibility.Node
		spanTarget bool
	}{
		{
			name:     "a label has nothing but the basics",
			node:     accessibility.Node{Role: role.Label},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name: "a button can be pressed",
			node: accessibility.Node{
				Role:    role.Button,
				Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
			},
			expected: []string{InterfaceAccessible, InterfaceAction, InterfaceCollection, InterfaceComponent},
		},
		{
			name: "a slider has a value and can be nudged",
			node: accessibility.Node{
				Role:      role.Slider,
				HasNumber: true,
				Actions:   accessibility.ActionSet(0).With(accessibility.Increment, accessibility.SetValue),
			},
			expected: []string{InterfaceAccessible, InterfaceAction, InterfaceCollection, InterfaceComponent, InterfaceValue},
		},
		{
			name:     "a progress bar has a value it cannot change",
			node:     accessibility.Node{Role: role.ProgressBar, HasNumber: true},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceValue},
		},
		{
			name:     "a list holds a selection",
			node:     accessibility.Node{Role: role.List},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceSelection},
		},
		{
			name:     "a tree holds a selection and is laid out as a grid",
			node:     accessibility.Node{Role: role.Tree},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceSelection, InterfaceTable},
		},
		{
			name:     "a table is a grid as well as a selection",
			node:     accessibility.Node{Role: role.Table, RowCount: 3, ColumnCount: 2},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceSelection, InterfaceTable},
		},
		{
			name:     "a cell says where in the grid it sits",
			node:     accessibility.Node{Role: role.Cell, RowIndex: 1, ColumnIndex: 0},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceTableCell},
		},
		{
			name:     "a row is the container the cells sit in rather than one of them",
			node:     accessibility.Node{Role: role.Row, RowIndex: 1},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name:     "a list is not a grid",
			node:     accessibility.Node{Role: role.List, RowCount: 3},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceSelection},
		},
		{
			name: "focus alone is not an AT-SPI action",
			node: accessibility.Node{
				Role:    role.TextField,
				Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection),
			},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name: "a field with content holds navigable text, and is a kind of control that is typed into",
			node: accessibility.Node{
				Role: role.TextField,
				Text: &accessibility.TextInfo{Text: "content"},
			},
			expected: []string{
				InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceEditableText, InterfaceText,
			},
		},
		{
			name: "the same field with its content locked has nothing to edit",
			node: accessibility.Node{
				Role:     role.TextField,
				ReadOnly: true,
				Text:     &accessibility.TextInfo{Text: "content"},
			},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		},
		{
			name:     "a password field has no text to navigate",
			node:     accessibility.Node{Role: role.TextField, Protected: true},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name: "a spin button has both text and a number, in that order",
			node: accessibility.Node{
				Role:      role.SpinButton,
				HasNumber: true,
				Text:      &accessibility.TextInfo{Text: "42"},
			},
			expected: []string{
				InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceEditableText, InterfaceText,
				InterfaceValue,
			},
		},
		{
			name: "a paragraph whose text holds a link has a hypertext",
			node: accessibility.Node{
				Role:     role.Paragraph,
				ReadOnly: true,
				Text: &accessibility.TextInfo{
					Text:  "see this",
					Spans: []accessibility.TextSpan{{Node: 2, Start: 4, End: 8}},
				},
			},
			expected: []string{
				InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceHypertext, InterfaceText,
			},
		},
		{
			name: "a paragraph whose text holds nothing has none",
			node: accessibility.Node{
				Role:     role.Paragraph,
				ReadOnly: true,
				Text:     &accessibility.TextInfo{Text: "see this"},
			},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		},
		{
			name: "a label carrying its text hands it over to be read by line, word and character",
			node: accessibility.Node{
				Role: role.Label,
				Text: &accessibility.TextInfo{Text: "Name:"},
			},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		},
		{
			name:     "a link leads somewhere",
			node:     accessibility.Node{Role: role.Link, Name: "Home", URL: "https://example.com/"},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceHyperlink},
		},
		{
			name:       "and so, as far as AT-SPI is concerned, does an image within someone's text",
			node:       accessibility.Node{Role: role.Image, Name: "Diagram"},
			spanTarget: true,
			expected:   []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceHyperlink},
		},
		{
			name:     "while an image that is nobody's is just an image",
			node:     accessibility.Node{Role: role.Image, Name: "Diagram"},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name: "a document hands over no text at all, however much of it it has composed",
			node: accessibility.Node{
				Role:     role.Document,
				Document: &accessibility.DocumentInfo{Text: accessibility.TextInfo{Text: "a whole document"}},
			},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
	} {
		c.Equal(one.expected, Interfaces(&one.node, one.spanTarget), one.name)
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

// TestACheckableCellCarriesTheStateAndNotTheWordsForIt verifies that a table cell mirroring the check box within it —
// a value of "Checked" alongside HasCheck — reports the check through its states and hands over no text: Orca reads the
// state off a cell that can be toggled, and text on the cell would be spoken on top of it and in place of the check
// box's own label.
func TestACheckableCellCarriesTheStateAndNotTheWordsForIt(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	cell := &accessibility.Node{Role: role.Cell, Value: "Checked", HasCheck: true, Checked: checkenum.On}
	states := States(cell, true, false)
	c.True(states.Has(StateCheckable))
	c.True(states.Has(StateChecked))
	c.False(supportsText(cell), "the words for the state are not text of the cell's own")
	c.Equal("", textualValue(cell))
	c.False(slices.Contains(Interfaces(cell, false), InterfaceText), "and so advertises no text interface")
	plain := &accessibility.Node{Role: role.Cell, Value: "35 KB"}
	c.True(supportsText(plain), "a cell whose value is its content still reports it as text")
	c.False(States(plain, true, false).Has(StateCheckable))
}

// TestACheckableControlKeepsAValueOfItsOwn verifies that the words a checkable node other than a mirroring cell carries
// are still reported. An application that fills in a value from its callback — a check box or a toggle button saying
// how much of something is on — is saying what the check state does not, and Linux would be the only platform to drop
// it.
func TestACheckableControlKeepsAValueOfItsOwn(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	box := &accessibility.Node{Role: role.CheckBox, Value: "3 of 5", HasCheck: true, Checked: checkenum.Mixed}
	c.True(supportsText(box), "a check box carrying a value of its own reports it")
	c.Equal("3 of 5", textualValue(box))
	toggle := &accessibility.Node{Role: role.ToggleButton, Value: "3 of 5", Pressed: true}
	c.True(States(toggle, true, false).Has(StateCheckable), "a toggle button is checkable without a check of its own")
	c.True(supportsText(toggle), "which does not take its value away")
	c.Equal("3 of 5", textualValue(toggle))
}

func TestStatesOfTextControls(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	field := States(&accessibility.Node{
		Role:    role.TextField,
		Text:    &accessibility.TextInfo{},
		Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection),
	}, true, false)
	c.True(field.Has(StateSingleLine))
	c.True(field.Has(StateSelectableText), "a field offers to have a selection put in it")
	// For a control that is typed into, SELECTABLE_TEXT is worked out from the role and the text, the way EDITABLE
	// is, rather than from the action set: a disabled field has its actions narrowed to the ones it will still answer,
	// and the two halves of the text state set have to move together or not at all.
	c.True(States(&accessibility.Node{Role: role.TextField, Text: &accessibility.TextInfo{}}, true,
		false).Has(StateSelectableText), "a field whose actions were narrowed still holds a selection")
	c.False(States(&accessibility.Node{Role: role.ListItem, Text: &accessibility.TextInfo{}}, true,
		false).Has(StateSelectableText), "a role nothing is read through by caret does not claim one")
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
	document := States(&accessibility.Node{
		Role:    role.Document,
		Text:    &accessibility.TextInfo{},
		Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection),
	}, true, false)
	c.True(document.Has(StateMultiLine))
	c.True(document.Has(StateSelectableText))
	c.False(document.Has(StateEditable), "a document is never editable")
	combo := States(&accessibility.Node{Role: role.ComboBox, Text: &accessibility.TextInfo{}}, true, false)
	c.True(combo.Has(StateHasPopup))
	c.True(combo.Has(StateEditable))
	c.True(States(&accessibility.Node{Role: role.PopupButton}, true, false).Has(StateHasPopup))
}

// TestStatesOfWrappingTextControls covers the one text control whose line count the role does not settle: a
// single-line field that wraps holds one line of text laid out over several, and says so through
// [accessibility.TextInfo.Multiline]. Reporting it as SINGLE_LINE would have an assistive technology read the whole of
// it as one run and offer no line-by-line navigation through what the user can see on the screen.
func TestStatesOfWrappingTextControls(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	wrapped := States(&accessibility.Node{
		Role:    role.TextField,
		Text:    &accessibility.TextInfo{Text: "a long line that wraps", Multiline: true},
		Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection),
	}, true, false)
	c.True(wrapped.Has(StateMultiLine), "a wrapping field lays its content out over more than one line")
	c.False(wrapped.Has(StateSingleLine))
	c.True(wrapped.Has(StateSelectableText))
	c.True(wrapped.Has(StateEditable))
	// A text area is multi-line whatever it happens to hold, so an empty one is not reported as a single-line control
	// just because nothing has been typed into it yet.
	empty := States(&accessibility.Node{Role: role.TextArea, Text: &accessibility.TextInfo{}}, true, false)
	c.True(empty.Has(StateMultiLine))
	c.False(empty.Has(StateSingleLine))
	// The other roles that carry text follow the text as well.
	spin := States(&accessibility.Node{
		Role: role.SpinButton,
		Text: &accessibility.TextInfo{Text: "1", Multiline: true},
	}, true, false)
	c.True(spin.Has(StateMultiLine))
	c.False(spin.Has(StateSingleLine))
	combo := States(&accessibility.Node{
		Role: role.ComboBox,
		Text: &accessibility.TextInfo{Text: "one"},
	}, true, false)
	c.True(combo.Has(StateSingleLine))
	c.False(combo.Has(StateMultiLine))
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
	c.False(slices.Contains(Interfaces(password, false), InterfaceText), "a protected field hands over no text")
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

// TestStatesOfALabelCarryingItsText covers the states of static text, which a label is: it is read by line, word and
// character like the content of a field, but there is no caret in it and nothing about it is typed into. A label that
// claimed EDITABLE would have Orca offer to type into a piece of the window that cannot take the focus.
func TestStatesOfALabelCarryingItsText(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	label := &accessibility.Node{Role: role.Label, Text: &accessibility.TextInfo{Text: "Name:"}}
	set := States(label, true, false)
	c.True(set.Has(StateSingleLine), "a label is laid out over the one line it was drawn on")
	c.False(set.Has(StateSelectableText), "nothing selects within a label, so it does not offer to")
	c.False(set.Has(StateMultiLine))
	c.False(set.Has(StateEditable), "static text is never typed into")
	c.False(supportsEditableText(label))
	interfaces := Interfaces(label, false)
	c.False(slices.Contains(interfaces, InterfaceEditableText))
	c.False(slices.Contains(interfaces, InterfaceHypertext), "a label's text is nobody's link")
	// A label drawn by this toolkit lays its title out on one line and never reports otherwise, so nothing in the root
	// package produces the multi-line answer today. It is asserted all the same because the states are read from the
	// text rather than written down per role: a label that does wrap, whenever one arrives, describes itself the way
	// every other text-bearing node does, and this is what holds that open.
	label.Text.Multiline = true
	c.True(States(label, true, false).Has(StateMultiLine))

	// A list item is not static text: the words a reader moves through live on the label within it, so an item that
	// happens to carry text is given no line states to describe text it does not lay out itself.
	item := &accessibility.Node{Role: role.ListItem, Text: &accessibility.TextInfo{Text: "Item one"}}
	c.True(slices.Contains(Interfaces(item, false), InterfaceText), "its text is still readable")
	itemStates := States(item, true, false)
	c.False(itemStates.Has(StateSingleLine))
	c.False(itemStates.Has(StateMultiLine))
	c.False(itemStates.Has(StateSelectableText))

	// A label that publishes no text of its own has nothing to say about lines either.
	plain := States(&accessibility.Node{Role: role.Label, Name: "Name:"}, true, false)
	c.False(plain.Has(StateSingleLine))
	c.False(plain.Has(StateMultiLine))
	c.False(plain.Has(StateSelectableText))
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
		Attributes(nil, &accessibility.Node{Role: role.Button}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: levelAttribute, Value: "3"},
	}, Attributes(nil, &accessibility.Node{Role: role.Heading, Level: 3}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: sortAttribute, Value: "ascending"},
		{Key: columnIndexAttribute, Value: "3"},
	}, Attributes(nil, &accessibility.Node{
		Role:        role.ColumnHeader,
		Sort:        accessibility.SortAscending,
		ColumnIndex: 2,
	}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: sortAttribute, Value: "descending"},
		{Key: columnIndexAttribute, Value: "1"},
	}, Attributes(nil, &accessibility.Node{Role: role.ColumnHeader, Sort: accessibility.SortDescending}, nil))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: placeholderTextAttribute, Value: "Search"},
	}, Attributes(nil, &accessibility.Node{Role: role.TextField, Placeholder: "Search"}, nil))
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
	}, Attributes(nil, &accessibility.Node{Role: role.Row, Level: 2, RowIndex: 3}, table))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: rowIndexAttribute, Value: "4"},
		{Key: columnIndexAttribute, Value: "2"},
	}, Attributes(nil, &accessibility.Node{Role: role.Cell, RowIndex: 3, ColumnIndex: 1}, table),
		"a cell is placed by row and column rather than by a position in a run")
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: rowIndexAttribute, Value: "1"},
		{Key: positionInSetAttribute, Value: "1"},
	}, Attributes(nil, &accessibility.Node{Role: role.ListItem}, &accessibility.Node{Role: role.List}),
		"a container that does not know how many rows it holds reports no size")
}

// TestAttributesOfARunOfSiblings covers the sets that are all there in the tree rather than being sampled the way a
// table's rows are: a tab, a menu item and a radio button are numbered by counting their siblings, which is what lets
// an assistive technology say "tab 2 of 3" and "radio button 1 of 2" — something every ATK-based toolkit says and both
// of the other adapters report.
func TestAttributesOfARunOfSiblings(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	tree := treeOf(1,
		&accessibility.Node{ID: 1, Role: role.Window, Children: []accessibility.NodeID{2, 6, 10}},
		&accessibility.Node{ID: 2, Parent: 1, Role: role.TabList, Children: []accessibility.NodeID{3, 4, 5}},
		&accessibility.Node{ID: 3, Parent: 2, Role: role.Tab, Name: "Summary"},
		&accessibility.Node{ID: 4, Parent: 2, Role: role.Tab, Name: "Details"},
		&accessibility.Node{ID: 5, Parent: 2, Role: role.Tab, Name: "History"},
		&accessibility.Node{ID: 6, Parent: 1, Role: role.Group, Children: []accessibility.NodeID{7, 8, 9}},
		&accessibility.Node{ID: 7, Parent: 6, Role: role.RadioButton, Name: "Yes"},
		&accessibility.Node{ID: 8, Parent: 6, Role: role.Separator},
		&accessibility.Node{ID: 9, Parent: 6, Role: role.RadioButton, Name: "No"},
		&accessibility.Node{ID: 10, Parent: 1, Role: role.Menu, Children: []accessibility.NodeID{11, 12}},
		&accessibility.Node{ID: 11, Parent: 10, Role: role.MenuItem, Name: "Open"},
		&accessibility.Node{ID: 12, Parent: 10, Role: role.Group, Ignored: true, Children: []accessibility.NodeID{13}},
		&accessibility.Node{ID: 13, Parent: 12, Role: role.MenuItem, Name: "Close"},
	)
	list := tree.Node(2)
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: positionInSetAttribute, Value: "2"},
		{Key: setSizeAttribute, Value: "3"},
	}, Attributes(tree, tree.Node(4), list))
	// Only the siblings that share the role are counted, so the separator between the two radio buttons is passed over
	// rather than making them one and three of three.
	group := tree.Node(6)
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: positionInSetAttribute, Value: "2"},
		{Key: setSizeAttribute, Value: "2"},
	}, Attributes(tree, tree.Node(9), group))
	c.Equal(dbus.Dict{{Key: toolkitAttribute, Value: toolkitName}}, Attributes(tree, tree.Node(8), group),
		"a separator is not one of a numbered run")
	c.Equal(dbus.Dict{{Key: toolkitAttribute, Value: toolkitName}}, Attributes(nil, tree.Node(4), list),
		"a caller with no tree to count has nothing to report")

	// A published snapshot counts the same sets from the indexes it built when it was published rather than deriving a
	// sibling list from the tree for every node it is asked about, which is what keeps an
	// org.a11y.atspi.Collection search over a menu or a radio group from rebuilding that list once per candidate. The
	// two have to agree node for node, including where an ignored container's children are spliced into the run: the
	// second menu item sits under a group nobody is shown, and is the second of two all the same.
	data := newWindowData(tree, sampleGeometry())
	tree.Walk(func(n *accessibility.Node) bool {
		c.Equal(Attributes(tree, n, tree.Node(tree.UnignoredParent(n.ID))), data.attributesOf(n),
			"the attributes of node %d", n.ID)
		position, size := tree.PositionInSet(n.ID)
		fromSnapshot, sizeFromSnapshot := data.positionInSet(n.ID)
		c.Equal(position, fromSnapshot, "where node %d sits in its set", n.ID)
		c.Equal(size, sizeFromSnapshot, "how big node %d's set is", n.ID)
		return true
	})
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: positionInSetAttribute, Value: "2"},
		{Key: setSizeAttribute, Value: "2"},
	}, data.attributesOf(tree.Node(13)), "the spliced menu item is the second of the menu's two items")
}

// TestTextInterfaceFromATextualValue covers the one way AT-SPI has of reporting a value that is not a number. A popup
// menu reports the item it has chosen as its Value, and a color well the ink it holds, and neither has an
// org.a11y.atspi.Value interface to hand a number over through, so the value becomes the content of a read-only text
// interface instead: without it Orca is told a combo box's name and role and never which item is in it.
func TestTextInterfaceFromATextualValue(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name       string
		expected   []string
		node       accessibility.Node
		spanTarget bool
	}{
		{
			name:     "a popup button reports the item it has chosen",
			node:     accessibility.Node{Role: role.PopupButton, Value: testChosenValue},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		},
		{
			name:     "so does a table cell whose content was moved into its value",
			node:     accessibility.Node{Role: role.Cell, Value: "10"},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceTableCell, InterfaceText},
		},
		{
			name:     "a value that is a number is reported as one instead",
			node:     accessibility.Node{Role: role.Slider, Value: "40%", HasNumber: true, Number: 0.4},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceValue},
		},
		{
			name:     "a node with no value at all has no text",
			node:     accessibility.Node{Role: role.PopupButton},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name:     "and neither has a password field, whatever it is carrying",
			node:     accessibility.Node{Role: role.TextField, Protected: true, Value: "hunter2"},
			expected: []string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		},
		{
			name: "text of its own is the real thing rather than a description of it",
			node: accessibility.Node{
				Role:  role.TextField,
				Value: testFieldValue,
				Text:  &accessibility.TextInfo{Text: testFieldValue},
			},
			expected: []string{
				InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceEditableText, InterfaceText,
			},
		},
	} {
		c.Equal(one.expected, Interfaces(&one.node, one.spanTarget), one.name)
	}
	// The states that describe text are not claimed for a synthesized one: they describe a control the user is inside,
	// with a caret and a selection, which a value is not.
	set := States(&accessibility.Node{Role: role.ComboBox, Value: testChosenValue}, true, false)
	c.False(set.Has(StateEditable))
	c.False(set.Has(StateSelectableText))
	c.False(set.Has(StateSingleLine))
}

// TestTheEditableStateAndInterfaceAgree covers the pair that used to be derived from two different rules: the interface
// from the actions the node offers and the state from its role and its text. A disabled field keeps its text while
// axSnapshot strips its actions, so it claimed ATSPI_STATE_EDITABLE with no interface to edit it through.
func TestTheEditableStateAndInterfaceAgree(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name     string
		node     accessibility.Node
		editable bool
	}{
		{
			name: "a field that offers to have its text replaced",
			node: accessibility.Node{
				Role:    role.TextField,
				Text:    &accessibility.TextInfo{Text: testFieldValue},
				Actions: accessibility.ActionSet(0).With(accessibility.ReplaceText),
			},
			editable: true,
		},
		{
			name: "a disabled field, whose actions have been stripped along with its sensitivity",
			node: accessibility.Node{
				Role:     role.TextField,
				Disabled: true,
				Text:     &accessibility.TextInfo{Text: testFieldValue},
				Actions:  accessibility.ActionSet(0).With(accessibility.Focus),
			},
			editable: true,
		},
		{
			name: "a field whose content cannot be changed at all",
			node: accessibility.Node{
				Role:     role.TextArea,
				ReadOnly: true,
				Text:     &accessibility.TextInfo{Text: "done"},
			},
		},
		{
			name: "a field that is not carrying its text",
			node: accessibility.Node{
				Role:    role.TextField,
				Actions: accessibility.ActionSet(0).With(accessibility.ReplaceText),
			},
		},
		{
			name: "a role that is never typed into",
			node: accessibility.Node{Role: role.Label, Text: &accessibility.TextInfo{Text: testLabelName}},
		},
	} {
		c.Equal(one.editable, supportsEditableText(&one.node), one.name)
		c.Equal(one.editable, slices.Contains(Interfaces(&one.node, false), InterfaceEditableText), one.name)
		c.Equal(one.editable, States(&one.node, true, false).Has(StateEditable), one.name)
	}
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

// TestStaticListsAreListsNotListBoxes covers the one role that two different AT-SPI roles are wanted for. A Unison List
// is a control a person picks from, which is what ATSPI_ROLE_LIST_BOX means; the lists of a Markdown document are
// static panels of items a person reads, which is what ATSPI_ROLE_LIST means and the only thing Orca's L and I
// structural navigation will find. A list box is presented as a control and never navigated into.
func TestStaticListsAreListsNotListBoxes(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(RoleList, MapRole(&accessibility.Node{Role: role.List}), "a static list of items to read")
	c.Equal(RoleListBox, MapRole(&accessibility.Node{Role: role.List, Focusable: true}),
		"a list that takes the focus is a control")
	c.Equal(RoleListBox, MapRole(&accessibility.Node{Role: role.List, RowCount: 3}),
		"and so is one that samples its rows")
	c.Equal(RoleListBox, MapRole(&accessibility.Node{Role: role.List, Focusable: true, RowCount: 3}))
	// A greyed-out list box is still a control. Panel.Focusable answers only while the panel is enabled, so the
	// snapshot's Focusable is false for a disabled List exactly as it is for a static panel, and reading that alone
	// would hand Orca an ordinary disabled control as static document content until it was enabled again.
	c.Equal(RoleListBox, MapRole(&accessibility.Node{Role: role.List, Disabled: true}),
		"a disabled list is a control that cannot be used rather than a list of items to read")
	c.Equal(RoleListBox, MapRole(&accessibility.Node{Role: role.List, Disabled: true, RowCount: 3}))
	// The items are list items either way, since AT-SPI has one role for both.
	c.Equal(RoleListItem, MapRole(&accessibility.Node{Role: role.ListItem}))
	// A list is a selection whichever of the two it is reported as, and never a grid.
	for _, list := range []accessibility.Node{{Role: role.List}, {Role: role.List, Focusable: true}} {
		interfaces := Interfaces(&list, false)
		c.True(slices.Contains(interfaces, InterfaceSelection))
		c.False(slices.Contains(interfaces, InterfaceTable))
	}
}

// TestStatesOfReadOnlyTextBlocks covers the state set of the blocks a document is made of. Each carries text a person
// can move a caret and a selection through but cannot change, and says whether it was laid out over one line or
// several, which is what decides whether Orca's caret navigation offers to move through it line by line.
func TestStatesOfReadOnlyTextBlocks(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name string
		role role.Enum
	}{
		{name: "a paragraph", role: role.Paragraph},
		{name: "a block of code", role: role.Code},
		{name: "a heading", role: role.Heading},
		{name: "a table cell", role: role.Cell},
		{name: "a column header", role: role.ColumnHeader},
	} {
		// A block of a document offers the caret and the selection its reader moves through it with, which is what
		// markdown_accessibility.go publishes on every one of them, and is what SELECTABLE_TEXT says.
		block := accessibility.Node{
			Role:     one.role,
			ReadOnly: true,
			Text:     &accessibility.TextInfo{Text: "a line of it"},
			Actions:  accessibility.ActionSet(0).With(accessibility.SetTextSelection),
		}
		set := States(&block, true, false)
		c.True(set.Has(StateSingleLine), "%s laid out over one line", one.name)
		c.False(set.Has(StateMultiLine), "%s", one.name)
		c.True(set.Has(StateSelectableText), "%s can have a selection put in it", one.name)
		// For a block that is only read, the state follows the offer: a heading built out of a label and a table's
		// column header share these roles with a document's blocks, carry text of their own, and have no caret to
		// move, so a block without the offer claims no selection an assistive technology is then refused.
		unselectable := block
		unselectable.Actions = accessibility.ActionSet(0)
		c.False(States(&unselectable, true, false).Has(StateSelectableText),
			"%s that takes no selection does not claim it does", one.name)
		c.False(States(&accessibility.Node{Role: one.role, ReadOnly: true}, true, false).Has(StateSelectableText),
			"%s carrying no text has no selection to offer", one.name)
		c.True(set.Has(StateReadOnly), "%s cannot be changed", one.name)
		c.False(set.Has(StateEditable), "%s is not typed into", one.name)
		c.False(supportsEditableText(&block), "%s", one.name)
		c.False(slices.Contains(Interfaces(&block, false), InterfaceEditableText), "%s", one.name)
		block.Text.Multiline = true
		wrapped := States(&block, true, false)
		c.True(wrapped.Has(StateMultiLine), "%s laid out over several lines", one.name)
		c.False(wrapped.Has(StateSingleLine), "%s", one.name)
		c.True(slices.Contains(textStateNames(&block), stateNameMultiLine),
			"%s announces the line count it has arrived at", one.name)
	}
	// A block that carries no text has neither state, and so has a role that is text-like but holds nothing.
	empty := States(&accessibility.Node{Role: role.Paragraph, ReadOnly: true}, true, false)
	c.False(empty.Has(StateSingleLine))
	c.False(empty.Has(StateMultiLine))
	c.False(empty.Has(StateSelectableText))
}

// TestAttributesOfACodeBlock covers the one thing that says a paragraph is really a block of code. [MapRole] reports it
// as ATSPI_ROLE_PARAGRAPH, since that is the role a reader arrows through and the role Orca's P navigation looks for,
// so the pair of attributes Orca reads for the same thing on a web page is what is left to say it with.
func TestAttributesOfACodeBlock(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	code := &accessibility.Node{Role: role.Code, ReadOnly: true, Text: &accessibility.TextInfo{Text: "go build"}}
	c.Equal(RoleParagraph, MapRole(code))
	c.Equal(dbus.Dict{
		{Key: toolkitAttribute, Value: toolkitName},
		{Key: xmlRolesAttribute, Value: codeXMLRole},
		{Key: tagAttribute, Value: preTag},
	}, Attributes(nil, code, nil))
	// An ordinary paragraph says nothing of the sort, which is what tells the two apart.
	c.Equal(dbus.Dict{{Key: toolkitAttribute, Value: toolkitName}},
		Attributes(nil, &accessibility.Node{Role: role.Paragraph}, nil))
}

// TestDocumentHasNoTextInterface covers the one thing a document must not do. Orca enters browse mode when the focus
// lands in an object with a document role and then reads the document by walking the objects within it, moving its
// caret from one to the next; a document that also handed over the whole content as text would have Orca read
// everything twice, once as the stream and once block by block, and its caret would never leave the document object.
func TestDocumentHasNoTextInterface(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	document := documentTree().Node(101)
	c.NotNil(document.Document, "the document has composed its content into one stream")
	c.Nil(document.Text, "and carries none of it as its own text")
	c.Equal([]string{
		InterfaceAccessible, InterfaceAction, InterfaceCollection, InterfaceComponent,
	}, ta.one(NodePath(101), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal(uint32(RoleDocumentFrame), ta.one(NodePath(101), InterfaceAccessible, "GetRole", ""))
	for _, member := range []struct {
		name string
		sig  dbus.Signature
		args []any
	}{
		{name: "GetText", sig: "ii", args: []any{int32(0), int32(-1)}},
		{name: "GetStringAtOffset", sig: "iu", args: []any{int32(0), uint32(GranularityLine)}},
		{name: "SetCaretOffset", sig: "i", args: []any{int32(1)}},
		{name: "GetAttributes", sig: "i", args: []any{int32(0)}},
		{name: "ScrollSubstringTo", sig: "iiu", args: []any{int32(0), int32(1), uint32(0)}},
	} {
		c.Equal(dbus.UnknownInterface,
			ta.errorName(NodePath(101), InterfaceText, member.name, member.sig, member.args...),
			"a document answers no text call, and %s is one", member.name)
	}
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(101), InterfaceHypertext, "GetNLinks", ""),
		"the spans of its stream are not hypertext links either")
	// Not even a value can put text on a document. Every other role with a value and no text of its own is handed a
	// read-only text interface synthesized from it, which for a document would be one more thing for Orca to read the
	// content through.
	valued := documentTree()
	valued.Generation++
	valued.Node(101).Value = "a whole document"
	c.Equal("", textualValue(valued.Node(101)))
	ta.Publish(documentWindow, valued, nil, sampleGeometry())
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(101), InterfaceText, "GetText", "ii", int32(0), int32(-1)))
	// None of the states that describe a control holding text are claimed, since there is no text interface for a
	// client to reach for.
	states, ok := ta.one(NodePath(101), InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	set := StateSet{states[0], states[1]}
	c.False(set.Has(StateSelectableText))
	c.False(set.Has(StateMultiLine))
	c.False(set.Has(StateSingleLine))
	c.True(set.Has(StateFocused), "what it does report is that it holds the keyboard focus")

	// A widget that filled in both is refused just as firmly, so that the guarantee does not rest on what a snapshot
	// builder happens to publish. [supportsText] reads the stream before it reads the text, and [lineState] refuses the
	// same node, which is what keeps the interface and the states that describe it together.
	both := documentTree()
	both.Generation += 2
	both.Node(101).Text = &accessibility.TextInfo{Text: documentStream().Text.Text, Multiline: true}
	c.False(supportsText(both.Node(101)), "a document carrying a stream has no text interface whatever else it holds")
	_, ok = lineState(both.Node(101))
	c.False(ok, "and neither of the line states")
	c.Nil(textStates(both.Node(101)))
	ta.Publish(documentWindow, both, nil, sampleGeometry())
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(101), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"a document that also carries its stream as its own text hands out none of it")
	states, ok = ta.one(NodePath(101), InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	set = StateSet{states[0], states[1]}
	c.False(set.Has(StateSelectableText))
	c.False(set.Has(StateMultiLine))
	c.False(set.Has(StateSingleLine))
}

// TestDisablingAFieldKeepsTheTextStates covers the half of the text state set that used to follow the action set. A
// disabled node has its actions narrowed by axSnapshot to the one it will still answer, so a SELECTABLE_TEXT worked out
// from [accessibility.SetTextSelection] was retracted by nothing more than the field being disabled, while EDITABLE —
// which is worked out from the role and the text — stayed. No ATK toolkit sends that retraction: an insensitive
// GtkEntry keeps both, since what has gone is the ability to reach the control at all rather than anything about the
// text in it.
func TestDisablingAFieldKeepsTheTextStates(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	enabled := &accessibility.Node{
		Role: role.TextField, Text: &accessibility.TextInfo{Text: testFieldValue},
		Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection),
	}
	disabled := *enabled
	disabled.Disabled = true
	disabled.Actions = accessibility.ActionSet(0).With(accessibility.ScrollIntoView)
	set := States(&disabled, true, false)
	c.True(set.Has(StateSelectableText), "a disabled field still holds text a selection could be put in")
	c.True(set.Has(StateEditable), "as the other half of the same set has always said")
	c.True(set.Has(StateSingleLine))
	c.Nil(attributeStateChanges(enabled, &disabled), "so disabling it retracts none of them")
}
