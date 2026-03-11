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
	r.Skip(23)
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

// KeymapNotifyEvent represents an X11 KeymapNotify event.
type KeymapNotifyEvent struct {
	Code byte
	Keys [31]byte
}

func newKeymapNotifyEvent(r *Reader) any {
	var e KeymapNotifyEvent
	e.Code = r.Byte()
	r.IntoBytes(e.Keys[:])
	return &e
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

func newExposeEvent(r *Reader) any {
	var e ExposeEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.X = r.Uint16()
	e.Y = r.Uint16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.Count = r.Uint16()
	r.Skip(2)
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

func newGraphicsExposureEvent(r *Reader) any {
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

// NoExposureEvent represents an X11 NoExposure event.
type NoExposureEvent struct {
	Sequence    uint16
	Drawable    DrawableID
	MinorOpcode uint16
	MajorOpcode byte
	Code        byte
}

func newNoExposureEvent(r *Reader) any {
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

// VisibilityNotifyEvent represents an X11 VisibilityNotify event.
type VisibilityNotifyEvent struct {
	Window   WindowID
	Sequence uint16
	State    byte
	Code     byte
}

func newVisibilityNotifyEvent(r *Reader) any {
	var e VisibilityNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.State = r.Byte()
	r.Skip(3)
	return &e
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

func newCreateNotifyEvent(r *Reader) any {
	var e CreateNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Parent = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	e.BorderWidth = r.Uint16()
	e.OverrideRedirect = r.Bool()
	r.Skip(1)
	return &e
}

// DestroyNotifyEvent represents an X11 DestroyNotify event.
type DestroyNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
}

func newDestroyNotifyEvent(r *Reader) any {
	var e DestroyNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	return &e
}

// UnmapNotifyEvent represents an X11 UnmapNotify event.
type UnmapNotifyEvent struct {
	Event         WindowID
	Window        WindowID
	Sequence      uint16
	Code          byte
	FromConfigure bool
}

func newUnmapNotifyEvent(r *Reader) any {
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

// MapNotifyEvent represents an X11 MapNotify event.
type MapNotifyEvent struct {
	Event            WindowID
	Window           WindowID
	Sequence         uint16
	Code             byte
	OverrideRedirect bool
}

func newMapNotifyEvent(r *Reader) any {
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

// MapRequestEvent represents an X11 MapRequest event.
type MapRequestEvent struct {
	Parent   WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
}

func newMapRequestEvent(r *Reader) any {
	var e MapRequestEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Parent = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	return &e
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

func newReparentNotifyEvent(r *Reader) any {
	var e ReparentNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.Parent = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	e.OverrideRedirect = r.Bool()
	r.Skip(3)
	return &e
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

func newConfigureNotifyEvent(r *Reader) any {
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

func newConfigureRequestEvent(r *Reader) any {
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

// GravityNotifyEvent represents an X11 GravityNotify event.
type GravityNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	X        int16
	Y        int16
	Code     byte
}

func newGravityNotifyEvent(r *Reader) any {
	var e GravityNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	return &e
}

// ResizeRequestEvent represents an X11 ResizeRequest event.
type ResizeRequestEvent struct {
	Window   WindowID
	Sequence uint16
	Width    uint16
	Height   uint16
	Code     byte
}

func newResizeRequestEvent(r *Reader) any {
	var e ResizeRequestEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	return &e
}

// CirculateNotifyEvent represents an X11 CirculateNotify event.
type CirculateNotifyEvent struct {
	Event    WindowID
	Window   WindowID
	Sequence uint16
	Code     byte
	Place    byte
}

func (e *CirculateNotifyEvent) read(r *Reader) {
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	r.Skip(4)
	e.Place = r.Byte()
	r.Skip(3)
}

func newCirculateNotifyEvent(r *Reader) any {
	var e CirculateNotifyEvent
	e.read(r)
	return &e
}

// CirculateRequestEvent represents an X11 CirculateRequest event.
type CirculateRequestEvent struct {
	CirculateNotifyEvent
}

func newCirculateRequestEvent(r *Reader) any {
	var e CirculateRequestEvent
	e.read(r)
	return &e
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

func newPropertyNotifyEvent(r *Reader) any {
	var e PropertyNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Atom = Atom(r.Uint32())
	e.Time = r.Uint32()
	e.State = r.Byte()
	r.Skip(3)
	return &e
}

// SelectionClearEvent represents an X11 SelectionClear event.
type SelectionClearEvent struct {
	Time      uint32
	Owner     WindowID
	Selection Atom
	Sequence  uint16
	Code      byte
}

func newSelectionClearEvent(r *Reader) any {
	var e SelectionClearEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Time = r.Uint32()
	e.Owner = WindowID(r.Uint32())
	e.Selection = Atom(r.Uint32())
	return &e
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

func newSelectionRequestEvent(r *Reader) any {
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

func newSelectionNotifyEvent(r *Reader) any {
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

// ColormapNotifyEvent represents an X11 ColormapNotify event.
type ColormapNotifyEvent struct {
	Sequence uint16
	Window   WindowID
	Colormap ColorMapID
	New      bool
	Code     byte
	State    byte
}

func newColormapNotifyEvent(r *Reader) any {
	var e ColormapNotifyEvent
	e.Code = r.Byte()
	r.Skip(1)
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
	e.Colormap = ColorMapID(r.Uint32())
	e.New = r.Bool()
	e.State = r.Byte()
	r.Skip(2)
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

func newClientMessageEvent(r *Reader) any {
	var e ClientMessageEvent
	e.Code = r.Byte()
	e.Format = r.Byte()
	e.Sequence = r.Uint16()
	e.Window = WindowID(r.Uint32())
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

// MappingNotifyEvent represents an X11 MappingNotify event.
type MappingNotifyEvent struct {
	Sequence     uint16
	Code         byte
	Request      byte
	FirstKeycode byte
	Count        byte
}

func newMappingNotifyEvent(r *Reader) any {
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
