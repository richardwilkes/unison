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
	"image/color"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/thememode"
)

// A headless session owns most of the package's mutable globals for as long as it runs, so none of these tests may
// call t.Parallel.

// startHeadlessTest starts a session and arranges for it to be shut down when the test ends, whether it got that far
// on its own or not.
func startHeadlessTest(t *testing.T, cfg HeadlessConfig, options ...StartupOption) *HeadlessScreen {
	t.Helper()
	screen, err := StartHeadless(cfg, options...)
	if err != nil {
		t.Fatalf("unable to start headless session: %v", err)
	}
	t.Cleanup(screen.Stop)
	return screen
}

// newHeadlessTestWindow creates a window of the given content rect and shows it, reporting any failure through t.
func newHeadlessTestWindow(t *testing.T, title string, rect geom.Rect) *Window {
	t.Helper()
	w, err := NewWindow(title)
	if err != nil {
		t.Errorf("unable to create window %q: %v", title, err)
		return nil
	}
	w.SetContentRect(rect)
	w.Show()
	return w
}

// stopActiveHeadlessOnCleanup arranges for whatever session is active when the test ends to be stopped, for tests that
// start their session on a goroutine of their own and so cannot register the screen itself the moment they have it. A
// test that fails while its session is still starting would otherwise leave that session behind, and every headless
// test after it would then fail with "a headless session is already active".
func stopActiveHeadlessOnCleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		if screen := ActiveHeadlessScreen(); screen != nil {
			screen.Stop()
		}
	})
}

// waitForHeadlessEnd waits for the session to end, failing the test rather than hanging the whole binary if it never
// does.
func waitForHeadlessEnd(t *testing.T, screen *HeadlessScreen) {
	t.Helper()
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		screen.Wait()
	}()
	select {
	case <-ended:
	case <-time.After(5 * time.Second):
		t.Fatal("the headless session never ended")
	}
}

// TestHeadlessSessionLifecycle covers a session from end to end: it starts, runs the application's startup callback,
// answers questions about itself while it runs, and leaves nothing of itself behind once it has quit.
func TestHeadlessSessionLifecycle(t *testing.T) {
	c := check.New(t)
	prior := captureHeadlessPriorState()
	var startupRan bool
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300, Scale: 2},
		StartupFinishedCallback(func() {
			startupRan = true
			wnd = newHeadlessTestWindow(t, "lifecycle", geom.NewRect(10, 20, 200, 100))
		}))
	c.True(startupRan, "the startup callback should have run before StartHeadless returned")
	c.NotNil(wnd, "the window created during startup should exist")
	c.Equal(screen, ActiveHeadlessScreen())
	c.Equal(geom.NewSize(400, 300), screen.Size())
	c.Equal(float32(2), screen.Scale())
	c.True(screen.Running())

	var cpuRendering bool
	var contentRect geom.Rect
	var visible bool
	c.True(screen.Do(func() {
		cpuRendering = IsCPURenderingActive()
		contentRect = wnd.ContentRect()
		visible = wnd.IsVisible()
	}))
	c.True(cpuRendering, "a headless session must render on the CPU")
	c.Equal(geom.NewRect(10, 20, 200, 100), contentRect)
	c.True(visible)

	c.True(screen.Quit(), "the session should have quit")
	c.False(screen.Running())
	checkHeadlessSessionReset(c, prior)
}

// headlessPriorState is the package-level state that a session must hand back exactly as it found it, captured before
// the session starts by captureHeadlessPriorState so that checkHeadlessSessionReset can compare against it afterwards.
type headlessPriorState struct {
	menuFactory    MenuFactory
	cursorSettings *cursorSettings
	cursors        []*Cursor
	builtIn        []*Cursor
	cpuRendering   bool
}

func captureHeadlessPriorState() *headlessPriorState {
	prior := &headlessPriorState{
		menuFactory:    defaultMenuFactory,
		cursorSettings: builtCursorSettings,
		cursors:        slices.Clone(cursorList),
		cpuRendering:   cpuRenderingActive,
	}
	for _, p := range builtInCursors() {
		prior.builtIn = append(prior.builtIn, *p)
	}
	return prior
}

// checkHeadlessSessionReset asserts that nothing of a session that has ended is still in place. It runs on the calling
// goroutine, which is only safe once the session's teardown has completed, since that is what releases the globals it
// reads.
func checkHeadlessSessionReset(c check.Checker, prior *headlessPriorState) {
	initTermLock.Lock()
	stillInitialized := initialized
	stillInitializing := initializing
	stillTerminating := terminating
	initTermLock.Unlock()
	c.False(stillInitialized)
	c.False(stillInitializing)
	c.False(stillTerminating)
	c.False(platformInited.Load())
	c.Equal(uint64(0), uiGoroutineID.Load())
	c.Nil(ActiveHeadlessScreen())
	c.Equal(0, len(windowList))
	c.Equal(0, len(modalStack))
	c.Equal(0, len(redrawSet))
	c.Nil(recoveryCallback)
	c.Nil(startupFinishedCallback)
	c.Nil(allowQuitCallback)
	c.True(defaultMenuFactory == prior.menuFactory, "the menu factory should be whatever it was before the session")
	// The cursors are compared against what was there before rather than against nothing, since a test that ran
	// earlier may legitimately have left cursors behind, and what a session owes is to hand back exactly what it found.
	c.True(builtCursorSettings == prior.cursorSettings, "the cursor settings should be whatever they were before")
	c.Equal(prior.cursors, cursorList, "the session's cursors should have been released and the prior ones put back")
	for i, p := range builtInCursors() {
		c.True(*p == prior.builtIn[i], "built-in cursor %d should be whatever it was before the session", i)
	}
	c.Equal("", lastWorkingDir, "the directory the file dialogs remember should not have outlived the session")
	c.Equal(prior.cpuRendering, cpuRenderingActive)
}

// TestHeadlessSequentialSessions verifies that a second session in the same process starts from the same clean slate
// the first one did, which is what lets a single test binary run many of them.
func TestHeadlessSequentialSessions(t *testing.T) {
	c := check.New(t)
	for i := range 2 {
		var windowsAtStartup int
		var optionsAtStartup bool
		options := []StartupOption{
			StartupFinishedCallback(func() {
				windowsAtStartup = WindowCount()
				optionsAtStartup = allowQuitCallback == nil && quittingCallback == nil && defaultMenuFactory == nil
				newHeadlessTestWindow(t, "sequential", geom.NewRect(0, 0, 100, 100))
			}),
		}
		if i == 0 {
			// Only the first session installs these. The second one asserting that they are gone is the point of the
			// test: a session must not leave its startup options behind for whatever runs next.
			options = append(options, AllowQuitCallback(func() bool { return true }),
				QuittingCallback(func() {}))
		}
		screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200}, options...)
		c.Equal(0, windowsAtStartup, "session %d should have started with no windows", i)
		if i != 0 {
			c.True(optionsAtStartup, "session %d should have started with no leftover option callbacks", i)
		}
		var count int
		c.True(screen.Do(func() { count = WindowCount() }))
		c.Equal(1, count)
		c.True(screen.Quit())
	}
}

// TestHeadlessStartRefusals verifies that the failures a test binary must survive are reported as errors rather than
// taking the process down the way Start() would.
func TestHeadlessStartRefusals(t *testing.T) {
	c := check.New(t)
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, cfg := range []HeadlessConfig{
		{},
		{Width: 100, Height: -1},
		{Width: nan, Height: 100},
		{Width: 100, Height: nan},
		{Width: inf, Height: 100},
		{Width: 100, Height: inf},
		{Width: 100, Height: 100, Scale: nan},
		{Width: 100, Height: 100, Scale: inf},
	} {
		_, err := StartHeadless(cfg)
		c.HasError(err, "%+v should be rejected", cfg)
		c.Nil(ActiveHeadlessScreen(), "a refused session should not have been published")
		// The option form reports the same failure the same way, since it validates the same thing.
		c.HasError(Headless(cfg)(startupOption{}), "the option built from %+v should be rejected", cfg)
		c.Nil(ActiveHeadlessScreen())
	}
	cfg, err := HeadlessConfig{Width: 100, Height: 100, Scale: -2}.normalized()
	c.NoError(err)
	c.Equal(float32(1), cfg.Scale, "a scale of zero or less still means 1")

	// The refusals that come from the state of the process rather than from the configuration. Each flag is set by
	// hand and put back, since nothing in a test binary can legitimately be left in these states.
	for _, one := range []struct {
		flag *bool
		name string
	}{
		{flag: &initialized, name: "already initialized"},
		{flag: &initializing, name: "initialization already in progress"},
		{flag: &terminating, name: "termination in progress"},
	} {
		initTermLock.Lock()
		*one.flag = true
		initTermLock.Unlock()
		_, err = StartHeadless(HeadlessConfig{Width: 100, Height: 100})
		initTermLock.Lock()
		*one.flag = false
		initTermLock.Unlock()
		c.HasError(err, "a session should be refused while the process is %s", one.name)
		if err != nil {
			c.Contains(err.Error(), one.name)
		}
		c.Nil(ActiveHeadlessScreen(), "a refused session should not have been published")
	}

	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	_, err = StartHeadless(HeadlessConfig{Width: 100, Height: 100})
	c.HasError(err, "a second session should be refused while one is active")
	// The option form refuses too, rather than swapping the running session out from under itself.
	c.HasError(Headless(HeadlessConfig{Width: 100, Height: 100})(startupOption{}))
	c.True(ActiveHeadlessScreen() == screen, "the running session should still be the published one")
	c.True(screen.Quit())

	// With the first session gone, another one may be started.
	screen = startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	c.True(screen.Quit())
}

// TestHeadlessAwaitStartup covers the wait StartHeadless performs for the session to release it, at the level of the
// two channels involved, since the one ordering that is an error — the session ending without ever releasing its
// caller — has no path through the real startup sequence to reach it by.
func TestHeadlessAwaitStartup(t *testing.T) {
	c := check.New(t)
	hs, err := newHeadlessScreen(HeadlessConfig{Width: 1, Height: 1})
	c.NoError(err)
	close(hs.done)
	err = hs.awaitStartup()
	c.HasError(err, "a session that ended without releasing its caller should be reported as a failure to start")
	if err != nil {
		c.Contains(err.Error(), "ended before startup completed")
	}

	// Both closed, in whichever order the select happens to pick, is the shortest possible successful session.
	hs, err = newHeadlessScreen(HeadlessConfig{Width: 1, Height: 1})
	c.NoError(err)
	hs.releaseReady()
	close(hs.done)
	c.NoError(hs.awaitStartup())
	hs.releaseReady() // a second release is harmless rather than a panic on a closed channel
}

// TestHeadlessThemeModeRestored verifies that a session starts at thememode.Auto whatever mode was set before it, and
// puts that mode back when it ends, so that the first session in a process and every later one start alike.
func TestHeadlessThemeModeRestored(t *testing.T) {
	c := check.New(t)
	found := CurrentThemeMode()
	t.Cleanup(func() { currentThemeMode.Store(int32(found)) })
	currentThemeMode.Store(int32(thememode.Dark))

	var atStartup thememode.Enum
	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100},
		StartupFinishedCallback(func() { atStartup = CurrentThemeMode() }))
	c.Equal(thememode.Auto, atStartup, "the session should have started at Auto rather than inheriting Dark")
	c.True(screen.Do(func() { SetThemeMode(thememode.Light) }))
	c.True(screen.Quit())
	c.Equal(thememode.Dark, CurrentThemeMode(), "the mode from before the session should have been put back")
}

// TestHeadlessCapture verifies that what a window drew can be read back, at the right pixel size and in the right
// place on the screen.
func TestHeadlessCapture(t *testing.T) {
	c := check.New(t)
	background := RGB(0, 0, 255)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 320, Height: 200, Scale: 2, Background: background},
		StartupFinishedCallback(func() {
			w, err := NewWindow("capture")
			if err != nil {
				t.Errorf("unable to create window: %v", err)
				return
			}
			w.Content().DrawCallback = func(gc *Canvas, rect geom.Rect) {
				gc.DrawRect(rect, Red.Paint(gc, rect, paintstyle.Fill))
			}
			w.SetContentRect(geom.NewRect(50, 50, 100, 100))
			w.Show()
			wnd = w
		}))
	c.NotNil(wnd)
	screen.Sync() // the window must have been drawn before there is anything to capture

	img := screen.Capture()
	c.NotNil(img)
	c.Equal(640, img.Bounds().Dx(), "the capture should be the screen size in device pixels")
	c.Equal(400, img.Bounds().Dy(), "the capture should be the screen size in device pixels")
	// The window covers device pixels (100,100) through (300,300), so this is well inside it.
	inside := img.NRGBAAt(200, 200)
	c.True(inside.R > 200 && inside.G < 60 && inside.B < 60 && inside.A == 255,
		"the pixels inside the window should be red, but were %v", inside)
	c.Equal(color.NRGBA{R: 0, G: 0, B: 255, A: 255}, img.NRGBAAt(10, 10),
		"the pixels outside the window should be the configured background")

	wndImg := screen.CaptureWindow(wnd)
	c.NotNil(wndImg)
	c.Equal(200, wndImg.Bounds().Dx(), "the window capture should be its content size in device pixels")
	c.Equal(200, wndImg.Bounds().Dy(), "the window capture should be its content size in device pixels")
	c.True(screen.Quit())
}

// TestHeadlessWindowGeometry covers the window geometry contract: frame changes report themselves, a window can be
// forced back onto the screen, and maximizing uses the display's usable area and can be undone.
func TestHeadlessWindowGeometry(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 320, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "geometry", geom.NewRect(10, 10, 100, 50))
		}))
	c.NotNil(wnd)

	var resized, moved int
	c.True(screen.Do(func() {
		wnd.ResizedCallback = func() { resized++ }
		wnd.MovedCallback = func() { moved++ }
		wnd.SetContentRect(geom.NewRect(20, 10, 100, 50))
	}))
	c.Equal(0, resized, "moving the window should not report a resize")
	c.Equal(1, moved)

	c.True(screen.Do(func() { wnd.SetContentRect(geom.NewRect(20, 10, 150, 60)) }))
	c.Equal(1, resized)
	c.Equal(1, moved, "resizing the window in place should not report a move")

	var rect geom.Rect
	c.True(screen.Do(func() {
		wnd.SetContentRect(geom.NewRect(300, 180, 100, 50))
		wnd.EnsureOnDisplay()
		rect = wnd.ContentRect()
	}))
	c.Equal(geom.NewRect(220, 150, 100, 50), rect, "the window should have been pulled back onto the screen")

	var maximizedTo geom.Rect
	var maximizedFlag bool
	var restoredTo geom.Rect
	var restoredFlag bool
	var reported []bool
	c.True(screen.Do(func() {
		wnd.MaximizedCallback = func(maximized bool) { reported = append(reported, maximized) }
		wnd.Maximize()
		maximizedTo = wnd.ContentRect()
		maximizedFlag = wnd.IsMaximized()
		wnd.Maximize()
		restoredTo = wnd.ContentRect()
		restoredFlag = wnd.IsMaximized()
	}))
	c.True(maximizedFlag)
	c.Equal(geom.NewRect(0, 0, 320, 200), maximizedTo, "maximizing should use the display's usable area")
	c.False(restoredFlag)
	c.Equal(geom.NewRect(220, 150, 100, 50), restoredTo, "restoring should return the window to where it was")
	c.Equal([]bool{true, false}, reported, "each change should have been reported through the callback")
	c.True(screen.Quit())
}

// TestHeadlessMinimizedWindow verifies that a minimized window is treated as being off the screen: it receives no
// input, it contributes nothing to a capture, and it does not keep the keyboard focus.
func TestHeadlessMinimizedWindow(t *testing.T) {
	c := check.New(t)
	background := RGB(0, 0, 255)
	var back, front *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200, Background: background},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(0, 0, 200, 200))
			if back != nil {
				back.Content().DrawCallback = func(gc *Canvas, rect geom.Rect) {
					gc.DrawRect(rect, Green.Paint(gc, rect, paintstyle.Fill))
				}
			}
			front = newHeadlessTestWindow(t, "front", geom.NewRect(0, 0, 100, 100))
			if front != nil {
				front.Content().DrawCallback = func(gc *Canvas, rect geom.Rect) {
					gc.DrawRect(rect, Red.Paint(gc, rect, paintstyle.Fill))
				}
				front.ToFront()
			}
		}))
	c.NotNil(back)
	c.NotNil(front)
	screen.Sync()

	c.True(screen.WindowAt(geom.NewPoint(50, 50)) == front, "the window on top should be the one found at that point")
	img := screen.Capture()
	c.NotNil(img)
	c.Equal(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, img.NRGBAAt(50, 50), "the window on top should have been captured")

	var reported []bool
	c.True(screen.Do(func() {
		front.MinimizedCallback = func(minimized bool) { reported = append(reported, minimized) }
		front.Minimize()
	}))
	c.True(screen.WindowAt(geom.NewPoint(50, 50)) == back, "a minimized window should be skipped by hit testing")
	c.True(screen.FocusedWindow() == back, "a minimized window should not keep the focus")
	img = screen.Capture()
	c.NotNil(img)
	c.Equal(color.NRGBA{R: 0, G: 128, B: 0, A: 255}, img.NRGBAAt(50, 50),
		"a minimized window should contribute nothing to a capture")

	// Put the other window on top while this one is minimized, so that restoring has to raise it as well as give it
	// the focus back.
	c.True(screen.Do(func() { back.ToFront() }))

	// Minimize() toggles, so this restores it.
	c.True(screen.Do(func() { front.Minimize() }))
	c.True(screen.WindowAt(geom.NewPoint(50, 50)) == front, "a restored window should be found again")
	c.True(screen.FocusedWindow() == front, "restoring a window should activate it")
	c.Equal([]bool{true, false}, reported, "each change should have been reported through the callback")

	// ToFront() on a minimized window restores it as well, since activation implies restoration on the platforms:
	// otherwise it would be a focused window that receives every key event yet is skipped by hit testing and by
	// Capture(). Disposing of another window, or a modal dialog returning, reaches this without anything exotic.
	var minimized bool
	c.True(screen.Do(func() {
		reported = nil
		front.Minimize()
		back.ToFront()
		front.ToFront()
		minimized = front.IsMinimized()
	}))
	c.False(minimized, "bringing a minimized window to the front should restore it")
	c.True(screen.WindowAt(geom.NewPoint(50, 50)) == front)
	c.True(screen.FocusedWindow() == front)
	c.Equal([]bool{true, false}, reported, "the restoration should have been reported through the callback")
	img = screen.Capture()
	c.NotNil(img)
	c.Equal(color.NRGBA{R: 255, G: 0, B: 0, A: 255}, img.NRGBAAt(50, 50), "the restored window should be captured")
}

// TestHeadlessMinimizeDuringPress verifies that minimizing a window while a button is down in it releases everything
// the press was holding, as hiding one does: the grab, the buttons it recorded, and the pointer, which is exited from
// the minimized window and handed to whatever is underneath. Without that, every mouse event for the rest of the
// session would go to a window that hit testing pretends is not there, and the buttons would stay recorded as down.
func TestHeadlessMinimizeDuringPress(t *testing.T) {
	c := check.New(t)
	var back, front *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(0, 0, 200, 200))
			front = newHeadlessTestWindow(t, "front", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(back)
	c.NotNil(front)

	var frontDrags, frontExits, backEnters, backMoves int
	c.True(screen.Do(func() {
		front.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
		front.MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			frontDrags++
			return true
		}
		front.MouseExitCallback = func() bool {
			frontExits++
			return false
		}
		back.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			backEnters++
			return false
		}
		back.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			backMoves++
			return false
		}
	}))

	screen.MouseDown(geom.NewPoint(50, 50), ButtonLeft, mod.None)
	var exitedOnMinimize, enteredOnMinimize, buttons int
	var capture, hover *Window
	c.True(screen.Do(func() {
		// The press entered the window, and Window.mouseEnter begins by exiting whatever was entered before, so the
		// counts this test is about start from here.
		frontExits, backEnters = 0, 0
		front.Minimize()
		exitedOnMinimize = frontExits
		enteredOnMinimize = backEnters
		buttons = len(screen.buttons)
		capture = screen.capture
		hover = screen.hover
	}))
	c.True(exitedOnMinimize > 0, "minimizing the window the pointer was in should have taken the pointer out of it")
	// Entered by the crossing, and again by the focus handoff, since Window.gainedFocus enters the window as well.
	c.True(enteredOnMinimize > 0, "the window revealed underneath should have been entered")
	c.Equal(0, buttons, "minimizing the pressed window should have forgotten the button it was holding")
	c.Nil(capture, "minimizing the pressed window should have released the grab")
	c.True(hover == back, "the pointer should now be over the window underneath")

	screen.MouseMove(geom.NewPoint(60, 60), mod.None)
	c.Equal(0, frontDrags, "the minimized window should not have gone on receiving the pointer")
	c.True(backMoves > 0, "the window underneath should be receiving the pointer")

	// The pointer is no longer spoken for, so a drag from outside the application may take it.
	d := screen.BeginExternalDrag(geom.NewPoint(60, 60), drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
	c.Equal(0, len(screen.Errors()), "a drag should have been able to enter: %v", screen.Errors())
	d.Cancel()
}

// TestHeadlessHideDuringPress verifies that hiding a window while a button is down releases everything the press was
// holding onto: the grab, the buttons it recorded, and the pointer itself, which then belongs to whichever window is
// still there.
func TestHeadlessHideDuringPress(t *testing.T) {
	c := check.New(t)
	var a, b *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			a = newHeadlessTestWindow(t, "a", geom.NewRect(0, 0, 100, 100))
			b = newHeadlessTestWindow(t, "b", geom.NewRect(150, 0, 100, 100))
		}))
	c.NotNil(a)
	c.NotNil(b)

	var aDrags, aExits, bEnters, bMoves int
	c.True(screen.Do(func() {
		a.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
		a.MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			aDrags++
			return true
		}
		a.MouseExitCallback = func() bool {
			aExits++
			return false
		}
		b.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			bEnters++
			return false
		}
		b.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			bMoves++
			return false
		}
	}))

	screen.MouseDown(geom.NewPoint(50, 50), ButtonLeft, mod.None)
	var exitedOnHide int
	c.True(screen.Do(func() {
		// The entry that came with the press exited the window first, as Window.mouseEnter always does, so the exits
		// this test is about are counted from zero.
		aExits = 0
		a.Hide()
		exitedOnHide = aExits
		// Handing the focus to the window that is left entered it, since Window.gainedFocus does that as well. The
		// crossing this test is about is the one the move below brings, so that count starts again from here too.
		bEnters = 0
	}))
	c.True(exitedOnHide > 0, "hiding the window the pointer was in should have taken the pointer out of it")

	screen.MouseMove(geom.NewPoint(200, 50), mod.None)
	c.Equal(0, aDrags, "the hidden window should not have gone on receiving the pointer")
	c.Equal(1, bEnters, "the pointer should have been handed to the window that is still on the screen")
	screen.MouseMove(geom.NewPoint(210, 60), mod.None)
	c.True(bMoves > 0, "moving within the window the pointer is now in should report the move")

	var buttons int
	c.True(screen.Do(func() { buttons = len(screen.buttons) }))
	c.Equal(0, buttons, "hiding the pressed window should have forgotten the button it was holding")

	// The pointer is no longer spoken for, so a drag from outside the application may take it.
	d := screen.BeginExternalDrag(geom.NewPoint(200, 50), drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
	c.Equal(0, len(screen.Errors()), "a drag should have been able to enter: %v", screen.Errors())
	d.Cancel()
}

// TestHeadlessQuitFromInsideCallback covers the awkward shape of a quit: the application asks for it from inside a
// widget callback, which is being run by the event loop, which the test goroutine is at that moment waiting on. Nothing
// may deadlock, the driver call must come back, and the session must be as thoroughly torn down as one that was quit
// from the outside.
func TestHeadlessQuitFromInsideCallback(t *testing.T) {
	c := check.New(t)
	prior := captureHeadlessPriorState()
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "quitter", geom.NewRect(0, 0, 100, 100))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	var quit bool
	c.True(screen.Do(func() {
		wnd.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			// The driver's own Quit, from the UI thread: it runs the quit inline and reports whether it was accepted,
			// which it cannot wait for the teardown to confirm, since the teardown is performed by the loop this
			// callback is suspended inside.
			quit = screen.Quit()
			return true
		}
	}))

	clicked := make(chan struct{})
	go func() {
		defer close(clicked)
		screen.Click(geom.NewPoint(50, 50))
	}()
	select {
	case <-clicked:
	case <-time.After(5 * time.Second):
		t.Fatal("the driver call never returned after the application quit from inside a callback")
	}
	// The driver call may return the moment the session stops accepting work, which is a little before the teardown has
	// finished, so wait for the end before looking at what it left behind.
	waitForHeadlessEnd(t, screen)
	c.False(screen.Running())
	c.True(quit, "Quit() from the UI thread should have reported that the quit was accepted")
	checkHeadlessSessionReset(c, prior)
}

// TestHeadlessSessionEndsInsideNestedLoop verifies that a session can be shut down from the outside while a nested
// event loop is running — a modal dialog here — and that the one that follows it is not left holding any of the
// wreckage.
func TestHeadlessSessionEndsInsideNestedLoop(t *testing.T) {
	c := check.New(t)
	prior := captureHeadlessPriorState()
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 300},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "modal host", geom.NewRect(0, 0, 200, 200))
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	c.True(screen.Do(func() {
		wnd.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			d, err := NewDialog(nil, nil, NewMessagePanel("Proceed?", ""), []*DialogButtonInfo{NewOKButtonInfo()})
			if err != nil {
				t.Errorf("unable to create dialog: %v", err)
				return true
			}
			d.RunModal()
			return true
		}
	}))

	// Returns once the application has gone quiet inside the dialog's modal loop.
	screen.Click(geom.NewPoint(50, 50))
	var modals int
	c.True(screen.Do(func() { modals = len(modalStack) }))
	c.Equal(1, modals, "the modal loop should be running")

	screen.Stop()
	c.False(screen.Running())
	checkHeadlessSessionReset(c, prior)

	// The point of the test: whatever the last session was in the middle of, the next one starts from nothing.
	next := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	var windows int
	c.True(next.Do(func() { windows = WindowCount() }))
	c.Equal(0, windows, "the next session should have started with no windows")
	c.True(next.Quit())
}

// TestHeadlessWindowDispatchWithoutSession verifies that the choice of backend is made per window rather than per
// process: a window carrying a headless backing dispatches to it even with no session running. The nil case, which is
// every window in every other test in this package, is covered by those tests.
func TestHeadlessWindowDispatchWithoutSession(t *testing.T) {
	c := check.New(t)
	c.Nil(ActiveHeadlessScreen(), "this test only means anything with no session running")
	hw := &headlessWindow{
		rect:    geom.NewRect(5, 6, 7, 8),
		visible: true,
	}
	w := &Window{
		wnd:   &apiWindow{hw: hw},
		valid: true,
	}
	c.True(w.IsVisible())
	c.Equal(geom.NewRect(5, 6, 7, 8), w.ContentRect())
	c.Equal(geom.NewRect(5, 6, 7, 8), w.FrameRect())
	w.SetTitle("dispatched")
	c.Equal("dispatched", hw.title, "the title should have been routed to the headless backing")
}

// TestHeadlessQuitRefused verifies that a refused quit leaves the session running and that Stop() ends it anyway.
func TestHeadlessQuitRefused(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100},
		AllowQuitCallback(func() bool { return false }),
		StartupFinishedCallback(func() {
			newHeadlessTestWindow(t, "refuses", geom.NewRect(0, 0, 50, 50))
		}))
	c.False(screen.Quit(), "the quit should have been refused")
	c.True(screen.Running())
	var fromUIThread bool
	c.True(screen.Do(func() { fromUIThread = screen.Quit() }))
	c.False(fromUIThread, "a quit attempted from the UI thread should report the refusal too")
	c.True(screen.Running())
	screen.Stop()
	c.False(screen.Running())
	c.Nil(ActiveHeadlessScreen())
}

// TestHeadlessSyncQuiescence verifies that Sync() waits for work that queues more work, right through to the redraws
// it ends up asking for.
func TestHeadlessSyncQuiescence(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "quiescence", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(wnd)

	var first, second, third bool
	c.True(screen.Do(func() {
		first = true
		InvokeTask(func() {
			second = true
			InvokeTask(func() {
				third = true
				wnd.MarkForRedraw()
			})
		})
	}))
	c.True(first)
	c.True(second, "Sync should have waited for the task queued by the first one")
	c.True(third, "Sync should have waited for the task queued by the second one")

	var pendingRedraws int
	c.True(screen.Do(func() { pendingRedraws = len(redrawSet) }))
	c.Equal(0, pendingRedraws, "Sync should have waited for the redraw the last task asked for")
	c.True(screen.Quit())
}

// TestHeadlessErrorsCollected verifies that a panic with no application-supplied RecoveryCallback becomes something
// the test can assert on rather than a line in the log.
func TestHeadlessErrorsCollected(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	c.Equal(0, len(screen.Errors()))
	c.True(screen.Do(func() { panic("headless test panic") }))
	recorded := screen.Errors()
	c.Equal(1, len(recorded))
	c.Contains(recorded[0].Error(), "headless test panic")
	c.True(screen.Running(), "a recovered panic should not end the session")
	c.True(screen.Quit())
}

// TestHeadlessInputRoutingAndFocus verifies that a click lands on the window that is actually on top at that point, and
// that clicking a window that does not have the focus takes it, in the order the platforms hand the focus over.
func TestHeadlessInputRoutingAndFocus(t *testing.T) {
	c := check.New(t)
	var back, front *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(0, 0, 100, 100))
			// Shown second, so this one is on top where the two overlap.
			front = newHeadlessTestWindow(t, "front", geom.NewRect(50, 0, 100, 100))
		}))
	c.NotNil(back)
	c.NotNil(front)

	var backHits, frontHits int
	var order []string
	c.True(screen.Do(func() {
		back.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			backHits++
			return true
		}
		front.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			frontHits++
			return true
		}
		back.GainedFocusCallback = func() { order = append(order, "back gained") }
		back.LostFocusCallback = func() { order = append(order, "back lost") }
		front.GainedFocusCallback = func() { order = append(order, "front gained") }
		front.LostFocusCallback = func() { order = append(order, "front lost") }
	}))

	// Inside both windows, so only the one on top may see it.
	screen.Click(geom.NewPoint(75, 50))
	c.Equal(1, frontHits, "the click should have gone to the window on top")
	c.Equal(0, backHits, "the window underneath should have seen nothing")

	var backFocused, frontFocused bool
	c.True(screen.Do(func() {
		backFocused = back.Focused()
		frontFocused = front.Focused()
		order = nil
	}))
	c.True(frontFocused, "clicking a window should have given it the focus")
	c.False(backFocused)

	// Inside the back window only.
	screen.Click(geom.NewPoint(10, 50))
	c.Equal(1, backHits)
	c.Equal(1, frontHits, "the window that lost the focus should not have seen the click")

	var first *Window
	c.True(screen.Do(func() {
		backFocused = back.Focused()
		frontFocused = front.Focused()
		first = windowList[0]
	}))
	c.True(backFocused, "clicking the unfocused window should have given it the focus")
	c.False(frontFocused)
	c.True(first == back, "the newly focused window should be at the front of the window list")
	c.Equal([]string{"front lost", "back gained"}, order, "the focus should be resigned before it is taken")
}

// TestHeadlessInputCapture verifies the grab a press installs: the window that was pressed keeps receiving the pointer
// however far outside it the pointer goes, and only when the button is released does the pointer belong to whatever
// window it is actually over.
func TestHeadlessInputCapture(t *testing.T) {
	c := check.New(t)
	var a, b *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			a = newHeadlessTestWindow(t, "a", geom.NewRect(0, 0, 100, 100))
			b = newHeadlessTestWindow(t, "b", geom.NewRect(150, 0, 100, 100))
		}))
	c.NotNil(a)
	c.NotNil(b)

	var drags, exits, enters, intrusions int
	c.True(screen.Do(func() {
		a.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
		a.Content().MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			drags++
			return true
		}
		a.MouseExitCallback = func() bool {
			exits++
			return false
		}
		b.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			enters++
			return false
		}
		b.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			intrusions++
			return false
		}
		b.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			intrusions++
			return false
		}
	}))

	screen.MouseDown(geom.NewPoint(50, 50), ButtonLeft, mod.None)
	screen.MouseMove(geom.NewPoint(200, 50), mod.None)
	c.True(drags > 0, "the pressed window should keep receiving the pointer outside its bounds")
	c.Equal(0, enters, "the other window should not be entered while the pointer is grabbed")
	c.Equal(0, intrusions, "the other window should see nothing at all while the pointer is grabbed")

	// The press entered the pressed window, both from the motion and from the focus it brought, and Window.mouseEnter
	// begins by exiting whatever was entered before, so the exits so far are not what this is about. Count from zero.
	// The release then exits the window twice, exactly as it does on the real platforms: once from Window.mouseUp
	// itself, which finds no panel of its own under a release outside the window and exits the one the pointer was
	// last over, and once from the crossing the router synthesizes when the grab ends with the pointer over another
	// window, which is the one this test exists for.
	c.True(screen.Do(func() { exits = 0 }))
	screen.MouseUp(geom.NewPoint(200, 50), ButtonLeft, mod.None)
	c.Equal(2, exits, "releasing elsewhere should take the pointer out of the pressed window")
	c.Equal(1, enters, "releasing elsewhere should put the pointer into the window it was released over")
}

// TestHeadlessInputCrossingAndWheel verifies that plain moves synthesize the window crossings, and that the wheel goes
// to the window under the pointer rather than to the one holding the focus.
func TestHeadlessInputCrossingAndWheel(t *testing.T) {
	c := check.New(t)
	var a, b *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			a = newHeadlessTestWindow(t, "a", geom.NewRect(0, 0, 100, 100))
			b = newHeadlessTestWindow(t, "b", geom.NewRect(150, 0, 100, 100))
		}))
	c.NotNil(a)
	c.NotNil(b)

	var aEnters, aExits, aMoves, aWheels, bEnters, bWheels int
	c.True(screen.Do(func() {
		// Give the focus to a window that is not the one the wheel will be aimed at. Done before the callbacks are
		// installed, since taking the focus enters the window and would otherwise be counted below.
		a.ToFront()
		a.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			aEnters++
			return false
		}
		a.MouseExitCallback = func() bool {
			aExits++
			return false
		}
		a.MouseMoveCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			aMoves++
			return false
		}
		a.MouseWheelCallback = func(_, _ geom.Point, _ mod.Modifiers) bool {
			aWheels++
			return false
		}
		b.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			bEnters++
			return false
		}
		b.MouseWheelCallback = func(_, _ geom.Point, _ mod.Modifiers) bool {
			bWheels++
			return false
		}
	}))

	screen.MouseMove(geom.NewPoint(25, 25), mod.None)
	c.Equal(1, aEnters, "moving onto a window should enter it")

	// Start the crossing assertions from zero. The entry above came with an exit of its own, since Window.mouseEnter
	// begins by taking the pointer out of wherever it was before, and that is not the exit this is about.
	c.True(screen.Do(func() { aEnters, aExits, aMoves, bEnters = 0, 0, 0, 0 }))

	screen.MouseMove(geom.NewPoint(35, 35), mod.None)
	c.Equal(0, aEnters, "moving within a window should not enter it again")
	c.Equal(0, aExits, "moving within a window should not exit it")
	c.True(aMoves > 0, "moving within a window should report the move")
	c.Equal(0, bEnters)

	screen.MouseMove(geom.NewPoint(200, 50), mod.None)
	c.Equal(1, aExits, "moving to another window should exit the one being left")
	c.Equal(1, bEnters, "moving to another window should enter it")
	c.Equal(0, aEnters)

	c.True(screen.FocusedWindow() == a, "the focus should not have followed the pointer")

	screen.Wheel(geom.NewPoint(200, 50), geom.NewPoint(0, 1), mod.None)
	c.Equal(1, bWheels, "the wheel should go to the window under the pointer")
	c.Equal(0, aWheels, "the wheel should not go to the focused window")

	// A wheel over a window other than the one the pointer was last in is a crossing like any other: the window being
	// left is exited once and the one being wheeled is entered once, and the move that follows adds nothing to that.
	var bExits int
	c.True(screen.Do(func() {
		b.MouseExitCallback = func() bool {
			bExits++
			return false
		}
		aEnters, aExits, bEnters = 0, 0, 0
	}))
	screen.Wheel(geom.NewPoint(25, 25), geom.NewPoint(0, 1), mod.None)
	c.Equal(1, aWheels, "the wheel should go to the window it is over")
	c.Equal(1, aEnters, "wheeling over another window should enter it")
	c.Equal(1, bExits, "wheeling over another window should exit the one being left")
	// The entry came with an exit of its own, as every Window.mouseEnter does, which is not the exit this is about.
	c.True(screen.Do(func() { aExits = 0 }))
	screen.MouseMove(geom.NewPoint(30, 30), mod.None)
	c.Equal(1, aEnters, "the move after the wheel should not enter the window a second time")
	c.Equal(0, aExits, "the move after the wheel should not exit the window the pointer is still in")
	c.Equal(1, bExits)

	// While a button is down the pointer is grabbed, and the wheel goes to the window holding the grab wherever the
	// pointer is, as it does under the platforms' grabs.
	c.True(screen.Do(func() {
		a.Content().MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	}))
	screen.MouseDown(geom.NewPoint(30, 30), ButtonLeft, mod.None)
	screen.Wheel(geom.NewPoint(200, 50), geom.NewPoint(0, 1), mod.None)
	c.Equal(2, aWheels, "the wheel should go to the window holding the grab")
	c.Equal(1, bWheels, "the window under the pointer should not see the wheel while another holds the grab")
	c.Equal(0, bEnters, "the window under the pointer should not be entered while another holds the grab")
	screen.MouseUp(geom.NewPoint(200, 50), ButtonLeft, mod.None)
}

// TestHeadlessClickCounts verifies that clicks issued microseconds apart are still counted the way a person clicking
// would expect: separate clicks stay separate, and a double-click is a double-click.
func TestHeadlessClickCounts(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "clicks", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(wnd)

	var counts []int
	c.True(screen.Do(func() {
		wnd.Content().MouseDownCallback = func(_ geom.Point, _, clickCount int, _ mod.Modifiers) bool {
			counts = append(counts, clickCount)
			return true
		}
		wnd.ToFront()
	}))

	at := geom.NewPoint(50, 50)
	screen.DoubleClick(at)
	c.Equal([]int{1, 2}, counts, "a double-click should report a count of two on its second press")

	c.True(screen.Do(func() { counts = nil }))
	screen.Click(at)
	screen.Click(at)
	c.Equal([]int{1, 1}, counts, "consecutive clicks should each be a click of their own")

	// A drag that follows a click at the same spot is a press of its own too, rather than the second press of a
	// double-click, which widgets that special-case a double-click press would otherwise see.
	c.True(screen.Do(func() { counts = nil }))
	screen.Click(at)
	screen.Drag(at, at.Add(geom.NewPoint(30, 0)), 3)
	c.Equal([]int{1, 1}, counts, "the press that starts a drag should be the first of its series")
}

// TestHeadlessInputBeforeProbe verifies the ordering Sync depends on: an event that was posted before the probe task
// was queued is always dispatched before that task runs, since a pass of the event loop drains every pending event
// before it runs a task.
func TestHeadlessInputBeforeProbe(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	var order []string
	screen.post(func() { order = append(order, "input") })
	c.True(screen.Do(func() { order = append(order, "probe") }))
	c.Equal([]string{"input", "probe"}, order, "the queued input should have been dispatched first")
	c.True(screen.Quit())
}

// TestHeadlessKeyInput verifies that key events reach the focused panel of the focused window, and that a key only
// produces a rune when the modifiers held with it leave it as text rather than as a command.
func TestHeadlessKeyInput(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var panel *Panel
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "keys", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			panel = NewPanel()
			panel.SetFocusable(true)
			wnd.Content().AddChild(panel)
			wnd.ToFront()
			wnd.SetFocus(panel)
		}))
	c.NotNil(wnd)
	c.NotNil(panel)

	var events []string
	c.True(screen.Do(func() {
		panel.KeyDownCallback = func(code KeyCode, mods mod.Modifiers, _ bool) bool {
			events = append(events, "down "+code.Key()+" "+mods.Key())
			return true
		}
		panel.RuneTypedCallback = func(ch rune) bool {
			events = append(events, "rune "+string(ch))
			return true
		}
		panel.KeyUpCallback = func(code KeyCode, mods mod.Modifiers) bool {
			events = append(events, "up "+code.Key()+" "+mods.Key())
			return true
		}
	}))

	screen.KeyPress(KeyA, mod.Shift)
	c.Equal([]string{"down A shift", "rune A", "up A shift"}, events,
		"a shifted letter should be delivered as the key and the rune it types")

	c.True(screen.Do(func() { events = nil }))
	screen.KeyPress(KeyA, mod.Control)
	c.Equal([]string{"down A ctrl", "up A ctrl"}, events, "a key held with control types no rune")
}

// TestHeadlessDragSteps verifies the shape of an injected drag: the button goes down at the start, the pointer arrives
// in the number of steps asked for, the last of them lands exactly on the destination, and the button comes back up
// there.
func TestHeadlessDragSteps(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "drag", geom.NewRect(0, 0, 200, 100))
		}))
	c.NotNil(wnd)

	var downs, ups int
	var dragged []geom.Point
	c.True(screen.Do(func() {
		wnd.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			downs++
			return false
		}
		wnd.MouseDragCallback = func(where geom.Point, _ int, _ mod.Modifiers) bool {
			dragged = append(dragged, where)
			return false
		}
		wnd.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			ups++
			return false
		}
		wnd.ToFront()
	}))

	// The window is at the origin, so its coordinate space and the screen's are the same one here.
	from := geom.NewPoint(20, 20)
	to := geom.NewPoint(120, 60)
	screen.Drag(from, to, 4)
	c.Equal(1, downs)
	c.Equal(1, ups)
	c.Equal(4, len(dragged), "the drag should have taken the number of steps asked for")
	if len(dragged) != 0 {
		c.Equal(to, dragged[len(dragged)-1], "the last step should land exactly on the destination")
		c.True(dragged[0] != from, "the first step should have moved away from where the press was")
	}

	c.True(screen.Do(func() { dragged = nil }))
	screen.Drag(from, to, 0)
	c.Equal(1, len(dragged), "a drag of fewer than one step should still take one")
	c.Equal(to, dragged[0])
}

// TestHeadlessNestedLoopSeesQueuedInput is the regression test for the wait in waitEvents: a nested event loop must
// dispatch the input that was already queued behind the event that started it, rather than waiting for a wake-up that
// nothing is going to send. The wake channel holds a single token however many events collapse into it, so the pass
// that took the token is the one running the handler here, and the events still queued behind that handler are the
// only thing that can end the loop it starts.
//
// The loop is hand-rolled rather than RunModal's, even though RunModal's is one of the two loops this is really about
// (the source side of a drag & drop is the other). Both of those do enough work on their way in — creating a window,
// moving the focus, queuing a task — to leave a wake-up pending by accident, which would release the wait and hide the
// hazard. Draining the channel first and then spinning on processEvents is the same loop with none of that noise.
func TestHeadlessNestedLoopSeesQueuedInput(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "nested", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(wnd)

	var pressed, released bool
	c.True(screen.Do(func() {
		wnd.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			pressed = true
			select {
			case <-screen.wake:
			default:
			}
			for !released && !screen.terminated.Load() {
				processEvents()
			}
			return true
		}
	}))

	// Both events are queued from a task, which is to say from the UI thread itself, so that the event loop cannot
	// pick the first one up before the second has joined it. They are posted rather than injected through the driver
	// because the injection methods Sync after each event, and the probe tasks that involves would post wake-ups of
	// their own.
	done := make(chan struct{})
	c.True(screen.Post(func() {
		screen.post(func() { screen.buttonPressed(geom.NewPoint(50, 50), ButtonLeft, mod.None) })
		screen.post(func() {
			released = true
			close(done)
		})
	}))
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the nested loop never dispatched the event that was queued behind the press that started it")
	}
	c.True(pressed, "the press should have reached the panel")
}

// TestHeadlessDragInfo covers the drag.Info a headless session hands to drop targets: which types it reports, how it
// decodes the composite ones from the lines they are supplied as, and what it makes of a source that named no
// operations.
func TestHeadlessDragInfo(t *testing.T) {
	c := check.New(t)
	info := &headlessDragInfo{
		data: []drag.Data{
			{Type: uti.UTF8PlainText, Data: []byte("payload")},
			{Type: uti.FileURL, Data: []byte("/a\n/b\n\n")},
			{Type: uti.URL, Data: []byte("https://example.com/one\n\n%zz\nhttps://example.com/two\n")},
			{Type: uti.UTF8PlainText, Data: []byte("a second entry of a type already present")},
		},
	}
	c.Equal([]string{uti.UTF8PlainText.UTI, uti.FileURL.UTI, uti.URL.UTI}, info.DataTypes(),
		"the types should be reported once each, in the order they were supplied")

	c.True(info.HasString())
	c.Equal("payload", info.Text(), "the first entry of a type is the one that answers for it")
	c.True(info.HasDataType(uti.FileURL.UTI))
	c.False(info.HasDataType(uti.PNG.UTI))
	c.Nil(info.Data(uti.PNG.UTI))

	// A request is satisfied by conformance in either direction, as a window's registration is, so the panel-level
	// checks find what the window was admitted for: plain text satisfies a request for UTF-8 text and vice versa,
	// while an exact match is preferred whenever there is one.
	c.True(info.HasDataType(uti.PlainText.UTI), "UTF-8 text should satisfy a request for its parent type")
	c.Equal("payload", string(info.Data(uti.PlainText.UTI)))
	c.True(info.HasDataType(uti.Text.UTI))
	conforming := &headlessDragInfo{data: []drag.Data{{Type: uti.PlainText, Data: []byte("plain")}}}
	c.True(conforming.HasString(), "plain text should satisfy a request for UTF-8 text")
	c.Equal("plain", conforming.Text())
	c.False(conforming.HasFilePaths())
	c.False(conforming.HasDataType("com.example.unregistered"), "an unregistered type can only be matched exactly")
	both := &headlessDragInfo{data: []drag.Data{
		{Type: uti.PlainText, Data: []byte("plain")},
		{Type: uti.UTF8PlainText, Data: []byte("utf8")},
	}}
	c.Equal("utf8", both.Text(), "an exact match should win over a conforming one that was supplied first")

	c.True(info.HasFilePaths())
	c.Equal([]string{"/a", "/b"}, info.FilePaths(), "the blank line should have been skipped")

	c.True(info.HasURLs())
	urls := info.URLs()
	c.Equal(2, len(urls), "the line that is not a URL should have been skipped")
	if len(urls) == 2 {
		c.Equal("https://example.com/one", urls[0].String())
		c.Equal("https://example.com/two", urls[1].String())
	}

	c.Equal(drag.Copy, info.SourceDragOpMask(), "a source that named no operations means a copy")
	info.opMask = drag.Move
	c.Equal(drag.Move, info.SourceDragOpMask())

	empty := &headlessDragInfo{}
	c.Equal(0, len(empty.DataTypes()))
	c.False(empty.HasString())
	c.False(empty.HasFilePaths())
	c.False(empty.HasURLs())
	c.Equal("", empty.Text())
	c.Nil(empty.FilePaths())
}

// TestHeadlessDragResultData verifies that the data a finished drag hands back is a copy of its own: it must share
// nothing with the drag.Info the target callbacks were given, so that neither a caller mutating the result nor a
// target that kept its info can reach into what the other one sees. Both endings are covered, since a drop and a
// cancel build their results separately.
func TestHeadlessDragResultData(t *testing.T) {
	c := check.New(t)
	const payload = "payload"
	var wnd *Window
	var seen drag.Info
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "drop target", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			wnd.RegisterForDragTypes(uti.UTF8PlainText)
			content := wnd.Content()
			content.DragEnteredCallback = func(di drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op {
				seen = di
				return drag.Copy
			}
			content.DragUpdatedCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op { return drag.Copy }
			content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
		}))
	c.NotNil(wnd)

	// The drag starts outside every window, which is where one coming from another application starts.
	at := geom.NewPoint(50, 50)
	dropped := screen.DropExternal(geom.NewPoint(200, 150), at, 1, drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte(payload)})
	c.True(dropped.Dropped)
	var info drag.Info
	c.True(screen.Do(func() { info = seen }))
	checkHeadlessDragResultData(c, screen, dropped, info, payload)

	canceled := screen.BeginExternalDrag(at, drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte(payload)}).Cancel()
	c.True(canceled.Canceled)
	c.True(screen.Do(func() { info = seen }))
	checkHeadlessDragResultData(c, screen, canceled, info, payload)

	// Every LastDrag call is handed a copy of its own, so one caller scribbling on what it took away cannot be seen by
	// the next.
	first := screen.LastDrag()
	c.Equal(1, len(first.Data))
	if len(first.Data) == 1 && len(first.Data[0].Data) != 0 {
		first.Data[0].Data[0] = '!'
	}
	second := screen.LastDrag()
	c.Equal(1, len(second.Data))
	if len(second.Data) == 1 {
		c.Equal(payload, string(second.Data[0].Data),
			"a later LastDrag call should not see what an earlier caller wrote")
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// checkHeadlessDragResultData asserts that result.Data is a copy of what the target was handed rather than the very
// same storage. seen is the drag.Info the target recorded, and what it holds is read on the UI thread, which is the
// thread the session hands it to.
func checkHeadlessDragResultData(c check.Checker, screen *HeadlessScreen, result HeadlessDragResult, seen drag.Info,
	payload string,
) {
	c.Equal(1, len(result.Data))
	if len(result.Data) != 1 || len(result.Data[0].Data) == 0 {
		return
	}
	c.Equal(payload, string(result.Data[0].Data))
	info, ok := seen.(*headlessDragInfo)
	c.True(ok, "the target should have been handed the session's own drag.Info")
	if !ok {
		return
	}
	var count int
	var shared bool
	c.True(screen.Do(func() {
		if count = len(info.data); count == 1 && len(info.data[0].Data) != 0 {
			shared = &info.data[0].Data[0] == &result.Data[0].Data[0]
		}
	}))
	c.Equal(1, count, "the target should have been handed the one datum the drag was carrying")
	c.False(shared, "the result should not be carrying the session's own storage")

	// The proof that it is a copy: what the target was given is untouched by a caller scribbling on the result.
	result.Data[0].Data[0] = '!'
	var text string
	c.True(screen.Do(func() { text = info.Text() }))
	c.Equal(payload, text, "mutating the result should not have altered what the drag delivered")
}

// headlessMenuItemPanels returns the panels of the items in a menu bar or an open menu, in the order they appear.
func headlessMenuItemPanels(p *menuPanel) []*Panel {
	children := p.Children()
	if len(children) == 0 {
		return nil
	}
	sp, ok := children[0].Self.(*ScrollPanel)
	if !ok {
		return nil
	}
	return sp.Content().AsPanel().Children()
}

// TestHeadlessInWindowMenuBar drives the pure-Go menu bar the way a person would: click a title on the bar to open its
// menu, then click an item in the menu that opened. A headless session always uses this menu implementation, since the
// only alternative is macOS's global menu bar, which has no screen to appear on.
func TestHeadlessInWindowMenuBar(t *testing.T) {
	c := check.New(t)
	const (
		testMenuID = UserBaseID + 1
		testItemID = UserBaseID + 2
	)
	var wnd *Window
	var bar, sub Menu
	activated := 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "menus", geom.NewRect(0, 0, 300, 200))
			if wnd == nil {
				return
			}
			factory := DefaultMenuFactory()
			if _, ok := factory.(*inWindowMenuFactory); !ok {
				// Anything else would be the native factory, which on macOS aborts the whole process rather than
				// failing the test when its bar is installed from the session's UI goroutine.
				t.Errorf("the session should be using the in-window menu factory, not %T", factory)
				return
			}
			bar = factory.BarForWindow(wnd, func(m Menu) {
				f := m.Factory()
				sub = f.NewMenu(testMenuID, "Test", nil)
				sub.InsertItem(-1, f.NewItem(testItemID, "Do It", KeyBinding{}, nil, func(MenuItem) { activated++ }))
				m.InsertMenu(-1, sub)
			})
			wnd.ToFront()
		}))
	c.NotNil(wnd)
	c.NotNil(bar)
	c.NotNil(sub)
	screen.Sync() // the bar has to have been laid out before there is anywhere to aim at

	var title *Panel
	c.True(screen.Do(func() {
		if panels := headlessMenuItemPanels(wnd.root.menuBarPanel); len(panels) != 0 {
			title = panels[0]
		}
	}))
	c.NotNil(title, "the menu bar should carry the one title that was added to it")
	screen.Click(screen.PanelCenter(title))

	var item *Panel
	c.True(screen.Do(func() {
		if m, ok := sub.(*menu); ok && m.popupPanel != nil {
			if panels := headlessMenuItemPanels(m.popupPanel); len(panels) != 0 {
				item = panels[0]
			}
		}
	}))
	c.NotNil(item, "clicking the title should have opened the menu")

	screen.Click(screen.PanelCenter(item))
	var count int
	var stillOpen bool
	c.True(screen.Do(func() {
		count = activated
		m, ok := sub.(*menu)
		stillOpen = ok && m.popupPanel != nil
	}))
	c.Equal(1, count, "clicking the item should have run its handler exactly once")
	c.False(stillOpen, "choosing an item should have closed the menu")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessPriorMenuFactory verifies that a menu factory built before a session — which is the native one, since
// nothing outside a session sets noGlobalMenuBar — is neither used by the session nor lost to it. On macOS that is
// what stands between a headless test and an AppKit abort of the entire test binary: the native factory installs its
// bar with cocoa.SetMainMenu, which AppKit refuses on any thread but the process main thread, and the session's UI
// goroutine is not it.
func TestHeadlessPriorMenuFactory(t *testing.T) {
	c := check.New(t)
	// Registered before the session is started, so that it runs after the session's own cleanup has stopped it.
	found := defaultMenuFactory
	t.Cleanup(func() { defaultMenuFactory = found })
	defaultMenuFactory = nil
	prior := DefaultMenuFactory()
	c.NotNil(prior)
	c.True(prior == defaultMenuFactory, "DefaultMenuFactory() should have installed the factory it built")

	var wnd *Window
	var inSession MenuFactory
	var bar Menu
	var atStartup MenuFactory
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			atStartup = defaultMenuFactory
			wnd = newHeadlessTestWindow(t, "prior-factory", geom.NewRect(0, 0, 300, 200))
			if wnd == nil {
				return
			}
			inSession = DefaultMenuFactory()
			if _, ok := inSession.(*inWindowMenuFactory); !ok {
				// Installing the native factory's bar from here would abort the process on macOS, which is a far less
				// useful report than this one.
				return
			}
			bar = inSession.BarForWindow(wnd, func(m Menu) {
				m.InsertMenu(-1, m.Factory().NewMenu(UserBaseID+1, "Test", nil))
			})
		}))
	c.NotNil(wnd)
	c.Nil(atStartup, "the session should have taken the prior factory out of the way before startup")
	c.NotNil(inSession)
	_, ok := inSession.(*inWindowMenuFactory)
	c.True(ok, "the session should build the in-window factory, not use the prior %T", inSession)
	c.True(inSession != prior, "the session should not have adopted the factory built before it")
	c.True(inSession.BarIsPerWindow(), "the session's menu bar should be per-window")
	c.NotNil(bar)
	var barInstalled bool
	c.True(screen.Do(func() { barInstalled = wnd.root.menuBarPanel != nil }))
	c.True(barInstalled, "the in-window bar should have been installed in the window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())

	screen.Stop()
	c.True(defaultMenuFactory == prior, "the session should have put the prior factory back when it ended")
	c.True(DefaultMenuFactory() == prior, "DefaultMenuFactory() should hand out the prior factory again")
}

// TestHeadlessQuitDuringStartup verifies the shortest possible session: the application quits from inside its own
// startup callback, which means the session ends before StartHeadless has been let go. The wait there has to notice
// that and hand back a session that has already finished rather than either blocking or calling it a failure to start.
func TestHeadlessQuitDuringStartup(t *testing.T) {
	c := check.New(t)
	prior := captureHeadlessPriorState()
	stopActiveHeadlessOnCleanup(t)
	started := make(chan *HeadlessScreen, 1)
	errors := make(chan error, 1)
	go func() {
		screen, err := StartHeadless(HeadlessConfig{Width: 100, Height: 100},
			StartupFinishedCallback(AttemptQuit))
		started <- screen
		errors <- err
	}()
	var screen *HeadlessScreen
	select {
	case screen = <-started:
		c.NoError(<-errors)
	case <-time.After(5 * time.Second):
		t.Fatal("StartHeadless never returned for a session that quit during startup")
	}
	c.NotNil(screen)
	if screen == nil {
		return
	}
	waitForHeadlessEnd(t, screen)
	c.False(screen.Running())
	// Stopping a session that has already ended must be harmless, since that is what a test's cleanup does.
	screen.Stop()
	checkHeadlessSessionReset(c, prior)
}

// TestHeadlessModalDuringStartup verifies that a StartupFinishedCallback which parks in a nested event loop — a
// first-run dialog is the realistic case — does not strand StartHeadless. The loop cannot end until the test injects
// the key that dismisses the dialog, and the test cannot inject anything until StartHeadless has handed it the screen,
// so the wait there has to be over by the time the callback reaches the loop rather than by the time it returns.
func TestHeadlessModalDuringStartup(t *testing.T) {
	c := check.New(t)
	stopActiveHeadlessOnCleanup(t)
	started := make(chan *HeadlessScreen, 1)
	errors := make(chan error, 1)
	go func() {
		screen, err := StartHeadless(HeadlessConfig{Width: 300, Height: 300},
			StartupFinishedCallback(func() {
				wnd := newHeadlessTestWindow(t, "modal host", geom.NewRect(0, 0, 200, 200))
				if wnd == nil {
					return
				}
				wnd.ToFront()
				d, dialogErr := NewDialog(nil, nil, NewMessagePanel("Proceed?", ""),
					[]*DialogButtonInfo{NewOKButtonInfo()})
				if dialogErr != nil {
					t.Errorf("unable to create dialog: %v", dialogErr)
					return
				}
				d.RunModal()
			}))
		started <- screen
		errors <- err
	}()
	var screen *HeadlessScreen
	select {
	case screen = <-started:
		c.NoError(<-errors)
	case <-time.After(5 * time.Second):
		t.Fatal("StartHeadless never returned for a startup callback that ran a nested event loop")
	}
	c.NotNil(screen)
	if screen == nil {
		return
	}

	var modals int
	c.True(screen.Do(func() { modals = len(modalStack) }))
	c.Equal(1, modals, "the dialog the startup callback put up should still be running its modal loop")

	screen.KeyPress(KeyReturn, mod.None)
	c.True(screen.Do(func() { modals = len(modalStack) }))
	c.Equal(0, modals, "the injected key should have dismissed the dialog")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessStartupOptionError verifies the promise StartHeadless makes about failures: an option that returns an
// error is reported to the caller. Start() runs xos.ExitIfErr on each option it is given, which would take the test
// binary down with it, so StartHeadless has to apply them itself.
func TestHeadlessStartupOptionError(t *testing.T) {
	c := check.New(t)
	screen, err := StartHeadless(HeadlessConfig{Width: 100, Height: 100},
		StartupFinishedCallback(func() { t.Error("the startup callback should never have run") }),
		func(_ startupOption) error { return errs.New("boom") })
	c.Nil(screen)
	c.HasError(err, "the option's error should have been reported")
	if err != nil {
		c.Contains(err.Error(), "boom")
	}
	c.Nil(ActiveHeadlessScreen(), "a session that never started should not have been left published")
	c.Nil(startupFinishedCallback, "the options that did run should have been undone")

	// The process is no worse off for the attempt, so the next session starts exactly as it would have.
	next := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100})
	c.True(next.Quit())
}

// TestHeadlessPreExistingCursors verifies that a session neither adopts nor destroys cursors that were already there
// when it started. It builds inert ones of its own while it runs and puts back exactly what it found when it ends,
// since the cursors it found hold real operating system resources that belong to whatever created them.
func TestHeadlessPreExistingCursors(t *testing.T) {
	c := check.New(t)
	// Stands in for a cursor built before any session, as an application that asked for one outside of a session would
	// have. Nothing is behind it, so there is nothing here to leak, but it is not marked headless, which is the whole
	// of what the session has to notice about it.
	// Whatever was there before is put back afterwards rather than being cleared, since these tests run shuffled and
	// what an earlier test left in place is that test's business, exactly as it is the session's.
	found := captureHeadlessPriorState()
	t.Cleanup(func() {
		cursorList = found.cursors
		builtCursorSettings = found.cursorSettings
		for i, p := range builtInCursors() {
			*p = found.builtIn[i]
		}
	})
	prior := &Cursor{}
	priorSettings := &cursorSettings{}
	cursorList = append(slices.Clone(found.cursors), prior)
	arrowCursor = prior
	builtCursorSettings = priorSettings

	var cursorsAtStartup int
	var arrowAtStartup *Cursor
	screen := startHeadlessTest(t, HeadlessConfig{Width: 100, Height: 100},
		StartupFinishedCallback(func() {
			cursorsAtStartup = len(cursorList)
			arrowAtStartup = arrowCursor
		}))
	c.Equal(0, cursorsAtStartup, "the session should have started with no cursors")
	c.Nil(arrowAtStartup, "the session should not have started holding a cursor that predates it")

	var sessionArrow *Cursor
	c.True(screen.Do(func() { sessionArrow = ArrowCursor() }))
	c.NotNil(sessionArrow)
	c.False(sessionArrow == prior, "the session should have built a cursor of its own")
	if sessionArrow != nil {
		c.True(sessionArrow.headless, "a cursor built during a session should be an inert one")
	}

	c.True(screen.Quit())
	c.Equal(len(found.cursors)+1, len(cursorList), "the cursor list from before the session should have been put back")
	if len(cursorList) == len(found.cursors)+1 {
		c.True(cursorList[len(cursorList)-1] == prior)
	}
	c.True(arrowCursor == prior, "the built-in cursor from before the session should have been put back")
	c.True(builtCursorSettings == priorSettings, "the settings from before the session should have been put back")
}

// TestHeadlessOptionReuse verifies that one Headless() option value can be handed to Start() as many times as there
// are sessions to run: the session is created when the option is applied, not when it is built, so the second Start()
// gets a session of its own rather than the finished remains of the first, whose done channel is already closed.
func TestHeadlessOptionReuse(t *testing.T) {
	c := check.New(t)
	prior := captureHeadlessPriorState()
	stopActiveHeadlessOnCleanup(t)
	option := Headless(HeadlessConfig{Width: 100, Height: 100})
	var sessions []*HeadlessScreen
	for i := range 2 {
		started := make(chan *HeadlessScreen, 1)
		returned := make(chan struct{})
		go func() {
			defer close(returned)
			Start(option, StartupFinishedCallback(func() { started <- ActiveHeadlessScreen() }))
		}()
		var screen *HeadlessScreen
		select {
		case screen = <-started:
		case <-time.After(5 * time.Second):
			t.Fatalf("session %d never reached its startup callback", i+1)
		}
		c.NotNil(screen)
		if screen == nil {
			return
		}
		c.True(screen.Running(), "session %d should be running", i+1)
		c.True(screen.Quit(), "session %d should have quit", i+1)
		select {
		case <-returned:
		case <-time.After(5 * time.Second):
			t.Fatalf("Start() never returned after session %d ended", i+1)
		}
		checkHeadlessSessionReset(c, prior)
		sessions = append(sessions, screen)
	}
	c.True(sessions[0] != sessions[1], "each application of the option should have created a session of its own")
}

// TestHeadlessDisposeDuringPressClearsButtons verifies that a window disposed of from inside its own mouse-down
// handling takes the buttons it was holding with it, as a hidden one does. The release that follows has no window to go
// to, so nothing else could ever clear them, and every external drag for the rest of the session would otherwise be
// refused on the strength of a press that no longer exists.
func TestHeadlessDisposeDuringPressClearsButtons(t *testing.T) {
	c := check.New(t)
	var doomed, other *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			// The second window is there so that disposing of the first does not close the last window and end the
			// session.
			other = newHeadlessTestWindow(t, "other", geom.NewRect(150, 0, 100, 100))
			doomed = newHeadlessTestWindow(t, "doomed", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(doomed)
	c.NotNil(other)
	c.True(screen.Do(func() {
		doomed.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			doomed.Dispose()
			return true
		}
	}))

	screen.MouseDown(geom.NewPoint(50, 50), ButtonLeft, mod.None)
	var windows []*Window
	var buttonsDown int
	c.True(screen.Do(func() {
		windows = slices.Clone(windowList)
		buttonsDown = len(screen.buttons)
	}))
	c.Equal([]*Window{other}, windows, "the window should have disposed of itself from inside the press")
	c.Equal(0, buttonsDown, "the press should have been forgotten along with the window it was in")

	screen.MouseUp(geom.NewPoint(50, 50), ButtonLeft, mod.None)
	result := screen.DropExternal(geom.NewPoint(200, 50), geom.NewPoint(210, 60), 2, drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
	c.False(result.Canceled, "an external drag should be accepted once the press has gone with its window")
	c.Equal(0, len(screen.Errors()), "nothing should have been refused: %v", screen.Errors())
}

// TestHeadlessDisposeTransientRestoresFocus verifies that disposing of a transient window which had taken the focus
// with ToFront() hands the focus back to the window that can hold it, as the platforms do when the key window closes.
// Window.Dispose() refocuses the front of the window list only when the window was the ActiveWindow(), which a
// transient one never is, so the session has to do this itself or nothing is focused and every key event is dropped
// until the next click.
func TestHeadlessDisposeTransientRestoresFocus(t *testing.T) {
	c := check.New(t)
	var main, popup *Window
	var keys int
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 200},
		StartupFinishedCallback(func() {
			main = newHeadlessTestWindow(t, "main", geom.NewRect(0, 0, 200, 150))
			if main == nil {
				return
			}
			main.KeyDownCallback = func(_ KeyCode, _ mod.Modifiers, _ bool) bool {
				keys++
				return true
			}
			main.ToFront()
			var err error
			if popup, err = NewWindow("popup", TransientWindowOption()); err != nil {
				t.Errorf("unable to create transient window: %v", err)
				return
			}
			popup.SetContentRect(geom.NewRect(20, 20, 100, 60))
			popup.ToFront()
		}))
	c.NotNil(main)
	c.NotNil(popup)

	var active *Window
	c.True(screen.Do(func() { active = ActiveWindow() }))
	c.True(screen.FocusedWindow() == popup, "the transient window should have taken the focus")
	c.True(active == main, "a transient window is never the active one")

	c.True(screen.Do(func() { popup.Dispose() }))
	c.True(screen.Do(func() { active = ActiveWindow() }))
	c.True(screen.FocusedWindow() == main, "the focus should have gone back to the window that can hold it")
	c.True(active == main)
	screen.KeyPress(KeyA, mod.None)
	c.Equal(1, keys, "key events should reach the window the focus went back to")
}

// TestHeadlessDriverCallFromCallback is the regression test for a driver call made from the UI thread, which is what
// happens when a widget callback reaches for the screen. Reading something from the screen in the middle of a gesture
// must not deliver the rest of that gesture inside the callback — a mouse up arriving while the mouse down is still on
// the stack is something no platform ever does — and input injected from there must be queued behind the event being
// handled rather than dispatched on the spot, which is when a platform would deliver it.
func TestHeadlessDriverCallFromCallback(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var panel *Panel
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "reentrancy", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			panel = NewPanel()
			panel.SetFocusable(true)
			wnd.Content().AddChild(panel)
			wnd.ToFront()
			wnd.SetFocus(panel)
		}))
	c.NotNil(wnd)
	c.NotNil(panel)

	var order []string
	var focusedDuringPress, captureDuringPress *Window
	var buttonsDuringPress int
	c.True(screen.Do(func() {
		wnd.Content().MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			order = append(order, "down-begin")
			// Every one of these goes through run(), and none of them may dispatch the mouse up that is queued behind
			// this press.
			focusedDuringPress = screen.FocusedWindow()
			screen.WindowAt(geom.NewPoint(50, 50))
			screen.Sync()
			screen.Do(func() {
				captureDuringPress = screen.capture
				buttonsDuringPress = len(screen.buttons)
			})
			// Input injected from here is queued rather than delivered, so the key arrives after the mouse up.
			screen.KeyPress(KeyA, mod.None)
			// A panic in work run through the driver from here is recorded for Errors(), as it would be for the
			// same call made from the test goroutine, rather than escaping into this callback.
			screen.Do(func() { panic("from a callback") })
			order = append(order, "down-end")
			return true
		}
		wnd.Content().MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
			order = append(order, "up")
			return true
		}
		panel.KeyDownCallback = func(code KeyCode, _ mod.Modifiers, _ bool) bool {
			order = append(order, "key "+code.Key())
			return true
		}
	}))

	screen.Click(geom.NewPoint(50, 50))
	c.Equal([]string{"down-begin", "down-end", "up", "key A"}, order)
	c.True(focusedDuringPress == wnd)
	c.True(captureDuringPress == wnd, "the press should still have held the pointer while its handler ran")
	c.Equal(1, buttonsDuringPress, "the button should still have been down while its handler ran")
	errors := screen.Errors()
	c.Equal(1, len(errors), "the panic in the driver call should have been recorded: %v", errors)
	if len(errors) == 1 {
		c.Contains(errors[0].Error(), "from a callback")
	}
}

// TestHeadlessStopPastDialogsOnTheWayOut verifies that Stop() ends a session whose application puts up a dialog as it
// is being torn down: a WillCloseCallback or QuittingCallback that runs a modal loop would otherwise park the UI thread
// waiting for input that only the goroutine blocked in Stop() could inject, and a test's cleanup would hang.
func TestHeadlessStopPastDialogsOnTheWayOut(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	dialogs := 0
	askOnTheWayOut := func() {
		dialogs++
		d, err := NewDialog(nil, nil, NewMessagePanel("Save changes?", ""), []*DialogButtonInfo{NewOKButtonInfo()})
		if err != nil {
			t.Errorf("unable to create dialog: %v", err)
			return
		}
		d.RunModal()
	}
	screen := startHeadlessTest(t, HeadlessConfig{Width: 300, Height: 300},
		QuittingCallback(askOnTheWayOut),
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "asks on close", geom.NewRect(0, 0, 100, 100))
			if wnd != nil {
				wnd.WillCloseCallback = askOnTheWayOut
			}
		}))
	c.NotNil(wnd)

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		screen.Stop()
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() never returned while the application was putting up dialogs on the way out")
	}
	c.False(screen.Running())
	c.Equal(0, dialogs, "Stop() should have taken away the callbacks that put up the dialogs")
}

// TestHeadlessTransparentBackground verifies that a capture can be asked to paint nothing behind the windows, which is
// what a test reading back the alpha of a transparent window wants, and that the zero-value configuration still gets
// the opaque default.
func TestHeadlessTransparentBackground(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t,
		HeadlessConfig{Width: 100, Height: 100, TransparentBackground: true, Background: RGB(0, 0, 255)},
		StartupFinishedCallback(func() {
			w, err := NewWindow("see-through", TransparentWindowOption())
			if err != nil {
				t.Errorf("unable to create window: %v", err)
				return
			}
			// Only the left half of the window is painted, so the right half is where the window's own transparency
			// shows through.
			w.Content().DrawCallback = func(gc *Canvas, rect geom.Rect) {
				rect.Width /= 2
				gc.DrawRect(rect, Red.Paint(gc, rect, paintstyle.Fill))
			}
			w.SetContentRect(geom.NewRect(20, 20, 60, 60))
			w.Show()
			wnd = w
		}))
	c.NotNil(wnd)
	screen.Sync() // the window must have been drawn before there is anything to capture

	img := screen.Capture()
	c.NotNil(img)
	c.Equal(color.NRGBA{}, img.NRGBAAt(5, 5), "nothing should have been painted behind the windows")
	painted := img.NRGBAAt(30, 50)
	c.True(painted.A == 255 && painted.R > 200 && painted.G < 60 && painted.B < 60,
		"the painted half of the window should be opaque red, but was %v", painted)
	c.Equal(uint8(0), img.NRGBAAt(70, 50).A, "the unpainted half of a transparent window should have no alpha")
	c.True(screen.Quit())

	cfg, err := HeadlessConfig{Width: 1, Height: 1}.normalized()
	c.NoError(err)
	c.Equal(RGB(headlessBackgroundLevel, headlessBackgroundLevel, headlessBackgroundLevel), cfg.Background,
		"the zero value should still ask for the opaque default")
	cfg, err = HeadlessConfig{Width: 1, Height: 1, TransparentBackground: true, Background: RGB(0, 0, 255)}.normalized()
	c.NoError(err)
	c.Equal(Transparent, cfg.Background, "the flag should win over a Background that was also set")
}

// TestHeadlessWindowCreatedDuringQuitIsInvalidated verifies that a window torn down by finishQuit rather than by
// Dispose() — which is the fate of a window created while the application is quitting — is left invalid, since the
// safety of every timer that outlives a session rests on the window it belongs to reporting IsValid() false.
func TestHeadlessWindowCreatedDuringQuitIsInvalidated(t *testing.T) {
	c := check.New(t)
	var early, late *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		QuittingCallback(func() {
			late = newHeadlessTestWindow(t, "late", geom.NewRect(0, 0, 50, 50))
		}),
		StartupFinishedCallback(func() {
			early = newHeadlessTestWindow(t, "early", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(early)
	c.True(screen.Quit())
	c.NotNil(late, "the quitting callback should have created a window")
	c.False(early.IsValid())
	c.False(late.IsValid(), "a window destroyed by finishQuit should be invalid")
}

// TestHeadlessExternalDragSessionEndsOnEntry verifies that an external drag whose arrival ends the session — a drop
// target that quits the application from its entry callback — leaves the handle usable: asking it anything is not a
// data race, and nothing it is asked to do hangs or panics now that there is no session to do it in.
func TestHeadlessExternalDragSessionEndsOnEntry(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "quits on entry", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			wnd.RegisterForDragTypes(uti.UTF8PlainText)
			content := wnd.Content()
			content.DragEnteredCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op {
				AttemptQuit()
				return drag.Copy
			}
			// A panel is only a drop target if it has a DropCallback, so the entry callback is never reached without
			// one.
			content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
		}))
	c.NotNil(wnd)

	finished := make(chan struct{})
	var result HeadlessDragResult
	go func() {
		defer close(finished)
		d := screen.BeginExternalDrag(geom.NewPoint(50, 50), drag.Copy,
			drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
		d.MoveTo(geom.NewPoint(60, 60))
		result = d.Drop()
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the external drag handle blocked after the session ended underneath it")
	}
	waitForHeadlessEnd(t, screen)
	c.False(screen.Running())
	// A drag the session ended underneath never completed, and says so: it is reported as canceled rather than as the
	// zero value, whose Canceled is false. The target is nil, since the window the drag had entered was destroyed by
	// the quit before the drag was ended, and a destroyed window is no longer one the drag can be said to be over.
	c.True(result.Canceled, "a drag cut short by the session ending should report itself as canceled")
	c.False(result.Dropped)
	c.Nil(result.Target)
	c.Equal(1, len(result.Data), "the result should still carry what the drag was carrying")
	last := screen.LastDrag()
	c.True(last.Canceled, "LastDrag should go on reporting the final drag after the session has ended")
	c.Equal(1, len(last.Data))
}

// TestHeadlessDropQuitsApplication is the other way a session can end around a drag: the drop itself quits the
// application. The drag completed, so the result says that it was dropped rather than canceled.
func TestHeadlessDropQuitsApplication(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "quits on drop", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			wnd.RegisterForDragTypes(uti.UTF8PlainText)
			content := wnd.Content()
			content.DragEnteredCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op { return drag.Copy }
			content.DragUpdatedCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op { return drag.Copy }
			content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool {
				AttemptQuit()
				return true
			}
		}))
	c.NotNil(wnd)

	result := screen.DropExternal(geom.NewPoint(150, 150), geom.NewPoint(50, 50), 2, drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
	waitForHeadlessEnd(t, screen)
	c.True(result.Dropped, "the drop happened, however soon afterwards the session ended")
	c.True(result.Handled)
	c.False(result.Canceled)
	c.True(screen.LastDrag().Dropped)
}

// TestHeadlessExternalDragExitsHoveredWindow verifies that a drag arriving from outside takes the pointer away from
// the window it was in, as a window server's leave event does when a drag grabs the pointer, and that the window gets
// it back — with an entry — once the drag is over, even when the drag ends over that very window.
func TestHeadlessExternalDragExitsHoveredWindow(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "hovered", geom.NewRect(0, 0, 100, 100))
			if wnd != nil {
				wnd.RegisterForDragTypes(uti.UTF8PlainText)
			}
		}))
	c.NotNil(wnd)

	var enters, exits int
	c.True(screen.Do(func() {
		wnd.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			enters++
			return false
		}
		wnd.MouseExitCallback = func() bool {
			exits++
			return false
		}
	}))
	screen.MouseMove(geom.NewPoint(50, 50), mod.None)
	c.True(screen.Do(func() { enters, exits = 0, 0 }))

	d := screen.BeginExternalDrag(geom.NewPoint(50, 50), drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("payload")})
	var hover *Window
	c.True(screen.Do(func() { hover = screen.hover }))
	c.Equal(1, exits, "the drag taking the pointer should have exited the window it was in")
	c.Equal(0, enters)
	c.Nil(hover, "no window should be recorded as hovered while the drag holds the pointer")

	d.Cancel()
	c.True(screen.Do(func() { hover = screen.hover }))
	c.Equal(1, enters, "the drag handing the pointer back should have entered the window it is over")
	c.True(hover == wnd)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessStartDragDuringDragRefused verifies that a source asking to start a drag while one is already in flight
// is refused: the refusal is recorded, the source's cleanup still runs, and the drag that was in flight is left alone.
func TestHeadlessStartDragDuringDragRefused(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	cleanups := 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "source", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			wnd.RegisterForDragTypes(uti.UTF8PlainText)
			content := wnd.Content()
			content.DragEnteredCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op { return drag.Copy }
			content.DragUpdatedCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op { return drag.Copy }
			content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
		}))
	c.NotNil(wnd)

	d := screen.BeginExternalDrag(geom.NewPoint(50, 50), drag.Copy,
		drag.Data{Type: uti.UTF8PlainText, Data: []byte("outside")})
	c.True(screen.Do(func() {
		wnd.StartDrag(nil, geom.NewPoint(10, 10), func() { cleanups++ }, drag.Copy,
			drag.Data{Type: uti.UTF8PlainText, Data: []byte("inside")})
	}))
	errors := screen.Errors()
	c.Equal(1, len(errors), "the refusal should have been recorded: %v", errors)
	if len(errors) == 1 {
		c.Contains(errors[0].Error(), "already in progress")
	}
	var cleaned int
	c.True(screen.Do(func() { cleaned = cleanups }))
	c.Equal(1, cleaned, "the refused source's cleanup should still have run")

	result := d.Drop()
	c.True(result.Dropped, "the drag that was in flight should have carried on")
	c.Nil(result.Source, "the drag that was in flight came from outside the application")
	c.Equal(1, len(result.Data))
	if len(result.Data) == 1 {
		c.Equal("outside", string(result.Data[0].Data))
	}
}

// TestHeadlessDragOverConformingType verifies that the two halves of drop-target resolution agree: a window registered
// for UTF-8 text is entered by a drag carrying plain text, since the types conform, and the panel-level checks then
// find the text rather than coming up empty and leaving the drag with no taker.
func TestHeadlessDragOverConformingType(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	var text string
	entered := 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "conforming", geom.NewRect(0, 0, 100, 100))
			if wnd == nil {
				return
			}
			wnd.RegisterForDragTypes(uti.UTF8PlainText)
			content := wnd.Content()
			content.CanAcceptDropCallback = func(di drag.Info) bool { return di.HasString() }
			content.DragEnteredCallback = func(di drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op {
				entered++
				text = di.Text()
				return drag.Copy
			}
			content.DragUpdatedCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) drag.Op { return drag.Copy }
			content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
		}))
	c.NotNil(wnd)

	result := screen.DropExternal(geom.NewPoint(150, 150), geom.NewPoint(50, 50), 2, drag.Copy,
		drag.Data{Type: uti.PlainText, Data: []byte("plain")})
	var seen string
	var count int
	c.True(screen.Do(func() {
		seen = text
		count = entered
	}))
	c.Equal(1, count, "the window should have been entered on the strength of the conforming type")
	c.Equal("plain", seen, "the panel should have found the text under the type it asked for")
	c.True(result.Dropped)
	c.True(result.Handled)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessCaptureWindowRefusals covers the two windows CaptureWindow declines to capture: one that has never been
// drawn, and one that belongs to another session — which keeps its headless backing, and the last frame in it, for
// life, so the session has to check rather than merely finding a frame.
func TestHeadlessCaptureWindowRefusals(t *testing.T) {
	c := check.New(t)
	var drawn, hidden *Window
	first := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			drawn = newHeadlessTestWindow(t, "drawn", geom.NewRect(0, 0, 100, 100))
			var err error
			if hidden, err = NewWindow("never shown"); err != nil {
				t.Errorf("unable to create window: %v", err)
			}
		}))
	c.NotNil(drawn)
	c.NotNil(hidden)
	first.Sync()
	c.NotNil(first.CaptureWindow(drawn))
	c.Nil(first.CaptureWindow(hidden), "a window that was never drawn has no frame to capture")
	c.Nil(first.CaptureWindow(nil))
	c.Nil(first.CaptureWindow(&Window{}), "a window with no backing at all belongs to no session")
	first.Stop()

	second := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200})
	c.Nil(second.CaptureWindow(drawn), "a window from an earlier session does not belong to this one")
	c.NotNil(headlessWindowFor(drawn).frame, "the frame is still there; it is the session check that refuses it")
}

// TestHeadlessNegativePositionCapture verifies that a window placed partly off the top-left of the screen is composited
// where its own pixels say it is: a negative fractional position is floored to a device pixel rather than truncated
// toward zero, since truncation would move the window a pixel toward the origin while the surface behind it was sized
// by flooring, leaving the window's right and bottom edges a pixel short of where it claims to end.
func TestHeadlessNegativePositionCapture(t *testing.T) {
	c := check.New(t)
	background := RGB(0, 0, 255)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200, Scale: 1.5, Background: background},
		StartupFinishedCallback(func() {
			w, err := NewWindow("off the edge")
			if err != nil {
				t.Errorf("unable to create window: %v", err)
				return
			}
			w.Content().DrawCallback = func(gc *Canvas, rect geom.Rect) {
				gc.DrawRect(rect, Red.Paint(gc, rect, paintstyle.Fill))
			}
			// SetFrameRect, unlike EnsureOnDisplay, does not pull a window back onto the screen.
			w.SetFrameRect(geom.NewRect(-1, -1, 101, 51))
			w.Show()
			wnd = w
		}))
	c.NotNil(wnd)
	screen.Sync()

	img := screen.Capture()
	c.NotNil(img)
	// The window starts at floor(-1*1.5) = -2 device pixels and is int(101*1.5) = 151 by int(51*1.5) = 76 pixels, so
	// its last column is at 148 and its last row at 73. Truncation would have put them at 149 and 74.
	const (
		right  = -2 + 151 - 1
		bottom = -2 + 76 - 1
	)
	for _, one := range []struct {
		name    string
		inside  image.Point
		outside image.Point
	}{
		{name: "right", inside: image.Pt(right, 30), outside: image.Pt(right+1, 30)},
		{name: "bottom", inside: image.Pt(60, bottom), outside: image.Pt(60, bottom+1)},
	} {
		in := img.NRGBAAt(one.inside.X, one.inside.Y)
		c.True(in.R > 200 && in.G < 60 && in.B < 60 && in.A == 255,
			"the pixel just inside the %s edge should be the window's red, but was %v", one.name, in)
		c.Equal(color.NRGBA{R: 0, G: 0, B: 255, A: 255}, img.NRGBAAt(one.outside.X, one.outside.Y),
			"the pixel just outside the %s edge should be the background", one.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessDisposeHandsPointerOn verifies what disposing of a window leaves behind for the input router: the window
// underneath is entered at once rather than at the next pointer event, and the disposed window no longer reports
// itself focused, since the resignation the platforms deliver has to be performed by the session here.
func TestHeadlessDisposeHandsPointerOn(t *testing.T) {
	c := check.New(t)
	var back, front *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			back = newHeadlessTestWindow(t, "back", geom.NewRect(0, 0, 200, 200))
			front = newHeadlessTestWindow(t, "front", geom.NewRect(0, 0, 100, 100))
			if front != nil {
				front.ToFront()
			}
		}))
	c.NotNil(back)
	c.NotNil(front)

	var backEnters int
	c.True(screen.Do(func() {
		back.MouseEnterCallback = func(_ geom.Point, _ mod.Modifiers) bool {
			backEnters++
			return false
		}
	}))
	screen.MouseMove(geom.NewPoint(50, 50), mod.None)
	c.True(screen.Do(func() { backEnters = 0 }))

	var frontFocusedBefore, frontFocusedAfter bool
	var hover, focused *Window
	c.True(screen.Do(func() {
		frontFocusedBefore = front.Focused()
		front.Dispose()
		frontFocusedAfter = front.Focused()
		hover = screen.hover
		focused = screen.focused
	}))
	c.True(frontFocusedBefore)
	c.False(frontFocusedAfter, "a disposed window should not go on reporting itself focused")
	c.True(hover == back, "the window revealed underneath should be entered as soon as the other is gone")
	// Entered by the crossing, and again by the focus handoff, since Window.gainedFocus enters the window as well.
	c.True(backEnters > 0)
	c.True(focused == back, "the focus should have been handed to the window underneath")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHeadlessTitleIcons verifies that a headless window records its title icons wherever the test is running: macOS
// has no window icons, and SetTitleIcons does nothing there for a native window, but a session behaves the same on
// every host.
func TestHeadlessTitleIcons(t *testing.T) {
	c := check.New(t)
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 200, Height: 200},
		StartupFinishedCallback(func() {
			wnd = newHeadlessTestWindow(t, "icons", geom.NewRect(0, 0, 100, 100))
		}))
	c.NotNil(wnd)

	var recorded, reported int
	c.True(screen.Do(func() {
		img, err := NewImageFromPixels(2, 2, make([]byte, 16), geom.NewPoint(1, 1))
		if err != nil {
			t.Errorf("unable to create image: %v", err)
			return
		}
		wnd.SetTitleIcons([]*Image{img})
		recorded = len(headlessWindowFor(wnd).icons)
		reported = len(wnd.TitleIcons())
	}))
	c.Equal(1, recorded, "the session's window should have been handed the icon")
	c.Equal(1, reported, "the window should report the icon it was given")
}

// TestCurrentKeyModifiersWithoutPlatformWindow verifies that CurrentKeyModifiers, which is not guarded by IsValid(),
// still answers for a window that was never initialized — the hand-built kind some tests use — rather than reaching
// for platform state the window does not have.
func TestCurrentKeyModifiersWithoutPlatformWindow(t *testing.T) {
	c := check.New(t)
	w := &Window{}
	w.root = newRootPanel(w)
	c.NotPanics(func() { w.CurrentKeyModifiers() })
}

// TestDisplayUsableDispatchesOnItsOwnBackend verifies that a display converts its usable area the way its own backend
// requires rather than the way the currently active session would: the session's display needs no conversion, while
// one the OS built before the session still goes through the native conversion during it.
func TestDisplayUsableDispatchesOnItsOwnBackend(t *testing.T) {
	c := check.New(t)
	screen := startHeadlessTest(t, HeadlessConfig{Width: 320, Height: 240, Scale: 2})
	native := &Display{
		Frame:  geom.NewRect(0, 0, 3840, 2160),
		Usable: geom.NewRect(0, 40, 3840, 2120),
		Scale:  geom.NewPoint(2, 2),
	}
	var sessionUsable, nativeUsable, nativeExpected geom.Rect
	c.True(screen.Do(func() {
		sessionUsable = PrimaryDisplay().apiUsableInWindowUnits()
		nativeUsable = native.apiUsableInWindowUnits()
		nativeExpected = native.nativeUsableInWindowUnits()
	}))
	c.Equal(geom.NewRect(0, 0, 320, 240), sessionUsable, "the session's display is already in window units")
	c.Equal(nativeExpected, nativeUsable, "a display built by the OS keeps the OS conversion during a session")
}
