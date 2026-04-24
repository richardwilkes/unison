// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goWindowKeyPressedCallback(NSWindowRef w, int key, uint mods);
void goWindowKeyReleasedCallback(NSWindowRef w, int key, uint mods);
void goWindowKeyTypedCallback(NSWindowRef w, uint32_t ch);
void goWindowCursorUpdateCallback(NSWindowRef w);
void goWindowMouseEnterCallback(NSWindowRef w);
void goWindowMouseExitCallback(NSWindowRef w);
void goWindowMouseMovedCallback(NSWindowRef w, float x, float y);
void goWindowMouseClickCallback(NSWindowRef w, int button, float x, float y, bool pressed, uint mods);
void goWindowScrollCallback(NSWindowRef w, float x, float y, bool pixels);
void goWindowUpdateLayerCallback(NSWindowRef w);
void goWindowRedrawCallback(NSWindowRef w);
void goWindowScaleCallback(NSWindowRef w, CGPoint scale);
void goWindowDropCallback(NSWindowRef w, int count, char** paths);

static const NSRange kEmptyRange = { NSNotFound, 0 };

@interface macContentView : NSView<NSTextInputClient> {
	NSWindow*                  wnd;
	NSTrackingArea*            trackingArea;
	NSMutableAttributedString* markedText;
}

- (instancetype)initWithWindow:(NSWindow*)window;

@end

@implementation macContentView

- (instancetype)initWithWindow:(NSWindow*)window {
	self = [super init];
	if (self != nil) {
		wnd = window;
		trackingArea = nil;
		markedText = [[NSMutableAttributedString alloc] init];
		[self updateTrackingAreas];
		[self registerForDraggedTypes:@[NSPasteboardTypeURL]];
	}
	return self;
}

- (void)dealloc {
	[trackingArea release];
	[markedText release];
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
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseClickCallback(wnd, 0, (float)pos.x, (float)(contentRect.size.height - pos.y), true,
		[event modifierFlags]);
}

- (void)mouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)mouseUp:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseClickCallback(wnd, 0, (float)pos.x, (float)(contentRect.size.height - pos.y), false,
		[event modifierFlags]);
}

- (void)mouseMoved:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseMovedCallback(wnd, (float)pos.x, (float)(contentRect.size.height - pos.y));
}

- (void)rightMouseDown:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseClickCallback(wnd, 1, (float)pos.x, (float)(contentRect.size.height - pos.y), true,
		[event modifierFlags]);
}

- (void)rightMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)rightMouseUp:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseClickCallback(wnd, 1, (float)pos.x, (float)(contentRect.size.height - pos.y), false,
		[event modifierFlags]);
}

- (void)otherMouseDown:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseClickCallback(wnd, (int)[event buttonNumber], (float)pos.x, (float)(contentRect.size.height - pos.y),
		true, [event modifierFlags]);
}

- (void)otherMouseDragged:(NSEvent *)event {
	[self mouseMoved:event];
}

- (void)otherMouseUp:(NSEvent *)event {
	const NSRect contentRect = [[wnd contentView] frame];
	const NSPoint pos = [event locationInWindow];
	goWindowMouseClickCallback(wnd, (int)[event buttonNumber], (float)pos.x, (float)(contentRect.size.height - pos.y),
		false, [event modifierFlags]);
}

- (void)mouseEntered:(NSEvent *)event {
	goWindowMouseEnterCallback(wnd);
}

- (void)mouseExited:(NSEvent *)event {
	goWindowMouseExitCallback(wnd);
}

- (void)viewDidChangeBackingProperties {
	goWindowScaleCallback(wnd, viewBackingScale([wnd contentView]));
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
	goWindowKeyPressedCallback(wnd, [event keyCode], [event modifierFlags]);
	[self interpretKeyEvents:@[event]];
}

- (void)keyUp:(NSEvent *)event {
	goWindowKeyReleasedCallback(wnd, [event keyCode], [event modifierFlags]);
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

- (BOOL)hasMarkedText {
	return [markedText length] > 0;
}

- (NSRange)markedRange {
	if ([markedText length] > 0) {
		return NSMakeRange(0, [markedText length] - 1);
	}
	return kEmptyRange;
}

- (NSRange)selectedRange {
	return kEmptyRange;
}

- (void)setMarkedText:(id)string selectedRange:(NSRange)selectedRange replacementRange:(NSRange)replacementRange {
	[markedText release];
	if ([string isKindOfClass:[NSAttributedString class]]) {
		markedText = [[NSMutableAttributedString alloc] initWithAttributedString:string];
	} else {
		markedText = [[NSMutableAttributedString alloc] initWithString:string];
	}
}

- (void)unmarkText {
	[[markedText mutableString] setString:@""];
}

- (NSArray*)validAttributesForMarkedText {
	return [NSArray array];
}

- (NSAttributedString*)attributedSubstringForProposedRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
	return nil;
}

- (NSUInteger)characterIndexForPoint:(NSPoint)point {
	return 0;
}

- (NSRect)firstRectForCharacterRange:(NSRange)range actualRange:(NSRangePointer)actualRange {
	const NSRect frame = [self frame];
	return NSMakeRect(frame.origin.x, frame.origin.y, 0.0, 0.0);
}

- (void)insertText:(id)string replacementRange:(NSRange)replacementRange {
	if (([[NSApp currentEvent] modifierFlags] & NSEventModifierFlagCommand) == 0) {
		NSString* characters;
		if ([string isKindOfClass:[NSAttributedString class]]) {
			characters = [string string];
		} else {
			characters = (NSString*)string;
		}
		NSRange range = NSMakeRange(0, [characters length]);
		while (range.length) {
			uint32_t ch = 0;
			if ([characters getBytes:&ch maxLength:sizeof(ch) usedLength:NULL
				encoding:NSUTF32StringEncoding options:0 range:range remainingRange:&range]) {
				if (ch >= 0xf700 && ch <= 0xf7ff) {
					continue;
				}
				goWindowKeyTypedCallback(wnd, ch);
			}
		}
	}
}

- (void)doCommandBySelector:(SEL)selector {
}

@end

NSViewRef newView(NSWindowRef w) {
	return (NSViewRef)[[macContentView alloc] initWithWindow:(NSWindow*)w];
}

CGPoint viewBackingScale(NSViewRef v) {
	const CGRect contentRect = CGRectMake(0, 0, 1000, 1000);
	const CGRect fbRect = [((NSView*)v) convertRectToBacking:contentRect];
	const float xscale = fbRect.size.width / contentRect.size.width;
	const float yscale = fbRect.size.height / contentRect.size.height;
	return CGPointMake(xscale, yscale);
}

void viewFrame(NSViewRef v, CGRect *frame) {
	*frame = [(NSView*)v frame];
}

bool viewMouseInRect(NSViewRef v, CGPoint mousePt, CGRect rect) {
	return [(NSView*)v mouse:mousePt inRect:rect];
}
