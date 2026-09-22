// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package uia

import (
	"unsafe"

	"github.com/richardwilkes/unison/internal/w32"
)

// SAFEARRAY is a self-describing array allocated by the OLE automation allocator. UI Automation uses one wherever a
// property or pattern method reports more than one value: a runtime identifier is a vector of VT_I4, a selection is a
// vector of VT_UNKNOWN, and so on.
//
// Like BSTR it is held as a uintptr rather than a pointer, because it never refers to Go memory. A zero SAFEARRAY means
// the allocation failed; every function here tolerates one, and a provider that gets one must report the property as
// unavailable rather than handing UI Automation a NULL array.
//
// Ownership follows the same rule as VARIANT: an array stored into a VARIANT or an out-parameter that UI Automation
// Core takes over is Core's to destroy, while one built to pass into a Raise* call is destroyed here.
type SAFEARRAY uintptr

// Destroy releases the array and everything in it. A zero SAFEARRAY is ignored, so this may be deferred before the
// allocation is attempted.
func (array SAFEARRAY) Destroy() {
	SafeArrayDestroy(array)
}

// NewSafeArrayInt32 builds a VT_I4 vector, indexed from zero, holding the given values. It returns a zero SAFEARRAY if
// the array cannot be built. This is the shape UI Automation wants for a runtime identifier and for every other
// integer-vector property.
func NewSafeArrayInt32(values []int32) SAFEARRAY {
	array := SafeArrayCreateVector(VT_I4, 0, uint32(len(values)))
	if array == 0 {
		return 0
	}
	for i := range values {
		if !w32.HResultSucceeded(SafeArrayPutElement(array, int32(i), unsafe.Pointer(&values[i]))) {
			array.Destroy()
			return 0
		}
	}
	return array
}

// NewSafeArrayFloat64 builds a VT_R8 vector, indexed from zero, holding the given values. It returns a zero SAFEARRAY
// if the array cannot be built.
func NewSafeArrayFloat64(values []float64) SAFEARRAY {
	array := SafeArrayCreateVector(VT_R8, 0, uint32(len(values)))
	if array == 0 {
		return 0
	}
	for i := range values {
		if !w32.HResultSucceeded(SafeArrayPutElement(array, int32(i), unsafe.Pointer(&values[i]))) {
			array.Destroy()
			return 0
		}
	}
	return array
}

// NewSafeArrayUnknown builds a VT_UNKNOWN vector, indexed from zero, holding the given interface pointers. It returns a
// zero SAFEARRAY if the array cannot be built.
//
// Storing an element adds a reference to it, so the references the caller holds are still the caller's to release: a
// provider building a selection array adds a reference per element, stores it, and then releases its own, leaving the
// array holding the only reference each element needs.
func NewSafeArrayUnknown(values []unsafe.Pointer) SAFEARRAY {
	array := SafeArrayCreateVector(VT_UNKNOWN, 0, uint32(len(values)))
	if array == 0 {
		return 0
	}
	for i, value := range values {
		if !w32.HResultSucceeded(SafeArrayPutElement(array, int32(i), value)) {
			array.Destroy()
			return 0
		}
	}
	return array
}

// safeArrayToInt32 reads a VT_I4 vector back into a slice, returning nil if the array is zero or cannot be read. It
// exists so that the provider's tests can check what was handed to UI Automation; nothing in the provider itself needs
// to read an array it built, so it is unexported like the package's other non-binding helpers.
func safeArrayToInt32(array SAFEARRAY) []int32 {
	if array == 0 {
		return nil
	}
	bound, hr := SafeArrayGetUBound(array, 1)
	if !w32.HResultSucceeded(hr) || bound < 0 {
		return nil
	}
	values := make([]int32, bound+1)
	for i := range values {
		if !w32.HResultSucceeded(SafeArrayGetElement(array, int32(i), unsafe.Pointer(&values[i]))) {
			return nil
		}
	}
	return values
}
