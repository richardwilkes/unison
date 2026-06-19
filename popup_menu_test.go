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
	"github.com/richardwilkes/unison"
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

func TestPopupRemoveAllItems(t *testing.T) {
	c := check.New(t)
	p := newTestPopup("a", "b", "c")
	p.SelectIndex(1)
	p.RemoveAllItems()
	c.Equal(0, p.ItemCount())
	c.Equal(-1, p.SelectedIndex())
}
