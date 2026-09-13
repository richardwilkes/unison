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
	"slices"
	"strconv"
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
//     DescriptionChanged, ValueChanged, NumberChanged, SortChanged, BoundsChanged, one StateChanged per changed flag,
//     then the text events — TextDeleted and TextInserted for the one run of runes that differs, followed by
//     TextSelectionChanged. RoleChanged comes first because a node's role decides how an adapter interprets everything
//     else about it, so the adapter must know the new role before it applies the rest.
//  6. WindowActivated or WindowDeactivated when the root's Focused flipped.
//  7. FocusChanged last, whenever the focus moved, so an adapter has already applied every structural and value change
//     before it tells its assistive technology where to look.
//
// Announcement events are never produced here; they come from an explicit request to speak something.
//
// Additions and per-node changes are found by walking cur from its root, so a node that is in the Nodes map but not
// reachable from Root is not reported. Removals, by contrast, are found from the Nodes maps alone, so nothing an
// adapter was told about can be left behind.
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
	if prev.HasNumber != cur.HasNumber || prev.Number != cur.Number || prev.Min != cur.Min || prev.Max != cur.Max {
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
	if prev.Bounds != cur.Bounds {
		events = append(events, Event{Kind: BoundsChanged, Node: cur.ID})
	}
	events = appendStateChanges(events, prev, cur)
	return appendTextChanges(events, prev, cur)
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

// appendTextChanges appends the text events for a node, which are produced only when both snapshots carry text. The
// edit is reduced to the single run of runes that differs by stripping the common prefix and suffix, which turns
// ordinary typing and deleting into the one insertion or deletion an assistive technology expects to hear about rather
// than a wholesale replacement.
func appendTextChanges(events []Event, prev, cur *Node) []Event {
	if prev.Text == nil || cur.Text == nil {
		return events
	}
	oldRunes := []rune(prev.Text.Text)
	curRunes := []rune(cur.Text.Text)
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
	if prev.Text.SelStart != cur.Text.SelStart || prev.Text.SelEnd != cur.Text.SelEnd {
		events = append(events, Event{
			Kind:   TextSelectionChanged,
			Node:   cur.ID,
			Start:  cur.Text.SelStart,
			Length: cur.Text.SelEnd - cur.Text.SelStart,
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
