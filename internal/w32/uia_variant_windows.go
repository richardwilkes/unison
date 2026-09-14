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
	"math"
	"unicode/utf16"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/xruntime"
)

// The OLE automation allocator's BSTR entry point lives here rather than with the rest of oleaut32.dll, so that the
// one place a BSTR can come into existence sits next to the comment on SysAllocStringLen explaining who owns the
// result. A hygiene test in w32_test.go checks that no other file in the package reaches for it.
var sysAllocStringLenProc = oleaut32.NewProc("SysAllocStringLen")

// VARIANT is the OLE automation tagged union, in its 64-bit layout: a 2-byte type tag, six reserved bytes, and a
// 16-byte union that every value this package stores fits in the first eight bytes of. The size is asserted by
// TestVariantSize, since UI Automation passes VARIANTs across the ABI by value and a wrong size would corrupt the
// stack rather than merely misreport a property.
//
// A VARIANT is inert until one of the Set methods fills it in, and the zero value is a valid VT_EMPTY variant, which is
// what a provider returns for a property it does not supply.
//
// Ownership works one of two ways, and mixing them up either leaks or double-frees:
//
//   - A VARIANT written into an out-parameter — the *VARIANT that IRawElementProviderSimple::GetPropertyValue and the
//     pattern property getters are handed — belongs to UI Automation Core from the moment the method returns S_OK. Core
//     calls VariantClear on it, so anything reachable from it (a BSTR, an interface reference, a SAFEARRAY) must not be
//     freed here.
//   - A VARIANT built locally to pass into one of the UiaRaise* functions stays ours, so it must be paired with a
//     deferred Clear.
type VARIANT struct {
	// VT is the type tag saying which kind of value Val holds.
	VT VARTYPE
	_  [3]uint16 // wReserved1, wReserved2 and wReserved3
	// Val holds the value itself: the low bits of an integer, the bits of a float64, or a pointer.
	Val uint64
	_   uint64 // The rest of the union, which nothing stored here uses
}

// Clear releases whatever the VARIANT owns and resets it to the zero value. It is safe to call more than once and on a
// VARIANT that was never filled in, so it can be deferred as soon as the variable is declared.
//
// VariantClear alone is not enough: it releases what the VARIANT owns and rewrites the type tag, but leaves the union
// holding whatever was there, which for a BSTR or a SAFEARRAY is a pointer to memory it has just freed. Zeroing the
// whole VARIANT afterwards makes a cleared one indistinguishable from one that was never filled in, so nothing can be
// misled by a stale pointer sitting behind a VT_EMPTY tag.
func (v *VARIANT) Clear() {
	VariantClear(v)
	*v = VARIANT{}
}

// SetI4 stores a 32-bit signed integer. It is the right type for every UI Automation property documented as an int,
// including the control type, the heading level and the enumeration-valued properties such as ToggleState.
func (v *VARIANT) SetI4(value int32) {
	v.VT = VT_I4
	v.Val = uint64(uint32(value))
}

// SetR8 stores a 64-bit float, which is what UI Automation uses for every numeric measurement, including the
// RangeValue pattern's value, bounds and increments.
func (v *VARIANT) SetR8(value float64) {
	v.VT = VT_R8
	v.Val = math.Float64bits(value)
}

// SetBool stores a VARIANT_BOOL. True is stored as every bit set rather than as one, since that is what COM clients
// compare against.
func (v *VARIANT) SetBool(value bool) {
	boolean := VARIANT_FALSE
	if value {
		boolean = VARIANT_TRUE
	}
	v.VT = VT_BOOL
	v.Val = uint64(uint16(boolean))
}

// SetBSTR stores a freshly allocated BSTR holding s. The VARIANT takes ownership of the allocation, so whoever owns the
// VARIANT owns the string: a VARIANT handed to UI Automation Core needs nothing further, while one built for a
// UiaRaise* call must be cleared afterwards.
func (v *VARIANT) SetBSTR(s string) {
	v.VT = VT_BSTR
	v.Val = uint64(NewBSTR(s))
}

// SetUnknown stores an interface pointer. The reference passed in becomes the VARIANT's, so the caller must have added
// one on its own behalf — every provider pointer handed out through a VARIANT is AddRef'd first.
func (v *VARIANT) SetUnknown(unknown unsafe.Pointer) {
	v.VT = VT_UNKNOWN
	v.Val = uint64(uintptr(unknown))
}

// SetArray stores a SAFEARRAY of the given element type, taking ownership of it. Clearing the VARIANT destroys the
// array and releases whatever its elements own.
func (v *VARIANT) SetArray(elementType VARTYPE, array SAFEARRAY) {
	v.VT = VT_ARRAY | elementType
	v.Val = uint64(array)
}

// BSTR is a length-prefixed, NUL-terminated UTF-16 string allocated by the OLE automation allocator. It is held as a
// uintptr rather than a *uint16 because it never points into the Go heap: the memory belongs to the COM allocator, so
// the garbage collector has nothing to track, and keeping it out of the pointer world means the rest of the package
// cannot accidentally dereference a string it does not own. A zero BSTR is the empty string, and every function here
// accepts one.
type BSTR uintptr

// NewBSTR allocates a BSTR holding s. The caller owns the result: either hand it to something that takes ownership,
// such as VARIANT.SetBSTR, or release it with Free.
func NewBSTR(s string) BSTR {
	if s == "" {
		// SysAllocStringLen(nil, 0) still allocates, returning a valid zero-length BSTR rather than NULL, which is
		// what a client expects for an empty string property.
		return SysAllocStringLen(nil, 0)
	}
	encoded := utf16.Encode([]rune(s))
	return SysAllocStringLen(&encoded[0], len(encoded))
}

// Free releases the BSTR. A zero BSTR is ignored, so this may be deferred before the allocation is attempted.
func (str BSTR) Free() {
	SysFreeString(str)
}

// BSTRToString returns the contents of a BSTR as a Go string, or "" for a zero BSTR. Embedded NUL characters are
// preserved, since a BSTR's length comes from its prefix rather than from a terminator. It does not free the BSTR.
func BSTRToString(str BSTR) string {
	count := SysStringLen(str)
	if count == 0 {
		return ""
	}
	// A BSTR satisfies PtrFromUintptr's ~uintptr constraint, so the UTF-16 array it points at is reached the way every
	// other uintptr-to-pointer conversion in the package is: through the address of a local copy, which checkptr
	// instrumentation under -race does not treat as pointer arithmetic. A BSTR never refers to Go memory, so there is
	// nothing here for the garbage collector to lose track of either.
	return string(utf16.Decode(unsafe.Slice(xruntime.PtrFromUintptr[uint16](str), count)))
}

// SysAllocStringLen allocates a BSTR holding a copy of count UTF-16 code units from chars, appending the terminating
// NUL itself. chars may be nil when count is zero, which allocates a valid empty BSTR. This is a callee-allocates
// contract: the OLE automation allocator owns the memory and the caller owns the reference, which must end up either in
// something that takes ownership, such as a VARIANT, or in a call to SysFreeString.
//
// It is the only BSTR allocator in the package, and a hygiene test in w32_test.go keeps it that way. Concentrating
// allocation here is what makes the ownership rule above auditable: a reader can see every place a BSTR is born without
// searching the package, and the rest of the package allocates through NewBSTR.
//
// https://learn.microsoft.com/en-us/windows/win32/api/oleauto/nf-oleauto-sysallocstringlen
func SysAllocStringLen(chars *uint16, count int) BSTR {
	//nolint:errcheck // The result is enough for our purposes, and the error is not useful.
	r, _, _ := sysAllocStringLenProc.Call(uintptr(unsafe.Pointer(chars)), uintptr(count))
	return BSTR(r)
}
