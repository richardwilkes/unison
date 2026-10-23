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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
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
		{name: "underscore emphasis", content: "some _italic_ words\n", want: "italic"},
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

// TestMarkdownStyledLinkLabels verifies that link labels whose text is nested inside styled inline nodes (emphasis,
// code spans, strikethrough, etc.) still surface their text. extractText must recurse into child nodes rather than
// only collecting the link node's direct Text children, or the rendered link ends up with an empty label.
func TestMarkdownStyledLinkLabels(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    string
	}{
		{name: "bold label", content: "[**bold label**](http://example.com)\n", want: "bold label"},
		{name: "italic label", content: "[*italic label*](http://example.com)\n", want: "italic label"},
		{name: "code label", content: "[`code label`](http://example.com)\n", want: "code label"},
		{name: "strikethrough label", content: "[~~struck label~~](http://example.com)\n", want: "struck label"},
		{name: "nested styles", content: "[**outer *inner* text**](http://example.com)\n", want: "outer inner text"},
		{name: "mixed plain and styled", content: "[pre **mid** post](http://example.com)\n", want: "pre mid post"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			m := NewMarkdown(false)
			m.SetContent(tc.content, 400)
			c.Contains(collectMarkdownText(m.AsPanel()), tc.want)
		})
	}
}

// TestMarkdownStyledImageAltText verifies that image alt text nested inside styled inline nodes still becomes the
// image's tooltip, exercising the same extractText recursion as styled link labels. The image target does not exist,
// so the load fails fast and the broken-image placeholder is used; the alt-text tooltip must be attached regardless.
func TestMarkdownStyledImageAltText(t *testing.T) {
	saved := markdownPrimaryDisplay
	defer func() { markdownPrimaryDisplay = saved }()
	markdownPrimaryDisplay = func() *Display { return nil }
	c := check.New(t)
	m := NewMarkdown(false)
	m.SetContent("![**styled alt text**](missing-image-for-test.png)\n", 400)
	var tip *Panel
	var walk func(p *Panel)
	walk = func(p *Panel) {
		if _, ok := p.Self.(*DrawablePanel); ok && p.Tooltip != nil {
			tip = p.Tooltip
		}
		for _, child := range p.Children() {
			walk(child)
		}
	}
	walk(m.AsPanel())
	c.NotNil(tip)
	if tip != nil {
		c.Contains(collectMarkdownText(tip), "styled alt text")
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
// the view (revealing the section that follows it) rather than leaving it pinned to the bottom of the view, even when
// the heading is taller than the view, so it isn't reduced to showing only its last "bit". A heading too near the end
// of the document to be pulled to the top is instead brought fully into view, the scroll clamped to the end.
func TestMarkdownScrollToAnchorAlignsTop(t *testing.T) {
	lines := func(n int, suffix string) string {
		var sb strings.Builder
		for i := range n {
			fmt.Fprintf(&sb, "line %d%s\n\n", i, suffix)
		}
		return sb.String()
	}
	for _, tc := range []struct {
		name       string
		content    string
		anchor     string // Given to ScrollToAnchor; slug is the key it resolves to.
		slug       string
		viewHeight float32
		taller     bool // The heading is taller than the view.
		nearEnd    bool // The content below the heading is too short for it to reach the top of the view.
	}{
		{
			name: "heading in the middle",
			content: "# Top\n\n" + lines(40, " of the top section") + "## Target Heading\n\n" +
				lines(40, " of the target section"),
			anchor:     "#Target-Heading",
			slug:       "target-heading",
			viewHeight: 300,
		},
		{
			name:       "heading taller than the view",
			content:    "# Top\n\n" + lines(40, "") + "## Target Heading\n\nbody\n",
			anchor:     "target-heading",
			slug:       "target-heading",
			viewHeight: 10,
			taller:     true,
		},
		{
			name:       "heading near the end",
			content:    "# Top\n\n" + lines(60, "") + "## Last\n\ntail\n",
			anchor:     "last",
			slug:       "last",
			viewHeight: 300,
			nearEnd:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			scroll, m := markdownInScrollPanel(tc.content, tc.viewHeight)
			p := m.anchors[tc.slug]
			c.NotNil(p)
			c.True(m.ScrollToAnchor(tc.anchor))
			rect := p.RectToRoot(p.ContentRect(true))
			if tc.nearEnd {
				c.True(rect.Y >= 0, "the heading is fully visible")
				bar := scroll.Bar(true)
				c.Equal(bar.Max(), bar.Value()+bar.Extent(), "the view is scrolled as far as the content allows")
			} else {
				c.Equal(float32(0), rect.Y, "the heading's top is brought to the top of the view")
			}
			if !tc.taller {
				c.True(rect.Bottom() <= tc.viewHeight, "the whole heading fits within the view")
			}
		})
	}
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

// TestReviseTarget verifies how targets are turned into links or absolute paths. An already-absolute local target used
// to be joined onto the working directory anyway, turning "/tmp/img.png" into "<workingDir>/tmp/img.png", which then
// failed to load and rendered as the broken-image placeholder.
func TestReviseTarget(t *testing.T) {
	c := check.New(t)
	workingDir := t.TempDir()

	// URLs pass through untouched.
	revised, err := ReviseTarget(workingDir, "https://example.com/a.png", nil)
	c.NoError(err)
	c.Equal("https://example.com/a.png", revised)

	// So do targets matching one of the alternate link prefixes.
	revised, err = ReviseTarget(workingDir, "mailto:someone@example.com", []string{"mailto:"})
	c.NoError(err)
	c.Equal("mailto:someone@example.com", revised)

	// A relative target is resolved against the working directory, after URL unescaping.
	revised, err = ReviseTarget(workingDir, "sub/img%20one.png", nil)
	c.NoError(err)
	c.Equal(filepath.Join(workingDir, "sub", "img one.png"), revised)

	// An absolute target is left where it is rather than being buried under the working directory.
	abs := filepath.Join(workingDir, "abs.png")
	revised, err = ReviseTarget(filepath.Join(workingDir, "elsewhere"), abs, nil)
	c.NoError(err)
	c.Equal(abs, revised)

	// The same holds when the absolute target arrives URL-escaped.
	absWithSpace := filepath.Join(workingDir, "abs two.png")
	revised, err = ReviseTarget(workingDir, filepath.ToSlash(filepath.Join(workingDir, "abs%20two.png")), nil)
	c.NoError(err)
	c.Equal(absWithSpace, revised)
}

// These tests cover the keyboard focus a Markdown takes for an assistive technology's sake — see Panel.axTakesFocus:
// that it takes it only while one is being served, and only on the platforms whose screen readers start from the focus;
// that a window in which nothing held the focus hands it to the document as support starts, while a focus that was
// already held is left alone; and that a keyboard user nothing is listening to never finds the document in the tab
// order. A session owns most of the package's mutable globals while it runs, so none of these may call t.Parallel.

// markdownFocusContent is the document the tests below show. What it says does not matter; that it is a Markdown does.
const markdownFocusContent = "# Title\n\nSome body text.\n"

// TestMarkdownTakesFocusOnlyWhileAccessibilityIsActive shows a window holding nothing but a document, which is the
// window a screen reader was stranded at the edge of: with nothing inside it holding the focus, Narrator's cursor
// stayed on the window itself. Once an assistive technology is being served, the document takes the focus and the
// description reports it there; once none is, the document is no longer focusable and the next move of the focus
// leaves it.
func TestMarkdownTakesFocusOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent(markdownFocusContent, 400)
			scroller := NewScrollPanel()
			scroller.SetContent(md, behavior.Fill, behavior.Fill)
			wnd = axNewTestWindow(t, "document focus", geom.NewRect(10, 10, 500, 500), scroller)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focusable := func() bool {
		var f bool
		screen.Do(func() { f = md.Focusable() })
		return f
	}
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	c.False(focusable(), "with no assistive technology, a document is not focusable")
	c.True(focus() == nil, "and a window holding nothing but one has no focus")

	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := screen.AccessibilityNodeFor(md)
	c.NotNil(node)
	if tree == nil || node == nil {
		return
	}
	if axReadersFollowFocus {
		c.True(focusable(), "while one is being served, the document takes the focus")
		c.True(focus().Is(md), "a window in which nothing held the focus hands it to the document as support starts")
		c.Equal(node.ID, tree.Focus, "and the description reports the focus there")
		c.True(node.Focusable, "the node says it can take the focus")
		c.True(node.Actions.Has(accessibility.Focus), "and offers to")
	} else {
		c.False(focusable(), "on a platform whose screen reader does not start from the focus, nothing changes")
		c.True(focus() == nil, "so the window still has no focus")
		c.Equal(accessibility.NodeID(0), tree.Focus, "and the description reports none")
		c.False(node.Focusable)
		c.False(node.Actions.Has(accessibility.Focus))
	}

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	c.False(focusable(), "once none is being served, the document is not focusable again")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus() == nil, "and the next move of the focus leaves it, with nothing else to land on")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownJoinsTabOrderOnlyWhileAccessibilityIsActive shows a document between two text fields. With no assistive
// technology, Tab moves from one field to the other and never lands on the document, as it never has. Once one is being
// served, on the platforms where a document takes the focus, the document is a tab stop between the two — while the
// field that held the focus as support started keeps it, since only a window with no focus at all is given one.
func TestMarkdownJoinsTabOrderOnlyWhileAccessibilityIsActive(t *testing.T) {
	c := check.New(t)
	var first, second *Field
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			first = NewField()
			md = NewMarkdown(false)
			md.SetContent(markdownFocusContent, 400)
			second = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(first)
			column.AddChild(md)
			column.AddChild(second)
			wnd = axNewTestWindow(t, "document tab order", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}
	focusFirst := func() {
		screen.Do(func() { first.RequestFocus() })
		c.True(focus().Is(first))
	}
	focusFirst()
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(second), "with no assistive technology, the tab order skips the document")

	focusFirst()
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.True(focus().Is(first), "support starting leaves a focus that was already held where it is")
	screen.KeyPress(KeyTab, mod.None)
	if axReadersFollowFocus {
		c.True(focus().Is(md), "while one is being served, the document is a tab stop")
		screen.KeyPress(KeyTab, mod.None)
	}
	c.True(focus().Is(second), "and the field after it is the next stop")

	screen.Do(deactivateAccessibility)
	c.False(IsAccessibilityActive())
	focusFirst()
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(second), "once none is being served, the tab order skips the document again")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownFailedImageLoadIsRetried verifies that a failed image load does not permanently poison the drawable
// cache: the failed entry must be removed so a later rebuild retries the load (and does not keep appending dead
// placeholder panels to the stale entry's target list). The load is then made to succeed at the same path to prove
// the retry actually happens.
func TestMarkdownFailedImageLoadIsRetried(t *testing.T) {
	c := check.New(t)
	m := NewMarkdown(false)
	dir := t.TempDir()
	m.WorkingDirProvider = func(_ Paneler) string { return dir }
	const target = "image.svg"

	// First attempt: the file does not exist, so the load fails and must not remain cached.
	c.Nil(m.retrieveImage(target, NewDrawablePanel()))
	deadline := time.Now().Add(10 * time.Second)
	for {
		m.drawableCacheLock.Lock()
		remaining := len(m.drawableCache)
		m.drawableCacheLock.Unlock()
		if remaining == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the failed image load was never removed from the drawable cache")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Second attempt: the file now exists, so the retry must succeed rather than being served the poisoned entry.
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	c.NoError(os.WriteFile(filepath.Join(dir, target), []byte(svg), 0o600))
	c.NotNil(m.retrieveImage(target, NewDrawablePanel()))
}
