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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/behavior"
)

// resizablePanel returns a panel whose preferred size tracks *w and *h, so a test can grow or shrink its content the
// way a real content panel does when its data changes.
func resizablePanel(w, h *float32) *unison.Panel {
	p := unison.NewPanel()
	p.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
		return geom.Size{}, geom.NewSize(*w, *h), geom.NewSize(100000, 100000)
	})
	return p
}

// TestScrollPanelContentShrinkWhileScrolled guards against a stack-overflow crash: when the content of a scrolled
// ScrollPanel shrinks below the current scroll offset, the blank-space adjustment in DefaultFrameChangeInChildHierarchy
// used to move the content directly while Sync kept restoring it from the stale scroll-bar value. With a column header
// present to re-trigger the frame-change handler, the two recursed until the stack overflowed. The adjustment now flows
// through the scroll bars, so this must settle without recursing and leave the content clamped fully into view. Content
// shrinking in the same layout pass as the view must be clamped against the new size of both.
func TestScrollPanelContentShrinkWhileScrolled(t *testing.T) {
	c := check.New(t)
	scroll := unison.NewScrollPanel()
	contentW, contentH := float32(2000), float32(2000)
	content := resizablePanel(&contentW, &contentH)
	headerW, headerH := float32(2000), float32(20)
	header := resizablePanel(&headerW, &headerH)
	scroll.SetColumnHeader(header)
	scroll.SetContent(content, behavior.Fill, behavior.Fill)
	scroll.SetFrameRect(geom.NewRect(0, 0, 200, 500))
	scroll.ValidateLayout()

	// Scroll far down and to the right so the content is offset in both axes.
	scroll.SetPosition(5000, 5000)
	scroll.ValidateLayout()
	h, v := scroll.Position()
	c.True(h > 0 && v > 0, "expected the content to be scrolled away from the origin")

	// Shrink the content well below the current offset and push the frame change through, the way syncing to a smaller
	// model does. Before the fix this recursed until "fatal error: stack overflow" killed the process.
	contentW, contentH = 100, 100
	frame := content.FrameRect()
	frame.Size = geom.NewSize(100, 100)
	content.SetFrameRect(frame)

	// The content is smaller than the view in both axes, so it must be pulled entirely back into view, and the scroll
	// bars must agree with that position rather than pointing off into the old, larger extent.
	c.Equal(float32(0), content.FrameRect().X, "content should be pulled back into view horizontally")
	c.Equal(float32(0), content.FrameRect().Y, "content should be pulled back into view vertically")
	h, v = scroll.Position()
	c.Equal(float32(0), h, "the horizontal scroll bar should have followed the content back to the origin")
	c.Equal(float32(0), v, "the vertical scroll bar should have followed the content back to the origin")

	// A further layout pass must remain stable.
	scroll.ValidateLayout()
	c.Equal(float32(0), content.FrameRect().Y, "content should stay in view after a subsequent layout")

	// Scrolled to the bottom of a 480-tall view, then the content shrinks and the view loses 100 in one layout pass.
	contentH = 2000
	scroll.MarkForLayoutAndRedraw()
	scroll.ValidateLayout()
	scroll.SetPosition(0, 1520)
	contentH = 1000
	scroll.SetFrameRect(geom.NewRect(0, 0, 200, 400))
	scroll.ValidateLayout()
	_, v = scroll.Position()
	c.Equal(float32(620), v, "content shrunk in the same layout pass as the view should stay scrolled to the bottom")
}

// TestScrollPanelScrollRectIntoViewWithHeaders verifies that a rect lying within the visible part of the content is
// left alone, and one just past the bottom is brought in by exactly its overhang, when headers are present.
// DefaultScrollRectIntoView used to set the header aside a second time — the layout already places the content view
// below it — which treated a strip the header's height along the bottom of the view as hidden, so that selecting a row
// there scrolled the content for no reason and left an assistive technology's idea of where the row was one strip out.
// It also verifies that a rect in a header scrolls only along the axis that header scrolls with the content.
func TestScrollPanelScrollRectIntoViewWithHeaders(t *testing.T) {
	c := check.New(t)
	scroll := unison.NewScrollPanel()
	contentW, contentH := float32(400), float32(2000)
	content := resizablePanel(&contentW, &contentH)
	headerW, headerH := float32(400), float32(20)
	header := resizablePanel(&headerW, &headerH)
	scroll.SetColumnHeader(header)
	rowHeaderW, rowHeaderH := float32(30), float32(2000)
	rowHeader := resizablePanel(&rowHeaderW, &rowHeaderH)
	scroll.SetRowHeader(rowHeader)
	scroll.SetContent(content, behavior.Fill, behavior.Fill)
	scroll.SetFrameRect(geom.NewRect(0, 0, 200, 500))
	scroll.ValidateLayout()
	view := scroll.ContentView().FrameRect()
	c.Equal(float32(20), view.Y, "the content view should sit below the column header")
	c.Equal(float32(480), view.Height, "the content view should fill the rest of the panel")

	// The last twenty rows' worth of the visible content, right against the bottom edge, is in view.
	content.ScrollRectIntoView(geom.NewRect(0, 460, 100, 20))
	_, v := scroll.Position()
	c.Equal(float32(0), v, "a rect that can be seen in full must not be scrolled")

	// One that hangs ten units past the bottom is brought in by those ten units and no more.
	content.ScrollRectIntoView(geom.NewRect(0, 470, 100, 20))
	_, v = scroll.Position()
	c.Equal(float32(10), v, "a rect just past the bottom should be scrolled in by its overhang")

	// And one that is now at the top edge after that scroll stays put too.
	content.ScrollRectIntoView(geom.NewRect(0, 10, 100, 20))
	_, v = scroll.Position()
	c.Equal(float32(10), v, "a rect at the top of the view must not be scrolled")

	// Beside the 30-wide row header, the column header's view is 170 wide; below the column header, the row header's view
	// is 480 tall.
	scroll.SetPosition(0, 300)
	header.ScrollRectIntoView(geom.NewRect(300, 0, 50, 20))
	h, v := scroll.Position()
	c.Equal(float32(180), h, "a rect past the right of the column header should be scrolled in by its overhang")
	c.Equal(float32(300), v, "a rect in the column header must not scroll the content vertically")
	rowHeader.ScrollRectIntoView(geom.NewRect(0, 1000, 30, 20))
	h, v = scroll.Position()
	c.Equal(float32(180), h, "a rect in the row header must not scroll the content horizontally")
	c.Equal(float32(540), v, "a rect past the bottom of the row header should be scrolled in by its overhang")
}

// TestScrollPanelScrollRectIntoViewReachesEnclosingPanel verifies that what a ScrollPanel's view shows of a rect it
// brings into view, whether it had nothing to scroll, could scroll the whole way or only part of it, the rect reaches
// past the panel it is given in, or the rect is in its column header, is brought into the view of the ScrollPanel
// enclosing it as well, and no more than that.
func TestScrollPanelScrollRectIntoViewReachesEnclosingPanel(t *testing.T) {
	// Taller than the inner view, so that its top is to be brought to the top of that view, which the inner panel can do
	// only partway, if at all.
	tall := geom.NewRect(0, 10, 100, 2000)
	for _, tc := range []struct {
		name string
		// rect is the rect to bring into view, in the inner panel's content, or in its column header when inHeader is set.
		rect geom.Rect
		// inHeader gives the inner panel a 20-tall column header.
		inHeader bool
		// innerHeight is how tall the inner panel is made, which 0 leaves as tall as its 100-tall content.
		innerHeight float32
		// border is the depth of an empty border along the top and bottom of the inner panel's content view.
		border float32
		// innerScroll is how far the inner panel is to scroll.
		innerScroll float32
		// outerScroll is how far the outer panel is to scroll, which brings the inner panel, below 1000 units of other
		// content, in from beneath its 300-tall view.
		outerScroll float32
	}{
		{name: "nothing to scroll", rect: tall, outerScroll: 800},
		{name: "a few units to scroll", rect: tall, innerHeight: 95, innerScroll: 5, outerScroll: 795},
		{
			name: "the whole way scrolled", rect: geom.NewRect(0, 80, 100, 10), innerHeight: 50, innerScroll: 40,
			outerScroll: 750,
		},
		{
			name: "taller than the inner view", rect: geom.NewRect(0, 20, 100, 75), innerHeight: 50, innerScroll: 20,
			outerScroll: 750,
		},
		{
			// Within the inner view, which is taller than the outer one, though past the content.
			name: "past the panel it is given in", rect: geom.NewRect(0, 10, 100, 380), innerHeight: 400,
			outerScroll: 1010,
		},
		{
			name: "covering a bordered inner view", rect: geom.NewRect(0, 0, 100, 100), innerHeight: 100, border: 10,
			outerScroll: 790,
		},
		{name: "in the column header", rect: geom.NewRect(0, 0, 100, 50), inHeader: true, outerScroll: 720},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			contentW, contentH := float32(200), float32(100)
			content := resizablePanel(&contentW, &contentH)
			inner := unison.NewScrollPanel()
			inner.SetContent(content, behavior.Fill, behavior.Unmodified)
			if tc.border != 0 {
				inner.ContentView().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: tc.border, Bottom: tc.border}))
			}
			target := content
			if tc.inHeader {
				headerW, headerH := float32(200), float32(20)
				target = resizablePanel(&headerW, &headerH)
				inner.SetColumnHeader(target)
			}
			if tc.innerHeight != 0 {
				inner.SetLayoutData(&unison.FlexLayoutData{SizeHint: geom.NewSize(200, tc.innerHeight)})
			}
			spacerW, spacerH := float32(200), float32(1000)
			column := unison.NewPanel()
			column.SetLayout(&unison.FlexLayout{Columns: 1})
			column.AddChild(resizablePanel(&spacerW, &spacerH))
			column.AddChild(inner)
			column.AddChild(resizablePanel(&spacerW, &spacerH))
			outer := unison.NewScrollPanel()
			outer.SetContent(column, behavior.Fill, behavior.Fill)
			outer.SetFrameRect(geom.NewRect(0, 0, 200, 300))
			outer.ValidateLayout()

			target.ScrollRectIntoView(tc.rect)
			_, v := inner.Position()
			c.Equal(tc.innerScroll, v, "how far the inner panel scrolled")
			_, v = outer.Position()
			c.Equal(tc.outerScroll, v, "how far the outer panel scrolled")
		})
	}
}

// TestScrollPanelScrollsWithinViewBorders verifies that the content and each header sit inside the border of the view
// they are seen through: at the inner top-left corner when unscrolled and against the inner bottom-right corner when
// scrolled the whole way. A frame change within the content must then scroll only to remove blank space past it, even
// once the content view's border has moved, and a border moved within its view must move what the view shows with it.
func TestScrollPanelScrollsWithinViewBorders(t *testing.T) {
	c := check.New(t)
	scroll := unison.NewScrollPanel()
	contentW, contentH := float32(400), float32(2000)
	content := resizablePanel(&contentW, &contentH)
	child := unison.NewPanel()
	content.AddChild(child)
	headerW, headerH := float32(400), float32(20)
	header := resizablePanel(&headerW, &headerH)
	scroll.SetColumnHeader(header)
	rowHeaderW, rowHeaderH := float32(30), float32(2000)
	rowHeader := resizablePanel(&rowHeaderW, &rowHeaderH)
	scroll.SetRowHeader(rowHeader)
	scroll.SetContent(content, behavior.Unmodified, behavior.Unmodified)
	scroll.ContentView().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 10, Left: 5, Bottom: 20, Right: 15}))
	scroll.ColumnHeaderView().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 3, Left: 5, Bottom: 4, Right: 15}))
	scroll.RowHeaderView().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 10, Left: 2, Bottom: 20, Right: 6}))
	scroll.SetFrameRect(geom.NewRect(0, 0, 200, 500))
	scroll.ValidateLayout()

	views := []struct {
		view  *unison.Panel
		panel *unison.Panel
		name  string
	}{
		{name: "content", view: scroll.ContentView(), panel: content},
		{name: "column header", view: scroll.ColumnHeaderView(), panel: header},
		{name: "row header", view: scroll.RowHeaderView(), panel: rowHeader},
	}
	for _, one := range views {
		c.Equal(one.view.ContentRect(false).Point, one.panel.FrameRect().Point, one.name+", unscrolled")
	}
	scroll.SetPosition(100000, 100000)
	for _, one := range views {
		c.Equal(one.view.ContentRect(false).BottomRight(), one.panel.FrameRect().BottomRight(),
			one.name+", scrolled the whole way")
	}

	h, v := scroll.Position()
	child.SetFrameRect(geom.NewRect(0, 0, 10, 10))
	h2, v2 := scroll.Position()
	c.Equal(geom.NewPoint(h, v), geom.NewPoint(h2, v2), "a frame change within the content must not scroll")

	// A border change leaves the content's position stale until the next Sync. Here the view loses 3 units on the left
	// and gains 3 at the bottom, which are left blank below the content.
	scroll.SetPosition(50, v)
	scroll.ContentView().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 10, Left: 8, Bottom: 17, Right: 15}))
	child.SetFrameRect(geom.NewRect(0, 0, 20, 20))
	h2, v2 = scroll.Position()
	c.Equal(geom.NewPoint(50, v-3), geom.NewPoint(h2, v2),
		"a frame change within the content must scroll only to remove blank space, though the view's border moved")

	scroll.SetPosition(0, 0)
	scroll.ColumnHeaderView().SetBorder(unison.NewEmptyBorder(geom.Insets{Top: 4, Left: 5, Bottom: 3, Right: 15}))
	scroll.MarkForLayoutAndRedraw()
	scroll.ValidateLayout()
	c.Equal(geom.NewPoint(5, 4), header.FrameRect().Point, "the column header must follow its view's border")
}

// TestScrollPanelHonorsOwnBorder verifies that a ScrollPanel with a border of its own lays out inside that border.
// LayoutSizes already adds the border insets to the sizes it reports, but PerformLayout used to start from the frame
// rect, so the content view, the headers and the scroll bars were all drawn on top of the border.
func TestScrollPanelHonorsOwnBorder(t *testing.T) {
	c := check.New(t)
	scroll := unison.NewScrollPanel()
	contentW, contentH := float32(500), float32(500)
	content := resizablePanel(&contentW, &contentH)
	headerW, headerH := float32(500), float32(20)
	header := resizablePanel(&headerW, &headerH)
	scroll.SetColumnHeader(header)
	scroll.SetContent(content, behavior.Fill, behavior.Fill)
	scroll.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(10)))
	scroll.SetFrameRect(geom.NewRect(0, 0, 100, 100))
	scroll.ValidateLayout()

	inside := scroll.ContentRect(false)
	c.Equal(geom.NewRect(10, 10, 80, 80), inside)

	// The column header sits at the top of the content rect and the content view fills what is left, with neither
	// straying into the border.
	headerRect := scroll.ColumnHeaderView().FrameRect()
	c.Equal(geom.NewPoint(10, 10), headerRect.Point, "the column header must start inside the border")
	viewRect := scroll.ContentView().FrameRect()
	c.Equal(geom.NewPoint(10, 10+headerRect.Height), viewRect.Point, "the content view must start inside the border")
	c.Equal(float32(80), viewRect.Width, "the content view must not extend past the border")
	c.Equal(inside.Bottom(), viewRect.Bottom(), "the content view must not extend past the border")

	// The scroll bars ride the inner edges of the content view, not the panel's outer edge.
	vBar := scroll.Bar(false)
	c.Equal(inside.Right(), vBar.FrameRect().Right(), "the vertical scroll bar must sit inside the border")
	hBar := scroll.Bar(true)
	c.Equal(inside.Bottom(), hBar.FrameRect().Bottom(), "the horizontal scroll bar must sit inside the border")
}
