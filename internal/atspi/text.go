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

// textInterface returns the org.a11y.atspi.Text interface of a node that holds navigable text, which is how an
// assistive technology reads a control a character, a word, a line or a selection at a time rather than as one string.
//
// A node whose value is textual rather than numeric gets the same interface synthesized from that value; see
// [textualValue]. What it hands over is read-only: the characters, their count and where they are, with no caret to
// move and no selection to set, since the value is a description of what the control shows rather than content the user
// is inside. The four methods that would change either therefore refuse, which [nodeObject.dispatchTextSelection]
// answers for all of them at once.
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

// textRunes returns the characters of the node's text, which are those of its textual value for a node that carries no
// text of its own. There is nothing to hide here: a password field carries neither text nor value in a published
// snapshot — [accessibility.Node] says so, and the snapshot builder returns before filling either in — so it is never
// given the org.a11y.atspi.Text interface in the first place, and reports ATSPI_ROLE_PASSWORD_TEXT and no character
// count rather than a string of bullets.
func (o *nodeObject) textRunes() []rune {
	if o.node.Text == nil {
		return []rune(textualValue(o.node))
	}
	return []rune(o.node.Text.Text)
}

// caret returns the rune index the caret sits at, which is one end of the selection but not always the end: extending a
// selection backwards with shift+Left or shift+Home leaves the caret at its start, and a client told otherwise puts its
// review cursor at the wrong end of what it has just read out.
//
// Text synthesized from a value has no caret in it at all, and reports the start of the content, which is what an
// implementation with no caret to report hands back and where a client reading the whole of a short value would put its
// review cursor anyway. The other convention, -1, needs a client that expects a negative offset.
func (o *nodeObject) caret() int {
	if o.node.Text == nil {
		return 0
	}
	return o.node.Text.Caret
}

// selection returns the selected range of runes, and whether anything is selected at all.
func (o *nodeObject) selection() (start, end int, exists bool) {
	info := o.node.Text
	if info == nil || info.SelStart == info.SelEnd {
		return 0, 0, false
	}
	return info.SelStart, info.SelEnd, true
}

// lines returns the laid-out lines of the node's text, and whether they are the ones the snapshot measured. A snapshot
// only measures the lines of the control that has the focus, since the measurements are too expensive to take for every
// text control in a window; when there are none, one line filling the node stands in for them, which is the best that
// can be said about where the characters of an unfocused field are. count is how many runes the text holds.
//
// That made-up line says where things are, not how the text divides: see [textContent.divisionsFor], which divides an
// unmeasured control at its own line feeds rather than pretending the whole of it is one line.
func (o *nodeObject) lines(count int) (lines []accessibility.Line, measured bool) {
	if info := o.node.Text; info != nil && len(info.Lines) != 0 {
		return info.Lines, true
	}
	return []accessibility.Line{
		{Start: 0, End: count, Bounds: geom.NewRect(0, 0, o.node.Bounds.Width, o.node.Bounds.Height)},
	}, false
}

// content returns the node's text as the questions about it need it: the characters it holds and the lines they are
// laid out over, each worked out once. Both are built from scratch every time they are asked for — the runes are a
// fresh conversion of the whole string, and the line slice is synthesized for any control that has not been measured —
// so anything that needs them more than once takes one of these along rather than asking again.
func (o *nodeObject) content() textContent {
	runes := o.textRunes()
	lines, measured := o.lines(len(runes))
	return textContent{runes: runes, lines: lines, measured: measured}
}

// lineAt returns the index of the line that holds an offset. An offset at the very end of the text belongs to the last
// line, which is where the caret sits once everything has been typed. A node with no lines at all has no line to answer
// with, which it says with -1, so a caller that indexes the slice with this has to look.
func lineAt(lines []accessibility.Line, offset int) int {
	for i, line := range lines {
		if offset < line.End {
			return i
		}
	}
	return len(lines) - 1
}

// getText implements org.a11y.atspi.Text.GetText. An end offset that is negative, which is how AT-SPI says "to the
// end", or past the end is the end of the text.
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
type rangePicker func(content *textContent, unit textUnit, form textForm, offset int) textRange

// getStringAtOffset implements org.a11y.atspi.Text.GetStringAtOffset, whose second argument is an AtspiTextGranularity.
// It is the newer of the two ways to ask the same question and the one a current assistive technology uses.
func (o *nodeObject) getStringAtOffset(call *dbus.Call) {
	o.replyWithRange(call, (*textContent).rangeAt, granularityUnit)
}

// granularityUnit reads the second argument of GetStringAtOffset, which names a unit but no form: the granularities
// divide text the way the START boundaries do.
func granularityUnit(value uint32) (unit textUnit, form textForm) {
	return unitForGranularity(value), formStart
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
// pick chooses the range from the content, the unit and form the caller asked for and the offset it asked about, and
// interpret turns the caller's second argument into that unit and form, since the newest of the four methods numbers
// the units differently from the three that predate it and asks for only one of the two forms.
func (o *nodeObject) replyWithRange(call *dbus.Call, pick rangePicker,
	interpret func(value uint32) (unit textUnit, form textForm),
) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	content := o.content()
	offset := clamp(int(int32Arg(args, 0)), 0, len(content.runes))
	unit, form := interpret(uint32Arg(args, 1))
	r := pick(&content, unit, form, offset)
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

// setCaretOffset implements org.a11y.atspi.Text.SetCaretOffset. An offset the content does not reach is refused rather
// than brought into it, since a client told that the caret went where it asked would then have to be told, by the next
// snapshot, that it went somewhere else. The answer is otherwise optimistic: the request has been handed to the user
// interface thread, and the caret is still where it was until the next snapshot says otherwise.
func (o *nodeObject) setCaretOffset(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	offset := int(int32Arg(args, 0))
	call.Reply(o.selectRange(offset, offset))
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
	call.Reply(o.selectRange(int(int32Arg(args, 0)), int(int32Arg(args, 1))))
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
	call.Reply(o.selectRange(int(int32Arg(args, 1)), int(int32Arg(args, 2))))
}

// selectRange asks the widget to select a range of its text, refusing a range that is reversed or that reaches outside
// the content rather than bringing it within one. A caller told that the selection it asked for was made and then shown
// something else by the next snapshot is worse off than one told no: AddSelection(3, 1) on a two-rune field would
// otherwise become a collapsed caret and still be reported as a selection. It is the same refusal
// [nodeObject.setCaretOffset] makes, which is why that method goes through here too.
func (o *nodeObject) selectRange(start, end int) bool {
	if start < 0 || end < start || end > len(o.textRunes()) {
		return false
	}
	return o.dispatchTextSelection(start, end)
}

// dispatchTextSelection asks the widget to put its caret or selection somewhere, reporting whether the request was
// accepted rather than whether it has happened. A control that does not let its selection be set says no, and so does
// one whose text is synthesized from its value: there is no caret in a value to move, and nothing here could change
// what the widget shows anyway. The range is brought within the content as a last defense, since every caller but
// [nodeObject.removeSelection] — which passes an end of the selection the node itself reported — has already refused
// anything outside it.
func (o *nodeObject) dispatchTextSelection(start, end int) bool {
	if o.node.Text == nil || !o.node.Actions.Has(accessibility.SetTextSelection) {
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
	index := lineAt(content.lines, offset)
	if offset < 0 || offset >= len(content.runes) || index < 0 {
		call.Reply(int32(0), int32(0), int32(0), int32(0))
		return
	}
	span, ok := o.lineSpan(&content.lines[index], offset, offset+1)
	if !ok {
		call.Reply(int32(0), int32(0), int32(0), int32(0))
		return
	}
	x, y, w, h := o.data.rectExtents(o.node, span, coordArg(args, 1))
	call.Reply(x, y, w, h)
}

// getRangeExtents implements org.a11y.atspi.Text.GetRangeExtents, which is the smallest rectangle that holds every
// character of the range. A range that holds no characters at all has no area and no place either.
//
// The union is taken a line at a time rather than a character at a time. Asking for each character's own area in turn
// would convert the whole string to runes and rebuild the line slice once per character, which made
// GetRangeExtents(0, -1) — the call an assistive technology makes to find out where a whole document sits — quadratic
// in the length of the content, on the one goroutine that answers every other question the bus asks.
//
// The union is taken edge by edge rather than with [geom.Rect.Union], which throws away a rectangle with no width or no
// height. A range of characters that take up no room — a line feed, or the blank line between two paragraphs — is still
// somewhere, and answering with the empty rectangle at the origin sends a client's review cursor to the top left corner
// of the screen instead of to the blank line the user is on.
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
	found := false
	for i := range content.lines {
		span, onLine := o.lineSpan(&content.lines[i], start, end)
		if !onLine {
			continue
		}
		if !found {
			union, found = span, true
			continue
		}
		union = spanning(union, span)
	}
	if !found {
		call.Reply(int32(0), int32(0), int32(0), int32(0))
		return
	}
	x, y, w, h := o.data.rectExtents(o.node, union, coordArg(args, 2))
	call.Reply(x, y, w, h)
}

// spanning returns the smallest rectangle holding two others, keeping the place of one that has no width or no height
// rather than discarding it as [geom.Rect.Union] does.
func spanning(a, b geom.Rect) geom.Rect {
	x := min(a.X, b.X)
	y := min(a.Y, b.Y)
	return geom.NewRect(x, y, max(a.Right(), b.Right())-x, max(a.Bottom(), b.Bottom())-y)
}

// lineSpan returns the area that the part of a rune range falling on one line occupies, in the same window-local
// logical space as the node's own Bounds. ok is false when the range does not reach the line at all; a range that does
// reach it but takes up no room there, which is what a line feed does, has a place and no width. A line's advances are
// measured from the line's own left edge, and the line's bounds from the node's top left corner, so both have to be
// added to reach the space the bounds are in.
func (o *nodeObject) lineSpan(line *accessibility.Line, start, end int) (span geom.Rect, ok bool) {
	from := max(start, line.Start)
	to := min(end, line.End)
	if from >= to {
		return geom.Rect{}, false
	}
	// The advances hold the offset of every rune boundary on the line, so the leading edge of the range's first
	// character and the trailing edge of its last are two of them. A line that was never measured has none, and stands
	// in for whatever is asked about with its whole width.
	left, right := float32(0), line.Bounds.Width
	if first, last := from-line.Start, to-line.Start; first >= 0 && last < len(line.Advances) {
		left, right = line.Advances[first], line.Advances[last]
	}
	return geom.NewRect(o.node.Bounds.X+line.Bounds.X+left, o.node.Bounds.Y+line.Bounds.Y, right-left,
		line.Bounds.Height), true
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
// content is one run that has none — but only for an offset the content actually has: ATK answers an offset outside the
// text with an empty run, and a client that walks a control by asking for the run at the end of the last one would
// otherwise be handed the whole content again and walk it forever.
func (o *nodeObject) getTextAttributes(call *dbus.Call) {
	args, ok := callArgs(call)
	if !ok {
		return
	}
	count := len(o.textRunes())
	if offset := int(int32Arg(args, 0)); offset < 0 || offset >= count {
		call.Reply(dbus.Dict{}, int32(0), int32(0))
		return
	}
	call.Reply(dbus.Dict{}, int32(0), int32(count))
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

// rectExtents converts a rectangle in the same window-local logical space as a node's Bounds into the physical pixels
// of the given coordinate space, which is what [windowData.extents] does for the node's own bounds.
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
