// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

// SortState holds data regarding a sort state.
type SortState struct {
	Order     int // A negative value indicates it isn't participating at the moment.
	Ascending bool
	Sortable  bool // A false value indicates it is not sortable at all
}

// TableColumnHeader defines the methods a table column header must implement.
type TableColumnHeader[T TableRowConstraint[T]] interface {
	Paneler
	SortState() SortState
	SetSortState(state SortState)
	Less() func(a, b string) bool // May return nil -- if so, the Less() from the table header will be used
}

// DefaultTableColumnHeaderTheme holds the default TableColumnHeaderTheme values for TableColumnHeaders. Modifying this
// data will not alter existing TableColumnHeaders, but will alter any TableColumnHeaders created in the future.
var DefaultTableColumnHeaderTheme = LabelTheme{
	TextDecoration: TextDecoration{
		Font:            LabelFont,
		OnBackgroundInk: ThemeOnSurface,
	},
	Gap:    StdIconGap,
	HAlign: align.Middle,
	VAlign: align.Middle,
	Side:   side.Left,
}

// DefaultTableColumnHeader provides a default table column header panel.
type DefaultTableColumnHeader[T TableRowConstraint[T]] struct {
	*Label
	less          func(a, b string) bool
	sortIndicator *DrawableSVG
	sortState     SortState
}

// NewTableColumnHeader creates a new table column header panel. May pass nil for 'less' to use the table header's Less.
func NewTableColumnHeader[T TableRowConstraint[T]](title, tooltip string, less func(a, b string) bool) *DefaultTableColumnHeader[T] {
	h := &DefaultTableColumnHeader[T]{
		Label: NewLabel(),
		sortState: SortState{
			Order:     -1,
			Ascending: true,
			Sortable:  true,
		},
		less: less,
	}
	h.Self = h
	h.LabelTheme = DefaultTableColumnHeaderTheme
	h.SetTitle(title)
	h.SetSizer(h.DefaultSizes)
	h.DrawCallback = h.DefaultDraw
	h.MouseUpCallback = h.DefaultMouseUp
	if tooltip != "" {
		h.Tooltip = NewTooltipWithText(tooltip)
	}
	return h
}

// DefaultSizes provides the default sizing.
func (h *DefaultTableColumnHeader[T]) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	prefSize, _ = LabelContentSizes(h.Text, h.Drawable, h.Font, h.Side, h.Gap)

	// Account for the potential sort indicator
	baseline := h.Font.Baseline()
	prefSize.Width += h.Gap + baseline
	if prefSize.Height < baseline {
		prefSize.Height = baseline
	}

	if b := h.Border(); b != nil {
		prefSize = prefSize.Add(b.Insets().Size())
	}
	prefSize = prefSize.Ceil().ConstrainForHint(hint)
	return prefSize, prefSize, prefSize
}

// DefaultDraw provides the default drawing.
func (h *DefaultTableColumnHeader[T]) DefaultDraw(canvas *Canvas, _ geom.Rect) {
	r := h.ContentRect(false)
	if h.sortIndicator != nil {
		r.Width -= h.Gap + h.sortIndicator.LogicalSize().Width
	}
	DrawLabel(canvas, r, h.HAlign, h.VAlign, h.Font, h.Text, h.OnBackgroundInk, nil, h.Drawable, h.Side, h.Gap,
		!h.Enabled())
	if h.sortIndicator != nil {
		size := h.sortIndicator.LogicalSize()
		r.X = r.Right() + h.Gap
		r.Y += (r.Height - size.Height) / 2
		r.Size = size
		paint := h.OnBackgroundInk.Paint(canvas, r, paintstyle.Fill)
		if !h.Enabled() {
			paint.SetColorFilter(Grayscale30Filter())
		}
		h.sortIndicator.DrawInRect(canvas, r, nil, paint)
	}
}

// SortState returns the current SortState.
func (h *DefaultTableColumnHeader[T]) SortState() SortState {
	return h.sortState
}

// SetSortState sets the SortState.
func (h *DefaultTableColumnHeader[T]) SetSortState(state SortState) {
	if h.sortState != state {
		h.sortState = state
		if h.sortState.Sortable && h.sortState.Order == 0 {
			baseline := h.Font.Baseline()
			if h.sortState.Ascending {
				h.sortIndicator = &DrawableSVG{
					SVG:  SortAscendingSVG,
					Size: geom.Size{Width: baseline, Height: baseline},
				}
			} else {
				h.sortIndicator = &DrawableSVG{
					SVG:  SortDescendingSVG,
					Size: geom.Size{Width: baseline, Height: baseline},
				}
			}
		} else {
			h.sortIndicator = nil
		}
		h.MarkForRedraw()
	}
}

// DefaultMouseUp provides the default mouse up handling.
func (h *DefaultTableColumnHeader[T]) DefaultMouseUp(where geom.Point, _ int, _ Modifiers) bool {
	if h.sortState.Sortable && where.In(h.ContentRect(false)) {
		if header, ok := h.Parent().Self.(*TableHeader[T]); ok {
			header.SortOn(h)
			header.ApplySort()
		}
	}
	return true
}

// Less returns the less function to use for this column, or nil if the table header's Less should be used.
func (h *DefaultTableColumnHeader[T]) Less() func(a, b string) bool {
	return h.less
}
