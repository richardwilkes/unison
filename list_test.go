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

// selectedIndexes returns the selected row indexes of the list in ascending order.
func selectedIndexes[T any](l *unison.List[T]) []int {
	var out []int
	i := l.Selection.FirstSet()
	for i != -1 {
		out = append(out, i)
		i = l.Selection.NextSet(i + 1)
	}
	return out
}

func newTestList(values ...string) *unison.List[string] {
	l := unison.NewList[string]()
	l.Append(values...)
	return l
}

func TestListAppendAndDataAtIndex(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	c.Equal(3, l.Count())
	c.Equal("b", l.DataAtIndex(1))
	// Out-of-range access yields the zero value rather than panicking.
	c.Equal("", l.DataAtIndex(-1))
	c.Equal("", l.DataAtIndex(99))
}

func TestListReplace(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.Replace(1, "B")
	c.Equal("B", l.DataAtIndex(1))
	// Out-of-range replace is a no-op.
	l.Replace(99, "x")
	c.Equal(3, l.Count())
}

func TestListInsertShiftsSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.Select(false, 2) // select "c"
	l.Insert(1, "x", "y")
	c.Equal(5, l.Count())
	c.Equal("c", l.DataAtIndex(4))
	// The selection follows the moved item.
	c.Equal([]int{4}, selectedIndexes(l))
}

func TestListInsertClampsIndex(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b")
	l.Insert(-1, "z") // negative clamps to append
	c.Equal("z", l.DataAtIndex(2))
}

func TestListRemoveShiftsSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	l.Select(true, 1, 3)
	l.Remove(2) // remove "c"
	c.Equal(3, l.Count())
	// Index 1 ("b") stays; index 3 ("d") slides down to 2.
	c.Equal([]int{1, 2}, selectedIndexes(l))
	c.Equal("d", l.DataAtIndex(2))
}

func TestListRemoveRangeShiftsSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.SelectAll()
	l.RemoveRange(1, 2) // remove "b","c"
	c.Equal(3, l.Count())
	c.Equal([]int{0, 1, 2}, selectedIndexes(l))
	c.Equal("d", l.DataAtIndex(1))
}

func TestListInsertShiftsAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.Select(false, 2) // anchor at "c"
	c.Equal(2, l.Anchor())
	l.Insert(1, "x", "y")
	// The anchor follows the moved item.
	c.Equal(4, l.Anchor())
	// Inserting after the anchor leaves it alone.
	l.Insert(5, "z")
	c.Equal(4, l.Anchor())
}

func TestListRemoveAdjustsAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	l.Select(false, 2) // anchor at "c"
	l.Remove(0)        // remove "a"; anchor slides down with its row
	c.Equal(1, l.Anchor())
	l.Remove(2) // remove "d", after the anchor; anchor unchanged
	c.Equal(1, l.Anchor())
	l.Remove(1) // remove the anchored row itself
	c.Equal(-1, l.Anchor())
}

func TestListRemoveRangeAdjustsAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.Select(false, 4) // anchor at "e"
	l.RemoveRange(0, 1)
	c.Equal(2, l.Anchor())
	l.RemoveRange(1, 2) // range covers the anchored row
	c.Equal(-1, l.Anchor())
}

func TestListRemoveRangeNoPhantomSelection(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e", "f", "g", "h", "i", "j")
	l.Select(false, 9)  // anchor at the last row
	l.RemoveRange(5, 9) // the anchored row is gone; the anchor must not point past the new row count
	c.Equal(-1, l.Anchor())
	// A subsequent range selection anchors afresh instead of extending from the stale index, so the selection can
	// never include rows beyond the current count and CanSelectAll stays consistent.
	l.SelectRange(2, 2, true)
	c.Equal([]int{2}, selectedIndexes(l))
	c.Equal(2, l.Anchor())
	c.True(l.Selection.Count() <= l.Count())
	c.True(l.CanSelectAll())
}

func TestListSelectReplaceVsAdd(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	l.Select(false, 1)
	c.Equal([]int{1}, selectedIndexes(l))
	// add=false replaces.
	l.Select(false, 3)
	c.Equal([]int{3}, selectedIndexes(l))
	// add=true augments; the existing anchor (3) is retained.
	l.Select(true, 0)
	c.Equal([]int{0, 3}, selectedIndexes(l))
	c.Equal(3, l.Anchor())
}

func TestListSelectRangeClamps(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d", "e")
	l.SelectRange(-3, 99, false)
	c.Equal([]int{0, 1, 2, 3, 4}, selectedIndexes(l))
}

func TestListSelectAllAndCanSelectAll(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	c.True(l.CanSelectAll())
	l.SelectAll()
	c.Equal([]int{0, 1, 2}, selectedIndexes(l))
	c.False(l.CanSelectAll())
}

func TestListClearResetsSelectionAndAnchor(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c")
	l.SelectAll()
	l.Clear()
	c.Equal(0, l.Count())
	c.Equal(0, l.Selection.Count())
	c.Equal(-1, l.Anchor())
}

func TestListSetAllowMultipleSelectionCollapses(t *testing.T) {
	c := check.New(t)
	l := newTestList("a", "b", "c", "d")
	c.True(l.AllowMultipleSelection())
	l.Select(true, 1, 2, 3)
	c.Equal(3, l.Selection.Count())

	// Disabling multiple selection collapses to a single anchored row.
	l.SetAllowMultipleSelection(false)
	c.False(l.AllowMultipleSelection())
	c.Equal(1, l.Selection.Count())

	// With multiple disabled, Select keeps only the last requested index.
	l.Select(false, 0, 2)
	c.Equal([]int{2}, selectedIndexes(l))
}
