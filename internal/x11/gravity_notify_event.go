// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import "log/slog"

var _ Event = &GravityNotifyEvent{}

// GravityNotifyEvent represents an X11 GravityNotify event.
type GravityNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	X        int16
	Y        int16
	Code     byte
}

func newGravityNotifyEvent(r *Reader) Event {
	var e GravityNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	return &e
}

// ID returns the event code.
func (e *GravityNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *GravityNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *GravityNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("GravityNotifyEvent received", "window", e.Window, "x", e.X, "y", e.Y)
}
