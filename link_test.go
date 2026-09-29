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
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
)

// newTestLink creates a link with a frame rect and returns it along with a pointer to its click count.
func newTestLink() (link *unison.Label, clicks *int) {
	count := 0
	link = unison.NewLink("link", "", "target", nil, func(_ unison.Paneler, _ string) { count++ })
	link.SetFrameRect(geom.NewRect(0, 0, 100, 20))
	return link, &count
}

// TestLinkActivatesOnLeftClick verifies the normal activation path: a left press claims the mouse and a left release
// inside the link invokes the click handler.
func TestLinkActivatesOnLeftClick(t *testing.T) {
	c := check.New(t)
	link, clicks := newTestLink()
	pt := geom.NewPoint(10, 10)
	c.True(link.MouseDownCallback(pt, unison.ButtonLeft, 1, 0))
	c.True(link.MouseUpCallback(pt, unison.ButtonLeft, 0))
	c.Equal(1, *clicks)
}

// TestLinkIgnoresNonLeftButtons verifies that right- and middle-button presses are not claimed and do not invoke the
// click handler, matching how other widgets branch on the button.
func TestLinkIgnoresNonLeftButtons(t *testing.T) {
	c := check.New(t)
	link, clicks := newTestLink()
	pt := geom.NewPoint(10, 10)
	for _, button := range []int{unison.ButtonRight, unison.ButtonMiddle} {
		c.False(link.MouseDownCallback(pt, button, 1, 0))
		c.False(link.MouseDragCallback(pt, button, 0))
		c.False(link.MouseUpCallback(pt, button, 0))
		c.Equal(0, *clicks)
	}
	// The link must still work normally after non-left clicks.
	c.True(link.MouseDownCallback(pt, unison.ButtonLeft, 1, 0))
	c.True(link.MouseUpCallback(pt, unison.ButtonLeft, 0))
	c.Equal(1, *clicks)
}

// TestLinkDoesNotActivateWhenReleasedOutside verifies that dragging off the link before releasing cancels activation.
func TestLinkDoesNotActivateWhenReleasedOutside(t *testing.T) {
	c := check.New(t)
	link, clicks := newTestLink()
	inside := geom.NewPoint(10, 10)
	outside := geom.NewPoint(200, 200)
	c.True(link.MouseDownCallback(inside, unison.ButtonLeft, 1, 0))
	c.True(link.MouseDragCallback(outside, unison.ButtonLeft, 0))
	c.True(link.MouseUpCallback(outside, unison.ButtonLeft, 0))
	c.Equal(0, *clicks)
}

// linkTestField returns a field whose caret never blinks, since a blink timer outlives the session that armed it and
// would enqueue a stray task into whichever test runs next.
func linkTestField() *unison.Field {
	f := unison.NewField()
	f.BlinkRate = time.Hour
	return f
}

func linkTestFocus(screen *unison.HeadlessScreen, wnd *unison.Window) *unison.Panel {
	var p *unison.Panel
	screen.Do(func() { p = wnd.CurrentFocus() })
	return p
}

// TestLinkIsATabStop verifies that a link takes the keyboard focus as any control does, whether or not an assistive
// technology is being served, that one the application made unfocusable or disabled does not, and that the link says
// so when it is described.
func TestLinkIsATabStop(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var link, unfocusable, disabled *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			link = unison.NewLink("Documentation", "", "https://example.com/docs", nil, nil)
			unfocusable = unison.NewLink("Guide", "", "https://example.com/guide", nil, nil)
			unfocusable.SetFocusable(false)
			disabled = unison.NewLink("Reference", "", "https://example.com/reference", nil, nil)
			disabled.SetEnabled(false)
			wnd = newHeadlessWindow(t, "tab stop", geom.NewRect(10, 10, 300, 200),
				axColumn(field, link, unfocusable, disabled))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	var focusable, unfocusableFocusable, disabledFocusable bool
	screen.Do(func() {
		focusable = link.Focusable()
		unfocusableFocusable = unfocusable.Focusable()
		disabledFocusable = disabled.Focusable()
		wnd.SetFocus(field)
	})
	c.True(focusable, "a link can take the focus")
	c.False(unfocusableFocusable, "unless the application said otherwise")
	c.False(disabledFocusable, "or disabled it")
	screen.KeyPress(unison.KeyTab, mod.None)
	c.True(linkTestFocus(screen, wnd).Is(link), "Tab stops at the link")
	screen.KeyPress(unison.KeyTab, mod.None)
	c.True(linkTestFocus(screen, wnd).Is(field), "and passes over the two that cannot take the focus")
	c.False(unison.IsAccessibilityActive(), "none of which needed an assistive technology")

	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := axMustNode(c, screen.AccessibilityNodeFor(link))
	c.True(node.Focusable)
	c.True(node.Actions.Has(accessibility.Focus))
	c.True(node.Actions.Has(accessibility.Press))
	c.False(axMustNode(c, screen.AccessibilityNodeFor(unfocusable)).Focusable)
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Focus,
	}), "a screen reader asking for the focus on the link gets it")
	c.True(linkTestFocus(screen, wnd).Is(link))
	tree = screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree != nil {
		c.Equal(node.ID, tree.Focus, "and the description reports the focus there")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinkIsFollowedByTheKeyboard verifies that Return, the keypad's Enter and the space bar each follow the link that
// holds the focus, taking the key so that Return does not go on to a dialog's default button, that no other key does,
// nor any of those with a modifier held, and that a disabled link is not followed though it holds the focus, as one
// that was disabled while holding it does. TestDisabledLinkTakesTheFocusForReading covers the disabled link that is
// given the focus so that it can be read.
func TestLinkIsFollowedByTheKeyboard(t *testing.T) {
	c := check.New(t)
	const target = "https://example.com/docs"
	var followed []string
	var passedOn []unison.KeyCode
	var field *unison.Field
	var link *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			link = unison.NewLink("Documentation", "", target, nil,
				func(_ unison.Paneler, where string) { followed = append(followed, where) })
			column := axColumn(field, link)
			// Stands in for whatever the link is inside, which is where a dialog's default button hears of Return.
			column.KeyDownCallback = func(keyCode unison.KeyCode, _ mod.Modifiers, _ bool) bool {
				if keyCode == unison.KeyReturn || keyCode == unison.KeyNumPadEnter || keyCode == unison.KeySpace {
					passedOn = append(passedOn, keyCode)
				}
				return false
			}
			wnd = newHeadlessWindow(t, "keys", geom.NewRect(10, 10, 300, 200), column)
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	count := func() int {
		n := 0
		screen.Do(func() { n = len(followed) })
		return n
	}

	screen.Do(func() { wnd.SetFocus(field) })
	screen.KeyPress(unison.KeySpace, mod.None)
	c.Equal(0, count(), "a link that does not hold the focus is not followed")

	screen.Do(func() { wnd.SetFocus(link) })
	screen.KeyPress(unison.KeySpace, mod.None)
	c.Equal(1, count(), "the space bar follows the link that holds the focus")
	screen.Do(func() {
		if len(followed) == 1 {
			c.Equal(target, followed[0])
		}
	})
	c.True(linkTestFocus(screen, wnd).Is(link), "which keeps the focus")

	screen.KeyPress(unison.KeyReturn, mod.None)
	c.Equal(2, count(), "so does Return")
	screen.KeyPress(unison.KeyNumPadEnter, mod.None)
	c.Equal(3, count(), "and the keypad's Enter")
	c.True(linkTestFocus(screen, wnd).Is(link))
	var seen int
	screen.Do(func() { seen = len(passedOn) })
	c.Equal(0, seen, "none of which goes on to what the link is inside")

	screen.KeyPress(unison.KeyA, mod.None)
	for _, key := range []unison.KeyCode{unison.KeySpace, unison.KeyReturn, unison.KeyNumPadEnter} {
		screen.KeyPress(key, mod.Shift)
		screen.KeyPress(key, mod.OSMenuCommand())
	}
	c.Equal(3, count(), "no other key follows it, nor any of those with a modifier held")
	screen.Do(func() {
		seen = len(passedOn)
		passedOn = nil
	})
	c.Equal(6, seen, "which are passed on instead")

	screen.Click(screen.PanelCenter(link))
	c.Equal(4, count(), "a click goes on following it")

	screen.Do(func() { link.SetEnabled(false) })
	c.True(linkTestFocus(screen, wnd).Is(link))
	screen.KeyPress(unison.KeySpace, mod.None)
	screen.KeyPress(unison.KeyReturn, mod.None)
	c.Equal(4, count(), "a disabled link is not followed")
	screen.Do(func() { seen = len(passedOn) })
	c.Equal(2, seen, "and its keys go to what it is inside, as for any disabled control")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinkWithoutAHandlerTakesItsKeys verifies that a link created without a click handler has nothing to follow, and
// does not panic for being asked to, while still taking the keys that would have followed it.
func TestLinkWithoutAHandlerTakesItsKeys(t *testing.T) {
	c := check.New(t)
	link := unison.NewLink("link", "", "target", nil, nil)
	c.True(link.KeyDownCallback(unison.KeySpace, mod.None, false))
	c.True(link.KeyDownCallback(unison.KeyReturn, mod.None, false))
	c.True(link.KeyDownCallback(unison.KeyNumPadEnter, mod.None, false))
	c.False(link.KeyDownCallback(unison.KeyReturn, mod.Shift, false))
	c.False(link.KeyDownCallback(unison.KeyTab, mod.None, false))
}

// TestLinkTakesTheFocusOnClickForAScreenReader verifies that a click gives the link the focus while an assistive
// technology is being served, as it does for a button, that it does not otherwise, and that a link that cannot hold
// the focus leaves it where it was rather than having the window take it away from whatever held it.
func TestLinkTakesTheFocusOnClickForAScreenReader(t *testing.T) {
	c := check.New(t)
	followed := 0
	handler := func(_ unison.Paneler, _ string) { followed++ }
	var field *unison.Field
	var link, unfocusable *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			link = unison.NewLink("Documentation", "", "https://example.com/docs", nil, handler)
			unfocusable = unison.NewLink("Guide", "", "https://example.com/guide", nil, handler)
			unfocusable.SetFocusable(false)
			wnd = newHeadlessWindow(t, "click", geom.NewRect(10, 10, 300, 200), axColumn(field, link, unfocusable))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	screen.Do(func() { wnd.SetFocus(field) })
	screen.Click(screen.PanelCenter(link))
	c.True(linkTestFocus(screen, wnd).Is(field), "with nothing listening, a click leaves the focus where it was")

	screen.EnableAccessibility()
	screen.Click(screen.PanelCenter(link))
	c.True(linkTestFocus(screen, wnd).Is(link), "with a screen reader listening, the clicked link takes the focus")
	screen.Do(func() { wnd.SetFocus(field) })
	screen.Click(screen.PanelCenter(unfocusable))
	c.True(linkTestFocus(screen, wnd).Is(field), "a link that cannot hold the focus leaves it where it was")
	var count int
	screen.Do(func() { count = followed })
	c.Equal(3, count, "every one of the clicks followed its link")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinkShowsTheFocus verifies that a link is drawn differently while it holds the focus, and as it was once the
// focus has gone.
func TestLinkShowsTheFocus(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var link *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			link = unison.NewLink("Documentation", "", "https://example.com/docs", nil, nil)
			wnd = newHeadlessWindow(t, "focus", geom.NewRect(10, 10, 300, 200), axColumn(field, link))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(field)
	}))

	capture := func() []byte {
		screen.Sync()
		img := screen.CaptureWindow(wnd)
		if img == nil {
			return nil
		}
		var rect geom.Rect
		screen.Do(func() { rect = link.RectToRoot(link.ContentRect(true)) })
		scale := screen.Scale()
		var pixels []byte
		for y := int(rect.Y * scale); y < int(rect.Bottom()*scale); y++ {
			for x := int(rect.X * scale); x < int(rect.Right()*scale); x++ {
				px := img.NRGBAAt(x, y)
				pixels = append(pixels, px.R, px.G, px.B, px.A)
			}
		}
		return pixels
	}

	unfocused := capture()
	c.True(len(unfocused) > 0, "the link should have been drawn")
	screen.Do(func() { wnd.SetFocus(link) })
	c.NotEqual(unfocused, capture(), "a link that holds the focus shows that it does")
	screen.Do(func() { wnd.SetFocus(field) })
	c.Equal(unfocused, capture(), "and is drawn as it was once the focus has gone")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestMarkdownLinksAreNotTabStops verifies that the links of a document are left to the document, which follows the
// one under its reading caret, rather than each becoming a tab stop of its own, and that a click still follows one,
// both while nothing is listening, when the document takes no focus either, and while a screen reader is being served
// with the switch on, when Tab stops at the document and a click on a link gives the document the focus with its
// reading caret on the link.
func TestMarkdownLinksAreNotTabStops(t *testing.T) {
	c := check.New(t)
	t.Cleanup(func() { unison.SetFocusForReading(false) })
	var followed []string
	var field *unison.Field
	var md *unison.Markdown
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			md = unison.NewMarkdown(false)
			md.LinkHandler = func(_ unison.Paneler, where string) { followed = append(followed, where) }
			md.SetContent("Read [the guide](https://example.com/guide) first.\n", 400)
			wnd = newHeadlessWindow(t, "markdown links", geom.NewRect(10, 10, 450, 300), axColumn(field, md))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))

	var links []*unison.Panel
	screen.Do(func() {
		var walk func(p *unison.Panel)
		walk = func(p *unison.Panel) {
			for _, child := range p.Children() {
				if child.Accessibility.Role == role.Link {
					links = append(links, child)
				}
				walk(child)
			}
		}
		walk(md.AsPanel())
	})
	c.Equal(1, len(links), "the document should hold the one link")
	if len(links) != 1 {
		return
	}
	var focusable bool
	screen.Do(func() {
		focusable = links[0].Focusable()
		wnd.SetFocus(field)
	})
	c.False(focusable, "a link in a document is not a tab stop")
	screen.KeyPress(unison.KeyTab, mod.None)
	c.True(linkTestFocus(screen, wnd).Is(field), "so Tab wraps around onto the field, the only tab stop there is")

	screen.Click(screen.PanelCenter(links[0]))
	var count int
	screen.Do(func() { count = len(followed) })
	c.Equal(1, count, "a click still follows the link")
	c.True(linkTestFocus(screen, wnd).Is(field), "and leaves the focus where it was")

	var width, textWidth float32
	screen.Do(func() {
		width = links[0].FrameRect().Width
		_, pref, _ := unison.NewLink("the guide", "", "", nil, nil).Sizes(geom.Size{})
		textWidth = pref.Width
	})
	c.True(width < textWidth, "a link in a document takes up no room for an outline it never draws: %v against %v",
		width, textWidth)

	screen.EnableAccessibility()
	unison.SetFocusForReading(true)
	screen.Sync()
	var documentFocusable bool
	screen.Do(func() {
		focusable = links[0].Focusable()
		documentFocusable = md.Focusable()
		wnd.SetFocus(field)
	})
	c.True(documentFocusable, "with a screen reader listening, the document takes the focus")
	c.False(focusable, "and its link still does not")
	screen.KeyPress(unison.KeyTab, mod.None)
	c.True(linkTestFocus(screen, wnd).Is(md), "so Tab stops at the document")
	screen.KeyPress(unison.KeyTab, mod.None)
	c.True(linkTestFocus(screen, wnd).Is(field), "and then wraps around, passing over the link")

	screen.Click(screen.PanelCenter(links[0]))
	screen.Do(func() { count = len(followed) })
	c.Equal(2, count, "a click follows the link")
	c.True(linkTestFocus(screen, wnd).Is(md), "and gives the document the focus, as a click anywhere else in it does")
	c.NotNil(screen.AccessibilityTree(wnd))
	node := axMustNode(c, screen.AccessibilityNodeFor(md))
	c.NotNil(node.Document)
	if node.Document != nil {
		text := node.Document.Text
		start := strings.Index(text.Text, "the guide")
		c.True(start >= 0, "the document's text should hold the link's: %q", text.Text)
		start = len([]rune(text.Text[:max(start, 0)]))
		c.True(text.Caret >= start && text.Caret < start+len([]rune("the guide")),
			"the reading caret is put on the link, not left at %d", text.Caret)
	}
	screen.KeyPress(unison.KeyReturn, mod.None)
	screen.Do(func() { count = len(followed) })
	c.Equal(3, count, "where Return follows the link")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDisabledLinkTakesTheFocusForReading verifies a disabled link while a screen reader is being served with the
// switch on: Tab stops at it, neither the space bar nor a click follows it, and it is published as disabled, able to
// take the focus, and offering that and no press.
func TestDisabledLinkTakesTheFocusForReading(t *testing.T) {
	c := check.New(t)
	t.Cleanup(func() { unison.SetFocusForReading(false) })
	followed := 0
	var field *unison.Field
	var link *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			link = unison.NewLink("Reference", "", "https://example.com/reference", nil,
				func(_ unison.Paneler, _ string) { followed++ })
			link.SetEnabled(false)
			wnd = newHeadlessWindow(t, "disabled link", geom.NewRect(10, 10, 300, 200), axColumn(field, link))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	screen.EnableAccessibility()
	unison.SetFocusForReading(true)
	screen.Sync()

	screen.Do(func() { wnd.SetFocus(field) })
	screen.KeyPress(unison.KeyTab, mod.None)
	c.True(linkTestFocus(screen, wnd).Is(link), "Tab stops at the disabled link")
	screen.KeyPress(unison.KeySpace, mod.None)
	screen.Do(func() { wnd.SetFocus(field) })
	screen.Click(screen.PanelCenter(link))
	c.True(linkTestFocus(screen, wnd).Is(field), "a click does not move the focus onto it")
	var count int
	screen.Do(func() { count = followed })
	c.Equal(0, count, "and neither the space bar nor a click follows it")

	screen.Do(func() { wnd.SetFocus(link) })
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := axMustNode(c, screen.AccessibilityNodeFor(link))
	c.Equal(role.Link, node.Role)
	c.True(node.Disabled)
	c.True(node.Focusable)
	c.True(node.Focused)
	c.Equal(accessibility.ActionSet(0).With(accessibility.ScrollIntoView, accessibility.Focus), node.Actions)
	if tree != nil {
		c.Equal(node.ID, tree.Focus)
	}
	c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
		Node:   node.ID,
		Action: accessibility.Press,
	}), "a press is refused")
	screen.Do(func() { count = followed })
	c.Equal(0, count)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestLinkOutlineClearsTheText verifies that the outline a link draws while it holds the focus has room of its own: no
// part of the link's text is drawn where the outline goes.
func TestLinkOutlineClearsTheText(t *testing.T) {
	c := check.New(t)
	var field *unison.Field
	var link *unison.Label
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			field = linkTestField()
			link = unison.NewLink("Typography jig", "", "https://example.com/docs", nil, nil)
			link.SetLayoutData(&unison.FlexLayoutData{})
			wnd = newHeadlessWindow(t, "outline", geom.NewRect(10, 10, 300, 200), axColumn(field, link))
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.ToFront()
		wnd.SetFocus(field)
	}))
	screen.Sync()
	img := screen.CaptureWindow(wnd)
	c.NotNil(img)
	if img == nil {
		return
	}
	var rect geom.Rect
	var prefWidth, textWidth float32
	screen.Do(func() {
		rect = link.RectToRoot(link.ContentRect(true))
		_, pref, _ := link.Sizes(geom.Size{})
		prefWidth = pref.Width
		textWidth = link.Text.Width()
	})
	c.True(prefWidth >= textWidth+4, "the link asks for room either side of its text: %v for text %v wide", prefWidth,
		textWidth)
	c.Equal(prefWidth, rect.Width, "the link should have been laid out at the size it asked for")

	// The outline is one pixel wide along the edge of the link. With the link not holding the focus, every pixel there
	// must be the background, which is what the corner holds.
	scale := screen.Scale()
	left, top := int(rect.X*scale), int(rect.Y*scale)
	right, bottom := int(rect.Right()*scale), int(rect.Bottom()*scale)
	band := int(scale)
	background := img.NRGBAAt(left, top)
	inked := 0
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			if x >= left+band && x < right-band && y >= top+band && y < bottom-band {
				continue
			}
			if img.NRGBAAt(x, y) != background {
				inked++
			}
		}
	}
	c.Equal(0, inked, "no part of the text may be drawn where the outline goes")
	text := 0
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			if img.NRGBAAt(x, y) != background {
				text++
			}
		}
	}
	c.True(text > 0, "the text itself should have been drawn")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// linkCellRow returns a row for a table of three columns whose cells hold a field, a link and a label. The field is
// built once, since a table keeps the cell that holds the focus installed and asks for it again on every draw.
func linkCellRow(id string, followed *int) *tableTestRow {
	row := newTableTestRow(id)
	field := linkTestField()
	field.SetText(id)
	row.cellFactory = func(_, col int) unison.Paneler {
		switch col {
		case 0:
			return field
		case 1:
			return unison.NewLink("Reference", "", "https://example.com/"+id, nil,
				func(_ unison.Paneler, _ string) { *followed++ })
		default:
			label := unison.NewLabel()
			label.SetTitle("Notes")
			return label
		}
	}
	return row
}

// TestLinkInATableCellTakesNoFocus verifies that a link used as the content of a table's cell is left out of the focus,
// however the focus is asked of it: Return on the cell, a press from an assistive technology on the cell or on the
// link, and a click while one is being served all leave the focus with the table, whose arrow keys go on moving the
// person from row to row, and Tab within the table passes over the cell. Each of them but Return still follows the
// link.
func TestLinkInATableCellTakesNoFocus(t *testing.T) {
	c := check.New(t)
	followed := 0
	var table *unison.Table[*tableTestRow]
	var wnd *unison.Window
	screen := startHeadless(t, unison.HeadlessConfig{Width: 500, Height: 400},
		unison.StartupFinishedCallback(func() {
			table = newCursorTable(linkCellRow("r0", &followed), linkCellRow("r1", &followed))
			wnd = newHeadlessWindow(t, "links in cells", geom.NewRect(10, 10, 400, 300), table)
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() { wnd.ToFront() }))
	screen.EnableAccessibility()

	type state struct {
		focusOnTable         bool
		focusedRow, focusCol int
		leadRow, leadCol     int
		followed             int
	}
	current := func() state {
		var s state
		screen.Do(func() {
			s.focusOnTable = wnd.CurrentFocus().Is(table)
			s.focusedRow, s.focusCol = table.FocusedCell()
			s.leadRow, s.leadCol = table.LeadRowIndex(), table.LeadColumnIndex()
			s.followed = followed
		})
		return s
	}
	onLinkCell := func() {
		screen.Do(func() {
			wnd.SetFocus(table)
			table.SetLeadCell(0, 1)
		})
	}
	verify := func(what string, wantFollowed int) {
		s := current()
		c.True(s.focusOnTable, "%s: the focus stays with the table", what)
		c.Equal(-1, s.focusedRow, "%s: no cell holds the focus", what)
		c.Equal(-1, s.focusCol, "%s: no cell holds the focus", what)
		c.Equal(wantFollowed, s.followed, "%s: the link followed", what)
		screen.KeyPress(unison.KeyDown, mod.None)
		s = current()
		c.Equal(1, s.leadRow, "%s: the down arrow still moves to the next row", what)
		c.True(s.focusOnTable)
	}

	onLinkCell()
	s := current()
	c.Equal(0, s.leadRow)
	c.Equal(1, s.leadCol)
	screen.KeyPress(unison.KeyReturn, mod.None)
	verify("Return on the cell", 0)

	onLinkCell()
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree == nil {
		return
	}
	var cell, link *accessibility.Node
	tree.Walk(func(n *accessibility.Node) bool {
		switch {
		case n.Role == role.Cell && n.RowIndex == 0 && n.ColumnIndex == 1:
			cell = n
		case n.Role == role.Link && link == nil:
			link = n
		}
		return true
	})
	c.NotNil(cell, "the cell holding the link should have been described")
	c.NotNil(link, "and so should the link")
	if cell == nil || link == nil {
		return
	}
	c.False(link.Focusable, "a link in a cell cannot take the focus")
	c.False(link.Actions.Has(accessibility.Focus))
	c.True(link.Actions.Has(accessibility.Press))
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: cell.ID, Action: accessibility.Press}))
	verify("a press on the cell", 1)

	onLinkCell()
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: link.ID, Action: accessibility.Press}))
	verify("a press on the link", 2)

	onLinkCell()
	var center geom.Point
	screen.Do(func() { center = table.RectToRoot(table.CellFrame(0, 1)).Center() })
	var origin geom.Point
	screen.Do(func() { origin = wnd.ContentRect().Point })
	screen.Click(origin.Add(center))
	s = current()
	c.Equal(3, s.followed, "a click follows the link")
	c.Equal(-1, s.focusedRow, "and leaves no cell holding the focus")

	// Tab from the field in the first cell of a row goes on to the field in the row below, passing over the link.
	var entered bool
	screen.Do(func() {
		wnd.SetFocus(table)
		entered = table.FocusCell(0, 0)
	})
	c.True(entered, "the field in the first cell takes the focus")
	screen.KeyPress(unison.KeyTab, mod.None)
	s = current()
	c.Equal(1, s.focusedRow, "Tab passes over the cell holding the link")
	c.Equal(0, s.focusCol)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
