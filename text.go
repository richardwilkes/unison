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
	"strings"
	"unicode"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

// Text holds data necessary to draw a string using font fallbacks where necessary.
type Text struct {
	Runes       []rune
	Decorations []*TextDecoration
	widths      []float32
	extents     geom32.Size
	baseline    float32
	str         string
}

// TextDecoration holds the decorations that can be applied to text when drawn.
type TextDecoration struct {
	Font           Font
	Paint          *Paint
	BaselineOffset float32
	Underline      bool
	StrikeThrough  bool
}

// Equivalent returns true if this TextDecoration is equivalent to the other.
func (d *TextDecoration) Equivalent(other *TextDecoration) bool {
	if d == nil {
		return other == nil
	}
	if other == nil {
		return false
	}
	return d.Underline == other.Underline && d.StrikeThrough == other.StrikeThrough &&
		d.BaselineOffset == other.BaselineOffset && d.Paint.Equivalent(other.Paint) &&
		d.Font.Descriptor() == other.Font.Descriptor()
}

// NewText creates a new Text. Note that tabs and line endings are not considered.
func NewText(str string, decoration *TextDecoration) *Text {
	return NewTextFromRunes([]rune(str), decoration)
}

// NewTextFromRunes creates a new Text. Note that tabs and line endings are not considered. This is more efficient than
// NewText(), since the string doesn't have to be converted to runes first.
func NewTextFromRunes(runes []rune, decoration *TextDecoration) *Text {
	t := &Text{
		Runes:       make([]rune, 0, len(runes)),
		Decorations: make([]*TextDecoration, 0, len(runes)),
		widths:      make([]float32, 0, len(runes)),
		extents:     geom32.Size{Width: -1},
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

// Slice creates a new Text that is a slice of this Text. The indexes refer to rune positions.
func (t *Text) Slice(i, j int) *Text {
	if i < 0 {
		i = 0
	}
	if j > len(t.Runes) {
		j = len(t.Runes)
	}
	if i >= j {
		return &Text{}
	}
	return &Text{
		Runes:       t.Runes[i:j],
		Decorations: t.Decorations[i:j],
		widths:      t.widths[i:j],
		extents:     geom32.Size{Width: -1},
	}
}

// String returns the string representation of this Text.
func (t *Text) String() string {
	if t.str == "" && len(t.Runes) != 0 {
		t.str = string(t.Runes)
	}
	return t.str
}

// Extents returns the width and height.
func (t *Text) Extents() geom32.Size {
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

// Baseline returns the largest baseline found, after considering any baseline offset adjustments.
func (t *Text) Baseline() float32 {
	t.cache()
	return t.baseline
}

func (t *Text) cache() {
	if t.extents.Width < 0 {
		t.extents.Width = 0
		t.extents.Height = 0
		t.baseline = 0
		for i, d := range t.Decorations {
			h := d.Font.LineHeight() + mathf32.Abs(d.BaselineOffset)
			t.extents.Width += t.widths[i]
			if t.extents.Height < h {
				t.extents.Height = h
			}
			b := d.Font.Baseline() + d.BaselineOffset
			if t.baseline < b {
				t.baseline = b
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
	t.str = ""
	t.extents.Width = -1
	start := len(t.Decorations)
	if start != 0 && decoration.Equivalent(t.Decorations[start-1]) {
		decoration = t.Decorations[start-1]
	}
	t.Runes = append(t.Runes, runes...)
	face := decoration.Font.Face()
	glyphs := decoration.Font.RunesToGlyphs(runes)
	t.widths = append(t.widths, decoration.Font.GlyphWidths(glyphs)...)
	for i, r := range runes {
		t.Decorations = append(t.Decorations, decoration)
		if glyphs[i] == 0 {
			if altFace := face.FallbackForCharacter(r); altFace != nil {
				altDec := *decoration
				altDec.Font = altFace.Font(decoration.Font.Size())
				if start != 0 && altDec.Equivalent(t.Decorations[start-1]) {
					t.Decorations[len(t.Decorations)-1] = t.Decorations[start-1]
				} else {
					t.Decorations[len(t.Decorations)-1] = &altDec
				}
				t.widths[start+i] = altDec.Font.GlyphWidths([]uint16{altDec.Font.RuneToGlyph(r)})[0]
			}
		}
	}
}

// Draw the Text at the given location. y is where the baseline of the text will be placed.
func (t *Text) Draw(canvas *Canvas, x, y float32) {
	if len(t.Decorations) == 0 {
		return
	}
	start := 0
	current := t.Decorations[0]
	nx := x
	for i, d := range t.Decorations {
		if i != 0 && !current.Equivalent(d) {
			canvas.DrawSimpleString(string(t.Runes[start:i]), x, y+current.BaselineOffset, current.Font, current.Paint)
			current = d
			x = nx
			start = i
		}
		nx += t.widths[i]
	}
	if start < len(t.Decorations) {
		canvas.DrawSimpleString(string(t.Runes[start:]), x, y+current.BaselineOffset, current.Font, current.Paint)
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
	for start < len(t.Runes) {
		i := start
		var w float32
		for i < len(t.Runes) && w+t.widths[i] < width {
			w += t.widths[i]
			i++
		}
		if i == len(t.Runes) {
			lines = append(lines, t.Slice(start, len(t.Runes)))
			break
		}
		// Forward past any additional whitespace
		for i < len(t.Runes) && unicode.IsSpace(t.Runes[i]) {
			i++
		}
		// Backup to first break
		for i > start && !isWordBreak(t.Runes[i-1]) {
			i--
		}
		if i == start {
			// Nothing found that fits, so take the first word and any trailing whitespace after it
			for i < len(t.Runes) && !isWordBreak(t.Runes[i]) {
				i++
			}
			if i < len(t.Runes) && isWordBreak(t.Runes[i]) {
				i++
			}
			for i < len(t.Runes) && unicode.IsSpace(t.Runes[i]) {
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
