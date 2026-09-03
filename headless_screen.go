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
	"image/draw"
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// This file is the driver: the half of HeadlessScreen a test calls from its own goroutine. Almost every method here
// funnels its work onto the UI thread through run(), because the state it reads and writes — the window stack, the
// focus, the rendered frames — belongs to that thread and to nothing else. The exceptions are Size, Scale, Beeps,
// Errors, Running and Done, which answer from the configuration, an atomic, a lock or a channel and are therefore safe
// to call from anywhere, including from a goroutine watching a session it did not start.

// run performs f on the UI thread and waits for it. Returns false, without having run f, if the session ended first.
func (s *HeadlessScreen) run(f func()) bool {
	if !s.Running() {
		// The session is over. There is no UI thread left to run f on, and queuing it would only leave a closure
		// behind in the task queue for whatever runs next to trip over.
		return false
	}
	if onUIThread() {
		// The caller is the UI thread itself — a widget callback reaching for the driver, say. Queuing the work and
		// waiting would deadlock, since the only goroutine that could ever run it is the one doing the waiting, so
		// dispatch synchronously instead. Nothing else is dispatched on the way: the events still queued behind the one
		// the caller is handling stay queued, since delivering them here would run the rest of a gesture inside the
		// handler for the start of it — a mouse up arriving while the mouse down is still on the stack — which no
		// platform ever does, and which a callback merely reading something from the screen must not provoke. What the
		// call cannot do is wait for the redraws its work provokes, since those happen further out in the event loop
		// that is currently suspended inside this call. SafeCall, so that a panic in f is recorded for Errors() here
		// exactly as processNextTask would record it for the same call made from the test goroutine, rather than
		// escaping into the widget callback that made it.
		SafeCall(f)
		return true
	}
	done := make(chan struct{})
	InvokeTask(s.guarded(f, func() { close(done) }))
	select {
	case <-done:
		return true
	case <-s.done:
		// The session ended while the task was still queued. It will never run now — finish() emptied the queue — so
		// waiting on done alone would block forever. Both channels may be closed by the time we get here, in which
		// case the choice above was made by coin toss, so ask again before reporting that f never ran.
		select {
		case <-done:
			return true
		default:
			return false
		}
	}
}

// guarded wraps f, and the after function that follows it, in a check that this session is still the one running.
// There is a window between a caller checking Running() and the task it then queues joining the queue, during which
// the session may end and finish() empty the queue: a task that joins after that is left behind in the package-global
// queue, which the next headless session discards but a native Start() would run — against a session that has long
// since ended. The wrapper makes such a stray inert. after is run whether f was or not, so a caller waiting on it is
// released either way.
func (s *HeadlessScreen) guarded(f, after func()) func() {
	return func() {
		if after != nil {
			defer after()
		}
		if s.Running() {
			f()
		}
	}
}

// quiescent reports whether the application has nothing left to do. UI thread only.
func (s *HeadlessScreen) quiescent() bool {
	s.inputLock.Lock()
	pendingInput := len(s.input)
	s.inputLock.Unlock()
	if pendingInput != 0 {
		return false
	}
	taskQueueLock.Lock()
	pendingTasks := len(taskQueue) - taskQueueHead
	taskQueueLock.Unlock()
	if pendingTasks != 0 {
		return false
	}
	// An activation ToFront() has asked for is performed at the end of the pass, after the probe that calls this has
	// run, so it is work still outstanding. As with a DrawCallback that marks its own panel for redraw, a
	// GainedFocusCallback that calls ToFront() renews this on every pass and leaves Sync() waiting until it gives up.
	if pendingFrontWindow != nil {
		return false
	}
	// Only the windows that can actually be drawn count as work outstanding. finishProcessingEvents puts a window that
	// is valid but not visible straight back into redrawSet, to be drawn if it is ever shown, so counting those would
	// leave a window that was created and never shown looking like work that is about to happen — forever.
	for wnd := range redrawSet {
		if wnd.IsVisible() {
			return false
		}
	}
	return true
}

// Sync waits until the application has finished reacting to everything that has been asked of it: no events left to
// dispatch, no tasks left to run and no windows left to redraw.
//
// It takes more than one pass because of how the loop is ordered. A pass dispatches the pending events first, then
// runs a single task, then performs the redraws — so the pass that runs the probe queued by run() has already drained
// the events, and still has the redraws that the work before it asked for ahead of it. The probe therefore reports
// "not settled", and the next probe, queued behind those redraws, sees a clean slate.
//
// Work scheduled with InvokeTaskAfter is deliberately not waited for: there is no way to tell a timer that will fire
// in a second from one that will fire in an hour.
//
// An application that keeps queueing work would leave this waiting forever, so that wait is bounded by
// HeadlessConfig.SyncTimeout. The usual cause is work that renews itself on every pass — a DrawCallback that marks its
// own panel for redraw is the classic one, since the redraw it asks for is outstanding again the instant the one it is
// performing finishes. Giving up is reported through Errors() rather than by blocking or panicking, so the test that
// tripped over it still runs to its end and can say what happened. The bound is on the application going quiet, not
// on any one thing it does: a UI-thread callback that blocks forever — waiting on the very test goroutine that is
// inside this call, say — is a deadlock, and no timeout here can reach into it.
func (s *HeadlessScreen) Sync() {
	if onUIThread() {
		// From the UI thread there is nothing to wait for: the events, tasks and redraws that may be outstanding can
		// only be performed by the event loop that is currently suspended inside this very call, once the caller has
		// returned to it. Looping would spin forever, and probing would only report what the caller already knows.
		return
	}
	started := time.Now()
	for {
		settled := false
		if !s.run(func() { settled = s.quiescent() }) || settled {
			return
		}
		if time.Since(started) >= s.cfg.SyncTimeout {
			s.recordError(errs.Newf("the application never went quiet within %v, so the wait was abandoned; the "+
				"usual cause is something marking for redraw on every draw", s.cfg.SyncTimeout))
			return
		}
	}
}

// Do runs f on the UI thread, waits for it, and then waits for everything it set in motion to finish, so that when Do
// returns the application has fully caught up. This is how a test creates windows, drives widgets and reads panel
// state. Returns false, without having run f, if the session has already ended.
//
// Work that runs a nested event loop of its own must go through Post() instead: RunModal(), and StartDrag() called
// from anywhere other than a mouse drag handler, do not return until something the test has yet to inject dismisses
// them, and Do cannot return until the closure it was given does.
//
// Called from the UI thread — from a widget callback, say — f runs inline and nothing further is waited for, since
// whatever it set in motion belongs to the event loop the caller is suspended inside.
func (s *HeadlessScreen) Do(f func()) bool {
	if !s.run(f) {
		return false
	}
	s.Sync()
	return true
}

// Post queues f to run on the UI thread and returns immediately. Use it for work that will not return until the test
// does something else, such as starting a modal dialog with RunModal(). Reports false, having queued nothing, if the
// session has already ended: a task queued then would only be left behind for whatever runs next to trip over. A task
// queued just before the session ends is discarded with it, and is never run by anything that comes after.
func (s *HeadlessScreen) Post(f func()) bool {
	if !s.Running() {
		return false
	}
	InvokeTask(s.guarded(f, nil))
	return true
}

// Everything from here to postKeyStroke is the input injection: the events an operating system would have delivered,
// posted onto the same queue and dispatched by the same drain the real ones would have been. Each of these methods
// posts its events and then waits for the application to finish reacting to all of them, so that by the time it returns
// the callbacks have run, the tasks they queued have run, and the redraws those asked for have been performed.
//
// Called from the UI thread — which happens when a widget callback drives the screen itself — the events are only
// queued: they are dispatched by the event loop the caller is suspended inside once it has returned there, which is
// when a platform delivers input that arrived while a handler was running. The call therefore comes back before the
// events have been delivered, and cannot wait for the tasks and redraws they provoke either.
//
// Positions are in the screen's logical coordinate space, the same space the window content rects are in. PanelCenter
// and PanelPoint convert from a panel's own space into it.

// MouseMove moves the pointer to pt. A window that has the pointer captured by a press in progress receives the motion
// wherever pt is; otherwise the window under pt does, preceded by the exit and entry callbacks if the pointer crossed
// from one window to another.
func (s *HeadlessScreen) MouseMove(pt geom.Point, mods mod.Modifiers) {
	s.post(func() { s.pointerMoved(pt, mods) })
	s.Sync()
}

// MouseDown presses the given button at pt, one of ButtonLeft, ButtonRight or ButtonMiddle. The pointer is moved there
// first, and the window that receives the press is given the keyboard focus if it did not already have it, as clicking
// a window does on every platform.
//
// The window keeps the pointer until the last button is released, so a MouseMove that follows this one is delivered to
// it however far outside it the pointer has gone.
func (s *HeadlessScreen) MouseDown(pt geom.Point, button int, mods mod.Modifiers) {
	s.post(func() { s.buttonPressed(pt, button, mods) })
	s.Sync()
}

// MouseUp releases the given button at pt. It goes to the window that received the press, wherever the pointer has
// since moved to; releasing the last button ends that and leaves the pointer over whatever window it is actually over.
func (s *HeadlessScreen) MouseUp(pt geom.Point, button int, mods mod.Modifiers) {
	s.post(func() { s.buttonReleased(pt, button, mods) })
	s.Sync()
}

// Click presses and releases the left button at pt with no modifiers.
func (s *HeadlessScreen) Click(pt geom.Point) {
	s.ClickWith(pt, ButtonLeft, mod.None)
}

// ClickWith presses and releases the given button at pt.
//
// The click count the window reports is one, however quickly clicks follow one another: the click tracking of the
// window about to be pressed is reset first. Injected clicks land microseconds apart rather than with the pause a
// person would leave, so without that reset consecutive clicks at the same spot would all be counted as one long
// double-, triple- and quadruple-click. Use DoubleClick when a click count of two is what the test wants.
func (s *HeadlessScreen) ClickWith(pt geom.Point, button int, mods mod.Modifiers) {
	s.post(func() { s.resetClickCount(pt) })
	s.post(func() { s.buttonPressed(pt, button, mods) })
	s.post(func() { s.buttonReleased(pt, button, mods) })
	s.Sync()
}

// DoubleClick presses and releases the left button twice at pt with no modifiers, so that the second press reports a
// click count of two. The two presses are microseconds apart at the very same point, which is comfortably inside the
// 500ms and 5px of drift a double-click is allowed, making the count a certainty rather than a matter of how fast the
// machine running the test is.
//
// The click tracking is reset ahead of the first press only, so the pair always reports one and then two rather than
// continuing the count of a click that happened to land at the same spot just before it.
func (s *HeadlessScreen) DoubleClick(pt geom.Point) {
	s.post(func() { s.resetClickCount(pt) })
	for range 2 {
		s.post(func() { s.buttonPressed(pt, ButtonLeft, mod.None) })
		s.post(func() { s.buttonReleased(pt, ButtonLeft, mod.None) })
	}
	s.Sync()
}

// Drag moves the pointer to from, presses the left button, moves the pointer to to in the given number of steps, and
// releases the button there. Fewer than one step is treated as one, and the last step always lands exactly on to.
//
// Each step covers an equal fraction of the distance, and the drift IsDragGesture measures is from the press rather
// than from the previous step, so a drag long enough to exceed the 5px of drift that makes a gesture a drag is
// recognized as one by the time it arrives — on its first step when there is only one, and on whichever step first
// carries it past the threshold otherwise — while a shorter one never is. The other half of IsDragGesture is a 250ms
// delay, and nothing here waits on a clock: a widget that relies on the delay rather than the drift needs the caller
// to sleep between the press and the moves, which means driving those with MouseDown, MouseMove and MouseUp.
//
// The click tracking of the window about to be pressed is reset first, as ClickWith does, so the press reports a
// click count of one however soon it follows a click at the same spot.
func (s *HeadlessScreen) Drag(from, to geom.Point, steps int) {
	if steps < 1 {
		steps = 1
	}
	// No motion is posted ahead of the press: buttonPressed delivers one itself, exactly as the platforms precede a
	// press with the motion that took the pointer there, and a second one would be a move the platforms never produce.
	s.post(func() { s.resetClickCount(from) })
	s.post(func() { s.buttonPressed(from, ButtonLeft, mod.None) })
	delta := to.Sub(from)
	for i := 1; i <= steps; i++ {
		pt := to
		if i != steps {
			// The last step is the destination itself rather than the same arithmetic as the others, so that it and the
			// release that follows land on precisely the same point. A widget comparing the two would otherwise see a
			// drag that stopped a rounding error short of where it was asked to go.
			pt = from.Add(delta.Mul(float32(i) / float32(steps)))
		}
		s.post(func() { s.pointerMoved(pt, mod.None) })
	}
	s.post(func() { s.buttonReleased(to, ButtonLeft, mod.None) })
	s.Sync()
}

// Wheel rotates the mouse wheel by delta at pt. It goes to the window under the pointer rather than to the focused one,
// as it does on every platform, and reaches a window that is blocked by a modal as well, since scrolling only moves a
// view and cannot trigger an action.
func (s *HeadlessScreen) Wheel(pt, delta geom.Point, mods mod.Modifiers) {
	s.post(func() { s.wheel(pt, delta, mods) })
	s.Sync()
}

// KeyDown presses a key. Key events go to the focused window and are dropped when there is none.
func (s *HeadlessScreen) KeyDown(code KeyCode, mods mod.Modifiers) {
	s.post(func() { s.keyDown(code, mods) })
	s.Sync()
}

// KeyUp releases a key. Key events go to the focused window and are dropped when there is none.
func (s *HeadlessScreen) KeyUp(code KeyCode, mods mod.Modifiers) {
	s.post(func() { s.keyUp(code, mods) })
	s.Sync()
}

// KeyPress presses and releases a key, with the rune that key produces delivered in between when it has one.
//
// Which keys have one is the US layout in headlessKeys, and a rune is only produced when none of Control, Option or
// Command is down — the rule the Linux backend applies, since a key that is part of a command is not text. Shift is not
// one of those, so KeyPress(KeyA, mod.Shift) types 'A' while KeyPress(KeyA, mod.Control) types nothing.
func (s *HeadlessScreen) KeyPress(code KeyCode, mods mod.Modifiers) {
	var ch rune
	if mods&(mod.Control|mod.Option|mod.Command) == 0 {
		ch = headlessKeys().strokeToRune[headlessKeyStroke{code: code, mods: mods & mod.Shift}]
	}
	s.postKeyStroke(code, mods, ch)
	s.Sync()
}

// Type enters text one rune at a time, the way a person at a US keyboard would: for each rune the key that produces it
// is pressed, the rune is delivered, and the key is released. Uppercase letters and the shifted punctuation carry
// mod.Shift, so typing "A" or "!" holds shift for exactly that key.
//
// A line ending is typed as Return, a tab as Tab, a backspace (U+0008) as Backspace and a delete (U+007F) as Delete,
// none of which produces a rune, so a field sees them as the keys they are rather than as characters to insert. All
// three of the conventional line endings are understood: "\n", "\r" and "\r\n" each type exactly one Return, so text
// carrying Windows line endings does not type a stray carriage return ahead of every one of them. A rune that no key
// on the layout produces — anything non-ASCII, and any other control character — is delivered on its own, with no key
// press around it, which is how a platform reports text coming from an input method rather than from a key.
func (s *HeadlessScreen) Type(text string) {
	keys := headlessKeys()
	afterCarriageReturn := false
	for _, ch := range text {
		wasAfterCarriageReturn := afterCarriageReturn
		afterCarriageReturn = ch == '\r'
		switch ch {
		case '\r':
			s.postKeyStroke(KeyReturn, mod.None, 0)
		case '\n':
			if wasAfterCarriageReturn {
				// The other half of a "\r\n" pair, which is one line ending and therefore one Return, already typed.
				continue
			}
			s.postKeyStroke(KeyReturn, mod.None, 0)
		case '\t':
			s.postKeyStroke(KeyTab, mod.None, 0)
		case '\b':
			s.postKeyStroke(KeyBackspace, mod.None, 0)
		case '\x7f':
			s.postKeyStroke(KeyDelete, mod.None, 0)
		default:
			if stroke, ok := keys.runeToStroke[ch]; ok {
				s.postKeyStroke(stroke.code, stroke.mods, ch)
			} else {
				s.post(func() { s.runeTyped(ch) })
			}
		}
	}
	s.Sync()
}

// postKeyStroke posts the press of a key, the rune it produced if it produced one, and its release. They are posted as
// three separate events, as the platforms deliver them, so that a nested event loop started by any one of them still
// receives the rest.
func (s *HeadlessScreen) postKeyStroke(code KeyCode, mods mod.Modifiers, ch rune) {
	s.post(func() { s.keyDown(code, mods) })
	if ch != 0 {
		s.post(func() { s.runeTyped(ch) })
	}
	s.post(func() { s.keyUp(code, mods) })
}

// The drag & drop entry points. A drag this application started needs none of them: a widget calling StartDrag from
// its MouseDragCallback turns the events an injected Drag() posts into a drag all by itself, since the session that
// call starts consumes them as the platforms' drag loops consume theirs. These are for the other direction — data
// arriving from outside the application, such as files dropped from a file manager — where there is no source in this
// process to start anything.
//
// The composite data types are encoded as headlessDragInfo documents them: file paths and URLs are one per line,
// separated by newlines.

// HeadlessDrag is a drag & drop that entered the application from outside it and has not ended yet. Move it with
// MoveTo and finish it with Drop or Cancel.
type HeadlessDrag struct {
	screen *HeadlessScreen
	// started is false for a drag that was refused, whose methods then do nothing and report a canceled result. It is
	// written on the UI thread, inside the event BeginExternalDrag posts, and read by whichever goroutine holds the
	// handle. The wait in Sync() normally orders the two, but a session that ends while the event is in flight lets
	// Sync() return without having waited for it, so the flag is atomic rather than relying on that.
	started atomic.Bool
}

// BeginExternalDrag starts a drag carrying the given data over the point at, as though the pointer had just entered
// the application holding data from somewhere else. A window under that point which registered for one of the types
// being carried is entered immediately.
//
// Starting one while a drag is already in progress, or while a mouse button is down, is refused: the pointer is
// already spoken for. The refusal is recorded for Errors() and the returned handle does nothing but report a
// canceled result.
func (s *HeadlessScreen) BeginExternalDrag(at geom.Point, opMask drag.Op, data ...drag.Data) *HeadlessDrag {
	d := &HeadlessDrag{screen: s}
	s.post(func() {
		switch {
		case s.drag != nil:
			s.recordError(errs.New("a drag & drop session is already in progress"))
		case len(s.buttons) != 0:
			s.recordError(errs.New("a mouse button is down, so a drag cannot enter from outside the application"))
		default:
			// The pointer arrives from outside, so it is simply placed rather than moved: there is no path from
			// wherever it was to here for the windows in between to react to. The window it was in is exited all the
			// same, since a window server delivers a leave when a drag takes the pointer, and the panels in that
			// window would otherwise believe the pointer was in them for the whole of the drag — and, if the drag
			// ended over that very window, for good, since the entry finishDrag performs is skipped for a window
			// that is still recorded as hovered.
			if hovered := s.hover; hovered != nil {
				s.hover = nil
				if hovered.IsValid() {
					hovered.mouseExit()
				}
			}
			s.pointer = at
			s.beginDrag(nil, nil, geom.Point{}, opMask, data)
			d.started.Store(true)
		}
	})
	s.Sync()
	return d
}

// MoveTo moves the drag to pt, entering and leaving windows as it crosses them.
func (d *HeadlessDrag) MoveTo(pt geom.Point) {
	if !d.started.Load() {
		return
	}
	d.screen.post(func() {
		if d.screen.drag != nil {
			d.screen.dragMoved(pt, d.screen.lastMods)
		}
	})
	d.screen.Sync()
}

// Drop releases the drag where it currently is and returns how it ended. The target is offered the drop only if it
// last reported that it would do something with one there.
func (d *HeadlessDrag) Drop() HeadlessDragResult {
	if !d.started.Load() {
		return HeadlessDragResult{Canceled: true}
	}
	d.screen.post(func() {
		if d.screen.drag != nil {
			d.screen.dropDrag(d.screen.pointer, d.screen.lastMods)
		}
	})
	d.screen.Sync()
	return d.screen.LastDrag()
}

// Cancel abandons the drag, exactly as pressing Escape during one does, and returns how it ended.
func (d *HeadlessDrag) Cancel() HeadlessDragResult {
	if !d.started.Load() {
		return HeadlessDragResult{Canceled: true}
	}
	d.screen.post(func() {
		if d.screen.drag != nil {
			d.screen.cancelDrag()
		}
	})
	d.screen.Sync()
	return d.screen.LastDrag()
}

// DropExternal carries the given data from outside the application to the point to, in the given number of steps, and
// drops it there. Fewer than one step is treated as one, and the last step always lands exactly on to. It is the whole
// of an external drag in one call, for the common case where nothing needs to be inspected while it is in flight.
func (s *HeadlessScreen) DropExternal(from, to geom.Point, steps int, opMask drag.Op,
	data ...drag.Data,
) HeadlessDragResult {
	d := s.BeginExternalDrag(from, opMask, data...)
	if steps < 1 {
		steps = 1
	}
	delta := to.Sub(from)
	for i := 1; i <= steps; i++ {
		pt := to
		if i != steps {
			// As in Drag(), the last step is the destination itself rather than the same arithmetic as the others, so
			// that it and the drop land on precisely the same point.
			pt = from.Add(delta.Mul(float32(i) / float32(steps)))
		}
		d.MoveTo(pt)
	}
	return d.Drop()
}

// LastDrag returns how the most recent drag & drop session ended, whether it started inside the application or
// outside it. The zero value is returned when there has not been one. Unlike the other methods that consult the UI
// thread, this one still answers after the session has ended: a drag that was in flight when the session ended is
// reported as canceled, since that is how it ended, and one that completed just before is reported as it completed.
func (s *HeadlessScreen) LastDrag() HeadlessDragResult {
	var result HeadlessDragResult
	if !s.run(func() { result = s.lastDrag }) {
		// run() only refuses once done has been closed, and finish() closes it after every write it and terminate()
		// made, so the field is safe to read directly from here.
		result = s.lastDrag
	}
	// A copy per call, so that a caller writing to the bytes it took away alters nothing but its own, neither what the
	// session is holding nor what an earlier or later call handed to somebody else.
	result.Data = cloneDragData(result.Data)
	return result
}

// PanelCenter returns the center of the panel's content area in the screen's logical coordinate space, which is what
// the mouse injection methods take. This is how a test aims at a widget: screen.Click(screen.PanelCenter(button)).
// Returns the zero point if the panel is not in a window.
func (s *HeadlessScreen) PanelCenter(p Paneler) geom.Point {
	var result geom.Point
	s.run(func() {
		pnl := p.AsPanel()
		w := pnl.Window()
		if w == nil {
			return
		}
		// RectToRoot takes the panel's own space up to the window's, applying the scale of every panel on the way, and
		// the window's content origin takes that the rest of the way to the screen.
		result = pnl.RectToRoot(pnl.ContentRect(false)).Center().Add(w.ContentRect().Point)
	})
	return result
}

// PanelPoint returns the point at the given offset from the top-left corner of the panel's content area, in the
// screen's logical coordinate space. The offset is in the panel's own coordinate space, so it is scaled exactly as the
// panel's content is. Returns the zero point if the panel is not in a window.
func (s *HeadlessScreen) PanelPoint(p Paneler, offset geom.Point) geom.Point {
	var result geom.Point
	s.run(func() {
		pnl := p.AsPanel()
		w := pnl.Window()
		if w == nil {
			return
		}
		result = pnl.PointToRoot(pnl.ContentRect(false).Point.Add(offset)).Add(w.ContentRect().Point)
	})
	return result
}

// headlessKeyStroke is a key and the modifiers held with it, which together are what a US keyboard turns into one rune.
// Only mod.Shift ever appears here: it is the one modifier that changes which rune a key produces rather than turning
// the key into part of a command.
type headlessKeyStroke struct {
	code KeyCode
	mods mod.Modifiers
}

// headlessKeyMaps is the US keyboard layout, in both directions: which stroke types a rune, and which rune a stroke
// types.
type headlessKeyMaps struct {
	runeToStroke map[rune]headlessKeyStroke
	strokeToRune map[headlessKeyStroke]rune
}

// headlessShiftedPunctuation is the half of the layout that cannot be derived, since the key codes are named for their
// unshifted rune and nothing records what shift turns them into.
var headlessShiftedPunctuation = map[rune]KeyCode{
	'!': Key1,
	'@': Key2,
	'#': Key3,
	'$': Key4,
	'%': Key5,
	'^': Key6,
	'&': Key7,
	'*': Key8,
	'(': Key9,
	')': Key0,
	'_': KeyMinus,
	'+': KeyEqual,
	'{': KeyOpenBracket,
	'}': KeyCloseBracket,
	'|': KeyBackslash,
	':': KeySemiColon,
	'"': KeyApostrophe,
	'<': KeyComma,
	'>': KeyPeriod,
	'?': KeySlash,
	'~': KeyBackQuote,
}

// headlessKeys returns the US keyboard layout, building it on first use. A layout is what turns the text a test wants
// typed into the key events a window expects, and every platform has one; this package already carries most of one, in
// the names keyCodeToString gives the key codes, so the layout is mostly a matter of reading it backwards. The letters
// are named by their uppercase rune there, which is precisely the shifted half, and the keys named with a word rather
// than a single character (space, escape, the function keys, the numeric keypad) produce no text on their own and are
// left out — except space, which is added by hand, since it does type a rune despite its name.
var headlessKeys = sync.OnceValue(func() *headlessKeyMaps {
	m := &headlessKeyMaps{
		runeToStroke: make(map[rune]headlessKeyStroke, 2*len(keyCodeToString)),
		strokeToRune: make(map[headlessKeyStroke]rune, 2*len(keyCodeToString)),
	}
	add := func(ch rune, code KeyCode, mods mod.Modifiers) {
		stroke := headlessKeyStroke{code: code, mods: mods}
		m.runeToStroke[ch] = stroke
		m.strokeToRune[stroke] = ch
	}
	for code, name := range keyCodeToString {
		runes := []rune(name)
		if len(runes) != 1 {
			continue
		}
		ch := runes[0]
		if lower := unicode.ToLower(ch); lower != ch {
			add(lower, code, mod.None)
			add(ch, code, mod.Shift)
			continue
		}
		add(ch, code, mod.None)
	}
	add(' ', KeySpace, mod.None)
	for ch, code := range headlessShiftedPunctuation {
		add(ch, code, mod.Shift)
	}
	return m
})

// Size returns the size of the screen in logical points. Safe to call from any goroutine.
func (s *HeadlessScreen) Size() geom.Size {
	return geom.NewSize(s.cfg.Width, s.cfg.Height)
}

// Scale returns the backing scale of the screen. Safe to call from any goroutine.
func (s *HeadlessScreen) Scale() float32 {
	return s.cfg.Scale
}

// Beeps returns how many times Beep() has been called during this session. Safe to call from any goroutine.
func (s *HeadlessScreen) Beeps() int {
	return int(s.beeps.Load())
}

// Errors returns everything that has gone wrong during this session: the panics that were recovered, and the requests
// the session refused, such as a drag started while another one was still in progress. The panics are only among them
// when the application did not supply a RecoveryCallback of its own, since that callback then decides what happens to
// them. Safe to call from any goroutine.
func (s *HeadlessScreen) Errors() []error {
	s.errLock.Lock()
	defer s.errLock.Unlock()
	return slices.Clone(s.recorded)
}

// SetDarkMode sets what IsDarkModeEnabled() reports while the theme mode is thememode.Auto and runs a full theme
// change, so the dynamic colors, the cursors and the windows all catch up before it returns.
func (s *HeadlessScreen) SetDarkMode(enabled bool) {
	s.Do(func() {
		s.darkMode = enabled
		needPlatformDarkModeUpdate = true
		ThemeChanged()
	})
}

// WindowAt returns the topmost window containing pt, which is in the screen's logical coordinate space, or nil if
// there is none.
func (s *HeadlessScreen) WindowAt(pt geom.Point) *Window {
	var result *Window
	s.run(func() { result = s.windowAt(pt) })
	return result
}

// FocusedWindow returns the window that currently holds the keyboard focus, or nil if none does.
func (s *HeadlessScreen) FocusedWindow() *Window {
	var result *Window
	s.run(func() { result = s.focused })
	return result
}

// Cursor returns the cursor most recently resolved by any window, or nil if none has been.
func (s *HeadlessScreen) Cursor() *Cursor {
	var result *Cursor
	s.run(func() { result = s.cursor })
	return result
}

// Capture returns what the screen would look like right now: the configured background with every visible window
// composited onto it in z-order. The result is in device pixels, so it is the screen's logical size multiplied by the
// backing scale. Windows that have never been drawn, and windows that are minimized, contribute nothing.
//
// Call Sync() first if you have just changed something, or you may capture the state from before the redraw.
func (s *HeadlessScreen) Capture() *image.NRGBA {
	var result *image.NRGBA
	s.run(func() { result = s.captureScreen() })
	return result
}

// CaptureWindow returns the pixels the given window last presented, or nil if it has never been drawn or does not
// belong to this session. The result is in device pixels, so it is the window's content size multiplied by the backing
// scale.
func (s *HeadlessScreen) CaptureWindow(w *Window) *image.NRGBA {
	var result *image.NRGBA
	s.run(func() {
		// A window keeps its headless backing for life, so one retained from an earlier session still has a backing,
		// and a frame in it; the check against the session is what keeps that frame from being handed out here.
		hw := headlessWindowFor(w)
		if hw == nil || hw.hs != s || hw.frame == nil {
			return
		}
		result = toNRGBA(hw.frame)
	})
	return result
}

// Quit asks the application to terminate, exactly as a user-initiated quit would, and reports whether it did. When it
// succeeds, Quit does not return until the session has been fully torn down.
//
// False means the session is still running, which covers two cases. The quit may have been refused outright, by the
// AllowQuitCallback or by a window declining to close. Or it may still be waiting on the application: an
// AllowCloseCallback that puts up a confirmation dialog leaves that dialog open and the quit suspended inside it, with
// the application idle and ready for the test to drive the dialog. Whichever it was, Wait(), Done() and Running() are
// what tell the test whether the session went on to end.
//
// That second case is why the quit is queued rather than run inline: a dialog only goes away once the test injects the
// input that dismisses it, which it cannot do while it is blocked waiting for the quit to finish.
//
// Called from the UI thread, it runs the quit inline and reports whether it was accepted, but cannot wait for the
// teardown, since that is performed by the event loop the caller is suspended inside.
func (s *HeadlessScreen) Quit() bool {
	if onUIThread() {
		s.run(AttemptQuit)
		return s.terminated.Load()
	}
	s.Post(AttemptQuit)
	// Waits for the quit to have been attempted, and for the application to have gone quiet again afterwards — which,
	// when the quit succeeded, means waiting for the session to end.
	s.Sync()
	if !s.terminated.Load() {
		return false
	}
	<-s.done
	return true
}

// Stop ends the session whether the application wants to or not, and does not return until it has. Use it as the
// cleanup for a test, where leaving a session running would strand the globals it owns and break whatever runs next.
// Stopping a session that has already ended does nothing.
//
// Called from the UI thread, it sets the shutdown in motion and returns, since the event loop that has to perform it is
// the one the caller is suspended inside.
func (s *HeadlessScreen) Stop() {
	s.run(func() {
		// Take away everything that could refuse, and everything that could stand in the way without refusing: a
		// WillCloseCallback or QuittingCallback that puts up a dialog with RunModal() would park the UI thread in a
		// nested loop waiting for input that only the goroutine blocked below could inject, and the session would
		// never end. The quitting callback is guarded by quitLock because quitting() takes it there.
		allowQuitCallback = nil
		quitLock.Lock()
		quittingCallback = nil
		quitLock.Unlock()
		for _, w := range Windows() {
			w.AllowCloseCallback = nil
			w.WillCloseCallback = nil
		}
		for _, w := range slices.Clone(modalStack) {
			w.StopModal(ModalResponseCancel)
		}
		// Queue the quit rather than attempting it here: stopping a modal only asks its nested RunModal loop to
		// finish, and that loop has to unwind — disposing its window on the way — before there is anything sensible
		// left to close.
		InvokeTask(AttemptQuit)
	})
	if !onUIThread() {
		<-s.done
	}
}

// Wait blocks until the session has ended. Do not call it from the UI thread: the session cannot end while the thread
// that has to end it is waiting.
func (s *HeadlessScreen) Wait() {
	<-s.done
}

// Done returns a channel that is closed when the session has ended.
func (s *HeadlessScreen) Done() <-chan struct{} {
	return s.done
}

// Running returns true while the session is still going. Safe to call from any goroutine.
func (s *HeadlessScreen) Running() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

// captureScreen composites the screen. UI thread only.
//
// Logical points become device pixels by flooring here, which is what surface.prepareCanvas's truncation amounts to
// when it works out how many pixels a window's rendering surface needs, since a size is never negative. Rounding
// instead would put a window whose size or position lands on a half pixel — which is every other one at a scale of
// 1.5 — a pixel away from where its own pixels were sized, leaving a seam of background along one edge or a row of the
// window's pixels beyond where it claims to end. A position, unlike a size, can be negative — SetFrameRect does not
// force a window onto the display — and there truncation would go the other way from flooring, moving the window a
// pixel toward the origin rather than away from it, so the floor is taken explicitly.
func (s *headlessState) captureScreen() *image.NRGBA {
	scale := s.cfg.Scale
	bounds := image.Rect(0, 0, int(s.cfg.Width*scale), int(s.cfg.Height*scale))
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, image.NewUniform(s.cfg.Background), image.Point{}, draw.Src)
	for _, w := range s.stack { // back to front, so later windows paint over earlier ones
		hw := headlessWindowFor(w)
		if hw == nil || !hw.visible || w.minimized || hw.frame == nil {
			continue
		}
		origin := image.Pt(int(math.Floor(float64(hw.rect.X*scale))), int(math.Floor(float64(hw.rect.Y*scale))))
		draw.Draw(dst, hw.frame.Bounds().Add(origin), hw.frame, image.Point{}, draw.Over)
	}
	return toNRGBA(dst)
}

// toNRGBA converts premultiplied pixels into the unpremultiplied form the rest of this package deals in, the same way
// Image.ToNRGBA hands back unpremultiplied pixels. The standard library's draw does the division.
func toNRGBA(src *image.RGBA) *image.NRGBA {
	bounds := src.Bounds()
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Src)
	return dst
}
