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
	"bytes"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

func TestNewNSImage(t *testing.T) {
	WithPool(func() {
		const actualWidth, actualHeight = 4, 3
		pixels := make([]byte, actualWidth*actualHeight*4)
		for i := range pixels {
			pixels[i] = byte(i * 7)
		}
		img := newNSImage(pixels, 8, 6, actualWidth, actualHeight)
		if img == 0 {
			t.Fatal("newNSImage returned 0")
		}
		defer Release(img)
		if size := objc.Send[NSSize](img, Sel("size")); size.Width != 8 || size.Height != 6 {
			t.Errorf("image size = %v, want {8 6}", size)
		}
		reps := img.Send(Sel("representations"))
		if count := NSArrayCount(reps); count != 1 {
			t.Fatalf("image has %d representations, want 1", count)
		}
		rep := NSArrayObjectAt(reps, 0)
		if w := objc.Send[int64](rep, Sel("pixelsWide")); w != actualWidth {
			t.Errorf("pixelsWide = %d, want %d", w, actualWidth)
		}
		if h := objc.Send[int64](rep, Sel("pixelsHigh")); h != actualHeight {
			t.Errorf("pixelsHigh = %d, want %d", h, actualHeight)
		}
		if format := objc.Send[uint64](rep, Sel("bitmapFormat")); format&nsBitmapFormatAlphaNonpremultiplied == 0 {
			t.Errorf("bitmapFormat %#x lacks the non-premultiplied alpha bit", format)
		}
		bitmap := objc.Send[*byte](rep, Sel("bitmapData"))
		if got := unsafe.Slice(bitmap, len(pixels)); !bytes.Equal(got, pixels) {
			t.Error("bitmap data does not match the source pixels")
		}
	})
}
