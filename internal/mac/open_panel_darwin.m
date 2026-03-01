// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import "macos.h"

NSOpenPanelRef newOpenPanel() {
	return [[NSOpenPanel openPanel] retain];
}

CFURLRef openPanelDirectoryURL(NSOpenPanelRef openPanel) {
	return (CFURLRef)[(NSOpenPanel *)openPanel directoryURL];
}

void openPanelSetDirectoryURL(NSOpenPanelRef openPanel, CFURLRef url) {
	[(NSOpenPanel *)openPanel setDirectoryURL:(NSURL *)url];
}

CFArrayRef openPanelAllowedFileTypes(NSOpenPanelRef openPanel) {
	return (CFArrayRef)([(NSOpenPanel *)openPanel allowedFileTypes]);
}

void openPanelSetAllowedFileTypes(NSOpenPanelRef openPanel, CFArrayRef types) {
	[(NSOpenPanel *)openPanel setAllowedFileTypes:(NSArray<NSString *>*)(types)];
}

bool openPanelCanChooseFiles(NSOpenPanelRef openPanel) {
	return [(NSOpenPanel *)openPanel canChooseFiles];
}

void openPanelSetCanChooseFiles(NSOpenPanelRef openPanel, bool set) {
	[(NSOpenPanel *)openPanel setCanChooseFiles:set];
}

bool openPanelCanChooseDirectories(NSOpenPanelRef openPanel) {
	return [(NSOpenPanel *)openPanel canChooseDirectories];
}

void openPanelSetCanChooseDirectories(NSOpenPanelRef openPanel, bool set) {
	[(NSOpenPanel *)openPanel setCanChooseDirectories:set];
}

bool openPanelResolvesAliases(NSOpenPanelRef openPanel) {
	return [(NSOpenPanel *)openPanel resolvesAliases];
}

void openPanelSetResolvesAliases(NSOpenPanelRef openPanel, bool set) {
	[(NSOpenPanel *)openPanel setResolvesAliases:set];
}

bool openPanelAllowsMultipleSelection(NSOpenPanelRef openPanel) {
	return [(NSOpenPanel *)openPanel allowsMultipleSelection];
}

void openPanelSetAllowsMultipleSelection(NSOpenPanelRef openPanel, bool set) {
	[(NSOpenPanel *)openPanel setAllowsMultipleSelection:set];
}

CFArrayRef openPanelURLs(NSOpenPanelRef openPanel) {
	return (CFArrayRef)[(NSOpenPanel *)openPanel URLs];
}

bool openPanelRunModal(NSOpenPanelRef openPanel) {
	return [(NSOpenPanel *)openPanel runModal] == NSModalResponseOK;
}
