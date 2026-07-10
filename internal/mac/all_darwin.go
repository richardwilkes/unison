// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
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
	"net/url"
	"strings"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/drag"
)

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

// ========= DragInfo ==========

type DragInfo C.NSDraggingInfoRef

var _ drag.Info = DragInfo(0)

type DragOp C.NSDragOperation

const (
	DragOpCopy DragOp = DragOp(C.NSDragOperationCopy)
	DragOpMove DragOp = DragOp(C.NSDragOperationMove)
	DragOpNone DragOp = DragOp(C.NSDragOperationNone)
)

func DragOpFromUnison(op drag.Op) DragOp {
	var nativeOp DragOp
	if op&drag.Copy != 0 {
		nativeOp |= DragOpCopy
	}
	if op&drag.Move != 0 {
		nativeOp |= DragOpMove
	}
	return nativeOp
}

func (d DragOp) ToUnisonDragOp() drag.Op {
	var op drag.Op
	if d&DragOpCopy != 0 {
		op |= drag.Copy
	}
	if d&DragOpMove != 0 {
		op |= drag.Move
	}
	return op
}

func (d DragInfo) SourceDragOpMask() drag.Op {
	return DragOp(C.dragSourceOperationMask(C.NSDraggingInfoRef(d))).ToUnisonDragOp()
}

func (d DragInfo) DataTypes() []string {
	return Array(C.dragDataTypes(C.NSDraggingInfoRef(d))).ArrayOfStringToStringSlice()
}

func (d DragInfo) HasString() bool {
	return bool(C.dragHasString(C.NSDraggingInfoRef(d)))
}

func (d DragInfo) HasFilePaths() bool {
	return bool(C.dragHasFilePaths(C.NSDraggingInfoRef(d)))
}

func (d DragInfo) HasURLs() bool {
	return bool(C.dragHasURLs(C.NSDraggingInfoRef(d)))
}

func (d DragInfo) HasDataType(dataType string) bool {
	s := NewString(dataType)
	defer s.Release()
	return bool(C.dragHasDataType(C.NSDraggingInfoRef(d), C.CFStringRef(s)))
}

func (d DragInfo) Text() string {
	s := C.dragText(C.NSDraggingInfoRef(d))
	if s == 0 {
		return ""
	}
	return String(s).String()
}

func (d DragInfo) FilePaths() []string {
	return Array(C.dragFilePaths(C.NSDraggingInfoRef(d))).ArrayOfStringToStringSlice()
}

func (d DragInfo) URLs() []*url.URL {
	urlStrs := Array(C.dragURLs(C.NSDraggingInfoRef(d))).ArrayOfURLToStringSlice()
	result := make([]*url.URL, 0, len(urlStrs))
	for _, urlStr := range urlStrs {
		u, err := url.Parse(urlStr)
		if err != nil {
			errs.Log(errs.NewWithCause("unable to parse URL", err), "url", urlStr)
			continue
		}
		result = append(result, u)
	}
	return result
}

func (d DragInfo) Data(dataType string) []byte {
	s := NewString(dataType)
	defer s.Release()
	var length uint64
	if buffer := C.dragBytes(C.NSDraggingInfoRef(d), C.CFStringRef(s), (*C.ulonglong)(&length)); buffer != nil && length > 0 {
		data := make([]byte, length)
		copy(data, unsafe.Slice((*byte)(unsafe.Pointer(buffer)), length))
		C.free(buffer)
		return data
	}
	return nil
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

type (
	Pasteboard     C.NSPasteboardRef
	PasteboardItem C.NSPasteboardItemRef
)

func PasteboardGeneral() Pasteboard {
	return Pasteboard(C.pasteboardGeneral())
}

func (p Pasteboard) AvailableDataTypes() []string {
	a := C.pasteboardAvailableDataTypes(C.NSPasteboardRef(p))
	return Array(a).ArrayOfStringToStringSlice()
}

func (p Pasteboard) HasDataType(dataType *uti.DataType) bool {
	s := NewString(dataType.UTI)
	defer s.Release()
	return bool(C.pasteboardHasDataType(C.NSPasteboardRef(p), C.CFStringRef(s)))
}

func (p Pasteboard) Bytes(dataType *uti.DataType) []byte {
	s := NewString(dataType.UTI)
	defer s.Release()
	var length uint64
	if buffer := C.pasteboardBytes(C.NSPasteboardRef(p), C.CFStringRef(s), (*C.ulonglong)(&length)); buffer != nil && length > 0 {
		data := make([]byte, length)
		copy(data, unsafe.Slice((*byte)(unsafe.Pointer(buffer)), length))
		C.free(buffer)
		return data
	}
	return nil
}

func (p Pasteboard) Clear() {
	C.pasteboardClearContents(C.NSPasteboardRef(p))
}

func (p Pasteboard) WriteItems(items ...PasteboardItem) {
	if len(items) == 0 {
		return
	}
	a := C.CFArrayCreateMutable(0, C.long(len(items)), &C.kCFTypeArrayCallBacks)
	defer Array(a).Release()
	for _, item := range items {
		C.CFArrayAppendValue(a, unsafe.Pointer(item))
	}
	C.pasteboardWriteObjects(C.NSPasteboardRef(p), C.CFArrayRef(a))
}

func NewPasteboardItem() PasteboardItem {
	return PasteboardItem(C.newPasteboardItem())
}

func (i PasteboardItem) SetString(s string) {
	str := NewString(s)
	defer str.Release()
	C.pasteboardItemSetString(C.NSPasteboardItemRef(i), C.CFStringRef(str))
}

func (i PasteboardItem) SetData(dataType *uti.DataType, data []byte) {
	dt := NewString(dataType.UTI)
	defer dt.Release()
	var ptr unsafe.Pointer
	if len(data) != 0 {
		ptr = unsafe.Pointer(&data[0])
	}
	C.pasteboardItemSetData(C.NSPasteboardItemRef(i), C.CFStringRef(dt), C.ulonglong(len(data)), ptr)
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

func (v View) BackingScale() geom.Point {
	return cgPointToPoint(C.viewBackingScale(C.NSViewRef(v)))
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

func (v View) BeginDraggingSession(img *image.NRGBA, frame geom.Rect, dragOpMask drag.Op, data ...drag.Data) {
	if len(data) == 0 {
		return
	}
	item := NewPasteboardItem()
	for _, d := range data {
		item.SetData(d.Type, d.Data)
	}
	imgRef := newNSImage(img.Pix, int(frame.Width), int(frame.Height), img.Rect.Dx(), img.Rect.Dy())
	defer Release(imgRef)
	dragItem := C.newDraggingItem(C.NSPasteboardItemRef(item), C.NSImageRef(imgRef), rectToCGRect(frame))
	C.viewBeginDraggingSession(C.NSViewRef(v), dragItem, C.NSDragOperation(DragOpFromUnison(dragOpMask)))
}

func (v View) RegisterDraggedTypes(types []*uti.DataType) {
	t := make([]string, 0, len(types))
	for _, dt := range types {
		t = append(t, dt.UTI)
	}
	C.viewRegisterDraggedTypes(C.NSViewRef(v), C.CFArrayRef(NewArrayFromStringSlice(t)))
}

func (v View) UnregisterDraggedTypes() {
	C.viewUnregisterDraggedTypes(C.NSViewRef(v))
}

// ========== View callbacks ==========
// These are still invoked from the Objective-C macContentView in view_darwin.m and will move to Go when the view is
// ported. The Window type itself now lives in window_darwin.go (purego-based), so the exported shims take
// C.NSWindowRef (a CFTypeRef, which cgo maps to uintptr) and convert.

var WindowKeyPressedCallback func(w Window, key uint16, mods uint)

//export goWindowKeyPressedCallback
func goWindowKeyPressedCallback(w C.NSWindowRef, key uint16, mods uint) {
	if WindowKeyPressedCallback != nil {
		WindowKeyPressedCallback(Window(w), key, mods)
	}
}

var WindowKeyTypedCallback func(w Window, ch rune)

//export goWindowKeyTypedCallback
func goWindowKeyTypedCallback(w C.NSWindowRef, ch rune) {
	if WindowKeyTypedCallback != nil {
		WindowKeyTypedCallback(Window(w), ch)
	}
}

var WindowKeyReleasedCallback func(w Window, key uint16, mods uint)

//export goWindowKeyReleasedCallback
func goWindowKeyReleasedCallback(w C.NSWindowRef, key uint16, mods uint) {
	if WindowKeyReleasedCallback != nil {
		WindowKeyReleasedCallback(Window(w), key, mods)
	}
}

var WindowCursorUpdateCallback func(Window)

//export goWindowCursorUpdateCallback
func goWindowCursorUpdateCallback(w C.NSWindowRef) {
	if WindowCursorUpdateCallback != nil {
		WindowCursorUpdateCallback(Window(w))
	}
}

var WindowMouseEnterCallback func(w Window, pt geom.Point, mods uint)

//export goWindowMouseEnterCallback
func goWindowMouseEnterCallback(w C.NSWindowRef, x, y float32, mods uint) {
	if WindowMouseEnterCallback != nil {
		WindowMouseEnterCallback(Window(w), geom.NewPoint(x, y), mods)
	}
}

var WindowMouseExitCallback func(Window)

//export goWindowMouseExitCallback
func goWindowMouseExitCallback(w C.NSWindowRef) {
	if WindowMouseExitCallback != nil {
		WindowMouseExitCallback(Window(w))
	}
}

var WindowMouseMovedCallback func(w Window, pt geom.Point, mods uint)

//export goWindowMouseMovedCallback
func goWindowMouseMovedCallback(w C.NSWindowRef, x, y float32, mods uint) {
	if WindowMouseMovedCallback != nil {
		WindowMouseMovedCallback(Window(w), geom.NewPoint(x, y), mods)
	}
}

var WindowScrollCallback func(w Window, delta geom.Point, mods uint)

//export goWindowScrollCallback
func goWindowScrollCallback(w C.NSWindowRef, deltaX, deltaY float32, mods uint) {
	if WindowScrollCallback != nil {
		WindowScrollCallback(Window(w), geom.NewPoint(deltaX, deltaY), mods)
	}
}

var WindowMouseClickCallback func(w Window, button int, where geom.Point, pressed bool, mods uint)

//export goWindowMouseClickCallback
func goWindowMouseClickCallback(w C.NSWindowRef, button int, x, y float32, pressed bool, mods uint) {
	if WindowMouseClickCallback != nil {
		WindowMouseClickCallback(Window(w), button, geom.NewPoint(x, y), pressed, mods)
	}
}

var WindowUpdateLayerCallback func(Window)

//export goWindowUpdateLayerCallback
func goWindowUpdateLayerCallback(w C.NSWindowRef) {
	if WindowUpdateLayerCallback != nil {
		WindowUpdateLayerCallback(Window(w))
	}
}

var WindowScaleCallback func(w Window, scale geom.Point)

//export goWindowScaleCallback
func goWindowScaleCallback(w C.NSWindowRef, scale C.CGPoint) {
	if WindowScaleCallback != nil {
		WindowScaleCallback(Window(w), cgPointToPoint(scale))
	}
}

var WindowRedrawCallback func(Window)

//export goWindowRedrawCallback
func goWindowRedrawCallback(w C.NSWindowRef) {
	if WindowRedrawCallback != nil {
		WindowRedrawCallback(Window(w))
	}
}

var WindowDragEnterCallback func(w Window, d DragInfo, where geom.Point, mods uint) drag.Op

//export goWindowDragEnterCallback
func goWindowDragEnterCallback(w C.NSWindowRef, d DragInfo, x, y float32, mods uint) DragOp {
	if WindowDragEnterCallback != nil {
		return DragOpFromUnison(WindowDragEnterCallback(Window(w), d, geom.NewPoint(x, y), mods) & d.SourceDragOpMask())
	}
	return DragOpNone
}

var WindowDragUpdateCallback func(w Window, d DragInfo, where geom.Point, mods uint) drag.Op

//export goWindowDragUpdateCallback
func goWindowDragUpdateCallback(w C.NSWindowRef, d DragInfo, x, y float32, mods uint) DragOp {
	if WindowDragUpdateCallback != nil {
		return DragOpFromUnison(WindowDragUpdateCallback(Window(w), d, geom.NewPoint(x, y), mods) & d.SourceDragOpMask())
	}
	return DragOpNone
}

var WindowDropCallback func(w Window, d DragInfo, where geom.Point, mods uint) bool

//export goWindowDropCallback
func goWindowDropCallback(w C.NSWindowRef, d DragInfo, x, y float32, mods uint) bool {
	var handled bool
	if WindowDropCallback != nil {
		handled = WindowDropCallback(Window(w), d, geom.NewPoint(x, y), mods)
	}
	return handled
}

var WindowDragExitCallback func(w Window)

//export goWindowDragExitCallback
func goWindowDragExitCallback(w C.NSWindowRef) {
	if WindowDragExitCallback != nil {
		WindowDragExitCallback(Window(w))
	}
}

var WindowDragSourceFinishedCallback func(w Window)

//export goWindowDragSourceFinishedCallback
func goWindowDragSourceFinishedCallback(w C.NSWindowRef) {
	if WindowDragSourceFinishedCallback != nil {
		WindowDragSourceFinishedCallback(Window(w))
	}
}
