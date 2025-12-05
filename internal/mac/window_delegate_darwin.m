// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goWindowShouldCloseCallback(NSWindowRef w);

@interface macWindowDelegate : NSObject {
	NSWindow* wnd;
}

- (instancetype)initWithWindow:(NSWindow*)window;

@end

@implementation macWindowDelegate

- (instancetype)initWithWindow:(NSWindow*)window {
	self = [super init];
	if (self != nil) {
		wnd = window;
	}
	return self;
}

- (BOOL)windowShouldClose:(id)sender {
	goWindowShouldCloseCallback(wnd);
	return NO;
}

- (void)windowDidResize:(NSNotification *)notification {
	// [wnd->context.nsglCtx update];
	// const int maximized = [wnd->nsWindow isZoomed];
	// if (wnd->maximized != maximized) {
	// 	wnd->maximized = maximized;
	// 	goWindowMaximizeCallback(wnd, maximized);
	// }
	// const NSRect contentRect = [wnd->nsView frame];
	// if (contentRect.size.width != wnd->width || contentRect.size.height != wnd->height) {
	// 	wnd->width  = contentRect.size.width;
	// 	wnd->height = contentRect.size.height;
	// 	goWindowSizeCallback(wnd);
	// }
}

- (void)windowDidMove:(NSNotification *)notification {
	// [wnd->context.nsglCtx update];
	// goWindowPosCallback(wnd);
}

- (void)windowDidMiniaturize:(NSNotification *)notification {
	// goWindowMinimizeCallback(wnd, true);
}

- (void)windowDidDeminiaturize:(NSNotification *)notification {
	// goWindowMinimizeCallback(wnd, false);
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
	// _plafNotifyOfFocusChange(wnd, true);
	// if (_plafCursorInContentArea(wnd)) {
	// 	_plafUpdateCursorImage(wnd);
	// }
}

- (void)windowDidResignKey:(NSNotification *)notification {
	// _plafNotifyOfFocusChange(wnd, false);
}

@end

NSWindowDelegateRef newWindowDelegate(NSWindowRef w) {
	return [[macWindowDelegate alloc] initWithWindow:(NSWindow*)w];
}
