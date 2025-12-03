// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSScreenRef screenForDisplayID(CGDirectDisplayID displayID) {
	@autoreleasepool {
		const uint32_t unitNumber = CGDisplayUnitNumber(displayID);
		for (NSScreen* screen in [NSScreen screens]) {
			NSNumber* screenNumber = [screen deviceDescription][@"NSScreenNumber"];
			if (CGDisplayUnitNumber([screenNumber unsignedIntValue]) == unitNumber) {
				return screen;
			}
		}
		return nil;
	}
}

void screenFrame(NSScreenRef s, CGRect *frame) {
	*frame = [(NSScreen *)s frame];
}

void screenVisibleFrame(NSScreenRef s, CGRect *frame) {
	*frame = [(NSScreen *)s visibleFrame];
}

void screenConvertRectToBacking(NSScreenRef s, CGRect *r) {
	*r = [(NSScreen *)s convertRectToBacking:*r];
}