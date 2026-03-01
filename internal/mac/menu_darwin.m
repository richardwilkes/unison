// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goUpdateMenuCallback(NSMenuRef menu);

@interface MenuDelegate : NSObject<NSMenuDelegate>
@end

@implementation MenuDelegate
- (void)menuNeedsUpdate:(NSMenu *)menu {
	goUpdateMenuCallback((NSMenuRef)(menu));
}
@end

static MenuDelegate *menuDelegate = nil;

NSMenuRef newMenu(CFStringRef title) {
	NSMenu *menu = [[[NSMenu alloc] initWithTitle:(NSString *)title] retain];
	if (!menuDelegate) {
		menuDelegate = [MenuDelegate new];
	}
	[menu setDelegate:menuDelegate];
	return (NSMenuRef)menu;
}

int menuNumberOfItems(NSMenuRef m) {
	return [(NSMenu *)m numberOfItems];
}

NSMenuItemRef menuItemAtIndex(NSMenuRef m, int index) {
	return (NSMenuItemRef)[(NSMenu *)m itemAtIndex:index];
}

void menuInsertItemAtIndex(NSMenuRef m, NSMenuItemRef mi, int index) {
	[(NSMenu *)m insertItem:(NSMenuItem *)mi atIndex:index];
}

void menuRemoveItemAtIndex(NSMenuRef m, int index) {
	[(NSMenu *)m removeItemAtIndex:index];
}

void menuRemoveAll(NSMenuRef m) {
	[(NSMenu *)m removeAllItems];
}

CFStringRef menuTitle(NSMenuRef m) {
	return (CFStringRef)[(NSMenu *)m title];
}

void menuPopup(NSWindowRef wnd, NSMenuRef m, NSMenuItemRef mi, CGRect bounds) {
	// popupMenuPositioningItem:atLocation:inView: is not being used here because it fails to work when a modal dialog
	// is being used.
	NSPopUpButtonCell *popUpButtonCell = [[[NSPopUpButtonCell alloc] initTextCell:@"" pullsDown:NO] retain];
	[popUpButtonCell setAutoenablesItems:NO];
	[popUpButtonCell setAltersStateOfSelectedItem:NO];
	[popUpButtonCell setMenu:(NSMenu *)m];
	[popUpButtonCell selectItem:(NSMenuItem *)mi];
	[popUpButtonCell performClickWithFrame:bounds inView:[(NSWindow *)wnd contentView]];
	[popUpButtonCell release];
}
