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

var _ Event = &ExposeEvent{}

// ExposeEvent represents an X11 Expose event.
type ExposeEvent struct {
	Window   WindowID
	Sequence uint16
	X        uint16
	Y        uint16
	Width    uint16
	Height   uint16
	Count    uint16
	Code     byte
}

func newExposeEvent(r *Reader) Event {
	var e ExposeEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.X = r.Uint16()
	e.Y = r.Uint16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.Count = r.Uint16()
	r.Skip(2)
	return &e
}

// ID returns the event code.
func (e *ExposeEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ExposeEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ExposeEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ExposeEvent received", "window", e.Window, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "count", e.Count)
}
