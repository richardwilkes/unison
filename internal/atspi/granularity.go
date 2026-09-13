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
)

// unitForGranularity returns the unit an AtspiTextGranularity names.
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

// unitForBoundary returns the unit an AtspiTextBoundaryType names. Every unit but the character comes in a START and an
// END form, which differ in whether a range runs from the beginning of one unit to the beginning of the next or from
// the end of the previous one to the end of this one; the two divide the same text the same way and differ only in
// which unit the spaces between them are counted with. Both are answered with the START form, which is what the ranges
// here are built as and what a current assistive technology asks for.
func unitForBoundary(value uint32) textUnit {
	switch Boundary(value) {
	case BoundaryChar:
		return unitChar
	case BoundaryWordStart, BoundaryWordEnd:
		return unitWord
	case BoundarySentenceStart, BoundarySentenceEnd:
		return unitSentence
	default:
		// BoundaryLineStart and BoundaryLineEnd, along with anything AT-SPI has not defined.
		return unitLine
	}
}

// textRange is a half-open range of rune offsets, which is what the three values a text method answers with describe.
type textRange struct {
	start int
	end   int
}

// textContent is a node's text as the methods that walk it need it: the characters it holds and the lines they are laid
// out over. See [nodeObject.content], which builds one.
type textContent struct {
	runes []rune
	lines []accessibility.Line
}

// rangeAt returns the range of the unit that holds an offset. An offset past the last character belongs to no character
// but does belong to the last line, word, sentence and paragraph, since that is where the caret sits once everything
// has been typed and it is what an assistive technology asking "what am I on?" has to be told about.
func (c *textContent) rangeAt(unit textUnit, offset int) textRange {
	offset = clamp(offset, 0, len(c.runes))
	if len(c.runes) == 0 {
		return textRange{}
	}
	switch unit {
	case unitChar:
		if offset >= len(c.runes) {
			return textRange{start: offset, end: offset}
		}
		return textRange{start: offset, end: offset + 1}
	case unitWord:
		return c.wordAt(offset)
	case unitSentence:
		return c.sentenceAt(offset)
	case unitParagraph:
		return c.paragraphAt(offset)
	default:
		return c.lineAtOffset(offset)
	}
}

// rangeBefore returns the range of the unit before the one holding an offset, which is empty when the offset is already
// in the first one. A client walking backwards through the content asks for this and then asks again from the start of
// what it was given, so answering with the unit at the offset would leave it walking on the spot.
func (c *textContent) rangeBefore(unit textUnit, offset int) textRange {
	at := c.rangeAt(unit, offset)
	if at.start <= 0 {
		return textRange{}
	}
	return c.rangeAt(unit, at.start-1)
}

// rangeAfter returns the range of the unit after the one holding an offset, which is empty when the offset is already
// in the last one.
func (c *textContent) rangeAfter(unit textUnit, offset int) textRange {
	at := c.rangeAt(unit, offset)
	if at.end >= len(c.runes) {
		return textRange{start: len(c.runes), end: len(c.runes)}
	}
	return c.rangeAt(unit, at.end)
}

// lineAtOffset returns the range of the laid-out line that holds an offset, taking the line feed that ends it with it,
// which is what AT-SPI's LINE_START boundary means. The lines are the ones the snapshot measured, so they are what the
// control actually draws, wrapping included; see [nodeObject.lines] for the control that has not been measured.
func (c *textContent) lineAtOffset(offset int) textRange {
	if len(c.lines) == 0 {
		return textRange{start: 0, end: len(c.runes)}
	}
	line := c.lines[lineAt(c.lines, offset)]
	start := clamp(line.Start, 0, len(c.runes))
	return textRange{start: start, end: clamp(line.End, start, len(c.runes))}
}

// wordAt returns the range of the word that holds an offset, which runs from the word's first character to the first
// character of the next word, so that the spaces between two words belong to the one they follow. That is what AT-SPI's
// WORD_START boundary means, and it is what makes walking a control a word at a time cover every character exactly once
// rather than stopping on the gaps.
//
// A word is a run of characters that are not spaces. Unison does not carry the language its text is written in, so that
// is the one division that holds everywhere; punctuation belongs to the word it is written against.
func (c *textContent) wordAt(offset int) textRange {
	start := 0
	for i := 1; i < len(c.runes) && i <= offset; i++ {
		if isWordStart(c.runes, i) {
			start = i
		}
	}
	end := len(c.runes)
	for i := start + 1; i < len(c.runes); i++ {
		if isWordStart(c.runes, i) {
			end = i
			break
		}
	}
	return textRange{start: start, end: end}
}

// isWordStart reports whether the rune at an index begins a word, which is when it is not a space and what comes before
// it is, or there is nothing before it.
func isWordStart(runes []rune, i int) bool {
	return !unicode.IsSpace(runes[i]) && (i == 0 || unicode.IsSpace(runes[i-1]))
}

// sentenceAt returns the range of the sentence that holds an offset, which runs to the first character of the next one,
// so that the terminator and the spaces after it belong to the sentence they end.
//
// Unison does not analyze prose: a sentence ends at a full stop, question mark or exclamation point that is followed by
// whitespace or by the end of the text, which means an abbreviation ends one here. An assistive technology asking for a
// sentence is asking for somewhere to pause, and a full stop is somewhere to pause.
func (c *textContent) sentenceAt(offset int) textRange {
	start := 0
	for {
		end := c.sentenceEnd(start)
		if end > offset || end >= len(c.runes) {
			return textRange{start: start, end: end}
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

// paragraphAt returns the range of the paragraph that holds an offset, taking the line feed that ends it with it. A
// paragraph is what the text itself is divided into, as distinct from a line, which is what the layout divided it into
// and which changes every time the control is resized.
func (c *textContent) paragraphAt(offset int) textRange {
	start := 0
	for i := 0; i < len(c.runes) && i < offset; i++ {
		if c.runes[i] == '\n' {
			start = i + 1
		}
	}
	end := len(c.runes)
	for i := start; i < len(c.runes); i++ {
		if c.runes[i] == '\n' {
			end = i + 1
			break
		}
	}
	return textRange{start: start, end: end}
}
