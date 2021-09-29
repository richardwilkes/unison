// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
	"github.com/richardwilkes/unison/fa"
)

const (
	// DefaultTableIndent is the default amount of space to indent for each level of depth in the hierarchy.
	DefaultTableIndent = 16
	// DefaultMinimumRowHeight is the default minimum height a row is permitted to have.
	DefaultMinimumRowHeight = 16
)

// TableRowData provides information about a single row of data.
type TableRowData interface {
	// CanHaveChildRows returns true if this row can have children, even if it currently does not have any.
	CanHaveChildRows() bool
	// ChildRows returns the child rows.
	ChildRows() []TableRowData
	// ColumnCell returns the panel that should be placed at the position of the cell for the given column index.
	ColumnCell(index int) Paneler
	// IsOpen returns true if the row can have children and is currently showing its children.
	IsOpen() bool
	// SetOpen sets the row's open state.
	SetOpen(open bool)
}

type tableCache struct {
	data   TableRowData
	depth  int
	height float32
}

type tableHitRect struct {
	geom32.Rect
	handler func(where geom32.Point, button, clickCount int, mod Modifiers)
}

// Table provides a control that can display data in columns and rows.
type Table struct {
	Panel
	DividerColor         Ink
	RowColor             Ink
	OnRowColor           Ink
	AltRowColor          Ink
	OnAltRowColor        Ink
	PressedColor         Ink
	OnPressedColor       Ink
	topLevelRows         []TableRowData
	selection            *xmath.BitSet
	HierarchyColumnIndex int       // The column index that will display the hierarchy
	ColumnWidths         []float32 // The widths of each column
	hitRects             []tableHitRect
	rowCache             []tableCache
	TableIndent          float32
	MinimumRowHeight     float32
	ShowRowDivider       bool
	ShowColumnDivider    bool
}

// NewTable creates a new Table control.
func NewTable() *Table {
	t := &Table{
		selection:         &xmath.BitSet{},
		TableIndent:       DefaultTableIndent,
		MinimumRowHeight:  DefaultMinimumRowHeight,
		ShowRowDivider:    true,
		ShowColumnDivider: true,
	}
	t.Self = t
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	t.MouseDownCallback = t.DefaultMouseDown
	return t
}

// DefaultDraw provides the default drawing.
func (t *Table) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	var insets geom32.Insets
	if border := t.Border(); border != nil {
		insets = border.Insets()
	}

	var firstCol int
	x := insets.Left
	for i, w := range t.ColumnWidths {
		x1 := x + w
		if t.ShowColumnDivider {
			x1++
		}
		if x1 >= dirty.X {
			break
		}
		x = x1
		firstCol = i + 1
	}

	var firstRow int
	y := insets.Top
	rowCount := len(t.rowCache)
	for i := 0; i < rowCount; i++ {
		y1 := y + t.rowCache[i].height
		if t.ShowRowDivider {
			y1++
		}
		if y1 >= dirty.Y {
			break
		}
		y = y1
		firstRow = i + 1
	}

	canvas.DrawRect(dirty, ChooseInk(t.RowColor, ListColor).Paint(canvas, dirty, Fill))

	lastY := dirty.Bottom()
	rect := dirty
	rect.Y = y
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		rect.Height = t.rowCache[r].height
		if t.selection.State(r) {
			canvas.DrawRect(rect, ChooseInk(t.PressedColor, ControlPressedColor).Paint(canvas, rect, Fill))
		} else if r%2 == 1 {
			canvas.DrawRect(rect, ChooseInk(t.AltRowColor, ListAltColor).Paint(canvas, rect, Fill))
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider && r != rowCount-1 {
			rect.Height = 1
			canvas.DrawRect(rect, ChooseInk(t.DividerColor, DividerColor).Paint(canvas, rect, Fill))
			rect.Y++
		}
	}

	if t.ShowColumnDivider {
		rect = dirty
		rect.X = x
		rect.Width = 1
		for c := firstCol; c < len(t.ColumnWidths)-1; c++ {
			rect.X += t.ColumnWidths[c]
			canvas.DrawRect(rect, ChooseInk(t.DividerColor, DividerColor).Paint(canvas, rect, Fill))
			rect.X++
		}
	}

	rect = dirty
	rect.Y = y
	lastX := dirty.Right()
	faFont := FontDescriptor{
		Family:  FontAwesomeFreeFamilyName,
		Size:    t.TableIndent - 8,
		Weight:  BlackFontWeight,
		Spacing: StandardSpacing,
		Slant:   NoSlant,
	}.Font()
	t.hitRects = nil
	for r := firstRow; r < rowCount && rect.Y < lastY; r++ {
		row := t.rowCache[r].data
		var fg Ink
		switch {
		case t.selection.State(r):
			fg = ChooseInk(t.OnPressedColor, OnControlPressedColor)
		case row.IsOpen():
			fg = ChooseInk(t.OnAltRowColor, OnListAltColor)
		default:
			fg = ChooseInk(t.OnRowColor, OnListColor)
		}
		rect.X = x
		rect.Height = t.rowCache[r].height
		for c := firstCol; c < len(t.ColumnWidths) && rect.X < lastX; c++ {
			rect.Width = t.ColumnWidths[c]
			cellRect := rect
			if c == t.HierarchyColumnIndex {
				if row.CanHaveChildRows() {
					var code string
					if row.IsOpen() {
						code = fa.ChevronCircleDown
					} else {
						code = fa.ChevronCircleRight
					}
					extents := faFont.Extents(code)
					canvas.DrawSimpleText(code, rect.X+(t.TableIndent-extents.Width)/2,
						rect.Y+(rect.Height-extents.Height)/2+faFont.Size(), faFont, fg.Paint(canvas, rect, Fill))
					t.hitRects = append(t.hitRects, t.newTableHitRect(rect, row))
				}
				indent := t.TableIndent * float32(t.rowCache[r].depth+1)
				cellRect.X += indent
				cellRect.Width -= indent
			}
			cell := row.ColumnCell(c).AsPanel()
			cell.SetFrameRect(rect)
			canvas.Save()
			canvas.Translate(cellRect.X, cellRect.Y)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, cellRect)
			canvas.Restore()
			rect.X += t.ColumnWidths[c]
			if t.ShowColumnDivider {
				rect.X++
			}
		}
		rect.Y += t.rowCache[r].height
		if t.ShowRowDivider {
			rect.Y++
		}
	}
}

func (t *Table) newTableHitRect(rect geom32.Rect, row TableRowData) tableHitRect {
	return tableHitRect{
		Rect: geom32.NewRect(rect.X, rect.Y+(rect.Height-t.TableIndent)/2, t.TableIndent, t.TableIndent),
		handler: func(where geom32.Point, button, clickCount int, mod Modifiers) {
			row.SetOpen(!row.IsOpen())
			t.SyncToModel()
			t.MarkForRedraw()
		},
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (t *Table) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	for _, one := range t.hitRects {
		if one.ContainsPoint(where) {
			one.handler(where, button, clickCount, mod)
			break
		}
	}
	return true
}

// SetTopLevelRows sets the top-level rows this table will display. This will call SyncToModel() automatically.
func (t *Table) SetTopLevelRows(rows []TableRowData) {
	t.topLevelRows = rows
	t.SyncToModel()
}

// SyncToModel causes the table to update its internal caches to reflect the current model.
func (t *Table) SyncToModel() {
	rowCount := 0
	for _, row := range t.topLevelRows {
		rowCount += t.countOpenRowChildrenRecursively(row)
	}
	t.rowCache = make([]tableCache, rowCount)
	j := 0
	for _, row := range t.topLevelRows {
		j = t.buildRowCacheEntry(row, j, 0)
	}
	_, pref, _ := t.DefaultSizes(geom32.Size{})
	rect := t.FrameRect()
	rect.Size = pref
	t.SetFrameRect(rect)
}

func (t *Table) countOpenRowChildrenRecursively(row TableRowData) int {
	count := 1
	if row.CanHaveChildRows() && row.IsOpen() {
		for _, child := range row.ChildRows() {
			count += t.countOpenRowChildrenRecursively(child)
		}
	}
	return count
}

func (t *Table) buildRowCacheEntry(row TableRowData, index, depth int) int {
	t.rowCache[index].data = row
	t.rowCache[index].depth = depth
	t.rowCache[index].height = t.heightForColumns(row)
	index++
	if row.CanHaveChildRows() && row.IsOpen() {
		for _, child := range row.ChildRows() {
			index = t.buildRowCacheEntry(child, index, depth+1)
		}
	}
	return index
}

func (t *Table) heightForColumns(row TableRowData) float32 {
	var height float32
	for i, w := range t.ColumnWidths {
		if w > 0 {
			_, cpref, _ := row.ColumnCell(i).AsPanel().Sizes(geom32.Size{Width: w})
			if height < cpref.Height {
				height = cpref.Height
			}
		}
	}
	return mathf32.Max(mathf32.Ceil(height), t.MinimumRowHeight)
}

// SizeColumnsToFit sizes each column to its preferred size.
func (t *Table) SizeColumnsToFit() {
	t.ColumnWidths = make([]float32, len(t.ColumnWidths))
	for _, cache := range t.rowCache {
		for i := range t.ColumnWidths {
			_, pref, _ := cache.data.ColumnCell(i).AsPanel().Sizes(geom32.Size{})
			if i == t.HierarchyColumnIndex {
				pref.Width += t.TableIndent * float32(cache.depth+1)
			}
			if t.ColumnWidths[i] < pref.Width {
				t.ColumnWidths[i] = pref.Width
			}
		}
	}
	for i, cache := range t.rowCache {
		t.rowCache[i].height = t.heightForColumns(cache.data)
	}
}

// DefaultSizes provides the default sizing.
func (t *Table) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	for _, w := range t.ColumnWidths {
		pref.Width += w
	}
	for _, cache := range t.rowCache {
		pref.Height += cache.height
	}
	if t.ShowColumnDivider {
		pref.Width += float32(len(t.ColumnWidths) - 1)
	}
	if t.ShowRowDivider {
		pref.Height += float32(len(t.rowCache) - 1)
	}
	if border := t.Border(); border != nil {
		pref.AddInsets(border.Insets())
	}
	pref.GrowToInteger()
	return pref, pref, pref
}
