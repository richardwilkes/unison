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

var appUsesLightThemeValue = uint32(1)

func isWindows10BuildOrGreater(build uint32) bool {
	cond := w32.VerSetConditionMask(0, w32.VER_MAJORVERSION, w32.VER_GREATER_EQUAL)
	cond = w32.VerSetConditionMask(cond, w32.VER_MINORVERSION, w32.VER_GREATER_EQUAL)
	cond = w32.VerSetConditionMask(cond, w32.VER_BUILDNUMBER, w32.VER_GREATER_EQUAL)
	return w32.RtlVerifyVersionInfo(&w32.OSVERSIONINFOEXW{
		MajorVersion: 10,
		MinorVersion: 0,
		BuildNumber:  build,
	}, w32.VER_MAJORVERSION|w32.VER_MINORVERSION|w32.VER_BUILDNUMBER, cond) == 0
}

func beginStartup() error {
	fillKeyCodes()
	if isWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
		w32.SetProcessDpiAwarenessContext(w32.DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)
	} else {
		w32.SetProcessDpiAwareness(w32.PROCESS_PER_MONITOR_DPI_AWARE)
	}
	return nil
}

func lateInit() {
	keyPath := `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	k, err := registry.OpenKey(registry.CURRENT_USER, keyPath, syscall.KEY_NOTIFY|registry.QUERY_VALUE)
	if err != nil {
		errs.Log(errs.NewWithCause("unable to open dark mode key", err), "key", "CURRENT_USER", "path", keyPath)
		return
	}
	if err = updateTheme(k, true); err != nil {
		errs.Log(err)
		xio.CloseIgnoringErrors(k)
		return
	}
	go func() {
		for {
			w32.RegNotifyChangeKeyValue(k, false, w32.RegNotifyChangeName|w32.RegNotifyChangeLastSet, 0, false)
			if err = updateTheme(k, false); err != nil {
				errs.Log(err)
				xio.CloseIgnoringErrors(k)
				return
			}
		}
	}()
}

func finalFinishStartup() {
	// TODO: Does this need anything?
}

func terminate() error {
	// TODO: Does this need anything?
	return nil
}

func beep() {
	w32.MessageBeep(w32.MBDefault)
}

func isColorModeTrackingPossible() bool {
	return true
}

func isDarkModeEnabled() bool {
	return atomic.LoadUint32(&appUsesLightThemeValue) == 0
}

func doubleClickInterval() time.Duration {
	return w32.GetDoubleClickTime()
}

func updateTheme(k registry.Key, sync bool) error {
	val, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return errs.NewWithCause("unable to retrieve current dark mode value", err)
	}
	var swapped bool
	if val == 0 {
		swapped = atomic.CompareAndSwapUint32(&appUsesLightThemeValue, 1, 0)
	} else {
		swapped = atomic.CompareAndSwapUint32(&appUsesLightThemeValue, 0, 1)
	}
	if swapped && currentThemeMode == thememode.Auto {
		if sync {
			ThemeChanged()
		} else {
			InvokeTask(ThemeChanged)
		}
	}
	return nil
}

func pollEvents() {
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
		if window := findWindowByHWND(hwnd); window != nil {
			keys := [4]struct {
				vk  int
				key KeyCode
			}{
				{0x02A, KeyLShift},
				{0x036, KeyRShift},
				{0x15B, KeyLCommand},
				{0x15C, KeyRCommand},
			}
			for _, k := range keys {
				if !window.pressedKeys[k.key] {
					continue
				}
				if w32.GetKeyState(k.vk)&0x8000 != 0 {
					continue
				}
				window.keyReleased(k.key, window.CurrentKeyModifiers())
			}
		}
	}
}

func waitEvents() {
	w32.WaitMessage()
	pollEvents()
}

func postEmptyEvent() {
	if plafInited.Load() {
		var wnd windows.HWND
		if len(windowList) != 0 {
			wnd = windowList[0].wnd.wnd
		}
		w32.PostMessageW(wnd, w32.WM_NULL, 0, 0)
	}
}
