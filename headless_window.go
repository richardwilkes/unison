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
	"slices"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// headlessWindow is the stand-in for an operating system window. There are no decorations, so the frame rect and the
// content rect are the same rectangle, expressed in the screen's logical coordinate space.
type headlessWindow struct {
	hs *headlessState
	w  *Window
	// frame holds a premultiplied copy of the pixels the window last presented, i.e. what would be on the screen. It
	// is nil until the window has been drawn at least once.
	frame     *image.RGBA
	cursor    *Cursor
	title     string
	icons     []*image.NRGBA
	dragTypes []*uti.DataType
	rect      geom.Rect
	// restoreRect is the content rect to go back to when a maximized window is restored.
	restoreRect geom.Rect
	visible     bool
	destroyed   bool
}

// headlessWindowFor returns the headless backing of w, or nil if w has none (it belongs to the OS or was hand-built by
// a test). The backing stays attached after the window has been destroyed, so that every api* call made on a dead
// window during a headless session still lands here rather than falling through to the OS backend.
func headlessWindowFor(w *Window) *headlessWindow {
	if w == nil || w.wnd == nil {
		return nil
	}
	return w.wnd.hw
}

// newWindow creates the headless backing for w. The initial rect is one point square rather than empty, matching what
// cocoa.NewWindow starts a real window at, so the first prepareCanvas is never asked for a zero-sized surface. The
// window does not join the z-order stack until it is shown.
func (s *headlessState) newWindow(w *Window) *headlessWindow {
	return &headlessWindow{
		hs:   s,
		w:    w,
		rect: geom.NewRect(0, 0, 1, 1),
	}
}

// destroy detaches the window from the session. It must tolerate being called more than once: finishQuit destroys
// every window directly rather than through Dispose(), and a modal window that was closed that way is destroyed a
// second time by the Dispose() that RunModal defers. The OS backends never see that sequence, since the native
// quitting() exits the process before finishQuit runs.
func (hw *headlessWindow) destroy() {
	if hw.destroyed {
		return
	}
	hw.destroyed = true
	w := hw.w
	// A destroyed window is invalid and off the screen whether or not it came here through Dispose(), which is the
	// only path that marks a window invalid on its own. The finishQuit path does not, and a window it tore down that
	// still reported itself valid would go on being drawn, go on being composited into a capture, and — more to the
	// point — go on re-arming every timer that checks IsValid() before arming the next one, such as a Field's caret
	// blink, for as long as the process lives. The safety argument for timers that outlive a session, in the header of
	// headless.go, rests on this.
	w.valid = false
	hw.visible = false
	if w.root != nil {
		// Bump the tooltip sequence so a tooltip timer that is already armed does nothing when it fires.
		w.ClearTooltip()
	}
	hs := hw.hs
	hs.stack = slices.DeleteFunc(hs.stack, func(other *Window) bool { return other == w })
	// The grab and the hover go the way they do in hide(), with one difference: no exit is delivered to this window,
	// since it is no longer valid and has no root panel left to route one through. The window revealed underneath it is
	// still entered, so hover state read between the dispose and the next pointer event is right rather than merely
	// self-correcting.
	hw.releasePointer(false)
	if hs.focused == w {
		// Hand the focus back to the frontmost window that can hold it, as hide() does and as the platforms do when the
		// key window closes. This cannot be left to Window.Dispose(), which refocuses windowList[0] only when the
		// window was the ActiveWindow(): a transient window never is, so disposing of one that had taken the focus
		// with ToFront() would otherwise leave nothing focused and every key event dropped until the next click. The
		// two do not fight over it: the window chosen here is at the front of the window list by the time Dispose()
		// looks, so its ToFront() finds the focus already where it would have put it. The focus is cleared before the
		// handoff rather than resigned by it, since the window is mid-destruction and, on the Dispose() path, has
		// already had its content torn out — which means lostFocus() never runs for it, so the flag it would have
		// cleared is cleared here, or the disposed window would go on reporting Focused() true forever. The native
		// backends get their resign notification while the delegate is still attached, so lostFocus() runs there.
		hs.focused = nil
		w.focused = false
		hs.setFocus(hs.topVisibleNonTransient())
	}
	// Last, so that a drag ending here finds the window already out of the z-order and hands the pointer to whatever
	// is now underneath it.
	hs.dragWindowDestroyed(w)
}

// releasePointer takes the window out of the input router's pointer state, for a window that is leaving the screen:
// the grab a press in it installed is released along with the buttons that press was holding, and if the pointer was
// in it, it is exited — when exit is true and the window is still valid — and whatever the pointer is now over is
// entered instead. hide(), minimize() and destroy() all need this, since a window that is hidden, minimized or gone is
// skipped by the same routing checks and would otherwise go on receiving every motion and hold the buttons down for
// the rest of the session.
func (hw *headlessWindow) releasePointer(exit bool) {
	hs := hw.hs
	if hs.capture == hw.w {
		// A press cannot be completed in a window that is no longer on the screen, so the grab it installed goes with
		// it and the buttons it was holding are forgotten — the same reasoning finishDrag applies to the press a drag
		// consumed. Leaving the grab in place would go on handing every motion to a window that is not there to
		// receive it, and leaving the buttons behind would have them recorded as down until some release happened to
		// arrive, which nothing guarantees: the window may well be disposed of before one ever does. buttonReleased
		// does its own bookkeeping only for a window that holds the pointer, and every external drag would be refused
		// on the strength of a press that no longer exists.
		hs.capture = nil
		clear(hs.buttons)
	}
	if hs.hover == hw.w {
		// A window server generates a crossing when a window unmaps or iconifies — an X11 LeaveNotify — so the window
		// is told the pointer has left it, since the panels within it still believe it is in them. updateHover only
		// exits the window it finds in hover, so that exit is delivered here, before hover is cleared, and updateHover
		// then enters whatever the pointer has been left over.
		if exit && hw.w.IsValid() {
			hw.w.mouseExit()
		}
		hs.hover = nil
		if hs.drag == nil {
			// While a drag is in flight the pointer belongs to it rather than to the hover, and finishDrag performs the
			// entry when the drag hands the pointer back.
			hs.updateHover(hs.pointer, hs.lastMods)
		}
	}
}

func (hw *headlessWindow) setTitle(title string) {
	hw.title = title
}

func (hw *headlessWindow) setTitleIcons(images []*image.NRGBA) {
	hw.icons = images
}

// display returns the session's one and only display. Multi-display configurations are out of scope.
func (hw *headlessWindow) display() *Display {
	return hw.hs.display
}

func (hw *headlessWindow) frameRect() geom.Rect {
	return hw.rect
}

func (hw *headlessWindow) contentRect() geom.Rect {
	return hw.rect
}

// frameRectForContentRect is the identity, since a headless window has no decorations around its content.
func (hw *headlessWindow) frameRectForContentRect(contentRect geom.Rect) geom.Rect {
	return contentRect
}

// contentRectForFrameRect is the identity, since a headless window has no decorations around its content.
func (hw *headlessWindow) contentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	return frameRect
}

func (hw *headlessWindow) ensureOnDisplay() {
	revised := hw.display().FitRectOnto(hw.rect)
	if revised != hw.rect {
		// Go back through the window rather than assigning the rect directly, so the size is still run past the
		// window's minimum and maximum and the resize and move callbacks fire. This mirrors nativeEnsureOnDisplay.
		hw.w.SetFrameRect(revised)
	}
}

func (hw *headlessWindow) setContentRect(rect geom.Rect) {
	previous := hw.rect
	hw.rect = rect
	hw.w.MarkForRedraw()
	// Report the change synchronously, the way AppKit does when a window's frame is set. X11 is asynchronous, so on
	// that platform these callbacks instead arrive later, when the ConfigureNotify comes back.
	if previous.Size != rect.Size {
		hw.w.resized()
	}
	if previous.Point != rect.Point {
		hw.w.moved()
	}
}

func (hw *headlessWindow) currentKeyModifiers() mod.Modifiers {
	return hw.hs.lastMods
}

// updateCursorImage records the cursor the window resolved to. There is nothing to draw it with, so what a test can
// observe is which cursor was chosen, both per window and for the screen as a whole.
func (hw *headlessWindow) updateCursorImage() {
	hw.cursor = hw.w.cursor
	hw.hs.cursor = hw.w.cursor
}

func (hw *headlessWindow) cursorInContentArea() bool {
	return hw.hs.pointer.In(hw.rect)
}

// cursorPosition returns the pointer position in the window's own coordinate space, which, as on the real platforms,
// is still reported when the pointer is outside the window.
func (hw *headlessWindow) cursorPosition() geom.Point {
	return hw.hs.pointer.Sub(hw.rect.Point)
}

func (hw *headlessWindow) backingScale() geom.Point {
	return geom.NewPoint(hw.hs.cfg.Scale, hw.hs.cfg.Scale)
}

func (hw *headlessWindow) minimize() {
	if hw.w.minimized {
		hw.restore()
		return
	}
	hw.w.minimized = true
	if hw.w.MinimizedCallback != nil {
		SafeCall(func() { hw.w.MinimizedCallback(true) })
	}
	// A minimized window is skipped by input routing and by Capture(), exactly as a hidden one is, so it can hold
	// neither the pointer nor the focus: the grab and the hover go the way they do in hide(), with the window exited
	// as a window server's LeaveNotify on iconify would have it, and the focus is handed on.
	hw.releasePointer(true)
	if hw.hs.focused == hw.w {
		hw.hs.setFocus(hw.hs.topVisibleNonTransient())
	}
}

// restore takes a minimized window back out of that state and, if it is on the screen, activates it. Activation is
// what both platforms do on restoration: X11 deiconifies the window and the window manager then focuses it as it maps,
// and AppKit's deminiaturize makes it the key window again. That is the same pair of steps acquireFocusAndBringToFront
// takes, minus the show(): the window never stopped being visible, it was merely being skipped for as long as it was
// minimized. A window that was hidden while minimized is left alone, since restoring it does not put it back on the
// screen.
func (hw *headlessWindow) restore() {
	hw.w.minimized = false
	if hw.w.MinimizedCallback != nil {
		SafeCall(func() { hw.w.MinimizedCallback(false) })
	}
	if hw.visible {
		hw.hs.raise(hw.w)
		hw.hs.setFocus(hw.w)
	}
}

func (hw *headlessWindow) maximize() {
	maximized := !hw.w.maximized
	hw.w.maximized = maximized
	if hw.w.MaximizedCallback != nil {
		SafeCall(func() { hw.w.MaximizedCallback(maximized) })
	}
	target := hw.restoreRect
	if maximized {
		hw.restoreRect = hw.rect
		target = hw.display().Usable
	}
	// Route through the window so the resize and move callbacks fire, exactly as they do when a window manager
	// maximizes a real window.
	hw.w.SetContentRect(target)
}

func (hw *headlessWindow) isVisible() bool {
	return hw.visible
}

func (hw *headlessWindow) show() {
	if hw.visible {
		return
	}
	hw.visible = true
	hw.hs.raise(hw.w)
	hw.w.MarkForRedraw()
	// Deliberately no focus change: this is the equivalent of orderFront on macOS and MapWindow on X11. ToFront()
	// layers acquireFocusAndBringToFront on top of it when the window should also take the focus.
}

func (hw *headlessWindow) hide() {
	if !hw.visible {
		return
	}
	hw.visible = false
	hs := hw.hs
	hs.stack = slices.DeleteFunc(hs.stack, func(other *Window) bool { return other == hw.w })
	hw.releasePointer(true)
	if hs.focused == hw.w {
		hs.setFocus(hs.topVisibleNonTransient())
	}
}

// acquireFocusAndBringToFront is the other half of ToFront(): the window is shown if it was not already, restored if
// it was minimized, raised to the top of the z-order, and given the keyboard focus. show() is used rather than setting
// the flag, so that a window reaching this without having been shown still joins the z-order and gets its first draw.
// The restoration is what AppKit's makeKeyAndOrderFront: does to a miniaturized window, and it is needed because a
// minimized window is still visible: show() would leave it minimized, and raising and focusing it in that state would
// make a focused window that receives every key event yet is skipped by hit-testing, by Capture() and by WindowAt().
// This is reached without anything exotic, since Window.Dispose() calls ToFront() on the front of the window list and
// RunModal's defer calls it on the window that was active.
func (hw *headlessWindow) acquireFocusAndBringToFront() {
	hw.show()
	if hw.w.minimized {
		hw.restore()
	}
	hw.hs.raise(hw.w)
	hw.hs.setFocus(hw.w)
}

// cancelMouseCapture releases the grab a press in this window installed, along with the buttons that press was
// holding, and then puts the pointer back over whatever it is actually above: the tail of buttonReleased, without the
// delivery. The window stays on the screen, so unlike releasePointer there is no exit to deliver for it.
func (hw *headlessWindow) cancelMouseCapture() {
	hs := hw.hs
	if hs.capture != hw.w {
		return
	}
	hs.capture = nil
	clear(hs.buttons)
	hs.updateHover(hs.pointer, hs.lastMods)
}

func (hw *headlessWindow) updateRegisteredDragTypes(types []*uti.DataType) {
	hw.dragTypes = types
}

// startDrag runs the source side of a drag & drop as a nested event loop, which is what the Linux and Windows backends
// do: the drag owns the thread until it ends, and the cleanup the source registered runs when it returns. The events
// the drag is made of — the motions that carry it and the release that drops it — are the ones already queued behind
// the press that led here, and this loop consumes them exactly as the outer one would have.
func (hw *headlessWindow) startDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
	defer hw.w.dragSourceFinished()
	hs := hw.hs
	if hs.drag != nil {
		// Nothing can start a second drag: the pointer is already spoken for. The platforms cannot even be asked, so
		// there is no established behavior to copy — record it for Errors() and leave the drag in progress alone.
		hs.recordError(errs.New("a drag & drop session is already in progress"))
		return
	}
	hs.beginDrag(hw.w, img, origin, opMask, data)
	for hs.drag != nil && !hs.terminated.Load() {
		processEvents()
	}
}

// presentCPUPixels adopts the freshly rendered frame as what is now "on the screen" for this window.
func (hw *headlessWindow) presentCPUPixels(pixels *raster.Pixmap) {
	// RGBA8888Bytes hands back a newly allocated copy in memory byte order (R, G, B, A per pixel) with the color
	// channels already multiplied by alpha, which is exactly image.RGBA's layout, so the bytes are adopted as they
	// are. The copy matters: the pixmap itself is reused by the next draw.
	hw.frame = &image.RGBA{
		Pix:    pixels.RGBA8888Bytes(),
		Stride: 4 * int(pixels.Width),
		Rect:   image.Rect(0, 0, int(pixels.Width), int(pixels.Height)),
	}
}

// The z-order stack runs back to front: the last entry is the topmost window. Only windows that are currently visible
// are in it. All of the following are UI thread only.

// raise moves w to the top of the z-order. Floating windows are kept above non-floating ones, matching the window
// levels the platforms give them, so raising a normal window slides it in below any floating ones rather than over
// them.
func (s *headlessState) raise(w *Window) {
	s.stack = slices.DeleteFunc(s.stack, func(other *Window) bool { return other == w })
	if w.floating {
		s.stack = append(s.stack, w)
		return
	}
	i := len(s.stack)
	for i > 0 && s.stack[i-1].floating {
		i--
	}
	s.stack = slices.Insert(s.stack, i, w)
}

// windowAt returns the topmost window containing pt, or nil if there is none. pt is in the screen's logical
// coordinate space.
func (s *headlessState) windowAt(pt geom.Point) *Window {
	for i := len(s.stack) - 1; i >= 0; i-- {
		w := s.stack[i]
		if hw := headlessWindowFor(w); hw != nil && hw.visible && !w.minimized && pt.In(hw.rect) {
			return w
		}
	}
	return nil
}

// topVisibleNonTransient returns the frontmost window that can hold the focus, or nil if there is none. Transient
// windows (menus, tooltips) are never considered active, so they are skipped.
func (s *headlessState) topVisibleNonTransient() *Window {
	for _, w := range slices.Backward(s.stack) {
		if hw := headlessWindowFor(w); hw != nil && hw.visible && !w.minimized && !w.transient {
			return w
		}
	}
	return nil
}

// setFocus moves the keyboard focus to next, which may be nil.
func (s *headlessState) setFocus(next *Window) {
	if s.focused == next {
		return
	}
	previous := s.focused
	s.focused = next
	// Resign before become, the order AppKit uses, so the outgoing window has finished giving up the focus — which
	// includes synthesizing a mouse up for any button still held — before the incoming one starts claiming it.
	if previous != nil && previous.IsValid() {
		previous.lostFocus()
	}
	if next != nil {
		// gainedFocus reorders windowList and refuses the focus itself when the window is blocked by a modal, so
		// there is nothing further to decide here.
		next.gainedFocus()
	}
}
