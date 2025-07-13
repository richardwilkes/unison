// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"unicode"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
)

// Text holds data necessary to draw a string using font fallbacks where necessary.
type Text struct {
	text        string
	runes       []rune
	decorations []*TextDecoration
	widths      []float32
	extents     geom.Size
	baseline    float32
	emptyHeight float32
}

// NewText creates a new Text. Note that tabs and line endings are not considered.
func NewText(str string, decoration *TextDecoration) *Text {
	return NewTextFromRunes([]rune(str), decoration)
}

// NewTextFromRunes creates a new Text. Note that tabs and line endings are not considered. This is more efficient than
// NewText(), since the string doesn't have to be converted to runes first.
func NewTextFromRunes(runes []rune, decoration *TextDecoration) *Text {
	t := &Text{
		runes:       make([]rune, 0, len(runes)),
		decorations: make([]*TextDecoration, 0, len(runes)),
		widths:      make([]float32, 0, len(runes)),
		extents:     geom.Size{Width: -1},
		baseline:    decoration.Font.Baseline(),
		emptyHeight: decoration.Font.LineHeight() + xmath.Abs(decoration.BaselineOffset),
	}
	t.AddRunes(runes, decoration)
	return t
}

// NewTextLines creates a new list of Text, one for each logical line. Tabs are not considered, but the text is split on
// any line feeds found.
func NewTextLines(text string, decoration *TextDecoration) []*Text {
	split := strings.Split(text, "\n")
	lines := make([]*Text, 0, len(split))
	for _, line := range split {
		lines = append(lines, NewText(line, decoration))
	}
	return lines
}

// NewTextWrappedLines creates a new list of Text, potentially multiple for each logical line. Tabs are not considered,
// but the text is split on any line feeds found and then wrapped to the given width. See Text.BreakToWidth().
func NewTextWrappedLines(text string, decoration *TextDecoration, width float32) []*Text {
	var lines []*Text
	for _, line := range NewTextLines(text, decoration) {
		lines = append(lines, line.BreakToWidth(width)...)
	}
	return lines
}

// NewSmallCapsText creates a new Text object with the given text, but with lowercase letters replaced by small caps.
func NewSmallCapsText(text string, decoration *TextDecoration) *Text {
	smaller := decoration.Clone()
	font := smaller.Font
	smaller.Font = &DynamicFont{
		Resolver: func() FontDescriptor {
			fd := font.Descriptor()
			fd.Size *= 0.75
			return fd
		},
	}
	t := NewTextFromRunes(nil, decoration)
	isLower := false
	run := make([]rune, 0, 32)
	for _, r := range text {
		if r >= 'a' && r <= 'z' {
			if !isLower {
				isLower = true
				if len(run) > 0 {
					t.AddRunes(run, decoration)
					run = make([]rune, 0, 32)
				}
			}
			run = append(run, unicode.ToUpper(r))
		} else {
			if isLower {
				isLower = false
				if len(run) > 0 {
					t.AddRunes(run, smaller)
					run = make([]rune, 0, 32)
				}
			}
			run = append(run, r)
		}
	}
	if len(run) > 0 {
		if isLower {
			t.AddRunes(run, smaller)
		} else {
			t.AddRunes(run, decoration)
		}
	}
	return t
}

// Empty returns true if this doesn't hold any characters. May be called on a nil *Text.
func (t *Text) Empty() bool {
	return t == nil || len(t.runes) == 0
}

// Slice creates a new Text that is a slice of this Text. The indexes refer to rune positions.
func (t *Text) Slice(i, j int) *Text {
	if i < 0 {
		i = 0
	}
	if j > len(t.runes) {
		j = len(t.runes)
	}
	if i >= j {
		return &Text{
			baseline:    t.baseline,
			emptyHeight: t.emptyHeight,
		}
	}
	return &Text{
		runes:       t.runes[i:j],
		decorations: t.decorations[i:j],
		widths:      t.widths[i:j],
		extents:     geom.Size{Width: -1},
		baseline:    t.baseline,
		emptyHeight: t.emptyHeight,
	}
}

// Runes returns the runes comprising this Text. Do not modify this slice.
func (t *Text) Runes() []rune {
	return t.runes
}

// String returns the string representation of this Text.
func (t *Text) String() string {
	if t.Empty() {
		return ""
	}
	if t.text == "" && len(t.runes) != 0 {
		t.text = string(t.runes)
	}
	return t.text
}

// Extents returns the width and height.
func (t *Text) Extents() geom.Size {
	t.cache()
	return t.extents
}

// Width returns the width.
func (t *Text) Width() float32 {
	t.cache()
	return t.extents.Width
}

// Height returns the height.
func (t *Text) Height() float32 {
	t.cache()
	return t.extents.Height
}

// Baseline returns the baseline that will be used, which is based on the original font passed in at creation time.
func (t *Text) Baseline() float32 {
	return t.baseline
}

func (t *Text) cache() {
	if t.extents.Width < 0 {
		t.extents.Width = 0
		t.extents.Height = t.emptyHeight
		for i, d := range t.decorations {
			h := d.Font.LineHeight() + xmath.Abs(d.BaselineOffset)
			t.extents.Width += t.widths[i]
			if t.extents.Height < h {
				t.extents.Height = h
			}
		}
	}
}

// AddString adds a string with the given decoration to this Text.
func (t *Text) AddString(str string, decoration *TextDecoration) {
	t.AddRunes([]rune(str), decoration)
}

// AddRunes adds runes with the given decoration to this Text. This is more efficient than AddString(), since the string
// doesn't have to be converted to runes first.
func (t *Text) AddRunes(runes []rune, decoration *TextDecoration) {
	if len(runes) == 0 {
		return
	}
	t.text = ""
	t.extents.Width = -1
	start := len(t.decorations)
	if start != 0 && decoration.Equivalent(t.decorations[start-1]) {
		decoration = t.decorations[start-1]
	} else {
		decoration = decoration.Clone()
	}
	t.runes = append(t.runes, runes...)
	face := decoration.Font.Face()
	glyphs := decoration.Font.RunesToGlyphs(runes)
	t.widths = append(t.widths, decoration.Font.GlyphWidths(glyphs)...)
	for i, r := range runes {
		t.decorations = append(t.decorations, decoration)
		if glyphs[i] != 0 {
			continue
		}
		if altFace := face.FallbackForCharacter(r); altFace != nil {
			altDec := *decoration
			altDec.Font = altFace.Font(decoration.Font.Size())
			if start != 0 && altDec.Equivalent(t.decorations[start-1]) {
				t.decorations[len(t.decorations)-1] = t.decorations[start-1]
			} else {
				t.decorations[len(t.decorations)-1] = &altDec
			}
			t.widths[start+i] = altDec.Font.GlyphWidths([]uint16{altDec.Font.RuneToGlyph(r)})[0]
		}
	}
}

// AdjustDecorations calls adjuster for each decoration in no particular order. The returned map can be passed to
// RestoreDecorations() to restore the decorations to their original state.
func (t *Text) AdjustDecorations(adjuster func(decoration *TextDecoration)) map[*TextDecoration]TextDecoration {
	if t == nil {
		return nil
	}
	m := make(map[*TextDecoration]TextDecoration)
	for _, d := range t.decorations {
		m[d] = *d
	}
	for k := range m {
		adjuster(k)
	}
	return m
}

// RestoreDecorations restores the decorations to the state they were in when AdjustDecorations() was called.
func (t *Text) RestoreDecorations(m map[*TextDecoration]TextDecoration) {
	t.AdjustDecorations(func(decoration *TextDecoration) {
		if d, ok := m[decoration]; ok {
			*decoration = d
		}
	})
}

// Draw the Text at the given location. y is where the baseline of the text will be placed.
func (t *Text) Draw(canvas *Canvas, x, y float32) {
	if len(t.decorations) == 0 {
		return
	}
	start := 0
	current := t.decorations[0]
	nx := x
	for i, d := range t.decorations {
		if i != 0 && !current.Equivalent(d) {
			current.DrawText(canvas, string(t.runes[start:i]), x, y+current.BaselineOffset, nx-x)
			current = d
			x = nx
			start = i
		}
		nx += t.widths[i]
	}
	if start < len(t.decorations) {
		current.DrawText(canvas, string(t.runes[start:]), x, y+current.BaselineOffset, nx-x)
	}
}

// RuneIndexForPosition returns the rune index within the string for the specified x-coordinate, where 0 is the start of
// the string.
func (t *Text) RuneIndexForPosition(x float32) int {
	if x <= 0 || len(t.widths) == 0 {
		return 0
	}
	var nx float32
	for i, w := range t.widths {
		nx += w
		if x < nx {
			if x > nx-w/2 {
				return i + 1
			}
			return i
		}
	}
	return len(t.widths)
}

// PositionForRuneIndex returns the x-coordinate where the specified rune index starts. The returned coordinate assumes
// 0 is the start of the string. Note that this does not account for any embedded line endings nor tabs.
func (t *Text) PositionForRuneIndex(index int) float32 {
	if index <= 0 || len(t.widths) == 0 {
		return 0
	}
	var x float32
	for i, w := range t.widths {
		if i >= index {
			break
		}
		x += w
	}
	return x
}

// BreakToWidth breaks the given text into multiple lines that are <= width. Trailing whitespace is not considered for
// purposes of fitting within the given width. A minimum of one word will be placed on a line, even if that word is
// wider than the given width.
func (t *Text) BreakToWidth(width float32) []*Text {
	if t.Width() <= width {
		return []*Text{t}
	}
	var lines []*Text
	start := 0
	for start < len(t.runes) {
		i := start
		var w float32
		for i < len(t.runes) && w+t.widths[i] < width {
			w += t.widths[i]
			i++
		}
		if i == len(t.runes) {
			lines = append(lines, t.Slice(start, len(t.runes)))
			break
		}
		// Forward past any additional whitespace
		for i < len(t.runes) && unicode.IsSpace(t.runes[i]) {
			i++
		}
		// Backup to first break
		for i > start && !isWordBreak(t.runes[i-1]) {
			i--
		}
		if i == start {
			// Nothing found that fits, so take the first word and any trailing whitespace after it
			for i < len(t.runes) && !isWordBreak(t.runes[i]) {
				i++
			}
			if i < len(t.runes) && isWordBreak(t.runes[i]) {
				i++
			}
			for i < len(t.runes) && unicode.IsSpace(t.runes[i]) {
				i++
			}
		}
		lines = append(lines, t.Slice(start, i))
		start = i
	}
	return lines
}

func isWordBreak(ch rune) bool {
	return unicode.IsSpace(ch) || ch == '/' || ch == '\\'
}
