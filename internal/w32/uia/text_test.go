// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests cover the whole of the Text pattern's arithmetic, which is where a mistake would be invisible until a
// screen reader read a document wrongly: the divisions a document is read in, what expanding and moving a range does,
// what a range says about itself, and where on screen it is. None of it needs Windows.

// The fixture's stream, laid out so that every offset in the tests can be checked by eye. The blocks are joined by line
// feeds, exactly as a document composes them, an image contributes the single object-replacement character, and the
// paragraph is wrapped across two visual lines after "link ".
//
//	offset  0       5 6                17 18   22 23   28 29 32 33 34 35 36
//	        Title   ⏎ Hello link       ￼  " end" ⏎ x = 1 ⏎ one ⏎ A  ⏎  B
//
//	 0..5   heading "Title", level 2
//	 6..22  paragraph "Hello link ￼ end", wrapped after offset 17
//	12..16  a link, "link"
//	17..18  an image, the object-replacement character
//	23..28  a code block, "x = 1"
//	29..32  a list holding one list item, "one"
//	33..36  a table holding one row of two cells, "A" and "B"
const (
	textFixtureText   = "Title\nHello link ￼ end\nx = 1\none\nA\nB"
	textFixtureLength = 36
)

// The ids of the fixture's nodes.
const (
	textWindowID    = accessibility.NodeID(1)
	textScrollID    = accessibility.NodeID(2)
	textDocumentID  = accessibility.NodeID(3)
	textHeadingID   = accessibility.NodeID(4)
	textParagraphID = accessibility.NodeID(5)
	textLinkID      = accessibility.NodeID(6)
	textImageID     = accessibility.NodeID(7)
	textCodeID      = accessibility.NodeID(8)
	textListID      = accessibility.NodeID(9)
	textListItemID  = accessibility.NodeID(10)
	textTableID     = accessibility.NodeID(11)
	textRowID       = accessibility.NodeID(12)
	textCellAID     = accessibility.NodeID(13)
	textCellBID     = accessibility.NodeID(14)
	textLabelID     = accessibility.NodeID(15)
	textButtonID    = accessibility.NodeID(16)
)

// textLine builds one visual line of the fixture: ten units per rune, so that an offset and a coordinate can be read
// off each other. The line feed a block is joined to the next with belongs to the line it ends and is zero-width, which
// is why a line's last two advances are equal.
func textLine(start, end int, y float32) accessibility.Line {
	runes := end - start
	advances := make([]float32, 0, runes+1)
	width := float32(0)
	for i := 0; i <= runes; i++ {
		advances = append(advances, width)
		if i < runes && textFixtureRunes()[start+i] != '\n' {
			width += 10
		}
	}
	return accessibility.Line{
		Advances: advances,
		Start:    start,
		End:      end,
		Bounds:   geom.NewRect(0, y, width, 10),
	}
}

// textFixtureRunes returns the fixture's stream as runes.
func textFixtureRunes() []rune {
	return []rune(textFixtureText)
}

// textFixtureSpans returns the fixture's spans, in the order accessibility.TextInfo documents: outermost first, then
// by start offset. The table's row is a virtual node, exactly as a Markdown table publishes one.
func textFixtureSpans() []accessibility.TextSpan {
	return []accessibility.TextSpan{
		{Node: textHeadingID, Start: 0, End: 5},
		{Node: textParagraphID, Start: 6, End: 22},
		{Node: textCodeID, Start: 23, End: 28},
		{Node: textListID, Start: 29, End: 32},
		{Node: textTableID, Start: 33, End: 36},
		{Node: textLinkID, Start: 12, End: 16},
		{Node: textImageID, Start: 17, End: 18},
		{Node: textListItemID, Start: 29, End: 32},
		{Node: textRowID, Start: 33, End: 36},
		{Node: textCellAID, Start: 33, End: 34},
		{Node: textCellBID, Start: 35, End: 36},
	}
}

// textFixtureRuns returns the fixture's styled runs, which tile the stream: a bold heading in one family, body text
// in another, an underlined link within it, a monospaced code block, and a last table cell whose text is italic and
// struck through, which is the one run either of those two attributes is true over. The split at offset 35 adds no
// formatting boundary of its own, since that cell's span already begins there.
func textFixtureRuns() []accessibility.TextRun {
	return []accessibility.TextRun{
		{Family: "Heading", Start: 0, End: 5, Weight: 700, Size: 20},
		{Family: "Body", Start: 5, End: 12, Weight: 400, Size: 12},
		{Family: "Body", Start: 12, End: 16, Weight: 400, Size: 12, Underline: true},
		{Family: "Body", Start: 16, End: 23, Weight: 400, Size: 12},
		{Family: "Mono", Start: 23, End: 28, Weight: 400, Size: 11, Monospace: true},
		{Family: "Body", Start: 28, End: 35, Weight: 400, Size: 12},
		{Family: "Body", Start: 35, End: 36, Weight: 400, Size: 12, Italic: true, Strikethrough: true},
	}
}

// textFixtureTree builds the snapshot the text tests work over: a window holding a scroll area, holding the
// document. The scroll area is shorter than the document, so the last three lines are clipped away, which is what makes
// the visible range and the clipped rectangles worth testing. The document holds the keyboard focus, which is what the
// IsActive attribute and GetCaretRange report.
//
// The document's bounds are offset from the window's origin so that a conversion that forgets to add them shows up, and
// its lines are measured from its own top left corner, as a snapshot records them.
func textFixtureTree() *accessibility.Tree {
	document := &accessibility.Node{
		ID: textDocumentID, Role: role.Document, Name: "Notes", Focusable: true, Focused: true,
		Bounds: geom.NewRect(10, 20, 110, 70),
		Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection, accessibility.ScrollRangeIntoView,
			accessibility.ShowContextMenu, accessibility.ScrollIntoView),
		Children: []accessibility.NodeID{
			textHeadingID, textParagraphID, textCodeID, textListID, textTableID,
		},
		Document: &accessibility.DocumentInfo{
			Text: accessibility.TextInfo{
				Text: textFixtureText,
				Lines: []accessibility.Line{
					textLine(0, 6, 0),
					textLine(6, 17, 10),
					textLine(17, 23, 20),
					textLine(23, 29, 30),
					textLine(29, 33, 40),
					textLine(33, 35, 50),
					textLine(35, 36, 60),
				},
				Runs:      textFixtureRuns(),
				Spans:     textFixtureSpans(),
				SelStart:  12,
				SelEnd:    16,
				Caret:     12,
				Multiline: true,
			},
		},
	}
	return newTestTree(textWindowID, textDocumentID,
		&accessibility.Node{
			ID: textWindowID, Role: role.Window, Name: "Window", Focused: true,
			Bounds:   geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{textScrollID, textButtonID},
		},
		&accessibility.Node{
			ID: textScrollID, Role: role.ScrollArea, Bounds: geom.NewRect(10, 20, 110, 35),
			Children: []accessibility.NodeID{textDocumentID},
		},
		document,
		&accessibility.Node{
			ID: textHeadingID, Role: role.Heading, Name: "Title", Level: 2, Bounds: geom.NewRect(10, 20, 50, 10),
		},
		&accessibility.Node{
			ID: textParagraphID, Role: role.Paragraph, Bounds: geom.NewRect(10, 30, 110, 20),
			Children: []accessibility.NodeID{textLinkID, textImageID, textLabelID},
			Text:     &accessibility.TextInfo{Text: "Hello link ￼ end"},
		},
		&accessibility.Node{
			ID: textLinkID, Role: role.Link, Name: "link", URL: "https://example.com",
			Bounds: geom.NewRect(70, 30, 40, 10),
		},
		&accessibility.Node{ID: textImageID, Role: role.Image, Name: "A picture", Bounds: geom.NewRect(10, 40, 10, 10)},
		&accessibility.Node{
			ID: textCodeID, Role: role.Code, Bounds: geom.NewRect(10, 50, 50, 10),
			Text: &accessibility.TextInfo{Text: "x = 1"},
		},
		&accessibility.Node{
			ID: textListID, Role: role.List, Bounds: geom.NewRect(10, 60, 30, 10),
			Children: []accessibility.NodeID{textListItemID},
		},
		&accessibility.Node{ID: textListItemID, Role: role.ListItem, Bounds: geom.NewRect(10, 60, 30, 10)},
		&accessibility.Node{
			ID: textTableID, Role: role.Table, Bounds: geom.NewRect(10, 70, 10, 20), RowCount: 1, ColumnCount: 2,
			Children: []accessibility.NodeID{textRowID},
		},
		&accessibility.Node{
			ID: textRowID, Role: role.Row, RowIndex: 0, Bounds: geom.NewRect(10, 70, 10, 20),
			Children: []accessibility.NodeID{textCellAID, textCellBID},
		},
		&accessibility.Node{
			ID: textCellAID, Role: role.Cell, RowIndex: 0, ColumnIndex: 0, Bounds: geom.NewRect(10, 70, 10, 10),
			Text: &accessibility.TextInfo{Text: "A"},
		},
		&accessibility.Node{
			ID: textCellBID, Role: role.Cell, RowIndex: 0, ColumnIndex: 1, Bounds: geom.NewRect(10, 80, 10, 10),
			Text: &accessibility.TextInfo{Text: "B"},
		},
		// A label inside the paragraph that the composition gave no span of its own, which is what an element answered
		// with its ancestor's stretch of the text looks like.
		&accessibility.Node{ID: textLabelID, Role: role.Label, Name: "link", Bounds: geom.NewRect(70, 30, 40, 10)},
		// A button outside the document altogether, which must never be answered with any of its text.
		&accessibility.Node{ID: textButtonID, Role: role.Button, Name: "Close", Bounds: geom.NewRect(150, 10, 40, 20)},
	)
}

// textFixture returns the view of the fixture document's stream, built fresh rather than from the memo so that one
// test cannot be affected by another.
func textFixture(t *testing.T) *textDocument {
	t.Helper()
	tree := textFixtureTree()
	doc := newTextDocument(tree, tree.Node(textDocumentID))
	check.New(t).NotNil(doc)
	return doc
}

// TestTextDocumentBasics verifies what the view says about the stream as a whole: its length, the text between two
// offsets, how offsets outside it are brought inside, and what it reports about the selection and the focus.
func TestTextDocumentBasics(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	c.Equal(textFixtureLength, doc.Len())
	c.Equal(textDocumentID, doc.Node().ID)
	c.Equal("Title", doc.Text(0, 5))
	c.Equal("link", doc.Text(12, 16))
	c.Equal("￼", doc.Text(17, 18))
	c.Equal(textFixtureText, doc.Text(0, doc.Len()))

	// Every offset a client can name is brought inside the stream, and an inverted range becomes the degenerate range
	// at its start: a range whose start had crossed its end would stand for text that runs backwards.
	c.Equal(0, doc.ClampOffset(-5))
	c.Equal(textFixtureLength, doc.ClampOffset(1000))
	start, end := doc.ClampRange(-3, 1000)
	c.Equal(0, start)
	c.Equal(textFixtureLength, end)
	start, end = doc.ClampRange(20, 10)
	c.Equal(20, start)
	c.Equal(20, end)
	c.Equal("", doc.Text(20, 10))

	selStart, selEnd := doc.Selection()
	c.Equal(12, selStart)
	c.Equal(16, selEnd)
	c.Equal(12, doc.Caret())
	c.True(doc.Focused())
	c.Equal(SupportedTextSelection_Single, doc.SupportedSelection())

	// A document that does not offer the selection action has no caret to place, and must say so rather than let a
	// client discover it when Select fails.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Actions = accessibility.ActionSet(0)
	c.Equal(SupportedTextSelection_None,
		newTextDocument(tree, tree.Node(textDocumentID)).SupportedSelection())
}

// TestTextBoundaries pins the divisions of the fixture's stream for every unit. These are what a screen reader reads
// a document by, so each list is written out rather than derived: a division that quietly moved would have Narrator
// read the wrong stretch of text and nothing else would notice.
func TestTextBoundaries(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)

	characters := make([]int, 0, textFixtureLength+1)
	for i := 0; i <= textFixtureLength; i++ {
		characters = append(characters, i)
	}
	c.Equal(characters, doc.Boundaries(TextUnit_Character))

	// A word is what internal/textunit says it is, so that a word is the same stretch of text here as on AT-SPI. The
	// image is a word of its own, and the text after it begins the next: the space between them belongs to the image's
	// word rather than being a word holding nothing but a space.
	c.Equal([]int{0, 6, 12, 17, 19, 23, 25, 27, 29, 33, 35, 36}, doc.Boundaries(TextUnit_Word))

	// A line is where the layout wrapped, which is the one division that is not in the text itself: offset 17 begins a
	// line without beginning a paragraph.
	c.Equal([]int{0, 6, 17, 23, 29, 33, 35, 36}, doc.Boundaries(TextUnit_Line))

	// A paragraph is a block, which is where the composition put a line feed and where each block's own span begins.
	c.Equal([]int{0, 6, 23, 29, 33, 35, 36}, doc.Boundaries(TextUnit_Paragraph))

	// A formatting unit is where either the styling or the elements change.
	c.Equal([]int{0, 5, 6, 12, 16, 17, 18, 22, 23, 28, 29, 32, 33, 34, 35, 36}, doc.Boundaries(TextUnit_Format))

	// Nothing paginates, so a page is the document.
	c.Equal([]int{0, textFixtureLength}, doc.Boundaries(TextUnit_Page))
	c.Equal([]int{0, textFixtureLength}, doc.Boundaries(TextUnit_Document))
	c.Equal([]int{0, textFixtureLength}, doc.Boundaries(TextUnit(99)), "an unknown unit answers as the document does")
	c.True(textUnitValid(TextUnit_Character))
	c.True(textUnitValid(TextUnit_Document))
	c.False(textUnitValid(TextUnit(-1)))
	c.False(textUnitValid(TextUnit(7)))

	// The endpoints a client can name are as closed a set as the units, and for the same reason: a method handed
	// something else would move or compare the wrong end of the range and report success.
	c.True(textEndpointValid(TextPatternRangeEndpoint_Start))
	c.True(textEndpointValid(TextPatternRangeEndpoint_End))
	c.False(textEndpointValid(TextPatternRangeEndpoint(-1)))
	c.False(textEndpointValid(TextPatternRangeEndpoint(2)))

	// A document whose lines were never measured falls back to its paragraphs, since a line whose extent cannot be
	// reported is worse than a paragraph that can.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Document.Text.Lines = nil
	unmeasured := newTextDocument(tree, tree.Node(textDocumentID))
	c.Equal([]int{0, 6, 23, 29, 33, 35, 36}, unmeasured.Boundaries(TextUnit_Line),
		"an unmeasured document's lines are its paragraphs")
	c.Equal([]int{0, 6, 23, 29, 33, 35, 36}, unmeasured.Boundaries(TextUnit_Paragraph))

	// An empty stream has one boundary, which is both of its ends.
	c.Equal([]int{0}, textEmptyFixture(t).Boundaries(TextUnit_Word))
}

// textEmptyFixture returns the view of a document whose stream is empty, which is the edge case every one of the
// arithmetic methods has to survive.
func textEmptyFixture(t *testing.T) *textDocument {
	t.Helper()
	tree := newTestTree(textWindowID, 0,
		&accessibility.Node{
			ID: textWindowID, Role: role.Window, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []accessibility.NodeID{textDocumentID},
		},
		&accessibility.Node{
			ID: textDocumentID, Role: role.Document, Bounds: geom.NewRect(0, 0, 100, 20),
			Document: &accessibility.DocumentInfo{},
		},
	)
	doc := newTextDocument(tree, tree.Node(textDocumentID))
	check.New(t).NotNil(doc)
	return doc
}

// TestTextUnitStartAndEnd verifies the two questions every other operation is built out of: where the unit
// containing an offset begins, and where it ends. The end of the stream is the one boundary that begins no unit.
func TestTextUnitStartAndEnd(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	c.Equal(6, doc.UnitStart(TextUnit_Word, 8))
	c.Equal(12, doc.UnitEnd(TextUnit_Word, 8))
	c.Equal(12, doc.UnitStart(TextUnit_Word, 12), "an offset on a boundary is the start of its own unit")
	c.Equal(17, doc.UnitEnd(TextUnit_Word, 12))
	c.Equal(35, doc.UnitStart(TextUnit_Word, textFixtureLength), "the end of the stream belongs to the last unit")
	c.Equal(textFixtureLength, doc.UnitEnd(TextUnit_Word, textFixtureLength))
	c.Equal(0, doc.UnitStart(TextUnit_Line, 3))
	c.Equal(6, doc.UnitEnd(TextUnit_Line, 3))
	c.Equal(17, doc.UnitStart(TextUnit_Line, 20), "a wrapped line begins where the layout wrapped it")
	c.Equal(6, doc.UnitStart(TextUnit_Paragraph, 20), "while the paragraph it is part of began earlier")
	c.True(doc.IsUnitBoundary(TextUnit_Line, 17))
	c.False(doc.IsUnitBoundary(TextUnit_Paragraph, 17))
	c.True(doc.IsUnitBoundary(TextUnit_Word, textFixtureLength))
}

// TestTextExpand covers the eight cases ExpandToEnclosingUnit is documented to handle, which is how every client
// turns a caret position into something to read out.
func TestTextExpand(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)

	// 1. An empty stream has no unit to expand to.
	start, end := textEmptyFixture(t).Expand(TextUnit_Word, 0, 0)
	c.Equal(0, start)
	c.Equal(0, end)

	for i, one := range []struct {
		name        string
		start, end  int
		wantStart   int
		wantEnd     int
		unit        TextUnit
		description string
	}{
		{name: "degenerate inside a unit", start: 8, end: 8, unit: TextUnit_Word, wantStart: 6, wantEnd: 12},
		{name: "degenerate on a boundary", start: 12, end: 12, unit: TextUnit_Word, wantStart: 12, wantEnd: 17},
		{
			name: "degenerate at the end of the stream", start: textFixtureLength, end: textFixtureLength,
			unit: TextUnit_Word, wantStart: 35, wantEnd: textFixtureLength,
		},
		{name: "already whole units", start: 6, end: 12, unit: TextUnit_Word, wantStart: 6, wantEnd: 12},
		{name: "start inside a unit", start: 8, end: 12, unit: TextUnit_Word, wantStart: 6, wantEnd: 12},
		{name: "end inside a unit", start: 6, end: 14, unit: TextUnit_Word, wantStart: 6, wantEnd: 17},
		{name: "both inside units", start: 8, end: 14, unit: TextUnit_Word, wantStart: 6, wantEnd: 17},
		{
			name: "the document unit is the whole stream", start: 8, end: 14, unit: TextUnit_Document, wantStart: 0,
			wantEnd: textFixtureLength,
		},
		{name: "a line stops where the layout wrapped", start: 8, end: 8, unit: TextUnit_Line, wantStart: 6, wantEnd: 17},
		{
			name: "while the paragraph it is in goes on", start: 8, end: 8, unit: TextUnit_Paragraph, wantStart: 6,
			wantEnd: 23,
		},
		{name: "a character is one rune", start: 8, end: 8, unit: TextUnit_Character, wantStart: 8, wantEnd: 9},
		{
			name: "a formatting unit runs as far as the styling and the elements do", start: 8, end: 8,
			unit: TextUnit_Format, wantStart: 6, wantEnd: 12,
		},
		{
			name: "and a page is the whole stream, since nothing paginates", start: 8, end: 14, unit: TextUnit_Page,
			wantStart: 0, wantEnd: textFixtureLength,
		},
	} {
		gotStart, gotEnd := doc.Expand(one.unit, one.start, one.end)
		c.Equal(one.wantStart, gotStart, "case %d (%s) start", i, one.name)
		c.Equal(one.wantEnd, gotEnd, "case %d (%s) end", i, one.name)

		// Expanding an expanded range must not move it again: that is what lets a client walk a document by expanding
		// and then moving.
		againStart, againEnd := doc.Expand(one.unit, gotStart, gotEnd)
		c.Equal(gotStart, againStart, "case %d (%s) is stable", i, one.name)
		c.Equal(gotEnd, againEnd, "case %d (%s) is stable", i, one.name)
	}
}

// TestTextMove verifies Move, which is how a client walks a document one unit at a time, including the count it
// reports: a client knows it has reached an end of the document by being told it moved fewer units than it asked for.
func TestTextMove(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	for i, one := range []struct {
		name       string
		start, end int
		count      int
		wantStart  int
		wantEnd    int
		wantMoved  int
		unit       TextUnit
	}{
		{
			name: "a caret moves by words and stays degenerate", start: 0, end: 0, unit: TextUnit_Word, count: 2,
			wantStart: 12, wantEnd: 12, wantMoved: 2,
		},
		{
			name: "a range of one word becomes the next word", start: 6, end: 12, unit: TextUnit_Word, count: 1,
			wantStart: 12, wantEnd: 17, wantMoved: 1,
		},
		{
			name: "backwards", start: 12, end: 17, unit: TextUnit_Word, count: -1, wantStart: 6, wantEnd: 12,
			wantMoved: -1,
		},
		{
			name: "a range that covers several units is normalized to the first", start: 6, end: 22,
			unit: TextUnit_Word, count: 0, wantStart: 6, wantEnd: 12, wantMoved: 0,
		},
		{
			name: "moving past the end reports how far it got", start: 0, end: 0, unit: TextUnit_Word, count: 100,
			wantStart: textFixtureLength, wantEnd: textFixtureLength, wantMoved: 11,
		},
		{
			name: "and past the beginning", start: 0, end: 0, unit: TextUnit_Word, count: -3, wantStart: 0, wantEnd: 0,
			wantMoved: 0,
		},
		{
			name: "by line", start: 0, end: 6, unit: TextUnit_Line, count: 1, wantStart: 6, wantEnd: 17, wantMoved: 1,
		},
		{
			name: "by paragraph", start: 0, end: 6, unit: TextUnit_Paragraph, count: 2, wantStart: 23, wantEnd: 29,
			wantMoved: 2,
		},
		{
			name: "by character", start: 8, end: 8, unit: TextUnit_Character, count: 3, wantStart: 11, wantEnd: 11,
			wantMoved: 3,
		},
		// A range that covers text stops at the last unit: there is no unit past it for the range to cover, and a count
		// of zero is how a client walking a document is told it has reached the end. A caret is different — the end of
		// the stream is somewhere a caret can sit — so a degenerate range does reach it.
		{
			name: "a range at the last word cannot move past it", start: 35, end: textFixtureLength,
			unit: TextUnit_Word, count: 1, wantStart: 35, wantEnd: textFixtureLength, wantMoved: 0,
		},
		{
			name: "while a caret there reaches the end of the stream", start: 35, end: 35, unit: TextUnit_Word,
			count: 1, wantStart: textFixtureLength, wantEnd: textFixtureLength, wantMoved: 1,
		},
		{
			name: "and the document is one unit nothing moves past", start: 0, end: textFixtureLength,
			unit: TextUnit_Document, count: 1, wantStart: 0, wantEnd: textFixtureLength, wantMoved: 0,
		},
		// A caret is moved by unit boundaries rather than normalized to the beginning of the unit it is in and stepped
		// from there. Normalizing first made a backward move skip a unit and report that it had not, and made a move of
		// no units at all relocate the caret — which is what a client that starts from a raw GetSelection range, as
		// NVDA's review commands do, hands in.
		{
			name: "a caret inside a word moves back to that word's beginning", start: 8, end: 8, unit: TextUnit_Word,
			count: -1, wantStart: 6, wantEnd: 6, wantMoved: -1,
		},
		{
			name: "and back two to the word before it", start: 8, end: 8, unit: TextUnit_Word, count: -2, wantStart: 0,
			wantEnd: 0, wantMoved: -2,
		},
		{
			name: "a caret inside a line moves back to where the layout wrapped", start: 20, end: 20,
			unit: TextUnit_Line, count: -1, wantStart: 17, wantEnd: 17, wantMoved: -1,
		},
		{
			name: "a caret inside a word moves forward to the next boundary", start: 8, end: 8, unit: TextUnit_Word,
			count: 1, wantStart: 12, wantEnd: 12, wantMoved: 1,
		},
		{
			name: "no units at all leaves a caret exactly where it was", start: 8, end: 8, unit: TextUnit_Word,
			count: 0, wantStart: 8, wantEnd: 8, wantMoved: 0,
		},
		{
			name: "a caret at the end of the stream cannot move forward", start: textFixtureLength,
			end: textFixtureLength, unit: TextUnit_Word, count: 1, wantStart: textFixtureLength,
			wantEnd: textFixtureLength, wantMoved: 0,
		},
		{
			name: "however many units it is asked for", start: textFixtureLength, end: textFixtureLength,
			unit: TextUnit_Character, count: 5, wantStart: textFixtureLength, wantEnd: textFixtureLength,
			wantMoved: 0,
		},
	} {
		gotStart, gotEnd, moved := doc.Move(one.unit, one.count, one.start, one.end)
		c.Equal(one.wantStart, gotStart, "case %d (%s) start", i, one.name)
		c.Equal(one.wantEnd, gotEnd, "case %d (%s) end", i, one.name)
		c.Equal(one.wantMoved, moved, "case %d (%s) moved", i, one.name)
	}

	// An empty stream has nowhere to move to, in either direction.
	empty := textEmptyFixture(t)
	start, end, moved := empty.Move(TextUnit_Word, 5, 0, 0)
	c.Equal(0, start)
	c.Equal(0, end)
	c.Equal(0, moved)
}

// TestTextMoveEndpoint verifies MoveEndpointByUnit, which is how a client grows or shrinks a range: the first move
// in either direction lands on the nearest boundary that way whether or not the endpoint was already on one, and an
// endpoint pushed past the other takes it along.
func TestTextMoveEndpoint(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	for i, one := range []struct {
		name       string
		start, end int
		count      int
		wantStart  int
		wantEnd    int
		wantMoved  int
		unit       TextUnit
		endpoint   TextPatternRangeEndpoint
	}{
		{
			name: "the end forward by a word", start: 6, end: 12, endpoint: TextPatternRangeEndpoint_End,
			unit: TextUnit_Word, count: 1, wantStart: 6, wantEnd: 17, wantMoved: 1,
		},
		{
			name: "the start backward by a word", start: 6, end: 12, endpoint: TextPatternRangeEndpoint_Start,
			unit: TextUnit_Word, count: -1, wantStart: 0, wantEnd: 12, wantMoved: -1,
		},
		{
			name: "an endpoint that is not on a boundary snaps, which counts as one move", start: 7, end: 12,
			endpoint: TextPatternRangeEndpoint_Start, unit: TextUnit_Word, count: -1, wantStart: 6, wantEnd: 12,
			wantMoved: -1,
		},
		{
			name: "and forward", start: 6, end: 13, endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Word,
			count: 1, wantStart: 6, wantEnd: 17, wantMoved: 1,
		},
		{
			name: "the start crossing the end collapses the range", start: 6, end: 12,
			endpoint: TextPatternRangeEndpoint_Start, unit: TextUnit_Word, count: 3, wantStart: 19, wantEnd: 19,
			wantMoved: 3,
		},
		{
			name: "the end crossing the start collapses it too", start: 6, end: 12,
			endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Word, count: -1, wantStart: 6, wantEnd: 6,
			wantMoved: -1,
		},
		{
			name: "there is nowhere past the end of the stream", start: textFixtureLength,
			end: textFixtureLength, endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Word, count: 1,
			wantStart: textFixtureLength, wantEnd: textFixtureLength, wantMoved: 0,
		},
		{
			name: "nor before its beginning", start: 0, end: 6, endpoint: TextPatternRangeEndpoint_Start,
			unit: TextUnit_Word, count: -1, wantStart: 0, wantEnd: 6, wantMoved: 0,
		},
		{
			name: "a count of zero moves nothing", start: 7, end: 12, endpoint: TextPatternRangeEndpoint_Start,
			unit: TextUnit_Word, count: 0, wantStart: 7, wantEnd: 12, wantMoved: 0,
		},
		{
			name: "several units at once, clamped at the end", start: 0, end: 0,
			endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Line, count: 100, wantStart: 0,
			wantEnd: textFixtureLength, wantMoved: 7,
		},
		{
			name: "the end backward from an offset that is not a boundary snaps to the one behind it", start: 6,
			end: 20, endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Line, count: -1, wantStart: 6, wantEnd: 17,
			wantMoved: -1,
		},
		{
			name: "and the caret at the end of the stream has nowhere further to go", start: textFixtureLength,
			end: textFixtureLength, endpoint: TextPatternRangeEndpoint_Start, unit: TextUnit_Character, count: 2,
			wantStart: textFixtureLength, wantEnd: textFixtureLength, wantMoved: 0,
		},
	} {
		gotStart, gotEnd, moved := doc.MoveEndpoint(one.unit, one.endpoint, one.count, one.start, one.end)
		c.Equal(one.wantStart, gotStart, "case %d (%s) start", i, one.name)
		c.Equal(one.wantEnd, gotEnd, "case %d (%s) end", i, one.name)
		c.Equal(one.wantMoved, moved, "case %d (%s) moved", i, one.name)
	}
}

// TestTextClipping verifies the one place UI Automation counts in UTF-16 code units rather than in characters: the
// maximum length GetText is given. A character that needs two units is dropped whole rather than cut in half, since
// half of a surrogate pair is not a character.
func TestTextClipping(t *testing.T) {
	c := check.New(t)
	c.Equal("abc", TextClip("abc", -1))
	c.Equal("abc", TextClip("abc", 10))
	c.Equal("ab", TextClip("abc", 2))
	c.Equal("", TextClip("abc", 0))
	c.Equal("", TextClip("", 5))

	// An emoji is one character and two code units.
	c.Equal("a", TextClip("a\U0001F600b", 2))
	c.Equal("a\U0001F600", TextClip("a\U0001F600b", 3))
	c.Equal("a\U0001F600b", TextClip("a\U0001F600b", 4))

	// A character outside the Basic Multilingual Plane is never split, however little room is left.
	c.Equal("", TextClip("\U0001F600", 1))
}

// TestTextFindText verifies searching within a range, in both directions and with and without regard for case.
func TestTextFindText(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	start, end, ok := doc.FindText(0, doc.Len(), "link", false, false)
	c.True(ok)
	c.Equal(12, start)
	c.Equal(16, end)

	_, _, ok = doc.FindText(0, doc.Len(), "LINK", false, false)
	c.False(ok, "case matters unless the client says otherwise")
	start, end, ok = doc.FindText(0, doc.Len(), "LINK", false, true)
	c.True(ok)
	c.Equal(12, start)
	c.Equal(16, end)

	// Backwards finds the last occurrence rather than the first, which is what lets a client walk the matches either
	// way.
	start, end, ok = doc.FindText(0, doc.Len(), "e", false, false)
	c.True(ok)
	c.Equal(4, start)
	c.Equal(5, end)
	start, end, ok = doc.FindText(0, doc.Len(), "e", true, false)
	c.True(ok)
	c.Equal(31, start, "the last e in the stream is the one in the list item")
	c.Equal(32, end)

	// The search covers the range and nothing outside it.
	_, _, ok = doc.FindText(18, doc.Len(), "link", false, false)
	c.False(ok)
	_, _, ok = doc.FindText(0, doc.Len(), "not here", false, false)
	c.False(ok)
	_, _, ok = doc.FindText(0, doc.Len(), "", false, false)
	c.False(ok, "an empty string is not text to be found")
}

// TestTextAttributes verifies what a range says about itself: the value every formatting unit in it agrees on, the
// reserved mixed answer where they do not, and the reserved not-supported answer for an attribute no document here
// records.
func TestTextAttributes(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)

	// The heading, which is one uniform run.
	c.Equal(attribute{Kind: attributeString, Text: "Heading"}, doc.Attribute(0, 5, FontNameAttributeId))
	c.Equal(attribute{Kind: attributeNumber, Number: 20}, doc.Attribute(0, 5, FontSizeAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: 700}, doc.Attribute(0, 5, FontWeightAttributeId))
	c.Equal(attribute{Kind: attributeBoolean, Bool: false}, doc.Attribute(0, 5, IsItalicAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_None)},
		doc.Attribute(0, 5, UnderlineStyleAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(StyleId_Heading2)},
		doc.Attribute(0, 5, StyleIdAttributeId))

	// Every document here is read-only, nothing in a stream is hidden, and the text is active while the document holds
	// the keyboard focus.
	c.Equal(attribute{Kind: attributeBoolean, Bool: true}, doc.Attribute(0, 5, IsReadOnlyAttributeId))
	c.Equal(attribute{Kind: attributeBoolean, Bool: false}, doc.Attribute(0, 5, IsHiddenAttributeId))
	c.Equal(attribute{Kind: attributeBoolean, Bool: true}, doc.Attribute(0, 5, IsActiveAttributeId))

	// A range covering two runs that disagree is mixed, and one covering two that agree is not.
	c.Equal(attribute{Kind: attributeMixed}, doc.Attribute(0, 12, FontNameAttributeId))
	c.Equal(attribute{Kind: attributeString, Text: "Body"}, doc.Attribute(5, 23, FontNameAttributeId))
	c.Equal(attribute{Kind: attributeMixed}, doc.Attribute(5, 23, UnderlineStyleAttributeId),
		"the link within it is underlined and the rest is not")

	// The link.
	c.Equal(attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_Single)},
		doc.Attribute(12, 16, UnderlineStyleAttributeId))
	c.Equal(attribute{Kind: attributeRange, Start: 12, End: 16}, doc.Attribute(12, 16, LinkAttributeId))
	c.Equal(attribute{Kind: attributeRange, Start: 12, End: 16}, doc.Attribute(13, 14, LinkAttributeId),
		"part of a link is still that link")
	c.Equal(attribute{Kind: attributeEmpty}, doc.Attribute(6, 12, LinkAttributeId),
		"text that is not part of a link has none, which is an empty answer rather than a refusal")
	c.Equal(attribute{Kind: attributeMixed}, doc.Attribute(6, 16, LinkAttributeId))

	// The code block is the one style UI Automation has no identifier for, so it is custom and carries a name.
	c.Equal(attribute{Kind: attributeInteger, Int: int32(StyleId_Custom)},
		doc.Attribute(23, 28, StyleIdAttributeId))
	c.Equal(attribute{Kind: attributeString, Text: "Code"}, doc.Attribute(23, 28, StyleNameAttributeId))
	c.Equal(attribute{Kind: attributeBoolean, Bool: true}, doc.Attribute(23, 28, IsReadOnlyAttributeId))
	c.Equal(attribute{Kind: attributeUnsupported}, doc.Attribute(0, 5, StyleNameAttributeId),
		"a style with an identifier of its own needs no name")

	// The list item and the table cells.
	c.Equal(attribute{Kind: attributeInteger, Int: int32(StyleId_BulletedList)},
		doc.Attribute(29, 32, StyleIdAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(StyleId_Normal)},
		doc.Attribute(33, 34, StyleIdAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(StyleId_Normal)},
		doc.Attribute(12, 16, StyleIdAttributeId), "a link is body text unless something around it says otherwise")

	// The last cell, which is the one italic, struck-through run.
	c.Equal(attribute{Kind: attributeBoolean, Bool: true}, doc.Attribute(35, 36, IsItalicAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_Single)},
		doc.Attribute(35, 36, StrikethroughStyleAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_None)},
		doc.Attribute(33, 34, StrikethroughStyleAttributeId), "while the cell before it is struck through by nothing")
	c.Equal(attribute{Kind: attributeMixed}, doc.Attribute(33, 36, IsItalicAttributeId),
		"a range covering both cells agrees about neither")
	c.Equal(attribute{Kind: attributeMixed}, doc.Attribute(33, 36, StrikethroughStyleAttributeId))
	start, end, ok := doc.FindAttribute(0, doc.Len(), IsItalicAttributeId,
		attribute{Kind: attributeBoolean, Bool: true}, false)
	c.True(ok, "the italic stretch can be jumped to")
	c.Equal(35, start)
	c.Equal(textFixtureLength, end)

	// A degenerate range answers from the unit its offset is in rather than being mixed.
	c.Equal(attribute{Kind: attributeString, Text: "Mono"}, doc.Attribute(25, 25, FontNameAttributeId))

	// The attributes no document here records are refused rather than guessed at.
	c.Equal(attribute{Kind: attributeUnsupported}, doc.Attribute(0, 5, CultureAttributeId))
	c.Equal(attribute{Kind: attributeUnsupported}, doc.Attribute(0, 5, AnnotationTypesAttributeId))
	c.Equal(attribute{Kind: attributeUnsupported}, doc.Attribute(0, 5, TextAttributeID(40099)))

	// A document that does not hold the focus has inactive text, and one whose runs were never filled in has no font to
	// report — but still says what it can. The focus is the snapshot's to name, which is why the flag on the node is
	// not what moves it.
	tree := textFixtureTree()
	tree.Focus = 0
	tree.Node(textDocumentID).Document.Text.Runs = nil
	plain := newTextDocument(tree, tree.Node(textDocumentID))
	c.Equal(attribute{Kind: attributeBoolean, Bool: false}, plain.Attribute(0, 5, IsActiveAttributeId))
	c.Equal(attribute{Kind: attributeUnsupported}, plain.Attribute(0, 5, FontNameAttributeId))
	c.Equal(attribute{Kind: attributeUnsupported}, plain.Attribute(0, 5, FontSizeAttributeId))
	c.Equal(attribute{Kind: attributeUnsupported}, plain.Attribute(0, 5, FontWeightAttributeId))
	c.Equal(attribute{Kind: attributeBoolean, Bool: false}, plain.Attribute(0, 5, IsItalicAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_None)},
		plain.Attribute(0, 5, StrikethroughStyleAttributeId))
	c.Equal(attribute{Kind: attributeInteger, Int: int32(StyleId_Heading2)},
		plain.Attribute(0, 5, StyleIdAttributeId), "a style comes from the elements rather than from the runs")

	// A heading with no level is still a heading, and one deeper than UI Automation counts reports the deepest level.
	c.Equal(StyleId_Heading1, headingStyle(0))
	c.Equal(StyleId_Heading1, headingStyle(1))
	c.Equal(StyleId_Heading9, headingStyle(9))
	c.Equal(StyleId_Heading9, headingStyle(20))
}

// TestTextFindAttribute verifies searching by attribute, which is how a client jumps to the next link or heading
// without reading everything in between.
func TestTextFindAttribute(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)

	// The heading is the first stretch styled as one.
	start, end, ok := doc.FindAttribute(0, doc.Len(),
		StyleIdAttributeId, attribute{Kind: attributeInteger, Int: int32(StyleId_Heading2)}, false)
	c.True(ok)
	c.Equal(0, start)
	c.Equal(5, end)

	// The link is the only underlined stretch.
	start, end, ok = doc.FindAttribute(0, doc.Len(), UnderlineStyleAttributeId,
		attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_Single)}, false)
	c.True(ok)
	c.Equal(12, start)
	c.Equal(16, end)

	// A stretch runs as far as the attribute goes on agreeing, and searching backward finds the last one rather than
	// the first.
	start, end, ok = doc.FindAttribute(0, doc.Len(), FontNameAttributeId,
		attribute{Kind: attributeString, Text: "Body"}, false)
	c.True(ok)
	c.Equal(5, start)
	c.Equal(23, end)
	start, end, ok = doc.FindAttribute(0, doc.Len(), FontNameAttributeId,
		attribute{Kind: attributeString, Text: "Body"}, true)
	c.True(ok)
	c.Equal(28, start)
	c.Equal(textFixtureLength, end)

	// Nothing in the range has it.
	_, _, ok = doc.FindAttribute(0, 5, FontNameAttributeId,
		attribute{Kind: attributeString, Text: "Mono"}, false)
	c.False(ok)

	// A value of a kind no stretch of text can have matches nothing, and neither does a degenerate range.
	_, _, ok = doc.FindAttribute(0, doc.Len(), LinkAttributeId,
		attribute{Kind: attributeRange, Start: 12, End: 16}, false)
	c.False(ok)
	_, _, ok = doc.FindAttribute(0, doc.Len(), FontNameAttributeId, attribute{Kind: attributeMixed}, false)
	c.False(ok)
	_, _, ok = doc.FindAttribute(12, 12, UnderlineStyleAttributeId,
		attribute{Kind: attributeInteger, Int: int32(TextDecorationLineStyle_Single)}, false)
	c.False(ok)
	c.True(attributeSearchable(attributeBoolean))
	c.False(attributeSearchable(attributeEmpty))
	c.False(attributeSearchable(attributeUnsupported))
}

// TestTextEnclosingAndChildren verifies the two questions a client walking a document by element asks of a range:
// what encloses it, and what is inside it.
func TestTextEnclosingAndChildren(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	c.Equal(textLinkID, doc.EnclosingSpan(12, 16))
	c.Equal(textLinkID, doc.EnclosingSpan(13, 14), "part of a link is still inside it")
	c.Equal(textLinkID, doc.EnclosingSpan(13, 13), "a caret is treated as the character that follows it")
	c.Equal(textParagraphID, doc.EnclosingSpan(6, 22))
	c.Equal(textParagraphID, doc.EnclosingSpan(6, 16))
	c.Equal(textHeadingID, doc.EnclosingSpan(0, 5))
	c.Equal(textCellAID, doc.EnclosingSpan(33, 34))
	c.Equal(textDocumentID, doc.EnclosingSpan(0, doc.Len()), "nothing but the document covers the whole stream")
	c.Equal(textDocumentID, doc.EnclosingSpan(5, 6), "nor the line feed between two blocks")

	// One level down, rather than everything within the range: a client narrows to a child and asks again.
	c.Equal([]accessibility.NodeID{
		textHeadingID, textParagraphID, textCodeID, textListID, textTableID,
	}, doc.Children(0, doc.Len()))
	c.Equal([]accessibility.NodeID{textLinkID, textImageID}, doc.Children(6, 22))
	c.Equal([]accessibility.NodeID{textLinkID}, doc.Children(6, 16), "and only the ones the range reaches")
	c.Equal([]accessibility.NodeID{textCellAID, textCellBID}, doc.Children(33, 36))
	c.Nil(doc.Children(12, 16), "a link holds no elements of its own")
	c.Nil(doc.Children(13, 13), "a degenerate range covers no text and so has no children")
}

// TestTextSpanFor verifies the answer ITextProvider::RangeFromChild and ITextChildProvider::get_TextRange are built
// from: which stretch of the stream an element occupies.
func TestTextSpanFor(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	start, end, ok := doc.SpanFor(textLinkID)
	c.True(ok)
	c.Equal(12, start)
	c.Equal(16, end)

	start, end, ok = doc.SpanFor(textImageID)
	c.True(ok)
	c.Equal(17, start)
	c.Equal(18, end)

	// An element the composition gave no stretch of its own is answered with its nearest ancestor's, which is the
	// smallest stretch of text that is certainly its.
	start, end, ok = doc.SpanFor(textLabelID)
	c.True(ok)
	c.Equal(6, start)
	c.Equal(22, end)

	// The document itself has no stretch, and neither has anything outside it.
	_, _, ok = doc.SpanFor(textDocumentID)
	c.False(ok)
	_, _, ok = doc.SpanFor(textWindowID)
	c.False(ok)
	_, _, ok = doc.SpanFor(textButtonID)
	c.False(ok)
	_, _, ok = doc.SpanFor(0)
	c.False(ok)
}

// TestTextContainerFor verifies what grants the TextChild pattern: sitting inside a document that claims some of its
// text.
func TestTextContainerFor(t *testing.T) {
	c := check.New(t)
	tree := textFixtureTree()
	for _, id := range []accessibility.NodeID{
		textHeadingID, textParagraphID, textLinkID, textImageID, textCodeID, textListID,
		textListItemID, textTableID, textRowID, textCellAID, textCellBID, textLabelID,
	} {
		c.Equal(textDocumentID, textContainerFor(tree, tree.Node(id)), "node %d", id)
	}

	// The document is the container rather than a child of one, and nothing outside it has a container at all.
	c.Equal(accessibility.NodeID(0), textContainerFor(tree, tree.Node(textDocumentID)))
	c.Equal(accessibility.NodeID(0), textContainerFor(tree, tree.Node(textScrollID)))
	c.Equal(accessibility.NodeID(0), textContainerFor(tree, tree.Node(textWindowID)))
	c.Equal(accessibility.NodeID(0), textContainerFor(tree, tree.Node(textButtonID)))
	c.Equal(accessibility.NodeID(0), textContainerFor(nil, tree.Node(textLinkID)))
	c.Equal(accessibility.NodeID(0), textContainerFor(tree, nil))

	// A panel inside the document that the composition passed over has no stretch of text to report, so it reports no
	// pattern either.
	tree.Nodes[textDocumentID].Children = append(tree.Nodes[textDocumentID].Children, 20)
	tree.Nodes[20] = &accessibility.Node{ID: 20, Role: role.Separator, Parent: textDocumentID}
	c.Equal(accessibility.NodeID(0), textContainerFor(tree, tree.Node(20)))

	// A Document that carries no stream is no container at all: it hands out no Text pattern for anything to be a child
	// of.
	plain := textFixtureTree()
	plain.Node(textDocumentID).Document = nil
	c.Equal(accessibility.NodeID(0), textContainerFor(plain, plain.Node(textLinkID)))
}

// TestTextRectangles verifies the rectangles a client draws its highlight from: one per line the range covers, in
// window coordinates, clipped to what the scroll area really shows.
func TestTextRectangles(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)

	// The heading, which is the first five characters of the first line. The document's own origin is added, and the
	// advances pick out the covered part of the line.
	c.Equal([]geom.Rect{geom.NewRect(10, 20, 50, 10)}, doc.Rectangles(0, 5))

	// Part of a line.
	c.Equal([]geom.Rect{geom.NewRect(30, 20, 20, 10)}, doc.Rectangles(2, 4))

	// A range crossing a wrap contributes one rectangle per line.
	c.Equal([]geom.Rect{
		geom.NewRect(10, 30, 110, 10),
		geom.NewRect(10, 40, 50, 10),
	}, doc.Rectangles(6, 22))

	// A line the scroll area only half shows is clipped to what is visible.
	c.Equal([]geom.Rect{geom.NewRect(10, 50, 50, 5)}, doc.Rectangles(23, 28))

	// Nothing below the scroll area's bottom edge has a rectangle at all.
	c.Nil(doc.Rectangles(29, doc.Len()))

	// A degenerate range covers no text, which UI Automation reports as no rectangles.
	c.Nil(doc.Rectangles(12, 12))

	// A document scrolled out of view has none either.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Offscreen = true
	c.Nil(newTextDocument(tree, tree.Node(textDocumentID)).Rectangles(0, 5))
}

// TestTextVisibleRange verifies what GetVisibleRanges reports: the stretch of the stream the user can actually see,
// which is what a client reading what is on screen starts from.
func TestTextVisibleRange(t *testing.T) {
	c := check.New(t)
	start, end, ok := textFixture(t).VisibleRange()
	c.True(ok)
	c.Equal(0, start)
	c.Equal(29, end, "the scroll area shows the first four lines and clips the rest")

	// A document scrolled out of view shows nothing.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Offscreen = true
	_, _, ok = newTextDocument(tree, tree.Node(textDocumentID)).VisibleRange()
	c.False(ok)

	// One clipped away entirely by an ancestor shows nothing either.
	clipped := textFixtureTree()
	clipped.Node(textScrollID).Bounds = geom.NewRect(500, 500, 10, 10)
	_, _, ok = newTextDocument(clipped, clipped.Node(textDocumentID)).VisibleRange()
	c.False(ok)

	// A document whose lines were never measured reports all of itself while it is on screen at all, since there is no
	// telling which part of it is where.
	unmeasured := textFixtureTree()
	unmeasured.Node(textDocumentID).Document.Text.Lines = nil
	start, end, ok = newTextDocument(unmeasured, unmeasured.Node(textDocumentID)).VisibleRange()
	c.True(ok)
	c.Equal(0, start)
	c.Equal(textFixtureLength, end)
}

// TestTextOffsetAt verifies the hit test a client placing the caret with the mouse goes through. Every point has an
// answer, since RangeFromPoint is documented to report the nearest range rather than to fail.
func TestTextOffsetAt(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)

	// Ten units per rune, measured from the document's own top left corner at (10,20).
	c.Equal(0, doc.OffsetAt(geom.NewPoint(10, 20)))
	c.Equal(2, doc.OffsetAt(geom.NewPoint(35, 25)))
	c.Equal(4, doc.OffsetAt(geom.NewPoint(55, 25)))
	c.Equal(12, doc.OffsetAt(geom.NewPoint(10+60, 20+15)), "the second line begins at offset 6")

	// A point past the end of a line answers with that line's last character rather than with nothing, which for a line
	// that ends in the zero-width line feed joining two blocks is the offset just past its visible text — where a caret
	// clicked past the end of a line belongs.
	c.Equal(5, doc.OffsetAt(geom.NewPoint(500, 25)))

	// A point above the text answers with the first line, and one below it with the last.
	c.Equal(0, doc.OffsetAt(geom.NewPoint(10, -100)))
	c.Equal(35, doc.OffsetAt(geom.NewPoint(15, 500)))

	// A document whose lines were never measured has nothing to place a point against.
	tree := textFixtureTree()
	tree.Node(textDocumentID).Document.Text.Lines = nil
	c.Equal(0, newTextDocument(tree, tree.Node(textDocumentID)).OffsetAt(geom.NewPoint(50, 50)))
}

// TestTextVisibleBounds verifies the clipping every rectangle and the visible range go through: a node's bounds
// intersected with every ancestor's, which is what a scroll area does to its content.
func TestTextVisibleBounds(t *testing.T) {
	c := check.New(t)
	tree := textFixtureTree()
	c.Equal(geom.NewRect(10, 20, 110, 35), visibleBounds(tree, tree.Node(textDocumentID)))
	c.Equal(geom.NewRect(0, 0, 200, 200), visibleBounds(tree, tree.Node(textWindowID)))

	// An offscreen node has no visible area, and neither has one an ancestor clips away entirely.
	offscreen := textFixtureTree()
	offscreen.Node(textDocumentID).Offscreen = true
	c.True(visibleBounds(offscreen, offscreen.Node(textDocumentID)).Empty())
	clipped := textFixtureTree()
	clipped.Node(textScrollID).Bounds = geom.NewRect(500, 500, 10, 10)
	c.True(visibleBounds(clipped, clipped.Node(textDocumentID)).Empty())

	// An ancestor with no bounds filled in is passed over rather than clipping everything away.
	unbounded := textFixtureTree()
	unbounded.Node(textScrollID).Bounds = geom.Rect{}
	c.Equal(geom.NewRect(10, 20, 110, 70), visibleBounds(unbounded, unbounded.Node(textDocumentID)))

	c.True(visibleBounds(nil, tree.Node(textDocumentID)).Empty())
	c.True(visibleBounds(tree, nil).Empty())
}

// TestTextDocumentMemo verifies that the view of a document's stream is worked out once per snapshot: a client
// reading a document asks the same questions thousands of times, and dividing the stream again for each of them would
// cost a pass over the whole document every time.
func TestTextDocumentMemo(t *testing.T) {
	c := check.New(t)
	forgetSnapshotMemo()
	t.Cleanup(forgetSnapshotMemo)
	tree := textFixtureTree()
	first := memoizedTextDocument(tree, textDocumentID)
	c.NotNil(first)
	c.True(first == memoizedTextDocument(tree, textDocumentID), "the same snapshot is answered from the memo")

	// A different snapshot gets a view of its own, and the memo moves on to it.
	next := textFixtureTree()
	next.Generation = 2
	second := memoizedTextDocument(next, textDocumentID)
	c.NotNil(second)
	c.False(first == second)
	c.True(second == memoizedTextDocument(next, textDocumentID))

	// A snapshot the memo has moved past is answered from itself rather than taking the memo back, which is what keeps
	// the memo on the snapshot a client is walking; see memoSwitchTo.
	third := memoizedTextDocument(tree, textDocumentID)
	c.NotNil(third)
	c.False(third == second)
	c.True(second == memoizedTextDocument(next, textDocumentID))

	// Nothing but a Document with a stream has a view.
	c.Nil(memoizedTextDocument(tree, textLinkID))
	c.Nil(memoizedTextDocument(tree, 0))
	c.Nil(memoizedTextDocument(nil, textDocumentID))
	plain := textFixtureTree()
	plain.Node(textDocumentID).Document = nil
	c.Nil(memoizedTextDocument(plain, textDocumentID))
	c.Nil(newTextDocument(plain, plain.Node(textDocumentID)))
	c.Nil(newTextDocument(nil, nil))

	// Forgetting the snapshot drops the views with it, so that a window nothing is answering from any more is not kept
	// alive by them.
	forgetSnapshotMemo()
	c.False(second == memoizedTextDocument(next, textDocumentID))
}

// TestTextSpanIndexMemo verifies that where each element sits in a document's stream is worked out once per
// snapshot. ProvidedPatterns asks it of every element a client so much as looks at, in both of the snapshots a
// publish compares, so answering it from a document's span list each time costs a scan of the whole document per
// property query.
func TestTextSpanIndexMemo(t *testing.T) {
	c := check.New(t)
	forgetSnapshotMemo()
	t.Cleanup(forgetSnapshotMemo)
	tree := textFixtureTree()
	c.Equal(textDocumentID, textContainerFor(tree, tree.Node(textLinkID)))
	c.True(snapshotMemo.textSpansDone, "asking where one element sits indexes the whole snapshot")
	c.Equal(documentSpan{document: textDocumentID, start: 12, end: 16}, snapshotMemo.textSpans[textLinkID])
	c.Equal(len(textFixtureSpans()), len(snapshotMemo.textSpans), "one entry per span the document records")
	_, held := snapshotMemo.textSpans[textLabelID]
	c.False(held, "an element the composition passed over has no span of its own")

	// The next answer comes from the index rather than from the spans: an entry put into it by hand is what is answered
	// with, and nothing else could produce that answer, since the button sits outside the document altogether.
	snapshotMemo.textSpans[textButtonID] = documentSpan{document: textDocumentID, start: 1, end: 2}
	start, end, ok := textSpanFor(tree, tree.Node(textDocumentID), textButtonID)
	c.True(ok)
	c.Equal(1, start)
	c.Equal(2, end)

	// A different snapshot is indexed afresh, so the sentinel stays with the snapshot it was put into.
	next := textFixtureTree()
	next.Generation = 2
	c.Equal(textDocumentID, textContainerFor(next, next.Node(textLinkID)))
	c.Equal(next, snapshotMemo.tree)
	_, held = snapshotMemo.textSpans[textButtonID]
	c.False(held)
	_, _, ok = textSpanFor(next, next.Node(textDocumentID), textButtonID)
	c.False(ok, "an element outside the document occupies none of its text")

	// A snapshot the memo has moved past is answered from its own spans and does not take the memo back; see
	// memoSwitchTo.
	c.Nil(memoizedTextSpans(tree), "such a snapshot is not indexed at all")
	c.Equal(next, snapshotMemo.tree)
	c.Equal(textDocumentID, textContainerFor(tree, tree.Node(textLinkID)),
		"but it is answered exactly as it would have been")
	spanStart, spanEnd, spanOK := textSpanFor(tree, tree.Node(textDocumentID), textLinkID)
	c.True(spanOK)
	c.Equal(12, spanStart)
	c.Equal(16, spanEnd)

	// Forgetting the snapshot drops the index with it, and a snapshot with no document to index allocates no map at
	// all.
	forgetSnapshotMemo()
	c.False(snapshotMemo.textSpansDone)
	c.Nil(snapshotMemo.textSpans)
	plain := textFixtureTree()
	plain.Node(textDocumentID).Document = nil
	c.Nil(memoizedTextSpans(plain))
	c.True(snapshotMemo.textSpansDone)
	c.Equal(accessibility.NodeID(0), textContainerFor(plain, plain.Node(textLinkID)))
}

// TestTextSpansSkipElementsWithNoProvider verifies that the elements a client cannot be handed are left out of the
// stream's spans: an ignored node has no provider, so GetEnclosingElement and GetChildren must answer with whatever
// contains it instead.
func TestTextSpansSkipElementsWithNoProvider(t *testing.T) {
	c := check.New(t)
	tree := textFixtureTree()
	tree.Node(textLinkID).Ignored = true
	doc := newTextDocument(tree, tree.Node(textDocumentID))
	c.Equal(textParagraphID, doc.EnclosingSpan(12, 16))
	c.Equal([]accessibility.NodeID{textImageID}, doc.Children(6, 22))

	// A span whose node the snapshot no longer holds at all is left out the same way.
	missing := textFixtureTree()
	delete(missing.Nodes, textImageID)
	doc = newTextDocument(missing, missing.Node(textDocumentID))
	c.Equal([]accessibility.NodeID{textLinkID}, doc.Children(6, 22))
}

// TestTextWalksTheWholeDocument follows what a screen reader reading a document from top to bottom actually does:
// expand the document range to the first unit, read it, move by one unit, read again, and stop when the move reports
// that it could not go as far as it asked. Every unit of the stream has to come out exactly once, in order, or a person
// hears a line twice, misses one, or never reaches the end.
func TestTextWalksTheWholeDocument(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	for _, one := range []struct {
		name     string
		expected []string
		unit     TextUnit
	}{
		{
			name: "by line", unit: TextUnit_Line,
			expected: []string{"Title\n", "Hello link ", "￼ end\n", "x = 1\n", "one\n", "A\n", "B"},
		},
		{
			name: "by paragraph", unit: TextUnit_Paragraph,
			expected: []string{"Title\n", "Hello link ￼ end\n", "x = 1\n", "one\n", "A\n", "B"},
		},
		{
			name: "by word", unit: TextUnit_Word,
			expected: []string{
				"Title\n", "Hello ", "link ", "￼ ", "end\n", "x ", "= ", "1\n", "one\n", "A\n", "B",
			},
		},
	} {
		// A client starts from the document range, collapses it to its start and expands to the first unit, which is
		// what Narrator's scan mode and NVDA's say-all both do.
		start, end := doc.Expand(one.unit, 0, 0)
		var read []string
		for range len(one.expected) + 5 {
			read = append(read, doc.Text(start, end))
			var moved int
			start, end, moved = doc.Move(one.unit, 1, start, end)
			if moved != 1 {
				break
			}
		}
		c.Equal(one.expected, read, one.name)
		c.Equal(strings.Join(one.expected, ""), textFixtureText, "%s must cover the stream exactly once", one.name)
	}
}

// TestTextCaretWalkReachesTheEnd follows the other walk a client makes over a document: stepping a caret rather than
// a range, which is what NVDA's say-all reader and every review cursor do. Nothing but the count Move reports tells
// such a client where the end of the document is, so a caret that has arrived there has to report that it did not move
// — for every unit, since a client picks the unit. Reporting a move it did not make leaves the reader stepping forever.
func TestTextCaretWalkReachesTheEnd(t *testing.T) {
	c := check.New(t)
	doc := textFixture(t)
	for _, unit := range []TextUnit{
		TextUnit_Character, TextUnit_Format, TextUnit_Word, TextUnit_Line, TextUnit_Paragraph, TextUnit_Page,
		TextUnit_Document,
	} {
		start, end, moved := doc.Move(unit, 1, textFixtureLength, textFixtureLength)
		c.Equal(textFixtureLength, start, "unit %d", unit)
		c.Equal(textFixtureLength, end, "unit %d stays degenerate", unit)
		c.Equal(0, moved, "a caret at the end of the stream has nowhere to go, unit %d", unit)

		// The whole walk, from a caret at the beginning of the stream. Every step has to advance and to be counted, and
		// the walk has to stop at the end. The iteration count is bounded only so that a regression fails instead of
		// hanging: one step per character is as many as any unit can need.
		at, steps := 0, 0
		for range textFixtureLength + 2 {
			next, _, movedOne := doc.Move(unit, 1, at, at)
			if movedOne == 0 {
				break
			}
			c.Equal(1, movedOne, "unit %d", unit)
			c.True(next > at, "a caret moving forward must advance, unit %d", unit)
			at, steps = next, steps+1
		}
		c.Equal(textFixtureLength, at, "the walk ends at the end of the stream, unit %d", unit)
		c.Equal(len(doc.Boundaries(unit))-1, steps, "one step per unit of the stream, unit %d", unit)

		// And backwards from there to the beginning, which is the same walk in reverse.
		for range textFixtureLength + 2 {
			next, _, movedOne := doc.Move(unit, -1, at, at)
			if movedOne == 0 {
				break
			}
			c.Equal(-1, movedOne, "unit %d", unit)
			c.True(next < at, "a caret moving backward must retreat, unit %d", unit)
			at = next
		}
		c.Equal(0, at, "the backward walk ends at the beginning of the stream, unit %d", unit)
	}
}
