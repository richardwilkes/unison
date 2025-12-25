// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goWindowInputKeyCallback(NSWindowRef w, int keyCode, bool pressed, uint mods);
void goWindowInputFlagsCallback(NSWindowRef w, int keyCode, uint mods);
void goWindowCursorUpdateCallback(NSWindowRef w);
void goWindowMouseEnterCallback(NSWindowRef w);
void goWindowMouseExitCallback(NSWindowRef w);
void goWindowMouseMovedCallback(NSWindowRef w, float x, float y);
void goWindowMouseClickCallback(NSWindowRef w, int button, bool pressed, uint mods);
void goWindowScrollCallback(NSWindowRef w, float x, float y, bool pixels);
void goWindowUpdateLayerCallback(NSWindowRef w);
void goWindowRedrawCallback(NSWindowRef w);
void goWindowScaleCallback(NSWindowRef w, float xScale, float yScale);
void goWindowDropCallback(NSWindowRef w, int count, char** paths);

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
	goWindowUpdateLayerCallback(wnd);
}

- (void)cursorUpdate:(NSEvent *)event {
	goWindowCursorUpdateCallback(wnd);
}

- (BOOL)acceptsFirstMouse:(NSEvent *)event {
	return YES;
}

- (void)mouseDown:(NSEvent *)event {
	goWindowMouseClickCallback(wnd, 0, true, [event modifierFlags]);
}

- (void)mouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)mouseUp:(NSEvent *)event {
	goWindowMouseClickCallback(wnd, 0, false, [event modifierFlags]);
}

- (void)mouseMoved:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseMovedCallback(wnd, (float)pos.x, (float)(contentRect.size.height - pos.y));
}

- (void)rightMouseDown:(NSEvent *)event {
	goWindowMouseClickCallback(wnd, 1, true, [event modifierFlags]);
}

- (void)rightMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)rightMouseUp:(NSEvent *)event {
	goWindowMouseClickCallback(wnd, 1, false, [event modifierFlags]);
}

- (void)otherMouseDown:(NSEvent *)event {
	goWindowMouseClickCallback(wnd, (int)[event buttonNumber], true, [event modifierFlags]);
}

- (void)otherMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)otherMouseUp:(NSEvent *)event {
	goWindowMouseClickCallback(wnd, (int)[event buttonNumber], false, [event modifierFlags]);
}

- (void)mouseEntered:(NSEvent *)event {
	goWindowMouseEnterCallback(wnd);
}

- (void)mouseExited:(NSEvent *)event {
	goWindowMouseExitCallback(wnd);
}

- (void)viewDidChangeBackingProperties {
	const NSView* view = [wnd contentView];
	const NSRect contentRect = [view frame];
	const NSRect fbRect = [view convertRectToBacking:contentRect];
	const float xscale = fbRect.size.width / contentRect.size.width;
	const float yscale = fbRect.size.height / contentRect.size.height;
	goWindowScaleCallback(wnd, xscale, yscale);
}

- (void)drawRect:(NSRect)rect {
	goWindowRedrawCallback(wnd);
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
	goWindowInputKeyCallback(wnd, [event keyCode], true, [event modifierFlags]);
	[self interpretKeyEvents:@[event]];
}

- (void)flagsChanged:(NSEvent *)event {
	goWindowInputFlagsCallback(wnd, [event keyCode],
		[event modifierFlags] & NSEventModifierFlagDeviceIndependentFlagsMask);
}

- (void)keyUp:(NSEvent *)event {
	goWindowInputKeyCallback(wnd, [event keyCode], false, [event modifierFlags]);
}

- (void)scrollWheel:(NSEvent *)event {
	goWindowScrollCallback(wnd, (float)[event scrollingDeltaX], (float)[event scrollingDeltaY],
		[event hasPreciseScrollingDeltas]);
}

- (NSDragOperation)draggingEntered:(id <NSDraggingInfo>)sender {
	return NSDragOperationGeneric;
}

- (BOOL)performDragOperation:(id <NSDraggingInfo>)sender {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [sender draggingLocation];
	goWindowMouseMovedCallback(wnd, (float)pos.x, (float)(contentRect.size.height - pos.y));
	NSPasteboard* pasteboard = [sender draggingPasteboard];
	NSDictionary* options = @{NSPasteboardURLReadingFileURLsOnlyKey:@YES};
	NSArray* urls = [pasteboard readObjectsForClasses:@[[NSURL class]] options:options];
	int count = [urls count];
	if (count) {
		char** paths = calloc(count, sizeof(char*));
		for (int i = 0; i < count; i++) {
			paths[i] = strdup([urls[i] fileSystemRepresentation]);
		}
		goWindowDropCallback(wnd, count, paths);
		for (int i = 0; i < count; i++) {
			free(paths[i]);
		}
		free(paths);
	}
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
