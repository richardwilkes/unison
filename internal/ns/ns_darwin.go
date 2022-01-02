// Copyright ©2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

// Important information about memory management:
// https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFMemoryMgmt/Concepts/Ownership.html#//apple_ref/doc/uid/20001148

/*
#cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void hideRunningApplication() {
	[[NSRunningApplication currentApplication] hide];
}

void beep() {
	NSBeep();
}

double doubleClickInterval() {
	return [NSEvent doubleClickInterval];
}

typedef CFTypeRef NSOpenPanelRef;

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

typedef CFTypeRef NSSavePanelRef;

NSSavePanelRef newSavePanel() {
	return [[NSSavePanel savePanel] retain];
}

CFURLRef savePanelDirectoryURL(NSSavePanelRef savePanel) {
	return (CFURLRef)[(NSSavePanel *)savePanel directoryURL];
}

void savePanelSetDirectoryURL(NSSavePanelRef savePanel, CFURLRef url) {
	[(NSSavePanel *)savePanel setDirectoryURL:(NSURL *)url];
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

typedef CFTypeRef NSViewRef;

void viewFrame(NSViewRef v, NSRect *frame) {
	*frame = [(NSView *)v frame];
}

typedef CFTypeRef NSWindowRef;

NSViewRef windowContentView(NSWindowRef w) {
	return (NSViewRef)[(NSWindow *)w contentView];
}


typedef CFTypeRef NSMenuRef;
typedef CFTypeRef NSMenuItemRef;

void updateMenuCallback(NSMenuRef menu);

@interface MenuDelegate : NSObject<NSMenuDelegate>
@end

@implementation MenuDelegate
- (void)menuNeedsUpdate:(NSMenu *)menu {
	updateMenuCallback((NSMenuRef)(menu));
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

CFStringRef menuTitle(NSMenuRef m) {
	return (CFStringRef)[(NSMenu *)m title];
}

bool menuItemValidateCallback(NSMenuItemRef item);
void menuItemHandleCallback(NSMenuItemRef item);

@interface MenuItemDelegate : NSObject<NSMenuItemValidation>
@end

@implementation MenuItemDelegate
- (BOOL)validateMenuItem:(NSMenuItem *)menuItem {
	return menuItemValidateCallback((NSMenuItemRef)menuItem) ? YES : NO;
}

- (void)handleMenuItem:(id)sender {
	menuItemHandleCallback((NSMenuItemRef)sender);
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

void menuPopup(NSWindowRef wnd, NSMenuRef m, NSMenuItemRef mi, CGRect bounds) {
	// popupMenuPositioningItem:atLocation:inView: is not being used here because it fails to work when a modal dialog
	// is being used.
	NSPopUpButtonCell *popUpButtonCell = [[NSPopUpButtonCell alloc] initTextCell:@"" pullsDown:NO];
	[popUpButtonCell setAutoenablesItems:NO];
	[popUpButtonCell setAltersStateOfSelectedItem:NO];
	[popUpButtonCell setMenu:(NSMenu *)m];
	[popUpButtonCell selectItem:(NSMenuItem *)mi];
	[popUpButtonCell performClickWithFrame:bounds inView:[(NSWindow *)wnd contentView]];
	[popUpButtonCell release];
}

static id<NSApplicationDelegate> underlyingAppDelegate;

void appOpenURLsCallback(CFArrayRef urls);

@interface UnisonAppDelegate : NSObject<NSApplicationDelegate>
@end

@implementation UnisonAppDelegate

- (void)applicationWillFinishLaunching:(NSNotification *)notification {
	[underlyingAppDelegate applicationWillFinishLaunching:notification];
}

- (void)applicationDidFinishLaunching:(NSNotification *)notification {
	[underlyingAppDelegate applicationDidFinishLaunching:notification];
}

- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
	return [underlyingAppDelegate applicationShouldTerminate:sender];
}

- (void)applicationDidChangeScreenParameters:(NSNotification *)notification {
	[underlyingAppDelegate applicationDidChangeScreenParameters:notification];
}

- (void)applicationDidHide:(NSNotification *)notification {
	[underlyingAppDelegate applicationDidHide:notification];
}

// All of the methods above this point are just pass-throughs to glfw. If glfw adds more methods, corresponding
// additions should be made here.

- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
	appOpenURLsCallback((CFArrayRef)(urls));
}

@end

void installAppDelegate() {
	NSApplication *app = [NSApplication sharedApplication];
	underlyingAppDelegate = [app delegate];
	UnisonAppDelegate *delegate = [UnisonAppDelegate new];
	[app setDelegate:delegate];
}

void themeChangedCallback();

@interface ThemeDelegate : NSObject
@end

@implementation ThemeDelegate

- (void)themeChanged:(NSNotification *)unused {
	themeChangedCallback();
}

@end

void installThemeChangedCallback() {
	ThemeDelegate *delegate = [ThemeDelegate new];
	[NSDistributedNotificationCenter.defaultCenter addObserver:delegate
		selector:@selector(themeChanged:) name:@"AppleInterfaceThemeChangedNotification" object: nil];
	[NSDistributedNotificationCenter.defaultCenter addObserver:delegate
		selector:@selector(themeChanged:) name:@"AppleColorPreferencesChangedNotification" object: nil];
}

void hideOtherApplications() {
	NSApplication *app = [NSApplication sharedApplication];
	[app hideOtherApplications:app];
}

void unhideAllApplications() {
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
*/
import "C"

import (
	"net/url"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xio/fs"
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

type EventModifierFlags uint

const (
	EventModifierFlagCapsLock EventModifierFlags = 1 << (16 + iota)
	EventModifierFlagShift
	EventModifierFlagControl
	EventModifierFlagOption
	EventModifierFlagCommand
)

type ControlStateValue int

const (
	ControlStateValueMixed ControlStateValue = iota - 1
	ControlStateValueOff
	ControlStateValueOn
)

func InstallAppDelegate(urlOpener func([]string)) {
	openURLsCallback = urlOpener
	if urlOpener != nil {
		C.installAppDelegate()
	}
}

func InstallSystemThemeChangedCallback(f func()) {
	systemThemeChangedCallback = f
	if f != nil {
		C.installThemeChangedCallback()
	}
}

func Beep() {
	C.beep()
}

func DoubleClickInterval() time.Duration {
	return time.Duration(C.doubleClickInterval()*1000) * time.Millisecond
}

type String C.CFStringRef

func NewString(str string) String {
	header := (*reflect.StringHeader)(unsafe.Pointer(&str))
	return String(C.CFStringCreateWithBytes(0, (*C.uint8)(unsafe.Pointer(header.Data)), C.long(header.Len),
		C.kCFStringEncodingUTF8, 0))
}

func (s String) String() string {
	strPtr := C.CFStringGetCStringPtr(C.CFStringRef(s), C.kCFStringEncodingUTF8)
	if strPtr == nil {
		maxBytes := 4*C.CFStringGetLength(C.CFStringRef(s)) + 1
		strPtr = (*C.char)(C.malloc(C.size_t(maxBytes)))
		defer C.free(unsafe.Pointer(strPtr))
		if C.CFStringGetCString(C.CFStringRef(s), strPtr, maxBytes, C.kCFStringEncodingUTF8) == 0 {
			jot.Warn(errs.New("failed to convert string"))
			return ""
		}
	}
	return C.GoString(strPtr)
}

func (s String) Release() {
	C.CFRelease(C.CFTypeRef(s))
}

type MutableArray C.CFMutableArrayRef
type Array C.CFArrayRef

func NewArrayFromStringSlice(slice []string) Array {
	a := C.CFArrayCreateMutable(0, C.long(len(slice)), &C.kCFTypeArrayCallBacks)
	for _, s := range slice {
		str := NewString(s)
		C.CFArrayAppendValue(a, unsafe.Pointer(str))
		str.Release()
	}
	return Array(a)
}

func (a Array) Count() int {
	return int(C.CFArrayGetCount(C.CFArrayRef(a)))
}

func (a Array) URLAtIndex(index int) URL {
	return URL(C.CFArrayGetValueAtIndex(C.CFArrayRef(a), C.long(index)))
}

func (a Array) StringAtIndex(index int) String {
	return String(C.CFArrayGetValueAtIndex(C.CFArrayRef(a), C.long(index)))
}

func (a Array) Release() {
	C.CFRelease(C.CFTypeRef(a))
}
func (a Array) ArrayOfURLToStringSlice() []string {
	count := a.Count()
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		u, err := url.Parse(a.URLAtIndex(i).AbsoluteString())
		if err != nil {
			jot.Warn(errs.NewWithCause("unable to parse URL", err))
			continue
		}
		result = append(result, u.Path)
	}
	return result
}

func (a Array) ArrayOfStringToStringSlice() []string {
	count := a.Count()
	result := make([]string, 0, count)
	for i := 0; i < count; i++ {
		result = append(result, a.StringAtIndex(i).String())
	}
	return result
}

type URL C.CFURLRef

func NewFileURL(str string) URL {
	var isDir C.uchar
	if strings.HasSuffix(str, "/") || fs.IsDir(str) {
		isDir = 1
	}
	header := (*reflect.StringHeader)(unsafe.Pointer(&str))
	return URL(C.CFURLCreateFromFileSystemRepresentation(0, (*C.uint8)(unsafe.Pointer(header.Data)), C.long(header.Len), isDir))
}

func (u URL) AbsoluteString() string {
	other := C.CFURLCopyAbsoluteURL(C.CFURLRef(u))
	s := String(C.CFURLGetString(other))
	URL(other).Release()
	defer s.Release()
	return s.String()
}

func (u URL) Release() {
	C.CFRelease(C.CFTypeRef(u))
}

type OpenPanel C.NSOpenPanelRef

func NewOpenPanel() OpenPanel {
	return OpenPanel(C.newOpenPanel())
}

func (p OpenPanel) DirectoryURL() URL {
	return URL(C.openPanelDirectoryURL(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetDirectoryURL(url URL) {
	C.openPanelSetDirectoryURL(C.NSOpenPanelRef(p), C.CFURLRef(url))
}

func (p OpenPanel) AllowedFileTypes() Array {
	return Array(C.openPanelAllowedFileTypes(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetAllowedFileTypes(types Array) {
	C.openPanelSetAllowedFileTypes(C.NSOpenPanelRef(p), C.CFArrayRef(types))
}

func (p OpenPanel) CanChooseFiles() bool {
	return bool(C.openPanelCanChooseFiles(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetCanChooseFiles(set bool) {
	C.openPanelSetCanChooseFiles(C.NSOpenPanelRef(p), C.bool(set))
}

func (p OpenPanel) CanChooseDirectories() bool {
	return bool(C.openPanelCanChooseDirectories(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetCanChooseDirectories(set bool) {
	C.openPanelSetCanChooseDirectories(C.NSOpenPanelRef(p), C.bool(set))
}

func (p OpenPanel) ResolvesAliases() bool {
	return bool(C.openPanelResolvesAliases(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetResolvesAliases(set bool) {
	C.openPanelSetResolvesAliases(C.NSOpenPanelRef(p), C.bool(set))
}

func (p OpenPanel) AllowsMultipleSelection() bool {
	return bool(C.openPanelAllowsMultipleSelection(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetAllowsMultipleSelection(set bool) {
	C.openPanelSetAllowsMultipleSelection(C.NSOpenPanelRef(p), C.bool(set))
}

func (p OpenPanel) URLs() Array {
	return Array(C.openPanelURLs(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) RunModal() bool {
	return bool(C.openPanelRunModal(C.NSOpenPanelRef(p)))
}

type SavePanel C.NSSavePanelRef

func NewSavePanel() SavePanel {
	return SavePanel(C.newSavePanel())
}

func (p SavePanel) DirectoryURL() URL {
	return URL(C.savePanelDirectoryURL(C.NSSavePanelRef(p)))
}

func (p SavePanel) SetDirectoryURL(url URL) {
	C.savePanelSetDirectoryURL(C.NSSavePanelRef(p), C.CFURLRef(url))
}

func (p SavePanel) AllowedFileTypes() Array {
	return Array(C.savePanelAllowedFileTypes(C.NSSavePanelRef(p)))
}

func (p SavePanel) SetAllowedFileTypes(types Array) {
	C.savePanelSetAllowedFileTypes(C.NSSavePanelRef(p), C.CFArrayRef(types))
}

func (p SavePanel) URL() URL {
	return URL(C.savePanelURL(C.NSSavePanelRef(p)))
}

func (p SavePanel) RunModal() bool {
	return bool(C.savePanelRunModal(C.NSSavePanelRef(p)))
}

type Window C.NSWindowRef

func (w Window) ContentView() View {
	return View(C.windowContentView(C.NSWindowRef(w)))
}

type View C.NSViewRef

func (v View) Frame() geom32.Rect {
	var frame C.NSRect
	C.viewFrame(C.NSViewRef(v), &frame)
	return geom32.Rect{
		Point: geom32.Point{
			X: float32(frame.origin.x),
			Y: float32(frame.origin.y),
		},
		Size: geom32.Size{
			Width:  float32(frame.size.width),
			Height: float32(frame.size.height),
		},
	}
}

func IsDarkModeEnabled() bool {
	if style := C.CFPreferencesCopyAppValue(C.CFStringRef(NewString("AppleInterfaceStyle")),
		C.kCFPreferencesCurrentApplication); style != 0 {
		s := String(style)
		str := s.String()
		s.Release()
		return strings.Contains(strings.ToLower(str), "dark")
	}
	return false
}

type Menu C.NSMenuRef

func NewMenu(title string, updater func(Menu)) Menu {
	s := NewString(title)
	m := Menu(C.newMenu(C.CFStringRef(s)))
	s.Release()
	if updater != nil {
		menuUpdaters[m] = updater
	}
	return m
}

func (m Menu) NumberOfItems() int {
	return int(C.menuNumberOfItems(C.NSMenuRef(m)))
}

func (m Menu) ItemAtIndex(index int) MenuItem {
	return MenuItem(C.menuItemAtIndex(C.NSMenuRef(m), C.int(index)))
}

func (m Menu) InsertItemAtIndex(item MenuItem, index int) {
	C.menuInsertItemAtIndex(C.NSMenuRef(m), C.NSMenuItemRef(item), C.int(index))
}

func (m Menu) RemoveItemAtIndex(index int) {
	C.menuRemoveItemAtIndex(C.NSMenuRef(m), C.int(index))
}

func (m Menu) Title() string {
	return String(C.menuTitle(C.NSMenuRef(m))).String()
}

func (m Menu) Popup(wnd Window, menu Menu, item MenuItem, bounds geom32.Rect) {
	C.menuPopup(C.NSWindowRef(wnd), C.NSMenuRef(menu), C.NSMenuItemRef(item), C.CGRect{
		origin: C.CGPoint{
			x: C.double(bounds.X),
			y: C.double(bounds.Y),
		},
		size: C.CGSize{
			width:  C.double(bounds.Width),
			height: C.double(bounds.Height),
		},
	})
}

func (m Menu) Release() {
	delete(menuUpdaters, m)
	for i := m.NumberOfItems() - 1; i >= 0; i-- {
		item := m.ItemAtIndex(i)
		delete(menuItemValidators, item)
		delete(menuItemHandlers, item)
	}
	C.CFRelease(C.CFTypeRef(m))
}

type MenuItem C.NSMenuItemRef

func NewMenuItem(tag int, title string, keyEquivalent string, modifiers EventModifierFlags, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
	titleStr := NewString(title)
	keyStr := NewString(keyEquivalent)
	item := MenuItem(C.newMenuItem(C.int(tag), C.CFStringRef(titleStr), C.CFStringRef(keyStr), C.NSEventModifierFlags(modifiers)))
	titleStr.Release()
	keyStr.Release()
	if validator != nil {
		menuItemValidators[item] = validator
	}
	if handler != nil {
		menuItemHandlers[item] = handler
	}
	return item
}

func NewSeparatorMenuItem() MenuItem {
	return MenuItem(C.newMenuSeparatorItem())
}

func (m MenuItem) Tag() int {
	return int(C.menuItemTag(C.NSMenuItemRef(m)))
}

func (m MenuItem) IsSeparatorItem() bool {
	return bool(C.menuItemIsSeparator(C.NSMenuItemRef(m)))
}

func (m MenuItem) Title() string {
	return String(C.menuItemTitle(C.NSMenuItemRef(m))).String()
}

func (m MenuItem) SetTitle(title string) {
	titleStr := NewString(title)
	C.menuItemSetTitle(C.NSMenuItemRef(m), C.CFStringRef(titleStr))
	titleStr.Release()
}

func (m MenuItem) Menu() Menu {
	return Menu(C.menuItemMenu(C.NSMenuItemRef(m)))
}

func (m MenuItem) SubMenu() Menu {
	return Menu(C.menuItemSubMenu(C.NSMenuItemRef(m)))
}

func (m MenuItem) SetSubMenu(menu Menu) {
	C.menuItemSetSubMenu(C.NSMenuItemRef(m), C.NSMenuRef(menu))
}

func (m MenuItem) State() ControlStateValue {
	return ControlStateValue(C.menuItemState(C.NSMenuItemRef(m)))
}

func (m MenuItem) SetState(state ControlStateValue) {
	C.menuItemSetState(C.NSMenuItemRef(m), C.NSControlStateValue(state))
}

type ActivationPolicy uint

const (
	ActivationPolicyRegular ActivationPolicy = iota
	ActivationPolicyAccessory
	ActivationPolicyProhibited
)

func HideApplication() {
	C.hideRunningApplication()
}

func HideOtherApplications() {
	C.hideOtherApplications()
}

func UnhideAllApplications() {
	C.unhideAllApplications()
}

func SetActivationPolicy(policy ActivationPolicy) {
	C.setActivationPolicy(C.NSApplicationActivationPolicy(policy))
}

func SetMainMenu(menu Menu) {
	C.setMainMenu(C.NSMenuRef(menu))
}

func SetServicesMenu(menu Menu) {
	C.setServicesMenu(C.NSMenuRef(menu))
}

func SetWindowsMenu(menu Menu) {
	C.setWindowsMenu(C.NSMenuRef(menu))
}

func SetHelpMenu(menu Menu) {
	C.setHelpMenu(C.NSMenuRef(menu))
}
