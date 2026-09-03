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
	"sync/atomic"
	"time"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// Everything the platform-neutral part of this package needs from the operating system — starting and stopping the
// application, pumping events, creating and manipulating windows, displays, cursors, the clipboard, menus and file
// dialogs — is expressed as an api* function or method. This file is the complete list of them and is the only place
// that decides where each one goes. There are two possible destinations:
//
//   - The OS implementation, named native* and written once per GOOS in the _darwin.go, _linux.go and _windows.go
//     files. Those are selected by file name suffix; the few files shared by more than one GOOS (console_other.go,
//     menu_bar_other.go and menu_factory_other.go) carry a //go:build constraint instead. There is no cgo anywhere.
//   - An in-memory headless backend, used for testing user interfaces with no display attached. It stands in for the
//     screen, the window server and the OS event queue, so a test can run the real event loop, inject input and read
//     back rendered pixels on a machine that has no windowing system at all.
//
// Confining the choice to these wrappers keeps both sides simple: the platform-neutral code above never learns that a
// headless backend exists, and the OS code below never learns that it can be bypassed. Each wrapper is therefore a
// one-liner, dispatching on whether a headless session is currently active — or passing straight through, for the
// few things that are about the process rather than the screen and so have no headless side at all.
//
// The native code also has native* helpers of its own that it calls among itself, such as nativePollEvents, which the
// platform-neutral code never reaches and which therefore have no wrapper here. TestAPIWrappersLiveInPlatformAPI keeps
// the list in this file complete by checking that no api* function is declared anywhere else.
//
// Per-window platform state is reached the same way. Window.wnd is an *apiWindow and Window.glCtx is an *apiGLContext,
// each of which embeds the OS struct, so the native code's field access (w.wnd.view, w.wnd.id, w.glCtx.ctx, ...) is
// promoted through the embedding and compiles unchanged.

// apiWindow is the per-window platform state held in Window.wnd.
type apiWindow struct {
	// hw is non-nil only for windows created while a headless session was active and is what every Window.api* method
	// dispatches on. A nil hw therefore means "this window belongs to the OS", which is also what hand-built windows
	// such as the &Window{wnd: &apiWindow{}} used by tests get, leaving them on the native path exactly as before.
	hw *headlessWindow
	nativeWindow
}

// apiGLContext is the per-window OpenGL context state held in Window.glCtx.
type apiGLContext struct {
	nativeGLContext
	// headless is true for a context belonging to a window created while a headless session was active and is what
	// every apiGLContext method dispatches on, so a context keeps its backend for life just as its window does.
	headless bool
}

// headlessActive holds the headless session that is currently standing in for the operating system, or nil when the
// real one is in use. It is atomic because apiPostEmptyEvent is reached from arbitrary goroutines (InvokeTask and the
// timers behind InvokeTaskAfter), while the session is published and retired by the UI thread.
var headlessActive atomic.Pointer[headlessState]

// activeHeadless returns the active headless session, or nil if the application is talking to the real operating
// system. It is safe to call from any goroutine.
func activeHeadless() *headlessState {
	return headlessActive.Load()
}

// Application startup, shutdown and the event loop.

func apiBeginStartup() error {
	if hs := activeHeadless(); hs != nil {
		return hs.beginStartup()
	}
	return nativeBeginStartup()
}

func apiLateInit() {
	if hs := activeHeadless(); hs != nil {
		hs.lateInit()
		return
	}
	nativeLateInit()
}

func apiFinalFinishStartup() {
	if hs := activeHeadless(); hs != nil {
		hs.finalFinishStartup()
		return
	}
	nativeFinalFinishStartup()
}

func apiTerminate() error {
	if hs := activeHeadless(); hs != nil {
		return hs.terminate()
	}
	return nativeTerminate()
}

func apiBeep() {
	if hs := activeHeadless(); hs != nil {
		hs.beep()
		return
	}
	nativeBeep()
}

func apiIsColorModeTrackingPossible() bool {
	if hs := activeHeadless(); hs != nil {
		return hs.isColorModeTrackingPossible()
	}
	return nativeIsColorModeTrackingPossible()
}

func apiIsDarkModeEnabled() bool {
	if hs := activeHeadless(); hs != nil {
		return hs.isDarkModeEnabled()
	}
	return nativeIsDarkModeEnabled()
}

func apiDoubleClickInterval() time.Duration {
	if hs := activeHeadless(); hs != nil {
		return hs.doubleClickInterval()
	}
	return nativeDoubleClickInterval()
}

func apiWaitEvents() {
	if hs := activeHeadless(); hs != nil {
		hs.waitEvents()
		return
	}
	nativeWaitEvents()
}

func apiPostEmptyEvent() {
	if hs := activeHeadless(); hs != nil {
		hs.postEmptyEvent()
		return
	}
	nativePostEmptyEvent()
}

func apiWithAutoreleasePool(f func()) {
	if hs := activeHeadless(); hs != nil {
		hs.withAutoreleasePool(f)
		return
	}
	nativeWithAutoreleasePool(f)
}

// apiAttachConsole is a pass-through: attaching the process to its parent's console is about where the process's
// standard streams go, not about the screen, and a headless session has no reason to want it done any differently.
func apiAttachConsole() {
	nativeAttachConsole()
}

// Displays.

func apiPrimaryDisplay() *Display {
	if hs := activeHeadless(); hs != nil {
		return hs.primaryDisplay()
	}
	return nativePrimaryDisplay()
}

func apiAllDisplays() []*Display {
	if hs := activeHeadless(); hs != nil {
		return hs.allDisplays()
	}
	return nativeAllDisplays()
}

// apiUsableInWindowUnits returns the usable area of the display in the coordinate space used by window rects. Which
// space that is varies by platform, so the conversion belongs to the platform implementation. A headless session
// places windows in the display's own coordinate space, so there is nothing to convert. It dispatches on the display's
// own flag rather than on the currently active session, as the window and cursor methods do, so a display built by
// the OS before a session and read during one is still converted the way its backend requires.
func (d *Display) apiUsableInWindowUnits() geom.Rect {
	if d.headless {
		return d.Usable
	}
	return d.nativeUsableInWindowUnits()
}

// Cursors.

func apiNewCursor(src *cursorSource) *Cursor {
	if hs := activeHeadless(); hs != nil {
		return hs.newCursor(src)
	}
	return nativeNewCursor(src)
}

// apiDestroy dispatches on the cursor's own headless flag rather than on the currently active session, so a cursor
// keeps the backend it was created with for its whole life. A native cursor created before a session and destroyed
// during one — by syncBuiltInCursors on a theme change, or by finishQuit's loop over cursorList — would otherwise have
// its OS resource leaked.
func (c *Cursor) apiDestroy() {
	if c.headless {
		// Headless cursors are inert: there is no OS resource behind them to release.
		return
	}
	c.nativeDestroy()
}

// Clipboard.

func apiClipboardAvailableDataTypes() []string {
	if hs := activeHeadless(); hs != nil {
		return hs.clipboardAvailableDataTypes()
	}
	return nativeClipboardAvailableDataTypes()
}

func apiClipboardHasDataType(dataType *uti.DataType) bool {
	if hs := activeHeadless(); hs != nil {
		return hs.clipboardHasDataType(dataType)
	}
	return nativeClipboardHasDataType(dataType)
}

func apiClipboardGetData(dataType *uti.DataType) []byte {
	if hs := activeHeadless(); hs != nil {
		return hs.clipboardGetData(dataType)
	}
	return nativeClipboardGetData(dataType)
}

func apiClipboardSetData(data ...drag.Data) {
	if hs := activeHeadless(); hs != nil {
		hs.clipboardSetData(data...)
		return
	}
	nativeClipboardSetData(data...)
}

// Menus.

func apiNewDefaultMenuFactory() MenuFactory {
	if hs := activeHeadless(); hs != nil {
		return hs.newDefaultMenuFactory()
	}
	return nativeNewDefaultMenuFactory()
}

func apiQuitMenuTitle() string {
	if hs := activeHeadless(); hs != nil {
		return hs.quitMenuTitle()
	}
	return nativeQuitMenuTitle()
}

func apiAddAppMenuEntries(m Menu) {
	if hs := activeHeadless(); hs != nil {
		hs.addAppMenuEntries(m)
		return
	}
	nativeAddAppMenuEntries(m)
}

// File dialogs.

func apiNewOpenDialog() OpenDialog {
	if hs := activeHeadless(); hs != nil {
		return hs.newOpenDialog()
	}
	return nativeNewOpenDialog()
}

func apiNewSaveDialog() SaveDialog {
	if hs := activeHeadless(); hs != nil {
		return hs.newSaveDialog()
	}
	return nativeNewSaveDialog()
}

// Windows. Every one of these dispatches on the window's own hw field rather than on the currently active session, so
// a window keeps the backend it was created with for its whole life.

// apiInit is where a window picks its side: a window created while a headless session is active gets the headless
// backend, and one created at any other time gets the OS.
func (w *Window) apiInit() error {
	if hs := activeHeadless(); hs != nil {
		w.wnd.hw = hs.newWindow(w)
		if w.glCtx != nil {
			// A headless session forces CPU rendering, so nothing will ever ask this context for anything, but mark it
			// all the same: a context, like its window, answers for itself rather than asking what is active now.
			w.glCtx.headless = true
		}
		return nil
	}
	return w.nativeInit()
}

func (w *Window) apiDestroy() {
	if hw := w.wnd.hw; hw != nil {
		hw.destroy()
		return
	}
	w.nativeDestroy()
}

func (w *Window) apiSetTitle(title string) {
	if hw := w.wnd.hw; hw != nil {
		hw.setTitle(title)
		return
	}
	w.nativeSetTitle(title)
}

func (w *Window) apiSetTitleIcons(images []*image.NRGBA) {
	if hw := w.wnd.hw; hw != nil {
		hw.setTitleIcons(images)
		return
	}
	w.nativeSetTitleIcons(images)
}

func (w *Window) apiDisplay() *Display {
	if hw := w.wnd.hw; hw != nil {
		return hw.display()
	}
	return w.nativeDisplay()
}

func (w *Window) apiFrameRect() geom.Rect {
	if hw := w.wnd.hw; hw != nil {
		return hw.frameRect()
	}
	return w.nativeFrameRect()
}

func (w *Window) apiFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	if hw := w.wnd.hw; hw != nil {
		return hw.frameRectForContentRect(contentRect)
	}
	return w.nativeFrameRectForContentRect(contentRect)
}

func (w *Window) apiEnsureOnDisplay() {
	if hw := w.wnd.hw; hw != nil {
		hw.ensureOnDisplay()
		return
	}
	w.nativeEnsureOnDisplay()
}

func (w *Window) apiContentRect() geom.Rect {
	if hw := w.wnd.hw; hw != nil {
		return hw.contentRect()
	}
	return w.nativeContentRect()
}

func (w *Window) apiContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	if hw := w.wnd.hw; hw != nil {
		return hw.contentRectForFrameRect(frameRect)
	}
	return w.nativeContentRectForFrameRect(frameRect)
}

func (w *Window) apiSetContentRect(rect geom.Rect) {
	if hw := w.wnd.hw; hw != nil {
		hw.setContentRect(rect)
		return
	}
	w.nativeSetContentRect(rect)
}

// apiCurrentKeyModifiers is reached through Window.CurrentKeyModifiers, which is not guarded by IsValid(), from windows
// that were never initialized — the hand-built &Window{} some tests use — so it must not assume w.wnd exists. None of
// the native implementations look at w.wnd; only the headless dispatch needs it, and headlessWindowFor tolerates its
// absence.
func (w *Window) apiCurrentKeyModifiers() mod.Modifiers {
	if hw := headlessWindowFor(w); hw != nil {
		return hw.currentKeyModifiers()
	}
	return w.nativeCurrentKeyModifiers()
}

func (w *Window) apiUpdateCursorImage() {
	if hw := w.wnd.hw; hw != nil {
		hw.updateCursorImage()
		return
	}
	w.nativeUpdateCursorImage()
}

func (w *Window) apiCursorInContentArea() bool {
	if hw := w.wnd.hw; hw != nil {
		return hw.cursorInContentArea()
	}
	return w.nativeCursorInContentArea()
}

func (w *Window) apiCursorPosition() geom.Point {
	if hw := w.wnd.hw; hw != nil {
		return hw.cursorPosition()
	}
	return w.nativeCursorPosition()
}

func (w *Window) apiBackingScale() geom.Point {
	if hw := w.wnd.hw; hw != nil {
		return hw.backingScale()
	}
	return w.nativeBackingScale()
}

func (w *Window) apiMinimize() {
	if hw := w.wnd.hw; hw != nil {
		hw.minimize()
		return
	}
	w.nativeMinimize()
}

func (w *Window) apiMaximize() {
	if hw := w.wnd.hw; hw != nil {
		hw.maximize()
		return
	}
	w.nativeMaximize()
}

func (w *Window) apiAcquireFocusAndBringToFront() {
	if hw := w.wnd.hw; hw != nil {
		hw.acquireFocusAndBringToFront()
		return
	}
	w.nativeAcquireFocusAndBringToFront()
}

// apiCancelMouseCapture releases any hold on the pointer that a press in this window installed, without waiting for
// the release that would normally end it. See cancelPressesForModal.
func (w *Window) apiCancelMouseCapture() {
	if hw := w.wnd.hw; hw != nil {
		hw.cancelMouseCapture()
		return
	}
	w.nativeCancelMouseCapture()
}

func (w *Window) apiVisible() bool {
	if hw := w.wnd.hw; hw != nil {
		return hw.isVisible()
	}
	return w.nativeVisible()
}

func (w *Window) apiShow() {
	if hw := w.wnd.hw; hw != nil {
		hw.show()
		return
	}
	w.nativeShow()
}

func (w *Window) apiHide() {
	if hw := w.wnd.hw; hw != nil {
		hw.hide()
		return
	}
	w.nativeHide()
}

func (w *Window) apiStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
	if hw := w.wnd.hw; hw != nil {
		hw.startDrag(img, origin, opMask, data...)
		return
	}
	w.nativeStartDrag(img, origin, opMask, data...)
}

func (w *Window) apiUpdateRegisteredDragTypes(types []*uti.DataType) {
	if hw := w.wnd.hw; hw != nil {
		hw.updateRegisteredDragTypes(types)
		return
	}
	w.nativeUpdateRegisteredDragTypes(types)
}

func (w *Window) apiPresentCPUPixels(pixels *raster.Pixmap) {
	if hw := w.wnd.hw; hw != nil {
		hw.presentCPUPixels(pixels)
		return
	}
	w.nativePresentCPUPixels(pixels)
}

// OpenGL contexts. The native methods are defined on *nativeGLContext and reached here through the embedded field. Each
// of these dispatches on the context's own headless flag, as the window methods above dispatch on the window's hw, so a
// context created against the OS goes on talking to it however it is torn down. A headless session forces CPU rendering
// for its whole life, so NewWindow never creates a context and Window.draw never reaches the swap; these guards exist
// so the promise is explicit rather than depending on that reasoning holding.

func (c *apiGLContext) apiCreate(wnd *Window) error {
	if c.headless {
		return nil
	}
	return c.nativeCreate(wnd)
}

func (c *apiGLContext) apiMakeCurrent() {
	if c.headless {
		return
	}
	c.nativeMakeCurrent()
}

func (c *apiGLContext) apiReleaseCurrent() {
	if c.headless {
		return
	}
	c.nativeReleaseCurrent()
}

func (c *apiGLContext) apiSwapBuffers() {
	if c.headless {
		return
	}
	c.nativeSwapBuffers()
}

func (c *apiGLContext) apiDestroy() {
	if c.headless {
		return
	}
	c.nativeDestroy()
}
