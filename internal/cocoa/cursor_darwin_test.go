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
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestNewCursorMultipleReps checks that every rasterization handed to NewCursor, one or several, becomes a
// representation of a single image whose point size is the logical size, since that is what lets AppKit pick the one
// matching the display's backing scale. The hot spot must survive as-is, a fractional one included, rather than being
// truncated.
func TestNewCursorMultipleReps(t *testing.T) {
	for _, c := range []struct {
		name    string
		pixels  []int
		hotSpot geom.Point
		size    float32
	}{
		{name: "1x and 2x reps", pixels: []int{24, 48}, hotSpot: geom.NewPoint(11.5, 12.5), size: 24},
		{name: "single 2x rep", pixels: []int{16}, hotSpot: geom.NewPoint(2, 3), size: 8},
	} {
		reps := make([]*image.NRGBA, len(c.pixels))
		for i, p := range c.pixels {
			reps[i] = filledNRGBA(p, p)
		}
		cursor := NewCursor(reps, c.hotSpot, geom.NewSize(c.size, c.size))
		if cursor == 0 {
			t.Errorf("%s: NewCursor returned 0", c.name)
			continue
		}
		WithPool(func() {
			if pt := objc.Send[NSPoint](objc.ID(cursor), Sel("hotSpot")); pt.X != float64(c.hotSpot.X) ||
				pt.Y != float64(c.hotSpot.Y) {
				t.Errorf("%s: hot spot = %v, want %v", c.name, pt, c.hotSpot)
			}
			cursorImg := objc.ID(cursor).Send(Sel("image"))
			if cursorImg == 0 {
				t.Errorf("%s: cursor has no image", c.name)
				return
			}
			if size := objc.Send[NSSize](cursorImg, Sel("size")); size.Width != float64(c.size) ||
				size.Height != float64(c.size) {
				t.Errorf("%s: cursor image size = %v, want {%v %v}", c.name, size, c.size, c.size)
			}
			imgReps := cursorImg.Send(Sel("representations"))
			if count := NSArrayCount(imgReps); count != uint64(len(c.pixels)) {
				t.Errorf("%s: cursor image has %d representations, want %d", c.name, count, len(c.pixels))
				return
			}
			for i, want := range c.pixels {
				rep := NSArrayObjectAt(imgReps, uint64(i))
				if pixels := objc.Send[int64](rep, Sel("pixelsWide")); pixels != int64(want) {
					t.Errorf("%s: representation %d pixelsWide = %d, want %d", c.name, i, pixels, want)
				}
				if size := objc.Send[NSSize](rep, Sel("size")); size.Width != float64(c.size) ||
					size.Height != float64(c.size) {
					t.Errorf("%s: representation %d size = %v, want {%v %v}", c.name, i, size, c.size, c.size)
				}
			}
		})
		cursor.Release()
	}
}

// TestNewCursorNoReps verifies that a cursor cannot be built without any pixels to draw.
func TestNewCursorNoReps(t *testing.T) {
	if cursor := NewCursor(nil, geom.NewPoint(0, 0), geom.NewSize(8, 8)); cursor != 0 {
		cursor.Release()
		t.Error("NewCursor with no representations returned a cursor, want 0")
	}
}

// filledNRGBA returns an opaque white image of the given pixel size.
func filledNRGBA(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	return img
}

func TestBuiltInCursors(t *testing.T) {
	// AppKit populates the shared cursors (arrowCursor, IBeamCursor) only once NSApplication exists; unison always
	// creates the shared application at startup before requesting cursors, so the test does too.
	objc.ID(Cls("NSApplication")).Send(Sel("sharedApplication"))
	for name, get := range map[string]func() Cursor{
		"arrow":           ArrowCursor,
		"iBeam":           IBeamCursor,
		"crosshair":       CrosshairCursor,
		"pointingHand":    PointingHandCursor,
		"resizeLeftRight": ResizeLeftRightCursor,
		"resizeUpDown":    ResizeUpDownCursor,
	} {
		cursor := get()
		if cursor == 0 {
			t.Errorf("%s cursor is 0", name)
			continue
		}
		// The built-ins are AppKit-owned shared singletons: repeated calls must hand back the same object without
		// stacking up retains, and callers must not release them.
		if again := get(); again != cursor {
			t.Errorf("%s cursor is not a stable singleton: %#x vs %#x", name, cursor, again)
		}
	}
}
