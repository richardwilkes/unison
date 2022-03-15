// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/richardwilkes/toolbox"
	"github.com/richardwilkes/toolbox/atexit"
	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/unison/internal/skia"
)

// DarkModeTracking holds the current dark mode control setting.
type DarkModeTracking uint8

// Possible values for DarkModeTracking.
const (
	DarkModeForcedOff DarkModeTracking = iota
	DarkModeForcedOn
	DarkModeTrackPlatform
)

var (
	redrawSet                         = make(map[*Window]struct{})
	startupFinishedCallback           func()
	openFilesCallback                 func([]string) //nolint:unused // Not all platforms use this
	themeChangedCallback              func()
	recoveryCallback                  errs.RecoveryHandler
	quitAfterLastWindowClosedCallback func() bool
	allowQuitCallback                 func() bool
	quittingCallback                  func()
	noGlobalMenuBar                   bool
	quitLock                          sync.RWMutex
	calledAtExit                      bool
	currentDarkModeTracking           = DarkModeTrackPlatform
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
// panic. If no recovery callback is set, the panic will be logged via jot.Error(err).
func RecoveryCallback(f errs.RecoveryHandler) StartupOption {
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
	pwd, err := filepath.Abs(".")
	if err != nil {
		jot.Error(err)
	}
	for _, option := range options {
		jot.FatalIfErr(option(startupOption{}))
	}
	glfw.InitHint(glfw.CocoaMenubar, glfw.False)
	jot.FatalIfErr(glfw.Init())
	// Restore the original working directory, as glfw changes it on some platforms
	if err = os.Chdir(pwd); err != nil {
		jot.Error(err)
	}
	atexit.Register(quitting)
	atexit.Register(func() {
		quitLock.Lock()
		calledAtExit = true
		quitLock.Unlock()
	})
	glfw.WindowHint(glfw.ContextVersionMajor, 3)
	glfw.WindowHint(glfw.ContextVersionMinor, 2)
	if runtime.GOOS != toolbox.LinuxOS {
		// Some Linux platforms I've tried fail if either of these two options are enabled.
		// macOS seems to require both be set.
		// Windows doesn't seem to care either way.
		// So... don't set them on Linux...
		glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
		glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	}
	platformEarlyInit()
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
		toolbox.Call(startupFinishedCallback)
	}
}

// ThemeChanged marks dynamic colors for rebuilding, calls any installed theme change callback, and then redraws all
// windows. This is normally called automatically for you, however, it has been made public to allow you to trigger it
// on demand.
func ThemeChanged() {
	MarkDynamicColorsForRebuild()
	if themeChangedCallback != nil {
		toolbox.Call(themeChangedCallback)
	}
	for _, wnd := range Windows() {
		wnd.MarkForRedraw()
	}
}

func uiTaskRecovery(err error) {
	if recoveryCallback != nil {
		toolbox.Call(func() { recoveryCallback(err) })
	} else {
		jot.Error(err)
	}
}

func quitAfterLastWindowClosed() bool {
	if quitAfterLastWindowClosedCallback != nil {
		quit := true
		toolbox.Call(func() { quit = quitAfterLastWindowClosedCallback() })
		return quit
	}
	return true
}

func allowQuit() bool {
	if allowQuitCallback != nil {
		allow := true
		toolbox.Call(func() { allow = allowQuitCallback() })
		return allow
	}
	return true
}

func quitting() {
	quitLock.Lock()
	callback := quittingCallback //nolint:ifshort // The callback must be made outside of holding the lock
	quittingCallback = nil
	quitLock.Unlock()
	if callback != nil {
		toolbox.Call(callback)
	}
	// atexit.Exit() is called here once to ensure registered atexit hooks are actually called, as OS's may directly
	// terminate the app after returning from this function.
	quitLock.Lock()
	calledExit := calledAtExit //nolint:ifshort // No, the short syntax is not appropriate here
	calledAtExit = true
	quitLock.Unlock()
	if !calledExit {
		atexit.Exit(0)
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

// IsDarkModeTrackingPossible returns true if the underlying platform can provide the current dark mode state. On those
// platforms that return false from this function, DarkModeTrackPlatform is the same as DarkModeForcedOff.
func IsDarkModeTrackingPossible() bool {
	return platformIsDarkModeTrackingPossible()
}

// CurrentDarkModeTracking returns the current DarkModeTracking state.
func CurrentDarkModeTracking() DarkModeTracking {
	return currentDarkModeTracking
}

// SetDarkModeTracking sets the way dark mode is tracked.
func SetDarkModeTracking(mode DarkModeTracking) {
	if currentDarkModeTracking != mode {
		currentDarkModeTracking = mode
		needPlatformDarkModeUpdate = true
		InvokeTask(ThemeChanged)
	}
}

// IsDarkModeEnabled returns true if the OS is currently using a "dark mode".
func IsDarkModeEnabled() bool {
	switch currentDarkModeTracking {
	case DarkModeForcedOff:
		return false
	case DarkModeForcedOn:
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
