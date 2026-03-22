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
	_ Event = &MapRequestEvent{}
	_ Event = &MapNotifyEvent{}
	_ Event = &UnmapNotifyEvent{}
)

// MapRequestEvent represents an X11 MapRequest event.
type MapRequestEvent struct {
	Parent   WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
}

func newMapRequestEvent(r *Reader) Event {
	var e MapRequestEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Parent = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *MapRequestEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *MapRequestEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *MapRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// MapNotifyEvent represents an X11 MapNotify event.
type MapNotifyEvent struct {
	Event            WindowID
	Window           WindowID
	Sequence         uint16
	Code             byte
	OverrideRedirect bool
}

func newMapNotifyEvent(r *Reader) Event {
	var e MapNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.OverrideRedirect = r.Bool()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *MapNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *MapNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *MapNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// UnmapNotifyEvent represents an X11 UnmapNotify event.
type UnmapNotifyEvent struct {
	Event         WindowID
	Window        WindowID
	Sequence      uint16
	Code          byte
	FromConfigure bool
}

func newUnmapNotifyEvent(r *Reader) Event {
	var e UnmapNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.FromConfigure = r.Bool()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *UnmapNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *UnmapNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *UnmapNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}
