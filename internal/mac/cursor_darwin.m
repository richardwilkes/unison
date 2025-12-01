// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSCursorRef newCursor(unsigned char* pixels, int width, int height, int xhot, int yhot) {
	NSBitmapImageRep* rep = [[NSBitmapImageRep alloc] initWithBitmapDataPlanes:NULL pixelsWide:width pixelsHigh:height
		bitsPerSample:8 samplesPerPixel:4 hasAlpha:YES isPlanar:NO colorSpaceName:NSCalibratedRGBColorSpace
		bitmapFormat:NSBitmapFormatAlphaNonpremultiplied bytesPerRow:width * 4 bitsPerPixel:32];
	if (rep == nil) {
		return nil;
	}
	memcpy([rep bitmapData], pixels, width * height * 4);
	NSImage* img = [[NSImage alloc] initWithSize:NSMakeSize(width, height)];
	[img addRepresentation:rep];
	NSCursor *cursor = [[[NSCursor alloc] initWithImage:img hotSpot:NSMakePoint(xhot, yhot)] retain];
	[img release];
	[rep release];
	return cursor;
}

NSCursorRef cursorArrow(void) {
	return [[NSCursor arrowCursor] retain];
}

NSCursorRef cursorIBeam(void) {
	return [[NSCursor IBeamCursor] retain];
}

NSCursorRef cursorCrosshair(void) {
	return [[NSCursor crosshairCursor] retain];
}

NSCursorRef cursorPointingHand(void) {
	return [[NSCursor pointingHandCursor] retain];
}

NSCursorRef cursorResizeLeftRight(void) {
	return [[NSCursor resizeLeftRightCursor] retain];
}

NSCursorRef cursorResizeUpDown(void) {
	return [[NSCursor resizeUpDownCursor] retain];
}

void cursorHide(void) {
	[NSCursor hide];
}

void cursorShow(void) {
	[NSCursor unhide];
}

void cursorSet(NSCursorRef cursor) {
	[(NSCursor*)cursor set];
}
