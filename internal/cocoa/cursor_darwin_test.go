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
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}
	cursor := NewCursor(img, geom.NewPoint(2, 3), geom.NewSize(8, 8))
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

func TestBuiltInCursors(t *testing.T) {
	// AppKit populates the shared cursors (arrowCursor, IBeamCursor) only once NSApplication exists; unison always
	// creates the shared application at startup before requesting cursors, so the test does too.
	objc.ID(Cls("NSApplication")).Send(Sel("sharedApplication"))
	for name, cursor := range map[string]Cursor{
		"arrow":           ArrowCursor(),
		"iBeam":           IBeamCursor(),
		"crosshair":       CrosshairCursor(),
		"pointingHand":    PointingHandCursor(),
		"resizeLeftRight": ResizeLeftRightCursor(),
		"resizeUpDown":    ResizeUpDownCursor(),
	} {
		if cursor == 0 {
			t.Errorf("%s cursor is 0", name)
			continue
		}
		cursor.Release()
	}
}
