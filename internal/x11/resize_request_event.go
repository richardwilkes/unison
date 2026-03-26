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

var _ Event = &ResizeRequestEvent{}

// ResizeRequestEvent represents an X11 ResizeRequest event.
type ResizeRequestEvent struct {
	Window   WindowID
	Sequence uint16
	Width    uint16
	Height   uint16
	Code     byte
}

func newResizeRequestEvent(r *Reader) Event {
	var e ResizeRequestEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	return &e
}

// ID returns the event code.
func (e *ResizeRequestEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ResizeRequestEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ResizeRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ResizeRequestEvent received", "sequence", e.Sequence, "window", e.Window, "width", e.Width, "height", e.Height)
}
