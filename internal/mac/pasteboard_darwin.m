// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSPasteboardRef pasteboardGeneral() {
	return (NSPasteboardRef)[NSPasteboard generalPasteboard];
}

CFArrayRef pasteboardAvailableDataTypes(NSPasteboardRef pasteboard) {
	return (CFArrayRef)[(NSPasteboard*)pasteboard types];
}

bool pasteboardHasDataType(NSPasteboardRef pasteboard, CFStringRef dataType) {
	return [[(NSPasteboard*)pasteboard types] containsObject:(NSPasteboardType)dataType];
}

void* pasteboardBytes(NSPasteboardRef pasteboard, CFStringRef dataType, unsigned long long* length) {
	if (![[(NSPasteboard*)pasteboard types] containsObject:(NSPasteboardType)dataType]) {
		return nil;
	}
	NSData* data = [(NSPasteboard*)pasteboard dataForType:(NSPasteboardType)dataType];
	*length = data.length;
	if (data.length == 0) {
		return nil;
	}
	void* buffer = malloc(data.length);
	[data getBytes:buffer length:data.length];
	return buffer;
}

void pasteboardClearContents(NSPasteboardRef pasteboard) {
	[(NSPasteboard*)pasteboard clearContents];
}

void pasteboardWriteObjects(NSPasteboardRef pasteboard, CFArrayRef items) {
	[(NSPasteboard*)pasteboard writeObjects:(NSArray*)items];
}

NSPasteboardItemRef newPasteboardItem() {
	return (NSPasteboardItemRef)[[NSPasteboardItem alloc] init];
}

void pasteboardItemSetString(NSPasteboardItemRef item, CFStringRef str) {
	[(NSPasteboardItem*)item setString:(NSString*)str forType:NSPasteboardTypeString];
}

void pasteboardItemSetData(NSPasteboardItemRef item, CFStringRef dataType, unsigned long long length, void* buffer) {
	[(NSPasteboardItem*)item setData:[NSData dataWithBytes:buffer length:length] forType:(NSPasteboardType)dataType];
}
