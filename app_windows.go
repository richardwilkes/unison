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

var (
	appUsesLightThemeValue = uint32(1)
	helperWnd              windows.HWND // TODO: Create this during initialization
)

func beginStartup() error {
	// TODO: Does this need anything?
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
	/*
		TODO: IMPLEMENT!

		MSG msg;
		plafWindow* window;
		while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE)) {
			if (msg.message == WM_QUIT) {
				window = _plaf.windowListHead;
				while (window) {
					_plafInputWindowCloseRequest(window);
					window = window->next;
				}
			} else {
				TranslateMessage(&msg);
				DispatchMessageW(&msg);
			}
		}
		HWND handle = GetActiveWindow();
		if (handle) {
			window = GetPropW(handle, L"PLAF");
			if (window) {
				int i;
				const int keys[4][2] = {
					{ VK_LSHIFT, KEY_LEFT_SHIFT },
					{ VK_RSHIFT, KEY_RIGHT_SHIFT },
					{ VK_LWIN, KEY_LEFT_SUPER },
					{ VK_RWIN, KEY_RIGHT_SUPER }
				};
				for (i = 0; i < 4; i++) {
					const int vk = keys[i][0];
					const int key = keys[i][1];
					const int scancode = _plaf.scanCodes[key];
					if ((GetKeyState(vk) & 0x8000)) {
						continue;
					}
					if (window->keys[key] != INPUT_PRESS) {
						continue;
					}
					_plafInputKey(window, key, scancode, INPUT_RELEASE, getKeyMods());
				}
			}
		}
	*/
}

func waitEvents() {
	/*
		TODO: IMPLEMENT!
		w32.WaitMessage();
		plafPollEvents();
	*/
}

func postEmptyEvent() {
	if plafInited.Load() {
		/*
			TODO: IMPLEMENT!
			w32.PostMessageW(_plaf.win32HelperWindowHandle, WM_NULL, 0, 0)
		*/
	}
}
