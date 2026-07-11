// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

// NSEventType values (verified against the macOS SDK's NSEvent.h).
const (
	nsEventTypeLeftMouseDown    uint64 = 1
	nsEventTypeLeftMouseUp      uint64 = 2
	nsEventTypeRightMouseDown   uint64 = 3
	nsEventTypeMouseMoved       uint64 = 5
	nsEventTypeLeftMouseDragged uint64 = 6
	nsEventTypeKeyDown          uint64 = 10
	nsEventTypeKeyUp            uint64 = 11
	nsEventTypeOtherMouseDown   uint64 = 25
)

// newTestWindowAndView builds the window+view pair the way unison's apiInit does. The returned cleanup mirrors
// apiDestroy (release the view, close the window), which also exercises the Go dealloc override.
func newTestWindowAndView(t *testing.T) (Window, View, func()) {
	t.Helper()
	w := newTestWindow(testTitledStyle, true, true)
	if w == 0 {
		t.Fatal("NewWindow returned 0")
	}
	v := NewView(w)
	if v == 0 {
		t.Fatal("NewView returned 0")
	}
	w.SetContentView(v)
	w.MakeFirstResponder(v)
	return w, v, func() {
		w.OrderOut()
		v.Release()
		w.Close()
	}
}

// synthMouseEvent returns an autoreleased NSEvent for a mouse event type; the caller must have a pool in place.
func synthMouseEvent(eventType uint64, where NSPoint, mods uint64, w Window) objc.ID {
	return objc.ID(Cls("NSEvent")).Send(
		Sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
		eventType, where, mods, float64(0), objc.Send[int64](objc.ID(w), Sel("windowNumber")), objc.ID(0),
		int64(0), int64(1), float32(1))
}

// synthKeyEvent returns an autoreleased NSEvent for a key event type; the caller must have a pool in place.
func synthKeyEvent(eventType uint64, chars string, keyCode uint16, mods uint64, w Window) objc.ID {
	return objc.ID(Cls("NSEvent")).Send(
		Sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
		eventType, NSPoint{}, mods, float64(0), objc.Send[int64](objc.ID(w), Sel("windowNumber")), objc.ID(0),
		NSStringFromGo(chars), NSStringFromGo(chars), false, keyCode)
}

// TestNewViewBasics proves the Go-registered macContentView class: creation, protocol conformance, the constant
// boolean overrides, isOpaque tracking the window, geometry, and the tracking-area maintenance cycle.
func TestNewViewBasics(t *testing.T) {
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		ov := objc.ID(v)
		WithPool(func() {
			if got := GoStringFromNSString(ov.Send(Sel("className"))); got != "macContentView" {
				t.Errorf("className = %q, want macContentView", got)
			}
		})
		if p := objc.GetProtocol("NSTextInputClient"); p == nil {
			t.Error("NSTextInputClient protocol not found in runtime")
		} else if !objc.Send[bool](ov, Sel("conformsToProtocol:"), unsafe.Pointer(p)) {
			t.Error("view does not conform to NSTextInputClient")
		}
		for _, sel := range []string{
			"canBecomeKeyView", "acceptsFirstResponder", "wantsUpdateLayer", "wantsPeriodicDraggingUpdates",
		} {
			if !objc.Send[bool](ov, Sel(sel)) {
				t.Errorf("%s = false, want true", sel)
			}
		}
		if !objc.Send[bool](ov, Sel("acceptsFirstMouse:"), objc.ID(0)) {
			t.Error("acceptsFirstMouse: = false, want true")
		}
		if objc.Send[bool](ov, Sel("ignoreModifierKeysForDraggingSession:"), objc.ID(0)) {
			t.Error("ignoreModifierKeysForDraggingSession: = true, want false")
		}
		if got := w.ContentView(); got != v {
			t.Errorf("ContentView() = %#x, want %#x", got, v)
		}

		// isOpaque must track the window's opacity, as the old implementation forwarded [wnd isOpaque].
		if !objc.Send[bool](ov, Sel("isOpaque")) {
			t.Error("view is not opaque while its window is opaque")
		}
		w.SetTransparent()
		if objc.Send[bool](ov, Sel("isOpaque")) {
			t.Error("view is still opaque after the window became transparent")
		}

		// Geometry: as the content view, the frame matches the window's content rect, and MouseInRect agrees with
		// obvious in/out points (window coordinates, bottom-left origin).
		frame := v.Frame()
		content := w.ContentRectForFrameRect(w.Frame())
		if frame.Size != content.Size {
			t.Errorf("view frame size %v != content rect size %v", frame.Size, content.Size)
		}
		if !v.MouseInRect(geom.NewPoint(10, 10), frame) {
			t.Error("MouseInRect() = false for an inside point")
		}
		if v.MouseInRect(geom.NewPoint(frame.Width+50, 10), frame) {
			t.Error("MouseInRect() = true for an outside point")
		}
		scale := v.BackingScale()
		if scale.X < 1 || scale.Y < 1 {
			t.Errorf("BackingScale() = %v, want components >= 1", scale)
		}

		// The tracking-area cycle: exactly one area owned by the view, surviving repeated updateTrackingAreas calls
		// (each of which must remove and release the prior area before installing a fresh one).
		for range 3 {
			ov.Send(Sel("updateTrackingAreas"))
		}
		var owned int
		var last objc.ID
		for _, ta := range IDsFromNSArray(ov.Send(Sel("trackingAreas"))) {
			if ta.Send(Sel("owner")) == ov {
				owned++
				last = ta
			}
		}
		if owned != 1 {
			t.Errorf("view owns %d tracking areas, want 1", owned)
		}
		if got := ov.Send(Sel("trackingArea")); got != last {
			t.Errorf("trackingArea ivar = %#x, want %#x", got, last)
		}
	})
}

// TestViewMouseAndKeyEvents drives the mouse and key overrides with synthesized NSEvents through real objc_msgSend
// dispatch and asserts the callbacks receive the window, flipped coordinates, modifiers, and key codes. The
// keyDown: path must also feed interpretKeyEvents:, which loops back through AppKit's text input system into our
// insertText:replacementRange: and from there to WindowKeyTypedCallback.
func TestViewMouseAndKeyEvents(t *testing.T) {
	defer func() {
		WindowMouseClickCallback = nil
		WindowMouseMovedCallback = nil
		WindowMouseEnterCallback = nil
		WindowMouseExitCallback = nil
		WindowCursorUpdateCallback = nil
		WindowKeyPressedCallback = nil
		WindowKeyReleasedCallback = nil
		WindowKeyTypedCallback = nil
	}()
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		ov := objc.ID(v)
		height := v.Frame().Height

		type click struct {
			w       Window
			where   geom.Point
			button  int
			pressed bool
			mods    uint
		}
		var clicks []click
		WindowMouseClickCallback = func(cbw Window, button int, where geom.Point, pressed bool, mods uint) {
			clicks = append(clicks, click{w: cbw, where: where, button: button, pressed: pressed, mods: mods})
		}
		WithPool(func() {
			loc := NSPoint{X: 50, Y: 60}
			ov.Send(Sel("mouseDown:"), synthMouseEvent(nsEventTypeLeftMouseDown, loc, uint64(EventModifierFlagShift), w))
			ov.Send(Sel("mouseUp:"), synthMouseEvent(nsEventTypeLeftMouseUp, loc, 0, w))
			ov.Send(Sel("rightMouseDown:"), synthMouseEvent(nsEventTypeRightMouseDown, loc, 0, w))
			otherEvent := synthMouseEvent(nsEventTypeOtherMouseDown, loc, 0, w)
			otherButton := int(objc.Send[int64](otherEvent, Sel("buttonNumber")))
			ov.Send(Sel("otherMouseDown:"), otherEvent)
			want := []click{
				{w: w, where: geom.NewPoint(50, float32(height)-60), button: 0, pressed: true, mods: uint(EventModifierFlagShift)},
				{w: w, where: geom.NewPoint(50, float32(height)-60), button: 0, pressed: false},
				{w: w, where: geom.NewPoint(50, float32(height)-60), button: 1, pressed: true},
				{w: w, where: geom.NewPoint(50, float32(height)-60), button: otherButton, pressed: true},
			}
			if len(clicks) != len(want) {
				t.Fatalf("got %d click callbacks, want %d", len(clicks), len(want))
			}
			for i, wc := range want {
				if clicks[i] != wc {
					t.Errorf("click %d = %+v, want %+v", i, clicks[i], wc)
				}
			}
		})

		// mouseMoved: plus the drag-forwarding contract: mouseDragged: must forward through mouseMoved: with the
		// event stashed in lastMouseDraggedEvent for the duration, and must do nothing while a drag we started is in
		// flight.
		var moved []geom.Point
		var stashedDuringMove objc.ID
		WindowMouseMovedCallback = func(_ Window, pt geom.Point, _ uint) {
			moved = append(moved, pt)
			stashedDuringMove = ov.Send(Sel("lastMouseDraggedEvent"))
		}
		WithPool(func() {
			ov.Send(Sel("mouseMoved:"), synthMouseEvent(nsEventTypeMouseMoved, NSPoint{X: 10, Y: 20}, 0, w))
			if len(moved) != 1 || moved[0] != geom.NewPoint(10, float32(height)-20) {
				t.Errorf("mouseMoved callbacks = %v, want one at (10,%v)", moved, height-20)
			}
			dragEvent := synthMouseEvent(nsEventTypeLeftMouseDragged, NSPoint{X: 11, Y: 21}, 0, w)
			ov.Send(Sel("mouseDragged:"), dragEvent)
			if len(moved) != 2 {
				t.Fatalf("mouseDragged did not forward to mouseMoved (%d callbacks)", len(moved))
			}
			if stashedDuringMove != dragEvent {
				t.Errorf("lastMouseDraggedEvent during drag-forwarded move = %#x, want %#x", stashedDuringMove, dragEvent)
			}
			if got := ov.Send(Sel("lastMouseDraggedEvent")); got != 0 {
				t.Errorf("lastMouseDraggedEvent = %#x after mouseDragged returned, want 0", got)
			}
			ov.Send(Sel("setInDragWeStarted:"), true)
			ov.Send(Sel("mouseDragged:"), dragEvent)
			ov.Send(Sel("setInDragWeStarted:"), false)
			if len(moved) != 2 {
				t.Error("mouseDragged forwarded to mouseMoved while a drag we started was in flight")
			}
		})

		// Enter/exit/cursor-update only need the location/modifiers (enter) or nothing (exit, cursor update).
		var entered []geom.Point
		var exited, cursorUpdated int
		WindowMouseEnterCallback = func(_ Window, pt geom.Point, _ uint) { entered = append(entered, pt) }
		WindowMouseExitCallback = func(Window) { exited++ }
		WindowCursorUpdateCallback = func(Window) { cursorUpdated++ }
		WithPool(func() {
			moveEvent := synthMouseEvent(nsEventTypeMouseMoved, NSPoint{X: 5, Y: 6}, 0, w)
			ov.Send(Sel("mouseEntered:"), moveEvent)
			ov.Send(Sel("mouseExited:"), moveEvent)
			ov.Send(Sel("cursorUpdate:"), moveEvent)
		})
		if len(entered) != 1 || entered[0] != geom.NewPoint(5, float32(height)-6) {
			t.Errorf("mouseEntered callbacks = %v, want one at (5,%v)", entered, height-6)
		}
		if exited != 1 || cursorUpdated != 1 {
			t.Errorf("exit/cursorUpdate callbacks = %d/%d, want 1/1", exited, cursorUpdated)
		}

		// Key events: keyDown must report the key and then run the event through interpretKeyEvents:, which loops
		// back through AppKit's text input system into insertText:replacementRange: and the typed callback.
		type key struct {
			code uint16
			mods uint
		}
		var pressed, released []key
		var typed []rune
		WindowKeyPressedCallback = func(_ Window, code uint16, mods uint) {
			pressed = append(pressed, key{code: code, mods: mods})
		}
		WindowKeyReleasedCallback = func(_ Window, code uint16, mods uint) {
			released = append(released, key{code: code, mods: mods})
		}
		WindowKeyTypedCallback = func(_ Window, ch rune) { typed = append(typed, ch) }
		w.MakeKeyAndOrderFront()
		WithPool(func() {
			ov.Send(Sel("keyDown:"), synthKeyEvent(nsEventTypeKeyDown, "a", 0, 0, w))
			ov.Send(Sel("keyUp:"), synthKeyEvent(nsEventTypeKeyUp, "a", 0, 0, w))
		})
		if len(pressed) != 1 || pressed[0].code != 0 {
			t.Errorf("key pressed callbacks = %v, want one with code 0", pressed)
		}
		if len(released) != 1 || released[0].code != 0 {
			t.Errorf("key released callbacks = %v, want one with code 0", released)
		}
		if len(typed) != 1 || typed[0] != 'a' {
			t.Errorf("key typed callbacks = %q, want %q", string(typed), "a")
		}
	})
}

// TestViewTextInputClient exercises the NSTextInputClient methods, including the struct-return paths (markedRange,
// firstRectForCharacterRange:actualRange:) and the two-NSRange setMarkedText:selectedRange:replacementRange:, which
// is driven through an NSInvocation so Foundation's compiled marshaling performs the call the way AppKit's IME
// machinery would (and to sidestep the purego amd64 caller-side struct-straddle bug documented in objc_darwin.go).
func TestViewTextInputClient(t *testing.T) {
	defer func() { WindowKeyTypedCallback = nil }()
	runOnMain(func() {
		_, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		ov := objc.ID(v)

		if objc.Send[bool](ov, Sel("hasMarkedText")) {
			t.Error("hasMarkedText = true for a fresh view")
		}
		if got := objc.Send[NSRange](ov, Sel("markedRange")); got != emptyRange {
			t.Errorf("markedRange = %+v, want %+v", got, emptyRange)
		}
		if got := objc.Send[NSRange](ov, Sel("selectedRange")); got != emptyRange {
			t.Errorf("selectedRange = %+v, want %+v", got, emptyRange)
		}

		setMarkedText := func(text objc.ID) {
			sig := objc.ID(Cls("NSMethodSignature")).Send(Sel("signatureWithObjCTypes:"),
				"v@:@{_NSRange=QQ}{_NSRange=QQ}")
			inv := objc.ID(Cls("NSInvocation")).Send(Sel("invocationWithMethodSignature:"), sig)
			inv.Send(Sel("setTarget:"), ov)
			inv.Send(Sel("setSelector:"), Sel("setMarkedText:selectedRange:replacementRange:"))
			selRange := NSRange{Location: 0, Length: 0}
			replRange := emptyRange
			inv.Send(Sel("setArgument:atIndex:"), unsafe.Pointer(&text), int64(2))
			inv.Send(Sel("setArgument:atIndex:"), unsafe.Pointer(&selRange), int64(3))
			inv.Send(Sel("setArgument:atIndex:"), unsafe.Pointer(&replRange), int64(4))
			inv.Send(Sel("invoke"))
			runtime.KeepAlive(&text)
			runtime.KeepAlive(&selRange)
			runtime.KeepAlive(&replRange)
		}

		WithPool(func() {
			// Plain NSString marked text (the "isKindOfClass: NSAttributedString" == false branch).
			setMarkedText(NSStringFromGo("abc"))
			if !objc.Send[bool](ov, Sel("hasMarkedText")) {
				t.Error("hasMarkedText = false after setMarkedText")
			}
			// The old implementation reported {0, length-1}; preserved verbatim.
			if got, want := objc.Send[NSRange](ov, Sel("markedRange")), (NSRange{Location: 0, Length: 2}); got != want {
				t.Errorf("markedRange = %+v, want %+v", got, want)
			}
			// NSAttributedString marked text (the other branch), with a different length.
			attr := objc.ID(Cls("NSAttributedString")).Send(Sel("alloc")).
				Send(Sel("initWithString:"), NSStringFromGo("日本語入力"))
			setMarkedText(attr)
			Release(attr)
			if got, want := objc.Send[NSRange](ov, Sel("markedRange")), (NSRange{Location: 0, Length: 4}); got != want {
				t.Errorf("markedRange after attributed setMarkedText = %+v, want %+v", got, want)
			}
			ov.Send(Sel("unmarkText"))
			if objc.Send[bool](ov, Sel("hasMarkedText")) {
				t.Error("hasMarkedText = true after unmarkText")
			}
			if got := objc.Send[NSRange](ov, Sel("markedRange")); got != emptyRange {
				t.Errorf("markedRange after unmarkText = %+v, want %+v", got, emptyRange)
			}

			// insertText:replacementRange: forwards each rune, skipping the function-key range (0xF700-0xF7FF).
			var typed []rune
			WindowKeyTypedCallback = func(_ Window, ch rune) { typed = append(typed, ch) }
			ov.Send(Sel("insertText:replacementRange:"), NSStringFromGo("abé漢"), emptyRange)
			if got, want := string(typed), "abé漢"; got != want {
				t.Errorf("typed %q, want %q", got, want)
			}
			// The attributed-string variant must extract the plain string.
			typed = nil
			attr = objc.ID(Cls("NSAttributedString")).Send(Sel("alloc")).
				Send(Sel("initWithString:"), NSStringFromGo("xy"))
			ov.Send(Sel("insertText:replacementRange:"), attr, emptyRange)
			Release(attr)
			if got, want := string(typed), "xy"; got != want {
				t.Errorf("typed %q from attributed string, want %q", got, want)
			}

			// The remaining protocol methods have fixed answers.
			var actual NSRange
			frame := objc.Send[NSRect](ov, Sel("frame"))
			got := objc.Send[NSRect](ov, Sel("firstRectForCharacterRange:actualRange:"),
				NSRange{Location: 0, Length: 1}, &actual)
			if want := (NSRect{Origin: frame.Origin}); got != want {
				t.Errorf("firstRectForCharacterRange = %+v, want %+v", got, want)
			}
			if got := objc.Send[uint64](ov, Sel("characterIndexForPoint:"), NSPoint{X: 5, Y: 5}); got != 0 {
				t.Errorf("characterIndexForPoint = %d, want 0", got)
			}
			if got := ov.Send(Sel("attributedSubstringForProposedRange:actualRange:"),
				NSRange{Location: 0, Length: 1}, &actual); got != 0 {
				t.Errorf("attributedSubstringForProposedRange = %#x, want nil", got)
			}
			if got := NSArrayCount(ov.Send(Sel("validAttributesForMarkedText"))); got != 0 {
				t.Errorf("validAttributesForMarkedText count = %d, want 0", got)
			}
			ov.Send(Sel("doCommandBySelector:"), Sel("moveLeft:")) // must be a no-op, not an exception
		})
	})
}

var (
	testDragInfoClassOnce sync.Once
	testDragInfoClass     objc.Class
	testDragLocation      NSPoint
	testDragSourceMask    uint64
	testDragPasteboard    objc.ID
)

// testDragInfo returns an owned instance of a minimal NSDraggingInfo stand-in whose draggingLocation,
// draggingSourceOperationMask, and draggingPasteboard answers come from the vars above. Since the DragInfo methods
// send real Objective-C messages to the sender, this stand-in serves both the view's destination overrides and the
// DragInfo accessor tests in drag_darwin_test.go.
func testDragInfo(t *testing.T) objc.ID {
	t.Helper()
	testDragInfoClassOnce.Do(func() {
		cls, err := objc.RegisterClass("unisonTestDraggingInfo", Cls("NSObject"), nil, nil, []objc.MethodDef{
			{
				Cmd: Sel("draggingLocation"),
				Fn:  func(_ objc.ID, _ objc.SEL) NSPoint { return testDragLocation },
			},
			{
				Cmd: Sel("draggingSourceOperationMask"),
				Fn:  func(_ objc.ID, _ objc.SEL) uint64 { return testDragSourceMask },
			},
			{
				Cmd: Sel("draggingPasteboard"),
				Fn:  func(_ objc.ID, _ objc.SEL) objc.ID { return testDragPasteboard },
			},
		})
		if err != nil {
			t.Fatalf("unable to register test dragging info class: %v", err)
		}
		testDragInfoClass = cls
	})
	return objc.ID(testDragInfoClass).Send(Sel("new"))
}

// TestViewDragAndDrop exercises the dragging-destination overrides (enter/update/drop/exit), including the
// source-mask intersection the old export shims performed, and the dragging-source methods backed by the dragMask
// and inDragWeStarted ivars.
func TestViewDragAndDrop(t *testing.T) {
	defer func() {
		WindowDragEnterCallback = nil
		WindowDragUpdateCallback = nil
		WindowDropCallback = nil
		WindowDragExitCallback = nil
		WindowDragSourceFinishedCallback = nil
	}()
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		ov := objc.ID(v)
		height := v.Frame().Height
		info := testDragInfo(t)
		defer Release(info)
		testDragLocation = NSPoint{X: 30, Y: 40}
		testDragSourceMask = uint64(DragOpCopy)

		// With no callback installed, the destination reports none.
		if got := objc.Send[uint64](ov, Sel("draggingEntered:"), info); got != uint64(DragOpNone) {
			t.Errorf("draggingEntered with no callback = %d, want %d", got, uint64(DragOpNone))
		}

		// The callback's answer must be intersected with the source's operation mask: copy+move offered, only copy
		// allowed by the source.
		var enterWnd Window
		var enterInfo DragInfo
		var enterWhere geom.Point
		WindowDragEnterCallback = func(cbw Window, d DragInfo, where geom.Point, _ uint) drag.Op {
			enterWnd = cbw
			enterInfo = d
			enterWhere = where
			return drag.Copy | drag.Move
		}
		if got := objc.Send[uint64](ov, Sel("draggingEntered:"), info); got != uint64(DragOpCopy) {
			t.Errorf("draggingEntered = %d, want %d (copy masked by source)", got, uint64(DragOpCopy))
		}
		if enterWnd != w || enterInfo != DragInfo(info) {
			t.Errorf("drag enter callback got window %#x info %#x, want %#x %#x", enterWnd, enterInfo, w, info)
		}
		if want := geom.NewPoint(30, float32(height)-40); enterWhere != want {
			t.Errorf("drag enter location = %v, want %v", enterWhere, want)
		}

		// A move-only answer is filtered out entirely by a copy-only source.
		WindowDragUpdateCallback = func(_ Window, _ DragInfo, _ geom.Point, _ uint) drag.Op { return drag.Move }
		if got := objc.Send[uint64](ov, Sel("draggingUpdated:"), info); got != uint64(DragOpNone) {
			t.Errorf("draggingUpdated = %d, want %d (move filtered by copy-only source)", got, uint64(DragOpNone))
		}

		var dropWhere geom.Point
		WindowDropCallback = func(_ Window, _ DragInfo, where geom.Point, _ uint) bool {
			dropWhere = where
			return true
		}
		if !objc.Send[bool](ov, Sel("performDragOperation:"), info) {
			t.Error("performDragOperation = false, want callback's true")
		}
		if want := geom.NewPoint(30, float32(height)-40); dropWhere != want {
			t.Errorf("drop location = %v, want %v", dropWhere, want)
		}

		var exited int
		WindowDragExitCallback = func(Window) { exited++ }
		ov.Send(Sel("draggingExited:"), info)
		if exited != 1 {
			t.Errorf("drag exit callbacks = %d, want 1", exited)
		}

		// Dragging source: the session operation mask comes from the dragMask ivar, and ending the session reports
		// the finish and clears inDragWeStarted.
		ov.Send(Sel("setDragMask:"), uint64(DragOpMove))
		if got := objc.Send[uint64](ov, Sel("draggingSession:sourceOperationMaskForDraggingContext:"),
			objc.ID(0), int64(0)); got != uint64(DragOpMove) {
			t.Errorf("sourceOperationMaskForDraggingContext = %d, want %d", got, uint64(DragOpMove))
		}
		var finished int
		WindowDragSourceFinishedCallback = func(Window) { finished++ }
		ov.Send(Sel("setInDragWeStarted:"), true)
		ov.Send(Sel("draggingSession:endedAtPoint:operation:"), objc.ID(0), NSPoint{X: 1, Y: 2}, uint64(0))
		if finished != 1 {
			t.Errorf("drag source finished callbacks = %d, want 1", finished)
		}
		if objc.Send[bool](ov, Sel("isInDragWeStarted")) {
			t.Error("inDragWeStarted still set after the drag session ended")
		}
	})
}

// TestViewRegisteredDragTypes proves RegisterDraggedTypes/UnregisterDraggedTypes against AppKit's own
// registeredDraggedTypes bookkeeping.
func TestViewRegisteredDragTypes(t *testing.T) {
	runOnMain(func() {
		_, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		v.RegisterDraggedTypes([]*uti.DataType{{UTI: "public.utf8-plain-text"}, {UTI: "public.file-url"}})
		WithPool(func() {
			registered := make(map[string]bool)
			for _, id := range IDsFromNSArray(objc.ID(v).Send(Sel("registeredDraggedTypes"))) {
				registered[GoStringFromNSString(id)] = true
			}
			if !registered["public.utf8-plain-text"] || !registered["public.file-url"] {
				t.Errorf("registeredDraggedTypes = %v, want both registered UTIs present", registered)
			}
		})
		v.UnregisterDraggedTypes()
		if got := NSArrayCount(objc.ID(v).Send(Sel("registeredDraggedTypes"))); got != 0 {
			t.Errorf("registeredDraggedTypes count = %d after unregister, want 0", got)
		}
	})
}

// TestViewDrawAndBackingCallbacks proves the drawing callbacks and the backing-scale notification. AppKit-initiated
// drawing is driven through displayRectIgnoringOpacity:inContext: with a bitmap-backed context, which routes to
// updateLayer for a wantsUpdateLayer view — a plain [view display] never invokes drawRect:/updateLayer for a
// non-layer-backed wantsUpdateLayer view (verified identical for the old compiled Objective-C macContentView shape,
// so this is pre-existing AppKit behavior, not a port difference). drawRect: is additionally dispatched directly to
// prove the NSRect-argument Go IMP.
func TestViewDrawAndBackingCallbacks(t *testing.T) {
	defer func() {
		WindowRedrawCallback = nil
		WindowUpdateLayerCallback = nil
		WindowScaleCallback = nil
	}()
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		var redrawn, layerUpdated int
		WindowRedrawCallback = func(Window) { redrawn++ }
		WindowUpdateLayerCallback = func(Window) { layerUpdated++ }
		WithPool(func() {
			w.MakeKeyAndOrderFront()
			// A premultiplied-alpha bitmap rep is required: graphicsContextWithBitmapImageRep: returns nil for
			// non-premultiplied formats.
			rep := objc.ID(Cls("NSBitmapImageRep")).Send(Sel("alloc")).Send(
				Sel("initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:hasAlpha:isPlanar:colorSpaceName:bitmapFormat:bytesPerRow:bitsPerPixel:"),
				unsafe.Pointer(nil), 64, 64, 8, 4, true, false,
				NSStringConstant("AppKit", "NSCalibratedRGBColorSpace"), 0, 64*4, 32)
			defer Release(rep)
			ctx := objc.ID(Cls("NSGraphicsContext")).Send(Sel("graphicsContextWithBitmapImageRep:"), rep)
			if ctx == 0 {
				t.Fatal("unable to create bitmap-backed NSGraphicsContext")
			}
			objc.ID(v).Send(Sel("displayRectIgnoringOpacity:inContext:"),
				NSRect{Size: NSSize{Width: 64, Height: 64}}, ctx)
		})
		if layerUpdated == 0 {
			t.Error("updateLayer did not fire for displayRectIgnoringOpacity:inContext:")
		}
		objc.ID(v).Send(Sel("drawRect:"), NSRect{Size: NSSize{Width: 10, Height: 10}})
		if redrawn != 1 {
			t.Errorf("drawRect: dispatch produced %d redraw callbacks, want 1", redrawn)
		}

		var scales []geom.Point
		WindowScaleCallback = func(_ Window, scale geom.Point) { scales = append(scales, scale) }
		objc.ID(v).Send(Sel("viewDidChangeBackingProperties"))
		if len(scales) != 1 || scales[0] != v.BackingScale() {
			t.Errorf("scale callbacks = %v, want one equal to %v", scales, v.BackingScale())
		}
	})
}

// TestViewReleaseDealloc proves the Go dealloc override runs without crashing for a view that was never installed in
// a window (the installed case is exercised by every other test's cleanup, which mirrors apiDestroy).
func TestViewReleaseDealloc(t *testing.T) {
	runOnMain(func() {
		w := newTestWindow(testTitledStyle, true, true)
		if w == 0 {
			t.Fatal("NewWindow returned 0")
		}
		defer w.Close()
		v := NewView(w)
		if v == 0 {
			t.Fatal("NewView returned 0")
		}
		v.Release() // drops the only reference: dealloc must release the tracking area and marked text
	})
}
