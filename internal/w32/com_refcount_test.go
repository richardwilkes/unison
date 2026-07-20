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
	"sync"
	"sync/atomic"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestComRefCountLifecycle verifies comAddRef/comRelease implement IUnknown semantics and that exactly one release
// observes the final-reference signal, even under concurrency. That signal is what unpins the DataObject and frees
// its mediums, so it must fire exactly once, and only after every holder has released — the previous scheme unpinned
// as soon as DoDragDrop returned, leaving a drop target that retained the IDataObject pointing at freed memory.
func TestComRefCountLifecycle(t *testing.T) {
	c := check.New(t)
	count := int32(1)
	c.Equal(uintptr(2), comAddRef(&count))
	remaining, final := comRelease(&count)
	c.Equal(uintptr(1), remaining)
	c.False(final)
	remaining, final = comRelease(&count)
	c.Equal(uintptr(0), remaining)
	c.True(final)

	// Concurrent holders: each takes and drops a reference while the creator's reference is still held, so none of
	// them may ever observe final; only the creator's closing release does.
	count = 1
	var finals int32
	var wg sync.WaitGroup
	for range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			comAddRef(&count)
			if _, f := comRelease(&count); f {
				atomic.AddInt32(&finals, 1)
			}
		}()
	}
	wg.Wait()
	if _, f := comRelease(&count); f {
		atomic.AddInt32(&finals, 1)
	}
	c.Equal(int32(1), atomic.LoadInt32(&finals))
}
