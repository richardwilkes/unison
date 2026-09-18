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

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// TestHypertextIsOnlyThereForTextHoldingObjects covers which objects get the two interfaces. Only the paragraph whose
// text holds something has a hypertext, and only the objects that lead somewhere or sit within someone's text are
// hyperlinks.
func TestHypertextIsOnlyThereForTextHoldingObjects(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal([]string{
		InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceHypertext, InterfaceText,
	}, ta.one(NodePath(103), InterfaceAccessible, "GetInterfaces", ""),
		"the paragraph holding a link and an image has a hypertext but is not a hyperlink itself")
	c.Equal([]string{
		InterfaceAccessible, InterfaceAction, InterfaceCollection, InterfaceComponent, InterfaceHyperlink,
	}, ta.one(NodePath(104), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceHyperlink},
		ta.one(NodePath(105), InterfaceAccessible, "GetInterfaces", ""),
		"an image within a text is a hyperlink even though it leads nowhere")
	// A block whose text holds nothing has no hypertext, however much text it holds.
	for _, id := range []accessibility.NodeID{102, 107, 108, 110} {
		interfaces, ok := ta.one(NodePath(id), InterfaceAccessible, "GetInterfaces", "").([]string)
		c.True(ok)
		c.False(slices.Contains(interfaces, InterfaceHypertext), "node %d holds nothing within its text", id)
		c.False(slices.Contains(interfaces, InterfaceHyperlink), "and is in nobody else's", id)
		c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(id), InterfaceHypertext, "GetNLinks", ""))
		c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(id), InterfaceHyperlink, "IsValid", ""))
	}
	// The document composes the text of every block into one stream with a span per block, but that stream is never
	// exposed, so the blocks are nobody's hyperlinks and the document itself is no hypertext.
	interfaces, ok := ta.one(NodePath(101), InterfaceAccessible, "GetInterfaces", "").([]string)
	c.True(ok)
	c.False(slices.Contains(interfaces, InterfaceHypertext))
	c.False(slices.Contains(interfaces, InterfaceHyperlink))
}

func TestHypertextLinksOfAParagraph(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal(int32(2), ta.one(NodePath(103), InterfaceHypertext, "GetNLinks", ""))
	c.Equal(nodeRef(104), ta.one(NodePath(103), InterfaceHypertext, "GetLink", "i", int32(0)))
	c.Equal(nodeRef(105), ta.one(NodePath(103), InterfaceHypertext, "GetLink", "i", int32(1)),
		"the objects are in the order they appear in the text")
	for _, index := range []int32{-1, 2, 99} {
		c.Equal(nullReference(), ta.one(NodePath(103), InterfaceHypertext, "GetLink", "i", index),
			"there is no link %d", index)
	}
	// Which object a character belongs to, which is what a client reading the paragraph asks as it goes.
	for _, one := range []struct {
		offset int32
		link   int32
	}{
		{offset: 0, link: -1},
		{offset: paragraphLinkStart - 1, link: -1},
		{offset: paragraphLinkStart, link: 0},
		{offset: paragraphLinkEnd - 1, link: 0},
		{offset: paragraphLinkEnd, link: -1},
		{offset: paragraphImageStart, link: 1},
		{offset: paragraphImageEnd, link: -1},
		{offset: documentParagraphLength - 1, link: -1},
		{offset: -1, link: -1},
		{offset: 99, link: -1},
	} {
		c.Equal(one.link, ta.one(NodePath(103), InterfaceHypertext, "GetLinkIndex", "i", one.offset),
			"the link at offset %d", one.offset)
	}
}

// TestHypertextLinksAreOrderedByPositionInTheText covers the order the objects of a text are handed out in. An AT-SPI
// link is named by its index, and a client walks a paragraph's objects from its beginning: GetLink(0) is the first
// object in the text and GetLinkIndex answers with the object an offset falls in. [accessibility.TextInfo.Spans]
// promises something else — outermost first, and only then by where it starts — so the order is put right rather than
// taken on trust.
func TestHypertextLinksAreOrderedByPositionInTheText(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	stored := documentTree()
	stored.Generation++
	slices.Reverse(stored.Node(103).Text.Spans)
	c.Equal(accessibility.NodeID(105), stored.Node(103).Text.Spans[0].Node,
		"the image is now the first span the paragraph stores, though it comes second in its text")
	ta.Publish(documentWindow, stored, nil, sampleGeometry())
	c.Equal(nodeRef(104), ta.one(NodePath(103), InterfaceHypertext, "GetLink", "i", int32(0)),
		"the link is still the first object of the text")
	c.Equal(nodeRef(105), ta.one(NodePath(103), InterfaceHypertext, "GetLink", "i", int32(1)))
	c.Equal(int32(0), ta.one(NodePath(103), InterfaceHypertext, "GetLinkIndex", "i", int32(paragraphLinkStart)))
	c.Equal(int32(1), ta.one(NodePath(103), InterfaceHypertext, "GetLinkIndex", "i", int32(paragraphImageStart)))

	// Sorting is only half of what makes an index into the spans an index into the text: nothing a snapshot builder
	// produces nests one span inside another, since a block's spans are the flat inline objects of its own prose. A
	// nested span would have GetLinkIndex answer an offset inside the inner object with the one around it, whichever
	// order the two arrived in.
	documentTree().Walk(func(n *accessibility.Node) bool {
		if n.Text == nil {
			return true
		}
		ordered := slices.Clone(n.Text.Spans)
		slices.SortStableFunc(ordered, func(a, b accessibility.TextSpan) int { return a.Start - b.Start })
		for i := 1; i < len(ordered); i++ {
			c.True(ordered[i].Start >= ordered[i-1].End,
				"the spans of node %d neither overlap nor nest, but %d..%d holds %d..%d", n.ID,
				ordered[i-1].Start, ordered[i-1].End, ordered[i].Start, ordered[i].End)
		}
		return true
	})
}

// TestHyperlinkOfAnObjectWithinAText covers what an assistive technology asks a link once the paragraph's hypertext has
// pointed it there: where in the text it sits, what it leads to, and whether it is still worth following.
func TestHyperlinkOfAnObjectWithinAText(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal(int16(1), ta.peer.getProperty(NodePath(104), InterfaceHyperlink, "NAnchors"),
		"a Unison link leads to one place")
	c.Equal(int32(paragraphLinkStart), ta.peer.getProperty(NodePath(104), InterfaceHyperlink, "StartIndex"))
	c.Equal(int32(paragraphLinkEnd), ta.peer.getProperty(NodePath(104), InterfaceHyperlink, "EndIndex"))
	c.Equal(nodeRef(104), ta.one(NodePath(104), InterfaceHyperlink, "GetObject", "i", int32(0)),
		"the object of a link is the link itself")
	c.Equal(nullReference(), ta.one(NodePath(104), InterfaceHyperlink, "GetObject", "i", int32(1)))
	c.Equal(documentGuideURL, ta.one(NodePath(104), InterfaceHyperlink, "GetURI", "i", int32(0)))
	c.Equal("", ta.one(NodePath(104), InterfaceHyperlink, "GetURI", "i", int32(1)),
		"there is no second anchor to ask about")
	c.Equal(true, ta.one(NodePath(104), InterfaceHyperlink, "IsValid", ""))

	// The image occupies the one U+FFFC that stands in for it and leads nowhere.
	c.Equal(int32(paragraphImageStart), ta.peer.getProperty(NodePath(105), InterfaceHyperlink, "StartIndex"))
	c.Equal(int32(paragraphImageEnd), ta.peer.getProperty(NodePath(105), InterfaceHyperlink, "EndIndex"))
	c.Equal("", ta.one(NodePath(105), InterfaceHyperlink, "GetURI", "i", int32(0)))
	c.Equal(nodeRef(105), ta.one(NodePath(105), InterfaceHyperlink, "GetObject", "i", int32(0)))
}

// TestHyperlinkOfALinkOfItsOwn covers the link that is not part of anyone's text: it still says where it leads, since
// that is the other half of what a hyperlink is for, and reports that it sits nowhere.
func TestHyperlinkOfALinkOfItsOwn(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal(documentHomeURL, ta.one(NodePath(125), InterfaceHyperlink, "GetURI", "i", int32(0)))
	c.Equal(int32(-1), ta.peer.getProperty(NodePath(125), InterfaceHyperlink, "StartIndex"))
	c.Equal(int32(-1), ta.peer.getProperty(NodePath(125), InterfaceHyperlink, "EndIndex"))
	c.Equal(int16(1), ta.peer.getProperty(NodePath(125), InterfaceHyperlink, "NAnchors"))
	c.Equal(true, ta.one(NodePath(125), InterfaceHyperlink, "IsValid", ""))
}

// TestHypertextSpansAreOnlyReadFromABlocksOwnText covers the one text a span index is built from. A span of the
// document's composed stream names a block, and if those spans counted the blocks would report themselves as links
// inside a document that hands over no text at all for the offsets to mean anything in.
func TestHypertextSpansAreOnlyReadFromABlocksOwnText(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	tree := documentTree()
	c.True(len(tree.Node(101).Document.Text.Spans) > 1, "the document's stream does hold spans over its blocks")
	data := newWindowData(tree, sampleGeometry())
	for _, block := range documentBlocks {
		c.False(data.isSpanTarget(block.node), "node %d is a block of the document rather than part of a text",
			block.node)
	}
	for _, id := range []accessibility.NodeID{104, 105} {
		span, ok := data.spanOf(id)
		c.True(ok, "node %d sits within the paragraph's text", id)
		c.Equal(accessibility.NodeID(103), span.container)
	}
}

func TestHypertextIntrospection(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	xml, ok := ta.one(NodePath(104), "org.freedesktop.DBus.Introspectable", "Introspect", "").(string)
	c.True(ok)
	for _, want := range []string{
		InterfaceHyperlink,
		`<property name="NAnchors" type="n" access="read"/>`,
		`<property name="StartIndex" type="i" access="read"/>`,
		`<property name="EndIndex" type="i" access="read"/>`,
		`<method name="GetURI">`,
		`<method name="IsValid">`,
	} {
		c.True(strings.Contains(xml, want), "the introspection of a link must mention %s", want)
	}
	advertised, ok := ta.one(NodePath(104), InterfaceAccessible, "GetInterfaces", "").([]string)
	c.True(ok)
	c.Equal(advertised, atspiInterfacesIn(xml))
}
