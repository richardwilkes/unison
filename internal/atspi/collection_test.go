// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package atspi

import (
	"slices"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// documentWindow is the window the tests that work over a Markdown document publish.
const documentWindow WindowKey = 8

// The text of every block of the document, and where the two objects within its paragraph sit in that paragraph's text.
// The paragraph holds a link over "the guide" and an image, which is the one U+FFFC in it.
const (
	documentHeadingText     = "Getting started"
	documentParagraphText   = "Read the guide ￼ now."
	documentSplicedText     = "Spliced in."
	documentCodeText        = "go build ./..."
	documentQuotedText      = "Mind the gap."
	documentFirstItemText   = "First"
	documentSecondItemText  = "Second"
	documentNameHeaderText  = "Name"
	documentSizeHeaderText  = "Size"
	documentNameCellText    = "go.mod"
	documentSizeCellText    = "1 KB"
	documentLastHeadingText = "Next steps"
	documentParagraphLength = 21
	paragraphLinkStart      = 5
	paragraphLinkEnd        = 14
	paragraphImageStart     = 15
	paragraphImageEnd       = 16
)

// Where the document's two links lead.
const (
	documentGuideURL = "https://example.com/guide"
	documentHomeURL  = "https://example.com/"
)

// The fonts the document is drawn in: prose in one, code in another, with a heading larger than either. The prose is
// drawn at [regularWeight], which is the weight this package reports for a run that does not say what its own is.
const (
	proseFamily = "Serif"
	codeFamily  = "Mono"
	proseSize   = 12
	codeSize    = 11
	headingSize = 18
	boldWeight  = 700
)

// runeWidth is how wide every rune of the document is, which is what the lines' advances are built from.
const runeWidth = 10

// documentDescendants is how many reported objects the document holds, which is every node beneath it but the ignored
// group, and is what a search that asks for nothing in particular finds.
const documentDescendants = 23

// documentTree is the Markdown document the browse-mode tests work over. It is what markdown.go publishes: a document
// whose composed stream is carried in Document and never exposed, over blocks that each carry their own text.
//
//	100 window "Guide"                     (0,0 300x400)    active
//	└─ 101 document "Notes"                (0,0 300x400)    focusable, focused, holds the composed stream
//	   ├─ 102 heading "Getting started"    (0,0 300x20)     level 1
//	   ├─ 103 paragraph                    (0,20 300x40)    two lines, a link and an image within its text
//	   │  ├─ 104 link "the guide"          (50,20 90x20)    occupies runes 5 to 14
//	   │  └─ 105 image "Logo"              (0,40 20x20)     occupies the one U+FFFC, rune 15
//	   ├─ 106 group             [ignored]  (0,60 300x20)
//	   │  └─ 107 paragraph                 (0,60 300x20)    spliced in where the ignored group sits
//	   ├─ 108 code                         (0,80 300x20)    reported as a paragraph, said to be code by its attributes
//	   ├─ 109 block quote "Note"           (0,100 300x20)
//	   │  └─ 110 paragraph                 (10,100 290x20)
//	   ├─ 111 list                         (0,120 300x40)   static: no focus of its own and no rows
//	   │  ├─ 112 list item                 (0,120 300x20)
//	   │  │  └─ 113 paragraph              (20,120 280x20)
//	   │  └─ 114 list item                 (0,140 300x20)
//	   │     └─ 115 paragraph              (20,140 280x20)
//	   ├─ 116 table                        (0,160 300x40)   2 rows, 2 columns
//	   │  ├─ 117 row                       (0,160 300x20)
//	   │  │  ├─ 118 column header "Name"   (0,160 150x20)
//	   │  │  └─ 119 column header "Size"   (150,160 150x20)
//	   │  └─ 120 row                       (0,180 300x20)
//	   │     ├─ 121 cell                   (0,180 150x20)
//	   │     └─ 122 cell                   (150,180 150x20)
//	   ├─ 123 separator                    (0,200 300x2)
//	   ├─ 124 heading "Next steps"         (0,210 300x20)   level 2
//	   └─ 125 link "Home"                  (0,230 100x20)   a link of its own rather than one within a text
func documentTree() *accessibility.Tree {
	tree := treeOf(1,
		&accessibility.Node{
			ID: 100, Role: role.Window, Name: "Guide", Focused: true, Bounds: geom.NewRect(0, 0, 300, 400),
			Children: []accessibility.NodeID{101},
		},
		&accessibility.Node{
			ID: 101, Parent: 100, Role: role.Document, Name: "Notes", Focusable: true, Focused: true,
			Bounds:   geom.NewRect(0, 0, 300, 400),
			Document: documentStream(),
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection,
				accessibility.ScrollRangeIntoView, accessibility.ShowContextMenu),
			Children: []accessibility.NodeID{102, 103, 106, 108, 109, 111, 116, 123, 124, 125},
		},
		documentHeading(102, documentHeadingText, 1, geom.NewRect(0, 0, 300, 20)),
		documentParagraphWithObjects(),
		&accessibility.Node{
			ID: 104, Parent: 103, Role: role.Link, Name: "the guide", URL: documentGuideURL,
			Bounds:  geom.NewRect(50, 20, 90, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
		&accessibility.Node{ID: 105, Parent: 103, Role: role.Image, Name: "Logo", Bounds: geom.NewRect(0, 40, 20, 20)},
		&accessibility.Node{
			ID: 106, Parent: 101, Role: role.Group, Ignored: true, Bounds: geom.NewRect(0, 60, 300, 20),
			Children: []accessibility.NodeID{107},
		},
		documentBlock(107, 106, role.Paragraph, documentSplicedText, geom.NewRect(0, 60, 300, 20)),
		documentCode(108, documentCodeText, geom.NewRect(0, 80, 300, 20)),
		&accessibility.Node{
			ID: 109, Parent: 101, Role: role.BlockQuote, Name: "Note", Bounds: geom.NewRect(0, 100, 300, 20),
			Children: []accessibility.NodeID{110},
		},
		documentBlock(110, 109, role.Paragraph, documentQuotedText, geom.NewRect(10, 100, 290, 20)),
		&accessibility.Node{
			ID: 111, Parent: 101, Role: role.List, Bounds: geom.NewRect(0, 120, 300, 40),
			Children: []accessibility.NodeID{112, 114},
		},
		&accessibility.Node{
			ID: 112, Parent: 111, Role: role.ListItem, Bounds: geom.NewRect(0, 120, 300, 20),
			Children: []accessibility.NodeID{113},
		},
		documentBlock(113, 112, role.Paragraph, documentFirstItemText, geom.NewRect(20, 120, 280, 20)),
		&accessibility.Node{
			ID: 114, Parent: 111, Role: role.ListItem, RowIndex: 1, Bounds: geom.NewRect(0, 140, 300, 20),
			Children: []accessibility.NodeID{115},
		},
		documentBlock(115, 114, role.Paragraph, documentSecondItemText, geom.NewRect(20, 140, 280, 20)),
		&accessibility.Node{
			ID: 116, Parent: 101, Role: role.Table, RowCount: 2, ColumnCount: 2, Bounds: geom.NewRect(0, 160, 300, 40),
			Children: []accessibility.NodeID{117, 120},
		},
		&accessibility.Node{
			ID: 117, Parent: 116, Role: role.Row, Bounds: geom.NewRect(0, 160, 300, 20),
			Children: []accessibility.NodeID{118, 119},
		},
		documentCell(118, 117, role.ColumnHeader, documentNameHeaderText, 0, 0, geom.NewRect(0, 160, 150, 20)),
		documentCell(119, 117, role.ColumnHeader, documentSizeHeaderText, 0, 1, geom.NewRect(150, 160, 150, 20)),
		&accessibility.Node{
			ID: 120, Parent: 116, Role: role.Row, RowIndex: 1, Bounds: geom.NewRect(0, 180, 300, 20),
			Children: []accessibility.NodeID{121, 122},
		},
		documentCell(121, 120, role.Cell, documentNameCellText, 1, 0, geom.NewRect(0, 180, 150, 20)),
		documentCell(122, 120, role.Cell, documentSizeCellText, 1, 1, geom.NewRect(150, 180, 150, 20)),
		&accessibility.Node{ID: 123, Parent: 101, Role: role.Separator, Bounds: geom.NewRect(0, 200, 300, 2)},
		documentHeading(124, documentLastHeadingText, 2, geom.NewRect(0, 210, 300, 20)),
		&accessibility.Node{
			ID: 125, Parent: 101, Role: role.Link, Name: "Home", URL: documentHomeURL,
			Bounds:  geom.NewRect(0, 230, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press, accessibility.Focus),
		},
	)
	// The document holds the keyboard focus. A block that holds the caret reports being focused as well — see
	// [TestCaretBlockCarriesFocusedState] — so which node the focus is really on is said here rather than left to
	// whichever focused node was listed last.
	tree.Focus = 101
	return tree
}

// documentBlocks lists the blocks of the document that carry text, in the order the document composes them, along with
// the text each one holds.
var documentBlocks = []struct {
	text string
	node accessibility.NodeID
}{
	{node: 102, text: documentHeadingText},
	{node: 103, text: documentParagraphText},
	{node: 107, text: documentSplicedText},
	{node: 108, text: documentCodeText},
	{node: 110, text: documentQuotedText},
	{node: 113, text: documentFirstItemText},
	{node: 115, text: documentSecondItemText},
	{node: 118, text: documentNameHeaderText},
	{node: 119, text: documentSizeHeaderText},
	{node: 121, text: documentNameCellText},
	{node: 122, text: documentSizeCellText},
	{node: 124, text: documentLastHeadingText},
}

// documentStream returns the one stream of text the document composes from the blocks beneath it: their text joined by
// line feeds, with a span saying which part of the stream each block occupies.
//
// AT-SPI never sees any of it. The document object is given no org.a11y.atspi.Text at all — Orca reads a document by
// walking the objects within it, and an object that also handed over the whole content as text would have everything
// read twice — so this is here to prove that none of it leaks; see [TestDocumentHasNoTextInterface].
func documentStream() *accessibility.DocumentInfo {
	var text strings.Builder
	info := &accessibility.DocumentInfo{Text: accessibility.TextInfo{Multiline: true}}
	offset := 0
	for i, block := range documentBlocks {
		if i != 0 {
			text.WriteByte('\n')
			offset++
		}
		length := len([]rune(block.text))
		info.Text.Spans = append(info.Text.Spans, accessibility.TextSpan{
			Node:  block.node,
			Start: offset,
			End:   offset + length,
		})
		text.WriteString(block.text)
		offset += length
	}
	info.Text.Text = text.String()
	return info
}

// documentBlock returns one text-bearing block of the document, which is what Markdown publishes for a paragraph: its
// content cannot be changed, it is laid out over one line here, and it offers to have its selection set and a range of
// it scrolled into view, which is what Orca's caret navigation asks for.
func documentBlock(id, parent accessibility.NodeID, r role.Enum, text string,
	bounds geom.Rect,
) *accessibility.Node {
	return &accessibility.Node{
		ID: id, Parent: parent, Role: r, ReadOnly: true, Bounds: bounds,
		Actions: blockActions(),
		Text: &accessibility.TextInfo{
			Text:  text,
			Runs:  []accessibility.TextRun{proseRun(0, len([]rune(text)))},
			Lines: []accessibility.Line{documentLine(text, 0, bounds.Height)},
		},
	}
}

// documentHeading returns one heading of the document, which carries its level and is drawn larger than the prose.
func documentHeading(id accessibility.NodeID, text string, level int, bounds geom.Rect) *accessibility.Node {
	n := documentBlock(id, 101, role.Heading, text, bounds)
	n.Name = text
	n.Level = level
	n.Text.Runs = []accessibility.TextRun{
		{Start: 0, End: len([]rune(text)), Family: proseFamily, Size: headingSize, Weight: boldWeight},
	}
	return n
}

// documentCode returns the document's code block, which is drawn in a fixed-pitch face.
func documentCode(id accessibility.NodeID, text string, bounds geom.Rect) *accessibility.Node {
	n := documentBlock(id, 101, role.Code, text, bounds)
	n.Text.Runs = []accessibility.TextRun{
		{Start: 0, End: len([]rune(text)), Family: codeFamily, Size: codeSize, Weight: regularWeight, Monospace: true},
	}
	return n
}

// documentCell returns one cell of the document's table, which carries where in the grid it sits along with its text.
func documentCell(id, parent accessibility.NodeID, r role.Enum, text string, row, column int,
	bounds geom.Rect,
) *accessibility.Node {
	n := documentBlock(id, parent, r, text, bounds)
	n.RowIndex = row
	n.ColumnIndex = column
	return n
}

// documentParagraphWithObjects returns the paragraph that holds a link and an image: three styled runs, a span for each
// object, and two laid-out lines, the second of which begins at the image.
func documentParagraphWithObjects() *accessibility.Node {
	bounds := geom.NewRect(0, 20, 300, 40)
	return &accessibility.Node{
		ID: 103, Parent: 101, Role: role.Paragraph, ReadOnly: true, Bounds: bounds,
		Actions:  blockActions(),
		Children: []accessibility.NodeID{104, 105},
		Text: &accessibility.TextInfo{
			Text:      documentParagraphText,
			Multiline: true,
			Runs: []accessibility.TextRun{
				proseRun(0, paragraphLinkStart),
				{
					Start: paragraphLinkStart, End: paragraphLinkEnd, Family: proseFamily, Size: proseSize,
					Weight: boldWeight, Underline: true,
				},
				proseRun(paragraphLinkEnd, documentParagraphLength),
			},
			Spans: []accessibility.TextSpan{
				{Node: 104, Start: paragraphLinkStart, End: paragraphLinkEnd},
				{Node: 105, Start: paragraphImageStart, End: paragraphImageEnd},
			},
			Lines: []accessibility.Line{
				lineOfRunes(0, paragraphImageStart, geom.NewRect(0, 0, paragraphImageStart*runeWidth, 20)),
				lineOfRunes(paragraphImageStart, documentParagraphLength,
					geom.NewRect(0, 20, (documentParagraphLength-paragraphImageStart)*runeWidth, 20)),
			},
		},
	}
}

// blockActions is what every block of a document offers: move the caret or the selection, and bring a range of the text
// into view. Neither is one of the things AT-SPI's Action interface covers, so a block that offers only these has
// nothing to do.
func blockActions() accessibility.ActionSet {
	return accessibility.ActionSet(0).With(accessibility.SetTextSelection, accessibility.ScrollRangeIntoView)
}

// proseRun is one run of the document's ordinary prose.
func proseRun(start, end int) accessibility.TextRun {
	return accessibility.TextRun{
		Start:  start,
		End:    end,
		Family: proseFamily,
		Size:   proseSize,
		Weight: regularWeight,
	}
}

// documentLine returns the one laid-out line of a block whose text fits on it, measured at [runeWidth] per rune.
func documentLine(text string, y, height float32) accessibility.Line {
	count := len([]rune(text))
	return lineOfRunes(0, count, geom.NewRect(0, y, float32(count)*runeWidth, height))
}

// lineOfRunes returns one laid-out line covering a range of runes, with an advance for every rune boundary on it.
func lineOfRunes(start, end int, bounds geom.Rect) accessibility.Line {
	advances := make([]float32, 0, end-start+1)
	for i := range end - start + 1 {
		advances = append(advances, float32(i*runeWidth))
	}
	return accessibility.Line{Start: start, End: end, Bounds: bounds, Advances: advances}
}

// putCaretIn moves one block's caret to a rune offset with nothing selected, which is what a reader arrowing through a
// document does to the block they are in.
func putCaretIn(n *accessibility.Node, offset int) {
	n.Text.SelStart, n.Text.SelEnd, n.Text.Caret = offset, offset, offset
}

// newDocumentAdapter starts an adapter and publishes the document window, leaving the signals both publishes sent
// unread, since none of the tests that use it are about them.
func newDocumentAdapter(t *testing.T) *testAdapter {
	t.Helper()
	ta := newTestAdapter(t)
	ta.Publish(documentWindow, documentTree(), nil, sampleGeometry())
	return ta
}

// testRule is a match rule as a test writes one, which [testRule.encode] turns into the structure the wire carries; see
// [matchRuleSignature].
type testRule struct {
	attributes      dbus.Dict
	interfaces      []string
	states          []StateBit
	roles           []Role
	statesMatch     MatchType
	attributesMatch MatchType
	rolesMatch      MatchType
	interfacesMatch MatchType
	invert          bool
}

// encode returns the rule as the structure a Collection method takes.
func (r *testRule) encode() dbus.Struct {
	attributes := r.attributes
	if attributes == nil {
		attributes = dbus.Dict{}
	}
	interfaces := r.interfaces
	if interfaces == nil {
		interfaces = []string{}
	}
	return dbus.Struct{
		stateWordsOf(r.states), int32(r.statesMatch),
		attributes, int32(r.attributesMatch),
		roleWordsOf(r.roles), int32(r.rolesMatch),
		interfaces, int32(r.interfacesMatch),
		r.invert,
	}
}

// rolesRule is the rule Orca's structural navigation builds: any of a set of roles, with nothing else asked about.
func rolesRule(roles ...Role) *testRule {
	return &testRule{roles: roles, rolesMatch: MatchAny}
}

// stateWordsOf returns the two words a state set is carried in.
func stateWordsOf(states []StateBit) []int32 {
	set := StateSet{}.With(states...)
	return []int32{int32(set[0]), int32(set[1])}
}

// roleWordsOf returns the bit array a set of roles is carried in: role r is bit r%32 of word r/32.
func roleWordsOf(roles []Role) []int32 {
	words := make([]int32, 0, 4)
	for _, r := range roles {
		index := int(r) / 32
		for len(words) <= index {
			words = append(words, 0)
		}
		words[index] |= int32(uint32(1) << (uint32(r) % 32))
	}
	return words
}

// nodeRefs is the references to a run of nodes, which is what a search answers with.
func nodeRefs(ids ...accessibility.NodeID) []dbus.ObjectRef {
	refs := make([]dbus.ObjectRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, nodeRef(id))
	}
	return refs
}

// matches asks one object for every descendant that satisfies a rule, in canonical order.
func (ta *testAdapter) matches(id accessibility.NodeID, rule *testRule) []dbus.ObjectRef {
	return ta.matchesWith(id, rule, SortCanonical, 0, true)
}

// matchesWith asks one object for the descendants that satisfy a rule, in the order, the number and the depth the
// caller asks for.
func (ta *testAdapter) matchesWith(id accessibility.NodeID, rule *testRule, order SortOrder, count int32,
	traverse bool,
) []dbus.ObjectRef {
	refs, ok := ta.one(NodePath(id), InterfaceCollection, "GetMatches", matchRuleSignature+"uib", rule.encode(),
		uint32(order), count, traverse).([]dbus.ObjectRef)
	ta.c.True(ok, "GetMatches must answer with object references")
	return refs
}

// matchesBefore asks one object for the descendants that satisfy a rule and come before another object within it.
func (ta *testAdapter) matchesBefore(id, current accessibility.NodeID, rule *testRule, tree TreeTraversal,
	limitScope bool,
) []dbus.ObjectRef {
	refs, ok := ta.one(NodePath(id), InterfaceCollection, "GetMatchesTo",
		objectRefSignature+matchRuleSignature+"uubib", nodeRef(current), rule.encode(), uint32(SortCanonical),
		uint32(tree), limitScope, int32(0), true).([]dbus.ObjectRef)
	ta.c.True(ok, "GetMatchesTo must answer with object references")
	return refs
}

// matchesAfter asks one object for the descendants that satisfy a rule and come after another object within it.
func (ta *testAdapter) matchesAfter(id, current accessibility.NodeID, rule *testRule,
	tree TreeTraversal,
) []dbus.ObjectRef {
	refs, ok := ta.one(NodePath(id), InterfaceCollection, "GetMatchesFrom",
		objectRefSignature+matchRuleSignature+"uuib", nodeRef(current), rule.encode(), uint32(SortCanonical),
		uint32(tree), int32(0), true).([]dbus.ObjectRef)
	ta.c.True(ok, "GetMatchesFrom must answer with object references")
	return refs
}

func TestCollectionIsOnEveryObject(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	for _, id := range []accessibility.NodeID{100, 101, 102, 104, 123, 125} {
		interfaces, ok := ta.one(NodePath(id), InterfaceAccessible, "GetInterfaces", "").([]string)
		c.True(ok)
		c.True(slices.Contains(interfaces, InterfaceCollection), "node %d must offer a collection to search", id)
	}
	// A search from a leaf finds nothing rather than failing: the object is there to be asked, and what is inside it is
	// nothing.
	c.Equal([]dbus.ObjectRef{}, ta.matches(123, rolesRule(RoleParagraph)))
}

// TestCollectionFindsHeadingsAndLinks covers what Orca's H and K commands do: ask the document for the objects of a
// role and move to the first one. The order is the order a person reads them in, which is the order the objects are
// described in.
func TestCollectionFindsHeadingsAndLinks(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal(nodeRefs(102, 124), ta.matches(101, rolesRule(RoleHeading)))
	c.Equal(nodeRefs(104, 125), ta.matches(101, rolesRule(RoleLink)),
		"a link within a paragraph and one standing on its own are both links")
	c.Equal(nodeRefs(116), ta.matches(101, rolesRule(RoleTable)))
	c.Equal(nodeRefs(111), ta.matches(101, rolesRule(RoleList)),
		"a static list is a list rather than a list box")
	c.Equal(nodeRefs(112, 114), ta.matches(101, rolesRule(RoleListItem)))
	c.Equal(nodeRefs(105), ta.matches(101, rolesRule(RoleImage)))
	// Asking for two roles at once is how Orca looks for the next block of any kind.
	c.Equal(nodeRefs(102, 109, 124), ta.matches(101, rolesRule(RoleHeading, RoleBlockQuote)),
		"the matches are in the order they are read in rather than in the order the roles were asked for")
}

// TestCollectionSplicesIgnoredContainers covers the hierarchy a search walks, which is the one an assistive technology
// is shown: an ignored group is not there at all and its children stand in for it, exactly as GetChildren reports them.
func TestCollectionSplicesIgnoredContainers(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	// Every paragraph of the document, including the one under the ignored group and the code block, which AT-SPI
	// reports as a paragraph so that a reader arrowing through the document walks it too.
	c.Equal(nodeRefs(103, 107, 108, 110, 113, 115), ta.matches(101, rolesRule(RoleParagraph)))
	// Nothing a rule that asks for nothing at all matches is the ignored group, since it has no object to match.
	found := ta.matches(101, &testRule{})
	c.False(slices.Contains(found, nodeRef(106)), "an ignored group is not part of the hierarchy that is searched")
	c.True(slices.Contains(found, nodeRef(107)), "its children are")
	c.Equal(documentDescendants, len(found), "a rule with no criteria matches every reported descendant")
}

func TestCollectionSortOrderAndCount(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	rule := rolesRule(RoleHeading)
	c.Equal(nodeRefs(124, 102), ta.matchesWith(101, rule, SortReverseCanonical, 0, true))
	c.Equal(nodeRefs(102), ta.matchesWith(101, rule, SortCanonical, 1, true))
	c.Equal(nodeRefs(124), ta.matchesWith(101, rule, SortReverseCanonical, 1, true),
		"the count is applied after the order, so one match backwards is the last of them")
	c.Equal(nodeRefs(102, 124), ta.matchesWith(101, rule, SortCanonical, 99, true),
		"a count larger than the number of matches is every match")
	c.Equal(nodeRefs(102, 124), ta.matchesWith(101, rule, SortFlow, 0, true),
		"flow order is answered as the reading order")
	c.Equal(nodeRefs(124, 102), ta.matchesWith(101, rule, SortReverseTab, 0, true))
	// Without traverse only the document's own children are considered, so the paragraphs inside the block quote, the
	// list and the table are not found.
	c.Equal(nodeRefs(103, 107, 108), ta.matchesWith(101, rolesRule(RoleParagraph), SortCanonical, 0, false))
}

func TestCollectionMatchesAttributes(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal(nodeRefs(102), ta.matches(101, &testRule{
		attributes:      dbus.Dict{{Key: levelAttribute, Value: "1"}},
		attributesMatch: MatchAll,
	}), "asking for a heading of one level is how Orca's 1 through 6 commands work")
	c.Equal(nodeRefs(102, 124), ta.matches(101, &testRule{
		attributes:      dbus.Dict{{Key: levelAttribute, Value: "1:2"}},
		attributesMatch: MatchAll,
	}), "several values for one attribute are written as one string")
	c.Equal(nodeRefs(108), ta.matches(101, &testRule{
		attributes:      dbus.Dict{{Key: xmlRolesAttribute, Value: codeXMLRole}},
		attributesMatch: MatchAll,
	}), "a code block is a paragraph that says it is code")
	c.Equal(nodeRefs(108), ta.matches(101, &testRule{
		attributes:      dbus.Dict{{Key: tagAttribute, Value: preTag}},
		attributesMatch: MatchAll,
	}))
	// Both of a rule's attributes have to hold for ALL and either of them for ANY.
	both := dbus.Dict{{Key: levelAttribute, Value: "2"}, {Key: tagAttribute, Value: preTag}}
	c.Equal([]dbus.ObjectRef{}, ta.matches(101, &testRule{attributes: both, attributesMatch: MatchAll}))
	c.Equal(nodeRefs(108, 124), ta.matches(101, &testRule{attributes: both, attributesMatch: MatchAny}))
	// NONE finds everything that carries neither, which is every reported descendant but those two.
	none := ta.matches(101, &testRule{attributes: both, attributesMatch: MatchNone})
	c.Equal(documentDescendants-2, len(none))
	c.False(slices.Contains(none, nodeRef(108)))
	c.False(slices.Contains(none, nodeRef(124)))
}

func TestCollectionMatchesStatesAndInterfaces(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	// Every block of the document holds text that cannot be changed.
	blocks := nodeRefs(102, 103, 107, 108, 110, 113, 115, 118, 119, 121, 122, 124)
	c.Equal(blocks, ta.matches(101, &testRule{states: []StateBit{StateReadOnly}, statesMatch: MatchAll}))
	c.Equal(blocks, ta.matches(101, &testRule{interfaces: []string{"Text"}, interfacesMatch: MatchAll}),
		"an interface is asked for by the last part of its name")
	c.Equal(blocks, ta.matches(101, &testRule{
		interfaces:      []string{InterfaceText},
		interfacesMatch: MatchAll,
	}), "or by the whole of it")
	c.Equal(nodeRefs(103), ta.matches(101, &testRule{interfaces: []string{"hypertext"}, interfacesMatch: MatchAny}),
		"only the paragraph that holds objects within its text has a hypertext, and case does not matter")
	c.Equal(nodeRefs(104, 105, 125), ta.matches(101, &testRule{
		interfaces:      []string{"Hyperlink"},
		interfacesMatch: MatchAny,
	}), "the objects within a text and the links are the hyperlinks")
	// The document itself is what holds the focus, which only a search from the window can find.
	c.Equal(nodeRefs(101), ta.matches(100, &testRule{
		states:      []StateBit{StateFocused, StateFocusable},
		statesMatch: MatchAll,
	}))
	c.Equal([]dbus.ObjectRef{}, ta.matches(101, &testRule{
		states:      []StateBit{StateFocused},
		statesMatch: MatchAll,
	}), "nothing inside the document holds the focus")
	// A state no object in the document has, asked for as one that must not be present, matches everything.
	c.Equal(documentDescendants,
		len(ta.matches(101, &testRule{states: []StateBit{StateEditable}, statesMatch: MatchNone})))
}

// TestCollectionEmptyAndInvertedRules covers the two ways of asking about the absence of something: MATCH_EMPTY, which
// asks for an object with nothing of that kind at all, and the invert flag, which turns the whole rule inside out.
func TestCollectionEmptyAndInvertedRules(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	// Every object reports the toolkit it came from and implements at least two interfaces, so nothing has an empty set
	// of either.
	c.Equal([]dbus.ObjectRef{}, ta.matches(101, &testRule{attributesMatch: MatchEmpty}))
	c.Equal([]dbus.ObjectRef{}, ta.matches(101, &testRule{interfacesMatch: MatchEmpty}))
	c.Equal([]dbus.ObjectRef{}, ta.matches(101, &testRule{statesMatch: MatchEmpty}),
		"and every object is visible, showing, enabled and sensitive")
	// MATCH_EMPTY over a criterion that does name something is MATCH_ALL.
	c.Equal(nodeRefs(102, 124), ta.matches(101, &testRule{roles: []Role{RoleHeading}, rolesMatch: MatchEmpty}))
	// An inverted rule matches what the rule as written does not.
	inverted := ta.matches(101, &testRule{roles: []Role{RoleHeading}, rolesMatch: MatchAny, invert: true})
	c.Equal(documentDescendants-2, len(inverted))
	c.False(slices.Contains(inverted, nodeRef(102)))
	c.True(slices.Contains(inverted, nodeRef(103)))
	// A criterion whose comparison was never filled in constrains nothing, so a rule that names roles without saying
	// how to compare them matches everything rather than nothing.
	c.Equal(documentDescendants, len(ta.matches(101, &testRule{roles: []Role{RoleHeading}})))
}

// TestCollectionSearchesFromAnObjectWithinTheDocument covers what Orca's structural navigation really calls: it asks
// the document for the next or the previous match, telling it where the reader is. The nearest match has to come first,
// since it asks for exactly one.
func TestCollectionSearchesFromAnObjectWithinTheDocument(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	headings := rolesRule(RoleHeading)
	c.Equal(nodeRefs(124), ta.matchesAfter(101, 108, headings, TreeInorder))
	c.Equal(nodeRefs(102), ta.matchesBefore(101, 108, headings, TreeInorder, false))
	c.Equal(nodeRefs(108, 107, 103), ta.matchesBefore(101, 110, rolesRule(RoleParagraph), TreeInorder, false),
		"the nearest match comes first, since a client asking for one match is moving to it")
	c.Equal(nodeRefs(113, 115), ta.matchesAfter(101, 110, rolesRule(RoleParagraph), TreeInorder))
	c.Equal([]dbus.ObjectRef{}, ta.matchesAfter(101, 124, headings, TreeInorder),
		"there is nothing after the last heading")
	c.Equal(nodeRefs(102, 124), ta.matchesAfter(101, 101, headings, TreeInorder),
		"everything the collection holds comes after the collection itself")
	c.Equal([]dbus.ObjectRef{}, ta.matchesBefore(101, 101, headings, TreeInorder, false))
	// An object the collection does not hold has no place in its order, so nothing is found: the client is holding a
	// reference from a snapshot that has been replaced.
	c.Equal([]dbus.ObjectRef{}, ta.matchesAfter(101, 4, headings, TreeInorder))
	c.Equal([]dbus.ObjectRef{}, ta.matchesBefore(101, 4, headings, TreeInorder, false))
}

// TestCollectionSearchScope covers the two traversals that narrow a search and the scope limit that GetMatchesTo
// carries, none of which Orca asks for but all of which a client may.
func TestCollectionSearchScope(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	links := rolesRule(RoleLink)
	c.Equal(nodeRefs(104), ta.matchesAfter(101, 103, links, TreeRestrictChildren),
		"restricted to the paragraph, only the link within it is found")
	c.Equal(nodeRefs(125), ta.matchesAfter(101, 103, links, TreeRestrictSibling),
		"restricted to its siblings, only the link beside it is")
	c.Equal(nodeRefs(104, 125), ta.matchesAfter(101, 103, links, TreeInorder))
	// A backward search from the paragraph inside the block quote finds the headings above it, unless it is held within
	// the block quote.
	c.Equal(nodeRefs(102), ta.matchesBefore(101, 110, rolesRule(RoleHeading), TreeInorder, false))
	c.Equal([]dbus.ObjectRef{}, ta.matchesBefore(101, 110, rolesRule(RoleHeading), TreeInorder, true))
	c.Equal(nodeRefs(108, 107, 103), ta.matchesBefore(101, 111, rolesRule(RoleParagraph), TreeRestrictSibling, false),
		"the paragraph inside the block quote is not a sibling of the list, so the search passes over it")
}

// TestCollectionStopsOnceItHasEnoughMatches covers what one forward structural-navigation keypress costs. Orca's H, K,
// L, T and P ask for exactly one match from where the reader is, so the rule needs to be tried only as far as the
// object the reader is about to move to; trying it against the whole of the rest of the document instead is work nobody
// reads, on every keypress, and the attribute criteria are the expensive ones.
//
// A reversed order is the one search that cannot stop early, since the match it answers with first is the one farthest
// from where it started.
func TestCollectionStopsOnceItHasEnoughMatches(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	data := newWindowData(documentTree(), sampleGeometry())
	o := &nodeObject{data: data, node: data.node(101)}
	for _, one := range []struct {
		why        string
		roles      []Role
		matches    []accessibility.NodeID
		order      SortOrder
		count      int
		considered int
	}{
		{
			why:   "the first heading is the first object the document reports, so one candidate is all it takes",
			roles: []Role{RoleHeading}, order: SortCanonical, count: 1, considered: 1,
			matches: []accessibility.NodeID{102},
		},
		{
			why:   "two paragraphs are found by the fifth candidate, the link and the image within the first lying between",
			roles: []Role{RoleParagraph}, order: SortCanonical, count: 2, considered: 5,
			matches: []accessibility.NodeID{103, 107},
		},
		{
			why:   "the block quote is the seventh, and nothing beyond it is looked at",
			roles: []Role{RoleBlockQuote}, order: SortCanonical, count: 1, considered: 7,
			matches: []accessibility.NodeID{109},
		},
		{
			why: "a search for every match has to see every candidate", roles: []Role{RoleHeading},
			order: SortCanonical, count: 0, considered: documentDescendants,
			matches: []accessibility.NodeID{102, 124},
		},
		{
			why: "and so does a reversed one, whose first answer is the last match", roles: []Role{RoleHeading},
			order: SortReverseCanonical, count: 1, considered: documentDescendants,
			matches: []accessibility.NodeID{124},
		},
	} {
		rule, failure := decodeMatchRule(rolesRule(one.roles...).encode())
		c.Nil(failure, one.why)
		considered := 0
		matches := o.matchesOf(rule, one.order, one.count,
			countingCandidates(o.candidateWalk(true), &considered))
		c.Equal(one.matches, matches, one.why)
		c.Equal(one.considered, considered, one.why)
	}

	// The search that starts somewhere within the collection, which is the one Orca really calls, stops the same way:
	// the candidates up to where the reader is are passed over without the rule being tried at all, and the walk ends at
	// the first match after them rather than going on to the link that closes the document.
	rule, failure := decodeMatchRule(rolesRule(RoleHeading).encode())
	c.Nil(failure)
	considered := 0
	matches := o.matchesOf(rule, SortCanonical, 1,
		startingAfter(countingCandidates(o.candidateWalk(true), &considered), 108, false))
	c.Equal([]accessibility.NodeID{124}, matches)
	c.Equal(documentDescendants-1, considered, "the last object of the document is never reached")
}

// countingCandidates returns a walk over the same candidates that counts how many of them it reached, which is how the
// test above measures how far a search got.
func countingCandidates(walk candidates, considered *int) candidates {
	return func(fn func(n *accessibility.Node) bool) {
		walk(func(n *accessibility.Node) bool {
			*considered++
			return fn(n)
		})
	}
}

func TestCollectionActiveDescendantIsNothing(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	ta.c.Equal(nullReference(), ta.one(NodePath(101), InterfaceCollection, "GetActiveDescendant", ""))
}

func TestMatchRuleDecoding(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	rule, failure := decodeMatchRule((&testRule{
		states:          []StateBit{StateFocused, StateReadOnly},
		statesMatch:     MatchAll,
		attributes:      dbus.Dict{{Key: levelAttribute, Value: `1:2:a\:b:c\\d`}},
		attributesMatch: MatchAny,
		roles:           []Role{RoleHeading, RoleParagraph},
		rolesMatch:      MatchAny,
		interfaces:      []string{"Text"},
		interfacesMatch: MatchAll,
		invert:          true,
	}).encode())
	c.Nil(failure)
	c.True(rule.states.Has(StateFocused))
	c.True(rule.states.Has(StateReadOnly), "a state in the second word arrives in the second word")
	c.Equal(2, rule.statesNamed)
	c.Equal(2, rule.rolesNamed)
	c.True(roleNamedIn(rule.roles, RoleHeading))
	c.True(roleNamedIn(rule.roles, RoleParagraph), "a role beyond the first word is in the word it belongs to")
	c.False(roleNamedIn(rule.roles, RoleLink))
	c.False(roleNamedIn(rule.roles, Role(9999)), "a role beyond the words the client sent is not in them")
	c.Equal([]attributeCriterion{{key: levelAttribute, values: []string{"1", "2", "a:b", `c\d`}}}, rule.attributes,
		"an escaped colon belongs to the value it is in, and so does an escaped backslash")
	c.Equal([]string{"Text"}, rule.interfaces)
	c.Equal(MatchAll, rule.statesMatch)
	c.Equal(MatchAny, rule.attributesMatch)
	c.Equal(MatchAny, rule.rolesMatch)
	c.Equal(MatchAll, rule.interfacesMatch)
	c.True(rule.invert)
	// A trailing backslash has nothing to escape, so it stands for itself.
	c.Equal([]string{`a\`}, splitAttributeValues(`a\`))
	c.Equal([]string{""}, splitAttributeValues(""), "an attribute that must be there and be empty")
}

func TestMatchRuleDecodingRefusesMalformedRules(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	sound := (&testRule{roles: []Role{RoleHeading}, rolesMatch: MatchAny}).encode()
	for _, one := range []struct {
		value any
		name  string
	}{
		{name: "something that is not a structure at all", value: int32(3)},
		{name: "a structure of the wrong size", value: dbus.Struct{int32(0)}},
		{name: "states that are not an array of integers", value: replacedField(sound, 0, "all of them")},
		{name: "a comparison that is not an integer", value: replacedField(sound, 1, uint32(1))},
		{name: "a comparison AT-SPI has never defined", value: replacedField(sound, 1, int32(9))},
		{name: "a negative comparison", value: replacedField(sound, 3, int32(-1))},
		{name: "attributes that are not a dictionary", value: replacedField(sound, 2, "level=1")},
		{name: "an attribute value that is not a string", value: replacedField(sound, 2, dbus.Dict{{
			Key:   levelAttribute,
			Value: int32(1),
		}})},
		{name: "an attribute name that is not a string", value: replacedField(sound, 2, dbus.Dict{{
			Key:   int32(1),
			Value: "1",
		}})},
		{name: "roles that are not an array of integers", value: replacedField(sound, 4, []uint32{1})},
		{name: "interfaces that are not an array of strings", value: replacedField(sound, 6, []int32{1})},
		{name: "an invert flag that is not a boolean", value: replacedField(sound, 8, int32(1))},
	} {
		rule, failure := decodeMatchRule(one.value)
		c.Nil(rule, one.name)
		c.NotNil(failure, one.name)
		if failure != nil {
			c.Equal(dbus.InvalidArgs, failure.Name, one.name)
		}
	}
	rule, failure := decodeMatchRule(sound)
	c.Nil(failure)
	c.NotNil(rule)
}

// TestCollectionRefusesARuleItCannotRead covers the same refusal over the bus: the one malformed rule the connection
// cannot catch from the signature alone is a comparison outside the four AT-SPI defines, since it is an integer like
// any other.
func TestCollectionRefusesARuleItCannotRead(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	broken := replacedField(rolesRule(RoleHeading).encode(), 5, int32(42))
	c.Equal(dbus.InvalidArgs, ta.errorName(NodePath(101), InterfaceCollection, "GetMatches",
		matchRuleSignature+"uib", broken, uint32(SortCanonical), int32(0), true))
	c.Equal(dbus.InvalidArgs, ta.errorName(NodePath(101), InterfaceCollection, "GetMatchesFrom",
		objectRefSignature+matchRuleSignature+"uuib", nodeRef(108), broken, uint32(SortCanonical),
		uint32(TreeInorder), int32(0), true))
	c.Equal(dbus.InvalidArgs, ta.errorName(NodePath(101), InterfaceCollection, "GetMatchesTo",
		objectRefSignature+matchRuleSignature+"uubib", nodeRef(108), broken, uint32(SortCanonical),
		uint32(TreeInorder), false, int32(0), true))
}

// replacedField returns a match rule structure with one of its fields swapped for something else, which is how the
// tests build a rule that is wrong in exactly one way.
func replacedField(rule dbus.Struct, field int, value any) dbus.Struct {
	broken := slices.Clone(rule)
	broken[field] = value
	return broken
}

func TestCollectionIntrospection(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	xml, ok := ta.one(NodePath(101), "org.freedesktop.DBus.Introspectable", "Introspect", "").(string)
	c.True(ok)
	for _, want := range []string{
		InterfaceCollection,
		`<method name="GetMatches">`,
		`<arg type="` + string(matchRuleSignature) + `" direction="in"/>`,
		`<method name="GetMatchesTo">`,
		`<method name="GetMatchesFrom">`,
		`<method name="GetActiveDescendant">`,
	} {
		c.True(strings.Contains(xml, want), "the introspection of a document must mention %s", want)
	}
	advertised, ok := ta.one(NodePath(101), InterfaceAccessible, "GetInterfaces", "").([]string)
	c.True(ok)
	c.Equal(advertised, atspiInterfacesIn(xml))
}
