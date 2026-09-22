// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// These tests ask the text controls what they tell an assistive technology about their content, caret and layout, and
// then ask them to change it the way a screen reader would. A session owns most of the package's mutable globals while
// it runs, so none of these may call t.Parallel.

// axEventsOfKind returns the events of one kind that name a particular node.
func axEventsOfKind(events []accessibility.Event, kind accessibility.EventKind,
	node accessibility.NodeID,
) []accessibility.Event {
	var found []accessibility.Event
	for _, event := range events {
		if event.Kind == kind && event.Node == node {
			found = append(found, event)
		}
	}
	return found
}

// TestFieldAccessibility verifies what a field says about itself: its content is its value along with the lines and
// the style it was drawn in, its watermark is the prompt an empty one shows, a field that fails validation says so,
// and a field that obscures what it holds gives up nothing.
func TestFieldAccessibility(t *testing.T) {
	c := check.New(t)
	var plain, watermarked, invalid, secret, multiLine *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
			// The window has to be the active one for the fields to offer the contextual menu they could only show
			// there, and a window becoming active hands the focus to the first thing in it that can take it — which
			// selects the whole of a field's content. This button is that first thing, so every field below is one
			// nobody is working in, which is what the caret and the unmeasured lines asserted further down are about.
			first := unison.NewButton()
			first.SetTitle("Elsewhere")

			plain = unison.NewField()
			plain.SetText("Hello")

			watermarked = unison.NewField()
			watermarked.Watermark = "Search"

			invalid = unison.NewField()
			invalid.ValidateCallback = func() bool { return false }
			invalid.SetText("nope")

			secret = unison.NewField()
			secret.ObscurementRune = '•'
			secret.SetText("hunter2")

			multiLine = unison.NewMultiLineField()
			multiLine.SetText("one\ntwo")

			wnd = newHeadlessWindow(t, "fields", geom.NewRect(10, 10, 500, 500),
				axColumn(first, plain, watermarked, invalid, secret, multiLine))
		}))
	c.NotNil(wnd)
	// Showing a window does not give it the focus, and a field in a window that is not the active one is published
	// without the contextual menu it could not show. See axMayPopupMenu and axSnapshot.narrowMenuActions.
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(plain))
	c.Equal(role.TextField, node.Role)
	c.Equal("Hello", node.Value, "a field's content is its value")
	c.False(node.Protected)
	c.False(node.Invalid)
	c.True(node.Focusable)
	c.True(node.Actions.Has(accessibility.Focus))
	c.True(node.Actions.Has(accessibility.SetValue))
	c.True(node.Actions.Has(accessibility.SetTextSelection))
	c.True(node.Actions.Has(accessibility.ReplaceText))
	c.True(node.Actions.Has(accessibility.ShowContextMenu))
	c.False(node.Actions.Has(accessibility.Press), "clicking a field does no more than move its caret")
	c.True(node.Actions.Has(accessibility.ScrollRangeIntoView), "a reading cursor can bring what it reached into view")
	c.True(node.Text != nil)
	if node.Text != nil {
		c.Equal("Hello", node.Text.Text)
		c.Equal(5, node.Text.SelStart, "setting the text left the caret at the end of it")
		c.Equal(5, node.Text.SelEnd)
		c.False(node.Text.Multiline)
		c.Equal(1, len(node.Text.Lines), "a field is read by line whether or not anyone is working in it")
		if len(node.Text.Lines) == 1 {
			line := node.Text.Lines[0]
			c.Equal(0, line.Start)
			c.Equal(5, line.End)
			c.Equal(6, len(line.Advances), "one offset per rune boundary")
			axCheckLine(c, line)
			c.True(line.Bounds.Height > 0)
		}
		c.Equal(1, len(node.Text.Runs), "a field draws the whole of its content in one font")
		if len(node.Text.Runs) == 1 {
			run := node.Text.Runs[0]
			c.Equal(0, run.Start)
			c.Equal(5, run.End)
			c.Equal(unison.FieldFont.Descriptor().Family, run.Family)
		}
	}

	watermarkNode := axMustNode(c, screen.AccessibilityNodeFor(watermarked))
	c.Equal("Search", watermarkNode.Placeholder, "the watermark is what an empty field prompts with")
	c.Equal("", watermarkNode.Value)

	invalidNode := axMustNode(c, screen.AccessibilityNodeFor(invalid))
	c.True(invalidNode.Invalid, "a field whose content was rejected says so")

	secretNode := axMustNode(c, screen.AccessibilityNodeFor(secret))
	c.Equal(role.TextField, secretNode.Role)
	c.True(secretNode.Protected)
	c.Equal("", secretNode.Value, "a password is never handed out")
	c.True(secretNode.Text == nil, "not even the caret within a password is reported")
	c.False(secretNode.Actions.Has(accessibility.ScrollRangeIntoView),
		"and nothing may ask for a range of it to be scrolled to, since no range of it was ever handed out")

	multiNode := axMustNode(c, screen.AccessibilityNodeFor(multiLine))
	c.Equal(role.TextArea, multiNode.Role, "a field that accepts line feeds is a text area")
	c.True(multiNode.Text != nil)
	if multiNode.Text != nil {
		c.True(multiNode.Text.Multiline)
		c.Equal("one\ntwo", multiNode.Text.Text)
		c.Equal(2, len(multiNode.Text.Lines), "both of the lines are reported")
		if len(multiNode.Text.Lines) == 2 {
			c.True(multiNode.Text.Lines[1].Bounds.Y > multiNode.Text.Lines[0].Bounds.Y,
				"the second line sits below the first")
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityActions verifies that an assistive technology can replace a field's value, move its caret and
// selection, and replace a range of its content in place.
func TestFieldAccessibilityActions(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("hello world")
			wnd = newHeadlessWindow(t, "field actions", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  2,
		End:    5,
	}))
	var start, end int
	var selected string
	screen.Do(func() {
		start, end = field.Selection()
		selected = field.SelectedText()
	})
	c.Equal(2, start)
	c.Equal(5, end)
	c.Equal("llo", selected)
	screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(field))
	if node.Text != nil {
		c.Equal(2, node.Text.SelStart, "the selection an assistive technology set is what it reads back")
		c.Equal(5, node.Text.SelEnd)
	}

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ReplaceText,
		Start:  6,
		End:    11,
		Value:  "there",
	}))
	var text string
	screen.Do(func() {
		text = field.Text()
		start, end = field.Selection()
	})
	c.Equal("hello there", text, "replacing a range should have left the rest of the content alone")
	c.Equal(11, start, "the caret should sit just past what was inserted")
	c.Equal(11, end)

	// A replacement is an ordinary modification, so it can be undone exactly as a paste can.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Value:  "replaced outright",
	}))
	screen.Do(func() { text = field.Text() })
	c.Equal("replaced outright", text)

	// Asking for the contextual menu shows the one a right-click would have, which appears within the window here.
	before := axRootChildCount(screen.AccessibilityTree(wnd))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)),
		"the field's contextual menu should have opened within the window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldPasteReplacesRunes verifies that pasting, which now shares its work with the replacement an assistive
// technology asks for, still does exactly what it did: it replaces the selection, or inserts at the caret when there is
// no selection, reports the modification, and deletes the selection when there is nothing to paste.
func TestFieldPasteReplacesRunes(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t, unison.HeadlessConfig{Width: 200, Height: 200})
	var replaced, inserted, emptied, before, after string
	var start, end int
	c.True(screen.Do(func() {
		unison.ClipboardSetText("there")

		field := unison.NewField()
		field.SetText("hello world")
		field.SetSelection(6, 11)
		field.ModifiedCallback = func(was, is *unison.FieldState) {
			before = was.Text
			after = is.Text
		}
		field.Paste()
		replaced = field.Text()
		start, end = field.Selection()

		field = unison.NewField()
		field.SetText("ac")
		field.SetSelectionTo(1)
		field.Paste()
		inserted = field.Text()

		unison.ClipboardSetText("")
		field = unison.NewField()
		field.SetText("hello")
		field.SetSelection(1, 3)
		field.Paste()
		emptied = field.Text()
	}))
	c.Equal("hello there", replaced, "pasting over a selection should have replaced it")
	c.Equal(11, start, "the caret should sit just past what was pasted")
	c.Equal(11, end)
	c.Equal("hello world", before, "the modification should have been reported with the state before it")
	c.Equal("hello there", after)
	c.Equal("atherec", inserted, "pasting without a selection should have inserted at the caret")
	c.Equal("hlo", emptied, "pasting nothing over a selection should have deleted it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityTextEvents verifies that typing into a field, deleting from it and moving its selection are all
// reported as the text events an assistive technology listens for.
func TestFieldAccessibilityTextEvents(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			wnd = newHeadlessWindow(t, "field events", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		field.SetText("ab")
		field.RequestFocus()
		field.SetSelectionToEnd()
	}))

	// The description published here is the one the events that follow are measured against.
	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	screen.AccessibilityEvents(wnd)

	screen.Type("c")
	screen.AccessibilityTree(wnd)
	events := screen.AccessibilityEvents(wnd)
	inserted := axEventsOfKind(events, accessibility.TextInserted, node.ID)
	c.Equal(1, len(inserted), "typing a rune should have been reported as one insertion: %v", events)
	if len(inserted) == 1 {
		c.Equal(2, inserted[0].Start)
		c.Equal(1, inserted[0].Length)
		c.Equal("c", inserted[0].New)
	}
	c.True(len(axEventsOfKind(events, accessibility.TextSelectionChanged, node.ID)) != 0,
		"the caret moved past what was typed: %v", events)

	screen.KeyPress(unison.KeyBackspace, mod.None)
	screen.AccessibilityTree(wnd)
	events = screen.AccessibilityEvents(wnd)
	deleted := axEventsOfKind(events, accessibility.TextDeleted, node.ID)
	c.Equal(1, len(deleted), "a backspace should have been reported as one deletion: %v", events)
	if len(deleted) == 1 {
		c.Equal(2, deleted[0].Start)
		c.Equal(1, deleted[0].Length)
		c.Equal("c", deleted[0].Old)
	}

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetTextSelection,
		Start:  0,
		End:    2,
	}))
	screen.AccessibilityTree(wnd)
	events = screen.AccessibilityEvents(wnd)
	selection := axEventsOfKind(events, accessibility.TextSelectionChanged, node.ID)
	c.Equal(1, len(selection), "moving the selection should have been reported once: %v", events)
	if len(selection) == 1 {
		c.Equal(0, selection[0].Start)
		c.Equal(2, selection[0].Length)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityLines verifies that the laid-out lines of a field are reported whether or not it holds the
// focus, that every rune boundary on a line is accounted for, and that the lines of a text area are stacked down the
// field in the order they are read in.
func TestFieldAccessibilityLines(t *testing.T) {
	c := check.New(t)
	var single, empty, area *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 700, Height: 600},
		unison.StartupFinishedCallback(func() {
			single = unison.NewField()
			single.SetText("Hello")

			empty = unison.NewField()

			area = unison.NewMultiLineField()
			area.SetText("one\ntwo\nthree")

			wnd = newHeadlessWindow(t, "field lines", geom.NewRect(10, 10, 600, 500),
				axColumn(single, empty, area))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	// Showing the window hands the focus to the first field within it, so the text area is the one nothing is working
	// in.
	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(area))
	c.True(node.Text != nil)
	if node.Text != nil {
		c.Equal(3, len(node.Text.Lines), "a field nobody is working in is read by line too")
	}

	c.True(screen.Do(func() {
		single.RequestFocus()
		single.SetSelectionTo(1)
	}))
	screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(single))
	c.True(node.Text != nil)
	if node.Text != nil {
		c.Equal(1, len(node.Text.Lines), "the field holds one line of text")
		if len(node.Text.Lines) == 1 {
			line := node.Text.Lines[0]
			c.Equal(0, line.Start)
			c.Equal(5, line.End)
			axCheckLine(c, line)
			c.True(line.Bounds.Width > 0)
			c.True(line.Bounds.Height > 0)
		}
	}

	c.True(screen.Do(func() { empty.RequestFocus() }))
	screen.AccessibilityTree(wnd)
	emptyNode := axMustNode(c, screen.AccessibilityNodeFor(empty))
	c.True(emptyNode.Text != nil)
	if emptyNode.Text != nil {
		c.Equal(1, len(emptyNode.Text.Lines), "an empty field still has the line its caret sits on")
		if len(emptyNode.Text.Lines) == 1 {
			line := emptyNode.Text.Lines[0]
			c.Equal(0, line.Start)
			c.Equal(0, line.End)
			c.Equal(1, len(line.Advances))
			c.True(line.Bounds.Height > 0)
		}
	}

	c.True(screen.Do(func() {
		area.RequestFocus()
		area.SetSelectionToStart()
	}))
	screen.AccessibilityTree(wnd)
	areaNode := axMustNode(c, screen.AccessibilityNodeFor(area))
	c.True(areaNode.Text != nil)
	if areaNode.Text != nil {
		c.True(areaNode.Text.Multiline)
		lines := areaNode.Text.Lines
		c.Equal(3, len(lines), "each of the three paragraphs is a line")
		if len(lines) == 3 {
			c.Equal(0, lines[0].Start)
			c.Equal(13, lines[len(lines)-1].End, "the last line ends where the content does")
			for i, line := range lines {
				axCheckLine(c, line)
				if i != 0 {
					c.Equal(lines[i-1].End, line.Start, "the lines together cover the whole content")
					c.True(line.Bounds.Y >= lines[i-1].Bounds.Bottom(),
						"each line sits below the one before it")
				}
			}
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityLinesFollowTheText verifies that the cached measurements a field hands out are rebuilt when
// what they measured changes — the content and the font — and that a tree already published goes on describing the
// text as it was when it was built.
func TestFieldAccessibilityLinesFollowTheText(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("Hi")
			wnd = newHeadlessWindow(t, "field text changes", geom.NewRect(10, 10, 500, 200), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	first := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(first.Text != nil)
	if first.Text == nil || len(first.Text.Lines) != 1 {
		c.Fatal("the field should have been described with the one line it drew")
	}
	before := first.Text.Lines[0]
	beforeAdvances := slices.Clone(before.Advances)

	c.True(screen.Do(func() { field.SetText("Hi there") }))
	screen.AccessibilityTree(wnd)
	second := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(second.Text != nil)
	if second.Text != nil && len(second.Text.Lines) == 1 {
		line := second.Text.Lines[0]
		c.Equal(8, line.End, "the line covers the new content")
		c.Equal(9, len(line.Advances))
		axCheckLine(c, line)
		c.True(line.Advances[len(line.Advances)-1] > before.Advances[len(before.Advances)-1],
			"the longer text reaches further across the field")
	}
	c.True(slices.Equal(beforeAdvances, before.Advances),
		"a tree already handed out describes the text as it was, so its measurements are never written over")

	c.True(screen.Do(func() {
		field.Font = unison.SystemFont.Face().Font(unison.LabelFont.Size() * 3)
		field.MarkForLayoutAndRedraw()
	}))
	screen.AccessibilityTree(wnd)
	third := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(third.Text != nil)
	if third.Text != nil {
		c.Equal(1, len(third.Text.Runs), "the whole of the content is drawn in the one font the field holds")
		if len(third.Text.Runs) == 1 {
			c.Equal(unison.SystemFont.Descriptor().Family, third.Text.Runs[0].Family,
				"the run says what the field is now drawn in")
			c.Equal(8, third.Text.Runs[0].End)
		}
		if len(third.Text.Lines) == 1 && len(second.Text.Lines) == 1 {
			c.True(third.Text.Lines[0].Advances[8] > second.Text.Lines[0].Advances[8],
				"the larger font pushed every rune boundary further across")
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityLinesFollowAResize verifies that the cached measurements a field hands out are dropped when
// the width it wrapped to changes. The content and the font are noticed by the line cache being rebuilt outright; a
// width change is noticed only by the lines coming back as a freshly allocated slice, which is the whole of the
// argument at Field.prepareLines, and a resize that rewraps the same text into the same number of lines is what that
// rests on. A stale cache shows up as advances that no longer reach as far as the line they are reported beside: the
// bounds are worked out afresh on every call from the lines the field actually laid out, while the advances would
// still be the ones measured over the wrap that was thrown away.
func TestFieldAccessibilityLinesFollowAResize(t *testing.T) {
	c := check.New(t)
	const content = "aaaaaaaaaa bb cccccc"
	var field *unison.Field
	var wnd *unison.Window
	var em float32
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 400},
		unison.StartupFinishedCallback(func() {
			field = unison.NewMultiLineField()
			// Monospaced, so that a width stated in characters is the width the text actually takes: the wrap has to
			// land where the test says it does for the same number of lines to come back from both widths.
			field.Font = unison.MonospacedFont
			field.SetWrap(true)
			field.SetText(content)
			em = unison.NewText("a", &unison.TextDecoration{Font: field.Font}).Width()
			wnd = newHeadlessWindow(t, "field resize", geom.NewRect(10, 10, 500, 300), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(em > 0)

	// The wrap width a field uses is its content width less two, so the hint is that much wider than the number of
	// characters each width is meant to hold.
	setWidth := func(chars float32) {
		c.True(screen.Do(func() {
			insets := geom.Size{}
			if border := field.Border(); border != nil {
				insets = border.Insets().Size()
			}
			field.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(chars*em+2+insets.Width, 0),
				HAlign:   align.Start,
				VAlign:   align.Start,
			})
			field.MarkForLayoutRecursivelyUpward()
			field.MarkForRedraw()
		}))
	}

	// Wide enough for "aaaaaaaaaa bb" but not for the whole of it, so it wraps as "aaaaaaaaaa bb" / "cccccc".
	setWidth(13.5)
	screen.AccessibilityTree(wnd)
	wide := axMustNode(c, screen.AccessibilityNodeFor(field))
	axCheckWrappedLines(c, wide, content, 2)

	// Wide enough for "aaaaaaaaaa" and for "bb cccccc", but not for the two together, so the same text wraps into the
	// same number of lines split somewhere else entirely.
	setWidth(10.5)
	screen.AccessibilityTree(wnd)
	narrow := axMustNode(c, screen.AccessibilityNodeFor(field))
	axCheckWrappedLines(c, narrow, content, 2)
	if wide.Text != nil && narrow.Text != nil && len(wide.Text.Lines) == 2 && len(narrow.Text.Lines) == 2 {
		c.True(narrow.Text.Lines[0].End < wide.Text.Lines[0].End,
			"the narrower field broke the text earlier: %d then %d", wide.Text.Lines[0].End,
			narrow.Text.Lines[0].End)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axCheckWrappedLines verifies that what a wrapping field reports about its lines was all measured over the same wrap:
// the lines cover the whole of the content in order, and the advances on each of them reach exactly as far as the
// bounds that line was drawn at. Advances that were measured over a wrap the field has since thrown away fall short of,
// or overrun, the line they are reported beside.
func axCheckWrappedLines(c check.Checker, node *accessibility.Node, content string, expected int) {
	c.True(node.Text != nil)
	if node.Text == nil {
		return
	}
	c.Equal(content, node.Text.Text)
	c.Equal(expected, len(node.Text.Lines), "the text wrapped into the number of lines the width called for")
	if len(node.Text.Lines) != expected {
		return
	}
	for i, line := range node.Text.Lines {
		axCheckLine(c, line)
		if i != 0 {
			c.True(line.Start >= node.Text.Lines[i-1].End, "the lines run through the content in order")
		}
		c.True(xmath.Abs(line.Advances[len(line.Advances)-1]-line.Bounds.Width) < 0.5,
			"line %d was measured over the wrap it was drawn with: advances reach %v, the line is %v wide", i,
			line.Advances[len(line.Advances)-1], line.Bounds.Width)
	}
	c.Equal(len([]rune(content)), node.Text.Lines[len(node.Text.Lines)-1].End,
		"the last line ends where the content does")
}

// TestFieldAccessibilityScrollRangeIntoView verifies that a field brings a range of its own content into view when a
// screen reader's reading cursor reaches text that cannot be seen, scrolling itself and then asking whatever it sits
// in to scroll the rest of the way.
func TestFieldAccessibilityScrollRangeIntoView(t *testing.T) {
	c := check.New(t)
	var narrow, area, secret *unison.Field
	var scroller *unison.ScrollPanel
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 500},
		unison.StartupFinishedCallback(func() {
			narrow = unison.NewField()
			narrow.SetText("a considerably longer piece of text than this field can show at once")
			narrow.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(80, 0),
				HAlign:   align.Start,
				VAlign:   align.Start,
			})

			secret = unison.NewField()
			secret.ObscurementRune = '\u2022'
			secret.SetText("a considerably longer piece of text than this field can show at once")
			secret.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(80, 0),
				HAlign:   align.Start,
				VAlign:   align.Start,
			})

			area = unison.NewMultiLineField()
			area.SetText(strings.Join([]string{
				"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten",
			}, "\n"))
			scroller = unison.NewScrollPanel()
			scroller.SetContent(area, behavior.Fill, behavior.Unmodified)
			scroller.SetLayoutData(&unison.FlexLayoutData{
				SizeHint: geom.NewSize(200, 60),
				HAlign:   align.Fill,
				VAlign:   align.Start,
			})

			wnd = newHeadlessWindow(t, "field scrolling", geom.NewRect(10, 10, 400, 300),
				axColumn(narrow, secret, scroller))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(narrow))
	c.True(node.Actions.Has(accessibility.ScrollRangeIntoView))
	var offset geom.Point
	c.True(screen.Do(func() {
		narrow.SetScrollOffset(geom.Point{})
		offset = narrow.ScrollOffset()
	}))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ScrollRangeIntoView,
		Start:  60,
		End:    68,
	}))
	var moved geom.Point
	screen.Do(func() { moved = narrow.ScrollOffset() })
	c.True(moved.X < offset.X, "the end of the content was brought into view by scrolling the field itself")

	areaNode := axMustNode(c, screen.AccessibilityNodeFor(area))
	var position float32
	screen.Do(func() { position = area.FrameRect().Y })
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   areaNode.ID,
		Action: accessibility.ScrollRangeIntoView,
		Start:  45,
		End:    48,
	}))
	var scrolled float32
	screen.Do(func() { scrolled = area.FrameRect().Y })
	c.True(scrolled < position, "the last line of the text area was scrolled into the view holding it")

	// A field that obscures what it shows publishes neither its text nor this action, so it must not carry it out
	// either: an adapter that trusts what a node advertises would otherwise be told the field cannot do something it
	// quietly does, over a range of a string that was never handed out. The field is asked directly, since the request
	// is refused before it ever reaches a widget when the node does not offer the action.
	secretNode := axMustNode(c, screen.AccessibilityNodeFor(secret))
	c.False(secretNode.Actions.Has(accessibility.ScrollRangeIntoView))
	var handled bool
	var secretBefore, secretAfter geom.Point
	c.True(screen.Do(func() {
		secret.SetScrollOffset(geom.Point{})
		secretBefore = secret.ScrollOffset()
		handled = secret.PerformAccessibilityAction(accessibility.ActionRequest{
			Action: accessibility.ScrollRangeIntoView,
			Start:  60,
			End:    68,
		})
		secretAfter = secret.ScrollOffset()
	}))
	c.False(handled, "a protected field refuses the action it never offered")
	c.Equal(secretBefore, secretAfter, "and it scrolls nowhere")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axCheckLine verifies the invariants of one measured line: there is an offset for every rune boundary on it, the
// offsets run left to right, and the first of them is the start of the line.
func axCheckLine(c check.Checker, line accessibility.Line) {
	c.Equal((line.End-line.Start)+1, len(line.Advances),
		"there is one offset per rune on the line, plus the end of the last one")
	if len(line.Advances) == 0 {
		return
	}
	c.Equal(float32(0), line.Advances[0], "the first offset is the start of the line")
	for i := 1; i < len(line.Advances); i++ {
		c.True(line.Advances[i] >= line.Advances[i-1], "the offsets run left to right")
	}
}

// TestNumericFieldAccessibility verifies that a numeric field reports the range it is confined to and that stepping it
// stops at either end of that range.
func TestNumericFieldAccessibility(t *testing.T) {
	c := check.New(t)
	var field *unison.NumericField[int]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			label := unison.NewLabel()
			label.SetTitle("Count:")
			field = unison.NewNumericField(5, 0, 7, strconv.Itoa, strconv.Atoi, nil)
			wnd = newHeadlessWindow(t, "numeric", geom.NewRect(10, 10, 300, 150), axColumn(label, field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.Equal(role.SpinButton, node.Role)
	c.Equal("Count", node.Name, "the label before it names it, minus the colon")
	c.True(node.HasNumber)
	c.Equal(float64(5), node.Number)
	c.Equal(float64(0), node.Min)
	c.Equal(float64(7), node.Max)
	c.Equal(float64(1), node.Step)
	c.Equal("5", node.Value, "it is still a text control, so its text is its value")
	c.True(node.Text != nil)
	c.True(node.Actions.Has(accessibility.Increment))
	c.True(node.Actions.Has(accessibility.Decrement))
	c.True(node.Actions.Has(accessibility.SetValue), "the actions a field offers are still offered")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Increment,
	}))
	var value int
	screen.Do(func() { value = field.Value() })
	c.Equal(6, value, "incrementing should have raised the value by one")

	// Two more increments reach the top of the range, where it stays.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Increment,
	}))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Increment,
	}))
	screen.Do(func() { value = field.Value() })
	c.Equal(7, value, "incrementing past the maximum should have stopped at it")

	for range 9 {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   node.ID,
			Action: accessibility.Decrement,
		}))
	}
	screen.Do(func() { value = field.Value() })
	c.Equal(0, value, "decrementing past the minimum should have stopped at it")

	screen.AccessibilityTree(wnd)
	node = axMustNode(c, screen.AccessibilityNodeFor(field))
	c.Equal(float64(0), node.Number)
	c.Equal("0", node.Value)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestComboFieldAccessibilityKeepsItsRole verifies that a combo field, which is a field that has been told what it
// really is, keeps that identity once the field itself has had its say.
func TestComboFieldAccessibilityKeepsItsRole(t *testing.T) {
	c := check.New(t)
	first := "One"
	second := "Two"
	var combo *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			combo = unison.NewComboField([]*string{&first, &second}, &first, nil)
			wnd = newHeadlessWindow(t, "combo role", geom.NewRect(10, 10, 300, 150), axColumn(combo))
		}))
	c.NotNil(wnd)
	// Showing a window does not give it the focus, and a combo box in a window that is not the active one is published
	// without the expansion it could not carry out. See axMayPopupMenu and axSnapshot.narrowMenuActions.
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := axMustNode(c, screen.AccessibilityNodeFor(combo))
	c.Equal(role.ComboBox, node.Role, "the field must not overrule what it was told it is")
	c.True(node.Expandable)
	c.True(node.Actions.Has(accessibility.Expand))
	c.Equal("One", node.Value, "it is a field, so its content is still its value")
	c.True(node.Text != nil)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityContextMenuTakesTheFocus verifies that asking a field that is not holding the focus for its
// contextual menu gives it the focus first. The menu is built out of the cut, copy, paste and select-all commands,
// every one of which is routed to whatever currently holds the focus, so a menu shown for an unfocused field described,
// and then acted on, whatever else did. The mouse path never had the problem, since a right-click takes the focus on
// the way in.
func TestFieldAccessibilityContextMenuTakesTheFocus(t *testing.T) {
	c := check.New(t)
	var first, second *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			first = unison.NewField()
			first.SetText("first")
			second = unison.NewField()
			second.SetText("second")
			wnd = newHeadlessWindow(t, "context menu focus", geom.NewRect(10, 10, 300, 200),
				axColumn(first, second))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	c.True(screen.Do(func() { first.RequestFocus() }))
	screen.Sync()
	var focused bool
	screen.Do(func() { focused = first.Focused() })
	c.True(focused, "the first field should be holding the focus")

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(second)
	c.True(node != nil)
	if node == nil {
		return
	}
	before := axRootChildCount(screen.AccessibilityTree(wnd))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}))
	screen.Do(func() { focused = second.Focused() })
	c.True(focused, "the field whose menu was asked for must be the one the menu's commands act on")
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(wnd)),
		"the field's contextual menu should have opened within the window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityContextMenuRefusedInABackgroundWindow verifies that a field in a window that is not the active
// one refuses to show its contextual menu. The menu is not built in the field's own window: Field.ShowContextMenu goes
// through menu.Popup to menu.createPopup, which inserts the popup into ActiveWindow(), so carrying the request out
// would have put this field's menu up in whatever window was frontmost, at coordinates translated from this one — and
// with no window active at all it would have done nothing while reporting that it had been carried out. The mouse path
// never could, since Window.mouseDown delivers nothing to a window that does not have the focus.
func TestFieldAccessibilityContextMenuRefusedInABackgroundWindow(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var background, front *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 500},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("background")
			background = newHeadlessWindow(t, "background", geom.NewRect(10, 10, 250, 150), axColumn(field))
			front = newHeadlessWindow(t, "front", geom.NewRect(300, 10, 250, 150), axColumn(unison.NewField()))
		}))
	c.NotNil(background)
	c.NotNil(front)

	// The field's own window is the active one to begin with, so what follows is about the window it is in rather than
	// about the field.
	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	node := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(node.Actions.Has(accessibility.ShowContextMenu))

	c.True(screen.Do(func() { front.ToFront() }))
	before := axRootChildCount(screen.AccessibilityTree(background))
	frontBefore := axRootChildCount(screen.AccessibilityTree(front))

	// What is refused is not advertised either. A field that went on offering a contextual menu which silently did
	// nothing would leave a screen reader saying the menu can be shown while nothing came of asking, so the action goes
	// with the window's activation: Window.lostFocus marks the window for publishing, and the description that follows
	// is one without it. See axSnapshot.narrowMenuActions.
	backgrounded := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.False(backgrounded.Actions.Has(accessibility.ShowContextMenu),
		"a field that would refuse to show its menu must not offer to")
	c.True(backgrounded.Actions.Has(accessibility.SetValue),
		"everything that acts on the field itself is still perfectly reasonable to ask of a background window")

	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}), "asking a field in a window that is not the active one for its menu has to be refused")
	c.Equal(before, axRootChildCount(screen.AccessibilityTree(background)),
		"nothing should have been opened in the background window")
	c.Equal(frontBefore, axRootChildCount(screen.AccessibilityTree(front)),
		"and nothing should have been opened in the window that is active")

	// Bringing the window back to the front brings the action back with it, since Window.gainedFocus marks it for
	// publishing just as losing the focus did.
	c.True(screen.Do(func() { background.ToFront() }))
	screen.AccessibilityTree(background)
	restored := axMustNode(c, screen.AccessibilityNodeFor(field))
	c.True(restored.Actions.Has(accessibility.ShowContextMenu), "the active window's field offers its menu again")
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.ShowContextMenu,
	}), "and shows it")
	c.Equal(before+1, axRootChildCount(screen.AccessibilityTree(background)),
		"the menu should have opened in the field's own window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilitySetValueStartsANewUndo verifies that replacing a field's value outright begins an edit of its
// own. SetText, unlike the replacement path, does not move the undo id on, so an application's undo manager was free to
// absorb an assistive technology's replacement into whatever edit preceded it.
func TestFieldAccessibilitySetValueStartsANewUndo(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("before")
			wnd = newHeadlessWindow(t, "undo id", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)
	if node == nil {
		return
	}
	var was, now int64
	var text string
	screen.Do(func() { was = field.CurrentUndoID() })
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Value:  "after",
	}))
	screen.Do(func() {
		now = field.CurrentUndoID()
		text = field.Text()
	})
	c.Equal("after", text)
	c.True(now != was, "replacing the value outright must begin an edit of its own rather than joining the last one")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axFloatFormat writes a number out the way the numeric field tests read it back.
func axFloatFormat(value float64) string { return strconv.FormatFloat(value, 'f', 3, 64) }

// axFloatExtract reads a number the way axFloatFormat wrote it.
func axFloatExtract(s string) (float64, error) { return strconv.ParseFloat(s, 64) }

// TestNumericFieldAccessibilityStepFollowsTheRange verifies that a numeric field steps by an amount its range can
// actually be walked in. The step used to be one whatever the field held, so a single increment of a field running from
// zero to one went straight to the maximum and stepping was of no use at all.
func TestNumericFieldAccessibilityStepFollowsTheRange(t *testing.T) {
	c := check.New(t)
	var whole *unison.NumericField[int]
	var fine, wide *unison.NumericField[float64]
	var wholeNarrow *unison.NumericField[int]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			whole = unison.NewNumericField(5, 0, 100, strconv.Itoa, strconv.Atoi, nil)
			wholeNarrow = unison.NewNumericField(2, 0, 4, strconv.Itoa, strconv.Atoi, nil)
			fine = unison.NewNumericField(0.5, 0, 1, axFloatFormat, axFloatExtract, nil)
			wide = unison.NewNumericField(100, 0, 255, axFloatFormat, axFloatExtract, nil)
			wnd = newHeadlessWindow(t, "steps", geom.NewRect(10, 10, 400, 300),
				axColumn(whole, wholeNarrow, fine, wide))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	wholeNode := screen.AccessibilityNodeFor(whole)
	c.True(wholeNode != nil)
	if wholeNode != nil {
		c.Equal(float64(1), wholeNode.Step, "a field of whole numbers steps by one of them")
	}
	narrowNode := screen.AccessibilityNodeFor(wholeNarrow)
	c.True(narrowNode != nil)
	if narrowNode != nil {
		c.Equal(float64(1), narrowNode.Step, "a whole-number field cannot step by less than one however small its range")
	}
	wideNode := screen.AccessibilityNodeFor(wide)
	c.True(wideNode != nil)
	if wideNode != nil {
		c.Equal(float64(1), wideNode.Step, "a range of twenty units or more is counted in whole numbers")
	}
	fineNode := screen.AccessibilityNodeFor(fine)
	c.True(fineNode != nil)
	if fineNode == nil {
		return
	}
	c.Equal(0.05, fineNode.Step, "a fractional range steps by a twentieth of itself")

	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   fineNode.ID,
		Action: accessibility.Increment,
	}))
	var value float64
	screen.Do(func() { value = fine.Value() })
	c.Equal(0.55, value, "incrementing should have moved the value by one step rather than to the maximum")

	for range 3 {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   fineNode.ID,
			Action: accessibility.Decrement,
		}))
	}
	screen.Do(func() { value = fine.Value() })
	c.Equal(0.4, value, "three steps down from 0.55 is 0.4")

	// A whole-number field still moves a whole number at a time.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   narrowNode.ID,
		Action: accessibility.Increment,
	}))
	var count int
	screen.Do(func() { count = wholeNarrow.Value() })
	c.Equal(3, count)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axPercentFormat writes a fraction out as a percentage, which is a field whose text is nothing like the number behind
// it.
func axPercentFormat(value float64) string {
	return strconv.FormatFloat(value*100, 'f', -1, 64) + "%"
}

// axPercentExtract reads a percentage the way axPercentFormat wrote it, and tolerates the trailing sign being left off,
// as a person typing into the field would.
func axPercentExtract(s string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(s), "%"), 64)
	if err != nil {
		return 0, err
	}
	return value / 100, nil
}

// TestNumericFieldAccessibilitySetValuePrefersTheNumber verifies that a spin button asked to take a new value takes the
// number it was sent rather than the text, when the text is nothing more than that number written out. Both macOS and
// Windows send the pair that way, so a field that shows its values as anything but bare digits — a percentage here —
// would otherwise be set to whatever those digits mean when read back through its own Extract, and would skip the clamp
// to its range besides. Text that says something the number does not is still what is used.
func TestNumericFieldAccessibilitySetValuePrefersTheNumber(t *testing.T) {
	c := check.New(t)
	var field *unison.NumericField[float64]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewNumericField(0.5, 0, 1, axPercentFormat, axPercentExtract, nil)
			wnd = newHeadlessWindow(t, "percent", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(role.SpinButton, node.Role)
	c.Equal("50%", node.Value, "the field shows its value the way it formats it")
	c.Equal(0.5, node.Number)

	// What both platforms send: the number, and the same number written out as its text.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Number: 0.25,
		Value:  "0.25",
	}))
	var value float64
	screen.Do(func() { value = field.Value() })
	c.Equal(0.25, value, "the number is what was asked for, not the text read back as a percentage")
	c.Equal("25%", axMustNode(c, screen.AccessibilityNodeFor(field)).Value,
		"which is shown the way the field formats it")

	// A number outside the range is brought into it, exactly as a typed one is.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Number: 9,
		Value:  "9",
	}))
	screen.Do(func() { value = field.Value() })
	c.Equal(float64(1), value, "a value past the maximum should have been clamped to it")

	// Text that says something the number does not is the person's own formatting, so it is what is used.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Number: 0.25,
		Value:  "30%",
	}))
	screen.Do(func() { value = field.Value() })
	c.Equal(0.3, value, "the text was not the number written out, so the text is what was meant")

	// Nothing but text, which is what AT-SPI sends.
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.SetValue,
		Value:  "10%",
	}))
	screen.Do(func() { value = field.Value() })
	c.Equal(0.1, value)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestNumericFieldAccessibilityRangeChangePublishes verifies that widening a field's range reaches an assistive
// technology. Nothing else about the field need change when it does — the value it is showing may still format to the
// same text — and a window is only described again once it has been drawn, so without a redraw the old range would
// stand until something unrelated happened to cause one.
func TestNumericFieldAccessibilityRangeChangePublishes(t *testing.T) {
	c := check.New(t)
	var field *unison.NumericField[int]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewNumericField(5, 0, 10, strconv.Itoa, strconv.Atoi, nil)
			wnd = newHeadlessWindow(t, "range", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)
	if node == nil {
		return
	}
	c.Equal(float64(10), node.Max)

	screen.Do(func() { field.SetMinMax(0, 100) })
	screen.Sync()
	node = screen.AccessibilityNodeFor(field)
	c.True(node != nil)
	if node != nil {
		c.Equal(float64(100), node.Max, "the new range should have been published")
		c.Equal("5", node.Value, "the value it was showing is unchanged, which is why nothing else would have redrawn")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFieldAccessibilityCaretFollowsTheSelection verifies that the caret a field reports is the end of the selection
// that moves: extending a selection to the left with shift+Left or shift+Home leaves the caret at its start, which is
// where the person is and where an assistive technology draws its own cursor, while the end it was extended from stays
// put.
func TestFieldAccessibilityCaretFollowsTheSelection(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = unison.NewField()
			field.SetText("abcdef")
			wnd = newHeadlessWindow(t, "caret", geom.NewRect(10, 10, 300, 150), axColumn(field))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	screen.Click(screen.PanelCenter(field))
	screen.Do(func() { field.SetSelectionTo(3) })
	screen.Sync()

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil && node.Text != nil)
	if node == nil || node.Text == nil {
		return
	}
	c.Equal(3, node.Text.SelStart)
	c.Equal(3, node.Text.SelEnd)
	c.Equal(3, node.Text.Caret, "a caret with nothing selected is both ends of the selection")

	// Extending to the left moves the start, and the caret goes with it.
	screen.KeyPress(unison.KeyLeft, mod.Shift)
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(field)
	c.True(node != nil && node.Text != nil)
	if node == nil || node.Text == nil {
		return
	}
	c.Equal(2, node.Text.SelStart)
	c.Equal(3, node.Text.SelEnd)
	c.True(node.Text.SelStart <= node.Text.SelEnd, "the selection is always reported in order")
	c.Equal(2, node.Text.Caret, "a selection extended backwards has its caret at the start")

	screen.KeyPress(unison.KeyHome, mod.Shift)
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(field)
	c.True(node != nil && node.Text != nil)
	if node == nil || node.Text == nil {
		return
	}
	c.Equal(0, node.Text.SelStart)
	c.Equal(3, node.Text.SelEnd)
	c.Equal(0, node.Text.Caret, "shift+Home leaves the caret at the start of the line")

	// Extending forwards from a fresh caret puts it at the end instead.
	screen.Do(func() { field.SetSelectionTo(1) })
	screen.KeyPress(unison.KeyRight, mod.Shift)
	screen.KeyPress(unison.KeyRight, mod.Shift)
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(field)
	c.True(node != nil && node.Text != nil)
	if node == nil || node.Text == nil {
		return
	}
	c.Equal(1, node.Text.SelStart)
	c.Equal(3, node.Text.SelEnd)
	c.Equal(3, node.Text.Caret, "a selection extended forwards has its caret at the end")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
