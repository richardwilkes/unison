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
	// Text holds the text content, caret and selection for the text roles (see role.Enum.IsText), and is nil for
	// every other role. It is also nil when Protected is true, so that a password never reaches an assistive
	// technology.
	Text *TextInfo
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
	// Lines holds the laid-out lines of Text. It is filled in only for the focused control, since the measurements it
	// needs are too expensive to take for every text control in a window on every snapshot. When it is empty, an
	// adapter must treat the whole content as one line.
	Lines []Line
	// SelStart is the rune index where the selection begins.
	SelStart int
	// SelEnd is the rune index where the selection ends, which is also where the caret sits. It equals SelStart when
	// nothing is selected.
	SelEnd int
	// Multiline reports that the control lays its content out over more than one line.
	Multiline bool
}

// Line describes one laid-out line of a TextInfo's content.
type Line struct {
	// Advances holds the horizontal offset of every rune boundary on the line, measured from the line's own left edge
	// (Bounds.X), so an adapter adds Bounds.X to get a panel-local x. It has one more entry than the line has runes:
	// entry i is the leading edge of rune Start+i, and the last entry is the trailing edge of the line's final rune.
	Advances []float32
	// Start is the rune index in TextInfo.Text where this line begins.
	Start int
	// End is the rune index in TextInfo.Text just past this line's last rune.
	End int
	// Bounds is the line's area in panel-local, top-left origin, logical units.
	Bounds geom.Rect
}

// CellKey identifies one cell of a table, for use as the stable key of a virtual child. It is comparable, so it works
// as a map key.
type CellKey struct {
	// Row is the identifier of the row the cell belongs to.
	Row tid.TID
	// Col is the zero-based column index of the cell.
	Col int
}
