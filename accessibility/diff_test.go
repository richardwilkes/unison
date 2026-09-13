// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package accessibility_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// diffTree builds the three-node hierarchy most of the tests in this file start from: a window holding a group that
// holds a button. Each call produces fresh nodes, so a test can mutate one side of a comparison freely.
func diffTree() *accessibility.Tree {
	return newTree(3,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Focused: true, Children: []axID{2}},
		&axNode{ID: 2, Parent: 1, Role: role.Group, Name: groupName, Children: []axID{3}},
		&axNode{ID: 3, Parent: 2, Role: role.Button, Name: "OK", Focusable: true},
	)
}

// fieldTree builds a window holding a single text field carrying the given text and selection.
func fieldTree(text string, selStart, selEnd int) *accessibility.Tree {
	return newTree(2,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Focused: true, Children: []axID{2}},
		&axNode{
			ID: 2, Parent: 1, Role: role.TextField, Focusable: true, Focused: true,
			Text: &accessibility.TextInfo{Text: text, SelStart: selStart, SelEnd: selEnd},
		},
	)
}

func TestDiffIdenticalTreesProduceNothing(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Nil(accessibility.Diff(diffTree(), diffTree()))
	c.Nil(accessibility.Diff(fieldTree("hello", 2, 4), fieldTree("hello", 2, 4)))
}

func TestDiffFirstTree(t *testing.T) {
	t.Parallel()
	c := check.New(t)

	// With nothing to compare against, the only thing an adapter cannot read straight out of the tree is where to point
	// its assistive technology.
	c.Equal([]axEvent{{Kind: accessibility.FocusChanged, Node: 3}}, accessibility.Diff(nil, diffTree()))

	unfocused := diffTree()
	unfocused.Focus = 0
	c.Nil(accessibility.Diff(nil, unfocused))
	c.Nil(accessibility.Diff(diffTree(), nil))
	c.Nil(accessibility.Diff(nil, nil))
}

func TestDiffAddLeaf(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	cur.Node(3).Children = []axID{4}
	cur.Nodes[4] = &axNode{ID: 4, Parent: 3, Role: role.Label, Name: "Badge"}
	c.Equal([]axEvent{
		{Kind: accessibility.ChildrenChanged, Node: 3},
		{Kind: accessibility.NodeAdded, Node: 4},
	}, accessibility.Diff(old, cur))
}

func TestDiffAddSubtreeReportsParentsFirst(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	cur.Node(2).Children = []axID{3, 4}
	cur.Nodes[4] = &axNode{ID: 4, Parent: 2, Role: role.Group, Name: "Extra", Children: []axID{5}}
	cur.Nodes[5] = &axNode{ID: 5, Parent: 4, Role: role.Button, Name: "More"}
	c.Equal([]axEvent{
		{Kind: accessibility.ChildrenChanged, Node: 2},
		{Kind: accessibility.NodeAdded, Node: 4},
		{Kind: accessibility.NodeAdded, Node: 5},
	}, accessibility.Diff(old, cur))
}

func TestDiffRemoveLeaf(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	cur.Node(2).Children = nil
	delete(cur.Nodes, 3)
	cur.Focus = 0
	c.Equal([]axEvent{
		{Kind: accessibility.NodeRemoved, Node: 3},
		{Kind: accessibility.ChildrenChanged, Node: 2},
		{Kind: accessibility.FocusChanged, Node: 0},
	}, accessibility.Diff(old, cur))
}

// TestDiffRemovalsAreSorted checks the one place the algorithm could otherwise be at the mercy of Go's map iteration
// order.
func TestDiffRemovalsAreSorted(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := newTree(0,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{7, 4, 9, 2}},
		&axNode{ID: 2, Parent: 1, Role: role.Button, Name: "B"},
		&axNode{ID: 4, Parent: 1, Role: role.Button, Name: "D"},
		&axNode{ID: 7, Parent: 1, Role: role.Button, Name: "G"},
		&axNode{ID: 9, Parent: 1, Role: role.Button, Name: "I"},
	)
	cur := newTree(0, &axNode{ID: 1, Role: role.Window, Name: windowName})
	c.Equal([]axEvent{
		{Kind: accessibility.NodeRemoved, Node: 2},
		{Kind: accessibility.NodeRemoved, Node: 4},
		{Kind: accessibility.NodeRemoved, Node: 7},
		{Kind: accessibility.NodeRemoved, Node: 9},
		{Kind: accessibility.ChildrenChanged, Node: 1},
	}, accessibility.Diff(old, cur))
}

func TestDiffReorderOnlyChangesChildren(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := newTree(0,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{2, 3}},
		&axNode{ID: 2, Parent: 1, Role: role.Button, Name: "A"},
		&axNode{ID: 3, Parent: 1, Role: role.Button, Name: "B"},
	)
	cur := newTree(0,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{3, 2}},
		&axNode{ID: 2, Parent: 1, Role: role.Button, Name: "A"},
		&axNode{ID: 3, Parent: 1, Role: role.Button, Name: "B"},
	)
	c.Equal([]axEvent{{Kind: accessibility.ChildrenChanged, Node: 1}}, accessibility.Diff(old, cur))
}

func TestDiffValueEvents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	n := cur.Node(3)
	n.Name = "Cancel"
	n.Description = "Abandon the changes"
	n.Value = "off"
	n.HasNumber = true
	n.Number = 4
	n.Max = 10
	n.Sort = accessibility.SortAscending
	n.Bounds = geom.NewRect(1, 2, 3, 4)
	c.Equal([]axEvent{
		{Kind: accessibility.NameChanged, Node: 3, Old: "OK", New: "Cancel"},
		{Kind: accessibility.DescriptionChanged, Node: 3, New: "Abandon the changes"},
		{Kind: accessibility.ValueChanged, Node: 3, New: "off"},
		{Kind: accessibility.NumberChanged, Node: 3, Old: "0", New: "4"},
		{Kind: accessibility.SortChanged, Node: 3, Old: "unsorted", New: "ascending"},
		{Kind: accessibility.BoundsChanged, Node: 3},
	}, accessibility.Diff(old, cur))
}

// TestDiffNumberReportsRangeChanges covers the case where the value itself did not move but the range it sits in did,
// which changes what an assistive technology says just as much.
func TestDiffNumberReportsRangeChanges(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	old.Node(3).HasNumber = true
	old.Node(3).Number = 5
	old.Node(3).Max = 10
	cur := diffTree()
	cur.Node(3).HasNumber = true
	cur.Node(3).Number = 5
	cur.Node(3).Max = 20
	c.Equal([]axEvent{{Kind: accessibility.NumberChanged, Node: 3, Old: "5", New: "5"}},
		accessibility.Diff(old, cur))
}

func TestDiffStateEvents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	cur.Node(3).HasCheck = true
	cur.Node(3).Checked = checkenum.On
	c.Equal([]axEvent{
		{Kind: accessibility.StateChanged, Node: 3, State: accessibility.StateChecked, Old: "off", New: "on"},
	}, accessibility.Diff(old, cur))

	// The flags come out in a fixed order regardless of which of them changed.
	old = diffTree()
	cur = diffTree()
	n := cur.Node(3)
	n.Expanded = true
	n.Disabled = true
	n.Selected = true
	c.Equal([]axEvent{
		{Kind: accessibility.StateChanged, Node: 3, State: accessibility.StateDisabled, Old: "false", New: "true"},
		{Kind: accessibility.StateChanged, Node: 3, State: accessibility.StateSelected, Old: "false", New: "true"},
		{Kind: accessibility.StateChanged, Node: 3, State: accessibility.StateExpanded, Old: "false", New: "true"},
	}, accessibility.Diff(old, cur))
}

// TestDiffFocusedIsNotAState checks that moving the focus produces one FocusChanged rather than that plus a pair of
// redundant state events on the nodes that gained and lost it.
func TestDiffFocusedIsNotAState(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	old.Node(3).Focused = true
	cur := diffTree()
	cur.Node(3).Focused = false
	cur.Focus = 0
	c.Equal([]axEvent{{Kind: accessibility.FocusChanged, Node: 0}}, accessibility.Diff(old, cur))
}

func TestDiffTextReplacement(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{
		{Kind: accessibility.TextDeleted, Node: 2, Start: 3, Length: 2, Old: "lo"},
		{Kind: accessibility.TextInserted, Node: 2, Start: 3, Length: 1, New: "p"},
	}, accessibility.Diff(fieldTree("hello", 0, 0), fieldTree("help", 0, 0)))
}

func TestDiffTextInsertOnly(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{
		{Kind: accessibility.TextInserted, Node: 2, Start: 2, Length: 1, New: "y"},
		{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 3},
	}, accessibility.Diff(fieldTree("he", 2, 2), fieldTree("hey", 3, 3)))

	// Typing in the middle must be reported at the point of the edit, not as a replacement of the tail.
	c.Equal([]axEvent{{Kind: accessibility.TextInserted, Node: 2, Start: 1, Length: 1, New: "X"}},
		accessibility.Diff(fieldTree("ab", 0, 0), fieldTree("aXb", 0, 0)))
}

func TestDiffTextDeleteAll(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{
		{Kind: accessibility.TextDeleted, Node: 2, Start: 0, Length: 3, Old: "abc"},
		{Kind: accessibility.TextSelectionChanged, Node: 2},
	}, accessibility.Diff(fieldTree("abc", 3, 3), fieldTree("", 0, 0)))
}

// TestDiffTextUsesRuneOffsets checks that offsets and lengths count runes rather than bytes, since every adapter
// converts from runes to whatever its platform counts in.
func TestDiffTextUsesRuneOffsets(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{{Kind: accessibility.TextInserted, Node: 2, Start: 2, Length: 1, New: "c"}},
		accessibility.Diff(fieldTree("äö", 0, 0), fieldTree("äöc", 0, 0)))
}

func TestDiffTextSelectionOnly(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 1, Length: 2}},
		accessibility.Diff(fieldTree("hello", 0, 0), fieldTree("hello", 1, 3)))

	// A node that has stopped carrying text, or has only just started, produces no text events at all.
	withText := fieldTree("hello", 0, 0)
	without := fieldTree("hello", 0, 0)
	without.Node(2).Text = nil
	c.Nil(accessibility.Diff(withText, without))
	c.Nil(accessibility.Diff(without, withText))
}

func TestDiffFocusComesLast(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	cur.Node(2).Name = "Renamed"
	cur.Node(3).Children = []axID{4}
	cur.Nodes[4] = &axNode{ID: 4, Parent: 3, Role: role.Label, Name: "Badge"}
	cur.Focus = 2
	events := accessibility.Diff(old, cur)
	c.Equal([]axEvent{
		{Kind: accessibility.ChildrenChanged, Node: 3},
		{Kind: accessibility.NodeAdded, Node: 4},
		{Kind: accessibility.NameChanged, Node: 2, Old: groupName, New: "Renamed"},
		{Kind: accessibility.FocusChanged, Node: 2},
	}, events)
}

func TestDiffWindowActivation(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	active := diffTree()
	inactive := diffTree()
	inactive.Node(1).Focused = false
	c.Equal([]axEvent{{Kind: accessibility.WindowDeactivated, Node: 1}}, accessibility.Diff(active, inactive))
	c.Equal([]axEvent{{Kind: accessibility.WindowActivated, Node: 1}}, accessibility.Diff(inactive, active))
}

// TestDiffIsDeterministic runs the same comparison twice over a tree with changes of every sort in it. Adapters replay
// these events against live platform objects, so an order that shifted between runs would be untestable and would make
// bugs impossible to reproduce.
func TestDiffIsDeterministic(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	build := func() (old, cur *accessibility.Tree) {
		old = newTree(3,
			&axNode{ID: 1, Role: role.Window, Name: windowName, Focused: true, Children: []axID{2, 5, 6}},
			&axNode{ID: 2, Parent: 1, Role: role.Group, Name: groupName, Children: []axID{3, 4}},
			&axNode{ID: 3, Parent: 2, Role: role.Button, Name: "OK", Focusable: true},
			&axNode{ID: 4, Parent: 2, Role: role.CheckBox, Name: "Wrap", HasCheck: true},
			&axNode{ID: 5, Parent: 1, Role: role.Label, Name: "Gone"},
			&axNode{
				ID: 6, Parent: 1, Role: role.TextField, Focusable: true,
				Text: &accessibility.TextInfo{Text: "hello", SelStart: 5, SelEnd: 5},
			},
		)
		cur = newTree(6,
			&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{2, 6}},
			&axNode{ID: 2, Parent: 1, Role: role.Group, Name: groupName, Children: []axID{3, 4, 7}},
			&axNode{
				ID: 3, Parent: 2, Role: role.Button, Name: "Okay", Focusable: true, Disabled: true,
				Bounds: geom.NewRect(0, 0, 10, 10),
			},
			&axNode{ID: 4, Parent: 2, Role: role.CheckBox, Name: "Wrap", HasCheck: true, Checked: checkenum.On},
			&axNode{
				ID: 6, Parent: 1, Role: role.TextField, Focusable: true, Focused: true,
				Text: &accessibility.TextInfo{Text: "help", SelStart: 4, SelEnd: 4},
			},
			&axNode{ID: 7, Parent: 2, Role: role.Label, Name: "New"},
		)
		return old, cur
	}
	firstOld, firstCur := build()
	first := accessibility.Diff(firstOld, firstCur)
	c.True(len(first) > 8, "the fixture must exercise every group of events")
	for i := 0; i < 10; i++ {
		nextOld, nextCur := build()
		c.Equal(first, accessibility.Diff(nextOld, nextCur), "run %d", i)
	}

	// Diffing must not disturb either tree, since both are shared with whatever is reading them.
	again := accessibility.Diff(firstOld, firstCur)
	c.Equal(first, again)
}
