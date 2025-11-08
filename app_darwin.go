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

	"github.com/richardwilkes/glfw"
	"github.com/richardwilkes/unison/internal/ns"
)

var (
	pendingFilesLock   sync.Mutex
	pendingFilesToOpen []string
	okToIssueFileOpens bool
)

func platformEarlyInit() {
	// macOS requires both of these hints to be set
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)

	ns.InstallAppDelegate(func(paths []string) {
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
	})
}

func platformLateInit() {
	ns.InstallSystemThemeChangedCallback(ThemeChanged)
	ns.SetActivationPolicy(ns.ActivationPolicyRegular)
}

func platformFinishedStartup() {
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

func platformBeep() {
	ns.Beep()
}

func platformIsDarkModeTrackingPossible() bool {
	return true
}

func platformIsDarkModeEnabled() bool {
	return ns.IsDarkModeEnabled()
}

func platformDoubleClickInterval() time.Duration {
	return ns.DoubleClickInterval()
}
