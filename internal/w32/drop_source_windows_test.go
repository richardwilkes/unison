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
	"sync/atomic"
	"testing"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestDropSourceReferenceCountLifetime verifies Release honors the COM reference count rather than unconditionally
// unpinning: a reference taken through QueryInterface must survive the creator's Release, with the count reaching zero
// — and only then triggering the unpin — when that holder releases too.
func TestDropSourceReferenceCountLifetime(t *testing.T) {
	c := check.New(t)
	src := NewDropSource()
	this := uintptr(unsafe.Pointer(src))
	var pin runtime.Pinner
	defer pin.Unpin()
	out := new(uintptr)
	guid := iidIDropSource
	pin.Pin(out)
	pin.Pin(&guid)

	// QueryInterface for IDropSource must return the same object and take a reference of its own.
	c.Equal(COM_S_OK, dropSrcQueryInterface(this, uintptr(unsafe.Pointer(&guid)), uintptr(unsafe.Pointer(out))))
	c.Equal(this, *out)
	c.Equal(int32(2), atomic.LoadInt32(&src.refCount))

	// The creator's Release must only drop its own reference, leaving the QueryInterface holder's intact.
	src.Release()
	c.Equal(int32(1), atomic.LoadInt32(&src.refCount))

	// The QueryInterface holder's release drops the final reference.
	c.Equal(uintptr(0), dropSrcRelease(this))

	// AddRef/Release through the COM vtbl entries must report standard IUnknown counts.
	src = NewDropSource()
	this = uintptr(unsafe.Pointer(src))
	c.Equal(uintptr(2), dropSrcAddRef(this))
	c.Equal(uintptr(1), dropSrcRelease(this))
	c.Equal(uintptr(0), dropSrcRelease(this))
}
