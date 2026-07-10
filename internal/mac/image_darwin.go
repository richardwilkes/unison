// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// nsBitmapFormatAlphaNonpremultiplied is AppKit's NSBitmapFormatAlphaNonpremultiplied bitmap format option.
const nsBitmapFormatAlphaNonpremultiplied = 1 << 1

// newNSImage returns an NSImage (with a +1 retain count that the caller must release) of the given logical size,
// backed by a single 32-bit non-premultiplied RGBA bitmap representation of actualWidth x actualHeight device
// pixels copied from pixels. It returns 0 if the bitmap representation cannot be created.
func newNSImage(pixels []byte, logicalWidth, logicalHeight, actualWidth, actualHeight int) objc.ID {
	rep := objc.ID(Cls("NSBitmapImageRep")).Send(Sel("alloc")).Send(
		Sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bitmapFormat:bytesPerRow:bitsPerPixel:"),
		unsafe.Pointer(nil), actualWidth, actualHeight, 8, 4, true, false,
		NSStringConstant("AppKit", "NSCalibratedRGBColorSpace"), nsBitmapFormatAlphaNonpremultiplied,
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
