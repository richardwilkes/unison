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
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/internal/cocoa"
)

type apiWindow struct {
	wnd            cocoa.Window
	view           cocoa.View
	nsCursorHidden bool
}

func macFindWindow(macWnd cocoa.Window) *Window {
	for _, w := range windowList {
		if w.wnd.wnd == macWnd {
			return w
		}
	}
	return nil
}

func macInitWindowCallbacks() {
	cocoa.WindowKeyPressedCallback = func(macWnd cocoa.Window, key uint16, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.keyPressed(rawScanCodeToKeyCodeMap[key], macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key pressed callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowKeyTypedCallback = func(macWnd cocoa.Window, ch rune) {
		if w := macFindWindow(macWnd); w != nil {
			w.runeTyped(ch)
		} else {
			slog.Warn("received window key typed callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowKeyReleasedCallback = func(macWnd cocoa.Window, key uint16, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.keyReleased(rawScanCodeToKeyCodeMap[key], macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key released callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowShouldCloseCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.requestClose()
		} else {
			slog.Warn("received window should close callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowDidResizeCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			maximized := w.wnd.wnd.Zoomed()
			if w.maximized != maximized {
				w.maximized = maximized
				if w.MaximizedCallback != nil {
					SafeCall(func() { w.MaximizedCallback(maximized) })
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
	cocoa.WindowDidMoveCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			w.moved()
		} else {
			slog.Warn("received window did move callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMinimizeCallback = func(macWnd cocoa.Window, minimized bool) {
		if w := macFindWindow(macWnd); w != nil {
			if w.minimized != minimized {
				w.minimized = minimized
				if w.MinimizedCallback != nil {
					SafeCall(func() { w.MinimizedCallback(minimized) })
				}
			}
		} else {
			slog.Warn("received window minimize callback for unknown window", "window", macWnd, "minimized", minimized)
		}
	}
	cocoa.WindowDidBecomeKeyCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.gainedFocus()
		} else {
			slog.Warn("received window did become key callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowDidResignKeyCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.lostFocus()
		} else {
			slog.Warn("received window did resign key callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowCursorUpdateCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.apiUpdateCursorImage()
		} else {
			slog.Warn("received window cursor update callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseEnterCallback = func(macWnd cocoa.Window, pt geom.Point, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.apiUpdateCursorImage()
			w.mouseEnter(pt, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse enter callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseExitCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.apiUpdateCursorImage()
			w.mouseExit()
		} else {
			slog.Warn("received window mouse exit callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseMovedCallback = func(macWnd cocoa.Window, pt geom.Point, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.mouseMovedOrDragged(pt, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse moved callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowScrollCallback = func(macWnd cocoa.Window, delta geom.Point, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.mouseWheel(w.MouseLocation(), delta, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window scroll callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseClickCallback = func(macWnd cocoa.Window, button int, where geom.Point, pressed bool, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			actualMods := macTranslateModifiers(cocoa.EventModifierFlags(mods))
			if pressed {
				w.mouseDown(where, button, actualMods)
			} else {
				w.mouseUp(where, button, actualMods)
			}
		} else {
			slog.Warn("received window mouse click callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowUpdateLayerCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.glCtx.ctx.Update()
			w.draw()
		} else {
			slog.Warn("received window update layer callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowRedrawCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.draw()
		} else {
			slog.Warn("received window draw rect callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowScaleCallback = func(macWnd cocoa.Window, scale geom.Point) {
		// This will be called once before the window is finished initializing, so just ignore any unknown windows here.
		if w := macFindWindow(macWnd); w != nil {
			if w.ContentScaleCallback != nil {
				SafeCall(func() { w.ContentScaleCallback(scale) })
			}
		}
	}
	cocoa.WindowDragEnterCallback = func(macWnd cocoa.Window, d cocoa.DragInfo, where geom.Point, mods uint) drag.Op {
		if w := macFindWindow(macWnd); w != nil {
			return w.dragEntered(d, where, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		}
		slog.Warn("received window drag enter callback for unknown window", "window", macWnd)
		return drag.None
	}
	cocoa.WindowDragUpdateCallback = func(macWnd cocoa.Window, d cocoa.DragInfo, where geom.Point, mods uint) drag.Op {
		if w := macFindWindow(macWnd); w != nil {
			return w.dragUpdate(d, where, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		}
		slog.Warn("received window drag update callback for unknown window", "window", macWnd)
		return drag.None
	}
	cocoa.WindowDropCallback = func(macWnd cocoa.Window, d cocoa.DragInfo, where geom.Point, mods uint) bool {
		if w := macFindWindow(macWnd); w != nil {
			return w.drop(d, where, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		}
		slog.Warn("received window drop callback for unknown window", "window", macWnd)
		return false
	}
	cocoa.WindowDragExitCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.dragExit()
		} else {
			slog.Warn("received window drag exit callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowDragSourceFinishedCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.dragSourceFinished()
		} else {
			slog.Warn("received window drag source finished callback for unknown window", "window", macWnd)
		}
	}
}

func (w *Window) apiInit() error {
	styleMask := cocoa.WindowStyleMaskMiniaturizable
	if w.undecorated {
		styleMask |= cocoa.WindowStyleMaskBorderless
	} else {
		styleMask |= cocoa.WindowStyleMaskTitled | cocoa.WindowStyleMaskClosable
		if !w.notResizable {
			styleMask |= cocoa.WindowStyleMaskResizable
		}
	}
	nw := cocoa.NewWindow(geom.NewRect(0, 0, 1, 1), styleMask, true, true)
	if nw == 0 {
		return errs.New("unable to create native window")
	}
	if w.notResizable {
		nw.SetCollectionBehavior(cocoa.WindowCollectionBehaviorFullScreenNone)
	} else {
		nw.SetCollectionBehavior(cocoa.WindowCollectionBehaviorFullScreenPrimary | cocoa.WindowCollectionBehaviorManaged)
	}
	if w.floating {
		nw.SetLevel(cocoa.WindowLevelFloating)
	}
	v := cocoa.NewView(nw)
	if w.transparent {
		nw.SetTransparent()
	}
	nw.SetContentView(v)
	nw.MakeFirstResponder(v)
	if w.title != "" {
		nw.SetTitle(w.title)
	}
	nw.SetDelegate(cocoa.NewWindowDelegate())
	nw.SetAcceptsMouseMovedEvents(true)
	nw.SetRestorable(false)
	nw.SetTabbingMode(cocoa.WindowTabbingModeDisallowed)
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

func (w *Window) apiCurrentKeyModifiers() mod.Modifiers {
	return macModifiersFromEventModifierFlags(cocoa.CurrentModifierFlags())
}

func (w *Window) apiUpdateCursorImage() {
	if w.cursorHidden {
		if !w.wnd.nsCursorHidden {
			cocoa.HideCursor()
			w.wnd.nsCursorHidden = true
		}
	} else {
		if w.wnd.nsCursorHidden {
			cocoa.ShowCursor()
			w.wnd.nsCursorHidden = false
		}
		if w.cursor != nil {
			w.cursor.cursor.Set()
		} else {
			cocoa.ArrowCursor().Set()
		}
	}
}

func (w *Window) apiCursorInContentArea() bool {
	return w.wnd.view.MouseInRect(w.wnd.wnd.MouseLocationOutsideOfEventStream(), w.wnd.view.Frame())
}

func (w *Window) apiCursorPosition() geom.Point {
	loc := w.wnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.wnd.view.Frame()
	return geom.NewPoint(loc.X, frame.Height-loc.Y)
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

func (w *Window) apiAcquireFocusAndBringToFront() {
	cocoa.ActivateIgnoringOtherApps()
	w.wnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) apiVisible() bool {
	return w.wnd.wnd.Visible()
}

func (w *Window) apiShow() {
	// Order the window front without making it key, matching the other platforms, where Show() makes the window
	// visible without giving it the focus. ToFront() layers apiAcquireFocusAndBringToFront on top of this for the
	// show-and-focus behavior.
	w.wnd.wnd.OrderFront()
}

func (w *Window) apiHide() {
	w.wnd.wnd.OrderOut()
}

func (w *Window) apiStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
	nrgba, r := macDragImageAndFrame(img, origin)
	r.Y = w.wnd.view.Frame().Height - r.Height - r.Y
	w.wnd.view.BeginDraggingSession(nrgba, r, opMask, data...)
}

// macDragImageAndFrame converts the optional drag image into the pixel data and top-left-origin frame handed to
// BeginDraggingSession. The image may be nil, and conversion failures are logged or ignored, since the drag itself
// still works without an image; in those cases nil pixel data and a 1x1 frame anchored at origin are returned so
// AppKit still gets a valid, effectively invisible dragging frame.
func macDragImageAndFrame(img *Image, origin geom.Point) (*image.NRGBA, geom.Rect) {
	if img != nil {
		size := img.LogicalSize()
		if size.Width >= 1 && size.Height >= 1 {
			nrgba, err := img.ToNRGBA()
			if err != nil {
				errs.Log(err)
			} else {
				return nrgba, geom.Rect{Point: origin, Size: size}
			}
		}
	}
	return nil, geom.Rect{Point: origin, Size: geom.NewSize(1, 1)}
}

func (w *Window) apiUpdateRegisteredDragTypes(types []*uti.DataType) {
	w.wnd.view.UnregisterDraggedTypes()
	if len(types) != 0 {
		w.wnd.view.RegisterDraggedTypes(types)
	}
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

func macModifiersFromEventModifierFlags(flags cocoa.EventModifierFlags) mod.Modifiers {
	var mods mod.Modifiers
	if flags&cocoa.EventModifierFlagShift != 0 {
		mods |= mod.Shift
	}
	if flags&cocoa.EventModifierFlagControl != 0 {
		mods |= mod.Control
	}
	if flags&cocoa.EventModifierFlagOption != 0 {
		mods |= mod.Option
	}
	if flags&cocoa.EventModifierFlagCommand != 0 {
		mods |= mod.Command
	}
	if flags&cocoa.EventModifierFlagCapsLock != 0 {
		mods |= mod.CapsLock
	}
	return mods
}

func macEventModifierFlagsFromModifiers(mods mod.Modifiers) cocoa.EventModifierFlags {
	var flags cocoa.EventModifierFlags
	if mods.ShiftDown() {
		flags |= cocoa.EventModifierFlagShift
	}
	if mods.ControlDown() {
		flags |= cocoa.EventModifierFlagControl
	}
	if mods.OptionDown() {
		flags |= cocoa.EventModifierFlagOption
	}
	if mods.CommandDown() {
		flags |= cocoa.EventModifierFlagCommand
	}
	if mods.CapsLockDown() {
		flags |= cocoa.EventModifierFlagCapsLock
	}
	return flags
}
