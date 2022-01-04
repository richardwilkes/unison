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
	"sort"

	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

// DefaultTableHeaderTheme holds the default TableHeaderTheme values for TableHeaders. Modifying this data will not
// alter existing TableHeaders, but will alter any TableHeaders created in the future.
var DefaultTableHeaderTheme = TableHeaderTheme{
	BackgroundInk: ControlColor,
	DividerInk:    DividerColor,
	HeaderBorder:  NewLineBorder(DividerColor, 0, geom32.Insets{Bottom: 1}, false),
}

// TableHeaderTheme holds theming data for a TableHeader.
type TableHeaderTheme struct {
	BackgroundInk Ink
	DividerInk    Ink
	HeaderBorder  Border
}

// TableHeader provides a header for a Table.
type TableHeader struct {
	Panel
	TableHeaderTheme
	Table                *Table
	ColumnHeaders        []TableColumnHeader
	interactionColumn    int
	columnResizeStart    float32
	columnResizeBase     float32
	columnResizeOverhead float32
	inHeader             bool
}

// NewTableHeader creates a new TableHeader.
func NewTableHeader(table *Table, columnHeaders ...TableColumnHeader) *TableHeader {
	h := &TableHeader{
		TableHeaderTheme: DefaultTableHeaderTheme,
		Table:            table,
		ColumnHeaders:    columnHeaders,
	}
	h.Self = h
	h.SetSizer(h.DefaultSizes)
	h.SetBorder(h.TableHeaderTheme.HeaderBorder)
	h.DrawCallback = h.DefaultDraw
	h.UpdateCursorCallback = h.DefaultUpdateCursorCallback
	h.UpdateTooltipCallback = h.DefaultUpdateTooltipCallback
	h.MouseMoveCallback = h.DefaultMouseMove
	h.MouseDownCallback = h.DefaultMouseDown
	h.MouseDragCallback = h.DefaultMouseDrag
	h.MouseUpCallback = h.DefaultMouseUp
	return h
}

// DefaultSizes provides the default sizing.
func (h *TableHeader) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	pref.Width = h.Table.FrameRect().Size.Width
	pref.Height = h.heightForColumns()
	if border := h.Border(); border != nil {
		insets := border.Insets()
		pref.Height += insets.Top + insets.Bottom
	}
	return pref, pref, pref
}

// ColumnFrame returns the frame of the given column.
func (h *TableHeader) ColumnFrame(col int) geom32.Rect {
	if col < 0 || col >= len(h.Table.ColumnSizes) {
		return geom32.Rect{}
	}
	insets := h.combinedInsets()
	x := insets.Left
	for c := 0; c < col; c++ {
		x += h.Table.ColumnSizes[c].Current
		if h.Table.ShowColumnDivider {
			x++
		}
	}
	rect := geom32.NewRect(x, insets.Top, h.Table.ColumnSizes[col].Current, h.FrameRect().Height-(insets.Top+insets.Bottom))
	rect.Inset(h.Table.Padding)
	return rect
}

func (h *TableHeader) heightForColumns() float32 {
	var height float32
	for i := range h.Table.ColumnSizes {
		w := h.Table.ColumnSizes[i].Current
		if w <= 0 {
			continue
		}
		w -= h.Table.Padding.Left + h.Table.Padding.Right
		if i < len(h.ColumnHeaders) {
			_, cpref, _ := h.ColumnHeaders[i].AsPanel().Sizes(geom32.Size{Width: w})
			cpref.Height += h.Table.Padding.Top + h.Table.Padding.Bottom
			if height < cpref.Height {
				height = cpref.Height
			}
		}
	}
	return mathf32.Max(mathf32.Ceil(height), h.Table.MinimumRowHeight)
}

func (h *TableHeader) combinedInsets() geom32.Insets {
	var insets geom32.Insets
	if border := h.Border(); border != nil {
		insets = border.Insets()
	}
	if border := h.Table.Border(); border != nil {
		insets2 := border.Insets()
		if insets.Left < insets2.Left {
			insets.Left = insets2.Left
		}
		if insets.Right < insets2.Right {
			insets.Right = insets2.Right
		}
	}
	return insets
}

// DefaultDraw provides the default drawing.
func (h *TableHeader) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	canvas.DrawRect(dirty, h.BackgroundInk.Paint(canvas, dirty, Fill))

	var firstCol int
	insets := h.combinedInsets()
	x := insets.Left
	for i := range h.Table.ColumnSizes {
		x1 := x + h.Table.ColumnSizes[i].Current
		if h.Table.ShowColumnDivider {
			x1++
		}
		if x1 >= dirty.X {
			break
		}
		x = x1
		firstCol = i + 1
	}

	if h.Table.ShowColumnDivider {
		rect := dirty
		rect.X = x
		rect.Width = 1
		for c := firstCol; c < len(h.Table.ColumnSizes)-1; c++ {
			rect.X += h.Table.ColumnSizes[c].Current
			canvas.DrawRect(rect, h.DividerInk.Paint(canvas, rect, Fill))
			rect.X++
		}
	}

	rect := dirty
	rect.X = x
	rect.Y = insets.Top
	rect.Height = h.heightForColumns()
	lastX := dirty.Right()
	for c := firstCol; c < len(h.Table.ColumnSizes) && rect.X < lastX; c++ {
		rect.Width = h.Table.ColumnSizes[c].Current
		cellRect := rect
		cellRect.Inset(h.Table.Padding)
		if c < len(h.ColumnHeaders) {
			cell := h.ColumnHeaders[c].AsPanel()
			h.installCell(cell, cellRect)
			canvas.Save()
			canvas.Translate(cellRect.X, cellRect.Y)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, cellRect)
			h.uninstallCell(cell)
			canvas.Restore()
		}
		rect.X += h.Table.ColumnSizes[c].Current
		if h.Table.ShowColumnDivider {
			rect.X++
		}
	}
}

func (h *TableHeader) installCell(cell *Panel, frame geom32.Rect) {
	cell.SetFrameRect(frame)
	cell.ValidateLayout()
	cell.parent = h.AsPanel()
}

func (h *TableHeader) uninstallCell(cell *Panel) {
	cell.parent = nil
}

// DefaultUpdateCursorCallback provides the default cursor update handling.
func (h *TableHeader) DefaultUpdateCursorCallback(where geom32.Point) *Cursor {
	if over := h.Table.OverColumnDivider(where.X); over != -1 {
		if h.Table.ColumnSizes[over].Minimum <= 0 || h.Table.ColumnSizes[over].Minimum < h.Table.ColumnSizes[over].Maximum {
			return ArrowsHorizontalCursor()
		}
	}
	if col := h.Table.OverColumn(where.X); col != -1 {
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.UpdateCursorCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			where.Subtract(rect.Point)
			cursor := cell.UpdateCursorCallback(where)
			h.uninstallCell(cell)
			return cursor
		}
	}
	return nil
}

// DefaultUpdateTooltipCallback provides the default tooltip update handling.
func (h *TableHeader) DefaultUpdateTooltipCallback(where geom32.Point, suggestedAvoid geom32.Rect) geom32.Rect {
	if col := h.Table.OverColumn(where.X); col != -1 {
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.UpdateTooltipCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			where.Subtract(rect.Point)
			avoid := cell.UpdateTooltipCallback(where, suggestedAvoid)
			h.Tooltip = cell.Tooltip
			h.uninstallCell(cell)
			return avoid
		}
	}
	h.Tooltip = nil
	return geom32.Rect{}
}

// DefaultMouseMove provides the default mouse move handling.
func (h *TableHeader) DefaultMouseMove(where geom32.Point, mod Modifiers) bool {
	stop := false
	if col := h.Table.OverColumn(where.X); col != -1 {
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.MouseMoveCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			where.Subtract(rect.Point)
			stop = cell.MouseMoveCallback(where, mod)
			h.uninstallCell(cell)
		}
	}
	return stop
}

// DefaultMouseDown provides the default mouse down handling.
func (h *TableHeader) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	h.interactionColumn = -1
	h.inHeader = false
	if over := h.Table.OverColumnDivider(where.X); over != -1 {
		if h.Table.ColumnSizes[over].Minimum <= 0 || h.Table.ColumnSizes[over].Minimum < h.Table.ColumnSizes[over].Maximum {
			if clickCount == 2 {
				h.Table.SizeColumnToFit(over, true)
				h.MarkForRedraw()
				h.Window().UpdateCursorNow()
				return true
			}
			h.interactionColumn = over
			h.columnResizeStart = where.X
			h.columnResizeBase = h.Table.ColumnSizes[over].Current
			h.columnResizeOverhead = h.Table.Padding.Left + h.Table.Padding.Right
			if over == h.Table.HierarchyColumnIndex {
				depth := 0
				for _, cache := range h.Table.rowCache {
					if depth < cache.depth {
						depth = cache.depth
					}
				}
				h.columnResizeOverhead += h.Table.Padding.Left + h.Table.HierarchyIndent*float32(depth+1)
			}
			return true
		}
	}
	stop := true
	if col := h.Table.OverColumn(where.X); col != -1 {
		h.interactionColumn = col
		h.inHeader = true
		cell := h.ColumnHeaders[col].AsPanel()
		if cell.MouseDownCallback != nil {
			rect := h.ColumnFrame(col)
			h.installCell(cell, rect)
			where.Subtract(rect.Point)
			stop = cell.MouseDownCallback(where, button, clickCount, mod)
			h.uninstallCell(cell)
		}
	}
	return stop
}

// DefaultMouseDrag provides the default mouse drag handling.
func (h *TableHeader) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	if !h.inHeader && h.interactionColumn != -1 {
		width := h.columnResizeBase + where.X - h.columnResizeStart
		if width < h.columnResizeOverhead {
			width = h.columnResizeOverhead
		}
		min := h.Table.ColumnSizes[h.interactionColumn].Minimum
		if min > 0 && width < min+h.columnResizeOverhead {
			width = min + h.columnResizeOverhead
		} else {
			max := h.Table.ColumnSizes[h.interactionColumn].Maximum
			if max > 0 && width > max+h.columnResizeOverhead {
				width = max + h.columnResizeOverhead
			}
		}
		if h.Table.ColumnSizes[h.interactionColumn].Current != width {
			h.Table.ColumnSizes[h.interactionColumn].Current = width
			h.Table.SyncToModel()
			h.MarkForRedraw()
		}
		return true
	}
	return false
}

// DefaultMouseUp provides the default mouse up handling.
func (h *TableHeader) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	stop := false
	if h.inHeader && h.interactionColumn != -1 {
		cell := h.ColumnHeaders[h.interactionColumn].AsPanel()
		if cell.MouseUpCallback != nil {
			rect := h.ColumnFrame(h.interactionColumn)
			h.installCell(cell, rect)
			where.Subtract(rect.Point)
			stop = cell.MouseUpCallback(where, button, mod)
			h.uninstallCell(cell)
		}
	}
	return stop
}

// SortOn adjusts the sort such that the specified header is the primary sort column. If the header was already the
// primary sort column, then its ascending/descending flag will be flipped instead.
func (h *TableHeader) SortOn(header TableColumnHeader) {
	if header.SortState().Sortable {
		headers := make([]TableColumnHeader, len(h.ColumnHeaders))
		copy(headers, h.ColumnHeaders)
		sort.Slice(headers, func(i, j int) bool {
			if headers[i] == header {
				return true
			}
			if headers[j] == header {
				return false
			}
			s1 := headers[i].SortState()
			if !s1.Sortable || s1.Order < 0 {
				return false
			}
			s2 := headers[j].SortState()
			if !s2.Sortable || s2.Order < 0 {
				return true
			}
			return s1.Order < s2.Order
		})
		for i, hdr := range headers {
			s := hdr.SortState()
			if s.Sortable {
				if i == 0 {
					if s.Order == 0 {
						s.Ascending = !s.Ascending
					} else {
						s.Order = 0
					}
				} else if s.Order >= 0 {
					s.Order = i
				}
			} else {
				s.Order = -1
			}
			hdr.SetSortState(s)
		}
	}
}

type headerWithIndex struct {
	index  int
	header TableColumnHeader
}

// ApplySort sorts the table according to the current sort criteria.
func (h *TableHeader) ApplySort() {
	headers := make([]*headerWithIndex, len(h.ColumnHeaders))
	for i, hdr := range h.ColumnHeaders {
		headers[i] = &headerWithIndex{
			index:  i,
			header: hdr,
		}
	}
	sort.Slice(headers, func(i, j int) bool {
		s1 := headers[i].header.SortState()
		if !s1.Sortable || s1.Order < 0 {
			return false
		}
		s2 := headers[j].header.SortState()
		if !s2.Sortable || s2.Order < 0 {
			return true
		}
		return s1.Order < s2.Order
	})
	for i, hdr := range headers {
		s := hdr.header.SortState()
		if !s.Sortable || s.Order < 0 {
			headers = headers[:i]
			break
		}
	}
	h.applySort(headers, h.Table.topLevelRows)
	h.Table.SyncToModel()
	h.MarkForRedraw()
}

func (h *TableHeader) applySort(headers []*headerWithIndex, rows []TableRowData) {
	if len(headers) > 0 && len(rows) > 0 {
		sort.Slice(rows, func(i, j int) bool {
			for _, hdr := range headers {
				d1 := rows[i].CellDataForSort(hdr.index)
				d2 := rows[j].CellDataForSort(hdr.index)
				if d1 != d2 {
					if txt.NaturalLess(d1, d2, true) {
						return hdr.header.SortState().Ascending
					}
					return !hdr.header.SortState().Ascending
				}
			}
			return i < j
		})
		for _, row := range rows {
			if row.CanHaveChildRows() {
				h.applySort(headers, row.ChildRows())
			}
		}
	}
}
