// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// AttachParentProcessID https://docs.microsoft.com/en-us/windows/console/attachconsole
	AttachParentProcessID = ^uint32(0)
	// GMemMoveable https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalalloc
	GMemMoveable = 0x0002
)

var (
	kernel32                = syscall.NewLazyDLL("kernel32.dll")
	attachConsoleProc       = kernel32.NewProc("AttachConsole")
	getModuleHandleWProc    = kernel32.NewProc("GetModuleHandleW")
	globalAllocProc         = kernel32.NewProc("GlobalAlloc")
	globalFreeProc          = kernel32.NewProc("GlobalFree")
	globalLockProc          = kernel32.NewProc("GlobalLock")
	globalUnlockProc        = kernel32.NewProc("GlobalUnlock")
	verSetConditionMaskProc = kernel32.NewProc("VerSetConditionMask")
)

// AttachConsole https://docs.microsoft.com/en-us/windows/console/attachconsole
func AttachConsole(processID uint32) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r1, _, _ := attachConsoleProc.Call(uintptr(processID))
	return r1&0xff != 0
}

// GetModuleHandleW https://docs.microsoft.com/en-us/windows/win32/api/libloaderapi/nf-libloaderapi-getmodulehandlew
func GetModuleHandleW(name string) HMODULE {
	var moduleName *uint16
	if name != "" {
		utf16Name, err := windows.UTF16FromString(name)
		if err != nil {
			return 0
		}
		defer runtime.KeepAlive(utf16Name)
		moduleName = &utf16Name[0]
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := getModuleHandleWProc.Call(uintptr(unsafe.Pointer(moduleName)))
	return HMODULE(ret)
}

// GlobalAlloc https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalalloc
func GlobalAlloc(flags uint, size int) syscall.Handle {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	mem, _, _ := globalAllocProc.Call(uintptr(flags), uintptr(size))
	return syscall.Handle(mem)
}

// GlobalFree https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalfree
func GlobalFree(handle syscall.Handle) {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	_, _, _ = globalFreeProc.Call(uintptr(handle))
}

// GlobalLock https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globallock
func GlobalLock(handle syscall.Handle) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	p, _, _ := globalLockProc.Call(uintptr(handle))
	return p
}

// GlobalUnlock https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalunlock
func GlobalUnlock(handle syscall.Handle) bool {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	b, _, _ := globalUnlockProc.Call(uintptr(handle))
	return b&0xff != 0
}

// VerSetConditionMask https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-versetconditionmask
func VerSetConditionMask(mask uint64, typeMask uint32, condition byte) uint64 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	ret, _, _ := verSetConditionMaskProc.Call(uintptr(mask), uintptr(typeMask), uintptr(condition))
	return uint64(ret)
}
