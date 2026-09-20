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
	"math"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/pathop"
	"github.com/richardwilkes/unison/enums/role"
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
	EditableInk:      ThemeDeepBelowSurface,
	OnEditableInk:    ThemeOnDeepBelowSurface,
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
	ModifiedCallback func(before, after *FieldState)
	ValidateCallback func() bool
	runes            []rune
	lines            []*Text
	endsWithLineFeed []lineEndingType
	// axLineCache holds what an assistive technology is told about each laid-out line other than where it is on the
	// screen: the rune range it covers and the offset of every rune boundary within it. Those are what cost something
	// to work out, and they change only when the lines themselves are rebuilt, so they are worked out once and reused
	// until prepareLines hands back a different set of lines. See Field.axTextLines.
	axLineCache        []accessibility.Line
	Watermark          string
	linesBuiltWithFont FontDescriptor
	forceShowUntil     time.Time
	FieldTheme
	Panel
	undoID             int64
	selectionStart     int
	selectionEnd       int
	selectionAnchor    int
	scrollOffset       geom.Point
	linesBuiltFor      float32
	linesBuiltWithRune rune
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
	f.MouseUpCallback = f.DefaultMouseUp
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
		f.linesBuiltFor = -1
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
func (f *Field) DefaultSizes(hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	var insets geom.Insets
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
	lines, endsWithLineFeed := f.buildLines(width)
	// buildLines hands back the very slice it cached when nothing it depends on has changed, and a freshly allocated
	// one whenever it laid text out again — which is every change of the content, the width, the wrapping, the font
	// and the obscurement rune. So the identity of the slice is exactly the question "are these the same lines as last
	// time", and it answers it without having to be told about any of those changes one by one.
	//
	// The one case it cannot answer that way is a field holding nothing, where every layout produces the same nil
	// slice and the cache therefore survives changes of the font, the width, the wrapping and the obscurement rune.
	// That costs nothing, since what an empty field caches is the single fixed line {Advances: [0]} covering no runes
	// at all, and axTextLines works out that line's bounds and its height from the font on every call.
	if len(lines) != len(f.lines) || (len(lines) != 0 && &lines[0] != &f.lines[0]) {
		f.axLineCache = nil
	}
	f.lines, f.endsWithLineFeed = lines, endsWithLineFeed
	f.linesBuiltFor = width
	f.linesBuiltWithFont = f.Font.Descriptor()
	f.linesBuiltWithRune = f.ObscurementRune
}

func (f *Field) prepareLinesForCurrentWidth() {
	f.prepareLines(f.ContentRect(false).Width - 2)
}

func (f *Field) buildLines(wrapWidth float32) (lines []*Text, endsWithLineFeed []lineEndingType) {
	// ObscurementRune and Font are public fields that may be changed at any time, so they must participate in the
	// cache validity check; wrap changes are handled by SetWrap resetting linesBuiltFor.
	if wrapWidth == f.linesBuiltFor && f.linesBuiltFor >= 0 && f.ObscurementRune == f.linesBuiltWithRune &&
		f.Font.Descriptor() == f.linesBuiltWithFont {
		return f.lines, f.endsWithLineFeed
	}
	if len(f.runes) != 0 {
		lines = make([]*Text, 0)
		decoration := &TextDecoration{Font: f.Font}
		if f.multiLine {
			endsWithLineFeed = make([]lineEndingType, 0, 16)
			for line := range strings.SplitSeq(string(f.runes), "\n") {
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
	return lines, endsWithLineFeed
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
func (f *Field) DefaultDraw(canvas *Canvas, _ geom.Rect) {
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
	backgroundPaint := bg.Paint(canvas, rect, paintstyle.Fill)
	canvas.DrawRect(rect, backgroundPaint)
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
				OnBackgroundInk: &ColorFilteredInk{
					OriginalInk: ink,
					ColorFilter: Alpha30Filter(),
				},
			})
			text.Draw(canvas, geom.NewPoint(f.textLeft(text, rect), textTop+text.Baseline()))
		}
		if !hasSelectionRange && enabled && focused {
			if f.showCursor {
				rect.X = f.textLeftForWidth(0, rect) + f.scrollOffset.X - 0.5
				rect.Width = 1
				rect.Height = f.Font.LineHeight()
				cursorPaint := fg.Paint(canvas, rect, paintstyle.Fill)
				canvas.DrawRect(rect, cursorPaint)
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
						Font:            f.Font,
						OnBackgroundInk: ink,
					})
					t.Draw(canvas, geom.NewPoint(left, textBaseLine))
					left += t.Width()
				}
				e := selEnd
				if end == selEnd && f.endsWithLineFeed[i] == hardLineEnding {
					e--
				}
				t := NewTextFromRunes(f.obscureIfNeeded(f.runes[selStart:e]), &TextDecoration{
					Font:            f.Font,
					OnBackgroundInk: f.OnSelectionInk,
				})
				right := left + t.Width()
				selRect := geom.NewRect(left, textTop, right-left, textHeight)
				selectionPaint := f.SelectionInk.Paint(canvas, selRect, paintstyle.Fill)
				canvas.DrawRect(selRect, selectionPaint)
				t.Draw(canvas, geom.NewPoint(left, textBaseLine))
				if selEnd < end {
					e = end
					if f.endsWithLineFeed[i] == hardLineEnding {
						e--
					}
					NewTextFromRunes(f.obscureIfNeeded(f.runes[selEnd:e]), &TextDecoration{
						Font:            f.Font,
						OnBackgroundInk: ink,
					}).Draw(canvas, geom.NewPoint(right, textBaseLine))
				}
			} else {
				line.AdjustDecorations(func(decoration *TextDecoration) { decoration.OnBackgroundInk = ink })
				line.Draw(canvas, geom.NewPoint(textLeft+f.scrollOffset.X, textBaseLine))
			}
			if !hasSelectionRange && enabled && focused && f.selectionEnd >= start && (f.selectionEnd < end ||
				(i == len(f.lines)-1 && f.selectionEnd <= end)) {
				if f.showCursor {
					t := NewTextFromRunes(f.obscureIfNeeded(f.runes[start:f.selectionEnd]),
						&TextDecoration{Font: f.Font})
					cursorPaint := fg.Paint(canvas, rect, paintstyle.Fill)
					canvas.DrawRect(geom.NewRect(textLeft+t.Width()+f.scrollOffset.X-0.5, textTop, 1, textHeight),
						cursorPaint)
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
	// The pending flag must be cleared even when the field is no longer in a valid window, since otherwise a blink
	// task that fires while the field is detached would leave it set and prevent the caret from ever blinking again
	// after the field is re-attached.
	f.pending = false
	window := f.Window()
	if window != nil && window.IsValid() {
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
func (f *Field) DefaultMouseDown(where geom.Point, button, clickCount int, mods mod.Modifiers) bool {
	f.undoID = NextUndoID()
	wasFocused := f.Focused()
	f.RequestFocus()
	if button == ButtonRight && clickCount == 1 {
		// Claim the click so that the mouse up is delivered to this field, where the context menu will be shown. The
		// menu cannot be popped up here, since it would swallow the mouse up event and leave the window convinced the
		// right button was still down, causing subsequent mouse moves to be treated as drags.
		return true
	}
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
					SafeCall(func() { selectAll = f.InitialClickSelectsAll(f) })
				}
			}
			if selectAll {
				f.setSelection(0, len(f.runes), f.ToSelectionIndex(where))
			} else {
				pos := f.ToSelectionIndex(where)
				if mods.ShiftDown() {
					// The anchor is the gesture's fixed end, so a shift-click extends from the existing anchor and
					// leaves it where it is; a following drag or shift-click then continues from that same fixed end.
					// Only an unshifted click moves the anchor to the click position.
					f.setSelectionFromAnchor(f.selectionAnchor, pos)
				} else {
					f.setSelection(pos, pos, pos)
				}
			}
		}
		return true
	}
	return false
}

// DefaultMouseUp provides the default mouse up handling.
func (f *Field) DefaultMouseUp(where geom.Point, button int, _ mod.Modifiers) bool {
	if button == ButtonRight {
		if where.In(f.ContentRect(true)) {
			f.ShowContextMenu(where)
		}
		return true
	}
	return false
}

// ShowContextMenu displays the context menu for the field at the specified position, which should be in local
// coordinates. Only the actions that can currently be performed (Cut, Copy, Paste, Select All) are included; if none
// of them can be performed, no menu is shown.
func (f *Field) ShowContextMenu(where geom.Point) {
	fac := DefaultMenuFactory()
	cm := fac.NewMenu(PopupMenuTemporaryBaseID|ContextMenuIDFlag, "", nil)
	cm.InsertItem(-1, CutAction().NewContextMenuItemFromAction(fac))
	cm.InsertItem(-1, CopyAction().NewContextMenuItemFromAction(fac))
	cm.InsertItem(-1, PasteAction().NewContextMenuItemFromAction(fac))
	cm.InsertItem(-1, SelectAllAction().NewContextMenuItemFromAction(fac))
	if cm.Count() > 0 {
		where = f.PointToRoot(where)
		cm.Popup(geom.NewRect(where.X, where.Y, 1, 1), 0)
	}
	cm.Dispose()
}

// DefaultMouseDrag provides the default mouse drag handling.
func (f *Field) DefaultMouseDrag(where geom.Point, button int, _ mod.Modifiers) bool {
	if button != ButtonLeft {
		return true
	}
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
func (f *Field) DefaultUpdateCursor(_ geom.Point) *Cursor {
	if f.Enabled() {
		return TextCursor()
	}
	return ArrowCursor()
}

// DefaultKeyDown provides the default key down handling.
func (f *Field) DefaultKeyDown(keyCode KeyCode, mods mod.Modifiers, _repeat bool) bool {
	if wnd := f.Window(); wnd != nil {
		wnd.HideCursorUntilMouseMoves()
	}
	if mods.OSMenuCommandDown() {
		switch keyCode {
		case KeyRight:
			f.handleEnd(f.multiLine, mods.ShiftDown())
		case KeyDown:
			f.handleEnd(false, mods.ShiftDown())
		case KeyLeft:
			f.handleHome(f.multiLine, mods.ShiftDown())
		case KeyUp:
			f.handleHome(false, mods.ShiftDown())
		default:
			// Handle cut/copy/paste/select all commands directly in case no menu is present
			if mods&mod.NonSticky == mod.OSMenuCommand() {
				switch keyCode {
				case KeyA:
					if f.CanSelectAll() {
						f.SelectAll()
						return true
					}
				case KeyX:
					if f.CanCut() {
						f.Cut()
						return true
					}
				case KeyC:
					if f.CanCopy() {
						f.Copy()
						return true
					}
				case KeyV:
					if f.CanPaste() {
						f.Paste()
						return true
					}
				}
			}
			return false
		}
		// The key was acted upon, so report it as handled to stop ancestors (e.g. a containing Table, which treats
		// bare arrow keys as navigation) from also processing it.
		return true
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
		f.handleArrowLeft(mods.ShiftDown(), mods.OptionDown())
	case KeyRight:
		f.handleArrowRight(mods.ShiftDown(), mods.OptionDown())
	case KeyEnd:
		f.handleEnd(f.multiLine, mods.ShiftDown())
	case KeyPageDown:
		if !f.multiLine {
			return false
		}
		f.handleEnd(false, mods.ShiftDown())
	case KeyHome:
		f.handleHome(f.multiLine, mods.ShiftDown())
	case KeyPageUp:
		if !f.multiLine {
			return false
		}
		f.handleHome(false, mods.ShiftDown())
	case KeyDown:
		if f.multiLine {
			f.handleArrowDown(mods.ShiftDown(), mods.OptionDown())
		} else {
			f.handleEnd(false, mods.ShiftDown())
		}
	case KeyUp:
		if f.multiLine {
			f.handleArrowUp(mods.ShiftDown(), mods.OptionDown())
		} else {
			f.handleHome(false, mods.ShiftDown())
		}
	case KeyTab:
		return false
	case KeyReturn, KeyNumPadEnter:
		f.undoID = NextUndoID()
		if !f.multiLine {
			return false
		}
		f.DefaultRuneTyped('\n')
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
		_, start := f.lineIndexForPos(f.selectionStart)
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
				f.setSelectionFromAnchor(anchor, pos)
			} else {
				pt := f.FromSelectionIndex(f.selectionStart)
				pt.Y--
				pos := f.ToSelectionIndex(pt)
				if byWord {
					start, _ := f.findWordAt(f.scanLeftToWordPart(pos))
					pos = min(start, pos)
				}
				f.setSelectionFromAnchor(anchor, pos)
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
				f.setSelectionFromAnchor(anchor, pos)
			} else {
				pt := f.FromSelectionIndex(f.selectionEnd)
				pt.Y += 1 + f.lineHeightAt(pt.Y)
				pos := f.ToSelectionIndex(pt)
				if byWord {
					_, end := f.findWordAt(f.scanRightToWordPart(pos))
					pos = max(end, pos)
				}
				f.setSelectionFromAnchor(anchor, pos)
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
		ClipboardSetText(f.SelectedText())
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
		ClipboardSetText(f.SelectedText())
	}
}

// CanPaste returns true if the clipboard has content that can be pasted into the field.
func (f *Field) CanPaste() bool {
	return ClipboardGetText() != ""
}

// Paste any text on the clipboard into the field.
func (f *Field) Paste() {
	text := ClipboardGetText()
	if text != "" {
		f.replaceRunes(f.selectionStart, f.selectionEnd, text)
	} else if f.HasSelectionRange() {
		f.Delete()
	}
}

// replaceRunes replaces the runes from start up to, but not including, end with text, leaving the caret just past what
// was inserted. The indexes are rune indexes and are constrained to the content, and the text is sanitized exactly as
// typed or pasted text is, so anything the field does not accept — a line feed in a single-line field — is dropped.
// This is what a paste does, and it is also how an assistive technology replaces a range of text.
func (f *Field) replaceRunes(start, end int, text string) {
	length := len(f.runes)
	start = min(max(start, 0), length)
	end = min(max(end, start), length)
	f.undoID = NextUndoID()
	before := f.GetFieldState()
	runes := f.sanitize([]rune(text))
	if end > start {
		f.runes = append(f.runes[:start], f.runes[end:]...)
	}
	f.runes = append(f.runes[:start], append(runes, f.runes[start:]...)...)
	f.linesBuiltFor = -1
	f.SetSelectionTo(start + len(runes))
	f.notifyOfModification(before, f.GetFieldState())
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
	if !slices.Equal(runes, f.runes) {
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
		SafeCall(func() { f.ModifiedCallback(before, after) })
	}
	f.Validate()
}

// Validate forces field content validation to be run.
func (f *Field) Validate() {
	invalid := false
	if f.ValidateCallback != nil {
		SafeCall(func() { invalid = !f.ValidateCallback() })
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

// setSelectionFromAnchor sets the selection to the range spanned by the gesture's fixed end (the anchor) and its moving
// end (pos), in whichever order they fall. Passing the two to setSelection directly would collapse the selection to an
// empty caret whenever the moving end crosses the anchor, since setSelection treats an end below the start as empty.
func (f *Field) setSelectionFromAnchor(anchor, pos int) {
	if pos < anchor {
		f.setSelection(pos, anchor, anchor)
	} else {
		f.setSelection(anchor, pos, anchor)
	}
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
	f.ScrollRectIntoView(geom.NewRect(pt.X-1, pt.Y, 3, f.lineHeightAt(pt.Y)))
}

// ScrollOffset returns the current autoscroll offset.
func (f *Field) ScrollOffset() geom.Point {
	return f.scrollOffset
}

// SetScrollOffset sets the autoscroll offset to the specified value.
func (f *Field) SetScrollOffset(offset geom.Point) {
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
			} else if top+f.lineHeightAt(top) >= rect.Bottom() {
				f.scrollOffset.Y = 0
				top = f.FromSelectionIndex(f.selectionEnd).Y
				f.scrollOffset.Y = rect.Bottom() - (top + f.lineHeightAt(top))
			}
		} else {
			top := f.FromSelectionIndex(f.selectionStart).Y
			if top < rect.Y {
				f.scrollOffset.Y = 0
				f.scrollOffset.Y = rect.Y - f.FromSelectionIndex(f.selectionStart).Y
			} else if top+f.lineHeightAt(top) >= rect.Bottom() {
				f.scrollOffset.Y = 0
				top = f.FromSelectionIndex(f.selectionStart).Y
				f.scrollOffset.Y = rect.Bottom() - (top + f.lineHeightAt(top))
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

func (f *Field) textLeft(text *Text, bounds geom.Rect) float32 {
	return f.textLeftForWidth(text.Width(), bounds)
}

func (f *Field) textLeftForWidth(width float32, bounds geom.Rect) float32 {
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
func (f *Field) ToSelectionIndex(where geom.Point) int {
	if len(f.runes) == 0 {
		return 0
	}
	f.prepareLinesForCurrentWidth()
	lineIndex, start := f.lineIndexForY(where.Y)
	line := f.lines[lineIndex]
	return start + line.RuneIndexForPosition(where.X-(f.textLeft(line, f.ContentRect(false))+f.scrollOffset.X))
}

// FromSelectionIndex returns a location in local coordinates for the specified rune index.
func (f *Field) FromSelectionIndex(index int) geom.Point {
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
		if index < start+length || i == len(f.lines)-1 {
			return geom.NewPoint(f.textLeft(line, rect)+line.PositionForRuneIndex(index-start)+f.scrollOffset.X, y)
		}
		lastHeight = max(line.Height(), f.Font.LineHeight())
		y += lastHeight
		start += length
	}
	return geom.NewPoint(f.textLeftForWidth(0, rect)+f.scrollOffset.X, y-lastHeight)
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

func (f *Field) findNextLineBreak(pos int) int {
	if pos < 0 {
		pos = 0
	} else {
		pos++
	}
	index, start := f.lineIndexForPos(pos)
	if index >= len(f.lines) {
		return len(f.runes)
	}
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
	if !slices.Equal(runes, f.runes) {
		f.runes = runes
		f.linesBuiltFor = -1
		f.MarkForRedraw()
	}
	f.setSelection(state.SelectionStart, state.SelectionEnd, state.SelectionAnchor)
}

// ProvideAccessibility describes the field to assistive technologies. A field that accepts line feeds is a text area
// rather than a text field, and one that obscures what it shows is a password: neither its content nor its caret is
// reported, since the run of bullets it is drawn as would say as much about what was typed as the text itself.
func (f *Field) ProvideAccessibility(b *AccessibilityBuilder) {
	node := b.Node()
	if node.Role == role.Auto {
		if f.multiLine {
			node.Role = role.TextArea
		} else {
			node.Role = role.TextField
		}
	}
	node.Placeholder = f.Watermark
	node.Invalid = f.invalid
	node.Protected = f.ObscurementRune != 0
	// Pressing a field is not activating it; the default behavior would synthesize a click at the center of the field,
	// which would do no more than drop the caret there.
	node.Actions = node.Actions.Without(accessibility.Press).
		With(accessibility.SetValue, accessibility.SetTextSelection, accessibility.ReplaceText,
			accessibility.ShowContextMenu)
	if node.Protected {
		return
	}
	node.Actions = node.Actions.With(accessibility.ScrollRangeIntoView)
	node.Value = f.Text()
	// Every line is reported, whether or not the person is working in this field: a screen reader reads a field it is
	// merely passing over by line and by word, and its review cursor asks where each character was drawn. What that
	// costs is one rectangle per line, since the rest is cached beside the wrapped lines it was measured from; see
	// axTextLines.
	lines := f.axTextLines()
	info := &accessibility.TextInfo{
		Text:     node.Value,
		Lines:    lines,
		SelStart: f.selectionStart,
		SelEnd:   f.selectionEnd,
		Caret:    f.axCaret(),
		// A field lays its content out over more than one line when it accepts line feeds, and also when it is a
		// single-line field that wraps: buildLines breaks the one line such a field holds to the field's width, so it
		// can show any number of them.
		Multiline: f.multiLine || len(lines) > 1,
	}
	if len(f.runes) != 0 {
		// A field draws the whole of its content in one font, so there is one run covering all of it. It is reported
		// rather than left out because a screen reader that asks what the text looks like — the font a person set out
		// to read it in, whether it is fixed-pitch — has nowhere else to learn it.
		run := axTextRunStyle(&TextDecoration{Font: f.Font})
		run.End = len(f.runes)
		info.Runs = []accessibility.TextRun{run}
	}
	node.Text = info
}

// axCaret returns where the caret is, which is the end of the selection that moves when the selection is extended: the
// end a further shift+Right would push along, and the start after a backward selection made with shift+Left or
// shift+Home. The other end is the anchor the selection was made from. Both ends are the same place when nothing is
// selected, and a selection made from somewhere in the middle — a double-click on a word — reports its end, which is
// where the next shift+Right would take it from.
func (f *Field) axCaret() int {
	if f.selectionAnchor == f.selectionEnd {
		return f.selectionStart
	}
	return f.selectionEnd
}

// axTextLines returns where each of the field's laid-out lines sits, in the field's own coordinates, along with the
// horizontal offset of every rune boundary on it. The lines are walked exactly as drawing and FromSelectionIndex walk
// them, so what an assistive technology is told a character's position is matches where the field actually drew it.
//
// The lines partition the content: a line ending in a line feed owns that line feed, so a caret at the end of a line is
// on that line rather than at the start of the next, and the last line always ends at the end of the content. Each
// line's Advances are measured from its own Bounds.X and hold one entry more than the line has runes, the last being
// the trailing edge of the line.
//
// Only the bounds are worked out here. Everything else is cached beside the wrapped lines it was measured from and
// dropped when those are rebuilt, so describing a field costs one rectangle per line rather than one measurement per
// rune — which is what makes it affordable to describe every field in a window on every snapshot rather than only the
// one the person is working in. The cached Advances are handed out as they are and must never be modified.
func (f *Field) axTextLines() []accessibility.Line {
	rect := f.ContentRect(false)
	f.prepareLinesForCurrentWidth()
	if f.axLineCache == nil {
		f.axLineCache = f.axBuildLineCache()
	}
	lines := slices.Clone(f.axLineCache)
	top := rect.Y + f.scrollOffset.Y
	if len(f.lines) == 0 {
		// There is nothing laid out, but the caret is still somewhere, so the empty line it sits on is described.
		lines[0].Bounds = geom.NewRect(f.textLeftForWidth(0, rect)+f.scrollOffset.X, top, 0, f.Font.LineHeight())
		return lines
	}
	for i, line := range f.lines {
		height := max(line.Height(), f.Font.LineHeight())
		lines[i].Bounds = geom.NewRect(f.textLeft(line, rect)+f.scrollOffset.X, top, line.Width(), height)
		top += height
	}
	return lines
}

// axBuildLineCache works out the rune range each laid-out line covers and the offset of every rune boundary within it.
// It is called once per set of lines; see Field.axTextLines, which fills in where each of them is.
func (f *Field) axBuildLineCache() []accessibility.Line {
	if len(f.lines) == 0 {
		return []accessibility.Line{{Advances: []float32{0}}}
	}
	total := len(f.runes)
	lines := make([]accessibility.Line, 0, len(f.lines))
	start := 0
	for i, line := range f.lines {
		end := start + len(line.Runes())
		if f.endsWithLineFeed[i] == hardLineEnding {
			// The final line is marked as ending with a line feed whether or not the content does, so the length of the
			// content is the bound.
			end = min(end+1, total)
		}
		// The boundaries are accumulated rather than each one being summed from the start of the line. Asking
		// Text.PositionForRuneIndex for every boundary in turn costs the square of the line's rune count, since each
		// answer sums the widths up to its index, so one long line — a pasted URL in a single-line field or a long
		// unwrapped run in a text area — would cost the square of its length each time the lines were rebuilt. A
		// boundary past the last width clamps to the full width of the line, exactly as PositionForRuneIndex does: a
		// line that owns the line feed ending it has one boundary more than it has runes laid out.
		count := end - start
		advances := make([]float32, count+1)
		position := float32(0)
		for j := range count {
			if j < len(line.widths) {
				position += line.widths[j]
			}
			advances[j+1] = position
		}
		lines = append(lines, accessibility.Line{Advances: advances, Start: start, End: end})
		start = end
	}
	return lines
}

// axScrollRangeIntoView brings a range of the field's content into view, which is what an assistive technology asks
// for when it moves its own reading cursor through text the person cannot see.
//
// A field that scrolls its own content has to be scrolled first: nothing outside it can reveal a word that the field
// itself is holding out of sight. The edge arithmetic autoScroll uses, shifted rather than assigned and without its
// clamp, puts the range within the content rect — horizontally always, vertically only for a field that accepts line
// feeds — and the rectangle is shifted along with the content so that whatever the field could not reveal by itself is
// then asked of the ancestors that can scroll.
func (f *Field) axScrollRangeIntoView(start, end int) {
	lines := f.axTextLines()
	rect := axRangeRect(lines, min(start, end), max(start, end))
	if rect.Empty() {
		return
	}
	if f.AutoScroll {
		content := f.ContentRect(false)
		original := f.scrollOffset
		if content.Width > 0 {
			if rect.X < content.X {
				f.scrollOffset.X += content.X - rect.X
			} else if rect.Right() > content.Right() {
				f.scrollOffset.X -= min(rect.Right()-content.Right(), rect.X-content.X)
			}
		}
		if f.multiLine && content.Height > 0 {
			if rect.Y < content.Y {
				f.scrollOffset.Y += content.Y - rect.Y
			} else if rect.Bottom() > content.Bottom() {
				f.scrollOffset.Y -= min(rect.Bottom()-content.Bottom(), rect.Y-content.Y)
			}
		}
		if original != f.scrollOffset {
			f.MarkForRedraw()
			rect.Point = rect.Point.Add(f.scrollOffset.Sub(original))
		}
	}
	f.ScrollRectIntoView(rect)
}

// PerformAccessibilityAction carries out a request from an assistive technology. The field's value may be replaced
// outright, a range of it may be replaced in place — which participates in undo exactly as a paste does — the caret or
// selection may be moved, a range of the content may be brought into view, and the field's contextual menu may be
// shown, which takes the focus first because the menu is built out of the commands that act on whatever holds it.
// Focusing the field is otherwise left to the default behavior. A field that obscures what it shows brings no range
// into view, since it publishes neither that action nor the text a range would address.
func (f *Field) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	switch req.Action {
	case accessibility.SetValue:
		// Replacing the whole value is an edit of its own and must not be folded into whatever edit came before it, so
		// the undo id is moved on first, exactly as the paste and replace paths move it on.
		f.undoID = NextUndoID()
		f.SetText(req.Value)
		return true
	case accessibility.SetTextSelection:
		f.SetSelection(req.Start, req.End)
		return true
	case accessibility.ReplaceText:
		f.replaceRunes(req.Start, req.End, req.Value)
		return true
	case accessibility.ScrollRangeIntoView:
		if f.ObscurementRune != 0 {
			// A protected field reports neither its text nor its caret, so it withholds this action as well — see
			// ProvideAccessibility. Carrying it out anyway would leave an adapter that trusts what the node advertises
			// told the field cannot do something it quietly does, and would scroll to a range of a string that was
			// never published.
			return false
		}
		f.axScrollRangeIntoView(req.Start, req.End)
		return true
	case accessibility.ShowContextMenu:
		if !axMayPopupMenu(f.AsPanel()) {
			// The menu would be built in whatever window is active rather than in this one. See axMayPopupMenu.
			return false
		}
		// The menu is built from the cut, copy, paste and select-all actions, every one of which is routed to whatever
		// holds the focus rather than to this field, so asking an unfocused field for its menu would describe, and then
		// operate on, whatever else the focus is in. DefaultMouseDown takes the focus before showing the menu for a
		// right-click, and the same has to happen here.
		f.RequestFocus()
		// A right-click would have put the menu under the pointer; there is no pointer here, so it goes where the
		// person's attention is, which is the caret — the end of the selection that moves, which after a backward
		// selection made with shift+Left or shift+Home is its start rather than its end. See axCaret.
		f.ShowContextMenu(f.FromSelectionIndex(f.axCaret()))
		return true
	default:
		return false
	}
}

// axMenuOpeningActions reports which of the field's actions would open a menu, so that a field in a window none of its
// menus would land in is published without them rather than advertising what PerformAccessibilityAction, and the action
// callback NewComboField installs, would then refuse. See axMenuActions.
//
// Expand belongs to a combo box, which is a field the dropdown NewComboField installs describes as one; a plain field
// never advertises it, so naming it here costs such a field nothing. It is not named while the choices are already
// showing, which is exactly what that callback does with the request: expanding a combo box that is already open is
// accepted as already done, with no menu opened anywhere. Collapse is never named either, since taking a menu down acts
// on the menu that is showing rather than on whatever window is active.
func (f *Field) axMenuOpeningActions(node *accessibility.Node) accessibility.ActionSet {
	actions := accessibility.ActionSet(0).With(accessibility.ShowContextMenu)
	if !node.Expanded {
		actions = actions.With(accessibility.Expand)
	}
	return actions
}

// InstallAccessoryPanel sets a panel into the field, attached to the right end. The editable text area will shrink by
// the preferred width of the accessory panel.
func (f *Field) InstallAccessoryPanel(panel Paneler) {
	p := panel.AsPanel()
	UninstallFocusBorders(f, f)
	_, prefSize, _ := p.Sizes(geom.Size{})
	adjustBorder := func(b Border) Border {
		return NewCompoundBorder(b, NewEmptyBorder(geom.Insets{Right: prefSize.Width}))
	}
	focusedBorder := adjustBorder(NewDefaultFieldBorder(true))
	unfocusedBorder := adjustBorder(NewDefaultFieldBorder(false))
	InstallFocusBorders(f, f, focusedBorder, unfocusedBorder)
	f.AddChild(p)
	f.SetLayout(&fieldAccessoryLayout{})
}

type fieldAccessoryLayout struct{}

func (l *fieldAccessoryLayout) LayoutSizes(p *Panel, hint geom.Size) (minSize, prefSize, maxSize geom.Size) {
	return p.Sizer()(hint)
}

func (l *fieldAccessoryLayout) PerformLayout(target *Panel) {
	if children := target.Children(); len(children) > 0 {
		_, prefSize, _ := children[0].Sizes(geom.Size{})
		r := target.ContentRect(false)
		r.X = r.Right()
		r.Width = prefSize.Width
		r.Y += (r.Height - prefSize.Height) / 2
		r.Height = prefSize.Height
		children[0].SetFrameRect(r)
	}
}
