// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

#import <Cocoa/Cocoa.h>

typedef CFTypeRef NSCursorRef;
typedef CFTypeRef NSDraggingInfoRef;
typedef CFTypeRef NSMenuRef;
typedef CFTypeRef NSMenuItemRef;
typedef CFTypeRef NSOpenPanelRef;
typedef CFTypeRef NSOpenGLContextRef;
typedef CFTypeRef NSOpenGLPixelFormatRef;
typedef CFTypeRef NSPasteboardRef;
typedef CFTypeRef NSPasteboardItemRef;
typedef CFTypeRef NSSavePanelRef;
typedef CFTypeRef NSScreenRef;
typedef CFTypeRef NSViewRef;
typedef CFTypeRef NSWindowRef;
typedef CFTypeRef NSWindowDelegateRef;

// App
bool installMacAppDelegate(void);
void uninstallMacAppDelegate(void);
void finishLaunching(void);
void activateIgnoringOtherApps(void);
void hideRunningApplication(void);
void hideOtherApplications(void);
void unhideAllApplications(void);
void setMainMenu(NSMenuRef menu);
void setServicesMenu(NSMenuRef menu);
void setWindowsMenu(NSMenuRef menu);
void setHelpMenu(NSMenuRef menu);

// Cursor
NSCursorRef newCursor(unsigned char* pixels, int xhot, int yhot, int logicalWidth, int logicalheight, int actualWidth, int actualHeight);
NSCursorRef cursorArrow(void);
NSCursorRef cursorIBeam(void);
NSCursorRef cursorCrosshair(void);
NSCursorRef cursorPointingHand(void);
NSCursorRef cursorResizeLeftRight(void);
NSCursorRef cursorResizeUpDown(void);
void cursorHide(void);
void cursorShow(void);
void cursorSet(NSCursorRef cursor);

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

// Event
double doubleClickInterval(void);
NSEventModifierFlags eventModifierFlags(void);
void postEmptyEvent(void);
void pollEvents(void);
void waitEvents(void);
void waitEventsTimeout(double timeout);
void stopMainEventLoop(void);

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

// Screen
NSScreenRef screenForDisplayID(CGDirectDisplayID displayID);
void screenFrame(NSScreenRef screen, CGRect* frame);
void screenVisibleFrame(NSScreenRef screen, CGRect *frame);
void screenConvertRectToBacking(NSScreenRef screen, CGRect *rect);

// Sound
void beep(void);

// Theme
void installThemeChangedCallback(void);

// View
CGPoint viewBackingScale(NSViewRef v);
void viewFrame(NSViewRef v, CGRect* frame);
bool viewMouseInRect(NSViewRef v, CGPoint mousePt, CGRect rect);

// Window
NSWindowRef newWindow(CGRect contentRect, NSWindowStyleMask styleMask, bool canBeKeyWindow, bool canBeMainWindow);
void windowSetCollectionBehavior(NSWindowRef w, NSWindowCollectionBehavior behavior);
void windowSetWindowLevel(NSWindowRef w, NSWindowLevel level);
NSWindowStyleMask windowStyleMask(NSWindowRef w);
void windowSetTransparent(NSWindowRef w);
void windowSetTitle(NSWindowRef w, CFStringRef title);
NSViewRef windowContentView(NSWindowRef w);
void windowSetContentView(NSWindowRef w, NSViewRef v);
void windowSetRestorable(NSWindowRef w, bool restorable);
void windowMakeFirstResponder(NSWindowRef w, NSViewRef v);
void windowSetTabbingMode(NSWindowRef w, NSWindowTabbingMode mode);
void windowSetAcceptsMouseMovedEvents(NSWindowRef w, bool accept);
CGPoint windowMouseLocationOutsideOfEventStream(NSWindowRef w);
void windowMakeKeyAndOrderFront(NSWindowRef w);
void windowOrderOut(NSWindowRef w);
NSWindowDelegateRef windowDelegate(NSWindowRef w);
void windowSetDelegate(NSWindowRef w, NSWindowDelegateRef delegate);
bool windowFocused(NSWindowRef w);
bool windowMiniaturized(NSWindowRef w);
void windowMiniaturize(NSWindowRef w);
bool windowZoomed(NSWindowRef w);
void windowZoom(NSWindowRef w);
void windowFrame(NSWindowRef w, CGRect* r);
void windowSetFrame(NSWindowRef w, CGRect r, bool display);
void windowContentRectForFrameRect(NSWindowRef w, CGRect* r);
void windowFrameRectForContentRect(NSWindowRef w, CGRect* r);
bool windowVisible(NSWindowRef w);
void windowClose(NSWindowRef w);

// Window Delegate
NSViewRef newView(NSWindowRef w);
NSWindowDelegateRef newWindowDelegate(NSWindowRef w);
