// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void hideRunningApplication(void) {
	[[NSRunningApplication currentApplication] hide];
}

void hideOtherApplications(void) {
	NSApplication *app = [NSApplication sharedApplication];
	[app hideOtherApplications:app];
}

void unhideAllApplications(void) {
	NSApplication *app = [NSApplication sharedApplication];
	[app unhideAllApplications:app];
}

void setActivationPolicy(NSApplicationActivationPolicy policy) {
	[[NSApplication sharedApplication] setActivationPolicy:policy];
}

void setMainMenu(NSMenuRef menu) {
	[[NSApplication sharedApplication] setMainMenu:(NSMenu *)menu];
}

void setServicesMenu(NSMenuRef menu) {
	[[NSApplication sharedApplication] setServicesMenu:(NSMenu *)menu];
}

void setWindowsMenu(NSMenuRef menu) {
	[[NSApplication sharedApplication] setWindowsMenu:(NSMenu *)menu];
}

void setHelpMenu(NSMenuRef menu) {
	[[NSApplication sharedApplication] setHelpMenu:(NSMenu *)menu];
}
