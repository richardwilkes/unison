// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goAppShouldTerminateCallback(void);
void goAppDidChangeScreenParametersCallback(void);
void goAppWillFinishLaunchingCallback(void);
void goAppDidFinishLaunchingCallback(void);
void goAppDidHideCallback(void);
void goOpenURLsCallback(CFArrayRef urls);

@interface macAppDelegate : NSObject <NSApplicationDelegate>
@end

@implementation macAppDelegate

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
	goAppShouldTerminateCallback();
	return NSTerminateCancel;
}

- (void)applicationDidChangeScreenParameters:(NSNotification *) notification {
	goAppDidChangeScreenParametersCallback();
}

- (void)applicationWillFinishLaunching:(NSNotification *)notification {
	goAppWillFinishLaunchingCallback();
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
	goAppDidFinishLaunchingCallback();
}

- (void)applicationDidHide:(NSNotification *)notification {
	goAppDidHideCallback();
}

- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
	goOpenURLsCallback((CFArrayRef)(urls));
}

@end

static id<NSApplicationDelegate> macAppDelegateSingleton = nil;
static id keyUpMonitorSingleton = nil;

bool installMacAppDelegate(void) {
	[NSApplication sharedApplication];
	macAppDelegateSingleton = [[macAppDelegate alloc] init];
	if (!macAppDelegateSingleton) {
		return false;
	}
	[NSApp setDelegate:macAppDelegateSingleton];
	NSEvent* (^block)(NSEvent*) = ^ NSEvent* (NSEvent* event) {
		if ([event modifierFlags] & NSEventModifierFlagCommand) {
			[[NSApp keyWindow] sendEvent:event];
		}
		return event;
	};
	keyUpMonitorSingleton = [NSEvent addLocalMonitorForEventsMatchingMask:NSEventMaskKeyUp handler:block];
	return true;
}

void uninstallMacAppDelegate(void) {
	[NSApp setDelegate:nil];
	if (keyUpMonitorSingleton) {
		[NSEvent removeMonitor:keyUpMonitorSingleton];
		keyUpMonitorSingleton = nil;
	}
	if (macAppDelegateSingleton) {
		[macAppDelegateSingleton release];
		macAppDelegateSingleton = nil;
	}
}

void finishLaunching(void) {
	if (![[NSRunningApplication currentApplication] isFinishedLaunching]) {
		[NSApp run];
	}
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
}

void activateIgnoringOtherApps(void) {
	[NSApp activateIgnoringOtherApps:YES];
}

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
