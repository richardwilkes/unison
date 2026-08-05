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
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/thememode"
)

// TestThemeModeConcurrentAccess verifies that CurrentThemeMode may be called from arbitrary goroutines while the UI
// thread changes the mode via SetThemeMode. The platform theme monitors (e.g. the Windows registry watcher goroutine)
// read the mode this way, so the access must be race-free: prior to currentThemeMode becoming atomic, this test failed
// under the race detector. This test shares global state and therefore must not call t.Parallel.
func TestThemeModeConcurrentAccess(t *testing.T) {
	c := check.New(t)
	resetTaskQueue() // SetThemeMode queues a ThemeChanged task; keep the shared queue clean for other tests.
	prev := CurrentThemeMode()
	t.Cleanup(func() {
		SetThemeMode(prev)
		resetTaskQueue()
	})

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					mode := CurrentThemeMode()
					c.True(mode == thememode.Auto || mode == thememode.Dark || mode == thememode.Light,
						"unexpected theme mode")
				}
			}
		}()
	}
	for i := range 1000 {
		if i%2 == 0 {
			SetThemeMode(thememode.Dark)
		} else {
			SetThemeMode(thememode.Light)
		}
	}
	close(stop)
	wg.Wait()
	c.Equal(thememode.Light, CurrentThemeMode())
}

// swapWindowList installs the given windows as the complete window list for the duration of the test, restoring the
// previous list when the test completes.
func swapWindowList(t *testing.T, windows ...*Window) {
	t.Helper()
	saved := windowList
	windowList = windows
	t.Cleanup(func() { windowList = saved })
}

// withAllowQuitCallback installs f as the allow-quit callback for the duration of the test, restoring the previous
// callback when the test completes.
func withAllowQuitCallback(t *testing.T, f func() bool) {
	t.Helper()
	saved := allowQuitCallback
	allowQuitCallback = f
	t.Cleanup(func() { allowQuitCallback = saved })
}

// guardQuitting prevents quitting() from exiting the test process and returns a pointer to a flag recording whether
// it was invoked.
func guardQuitting(t *testing.T) *bool {
	t.Helper()
	quit := false
	guardQuittingWith(t, func() { quit = true })
	return &quit
}

// guardQuittingWith installs f as the quitting callback and prevents quitting() from exiting the test process, both
// only for the duration of the test. Setting calledAtExit keeps quitting() from calling xos.Exit; it is also what the
// signal path would have done by the time quitting() runs, since xos.Exit invokes the registered exit functions in
// reverse order of registration and the one that sets calledAtExit is registered last. Since the package was never
// initialized, the finishQuit() quitting() then falls through to fails fast with a logged error before touching any
// platform APIs.
func guardQuittingWith(t *testing.T, f func()) {
	t.Helper()
	quitLock.Lock()
	savedCalledAtExit := calledAtExit
	savedQuittingCallback := quittingCallback
	calledAtExit = true
	quittingCallback = f
	quitLock.Unlock()
	t.Cleanup(func() {
		quitLock.Lock()
		calledAtExit = savedCalledAtExit
		quittingCallback = savedQuittingCallback
		quitLock.Unlock()
	})
}

// withUIThreadIdentity makes the calling goroutine pass onUIThread's check and marks the platform as initialized, both
// only for the duration of the test, so the tests can drive the UI-thread-marshaling logic without a live windowing
// system. Tests using this must pump the task queue themselves, as the event loop would.
func withUIThreadIdentity(t *testing.T) {
	t.Helper()
	savedID := uiGoroutineID.Load()
	savedInited := platformInited.Load()
	uiGoroutineID.Store(currentGoroutineID())
	platformInited.Store(true)
	t.Cleanup(func() {
		uiGoroutineID.Store(savedID)
		platformInited.Store(savedInited)
	})
}

// waitForQueuedTask blocks until at least one task is pending on the shared queue, failing the test rather than
// hanging the test binary if none ever appears.
func waitForQueuedTask(t *testing.T) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if length, head := taskQueueState(); length > head {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no task was queued")
}

// pumpTasksUntil services the task queue from the calling goroutine, the way the event loop would, until done is
// closed. It fails the test rather than hanging the test binary if that takes too long, so a regression in the
// marshaling surfaces as a failure.
func pumpTasksUntil(t *testing.T, done <-chan struct{}) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-done:
			return
		case <-deadline:
			t.Fatal("timed out waiting for the off-thread quitting() to return")
		default:
			processNextTask()
			time.Sleep(time.Millisecond)
		}
	}
}

// newCloseTestWindow returns a minimal Window (via newRedrawTestWindow) whose AllowCloseCallback increments *asked and
// returns allow. When the close is permitted, the WillCloseCallback simulates the platform teardown that destroy()
// would perform — marking the window invalid so Dispose skips the native teardown, which cannot run in a headless
// test, and removing it from the window list.
func newCloseTestWindow(allow bool, asked *int) *Window {
	w := newRedrawTestWindow()
	w.AllowCloseCallback = func() bool {
		*asked++
		return allow
	}
	w.WillCloseCallback = func() {
		w.valid = false
		windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
	}
	return w
}

// TestCloseAllWindowsAsksEachWindowOnce is the regression test for closeAllWindows both re-invoking a vetoing
// window's AllowCloseCallback and never asking the windows behind it: the old loop detected the lack of progress only
// after requestClose had been re-invoked on the vetoing front window, and then gave up entirely. Each window must be
// asked exactly once, a veto must leave that window open without preventing the remaining windows from being asked,
// and the result must report that not all windows closed. This test mutates global state and therefore must not call
// t.Parallel.
func TestCloseAllWindowsAsksEachWindowOnce(t *testing.T) {
	c := check.New(t)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	var frontAsked, midAsked, backAsked int
	front := newCloseTestWindow(false, &frontAsked)
	mid := newCloseTestWindow(true, &midAsked)
	back := newCloseTestWindow(true, &backAsked)
	swapWindowList(t, front, mid, back)
	c.False(closeAllWindows(), "closeAllWindows must report failure when a window vetoes")
	c.Equal(1, frontAsked, "the vetoing window must be asked exactly once, not re-asked")
	c.Equal(1, midAsked, "windows behind a vetoing window must still be asked")
	c.Equal(1, backAsked, "windows behind a vetoing window must still be asked")
	c.Equal(1, len(windowList), "only the vetoing window may remain open")
	c.True(windowList[0] == front, "only the vetoing window may remain open")
	c.True(front.IsValid())
	c.False(mid.IsValid())
	c.False(back.IsValid())
}

// TestCloseAllWindowsClosesAll verifies that closeAllWindows reports success once every window has permitted the
// close and been removed from the window list. This test mutates global state and therefore must not call t.Parallel.
func TestCloseAllWindowsClosesAll(t *testing.T) {
	c := check.New(t)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	var oneAsked, twoAsked int
	swapWindowList(t, newCloseTestWindow(true, &oneAsked), newCloseTestWindow(true, &twoAsked))
	c.True(closeAllWindows(), "closeAllWindows must report success when every window closes")
	c.Equal(1, oneAsked)
	c.Equal(1, twoAsked)
	c.Equal(0, len(windowList))
}

// TestAttemptQuitDeniedByAllowQuitCallback verifies that a denied AllowQuitCallback stops the termination sequence
// before any window is asked to close. This is also the path taken for system-initiated termination on macOS (e.g.
// Dock -> Quit or logout), which previously ignored the callback and exited unconditionally. This test mutates global
// state and therefore must not call t.Parallel.
func TestAttemptQuitDeniedByAllowQuitCallback(t *testing.T) {
	c := check.New(t)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	withAllowQuitCallback(t, func() bool { return false })
	quit := guardQuitting(t)
	var asked int
	wnd := newCloseTestWindow(true, &asked)
	swapWindowList(t, wnd)
	AttemptQuit()
	c.False(*quit, "a denied AllowQuitCallback must prevent termination")
	c.Equal(0, asked, "no window may be asked to close when the quit itself was denied")
	c.Equal(1, len(windowList), "no window may be closed when the quit itself was denied")
	c.True(wnd.IsValid())
}

// TestAttemptQuitCanceledByWindowVeto verifies that a window vetoing its close (e.g. an unsaved-changes prompt)
// cancels the termination sequence, while windows that permitted the close are still closed. This test mutates global
// state and therefore must not call t.Parallel.
func TestAttemptQuitCanceledByWindowVeto(t *testing.T) {
	c := check.New(t)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	allowQuitConsulted := false
	withAllowQuitCallback(t, func() bool {
		allowQuitConsulted = true
		return true
	})
	quit := guardQuitting(t)
	var vetoAsked, allowAsked int
	veto := newCloseTestWindow(false, &vetoAsked)
	allow := newCloseTestWindow(true, &allowAsked)
	swapWindowList(t, veto, allow)
	AttemptQuit()
	c.True(allowQuitConsulted, "the AllowQuitCallback must be consulted")
	c.False(*quit, "a window veto must prevent termination")
	c.Equal(1, vetoAsked)
	c.Equal(1, allowAsked)
	c.Equal(1, len(windowList), "only the vetoing window may remain open")
	c.True(windowList[0] == veto, "only the vetoing window may remain open")
}

// TestAttemptQuitProceedsWhenPermitted verifies that once the AllowQuitCallback and every window permit it, the
// termination sequence runs. This test mutates global state and therefore must not call t.Parallel.
func TestAttemptQuitProceedsWhenPermitted(t *testing.T) {
	c := check.New(t)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	withAllowQuitCallback(t, func() bool { return true })
	quit := guardQuitting(t)
	var asked int
	swapWindowList(t, newCloseTestWindow(true, &asked))
	AttemptQuit()
	c.True(*quit, "termination must proceed once the quit and all window closes are permitted")
	c.Equal(1, asked)
	c.Equal(0, len(windowList))
}

// TestQuittingFromSignalGoroutineMarshalsToUIThread is the regression test for a SIGINT/SIGTERM crashing the app
// instead of terminating it cleanly. quitting() is registered with xos.RunAtExit, so the signal handler goroutine xos
// installs runs it directly; the window teardown it leads to may only happen on the UI thread, and on macOS AppKit
// traps when it doesn't ("SIGTRAP: trace trap"). The quit must instead be marshaled to the UI thread and the calling
// goroutine must block until it has completed, so the process isn't torn down mid-teardown. This test mutates global
// state and therefore must not call t.Parallel.
func TestQuittingFromSignalGoroutineMarshalsToUIThread(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	withAllowQuitCallback(t, func() bool { return true })
	withUIThreadIdentity(t)

	uiID := currentGoroutineID()
	returned := make(chan struct{})
	var callbackGoroutineID atomic.Uint64
	var callbackRanBeforeReturn atomic.Bool
	guardQuittingWith(t, func() {
		callbackGoroutineID.Store(currentGoroutineID())
		select {
		case <-returned:
		default:
			callbackRanBeforeReturn.Store(true)
		}
	})
	var asked int
	wnd := newCloseTestWindow(true, &asked)
	swapWindowList(t, wnd)

	go func() { // Stands in for the signal handler goroutine calling quitting() via xos.Exit.
		quitting()
		close(returned)
	}()

	// The quit must have been handed to the task queue rather than performed in place, and the caller must still be
	// blocked while the task sits there unserviced.
	waitForQueuedTask(t)
	select {
	case <-returned:
		t.Fatal("quitting() returned without waiting for the marshaled quit to complete")
	case <-time.After(50 * time.Millisecond):
	}

	pumpTasksUntil(t, returned)
	c.Equal(uiID, callbackGoroutineID.Load(), "the quitting callback must run on the UI thread")
	c.True(callbackRanBeforeReturn.Load(), "quitting() must not return before the marshaled quit has run")
	c.Equal(1, asked, "the window must be asked to close")
	c.Equal(0, len(windowList), "the window must be closed")
	c.False(wnd.IsValid())
}

// TestQuittingFromSignalGoroutineWithVetoedQuit verifies the semantic that a veto cannot cancel a signal-initiated
// termination: once xos.Exit has been entered, os.Exit will be called no matter what, so the off-thread quitting()
// must still return promptly after the marshaled quit request is denied. The termination sequence itself must not run,
// leaving the windows untouched, and the process then exits without the orderly teardown. This test mutates global
// state and therefore must not call t.Parallel.
func TestQuittingFromSignalGoroutineWithVetoedQuit(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	withAllowQuitCallback(t, func() bool { return false })
	withUIThreadIdentity(t)

	returned := make(chan struct{})
	var callbackRan atomic.Bool
	guardQuittingWith(t, func() { callbackRan.Store(true) })
	var asked int
	wnd := newCloseTestWindow(true, &asked)
	swapWindowList(t, wnd)

	go func() { // Stands in for the signal handler goroutine calling quitting() via xos.Exit.
		quitting()
		close(returned)
	}()

	pumpTasksUntil(t, returned)
	c.False(callbackRan.Load(), "a denied quit must not run the termination sequence")
	c.Equal(0, asked, "no window may be asked to close when the quit itself was denied")
	c.Equal(1, len(windowList), "no window may be closed when the quit itself was denied")
	c.True(wnd.IsValid())
}

// TestQuittingOnUIThreadRunsInline verifies that a quit already running on the UI thread (the normal Cmd-Q or
// last-window-closed path) is performed in place rather than being bounced through the task queue, which would
// deadlock the event loop or silently defer the teardown. This test mutates global state and therefore must not call
// t.Parallel.
func TestQuittingOnUIThreadRunsInline(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	preventQuitOnLastWindowClosed(t)
	withUIThreadIdentity(t)
	quit := guardQuitting(t)
	swapWindowList(t)

	quitting()
	c.True(*quit, "the quitting callback must be invoked inline when already on the UI thread")
	length, head := taskQueueState()
	c.Equal(0, length, "a quit on the UI thread must not enqueue a task")
	c.Equal(0, head)
}
