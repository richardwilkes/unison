// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/enums/thememode"
	"github.com/richardwilkes/unison/internal/skia"
)

var (
	redrawSet                         = make(map[*Window]struct{})
	startupFinishedCallback           func()
	openFilesCallback                 func([]string) //nolint:unused // Not all platforms use this
	themeChangedCallback              func()
	recoveryCallback                  func(error)
	quitAfterLastWindowClosedCallback func() bool
	allowQuitCallback                 func() bool
	quittingCallback                  func()
	glfwInited                        atomic.Bool
	noGlobalMenuBar                   bool
	quitLock                          sync.RWMutex
	calledAtExit                      bool
	currentThemeMode                  = thememode.Auto
	needPlatformDarkModeUpdate        = true
	platformDarkModeEnabled           bool
)

type startupOption struct { // This exists just to prevent arbitrary functions from being passed to application startup.
}

// StartupOption holds an option for application startup.
type StartupOption func(startupOption) error

func init() {
	// All init functions are run on the startup thread. Calling LockOSThread from an init function will cause the main
	// function to be invoked on that thread.
	runtime.LockOSThread()
}

// StartupFinishedCallback will cause f to be called once application startup has completed and it is about to start
// servicing the event loop. You should create your app's windows at this point.
func StartupFinishedCallback(f func()) StartupOption {
	return func(_ startupOption) error {
		startupFinishedCallback = f
		return nil
	}
}

// OpenFilesCallback will cause f to be called when the application is asked to open one or more files by the OS or an
// external application. By default, nothing is done with the request.
func OpenFilesCallback(f func(urls []string)) StartupOption {
	return func(_ startupOption) error {
		openFilesCallback = f
		return nil
	}
}

// ThemeChangedCallback will cause f to be called when the theme is changed. This occurs after the colors have been
// updated, but before any windows have been redraw.
func ThemeChangedCallback(f func()) StartupOption {
	return func(_ startupOption) error {
		themeChangedCallback = f
		return nil
	}
}

// RecoveryCallback will cause f to be called should a task invoked via task.InvokeTask() or task.InvokeTaskAfter()
// panic. If no recovery callback is set, the panic will be logged via errs.Log(err).
func RecoveryCallback(f func(error)) StartupOption {
	return func(_ startupOption) error {
		recoveryCallback = f
		return nil
	}
}

// QuitAfterLastWindowClosedCallback will cause f to be called when the last window is closed to determine if the
// application should quit as a result. By default, the app will terminate when the last window is closed.
func QuitAfterLastWindowClosedCallback(f func() bool) StartupOption {
	return func(_ startupOption) error {
		quitAfterLastWindowClosedCallback = f
		return nil
	}
}

// AllowQuitCallback will cause f to be called when app termination has been requested. Return true to permit the
// request. By default, app termination requests are permitted.
func AllowQuitCallback(f func() bool) StartupOption {
	return func(_ startupOption) error {
		allowQuitCallback = f
		return nil
	}
}

// QuittingCallback will cause f to be called just before the app terminates.
func QuittingCallback(f func()) StartupOption {
	return func(_ startupOption) error {
		quittingCallback = f
		return nil
	}
}

// NoGlobalMenuBar will disable the global menu bar on platforms that normally use it, instead using the in-window menu
// bar.
func NoGlobalMenuBar() StartupOption {
	return func(_ startupOption) error {
		noGlobalMenuBar = true
		return nil
	}
}

// Start the application. This function does NOT return. While some calls may be safe to make, it should be assumed no
// calls into unison can be made prior to Start() being called unless explicitly stated otherwise.
func Start(options ...StartupOption) {
	for _, option := range options {
		xos.ExitIfErr(option(startupOption{}))
	}
	glfw.InitHint(glfw.CocoaMenubar, glfw.False)
	glfw.InitHint(glfw.CocoaChdirResources, glfw.False)
	xos.ExitIfErr(glfw.Init())
	xos.RunAtExit(quitting)
	xos.RunAtExit(func() {
		quitLock.Lock()
		calledAtExit = true
		quitLock.Unlock()
	})
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 2)
	platformEarlyInit()
	glfwInited.Store(true)
	InvokeTask(finishStartup)
	for {
		processEvents()
	}
}

func processEvents() {
	glfw.WaitEvents()
	processNextTask(uiTaskRecovery)
	if len(redrawSet) > 0 {
		set := redrawSet
		redrawSet = make(map[*Window]struct{})
		for wnd := range set {
			if wnd.IsVisible() {
				wnd.draw()
			} else {
				redrawSet[wnd] = struct{}{}
			}
		}
	}
}

func finishStartup() {
	skiaColorspace = skia.ColorSpaceNewSRGB()
	RebuildDynamicColors()
	platformLateInit()
	if startupFinishedCallback != nil {
		xos.SafeCall(startupFinishedCallback, nil)
	}
	platformFinishedStartup()
}

// ThemeChanged marks dynamic colors for rebuilding, calls any installed theme change callback, and then redraws all
// windows. This is normally called automatically for you, however, it has been made public to allow you to trigger it
// on demand.
func ThemeChanged() {
	MarkDynamicColorsForRebuild()
	if themeChangedCallback != nil {
		xos.SafeCall(themeChangedCallback, nil)
	}
	for _, wnd := range Windows() {
		wnd.MarkForRedraw()
	}
}

func uiTaskRecovery(err error) {
	if recoveryCallback != nil {
		xos.SafeCall(func() { recoveryCallback(err) }, nil)
	} else {
		errs.Log(err)
	}
}

func quitAfterLastWindowClosed() bool {
	if quitAfterLastWindowClosedCallback != nil {
		quit := true
		xos.SafeCall(func() { quit = quitAfterLastWindowClosedCallback() }, nil)
		return quit
	}
	return true
}

func allowQuit() bool {
	if allowQuitCallback != nil {
		allow := true
		xos.SafeCall(func() { allow = allowQuitCallback() }, nil)
		return allow
	}
	return true
}

func quitting() {
	quitLock.Lock()
	callback := quittingCallback
	quittingCallback = nil
	quitLock.Unlock()
	if callback != nil {
		xos.SafeCall(callback, nil)
	}
	// xos.Exit() is called here once to ensure registered exit hooks are actually called, as OS's may directly
	// terminate the app after returning from this function.
	quitLock.Lock()
	calledExit := calledAtExit
	calledAtExit = true
	quitLock.Unlock()
	if !calledExit {
		xos.Exit(0)
	}
	glfw.Terminate()
}

// AttemptQuit initiates the termination sequence.
func AttemptQuit() {
	if allowQuit() {
		quitting()
	}
}

// Beep plays the system beep sound.
func Beep() {
	platformBeep()
}

// IsColorModeTrackingPossible returns true if the underlying platform can provide the current dark mode state. On those
// platforms that return false from this function, thememode.Auto is the same as thememode.Light.
func IsColorModeTrackingPossible() bool {
	return platformIsDarkModeTrackingPossible()
}

// CurrentThemeMode returns the current theme mode state.
func CurrentThemeMode() thememode.Enum {
	return currentThemeMode
}

// SetThemeMode sets the current theme mode state.
func SetThemeMode(mode thememode.Enum) {
	if currentThemeMode != mode {
		currentThemeMode = mode
		needPlatformDarkModeUpdate = true
		InvokeTask(ThemeChanged)
	}
}

// IsDarkModeEnabled returns true if the OS is currently using a "dark mode".
func IsDarkModeEnabled() bool {
	switch currentThemeMode {
	case thememode.Light:
		return false
	case thememode.Dark:
		return true
	default:
		if needPlatformDarkModeUpdate {
			needPlatformDarkModeUpdate = false
			platformDarkModeEnabled = platformIsDarkModeEnabled()
		}
		return platformDarkModeEnabled
	}
}

// DoubleClickParameters returns the maximum delay between clicks and the maximum pixel drift allowed to register as a
// double-click.
func DoubleClickParameters() (maxDelay time.Duration, maxMouseDrift float32) {
	return platformDoubleClickInterval(), 5
}

// DragGestureParameters returns the minimum delay before mouse movement should be recognized as a drag as well as the
// minimum pixel drift required to trigger a drag.
func DragGestureParameters() (minDelay time.Duration, minMouseDrift float32) {
	return 250 * time.Millisecond, 5
}

func postEmptyEvent() {
	if glfwInited.Load() {
		glfw.PostEmptyEvent()
	}
}
