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
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/x11"
)

func apiPrimaryDisplay() *Display {
	displays := AllDisplays()
	for _, d := range displays {
		if d.Primary {
			return d
		}
	}
	if len(displays) > 0 {
		return displays[0]
	}
	return nil
}

func apiAllDisplays() []*Display {
	scale, err := x11Conn.ContentScale()
	if err != nil {
		// scale will be 1 if an error occurred
		errs.Log(err)
	}
	root := x11Conn.RootWindow()
	var m []x11.Monitor
	if m, err = x11Conn.ExtRandr.GetMonitors(root, true); err == nil && len(m) != 0 {
		displays := make([]*Display, len(m))
		for i := range m {
			frame := geom.NewRect(float32(m[i].X), float32(m[i].Y), float32(m[i].Width), float32(m[i].Height))
			displays[i] = x11NewDisplay(frame, x11Conn.MonitorWorkArea(root, frame), scale,
				int(float64(m[i].Width)/(float64(m[i].WidthMM)/25.4)), m[i].Primary)
		}
		return displays
	}
	// Fall back to the root window if nothing is being reported by RandR.
	screen := x11Conn.Roots[x11Conn.DefaultScreen]
	frame := geom.NewRect(0, 0, float32(screen.WidthInPixels), float32(screen.HeightInPixels))
	return []*Display{
		x11NewDisplay(frame, x11Conn.MonitorWorkArea(root, frame), scale,
			int(float64(screen.WidthInPixels)/(float64(screen.WidthInMillimeters)/25.4)), true),
	}
}

// x11NewDisplay builds a Display from the frame and usable area reported by the X server, which are in raw pixels,
// converting both into the logical, 1x-scale coordinate space that window rects use. Without this conversion, any
// comparison between window rects and display rects (e.g. in BestDisplayForRect, FitRectOnto, or the fallback path of
// MoveToModalCenter) would mix coordinate spaces whenever the content scale is not 1 (i.e. Xft.dpi is not 96), causing
// windows to be assigned to the wrong display or positioned incorrectly. The PPI is deliberately left in terms of raw
// pixels, since it describes the physical panel.
func x11NewDisplay(rawFrame, rawUsable geom.Rect, scale float32, ppi int, primary bool) *Display {
	return &Display{
		Frame:   x11LogicalRect(rawFrame, scale),
		Usable:  x11LogicalRect(rawUsable, scale),
		Scale:   geom.NewPoint(scale, scale),
		PPI:     ppi,
		Primary: primary,
	}
}

// x11LogicalRect converts a rectangle expressed in raw pixels into the logical, 1x-scale coordinate space.
func x11LogicalRect(r geom.Rect, scale float32) geom.Rect {
	r.X /= scale
	r.Y /= scale
	r.Width /= scale
	r.Height /= scale
	return r
}
