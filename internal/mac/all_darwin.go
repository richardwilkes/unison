// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

// NOTE: A single Go file that imports the C package was an intentional choice here, as it dramatically reduces the
//       compile time of the package.

// Important information about memory management on macOS:
// https://developer.apple.com/library/archive/documentation/CoreFoundation/Conceptual/CFMemoryMgmt/Concepts/Ownership.html#//apple_ref/doc/uid/20001148

// #cgo CFLAGS: -x objective-c -Wno-deprecated-declarations
// #cgo LDFLAGS: -framework Cocoa
// #import "macos.h"
import "C"

import (
	"image"
	"image/draw"
	"net/url"
	"strings"
	"time"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// ========== App ==========

func InstallMacAppDelegate() error {
	if !C.installMacAppDelegate() {
		return errs.New("InstallMacAppDelegate: unable to install app delegate")
	}
	return nil
}

func UninstallMacAppDelegate() {
	C.uninstallMacAppDelegate()
}

func FinishLaunching() {
	C.finishLaunching()
}

func HideApplication() {
	C.hideRunningApplication()
}

func HideOtherApplications() {
	C.hideOtherApplications()
}

func UnhideAllApplications() {
	C.unhideAllApplications()
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

var AppShouldTerminateCallback func()

//export goAppShouldTerminateCallback
func goAppShouldTerminateCallback() {
	if AppShouldTerminateCallback != nil {
		AppShouldTerminateCallback()
	}
}

var AppDidChangeScreenParameters func()

//export goAppDidChangeScreenParametersCallback
func goAppDidChangeScreenParametersCallback() {
	if AppDidChangeScreenParameters != nil {
		AppDidChangeScreenParameters()
	}
}

var AppWillFinishLaunchingCallback func()

//export goAppWillFinishLaunchingCallback
func goAppWillFinishLaunchingCallback() {
	if AppWillFinishLaunchingCallback != nil {
		AppWillFinishLaunchingCallback()
	}
}

var AppDidFinishLaunchingCallback func()

//export goAppDidFinishLaunchingCallback
func goAppDidFinishLaunchingCallback() {
	if AppDidFinishLaunchingCallback != nil {
		AppDidFinishLaunchingCallback()
	}
}

var AppDidHideCallback func()

//export goAppDidHideCallback
func goAppDidHideCallback() {
	if AppDidHideCallback != nil {
		AppDidHideCallback()
	}
}

var OpenFilesCallback func([]string)

//export goOpenURLsCallback
func goOpenURLsCallback(a C.CFArrayRef) {
	if OpenFilesCallback != nil {
		if urls := Array(a).ArrayOfURLToStringSlice(); len(urls) > 0 {
			OpenFilesCallback(urls)
		}
	}
}

// ========== Array ==========

type (
	MutableArray C.CFMutableArrayRef
	Array        C.CFArrayRef
)

func NewArrayFromStringSlice(slice []string) Array {
	//nolint:gocritic // Spurious lint flagging due to C code
	a := C.CFArrayCreateMutable(0, C.long(len(slice)), &C.kCFTypeArrayCallBacks)
	for _, s := range slice {
		str := NewString(s)
		//nolint:govet // Spurious lint flagging due to C code
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
	for i := range count {
		urlStr := a.URLAtIndex(i).AbsoluteString()
		u, err := url.Parse(urlStr)
		if err != nil {
			errs.Log(errs.NewWithCause("unable to parse URL", err), "url", urlStr)
			continue
		}
		result = append(result, u.Path)
	}
	return result
}

func (a Array) ArrayOfStringToStringSlice() []string {
	count := a.Count()
	result := make([]string, 0, count)
	for i := range count {
		result = append(result, a.StringAtIndex(i).String())
	}
	return result
}

// ========== Cursor ==========

type Cursor C.NSCursorRef

func NewCursor(img *image.NRGBA, xhot, yhot int) Cursor {
	if img.Stride != img.Rect.Dx()*4 {
		nImg := image.NewNRGBA(image.Rect(0, 0, img.Rect.Dx(), img.Rect.Dy()))
		draw.Draw(nImg, nImg.Bounds(), img, img.Rect.Min, draw.Src)
		img = nImg
	}
	return Cursor(C.newCursor((*C.uchar)(&img.Pix[0]), C.int(img.Rect.Dx()), C.int(img.Rect.Dy()), C.int(xhot),
		C.int(yhot)))
}

func (c Cursor) Set() {
	C.cursorSet(C.NSCursorRef(c))
}

func (c Cursor) Release() {
	C.CFRelease(C.CFTypeRef(c))
}

func HideCursor() {
	C.cursorHide()
}

func ShowCursor() {
	C.cursorShow()
}

func ArrowCursor() Cursor {
	return Cursor(C.cursorArrow())
}

func IBeamCursor() Cursor {
	return Cursor(C.cursorIBeam())
}

func CrosshairCursor() Cursor {
	return Cursor(C.cursorCrosshair())
}

func PointingHandCursor() Cursor {
	return Cursor(C.cursorPointingHand())
}

func ResizeLeftRightCursor() Cursor {
	return Cursor(C.cursorResizeLeftRight())
}

func ResizeUpDownCursor() Cursor {
	return Cursor(C.cursorResizeUpDown())
}

// ========== Display ==========

type DisplayID = C.CGDirectDisplayID

func ActiveDisplayList() []DisplayID {
	var displayIDs [16]C.CGDirectDisplayID
	var count C.uint32
	C.CGGetActiveDisplayList(C.uint32(len(displayIDs)), &displayIDs[0], &count)
	return displayIDs[:int(count)]
}

func MainDisplayID() DisplayID {
	return C.CGMainDisplayID()
}

func DisplayIsAsleep(id DisplayID) bool {
	return C.CGDisplayIsAsleep(id) != 0
}

func DisplayBounds(id DisplayID) geom.Rect {
	return cgRectToRect(C.CGDisplayBounds(id))
}

func DisplayScreenSize(id DisplayID) geom.Size {
	sizeMM := C.CGDisplayScreenSize(id)
	return geom.NewSize(float32(sizeMM.width), float32(sizeMM.height))
}

// ========== Event ==========

type EventModifierFlags uint

const (
	EventModifierFlagCapsLock EventModifierFlags = 1 << (16 + iota)
	EventModifierFlagShift
	EventModifierFlagControl
	EventModifierFlagOption
	EventModifierFlagCommand
)

func DoubleClickInterval() time.Duration {
	return time.Duration(C.doubleClickInterval()*1000) * time.Millisecond
}

func CurrentModifierFlags() EventModifierFlags {
	return EventModifierFlags(C.eventModifierFlags())
}

func PostEmptyEvent() {
	C.postEmptyEvent()
}

func StopMainEventLoop() {
	C.stopMainEventLoop()
}

// ========== Menu ==========

type Menu C.NSMenuRef

var menuUpdaters = make(map[Menu]func(Menu))

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

func (m Menu) RemoveAll() {
	C.menuRemoveAll(C.NSMenuRef(m))
}

func (m Menu) Title() string {
	return String(C.menuTitle(C.NSMenuRef(m))).String()
}

func (m Menu) Popup(wnd Window, menu Menu, item MenuItem, bounds geom.Rect) {
	C.menuPopup(C.NSWindowRef(wnd), C.NSMenuRef(menu), C.NSMenuItemRef(item), rectToCGRect(bounds))
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

//export goUpdateMenuCallback
func goUpdateMenuCallback(m C.NSMenuRef) {
	menu := Menu(m)
	if updater, ok := menuUpdaters[menu]; ok && updater != nil {
		updater(menu)
	}
}

// ========== Menu Item ==========

type ControlStateValue int

const (
	ControlStateValueMixed ControlStateValue = iota - 1
	ControlStateValueOff
	ControlStateValueOn
)

type MenuItem C.NSMenuItemRef

var (
	menuItemValidators = make(map[MenuItem]func(item MenuItem) bool)
	menuItemHandlers   = make(map[MenuItem]func(item MenuItem))
)

func NewMenuItem(tag int, title, keyEquivalent string, modifiers EventModifierFlags, validator func(MenuItem) bool, handler func(MenuItem)) MenuItem {
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

func (m MenuItem) KeyBinding() (keyEquivalent string, modifiers EventModifierFlags) {
	ref := C.NSMenuItemRef(m)
	return String(C.menuItemKeyEquivalent(ref)).String(), EventModifierFlags(C.menuItemKeyEquivalentModifierMask(ref))
}

func (m MenuItem) SetKeyBinding(keyEquivalent string, modifiers EventModifierFlags) {
	keyStr := NewString(keyEquivalent)
	C.menuItemSetKeyBinding(C.NSMenuItemRef(m), C.CFStringRef(keyStr), C.NSEventModifierFlags(modifiers))
	keyStr.Release()
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

//export goMenuItemValidateCallback
func goMenuItemValidateCallback(mi C.NSMenuItemRef) bool {
	item := MenuItem(mi)
	if validator, ok := menuItemValidators[item]; ok && validator != nil {
		return validator(item)
	}
	return true
}

//export goMenuItemHandleCallback
func goMenuItemHandleCallback(mi C.NSMenuItemRef) {
	item := MenuItem(mi)
	if handler, ok := menuItemHandlers[item]; ok && handler != nil {
		handler(item)
	}
}

// ========== Open Panel ==========

type OpenPanel C.NSOpenPanelRef

func NewOpenPanel() OpenPanel {
	return OpenPanel(C.newOpenPanel())
}

func (p OpenPanel) DirectoryURL() URL {
	return URL(C.openPanelDirectoryURL(C.NSOpenPanelRef(p)))
}

func (p OpenPanel) SetDirectoryURL(theURL URL) {
	C.openPanelSetDirectoryURL(C.NSOpenPanelRef(p), C.CFURLRef(theURL))
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

// ========== OpenGL Context ==========

type OpenGLContextRef C.NSOpenGLContextRef

func NewOpenGLContext(view View, pixFmt OpenGLPixelFormatRef, shareCtx OpenGLContextRef, transparent bool) OpenGLContextRef {
	return OpenGLContextRef(C.newOpenGLContext(C.NSViewRef(view), C.NSOpenGLPixelFormatRef(pixFmt),
		C.NSOpenGLContextRef(shareCtx), C.bool(transparent)))
}

func (c OpenGLContextRef) MakeCurrent() {
	C.openGLMakeCurrent(C.NSOpenGLContextRef(c))
}

func (c OpenGLContextRef) FlushBuffer() {
	C.openGLFlushBuffer(C.NSOpenGLContextRef(c))
}

func (c OpenGLContextRef) Update() {
	C.openGLUpdate(C.NSOpenGLContextRef(c))
}

func (c OpenGLContextRef) Release() {
	C.CFRelease(C.CFTypeRef(c))
}

func ClearOpenGLCurrentContext() {
	C.openGLMakeCurrent(0)
}

// ========== OpenGL Pixel Format ==========

type OpenGLPixelFormatRef C.NSOpenGLPixelFormatRef

func NewOpenGLPixelFormat() OpenGLPixelFormatRef {
	return OpenGLPixelFormatRef(C.newOpenGLPixelFormat())
}

func (f OpenGLPixelFormatRef) Release() {
	C.CFRelease(C.CFTypeRef(f))
}

// ========== Pasteboard ==========

func PasteboardString() string {
	s := C.pasteboardString()
	if s == 0 {
		return ""
	}
	return String(s).String()
}

func SetPasteboardString(str string) {
	s := NewString(str)
	defer s.Release()
	C.pasteboardSetString(C.CFStringRef(s))
}

// ========== Save Panel ==========

type SavePanel C.NSSavePanelRef

func NewSavePanel() SavePanel {
	return SavePanel(C.newSavePanel())
}

func (p SavePanel) DirectoryURL() URL {
	return URL(C.savePanelDirectoryURL(C.NSSavePanelRef(p)))
}

func (p SavePanel) SetDirectoryURL(theURL URL) {
	C.savePanelSetDirectoryURL(C.NSSavePanelRef(p), C.CFURLRef(theURL))
}

func (p SavePanel) InitialFileName() string {
	return String(C.savePanelNameFieldStringValue(C.NSSavePanelRef(p))).String()
}

func (p SavePanel) SetInitialFileName(name string) {
	str := NewString(name)
	C.savePanelSetNameFieldStringValue(C.NSSavePanelRef(p), C.CFStringRef(str))
	str.Release()
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

// ========== Screen ==========

type Screen C.NSScreenRef

func ScreenForDisplayID(id DisplayID) Screen {
	return Screen(C.screenForDisplayID(id))
}

func (s Screen) Frame() geom.Rect {
	var frame C.CGRect
	C.screenFrame(C.NSScreenRef(s), &frame)
	return cgRectToRect(frame)
}

func (s Screen) VisibleFrame() geom.Rect {
	var frame C.CGRect
	C.screenVisibleFrame(C.NSScreenRef(s), &frame)
	return cgRectToRect(frame)
}

func (s Screen) ConvertRectToBacking(r geom.Rect) geom.Rect {
	backing := rectToCGRect(r)
	C.screenConvertRectToBacking(C.NSScreenRef(s), &backing)
	return cgRectToRect(backing)
}

// ========== Sound ==========

func Beep() {
	C.beep()
}

// ========== String ==========

type String C.CFStringRef

func NewString(str string) String {
	return String(C.CFStringCreateWithBytes(0, (*C.uint8)(unsafe.Pointer(unsafe.StringData(str))), C.long(len(str)),
		C.kCFStringEncodingUTF8, 0))
}

func (s String) String() string {
	strPtr := C.CFStringGetCStringPtr(C.CFStringRef(s), C.kCFStringEncodingUTF8)
	if strPtr == nil {
		maxBytes := 4*C.CFStringGetLength(C.CFStringRef(s)) + 1
		strPtr = (*C.char)(C.malloc(C.size_t(maxBytes)))
		defer C.free(unsafe.Pointer(strPtr))
		if C.CFStringGetCString(C.CFStringRef(s), strPtr, maxBytes, C.kCFStringEncodingUTF8) == 0 {
			errs.Log(errs.New("failed to convert string"))
			return ""
		}
	}
	return C.GoString(strPtr)
}

func (s String) Release() {
	C.CFRelease(C.CFTypeRef(s))
}

// ========== Theme ==========

var systemThemeChangedCallback func()

func InstallSystemThemeChangedCallback(f func()) {
	systemThemeChangedCallback = f
	if f != nil {
		C.installThemeChangedCallback()
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

//export goThemeChangedCallback
func goThemeChangedCallback() {
	if systemThemeChangedCallback != nil {
		systemThemeChangedCallback()
	}
}

// ========== URL ==========

type URL C.CFURLRef

func NewFileURL(str string) URL {
	var isDir C.uchar
	if strings.HasSuffix(str, "/") || xos.IsDir(str) {
		isDir = 1
	}
	return URL(C.CFURLCreateFromFileSystemRepresentation(0, (*C.uint8)(unsafe.Pointer(unsafe.StringData(str))),
		C.long(len(str)), isDir))
}

func (u URL) AbsoluteString() string {
	other := C.CFURLCopyAbsoluteURL(C.CFURLRef(u))
	str := String(C.CFURLGetString(other)).String()
	URL(other).Release()
	return str
}

func (u URL) Release() {
	C.CFRelease(C.CFTypeRef(u))
}

// ========== Utility ==========

func pointToCGPoint(pt geom.Point) C.CGPoint {
	return C.CGPoint{
		x: C.double(pt.X),
		y: C.double(pt.Y),
	}
}

func cgPointToPoint(pt C.CGPoint) geom.Point {
	return geom.NewPoint(float32(pt.x), float32(pt.y))
}

func rectToCGRect(r geom.Rect) C.CGRect {
	return C.CGRect{
		origin: C.CGPoint{
			x: C.double(r.X),
			y: C.double(r.Y),
		},
		size: C.CGSize{
			width:  C.double(r.Width),
			height: C.double(r.Height),
		},
	}
}

func cgRectToRect(r C.CGRect) geom.Rect {
	return geom.NewRect(float32(r.origin.x), float32(r.origin.y), float32(r.size.width), float32(r.size.height))
}

// ========== View ==========

type View C.NSViewRef

func NewView(w Window) View {
	return View(C.newView(C.NSWindowRef(w)))
}

func (v View) Frame() geom.Rect {
	var frame C.CGRect
	C.viewFrame(C.NSViewRef(v), &frame)
	return cgRectToRect(frame)
}

func (v View) MouseInRect(mousePt geom.Point, rect geom.Rect) bool {
	return bool(C.viewMouseInRect(C.NSViewRef(v), pointToCGPoint(mousePt), rectToCGRect(rect)))
}

func (v View) Release() {
	C.CFRelease(C.CFTypeRef(v))
}

// ========== Window ==========

type (
	Window                   C.NSWindowRef
	WindowStyleMask          = C.NSWindowStyleMask
	WindowCollectionBehavior = C.NSWindowCollectionBehavior
	WindowLevel              = C.NSWindowLevel
	WindowTabbingMode        = C.NSWindowTabbingMode
)

const (
	WindowStyleMaskMiniaturizable WindowStyleMask = C.NSWindowStyleMaskMiniaturizable
	WindowStyleMaskTitled         WindowStyleMask = C.NSWindowStyleMaskTitled
	WindowStyleMaskClosable       WindowStyleMask = C.NSWindowStyleMaskClosable
	WindowStyleMaskResizable      WindowStyleMask = C.NSWindowStyleMaskResizable
	WindowStyleMaskBorderless     WindowStyleMask = C.NSWindowStyleMaskBorderless
)

const (
	WindowCollectionBehaviorFullScreenPrimary WindowCollectionBehavior = C.NSWindowCollectionBehaviorFullScreenPrimary
	WindowCollectionBehaviorManaged           WindowCollectionBehavior = C.NSWindowCollectionBehaviorManaged
	WindowCollectionBehaviorFullScreenNone    WindowCollectionBehavior = C.NSWindowCollectionBehaviorFullScreenNone
)

const (
	WindowLevelNormal    WindowLevel = C.NSNormalWindowLevel
	WindowLevelFloating  WindowLevel = C.NSFloatingWindowLevel
	WindowLevelPopUpMenu WindowLevel = C.NSPopUpMenuWindowLevel
)

const (
	WindowTabbingModeAutomatic  WindowTabbingMode = C.NSWindowTabbingModeAutomatic
	WindowTabbingModeDisallowed WindowTabbingMode = C.NSWindowTabbingModeDisallowed
	WindowTabbingModePreferred  WindowTabbingMode = C.NSWindowTabbingModePreferred
)

func NewWindow(contentRect geom.Rect, styleMask WindowStyleMask, canBeKey, canBeMain bool) Window {
	return Window(C.newWindow(rectToCGRect(contentRect), styleMask, C.bool(canBeKey), C.bool(canBeMain)))
}

func (w Window) SetCollectionBehavior(behavior WindowCollectionBehavior) {
	C.windowSetCollectionBehavior(C.NSWindowRef(w), behavior)
}

func (w Window) SetLevel(level WindowLevel) {
	C.windowSetWindowLevel(C.NSWindowRef(w), level)
}

func (w Window) SetTransparent() {
	C.windowSetTransparent(C.NSWindowRef(w))
}

func (w Window) SetTitle(title string) {
	str := NewString(title)
	defer str.Release()
	C.windowSetTitle(C.NSWindowRef(w), C.CFStringRef(str))
}

func (w Window) ContentView() View {
	return View(C.windowContentView(C.NSWindowRef(w)))
}

func (w Window) SetContentView(v View) {
	C.windowSetContentView(C.NSWindowRef(w), C.NSViewRef(v))
}

func (w Window) SetRestorable(restorable bool) {
	C.windowSetRestorable(C.NSWindowRef(w), C.bool(restorable))
}

func (w Window) MakeFirstResponder(v View) {
	C.windowMakeFirstResponder(C.NSWindowRef(w), C.NSViewRef(v))
}

func (w Window) SetTabbingMode(mode WindowTabbingMode) {
	C.windowSetTabbingMode(C.NSWindowRef(w), mode)
}

func (w Window) SetAcceptsMouseMovedEvents(accepts bool) {
	C.windowSetAcceptsMouseMovedEvents(C.NSWindowRef(w), C.bool(accepts))
}

func (w Window) MouseLocationOutsideOfEventStream() geom.Point {
	return cgPointToPoint(C.windowMouseLocationOutsideOfEventStream(C.NSWindowRef(w)))
}

func (w Window) OrderOut() {
	C.windowOrderOut(C.NSWindowRef(w))
}

func (w Window) Delegate() WindowDelegate {
	return WindowDelegate(C.windowDelegate(C.NSWindowRef(w)))
}

func (w Window) SetDelegate(delegate WindowDelegate) {
	C.windowSetDelegate(C.NSWindowRef(w), C.NSWindowDelegateRef(delegate))
}

func (w Window) Focused() bool {
	return bool(C.windowFocused(C.NSWindowRef(w)))
}

func (w Window) Zoomed() bool {
	return bool(C.windowZoomed(C.NSWindowRef(w)))
}

func (w Window) Frame() geom.Rect {
	var frame C.CGRect
	C.windowFrame(C.NSWindowRef(w), &frame)
	return cgRectToRect(frame)
}

func (w Window) SetFrame(frameRect geom.Rect) {
	C.windowSetFrame(C.NSWindowRef(w), rectToCGRect(frameRect), true)
}

func (w Window) ContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	r := rectToCGRect(frameRect)
	C.windowContentRectForFrameRect(C.NSWindowRef(w), &r)
	return cgRectToRect(r)
}

func (w Window) FrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	r := rectToCGRect(contentRect)
	C.windowFrameRectForContentRect(C.NSWindowRef(w), &r)
	return cgRectToRect(r)
}

func (w Window) Close() {
	C.windowClose(C.NSWindowRef(w))
}

// ========== WindowDelegate ==========

type WindowDelegate C.NSWindowDelegateRef

func NewWindowDelegate(w Window) WindowDelegate {
	return WindowDelegate(C.newWindowDelegate(C.NSWindowRef(w)))
}

func (d WindowDelegate) Release() {
	C.CFRelease(C.CFTypeRef(d))
}

var WindowShouldCloseCallback func(Window)

//export goWindowShouldCloseCallback
func goWindowShouldCloseCallback(w Window) bool {
	if WindowShouldCloseCallback == nil {
		return true
	}
	WindowShouldCloseCallback(w)
	return false
}

var WindowDidResizeCallback func(Window)

//export goWindowDidResizeCallback
func goWindowDidResizeCallback(w Window) {
	if WindowDidResizeCallback != nil {
		WindowDidResizeCallback(w)
	}
}

var WindowDidMoveCallback func(Window)

//export goWindowDidMoveCallback
func goWindowDidMoveCallback(w Window) {
	if WindowDidMoveCallback != nil {
		WindowDidMoveCallback(w)
	}
}
