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
	"time"

	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

// List provides a control that allows the user to select from a list of items, represented by cells.
type List struct {
	Panel
	DoubleClickCallback  func()
	NewSelectionCallback func()
	PressedColor         Ink
	OnPressedColor       Ink
	RowColor             Ink
	OnRowColor           Ink
	AltRowColor          Ink
	OnAltRowColor        Ink
	Factory              CellFactory
	FlashAnimationTime   time.Duration
	rows                 []interface{}
	Selection            *xmath.BitSet
	savedSelection       *xmath.BitSet
	anchor               int
	allowMultiple        bool
	pressed              bool
	suppressSelection    bool
}

// NewList creates a new List control.
func NewList() *List {
	l := &List{
		Factory:            &DefaultCellFactory{},
		FlashAnimationTime: 100 * time.Millisecond,
		Selection:          &xmath.BitSet{},
		savedSelection:     &xmath.BitSet{},
		anchor:             -1,
		allowMultiple:      true,
	}
	l.Self = l
	l.SetFocusable(true)
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	l.MouseDownCallback = l.DefaultMouseDown
	l.MouseDragCallback = l.DefaultMouseDrag
	l.MouseUpCallback = l.DefaultMouseUp
	l.KeyDownCallback = l.DefaultKeyDown
	l.CanPerformCmdCallback = l.DefaultCanPerformCmd
	l.PerformCmdCallback = l.DefaultPerformCmd
	return l
}

// Count returns the number of rows.
func (l *List) Count() int {
	return len(l.rows)
}

// DataAtIndex returns the data for the specified row index.
func (l *List) DataAtIndex(index int) interface{} {
	if index >= 0 && index < len(l.rows) {
		return l.rows[index]
	}
	return nil
}

// Append values to the list of items.
func (l *List) Append(values ...interface{}) {
	l.rows = append(l.rows, values...)
	l.MarkForLayoutAndRedraw()
}

// Insert values at the specified index.
func (l *List) Insert(index int, values ...interface{}) {
	if index < 0 || index > len(l.rows) {
		index = len(l.rows)
	}
	l.rows = append(l.rows[:index], append(values, l.rows[index:]...)...)
	l.MarkForLayoutAndRedraw()
}

// Replace the value at the specified index.
func (l *List) Replace(index int, value interface{}) {
	if index >= 0 && index < len(l.rows) {
		l.rows[index] = value
		l.MarkForLayoutAndRedraw()
	}
}

// Remove the item at the specified index.
func (l *List) Remove(index int) {
	if index >= 0 && index < len(l.rows) {
		copy(l.rows[index:], l.rows[index+1:])
		size := len(l.rows) - 1
		l.rows[size] = nil
		l.rows = l.rows[:size]
		l.MarkForLayoutAndRedraw()
	}
}

// RemoveRange removes the items at the specified index range, inclusive.
func (l *List) RemoveRange(from, to int) {
	if from >= 0 && from < len(l.rows) && to >= from && to < len(l.rows) {
		copy(l.rows[from:], l.rows[to+1:])
		size := len(l.rows) - (1 + to - from)
		for i := size; i < len(l.rows); i++ {
			l.rows[i] = nil
		}
		l.rows = l.rows[:size]
		l.MarkForLayoutAndRedraw()
	}
}

// DefaultSizes provides the default sizing.
func (l *List) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	max = MaxSize(max)
	height := mathf32.Ceil(l.Factory.CellHeight())
	if height < 1 {
		height = 0
	}
	size := geom32.Size{Width: hint.Width, Height: height}
	for i, row := range l.rows {
		cell := l.Factory.CreateCell(l, row, i, Black, false, false)
		_, cPref, cMax := cell.Sizes(size)
		cPref.GrowToInteger()
		cMax.GrowToInteger()
		if pref.Width < cPref.Width {
			pref.Width = cPref.Width
		}
		if max.Width < cMax.Width {
			max.Width = cMax.Width
		}
		if height < 1 {
			pref.Height += cPref.Height
			max.Height += cMax.Height
		}
	}
	if height >= 1 {
		count := float32(len(l.rows))
		if count < 1 {
			count = 1
		}
		pref.Height = count * height
		max.Height = count * height
		if max.Height < DefaultMaxSize {
			max.Height = DefaultMaxSize
		}
	}
	if border := l.Border(); border != nil {
		insets := border.Insets()
		pref.AddInsets(insets)
		max.AddInsets(insets)
	}
	pref.GrowToInteger()
	max.GrowToInteger()
	return pref, pref, max
}

// DefaultDraw provides the default drawing.
func (l *List) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	index, y := l.rowAt(dirty.Y)
	if index >= 0 {
		cellHeight := mathf32.Ceil(l.Factory.CellHeight())
		count := len(l.rows)
		yMax := dirty.Y + dirty.Height
		focused := l.Focused()
		var selCount int
		if !l.suppressSelection {
			selCount = l.Selection.Count()
		}
		rect := l.ContentRect(false)
		for index < count && y < yMax {
			var selected bool
			if !l.suppressSelection {
				selected = l.Selection.State(index)
			}
			var fg, bg Ink
			switch {
			case selected:
				bg = ChooseInk(l.PressedColor, SelectionColor)
				fg = ChooseInk(l.OnPressedColor, OnSelectionColor)
			case index%2 == 0:
				bg = ChooseInk(l.RowColor, ContentColor)
				fg = ChooseInk(l.OnRowColor, OnContentColor)
			default:
				bg = ChooseInk(l.AltRowColor, BandingColor)
				fg = ChooseInk(l.OnAltRowColor, OnBandingColor)
			}
			cell := l.Factory.CreateCell(l, l.rows[index], index, fg, selected, focused && selected && selCount == 1)
			cellRect := geom32.Rect{Point: geom32.Point{X: rect.X, Y: y}, Size: geom32.Size{Width: rect.Width, Height: cellHeight}}
			if cellHeight < 1 {
				_, pref, _ := cell.Sizes(geom32.Size{})
				pref.GrowToInteger()
				cellRect.Height = pref.Height
			}
			cell.SetFrameRect(cellRect)
			y += cellRect.Height
			r := geom32.NewRect(rect.X, cellRect.Y, rect.Width, cellRect.Height)
			canvas.DrawRect(r, bg.Paint(canvas, r, Fill))
			canvas.Save()
			tl := cellRect.Point
			dirty.Point.Subtract(tl)
			canvas.Translate(cellRect.X, cellRect.Y)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, dirty)
			dirty.Point.Add(tl)
			canvas.Restore()
			index++
		}
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (l *List) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	l.RequestFocus()
	l.savedSelection = l.Selection.Clone()
	if index, _ := l.rowAt(where.Y); index >= 0 {
		switch {
		case mod.CommandDown():
			if l.allowMultiple {
				l.Selection.Flip(index)
			} else {
				wasSet := l.Selection.State(index)
				l.Selection.Reset()
				if !wasSet {
					l.Selection.Set(index)
				}
			}
			l.anchor = index
		case mod.ShiftDown():
			if l.allowMultiple {
				if l.anchor != -1 {
					l.Selection.SetRange(l.anchor, index)
				} else {
					l.Selection.Set(index)
					l.anchor = index
				}
			} else {
				l.Selection.Reset()
				l.Selection.Set(index)
			}
		case l.Selection.State(index):
			l.anchor = index
			if clickCount == 2 && l.DoubleClickCallback != nil {
				l.DoubleClickCallback()
				return true
			}
		default:
			l.Selection.Reset()
			l.Selection.Set(index)
			l.anchor = index
		}
		if !l.Selection.Equal(l.savedSelection) {
			l.MarkForRedraw()
		}
	}
	l.pressed = true
	return true
}

// DefaultMouseDrag provides the default mouse drag handling.
func (l *List) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	if l.pressed {
		l.Selection.Copy(l.savedSelection)
		if index, _ := l.rowAt(where.Y); index >= 0 {
			if l.allowMultiple {
				if l.anchor == -1 {
					l.anchor = index
				}
				switch {
				case mod.CommandDown():
					l.Selection.FlipRange(l.anchor, index)
				case mod.ShiftDown():
					l.Selection.SetRange(l.anchor, index)
				default:
					l.Selection.Reset()
					l.Selection.SetRange(l.anchor, index)
				}
			} else {
				l.Selection.Reset()
				l.Selection.Set(index)
				l.anchor = index
			}
			if !l.Selection.Equal(l.savedSelection) {
				l.MarkForRedraw()
			}
		}
	}
	return true
}

// DefaultMouseUp provides the default mouse up handling.
func (l *List) DefaultMouseUp(where geom32.Point, button int, mod Modifiers) bool {
	if l.pressed {
		l.pressed = false
		if l.NewSelectionCallback != nil && !l.Selection.Equal(l.savedSelection) {
			l.NewSelectionCallback()
		}
	}
	l.savedSelection = nil
	return true
}

// DefaultKeyDown provides the default key down handling.
func (l *List) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if IsControlAction(keyCode, mod) {
		if l.DoubleClickCallback != nil && l.Selection.Count() > 0 {
			l.DoubleClickCallback()
		}
	} else {
		switch keyCode {
		case KeyUp, KeyNumPadUp:
			var first int
			if l.Selection.Count() == 0 {
				first = len(l.rows) - 1
			} else {
				first = l.Selection.FirstSet() - 1
				if first < 0 {
					first = 0
				}
			}
			l.Select(mod.ShiftDown(), first)
			if l.NewSelectionCallback != nil {
				l.NewSelectionCallback()
			}
		case KeyDown, KeyNumPadDown:
			last := l.Selection.LastSet() + 1
			if last >= len(l.rows) {
				last = len(l.rows) - 1
			}
			l.Select(mod.ShiftDown(), last)
			if l.NewSelectionCallback != nil {
				l.NewSelectionCallback()
			}
		case KeyHome, KeyNumPadHome:
			l.Select(mod.ShiftDown(), 0)
			if l.NewSelectionCallback != nil {
				l.NewSelectionCallback()
			}
		case KeyEnd, KeyNumPadEnd:
			l.Select(mod.ShiftDown(), len(l.rows)-1)
			if l.NewSelectionCallback != nil {
				l.NewSelectionCallback()
			}
		default:
			return false
		}
	}
	return true
}

// DefaultCanPerformCmd provides the default can perform cmd handling.
func (l *List) DefaultCanPerformCmd(source interface{}, id int) bool {
	return id == SelectAllItemID && l.Selection.Count() < len(l.rows)
}

// DefaultPerformCmd provides the default perform cmd handling.
func (l *List) DefaultPerformCmd(source interface{}, id int) {
	if id == SelectAllItemID {
		l.SelectRange(0, len(l.rows)-1, false)
	}
}

// SelectRange selects items from 'start' to 'end', inclusive. If 'add' is true, then any existing selection is added to
// rather than replaced.
func (l *List) SelectRange(start, end int, add bool) {
	if !l.allowMultiple {
		add = false
		end = start
	}
	if !add {
		l.Selection.Reset()
		l.anchor = -1
	}
	max := len(l.rows) - 1
	start = xmath.MaxInt(xmath.MinInt(start, max), 0)
	end = xmath.MaxInt(xmath.MinInt(end, max), 0)
	l.Selection.SetRange(start, end)
	if l.anchor == -1 || !l.allowMultiple {
		l.anchor = start
	}
	l.MarkForRedraw()
}

// Select items at the specified indexes. If 'add' is true, then any existing selection is added to rather than
// replaced.
func (l *List) Select(add bool, index ...int) {
	if !l.allowMultiple {
		add = false
		if len(index) > 0 {
			index = index[len(index)-1:]
		}
	}
	if !add {
		l.Selection.Reset()
		l.anchor = -1
	}
	max := len(l.rows)
	for _, v := range index {
		if v >= 0 && v < max {
			l.Selection.Set(v)
			if l.anchor == -1 {
				l.anchor = v
			}
		}
	}
	l.MarkForRedraw()
}

// Anchor returns the index that is the current anchor point. Will be -1 if there is no anchor point.
func (l *List) Anchor() int {
	return l.anchor
}

// AllowMultipleSelection returns whether multiple rows may be selected at once.
func (l *List) AllowMultipleSelection() bool {
	return l.allowMultiple
}

// SetAllowMultipleSelection sets whether multiple rows may be selected at once.
func (l *List) SetAllowMultipleSelection(allow bool) *List {
	l.allowMultiple = allow
	if !allow && l.Selection.Count() > 1 {
		i := l.anchor
		if i < 0 || i >= l.Count() {
			i = l.Selection.FirstSet()
		}
		l.Select(false, i)
	}
	return l
}

func (l *List) rowAt(y float32) (index int, top float32) {
	count := len(l.rows)
	top = l.ContentRect(false).Y
	cellHeight := mathf32.Ceil(l.Factory.CellHeight())
	if cellHeight < 1 {
		for index < count {
			cell := l.Factory.CreateCell(l, l.rows[index], index, Black, false, false)
			_, pref, _ := cell.Sizes(geom32.Size{})
			pref.GrowToInteger()
			if top+pref.Height >= y {
				break
			}
			top += pref.Height
			index++
		}
	} else {
		index = int(mathf32.Floor((y - top) / cellHeight))
		top += float32(index) * cellHeight
	}
	if index >= count {
		index = -1
		top = 0
	}
	return index, top
}

// FlashSelection flashes the current selection.
func (l *List) FlashSelection() {
	l.suppressSelection = true
	l.MarkForRedraw()
	l.FlushDrawing()
	time.Sleep(l.FlashAnimationTime)
	l.suppressSelection = false
	l.MarkForRedraw()
	l.FlushDrawing()
	time.Sleep(l.FlashAnimationTime)
}
