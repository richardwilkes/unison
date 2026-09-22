// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package textunit divides a run of text into the units an assistive technology walks it in. It holds the rules the
// platform adapters share, so that a word or a paragraph is the same piece of text on every platform: AT-SPI's
// GetStringAtOffset and the UI Automation Text pattern's ExpandToEnclosingUnit are asked the same question by the same
// screen readers, and an answer that differed between them would have a person reading a document hear one thing on
// one desktop and another on the next. It depends on nothing but the standard library, so an adapter can use it without
// reaching into the toolkit.
package textunit

import "unicode"

// ObjectReplacement is the rune that stands in for something that is not text at all — an image within a document. The
// character has no width of its own and nothing to read out, so it is its own word: a screen reader walking by word
// lands on it, says what the image is from the element occupying it, and moves on, rather than pronouncing it as part
// of whatever it is written against.
const ObjectReplacement = '￼'

// WordStarts returns the offset every word begins at, in ascending order. The first entry is always zero, whether or
// not a word begins there, since every offset has to belong to a unit and the text before the first word belongs to the
// first one.
//
// A word is a run of characters that are not spaces: the text does not carry the language it is written in, so that is
// the one division that holds everywhere, and punctuation belongs to the word it is written against.
func WordStarts(runes []rune) []int {
	starts := make([]int, 1, len(runes)/4+2)
	for i := 1; i < len(runes); i++ {
		if IsWordStart(runes, i) {
			starts = append(starts, i)
		}
	}
	return starts
}

// IsWordStart reports whether the rune at an index begins a word, which is when it is not a space and what comes before
// it is, or there is nothing before it. An object-replacement character is a word of its own, so it begins one wherever
// it sits, and the first rune after it that is not a space begins the next.
//
// A space following an object begins nothing, which is what keeps an inline image written between spaces — the ordinary
// case for one — from producing a word that holds nothing but that space: a screen reader walking by word, or asking
// what the word at an offset is, would otherwise stop between the image and the word after it with nothing to read out.
// The space belongs to the image's word instead, exactly as the space after any other word does.
//
// An index outside the text is not the start of anything.
func IsWordStart(runes []rune, i int) bool {
	if i < 0 || i >= len(runes) {
		return false
	}
	if runes[i] == ObjectReplacement {
		return true
	}
	return !unicode.IsSpace(runes[i]) && (i == 0 || unicode.IsSpace(runes[i-1]) || runes[i-1] == ObjectReplacement)
}

// ParagraphStarts returns the offset every paragraph begins at, in ascending order, always beginning with zero. A
// paragraph is what the text itself is divided into — the runs between its line feeds — as distinct from a line, which
// is what the layout divided it into and which changes every time the control is resized.
//
// The line feed belongs to the paragraph it ends, so the paragraph that follows begins after it. Text ending in a line
// feed therefore has one last, empty paragraph, which is where a caret sits once everything has been typed.
func ParagraphStarts(runes []rune) []int {
	starts := make([]int, 1, 8)
	for i, r := range runes {
		if r == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}
