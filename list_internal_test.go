// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// newFixedHeightBorderedList returns a list with a fixed cell height of 20 and a 2px border on all sides, sized so all
// rows fit.
func newFixedHeightBorderedList(values ...string) *List[string] {
	l := NewList[string]()
	l.Factory = &DefaultCellFactory{Height: 20}
	l.Append(values...)
	l.SetBorder(NewLineBorder(Black, geom.Size{}, geom.NewUniformInsets(2), false))
	l.SetFrameRect(geom.NewRect(0, 0, 100, 100))
	return l
}

func TestListRowAtFixedHeightClampsAboveContent(t *testing.T) {
	c := check.New(t)
	l := newFixedHeightBorderedList("a", "b", "c")
	contentTop := l.ContentRect(false).Y
	c.Equal(float32(2), contentTop)

	// A y within the top border inset (above the content rect) maps to the first row, matching the variable-height
	// path, rather than going negative and causing DefaultDraw to skip all rows.
	row, top := l.rowAt(0)
	c.Equal(0, row)
	c.Equal(contentTop, top)

	// Rows within the content still map to the correct index and top.
	row, top = l.rowAt(contentTop)
	c.Equal(0, row)
	c.Equal(contentTop, top)
	row, top = l.rowAt(contentTop + 20)
	c.Equal(1, row)
	c.Equal(contentTop+20, top)
	row, top = l.rowAt(contentTop + 59)
	c.Equal(2, row)
	c.Equal(contentTop+40, top)

	// A y beyond the last row still reports no row.
	row, top = l.rowAt(contentTop + 60)
	c.Equal(-1, row)
	c.Equal(float32(0), top)
}

func TestListSelectRangeEmptyList(t *testing.T) {
	c := check.New(t)
	l := NewList[string]()

	// SelectRange and SelectAll on an empty list must not create a phantom selection at index 0.
	l.SelectRange(0, 0, false)
	c.Equal(0, l.Selection.Count())
	l.SelectAll()
	c.Equal(0, l.Selection.Count())
	c.Equal(-1, l.anchor)
}

func TestListSelectRangeNonEmptyList(t *testing.T) {
	c := check.New(t)
	l := NewList[string]()
	l.Append("a", "b", "c")
	l.SetAllowMultipleSelection(true)

	// Normal ranges still work, including clamping of out-of-bounds indexes.
	l.SelectRange(1, 5, false)
	c.Equal(2, l.Selection.Count())
	c.True(l.Selection.State(1))
	c.True(l.Selection.State(2))
	c.Equal(1, l.anchor)

	l.SelectAll()
	c.Equal(3, l.Selection.Count())
}

func TestListRowAtFixedHeightEmptyList(t *testing.T) {
	c := check.New(t)
	l := newFixedHeightBorderedList()
	row, top := l.rowAt(0)
	c.Equal(-1, row)
	c.Equal(float32(0), top)
}

// variableHeightCellFactory produces cells whose heights differ per row, selecting the variable-height code paths that
// a CellHeight() of less than 1 turns on.
type variableHeightCellFactory struct {
	heights []float32
}

// CellHeight implements CellFactory.
func (f *variableHeightCellFactory) CellHeight() float32 {
	return 0
}

// CreateCell implements CellFactory.
func (f *variableHeightCellFactory) CreateCell(_ Paneler, _ any, row int, _, _ Ink, _, _ bool) Paneler {
	size := geom.NewSize(50, f.heights[row])
	p := NewPanel()
	p.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) { return size, size, size })
	return p
}

// TestListRowRectVariableHeight verifies that a row's rectangle in a variable-height list starts where the rows above
// it end. RowRect used to multiply the row's own height by its index, which places every row after the first at the
// wrong offset unless all rows happen to be the same height, so DefaultKeyDown's ScrollRectIntoView could scroll to a
// position that left the selected row off-screen.
func TestListRowRectVariableHeight(t *testing.T) {
	c := check.New(t)
	l := NewList[string]()
	l.Factory = &variableHeightCellFactory{heights: []float32{10, 30, 20, 40}}
	l.Append("a", "b", "c", "d")
	l.SetBorder(NewLineBorder(Black, geom.Size{}, geom.NewUniformInsets(2), false))
	l.SetFrameRect(geom.NewRect(0, 0, 100, 200))
	content := l.ContentRect(false)

	for i, want := range []geom.Rect{
		geom.NewRect(content.X, content.Y, content.Width, 10),
		geom.NewRect(content.X, content.Y+10, content.Width, 30),
		geom.NewRect(content.X, content.Y+40, content.Width, 20),
		geom.NewRect(content.X, content.Y+60, content.Width, 40),
	} {
		c.Equal(want, l.RowRect(i), "row %d", i)
	}

	// Every row's rectangle must agree with the row rowAt reports for a point inside it.
	for i := range 4 {
		rect := l.RowRect(i)
		row, top := l.rowAt(rect.Y + rect.Height/2)
		c.Equal(i, row, "rowAt must report row %d for a point within its rectangle", i)
		c.Equal(rect.Y, top, "rowAt must report the same top as RowRect for row %d", i)
	}

	// Out-of-range rows yield an empty rectangle.
	c.Equal(geom.Rect{}, l.RowRect(-1))
	c.Equal(geom.Rect{}, l.RowRect(4))
}

// TestListRowRectFixedHeight verifies that the fixed-height path is unaffected by the variable-height correction.
func TestListRowRectFixedHeight(t *testing.T) {
	c := check.New(t)
	l := newFixedHeightBorderedList("a", "b", "c")
	content := l.ContentRect(false)
	for i := range 3 {
		c.Equal(geom.NewRect(content.X, content.Y+float32(i)*20, content.Width, 20), l.RowRect(i), "row %d", i)
	}
}
