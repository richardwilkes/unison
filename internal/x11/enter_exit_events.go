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
	_ Event = &EnterNotifyEvent{}
	_ Event = &LeaveNotifyEvent{}
)

// EnterNotifyEvent represents an X11 EnterNotify event.
type EnterNotifyEvent struct {
	Root            WindowID
	Event           WindowID
	Child           WindowID
	Time            uint32
	Sequence        uint16
	State           uint16
	RootX           int16
	RootY           int16
	EventX          int16
	EventY          int16
	Code            byte
	Detail          byte
	Mode            byte
	SameScreenFocus byte
}

func newEnterNotifyEvent(r *Reader) Event {
	var e EnterNotifyEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *EnterNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}

func (e *EnterNotifyEvent) read(r *Reader) {
	e.Code = r.Byte()
	e.Detail = r.Byte()
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Root = WindowID(r.Uint32())
	e.Event = WindowID(r.Uint32())
	e.Child = WindowID(r.Uint32())
	e.RootX = r.Int16()
	e.RootY = r.Int16()
	e.EventX = r.Int16()
	e.EventY = r.Int16()
	e.State = r.Uint16()
	e.Mode = r.Byte()
	e.SameScreenFocus = r.Byte()
}

// LeaveNotifyEvent represents an X11 LeaveNotify event.
type LeaveNotifyEvent struct {
	EnterNotifyEvent
}

func newLeaveNotifyEvent(r *Reader) Event {
	var e LeaveNotifyEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *LeaveNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}
