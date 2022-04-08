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
	"strings"
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

// Field provides a text input control.
type Field struct {
	Panel
	FieldTheme
	ModifiedCallback func()
	ValidateCallback func() bool
	Watermark        string
	runes            []rune
	lines            []*Text
	endsWithLineFeed []bool
	selectionStart   int
	selectionEnd     int
	selectionAnchor  int
	forceShowUntil   time.Time
	scrollOffset     geom.Point[float32]
	linesBuiltFor    float32
	multiLine        bool
	wrap             bool
	showCursor       bool
	pending          bool
	extendByWord     bool
	invalid          bool
}

// NewField creates a new, empty, field.
func NewField() *Field {
	f := &Field{
		FieldTheme:    DefaultFieldTheme,
		linesBuiltFor: -1,
	}
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

// NewMultiLineField creates a new, empty, multi-line, field.
func NewMultiLineField() *Field {
	f := NewField()
	f.multiLine = true
	f.wrap = true
	return f
}

// AllowsMultipleLines returns true if this field allows embedded line feeds.
func (f *Field) AllowsMultipleLines() bool {
	return f.multiLine
}

// Wrap returns true if this field wraps lines that don't fit the width of the component.
func (f *Field) Wrap() bool {
	return f.wrap
}

// SetWrap sets the wrapping attribute.
func (f *Field) SetWrap(wrap bool) {
	if wrap != f.wrap {
		f.MarkForLayoutAndRedraw()
	}
}

// DefaultSizes provides the default sizing.
func (f *Field) DefaultSizes(hint geom.Size[float32]) (min, pref, max geom.Size[float32]) {
	var insets geom.Insets[float32]
	if b := f.Border(); b != nil {
		insets = b.Insets()
	}
	lines, _ := f.buildLines(hint.Width - (2 + insets.Left + insets.Right))
	for _, line := range lines {
		size := line.Extents()
		if pref.Width < size.Width {
			pref.Width = size.Width
		}
		pref.Height += size.Height
	}
	if pref.Width < f.MinimumTextWidth {
		pref.Width = f.MinimumTextWidth
	}
	if height := f.Font.LineHeight(); pref.Height < height {
		pref.Height = height
	}
	pref.Width += 2 // Allow room for the cursor on either side of the text
	minWidth := f.MinimumTextWidth + 2 + insets.Left + insets.Right
	pref.AddInsets(insets)
	pref.GrowToInteger()
	if hint.Width >= 1 && hint.Width < minWidth {
		hint.Width = minWidth
	}
	pref.ConstrainForHint(hint)
	if hint.Width > 0 && pref.Width < hint.Width {
		pref.Width = hint.Width
	}
	min = pref
	min.Width = minWidth
	return min, pref, MaxSize(pref)
}

func (f *Field) prepareLines(width float32) {
	f.lines, f.endsWithLineFeed = f.buildLines(width)
	f.linesBuiltFor = xmath.Max(width, 0)
}

func (f *Field) prepareLinesForCurrentWidth() {
	f.prepareLines(f.ContentRect(false).Width - 2)
}

func (f *Field) buildLines(wrapWidth float32) (lines []*Text, endsWithLineFeed []bool) {
	if wrapWidth == f.linesBuiltFor && f.linesBuiltFor >= 0 {
		return f.lines, f.endsWithLineFeed
	}
	if len(f.runes) != 0 {
		lines = make([]*Text, 0)
		decoration := &TextDecoration{
			Font:  f.Font,
			Paint: nil,
		}
		if f.multiLine {
			endsWithLineFeed = make([]bool, 0, 16)
			for _, line := range strings.Split(string(f.runes), "\n") {
				one := NewText(line, decoration)
				if f.wrap && wrapWidth > 0 {
					parts := one.BreakToWidth(wrapWidth)
					for i, part := range parts {
						lines = append(lines, part)
						endsWithLineFeed = append(endsWithLineFeed, i == len(parts)-1)
					}
				} else {
					lines = append(lines, one)
					endsWithLineFeed = append(endsWithLineFeed, true)
				}
			}
		} else {
			one := NewTextFromRunes(f.runes, decoration)
			if f.wrap && wrapWidth > 0 {
				lines = append(lines, one.BreakToWidth(wrapWidth)...)
			} else {
				lines = append(lines, one)
			}
			endsWithLineFeed = make([]bool, len(lines))
		}
	}
	return
}

// DefaultDraw provides the default drawing.
func (f *Field) DefaultDraw(canvas *Canvas, dirty geom.Rect[float32]) {
	var fg, bg Ink
	enabled := f.Enabled()
	switch {
	case f.invalid:
		fg = f.ErrorInk
		bg = f.OnErrorInk
	case enabled:
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
	f.prepareLines(rect.Width - 2)
	paint := bg.Paint(canvas, rect, Fill)
	if !enabled {
		paint.SetColorFilter(Grayscale30PercentFilter())
	}
	textTop := rect.Y + f.scrollOffset.Y
	focused := f.Focused()
	hasSelectionRange := f.HasSelectionRange()
	start := 0
	if len(f.runes) == 0 {
		if f.Watermark != "" {
			paint.SetColorFilter(NewAlphaFilter(0.3))
			text := NewText(f.Watermark, &TextDecoration{
				Font:  f.Font,
				Paint: paint,
			})
			text.Draw(canvas, f.textLeft(text, rect)-text.Width(), textTop+text.Baseline())
		}
		if !hasSelectionRange && enabled && focused {
			if f.showCursor {
				rect.X = f.textLeftForWidth(0, rect) + f.scrollOffset.X - 0.5
				rect.Width = 1
				rect.Height = f.Font.LineHeight()
				canvas.DrawRect(rect, bg.Paint(canvas, rect, Fill))
			}
			f.scheduleBlink()
		}
	} else {
		for i, line := range f.lines {
			textLeft := f.textLeft(line, rect)
			textBaseLine := textTop + line.Baseline()
			textHeight := xmath.Max(line.Height(), f.Font.LineHeight())
			end := start + len(line.Runes())
			if f.endsWithLineFeed[i] {
				end++
			}
			if enabled && focused && hasSelectionRange && f.selectionStart < end && f.selectionEnd > start {
				left := textLeft + f.scrollOffset.X
				selStart := xmath.Max(f.selectionStart, start)
				selEnd := xmath.Min(f.selectionEnd, end)
				if selStart > start {
					t := NewTextFromRunes(f.runes[start:selStart], &TextDecoration{
						Font:  f.Font,
						Paint: paint,
					})
					t.Draw(canvas, left, textBaseLine)
					left += t.Width()
				}
				t := NewTextFromRunes(f.runes[selStart:selEnd], &TextDecoration{
					Font:  f.Font,
					Paint: f.OnSelectionInk.Paint(canvas, rect, Fill),
				})
				right := left + t.Width()
				selRect := geom.Rect[float32]{
					Point: geom.Point[float32]{X: left, Y: textTop},
					Size:  geom.Size[float32]{Width: right - left, Height: textHeight},
				}
				canvas.DrawRect(selRect, f.SelectionInk.Paint(canvas, selRect, Fill))
				t.Draw(canvas, left, textBaseLine)
				if selEnd < end {
					e := end
					if f.endsWithLineFeed[i] {
						e--
					}
					NewTextFromRunes(f.runes[selEnd:e], &TextDecoration{
						Font:  f.Font,
						Paint: paint,
					}).Draw(canvas, right, textBaseLine)
				}
			} else {
				line.ReplacePaint(paint)
				line.Draw(canvas, textLeft+f.scrollOffset.X, textBaseLine)
			}
			if !hasSelectionRange && enabled && focused && f.selectionEnd >= start && (f.selectionEnd < end || (!f.multiLine && f.selectionEnd <= end)) {
				if f.showCursor {
					t := NewTextFromRunes(f.runes[start:f.selectionEnd], &TextDecoration{
						Font:  f.Font,
						Paint: nil,
					})
					canvas.DrawRect(geom.NewRect(textLeft+t.Width()+f.scrollOffset.X-0.5, textTop, 1, textHeight),
						bg.Paint(canvas, rect, Fill))
				}
				f.scheduleBlink()
			}
			textTop += textHeight
			start = end
		}
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
			start, end := f.findWordAt(f.ToSelectionIndex(where))
			f.SetSelection(start, end)
			f.extendByWord = true
		case 3:
			f.SelectAll()
		default:
			oldAnchor := f.selectionAnchor
			f.selectionAnchor = f.ToSelectionIndex(where)
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
	pos := f.ToSelectionIndex(where)
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
	case KeyDelete:
		if f.HasSelectionRange() {
			f.Delete()
		} else if f.selectionStart < len(f.runes) {
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionStart+1:]...)
			f.linesBuiltFor = -1
			f.notifyOfModification()
		}
		f.MarkForRedraw()
	case KeyLeft:
		extend := mod.ShiftDown()
		if mod.CommandDown() {
			f.handleHome(extend)
		} else {
			f.handleArrowLeft(extend, mod.OptionDown())
		}
	case KeyRight:
		extend := mod.ShiftDown()
		if mod.CommandDown() {
			f.handleEnd(extend)
		} else {
			f.handleArrowRight(extend, mod.OptionDown())
		}
	case KeyEnd, KeyPageDown:
		f.handleEnd(mod.ShiftDown())
	case KeyHome, KeyPageUp:
		f.handleHome(mod.ShiftDown())
	case KeyDown:
		if f.multiLine {
			extend := mod.ShiftDown()
			if mod.CommandDown() {
				f.handleEnd(extend)
			} else {
				f.handleArrowDown(extend, mod.OptionDown())
			}
		} else {
			f.handleEnd(mod.ShiftDown())
		}
	case KeyUp:
		if f.multiLine {
			extend := mod.ShiftDown()
			if mod.CommandDown() {
				f.handleHome(extend)
			} else {
				f.handleArrowUp(extend, mod.OptionDown())
			}
		} else {
			f.handleHome(mod.ShiftDown())
		}
	case KeyTab:
		return false
	case KeyReturn, KeyNumPadEnter:
		if f.multiLine {
			f.DefaultRuneTyped('\n')
		}
	}
	return true
}

// DefaultRuneTyped provides the default rune typed handling.
func (f *Field) DefaultRuneTyped(ch rune) bool {
	if wnd := f.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	if unicode.IsControl(ch) && (!f.multiLine || ch != '\n') {
		return false
	}
	if f.HasSelectionRange() {
		f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
	}
	f.runes = append(f.runes[:f.selectionStart], append([]rune{ch}, f.runes[f.selectionStart:]...)...)
	f.linesBuiltFor = -1
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

func (f *Field) handleArrowUp(extend, byWord bool) {
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionStart == anchor {
				pt := f.FromSelectionIndex(f.selectionEnd)
				pt.Y -= f.Font.LineHeight()
				pos := f.ToSelectionIndex(pt)
				if byWord {
					start, _ := f.findWordAt(pos)
					pos = xmath.Min(xmath.Max(start, anchor), pos)
				}
				f.setSelection(anchor, pos, anchor)
			} else {
				pt := f.FromSelectionIndex(f.selectionStart)
				pt.Y -= f.Font.LineHeight()
				pos := f.ToSelectionIndex(pt)
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
		pt := f.FromSelectionIndex(f.selectionStart)
		pt.Y -= f.Font.LineHeight()
		pos := f.ToSelectionIndex(pt)
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

func (f *Field) handleArrowDown(extend, byWord bool) {
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionEnd == anchor {
				pt := f.FromSelectionIndex(f.selectionStart)
				pt.Y += f.Font.LineHeight()
				pos := f.ToSelectionIndex(pt)
				if byWord {
					_, end := f.findWordAt(pos)
					pos = xmath.Max(xmath.Min(end, anchor), pos)
				}
				f.setSelection(pos, anchor, anchor)
			} else {
				pt := f.FromSelectionIndex(f.selectionEnd)
				pt.Y += f.Font.LineHeight()
				pos := f.ToSelectionIndex(pt)
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
		pt := f.FromSelectionIndex(f.selectionEnd)
		pt.Y += f.Font.LineHeight()
		pos := f.ToSelectionIndex(pt)
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
		f.linesBuiltFor = -1
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
		f.linesBuiltFor = -1
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
		f.linesBuiltFor = -1
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
	if f.multiLine {
		return runes
	}
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
func (f *Field) ScrollOffset() geom.Point[float32] {
	return f.scrollOffset
}

// SetScrollOffset sets the autoscroll offset to the specified value.
func (f *Field) SetScrollOffset(offset geom.Point[float32]) {
	if f.scrollOffset != offset {
		f.scrollOffset = offset
		f.MarkForRedraw()
	}
}

func (f *Field) autoScroll() {
	rect := f.ContentRect(false)
	original := f.scrollOffset //nolint:ifshort // Can't do this later
	if rect.Width > 0 {
		if f.selectionStart == f.selectionAnchor {
			right := f.FromSelectionIndex(f.selectionEnd).X
			if right < rect.X {
				f.scrollOffset.X = 0
				f.scrollOffset.X = rect.X - f.FromSelectionIndex(f.selectionEnd).X
			} else if right >= rect.Right() {
				f.scrollOffset.X = 0
				f.scrollOffset.X = rect.Right() - 1 - f.FromSelectionIndex(f.selectionEnd).X
			}
		} else {
			left := f.FromSelectionIndex(f.selectionStart).X
			if left < rect.X {
				f.scrollOffset.X = 0
				f.scrollOffset.X = rect.X - f.FromSelectionIndex(f.selectionStart).X
			} else if left >= rect.Right() {
				f.scrollOffset.X = 0
				f.scrollOffset.X = rect.Right() - 1 - f.FromSelectionIndex(f.selectionStart).X
			}
		}
		save := f.scrollOffset.X
		f.scrollOffset.X = 0
		min := rect.Right() - 1 - f.FromSelectionIndex(len(f.runes)).X
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
		f.scrollOffset.X = save
	}
	if rect.Height > 0 {
		if f.selectionStart == f.selectionAnchor {
			top := f.FromSelectionIndex(f.selectionEnd).Y
			if top < rect.Y {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Y - f.FromSelectionIndex(f.selectionEnd).Y
			} else if top+f.Font.LineHeight() >= rect.Bottom() {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Bottom() - 1 - (f.FromSelectionIndex(f.selectionEnd).Y + f.Font.LineHeight())
			}
		} else {
			top := f.FromSelectionIndex(f.selectionStart).Y
			if top < rect.Y {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Y - f.FromSelectionIndex(f.selectionStart).Y
			} else if top+f.Font.LineHeight() >= rect.Bottom() {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Bottom() - 1 - (f.FromSelectionIndex(f.selectionStart).Y + f.Font.LineHeight())
			}
		}
		save := f.scrollOffset.Y
		f.scrollOffset.Y = 0
		min := rect.Bottom() - 1 - (f.FromSelectionIndex(len(f.runes)).Y + f.Font.LineHeight())
		if min > 0 {
			min = 0
		}
		max := rect.Y - (f.FromSelectionIndex(0).Y + f.Font.LineHeight())
		if max < 0 {
			max = 0
		}
		if save < min {
			save = min
		} else if save > max {
			save = max
		}
		f.scrollOffset.Y = save
	}
	if original != f.scrollOffset {
		f.MarkForRedraw()
	}
}

func (f *Field) textLeft(text *Text, bounds geom.Rect[float32]) float32 {
	return f.textLeftForWidth(text.Width(), bounds)
}

func (f *Field) textLeftForWidth(width float32, bounds geom.Rect[float32]) float32 {
	left := bounds.X
	switch f.HAlign {
	case MiddleAlignment:
		left += (bounds.Width - width) / 2
	case EndAlignment:
		left += bounds.Width - width - 1 // Inset since we leave space for the cursor
	default:
		left++ // Inset since we leave space for the cursor
	}
	return left
}

// ToSelectionIndex returns the rune index for the coordinates.
func (f *Field) ToSelectionIndex(where geom.Point[float32]) int {
	if where.Y < 0 {
		return 0
	}
	f.prepareLinesForCurrentWidth()
	y := f.scrollOffset.Y
	pos := 0
	for i, line := range f.lines {
		lineHeight := xmath.Max(line.Height(), f.Font.LineHeight())
		if where.Y >= y && where.Y < y+lineHeight {
			return pos + line.RuneIndexForPosition(where.X-(f.textLeft(line, f.ContentRect(false))+f.scrollOffset.X))
		}
		y += lineHeight
		pos += len(line.Runes())
		if f.endsWithLineFeed[i] {
			pos++
		}
	}
	return len(f.runes)
}

// FromSelectionIndex returns a location in local coordinates for the specified rune index.
func (f *Field) FromSelectionIndex(index int) geom.Point[float32] {
	index = xmath.Max(xmath.Min(index, len(f.runes)), 0)
	f.prepareLinesForCurrentWidth()
	rect := f.ContentRect(false)
	y := rect.Y + f.scrollOffset.Y
	pos := 0
	for i, line := range f.lines {
		lineLength := len(line.Runes())
		if lineLength >= index-pos {
			return geom.NewPoint(f.textLeft(line, rect)+line.PositionForRuneIndex(index-pos)+f.scrollOffset.X, y)
		}
		y += xmath.Max(line.Height(), f.Font.LineHeight())
		if f.endsWithLineFeed[i] {
			lineLength++
		}
		pos += lineLength
	}
	return geom.NewPoint(f.textLeftForWidth(0, rect)+f.scrollOffset.X, y)
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
