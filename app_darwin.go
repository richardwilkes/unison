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

	"github.com/richardwilkes/unison/internal/cocoa"
)

var (
	macPendingFilesLock   sync.Mutex
	macPendingFilesToOpen []string
	macMayIssueFileOpens  bool
)

func apiBeginStartup() error {
	// System-initiated termination (e.g. Dock -> Quit, logout) takes the same path as the in-app Quit menu item, so
	// the AllowQuitCallback and per-window AllowCloseCallback vetoes are honored for both. The delegate always reports
	// NSTerminateCancel to AppKit, so when the request is permitted, AttemptQuit is responsible for actually exiting.
	cocoa.AppShouldTerminateCallback = AttemptQuit
	cocoa.AppDidChangeScreenParameters = func() {
		for _, w := range windowList {
			w.glCtx.ctx.Update()
		}
	}
	cocoa.AppDidFinishLaunchingCallback = func() {
		cocoa.PostEmptyEvent()
		cocoa.StopMainEventLoop()
	}
	cocoa.OpenFilesCallback = macOpenFilesRequested
	// NOTE: Two additional app delegate callbacks exist: AppWillFinishLaunchingCallback and AppDidHideCallback.
	if err := cocoa.InstallMacAppDelegate(); err != nil {
		return err
	}
	apiFillKeyCodes()
	macInitWindowCallbacks()
	cocoa.FinishLaunching()
	return nil
}

func apiLateInit() {
	cocoa.InstallSystemThemeChangedCallback(ThemeChanged)
}

// macOpenFilesRequested is installed as cocoa.OpenFilesCallback. Until apiFinalFinishStartup marks startup as
// complete, requests are buffered; afterward they are routed to the user's callback via the task queue. The decision
// is made under macPendingFilesLock, but InvokeTask is called outside it so the lock is never held while other
// package machinery runs.
func macOpenFilesRequested(paths []string) {
	macPendingFilesLock.Lock()
	mayIssue := macMayIssueFileOpens
	if !mayIssue {
		macPendingFilesToOpen = append(macPendingFilesToOpen, paths...)
	}
	macPendingFilesLock.Unlock()
	if mayIssue {
		InvokeTask(func() {
			if openFilesCallback != nil {
				openFilesCallback(paths)
			}
		})
	}
}

func apiFinalFinishStartup() {
	macPendingFilesLock.Lock()
	macMayIssueFileOpens = true
	paths := macPendingFilesToOpen
	macPendingFilesToOpen = nil
	macPendingFilesLock.Unlock()
	// The callback must be invoked without holding macPendingFilesLock: if it pumps events (e.g. via RunModal) and
	// AppKit delivers another open-files request during the nested loop, macOpenFilesRequested would re-lock the
	// same non-reentrant mutex on the same thread and self-deadlock.
	if len(paths) != 0 && openFilesCallback != nil {
		openFilesCallback(paths)
	}
}

func apiTerminate() error {
	cocoa.UninstallMacAppDelegate()
	return nil
}

func apiBeep() {
	cocoa.Beep()
}

func apiIsColorModeTrackingPossible() bool {
	return true
}

func apiIsDarkModeEnabled() bool {
	return cocoa.IsDarkModeEnabled()
}

func apiDoubleClickInterval() time.Duration {
	return cocoa.DoubleClickInterval()
}

func apiPollEvents() {
	cocoa.PollEvents()
}

func apiWaitEvents() {
	cocoa.WaitEvents()
}

func apiPostEmptyEvent() {
	if platformInited.Load() {
		cocoa.PostEmptyEvent()
	}
}
