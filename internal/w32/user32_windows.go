// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	closeClipboardProc             = user32.NewProc("CloseClipboard")
	emptyClipboardProc             = user32.NewProc("EmptyClipboard")
	enumClipboardFormatsProc       = user32.NewProc("EnumClipboardFormats")
	getActiveWindowProc            = user32.NewProc("GetActiveWindow")
	getClipboardDataProc           = user32.NewProc("GetClipboardData")
	getClipboardSequenceNumberProc = user32.NewProc("GetClipboardSequenceNumber")
	getDoubleClickTimeProc         = user32.NewProc("GetDoubleClickTime")
	getSysColorProc                = user32.NewProc("GetSysColor")
	messageBeepProc                = user32.NewProc("MessageBeep")
	openClipboardProc              = user32.NewProc("OpenClipboard")
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

// CloseClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-closeclipboard
func CloseClipboard() bool {
	b, _, _ := closeClipboardProc.Call()
	return b != 0
}

// EmptyClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-emptyclipboard
func EmptyClipboard() bool {
	b, _, _ := emptyClipboardProc.Call()
	return b != 0
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

// MessageBeep https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-messagebeep
func MessageBeep(beepType BeepType) bool {
	b, _, _ := messageBeepProc.Call(uintptr(beepType))
	return b != 0
}

// OpenClipboard https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-openclipboard
func OpenClipboard(newOwner HWND) bool {
	b, _, _ := openClipboardProc.Call(uintptr(newOwner))
	return b != 0
}

// SetClipboardData https://docs.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-setclipboarddata
func SetClipboardData(format ClipboardFormat, handle syscall.Handle) syscall.Handle {
	h, _, _ := setClipboardDataProc.Call(uintptr(format), uintptr(handle))
	return syscall.Handle(h)
}
