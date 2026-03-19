// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &ReparentNotifyEvent{}

// ReparentNotifyEvent represents an X11 ReparentNotify event.
type ReparentNotifyEvent struct {
	Event            WindowID
	Window           WindowID
	Parent           WindowID
	Sequence         uint16
	X                int16
	Y                int16
	Code             byte
	OverrideRedirect bool
}

func newReparentNotifyEvent(r *Reader) Event {
	var e ReparentNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.Parent = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	e.OverrideRedirect = r.Bool()
	r.Skip(3)
	return &e
}

// Process the event.
func (e *ReparentNotifyEvent) Process(conn *Conn) {
	// TODO: Implement
}
