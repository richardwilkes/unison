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
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// textWindow is the window the text tests publish.
const textWindow WindowKey = 3

// The content of the measured text area: two lines, the first of which ends with a line feed.
const (
	textBody       = firstTextLine + secondTextLine
	firstTextLine  = "ab\n"
	secondTextLine = "cd"
)

// The members of org.a11y.atspi.Text whose second argument is an AtspiTextBoundaryType rather than a granularity.
const (
	textAtOffset     = "GetTextAtOffset"
	textBeforeOffset = "GetTextBeforeOffset"
	textAfterOffset  = "GetTextAfterOffset"
)

// textTree is the window the text tests work over:
//
//	40 window "Notes"            (0,0 200x100)    active
//	├─ 41 text area "Body"       (10,10 100x40)   focused, "ab\ncd" over two measured lines, "b\nc" selected
//	├─ 42 text field "Title"     (10,60 100x20)   "Hi", unmeasured, caret at the end
//	└─ 43 password field         (10,80 100x20)   protected, and holding text it must not hand over
//
// The two lines of the text area are each twenty units tall, and every character on them is ten wide except the line
// feed, which has no width of its own.
func textTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 40, Role: role.Window, Name: "Notes", Focused: true, Bounds: geom.NewRect(0, 0, 200, 100),
			Children: []accessibility.NodeID{41, 42, 43},
		},
		&accessibility.Node{
			ID: 41, Parent: 40, Role: role.TextArea, Name: "Body", Focusable: true, Focused: true,
			Bounds:  geom.NewRect(10, 10, 100, 40),
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection),
			Text: &accessibility.TextInfo{
				Text:      textBody,
				Multiline: true,
				SelStart:  1,
				SelEnd:    4,
				Lines: []accessibility.Line{
					{
						Start:    0,
						End:      3,
						Bounds:   geom.NewRect(0, 0, 20, 20),
						Advances: []float32{0, 10, 20, 20},
					},
					{
						Start:    3,
						End:      5,
						Bounds:   geom.NewRect(0, 20, 20, 20),
						Advances: []float32{0, 10, 20},
					},
				},
			},
		},
		&accessibility.Node{
			ID: 42, Parent: 40, Role: role.TextField, Name: "Title", Focusable: true,
			Bounds:  geom.NewRect(10, 60, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection),
			Text:    &accessibility.TextInfo{Text: "Hi", SelStart: 2, SelEnd: 2},
		},
		&accessibility.Node{
			ID: 43, Parent: 40, Role: role.TextField, Name: "Secret", Protected: true, Focusable: true,
			Bounds: geom.NewRect(10, 80, 100, 20),
			Text:   &accessibility.TextInfo{Text: "secret", SelStart: 6, SelEnd: 6},
		},
	)
}

// newTextAdapter starts an adapter and publishes the text window, leaving the signals both publishes sent unread, since
// none of the text tests are about them.
func newTextAdapter(t *testing.T) *testAdapter {
	t.Helper()
	ta := newTestAdapter(t)
	ta.Publish(textWindow, textTree(), nil, sampleGeometry())
	return ta
}

func TestTextInterfaceIsOnlyThereForText(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	// Neither taking the focus nor setting a selection is one of the things AT-SPI's Action interface covers, so a text
	// area that does both still has nothing to do.
	c.Equal([]string{InterfaceAccessible, InterfaceComponent, InterfaceText},
		ta.one(NodePath(41), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceComponent, InterfaceText},
		ta.one(NodePath(43), InterfaceAccessible, "GetInterfaces", ""),
		"a password field that holds text still has the interface")
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(40), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"a window holds no text of its own")
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(3), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"neither does a label")
}

func TestTextContent(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	c.Equal(int32(5), ta.peer.getProperty(NodePath(41), InterfaceText, "CharacterCount"))
	c.Equal(int32(4), ta.peer.getProperty(NodePath(41), InterfaceText, "CaretOffset"),
		"the caret is where the selection ends")
	c.Equal(textBody, ta.one(NodePath(41), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"a negative end offset is the end of the text")
	c.Equal("b\n", ta.one(NodePath(41), InterfaceText, "GetText", "ii", int32(1), int32(3)))
	c.Equal(textBody, ta.one(NodePath(41), InterfaceText, "GetText", "ii", int32(-5), int32(99)),
		"offsets outside the text are brought back into it")
	c.Equal("", ta.one(NodePath(41), InterfaceText, "GetText", "ii", int32(3), int32(1)),
		"a range that ends before it starts holds nothing")
	c.Equal(int32('b'), ta.one(NodePath(41), InterfaceText, "GetCharacterAtOffset", "i", int32(1)))
	c.Equal(int32('\n'), ta.one(NodePath(41), InterfaceText, "GetCharacterAtOffset", "i", int32(2)))
	c.Equal(int32(0), ta.one(NodePath(41), InterfaceText, "GetCharacterAtOffset", "i", int32(5)),
		"there is no character at the end of the text")
}

// TestTextGranularityAtOffset covers GetStringAtOffset, whose second argument is an AtspiTextGranularity. Orca's
// character echo and word echo ask for CHAR and WORD on every arrow key, so answering all of them with the line — which
// is what this used to do — reads a whole line out on every keystroke.
func TestTextGranularityAtOffset(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	for _, one := range []struct {
		why         string
		text        string
		granularity Granularity
		offset      int32
		start       int32
		end         int32
	}{
		{why: "the first character", granularity: GranularityChar, offset: 0, text: "a", start: 0, end: 1},
		{why: "a line feed is a character too", granularity: GranularityChar, offset: 2, text: "\n", start: 2, end: 3},
		{
			why: "there is no character at the end of the text", granularity: GranularityChar, offset: 5,
			text: "", start: 5, end: 5,
		},
		{
			why: "a word takes the whitespace that follows it", granularity: GranularityWord, offset: 1,
			text: firstTextLine, start: 0, end: 3,
		},
		{why: "the second word", granularity: GranularityWord, offset: 4, text: secondTextLine, start: 3, end: 5},
		{
			why: "a line takes the line feed that ends it", granularity: GranularityLine, offset: 0,
			text: firstTextLine, start: 0, end: 3,
		},
		{why: "the second line", granularity: GranularityLine, offset: 4, text: secondTextLine, start: 3, end: 5},
		{
			why: "the end of the text belongs to the last line", granularity: GranularityLine, offset: 5,
			text: secondTextLine, start: 3, end: 5,
		},
		{
			why: "a paragraph is what the text is divided into", granularity: GranularityParagraph, offset: 1,
			text: firstTextLine, start: 0, end: 3,
		},
		{
			why: "content with nothing to end a sentence is one sentence", granularity: GranularitySentence,
			offset: 1, text: textBody, start: 0, end: 5,
		},
		{
			why: "a granularity AT-SPI has never defined is read as a line", granularity: Granularity(99),
			offset: 4, text: secondTextLine, start: 3, end: 5,
		},
	} {
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(41), InterfaceText, "GetStringAtOffset", "iu", one.offset, uint32(one.granularity)),
			one.why)
	}

	// A field that has not been measured, which is every text control but the focused one, reports its whole content as
	// one line.
	c.Equal([]any{"Hi", int32(0), int32(2)},
		ta.values(NodePath(42), InterfaceText, "GetStringAtOffset", "iu", int32(1), uint32(GranularityLine)))
	c.Equal([]any{"i", int32(1), int32(2)},
		ta.values(NodePath(42), InterfaceText, "GetStringAtOffset", "iu", int32(1), uint32(GranularityChar)))
}

// TestTextBoundaryAtBeforeAndAfterOffset covers the three older methods, whose second argument is an
// AtspiTextBoundaryType rather than a granularity — the same units under different numbers — and whose before and after
// forms have to answer with the neighboring unit. Handing back the unit at the offset instead leaves a client walking
// the content on the spot, never reaching the end.
func TestTextBoundaryAtBeforeAndAfterOffset(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	for _, one := range []struct {
		member   string
		why      string
		text     string
		boundary Boundary
		offset   int32
		start    int32
		end      int32
	}{
		{
			member: textAtOffset, why: "the line holding the offset", boundary: BoundaryLineStart,
			offset: 4, text: secondTextLine, start: 3, end: 5,
		},
		{
			member: textAtOffset, why: "boundary 3 is a sentence rather than a line", boundary: BoundarySentenceStart,
			offset: 4, text: textBody, start: 0, end: 5,
		},
		{
			member: textAtOffset, why: "the character at the offset", boundary: BoundaryChar,
			offset: 1, text: "b", start: 1, end: 2,
		},
		{
			member: textBeforeOffset, why: "the line before the one holding the offset",
			boundary: BoundaryLineStart, offset: 4, text: firstTextLine, start: 0, end: 3,
		},
		{
			member: textBeforeOffset, why: "there is nothing before the first line",
			boundary: BoundaryLineStart, offset: 1, text: "", start: 0, end: 0,
		},
		{
			member: textBeforeOffset, why: "the character before the offset", boundary: BoundaryChar,
			offset: 1, text: "a", start: 0, end: 1,
		},
		{
			member: textBeforeOffset, why: "the word before the one holding the offset",
			boundary: BoundaryWordStart, offset: 4, text: firstTextLine, start: 0, end: 3,
		},
		{
			member: textAfterOffset, why: "the line after the one holding the offset",
			boundary: BoundaryLineStart, offset: 0, text: secondTextLine, start: 3, end: 5,
		},
		{
			member: textAfterOffset, why: "there is nothing after the last line", boundary: BoundaryLineStart,
			offset: 4, text: "", start: 5, end: 5,
		},
		{
			member: textAfterOffset, why: "the character after the offset", boundary: BoundaryChar,
			offset: 0, text: "b", start: 1, end: 2,
		},
		{
			member: textAfterOffset, why: "the end form divides the text the same way as the start form",
			boundary: BoundaryWordEnd, offset: 0, text: secondTextLine, start: 3, end: 5,
		},
	} {
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(41), InterfaceText, one.member, "iu", one.offset, uint32(one.boundary)),
			"%s: %s", one.member, one.why)
	}
}

// TestTextUnits covers the division of content into units directly, over content the window the other text tests work
// with does not hold: real words, sentences and paragraphs. Every unit runs to the start of the next one, so walking a
// control a unit at a time covers every character exactly once.
func TestTextUnits(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	content := textContent{runes: []rune("Hi there.  Bye now!\nNext line")}
	for _, one := range []struct {
		why    string
		unit   textUnit
		offset int
		start  int
		end    int
	}{
		{why: "the first word, with the space after it", unit: unitWord, offset: 0, start: 0, end: 3},
		{why: "an offset inside a word finds the whole of it", unit: unitWord, offset: 4, start: 3, end: 11},
		{why: "the two spaces belong to the word they follow", unit: unitWord, offset: 9, start: 3, end: 11},
		{why: "a line feed is whitespace like any other", unit: unitWord, offset: 16, start: 15, end: 20},
		{why: "the last word runs to the end", unit: unitWord, offset: 25, start: 25, end: 29},
		{why: "the first sentence, with the spaces after it", unit: unitSentence, offset: 0, start: 0, end: 11},
		{why: "the second sentence, ending at the line feed after it", unit: unitSentence, offset: 12, start: 11, end: 20},
		{why: "the last sentence runs to the end", unit: unitSentence, offset: 22, start: 20, end: 29},
		{why: "the first paragraph, with the line feed", unit: unitParagraph, offset: 3, start: 0, end: 20},
		{why: "the second paragraph", unit: unitParagraph, offset: 20, start: 20, end: 29},
		{why: "a control with no measured lines is one line", unit: unitLine, offset: 3, start: 0, end: 29},
	} {
		c.Equal(textRange{start: one.start, end: one.end}, content.rangeAt(one.unit, one.offset), one.why)
	}

	// A full stop with a character hard against it is part of the word rather than the end of a sentence, which is what
	// keeps a version number or a file name from being read as several of them.
	run := textContent{runes: []rune("Use v1.2.3 now. Then stop.")}
	c.Equal(textRange{start: 0, end: 16}, run.rangeAt(unitSentence, 0))
	c.Equal(textRange{start: 16, end: 26}, run.rangeAt(unitSentence, 20))

	// Walking forwards has to carry on from where the unit before it ended and reach the end of the content, which is
	// what makes a client reading the whole of a control with GetTextAfterOffset terminate.
	for _, unit := range []textUnit{unitChar, unitWord, unitSentence, unitParagraph} {
		at := content.rangeAt(unit, 0)
		c.Equal(0, at.start, "unit %d must begin at the beginning", unit)
		for range len(content.runes) {
			if at.end >= len(content.runes) {
				break
			}
			next := content.rangeAfter(unit, at.start)
			c.Equal(at.end, next.start, "unit %d must carry on from where the one before it ended", unit)
			c.True(next.end > at.end, "unit %d must advance past %d", unit, at.end)
			at = next
		}
		c.Equal(len(content.runes), at.end, "unit %d must reach the end", unit)
	}
	empty := textContent{}
	c.Equal(textRange{}, empty.rangeAt(unitWord, 0), "there is nothing in an empty control")
	c.Equal(textRange{}, empty.rangeBefore(unitLine, 0))
	c.Equal(textRange{}, empty.rangeAfter(unitLine, 0))
}

func TestTextExtents(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	// The window sits at 100,50 on the screen and everything in it is twice the size in physical pixels, so the first
	// character of the first line, ten units wide and twenty tall at the text area's own top left corner, is forty
	// pixels tall at 120,70.
	c.Equal([]any{int32(120), int32(70), int32(20), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetCharacterExtents", "iu", int32(0), uint32(CoordScreen)))
	c.Equal([]any{int32(20), int32(20), int32(20), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetCharacterExtents", "iu", int32(0), uint32(CoordWindow)))
	c.Equal([]any{int32(40), int32(20), int32(20), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetCharacterExtents", "iu", int32(1), uint32(CoordWindow)))
	c.Equal([]any{int32(60), int32(20), int32(0), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetCharacterExtents", "iu", int32(2), uint32(CoordWindow)),
		"a line feed takes up no room")
	c.Equal([]any{int32(20), int32(60), int32(20), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetCharacterExtents", "iu", int32(3), uint32(CoordWindow)),
		"the second line is below the first")
	c.Equal([]any{int32(0), int32(0), int32(0), int32(0)},
		ta.values(NodePath(41), InterfaceText, "GetCharacterExtents", "iu", int32(9), uint32(CoordWindow)),
		"there is nothing at an offset the text does not reach")

	// The whole content is two lines of two visible characters each.
	c.Equal([]any{int32(20), int32(20), int32(40), int32(80)},
		ta.values(NodePath(41), InterfaceText, "GetRangeExtents", "iiu", int32(0), int32(-1), uint32(CoordWindow)))
	c.Equal([]any{int32(20), int32(20), int32(40), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetRangeExtents", "iiu", int32(0), int32(2), uint32(CoordWindow)))
	c.Equal([]any{int32(0), int32(0), int32(0), int32(0)},
		ta.values(NodePath(41), InterfaceText, "GetRangeExtents", "iiu", int32(2), int32(3), uint32(CoordWindow)),
		"a range that holds nothing but a line feed has no area")

	// An unmeasured field has one line the size of the field, and its characters are not told apart.
	c.Equal([]any{int32(20), int32(120), int32(200), int32(40)},
		ta.values(NodePath(42), InterfaceText, "GetCharacterExtents", "iu", int32(0), uint32(CoordWindow)))
}

func TestTextOffsetAtPoint(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	for _, one := range []struct {
		why      string
		x, y     int32
		coord    CoordType
		expected int32
	}{
		{why: "the first character", x: 25, y: 25, coord: CoordWindow, expected: 0},
		{why: "the second character", x: 55, y: 25, coord: CoordWindow, expected: 1},
		{why: "the start of the second line", x: 25, y: 65, coord: CoordWindow, expected: 3},
		{why: "past the end of a line is that line's last character", x: 195, y: 65, coord: CoordWindow, expected: 4},
		{why: "the same point on the screen", x: 125, y: 75, coord: CoordScreen, expected: 0},
		{why: "below the whole control", x: 25, y: 400, coord: CoordWindow, expected: -1},
		{why: "left of the whole control", x: 0, y: 25, coord: CoordWindow, expected: -1},
	} {
		c.Equal(one.expected, ta.one(NodePath(41), InterfaceText, "GetOffsetAtPoint", "iiu", one.x, one.y,
			uint32(one.coord)), one.why)
	}
	c.Equal(int32(0), ta.one(NodePath(42), InterfaceText, "GetOffsetAtPoint", "iiu", int32(30), int32(130),
		uint32(CoordWindow)), "an unmeasured field answers with its first character wherever it is asked about")
}

func TestTextSelection(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	c.Equal(int32(1), ta.one(NodePath(41), InterfaceText, "GetNSelections", ""))
	c.Equal([]any{int32(1), int32(4)}, ta.values(NodePath(41), InterfaceText, "GetSelection", "i", int32(0)))
	c.Equal([]any{int32(0), int32(0)}, ta.values(NodePath(41), InterfaceText, "GetSelection", "i", int32(1)),
		"there is never more than one selection")
	c.Equal(int32(0), ta.one(NodePath(42), InterfaceText, "GetNSelections", ""),
		"a caret with nothing selected is no selection at all")
	c.Equal([]any{int32(0), int32(0)}, ta.values(NodePath(42), InterfaceText, "GetSelection", "i", int32(0)))

	// Moving the caret, which the widget is asked to do rather than told about.
	c.Equal(true, ta.one(NodePath(41), InterfaceText, "SetCaretOffset", "i", int32(2)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.SetTextSelection, Start: 2, End: 2,
	}, ta.nextRequest(t))
	c.Equal(int32(4), ta.peer.getProperty(NodePath(41), InterfaceText, "CaretOffset"),
		"the caret only moves when the next snapshot says it has")
	for _, offset := range []int32{-1, 6} {
		c.Equal(false, ta.one(NodePath(41), InterfaceText, "SetCaretOffset", "i", offset),
			"offset %d is not in the text", offset)
	}
	c.Equal(false, ta.one(NodePath(43), InterfaceText, "SetCaretOffset", "i", int32(1)),
		"a control that does not let its selection be set says so")
	ta.noRequest(t)

	// Replacing and removing the selection.
	c.Equal(true, ta.one(NodePath(41), InterfaceText, "SetSelection", "iii", int32(0), int32(0), int32(2)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.SetTextSelection, Start: 0, End: 2,
	}, ta.nextRequest(t))
	c.Equal(false, ta.one(NodePath(41), InterfaceText, "SetSelection", "iii", int32(1), int32(0), int32(2)))
	c.Equal(true, ta.one(NodePath(41), InterfaceText, "RemoveSelection", "i", int32(0)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.SetTextSelection, Start: 4, End: 4,
	}, ta.nextRequest(t), "removing the selection leaves the caret where it ended")
	c.Equal(false, ta.one(NodePath(41), InterfaceText, "RemoveSelection", "i", int32(1)))
	c.Equal(false, ta.one(NodePath(42), InterfaceText, "RemoveSelection", "i", int32(0)),
		"there is no selection to remove")

	// Adding one, which only works while there is none.
	c.Equal(false, ta.one(NodePath(41), InterfaceText, "AddSelection", "ii", int32(0), int32(2)))
	c.Equal(true, ta.one(NodePath(42), InterfaceText, "AddSelection", "ii", int32(0), int32(2)))
	c.Equal(accessibility.ActionRequest{
		Node: 42, Action: accessibility.SetTextSelection, Start: 0, End: 2,
	}, ta.nextRequest(t))
	ta.noRequest(t)
}

func TestTextOfAProtectedField(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	// A password never reaches an assistive technology, but how much of one has been typed does.
	c.Equal(int32(6), ta.peer.getProperty(NodePath(43), InterfaceText, "CharacterCount"))
	c.Equal("••••••", ta.one(NodePath(43), InterfaceText, "GetText", "ii", int32(0), int32(-1)))
	c.Equal([]any{"••••••", int32(0), int32(6)},
		ta.values(NodePath(43), InterfaceText, "GetStringAtOffset", "iu", int32(0), uint32(GranularityLine)))
	c.Equal([]any{"•", int32(2), int32(3)},
		ta.values(NodePath(43), InterfaceText, "GetStringAtOffset", "iu", int32(2), uint32(GranularityChar)),
		"a bullet is what every unit of a password is made of, however small the unit")
	c.Equal(int32(bulletRune), ta.one(NodePath(43), InterfaceText, "GetCharacterAtOffset", "i", int32(0)))
}

func TestTextAttributes(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	// Unison reports no text attributes in V1, so the whole content is one run that has none.
	c.Equal([]any{dbus.Dict{}, int32(0), int32(5)},
		ta.values(NodePath(41), InterfaceText, "GetAttributes", "i", int32(2)))
	c.Equal([]any{dbus.Dict{}, int32(0), int32(5)},
		ta.values(NodePath(41), InterfaceText, "GetAttributeRun", "ib", int32(2), true))
	c.Equal(dbus.Dict{}, ta.one(NodePath(41), InterfaceText, "GetDefaultAttributes", ""))
	c.Equal("", ta.one(NodePath(41), InterfaceText, "GetAttributeValue", "is", int32(2), "weight"))
}

func TestTextIntrospection(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	xml, ok := ta.one(NodePath(41), "org.freedesktop.DBus.Introspectable", "Introspect", "").(string)
	c.True(ok)
	for _, want := range []string{
		InterfaceText,
		`<property name="CharacterCount" type="i" access="read"/>`,
		`<property name="CaretOffset" type="i" access="read"/>`,
		`<method name="GetStringAtOffset">`,
		`<method name="GetCharacterExtents">`,
		`<method name="GetAttributeRun">`,
	} {
		c.True(strings.Contains(xml, want), "the introspection of a text node must mention %s", want)
	}
}
