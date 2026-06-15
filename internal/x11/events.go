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
	_ Event         = &CirculateNotifyEvent{}
	_ Event         = &CirculateRequestEvent{}
	_ WritableEvent = &ClientMessageEvent{}
	_ Event         = &ColormapNotifyEvent{}
	_ Event         = &ConfigureRequestEvent{}
	_ Event         = &ConfigureNotifyEvent{}
	_ Event         = &CreateNotifyEvent{}
	_ Event         = &DestroyNotifyEvent{}
	_ Event         = &EnterNotifyEvent{}
	_ Event         = &LeaveNotifyEvent{}
	_ Event         = &ErrorEvent{}
	_ Event         = &ExposeEvent{}
	_ Event         = &FocusInEvent{}
	_ Event         = &FocusOutEvent{}
	_ Event         = &GraphicsExposureEvent{}
	_ Event         = &GravityNotifyEvent{}
	_ Event         = &KeyPressEvent{}
	_ Event         = &KeyReleaseEvent{}
	_ Event         = &ButtonPressEvent{}
	_ Event         = &ButtonReleaseEvent{}
	_ Event         = &MotionNotifyEvent{}
	_ Event         = &KeymapNotifyEvent{}
	_ Event         = &MappingNotifyEvent{}
	_ Event         = &NoExposureEvent{}
	_ Event         = &PropertyNotifyEvent{}
	_ Event         = &ReparentNotifyEvent{}
	_ Event         = &ResizeRequestEvent{}
	_ Event         = &SelectionClearEvent{}
	_ Event         = &SelectionRequestEvent{}
	_ WritableEvent = &SelectionNotifyEvent{}
	_ Event         = &VisibilityNotifyEvent{}
	_ Event         = &MapRequestEvent{}
	_ Event         = &MapNotifyEvent{}
	_ Event         = &UnmapNotifyEvent{}
)

// Constants for X11 event codes.
const (
	eventCodeNone = iota
	_
	eventCodeKeyPress
	eventCodeKeyRelease
	eventCodeButtonPress
	eventCodeButtonRelease
	eventCodeMotionNotify
	eventCodeEnterNotify
	eventCodeLeaveNotify
	eventCodeFocusIn
	eventCodeFocusOut
	eventCodeKeymapNotify
	eventCodeExpose
	eventCodeGraphicsExposure
	eventCodeNoExposure
	eventCodeVisibilityNotify
	eventCodeCreateNotify
	eventCodeDestroyNotify
	eventCodeUnmapNotify
	eventCodeMapNotify
	eventCodeMapRequest
	eventCodeReparentNotify
	eventCodeConfigureNotify
	eventCodeConfigureRequest
	eventCodeGravityNotify
	eventCodeResizeRequest
	eventCodeCirculateNotify
	eventCodeCirculateRequest
	eventCodePropertyNotify
	eventCodeSelectionClear
	eventCodeSelectionRequest
	eventCodeSelectionNotify
	eventCodeColormapNotify
	eventCodeClientMessage
	eventCodeMappingNotify
	eventCodeGenericEvent // XGE; variable-length, with the number of additional 32-bit words in its length field
)

// Constants for X11 event masks.
const (
	EventMaskKeyPress = 1 << iota
	EventMaskKeyRelease
	EventMaskButtonPress
	EventMaskButtonRelease
	EventMaskEnterWindow
	EventMaskLeaveWindow
	EventMaskPointerMotion
	EventMaskPointerMotionHint
	EventMaskButton1Motion
	EventMaskButton2Motion
	EventMaskButton3Motion
	EventMaskButton4Motion
	EventMaskButton5Motion
	EventMaskButtonMotion
	EventMaskKeymapState
	EventMaskExposure
	EventMaskVisibilityChange
	EventMaskStructureNotify
	EventMaskResizeRedirect
	EventMaskSubstructureNotify
	EventMaskSubstructureRedirect
	EventMaskFocusChange
	EventMaskPropertyChange
	EventMaskColormapChange
	EventMaskOwnerGrabButton
	EventMaskNone = 0
)

// Event represents a generic X11 event. Specific event types will implement this interface.
type Event interface {
	// ID returns a byte value that identifies the type of the event, which can be used to determine how to process it.
	ID() byte
}

// WritableEvent represents an event that can be sent to the X server.
type WritableEvent interface {
	Write(sequence uint16, w *Writer)
	Event
}

func newEventMap() map[byte]func(*Reader) Event {
	return map[byte]func(r *Reader) Event{
		eventCodeKeyPress:         newKeyPressEvent,
		eventCodeKeyRelease:       newKeyReleaseEvent,
		eventCodeButtonPress:      newButtonPressEvent,
		eventCodeButtonRelease:    newButtonReleaseEvent,
		eventCodeMotionNotify:     newMotionNotifyEvent,
		eventCodeEnterNotify:      newEnterNotifyEvent,
		eventCodeLeaveNotify:      newLeaveNotifyEvent,
		eventCodeFocusIn:          newFocusInEvent,
		eventCodeFocusOut:         newFocusOutEvent,
		eventCodeKeymapNotify:     newKeymapNotifyEvent,
		eventCodeExpose:           newExposeEvent,
		eventCodeGraphicsExposure: newGraphicsExposureEvent,
		eventCodeNoExposure:       newNoExposureEvent,
		eventCodeVisibilityNotify: newVisibilityNotifyEvent,
		eventCodeCreateNotify:     newCreateNotifyEvent,
		eventCodeDestroyNotify:    newDestroyNotifyEvent,
		eventCodeUnmapNotify:      newUnmapNotifyEvent,
		eventCodeMapNotify:        newMapNotifyEvent,
		eventCodeMapRequest:       newMapRequestEvent,
		eventCodeReparentNotify:   newReparentNotifyEvent,
		eventCodeConfigureNotify:  newConfigureNotifyEvent,
		eventCodeConfigureRequest: newConfigureRequestEvent,
		eventCodeGravityNotify:    newGravityNotifyEvent,
		eventCodeResizeRequest:    newResizeRequestEvent,
		eventCodeCirculateNotify:  newCirculateNotifyEvent,
		eventCodeCirculateRequest: newCirculateRequestEvent,
		eventCodePropertyNotify:   newPropertyNotifyEvent,
		eventCodeSelectionClear:   newSelectionClearEvent,
		eventCodeSelectionRequest: newSelectionRequestEvent,
		eventCodeSelectionNotify:  newSelectionNotifyEvent,
		eventCodeColormapNotify:   newColormapNotifyEvent,
		eventCodeClientMessage:    newClientMessageEvent,
		eventCodeMappingNotify:    newMappingNotifyEvent,
	}
}

// CirculateEvent represents an X11 generic circulate event.
type CirculateEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
	Place    byte
}

func (e *CirculateEvent) read(r *Reader) {
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	r.Skip(4)
	e.Place = r.Byte()
	r.Skip(3)
}

// ID returns the event code.
func (e *CirculateEvent) ID() byte {
	return e.Code
}

// CirculateNotifyEvent represents an X11 CirculateNotify event.
type CirculateNotifyEvent struct {
	CirculateEvent
}

func newCirculateNotifyEvent(r *Reader) Event {
	var e CirculateNotifyEvent
	e.read(r)
	return &e
}

// CirculateRequestEvent represents an X11 CirculateRequest event.
type CirculateRequestEvent struct {
	CirculateEvent
}

func newCirculateRequestEvent(r *Reader) Event {
	var e CirculateRequestEvent
	e.read(r)
	return &e
}

// ClientMessageEvent represents an X11 ClientMessage event.
type ClientMessageEvent struct {
	Data8    [20]byte
	Data16   [10]uint16
	Data32   [5]uint32
	Window   WindowID
	Type     Atom
	Sequence uint16
	Code     byte
	Format   byte
}

func newClientMessageEvent(r *Reader) Event {
	var e ClientMessageEvent
	e.Code = r.Byte()
	e.Format = r.Byte()
	e.Sequence = r.Uint16()
	e.Window = r.WindowID()
	e.Type = Atom(r.Uint32())
	r.IntoBytes(e.Data8[:])
	r.SeekRelative(-len(e.Data8))
	for i := range e.Data16 {
		e.Data16[i] = r.Uint16()
	}
	r.SeekRelative(-len(e.Data8))
	for i := range e.Data32 {
		e.Data32[i] = r.Uint32()
	}
	return &e
}

// Write implements [WritableEvent].
func (e *ClientMessageEvent) Write(sequence uint16, w *Writer) {
	w.Byte(eventCodeClientMessage)
	w.Byte(e.Format)
	w.Uint16(sequence)
	w.WindowID(e.Window)
	w.Atom(e.Type)
	switch e.Format {
	case 8:
		w.Bytes(e.Data8[:])
	case 16:
		w.Uint16Slice(e.Data16[:])
	case 32:
		w.Uint32Slice(e.Data32[:])
	}
}

// ID returns the event code.
func (e *ClientMessageEvent) ID() byte {
	return e.Code
}

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
	e.Window = r.WindowID()
	e.Colormap = r.ColorMapID()
	e.New = r.Bool()
	e.State = r.Byte()
	r.Skip(2)
	return &e
}

// ID returns the event code.
func (e *ColormapNotifyEvent) ID() byte {
	return e.Code
}

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
	StackMode   StackMode
}

func newConfigureRequestEvent(r *Reader) Event {
	var e ConfigureRequestEvent
	e.Code = r.Byte()
	e.StackMode = StackMode(r.Byte())
	e.Sequence = r.Uint16()
	e.Parent = r.WindowID()
	e.Window = r.WindowID()
	e.Sibling = r.WindowID()
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.ValueMask = r.Uint16()
	return &e
}

// ID returns the event code.
func (e *ConfigureRequestEvent) ID() byte {
	return e.Code
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
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	e.AboveSibling = r.WindowID()
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.OverrideRedirect = r.Bool()
	r.Skip(1)
	return &e
}

// ID returns the event code.
func (e *ConfigureNotifyEvent) ID() byte {
	return e.Code
}

// CreateNotifyEvent represents an X11 CreateNotify event.
type CreateNotifyEvent struct {
	Parent           WindowID
	Window           WindowID
	Sequence         uint16
	X                int16
	Y                int16
	Width            uint16
	Height           uint16
	BorderWidth      uint16
	Code             byte
	OverrideRedirect bool
}

func newCreateNotifyEvent(r *Reader) Event {
	var e CreateNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Parent = r.WindowID()
	e.Window = r.WindowID()
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.OverrideRedirect = r.Bool()
	r.Skip(1)
	return &e
}

// ID returns the event code.
func (e *CreateNotifyEvent) ID() byte {
	return e.Code
}

// DestroyNotifyEvent represents an X11 DestroyNotify event.
type DestroyNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
}

func newDestroyNotifyEvent(r *Reader) Event {
	var e DestroyNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	return &e
}

// ID returns the event code.
func (e *DestroyNotifyEvent) ID() byte {
	return e.Code
}

// EnterLeaveEvent represents an X11 generic enter/leave event.
type EnterLeaveEvent struct {
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

func newEnterNotifyEvent(r *Reader) Event {
	var e EnterNotifyEvent
	e.read(r)
	return &e
}

func (e *EnterLeaveEvent) read(r *Reader) {
	e.Code = r.Byte()
	e.Detail = r.Byte()
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Root = r.WindowID()
	e.Event = r.WindowID()
	e.Child = r.WindowID()
	e.RootX = r.Int16()
	e.RootY = r.Int16()
	e.EventX = r.Int16()
	e.EventY = r.Int16()
	e.State = r.Uint16()
	e.Mode = r.Byte()
	e.SameScreenFocus = r.Byte()
}

// ID returns the event code.
func (e *EnterLeaveEvent) ID() byte {
	return e.Code
}

// EnterNotifyEvent represents an X11 EnterNotify event.
type EnterNotifyEvent struct {
	EnterLeaveEvent
}

// LeaveNotifyEvent represents an X11 LeaveNotify event.
type LeaveNotifyEvent struct {
	EnterLeaveEvent
}

func newLeaveNotifyEvent(r *Reader) Event {
	var e LeaveNotifyEvent
	e.read(r)
	return &e
}

// ErrorEvent is an error delivered as an event.
type ErrorEvent struct {
	Error error
}

// ID returns the event code.
func (e *ErrorEvent) ID() byte {
	return eventCodeNone
}

// ExposeEvent represents an X11 Expose event.
type ExposeEvent struct {
	Window   WindowID
	Sequence uint16
	X        uint16
	Y        uint16
	Width    uint16
	Height   uint16
	Count    uint16
	Code     byte
}

func newExposeEvent(r *Reader) Event {
	var e ExposeEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = r.WindowID()
	e.X = r.Uint16()
	e.Y = r.Uint16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.Count = r.Uint16()
	r.Skip(2)
	return &e
}

// ID returns the event code.
func (e *ExposeEvent) ID() byte {
	return e.Code
}

// Possible values for the FocusEvent mode.
const (
	NotifyNormal = iota
	NotifyGrab
	NotifyUngrab
	NotifyWhileGrabbed
)

// FocusEvent represents an X11 generic focus event.
type FocusEvent struct {
	Window   WindowID
	Sequence uint16
	Code     byte
	Detail   byte
	Mode     byte
}

func (e *FocusEvent) read(r *Reader) {
	e.Code = r.Byte()
	e.Detail = r.Byte()
	e.Sequence = r.Uint16()
	e.Window = r.WindowID()
	e.Mode = r.Byte()
	r.Skip(23)
}

// ID returns the event code.
func (e *FocusEvent) ID() byte {
	return e.Code
}

// FocusInEvent represents an X11 FocusIn event.
type FocusInEvent struct {
	FocusEvent
}

func newFocusInEvent(r *Reader) Event {
	var e FocusInEvent
	e.read(r)
	return &e
}

// FocusOutEvent represents an X11 FocusOut event.
type FocusOutEvent struct {
	FocusEvent
}

func newFocusOutEvent(r *Reader) Event {
	var e FocusOutEvent
	e.read(r)
	return &e
}

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
	e.Drawable = r.DrawableID()
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

// GravityNotifyEvent represents an X11 GravityNotify event.
type GravityNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	X        int16
	Y        int16
	Code     byte
}

func newGravityNotifyEvent(r *Reader) Event {
	var e GravityNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	e.X = r.Int16()
	e.Y = r.Int16()
	return &e
}

// ID returns the event code.
func (e *GravityNotifyEvent) ID() byte {
	return e.Code
}

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
	e.Root = r.WindowID()
	e.Event = r.WindowID()
	e.Child = r.WindowID()
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

// KeyPressEvent represents an X11 KeyPress event.
type KeyPressEvent struct {
	InputEvent
}

func newKeyPressEvent(r *Reader) Event {
	var e KeyPressEvent
	e.read(r)
	return &e
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

// ButtonPressEvent represents an X11 ButtonPress event.
type ButtonPressEvent struct {
	InputEvent
}

func newButtonPressEvent(r *Reader) Event {
	var e ButtonPressEvent
	e.read(r)
	return &e
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

// MotionNotifyEvent represents an X11 MotionNotify event.
type MotionNotifyEvent struct {
	InputEvent
}

func newMotionNotifyEvent(r *Reader) Event {
	var e MotionNotifyEvent
	e.read(r)
	return &e
}

// KeymapNotifyEvent represents an X11 KeymapNotify event.
type KeymapNotifyEvent struct {
	Code byte
	Keys [31]byte
}

func newKeymapNotifyEvent(r *Reader) Event {
	var e KeymapNotifyEvent
	e.Code = r.Byte()
	r.IntoBytes(e.Keys[:])
	return &e
}

// ID returns the event code.
func (e *KeymapNotifyEvent) ID() byte {
	return e.Code
}

// MappingNotifyEvent represents an X11 MappingNotify event.
type MappingNotifyEvent struct {
	Sequence     uint16
	Code         byte
	Request      byte
	FirstKeycode byte
	Count        byte
}

func newMappingNotifyEvent(r *Reader) Event {
	var e MappingNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Request = r.Byte()
	e.FirstKeycode = r.Byte()
	e.Count = r.Byte()
	r.Skip(1)
	return &e
}

// ID returns the event code.
func (e *MappingNotifyEvent) ID() byte {
	return e.Code
}

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
	e.Drawable = r.DrawableID()
	e.MinorOpcode = r.Uint16()
	e.MajorOpcode = r.Byte()
	r.Skip(1)
	return &e
}

// ID returns the event code.
func (e *NoExposureEvent) ID() byte {
	return e.Code
}

// PropertyNotifyEvent represents an X11 PropertyNotify event.
type PropertyNotifyEvent struct {
	Window   WindowID
	Atom     Atom
	Time     uint32
	Sequence uint16
	Code     byte
	State    byte
}

func newPropertyNotifyEvent(r *Reader) Event {
	var e PropertyNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = r.WindowID()
	e.Atom = Atom(r.Uint32())
	e.Time = r.Uint32()
	e.State = r.Byte()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *PropertyNotifyEvent) ID() byte {
	return e.Code
}

// ReparentNotifyEvent represents an X11 ReparentNotify event.
type ReparentNotifyEvent struct {
	Event            WindowID
	Window           WindowID
	Parent           WindowID
	Sequence         uint16
	X                int16
	Y                int16
	Code             byte
	OverrideRedirect bool
}

func newReparentNotifyEvent(r *Reader) Event {
	var e ReparentNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	e.Parent = r.WindowID()
	e.X = r.Int16()
	e.Y = r.Int16()
	e.OverrideRedirect = r.Bool()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *ReparentNotifyEvent) ID() byte {
	return e.Code
}

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
	e.Window = r.WindowID()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	return &e
}

// ID returns the event code.
func (e *ResizeRequestEvent) ID() byte {
	return e.Code
}

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
	e.Owner = r.WindowID()
	e.Selection = Atom(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *SelectionClearEvent) ID() byte {
	return e.Code
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
	e.Owner = r.WindowID()
	e.Requestor = r.WindowID()
	e.Selection = Atom(r.Uint32())
	e.Target = Atom(r.Uint32())
	e.Property = Atom(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *SelectionRequestEvent) ID() byte {
	return e.Code
}

func (e *SelectionRequestEvent) writeTargetToProperty(c *Conn) (property Atom, transfers []*incrTransfer) {
	if e.Property == AtomNone {
		return AtomNone, nil
	}
	entries := c.selectionEntries(e.Selection)
	switch e.Target {
	case c.Atoms.ClipboardTargets:
		w := NewWriter(4 * (2 + len(entries)))
		w.Atom(c.Atoms.ClipboardTargets)
		w.Atom(c.Atoms.ClipboardMultiple)
		for _, entry := range entries {
			w.Atom(entry.target)
		}
		c.ChangeProperty(e.Requestor, e.Property, AtomAtom, 32, PropModeReplace, w.Retrieve())
		return e.Property, nil
	case c.Atoms.ClipboardMultiple:
		format, kind, value, _, err := c.GetProperty(e.Requestor, e.Property, c.Atoms.Pair, 0, math.MaxUint32, false)
		if err != nil {
			errs.Log(err)
			return e.Property, nil
		}
		count := len(value) / 4
		if format != 32 || kind != c.Atoms.Pair || count%2 != 0 {
			slog.Error("unexpected result from GetProperty for MULTIPLE property", "format", format, "kind", kind, "count", count)
			return e.Property, nil
		}
		w := NewWriter(4 * count)
		r := NewReader(value)
		for i := 0; i < count; i += 2 {
			target := r.Atom()
			prop := r.Atom()
			w.Atom(target)
			if entry, ok := entryForTarget(entries, target); ok && prop != AtomNone {
				if t := c.writeClipboardProperty(e.Requestor, prop, entry); t != nil {
					transfers = append(transfers, t)
				}
				w.Atom(prop)
			} else {
				// Per ICCCM, a failed conversion is indicated by replacing the property in the pair with None
				w.Atom(AtomNone)
			}
		}
		c.ChangeProperty(e.Requestor, e.Property, c.Atoms.Pair, 32, PropModeReplace, w.Retrieve())
		return e.Property, transfers
	case c.Atoms.ClipboardSaveTargets:
		c.ChangeProperty(e.Requestor, e.Property, c.Atoms.Null, 32, PropModeReplace, nil)
		return e.Property, nil
	default:
		if entry, ok := entryForTarget(entries, e.Target); ok {
			if t := c.writeClipboardProperty(e.Requestor, e.Property, entry); t != nil {
				transfers = append(transfers, t)
			}
			return e.Property, transfers
		}
	}
	return AtomNone, nil
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
	e.Requestor = r.WindowID()
	e.Selection = Atom(r.Uint32())
	e.Target = Atom(r.Uint32())
	e.Property = Atom(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *SelectionNotifyEvent) ID() byte {
	return e.Code
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

// VisibilityNotifyEvent represents an X11 VisibilityNotify event.
type VisibilityNotifyEvent struct {
	Window   WindowID
	Sequence uint16
	State    byte
	Code     byte
}

func newVisibilityNotifyEvent(r *Reader) Event {
	var e VisibilityNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = r.WindowID()
	e.State = r.Byte()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *VisibilityNotifyEvent) ID() byte {
	return e.Code
}

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
	e.Parent = r.WindowID()
	e.Window = r.WindowID()
	return &e
}

// ID returns the event code.
func (e *MapRequestEvent) ID() byte {
	return e.Code
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
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	e.OverrideRedirect = r.Bool()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *MapNotifyEvent) ID() byte {
	return e.Code
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
	e.Event = r.WindowID()
	e.Window = r.WindowID()
	e.FromConfigure = r.Bool()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *UnmapNotifyEvent) ID() byte {
	return e.Code
}
