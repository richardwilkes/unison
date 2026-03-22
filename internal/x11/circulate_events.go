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

// CirculateEvent represents an X11 generic circulate event.
type CirculateEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
	Place    byte
}

func (e *CirculateEvent) read(r *Reader) {
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	r.Skip(4)
	e.Place = r.Byte()
	r.Skip(3)
}

// ID returns the event code.
func (e *CirculateEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *CirculateEvent) TargetWindow() WindowID {
	return e.Event
}

// CirculateNotifyEvent represents an X11 CirculateNotify event.
type CirculateNotifyEvent struct {
	CirculateEvent
}

func newCirculateNotifyEvent(r *Reader) Event {
	var e CirculateNotifyEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *CirculateNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// CirculateRequestEvent represents an X11 CirculateRequest event.
type CirculateRequestEvent struct {
	CirculateEvent
}

func newCirculateRequestEvent(r *Reader) Event {
	var e CirculateRequestEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *CirculateRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
}
