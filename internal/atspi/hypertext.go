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

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// hypertextInterface returns the org.a11y.atspi.Hypertext interface of a node whose text holds other objects, which is
// what a paragraph with a link or an image in it does. It is the second half of how AT-SPI reports inline content: the
// text says where the objects are and this says which they are, so an assistive technology reading a paragraph a line
// at a time can announce "link" at the right word and follow it.
//
// The objects are the node's own text spans; see [accessibility.TextInfo.Spans]. An image occupies the one U+FFFC that
// stands in for it in the text, which is the object replacement character every AT-SPI producer uses for an embedded
// object, so a link and an image are reported the same way and told apart by their roles.
func (o *nodeObject) hypertextInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceHypertext,
		Methods: []*dbus.Method{
			{Name: "GetNLinks", Out: "i", Handle: o.getNLinks},
			{Name: "GetLink", In: "i", Out: objectRefSignature, Handle: o.getLink},
			{Name: "GetLinkIndex", In: "i", Out: "i", Handle: o.getLinkIndex},
		},
	}
}

// hyperlinkInterface returns the org.a11y.atspi.Hyperlink interface of a node that leads somewhere or that occupies
// part of another node's text. The object itself implements it rather than a separate one standing for the link, which
// is what libatspi expects: atspi_accessible_get_hyperlink hands back a hyperlink at the same path and then reads these
// properties off it.
//
// There is one anchor. AT-SPI allows several, for the image maps of a web page, where one link leads to different
// places depending on where it was clicked; a Unison link is one panel that leads to one place.
//
// StartIndex and EndIndex are where in the containing text the object sits, which is what an assistive technology
// matches against the offsets it is reading. A link that is in nobody's text — one standing on its own rather than
// within a paragraph — has no place in any text and reports -1 for both, which is what ATK reports for a link with no
// hypertext around it.
func (o *nodeObject) hyperlinkInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceHyperlink,
		Methods: []*dbus.Method{
			{Name: "GetObject", In: "i", Out: objectRefSignature, Handle: o.getHyperlinkObject},
			{Name: "GetURI", In: "i", Out: "s", Handle: o.getURI},
			{Name: "IsValid", Out: "b", Handle: o.isHyperlinkValid},
		},
		Properties: []*dbus.Property{
			{Name: "NAnchors", Sig: "n", Get: func() (any, error) { return int16(1), nil }},
			{Name: "StartIndex", Sig: "i", Get: func() (any, error) { return int32(o.spanRange().start), nil }},
			{Name: "EndIndex", Sig: "i", Get: func() (any, error) { return int32(o.spanRange().end), nil }},
		},
	}
}

// links returns the spans of this node's text that an assistive technology can be pointed at, ordered by where in the
// text they begin. A span whose object has no object of its own is left out, and so is one whose object another node's
// text claimed first, so that what this hands out and what that object reports as its own place agree; see
// [buildSpanIndex].
//
// The order is put right here rather than taken on trust. [accessibility.TextInfo.Spans] is ordered outermost first and
// only then by where it starts, while the index an AT-SPI link is named by is a position in the text: GetLink(i) walks
// the objects of a paragraph from its beginning and GetLinkIndex answers with the first span an offset falls in, so
// spans arriving in any other order would have a client told that the second link of a paragraph comes before the
// first, and an offset inside a nested object answered with the object around it. Sorting is stable, so two spans that
// begin together — which nothing a snapshot builder produces, since a block's spans are the flat inline objects of one
// line of prose — keep the order the text gave them.
func (o *nodeObject) links() []accessibility.TextSpan {
	info := o.node.Text
	if info == nil {
		return nil
	}
	links := make([]accessibility.TextSpan, 0, len(info.Spans))
	for _, span := range info.Spans {
		if ref, ok := o.data.spanOf(span.Node); ok && ref.container == o.node.ID {
			links = append(links, span)
		}
	}
	slices.SortStableFunc(links, func(a, b accessibility.TextSpan) int { return a.Start - b.Start })
	return links
}

// spanRange returns where in the containing node's text this node sits, or a range of -1 to -1 when no text holds it.
func (o *nodeObject) spanRange() spanRef {
	if span, ok := o.data.spanOf(o.node.ID); ok {
		return span
	}
	return spanRef{start: -1, end: -1}
}

// getNLinks implements org.a11y.atspi.Hypertext.GetNLinks.
func (o *nodeObject) getNLinks(call *dbus.Call) {
	call.Reply(int32(len(o.links())))
}

// getLink implements org.a11y.atspi.Hypertext.GetLink. An index the text does not hold is answered with the null
// reference rather than an error, as an out of range child is: the client may have counted the links from a snapshot
// that has since been replaced.
func (o *nodeObject) getLink(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	links := o.links()
	index := int(int32Arg(args, 0))
	if index < 0 || index >= len(links) {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.a.reference(links[index].Node))
}

// getLinkIndex implements org.a11y.atspi.Hypertext.GetLinkIndex, which asks which of the links a character offset falls
// inside. An offset that is not inside one has no link, which AT-SPI spells as -1.
func (o *nodeObject) getLinkIndex(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	offset := int(int32Arg(args, 0))
	for i, span := range o.links() {
		if offset >= span.Start && offset < span.End {
			call.Reply(int32(i))
			return
		}
	}
	call.Reply(int32(-1))
}

// getHyperlinkObject implements org.a11y.atspi.Hyperlink.GetObject, which asks for the object one anchor of the link
// leads to. There is one anchor and the object is the link itself, which is what every AT-SPI producer that does not
// serve image maps answers; any other index is the null reference.
func (o *nodeObject) getHyperlinkObject(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if int32Arg(args, 0) != 0 {
		call.Reply(nullReference())
		return
	}
	call.Reply(o.reference())
}

// getURI implements org.a11y.atspi.Hyperlink.GetURI, which is where the link leads, in the form the application
// supplied it in. An object that is part of a text without leading anywhere — an image within a paragraph — has none,
// which AT-SPI spells as an empty string, and so has any index but the one anchor.
func (o *nodeObject) getURI(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if int32Arg(args, 0) != 0 {
		call.Reply("")
		return
	}
	call.Reply(o.node.URL)
}

// isHyperlinkValid implements org.a11y.atspi.Hyperlink.IsValid, which asks whether the link still refers to anything.
// This object was resolved from a published snapshot that still holds the node, so it does; a node that has left the
// tree has no object at all to ask, and the call fails with an unknown object instead.
func (o *nodeObject) isHyperlinkValid(call *dbus.Call) {
	call.Reply(true)
}
