// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSImageRef newImage(unsigned char* pixels, int logicalWidth, int logicalheight, int actualWidth, int actualHeight) {
	NSBitmapImageRep* rep = [[NSBitmapImageRep alloc]
		initWithBitmapDataPlanes:NULL
		pixelsWide:actualWidth
		pixelsHigh:actualHeight
		bitsPerSample:8
		samplesPerPixel:4
		hasAlpha:YES
		isPlanar:NO
		colorSpaceName:NSCalibratedRGBColorSpace
		bitmapFormat:NSBitmapFormatAlphaNonpremultiplied
		bytesPerRow:actualWidth * 4
		bitsPerPixel:32];
	if (rep == nil) {
		return nil;
	}
	memcpy([rep bitmapData], pixels, actualWidth * actualHeight * 4);
	NSImage* img = [[NSImage alloc] initWithSize:NSMakeSize(logicalWidth, logicalheight)];
	[img addRepresentation:rep];
	[rep release];
	return (NSImageRef)img;
}
