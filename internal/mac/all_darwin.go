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
	"net/url"
	"strings"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
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
