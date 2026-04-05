// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"log/slog"
	"math"

	"github.com/richardwilkes/toolbox/v2/errs"
)

var (
	_ Event         = &SelectionClearEvent{}
	_ Event         = &SelectionRequestEvent{}
	_ WritableEvent = &SelectionNotifyEvent{}
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
	slog.Info("SelectionClearEvent received", "sequence", e.Sequence, "owner", e.Owner, "selection", e.Selection, "time", e.Time)
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
func (e *SelectionRequestEvent) Process(c *Conn) {
	slog.Info("SelectionRequestEvent received", "sequence", e.Sequence, "owner", e.Owner, "requestor", e.Requestor, "selection", e.Selection, "target", e.Target, "property", e.Property, "time", e.Time)
	if err := c.sendEvent(e.Requestor, false, 0, &SelectionNotifyEvent{
		Time:      e.Time,
		Requestor: e.Requestor,
		Selection: e.Selection,
		Target:    e.Target,
		Property:  e.writeTargetToProperty(c),
	}); err != nil {
		errs.Log(err)
	}
}

func (e *SelectionRequestEvent) writeTargetToProperty(c *Conn) Atom {
	if e.Property == AtomNone {
		return AtomNone
	}
	switch e.Target {
	case c.atoms[atomClipboardTargets]:
		w := NewWriter(16)
		w.Atom(c.atoms[atomClipboardTargets])
		w.Atom(c.atoms[atomClipboardMultiple])
		w.Atom(c.atoms[atomUTF8String])
		w.Atom(AtomString)
		c.ChangeProperty(e.Requestor, e.Property, AtomAtom, 32, PropModeReplace, w.Retrieve())
		return e.Property
	case c.atoms[atomClipboardMultiple]:
		format, kind, value, err := c.GetProperty(e.Requestor, e.Property, c.atoms[atomPair], 0, math.MaxUint32, false)
		count := len(value) / (int(format) / 8)
		if err != nil {
			errs.Log(err)
			return e.Property
		}
		if format != 32 || kind != c.atoms[atomPair] || count%2 != 0 {
			slog.Error("unexpected result from GetProperty for MULTIPLE property", "format", format, "kind", kind, "count", count)
			return e.Property
		}
		content := []byte(c.clipboard)
		w := NewWriter(8 * count)
		r := NewReader(value)
		for i := 0; i < count; i += 2 {
			propType := r.Atom()
			prop := r.Atom()
			if propType == c.atoms[atomUTF8String] || propType == AtomString {
				w.Atom(propType)
				c.ChangeProperty(e.Requestor, prop, propType, 8, PropModeReplace, content)
			} else {
				w.Atom(AtomNone)
			}
			w.Atom(prop)
		}
		c.ChangeProperty(e.Requestor, e.Property, c.atoms[atomPair], 32, PropModeReplace, w.Retrieve())
		return e.Property
	case c.atoms[atomClipboardSaveTargets]:
		c.ChangeProperty(e.Requestor, e.Property, c.atoms[atomNull], 32, PropModeReplace, nil)
		return e.Property
	case c.atoms[atomUTF8String], AtomString:
		c.ChangeProperty(e.Requestor, e.Property, e.Target, 8, PropModeReplace, []byte(c.clipboard))
		return e.Property
	}
	return AtomNone
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

// Write the event to the given Writer. The sequence number and event code inside the event struct are ignored.
func (e *SelectionNotifyEvent) Write(sequence uint16, w *Writer) {
	w.Byte(eventCodeSelectionNotify)
	w.Zero(1)
	w.Uint16(sequence)
	w.Uint32(e.Time)
	w.WindowID(e.Requestor)
	w.Atom(e.Selection)
	w.Atom(e.Target)
	w.Atom(e.Property)
	w.Zero(8)
}

// Process the event.
func (e *SelectionNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement; this might be a noop, as the clipboard logic is currently implemented in the Conn's
	// GetClipboardText method, which waits for a SelectionNotifyEvent and processes it there.
	slog.Info("SelectionNotifyEvent received", "sequence", e.Sequence, "requestor", e.Requestor, "selection", e.Selection, "target", e.Target, "property", e.Property, "time", e.Time)
}
