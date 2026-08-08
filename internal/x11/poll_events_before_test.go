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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

func isMotion(e Event) bool {
	_, ok := e.(*MotionNotifyEvent)
	return ok
}

func isButtonRelease(e Event) bool {
	_, ok := e.(*ButtonReleaseEvent)
	return ok
}

// queueEvents fills a connection's event queue directly, in order.
func queueEvents(events ...Event) *Conn {
	conn := &Conn{events: make(chan Event, 1)}
	for _, e := range events {
		conn.deliverEvent(e)
	}
	return conn
}

// remaining returns the events still queued, in order, draining the queue in the process.
func remaining(conn *Conn) []Event {
	var events []Event
	for {
		e := conn.PollEvents(nil)
		if e == nil {
			return events
		}
		events = append(events, e)
	}
}

// TestPollEventsBeforeStopsAtBarrier verifies that an event queued behind a barrier event is left alone. Motion
// coalescing during a drag relies on this: pulling the motion that arrived after a ButtonRelease out from behind it
// would report the pointer's post-release position as the drop position.
func TestPollEventsBeforeStopsAtBarrier(t *testing.T) {
	c := check.New(t)
	release := &ButtonReleaseEvent{}
	motion := &MotionNotifyEvent{}
	conn := queueEvents(release, motion)

	c.Nil(conn.PollEventsBefore(isMotion, isButtonRelease), "motion behind a barrier must not be returned")

	// Neither the barrier nor the event behind it may be consumed.
	c.Equal([]Event{release, motion}, remaining(conn))
}

// TestPollEventsBeforeReturnsMatchAheadOfBarrier verifies that matching events queued ahead of the barrier are still
// returned, one call at a time, until the barrier is reached.
func TestPollEventsBeforeReturnsMatchAheadOfBarrier(t *testing.T) {
	c := check.New(t)
	first := &MotionNotifyEvent{}
	second := &MotionNotifyEvent{}
	release := &ButtonReleaseEvent{}
	behind := &MotionNotifyEvent{}
	conn := queueEvents(first, second, release, behind)

	c.Equal(Event(first), conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal(Event(second), conn.PollEventsBefore(isMotion, isButtonRelease))

	// The barrier is now at the head, so the motion behind it stays put.
	c.Nil(conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal([]Event{release, behind}, remaining(conn))
}

// TestPollEventsBeforeSkipsUnrelatedEvents verifies that events matching neither the filter nor the barrier are
// skipped over and left queued, as PollEvents does.
func TestPollEventsBeforeSkipsUnrelatedEvents(t *testing.T) {
	c := check.New(t)
	expose := &ExposeEvent{}
	press := &ButtonPressEvent{}
	motion := &MotionNotifyEvent{}
	release := &ButtonReleaseEvent{}
	conn := queueEvents(expose, press, motion, release)

	c.Equal(Event(motion), conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal([]Event{expose, press, release}, remaining(conn))
}

// TestPollEventsBeforeWithoutBarrier verifies that a nil barrier makes the call behave like PollEvents, that a nil
// filter takes the event at the head, and that a queue with nothing to return yields nil without consuming anything.
func TestPollEventsBeforeWithoutBarrier(t *testing.T) {
	c := check.New(t)

	c.Nil((&Conn{events: make(chan Event, 1)}).PollEventsBefore(isMotion, isButtonRelease), "an empty queue yields nil")

	release := &ButtonReleaseEvent{}
	motion := &MotionNotifyEvent{}
	conn := queueEvents(release, motion)
	c.Equal(Event(motion), conn.PollEventsBefore(isMotion, nil), "with no barrier, motion behind a release is returned")
	c.Equal([]Event{release}, remaining(conn))

	// A nil filter returns the event at the head, so long as it isn't the barrier itself.
	expose := &ExposeEvent{}
	release = &ButtonReleaseEvent{}
	conn = queueEvents(expose, release)
	c.Equal(Event(expose), conn.PollEventsBefore(nil, isButtonRelease))
	c.Nil(conn.PollEventsBefore(nil, isButtonRelease), "the barrier itself is never returned")
	c.Equal([]Event{release}, remaining(conn))

	// A queue holding nothing that matches yields nil without consuming anything.
	press := &ButtonPressEvent{}
	conn = queueEvents(expose, press)
	c.Nil(conn.PollEventsBefore(isMotion, isButtonRelease))
	c.Equal([]Event{expose, press}, remaining(conn))
}
