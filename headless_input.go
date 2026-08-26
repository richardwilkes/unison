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
	"time"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/mod"
)

// This file is the input router: the part of the headless window server that decides which window an injected mouse or
// keyboard event belongs to, then calls the very same Window entry points the operating system backends call. Nothing
// below this line knows the events were made up rather than delivered by a window server.
//
// Everything here runs on the UI thread, reached from drainInput as one of the queued closures the driver in
// headless_screen.go posts, so none of the session state it reads and writes needs locking.
//
// Positions arrive in the screen's logical coordinate space and are handed on in the window's own space. That is a
// plain subtraction, since a headless window has no decorations and its frame is therefore its content.
//
// Drag & drop (headless_drag.go) diverts the pointer, button and wheel events for as long as a drag session is active,
// which is why each of pointerMoved, buttonPressed, buttonReleased and wheel begins by asking whether there is one.
// Key events are still routed normally, with the single exception of Escape, which abandons the drag.

// windowLocal converts a point in the screen's logical coordinate space into w's own.
func windowLocal(w *Window, pt geom.Point) geom.Point {
	if hw := headlessWindowFor(w); hw != nil {
		return pt.Sub(hw.rect.Point)
	}
	return pt
}

// pointerMoved delivers a pointer motion at pt.
//
// A window that has the pointer captured by a press in progress receives the motion no matter where pt is, which is the
// grab the platforms install for the duration of a press. Whether it arrives as a drag or as a move is the window's own
// decision, taken from its inMouseDown state, exactly as it is on the real platforms.
func (s *headlessState) pointerMoved(pt geom.Point, mods mod.Modifiers) {
	if s.drag != nil {
		s.dragMoved(pt, mods)
		return
	}
	s.pointer = pt
	s.lastMods = mods
	if s.capture != nil {
		s.capture.mouseMovedOrDragged(windowLocal(s.capture, pt), mods)
		return
	}
	if target, crossed := s.updateHover(pt, mods); target != nil && !crossed {
		target.mouseMovedOrDragged(windowLocal(target, pt), mods)
	}
}

// updateHover works out which window the pointer is over and, when that is not the one it was over before, synthesizes
// the crossing: the window being left is told the pointer has gone, and the window being entered is told where it
// arrived. Reports the window now under the pointer and whether a crossing happened, since a caller must not follow an
// entry with a motion — mouseEnter has already delivered the position.
func (s *headlessState) updateHover(pt geom.Point, mods mod.Modifiers) (target *Window, crossed bool) {
	target = s.windowAt(pt)
	if target == s.hover {
		return target, false
	}
	previous := s.hover
	s.hover = target
	// The window being left may have been disposed while the pointer was in it — that is one of the ways the pointer
	// comes to be somewhere else — and a disposed window has no root panel left to route the exit through.
	if previous != nil && previous.IsValid() {
		previous.mouseExit()
	}
	if target != nil {
		target.mouseEnter(windowLocal(target, pt), mods)
	}
	return target, true
}

// buttonPressed delivers a press of the given button at pt.
func (s *headlessState) buttonPressed(pt geom.Point, button int, mods mod.Modifiers) {
	if s.drag != nil {
		// Swallowed, and not even recorded: a drag is watching for the release that ends it and takes no interest in
		// anything else the buttons do, which is exactly how the platforms' drag loops behave.
		return
	}
	// Move first, so that the window under the pointer has been entered and the press reports the position that window
	// last saw. The platforms guarantee the same ordering, since a press is always preceded by the motion that took the
	// pointer to it.
	s.pointerMoved(pt, mods)
	target := s.capture
	if target == nil {
		target = s.windowAt(pt)
	}
	if target == nil {
		return
	}
	if !target.transient && s.focused != target {
		// Click to focus, before the press is delivered: mouseDown refuses to hand a press to the panels of a window
		// that is neither focused nor transient. A window that is blocked by a modal is asked for the focus like any
		// other and turns it down itself, in gainedFocus, so its mouseDown then drops the press — which is what
		// happens on the real platforms too.
		s.raise(target)
		s.setFocus(target)
	}
	s.capture = target
	s.buttons[button] = true
	target.mouseDown(windowLocal(target, pt), button, mods)
}

// buttonReleased delivers a release of the given button at pt.
func (s *headlessState) buttonReleased(pt geom.Point, button int, mods mod.Modifiers) {
	if s.drag != nil {
		// Any button ends a drag, whichever one started it: the platforms treat the release as the end of the gesture
		// rather than as the release of a particular button.
		s.dropDrag(pt, mods)
		return
	}
	s.pointer = pt
	s.lastMods = mods
	target := s.capture
	if target == nil {
		// Nothing holds the pointer, so this is the release of a press that was never delivered — one that had no
		// window under it, say. There is nowhere to send it.
		return
	}
	delete(s.buttons, button)
	// Release the grab before delivering rather than after: the handler may run a nested event loop — a modal dialog
	// put up from a mouse up does exactly that — in which further input must be routed with this button already up.
	last := len(s.buttons) == 0
	if last {
		s.capture = nil
	}
	target.mouseUp(windowLocal(target, pt), button, mods)
	if last {
		// With the grab over, the pointer is once again wherever it actually is. A release that landed over a different
		// window than the press did therefore leaves that window entered and the pressed one exited.
		s.updateHover(pt, mods)
	}
}

// resetClickCount makes the next press at pt the first of its series, by moving the window's last press far enough into
// the past that no press can be part of it. Injected clicks land microseconds apart, so without this every click a test
// issues at the same spot would be counted as a continuation of the one before it.
func (s *headlessState) resetClickCount(pt geom.Point) {
	target := s.capture
	if target == nil {
		target = s.windowAt(pt)
	}
	if target == nil {
		return
	}
	// The zero time is what does the work, since mouseDown measures the gap from it. The count is cleared as well, so
	// that a window inspected between clicks does not report a stale one.
	target.lastButtonTime = time.Time{}
	target.lastButtonCount = 0
}

// wheel delivers a wheel rotation of delta at pt to the window under the pointer, which is where every platform sends
// it rather than to the focused window — or to the window holding the pointer grab while a button is down, since the
// platforms' grabs route wheel events to the grab window along with everything else. It is deliberately not gated on
// anything: mouseWheel lets a window blocked by a modal scroll, on the grounds that scrolling only moves a view and
// cannot trigger an action.
//
// The pointer is moved to pt with the same crossing logic a motion gets, rather than merely placed there: mouseWheel
// ends by delivering a move, which enters the window internally, and a window that was left without an exit would
// otherwise be exited and re-entered by the next motion, so a panel would see enter, exit, enter for a single wheel
// followed by a move.
func (s *headlessState) wheel(pt, delta geom.Point, mods mod.Modifiers) {
	if s.drag != nil {
		s.dragWheel(pt, delta, mods)
		return
	}
	s.pointer = pt
	s.lastMods = mods
	target := s.capture
	if target == nil {
		target, _ = s.updateHover(pt, mods)
	}
	if target != nil {
		target.mouseWheel(windowLocal(target, pt), delta, mods)
	}
}

// keyDown delivers a key press to the focused window.
//
// With no focused window there is nothing to deliver to and the event is dropped. A window that is blocked by a modal
// is not that case: it still holds the focus and reroutes the event to the top modal window itself.
func (s *headlessState) keyDown(code KeyCode, mods mod.Modifiers) {
	s.lastMods = mods
	if s.drag != nil && code == KeyEscape {
		// Escape abandons a drag on every platform. Every other key goes to the focused window as usual, since the
		// application underneath the drag is still running and still able to react to them.
		s.cancelDrag()
		return
	}
	if s.focused == nil {
		return
	}
	s.focused.keyPressed(code, mods)
}

// keyUp delivers a key release to the focused window. See keyDown for what happens when there is none.
func (s *headlessState) keyUp(code KeyCode, mods mod.Modifiers) {
	s.lastMods = mods
	if s.focused == nil {
		return
	}
	s.focused.keyReleased(code, mods)
}

// runeTyped delivers a typed rune to the focused window. See keyDown for what happens when there is none.
func (s *headlessState) runeTyped(ch rune) {
	if s.focused == nil {
		return
	}
	s.focused.runeTyped(ch)
}
