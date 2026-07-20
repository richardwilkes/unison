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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

func TestX11LogicalRect(t *testing.T) {
	c := check.New(t)

	// At 1x, the raw and logical coordinate spaces are identical.
	r := geom.NewRect(10, 20, 300, 400)
	c.Equal(r, x11LogicalRect(r, 1))

	// At 2x, both the origin and the size shrink by the scale factor.
	c.Equal(geom.NewRect(5, 10, 150, 200), x11LogicalRect(r, 2))

	// Non-integer scales (e.g. Xft.dpi of 144 yielding 1.5) divide through as well.
	c.Equal(geom.NewRect(20, 40, 200, 400), x11LogicalRect(geom.NewRect(30, 60, 300, 600), 1.5))
}

func TestX11NewDisplay(t *testing.T) {
	c := check.New(t)

	// A 4K monitor at raw offset (3840,0) with a 32-pixel task bar at the top, running at a 2x content scale. The frame
	// and usable area must come out in the logical space shared with window rects, while the scale, PPI (a physical
	// property), and primary flag pass through unchanged.
	d := x11NewDisplay(geom.NewRect(3840, 0, 3840, 2160), geom.NewRect(3840, 32, 3840, 2128), 2, 163, true)
	c.Equal(geom.NewRect(1920, 0, 1920, 1080), d.Frame)
	c.Equal(geom.NewRect(1920, 16, 1920, 1064), d.Usable)
	c.Equal(geom.NewPoint(2, 2), d.Scale)
	c.Equal(163, d.PPI)
	c.True(d.Primary)
}

// TestX11DisplayLogicalSpaceMatchesWindowRects is the regression test for Linux display rects having been reported in
// raw pixels while window rects are in logical units. With a content scale of 2 (Xft.dpi of 192), the raw display was
// twice the size of the logical coordinate space, so windows positioned beyond the logical display edge still appeared
// to be "on" the display and were never pulled back, and rects on a second monitor intersected the first monitor's
// oversized raw frame, selecting the wrong display.
func TestX11DisplayLogicalSpaceMatchesWindowRects(t *testing.T) {
	c := check.New(t)

	// Two side-by-side 4K monitors at a 2x content scale: logical spaces (0,0,1920,1080) and (1920,0,1920,1080).
	left := x11NewDisplay(geom.NewRect(0, 0, 3840, 2160), geom.NewRect(0, 0, 3840, 2160), 2, 163, true)
	right := x11NewDisplay(geom.NewRect(3840, 0, 3840, 2160), geom.NewRect(3840, 0, 3840, 2160), 2, 163, false)

	// A window hanging past the left display's logical right edge must be pulled back on-screen. Before the fix, the
	// raw usable area extended to 3840, so FitRectOnto (the core of EnsureOnDisplay) left the window untouched at a
	// position that is physically off the display.
	c.Equal(geom.NewRect(1120, 100, 800, 600), left.FitRectOnto(geom.NewRect(1500, 100, 800, 600)))

	// A window on the second monitor lies entirely within its logical usable area and not within the first monitor's,
	// which is what BestDisplayForRect keys off of. Before the fix, this rect was inside the left display's raw frame,
	// so the left display won.
	wnd := geom.NewRect(2000, 100, 800, 600)
	c.True(wnd.In(right.Usable))
	c.False(wnd.Intersects(left.Usable))

	// Centering within the primary display's usable area (MoveToModalCenter's fallback) must produce logical
	// coordinates that remain on the display, rather than the ~2x coordinates the raw frame produced.
	within := left.Usable
	within.X += (within.Width - wnd.Width) / 2
	within.Y += (within.Height - wnd.Height) / 3
	within.Size = wnd.Size
	c.True(within.In(left.Usable))
}
