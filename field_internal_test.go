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
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestFieldAutoScrollBottomOverflowTracksSelectionStart verifies that when a selection is being extended backward
// (anchor at the end) and its start lies below the visible area, autoScroll brings the start of the selection into
// view. A copy-paste bug used to make this case scroll the end of the selection into view instead.
func TestFieldAutoScrollBottomOverflowTracksSelectionStart(t *testing.T) {
	c := check.New(t)
	f := NewMultiLineField()
	lines := make([]string, 10)
	for i := range lines {
		lines[i] = "line" // Every line is 5 runes long, counting its newline, so line i starts at rune i*5.
	}
	f.SetText(strings.Join(lines, "\n"))
	var insets geom.Insets
	if b := f.Border(); b != nil {
		insets = b.Insets()
	}
	lineHeight := f.Font.LineHeight()
	f.SetFrameRect(geom.NewRect(0, 0, 200+insets.Width(), 2*lineHeight+insets.Height()))

	// Scroll back to the top with the caret at the start.
	f.SetSelectionToStart()
	c.Equal(float32(0), f.ScrollOffset().Y)

	// Extend the selection backward from an anchor at the end of line 8 to the start of line 5. Both lines are below
	// the two visible lines, and since the anchor is the selection end, autoScroll must scroll the selection start
	// (line 5) into view, not the selection end (line 8).
	f.setSelection(5*5, 8*5, 8*5)
	rect := f.ContentRect(false)
	pt := f.FromSelectionIndex(f.selectionStart)
	bottomGap := rect.Bottom() - (pt.Y + f.lineHeightAt(pt.Y))
	if bottomGap < 0 {
		bottomGap = -bottomGap
	}
	c.True(bottomGap < 1, "the start of the selection should be bottom-aligned, but is off by %v", bottomGap)
	c.True(f.FromSelectionIndex(f.selectionEnd).Y > rect.Bottom(),
		"the end of the selection should remain below the visible area")
}

// wrapTestField returns a single-line field with wrap enabled whose text soft-wraps into exactly two lines, plus the
// rune index of the soft-wrap boundary (the first rune of the second visual line).
func wrapTestField(c check.Checker) (f *Field, boundary int) {
	f = NewField()
	f.SetWrap(true)
	f.SetText("aaaa aaaa")
	var insets geom.Insets
	if b := f.Border(); b != nil {
		insets = b.Insets()
	}
	// Choose a wrap width that fits the first word plus its trailing space, but not the whole text, so the field
	// breaks into two soft-wrapped lines. prepareLines uses ContentRect(false).Width - 2 as the wrap width.
	full := NewText("aaaa aaaa", &TextDecoration{Font: f.Font}).Width()
	f.SetFrameRect(geom.NewRect(0, 0, full*0.7+2+insets.Width(), 4*f.Font.LineHeight()+insets.Height()))
	f.prepareLinesForCurrentWidth()
	c.Equal(2, len(f.lines), "the text should soft-wrap into exactly two lines")
	return f, len(f.lines[0].Runes())
}

// TestFieldWrapSingleLineFromSelectionIndex verifies that FromSelectionIndex maps rune indexes past the first visual
// line of a wrap-enabled single-line field onto the correct wrapped line. It used to short-circuit on !multiLine and
// place every index on line 0, breaking the caret position, ScrollSelectionIntoView, and autoScroll.
func TestFieldWrapSingleLineFromSelectionIndex(t *testing.T) {
	c := check.New(t)
	f, boundary := wrapTestField(c)
	rect := f.ContentRect(false)
	line0Height := max(f.lines[0].Height(), f.Font.LineHeight())

	// An index on the first line stays on the first line.
	nearlyEqual(c, rect.Y, f.FromSelectionIndex(0).Y)

	// The soft-wrap boundary maps to the start of the second visual line, matching where DefaultDraw puts the caret.
	pt := f.FromSelectionIndex(boundary)
	nearlyEqual(c, rect.Y+line0Height, pt.Y)
	nearlyEqual(c, f.textLeft(f.lines[1], rect), pt.X)

	// The end of the text maps to the end of the second visual line, not the fallback zero-width position.
	pt = f.FromSelectionIndex(len(f.runes))
	nearlyEqual(c, rect.Y+line0Height, pt.Y)
	nearlyEqual(c, f.textLeft(f.lines[1], rect)+f.lines[1].Width(), pt.X)
}

// TestFieldWrapSingleLineDrawsSingleCaret verifies that a caret sitting on a soft-wrap boundary of a wrap-enabled
// single-line field is drawn exactly once, at the start of the second visual line. The cursor-draw condition used to be
// satisfied on both sides of the boundary, drawing a second caret at the end of the first line.
func TestFieldWrapSingleLineDrawsSingleCaret(t *testing.T) {
	c := check.New(t)
	swapRedrawSet(t)
	w := newRedrawTestWindow()
	f, boundary := wrapTestField(c)
	w.root.AddChild(f)
	w.focused = true
	w.focus = f.AsPanel()
	f.setSelection(boundary, boundary, boundary)
	f.showCursor = true
	f.pending = true            // Keep scheduleBlink from queuing a blink task in this headless test.
	f.EditableInk = Transparent // Keep the background fill out of the pixmap so only text and caret ink remain.

	frame := f.FrameRect()
	cnv, pix := newPixmapCanvas(int32(frame.Width)+1, int32(frame.Height)+1)
	f.DefaultDraw(cnv, f.ContentRect(true))

	rect := f.ContentRect(false)
	line0Height := max(f.lines[0].Height(), f.Font.LineHeight())
	line0Top := int32(rect.Y + 1)
	line0Bottom := int32(rect.Y + line0Height - 1)
	rowAt := func(y int32) []uint32 {
		return pix.Pix[int(y)*int(pix.RowPixels) : int(y)*int(pix.RowPixels)+int(pix.Width)]
	}

	// Nothing may be drawn to the right of the first line's glyphs: the phantom caret used to appear there, one space
	// width past the last glyph. The trailing space is excluded from the width so the scan covers the caret location.
	glyphEnd := f.textLeft(f.lines[0], rect) + NewTextFromRunes(f.runes[:boundary-1], &TextDecoration{Font: f.Font}).Width()
	for y := line0Top; y <= line0Bottom; y++ {
		row := rowAt(y)
		for x := int32(glyphEnd + 2); x < pix.Width; x++ {
			c.Equal(uint32(0), row[x]>>24, "no caret may be drawn on the first line past its glyphs (row %d, col %d)", y, x)
		}
	}

	// The caret must be drawn at the start of the second visual line.
	line1Top := line0Bottom + 2
	line1Bottom := int32(rect.Y + line0Height + max(f.lines[1].Height(), f.Font.LineHeight()) - 1)
	caretX := int32(f.textLeft(f.lines[1], rect))
	found := false
	for y := line1Top; y <= line1Bottom && !found; y++ {
		row := rowAt(y)
		for x := max(caretX-2, 0); x <= caretX+2 && x < pix.Width; x++ {
			if row[x]>>24 != 0 {
				found = true
				break
			}
		}
	}
	c.True(found, "the caret should be drawn at the start of the second visual line")
}

// TestFieldApplyFieldStateRedrawsWhenOnlyTextChanges verifies that ApplyFieldState marks the field for redraw when the
// text changes but the selection is identical, as happens when undoing a forward-delete: the deletion removes the rune
// at the caret without moving it, so undo restores different text with the same selection triple.
func TestFieldApplyFieldStateRedrawsWhenOnlyTextChanges(t *testing.T) {
	c := check.New(t)
	w := newRedrawTestWindow()
	swapRedrawSet(t)
	f := NewField()
	w.root.AddChild(f)
	f.SetText("ab") // Leaves the caret at the end: selection (2, 2), anchor 2.
	redrawSet = make(map[*Window]struct{})
	f.ApplyFieldState(&FieldState{Text: "abX", SelectionStart: 2, SelectionEnd: 2, SelectionAnchor: 2})
	c.Equal("abX", f.Text())
	_, pending := redrawSet[w]
	c.True(pending, "restoring text without changing the selection must still mark the field for redraw")
}

// TestFieldSetWrapInvalidatesLineCache verifies that toggling SetWrap rebuilds the cached lines even when the field's
// content width is unchanged. SetWrap used to only mark for layout/redraw without resetting linesBuiltFor, so a later
// prepareLines call at the same width silently reused lines built with the old wrap setting and the toggle had no
// visible effect.
func TestFieldSetWrapInvalidatesLineCache(t *testing.T) {
	c := check.New(t)
	f, _ := wrapTestField(c) // Wrap is enabled and the text soft-wraps into exactly two lines.
	f.SetWrap(false)
	f.prepareLinesForCurrentWidth()
	c.Equal(1, len(f.lines), "disabling wrap at the same width must rebuild the lines unwrapped")
	f.SetWrap(true)
	f.prepareLinesForCurrentWidth()
	c.Equal(2, len(f.lines), "re-enabling wrap at the same width must rebuild the soft-wrapped lines")
}

// TestFieldObscurementRuneChangeInvalidatesLineCache verifies that changing the public ObscurementRune field (e.g. a
// "show password" toggle) rebuilds the cached lines even when the field's content width is unchanged.
func TestFieldObscurementRuneChangeInvalidatesLineCache(t *testing.T) {
	c := check.New(t)
	f := NewField()
	f.SetText("secret")
	f.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	f.prepareLinesForCurrentWidth()
	c.Equal(1, len(f.lines))
	c.Equal("secret", f.lines[0].String())
	f.ObscurementRune = '•'
	f.prepareLinesForCurrentWidth()
	c.Equal("••••••", f.lines[0].String(), "setting ObscurementRune must rebuild the lines obscured")
	f.ObscurementRune = 0
	f.prepareLinesForCurrentWidth()
	c.Equal("secret", f.lines[0].String(), "clearing ObscurementRune must rebuild the lines unobscured")
}

// TestFieldFontChangeInvalidatesLineCache verifies that changing the public Font field rebuilds the cached lines even
// when the field's content width is unchanged.
func TestFieldFontChangeInvalidatesLineCache(t *testing.T) {
	c := check.New(t)
	f := NewField()
	f.SetText("hello")
	f.SetFrameRect(geom.NewRect(0, 0, 200, 80))
	f.prepareLinesForCurrentWidth()
	oldWidth := f.lines[0].Width()
	f.Font = f.Font.Face().Font(f.Font.Size() * 2)
	f.prepareLinesForCurrentWidth()
	c.True(f.lines[0].Width() > oldWidth,
		"doubling the font size must rebuild the lines with the new font (old width %v, new width %v)",
		oldWidth, f.lines[0].Width())
}
