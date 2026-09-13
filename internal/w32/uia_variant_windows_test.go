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
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestVariantSize verifies the layout of VARIANT against the 64-bit ABI. UI Automation passes VARIANTs by value, which
// on both x64 and ARM64 means the caller allocates a 24-byte copy and passes its address; a VARIANT of the wrong size,
// or with its value at the wrong offset, would therefore corrupt whatever sits next to it rather than merely report a
// property incorrectly.
func TestVariantSize(t *testing.T) {
	c := check.New(t)
	var v VARIANT
	c.Equal(uintptr(24), unsafe.Sizeof(v))
	c.Equal(uintptr(0), unsafe.Offsetof(v.VT))
	c.Equal(uintptr(8), unsafe.Offsetof(v.Val))
	c.Equal(uintptr(2), unsafe.Sizeof(v.VT))
	c.Equal(VT_EMPTY, v.VT)
	c.Equal(uint64(0), v.Val)
}

// TestVariantScalarSetters verifies that each scalar setter tags the VARIANT correctly and stores the value where the
// ABI expects it. VARIANT_TRUE is the one that matters most: a client compares against every bit set, so storing 1 would
// read as neither true nor false.
func TestVariantScalarSetters(t *testing.T) {
	c := check.New(t)
	var v VARIANT

	v.SetI4(-2)
	c.Equal(VT_I4, v.VT)
	c.Equal(uint64(0xFFFFFFFE), v.Val)

	v.SetI4(int32(UIA_ButtonControlTypeId))
	c.Equal(VT_I4, v.VT)
	c.Equal(uint64(50000), v.Val)

	v.SetR8(-1.5)
	c.Equal(VT_R8, v.VT)
	c.Equal(-1.5, math.Float64frombits(v.Val))

	v.SetBool(true)
	c.Equal(VT_BOOL, v.VT)
	c.Equal(uint64(0xFFFF), v.Val)

	v.SetBool(false)
	c.Equal(VT_BOOL, v.VT)
	c.Equal(uint64(0), v.Val)
}

// TestBSTRRoundTrip verifies that a BSTR survives the trip through the OLE automation allocator unchanged, including the
// cases a naive NUL-terminated conversion would get wrong: an empty string must still produce a usable BSTR rather than
// NULL, an embedded NUL must be preserved because a BSTR's length comes from its prefix, and a character outside the
// basic multilingual plane must come back as one rune rather than as its two surrogates.
func TestBSTRRoundTrip(t *testing.T) {
	c := check.New(t)
	for _, s := range []string{"", "hello", "café", "a\x00b", "\U0001F600", "line1\nline2"} {
		str := NewBSTR(s)
		c.NotEqual(BSTR(0), str)
		c.Equal(s, BSTRToString(str))
		c.Equal(uint32(len(utf16.Encode([]rune(s)))), SysStringLen(str))
		str.Free()
	}
}

// TestBSTRZeroIsEmpty verifies that the zero BSTR behaves as the empty string everywhere, so that a failed allocation
// cannot turn into a crash: measuring it, reading it and freeing it must all be safe.
func TestBSTRZeroIsEmpty(t *testing.T) {
	c := check.New(t)
	c.Equal(uint32(0), SysStringLen(0))
	c.Equal("", BSTRToString(0))
	BSTR(0).Free()
}

// TestVariantBSTR verifies that a VARIANT holding a BSTR reports the string it was given and that clearing it releases
// the string and leaves an empty VARIANT behind.
func TestVariantBSTR(t *testing.T) {
	c := check.New(t)
	var v VARIANT
	v.SetBSTR("hello")
	c.Equal(VT_BSTR, v.VT)
	c.Equal("hello", BSTRToString(BSTR(v.Val)))
	v.Clear()
	c.Equal(VT_EMPTY, v.VT)
	c.Equal(uint64(0), v.Val)
}

// TestSafeArrayInt32RoundTrip verifies that the runtime identifier of a node survives being packed into a SAFEARRAY,
// which is the form IRawElementProviderFragment::GetRuntimeId has to return it in.
func TestSafeArrayInt32RoundTrip(t *testing.T) {
	c := check.New(t)
	values := []int32{UiaAppendRuntimeId, 7, 0}
	array := NewSafeArrayInt32(values)
	c.NotEqual(SAFEARRAY(0), array)
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(hresultSucceeded(hr))
	c.Equal(int32(2), bound)
	c.Equal(values, SafeArrayToInt32(array))
	array.Destroy()
}

// TestSafeArrayEmpty verifies that a zero-length vector is still a real array rather than a failure, since an element
// with no children reports an empty selection that way.
func TestSafeArrayEmpty(t *testing.T) {
	c := check.New(t)
	array := NewSafeArrayInt32(nil)
	c.NotEqual(SAFEARRAY(0), array)
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(hresultSucceeded(hr))
	c.Equal(int32(-1), bound)
	array.Destroy()
	SAFEARRAY(0).Destroy()
}

// TestSafeArrayFloat64 verifies that a vector of doubles is built with the right element count and can be read back,
// which is the shape a provider uses for anything measured rather than counted.
func TestSafeArrayFloat64(t *testing.T) {
	c := check.New(t)
	array := NewSafeArrayFloat64([]float64{1.5, -2.25})
	c.NotEqual(SAFEARRAY(0), array)
	bound, hr := SafeArrayGetUBound(array, 1)
	c.True(hresultSucceeded(hr))
	c.Equal(int32(1), bound)
	var second float64
	c.True(hresultSucceeded(SafeArrayGetElement(array, 1, unsafe.Pointer(&second))))
	c.Equal(-2.25, second)
	array.Destroy()
}

// TestVariantArray verifies that a VARIANT holding a SAFEARRAY is tagged with both the array flag and the element type,
// and that clearing it destroys the array.
func TestVariantArray(t *testing.T) {
	c := check.New(t)
	var v VARIANT
	v.SetArray(VT_I4, NewSafeArrayInt32([]int32{1, 2, 3}))
	c.Equal(VT_ARRAY|VT_I4, v.VT)
	c.NotEqual(uint64(0), v.Val)
	v.Clear()
	c.Equal(VT_EMPTY, v.VT)
	c.Equal(uint64(0), v.Val)
}
