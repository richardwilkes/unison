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
	_ Event = &ConfigureRequestEvent{}
	_ Event = &ConfigureNotifyEvent{}
)

// ConfigureRequestEvent represents an X11 ConfigureRequest event.
type ConfigureRequestEvent struct {
	Parent      WindowID
	Window      WindowID
	Sibling     WindowID
	Sequence    uint16
	X           int16
	Y           int16
	Width       uint16
	Height      uint16
	BorderWidth uint16
	ValueMask   uint16
	Code        byte
	StackMode   byte
}

func newConfigureRequestEvent(r *Reader) Event {
	var e ConfigureRequestEvent
	e.Code = r.Byte()
	e.StackMode = r.Byte()
	e.Sequence = r.Uint16()
	e.Parent = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.Sibling = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.ValueMask = r.Uint16()
	return &e
}

// Process the event.
func (e *ConfigureRequestEvent) Process(conn *Conn) {
	// TODO: Implement
}

// ConfigureNotifyEvent represents an X11 ConfigureNotify event.
type ConfigureNotifyEvent struct {
	Event            WindowID
	Window           WindowID
	AboveSibling     WindowID
	Sequence         uint16
	X                int16
	Y                int16
	Width            uint16
	Height           uint16
	BorderWidth      uint16
	Code             byte
	OverrideRedirect bool
}

func newConfigureNotifyEvent(r *Reader) Event {
	var e ConfigureNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.AboveSibling = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.OverrideRedirect = r.Bool()
	r.Skip(1)
	return &e
}

// Process the event.
func (e *ConfigureNotifyEvent) Process(conn *Conn) {
	// TODO: Implement
}
