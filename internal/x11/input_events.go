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
	_ Event = &KeyPressEvent{}
	_ Event = &KeyReleaseEvent{}
	_ Event = &ButtonPressEvent{}
	_ Event = &ButtonReleaseEvent{}
	_ Event = &MotionNotifyEvent{}
)

// InputEvent represents a generic X11 input event.
type InputEvent struct {
	Root       WindowID
	Event      WindowID
	Child      WindowID
	Time       uint32
	Sequence   uint16
	State      uint16
	RootX      int16
	RootY      int16
	EventX     int16
	EventY     int16
	Code       byte
	Detail     byte
	SameScreen bool
}

func (e *InputEvent) read(r *Reader) {
	e.Code = r.Byte()
	e.Detail = r.Byte()
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Root = WindowID(r.Uint32())
	e.Event = WindowID(r.Uint32())
	e.Child = WindowID(r.Uint32())
	e.RootX = r.Int16()
	e.RootY = r.Int16()
	e.EventX = r.Int16()
	e.EventY = r.Int16()
	e.State = r.Uint16()
	e.SameScreen = r.Bool()
	r.Skip(1)
}

// ID returns the event code.
func (e *InputEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *InputEvent) TargetWindow() WindowID {
	return e.Event
}

// KeyPressEvent represents an X11 KeyPress event.
type KeyPressEvent struct {
	InputEvent
}

func newKeyPressEvent(r *Reader) Event {
	var e KeyPressEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *KeyPressEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// KeyReleaseEvent represents an X11 KeyRelease event.
type KeyReleaseEvent struct {
	InputEvent
}

func newKeyReleaseEvent(r *Reader) Event {
	var e KeyReleaseEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *KeyReleaseEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// ButtonPressEvent represents an X11 ButtonPress event.
type ButtonPressEvent struct {
	InputEvent
}

func newButtonPressEvent(r *Reader) Event {
	var e ButtonPressEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *ButtonPressEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// ButtonReleaseEvent represents an X11 ButtonRelease event.
type ButtonReleaseEvent struct {
	InputEvent
}

func newButtonReleaseEvent(r *Reader) Event {
	var e ButtonReleaseEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *ButtonReleaseEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// MotionNotifyEvent represents an X11 MotionNotify event.
type MotionNotifyEvent struct {
	InputEvent
}

func newMotionNotifyEvent(r *Reader) Event {
	var e MotionNotifyEvent
	e.read(r)
	return &e
}

// Process the event.
func (e *MotionNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
}
