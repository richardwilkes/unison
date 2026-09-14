// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"slices"
	"unicode"

	"github.com/richardwilkes/unison/accessibility"
)

// textUnit is the piece of a control's content that the four methods of org.a11y.atspi.Text answering with a range work
// in. AT-SPI has two enumerations for these — [Granularity] for GetStringAtOffset and [Boundary] for the three methods
// that predate it — which number the same units differently, so both are read as one of these.
type textUnit uint8

// The units a piece of text is divided into.
const (
	unitChar textUnit = iota
	unitWord
	unitSentence
	unitLine
	unitParagraph
	unitCount
)

// textForm is which of the two ways AT-SPI divides text into units of the same kind a range is built in. They differ in
// where the separators between units are counted: the START form runs from the beginning of one unit to the beginning
// of the next, so the spaces after a word belong to the word they follow, while the END form runs from the end of one
// to the end of the next, so those spaces belong to the word that comes after them. The two therefore hand back
// different ranges for the same offset, and a client walking with one of them and being answered in the other lands a
// separator off on every step.
type textForm uint8

// The two forms.
const (
	formStart textForm = iota
	formEnd
)

// unitForGranularity returns the unit an AtspiTextGranularity names. There is only one form of these: GetStringAtOffset
// divides text the way the START boundaries do.
func unitForGranularity(value uint32) textUnit {
	switch Granularity(value) {
	case GranularityChar:
		return unitChar
	case GranularityWord:
		return unitWord
	case GranularitySentence:
		return unitSentence
	case GranularityParagraph:
		return unitParagraph
	default:
		// GranularityLine, along with anything AT-SPI has not defined: a line is the unit the whole content can be
		// walked in, so it is what a client asking for something unrecognizable is best served by.
		return unitLine
	}
}

// unitForBoundary returns the unit an AtspiTextBoundaryType names and the form it asks for. Every unit but the
// character comes in both forms; a character is a character either way.
func unitForBoundary(value uint32) (unit textUnit, form textForm) {
	switch Boundary(value) {
	case BoundaryChar:
		return unitChar, formStart
	case BoundaryWordStart:
		return unitWord, formStart
	case BoundaryWordEnd:
		return unitWord, formEnd
	case BoundarySentenceStart:
		return unitSentence, formStart
	case BoundarySentenceEnd:
		return unitSentence, formEnd
	case BoundaryLineEnd:
		return unitLine, formEnd
	default:
		// BoundaryLineStart, along with anything AT-SPI has not defined.
		return unitLine, formStart
	}
}

// textRange is a half-open range of rune offsets, which is what the three values a text method answers with describe.
type textRange struct {
	start int
	end   int
}

// textContent is a node's text as the methods that walk it need it: the characters it holds, the lines they are laid
// out over, and where the units of each kind begin and end. See [nodeObject.content], which builds one.
type textContent struct {
	runes []rune
	// lines are the lines the content occupies on the screen, which is what the questions about where a character sits
	// are answered from. A control the snapshot did not measure has one standing in for the whole of it; measured says
	// whether these are the real thing, since a single made-up line is not a division of the text at all.
	lines []accessibility.Line
	// divided holds, per unit, where the units of that kind begin and end. Each is worked out once, the first time a
	// unit of that kind is asked about, and then answered by a search rather than a walk: a client reading a control
	// through with GetTextAfterOffset asks once per unit, and working the division out again on each of those calls —
	// from the beginning of the content every time — is what turns reading a control into quadratic work on the one
	// goroutine that answers every other question the bus asks.
	divided  []*divisions
	measured bool
}

// divisions is where the units of one kind begin and end within a content. starts holds the offset each unit begins at,
// in ascending order, always beginning with zero, and contentEnds holds where the same unit's own characters stop, with
// the separator that follows it left out. edges is what the END form divides the text at, which is those content ends
// along with the two ends of the content itself; see [endEdges].
type divisions struct {
	starts      []int
	contentEnds []int
	edges       []int
}

// rangeAt returns the range of the unit that holds an offset. An offset past the last character belongs to no character
// but does belong to the last line, word, sentence and paragraph, since that is where the caret sits once everything
// has been typed and it is what an assistive technology asking "what am I on?" has to be told about.
func (c *textContent) rangeAt(unit textUnit, form textForm, offset int) textRange {
	offset = clamp(offset, 0, len(c.runes))
	if len(c.runes) == 0 {
		return textRange{}
	}
	if unit == unitChar {
		if offset >= len(c.runes) {
			return textRange{start: offset, end: offset}
		}
		return textRange{start: offset, end: offset + 1}
	}
	d := c.divisionsFor(unit)
	if form == formEnd {
		return d.endRange(offset)
	}
	return d.startRange(offset, len(c.runes))
}

// rangeBefore returns the range of the unit before the one holding an offset, which is empty when the offset is already
// in the first one. A client walking backwards through the content asks for this and then asks again from the start of
// what it was given, so answering with the unit at the offset would leave it walking on the spot.
func (c *textContent) rangeBefore(unit textUnit, form textForm, offset int) textRange {
	at := c.rangeAt(unit, form, offset)
	if at.start <= 0 {
		return textRange{}
	}
	return c.rangeAt(unit, form, at.start-1)
}

// rangeAfter returns the range of the unit after the one holding an offset, which is empty when the offset is already
// in the last one.
func (c *textContent) rangeAfter(unit textUnit, form textForm, offset int) textRange {
	at := c.rangeAt(unit, form, offset)
	if at.end >= len(c.runes) {
		return textRange{start: len(c.runes), end: len(c.runes)}
	}
	return c.rangeAt(unit, form, at.end)
}

// divisionsFor returns where the units of one kind begin and end, working them out the first time they are asked for.
func (c *textContent) divisionsFor(unit textUnit) *divisions {
	if c.divided == nil {
		c.divided = make([]*divisions, unitCount)
	} else if d := c.divided[unit]; d != nil {
		return d
	}
	var d *divisions
	switch unit {
	case unitWord:
		d = c.wordDivisions()
	case unitSentence:
		d = c.sentenceDivisions()
	case unitLine:
		// The lines a control was measured over are what it actually draws, wrapping included. A control the snapshot
		// did not measure — which is every text control but the focused one — is divided at its own line feeds instead,
		// since treating the whole of a multi-line control as one line would have flat review read it out in one go
		// while its state set claims it holds more than one line.
		if c.measured {
			d = c.lineDivisions()
		} else {
			d = c.paragraphDivisions()
		}
	default: // unitParagraph; unitChar is answered without a division at all
		d = c.paragraphDivisions()
	}
	d.edges = endEdges(d.starts, d.contentEnds, len(c.runes))
	c.divided[unit] = d
	return d
}

// startRange returns the range of the unit holding an offset in the START form, which runs from the beginning of that
// unit to the beginning of the next one, so that the separators between two units belong to the one they follow. count
// is how many runes the content holds.
func (d *divisions) startRange(offset, count int) textRange {
	i := lastAtOrBefore(d.starts, offset)
	if i < 0 {
		return textRange{start: 0, end: count}
	}
	end := count
	if i+1 < len(d.starts) {
		end = d.starts[i+1]
	}
	return textRange{start: d.starts[i], end: end}
}

// endRange returns the range of the unit holding an offset in the END form, which runs from the end of the unit before
// it to its own end, so that the separators between two units belong to the one that follows them.
func (d *divisions) endRange(offset int) textRange {
	i, found := slices.BinarySearch(d.edges, offset)
	if found {
		i++
	}
	i = clamp(i, 1, len(d.edges)-1)
	return textRange{start: d.edges[i-1], end: d.edges[i]}
}

// lastAtOrBefore returns the index of the last ascending value that is no greater than offset, or -1 when there is
// none.
func lastAtOrBefore(values []int, offset int) int {
	i, found := slices.BinarySearch(values, offset)
	if found {
		return i
	}
	return i - 1
}

// endEdges returns the boundaries the END form of a unit divides the content at: the beginning of the content, the
// offset each unit's own characters end at, and the end of the content. Anything that would repeat a boundary, which an
// empty unit or one made of nothing but separators produces, is left out, so the result is strictly ascending and every
// offset falls in exactly one of the ranges between two of them.
func endEdges(starts, contentEnds []int, count int) []int {
	edges := make([]int, 0, len(starts)+2)
	edges = append(edges, 0)
	for _, end := range contentEnds {
		if end > edges[len(edges)-1] && end < count {
			edges = append(edges, end)
		}
	}
	return append(edges, count)
}

// wordDivisions returns where the words begin and end. A word is a run of characters that are not spaces: Unison does
// not carry the language its text is written in, so that is the one division that holds everywhere, and punctuation
// belongs to the word it is written against.
//
// The first unit always begins at the start of the content, whether or not a word does, since every offset has to
// belong to one.
func (c *textContent) wordDivisions() *divisions {
	d := &divisions{starts: []int{0}}
	for i := 1; i < len(c.runes); i++ {
		if isWordStart(c.runes, i) {
			d.starts = append(d.starts, i)
		}
	}
	d.contentEnds = make([]int, 0, len(d.starts))
	for _, start := range d.starts {
		end := start
		for end < len(c.runes) && !unicode.IsSpace(c.runes[end]) {
			end++
		}
		d.contentEnds = append(d.contentEnds, end)
	}
	return d
}

// isWordStart reports whether the rune at an index begins a word, which is when it is not a space and what comes before
// it is, or there is nothing before it.
func isWordStart(runes []rune, i int) bool {
	return !unicode.IsSpace(runes[i]) && (i == 0 || unicode.IsSpace(runes[i-1]))
}

// sentenceDivisions returns where the sentences begin and end.
//
// Unison does not analyze prose: a sentence ends at a full stop, question mark or exclamation point that is followed by
// whitespace or by the end of the text, which means an abbreviation ends one here. An assistive technology asking for a
// sentence is asking for somewhere to pause, and a full stop is somewhere to pause.
func (c *textContent) sentenceDivisions() *divisions {
	d := &divisions{}
	for start := 0; ; {
		end := c.sentenceEnd(start)
		d.starts = append(d.starts, start)
		d.contentEnds = append(d.contentEnds, trimTrailingSpace(c.runes, start, end))
		if end >= len(c.runes) {
			return d
		}
		start = end
	}
}

// sentenceEnd returns the offset the sentence beginning at from ends at, which is where the next one begins.
func (c *textContent) sentenceEnd(from int) int {
	for i := from; i < len(c.runes); i++ {
		if !isSentenceTerminator(c.runes[i]) {
			continue
		}
		end := i + 1
		if end >= len(c.runes) {
			return len(c.runes)
		}
		if !unicode.IsSpace(c.runes[end]) {
			// A full stop with a character hard against it is part of the word rather than the end of a sentence,
			// which is what keeps a number or a file name from being read as several of them.
			continue
		}
		for end < len(c.runes) && unicode.IsSpace(c.runes[end]) {
			end++
		}
		return end
	}
	return len(c.runes)
}

// isSentenceTerminator reports whether a rune is one of the marks a sentence can end at.
func isSentenceTerminator(r rune) bool {
	switch r {
	case '.', '?', '!':
		return true
	default:
		return false
	}
}

// paragraphDivisions returns where the paragraphs begin and end. A paragraph is what the text itself is divided into,
// as distinct from a line, which is what the layout divided it into and which changes every time the control is
// resized.
func (c *textContent) paragraphDivisions() *divisions {
	d := &divisions{starts: []int{0}}
	for i, r := range c.runes {
		if r == '\n' {
			d.contentEnds = append(d.contentEnds, i)
			d.starts = append(d.starts, i+1)
		}
	}
	d.contentEnds = append(d.contentEnds, len(c.runes))
	return d
}

// lineDivisions returns where the laid-out lines begin and end, which is what AT-SPI's LINE boundaries mean for a
// control that has been measured: the lines the control actually draws, wrapping included.
func (c *textContent) lineDivisions() *divisions {
	d := &divisions{starts: make([]int, 0, len(c.lines)), contentEnds: make([]int, 0, len(c.lines))}
	for i := range c.lines {
		start := clamp(c.lines[i].Start, 0, len(c.runes))
		if len(d.starts) != 0 && start < d.starts[len(d.starts)-1] {
			continue
		}
		end := clamp(c.lines[i].End, start, len(c.runes))
		d.starts = append(d.starts, start)
		d.contentEnds = append(d.contentEnds, trimTrailingNewline(c.runes, start, end))
	}
	if len(d.starts) == 0 || d.starts[0] != 0 {
		d.starts = append([]int{0}, d.starts...)
		d.contentEnds = append([]int{0}, d.contentEnds...)
	}
	return d
}

// trimTrailingNewline returns the end of a range with the line feed that ends it left out, which is where the line's
// own characters stop.
func trimTrailingNewline(runes []rune, start, end int) int {
	if end > start && runes[end-1] == '\n' {
		return end - 1
	}
	return end
}

// trimTrailingSpace returns the end of a range with the whitespace that follows its last character left out.
func trimTrailingSpace(runes []rune, start, end int) int {
	for end > start && unicode.IsSpace(runes[end-1]) {
		end--
	}
	return end
}
