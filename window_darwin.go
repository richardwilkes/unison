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
	"github.com/richardwilkes/unison/enums/thememode"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// macAppAppearance is the appearance name last handed to cocoa.SetAppAppearance, so that a theme change that reaches
// several windows sets the application's appearance once rather than once per window. The zero value matches the
// application's initial state, which is to follow the system.
var macAppAppearance string

type nativeWindow struct {
	// ax is the NSAccessibility adapter serving this window, or nil until an assistive technology has asked about it.
	ax   *cocoa.AXAdapter
	wnd  cocoa.Window
	view cocoa.View
	// covered is true from the exit made for the pointer leaving the window's content until it comes back; see
	// macCoveredMove and macMouseExited.
	covered bool
	// number is the window's number, which identifies it to the window server, or 0 until macNumber reads it.
	number int
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
	cocoa.WindowFrameMouseDownCallback = macFrameMouseDown
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
			w.nativeUpdateCursorImage()
		} else {
			slog.Warn("received window cursor update callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseEnterCallback = func(macWnd cocoa.Window, pt geom.Point, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.macMouseEntered(pt, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse enter callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseExitCallback = func(macWnd cocoa.Window) {
		if w := macFindWindow(macWnd); w != nil {
			w.macMouseExited()
		} else {
			slog.Warn("received window mouse exit callback for unknown window", "window", macWnd)
		}
	}
	cocoa.WindowMouseMovedCallback = func(macWnd cocoa.Window, pt geom.Point, mods uint) {
		if w := macFindWindow(macWnd); w != nil {
			w.macMouseMoved(pt, macTranslateModifiers(cocoa.EventModifierFlags(mods)))
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
	cocoa.WindowDragSourceFinishedCallback = macDragSourceFinished
}

// macFrameMouseDown is the WindowFrameMouseDownCallback: a press on macWnd's title bar or frame. It lands outside
// every menu, so the window's open menus close at once rather than only once the press has moved the window.
func macFrameMouseDown(macWnd cocoa.Window) {
	if w := macFindWindow(macWnd); w != nil && w.root != nil {
		w.root.closeMenuStackStoppingAt(nil)
	}
}

// macMouseMoved handles a mouse-moved or mouse-dragged event AppKit delivered to the window at pt, in its content
// coordinates. AppKit sends every mouse-moved event to the key window, wherever the pointer is, so the owner of an open
// menu stack is sent the moves over its popup menu windows, and a key window those over any window above it: another
// of the application's shown or floating over it, or another application's. Such a move is dropped rather than
// hit-tested against the panels hidden under the other window, which would get crossings, tooltips and the cursor meant
// for it, and the panel the pointer left is exited and the tooltip taken down, as the crossing events of the other
// platforms do. A drag goes to the window holding the press wherever the pointer is, as on the other platforms.
func (w *Window) macMouseMoved(pt geom.Point, mods mod.Modifiers) {
	if w.macCoveredMove(!w.inMouseDown && w.macCovered(w.nativePointToScreen(pt))) {
		return
	}
	w.mouseMovedOrDragged(pt, mods)
}

// macCoveredMove records whether the mouse-moved event being handled is over another window (see macCovered), and
// returns covered. The first of a run of such moves exits whatever the pointer left and takes down the tooltip (see
// pointerLeft), as the crossing event of the other platforms does, unless macMouseExited already has; the rest are
// dropped without that, since the window's MouseExitCallback may keep the panel it is told about, and would otherwise
// be told of the same crossing on every move.
func (w *Window) macCoveredMove(covered bool) bool {
	if !covered {
		w.wnd.covered = false
		return false
	}
	if !w.wnd.covered {
		w.wnd.covered = true
		w.pointerLeft()
	}
	return true
}

// macMouseEntered handles AppKit's report, through the window's tracking area, that the pointer entered its content,
// which starts a new crossing for macCoveredMove.
func (w *Window) macMouseEntered(pt geom.Point, mods mod.Modifiers) {
	w.wnd.covered = false
	w.nativeUpdateCursorImage()
	w.mouseEnter(pt, mods)
}

// macMouseExited handles AppKit's report, through the window's tracking area, that the pointer left its content. It
// exits whatever the pointer left and takes down the tooltip (see pointerLeft) unless macCoveredMove already has for a
// move over another window, and keeps the moves after it from doing so again.
func (w *Window) macMouseExited() {
	w.nativeUpdateCursorImage()
	if !w.wnd.covered {
		w.wnd.covered = true
		w.pointerLeft()
	}
}

// macWindowNumberAt returns the number of the window, of any application, that is frontmost at pt, in the space window
// rects use, or 0 for none. It is replaceable by the tests, which have no windows on the screen.
var macWindowNumberAt = func(pt geom.Point) int {
	return cocoa.WindowNumberAtPoint(geom.NewPoint(pt.X, macTransformY(pt.Y)))
}

// macCovered returns true if pt, in the space window rects use, is over a window other than this one: one of its popup
// menu windows, another of the application's windows ordered above it, another application's, or no window at all. Its
// tooltip window does not cover it: that window ignores mouse events, so the window server's hit test passes over it.
func (w *Window) macCovered(pt geom.Point) bool {
	return macWindowNumberAt(pt) != w.macNumber()
}

// macNumber returns the window's number, which identifies it to the window server, read from the window the first time
// it is asked for.
func (w *Window) macNumber() int {
	if w.wnd.number == 0 {
		w.wnd.number = w.wnd.wnd.Number()
	}
	return w.wnd.number
}

// macDragSourceFinished is the WindowDragSourceFinishedCallback: AppKit's report that a drag begun from macWnd's view
// has ended.
func macDragSourceFinished(macWnd cocoa.Window) {
	switch w := macFindWindow(macWnd); {
	case w != nil:
		w.dragSourceFinished()
	case dragSource != nil:
		// The source was disposed while its drag was in progress, as happens when the drop closes it (a tab moved out
		// of a window it was the last of, say), so it is no longer in the list the lookup walks. AppKit reports the
		// session's end to the source view all the same, and this is that report: wind the drag up so that the cleanup
		// runs and the drag is no longer recorded as in progress, as the other platforms do unconditionally once their
		// drag loop returns.
		dragSource.dragSourceFinished()
	default:
		slog.Warn("received window drag source finished callback for unknown window", "window", macWnd)
	}
}

func (w *Window) nativeInit() error {
	if w.isPopup() {
		return w.macInitPopup()
	}
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
	w.nativeUpdateFrameTheme()
	return nil
}

// macInitPopup creates the native window for a popup window, a menu's or a tooltip's: borderless, never key, above
// every ordinary window, shown without animation and left out of the accessibility hierarchy, its content view
// answering assistive technologies through the owner's, whose tree describes the popup's content. A tooltip window
// also ignores mouse events, so that the pointer passes over it to whatever lies beneath, and so does the hit test
// macCovered makes, which leaves the owner handling the moves made over it. It has no title and does not touch the
// application appearance the owner has already set.
func (w *Window) macInitPopup() error {
	nw := cocoa.NewPopupWindow(geom.NewRect(0, 0, 1, 1))
	if nw == 0 {
		return errs.New("unable to create native popup window")
	}
	if w.isTooltipWindow() {
		nw.SetIgnoresMouseEvents(true)
	}
	v := cocoa.NewPopupView(nw)
	nw.SetContentView(v)
	nw.MakeFirstResponder(v)
	nw.SetDelegate(cocoa.NewWindowDelegate())
	nw.SetAcceptsMouseMovedEvents(true)
	w.wnd.wnd = nw
	w.wnd.view = v
	cocoa.SetAXViewAlias(v, w.owner.wnd.view)
	return nil
}

func (w *Window) nativeSetTitle(title string) {
	w.wnd.wnd.SetTitle(title)
}

func (w *Window) nativeSetTitleIcons(_images []*image.NRGBA) {
	// macOS doesn't have window icons, so just ignore this.
}

func (w *Window) nativeDisplay() *Display {
	return BestDisplayForRect(w.nativeFrameRect())
}

func (w *Window) nativeFrameRect() geom.Rect {
	r := w.wnd.wnd.Frame()
	r.Y = macTransformY(r.Bottom())
	return r
}

func (w *Window) nativeFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	contentRect.Y = macTransformY(contentRect.Bottom())
	frameRect := w.wnd.wnd.FrameRectForContentRect(contentRect)
	frameRect.Y = macTransformY(frameRect.Bottom())
	return frameRect
}

func (w *Window) nativeEnsureOnDisplay() {
	frameRect := w.nativeFrameRect()
	revisedRect := w.nativeDisplay().FitRectOnto(frameRect)
	if frameRect != revisedRect {
		w.SetFrameRect(revisedRect)
	}
}

func (w *Window) nativeContentRect() geom.Rect {
	r := w.wnd.wnd.ContentRectForFrameRect(w.wnd.wnd.Frame())
	r.Y = macTransformY(r.Bottom())
	return r
}

func (w *Window) nativePointToScreen(pt geom.Point) geom.Point {
	return w.nativeContentRect().Point.Add(pt)
}

func (w *Window) nativePointFromScreen(pt geom.Point) geom.Point {
	return pt.Sub(w.nativeContentRect().Point)
}

func (w *Window) nativeContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	frameRect.Y = macTransformY(frameRect.Bottom())
	contentRect := w.wnd.wnd.ContentRectForFrameRect(frameRect)
	contentRect.Y = macTransformY(contentRect.Bottom())
	return contentRect
}

func (w *Window) nativeSetContentRect(rect geom.Rect) {
	rect.Y = macTransformY(rect.Bottom())
	w.wnd.wnd.SetFrame(w.wnd.wnd.FrameRectForContentRect(rect))
}

func (w *Window) nativeCurrentKeyModifiers() mod.Modifiers {
	return macModifiersFromEventModifierFlags(cocoa.CurrentModifierFlags())
}

// macCursorHidden is true while the cursor is hidden through macHideCursor. AppKit hides it for the whole application
// rather than for a window, so a window applying its cursor without hiding it undoes a hide another made, as a popup
// menu window does when the pointer is over it after typing in its owner hid the cursor.
var macCursorHidden bool

// macHideCursor and macShowCursor are cocoa.HideCursor and cocoa.ShowCursor, replaceable by the tests, which run off
// the main thread.
var (
	macHideCursor = cocoa.HideCursor
	macShowCursor = cocoa.ShowCursor
)

func (w *Window) nativeUpdateCursorImage() {
	if w.cursorHidden {
		if !macCursorHidden {
			macHideCursor()
			macCursorHidden = true
		}
	} else {
		if macCursorHidden {
			macShowCursor()
			macCursorHidden = false
		}
		if w.cursor != nil {
			w.cursor.cursor.Set()
		} else {
			cocoa.ArrowCursor().Set()
		}
	}
}

// nativeCursorInContentArea reports whether the pointer is over the window's content, which it is not while over
// another window above it (see macCovered): the window would otherwise show the cursor of a panel hidden under one of
// its own popup menu windows, which the popup, whose own cursor is the unchanged arrow, never puts back.
func (w *Window) nativeCursorInContentArea() bool {
	loc := w.wnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.wnd.view.Frame()
	if !w.wnd.view.MouseInRect(loc, frame) {
		return false
	}
	return !w.macCovered(w.nativePointToScreen(geom.NewPoint(loc.X, frame.Height-loc.Y)))
}

func (w *Window) nativeCursorPosition() geom.Point {
	loc := w.wnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.wnd.view.Frame()
	return geom.NewPoint(loc.X, frame.Height-loc.Y)
}

func (w *Window) nativeBackingScale() geom.Point {
	return w.wnd.view.BackingScale()
}

func (w *Window) nativeMinimize() {
	w.wnd.wnd.Miniaturize()
}

func (w *Window) nativeMaximize() {
	w.wnd.wnd.Zoom()
}

func (w *Window) nativeAcquireFocusAndBringToFront() {
	cocoa.ActivateIgnoringOtherApps()
	w.wnd.wnd.MakeKeyAndOrderFront()
}

// nativeCancelMouseCapture is a no-op: AppKit routes a press to the view that took it until the release, with nothing
// for the application to hand back. The release itself, when it arrives, is dropped by mouseUp, since the button has
// already been recorded as up.
func (w *Window) nativeCancelMouseCapture() {
}

// macMenuStackWindow is the window whose open menu stack the monitor installed by nativeMenuStackChanged serves, or nil
// while none is installed.
var macMenuStackWindow *Window

// macInstallMenuStackMonitor and macRemoveMenuStackMonitor are cocoa.InstallMenuStackMonitor and
// cocoa.RemoveMenuStackMonitor, replaceable by the tests, which run off the main thread.
var (
	macInstallMenuStackMonitor = cocoa.InstallMenuStackMonitor
	macRemoveMenuStackMonitor  = cocoa.RemoveMenuStackMonitor
)

// nativeMenuStackChanged watches, while the window's menu stack is open, for the presses that close native menus but
// leave the window key, and so would otherwise leave its menus open: one on the Dock, on a menu bar extra or in
// Notification Center, and one on the application's own menu bar, which no event monitor sees and which is reported as
// the bar's menu begins tracking instead (see cocoa.InstallMenuStackMonitor). One on another application's window is
// seen too, and closes the menus as it is made rather than once that application's activation takes the key status
// from the window. A press in the window itself or in one of its popup menu windows is left to their root panels,
// which close whatever menus it makes obsolete. A change of the active Space closes the menus too, since the popup
// windows join every Space and would otherwise follow the user to the new one without their owner. A stack opened for
// another window takes the monitor over, and only the window it serves takes it down.
func (w *Window) nativeMenuStackChanged(open bool) {
	if !open {
		if macMenuStackWindow == w {
			macMenuStackWindow = nil
			macRemoveMenuStackMonitor()
		}
		return
	}
	macMenuStackWindow = w
	macInstallMenuStackMonitor(func(macWnd cocoa.Window) {
		if !w.macPressWithinMenuStack(macWnd) && w.root != nil {
			w.root.closeMenuStackStoppingAt(nil)
		}
	})
}

// macPressWithinMenuStack returns true if a press in macWnd, as the menu stack monitor reports it, is one the root
// panels of w's menu stack handle: one in w itself or in one of its popup menu windows. A press in any other window, or
// in none of the application's (0), is not.
func (w *Window) macPressWithinMenuStack(macWnd cocoa.Window) bool {
	if macWnd == 0 {
		return false
	}
	other := macFindWindow(macWnd)
	return other != nil && other.menuOwner() == w
}

// nativeUpdateFrameTheme makes the application appearance follow an explicit SetThemeMode() choice, so that the frame,
// and with it everything else AppKit draws for the application, such as its menus and alerts, is dark or light along
// with the content rather than with the system. thememode.Auto clears the appearance, which hands the choice back to
// the system. The appearance belongs to the application rather than the window, so it is set only when it changes.
func (w *Window) nativeUpdateFrameTheme() {
	name := macAppearanceNameFor(CurrentThemeMode())
	if name == macAppAppearance {
		return
	}
	macAppAppearance = name
	cocoa.SetAppAppearance(name)
}

// macAppearanceNameFor returns the application appearance that an explicit theme mode asks for, or an empty string for
// thememode.Auto, which leaves the appearance to the system.
func macAppearanceNameFor(mode thememode.Enum) string {
	switch mode {
	case thememode.Dark:
		return cocoa.AppearanceNameDarkAqua
	case thememode.Light:
		return cocoa.AppearanceNameAqua
	default:
		return ""
	}
}

func (w *Window) nativeVisible() bool {
	return w.wnd.wnd.Visible()
}

func (w *Window) nativeShow() {
	// Order the window front without making it key, matching the other platforms, where Show() makes the window
	// visible without giving it the focus. ToFront() layers nativeAcquireFocusAndBringToFront on top of this for the
	// show-and-focus behavior.
	w.wnd.wnd.OrderFront()
}

func (w *Window) nativeHide() {
	w.wnd.wnd.OrderOut()
}

func (w *Window) nativeStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
	nrgba, r := macDragImageAndFrame(img, origin)
	r.Y = w.wnd.view.Frame().Height - r.Height - r.Y
	if !w.wnd.view.BeginDraggingSession(nrgba, r, opMask, data...) {
		// No session was begun (see cocoa.dragSessionEvent), so AppKit will never report one ending. Wind the drag up
		// here instead, so that the source's cleanup runs and nothing goes on believing a drag is in progress.
		w.dragSourceFinished()
	}
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

func (w *Window) nativeUpdateRegisteredDragTypes(types []*uti.DataType) {
	w.wnd.view.UnregisterDraggedTypes()
	if len(types) != 0 {
		w.wnd.view.RegisterDraggedTypes(types)
	}
}

func (w *Window) nativeDestroy() {
	w.glCtx.nativeDestroy()
	if w.wnd.wnd != 0 {
		w.wnd.wnd.OrderOut()
		if delegate := w.wnd.wnd.Delegate(); delegate != 0 {
			w.wnd.wnd.SetDelegate(0)
			delegate.Release()
		}
		if w.isPopup() {
			cocoa.ClearAXViewAlias(w.wnd.view)
		}
		w.wnd.view.Release()
		w.wnd.wnd.Close()
		w.wnd.wnd = 0
	}
	// Nothing polls events here to let AppKit catch up: that would dispatch whatever is queued from inside the
	// caller, which is often an event handler, such as the press that closed a popup menu, with its release queued
	// behind it. The event loop catches up once the handler returns.
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
