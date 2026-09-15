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
	list := make([]string, 0, 8)
	list = append(list, InterfaceAccessible)
	if hasActions(n) {
		list = append(list, InterfaceAction)
	}
	list = append(list, InterfaceComponent)
	if supportsEditableText(n) {
		list = append(list, InterfaceEditableText)
	}
	if supportsSelection(n.Role) {
		list = append(list, InterfaceSelection)
	}
	if supportsTable(n.Role) {
		list = append(list, InterfaceTable)
	}
	if supportsTableCell(n.Role) {
		list = append(list, InterfaceTableCell)
	}
	if supportsText(n) {
		list = append(list, InterfaceText)
	}
	if n.HasNumber {
		list = append(list, InterfaceValue)
	}
	return list
}

// supportsText returns true if a node is given org.a11y.atspi.Text: either it carries navigable text of its own, or its
// value is text that has nowhere else to be reported and is handed over as a read-only text interface synthesized from
// it; see [textualValue]. This is the one place the answer is worked out, so that what [Interfaces] advertises and what
// [nodeObject.Interfaces] implements cannot drift apart.
func supportsText(n *accessibility.Node) bool {
	return n.Text != nil || textualValue(n) != ""
}

// textualValue returns the node's value when that value is text, and an empty string otherwise. AT-SPI carries a value
// as either the number of org.a11y.atspi.Value or the characters of org.a11y.atspi.Text and has nothing else to put one
// in, so a value that is neither a number nor the content of a text control — the item a popup menu has chosen, the ink
// a color well holds, the content a populated table cell reports — would go unreported on Linux altogether, while macOS
// reports it through accessibilityValue and Windows through the value pattern.
//
// A node that carries a number reports it through org.a11y.atspi.Value instead, and one that carries text of its own
// hands over the real thing rather than a description of it. A protected node is left out whatever it holds:
// [accessibility.Node] says a password field carries neither a value nor text, and the cost of being wrong about that
// is the password.
func textualValue(n *accessibility.Node) string {
	if n == nil || n.HasNumber || n.Text != nil || n.Protected {
		return ""
	}
	return n.Value
}

// supportsEditableText returns true if a node's text can be changed through org.a11y.atspi.EditableText, which is the
// only way AT-SPI has of typing into a control: the Text interface moves the caret and the selection but never changes
// a character. It is the predicate [textStates] gives ATSPI_STATE_EDITABLE from as well — a text-bearing role that is
// carrying its text and is not read-only — so that the state and the interface cannot drift apart: a client told a
// control is editable and then handed no interface to edit it with has been told a falsehood, and so has one told the
// reverse.
//
// Which actions the node offers is deliberately no part of it. A disabled field keeps its text while axSnapshot strips
// its actions, and a disabled field is still an editable kind of control — an insensitive GtkEntry reports
// ATSPI_STATE_EDITABLE too — so whether an edit can be carried out at this moment is answered by the methods refusing,
// which is what [nodeObject.replaceRunes] and [nodeObject.setTextContents] do without the actions they need, rather
// than by the interface coming and going.
func supportsEditableText(n *accessibility.Node) bool {
	return editableText(n) && !n.ReadOnly
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
// window the user is not looking at. isRoot reports whether the node is that root, which is the one node whose Focused
// field means the window is active rather than that the node holds the keyboard focus. Only the tree can say which node
// that is: a panel inside a window may report a window role of its own.
func States(n *accessibility.Node, windowActive, isRoot bool) StateSet {
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
	if n.Focused && windowActive && !isRoot {
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
	return set.With(roleStates(n, isRoot)...)
}

// roleStates returns the states that a node has because of its role rather than because of one of its flags. isRoot is
// what [States] was told, since the states that describe a top-level window belong to the window itself rather than to
// a panel within it that happens to report a window role.
func roleStates(n *accessibility.Node, isRoot bool) []StateBit {
	states := make([]StateBit, 0, 4)
	switch n.Role {
	case role.Window, role.Dialog:
		if n.Resizable {
			// A window created with NotResizableWindowOption cannot be resized, and telling an assistive technology
			// that a fixed-size dialog can be would have it offer the user a way to do something that does nothing.
			states = append(states, StateResizable)
		}
		if n.Focused && isRoot {
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
// nil gets none of them, however text-like its role, and whatever its value: these states describe a control the user
// can move a caret and a selection about in, which the read-only text [textualValue] synthesizes from a value is not. A
// password field carries no text at all, so it has none of them either.
//
// Which of SINGLE_LINE and MULTI_LINE a node gets is [lineState]'s answer, and it can move while the window lives, so
// [attributeStateChanges] retracts the one a node has left and announces the one it has arrived at.
func textStates(n *accessibility.Node) []StateBit {
	lines, ok := lineState(n)
	if !ok {
		return nil
	}
	if n.Role == role.Document {
		// A document is readable but never editable, so it gets neither EDITABLE nor a line count claim beyond being
		// more than one line long.
		return []StateBit{lines, StateSelectableText}
	}
	states := []StateBit{lines, StateSelectableText}
	if supportsEditableText(n) {
		states = append(states, StateEditable)
	}
	return states
}

// lineState returns the state that says whether a node lays its text out over one line or several, and whether the
// node's state set holds either of them at all. It is the one place the answer is worked out, so that the state
// [textStates] puts in the set a client caches and the change [attributeStateChanges] announces cannot drift apart.
//
// Whether the control is one line or several is read from the text rather than worked out from the role, since the two
// do not line up: a single-line field that wraps lays its content out over several lines, and [accessibility.TextInfo]
// is where the widget says so. Deciding from the role alone reports such a field to an assistive technology as
// SINGLE_LINE, which has it read the content as one run and offer no line-by-line navigation through it. A text area is
// the other way about: it is multi-line however little it happens to hold at the moment, so its role decides. A
// document is always multi-line, and a node carrying no text has neither state, however text-like its role.
func lineState(n *accessibility.Node) (state StateBit, ok bool) {
	if n == nil || n.Text == nil {
		return 0, false
	}
	switch n.Role {
	case role.TextField, role.SpinButton, role.ComboBox, role.TextArea:
		if n.Text.Multiline || n.Role == role.TextArea {
			return StateMultiLine, true
		}
		return StateSingleLine, true
	case role.Document:
		return StateMultiLine, true
	default:
		return 0, false
	}
}

// Attributes returns the node's AT-SPI object attributes, which are the pieces of information that have no interface of
// their own. Every object reports the toolkit it came from; the rest appear only when they apply.
//
// container is the node's reported parent, or nil when it has none. It is what the size of the set a row belongs to is
// read from: a snapshot describes only the rows that can be seen, plus the selection, so the container is the only
// place that knows how many rows there are altogether, and "row 4 of 6" would otherwise become "row 4 of 6000".
//
// t is the tree the node came from, or nil when the caller has none. It is what the members of a set that is actually
// all there — a tab, a menu item, a radio button — are numbered from; see [appendPositionAttributes].
func Attributes(t *accessibility.Tree, n, container *accessibility.Node) dbus.Dict {
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
	return appendPositionAttributes(attributes, t, n, container)
}

// appendPositionAttributes appends the attributes that say where a node sits within the grid, the list or the run of
// siblings it belongs to. AT-SPI carries this as object attributes rather than through an interface for everything but
// a table cell, and even a cell's are worth reporting, since an assistive technology that has not asked for
// org.a11y.atspi.TableCell still reads them.
//
// The two ways a position in a set is arrived at are the two kinds of set there are. A row of a table or an item of a
// list is one of a run only part of which is described — a snapshot carries the rows that can be seen, plus the
// selection — so its own RowIndex and its container's RowCount are the only things that know where it sits and how many
// there are. A tab, a menu item and a radio button belong to a set that is all there in the tree, so
// [accessibility.Tree.PositionInSet] counts it, which is also what the other two adapters report for those three roles.
func appendPositionAttributes(attributes dbus.Dict, t *accessibility.Tree, n, container *accessibility.Node) dbus.Dict {
	if hasRowPosition(n.Role) {
		attributes = append(attributes, dbus.DictEntry{Key: rowIndexAttribute, Value: strconv.Itoa(n.RowIndex + 1)})
	}
	if hasColumnPosition(n.Role) {
		attributes = append(attributes, dbus.DictEntry{
			Key:   columnIndexAttribute,
			Value: strconv.Itoa(n.ColumnIndex + 1),
		})
	}
	switch {
	case isRowSetMember(n.Role):
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
	case isSiblingSetMember(n.Role) && t != nil:
		if position, size := t.PositionInSet(n.ID); position > 0 {
			attributes = append(attributes,
				dbus.DictEntry{Key: positionInSetAttribute, Value: strconv.Itoa(position)},
				dbus.DictEntry{Key: setSizeAttribute, Value: strconv.Itoa(size)},
			)
		}
	default:
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

// isRowSetMember returns true if a role is one of a numbered run of siblings that the snapshot describes only part of,
// which is numbered from the node's own RowIndex and its container's RowCount. A cell is not one: it is placed by its
// row and its column rather than by a position in a run.
func isRowSetMember(r role.Enum) bool {
	switch r {
	case role.Row, role.ListItem:
		return true
	default:
		return false
	}
}

// isSiblingSetMember returns true if a role is one of a numbered run of siblings that is all there in the tree, so that
// the run can simply be counted. An assistive technology says "tab 2 of 5" and "radio button 1 of 3" from these, which
// every ATK-based toolkit reports and which a Unison window would otherwise be silent about on Linux alone.
func isSiblingSetMember(r role.Enum) bool {
	switch r {
	case role.Tab, role.MenuItem, role.RadioButton:
		return true
	default:
		return false
	}
}

// layerFor returns the layer a node is in: a top-level window is in the window layer, and everything inside one is in
// the widget layer. isRoot is what says which is which, since a panel inside a window may report a window role of its
// own — a nested dialog does — while the only top-level window in a published tree is its root.
func layerFor(isRoot bool) Layer {
	if isRoot {
		return LayerWindow
	}
	return LayerWidget
}
