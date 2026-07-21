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
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// comRefCount probes obj's reference count with an AddRef/Release pair and returns the steady-state count. IUnknown's
// AddRef and Release both return the updated count; COM documents the values as reliable only for testing, which is
// exactly this use.
func comRefCount(obj *Unknown) uintptr {
	//nolint:errcheck // The updated count comes back in r1; these calls have no error to check.
	count, _, _ := syscall.SyscallN(obj.vmt().AddRef, uintptr(unsafe.Pointer(obj)))
	//nolint:errcheck // See above.
	syscall.SyscallN(obj.vmt().Release, uintptr(unsafe.Pointer(obj)))
	return count - 1
}

// releaseReturningCount is Unknown.Release, except it reports the count IUnknown::Release returned, so a test can
// observe that dropping the creator's reference actually destroys the object.
func releaseReturningCount(obj *Unknown) uintptr {
	//nolint:errcheck // The updated count comes back in r1; the call has no error to check.
	count, _, _ := syscall.SyscallN(obj.vmt().Release, uintptr(unsafe.Pointer(obj)))
	return count
}

// TestFileDialogComLifecycle verifies that NewOpenDialog and NewSaveDialog hand back exactly one (+1) COM reference
// and that a single Release balances it, destroying the dialog object. The root dialogs' RunModal defers Release on
// that reference; before it did, every file-dialog use leaked the COM object and its shell state. IFileDialog is an
// STA-only object, so the test pins its goroutine to one OS thread and initializes OLE there, the same way the
// production UI thread does during startup.
func TestFileDialogComLifecycle(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := OleInitialize(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		//nolint:errcheck // Balances the OleInitialize above; nothing to do about a failure here.
		windows.NewLazySystemDLL("ole32.dll").NewProc("OleUninitialize").Call()
	}()
	for _, one := range []struct {
		create func() *Unknown
		name   string
	}{
		{name: "IFileOpenDialog", create: func() *Unknown { return (*Unknown)(unsafe.Pointer(NewOpenDialog())) }},
		{name: "IFileSaveDialog", create: func() *Unknown { return (*Unknown)(unsafe.Pointer(NewSaveDialog())) }},
	} {
		obj := one.create()
		if obj == nil {
			t.Errorf("unable to create %s", one.name)
			continue
		}
		if got := comRefCount(obj); got != 1 {
			t.Errorf("%s: reference count after creation = %d, want 1", one.name, got)
		}
		if remaining := releaseReturningCount(obj); remaining != 0 {
			t.Errorf("%s: Release left %d references, want 0 (destroyed)", one.name, remaining)
		}
	}
}
