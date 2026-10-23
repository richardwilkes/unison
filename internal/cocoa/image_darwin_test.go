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

// TestNewNSImage checks what newNSImage, and newNSImageWithFormat given format 0, build: the logical size, a single
// representation of the actual pixel size, the non-premultiplied alpha bit for newNSImage only, and the source bytes.
func TestNewNSImage(t *testing.T) {
	const actualWidth, actualHeight = 4, 3
	pixels := make([]byte, actualWidth*actualHeight*4)
	for i := range pixels {
		pixels[i] = byte(i * 7)
	}
	for _, c := range []struct {
		build         func() objc.ID
		name          string
		premultiplied bool
	}{
		{
			name:  "newNSImage",
			build: func() objc.ID { return newNSImage(pixels, 8, 6, actualWidth, actualHeight) },
		},
		{
			name:          "newNSImageWithFormat premultiplied",
			build:         func() objc.ID { return newNSImageWithFormat(pixels, 8, 6, actualWidth, actualHeight, 0) },
			premultiplied: true,
		},
	} {
		WithPool(func() {
			img := c.build()
			if img == 0 {
				t.Errorf("%s returned 0", c.name)
				return
			}
			defer Release(img)
			if size := objc.Send[NSSize](img, Sel("size")); size.Width != 8 || size.Height != 6 {
				t.Errorf("%s: image size = %v, want {8 6}", c.name, size)
			}
			reps := img.Send(Sel("representations"))
			if count := NSArrayCount(reps); count != 1 {
				t.Errorf("%s: image has %d representations, want 1", c.name, count)
				return
			}
			rep := NSArrayObjectAt(reps, 0)
			if w := objc.Send[int64](rep, Sel("pixelsWide")); w != actualWidth {
				t.Errorf("%s: pixelsWide = %d, want %d", c.name, w, actualWidth)
			}
			if h := objc.Send[int64](rep, Sel("pixelsHigh")); h != actualHeight {
				t.Errorf("%s: pixelsHigh = %d, want %d", c.name, h, actualHeight)
			}
			format := objc.Send[uint64](rep, Sel("bitmapFormat"))
			if (format&nsBitmapFormatAlphaNonpremultiplied != 0) == c.premultiplied {
				t.Errorf("%s: bitmapFormat %#x has the non-premultiplied alpha bit set = %v, want %v", c.name,
					format, c.premultiplied, !c.premultiplied)
			}
			bitmap := objc.Send[*byte](rep, Sel("bitmapData"))
			if got := unsafe.Slice(bitmap, len(pixels)); !bytes.Equal(got, pixels) {
				t.Errorf("%s: bitmap data does not match the source pixels", c.name)
			}
		})
	}
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

// TestNewNSImageFromNRGBASubImage is the regression test for the stride/origin bug: NewCursor and
// BeginDraggingSession used to pass img.Pix with dimensions taken from img.Rect, so a sub-image (non-zero Rect.Min,
// stride wider than the visible width) garbled every row after the first. The rows must be repacked from the
// sub-image's origin, honoring the parent stride, while a whole image with a tight stride is passed through unchanged.
func TestNewNSImageFromNRGBASubImage(t *testing.T) {
	base := image.NewNRGBA(image.Rect(0, 0, 7, 6))
	for i := range base.Pix {
		base.Pix[i] = byte(i)
	}
	sub, ok := base.SubImage(image.Rect(2, 1, 6, 5)).(*image.NRGBA)
	if !ok {
		t.Fatal("SubImage did not return *image.NRGBA")
	}
	subRows := make([]byte, 0, sub.Rect.Dx()*sub.Rect.Dy()*4)
	for y := sub.Rect.Min.Y; y < sub.Rect.Max.Y; y++ {
		off := base.PixOffset(sub.Rect.Min.X, y)
		subRows = append(subRows, base.Pix[off:off+sub.Rect.Dx()*4]...)
	}
	for _, c := range []struct {
		src           *image.NRGBA
		name          string
		want          []byte
		logicalWidth  int
		logicalHeight int
	}{
		{name: "sub-image", src: sub, logicalWidth: 4, logicalHeight: 4, want: subRows},
		{name: "whole image", src: base, logicalWidth: 14, logicalHeight: 12, want: base.Pix},
	} {
		WithPool(func() {
			img := newNSImageFromNRGBA(c.src, c.logicalWidth, c.logicalHeight)
			if img == 0 {
				t.Errorf("%s: newNSImageFromNRGBA returned 0", c.name)
				return
			}
			defer Release(img)
			if size := objc.Send[NSSize](img, Sel("size")); size.Width != float64(c.logicalWidth) ||
				size.Height != float64(c.logicalHeight) {
				t.Errorf("%s: image size = %v, want {%d %d}", c.name, size, c.logicalWidth, c.logicalHeight)
			}
			if got := repBitmapData(t, img, len(c.want)); got != nil && !bytes.Equal(got, c.want) {
				t.Errorf("%s: bitmap data does not match the source's visible rows", c.name)
			}
		})
	}
}

// TestCopyPixelsToRepNilBitmapData proves the bitmapData nil check: the buffer is allocated lazily and bitmapData
// returns NULL on allocation failure, and copyPixelsToRep must report failure instead of letting unsafe.Slice panic
// on the nil pointer. A stand-in class whose bitmapData returns nil simulates the failure, since a real
// NSBitmapImageRep only does so under memory exhaustion.
func TestCopyPixelsToRepNilBitmapData(t *testing.T) {
	cls, err := objc.RegisterClass("unisonTestNilBitmapRep", Cls("NSObject"), nil, nil, []objc.MethodDef{
		{
			Cmd: Sel("bitmapData"),
			// Declared as returning objc.ID purely so purego accepts the signature; the zero return reaches
			// objc.Send[*byte] as a nil pointer, exactly like NSBitmapImageRep's NULL on allocation failure.
			Fn: func(_ objc.ID, _ objc.SEL) objc.ID { return 0 },
		},
	})
	if err != nil {
		t.Fatalf("unable to register test bitmap rep class: %v", err)
	}
	rep := objc.ID(cls).Send(Sel("new"))
	defer Release(rep)
	if copyPixelsToRep(rep, []byte{1, 2, 3, 4}, 4) {
		t.Error("copyPixelsToRep = true for a rep whose bitmapData is nil, want false")
	}
}
