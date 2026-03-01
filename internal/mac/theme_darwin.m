// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

void goThemeChangedCallback();

@interface ThemeDelegate : NSObject
@end

@implementation ThemeDelegate

- (void)themeChanged:(NSNotification *)unused {
	goThemeChangedCallback();
}

@end

void installThemeChangedCallback(void) {
	ThemeDelegate *delegate = [ThemeDelegate new];
	[NSDistributedNotificationCenter.defaultCenter addObserver:delegate
		selector:@selector(themeChanged:) name:@"AppleInterfaceThemeChangedNotification" object: nil];
	[NSDistributedNotificationCenter.defaultCenter addObserver:delegate
		selector:@selector(themeChanged:) name:@"AppleColorPreferencesChangedNotification" object: nil];
}
