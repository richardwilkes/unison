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
	"math"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	checkenum "github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/role"
)

// The values a StateChanged event carries: "false" and "true" for an ordinary flag, and the check state's key for the
// tri-state one. Named because the test that covers every state in turn spells each of them out sixteen times.
const (
	flagOff   = "false"
	flagOn    = "true"
	checkOff  = "off"
	checkOn   = "on"
	flagsName = "Flags"
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

// documentTree builds a window holding a Markdown-shaped document: the Document node carries the composed stream rather
// than text of its own, and the paragraph beneath it carries the block-local copy of the same words.
func documentTree(text string, selStart, selEnd int) *accessibility.Tree {
	return newTree(2,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Focused: true, Children: []axID{2}},
		&axNode{
			ID: 2, Parent: 1, Role: role.Document, Focusable: true, Focused: true, Children: []axID{3},
			Document: &accessibility.DocumentInfo{
				Text: accessibility.TextInfo{
					Text:      text,
					SelStart:  selStart,
					SelEnd:    selEnd,
					Caret:     selEnd,
					Multiline: true,
					Spans:     []accessibility.TextSpan{{Node: 3, Start: 0, End: len([]rune(text))}},
				},
			},
		},
		&axNode{
			ID: 3, Parent: 2, Role: role.Paragraph, ReadOnly: true,
			Text: &accessibility.TextInfo{Text: text, SelStart: selStart, SelEnd: selEnd, Caret: selEnd},
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

// TestDiffReparentedNodeReportsBothParents pins the one structural change no event names directly: a node that survives
// but moves to a different parent. Node.Parent changing produces nothing of its own, so what an adapter has to rebuild
// its own hierarchy from is the pair of ChildrenChanged events, one for the parent the node left and one for the parent
// it joined.
func TestDiffReparentedNodeReportsBothParents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := newTree(0,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{2, 4}},
		&axNode{ID: 2, Parent: 1, Role: role.Group, Name: groupName, Children: []axID{3}},
		&axNode{ID: 3, Parent: 2, Role: role.Button, Name: "OK"},
		&axNode{ID: 4, Parent: 1, Role: role.Group, Name: "Other"},
	)
	cur := newTree(0,
		&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{2, 4}},
		&axNode{ID: 2, Parent: 1, Role: role.Group, Name: groupName},
		&axNode{ID: 3, Parent: 4, Role: role.Button, Name: "OK"},
		&axNode{ID: 4, Parent: 1, Role: role.Group, Name: "Other", Children: []axID{3}},
	)
	c.Equal([]axEvent{
		{Kind: accessibility.ChildrenChanged, Node: 2},
		{Kind: accessibility.ChildrenChanged, Node: 4},
	}, accessibility.Diff(old, cur))
}

func TestDiffValueEvents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	old := diffTree()
	cur := diffTree()
	n := cur.Node(3)
	n.Role = role.ToggleButton
	n.Name = "Cancel"
	n.Description = "Abandon the changes"
	n.Value = "off"
	n.HasNumber = true
	n.Number = 4
	n.Max = 10
	n.Sort = accessibility.SortAscending
	n.Bounds = geom.NewRect(1, 2, 3, 4)
	c.Equal([]axEvent{
		{Kind: accessibility.RoleChanged, Node: 3, Old: "button", New: "toggle-button"},
		{Kind: accessibility.NameChanged, Node: 3, Old: "OK", New: "Cancel"},
		{Kind: accessibility.DescriptionChanged, Node: 3, New: "Abandon the changes"},
		{Kind: accessibility.ValueChanged, Node: 3, New: "off"},
		{Kind: accessibility.NumberChanged, Node: 3, Old: "0", New: "4"},
		{Kind: accessibility.SortChanged, Node: 3, Old: "unsorted", New: "ascending"},
		{Kind: accessibility.BoundsChanged, Node: 3},
	}, accessibility.Diff(old, cur))
}

// TestDiffAttributeEvents covers the secondary facts that share one event. None of them has a kind of its own, and
// each is something an assistive technology re-reads when it is told an element's attributes changed, so what is
// checked here is that every one of them produces exactly one such event and that a node holding the same values
// produces none.
func TestDiffAttributeEvents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	attributed := func(fill func(n *axNode)) *accessibility.Tree {
		tree := diffTree()
		fill(tree.Node(3))
		return tree
	}
	for _, one := range []struct {
		fill func(n *axNode)
		name string
	}{
		{name: "Actions", fill: func(n *axNode) { n.Actions = n.Actions.With(accessibility.SetValue) }},
		{name: "Placeholder", fill: func(n *axNode) { n.Placeholder = "Search" }},
		{name: "Shortcut", fill: func(n *axNode) { n.Shortcut = "Ctrl+S" }},
		{name: "Level", fill: func(n *axNode) { n.Level = 2 }},
		{name: "RowIndex", fill: func(n *axNode) { n.RowIndex = 7 }},
		{name: "ColumnIndex", fill: func(n *axNode) { n.ColumnIndex = 3 }},
		{name: "RowCount", fill: func(n *axNode) { n.RowCount = 99 }},
		{name: "ColumnCount", fill: func(n *axNode) { n.ColumnCount = 4 }},
		{name: "Step", fill: func(n *axNode) { n.Step = 5 }},
		{name: "Orientation", fill: func(n *axNode) { n.Orientation = accessibility.OrientationVertical }},
		{name: "LabeledBy", fill: func(n *axNode) { n.LabeledBy = []axID{2} }},
		{name: "DescribedBy", fill: func(n *axNode) { n.DescribedBy = []axID{2} }},
		{name: "Controls", fill: func(n *axNode) { n.Controls = []axID{1} }},
	} {
		cur := attributed(one.fill)
		c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}}, accessibility.Diff(diffTree(), cur),
			one.name)
		c.Nil(accessibility.Diff(cur, attributed(one.fill)), "%s must compare equal to itself", one.name)
	}

	// However many of them moved at once, the node gets one event: an assistive technology re-reads the attributes it
	// cares about, so naming each of them separately would only repeat the same instruction.
	many := attributed(func(n *axNode) {
		n.Placeholder = "Search"
		n.Level = 2
		n.RowIndex = 7
		n.Orientation = accessibility.OrientationVertical
		n.Controls = []axID{1}
	})
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}}, accessibility.Diff(diffTree(), many))

	// A relation is compared by what it points at rather than by the slice holding it, so the same ids in the same
	// order are the same relation however they were built.
	rebuilt := attributed(func(n *axNode) { n.Controls = append([]axID{}, 1, 2) })
	c.Nil(accessibility.Diff(attributed(func(n *axNode) { n.Controls = []axID{1, 2} }), rebuilt))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}},
		accessibility.Diff(rebuilt, attributed(func(n *axNode) { n.Controls = []axID{2, 1} })),
		"the order the ids are in is part of the relation")

	// The event comes after the sort direction and before the bounds, which is where an adapter expects it among the
	// rest of what one node reports.
	both := attributed(func(n *axNode) {
		n.Sort = accessibility.SortAscending
		n.Level = 3
		n.Bounds = geom.NewRect(1, 2, 3, 4)
	})
	c.Equal([]axEvent{
		{Kind: accessibility.SortChanged, Node: 3, Old: "unsorted", New: "ascending"},
		{Kind: accessibility.AttributesChanged, Node: 3},
		{Kind: accessibility.BoundsChanged, Node: 3},
	}, accessibility.Diff(diffTree(), both))
}

// TestDiffActionsAloneAreReported pins the case that reaches this from stock unison: List.SetAllowMultipleSelection
// adding AddToSelection and RemoveFromSelection to every row while nothing else about any row moves. What a node can be
// asked to do is what each adapter answers AT-SPI's NActions and GetActions from, what decides which accessibility
// setters a macOS element responds to, and what every adapter checks before it will dispatch a request, so a client
// holding the old set would go on offering an action that is now refused — or never offer one that has appeared.
func TestDiffActionsAloneAreReported(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	row := func(actions accessibility.ActionSet) *accessibility.Tree {
		tree := diffTree()
		n := tree.Node(3)
		n.Role = role.Row
		n.Selectable = true
		n.Actions = actions
		return tree
	}
	single := accessibility.ActionSet(0).With(accessibility.ScrollIntoView, accessibility.Select)
	multiple := single.With(accessibility.AddToSelection, accessibility.RemoveFromSelection)
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}}, accessibility.Diff(row(single), row(multiple)))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}}, accessibility.Diff(row(multiple), row(single)),
		"losing an action is as much a change as gaining one")
	c.Nil(accessibility.Diff(row(multiple), row(multiple)))
}

// TestDiffPlaceholderAloneIsReported pins the case that reaches this from stock unison: a combo field switching its
// watermark between two prompts while its content stays empty. The node's name, value and text are identical either
// way and the placeholder is the only difference, so without an event nothing would prompt a screen reader to re-read
// it and Orca would go on announcing the prompt that is no longer shown.
func TestDiffPlaceholderAloneIsReported(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	field := func(placeholder string) *accessibility.Tree {
		tree := fieldTree("", 0, 0)
		tree.Node(2).Placeholder = placeholder
		return tree
	}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}},
		accessibility.Diff(field("not set"), field("empty")))
}

// TestDiffMultilineAloneIsReported pins the case that reaches this from stock unison: a single-line field that wraps
// reports Multiline from the number of lines it actually drew, so resizing the field around text that already fits
// flips it while the text, the selection and the caret all stay where they were. Every adapter turns it into something
// a client caches — the SINGLE_LINE and MULTI_LINE states on AT-SPI — so without an event a field that has grown onto
// a second line would go on being read out as a single run, with no line-by-line navigation through it, for the life
// of the window.
func TestDiffMultilineAloneIsReported(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	field := func(multiline bool) *accessibility.Tree {
		tree := fieldTree("a long line that wraps", 3, 3)
		tree.Node(2).Text.Multiline = multiline
		return tree
	}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(field(false), field(true)))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(field(true), field(false)),
		"shrinking back onto one line is as much a change as growing off it")
	c.Nil(accessibility.Diff(field(true), field(true)))

	// A node carrying no text lays nothing out over anything, so text arriving already wrapped moves the answer too.
	bare := fieldTree("", 0, 0)
	bare.Node(2).Text = nil
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(bare, field(true)))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(bare, field(false)),
		"text arriving on a node that carried none is a change even when it fits on one line: see TestDiffTextGainedOrLost")
}

// TestDiffTextGainedOrLost pins the AttributesChanged a node gets when it starts, or stops, carrying text at all while
// nothing else about it moves. Two things in stock unison do exactly that: a Label whose title is cleared while an
// application-set name or a LabeledBy keeps it in the tree, and a Field toggled between plain and Protected. Neither
// is multiline on either side and the text events only describe an edit to text present on both sides, so without
// this event an adapter that turns carrying text into something a client caches — AT-SPI's SINGLE_LINE and
// SELECTABLE_TEXT states and its Text interface, UI Automation's Text pattern availability — would never be told to
// grant or retract it.
func TestDiffTextGainedOrLost(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	withText := fieldTree("hello", 0, 0)
	without := fieldTree("hello", 0, 0)
	without.Node(2).Text = nil
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(without, withText))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(withText, without),
		"losing the text altogether is as much a change as gaining it")
	c.Nil(accessibility.Diff(without, without))
}

// TestDiffShortcutAloneIsReported pins the case that reaches this from stock unison: MenuItem.SetKeyBinding rebinding
// an item's accelerator while nothing else about it moves. Every adapter caches the accelerator as a property of the
// element — UI Automation re-reads AcceleratorKey only when a property-changed event says to — so without an event the
// item would go on announcing the key that used to work.
func TestDiffShortcutAloneIsReported(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	item := func(shortcut string) *accessibility.Tree {
		tree := diffTree()
		tree.Node(3).Role = role.MenuItem
		tree.Node(3).Shortcut = shortcut
		return tree
	}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}},
		accessibility.Diff(item("Ctrl+S"), item("Ctrl+Shift+S")))
	c.Nil(accessibility.Diff(item("Ctrl+S"), item("Ctrl+S")))
}

// TestDiffStepAloneIsReported covers the other fact an assistive technology reads out of the numeric group without any
// event of its own naming it: how far one press of an arrow key moves the value. A spin button that changes its
// increment while sitting on the same value tells the person a different thing about what a key press will do.
func TestDiffStepAloneIsReported(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	spinner := func(step float64) *accessibility.Tree {
		tree := diffTree()
		n := tree.Node(3)
		n.Role = role.SpinButton
		n.HasNumber = true
		n.Number = 5
		n.Max = 100
		n.Step = step
		return tree
	}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}}, accessibility.Diff(spinner(1), spinner(10)))
	c.Nil(accessibility.Diff(spinner(1), spinner(1)))
}

// TestDiffToleratesNaN covers the one value that is not equal to itself. A NaN reaching a node — from a widget whose
// arithmetic divided by zero, or a bounds computation over an empty rectangle — would otherwise make a tree differ from
// itself, which is both the documented promise broken and a permanent stream of events to the assistive technology, one
// batch per publish for as long as the window exists.
func TestDiffToleratesNaN(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	nan := math.NaN()
	fnan := float32(math.NaN())
	for _, one := range []struct {
		fill func(n *axNode)
		name string
	}{
		{name: "Number", fill: func(n *axNode) { n.HasNumber = true; n.Number = nan }},
		{name: "Min", fill: func(n *axNode) { n.HasNumber = true; n.Min = nan }},
		{name: "Max", fill: func(n *axNode) { n.HasNumber = true; n.Max = nan }},
		{name: "Step", fill: func(n *axNode) { n.HasNumber = true; n.Step = nan }},
		{name: "Bounds.X", fill: func(n *axNode) { n.Bounds = geom.NewRect(fnan, 0, 10, 10) }},
		{name: "Bounds.Y", fill: func(n *axNode) { n.Bounds = geom.NewRect(0, fnan, 10, 10) }},
		{name: "Bounds.Width", fill: func(n *axNode) { n.Bounds = geom.NewRect(0, 0, fnan, 10) }},
		{name: "Bounds.Height", fill: func(n *axNode) { n.Bounds = geom.NewRect(0, 0, 10, fnan) }},
	} {
		build := func() *accessibility.Tree {
			tree := diffTree()
			one.fill(tree.Node(3))
			return tree
		}
		c.Nil(accessibility.Diff(build(), build()), "%s must not differ from itself", one.name)
	}

	// A NaN arriving where a real value was, or being replaced by one, is a change like any other: what must not be
	// reported is the pair that never moved.
	valued := diffTree()
	valued.Node(3).HasNumber = true
	valued.Node(3).Number = 4
	broken := diffTree()
	broken.Node(3).HasNumber = true
	broken.Node(3).Number = nan
	c.Equal([]axEvent{{Kind: accessibility.NumberChanged, Node: 3, Old: "4", New: "NaN"}},
		accessibility.Diff(valued, broken))
	c.Equal([]axEvent{{Kind: accessibility.NumberChanged, Node: 3, Old: "NaN", New: "4"}},
		accessibility.Diff(broken, valued))
	placed := diffTree()
	placed.Node(3).Bounds = geom.NewRect(0, 0, 10, 10)
	adrift := diffTree()
	adrift.Node(3).Bounds = geom.NewRect(0, 0, fnan, 10)
	c.Equal([]axEvent{{Kind: accessibility.BoundsChanged, Node: 3}}, accessibility.Diff(placed, adrift))
	c.Equal([]axEvent{{Kind: accessibility.BoundsChanged, Node: 3}}, accessibility.Diff(adrift, placed))
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

// TestDiffStateEventsCoverEveryState pins each of the sixteen states to the one pair of fields it is derived from, by
// moving one field at a time and asserting the single event that must come of it. Every one of them is wired up by
// hand, and each is something an adapter branches on, so a state compared against the wrong field — or left out of the
// comparison altogether — would otherwise ship unnoticed. Flipping the whole set at once cannot catch that: a state
// read from the wrong field would produce exactly the same list of events, since every field moved the same way.
//
// The table is in the order the events come out in, which the check at the end uses to pin that order as well.
func TestDiffStateEventsCoverEveryState(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	unflagged := func(_ *axNode) {}
	flagged := func(fill func(n *axNode)) *accessibility.Tree {
		tree := newTree(0,
			&axNode{ID: 1, Role: role.Window, Name: windowName, Children: []axID{2}},
			&axNode{ID: 2, Parent: 1, Role: role.CheckBox, Name: flagsName},
		)
		fill(tree.Node(2))
		return tree
	}
	cases := []struct {
		fill     func(n *axNode)
		oldValue string
		newValue string
		state    accessibility.State
	}{
		{state: accessibility.StateDisabled, fill: func(n *axNode) { n.Disabled = true }},
		{state: accessibility.StateFocusable, fill: func(n *axNode) { n.Focusable = true }},
		{state: accessibility.StateSelectable, fill: func(n *axNode) { n.Selectable = true }},
		{state: accessibility.StateSelected, fill: func(n *axNode) { n.Selected = true }},
		{state: accessibility.StateMultiselectable, fill: func(n *axNode) { n.Multiselectable = true }},
		{state: accessibility.StatePressed, fill: func(n *axNode) { n.Pressed = true }},
		{state: accessibility.StateReadOnly, fill: func(n *axNode) { n.ReadOnly = true }},
		{state: accessibility.StateModal, fill: func(n *axNode) { n.Modal = true }},
		{state: accessibility.StateBusy, fill: func(n *axNode) { n.Busy = true }},
		{state: accessibility.StateInvalid, fill: func(n *axNode) { n.Invalid = true }},
		{state: accessibility.StateOffscreen, fill: func(n *axNode) { n.Offscreen = true }},
		{state: accessibility.StateExpandable, fill: func(n *axNode) { n.Expandable = true }},
		{state: accessibility.StateExpanded, fill: func(n *axNode) { n.Expanded = true }},
		{state: accessibility.StateIgnored, fill: func(n *axNode) { n.Ignored = true }},
		{state: accessibility.StateProtected, fill: func(n *axNode) { n.Protected = true }},
		{
			state: accessibility.StateChecked, oldValue: checkOff, newValue: checkOn,
			fill: func(n *axNode) { n.HasCheck = true; n.Checked = checkenum.On },
		},
	}
	seen := make(map[accessibility.State]bool, len(cases))
	all := make([]axEvent, 0, len(cases))
	fillAll := make([]func(n *axNode), 0, len(cases))
	for _, one := range cases {
		oldValue, newValue := one.oldValue, one.newValue
		if oldValue == "" {
			// The tri-state one reports the check state's key; every other state is an ordinary flag.
			oldValue, newValue = flagOff, flagOn
		}
		want := axEvent{Kind: accessibility.StateChanged, Node: 2, State: one.state, Old: oldValue, New: newValue}
		cur := flagged(one.fill)
		c.Equal([]axEvent{want}, accessibility.Diff(flagged(unflagged), cur), one.state)
		c.Nil(accessibility.Diff(cur, flagged(one.fill)), "%s must compare equal to itself", one.state)
		seen[one.state] = true
		all = append(all, want)
		fillAll = append(fillAll, one.fill)
	}

	// Nothing but StateNone, which is what a non-StateChanged event carries, may be missing from that table.
	for state := accessibility.StateDisabled; state <= accessibility.StateChecked; state++ {
		c.True(seen[state], "no case covers %s", state)
	}

	// Moving every one of them at once produces the whole list in the order the table declares, which is the order an
	// adapter replaying the events sees them in.
	c.Equal(all, accessibility.Diff(flagged(unflagged), flagged(func(n *axNode) {
		for _, fill := range fillAll {
			fill(n)
		}
	})))
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

	// A node that has stopped carrying text, or has only just started, produces no text events at all: the one event
	// is the AttributesChanged that says the text came or went, which TestDiffTextGainedOrLost pins.
	withText := fieldTree("hello", 0, 0)
	without := fieldTree("hello", 0, 0)
	without.Node(2).Text = nil
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(withText, without))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(without, withText))
}

// TestDiffTextSelectionCaretAndOrdering covers the two things the selection event has to get right beyond the offsets
// themselves: the caret moving from one end of an unchanged selection to the other is a change, and the length the
// event reports is a count of runes, so it is never negative however the pair was filled in.
func TestDiffTextSelectionCaretAndOrdering(t *testing.T) {
	t.Parallel()
	c := check.New(t)

	// Shift+Left over an already selected run leaves the selection where it is and moves the caret to the other end,
	// which an assistive technology has to be told about or it goes on reading from the end the caret left.
	atEnd := fieldTree("hello", 1, 3)
	atEnd.Node(2).Text.Caret = 3
	atStart := fieldTree("hello", 1, 3)
	atStart.Node(2).Text.Caret = 1
	c.Equal([]axEvent{{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 1, Length: 2}},
		accessibility.Diff(atEnd, atStart))

	// A widget that fills the pair in from an anchor and a caret without ordering them is still reported as covering
	// three runes from offset one rather than as covering minus three from offset four.
	reversed := fieldTree("hello", 4, 1)
	c.Equal([]axEvent{{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 1, Length: 3}},
		accessibility.Diff(fieldTree("hello", 0, 0), reversed))
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

// TestDiffDocumentTextEvents pins that a Document's composed stream is diffed exactly as a field's own text is. The
// stream is where a document's text lives — Node.Text stays nil on one, so that only the adapters presenting a document
// as text ever see it — and an assistive technology reading a document with its caret is told what changed and where
// the caret went in the same events it is told for a field.
func TestDiffDocumentTextEvents(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{
		{Kind: accessibility.TextDeleted, Node: 2, Start: 3, Length: 2, Old: "lo"},
		{Kind: accessibility.TextInserted, Node: 2, Start: 3, Length: 1, New: "p"},
		{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 4},
		{Kind: accessibility.TextDeleted, Node: 3, Start: 3, Length: 2, Old: "lo"},
		{Kind: accessibility.TextInserted, Node: 3, Start: 3, Length: 1, New: "p"},
		{Kind: accessibility.TextSelectionChanged, Node: 3, Start: 4},
	}, accessibility.Diff(documentTree("hello", 5, 5), documentTree("help", 4, 4)),
		"the document and the block beneath it each report their own edit")
}

// TestDiffDocumentSelectionOnly covers the reading caret moving with the content standing still, which is every arrow
// key a person reading a document presses.
func TestDiffDocumentSelectionOnly(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal([]axEvent{
		{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 1, Length: 2},
		{Kind: accessibility.TextSelectionChanged, Node: 3, Start: 1, Length: 2},
	}, accessibility.Diff(documentTree("hello", 0, 0), documentTree("hello", 1, 3)))

	// The caret moving from one end of an unchanged selection to the other is a change on the stream as much as on a
	// field, since it is the end a screen reader goes on reading from.
	atEnd := documentTree("hello", 1, 3)
	atStart := documentTree("hello", 1, 3)
	atStart.Node(2).Document.Text.Caret = 1
	atStart.Node(3).Text.Caret = 1
	c.Equal([]axEvent{
		{Kind: accessibility.TextSelectionChanged, Node: 2, Start: 1, Length: 2},
		{Kind: accessibility.TextSelectionChanged, Node: 3, Start: 1, Length: 2},
	}, accessibility.Diff(atEnd, atStart))
}

// TestDiffDocumentGainedOrLost pins the AttributesChanged a node gets when it starts, or stops, carrying a composed
// stream. Which patterns and interfaces an adapter offers on the element is decided by that — the UI Automation Text
// pattern among them — and a client reads what an element offers once and caches it, so the arrival of the stream has
// to be announced as well as its content.
func TestDiffDocumentGainedOrLost(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	with := documentTree("hello", 0, 0)
	without := documentTree("hello", 0, 0)
	without.Node(2).Document = nil
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(without, with),
		"a document that has only just composed its content")
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}}, accessibility.Diff(with, without),
		"and one whose content has gone")

	// No text events either way: there is nothing to diff the stream against, exactly as for a field that has only just
	// gained its text.
	for _, events := range [][]axEvent{accessibility.Diff(without, with), accessibility.Diff(with, without)} {
		for _, event := range events {
			c.NotEqual(accessibility.TextInserted, event.Kind)
			c.NotEqual(accessibility.TextDeleted, event.Kind)
			c.NotEqual(accessibility.TextSelectionChanged, event.Kind)
		}
	}
}

// TestDiffURLChange pins that re-pointing a link is reported. Every adapter carries the target as a property of the
// element that a client reads once and caches, so a link that had been given a new target would otherwise go on telling
// the person where it used to go.
func TestDiffURLChange(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	link := func(url string) *accessibility.Tree {
		tree := diffTree()
		tree.Node(3).Role = role.Link
		tree.Node(3).URL = url
		return tree
	}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}},
		accessibility.Diff(link("https://example.com/one"), link("https://example.com/two")))
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 3}},
		accessibility.Diff(link(""), link("https://example.com/one")), "gaining a target is a change too")
	c.Nil(accessibility.Diff(link("https://example.com/one"), link("https://example.com/one")))
}

// TestDiffDocumentComingAndGoingBesideText covers a node that carries text of its own on both sides and gains or loses
// a DocumentInfo beside it. What such a node presents as text never moves — Node.Text is answered first — so the
// comparison of whether it presents any text at all says nothing, yet every adapter keys what it offers on the node
// carrying a stream at all: one refuses the text interface outright to a document, another picks which stream the node
// hands over from the same fact, and a document is a different kind of element from a group. Nothing in the root
// package publishes both today, which is exactly why the guarantee is pinned here.
func TestDiffDocumentComingAndGoingBesideText(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	withText := func(composed bool) *accessibility.Tree {
		n := &axNode{
			ID: 2, Parent: 1, Role: role.Document, Focusable: true, Focused: true,
			Text: &accessibility.TextInfo{Text: "hello", Multiline: true},
		}
		if composed {
			n.Document = &accessibility.DocumentInfo{
				Text: accessibility.TextInfo{Text: "hello", Multiline: true},
			}
		}
		return newTree(2,
			&axNode{ID: 1, Role: role.Window, Name: windowName, Focused: true, Children: []axID{2}}, n)
	}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}},
		accessibility.Diff(withText(false), withText(true)),
		"gaining a stream beside text of its own moves what every adapter offers")
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}},
		accessibility.Diff(withText(true), withText(false)), "and so does losing it")
	c.Nil(accessibility.Diff(withText(true), withText(true)))
	c.Nil(accessibility.Diff(withText(false), withText(false)))
}

// TestDiffSpansComingAndGoing covers a node whose text stands still while the objects sitting within it move. The runes
// are the same on both sides, so the text events say nothing, and every other fact about the node is unchanged; what
// moved is which nodes occupy which part of the text, which decides both what the container can be asked — which
// objects are in it and where — and what each node named by a span answers about where it sits. Neither end would hear
// anything at all without this.
func TestDiffSpansComingAndGoing(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	spanned := func(spans ...accessibility.TextSpan) *accessibility.Tree {
		return newTree(3,
			&axNode{ID: 1, Role: role.Window, Name: windowName, Focused: true, Children: []axID{2}},
			&axNode{
				ID: 2, Parent: 1, Role: role.Paragraph, ReadOnly: true, Children: []axID{3},
				Text: &accessibility.TextInfo{Text: "see the guide", Spans: spans},
			},
			&axNode{ID: 3, Parent: 2, Role: role.Link, Name: "the guide"},
		)
	}
	link := accessibility.TextSpan{Node: 3, Start: 4, End: 13}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}},
		accessibility.Diff(spanned(), spanned(link)), "a paragraph gaining its first span")
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}},
		accessibility.Diff(spanned(link), spanned()), "and losing its last one")
	// Where a span sits is not an attribute of the paragraph: offsets move with every edit ahead of them, which the text
	// events already report, and a client asks for them fresh each time. Only which node a span names is compared.
	moved := accessibility.TextSpan{Node: 3, Start: 0, End: 9}
	c.Nil(accessibility.Diff(spanned(link), spanned(moved)), "a span that covers a different stretch of the same text")
	other := accessibility.TextSpan{Node: 4, Start: 4, End: 13}
	c.Equal([]axEvent{{Kind: accessibility.AttributesChanged, Node: 2}},
		accessibility.Diff(spanned(link), spanned(other)), "and one that names a different node")
	c.Nil(accessibility.Diff(spanned(link), spanned(link)))
}
