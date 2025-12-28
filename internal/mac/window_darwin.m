// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

@interface macWindow : NSWindow {
	bool canBeKeyWindow;
	bool canBeMainWindow;
}

- (instancetype)initWithContentRect:(CGRect)contentRect styleMask:(NSWindowStyleMask)styleMask canBeKey:(bool)keyFlag
	canBeMain:(bool)mainFlag;

@end

@implementation macWindow

- (instancetype)initWithContentRect:(CGRect)contentRect styleMask:(NSWindowStyleMask)styleMask canBeKey:(bool)keyFlag
	canBeMain:(bool)mainFlag {
	self = [super initWithContentRect:contentRect styleMask:styleMask backing:NSBackingStoreBuffered defer:NO];
	if (self != nil) {
		canBeKeyWindow = keyFlag;
		canBeMainWindow = mainFlag;
	}
	return self;
}

- (BOOL)canBecomeKeyWindow {
	return canBeKeyWindow;
}

- (BOOL)canBecomeMainWindow {
	return canBeMainWindow;
}

@end

NSWindowRef newWindow(CGRect contentRect, NSWindowStyleMask styleMask, bool canBeKeyWindow, bool canBeMainWindow) {
	return (NSWindowRef)[[macWindow alloc] initWithContentRect:contentRect styleMask:styleMask canBeKey:canBeKeyWindow
		canBeMain:canBeMainWindow];
}

void windowSetCollectionBehavior(NSWindowRef w, NSWindowCollectionBehavior behavior) {
	[(NSWindow*)w setCollectionBehavior:behavior];
}

void windowSetWindowLevel(NSWindowRef w, NSWindowLevel level) {
	[(NSWindow*)w setLevel:level];
}

NSWindowStyleMask windowStyleMask(NSWindowRef w) {
	return [(NSWindow*)w styleMask];
}

void windowSetTransparent(NSWindowRef w) {
	NSWindow* wnd = (NSWindow*)w;
	[wnd setOpaque:NO];
	[wnd setHasShadow:NO];
	[wnd setBackgroundColor:[NSColor clearColor]];
}

void windowSetTitle(NSWindowRef w, CFStringRef title) {
	[(NSWindow*)w setTitle:(NSString *)title];
}

NSViewRef windowContentView(NSWindowRef w) {
	return (NSViewRef)[(NSWindow*)w contentView];
}

void windowSetContentView(NSWindowRef w, NSViewRef v) {
	[(NSWindow*)w setContentView:(NSView*)v];
}

void windowSetRestorable(NSWindowRef w, bool restorable) {
	[(NSWindow*)w setRestorable:restorable];
}

void windowMakeFirstResponder(NSWindowRef w, NSViewRef v) {
	[(NSWindow*)w makeFirstResponder:(NSView*)v];
}

void windowSetTabbingMode(NSWindowRef w, NSWindowTabbingMode mode) {
	[(NSWindow*)w setTabbingMode:mode];
}

void windowSetAcceptsMouseMovedEvents(NSWindowRef w, bool accept) {
	[(NSWindow*)w setAcceptsMouseMovedEvents:accept];
}

CGPoint windowMouseLocationOutsideOfEventStream(NSWindowRef w) {
	return [(NSWindow*)w mouseLocationOutsideOfEventStream];
}

void windowMakeKeyAndOrderFront(NSWindowRef w) {
	[(NSWindow*)w makeKeyAndOrderFront:nil];
}

void windowOrderOut(NSWindowRef w) {
	[(NSWindow*)w orderOut:nil];
}

NSWindowDelegateRef windowDelegate(NSWindowRef w) {
	return (NSWindowDelegateRef)[(NSWindow*)w delegate];
}

void windowSetDelegate(NSWindowRef w, NSWindowDelegateRef delegate) {
	[(NSWindow*)w setDelegate:delegate];
}

bool windowFocused(NSWindowRef w) {
	return [(NSWindow*)w isKeyWindow];
}

bool windowMiniaturized(NSWindowRef w) {
	return [(NSWindow*)w isMiniaturized];
}

void windowMiniaturize(NSWindowRef w) {
	[(NSWindow*)w miniaturize:nil];
}

bool windowZoomed(NSWindowRef w) {
	return [(NSWindow*)w isZoomed];
}

void windowZoom(NSWindowRef w) {
	[(NSWindow*)w zoom:nil];
}

void windowFrame(NSWindowRef w, CGRect* r) {
	*r = [(NSWindow*)w frame];
}

void windowSetFrame(NSWindowRef w, CGRect r, bool display) {
	[(NSWindow*)w setFrame:r display:display];
}

void windowContentRectForFrameRect(NSWindowRef w, CGRect* r) {
	*r = [(NSWindow*)w contentRectForFrameRect:*r];
}

void windowFrameRectForContentRect(NSWindowRef w, CGRect* r) {
	*r = [(NSWindow*)w frameRectForContentRect:*r];
}

bool windowVisible(NSWindowRef w) {
	return [(NSWindow*)w isVisible];
}

void windowClose(NSWindowRef w) {
	[(NSWindow*)w close];
}
