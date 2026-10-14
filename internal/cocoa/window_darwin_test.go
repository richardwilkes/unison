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
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// newTestWindow creates a window the way unison's nativeInit does (titled unless undecorated), on the caller's
// (main-thread) goroutine. Production always creates the shared application before any window, so the tests do too.
func newTestWindow(styleMask WindowStyleMask, canBeKey, canBeMain bool) Window {
	sharedApp()
	return NewWindow(geom.NewRect(120, 120, 320, 240), styleMask, canBeKey, canBeMain)
}

const testTitledStyle = WindowStyleMaskTitled | WindowStyleMaskClosable | WindowStyleMaskMiniaturizable |
	WindowStyleMaskResizable

// TestNewWindowKeyMainFlags proves the Go-registered macWindow class: its canBecomeKeyWindow/canBecomeMainWindow
// overrides must report exactly the flags fixed at creation, overriding NSWindow's own style-based answers in both
// directions (a titled NSWindow defaults to YES for both).
func TestNewWindowKeyMainFlags(t *testing.T) {
	runOnMain(func() {
		for _, c := range []struct {
			canBeKey  bool
			canBeMain bool
		}{{true, true}, {false, false}, {true, false}, {false, true}} {
			w := newTestWindow(testTitledStyle, c.canBeKey, c.canBeMain)
			if w == 0 {
				t.Fatalf("NewWindow(%v, %v) returned 0", c.canBeKey, c.canBeMain)
			}
			if got := objc.Send[bool](objc.ID(w), Sel("canBecomeKeyWindow")); got != c.canBeKey {
				t.Errorf("canBecomeKeyWindow = %v, want %v", got, c.canBeKey)
			}
			if got := objc.Send[bool](objc.ID(w), Sel("canBecomeMainWindow")); got != c.canBeMain {
				t.Errorf("canBecomeMainWindow = %v, want %v", got, c.canBeMain)
			}
			if got := w.StyleMask(); got != testTitledStyle {
				t.Errorf("StyleMask() = %#x, want %#x", got, testTitledStyle)
			}
			w.Close()
		}
		// A borderless window must be creatable too (undecorated unison windows use this).
		w := newTestWindow(WindowStyleMaskBorderless|WindowStyleMaskMiniaturizable, true, true)
		if w == 0 {
			t.Fatal("NewWindow(borderless) returned 0")
		}
		if got := objc.Send[bool](objc.ID(w), Sel("canBecomeKeyWindow")); !got {
			t.Error("borderless canBecomeKeyWindow = false, want true (override must beat NSWindow's default NO)")
		}
		w.Close()
	})
}

func TestWindowTitleAndTransparency(t *testing.T) {
	runOnMain(func() {
		w := newTestWindow(testTitledStyle, true, true)
		if w == 0 {
			t.Fatal("NewWindow returned 0")
		}
		defer w.Close()
		WithPool(func() {
			// Normalization-stable characters only (ASCII, CJK); AppKit may hand strings back in NFD form.
			for _, title := range []string{"Test Window", "漢字 テスト", ""} {
				w.SetTitle(title)
				if got := GoStringFromNSString(objc.ID(w).Send(Sel("title"))); got != title {
					t.Errorf("title round trip of %q produced %q", title, got)
				}
			}
		})
		if !objc.Send[bool](objc.ID(w), Sel("isOpaque")) {
			t.Error("window is not opaque before SetTransparent")
		}
		w.SetTransparent()
		if objc.Send[bool](objc.ID(w), Sel("isOpaque")) {
			t.Error("window is still opaque after SetTransparent")
		}
		if objc.Send[bool](objc.ID(w), Sel("hasShadow")) {
			t.Error("window still has a shadow after SetTransparent")
		}
	})
}

// TestWindowFrameMath exercises the struct-heavy geometry calls: 32-byte NSRect returns (stret path on amd64) and
// NSRect arguments, plus the titled-window content/frame conversions unison's nativeContentRect math depends on.
func TestWindowFrameMath(t *testing.T) {
	runOnMain(func() {
		w := newTestWindow(testTitledStyle, true, true)
		if w == 0 {
			t.Fatal("NewWindow returned 0")
		}
		defer w.Close()
		frame := geom.NewRect(150, 130, 400, 300)
		w.SetFrame(frame)
		if got := w.Frame(); got != frame {
			t.Errorf("Frame() = %v, want %v", got, frame)
		}
		content := w.ContentRectForFrameRect(frame)
		if content.Width != frame.Width || content.Height >= frame.Height {
			t.Errorf("content rect %v does not fit inside titled frame %v", content, frame)
		}
		if got := w.FrameRectForContentRect(content); got != frame {
			t.Errorf("FrameRectForContentRect(ContentRectForFrameRect(f)) = %v, want %v", got, frame)
		}
		// NSPoint struct return; the value depends on the live mouse position, so only prove the call works.
		w.MouseLocationOutsideOfEventStream()
		if w.Miniaturized() {
			t.Error("Miniaturized() = true for a freshly created window")
		}
		if w.Zoomed() {
			t.Error("Zoomed() = true for a small window")
		}
	})
}

func TestWindowVisibility(t *testing.T) {
	runOnMain(func() {
		w := newTestWindow(testTitledStyle, true, true)
		if w == 0 {
			t.Fatal("NewWindow returned 0")
		}
		WithPool(func() {
			w.SetTitle("unison mac test window")
			if w.Visible() {
				t.Error("window is visible before MakeKeyAndOrderFront")
			}
			w.MakeKeyAndOrderFront()
			if !w.Visible() {
				t.Error("window is not visible after MakeKeyAndOrderFront")
			}
			// Focused() cannot be asserted true here: an inactive app's windows do not become key until the app
			// activates, and tests must not steal focus. It must at least agree with AppKit.
			if got, want := w.Focused(), objc.Send[bool](objc.ID(w), Sel("isKeyWindow")); got != want {
				t.Errorf("Focused() = %v, want %v", got, want)
			}
			w.OrderOut()
			if w.Visible() {
				t.Error("window is still visible after OrderOut")
			}
			w.Close()
		})
	})
}

// TestWindowDelegate proves the Go-registered macWindowDelegate class end to end: handle plumbing through
// SetDelegate/Delegate, real AppKit-initiated delivery (setFrame:display: posts resize/move notifications
// synchronously to the delegate), the direct windowShouldClose: contract, and the notification-object derivation
// used by the remaining delegate methods.
func TestWindowDelegate(t *testing.T) {
	defer func() {
		WindowShouldCloseCallback = nil
		WindowDidResizeCallback = nil
		WindowDidMoveCallback = nil
		WindowMinimizeCallback = nil
		WindowDidBecomeKeyCallback = nil
		WindowDidResignKeyCallback = nil
	}()
	runOnMain(func() {
		w := newTestWindow(testTitledStyle, true, true)
		if w == 0 {
			t.Fatal("NewWindow returned 0")
		}
		d := NewWindowDelegate()
		if d == 0 {
			t.Fatal("NewWindowDelegate returned 0")
		}
		w.SetDelegate(d)
		if got := w.Delegate(); got != d {
			t.Fatalf("Delegate() = %#x, want %#x", got, d)
		}

		// windowShouldClose: nil callback lets the close proceed; a set callback is invoked with the window and
		// blocks the close (unison closes the window itself from the callback).
		if !objc.Send[bool](objc.ID(d), Sel("windowShouldClose:"), objc.ID(w)) {
			t.Error("windowShouldClose: = false with no callback set")
		}
		var shouldCloseWnd Window
		WindowShouldCloseCallback = func(cbw Window) { shouldCloseWnd = cbw }
		if objc.Send[bool](objc.ID(d), Sel("windowShouldClose:"), objc.ID(w)) {
			t.Error("windowShouldClose: = true with a callback set")
		}
		if shouldCloseWnd != w {
			t.Errorf("WindowShouldCloseCallback got %#x, want %#x", shouldCloseWnd, w)
		}

		// Real AppKit delivery: NSWindow posts its didResize/didMove notifications synchronously from
		// setFrame:display:, so the Go delegate methods must have run by the time SetFrame returns. A size change
		// posts only didResize (AppKit treats a combined move+resize as a resize); an origin-only change posts
		// didMove.
		var resized, moved []Window
		WindowDidResizeCallback = func(cbw Window) { resized = append(resized, cbw) }
		WindowDidMoveCallback = func(cbw Window) { moved = append(moved, cbw) }
		w.SetFrame(geom.NewRect(200, 180, 500, 400))
		if len(resized) == 0 {
			t.Error("windowDidResize: was not delivered for a SetFrame size change")
		}
		w.SetFrame(geom.NewRect(220, 200, 500, 400))
		if len(moved) == 0 {
			t.Error("windowDidMove: was not delivered for a SetFrame origin change")
		}
		for _, cbw := range append(resized, moved...) {
			if cbw != w {
				t.Errorf("resize/move callback got %#x, want %#x", cbw, w)
			}
		}

		// The remaining delegate methods derive the window from the notification object. Drive them through real
		// objc_msgSend dispatch with constructed NSNotifications, the same shape AppKit delivers.
		var minimized []bool
		var minimizedWnd, becameKey, resignedKey Window
		WindowMinimizeCallback = func(cbw Window, m bool) {
			minimizedWnd = cbw
			minimized = append(minimized, m)
		}
		WindowDidBecomeKeyCallback = func(cbw Window) { becameKey = cbw }
		WindowDidResignKeyCallback = func(cbw Window) { resignedKey = cbw }
		WithPool(func() {
			notif := objc.ID(Cls("NSNotification")).Send(Sel("notificationWithName:object:"),
				NSStringFromGo("unisonWindowDelegateTest"), objc.ID(w))
			for _, sel := range []string{
				"windowDidMiniaturize:", "windowDidDeminiaturize:", "windowDidBecomeKey:", "windowDidResignKey:",
			} {
				objc.ID(d).Send(Sel(sel), notif)
			}
		})
		if len(minimized) != 2 || !minimized[0] || minimized[1] {
			t.Errorf("minimize callbacks produced %v, want [true false]", minimized)
		}
		if minimizedWnd != w || becameKey != w || resignedKey != w {
			t.Errorf("notification-derived windows = %#x/%#x/%#x, want %#x", minimizedWnd, becameKey, resignedKey, w)
		}

		// Tear down the way unison's nativeDestroy does: detach, release the delegate, close the window.
		w.SetDelegate(0)
		if got := w.Delegate(); got != 0 {
			t.Errorf("Delegate() = %#x after SetDelegate(0)", got)
		}
		d.Release()
		w.Close()
	})
}

// newTestPopupWindow creates a popup window at the given content rect with its content view installed, as unison's
// macInitPopup does. The returned cleanup mirrors nativeDestroy.
func newTestPopupWindow(t *testing.T, contentRect geom.Rect) (Window, View, func()) {
	t.Helper()
	sharedApp()
	w := NewPopupWindow(contentRect)
	if w == 0 {
		t.Fatal("NewPopupWindow returned 0")
	}
	v := NewPopupView(w)
	if v == 0 {
		w.Close()
		t.Fatal("NewPopupView returned 0")
	}
	w.SetContentView(v)
	w.MakeFirstResponder(v)
	return w, v, func() {
		ClearAXViewAlias(v)
		w.OrderOut()
		v.Release()
		w.Close()
	}
}

// TestNewPopupWindow proves the configuration a popup menu window is created with: borderless, never key or main, at
// the pop-up menu level in every space and beside full-screen windows, out of the window cycle, without show and hide
// animations, with a shadow, and hidden from assistive technologies. Ordered front, as Window.Show does it, it is on
// the screen but not key.
func TestNewPopupWindow(t *testing.T) {
	runOnMain(func() {
		w, _, cleanup := newTestPopupWindow(t, geom.NewRect(140, 140, 120, 80))
		defer cleanup()
		ow := objc.ID(w)
		if got := w.StyleMask(); got != WindowStyleMaskBorderless {
			t.Errorf("StyleMask() = %#x, want borderless", got)
		}
		for _, sel := range []string{
			"canBecomeKeyWindow", "canBecomeMainWindow", "isRestorable", "isAccessibilityElement",
		} {
			if objc.Send[bool](ow, Sel(sel)) {
				t.Errorf("%s = true, want false", sel)
			}
		}
		if !objc.Send[bool](ow, Sel("hasShadow")) {
			t.Error("hasShadow = false, want true")
		}
		if got := objc.Send[int64](ow, Sel("level")); got != WindowLevelPopUpMenu {
			t.Errorf("level = %d, want %d", got, WindowLevelPopUpMenu)
		}
		wantBehavior := WindowCollectionBehaviorFullScreenAuxiliary | WindowCollectionBehaviorCanJoinAllSpaces |
			WindowCollectionBehaviorTransient | WindowCollectionBehaviorIgnoresCycle
		if got := objc.Send[uint64](ow, Sel("collectionBehavior")); got != wantBehavior {
			t.Errorf("collectionBehavior = %#x, want %#x", got, wantBehavior)
		}
		if got := objc.Send[int64](ow, Sel("animationBehavior")); got != WindowAnimationBehaviorNone {
			t.Errorf("animationBehavior = %d, want %d", got, WindowAnimationBehaviorNone)
		}
		if got := objc.Send[int64](ow, Sel("tabbingMode")); got != WindowTabbingModeDisallowed {
			t.Errorf("tabbingMode = %d, want %d", got, WindowTabbingModeDisallowed)
		}
		w.OrderFront()
		if !w.Visible() {
			t.Error("popup window is not visible after OrderFront")
		}
		if w.Focused() {
			t.Error("popup window is key after OrderFront")
		}
	})
}

// TestWindowAccessibilityHidden proves the overrides SetAccessibilityHidden turns on: a hidden window is no
// accessibility element, has no children and hands hit tests to its content view, which is shown with a point outside
// the window's frame, where NSWindow itself answers with the window and only the content view's tree, holding an
// overlay there, finds anything, and turning it off again restores NSWindow's own answers.
func TestWindowAccessibilityHidden(t *testing.T) {
	const (
		overlayID accessibility.NodeID = 50 + iota
		overlayItemID
	)
	runOnMain(func() {
		tree := newAXTestTree()
		tree.Nodes[overlayID] = &accessibility.Node{
			ID:       overlayID,
			Parent:   axTestRoot,
			Children: []accessibility.NodeID{overlayItemID},
			Name:     "Menu",
			Role:     role.Menu,
			Bounds:   geom.NewRect(400, 300, 100, 40),
			Overlay:  true,
		}
		tree.Nodes[overlayItemID] = &accessibility.Node{
			ID:      overlayItemID,
			Parent:  overlayID,
			Name:    "Item",
			Role:    role.MenuItem,
			Bounds:  geom.NewRect(404, 304, 92, 20),
			Actions: accessibility.ActionSet(0).With(accessibility.Press),
		}
		tree.Nodes[axTestRoot].Children = append(tree.Nodes[axTestRoot].Children, overlayID)
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		w := a.wnd
		ow := objc.ID(w)
		WithPool(func() {
			if !objc.Send[bool](ow, Sel("isAccessibilityElement")) {
				t.Error("an ordinary window is not an accessibility element")
			}
			shown := NSArrayCount(ow.Send(Sel("accessibilityChildren")))
			if shown == 0 {
				t.Error("an ordinary window reports no accessibility children")
			}
			// NSWindow's own answer for a point outside its frame is the window itself, never anything the content
			// view's tree holds there.
			outside := axTestScreenPoint(a, geom.NewPoint(450, 314))
			if got := ow.Send(Sel("accessibilityHitTest:"), outside); got != ow {
				t.Errorf("ordinary window hit test outside its frame = %#x, want the window itself %#x", got, ow)
			}
			w.SetAccessibilityHidden(true)
			if objc.Send[bool](ow, Sel("isAccessibilityElement")) {
				t.Error("a hidden window is still an accessibility element")
			}
			if children := ow.Send(Sel("accessibilityChildren")); children == 0 || NSArrayCount(children) != 0 {
				t.Errorf("a hidden window reports children %#x (%d), want an empty array", children,
					NSArrayCount(children))
			}
			item := objc.ID(v).Send(Sel("accessibilityHitTest:"), outside)
			if item == 0 || item != a.Element(overlayItemID) {
				t.Fatalf("content view hit test = %#x, want the overlay's item %#x", item, a.Element(overlayItemID))
			}
			if got := ow.Send(Sel("accessibilityHitTest:"), outside); got != item {
				t.Errorf("hidden window hit test = %#x, want the content view's answer %#x", got, item)
			}
			w.SetAccessibilityHidden(false)
			if !objc.Send[bool](ow, Sel("isAccessibilityElement")) {
				t.Error("a window shown again is not an accessibility element")
			}
			if got := NSArrayCount(ow.Send(Sel("accessibilityChildren"))); got != shown {
				t.Errorf("a window shown again reports %d children, want %d", got, shown)
			}
			if got := ow.Send(Sel("accessibilityHitTest:"), outside); got != ow {
				t.Errorf("window shown again hit test outside its frame = %#x, want the window itself %#x", got, ow)
			}
		})
	})
}

// TestPopupWindowHitTestReachesTheTarget proves the path an assistive technology's hit test over a popup takes: the
// hidden popup window hands it to its content view, whose alias resolves it in the target's tree, so a point the
// popup's own frame does not even contain still finds the target's element there.
func TestPopupWindowHitTestReachesTheTarget(t *testing.T) {
	runOnMain(func() {
		target, a, _, cleanup := newAXTestAdapter(t)
		defer cleanup()
		if a == nil {
			return
		}
		popup, popupView, closePopup := newTestPopupWindow(t, geom.NewRect(700, 500, 120, 80))
		defer closePopup()
		popup.OrderFront()
		SetAXViewAlias(popupView, target)
		WithPool(func() {
			pt := axTestScreenPoint(a, geom.NewPoint(50, 30))
			button := objc.ID(target).Send(Sel("accessibilityHitTest:"), pt)
			if button == 0 || button != a.Element(axTestButton) {
				t.Fatalf("target hit test = %#x, want the button %#x", button, a.Element(axTestButton))
			}
			if got := objc.ID(popup).Send(Sel("accessibilityHitTest:"), pt); got != button {
				t.Errorf("popup window hit test = %#x, want the target's button %#x", got, button)
			}
		})
	})
}

// TestWindowFrameMouseDown proves the classification behind the sendEvent: override: a mouse button press is reported
// as one on the frame only when it lies outside the content view, and no other event is.
func TestWindowFrameMouseDown(t *testing.T) {
	runOnMain(func() {
		w, v, cleanup := newTestWindowAndView(t)
		defer cleanup()
		WithPool(func() {
			frame := objc.Send[NSRect](objc.ID(v), Sel("frame"))
			inside := NSPoint{X: frame.Origin.X + 10, Y: frame.Origin.Y + 10}
			title := NSPoint{X: frame.Origin.X + 10, Y: frame.Origin.Y + frame.Size.Height + 5}
			for _, c := range []struct {
				name      string
				eventType uint64
				where     NSPoint
				want      bool
			}{
				{"left press on the title bar", nsEventTypeLeftMouseDown, title, true},
				{"right press on the title bar", nsEventTypeRightMouseDown, title, true},
				{"other press on the title bar", nsEventTypeOtherMouseDown, title, true},
				{"left press on the content", nsEventTypeLeftMouseDown, inside, false},
				{"left release on the title bar", nsEventTypeLeftMouseUp, title, false},
				{"move over the title bar", nsEventTypeMouseMoved, title, false},
			} {
				event := synthMouseEvent(c.eventType, c.where, 0, w)
				if got := windowFrameMouseDown(objc.ID(w), event); got != c.want {
					t.Errorf("%s: windowFrameMouseDown = %v, want %v", c.name, got, c.want)
				}
			}
		})
	})
}
