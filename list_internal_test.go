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
