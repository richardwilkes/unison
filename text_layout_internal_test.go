// Copyright (c) 2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
)

// These tests lock down the observable layout behavior of Text: the metrics produced by cache(), the two
// position/index mappings, line breaking and the run coalescing performed by Draw(). Every float32 comparison here
// goes through closeTo() or checkClose(), so that a change in the association order of a width sum -- summing the
// per-rune widths in fixed-size lanes rather than strictly left to right, for instance -- cannot break them. Where a
// test needs an exact answer (the midpoint rule of RuneIndexForPosition, for example), it uses a Text whose widths are
// exactly representable powers of two so that no association order can produce a different result.

// closeTo reports whether two float32 values agree to within a tolerance that scales with their magnitude. The
// absolute term covers values near zero and the relative term covers long width sums.
func closeTo(expected, actual float32) bool {
	return xmath.Abs(expected-actual) <= 0.001+1e-4*xmath.Abs(expected)
}

// checkClose asserts that two float32 values agree to within closeTo()'s tolerance.
func checkClose(c check.Checker, expected, actual float32, msg string, args ...any) {
	c.Helper()
	all := make([]any, 0, len(args)+3)
	all = append(all, msg+": expected %v, got %v")
	all = append(all, args...)
	all = append(all, expected, actual)
	c.True(closeTo(expected, actual), all...)
}

// monoDec returns a decoration using the monospaced font, whose uniform advance widths make the line breaking cases
// below easy to reason about.
func monoDec() *TextDecoration {
	return &TextDecoration{Font: MonospacedFont, OnBackgroundInk: Black}
}

// exactWidthText returns a Text whose per-rune widths are exactly the values given. This is only possible from inside
// the package, but it is what makes the boundary conditions of RuneIndexForPosition testable without any dependence on
// the metrics of a real font: every width used is exactly representable as a float32, so the prefix sums are exact no
// matter what order they are accumulated in.
func exactWidthText(dec *TextDecoration, widths ...float32) *Text {
	t := NewTextFromRunes([]rune(strings.Repeat("a", len(widths))), dec)
	copy(t.widths, widths)
	t.extents.Width = -1
	return t
}

// sumWidths returns the sum of the per-rune widths in the range [i, j).
func sumWidths(t *Text, i, j int) float32 {
	var total float32
	for _, w := range t.widths[i:j] {
		total += w
	}
	return total
}

// trimmedWidth returns the width of the Text with any trailing whitespace removed. BreakToWidth() documents that
// trailing whitespace is not considered when deciding whether a line fits.
func trimmedWidth(t *Text) float32 {
	end := len(t.runes)
	for end > 0 && unicode.IsSpace(t.runes[end-1]) {
		end--
	}
	return sumWidths(t, 0, end)
}

// isSingleWord reports whether the runes, with trailing whitespace removed, hold at most one word break and that break
// is the final rune. BreakToWidth() documents that a minimum of one word is placed on a line even when that word is
// wider than the requested width; such a line always looks like this.
func isSingleWord(runes []rune) bool {
	end := len(runes)
	for end > 0 && unicode.IsSpace(runes[end-1]) {
		end--
	}
	for i := range end - 1 {
		if isWordBreak(runes[i]) {
			return false
		}
	}
	return true
}

// pixmapDelta returns the number of differing pixels between two pixmaps of the same size along with the largest
// per-channel difference found. A rasterization that shifts by a fraction of a pixel shows up as a small per-channel
// difference, so callers allow a delta of a couple of levels rather than demanding bit equality.
func pixmapDelta(a, b *raster.Pixmap) (differing, maxChannelDelta int) {
	for i := range a.Pix {
		if a.Pix[i] == b.Pix[i] {
			continue
		}
		differing++
		for shift := range 4 {
			d := int((a.Pix[i]>>(shift*8))&0xff) - int((b.Pix[i]>>(shift*8))&0xff)
			if d < 0 {
				d = -d
			}
			if d > maxChannelDelta {
				maxChannelDelta = d
			}
		}
	}
	return differing, maxChannelDelta
}

// regionDelta is pixmapDelta() restricted to the columns in [minX, maxX).
func regionDelta(a, b *raster.Pixmap, minX, maxX int32) (differing, maxChannelDelta int) {
	for y := range a.Height {
		for x := minX; x < maxX; x++ {
			i := int(y)*int(a.RowPixels) + int(x)
			if a.Pix[i] == b.Pix[i] {
				continue
			}
			differing++
			for shift := range 4 {
				d := int((a.Pix[i]>>(shift*8))&0xff) - int((b.Pix[i]>>(shift*8))&0xff)
				if d < 0 {
					d = -d
				}
				if d > maxChannelDelta {
					maxChannelDelta = d
				}
			}
		}
	}
	return differing, maxChannelDelta
}

// hasColoredPixel reports whether any pixel in the columns [minX, maxX) has color channels that are not all equal to
// each other, i.e. whether anything other than black, white or gray was drawn there. The alpha channel is the high
// byte, matching inkRowBounds(), and the remaining three bytes are the color channels in some order; comparing them to
// each other avoids depending on which order that is.
func hasColoredPixel(pix *raster.Pixmap, minX, maxX int32) bool {
	for y := range pix.Height {
		for x := minX; x < maxX; x++ {
			v := pix.Pix[int(y)*int(pix.RowPixels)+int(x)]
			if v>>24 == 0 {
				continue
			}
			c0 := v & 0xff
			c1 := (v >> 8) & 0xff
			c2 := (v >> 16) & 0xff
			if c0 != c1 || c1 != c2 {
				return true
			}
		}
	}
	return false
}

// inkColumnBounds returns the leftmost and rightmost columns holding a non-transparent pixel in the given row, or
// (-1, -1) if the row is empty.
func inkColumnBounds(pix *raster.Pixmap, y int32) (left, right int32) {
	left = -1
	right = -1
	for x := range pix.Width {
		if pix.Pix[int(y)*int(pix.RowPixels)+int(x)]>>24 != 0 {
			if left == -1 {
				left = x
			}
			right = x
		}
	}
	return left, right
}

// hasInk reports whether any pixel in the pixmap is non-transparent.
func hasInk(pix *raster.Pixmap) bool {
	top, _ := inkRowBounds(pix, 0, pix.Width)
	return top != -1
}

// randomLayoutText builds a deterministic pseudo-random string from an alphabet that exercises both kinds of word
// break recognized by isWordBreak(): whitespace and the slash characters.
func randomLayoutText(rng *rand.Rand, count int) string {
	const alphabet = "aaabbbcccdddeee   //\\"
	var buffer strings.Builder
	for range count {
		buffer.WriteByte(alphabet[rng.IntN(len(alphabet))])
	}
	return buffer.String()
}

// textRun describes one AddString() call made while assembling a multi-decoration Text.
type textRun struct {
	dec *TextDecoration
	str string
}

// TestTextLayoutMetrics locks the results of cache(): the width is the sum of the per-rune widths, and the height and
// baseline are the union of the line boxes of the creation decoration and of every run, each run reserving both its
// font's normal line box and that box shifted by its own BaselineOffset.
func TestTextLayoutMetrics(t *testing.T) {
	plain := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	bold := &TextDecoration{Font: EmphasizedSystemFont, OnBackgroundInk: Black, Underline: true}
	mono := &TextDecoration{Font: MonospacedFont, OnBackgroundInk: Black, StrikeThrough: true}
	bigFont := MonospacedFont.Face().Font(20)
	big := &TextDecoration{Font: bigFont, OnBackgroundInk: Black}
	bigSup := &TextDecoration{Font: bigFont, OnBackgroundInk: Black, BaselineOffset: -7}
	bigSub := &TextDecoration{Font: bigFont, OnBackgroundInk: Black, BaselineOffset: 7}
	sup := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: -6}
	sub := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: 5}

	for _, tc := range []struct {
		name        string
		creationDec *TextDecoration
		runs        []textRun
	}{
		{
			name:        "no runs",
			creationDec: plain,
		},
		{
			name:        "empty with superscript creation decoration",
			creationDec: sup,
		},
		{
			name:        "single run",
			creationDec: plain,
			runs:        []textRun{{str: "Hello, World", dec: plain}},
		},
		{
			name:        "mixed fonts",
			creationDec: plain,
			runs: []textRun{
				{str: "Hello ", dec: plain},
				{str: "big ", dec: bold},
				{str: "wide ", dec: mono},
				{str: "world", dec: big},
			},
		},
		{
			name:        "baseline offsets",
			creationDec: plain,
			runs: []textRun{
				{str: "base", dec: plain},
				{str: "up", dec: sup},
				{str: "down", dec: sub},
				{str: "base again", dec: plain},
			},
		},
		{
			// The tallest run also carries a positive offset, so the top of the extents must come from that run's
			// unshifted line box rather than from its shifted one.
			name:        "large font raised by a positive baseline offset",
			creationDec: plain,
			runs: []textRun{
				{str: "small ", dec: plain},
				{str: "lowered", dec: bigSub},
			},
		},
		{
			// The mirror image: the bottom of the extents must come from the tallest run's unshifted line box rather
			// than from the box its negative offset moved up.
			name:        "large font raised by a negative baseline offset",
			creationDec: plain,
			runs: []textRun{
				{str: "small ", dec: plain},
				{str: "raised", dec: bigSup},
			},
		},
		{
			name:        "creation decoration taller than every run",
			creationDec: sup,
			runs: []textRun{
				{str: "only ", dec: plain},
				{str: "plain runs", dec: mono},
			},
		},
		{
			name:        "adjacent equivalent decorations",
			creationDec: plain,
			runs: []textRun{
				{str: "one", dec: plain},
				{str: "two", dec: plain.Clone()},
				{str: "three", dec: plain.Clone()},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			txt := NewText("", tc.creationDec)
			for _, r := range tc.runs {
				txt.AddString(r.str, r.dec)
			}

			// Build a reference Text per run using only that run's decoration. For a single-decoration Text the
			// height and baseline are exactly that decoration's line box, so the union of the reference boxes is what
			// the combined Text must report.
			empty := NewText("", tc.creationDec)
			expectedWidth := float32(0)
			expectedBaseline := empty.Baseline()
			expectedDescent := empty.Height() - empty.Baseline()
			var expectedStr strings.Builder
			for _, r := range tc.runs {
				ref := NewText(r.str, r.dec)
				expectedWidth += ref.Width()
				expectedBaseline = max(expectedBaseline, ref.Baseline())
				expectedDescent = max(expectedDescent, ref.Height()-ref.Baseline())
				expectedStr.WriteString(r.str)
			}

			c.Equal(expectedStr.String(), txt.String())
			c.Equal(len([]rune(expectedStr.String())), len(txt.runes))
			c.Equal(len(txt.runes), len(txt.widths))
			c.Equal(len(txt.runes), len(txt.decorations))

			checkClose(c, expectedWidth, txt.Width(), "width vs sum of per-run reference widths")
			checkClose(c, sumWidths(txt, 0, len(txt.widths)), txt.Width(), "width vs sum of per-rune widths")
			checkClose(c, expectedBaseline, txt.Baseline(), "baseline vs union of line boxes")
			checkClose(c, expectedBaseline+expectedDescent, txt.Height(), "height vs union of line boxes")

			// Extents() must agree with the individual accessors.
			extents := txt.Extents()
			checkClose(c, txt.Width(), extents.Width, "extents width")
			checkClose(c, txt.Height(), extents.Height, "extents height")

			// The height always covers a full line, even when there is no content at all.
			c.True(txt.Height() > 0, "height must always reserve a full line")
			c.True(txt.Baseline() > 0, "baseline must always be positive")
			c.True(txt.Baseline() <= txt.Height(), "baseline must fall within the extents")

			if len(tc.runs) == 0 {
				c.Equal(float32(0), txt.Width())
				c.True(txt.Empty(), "a Text with no runs must report itself as empty")
			} else {
				c.True(txt.Width() > 0, "a Text with content must have a positive width")
				c.False(txt.Empty(), "a Text with content must not report itself as empty")
			}
		})
	}

	t.Run("metrics recompute after adding runes", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("abc", plain)
		before := txt.Width()
		txt.AddString("def", sub)
		checkClose(c, before+NewText("def", sub).Width(), txt.Width(), "width after adding a run")
		checkClose(c, NewText("abcdef", plain).Height()+5, txt.Height(), "height after adding a subscript run")
	})

	t.Run("slice width matches the span it covers", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("Hello ", plain)
		txt.AddString("World", bold)
		for i := range len(txt.runes) + 1 {
			for j := i; j <= len(txt.runes); j++ {
				checkClose(c, sumWidths(txt, i, j), txt.Slice(i, j).Width(), "Slice(%d, %d) width", i, j)
			}
		}
	})
}

// TestTextLayoutPositionForRuneIndex locks the index-to-x mapping: index 0 and any negative index map to 0, the
// length maps to the full width, indexes past the length clamp to the full width rather than panicking, and the
// mapping never decreases.
func TestTextLayoutPositionForRuneIndex(t *testing.T) {
	plain := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	sub := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BaselineOffset: 5}

	mixed := NewText("Hello ", plain)
	mixed.AddString("World", sub)

	for _, tc := range []struct {
		txt  *Text
		name string
	}{
		{name: "empty text", txt: NewText("", plain)},
		{name: "single rune", txt: NewText("H", plain)},
		{name: "single run", txt: NewText("Hello, World", plain)},
		{name: "multiple runs", txt: mixed},
		{name: "exact widths", txt: exactWidthText(plain, 4, 8, 16)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			count := len(tc.txt.runes)

			// Negative indexes and index 0 are exactly 0, not merely close to it.
			c.Equal(float32(0), tc.txt.PositionForRuneIndex(-1000))
			c.Equal(float32(0), tc.txt.PositionForRuneIndex(-1))
			c.Equal(float32(0), tc.txt.PositionForRuneIndex(0))

			// The end of the string is the full width, and indexes past the end clamp to it.
			checkClose(c, tc.txt.Width(), tc.txt.PositionForRuneIndex(count), "position of the final index")
			checkClose(c, tc.txt.Width(), tc.txt.PositionForRuneIndex(count+1), "position one past the end")
			checkClose(c, tc.txt.Width(), tc.txt.PositionForRuneIndex(count+9999), "position far past the end")

			// The mapping is monotone and each step advances by exactly that rune's width.
			previous := float32(0)
			for i := range count {
				pos := tc.txt.PositionForRuneIndex(i)
				c.True(pos >= previous-0.001, "position at %d (%v) must not be less than at %d (%v)", i, pos, i-1,
					previous)
				checkClose(c, sumWidths(tc.txt, 0, i), pos, "position of index %d", i)
				next := tc.txt.PositionForRuneIndex(i + 1)
				checkClose(c, tc.txt.widths[i], next-pos, "step from index %d to %d", i, i+1)
				previous = pos
			}
		})
	}

	t.Run("known positions", func(t *testing.T) {
		c := check.New(t)
		txt := exactWidthText(plain, 4, 8, 16)
		for i, expected := range []float32{0, 4, 12, 28, 28, 28} {
			checkClose(c, expected, txt.PositionForRuneIndex(i), "PositionForRuneIndex(%d)", i)
		}
		checkClose(c, 28, txt.Width(), "width")
	})
}

// TestTextLayoutRuneIndexForPosition locks the x-to-index mapping, including the midpoint rule: a coordinate that
// falls inside a glyph resolves to the following index only when it is strictly past the middle of that glyph, so a
// coordinate exactly on the midpoint resolves to the glyph's own index.
func TestTextLayoutRuneIndexForPosition(t *testing.T) {
	plain := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
	bold := &TextDecoration{Font: EmphasizedSystemFont, OnBackgroundInk: Black}

	t.Run("midpoint rule with exact widths", func(t *testing.T) {
		c := check.New(t)
		// Glyph 0 spans [0, 4) with midpoint 2, glyph 1 spans [4, 12) with midpoint 8 and glyph 2 spans [12, 28) with
		// midpoint 20. Every value below is exactly representable, so the comparisons are exact.
		txt := exactWidthText(plain, 4, 8, 16)
		for _, tc := range []struct {
			name     string
			x        float32
			expected int
		}{
			{name: "far left", x: -1000, expected: 0},
			{name: "negative", x: -0.5, expected: 0},
			{name: "origin", x: 0, expected: 0},
			{name: "just before the first midpoint", x: 1.9375, expected: 0},
			{name: "exactly on the first midpoint", x: 2, expected: 0},
			{name: "just past the first midpoint", x: 2.0625, expected: 1},
			{name: "just before the first boundary", x: 3.9375, expected: 1},
			{name: "exactly on the first boundary", x: 4, expected: 1},
			{name: "just before the second midpoint", x: 7.9375, expected: 1},
			{name: "exactly on the second midpoint", x: 8, expected: 1},
			{name: "just past the second midpoint", x: 8.0625, expected: 2},
			{name: "exactly on the second boundary", x: 12, expected: 2},
			{name: "exactly on the third midpoint", x: 20, expected: 2},
			{name: "just past the third midpoint", x: 20.0625, expected: 3},
			{name: "just before the end", x: 27.9375, expected: 3},
			{name: "exactly at the end", x: 28, expected: 3},
			{name: "past the end", x: 28.0625, expected: 3},
			{name: "far right", x: 1000, expected: 3},
		} {
			c.Equal(tc.expected, txt.RuneIndexForPosition(tc.x), "%s: RuneIndexForPosition(%v)", tc.name, tc.x)
		}
	})

	mixed := NewText("Hello ", plain)
	mixed.AddString("World", bold)

	for _, tc := range []struct {
		txt  *Text
		name string
	}{
		{name: "no runes", txt: NewText("", plain)},
		{name: "single rune", txt: NewText("H", plain)},
		{name: "single run", txt: NewText("Hello, World", plain)},
		{name: "multiple runs", txt: mixed},
		{name: "exact widths", txt: exactWidthText(plain, 4, 8, 16)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			count := len(tc.txt.runes)

			// Anything at or left of the origin is the start of the string.
			c.Equal(0, tc.txt.RuneIndexForPosition(-1000))
			c.Equal(0, tc.txt.RuneIndexForPosition(-0.5))
			c.Equal(0, tc.txt.RuneIndexForPosition(0))

			// Anything at or right of the full width is the end of the string.
			c.Equal(count, tc.txt.RuneIndexForPosition(tc.txt.Width()))
			c.Equal(count, tc.txt.RuneIndexForPosition(tc.txt.Width()+1000))

			// Round trip every rune index. A coordinate a quarter of the way into a glyph is well short of that
			// glyph's midpoint and a coordinate three quarters of the way in is well past it, so the margins dwarf
			// any change in the low bits of a width sum.
			for i := range count {
				w := tc.txt.widths[i]
				if w <= 0 {
					continue
				}
				pos := tc.txt.PositionForRuneIndex(i)
				early := tc.txt.RuneIndexForPosition(pos + w/4)
				c.True(early == i || early == i+1, "index %d: quarter-way lookup returned %d", i, early)
				late := tc.txt.RuneIndexForPosition(pos + 3*w/4)
				c.True(late == i || late == i+1, "index %d: three-quarter-way lookup returned %d", i, late)
				if w > 0.1 {
					// With a comfortable glyph width the midpoint rule pins down the exact answer.
					c.Equal(i, early, "index %d: quarter-way lookup must stay on the glyph", i)
					c.Equal(i+1, late, "index %d: three-quarter-way lookup must advance past the glyph", i)
				}
			}
		})
	}

	t.Run("randomized round trip", func(t *testing.T) {
		c := check.New(t)
		rng := rand.New(rand.NewPCG(0x7ea7, 0x1a4014)) //nolint:gosec // Deterministic seed for reproducible tests
		for range 32 {
			txt := NewText(randomLayoutText(rng, 1+rng.IntN(24)), monoDec())
			for i := range len(txt.runes) {
				w := txt.widths[i]
				if w <= 0 {
					continue
				}
				pos := txt.PositionForRuneIndex(i)
				idx := txt.RuneIndexForPosition(pos + w/4)
				c.True(idx == i || idx == i+1, "%q index %d: lookup returned %d", txt.String(), i, idx)
			}
		}
	})
}

// TestTextLayoutBreakToWidth locks the line breaking behavior: the lines rejoin into the original runes, each line
// other than the last fits within the width once trailing whitespace is discounted, a lone word wider than the width
// still gets a line of its own, a width the text already fits within returns the very same Text, and the decorations
// ride along with the lines.
func TestTextLayoutBreakToWidth(t *testing.T) {
	dec := monoDec()
	// Every rune of the monospaced font advances by the same amount, so the expectations below are stated in terms of
	// that advance and stay well clear of the comparisons BreakToWidth() makes.
	advance := NewText("a", dec).Width()

	for _, tc := range []struct {
		name     string
		str      string
		expected []string
		width    float32
	}{
		{
			name:     "breaks at spaces",
			str:      "aaa bbb ccc",
			width:    advance * 3.7,
			expected: []string{"aaa ", "bbb ", "ccc"},
		},
		{
			name:     "breaks at a slash",
			str:      "aaa/bbb ccc",
			width:    advance * 4.7,
			expected: []string{"aaa/", "bbb ", "ccc"},
		},
		{
			name:     "trailing whitespace is not counted against the width",
			str:      "aa  bb",
			width:    advance * 3.7,
			expected: []string{"aa  ", "bb"},
		},
		{
			name:     "a word wider than the width gets its own line",
			str:      "aaaaaaaaaa bb",
			width:    advance * 3.7,
			expected: []string{"aaaaaaaaaa ", "bb"},
		},
		{
			name:     "a single word wider than the width is not split",
			str:      "aaaaaaaaaa",
			width:    advance * 3.7,
			expected: []string{"aaaaaaaaaa"},
		},
		{
			name:     "whitespace only",
			str:      "   ",
			width:    advance * 1.2,
			expected: []string{"   "},
		},
		{
			name:     "already fits",
			str:      "aaa bbb ccc",
			width:    advance * 100,
			expected: []string{"aaa bbb ccc"},
		},
		{
			name:     "empty string",
			str:      "",
			width:    advance * 3.7,
			expected: []string{""},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			txt := NewText(tc.str, dec)
			lines := txt.BreakToWidth(tc.width)

			actual := make([]string, len(lines))
			for i, line := range lines {
				actual[i] = line.String()
			}
			c.Equal(tc.expected, actual)

			// The lines always rejoin into the original runes and always carry the original decorations along, by
			// pointer, so a caller can still compare decorations across lines.
			var rejoined []rune
			var decorations []*TextDecoration
			for _, line := range lines {
				rejoined = append(rejoined, line.runes...)
				decorations = append(decorations, line.decorations...)
			}
			c.Equal(tc.str, string(rejoined))
			c.Equal(len(txt.decorations), len(decorations))
			for i := range decorations {
				c.True(txt.decorations[i] == decorations[i], "decoration %d was not preserved", i)
			}

			// A width the text already fits within hands back the very same Text rather than a copy.
			if txt.Width() <= tc.width {
				c.Equal(1, len(lines))
				c.True(lines[0] == txt, "a text that already fits must be returned unchanged")
			}

			// Every line but the last fits, once trailing whitespace is discounted, unless it is the documented
			// single-word-wider-than-the-width case.
			for i, line := range lines[:max(len(lines)-1, 0)] {
				if isSingleWord(line.runes) {
					continue
				}
				c.True(trimmedWidth(line) <= tc.width+0.001, "line %d (%q) does not fit within %v: %v", i,
					line.String(), tc.width, trimmedWidth(line))
			}
		})
	}

	t.Run("a width at least the full width returns the same text", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("aaa bbb ccc", dec)
		for _, width := range []float32{txt.Width(), txt.Width() + 0.5, txt.Width() * 2, 100000} {
			lines := txt.BreakToWidth(width)
			c.Equal(1, len(lines))
			c.True(lines[0] == txt, "BreakToWidth(%v) must return the same Text", width)
		}
	})

	t.Run("an over-wide single word yields a distinct one line result", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("aaaaaaaaaa", dec)
		lines := txt.BreakToWidth(advance * 3.7)
		c.Equal(1, len(lines))
		c.Equal(txt.String(), lines[0].String())
		c.True(lines[0] != txt, "a text that had to be broken must not be returned as itself")
		checkClose(c, txt.Width(), lines[0].Width(), "width of the sole line")
	})

	t.Run("decorations are preserved across a multi-decoration break", func(t *testing.T) {
		c := check.New(t)
		bold := &TextDecoration{Font: MonospacedFont, OnBackgroundInk: Black, Underline: true}
		txt := NewText("aaa bbb ", dec)
		txt.AddString("ccc ddd", bold)
		lines := txt.BreakToWidth(advance * 4.7)
		c.True(len(lines) > 1, "the text should have been broken into multiple lines")
		offset := 0
		for _, line := range lines {
			for i := range line.runes {
				c.Equal(txt.runes[offset+i], line.runes[i])
				c.True(txt.decorations[offset+i] == line.decorations[i], "decoration %d was not preserved", offset+i)
				c.Equal(txt.widths[offset+i], line.widths[i])
			}
			offset += len(line.runes)
		}
		c.Equal(len(txt.runes), offset)
		// Both decorations must still be present somewhere in the results.
		var sawPlain, sawBold bool
		for _, line := range lines {
			for _, d := range line.decorations {
				if d.Underline {
					sawBold = true
				} else {
					sawPlain = true
				}
			}
		}
		c.True(sawPlain, "the plain decoration was lost")
		c.True(sawBold, "the underlined decoration was lost")
	})

	t.Run("randomized rejoin and fit", func(t *testing.T) {
		c := check.New(t)
		rng := rand.New(rand.NewPCG(0xb4ea6, 0x104d7)) //nolint:gosec // Deterministic seed for reproducible tests
		for range 64 {
			str := randomLayoutText(rng, 1+rng.IntN(60))
			txt := NewText(str, dec)
			width := advance * float32(1+rng.IntN(10)) * 1.3
			lines := txt.BreakToWidth(width)
			c.True(len(lines) > 0, "%q must break into at least one line", str)
			var rejoined []rune
			for _, line := range lines {
				rejoined = append(rejoined, line.runes...)
			}
			c.Equal(str, string(rejoined))
			for i, line := range lines[:max(len(lines)-1, 0)] {
				if isSingleWord(line.runes) {
					continue
				}
				c.True(trimmedWidth(line) <= width+0.001, "%q line %d (%q) does not fit within %v", str, i,
					line.String(), width)
			}
		}
	})
}

// TestTextLayoutDrawRunCoalescing locks the run coalescing Draw() performs. Adjacent decorations that are merely
// equivalent, rather than the identical pointer, must still be drawn as a single run, and a genuinely different
// decoration must start a new run at exactly the accumulated width of everything before it.
func TestTextLayoutDrawRunCoalescing(t *testing.T) {
	const width = int32(160)
	const height = int32(64)
	origin := geom.NewPoint(4, 32)
	// A background ink makes coalescing visible: a run is painted as one filled rectangle, so a text that failed to
	// coalesce would paint a rectangle per rune and leave anti-aliased seams between them.
	base := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black, BackgroundInk: Blue}

	drawn := func(txt *Text) *raster.Pixmap {
		canvas, pix := newPixmapCanvas(width, height)
		txt.Draw(canvas, origin)
		return pix
	}

	same := NewText("Hello", base)

	// The expectations below are ground truth built by calling TextDecoration.DrawText() directly rather than by
	// running Draw() over some other Text, so that a Draw() that stopped coalescing would not quietly move the
	// expectation along with the result. oneRun is what "Hello" must look like when it is emitted as a single run;
	// perRun is what it would look like if every rune were emitted on its own.
	oneRunCanvas, oneRun := newPixmapCanvas(width, height)
	base.DrawText(oneRunCanvas, "Hello", origin, same.Width())
	perRunCanvas, perRun := newPixmapCanvas(width, height)
	runeX := origin.X
	for i, r := range same.runes {
		base.DrawText(perRunCanvas, string(r), geom.NewPoint(runeX, origin.Y), same.widths[i])
		runeX += same.widths[i]
	}

	t.Run("the coalesced and per rune renderings are distinguishable", func(t *testing.T) {
		c := check.New(t)
		c.True(hasInk(oneRun), "nothing was drawn")
		// Without this the comparisons below would have no teeth: they only mean something because emitting the runes
		// one at a time really does produce a visibly different result.
		differing, maxDelta := pixmapDelta(oneRun, perRun)
		c.True(differing > 0 && maxDelta > 2, "a per-rune rendering must differ from a single-run rendering")
	})

	t.Run("identical decoration pointers coalesce into one run", func(t *testing.T) {
		c := check.New(t)
		// AddString() unifies adjacent equivalent decorations, so a Text built the normal way holds one pointer.
		for _, d := range same.decorations {
			c.True(d == same.decorations[0], "adjacent equivalent decorations should share a single pointer")
		}
		differing, maxDelta := pixmapDelta(oneRun, drawn(same))
		c.True(maxDelta <= 2, "%d pixels differ, by up to %d levels, from a single DrawText() call", differing,
			maxDelta)
	})

	t.Run("equivalent decorations with distinct pointers coalesce into one run", func(t *testing.T) {
		c := check.New(t)
		txt := NewText("Hello", base)
		// AddString() collapses adjacent equivalent decorations into one pointer, so the distinct-pointer case has to
		// be built directly. Draw() must still coalesce these into a single run.
		for i := range txt.decorations {
			txt.decorations[i] = txt.decorations[i].Clone()
		}
		for i := 1; i < len(txt.decorations); i++ {
			c.True(txt.decorations[i] != txt.decorations[i-1], "the clones must be distinct pointers")
			c.True(txt.decorations[i].Equivalent(txt.decorations[i-1]), "the clones must remain equivalent")
		}
		pix := drawn(txt)
		differing, maxDelta := pixmapDelta(oneRun, pix)
		c.True(maxDelta <= 2, "%d pixels differ, by up to %d levels, from a single DrawText() call", differing,
			maxDelta)
		// Distinct pointers and shared pointers must land on exactly the same pixels.
		differing, maxDelta = pixmapDelta(drawn(same), pix)
		c.True(maxDelta <= 2, "%d pixels differ, by up to %d levels, from the shared-pointer rendering", differing,
			maxDelta)
	})

	t.Run("a distinct decoration mid string starts a new run", func(t *testing.T) {
		c := check.New(t)
		const splitIndex = 3
		// No background ink here, so the only colored pixels are the ones the second run's ink puts down.
		noBackground := &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}
		plainReference := drawn(NewText("Hello", noBackground))
		txt := NewText("Hel", noBackground)
		colored := noBackground.Clone()
		colored.OnBackgroundInk = Red
		txt.AddString("lo", colored)
		c.False(txt.decorations[splitIndex].Equivalent(txt.decorations[splitIndex-1]),
			"the two runs must not be equivalent")
		pix := drawn(txt)
		c.True(hasInk(pix), "nothing was drawn")

		// The whole rendering differs from the single-run rendering, but only to the right of the split: the first
		// run is drawn at the same place with the same decoration either way.
		differing, _ := pixmapDelta(plainReference, pix)
		c.True(differing > 0, "the second run's decoration had no effect on the rendering")
		split := origin.X + txt.PositionForRuneIndex(splitIndex)
		leftEdge := int32(xmath.Floor(split))
		leftDiffering, leftMaxDelta := regionDelta(plainReference, pix, 0, leftEdge)
		c.True(leftMaxDelta <= 2, "%d pixels left of the split differ, by up to %d levels", leftDiffering,
			leftMaxDelta)

		// The second run's ink is colored and the first run's is not, so the split landed where the accumulated
		// widths say it should.
		c.False(hasColoredPixel(pix, 0, leftEdge), "the first run must not have picked up the second run's ink")
		c.True(hasColoredPixel(pix, int32(xmath.Ceil(split)), width),
			"the second run's ink must be visible to the right of the split")
	})

	t.Run("an empty text draws nothing", func(t *testing.T) {
		c := check.New(t)
		c.False(hasInk(drawn(NewText("", base))), "an empty Text must not draw anything")
		c.False(hasInk(drawn(NewText("Hello", base).Slice(2, 2))), "an empty slice must not draw anything")
	})

	t.Run("the run background spans the run width", func(t *testing.T) {
		c := check.New(t)
		// Find a row that is inside the line box but above every glyph, using a rendering with no background at all.
		plainPix := drawn(NewText("Hello", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black}))
		glyphTop, _ := inkRowBounds(plainPix, 0, width)
		c.NotEqual(int32(-1), glyphTop)
		row := int32(xmath.Ceil(origin.Y - SystemFont.Baseline()))
		c.True(row < glyphTop, "row %d is not above the topmost glyph row %d", row, glyphTop)

		left, right := inkColumnBounds(drawn(same), row)
		c.Equal(int32(xmath.Floor(origin.X)), left)
		// The rectangle's right edge lands on a fractional coordinate, so allow the column it partially covers to
		// move by one rather than pinning it to a value derived from a float32 sum.
		expected := int32(xmath.Ceil(origin.X+same.Width())) - 1
		c.True(right >= expected-1 && right <= expected+1,
			"the background ends at column %d, expected about %d", right, expected)
	})

	t.Run("the run boundary falls at the accumulated width", func(t *testing.T) {
		c := check.New(t)
		const splitIndex = 3
		// Only the first run gets a background, so the painted rectangle marks exactly where the run ends.
		txt := NewText("Hel", base)
		txt.AddString("lo", &TextDecoration{Font: SystemFont, OnBackgroundInk: Black})
		pix := drawn(txt)
		row := int32(xmath.Ceil(origin.Y - SystemFont.Baseline()))
		left, right := inkColumnBounds(pix, row)
		c.Equal(int32(xmath.Floor(origin.X)), left)
		expected := int32(xmath.Ceil(origin.X+txt.PositionForRuneIndex(splitIndex))) - 1
		c.True(right >= expected-1 && right <= expected+1,
			"the first run's background ends at column %d, expected about %d", right, expected)
	})
}
