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
typedef CFTypeRef NSDraggingItemRef;
typedef CFTypeRef NSImageRef;
typedef CFTypeRef NSMenuRef;
typedef CFTypeRef NSMenuItemRef;
typedef CFTypeRef NSOpenPanelRef;
typedef CFTypeRef NSOpenGLContextRef;
typedef CFTypeRef NSOpenGLPixelFormatRef;
typedef CFTypeRef NSPasteboardRef;
typedef CFTypeRef NSPasteboardItemRef;
typedef CFTypeRef NSSavePanelRef;
typedef CFTypeRef NSViewRef;
typedef CFTypeRef NSWindowRef;

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

// Menu
NSMenuRef newMenu(CFStringRef title);
int menuNumberOfItems(NSMenuRef m);
NSMenuItemRef menuItemAtIndex(NSMenuRef m, int index);
void menuInsertItemAtIndex(NSMenuRef m, NSMenuItemRef mi, int index);
void menuRemoveItemAtIndex(NSMenuRef m, int index);
void menuRemoveAll(NSMenuRef m);
CFStringRef menuTitle(NSMenuRef m);
void menuPopup(NSWindowRef wnd, NSMenuRef m, NSMenuItemRef mi, CGRect bounds);

// Menu Item
NSMenuItemRef newMenuItem(int tag, CFStringRef title, CFStringRef keyEquiv, NSEventModifierFlags modifiers);
NSMenuItemRef newMenuSeparatorItem();
bool menuItemIsSeparator(NSMenuItemRef mi);
int menuItemTag(NSMenuItemRef mi);
CFStringRef menuItemTitle(NSMenuItemRef mi);
void menuItemSetTitle(NSMenuItemRef mi, CFStringRef title);
CFStringRef menuItemKeyEquivalent(NSMenuItemRef mi);
NSEventModifierFlags menuItemKeyEquivalentModifierMask(NSMenuItemRef mi);
void menuItemSetKeyBinding(NSMenuItemRef mi, CFStringRef keyEquiv, NSEventModifierFlags modifiers);
NSMenuRef menuItemMenu(NSMenuItemRef mi);
NSMenuRef menuItemSubMenu(NSMenuItemRef mi);
void menuItemSetSubMenu(NSMenuItemRef mi, NSMenuRef m);
NSControlStateValue menuItemState(NSMenuItemRef mi);
void menuItemSetState(NSMenuItemRef mi, NSControlStateValue state);

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

// OpenGL Context
NSOpenGLContextRef newOpenGLContext(NSViewRef view, NSOpenGLPixelFormatRef pixFmt, NSOpenGLContextRef shareCtx, bool transparent);
void openGLUpdate(NSOpenGLContextRef ctx);
void openGLMakeCurrent(NSOpenGLContextRef ctx);
void openGLFlushBuffer(NSOpenGLContextRef ctx);

// OpenGL Pixel Format
NSOpenGLPixelFormatRef newOpenGLPixelFormat(void);

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
NSDraggingItemRef newDraggingItem(NSPasteboardItemRef item, NSImageRef img, CGRect frame);

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

// View
NSViewRef newView(NSWindowRef w);
CGPoint viewBackingScale(NSViewRef v);
void viewFrame(NSViewRef v, CGRect* frame);
bool viewMouseInRect(NSViewRef v, CGPoint mousePt, CGRect rect);
void viewBeginDraggingSession(NSViewRef v, NSPasteboardItemRef item, NSDragOperation dragMask);
void viewRegisterDraggedTypes(NSViewRef v, CFArrayRef types);
void viewUnregisterDraggedTypes(NSViewRef v);
