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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/behavior"
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

func TestMarkdownHeadingAnchors(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("# New\n\ntext\n\n## Second Heading\n\nmore\n\n### Custom {#my-id}\n\nend\n", 400)

	// Headings automatically get GitHub-style slug anchors.
	c.NotNil(m.anchors["new"])
	c.NotNil(m.anchors["second-heading"])
	// An explicit {#id} overrides the auto-generated slug.
	c.NotNil(m.anchors["my-id"])

	// ScrollToAnchor accepts a bare slug, a '#'-prefixed slug, and is case-insensitive (so a link written as "#New"
	// still resolves to the lower-cased "new" slug).
	c.True(m.ScrollToAnchor("new"))
	c.True(m.ScrollToAnchor("#new"))
	c.True(m.ScrollToAnchor("#New"))
	c.True(m.ScrollToAnchor("#Second-Heading"))
	c.True(m.ScrollToAnchor("#my-id"))

	// A URL-escaped anchor is unescaped before matching.
	c.True(m.ScrollToAnchor("#second%2Dheading"))

	// An unknown anchor reports no match so the caller can fall back to normal link handling.
	c.False(m.ScrollToAnchor("#does-not-exist"))
}

// markdownInScrollPanel builds a Markdown from the given content inside a ScrollPanel with the given viewport height,
// lays it out, and returns both so tests can assert on scroll behavior. The root of the ScrollPanel sits at (0,0), so a
// panel's root coordinates are also its coordinates relative to the visible viewport (0..viewHeight).
func markdownInScrollPanel(content string, viewHeight float32) (*ScrollPanel, *Markdown) {
	m := NewMarkdown(false)
	m.SetContent(content, 400)
	scroll := NewScrollPanel()
	scroll.SetContent(m, behavior.Fill, behavior.Fill)
	scroll.SetFrameRect(geom.NewRect(0, 0, 420, viewHeight))
	scroll.ValidateLayout()
	return scroll, m
}

// TestMarkdownScrollToAnchorAlignsTop verifies that scrolling to an anchor brings the top of the heading to the top of
// the view (revealing the section that follows it) rather than leaving it pinned to the bottom of the view.
func TestMarkdownScrollToAnchorAlignsTop(t *testing.T) {
	c := check.New(t)
	var sb strings.Builder
	sb.WriteString("# Top\n\n")
	for i := range 40 {
		fmt.Fprintf(&sb, "line %d of the top section\n\n", i)
	}
	sb.WriteString("## Target Heading\n\n")
	for i := range 40 {
		fmt.Fprintf(&sb, "line %d of the target section\n\n", i)
	}
	const viewHeight float32 = 300
	_, m := markdownInScrollPanel(sb.String(), viewHeight)

	p := m.anchors["target-heading"]
	c.NotNil(p)
	// Before scrolling, the heading is far below the viewport.
	c.True(m.ScrollToAnchor("#Target-Heading"))
	rect := p.RectToRoot(p.ContentRect(true))
	// The heading's top is brought to the top of the view, and the whole heading fits within the view.
	c.Equal(float32(0), rect.Y)
	c.True(rect.Bottom() <= viewHeight)
}

// TestMarkdownScrollToAnchorTallerThanView verifies that when the heading itself is taller than the view, its top (not
// its bottom) is aligned with the top of the view, so it isn't reduced to showing only its last "bit".
func TestMarkdownScrollToAnchorTallerThanView(t *testing.T) {
	c := check.New(t)
	var sb strings.Builder
	sb.WriteString("# Top\n\n")
	for i := range 40 {
		fmt.Fprintf(&sb, "line %d\n\n", i)
	}
	sb.WriteString("## Target Heading\n\nbody\n")
	// A viewport shorter than a single heading forces the "cannot fit" path.
	_, m := markdownInScrollPanel(sb.String(), 10)

	p := m.anchors["target-heading"]
	c.NotNil(p)
	c.True(m.ScrollToAnchor("target-heading"))
	c.Equal(float32(0), p.RectToRoot(p.ContentRect(true)).Y)
}

// TestMarkdownScrollToAnchorNearEndClamps verifies that a heading near the very end of the document, which cannot be
// pulled all the way to the top because there isn't enough content below it, is still brought fully into view (clamped
// to the available scroll range) rather than being left off-screen.
func TestMarkdownScrollToAnchorNearEndClamps(t *testing.T) {
	c := check.New(t)
	var sb strings.Builder
	sb.WriteString("# Top\n\n")
	for i := range 60 {
		fmt.Fprintf(&sb, "line %d\n\n", i)
	}
	sb.WriteString("## Last\n\ntail\n")
	const viewHeight float32 = 300
	scroll, m := markdownInScrollPanel(sb.String(), viewHeight)

	p := m.anchors["last"]
	c.NotNil(p)
	c.True(m.ScrollToAnchor("last"))
	rect := p.RectToRoot(p.ContentRect(true))
	// The heading is fully visible within the viewport...
	c.True(rect.Y >= 0)
	c.True(rect.Bottom() <= viewHeight)
	// ...and the scroll is clamped to the bottom of the content (the last line is visible), confirming we scrolled as
	// far as possible even though a strict top-alignment wasn't achievable.
	c.Equal(scroll.Bar(true).Max(), scroll.Bar(true).Value()+scroll.Bar(true).Extent())
}

func TestMarkdownAnchorsResetOnNewContent(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("# First\n", 400)
	c.True(m.ScrollToAnchor("#first"))
	// Replacing the content must clear stale anchors from the prior content.
	m.SetContent("# Second\n", 400)
	c.False(m.ScrollToAnchor("#first"))
	c.True(m.ScrollToAnchor("#second"))
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

// TestMarkdownRetrieveImageQueriesDisplayOnCallingGoroutine verifies that retrieveImage queries the primary display
// synchronously on the calling goroutine (the UI thread in production) rather than from the background image-loading
// goroutine it spawns. Querying displays off the UI thread is unsafe: on macOS the lookup calls into AppKit, which
// only permits access from the main thread, and the platform implementations rely on the UI thread for
// synchronization.
func TestMarkdownRetrieveImageQueriesDisplayOnCallingGoroutine(t *testing.T) {
	saved := markdownPrimaryDisplay
	defer func() { markdownPrimaryDisplay = saved }()
	entered := make(chan struct{})
	release := make(chan struct{})
	markdownPrimaryDisplay = func() *Display {
		close(entered)
		<-release
		return nil
	}
	m := NewMarkdown(false)
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.retrieveImage("missing-image-for-test.png", NewDrawablePanel())
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("retrieveImage never queried the display")
	}
	// retrieveImage waits at most one second for the image fetch before giving up and returning nil. If the display
	// query were still being made from the image-loading goroutine, retrieveImage would therefore return while the
	// blocked query keeps that goroutine stuck; when the query is made on the calling goroutine, retrieveImage cannot
	// return until the query has been released. Wait longer than that internal timeout to tell the two apart.
	select {
	case <-done:
		t.Fatal("retrieveImage returned while the display query was still blocked; it must query the display on the calling goroutine")
	case <-time.After(1500 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("retrieveImage did not return after the display query completed")
	}
}
