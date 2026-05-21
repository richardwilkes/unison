// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSDragOperation dragSourceOperationMask(NSDraggingInfoRef sender) {
	return [(id <NSDraggingInfo>)sender draggingSourceOperationMask];
}

CFArrayRef dragDataTypes(NSDraggingInfoRef sender) {
	return (CFArrayRef)[[(id <NSDraggingInfo>)sender draggingPasteboard] types];
}

bool dragHasString(NSDraggingInfoRef sender) {
	NSPasteboard *pb = [(id <NSDraggingInfo>)sender draggingPasteboard];
	return [[pb types] containsObject:NSPasteboardTypeString];
}

CFStringRef dragText(NSDraggingInfoRef sender) {
	NSPasteboard *pb = [(id <NSDraggingInfo>)sender draggingPasteboard];
	if (![[pb types] containsObject:NSPasteboardTypeString]) {
		return nil;
	}
	return (CFStringRef)([pb stringForType:NSPasteboardTypeString]);
}

bool dragHasDataType(NSDraggingInfoRef sender, CFStringRef dataType) {
	NSPasteboard *pb = [(id <NSDraggingInfo>)sender draggingPasteboard];
	return [[pb types] containsObject:(NSPasteboardType)dataType];
}

void* dragBytes(NSDraggingInfoRef sender, CFStringRef dataType, unsigned long long* length) {
	NSPasteboard *pb = [(id <NSDraggingInfo>)sender draggingPasteboard];
	if (![[pb types] containsObject:(NSPasteboardType)dataType]) {
		return nil;
	}
	NSData* data = [pb dataForType:(NSPasteboardType)dataType];
	*length = data.length;
	if (data.length == 0) {
		return nil;
	}
	void* buffer = malloc(data.length);
	[data getBytes:buffer length:data.length];
	return buffer;
}
