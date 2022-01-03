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
)

const (
	// AttachParentProcessID https://docs.microsoft.com/en-us/windows/console/attachconsole
	AttachParentProcessID = ^uint32(0)
	// GMemMoveable https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalalloc
	GMemMoveable = 0x0002
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	attachConsoleProc = kernel32.NewProc("AttachConsole")
	globalAllocProc   = kernel32.NewProc("GlobalAlloc")
	globalFreeProc    = kernel32.NewProc("GlobalFree")
	globalLockProc    = kernel32.NewProc("GlobalLock")
	globalUnlockProc  = kernel32.NewProc("GlobalUnlock")
)

// AttachConsole https://docs.microsoft.com/en-us/windows/console/attachconsole
func AttachConsole(processID uint32) bool {
	r1, _, _ := attachConsoleProc.Call(uintptr(processID))
	return r1 != 0
}

// GlobalAlloc https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalalloc
func GlobalAlloc(flags uint, size int) syscall.Handle {
	mem, _, _ := globalAllocProc.Call(uintptr(flags), uintptr(size))
	return syscall.Handle(mem)
}

// GlobalFree https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalfree
func GlobalFree(handle syscall.Handle) {
	_, _, _ = globalFreeProc.Call(uintptr(handle))
}

// GlobalLock https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globallock
func GlobalLock(handle syscall.Handle) uintptr {
	p, _, _ := globalLockProc.Call(uintptr(handle))
	return p
}

// GlobalUnlock https://docs.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-globalunlock
func GlobalUnlock(handle syscall.Handle) bool {
	b, _, _ := globalUnlockProc.Call(uintptr(handle))
	return b != 0
}
