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

var _ Event = &CreateNotifyEvent{}

// CreateNotifyEvent represents an X11 CreateNotify event.
type CreateNotifyEvent struct {
	Parent           WindowID
	Window           WindowID
	Sequence         uint16
	X                int16
	Y                int16
	Width            uint16
	Height           uint16
	BorderWidth      uint16
	Code             byte
	OverrideRedirect bool
}

func newCreateNotifyEvent(r *Reader) Event {
	var e CreateNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Parent = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.OverrideRedirect = r.Bool()
	r.Skip(1)
	return &e
}

// ID returns the event code.
func (e *CreateNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *CreateNotifyEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *CreateNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("CreateNotifyEvent received", "sequence", e.Sequence, "parent", e.Parent, "window", e.Window, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "borderWidth", e.BorderWidth, "overrideRedirect", e.OverrideRedirect)
}
