// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import "sync/atomic"

// comAddRef increments a COM reference count and returns the new count, matching IUnknown::AddRef semantics.
func comAddRef(count *int32) uintptr {
	return uintptr(atomic.AddInt32(count, 1))
}

// comRelease decrements a COM reference count, matching IUnknown::Release semantics. It returns the remaining count
// and whether this call dropped the final reference, in which case the caller must perform its cleanup — exactly one
// concurrent releaser observes final == true, so cleanup cannot run twice even with a misbehaving client.
func comRelease(count *int32) (remaining uintptr, final bool) {
	n := atomic.AddInt32(count, -1)
	return uintptr(n), n == 0
}
