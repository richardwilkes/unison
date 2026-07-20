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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/unison/enums/thememode"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var w32AppUsesLightThemeValue = uint32(1)

// w32MainThreadID holds the thread ID of the main (UI) thread, captured during startup so that apiPostEmptyEvent can
// wake the main event loop from any goroutine without touching UI-thread-only state such as windowList.
var w32MainThreadID atomic.Uint32

func w32IsWindows10BuildOrGreater(build uint32) bool {
	cond := w32.VerSetConditionMask(0, w32.VER_MAJORVERSION, w32.VER_GREATER_EQUAL)
	cond = w32.VerSetConditionMask(cond, w32.VER_MINORVERSION, w32.VER_GREATER_EQUAL)
	cond = w32.VerSetConditionMask(cond, w32.VER_BUILDNUMBER, w32.VER_GREATER_EQUAL)
	return w32.RtlVerifyVersionInfo(&w32.OSVERSIONINFOEXW{
		MajorVersion: 10,
		MinorVersion: 0,
		BuildNumber:  build,
	}, w32.VER_MAJORVERSION|w32.VER_MINORVERSION|w32.VER_BUILDNUMBER, cond) == 0
}

func apiBeginStartup() error {
	w32MainThreadID.Store(windows.GetCurrentThreadId())
	apiFillKeyCodes()
	if w32IsWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
		w32.SetProcessDpiAwarenessContext(w32.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
	} else {
		w32.SetProcessDpiAwareness(w32.PROCESS_PER_MONITOR_DPI_AWARE)
	}
	return w32.OleInitialize()
}

func apiLateInit() {
	keyPath := `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath, syscall.KEY_NOTIFY|registry.QUERY_VALUE)
	if err != nil {
		errs.Log(errs.NewWithCause("unable to open dark mode key", err), "key", "CURRENT_USER", "path", keyPath)
		return
	}
	if err = w32UpdateTheme(k, true); err != nil {
		errs.Log(err)
		xio.CloseIgnoringErrors(k)
		return
	}
	go w32MonitorThemeChanges(k)
}

func w32MonitorThemeChanges(key registry.Key) {
	defer xio.CloseIgnoringErrors(key)
	for {
		if err := windows.RegNotifyChangeKeyValue(windows.Handle(key), false, windows.REG_NOTIFY_CHANGE_NAME|windows.REG_NOTIFY_CHANGE_LAST_SET, 0, false); err != nil {
			errs.Log(err)
			return
		}
		if err := w32UpdateTheme(key, false); err != nil {
			errs.Log(err)
			return
		}
	}
}

func apiFinalFinishStartup() {
	// Not used on Windows
}

func apiTerminate() error {
	// Not used on Windows
	return nil
}

func apiBeep() {
	w32.MessageBeep(w32.MB_OK)
}

func apiIsColorModeTrackingPossible() bool {
	return true
}

func apiIsDarkModeEnabled() bool {
	return atomic.LoadUint32(&w32AppUsesLightThemeValue) == 0
}

func apiDoubleClickInterval() time.Duration {
	return w32.GetDoubleClickTime()
}

func w32UpdateTheme(k registry.Key, sync bool) error {
	val, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return errs.NewWithCause("unable to retrieve current dark mode value", err)
	}
	var swapped bool
	if val == 0 {
		swapped = atomic.CompareAndSwapUint32(&w32AppUsesLightThemeValue, 1, 0)
	} else {
		swapped = atomic.CompareAndSwapUint32(&w32AppUsesLightThemeValue, 0, 1)
	}
	if swapped && CurrentThemeMode() == thememode.Auto {
		if sync {
			ThemeChanged()
		} else {
			InvokeTask(ThemeChanged)
		}
	}
	return nil
}

func apiPollEvents() {
	var msg w32.MSG
	for w32.PeekMessageW(&msg, 0, 0, 0, w32.PM_REMOVE) {
		if msg.Message == w32.WM_QUIT {
			closeAllWindows()
		} else {
			w32.TranslateMessage(&msg)
			w32.DispatchMessageW(&msg)
		}
	}

	// Hack to release some modifiers keys that the system did not emit KEYUP events for.
	if hwnd := w32.GetActiveWindow(); hwnd != 0 {
		if window := w32FindWindowByHWND(hwnd); window != nil {
			for _, key := range w32CollectStuckModifiers(window.pressedKeys, w32.GetKeyState) {
				window.keyReleased(key, window.CurrentKeyModifiers())
			}
		}
	}
}

// w32StuckModifierKeys maps each modifier key that Windows sometimes fails to deliver a KEYUP for to the virtual-key
// code used to query its current state. These must be virtual-key codes (VK_*), since that is what GetKeyState takes,
// not the raw scan codes used by rawScanCodeToKeyCodeMap to translate WM_KEYDOWN/WM_KEYUP events.
var w32StuckModifierKeys = []struct {
	virtualKey int
	key        KeyCode
}{
	{w32.VK_LSHIFT, KeyLShift},
	{w32.VK_RSHIFT, KeyRShift},
	{w32.VK_LWIN, KeyLCommand},
	{w32.VK_RWIN, KeyRCommand},
}

// w32CollectStuckModifiers returns the modifier keys that pressedKeys still considers held even though getKeyState
// reports them as up, meaning the KEYUP event was never delivered and a release needs to be synthesized.
func w32CollectStuckModifiers(pressedKeys map[KeyCode]bool, getKeyState func(virtualKey int) uint16) []KeyCode {
	var stuck []KeyCode
	for _, k := range w32StuckModifierKeys {
		if pressedKeys[k.key] && getKeyState(k.virtualKey)&0x8000 == 0 {
			stuck = append(stuck, k.key)
		}
	}
	return stuck
}

func apiWaitEvents() {
	w32.WaitMessage()
	apiPollEvents()
}

func apiPostEmptyEvent() {
	if platformInited.Load() {
		// Post directly to the main thread's message queue rather than to a window. apiPostEmptyEvent may be called
		// from arbitrary goroutines, so it must not touch windowList, which is UI-thread-only state, and posting to a
		// window would not work anyway when no windows exist: PostMessageW with a null hwnd posts to the *calling*
		// thread's queue, which would fail to wake the main loop blocked in WaitMessage.
		w32.PostThreadMessageW(w32MainThreadID.Load(), w32.WM_NULL, 0, 0)
	}
}
