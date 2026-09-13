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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/dbus"
)

// bulletRune is what a protected node reports in place of every character it holds. A node whose Protected flag is set
// has no Text at all in a published snapshot, so this only comes into play if one ever does: what was typed into a
// password field must never leave the process, but its length is what an assistive technology reads out as it is typed.
const bulletRune = '•'

// textInterface returns the org.a11y.atspi.Text interface of a node that holds navigable text, which is how an assistive
// technology reads a control a character, a word, a line or a selection at a time rather than as one string.
//
// Every offset is a rune index, which is what AT-SPI calls a character. Nothing here changes the control: moving the
// caret or the selection is handed to the user interface thread and answered optimistically, exactly as the other
// interfaces do.
//
// Four of the interface's methods are left out, since nothing Unison reports could answer them with more than a
// refusal: GetBoundedRanges, which asks which ranges of text lie within a rectangle; GetDefaultAttributeSet, which is
// GetDefaultAttributes under an older name that libatspi no longer calls; and ScrollSubstringTo and
// ScrollSubstringToPoint, which ask for part of the text to be brought into view. A caller that asks for one of them is
// answered with an unknown method, which is what libatspi expects of an interface it has to treat as optional anyway.
func (o *nodeObject) textInterface() *dbus.Interface {
	return &dbus.Interface{
		Name: InterfaceText,
		Methods: []*dbus.Method{
			{Name: "GetText", In: "ii", Out: "s", Handle: o.getText},
			{Name: "GetStringAtOffset", In: "iu", Out: textRangeSignature, Handle: o.getStringAtOffset},
			{Name: "GetTextAtOffset", In: "iu", Out: textRangeSignature, Handle: o.getTextAtOffset},
			{Name: "GetTextBeforeOffset", In: "iu", Out: textRangeSignature, Handle: o.getTextBeforeOffset},
			{Name: "GetTextAfterOffset", In: "iu", Out: textRangeSignature, Handle: o.getTextAfterOffset},
			{Name: "GetCharacterAtOffset", In: "i", Out: "i", Handle: o.getCharacterAtOffset},
			{Name: "SetCaretOffset", In: "i", Out: "b", Handle: o.setCaretOffset},
			{Name: "GetNSelections", Out: "i", Handle: o.getNSelections},
			{Name: "GetSelection", In: "i", Out: "ii", Handle: o.getSelection},
			{Name: "AddSelection", In: "ii", Out: "b", Handle: o.addSelection},
			{Name: "RemoveSelection", In: "i", Out: "b", Handle: o.removeSelection},
			{Name: "SetSelection", In: "iii", Out: "b", Handle: o.setSelection},
			{Name: "GetCharacterExtents", In: "iu", Out: textExtentsSignature, Handle: o.getCharacterExtents},
			{
				Name: "GetRangeExtents", In: intPairAndCoordSignature, Out: textExtentsSignature,
				Handle: o.getRangeExtents,
			},
			{Name: "GetOffsetAtPoint", In: intPairAndCoordSignature, Out: "i", Handle: o.getOffsetAtPoint},
			{Name: "GetAttributes", In: "i", Out: textAttributesSignature, Handle: o.getTextAttributes},
			{Name: "GetAttributeRun", In: "ib", Out: textAttributesSignature, Handle: o.getAttributeRun},
			{Name: "GetAttributeValue", In: "is", Out: "s", Handle: o.getAttributeValue},
			{Name: "GetDefaultAttributes", Out: stringDictSignature, Handle: o.getDefaultAttributes},
		},
		Properties: []*dbus.Property{
			{Name: "CharacterCount", Sig: "i", Get: func() (any, error) { return int32(len(o.textRunes())), nil }},
			{Name: "CaretOffset", Sig: "i", Get: func() (any, error) { return int32(o.caret()), nil }},
		},
	}
}

// textRunes returns the characters of the node's text. A protected node reports bullets: the same number of characters
// as it holds, none of them the ones the user typed.
func (o *nodeObject) textRunes() []rune {
	if o.node.Text == nil {
		return nil
	}
	runes := []rune(o.node.Text.Text)
	if o.node.Protected {
		for i := range runes {
			runes[i] = bulletRune
		}
	}
	return runes
}

// caret returns the rune index the caret sits at, which is the end of the selection.
func (o *nodeObject) caret() int {
	if o.node.Text == nil {
		return 0
	}
	return o.node.Text.SelEnd
}

// selection returns the selected range of runes, and whether anything is selected at all.
func (o *nodeObject) selection() (start, end int, exists bool) {
	info := o.node.Text
	if info == nil || info.SelStart == info.SelEnd {
		return 0, 0, false
	}
	return info.SelStart, info.SelEnd, true
}

// lines returns the laid-out lines of the node's text. A snapshot only measures the lines of the control that has the
// focus, since the measurements are too expensive to take for every text control in a window; when there are none, the
// whole content is reported as one line filling the node, which is what an assistive technology gets for an unfocused
// field. count is how many runes the text holds.
func (o *nodeObject) lines(count int) []accessibility.Line {
	if info := o.node.Text; info != nil && len(info.Lines) != 0 {
		return info.Lines
	}
	return []accessibility.Line{
		{Start: 0, End: count, Bounds: geom.NewRect(0, 0, o.node.Bounds.Width, o.node.Bounds.Height)},
	}
}

// content returns the node's text as the questions about it need it: the characters it holds and the lines they are
// laid out over, each worked out once. Both are built from scratch every time they are asked for — the runes are a
// fresh conversion of the whole string, rewritten rune by rune for a protected node, and the line slice is synthesized
// for any control that has not been measured — so anything that needs them more than once takes one of these along
// rather than asking again.
func (o *nodeObject) content() textContent {
	runes := o.textRunes()
	return textContent{runes: runes, lines: o.lines(len(runes))}
}

// lineAt returns the index of the line that holds an offset. An offset at the very end of the text belongs to the last
// line, which is where the caret sits once everything has been typed.
func lineAt(lines []accessibility.Line, offset int) int {
	for i, line := range lines {
		if offset < line.End {
			return i
		}
	}
	return len(lines) - 1
}

// getText implements org.a11y.atspi.Text.GetText. An end offset that is negative, which is how AT-SPI says "to the end",
// or past the end is the end of the text.
func (o *nodeObject) getText(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	runes := o.textRunes()
	start := clamp(int(int32Arg(args, 0)), 0, len(runes))
	end := int(int32Arg(args, 1))
	if end < 0 || end > len(runes) {
		end = len(runes)
	}
	if end <= start {
		call.Reply("")
		return
	}
	call.Reply(string(runes[start:end]))
}

// rangePicker chooses the piece of a node's content that one of the four methods answering with a range hands back.
type rangePicker func(content *textContent, unit textUnit, offset int) textRange

// getStringAtOffset implements org.a11y.atspi.Text.GetStringAtOffset, whose second argument is an AtspiTextGranularity.
// It is the newer of the two ways to ask the same question and the one a current assistive technology uses.
func (o *nodeObject) getStringAtOffset(call *dbus.Call) {
	o.replyWithRange(call, (*textContent).rangeAt, unitForGranularity)
}

// getTextAtOffset implements org.a11y.atspi.Text.GetTextAtOffset, whose second argument is an AtspiTextBoundaryType.
func (o *nodeObject) getTextAtOffset(call *dbus.Call) {
	o.replyWithRange(call, (*textContent).rangeAt, unitForBoundary)
}

// getTextBeforeOffset implements org.a11y.atspi.Text.GetTextBeforeOffset, which answers with the unit before the one
// holding the offset rather than with that one, so that a client walking backwards through the content advances.
func (o *nodeObject) getTextBeforeOffset(call *dbus.Call) {
	o.replyWithRange(call, (*textContent).rangeBefore, unitForBoundary)
}

// getTextAfterOffset implements org.a11y.atspi.Text.GetTextAfterOffset, which answers with the unit after the one
// holding the offset.
func (o *nodeObject) getTextAfterOffset(call *dbus.Call) {
	o.replyWithRange(call, (*textContent).rangeAfter, unitForBoundary)
}

// replyWithRange answers one of the four methods that hand back a piece of the text along with where it came from.
// pick chooses the range from the content, the unit the caller asked for and the offset it asked about, and interpret
// turns the caller's second argument into that unit, since the newest of the four methods numbers the units
// differently from the three that predate it.
func (o *nodeObject) replyWithRange(call *dbus.Call, pick rangePicker, interpret func(value uint32) textUnit) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	content := o.content()
	offset := clamp(int(int32Arg(args, 0)), 0, len(content.runes))
	r := pick(&content, interpret(uint32Arg(args, 1)), offset)
	call.Reply(string(content.runes[r.start:r.end]), int32(r.start), int32(r.end))
}

// getCharacterAtOffset implements org.a11y.atspi.Text.GetCharacterAtOffset, which reports the code point rather than a
// string. An offset that is not on a character has none, which AT-SPI spells as zero.
func (o *nodeObject) getCharacterAtOffset(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	runes := o.textRunes()
	offset := int(int32Arg(args, 0))
	if offset < 0 || offset >= len(runes) {
		call.Reply(int32(0))
		return
	}
	call.Reply(runes[offset])
}

// setCaretOffset implements org.a11y.atspi.Text.SetCaretOffset. The answer is optimistic: the request has been handed to
// the user interface thread, and the caret is still where it was until the next snapshot says otherwise.
func (o *nodeObject) setCaretOffset(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	offset := int(int32Arg(args, 0))
	if offset < 0 || offset > len(o.textRunes()) {
		call.Reply(false)
		return
	}
	call.Reply(o.dispatchTextSelection(offset, offset))
}

// getNSelections implements org.a11y.atspi.Text.GetNSelections. A Unison text control has one selection at most, and a
// caret with nothing selected is no selection at all.
func (o *nodeObject) getNSelections(call *dbus.Call) {
	if _, _, exists := o.selection(); exists {
		call.Reply(int32(1))
		return
	}
	call.Reply(int32(0))
}

// getSelection implements org.a11y.atspi.Text.GetSelection. There is nothing to report for any selection but the first,
// and an empty range is how AT-SPI implementations say so.
func (o *nodeObject) getSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	start, end, exists := o.selection()
	if !exists || int32Arg(args, 0) != 0 {
		call.Reply(int32(0), int32(0))
		return
	}
	call.Reply(int32(start), int32(end))
}

// addSelection implements org.a11y.atspi.Text.AddSelection. Since there is only ever one selection, adding one works
// only while there is none; replacing the one that is there is what SetSelection is for.
func (o *nodeObject) addSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if _, _, exists := o.selection(); exists {
		call.Reply(false)
		return
	}
	call.Reply(o.dispatchTextSelection(int(int32Arg(args, 0)), int(int32Arg(args, 1))))
}

// removeSelection implements org.a11y.atspi.Text.RemoveSelection, which leaves the caret where the selection ended.
func (o *nodeObject) removeSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	_, end, exists := o.selection()
	if !exists || int32Arg(args, 0) != 0 {
		call.Reply(false)
		return
	}
	call.Reply(o.dispatchTextSelection(end, end))
}

// setSelection implements org.a11y.atspi.Text.SetSelection. Only the first selection exists, so only it can be set.
func (o *nodeObject) setSelection(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	if int32Arg(args, 0) != 0 {
		call.Reply(false)
		return
	}
	call.Reply(o.dispatchTextSelection(int(int32Arg(args, 1)), int(int32Arg(args, 2))))
}

// dispatchTextSelection asks the widget to put its caret or selection somewhere, reporting whether the request was
// accepted rather than whether it has happened. A control that does not let its selection be set says no.
func (o *nodeObject) dispatchTextSelection(start, end int) bool {
	if !o.node.Actions.Has(accessibility.SetTextSelection) {
		return false
	}
	count := len(o.textRunes())
	return o.a.dispatch(accessibility.ActionRequest{
		Node:   o.node.ID,
		Action: accessibility.SetTextSelection,
		Start:  clamp(start, 0, count),
		End:    clamp(end, 0, count),
	})
}

// getCharacterExtents implements org.a11y.atspi.Text.GetCharacterExtents. An offset that is not on a character has no
// area, which AT-SPI spells as an empty rectangle at the origin.
func (o *nodeObject) getCharacterExtents(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	content := o.content()
	offset := int(int32Arg(args, 0))
	if offset < 0 || offset >= len(content.runes) {
		call.Reply(int32(0), int32(0), int32(0), int32(0))
		return
	}
	line := content.lines[lineAt(content.lines, offset)]
	x, y, w, h := o.data.rectExtents(o.node, o.lineSpan(&line, offset, offset+1), coordArg(args, 1))
	call.Reply(x, y, w, h)
}

// getRangeExtents implements org.a11y.atspi.Text.GetRangeExtents, which is the smallest rectangle that holds every
// character of the range. A range that holds no characters has no area.
//
// The union is taken a line at a time rather than a character at a time. Asking for each character's own area in turn
// would convert the whole string to runes and rebuild the line slice once per character, which made
// GetRangeExtents(0, -1) — the call an assistive technology makes to find out where a whole document sits — quadratic
// in the length of the content, on the one goroutine that answers every other question the bus asks.
func (o *nodeObject) getRangeExtents(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	content := o.content()
	start := clamp(int(int32Arg(args, 0)), 0, len(content.runes))
	end := int(int32Arg(args, 1))
	if end < 0 || end > len(content.runes) {
		end = len(content.runes)
	}
	var union geom.Rect
	for i := range content.lines {
		union = union.Union(o.lineSpan(&content.lines[i], start, end))
	}
	if union.Empty() {
		call.Reply(int32(0), int32(0), int32(0), int32(0))
		return
	}
	x, y, w, h := o.data.rectExtents(o.node, union, coordArg(args, 2))
	call.Reply(x, y, w, h)
}

// lineSpan returns the area that the part of a rune range falling on one line occupies, in the same window-local
// logical space as the node's own Bounds. It is empty when the range does not reach the line at all and when what it
// covers there takes up no room, which a line feed does not. A line's advances are measured from the line's own left
// edge, and the line's bounds from the node's top left corner, so both have to be added to reach the space the bounds
// are in.
func (o *nodeObject) lineSpan(line *accessibility.Line, start, end int) geom.Rect {
	from := max(start, line.Start)
	to := min(end, line.End)
	if from >= to {
		return geom.Rect{}
	}
	// The advances hold the offset of every rune boundary on the line, so the leading edge of the range's first
	// character and the trailing edge of its last are two of them. A line that was never measured has none, and stands
	// in for whatever is asked about with its whole width.
	left, right := float32(0), line.Bounds.Width
	if first, last := from-line.Start, to-line.Start; first >= 0 && last < len(line.Advances) {
		left, right = line.Advances[first], line.Advances[last]
	}
	return geom.NewRect(o.node.Bounds.X+line.Bounds.X+left, o.node.Bounds.Y+line.Bounds.Y, right-left,
		line.Bounds.Height)
}

// getOffsetAtPoint implements org.a11y.atspi.Text.GetOffsetAtPoint. A point that is not on a line of this node's text
// has no offset, which AT-SPI spells as -1.
func (o *nodeObject) getOffsetAtPoint(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	// The point arrives in physical pixels, in one of the three coordinate spaces, while the lines are in logical units
	// measured from the node's own top left corner.
	pt := o.data.logicalPoint(o.node, int32Arg(args, 0), int32Arg(args, 1), coordArg(args, 2))
	pt.X -= o.node.Bounds.X
	pt.Y -= o.node.Bounds.Y
	if pt.X < 0 || pt.Y < 0 || pt.X > o.node.Bounds.Width || pt.Y > o.node.Bounds.Height {
		call.Reply(int32(-1))
		return
	}
	content := o.content()
	for i := range content.lines {
		line := &content.lines[i]
		// Only the vertical span picks the line: a point to the right of the last character of a line is still on that
		// line as far as an assistive technology hunting for the nearest character is concerned.
		if pt.Y < line.Bounds.Y || pt.Y >= line.Bounds.Bottom() {
			continue
		}
		call.Reply(int32(offsetOnLine(line, pt.X-line.Bounds.X, len(content.runes))))
		return
	}
	call.Reply(int32(-1))
}

// offsetOnLine returns the rune index of the character at a horizontal position on a line, measured from the line's own
// left edge. A position beyond either end of the line answers with the character at that end, so that a point anywhere
// within the line's area has an offset. count is how many runes the whole text holds.
func offsetOnLine(line *accessibility.Line, x float32, count int) int {
	last := min(line.End, count) - 1
	switch {
	case last < line.Start:
		// An empty line, which is what a blank line in a multi-line control is, has no character to report, but it is
		// still a place the caret can be.
		return line.Start
	case len(line.Advances) < 2:
		// Without measurements there is no telling which character is where, so the line's first one stands in for the
		// whole line.
		return line.Start
	default:
		for i := range len(line.Advances) - 1 {
			if x < line.Advances[i+1] {
				return min(line.Start+i, last)
			}
		}
		return last
	}
}

// getTextAttributes implements org.a11y.atspi.Text.GetAttributes. Unison reports no text attributes in V1, so the whole
// content is one run that has none.
func (o *nodeObject) getTextAttributes(call *dbus.Call) {
	if _, ok := callArgs(call); !ok {
		return
	}
	call.Reply(dbus.Dict{}, int32(0), int32(len(o.textRunes())))
}

// getAttributeRun implements org.a11y.atspi.Text.GetAttributeRun, which differs from GetAttributes only in offering to
// fold in the defaults. There are no attributes either way.
func (o *nodeObject) getAttributeRun(call *dbus.Call) {
	o.getTextAttributes(call)
}

// getAttributeValue implements org.a11y.atspi.Text.GetAttributeValue. No attribute has a value, which AT-SPI spells as
// an empty string.
func (o *nodeObject) getAttributeValue(call *dbus.Call) {
	if _, ok := callArgs(call); !ok {
		return
	}
	call.Reply("")
}

// getDefaultAttributes implements org.a11y.atspi.Text.GetDefaultAttributes. A Unison text control has one font and one
// color throughout, and neither is anything an assistive technology needs, so there are none to report.
func (o *nodeObject) getDefaultAttributes(call *dbus.Call) {
	call.Reply(dbus.Dict{})
}

// rectExtents converts a rectangle in the same window-local logical space as a node's Bounds into the physical pixels of
// the given coordinate space, which is what [windowData.extents] does for the node's own bounds.
func (d *windowData) rectExtents(n *accessibility.Node, r geom.Rect, coord CoordType) (x, y, w, h int32) {
	origin := d.originFor(n, coord)
	scale := d.geometry.effectiveScale()
	return pixels(origin.X + r.X*scale.X), pixels(origin.Y + r.Y*scale.Y), pixels(r.Width * scale.X),
		pixels(r.Height * scale.Y)
}

// clamp returns v held within the given bounds.
func clamp(v, low, high int) int {
	return min(max(v, low), high)
}
