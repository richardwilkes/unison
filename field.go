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
	"math"
	"time"
	"unicode"

	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/toolbox/xmath"
	"github.com/richardwilkes/toolbox/xmath/geom"
)

// DefaultFieldTheme holds the default FieldTheme values for Fields. Modifying this data will not alter existing Fields,
// but will alter any Fields created in the future.
var DefaultFieldTheme = FieldTheme{
	Font:             FieldFont,
	BackgroundInk:    BackgroundColor,
	OnBackgroundInk:  OnBackgroundColor,
	EditableInk:      EditableColor,
	OnEditableInk:    OnEditableColor,
	SelectionInk:     SelectionColor,
	OnSelectionInk:   OnSelectionColor,
	ErrorInk:         ErrorColor,
	OnErrorInk:       OnErrorColor,
	FocusedBorder:    NewDefaultFieldBorder(true),
	UnfocusedBorder:  NewDefaultFieldBorder(false),
	BlinkRate:        560 * time.Millisecond,
	MinimumTextWidth: 10,
	HAlign:           StartAlignment,
}

// NewDefaultFieldBorder creates the default border for a field.
func NewDefaultFieldBorder(focused bool) Border {
	adj := float32(1)
	if focused {
		adj = 0
	}
	return NewCompoundBorder(NewLineBorder(ControlEdgeColor, 0, geom.NewUniformInsets[float32](2-adj), false),
		NewEmptyBorder(geom.Insets[float32]{Top: 2 + adj, Left: 2 + adj, Bottom: 1 + adj, Right: 2 + adj}))
}

// FieldTheme holds theming data for a Field.
type FieldTheme struct {
	Font             Font
	BackgroundInk    Ink
	OnBackgroundInk  Ink
	EditableInk      Ink
	OnEditableInk    Ink
	SelectionInk     Ink
	OnSelectionInk   Ink
	ErrorInk         Ink
	OnErrorInk       Ink
	FocusedBorder    Border
	UnfocusedBorder  Border
	BlinkRate        time.Duration
	MinimumTextWidth float32
	HAlign           Alignment
}

// Field provides a single-line text input control.
type Field struct {
	Panel
	FieldTheme
	ModifiedCallback func()
	ValidateCallback func() bool
	Watermark        string
	runes            []rune
	selectionStart   int
	selectionEnd     int
	selectionAnchor  int
	forceShowUntil   time.Time
	scrollOffset     float32
	showCursor       bool
	pending          bool
	extendByWord     bool
	invalid          bool
}

// NewField creates a new, empty, field.
func NewField() *Field {
	f := &Field{FieldTheme: DefaultFieldTheme}
	f.Self = f
	f.SetBorder(f.UnfocusedBorder)
	f.SetFocusable(true)
	f.SetSizer(f.DefaultSizes)
	f.DrawCallback = f.DefaultDraw
	f.GainedFocusCallback = f.DefaultFocusGained
	f.LostFocusCallback = f.DefaultFocusLost
	f.MouseDownCallback = f.DefaultMouseDown
	f.MouseDragCallback = f.DefaultMouseDrag
	f.UpdateCursorCallback = f.DefaultUpdateCursor
	f.KeyDownCallback = f.DefaultKeyDown
	f.RuneTypedCallback = f.DefaultRuneTyped
	f.CanPerformCmdCallback = f.DefaultCanPerformCmd
	f.PerformCmdCallback = f.DefaultPerformCmd
	return f
}

// DefaultSizes provides the default sizing.
func (f *Field) DefaultSizes(hint geom.Size[float32]) (min, pref, max geom.Size[float32]) {
	var r []rune
	if len(f.runes) != 0 {
		r = f.runes
	} else {
		r = []rune{'M'}
	}
	minWidth := f.MinimumTextWidth
	pref = NewTextFromRunes(r, &TextDecoration{
		Font:  f.Font,
		Paint: nil,
	}).Extents()
	if pref.Width < minWidth {
		pref.Width = minWidth
	}
	pref.Width += 2 // Allow room for the cursor on either side of the text
	if b := f.Border(); b != nil {
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
func (f *Field) DefaultDraw(canvas *Canvas, dirty geom.Rect[float32]) {
	var fg, bg Ink
	switch {
	case f.invalid:
		fg = f.ErrorInk
		bg = f.OnErrorInk
	case f.Enabled():
		fg = f.EditableInk
		bg = f.OnEditableInk
	default:
		fg = f.BackgroundInk
		bg = f.OnBackgroundInk
	}
	rect := f.ContentRect(true)
	canvas.DrawRect(rect, fg.Paint(canvas, rect, Fill))
	rect = f.ContentRect(false)
	canvas.ClipRect(rect, IntersectClipOp, false)
	paint := bg.Paint(canvas, rect, Fill)
	text := NewTextFromRunes(f.runes, &TextDecoration{
		Font:  f.Font,
		Paint: paint,
	})
	textLeft := f.textLeft(text, rect)
	textTop := rect.Y + (rect.Height-f.Font.LineHeight())/2
	textBaseLine := textTop + f.Font.Baseline()
	switch {
	case f.Enabled() && f.Focused() && f.HasSelectionRange():
		left := textLeft + f.scrollOffset
		if f.selectionStart > 0 {
			t := NewTextFromRunes(f.runes[:f.selectionStart], &TextDecoration{
				Font:  f.Font,
				Paint: paint,
			})
			t.Draw(canvas, left, textBaseLine)
			left += t.Width()
		}
		t := NewTextFromRunes(f.runes[f.selectionStart:f.selectionEnd], &TextDecoration{
			Font:  f.Font,
			Paint: f.OnSelectionInk.Paint(canvas, rect, Fill),
		})
		right := left + t.Width()
		selRect := geom.Rect[float32]{
			Point: geom.Point[float32]{X: left, Y: rect.Y},
			Size:  geom.Size[float32]{Width: right - left, Height: rect.Height},
		}
		canvas.DrawRect(selRect, f.SelectionInk.Paint(canvas, selRect, Fill))
		t.Draw(canvas, left, textBaseLine)
		if f.selectionStart < len(f.runes) {
			NewTextFromRunes(f.runes[f.selectionEnd:], &TextDecoration{
				Font:  f.Font,
				Paint: paint,
			}).Draw(canvas, right, textBaseLine)
		}
	case len(f.runes) == 0:
		if f.Watermark != "" {
			paint.SetColorFilter(NewAlphaFilter(0.3))
			text = NewText(f.Watermark, &TextDecoration{
				Font:  f.Font,
				Paint: paint,
			})
			text.Draw(canvas, textLeft-text.Width(), textBaseLine)
		}
	default:
		if !f.Enabled() {
			paint.SetColorFilter(Grayscale30PercentFilter())
		}
		text.Draw(canvas, textLeft+f.scrollOffset, textBaseLine)
	}
	if !f.HasSelectionRange() && f.Enabled() && f.Focused() {
		if f.showCursor {
			canvas.DrawRect(geom.NewRect(textLeft+NewTextFromRunes(f.runes[:f.selectionEnd], &TextDecoration{
				Font:  f.Font,
				Paint: nil,
			}).Width()+f.scrollOffset-0.5, rect.Y, 1, rect.Height),
				bg.Paint(canvas, rect, Fill))
		}
		f.scheduleBlink()
	}
}

// Invalid returns true if the field is currently marked as invalid.
func (f *Field) Invalid() bool {
	return f.invalid
}

func (f *Field) scheduleBlink() {
	window := f.Window()
	if window != nil && window.IsValid() && !f.pending && f.Enabled() && f.Focused() {
		f.pending = true
		InvokeTaskAfter(f.blink, f.BlinkRate)
	}
}

func (f *Field) blink() {
	window := f.Window()
	if window != nil && window.IsValid() {
		f.pending = false
		if time.Now().After(f.forceShowUntil) {
			f.showCursor = !f.showCursor
			f.MarkForRedraw()
		}
		f.scheduleBlink()
	}
}

// DefaultFocusGained provides the default focus gained handling.
func (f *Field) DefaultFocusGained() {
	f.SetBorder(f.FocusedBorder)
	if !f.HasSelectionRange() {
		f.SelectAll()
	}
	f.showCursor = true
	f.MarkForRedraw()
}

// DefaultFocusLost provides the default focus lost handling.
func (f *Field) DefaultFocusLost() {
	f.SetBorder(f.UnfocusedBorder)
	if !f.CanSelectAll() {
		f.SetSelectionToStart()
	}
	f.MarkForRedraw()
}

// DefaultMouseDown provides the default mouse down handling.
func (f *Field) DefaultMouseDown(where geom.Point[float32], button, clickCount int, mod Modifiers) bool {
	f.RequestFocus()
	if button == ButtonLeft {
		f.extendByWord = false
		switch clickCount {
		case 2:
			start, end := f.findWordAt(f.ToSelectionIndex(where.X))
			f.SetSelection(start, end)
			f.extendByWord = true
		case 3:
			f.SelectAll()
		default:
			oldAnchor := f.selectionAnchor
			f.selectionAnchor = f.ToSelectionIndex(where.X)
			var start, end int
			if mod.ShiftDown() {
				if oldAnchor > f.selectionAnchor {
					start = f.selectionAnchor
					end = oldAnchor
				} else {
					start = oldAnchor
					end = f.selectionAnchor
				}
			} else {
				start = f.selectionAnchor
				end = f.selectionAnchor
			}
			f.setSelection(start, end, f.selectionAnchor)
		}
		return true
	}
	return false
}

// DefaultMouseDrag provides the default mouse drag handling.
func (f *Field) DefaultMouseDrag(where geom.Point[float32], button int, mod Modifiers) bool {
	oldAnchor := f.selectionAnchor
	pos := f.ToSelectionIndex(where.X)
	var start, end int
	if f.extendByWord {
		s1, e1 := f.findWordAt(oldAnchor)
		var dir int
		if pos > s1 {
			dir = -1
		} else {
			dir = 1
		}
		for {
			start, end = f.findWordAt(pos)
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
	f.setSelection(start, end, oldAnchor)
	return true
}

// DefaultUpdateCursor provides the default cursor update handling.
func (f *Field) DefaultUpdateCursor(where geom.Point[float32]) *Cursor {
	if f.Enabled() {
		return TextCursor()
	}
	return ArrowCursor()
}

// DefaultKeyDown provides the default key down handling.
func (f *Field) DefaultKeyDown(keyCode KeyCode, mod Modifiers, repeat bool) bool {
	if mod.OSMenuCmdModifierDown() {
		return false
	}
	if wnd := f.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	switch keyCode {
	case KeyBackspace:
		f.Delete()
	case KeyDelete, KeyNumPadDelete:
		if f.HasSelectionRange() {
			f.Delete()
		} else if f.selectionStart < len(f.runes) {
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionStart+1:]...)
			f.notifyOfModification()
		}
		f.MarkForRedraw()
	case KeyLeft, KeyNumPadLeft:
		extend := mod.ShiftDown()
		if mod.CommandDown() {
			f.handleHome(extend)
		} else {
			f.handleArrowLeft(extend, mod.OptionDown())
		}
	case KeyRight, KeyNumPadRight:
		extend := mod.ShiftDown()
		if mod.CommandDown() {
			f.handleEnd(extend)
		} else {
			f.handleArrowRight(extend, mod.OptionDown())
		}
	case KeyEnd, KeyNumPadEnd, KeyPageDown, KeyNumPadPageDown, KeyDown, KeyNumPadDown:
		f.handleEnd(mod.ShiftDown())
	case KeyHome, KeyNumPadHome, KeyPageUp, KeyNumPadPageUp, KeyUp, KeyNumPadUp:
		f.handleHome(mod.ShiftDown())
	case KeyTab:
		return false
	}
	return true
}

// DefaultRuneTyped provides the default rune typed handling.
func (f *Field) DefaultRuneTyped(ch rune) bool {
	if wnd := f.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	if unicode.IsControl(ch) {
		return false
	}
	if f.HasSelectionRange() {
		f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
	}
	f.runes = append(f.runes[:f.selectionStart], append([]rune{ch}, f.runes[f.selectionStart:]...)...)
	f.SetSelectionTo(f.selectionStart + 1)
	f.notifyOfModification()
	return true
}

func (f *Field) handleHome(extend bool) {
	if extend {
		f.setSelection(0, f.selectionEnd, f.selectionEnd)
	} else {
		f.SetSelectionToStart()
	}
}

func (f *Field) handleEnd(extend bool) {
	if extend {
		f.SetSelection(f.selectionStart, len(f.runes))
	} else {
		f.SetSelectionToEnd()
	}
}

func (f *Field) handleArrowLeft(extend, byWord bool) {
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionStart == anchor {
				pos := f.selectionEnd - 1
				if byWord {
					start, _ := f.findWordAt(pos)
					pos = xmath.Min(xmath.Max(start, anchor), pos)
				}
				f.setSelection(anchor, pos, anchor)
			} else {
				pos := f.selectionStart - 1
				if byWord {
					start, _ := f.findWordAt(pos)
					pos = xmath.Min(start, pos)
				}
				f.setSelection(pos, anchor, anchor)
			}
		} else {
			f.SetSelectionTo(f.selectionStart)
		}
	} else {
		pos := f.selectionStart - 1
		if byWord {
			start, _ := f.findWordAt(pos)
			pos = xmath.Min(start, pos)
		}
		if extend {
			f.setSelection(pos, f.selectionStart, f.selectionEnd)
		} else {
			f.SetSelectionTo(pos)
		}
	}
}

func (f *Field) handleArrowRight(extend, byWord bool) {
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionEnd == anchor {
				pos := f.selectionStart + 1
				if byWord {
					_, end := f.findWordAt(pos)
					pos = xmath.Max(xmath.Min(end, anchor), pos)
				}
				f.setSelection(pos, anchor, anchor)
			} else {
				pos := f.selectionEnd + 1
				if byWord {
					_, end := f.findWordAt(pos)
					pos = xmath.Max(end, pos)
				}
				f.setSelection(anchor, pos, anchor)
			}
		} else {
			f.SetSelectionTo(f.selectionEnd)
		}
	} else {
		pos := f.selectionEnd + 1
		if byWord {
			_, end := f.findWordAt(pos)
			pos = xmath.Max(end, pos)
		}
		if extend {
			f.SetSelection(f.selectionStart, pos)
		} else {
			f.SetSelectionTo(pos)
		}
	}
}

// DefaultCanPerformCmd provides the default can perform command handling.
func (f *Field) DefaultCanPerformCmd(source interface{}, id int) bool {
	switch id {
	case CutItemID:
		return f.CanCut()
	case CopyItemID:
		return f.CanCopy()
	case PasteItemID:
		return f.CanPaste()
	case DeleteItemID:
		return f.CanDelete()
	case SelectAllItemID:
		return f.CanSelectAll()
	default:
		return false
	}
}

// DefaultPerformCmd provides the default perform command handling.
func (f *Field) DefaultPerformCmd(source interface{}, id int) {
	switch id {
	case CutItemID:
		f.Cut()
	case CopyItemID:
		f.Copy()
	case PasteItemID:
		f.Paste()
	case DeleteItemID:
		f.Delete()
	case SelectAllItemID:
		f.SelectAll()
	default:
	}
}

// CanCut returns true if the field has a selection that can be cut.
func (f *Field) CanCut() bool {
	return f.HasSelectionRange()
}

// Cut the selected text to the clipboard.
func (f *Field) Cut() {
	if f.HasSelectionRange() {
		GlobalClipboard.SetText(f.SelectedText())
		f.Delete()
	}
}

// CanCopy returns true if the field has a selection that can be copied.
func (f *Field) CanCopy() bool {
	return f.HasSelectionRange()
}

// Copy the selected text to the clipboard.
func (f *Field) Copy() {
	if f.HasSelectionRange() {
		GlobalClipboard.SetText(f.SelectedText())
	}
}

// CanPaste returns true if the clipboard has content that can be pasted into the field.
func (f *Field) CanPaste() bool {
	return GlobalClipboard.GetText() != ""
}

// Paste any text on the clipboard into the field.
func (f *Field) Paste() {
	text := GlobalClipboard.GetText()
	if text != "" {
		runes := f.sanitize([]rune(text))
		if f.HasSelectionRange() {
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
		}
		f.runes = append(f.runes[:f.selectionStart], append(runes, f.runes[f.selectionStart:]...)...)
		f.SetSelectionTo(f.selectionStart + len(runes))
		f.notifyOfModification()
	} else if f.HasSelectionRange() {
		f.Delete()
	}
}

// RunesIfPasted returns the resulting runes if the given input was pasted into the field.
func (f *Field) RunesIfPasted(input []rune) []rune {
	runes := f.sanitize(input)
	result := make([]rune, 0, len(runes)+len(f.runes))
	result = append(result, f.runes[:f.selectionStart]...)
	result = append(result, runes...)
	return append(result, f.runes[f.selectionEnd:]...)
}

// CanDelete returns true if the field has a selection that can be deleted.
func (f *Field) CanDelete() bool {
	return f.HasSelectionRange() || f.selectionStart > 0
}

// Delete removes the currently selected text, if any.
func (f *Field) Delete() {
	if f.CanDelete() {
		if f.HasSelectionRange() {
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
			f.SetSelectionTo(f.selectionStart)
		} else {
			f.runes = append(f.runes[:f.selectionStart-1], f.runes[f.selectionStart:]...)
			f.SetSelectionTo(f.selectionStart - 1)
		}
		f.notifyOfModification()
		f.MarkForRedraw()
	}
}

// CanSelectAll returns true if the field's selection can be expanded.
func (f *Field) CanSelectAll() bool {
	return f.selectionStart != 0 || f.selectionEnd != len(f.runes)
}

// SelectAll selects all of the text in the field.
func (f *Field) SelectAll() {
	f.SetSelection(0, len(f.runes))
}

// Text returns the content of the field.
func (f *Field) Text() string {
	return string(f.runes)
}

// SetText sets the content of the field.
func (f *Field) SetText(text string) {
	runes := f.sanitize([]rune(text))
	if !txt.RunesEqual(runes, f.runes) {
		f.runes = runes
		f.SetSelectionToEnd()
		f.notifyOfModification()
	}
}

func (f *Field) notifyOfModification() {
	f.MarkForRedraw()
	if f.ModifiedCallback != nil {
		f.ModifiedCallback()
	}
	f.Validate()
}

// Validate forces field content validation to be run.
func (f *Field) Validate() {
	invalid := false
	if f.ValidateCallback != nil {
		invalid = !f.ValidateCallback()
	}
	if invalid != f.invalid {
		f.invalid = invalid
		f.MarkForRedraw()
	}
}

func (f *Field) sanitize(runes []rune) []rune {
	i := 0
	for _, ch := range runes {
		if ch != '\n' && ch != '\r' {
			runes[i] = ch
			i++
		}
	}
	return runes[:i]
}

// SelectedText returns the currently selected text.
func (f *Field) SelectedText() string {
	return string(f.runes[f.selectionStart:f.selectionEnd])
}

// HasSelectionRange returns true is a selection range is currently present.
func (f *Field) HasSelectionRange() bool {
	return f.selectionStart < f.selectionEnd
}

// SelectionCount returns the number of characters currently selected.
func (f *Field) SelectionCount() int {
	return f.selectionEnd - f.selectionStart
}

// Selection returns the current start and end selection indexes.
func (f *Field) Selection() (start, end int) {
	return f.selectionStart, f.selectionEnd
}

// SetSelectionToStart moves the cursor to the beginning of the text and removes any range that may have been present.
func (f *Field) SetSelectionToStart() {
	f.SetSelection(0, 0)
}

// SetSelectionToEnd moves the cursor to the end of the text and removes any range that may have been present.
func (f *Field) SetSelectionToEnd() {
	f.SetSelection(math.MaxInt64, math.MaxInt64)
}

// SetSelectionTo moves the cursor to the specified index and removes any range that may have been present.
func (f *Field) SetSelectionTo(pos int) {
	f.SetSelection(pos, pos)
}

// SetSelection sets the start and end range of the selection. Values beyond either end will be constrained to the
// appropriate end. Likewise, an end value less than the start value will be treated as if the start and end values were
// the same.
func (f *Field) SetSelection(start, end int) {
	f.setSelection(start, end, start)
}

func (f *Field) setSelection(start, end, anchor int) {
	length := len(f.runes)
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
	if f.selectionStart != start || f.selectionEnd != end || f.selectionAnchor != anchor {
		f.selectionStart = start
		f.selectionEnd = end
		f.selectionAnchor = anchor
		f.forceShowUntil = time.Now().Add(f.BlinkRate)
		f.showCursor = true
		f.MarkForRedraw()
		f.ScrollIntoView()
		f.autoScroll()
	}
}

// ScrollOffset returns the current autoscroll offset.
func (f *Field) ScrollOffset() float32 {
	return f.scrollOffset
}

// SetScrollOffset sets the autoscroll offset to the specified value.
func (f *Field) SetScrollOffset(offset float32) {
	if f.scrollOffset != offset {
		f.scrollOffset = offset
		f.MarkForRedraw()
	}
}

func (f *Field) autoScroll() {
	rect := f.ContentRect(false)
	if rect.Width > 0 {
		original := f.scrollOffset
		if f.selectionStart == f.selectionAnchor {
			right := f.FromSelectionIndex(f.selectionEnd).X
			if right < rect.X {
				f.scrollOffset = 0
				f.scrollOffset = rect.X - f.FromSelectionIndex(f.selectionEnd).X
			} else if right >= rect.X+rect.Width {
				f.scrollOffset = 0
				f.scrollOffset = rect.X + rect.Width - 1 - f.FromSelectionIndex(f.selectionEnd).X
			}
		} else {
			left := f.FromSelectionIndex(f.selectionStart).X
			if left < rect.X {
				f.scrollOffset = 0
				f.scrollOffset = rect.X - f.FromSelectionIndex(f.selectionStart).X
			} else if left >= rect.X+rect.Width {
				f.scrollOffset = 0
				f.scrollOffset = rect.X + rect.Width - 1 - f.FromSelectionIndex(f.selectionStart).X
			}
		}
		save := f.scrollOffset
		f.scrollOffset = 0
		min := rect.X + rect.Width - 1 - f.FromSelectionIndex(len(f.runes)).X
		if min > 0 {
			min = 0
		}
		max := rect.X - f.FromSelectionIndex(0).X
		if max < 0 {
			max = 0
		}
		if save < min {
			save = min
		} else if save > max {
			save = max
		}
		f.scrollOffset = save
		if original != f.scrollOffset {
			f.MarkForRedraw()
		}
	}
}

func (f *Field) textLeft(text *Text, bounds geom.Rect[float32]) float32 {
	left := bounds.X
	switch f.HAlign {
	case MiddleAlignment:
		left += (bounds.Width - text.Width()) / 2
	case EndAlignment:
		left += bounds.Width - text.Width() - 1 // Inset since we leave space for the cursor
	default:
		left++ // Inset since we leave space for the cursor
	}
	return left
}

// ToSelectionIndex returns the rune index for the specified x-coordinate.
func (f *Field) ToSelectionIndex(x float32) int {
	text := NewTextFromRunes(f.runes, &TextDecoration{Font: f.Font})
	return text.RuneIndexForPosition(x - (f.textLeft(text, f.ContentRect(false)) + f.scrollOffset))
}

// FromSelectionIndex returns a location in local coordinates for the specified rune index.
func (f *Field) FromSelectionIndex(index int) geom.Point[float32] {
	text := NewTextFromRunes(f.runes, &TextDecoration{Font: f.Font})
	rect := f.ContentRect(false)
	left := f.textLeft(text, rect)
	x := left + f.scrollOffset
	top := rect.Y + rect.Height/2
	if index > 0 {
		length := len(f.runes)
		if index > length {
			index = length
		}
		x += text.PositionForRuneIndex(index)
	}
	return geom.Point[float32]{X: x, Y: top}
}

func (f *Field) findWordAt(pos int) (start, end int) {
	length := len(f.runes)
	if pos < 0 {
		pos = 0
	} else if pos >= length {
		pos = length - 1
	}
	start = pos
	end = pos
	if length > 0 && !unicode.IsSpace(f.runes[start]) {
		for start > 0 && !unicode.IsSpace(f.runes[start-1]) {
			start--
		}
		for end < length && !unicode.IsSpace(f.runes[end]) {
			end++
		}
	}
	return start, end
}
