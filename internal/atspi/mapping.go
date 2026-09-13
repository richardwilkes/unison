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

// The names of the object attributes that [Attributes] reports. The four that describe a position within a set are
// spelled and numbered the way the ARIA attributes of the same names are, which is what every assistive technology
// already reads them as: one-based, and counting the whole set rather than the part of it that happens to be described.
const (
	toolkitAttribute         = "toolkit"
	levelAttribute           = "level"
	sortAttribute            = "sort"
	placeholderTextAttribute = "placeholder-text"
	rowIndexAttribute        = "rowindex"
	columnIndexAttribute     = "colindex"
	positionInSetAttribute   = "posinset"
	setSizeAttribute         = "setsize"
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
	list := make([]string, 0, 7)
	list = append(list, InterfaceAccessible)
	if hasActions(n) {
		list = append(list, InterfaceAction)
	}
	list = append(list, InterfaceComponent)
	if supportsSelection(n.Role) {
		list = append(list, InterfaceSelection)
	}
	if supportsTable(n.Role) {
		list = append(list, InterfaceTable)
	}
	if supportsTableCell(n.Role) {
		list = append(list, InterfaceTableCell)
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

// supportsTable returns true if a role is laid out as a grid of cells, which is what org.a11y.atspi.Table describes and
// what an assistive technology's table navigation commands work over. A list is not one: AT-SPI has a list box for a
// single column of items, and its rows are reached by walking the children.
func supportsTable(r role.Enum) bool {
	switch r {
	case role.Table, role.Tree:
		return true
	default:
		return false
	}
}

// supportsTableCell returns true if a role is one cell of such a grid, which is what org.a11y.atspi.TableCell
// describes. Only a cell is one: a row is the container the cells sit in, and says where it is through the rowindex
// and posinset attributes rather than through an interface.
func supportsTableCell(r role.Enum) bool {
	return r == role.Cell
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
	if checkable(n) {
		set = set.With(StateCheckable)
	}
	if n.HasCheck {
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
		if n.Resizable {
			// A window created with NotResizableWindowOption cannot be resized, and telling an assistive technology
			// that a fixed-size dialog can be would have it offer the user a way to do something that does nothing.
			states = append(states, StateResizable)
		}
		if n.Focused {
			states = append(states, StateActive)
		}
	case role.ToggleButton, role.DisclosureTriangle:
		// CHECKABLE comes from [checkable], which the whole package reads a node's checkability from; what is left
		// here is the pressed-ness that is this role's check.
		if n.Pressed {
			states = append(states, StateChecked, StatePressed)
		}
	default:
	}
	if ManagesDescendants(n) {
		states = append(states, StateManagesDescendants)
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

// ManagesDescendants reports whether a node claims ATSPI_STATE_MANAGES_DESCENDANTS, which is a promise as much as a
// hint: the container is telling its client not to walk or cache what is inside it, and undertaking in exchange to say
// which descendant is the current one through object:active-descendant-changed. Only a table or a tree with more rows
// than [manageDescendantsRowThreshold] makes that promise, and [Adapter.emitActiveDescendants] keeps it.
func ManagesDescendants(n *accessibility.Node) bool {
	switch n.Role {
	case role.Table, role.Tree:
		return n.RowCount > manageDescendantsRowThreshold
	default:
		return false
	}
}

// checkable reports whether a node's state set holds ATSPI_STATE_CHECKABLE. A node is checkable either because it
// carries a check of its own or because its role is one whose pressed-ness AT-SPI reports as a check; see
// [pressedIsChecked]. This is the one place the answer is worked out, so that [States] and the state changes
// [Adapter.emitStateChanged] sends cannot drift apart.
func checkable(n *accessibility.Node) bool {
	return n != nil && (n.HasCheck || pressedIsChecked(n))
}

// pressedIsChecked reports whether [roleStates] derives ATSPI_STATE_CHECKED from a node's Pressed flag, which is how
// AT-SPI reports the two roles it turns into ATSPI_ROLE_TOGGLE_BUTTON: a toggle that is down is a control that is
// checked, so the two states have to move together.
func pressedIsChecked(n *accessibility.Node) bool {
	switch n.Role {
	case role.ToggleButton, role.DisclosureTriangle:
		return true
	default:
		return false
	}
}

// editableText reports whether a node's state set can hold ATSPI_STATE_EDITABLE once the node is not read-only, which
// only the text-bearing roles a person types into can, and only while they actually carry the text an assistive
// technology would edit. [Adapter.emitStateChanged] needs the answer so that a control becoming read-only does not
// retract a state it never had.
func editableText(n *accessibility.Node) bool {
	if n.Text == nil {
		return false
	}
	switch n.Role {
	case role.TextField, role.SpinButton, role.ComboBox, role.TextArea:
		return true
	default:
		return false
	}
}

// textStates returns the states that describe a node's text content, for the roles that have some. A node whose Text is
// nil gets none of them, however text-like its role: [Interfaces] leaves org.a11y.atspi.Text off such a node, and a
// state set promising text that the object has no interface to hand over would send an assistive technology asking
// questions it cannot answer. A password field is exactly that, since a protected node never carries its text.
func textStates(n *accessibility.Node) []StateBit {
	if n.Text == nil {
		return nil
	}
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
	if editableText(n) && !n.ReadOnly {
		states = append(states, StateEditable)
	}
	return states
}

// Attributes returns the node's AT-SPI object attributes, which are the pieces of information that have no interface of
// their own. Every object reports the toolkit it came from; the rest appear only when they apply.
//
// container is the node's reported parent, or nil when it has none. It is what the size of the set a row belongs to is
// read from: a snapshot describes only the rows that can be seen, plus the selection, so the container is the only
// place that knows how many rows there are altogether, and "row 4 of 6" would otherwise become "row 4 of 6000".
func Attributes(n, container *accessibility.Node) dbus.Dict {
	attributes := make(dbus.Dict, 0, 6)
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
	return appendPositionAttributes(attributes, n, container)
}

// appendPositionAttributes appends the attributes that say where a node sits within the grid or the list it belongs to.
// AT-SPI carries this as object attributes rather than through an interface for everything but a table cell, and even a
// cell's are worth reporting, since an assistive technology that has not asked for org.a11y.atspi.TableCell still reads
// them.
func appendPositionAttributes(attributes dbus.Dict, n, container *accessibility.Node) dbus.Dict {
	if hasRowPosition(n.Role) {
		attributes = append(attributes, dbus.DictEntry{Key: rowIndexAttribute, Value: strconv.Itoa(n.RowIndex + 1)})
	}
	if hasColumnPosition(n.Role) {
		attributes = append(attributes, dbus.DictEntry{
			Key:   columnIndexAttribute,
			Value: strconv.Itoa(n.ColumnIndex + 1),
		})
	}
	if !isSetMember(n.Role) {
		return attributes
	}
	attributes = append(attributes, dbus.DictEntry{
		Key:   positionInSetAttribute,
		Value: strconv.Itoa(n.RowIndex + 1),
	})
	if container != nil && container.RowCount > 0 {
		attributes = append(attributes, dbus.DictEntry{
			Key:   setSizeAttribute,
			Value: strconv.Itoa(container.RowCount),
		})
	}
	return attributes
}

// hasRowPosition returns true if a role's RowIndex says which row of its container the node occupies.
func hasRowPosition(r role.Enum) bool {
	switch r {
	case role.Row, role.Cell, role.ListItem:
		return true
	default:
		return false
	}
}

// hasColumnPosition returns true if a role's ColumnIndex says which column of its container the node occupies.
func hasColumnPosition(r role.Enum) bool {
	switch r {
	case role.Cell, role.ColumnHeader:
		return true
	default:
		return false
	}
}

// isSetMember returns true if a role is one of a numbered run of siblings, which is what posinset and setsize describe.
// Only the rows are: a cell is placed by its row and column rather than by a position in a run.
func isSetMember(r role.Enum) bool {
	switch r {
	case role.Row, role.ListItem:
		return true
	default:
		return false
	}
}

// layerFor returns the layer a node is in: a top-level window is in the window layer, and everything inside one is in
// the widget layer.
func layerFor(n *accessibility.Node) Layer {
	if n.Role.IsWindow() {
		return LayerWindow
	}
	return LayerWidget
}
