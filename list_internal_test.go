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

// TestListRowAtFixedHeightClampsAboveContent verifies the row and row top rowAt reports for a y in a fixed-height list
// whose 2px border puts the content top at 2. A y within the top border inset maps to the first row, matching the
// variable-height path, rather than going negative and causing DefaultDraw to skip all rows. A y beyond the last row,
// or any y in an empty list, reports no row.
func TestListRowAtFixedHeightClampsAboveContent(t *testing.T) {
	abc := []string{"a", "b", "c"}
	for _, tc := range []struct {
		name   string
		values []string
		row    int
		y      float32
		top    float32
	}{
		{name: "above the content", values: abc, y: 0, row: 0, top: 2},
		{name: "top of the first row", values: abc, y: 2, row: 0, top: 2},
		{name: "top of the second row", values: abc, y: 22, row: 1, top: 22},
		{name: "last pixel of the last row", values: abc, y: 61, row: 2, top: 42},
		{name: "beyond the last row", values: abc, y: 62, row: -1, top: 0},
		{name: "empty list", y: 0, row: -1, top: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			l := newFixedHeightBorderedList(tc.values...)
			c.Equal(float32(2), l.ContentRect(false).Y)
			row, top := l.rowAt(tc.y)
			c.Equal(tc.row, row)
			c.Equal(tc.top, top)
		})
	}
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
// position that left the selected row off-screen. The fixed-height path, which that correction must leave alone, is
// checked the same way.
func TestListRowRectVariableHeight(t *testing.T) {
	heights := []float32{10, 30, 20, 40}
	variable := NewList[string]()
	variable.Factory = &variableHeightCellFactory{heights: heights}
	variable.Append("a", "b", "c", "d")
	variable.SetBorder(NewLineBorder(Black, geom.Size{}, geom.NewUniformInsets(2), false))
	variable.SetFrameRect(geom.NewRect(0, 0, 100, 200))

	for _, tc := range []struct {
		name    string
		list    *List[string]
		tops    []float32 // Each row's offset from the content top.
		heights []float32
	}{
		{name: "variable height", list: variable, tops: []float32{0, 10, 40, 60}, heights: heights},
		{
			name:    "fixed height",
			list:    newFixedHeightBorderedList("a", "b", "c"),
			tops:    []float32{0, 20, 40},
			heights: []float32{20, 20, 20},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			l := tc.list
			content := l.ContentRect(false)
			for i, top := range tc.tops {
				c.Equal(geom.NewRect(content.X, content.Y+top, content.Width, tc.heights[i]), l.RowRect(i), "row %d", i)
			}

			// Every row's rectangle must agree with the row rowAt reports for a point inside it.
			for i := range tc.tops {
				rect := l.RowRect(i)
				row, top := l.rowAt(rect.Y + rect.Height/2)
				c.Equal(i, row, "rowAt must report row %d for a point within its rectangle", i)
				c.Equal(rect.Y, top, "rowAt must report the same top as RowRect for row %d", i)
			}

			// Out-of-range rows yield an empty rectangle.
			c.Equal(geom.Rect{}, l.RowRect(-1))
			c.Equal(geom.Rect{}, l.RowRect(len(tc.tops)))
		})
	}
}

// TestListRowAtBoundaryPicksLowerRow verifies that a y lying exactly on a row's top boundary maps to that row rather
// than the one above it. The variable-height path's inclusive comparison used to assign the boundary to the previous
// row, so a click landing exactly on a row boundary selected the row above the one clicked, while the fixed-height
// path's floor((y-top)/cellHeight) assigned it to the row itself.
func TestListRowAtBoundaryPicksLowerRow(t *testing.T) {
	c := check.New(t)
	heights := []float32{10, 30, 20, 40}
	l := NewList[string]()
	l.Factory = &variableHeightCellFactory{heights: heights}
	l.Append("a", "b", "c", "d")
	l.SetBorder(NewLineBorder(Black, geom.Size{}, geom.NewUniformInsets(2), false))
	l.SetFrameRect(geom.NewRect(0, 0, 100, 200))

	for i := range heights {
		rect := l.RowRect(i)
		row, top := l.rowAt(rect.Y)
		c.Equal(i, row, "the top boundary of row %d belongs to row %d", i, i)
		c.Equal(rect.Y, top, "rowAt must report the same top as RowRect for row %d", i)

		// The last pixel of the row still belongs to it.
		row, top = l.rowAt(rect.Bottom() - 1)
		c.Equal(i, row, "the last pixel of row %d belongs to row %d", i, i)
		c.Equal(rect.Y, top, "rowAt must report the same top as RowRect for row %d", i)
	}

	// A y past the last row has no row, matching the fixed-height path.
	row, top := l.rowAt(l.RowRect(len(heights) - 1).Bottom())
	c.Equal(-1, row)
	c.Equal(float32(0), top)

	// The fixed-height path resolves boundaries the same way.
	fixed := newFixedHeightBorderedList("a", "b", "c")
	for i := range 3 {
		rect := fixed.RowRect(i)
		row, top = fixed.rowAt(rect.Y)
		c.Equal(i, row, "the top boundary of fixed-height row %d belongs to row %d", i, i)
		c.Equal(rect.Y, top)
	}
}
