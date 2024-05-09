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

	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/txt"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
)

type lineEndingType byte

const (
	noLineEnding lineEndingType = iota
	hardLineEnding
	softLineEnding
)

// DefaultFieldTheme holds the default FieldTheme values for Fields. Modifying this data will not alter existing Fields,
// but will alter any Fields created in the future.
var DefaultFieldTheme = FieldTheme{
	Font:             FieldFont,
	BackgroundInk:    ThemeSurface,
	OnBackgroundInk:  ThemeOnSurface,
	EditableInk:      ThemeBelowSurface,
	OnEditableInk:    ThemeOnBelowSurface,
	SelectionInk:     ThemeFocus,
	OnSelectionInk:   ThemeOnFocus,
	ErrorInk:         ThemeError,
	OnErrorInk:       ThemeOnError,
	BlinkRate:        560 * time.Millisecond,
	MinimumTextWidth: 10,
	HAlign:           align.Start,
}

// FieldTheme holds theming data for a Field.
type FieldTheme struct {
	InitialClickSelectsAll func(*Field) bool
	Font                   Font
	BackgroundInk          Ink
	OnBackgroundInk        Ink
	EditableInk            Ink
	OnEditableInk          Ink
	SelectionInk           Ink
	OnSelectionInk         Ink
	ErrorInk               Ink
	OnErrorInk             Ink
	BlinkRate              time.Duration
	MinimumTextWidth       float32
	HAlign                 align.Enum
}

// Field provides a text input control.
type Field struct {
	Panel
	FieldTheme
	ModifiedCallback   func(before, after *FieldState)
	ValidateCallback   func() bool
	Watermark          string
	undoID             int64
	runes              []rune
	lines              []*Text
	endsWithLineFeed   []lineEndingType
	selectionStart     int
	selectionEnd       int
	selectionAnchor    int
	forceShowUntil     time.Time
	scrollOffset       Point
	linesBuiltFor      float32
	ObscurementRune    rune
	AutoScroll         bool
	NoSelectAllOnFocus bool
	multiLine          bool
	wrap               bool
	showCursor         bool
	pending            bool
	extendByWord       bool
	invalid            bool
}

// FieldState holds the text and selection data for the field.
type FieldState struct {
	Text            string
	SelectionStart  int
	SelectionEnd    int
	SelectionAnchor int
}

// NewField creates a new, empty, field.
func NewField() *Field {
	f := &Field{
		FieldTheme:    DefaultFieldTheme,
		undoID:        NextUndoID(),
		linesBuiltFor: -1,
		AutoScroll:    true,
	}
	f.Self = f
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
	f.InstallCmdHandlers(CutItemID, func(_ any) bool { return f.CanCut() }, func(_ any) { f.Cut() })
	f.InstallCmdHandlers(CopyItemID, func(_ any) bool { return f.CanCopy() }, func(_ any) { f.Copy() })
	f.InstallCmdHandlers(PasteItemID, func(_ any) bool { return f.CanPaste() }, func(_ any) { f.Paste() })
	f.InstallCmdHandlers(DeleteItemID, func(_ any) bool { return f.CanDelete() }, func(_ any) { f.Delete() })
	f.InstallCmdHandlers(SelectAllItemID, func(_ any) bool { return f.CanSelectAll() }, func(_ any) { f.SelectAll() })
	InstallDefaultFieldBorder(f, f)
	return f
}

// NewMultiLineField creates a new, empty, multi-line, field.
func NewMultiLineField() *Field {
	f := NewField()
	f.multiLine = true
	f.wrap = true
	return f
}

// CurrentUndoID returns the undo ID to use.
func (f *Field) CurrentUndoID() int64 {
	return f.undoID
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
		f.wrap = wrap
		f.MarkForLayoutAndRedraw()
	}
}

// SetMinimumTextWidthUsing sets the MinimumTextWidth by measuring the provided candidates and using the widest.
func (f *Field) SetMinimumTextWidthUsing(candidates ...string) {
	var width float32
	f.MinimumTextWidth = 10
	for _, one := range candidates {
		if width = NewText(one, &TextDecoration{Font: f.Font}).Width(); width > f.MinimumTextWidth {
			f.MinimumTextWidth = width
		}
	}
}

// DefaultSizes provides the default sizing.
func (f *Field) DefaultSizes(hint Size) (minSize, prefSize, maxSize Size) {
	var insets Insets
	if b := f.Border(); b != nil {
		insets = b.Insets()
	}
	lines, _ := f.buildLines(hint.Width - (2 + insets.Width()))
	for _, line := range lines {
		size := line.Extents()
		if prefSize.Width < size.Width {
			prefSize.Width = size.Width
		}
		prefSize.Height += size.Height
	}
	if prefSize.Width < f.MinimumTextWidth {
		prefSize.Width = f.MinimumTextWidth
	}
	if height := f.Font.LineHeight(); prefSize.Height < height {
		prefSize.Height = height
	}
	prefSize.Width += 2 // Allow room for the cursor on either side of the text
	minWidth := f.MinimumTextWidth + 2 + insets.Width()
	prefSize = prefSize.Add(insets.Size()).Ceil()
	if hint.Width >= 1 && hint.Width < minWidth {
		hint.Width = minWidth
	}
	prefSize = prefSize.ConstrainForHint(hint)
	if hint.Width > 0 && prefSize.Width < hint.Width {
		prefSize.Width = hint.Width
	}
	minSize = prefSize
	minSize.Width = minWidth
	return minSize, prefSize, MaxSize(prefSize)
}

func (f *Field) prepareLines(width float32) {
	width = max(width, 0)
	f.lines, f.endsWithLineFeed = f.buildLines(width)
	f.linesBuiltFor = width
}

func (f *Field) prepareLinesForCurrentWidth() {
	f.prepareLines(f.ContentRect(false).Width - 2)
}

func (f *Field) buildLines(wrapWidth float32) (lines []*Text, endsWithLineFeed []lineEndingType) {
	if wrapWidth == f.linesBuiltFor && f.linesBuiltFor >= 0 {
		return f.lines, f.endsWithLineFeed
	}
	if len(f.runes) != 0 {
		lines = make([]*Text, 0)
		decoration := &TextDecoration{Font: f.Font}
		if f.multiLine {
			endsWithLineFeed = make([]lineEndingType, 0, 16)
			for _, line := range strings.Split(string(f.runes), "\n") {
				one := NewText(f.obscureStringIfNeeded(line), decoration)
				if f.wrap && wrapWidth > 0 {
					parts := one.BreakToWidth(wrapWidth)
					for i, part := range parts {
						lines = append(lines, part)
						var eol lineEndingType
						if i == len(parts)-1 {
							eol = hardLineEnding
						} else {
							eol = softLineEnding
						}
						endsWithLineFeed = append(endsWithLineFeed, eol)
					}
				} else {
					lines = append(lines, one)
					endsWithLineFeed = append(endsWithLineFeed, hardLineEnding)
				}
			}
		} else {
			one := NewTextFromRunes(f.obscureIfNeeded(f.runes), decoration)
			if f.wrap && wrapWidth > 0 {
				lines = append(lines, one.BreakToWidth(wrapWidth)...)
			} else {
				lines = append(lines, one)
			}
			endsWithLineFeed = make([]lineEndingType, len(lines))
		}
	}
	return
}

func (f *Field) obscureStringIfNeeded(in string) string {
	if f.ObscurementRune == 0 {
		return in
	}
	r := []rune(in)
	replacement := make([]rune, len(r))
	for i := range r {
		replacement[i] = f.ObscurementRune
	}
	return string(replacement)
}

func (f *Field) obscureIfNeeded(in []rune) []rune {
	if f.ObscurementRune == 0 {
		return in
	}
	replacement := make([]rune, len(in))
	for i := range in {
		replacement[i] = f.ObscurementRune
	}
	return replacement
}

// DefaultDraw provides the default drawing.
func (f *Field) DefaultDraw(canvas *Canvas, _ Rect) {
	var bg, fg Ink
	enabled := f.Enabled()
	switch {
	case f.invalid:
		bg = f.ErrorInk
		fg = f.OnErrorInk
	case enabled:
		bg = f.EditableInk
		fg = f.OnEditableInk
	default:
		bg = f.BackgroundInk
		fg = f.OnBackgroundInk
	}
	rect := f.ContentRect(true)
	canvas.DrawRect(rect, bg.Paint(canvas, rect, paintstyle.Fill))
	rect = f.ContentRect(false)
	canvas.ClipRect(rect, pathop.Intersect, false)
	f.prepareLines(rect.Width - 2)
	ink := fg
	if !enabled {
		ink = &ColorFilteredInk{
			OriginalInk: ink,
			ColorFilter: Grayscale30Filter(),
		}
	}
	textTop := rect.Y + f.scrollOffset.Y
	focused := f.Focused()
	hasSelectionRange := f.HasSelectionRange()
	start := 0
	if len(f.runes) == 0 {
		if f.Watermark != "" {
			text := NewText(f.Watermark, &TextDecoration{
				Font: f.Font,
				Foreground: &ColorFilteredInk{
					OriginalInk: ink,
					ColorFilter: Alpha30Filter(),
				},
			})
			text.Draw(canvas, f.textLeft(text, rect), textTop+text.Baseline())
		}
		if !hasSelectionRange && enabled && focused {
			if f.showCursor {
				rect.X = f.textLeftForWidth(0, rect) + f.scrollOffset.X - 0.5
				rect.Width = 1
				rect.Height = f.Font.LineHeight()
				canvas.DrawRect(rect, fg.Paint(canvas, rect, paintstyle.Fill))
			}
			f.scheduleBlink()
		}
	} else {
		for i, line := range f.lines {
			textLeft := f.textLeft(line, rect)
			textBaseLine := textTop + line.Baseline()
			textHeight := max(line.Height(), f.Font.LineHeight())
			end := start + len(line.Runes())
			if f.endsWithLineFeed[i] == hardLineEnding {
				end++
			}
			if enabled && focused && hasSelectionRange && f.selectionStart < end && f.selectionEnd > start {
				left := textLeft + f.scrollOffset.X
				selStart := max(f.selectionStart, start)
				selEnd := min(f.selectionEnd, end)
				if selStart > start {
					t := NewTextFromRunes(f.obscureIfNeeded(f.runes[start:selStart]), &TextDecoration{
						Font:       f.Font,
						Foreground: ink,
					})
					t.Draw(canvas, left, textBaseLine)
					left += t.Width()
				}
				e := selEnd
				if end == selEnd && f.endsWithLineFeed[i] == hardLineEnding {
					e--
				}
				t := NewTextFromRunes(f.obscureIfNeeded(f.runes[selStart:e]), &TextDecoration{
					Font:       f.Font,
					Foreground: f.OnSelectionInk,
				})
				right := left + t.Width()
				selRect := Rect{
					Point: Point{X: left, Y: textTop},
					Size:  Size{Width: right - left, Height: textHeight},
				}
				canvas.DrawRect(selRect, f.SelectionInk.Paint(canvas, selRect, paintstyle.Fill))
				t.Draw(canvas, left, textBaseLine)
				if selEnd < end {
					e = end
					if f.endsWithLineFeed[i] == hardLineEnding {
						e--
					}
					NewTextFromRunes(f.obscureIfNeeded(f.runes[selEnd:e]), &TextDecoration{
						Font:       f.Font,
						Foreground: ink,
					}).Draw(canvas, right, textBaseLine)
				}
			} else {
				line.AdjustDecorations(func(decoration *TextDecoration) { decoration.Foreground = ink })
				line.Draw(canvas, textLeft+f.scrollOffset.X, textBaseLine)
			}
			if !hasSelectionRange && enabled && focused && f.selectionEnd >= start && (f.selectionEnd < end || (!f.multiLine && f.selectionEnd <= end)) {
				if f.showCursor {
					t := NewTextFromRunes(f.obscureIfNeeded(f.runes[start:f.selectionEnd]), &TextDecoration{Font: f.Font})
					canvas.DrawRect(Rect{
						Point: Point{X: textLeft + t.Width() + f.scrollOffset.X - 0.5, Y: textTop},
						Size:  Size{Width: 1, Height: textHeight},
					}, fg.Paint(canvas, rect, paintstyle.Fill))
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
	if !f.NoSelectAllOnFocus && !f.HasSelectionRange() {
		f.SelectAll()
	}
	f.showCursor = true
	f.ScrollSelectionIntoView()
	f.MarkForRedraw()
}

// DefaultFocusLost provides the default focus lost handling.
func (f *Field) DefaultFocusLost() {
	f.undoID = NextUndoID()
	f.MarkForRedraw()
}

// DefaultMouseDown provides the default mouse down handling.
func (f *Field) DefaultMouseDown(where Point, button, clickCount int, mod Modifiers) bool {
	f.undoID = NextUndoID()
	wasFocused := f.Focused()
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
			selectAll := false
			if !wasFocused {
				if f.InitialClickSelectsAll != nil {
					toolbox.Call(func() { selectAll = f.InitialClickSelectsAll(f) })
				}
			}
			if selectAll {
				f.setSelection(0, len(f.runes), f.ToSelectionIndex(where))
			} else {
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
		}
		return true
	}
	return false
}

// DefaultMouseDrag provides the default mouse drag handling.
func (f *Field) DefaultMouseDrag(where Point, _ int, _ Modifiers) bool {
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
func (f *Field) DefaultUpdateCursor(_ Point) *Cursor {
	if f.Enabled() {
		return TextCursor()
	}
	return ArrowCursor()
}

// DefaultKeyDown provides the default key down handling.
func (f *Field) DefaultKeyDown(keyCode KeyCode, mod Modifiers, _ bool) bool {
	if wnd := f.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	if mod.OSMenuCmdModifierDown() {
		switch keyCode {
		case KeyRight:
			f.handleEnd(f.multiLine, mod.ShiftDown())
		case KeyDown:
			f.handleEnd(false, mod.ShiftDown())
		case KeyLeft:
			f.handleHome(f.multiLine, mod.ShiftDown())
		case KeyUp:
			f.handleHome(false, mod.ShiftDown())
		}
		return false
	}
	switch keyCode {
	case KeyBackspace:
		f.Delete()
	case KeyDelete:
		if f.HasSelectionRange() {
			f.Delete()
		} else if f.selectionStart < len(f.runes) {
			before := f.GetFieldState()
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionStart+1:]...)
			f.linesBuiltFor = -1
			f.notifyOfModification(before, f.GetFieldState())
		}
		f.MarkForRedraw()
	case KeyLeft:
		f.handleArrowLeft(mod.ShiftDown(), mod.OptionDown())
	case KeyRight:
		f.handleArrowRight(mod.ShiftDown(), mod.OptionDown())
	case KeyEnd:
		f.handleEnd(f.multiLine, mod.ShiftDown())
	case KeyPageDown:
		if !f.multiLine {
			return false
		}
		f.handleEnd(false, mod.ShiftDown())
	case KeyHome:
		f.handleHome(f.multiLine, mod.ShiftDown())
	case KeyPageUp:
		if !f.multiLine {
			return false
		}
		f.handleHome(false, mod.ShiftDown())
	case KeyDown:
		if f.multiLine {
			f.handleArrowDown(mod.ShiftDown(), mod.OptionDown())
		} else {
			f.handleEnd(false, mod.ShiftDown())
		}
	case KeyUp:
		if f.multiLine {
			f.handleArrowUp(mod.ShiftDown(), mod.OptionDown())
		} else {
			f.handleHome(false, mod.ShiftDown())
		}
	case KeyTab:
		return false
	case KeyReturn, KeyNumPadEnter:
		f.undoID = NextUndoID()
		if f.multiLine {
			f.DefaultRuneTyped('\n')
		} else {
			return false
		}
	case KeyEscape:
		return false
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
	before := f.GetFieldState()
	if f.HasSelectionRange() {
		f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
	}
	f.runes = append(f.runes[:f.selectionStart], append([]rune{ch}, f.runes[f.selectionStart:]...)...)
	f.linesBuiltFor = -1
	f.SetSelectionTo(f.selectionStart + 1)
	f.notifyOfModification(before, f.GetFieldState())
	return true
}

func (f *Field) handleHome(lineOnly, extend bool) {
	f.undoID = NextUndoID()
	switch {
	case lineOnly:
		var start int
		if f.selectionStart == 0 || f.runes[f.selectionStart-1] == '\n' {
			start = f.findPrevLineBreak(f.selectionStart + 1)
		} else {
			start = f.findPrevLineBreak(f.selectionStart)
		}
		if start != 0 {
			start++
		}
		if extend {
			f.setSelection(start, f.selectionEnd, f.selectionEnd)
		} else {
			f.SetSelectionTo(start)
		}
	case extend:
		f.setSelection(0, f.selectionEnd, f.selectionEnd)
	default:
		f.SetSelectionToStart()
	}
}

func (f *Field) handleEnd(lineOnly, extend bool) {
	f.undoID = NextUndoID()
	switch {
	case lineOnly:
		var end int
		if f.selectionEnd == len(f.runes) || f.runes[f.selectionEnd] == '\n' {
			end = f.findNextLineBreak(f.selectionEnd - 1)
		} else {
			end = f.findNextLineBreak(f.selectionEnd)
		}
		if extend {
			f.setSelection(f.selectionStart, end, f.selectionStart)
		} else {
			f.SetSelectionTo(end)
		}
	case extend:
		f.SetSelection(f.selectionStart, len(f.runes))
	default:
		f.SetSelectionToEnd()
	}
}

func (f *Field) scanLeftToWordPart(pos int) int {
	if pos >= len(f.runes) {
		pos = len(f.runes) - 1
	}
	if pos < 0 {
		return 0
	}
	for pos > 0 && !f.isWordPart(pos) {
		pos--
	}
	return pos
}

func (f *Field) scanRightToWordPart(pos int) int {
	if pos >= len(f.runes) {
		return max(len(f.runes)-1, 0)
	}
	if pos < 0 {
		pos = 0
	}
	for pos < len(f.runes)-1 && !f.isWordPart(pos) {
		pos++
	}
	return pos
}

func (f *Field) handleArrowLeft(extend, byWord bool) {
	f.undoID = NextUndoID()
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionStart == anchor {
				pos := f.selectionEnd - 1
				if byWord {
					start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
					pos = min(max(start, anchor), pos)
				}
				f.setSelection(anchor, pos, anchor)
			} else {
				pos := f.selectionStart - 1
				if byWord {
					start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
					pos = min(start, pos)
				}
				f.setSelection(pos, anchor, anchor)
			}
		} else {
			f.SetSelectionTo(f.selectionStart)
		}
	} else {
		pos := f.selectionStart - 1
		if byWord {
			start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
			pos = min(start, pos)
		}
		if extend {
			f.setSelection(pos, f.selectionStart, f.selectionEnd)
		} else {
			f.SetSelectionTo(pos)
		}
	}
}

func (f *Field) handleArrowRight(extend, byWord bool) {
	f.undoID = NextUndoID()
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionEnd == anchor {
				pos := f.selectionStart + 1
				if byWord {
					_, end := f.findWordAt(f.scanRightToWordPart(pos))
					pos = max(min(end, anchor), pos)
				}
				f.setSelection(pos, anchor, anchor)
			} else {
				pos := f.selectionEnd + 1
				if byWord {
					_, end := f.findWordAt(f.scanRightToWordPart(pos))
					pos = max(end, pos)
				}
				f.setSelection(anchor, pos, anchor)
			}
		} else {
			f.SetSelectionTo(f.selectionEnd)
		}
	} else {
		pos := f.selectionEnd + 1
		if byWord {
			_, end := f.findWordAt(f.scanRightToWordPart(pos))
			pos = max(end, pos)
		}
		if extend {
			f.SetSelection(f.selectionStart, pos)
		} else {
			f.SetSelectionTo(pos)
		}
	}
}

func (f *Field) handleArrowUp(extend, byWord bool) {
	f.undoID = NextUndoID()
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionStart == anchor {
				pt := f.FromSelectionIndex(f.selectionEnd)
				pt.Y--
				pos := f.ToSelectionIndex(pt)
				if byWord {
					start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
					pos = min(max(start, anchor), pos)
				}
				f.setSelection(anchor, pos, anchor)
			} else {
				pt := f.FromSelectionIndex(f.selectionStart)
				pt.Y--
				pos := f.ToSelectionIndex(pt)
				if byWord {
					start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
					pos = min(start, pos)
				}
				f.setSelection(pos, anchor, anchor)
			}
		} else {
			f.SetSelectionTo(f.selectionStart)
		}
	} else {
		pt := f.FromSelectionIndex(f.selectionStart)
		pt.Y--
		pos := f.ToSelectionIndex(pt)
		if byWord {
			start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
			pos = min(start, pos)
		}
		if extend {
			f.setSelection(pos, f.selectionStart, f.selectionEnd)
		} else {
			f.SetSelectionTo(pos)
		}
	}
}

func (f *Field) handleArrowDown(extend, byWord bool) {
	f.undoID = NextUndoID()
	if f.HasSelectionRange() {
		if extend {
			anchor := f.selectionAnchor
			if f.selectionEnd == anchor {
				pt := f.FromSelectionIndex(f.selectionStart)
				pt.Y += 1 + f.lineHeightAt(pt.Y)
				pos := f.ToSelectionIndex(pt)
				if byWord {
					_, end := f.findWordAt(f.scanRightToWordPart(pos))
					pos = max(min(end, anchor), pos)
				}
				f.setSelection(pos, anchor, anchor)
			} else {
				pt := f.FromSelectionIndex(f.selectionEnd)
				pt.Y += 1 + f.lineHeightAt(pt.Y)
				pos := f.ToSelectionIndex(pt)
				if byWord {
					_, end := f.findWordAt(f.scanRightToWordPart(pos))
					pos = max(end, pos)
				}
				f.setSelection(anchor, pos, anchor)
			}
		} else {
			f.SetSelectionTo(f.selectionEnd)
		}
	} else {
		pt := f.FromSelectionIndex(f.selectionEnd)
		pt.Y += 1 + f.lineHeightAt(pt.Y)
		pos := f.ToSelectionIndex(pt)
		if byWord {
			_, end := f.findWordAt(f.scanRightToWordPart(pos))
			pos = max(end, pos)
		}
		if extend {
			f.SetSelection(f.selectionStart, pos)
		} else {
			f.SetSelectionTo(pos)
		}
	}
}

func (f *Field) lineHeightAt(y float32) float32 {
	if len(f.lines) == 0 {
		return f.Font.LineHeight()
	}
	index, _ := f.lineIndexForY(y)
	return max(f.lines[index].Height(), f.Font.LineHeight())
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
		f.undoID = NextUndoID()
		before := f.GetFieldState()
		runes := f.sanitize([]rune(text))
		if f.HasSelectionRange() {
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
		}
		f.runes = append(f.runes[:f.selectionStart], append(runes, f.runes[f.selectionStart:]...)...)
		f.linesBuiltFor = -1
		f.SetSelectionTo(f.selectionStart + len(runes))
		f.notifyOfModification(before, f.GetFieldState())
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
		f.undoID = NextUndoID()
		before := f.GetFieldState()
		f.linesBuiltFor = -1
		if f.HasSelectionRange() {
			f.runes = append(f.runes[:f.selectionStart], f.runes[f.selectionEnd:]...)
			f.SetSelectionTo(f.selectionStart)
		} else {
			f.runes = append(f.runes[:f.selectionStart-1], f.runes[f.selectionStart:]...)
			f.SetSelectionTo(f.selectionStart - 1)
		}
		f.notifyOfModification(before, f.GetFieldState())
		f.MarkForRedraw()
	}
}

// CanSelectAll returns true if the field's selection can be expanded.
func (f *Field) CanSelectAll() bool {
	return f.selectionStart != 0 || f.selectionEnd != len(f.runes)
}

// SelectAll selects all of the text in the field.
func (f *Field) SelectAll() {
	f.undoID = NextUndoID()
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
		before := f.GetFieldState()
		f.runes = runes
		f.linesBuiltFor = -1
		f.SetSelectionToEnd()
		f.notifyOfModification(before, f.GetFieldState())
	}
}

func (f *Field) notifyOfModification(before, after *FieldState) {
	f.MarkForRedraw()
	if f.ModifiedCallback != nil {
		f.ModifiedCallback(before, after)
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
		if ch >= ' ' || ch == '\t' || (f.multiLine && ch == '\n') {
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
	f.SetSelection(math.MaxInt32, math.MaxInt32)
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
		f.ScrollSelectionIntoView()
	}
}

// ScrollSelectionIntoView scrolls the selection into view.
func (f *Field) ScrollSelectionIntoView() {
	f.autoScroll()
	var pos int
	if f.selectionAnchor == f.selectionStart {
		pos = f.selectionEnd
	} else {
		pos = f.selectionStart
	}
	pt := f.FromSelectionIndex(pos)
	f.ScrollRectIntoView(Rect{Point: Point{X: pt.X - 1, Y: pt.Y}, Size: Size{Width: 3, Height: f.lineHeightAt(pt.Y)}})
}

// ScrollOffset returns the current autoscroll offset.
func (f *Field) ScrollOffset() Point {
	return f.scrollOffset
}

// SetScrollOffset sets the autoscroll offset to the specified value.
func (f *Field) SetScrollOffset(offset Point) {
	if f.AutoScroll && f.scrollOffset != offset {
		f.scrollOffset = offset
		f.MarkForRedraw()
	}
}

func (f *Field) autoScroll() {
	if !f.AutoScroll {
		return
	}
	rect := f.ContentRect(false)
	original := f.scrollOffset
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
	}
	if f.multiLine && rect.Height > 0 {
		if f.selectionStart == f.selectionAnchor {
			top := f.FromSelectionIndex(f.selectionEnd).Y
			if top < rect.Y {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Y - f.FromSelectionIndex(f.selectionEnd).Y
			} else {
				if top+f.lineHeightAt(top) >= rect.Bottom() {
					f.scrollOffset.Y = 0
					top = f.FromSelectionIndex(f.selectionEnd).Y
					f.scrollOffset.Y = rect.Bottom() - (top + f.lineHeightAt(top))
				}
			}
		} else {
			top := f.FromSelectionIndex(f.selectionStart).Y
			if top < rect.Y {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Y - f.FromSelectionIndex(f.selectionStart).Y
			} else {
				if top+f.lineHeightAt(top) >= rect.Bottom() {
					f.scrollOffset.Y = 0
					top = f.FromSelectionIndex(f.selectionEnd).Y
					f.scrollOffset.Y = rect.Bottom() - (top + f.lineHeightAt(top))
				}
			}
		}
		save := f.scrollOffset.Y
		f.scrollOffset.Y = 0
		top := f.FromSelectionIndex(len(f.runes)).Y
		minimum := rect.Bottom() - (top + f.lineHeightAt(top))
		if minimum > 0 {
			minimum = 0
		}
		top = f.FromSelectionIndex(0).Y
		maximum := rect.Y - (top + f.lineHeightAt(top))
		if maximum < 0 {
			maximum = 0
		}
		if save < minimum {
			save = minimum
		} else if save > maximum {
			save = maximum
		}
		f.scrollOffset.Y = save
	}
	if original != f.scrollOffset {
		f.MarkForRedraw()
	}
}

func (f *Field) textLeft(text *Text, bounds Rect) float32 {
	return f.textLeftForWidth(text.Width(), bounds)
}

func (f *Field) textLeftForWidth(width float32, bounds Rect) float32 {
	left := bounds.X
	switch f.HAlign {
	case align.Middle:
		left += (bounds.Width - width) / 2
	case align.End:
		left += bounds.Width - width - 1 // Inset since we leave space for the cursor
	default:
		left++ // Inset since we leave space for the cursor
	}
	return left
}

// ToSelectionIndex returns the rune index for the coordinates.
func (f *Field) ToSelectionIndex(where Point) int {
	if len(f.runes) == 0 {
		return 0
	}
	f.prepareLinesForCurrentWidth()
	lineIndex, start := f.lineIndexForY(where.Y)
	line := f.lines[lineIndex]
	return start + line.RuneIndexForPosition(where.X-(f.textLeft(line, f.ContentRect(false))+f.scrollOffset.X))
}

// FromSelectionIndex returns a location in local coordinates for the specified rune index.
func (f *Field) FromSelectionIndex(index int) Point {
	f.prepareLinesForCurrentWidth()
	index = max(min(index, len(f.runes)), 0)
	rect := f.ContentRect(false)
	y := rect.Y + f.scrollOffset.Y
	start := 0
	var lastHeight float32
	for i, line := range f.lines {
		length := len(line.Runes())
		if f.endsWithLineFeed[i] == hardLineEnding {
			length++
		}
		if !f.multiLine || index < start+length {
			return Point{X: f.textLeft(line, rect) + line.PositionForRuneIndex(index-start) + f.scrollOffset.X, Y: y}
		}
		lastHeight = max(line.Height(), f.Font.LineHeight())
		y += lastHeight
		start += length
	}
	return Point{X: f.textLeftForWidth(0, rect) + f.scrollOffset.X, Y: y - lastHeight}
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
	if length > 0 && f.isWordPart(start) {
		for start > 0 && f.isWordPart(start-1) {
			start--
		}
		for end < length && f.isWordPart(end) {
			end++
		}
	}
	return start, end
}

func (f *Field) isWordPart(index int) bool {
	r := f.runes[index]
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (f *Field) findPrevLineBreak(pos int) int {
	if pos >= len(f.runes) {
		pos = len(f.runes) - 1
	} else {
		pos--
	}
	_, start := f.lineIndexForPos(pos)
	return max(start-1, 0)
}

func (f *Field) findNextLineBreak(pos int) int {
	if pos < 0 {
		pos = 0
	} else {
		pos++
	}
	index, start := f.lineIndexForPos(pos)
	start += len(f.lines[index].runes)
	if f.multiLine && f.endsWithLineFeed[index] != hardLineEnding {
		start--
	}
	return min(start, len(f.runes))
}

func (f *Field) lineIndexForPos(pos int) (index, startPos int) {
	if pos < 0 {
		return 0, 0
	}
	f.prepareLinesForCurrentWidth()
	start := 0
	length := 0
	for i, line := range f.lines {
		length = len(line.Runes())
		if f.endsWithLineFeed[i] == hardLineEnding {
			length++
		}
		if pos < start+length {
			return i, start
		}
		start += length
	}
	return max(len(f.lines)-1, 0), start - length
}

func (f *Field) lineIndexForY(y float32) (index, startPos int) {
	y -= f.ContentRect(false).Y
	if y < f.scrollOffset.Y {
		return 0, 0
	}
	f.prepareLinesForCurrentWidth()
	offsetY := f.scrollOffset.Y
	start := 0
	length := 0
	for i, line := range f.lines {
		lineHeight := max(line.Height(), f.Font.LineHeight())
		if y >= offsetY && y <= offsetY+lineHeight {
			return i, start
		}
		offsetY += lineHeight
		length = len(line.Runes())
		if f.endsWithLineFeed[i] == hardLineEnding {
			length++
		}
		start += length
	}
	return max(len(f.lines)-1, 0), start - length
}

// GetFieldState returns the current field state, usually used for undo.
func (f *Field) GetFieldState() *FieldState {
	runes := make([]rune, len(f.runes))
	copy(runes, f.runes)
	return &FieldState{
		Text:            string(runes),
		SelectionStart:  f.selectionStart,
		SelectionEnd:    f.selectionEnd,
		SelectionAnchor: f.selectionAnchor,
	}
}

// ApplyFieldState sets the underlying field state to match the input and without triggering calls to the modification
// callback.
func (f *Field) ApplyFieldState(state *FieldState) {
	runes := f.sanitize([]rune(state.Text))
	if !txt.RunesEqual(runes, f.runes) {
		f.runes = runes
		f.linesBuiltFor = -1
	}
	f.setSelection(state.SelectionStart, state.SelectionEnd, state.SelectionAnchor)
}
