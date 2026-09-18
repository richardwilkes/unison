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
// The two that describe a block of code are the pair Orca recognizes one by: it reads xml-roles and tag exactly as it
// does for a web page, where a code block is a <pre> element with the code role, and announces "code" from them. AT-SPI
// has no role for code, so these are the only thing that says what such a block is.
const (
	toolkitAttribute         = "toolkit"
	levelAttribute           = "level"
	sortAttribute            = "sort"
	placeholderTextAttribute = "placeholder-text"
	rowIndexAttribute        = "rowindex"
	columnIndexAttribute     = "colindex"
	positionInSetAttribute   = "posinset"
	setSizeAttribute         = "setsize"
	xmlRolesAttribute        = "xml-roles"
	tagAttribute             = "tag"
	// codeXMLRole and preTag are what a block of preformatted source code reports through those two attributes.
	codeXMLRole = "code"
	preTag      = "pre"
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
	case role.Paragraph, role.Code:
		// A code block is a paragraph as well. Orca finds paragraphs by their role — its P structural navigation asks
		// the Collection interface for ATSPI_ROLE_PARAGRAPH — and a code block is one of the blocks a person reading a
		// document arrows through, so giving it a role of its own would take it out of that walk; that it is code is
		// said with the xml-roles and tag attributes instead, which is how Orca recognizes one.
		return RoleParagraph
	case role.BlockQuote:
		return RoleBlockQuote
	case role.Link:
		return RoleLink
	case role.Image:
		return RoleImage
	case role.List:
		// AT-SPI has two roles here and they are not interchangeable. ATSPI_ROLE_LIST is a list of items a person
		// reads, which is what Orca's L and I structural navigation looks for and what it counts the items of; a list
		// box is a control a person picks from, which Orca presents as one and never navigates into. A Unison List is
		// the control, while the lists of a document are static panels: no focus of their own to take and no rows,
		// since a document's items are real children rather than the sampled rows a List reports.
		//
		// Whether the control can be used at this moment is no part of it. [accessibility.Node.Focusable] is false for
		// a disabled control as well as for a static panel — Panel.Focusable answers only while the panel is enabled —
		// so a greyed-out empty List would otherwise be handed to Orca as static document content rather than as the
		// list box it is and go on being presented that way until it was enabled again.
		if !n.Focusable && !n.Disabled && n.RowCount == 0 {
			return RoleList
		}
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
	RoleBlockQuote:    "block quote",
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
	RoleList:          "list",
	RoleListBox:       "list box",
	RoleListItem:      "list item",
	RoleMenu:          "menu",
	RoleMenuBar:       "menu bar",
	RoleMenuItem:      "menu item",
	RolePageTab:       "page tab",
	RolePageTabList:   "page tab list",
	RolePanel:         "panel",
	RoleParagraph:     "paragraph",
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
// a name, a role and a place on the screen, and so is Collection, which is answered from the published tree for any
// object an assistive technology cares to search from.
//
// spanTarget reports whether another node's text says that this node occupies part of it, which only the tree can
// answer; see [windowData.isSpanTarget]. A link or an image within a paragraph is one, and is handed
// org.a11y.atspi.Hyperlink so that the paragraph's hypertext can point at it and say where in the text it sits.
func Interfaces(n *accessibility.Node, spanTarget bool) []string {
	list := make([]string, 0, 12)
	list = append(list, InterfaceAccessible)
	if hasActions(n) {
		list = append(list, InterfaceAction)
	}
	list = append(list, InterfaceCollection, InterfaceComponent)
	if supportsEditableText(n) {
		list = append(list, InterfaceEditableText)
	}
	if supportsHyperlink(n, spanTarget) {
		list = append(list, InterfaceHyperlink)
	}
	if supportsHypertext(n) {
		list = append(list, InterfaceHypertext)
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

// supportsHyperlink returns true if a node is given org.a11y.atspi.Hyperlink, which says two things at once: that the
// object leads somewhere, which a link does, and that it occupies a range of another object's text, which is what a
// link or an image within a paragraph does. Both kinds answer it, since an assistive technology reaching one through
// org.a11y.atspi.Hypertext asks it where it sits and where it goes without knowing which kind it found; see
// [nodeObject.hyperlinkInterface] for what a link that is nowhere in anyone's text answers.
func supportsHyperlink(n *accessibility.Node, spanTarget bool) bool {
	return n.Role == role.Link || spanTarget
}

// supportsHypertext returns true if a node is given org.a11y.atspi.Hypertext, which is the interface that says which
// objects a stretch of text holds and where in it each one sits. Only a node whose own text says so has any: the spans
// are how a paragraph reports the links and images within it, and a node with none has nothing to point at.
func supportsHypertext(n *accessibility.Node) bool {
	return n.Text != nil && len(n.Text.Spans) != 0
}

// supportsText returns true if a node is given org.a11y.atspi.Text: either it carries navigable text of its own, or its
// value is text that has nowhere else to be reported and is handed over as a read-only text interface synthesized from
// it; see [textualValue]. This is the one place the answer is worked out, so that what [Interfaces] advertises, what
// [nodeObject.Interfaces] implements, which states [textStates] puts in the set and which text events
// [Adapter.emitEvents] sends cannot drift apart.
//
// A node that has composed its content into a document has no text interface whatever else it carries. Orca reads a
// document by walking the objects within it, moving its caret from one to the next, so a document that also handed over
// text of any kind would have everything read twice and Orca's caret would never leave the document object; see
// [textualValue] for the whole of that reasoning. Refusing it here rather than only where the stream is turned into
// text keeps the guarantee whatever a widget publishes: a Document that filled in Text as well as
// [accessibility.Node.Document] still reports no text, no MULTI_LINE and no SELECTABLE_TEXT, and its caret moves are
// never announced.
func supportsText(n *accessibility.Node) bool {
	if n == nil || n.Document != nil {
		return false
	}
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
//
// A node that has composed its content into a document is left out as well. Orca enters browse mode when the focus
// lands inside an object with a document role and then reads it by walking the objects within it, moving its caret from
// one to the next, so a document that also handed over text of any kind would have everything read twice and Orca's
// caret would never leave the document object. The whole point of [accessibility.DocumentInfo] being a type of its own
// is that an adapter has to opt into presenting a document as text, and this one does not.
func textualValue(n *accessibility.Node) string {
	if n == nil || n.HasNumber || n.Text != nil || n.Protected || n.Document != nil {
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
// password field carries no text at all, so it has none of them either, and neither does a node that has composed its
// content into a document, which is given no text interface to move a caret about in; see [supportsText].
//
// Which of SINGLE_LINE and MULTI_LINE a node gets is [lineState]'s answer, and it can move while the window lives, so
// [attributeStateChanges] retracts the one a node has left and announces the one it has arrived at.
func textStates(n *accessibility.Node) []StateBit {
	lines, ok := lineState(n)
	if !ok {
		return nil
	}
	states := []StateBit{lines, StateSelectableText}
	if n.Role == role.Document {
		// A document is readable but never editable. This is the document presented as its own text rather than as a
		// composed stream — one carrying [accessibility.Node.Document] has no text interface at all, so [lineState]
		// has already refused it — and even that one is never typed into.
		return states
	}
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
//
// A node that has composed its content into a document has neither state either, whatever else it carries: it is given
// no org.a11y.atspi.Text to lay out over lines in the first place, and a state describing text a client is never handed
// is a state it caches and never sees retracted. That leaves the role.Document case below for a document presented as
// its own text rather than as a composed stream; see [supportsText].
//
// The blocks of a document — its paragraphs, its headings, its code and its table cells — answer from their text too. A
// block is what Orca's caret navigation moves through, and it moves by line within a block that says it has several, so
// a wrapped paragraph reported as SINGLE_LINE is read out as one long run with the down arrow stepping straight over
// it.
func lineState(n *accessibility.Node) (state StateBit, ok bool) {
	if n == nil || n.Text == nil || n.Document != nil {
		return 0, false
	}
	switch n.Role {
	case role.TextField, role.SpinButton, role.ComboBox, role.TextArea:
		if n.Text.Multiline || n.Role == role.TextArea {
			return StateMultiLine, true
		}
		return StateSingleLine, true
	case role.Paragraph, role.Code, role.Cell, role.ColumnHeader, role.Heading:
		if n.Text.Multiline {
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
//
// A caller that already holds the reported children of every node asks [windowData.attributesOf] instead, which counts
// those sets from the index built with the snapshot rather than deriving each one from the tree again.
func Attributes(t *accessibility.Tree, n, container *accessibility.Node) dbus.Dict {
	var positions positionCounter
	if t != nil {
		positions = t.PositionInSet
	}
	return nodeAttributes(n, container, positions)
}

// positionCounter answers where a node sits among the siblings of its own role and how many of them there are, which is
// what the posinset and setsize attributes of a set that is all there in the tree are written from.
// [accessibility.Tree.PositionInSet] is one, and it derives that sibling list from the tree every time it is asked; a
// [windowData] hands over its own, which reads the list it indexed when the snapshot was published. A nil one is a
// caller with nothing to count from, which reports no position at all.
type positionCounter func(id accessibility.NodeID) (pos, size int)

// nodeAttributes is [Attributes] with the position counter named outright; see [positionCounter].
func nodeAttributes(n, container *accessibility.Node, positions positionCounter) dbus.Dict {
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
	if n.Role == role.Code {
		// [MapRole] reports a block of code as a paragraph, since that is the role a person arrowing through a document
		// walks and the role Orca's P navigation looks for, so these two attributes are the only thing that says the
		// paragraph is code. They are the pair Orca reads for the same thing on a web page.
		attributes = append(attributes,
			dbus.DictEntry{Key: xmlRolesAttribute, Value: codeXMLRole},
			dbus.DictEntry{Key: tagAttribute, Value: preTag},
		)
	}
	return appendPositionAttributes(attributes, n, container, positions)
}

// appendPositionAttributes appends the attributes that say where a node sits within the grid, the list or the run of
// siblings it belongs to. AT-SPI carries this as object attributes rather than through an interface for everything but
// a table cell, and even a cell's are worth reporting, since an assistive technology that has not asked for
// org.a11y.atspi.TableCell still reads them.
//
// The two ways a position in a set is arrived at are the two kinds of set there are. A row of a table or an item of a
// list is one of a run only part of which is described — a snapshot carries the rows that can be seen, plus the
// selection — so its own RowIndex and its container's RowCount are the only things that know where it sits and how many
// there are. A tab, a menu item and a radio button belong to a set that is all there in the tree, so the caller's
// [positionCounter] counts it, which is also what the other two adapters report for those three roles.
func appendPositionAttributes(attributes dbus.Dict, n, container *accessibility.Node,
	positions positionCounter,
) dbus.Dict {
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
	case isSiblingSetMember(n.Role) && positions != nil:
		if position, size := positions(n.ID); position > 0 {
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
