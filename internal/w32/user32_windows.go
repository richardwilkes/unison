// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	closeClipboardProc             = user32.NewProc("CloseClipboard")
	createIconIndirectProc         = user32.NewProc("CreateIconIndirect")
	destroyIconProc                = user32.NewProc("DestroyIcon")
	emptyClipboardProc             = user32.NewProc("EmptyClipboard")
	enumClipboardFormatsProc       = user32.NewProc("EnumClipboardFormats")
	getActiveWindowProc            = user32.NewProc("GetActiveWindow")
	getClipboardDataProc           = user32.NewProc("GetClipboardData")
	getClipboardSequenceNumberProc = user32.NewProc("GetClipboardSequenceNumber")
	procGetDCProc                  = user32.NewProc("GetDC")
	getDoubleClickTimeProc         = user32.NewProc("GetDoubleClickTime")
	getSysColorProc                = user32.NewProc("GetSysColor")
	loadImageWProc                 = user32.NewProc("LoadImageW")
	messageBeepProc                = user32.NewProc("MessageBeep")
	openClipboardProc              = user32.NewProc("OpenClipboard")
	releaseDCProc                  = user32.NewProc("ReleaseDC")
	setClipboardDataProc           = user32.NewProc("SetClipboardData")
)

// Clipboard format types https://docs.microsoft.com/en-us/windows/desktop/dataxchg/standard-clipboard-formats
const (
	CFNone         ClipboardFormat = 0
	CFText         ClipboardFormat = 1
	CFOEMText      ClipboardFormat = 7
	CFUnicodeText  ClipboardFormat = 13
	CFHDrop        ClipboardFormat = 15
	CFPrivateFirst ClipboardFormat = 0x0200
)

// ColorHighlight https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsyscolor
const ColorHighlight = 13

// BeepType https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
type BeepType uint

// Possible beep types https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
const (
	MBDefault  BeepType = 0
	MBError    BeepType = 0x10
	MBQuestion BeepType = 0x20
	MBWarning  BeepType = 0x30
	MBInfo     BeepType = 0x40
)

// Constants for some standard cursors
const (
	OCR_NORMAL = 32512
	OCR_HAND   = 32649
	OCR_IBEAM  = 32513
)

// Constants for image types.
const (
	IMAGE_ICON   = 1
	IMAGE_CURSOR = 2
)

// Constants for LoadImageW function.
const (
	LR_DEFAULT_SIZE = 0x40
	LR_SHARED       = 0x8000
)

// https://learn.microsoft.com/openspecs/windows_protocols/ms-wmf/4e588f70-bd92-4a6f-b77f-35d0feaf7a57
const (
	BI_RGB       = 0
	BI_RLE8      = 1
	BI_RLE4      = 2
	BI_BITFIELDS = 3
	BI_JPEG      = 4
	BI_PNG       = 5
	BI_CMYK      = 11
	BI_CMYKRLE8  = 12
	BI_CMYKRLE4  = 13
	BI_1632      = 842217009
)

// https://learn.microsoft.com/windows/win32/api/winuser/ns-winuser-iconinfo
type ICONINFO struct {
	Icon     int32 // 1 for icon, 0 for cursor.
	XHotspot uint32
	YHotspot uint32
	Mask     HBITMAP
	Color    HBITMAP
}

// CloseClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-closeclipboard
func CloseClipboard() bool {
	b, _, _ := closeClipboardProc.Call()
	return b&0xff != 0
}

// CreateIconIndirect https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-createiconindirect
func CreateIconIndirect(info *ICONINFO) HICON {
	ret, _, _ := createIconIndirectProc.Call(uintptr(unsafe.Pointer(info)))
	return HICON(ret)
}

// DestroyIcon https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-destroyicon
func DestroyIcon(icon HICON) bool {
	b, _, _ := destroyIconProc.Call(uintptr(icon))
	return b&0xff != 0
}

// EmptyClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-emptyclipboard
func EmptyClipboard() bool {
	b, _, _ := emptyClipboardProc.Call()
	return b&0xff != 0
}

// EnumClipboardFormats https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-enumclipboardformats
func EnumClipboardFormats(format ClipboardFormat) ClipboardFormat {
	r, _, _ := enumClipboardFormatsProc.Call(uintptr(format))
	return ClipboardFormat(r)
}

// GetActiveWindow https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getactivewindow
func GetActiveWindow() HWND {
	hwnd, _, _ := getActiveWindowProc.Call()
	return HWND(hwnd)
}

// GetClipboardData https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclipboarddata
func GetClipboardData(format ClipboardFormat) syscall.Handle {
	h, _, _ := getClipboardDataProc.Call(uintptr(format))
	return syscall.Handle(h)
}

// GetClipboardSequenceNumber https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getclipboardsequencenumber
func GetClipboardSequenceNumber() int {
	num, _, _ := getClipboardSequenceNumberProc.Call()
	return int(num)
}

// GetDC https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-getdc
func GetDC(hwnd HWND) HDC {
	dc, _, _ := procGetDCProc.Call(uintptr(hwnd))
	return HDC(dc)
}

// GetDoubleClickTime https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getdoubleclicktime
func GetDoubleClickTime() time.Duration {
	millis, _, _ := getDoubleClickTimeProc.Call()
	return time.Millisecond * time.Duration(millis)
}

// GetSysColor https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getsyscolor
func GetSysColor(index int) uint32 {
	color, _, _ := getSysColorProc.Call(uintptr(index))
	return uint32(color)
}

// LoadImageW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-loadimagew
func LoadImageW(inst HINSTANCE, name UTF16String, typ uint32, cx, cy int, load uint32) HANDLE {
	ret, _, _ := loadImageWProc.Call(uintptr(inst), uintptr(unsafe.Pointer(name)), uintptr(typ), uintptr(cx),
		uintptr(cy), uintptr(load))
	return HANDLE(ret)
}

// MakeIntResourceW https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-makeintresourcew
func MakeIntResourceW(id int) UTF16String {
	return UTF16String(unsafe.Pointer(uintptr(id)))
}

// MessageBeep https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
func MessageBeep(beepType BeepType) bool {
	b, _, _ := messageBeepProc.Call(uintptr(beepType))
	return b&0xff != 0
}

// OpenClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-openclipboard
func OpenClipboard(newOwner HWND) bool {
	b, _, _ := openClipboardProc.Call(uintptr(newOwner))
	return b&0xff != 0
}

// ReleaseDC https://learn.microsoft.com/windows/win32/api/winuser/nf-winuser-releasedc
func ReleaseDC(hwnd HWND, dc HDC) bool {
	ret, _, _ := releaseDCProc.Call(uintptr(hwnd), uintptr(dc))
	return ret == 1
}

// SetClipboardData https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setclipboarddata
func SetClipboardData(format ClipboardFormat, handle syscall.Handle) syscall.Handle {
	h, _, _ := setClipboardDataProc.Call(uintptr(format), uintptr(handle))
	return syscall.Handle(h)
}
