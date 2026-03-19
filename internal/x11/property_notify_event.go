// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &PropertyNotifyEvent{}

// PropertyNotifyEvent represents an X11 PropertyNotify event.
type PropertyNotifyEvent struct {
	Window   WindowID
	Atom     Atom
	Time     uint32
	Sequence uint16
	Code     byte
	State    byte
}

func newPropertyNotifyEvent(r *Reader) Event {
	var e PropertyNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Atom = Atom(r.Uint32())
	e.Time = r.Uint32()
	e.State = r.Byte()
	r.Skip(3)
	return &e
}

// Process the event.
func (e *PropertyNotifyEvent) Process(conn *Conn) {
	// TODO: Implement
}
