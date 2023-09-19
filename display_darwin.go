// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/go-gl/glfw/v3.3/glfw"

func convertMonitorToDisplay(monitor *glfw.Monitor) *Display {
	x, y := monitor.GetPos()
	vidMode := monitor.GetVideoMode()
	workX, workY, workWidth, workHeight := monitor.GetWorkarea()
	sx, sy := monitor.GetContentScale()
	mmx, mmy := monitor.GetPhysicalSize()
	display := &Display{
		Name: monitor.GetName(),
		Frame: Rect{
			Point: Point{X: float32(x), Y: float32(y)},
			Size:  Size{Width: float32(vidMode.Width), Height: float32(vidMode.Height)},
		},
		Usable: Rect{
			Point: Point{X: float32(workX), Y: float32(workY)},
			Size:  Size{Width: float32(workWidth), Height: float32(workHeight)},
		},
		ScaleX:      sx,
		ScaleY:      sy,
		RefreshRate: vidMode.RefreshRate,
		WidthMM:     mmx,
		HeightMM:    mmy,
	}
	return display
}
