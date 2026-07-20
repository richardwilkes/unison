// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestPostEmptyEventDeliversWakeUp verifies the basic contract: posting on an open connection places a nil wake-up
// event on the events channel.
func TestPostEmptyEventDeliversWakeUp(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 4)}
	conn.PostEmptyEvent()
	c.Equal(1, len(conn.events))
	e := <-conn.events
	c.True(e == nil, "the wake-up event must be nil")
}

// TestPostEmptyEventDoesNotBlockWhenFull verifies that posting to a full events channel returns immediately instead
// of blocking. Blocking there while holding postEventLock would deadlock a concurrent closeEvents during shutdown,
// and the wake-up is redundant anyway: a full channel already guarantees any waiter will wake.
func TestPostEmptyEventDoesNotBlockWhenFull(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.PostEmptyEvent()
	conn.PostEmptyEvent() // Would block forever before the fix's non-blocking send if delivery were attempted.
	c.Equal(1, len(conn.events))
}

// TestPostEmptyEventSafeOnUnconnectedConn verifies that posting on a zero-value Conn (nil events channel) is a no-op
// rather than a permanent block or panic. app_linux.go's shutdown race test relies on this to exercise the wake path
// without a live X server.
func TestPostEmptyEventSafeOnUnconnectedConn(_ *testing.T) {
	var conn Conn
	conn.PostEmptyEvent() // Must return without blocking or panicking.
}

// TestPostEmptyEventRacesClose is the regression test for the shutdown race: PostEmptyEvent may be called from any
// goroutine while readResponses closes the events channel during Conn.Close. Before the fix, PostEmptyEvent did a
// bare send on the channel, so a post racing the close panicked with "send on closed channel". Run with -race, this
// also proves the eventsClosed flag itself is properly synchronized. Posts after the close must be silent no-ops.
func TestPostEmptyEventRacesClose(_ *testing.T) {
	for range 100 {
		conn := &Conn{events: make(chan Event, 2)}
		start := make(chan struct{})
		var wg sync.WaitGroup
		for range 4 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				for range 8 {
					conn.PostEmptyEvent()
				}
			}()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			conn.closeEvents() // What readResponses does when the connection shuts down.
		}()
		close(start)
		wg.Wait()
		conn.PostEmptyEvent() // After the close, posting must be a no-op, not a panic.
	}
}
