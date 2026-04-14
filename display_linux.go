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
	for _, d := range AllDisplays() {
		if d.Primary {
			return d
		}
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
			displays[i] = &Display{
				Frame:   frame,
				Usable:  x11Conn.MonitorWorkArea(root, frame),
				Scale:   geom.NewPoint(scale, scale),
				PPI:     int(float64(m[i].Width) / (float64(m[i].WidthMM) / 25.4)),
				Primary: m[i].Primary,
			}
		}
		return displays
	}
	// Fall back to the root window if nothing is being reported by RandR.
	screen := x11Conn.Roots[x11Conn.DefaultScreen]
	frame := geom.NewRect(0, 0, float32(screen.WidthInPixels), float32(screen.HeightInPixels))
	return []*Display{
		{
			Frame:   frame,
			Usable:  x11Conn.MonitorWorkArea(root, frame),
			Scale:   geom.NewPoint(scale, scale),
			PPI:     int(float64(screen.WidthInPixels) / (float64(screen.WidthInMillimeters) / 25.4)),
			Primary: true,
		},
	}
}
