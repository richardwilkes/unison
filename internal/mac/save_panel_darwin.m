// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSSavePanelRef newSavePanel() {
	return [[NSSavePanel savePanel] retain];
}

CFURLRef savePanelDirectoryURL(NSSavePanelRef savePanel) {
	return (CFURLRef)[(NSSavePanel *)savePanel directoryURL];
}

void savePanelSetDirectoryURL(NSSavePanelRef savePanel, CFURLRef url) {
	[(NSSavePanel *)savePanel setDirectoryURL:(NSURL *)url];
}

CFStringRef savePanelNameFieldStringValue(NSSavePanelRef savePanel) {
	return (CFStringRef)[(NSSavePanel *)savePanel nameFieldStringValue];
}

void savePanelSetNameFieldStringValue(NSSavePanelRef savePanel, CFStringRef name) {
	[(NSSavePanel *)savePanel setNameFieldStringValue:(NSString *)name];
}

CFArrayRef savePanelAllowedFileTypes(NSSavePanelRef savePanel) {
	return (CFArrayRef)[(NSSavePanel *)savePanel allowedFileTypes];
}

void savePanelSetAllowedFileTypes(NSSavePanelRef savePanel, CFArrayRef types) {
	[(NSSavePanel *)savePanel setAllowedFileTypes:(NSArray<NSString *>*)types];
}

CFURLRef savePanelURL(NSSavePanelRef savePanel) {
	return (CFURLRef)[(NSSavePanel *)savePanel URL];
}

bool savePanelRunModal(NSSavePanelRef savePanel) {
	return [(NSSavePanel *)savePanel runModal] == NSModalResponseOK;
}
