// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

@interface macContentView : NSView {
	NSWindow*       wnd;
	NSTrackingArea* trackingArea;
}

- (instancetype)initWithWindow:(NSWindow*)window;

@end

@implementation macContentView

- (instancetype)initWithWindow:(NSWindow*)window {
	self = [super init];
	if (self != nil) {
		wnd = window;
		trackingArea = nil;
		[self updateTrackingAreas];
		[self registerForDraggedTypes:@[NSPasteboardTypeURL]];
	}
	return self;
}

- (void)dealloc {
	[trackingArea release];
	[super dealloc];
}

- (BOOL)isOpaque {
	return [wnd isOpaque];
}

- (BOOL)canBecomeKeyView {
	return YES;
}

- (BOOL)acceptsFirstResponder {
	return YES;
}

- (BOOL)wantsUpdateLayer {
	return YES;
}

- (void)updateLayer {
	// TODO
	//[wnd->context.nsglCtx update];
	//goWindowDrawCallback(wnd);
}

- (void)cursorUpdate:(NSEvent *)event {
	// TODO
	//_plafUpdateCursorImage(wnd);
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (void)mouseDown:(NSEvent *)event {
	// TODO
	//_plafInputMouseClick(wnd, MOUSE_BUTTON_LEFT, INPUT_PRESS, translateFlags([event modifierFlags]));
}

- (void)mouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)mouseUp:(NSEvent *)event {
	// TODO
	//_plafInputMouseClick(wnd, MOUSE_BUTTON_LEFT, INPUT_RELEASE, translateFlags([event modifierFlags]));
}

- (void)mouseMoved:(NSEvent *)event {
	// TODO
	// const NSRect contentRect = [wnd->nsView frame];
	// const NSPoint pos = [event locationInWindow];
	// _plafInputCursorPos(wnd, pos.x, contentRect.size.height - pos.y);
}

- (void)rightMouseDown:(NSEvent *)event {
	// TODO
	// _plafInputMouseClick(wnd, MOUSE_BUTTON_RIGHT, INPUT_PRESS, translateFlags([event modifierFlags]));
}

- (void)rightMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)rightMouseUp:(NSEvent *)event {
	// TODO
	// _plafInputMouseClick(wnd, MOUSE_BUTTON_RIGHT, INPUT_RELEASE, translateFlags([event modifierFlags]));
}

- (void)otherMouseDown:(NSEvent *)event {
	// TODO
	// _plafInputMouseClick(wnd, (int) [event buttonNumber], INPUT_PRESS, translateFlags([event modifierFlags]));
}

- (void)otherMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)otherMouseUp:(NSEvent *)event {
	// TODO
	// _plafInputMouseClick(wnd, (int) [event buttonNumber], INPUT_RELEASE, translateFlags([event modifierFlags]));
}

- (void)mouseEntered:(NSEvent *)event {
	// TODO
	// if (wnd->cursorHidden) {
	// 	hideCursor(wnd);
	// }
	// goCursorEnterCallback(wnd, true);
}

- (void)mouseExited:(NSEvent *)event {
	// TODO
	// if (wnd->cursorHidden) {
	// 	showCursor(wnd);
	// }
	// goCursorEnterCallback(wnd, false);
}

- (void)viewDidChangeBackingProperties {
	// TODO
	// const NSRect contentRect = [wnd->nsView frame];
	// const NSRect fbRect = [wnd->nsView convertRectToBacking:contentRect];
	// const float xscale = fbRect.size.width / contentRect.size.width;
	// const float yscale = fbRect.size.height / contentRect.size.height;
	// if (xscale != wnd->nsXScale || yscale != wnd->nsYScale) {
	// 	wnd->nsXScale = xscale;
	// 	wnd->nsYScale = yscale;
	// 	goWindowContentScaleCallback(wnd);
	// }
}

- (void)drawRect:(NSRect)rect {
	// TODO
	// goWindowDrawCallback(wnd);
}

- (void)updateTrackingAreas {
	if (trackingArea != nil) {
		[self removeTrackingArea:trackingArea];
		[trackingArea release];
	}
	trackingArea = [[NSTrackingArea alloc] initWithRect:[self bounds]
		options:NSTrackingMouseEnteredAndExited | NSTrackingActiveInKeyWindow | NSTrackingEnabledDuringMouseDrag |
			NSTrackingCursorUpdate | NSTrackingInVisibleRect | NSTrackingAssumeInside
		owner:self userInfo:nil];
	[self addTrackingArea:trackingArea];
	[super updateTrackingAreas];
}

- (void)keyDown:(NSEvent *)event {
	// TODO
	// const int key = translateKey([event keyCode]);
	// const int mods = translateFlags([event modifierFlags]);
	// _plafInputKey(wnd, key, [event keyCode], INPUT_PRESS, mods);
	// [self interpretKeyEvents:@[event]];
}

- (void)flagsChanged:(NSEvent *)event {
	// TODO
	// int action;
	// const unsigned int modifierFlags = [event modifierFlags] & NSEventModifierFlagDeviceIndependentFlagsMask;
	// const int key = translateKey([event keyCode]);
	// const int mods = translateFlags(modifierFlags);
	// const NSUInteger keyFlag = translateKeyToModifierFlag(key);
	// if (keyFlag & modifierFlags) {
	// 	if (wnd->keys[key] == INPUT_PRESS) {
	// 		action = INPUT_RELEASE;
	// 	} else {
	// 		action = INPUT_PRESS;
	// 	}
	// } else {
	// 	action = INPUT_RELEASE;
	// }
	// _plafInputKey(wnd, key, [event keyCode], action, mods);
}

- (void)keyUp:(NSEvent *)event {
	// TODO
	// const int key = translateKey([event keyCode]);
	// const int mods = translateFlags([event modifierFlags]);
	// _plafInputKey(wnd, key, [event keyCode], INPUT_RELEASE, mods);
}

- (void)scrollWheel:(NSEvent *)event {
	// TODO
	// double deltaX = [event scrollingDeltaX];
	// double deltaY = [event scrollingDeltaY];
	// if ([event hasPreciseScrollingDeltas]) {
	// 	deltaX *= 0.1;
	// 	deltaY *= 0.1;
	// }
	// if (fabs(deltaX) > 0.0 || fabs(deltaY) > 0.0) {
	// 	goScrollCallback(wnd, deltaX, deltaY);
	// }
}

- (NSDragOperation)draggingEntered:(id <NSDraggingInfo>)sender {
	return NSDragOperationGeneric;
}

- (BOOL)performDragOperation:(id <NSDraggingInfo>)sender {
	// TODO
	// const NSRect contentRect = [wnd->nsView frame];
	// const NSPoint pos = [sender draggingLocation];
	// _plafInputCursorPos(wnd, pos.x, contentRect.size.height - pos.y);
	// NSPasteboard* pasteboard = [sender draggingPasteboard];
	// NSDictionary* options = @{NSPasteboardURLReadingFileURLsOnlyKey:@YES};
	// NSArray* urls = [pasteboard readObjectsForClasses:@[[NSURL class]] options:options];
	// int count = [urls count];
	// if (count) {
	// 	char** paths = _plaf_calloc(count, sizeof(char*));
	// 	for (int i = 0; i < count; i++) {
	// 		paths[i] = _plaf_strdup([urls[i] fileSystemRepresentation]);
	// 	}
	// 	goDropCallback(wnd, count, paths);
	// 	for (NSUInteger i = 0; i < count; i++) {
	// 		_plaf_free(paths[i]);
	// 	}
	// 	_plaf_free(paths);
	// }
	return YES;
}

@end

NSViewRef newView(NSWindowRef w) {
	return (NSViewRef)[[macContentView alloc] initWithWindow:(NSWindow*)w];
}

void viewFrame(NSViewRef v, CGRect *frame) {
	*frame = [(NSView*)v frame];
}

bool viewMouseInRect(NSViewRef v, CGPoint mousePt, CGRect rect) {
	return [(NSView*)v mouse:mousePt inRect:rect];
}
