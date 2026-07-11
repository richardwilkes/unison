// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// DisplayID is a CoreGraphics CGDirectDisplayID.
type DisplayID = uint32

var (
	displayOnce            sync.Once
	cgGetActiveDisplayList func(maxDisplays uint32, activeDisplays *DisplayID, displayCount *uint32) int32
	cgMainDisplayID        func() DisplayID
	cgDisplayIsAsleep      func(display DisplayID) bool
	cgDisplayBounds        func(display DisplayID) NSRect
	cgDisplayScreenSize    func(display DisplayID) NSSize
	cgDisplayUnitNumber    func(display DisplayID) uint32
)

func ensureDisplayFuncs() {
	displayOnce.Do(func() {
		cg := LoadFramework("CoreGraphics")
		purego.RegisterLibFunc(&cgGetActiveDisplayList, cg, "CGGetActiveDisplayList")
		purego.RegisterLibFunc(&cgMainDisplayID, cg, "CGMainDisplayID")
		purego.RegisterLibFunc(&cgDisplayIsAsleep, cg, "CGDisplayIsAsleep")
		purego.RegisterLibFunc(&cgDisplayBounds, cg, "CGDisplayBounds")
		purego.RegisterLibFunc(&cgDisplayScreenSize, cg, "CGDisplayScreenSize")
		purego.RegisterLibFunc(&cgDisplayUnitNumber, cg, "CGDisplayUnitNumber")
	})
}

// ActiveDisplayList returns the IDs of the displays that are active (drawable), up to a limit of 16 displays. Note
// that sleeping displays are excluded — while the login session is locked with the displays asleep, this list is
// empty even though CGMainDisplayID still reports a main display.
func ActiveDisplayList() []DisplayID {
	ensureDisplayFuncs()
	var displayIDs [16]DisplayID
	var count uint32
	cgGetActiveDisplayList(uint32(len(displayIDs)), &displayIDs[0], &count)
	return displayIDs[:count]
}

// MainDisplayID returns the ID of the main display.
func MainDisplayID() DisplayID {
	ensureDisplayFuncs()
	return cgMainDisplayID()
}

// DisplayIsAsleep returns true if the display is asleep.
func DisplayIsAsleep(id DisplayID) bool {
	ensureDisplayFuncs()
	return cgDisplayIsAsleep(id)
}

// DisplayBounds returns the bounds of the display in global display coordinates (top-left origin).
func DisplayBounds(id DisplayID) geom.Rect {
	ensureDisplayFuncs()
	return RectFromNSRect(cgDisplayBounds(id))
}

// DisplayScreenSize returns the physical size of the display in millimeters.
func DisplayScreenSize(id DisplayID) geom.Size {
	ensureDisplayFuncs()
	return SizeFromNSSize(cgDisplayScreenSize(id))
}
