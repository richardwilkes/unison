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
	"github.com/richardwilkes/toolbox/v2/geom"
)

// Display holds information about each available active display.
type Display struct {
	// The position of the display in the global screen coordinate system. Note that some platforms (e.g. Windows) don't
	// use a consistent linear global coordinate system for these rects and instead use the raw pixel counts, which
	// means that the rects may not be in the same coordinate space as the windows, which are normalized to a 1x scale.
	Frame  geom.Rect
	Usable geom.Rect  // The usable area, i.e. the Frame minus the area used by global menu bars or task bars
	Scale  geom.Point // The scale of the content
	// The pixels-per-inch for the display. This may not be accurate, either because the monitor's EDID data is
	// incorrect, or because the driver does not report it accurately.
	PPI     int
	Primary bool
}

// defaultDisplayPPI is the pixels-per-inch assumed when the true value cannot be determined.
const defaultDisplayPPI = 96

// displayPPI computes the pixels-per-inch of a display from its pixel extent and its physical size in millimeters.
// Virtual displays and VMs commonly report a physical size of zero, which would otherwise produce an infinite result,
// so implausible inputs fall back to defaultDisplayPPI.
func displayPPI(pixels, sizeMM float64) int {
	if sizeMM <= 0 {
		return defaultDisplayPPI
	}
	ppi := int(pixels / (sizeMM / 25.4))
	if ppi <= 0 {
		return defaultDisplayPPI
	}
	return ppi
}

// PrimaryDisplay returns the primary display. This is usually the display where elements like the Windows task bar or
// the macOS menu bar is located. It may only be called on the UI thread.
func PrimaryDisplay() *Display {
	return apiPrimaryDisplay()
}

// AllDisplays returns all currently active displays. It may only be called on the UI thread.
func AllDisplays() []*Display {
	return apiAllDisplays()
}

// FitRectOnto returns a rectangle that fits onto this display, trying to preserve its position and size as much as
// possible.
func (d *Display) FitRectOnto(r geom.Rect) geom.Rect {
	if d == nil {
		return r
	}
	if r.Width > d.Usable.Width {
		r.Width = d.Usable.Width
	}
	if r.Height > d.Usable.Height {
		r.Height = d.Usable.Height
	}
	right := d.Usable.Right()
	if r.Right() > right {
		r.X = right - r.Width
	}
	if r.X < d.Usable.X {
		r.X = d.Usable.X
	}
	bottom := d.Usable.Bottom()
	if r.Bottom() > bottom {
		r.Y = bottom - r.Height
	}
	if r.Y < d.Usable.Y {
		r.Y = d.Usable.Y
	}
	return r
}

// BestDisplayForRect returns the display with the greatest overlap with the rectangle, or the primary display if there
// is no overlap.
func BestDisplayForRect(r geom.Rect) *Display {
	var bestArea float32
	var bestDisplay *Display
	for _, display := range AllDisplays() {
		if r.In(display.Usable) {
			return display
		}
		ri := r.Intersect(display.Usable)
		if !ri.Empty() {
			area := ri.Width * ri.Height
			if bestArea < area {
				bestArea = area
				bestDisplay = display
			}
		}
	}
	if bestDisplay == nil {
		return PrimaryDisplay()
	}
	return bestDisplay
}
