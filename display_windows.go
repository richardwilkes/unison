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
	"sync"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/w32"
)

var (
	w32MonitorCallbackPtr = w32.NewEnumDisplayMonitorsCallback(monitorCallback)
	// w32DisplaysLock serializes use of the w32Displays scratch slice. Display queries are normally made on the UI
	// thread, but they are also reachable from other goroutines (e.g. background image loading), so the enumeration
	// and the scratch slice handoff must not be allowed to interleave.
	w32DisplaysLock sync.Mutex
	w32Displays     []*Display
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
	return w32EnumDisplays(func() {
		w32.EnumDisplayMonitors(0, nil, w32MonitorCallbackPtr, 0)
	})
}

// w32EnumDisplays invokes enum — which must synchronously deliver its results by appending to w32Displays, as the
// EnumDisplayMonitors callback does — while holding the lock that guards the scratch slice, then takes ownership of
// the accumulated result, leaving the scratch slice empty for the next caller.
func w32EnumDisplays(enum func()) []*Display {
	w32DisplaysLock.Lock()
	defer w32DisplaysLock.Unlock()
	w32Displays = nil
	enum()
	displays := w32Displays
	w32Displays = nil
	return displays
}

func monitorCallback(monitor w32.HMONITOR, _hdc w32.HDC, _bounds w32.RECT, _lParam uintptr) bool {
	w32Displays = append(w32Displays, monitorInfo(monitor))
	return true
}

func monitorInfo(monitor w32.HMONITOR) *Display {
	var display Display
	var info w32.MONITORINFO
	if w32.GetMonitorInfoW(monitor, &info) {
		display.Frame = rectFromW32Rect(info.Monitor)
		display.Usable = rectFromW32Rect(info.Work)
		display.Primary = (info.Flags & w32.MONITORINFOF_PRIMARY) != 0
		sx, sy := w32.GetDpiForMonitor(monitor, w32.MDT_EFFECTIVE_DPI)
		display.PPI = int(sx)
		display.Scale = geom.NewPoint(float32(sx)/96.0, float32(sy)/96.0)
	}
	return &display
}

func rectFromW32Rect(r w32.RECT) geom.Rect {
	return geom.NewRect(float32(r.Left), float32(r.Top), float32(r.Right-r.Left), float32(r.Bottom-r.Top))
}
