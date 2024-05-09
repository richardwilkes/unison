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
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/side"
)

// TableColumnHeader defines the methods a table column header must implement.
type TableColumnHeader[T TableRowConstraint[T]] interface {
	Paneler
	SortState() SortState
	SetSortState(state SortState)
}

// DefaultTableColumnHeaderTheme holds the default TableColumnHeaderTheme values for TableColumnHeaders. Modifying this
// data will not alter existing TableColumnHeaders, but will alter any TableColumnHeaders created in the future.
var DefaultTableColumnHeaderTheme = LabelTheme{
	Font:            LabelFont,
	OnBackgroundInk: ThemeOnSurface,
	Gap:             3,
	HAlign:          align.Middle,
	VAlign:          align.Middle,
	Side:            side.Left,
}

// DefaultTableColumnHeader provides a default table column header panel.
type DefaultTableColumnHeader[T TableRowConstraint[T]] struct {
	Label
	sortState     SortState
	sortIndicator *DrawableSVG
}

// NewTableColumnHeader creates a new table column header panel.
func NewTableColumnHeader[T TableRowConstraint[T]](title, tooltip string) *DefaultTableColumnHeader[T] {
	h := &DefaultTableColumnHeader[T]{
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
	if tooltip != "" {
		h.Tooltip = NewTooltipWithText(tooltip)
	}
	return h
}

// DefaultSizes provides the default sizing.
func (h *DefaultTableColumnHeader[T]) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	prefSize = LabelSize(h.textCache.Text(h.Text, h.Font), h.Drawable, h.Side, h.Gap)

	// Account for the potential sort indicator
	baseline := h.Font.Baseline()
	prefSize.Width += h.LabelTheme.Gap + baseline
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
func (h *DefaultTableColumnHeader[T]) DefaultDraw(canvas *Canvas, _ Rect) {
	r := h.ContentRect(false)
	if h.sortIndicator != nil {
		r.Width -= h.LabelTheme.Gap + h.sortIndicator.LogicalSize().Width
	}
	DrawLabel(canvas, r, h.HAlign, h.VAlign, h.textCache.Text(h.Text, h.Font), h.OnBackgroundInk, h.Drawable, h.Side,
		h.Gap, !h.Enabled())
	if h.sortIndicator != nil {
		size := h.sortIndicator.LogicalSize()
		r.X = r.Right() + h.LabelTheme.Gap
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
					Size: Size{Width: baseline, Height: baseline},
				}
			} else {
				h.sortIndicator = &DrawableSVG{
					SVG:  SortDescendingSVG,
					Size: Size{Width: baseline, Height: baseline},
				}
			}
		} else {
			h.sortIndicator = nil
		}
		h.MarkForRedraw()
	}
}

// DefaultMouseUp provides the default mouse up handling.
func (h *DefaultTableColumnHeader[T]) DefaultMouseUp(where Point, _ int, _ Modifiers) bool {
	if h.sortState.Sortable && where.In(h.ContentRect(false)) {
		if header, ok := h.Parent().Self.(*TableHeader[T]); ok {
			header.SortOn(h)
			header.ApplySort()
		}
	}
	return true
}
