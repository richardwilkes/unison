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
// through the scroll bars, so this must settle without recursing and leave the content clamped fully into view.
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
