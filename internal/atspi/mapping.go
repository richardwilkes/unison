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
	"strconv"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// manageDescendantsRowThreshold is how many rows a table or tree must hold before it claims
// ATSPI_STATE_MANAGES_DESCENDANTS. Claiming it tells an assistive technology not to walk the children, which is what
// keeps a huge table usable, but it also costs the row-by-row reporting that makes a small one pleasant, so it is only
// worth doing once the table is large.
const manageDescendantsRowThreshold = 500

// The names of the object attributes that [Attributes] reports.
const (
	toolkitAttribute         = "toolkit"
	levelAttribute           = "level"
	sortAttribute            = "sort"
	placeholderTextAttribute = "placeholder-text"
)

// MapRole returns the AT-SPI role that represents the node. A role that has no AT-SPI equivalent, which includes the
// two that never reach a published tree (role.Auto and role.None), becomes ATSPI_ROLE_UNKNOWN rather than
// ATSPI_ROLE_INVALID, since the object does exist.
func MapRole(n *accessibility.Node) Role {
	switch n.Role {
	case role.Window:
		return RoleFrame
	case role.Dialog:
		return RoleDialog
	case role.Group, role.TabPanel:
		// A named group is worth announcing as a group, which is what ATSPI_ROLE_GROUPING means; an unnamed one is
		// just layout, and ATSPI_ROLE_PANEL is the role assistive technologies pass over quietly.
		if n.Name != "" {
			return RoleGrouping
		}
		return RolePanel
	case role.Button, role.ColorWell:
		return RolePushButton
	case role.ToggleButton, role.DisclosureTriangle:
		return RoleToggleButton
	case role.CheckBox:
		return RoleCheckBox
	case role.RadioButton:
		return RoleRadioButton
	case role.TextField:
		if n.Protected {
			return RolePasswordText
		}
		return RoleEntry
	case role.TextArea:
		return RoleText
	case role.SpinButton:
		return RoleSpinButton
	case role.ComboBox, role.PopupButton:
		return RoleComboBox
	case role.Label:
		return RoleLabel
	case role.Heading:
		return RoleHeading
	case role.Link:
		return RoleLink
	case role.Image:
		return RoleImage
	case role.List:
		return RoleListBox
	case role.ListItem:
		return RoleListItem
	case role.Table:
		return RoleTable
	case role.Tree:
		return RoleTreeTable
	case role.Row:
		return RoleTableRow
	case role.Cell:
		return RoleTableCell
	case role.ColumnHeader:
		return RoleColumnHeader
	case role.TableHeader:
		return RolePanel
	case role.MenuBar:
		return RoleMenuBar
	case role.Menu:
		return RoleMenu
	case role.MenuItem:
		if n.HasCheck {
			return RoleCheckMenuItem
		}
		return RoleMenuItem
	case role.Separator:
		return RoleSeparator
	case role.Slider:
		return RoleSlider
	case role.ProgressBar:
		return RoleProgressBar
	case role.ScrollBar:
		return RoleScrollBar
	case role.ScrollArea:
		return RoleScrollPane
	case role.TabList:
		return RolePageTabList
	case role.Tab:
		return RolePageTab
	case role.Tooltip:
		return RoleToolTip
	case role.Document:
		return RoleDocumentFrame
	case role.Toolbar:
		return RoleToolBar
	default:
		return RoleUnknown
	}
}

// roleNames holds the name of every role this package reports. The names are the ones libatspi's atspi_role_get_name
// returns, since that is what an assistive technology compares against and what its logs show.
var roleNames = map[Role]string{
	RoleApplication:   "application",
	RoleCheckBox:      "check box",
	RoleCheckMenuItem: "check menu item",
	RoleColumnHeader:  "column header",
	RoleComboBox:      "combo box",
	RoleDialog:        "dialog",
	RoleDocumentFrame: "document frame",
	RoleEntry:         "entry",
	RoleFrame:         "frame",
	RoleGrouping:      "grouping",
	RoleHeading:       "heading",
	RoleImage:         "image",
	RoleInvalid:       "invalid",
	RoleLabel:         "label",
	RoleLink:          "link",
	RoleListBox:       "list box",
	RoleListItem:      "list item",
	RoleMenu:          "menu",
	RoleMenuBar:       "menu bar",
	RoleMenuItem:      "menu item",
	RolePageTab:       "page tab",
	RolePageTabList:   "page tab list",
	RolePanel:         "panel",
	RolePasswordText:  "password text",
	RoleProgressBar:   "progress bar",
	RolePushButton:    "push button",
	RoleRadioButton:   "radio button",
	RoleScrollBar:     "scroll bar",
	RoleScrollPane:    "scroll pane",
	RoleSeparator:     "separator",
	RoleSlider:        "slider",
	RoleSpinButton:    "spin button",
	RoleTable:         "table",
	RoleTableCell:     "table cell",
	RoleTableRow:      "table row",
	RoleText:          "text",
	RoleToggleButton:  "toggle button",
	RoleToolBar:       "tool bar",
	RoleToolTip:       "tool tip",
	RoleTreeTable:     "tree table",
	RoleUnknown:       "unknown",
}

// RoleName returns the name AT-SPI uses for a role, which is what org.a11y.atspi.Accessible.GetRoleName reports. A role
// this package never produces has no name, and reports the one for ATSPI_ROLE_UNKNOWN.
func RoleName(r Role) string {
	if name, exists := roleNames[r]; exists {
		return name
	}
	return roleNames[RoleUnknown]
}

// Interfaces returns the names of the AT-SPI interfaces the node's object implements, in the order
// org.a11y.atspi.Accessible.GetInterfaces reports them. Accessible and Component are always there, since every node has
// a name, a role and a place on the screen.
func Interfaces(n *accessibility.Node) []string {
	list := make([]string, 0, 5)
	list = append(list, InterfaceAccessible)
	if hasActions(n) {
		list = append(list, InterfaceAction)
	}
	list = append(list, InterfaceComponent)
	if supportsSelection(n.Role) {
		list = append(list, InterfaceSelection)
	}
	if n.Text != nil {
		list = append(list, InterfaceText)
	}
	if n.HasNumber {
		list = append(list, InterfaceValue)
	}
	return list
}

// supportsSelection returns true if a role's children are selected one or more at a time, which is what
// org.a11y.atspi.Selection describes.
func supportsSelection(r role.Enum) bool {
	switch r {
	case role.List, role.TabList, role.Table, role.Tree:
		return true
	default:
		return false
	}
}

// States returns the states of the node. windowActive reports whether the window the node belongs to is the active one,
// which the root of a published tree says through its Focused field: AT-SPI expects ATSPI_STATE_FOCUSED on the focused
// object of the active window only, and an assistive technology that sees it anywhere else follows the focus into a
// window the user is not looking at.
func States(n *accessibility.Node, windowActive bool) StateSet {
	var set StateSet
	set = set.With(StateVisible)
	if !n.Offscreen {
		// A node that has been scrolled or clipped out of view is still part of the hierarchy, and still VISIBLE in
		// AT-SPI's sense of "not hidden", but it is not SHOWING.
		set = set.With(StateShowing)
	}
	if !n.Disabled {
		set = set.With(StateEnabled, StateSensitive)
	}
	if n.Focusable {
		set = set.With(StateFocusable)
	}
	if n.Focused && windowActive && !n.Role.IsWindow() {
		set = set.With(StateFocused)
	}
	if n.Busy {
		set = set.With(StateBusy)
	}
	if n.Invalid {
		set = set.With(StateInvalidEntry)
	}
	if n.ReadOnly {
		set = set.With(StateReadOnly)
	}
	if n.Modal {
		set = set.With(StateModal)
	}
	if n.Selectable {
		set = set.With(StateSelectable)
	}
	if n.Selected {
		set = set.With(StateSelected)
	}
	if n.Multiselectable {
		set = set.With(StateMultiselectable)
	}
	if n.Expandable {
		set = set.With(StateExpandable)
		if n.Expanded {
			set = set.With(StateExpanded)
		} else {
			set = set.With(StateCollapsed)
		}
	}
	if n.HasCheck {
		set = set.With(StateCheckable)
		switch n.Checked {
		case check.On:
			set = set.With(StateChecked)
		case check.Mixed:
			set = set.With(StateIndeterminate)
		case check.Off:
		}
	}
	return set.With(roleStates(n)...)
}

// roleStates returns the states that a node has because of its role rather than because of one of its flags.
func roleStates(n *accessibility.Node) []StateBit {
	states := make([]StateBit, 0, 4)
	switch n.Role {
	case role.Window, role.Dialog:
		states = append(states, StateResizable)
		if n.Focused {
			states = append(states, StateActive)
		}
	case role.ToggleButton, role.DisclosureTriangle:
		states = append(states, StateCheckable)
		if n.Pressed {
			states = append(states, StateChecked, StatePressed)
		}
	case role.Table, role.Tree:
		if n.RowCount > manageDescendantsRowThreshold {
			states = append(states, StateManagesDescendants)
		}
	default:
	}
	states = append(states, textStates(n)...)
	if n.Role == role.ComboBox || n.Role == role.PopupButton {
		states = append(states, StateHasPopup)
	}
	switch n.Orientation {
	case accessibility.OrientationHorizontal:
		states = append(states, StateHorizontal)
	case accessibility.OrientationVertical:
		states = append(states, StateVertical)
	case accessibility.OrientationNone:
	}
	return states
}

// textStates returns the states that describe a node's text content, for the roles that have some.
func textStates(n *accessibility.Node) []StateBit {
	var states []StateBit
	switch n.Role {
	case role.TextField, role.SpinButton, role.ComboBox:
		states = []StateBit{StateSingleLine, StateSelectableText}
	case role.TextArea:
		states = []StateBit{StateMultiLine, StateSelectableText}
	case role.Document:
		// A document is readable but never editable, so it gets neither EDITABLE nor a line count claim beyond being
		// more than one line long.
		return []StateBit{StateMultiLine, StateSelectableText}
	default:
		return nil
	}
	if !n.ReadOnly {
		states = append(states, StateEditable)
	}
	return states
}

// Attributes returns the node's AT-SPI object attributes, which are the pieces of information that have no interface of
// their own. Every object reports the toolkit it came from; the rest appear only when they apply.
func Attributes(n *accessibility.Node) dbus.Dict {
	attributes := make(dbus.Dict, 0, 4)
	attributes = append(attributes, dbus.DictEntry{Key: toolkitAttribute, Value: toolkitName})
	if n.Level > 0 {
		attributes = append(attributes, dbus.DictEntry{
			Key:   levelAttribute,
			Value: strconv.Itoa(n.Level),
		})
	}
	switch n.Sort {
	case accessibility.SortAscending:
		attributes = append(attributes, dbus.DictEntry{Key: sortAttribute, Value: "ascending"})
	case accessibility.SortDescending:
		attributes = append(attributes, dbus.DictEntry{Key: sortAttribute, Value: "descending"})
	case accessibility.SortNone:
	}
	if n.Placeholder != "" {
		attributes = append(attributes, dbus.DictEntry{Key: placeholderTextAttribute, Value: n.Placeholder})
	}
	return attributes
}

// layerFor returns the layer a node is in: a top-level window is in the window layer, and everything inside one is in
// the widget layer.
func layerFor(n *accessibility.Node) Layer {
	if n.Role.IsWindow() {
		return LayerWindow
	}
	return LayerWidget
}
