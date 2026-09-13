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
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
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

// TestFieldAccessibility verifies what a field says about itself: its content is its value, its watermark is the prompt
// an empty one shows, a field that fails validation says so, and a field that obscures what it holds gives up nothing.
func TestFieldAccessibility(t *testing.T) {
	c := check.New(t)
	var plain, watermarked, invalid, secret, multiLine *unison.Field
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 600, Height: 600},
		unison.StartupFinishedCallback(func() {
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
				axColumn(plain, watermarked, invalid, secret, multiLine))
		}))
	c.NotNil(wnd)

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(plain)
	c.True(node != nil)
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
	c.True(node.Text != nil)
	if node.Text != nil {
		c.Equal("Hello", node.Text.Text)
		c.Equal(5, node.Text.SelStart, "setting the text left the caret at the end of it")
		c.Equal(5, node.Text.SelEnd)
		c.False(node.Text.Multiline)
		c.Equal(0, len(node.Text.Lines), "the lines of a field nobody is working in are not measured")
	}

	watermarkNode := screen.AccessibilityNodeFor(watermarked)
	c.True(watermarkNode != nil)
	c.Equal("Search", watermarkNode.Placeholder, "the watermark is what an empty field prompts with")
	c.Equal("", watermarkNode.Value)

	invalidNode := screen.AccessibilityNodeFor(invalid)
	c.True(invalidNode != nil)
	c.True(invalidNode.Invalid, "a field whose content was rejected says so")

	secretNode := screen.AccessibilityNodeFor(secret)
	c.True(secretNode != nil)
	c.Equal(role.TextField, secretNode.Role)
	c.True(secretNode.Protected)
	c.Equal("", secretNode.Value, "a password is never handed out")
	c.True(secretNode.Text == nil, "not even the caret within a password is reported")

	multiNode := screen.AccessibilityNodeFor(multiLine)
	c.True(multiNode != nil)
	c.Equal(role.TextArea, multiNode.Role, "a field that accepts line feeds is a text area")
	c.True(multiNode.Text != nil)
	if multiNode.Text != nil {
		c.True(multiNode.Text.Multiline)
		c.Equal("one\ntwo", multiNode.Text.Text)
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
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)

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
	node = screen.AccessibilityNodeFor(field)
	c.True(node != nil)
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
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)
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

// TestFieldAccessibilityLines verifies that the laid-out lines of a field are measured only for the field holding the
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
	node := screen.AccessibilityNodeFor(area)
	c.True(node != nil)
	c.True(node.Text != nil)
	if node.Text != nil {
		c.Equal(0, len(node.Text.Lines), "an unfocused field's lines are not measured")
	}

	c.True(screen.Do(func() {
		single.RequestFocus()
		single.SetSelectionTo(1)
	}))
	screen.AccessibilityTree(wnd)
	node = screen.AccessibilityNodeFor(single)
	c.True(node != nil)
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
	emptyNode := screen.AccessibilityNodeFor(empty)
	c.True(emptyNode != nil)
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
	areaNode := screen.AccessibilityNodeFor(area)
	c.True(areaNode != nil)
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
	node := screen.AccessibilityNodeFor(field)
	c.True(node != nil)
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
	node = screen.AccessibilityNodeFor(field)
	c.True(node != nil)
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

	screen.AccessibilityTree(wnd)
	node := screen.AccessibilityNodeFor(combo)
	c.True(node != nil)
	c.Equal(role.ComboBox, node.Role, "the field must not overrule what it was told it is")
	c.True(node.Expandable)
	c.True(node.Actions.Has(accessibility.Expand))
	c.Equal("One", node.Value, "it is a field, so its content is still its value")
	c.True(node.Text != nil)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
