// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

bool goWindowShouldCloseCallback(NSWindowRef w);
void goWindowDidResizeCallback(NSWindowRef w);
void goWindowDidMoveCallback(NSWindowRef w);

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
	return goWindowShouldCloseCallback(wnd);
}

- (void)windowDidResize:(NSNotification *)notification {
	goWindowDidResizeCallback(wnd);
}

- (void)windowDidMove:(NSNotification *)notification {
	goWindowDidMoveCallback(wnd);
}

- (void)windowDidMiniaturize:(NSNotification *)notification {
	// TODO
	// goWindowMinimizeCallback(wnd, true);
}

- (void)windowDidDeminiaturize:(NSNotification *)notification {
	// TODO
	// goWindowMinimizeCallback(wnd, false);
}

- (void)windowDidBecomeKey:(NSNotification *)notification {
	// TODO
	// _plafNotifyOfFocusChange(wnd, true);
	// if (_plafCursorInContentArea(wnd)) {
	// 	_plafUpdateCursorImage(wnd);
	// }
}

- (void)windowDidResignKey:(NSNotification *)notification {
	// TODO
	// _plafNotifyOfFocusChange(wnd, false);
}

@end

NSWindowDelegateRef newWindowDelegate(NSWindowRef w) {
	return [[macWindowDelegate alloc] initWithWindow:(NSWindow*)w];
}
