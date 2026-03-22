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
	_ Event = &SelectionClearEvent{}
	_ Event = &SelectionRequestEvent{}
	_ Event = &SelectionNotifyEvent{}
)

// SelectionClearEvent represents an X11 SelectionClear event.
type SelectionClearEvent struct {
	Time      uint32
	Owner     WindowID
	Selection Atom
	Sequence  uint16
	Code      byte
}

func newSelectionClearEvent(r *Reader) Event {
	var e SelectionClearEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Owner = WindowID(r.Uint32())
	e.Selection = Atom(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *SelectionClearEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *SelectionClearEvent) TargetWindow() WindowID {
	return e.Owner
}

// Process the event.
func (e *SelectionClearEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// SelectionRequestEvent represents an X11 SelectionRequest event.
type SelectionRequestEvent struct {
	Time      uint32
	Owner     WindowID
	Requestor WindowID
	Selection Atom
	Target    Atom
	Property  Atom
	Sequence  uint16
	Code      byte
}

func newSelectionRequestEvent(r *Reader) Event {
	var e SelectionRequestEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Owner = WindowID(r.Uint32())
	e.Requestor = WindowID(r.Uint32())
	e.Selection = Atom(r.Uint32())
	e.Target = Atom(r.Uint32())
	e.Property = Atom(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *SelectionRequestEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *SelectionRequestEvent) TargetWindow() WindowID {
	return e.Owner
}

// Process the event.
func (e *SelectionRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
}

// SelectionNotifyEvent represents an X11 SelectionNotify event.
type SelectionNotifyEvent struct {
	Time      uint32
	Requestor WindowID
	Selection Atom
	Target    Atom
	Property  Atom
	Sequence  uint16
	Code      byte
}

func newSelectionNotifyEvent(r *Reader) Event {
	var e SelectionNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Requestor = WindowID(r.Uint32())
	e.Selection = Atom(r.Uint32())
	e.Target = Atom(r.Uint32())
	e.Property = Atom(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *SelectionNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *SelectionNotifyEvent) TargetWindow() WindowID {
	return e.Requestor
}

// Process the event.
func (e *SelectionNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement; this might be a noop, as the clipboard logic is currently implemented in the Conn's
	// getClipboardString method, which waits for a SelectionNotifyEvent and processes it there.
}
