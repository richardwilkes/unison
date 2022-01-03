// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var lastPrimaryDisplay *Display

// Display holds information about each available active display.
type Display struct {
	Name        string      // The name of the display.
	Frame       geom32.Rect // The position of the display in the global screen coordinate system.
	Usable      geom32.Rect // The usable area, i.e. the Frame minus the area used by global menu bars or task bars.
	ScaleX      float32     // The horizontal scale of content.
	ScaleY      float32     // The vertical scale of content.
	RefreshRate int         // The refresh rate, in Hz.
}

// PrimaryDisplay returns the primary display.
func PrimaryDisplay() *Display {
	if monitor := glfw.GetPrimaryMonitor(); monitor == nil {
		// On macOS, I've had cases where the monitor list has been emptied after some time has passed. Appears to be a
		// bug in glfw, but we can try to work around it by just using the last primary monitor we found.
		if lastPrimaryDisplay == nil {
			return nil
		}
	} else {
		lastPrimaryDisplay = convertMonitorToDisplay(monitor)
	}
	if lastPrimaryDisplay != nil {
		d := *lastPrimaryDisplay
		return &d
	}
	return nil
}

// AllDisplays returns all displays.
func AllDisplays() []*Display {
	monitors := glfw.GetMonitors()
	displays := make([]*Display, len(monitors))
	for i, monitor := range monitors {
		displays[i] = convertMonitorToDisplay(monitor)
	}
	return displays
}

func convertMonitorToDisplay(monitor *glfw.Monitor) *Display {
	x, y := monitor.GetPos()
	vidMode := monitor.GetVideoMode()
	workX, workY, workWidth, workHeight := monitor.GetWorkarea()
	sx, sy := monitor.GetContentScale()
	display := &Display{
		Name:        monitor.GetName(),
		Frame:       geom32.NewRect(float32(x), float32(y), float32(vidMode.Width), float32(vidMode.Height)),
		Usable:      geom32.NewRect(float32(workX), float32(workY), float32(workWidth), float32(workHeight)),
		ScaleX:      sx,
		ScaleY:      sy,
		RefreshRate: vidMode.RefreshRate,
	}
	if runtime.GOOS != toolbox.MacOS {
		display.Frame.X /= display.ScaleX
		display.Frame.Y /= display.ScaleY
		display.Frame.Width /= display.ScaleX
		display.Frame.Height /= display.ScaleY
		display.Usable.X /= display.ScaleX
		display.Usable.Y /= display.ScaleY
		display.Usable.Width /= display.ScaleX
		display.Usable.Height /= display.ScaleY
	}
	return display
}
