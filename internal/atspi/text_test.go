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

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/dbus"
)

// textWindow is the window the text tests publish.
const textWindow WindowKey = 3

// The granularities of org.a11y.atspi.Text.GetStringAtOffset, from AtspiTextGranularity. V1 answers with the line
// whichever one is asked for, so the tests use the two that matter: the one that is honored and one that is not.
const (
	granularityChar = uint32(0)
	granularityLine = uint32(3)
)

// textBody is the content of the measured text area: two lines, the first of which ends with a line feed.
const textBody = "ab\ncd"

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

func TestTextLines(t *testing.T) {
	t.Parallel()
	ta := newTextAdapter(t)
	c := ta.c
	for _, member := range []string{
		"GetStringAtOffset", "GetTextAtOffset", "GetTextBeforeOffset", "GetTextAfterOffset",
	} {
		// V1 works in lines, whatever granularity or boundary the caller asked for, and a line takes the line feed that
		// ends it with it.
		c.Equal([]any{"ab\n", int32(0), int32(3)},
			ta.values(NodePath(41), InterfaceText, member, "iu", int32(0), granularityLine), member)
		c.Equal([]any{"ab\n", int32(0), int32(3)},
			ta.values(NodePath(41), InterfaceText, member, "iu", int32(2), granularityChar), member)
		c.Equal([]any{"cd", int32(3), int32(5)},
			ta.values(NodePath(41), InterfaceText, member, "iu", int32(4), granularityLine), member)
		c.Equal([]any{"cd", int32(3), int32(5)},
			ta.values(NodePath(41), InterfaceText, member, "iu", int32(5), granularityLine), member,
			"the end of the text belongs to the last line")
	}

	// A field that has not been measured, which is every text control but the focused one, reports its whole content as
	// one line.
	c.Equal([]any{"Hi", int32(0), int32(2)},
		ta.values(NodePath(42), InterfaceText, "GetStringAtOffset", "iu", int32(1), granularityLine))
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
		ta.values(NodePath(43), InterfaceText, "GetStringAtOffset", "iu", int32(0), granularityLine))
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
