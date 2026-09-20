// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility

import (
	"strconv"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// NodeID identifies one node. Ids are allocated from a single process-wide counter, so they are unique across every
// window in the process and may be used as a key without qualification. Zero is never a valid node and is used
// throughout this package to mean "no node".
type NodeID uint64

// Orientation holds the axis along which a node's value varies, for the roles where that is meaningful, such as
// Slider and ScrollBar.
type Orientation uint8

// Possible Orientation values.
const (
	OrientationNone Orientation = iota // The node has no meaningful orientation
	OrientationHorizontal
	OrientationVertical
)

// String implements fmt.Stringer.
func (o Orientation) String() string {
	switch o {
	case OrientationNone:
		return "none"
	case OrientationHorizontal:
		return "horizontal"
	case OrientationVertical:
		return "vertical"
	default:
		return "Orientation(" + strconv.FormatUint(uint64(o), 10) + ")"
	}
}

// SortDirection holds the direction a sortable node is currently sorted in.
type SortDirection uint8

// Possible SortDirection values.
const (
	SortNone SortDirection = iota // The node is not a sort key
	SortAscending
	SortDescending
)

// String implements fmt.Stringer.
func (s SortDirection) String() string {
	switch s {
	case SortNone:
		return "unsorted"
	case SortAscending:
		return "ascending"
	case SortDescending:
		return "descending"
	default:
		return "SortDirection(" + strconv.FormatUint(uint64(s), 10) + ")"
	}
}

// Node is one element within a Tree. Every field is filled in by the snapshot builder and is never modified afterwards,
// so a node may be read from any thread once its tree has been published. Only the fields that are meaningful for the
// node's Role are filled in; everything else is left at its zero value.
//
// The fields are ordered for a compact memory layout rather than by topic, since a snapshot of a busy window holds
// thousands of these.
type Node struct {
	// Text holds the text content, caret and selection. Only a node whose Role is one of the text roles (see
	// role.Enum.IsText) ever carries it, but not every such node does: it is nil when Protected is true, so that a
	// password never reaches an assistive technology, and nil on a Document, whose one stream of text is carried by
	// Document instead so that only the adapters which present a document as text ever see it. Check it for nil rather
	// than deciding from the role.
	Text *TextInfo
	// Document holds the one body of text a Document presents, which is the content of every node beneath it composed
	// into a single stream. Only a node whose Role is Document ever carries it, and only one that has composed its
	// content does: check it for nil rather than deciding from the role. The nodes beneath it describe the same content
	// as elements, so an adapter presents either the stream or those elements, never both.
	Document *DocumentInfo
	// Name is what an assistive technology announces for this node. It is the primary label, not a description.
	Name string
	// Description elaborates on Name when there is more to say, and falls back to the panel's tooltip text.
	Description string
	// Value is the node's current value in textual form, for the roles that have one.
	Value string
	// Placeholder is the prompt shown in an empty text control.
	Placeholder string
	// Shortcut is the human-readable key binding that activates this node, such as a menu item's accelerator.
	Shortcut string
	// URL is where a Link leads, exactly as the application supplied it and with nothing checked about it: a fragment
	// such as "#section", a relative path, or a token that means something only to the application are all passed
	// through as they were given, since only the application knows what following one would do. It is empty for
	// everything else, and for a link that was given no target at all.
	URL string
	// Children holds the child nodes in reading order, which is also the panel index order. Index 0 is the topmost
	// child, so it is the one a hit test at an overlapping point should choose.
	Children []NodeID
	// LabeledBy holds the nodes whose text names this one, most often a Label sitting beside a control.
	LabeledBy []NodeID
	// DescribedBy holds the nodes whose text further describes this one.
	DescribedBy []NodeID
	// Controls holds the nodes whose content or state this node governs.
	Controls []NodeID
	// ID is this node's identity within the tree. It is never zero.
	ID NodeID
	// Parent is the node that holds this one as a child, or zero for the root.
	Parent NodeID
	// Number is the current numeric value, valid only when HasNumber is true.
	Number float64
	// Min is the smallest value Number may take, valid only when HasNumber is true.
	Min float64
	// Max is the largest value Number may take, valid only when HasNumber is true.
	Max float64
	// Step is how far one increment or decrement moves Number, valid only when HasNumber is true.
	Step float64
	// Level is the one-based nesting depth, used by headings and by rows in a hierarchical table.
	Level int
	// RowIndex is the zero-based row this node occupies within its table or list.
	RowIndex int
	// ColumnIndex is the zero-based column this node occupies within its row.
	ColumnIndex int
	// RowCount is how many rows this node contains.
	RowCount int
	// ColumnCount is how many columns this node contains.
	ColumnCount int
	// Bounds is the node's whole area in window-local, top-left origin, logical units. It is not clipped by the node's
	// ancestors: a row half scrolled out of a table's view port reports its full height, and one scrolled out entirely
	// reports where it would be, with Offscreen set. An assistive technology needs the whole of an element to know how
	// far to scroll to reveal it; Tree.HitTest confines each node to its ancestors, as the drawing does.
	Bounds geom.Rect
	// Actions holds everything an assistive technology may ask this node to do.
	Actions ActionSet
	// Role is what kind of element this node is. It is never role.Auto or role.None in a published tree: the builder
	// resolves Auto to a concrete role and drops None nodes, promoting their children.
	Role role.Enum
	// Checked is the check state, valid only when HasCheck is true.
	Checked check.Enum
	// Orientation is the axis this node's value varies along, for the roles where that matters.
	Orientation Orientation
	// Sort is the direction this node is sorted in, for column headers.
	Sort SortDirection
	// Disabled reports that the node is present but cannot be used.
	Disabled bool
	// Focusable reports that the node can take the keyboard focus.
	Focusable bool
	// Focused reports that the node holds the keyboard focus within its window, whether or not that window is active.
	// On the root, it instead reports that the window itself is active.
	//
	// Exactly one node of a window other than the root reports it, which is the node Tree.Focus names, with one
	// deliberate exception: a node inside the panel that really holds the focus may report it as well, which is how a
	// document whose reading caret has been moved says which of its blocks the caret is now in. That second claim is
	// kept only on the platforms whose screen readers need it — AT-SPI carries the state per object, while UI
	// Automation and AppKit have one focused element apiece — and only while the focus panel's own node is what
	// Tree.Focus names, so a window whose focus an open menu has taken over carries none. A claim from anywhere else is
	// taken away before the tree is published.
	Focused bool
	// Selectable reports that the node can be selected within its container.
	Selectable bool
	// Selected reports that the node is currently selected within its container.
	Selected bool
	// Multiselectable reports that more than one of this node's children may be selected at once.
	Multiselectable bool
	// Pressed reports that a toggle button is in its on state.
	Pressed bool
	// ReadOnly reports that the node's value can be read but not changed.
	ReadOnly bool
	// Modal reports that the node must be dealt with before anything outside it can be used.
	Modal bool
	// Resizable reports that a window or dialog node can be resized by the user. It is meaningful only on a root node.
	Resizable bool
	// Floating reports that a window node stays in front of the ordinary windows. It is meaningful only on a root node.
	Floating bool
	// Busy reports that the node is working and its value is not yet meaningful.
	Busy bool
	// Invalid reports that the node's current value has been rejected.
	Invalid bool
	// Offscreen reports that the node is part of the hierarchy but scrolled or clipped out of view entirely. A node
	// that is partly visible is not Offscreen.
	Offscreen bool
	// Expandable reports that the node can be expanded to reveal more.
	Expandable bool
	// Expanded reports that an expandable node is currently expanded.
	Expanded bool
	// Ignored reports that the node carries no information of its own and should be skipped, with its children taking
	// its place. Ignored nodes stay in the tree so that hit testing and coordinate clipping still work; see
	// Tree.UnignoredChildren and Tree.UnignoredParent.
	Ignored bool
	// Protected reports that the node is a password field. Value and Text are never filled in for such a node.
	Protected bool
	// HasCheck reports that Checked is meaningful.
	HasCheck bool
	// HasNumber reports that Number, Min, Max and Step are meaningful.
	HasNumber bool
}

// TextInfo holds the text content of a node whose Role reports text, along with where the caret and selection sit
// within it. All offsets are rune indexes into Text.
type TextInfo struct {
	// Text is the full content of the control.
	Text string
	// Lines holds the laid-out lines of Text. Every control that carries text fills it in, whether or not it holds the
	// focus, since a screen reader reads by line, by word and by character wherever its own cursor is: a field caches
	// the rune boundaries of each line beside the wrapped lines they were measured from, a document's blocks were
	// measured by the layout that drew them, and a piece of static text reports the one line it was drawn on. When it
	// is empty, an adapter must treat the whole content as one line.
	Lines []Line
	// Runs holds the styled runs of Text, in ascending order and tiling it end to end, so that run i ends where run i+1
	// begins. It is empty for a control that draws the whole of its content in one style, which an adapter reads as one
	// unstyled run covering everything.
	Runs []TextRun
	// Spans holds the nodes that occupy part of Text, which is how a document says where the links, images and blocks
	// within its stream are. They are ordered outermost first and then by Start, so a span is always preceded by the
	// spans that contain it. It is empty for a control whose content is nothing but its own text.
	Spans []TextSpan
	// SelStart is the rune index where the selection begins. It is never greater than SelEnd: a widget that keeps its
	// selection as an anchor and a caret orders the pair before filling these in, and reports which end the caret is
	// at through Caret.
	SelStart int
	// SelEnd is the rune index where the selection ends. It equals SelStart when nothing is selected.
	SelEnd int
	// Caret is the rune index of the caret. It is always one of SelStart and SelEnd: the end that moves when the
	// selection is extended, which after a backward selection (shift+Left, shift+Home) is SelStart. When nothing is
	// selected all three are equal.
	Caret int
	// Multiline reports that the control lays its content out over more than one line. It says what the control
	// actually drew rather than what kind of control it is, so a single-line field that wraps sets it and stops
	// setting it again as its text and its width change. [Diff] reports a flip as an AttributesChanged, since what an
	// adapter makes of it is something a client caches until it is told otherwise.
	Multiline bool
}

// Line describes one laid-out line of a TextInfo's content.
type Line struct {
	// Advances holds the horizontal offset of every rune boundary on the line, measured from the line's own left edge
	// (Bounds.X), so an adapter adds Bounds.X to get a panel-local x. It has one more entry than the line has runes:
	// entry i is the leading edge of rune Start+i, and the last entry is the trailing edge of the line's final rune.
	// Do not modify this slice: a widget may hand the same slice to every snapshot it publishes, so writing into it
	// would corrupt the widget's own cache and every snapshot already handed out.
	Advances []float32
	// Start is the rune index in TextInfo.Text where this line begins.
	Start int
	// End is the rune index in TextInfo.Text just past this line's last rune.
	End int
	// Bounds is the line's area in panel-local, top-left origin, logical units.
	Bounds geom.Rect
}

// TextRun describes one run of uniformly styled runes within a TextInfo's text. The runs tile the text, so every rune
// belongs to exactly one of them, and an adapter that is asked about a range which crosses two of them reports the
// attributes they disagree on as mixed.
type TextRun struct {
	// Family is the name of the font family the run is drawn in.
	Family string
	// Start is the rune index in TextInfo.Text where this run begins.
	Start int
	// End is the rune index in TextInfo.Text just past this run's last rune.
	End int
	// Weight is the font's weight on the usual 100-to-900 scale, where 400 is regular and 700 is bold.
	Weight int
	// Size is the font's size in logical units.
	Size float32
	// Italic reports that the run is drawn in an italic or oblique face.
	Italic bool
	// Underline reports that the run is underlined.
	Underline bool
	// Strikethrough reports that the run has a line struck through it.
	Strikethrough bool
	// Monospace reports that the run is drawn in a fixed-pitch face, which is what marks code out from prose.
	Monospace bool
}

// TextSpan says that another node occupies a range of a TextInfo's text: a link or an image within a document's stream,
// a block of it, or a container holding several blocks. It is what lets an adapter answer both questions an assistive
// technology asks about a document — which element is at this offset, and which part of the text is this element — from
// the one stream.
type TextSpan struct {
	// Node is the node occupying the range. An image occupies the single U+FFFC that stands in for it.
	Node NodeID
	// Start is the rune index in TextInfo.Text where the node's content begins.
	Start int
	// End is the rune index in TextInfo.Text just past the node's last rune.
	End int
}

// DocumentInfo holds the one stream of text a Document presents: the content of every node beneath it, composed in
// reading order, with a span per node saying which part of the stream that node occupies. It is a type of its own
// rather than a second TextInfo on the node so that an adapter has to opt into presenting a document as text — the
// nodes beneath it describe the same content as elements, and an assistive technology told to read both would hear
// everything twice.
type DocumentInfo struct {
	// Text is the composed stream, along with its lines, runs, spans, caret and selection.
	Text TextInfo
}

// CellKey identifies one cell of a table, for use as the stable key of a virtual child. It is comparable, so it works
// as a map key.
type CellKey struct {
	// Row is the identifier of the row the cell belongs to.
	Row tid.TID
	// Col is the zero-based column index of the cell.
	Col int
}
