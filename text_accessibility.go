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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/side"
	"github.com/richardwilkes/unison/enums/slant"
	"github.com/richardwilkes/unison/enums/weight"
)

// This file holds what every piece of text in a window has in common, whatever widget drew it: how the styles it was
// drawn in become the styled runs an assistive technology is told about, how a piece of static text reports the one
// line it occupies, and where a caret or a range of text sits on the screen.
//
// It exists because none of that is a property of a document. A screen reader reads static text the same way it reads
// a paragraph — by line, by word, by character — so a label, a tag and a table's column header all have to answer the
// same questions a markdown block does, and they answer them with the same code rather than with three approximations
// of it. What the widgets themselves contribute is only where their text was drawn; see Label.axTextLine.

// axTextLiner is implemented by a widget that draws a single line of static text and can say what that line is: its
// runes, what each of them was drawn with, and where the line sits in the widget's own coordinates. A Label provides
// it, so every widget built by embedding one does too, and a widget that draws other words, or draws them somewhere
// else — a column header shrunk by its sort indicator — shadows it through Self with an answer of its own. The runes
// that come back are the text that is published as well as what the line boundaries, the advances and the styled runs
// index, so an override that elides or prefixes what it draws is read as what is on the screen.
type axTextLiner interface {
	axTextLine() ([]rune, []*TextDecoration, accessibility.Line)
}

// axStaticTextLine returns what a widget drawing a single line of static text contributes: its runes, the decoration
// each of them is drawn with, and the one line they occupy, with the line's bounds in the widget's own coordinates and
// one advance per rune boundary. The placement arguments are the ones the widget passes to DrawLabel, so the line is
// where the text actually is rather than where a second calculation of it would put it.
//
// The decorations and runes are the Text's own slices, so neither may be modified. A widget holding no text still
// occupies a line, since it is still as tall as one and a caret can sit in it: the line carries the single advance
// that says where that caret goes and covers no runes at all.
func axStaticTextLine(rect geom.Rect, hAlign, vAlign align.Enum, font Font, text *Text, drawable Drawable,
	drawableSide side.Enum, gap float32,
) (runes []rune, decorations []*TextDecoration, line accessibility.Line) {
	_, _, textPt, txtSize := labelPlacement(rect, hAlign, vAlign, font, text, drawable, drawableSide, gap)
	var widths []float32
	if text != nil {
		runes = text.runes
		decorations = text.decorations
		widths = text.widths
	}
	line.Advances = make([]float32, 0, len(widths)+1)
	line.Advances = append(line.Advances, 0)
	var x float32
	for _, w := range widths {
		x += w
		line.Advances = append(line.Advances, x)
	}
	line.End = len(runes)
	line.Bounds = geom.NewRect(textPt.X, textPt.Y, txtSize.Width, txtSize.Height)
	return runes, decorations, line
}

// axStaticTextInfo turns the answer of axStaticTextLine into what a node carries for a piece of static text: the text
// itself, the one line it was drawn on and the styled runs within it. There is no caret and nothing is selected, since
// static text has neither, and it is not multiline, since it is one line by definition.
//
// Text that covers no runes is reported as no text at all. A label used for spacing, or one showing only an image, has
// nothing for a reading caret to move through, and an empty piece of text an assistive technology could navigate into
// is worse than one it steps over.
func axStaticTextInfo(text string, decorations []*TextDecoration, line accessibility.Line) *accessibility.TextInfo {
	if line.End == 0 {
		return nil
	}
	return &accessibility.TextInfo{
		Text:  text,
		Lines: []accessibility.Line{line},
		Runs:  axRunsFromDecorations(decorations),
	}
}

// axRunsFromDecorations turns what each rune was drawn with into the styled runs an assistive technology is told about,
// coalescing the runes that were drawn the same way so that a paragraph with one bold word in it is three runs rather
// than one per character.
//
// A rune nothing was drawn for — the placeholder standing in for an image, the line feed ending a line — belongs to the
// run before it: it has no style of its own to report, and the runs have to tile the text. One that comes before
// anything else joins the run after it instead, so that an image at the start of a paragraph is reported in the style
// of the words beside it rather than in no style at all.
func axRunsFromDecorations(decorations []*TextDecoration) []accessibility.TextRun {
	if len(decorations) == 0 {
		return nil
	}
	runs := make([]accessibility.TextRun, 0, 8)
	for i, decoration := range decorations {
		if decoration == nil {
			if len(runs) != 0 {
				runs[len(runs)-1].End = i + 1
				continue
			}
			for _, following := range decorations[i+1:] {
				if following != nil {
					decoration = following
					break
				}
			}
			if decoration == nil {
				runs = append(runs, accessibility.TextRun{Start: i, End: i + 1})
				continue
			}
		}
		run := axTextRunStyle(decoration)
		run.Start = i
		run.End = i + 1
		if len(runs) != 0 && axSameStyle(runs[len(runs)-1], run) {
			runs[len(runs)-1].End = i + 1
			continue
		}
		runs = append(runs, run)
	}
	return runs
}

// axTextRunStyle returns the style a decoration draws text in, as an assistive technology is told it. The range is left
// at zero, since it is the caller that knows which runes were drawn this way.
func axTextRunStyle(decoration *TextDecoration) accessibility.TextRun {
	run := accessibility.TextRun{
		Underline:     decoration.Underline,
		Strikethrough: decoration.StrikeThrough,
	}
	if xreflect.IsNil(decoration.Font) {
		return run
	}
	fd := decoration.Font.Descriptor()
	run.Family = fd.Family
	run.Size = fd.Size
	run.Weight = axFontWeight(fd.Weight)
	run.Italic = fd.Slant != slant.Upright
	if face := decoration.Font.Face(); face != nil {
		// A fixed-pitch face is what marks code out from prose on every platform, and it is the only way a code span
		// within a sentence can be told apart from the words around it.
		run.Monospace = face.Monospaced()
	}
	return run
}

// axFontWeight turns a font's weight into the 100-to-900 scale every platform states weights on, where 400 is regular
// and 700 is bold. The two extremes unison offers lie outside that scale and are reported as its ends.
func axFontWeight(w weight.Enum) int {
	return min(max(int(w.EnsureValid()), 100), 900)
}

// axSameStyle reports whether two runs were drawn the same way, whatever part of the text each of them covers.
func axSameStyle(a, b accessibility.TextRun) bool {
	a.Start, a.End = 0, 0
	b.Start, b.End = 0, 0
	return a == b
}

// axAppendRuns adds runs to a list, shifted by offset and coalesced with the run before them when they were drawn the
// same way. The runs have to tile the text, so runes nothing said anything about — the line feed joining one block to
// the next — belong to the run before them.
func axAppendRuns(runs, add []accessibility.TextRun, offset int) []accessibility.TextRun {
	for _, run := range add {
		run.Start += offset
		run.End += offset
		if len(runs) != 0 {
			last := &runs[len(runs)-1]
			if last.End < run.Start {
				last.End = run.Start
			}
			run.Start = max(run.Start, last.End)
			if axSameStyle(*last, run) {
				last.End = max(last.End, run.End)
				continue
			}
		}
		if run.End <= run.Start {
			continue
		}
		runs = append(runs, run)
	}
	return runs
}

// axCoverRuns extends the last run to end, which is how the runes joining one piece of a document to the next — the
// line feed between two blocks, the space after a bullet — are covered without a style of their own.
func axCoverRuns(runs []accessibility.TextRun, end int) []accessibility.TextRun {
	if len(runs) == 0 {
		if end <= 0 {
			return runs
		}
		return append(runs, accessibility.TextRun{End: end})
	}
	if runs[len(runs)-1].End < end {
		runs[len(runs)-1].End = end
	}
	return runs
}

// axLineIndexFor returns the index of the line an offset sits on. A line owns the offsets from its own start up to the
// start of the next, so an offset just past a line feed is the beginning of the line that follows it rather than the
// end of the one before.
func axLineIndexFor(lines []accessibility.Line, offset int) int {
	for i := len(lines) - 1; i > 0; i-- {
		if offset >= lines[i].Start {
			return i
		}
	}
	return 0
}

// axCaretRect returns the one-unit-wide bar the caret at an offset is drawn as, in the coordinates the lines are in.
func axCaretRect(lines []accessibility.Line, offset int) geom.Rect {
	if len(lines) == 0 {
		return geom.Rect{}
	}
	line := lines[axLineIndexFor(lines, offset)]
	x := line.Bounds.X
	if len(line.Advances) != 0 {
		x += line.Advances[min(max(offset-line.Start, 0), len(line.Advances)-1)]
	}
	return geom.NewRect(x, line.Bounds.Y, 1, line.Bounds.Height)
}

// axLineRects returns one rectangle per line a range covers, in the coordinates the lines are in. A piece of a line
// that nothing was drawn for — the line feed that ends it — has no width and contributes nothing.
func axLineRects(lines []accessibility.Line, start, end int) []geom.Rect {
	var rects []geom.Rect
	for _, line := range lines {
		if line.End <= start || line.Start >= end || len(line.Advances) == 0 {
			continue
		}
		from := min(max(start-line.Start, 0), len(line.Advances)-1)
		to := min(max(end-line.Start, 0), len(line.Advances)-1)
		if to <= from {
			continue
		}
		left := line.Bounds.X + line.Advances[from]
		right := line.Bounds.X + line.Advances[to]
		if right <= left {
			continue
		}
		rects = append(rects, geom.NewRect(left, line.Bounds.Y, right-left, line.Bounds.Height))
	}
	return rects
}

// axRangeRect returns the whole area a range covers, which is what has to be brought into view to show it.
func axRangeRect(lines []accessibility.Line, start, end int) geom.Rect {
	var rect geom.Rect
	for i, one := range axLineRects(lines, start, end) {
		if i == 0 {
			rect = one
			continue
		}
		rect = rect.Union(one)
	}
	if rect.Empty() {
		return axCaretRect(lines, start)
	}
	return rect
}
