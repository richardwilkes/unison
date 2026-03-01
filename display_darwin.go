// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	"github.com/richardwilkes/unison/internal/mac"
)

func apiPrimaryDisplay() *Display {
	return convertDarwinDisplay(mac.MainDisplayID())
}

func apiAllDisplays() []*Display {
	displayIDs := mac.ActiveDisplayList()
	result := make([]*Display, 0, len(displayIDs))
	for _, id := range displayIDs {
		if display := convertDarwinDisplay(id); display != nil {
			result = append(result, display)
		}
	}
	return result
}

func convertDarwinDisplay(id mac.DisplayID) *Display {
	if mac.DisplayIsAsleep(id) {
		return nil
	}
	screen := mac.ScreenForDisplayID(id)
	if screen == 0 {
		return nil
	}
	mainDisplayID := mac.MainDisplayID()
	height := mac.DisplayBounds(mainDisplayID).Height
	var display Display
	display.Frame = screen.Frame()
	pixels := screen.ConvertRectToBacking(display.Frame)
	display.Frame.Y = height - display.Frame.Bottom()
	display.Usable = screen.VisibleFrame()
	display.Usable.Y = height - display.Usable.Bottom()
	display.Scale = geom.NewPoint(pixels.Width/display.Frame.Width, pixels.Height/display.Frame.Height)
	sizeMM := mac.DisplayScreenSize(id)
	display.PPI = int(pixels.Width / (sizeMM.Width / 25.4))
	display.Primary = id == mainDisplayID
	return &display
}

func transformCocoaY(y float32) float32 {
	return mac.DisplayBounds(mac.MainDisplayID()).Height - y
}
