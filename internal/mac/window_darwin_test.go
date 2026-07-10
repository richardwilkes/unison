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
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// newTestWindow creates a window the way unison's apiInit does (titled unless undecorated), on the caller's
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
// NSRect arguments, plus the titled-window content/frame conversions unison's apiContentRect math depends on.
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

		// Tear down the way unison's apiClose does: detach, release the delegate, close the window.
		w.SetDelegate(0)
		if got := w.Delegate(); got != 0 {
			t.Errorf("Delegate() = %#x after SetDelegate(0)", got)
		}
		d.Release()
		w.Close()
	})
}
