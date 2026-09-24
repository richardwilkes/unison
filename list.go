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
	lead              int
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
		lead:           -1,
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
	if l.lead >= index {
		l.lead += len(values)
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
	l.lead = -1
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
		switch {
		case l.lead == index:
			l.lead = -1
		case l.lead > index:
			l.lead--
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
		switch {
		case l.lead >= from && l.lead <= to:
			l.lead = -1
		case l.lead > to:
			l.lead -= delta
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

// installCell attaches a cell to the list and lays it out at the row's frame, which is what it takes for the cell to
// be asked anything about where its content sits or to be handed a request for that content: a panel with no parent
// cannot find its window, and a panel that has not been laid out has all of its content at the origin. Drawing lays
// the cell out at that same frame — List.DefaultDraw sets the frame and validates the layout inline before translating
// the canvas to the row and handing the cell to Panel.Draw, which draws each child at its own frame — so what an
// assistive technology is told about where the content sits is where the content was painted. What drawing does not
// need is the parent, since it never has to find the window.
func (l *List[T]) installCell(cell *Panel, rect geom.Rect) {
	cell.SetFrameRect(rect)
	cell.ValidateLayout()
	cell.parent = l.AsPanel()
}

// uninstallCell detaches a cell that installCell attached. Unlike a table, a list keeps no cell: it builds one
// whenever it needs one and throws it away again, and it has nowhere to put one that a widget inside it has just
// handed the keyboard focus to — a list is one tab stop, the rows within it are not reachable with the keyboard, and
// the cell holding the focus would not even be the cell the next draw creates. So nothing is adopted here; see
// List.axPerformInCell, which puts the focus back on the list itself rather than leaving it on a panel that is about
// to be detached.
func (l *List[T]) uninstallCell(cell *Panel) {
	cell.parent = nil
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
			// Panel.Draw paints each child at its own frame, which on a cell that was just built is the zero rect, so
			// a cell holding more than one child has to be laid out before it is painted — at the very frame
			// installCell uses, so that what a row is described as holding and where sits where it is drawn.
			cell.ValidateLayout()
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
func (l *List[T]) DefaultMouseDown(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
	l.requestFocusWithoutScroll()
	l.savedSelection = l.Selection.Clone()
	l.lastSel = -1
	l.wasDragged = false
	if index, _ := l.rowAt(where.Y); index >= 0 {
		// Every branch below acts on the selection, including the one that only notes the row for the mouse up to
		// settle, so the row the person is on is this one whichever way the press is modified.
		l.setLead(index)
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
			// Only a left press is noted for the mouse up, which narrows the selection to this row on a click; any
			// other button leaves the selection alone, as a right-click on a list with a contextual menu does.
			l.anchor = index
			if button == ButtonLeft {
				l.lastSel = index
				if clickCount == 2 && l.DoubleClickCallback != nil {
					SafeCall(l.DoubleClickCallback)
					return true
				}
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
			l.setLead(index)
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

// DefaultMouseUp provides the default mouse up handling. A left click on a selected row narrows the selection to that
// row here, on the release, so that a drag that starts on it can carry the whole selection. A release outside the list
// is not a click and leaves the selection alone: the release that ends a press for a contextual menu arrives outside
// every panel (see Window.endPressesForContextMenu), and the menu then acts on the selection the person made.
func (l *List[T]) DefaultMouseUp(where geom.Point, _ int, _ mod.Modifiers) bool {
	if l.pressed {
		l.pressed = false
		if !l.wasDragged && l.lastSel != -1 && where.In(l.ContentRect(true)) {
			l.Selection.Reset()
			l.Selection.Set(l.lastSel)
			l.anchor = l.lastSel
			l.setLead(l.lastSel)
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
		l.setLead(first)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(first))
	case KeyDown:
		last := l.Selection.LastSet() + 1
		if last >= len(l.rows) {
			last = len(l.rows) - 1
		}
		l.Select(mods.ShiftDown(), last)
		l.setLead(last)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(last))
	case KeyHome:
		l.Select(mods.ShiftDown(), 0)
		l.setLead(0)
		SafeCall(l.NewSelectionCallback)
		l.ScrollRectIntoView(l.RowRect(0))
	case KeyEnd:
		l.Select(mods.ShiftDown(), len(l.rows)-1)
		l.setLead(len(l.rows) - 1)
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
	keep := l.lead
	l.SelectRange(0, len(l.rows)-1, false)
	if len(l.rows) == 0 {
		return
	}
	// The row the person is on is left where it was, since selecting everything does not move anyone: what it changes
	// is what else is selected around them. A lead that named no row at all lands on the first row, which is where a
	// list with everything selected and nowhere in particular to be reads from.
	if keep >= 0 && keep < len(l.rows) && l.Selection.State(keep) {
		l.setLead(keep)
	} else {
		l.setLead(0)
	}
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
	// The end of the range is where a selection made by dragging or by shift-arrowing has arrived, so that is the row
	// the person is on.
	l.setLead(end)
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
	lead := -1
	for _, v := range index {
		if v >= 0 && v < maximum {
			l.Selection.Set(v)
			if l.anchor == -1 {
				l.anchor = v
			}
			lead = v
		}
	}
	switch {
	case lead != -1:
		// The last row that was actually selected is the one the person has arrived on, which is what the row the
		// keyboard focus is reported on is worked out from.
		l.setLead(lead)
	case !add:
		// Nothing was selected and whatever had been is gone, so there is no row to be on.
		l.setLead(-1)
	}
	l.MarkForRedraw()
}

// Anchor returns the index that is the current anchor point. Will be -1 if there is no anchor point.
func (l *List[T]) Anchor() int {
	return l.anchor
}

// Lead returns the index of the lead row — the row the person is on, which is the last one a gesture or a call moved
// the selection to. Will be -1 if there is no lead row. It is what the keyboard focus is reported on, so that a screen
// reader lands on the row the person moved to rather than on the list as a whole, while it is still one of the selected
// rows; see List.axCurrentRow.
func (l *List[T]) Lead() int {
	return l.lead
}

// setLead moves the row the person is on. A change to it that leaves the selection alone — an arrow key on a list where
// the row moved to was selected already — still has to reach an assistive technology, and a window is described again
// only once it has been drawn, so the redraw is what republishes it.
func (l *List[T]) setLead(index int) {
	if l.lead == index {
		return
	}
	l.lead = index
	l.MarkForRedraw()
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
	current := l.axCurrentRow()
	var currentID accessibility.NodeID
	if cellHeight := xmath.Ceil(l.Factory.CellHeight()); cellHeight >= 1 {
		currentID = l.axDescribeUniformRows(b, reach, cellHeight, current)
	} else {
		currentID = l.axDescribeVaryingRows(b, reach, current)
	}
	if currentID != 0 {
		// The keyboard focus the list holds is reported on the row the person is on, which is where every native list
		// puts it and the only thing a screen reader follows: a container that keeps the focus to itself and says
		// nothing but "the selection changed" leaves the reader where it was. The list keeps the focus when there is no
		// current row, as a native empty list box does. See AccessibilityBuilder.FocusChild, which refuses the
		// delegation unless the list really is the panel holding the focus.
		b.FocusChild(currentID)
	}
}

// axCurrentRow returns the row the person is on, which is the row the keyboard focus is reported on: the lead row,
// while the list still has it and it is still selected, and otherwise the first selected row. The result is -1 when
// nothing is selected, since the selection is what a list is "on" — an assistive technology reads the rows it holds —
// and there is then nowhere within the list to report the focus.
func (l *List[T]) axCurrentRow() int {
	if l.lead >= 0 && l.lead < len(l.rows) && l.Selection.State(l.lead) {
		return l.lead
	}
	if first := l.Selection.FirstSet(); first >= 0 && first < len(l.rows) {
		return first
	}
	return -1
}

// axDescribeUniformRows describes the rows worth describing — those within reach of the part that can be seen (see
// axReach), plus the selection — when every row is the same height, which is when where a row sits is arithmetic rather
// than a walk through the rows before it.
func (l *List[T]) axDescribeUniformRows(b *AccessibilityBuilder, reach geom.Rect, cellHeight float32,
	current int,
) accessibility.NodeID {
	rect := l.ContentRect(false)
	first, last := 0, -1
	if !reach.Empty() {
		first = max(int(xmath.Floor((reach.Y-rect.Y)/cellHeight)), 0)
		last = min(int(xmath.Ceil((reach.Bottom()-rect.Y)/cellHeight)), len(l.rows)-1)
	}
	var currentID accessibility.NodeID
	rowRect := geom.NewRect(rect.X, rect.Y, rect.Width, cellHeight)
	for row := first; row <= last; row++ {
		rowRect.Y = rect.Y + cellHeight*float32(row)
		id := l.axAddRow(b, row, rowRect, nil, false)
		if row == current {
			currentID = id
		}
	}
	// The selected rows that were not reached above are described as well, however far out of sight they are, but as
	// nothing more than themselves: see axAddRow. The current row goes first, so that the cap on how many of them are
	// described can never be what leaves out the one row the focus is to be reported on.
	described := 0
	if current >= 0 && currentID == 0 {
		rowRect.Y = rect.Y + cellHeight*float32(current)
		currentID = l.axAddRow(b, current, rowRect, nil, true)
		described++
	}
	for row := l.Selection.FirstSet(); row >= 0 && described < axMaxSelectedRows; row = l.Selection.NextSet(row + 1) {
		if row >= len(l.rows) {
			break
		}
		if (row >= first && row <= last) || row == current {
			continue
		}
		rowRect.Y = rect.Y + cellHeight*float32(row)
		l.axAddRow(b, row, rowRect, nil, true)
		described++
	}
	return currentID
}

// axDescribeVaryingRows describes the rows worth describing when each row's height is its own. Every row has to be
// measured to know where the ones after it sit, and measuring means creating the cell, so the cell that was created is
// handed on to be named from rather than being created a second time.
//
// The walk stops as soon as nothing worth describing can be left: creating and laying out a panel for every row of a
// long list, up to twenty times a second, is exactly what VisibleRect exists to avoid, and is far more than drawing
// does, which stops at the bottom of the dirty rect.
func (l *List[T]) axDescribeVaryingRows(b *AccessibilityBuilder, reach geom.Rect,
	current int,
) accessibility.NodeID {
	rect := l.ContentRect(false)
	rowRect := geom.NewRect(rect.X, rect.Y, rect.Width, 0)
	described := 0
	var currentID accessibility.NodeID
	for row := range l.rows {
		cell := l.cell(row)
		_, pref, _ := cell.Sizes(geom.Size{})
		rowRect.Height = pref.Ceil().Height
		switch {
		case rowRect.Intersects(reach):
			id := l.axAddRow(b, row, rowRect, cell, false)
			if row == current {
				currentID = id
			}
		case row == current:
			// The row the focus is to be reported on is described however far out of sight it is and whatever the cap
			// on selected rows has reached: it is where the person is. It is not counted against that cap, so the rest
			// of the selection is described exactly as much as it would have been without it.
			currentID = l.axAddRow(b, row, rowRect, cell, true)
		case l.Selection.State(row) && described < axMaxSelectedRows:
			l.axAddRow(b, row, rowRect, cell, true)
			described++
		}
		rowRect.Y += rowRect.Height
		if rowRect.Y > reach.Bottom() && row >= current &&
			(described >= axMaxSelectedRows || l.Selection.NextSet(row+1) < 0) {
			// Every row from here down starts below the reach, so none of them can be seen; the current row has gone
			// by; and either there is no selected row left to describe or as many of them as will be described have
			// been.
			break
		}
	}
	return currentID
}

// axAddRow describes one row of the list. cell, when not nil, is the cell that was already created for the row, whose
// text is what the row is named by; one is created if it is nil.
//
// A row described with nameOnly is described as itself and nothing more: what it is, where it is and whether it is
// selected, without what the cell drawing it is made of. That is what is left of a row nobody can see — one described
// only because the selection holds it — and it is all an assistive technology asks of such a row, since what it does
// with the selection is read out what is in it. Laying the cell out and walking it for every selected row of a long
// list, over and over, is what that avoids; the table does the same thing for the same reason.
//
// The node id of the row is returned, or zero when nothing was added, which is what ProvideAccessibility reports the
// keyboard focus on for the current row.
func (l *List[T]) axAddRow(b *AccessibilityBuilder, row int, rect geom.Rect, cell *Panel,
	nameOnly bool,
) accessibility.NodeID {
	if cell == nil {
		cell = l.cell(row)
	}
	name := axLabelText(cell)
	selected := l.Selection.State(row)
	rowID := b.AddVirtualChild(row, func(n *accessibility.Node) {
		n.Role = role.ListItem
		n.Name = name
		n.Bounds = rect
		n.RowIndex = row
		n.Selectable = true
		n.Selected = selected
		// The rows are where the keyboard focus within a list is, as they are in a native one: the list reports the
		// focus it holds on the row the person is on, and an assistive technology asking for the focus to be put on
		// another row is asking to move to it, which selects it. See PerformAccessibilityAction.
		n.Focusable = true
		n.Actions = n.Actions.With(accessibility.Select, accessibility.ScrollIntoView, accessibility.Focus)
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
		if l.offersContextMenu() {
			// Each row offers the list's menu, opening it as a right-click on the row would.
			n.Actions = n.Actions.With(accessibility.ShowContextMenu)
		}
	})
	if rowID == 0 || nameOnly {
		return rowID
	}
	// Whatever the factory drew the row with — a label, a check box, a box full of both — is described within the
	// item, attached and laid out for the moment exactly as it is for drawing, so that what the row is made of can be
	// read by line, by word and by character and can be acted on where it sits. An item with content of its own to
	// describe has no need of that content's text as a name on top of it; an item whose cell amounts to nothing keeps
	// the name, which is all there would otherwise be.
	l.installCell(cell, rect)
	b.addCellPanel(rowID, row, cell, false)
	l.uninstallCell(cell)
	axClearCellFocusability(b.snapshot.tree, rowID)
	content := b.snapshot.tree.UnignoredChildren(rowID)
	if len(content) == 0 {
		return rowID
	}
	itemNode := b.snapshot.tree.Node(rowID)
	if itemNode == nil {
		return rowID
	}
	itemNode.Name = ""
	if len(content) == 1 {
		// An item holding a single widget with a state of its own reports that state as its value, so that a change to
		// it is heard as a change to the item. What the item does with a press is not the widget's to take over,
		// unlike a table cell: pressing a list row means opening it, which is what a double-click and the Return key
		// stand for, and the widget within the row offers its own press where it sits.
		itemNode.Value = axContentValue(b.snapshot.tree.Node(content[0]))
	}
	return rowID
}

// axClearCellFocusability takes the keyboard focus out of the reach of everything described beneath a list row.
// Nothing inside the cell that draws a row can hold the focus: the cell is built for the moment it takes to describe
// it or draw it and is thrown away again, the next draw builds another, and a focus left on the panel that was
// described is a focus the window cannot find, silently gone until the person presses Tab. So those nodes neither
// claim to be focusable nor offer to take the focus, and List.axPerformInCell refuses a Focus request that names one
// of them anyway for the same reason. The row's own node is left alone; only what the cell put beneath it is changed.
//
// A node that was only kept out of the scaffolding it otherwise is by being focusable or by offering the focus is
// judged again once it is neither — see axIsScaffolding, which refuses to look past anything that can be focused or
// acted on. Without that, an anonymous wrapper panel that happened to be focusable would survive as the row's only
// unignored content: a nameless, action-less group that the row would then drop its own name for and read its value
// from, announcing nothing in place of the widget inside it. Nothing else is reconsidered, so a node that said outright
// it was not to be looked past keeps its say.
//
// The walk needs no record of where it has been: what the cell put beneath the row is a tree, so no id can be reached
// twice.
func axClearCellFocusability(tree *accessibility.Tree, rowID accessibility.NodeID) {
	row := tree.Node(rowID)
	if row == nil {
		return
	}
	stack := slices.Clone(row.Children)
	for len(stack) != 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := tree.Node(id)
		if node == nil {
			continue
		}
		reachable := node.Focusable || node.Actions.Has(accessibility.Focus)
		node.Focusable = false
		node.Actions = node.Actions.Without(accessibility.Focus)
		if reachable && !node.Ignored && axIsScaffolding(node) {
			node.Ignored = true
		}
		stack = append(stack, node.Children...)
	}
}

// PerformAccessibilityAction carries out a request from an assistive technology. Every request that reaches here names
// one of the rows described by ProvideAccessibility, which arrives as the index that row was keyed by. Selecting a row
// also scrolls it into view, as the arrow keys do, since an assistive technology moving through the rows selects each
// one as it goes and expects to see where it has got to. Pressing a row opens it, which is the gesture a double-click
// and the Return key stand for. Putting the focus on a row moves the person onto it: the list takes the keyboard focus
// and the row becomes the whole of the selection, which is the same thing clicking the row does, since the selection is
// where a list's cursor is. Asking a row for the contextual menu does what a right-click on it would. A request aimed
// at something within the cell that draws a row is passed on to it.
func (l *List[T]) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	if key, isPanel := req.Key.(axCellPanelKey); isPanel {
		return l.axPerformInCell(key, req)
	}
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
	case accessibility.Focus:
		// The list is the one tab stop this widget has, so the focus goes there; the row is what the list then reports
		// it on, which the selection is what decides. Both happen for the same reason a click on the row does both.
		// The callback is for a selection that actually changed, exactly as it is for Select above.
		changed := !l.Selection.State(row) || l.Selection.Count() != 1
		l.Select(false, row)
		if changed {
			SafeCall(l.NewSelectionCallback)
		}
		l.ScrollRectIntoView(l.RowRect(row))
		l.RequestFocus()
		if wnd := l.Window(); wnd == nil || !l.Is(wnd.CurrentFocus()) {
			// A focus request that did not leave the focus where it said it would must be reported as refused rather
			// than as carried out, exactly as axDispatchAction checks for a real panel: the window may decline it, and
			// a list with no window cannot take it at all.
			return false
		}
	case accessibility.ScrollIntoView:
		l.ScrollRectIntoView(l.RowRect(row))
	case accessibility.ShowContextMenu:
		return l.axShowRowContextMenu(row)
	default:
		return false
	}
	return true
}

// axShowRowContextMenu opens the list's contextual menu for an assistive technology as a right-click on the row would:
// the list takes the focus when it can hold it, since menu commands act on whatever holds it, and the row is then
// selected for the menu, in the order ContextMenuPressed does both. The menu opens beneath the row, scrolled into view
// first. It is refused with nothing changed when axMayShowContextMenu says so. A mouse button still held is released
// first, since a native menu's tracking loop would swallow its release, and before the row is readied, since the
// release may narrow the selection or change the rows. The panel losing the focus and the NewSelectionCallback may
// change the rows or end the offer too, so both are checked again after each step. A row is known here only by its
// index, so a change that leaves a row at the index is taken at its word.
func (l *List[T]) axShowRowContextMenu(row int) bool {
	if row < 0 || row >= len(l.rows) || !axMayShowContextMenu(l.AsPanel()) {
		return false
	}
	// axMayShowContextMenu requires the active window, so Window is not nil.
	l.Window().endPressesForContextMenu()
	if row >= len(l.rows) || !axMayShowContextMenu(l.AsPanel()) {
		return false
	}
	if l.Focusable() {
		l.requestFocusWithoutScroll()
		if row >= len(l.rows) || !axMayShowContextMenu(l.AsPanel()) {
			return false
		}
	}
	l.selectRowForContextMenu(row)
	if row >= len(l.rows) || !axMayShowContextMenu(l.AsPanel()) {
		return false
	}
	rect := l.RowRect(row)
	l.ScrollRectIntoView(rect)
	return l.ShowContextMenu(geom.NewPoint(rect.X, rect.Bottom()))
}

// selectRowForContextMenu readies a row for a contextual menu about to open over it: an unselected row becomes the
// whole selection, while a selected one keeps the rest of the selection for the menu to act on. Either way it becomes
// the anchor and the lead row. A change is reported at once, as no DefaultMouseUp follows to report it.
func (l *List[T]) selectRowForContextMenu(row int) {
	if !l.Selection.State(row) {
		l.Select(false, row)
		SafeCall(l.NewSelectionCallback)
	}
	l.anchor = row
	l.setLead(row)
}

// requestFocusWithoutScroll takes the keyboard focus without DefaultFocusGained scrolling the list into view, which
// would move the rows out from under the pointer.
func (l *List[T]) requestFocusWithoutScroll() {
	l.suppressScroll = true
	l.RequestFocus()
	l.suppressScroll = false
}

// ContextMenuPressed implements ContextMenuPressHandler: the list takes the focus without scrolling, when it can hold
// it, and selects the row under the pointer whatever the modifiers (see selectRowForContextMenu). The press is asked
// back should it become a drag, so that a right-drag extends the selection as on a list without a menu, except while a
// modifier is down, since DefaultMouseDown would then toggle the row just selected or extend from the anchor just
// moved.
func (l *List[T]) ContextMenuPressed(where geom.Point, mods mod.Modifiers) bool {
	if l.Focusable() {
		l.requestFocusWithoutScroll()
	}
	if index, _ := l.rowAt(where.Y); index >= 0 {
		l.selectRowForContextMenu(index)
	}
	return mods&mod.NonSticky == 0
}

// ContextMenuAnchor implements ContextMenuAnchorer: beneath the row the person is on, scrolled into view first, or
// DefaultContextMenuAnchor when nothing is selected.
func (l *List[T]) ContextMenuAnchor() geom.Point {
	row := l.axCurrentRow()
	if row < 0 {
		return l.DefaultContextMenuAnchor()
	}
	rect := l.RowRect(row)
	l.ScrollRectIntoView(rect)
	return geom.NewPoint(rect.X, rect.Bottom())
}

// axPerformInCell carries out a request aimed at a panel inside the cell that draws one of the rows. The cell is built
// and installed again, exactly as it is to draw it, so that the panel the request is for exists where it was described
// and can reach its window; the panel is then found at the position within the cell it was described at.
//
// A Focus request is refused before any of that, without building anything. Nothing within the cell can keep the
// focus: the panel the request names is detached again as soon as the call is done, the next draw builds another cell
// entirely, and a focus left on a detached panel is a focus the window cannot find, silently gone until the person
// presses Tab. A Focus request that cannot leave the focus on the node it named must be refused rather than reported
// as carried out, so the nodes described within a row are published as neither focusable nor offering Focus, and one
// that arrives anyway is turned away here. A ShowContextMenu request is refused for the same reason, since the menu's
// commands act on whatever holds the focus; nothing within a row offers the menu, and the list offers its own on each
// row instead. The row itself is another matter: it does offer the focus,
// and the list carries that out by taking the focus and selecting the row — see PerformAccessibilityAction.
//
// A widget that took the focus while handling one of the remaining requests — which Press does on its way to the click
// it synthesizes — is handed straight back for that same reason. The focus goes to the list itself, which is the one
// tab stop this widget has and where a real click on a row leaves it.
//
// A key naming a cell of something other than a list is refused: the key travels with the request from whatever
// described the panel, and only the widget that put it there knows how to read it.
func (l *List[T]) axPerformInCell(key axCellPanelKey, req accessibility.ActionRequest) bool {
	if req.Action == accessibility.Focus || req.Action == accessibility.ShowContextMenu {
		return false
	}
	row, ok := key.Cell.(int)
	if !ok || row < 0 || row >= len(l.rows) {
		return false
	}
	// The key that brought the request here named one of the list's virtual children. What it is being handed to is a
	// real panel, for which ActionRequest.Key is nil: an application's own action callback on a panel within a cell
	// would otherwise be given a key it never handed out, and a widget that keys virtual children of its own would
	// take this one for one of them.
	req.Key = nil
	cell := l.cell(row)
	l.installCell(cell, l.RowRect(row))
	handled := false
	if target := axPanelAtPath(cell, key.Path); target != nil {
		handled = target.axDispatchAction(req, false)
	}
	takeBackFocus := false
	if wnd := l.Window(); wnd != nil && wnd.focus != nil && panelContains(cell, wnd.focus) {
		takeBackFocus = true
	}
	l.uninstallCell(cell)
	if takeBackFocus {
		l.RequestFocus()
	}
	l.MarkForRedraw()
	return handled
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
