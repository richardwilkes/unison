// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"image"
	"log/slog"
	"slices"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/mac"
)

type platformWindow struct {
	wnd            mac.Window
	view           mac.View
	nsCursorHidden bool
	maximized      bool
}

func findWindowByNSWindow(macWnd mac.Window) *Window {
	if i := slices.IndexFunc(windowList, func(w *Window) bool {
		return w.wnd.wnd == macWnd
	}); i != -1 {
		return windowList[i]
	}
	return nil
}

func initNativeWindowCallbacks() {
	mac.WindowKeyPressedCallback = func(macWnd mac.Window, key int, mods uint) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.keyPressed(rawScanCodeToKeyCodeMap[key], translateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key pressed callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowKeyTypedCallback = func(macWnd mac.Window, ch rune) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.runeTyped(ch)
		} else {
			slog.Warn("received window key typed callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowKeyReleasedCallback = func(macWnd mac.Window, key int, mods uint) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.keyReleased(rawScanCodeToKeyCodeMap[key], translateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key released callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowShouldCloseCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.nativeRequestClose()
		} else {
			slog.Warn("received window should close callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidResizeCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			maximized := w.wnd.wnd.Zoomed()
			if w.wnd.maximized != maximized {
				w.wnd.maximized = maximized
				if w.MaximizeCallback != nil {
					w.MaximizeCallback(maximized)
				}
			}
			r := w.wnd.view.Frame()
			if r.Width != w.lastWidth || r.Height != w.lastHeight {
				w.lastWidth = r.Width
				w.lastHeight = r.Height
				current := w.ContentRect()
				adjusted := w.adjustContentRectForMinMax(current)
				if adjusted != current {
					w.SetContentRect(adjusted)
				}
				w.resized()
			}
		} else {
			slog.Warn("received window did resize callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidMoveCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			w.moved()
		} else {
			slog.Warn("received window did move callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMinimizeCallback = func(macWnd mac.Window, minimized bool) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.MinimizedCallback != nil {
				w.MinimizedCallback(minimized)
			}
		} else {
			slog.Warn("received window minimize callback for unknown window", "window", macWnd, "minimized", minimized)
		}
	}
	mac.WindowDidBecomeKeyCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.gainedFocus()
		} else {
			slog.Warn("received window did become key callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidResignKeyCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.lostFocus()
		} else {
			slog.Warn("received window did resign key callback for unknown window", "window", macWnd, "error", errs.New("here"))
		}
	}
	mac.WindowCursorUpdateCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.updateCursorImage()
		} else {
			slog.Warn("received window cursor update callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseEnterCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.updateCursorImage()
			w.mouseEnter(w.MouseLocation(), w.lastKeyModifiers)
		} else {
			slog.Warn("received window mouse enter callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseExitCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.updateCursorImage()
			w.mouseExit()
		} else {
			slog.Warn("received window mouse exit callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseMovedCallback = func(macWnd mac.Window, pt geom.Point) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.nativeMouseMoved(pt)
		} else {
			slog.Warn("received window mouse moved callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowScrollCallback = func(macWnd mac.Window, deltaX, deltaY float32, pixels bool) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.nativeMouseWheel(geom.NewPoint(deltaX, deltaY), pixels)
		} else {
			slog.Warn("received window scroll callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseClickCallback = func(macWnd mac.Window, button int, pressed bool, mods uint) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.nativeMouseClick(button, pressed, translateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse click callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowUpdateLayerCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			w.draw()
		} else {
			slog.Warn("received window update layer callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowRedrawCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.draw()
		} else {
			slog.Warn("received window draw rect callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowScaleCallback = func(macWnd mac.Window, scale geom.Point) {
		// This will be called once before the window is finished initializing, so just ignore any unknown windows here.
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.ContentScaleCallback != nil {
				w.ContentScaleCallback(scale)
			}
		}
	}
	mac.WindowDropCallback = func(macWnd mac.Window, filePaths []string) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.fileDrop(filePaths)
		} else {
			slog.Warn("received window drop callback for unknown window", "window", macWnd)
		}
	}
}

func (w *Window) initNativeWindow(cfg *WindowConfig) error {
	styleMask := mac.WindowStyleMaskMiniaturizable
	if cfg.Undecorated {
		styleMask |= mac.WindowStyleMaskBorderless
	} else {
		styleMask |= mac.WindowStyleMaskTitled | mac.WindowStyleMaskClosable
		if !cfg.NotResizable {
			styleMask |= mac.WindowStyleMaskResizable
		}
	}
	nw := mac.NewWindow(geom.NewRect(0, 0, 1, 1), styleMask, true, true)
	if nw == 0 {
		return errs.New("unable to create native window")
	}
	if cfg.NotResizable {
		nw.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenNone)
	} else {
		nw.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenPrimary | mac.WindowCollectionBehaviorManaged)
	}
	if cfg.Floating {
		nw.SetLevel(mac.WindowLevelFloating)
	}
	v := mac.NewView(nw)
	if cfg.Transparent {
		nw.SetTransparent()
	}
	nw.SetContentView(v)
	nw.MakeFirstResponder(v)
	if cfg.Title != "" {
		nw.SetTitle(cfg.Title)
	}
	delegate := mac.NewWindowDelegate(nw)
	nw.SetDelegate(delegate)
	nw.SetAcceptsMouseMovedEvents(true)
	nw.SetRestorable(false)
	nw.SetTabbingMode(mac.WindowTabbingModeDisallowed)
	w.wnd.wnd = nw
	w.wnd.view = v
	return w.glCtx.create(w, cfg.Share, cfg.Transparent)
}

func (w *Window) setTitle(title string) {
	w.wnd.wnd.SetTitle(title)
}

func (w *Window) setTitleIcons(_images []*image.NRGBA) {
	// macOS doesn't have window icons, so just ignore this.
}

func (w *Window) frameRect() geom.Rect {
	r := w.wnd.wnd.Frame()
	r.Y = transformCocoaY(r.Bottom())
	return r
}

func (w *Window) frameRectForContentRect(contentRect geom.Rect) geom.Rect {
	contentRect.Y = transformCocoaY(contentRect.Bottom())
	frameRect := w.wnd.wnd.FrameRectForContentRect(contentRect)
	frameRect.Y = transformCocoaY(frameRect.Bottom())
	return frameRect
}

func (w *Window) contentRect() geom.Rect {
	r := w.wnd.wnd.ContentRectForFrameRect(w.wnd.wnd.Frame())
	r.Y = transformCocoaY(r.Bottom())
	return r
}

func (w *Window) contentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	frameRect.Y = transformCocoaY(frameRect.Bottom())
	contentRect := w.wnd.wnd.ContentRectForFrameRect(frameRect)
	contentRect.Y = transformCocoaY(contentRect.Bottom())
	return contentRect
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect geom.Rect) {
	if w.IsValid() {
		rect = w.adjustContentRectForMinMax(rect)
		rect.Y = transformCocoaY(rect.Bottom())
		w.wnd.wnd.SetFrame(w.wnd.wnd.FrameRectForContentRect(rect))
	}
}

func (w *Window) convertRawMouseLocationForPlatform(where geom.Point) geom.Point {
	return where
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	return modifiersFromEventModifierFlags(mac.CurrentModifierFlags())
}

func (w *Window) adjustToCursorChange() {
	if w.cursorInContentArea() {
		w.updateCursorImage()
	}
}

func (w *Window) updateCursorImage() {
	if w.cursorHidden {
		if !w.wnd.nsCursorHidden {
			mac.HideCursor()
			w.wnd.nsCursorHidden = true
		}
	} else {
		if w.wnd.nsCursorHidden {
			mac.ShowCursor()
			w.wnd.nsCursorHidden = false
		}
		if w.cursor != nil {
			w.cursor.cursor.Set()
		} else {
			mac.ArrowCursor().Set()
		}
	}
}

func (w *Window) cursorInContentArea() bool {
	return w.wnd.view.MouseInRect(w.wnd.wnd.MouseLocationOutsideOfEventStream(), w.wnd.view.Frame())
}

func (w *Window) cursorPosition() geom.Point {
	loc := w.wnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.wnd.view.Frame()
	return geom.NewPoint(loc.X, frame.Height-loc.Y)
}

func (w *Window) backingScale() geom.Point {
	return w.wnd.view.BackingScale()
}

func (w *Window) minimize() {
	if !w.wnd.wnd.Miniaturized() {
		w.wnd.wnd.Miniaturize()
	}
}

func (w *Window) maximize() {
	if !w.wnd.wnd.Zoomed() {
		w.wnd.wnd.Zoom()
	}
}

func (w *Window) acquireFocus() {
	mac.ActivateIgnoringOtherApps()
	w.wnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) visible() bool {
	return w.wnd.wnd.Visible()
}

func (w *Window) show() {
	w.wnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) hide() {
	w.wnd.wnd.OrderOut()
}

func (w *Window) nativeDestroy() {
	w.wnd.wnd.OrderOut()
	w.glCtx.destroy()
	delegate := w.wnd.wnd.Delegate()
	w.wnd.wnd.SetDelegate(0)
	delegate.Release()
	w.wnd.view.Release()
	w.wnd.wnd.Close()
	w.wnd.wnd = 0
	pollEvents()
}
