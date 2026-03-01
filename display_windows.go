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
	"github.com/richardwilkes/unison/internal/w32"
)

var (
	monitorCallbackPtr = w32.NewEnumDisplayMonitorsCallback(monitorCallback)
	displays           []*Display
)

// PrimaryDisplay returns the primary display. This is usually the display where elements like the Windows task bar or
// the macOS menu bar is located.
func PrimaryDisplay() *Display {
	for _, d := range AllDisplays() {
		if d.Primary {
			return d
		}
	}
	return nil
}

// AllDisplays returns all currently active displays.
func AllDisplays() []*Display {
	displays = nil
	w32.EnumDisplayMonitors(0, nil, monitorCallbackPtr, 0)
	return displays
}

func monitorCallback(monitor w32.HMONITOR, _hdc w32.HDC, _bounds w32.RECT, _lParam uintptr) bool {
	var info w32.MONITORINFO
	if w32.GetMonitorInfoW(monitor, &info) {
		var display Display
		display.Frame = rectFromW32Rect(info.Monitor)
		display.Usable = rectFromW32Rect(info.Work)
		display.Primary = (info.Flags & w32.MONITORINFOF_PRIMARY) != 0
		sx, sy := w32.GetDpiForMonitor(monitor, w32.MDT_EFFECTIVE_DPI)
		display.PPI = int(sx)
		display.Scale = geom.NewPoint(float32(sx)/96.0, float32(sy)/96.0)
		displays = append(displays, &display)
	}
	return true
}

func rectFromW32Rect(r w32.RECT) geom.Rect {
	return geom.NewRect(float32(r.Left), float32(r.Top), float32(r.Right-r.Left), float32(r.Bottom-r.Top))
}

func w32RectFromRect(r geom.Rect) w32.RECT {
	return w32.RECT{
		Left:   int32(r.X),
		Top:    int32(r.Y),
		Right:  int32(r.Right()),
		Bottom: int32(r.Bottom()),
	}
}
