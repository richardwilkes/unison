// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &GraphicsExposureEvent{}

// GraphicsExposureEvent represents an X11 GraphicsExposure event.
type GraphicsExposureEvent struct {
	Drawable    DrawableID
	Sequence    uint16
	X           uint16
	Y           uint16
	Width       uint16
	Height      uint16
	MinorOpcode uint16
	Count       uint16
	Code        byte
	MajorOpcode byte
}

func newGraphicsExposureEvent(r *Reader) Event {
	var e GraphicsExposureEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Drawable = DrawableID(r.Uint32())
	e.X = r.Uint16()
	e.Y = r.Uint16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.MinorOpcode = r.Uint16()
	e.Count = r.Uint16()
	e.MajorOpcode = r.Byte()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *GraphicsExposureEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *GraphicsExposureEvent) TargetWindow() WindowID {
	return WindowID(e.Drawable)
}

// Process the event.
func (e *GraphicsExposureEvent) Process(_conn *Conn) {
	// TODO: Implement
}
