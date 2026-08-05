// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestCurrentGoroutineID verifies the property the UI thread check depends on: the ID identifies the goroutine, so it
// is stable within one and differs between two.
func TestCurrentGoroutineID(t *testing.T) {
	c := check.New(t)
	id := currentGoroutineID()
	c.True(id != 0, "the goroutine ID must be parsed from the stack header")
	c.Equal(id, currentGoroutineID(), "the goroutine ID must be stable within a goroutine")
	other := make(chan uint64, 1)
	go func() { other <- currentGoroutineID() }()
	otherID := <-other
	c.True(otherID != 0, "the goroutine ID must be parsed from the stack header")
	c.True(otherID != id, "different goroutines must report different IDs")
}

// TestOnUIThread verifies that the UI thread check only reports true for the goroutine recorded by Start(), and never
// before one has been recorded. This test mutates global state and therefore must not call t.Parallel.
func TestOnUIThread(t *testing.T) {
	c := check.New(t)
	saved := uiGoroutineID.Load()
	t.Cleanup(func() { uiGoroutineID.Store(saved) })

	uiGoroutineID.Store(0)
	c.False(onUIThread(), "no goroutine may be considered the UI thread before Start() records one")

	uiGoroutineID.Store(currentGoroutineID())
	c.True(onUIThread())
	other := make(chan bool, 1)
	go func() { other <- onUIThread() }()
	c.False(<-other, "another goroutine must never be mistaken for the UI thread")
}
