// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"image"
	"reflect"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// Callbacks invoked by the macContentView created by NewView. They are invoked on the main thread from within the
// event loop.
var (
	// WindowKeyPressedCallback is invoked when a key is pressed while the view is first responder.
	WindowKeyPressedCallback func(w Window, key uint16, mods uint)
	// WindowKeyTypedCallback is invoked for each character produced by key input (via the NSTextInputClient path, so
	// IME composition results arrive here as well).
	WindowKeyTypedCallback func(w Window, ch rune)
	// WindowKeyReleasedCallback is invoked when a key is released while the view is first responder.
	WindowKeyReleasedCallback func(w Window, key uint16, mods uint)
	// WindowCursorUpdateCallback is invoked when the cursor should be updated for the view's tracking area.
	WindowCursorUpdateCallback func(Window)
	// WindowMouseEnterCallback is invoked when the mouse enters the view's tracking area.
	WindowMouseEnterCallback func(w Window, pt geom.Point, mods uint)
	// WindowMouseExitCallback is invoked when the mouse exits the view's tracking area.
	WindowMouseExitCallback func(Window)
	// WindowMouseMovedCallback is invoked when the mouse moves (or drags) within the view.
	WindowMouseMovedCallback func(w Window, pt geom.Point, mods uint)
	// WindowScrollCallback is invoked when the scroll wheel is used within the view.
	WindowScrollCallback func(w Window, delta geom.Point, mods uint)
	// WindowMouseClickCallback is invoked when a mouse button is pressed or released within the view.
	WindowMouseClickCallback func(w Window, button int, where geom.Point, pressed bool, mods uint)
	// WindowUpdateLayerCallback is invoked when the view's backing layer needs to be redrawn.
	WindowUpdateLayerCallback func(Window)
	// WindowScaleCallback is invoked when the view's backing scale changes (e.g. moving between retina and non-retina
	// displays).
	WindowScaleCallback func(w Window, scale geom.Point)
	// WindowRedrawCallback is invoked when the view needs to be redrawn via drawRect:.
	WindowRedrawCallback func(Window)
	// WindowDragEnterCallback is invoked when a drag enters the view.
	WindowDragEnterCallback func(w Window, d DragInfo, where geom.Point, mods uint) drag.Op
	// WindowDragUpdateCallback is invoked while a drag moves over the view.
	WindowDragUpdateCallback func(w Window, d DragInfo, where geom.Point, mods uint) drag.Op
	// WindowDropCallback is invoked when a drag is dropped on the view.
	WindowDropCallback func(w Window, d DragInfo, where geom.Point, mods uint) bool
	// WindowDragExitCallback is invoked when a drag leaves the view.
	WindowDragExitCallback func(w Window)
	// WindowDragSourceFinishedCallback is invoked when a drag this view started ends.
	WindowDragSourceFinishedCallback func(w Window)
)

// NSTrackingArea options (values verified against the macOS SDK's NSTrackingArea.h, which documents them as fixed).
const (
	nsTrackingMouseEnteredAndExited  uint64 = 0x01
	nsTrackingCursorUpdate           uint64 = 0x04
	nsTrackingActiveInKeyWindow      uint64 = 0x20
	nsTrackingAssumeInside           uint64 = 0x100
	nsTrackingInVisibleRect          uint64 = 0x200
	nsTrackingEnabledDuringMouseDrag uint64 = 0x400

	// nsTrackingOptions is the option set the old Objective-C macContentView used for its tracking area.
	nsTrackingOptions = nsTrackingMouseEnteredAndExited | nsTrackingActiveInKeyWindow |
		nsTrackingEnabledDuringMouseDrag | nsTrackingCursorUpdate | nsTrackingInVisibleRect | nsTrackingAssumeInside
)

// nsNotFound is Foundation's NSNotFound (NSIntegerMax).
const nsNotFound uint64 = 1<<63 - 1

// emptyRange mirrors the old bridge's kEmptyRange: {NSNotFound, 0}.
var emptyRange = NSRange{Location: nsNotFound, Length: 0}

// View is a handle to a macContentView instance, the NSView subclass that fills each unison window and routes all
// input, drawing, IME, and drag & drop through the callback funcs above. NewView returns an owned (+1) reference;
// balance it with Release.
type View objc.ID

var (
	macContentViewClassOnce sync.Once
	macContentViewClass     objc.Class
	macContentViewClassErr  error
)

// viewWindow returns the window a macContentView was created for (stored in its wnd ivar at creation time, exactly
// like the old Objective-C class, since callbacks may need it before the view is installed in the window).
func viewWindow(self objc.ID) Window {
	return Window(self.Send(Sel("wnd")))
}

// viewLocationFromEvent returns an event's location converted to the view's top-left-origin coordinate system,
// mirroring the old locationInWindowFromEvent: (the view fills its window, so only the y axis needs flipping).
func viewLocationFromEvent(self, event objc.ID) geom.Point {
	pt := objc.Send[NSPoint](event, Sel("locationInWindow"))
	frame := objc.Send[NSRect](self, Sel("frame"))
	return geom.NewPoint(float32(pt.X), float32(frame.Size.Height-pt.Y))
}

// viewLocationFromDrag returns a dragging-info location converted to the view's top-left-origin coordinate system,
// mirroring the old locationInWindowFromDrag:.
func viewLocationFromDrag(self, sender objc.ID) geom.Point {
	pt := objc.Send[NSPoint](sender, Sel("draggingLocation"))
	frame := objc.Send[NSRect](self, Sel("frame"))
	return geom.NewPoint(float32(pt.X), float32(frame.Size.Height-pt.Y))
}

// eventMods returns an event's modifier flags.
func eventMods(event objc.ID) uint {
	return uint(objc.Send[uint64](event, Sel("modifierFlags")))
}

// viewMouseClick routes a mouse press/release to WindowMouseClickCallback.
func viewMouseClick(self, event objc.ID, button int, pressed bool) {
	if WindowMouseClickCallback != nil {
		WindowMouseClickCallback(viewWindow(self), button, viewLocationFromEvent(self, event), pressed,
			eventMods(event))
	}
}

// viewMouseDragged handles a mouse-dragged event: unless this view is the source of an active drag session, it is
// forwarded to mouseMoved: with the event stashed in the lastMouseDraggedEvent ivar so that a drag session started
// from within the callback (View.BeginDraggingSession) can hand AppKit the triggering event.
func viewMouseDragged(self, event objc.ID) {
	if !objc.Send[bool](self, Sel("isInDragWeStarted")) {
		self.Send(Sel("setLastMouseDraggedEvent:"), event)
		self.Send(Sel("mouseMoved:"), event)
		self.Send(Sel("setLastMouseDraggedEvent:"), objc.ID(0))
	}
}

// viewDragOp implements the shared body of draggingEntered: and draggingUpdated:: the callback's answer is
// intersected with the drag source's allowed operations, mirroring the old export shims.
func viewDragOp(self, sender objc.ID, cb func(w Window, d DragInfo, where geom.Point, mods uint) drag.Op) uint64 {
	if cb == nil {
		return uint64(DragOpNone)
	}
	d := DragInfo(sender)
	op := cb(viewWindow(self), d, viewLocationFromDrag(self, sender), uint(CurrentModifierFlags()))
	return uint64(DragOpFromUnison(op & d.SourceDragOpMask()))
}

// viewBackingScale returns the scale of the view's backing store relative to its logical coordinate system,
// mirroring the old viewBackingScale helper.
func viewBackingScale(v objc.ID) geom.Point {
	const size = 1000
	fbRect := objc.Send[NSRect](v, Sel("convertRectToBacking:"),
		NSRect{Size: NSSize{Width: size, Height: size}})
	return geom.NewPoint(float32(fbRect.Size.Width)/size, float32(fbRect.Size.Height)/size)
}

// registerMacContentViewClass registers the macContentView Objective-C class: NSView plus the NSTextInputClient and
// NSDraggingSource protocols, with every override the old Objective-C implementation had. Registration is
// process-global and can only happen once per class name, so it is guarded by macContentViewClassOnce.
//
//nolint:gocognit,funlen // the method table is long by nature; splitting it apart would only obscure it
func registerMacContentViewClass() {
	LoadAppKit()
	var protocols []*objc.Protocol
	for _, name := range []string{"NSTextInputClient", "NSDraggingSource"} {
		if p := objc.GetProtocol(name); p != nil {
			protocols = append(protocols, p)
		}
	}
	cls, err := objc.RegisterClass("macContentView", Cls("NSView"), protocols, []objc.FieldDef{
		{Name: "wnd", Type: reflect.TypeFor[objc.ID](), Attribute: objc.ReadWrite},
		{Name: "trackingArea", Type: reflect.TypeFor[objc.ID](), Attribute: objc.ReadWrite},
		{Name: "markedText", Type: reflect.TypeFor[objc.ID](), Attribute: objc.ReadWrite},
		{Name: "lastMouseDraggedEvent", Type: reflect.TypeFor[objc.ID](), Attribute: objc.ReadWrite},
		{Name: "dragMask", Type: reflect.TypeFor[uint64](), Attribute: objc.ReadWrite},
		{Name: "inDragWeStarted", Type: reflect.TypeFor[bool](), Attribute: objc.ReadWrite},
	}, []objc.MethodDef{
		{
			Cmd: Sel("dealloc"),
			Fn: func(self objc.ID, _ objc.SEL) {
				Release(self.Send(Sel("trackingArea"))) // release is nil-safe
				Release(self.Send(Sel("markedText")))
				self.SendSuper(Sel("dealloc"))
			},
		},
		{
			Cmd: Sel("isOpaque"),
			Fn: func(self objc.ID, _ objc.SEL) bool {
				return objc.Send[bool](objc.ID(viewWindow(self)), Sel("isOpaque"))
			},
		},
		{
			Cmd: Sel("canBecomeKeyView"),
			Fn:  func(_ objc.ID, _ objc.SEL) bool { return true },
		},
		{
			Cmd: Sel("acceptsFirstResponder"),
			Fn:  func(_ objc.ID, _ objc.SEL) bool { return true },
		},
		{
			Cmd: Sel("wantsUpdateLayer"),
			Fn:  func(_ objc.ID, _ objc.SEL) bool { return true },
		},
		{
			Cmd: Sel("updateLayer"),
			Fn: func(self objc.ID, _ objc.SEL) {
				if WindowUpdateLayerCallback != nil {
					WindowUpdateLayerCallback(viewWindow(self))
				}
			},
		},
		{
			Cmd: Sel("cursorUpdate:"),
			Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
				if WindowCursorUpdateCallback != nil {
					WindowCursorUpdateCallback(viewWindow(self))
				}
			},
		},
		{
			// Without this, clicks on an inactive window would only activate it instead of also being delivered.
			Cmd: Sel("acceptsFirstMouse:"),
			Fn:  func(_ objc.ID, _ objc.SEL, _ objc.ID) bool { return true },
		},
		{
			Cmd: Sel("mouseDown:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseClick(self, event, 0, true) },
		},
		{
			Cmd: Sel("mouseDragged:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseDragged(self, event) },
		},
		{
			Cmd: Sel("mouseUp:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseClick(self, event, 0, false) },
		},
		{
			Cmd: Sel("mouseMoved:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				if WindowMouseMovedCallback != nil {
					WindowMouseMovedCallback(viewWindow(self), viewLocationFromEvent(self, event), eventMods(event))
				}
			},
		},
		{
			Cmd: Sel("rightMouseDown:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseClick(self, event, 1, true) },
		},
		{
			Cmd: Sel("rightMouseDragged:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseDragged(self, event) },
		},
		{
			Cmd: Sel("rightMouseUp:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseClick(self, event, 1, false) },
		},
		{
			Cmd: Sel("otherMouseDown:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				viewMouseClick(self, event, int(objc.Send[int64](event, Sel("buttonNumber"))), true)
			},
		},
		{
			Cmd: Sel("otherMouseDragged:"),
			Fn:  func(self objc.ID, _ objc.SEL, event objc.ID) { viewMouseDragged(self, event) },
		},
		{
			Cmd: Sel("otherMouseUp:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				viewMouseClick(self, event, int(objc.Send[int64](event, Sel("buttonNumber"))), false)
			},
		},
		{
			Cmd: Sel("mouseEntered:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				if WindowMouseEnterCallback != nil {
					WindowMouseEnterCallback(viewWindow(self), viewLocationFromEvent(self, event), eventMods(event))
				}
			},
		},
		{
			Cmd: Sel("mouseExited:"),
			Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
				if WindowMouseExitCallback != nil {
					WindowMouseExitCallback(viewWindow(self))
				}
			},
		},
		{
			Cmd: Sel("viewDidChangeBackingProperties"),
			Fn: func(self objc.ID, _ objc.SEL) {
				if WindowScaleCallback != nil {
					w := viewWindow(self)
					WindowScaleCallback(w, viewBackingScale(objc.ID(w).Send(Sel("contentView"))))
				}
			},
		},
		{
			// The dirty rect is ignored, as it was in the old implementation: unison always redraws the whole view.
			Cmd: Sel("drawRect:"),
			Fn: func(self objc.ID, _ objc.SEL, _ NSRect) {
				if WindowRedrawCallback != nil {
					WindowRedrawCallback(viewWindow(self))
				}
			},
		},
		{
			Cmd: Sel("updateTrackingAreas"),
			Fn: func(self objc.ID, _ objc.SEL) {
				if ta := self.Send(Sel("trackingArea")); ta != 0 {
					self.Send(Sel("removeTrackingArea:"), ta)
					Release(ta)
				}
				ta := objc.ID(Cls("NSTrackingArea")).Send(Sel("alloc")).Send(
					Sel("initWithRect:options:owner:userInfo:"), objc.Send[NSRect](self, Sel("bounds")),
					nsTrackingOptions, self, objc.ID(0))
				self.Send(Sel("setTrackingArea:"), ta)
				self.Send(Sel("addTrackingArea:"), ta)
				self.SendSuper(Sel("updateTrackingAreas"))
			},
		},
		{
			Cmd: Sel("keyDown:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				if WindowKeyPressedCallback != nil {
					WindowKeyPressedCallback(viewWindow(self), objc.Send[uint16](event, Sel("keyCode")),
						eventMods(event))
				}
				self.Send(Sel("interpretKeyEvents:"), NSArrayFromIDs(event))
			},
		},
		{
			Cmd: Sel("keyUp:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				if WindowKeyReleasedCallback != nil {
					WindowKeyReleasedCallback(viewWindow(self), objc.Send[uint16](event, Sel("keyCode")),
						eventMods(event))
				}
			},
		},
		{
			Cmd: Sel("scrollWheel:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				if WindowScrollCallback != nil {
					WindowScrollCallback(viewWindow(self),
						geom.NewPoint(float32(objc.Send[float64](event, Sel("scrollingDeltaX"))),
							float32(objc.Send[float64](event, Sel("scrollingDeltaY")))), eventMods(event))
				}
			},
		},
		{
			Cmd: Sel("draggingSession:sourceOperationMaskForDraggingContext:"),
			Fn: func(self objc.ID, _ objc.SEL, _ objc.ID, _ int64) uint64 {
				return objc.Send[uint64](self, Sel("dragMask"))
			},
		},
		{
			Cmd: Sel("draggingSession:endedAtPoint:operation:"),
			Fn: func(self objc.ID, _ objc.SEL, _ objc.ID, _ NSPoint, _ uint64) {
				if WindowDragSourceFinishedCallback != nil {
					WindowDragSourceFinishedCallback(viewWindow(self))
				}
				self.Send(Sel("setInDragWeStarted:"), false)
			},
		},
		{
			Cmd: Sel("ignoreModifierKeysForDraggingSession:"),
			Fn:  func(_ objc.ID, _ objc.SEL, _ objc.ID) bool { return false },
		},
		{
			Cmd: Sel("wantsPeriodicDraggingUpdates"),
			Fn:  func(_ objc.ID, _ objc.SEL) bool { return true },
		},
		{
			Cmd: Sel("draggingEntered:"),
			Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) uint64 {
				return viewDragOp(self, sender, WindowDragEnterCallback)
			},
		},
		{
			Cmd: Sel("draggingUpdated:"),
			Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) uint64 {
				return viewDragOp(self, sender, WindowDragUpdateCallback)
			},
		},
		{
			Cmd: Sel("performDragOperation:"),
			Fn: func(self objc.ID, _ objc.SEL, sender objc.ID) bool {
				if WindowDropCallback == nil {
					return false
				}
				return WindowDropCallback(viewWindow(self), DragInfo(sender), viewLocationFromDrag(self, sender),
					uint(CurrentModifierFlags()))
			},
		},
		{
			Cmd: Sel("draggingExited:"),
			Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) {
				if WindowDragExitCallback != nil {
					WindowDragExitCallback(viewWindow(self))
				}
			},
		},
		{
			Cmd: Sel("hasMarkedText"),
			Fn: func(self objc.ID, _ objc.SEL) bool {
				return objc.Send[uint64](self.Send(Sel("markedText")), Sel("length")) > 0
			},
		},
		{
			Cmd: Sel("markedRange"),
			Fn: func(self objc.ID, _ objc.SEL) NSRange {
				if length := objc.Send[uint64](self.Send(Sel("markedText")), Sel("length")); length > 0 {
					return NSRange{Location: 0, Length: length - 1}
				}
				return emptyRange
			},
		},
		{
			Cmd: Sel("selectedRange"),
			Fn:  func(_ objc.ID, _ objc.SEL) NSRange { return emptyRange },
		},
		{
			Cmd: Sel("setMarkedText:selectedRange:replacementRange:"),
			Fn: func(self objc.ID, _ objc.SEL, text objc.ID, _, _ NSRange) {
				Release(self.Send(Sel("markedText")))
				marked := objc.ID(Cls("NSMutableAttributedString")).Send(Sel("alloc"))
				if objc.Send[bool](text, Sel("isKindOfClass:"), Cls("NSAttributedString")) {
					marked = marked.Send(Sel("initWithAttributedString:"), text)
				} else {
					marked = marked.Send(Sel("initWithString:"), text)
				}
				self.Send(Sel("setMarkedText:"), marked)
			},
		},
		{
			Cmd: Sel("unmarkText"),
			Fn: func(self objc.ID, _ objc.SEL) {
				str := NewNSString("")
				self.Send(Sel("markedText")).Send(Sel("mutableString")).Send(Sel("setString:"), str)
				Release(str)
			},
		},
		{
			Cmd: Sel("validAttributesForMarkedText"),
			Fn:  func(_ objc.ID, _ objc.SEL) objc.ID { return NSArrayFromIDs() },
		},
		{
			Cmd: Sel("attributedSubstringForProposedRange:actualRange:"),
			Fn:  func(_ objc.ID, _ objc.SEL, _ NSRange, _ *NSRange) objc.ID { return 0 },
		},
		{
			Cmd: Sel("characterIndexForPoint:"),
			Fn:  func(_ objc.ID, _ objc.SEL, _ NSPoint) uint64 { return 0 },
		},
		{
			Cmd: Sel("firstRectForCharacterRange:actualRange:"),
			Fn: func(self objc.ID, _ objc.SEL, _ NSRange, _ *NSRange) NSRect {
				return NSRect{Origin: objc.Send[NSRect](self, Sel("frame")).Origin}
			},
		},
		{
			Cmd: Sel("insertText:replacementRange:"),
			Fn: func(self objc.ID, _ objc.SEL, text objc.ID, _ NSRange) {
				current := sharedApp().Send(Sel("currentEvent"))
				if current != 0 && EventModifierFlags(eventMods(current))&EventModifierFlagCommand != 0 {
					return
				}
				str := text
				if objc.Send[bool](text, Sel("isKindOfClass:"), Cls("NSAttributedString")) {
					str = text.Send(Sel("string"))
				}
				w := viewWindow(self)
				for _, ch := range GoStringFromNSString(str) {
					if ch >= 0xf700 && ch <= 0xf7ff { // function-key code points are reported via keyDown: instead
						continue
					}
					if WindowKeyTypedCallback != nil {
						WindowKeyTypedCallback(w, ch)
					}
				}
			},
		},
		{
			Cmd: Sel("doCommandBySelector:"),
			Fn:  func(_ objc.ID, _, _ objc.SEL) {},
		},
	})
	if err != nil {
		macContentViewClassErr = errs.NewWithCause("NewView: unable to register content view class", err)
		return
	}
	macContentViewClass = cls
}

// NewView returns a new macContentView for the given window, or 0 if the view could not be created. The steps mirror
// the old initWithWindow: initializer: super's init runs first, then the window ivar and an empty marked-text buffer
// are assigned, then the tracking area is installed.
func NewView(w Window) View {
	macContentViewClassOnce.Do(registerMacContentViewClass)
	if macContentViewClassErr != nil {
		errs.Log(macContentViewClassErr)
		return 0
	}
	v := objc.ID(macContentViewClass).Send(Sel("alloc")).Send(Sel("init"))
	if v == 0 {
		return 0
	}
	v.Send(Sel("setWnd:"), objc.ID(w))
	v.Send(Sel("setMarkedText:"), objc.ID(Cls("NSMutableAttributedString")).Send(Sel("alloc")).Send(Sel("init")))
	v.Send(Sel("updateTrackingAreas"))
	return View(v)
}

// BackingScale returns the scale of the view's backing store relative to its logical coordinate system (e.g. 2 on
// retina displays).
func (v View) BackingScale() geom.Point {
	return viewBackingScale(objc.ID(v))
}

// Frame returns the view's frame rect within its superview.
func (v View) Frame() geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(v), Sel("frame")))
}

// MouseInRect returns true if the given point (in the view's bottom-left-origin window coordinates) is inside the
// given rect, accounting for the view's flipped state.
func (v View) MouseInRect(mousePt geom.Point, rect geom.Rect) bool {
	return objc.Send[bool](objc.ID(v), Sel("mouse:inRect:"), NSPointFromPoint(mousePt), NSRectFromRect(rect))
}

// Release releases the view.
func (v View) Release() {
	Release(objc.ID(v))
}

// BeginDraggingSession starts a drag from this view with the given image and data. It must be called from within a
// mouse-dragged callback, since AppKit requires the triggering mouse event (stashed in the lastMouseDraggedEvent
// ivar) to start a session. Matching the old bridge, the pasteboard item and dragging item are not released here.
func (v View) BeginDraggingSession(img *image.NRGBA, frame geom.Rect, dragOpMask drag.Op, data ...drag.Data) {
	if len(data) == 0 {
		return
	}
	item := NewPasteboardItem()
	for _, d := range data {
		item.SetData(d.Type, d.Data)
	}
	imgRef := newNSImage(img.Pix, int(frame.Width), int(frame.Height), img.Rect.Dx(), img.Rect.Dy())
	defer Release(imgRef)
	dragItem := objc.ID(Cls("NSDraggingItem")).Send(Sel("alloc")).Send(Sel("initWithPasteboardWriter:"),
		objc.ID(item))
	dragItem.Send(Sel("setDraggingFrame:contents:"), NSRectFromRect(frame), imgRef)
	ov := objc.ID(v)
	ov.Send(Sel("setDragMask:"), uint64(DragOpFromUnison(dragOpMask)))
	ov.Send(Sel("setInDragWeStarted:"), true)
	ov.Send(Sel("beginDraggingSessionWithItems:event:source:"), NSArrayFromIDs(dragItem),
		ov.Send(Sel("lastMouseDraggedEvent")), ov)
}

// RegisterDraggedTypes registers the data types the view accepts in drag & drop operations.
func (v View) RegisterDraggedTypes(types []*uti.DataType) {
	WithPool(func() {
		ids := make([]objc.ID, 0, len(types))
		for _, dt := range types {
			ids = append(ids, NSStringFromGo(dt.UTI))
		}
		objc.ID(v).Send(Sel("registerForDraggedTypes:"), NSArrayFromIDs(ids...))
	})
}

// UnregisterDraggedTypes removes all registered drag & drop data types from the view.
func (v View) UnregisterDraggedTypes() {
	objc.ID(v).Send(Sel("unregisterDraggedTypes"))
}
