// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

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

// KeyPressEvent represents an X11 KeyPress event.
type KeyPressEvent struct {
	InputEvent
}

func newKeyPressEvent(r *Reader) any {
	var e KeyPressEvent
	e.read(r)
	return &e
}

// KeyReleaseEvent represents an X11 KeyRelease event.
type KeyReleaseEvent struct {
	InputEvent
}

func newKeyReleaseEvent(r *Reader) any {
	var e KeyReleaseEvent
	e.read(r)
	return &e
}

// ButtonPressEvent represents an X11 ButtonPress event.
type ButtonPressEvent struct {
	InputEvent
}

func newButtonPressEvent(r *Reader) any {
	var e ButtonPressEvent
	e.read(r)
	return &e
}

// ButtonReleaseEvent represents an X11 ButtonRelease event.
type ButtonReleaseEvent struct {
	InputEvent
}

func newButtonReleaseEvent(r *Reader) any {
	var e ButtonReleaseEvent
	e.read(r)
	return &e
}

// MotionNotifyEvent represents an X11 MotionNotify event.
type MotionNotifyEvent struct {
	InputEvent
}

func newMotionNotifyEvent(r *Reader) any {
	var e MotionNotifyEvent
	e.read(r)
	return &e
}

// EnterNotifyEvent represents an X11 EnterNotify event.
type EnterNotifyEvent struct {
	Root            WindowID
	Event           WindowID
	Child           WindowID
	Time            uint32
	Sequence        uint16
	State           uint16
	RootX           int16
	RootY           int16
	EventX          int16
	EventY          int16
	Code            byte
	Detail          byte
	Mode            byte
	SameScreenFocus byte
}

func (e *EnterNotifyEvent) read(r *Reader) {
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
	e.Mode = r.Byte()
	e.SameScreenFocus = r.Byte()
}

func newEnterNotifyEvent(r *Reader) any {
	var e EnterNotifyEvent
	e.read(r)
	return &e
}

// LeaveNotifyEvent represents an X11 LeaveNotify event.
type LeaveNotifyEvent struct {
	EnterNotifyEvent
}

func newLeaveNotifyEvent(r *Reader) any {
	var e LeaveNotifyEvent
	e.read(r)
	return &e
}

// FocusEvent represents an X11 generic focus event.
type FocusEvent struct {
	Event    WindowID
	Sequence uint16
	Code     byte
	Detail   byte
	Mode     byte
}

func (e *FocusEvent) read(r *Reader) {
	e.Code = r.Byte()
	e.Detail = r.Byte()
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Mode = r.Byte()
}

// FocusInEvent represents an X11 FocusIn event.
type FocusInEvent struct {
	FocusEvent
}

func newFocusInEvent(r *Reader) any {
	var e FocusInEvent
	e.read(r)
	return &e
}

// FocusOutEvent represents an X11 FocusOut event.
type FocusOutEvent struct {
	FocusEvent
}

func newFocusOutEvent(r *Reader) any {
	var e FocusOutEvent
	e.read(r)
	return &e
}

func newKeymapNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newExposeEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newGraphicsExposureEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newNoExposureEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newVisibilityNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newCreateNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newDestroyNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newUnmapNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newMapNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newMapRequestEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newReparentNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newConfigureNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newConfigureRequestEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newGravityNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newResizeRequestEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newCirculateNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newCirculateRequestEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newPropertyNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newSelectionClearEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newSelectionRequestEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newSelectionNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newColormapNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newClientMessageEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newMappingNotifyEvent(r *Reader) any {
	// TODO: Implement
	return nil
}

func newGenericEventEvent(r *Reader) any {
	// TODO: Implement
	return nil
}
