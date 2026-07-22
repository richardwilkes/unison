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
	"net/url"
	"testing"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/drag"
)

// countPixels returns the number of pixels in the canvas's pixmap that exactly match the given value.
func countPixels(pix []uint32, value uint32) int {
	count := 0
	for _, p := range pix {
		if p == value {
			count++
		}
	}
	return count
}

// TestDockHeaderDropIndicatorOnEmptyHeader verifies that drawing the drag-insertion indicator on a header with no tabs
// does not panic by indexing tabs[-1] and still renders the indicator at the left edge. An empty header is reachable
// via DockState.apply, which creates a DockContainer with a nil dockable and only populates it with dockables whose
// keys resolve.
func TestDockHeaderDropIndicatorOnEmptyHeader(t *testing.T) {
	c := check.New(t)
	dock := NewDock()
	dc := NewDockContainer(dock, nil)
	c.Equal(0, len(dc.Dockables()))
	header := dc.header
	tabs, _ := header.partition()
	c.Equal(0, len(tabs))
	header.SetFrameRect(geom.NewRect(0, 0, 120, 24))
	// Use plain colors so the rendered output is deterministic regardless of theme state.
	header.BackgroundInk = Black
	header.DropAreaInk = White

	// Baseline: no drag in progress, so only the background is drawn.
	cv, pix := newPixmapCanvas(120, 24)
	header.DefaultDraw(cv, header.ContentRect(true))
	c.Equal(0, countPixels(pix.Pix, 0xFFFFFFFF))

	// DefaultDragUpdated sets dragInsertIndex to len(tabs), which is 0 for an empty header. Before the guard was
	// added, this drew tabs[-1] and panicked on every frame of the drag.
	header.dragInsertIndex = 0
	cv, pix = newPixmapCanvas(120, 24)
	header.DefaultDraw(cv, header.ContentRect(true))
	c.True(countPixels(pix.Pix, 0xFFFFFFFF) > 0, "expected the drop indicator to be drawn")
}

// dockHeaderTestDragInfo is a minimal drag.Info representing a dockable tab drag.
type dockHeaderTestDragInfo struct{}

func (d *dockHeaderTestDragInfo) SourceDragOpMask() drag.Op  { return drag.Move }
func (d *dockHeaderTestDragInfo) DataTypes() []string        { return []string{dockableDataType.UTI} }
func (d *dockHeaderTestDragInfo) HasString() bool            { return false }
func (d *dockHeaderTestDragInfo) HasFilePaths() bool         { return false }
func (d *dockHeaderTestDragInfo) HasURLs() bool              { return false }
func (d *dockHeaderTestDragInfo) HasDataType(dt string) bool { return dt == dockableDataType.UTI }
func (d *dockHeaderTestDragInfo) Text() string               { return "" }
func (d *dockHeaderTestDragInfo) FilePaths() []string        { return nil }
func (d *dockHeaderTestDragInfo) URLs() []*url.URL           { return nil }
func (d *dockHeaderTestDragInfo) Data(_ string) []byte       { return nil }

// newOverflowingDockHeader returns a header for four tabs in the state produced by an overflow re-layout: the first
// and current (last) tabs are visible at the left edge, while the two middle tabs are hidden and retain the frames
// from the earlier, wider layout, which overlap and extend past the visible tabs.
func newOverflowingDockHeader() (header *dockHeader, tabs []*dockTab) {
	d1 := newTestDockable("one")
	d2 := newTestDockable("two")
	d3 := newTestDockable("three")
	d4 := newTestDockable("four")
	_, dc := newTestDockContainer(d1, d2, d3, d4)
	header = dc.header
	header.SetFrameRect(geom.NewRect(0, 0, 320, 24))
	tabs, _ = header.partition()
	tabs[0].Hidden = false
	tabs[0].SetFrameRect(geom.NewRect(0, 0, 50, 20))
	tabs[1].Hidden = true
	tabs[1].SetFrameRect(geom.NewRect(104, 0, 100, 20))
	tabs[2].Hidden = true
	tabs[2].SetFrameRect(geom.NewRect(208, 0, 100, 20))
	tabs[3].Hidden = false
	tabs[3].SetFrameRect(geom.NewRect(54, 0, 50, 20))
	return header, tabs
}

// TestDockHeaderDragInsertIndexIgnoresHiddenTabs verifies that DefaultDragUpdated hit-tests only visible tabs.
// Overflow-hidden tabs keep the frame from when they were last visible, and before the fix those stale rects could
// match first, yielding a wrong insert index for both the insertion marker and the Stack() drop position.
func TestDockHeaderDragInsertIndexIgnoresHiddenTabs(t *testing.T) {
	c := check.New(t)
	header, tabs := newOverflowingDockHeader()
	dragDockable = tabs[0].dockable
	defer func() { dragDockable = nil }()
	info := &dockHeaderTestDragInfo{}

	// Left half of the first visible tab.
	c.Equal(drag.Move, header.DefaultDragUpdated(info, geom.NewPoint(10, 10), 0))
	c.Equal(0, header.dragInsertIndex)

	// Left half of the second visible tab (index 3). The stale frame of hidden tab 1 spans [104, 204), so its center
	// (154) used to capture any X below it and misreport index 1.
	c.Equal(drag.Move, header.DefaultDragUpdated(info, geom.NewPoint(60, 10), 0))
	c.Equal(3, header.dragInsertIndex)

	// Past the right edge of the last visible tab: insert at the end, not at hidden tab 1's stale rect.
	c.Equal(drag.Move, header.DefaultDragUpdated(info, geom.NewPoint(110, 10), 0))
	c.Equal(4, header.dragInsertIndex)
}

// leftmostPixelX returns the smallest x coordinate of a pixel exactly matching the given value, or -1 if none match.
func leftmostPixelX(pix *raster.Pixmap, value uint32) int32 {
	leftmost := int32(-1)
	for y := int32(0); y < pix.Height; y++ {
		for x := int32(0); x < pix.Width; x++ {
			if pix.Pix[int(y)*int(pix.RowPixels)+int(x)] == value {
				if leftmost == -1 || x < leftmost {
					leftmost = x
				}
				break
			}
		}
	}
	return leftmost
}

// TestDockHeaderDropIndicatorAnchorsToVisibleTabs verifies that when the insert index refers to an overflow-hidden
// tab, the insertion marker is drawn against the nearest visible tab rather than the hidden tab's stale frame.
func TestDockHeaderDropIndicatorAnchorsToVisibleTabs(t *testing.T) {
	c := check.New(t)
	header, tabs := newOverflowingDockHeader()
	header.BackgroundInk = Black
	header.DropAreaInk = White
	dragDockable = tabs[0].dockable
	defer func() { dragDockable = nil }()

	// The right half of the first visible tab yields insert index 1, which refers to a hidden tab.
	c.Equal(drag.Move, header.DefaultDragUpdated(&dockHeaderTestDragInfo{}, geom.NewPoint(30, 10), 0))
	c.Equal(1, header.dragInsertIndex)

	cv, pix := newPixmapCanvas(320, 24)
	header.DefaultDraw(cv, header.ContentRect(true))
	x := leftmostPixelX(pix, 0xFFFFFFFF)
	c.True(x >= 0, "expected the drop indicator to be drawn")
	// The next visible tab (index 3) starts at x=54, so the marker belongs just left of it. The stale frame of hidden
	// tab 1 starts at x=104, so anything at or beyond it means the marker was anchored to the stale rect.
	c.True(x > 40 && x < 60, "expected the drop indicator near x=50, got x=%d", x)
}
