// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var _ TableColumnHeader = &DefaultTableColumnHeader{}

// TableColumnHeader defines the methods a table column header must implement.
type TableColumnHeader interface {
	Paneler
	SortState() SortState
	SetSortState(state SortState)
}

// DefaultTableColumnHeaderTheme holds the default TableColumnHeaderTheme values for TableColumnHeaders. Modifying this
// data will not alter existing TableColumnHeaders, but will alter any TableColumnHeaders created in the future.
var DefaultTableColumnHeaderTheme = LabelTheme{
	Font:            LabelFont,
	OnBackgroundInk: OnBackgroundColor,
	Gap:             3,
	HAlign:          MiddleAlignment,
	VAlign:          MiddleAlignment,
	Side:            RightSide,
}

// DefaultTableColumnHeader provides a default table column header panel.
type DefaultTableColumnHeader struct {
	Label
	sortState SortState
}

// NewTableColumnHeader creates a new table column header panel with the given title.
func NewTableColumnHeader(title string) *DefaultTableColumnHeader {
	h := &DefaultTableColumnHeader{
		Label: Label{
			LabelTheme: DefaultTableColumnHeaderTheme,
			Text:       title,
		},
		sortState: SortState{
			Order:     -1,
			Ascending: true,
			Sortable:  true,
		},
	}
	h.Self = h
	h.SetSizer(h.DefaultSizes)
	h.DrawCallback = h.DefaultDraw
	h.MouseUpCallback = h.DefaultMouseUp
	return h
}

// SortState returns the current SortState.
func (h *DefaultTableColumnHeader) SortState() SortState {
	return h.sortState
}

// SetSortState sets the SortState.
func (h *DefaultTableColumnHeader) SetSortState(state SortState) {
	if h.sortState != state {
		h.sortState = state
		if h.sortState.Sortable && h.sortState.Order == 0 {
			baseline := h.Font.ResolvedFont().Baseline()
			if h.sortState.Ascending {
				h.Drawable = &DrawableSVG{
					SVG:  SortAscendingSVG(),
					Size: geom32.NewSize(baseline, baseline),
				}
			} else {
				h.Drawable = &DrawableSVG{
					SVG:  SortDescendingSVG(),
					Size: geom32.NewSize(baseline, baseline),
				}
			}
		} else {
			h.Drawable = nil
		}
		h.MarkForRedraw()
	}
}

// DefaultMouseUp provides the default mouse up handling.
func (h *DefaultTableColumnHeader) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	if h.sortState.Sortable && h.ContentRect(false).ContainsPoint(where) {
		if header, ok := h.Parent().Self.(*TableHeader); ok {
			header.SortOn(h)
			header.ApplySort()
		}
	}
	return true
}
