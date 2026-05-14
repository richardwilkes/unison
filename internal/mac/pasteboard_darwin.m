// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

CFStringRef pasteboardString() {
	NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
	if (![[pasteboard types] containsObject:NSPasteboardTypeString]) {
		return nil;
	}
	return (CFStringRef)([pasteboard stringForType:NSPasteboardTypeString]);
}

void pasteboardSetString(CFStringRef str) {
	NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
	[pasteboard declareTypes:@[NSPasteboardTypeString] owner:nil];
	[pasteboard setString:(NSString *)str forType:NSPasteboardTypeString];
}

void* pasteboardBytes(CFStringRef dataType, unsigned long long* length) {
	NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
	if (![[pasteboard types] containsObject:(NSPasteboardType)dataType]) {
		return nil;
	}
	NSData* data = [pasteboard dataForType:(NSPasteboardType)dataType];
	*length = data.length;
	if (data.length == 0) {
		return nil;
	}
	void* buffer = malloc(data.length);
	[data getBytes:buffer length:data.length];
	return buffer;
}

void pasteboardSetBytes(CFStringRef dataType, unsigned long long length, void* buffer) {
	NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
	[pasteboard declareTypes:@[(NSPasteboardType)dataType] owner:nil];
	[pasteboard setData:[NSData dataWithBytes:buffer length:length] forType:(NSPasteboardType)dataType];
}
