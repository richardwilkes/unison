// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &ColormapNotifyEvent{}

// ColormapNotifyEvent represents an X11 ColormapNotify event.
type ColormapNotifyEvent struct {
	Sequence uint16
	Window   WindowID
	Colormap ColorMapID
	New      bool
	Code     byte
	State    byte
}

func newColormapNotifyEvent(r *Reader) Event {
	var e ColormapNotifyEvent
	e.Code = r.Byte()
	r.Skip(3)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Colormap = ColorMapID(r.Uint32())
	e.New = r.Bool()
	e.State = r.Byte()
	r.Skip(2)
	return &e
}

// ID returns the event code.
func (e *ColormapNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ColormapNotifyEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ColormapNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}
