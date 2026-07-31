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
	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// Screen is a handle to an NSScreen. The instance is owned by AppKit's screen list, so the handle is not retained;
// it remains valid as long as the screen configuration does not change, matching the lifetime the cgo bridge
// provided.
type Screen objc.ID

// ScreenForDisplayID returns the screen whose display unit matches that of the given display ID, or 0 if there is
// none.
func ScreenForDisplayID(id DisplayID) Screen {
	ensureDisplayFuncs()
	var result Screen
	WithPool(func() {
		unitNumber := cgDisplayUnitNumber(id)
		for _, screen := range IDsFromNSArray(objc.ID(Cls("NSScreen")).Send(Sel("screens"))) {
			num := screen.Send(Sel("deviceDescription")).Send(Sel("objectForKey:"), NSStringFromGo("NSScreenNumber"))
			if cgDisplayUnitNumber(objc.Send[DisplayID](num, Sel("unsignedIntValue"))) == unitNumber {
				result = Screen(screen)
				return
			}
		}
	})
	return result
}

// Frame returns the dimensions and location of the screen in global screen coordinates (bottom-left origin).
func (s Screen) Frame() geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(s), Sel("frame")))
}

// VisibleFrame returns the portion of the screen's frame not obscured by the menu bar or Dock.
func (s Screen) VisibleFrame() geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(s), Sel("visibleFrame")))
}

// ConvertRectToBacking converts the rect from the screen's logical coordinate system to its device pixel backing
// store coordinate system.
func (s Screen) ConvertRectToBacking(r geom.Rect) geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(s), Sel("convertRectToBacking:"), NSRectFromRect(r)))
}
