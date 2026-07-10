// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import <Cocoa/Cocoa.h>

typedef CFTypeRef NSDraggingInfoRef;
typedef CFTypeRef NSOpenPanelRef;
typedef CFTypeRef NSPasteboardRef;
typedef CFTypeRef NSPasteboardItemRef;
typedef CFTypeRef NSSavePanelRef;

// Drag
NSDragOperation dragSourceOperationMask(NSDraggingInfoRef sender);
CFArrayRef dragDataTypes(NSDraggingInfoRef sender);
bool dragHasString(NSDraggingInfoRef sender);
CFStringRef dragText(NSDraggingInfoRef sender);
bool dragHasFilePaths(NSDraggingInfoRef sender);
CFArrayRef dragFilePaths(NSDraggingInfoRef sender);
bool dragHasURLs(NSDraggingInfoRef sender);
CFArrayRef dragURLs(NSDraggingInfoRef sender);
bool dragHasDataType(NSDraggingInfoRef sender, CFStringRef dataType);
void* dragBytes(NSDraggingInfoRef sender, CFStringRef dataType, unsigned long long* length);

// Open Panel
NSOpenPanelRef newOpenPanel();
CFURLRef openPanelDirectoryURL(NSOpenPanelRef openPanel);
void openPanelSetDirectoryURL(NSOpenPanelRef openPanel, CFURLRef url);
CFArrayRef openPanelAllowedFileTypes(NSOpenPanelRef openPanel);
void openPanelSetAllowedFileTypes(NSOpenPanelRef openPanel, CFArrayRef types);
bool openPanelCanChooseFiles(NSOpenPanelRef openPanel);
void openPanelSetCanChooseFiles(NSOpenPanelRef openPanel, bool set);
bool openPanelCanChooseDirectories(NSOpenPanelRef openPanel);
void openPanelSetCanChooseDirectories(NSOpenPanelRef openPanel, bool set);
bool openPanelResolvesAliases(NSOpenPanelRef openPanel);
void openPanelSetResolvesAliases(NSOpenPanelRef openPanel, bool set);
bool openPanelAllowsMultipleSelection(NSOpenPanelRef openPanel);
void openPanelSetAllowsMultipleSelection(NSOpenPanelRef openPanel, bool set);
CFArrayRef openPanelURLs(NSOpenPanelRef openPanel);
bool openPanelRunModal(NSOpenPanelRef openPanel);

// Pasteboard
NSPasteboardRef pasteboardGeneral();
CFArrayRef pasteboardAvailableDataTypes(NSPasteboardRef pasteboard);
bool pasteboardHasDataType(NSPasteboardRef pasteboard, CFStringRef str);
void* pasteboardBytes(NSPasteboardRef pasteboard, CFStringRef dataType, unsigned long long* length);
void pasteboardClearContents(NSPasteboardRef pasteboard);
void pasteboardWriteObjects(NSPasteboardRef pasteboard, CFArrayRef items);
NSPasteboardItemRef newPasteboardItem();
void pasteboardItemSetString(NSPasteboardItemRef item, CFStringRef str);
void pasteboardItemSetData(NSPasteboardItemRef item, CFStringRef dataType, unsigned long long length, void* buffer);

// Save Panel
NSSavePanelRef newSavePanel();
CFURLRef savePanelDirectoryURL(NSSavePanelRef savePanel);
void savePanelSetDirectoryURL(NSSavePanelRef savePanel, CFURLRef url);
CFStringRef savePanelNameFieldStringValue(NSSavePanelRef savePanel);
void savePanelSetNameFieldStringValue(NSSavePanelRef savePanel, CFStringRef name);
CFArrayRef savePanelAllowedFileTypes(NSSavePanelRef savePanel);
void savePanelSetAllowedFileTypes(NSSavePanelRef savePanel, CFArrayRef types);
CFURLRef savePanelURL(NSSavePanelRef savePanel);
bool savePanelRunModal(NSSavePanelRef savePanel);
