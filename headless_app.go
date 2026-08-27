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

	"github.com/richardwilkes/unison/enums/thememode"
)

// This file is the headless stand-in for the operating system's application object and event queue. The queue is a
// slice of closures rather than a channel of event structs: the quiescence probe behind Sync() has to be able to ask
// how many are still pending, and draining has to happen one at a time (see drainInput), neither of which a channel
// offers.

// post adds f to the event queue and wakes the event loop. It may be called from any goroutine.
func (s *headlessState) post(f func()) {
	s.inputLock.Lock()
	s.input = append(s.input, f)
	s.inputLock.Unlock()
	s.wakeUp()
}

// wakeUp releases the event loop if it is waiting, and does nothing if it is already awake or has a wake-up pending.
// The channel has a capacity of one, so a burst of posts collapses into a single wake-up, exactly as the platforms'
// empty-event postings do. It may be called from any goroutine.
func (s *headlessState) wakeUp() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// drainInput dispatches every queued event, taking the lock once per event rather than snapshotting the queue. That
// matters because a handler may start a nested event loop — RunModal() does, and so does the source side of a drag &
// drop — which re-enters here through processEvents. Popping one at a time lets the nested loop consume the events
// that were queued behind the one it is running, which a snapshot would have hidden from it. This is the same reason
// nativeWaitEvents on Linux processes pending X11 events one at a time.
func (s *headlessState) drainInput() {
	// Stop early once the session has been terminated: the windows the remaining events were aimed at are gone.
	for !s.terminated.Load() {
		s.inputLock.Lock()
		if len(s.input) == 0 {
			s.inputLock.Unlock()
			return
		}
		f := s.input[0]
		s.input[0] = nil // release the closure for GC
		s.input = s.input[1:]
		s.inputLock.Unlock()
		SafeCall(f)
	}
}

func (s *headlessState) beginStartup() error {
	// Force CPU rendering for the whole session. NewWindow then never asks for an OpenGL context and
	// surface.prepareCanvas builds a raster surface instead, whose pixmap Window.draw hands to apiPresentCPUPixels.
	// That pixmap is the only "screen" a headless session has.
	s.prevCPURendering = cpuRenderingActive
	cpuRenderingActive = true
	// Drop anything left in the task queue by earlier work in this process — another test, or a session that was torn
	// down while tasks were still pending. Those closures refer to windows that no longer exist and, worse, would run
	// against this session's freshly reset state.
	taskQueueLock.Lock()
	clear(taskQueue)
	taskQueue = taskQueue[:0]
	taskQueueHead = 0
	taskQueueLock.Unlock()
	// Disown any windows left behind the same way. A real application reaches this with all of these empty, since
	// Start() runs before it can have made a window, but a test binary may have hand-built windows earlier that this
	// session must not adopt: their platform state belongs to the OS backend, and a window that is valid but not
	// visible is put straight back into redrawSet by finishProcessingEvents, so a single leftover would keep the
	// session from ever going quiet and spin the event loop forever.
	windowList = nil
	modalStack = nil
	redrawSet = make(map[*Window]struct{})
	wndWithCurrentCtx = nil
	// Take any cursors that predate the session out of the way, so that it neither adopts nor destroys them. Those are
	// native cursors, holding operating system resources that belong to whatever built them, while everything a session
	// hands out is inert — and a session assumes as much about every cursor it can see. It would destroy these out from
	// under their owner, either as it went (syncBuiltInCursors rebuilds on a theme change) or on the way out
	// (finishQuit's loop over cursorList), and finish() would then treat the singletons pointing at them as its own to
	// discard. Detaching leaves the session building its own from scratch, and finish() puts these back untouched. The
	// change callbacks go with them: they were registered by panels outside the session, which must not be told about
	// rebuilds of cursors they will never see.
	s.priorCursors = cursorList
	s.priorCursorSettings = builtCursorSettings
	s.priorCursorChangedCallbacks = cursorChangedCallbacks
	s.priorBuiltInCursors = make([]*Cursor, 0, len(builtInCursors()))
	for _, p := range builtInCursors() {
		s.priorBuiltInCursors = append(s.priorBuiltInCursors, *p)
		*p = nil
	}
	cursorList = nil
	builtCursorSettings = nil
	cursorChangedCallbacks = nil
	s.priorMenuFactory = defaultMenuFactory
	defaultMenuFactory = nil
	// The in-window file dialogs, which are the only ones a headless session has, remember the directory they were last
	// used in. The first session in a process must not inherit that from whatever ran before it.
	lastWorkingDir = ""
	// The theme mode is the session's to set, starting from Auto as a fresh process would, so whatever was set before
	// the session is taken out of the way here and put back by finish(), exactly as the cursors are. Without this the
	// first session in a process would inherit a mode set before it while every later one started at Auto.
	s.priorThemeMode = thememode.Enum(currentThemeMode.Swap(int32(thememode.Auto)))
	// The dark mode answer now comes from this session rather than the OS, so discard the cached value.
	needPlatformDarkModeUpdate = true
	if recoveryCallback == nil {
		// Make panics assertable through Errors() rather than leaving them as log output nothing can see. An
		// application-supplied RecoveryCallback wins, since it may be the very thing under test.
		recoveryCallback = s.recordError
	}
	return nil
}

// lateInit releases StartHeadless. The platform work it exists for — installing the global menu bar, starting the theme
// monitors — has no headless counterpart, which leaves it free to serve as the moment startup is far enough along for
// the caller to be let go.
//
// That is here rather than in finalFinishStartup because this runs immediately before finishStartup calls the
// application's StartupFinishedCallback and that runs immediately after it. A callback which starts a nested event loop
// of its own — a first-run dialog put up with RunModal(), say — never reaches the far side, and waiting there would
// leave StartHeadless blocked forever on a loop waiting for input that only the blocked caller could inject.
// StartHeadless makes up the difference by settling before it returns; see the comment there.
func (s *headlessState) lateInit() {
	s.releaseReady()
}

// finalFinishStartup is the backstop for lateInit, which has already released StartHeadless by the time this is
// reached on the ordinary path.
func (s *headlessState) finalFinishStartup() {
	s.releaseReady()
}

// releaseReady lets StartHeadless go. Closing an already-closed channel panics rather than merely being redundant, and
// this is deliberately reached from more than one point in startup, so it is guarded. UI thread only, so a plain bool
// suffices.
func (s *headlessState) releaseReady() {
	if !s.readyClosed {
		s.readyClosed = true
		close(s.ready)
	}
}

// terminate ends the session. It must be safe to call more than once: RunModal's deferred Dispose can close the last
// window, which re-enters quitting() and arrives back here.
func (s *headlessState) terminate() error {
	// Release every nested event loop. RunModal spins on "for w.inModal" and the source side of a drag & drop spins on
	// "for s.drag != nil"; a quit issued from inside a dialog's handler, or from a drop, unwinds through here while
	// one of those loops is still running, and with no events left to arrive and nothing to clear the condition it
	// would spin forever.
	for _, w := range modalStack {
		w.inModal = false
	}
	modalStack = nil
	if session := s.drag; session != nil {
		// A drag still in flight ends here, and it ends canceled: nothing dropped, and nothing ever will. Recording
		// that is what lets a HeadlessDrag handle, or LastDrag(), report something truthful about it afterwards rather
		// than the zero value, whose Canceled is false. No target callbacks are run, since the windows are about to go.
		s.lastDrag = HeadlessDragResult{
			Source:   session.source,
			Target:   session.target,
			Data:     cloneDragData(session.info.data),
			Image:    session.img,
			Origin:   session.origin,
			Canceled: true,
		}
		s.drag = nil
	}
	s.terminated.Store(true)
	s.wakeUp()
	return nil
}

func (s *headlessState) beep() {
	s.beeps.Add(1)
}

// isColorModeTrackingPossible reports true: the session answers the dark mode question itself, so thememode.Auto is
// meaningful here even though there is no OS setting behind it.
func (s *headlessState) isColorModeTrackingPossible() bool {
	return true
}

func (s *headlessState) isDarkModeEnabled() bool {
	return s.darkMode
}

// headlessDoubleClickInterval is the maximum delay between clicks that still registers as a double-click. It matches
// what the Linux and Windows backends report, so tests see the same timing rules everywhere.
const headlessDoubleClickInterval = 500 * time.Millisecond

func (s *headlessState) doubleClickInterval() time.Duration {
	return headlessDoubleClickInterval
}

// waitEvents blocks until there is something to do, then dispatches everything that is pending.
//
// The wait is skipped whenever work is already waiting, and that is what makes a nested event loop possible. The wake
// channel holds a single token however many posts collapse into it, so a pass that has taken the token may go on to
// dispatch an event whose handler starts a loop of its own — RunModal does, and so does the source side of a drag &
// drop. That loop arrives back here with the token already spent, and blocking on another one would strand the events
// still queued behind the one it is running, the tasks waiting to be run and the windows waiting to be redrawn, none
// of which anything is going to post a fresh wake-up for.
//
// A token left in the channel by work that an earlier pass had already dealt with is harmless in the other direction:
// it buys one pass that finds nothing to do and comes straight back here to wait properly.
func (s *headlessState) waitEvents() {
	if !s.terminated.Load() && s.quiescent() {
		<-s.wake
	}
	s.drainInput()
}

func (s *headlessState) postEmptyEvent() {
	s.wakeUp()
}

// withAutoreleasePool runs f directly: autorelease pools are a macOS concept with nothing to reclaim here.
func (s *headlessState) withAutoreleasePool(f func()) {
	f()
}
