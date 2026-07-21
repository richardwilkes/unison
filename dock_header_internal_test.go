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
