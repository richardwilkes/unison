// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &ClientMessageEvent{}

// ClientMessageEvent represents an X11 ClientMessage event.
type ClientMessageEvent struct {
	Data8    [20]byte
	Data16   [10]uint16
	Data32   [5]uint32
	Window   WindowID
	Type     Atom
	Sequence uint16
	Code     byte
	Format   byte
}

func newClientMessageEvent(r *Reader) Event {
	var e ClientMessageEvent
	e.Code = r.Byte()
	e.Format = r.Byte()
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Type = Atom(r.Uint32())
	r.IntoBytes(e.Data8[:])
	r.SeekRelative(-len(e.Data8))
	for i := range e.Data16 {
		e.Data16[i] = r.Uint16()
	}
	r.SeekRelative(-len(e.Data8))
	for i := range e.Data32 {
		e.Data32[i] = r.Uint32()
	}
	return &e
}

// Process the event.
func (e *ClientMessageEvent) Process(_conn *Conn) {
	// TODO: Implement
}
