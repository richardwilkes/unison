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

func TestNewCursor(t *testing.T) {
	cursor := NewCursor([]*image.NRGBA{filledNRGBA(16, 16)}, geom.NewPoint(2, 3), geom.NewSize(8, 8))
	if cursor == 0 {
		t.Fatal("NewCursor returned 0")
	}
	defer cursor.Release()
	WithPool(func() {
		if pt := objc.Send[NSPoint](objc.ID(cursor), Sel("hotSpot")); pt.X != 2 || pt.Y != 3 {
			t.Errorf("hot spot = %v, want {2 3}", pt)
		}
		cursorImg := objc.ID(cursor).Send(Sel("image"))
		if cursorImg == 0 {
			t.Fatal("cursor has no image")
		}
		if size := objc.Send[NSSize](cursorImg, Sel("size")); size.Width != 8 || size.Height != 8 {
			t.Errorf("cursor image size = %v, want {8 8}", size)
		}
	})
}

// TestNewCursorMultipleReps covers the vector-backed path: every rasterization handed to NewCursor must become a
// representation of a single image whose point size is the logical size, since that is what lets AppKit pick the one
// matching the display's backing scale. A fractional hot spot must survive as-is, too, rather than being truncated.
func TestNewCursorMultipleReps(t *testing.T) {
	cursor := NewCursor([]*image.NRGBA{filledNRGBA(24, 24), filledNRGBA(48, 48)}, geom.NewPoint(11.5, 12.5),
		geom.NewSize(24, 24))
	if cursor == 0 {
		t.Fatal("NewCursor returned 0")
	}
	defer cursor.Release()
	WithPool(func() {
		if pt := objc.Send[NSPoint](objc.ID(cursor), Sel("hotSpot")); pt.X != 11.5 || pt.Y != 12.5 {
			t.Errorf("hot spot = %v, want {11.5 12.5}", pt)
		}
		cursorImg := objc.ID(cursor).Send(Sel("image"))
		if cursorImg == 0 {
			t.Fatal("cursor has no image")
		}
		if size := objc.Send[NSSize](cursorImg, Sel("size")); size.Width != 24 || size.Height != 24 {
			t.Errorf("cursor image size = %v, want {24 24}", size)
		}
		reps := cursorImg.Send(Sel("representations"))
		count := NSArrayCount(reps)
		if count != 2 {
			t.Fatalf("cursor image has %d representations, want 2", count)
		}
		for i, want := range []int64{24, 48} {
			rep := NSArrayObjectAt(reps, uint64(i))
			if pixels := objc.Send[int64](rep, Sel("pixelsWide")); pixels != want {
				t.Errorf("representation %d pixelsWide = %d, want %d", i, pixels, want)
			}
			if size := objc.Send[NSSize](rep, Sel("size")); size.Width != 24 || size.Height != 24 {
				t.Errorf("representation %d size = %v, want {24 24}", i, size)
			}
		}
	})
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
