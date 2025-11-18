#if defined(__APPLE__)

#include "platform.h"

const char* getClipboardString(void) {
	@autoreleasepool {
		NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
		if (![[pasteboard types] containsObject:NSPasteboardTypeString]) {
			return NULL;
		}
		NSString* object = [pasteboard stringForType:NSPasteboardTypeString];
		if (!object) {
			return NULL;
		}
		_glfw_free(_glfw.clipboardString);
		_glfw.clipboardString = _glfw_strdup([object UTF8String]);
		return _glfw.clipboardString;
	}
}

void setClipboardString(const char* string) {
	@autoreleasepool {
		NSPasteboard* pasteboard = [NSPasteboard generalPasteboard];
		[pasteboard declareTypes:@[NSPasteboardTypeString] owner:nil];
		[pasteboard setString:@(string) forType:NSPasteboardTypeString];
	}
}

#endif // __APPLE__
