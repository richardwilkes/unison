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
	"testing"

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
// it was invoked. Setting calledAtExit keeps quitting() from calling xos.Exit; since the package was never
// initialized, the finishQuit() it then falls through to fails fast with a logged error before touching any platform
// APIs.
func guardQuitting(t *testing.T) *bool {
	t.Helper()
	quit := false
	quitLock.Lock()
	savedCalledAtExit := calledAtExit
	savedQuittingCallback := quittingCallback
	calledAtExit = true
	quittingCallback = func() { quit = true }
	quitLock.Unlock()
	t.Cleanup(func() {
		quitLock.Lock()
		calledAtExit = savedCalledAtExit
		quittingCallback = savedQuittingCallback
		quitLock.Unlock()
	})
	return &quit
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
