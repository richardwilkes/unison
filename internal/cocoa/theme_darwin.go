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

const (
	// AppearanceNameAqua is NSAppearanceNameAqua, the light appearance.
	AppearanceNameAqua = "NSAppearanceNameAqua"
	// AppearanceNameDarkAqua is NSAppearanceNameDarkAqua, the dark appearance.
	AppearanceNameDarkAqua = "NSAppearanceNameDarkAqua"
)

var (
	systemThemeChangedCallback func()
	themeObserverOnce          sync.Once
)

// SetAppAppearance sets the application's appearance (NSApp.appearance) to the named one, AppearanceNameAqua or
// AppearanceNameDarkAqua, or clears it when name is empty so that the application follows the system appearance again.
// Every window, menu and panel the application shows takes the appearance from it. It must be called on the main
// thread.
func SetAppAppearance(name string) {
	WithPool(func() {
		var appearance objc.ID
		if name != "" {
			appearance = objc.ID(Cls("NSAppearance")).Send(Sel("appearanceNamed:"), NSStringFromGo(name))
		}
		sharedApp().Send(Sel("setAppearance:"), appearance)
	})
}

// AppAppearanceName returns the name of the appearance set on the application, or an empty string when none has been
// set and it follows the system.
func AppAppearanceName() string {
	var name string
	WithPool(func() {
		if appearance := sharedApp().Send(Sel("appearance")); appearance != 0 {
			name = GoStringFromNSString(appearance.Send(Sel("name")))
		}
	})
	return name
}

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
