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
	"strconv"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// textWindow is the window the text tests publish, and labelWindow the one the static text tests publish.
const (
	textWindow  WindowKey = 3
	labelWindow WindowKey = 9
)

// The static text of [labelTree]: what the label says and what the column header says.
const (
	labelTextBody  = "Name:"
	headerTextBody = "Size"
)

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
//	40 window "Notes"            (0,0 200x140)    active
//	├─ 41 text area "Body"       (10,10 100x40)   focused, "ab\ncd" over two measured lines, "b\nc" selected
//	├─ 42 text field "Title"     (10,60 100x20)   "Hi", unmeasured, caret at the end
//	├─ 43 password field         (10,80 100x20)   protected, so it carries no text at all
//	└─ 44 text area "Log"        (10,110 100x20)  text that can be neither selected nor changed
//
// The two lines of the text area are each twenty units tall, and every character on them is ten wide except the line
// feed, which has no width of its own.
func textTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 40, Role: role.Window, Name: "Notes", Focused: true, Bounds: geom.NewRect(0, 0, 200, 140),
			Children: []accessibility.NodeID{41, 42, 43, 44},
		},
		&accessibility.Node{
			ID: 41, Parent: 40, Role: role.TextArea, Name: "Body", Focusable: true, Focused: true,
			Bounds: geom.NewRect(10, 10, 100, 40),
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection,
				accessibility.ReplaceText),
			Text: &accessibility.TextInfo{
				Text:      textBody,
				Multiline: true,
				SelStart:  1,
				SelEnd:    4,
				Caret:     4,
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
			Bounds: geom.NewRect(10, 60, 100, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Focus, accessibility.SetTextSelection,
				accessibility.ReplaceText, accessibility.SetValue),
			Text: &accessibility.TextInfo{Text: "Hi", SelStart: 2, SelEnd: 2, Caret: 2},
		},
		&accessibility.Node{
			ID: 43, Parent: 40, Role: role.TextField, Name: "Secret", Protected: true, Focusable: true,
			Bounds: geom.NewRect(10, 80, 100, 20),
		},
		&accessibility.Node{
			ID: 44, Parent: 40, Role: role.TextArea, Name: "Log", ReadOnly: true,
			Bounds: geom.NewRect(10, 110, 100, 20),
			Text:   &accessibility.TextInfo{Text: "done", Multiline: true},
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

// labelTree is the window the static text tests work over, holding the two things that carry text without ever being
// typed into:
//
//	50 window "Form"            (0,0 200x80)   active
//	├─ 51 label "Name:"         (10,10 60x20)  one measured line, and nothing to do but come into view
//	└─ 52 column header "Size"  (10,40 80x20)  text that was never measured
//
// The label's line is twenty units tall and each of its characters ten wide. The column header carries the text it
// draws without the line it drew it on, which is what a header that is not the focus of the window publishes. It is
// also given no actions at all, which no published node is — axSnapshot hands every one of them ScrollIntoView — so
// that the methods that refuse a node with nothing to ask of it have something to refuse.
func labelTree() *accessibility.Tree {
	return treeOf(1,
		&accessibility.Node{
			ID: 50, Role: role.Window, Name: "Form", Focused: true, Bounds: geom.NewRect(0, 0, 200, 80),
			Children: []accessibility.NodeID{51, 52},
		},
		&accessibility.Node{
			ID: 51, Parent: 50, Role: role.Label, Name: labelTextBody, Bounds: geom.NewRect(10, 10, 60, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.ScrollIntoView),
			Text: &accessibility.TextInfo{
				Text: labelTextBody,
				Lines: []accessibility.Line{
					{
						Start:    0,
						End:      len(labelTextBody),
						Bounds:   geom.NewRect(0, 0, 50, 20),
						Advances: []float32{0, 10, 20, 30, 40, 50},
					},
				},
			},
		},
		&accessibility.Node{
			ID: 52, Parent: 50, Role: role.ColumnHeader, Name: headerTextBody, Bounds: geom.NewRect(10, 40, 80, 20),
			Text: &accessibility.TextInfo{Text: headerTextBody},
		},
	)
}

// newLabelAdapter starts an adapter and publishes the static text window, leaving the signals both publishes sent
// unread, since none of the static text tests are about them.
func newLabelAdapter(t *testing.T) *testAdapter {
	t.Helper()
	ta := newTestAdapter(t)
	ta.Publish(labelWindow, labelTree(), nil, sampleGeometry())
	return ta
}

func TestTextInterfaceIsOnlyThereForText(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	// Neither taking the focus nor setting a selection is one of the things AT-SPI's Action interface covers, so a text
	// area that does both still has nothing to do.
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceEditableText, InterfaceText},
		ta.one(NodePath(41), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		ta.one(NodePath(44), InterfaceAccessible, "GetInterfaces", ""),
		"text that cannot be changed has no EditableText")
	// A password field carries no text at all in a published snapshot, so it has no text interface either: what an
	// assistive technology is told is that it is a password field, which is what ATSPI_ROLE_PASSWORD_TEXT says.
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent},
		ta.one(NodePath(43), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal(uint32(RolePasswordText), ta.one(NodePath(43), InterfaceAccessible, "GetRole", ""))
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(43), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"and nothing can ask it for a character count or for the characters themselves")
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(40), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"a window holds no text of its own")
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(3), InterfaceText, "GetText", "ii", int32(0), int32(-1)),
		"neither does a label that publishes no text of its own; see TestTextOfALabel for one that does")
}

func TestTextContent(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	c.Equal(int32(5), ta.peer.getProperty(NodePath(41), InterfaceText, "CharacterCount"))
	c.Equal(int32(4), ta.peer.getProperty(NodePath(41), InterfaceText, "CaretOffset"),
		"the caret is at the end of this selection")

	// Either end of a selection may be the one the caret is at: shift+Left and shift+Home extend it backwards, and a
	// client told the wrong end puts its review cursor at the wrong end of what it has just read out.
	backwards := textTree()
	backwards.Generation++
	backwards.Node(41).Text.Caret = 1
	ta.Publish(textWindow, backwards, nil, sampleGeometry())
	c.Equal(int32(1), ta.peer.getProperty(NodePath(41), InterfaceText, "CaretOffset"),
		"a selection extended backwards has its caret at the start")
	ta.Publish(textWindow, textTree(), nil, sampleGeometry())
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
		// The END forms divide the same text at different places: a line ends where its own characters stop rather
		// than after the line feed that follows them, so the range a client is handed for the same offset is a
		// separator shorter at one end and a separator longer at the other.
		{
			member: textAtOffset, why: "a line end runs to where the line's characters stop",
			boundary: BoundaryLineEnd, offset: 0, text: "ab", start: 0, end: 2,
		},
		{
			member: textAtOffset, why: "and the line feed belongs to the line after it",
			boundary: BoundaryLineEnd, offset: 2, text: "\n" + secondTextLine, start: 2, end: 5,
		},
		{
			member: textBeforeOffset, why: "the line end before the one holding the offset",
			boundary: BoundaryLineEnd, offset: 4, text: "ab", start: 0, end: 2,
		},
		{
			member: textAfterOffset, why: "the word end after the one holding the offset",
			boundary: BoundaryWordEnd, offset: 0, text: "\n" + secondTextLine, start: 2, end: 5,
		},
	} {
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(41), InterfaceText, one.member, "iu", one.offset, uint32(one.boundary)),
			"%s: %s", one.member, one.why)
	}
}

// TestTextUnits covers the division of content into units directly, over content the window the other text tests work
// with does not hold: real words, sentences and paragraphs. Every START unit runs to the start of the next one, so
// walking a control a unit at a time covers every character exactly once.
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
		{why: "the second sentence, ending at the line feed", unit: unitSentence, offset: 12, start: 11, end: 20},
		{why: "the last sentence runs to the end", unit: unitSentence, offset: 22, start: 20, end: 29},
		{why: "the first paragraph, with the line feed", unit: unitParagraph, offset: 3, start: 0, end: 20},
		{why: "the second paragraph", unit: unitParagraph, offset: 20, start: 20, end: 29},
		// A control the snapshot did not measure is divided at its own line feeds rather than being treated as one
		// line, so flat review reads an unfocused multi-line control a line at a time as its state set promises.
		{why: "an unmeasured control is divided at its line feeds", unit: unitLine, offset: 3, start: 0, end: 20},
		{why: "and its last line runs to the end", unit: unitLine, offset: 25, start: 20, end: 29},
	} {
		c.Equal(textRange{start: one.start, end: one.end}, content.rangeAt(one.unit, formStart, one.offset), one.why)
	}

	// A full stop with a character hard against it is part of the word rather than the end of a sentence, which is what
	// keeps a version number or a file name from being read as several of them.
	run := textContent{runes: []rune("Use v1.2.3 now. Then stop.")}
	c.Equal(textRange{start: 0, end: 16}, run.rangeAt(unitSentence, formStart, 0))
	c.Equal(textRange{start: 16, end: 26}, run.rangeAt(unitSentence, formStart, 20))

	// Walking forwards has to carry on from where the unit before it ended and reach the end of the content, which is
	// what makes a client reading the whole of a control with GetTextAfterOffset terminate. It holds for both forms:
	// they divide the same text at different places, but each of them divides all of it.
	for _, form := range []textForm{formStart, formEnd} {
		for _, unit := range []textUnit{unitChar, unitWord, unitSentence, unitLine, unitParagraph} {
			at := content.rangeAt(unit, form, 0)
			c.Equal(0, at.start, "unit %d in form %d must begin at the beginning", unit, form)
			for range len(content.runes) {
				if at.end >= len(content.runes) {
					break
				}
				next := content.rangeAfter(unit, form, at.start)
				c.Equal(at.end, next.start, "unit %d in form %d must carry on from where the one before it ended",
					unit, form)
				c.True(next.end > at.end, "unit %d in form %d must advance past %d", unit, form, at.end)
				at = next
			}
			c.Equal(len(content.runes), at.end, "unit %d in form %d must reach the end", unit, form)
		}
	}
	empty := textContent{}
	c.Equal(textRange{}, empty.rangeAt(unitWord, formStart, 0), "there is nothing in an empty control")
	c.Equal(textRange{}, empty.rangeBefore(unitLine, formStart, 0))
	c.Equal(textRange{}, empty.rangeAfter(unitLine, formEnd, 0))
}

// TestTextUnitsInTheEndForm covers the other way AT-SPI divides text into units of the same kind. The END form runs
// from the end of one unit to the end of the next rather than from start to start, so the separators between two units
// belong to the unit that follows them rather than to the one they follow. A client walking with the END form and
// answered in the START form lands a separator off on every step.
func TestTextUnitsInTheEndForm(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	content := textContent{runes: []rune("hello world")}
	c.Equal(textRange{start: 0, end: 5}, content.rangeAt(unitWord, formEnd, 0), "the first word, without the space")
	c.Equal(textRange{start: 0, end: 5}, content.rangeAt(unitWord, formEnd, 4))
	c.Equal(textRange{start: 5, end: 11}, content.rangeAt(unitWord, formEnd, 5),
		"the space belongs to the word that follows it")
	c.Equal(textRange{start: 5, end: 11}, content.rangeAt(unitWord, formEnd, 11), "the end of the text is in the last")
	c.Equal(textRange{start: 5, end: 11}, content.rangeAfter(unitWord, formEnd, 0))
	c.Equal(textRange{start: 0, end: 5}, content.rangeBefore(unitWord, formEnd, 7))
	c.Equal(textRange{}, content.rangeBefore(unitWord, formEnd, 2), "there is nothing before the first")

	// The same text in the START form, which is where the separator goes instead.
	c.Equal(textRange{start: 0, end: 6}, content.rangeAt(unitWord, formStart, 0))

	lines := textContent{runes: []rune("ab\ncd")}
	c.Equal(textRange{start: 0, end: 2}, lines.rangeAt(unitLine, formEnd, 0), "a line ends before its line feed")
	c.Equal(textRange{start: 2, end: 5}, lines.rangeAt(unitLine, formEnd, 2))
	sentences := textContent{runes: []rune("One. Two.")}
	c.Equal(textRange{start: 0, end: 4}, sentences.rangeAt(unitSentence, formEnd, 0),
		"a sentence ends at its full stop, without the spaces after it")
	c.Equal(textRange{start: 4, end: 9}, sentences.rangeAt(unitSentence, formEnd, 6))
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
	c.Equal([]any{int32(60), int32(20), int32(0), int32(40)},
		ta.values(NodePath(41), InterfaceText, "GetRangeExtents", "iiu", int32(2), int32(3), uint32(CoordWindow)),
		"a range that holds nothing but a line feed has no width, but it is still somewhere")
	c.Equal([]any{int32(0), int32(0), int32(0), int32(0)},
		ta.values(NodePath(41), InterfaceText, "GetRangeExtents", "iiu", int32(3), int32(3), uint32(CoordWindow)),
		"a range that holds no characters at all has no place either")

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

// TestTextOfALabel covers the static text a label hands over. A label is not typed into and holds no caret, but it is
// read by line, word and character like any other text: Orca's flat review walks its lines, and without the text
// interface all an assistive technology can say about it is its name, in one breath.
func TestTextOfALabel(t *testing.T) {
	t.Parallel()
	ta := newLabelAdapter(t)
	c := ta.c
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		ta.one(NodePath(51), InterfaceAccessible, "GetInterfaces", ""),
		"bringing itself into view is not one of the things AT-SPI's Action interface covers")
	c.Equal(int32(len(labelTextBody)), ta.peer.getProperty(NodePath(51), InterfaceText, "CharacterCount"))
	c.Equal(labelTextBody, ta.one(NodePath(51), InterfaceText, "GetText", "ii", int32(0), int32(-1)))
	c.Equal(int32(0), ta.peer.getProperty(NodePath(51), InterfaceText, "CaretOffset"),
		"nothing ever moves a caret about in a label")

	// The label sits at 10,10 within a window at 100,50 on the screen, and everything in it is twice the size in
	// physical pixels, so its first character — ten units wide and twenty tall at the label's own top left corner — is
	// forty pixels tall at 120,70.
	c.Equal([]any{int32(120), int32(70), int32(20), int32(40)},
		ta.values(NodePath(51), InterfaceText, "GetCharacterExtents", "iu", int32(0), uint32(CoordScreen)))
	c.Equal([]any{int32(20), int32(20), int32(20), int32(40)},
		ta.values(NodePath(51), InterfaceText, "GetCharacterExtents", "iu", int32(0), uint32(CoordWindow)))
	c.Equal([]any{int32(100), int32(20), int32(20), int32(40)},
		ta.values(NodePath(51), InterfaceText, "GetCharacterExtents", "iu", int32(4), uint32(CoordWindow)),
		"the last character is four of them along")
	c.Equal([]any{int32(20), int32(20), int32(100), int32(40)},
		ta.values(NodePath(51), InterfaceText, "GetRangeExtents", "iiu", int32(0), int32(-1), uint32(CoordWindow)),
		"the whole of it is the one line it was drawn on")

	// The whole label is one line, and a reader asking for the line at any offset within it gets all of it.
	c.Equal([]any{labelTextBody, int32(0), int32(len(labelTextBody))},
		ta.values(NodePath(51), InterfaceText, "GetStringAtOffset", "iu", int32(2), uint32(GranularityLine)))
	c.Equal([]any{"m", int32(2), int32(3)},
		ta.values(NodePath(51), InterfaceText, "GetStringAtOffset", "iu", int32(2), uint32(GranularityChar)))
	for _, one := range []struct {
		why      string
		x, y     int32
		expected int32
	}{
		{why: "the first character", x: 25, y: 25, expected: 0},
		{why: "the second character", x: 55, y: 25, expected: 1},
		{why: "past the end of the text is its last character", x: 135, y: 25, expected: 4},
		{why: "below the label altogether", x: 25, y: 400, expected: -1},
	} {
		c.Equal(one.expected, ta.one(NodePath(51), InterfaceText, "GetOffsetAtPoint", "iiu", one.x, one.y,
			uint32(CoordWindow)), one.why)
	}

	// A label lays its text out over one line, but nothing about it is typed into, nothing selects within it, and none
	// of its text is somebody else's link. SELECTABLE_TEXT goes with offering [accessibility.SetTextSelection], which
	// a label does not: claiming it would have a reader put a selection in the label and be refused.
	states := ta.statesOf(51)
	c.True(states.Has(StateSingleLine))
	c.False(states.Has(StateSelectableText))
	c.False(states.Has(StateMultiLine))
	c.False(states.Has(StateEditable))
	c.Equal(dbus.UnknownInterface,
		ta.errorName(NodePath(51), InterfaceEditableText, "InsertText", "isi", int32(0), "no", int32(2)))
	c.Equal(dbus.UnknownInterface, ta.errorName(NodePath(51), InterfaceHypertext, "GetNLinks", ""))
}

// TestTextOfAnUnmeasuredColumnHeader covers the static text of a node that published what it drew without where it
// drew it, which is what everything but the focus of a window does: the whole node stands in for the one line, so a
// reader is told where the text is rather than where each character of it is.
func TestTextOfAnUnmeasuredColumnHeader(t *testing.T) {
	t.Parallel()
	ta := newLabelAdapter(t)
	c := ta.c
	c.Equal(int32(len(headerTextBody)), ta.peer.getProperty(NodePath(52), InterfaceText, "CharacterCount"))
	c.Equal(headerTextBody, ta.one(NodePath(52), InterfaceText, "GetText", "ii", int32(0), int32(-1)))
	// The header is eighty units wide and twenty tall at 10,40 within the window, which is the area every one of its
	// characters is reported at.
	c.Equal([]any{int32(20), int32(80), int32(160), int32(40)},
		ta.values(NodePath(52), InterfaceText, "GetCharacterExtents", "iu", int32(0), uint32(CoordWindow)))
	c.Equal([]any{int32(20), int32(80), int32(160), int32(40)},
		ta.values(NodePath(52), InterfaceText, "GetRangeExtents", "iiu", int32(0), int32(-1), uint32(CoordWindow)))
	c.Equal([]any{headerTextBody, int32(0), int32(len(headerTextBody))},
		ta.values(NodePath(52), InterfaceText, "GetStringAtOffset", "iu", int32(1), uint32(GranularityLine)),
		"the whole of an unmeasured header is one line")
	c.Equal(int32(0), ta.one(NodePath(52), InterfaceText, "GetOffsetAtPoint", "iiu", int32(150), int32(90),
		uint32(CoordWindow)), "with no measurements there is no telling which character is where")
	c.True(ta.statesOf(52).Has(StateSingleLine))
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
	c.Equal(false, ta.one(NodePath(44), InterfaceText, "SetCaretOffset", "i", int32(1)),
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

	// A range that is reversed, or that reaches outside the content, is refused exactly as an offset outside it is.
	// Bringing it within the content instead would turn AddSelection(3, 1) on the two-rune field into a collapsed
	// caret and still report that a selection had been made, which the next snapshot would then contradict.
	for _, one := range []struct {
		start, end int32
	}{
		{start: 3, end: 1},
		{start: -1, end: 2},
		{start: 0, end: 3},
		{start: 4, end: 5},
	} {
		c.Equal(false, ta.one(NodePath(42), InterfaceText, "AddSelection", "ii", one.start, one.end),
			"%d to %d is not a range the field holds", one.start, one.end)
		c.Equal(false, ta.one(NodePath(42), InterfaceText, "SetSelection", "iii", int32(0), one.start, one.end),
			"%d to %d is not a range the field holds", one.start, one.end)
	}
	ta.noRequest(t)
}

// TestTextSynthesizedFromAValue covers the read-only text a node whose value is textual hands over. A popup menu
// reports the item it has chosen as its Value and a color well the ink it holds; neither is a number, so neither has an
// org.a11y.atspi.Value interface to be read through, and without this an assistive technology would be told what such a
// control is called and never what is in it.
func TestTextSynthesizedFromAValue(t *testing.T) {
	t.Parallel()
	ta := newTestAdapter(t)
	c := ta.c
	// The main window's field is carrying no text of its own, so its value is what there is to read.
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceText},
		ta.one(NodePath(4), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal(int32(4), ta.peer.getProperty(NodePath(4), InterfaceText, "CharacterCount"))
	c.Equal(int32(0), ta.peer.getProperty(NodePath(4), InterfaceText, "CaretOffset"),
		"there is no caret in a value, so the start of the content is what is reported")
	c.Equal(testFieldValue, ta.one(NodePath(4), InterfaceText, "GetText", "ii", int32(0), int32(-1)))
	c.Equal("re", ta.one(NodePath(4), InterfaceText, "GetText", "ii", int32(1), int32(3)))
	c.Equal(int32('F'), ta.one(NodePath(4), InterfaceText, "GetCharacterAtOffset", "i", int32(0)))
	c.Equal([]any{testFieldValue, int32(0), int32(4)},
		ta.values(NodePath(4), InterfaceText, "GetStringAtOffset", "iu", int32(0), uint32(GranularityLine)))
	c.Equal(int32(0), ta.one(NodePath(4), InterfaceText, "GetNSelections", ""), "and nothing is selected in one")

	// Nothing about it can be changed: there is no caret to move, no selection to set, and no interface to type
	// through. The states that describe a control the user is inside are not claimed either.
	c.Equal(false, ta.one(NodePath(4), InterfaceText, "SetCaretOffset", "i", int32(2)))
	c.Equal(false, ta.one(NodePath(4), InterfaceText, "AddSelection", "ii", int32(0), int32(2)))
	c.Equal(false, ta.one(NodePath(4), InterfaceText, "SetSelection", "iii", int32(0), int32(0), int32(2)))
	c.Equal(false, ta.one(NodePath(4), InterfaceText, "RemoveSelection", "i", int32(0)))
	c.Equal(dbus.UnknownInterface,
		ta.errorName(NodePath(4), InterfaceEditableText, "SetTextContents", "s", changedFieldValue))
	states, ok := ta.one(NodePath(4), InterfaceAccessible, "GetState", "").([]uint32)
	c.True(ok)
	var set StateSet
	set[0], set[1] = states[0], states[1]
	c.False(set.Has(StateEditable))
	c.False(set.Has(StateSelectableText))
	ta.noRequest(t)

	// A popup menu is the case this exists for: its role has no text of its own at all, and the item it has chosen is
	// its value.
	popup := mainTree()
	popup.Generation++
	chooser := popup.Node(4)
	chooser.Role = role.PopupButton
	chooser.Value = testChosenValue
	ta.Publish(mainWindow, popup, nil, sampleGeometry())
	c.Equal(uint32(RoleComboBox), ta.one(NodePath(4), InterfaceAccessible, "GetRole", ""))
	c.Equal(testChosenValue, ta.one(NodePath(4), InterfaceText, "GetText", "ii", int32(0), int32(-1)))

	// So is a populated table cell: Table.axAddRow clears such a cell's name and puts what its content reports into
	// the value, which would otherwise leave the cell with nothing an assistive technology could read.
	table := tableTree()
	cell := table.Node(65)
	cell.Name = ""
	cell.Value = "10"
	ta.Publish(tableWindow, table, nil, sampleGeometry())
	c.Equal([]string{InterfaceAccessible, InterfaceCollection, InterfaceComponent, InterfaceTableCell, InterfaceText},
		ta.one(NodePath(65), InterfaceAccessible, "GetInterfaces", ""))
	c.Equal("10", ta.one(NodePath(65), InterfaceText, "GetText", "ii", int32(0), int32(-1)))
	c.Equal("", ta.peer.getProperty(NodePath(65), InterfaceAccessible, "Name"))
}

// TestEditableText covers the only way AT-SPI has of putting characters into a control. org.a11y.atspi.Text moves the
// caret and the selection but never changes a character, and [States] claims ATSPI_STATE_EDITABLE for these nodes, so
// without this interface an assistive technology could read a Unison field and walk about inside it but never edit it —
// while the same field is writable through VoiceOver and through UI Automation's value and text patterns.
func TestEditableText(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	// Replacing the whole content of a control that takes a value is a value change, which is what a control with no
	// notion of ranges, such as a spin button, knows what to do with.
	c.Equal(true, ta.one(NodePath(42), InterfaceEditableText, "SetTextContents", "s", "Bye"))
	c.Equal(accessibility.ActionRequest{Node: 42, Action: accessibility.SetValue, Value: "Bye"}, ta.nextRequest(t))
	// One that only offers to have a range of its text replaced has the whole of it replaced instead.
	c.Equal(true, ta.one(NodePath(41), InterfaceEditableText, "SetTextContents", "s", "new"))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.ReplaceText, Value: "new", Start: 0, End: 5,
	}, ta.nextRequest(t))

	// An insertion is an empty range replaced by the text, and a deletion is a range replaced by nothing.
	c.Equal(true, ta.one(NodePath(41), InterfaceEditableText, "InsertText", "isi", int32(2), "xy", int32(2)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.ReplaceText, Value: "xy", Start: 2, End: 2,
	}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(41), InterfaceEditableText, "InsertText", "isi", int32(0), "xy", int32(1)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.ReplaceText, Value: "x", Start: 0, End: 0,
	}, ta.nextRequest(t), "a length shorter than the string is how many characters of it the caller means")
	c.Equal(true, ta.one(NodePath(41), InterfaceEditableText, "DeleteText", "ii", int32(1), int32(3)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.ReplaceText, Start: 1, End: 3,
	}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(41), InterfaceEditableText, "DeleteText", "ii", int32(-5), int32(99)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.ReplaceText, Start: 0, End: 5,
	}, ta.nextRequest(t), "a range outside the content is brought back into it")
	c.Equal(true, ta.one(NodePath(41), InterfaceEditableText, "DeleteText", "ii", int32(4), int32(1)))
	c.Equal(accessibility.ActionRequest{
		Node: 41, Action: accessibility.ReplaceText, Start: 4, End: 4,
	}, ta.nextRequest(t), "a range that ends before it starts is the empty range where it starts")

	// The clipboard is not something the schema carries a request for, so the three methods that would use it report
	// that they did nothing. Copying is the one method of the interface that answers with nothing at all.
	c.Equal(0, len(ta.values(NodePath(41), InterfaceEditableText, "CopyText", "ii", int32(0), int32(2))))
	c.Equal(false, ta.one(NodePath(41), InterfaceEditableText, "CutText", "ii", int32(0), int32(2)))
	c.Equal(false, ta.one(NodePath(41), InterfaceEditableText, "PasteText", "i", int32(0)))

	// A control that does not offer to have its text changed has no such interface to call at all, and neither has one
	// that holds no text.
	c.Equal(dbus.UnknownInterface,
		ta.errorName(NodePath(44), InterfaceEditableText, "DeleteText", "ii", int32(0), int32(1)))
	c.Equal(dbus.UnknownInterface,
		ta.errorName(NodePath(43), InterfaceEditableText, "SetTextContents", "s", "guess"))
	ta.noRequest(t)
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
	// An offset the content does not reach is an empty run, which is what ATK answers with and what stops a client
	// walking the runs of a control from being handed the whole of it again at the end.
	for _, offset := range []int32{-1, 5, 99} {
		c.Equal([]any{dbus.Dict{}, int32(0), int32(0)},
			ta.values(NodePath(41), InterfaceText, "GetAttributes", "i", offset), "offset %d is outside the text",
			offset)
		c.Equal([]any{dbus.Dict{}, int32(0), int32(0)},
			ta.values(NodePath(41), InterfaceText, "GetAttributeRun", "ib", offset, true))
	}
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
		InterfaceEditableText,
		`<property name="CharacterCount" type="i" access="read"/>`,
		`<property name="CaretOffset" type="i" access="read"/>`,
		`<method name="GetStringAtOffset">`,
		`<method name="GetCharacterExtents">`,
		`<method name="GetAttributeRun">`,
		`<method name="InsertText">`,
	} {
		c.True(strings.Contains(xml, want), "the introspection of a text node must mention %s", want)
	}
	// What the node says it implements has to be what is there, in the same order, so that a client that walks the
	// introspection and one that asks the shorter question see the same object.
	advertised, ok := ta.one(NodePath(41), InterfaceAccessible, "GetInterfaces", "").([]string)
	c.True(ok)
	c.Equal(advertised, atspiInterfacesIn(xml))
}

// documentRunAttributes is what one styled run of the document reports. Every run says all six of these things, so a
// client comparing two of them to find out where the style changes is comparing like with like.
func documentRunAttributes(weight int, underline, family, size string) dbus.Dict {
	return dbus.Dict{
		{Key: weightTextAttribute, Value: strconv.Itoa(weight)},
		{Key: styleTextAttribute, Value: normalStyleValue},
		{Key: underlineTextAttribute, Value: underline},
		{Key: strikethroughTextAttribute, Value: falseValue},
		{Key: familyNameTextAttribute, Value: family},
		{Key: sizeTextAttribute, Value: size},
	}
}

// proseAttributes is what the document's ordinary prose reports, and linkAttributes what the bold, underlined run over
// the link within the paragraph does.
func proseAttributes() dbus.Dict {
	return documentRunAttributes(regularWeight, noUnderlineValue, proseFamily, "12")
}

func linkAttributes() dbus.Dict {
	return documentRunAttributes(boldWeight, singleUnderlineValue, proseFamily, "12")
}

// TestTextAttributesFromRuns covers the attributes of a document's block, which is the one text Unison styles a piece
// at a time. The range each answer covers is what a client walks the content by, asking about the offset the last run
// ended at.
func TestTextAttributesFromRuns(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	for _, one := range []struct {
		why        string
		attributes dbus.Dict
		offset     int32
		start      int32
		end        int32
	}{
		{offset: 0, attributes: proseAttributes(), start: 0, end: paragraphLinkStart, why: "the prose before the link"},
		{offset: 4, attributes: proseAttributes(), start: 0, end: paragraphLinkStart},
		{
			offset: paragraphLinkStart, attributes: linkAttributes(), start: paragraphLinkStart,
			end: paragraphLinkEnd, why: "the link itself, which is bold and underlined",
		},
		{offset: paragraphLinkEnd - 1, attributes: linkAttributes(), start: paragraphLinkStart, end: paragraphLinkEnd},
		{
			offset: paragraphLinkEnd, attributes: proseAttributes(), start: paragraphLinkEnd,
			end: paragraphImageStart, why: "the prose between the link and the image, which reaches neither",
		},
		{
			offset: paragraphImageStart, attributes: proseAttributes(), start: paragraphImageStart,
			end: paragraphImageEnd, why: "the one character the image occupies",
		},
		{
			offset: paragraphImageEnd, attributes: proseAttributes(), start: paragraphImageEnd,
			end: documentParagraphLength, why: "the prose after the image",
		},
		{
			offset: documentParagraphLength - 1, attributes: proseAttributes(), start: paragraphImageEnd,
			end: documentParagraphLength,
		},
	} {
		c.Equal([]any{one.attributes, one.start, one.end},
			ta.values(NodePath(103), InterfaceText, "GetAttributes", "i", one.offset),
			"the attributes at offset %d: %s", one.offset, one.why)
		c.Equal([]any{one.attributes, one.start, one.end},
			ta.values(NodePath(103), InterfaceText, "GetAttributeRun", "ib", one.offset, true),
			"folding in the defaults adds nothing, since a run reports everything")
	}
	// An offset the block does not reach has no attributes and an empty range, which is what stops a client walking the
	// runs from being handed the whole content again at the end.
	for _, offset := range []int32{-1, documentParagraphLength, 99} {
		c.Equal([]any{dbus.Dict{}, int32(0), int32(0)},
			ta.values(NodePath(103), InterfaceText, "GetAttributes", "i", offset))
	}
	// The defaults are the attributes of the first run, which is the style the block's prose begins in.
	c.Equal(proseAttributes(), ta.one(NodePath(103), InterfaceText, "GetDefaultAttributes", ""))
	// One attribute at a time, which is how a client that only cares about the weight asks.
	c.Equal("700", ta.one(NodePath(103), InterfaceText, "GetAttributeValue", "is", int32(paragraphLinkStart),
		weightTextAttribute))
	c.Equal(proseFamily, ta.one(NodePath(103), InterfaceText, "GetAttributeValue", "is", int32(0),
		familyNameTextAttribute))
	c.Equal("", ta.one(NodePath(103), InterfaceText, "GetAttributeValue", "is", int32(0), "justification"),
		"an attribute this package does not report has no value")
	c.Equal("", ta.one(NodePath(103), InterfaceText, "GetAttributeValue", "is", int32(99), weightTextAttribute),
		"and neither has any attribute of an offset the text does not reach")

	// The other blocks are drawn in one style throughout, so the whole of each is one run: code in a fixed-pitch face,
	// a heading larger than the prose.
	c.Equal([]any{
		documentRunAttributes(regularWeight, noUnderlineValue, codeFamily, "11"),
		int32(0), int32(len([]rune(documentCodeText))),
	}, ta.values(NodePath(108), InterfaceText, "GetAttributes", "i", int32(3)))
	c.Equal([]any{
		documentRunAttributes(boldWeight, noUnderlineValue, proseFamily, "18"),
		int32(0), int32(len([]rune(documentHeadingText))),
	}, ta.values(NodePath(102), InterfaceText, "GetAttributes", "i", int32(0)))
}

// TestTextAttributesOfStyledRuns covers the values themselves, which are the ones ATK reports and the ones Orca reads.
// A run that says nothing about its family or its size reports neither, rather than reporting an empty name and a size
// of zero as though the text were drawn in them.
func TestTextAttributesOfStyledRuns(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	c.Equal(dbus.Dict{
		{Key: weightTextAttribute, Value: "400"},
		{Key: styleTextAttribute, Value: normalStyleValue},
		{Key: underlineTextAttribute, Value: noUnderlineValue},
		{Key: strikethroughTextAttribute, Value: falseValue},
	}, runAttributes(accessibility.TextRun{End: 4}), "a run that says nothing is regular, upright and undecorated")
	c.Equal(dbus.Dict{
		{Key: weightTextAttribute, Value: "300"},
		{Key: styleTextAttribute, Value: italicStyleValue},
		{Key: underlineTextAttribute, Value: singleUnderlineValue},
		{Key: strikethroughTextAttribute, Value: trueValue},
		{Key: familyNameTextAttribute, Value: "Mono"},
		{Key: sizeTextAttribute, Value: "10.5"},
	}, runAttributes(accessibility.TextRun{
		Family: "Mono", Size: 10.5, Start: 0, End: 4, Weight: 300, Italic: true, Underline: true,
		Strikethrough: true, Monospace: true,
	}), "a fractional size keeps its fraction, and a monospaced face is said with its name")
	c.Equal(dbus.Dict{}, defaultTextAttributes(nil), "a node with no text has no defaults")
	c.Equal(dbus.Dict{}, defaultTextAttributes(&accessibility.TextInfo{Text: "plain"}))
}

// TestScrollSubstringTo covers the call Orca makes after every caret move it asks for, which is what keeps a document
// whose reader has arrowed past the bottom of the view port scrolling along with them.
func TestScrollSubstringTo(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	ta.Publish(textWindow, textTree(), nil, sampleGeometry())
	c.Equal(true, ta.one(NodePath(103), InterfaceText, "ScrollSubstringTo", "iiu", int32(paragraphLinkStart),
		int32(paragraphLinkEnd), uint32(0)))
	c.Equal(accessibility.ActionRequest{
		Node: 103, Action: accessibility.ScrollRangeIntoView, Start: paragraphLinkStart, End: paragraphLinkEnd,
	}, ta.nextRequest(t))
	// A range reaching outside the content is brought within it rather than refused: refusing would leave the caret at
	// the end of a document off the bottom of the screen.
	c.Equal(true, ta.one(NodePath(103), InterfaceText, "ScrollSubstringTo", "iiu", int32(-5), int32(99), uint32(0)))
	c.Equal(accessibility.ActionRequest{
		Node: 103, Action: accessibility.ScrollRangeIntoView, Start: 0, End: documentParagraphLength,
	}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(103), InterfaceText, "ScrollSubstringTo", "iiu", int32(paragraphLinkEnd), int32(2),
		uint32(0)))
	c.Equal(accessibility.ActionRequest{
		Node: 103, Action: accessibility.ScrollRangeIntoView, Start: paragraphLinkEnd, End: paragraphLinkEnd,
	}, ta.nextRequest(t), "a range that ends before it starts is the empty range where it starts")

	// A control that does not offer to scroll a range of its text into view says no and is asked for nothing.
	c.Equal(false, ta.one(NodePath(41), InterfaceText, "ScrollSubstringTo", "iiu", int32(0), int32(2), uint32(0)))

	// A label cannot scroll to a range of its text, but it can bring itself into view, which puts the range the
	// caller asked about on the screen just the same.
	ta.Publish(labelWindow, labelTree(), nil, sampleGeometry())
	c.Equal(true, ta.one(NodePath(51), InterfaceText, "ScrollSubstringTo", "iiu", int32(0), int32(2), uint32(0)))
	c.Equal(accessibility.ActionRequest{Node: 51, Action: accessibility.ScrollIntoView}, ta.nextRequest(t))
	// A node that offers neither says no and is asked for nothing. No published node reaches this: axSnapshot gives
	// every one of them ScrollIntoView and keeps it even for a disabled node, so the header is stripped of its actions
	// by the fixture to get the refusal branch under test.
	c.Equal(false, ta.one(NodePath(52), InterfaceText, "ScrollSubstringTo", "iiu", int32(0), int32(2), uint32(0)))
	// Where in the view the range ends up is the widget's business, so the one method that asks for a particular place
	// on the screen reports that it did nothing, as ScrollToPoint does.
	c.Equal(false, ta.one(NodePath(103), InterfaceText, "ScrollSubstringToPoint", "iiuii", int32(0), int32(2),
		uint32(0), int32(10), int32(10)))
	ta.noRequest(t)
}

// TestCaretPlacementInAReadOnlyBlock covers what Orca's caret navigation does as it crosses from one block of a
// document into the next: it puts the caret in the block it has moved to. The block's content cannot be changed, which
// is a different thing from its caret being fixed, so the request is carried out although the object hands over no
// interface to edit it through.
func TestCaretPlacementInAReadOnlyBlock(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	c.Equal(true, ta.one(NodePath(103), InterfaceText, "SetCaretOffset", "i", int32(7)))
	c.Equal(accessibility.ActionRequest{
		Node: 103, Action: accessibility.SetTextSelection, Start: 7, End: 7,
	}, ta.nextRequest(t))
	c.Equal(true, ta.one(NodePath(103), InterfaceText, "SetSelection", "iii", int32(0), int32(paragraphLinkStart),
		int32(paragraphLinkEnd)))
	c.Equal(accessibility.ActionRequest{
		Node: 103, Action: accessibility.SetTextSelection, Start: paragraphLinkStart, End: paragraphLinkEnd,
	}, ta.nextRequest(t))
	// An offset the block does not reach is refused rather than brought into it, since a client told the caret went
	// where it asked would have to be told otherwise by the next snapshot.
	c.Equal(false, ta.one(NodePath(103), InterfaceText, "SetCaretOffset", "i", int32(99)))
	c.Equal(false, ta.one(NodePath(103), InterfaceText, "SetCaretOffset", "i", int32(-1)))
	// Nothing about the block can be typed into, and nothing is selected in it to begin with.
	c.Equal(dbus.UnknownInterface,
		ta.errorName(NodePath(103), InterfaceEditableText, "InsertText", "isi", int32(0), "no", int32(2)))
	c.Equal(int32(0), ta.one(NodePath(103), InterfaceText, "GetNSelections", ""))
	c.Equal(int32(0), ta.peer.getProperty(NodePath(103), InterfaceText, "CaretOffset"))
	ta.noRequest(t)

	// The caret and the selection of a block are read from the block, as a field's are.
	selected := documentTree()
	selected.Generation++
	selected.Node(103).Text.SelStart = paragraphLinkStart
	selected.Node(103).Text.SelEnd = paragraphLinkEnd
	selected.Node(103).Text.Caret = paragraphLinkStart
	ta.Publish(documentWindow, selected, nil, sampleGeometry())
	c.Equal(int32(1), ta.one(NodePath(103), InterfaceText, "GetNSelections", ""))
	c.Equal([]any{int32(paragraphLinkStart), int32(paragraphLinkEnd)},
		ta.values(NodePath(103), InterfaceText, "GetSelection", "i", int32(0)))
	c.Equal(int32(paragraphLinkStart), ta.peer.getProperty(NodePath(103), InterfaceText, "CaretOffset"),
		"a selection extended backwards has its caret at the start")
}

// TestTextAttributeWireStrings pins the strings this package hands an assistive technology, written out rather than
// spelled with the same constants the production code uses. The keys and values are the whole of the contract with Orca
// — it reads weight to decide whether a stretch of text is bold, style for italic, underline and strikethrough for the
// two decorations, and xml-roles together with tag to recognize a block of code — so a typo inside one of those
// constants would leave every other expectation in the suite agreeing with it and silently break what Orca matches on.
func TestTextAttributeWireStrings(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		why  string
		want dbus.Dict
		run  accessibility.TextRun
	}{
		{
			why: "a run that says nothing about how it is drawn still reports the four attributes every run has",
			run: accessibility.TextRun{End: 4},
			want: dbus.Dict{
				{Key: "weight", Value: "400"},
				{Key: "style", Value: "normal"},
				{Key: "underline", Value: "none"},
				{Key: "strikethrough", Value: "false"},
			},
		},
		{
			why: "and one that says everything a Unison font can reports all six",
			run: accessibility.TextRun{
				End: 4, Family: "Serif", Size: 12.5, Weight: 700, Italic: true, Underline: true, Strikethrough: true,
			},
			want: dbus.Dict{
				{Key: "weight", Value: "700"},
				{Key: "style", Value: "italic"},
				{Key: "underline", Value: "single"},
				{Key: "strikethrough", Value: "true"},
				{Key: "family-name", Value: "Serif"},
				{Key: "size", Value: "12.5"},
			},
		},
	} {
		c.Equal(one.want, runAttributes(one.run), one.why)
	}

	// The object attributes of a block of code, which is the one role AT-SPI has no role for: the pair below is what
	// Orca reads on a web page, where a code block is a <pre> element carrying the code role.
	c.Equal(dbus.Dict{
		{Key: "toolkit", Value: "unison"},
		{Key: "xml-roles", Value: "code"},
		{Key: "tag", Value: "pre"},
	}, Attributes(nil, documentCode(1, documentCodeText, geom.Rect{}), nil))
}

// TestWordUnitsAroundAnInlineObject covers the division of a block's words around the U+FFFC that stands in for an
// image, which is the one place the word rule is more than "a run of characters that are not spaces". The character has
// nothing to read out, so it is a word of its own however tightly it is written against the prose — otherwise a screen
// reader walking by word would read the image's name as part of the word beside it — while a space that follows it
// belongs to that word rather than beginning one of its own: an empty word would have Orca stop between the image and
// the word after it with nothing to say, which is exactly what Markdown's `![alt](x) text` renders as.
//
// Both shapes are asked for here, since only the tight one tells this rule apart from a plain scan for spaces.
func TestWordUnitsAroundAnInlineObject(t *testing.T) {
	t.Parallel()
	ta := newDocumentAdapter(t)
	c := ta.c
	const object = "￼"
	for _, one := range []struct {
		why    string
		text   string
		offset int32
		start  int32
		end    int32
	}{
		{why: "the word before the image", text: "guide ", offset: 9, start: 9, end: paragraphImageStart},
		{
			why: "the word before the image, from the space that ends it", text: "guide ", offset: 14, start: 9,
			end: paragraphImageStart,
		},
		{
			why: "the image is a word of its own, and takes the space after it", text: object + " ",
			offset: paragraphImageStart, start: paragraphImageStart, end: paragraphImageEnd + 1,
		},
		{
			why:  "the space after the image is part of the image's word rather than a word of its own",
			text: object + " ", offset: paragraphImageEnd, start: paragraphImageStart, end: paragraphImageEnd + 1,
		},
		{
			why: "and the word after it begins at its own first character", text: "now.",
			offset: paragraphImageEnd + 1, start: paragraphImageEnd + 1, end: documentParagraphLength,
		},
	} {
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(103), InterfaceText, "GetStringAtOffset", "iu", one.offset, uint32(GranularityWord)),
			one.why)
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(103), InterfaceText, textAtOffset, "iu", one.offset, uint32(BoundaryWordStart)),
			"the older form answers the same unit: %s", one.why)
	}
	c.Equal([]any{"now.", int32(paragraphImageEnd + 1), int32(documentParagraphLength)},
		ta.values(NodePath(103), InterfaceText, textAfterOffset, "iu", int32(paragraphImageStart),
			uint32(BoundaryWordStart)),
		"walking on from the image reaches the next word rather than the space after it")

	// The same rule with nothing to separate the image from the prose, which is what `text![alt](x)text` renders as:
	// three words, one of them the object replacement on its own.
	const tight = "head" + object + "tail"
	tree := documentTree()
	tree.Generation++
	tree.Nodes[107] = documentBlock(107, 106, role.Paragraph, tight, geom.NewRect(0, 60, 300, 20))
	ta.Publish(documentWindow, tree, nil, sampleGeometry())
	for _, one := range []struct {
		text   string
		offset int32
		start  int32
		end    int32
	}{
		{offset: 0, text: "head", start: 0, end: 4},
		{offset: 3, text: "head", start: 0, end: 4},
		{offset: 4, text: object, start: 4, end: 5},
		{offset: 5, text: "tail", start: 5, end: 9},
		{offset: 8, text: "tail", start: 5, end: 9},
	} {
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(107), InterfaceText, "GetStringAtOffset", "iu", one.offset, uint32(GranularityWord)),
			"the word at offset %d of %q", one.offset, tight)
		c.Equal([]any{one.text, one.start, one.end},
			ta.values(NodePath(107), InterfaceText, textAtOffset, "iu", one.offset, uint32(BoundaryWordStart)),
			"and the older form says the same")
	}
}
