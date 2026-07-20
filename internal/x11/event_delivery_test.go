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
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// fakeEvent is a minimal Event implementation for exercising the delivery machinery without a live X server.
type fakeEvent struct {
	n int
}

func (e *fakeEvent) ID() byte {
	return 255
}

// TestEventDeliveryDoesNotBlockWithoutConsumer verifies that deliverEvent never blocks, no matter how far event
// delivery gets ahead of consumption. readResponses is the sole reader of the connection, so if delivery could block
// (as it did when events were sent through a bounded channel), a reply queued behind a large event backlog would never
// be read and the connection would deadlock.
func TestEventDeliveryDoesNotBlockWithoutConsumer(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	const total = 10000
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range total {
			conn.deliverEvent(&fakeEvent{n: i})
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("deliverEvent blocked with no consumer draining events")
	}
	for i := range total {
		fe, ok := conn.PollEvents(nil).(*fakeEvent)
		if !ok {
			t.Fatalf("event %d: not a *fakeEvent", i)
		}
		c.Equal(i, fe.n)
	}
	c.True(conn.PollEvents(nil) == nil, "queue must be empty after draining")
}

// TestPostEmptyEventWakesBlockedWaiter verifies that a wake-up posted while WaitEvents is blocked causes it to return
// nil, which is what the event loop relies on to run tasks posted from other goroutines.
func TestPostEmptyEventWakesBlockedWaiter(t *testing.T) {
	conn := &Conn{events: make(chan Event, 1)}
	got := make(chan Event, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		got <- conn.WaitEvents(nil)
	}()
	<-started
	conn.PostEmptyEvent()
	select {
	case e := <-got:
		if e != nil {
			t.Fatalf("expected nil from a wake-up, got %#v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("PostEmptyEvent failed to wake WaitEvents")
	}
}

// TestWaitEventsFilterLeavesNonMatchingQueued verifies that filtered waits return the first matching event while
// leaving non-matching events queued, in order, for later retrieval.
func TestWaitEventsFilterLeavesNonMatchingQueued(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.deliverEvent(&fakeEvent{n: 1})
	conn.deliverEvent(&fakeEvent{n: 2})
	fe, ok := conn.WaitEvents(func(e Event) bool {
		f, ok2 := e.(*fakeEvent)
		return ok2 && f.n == 2
	}).(*fakeEvent)
	c.True(ok)
	c.Equal(2, fe.n)
	fe, ok = conn.PollEvents(nil).(*fakeEvent)
	c.True(ok)
	c.Equal(1, fe.n)
	c.True(conn.PollEvents(nil) == nil, "queue must be empty after draining")
}

// TestShutdownDrainsQueueThenReturnsNil verifies the shutdown contract: once the connection is dead, already-queued
// events are still retrievable, after which every wait returns nil immediately and Dead reports true so callers can
// distinguish a lost connection from an ordinary wake-up.
func TestShutdownDrainsQueueThenReturnsNil(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	conn.deliverEvent(&fakeEvent{n: 1})
	conn.deliverEvent(&fakeEvent{n: 2})
	c.False(conn.Dead())
	conn.closeEvents() // What readResponses does when the connection shuts down.
	c.True(conn.Dead())
	for i := 1; i <= 2; i++ {
		fe, ok := conn.WaitEvents(nil).(*fakeEvent)
		c.True(ok)
		c.Equal(i, fe.n)
	}
	c.True(conn.WaitEvents(nil) == nil, "WaitEvents must return nil once dead and drained")
	c.True(conn.WaitEvents(nil) == nil, "WaitEvents must keep returning nil, not block")
	c.True(conn.WaitEventsUntil(nil, time.Second) == nil, "WaitEventsUntil must return nil once dead and drained")
	c.True(conn.PollEvents(nil) == nil, "PollEvents must return nil once dead and drained")
}

// TestWaitEventsUntilTimesOut verifies that a filtered timed wait with nothing pending returns nil once the timeout
// elapses.
func TestWaitEventsUntilTimesOut(t *testing.T) {
	c := check.New(t)
	conn := &Conn{events: make(chan Event, 1)}
	start := time.Now()
	c.True(conn.WaitEventsUntil(nil, 50*time.Millisecond) == nil)
	c.True(time.Since(start) >= 50*time.Millisecond, "must not return before the timeout")
}

// TestReplyNotStarvedByEventBacklog is the regression test for the reply-starvation deadlock: with the main thread
// parked in sendNewRequest waiting for a reply and nothing draining events, readResponses must still be able to work
// through an event backlog larger than any fixed buffer to reach the reply. Before the fix, events were delivered
// through a bounded channel, so a backlog exceeding its capacity blocked readResponses forever and the connection
// deadlocked.
func TestReplyNotStarvedByEventBacklog(t *testing.T) {
	c := check.New(t)
	client, server := net.Pipe()
	conn := &Conn{
		conn:         client,
		events:       make(chan Event, 1),
		requests:     make(chan *request, 128),
		closed:       make(chan struct{}),
		readClosed:   make(chan struct{}),
		eventNewMap:  newEventMap(),
		errorCodeMap: newErrorMap(),
		requestMap:   make(map[uint16]*request),
	}
	go conn.sendRequests()
	go conn.readResponses()
	const backlog = 10000
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- func() error {
			buf := make([]byte, 4) // The GetInputFocus request is a single 4-byte packet.
			if _, err := io.ReadFull(server, buf); err != nil {
				return err
			}
			event := make([]byte, 32)
			event[0] = eventCodeKeyPress
			for range backlog {
				if _, err := server.Write(event); err != nil {
					return err
				}
			}
			reply := make([]byte, 32)
			reply[0] = 1                                 // Reply
			binary.LittleEndian.PutUint16(reply[2:4], 1) // Sequence number of the first request sent
			_, err := server.Write(reply)
			return err
		}()
	}()
	requestDone := make(chan error, 1)
	go func() {
		_, _, err := conn.GetInputFocus()
		requestDone <- err
	}()
	select {
	case err := <-requestDone:
		c.NoError(err)
	case <-time.After(10 * time.Second):
		t.Fatal("deadlock: reply starved behind event backlog")
	}
	c.NoError(<-serverErr)
	count := 0
	for {
		e := conn.PollEvents(nil)
		if e == nil {
			break
		}
		if _, ok := e.(*KeyPressEvent); !ok {
			t.Fatalf("event %d: not a *KeyPressEvent", count)
		}
		count++
	}
	c.Equal(backlog, count)
	close(conn.requests) // Shut the connection down and wait for both goroutines to finish.
	select {
	case <-conn.readClosed:
	case <-time.After(10 * time.Second):
		t.Fatal("connection failed to shut down")
	}
	c.True(conn.Dead())
}
