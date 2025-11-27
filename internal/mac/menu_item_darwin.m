// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goMenuItemHandleCallback(NSMenuItemRef item);
bool goMenuItemValidateCallback(NSMenuItemRef item);

@interface MenuItemDelegate : NSObject<NSMenuItemValidation>
@end

@implementation MenuItemDelegate
- (BOOL)validateMenuItem:(NSMenuItem *)menuItem {
	return goMenuItemValidateCallback((NSMenuItemRef)menuItem) ? YES : NO;
}

- (void)handleMenuItem:(id)sender {
	goMenuItemHandleCallback((NSMenuItemRef)sender);
}
@end

static MenuItemDelegate *menuItemDelegate = nil;

NSMenuItemRef newMenuItem(int tag, CFStringRef title, CFStringRef keyEquiv, NSEventModifierFlags modifiers) {
	NSMenuItem *item = [[[NSMenuItem alloc] initWithTitle:(NSString *)title
		action:NSSelectorFromString(@"handleMenuItem:") keyEquivalent:(NSString *)keyEquiv] retain];
	[item setTag:tag];
	[item setKeyEquivalentModifierMask:modifiers];
	if (!menuItemDelegate) {
		menuItemDelegate = [MenuItemDelegate new];
	}
	[item setTarget:menuItemDelegate];
	return (NSMenuItemRef)item;
}

NSMenuItemRef newMenuSeparatorItem() {
	return (NSMenuItemRef)[[NSMenuItem separatorItem] retain];
}

bool menuItemIsSeparator(NSMenuItemRef mi) {
	return [(NSMenuItem *)mi isSeparatorItem];
}

int menuItemTag(NSMenuItemRef mi) {
	return [(NSMenuItem *)mi tag];
}

CFStringRef menuItemTitle(NSMenuItemRef mi) {
	return (CFStringRef)[(NSMenuItem *)mi title];
}

void menuItemSetTitle(NSMenuItemRef mi, CFStringRef title) {
	[(NSMenuItem *)mi setTitle:(NSString *)title];
}

CFStringRef menuItemKeyEquivalent(NSMenuItemRef mi) {
	return (CFStringRef)[(NSMenuItem *)mi keyEquivalent];
}

NSEventModifierFlags menuItemKeyEquivalentModifierMask(NSMenuItemRef mi) {
	return [(NSMenuItem *)mi keyEquivalentModifierMask];
}

void menuItemSetKeyBinding(NSMenuItemRef mi, CFStringRef keyEquiv, NSEventModifierFlags modifiers) {
	[(NSMenuItem *)mi setKeyEquivalent:(NSString *)keyEquiv];
	[(NSMenuItem *)mi setKeyEquivalentModifierMask:modifiers];
}

NSMenuRef menuItemMenu(NSMenuItemRef mi) {
	return (NSMenuRef)[(NSMenuItem *)mi menu];
}

NSMenuRef menuItemSubMenu(NSMenuItemRef mi) {
	return [(NSMenuItem *)mi submenu];
}

void menuItemSetSubMenu(NSMenuItemRef mi, NSMenuRef m) {
	[(NSMenuItem *)mi setSubmenu:(NSMenu *)m];
}

NSControlStateValue menuItemState(NSMenuItemRef mi) {
	return [(NSMenuItem *)mi state];
}

void menuItemSetState(NSMenuItemRef mi, NSControlStateValue state) {
	[(NSMenuItem *)mi setState:(NSControlStateValue)state];
}
