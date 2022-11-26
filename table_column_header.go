// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

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
	OnBackgroundInk: OnBackgroundColor,
	Gap:             3,
	HAlign:          MiddleAlignment,
	VAlign:          MiddleAlignment,
	Side:            LeftSide,
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
func (h *DefaultTableColumnHeader[T]) DefaultSizes(hint Size) (min, pref, max Size) {
	pref = LabelSize(h.textCache.Text(h.Text, h.Font), h.Drawable, h.Side, h.Gap)

	// Account for the potential sort indicator
	baseline := h.Font.Baseline()
	pref.Width += h.LabelTheme.Gap + baseline
	if pref.Height < baseline {
		pref.Height = baseline
	}

	if b := h.Border(); b != nil {
		pref.AddInsets(b.Insets())
	}
	pref.GrowToInteger()
	pref.ConstrainForHint(hint)
	return pref, pref, pref
}

// DefaultDraw provides the default drawing.
func (h *DefaultTableColumnHeader[T]) DefaultDraw(canvas *Canvas, dirty Rect) {
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
		paint := h.OnBackgroundInk.Paint(canvas, r, Fill)
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
					Size: NewSize(baseline, baseline),
				}
			} else {
				h.sortIndicator = &DrawableSVG{
					SVG:  SortDescendingSVG,
					Size: NewSize(baseline, baseline),
				}
			}
		} else {
			h.sortIndicator = nil
		}
		h.MarkForRedraw()
	}
}

// DefaultMouseUp provides the default mouse up handling.
func (h *DefaultTableColumnHeader[T]) DefaultMouseUp(where Point, button int, mod Modifiers) bool {
	if h.sortState.Sortable && h.ContentRect(false).ContainsPoint(where) {
		if header, ok := h.Parent().Self.(*TableHeader[T]); ok {
			header.SortOn(h)
			header.ApplySort()
		}
	}
	return true
}
