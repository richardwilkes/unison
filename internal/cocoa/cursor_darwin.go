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
	"image"
	"math"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// Cursor is a handle to an NSCursor.
type Cursor objc.ID

// NewCursor creates a new custom cursor from the non-premultiplied RGBA pixels of one or more images, or returns 0 if
// the cursor's image cannot be created. Each entry in reps becomes a representation of a single image whose point size
// is logicalSize, so AppKit can draw the one that best matches the backing scale of the display the cursor appears on.
// hotSpot is in logical points, measured from the upper-left corner of the cursor. The returned cursor is an owned
// (+1) reference; balance it with Release. Releasing a cursor while it is the current cursor is safe: AppKit retains
// the current cursor for as long as it is in use.
func NewCursor(reps []*image.NRGBA, hotSpot geom.Point, logicalSize geom.Size) Cursor {
	var cursor Cursor
	WithPool(func() {
		nsImg := newNSImageFromNRGBAReps(reps, int(math.Round(float64(logicalSize.Width))),
			int(math.Round(float64(logicalSize.Height))))
		if nsImg == 0 {
			return
		}
		cursor = Cursor(objc.ID(Cls("NSCursor")).Send(Sel("alloc")).Send(Sel("initWithImage:hotSpot:"),
			nsImg, NSPoint{X: float64(hotSpot.X), Y: float64(hotSpot.Y)}))
		Release(nsImg)
	})
	return cursor
}

// Set makes this cursor the current cursor.
func (c Cursor) Set() {
	objc.ID(c).Send(Sel("set"))
}

// Release releases the cursor. Only cursors returned by NewCursor own a reference; the shared built-in cursors
// (ArrowCursor and friends) are owned by AppKit and must not be released.
func (c Cursor) Release() {
	Release(objc.ID(c))
}

// HideCursor hides the cursor.
func HideCursor() {
	objc.ID(Cls("NSCursor")).Send(Sel("hide"))
}

// ShowCursor shows the cursor if it was hidden.
func ShowCursor() {
	objc.ID(Cls("NSCursor")).Send(Sel("unhide"))
}

// ArrowCursor returns the standard arrow cursor.
func ArrowCursor() Cursor {
	return builtInCursor("arrowCursor")
}

// IBeamCursor returns the standard text-insertion cursor.
func IBeamCursor() Cursor {
	return builtInCursor("IBeamCursor")
}

// CrosshairCursor returns the standard crosshair cursor.
func CrosshairCursor() Cursor {
	return builtInCursor("crosshairCursor")
}

// PointingHandCursor returns the standard pointing-hand cursor.
func PointingHandCursor() Cursor {
	return builtInCursor("pointingHandCursor")
}

// ResizeLeftRightCursor returns the standard horizontal-resize cursor.
func ResizeLeftRightCursor() Cursor {
	return builtInCursor("resizeLeftRightCursor")
}

// ResizeUpDownCursor returns the standard vertical-resize cursor.
func ResizeUpDownCursor() Cursor {
	return builtInCursor("resizeUpDownCursor")
}

// builtInCursor returns one of NSCursor's shared cursor singletons. The returned handle is owned by AppKit and lives
// for the life of the process; it is not retained here and must not be released. (The bridge previously retained on
// every call and no caller ever balanced that, growing the singletons' retain counts on every mouse enter/exit.)
func builtInCursor(selName string) Cursor {
	var cursor Cursor
	WithPool(func() {
		cursor = Cursor(objc.ID(Cls("NSCursor")).Send(Sel(selName)))
	})
	return cursor
}
