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
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/textunit"
)

// These tests reach inside a rendered markdown: the text each block worked out from the panels drawing it, the two ways
// a line can end, and what throws that work away again. A session owns most of the package's mutable globals while it
// runs, so none of them may call t.Parallel.

// axMarkdownBlocks returns every block within a panel, in reading order.
func axMarkdownBlocks(p *Panel) []*markdownBlock {
	var blocks []*markdownBlock
	var walk func(panel *Panel)
	walk = func(panel *Panel) {
		for _, child := range panel.Children() {
			if block, ok := child.Self.(*markdownBlock); ok {
				blocks = append(blocks, block)
			}
			walk(child)
		}
	}
	walk(p)
	return blocks
}

// axMarkdownRows returns every row within a panel, in reading order.
func axMarkdownRows(p *Panel) []*markdownRow {
	var rows []*markdownRow
	var walk func(panel *Panel)
	walk = func(panel *Panel) {
		for _, child := range panel.Children() {
			if row, ok := child.Self.(*markdownRow); ok {
				rows = append(rows, row)
			}
			walk(child)
		}
	}
	walk(p)
	return rows
}

// axNewTestMarkdown shows a markdown in a window, laid out, so that the panels drawing it are where they will be drawn.
func axNewTestMarkdown(t *testing.T, content string, width float32) (*HeadlessScreen, *Window, *Markdown) {
	t.Helper()
	var md *Markdown
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 800},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetFocusable(true)
			md.SetContent(content, width)
			scroller := NewScrollPanel()
			scroller.SetContent(md, behavior.Fill, behavior.Unmodified)
			wnd = axNewTestWindow(t, "markdown", geom.NewRect(10, 10, 500, 400), scroller)
			if wnd != nil {
				wnd.ToFront()
				wnd.SetFocus(md)
			}
		}))
	if wnd == nil {
		t.Fatal("the window holding the markdown was not created")
	}
	screen.Sync()
	return screen, wnd, md
}

// TestMarkdownBlockTextChunkAdvances verifies that the rune boundaries a block reports are where the pieces of its text
// were actually drawn: each piece sits in a panel of its own, whose frame was rounded up to a whole unit, so a boundary
// worked out by summing the widths of the pieces before it drifts away from the text on the screen — and it is the text
// on the screen that a caret dropped at a boundary, and a highlight drawn from one, have to agree with.
func TestMarkdownBlockTextChunkAdvances(t *testing.T) {
	c := check.New(t)
	screen, _, md := axNewTestMarkdown(t, "A [linked](https://example.com/x) run of words with **bold** in it.\n", 400)
	var paragraph *markdownBlock
	screen.Do(func() {
		for _, block := range axMarkdownBlocks(md.AsPanel()) {
			if block.kind == role.Paragraph {
				paragraph = block
			}
		}
	})
	c.NotNil(paragraph)
	if paragraph == nil {
		return
	}
	screen.Do(func() {
		text := paragraph.axText()
		c.Equal("A linked run of words with bold in it.", string(text.text))
		c.Equal(1, len(text.lines), "the paragraph fits on one line at this width")
		line := text.lines[0]
		c.Equal(len(text.text)+1, len(line.Advances))
		c.Equal(float32(0), line.Advances[0], "the advances are measured from the first rune of the line")
		rows := axMarkdownRows(paragraph.AsPanel())
		c.Equal(1, len(rows))
		if len(rows) != 1 {
			return
		}
		offset := 0
		chunks := 0
		for _, chunk := range rows[0].Children() {
			label, ok := chunk.Self.(*Label)
			if !ok {
				continue
			}
			chunks++
			want := label.PointTo(label.axTextOrigin(), paragraph.AsPanel()).X - line.Bounds.X
			nearlyEqual(c, want, line.Advances[offset])
			offset += len(label.Text.Runes())
		}
		c.True(chunks >= 3, "the line should be drawn as several pieces, got %d", chunks)
		c.Equal(len(text.text), offset, "the pieces together are the whole of the line")
		for i := 1; i < len(line.Advances); i++ {
			c.True(line.Advances[i] >= line.Advances[i-1], "the advances must not go backwards at %d", i)
		}
		// The link occupies the words it was written as, which is what an assistive technology offers among the
		// document's links.
		c.Equal(1, len(text.spans))
		if len(text.spans) == 1 {
			c.Equal("linked", string(text.text[text.spans[0].start:text.spans[0].end]))
			c.Equal(role.Link, text.spans[0].panel.Accessibility.Role)
		}
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownRowFlags verifies that a block's text reads as what was written however the lines of it were broken: a
// line the text ran out of room on had the space that separated it from the next taken off its end, and it is put back,
// while a line a break in the markdown ended is ended by a line feed. Neither of them was drawn, so neither takes up
// any room on the line that owns it.
func TestMarkdownRowFlags(t *testing.T) {
	c := check.New(t)
	screen, _, md := axNewTestMarkdown(t,
		"first line  \nsecond line\n\nA paragraph long enough that it has to be broken across more than one line "+
			"of its own before it ends.\n", 160)
	screen.Do(func() {
		blocks := axMarkdownBlocks(md.AsPanel())
		c.Equal(2, len(blocks))
		if len(blocks) != 2 {
			return
		}
		broken := blocks[0]
		rows := axMarkdownRows(broken.AsPanel())
		c.True(len(rows) >= 2)
		c.True(rows[0].hardBreak, "the line the markdown itself ended says so")
		c.False(rows[len(rows)-1].hardBreak, "and the last line of the block does not, since nothing ended it")
		text := broken.axText()
		c.Equal("first line\nsecond line", string(text.text))
		lines := text.lines
		c.True(len(lines) >= 2)
		c.Equal('\n', text.text[lines[0].End-1], "the line feed belongs to the line it ended")
		c.Equal(lines[0].Advances[len(lines[0].Advances)-1], lines[0].Advances[len(lines[0].Advances)-2],
			"nothing was drawn for it, so it takes up no room")

		wrapped := blocks[1]
		rows = axMarkdownRows(wrapped.AsPanel())
		c.True(len(rows) > 1, "the paragraph should have been broken to fit")
		c.True(rows[0].strippedSpace, "the space the line was broken at was taken off the end of it")
		text = wrapped.axText()
		c.True(len(text.lines) > 1)
		first := text.lines[0]
		c.Equal(' ', text.text[first.End-1], "and put back, on the line that ended with it")
		c.Equal(first.Advances[len(first.Advances)-1], first.Advances[len(first.Advances)-2],
			"with no room of its own, since nothing was drawn for it")
		c.False(rows[len(rows)-1].strippedSpace, "the last line of the paragraph was not broken at all")
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownDocumentCacheInvalidation verifies that the text of a document is worked out once and then reused, and
// that everything which could move a rune of it throws it away again: new content, a new layout, and an image arriving
// where a placeholder was.
func TestMarkdownDocumentCacheInvalidation(t *testing.T) {
	c := check.New(t)
	screen, _, md := axNewTestMarkdown(t, "# Title\n\nSome words.\n\n![x](missing-image-for-test.png)\n", 300)
	var first *axDocument
	var block *markdownBlock
	screen.Do(func() {
		first = md.axDocument()
		c.True(first == md.axDocument(), "the stream is composed once and then reused")
		blocks := axMarkdownBlocks(md.AsPanel())
		c.True(len(blocks) > 0)
		block = blocks[0]
		text := block.axText()
		c.True(text == block.axText(), "as is the text of each block within it")
	})
	c.NotNil(first)
	c.NotNil(block)
	if first == nil || block == nil {
		return
	}

	screen.Do(func() {
		before := block.axText()
		md.SetContent("# Title\n\nSome words.\n\n![x](missing-image-for-test.png)\n", 240)
		c.True(md.axDoc == nil, "content set again throws the stream away")
		c.True(md.axDocument() != first, "so the next thing to ask is given the document as it now is")
		// The blocks were built afresh, and a block that was kept would still work its text out again, since what it says
		// about where its lines are is read from where its panels were placed.
		c.True(before != block.axText())
	})

	screen.Do(func() {
		composed := md.axDocument()
		moved := md.Children()[0]
		frame := moved.FrameRect()
		frame.Y += 5
		moved.SetFrameRect(frame)
		c.True(md.axDoc == nil, "anything within the document moving throws it away as well, since where the lines "+
			"are is read from where the panels drawing them were placed")
		c.True(md.axDocument() != composed)
	})

	screen.Do(func() {
		composed := md.axDocument()
		md.drawableCache["retrieved"] = &drawableCacheEntry{}
		md.updateDrawable("retrieved", &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewUniformSize(16)})
		c.True(md.axDoc == nil, "an image arriving where a placeholder was moves every rune after it")
		c.True(md.axDocument() != composed)
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownCaretBlockReportsFocus covers the second claim on the focus the block holding the reading caret makes: on
// the platforms whose screen readers need it the claim stands, and the document is still what the tree reports the
// focus on, while everywhere else the builder takes it away. Orca presents a caret the application moved only from an
// object carrying the focused state; UI Automation and AppKit have one focused element apiece and would follow the
// wrong one.
//
// Both answers are pinned whichever platform this runs on, since the document is built once for all three.
func TestMarkdownCaretBlockReportsFocus(t *testing.T) {
	c := check.New(t)
	screen, wnd, md := axNewTestMarkdown(t, "# Title\n\nSome words in a paragraph.\n", 300)
	var heading, paragraph *markdownBlock
	screen.Do(func() {
		for _, block := range axMarkdownBlocks(md.AsPanel()) {
			switch block.kind {
			case role.Heading:
				heading = block
			case role.Paragraph:
				paragraph = block
			}
		}
	})
	c.NotNil(heading)
	c.NotNil(paragraph)
	if heading == nil || paragraph == nil {
		return
	}
	// The caret is put in the paragraph, so the heading is a block that has one and the paragraph is the block that holds
	// it.
	screen.Do(func() { md.axSetSelection(len("Title")+3, len("Title")+3) })

	saved := axCaretBlockReportsFocus
	defer screen.Do(func() { axCaretBlockReportsFocus = saved })
	for _, reports := range []bool{true, false} {
		// Written on the UI thread, which is the only thread that reads it, since a snapshot is built there.
		screen.Do(func() { axCaretBlockReportsFocus = reports })
		tree := screen.AccessibilityTree(wnd)
		c.NotNil(tree)
		document := screen.AccessibilityNodeFor(md)
		caretBlock := screen.AccessibilityNodeFor(paragraph)
		otherBlock := screen.AccessibilityNodeFor(heading)
		c.NotNil(document)
		c.NotNil(caretBlock)
		c.NotNil(otherBlock)
		if tree == nil || document == nil || caretBlock == nil || otherBlock == nil {
			return
		}
		c.True(document.Focused, "the document holds the keyboard focus whatever the blocks within it say")
		c.Equal(document.ID, tree.Focus, "and is what the tree reports the focus on")
		c.Equal(reports, caretBlock.Focused,
			"the block holding the caret reports the focus only where a screen reader needs it (%v)", reports)
		c.False(otherBlock.Focused, "no other block ever does")
	}

	// With the focus somewhere else entirely there is no caret to present, so no block claims it however much the
	// platform's screen reader would have wanted one.
	screen.Do(func() {
		axCaretBlockReportsFocus = true
		wnd.SetFocus(nil)
	})
	screen.AccessibilityTree(wnd)
	caretBlock := screen.AccessibilityNodeFor(paragraph)
	c.NotNil(caretBlock)
	if caretBlock != nil {
		c.False(caretBlock.Focused)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axVisualRows returns how many visual rows the pieces of one row were flowed onto, counted from the tops the layout
// actually placed them at rather than from the grouping the text does, so that the two are checked against each other.
func axVisualRows(row *Panel) int {
	var tops []float32
	for _, chunk := range row.Children() {
		if chunk.Hidden {
			continue
		}
		if top := chunk.FrameRect().Y; !slices.Contains(tops, top) {
			tops = append(tops, top)
		}
	}
	return max(len(tops), 1)
}

// axCheckBlockLines verifies what a block says about the lines it drew: the rune boundaries of a line never walk back
// to the left, and the lines of one block follow one another down the page rather than sharing a vertical band.
func axCheckBlockLines(c check.Checker, block *markdownBlock) {
	c.Helper()
	text := block.axText()
	for i, line := range text.lines {
		for j := 1; j < len(line.Advances); j++ {
			c.True(line.Advances[j] >= line.Advances[j-1],
				"line %d of %q: the advances must not go backwards at %d (%v)", i, string(text.text), j, line.Advances)
		}
		if i != 0 {
			c.True(line.Bounds.Y >= text.lines[i-1].Bounds.Bottom(),
				"line %d of %q at %v should sit below line %d at %v", i, string(text.text), line.Bounds, i-1,
				text.lines[i-1].Bounds)
		}
	}
}

// axSetMarkdownWidth lays a markdown out to exactly the width its content was wrapped to, which is where the pieces of
// a line have no slack left: everything that fits, fits exactly, and anything the wrapping did not account for is
// flowed onto a row of its own. A markdown in a view wider than its content never shows that. This is the width an
// application that sizes its markdown from its parent gets, since SetContentBytes takes the border off the width it
// wraps to.
func axSetMarkdownWidth(md *Markdown, width float32) {
	md.SetContentBytes(md.ContentBytes(), width)
	frame := md.FrameRect()
	frame.Width = width
	if border := md.Border(); border != nil {
		frame.Width += border.Insets().Width()
	}
	md.SetFrameRect(frame)
	md.ValidateLayout()
}

// TestMarkdownRowIsOneVisualLine verifies that a row of a block holds the one visual line the text takes it for, and
// that nothing on a row is pushed past the width the content was wrapped to. A row holds the pieces of a line in a
// FlowLayout, which puts a piece that does not fit in what is left of the line onto a row of its own beneath the
// others; an image is the piece that could, since it is added at whatever size it was drawn at, so it begins a new line
// when the rest of this one will not hold it, exactly as a link does. Without that, the line it is on ends up wider
// than the document was wrapped to — and wherever the room for it is not there to be taken, as in a cell of a table,
// the row is flowed onto two visual rows while remaining the one line the text reads it as. The widths are walked over
// because which of them leaves an image with too little room depends on the font the text around it was measured in.
func TestMarkdownRowIsOneVisualLine(t *testing.T) {
	c := check.New(t)
	const content = "Body text long enough that it has to be broken across several lines of its own before it reaches " +
		"the ![A cat](missing-image-for-test.png) image, which therefore lands wherever the width leaves it, and then " +
		"more words after the image.\n"
	screen, _, md := axNewTestMarkdown(t, content, 300)
	for width := float32(150); width <= 330; width += 3 {
		screen.Do(func() {
			axSetMarkdownWidth(md, width)
			blocks := axMarkdownBlocks(md.AsPanel())
			c.True(len(blocks) > 0)
			for _, block := range blocks {
				c.True(block.FrameRect().Width <= width,
					"at a width of %v the block is %v wide, so something on it was not given a line of its own",
					width, block.FrameRect().Width)
				for i, row := range axMarkdownRows(block.AsPanel()) {
					c.Equal(1, axVisualRows(row.AsPanel()),
						"at a width of %v, row %d should hold one visual line", width, i)
				}
				axCheckBlockLines(c, block)
			}
		})
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownOverflowingRowIsSeveralLines verifies what a block says about a row that was flowed onto more than one
// visual row anyway. Nothing builds such a row — see TestMarkdownRowIsOneVisualLine — so one is made here by giving a
// row a third of the width it was laid out at, which is what a line whose pieces no longer fit becomes. Reported as one
// line, such a row would be a line three rows tall whose rune boundaries walk back to the left twice along the way,
// from which a caret dropped at a point, the highlight drawn over a range and the rectangles an assistive technology is
// handed all read nonsense.
func TestMarkdownOverflowingRowIsSeveralLines(t *testing.T) {
	c := check.New(t)
	const content = "Words with ![A cat](missing-image-for-test.png) in it.\n"
	screen, _, md := axNewTestMarkdown(t, content, 400)
	screen.Do(func() {
		axSetMarkdownWidth(md, 400)
		var block *markdownBlock
		for _, one := range axMarkdownBlocks(md.AsPanel()) {
			if one.kind == role.Paragraph {
				block = one
			}
		}
		c.NotNil(block)
		if block == nil {
			return
		}
		rows := axMarkdownRows(block.AsPanel())
		c.Equal(1, len(rows), "the paragraph fits on one row at this width")
		if len(rows) != 1 {
			return
		}
		row := rows[0].AsPanel()
		c.Equal(1, len(block.axText().lines), "so the block is one line of text")
		frame := row.FrameRect()
		frame.Width /= 3
		row.SetFrameRect(frame)
		row.ValidateLayout()
		flowed := axVisualRows(row)
		c.True(flowed > 1, "the row should have been flowed onto more than one visual row, got %d", flowed)
		text := block.axText()
		c.Equal(flowed, len(text.lines), "a block reports one line per visual row it was drawn as")
		axCheckBlockLines(c, block)
		c.Equal(0, text.lines[0].Start, "the lines still begin at the beginning of the text")
		c.Equal(len(text.text), text.lines[len(text.lines)-1].End, "and cover the whole of it")
		for i, line := range text.lines {
			if i != 0 {
				c.Equal(text.lines[i-1].End, line.Start, "following one another through it")
			}
			c.True(len(line.Advances) != 0, "line %d should hold the boundary of every rune on it", i)
		}
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownComposedStringsAreKept verifies that the text handed to an assistive technology is composed once for the
// layout it was read from rather than on every snapshot. A description is built after every redraw while a screen
// reader is attached, and turning a whole document's runes into a string on each of them is a copy of the entire
// document for text that has not changed.
func TestMarkdownComposedStringsAreKept(t *testing.T) {
	c := check.New(t)
	screen, wnd, md := axNewTestMarkdown(t, "# Title\n\nSome words in a paragraph.\n", 300)
	screen.Do(func() {
		md.axInvalidate()
		doc := md.axDocument()
		blocks := axMarkdownBlocks(md.AsPanel())
		c.True(len(blocks) > 0)
		if len(blocks) == 0 {
			return
		}
		text := blocks[0].axText()
		c.True(doc.composed == nil, "nothing is composed into a string until something asks for it")
		c.True(text.composed == nil)
		c.Equal(string(doc.text), doc.str(), "and what comes back is the text itself")
		c.Equal(string(text.text), text.str())
		c.True(doc.composed != nil && text.composed != nil, "which is then kept beside the runes")
		kept := doc.composed
		c.Equal(*kept, doc.str())
		c.True(kept == doc.composed, "so asking a second time composes nothing")
	})
	// A description hands the same strings over, and anything that moves a rune of the text throws them away with it.
	screen.AccessibilityTree(wnd)
	screen.Do(func() {
		c.True(md.axDocument().composed != nil, "publishing a description composes the stream")
		md.axInvalidate()
		c.True(md.axDocument().composed == nil, "which the next layout throws away along with the text")
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownStreamComposesLooseRowsAndImages verifies the two pieces of content the document builder deals with
// rather than walking into: a row of text that no block owns, which is one visual line rather than a line per piece of
// it, and an image that no block's text holds, which occupies the one rune standing in for something that is not text.
// Nothing builds either of them today — a row and an image are added to whatever block is being filled — so they are
// put in by hand, since what a walk that fell through to its default case would compose is a stream that says the wrong
// thing about the content.
func TestMarkdownStreamComposesLooseRowsAndImages(t *testing.T) {
	c := check.New(t)
	screen, _, md := axNewTestMarkdown(t, "# Title\n", 300)
	screen.Do(func() {
		decoration := &TextDecoration{Font: LabelFont, OnBackgroundInk: ThemeOnSurface}
		row := newMarkdownRow()
		for _, word := range []string{"loose ", "words"} {
			label := NewLabel()
			label.Accessibility.Role = role.None
			label.Text = NewText(word, decoration)
			row.AddChild(label)
		}
		md.AddChild(row)
		image := NewDrawablePanel()
		image.Drawable = &DrawableSVG{SVG: BrokenImageSVG, Size: geom.NewUniformSize(16)}
		image.Accessibility.Name = "a loose image"
		md.AddChild(image)
		md.MarkForLayoutAndRedraw()
		md.ValidateLayout()
		md.axInvalidate()
		doc := md.axDocument()
		c.Equal("Title\nloose words\n"+string(textunit.ObjectReplacement), string(doc.text),
			"the row reads as the one line it is, and the image as the rune that stands in for one")
		c.Equal(3, len(doc.lines), "one line for the heading, one for the row and one for the image")
		for i, line := range doc.lines {
			for j := 1; j < len(line.Advances); j++ {
				c.True(line.Advances[j] >= line.Advances[j-1], "line %d: the advances must not go backwards at %d", i, j)
			}
		}
		var forImage, forRow bool
		for _, span := range doc.spans {
			switch span.panel {
			case image.AsPanel():
				forImage = true
				c.Equal(string(textunit.ObjectReplacement), string(doc.text[span.start:span.end]))
			case row.AsPanel():
				forRow = true
			}
		}
		c.True(forImage, "the image occupies the rune that stands in for it, so it can be reached from the text")
		c.False(forRow, "while the row is no element of the document and names no node in the tree")
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownFrameChangeCallbackChains verifies the one callback a document cannot lose. Everything the text of a
// document says about itself is read from where the panels drawing it were placed, and the frame change in child
// hierarchy callback is the only thing that throws that away when they move, so an application that wants to be told as
// well installs its own and calls the default from it — which is what DefaultFrameChangeInChildHierarchy is for.
func TestMarkdownFrameChangeCallbackChains(t *testing.T) {
	c := check.New(t)
	screen, _, md := axNewTestMarkdown(t, "# Title\n\nSome words.\n", 300)
	screen.Do(func() {
		told := 0
		md.FrameChangeInChildHierarchyCallback = func(panel *Panel) {
			told++
			md.DefaultFrameChangeInChildHierarchy(panel)
		}
		composed := md.axDocument()
		moved := md.Children()[0]
		frame := moved.FrameRect()
		frame.Y += 5
		moved.SetFrameRect(frame)
		c.True(told > 0, "an application that put its own callback in is the one that is told")
		c.True(md.axDoc == nil, "and the default it called threw the composed stream away")
		c.True(md.axDocument() != composed, "so the next thing to ask is given the document as it now is")
	})
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
