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
	"sync"
	"testing"

	"github.com/richardwilkes/unison/internal/x11"
)

// TestPostEmptyEventRacesTerminate is the regression test for the shutdown race between nativeTerminate and
// nativePostEmptyEvent. Before the fix, nativePostEmptyEvent nil-checked and dereferenced x11Conn — UI-thread-only
// state that nativeTerminate tears down — from arbitrary goroutines with no synchronization, a data race flagged
// by -race during quit. The wake path now goes through the atomic x11PostConn handle, which nativeTerminate withdraws
// before closing the connection. This test hammers nativePostEmptyEvent from several goroutines while the main
// goroutine repeatedly publishes and withdraws a connection the way startup and terminate do; under -race it fails if
// any access is unsynchronized. A zero-value x11.Conn works here because PostEmptyEvent on an unconnected Conn is a
// documented no-op (see x11's TestPostEmptyEventSafeOnUnconnectedConn). This test mutates global state and therefore
// must not call t.Parallel.
func TestPostEmptyEventRacesTerminate(_ *testing.T) {
	saved := x11PostConn.Load()
	defer x11PostConn.Store(saved)
	conn := &x11.Conn{}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					nativePostEmptyEvent()
				}
			}
		}()
	}
	for range 1000 {
		x11PostConn.Store(conn) // What nativeBeginStartup does after connecting.
		x11PostConn.Store(nil)  // What nativeTerminate does before closing the connection.
	}
	close(stop)
	wg.Wait()
}
