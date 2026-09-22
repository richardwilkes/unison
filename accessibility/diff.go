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
	"math"
	"slices"
	"strconv"

	"github.com/richardwilkes/toolbox/v2/geom"
)

// Diff returns the events that describe how cur differs from old, in the order an adapter must report them. It is pure
// and deterministic: neither tree is modified, nothing outside the two trees is consulted, and the same pair of trees
// always produces the same events in the same order. Identical trees produce no events at all.
//
// The events come back in these groups:
//
//  1. When old is nil, nothing is being replaced, so the only event is a FocusChanged naming cur's focus — and only if
//     it has one. An adapter seeing its first tree builds everything from the tree itself.
//  2. NodeRemoved for every id old held that cur does not, in ascending id order.
//  3. ChildrenChanged, in pre-order, for every surviving node whose list of children is not what it was.
//  4. NodeAdded, in pre-order, for every id cur holds that old did not. These follow the ChildrenChanged events so that
//     an adapter has already been told about the parent's new shape, and pre-order guarantees a new parent is reported
//     before the new children inside it.
//  5. For each surviving node, in pre-order, its own changes in a fixed order: RoleChanged, NameChanged,
//     DescriptionChanged, ValueChanged, NumberChanged, SortChanged, AttributesChanged, BoundsChanged, one StateChanged
//     per changed flag, then the text events — TextDeleted and TextInserted for the one run of runes that differs,
//     followed by TextSelectionChanged. RoleChanged comes first because a node's role decides how an adapter interprets
//     everything else about it, so the adapter must know the new role before it applies the rest. At most one
//     AttributesChanged is produced per node, and it covers the secondary facts no kind of its own reports: Actions,
//     Placeholder, Shortcut, URL, Level, RowIndex, ColumnIndex, RowCount, ColumnCount, Step, Orientation, the Multiline
//     flag of the node's text, whether the node presents any text at all, whether it carries a Document, which nodes
//     occupy spans of the text it presents, and the LabeledBy, DescribedBy and Controls relations. Every platform
//     carries those as attributes, relations or the set of requests an element answers rather than as its value, and an
//     assistive technology re-reads the ones it cares about when it is told the element's attributes changed, so naming
//     which of them moved would buy nothing.
//  6. WindowActivated or WindowDeactivated when the root's Focused flipped.
//  7. FocusChanged last, whenever the focus moved, so an adapter has already applied every structural and value change
//     before it tells its assistive technology where to look.
//
// Additions and per-node changes are found by walking cur from its root, so a node that is in the Nodes map but not
// reachable from Root is not reported. Removals, by contrast, are found from the Nodes maps alone, so nothing an
// adapter was told about can be left behind.
//
// The text events are produced from whatever text a node presents, which for a node carrying a DocumentInfo is the
// stream it composed out of the nodes beneath it. Whether that stream is presented to an assistive technology at all is
// the adapter's decision — one platform reads a document through the same text interface it reads a field through,
// another shows it as a container of elements and gives it no text interface — so an adapter that does not present a
// Document's stream must ignore the text events for that node rather than report a caret or an edit on an element it
// has told its client holds no text.
func Diff(old, cur *Tree) []Event {
	if cur == nil {
		return nil
	}
	if old == nil {
		if cur.Focus != 0 {
			return []Event{{Kind: FocusChanged, Node: cur.Focus}}
		}
		return nil
	}
	events := appendRemovals(nil, old, cur)
	events = appendStructureChanges(events, old, cur)
	events = appendNodeChanges(events, old, cur)
	events = appendWindowActivation(events, old, cur)
	if old.Focus != cur.Focus {
		events = append(events, Event{Kind: FocusChanged, Node: cur.Focus})
	}
	return events
}

// appendRemovals appends a NodeRemoved for every id old holds that cur does not, in ascending id order. Sorting is what
// makes this deterministic, since the ids can only be gathered by iterating a map.
func appendRemovals(events []Event, old, cur *Tree) []Event {
	// The capacity hint is the net shrinkage, which is exact whenever nodes were only removed.
	removed := make([]NodeID, 0, max(len(old.Nodes)-len(cur.Nodes), 0))
	for id := range old.Nodes {
		if _, ok := cur.Nodes[id]; !ok {
			removed = append(removed, id)
		}
	}
	slices.Sort(removed)
	for _, id := range removed {
		events = append(events, Event{Kind: NodeRemoved, Node: id})
	}
	return events
}

// appendStructureChanges appends the ChildrenChanged events for surviving nodes whose children changed, then the
// NodeAdded events for the ids cur has gained. Both passes are in pre-order over cur.
func appendStructureChanges(events []Event, old, cur *Tree) []Event {
	cur.Walk(func(n *Node) bool {
		if prev := old.Nodes[n.ID]; prev != nil && !slices.Equal(prev.Children, n.Children) {
			events = append(events, Event{Kind: ChildrenChanged, Node: n.ID})
		}
		return true
	})
	cur.Walk(func(n *Node) bool {
		if _, ok := old.Nodes[n.ID]; !ok {
			events = append(events, Event{Kind: NodeAdded, Node: n.ID})
		}
		return true
	})
	return events
}

// appendNodeChanges appends the per-node events for every node cur and old both hold, in pre-order over cur.
func appendNodeChanges(events []Event, old, cur *Tree) []Event {
	cur.Walk(func(n *Node) bool {
		if prev := old.Nodes[n.ID]; prev != nil {
			events = appendChangesForNode(events, prev, n)
		}
		return true
	})
	return events
}

// appendChangesForNode appends the events describing how cur differs from prev, which are the same node in two
// successive snapshots.
func appendChangesForNode(events []Event, prev, cur *Node) []Event {
	// A live node really can change role: a label becomes an image when its text is swapped for a drawable, a button
	// becomes a toggle button when it is made sticky, a table becomes a tree when a hierarchy appears. No adapter
	// re-derives the role on its own, so it has to be told.
	if prev.Role != cur.Role {
		events = append(events, Event{Kind: RoleChanged, Node: cur.ID, Old: prev.Role.Key(), New: cur.Role.Key()})
	}
	if prev.Name != cur.Name {
		events = append(events, Event{Kind: NameChanged, Node: cur.ID, Old: prev.Name, New: cur.Name})
	}
	if prev.Description != cur.Description {
		events = append(events, Event{
			Kind: DescriptionChanged,
			Node: cur.ID,
			Old:  prev.Description,
			New:  cur.Description,
		})
	}
	if prev.Value != cur.Value {
		events = append(events, Event{Kind: ValueChanged, Node: cur.ID, Old: prev.Value, New: cur.Value})
	}
	// A change to the range a value sits in matters as much as a change to the value itself, since an assistive
	// technology reports the two together, usually as a percentage.
	if prev.HasNumber != cur.HasNumber || numbersDiffer(prev.Number, cur.Number) ||
		numbersDiffer(prev.Min, cur.Min) || numbersDiffer(prev.Max, cur.Max) {
		events = append(events, Event{
			Kind: NumberChanged,
			Node: cur.ID,
			Old:  formatNumber(prev.Number),
			New:  formatNumber(cur.Number),
		})
	}
	if prev.Sort != cur.Sort {
		events = append(events, Event{Kind: SortChanged, Node: cur.ID, Old: prev.Sort.String(), New: cur.Sort.String()})
	}
	events = appendAttributeChanges(events, prev, cur)
	if boundsDiffer(prev.Bounds, cur.Bounds) {
		events = append(events, Event{Kind: BoundsChanged, Node: cur.ID})
	}
	events = appendStateChanges(events, prev, cur)
	return appendTextChanges(events, prev, cur)
}

// appendAttributeChanges appends the one AttributesChanged a node gets when any of the secondary facts about it moved.
// See the list in the Diff documentation for what they are and why they share a single event.
//
// Shortcut is here because an accelerator is live state: a menu item fills it in from its key binding every time it is
// described, and MenuItem.SetKeyBinding is exported, so one can be rebound while the application runs. Every adapter
// caches it as a property of the element — UI Automation re-reads AcceleratorKey only when a property-changed event
// says to — and an item that went on announcing the key it used to answer to would be telling the person to press
// something that no longer does anything. Step is here for the same reason, being what an assistive technology tells
// the person one press of an arrow key will move a slider or a spin button by.
//
// Actions is here because what a node can be asked to do is live state too, and it is the one thing about a node that
// can move with nothing else about it moving at all: List.SetAllowMultipleSelection adds AddToSelection and
// RemoveFromSelection to every row while leaving each row's name, value, bounds and every state flag exactly as they
// were. Each platform derives what it offers from the set — AT-SPI answers NActions and GetActions from it, and a node
// gaining its first action gains the org.a11y.atspi.Action interface along with it; macOS decides which accessibility
// setters an element responds to from it — so a client that cached the old set would go on offering an action that is
// now refused, or never offer one that has appeared. Which action moved is not named for the same reason none of the
// others are: an assistive technology re-reads what it cares about once it has been told to look again.
//
// URL is here because where a link leads is live state as well: a link is a Label whose target the application supplies
// when it is built, and nothing stops an application replacing one. Every adapter carries it as a property of the
// element that a client reads once and caches — AT-SPI answers GetURI from it, macOS accessibilityURL, UI Automation
// the link's value — so a link that had been re-pointed would go on telling the person where it used to go. Nothing
// else would report it: a link's target is not the node's Value, so no ValueChanged is produced for one, and the UI
// Automation adapter raises the value property from this event for a node that hands out the value pattern rather than
// leaving a client to notice on its own.
//
// Whether the node presents any text at all is here because it decides what an adapter offers on the element rather
// than what the element currently says: a node that presents text is given the text interfaces that go with it, and a
// node that presents none is presented as a plain container. Both are things a client asks about once and caches, so a
// node that has only just gained text — or one that has lost it, because its content was emptied — has to be re-read.
// What that text says is not reported here: that is what the text events are for. The text a node presents is what
// [textInfoOf] answers with, which is the node's own text when it has any and the stream a Document has composed
// otherwise.
//
// Whether the node carries a Document at all is compared as well, on top of that, because every adapter keys what it
// offers on a Document's presence independently of whatever Text the node also carries: one refuses the text interface
// outright to a node carrying a stream, another picks which stream a node hands over from its being a document, and a
// document is presented as a different kind of element from a group. A node carrying its own Text on both sides
// presents text either way, so the comparison above says nothing about it, and without this term gaining or losing a
// Document beside that Text would move what every adapter offers with nothing sent to say so.
//
// Which nodes TextInfo.Spans name is here because which other nodes occupy part of a node's text decides what an
// adapter offers on both ends of the span: the node whose text holds them is the one that can be asked which objects
// are in it and where, and each node named by one is reachable through that and answers where it sits. A node that has
// gained its first span while its text stayed exactly as it was moves both, and nothing else would report it, since
// the text events describe only an edit to the runes. Where a span sits is not compared: see [spanTargets].
//
// TextInfo.Multiline is here because how many lines a control lays its content out over is live state too, and it is
// the one fact about a wrapping field that moves with nothing else about the field moving at all: a single-line field
// reports it from the number of lines it actually drew, so it flips as the field is resized around text that already
// fits — the text, the selection and the caret all staying exactly where they were. Every adapter turns it into
// something a client caches until it is told otherwise, AT-SPI's SINGLE_LINE and MULTI_LINE states among them, so a
// field that has grown from one drawn line to two would otherwise go on being read out as a single run, with no
// line-by-line navigation through it, for the life of the window.
func appendAttributeChanges(events []Event, prev, cur *Node) []Event {
	if prev.Actions != cur.Actions ||
		prev.Placeholder != cur.Placeholder || prev.Shortcut != cur.Shortcut || prev.URL != cur.URL ||
		prev.Level != cur.Level || prev.RowIndex != cur.RowIndex || prev.ColumnIndex != cur.ColumnIndex ||
		prev.RowCount != cur.RowCount || prev.ColumnCount != cur.ColumnCount ||
		numbersDiffer(prev.Step, cur.Step) || prev.Orientation != cur.Orientation ||
		multiline(prev) != multiline(cur) || (textInfoOf(prev) == nil) != (textInfoOf(cur) == nil) ||
		(prev.Document == nil) != (cur.Document == nil) || !slices.Equal(spanTargets(prev), spanTargets(cur)) ||
		!slices.Equal(prev.LabeledBy, cur.LabeledBy) ||
		!slices.Equal(prev.DescribedBy, cur.DescribedBy) || !slices.Equal(prev.Controls, cur.Controls) {
		events = append(events, Event{Kind: AttributesChanged, Node: cur.ID})
	}
	return events
}

// multiline reports whether a node lays its text out over more than one line, which a node carrying no text never does.
//
// Answering false for a node with no text would, on its own, leave text appearing on or disappearing from a node that
// is not multiline either way looking like no attribute change at all, and an adapter that turns carrying text into a
// state of its own — AT-SPI's SINGLE_LINE, or UIA's Text pattern being available — would never be told to retract or
// grant it. That is why appendAttributeChanges also compares whether the two snapshots carry any text at all: a label
// whose title is cleared while an application-set name or a LabeledBy keeps it in the tree, and a field toggled
// between plain and Protected, both flip Node.Text between nil and non-nil with nothing else about them moving. The
// text events say nothing in either case, since they only describe an edit to text present on both sides.
func multiline(n *Node) bool {
	info := textInfoOf(n)
	return info != nil && info.Multiline
}

// textInfoOf returns the text a node presents: its own, or, for a Document, the stream it has composed from the nodes
// beneath it. A Document carries the one and never the other — see [Node.Text] — so there is no ambiguity about which
// is answered, and everything that compares one snapshot of a node's text with the next goes through here rather than
// reading the fields, since a document's text changes and moves its caret exactly as a field's does and an assistive
// technology has to be told about both the same way.
func textInfoOf(n *Node) *TextInfo {
	if n.Text != nil {
		return n.Text
	}
	if n.Document != nil {
		return &n.Document.Text
	}
	return nil
}

// spanTargets returns the nodes the spans of the text a node presents name, in span order, which is nothing at all for a
// node that presents none. It is what [appendAttributeChanges] compares, so that a node gaining or losing the objects
// sitting within its text is reported even when the runes around them did not move. Where each of them sits is left
// out on purpose: what an adapter offers on either end of a span depends only on which nodes are named, and offsets
// shift with every edit made ahead of them, which the text events already report and a client re-reads on demand.
// Comparing them would send an attribute change alongside nearly every edit to a document for nothing.
func spanTargets(n *Node) []NodeID {
	info := textInfoOf(n)
	if info == nil || len(info.Spans) == 0 {
		return nil
	}
	targets := make([]NodeID, len(info.Spans))
	for i, span := range info.Spans {
		targets[i] = span.Node
	}
	return targets
}

// appendStateChanges appends one StateChanged per flag that differs between the two snapshots of a node. The order here
// is the order the events come out in, and it matches the order the State constants are declared in.
func appendStateChanges(events []Event, prev, cur *Node) []Event {
	events = appendStateChange(events, cur.ID, StateDisabled, prev.Disabled, cur.Disabled)
	events = appendStateChange(events, cur.ID, StateFocusable, prev.Focusable, cur.Focusable)
	events = appendStateChange(events, cur.ID, StateSelectable, prev.Selectable, cur.Selectable)
	events = appendStateChange(events, cur.ID, StateSelected, prev.Selected, cur.Selected)
	events = appendStateChange(events, cur.ID, StateMultiselectable, prev.Multiselectable, cur.Multiselectable)
	events = appendStateChange(events, cur.ID, StatePressed, prev.Pressed, cur.Pressed)
	events = appendStateChange(events, cur.ID, StateReadOnly, prev.ReadOnly, cur.ReadOnly)
	events = appendStateChange(events, cur.ID, StateModal, prev.Modal, cur.Modal)
	events = appendStateChange(events, cur.ID, StateBusy, prev.Busy, cur.Busy)
	events = appendStateChange(events, cur.ID, StateInvalid, prev.Invalid, cur.Invalid)
	events = appendStateChange(events, cur.ID, StateOffscreen, prev.Offscreen, cur.Offscreen)
	events = appendStateChange(events, cur.ID, StateExpandable, prev.Expandable, cur.Expandable)
	events = appendStateChange(events, cur.ID, StateExpanded, prev.Expanded, cur.Expanded)
	events = appendStateChange(events, cur.ID, StateIgnored, prev.Ignored, cur.Ignored)
	events = appendStateChange(events, cur.ID, StateProtected, prev.Protected, cur.Protected)
	// Checked is tri-state and only meaningful while HasCheck is set, so losing or gaining checkability counts as a
	// change just as much as moving between off, on and mixed does.
	if prev.HasCheck != cur.HasCheck || prev.Checked != cur.Checked {
		events = append(events, Event{
			Kind:  StateChanged,
			Node:  cur.ID,
			State: StateChecked,
			Old:   prev.Checked.Key(),
			New:   cur.Checked.Key(),
		})
	}
	return events
}

// appendStateChange appends a StateChanged for the given state when the flag differs between the two snapshots.
func appendStateChange(events []Event, id NodeID, state State, prev, cur bool) []Event {
	if prev != cur {
		events = append(events, Event{
			Kind:  StateChanged,
			Node:  id,
			State: state,
			Old:   strconv.FormatBool(prev),
			New:   strconv.FormatBool(cur),
		})
	}
	return events
}

// appendTextChanges appends the text events for a node, which are produced only when both snapshots carry text — its
// own or, for a Document, the stream it has composed; see [textInfoOf]. The edit is reduced to the single run of runes
// that differs by stripping the common prefix and suffix, which turns ordinary typing and deleting into the one
// insertion or deletion an assistive technology expects to hear about rather than a wholesale replacement.
func appendTextChanges(events []Event, prev, cur *Node) []Event {
	prevText := textInfoOf(prev)
	curText := textInfoOf(cur)
	if prevText == nil || curText == nil {
		return events
	}
	oldRunes := []rune(prevText.Text)
	curRunes := []rune(curText.Text)
	prefix := 0
	for prefix < len(oldRunes) && prefix < len(curRunes) && oldRunes[prefix] == curRunes[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldRunes)-prefix && suffix < len(curRunes)-prefix &&
		oldRunes[len(oldRunes)-1-suffix] == curRunes[len(curRunes)-1-suffix] {
		suffix++
	}
	if deleted := len(oldRunes) - prefix - suffix; deleted > 0 {
		events = append(events, Event{
			Kind:   TextDeleted,
			Node:   cur.ID,
			Start:  prefix,
			Length: deleted,
			Old:    string(oldRunes[prefix : prefix+deleted]),
		})
	}
	if inserted := len(curRunes) - prefix - suffix; inserted > 0 {
		events = append(events, Event{
			Kind:   TextInserted,
			Node:   cur.ID,
			Start:  prefix,
			Length: inserted,
			New:    string(curRunes[prefix : prefix+inserted]),
		})
	}
	if prevText.SelStart != curText.SelStart || prevText.SelEnd != curText.SelEnd ||
		prevText.Caret != curText.Caret {
		// TextInfo requires SelStart <= SelEnd, and the pair is ordered again here so that a widget which fills the two
		// in from an anchor and a caret without ordering them cannot produce a negative Length. Length is a count of
		// runes, which adapters turn into a platform range, and there is nothing such a range could make of a negative
		// one.
		start := min(curText.SelStart, curText.SelEnd)
		events = append(events, Event{
			Kind:   TextSelectionChanged,
			Node:   cur.ID,
			Start:  start,
			Length: max(curText.SelStart, curText.SelEnd) - start,
		})
	}
	return events
}

// appendWindowActivation appends a WindowActivated or WindowDeactivated when the root's Focused flag flipped, which is
// how a window reports that it became, or stopped being, the active one.
func appendWindowActivation(events []Event, old, cur *Tree) []Event {
	prevRoot := old.Node(old.Root)
	curRoot := cur.Node(cur.Root)
	if prevRoot == nil || curRoot == nil || prevRoot.Focused == curRoot.Focused {
		return events
	}
	kind := WindowDeactivated
	if curRoot.Focused {
		kind = WindowActivated
	}
	return append(events, Event{Kind: kind, Node: cur.Root})
}

// formatNumber renders a numeric value the way every adapter reports it, with just enough digits to read back as the
// same float64.
func formatNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

// numbersDiffer reports whether two numeric values are not the same value, counting two NaNs as the same.
//
// NaN is equal to nothing at all, itself included, so comparing it with == would report a node carrying one as having
// changed every time it is described: the identical bad value against itself, forever, as often as the window is
// published. That is a stream of events an assistive technology has to be handed and can do nothing with, and it
// contradicts the promise that identical trees produce none. Two NaNs say the same thing — neither is a quantity
// anything can be said about — so nothing is reported for the pair.
func numbersDiffer[T float32 | float64](a, b T) bool {
	if a == b {
		return false
	}
	// The two are unequal, which settles it for every value but a NaN, since only a NaN is unequal to itself. The pair
	// counts as unchanged just when both of them are one.
	return !math.IsNaN(float64(a)) || !math.IsNaN(float64(b))
}

// boundsDiffer reports whether two rectangles hold different geometry, comparing each field with numbersDiffer so that
// a NaN in either is not read as a change from itself. The == that geom.Rect would otherwise be compared with is what
// makes a single NaN coordinate a permanent stream of BoundsChanged events.
func boundsDiffer(a, b geom.Rect) bool {
	return numbersDiffer(a.X, b.X) || numbersDiffer(a.Y, b.Y) || numbersDiffer(a.Width, b.Width) ||
		numbersDiffer(a.Height, b.Height)
}
