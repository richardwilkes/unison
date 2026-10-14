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
	"reflect"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// Window is a handle to a macWindow instance, an NSWindow subclass whose key/main window eligibility is fixed at
// creation time. NewWindow returns an owned (+1) reference; matching the cgo bridge, no Release is exposed — AppKit's
// default releasedWhenClosed behavior drops the reference when Close is called, after which the handle is invalid.
type Window objc.ID

// WindowStyleMask holds constants that specify the style of a window.
type WindowStyleMask = uint64

// Some WindowStyleMask values.
const (
	WindowStyleMaskTitled WindowStyleMask = 1 << iota
	WindowStyleMaskClosable
	WindowStyleMaskMiniaturizable
	WindowStyleMaskResizable
	WindowStyleMaskBorderless WindowStyleMask = 0
)

// WindowCollectionBehavior holds window collection behaviors related to Mission Control, Spaces, and Stage Manager.
type WindowCollectionBehavior = uint64

const (
	// WindowCollectionBehaviorCanJoinAllSpaces indicates the window appears in every space.
	WindowCollectionBehaviorCanJoinAllSpaces WindowCollectionBehavior = 1 << 0
	// WindowCollectionBehaviorManaged indicates the window participates in Mission Control and Spaces.
	WindowCollectionBehaviorManaged WindowCollectionBehavior = 1 << 2
	// WindowCollectionBehaviorTransient indicates the window floats in Spaces and is hidden by Mission Control.
	WindowCollectionBehaviorTransient WindowCollectionBehavior = 1 << 3
	// WindowCollectionBehaviorIgnoresCycle indicates the window is left out of the window cycle (Command-`).
	WindowCollectionBehaviorIgnoresCycle WindowCollectionBehavior = 1 << 6
	// WindowCollectionBehaviorFullScreenPrimary indicates the window can enter full-screen mode.
	WindowCollectionBehaviorFullScreenPrimary WindowCollectionBehavior = 1 << 7
	// WindowCollectionBehaviorFullScreenAuxiliary indicates the window can be shown in the same space as a full-screen
	// window.
	WindowCollectionBehaviorFullScreenAuxiliary WindowCollectionBehavior = 1 << 8
	// WindowCollectionBehaviorFullScreenNone indicates the window doesn’t support full-screen mode.
	WindowCollectionBehaviorFullScreenNone WindowCollectionBehavior = 1 << 9
)

// WindowAnimationBehavior holds the animation AppKit runs when a window is ordered in or out.
type WindowAnimationBehavior = int64

// WindowAnimationBehaviorNone suppresses the window's show and hide animations.
const WindowAnimationBehaviorNone WindowAnimationBehavior = 2

// WindowLevel holds the standard window levels in macOS.
type WindowLevel = int64

// Some WindowLevel values.
const (
	WindowLevelNormal    WindowLevel = 0
	WindowLevelFloating  WindowLevel = 3
	WindowLevelPopUpMenu WindowLevel = 101
)

// WindowTabbingMode holds the preferred tabbing behavior of a window.
type WindowTabbingMode = int64

// Some WindowTabbingMode values.
const (
	WindowTabbingModeAutomatic WindowTabbingMode = iota
	WindowTabbingModePreferred
	WindowTabbingModeDisallowed
)

// nsBackingStoreBuffered is NSBackingStoreType's NSBackingStoreBuffered.
const nsBackingStoreBuffered uint64 = 2

// NSEventType values of the mouse-down events (verified against the macOS SDK's NSEvent.h).
const (
	nsEventTypeLeftMouseDown  uint64 = 1
	nsEventTypeRightMouseDown uint64 = 3
	nsEventTypeOtherMouseDown uint64 = 25
)

// WindowFrameMouseDownCallback is invoked when a mouse button is pressed on a window outside its content view: on its
// title bar or its frame. The press goes on to AppKit afterwards, so a window drag or a title bar button still works.
var WindowFrameMouseDownCallback func(Window)

var (
	macWindowClassOnce sync.Once
	macWindowClass     objc.Class
	macWindowClassErr  error
)

// windowFrameMouseDown reports whether event is a mouse button press on wnd outside its content view.
func windowFrameMouseDown(wnd, event objc.ID) bool {
	switch objc.Send[uint64](event, Sel("type")) {
	case nsEventTypeLeftMouseDown, nsEventTypeRightMouseDown, nsEventTypeOtherMouseDown:
	default:
		return false
	}
	content := wnd.Send(Sel("contentView"))
	if content == 0 {
		return false
	}
	pt := objc.Send[NSPoint](event, Sel("locationInWindow"))
	frame := objc.Send[NSRect](content, Sel("frame"))
	return pt.X < frame.Origin.X || pt.Y < frame.Origin.Y || pt.X >= frame.Origin.X+frame.Size.Width ||
		pt.Y >= frame.Origin.Y+frame.Size.Height
}

// registerMacWindowClass registers the macWindow Objective-C class: NSWindow plus two bool ivars set at creation
// time and read back by the canBecomeKeyWindow/canBecomeMainWindow overrides (through the accessors objc.RegisterClass
// generates for ReadWrite fields), and a third, set by SetAccessibilityHidden, that the accessibility overrides
// consult. Registration is process-global and can only happen once per class name, so it is guarded by
// macWindowClassOnce.
func registerMacWindowClass() {
	LoadAppKit()
	cls, err := objc.RegisterClass("macWindow", Cls("NSWindow"), nil, []objc.FieldDef{
		{Name: "canBeKeyWindow", Type: reflect.TypeFor[bool](), Attribute: objc.ReadWrite},
		{Name: "canBeMainWindow", Type: reflect.TypeFor[bool](), Attribute: objc.ReadWrite},
		{Name: "axHidden", Type: reflect.TypeFor[bool](), Attribute: objc.ReadWrite},
	}, []objc.MethodDef{
		{
			Cmd: Sel("canBecomeKeyWindow"),
			Fn: func(self objc.ID, _ objc.SEL) bool {
				return objc.Send[bool](self, Sel("isCanBeKeyWindow"))
			},
		},
		{
			Cmd: Sel("canBecomeMainWindow"),
			Fn: func(self objc.ID, _ objc.SEL) bool {
				return objc.Send[bool](self, Sel("isCanBeMainWindow"))
			},
		},
		{
			Cmd: Sel("isAccessibilityElement"),
			Fn: func(self objc.ID, _ objc.SEL) bool {
				if objc.Send[bool](self, Sel("isAxHidden")) {
					return false
				}
				// A BOOL comes back in the low byte of the return register only; see axSuperAllowsSelector.
				return byte(SendSuper(self, macWindowClass, Sel("isAccessibilityElement"))) != 0
			},
		},
		{
			Cmd: Sel("accessibilityChildren"),
			Fn: func(self objc.ID, _ objc.SEL) objc.ID {
				if objc.Send[bool](self, Sel("isAxHidden")) {
					return NSArrayFromIDs()
				}
				return SendSuper(self, macWindowClass, Sel("accessibilityChildren"))
			},
		},
		{
			Cmd: Sel("accessibilityHitTest:"),
			Fn: func(self objc.ID, _ objc.SEL, pt NSPoint) objc.ID {
				if objc.Send[bool](self, Sel("isAxHidden")) {
					return self.Send(Sel("contentView")).Send(Sel("accessibilityHitTest:"), pt)
				}
				return SendSuper(self, macWindowClass, Sel("accessibilityHitTest:"), pt)
			},
		},
		{
			// A press on the title bar or the frame never reaches the content view, so it is reported from here before
			// AppKit handles it.
			Cmd: Sel("sendEvent:"),
			Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
				if WindowFrameMouseDownCallback != nil && windowFrameMouseDown(self, event) {
					WindowFrameMouseDownCallback(Window(self))
				}
				SendSuper(self, macWindowClass, Sel("sendEvent:"), event)
			},
		},
	})
	if err != nil {
		macWindowClassErr = errs.NewWithCause("NewWindow: unable to register window class", err)
		return
	}
	macWindowClass = cls
}

// NewWindow returns a new macWindow with the given content rect (in bottom-left-origin global screen coordinates)
// and style, or 0 if the window could not be created. The window ivars are set immediately after init, matching the
// ordering of the old Objective-C initializer (super's init also ran before the flags were assigned there).
func NewWindow(contentRect geom.Rect, styleMask WindowStyleMask, canBeKey, canBeMain bool) Window {
	macWindowClassOnce.Do(registerMacWindowClass)
	if macWindowClassErr != nil {
		errs.Log(macWindowClassErr)
		return 0
	}
	wnd := objc.ID(macWindowClass).Send(Sel("alloc")).Send(Sel("initWithContentRect:styleMask:backing:defer:"),
		NSRectFromRect(contentRect), styleMask, nsBackingStoreBuffered, false)
	if wnd == 0 {
		return 0
	}
	wnd.Send(Sel("setCanBeKeyWindow:"), canBeKey)
	wnd.Send(Sel("setCanBeMainWindow:"), canBeMain)
	return Window(wnd)
}

// NewPopupWindow returns a new borderless macWindow set up for one level of a popup menu, or 0 if the window could not
// be created: it never becomes key or main, sits at the pop-up menu level in every space and beside full-screen
// windows, stays out of the window cycle, shows and hides without animation, casts a shadow and is hidden from
// assistive technologies (see SetAccessibilityHidden).
func NewPopupWindow(contentRect geom.Rect) Window {
	w := NewWindow(contentRect, WindowStyleMaskBorderless, false, false)
	if w == 0 {
		return 0
	}
	w.SetLevel(WindowLevelPopUpMenu)
	w.SetCollectionBehavior(WindowCollectionBehaviorFullScreenAuxiliary | WindowCollectionBehaviorCanJoinAllSpaces |
		WindowCollectionBehaviorTransient | WindowCollectionBehaviorIgnoresCycle)
	w.SetAnimationBehavior(WindowAnimationBehaviorNone)
	w.SetHasShadow(true)
	w.SetAccessibilityHidden(true)
	w.SetRestorable(false)
	w.SetTabbingMode(WindowTabbingModeDisallowed)
	return w
}

// SetCollectionBehavior sets the window's collection behavior.
func (w Window) SetCollectionBehavior(behavior WindowCollectionBehavior) {
	objc.ID(w).Send(Sel("setCollectionBehavior:"), behavior)
}

// SetLevel sets the window's level.
func (w Window) SetLevel(level WindowLevel) {
	objc.ID(w).Send(Sel("setLevel:"), level)
}

// SetAnimationBehavior sets the animation AppKit runs when the window is ordered in or out.
func (w Window) SetAnimationBehavior(behavior WindowAnimationBehavior) {
	objc.ID(w).Send(Sel("setAnimationBehavior:"), behavior)
}

// SetHasShadow sets whether the window casts a shadow.
func (w Window) SetHasShadow(hasShadow bool) {
	objc.ID(w).Send(Sel("setHasShadow:"), hasShadow)
}

// SetAccessibilityHidden sets whether the window is hidden from assistive technologies. A hidden window reports that it
// is not an accessibility element and has no children, and hands hit tests to its content view, so that a window
// whose content another window describes (see SetAXViewAlias) is never presented as a window of its own.
func (w Window) SetAccessibilityHidden(hidden bool) {
	objc.ID(w).Send(Sel("setAxHidden:"), hidden)
}

// StyleMask returns the window's style mask.
func (w Window) StyleMask() WindowStyleMask {
	return objc.Send[WindowStyleMask](objc.ID(w), Sel("styleMask"))
}

// SetTransparent makes the window non-opaque, shadowless, and clear-backed so that transparent content shows what is
// behind the window.
func (w Window) SetTransparent() {
	wnd := objc.ID(w)
	wnd.Send(Sel("setOpaque:"), false)
	wnd.Send(Sel("setHasShadow:"), false)
	wnd.Send(Sel("setBackgroundColor:"), objc.ID(Cls("NSColor")).Send(Sel("clearColor")))
}

// SetTitle sets the window's title.
func (w Window) SetTitle(title string) {
	str := NewNSString(title)
	objc.ID(w).Send(Sel("setTitle:"), str)
	Release(str)
}

// ContentView returns the window's content view.
func (w Window) ContentView() View {
	return View(objc.ID(w).Send(Sel("contentView")))
}

// SetContentView sets the window's content view.
func (w Window) SetContentView(v View) {
	objc.ID(w).Send(Sel("setContentView:"), objc.ID(v))
}

// SetRestorable sets whether the window should be restored at relaunch by the system.
func (w Window) SetRestorable(restorable bool) {
	objc.ID(w).Send(Sel("setRestorable:"), restorable)
}

// MakeFirstResponder makes the given view the window's first responder.
func (w Window) MakeFirstResponder(v View) {
	objc.ID(w).Send(Sel("makeFirstResponder:"), objc.ID(v))
}

// SetTabbingMode sets the window's tabbing mode.
func (w Window) SetTabbingMode(mode WindowTabbingMode) {
	objc.ID(w).Send(Sel("setTabbingMode:"), mode)
}

// SetAcceptsMouseMovedEvents sets whether the window receives mouse-moved events.
func (w Window) SetAcceptsMouseMovedEvents(accepts bool) {
	objc.ID(w).Send(Sel("setAcceptsMouseMovedEvents:"), accepts)
}

// Number returns the window's number, which identifies it to the window server.
func (w Window) Number() int {
	return int(objc.Send[int64](objc.ID(w), Sel("windowNumber")))
}

// WindowNumberAtPoint returns the number of the window, of any application, that is frontmost at pt, in screen
// coordinates with AppKit's bottom-left origin, or 0 when there is none. A window that ignores mouse events is passed
// over, as a press there would pass over it.
func WindowNumberAtPoint(pt geom.Point) int {
	return int(objc.Send[int64](objc.ID(Cls("NSWindow")), Sel("windowNumberAtPoint:belowWindowWithWindowNumber:"),
		NSPointFromPoint(pt), int64(0)))
}

// MouseLocationOutsideOfEventStream returns the current mouse position in the window's coordinate system.
func (w Window) MouseLocationOutsideOfEventStream() geom.Point {
	return PointFromNSPoint(objc.Send[NSPoint](objc.ID(w), Sel("mouseLocationOutsideOfEventStream")))
}

// MakeKeyAndOrderFront shows the window, moves it to the front, and makes it the key window.
func (w Window) MakeKeyAndOrderFront() {
	objc.ID(w).Send(Sel("makeKeyAndOrderFront:"), objc.ID(0))
}

// OrderFront shows the window and moves it to the front without making it the key window.
func (w Window) OrderFront() {
	objc.ID(w).Send(Sel("orderFront:"), objc.ID(0))
}

// OrderOut hides the window.
func (w Window) OrderOut() {
	objc.ID(w).Send(Sel("orderOut:"), objc.ID(0))
}

// Delegate returns the window's delegate.
func (w Window) Delegate() WindowDelegate {
	return WindowDelegate(objc.ID(w).Send(Sel("delegate")))
}

// SetDelegate sets the window's delegate.
func (w Window) SetDelegate(delegate WindowDelegate) {
	objc.ID(w).Send(Sel("setDelegate:"), objc.ID(delegate))
}

// Focused returns true if the window is the key window.
func (w Window) Focused() bool {
	return objc.Send[bool](objc.ID(w), Sel("isKeyWindow"))
}

// Miniaturized returns true if the window is currently miniaturized into the Dock.
func (w Window) Miniaturized() bool {
	return objc.Send[bool](objc.ID(w), Sel("isMiniaturized"))
}

// Miniaturize toggles the window's miniaturized state.
func (w Window) Miniaturize() {
	if w.Miniaturized() {
		objc.ID(w).Send(Sel("deminiaturize:"), objc.ID(0))
	} else {
		objc.ID(w).Send(Sel("miniaturize:"), objc.ID(0))
	}
}

// Zoomed returns true if the window is currently zoomed.
func (w Window) Zoomed() bool {
	return objc.Send[bool](objc.ID(w), Sel("isZoomed"))
}

// Zoom toggles the window's zoomed state.
func (w Window) Zoom() {
	objc.ID(w).Send(Sel("zoom:"), objc.ID(0))
}

// Frame returns the window's frame rect in bottom-left-origin global screen coordinates.
func (w Window) Frame() geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(w), Sel("frame")))
}

// SetFrame sets the window's frame rect and redisplays it.
func (w Window) SetFrame(frameRect geom.Rect) {
	objc.ID(w).Send(Sel("setFrame:display:"), NSRectFromRect(frameRect), true)
}

// ContentRectForFrameRect returns the content rect for the given frame rect.
func (w Window) ContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(w), Sel("contentRectForFrameRect:"), NSRectFromRect(frameRect)))
}

// FrameRectForContentRect returns the frame rect for the given content rect.
func (w Window) FrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	return RectFromNSRect(objc.Send[NSRect](objc.ID(w), Sel("frameRectForContentRect:"), NSRectFromRect(contentRect)))
}

// Visible returns true if the window is on screen (even if obscured or on another space).
func (w Window) Visible() bool {
	return objc.Send[bool](objc.ID(w), Sel("isVisible"))
}

// Close closes the window. Since AppKit windows release themselves when closed, the handle must not be used again.
func (w Window) Close() {
	objc.ID(w).Send(Sel("close"))
}
