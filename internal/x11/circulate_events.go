// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var (
	_ Event = &CirculateNotifyEvent{}
	_ Event = &CirculateRequestEvent{}
)

// CirculateNotifyEvent represents an X11 CirculateNotify event.
type CirculateNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
	Place    byte
}

func (e *CirculateNotifyEvent) read(r *Reader) {
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	r.Skip(4)
	e.Place = r.Byte()
	r.Skip(3)
}

func newCirculateNotifyEvent(r *Reader) Event {
	var e CirculateNotifyEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *CirculateNotifyEvent) Process(conn *Conn) {
	// TODO: Implement
}

// CirculateRequestEvent represents an X11 CirculateRequest event.
type CirculateRequestEvent struct {
	CirculateNotifyEvent
}

func newCirculateRequestEvent(r *Reader) Event {
	var e CirculateRequestEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *CirculateRequestEvent) Process(conn *Conn) {
	// TODO: Implement
}
