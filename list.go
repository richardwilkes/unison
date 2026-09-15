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
	"slices"
	"time"

	"github.com/richardwilkes/toolbox/v2/collection/bitset"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/role"
)

// DefaultListTheme holds the default ListTheme values for Lists. Modifying this data will not alter existing Lists,
// but will alter any Lists created in the future.
var DefaultListTheme = ListTheme{
	BackgroundInk:          ThemeBelowSurface,
	OnBackgroundInk:        ThemeOnBelowSurface,
	BandingInk:             ThemeBanding,
	OnBandingInk:           ThemeOnBanding,
	SelectionInk:           ThemeFocus,
	OnSelectionInk:         ThemeOnFocus,
	InactiveSelectionInk:   ThemeDeepFocus,
	OnInactiveSelectionInk: ThemeOnDeepFocus,
	FlashAnimationTime:     100 * time.Millisecond,
}

// ListTheme holds theming data for a List.
type ListTheme struct {
	BackgroundInk          Ink
	OnBackgroundInk        Ink
	BandingInk             Ink
	OnBandingInk           Ink
	SelectionInk           Ink
	OnSelectionInk         Ink
	InactiveSelectionInk   Ink
	OnInactiveSelectionInk Ink
	FlashAnimationTime     time.Duration
}

// List provides a control that allows the user to select from a list of items, represented by cells.
type List[T any] struct {
	DoubleClickCallback  func()
	NewSelectionCallback func()
	Factory              CellFactory
	Selection            *bitset.BitSet
	savedSelection       *bitset.BitSet
	rows                 []T
	ListTheme
	Panel
	anchor            int
	lastSel           int
	allowMultiple     bool
	pressed           bool
	suppressSelection bool
	suppressScroll    bool
	wasDragged        bool
}

// NewList creates a new List control.
func NewList[T any]() *List[T] {
	l := &List[T]{
		ListTheme:      DefaultListTheme,
		Factory:        &DefaultCellFactory{},
		Selection:      &bitset.BitSet{},
		savedSelection: &bitset.BitSet{},
		anchor:         -1,
		lastSel:        -1,
		allowMultiple:  true,
	}
	l.Self = l
	l.SetFocusable(true)
	l.SetSizer(l.DefaultSizes)
	l.DrawCallback = l.DefaultDraw
	l.GainedFocusCallback = l.DefaultFocusGained
	l.MouseDownCallback = l.DefaultMouseDown
	l.MouseDragCallback = l.DefaultMouseDrag
	l.MouseUpCallback = l.DefaultMouseUp
	l.KeyDownCallback = l.DefaultKeyDown
	l.InstallCmdHandlers(SelectAllItemID, func(_ any) bool { return l.CanSelectAll() }, func(_ any) { l.SelectAll() })
	return l
}

// Count returns the number of rows.
func (l *List[T]) Count() int {
	return len(l.rows)
}

// DataAtIndex returns the data for the specified row index.
func (l *List[T]) DataAtIndex(index int) T {
	if index >= 0 && index < len(l.rows) {
		return l.rows[index]
	}
	var zero T
	return zero
}

// Append values to the list of items.
func (l *List[T]) Append(values ...T) {
	l.rows = append(l.rows, values...)
	l.MarkForLayoutAndRedraw()
}

// Insert values at the specified index.
func (l *List[T]) Insert(index int, values ...T) {
	if index < 0 || index > len(l.rows) {
		index = len(l.rows)
	}
	l.rows = append(l.rows[:index], append(values, l.rows[index:]...)...)
	i := l.Selection.LastSet() + 1
	if i >= index {
		delta := len(values)
		for {
			if i = l.Selection.PreviousSet(i); i == -1 || i < index {
				break
			}
			l.Selection.Set(i + delta)
			l.Selection.Clear(i)
		}
	}
	if l.anchor >= index {
		l.anchor += len(values)
	}
	l.MarkForLayoutAndRedraw()
}

// Replace the value at the specified index.
func (l *List[T]) Replace(index int, value T) {
	if index >= 0 && index < len(l.rows) {
		l.rows[index] = value
		l.MarkForLayoutAndRedraw()
	}
}

// Clear the list of items.
func (l *List[T]) Clear() {
	l.rows = nil
	l.Selection.Reset()
	l.anchor = -1
	l.MarkForLayoutAndRedraw()
}

// Remove the item at the specified index.
func (l *List[T]) Remove(index int) {
	if index >= 0 && index < len(l.rows) {
		l.rows = slices.Delete(l.rows, index, index+1)
		l.Selection.Clear(index)
		switch {
		case l.anchor == index:
			l.anchor = -1
		case l.anchor > index:
			l.anchor--
		}
		for {
			if index = l.Selection.NextSet(index); index == -1 {
				break
			}
			l.Selection.Set(index - 1)
			l.Selection.Clear(index)
		}
		l.MarkForLayoutAndRedraw()
	}
}

// RemoveRange removes the items at the specified index range, inclusive.
func (l *List[T]) RemoveRange(from, to int) {
	if from >= 0 && from < len(l.rows) && to >= from && to < len(l.rows) {
		l.rows = slices.Delete(l.rows, from, to+1)
		l.Selection.ClearRange(from, to)
		delta := to - from + 1
		switch {
		case l.anchor >= from && l.anchor <= to:
			l.anchor = -1
		case l.anchor > to:
			l.anchor -= delta
		}
		for {
			if from = l.Selection.NextSet(from); from == -1 {
				break
			}
			l.Selection.Set(from - delta)
			l.Selection.Clear(from)
		}
		l.MarkForLayoutAndRedraw()
	}
}

// DefaultSizes provides the default sizing.
func (l *List[T]) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	maxSize = MaxSize(maxSize)
	height := xmath.Ceil(l.Factory.CellHeight())
	if height < 1 {
		height = 0
	}
	size := geom.NewSize(hint.Width, height)
	for row := range l.rows {
		cell := l.cell(row)
		_, cPref, cMax := cell.Sizes(size)
		cPref = cPref.Ceil()
		cMax = cMax.Ceil()
		if prefSize.Width < cPref.Width {
			prefSize.Width = cPref.Width
		}
		if maxSize.Width < cMax.Width {
			maxSize.Width = cMax.Width
		}
		if height < 1 {
			prefSize.Height += cPref.Height
			maxSize.Height += cMax.Height
		}
	}
	if height >= 1 {
		count := float32(len(l.rows))
		if count < 1 {
			count = 1
		}
		prefSize.Height = count * height
		maxSize.Height = count * height
		if maxSize.Height < DefaultMaxSize {
			maxSize.Height = DefaultMaxSize
		}
	}
	if border := l.Border(); border != nil {
		insets := border.Insets().Size()
		prefSize = prefSize.Add(insets)
		maxSize = maxSize.Add(insets)
	}
	prefSize = prefSize.Ceil()
	return prefSize, prefSize, maxSize.Ceil()
}

// DefaultFocusGained provides the default focus gained handling.
func (l *List[T]) DefaultFocusGained() {
	if !l.suppressScroll {
		l.ScrollIntoView()
	}
	l.MarkForRedraw()
}

func (l *List[T]) cellParams(row int) (fg, bg Ink, selected, focused bool) {
	focused = l.Focused()
	if !l.suppressSelection {
		selected = l.Selection.State(row)
	}
	switch {
	case selected && focused && l.Enabled():
		fg = l.OnSelectionInk
		bg = l.SelectionInk
	case selected:
		fg = l.OnInactiveSelectionInk
		bg = l.InactiveSelectionInk
	case row%2 == 1:
		fg = l.OnBandingInk
		bg = l.BandingInk
	default:
		fg = l.OnBackgroundInk
		bg = l.BackgroundInk
	}
	return fg, bg, selected, focused
}

func (l *List[T]) cell(row int) *Panel {
	fg, bg, selected, focused := l.cellParams(row)
	return l.Factory.CreateCell(l, l.rows[row], row, fg, bg, selected, focused).AsPanel()
}

// RowRect returns the rectangle for the specified row.
func (l *List[T]) RowRect(row int) geom.Rect {
	if row < 0 || row >= len(l.rows) {
		return geom.Rect{}
	}
	rect := l.ContentRect(false)
	cellHeight := xmath.Ceil(l.Factory.CellHeight())
	if cellHeight < 1 {
		// Rows may each have a different height, so the top of this row is the sum of the heights of the rows that
		// precede it, not a multiple of this row's own height.
		for i := range row {
			_, pref, _ := l.cell(i).Sizes(geom.Size{})
			rect.Y += pref.Ceil().Height
		}
		_, pref, _ := l.cell(row).Sizes(geom.Size{})
		cellHeight = pref.Ceil().Height
	} else {
		rect.Y += cellHeight * float32(row)
	}
	rect.Height = cellHeight
	return rect
}

// DefaultDraw provides the default drawing.
func (l *List[T]) DefaultDraw(canvas *Canvas, dirty geom.Rect) {
	rect := l.ContentRect(false)
	intersect := rect.Intersect(dirty)
	backgroundPaint := l.BackgroundInk.Paint(canvas, intersect, paintstyle.Fill)
	canvas.DrawRect(intersect, backgroundPaint)
	row, y := l.rowAt(dirty.Y)
	if row >= 0 {
		cellHeight := xmath.Ceil(l.Factory.CellHeight())
		count := len(l.rows)
		yMax := dirty.Y + dirty.Height
		for row < count && y < yMax {
			fg, bg, selected, focused := l.cellParams(row)
			cell := l.Factory.CreateCell(l, l.rows[row], row, fg, bg, selected, focused).AsPanel()
			cellRect := geom.NewRect(rect.X, y, rect.Width, cellHeight)
			if cellHeight < 1 {
				_, pref, _ := cell.Sizes(geom.Size{})
				cellRect.Height = pref.Ceil().Height
			}
			cell.SetFrameRect(cellRect)
			y += cellRect.Height
			r := geom.NewRect(rect.X, cellRect.Y, rect.Width, cellRect.Height)
			paint := bg.Paint(canvas, r, paintstyle.Fill)
			canvas.DrawRect(r, paint)
			canvas.Save()
			tl := cellRect.Point
			dirty.Point = dirty.Point.Sub(tl)
			canvas.Translate(cellRect.Point)
			cellRect.X = 0
			cellRect.Y = 0
			cell.Draw(canvas, dirty)
			dirty.Point = dirty.Point.Add(tl)
			canvas.Restore()
			row++
		}
	}
}

// DefaultMouseDown provides the default mouse down handling.
func (l *List[T]) DefaultMouseDown(where geom.Point, _, clickCount int, mods mod.Modifiers) bool {
	l.suppressScroll = true
	l.RequestFocus()
	l.suppressScroll = false
	l.savedSelection = l.Selection.Clone()
	l.lastSel = -1
	l.wasDragged = false
	if index, _ := l.rowAt(where.Y); index >= 0 {
		switch {
		case mods.DiscontiguousSelectionDown():
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
		case mods.ShiftDown():
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
			l.lastSel = index
			l.anchor = index
			if clickCount == 2 && l.DoubleClickCallback != nil {
				SafeCall(l.DoubleClickCallback)
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
func (l *List[T]) DefaultMouseDrag(where geom.Point, _ int, mods mod.Modifiers) bool {
	if l.pressed {
		l.wasDragged = true
		l.Selection.Copy(l.savedSelection)
		if index, _ := l.rowAt(where.Y); index >= 0 {
			if l.allowMultiple {
				if l.anchor == -1 {
					l.anchor = index
				}
				switch {
				case mods.DiscontiguousSelectionDown():
					l.Selection.FlipRange(l.anchor, index)
				case mods.ShiftDown():
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
func (l *List[T]) DefaultMouseUp(_ geom.Point, _ int, _ mod.Modifiers) bool {
	if l.pressed {
		l.pressed = false
		if !l.wasDragged && l.lastSel != -1 {
			l.Selection.Reset()
			l.Selection.Set(l.lastSel)
			l.anchor = l.lastSel
			l.MarkForRedraw()
		}
		if l.NewSelectionCallback != nil && !l.Selection.Equal(l.savedSelection) {
			SafeCall(l.NewSelectionCallback)
		}
	}
	l.savedSelection = nil
	return true
}

// DefaultKeyDown provides the default key down handling.
func (l *List[T]) DefaultKeyDown(keyCode KeyCode, mods mod.Modifiers, _repeat bool) bool {
	if IsControlAction(keyCode, mods) {
		if l.DoubleClickCallback != nil && l.Selection.Count() > 0 {
			SafeCall(l.DoubleClickCallback)
		}
		return true
	}
	switch keyCode {
	case KeyUp:
		var first int
		if l.Selection.Count() == 0 {
			first = len(l.rows) - 1
		} else {
			first = max(l.Selection.FirstSet()-1, 0)
		}
		l.Select(mods.ShiftDown(), first)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(first))
	case KeyDown:
		last := l.Selection.LastSet() + 1
		if last >= len(l.rows) {
			last = len(l.rows) - 1
		}
		l.Select(mods.ShiftDown(), last)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(last))
	case KeyHome:
		l.Select(mods.ShiftDown(), 0)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(0))
	case KeyEnd:
		l.Select(mods.ShiftDown(), len(l.rows)-1)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(len(l.rows) - 1))
	default:
		return false
	}
	return true
}

// CanSelectAll returns true if the list's selection can be expanded.
func (l *List[T]) CanSelectAll() bool {
	return l.Selection.Count() < len(l.rows)
}

// SelectAll selects all of the rows in the list.
func (l *List[T]) SelectAll() {
	l.SelectRange(0, len(l.rows)-1, false)
}

// SelectRange selects items from 'start' to 'end', inclusive. If 'add' is true, then any existing selection is added to
// rather than replaced.
func (l *List[T]) SelectRange(start, end int, add bool) {
	maximum := len(l.rows) - 1
	if maximum < 0 {
		// With no rows, the clamps below would produce SetRange(0, 0), creating a phantom selection at index 0.
		return
	}
	if !l.allowMultiple {
		add = false
		end = start
	}
	if !add {
		l.Selection.Reset()
		l.anchor = -1
	}
	start = max(min(start, maximum), 0)
	end = max(min(end, maximum), 0)
	l.Selection.SetRange(start, end)
	if l.anchor == -1 || !l.allowMultiple {
		l.anchor = start
	}
	l.MarkForRedraw()
}

// Select items at the specified indexes. If 'add' is true, then any existing selection is added to rather than
// replaced.
func (l *List[T]) Select(add bool, index ...int) {
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
	maximum := len(l.rows)
	for _, v := range index {
		if v >= 0 && v < maximum {
			l.Selection.Set(v)
			if l.anchor == -1 {
				l.anchor = v
			}
		}
	}
	l.MarkForRedraw()
}

// Anchor returns the index that is the current anchor point. Will be -1 if there is no anchor point.
func (l *List[T]) Anchor() int {
	return l.anchor
}

// AllowMultipleSelection returns whether multiple rows may be selected at once.
func (l *List[T]) AllowMultipleSelection() bool {
	return l.allowMultiple
}

// SetAllowMultipleSelection sets whether multiple rows may be selected at once.
func (l *List[T]) SetAllowMultipleSelection(allow bool) *List[T] {
	l.allowMultiple = allow
	if !allow && l.Selection.Count() > 1 {
		i := l.anchor
		if i < 0 || i >= l.Count() {
			i = l.Selection.FirstSet()
		}
		l.Select(false, i)
	}
	// Marked unconditionally, and not only on the path above that alters the selection: whether more than one row may
	// be selected is part of what an assistive technology is told, both on the list and on every row of it, and a
	// window is only described again once it has been drawn, so the change would otherwise not be published until
	// something unrelated happened to redraw.
	l.MarkForRedraw()
	return l
}

func (l *List[T]) rowAt(y float32) (row int, top float32) {
	count := len(l.rows)
	top = l.ContentRect(false).Y
	cellHeight := xmath.Ceil(l.Factory.CellHeight())
	if cellHeight < 1 {
		for row < count {
			_, pref, _ := l.cell(row).Sizes(geom.Size{})
			pref = pref.Ceil()
			if top+pref.Height > y {
				break
			}
			top += pref.Height
			row++
		}
	} else {
		// y can be above the content rect (e.g. within a top border inset when redrawing the full widget), which would
		// otherwise produce a negative row and cause DefaultDraw to skip all rows.
		row = max(int(xmath.Floor((y-top)/cellHeight)), 0)
		top += float32(row) * cellHeight
	}
	if row >= count {
		row = -1
		top = 0
	}
	return row, top
}

// ProvideAccessibility describes the list to assistive technologies. The rows have no panels of their own — a cell is
// created to draw a row and thrown away again — so each one is described directly, as a virtual child keyed by its
// index. Only the rows that can be seen, plus the ones that are selected, are described: a list may hold far more rows
// than it shows, and an assistive technology is interested in what is on the screen and in what the selection is.
func (l *List[T]) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		node.Role = role.List
	}
	node.Multiselectable = l.allowMultiple
	node.RowCount = len(l.rows)
	// Pressing the list is not activating it; the default behavior would synthesize a click at the center of the list's
	// whole frame, which would replace the selection with whatever row happens to sit there — for a scrolled list, one
	// nowhere near what can be seen.
	node.Actions = node.Actions.Without(accessibility.Press)
	if len(l.rows) == 0 {
		return
	}
	reach := axReach(b.VisibleRect())
	if cellHeight := xmath.Ceil(l.Factory.CellHeight()); cellHeight >= 1 {
		l.axDescribeUniformRows(b, reach, cellHeight)
		return
	}
	l.axDescribeVaryingRows(b, reach)
}

// axDescribeUniformRows describes the rows worth describing — those within reach of the part that can be seen (see
// axReach), plus the selection — when every row is the same height, which is when where a row sits is arithmetic rather
// than a walk through the rows before it.
func (l *List[T]) axDescribeUniformRows(b *AccessibilityBuilder, reach geom.Rect, cellHeight float32) {
	rect := l.ContentRect(false)
	first, last := 0, -1
	if !reach.Empty() {
		first = max(int(xmath.Floor((reach.Y-rect.Y)/cellHeight)), 0)
		last = min(int(xmath.Ceil((reach.Bottom()-rect.Y)/cellHeight)), len(l.rows)-1)
	}
	rowRect := geom.NewRect(rect.X, rect.Y, rect.Width, cellHeight)
	for row := first; row <= last; row++ {
		rowRect.Y = rect.Y + cellHeight*float32(row)
		l.axAddRow(b, row, rowRect, nil)
	}
	// The selected rows that were not reached above are described as well, however far out of sight they are.
	described := 0
	for row := l.Selection.FirstSet(); row >= 0 && described < axMaxSelectedRows; row = l.Selection.NextSet(row + 1) {
		if row >= len(l.rows) {
			break
		}
		if row >= first && row <= last {
			continue
		}
		rowRect.Y = rect.Y + cellHeight*float32(row)
		l.axAddRow(b, row, rowRect, nil)
		described++
	}
}

// axDescribeVaryingRows describes the rows worth describing when each row's height is its own. Every row has to be
// measured to know where the ones after it sit, and measuring means creating the cell, so the cell that was created is
// handed on to be named from rather than being created a second time.
//
// The walk stops as soon as nothing worth describing can be left: creating and laying out a panel for every row of a
// long list, up to twenty times a second, is exactly what VisibleRect exists to avoid, and is far more than drawing
// does, which stops at the bottom of the dirty rect.
func (l *List[T]) axDescribeVaryingRows(b *AccessibilityBuilder, reach geom.Rect) {
	rect := l.ContentRect(false)
	rowRect := geom.NewRect(rect.X, rect.Y, rect.Width, 0)
	described := 0
	for row := range l.rows {
		cell := l.cell(row)
		_, pref, _ := cell.Sizes(geom.Size{})
		rowRect.Height = pref.Ceil().Height
		switch {
		case rowRect.Intersects(reach):
			l.axAddRow(b, row, rowRect, cell)
		case l.Selection.State(row) && described < axMaxSelectedRows:
			l.axAddRow(b, row, rowRect, cell)
			described++
		}
		rowRect.Y += rowRect.Height
		if rowRect.Y > reach.Bottom() && (described >= axMaxSelectedRows || l.Selection.NextSet(row+1) < 0) {
			// Every row from here down starts below the reach, so none of them can be seen, and either there is no
			// selected row left to describe or as many of them as will be described have been.
			break
		}
	}
}

// axAddRow describes one row of the list. cell, when not nil, is the cell that was already created for the row, whose
// text is what the row is named by; one is created if it is nil.
func (l *List[T]) axAddRow(b *AccessibilityBuilder, row int, rect geom.Rect, cell *Panel) {
	if cell == nil {
		cell = l.cell(row)
	}
	name := axLabelText(cell)
	selected := l.Selection.State(row)
	b.AddVirtualChild(row, func(n *accessibility.Node) {
		n.Role = role.ListItem
		n.Name = name
		n.Bounds = rect
		n.RowIndex = row
		n.Selectable = true
		n.Selected = selected
		n.Actions = n.Actions.With(accessibility.Select, accessibility.ScrollIntoView)
		if l.allowMultiple {
			// A list that holds one row at a time has nothing to add to or take out of: Select on such a list replaces
			// whatever was selected, so offering to add to the selection would be offering something that quietly does
			// the opposite of what it says.
			n.Actions = n.Actions.With(accessibility.AddToSelection, accessibility.RemoveFromSelection)
		}
		if l.DoubleClickCallback != nil {
			// Pressing a row is opening it, which is what a double-click and the Return key do, so it is offered only
			// when there is something for it to do.
			n.Actions = n.Actions.With(accessibility.Press)
		}
	})
}

// PerformAccessibilityAction carries out a request from an assistive technology. Every request that reaches here names
// one of the rows described by ProvideAccessibility, which arrives as the index that row was keyed by. Selecting a row
// also scrolls it into view, as the arrow keys do, since an assistive technology moving through the rows selects each
// one as it goes and expects to see where it has got to. Pressing a row opens it, which is the gesture a double-click
// and the Return key stand for.
func (l *List[T]) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	row, ok := req.Key.(int)
	if !ok || row < 0 || row >= len(l.rows) {
		return false
	}
	switch req.Action {
	case accessibility.Press:
		if l.DoubleClickCallback == nil {
			return false
		}
		// Both of the gestures this stands for act on the selection: a double-click has already selected the row with
		// its first click, and the Return key runs against whatever is selected. So the row is selected first when it
		// was not already, and the callback then finds what it expects.
		if !l.Selection.State(row) {
			l.Select(false, row)
			SafeCall(l.NewSelectionCallback)
		}
		SafeCall(l.DoubleClickCallback)
	case accessibility.Select:
		// The callback is for a selection that actually changed, exactly as it is for a click: an assistive technology
		// moving through the rows re-selects the row it is already on often enough — landing on it, then acting on it —
		// and an application told its selection changed sets about whatever it does when that happens. The selection is
		// still made either way, since it also puts the anchor a later shift-click extends from on the row.
		changed := !l.Selection.State(row) || l.Selection.Count() != 1
		l.Select(false, row)
		if changed {
			SafeCall(l.NewSelectionCallback)
		}
		l.ScrollRectIntoView(l.RowRect(row))
	case accessibility.AddToSelection:
		if !l.allowMultiple {
			// Select(true, row) on a list that holds one row at a time replaces the selection rather than adding to
			// it, so carrying this out would quietly do the opposite of what it says. The rows of such a list are not
			// described as offering it, but this method is exported and the dispatcher does not check that an action
			// was advertised, so it is refused here too, the way Press refuses a list with nothing to open a row with.
			// Taking a row out of the selection is left alone: that does exactly what it says whether or not more than
			// one row may be selected.
			return false
		}
		changed := !l.Selection.State(row)
		l.Select(true, row)
		// The row a later shift-click extends from is put on this row, which Select does only when there was no anchor
		// at all. This request stands for the ctrl-click that adds a row to the selection, and DefaultMouseDown's
		// DiscontiguousSelectionDown branch moves the anchor to the row it touched every time, so a shift-click after
		// an assistive technology added a row has to extend from the same place it would have after the click.
		l.anchor = row
		if changed {
			SafeCall(l.NewSelectionCallback)
		}
		l.ScrollRectIntoView(l.RowRect(row))
	case accessibility.RemoveFromSelection:
		if !l.Selection.State(row) {
			return true
		}
		l.Selection.Clear(row)
		// As for adding: the ctrl-click this stands for leaves the anchor on the row it touched whether it added the
		// row to the selection or took it out, so taking a row out must not leave the list with no anchor at all —
		// a shift-click after that would select only the row it landed on rather than extending from here.
		l.anchor = row
		l.MarkForRedraw()
		SafeCall(l.NewSelectionCallback)
	case accessibility.ScrollIntoView:
		l.ScrollRectIntoView(l.RowRect(row))
	default:
		return false
	}
	return true
}

// FlashSelection flashes the current selection.
func (l *List[T]) FlashSelection() {
	l.suppressSelection = true
	l.MarkForRedraw()
	l.FlushDrawing()
	time.Sleep(l.FlashAnimationTime)
	l.suppressSelection = false
	l.MarkForRedraw()
	l.FlushDrawing()
	time.Sleep(l.FlashAnimationTime)
}
