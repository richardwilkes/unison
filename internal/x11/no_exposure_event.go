// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

var _ Event = &NoExposureEvent{}

// NoExposureEvent represents an X11 NoExposure event.
type NoExposureEvent struct {
	Sequence    uint16
	Drawable    DrawableID
	MinorOpcode uint16
	MajorOpcode byte
	Code        byte
}

func newNoExposureEvent(r *Reader) Event {
	var e NoExposureEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Drawable = DrawableID(r.Uint32())
	e.MinorOpcode = r.Uint16()
	e.MajorOpcode = r.Byte()
	r.Skip(1)
	return &e
}

// ID returns the event code.
func (e *NoExposureEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *NoExposureEvent) TargetWindow() WindowID {
	return WindowID(e.Drawable)
}

// Process the event.
func (e *NoExposureEvent) Process(_conn *Conn) {
	// TODO: Implement
}
