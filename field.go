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
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

// Field provides a single-line text input control.
type Field struct {
	Panel
	ModifiedCallback func()
	ValidateCallback func() bool
	Font             FontProvider
	ErrorColor       Ink
	OnErrorColor     Ink
	EnabledColor     Ink
	OnEnabledColor   Ink
	DisabledColor    Ink
	OnDisabledColor  Ink
	SelectionColor   Ink
	OnSelectionColor Ink
	BlinkRate        time.Duration
	Watermark        string
	FocusedBorder    Border
	UnfocusedBorder  Border
	runes            []rune
	selectionStart   int
	selectionEnd     int
	selectionAnchor  int
	forceShowUntil   time.Time
	MinimumTextWidth float32
	scrollOffset     float32
	showCursor       bool
	pending          bool
	extendByWord     bool
	invalid          bool
}

// NewField creates a new, empty, field.
func NewField() *Field {
	t := &Field{
		MinimumTextWidth: 10,
		BlinkRate:        560 * time.Millisecond,
		FocusedBorder:    NewCompoundBorder(NewLineBorder(SelectionColor, 0, geom32.NewUniformInsets(2), false), NewEmptyBorder(geom32.Insets{Top: 2, Left: 2, Bottom: 1, Right: 2})),
		UnfocusedBorder:  NewCompoundBorder(NewLineBorder(ControlEdgeColor, 0, geom32.NewUniformInsets(1), false), NewEmptyBorder(geom32.Insets{Top: 3, Left: 3, Bottom: 2, Right: 3})),
	}
	t.Self = t
	t.SetBorder(t.UnfocusedBorder)
	t.SetFocusable(true)
	t.SetSizer(t.DefaultSizes)
	t.DrawCallback = t.DefaultDraw
	t.GainedFocusCallback = t.DefaultFocusGained
	t.LostFocusCallback = t.DefaultFocusLost
	t.MouseDownCallback = t.DefaultMouseDown
	t.MouseDragCallback = t.DefaultMouseDrag
	t.UpdateCursorCallback = t.DefaultUpdateCursor
	t.KeyDownCallback = t.DefaultKeyDown
	t.RuneTypedCallback = t.DefaultRuneTyped
	t.CanPerformCmdCallback = t.DefaultCanPerformCmd
	t.PerformCmdCallback = t.DefaultPerformCmd
	return t
}

// DefaultSizes provides the default sizing.
func (t *Field) DefaultSizes(hint geom32.Size) (min, pref, max geom32.Size) {
	var text string
	if len(t.runes) != 0 {
		text = string(t.runes)
	} else {
		text = "M"
	}
	minWidth := t.MinimumTextWidth
	pref = ChooseFont(t.Font, FieldFont).Extents(text)
	if pref.Width < minWidth {
		pref.Width = minWidth
	}
	if b := t.Border(); b != nil {
		insets := b.Insets()
		pref.AddInsets(insets)
		minWidth += insets.Left + insets.Right
	}
	pref.GrowToInteger()
	if hint.Width >= 1 && hint.Width < minWidth {
		hint.Width = minWidth
	}
	pref.ConstrainForHint(hint)
	min = pref
	min.Width = minWidth
	return min, pref, MaxSize(pref)
}

// DefaultDraw provides the default drawing.
func (t *Field) DefaultDraw(canvas *Canvas, dirty geom32.Rect) {
	var fg, bg Ink
	switch {
	case t.invalid:
		fg = ChooseInk(t.ErrorColor, EditableErrorColor)
		bg = ChooseInk(t.OnErrorColor, OnEditableErrorColor)
	case !t.Enabled():
		fg = ChooseInk(t.DisabledColor, BackgroundColor)
		bg = ChooseInk(t.OnDisabledColor, OnBackgroundColor)
	default:
		fg = ChooseInk(t.EnabledColor, EditableColor)
		bg = ChooseInk(t.OnEnabledColor, OnEditableColor)
	}
	rect := t.ContentRect(true)
	canvas.DrawRect(rect, fg.Paint(canvas, rect, Fill))
	rect = t.ContentRect(false)
	clipRect := rect
	clipRect.Inset(geom32.NewUniformInsets(-2)) // Remove interior padding for the clip
	canvas.ClipRect(clipRect, IntersectClipOp, false)
	f := ChooseFont(t.Font, FieldFont)
	textTop := rect.Y + (rect.Height-f.LineHeight())/2
	textBaseLine := textTop + f.Baseline()
	switch {
	case t.Enabled() && t.Focused() && t.HasSelectionRange():
		left := rect.X + t.scrollOffset
		if t.selectionStart > 0 {
			pre := string(t.runes[:t.selectionStart])
			canvas.DrawSimpleText(pre, left, textBaseLine, f, bg.Paint(canvas, rect, Fill))
			left += f.Width(pre)
		}
		mid := string(t.runes[t.selectionStart:t.selectionEnd])
		right := rect.X + f.Width(string(t.runes[:t.selectionEnd])) + t.scrollOffset
		selRect := geom32.Rect{
			Point: geom32.Point{X: left, Y: clipRect.Y},
			Size:  geom32.Size{Width: right - left, Height: clipRect.Height},
		}
		focusInk := ChooseInk(t.SelectionColor, SelectionColor)
		canvas.DrawRect(selRect, focusInk.Paint(canvas, selRect, Fill))
		canvas.DrawSimpleText(mid, left, textBaseLine, f,
			ChooseInk(t.OnSelectionColor, OnSelectionColor).Paint(canvas, rect, Fill))
		if t.selectionStart < len(t.runes) {
			canvas.DrawSimpleText(string(t.runes[t.selectionEnd:]), right, textBaseLine, f,
				bg.Paint(canvas, rect, Fill))
		}
	case len(t.runes) == 0:
		if t.Watermark != "" {
			canvas.SaveWithOpacity(0.33)
			canvas.DrawSimpleText(t.Watermark, rect.X, textBaseLine, f, bg.Paint(canvas, rect, Fill))
			canvas.Restore()
		}
	default:
		canvas.DrawSimpleText(string(t.runes), rect.X+t.scrollOffset, textBaseLine, f,
			bg.Paint(canvas, rect, Fill))
	}
	if !t.HasSelectionRange() && t.Enabled() && t.Focused() {
		if t.showCursor {
			canvas.DrawRect(geom32.NewRect(rect.X+f.Width(string(t.runes[:t.selectionEnd]))+t.scrollOffset-0.5,
				clipRect.Y, 1, clipRect.Height), bg.Paint(canvas, rect, Fill))
		}
		t.scheduleBlink()
	}
}

// Invalid returns true if the field is currently marked as invalid.
func (t *Field) Invalid() bool {
	return t.invalid
}

func (t *Field) scheduleBlink() {
	window := t.Window()
	if window != nil && window.IsValid() && !t.pending && t.Enabled() && t.Focused() {
		t.pending = true
		InvokeTaskAfter(t.blink, t.BlinkRate)
	}
}

func (t *Field) blink() {
	window := t.Window()
	if window != nil && window.IsValid() {
		t.pending = false
		if time.Now().After(t.forceShowUntil) {
			t.showCursor = !t.showCursor
			t.MarkForRedraw()
		}
		t.scheduleBlink()
	}
}

// DefaultFocusGained provides the default focus gained handling.
func (t *Field) DefaultFocusGained() {
	t.SetBorder(t.FocusedBorder)
	if !t.HasSelectionRange() {
		t.SelectAll()
	}
	t.showCursor = true
	t.MarkForRedraw()
}

// DefaultFocusLost provides the default focus lost handling.
func (t *Field) DefaultFocusLost() {
	t.SetBorder(t.UnfocusedBorder)
	if !t.CanSelectAll() {
		t.SetSelectionToStart()
	}
	t.MarkForRedraw()
}

// DefaultMouseDown provides the default mouse down handling.
func (t *Field) DefaultMouseDown(where geom32.Point, button, clickCount int, mod Modifiers) bool {
	t.RequestFocus()
	if button == ButtonLeft {
		t.extendByWord = false
		switch clickCount {
		case 2:
			start, end := t.findWordAt(t.ToSelectionIndex(where.X))
			t.SetSelection(start, end)
			t.extendByWord = true
		case 3:
			t.SelectAll()
		default:
			oldAnchor := t.selectionAnchor
			t.selectionAnchor = t.ToSelectionIndex(where.X)
			var start, end int
			if mod.ShiftDown() {
				if oldAnchor > t.selectionAnchor {
					start = t.selectionAnchor
					end = oldAnchor
				} else {
					start = oldAnchor
					end = t.selectionAnchor
				}
			} else {
				start = t.selectionAnchor
				end = t.selectionAnchor
			}
			t.setSelection(start, end, t.selectionAnchor)
		}
		return true
	}
	return false
}

// DefaultMouseDrag provides the default mouse drag handling.
func (t *Field) DefaultMouseDrag(where geom32.Point, button int, mod Modifiers) bool {
	oldAnchor := t.selectionAnchor
	pos := t.ToSelectionIndex(where.X)
	var start, end int
	if t.extendByWord {
		s1, e1 := t.findWordAt(oldAnchor)
		var dir int
		if pos > s1 {
			dir = -1
		} else {
			dir = 1
		}
		for {
			start, end = t.findWordAt(pos)
			if start != end {
				if start > s1 {
					start = s1
				}
				if end < e1 {
					end = e1
				}
				break
			}
			pos += dir
			if dir > 0 && pos >= s1 || dir < 0 && pos <= e1 {
				start = s1
				end = e1
				break
			}
		}
	} else {
		if pos > oldAnchor {
			start = oldAnchor
			end = pos
		} else {
			start = pos
			end = oldAnchor
		}
	}
	t.setSelection(start, end, oldAnchor)
	return true
}

// DefaultUpdateCursor provides the default cursor update handling.
func (t *Field) DefaultUpdateCursor(where geom32.Point) *Cursor {
	if t.Enabled() {
		return TextCursor()
	}
	return ArrowCursor()
}

// DefaultKeyDown provides the default key down handling.
func (t *Field) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if mod.OSMenuCmdModifierDown() {
		return false
	}
	if wnd := t.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	switch keyCode {
	case KeyBackspace:
		t.Delete()
	case KeyDelete, KeyNumPadDelete:
		if t.HasSelectionRange() {
			t.Delete()
		} else if t.selectionStart < len(t.runes) {
			t.runes = append(t.runes[:t.selectionStart], t.runes[t.selectionStart+1:]...)
			t.notifyOfModification()
		}
		t.MarkForRedraw()
	case KeyLeft, KeyNumPadLeft:
		extend := mod.ShiftDown()
		if mod.CommandDown() {
			t.handleHome(extend)
		} else {
			t.handleArrowLeft(extend, mod.OptionDown())
		}
	case KeyRight, KeyNumPadRight:
		extend := mod.ShiftDown()
		if mod.CommandDown() {
			t.handleEnd(extend)
		} else {
			t.handleArrowRight(extend, mod.OptionDown())
		}
	case KeyEnd, KeyNumPadEnd, KeyPageDown, KeyNumPadPageDown, KeyDown, KeyNumPadDown:
		t.handleEnd(mod.ShiftDown())
	case KeyHome, KeyNumPadHome, KeyPageUp, KeyNumPadPageUp, KeyUp, KeyNumPadUp:
		t.handleHome(mod.ShiftDown())
	case KeyTab:
		return false
	}
	return true
}

// DefaultRuneTyped provides the default rune typed handling.
func (t *Field) DefaultRuneTyped(ch rune) bool {
	if wnd := t.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	if unicode.IsControl(ch) {
		return false
	}
	if t.HasSelectionRange() {
		t.runes = append(t.runes[:t.selectionStart], t.runes[t.selectionEnd:]...)
	}
	t.runes = append(t.runes[:t.selectionStart], append([]rune{ch}, t.runes[t.selectionStart:]...)...)
	t.SetSelectionTo(t.selectionStart + 1)
	t.notifyOfModification()
	return true
}

func (t *Field) handleHome(extend bool) {
	if extend {
		t.setSelection(0, t.selectionEnd, t.selectionEnd)
	} else {
		t.SetSelectionToStart()
	}
}

func (t *Field) handleEnd(extend bool) {
	if extend {
		t.SetSelection(t.selectionStart, len(t.runes))
	} else {
		t.SetSelectionToEnd()
	}
}

func (t *Field) handleArrowLeft(extend, byWord bool) {
	if t.HasSelectionRange() {
		if extend {
			anchor := t.selectionAnchor
			if t.selectionStart == anchor {
				pos := t.selectionEnd - 1
				if byWord {
					start, _ := t.findWordAt(pos)
					pos = xmath.MinInt(xmath.MaxInt(start, anchor), pos)
				}
				t.setSelection(anchor, pos, anchor)
			} else {
				pos := t.selectionStart - 1
				if byWord {
					start, _ := t.findWordAt(pos)
					pos = xmath.MinInt(start, pos)
				}
				t.setSelection(pos, anchor, anchor)
			}
		} else {
			t.SetSelectionTo(t.selectionStart)
		}
	} else {
		pos := t.selectionStart - 1
		if byWord {
			start, _ := t.findWordAt(pos)
			pos = xmath.MinInt(start, pos)
		}
		if extend {
			t.setSelection(pos, t.selectionStart, t.selectionEnd)
		} else {
			t.SetSelectionTo(pos)
		}
	}
}

func (t *Field) handleArrowRight(extend, byWord bool) {
	if t.HasSelectionRange() {
		if extend {
			anchor := t.selectionAnchor
			if t.selectionEnd == anchor {
				pos := t.selectionStart + 1
				if byWord {
					_, end := t.findWordAt(pos)
					pos = xmath.MaxInt(xmath.MinInt(end, anchor), pos)
				}
				t.setSelection(pos, anchor, anchor)
			} else {
				pos := t.selectionEnd + 1
				if byWord {
					_, end := t.findWordAt(pos)
					pos = xmath.MaxInt(end, pos)
				}
				t.setSelection(anchor, pos, anchor)
			}
		} else {
			t.SetSelectionTo(t.selectionEnd)
		}
	} else {
		pos := t.selectionEnd + 1
		if byWord {
			_, end := t.findWordAt(pos)
			pos = xmath.MaxInt(end, pos)
		}
		if extend {
			t.SetSelection(t.selectionStart, pos)
		} else {
			t.SetSelectionTo(pos)
		}
	}
}

// DefaultCanPerformCmd provides the default can perform command handling.
func (t *Field) DefaultCanPerformCmd(source interface{}, id int) bool {
	switch id {
	case CutItemID:
		return t.CanCut()
	case CopyItemID:
		return t.CanCopy()
	case PasteItemID:
		return t.CanPaste()
	case DeleteItemID:
		return t.CanDelete()
	case SelectAllItemID:
		return t.CanSelectAll()
	default:
		return false
	}
}

// DefaultPerformCmd provides the default perform command handling.
func (t *Field) DefaultPerformCmd(source interface{}, id int) {
	switch id {
	case CutItemID:
		t.Cut()
	case CopyItemID:
		t.Copy()
	case PasteItemID:
		t.Paste()
	case DeleteItemID:
		t.Delete()
	case SelectAllItemID:
		t.SelectAll()
	default:
	}
}

// CanCut returns true if the field has a selection that can be cut.
func (t *Field) CanCut() bool {
	return t.HasSelectionRange()
}

// Cut the selected text to the clipboard.
func (t *Field) Cut() {
	if t.HasSelectionRange() {
		GlobalClipboard.SetText(t.SelectedText())
		t.Delete()
	}
}

// CanCopy returns true if the field has a selection that can be copied.
func (t *Field) CanCopy() bool {
	return t.HasSelectionRange()
}

// Copy the selected text to the clipboard.
func (t *Field) Copy() {
	if t.HasSelectionRange() {
		GlobalClipboard.SetText(t.SelectedText())
	}
}

// CanPaste returns true if the clipboard has content that can be pasted into the field.
func (t *Field) CanPaste() bool {
	return GlobalClipboard.GetText() != ""
}

// Paste any text on the clipboard into the field.
func (t *Field) Paste() {
	text := GlobalClipboard.GetText()
	if text != "" {
		runes := []rune(t.sanitize(text))
		if t.HasSelectionRange() {
			t.runes = append(t.runes[:t.selectionStart], t.runes[t.selectionEnd:]...)
		}
		t.runes = append(t.runes[:t.selectionStart], append(runes, t.runes[t.selectionStart:]...)...)
		t.SetSelectionTo(t.selectionStart + len(runes))
		t.notifyOfModification()
	} else if t.HasSelectionRange() {
		t.Delete()
	}
}

// CanDelete returns true if the field has a selection that can be deleted.
func (t *Field) CanDelete() bool {
	return t.HasSelectionRange() || t.selectionStart > 0
}

// Delete removes the currently selected text, if any.
func (t *Field) Delete() {
	if t.CanDelete() {
		if t.HasSelectionRange() {
			t.runes = append(t.runes[:t.selectionStart], t.runes[t.selectionEnd:]...)
			t.SetSelectionTo(t.selectionStart)
		} else {
			t.runes = append(t.runes[:t.selectionStart-1], t.runes[t.selectionStart:]...)
			t.SetSelectionTo(t.selectionStart - 1)
		}
		t.notifyOfModification()
		t.MarkForRedraw()
	}
}

// CanSelectAll returns true if the field's selection can be expanded.
func (t *Field) CanSelectAll() bool {
	return t.selectionStart != 0 || t.selectionEnd != len(t.runes)
}

// SelectAll selects all of the text in the field.
func (t *Field) SelectAll() {
	t.SetSelection(0, len(t.runes))
}

// Text returns the content of the field.
func (t *Field) Text() string {
	return string(t.runes)
}

// SetText sets the content of the field.
func (t *Field) SetText(text string) {
	text = t.sanitize(text)
	if string(t.runes) != text {
		t.runes = []rune(text)
		t.SetSelectionToEnd()
		t.notifyOfModification()
	}
}

func (t *Field) notifyOfModification() {
	t.MarkForRedraw()
	if t.ModifiedCallback != nil {
		t.ModifiedCallback()
	}
	t.Validate()
}

// Validate forces field content validation to be run.
func (t *Field) Validate() {
	invalid := false
	if t.ValidateCallback != nil {
		invalid = !t.ValidateCallback()
	}
	if invalid != t.invalid {
		t.invalid = invalid
		t.MarkForRedraw()
	}
}

func (t *Field) sanitize(text string) string {
	return strings.NewReplacer("\n", "", "\r", "").Replace(text)
}

// SelectedText returns the currently selected text.
func (t *Field) SelectedText() string {
	return string(t.runes[t.selectionStart:t.selectionEnd])
}

// HasSelectionRange returns true is a selection range is currently present.
func (t *Field) HasSelectionRange() bool {
	return t.selectionStart < t.selectionEnd
}

// SelectionCount returns the number of characters currently selected.
func (t *Field) SelectionCount() int {
	return t.selectionEnd - t.selectionStart
}

// Selection returns the current start and end selection indexes.
func (t *Field) Selection() (start, end int) {
	return t.selectionStart, t.selectionEnd
}

// SetSelectionToStart moves the cursor to the beginning of the text and removes any range that may have been present.
func (t *Field) SetSelectionToStart() {
	t.SetSelection(0, 0)
}

// SetSelectionToEnd moves the cursor to the end of the text and removes any range that may have been present.
func (t *Field) SetSelectionToEnd() {
	t.SetSelection(math.MaxInt64, math.MaxInt64)
}

// SetSelectionTo moves the cursor to the specified index and removes any range that may have been present.
func (t *Field) SetSelectionTo(pos int) {
	t.SetSelection(pos, pos)
}

// SetSelection sets the start and end range of the selection. Values beyond either end will be constrained to the
// appropriate end. Likewise, an end value less than the start value will be treated as if the start and end values were
// the same.
func (t *Field) SetSelection(start, end int) {
	t.setSelection(start, end, start)
}

func (t *Field) setSelection(start, end, anchor int) {
	length := len(t.runes)
	if start < 0 {
		start = 0
	} else if start > length {
		start = length
	}
	if end < start {
		end = start
	} else if end > length {
		end = length
	}
	if anchor < start {
		anchor = start
	} else if anchor > end {
		anchor = end
	}
	if t.selectionStart != start || t.selectionEnd != end || t.selectionAnchor != anchor {
		t.selectionStart = start
		t.selectionEnd = end
		t.selectionAnchor = anchor
		t.forceShowUntil = time.Now().Add(t.BlinkRate)
		t.showCursor = true
		t.MarkForRedraw()
		t.ScrollIntoView()
		t.autoScroll()
	}
}

// ScrollOffset returns the current autoscroll offset.
func (t *Field) ScrollOffset() float32 {
	return t.scrollOffset
}

// SetScrollOffset sets the autoscroll offset to the specified value.
func (t *Field) SetScrollOffset(offset float32) {
	if t.scrollOffset != offset {
		t.scrollOffset = offset
		t.MarkForRedraw()
	}
}

func (t *Field) autoScroll() {
	rect := t.ContentRect(false)
	if rect.Width > 0 {
		original := t.scrollOffset
		if t.selectionStart == t.selectionAnchor {
			right := t.FromSelectionIndex(t.selectionEnd).X
			if right < rect.X {
				t.scrollOffset = 0
				t.scrollOffset = rect.X - t.FromSelectionIndex(t.selectionEnd).X
			} else if right >= rect.X+rect.Width {
				t.scrollOffset = 0
				t.scrollOffset = rect.X + rect.Width - 1 - t.FromSelectionIndex(t.selectionEnd).X
			}
		} else {
			left := t.FromSelectionIndex(t.selectionStart).X
			if left < rect.X {
				t.scrollOffset = 0
				t.scrollOffset = rect.X - t.FromSelectionIndex(t.selectionStart).X
			} else if left >= rect.X+rect.Width {
				t.scrollOffset = 0
				t.scrollOffset = rect.X + rect.Width - 1 - t.FromSelectionIndex(t.selectionStart).X
			}
		}
		save := t.scrollOffset
		t.scrollOffset = 0
		min := rect.X + rect.Width - 1 - t.FromSelectionIndex(len(t.runes)).X
		if min > 0 {
			min = 0
		}
		max := rect.X - t.FromSelectionIndex(0).X
		if max < 0 {
			max = 0
		}
		if save < min {
			save = min
		} else if save > max {
			save = max
		}
		t.scrollOffset = save
		if original != t.scrollOffset {
			t.MarkForRedraw()
		}
	}
}

// ToSelectionIndex returns the rune index for the specified x-coordinate.
func (t *Field) ToSelectionIndex(x float32) int {
	rect := t.ContentRect(false)
	return ChooseFont(t.Font, FieldFont).IndexForPosition(x-(rect.X+t.scrollOffset), string(t.runes))
}

// FromSelectionIndex returns a location in local coordinates for the specified rune index.
func (t *Field) FromSelectionIndex(index int) geom32.Point {
	rect := t.ContentRect(false)
	x := rect.X + t.scrollOffset
	top := rect.Y + rect.Height/2
	if index > 0 {
		length := len(t.runes)
		if index > length {
			index = length
		}
		x += ChooseFont(t.Font, FieldFont).PositionForIndex(index, string(t.runes))
	}
	return geom32.Point{X: x, Y: top}
}

func (t *Field) findWordAt(pos int) (start, end int) {
	length := len(t.runes)
	if pos < 0 {
		pos = 0
	} else if pos >= length {
		pos = length - 1
	}
	start = pos
	end = pos
	if length > 0 && !unicode.IsSpace(t.runes[start]) {
		for start > 0 && !unicode.IsSpace(t.runes[start-1]) {
			start--
		}
		for end < length && !unicode.IsSpace(t.runes[end]) {
			end++
		}
	}
	return start, end
}
