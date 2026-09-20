// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"fmt"
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// The node ids of the document fixture below. It is the shape a Markdown view publishes — a document of headings,
// paragraphs with inline links and images, quotes, lists, a code block and a table — with a panel of controls beside
// it, so that one snapshot answers for every search key and for the attributed string as well.
const (
	axDocRoot accessibility.NodeID = 1600 + iota
	axDocDocument
	axDocHeading1
	axDocParagraph
	axDocLink
	axDocImage
	axDocIgnoredGroup
	axDocHeading2
	axDocSeparator
	axDocQuote
	axDocQuoteParagraph
	axDocInnerQuote
	axDocInnerParagraph
	axDocList
	axDocItem1
	axDocItem1Paragraph
	axDocItem2
	axDocItem2Paragraph
	axDocQuote2
	axDocQuote2Paragraph
	axDocCode
	axDocHeading1b
	axDocTable
	axDocRow
	axDocCell
	axDocCheckCell
	axDocCheck
	axDocForm
	axDocField
	axDocButton
	axDocRadios
	axDocRadioSmall
	axDocRadioLarge
	axDocOutline
	axDocTable2
	axDocRow2
	axDocNestedCell
	axDocNestedTable
	axDocNestedRow
	axDocNestedTableCell
)

// axDocSerif is the font family the fixture's prose is drawn in, and axDocMono the one its code block uses. They are
// named because the font name the attributed string reports is composed from the family, so the two have to agree.
const (
	axDocSerif = "Serif"
	axDocMono  = "Mono"
)

// axDocParagraphText is the content of the fixture's first paragraph. It holds a link whose own text carries an emoji —
// which takes two UTF-16 code units, so the offsets the attributed string is written at cannot be right by accident,
// and putting it inside the link puts a span boundary and a run boundary on the far side of a surrogate pair — and the
// U+FFFC an image stands in the stream as.
const axDocParagraphText = "See the 😀 guide and this ￼"

// The rune ranges of axDocParagraphText the fixture's runs and spans cover. They are spelled out here because both the
// search tests and the attributed-string test reason about them.
const (
	axDocLinkStart  = 4  // "the 😀 guide" begins
	axDocLinkEnd    = 15 // and ends
	axDocStrikeEnd  = 20 // the struck-through run, which starts where the link ends, runs to here
	axDocImageStart = 25 // the U+FFFC
	axDocImageEnd   = 26
)

// axDocPresentedOrder is the order an assistive technology reaches the fixture's nodes in, which is what a search
// walks: the presented hierarchy pre-order, with the ignored group spliced out, the separator dropped and the check box
// standing in for the cell that holds nothing else. It is spelled out rather than derived from the walk, so that a
// change to the walk has to be a deliberate change to this list.
var axDocPresentedOrder = []accessibility.NodeID{
	axDocDocument,
	axDocHeading1,
	axDocParagraph, axDocLink, axDocImage,
	axDocHeading2,
	axDocQuote, axDocQuoteParagraph, axDocInnerQuote, axDocInnerParagraph,
	axDocList, axDocItem1, axDocItem1Paragraph, axDocItem2, axDocItem2Paragraph,
	axDocQuote2, axDocQuote2Paragraph,
	axDocCode,
	axDocHeading1b,
	axDocTable, axDocRow, axDocCell, axDocCheck,
	axDocTable2, axDocRow2, axDocNestedTable, axDocNestedRow, axDocNestedTableCell,
	axDocForm, axDocField, axDocButton, axDocRadios, axDocRadioSmall, axDocRadioLarge, axDocOutline,
}

// axDocDocumentOrder returns the part of axDocPresentedOrder that lies within the document: everything from its first
// block to the last node beneath it, which is what a search of the document answers with. The panel of controls beside
// it is outside every scope inside the document.
func axDocDocumentOrder() []accessibility.NodeID {
	return axDocPresentedOrder[1:slices.Index(axDocPresentedOrder, axDocForm)]
}

// axDocStaticTextNodes returns the fixture's nodes an assistive technology is shown as AXStaticText, in presented
// order: the document's blocks of text, and its headings as well on a system that does not honor the AXHeading role,
// where axHeadingRole falls back to static text and a heading answers the static-text keys with the rest. The
// expectations follow the rule the adapter follows rather than this machine's answer, so they hold either way.
func axDocStaticTextNodes() []accessibility.NodeID {
	ids := []accessibility.NodeID{
		axDocParagraph, axDocQuoteParagraph, axDocInnerParagraph, axDocItem1Paragraph, axDocItem2Paragraph,
		axDocQuote2Paragraph, axDocCode,
	}
	if axHeadingIsStaticText() {
		ids = append(ids, axDocHeading1, axDocHeading2, axDocHeading1b)
	}
	slices.SortFunc(ids, func(a, b accessibility.NodeID) int {
		return slices.Index(axDocPresentedOrder, a) - slices.Index(axDocPresentedOrder, b)
	})
	return ids
}

// axDocNodesAfter returns the nodes of ids a search from a start element can reach, which is those after it in
// presented order: a search only ever moves on from where it started.
func axDocNodesAfter(start accessibility.NodeID, ids []accessibility.NodeID) []accessibility.NodeID {
	from := slices.Index(axDocPresentedOrder, start)
	after := make([]accessibility.NodeID, 0, len(ids))
	for _, id := range ids {
		if slices.Index(axDocPresentedOrder, id) > from {
			after = append(after, id)
		}
	}
	return after
}

// axDocAdvances returns the per-rune advances of a line whose runes are all the same width, which is what a fixture
// needs: the widths themselves say nothing, only that there is one boundary per rune plus the line's end.
func axDocAdvances(runes int, width float32) []float32 {
	advances := make([]float32, runes+1)
	for i := range advances {
		advances[i] = float32(i) * width
	}
	return advances
}

// axDocBlock returns one text-bearing block of the fixture: a paragraph, heading, code block or cell, with its content
// measured as a single line and tiled by the runs it is given.
func axDocBlock(id, parent accessibility.NodeID, r role.Enum, text string, y float32,
	runs []accessibility.TextRun,
) *accessibility.Node {
	count := len([]rune(text))
	return &accessibility.Node{
		ID:       id,
		Parent:   parent,
		Role:     r,
		Bounds:   geom.NewRect(20, y, 260, 16),
		ReadOnly: true,
		Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection,
			accessibility.ScrollRangeIntoView),
		Text: &accessibility.TextInfo{
			Text: text,
			Runs: runs,
			Lines: []accessibility.Line{
				{Advances: axDocAdvances(count, 7), Start: 0, End: count, Bounds: geom.NewRect(0, 0, float32(count)*7, 16)},
			},
		},
	}
}

// axDocHeading returns one heading of the fixture: a block that also carries the level a search by heading level and
// the AXHeadingLevel attribute are both answered from, and the name the snapshot builder folds a heading's own text
// into.
func axDocHeading(id, parent accessibility.NodeID, text string, y float32, level int,
	runs []accessibility.TextRun,
) *accessibility.Node {
	n := axDocBlock(id, parent, role.Heading, text, y, runs)
	n.Name = text
	n.Level = level
	return n
}

// axDocRun returns one run of a block's text.
func axDocRun(start, end int, family string, size float32, weight int) accessibility.TextRun {
	return accessibility.TextRun{Family: family, Start: start, End: end, Weight: weight, Size: size}
}

// axDocNamed gives a node a name and returns it, which is how the fixture puts a name on a block whose name this
// platform deliberately does not report.
func axDocNamed(n *accessibility.Node, name string) *accessibility.Node {
	n.Name = name
	return n
}

// axDocOffscreen marks a node as scrolled out of sight and returns it, which is how the fixture puts a list item and
// its content beyond the visible area in one expression.
func axDocOffscreen(n *accessibility.Node) *accessibility.Node {
	n.Offscreen = true
	return n
}

// axDocTestTree returns the document fixture: a window holding a document — heading, paragraph with an inline link, an
// emoji and an image, a heading inside an ignored group, a separator, a quote holding a nested quote, a list whose
// second item is scrolled out of sight, a second quote at the outer level, a code block, a second level-one heading and
// a table whose last cell holds nothing but a check box — beside a panel of controls.
//
//nolint:funlen // one snapshot standing in for a whole window; splitting it apart would only scatter it
func axDocTestTree() *accessibility.Tree {
	nodes := []*accessibility.Node{
		{
			ID:       axDocRoot,
			Children: []accessibility.NodeID{axDocDocument, axDocForm},
			Name:     "Notes",
			Role:     role.Window,
			Bounds:   geom.NewRect(0, 0, 320, 480),
		},
		{
			ID:     axDocDocument,
			Parent: axDocRoot,
			Children: []accessibility.NodeID{
				axDocHeading1, axDocParagraph, axDocIgnoredGroup, axDocSeparator, axDocQuote, axDocList, axDocQuote2,
				axDocCode, axDocHeading1b, axDocTable, axDocTable2,
			},
			Role:      role.Document,
			Bounds:    geom.NewRect(10, 10, 300, 320),
			ReadOnly:  true,
			Focusable: true,
			// The stream itself, which this platform deliberately does not present: the blocks beneath the document say
			// the same thing as elements, and an assistive technology told to read both would hear everything twice.
			Document: &accessibility.DocumentInfo{Text: accessibility.TextInfo{Text: "Release notes\n", Multiline: true}},
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection,
				accessibility.ScrollRangeIntoView, accessibility.ShowContextMenu),
		},
		axDocHeading(axDocHeading1, axDocDocument, "Release notes", 10, 1,
			[]accessibility.TextRun{axDocRun(0, 13, axDocSerif, 20, 700)}),
		{
			ID:       axDocParagraph,
			Parent:   axDocDocument,
			Children: []accessibility.NodeID{axDocLink, axDocImage},
			// A name this platform does not report either, for the same reason the code block's is not reported. A search
			// for text finds the paragraph by its content rather than by this.
			Name:     "Intro",
			Role:     role.Paragraph,
			Bounds:   geom.NewRect(20, 30, 260, 16),
			ReadOnly: true,
			Actions: accessibility.ActionSet(0).With(accessibility.SetTextSelection,
				accessibility.ScrollRangeIntoView),
			Text: &accessibility.TextInfo{
				Text: axDocParagraphText,
				Runs: []accessibility.TextRun{
					axDocRun(0, axDocLinkStart, axDocSerif, 13, 400),
					axDocRun(axDocLinkStart, axDocLinkEnd, axDocSerif, 13, 700),
					{
						Family: axDocSerif, Start: axDocLinkEnd, End: axDocStrikeEnd, Weight: 400, Size: 13,
						Strikethrough: true,
					},
					axDocRun(axDocStrikeEnd, axDocImageEnd, axDocSerif, 13, 400),
				},
				Spans: []accessibility.TextSpan{
					{Node: axDocLink, Start: axDocLinkStart, End: axDocLinkEnd},
					{Node: axDocImage, Start: axDocImageStart, End: axDocImageEnd},
				},
				Lines: []accessibility.Line{
					{
						Advances: axDocAdvances(len([]rune(axDocParagraphText)), 7),
						Start:    0,
						End:      len([]rune(axDocParagraphText)),
						Bounds:   geom.NewRect(0, 0, 182, 16),
					},
				},
			},
		},
		{
			ID: axDocLink, Parent: axDocParagraph, Name: "the guide", Role: role.Link,
			URL: "https://example.com/guide", Bounds: geom.NewRect(48, 30, 63, 16), Focusable: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
		{
			ID: axDocImage, Parent: axDocParagraph, Name: "A picture", Role: role.Image,
			Bounds: geom.NewRect(195, 30, 16, 16),
		},
		{
			ID: axDocIgnoredGroup, Parent: axDocDocument, Children: []accessibility.NodeID{axDocHeading2},
			Role: role.Group, Bounds: geom.NewRect(20, 50, 260, 16), Ignored: true,
		},
		axDocHeading(axDocHeading2, axDocIgnoredGroup, "Details", 50, 2,
			[]accessibility.TextRun{axDocRun(0, 7, axDocSerif, 16, 700)}),
		{
			ID: axDocSeparator, Parent: axDocDocument, Role: role.Separator,
			Bounds: geom.NewRect(20, 70, 260, 1),
		},
		{
			ID: axDocQuote, Parent: axDocDocument,
			Children: []accessibility.NodeID{axDocQuoteParagraph, axDocInnerQuote}, Name: "Note",
			Role: role.BlockQuote, Bounds: geom.NewRect(20, 80, 260, 40),
		},
		axDocBlock(axDocQuoteParagraph, axDocQuote, role.Paragraph, "Mind the gap", 80,
			[]accessibility.TextRun{{Family: axDocSerif, Start: 0, End: 12, Weight: 400, Size: 13, Italic: true}}),
		{
			ID: axDocInnerQuote, Parent: axDocQuote, Children: []accessibility.NodeID{axDocInnerParagraph},
			Role: role.BlockQuote, Bounds: geom.NewRect(30, 100, 250, 16),
		},
		axDocBlock(axDocInnerParagraph, axDocInnerQuote, role.Paragraph, "Nested", 100,
			[]accessibility.TextRun{axDocRun(0, 6, axDocSerif, 13, 400)}),
		{
			ID: axDocList, Parent: axDocDocument, Children: []accessibility.NodeID{axDocItem1, axDocItem2},
			Role: role.List, Bounds: geom.NewRect(20, 120, 260, 32),
		},
		{
			ID: axDocItem1, Parent: axDocList, Children: []accessibility.NodeID{axDocItem1Paragraph},
			Role: role.ListItem, Bounds: geom.NewRect(20, 120, 260, 16), RowIndex: 0,
		},
		axDocBlock(axDocItem1Paragraph, axDocItem1, role.Paragraph, "First", 120,
			[]accessibility.TextRun{axDocRun(0, 5, axDocSerif, 13, 400)}),
		{
			// Scrolled out of sight, along with its content: what a search asked to answer with visible nodes only has
			// to leave out.
			ID: axDocItem2, Parent: axDocList, Children: []accessibility.NodeID{axDocItem2Paragraph},
			Role: role.ListItem, Bounds: geom.NewRect(20, 136, 260, 16), RowIndex: 1, Offscreen: true,
		},
		axDocOffscreen(axDocBlock(axDocItem2Paragraph, axDocItem2, role.Paragraph, "Second", 136,
			[]accessibility.TextRun{{Family: axDocSerif, Start: 0, End: 6, Weight: 400, Size: 13, Underline: true}})),
		{
			ID: axDocQuote2, Parent: axDocDocument, Children: []accessibility.NodeID{axDocQuote2Paragraph},
			Role: role.BlockQuote, Bounds: geom.NewRect(20, 160, 260, 16),
		},
		axDocBlock(axDocQuote2Paragraph, axDocQuote2, role.Paragraph, "Also", 160,
			[]accessibility.TextRun{axDocRun(0, 4, axDocSerif, 13, 400)}),
		// A name on a code block, which this platform does not report: its content is its value, and a name as well would
		// be spoken ahead of every line of it.
		axDocNamed(axDocBlock(axDocCode, axDocDocument, role.Code, "go build ./...", 180,
			[]accessibility.TextRun{{Family: axDocMono, Start: 0, End: 14, Weight: 400, Size: 12, Monospace: true}}),
			"Example"),
		axDocHeading(axDocHeading1b, axDocDocument, "Fixes", 200, 1,
			[]accessibility.TextRun{axDocRun(0, 5, axDocSerif, 20, 700)}),
		{
			ID: axDocTable, Parent: axDocDocument, Children: []accessibility.NodeID{axDocRow}, Name: "Sizes",
			Role: role.Table, Bounds: geom.NewRect(20, 220, 260, 20), RowCount: 1, ColumnCount: 2,
		},
		{
			ID: axDocRow, Parent: axDocTable, Children: []accessibility.NodeID{axDocCell, axDocCheckCell},
			Role: role.Row, Bounds: geom.NewRect(20, 220, 260, 20), RowIndex: 0,
		},
		axDocBlock(axDocCell, axDocRow, role.Cell, "Ada", 220,
			[]accessibility.TextRun{axDocRun(0, 3, axDocSerif, 13, 400)}),
		{
			// A cell holding nothing but a check box, which the check box stands in for.
			ID: axDocCheckCell, Parent: axDocRow, Children: []accessibility.NodeID{axDocCheck}, Role: role.Cell,
			Bounds: geom.NewRect(150, 220, 130, 20), RowIndex: 0, ColumnIndex: 1,
		},
		{
			ID: axDocCheck, Parent: axDocCheckCell, Name: "Done", Role: role.CheckBox,
			Bounds: geom.NewRect(150, 220, 20, 20), Focusable: true, HasCheck: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
		{
			// A second table at the same level as the first, which is what makes the same-level table key answer
			// something rather than pass for want of any other table.
			ID: axDocTable2, Parent: axDocDocument, Children: []accessibility.NodeID{axDocRow2}, Name: "Totals",
			Role: role.Table, Bounds: geom.NewRect(20, 250, 260, 40), RowCount: 1, ColumnCount: 1,
		},
		{
			ID: axDocRow2, Parent: axDocTable2, Children: []accessibility.NodeID{axDocNestedCell},
			Role: role.Row, Bounds: geom.NewRect(20, 250, 260, 40), RowIndex: 0,
		},
		{
			// A cell holding nothing but a table, which that table stands in for — so the nested table is presented one
			// table deeper than the two outer ones and answers the same-level key for neither of them.
			ID: axDocNestedCell, Parent: axDocRow2, Children: []accessibility.NodeID{axDocNestedTable},
			Role: role.Cell, Bounds: geom.NewRect(20, 250, 260, 40), RowIndex: 0,
		},
		{
			ID: axDocNestedTable, Parent: axDocNestedCell, Children: []accessibility.NodeID{axDocNestedRow},
			Name: "Details", Role: role.Table, Bounds: geom.NewRect(24, 254, 250, 20), RowCount: 1, ColumnCount: 1,
		},
		{
			ID: axDocNestedRow, Parent: axDocNestedTable, Children: []accessibility.NodeID{axDocNestedTableCell},
			Role: role.Row, Bounds: geom.NewRect(24, 254, 250, 20), RowIndex: 0,
		},
		axDocBlock(axDocNestedTableCell, axDocNestedRow, role.Cell, "Two", 254,
			[]accessibility.TextRun{axDocRun(0, 3, axDocSerif, 13, 400)}),
		{
			ID: axDocForm, Parent: axDocRoot,
			Children: []accessibility.NodeID{axDocField, axDocButton, axDocRadios, axDocOutline}, Name: "Options",
			Role: role.Group, Bounds: geom.NewRect(10, 340, 300, 130),
		},
		{
			ID: axDocField, Parent: axDocForm, Name: "Author", Value: "Ada Lovelace", Role: role.TextField,
			Bounds: geom.NewRect(20, 340, 200, 24), Focusable: true,
			Text: &accessibility.TextInfo{Text: "Ada Lovelace", SelStart: 12, SelEnd: 12, Caret: 12},
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetValue,
				accessibility.SetTextSelection),
		},
		{
			ID: axDocButton, Parent: axDocForm, Name: "OK", Role: role.Button,
			Bounds: geom.NewRect(20, 370, 80, 24), Focusable: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
		{
			ID: axDocRadios, Parent: axDocForm, Children: []accessibility.NodeID{axDocRadioSmall, axDocRadioLarge},
			Name: "Size", Role: role.Group, Bounds: geom.NewRect(20, 400, 200, 40),
		},
		{
			ID: axDocRadioSmall, Parent: axDocRadios, Name: "Small", Role: role.RadioButton,
			Bounds: geom.NewRect(20, 400, 200, 20), Focusable: true, HasCheck: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
		{
			ID: axDocRadioLarge, Parent: axDocRadios, Name: "Large", Role: role.RadioButton,
			Bounds: geom.NewRect(20, 420, 200, 20), Focusable: true, HasCheck: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
		{
			ID: axDocOutline, Parent: axDocForm, Name: "Documents", Role: role.Tree,
			Bounds: geom.NewRect(20, 445, 200, 24), Focusable: true,
			Actions: accessibility.ActionSet(0).With(accessibility.Focus),
		},
	}
	tree := &accessibility.Tree{
		Nodes:      make(map[accessibility.NodeID]*accessibility.Node, len(nodes)),
		Root:       axDocRoot,
		Focus:      axDocField,
		Generation: 1,
	}
	for _, n := range nodes {
		tree.Nodes[n.ID] = n
	}
	return tree
}

// axDocNames turns a list of node ids into the fixture's own names for them, so that a failing comparison reads as the
// nodes it is about rather than as a row of numbers.
func axDocNames(ids []accessibility.NodeID) string {
	names := map[accessibility.NodeID]string{
		axDocRoot: "root", axDocDocument: "document", axDocHeading1: "heading1", axDocParagraph: "paragraph",
		axDocLink: "link", axDocImage: "image", axDocIgnoredGroup: "ignoredGroup", axDocHeading2: "heading2",
		axDocSeparator: "separator", axDocQuote: "quote", axDocQuoteParagraph: "quoteParagraph",
		axDocInnerQuote: "innerQuote", axDocInnerParagraph: "innerParagraph", axDocList: "list", axDocItem1: "item1",
		axDocItem1Paragraph: "item1Paragraph", axDocItem2: "item2", axDocItem2Paragraph: "item2Paragraph",
		axDocQuote2: "quote2", axDocQuote2Paragraph: "quote2Paragraph", axDocCode: "code",
		axDocHeading1b: "heading1b", axDocTable: "table", axDocRow: "row", axDocCell: "cell",
		axDocCheckCell: "checkCell", axDocCheck: "check", axDocTable2: "table2", axDocRow2: "row2",
		axDocNestedCell: "nestedCell", axDocNestedTable: "nestedTable", axDocNestedRow: "nestedRow",
		axDocNestedTableCell: "nestedTableCell", axDocForm: "form", axDocField: "field",
		axDocButton: "button", axDocRadios: "radios", axDocRadioSmall: "radioSmall", axDocRadioLarge: "radioLarge",
		axDocOutline: "outline",
	}
	out := make([]string, len(ids))
	for i, id := range ids {
		if name, ok := names[id]; ok {
			out[i] = name
			continue
		}
		out[i] = fmt.Sprintf("%d", id)
	}
	return "[" + fmt.Sprint(out) + "]"
}

// axCheckSearch fails the test unless a search answered with exactly the nodes wanted, in that order.
func axCheckSearch(t *testing.T, what string, got, want []accessibility.NodeID) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s answered %s, want %s", what, axDocNames(got), axDocNames(want))
	}
}

// TestAXSearchOrderIsPresentedPreOrder proves a search walks the hierarchy an assistive technology can actually see:
// the presented children of each node in turn, depth first. The ignored group is spliced out and the heading inside it
// takes its place, the separator is dropped, and the cell holding nothing but a check box is stood in for by that check
// box — the same three decisions every other answer this adapter gives is made with. A walk over the snapshot's own
// children would hand a client elements that appear nowhere in the hierarchy it was shown.
func TestAXSearchOrderIsPresentedPreOrder(t *testing.T) {
	tree := axDocTestTree()
	axCheckSearch(t, "the presented order", axSearchOrder(tree, tree.Root, false), axDocPresentedOrder)
	for _, id := range []accessibility.NodeID{
		axDocRoot, axDocIgnoredGroup, axDocSeparator, axDocCheckCell, axDocNestedCell,
	} {
		if slices.Contains(axSearchOrder(tree, tree.Root, false), id) {
			t.Errorf("%s is in the presented order, and nothing presents it", axDocNames([]accessibility.NodeID{id}))
		}
	}
	// A scope of its own searches only what is inside it, and the scope itself is never among the answers.
	axCheckSearch(t, "the order within the quote", axSearchOrder(tree, axDocQuote, false),
		[]accessibility.NodeID{axDocQuoteParagraph, axDocInnerQuote, axDocInnerParagraph})
	// A node with nothing inside it has nothing to search.
	axCheckSearch(t, "the order within a paragraph of a list item", axSearchOrder(tree, axDocItem1Paragraph, false),
		nil)
}

// TestAXSearchDirectionAndStart proves the two halves of moving through the matches: the start element is never an
// answer, so asking again from each answer steps through them, and going backwards answers with the matches before it,
// nearest first. A start element the scope does not hold is the same as naming none, which is what keeps a stale
// element from silently answering nothing.
func TestAXSearchDirectionAndStart(t *testing.T) {
	tree := axDocTestTree()
	headings := axSearchQuery{Keys: []string{axSearchKeyHeading}}
	axCheckSearch(t, "the headings", axSearch(tree, tree.Root, headings),
		[]accessibility.NodeID{axDocHeading1, axDocHeading2, axDocHeading1b})
	from := headings
	from.Start = axDocHeading1
	axCheckSearch(t, "the headings after the first", axSearch(tree, tree.Root, from),
		[]accessibility.NodeID{axDocHeading2, axDocHeading1b})
	from.Start = axDocHeading1b
	axCheckSearch(t, "the headings after the last", axSearch(tree, tree.Root, from), nil)
	back := headings
	back.Previous = true
	axCheckSearch(t, "the headings backwards", axSearch(tree, tree.Root, back),
		[]accessibility.NodeID{axDocHeading1b, axDocHeading2, axDocHeading1})
	back.Start = axDocHeading2
	axCheckSearch(t, "the headings before the second", axSearch(tree, tree.Root, back),
		[]accessibility.NodeID{axDocHeading1})
	// A start element outside the scope names nothing the scope holds, so the search runs from the scope's own end.
	outside := headings
	outside.Start = axDocField
	axCheckSearch(t, "the headings of the document from a start outside it", axSearch(tree, axDocDocument, outside),
		[]accessibility.NodeID{axDocHeading1, axDocHeading2, axDocHeading1b})
	// And a search of a scope that holds no matches answers with nothing rather than climbing out of it.
	axCheckSearch(t, "the headings within the list", axSearch(tree, axDocList, headings), nil)
}

// TestAXSearchKeys proves what each search key answers, which is the whole of what VoiceOver's rotor and Quick Nav ask
// for. The relative keys — the same heading or quote level, the same or a different kind of thing, a change of font or
// of style — are asked with a start element, since without one there is nothing for them to be relative to.
//
//nolint:funlen // one row per search key; the table is the test
func TestAXSearchKeys(t *testing.T) {
	tree := axDocTestTree()
	for _, c := range []struct {
		key   string
		want  []accessibility.NodeID
		start accessibility.NodeID
	}{
		{key: axSearchKeyHeading, want: []accessibility.NodeID{axDocHeading1, axDocHeading2, axDocHeading1b}},
		{key: axSearchKeyHeadingLevel1, want: []accessibility.NodeID{axDocHeading1, axDocHeading1b}},
		{key: axSearchKeyHeadingLevel2, want: []accessibility.NodeID{axDocHeading2}},
		{key: axSearchKeyHeadingLevel3},
		{key: axSearchKeyHeadingSameLevel, start: axDocHeading1, want: []accessibility.NodeID{axDocHeading1b}},
		{key: axSearchKeyHeadingSameLevel, start: axDocHeading2},
		// Nothing for the key to be relative to, so nothing answers it.
		{key: axSearchKeyHeadingSameLevel},
		{key: axSearchKeyLink, want: []accessibility.NodeID{axDocLink}},
		{key: axSearchKeyUnvisitedLink, want: []accessibility.NodeID{axDocLink}},
		// Nothing records where a person has been, so no link is a visited one.
		{key: axSearchKeyVisitedLink},
		{key: axSearchKeyList, want: []accessibility.NodeID{axDocList}},
		{key: axSearchKeyTable, want: []accessibility.NodeID{axDocTable, axDocTable2, axDocNestedTable}},
		// The table at the same level as the first one is the second, and not the one nested inside a cell of it, which
		// sits a table deeper. From that second table nothing is left at its own level, which is the negative half.
		{key: axSearchKeyTableSameLevel, start: axDocTable, want: []accessibility.NodeID{axDocTable2}},
		{key: axSearchKeyTableSameLevel, start: axDocTable2},
		{key: axSearchKeyTableSameLevel, start: axDocNestedTable},
		// Nothing for the key to be relative to, so nothing answers it.
		{key: axSearchKeyTableSameLevel},
		{key: axSearchKeyGraphic, want: []accessibility.NodeID{axDocImage}},
		{key: axSearchKeyStaticText, want: axDocStaticTextNodes()},
		{
			key: axSearchKeyPlainText,
			want: []accessibility.NodeID{
				axDocInnerParagraph, axDocItem1Paragraph, axDocQuote2Paragraph, axDocCode,
			},
		},
		{
			key:  axSearchKeyBoldFont,
			want: []accessibility.NodeID{axDocHeading1, axDocParagraph, axDocHeading2, axDocHeading1b},
		},
		{key: axSearchKeyItalicFont, want: []accessibility.NodeID{axDocQuoteParagraph}},
		{key: axSearchKeyUnderline, want: []accessibility.NodeID{axDocItem2Paragraph}},
		{
			// The blocks after the paragraph the search started from that are drawn in another face or size than it: the
			// headings, which are larger, and the code block, which is monospaced. The heading before it is no answer,
			// since a search only ever moves on from where it started.
			key: axSearchKeyFontChange, start: axDocParagraph,
			want: []accessibility.NodeID{axDocHeading2, axDocCode, axDocHeading1b},
		},
		{
			// The blocks after the same paragraph whose first run is styled where its own is plain.
			key: axSearchKeyStyleChange, start: axDocParagraph,
			want: []accessibility.NodeID{
				axDocHeading2, axDocQuoteParagraph, axDocItem2Paragraph, axDocHeading1b,
			},
		},
		{key: axSearchKeyButton, want: []accessibility.NodeID{axDocButton}},
		{key: axSearchKeyCheckBox, want: []accessibility.NodeID{axDocCheck}},
		{key: axSearchKeyTextField, want: []accessibility.NodeID{axDocField}},
		{key: axSearchKeyRadioGroup, want: []accessibility.NodeID{axDocRadios}},
		{
			key: axSearchKeyControl,
			want: []accessibility.NodeID{
				axDocLink, axDocCheck, axDocField, axDocButton, axDocRadioSmall, axDocRadioLarge,
			},
		},
		{key: axSearchKeyBlockquote, want: []accessibility.NodeID{axDocQuote, axDocInnerQuote, axDocQuote2}},
		{key: axSearchKeyBlockquoteSameLevel, start: axDocQuote, want: []accessibility.NodeID{axDocQuote2}},
		{key: axSearchKeyBlockquoteSameLevel, start: axDocInnerQuote},
		{key: axSearchKeyOutline, want: []accessibility.NodeID{axDocOutline}},
		{key: axSearchKeyFrame, want: []accessibility.NodeID{axDocDocument}},
		{
			key: axSearchKeyKeyboardFocusable,
			want: []accessibility.NodeID{
				axDocDocument, axDocLink, axDocCheck, axDocField, axDocButton, axDocRadioSmall, axDocRadioLarge,
				axDocOutline,
			},
		},
		{
			// A paragraph, a code block and a label are one kind of element on this platform, so "the same type" as a
			// paragraph is the next block of text whatever the snapshot calls it — including a heading on a system where
			// a heading is reported as static text.
			key: axSearchKeySameType, start: axDocParagraph,
			want: axDocNodesAfter(axDocParagraph, axDocStaticTextNodes()),
		},
		{key: axSearchKeySameType},
		{key: axSearchKeyDifferentType},
		// The keys nothing a snapshot records can answer, and a key this adapter has never heard of.
		{key: axSearchKeyLandmark},
		{key: axSearchKeyArticle},
		{key: axSearchKeyLiveRegion},
		{key: axSearchKeyMisspelledWord},
		{key: axSearchKeyFontColorChange},
		{key: "AXBogusSearchKey"},
	} {
		got := axSearch(tree, tree.Root, axSearchQuery{Keys: []string{c.key}, Start: c.start})
		axCheckSearch(t, fmt.Sprintf("%s from %s", c.key, axDocNames([]accessibility.NodeID{c.start})), got, c.want)
	}
	// Asking for anything at all, whether by naming the key or by naming no key, answers with the whole presented
	// hierarchy in order.
	axCheckSearch(t, "any type", axSearch(tree, tree.Root, axSearchQuery{Keys: []string{axSearchKeyAnyType}}),
		axDocPresentedOrder)
	axCheckSearch(t, "no key at all", axSearch(tree, tree.Root, axSearchQuery{}), axDocPresentedOrder)
	// Several keys are an either-or, which is how the rotor asks for a list of more than one kind of thing.
	axCheckSearch(t, "headings or links",
		axSearch(tree, tree.Root, axSearchQuery{Keys: []string{axSearchKeyHeading, axSearchKeyLink}}),
		[]accessibility.NodeID{axDocHeading1, axDocLink, axDocHeading2, axDocHeading1b})
	// A different kind of thing than the paragraph the search started from is everything else it goes on to reach, so the
	// two searches together account for every node after the start element and nothing before it.
	same := axSearch(tree, tree.Root, axSearchQuery{Keys: []string{axSearchKeySameType}, Start: axDocParagraph})
	different := axSearch(tree, tree.Root,
		axSearchQuery{Keys: []string{axSearchKeyDifferentType}, Start: axDocParagraph})
	after := len(axDocPresentedOrder) - (slices.Index(axDocPresentedOrder, axDocParagraph) + 1)
	if got := len(same) + len(different); got != after {
		t.Errorf("the same and different type searches cover %d nodes between them, want %d", got, after)
	}
}

// TestAXSearchTextFilter proves the text a predicate may carry narrows the answers to the nodes that say it, whatever
// the case, and that it narrows rather than replaces the keys: a search for links holding "guide" answers with the link
// and not with the paragraph that also mentions it.
func TestAXSearchTextFilter(t *testing.T) {
	tree := axDocTestTree()
	axCheckSearch(t, "a search for the word guide", axSearch(tree, tree.Root, axSearchQuery{Text: "GUIDE"}),
		[]accessibility.NodeID{axDocParagraph, axDocLink})
	axCheckSearch(t, "a search for links holding the word guide",
		axSearch(tree, tree.Root, axSearchQuery{Keys: []string{axSearchKeyLink}, Text: "guide"}),
		[]accessibility.NodeID{axDocLink})
	axCheckSearch(t, "a search for text nothing holds",
		axSearch(tree, tree.Root, axSearchQuery{Text: "not in this window"}), nil)
	// The name of a block whose name is reported nowhere is searched nowhere either: the paragraph is found by what it
	// says, not by what a widget called it.
	axCheckSearch(t, "a search for the paragraph's unreported name",
		axSearch(tree, tree.Root, axSearchQuery{Text: "Intro"}), nil)
	// A node's own name is what it is heard saying, so a control is found by it as well as a block of text by its
	// content.
	axCheckSearch(t, "a search for the word small", axSearch(tree, tree.Root, axSearchQuery{Text: "small"}),
		[]accessibility.NodeID{axDocRadioSmall})
	// Everything a node is heard saying is searched, not just the first of those things it happens to have: the field is
	// spoken as "Author" and as "Ada Lovelace", so a search for either has to find it.
	for _, text := range []string{"author", "Lovelace"} {
		axCheckSearch(t, "a search for "+text, axSearch(tree, tree.Root, axSearchQuery{Text: text}),
			[]accessibility.NodeID{axDocField})
	}
	// Including the description, which is spoken after the name and the value and is what WebKit searches too.
	tree.Node(axDocButton).Description = "Applies the changes"
	axCheckSearch(t, "a search for a word only a description holds",
		axSearch(tree, tree.Root, axSearchQuery{Text: "applies"}), []accessibility.NodeID{axDocButton})
}

// TestAXSearchStartRange proves a search asked from a position inside a block of text starts from that position rather
// than from the block. With the VoiceOver cursor parked in a paragraph past an inline link, the next node in presented
// order after the paragraph is that very link, so Quick Nav's "next link" would answer with the link already read and
// the cursor would jump backwards; what the nodes the paragraph's text carries before the position get instead is
// nothing.
func TestAXSearchStartRange(t *testing.T) {
	tree := axDocTestTree()
	inside := axSearchQuery{Start: axDocParagraph, StartRange: true}
	// Before the link, both the link and the image are still ahead.
	inside.StartOffset = axDocLinkStart
	axCheckSearch(t, "everything in the paragraph from before its link", axSearch(tree, axDocParagraph, inside),
		[]accessibility.NodeID{axDocLink, axDocImage})
	// Past the link, only the image is.
	inside.StartOffset = axDocLinkEnd
	axCheckSearch(t, "everything in the paragraph from past its link", axSearch(tree, axDocParagraph, inside),
		[]accessibility.NodeID{axDocImage})
	// A position inside the link itself has not passed it: the span ends after the offset, so the link is still an
	// answer, which is what keeps a cursor in the middle of a link from skipping it.
	inside.StartOffset = axDocLinkEnd - 1
	axCheckSearch(t, "everything in the paragraph from inside its link", axSearch(tree, axDocParagraph, inside),
		[]accessibility.NodeID{axDocLink, axDocImage})
	// Past the image as well, nothing in the paragraph is left — and the search does not climb out of the scope it was
	// asked of.
	inside.StartOffset = axDocImageEnd
	axCheckSearch(t, "everything in the paragraph from past its image", axSearch(tree, axDocParagraph, inside), nil)
	// The same position, searched from the document: the link is behind the cursor and the image is not, so "the next
	// link" is no link at all while the next graphic is the image.
	links := axSearchQuery{
		Keys:        []string{axSearchKeyLink, axSearchKeyGraphic},
		Start:       axDocParagraph,
		StartOffset: axDocLinkEnd,
		StartRange:  true,
	}
	axCheckSearch(t, "the links and graphics of the document past the paragraph's link",
		axSearch(tree, axDocDocument, links), []accessibility.NodeID{axDocImage})
	// Without the range the same search answers with the link the cursor has already read, which is the behavior the
	// key exists to refine.
	whole := links
	whole.StartRange = false
	axCheckSearch(t, "the links and graphics of the document after the whole paragraph",
		axSearch(tree, axDocDocument, whole), []accessibility.NodeID{axDocLink, axDocImage})
	// A start range on an element that holds no text says nothing, since there is no position in it to be past.
	onControl := axSearchQuery{Start: axDocButton, StartOffset: 3, StartRange: true}
	axCheckSearch(t, "everything after the button", axSearch(tree, axDocForm, onControl),
		[]accessibility.NodeID{axDocRadios, axDocRadioSmall, axDocRadioLarge, axDocOutline})
}

// TestAXSearchButtonAndCheckBoxKeys proves the two keys that ask for something pressable answer with everything a
// person presses, which is what WebKit's own button test does: a pop-up button is an AXPopUpButton, so no key but the
// control one would offer it and VoiceOver's "Buttons" rotor and Quick Nav's B skipped every one of them, and a toggle
// button is pressed like a button whatever the check box role it is reported with.
func TestAXSearchButtonAndCheckBoxKeys(t *testing.T) {
	const (
		rootID accessibility.NodeID = 1800 + iota
		buttonID
		popupID
		toggleID
		checkID
		headerID
		labelID
	)
	node := func(id accessibility.NodeID, r role.Enum, name string, y float32) *accessibility.Node {
		return &accessibility.Node{
			ID: id, Parent: rootID, Name: name, Role: r, Bounds: geom.NewRect(0, y, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		}
	}
	tree := &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID, Children: []accessibility.NodeID{buttonID, popupID, toggleID, checkID, headerID, labelID},
				Role: role.Window, Bounds: geom.NewRect(0, 0, 100, 140),
			},
			buttonID: node(buttonID, role.Button, "OK", 0),
			popupID:  node(popupID, role.PopupButton, "Size", 20),
			toggleID: node(toggleID, role.ToggleButton, "Bold", 40),
			checkID:  node(checkID, role.CheckBox, "Done", 60),
			headerID: node(headerID, role.ColumnHeader, "Name", 80),
			labelID:  node(labelID, role.Label, "Ready", 100),
		},
		Root:       rootID,
		Generation: 1,
	}
	got := axSearch(tree, rootID, axSearchQuery{Keys: []string{axSearchKeyButton}})
	if want := []accessibility.NodeID{buttonID, popupID, toggleID, headerID}; !slices.Equal(got, want) {
		t.Errorf("the button key answered %v, want %v", got, want)
	}
	// A toggle button answers the check box key as well, since AXCheckBox with the toggle subrole is what a client is
	// told it is.
	got = axSearch(tree, rootID, axSearchQuery{Keys: []string{axSearchKeyCheckBox}})
	if want := []accessibility.NodeID{toggleID, checkID}; !slices.Equal(got, want) {
		t.Errorf("the check box key answered %v, want %v", got, want)
	}
}

// TestAXSearchLimitVisibleOnlyAndImmediateDescendants proves the three narrowings a predicate may carry besides its
// keys: how many answers are wanted, whether the nodes scrolled out of sight count, and whether the search descends at
// all.
func TestAXSearchLimitVisibleOnlyAndImmediateDescendants(t *testing.T) {
	tree := axDocTestTree()
	limited := axSearchQuery{Keys: []string{axSearchKeyHeading}, Limit: 2}
	axCheckSearch(t, "the first two headings", axSearch(tree, tree.Root, limited),
		[]accessibility.NodeID{axDocHeading1, axDocHeading2})
	limited.Limit = 1
	limited.Previous = true
	axCheckSearch(t, "the nearest heading backwards", axSearch(tree, tree.Root, limited),
		[]accessibility.NodeID{axDocHeading1b})
	// A limit of zero — which is what an absent AXResultsLimit means — asks for all of them.
	limited.Limit = 0
	limited.Previous = false
	if got := len(axSearch(tree, tree.Root, limited)); got != 3 {
		t.Errorf("an unlimited search for headings answered %d nodes, want 3", got)
	}
	// The second list item and its paragraph are scrolled out of sight, so a search for the visible nodes only leaves
	// both of them out.
	axCheckSearch(t, "everything in the list", axSearch(tree, axDocList, axSearchQuery{}),
		[]accessibility.NodeID{axDocItem1, axDocItem1Paragraph, axDocItem2, axDocItem2Paragraph})
	axCheckSearch(t, "everything visible in the list", axSearch(tree, axDocList, axSearchQuery{VisibleOnly: true}),
		[]accessibility.NodeID{axDocItem1, axDocItem1Paragraph})
	// Immediate descendants only stops at the document's own blocks: the link and the image inside a paragraph, and the
	// paragraphs inside the quotes and the list, are a level further down.
	axCheckSearch(t, "the document's own children",
		axSearch(tree, axDocDocument, axSearchQuery{ImmediateOnly: true}),
		[]accessibility.NodeID{
			axDocHeading1, axDocParagraph, axDocHeading2, axDocQuote, axDocList, axDocQuote2, axDocCode,
			axDocHeading1b, axDocTable, axDocTable2,
		})
	axCheckSearch(t, "the document's own headings",
		axSearch(tree, axDocDocument, axSearchQuery{Keys: []string{axSearchKeyHeading}, ImmediateOnly: true}),
		[]accessibility.NodeID{axDocHeading1, axDocHeading2, axDocHeading1b})
}

// TestAXSearchCannotLoop proves a malformed snapshot cannot make a search run forever. A search runs inside an AppKit
// callback, so a walk that never finished would hang the whole application rather than merely answer wrongly: a node
// already reached is never descended into again, and the descent is bounded besides.
func TestAXSearchCannotLoop(t *testing.T) {
	const (
		rootID accessibility.NodeID = 1700 + iota
		groupID
		labelID
	)
	// The group's children name the root again, so the hierarchy is a cycle rather than a tree.
	tree := &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID, Children: []accessibility.NodeID{groupID}, Role: role.Window,
				Bounds: geom.NewRect(0, 0, 100, 100),
			},
			groupID: {
				ID: groupID, Parent: rootID, Children: []accessibility.NodeID{labelID, rootID}, Name: "Group",
				Role: role.Group, Bounds: geom.NewRect(0, 0, 100, 50),
			},
			labelID: {
				ID: labelID, Parent: groupID, Name: "Cycled", Role: role.Label, Bounds: geom.NewRect(0, 0, 60, 16),
			},
		},
		Root:       rootID,
		Generation: 1,
	}
	order := axSearchOrder(tree, rootID, false)
	axCheckSearch(t, "the presented order of a cyclic hierarchy", order,
		[]accessibility.NodeID{groupID, labelID})
	if got := axSearch(tree, rootID, axSearchQuery{Keys: []string{axSearchKeyStaticText}}); len(got) != 1 {
		t.Errorf("a search of a cyclic hierarchy for static text answered %d nodes, want 1", len(got))
	}
}

// TestAXSearchKeysOnLabels proves what the rotor and Quick Nav find now that an ordinary label carries the text it
// drew. The text keys answer from what a node holds rather than from where it sits, so a form's labels are found by
// the same keys a document's paragraphs are: the static-text key reaches every one of them, the plain-text key
// reaches only the ones drawn in one unremarkable style — a label drawn bold is exactly what a person stepping by
// "plain text" is trying to skip — and the font keys, which no label could ever have answered before, now match the
// runs a label was drawn with.
//
// The keys that have nothing to do with text are the control: a heading is still found by the heading keys and by
// them alone, and a label is keyboard-focusable nowhere on this platform, since VoiceOver moves a cursor of its own
// and headings are given no Focus action here.
func TestAXSearchKeysOnLabels(t *testing.T) {
	const (
		rootID accessibility.NodeID = 1900 + iota
		plainID
		boldID
		headingID
		fieldID
	)
	styled := func(text string, weight int) *accessibility.TextInfo {
		count := len([]rune(text))
		return &accessibility.TextInfo{
			Text: text,
			Runs: []accessibility.TextRun{axDocRun(0, count, axDocSerif, 13, weight)},
			Lines: []accessibility.Line{
				{
					Advances: axDocAdvances(count, 7), Start: 0, End: count,
					Bounds: geom.NewRect(0, 0, float32(count)*7, 16),
				},
			},
		}
	}
	tree := &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			rootID: {
				ID: rootID, Children: []accessibility.NodeID{plainID, boldID, headingID, fieldID}, Role: role.Window,
				Bounds: geom.NewRect(0, 0, 320, 240),
			},
			plainID: {
				ID: plainID, Parent: rootID, Role: role.Label, Name: "Stopped", Bounds: geom.NewRect(10, 10, 60, 16),
				Text: styled("Stopped", 400),
			},
			boldID: {
				ID: boldID, Parent: rootID, Role: role.Label, Name: "Warning", Bounds: geom.NewRect(10, 30, 60, 16),
				Text: styled("Warning", 700),
			},
			headingID: {
				ID: headingID, Parent: rootID, Role: role.Heading, Name: "Colors", Level: 2,
				Bounds: geom.NewRect(10, 50, 100, 20), Text: styled("Colors", 700),
			},
			fieldID: {
				ID: fieldID, Parent: rootID, Role: role.TextField, Name: "Owner", Value: "Hopper",
				Bounds: geom.NewRect(10, 80, 200, 24), Focusable: true,
				Text:    &accessibility.TextInfo{Text: "Hopper"},
				Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetValue),
			},
		},
		Root:       rootID,
		Generation: 1,
	}
	// A heading is reported as static text only where the AXHeading role is not honored, so the static-text keys
	// follow the same rule the adapter follows rather than this machine's answer (see axIsStaticText).
	staticText := []accessibility.NodeID{plainID, boldID}
	plainText := []accessibility.NodeID{plainID}
	if axHeadingIsStaticText() {
		staticText = []accessibility.NodeID{plainID, boldID, headingID}
	}
	for _, c := range []struct {
		key  string
		want []accessibility.NodeID
	}{
		{key: axSearchKeyStaticText, want: staticText},
		{key: axSearchKeyPlainText, want: plainText},
		{key: axSearchKeyBoldFont, want: []accessibility.NodeID{boldID, headingID}},
		{key: axSearchKeyItalicFont, want: nil},
		{key: axSearchKeyHeading, want: []accessibility.NodeID{headingID}},
		{key: axSearchKeyHeadingLevel2, want: []accessibility.NodeID{headingID}},
		{key: axSearchKeyHeadingLevel1, want: nil},
		{key: axSearchKeyKeyboardFocusable, want: []accessibility.NodeID{fieldID}},
		{key: axSearchKeyTextField, want: []accessibility.NodeID{fieldID}},
	} {
		if got := axSearch(tree, rootID, axSearchQuery{Keys: []string{c.key}}); !slices.Equal(got, c.want) {
			t.Errorf("%s answered %v, want %v", c.key, got, c.want)
		}
	}
	// Searching by text finds a label by the text it drew, which is the string a person searching heard it speak.
	if got := axSearch(tree, rootID, axSearchQuery{Text: "warn"}); !slices.Equal(got,
		[]accessibility.NodeID{boldID}) {
		t.Errorf("a search for \"warn\" answered %v, want %v", got, []accessibility.NodeID{boldID})
	}
}
