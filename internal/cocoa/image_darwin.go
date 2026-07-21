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
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// nsBitmapFormatAlphaNonpremultiplied is AppKit's NSBitmapFormatAlphaNonpremultiplied bitmap format option.
const nsBitmapFormatAlphaNonpremultiplied = 1 << 1

// newNSImage returns an NSImage (with a +1 retain count that the caller must release) of the given logical size,
// backed by a single 32-bit non-premultiplied RGBA bitmap representation of actualWidth x actualHeight device
// pixels copied from pixels. It returns 0 if the bitmap representation cannot be created.
func newNSImage(pixels []byte, logicalWidth, logicalHeight, actualWidth, actualHeight int) objc.ID {
	return newNSImageWithFormat(pixels, logicalWidth, logicalHeight, actualWidth, actualHeight,
		nsBitmapFormatAlphaNonpremultiplied)
}

// newNSImageFromNRGBA is newNSImage for a non-premultiplied NRGBA image, honoring the image's stride and sub-image
// origin: when the pixel data is not tightly packed (a sub-image or a padded stride), the visible rows are repacked
// into a contiguous buffer first, so passing img.Pix directly cannot garble rows.
func newNSImageFromNRGBA(img *image.NRGBA, logicalWidth, logicalHeight int) objc.ID {
	width := img.Rect.Dx()
	height := img.Rect.Dy()
	pixels := img.Pix
	if img.Stride != width*4 {
		pixels = make([]byte, width*height*4)
		for y := range height {
			src := img.PixOffset(img.Rect.Min.X, img.Rect.Min.Y+y)
			copy(pixels[y*width*4:(y+1)*width*4], img.Pix[src:src+width*4])
		}
	}
	return newNSImage(pixels, logicalWidth, logicalHeight, width, height)
}

// newNSImageWithFormat is newNSImage with an explicit NSBitmapFormat: pass 0 for premultiplied RGBA pixels or
// nsBitmapFormatAlphaNonpremultiplied for non-premultiplied ones.
func newNSImageWithFormat(pixels []byte, logicalWidth, logicalHeight, actualWidth, actualHeight, format int) objc.ID {
	rep := objc.ID(Cls("NSBitmapImageRep")).Send(Sel("alloc")).Send(
		Sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bitmapFormat:bytesPerRow:bitsPerPixel:"),
		unsafe.Pointer(nil), actualWidth, actualHeight, 8, 4, true, false,
		NSStringConstant("AppKit", "NSCalibratedRGBColorSpace"), format,
		actualWidth*4, 32)
	if rep == 0 {
		return 0
	}
	bitmap := objc.Send[*byte](rep, Sel("bitmapData"))
	copy(unsafe.Slice(bitmap, actualWidth*actualHeight*4), pixels)
	img := objc.ID(Cls("NSImage")).Send(Sel("alloc")).Send(Sel("initWithSize:"),
		NSSize{Width: float64(logicalWidth), Height: float64(logicalHeight)})
	img.Send(Sel("addRepresentation:"), rep)
	Release(rep)
	return img
}
