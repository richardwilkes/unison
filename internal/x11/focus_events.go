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
	_ Event = &FocusInEvent{}
	_ Event = &FocusOutEvent{}
)

// FocusEvent represents an X11 generic focus event.
type FocusEvent struct {
	Event    WindowID
	Sequence uint16
	Code     byte
	Detail   byte
	Mode     byte
}

func (e *FocusEvent) read(r *Reader) {
	e.Code = r.Byte()
	e.Detail = r.Byte()
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Mode = r.Byte()
	r.Skip(23)
}

// FocusInEvent represents an X11 FocusIn event.
type FocusInEvent struct {
	FocusEvent
}

func newFocusInEvent(r *Reader) Event {
	var e FocusInEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *FocusInEvent) Process(conn *Conn) {
	// TODO: Implement
}

// FocusOutEvent represents an X11 FocusOut event.
type FocusOutEvent struct {
	FocusEvent
}

func newFocusOutEvent(r *Reader) Event {
	var e FocusOutEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *FocusOutEvent) Process(conn *Conn) {
	// TODO: Implement
}
