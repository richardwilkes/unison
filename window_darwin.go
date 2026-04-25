// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/mac"
)

type apiWindow struct {
	wnd            mac.Window
	view           mac.View
	nsCursorHidden bool
}

func macFindWindow(macWnd mac.Window) *Window {
	for _, w := range windowList {
		if w.wnd.wnd == macWnd {
			return w
		}
	}
	return nil
}

func macInitWindowCallbacks() {
	mac.WindowKeyPressedCallback = func(macWnd mac.Window, key uint16, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.keyPressed(rawScanCodeToKeyCodeMap[key], macTranslateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key pressed callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowKeyTypedCallback = func(macWnd mac.Window, ch rune) {
		if w := macFindWindow(macWnd); w != nil {
			w.runeTyped(ch)
		} else {
			slog.Warn("received window key typed callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowKeyReleasedCallback = func(macWnd mac.Window, key uint16, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.keyReleased(rawScanCodeToKeyCodeMap[key], macTranslateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key released callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowShouldCloseCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.requestClose()
		} else {
			slog.Warn("received window should close callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidResizeCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			maximized := w.wnd.wnd.Zoomed()
			if w.maximized != maximized {
				w.maximized = maximized
				if w.MaximizedCallback != nil {
					w.MaximizedCallback(maximized)
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
		if w := macFindWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			w.moved()
		} else {
			slog.Warn("received window did move callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMinimizeCallback = func(macWnd mac.Window, minimized bool) {
		if w := macFindWindow(macWnd); w != nil {
			if w.minimized != minimized {
				w.minimized = minimized
				if w.MinimizedCallback != nil {
					w.MinimizedCallback(minimized)
				}
			}
		} else {
			slog.Warn("received window minimize callback for unknown window", "window", macWnd, "minimized", minimized)
		}
	}
	mac.WindowDidBecomeKeyCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.gainedFocus()
		} else {
			slog.Warn("received window did become key callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidResignKeyCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.lostFocus()
		} else {
			slog.Warn("received window did resign key callback for unknown window", "window", macWnd, "error",
				errs.New("here"))
		}
	}
	mac.WindowCursorUpdateCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.apiUpdateCursorImage()
		} else {
			slog.Warn("received window cursor update callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseEnterCallback = func(macWnd mac.Window, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.apiUpdateCursorImage()
			w.mouseEnter(w.MouseLocation(), macTranslateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse enter callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseExitCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.apiUpdateCursorImage()
			w.mouseExit()
		} else {
			slog.Warn("received window mouse exit callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseMovedCallback = func(macWnd mac.Window, pt geom.Point, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.mouseMovedOrDragged(w.apiConvertRawMouse(pt), macTranslateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse moved callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowScrollCallback = func(macWnd mac.Window, deltaX, deltaY float32, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.mouseWheel(w.MouseLocation(), geom.NewPoint(deltaX, deltaY),
				macTranslateModifiers(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window scroll callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseClickCallback = func(macWnd mac.Window, button int, where geom.Point, pressed bool, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			actualMods := macTranslateModifiers(mac.EventModifierFlags(mods))
			if pressed {
				w.mouseDown(where, button, actualMods)
			} else {
				w.mouseUp(where, button, actualMods)
			}
		} else {
			slog.Warn("received window mouse click callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowUpdateLayerCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			w.draw()
		} else {
			slog.Warn("received window update layer callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowRedrawCallback = func(macWnd mac.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.draw()
		} else {
			slog.Warn("received window draw rect callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowScaleCallback = func(macWnd mac.Window, scale geom.Point) {
		// This will be called once before the window is finished initializing, so just ignore any unknown windows here.
		if w := macFindWindow(macWnd); w != nil {
			if w.ContentScaleCallback != nil {
				w.ContentScaleCallback(scale)
			}
		}
	}
	mac.WindowDropCallback = func(macWnd mac.Window, filePaths []string) {
		if w := macFindWindow(macWnd); w != nil {
			w.fileDrop(filePaths)
		} else {
			slog.Warn("received window drop callback for unknown window", "window", macWnd)
		}
	}
}

func (w *Window) apiInit() error {
	styleMask := mac.WindowStyleMaskMiniaturizable
	if w.undecorated {
		styleMask |= mac.WindowStyleMaskBorderless
	} else {
		styleMask |= mac.WindowStyleMaskTitled | mac.WindowStyleMaskClosable
		if !w.notResizable {
			styleMask |= mac.WindowStyleMaskResizable
		}
	}
	nw := mac.NewWindow(geom.NewRect(0, 0, 1, 1), styleMask, true, true)
	if nw == 0 {
		return errs.New("unable to create native window")
	}
	if w.notResizable {
		nw.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenNone)
	} else {
		nw.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenPrimary | mac.WindowCollectionBehaviorManaged)
	}
	if w.floating {
		nw.SetLevel(mac.WindowLevelFloating)
	}
	v := mac.NewView(nw)
	if w.transparent {
		nw.SetTransparent()
	}
	nw.SetContentView(v)
	nw.MakeFirstResponder(v)
	if w.title != "" {
		nw.SetTitle(w.title)
	}
	delegate := mac.NewWindowDelegate(nw)
	nw.SetDelegate(delegate)
	nw.SetAcceptsMouseMovedEvents(true)
	nw.SetRestorable(false)
	nw.SetTabbingMode(mac.WindowTabbingModeDisallowed)
	w.wnd.wnd = nw
	w.wnd.view = v
	return nil
}

func (w *Window) apiSetTitle(title string) {
	w.wnd.wnd.SetTitle(title)
}

func (w *Window) apiSetTitleIcons(_images []*image.NRGBA) {
	// macOS doesn't have window icons, so just ignore this.
}

func (w *Window) apiDisplay() *Display {
	return BestDisplayForRect(w.apiFrameRect())
}

func (w *Window) apiFrameRect() geom.Rect {
	r := w.wnd.wnd.Frame()
	r.Y = macTransformY(r.Bottom())
	return r
}

func (w *Window) apiFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	contentRect.Y = macTransformY(contentRect.Bottom())
	frameRect := w.wnd.wnd.FrameRectForContentRect(contentRect)
	frameRect.Y = macTransformY(frameRect.Bottom())
	return frameRect
}

func (w *Window) apiEnsureOnDisplay() {
	frameRect := w.apiFrameRect()
	revisedRect := w.apiDisplay().FitRectOnto(frameRect)
	if frameRect != revisedRect {
		w.SetFrameRect(revisedRect)
	}
}

func (w *Window) apiContentRect() geom.Rect {
	r := w.wnd.wnd.ContentRectForFrameRect(w.wnd.wnd.Frame())
	r.Y = macTransformY(r.Bottom())
	return r
}

func (w *Window) apiContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	frameRect.Y = macTransformY(frameRect.Bottom())
	contentRect := w.wnd.wnd.ContentRectForFrameRect(frameRect)
	contentRect.Y = macTransformY(contentRect.Bottom())
	return contentRect
}

func (w *Window) apiSetContentRect(rect geom.Rect) {
	rect.Y = macTransformY(rect.Bottom())
	w.wnd.wnd.SetFrame(w.wnd.wnd.FrameRectForContentRect(rect))
}

func (w *Window) apiConvertRawMouse(where geom.Point) geom.Point {
	return where
}

func (w *Window) apiCurrentKeyModifiers() Modifiers {
	return macModifiersFromEventModifierFlags(mac.CurrentModifierFlags())
}

func (w *Window) apiUpdateCursorImage() {
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
			w.cursor.cursor.cursor.Set()
		} else {
			mac.ArrowCursor().Set()
		}
	}
}

func (w *Window) apiCursorInContentArea() bool {
	return w.wnd.view.MouseInRect(w.wnd.wnd.MouseLocationOutsideOfEventStream(), w.wnd.view.Frame())
}

func (w *Window) apiCursorPosition() geom.Point {
	loc := w.wnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.wnd.view.Frame()
	return w.apiConvertRawMouse(geom.NewPoint(loc.X, frame.Height-loc.Y))
}

func (w *Window) apiBackingScale() geom.Point {
	return w.wnd.view.BackingScale()
}

func (w *Window) apiMinimize() {
	w.wnd.wnd.Miniaturize()
}

func (w *Window) apiMaximize() {
	w.wnd.wnd.Zoom()
}

func (w *Window) apiAcquireFocus() {
	mac.ActivateIgnoringOtherApps()
	w.wnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) apiVisible() bool {
	return w.wnd.wnd.Visible()
}

func (w *Window) apiShow() {
	w.wnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) apiHide() {
	w.wnd.wnd.OrderOut()
}

func (w *Window) apiDestroy() {
	w.glCtx.apiDestroy()
	if w.wnd.wnd != 0 {
		w.wnd.wnd.OrderOut()
		if delegate := w.wnd.wnd.Delegate(); delegate != 0 {
			w.wnd.wnd.SetDelegate(0)
			delegate.Release()
		}
		w.wnd.view.Release()
		w.wnd.wnd.Close()
		w.wnd.wnd = 0
	}
	apiPollEvents()
}
