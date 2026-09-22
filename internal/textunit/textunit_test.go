// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package textunit_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/textunit"
)

// TestWordStarts covers the divisions both platform adapters walk a document's words by. The leading zero is there in
// every answer, including for text that begins with a space and for no text at all, since every offset has to belong to
// a unit.
func TestWordStarts(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name     string
		text     string
		expected []int
	}{
		{name: "nothing at all", text: "", expected: []int{0}},
		{name: "one word", text: "hello", expected: []int{0}},
		{name: "two words", text: "hello there", expected: []int{0, 6}},
		{name: "punctuation belongs to its word", text: "e.g. a file.name", expected: []int{0, 5, 7}},
		{name: "leading space", text: "  hi", expected: []int{0, 2}},
		{name: "runs of spaces", text: "a   b", expected: []int{0, 4}},
		{name: "a line feed is a space", text: "a\nb", expected: []int{0, 2}},
		{name: "trailing space starts nothing", text: "a ", expected: []int{0}},
		{name: "tabs divide too", text: "a\tb", expected: []int{0, 2}},
	} {
		c.Equal(one.expected, textunit.WordStarts([]rune(one.text)), one.name)
	}
}

// TestWordStartsAroundAnObject pins the rule that keeps an image in a document from being read as part of the words
// around it: the object-replacement character is a word of its own, so it begins one wherever it sits and the first
// rune after it that is not a space begins the next. A screen reader walking by word therefore lands on the image, is
// told what it is by the element occupying that offset, and moves on — and never stops on a word holding nothing but
// the space an inline image was written against, which is what the spaces around one would otherwise produce.
func TestWordStartsAroundAnObject(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	const obj = string(textunit.ObjectReplacement)
	c.Equal([]int{0}, textunit.WordStarts([]rune(obj)), "an object on its own is the first word")
	c.Equal([]int{0, 1}, textunit.WordStarts([]rune(obj+"tail")))
	c.Equal([]int{0, 4, 5}, textunit.WordStarts([]rune("head"+obj+"tail")),
		"the object interrupts a word it is written against")
	c.Equal([]int{0, 2, 3}, textunit.WordStarts([]rune("a "+obj+obj)), "two objects are two words")
	c.Equal([]int{0, 2}, textunit.WordStarts([]rune("a "+obj)), "an object after a space is the word the space led to")
	c.Equal([]int{0, 5, 9, 15, 17}, textunit.WordStarts([]rune("Read the guide "+obj+" now.")),
		"the space after an inline image belongs to the image's word rather than being a word of its own")
	c.Equal([]int{0, 3}, textunit.WordStarts([]rune(obj+"  tail")),
		"however many spaces follow an object, the word after it begins where its text does")

	runes := []rune("a" + obj + "b")
	c.True(textunit.IsWordStart(runes, 0))
	c.True(textunit.IsWordStart(runes, 1), "the object begins a word")
	c.True(textunit.IsWordStart(runes, 2), "and the rune after it begins the next")
	c.False(textunit.IsWordStart(runes, -1), "an index outside the text begins nothing")
	c.False(textunit.IsWordStart(runes, len(runes)))
	c.False(textunit.IsWordStart([]rune(" x"), 0), "a space is not the start of a word, however the offsets divide")

	spaced := []rune("a" + obj + " b")
	c.True(textunit.IsWordStart(spaced, 1), "the object still begins a word when a space follows it")
	c.False(textunit.IsWordStart(spaced, 2), "the space after it begins nothing")
	c.True(textunit.IsWordStart(spaced, 3), "and the text after that space begins the next word")
}

// TestParagraphStarts covers the other division the adapters share. The line feed belongs to the paragraph it ends, so
// text that ends in one has a last, empty paragraph: that is where the caret sits once everything has been typed, and
// both platforms have to put it in the same place.
func TestParagraphStarts(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		name     string
		text     string
		expected []int
	}{
		{name: "nothing at all", text: "", expected: []int{0}},
		{name: "one paragraph", text: "one line", expected: []int{0}},
		{name: "two paragraphs", text: "one\ntwo", expected: []int{0, 4}},
		{name: "an empty paragraph between", text: "one\n\ntwo", expected: []int{0, 4, 5}},
		{name: "a trailing line feed", text: "one\n", expected: []int{0, 4}},
		{name: "nothing but line feeds", text: "\n\n", expected: []int{0, 1, 2}},
		{name: "a carriage return is not a division", text: "one\rtwo", expected: []int{0}},
	} {
		c.Equal(one.expected, textunit.ParagraphStarts([]rune(one.text)), one.name)
	}
}
