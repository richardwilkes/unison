#if defined(__APPLE__)

#include "platform.h"

const char* plafGetClipboardString(void) {
	@autoreleasepool {
		NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
		if (![[pasteboard types] containsObject:NSPasteboardTypeString]) {
			return NULL;
		}
		NSString* object = [pasteboard stringForType:NSPasteboardTypeString];
		if (!object) {
			return NULL;
		}
		_plaf_free(_plaf.clipboardString);
		_plaf.clipboardString = _plaf_strdup([object UTF8String]);
		return _plaf.clipboardString;
	}
}

void plafSetClipboardString(const char* string) {
	@autoreleasepool {
		NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
		[pasteboard declareTypes:@[NSPasteboardTypeString] owner:nil];
		[pasteboard setString:@(string) forType:NSPasteboardTypeString];
	}
}

#endif // __APPLE__
