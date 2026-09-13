// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
)

func newTestPopup(items ...string) *unison.PopupMenu[string] {
	p := unison.NewPopupMenu[string]()
	p.AddItem(items...)
	return p
}

func TestPopupAddItemsAndCount(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b")
	p.AddSeparator()
	p.AddDisabledItem("c")
	c.Equal(4, p.ItemCount())

	item, ok := p.ItemAt(0)
	c.True(ok)
	c.Equal("a", item)
	c.True(p.ItemEnabledAt(1))
	c.False(p.ItemEnabledAt(3)) // disabled item

	// A separator slot reports no item.
	_, ok = p.ItemAt(2)
	c.False(ok)
}

func TestPopupIndexOfItem(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	c.Equal(1, p.IndexOfItem("b"))
	c.Equal(-1, p.IndexOfItem("missing"))
}

func TestPopupSelectByValue(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.Select("b")
	c.Equal(1, p.SelectedIndex())
	item, ok := p.Selected()
	c.True(ok)
	c.Equal("b", item)
	c.Equal("b", p.Text())
}

func TestPopupSelectIndexReplaces(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(0)
	c.Equal([]int{0}, p.SelectedIndexes())
	p.SelectIndex(2)
	c.Equal([]int{2}, p.SelectedIndexes())
}

func TestPopupSelectMultipleShowsMultiple(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(0, 2)
	c.Equal([]int{0, 2}, p.SelectedIndexes())
	c.Equal("Multiple", p.Text())
}

func TestPopupSelectIgnoresSeparator(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a")
	p.AddSeparator()
	p.AddItem("c")
	// Selecting the separator index has no effect.
	p.SelectIndex(1)
	c.Equal(-1, p.SelectedIndex())
	c.Equal("", p.Text())
}

func TestPopupSelectionChangedCallback(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	calls := 0
	p.SelectionChangedCallback = func(_ *unison.PopupMenu[string]) { calls++ }
	p.SelectIndex(1)
	c.Equal(1, calls)
	// Selecting the same index again does not fire the callback.
	p.SelectIndex(1)
	c.Equal(1, calls)
	p.SelectIndex(2)
	c.Equal(2, calls)
}

func TestPopupRemoveItemAtShiftsSelection(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c", "d")
	p.SelectIndex(3)  // select "d"
	p.RemoveItemAt(1) // remove "b"
	c.Equal(3, p.ItemCount())
	// "d" slid down from index 3 to index 2 and stays selected.
	c.Equal([]int{2}, p.SelectedIndexes())
	item, ok := p.Selected()
	c.True(ok)
	c.Equal("d", item)
}

func TestPopupRemoveItemByValue(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(2)
	p.RemoveItem("a")
	c.Equal(2, p.ItemCount())
	c.Equal("c", p.Text())
}

func TestPopupRemoveMissingItemLeavesSelectionAlone(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(2)
	p.RemoveItem("missing")
	c.Equal(3, p.ItemCount())
	c.Equal([]int{2}, p.SelectedIndexes())
	item, ok := p.Selected()
	c.True(ok)
	c.Equal("c", item)
}

func TestPopupSetItemEnabledAndReplace(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b")
	p.SetItemEnabledAt(0, false)
	c.False(p.ItemEnabledAt(0))
	p.SetItemAt(1, "B", true)
	item, ok := p.ItemAt(1)
	c.True(ok)
	c.Equal("B", item)
}

// TestPopupSetItemAtUpdatesEnabledForUnchangedItem verifies that SetItemAt applies a new enabled state even when the
// item value itself is unchanged. The update used to be skipped entirely whenever the value matched, silently ignoring
// the enabled argument.
func TestPopupSetItemAtUpdatesEnabledForUnchangedItem(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b")
	c.True(p.ItemEnabledAt(1))

	p.SetItemAt(1, "b", false)
	c.False(p.ItemEnabledAt(1))
	item, ok := p.ItemAt(1)
	c.True(ok)
	c.Equal("b", item)

	p.SetItemAt(1, "b", true)
	c.True(p.ItemEnabledAt(1))

	// Replacing a separator with an item continues to work, including when it starts out disabled.
	p.AddSeparator()
	p.SetItemAt(2, "c", false)
	item, ok = p.ItemAt(2)
	c.True(ok)
	c.Equal("c", item)
	c.False(p.ItemEnabledAt(2))
}

func TestPopupRemoveAllItems(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(1)
	p.RemoveAllItems()
	c.Equal(0, p.ItemCount())
	c.Equal(-1, p.SelectedIndex())
}

// TestPopupMenuAccessibilityReportsWhetherItIsOpen verifies that a popup menu says whether its choices are showing and
// can be asked to put them away again. It always claimed to be collapsed, whatever was on the screen, so UI
// Automation's ExpandCollapseState and AT-SPI's STATE_EXPANDED never moved and there was no collapse to ask for.
func TestPopupMenuAccessibilityReportsWhetherItIsOpen(t *testing.T) {
	c := check.New(t)
	var popup *unison.PopupMenu[string]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			popup = unison.NewPopupMenu[string]()
			popup.AddItem("Light", "Regular", "Bold")
			popup.Select("Regular")
			wnd = newHeadlessWindow(t, "popup state", geom.NewRect(10, 10, 300, 150), axColumn(popup))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Expandable)
	c.False(node.Expanded, "nothing is showing yet")
	c.True(node.Actions.Has(accessibility.Expand))
	c.True(node.Actions.Has(accessibility.Collapse))

	before := axRootChildCount(screen.AccessibilityTree(wnd))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)),
		"expanding the popup menu should have opened its menu within the window")
	node = screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.True(node.Expanded, "the popup must say its choices are showing while they are")

	// Asking again for what is already there changes nothing, rather than tearing the menu down and building it back.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Expand,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)))

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Collapse,
	}))
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(wnd)),
		"collapsing the popup menu should have taken its menu away")
	node = screen.AccessibilityNodeFor(popup)
	c.True(node != nil)
	if node != nil {
		c.False(node.Expanded)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
