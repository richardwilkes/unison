// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests read a rendered markdown document the way a screen reader does: they ask for the one stream of text it
// composes, the text of each block within it, and what happens to the reading caret as the keys and the mouse move it.
// A session owns most of the package's mutable globals while it runs, so none of them may call t.Parallel.

// markdownAXContent is the document the tests below read. It holds one of everything a person moves through: a heading,
// a paragraph long enough to wrap with a link, bold words and a code span in it, an image, an ordered list whose
// numbers are of different widths, an unordered list, a code block of two lines, an alert quote, a table with a header
// row, and a rule across the page.
const markdownAXContent = "# Title\n\n" +
	"Body text with a [Docs](https://example.com/docs) link, **bold** words and a `code` span that is long " +
	"enough to wrap onto a second line of its own.\n\n" +
	"![A cat](missing-image-for-test.png)\n\n" +
	"9. ninth item\n10. tenth item\n\n" +
	"- alpha\n- beta\n\n" +
	"```go\nfmt.Println()\nmore()\n```\n\n" +
	"> [!NOTE]\n> Take care.\n\n" +
	"| H1 | H2 |\n| -- | -- |\n| a | b |\n\n" +
	"---\n"

// axMarkdownFixture is a markdown being read: the session showing it, the window it is in, the scroll panel it is read
// through, and wherever a link within it was followed to.
type axMarkdownFixture struct {
	screen   *unison.HeadlessScreen
	wnd      *unison.Window
	markdown *unison.Markdown
	scroller *unison.ScrollPanel
	extra    *unison.Field
	followed string
}

// newAXMarkdownFixture shows content in a window, with the document focusable and holding the focus, which is what an
// application that wants its markdown read arranges for and what a screen reader on Windows or Linux is given
// automatically. The window also holds a field, so that a test has somewhere else to put the focus.
func newAXMarkdownFixture(t *testing.T, content string, contentWidth float32, windowSize geom.Size) *axMarkdownFixture {
	t.Helper()
	f := &axMarkdownFixture{}
	f.screen = startHeadless(t, unison.HeadlessConfig{Width: 1000, Height: 1000},
		unison.StartupFinishedCallback(func() {
			f.markdown = unison.NewMarkdown(false)
			f.markdown.LinkHandler = func(_ unison.Paneler, target string) { f.followed = target }
			// What an application that wants its markdown read does, and what a screen reader on the platforms whose
			// readers start from the focus is given without asking. See Panel.axTakesFocus.
			f.markdown.SetFocusable(true)
			f.markdown.SetContent(content, contentWidth)
			// Read through a view shorter than the document, since a document taller than the view showing it is what
			// the scrolling a screen reader asks for is about.
			f.scroller = axScroller(f.markdown, geom.NewSize(windowSize.Width-30, windowSize.Height-60))
			f.extra = unison.NewField()
			f.wnd = newHeadlessWindow(t, "markdown", geom.NewRect(10, 10, windowSize.Width, windowSize.Height),
				axColumn(f.scroller, f.extra))
			if f.wnd != nil {
				f.wnd.ToFront()
				f.wnd.SetFocus(f.markdown)
			}
		}))
	if f.wnd == nil {
		t.Fatal("the window holding the markdown was not created")
	}
	f.screen.Sync()
	return f
}

// document returns the node describing the markdown, which is the document, after describing the window afresh.
func (f *axMarkdownFixture) document(c check.Checker) (*accessibility.Tree, *accessibility.Node) {
	c.Helper()
	tree := f.screen.AccessibilityTree(f.wnd)
	c.NotNil(tree)
	node := axMustNode(c, f.screen.AccessibilityNodeFor(f.markdown))
	if node.Document == nil {
		c.Fatal("the markdown must carry the stream of text it composed")
	}
	return tree, node
}

// stream returns the text the document composed.
func (f *axMarkdownFixture) stream(c check.Checker) string {
	c.Helper()
	_, node := f.document(c)
	return node.Document.Text.Text
}

// caret returns where the reading caret and the selection are within the stream.
func (f *axMarkdownFixture) caret(c check.Checker) (selStart, selEnd, caret int) {
	c.Helper()
	_, node := f.document(c)
	info := node.Document.Text
	return info.SelStart, info.SelEnd, info.Caret
}

// pointAt returns the point on the screen at the given offset of the stream, which is where a test aims the mouse to
// put the caret there.
func (f *axMarkdownFixture) pointAt(c check.Checker, offset int) geom.Point {
	c.Helper()
	_, node := f.document(c)
	lines := node.Document.Text.Lines
	for _, line := range lines {
		if offset < line.Start || offset > line.End || len(line.Advances) == 0 {
			continue
		}
		i := min(max(offset-line.Start, 0), len(line.Advances)-1)
		return f.screen.PanelPoint(f.markdown,
			geom.NewPoint(line.Bounds.X+line.Advances[i]+1, line.Bounds.CenterY()))
	}
	c.Fatal("no line holds offset ", offset)
	return geom.Point{}
}

// pointInLineAt returns a point on the screen within the line that owns an offset of the stream, at that offset's own
// rune boundary. A line owns the offsets from its own start up to the start of the next, which is what tells the end of
// one line apart from the beginning of the following one — and what a table needs, where every cell of a row is drawn
// in the same vertical band and f.pointAt would answer with whichever of them holds the offset as a boundary.
func (f *axMarkdownFixture) pointInLineAt(c check.Checker, offset int) geom.Point {
	c.Helper()
	_, node := f.document(c)
	lines := node.Document.Text.Lines
	index := -1
	for i, line := range lines {
		if line.Start <= offset && len(line.Advances) != 0 {
			index = i
		}
	}
	if index < 0 {
		c.Fatal("no line owns offset ", offset)
		return geom.Point{}
	}
	line := lines[index]
	at := min(max(offset-line.Start, 0), len(line.Advances)-1)
	return f.screen.PanelPoint(f.markdown, geom.NewPoint(line.Bounds.X+line.Advances[at]+1, line.Bounds.CenterY()))
}

// axTextNodesUnder returns every node beneath one that carries text, in reading order, which beneath a document is
// every block it is read as.
func axTextNodesUnder(tree *accessibility.Tree, root *accessibility.Node) []*accessibility.Node {
	var nodes []*accessibility.Node
	var walk func(n *accessibility.Node)
	walk = func(n *accessibility.Node) {
		for _, child := range axChildNodes(tree, n) {
			if child.Text != nil {
				nodes = append(nodes, child)
			}
			walk(child)
		}
	}
	walk(root)
	return nodes
}

// axNodeWithText returns the first node beneath root whose text is exactly the given text, or nil if there is none.
func axNodeWithText(tree *accessibility.Tree, root *accessibility.Node, text string) *accessibility.Node {
	for _, node := range axTextNodesUnder(tree, root) {
		if node.Text.Text == text {
			return node
		}
	}
	return nil
}

// TestMarkdownDocumentStreamJoinsBlocks verifies that a document composes the content of everything within it into the
// one stream of text a screen reader reads: the blocks joined by line feeds, a placeholder where an image sits, a list
// item's bullet read with the words beside it, the lines of a code block kept apart, and no line feed left dangling at
// the end. The stream is carried apart from the node's own text, and the value is left empty, so that an adapter has to
// opt into presenting it rather than reading both it and the elements describing the same content.
func TestMarkdownDocumentStreamJoinsBlocks(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	_, node := f.document(c)
	c.Equal(role.Document, node.Role)
	c.Nil(node.Text, "a document's stream is carried apart from its own text")
	c.Equal("", node.Value, "and is not its value either")
	c.True(node.Document.Text.Multiline)
	text := node.Document.Text.Text
	for _, want := range []string{
		"Title\nBody text with a Docs link",
		"bold words and a code span",
		"span that is long enough to wrap",
		"￼",
		"9. ninth item\n10. tenth item",
		"• alpha\n• beta",
		"fmt.Println()\nmore()",
		"Note\nTake care.",
		"H1\nH2\na\nb",
	} {
		c.True(strings.Contains(text, want), "the stream should hold %q, but was %q", want, text)
	}
	c.False(strings.HasSuffix(text, "\n"), "nothing follows the last block, so nothing joins it to anything")
	c.True(node.Actions.Has(accessibility.SetTextSelection), "the caret can be moved anywhere in the stream")
	c.True(node.Actions.Has(accessibility.ScrollRangeIntoView), "and any part of it can be brought into view")
	c.False(node.Actions.Has(accessibility.ShowContextMenu), "a document has no menu of its own to offer")
	c.False(node.Actions.Has(accessibility.Press), "pressing a document is not activating it")
	c.True(node.ReadOnly, "a document is as unwritable as every block within it")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownDocumentSpansCoverEveryDescendant verifies that every element within a document says which part of the
// stream it occupies, that each of them names a node that is really there, that a block's span holds exactly the
// block's own text, and that the spans arrive outermost first so that an adapter can tell which element is the
// innermost one at an offset without sorting them itself.
func TestMarkdownDocumentSpansCoverEveryDescendant(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, node := f.document(c)
	stream := []rune(node.Document.Text.Text)
	spans := node.Document.Text.Spans
	c.True(len(spans) > 0)
	byNode := make(map[accessibility.NodeID]accessibility.TextSpan, len(spans))
	for _, span := range spans {
		c.True(span.Start >= 0 && span.End <= len(stream) && span.Start < span.End,
			"the span [%d,%d) must lie within the stream of %d runes", span.Start, span.End, len(stream))
		c.NotNil(tree.Node(span.Node), "a span must name a node that is in the tree")
		byNode[span.Node] = span
	}
	for i := range spans {
		for j := i + 1; j < len(spans); j++ {
			outer := spans[j].Start <= spans[i].Start && spans[j].End >= spans[i].End &&
				(spans[j].Start < spans[i].Start || spans[j].End > spans[i].End)
			c.False(outer, "span %d contains span %d, so it should have come first", j, i)
		}
	}
	for _, block := range axTextNodesUnder(tree, node) {
		span, ok := byNode[block.ID]
		c.True(ok, "the block holding %q should occupy part of the stream", block.Text.Text)
		if !ok {
			continue
		}
		c.Equal(block.Text.Text, string(stream[span.Start:span.End]),
			"a block's span should hold exactly the block's own text")
	}
	for _, r := range []role.Enum{role.List, role.ListItem, role.BlockQuote, role.Table, role.Row, role.Link} {
		nodes := axNodesWithRole(tree, r)
		c.True(len(nodes) > 0, "the fixture should hold at least one %v", r)
		for _, one := range nodes {
			_, ok := byNode[one.ID]
			c.True(ok, "the %v named %q should occupy part of the stream", r, one.Name)
		}
	}
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownBlockLinesTile verifies what each block says about its own text: the lines cover it end to end, there is
// an offset for every rune boundary on each of them, the space a wrapped line was broken at is back in the text, and
// every line sits where the block actually drew it.
func TestMarkdownBlockLinesTile(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, document := f.document(c)
	blocks := axTextNodesUnder(tree, document)
	c.True(len(blocks) > 1)
	wrapped := 0
	for _, block := range blocks {
		c.True(block.ReadOnly, "a document is read rather than written: %q", block.Text.Text)
		runes := []rune(block.Text.Text)
		lines := block.Text.Lines
		c.True(len(lines) > 0, "a block always drew at least one line: %q", block.Text.Text)
		for i, line := range lines {
			axCheckLine(c, line)
			if i == 0 {
				c.Equal(0, line.Start, "the first line begins at the beginning of the text")
			} else {
				c.Equal(lines[i-1].End, line.Start, "the lines together cover the whole of the text")
			}
			area := line.Bounds
			area.Point = area.Point.Add(block.Bounds.Point)
			c.True(document.Bounds.Union(area) == document.Bounds,
				"the line %v of %q should sit within the document at %v", area, block.Text.Text, document.Bounds)
		}
		c.Equal(len(runes), lines[len(lines)-1].End, "the last line ends at the end of the text")
		c.Equal(len(lines) > 1, block.Text.Multiline, "a block that drew more than one line says so")
		if len(lines) > 1 && !strings.Contains(block.Text.Text, "\n") {
			// The paragraph that had to be broken to fit: the space it was broken at is part of what was written, so it
			// is back in the text, and it is on the line that ended with it.
			wrapped++
			for i, line := range lines[:len(lines)-1] {
				c.Equal(' ', runes[line.End-1], "line %d of %q should end with the space it was broken at", i,
					block.Text.Text)
			}
		}
	}
	c.True(wrapped > 0, "the fixture should hold a paragraph that had to be broken to fit")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownRunsAndSpans verifies the styles a block reports: the runs tile its text, the bold words and the code
// span are runs of their own, and the link within it occupies the words it was written as and says where it leads.
func TestMarkdownRunsAndSpans(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, document := f.document(c)
	link := axMustNode(c, axNamed(tree, "Docs"))
	var paragraph *accessibility.Node
	for _, block := range axTextNodesUnder(tree, document) {
		if block.Role == role.Paragraph && strings.Contains(block.Text.Text, "bold words") {
			paragraph = block
		}
	}
	paragraph = axMustNode(c, paragraph, "the paragraph holding the styled words")
	runes := []rune(paragraph.Text.Text)
	runs := paragraph.Text.Runs
	c.True(len(runs) > 2, "the paragraph is drawn in more than one style")
	c.Equal(0, runs[0].Start)
	c.Equal(len(runes), runs[len(runs)-1].End, "the runs tile the text")
	for i, run := range runs {
		if i != 0 {
			c.Equal(runs[i-1].End, run.Start, "run %d begins where the one before it ends", i)
		}
		c.True(run.Weight >= 100 && run.Weight <= 900, "a weight is on the 100 to 900 scale, got %d", run.Weight)
	}
	runAt := func(text, within string) accessibility.TextRun {
		c.Helper()
		at := strings.Index(within, text)
		c.True(at >= 0, "%q should hold %q", within, text)
		offset := len([]rune(within[:at]))
		for _, run := range runs {
			if offset >= run.Start && offset < run.End {
				return run
			}
		}
		c.Fatal("no run covers ", text)
		return accessibility.TextRun{}
	}
	bold := runAt("bold", paragraph.Text.Text)
	c.True(bold.Weight >= 600, "the bold words are drawn bold, got weight %d", bold.Weight)
	c.False(bold.Monospace)
	code := runAt("code", paragraph.Text.Text)
	c.True(code.Monospace, "a code span is drawn in a fixed-pitch face, which is what marks it out")
	plain := runAt("Body", paragraph.Text.Text)
	c.True(plain.Weight < 600 && !plain.Monospace && plain.Family != "", "%v", plain)

	var linkSpan accessibility.TextSpan
	for _, span := range paragraph.Text.Spans {
		if span.Node == link.ID {
			linkSpan = span
		}
	}
	c.Equal(link.ID, linkSpan.Node, "the link should occupy part of the paragraph's text")
	c.Equal("Docs", string(runes[linkSpan.Start:linkSpan.End]))
	var streamSpan accessibility.TextSpan
	for _, span := range document.Document.Text.Spans {
		if span.Node == link.ID {
			streamSpan = span
		}
	}
	c.Equal(link.ID, streamSpan.Node, "the link should occupy part of the document's stream as well")
	stream := []rune(document.Document.Text.Text)
	c.Equal("Docs", string(stream[streamSpan.Start:streamSpan.End]), "and the same words of the stream")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownLinkURL verifies that a link within a document says where it leads as a property of its own, which is
// what an assistive technology offers among the document's links and reads out as the person moves onto it.
func TestMarkdownLinkURL(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, _ := f.document(c)
	link := axMustNode(c, axNamed(tree, "Docs"))
	c.Equal(role.Link, link.Role)
	c.Equal("https://example.com/docs", link.URL)
	c.Equal("https://example.com/docs", link.Description, "where it leads is worth hearing as well")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownReadingCaretKeys verifies the reading caret: the arrow keys move it by character, by word and by line,
// the page keys by the height of the view it is read through, Home and End go to the ends of a line and the command key
// with them to the ends of the document, shift extends the selection to wherever it goes, a move without shift
// collapses a selection to the end it is moving towards, the block holding the caret reports it as its own, a block the
// caret has left goes on reporting where it last was rather than claiming one it does not have, and every move tells an
// assistive technology the selection changed.
func TestMarkdownReadingCaretKeys(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, node := f.document(c)
	stream := node.Document.Text.Text
	heading := axMustNode(c, axNodeWithText(tree, node, "Title"))
	selStart, selEnd, caret := f.caret(c)
	c.Equal([]int{0, 0, 0}, []int{selStart, selEnd, caret}, "the caret starts at the top of the document")

	f.screen.KeyPress(unison.KeyRight, mod.None)
	_, _, caret = f.caret(c)
	c.Equal(1, caret, "the right arrow moves the caret one rune")
	c.Equal(1, axMustNode(c, f.screen.AccessibilityNodeFor(f.markdown)).Document.Text.Caret)

	f.screen.KeyPress(unison.KeyEnd, mod.None)
	_, _, caret = f.caret(c)
	c.Equal(len("Title"), caret, "End goes to the end of the line the caret is on")
	tree, _ = f.document(c)
	heading = axMustNode(c, tree.Node(heading.ID))
	c.Equal(len("Title"), heading.Text.Caret, "the block holding the caret reports it as its own offset")
	c.Equal(len("Title"), heading.Text.SelEnd)

	f.screen.AccessibilityEvents(f.wnd)
	f.screen.KeyPress(unison.KeyDown, mod.None)
	_, _, caret = f.caret(c)
	c.True(caret > len("Title"), "the down arrow moves the caret onto the next line, got %d", caret)
	events := f.screen.AccessibilityEvents(f.wnd)
	moved := false
	for _, event := range events {
		if event.Kind == accessibility.TextSelectionChanged && event.Node == node.ID {
			moved = true
		}
	}
	c.True(moved, "the document should have reported that its selection moved: %v", events)
	tree, _ = f.document(c)
	heading = axMustNode(c, tree.Node(heading.ID))
	c.Equal(len("Title"), heading.Text.Caret,
		"the block the caret has left goes on reporting where it last was, so nothing is said about it")

	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	selStart, selEnd, caret = f.caret(c)
	c.Equal(1, selEnd-selStart, "shift and an arrow key extends the selection by a rune")
	c.Equal(selEnd, caret, "the caret is the end that moved")

	f.screen.KeyPress(unison.KeyEnd, mod.OSMenuCommand())
	_, _, caret = f.caret(c)
	c.Equal(len([]rune(stream)), caret, "the command key with End goes to the end of the document")
	f.screen.KeyPress(unison.KeyHome, mod.OSMenuCommand())
	_, _, caret = f.caret(c)
	c.Equal(0, caret, "and with Home to the beginning of it")

	f.screen.KeyPress(unison.KeyRight, mod.None)
	f.screen.KeyPress(unison.KeyRight, mod.None)
	f.screen.KeyPress(unison.KeyLeft, mod.None)
	_, _, caret = f.caret(c)
	c.Equal(1, caret, "the left arrow moves the caret back one rune")

	// A selection collapses to the end the caret is moving towards rather than the caret stepping out of it, which is
	// what every text control does.
	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	f.screen.KeyPress(unison.KeyLeft, mod.None)
	selStart, selEnd, caret = f.caret(c)
	c.Equal([]int{1, 1, 1}, []int{selStart, selEnd, caret},
		"the left arrow collapses a selection to the end it is moving towards")
	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	f.screen.KeyPress(unison.KeyRight, mod.None)
	selStart, selEnd, caret = f.caret(c)
	c.Equal([]int{3, 3, 3}, []int{selStart, selEnd, caret}, "and the right arrow to the other end of it")

	f.screen.KeyPress(unison.KeyHome, mod.None)
	_, _, caret = f.caret(c)
	c.Equal(0, caret, "Home on its own goes to the beginning of the line the caret is on")

	f.screen.KeyPress(unison.KeyDown, mod.None)
	_, _, below := f.caret(c)
	c.True(below > len("Title"), "the down arrow moves onto the line below, got %d", below)
	f.screen.KeyPress(unison.KeyUp, mod.None)
	_, _, caret = f.caret(c)
	c.True(caret <= len("Title"), "and the up arrow back onto the line above it, got %d", caret)
	f.screen.KeyPress(unison.KeyUp, mod.None)
	_, _, caret = f.caret(c)
	c.Equal(0, caret, "with nothing above it, the up arrow goes to the beginning of the document")

	// The option key with an arrow moves by word on every platform; the control key does so where the menu commands are
	// on the control key rather than on the command key.
	f.screen.KeyPress(unison.KeyRight, mod.Option)
	_, _, caret = f.caret(c)
	c.Equal(len("Title\n"), caret, "the option key with an arrow moves to the beginning of the next word")
	f.screen.KeyPress(unison.KeyLeft, mod.Option)
	_, _, caret = f.caret(c)
	c.Equal(0, caret, "and back to the beginning of the word before it")
	if mod.OSMenuCommand() == mod.Control {
		f.screen.KeyPress(unison.KeyRight, mod.Control)
		_, _, caret = f.caret(c)
		c.Equal(len("Title\n"), caret, "as does the control key with one, where that is the convention")
		f.screen.KeyPress(unison.KeyLeft, mod.Control)
		_, _, caret = f.caret(c)
		c.Equal(0, caret)
	}

	// A page is the height of the view the document is read through, which is shorter than the document.
	f.screen.KeyPress(unison.KeyPageDown, mod.None)
	_, _, paged := f.caret(c)
	c.True(paged > 0, "page down moves the caret a view's height down the document, got %d", paged)
	f.screen.KeyPress(unison.KeyPageUp, mod.None)
	_, _, caret = f.caret(c)
	c.True(caret < paged, "and page up brings it back up again, got %d", caret)

	f.screen.KeyPress(unison.KeyTab, mod.None)
	var focused bool
	f.screen.Do(func() { focused = f.markdown.Focused() })
	c.False(focused, "Tab is left alone, so it still moves the focus out of the document")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownCaretActivatesLink verifies that Return, the numeric keypad's enter key and Space follow the link the
// caret is on, and that with the caret anywhere else they are left to whatever holds the document, where Space still
// scrolls.
func TestMarkdownCaretActivatesLink(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	_, node := f.document(c)
	at := strings.Index(node.Document.Text.Text, "Docs")
	c.True(at >= 0)
	offset := len([]rune(node.Document.Text.Text[:at])) + 1
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  offset,
		End:    offset,
	}))
	f.screen.KeyPress(unison.KeyReturn, mod.None)
	var followed string
	f.screen.Do(func() { followed = f.followed })
	c.Equal("https://example.com/docs", followed, "Return on a link follows it")

	f.screen.Do(func() { f.followed = "" })
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  0,
		End:    0,
	}))
	f.screen.KeyPress(unison.KeySpace, mod.None)
	f.screen.Do(func() { followed = f.followed })
	c.Equal("", followed, "with the caret anywhere else there is no link to follow")

	// The enter key of the numeric keypad is the same key as Return to anybody using one.
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  offset,
		End:    offset,
	}))
	f.screen.KeyPress(unison.KeyNumPadEnter, mod.None)
	f.screen.Do(func() { followed = f.followed })
	c.Equal("https://example.com/docs", followed, "as does the enter key of the numeric keypad")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownSetTextSelectionActions verifies that an assistive technology can move the caret and the selection from
// either end: the document takes offsets into the whole stream, and a block takes its own, which is what a screen
// reader asking the element it is reading from hands over.
func TestMarkdownSetTextSelectionActions(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, node := f.document(c)
	stream := []rune(node.Document.Text.Text)
	quote := axMustNode(c, axNodeWithText(tree, node, "Take care."))
	var quoteSpan accessibility.TextSpan
	for _, span := range node.Document.Text.Spans {
		if span.Node == quote.ID {
			quoteSpan = span
		}
	}
	c.Equal(quote.ID, quoteSpan.Node)

	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  quoteSpan.Start + 1,
		End:    quoteSpan.Start + 5,
	}))
	selStart, selEnd, caret := f.caret(c)
	c.Equal(quoteSpan.Start+1, selStart)
	c.Equal(quoteSpan.Start+5, selEnd)
	c.Equal(selEnd, caret)
	tree, _ = f.document(c)
	quote = axMustNode(c, tree.Node(quote.ID))
	c.Equal(1, quote.Text.SelStart, "the block reports the selection as its own offsets")
	c.Equal(5, quote.Text.SelEnd)
	c.Equal("ake ", string(stream[quoteSpan.Start+1:quoteSpan.Start+5]))

	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   quote.ID,
		Action: accessibility.SetTextSelection,
		Start:  0,
		End:    4,
	}))
	selStart, selEnd, _ = f.caret(c)
	c.Equal(quoteSpan.Start, selStart, "a request aimed at a block moves the document's one caret")
	c.Equal(quoteSpan.Start+4, selEnd)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownScrollRangeIntoView verifies that a range of the stream, and a range of one block's text, can be brought
// into view, which is what a screen reader asks for as it reads a document taller than the view showing it.
func TestMarkdownScrollRangeIntoView(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(400, 160))
	_, node := f.document(c)
	position := func() float32 {
		var v float32
		f.screen.Do(func() { _, v = f.scroller.Position() })
		return v
	}
	c.Equal(float32(0), position(), "the document begins at the top")
	stream := []rune(node.Document.Text.Text)
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ScrollRangeIntoView,
		Start:  len(stream) - 1,
		End:    len(stream),
	}))
	scrolled := position()
	c.True(scrolled > 0, "the end of the document should have been brought into view, position %v", scrolled)

	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ScrollRangeIntoView,
		Start:  0,
		End:    1,
	}))
	c.Equal(float32(0), position(), "and the beginning of it again")

	tree, _ := f.document(c)
	cells := axNodesWithRole(tree, role.Cell)
	c.True(len(cells) > 0)
	last := cells[len(cells)-1]
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   last.ID,
		Action: accessibility.ScrollRangeIntoView,
		Start:  0,
		End:    1,
	}))
	c.True(position() > 0, "a range of one block's own text is brought into view the same way")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownListAccessibility verifies that the lists of a document are lists: each item is one element that knows
// which of how many it is, the bullets are read as part of the items rather than announced on their own, and every
// bullet is given the same width so that the numbers of an ordered list still line up.
func TestMarkdownListAccessibility(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, document := f.document(c)
	lists := axNodesWithRole(tree, role.List)
	c.Equal(2, len(lists), "the fixture holds an ordered and an unordered list")
	for _, list := range lists {
		c.Equal(0, list.RowCount, "a static list has no rows; it has items")
		items := axChildNodes(tree, list)
		c.Equal(2, len(items))
		for i, item := range items {
			c.Equal(role.ListItem, item.Role)
			pos, size := tree.PositionInSet(item.ID)
			c.Equal(i+1, pos, "an item says which of the list it is")
			c.Equal(2, size)
		}
	}
	c.True(axNamed(tree, "9.") == nil, "a bullet is read with the words beside it rather than announced on its own")
	c.True(axNamed(tree, "•") == nil)

	// The numbers of the ordered list are of different widths, so the content beside them lines up only if every bullet
	// was given the width of the widest.
	ninth := axMustNode(c, axNodeWithText(tree, document, "ninth item"))
	tenth := axMustNode(c, axNodeWithText(tree, document, "tenth item"))
	c.Equal(ninth.Bounds.X, tenth.Bounds.X, "the content of every item begins at the same place")

	c.True(strings.Contains(document.Document.Text.Text, "9. ninth item\n10. tenth item"),
		"a bullet is read as part of the line its item is on: %q", document.Document.Text.Text)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownTableAccessibility verifies that the table of a document is a table: rows a person moves through, cells
// that know which row and column they are in, a header row of column headers, a row whose area covers the cells within
// it, and a row that can be brought into view although it has no panel of its own.
func TestMarkdownTableAccessibility(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(400, 160))
	tree, _ := f.document(c)
	tables := axNodesWithRole(tree, role.Table)
	c.Equal(1, len(tables))
	table := tables[0]
	c.Equal(2, table.RowCount)
	c.Equal(2, table.ColumnCount)
	rows := axChildNodes(tree, table)
	c.Equal(2, len(rows), "the cells are described beneath the rows they belong to, and only once")
	for r, row := range rows {
		c.Equal(role.Row, row.Role)
		c.Equal(r, row.RowIndex)
		c.True(row.Actions.Has(accessibility.ScrollIntoView))
		cells := axChildNodes(tree, row)
		c.Equal(2, len(cells))
		var area geom.Rect
		for col, cell := range cells {
			if r == 0 {
				c.Equal(role.ColumnHeader, cell.Role, "the first row of this table is its header")
			} else {
				c.Equal(role.Cell, cell.Role)
			}
			c.Equal(r, cell.RowIndex)
			c.Equal(col, cell.ColumnIndex)
			c.True(cell.Text != nil, "a cell is read as text")
			if col == 0 {
				area = cell.Bounds
			} else {
				area = area.Union(cell.Bounds)
			}
		}
		c.Equal(area, row.Bounds, "a row covers the cells within it")
	}
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   rows[1].ID,
		Action: accessibility.ScrollIntoView,
	}), "a row with no panel of its own can still be brought into view")
	var position float32
	f.screen.Do(func() { _, position = f.scroller.Position() })
	c.True(position > 0, "which scrolled the table's last row into the view")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownCodeAndBlockquoteAccessibility verifies that a code block is read as the code it is — one element whose
// lines are kept apart and whose text is fixed-pitch — and that an alert quote is a quoted passage named by the kind of
// alert it is, with its title read where it is drawn rather than announced as a label of its own.
func TestMarkdownCodeAndBlockquoteAccessibility(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, _ := f.document(c)
	codes := axNodesWithRole(tree, role.Code)
	c.Equal(1, len(codes))
	code := codes[0]
	c.Equal("fmt.Println()\nmore()", code.Text.Text, "the lines of a code block are the lines that were written")
	c.True(code.Text.Multiline)
	c.True(code.ReadOnly)
	c.Equal(2, len(code.Text.Lines))
	for _, run := range code.Text.Runs {
		c.True(run.Monospace, "code is drawn in a fixed-pitch face")
	}
	c.Equal(0, len(axChildNodes(tree, code)), "a code block is one element rather than a label per line")

	quotes := axNodesWithRole(tree, role.BlockQuote)
	c.Equal(1, len(quotes))
	quote := quotes[0]
	c.Equal("Note", quote.Name, "the kind of alert is what the quote is called")
	c.True(quote.Text == nil, "a quote holds blocks rather than text of its own")
	children := axChildNodes(tree, quote)
	c.Equal(1, len(children), "the title is read where it is drawn rather than described as a label")
	c.Equal(role.Paragraph, children[0].Role)
	c.Equal("Take care.", children[0].Text.Text)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownCaretSurvivesRebuild verifies what becomes of the reading caret when the content is set again: the same
// content laid out to a different width is the same document, so the caret stays where it was in it, while different
// content is a different document and the caret goes back to the top of it.
func TestMarkdownCaretSurvivesRebuild(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	_, node := f.document(c)
	at := strings.Index(node.Document.Text.Text, "Take care.")
	c.True(at >= 0)
	offset := len([]rune(node.Document.Text.Text[:at]))
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  offset,
		End:    offset,
	}))

	f.screen.Do(func() { f.markdown.SetContent(markdownAXContent, 260) })
	_, _, caret := f.caret(c)
	c.Equal(offset, caret, "the same content at another width is the same document, read from where it was left")

	f.screen.Do(func() { f.markdown.SetContent("# Something else\n\nA new document.\n", 260) })
	selStart, selEnd, caret := f.caret(c)
	c.Equal([]int{0, 0, 0}, []int{selStart, selEnd, caret}, "different content is read from the top")
	c.Equal("Something else\nA new document.", f.stream(c))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownCaretDrawsOnlyWhenFocused verifies the user-visible half of the reading caret: while the document holds
// the focus the caret and whatever it has selected are drawn, and while it does not, nothing is, so a markdown nobody
// is reading looks exactly as it always did.
//
// The focus is moved to nowhere rather than to the field beside the document, and nothing is published in between. A
// window is repainted as a whole, so a field taking the focus, or a description being published for an assistive
// technology, would repaint the document along with everything else and hide a document that never asked to be
// repainted for a focus it draws differently for. See Markdown.DefaultFocusGained and Window.axMarkForPublish.
func TestMarkdownCaretDrawsOnlyWhenFocused(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	f.screen.Do(func() { f.wnd.SetFocus(nil) })
	f.screen.Sync()
	unfocused := f.screen.CaptureWindow(f.wnd)
	c.NotNil(unfocused)

	f.screen.Do(func() { f.wnd.SetFocus(f.markdown) })
	f.screen.Sync()
	withCaret := f.screen.CaptureWindow(f.wnd)
	c.NotNil(withCaret)
	if unfocused == nil || withCaret == nil {
		return
	}
	c.False(slices.Equal(unfocused.Pix, withCaret.Pix),
		"the caret is drawn as the document takes the focus, with nothing else having repainted for it")

	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	f.screen.KeyPress(unison.KeyRight, mod.Shift)
	f.screen.Sync()
	focused := f.screen.CaptureWindow(f.wnd)
	c.NotNil(focused)
	if focused == nil {
		return
	}
	c.False(slices.Equal(unfocused.Pix, focused.Pix), "the caret and the selection it has made are drawn")

	f.screen.Do(func() { f.wnd.SetFocus(nil) })
	f.screen.Sync()
	again := f.screen.CaptureWindow(f.wnd)
	c.NotNil(again)
	if again != nil {
		c.True(slices.Equal(unfocused.Pix, again.Pix),
			"and both are gone again the moment the focus leaves, with nothing else having repainted either")
	}
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownMouseCaretAndSelection verifies the mouse: a click places the caret, a shift-click extends the selection
// to it, a double-click takes the word, a triple-click the block, a drag the range it covers, and a click on a link
// still follows the link. None of it happens while the document is not focusable, which is every markdown nothing is
// reading.
func TestMarkdownMouseCaretAndSelection(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	_, node := f.document(c)
	stream := node.Document.Text.Text
	at := func(text string) int {
		c.Helper()
		index := strings.Index(stream, text)
		c.True(index >= 0, "the stream should hold %q", text)
		return len([]rune(stream[:index]))
	}
	words := at("words and a")

	f.screen.Click(f.pointAt(c, words))
	selStart, selEnd, caret := f.caret(c)
	c.Equal(words, caret, "a click places the caret where it was clicked")
	c.Equal(caret, selStart)
	c.Equal(caret, selEnd)

	f.screen.ClickWith(f.pointAt(c, words+5), unison.ButtonLeft, mod.Shift)
	selStart, selEnd, caret = f.caret(c)
	c.Equal(words, selStart, "a shift-click extends the selection from where it was made")
	c.Equal(words+5, selEnd)
	c.Equal(selEnd, caret)

	f.screen.DoubleClick(f.pointAt(c, words+1))
	selStart, selEnd, _ = f.caret(c)
	c.Equal("words", string([]rune(stream)[selStart:selEnd]), "a double-click takes the word")

	f.screen.Drag(f.pointAt(c, words), f.pointAt(c, words+9), 3)
	selStart, selEnd, caret = f.caret(c)
	c.Equal(words, selStart, "a drag selects what it covers")
	c.Equal(words+9, selEnd)
	c.Equal(selEnd, caret)

	link := at("Docs")
	f.screen.Click(f.pointAt(c, link+1))
	var followed string
	f.screen.Do(func() { followed = f.followed })
	c.Equal("https://example.com/docs", followed, "a click on a link still follows it")

	// A press of a button the document does nothing with is not a gesture it takes the focus for: it neither claims the
	// press nor pulls the keyboard focus out of whatever holds it.
	f.screen.Do(func() { f.wnd.SetFocus(f.extra) })
	f.screen.Sync()
	middle := f.pointAt(c, words)
	f.screen.MouseDown(middle, unison.ButtonMiddle, mod.None)
	f.screen.MouseUp(middle, unison.ButtonMiddle, mod.None)
	var focused bool
	f.screen.Do(func() { focused = f.markdown.Focused() })
	c.False(focused, "a middle-button press leaves the focus where it was")
	f.screen.Do(func() { f.wnd.SetFocus(f.markdown) })
	f.screen.Sync()

	// A markdown nothing is reading is untouched: the caret is never placed and nothing is selected. That is every
	// markdown in an application no assistive technology is watching and no application asked to be read.
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
	}))
	// Where the words and the link are on the screen, worked out while the document still answers: nothing may ask it
	// once support has been refused, and nothing moves in between.
	wordStart := f.pointAt(c, words)
	withinWord := f.pointAt(c, words+1)
	withinLink := f.pointAt(c, link+1)
	f.screen.Do(func() {
		f.markdown.SetFocusable(false)
		f.wnd.SetFocus(f.extra)
		f.followed = ""
	})
	// SetFocusable(false) is not enough on its own. On the platforms whose screen readers start from the keyboard focus a
	// document is focusable for as long as one is being served, whatever the application asked for, and this test has
	// been asking for the description all along — so support is refused as well, which is what an application with no
	// assistive technology attached is on every platform. See Panel.axTakesFocus.
	unison.SetAccessibilityEnabled(false)
	f.screen.Sync()
	var focusable bool
	f.screen.Do(func() { focusable = f.markdown.Focusable() })
	c.False(focusable, "with nothing being served and the application not asking, the document cannot be read")

	f.screen.Click(wordStart)
	f.screen.DoubleClick(withinWord)
	var canCopy bool
	f.screen.Do(func() { canCopy = f.markdown.CanCopy() })
	c.False(canCopy, "a document that cannot be read has nothing to select")
	f.screen.Click(withinLink)
	f.screen.Do(func() { followed = f.followed })
	c.Equal("https://example.com/docs", followed, "while the links within it are as clickable as they ever were")

	unison.SetAccessibilityEnabled(true)
	f.screen.Sync()
	selStart, selEnd, caret = f.caret(c)
	c.Equal([]int{0, 0, 0}, []int{selStart, selEnd, caret},
		"and none of those presses placed a caret or selected a rune")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownMouseCaretInTableColumn verifies that a click in a table lands in the column it was aimed at. Every cell
// of one row of a table is drawn in the same vertical band, so a point has to be resolved to the line whose band holds
// it and whose words are nearest it horizontally: resolving it by its vertical position alone puts every click in a
// table in the first column, from which the caret can never be moved into any other, and a drag across a row selects
// the wrong runes. A nested list, whose outer and inner bullets share a band, has the same shape.
func TestMarkdownMouseCaretInTableColumn(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, "| H1 | H2 |\n| -- | -- |\n| a | b |\n", 300, geom.NewSize(500, 400))
	_, node := f.document(c)
	c.Equal("H1\nH2\na\nb", node.Document.Text.Text, "the whole of this document is one table")
	for _, cell := range []struct {
		name   string
		offset int
	}{
		{name: "H1", offset: 0},
		{name: "H2", offset: 3},
		{name: "a", offset: 6},
		{name: "b", offset: 8},
	} {
		f.screen.Click(f.pointInLineAt(c, cell.offset))
		selStart, selEnd, caret := f.caret(c)
		c.Equal(cell.offset, caret, "a click on the cell holding %q should place the caret in that cell", cell.name)
		c.Equal(caret, selStart)
		c.Equal(caret, selEnd)
	}
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownReadingCaretEndOfWrappedLine verifies where End leaves the caret on a line the text was broken at. Such a
// line ends with the space that separated it from the next, which was taken off the end of it as it was broken and put
// back in the text: that space is the last thing on the line, and the offset just past it is the beginning of the line
// below. A caret taken there is drawn at the start of the next line, and a screen reader following it reads that line
// rather than the one the person is on.
func TestMarkdownReadingCaretEndOfWrappedLine(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	_, node := f.document(c)
	stream := []rune(node.Document.Text.Text)
	var wrapped accessibility.Line
	for _, line := range node.Document.Text.Lines {
		at := line.End - line.Start
		if line.End <= line.Start || line.End > len(stream) || stream[line.End-1] != ' ' {
			continue
		}
		if at < len(line.Advances) && line.Advances[at] == line.Advances[at-1] {
			// A space nothing was drawn for, which is the space a line the text ran out of room on was broken at.
			wrapped = line
			break
		}
	}
	c.True(wrapped.End > wrapped.Start, "the fixture should hold a paragraph the text was broken to fit")
	if wrapped.End <= wrapped.Start {
		return
	}
	toStartOfLine := func() {
		c.Helper()
		c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.SetTextSelection,
			Start:  wrapped.Start,
			End:    wrapped.Start,
		}))
	}

	toStartOfLine()
	f.screen.KeyPress(unison.KeyEnd, mod.None)
	_, _, caret := f.caret(c)
	c.Equal(wrapped.End-1, caret, "End stops at the space the line was broken at, which is the last thing on the line")
	f.screen.KeyPress(unison.KeyEnd, mod.None)
	_, _, caret = f.caret(c)
	c.Equal(wrapped.End-1, caret, "and pressing it again leaves the caret there rather than walking onto the next line")

	if mod.OSMenuCommand() != mod.Control {
		// On macOS the command key with an arrow is what goes to the ends of a line; where the menu commands are on the
		// control key, that is what Home and End are for and control with an arrow moves by word instead.
		toStartOfLine()
		f.screen.KeyPress(unison.KeyRight, mod.OSMenuCommand())
		_, _, caret = f.caret(c)
		c.Equal(wrapped.End-1, caret, "as does the command key with the right arrow")
	}

	// The same boundary from the other side: a click past the end of a wrapped line stays on that line.
	f.screen.Click(f.pointInLineAt(c, wrapped.End-1))
	_, _, caret = f.caret(c)
	c.Equal(wrapped.End-1, caret, "and a click past the last word of the line lands there too")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownCopyAndSelectAll verifies that what the caret has selected can be copied, from the keys, from the
// commands a menu routes to the focus, and that the placeholders standing in for the images within the selection are
// left out of what is put on the clipboard.
func TestMarkdownCopyAndSelectAll(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	var canCopy, canSelectAll bool
	f.screen.Do(func() {
		canCopy = f.markdown.CanCopy()
		canSelectAll = f.markdown.CanSelectAll()
	})
	c.False(canCopy, "nothing is selected, so there is nothing to copy")
	c.True(canSelectAll)

	f.screen.KeyPress(unison.KeyA, mod.OSMenuCommand())
	selStart, selEnd, _ := f.caret(c)
	stream := f.stream(c)
	c.Equal(0, selStart)
	c.Equal(len([]rune(stream)), selEnd, "the command key with A selects the whole document")
	f.screen.Do(func() { canCopy = f.markdown.CanCopy() })
	c.True(canCopy)

	f.screen.Do(func() { unison.ClipboardSetText("") })
	f.screen.KeyPress(unison.KeyC, mod.OSMenuCommand())
	var copied string
	f.screen.Do(func() { copied = unison.ClipboardGetText() })
	c.Equal(strings.ReplaceAll(stream, "￼", ""), copied,
		"the text is copied without the placeholders that stand in for its images")
	c.False(strings.Contains(copied, "￼"))

	// The same two things again, this time as the commands a menu routes to whatever holds the focus.
	_, node := f.document(c)
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
	}))
	f.screen.Do(func() { unison.ClipboardSetText("") })
	var can bool
	f.screen.Do(func() { can = f.markdown.CanPerformCmd(nil, unison.CopyItemID) })
	c.False(can, "with nothing selected the copy command is offered as something that cannot be done")
	f.screen.Do(func() { f.markdown.PerformCmd(nil, unison.SelectAllItemID) })
	selStart, selEnd, _ = f.caret(c)
	c.Equal(0, selStart)
	c.Equal(len([]rune(stream)), selEnd)
	f.screen.Do(func() {
		can = f.markdown.CanPerformCmd(nil, unison.CopyItemID)
		f.markdown.PerformCmd(nil, unison.CopyItemID)
		copied = unison.ClipboardGetText()
	})
	c.True(can)
	c.Equal(strings.ReplaceAll(stream, "￼", ""), copied)
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownSelectAllAndReassertedSelectionDoNotScroll verifies what the view does as the selection changes: every
// move of the caret brings it into view, while selecting the whole document leaves the view where the person was
// reading — selecting all of something is not asking to be taken to the end of it — and a selection that has not moved
// at all neither scrolls nor repaints, which is what an assistive technology re-asserting the caret it already has
// would otherwise cost on every one of its polls.
func TestMarkdownSelectAllAndReassertedSelectionDoNotScroll(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(400, 160))
	position := func() float32 {
		var v float32
		f.screen.Do(func() { _, v = f.scroller.Position() })
		return v
	}
	c.Equal(float32(0), position(), "the document begins at the top")

	f.screen.KeyPress(unison.KeyA, mod.OSMenuCommand())
	selStart, selEnd, _ := f.caret(c)
	c.Equal(0, selStart)
	c.True(selEnd > 0, "the command key with A selects the whole document")
	c.Equal(float32(0), position(), "and leaves the view where the person was reading")

	f.screen.Do(func() { f.markdown.PerformCmd(nil, unison.SelectAllItemID) })
	c.Equal(float32(0), position(), "as does the menu item that does the same thing")

	f.screen.KeyPress(unison.KeyEnd, mod.OSMenuCommand())
	_, _, caret := f.caret(c)
	c.True(caret > 0)
	c.True(position() > 0, "moving the caret to the end of the document does bring it into view")

	// The view is taken back to the top by hand, as a person scrolling away from the caret would, and the selection that
	// is already there is asserted again.
	f.screen.Do(func() { f.scroller.SetPosition(0, 0) })
	f.screen.Sync()
	c.Equal(float32(0), position())
	_, node := f.document(c)
	selStart, selEnd, _ = f.caret(c)
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  selStart,
		End:    selEnd,
	}))
	c.Equal(float32(0), position(), "a selection that has not moved leaves the view alone")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownEmptyTableCellIsStillAPlaceToRead verifies that a cell of a table with nothing in it is still part of the
// document: it took up the room for a line, so it reports the one empty line it drew, it occupies the blank line that
// line is in the stream, and it carries out the selection actions it advertises rather than refusing what it offered.
func TestMarkdownEmptyTableCellIsStillAPlaceToRead(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, "| H1 | H2 |\n| -- | -- |\n| a |  |\n", 300, geom.NewSize(500, 400))
	tree, node := f.document(c)
	cells := axNodesWithRole(tree, role.Cell)
	c.Equal(2, len(cells), "the table has one row of two cells beneath its header")
	var empty *accessibility.Node
	for _, cell := range cells {
		if cell.Text != nil && cell.Text.Text == "" {
			empty = cell
		}
	}
	empty = axMustNode(c, empty, "the cell with nothing in it")
	c.Equal(1, len(empty.Text.Lines), "a block always drew at least one line")
	if len(empty.Text.Lines) == 1 {
		c.Equal(0, empty.Text.Lines[0].Start)
		c.Equal(0, empty.Text.Lines[0].End)
		c.Equal(1, len(empty.Text.Lines[0].Advances), "which holds the one place a caret can sit on it")
	}
	c.False(empty.Text.Multiline)
	c.True(empty.Actions.Has(accessibility.SetTextSelection))
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   empty.ID,
		Action: accessibility.SetTextSelection,
	}), "so it carries the action out rather than refusing what it offered")
	c.Equal("H1\nH2\na\n", node.Document.Text.Text, "and the blank line it drew is part of the stream")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// axMenuItemTitles returns the titles of every menu item in the tree, in the order they are described, which is the
// order they are shown in.
func axMenuItemTitles(tree *accessibility.Tree) []string {
	items := axNodesWithRole(tree, role.MenuItem)
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Name)
	}
	return titles
}

// TestMarkdownContextMenu verifies that a document has no contextual menu of its own, so that the Menu key opens
// nothing, an assistive technology is not offered one and a right-click on the text is an ordinary press; and that,
// given a menu by the application, a right-click on the text is still an ordinary press, since the text is drawn with
// panels of the document's own, while the Menu key opens the menu, an assistive technology may ask for it and have the
// selection placed where the request says first, and the request is refused while the menu would open in another
// window — which is also when the document stops offering it.
func TestMarkdownContextMenu(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	tree, node := f.document(c)
	c.Equal(0, len(axNodesWithRole(tree, role.Menu)), "no menu is open to begin with")
	c.False(node.Actions.Has(accessibility.ShowContextMenu), "a document offers no menu of its own")
	f.screen.KeyPress(unison.KeyMenu, mod.None)
	tree, _ = f.document(c)
	c.Equal(0, len(axNodesWithRole(tree, role.Menu)), "and the Menu key opens nothing")

	// The panel the document sits in has a menu, and is told of every right press the document is sent.
	var outerAsked, rightPresses int
	var outerAt geom.Point
	f.screen.Do(func() {
		f.wnd.Content().ContextMenuCallback = axRecordingMenu(&outerAsked, &outerAt, "Outer")
		mdDown := f.markdown.MouseDownCallback
		f.markdown.MouseDownCallback = func(where geom.Point, button, count int, mods mod.Modifiers) bool {
			if button == unison.ButtonRight {
				rightPresses++
			}
			if mdDown != nil {
				return mdDown(where, button, count, mods)
			}
			return false
		}
	})
	presses := func() int {
		var count int
		f.screen.Do(func() { count = rightPresses })
		return count
	}
	f.screen.ClickWith(f.pointAt(c, 1), unison.ButtonRight, mod.None)
	tree, _ = f.document(c)
	c.Equal(0, len(axNodesWithRole(tree, role.Menu)),
		"a right-click on the text of a document without a menu opens nothing")
	c.Equal(1, presses(), "and is delivered as an ordinary press")
	outerCount, _ := axRecorded(f.screen, &outerAsked, &outerAt)
	c.Equal(0, outerCount, "the menu of the panel the document sits in is not the document's own")

	var asked int
	var where geom.Point
	f.screen.Do(func() { f.markdown.ContextMenuCallback = axRecordingMenu(&asked, &where, "Custom") })
	f.screen.ClickWith(f.pointAt(c, 1), unison.ButtonRight, mod.None)
	tree, node = f.document(c)
	c.Equal(0, len(axNodesWithRole(tree, role.Menu)),
		"a right-click on the text is an ordinary press and opens nothing")
	c.Equal(2, presses(), "the press is delivered as any other")
	count, _ := axRecorded(f.screen, &asked, &where)
	c.Equal(0, count, "and the document is not asked for its menu")
	c.True(node.Actions.Has(accessibility.ShowContextMenu), "a menu the application gave the document is offered")
	f.screen.KeyPress(unison.KeyMenu, mod.None)
	tree, _ = f.document(c)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)), "the Menu key opens the contextual menu")
	c.Equal([]string{"Custom"}, axMenuItemTitles(tree))
	count, _ = axRecorded(f.screen, &asked, &where)
	c.Equal(1, count)
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	f.screen.Sync()
	c.Equal(0, len(axNodesWithRole(f.screen.AccessibilityTree(f.wnd), role.Menu)), "which Escape closes again")

	stream := f.stream(c)
	at := strings.Index(stream, "Take care.")
	c.True(at >= 0)
	offset := len([]rune(stream[:at]))
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
		Start:  offset,
		End:    offset + 4,
	}))
	selStart, selEnd, _ := f.caret(c)
	c.Equal(offset, selStart, "the range the request named is selected before the menu opens")
	c.Equal(offset+4, selEnd)
	tree, _ = f.document(c)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)))
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	f.screen.Sync()

	// A request for a single position within that selection keeps it, while one for a position outside it moves the
	// caret there.
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
		Start:  offset + 2,
		End:    offset + 2,
	}))
	selStart, selEnd, _ = f.caret(c)
	c.Equal(offset, selStart, "a position within the selection keeps the selection for the menu")
	c.Equal(offset+4, selEnd)
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	f.screen.Sync()
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
		Start:  offset + 8,
		End:    offset + 8,
	}))
	selStart, selEnd, _ = f.caret(c)
	c.Equal(offset+8, selStart, "a position outside it moves the caret there")
	c.Equal(offset+8, selEnd)
	f.screen.KeyPress(unison.KeyEscape, mod.None)
	f.screen.Sync()

	// A menu opens in whatever window is active rather than in the one the document is in, so a document in a window that
	// is not the active one neither offers its menu nor opens one.
	var other *unison.Window
	f.screen.Do(func() {
		other = newHeadlessWindow(t, "other", geom.NewRect(300, 300, 200, 100), unison.NewField())
		if other != nil {
			other.ToFront()
		}
	})
	f.screen.Sync()
	c.True(other != nil)
	_, node = f.document(c)
	c.False(node.Actions.Has(accessibility.ShowContextMenu),
		"a document whose menu would open somewhere else does not offer it")
	c.False(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}), "and refuses to open one")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// axRecordingMenu returns a ContextMenuCallback that counts its calls in asked and records where in at, returning a
// menu with one item named title, or nil when title is empty.
func axRecordingMenu(asked *int, at *geom.Point, title string) func(geom.Point) unison.Menu {
	return func(where geom.Point) unison.Menu {
		*asked++
		*at = where
		if title == "" {
			return nil
		}
		f := unison.DefaultMenuFactory()
		m := f.NewMenu(unison.PopupMenuTemporaryBaseID|unison.ContextMenuIDFlag, "", nil)
		m.InsertItem(-1, f.NewItem(-1, title, unison.KeyBinding{}, nil, func(unison.MenuItem) {}))
		return m
	}
}

// axRecorded reads what axRecordingMenu recorded, on the UI thread, the only thread that writes it.
func axRecorded(screen *unison.HeadlessScreen, asked *int, at *geom.Point) (count int, where geom.Point) {
	screen.Do(func() {
		count = *asked
		where = *at
	})
	return count, where
}

// TestMarkdownContextMenuRequestAsksOnce verifies that an assistive technology's request for a document's menu calls
// the callback once whether or not it returns a menu, and that a range it names places the caret first.
func TestMarkdownContextMenuRequestAsksOnce(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	var asked int
	var at geom.Point
	f.screen.Do(func() { f.markdown.ContextMenuCallback = axRecordingMenu(&asked, &at, "") })
	_, node := f.document(c)
	c.True(node.Actions.Has(accessibility.ShowContextMenu))
	c.False(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
		Start:  3,
		End:    3,
	}), "a callback with nothing to offer shows nothing")
	count, where := axRecorded(f.screen, &asked, &at)
	c.Equal(1, count, "and is asked once for one request")
	_, node = f.document(c)
	info := node.Document.Text
	c.Equal(3, info.Caret, "the range the request named places the caret")
	line := info.Lines[0]
	c.True(info.Caret < line.End, "the caret is on the first line")
	c.Equal(geom.NewPoint(line.Bounds.X+line.Advances[info.Caret-line.Start], line.Bounds.Bottom()), where,
		"before the menu is asked for, which is asked for beneath it")

	f.screen.Do(func() { f.markdown.ContextMenuCallback = axRecordingMenu(&asked, &at, "Something") })
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	count, _ = axRecorded(f.screen, &asked, &at)
	c.Equal(2, count, "a callback with a menu is asked once as well")
	tree, _ := f.document(c)
	c.Equal(1, len(axNodesWithRole(tree, role.Menu)))
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}

// TestMarkdownContextMenuAnchorBringsTheCaretIntoView verifies that a document's menu opened without a pointer scrolls
// the reading caret back into view first and opens beneath its line.
func TestMarkdownContextMenuAnchorBringsTheCaretIntoView(t *testing.T) {
	c := check.New(t)
	f := newAXMarkdownFixture(t, markdownAXContent, 300, geom.NewSize(500, 400))
	var asked int
	var at geom.Point
	f.screen.Do(func() { f.markdown.ContextMenuCallback = axRecordingMenu(&asked, &at, "Custom") })
	_, node := f.document(c)
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  2,
		End:    2,
	}))
	var top, scrolled float32
	f.screen.Do(func() {
		_, top = f.scroller.Position()
		f.scroller.SetPosition(0, 10000)
		_, scrolled = f.scroller.Position()
	})
	c.True(scrolled > top+50, "the view has to have moved well away from the caret for this to mean anything")

	_, node = f.document(c)
	c.True(f.screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	var after float32
	f.screen.Do(func() { _, after = f.scroller.Position() })
	c.True(after < scrolled, "the caret is brought back into view before the menu opens")

	tree, node := f.document(c)
	menus := axNodesWithRole(tree, role.Menu)
	c.Equal(1, len(menus))
	if len(menus) != 1 {
		return
	}
	info := node.Document.Text
	c.Equal(2, info.Caret)
	var expected geom.Point
	f.screen.Do(func() {
		for i, line := range info.Lines {
			if i == len(info.Lines)-1 || info.Caret < info.Lines[i+1].Start {
				local := geom.NewPoint(line.Bounds.X+line.Advances[info.Caret-line.Start], line.Bounds.Bottom())
				expected = f.markdown.PointToRoot(local)
				break
			}
		}
	})
	c.Equal(expected, menus[0].Bounds.Point, "the menu opens beneath the caret's line, where it can now be seen")
	c.Equal(0, len(f.screen.Errors()), "nothing should have panicked: %v", f.screen.Errors())
}
