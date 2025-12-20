// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

double doubleClickInterval(void) {
	return [NSEvent doubleClickInterval];
}

NSEventModifierFlags eventModifierFlags(void) {
	return NSEvent.modifierFlags;
}

void postEmptyEvent(void) {
	@autoreleasepool {
		NSEvent* event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSMakePoint(0, 0)
			modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0];
		[NSApp postEvent:event atStart:YES];
	}
}

void pollEvents(void) {
	@autoreleasepool {
		for (;;) {
			NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantPast]
				inMode:NSDefaultRunLoopMode dequeue:YES];
			if (!event) {
				break;
			}
			[NSApp sendEvent:event];
		}
	}
}

void waitEvents(void) {
	@autoreleasepool {
		NSEvent *event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:[NSDate distantFuture]
			inMode:NSDefaultRunLoopMode dequeue:YES];
		[NSApp sendEvent:event];
		pollEvents();
	}
}

void waitEventsTimeout(double timeout) {
	@autoreleasepool {
		NSDate* date = [NSDate dateWithTimeIntervalSinceNow:timeout];
		NSEvent* event = [NSApp nextEventMatchingMask:NSEventMaskAny untilDate:date inMode:NSDefaultRunLoopMode
			dequeue:YES];
		if (event) {
			[NSApp sendEvent:event];
		}
		pollEvents();
	}
}

void stopMainEventLoop(void) {
	[NSApp stop:nil];
}
