// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"slices"
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// This file turns an accessibility snapshot into the answers UI Automation wants. Everything here is a pure function of
// the snapshot: nothing touches the OS, allocates COM memory or depends on which thread it runs on, which is why the
// file carries no build constraint and its tests run on any platform. The Windows-only files translate these answers
// into VARIANTs, SAFEARRAYs and Raise* calls, and nothing more.

// PatternSet is the set of UI Automation control patterns one element supports, held as a bit per pattern. A provider's
// QueryInterface and GetPatternProvider must agree about this set — a client that obtains a pattern interface one way
// and not the other treats the element as broken — so both consult Patterns rather than deciding for themselves.
type PatternSet uint32

// The patterns this package implements. There is deliberately no bit for a pattern the provider does not implement: a
// missing bit and an unimplemented pattern must be the same thing.
const (
	PatternInvoke PatternSet = 1 << iota
	PatternToggle
	PatternValue
	PatternRangeValue
	PatternSelection
	PatternSelectionItem
	PatternExpandCollapse
	PatternScrollItem
	PatternGrid
	PatternGridItem
	PatternTable
	PatternTableItem
	PatternWindow
	PatternText
	PatternText2
	PatternTextChild
)

// patternInfo pairs one PatternSet bit with the UI Automation identifier clients ask for it by, the property that
// reports whether the pattern is available on an element, and a name for diagnostics.
type patternInfo struct {
	name      string
	id        PatternID
	available PropertyID
	pattern   PatternSet
}

// patternInfos lists every pattern this package implements, in bit order. Every entry carries an availability
// property, since every pattern that can appear on an element can be taken away from it again; see
// decider.patternAvailability.
var patternInfos = []patternInfo{
	{name: "invoke", id: InvokePatternId, available: IsInvokePatternAvailablePropertyId, pattern: PatternInvoke},
	{name: "toggle", id: TogglePatternId, available: IsTogglePatternAvailablePropertyId, pattern: PatternToggle},
	{name: "value", id: ValuePatternId, available: IsValuePatternAvailablePropertyId, pattern: PatternValue},
	{
		name: "range-value", id: RangeValuePatternId, available: IsRangeValuePatternAvailablePropertyId,
		pattern: PatternRangeValue,
	},
	{
		name: "selection", id: SelectionPatternId, available: IsSelectionPatternAvailablePropertyId,
		pattern: PatternSelection,
	},
	{
		name: "selection-item", id: SelectionItemPatternId,
		available: IsSelectionItemPatternAvailablePropertyId, pattern: PatternSelectionItem,
	},
	{
		name: "expand-collapse", id: ExpandCollapsePatternId,
		available: IsExpandCollapsePatternAvailablePropertyId, pattern: PatternExpandCollapse,
	},
	{
		name: "scroll-item", id: ScrollItemPatternId, available: IsScrollItemPatternAvailablePropertyId,
		pattern: PatternScrollItem,
	},
	{name: "grid", id: GridPatternId, available: IsGridPatternAvailablePropertyId, pattern: PatternGrid},
	{
		name: "grid-item", id: GridItemPatternId, available: IsGridItemPatternAvailablePropertyId,
		pattern: PatternGridItem,
	},
	{name: "table", id: TablePatternId, available: IsTablePatternAvailablePropertyId, pattern: PatternTable},
	{
		name: "table-item", id: TableItemPatternId, available: IsTableItemPatternAvailablePropertyId,
		pattern: PatternTableItem,
	},
	{name: "window", id: WindowPatternId, available: IsWindowPatternAvailablePropertyId, pattern: PatternWindow},
	{name: "text", id: TextPatternId, available: IsTextPatternAvailablePropertyId, pattern: PatternText},
	{
		name: "text2", id: TextPattern2Id, available: IsTextPattern2AvailablePropertyId,
		pattern: PatternText2,
	},
	{
		name: "text-child", id: TextChildPatternId, available: IsTextChildPatternAvailablePropertyId,
		pattern: PatternTextChild,
	},
}

// Has returns true if the set contains every pattern in patterns. Passing more than one bit therefore asks whether all
// of them are present.
func (s PatternSet) Has(patterns PatternSet) bool {
	return s&patterns == patterns
}

// String implements fmt.Stringer, listing the patterns in the set separated by commas. An empty set yields an empty
// string.
func (s PatternSet) String() string {
	var buffer strings.Builder
	for _, info := range patternInfos {
		if s&info.pattern != 0 {
			if buffer.Len() != 0 {
				buffer.WriteString(",")
			}
			buffer.WriteString(info.name)
		}
	}
	return buffer.String()
}

// PatternSetForID returns the single-bit PatternSet a UI Automation pattern identifier names, or zero when this package
// does not implement that pattern. A provider asked for an unimplemented pattern must return a nil interface, which is
// exactly what a zero result leads it to do.
func PatternSetForID(id PatternID) PatternSet {
	for _, info := range patternInfos {
		if info.id == id {
			return info.pattern
		}
	}
	return 0
}

// PatternAvailableProperty returns the property that reports whether one pattern is available on an element, or
// zero when the argument names anything but a single pattern this package implements.
func PatternAvailableProperty(pattern PatternSet) PropertyID {
	for _, info := range patternInfos {
		if info.pattern == pattern {
			return info.available
		}
	}
	return 0
}

// AvailabilityPattern returns the pattern whose availability the given property reports, or zero when the property
// is not one of those. It is the inverse of PatternAvailableProperty.
//
// A pattern availability property is deliberately not one PropertyPattern claims for a pattern: it is answered by
// every element, and answering it false for an element without the pattern is the whole of its purpose.
func AvailabilityPattern(propertyID PropertyID) PatternSet {
	for _, info := range patternInfos {
		if info.available == propertyID {
			return info.pattern
		}
	}
	return 0
}

// PropertyPattern returns the control pattern that owns a property — the one whose interface a client reads that
// property through — or zero for a property every element answers in its own right. It is what decides whether a node
// has anything to say about a property: a property belonging to a pattern the node does not support is not its to
// report, and the answer is an empty VARIANT rather than a value worked out from fields the client can no longer reach.
//
// Only the properties this package reports appear here. UI Automation defines several more per pattern — the range's
// minimum, maximum and increments, the selection's required flag — but nothing raises those or answers them through
// GetPropertyValue, so listing them would be a claim about behavior that does not exist.
func PropertyPattern(propertyID PropertyID) PatternSet {
	switch propertyID {
	case ValueValuePropertyId, ValueIsReadOnlyPropertyId:
		return PatternValue
	case RangeValueValuePropertyId, RangeValueIsReadOnlyPropertyId:
		return PatternRangeValue
	case ToggleToggleStatePropertyId:
		return PatternToggle
	case ExpandCollapseExpandCollapseStatePropertyId:
		return PatternExpandCollapse
	case SelectionItemIsSelectedPropertyId:
		return PatternSelectionItem
	case SelectionCanSelectMultiplePropertyId:
		return PatternSelection
	case WindowIsModalPropertyId:
		return PatternWindow
	default:
		return 0
	}
}

// ProvidedPatterns returns the patterns an element actually hands interfaces out for, which is what Patterns says
// with two exceptions, both of which need the tree rather than the node:
//
//   - The Window pattern belongs to the fragment root alone. A nested node with a window-like role is a dialog-shaped
//     panel rather than a window of its own, and the provider refuses IWindowProvider for it — see Provider.supports
//     — so a set that still held the bit would have the adapter describe an element through a pattern a client cannot
//     obtain from it. A nil tree, or one that does not hold the node, cannot say the node is the root, so it is treated
//     as a nested one, exactly as ControlType treats it.
//   - The TextChild pattern belongs to an element that sits inside a document's stream, which is a fact about where the
//     node is rather than about what it is: the pattern's two methods report the document that contains the element and
//     the stretch of that document's text the element occupies, so it is handed out precisely when there is such a
//     document to point at. See textContainerFor. Gaining it costs the element the Text pattern, which Patterns grants
//     to anything carrying text of its own: the two would be two answers about the same words.
//
// Everything that decides what an element supports goes through this rather than through Patterns, which knows a
// node and not where it sits: the provider that hands the interfaces out, ReportsProperty, and the decider that
// reports a pattern appearing or vanishing.
func ProvidedPatterns(t *accessibility.Tree, n *accessibility.Node) PatternSet {
	patterns := Patterns(n)
	if patterns.Has(PatternWindow) && (t == nil || n.ID != t.Root) {
		patterns &^= PatternWindow
	}
	if textContainerFor(t, n) != 0 {
		// The two are exclusive. A block that carries its own text and sits inside a document's stream would
		// otherwise hand out both, and a client would read the same words through the document's Text pattern and
		// through the block's, with two sets of offsets that mean different things. TextChild wins: the document is
		// what the words belong to, and the pattern says which stretch of it this element is. See textInfoOf, which
		// withholds the text itself for the same reason, so that nothing here could hand out a pattern with nothing
		// behind it.
		patterns = patterns&^(PatternText|PatternText2) | PatternTextChild
	}
	return patterns
}

// ProvidesPattern reports whether an element hands out every pattern in pattern. See ProvidedPatterns.
func ProvidesPattern(t *accessibility.Tree, n *accessibility.Node, pattern PatternSet) bool {
	return ProvidedPatterns(t, n).Has(pattern)
}

// ReportsProperty reports whether a node answers the given property with a value of its own. Everything but a
// pattern's property is answered by every element; a pattern's property is answered only while the node hands that
// pattern out, the fragment-root rule for the Window pattern included, so that what a client is told and what it can
// read back through the pattern interface cannot disagree.
//
// The distinction matters because a pattern can be taken away by a change of state rather than of role — a spin button
// that becomes Protected loses its number and with it the RangeValue pattern, a row that stops being expandable loses
// ExpandCollapse, a cell whose value empties loses Value. DecideRaises reports the loss through the pattern's
// availability property and stops raising the pattern's own properties on the element, so a snapshot answered here for
// one of those is normally a snapshot that supports the pattern; this is the guard that keeps it so. The one pairing
// that still reaches it is the granting direction, where the element has gained the pattern and the previous snapshot
// has nothing to report for it: an empty VARIANT is read as "no value here", which is the truth, rather than as a value
// from a pattern that element never implemented.
func ReportsProperty(t *accessibility.Tree, n *accessibility.Node, propertyID PropertyID) bool {
	pattern := PropertyPattern(propertyID)
	return pattern == 0 || ProvidesPattern(t, n, pattern)
}

// ControlType returns the UI Automation control type to report for a node. It is the single most consequential
// mapping in the adapter: a client derives the spoken control type, the patterns it bothers looking for, and the
// element's treatment in the control and content views from this one value. A role with no reasonable equivalent
// becomes Custom, which tells the client to rely on the name and the patterns instead of on a built-in behavior.
//
// The tree is needed for one distinction. The Window control type lists IWindowProvider as a required pattern, and the
// provider hands that interface out for the fragment root alone: a nested node with a window-like role is a
// dialog-shaped panel rather than a window of its own, as Provider.supports explains. Such a panel is entitled to
// report role.Dialog — the role is documented as "a window that asks for a response before work can continue", which
// says nothing about being top-level, while role.Window alone is "a top-level window"; both comments live with the
// values in cmd/enumgen/main.go, which generates enums/role. A nested one therefore reports Pane, which requires no
// pattern, so that no element advertises a control type whose required pattern it refuses. A nil tree, or one that does
// not hold the node, cannot say the node is the root, so it is treated as a nested one.
func ControlType(t *accessibility.Tree, n *accessibility.Node) ControlTypeID {
	if n == nil {
		return CustomControlTypeId
	}
	switch n.Role {
	case role.Window, role.Dialog:
		if t != nil && n.ID == t.Root {
			return WindowControlTypeId
		}
		return PaneControlTypeId
	case role.Group, role.BlockQuote:
		// UI Automation has no control type for a quoted passage, so a block quote is a group, which is what it is: a
		// container of the blocks inside it, with the quoting itself said as a text attribute of the content.
		return GroupControlTypeId
	case role.TabPanel, role.ScrollArea:
		return PaneControlTypeId
	case role.TableHeader:
		return HeaderControlTypeId
	case role.Button, role.ToggleButton, role.DisclosureTriangle, role.ColorWell:
		return ButtonControlTypeId
	case role.CheckBox:
		return CheckBoxControlTypeId
	case role.RadioButton:
		return RadioButtonControlTypeId
	case role.Link:
		return HyperlinkControlTypeId
	case role.Label, role.Heading, role.Paragraph, role.Code:
		// Text is the control type for a run of static text, which is what each of these is: a paragraph and a code
		// block are the blocks a document is read as, and Narrator's item navigation steps onto them and speaks their
		// content.
		return TextControlTypeId
	case role.TextField, role.TextArea:
		return EditControlTypeId
	case role.SpinButton:
		return SpinnerControlTypeId
	case role.ComboBox, role.PopupButton:
		return ComboBoxControlTypeId
	case role.Slider:
		return SliderControlTypeId
	case role.ProgressBar:
		return ProgressBarControlTypeId
	case role.ScrollBar:
		return ScrollBarControlTypeId
	case role.Separator:
		return SeparatorControlTypeId
	case role.List:
		return ListControlTypeId
	case role.ListItem:
		return ListItemControlTypeId
	case role.Table, role.Tree:
		// Both report DataGrid, rather than the flat one reporting Table. The two roles describe the same widget:
		// Patterns hands out Grid, Table and Selection for either, and both build the same Row and Cell children.
		// Table[T] chooses between them from whether its model has any hierarchy, which SyncToModel recomputes on every
		// sync, so reporting different control types would have the first row that can hold children turn a widget a
		// client had been calling a table into a data grid. DataGrid is the one of the pair whose documented pattern
		// set includes Selection, which is exactly what these nodes advertise.
		return DataGridControlTypeId
	case role.Row, role.Cell:
		return DataItemControlTypeId
	case role.ColumnHeader:
		return HeaderItemControlTypeId
	case role.TabList:
		return TabControlTypeId
	case role.Tab:
		return TabItemControlTypeId
	case role.MenuBar:
		return MenuBarControlTypeId
	case role.Menu:
		return MenuControlTypeId
	case role.MenuItem:
		return MenuItemControlTypeId
	case role.Image:
		return ImageControlTypeId
	case role.Tooltip:
		return ToolTipControlTypeId
	case role.Document:
		// The document control type lists ITextProvider as a required pattern, so it is the right answer for exactly
		// the documents that have one: those carrying a composed stream in Node.Document, which rolePatterns hands
		// out the Text and Text2 patterns for. It is what puts Narrator into its document reading mode, where the text
		// is read by line, word and character through the pattern rather than element by element.
		//
		// A Document without a stream is a group. Nothing would be there for a client to read through the pattern it
		// would then be entitled to ask for, and the elements beneath it are all there is — which is what a group is.
		// The Cocoa adapter draws the same distinction: a document with a stream answers no text selectors either way,
		// since AppKit reads the blocks beneath it instead.
		if n.Document != nil {
			return DocumentControlTypeId
		}
		return GroupControlTypeId
	case role.Toolbar:
		return ToolBarControlTypeId
	default:
		return CustomControlTypeId
	}
}

// Patterns returns the patterns a node supports. The answer depends on the node's role and, for a few roles, on a
// state flag that decides whether a pattern is meaningful at all: a row is expandable only in a hierarchical table, a
// progress bar has a range only once it has a number, and a menu item toggles only when it carries a check mark.
//
// The set says which pattern interfaces exist, not which of their methods will work. A disabled button still supports
// Invoke; invoking it fails with E_ELEMENTNOTENABLED. Declaring the pattern and refusing the operation is what
// lets a client describe a control correctly while it is unusable.
//
// ScrollItem is the one pattern that comes from what the node can do rather than from what it is. Every node in a
// snapshot carries the ScrollIntoView action — a disabled one keeps it when it keeps nothing else — and the pattern's
// single method does nothing but dispatch that action, so any node offering it can be brought into view: a cell
// scrolled off to the side, a column header, a tab, a menu item, a control inside a scroll area. The other two adapters
// answer the same way, from the same action; see Component.ScrollTo in internal/atspi and AXScrollToVisible in
// internal/cocoa.
func Patterns(n *accessibility.Node) PatternSet {
	if n == nil {
		return 0
	}
	patterns := rolePatterns(n)
	if n.Actions.Has(accessibility.ScrollIntoView) {
		patterns |= PatternScrollItem
	}
	return patterns
}

// textPatterns returns the Text patterns a node that carries its own text hands out, and nothing for one that does
// not. Every role role.IsText names except Document reports its text through it — a field, a text area, a spin
// button, a combo box, a label, a heading, a paragraph, a code block, a table cell and a column header — which is
// what lets Narrator's scan mode read one by line, word and character and what gives NVDA the caret it follows
// through a field. A Document is read from the stream its blocks were composed into instead; see the Document arm of
// rolePatterns and ownsText.
//
// Both patterns or neither, for the reason a document's are: ITextProvider2 derives from ITextProvider and one
// interface answers both — see patternIfaces — so a node that handed out one and not the other would have a client
// reach the same six methods through a pattern the element says it does not support.
//
// Whether the node is inside a document's stream is not asked here, since a node cannot say where it sits.
// ProvidedPatterns takes these away again from a block a document has claimed, and textInfoOf withholds the text
// behind them, so the two answers cannot disagree.
func textPatterns(n *accessibility.Node) PatternSet {
	if ownsText(n) {
		return PatternText | PatternText2
	}
	return 0
}

// rolePatterns returns the patterns a node's role and state alone call for, which is everything but ScrollItem.
func rolePatterns(n *accessibility.Node) PatternSet {
	switch n.Role {
	case role.Window, role.Dialog:
		return PatternWindow
	case role.Button, role.ColorWell, role.Link, role.ColumnHeader:
		// A color well also reports its color through a read-only Value, and a column header sorts when invoked. A link
		// that knows where it leads reports the target the same way: UI Automation has no property for a hyperlink's
		// destination, and the Value pattern is where every client looks for one — which is what lets a screen reader
		// say where a link goes before the user follows it. It is read-only, since nothing retargets a link through
		// accessibility; see IsValueReadOnly.
		//
		// A column header that was drawn as plain text carries that text, and is the one role of the four that can:
		// the other three are not read as text at all, so textPatterns answers nothing for them.
		patterns := PatternInvoke | textPatterns(n)
		if n.Role == role.ColorWell || (n.Role == role.Link && n.URL != "") {
			patterns |= PatternValue
		}
		return patterns
	case role.ToggleButton, role.CheckBox:
		return PatternToggle
	case role.DisclosureTriangle:
		// The triangle a table puts on a container row carries the Expandable and Expanded flags, and
		// ExpandCollapseStateOf is reachable only through the ExpandCollapse pattern, so without it the state the node
		// reports and the Expand and Collapse actions it offers have no way of arriving at a client. It is gated on the
		// flag exactly as a row's and a menu item's are: a triangle that expands nothing would otherwise carry a
		// pattern whose state is permanently LeafNode. The other two adapters report the same state for it — see
		// isAccessibilityExpanded in internal/cocoa and the EXPANDABLE state in internal/atspi.
		patterns := PatternToggle
		if n.Expandable {
			patterns |= PatternExpandCollapse
		}
		return patterns
	case role.RadioButton:
		// A radio button is a selection item rather than a toggle, which is how a client knows to announce "n of m"
		// alongside the state. The numbers themselves come from PositionInSet, since the group a radio button
		// belongs to is a layout panel with no pattern to report a container through.
		return PatternSelectionItem
	case role.TextField, role.TextArea:
		// The content goes out twice over: as the Value pattern's string, which is how a client reads the whole of a
		// field at once, and as the Text pattern's text, which is how it reads the field by line, word and character
		// and follows the caret. NVDA and Narrator both want the second — arrowing through a field says nothing
		// without it — and neither is confused by the first. A Protected field carries no text at all, so it keeps
		// Value alone.
		return PatternValue | textPatterns(n)
	case role.SpinButton:
		// The range is gated the way the progress bar's is: an obscured numeric field never fills in a number, a
		// minimum, a maximum or a step, and IRangeValueProvider answering zero for all four would have a screen reader
		// read a PIN field as "0". The Value pattern stays, as it does for any other protected field, and answers with
		// the empty string ValueString gives every protected node.
		if n.HasNumber {
			return PatternValue | PatternRangeValue | textPatterns(n)
		}
		return PatternValue | textPatterns(n)
	case role.ComboBox:
		return PatternValue | PatternExpandCollapse | textPatterns(n)
	case role.PopupButton:
		// A popup button has no editable text, so its Value is read-only, but it is still the way the current choice
		// is reported.
		return PatternValue | PatternExpandCollapse
	case role.Slider, role.ScrollBar:
		// Gated on the number for the reason the spin button's and the progress bar's are: the role is public API, so a
		// panel may set it without ever filling in a number, and IRangeValueProvider answering zero for the value, the
		// minimum, the maximum and both increments would have a client read the control as "0 percent".
		if n.HasNumber {
			return PatternRangeValue
		}
		return 0
	case role.ProgressBar:
		if n.HasNumber {
			return PatternRangeValue
		}
		return 0
	case role.List, role.TabList:
		return PatternSelection
	case role.ListItem, role.Tab:
		return PatternSelectionItem
	case role.Table, role.Tree:
		return PatternGrid | PatternTable | PatternSelection
	case role.Row:
		if n.Expandable {
			return PatternSelectionItem | PatternExpandCollapse
		}
		return PatternSelectionItem
	case role.Cell:
		// A cell that holds a single widget reports that widget's state as its own value — Table.axAddRow clears such a
		// cell's name and fills in its value precisely so that a change to the widget is a change to the cell — and the
		// Value pattern is the only way a client can read it. The pattern is gated on there being a value, since a cell
		// whose name carries its content has nothing to report through it, and it is read-only: a cell offers no
		// SetValue action, which is what IsValueReadOnly answers from.
		//
		// A cell that carries text of its own reports it through the Text pattern as well, so that a review cursor can
		// read the cell by word and character rather than only hear it named. Nothing in the toolkit fills a cell's
		// text in: a table cell's words live on the Label inside it, which carries and reports them itself, so the
		// grant is here for an application that fills them in from a unison.AccessibilityInfo.Callback. A cell inside
		// a Markdown table is the one the toolkit does fill in, and it has its text claimed by the document and keeps
		// TextChild instead; see ProvidedPatterns.
		patterns := PatternGridItem | PatternTableItem | textPatterns(n)
		if n.Value != "" {
			patterns |= PatternValue
		}
		return patterns
	case role.MenuItem:
		patterns := PatternInvoke
		if n.HasCheck {
			patterns |= PatternToggle
		}
		if n.Expandable {
			patterns |= PatternExpandCollapse
		}
		return patterns
	case role.Document:
		// A document that carries a composed stream is read as text, which is the whole of what the Text pattern is
		// for: Narrator's document mode and NVDA's caret reading both work through it, and without it a document is a
		// pile of elements a person cannot read by line, word or character at all.
		//
		// Both patterns or neither. ITextProvider2 derives from ITextProvider and one interface answers both — see
		// patternIfaces — so a node that handed out one and not the other would have a client reach the same six
		// methods through a pattern the element says it does not support. Text2 adds GetCaretRange, which is how a
		// client finds the reading caret without a selection to go by.
		//
		// A Document with no stream reports neither, and no value either: the value would be the empty string as the
		// whole content of the document, instead of letting a client fall through to the elements that hold it.
		if n.Document != nil {
			return PatternText | PatternText2
		}
		return 0
	case role.Label, role.Heading, role.Paragraph, role.Code:
		// Static text, which is read the same way everywhere a screen reader reads text at all: by line, by word and
		// by character, with a review cursor that has to know where each character was drawn. The Text pattern is the
		// only way any of that reaches a Windows client — Narrator's scan mode reads an element through it, and
		// NVDA's review cursor has nothing else to work from — so a label that had only a name could be announced but
		// never explored.
		//
		// No Value pattern goes with it. The same words are already the element's Name, and a client that read both
		// would speak them twice; see NameString.
		//
		// A paragraph or a code block inside a Markdown view is a block of that document's stream and loses the
		// pattern again in ProvidedPatterns, which hands it TextChild instead. One outside a document — a widget that
		// set the role itself and filled in its own text — keeps it, which is the same answer a label gets.
		return textPatterns(n)
	default:
		// Group, TabPanel, ScrollArea, TableHeader, Image, Separator, MenuBar, Menu, Tooltip, Toolbar and BlockQuote
		// all present themselves through their properties and their children alone.
		//
		// BlockQuote is one of the blocks a document is composed of, and a client reads its text without a pattern of
		// its own: through the containing document's Text pattern, which is what a document is read by, and as the
		// Name it takes from its own content for the item navigation that steps onto elements — see NameString. It
		// carries no text of its own either, since role.IsText leaves it out.
		//
		// A Menu is the one of these that looks as though it should expand: the role belongs to the panel of an open
		// menu, which nothing ever collapses and which never reports Expandable, so ExpandCollapse would be a pattern
		// whose state is permanently LeafNode and whose two methods report success without doing anything. The menu
		// item that opened it carries the pattern instead, which is where a client looks for it.
		return 0
	}
}

// IsControlElement reports whether a node belongs to UI Automation's control view, the view a client walks when it
// wants every element a user can perceive or act on. Only the nodes the snapshot marks Ignored are left out, since
// those exist purely to lay other things out and are spliced away by the navigation helpers anyway.
func IsControlElement(n *accessibility.Node) bool {
	return n != nil && !n.Ignored
}

// IsContentElement reports whether a node belongs to UI Automation's content view, the narrower view a client walks
// when it wants the information on screen rather than the machinery presenting it. Everything in the control view is in
// the content view except:
//
//   - separators, scroll bars and tooltips, which are presentation or transient commentary rather than content;
//   - a label that names another element, because that element already reports the label's text as its own name, so
//     leaving the label in the content view makes a screen reader say it twice.
//
// Finding out whether a label names something means looking at every node's LabeledBy, so this costs a scan of the
// snapshot for label nodes and nothing at all for the rest. The scan is made once per snapshot rather than once per
// label — a client walking a form asks the property of every label there is — and is remembered for the snapshot it was
// made from; see memoizedNamesAnother.
func IsContentElement(t *accessibility.Tree, n *accessibility.Node) bool {
	if !IsControlElement(n) {
		return false
	}
	switch n.Role {
	case role.Separator, role.ScrollBar, role.Tooltip:
		return false
	case role.Label:
		return !memoizedNamesAnother(t, n.ID)
	default:
		return true
	}
}

// NameString returns the text a node answers the Name property with. It is Node.Name for everything but the pieces
// that are nothing but the text drawn in them — a paragraph, a code block, a table cell and a column header — whose
// name is their own content when the widget gave them none.
//
// Those need it because Narrator steps onto elements as well as reading text: its item navigation walks the control
// view and speaks each element's name, and a paragraph with no name at all is announced as a bare "text". The content
// is the only thing there is to say about such a block, and both other adapters say exactly that — AT-SPI reads the
// block through its Text interface and AppKit through AXValue — so nothing is invented.
//
// It is the name and not the value because the name is the one of the two a client reads for an element it has stepped
// onto. None of the four hands out a Value pattern for its text — a cell's is there only while the widget filled a
// value in — so nothing here is reachable through ValueString, which is deliberate: an element answering the same text
// as both its name and its value has Narrator speak it twice. The Text pattern such an element may also hand out is
// not a repetition of it: that is what a client reads the words *through*, character by character, rather than another
// thing for it to speak whole.
//
// A block the widget did name keeps that name. A heading folds its fragments into a name of its own, and a cell that
// holds one widget is named by the column it sits in, which is more useful than the text drawn in it. A label is not
// among them either: it always has a name, which is the text it drew unless the application overrode it.
func NameString(n *accessibility.Node) string {
	if n == nil {
		return ""
	}
	if n.Name == "" && namedByItsText(n) {
		return n.Text.Text
	}
	return n.Name
}

// namedByItsText reports whether a node's name comes from its own text, which is what NameString answers with and
// what makes an edit to that text a change of name. A block with no text carries none, and a node that already has a
// name is not renamed by what it draws.
//
// A column header is among them for the same reason, but nothing the toolkit builds by itself reaches it: TableHeader
// names every column node after its title before filling the node's text in, and it fills the text in only for a
// header whose label has some, so a header carrying text always has a name to go with it. What reaches it is an
// application that cleared the name in an AccessibilityInfo.Callback, which runs last and may empty a name the
// builder set. Such a header still draws its title, and answering with it beats telling a client stepping onto the
// column that its heading is a bare "header".
func namedByItsText(n *accessibility.Node) bool {
	if n == nil || n.Text == nil || n.Text.Text == "" {
		return false
	}
	switch n.Role {
	case role.Paragraph, role.Code, role.Cell, role.ColumnHeader:
		return true
	default:
		return false
	}
}

// HasKeyboardFocus reports whether a node answers the HasKeyboardFocus property with true. At most one element of a
// fragment ever does: the one the snapshot's Focus names, and nothing at all when the snapshot names none. A client
// handed two elements that both claim the keyboard has no way to decide which of them the user is actually on, and an
// element claiming it while reporting IsKeyboardFocusable false is a pair that cannot be made sense of at all — which
// is why the root is not the answer for an empty focus: a root is never focusable, since the builder never marks it so.
//
// The root's own Focused flag is not the answer for it either, because it means something different there: on the root
// it says the window is active, not that the window itself is where typing goes. That is also the second half of this
// property for every other element — a node holds the focus within its window whether or not that window has it, and
// only the active window's focused element really has the keyboard — so an inactive window answers false everywhere. It
// is the same answer IRawElementProviderFragmentRoot::GetFocus gives, including for an empty focus, which it reports as
// a NULL element.
func HasKeyboardFocus(t *accessibility.Tree, n *accessibility.Node) bool {
	if t == nil || n == nil || t.Focus == 0 || !rootFocused(t) {
		return false
	}
	return n.ID == t.Focus
}

// rootFocused reports whether the window a snapshot describes is the active one.
func rootFocused(t *accessibility.Tree) bool {
	if t == nil {
		return false
	}
	root := t.Node(t.Root)
	return root != nil && root.Focused
}

// HeadingLevel returns the value of the HeadingLevel property for a node. Only a heading has one; everything else
// reports HeadingLevel_None, and so does a heading whose level the snapshot never filled in. Levels beyond nine clamp
// to the ninth, since UI Automation defines no more than that.
func HeadingLevel(n *accessibility.Node) HeadingLevelID {
	if n == nil || n.Role != role.Heading || n.Level < 1 {
		return HeadingLevel_None
	}
	if n.Level > 9 {
		return HeadingLevel9
	}
	return HeadingLevel1 + HeadingLevelID(n.Level-1)
}

// Orientation returns the value of the Orientation property for a node.
func Orientation(n *accessibility.Node) OrientationType {
	if n == nil {
		return OrientationType_None
	}
	switch n.Orientation {
	case accessibility.OrientationHorizontal:
		return OrientationType_Horizontal
	case accessibility.OrientationVertical:
		return OrientationType_Vertical
	default:
		return OrientationType_None
	}
}

// ToggleStateOf returns the state an element that supports the Toggle pattern reports. A toggle button's state is
// whether it is pressed; everything else checkable reports its check state, where a mixed check becomes
// ToggleState_Indeterminate.
func ToggleStateOf(n *accessibility.Node) ToggleState {
	if n == nil {
		return ToggleState_Off
	}
	if n.Role == role.ToggleButton || !n.HasCheck {
		if n.Pressed {
			return ToggleState_On
		}
		return ToggleState_Off
	}
	switch n.Checked {
	case check.On:
		return ToggleState_On
	case check.Mixed:
		return ToggleState_Indeterminate
	default:
		return ToggleState_Off
	}
}

// ExpandCollapseStateOf returns the state an element that supports the ExpandCollapse pattern reports. A node that
// cannot be expanded at all is a leaf, which is a different answer from being collapsed: a client announces a leaf
// silently and a collapsed node as collapsed.
func ExpandCollapseStateOf(n *accessibility.Node) ExpandCollapseState {
	if n == nil || !n.Expandable {
		return ExpandCollapseState_LeafNode
	}
	if n.Expanded {
		return ExpandCollapseState_Expanded
	}
	return ExpandCollapseState_Collapsed
}

// ItemStatus returns the value of the ItemStatus property for a node, which is how a sorted column header tells a
// client which way it is sorted and how a node that is working tells one that its value is not yet meaningful. A node
// that is neither has no item status, reported as the empty string so that the provider answers VT_EMPTY.
//
// UI Automation has no enumeration for this: the property is free text, and a screen reader speaks it exactly as it is
// given, so these are translated phrases rather than the names the SortDirection enumeration and the Busy flag go by.
// The other two adapters have machine-readable answers to give for the direction — AT-SPI's sort attribute and AppKit's
// accessibilitySortDirection — and pass it along untranslated.
//
// Busy is here because ItemStatus is the only property a UI Automation client watches that can carry it. It is what an
// indeterminate progress bar has to say for itself: such a bar reports no number at all — a range whose maximum equals
// its minimum is forbidden, and a value of nought that never moves would have a screen reader announce "0 percent" for
// as long as the work takes, which is why ProgressBar.ProvideAccessibility sets Busy instead — so without this the
// element would be a progress bar with no value, no range and nothing whatever to say. AT-SPI reports the same flag as
// STATE_BUSY; see internal/atspi/mapping.go.
//
// The two are folded together rather than one winning, since a node may carry both — Node.Busy is public API and
// nothing stops a sorted column header from setting it — and a client speaks the whole property as one piece of text.
func ItemStatus(n *accessibility.Node) string {
	if n == nil {
		return ""
	}
	var status string
	switch n.Sort {
	case accessibility.SortAscending:
		status = i18n.Text("Sorted ascending")
	case accessibility.SortDescending:
		status = i18n.Text("Sorted descending")
	default:
	}
	if !n.Busy {
		return status
	}
	if status == "" {
		return i18n.Text("Busy")
	}
	return status + ", " + i18n.Text("Busy")
}

// WindowInteractionStateOf returns the value of the WindowInteractionState property for a fragment root. A window the
// snapshot reports as disabled is disabled because something modal is in front of it, which is the one distinction UI
// Automation cares about here.
func WindowInteractionStateOf(root *accessibility.Node) WindowInteractionState {
	if root != nil && root.Disabled {
		return WindowInteractionState_BlockedByModalWindow
	}
	return WindowInteractionState_ReadyForUserInteraction
}

// SmallChange returns the value of the RangeValue pattern's SmallChange property: how far one press of an arrow key
// moves the value.
func SmallChange(n *accessibility.Node) float64 {
	if n == nil {
		return 0
	}
	return n.Step
}

// LargeChange returns the value of the RangeValue pattern's LargeChange property: how far one press of a paging key
// moves the value. Ten small steps is the conventional answer for a control that does not say.
func LargeChange(n *accessibility.Node) float64 {
	if n == nil {
		return 0
	}
	return n.Step * 10
}

// RuntimeID returns the runtime identifier for a node other than a fragment root. It begins with
// AppendRuntimeId, which tells UI Automation to prepend the fragment root's own identifier, and continues with the
// two halves of the node id — runtime identifiers are arrays of 32-bit integers, and a node id is 64 bits wide.
//
// A fragment root has no runtime identifier of its own to report: it answers GetRuntimeId with a NULL array and lets UI
// Automation use the window handle.
func RuntimeID(id accessibility.NodeID) []int32 {
	return []int32{AppendRuntimeId, int32(uint32(id)), int32(uint32(id >> 32))}
}

// Navigate returns the id of the node IRawElementProviderFragment::Navigate should move to from the node with the
// given id, or zero when there is nothing in that direction.
//
// Navigation runs over the unignored tree, so the layout panels the snapshot marks Ignored are invisible here: their
// children take their place as children of the nearest unignored ancestor. The fragment root has no parent and no
// siblings, so those three directions report nothing for it, and an Ignored node — which has no provider to navigate
// from in the first place — reports nothing for either sibling direction, since it is not among the unignored children
// of its own unignored parent. Its children and its parent are still answered, from the unignored tree like everything
// else, so a caller that asks anyway is told something consistent rather than nothing.
func Navigate(t *accessibility.Tree, id accessibility.NodeID, direction NavigateDirection) accessibility.NodeID {
	if t == nil || t.Node(id) == nil {
		return 0
	}
	switch direction {
	case NavigateDirection_Parent:
		if id == t.Root {
			return 0
		}
		return t.UnignoredParent(id)
	case NavigateDirection_FirstChild:
		if children := t.UnignoredChildren(id); len(children) != 0 {
			return children[0]
		}
		return 0
	case NavigateDirection_LastChild:
		if children := t.UnignoredChildren(id); len(children) != 0 {
			return children[len(children)-1]
		}
		return 0
	case NavigateDirection_NextSibling, NavigateDirection_PreviousSibling:
		return sibling(t, id, direction == NavigateDirection_NextSibling)
	default:
		return 0
	}
}

// sibling returns the id of the node before or after the given one among the unignored children of its unignored
// parent, or zero when there is none.
func sibling(t *accessibility.Tree, id accessibility.NodeID, next bool) accessibility.NodeID {
	if id == t.Root {
		return 0
	}
	siblings := t.UnignoredChildren(t.UnignoredParent(id))
	for i, siblingID := range siblings {
		if siblingID != id {
			continue
		}
		if next {
			if i+1 < len(siblings) {
				return siblings[i+1]
			}
			return 0
		}
		if i > 0 {
			return siblings[i-1]
		}
		return 0
	}
	return 0
}

// HitTest returns the id of the node IRawElementProviderFragmentRoot::ElementProviderFromPoint should report for a
// point, or zero when the point is outside the window. pt is in the same window-local, top-left origin, logical
// coordinate space as Node.Bounds, so the caller converts from the screen coordinates UI Automation supplies first.
//
// The answer is always an unignored node, since an Ignored one has no provider: a hit on a layout panel is reported as
// a hit on the nearest unignored ancestor.
func HitTest(t *accessibility.Tree, pt geom.Point) accessibility.NodeID {
	hit := t.HitTest(pt)
	if hit == 0 {
		return 0
	}
	if n := t.Node(hit); n != nil && n.Ignored {
		return t.UnignoredParent(hit)
	}
	return hit
}

// PositionInSet returns the values of the PositionInSet and SizeOfSet properties for a node, or zero for both when
// the node is not one of a numbered set and the properties should be left unreported.
//
// Rows and list items are numbered from what the snapshot recorded, not from what is in the tree: a table exposes only
// the rows in its viewport, so counting siblings would tell the user they are on row 3 of 12 while scrolled to the
// bottom of a thousand. Everything else numbered — tabs, menu items and radio buttons — is fully present in the tree,
// so its position comes from counting siblings that share its role.
//
// A radio button is numbered because that is the "n of m" a client announces alongside the state, which is the whole
// reason it is given the SelectionItem pattern rather than the Toggle one. Counting siblings is also the only way to
// number it: its group is a layout panel the snapshot marks Ignored, so there is no container to ask, which is why
// SelectionContainer reports none for it. The Cocoa adapter answers accessibilityIndex for a radio button the same
// way.
//
// The answer is remembered for the snapshot it was worked out from, since PositionInSet and SizeOfSet are separate
// properties carrying its two halves and a client reading an element reads both; see snapshotMemo.
func PositionInSet(t *accessibility.Tree, n *accessibility.Node) (position, size int) {
	if t == nil || n == nil || n.Ignored {
		return 0, 0
	}
	return memoizedPositionInSet(t, n)
}

// positionInSet works out the answer PositionInSet gives, for a node the caller has already established is in the
// tree and not ignored.
func positionInSet(t *accessibility.Tree, n *accessibility.Node) (position, size int) {
	if n.Role.IsRowLike() {
		if count := containerRowCount(t, n); count > 0 && n.RowIndex >= 0 && n.RowIndex < count {
			return n.RowIndex + 1, count
		}
	}
	switch n.Role {
	case role.Row, role.ListItem, role.Tab, role.MenuItem, role.RadioButton:
		return t.PositionInSet(n.ID)
	default:
		return 0, 0
	}
}

// maxTreeDepth bounds how far the helpers here will walk up a tree.
const maxTreeDepth = 512

// containerRowCount returns the row count of the nearest ancestor of n that reports one, or zero when no ancestor
// does. This is the total number of rows the container holds, which is not the number the snapshot exposes whenever the
// container only publishes the rows in its viewport.
//
// The walk is bounded so that a malformed tree — one whose Parent links form a cycle — cannot spin here forever. Real
// hierarchies are far shallower than the limit.
func containerRowCount(t *accessibility.Tree, n *accessibility.Node) int {
	parent := t.Node(n.Parent)
	for depth := 0; parent != nil && depth < maxTreeDepth; depth++ {
		if parent.RowCount > 0 {
			return parent.RowCount
		}
		parent = t.Node(parent.Parent)
	}
	return 0
}

// RaiseKind says which UI Automation call a Raise stands for.
type RaiseKind uint8

// Possible RaiseKind values.
const (
	// RaiseEvent is a RaiseAutomationEvent call: raise Raise.Event on Raise.Node.
	RaiseEvent RaiseKind = iota
	// RaiseProperty is a RaiseAutomationPropertyChangedEvent call: report Raise.Property as changed on
	// Raise.Node. The new value is read from the node itself, so that the value a client asks for and the value it
	// was told about cannot disagree.
	RaiseProperty
	// RaiseStructure is a RaiseStructureChangedEvent call: report Raise.Change on Raise.Node, naming
	// Raise.Child when the change is a child being added or removed.
	RaiseStructure
	// RaiseDisconnect is not an event at all: it says that Raise.Node has left the tree, so its provider should
	// be marked stale, disconnected from UI Automation and released. It always follows any structure change that
	// reported the removal.
	RaiseDisconnect
)

// String implements fmt.Stringer.
func (k RaiseKind) String() string {
	switch k {
	case RaiseEvent:
		return "event"
	case RaiseProperty:
		return "property"
	case RaiseStructure:
		return "structure"
	case RaiseDisconnect:
		return "disconnect"
	default:
		return "RaiseKind(" + strconv.FormatUint(uint64(k), 10) + ")"
	}
}

// Raise is one thing the adapter should tell UI Automation about, as decided by DecideRaises. Which fields carry
// information depends on Kind; the rest are left at their zero values.
//
// There is nothing here for an announcement: one is never decided from a snapshot, since nothing in the tree accounts
// for it, so Window.Announce raises the notification itself.
type Raise struct {
	// Node is the element the call is made on.
	Node accessibility.NodeID
	// Child is the element a structure change is about, when the change names one.
	Child accessibility.NodeID
	// Event is the automation event to raise, for RaiseEvent.
	Event EventID
	// Property is the property to report as changed, for RaiseProperty.
	Property PropertyID
	// Change is the kind of structure change, for RaiseStructure.
	Change StructureChangeType
	// Kind says which call this is.
	Kind RaiseKind
}

// String implements fmt.Stringer, in a form meant for test failures rather than for users.
func (r Raise) String() string {
	var buffer strings.Builder
	buffer.WriteString(r.Kind.String())
	buffer.WriteString("{node:")
	buffer.WriteString(strconv.FormatUint(uint64(r.Node), 10))
	switch r.Kind {
	case RaiseEvent:
		buffer.WriteString(",event:")
		buffer.WriteString(strconv.FormatInt(int64(r.Event), 10))
	case RaiseProperty:
		buffer.WriteString(",property:")
		buffer.WriteString(strconv.FormatInt(int64(r.Property), 10))
	case RaiseStructure:
		buffer.WriteString(",change:")
		buffer.WriteString(strconv.FormatInt(int64(r.Change), 10))
		if r.Child != 0 {
			buffer.WriteString(",child:")
			buffer.WriteString(strconv.FormatUint(uint64(r.Child), 10))
		}
	default:
	}
	buffer.WriteString("}")
	return buffer.String()
}

// DecideRaises turns the events describing how cur differs from old into the UI Automation calls the adapter should
// make, in the order it should make them. It is pure: neither tree is touched, nothing outside them and the events is
// consulted, and the same inputs always produce the same output. The caller supplies the events rather than having them
// derived here, so that one diff can feed every platform's adapter.
//
// The translation is mostly one event to one call, with these rules on top:
//
//   - A focus event is raised only while the root reports itself focused. Telling a client that the focus moved inside
//     a window that is not the active one makes a screen reader jump to a window the user is not looking at.
//   - A window that becomes active re-raises the focus, since the client needs to be pointed back at whatever inside
//     the window has it.
//   - The focus moving to nothing is raised on the fragment root, since there is no element left to raise it on and a
//     client that was told nothing would go on pointing at an element that has stopped being focused. No element then
//     claims HasKeyboardFocus and GetFocus reports a NULL element, so a client that follows the event up is told the
//     window itself is where the user is.
//   - A bounding-rectangle change is raised only for the focused node and for the root. Every node's bounds change when
//     a window is resized, and a client that wanted them all would ask.
//   - A property change is raised only when the element actually supports the property. A slider reports its value
//     through the RangeValue pattern and a text field through the Value pattern, so the same ValueChanged event becomes
//     a different property on each, and becomes nothing at all on an element with neither pattern.
//   - A pattern the element has gained or lost between the two snapshots is reported through that pattern's
//     availability property, ahead of anything else about the element, and an element that has lost one reports nothing
//     at all through the pattern's own properties. Several patterns are gated on a state, so a state change takes one
//     away as readily as it grants one, and the availability property is the only one that may be raised on an element
//     without the pattern. See patternAvailability.
//   - An attributes change says that one of a group of secondary attributes and relations differs without saying which,
//     so every property the provider derives from that group is reported; see attributeProperties.
//   - When a node's children changed, the client is told once, with ChildrenInvalidated, and the individual additions
//     and removals anywhere beneath that node are dropped. A client responds to ChildrenInvalidated by reading the
//     children again, so the per-child events would only make it do the same work repeatedly — and a whole subtree
//     appearing at once would otherwise report every element in it.
//   - A removal whose parent also left the tree reports no structure change either, since there is no provider left to
//     raise it on, but it still reports the disconnect that releases the removed node's provider.
//   - Nodes the snapshot marks Ignored never appear: they have no provider, so there is nothing to raise an event on.
//     The exception is the moment the flag itself flips, which is the node joining or leaving the tree a client sees
//     while nobody's list of children changed — a scroll bar appearing as its content outgrows its view port is exactly
//     this — so the nearest unignored parent is told to read its children again.
//   - UI Automation's two text events are raised on the elements that hand out the Text pattern, which is every
//     element carrying text a client can read through ITextProvider: a document with its composed stream, a field, a
//     label, a heading, a plain cell or column header. A client answers TextChanged by reading the text again and
//     TextSelectionChanged by reading the selection, both through that pattern. An edit is reported through every
//     channel the element has — the text event, the value property for an element that also hands out Value, and the
//     name when the name is the text itself; see NameString — because which of them a client is listening to depends
//     on how it is reading the element. An element with no Text pattern, such as a paragraph whose words a document
//     owns, reports only the ones it has, and a caret move on it reports nothing at all, since it changes no property
//     a client could read back.
//   - Duplicates are dropped. One text edit arrives as a value change and as the deletion and the insertion that made
//     it, and all three ask for the same value property, so without this a client would hear about one edit three
//     times.
//   - A menu or a tooltip joining or leaving the tree raises the event UI Automation has for that, over and above the
//     structure change: those events are what tell a screen reader to enter and leave menu mode and to read a tip that
//     has just appeared. See menuOrTooltip.
//
// The first publish of a window is the one case that produces a call with no event behind it: the window announces
// itself with Window_WindowOpened, which is how a screen reader knows to read a dialog out as it appears. Diff reports
// nothing but the focus for a first publish, since there is no previous tree to compare against, so old being nil is
// what identifies that moment.
//
// Every root raises it, not only a dialog. The Window control type lists the opened and closed events as required, and
// Window.Destroy raises Window_WindowClosed for every window, so raising the opening for dialogs alone would tell a
// client tracking window lifetimes that an ordinary window it was never told about had closed.
func DecideRaises(old, cur *accessibility.Tree, events []accessibility.Event) []Raise {
	if cur == nil {
		return nil
	}
	d := &decider{
		old:         old,
		cur:         cur,
		invalidated: make(map[accessibility.NodeID]bool),
		seen:        make(map[Raise]bool),
	}
	if root := cur.Node(cur.Root); root != nil {
		d.rootFocused = root.Focused
		if old == nil {
			d.add(Raise{Kind: RaiseEvent, Node: cur.Root, Event: Window_WindowOpenedEventId})
		}
	}
	for _, event := range events {
		if target := d.invalidationTarget(event); target != 0 {
			d.invalidated[target] = true
		}
	}
	for _, event := range events {
		d.translate(event)
	}
	return d.raises
}

// decider holds the state DecideRaises threads through the translation of one batch of events.
type decider struct {
	old         *accessibility.Tree
	cur         *accessibility.Tree
	invalidated map[accessibility.NodeID]bool
	seen        map[Raise]bool
	raises      []Raise
	rootFocused bool
}

// add records one call to make, dropping it when it would duplicate one already recorded or when it names a node with
// no provider behind it.
func (d *decider) add(raise Raise) {
	if raise.Node == 0 {
		return
	}
	switch raise.Kind {
	case RaiseEvent, RaiseProperty:
		if n := d.cur.Node(raise.Node); n == nil || n.Ignored {
			return
		}
	default:
	}
	if d.seen[raise] {
		return
	}
	d.seen[raise] = true
	d.raises = append(d.raises, raise)
}

// invalidationTarget returns the node a client must be told to read the children of again because of one event, or
// zero when the event asks for no such thing. Two kinds do: a node whose list of children changed, and a node whose
// Ignored flag flipped, which joins it to or removes it from the tree a client sees without any list of children
// having changed at all.
func (d *decider) invalidationTarget(event accessibility.Event) accessibility.NodeID {
	switch {
	case event.Kind == accessibility.ChildrenChanged:
		return d.target(d.cur, event.Node)
	case event.Kind == accessibility.StateChanged && event.State == accessibility.StateIgnored:
		return d.cur.UnignoredParent(event.Node)
	default:
		return 0
	}
}

// target returns the node an event about the given id should be raised on: the node itself when it has a provider, or
// its nearest unignored ancestor when it does not.
func (d *decider) target(t *accessibility.Tree, id accessibility.NodeID) accessibility.NodeID {
	n := t.Node(id)
	if n == nil {
		return 0
	}
	if !n.Ignored {
		return id
	}
	return t.UnignoredParent(id)
}

// translate records the calls one event asks for.
func (d *decider) translate(event accessibility.Event) {
	switch event.Kind {
	case accessibility.FocusChanged:
		d.focus(event.Node)
	case accessibility.NameChanged:
		// Nothing but the name, even though the change behind it may have granted or removed the Text pattern: a label
		// whose title is cleared, or set from empty, is renamed and stops or starts carrying text at once. The
		// availability of the pattern is not raised here because the snapshot reports that flip as an attributes
		// change alongside this event — appendAttributeChanges compares whether the two snapshots carry any text at
		// all — and the attributes branch records patternAvailability before anything else. Recording it here as well
		// would be a second route to the same raise, which add would drop as a duplicate, so it would buy nothing but
		// a second reading of both snapshots' patterns for every rename in the window.
		d.property(event.Node, NamePropertyId)
	case accessibility.DescriptionChanged:
		// Both properties, because the provider answers both from Node.Description: a client that cached
		// FullDescription and was told only about HelpText would keep the stale one forever.
		d.property(event.Node, HelpTextPropertyId)
		d.property(event.Node, FullDescriptionPropertyId)
	case accessibility.ValueChanged:
		d.valueProperty(event.Node)
	case accessibility.NumberChanged:
		// A number arriving or departing is one of the changes that grants or removes the RangeValue pattern, which is
		// why the availability comes first and why a node that no longer has the pattern reports nothing else.
		d.patternAvailability(event.Node)
		if d.patterns(event.Node).Has(PatternRangeValue) {
			d.property(event.Node, RangeValueValuePropertyId)
		}
	case accessibility.StateChanged:
		d.state(event)
	case accessibility.TextInserted, accessibility.TextDeleted:
		d.textChanged(event.Node)
	case accessibility.TextSelectionChanged:
		// The Text pattern's own event, and the only thing that reports a caret move: an element without the pattern
		// has no selection a client could read, so nothing is said about it at all. Every element that hands the
		// pattern out raises it, which is how NVDA learns that the caret in a field has moved — it reads the new line,
		// word or character through ITextProvider from the selection this event sent it back to.
		if d.patterns(event.Node).Has(PatternText) {
			d.event(event.Node, Text_TextSelectionChangedEventId)
		}
	case accessibility.ChildrenChanged:
		d.add(Raise{
			Kind:   RaiseStructure,
			Node:   d.target(d.cur, event.Node),
			Change: StructureChangeType_ChildrenInvalidated,
		})
	case accessibility.NodeAdded:
		d.added(event.Node)
		d.menuOrTooltip(d.cur, event.Node, true)
	case accessibility.NodeRemoved:
		d.menuOrTooltip(d.old, event.Node, false)
		d.removed(event.Node)
	case accessibility.BoundsChanged:
		if event.Node == d.cur.Focus || event.Node == d.cur.Root {
			d.property(event.Node, BoundingRectanglePropertyId)
		}
	case accessibility.SortChanged:
		d.property(event.Node, ItemStatusPropertyId)
	case accessibility.AttributesChanged:
		d.attributes(event.Node)
	case accessibility.RoleChanged:
		// A role change is a control-type change: the control type follows the role — and, for a window-like role,
		// whether the node is the fragment root, which a role change never alters — and it is what a client derives
		// the spoken name of the element and the patterns it bothers looking for from. The
		// localized control type is not raised alongside it, since this package never answers that property — UI
		// Automation derives it from the control type, which it has just been told about.
		d.property(event.Node, ControlTypePropertyId)
	case accessibility.WindowActivated:
		d.focus(d.cur.Focus)
	default:
		// WindowDeactivated has no UI Automation equivalent: the window that became active raises the events that
		// matter.
	}
}

// menuOrTooltip records the event a menu or a tooltip joining or leaving the tree asks for. UI Automation reports both
// as events of their own rather than as anything a client could read from a property: MenuOpened and MenuClosed are
// what tell a screen reader to enter and leave menu mode, and ToolTipOpened is the only way a tip that has just
// appeared is announced at all, since nothing about it is focused and nothing names it. A structure change alone says
// none of that. t is the tree the node is in — the current one when it joined, the previous one when it left.
//
// The closing event cannot be raised on the node itself: it has left the tree, so it has no provider a client could ask
// anything of. It goes to the nearest ancestor that survived, which is the window in every layout the toolkit builds —
// an open menu's panel and the tooltip panel both hang directly off the root.
func (d *decider) menuOrTooltip(t *accessibility.Tree, id accessibility.NodeID, joined bool) {
	n := t.Node(id)
	if n == nil || n.Ignored {
		return
	}
	var opened, closed EventID
	switch n.Role {
	case role.Menu:
		opened, closed = MenuOpenedEventId, MenuClosedEventId
	case role.Tooltip:
		opened, closed = ToolTipOpenedEventId, ToolTipClosedEventId
	default:
		return
	}
	if joined {
		d.event(id, opened)
		return
	}
	d.event(d.survivor(n.Parent), closed)
}

// survivor returns the nearest ancestor of a departed node, taken from the previous tree, that the current tree still
// holds, or the fragment root when there is none. It is what an event about something that is gone is raised on.
func (d *decider) survivor(id accessibility.NodeID) accessibility.NodeID {
	if parent := d.target(d.old, id); parent != 0 && d.cur.Node(parent) != nil {
		return parent
	}
	return d.cur.Root
}

// patterns returns the patterns the node with the given id hands out as of the current snapshot, which is what decides
// whether one of that pattern's properties is worth raising on it. A pattern the node has lost is reported through
// patternAvailability instead, and every caller of this records that first.
func (d *decider) patterns(id accessibility.NodeID) PatternSet {
	return ProvidedPatterns(d.cur, d.cur.Node(id))
}

// patternAvailability records the availability of every pattern the node with the given id has gained or lost between
// the two snapshots, which is how a client holding a pattern interface is told that the pattern has appeared or gone.
//
// Several patterns are gated on a state rather than on the role alone: ExpandCollapse on Expandable, RangeValue on
// HasNumber, a menu item's Toggle on HasCheck, and a cell's Value on there being a value. A state change therefore
// takes a pattern away as readily as it grants one — a table row that stops being expandable stops having an
// expand-collapse state at all, and Table.ApplyFilter with a flat filter does that to every container row at once — and
// a client that was never told would go on announcing rows as expanded long after they had become leaves. The Linux
// adapter reports the same flips for the same reason; see the StateExpandable branch of internal/atspi/events.go.
//
// The availability property is the only thing that can carry the news. A pattern's own properties may not be raised on
// an element that does not support the pattern, which is exactly the element that has just lost it, so the loss is
// reported here and the pattern's properties are not reported at all — see constants.go, and patterns, which
// answers from the current snapshot alone. This is recorded before whatever else the event asks for, so that a client
// reads that the pattern is gone before it reads anything else about the element.
//
// A node only one of the snapshots holds is left alone: its arrival or departure is reported structurally, and a client
// that has never seen an element has nothing cached about it to correct.
func (d *decider) patternAvailability(id accessibility.NodeID) {
	old := d.old.Node(id)
	cur := d.cur.Node(id)
	if old == nil || cur == nil {
		return
	}
	changed := ProvidedPatterns(d.old, old) ^ ProvidedPatterns(d.cur, cur)
	if changed == 0 {
		return
	}
	for _, info := range patternInfos {
		if changed&info.pattern != 0 {
			d.property(id, info.available)
		}
	}
}

// event records an automation event.
func (d *decider) event(id accessibility.NodeID, eventID EventID) {
	d.add(Raise{Kind: RaiseEvent, Node: id, Event: eventID})
}

// property records a property change.
func (d *decider) property(id accessibility.NodeID, propertyID PropertyID) {
	d.add(Raise{Kind: RaiseProperty, Node: id, Property: propertyID})
}

// attributeProperties lists the properties an accessibility.AttributesChanged event reports, in the order they are
// reported. They are the ones Provider.propertyValue answers from the fields that event covers:
//
//   - HelpText, from Placeholder — the watermark of an unnamed field is announced through it, and Description, which
//     the same property also answers from, has its own event.
//   - Level, straight from Level, and HeadingLevel, which HeadingLevel derives from the same field. A client reads
//     a heading's depth from the second of those rather than from the first, so leaving it out would have one that
//     cached it announce the wrong depth for as long as the heading lives.
//   - PositionInSet and SizeOfSet, from RowIndex and the container's RowCount by way of PositionInSet.
//   - Orientation, from Orientation.
//   - LabeledBy, DescribedBy and ControllerFor, from the three relations.
//
// The event does not say which of those changed, so all of them are reported: they are element-wide properties that
// every element answers, and one with nothing to say for a property answers it empty both times, which a client reads
// as no change. The Grid and GridItem patterns' row and column properties are deliberately not among them — a client
// reads those through IGridProvider and IGridItemProvider rather than through GetPropertyValue, so a raise would carry
// an empty value on both sides — and a change to the shape of a grid arrives as the structure change that follows it.
var attributeProperties = []PropertyID{
	HelpTextPropertyId,
	LevelPropertyId,
	HeadingLevelPropertyId,
	PositionInSetPropertyId,
	SizeOfSetPropertyId,
	OrientationPropertyId,
	LabeledByPropertyId,
	DescribedByPropertyId,
	ControllerForPropertyId,
}

// attributes records the property changes an attributes change asks for. See attributeProperties for which they are
// and why they are all reported at once, and labelContent for the one property the event changes on a node other than
// the one it names.
//
// The availability of any pattern that came or went is reported first. An attributes change is the only event that
// reports a change to the action set, and the ScrollItem pattern is gated on the ScrollIntoView action alone, so this
// is the one place a client can be told that the pattern has appeared or gone.
//
// Two more properties are reported that are not element-wide, because this event is the only thing that reports the
// fields behind them:
//
//   - The control type, when the two snapshots disagree about it. See controlType.
//   - The Value pattern's value, on a node that hands the pattern out. A link's value is its URL, and an application
//     that re-points an existing link changes nothing else: the pattern is there both before and after, so the
//     availability raise says nothing, and Node.Value is untouched, so no text change is reported either. A client that
//     cached the link's value would go on announcing the old destination for the life of the window. The other elements
//     that hand out the pattern read their value from a field this event does not carry, so for them this reports a
//     value that has not changed — the same bargain the element-wide properties above are reported on.
func (d *decider) attributes(id accessibility.NodeID) {
	d.patternAvailability(id)
	d.controlType(id)
	for _, propertyID := range attributeProperties {
		d.property(id, propertyID)
	}
	if d.patterns(id).Has(PatternValue) {
		d.property(id, ValueValuePropertyId)
	}
	d.labelContent(id)
}

// controlType records the control type as changed when the two snapshots disagree about what it is.
//
// One field an attributes change carries decides a control type: a Document that gains or loses its composed stream is
// a Document with one and a Group without, since a Group is what an element a client cannot read as text has to be.
// That flip arrives as an attributes change and as nothing else, so a client that cached the control type — Narrator
// keys its document reading mode off it — would go on treating a document as a group, or a group as a document, for the
// life of the window.
//
// A node only one of the snapshots holds is left alone, for the reason patternAvailability gives.
func (d *decider) controlType(id accessibility.NodeID) {
	old := d.old.Node(id)
	cur := d.cur.Node(id)
	if old == nil || cur == nil || ControlType(d.old, old) == ControlType(d.cur, cur) {
		return
	}
	d.property(id, ControlTypePropertyId)
}

// labelContent records the content-view change that a change to one node's LabeledBy relation makes to the labels at
// the other end of it.
//
// A label that names another element is left out of the content view, since that element reports the label's text as
// its own name and a client walking the content view would otherwise read it out twice; see IsContentElement.
// Whether a label is in that view therefore depends on what every other node's LabeledBy says, so the label's own
// IsContentElement flips when some other node starts or stops naming it — and the event says only that the other node's
// attributes changed. The labels on either side of the relation are looked at here because nothing else would report
// them.
//
// Only the labels whose membership of the relation actually changed are asked about, and only a real move is reported:
// a label two elements name that loses one of them is still out of the content view, and saying otherwise would have a
// client add it to what it reads. Asking costs a scan of the snapshot's nodes per label, which is why an attributes
// change that leaves the relation alone asks about none.
func (d *decider) labelContent(id accessibility.NodeID) {
	old := d.old.Node(id)
	cur := d.cur.Node(id)
	if old == nil || cur == nil {
		return
	}
	for _, labelID := range symmetricDifference(old.LabeledBy, cur.LabeledBy) {
		if IsContentElement(d.old, d.old.Node(labelID)) != IsContentElement(d.cur, d.cur.Node(labelID)) {
			d.property(labelID, IsContentElementPropertyId)
		}
	}
}

// symmetricDifference returns the ids that appear in one of the two lists but not in the other, those from a ahead
// of those from b. Each is one node's LabeledBy list, which holds a handful of entries at most, so scanning one for
// each entry of the other costs less than building a set of either.
func symmetricDifference(a, b []accessibility.NodeID) []accessibility.NodeID {
	var ids []accessibility.NodeID
	for _, id := range a {
		if !slices.Contains(b, id) {
			ids = append(ids, id)
		}
	}
	for _, id := range b {
		if !slices.Contains(a, id) {
			ids = append(ids, id)
		}
	}
	return ids
}

// focus records a focus change, which is worth reporting only while the window itself is active.
//
// The focus moving to nothing is reported on the fragment root. A client answers a focus event by moving its cursor to
// the element the event names, so saying nothing would leave it on an element that has just stopped being focused; the
// root is the one element left to name, and a client that asks it about the focus is told there is none — GetFocus
// reports a NULL element and nothing claims HasKeyboardFocus. It reaches a real window whenever the focused panel is
// removed or gives the focus up, and when a window becomes active with nothing inside it focused. The Linux adapter
// reports the same two moments; see internal/atspi/events.go.
func (d *decider) focus(id accessibility.NodeID) {
	if !d.rootFocused {
		return
	}
	if id == 0 {
		id = d.cur.Root
	}
	d.event(id, AutomationFocusChangedEventId)
}

// textChanged records the calls an insertion or a deletion asks for. Every way a client can learn of the edit is
// used, because which of them a given one listens to depends on how it is reading the element, and an element is
// read in more than one way at once.
//
// An element that hands out the Text pattern reports UI Automation's own text event, which is what tells a client to
// read the text again through ITextProvider. Nothing else can: the pattern's text is not a property, so there is no
// property change to raise, and the event may only be raised on an element that has the pattern.
//
// The value goes out as well, and not instead. A field hands out both patterns over the same content — Value for
// reading it whole, Text for reading it by line and following the caret — and a client watching either one has to be
// told. A label hands out no Value pattern, so nothing is raised for it here; a paragraph inside a document has
// neither, since the document owns its words.
//
// The name goes out when the name is that text — a paragraph, a code block, a cell and a plain column header are
// named by their content, so an edit renames them, and a client that cached the name would otherwise go on speaking
// the old one while item navigation stepped onto the block. Either snapshot having been named by its text is enough:
// a block whose text is emptied is named by nothing afterwards, which is as much a change of name as gaining one is.
// A label's name changes through NameChanged instead, which the snapshot raises for it in its own right.
//
// The availability of the patterns is recorded first, because an edit can be the only event a node gets while the
// patterns it hands out change underneath it: a block whose words a document has just composed into its stream stops
// answering the Text pattern and answers TextChild instead, and the stream arriving is an attributes change on the
// document rather than on the block. Recording it before the rest also keeps a pattern that has just been taken away
// from being reported as gone after something was raised through it.
func (d *decider) textChanged(id accessibility.NodeID) {
	d.patternAvailability(id)
	patterns := d.patterns(id)
	if patterns.Has(PatternText) {
		d.event(id, Text_TextChangedEventId)
	}
	d.valueOf(id, patterns)
	if namedByItsText(d.old.Node(id)) || namedByItsText(d.cur.Node(id)) {
		d.property(id, NamePropertyId)
	}
}

// valueProperty records the change of whichever value property the node actually has, and nothing when it has neither.
// A node that has just lost the pattern — a cell whose value became empty — reports that loss through
// patternAvailability instead, which is recorded first.
func (d *decider) valueProperty(id accessibility.NodeID) {
	d.patternAvailability(id)
	d.valueOf(id, d.patterns(id))
}

// valueOf records the change of whichever value property a node holding the given patterns has, and nothing when it
// holds neither. The patterns are passed in rather than looked up because both callers have already worked them out,
// and because the availability change that may have taken one away has to be recorded before this either way.
func (d *decider) valueOf(id accessibility.NodeID, patterns PatternSet) {
	switch {
	case patterns.Has(PatternValue):
		d.property(id, ValueValuePropertyId)
	case patterns.Has(PatternRangeValue):
		d.property(id, RangeValueValuePropertyId)
	default:
	}
}

// state records the calls a state change asks for. A flag with no UI Automation property behind it, or one belonging
// to a pattern the current snapshot says this node does not hand out, records nothing more than the availability
// change: a state is one of the things a pattern is gated on, so the flip that silences a property here is often the
// same flip that took the pattern away, and patternAvailability is what reports that.
func (d *decider) state(event accessibility.Event) {
	n := d.cur.Node(event.Node)
	if n == nil {
		return
	}
	d.patternAvailability(event.Node)
	patterns := d.patterns(event.Node)
	switch event.State {
	case accessibility.StateDisabled:
		d.property(n.ID, IsEnabledPropertyId)
	case accessibility.StateFocusable:
		d.property(n.ID, IsKeyboardFocusablePropertyId)
	case accessibility.StateOffscreen:
		d.property(n.ID, IsOffscreenPropertyId)
	case accessibility.StateInvalid:
		d.property(n.ID, IsDataValidForFormPropertyId)
	case accessibility.StateSelected:
		if patterns.Has(PatternSelectionItem) {
			// IsSelected rather than the flag itself, so that what the event says and what
			// ISelectionItemProvider::get_IsSelected answers cannot disagree: a radio button's selected state is its
			// check state, as the StateChecked branch below already relies on.
			d.selectionItem(n, IsSelected(n))
		}
	case accessibility.StatePressed, accessibility.StateChecked:
		switch {
		case patterns.Has(PatternToggle):
			d.property(n.ID, ToggleToggleStatePropertyId)
		case patterns.Has(PatternSelectionItem):
			// A radio button has no toggle state: being checked is being the selected one of its group.
			d.selectionItem(n, n.Checked == check.On)
		default:
		}
	case accessibility.StateReadOnly:
		switch {
		case patterns.Has(PatternValue):
			d.property(n.ID, ValueIsReadOnlyPropertyId)
		case patterns.Has(PatternRangeValue):
			d.property(n.ID, RangeValueIsReadOnlyPropertyId)
		default:
		}
	case accessibility.StateExpandable, accessibility.StateExpanded:
		if patterns.Has(PatternExpandCollapse) {
			d.property(n.ID, ExpandCollapseExpandCollapseStatePropertyId)
		}
	case accessibility.StateMultiselectable:
		// The Selection pattern's CanSelectMultiple. A container that changes whether it holds more than one selection
		// at a time changes what AddToSelection and RemoveFromSelection mean for every item in it — and what each of
		// those items' own selection changes are reported as, since selectionItem asks the container — so a client that
		// was never told would keep answering from the stale one.
		if patterns.Has(PatternSelection) {
			d.property(n.ID, SelectionCanSelectMultiplePropertyId)
		}
	case accessibility.StateProtected:
		// A field that became, or stopped being, a password field changes what every client may read from it, and the
		// provider answers the property for every element, so it is reported for every element too.
		d.property(n.ID, IsPasswordPropertyId)
	case accessibility.StateModal:
		// Modality is the Window pattern's to report, and the fragment root is the only element that hands that pattern
		// out — a nested node with a window-like role is a panel rather than a window of its own, as
		// Provider.supports explains. That is not an extra condition here: ProvidedPatterns, which patterns
		// answers from, withholds the bit from every node but the root, so a dialog-shaped panel reports nothing and
		// ReportsProperty would refuse it a value even if something did. A client watches the property to know
		// whether to keep the user inside this window until it is dealt with.
		if patterns.Has(PatternWindow) {
			d.property(n.ID, WindowIsModalPropertyId)
		}
	case accessibility.StateBusy:
		// Whether a node is working is part of its item status, which is the only property a client watches that can
		// carry it; see ItemStatus. An indeterminate progress bar is the case that matters — it starts and stops
		// being busy without any other property of it changing, since it reports no number at all — and a client that
		// was never told would keep announcing the bar as idle while it worked, or as working long after it had
		// finished.
		d.property(n.ID, ItemStatusPropertyId)
	case accessibility.StateIgnored:
		// An ignored node has no provider, so there is no property to report and nothing to report it on. What has
		// happened is structural: the node joined or left the tree a client sees, and no node's list of children
		// changed to say so, since an ignored node stays in the snapshot for hit testing and coordinate clipping. The
		// nearest unignored parent is therefore told to read its children again, which is the only thing that brings a
		// client's cached hierarchy back in line with what Navigate answers. It reaches a real window often:
		// ScrollBar.ProvideAccessibility ignores a scroll bar with nothing to scroll, so one appears and disappears as
		// its content grows and shrinks. The Linux adapter reports the same flip the same way; see emitIgnoredChanged
		// in internal/atspi/events.go.
		d.add(Raise{
			Kind:   RaiseStructure,
			Node:   d.invalidationTarget(event),
			Change: StructureChangeType_ChildrenInvalidated,
		})
	default:
		// Selectable has no property a client watches for.
	}
}

// selectionItem records the calls a change to a selection item's selected state asks for. Which event that is depends
// on the container: one that allows several selections at once reports each element joining and leaving the selection,
// while one that does not reports only the element that became the selection, since that implicitly deselects the
// previous one.
//
// The container is the one the provider reports through ISelectionItemProvider::get_SelectionContainer — the nearest
// ancestor supporting the Selection pattern — rather than the immediate parent, which in a hierarchical table is
// another Row. A row is never multiselectable, so asking the parent would report a child row joining a multiple
// selection with ElementSelected, which tells the client everything else was just deselected.
func (d *decider) selectionItem(n *accessibility.Node, selected bool) {
	d.property(n.ID, SelectionItemIsSelectedPropertyId)
	container := d.cur.Node(SelectionContainer(d.cur, n.ID))
	if container != nil && container.Multiselectable {
		if selected {
			d.event(n.ID, SelectionItem_ElementAddedToSelectionEventId)
			return
		}
		d.event(n.ID, SelectionItem_ElementRemovedFromSelectionEventId)
		return
	}
	if selected {
		d.event(n.ID, SelectionItem_ElementSelectedEventId)
	}
}

// added records the structure change a new node asks for, unless something at or above its parent has already been
// reported as having all of its children invalidated.
//
// The whole chain has to be looked at rather than only the parent. A subtree arrives as one addition of its top node
// plus one of every node under it, and the invalidation the diff produces names the surviving parent the subtree hangs
// off: testing only the immediate parent would drop the top node, whose parent is that one, and then report every
// descendant individually, whose parents are the new nodes themselves.
func (d *decider) added(id accessibility.NodeID) {
	n := d.cur.Node(id)
	if n == nil || n.Ignored {
		return
	}
	if d.invalidatedAt(d.cur, n.Parent) {
		return
	}
	d.add(Raise{
		Kind:   RaiseStructure,
		Node:   id,
		Child:  id,
		Change: StructureChangeType_ChildAdded,
	})
}

// removed records the structure change a departed node asks for, followed by the disconnect that releases its provider.
// The structure change is raised on the parent, since the node itself is gone, and is skipped when something at or
// above that parent has already been reported as having all of its children invalidated, or when the parent has left
// the tree too. The chain is walked over the previous tree, which is the only one that still holds the departed node.
func (d *decider) removed(id accessibility.NodeID) {
	if n := d.old.Node(id); n != nil && !n.Ignored {
		parent := d.target(d.old, n.Parent)
		if parent != 0 && !d.invalidatedAt(d.old, n.Parent) && d.cur.Node(parent) != nil {
			d.add(Raise{
				Kind:   RaiseStructure,
				Node:   parent,
				Child:  id,
				Change: StructureChangeType_ChildRemoved,
			})
		}
	}
	d.add(Raise{Kind: RaiseDisconnect, Node: id})
}

// invalidatedAt reports whether the node with the given id, or any ancestor of it within t, has already been reported
// as having all of its children invalidated. The walk is bounded so that a malformed tree — one whose Parent links form
// a cycle — cannot spin here forever.
func (d *decider) invalidatedAt(t *accessibility.Tree, id accessibility.NodeID) bool {
	for depth := 0; id != 0 && depth < maxTreeDepth; depth++ {
		if d.invalidated[d.target(t, id)] {
			return true
		}
		n := t.Node(id)
		if n == nil {
			return false
		}
		id = n.Parent
	}
	return false
}
