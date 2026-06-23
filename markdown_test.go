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
)

// collectMarkdownText concatenates the text of every Label in the panel subtree, in tree order, so tests can assert on
// rendered content without depending on the exact panel structure the builder produces.
func collectMarkdownText(p *Panel) string {
	var sb strings.Builder
	var walk func(panel *Panel)
	walk = func(panel *Panel) {
		if label, ok := panel.Self.(*Label); ok {
			if s := label.String(); s != "" {
				if sb.Len() > 0 {
					sb.WriteByte(' ')
				}
				sb.WriteString(s)
			}
		}
		for _, child := range panel.Children() {
			walk(child)
		}
	}
	walk(p)
	return sb.String()
}

func TestMarkdownPreservesText(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("# Title\n\nSome **bold** and _italic_ words.\n\n- first\n- second\n", 400)

	c.True(len(m.Children()) > 0)
	text := collectMarkdownText(m.AsPanel())
	for _, want := range []string{"Title", "bold", "italic", "words", "first", "second"} {
		c.Contains(text, want)
	}
}

func TestMarkdownContentRoundTrips(t *testing.T) {
	c := check.New(t)
	const content = "Hello world\n"
	m := NewMarkdown(false)
	m.SetContent(content, 400)
	c.Equal(content, string(m.ContentBytes()))
}

func TestMarkdownSetContentIsIdempotent(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("# Heading\n\nA paragraph.\n", 400)
	first := m.Children()

	// Re-setting identical content and width must be a no-op: the existing child panels are retained rather than
	// rebuilt.
	m.SetContent("# Heading\n\nA paragraph.\n", 400)
	second := m.Children()
	c.Equal(len(first), len(second))
	if len(first) == len(second) {
		for i := range first {
			c.True(first[i] == second[i])
		}
	}

	// Changing the content rebuilds the tree.
	m.SetContent("Different content.\n", 400)
	c.Contains(collectMarkdownText(m.AsPanel()), "Different")
}

// TestMarkdownHandlesGFMConstructs exercises every block/inline handler so a panic or unhandled-element regression in
// any of them is caught. Assertions are intentionally light because the exact rendered structure is not part of the
// public contract.
func TestMarkdownHandlesGFMConstructs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    string
	}{
		{name: "heading", content: "# A Heading\n", want: "Heading"},
		{name: "thematic break", content: "before\n\n---\n\nafter\n", want: "after"},
		{name: "fenced code", content: "```go\nfmt.Println()\n```\n", want: "Println"},
		{name: "blockquote", content: "> quoted text\n", want: "quoted"},
		{name: "ordered list", content: "1. one\n2. two\n", want: "two"},
		{name: "unordered list", content: "- alpha\n- beta\n", want: "beta"},
		{name: "link", content: "a [link label](http://example.com) here\n", want: "link label"},
		{name: "autolink", content: "see http://example.com today\n", want: "example.com"},
		{name: "emphasis", content: "this is **strong** and *weak*\n", want: "strong"},
		{name: "code span", content: "use `code` inline\n", want: "code"},
		{name: "strikethrough", content: "this is ~~gone~~\n", want: "gone"},
		{
			name:    "table",
			content: "| H1 | H2 |\n| -- | -- |\n| a | b |\n",
			want:    "H1",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			c.True(len(m.Children()) > 0)
			c.Contains(collectMarkdownText(m.AsPanel()), tc.want)
		})
	}
}

func TestMarkdownEmptyContent(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("", 400)
	// Empty content must not panic and must leave no rendered text.
	c.Equal("", collectMarkdownText(m.AsPanel()))
}

// TestMarkdownTableDoesNotWrapWhenItFits guards against regressing the table column-sizing logic so that a table whose
// natural width fits within the available width is left at full width rather than having its columns shrunk (which
// would force premature text wrapping). The column widths are only reduced below the available width when the table's
// natural width actually exceeds it.
func TestMarkdownTableDoesNotWrapWhenItFits(t *testing.T) {
	c := check.New(t)
	const content = "| one | two | three | four | five | six | seven | eight | nine | ten |\n" +
		"|-|-|-|-|-|-|-|-|-|-|\n" +
		"| some text | some more text | ccc | ddd | eee/eee | fff | ggg | hhh | iii | jjj |\n"

	// At a generous width the table fits easily, so no column should be shrunk below the available width.
	m := NewMarkdown(false)
	m.SetContent(content, 800)
	for i, w := range m.columnWidths {
		c.Equal(800, w, "column %d should not be shrunk when the table fits", i)
	}

	// At a width narrower than the table's natural width, columns must be shrunk so the table fits.
	m = NewMarkdown(false)
	m.SetContent(content, 200)
	total := 0
	for _, w := range m.columnWidths {
		c.True(w < 200, "columns should be shrunk when the table cannot fit")
		total += w
	}
	c.True(total <= 200, "shrunk columns must sum to no more than the available width")
}
