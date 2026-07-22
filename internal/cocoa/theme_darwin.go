// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"strings"
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
)

var (
	systemThemeChangedCallback func()
	themeObserverOnce          sync.Once
)

// InstallSystemThemeChangedCallback installs f as the function invoked when the system theme (dark/light mode or
// accent colors) changes. Distributed notifications are delivered on the run loop of the thread that first created
// the default NSDistributedNotificationCenter (verified empirically — the addObserver: thread is irrelevant), so
// the first call with a non-nil f must happen on the main thread running the event loop, before anything else in
// the process touches the distributed center from another thread. unison calls this once from the main thread
// during startup, matching where the cgo bridge ran it.
func InstallSystemThemeChangedCallback(f func()) {
	systemThemeChangedCallback = f
	if f != nil {
		installThemeObserver()
	}
}

func installThemeObserver() {
	themeObserverOnce.Do(func() {
		if err := registerThemeObserver("macThemeDelegate"); err != nil {
			errs.Log(err)
		}
	})
}

// registerThemeObserver registers the theme delegate class under the given name and subscribes an instance of it to
// the theme/accent distributed notifications. On class-registration failure (e.g. the host process already defines a
// class with that name) it returns the error so the caller can log it and degrade — dark-mode tracking is lost, but
// startup continues, matching the app/window/menu delegate registration paths.
func registerThemeObserver(className string) error {
	cls, err := objc.RegisterClass(className, Cls("NSObject"), nil, nil, []objc.MethodDef{{
		Cmd: Sel("themeChanged:"),
		Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
			if systemThemeChangedCallback != nil {
				systemThemeChangedCallback()
			}
		},
	}})
	if err != nil {
		return errs.NewWithCause("unable to register "+className+" class", err)
	}
	WithPool(func() {
		delegate := objc.ID(cls).Send(Sel("new"))
		center := objc.ID(Cls("NSDistributedNotificationCenter")).Send(Sel("defaultCenter"))
		for _, name := range []string{
			"AppleInterfaceThemeChangedNotification",
			"AppleColorPreferencesChangedNotification",
		} {
			center.Send(Sel("addObserver:selector:name:object:"), delegate, Sel("themeChanged:"),
				NSStringFromGo(name), 0)
		}
	})
	return nil
}

// IsDarkModeEnabled returns true if the system is currently configured for dark mode. The cgo bridge read the
// AppleInterfaceStyle preference via CFPreferencesCopyAppValue; NSUserDefaults' standard search list covers the
// same domains (the value lives in NSGlobalDomain), so the result is identical.
func IsDarkModeEnabled() bool {
	var dark bool
	WithPool(func() {
		style := objc.ID(Cls("NSUserDefaults")).Send(Sel("standardUserDefaults")).
			Send(Sel("stringForKey:"), NSStringFromGo("AppleInterfaceStyle"))
		dark = strings.Contains(strings.ToLower(GoStringFromNSString(style)), "dark")
	})
	return dark
}
