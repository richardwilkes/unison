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
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	oleaut32 = windows.NewLazySystemDLL("oleaut32.dll")
	// The BSTR allocator is declared in uia_variant_windows.go rather than here, so that the one place a BSTR can be
	// born sits beside the comment explaining who then owns it. A hygiene test in w32_test.go keeps it there.
	sysFreeStringProc         = oleaut32.NewProc("SysFreeString")
	sysStringLenProc          = oleaut32.NewProc("SysStringLen")
	variantClearProc          = oleaut32.NewProc("VariantClear")
	safeArrayCreateVectorProc = oleaut32.NewProc("SafeArrayCreateVector")
	safeArrayDestroyProc      = oleaut32.NewProc("SafeArrayDestroy")
	safeArrayGetElementProc   = oleaut32.NewProc("SafeArrayGetElement")
	safeArrayGetUBoundProc    = oleaut32.NewProc("SafeArrayGetUBound")
	safeArrayPutElementProc   = oleaut32.NewProc("SafeArrayPutElement")
)

// SysFreeString releases a BSTR allocated by the OLE automation allocator. Passing a zero BSTR is explicitly allowed
// and does nothing, so no nil check is needed at the call site.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-sysfreestring
func SysFreeString(str BSTR) {
	//nolint:errcheck // Nothing we can do about an error here
	sysFreeStringProc.Call(uintptr(str))
}

// SysStringLen returns the number of UTF-16 code units in a BSTR, not counting the terminating NUL. A zero BSTR has
// length zero. The count is stored in front of the characters, so a BSTR may contain embedded NULs and this is the only
// correct way to measure one.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-sysstringlen
func SysStringLen(str BSTR) uint32 {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := sysStringLenProc.Call(uintptr(str))
	return uint32(r)
}

// VariantClear releases whatever a VARIANT owns — a BSTR, an interface reference, a SAFEARRAY — and leaves it VT_EMPTY.
// It is safe to call on an already-cleared VARIANT, so VARIANT.Clear may be deferred unconditionally.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-variantclear
func VariantClear(v *VARIANT) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := variantClearProc.Call(uintptr(unsafe.Pointer(v)))
	return r
}

// SafeArrayCreateVector creates a one-dimensional SAFEARRAY of count elements of the given type, with its first index
// at lowerBound. The elements start out zeroed. It returns a zero SAFEARRAY if the allocation fails.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-safearraycreatevector
func SafeArrayCreateVector(vt VARTYPE, lowerBound int32, count uint32) SAFEARRAY {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := safeArrayCreateVectorProc.Call(uintptr(vt), uintptr(lowerBound), uintptr(count))
	return SAFEARRAY(r)
}

// SafeArrayDestroy releases a SAFEARRAY and everything in it, releasing each element for the element types that own
// something (VT_BSTR, VT_UNKNOWN, VT_DISPATCH, VT_VARIANT). A zero SAFEARRAY is ignored.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-safearraydestroy
func SafeArrayDestroy(array SAFEARRAY) uintptr {
	if array == 0 {
		return uintptr(COM_S_OK)
	}
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := safeArrayDestroyProc.Call(uintptr(array))
	return r
}

// SafeArrayPutElement stores one element into a one-dimensional SAFEARRAY at the given index. The value is passed the
// way the element's type requires: for the scalar types (VT_I4, VT_R8 and friends) value points at the scalar, while
// for the types that are themselves pointers (VT_BSTR, VT_UNKNOWN, VT_DISPATCH) value *is* that pointer rather than a
// pointer to it. Storing a VT_UNKNOWN element adds a reference to the interface, so the caller keeps, and must still
// release, the reference it passed in.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-safearrayputelement
func SafeArrayPutElement(array SAFEARRAY, index int32, value unsafe.Pointer) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := safeArrayPutElementProc.Call(uintptr(array), uintptr(unsafe.Pointer(&index)), uintptr(value))
	return r
}

// SafeArrayGetElement copies one element out of a one-dimensional SAFEARRAY into the storage value points at. The
// caller is responsible for whatever comes back owning something: a VT_BSTR element arrives as a BSTR that must be
// freed, and a VT_UNKNOWN element arrives with a reference that must be released.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-safearraygetelement
func SafeArrayGetElement(array SAFEARRAY, index int32, value unsafe.Pointer) uintptr {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := safeArrayGetElementProc.Call(uintptr(array), uintptr(unsafe.Pointer(&index)), uintptr(value))
	return r
}

// SafeArrayGetUBound returns the largest valid index of the given one-based dimension of a SAFEARRAY, along with the
// HRESULT of the call. For a vector built by SafeArrayCreateVector with a lower bound of zero, the result is one less
// than the element count.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-safearraygetubound
func SafeArrayGetUBound(array SAFEARRAY, dimension uint32) (bound int32, hr uintptr) {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := safeArrayGetUBoundProc.Call(uintptr(array), uintptr(dimension), uintptr(unsafe.Pointer(&bound)))
	return bound, r
}
