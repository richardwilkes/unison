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
	_ Event         = &ClientMessageEvent{}
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

// Event represents a generic X11 event. Specific event types will implement this interface.
type Event interface {
	// ID returns a byte value that identifies the type of the event, which can be used to determine how to process it.
	ID() byte
	// TargetWindow returns the ID of the window that is the target of the event, if applicable. For events that do not
	// have a specific target window, this will return WindowNone.
	TargetWindow() WindowID
	// Process the event using the provided connection. The implementation should perform any necessary actions based on
	// the event type and its data.
	Process(*Conn)
}

// WritableEvent represents an event that can be sent to the X server.
type WritableEvent interface {
	Write(sequence uint16, w *Writer)
	Event
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
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	r.Skip(4)
	e.Place = r.Byte()
	r.Skip(3)
}

// ID returns the event code.
func (e *CirculateEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *CirculateEvent) TargetWindow() WindowID {
	return e.Event
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

// Process the event.
func (e *CirculateNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("CirculateNotifyEvent received", "sequence", e.Sequence, "event", e.Event, "window", e.Window, "place", e.Place)
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

// Process the event.
func (e *CirculateRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("CirculateRequestEvent received", "sequence", e.Sequence, "event", e.Event, "window", e.Window, "place", e.Place)
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

// ID returns the event code.
func (e *ClientMessageEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ClientMessageEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ClientMessageEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ClientMessageEvent received", "sequence", e.Sequence, "window", e.Window, "type", e.Type, "format", e.Format, "data8", e.Data8, "data16", e.Data16, "data32", e.Data32)
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
	e.Window = WindowID(r.Uint32())
	e.Colormap = ColorMapID(r.Uint32())
	e.New = r.Bool()
	e.State = r.Byte()
	r.Skip(2)
	return &e
}

// ID returns the event code.
func (e *ColormapNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ColormapNotifyEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ColormapNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ColormapNotifyEvent received", "sequence", e.Sequence, "window", e.Window, "colormap", e.Colormap, "new", e.New, "state", e.State)
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

// ID returns the event code.
func (e *ConfigureRequestEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ConfigureRequestEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ConfigureRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ConfigureRequestEvent received", "sequence", e.Sequence, "window", e.Window, "parent", e.Parent, "sibling", e.Sibling, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "borderWidth", e.BorderWidth, "valueMask", e.ValueMask, "stackMode", e.StackMode)
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

// ID returns the event code.
func (e *ConfigureNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ConfigureNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *ConfigureNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ConfigureNotifyEvent received", "sequence", e.Sequence, "event", e.Event, "window", e.Window, "aboveSibling", e.AboveSibling, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "borderWidth", e.BorderWidth, "overrideRedirect", e.OverrideRedirect)
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

// ID returns the event code.
func (e *CreateNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *CreateNotifyEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *CreateNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("CreateNotifyEvent received", "sequence", e.Sequence, "parent", e.Parent, "window", e.Window, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "borderWidth", e.BorderWidth, "overrideRedirect", e.OverrideRedirect)
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
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	return &e
}

// ID returns the event code.
func (e *DestroyNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *DestroyNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *DestroyNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("DestroyNotifyEvent received", "sequence", e.Sequence, "event", e.Event, "window", e.Window)
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

// ID returns the event code.
func (e *EnterLeaveEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *EnterLeaveEvent) TargetWindow() WindowID {
	return e.Event
}

// EnterNotifyEvent represents an X11 EnterNotify event.
type EnterNotifyEvent struct {
	EnterLeaveEvent
}

// Process the event.
func (e *EnterNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("EnterNotifyEvent received", "sequence", e.Sequence, "root", e.Root, "event", e.Event, "child", e.Child, "time", e.Time, "state", e.State, "rootX", e.RootX, "rootY", e.RootY, "eventX", e.EventX, "eventY", e.EventY, "detail", e.Detail, "mode", e.Mode, "sameScreenFocus", e.SameScreenFocus)
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

// Process the event.
func (e *LeaveNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("LeaveNotifyEvent received", "sequence", e.Sequence, "root", e.Root, "event", e.Event, "child", e.Child, "time", e.Time, "state", e.State, "rootX", e.RootX, "rootY", e.RootY, "eventX", e.EventX, "eventY", e.EventY, "detail", e.Detail, "mode", e.Mode, "sameScreenFocus", e.SameScreenFocus)
}

// ErrorEvent is an error delivered as an event.
type ErrorEvent struct {
	Error error
}

// ID returns the event code.
func (e *ErrorEvent) ID() byte {
	return eventCodeNone
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ErrorEvent) TargetWindow() WindowID {
	return 0
}

// Process the event.
func (e *ErrorEvent) Process(_conn *Conn) {
	errs.Log(e.Error)
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
	e.Window = WindowID(r.Uint32())
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

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ExposeEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ExposeEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ExposeEvent received", "sequence", e.Sequence, "window", e.Window, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "count", e.Count)
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

// ID returns the event code.
func (e *FocusEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *FocusEvent) TargetWindow() WindowID {
	return e.Event
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

// Process the event.
func (e *FocusInEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("FocusInEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "mode", e.Mode)
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

// Process the event.
func (e *FocusOutEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("FocusOutEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "mode", e.Mode)
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
	slog.Info("GraphicsExposureEvent received", "sequence", e.Sequence, "drawable", e.Drawable, "x", e.X, "y", e.Y, "width", e.Width, "height", e.Height, "minorOpcode", e.MinorOpcode, "count", e.Count, "majorOpcode", e.MajorOpcode)
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
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.X = r.Int16()
	e.Y = r.Int16()
	return &e
}

// ID returns the event code.
func (e *GravityNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *GravityNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *GravityNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("GravityNotifyEvent received", "sequence", e.Sequence, "window", e.Window, "x", e.X, "y", e.Y)
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
	slog.Info("KeyPressEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "state", e.State, "sameScreen", e.SameScreen)
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
	slog.Info("KeyReleaseEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "state", e.State, "sameScreen", e.SameScreen)
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
	slog.Info("ButtonPressEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "state", e.State, "sameScreen", e.SameScreen)
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
	slog.Info("ButtonReleaseEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "state", e.State, "sameScreen", e.SameScreen)
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
	slog.Info("MotionNotifyEvent received", "sequence", e.Sequence, "window", e.Event, "detail", e.Detail, "state", e.State, "sameScreen", e.SameScreen)
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

// TargetWindow returns the ID of the window that is the target of the event.
func (e *KeymapNotifyEvent) TargetWindow() WindowID {
	return 0
}

// Process the event.
func (e *KeymapNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("KeymapNotifyEvent received", "keys", e.Keys)
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

// TargetWindow returns the ID of the window that is the target of the event.
func (e *MappingNotifyEvent) TargetWindow() WindowID {
	return 0
}

// Process the event.
func (e *MappingNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("MappingNotifyEvent received", "sequence", e.Sequence, "request", e.Request, "firstKeycode", e.FirstKeycode, "count", e.Count)
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
	slog.Info("NoExposureEvent received", "sequence", e.Sequence, "drawable", e.Drawable, "minorOpcode", e.MinorOpcode, "majorOpcode", e.MajorOpcode)
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
	e.Window = WindowID(r.Uint32())
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

// TargetWindow returns the ID of the window that is the target of the event.
func (e *PropertyNotifyEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *PropertyNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("PropertyNotifyEvent received", "sequence", e.Sequence, "window", e.Window, "atom", e.Atom, "time", e.Time, "state", e.State)
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
	e.Event = WindowID(r.Uint32())
	e.Window = WindowID(r.Uint32())
	e.Parent = WindowID(r.Uint32())
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

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ReparentNotifyEvent) TargetWindow() WindowID {
	return e.Event
}

// Process the event.
func (e *ReparentNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ReparentNotifyEvent received", "sequence", e.Sequence, "window", e.Window, "parent", e.Parent, "x", e.X, "y", e.Y, "overrideRedirect", e.OverrideRedirect)
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
	e.Window = WindowID(r.Uint32())
	e.Width = r.Uint16()
	e.Height = r.Uint16()
	return &e
}

// ID returns the event code.
func (e *ResizeRequestEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *ResizeRequestEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *ResizeRequestEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("ResizeRequestEvent received", "sequence", e.Sequence, "window", e.Window, "width", e.Width, "height", e.Height)
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
	case c.Atoms.ClipboardTargets:
		w := NewWriter(16)
		w.Atom(c.Atoms.ClipboardTargets)
		w.Atom(c.Atoms.ClipboardMultiple)
		w.Atom(c.Atoms.UTF8String)
		w.Atom(AtomString)
		c.ChangeProperty(e.Requestor, e.Property, AtomAtom, 32, PropModeReplace, w.Retrieve())
		return e.Property
	case c.Atoms.ClipboardMultiple:
		format, kind, value, err := c.GetProperty(e.Requestor, e.Property, c.Atoms.Pair, 0, math.MaxUint32, false)
		count := len(value) / (int(format) / 8)
		if err != nil {
			errs.Log(err)
			return e.Property
		}
		if format != 32 || kind != c.Atoms.Pair || count%2 != 0 {
			slog.Error("unexpected result from GetProperty for MULTIPLE property", "format", format, "kind", kind, "count", count)
			return e.Property
		}
		content := []byte(c.clipboard)
		w := NewWriter(8 * count)
		r := NewReader(value)
		for i := 0; i < count; i += 2 {
			propType := r.Atom()
			prop := r.Atom()
			if propType == c.Atoms.UTF8String || propType == AtomString {
				w.Atom(propType)
				c.ChangeProperty(e.Requestor, prop, propType, 8, PropModeReplace, content)
			} else {
				w.Atom(AtomNone)
			}
			w.Atom(prop)
		}
		c.ChangeProperty(e.Requestor, e.Property, c.Atoms.Pair, 32, PropModeReplace, w.Retrieve())
		return e.Property
	case c.Atoms.ClipboardSaveTargets:
		c.ChangeProperty(e.Requestor, e.Property, c.Atoms.Null, 32, PropModeReplace, nil)
		return e.Property
	case c.Atoms.UTF8String, AtomString:
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
	e.Window = WindowID(r.Uint32())
	e.State = r.Byte()
	r.Skip(3)
	return &e
}

// ID returns the event code.
func (e *VisibilityNotifyEvent) ID() byte {
	return e.Code
}

// TargetWindow returns the ID of the window that is the target of the event.
func (e *VisibilityNotifyEvent) TargetWindow() WindowID {
	return e.Window
}

// Process the event.
func (e *VisibilityNotifyEvent) Process(_conn *Conn) {
	// TODO: Implement
	slog.Info("VisibilityNotifyEvent received", "sequence", e.Sequence, "window", e.Window, "state", e.State)
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
	slog.Info("MapRequestEvent received", "sequence", e.Sequence, "window", e.Window, "parent", e.Parent)
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
	slog.Info("MapNotifyEvent received", "sequence", e.Sequence, "sequence", e.Sequence, "window", e.Window, "event", e.Event, "overrideRedirect", e.OverrideRedirect)
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
	slog.Info("UnmapNotifyEvent received", "sequence", e.Sequence, "window", e.Window, "event", e.Event, "fromConfigure", e.FromConfigure)
}
