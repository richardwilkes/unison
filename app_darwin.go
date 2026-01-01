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
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/mac"
)

var (
	pendingFilesLock   sync.Mutex
	pendingFilesToOpen []string
	okToIssueFileOpens bool
)

func beginStartup() error {
	mac.AppShouldTerminateCallback = func() {
		var last *Window
		for len(windowList) > 0 {
			windowList[0].nativeRequestClose()
			if len(windowList) != 0 {
				if windowList[0] == last {
					break
				}
				last = windowList[0]
			}
		}
		xos.Exit(0)
	}
	mac.AppDidChangeScreenParameters = func() {
		for _, w := range windowList {
			w.glCtx.ctx.Update()
		}
	}
	mac.AppDidFinishLaunchingCallback = func() {
		mac.PostEmptyEvent()
		mac.StopMainEventLoop()
	}
	mac.OpenFilesCallback = func(paths []string) {
		pendingFilesLock.Lock()
		defer pendingFilesLock.Unlock()
		if okToIssueFileOpens {
			InvokeTask(func() {
				if openFilesCallback != nil {
					openFilesCallback(paths)
				}
			})
		} else {
			pendingFilesToOpen = append(pendingFilesToOpen, paths...)
		}
	}
	// NOTE: Two additional app delegate callbacks exist: AppWillFinishLaunchingCallback and AppDidHideCallback.
	if err := mac.InstallMacAppDelegate(); err != nil {
		return err
	}
	fillKeyCodes()
	initNativeWindowCallbacks()
	mac.FinishLaunching()
	return nil
}

func lateInit() {
	mac.InstallSystemThemeChangedCallback(ThemeChanged)
}

func finalFinishStartup() {
	pendingFilesLock.Lock()
	defer pendingFilesLock.Unlock()
	okToIssueFileOpens = true
	if len(pendingFilesToOpen) != 0 {
		paths := pendingFilesToOpen
		pendingFilesToOpen = nil
		if openFilesCallback != nil {
			openFilesCallback(paths)
		}
	}
}

func terminate() error {
	mac.UninstallMacAppDelegate()
	return nil
}

func beep() {
	mac.Beep()
}

func isColorModeTrackingPossible() bool {
	return true
}

func isDarkModeEnabled() bool {
	return mac.IsDarkModeEnabled()
}

func doubleClickInterval() time.Duration {
	return mac.DoubleClickInterval()
}

func pollEvents() {
	mac.PollEvents()
}

func waitEvents() {
	mac.WaitEvents()
}

func postEmptyEvent() {
	if plafInited.Load() {
		mac.PostEmptyEvent()
	}
}
