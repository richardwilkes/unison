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
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/mac"
)

var (
	macPendingFilesLock   sync.Mutex
	macPendingFilesToOpen []string
	macMayIssueFileOpens  bool
)

func apiBeginStartup() error {
	mac.AppShouldTerminateCallback = func() {
		closeAllWindows()
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
		macPendingFilesLock.Lock()
		defer macPendingFilesLock.Unlock()
		if macMayIssueFileOpens {
			InvokeTask(func() {
				if openFilesCallback != nil {
					openFilesCallback(paths)
				}
			})
		} else {
			macPendingFilesToOpen = append(macPendingFilesToOpen, paths...)
		}
	}
	// NOTE: Two additional app delegate callbacks exist: AppWillFinishLaunchingCallback and AppDidHideCallback.
	if err := mac.InstallMacAppDelegate(); err != nil {
		return err
	}
	apiFillKeyCodes()
	macInitWindowCallbacks()
	mac.FinishLaunching()
	return nil
}

func apiLateInit() {
	mac.InstallSystemThemeChangedCallback(ThemeChanged)
}

func apiFinalFinishStartup() {
	macPendingFilesLock.Lock()
	defer macPendingFilesLock.Unlock()
	macMayIssueFileOpens = true
	if len(macPendingFilesToOpen) != 0 {
		paths := macPendingFilesToOpen
		macPendingFilesToOpen = nil
		if openFilesCallback != nil {
			openFilesCallback(paths)
		}
	}
}

func apiTerminate() error {
	mac.UninstallMacAppDelegate()
	return nil
}

func apiBeep() {
	mac.Beep()
}

func apiIsColorModeTrackingPossible() bool {
	return true
}

func apiIsDarkModeEnabled() bool {
	return mac.IsDarkModeEnabled()
}

func apiDoubleClickInterval() time.Duration {
	return mac.DoubleClickInterval()
}

func apiPollEvents() {
	mac.PollEvents()
}

func apiWaitEvents() {
	mac.WaitEvents()
}

func apiPostEmptyEvent() {
	if platformInited.Load() {
		mac.PostEmptyEvent()
	}
}
