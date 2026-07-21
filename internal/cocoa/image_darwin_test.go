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
	"image"
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

// repBitmapData returns the bitmap bytes of img's single representation, or nil (with an error reported) if the
// representation count is not 1.
func repBitmapData(t *testing.T, img objc.ID, size int) []byte {
	t.Helper()
	reps := img.Send(Sel("representations"))
	if count := NSArrayCount(reps); count != 1 {
		t.Errorf("image has %d representations, want 1", count)
		return nil
	}
	bitmap := objc.Send[*byte](NSArrayObjectAt(reps, 0), Sel("bitmapData"))
	return unsafe.Slice(bitmap, size)
}

// TestNewNSImageFromNRGBATight covers the fast path: a whole image with a tight stride must be passed through
// unchanged.
func TestNewNSImageFromNRGBATight(t *testing.T) {
	WithPool(func() {
		src := image.NewNRGBA(image.Rect(0, 0, 4, 3))
		for i := range src.Pix {
			src.Pix[i] = byte(i * 5)
		}
		img := newNSImageFromNRGBA(src, 8, 6)
		if img == 0 {
			t.Fatal("newNSImageFromNRGBA returned 0")
		}
		defer Release(img)
		if size := objc.Send[NSSize](img, Sel("size")); size.Width != 8 || size.Height != 6 {
			t.Errorf("image size = %v, want {8 6}", size)
		}
		if got := repBitmapData(t, img, len(src.Pix)); got != nil && !bytes.Equal(got, src.Pix) {
			t.Error("bitmap data does not match the source pixels")
		}
	})
}

// TestNewNSImageFromNRGBASubImage is the regression test for the stride/origin bug: NewCursor and
// BeginDraggingSession used to pass img.Pix with dimensions taken from img.Rect, so a sub-image (non-zero Rect.Min,
// stride wider than the visible width) garbled every row after the first. The rows must be repacked from the
// sub-image's origin, honoring the parent stride.
func TestNewNSImageFromNRGBASubImage(t *testing.T) {
	WithPool(func() {
		base := image.NewNRGBA(image.Rect(0, 0, 7, 6))
		for i := range base.Pix {
			base.Pix[i] = byte(i)
		}
		sub, ok := base.SubImage(image.Rect(2, 1, 6, 5)).(*image.NRGBA)
		if !ok {
			t.Fatal("SubImage did not return *image.NRGBA")
		}
		const width, height = 4, 4
		img := newNSImageFromNRGBA(sub, width, height)
		if img == 0 {
			t.Fatal("newNSImageFromNRGBA returned 0")
		}
		defer Release(img)
		want := make([]byte, 0, width*height*4)
		for y := 1; y < 1+height; y++ {
			off := base.PixOffset(2, y)
			want = append(want, base.Pix[off:off+width*4]...)
		}
		if got := repBitmapData(t, img, len(want)); got != nil && !bytes.Equal(got, want) {
			t.Error("bitmap data does not match the sub-image's visible rows")
		}
	})
}

func TestNewNSImageWithFormatPremultiplied(t *testing.T) {
	WithPool(func() {
		const actualWidth, actualHeight = 3, 2
		pixels := make([]byte, actualWidth*actualHeight*4)
		for i := range pixels {
			pixels[i] = byte(i * 11)
		}
		img := newNSImageWithFormat(pixels, 3, 2, actualWidth, actualHeight, 0)
		if img == 0 {
			t.Fatal("newNSImageWithFormat returned 0")
		}
		defer Release(img)
		reps := img.Send(Sel("representations"))
		if count := NSArrayCount(reps); count != 1 {
			t.Fatalf("image has %d representations, want 1", count)
		}
		rep := NSArrayObjectAt(reps, 0)
		if format := objc.Send[uint64](rep, Sel("bitmapFormat")); format&nsBitmapFormatAlphaNonpremultiplied != 0 {
			t.Errorf("bitmapFormat %#x has the non-premultiplied alpha bit, want premultiplied", format)
		}
		bitmap := objc.Send[*byte](rep, Sel("bitmapData"))
		if got := unsafe.Slice(bitmap, len(pixels)); !bytes.Equal(got, pixels) {
			t.Error("bitmap data does not match the source pixels")
		}
	})
}
