// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

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
	uiaTextFixtureText   = "Title\nHello link ￼ end\nx = 1\none\nA\nB"
	uiaTextFixtureLength = 36
)

// The ids of the fixture's nodes.
const (
	uiaTextWindowID    = accessibility.NodeID(1)
	uiaTextScrollID    = accessibility.NodeID(2)
	uiaTextDocumentID  = accessibility.NodeID(3)
	uiaTextHeadingID   = accessibility.NodeID(4)
	uiaTextParagraphID = accessibility.NodeID(5)
	uiaTextLinkID      = accessibility.NodeID(6)
	uiaTextImageID     = accessibility.NodeID(7)
	uiaTextCodeID      = accessibility.NodeID(8)
	uiaTextListID      = accessibility.NodeID(9)
	uiaTextListItemID  = accessibility.NodeID(10)
	uiaTextTableID     = accessibility.NodeID(11)
	uiaTextRowID       = accessibility.NodeID(12)
	uiaTextCellAID     = accessibility.NodeID(13)
	uiaTextCellBID     = accessibility.NodeID(14)
	uiaTextLabelID     = accessibility.NodeID(15)
	uiaTextButtonID    = accessibility.NodeID(16)
)

// uiaTextLine builds one visual line of the fixture: ten units per rune, so that an offset and a coordinate can be read
// off each other. The line feed a block is joined to the next with belongs to the line it ends and is zero-width, which
// is why a line's last two advances are equal.
func uiaTextLine(start, end int, y float32) accessibility.Line {
	runes := end - start
	advances := make([]float32, 0, runes+1)
	width := float32(0)
	for i := 0; i <= runes; i++ {
		advances = append(advances, width)
		if i < runes && uiaTextFixtureRunes()[start+i] != '\n' {
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

// uiaTextFixtureRunes returns the fixture's stream as runes.
func uiaTextFixtureRunes() []rune {
	return []rune(uiaTextFixtureText)
}

// uiaTextFixtureSpans returns the fixture's spans, in the order accessibility.TextInfo documents: outermost first, then
// by start offset. The table's row is a virtual node, exactly as a Markdown table publishes one.
func uiaTextFixtureSpans() []accessibility.TextSpan {
	return []accessibility.TextSpan{
		{Node: uiaTextHeadingID, Start: 0, End: 5},
		{Node: uiaTextParagraphID, Start: 6, End: 22},
		{Node: uiaTextCodeID, Start: 23, End: 28},
		{Node: uiaTextListID, Start: 29, End: 32},
		{Node: uiaTextTableID, Start: 33, End: 36},
		{Node: uiaTextLinkID, Start: 12, End: 16},
		{Node: uiaTextImageID, Start: 17, End: 18},
		{Node: uiaTextListItemID, Start: 29, End: 32},
		{Node: uiaTextRowID, Start: 33, End: 36},
		{Node: uiaTextCellAID, Start: 33, End: 34},
		{Node: uiaTextCellBID, Start: 35, End: 36},
	}
}

// uiaTextFixtureRuns returns the fixture's styled runs, which tile the stream: a bold heading in one family, body text
// in another, an underlined link within it, a monospaced code block, and a last table cell whose text is italic and
// struck through, which is the one run either of those two attributes is true over. The split at offset 35 adds no
// formatting boundary of its own, since that cell's span already begins there.
func uiaTextFixtureRuns() []accessibility.TextRun {
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

// uiaTextFixtureTree builds the snapshot the text tests work over: a window holding a scroll area, holding the
// document. The scroll area is shorter than the document, so the last three lines are clipped away, which is what makes
// the visible range and the clipped rectangles worth testing. The document holds the keyboard focus, which is what the
// IsActive attribute and GetCaretRange report.
//
// The document's bounds are offset from the window's origin so that a conversion that forgets to add them shows up, and
// its lines are measured from its own top left corner, as a snapshot records them.
func uiaTextFixtureTree() *accessibility.Tree {
	document := &accessibility.Node{
		ID: uiaTextDocumentID, Role: role.Document, Name: "Notes", Focusable: true, Focused: true,
		Bounds: geom.NewRect(10, 20, 110, 70),
		Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection, accessibility.ScrollRangeIntoView,
			accessibility.ShowContextMenu, accessibility.ScrollIntoView),
		Children: []accessibility.NodeID{
			uiaTextHeadingID, uiaTextParagraphID, uiaTextCodeID, uiaTextListID, uiaTextTableID,
		},
		Document: &accessibility.DocumentInfo{
			Text: accessibility.TextInfo{
				Text: uiaTextFixtureText,
				Lines: []accessibility.Line{
					uiaTextLine(0, 6, 0),
					uiaTextLine(6, 17, 10),
					uiaTextLine(17, 23, 20),
					uiaTextLine(23, 29, 30),
					uiaTextLine(29, 33, 40),
					uiaTextLine(33, 35, 50),
					uiaTextLine(35, 36, 60),
				},
				Runs:      uiaTextFixtureRuns(),
				Spans:     uiaTextFixtureSpans(),
				SelStart:  12,
				SelEnd:    16,
				Caret:     12,
				Multiline: true,
			},
		},
	}
	return newTestTree(uiaTextWindowID, uiaTextDocumentID,
		&accessibility.Node{
			ID: uiaTextWindowID, Role: role.Window, Name: "Window", Focused: true,
			Bounds:   geom.NewRect(0, 0, 200, 200),
			Children: []accessibility.NodeID{uiaTextScrollID, uiaTextButtonID},
		},
		&accessibility.Node{
			ID: uiaTextScrollID, Role: role.ScrollArea, Bounds: geom.NewRect(10, 20, 110, 35),
			Children: []accessibility.NodeID{uiaTextDocumentID},
		},
		document,
		&accessibility.Node{
			ID: uiaTextHeadingID, Role: role.Heading, Name: "Title", Level: 2, Bounds: geom.NewRect(10, 20, 50, 10),
		},
		&accessibility.Node{
			ID: uiaTextParagraphID, Role: role.Paragraph, Bounds: geom.NewRect(10, 30, 110, 20),
			Children: []accessibility.NodeID{uiaTextLinkID, uiaTextImageID, uiaTextLabelID},
			Text:     &accessibility.TextInfo{Text: "Hello link ￼ end"},
		},
		&accessibility.Node{
			ID: uiaTextLinkID, Role: role.Link, Name: "link", URL: "https://example.com",
			Bounds: geom.NewRect(70, 30, 40, 10),
		},
		&accessibility.Node{ID: uiaTextImageID, Role: role.Image, Name: "A picture", Bounds: geom.NewRect(10, 40, 10, 10)},
		&accessibility.Node{
			ID: uiaTextCodeID, Role: role.Code, Bounds: geom.NewRect(10, 50, 50, 10),
			Text: &accessibility.TextInfo{Text: "x = 1"},
		},
		&accessibility.Node{
			ID: uiaTextListID, Role: role.List, Bounds: geom.NewRect(10, 60, 30, 10),
			Children: []accessibility.NodeID{uiaTextListItemID},
		},
		&accessibility.Node{ID: uiaTextListItemID, Role: role.ListItem, Bounds: geom.NewRect(10, 60, 30, 10)},
		&accessibility.Node{
			ID: uiaTextTableID, Role: role.Table, Bounds: geom.NewRect(10, 70, 10, 20), RowCount: 1, ColumnCount: 2,
			Children: []accessibility.NodeID{uiaTextRowID},
		},
		&accessibility.Node{
			ID: uiaTextRowID, Role: role.Row, RowIndex: 0, Bounds: geom.NewRect(10, 70, 10, 20),
			Children: []accessibility.NodeID{uiaTextCellAID, uiaTextCellBID},
		},
		&accessibility.Node{
			ID: uiaTextCellAID, Role: role.Cell, RowIndex: 0, ColumnIndex: 0, Bounds: geom.NewRect(10, 70, 10, 10),
			Text: &accessibility.TextInfo{Text: "A"},
		},
		&accessibility.Node{
			ID: uiaTextCellBID, Role: role.Cell, RowIndex: 0, ColumnIndex: 1, Bounds: geom.NewRect(10, 80, 10, 10),
			Text: &accessibility.TextInfo{Text: "B"},
		},
		// A label inside the paragraph that the composition gave no span of its own, which is what an element answered
		// with its ancestor's stretch of the text looks like.
		&accessibility.Node{ID: uiaTextLabelID, Role: role.Label, Name: "link", Bounds: geom.NewRect(70, 30, 40, 10)},
		// A button outside the document altogether, which must never be answered with any of its text.
		&accessibility.Node{ID: uiaTextButtonID, Role: role.Button, Name: "Close", Bounds: geom.NewRect(150, 10, 40, 20)},
	)
}

// uiaTextFixture returns the view of the fixture document's stream, built fresh rather than from the memo so that one
// test cannot be affected by another.
func uiaTextFixture(t *testing.T) *uiaTextDocument {
	t.Helper()
	tree := uiaTextFixtureTree()
	doc := uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID))
	check.New(t).NotNil(doc)
	return doc
}

// TestUIATextDocumentBasics verifies what the view says about the stream as a whole: its length, the text between two
// offsets, how offsets outside it are brought inside, and what it reports about the selection and the focus.
func TestUIATextDocumentBasics(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
	c.Equal(uiaTextFixtureLength, doc.Len())
	c.Equal(uiaTextDocumentID, doc.Node().ID)
	c.Equal("Title", doc.Text(0, 5))
	c.Equal("link", doc.Text(12, 16))
	c.Equal("￼", doc.Text(17, 18))
	c.Equal(uiaTextFixtureText, doc.Text(0, doc.Len()))

	// Every offset a client can name is brought inside the stream, and an inverted range becomes the degenerate range
	// at its start: a range whose start had crossed its end would stand for text that runs backwards.
	c.Equal(0, doc.ClampOffset(-5))
	c.Equal(uiaTextFixtureLength, doc.ClampOffset(1000))
	start, end := doc.ClampRange(-3, 1000)
	c.Equal(0, start)
	c.Equal(uiaTextFixtureLength, end)
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
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextDocumentID).Actions = accessibility.ActionSet(0)
	c.Equal(SupportedTextSelection_None,
		uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID)).SupportedSelection())
}

// TestUIATextBoundaries pins the divisions of the fixture's stream for every unit. These are what a screen reader reads
// a document by, so each list is written out rather than derived: a division that quietly moved would have Narrator
// read the wrong stretch of text and nothing else would notice.
func TestUIATextBoundaries(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)

	characters := make([]int, 0, uiaTextFixtureLength+1)
	for i := 0; i <= uiaTextFixtureLength; i++ {
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
	c.Equal([]int{0, uiaTextFixtureLength}, doc.Boundaries(TextUnit_Page))
	c.Equal([]int{0, uiaTextFixtureLength}, doc.Boundaries(TextUnit_Document))
	c.Equal([]int{0, uiaTextFixtureLength}, doc.Boundaries(TextUnit(99)), "an unknown unit answers as the document does")
	c.True(uiaTextUnitValid(TextUnit_Character))
	c.True(uiaTextUnitValid(TextUnit_Document))
	c.False(uiaTextUnitValid(TextUnit(-1)))
	c.False(uiaTextUnitValid(TextUnit(7)))

	// The endpoints a client can name are as closed a set as the units, and for the same reason: a method handed
	// something else would move or compare the wrong end of the range and report success.
	c.True(uiaTextEndpointValid(TextPatternRangeEndpoint_Start))
	c.True(uiaTextEndpointValid(TextPatternRangeEndpoint_End))
	c.False(uiaTextEndpointValid(TextPatternRangeEndpoint(-1)))
	c.False(uiaTextEndpointValid(TextPatternRangeEndpoint(2)))

	// A document whose lines were never measured falls back to its paragraphs, since a line whose extent cannot be
	// reported is worse than a paragraph that can.
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextDocumentID).Document.Text.Lines = nil
	unmeasured := uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID))
	c.Equal([]int{0, 6, 23, 29, 33, 35, 36}, unmeasured.Boundaries(TextUnit_Line),
		"an unmeasured document's lines are its paragraphs")
	c.Equal([]int{0, 6, 23, 29, 33, 35, 36}, unmeasured.Boundaries(TextUnit_Paragraph))

	// An empty stream has one boundary, which is both of its ends.
	c.Equal([]int{0}, uiaTextEmptyFixture(t).Boundaries(TextUnit_Word))
}

// uiaTextEmptyFixture returns the view of a document whose stream is empty, which is the edge case every one of the
// arithmetic methods has to survive.
func uiaTextEmptyFixture(t *testing.T) *uiaTextDocument {
	t.Helper()
	tree := newTestTree(uiaTextWindowID, 0,
		&accessibility.Node{
			ID: uiaTextWindowID, Role: role.Window, Bounds: geom.NewRect(0, 0, 100, 100),
			Children: []accessibility.NodeID{uiaTextDocumentID},
		},
		&accessibility.Node{
			ID: uiaTextDocumentID, Role: role.Document, Bounds: geom.NewRect(0, 0, 100, 20),
			Document: &accessibility.DocumentInfo{},
		},
	)
	doc := uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID))
	check.New(t).NotNil(doc)
	return doc
}

// TestUIATextUnitStartAndEnd verifies the two questions every other operation is built out of: where the unit
// containing an offset begins, and where it ends. The end of the stream is the one boundary that begins no unit.
func TestUIATextUnitStartAndEnd(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
	c.Equal(6, doc.UnitStart(TextUnit_Word, 8))
	c.Equal(12, doc.UnitEnd(TextUnit_Word, 8))
	c.Equal(12, doc.UnitStart(TextUnit_Word, 12), "an offset on a boundary is the start of its own unit")
	c.Equal(17, doc.UnitEnd(TextUnit_Word, 12))
	c.Equal(35, doc.UnitStart(TextUnit_Word, uiaTextFixtureLength), "the end of the stream belongs to the last unit")
	c.Equal(uiaTextFixtureLength, doc.UnitEnd(TextUnit_Word, uiaTextFixtureLength))
	c.Equal(0, doc.UnitStart(TextUnit_Line, 3))
	c.Equal(6, doc.UnitEnd(TextUnit_Line, 3))
	c.Equal(17, doc.UnitStart(TextUnit_Line, 20), "a wrapped line begins where the layout wrapped it")
	c.Equal(6, doc.UnitStart(TextUnit_Paragraph, 20), "while the paragraph it is part of began earlier")
	c.True(doc.IsUnitBoundary(TextUnit_Line, 17))
	c.False(doc.IsUnitBoundary(TextUnit_Paragraph, 17))
	c.True(doc.IsUnitBoundary(TextUnit_Word, uiaTextFixtureLength))
}

// TestUIATextExpand covers the eight cases ExpandToEnclosingUnit is documented to handle, which is how every client
// turns a caret position into something to read out.
func TestUIATextExpand(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)

	// 1. An empty stream has no unit to expand to.
	start, end := uiaTextEmptyFixture(t).Expand(TextUnit_Word, 0, 0)
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
			name: "degenerate at the end of the stream", start: uiaTextFixtureLength, end: uiaTextFixtureLength,
			unit: TextUnit_Word, wantStart: 35, wantEnd: uiaTextFixtureLength,
		},
		{name: "already whole units", start: 6, end: 12, unit: TextUnit_Word, wantStart: 6, wantEnd: 12},
		{name: "start inside a unit", start: 8, end: 12, unit: TextUnit_Word, wantStart: 6, wantEnd: 12},
		{name: "end inside a unit", start: 6, end: 14, unit: TextUnit_Word, wantStart: 6, wantEnd: 17},
		{name: "both inside units", start: 8, end: 14, unit: TextUnit_Word, wantStart: 6, wantEnd: 17},
		{
			name: "the document unit is the whole stream", start: 8, end: 14, unit: TextUnit_Document, wantStart: 0,
			wantEnd: uiaTextFixtureLength,
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
			wantStart: 0, wantEnd: uiaTextFixtureLength,
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

// TestUIATextMove verifies Move, which is how a client walks a document one unit at a time, including the count it
// reports: a client knows it has reached an end of the document by being told it moved fewer units than it asked for.
func TestUIATextMove(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
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
			wantStart: uiaTextFixtureLength, wantEnd: uiaTextFixtureLength, wantMoved: 11,
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
			name: "a range at the last word cannot move past it", start: 35, end: uiaTextFixtureLength,
			unit: TextUnit_Word, count: 1, wantStart: 35, wantEnd: uiaTextFixtureLength, wantMoved: 0,
		},
		{
			name: "while a caret there reaches the end of the stream", start: 35, end: 35, unit: TextUnit_Word,
			count: 1, wantStart: uiaTextFixtureLength, wantEnd: uiaTextFixtureLength, wantMoved: 1,
		},
		{
			name: "and the document is one unit nothing moves past", start: 0, end: uiaTextFixtureLength,
			unit: TextUnit_Document, count: 1, wantStart: 0, wantEnd: uiaTextFixtureLength, wantMoved: 0,
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
			name: "a caret at the end of the stream cannot move forward", start: uiaTextFixtureLength,
			end: uiaTextFixtureLength, unit: TextUnit_Word, count: 1, wantStart: uiaTextFixtureLength,
			wantEnd: uiaTextFixtureLength, wantMoved: 0,
		},
		{
			name: "however many units it is asked for", start: uiaTextFixtureLength, end: uiaTextFixtureLength,
			unit: TextUnit_Character, count: 5, wantStart: uiaTextFixtureLength, wantEnd: uiaTextFixtureLength,
			wantMoved: 0,
		},
	} {
		gotStart, gotEnd, moved := doc.Move(one.unit, one.count, one.start, one.end)
		c.Equal(one.wantStart, gotStart, "case %d (%s) start", i, one.name)
		c.Equal(one.wantEnd, gotEnd, "case %d (%s) end", i, one.name)
		c.Equal(one.wantMoved, moved, "case %d (%s) moved", i, one.name)
	}

	// An empty stream has nowhere to move to, in either direction.
	empty := uiaTextEmptyFixture(t)
	start, end, moved := empty.Move(TextUnit_Word, 5, 0, 0)
	c.Equal(0, start)
	c.Equal(0, end)
	c.Equal(0, moved)
}

// TestUIATextMoveEndpoint verifies MoveEndpointByUnit, which is how a client grows or shrinks a range: the first move
// in either direction lands on the nearest boundary that way whether or not the endpoint was already on one, and an
// endpoint pushed past the other takes it along.
func TestUIATextMoveEndpoint(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
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
			name: "there is nowhere past the end of the stream", start: uiaTextFixtureLength,
			end: uiaTextFixtureLength, endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Word, count: 1,
			wantStart: uiaTextFixtureLength, wantEnd: uiaTextFixtureLength, wantMoved: 0,
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
			wantEnd: uiaTextFixtureLength, wantMoved: 7,
		},
		{
			name: "the end backward from an offset that is not a boundary snaps to the one behind it", start: 6,
			end: 20, endpoint: TextPatternRangeEndpoint_End, unit: TextUnit_Line, count: -1, wantStart: 6, wantEnd: 17,
			wantMoved: -1,
		},
		{
			name: "and the caret at the end of the stream has nowhere further to go", start: uiaTextFixtureLength,
			end: uiaTextFixtureLength, endpoint: TextPatternRangeEndpoint_Start, unit: TextUnit_Character, count: 2,
			wantStart: uiaTextFixtureLength, wantEnd: uiaTextFixtureLength, wantMoved: 0,
		},
	} {
		gotStart, gotEnd, moved := doc.MoveEndpoint(one.unit, one.endpoint, one.count, one.start, one.end)
		c.Equal(one.wantStart, gotStart, "case %d (%s) start", i, one.name)
		c.Equal(one.wantEnd, gotEnd, "case %d (%s) end", i, one.name)
		c.Equal(one.wantMoved, moved, "case %d (%s) moved", i, one.name)
	}
}

// TestUIATextClipping verifies the one place UI Automation counts in UTF-16 code units rather than in characters: the
// maximum length GetText is given. A character that needs two units is dropped whole rather than cut in half, since
// half of a surrogate pair is not a character.
func TestUIATextClipping(t *testing.T) {
	c := check.New(t)
	c.Equal("abc", UIATextClip("abc", -1))
	c.Equal("abc", UIATextClip("abc", 10))
	c.Equal("ab", UIATextClip("abc", 2))
	c.Equal("", UIATextClip("abc", 0))
	c.Equal("", UIATextClip("", 5))

	// An emoji is one character and two code units.
	c.Equal("a", UIATextClip("a\U0001F600b", 2))
	c.Equal("a\U0001F600", UIATextClip("a\U0001F600b", 3))
	c.Equal("a\U0001F600b", UIATextClip("a\U0001F600b", 4))

	// A character outside the Basic Multilingual Plane is never split, however little room is left.
	c.Equal("", UIATextClip("\U0001F600", 1))
}

// TestUIATextFindText verifies searching within a range, in both directions and with and without regard for case.
func TestUIATextFindText(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
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

// TestUIATextAttributes verifies what a range says about itself: the value every formatting unit in it agrees on, the
// reserved mixed answer where they do not, and the reserved not-supported answer for an attribute no document here
// records.
func TestUIATextAttributes(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)

	// The heading, which is one uniform run.
	c.Equal(uiaAttribute{Kind: uiaAttributeString, Text: "Heading"}, doc.Attribute(0, 5, UIA_FontNameAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeNumber, Number: 20}, doc.Attribute(0, 5, UIA_FontSizeAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: 700}, doc.Attribute(0, 5, UIA_FontWeightAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: false}, doc.Attribute(0, 5, UIA_IsItalicAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_None)},
		doc.Attribute(0, 5, UIA_UnderlineStyleAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_Heading2)},
		doc.Attribute(0, 5, UIA_StyleIdAttributeId))

	// Every document here is read-only, nothing in a stream is hidden, and the text is active while the document holds
	// the keyboard focus.
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: true}, doc.Attribute(0, 5, UIA_IsReadOnlyAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: false}, doc.Attribute(0, 5, UIA_IsHiddenAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: true}, doc.Attribute(0, 5, UIA_IsActiveAttributeId))

	// A range covering two runs that disagree is mixed, and one covering two that agree is not.
	c.Equal(uiaAttribute{Kind: uiaAttributeMixed}, doc.Attribute(0, 12, UIA_FontNameAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeString, Text: "Body"}, doc.Attribute(5, 23, UIA_FontNameAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeMixed}, doc.Attribute(5, 23, UIA_UnderlineStyleAttributeId),
		"the link within it is underlined and the rest is not")

	// The link.
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_Single)},
		doc.Attribute(12, 16, UIA_UnderlineStyleAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeRange, Start: 12, End: 16}, doc.Attribute(12, 16, UIA_LinkAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeRange, Start: 12, End: 16}, doc.Attribute(13, 14, UIA_LinkAttributeId),
		"part of a link is still that link")
	c.Equal(uiaAttribute{Kind: uiaAttributeEmpty}, doc.Attribute(6, 12, UIA_LinkAttributeId),
		"text that is not part of a link has none, which is an empty answer rather than a refusal")
	c.Equal(uiaAttribute{Kind: uiaAttributeMixed}, doc.Attribute(6, 16, UIA_LinkAttributeId))

	// The code block is the one style UI Automation has no identifier for, so it is custom and carries a name.
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_Custom)},
		doc.Attribute(23, 28, UIA_StyleIdAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeString, Text: "Code"}, doc.Attribute(23, 28, UIA_StyleNameAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: true}, doc.Attribute(23, 28, UIA_IsReadOnlyAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, doc.Attribute(0, 5, UIA_StyleNameAttributeId),
		"a style with an identifier of its own needs no name")

	// The list item and the table cells.
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_BulletedList)},
		doc.Attribute(29, 32, UIA_StyleIdAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_Normal)},
		doc.Attribute(33, 34, UIA_StyleIdAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_Normal)},
		doc.Attribute(12, 16, UIA_StyleIdAttributeId), "a link is body text unless something around it says otherwise")

	// The last cell, which is the one italic, struck-through run.
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: true}, doc.Attribute(35, 36, UIA_IsItalicAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_Single)},
		doc.Attribute(35, 36, UIA_StrikethroughStyleAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_None)},
		doc.Attribute(33, 34, UIA_StrikethroughStyleAttributeId), "while the cell before it is struck through by nothing")
	c.Equal(uiaAttribute{Kind: uiaAttributeMixed}, doc.Attribute(33, 36, UIA_IsItalicAttributeId),
		"a range covering both cells agrees about neither")
	c.Equal(uiaAttribute{Kind: uiaAttributeMixed}, doc.Attribute(33, 36, UIA_StrikethroughStyleAttributeId))
	start, end, ok := doc.FindAttribute(0, doc.Len(), UIA_IsItalicAttributeId,
		uiaAttribute{Kind: uiaAttributeBoolean, Bool: true}, false)
	c.True(ok, "the italic stretch can be jumped to")
	c.Equal(35, start)
	c.Equal(uiaTextFixtureLength, end)

	// A degenerate range answers from the unit its offset is in rather than being mixed.
	c.Equal(uiaAttribute{Kind: uiaAttributeString, Text: "Mono"}, doc.Attribute(25, 25, UIA_FontNameAttributeId))

	// The attributes no document here records are refused rather than guessed at.
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, doc.Attribute(0, 5, UIA_CultureAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, doc.Attribute(0, 5, UIA_AnnotationTypesAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, doc.Attribute(0, 5, TextAttributeID(40099)))

	// A document that does not hold the focus has inactive text, and one whose runs were never filled in has no font to
	// report — but still says what it can. The focus is the snapshot's to name, which is why the flag on the node is
	// not what moves it.
	tree := uiaTextFixtureTree()
	tree.Focus = 0
	tree.Node(uiaTextDocumentID).Document.Text.Runs = nil
	plain := uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID))
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: false}, plain.Attribute(0, 5, UIA_IsActiveAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, plain.Attribute(0, 5, UIA_FontNameAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, plain.Attribute(0, 5, UIA_FontSizeAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeUnsupported}, plain.Attribute(0, 5, UIA_FontWeightAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeBoolean, Bool: false}, plain.Attribute(0, 5, UIA_IsItalicAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_None)},
		plain.Attribute(0, 5, UIA_StrikethroughStyleAttributeId))
	c.Equal(uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_Heading2)},
		plain.Attribute(0, 5, UIA_StyleIdAttributeId), "a style comes from the elements rather than from the runs")

	// A heading with no level is still a heading, and one deeper than UI Automation counts reports the deepest level.
	c.Equal(StyleId_Heading1, uiaHeadingStyle(0))
	c.Equal(StyleId_Heading1, uiaHeadingStyle(1))
	c.Equal(StyleId_Heading9, uiaHeadingStyle(9))
	c.Equal(StyleId_Heading9, uiaHeadingStyle(20))
}

// TestUIATextFindAttribute verifies searching by attribute, which is how a client jumps to the next link or heading
// without reading everything in between.
func TestUIATextFindAttribute(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)

	// The heading is the first stretch styled as one.
	start, end, ok := doc.FindAttribute(0, doc.Len(),
		UIA_StyleIdAttributeId, uiaAttribute{Kind: uiaAttributeInteger, Int: int32(StyleId_Heading2)}, false)
	c.True(ok)
	c.Equal(0, start)
	c.Equal(5, end)

	// The link is the only underlined stretch.
	start, end, ok = doc.FindAttribute(0, doc.Len(), UIA_UnderlineStyleAttributeId,
		uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_Single)}, false)
	c.True(ok)
	c.Equal(12, start)
	c.Equal(16, end)

	// A stretch runs as far as the attribute goes on agreeing, and searching backward finds the last one rather than
	// the first.
	start, end, ok = doc.FindAttribute(0, doc.Len(), UIA_FontNameAttributeId,
		uiaAttribute{Kind: uiaAttributeString, Text: "Body"}, false)
	c.True(ok)
	c.Equal(5, start)
	c.Equal(23, end)
	start, end, ok = doc.FindAttribute(0, doc.Len(), UIA_FontNameAttributeId,
		uiaAttribute{Kind: uiaAttributeString, Text: "Body"}, true)
	c.True(ok)
	c.Equal(28, start)
	c.Equal(uiaTextFixtureLength, end)

	// Nothing in the range has it.
	_, _, ok = doc.FindAttribute(0, 5, UIA_FontNameAttributeId,
		uiaAttribute{Kind: uiaAttributeString, Text: "Mono"}, false)
	c.False(ok)

	// A value of a kind no stretch of text can have matches nothing, and neither does a degenerate range.
	_, _, ok = doc.FindAttribute(0, doc.Len(), UIA_LinkAttributeId,
		uiaAttribute{Kind: uiaAttributeRange, Start: 12, End: 16}, false)
	c.False(ok)
	_, _, ok = doc.FindAttribute(0, doc.Len(), UIA_FontNameAttributeId, uiaAttribute{Kind: uiaAttributeMixed}, false)
	c.False(ok)
	_, _, ok = doc.FindAttribute(12, 12, UIA_UnderlineStyleAttributeId,
		uiaAttribute{Kind: uiaAttributeInteger, Int: int32(TextDecorationLineStyle_Single)}, false)
	c.False(ok)
	c.True(uiaAttributeSearchable(uiaAttributeBoolean))
	c.False(uiaAttributeSearchable(uiaAttributeEmpty))
	c.False(uiaAttributeSearchable(uiaAttributeUnsupported))
}

// TestUIATextEnclosingAndChildren verifies the two questions a client walking a document by element asks of a range:
// what encloses it, and what is inside it.
func TestUIATextEnclosingAndChildren(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
	c.Equal(uiaTextLinkID, doc.EnclosingSpan(12, 16))
	c.Equal(uiaTextLinkID, doc.EnclosingSpan(13, 14), "part of a link is still inside it")
	c.Equal(uiaTextLinkID, doc.EnclosingSpan(13, 13), "a caret is treated as the character that follows it")
	c.Equal(uiaTextParagraphID, doc.EnclosingSpan(6, 22))
	c.Equal(uiaTextParagraphID, doc.EnclosingSpan(6, 16))
	c.Equal(uiaTextHeadingID, doc.EnclosingSpan(0, 5))
	c.Equal(uiaTextCellAID, doc.EnclosingSpan(33, 34))
	c.Equal(uiaTextDocumentID, doc.EnclosingSpan(0, doc.Len()), "nothing but the document covers the whole stream")
	c.Equal(uiaTextDocumentID, doc.EnclosingSpan(5, 6), "nor the line feed between two blocks")

	// One level down, rather than everything within the range: a client narrows to a child and asks again.
	c.Equal([]accessibility.NodeID{
		uiaTextHeadingID, uiaTextParagraphID, uiaTextCodeID, uiaTextListID, uiaTextTableID,
	}, doc.Children(0, doc.Len()))
	c.Equal([]accessibility.NodeID{uiaTextLinkID, uiaTextImageID}, doc.Children(6, 22))
	c.Equal([]accessibility.NodeID{uiaTextLinkID}, doc.Children(6, 16), "and only the ones the range reaches")
	c.Equal([]accessibility.NodeID{uiaTextCellAID, uiaTextCellBID}, doc.Children(33, 36))
	c.Nil(doc.Children(12, 16), "a link holds no elements of its own")
	c.Nil(doc.Children(13, 13), "a degenerate range covers no text and so has no children")
}

// TestUIATextSpanFor verifies the answer ITextProvider::RangeFromChild and ITextChildProvider::get_TextRange are built
// from: which stretch of the stream an element occupies.
func TestUIATextSpanFor(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
	start, end, ok := doc.SpanFor(uiaTextLinkID)
	c.True(ok)
	c.Equal(12, start)
	c.Equal(16, end)

	start, end, ok = doc.SpanFor(uiaTextImageID)
	c.True(ok)
	c.Equal(17, start)
	c.Equal(18, end)

	// An element the composition gave no stretch of its own is answered with its nearest ancestor's, which is the
	// smallest stretch of text that is certainly its.
	start, end, ok = doc.SpanFor(uiaTextLabelID)
	c.True(ok)
	c.Equal(6, start)
	c.Equal(22, end)

	// The document itself has no stretch, and neither has anything outside it.
	_, _, ok = doc.SpanFor(uiaTextDocumentID)
	c.False(ok)
	_, _, ok = doc.SpanFor(uiaTextWindowID)
	c.False(ok)
	_, _, ok = doc.SpanFor(uiaTextButtonID)
	c.False(ok)
	_, _, ok = doc.SpanFor(0)
	c.False(ok)
}

// TestUIATextContainerFor verifies what grants the TextChild pattern: sitting inside a document that claims some of its
// text.
func TestUIATextContainerFor(t *testing.T) {
	c := check.New(t)
	tree := uiaTextFixtureTree()
	for _, id := range []accessibility.NodeID{
		uiaTextHeadingID, uiaTextParagraphID, uiaTextLinkID, uiaTextImageID, uiaTextCodeID, uiaTextListID,
		uiaTextListItemID, uiaTextTableID, uiaTextRowID, uiaTextCellAID, uiaTextCellBID, uiaTextLabelID,
	} {
		c.Equal(uiaTextDocumentID, uiaTextContainerFor(tree, tree.Node(id)), "node %d", id)
	}

	// The document is the container rather than a child of one, and nothing outside it has a container at all.
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(tree, tree.Node(uiaTextDocumentID)))
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(tree, tree.Node(uiaTextScrollID)))
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(tree, tree.Node(uiaTextWindowID)))
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(tree, tree.Node(uiaTextButtonID)))
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(nil, tree.Node(uiaTextLinkID)))
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(tree, nil))

	// A panel inside the document that the composition passed over has no stretch of text to report, so it reports no
	// pattern either.
	tree.Nodes[uiaTextDocumentID].Children = append(tree.Nodes[uiaTextDocumentID].Children, 20)
	tree.Nodes[20] = &accessibility.Node{ID: 20, Role: role.Separator, Parent: uiaTextDocumentID}
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(tree, tree.Node(20)))

	// A Document that carries no stream is no container at all: it hands out no Text pattern for anything to be a child
	// of.
	plain := uiaTextFixtureTree()
	plain.Node(uiaTextDocumentID).Document = nil
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(plain, plain.Node(uiaTextLinkID)))
}

// TestUIATextRectangles verifies the rectangles a client draws its highlight from: one per line the range covers, in
// window coordinates, clipped to what the scroll area really shows.
func TestUIATextRectangles(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)

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
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextDocumentID).Offscreen = true
	c.Nil(uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID)).Rectangles(0, 5))
}

// TestUIATextVisibleRange verifies what GetVisibleRanges reports: the stretch of the stream the user can actually see,
// which is what a client reading what is on screen starts from.
func TestUIATextVisibleRange(t *testing.T) {
	c := check.New(t)
	start, end, ok := uiaTextFixture(t).VisibleRange()
	c.True(ok)
	c.Equal(0, start)
	c.Equal(29, end, "the scroll area shows the first four lines and clips the rest")

	// A document scrolled out of view shows nothing.
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextDocumentID).Offscreen = true
	_, _, ok = uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID)).VisibleRange()
	c.False(ok)

	// One clipped away entirely by an ancestor shows nothing either.
	clipped := uiaTextFixtureTree()
	clipped.Node(uiaTextScrollID).Bounds = geom.NewRect(500, 500, 10, 10)
	_, _, ok = uiaNewTextDocument(clipped, clipped.Node(uiaTextDocumentID)).VisibleRange()
	c.False(ok)

	// A document whose lines were never measured reports all of itself while it is on screen at all, since there is no
	// telling which part of it is where.
	unmeasured := uiaTextFixtureTree()
	unmeasured.Node(uiaTextDocumentID).Document.Text.Lines = nil
	start, end, ok = uiaNewTextDocument(unmeasured, unmeasured.Node(uiaTextDocumentID)).VisibleRange()
	c.True(ok)
	c.Equal(0, start)
	c.Equal(uiaTextFixtureLength, end)
}

// TestUIATextOffsetAt verifies the hit test a client placing the caret with the mouse goes through. Every point has an
// answer, since RangeFromPoint is documented to report the nearest range rather than to fail.
func TestUIATextOffsetAt(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)

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
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextDocumentID).Document.Text.Lines = nil
	c.Equal(0, uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID)).OffsetAt(geom.NewPoint(50, 50)))
}

// TestUIATextVisibleBounds verifies the clipping every rectangle and the visible range go through: a node's bounds
// intersected with every ancestor's, which is what a scroll area does to its content.
func TestUIATextVisibleBounds(t *testing.T) {
	c := check.New(t)
	tree := uiaTextFixtureTree()
	c.Equal(geom.NewRect(10, 20, 110, 35), uiaVisibleBounds(tree, tree.Node(uiaTextDocumentID)))
	c.Equal(geom.NewRect(0, 0, 200, 200), uiaVisibleBounds(tree, tree.Node(uiaTextWindowID)))

	// An offscreen node has no visible area, and neither has one an ancestor clips away entirely.
	offscreen := uiaTextFixtureTree()
	offscreen.Node(uiaTextDocumentID).Offscreen = true
	c.True(uiaVisibleBounds(offscreen, offscreen.Node(uiaTextDocumentID)).Empty())
	clipped := uiaTextFixtureTree()
	clipped.Node(uiaTextScrollID).Bounds = geom.NewRect(500, 500, 10, 10)
	c.True(uiaVisibleBounds(clipped, clipped.Node(uiaTextDocumentID)).Empty())

	// An ancestor with no bounds filled in is passed over rather than clipping everything away.
	unbounded := uiaTextFixtureTree()
	unbounded.Node(uiaTextScrollID).Bounds = geom.Rect{}
	c.Equal(geom.NewRect(10, 20, 110, 70), uiaVisibleBounds(unbounded, unbounded.Node(uiaTextDocumentID)))

	c.True(uiaVisibleBounds(nil, tree.Node(uiaTextDocumentID)).Empty())
	c.True(uiaVisibleBounds(tree, nil).Empty())
}

// TestUIATextDocumentMemo verifies that the view of a document's stream is worked out once per snapshot: a client
// reading a document asks the same questions thousands of times, and dividing the stream again for each of them would
// cost a pass over the whole document every time.
func TestUIATextDocumentMemo(t *testing.T) {
	c := check.New(t)
	uiaForgetSnapshotMemo()
	t.Cleanup(uiaForgetSnapshotMemo)
	tree := uiaTextFixtureTree()
	first := uiaMemoizedTextDocument(tree, uiaTextDocumentID)
	c.NotNil(first)
	c.True(first == uiaMemoizedTextDocument(tree, uiaTextDocumentID), "the same snapshot is answered from the memo")

	// A different snapshot gets a view of its own, and the memo moves on to it.
	next := uiaTextFixtureTree()
	next.Generation = 2
	second := uiaMemoizedTextDocument(next, uiaTextDocumentID)
	c.NotNil(second)
	c.False(first == second)
	c.True(second == uiaMemoizedTextDocument(next, uiaTextDocumentID))

	// A snapshot the memo has moved past is answered from itself rather than taking the memo back, which is what keeps
	// the memo on the snapshot a client is walking; see uiaMemoSwitchTo.
	third := uiaMemoizedTextDocument(tree, uiaTextDocumentID)
	c.NotNil(third)
	c.False(third == second)
	c.True(second == uiaMemoizedTextDocument(next, uiaTextDocumentID))

	// Nothing but a Document with a stream has a view.
	c.Nil(uiaMemoizedTextDocument(tree, uiaTextLinkID))
	c.Nil(uiaMemoizedTextDocument(tree, 0))
	c.Nil(uiaMemoizedTextDocument(nil, uiaTextDocumentID))
	plain := uiaTextFixtureTree()
	plain.Node(uiaTextDocumentID).Document = nil
	c.Nil(uiaMemoizedTextDocument(plain, uiaTextDocumentID))
	c.Nil(uiaNewTextDocument(plain, plain.Node(uiaTextDocumentID)))
	c.Nil(uiaNewTextDocument(nil, nil))

	// Forgetting the snapshot drops the views with it, so that a window nothing is answering from any more is not kept
	// alive by them.
	uiaForgetSnapshotMemo()
	c.False(second == uiaMemoizedTextDocument(next, uiaTextDocumentID))
}

// TestUIATextSpanIndexMemo verifies that where each element sits in a document's stream is worked out once per
// snapshot. UIAProvidedPatterns asks it of every element a client so much as looks at, in both of the snapshots a
// publish compares, so answering it from a document's span list each time costs a scan of the whole document per
// property query.
func TestUIATextSpanIndexMemo(t *testing.T) {
	c := check.New(t)
	uiaForgetSnapshotMemo()
	t.Cleanup(uiaForgetSnapshotMemo)
	tree := uiaTextFixtureTree()
	c.Equal(uiaTextDocumentID, uiaTextContainerFor(tree, tree.Node(uiaTextLinkID)))
	c.True(uiaSnapshotMemo.textSpansDone, "asking where one element sits indexes the whole snapshot")
	c.Equal(uiaDocumentSpan{document: uiaTextDocumentID, start: 12, end: 16}, uiaSnapshotMemo.textSpans[uiaTextLinkID])
	c.Equal(len(uiaTextFixtureSpans()), len(uiaSnapshotMemo.textSpans), "one entry per span the document records")
	_, held := uiaSnapshotMemo.textSpans[uiaTextLabelID]
	c.False(held, "an element the composition passed over has no span of its own")

	// The next answer comes from the index rather than from the spans: an entry put into it by hand is what is answered
	// with, and nothing else could produce that answer, since the button sits outside the document altogether.
	uiaSnapshotMemo.textSpans[uiaTextButtonID] = uiaDocumentSpan{document: uiaTextDocumentID, start: 1, end: 2}
	start, end, ok := uiaTextSpanFor(tree, tree.Node(uiaTextDocumentID), uiaTextButtonID)
	c.True(ok)
	c.Equal(1, start)
	c.Equal(2, end)

	// A different snapshot is indexed afresh, so the sentinel stays with the snapshot it was put into.
	next := uiaTextFixtureTree()
	next.Generation = 2
	c.Equal(uiaTextDocumentID, uiaTextContainerFor(next, next.Node(uiaTextLinkID)))
	c.Equal(next, uiaSnapshotMemo.tree)
	_, held = uiaSnapshotMemo.textSpans[uiaTextButtonID]
	c.False(held)
	_, _, ok = uiaTextSpanFor(next, next.Node(uiaTextDocumentID), uiaTextButtonID)
	c.False(ok, "an element outside the document occupies none of its text")

	// A snapshot the memo has moved past is answered from its own spans and does not take the memo back; see
	// uiaMemoSwitchTo.
	c.Nil(uiaMemoizedTextSpans(tree), "such a snapshot is not indexed at all")
	c.Equal(next, uiaSnapshotMemo.tree)
	c.Equal(uiaTextDocumentID, uiaTextContainerFor(tree, tree.Node(uiaTextLinkID)),
		"but it is answered exactly as it would have been")
	spanStart, spanEnd, spanOK := uiaTextSpanFor(tree, tree.Node(uiaTextDocumentID), uiaTextLinkID)
	c.True(spanOK)
	c.Equal(12, spanStart)
	c.Equal(16, spanEnd)

	// Forgetting the snapshot drops the index with it, and a snapshot with no document to index allocates no map at
	// all.
	uiaForgetSnapshotMemo()
	c.False(uiaSnapshotMemo.textSpansDone)
	c.Nil(uiaSnapshotMemo.textSpans)
	plain := uiaTextFixtureTree()
	plain.Node(uiaTextDocumentID).Document = nil
	c.Nil(uiaMemoizedTextSpans(plain))
	c.True(uiaSnapshotMemo.textSpansDone)
	c.Equal(accessibility.NodeID(0), uiaTextContainerFor(plain, plain.Node(uiaTextLinkID)))
}

// TestUIATextSpansSkipElementsWithNoProvider verifies that the elements a client cannot be handed are left out of the
// stream's spans: an ignored node has no provider, so GetEnclosingElement and GetChildren must answer with whatever
// contains it instead.
func TestUIATextSpansSkipElementsWithNoProvider(t *testing.T) {
	c := check.New(t)
	tree := uiaTextFixtureTree()
	tree.Node(uiaTextLinkID).Ignored = true
	doc := uiaNewTextDocument(tree, tree.Node(uiaTextDocumentID))
	c.Equal(uiaTextParagraphID, doc.EnclosingSpan(12, 16))
	c.Equal([]accessibility.NodeID{uiaTextImageID}, doc.Children(6, 22))

	// A span whose node the snapshot no longer holds at all is left out the same way.
	missing := uiaTextFixtureTree()
	delete(missing.Nodes, uiaTextImageID)
	doc = uiaNewTextDocument(missing, missing.Node(uiaTextDocumentID))
	c.Equal([]accessibility.NodeID{uiaTextLinkID}, doc.Children(6, 22))
}

// TestUIATextWalksTheWholeDocument follows what a screen reader reading a document from top to bottom actually does:
// expand the document range to the first unit, read it, move by one unit, read again, and stop when the move reports
// that it could not go as far as it asked. Every unit of the stream has to come out exactly once, in order, or a person
// hears a line twice, misses one, or never reaches the end.
func TestUIATextWalksTheWholeDocument(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
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
		c.Equal(strings.Join(one.expected, ""), uiaTextFixtureText, "%s must cover the stream exactly once", one.name)
	}
}

// TestUIATextCaretWalkReachesTheEnd follows the other walk a client makes over a document: stepping a caret rather than
// a range, which is what NVDA's say-all reader and every review cursor do. Nothing but the count Move reports tells
// such a client where the end of the document is, so a caret that has arrived there has to report that it did not move
// — for every unit, since a client picks the unit. Reporting a move it did not make leaves the reader stepping forever.
func TestUIATextCaretWalkReachesTheEnd(t *testing.T) {
	c := check.New(t)
	doc := uiaTextFixture(t)
	for _, unit := range []TextUnit{
		TextUnit_Character, TextUnit_Format, TextUnit_Word, TextUnit_Line, TextUnit_Paragraph, TextUnit_Page,
		TextUnit_Document,
	} {
		start, end, moved := doc.Move(unit, 1, uiaTextFixtureLength, uiaTextFixtureLength)
		c.Equal(uiaTextFixtureLength, start, "unit %d", unit)
		c.Equal(uiaTextFixtureLength, end, "unit %d stays degenerate", unit)
		c.Equal(0, moved, "a caret at the end of the stream has nowhere to go, unit %d", unit)

		// The whole walk, from a caret at the beginning of the stream. Every step has to advance and to be counted, and
		// the walk has to stop at the end. The iteration count is bounded only so that a regression fails instead of
		// hanging: one step per character is as many as any unit can need.
		at, steps := 0, 0
		for range uiaTextFixtureLength + 2 {
			next, _, movedOne := doc.Move(unit, 1, at, at)
			if movedOne == 0 {
				break
			}
			c.Equal(1, movedOne, "unit %d", unit)
			c.True(next > at, "a caret moving forward must advance, unit %d", unit)
			at, steps = next, steps+1
		}
		c.Equal(uiaTextFixtureLength, at, "the walk ends at the end of the stream, unit %d", unit)
		c.Equal(len(doc.Boundaries(unit))-1, steps, "one step per unit of the stream, unit %d", unit)

		// And backwards from there to the beginning, which is the same walk in reverse.
		for range uiaTextFixtureLength + 2 {
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
