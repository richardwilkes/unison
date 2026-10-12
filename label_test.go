// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/side"
)

// axLabelTestDrawable stands in for an icon: it takes up room and paints nothing, so what is left on the canvas after a
// label holding one has been drawn is the label's text alone.
type axLabelTestDrawable struct {
	size geom.Size
}

func (d *axLabelTestDrawable) LogicalSize() geom.Size {
	return d.size
}

func (d *axLabelTestDrawable) DrawInRect(_ *Canvas, _ geom.Rect, _ *SamplingOptions, _ *Paint) {
}

// axInkBounds returns the smallest rectangle of pixel indexes holding every non-transparent pixel of a pixmap, or a
// rectangle with a negative origin when nothing was drawn at all.
func axInkBounds(pix *raster.Pixmap) (minX, minY, maxX, maxY int32) {
	minX, minY, maxX, maxY = -1, -1, -1, -1
	for y := int32(0); y < pix.Height; y++ {
		for x := int32(0); x < pix.Width; x++ {
			if pix.Pix[int(y)*int(pix.RowPixels)+int(x)]>>24 == 0 {
				continue
			}
			if minX == -1 || x < minX {
				minX = x
			}
			if minY == -1 || y < minY {
				minY = y
			}
			maxX = max(maxX, x)
			maxY = max(maxY, y)
		}
	}
	return minX, minY, maxX, maxY
}

// TestLabelTextOriginMatchesDrawLabel checks that where a label says its text is, for an assistive technology, is where
// the label actually drew it: the text is drawn once by DrawLabel and once on its own at Label.axTextOrigin, and the
// two canvases must hold the same pixels. Both go through labelPlacement, so what this pins is that the origin the
// accessibility side hands out is the origin the drawing used, including the flooring the alignments do and the room a
// drawable takes on each of the four sides. A document composed of labels reports each run of its text at that origin,
// and an origin off by the height of a line would put a screen reader's highlight, and a person's reading caret, on the
// wrong words.
func TestLabelTextOriginMatchesDrawLabel(t *testing.T) {
	c := check.New(t)
	const extent = int32(140)
	for _, one := range []struct {
		name     string
		hAlign   align.Enum
		vAlign   align.Enum
		side     side.Enum
		drawable geom.Size
	}{
		{name: "start/start", hAlign: align.Start, vAlign: align.Start},
		{name: "middle/middle", hAlign: align.Middle, vAlign: align.Middle},
		{name: "end/end", hAlign: align.End, vAlign: align.End},
		{name: "fill/fill", hAlign: align.Fill, vAlign: align.Fill},
		{
			name: "drawable on the left", hAlign: align.Middle, vAlign: align.Middle, side: side.Left,
			drawable: geom.NewSize(24, 24),
		},
		{
			name: "drawable on the right", hAlign: align.Start, vAlign: align.Middle, side: side.Right,
			drawable: geom.NewSize(24, 24),
		},
		{
			name: "drawable on top", hAlign: align.Middle, vAlign: align.Start, side: side.Top,
			drawable: geom.NewSize(30, 18),
		},
		{
			name: "drawable below", hAlign: align.End, vAlign: align.End, side: side.Bottom,
			drawable: geom.NewSize(30, 18),
		},
		{
			name: "a drawable narrower than the text", hAlign: align.Middle, vAlign: align.Middle, side: side.Top,
			drawable: geom.NewSize(4, 4),
		},
	} {
		label := NewLabel()
		label.HAlign = one.hAlign
		label.VAlign = one.vAlign
		label.Side = one.side
		label.SetTitle("Hg jy")
		if one.drawable.Width > 0 {
			label.Drawable = &axLabelTestDrawable{size: one.drawable}
		}
		label.SetFrameRect(geom.NewRect(0, 0, float32(extent), float32(extent)))
		rect := label.ContentRect(false)

		drawn, drawnPix := newPixmapCanvas(extent, extent)
		DrawLabel(drawn, rect, label.HAlign, label.VAlign, label.Font, label.Text, label.OnBackgroundInk, nil,
			label.Drawable, label.Side, label.Gap, false)

		reported, reportedPix := newPixmapCanvas(extent, extent)
		origin := label.axTextOrigin()
		label.Text.Draw(reported, geom.NewPoint(origin.X, origin.Y+label.Text.Baseline()))

		minX, minY, maxX, maxY := axInkBounds(drawnPix)
		c.True(minX >= 0, "%s: the label drew nothing to compare", one.name)
		rMinX, rMinY, rMaxX, rMaxY := axInkBounds(reportedPix)
		c.Equal([]int32{minX, minY, maxX, maxY}, []int32{rMinX, rMinY, rMaxX, rMaxY},
			"%s: the text was drawn somewhere other than where axTextOrigin says", one.name)
		c.Equal(drawnPix.Pix, reportedPix.Pix, "%s: the two renderings must be identical", one.name)
	}
}

// TestLabelTextLineDescribesWhatWasDrawn covers the line a label contributes to a document's text: its runes, one
// advance per rune boundary, and bounds that sit where the text was drawn. It is what a list's bullet and an alert's
// title are folded into a document's stream as.
func TestLabelTextLineDescribesWhatWasDrawn(t *testing.T) {
	c := check.New(t)
	label := NewLabel()
	label.SetTitle("Hi there")
	label.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	runes, decorations, line := label.axTextLine()
	c.Equal([]rune("Hi there"), runes)
	c.Equal(len(runes), len(decorations), "every rune must say what it was drawn with")
	for i, decoration := range decorations {
		c.True(decoration != nil, "rune %d has no decoration", i)
	}
	c.Equal(0, line.Start)
	c.Equal(len(runes), line.End)
	c.Equal(len(runes)+1, len(line.Advances), "the advances are the rune boundaries, so there is one more of them")
	c.Equal(float32(0), line.Advances[0])
	for i := 1; i < len(line.Advances); i++ {
		c.True(line.Advances[i] >= line.Advances[i-1], "the advances must not go backwards at %d", i)
	}
	nearlyEqual(c, label.Text.Width(), line.Advances[len(line.Advances)-1])
	origin := label.axTextOrigin()
	c.Equal(origin, line.Bounds.Point, "the line sits where the text was drawn")
	nearlyEqual(c, label.Text.Width(), line.Bounds.Width)
	nearlyEqual(c, label.Text.Height(), line.Bounds.Height)

	// A label holding no text still occupies a line, since it is still as tall as one and a caret can sit in it.
	empty := NewLabel()
	empty.SetTitle("")
	empty.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	runes, decorations, line = empty.axTextLine()
	c.Equal(0, len(runes))
	c.Equal(0, len(decorations))
	c.Equal([]float32{0}, line.Advances, "one advance, which is where a caret in empty text sits")
	c.Equal(0, line.End)
	nearlyEqual(c, empty.Text.Height(), line.Bounds.Height)

	// That holds for a label showing nothing but an icon as well: what the drawable decides is how big the label is, not
	// how tall the line of text within it is, and a line with no height is not one a caret could sit in or a rectangle an
	// assistive technology could be told about.
	iconOnly := NewLabel()
	iconOnly.SetTitle("")
	iconOnly.Drawable = &axLabelTestDrawable{size: geom.NewSize(24, 18)}
	iconOnly.SetFrameRect(geom.NewRect(0, 0, 200, 40))
	runes, decorations, line = iconOnly.axTextLine()
	c.Equal(0, len(runes))
	c.Equal(0, len(decorations))
	c.Equal([]float32{0}, line.Advances, "one advance, which is where a caret in empty text sits")
	c.Equal(0, line.End)
	c.True(line.Bounds.Height > 0, "an icon-only label's line is as tall as a line of its font")
	nearlyEqual(c, iconOnly.Text.Height(), line.Bounds.Height)

	// The size the label lays out at is untouched by that: an icon-only label is exactly as big as its icon, which is
	// what every widget built on LabelContentSizes measures itself by.
	size, txtSize := LabelContentSizes(iconOnly.Text, iconOnly.Drawable, iconOnly.Font, iconOnly.Side, iconOnly.Gap)
	c.Equal(geom.NewSize(24, 18), size, "the label is still exactly as big as the icon it shows")
	c.Equal(float32(0), txtSize.Width, "and its text is still as wide as the nothing it holds")
	nearlyEqual(c, iconOnly.Text.Height(), txtSize.Height)
}

// These tests cover the keyboard focus a heading takes for an assistive technology's sake — see Panel.axTakesFocus and
// axHeadingsTakeFocus: that it takes it only while one is being served and only where a screen reader asks a heading
// for the focus, that a plain label never takes it, and that a heading is not what a window with nothing focused hands
// the focus to when there is a real tab stop in it. Both answers are pinned rather than left to the platform the test
// happens to run on. A session owns most of the package's mutable globals while it runs, so none of these may call
// t.Parallel.

// axNewHeadingLabel returns a label marked up as a heading at the given depth, which is all an application does to
// make one.
func axNewHeadingLabel(title string, level int) *Label {
	l := NewLabel()
	l.SetTitle(title)
	l.Accessibility.Role = role.Heading
	l.Accessibility.Level = level
	return l
}

// TestHeadingTakesFocusOnlyWhereScreenReadersGrabIt verifies that a heading becomes focusable exactly while an
// assistive technology is being served on a platform whose screen reader lands on a heading by asking for the focus
// there, that the plain label beside it never does, and that a screen reader asking for the focus gets it.
func TestHeadingTakesFocusOnlyWhereScreenReadersGrabIt(t *testing.T) {
	c := check.New(t)
	var heading, plain *Label
	var field *Field
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			heading = axNewHeadingLabel("Settings", 1)
			plain = NewLabel()
			plain.SetTitle("Some text")
			field = NewField()
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(heading)
			column.AddChild(plain)
			column.AddChild(field)
			wnd = axNewTestWindow(t, "heading focus", geom.NewRect(10, 10, 500, 500), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	focusable := func(p Paneler) bool {
		var f bool
		screen.Do(func() { f = p.AsPanel().Focusable() })
		return f
	}
	focus := func() *Panel {
		var p *Panel
		screen.Do(func() { p = wnd.CurrentFocus() })
		return p
	}

	saved := axHeadingsTakeFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })

	c.False(focusable(heading), "with no assistive technology, a heading is not focusable")
	c.False(focusable(plain), "and a plain label never is")

	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())
	c.True(focusable(heading), "while one is being served, a heading takes the focus so the jump to it can land")
	c.False(focusable(plain), "a plain label is read where it is and is never jumped to")
	tree := screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	node := screen.AccessibilityNodeFor(heading)
	plainNode := screen.AccessibilityNodeFor(plain)
	c.NotNil(node)
	c.NotNil(plainNode)
	if tree == nil || node == nil || plainNode == nil {
		return
	}
	c.True(node.Focusable, "the node says it can take the focus")
	c.True(node.Actions.Has(accessibility.Focus), "and offers to")
	c.False(plainNode.Focusable)
	c.False(plainNode.Actions.Has(accessibility.Focus))

	screen.Do(func() { wnd.SetFocus(field) })
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(heading), "a heading that can be landed on is a tab stop, so Tab wraps around onto it")

	screen.Do(func() { wnd.SetFocus(nil) })
	c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{Node: node.ID, Action: accessibility.Focus}))
	c.True(focus().Is(heading), "a screen reader asking for the focus on the heading gets it")
	tree = screen.AccessibilityTree(wnd)
	c.NotNil(tree)
	if tree != nil {
		c.Equal(node.ID, tree.Focus, "and the description reports the focus there")
	}

	// The focus is left on the heading, so that the Tab below has to move it: were the heading still a tab stop, the
	// move would wrap around onto it again and the field would never be reached.
	screen.Do(func() {
		wnd.SetFocus(heading)
		axHeadingsTakeFocus = false
	})
	c.True(focus().Is(heading), "the heading is where the focus was left")
	c.False(focusable(heading), "where a screen reader moves a cursor of its own, a heading is never a place to land")
	screen.KeyPress(KeyTab, mod.None)
	c.True(focus().Is(field), "so Tab passes it by and lands on the field, the only tab stop there is")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadingDoesNotSeedTheFocus verifies that a window with nothing focused starts the person off at a real tab stop
// rather than at the heading beside it, whichever end the move of the focus starts from, and that a window holding
// nothing else still starts them at the heading, since a screen reader has to begin somewhere inside the content.
func TestHeadingDoesNotSeedTheFocus(t *testing.T) {
	c := check.New(t)
	var heading, trailing, lone *Label
	var field *Field
	var wnd, headingOnly *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			heading = axNewHeadingLabel("Network", 2)
			field = NewField()
			// A heading at each end, so that neither the forward seeding nor the backward one can reach a real tab
			// stop by simply taking the panel at its own end of the order.
			trailing = axNewHeadingLabel("Advanced", 2)
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(heading)
			column.AddChild(field)
			column.AddChild(trailing)
			wnd = axNewTestWindow(t, "heading seeding", geom.NewRect(10, 10, 500, 300), column)

			lone = axNewHeadingLabel("Alone", 1)
			headingOnly = axNewTestWindow(t, "heading only", geom.NewRect(60, 60, 300, 200), lone)
		}))
	c.NotNil(wnd)
	c.NotNil(headingOnly)
	screen.Sync()

	saved := axHeadingsTakeFocus
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })
	screen.EnableAccessibility()

	var seeded, seededBackward, seededAlone, seededAloneBackward *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.FocusNext()
		seeded = wnd.CurrentFocus()
		wnd.SetFocus(nil)
		wnd.FocusPrevious()
		seededBackward = wnd.CurrentFocus()
		headingOnly.SetFocus(nil)
		headingOnly.FocusNext()
		seededAlone = headingOnly.CurrentFocus()
		headingOnly.SetFocus(nil)
		headingOnly.FocusPrevious()
		seededAloneBackward = headingOnly.CurrentFocus()
	})
	c.True(seeded.Is(field), "the person is put in the first real tab stop, not on the heading above it")
	c.True(seededBackward.Is(field), "and moving the other way passes over the trailing heading just the same")
	c.True(seededAlone.Is(lone), "a window holding only a heading still starts a screen reader inside its content")
	c.True(seededAloneBackward.Is(lone), "from either end")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestDocumentSeedsTheFocus verifies the rest of the seeding rule FocusNext documents. A document can take the focus
// only for an assistive technology's sake — see Panel.axFocusable — so it is preferred over a heading but passed over
// for a real tab stop: a dialog whose first element is a Markdown explanation above its fields must still open with
// the person in the first field, where what they type goes somewhere, while a window holding nothing but the document
// hands it the focus, since that is where a screen reader has to begin. Both answers of axReadersFollowFocus are
// pinned rather than left to the platform the test happens to run on.
func TestDocumentSeedsTheFocus(t *testing.T) {
	c := check.New(t)
	var md, lone *Markdown
	var button *Button
	var wnd, docOnly *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			md = NewMarkdown(false)
			md.SetContent("# Title\n\nSome body text.\n", 400)
			button = NewButton()
			button.SetTitle("OK")
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(md)
			column.AddChild(button)
			wnd = axNewTestWindow(t, "document seeding", geom.NewRect(10, 10, 500, 300), column)

			lone = NewMarkdown(false)
			lone.SetContent("# Alone\n\nNothing else to focus.\n", 400)
			docOnly = axNewTestWindow(t, "document only", geom.NewRect(60, 60, 300, 200), lone)
		}))
	c.NotNil(wnd)
	c.NotNil(docOnly)
	screen.Sync()

	saved := axReadersFollowFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axReadersFollowFocus = saved })
	screen.EnableAccessibility()
	c.True(IsAccessibilityActive())

	for _, follows := range []bool{true, false} {
		var seeded, seededBackward, seededAlone *Panel
		screen.Do(func() {
			axReadersFollowFocus = follows
			wnd.SetFocus(nil)
			wnd.FocusNext()
			seeded = wnd.CurrentFocus()
			wnd.SetFocus(nil)
			wnd.FocusPrevious()
			seededBackward = wnd.CurrentFocus()
			docOnly.SetFocus(nil)
			docOnly.FocusNext()
			seededAlone = docOnly.CurrentFocus()
		})
		c.True(seeded.Is(button), "the person is put in the first real tab stop, not in the document above it: %v",
			follows)
		c.True(seededBackward.Is(button), "the last thing that can take the focus is the button either way: %v",
			follows)
		if follows {
			c.True(seededAlone.Is(lone), "a window holding nothing but a document starts a screen reader inside it")
		} else {
			c.True(seededAlone == nil, "where a document never takes the focus, there is nothing there to focus")
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestSetFocusOnContainerSkipsHeading verifies that resolving a container to something that can hold the keyboard
// focus follows the same rule the seeding of an empty window does: a real tab stop is preferred over a heading, which
// can take the focus only for a screen reader's sake. Window.SetFocus on a container comes through
// Panel.FirstFocusableChild, and DockContainer.AcquireFocus takes exactly that path on every tab switch, so a
// container whose first focusable descendant is a heading would otherwise land the keyboard on the heading rather than
// on the first real tab stop of the panel that was just switched to. A container holding nothing else still lands on
// the heading, since a screen reader has to begin somewhere inside the content.
func TestSetFocusOnContainerSkipsHeading(t *testing.T) {
	c := check.New(t)
	var heading, lone *Label
	var field *Field
	var column, headingOnly *Panel
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			heading = axNewHeadingLabel("Network", 2)
			field = NewField()
			column = NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(heading)
			column.AddChild(field)

			lone = axNewHeadingLabel("Alone", 1)
			headingOnly = NewPanel()
			headingOnly.SetLayout(&FlexLayout{Columns: 1})
			headingOnly.AddChild(lone)

			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1})
			content.AddChild(column)
			content.AddChild(headingOnly)
			wnd = axNewTestWindow(t, "container focus", geom.NewRect(10, 10, 500, 300), content)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	saved := axHeadingsTakeFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })
	screen.EnableAccessibility()

	var landed, landedLast, landedAlone *Panel
	screen.Do(func() {
		wnd.SetFocus(nil)
		wnd.SetFocus(column)
		landed = wnd.CurrentFocus()
		landedLast = column.LastFocusableChild()
		wnd.SetFocus(nil)
		wnd.SetFocus(headingOnly)
		landedAlone = wnd.CurrentFocus()
	})
	c.True(landed.Is(field), "the keyboard lands on the first real tab stop, not on the heading above it")
	c.True(landedLast.Is(field), "and asking from the other end passes the heading over just the same")
	c.True(landedAlone.Is(lone), "a container holding only a heading still puts a screen reader inside its content")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestEmptyHeadingNeverTakesFocus verifies that a label with nothing to announce does not become a place a screen
// reader can land merely because the application marked it up as a heading, and that one which draws no text but was
// given a name of its own does. The first is spacing, and says nothing when it is jumped to, so Orca's structural
// navigation must not be able to stop on it; the second is announced by its name, so a jump to it has somewhere to
// land and must not be left in the list of headings with no way to reach it. See Panel.axTakesFocus, which is what
// decides both. A session owns most of the package's mutable globals while it runs, so this may not call t.Parallel.
func TestEmptyHeadingNeverTakesFocus(t *testing.T) {
	c := check.New(t)
	var empty, named, titled *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			empty = NewLabel()
			empty.Accessibility.Role = role.Heading
			empty.Accessibility.Level = 1
			// Draws nothing at all, but the application said what it is, so a screen reader has something to say on
			// landing there.
			named = NewLabel()
			named.Accessibility.Role = role.Heading
			named.Accessibility.Level = 1
			named.Accessibility.Name = "Printers"
			titled = axNewHeadingLabel("Network", 1)
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(empty)
			column.AddChild(named)
			column.AddChild(titled)
			wnd = axNewTestWindow(t, "empty heading focus", geom.NewRect(10, 10, 500, 300), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	saved := axHeadingsTakeFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })
	screen.EnableAccessibility()

	var emptyFocusable, namedFocusable, titledFocusable bool
	screen.Do(func() {
		emptyFocusable = empty.AsPanel().Focusable()
		namedFocusable = named.AsPanel().Focusable()
		titledFocusable = titled.AsPanel().Focusable()
	})
	c.False(emptyFocusable, "a heading with nothing to announce is not somewhere to land")
	c.True(namedFocusable, "one that draws nothing but was named is announced by that name, so the jump can land")
	c.True(titledFocusable, "while the heading beside it, which has text, still is")

	c.NotNil(screen.AccessibilityTree(wnd))
	node := screen.AccessibilityNodeFor(empty)
	c.NotNil(node)
	if node != nil {
		c.False(node.Focusable, "the node says as much")
		c.False(node.Actions.Has(accessibility.Focus))
	}
	namedNode := screen.AccessibilityNodeFor(named)
	c.NotNil(namedNode)
	if namedNode != nil {
		c.Equal(role.Heading, namedNode.Role, "the role the application asked for is left alone")
		c.Equal("Printers", namedNode.Name)
		c.False(namedNode.Ignored, "a heading with a name is worth announcing")
		c.True(namedNode.Focusable, "so the node offers the focus Orca lands with")
		c.True(namedNode.Actions.Has(accessibility.Focus))
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadingWithoutALabelTakesFocus verifies that what decides whether a heading is somewhere a screen reader's
// structural navigation can land is what the published node will announce, not what kind of widget the heading was
// built out of. An application marks up a heading by setting Role and Level on any panel at all — see
// AccessibilityInfo.Level — so a grouping Panel carrying a name, and a Label that draws only a drawable but takes its
// name from its tooltip (see axDescribeStaticContent), are both headings Orca jumps to and both have to be able to
// take the focus it lands with. A panel with nothing to announce still cannot. A session owns most of the package's
// mutable globals while it runs, so this may not call t.Parallel.
func TestHeadingWithoutALabelTakesFocus(t *testing.T) {
	c := check.New(t)
	var group, anonymous *Panel
	var icon *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			// Not a Label at all: a container the application marked up as the heading of the group it holds.
			group = NewPanel()
			group.SetLayout(&FlexLayout{Columns: 1})
			group.Accessibility.Role = role.Heading
			group.Accessibility.Level = 1
			group.Accessibility.Name = "Network"
			inner := NewLabel()
			inner.SetTitle("Inside the group")
			group.AddChild(inner)

			// Marked up as a heading, but says nothing at all when it is landed on.
			anonymous = NewPanel()
			anonymous.SetLayout(&FlexLayout{Columns: 1})
			anonymous.Accessibility.Role = role.Heading
			anonymous.Accessibility.Level = 1

			// Draws no text, so its name comes from its tooltip, which is written onto the node before the role the
			// application asked for is looked at.
			icon = NewLabel()
			icon.Drawable = &axLabelTestDrawable{size: geom.NewSize(16, 16)}
			icon.Tooltip = NewTooltipWithText("Printers")
			icon.Accessibility.Role = role.Heading
			icon.Accessibility.Level = 2

			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(group)
			column.AddChild(anonymous)
			column.AddChild(icon)
			wnd = axNewTestWindow(t, "non-label heading focus", geom.NewRect(10, 10, 500, 300), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()

	saved := axHeadingsTakeFocus
	// Written on the UI thread, which is the only thread that reads it, since focus and snapshots both live there.
	defer screen.Do(func() { axHeadingsTakeFocus = saved })
	screen.Do(func() { axHeadingsTakeFocus = true })
	screen.EnableAccessibility()

	var groupFocusable, anonymousFocusable, iconFocusable bool
	screen.Do(func() {
		groupFocusable = group.Focusable()
		anonymousFocusable = anonymous.Focusable()
		iconFocusable = icon.AsPanel().Focusable()
	})
	c.True(groupFocusable, "a heading that is not a Label still has to take the focus Orca lands with")
	c.False(anonymousFocusable, "one with nothing to announce is still not somewhere to land")
	c.True(iconFocusable, "and one named only by its tooltip is announced by that name, so the jump can land")

	c.NotNil(screen.AccessibilityTree(wnd))
	groupNode := screen.AccessibilityNodeFor(group)
	c.NotNil(groupNode)
	if groupNode != nil {
		c.Equal(role.Heading, groupNode.Role)
		c.Equal("Network", groupNode.Name)
		c.True(groupNode.Focusable, "the node offers the focus")
		c.True(groupNode.Actions.Has(accessibility.Focus))
	}
	iconNode := screen.AccessibilityNodeFor(icon)
	c.NotNil(iconNode)
	if iconNode != nil {
		c.Equal(role.Heading, iconNode.Role)
		c.Equal("Printers", iconNode.Name, "the tooltip is what names a heading that draws no text")
		c.True(iconNode.Focusable, "so it offers the focus too")
		c.True(iconNode.Actions.Has(accessibility.Focus))
	}

	// A screen reader asking for the focus on the heading it jumped to gets it, which is the whole point of the flag.
	if groupNode != nil {
		c.True(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   groupNode.ID,
			Action: accessibility.Focus,
		}))
		var focus *Panel
		screen.Do(func() { focus = wnd.CurrentFocus() })
		c.True(focus.Is(group), "and the focus lands on it")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// axShadowedLabel is a widget built the documented way — by embedding a *Label and pointing Self at itself — which
// draws other words than the label beneath it holds, and draws them somewhere other than where a plain label would,
// and shadows axTextLine to say so, much as DefaultTableColumnHeader does for a column shrunk by its sort indicator.
// What it reports is its own runes and a fixed rectangle, so that a description taken from the label's own answer
// instead is unmistakable.
type axShadowedLabel struct {
	*Label
}

// axShadowedLabelBounds is where axShadowedLabel claims its text was drawn.
var axShadowedLabelBounds = geom.NewRect(7, 11, 23, 29)

// axShadowedLabelDrawn is what axShadowedLabel claims it actually drew, which is not what the label beneath it holds.
const axShadowedLabelDrawn = "Shad\u2026"

func (s *axShadowedLabel) axTextLine() (runes []rune, decorations []*TextDecoration, line accessibility.Line) {
	runes, decorations, line = axStaticTextLine(s.ContentRect(false), s.HAlign, s.VAlign, s.Font,
		NewText(axShadowedLabelDrawn, &TextDecoration{Font: s.Font}), nil, s.Side, s.Gap)
	line.Bounds = axShadowedLabelBounds
	return runes, decorations, line
}

// TestLabelPublishesShadowedTextLine verifies that both the runes and the line a label publishes for a screen reader to
// read by are asked for through Self, so that a widget embedding a *Label and answering for its own text has that
// answer published rather than the one the label would have given — text and line boundaries alike, since an adapter
// derives every offset it uses from the two together. See Label.ProvideAccessibility and axTextLiner. A session owns
// most of the package's mutable globals while it runs, so this may not call t.Parallel.
func TestLabelPublishesShadowedTextLine(t *testing.T) {
	c := check.New(t)
	var shadowed, plain *Label
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 600, Height: 600},
		StartupFinishedCallback(func() {
			shadowed = NewLabel()
			shadowed.SetTitle("Shadowed")
			shadowed.Self = &axShadowedLabel{Label: shadowed}
			plain = NewLabel()
			plain.SetTitle("Plain")
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1})
			column.AddChild(shadowed)
			column.AddChild(plain)
			wnd = axNewTestWindow(t, "shadowed text line", geom.NewRect(10, 10, 500, 300), column)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	screen.Sync()
	screen.EnableAccessibility()
	c.NotNil(screen.AccessibilityTree(wnd))

	node := screen.AccessibilityNodeFor(shadowed)
	c.NotNil(node)
	if node != nil {
		c.NotNil(node.Text, "a label showing text reports the line it was drawn on")
		if node.Text != nil {
			c.Equal(1, len(node.Text.Lines), "static text occupies exactly one line")
			c.Equal(axShadowedLabelDrawn, node.Text.Text,
				"the text is the runes the widget says it drew, not the ones the label beneath it holds")
			if len(node.Text.Lines) == 1 {
				c.Equal(axShadowedLabelBounds, node.Text.Lines[0].Bounds,
					"the line comes from the widget's own answer, not from the label beneath it")
				c.Equal(len([]rune(axShadowedLabelDrawn)), node.Text.Lines[0].End,
					"and the line covers those same runes, so every offset an adapter derives addresses them")
			}
		}
	}

	plainNode := screen.AccessibilityNodeFor(plain)
	c.NotNil(plainNode)
	if plainNode != nil {
		c.NotNil(plainNode.Text, "a plain label still carries the text it drew")
		if plainNode.Text != nil {
			c.Equal("Plain", plainNode.Text.Text, "a label that answers for itself reports its own words")
			c.Equal(1, len(plainNode.Text.Lines))
			if len(plainNode.Text.Lines) == 1 {
				c.True(plainNode.Text.Lines[0].Bounds != axShadowedLabelBounds,
					"a label that answers for itself is unaffected")
			}
		}
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
