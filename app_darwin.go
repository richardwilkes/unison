// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"strings"
	"time"

	"github.com/progrium/macdriver/objc"
	"github.com/richardwilkes/unison/internal/ns"
)

func platformEarlyInit() {
	if openURLsCallback != nil {
		appDelegate := ns.App().GetDelegate()
		cls := objc.NewClass("UnisonAppDelegate", "NSObject")
		cls.AddMethod("applicationShouldTerminate:", func(_, p1 objc.Object) uint {
			return uint(appDelegate.Send("applicationShouldTerminte:", p1).Uint())
		})
		cls.AddMethod("applicationDidChangeScreenParameters:", func(_, p1 objc.Object) {
			appDelegate.Send("applicationDidChangeScreenParameters:", p1)
		})
		cls.AddMethod("applicationWillFinishLaunching:", func(_, p1 objc.Object) {
			appDelegate.Send("applicationWillFinishLaunching:", p1)
		})
		cls.AddMethod("applicationDidFinishLaunching:", func(_, p1 objc.Object) {
			appDelegate.Send("applicationDidFinishLaunching:", p1)
		})
		cls.AddMethod("applicationDidHide:", func(_, p1 objc.Object) {
			appDelegate.Send("applicationDidHide:", p1)
		})
		// All of the methods above this point are just pass-throughs to glfw. If glfw adds more methods, corresponding
		// additions should be made here.
		cls.AddMethod("application:openURLs:", func(_, app, urls objc.Object) {
			if data := ns.URLArrayToStringSlice(ns.Array{Object: urls}); len(data) != 0 {
				openURLsCallback(data)
			}
		})
		ns.App().SetDelegate(cls.Alloc().Init())
	}
}

func platformLateInit() {
	cls := objc.NewClass("ThemeDelegate", "NSObject")
	cls.AddMethod("themeChanged:", func(objc.Object) { themeChanged() })
	delegate := cls.Alloc().Init()
	def := ns.DefaultCenter()
	selector := objc.Sel("themeChanged:")
	def.AddObserver(delegate, selector, "AppleInterfaceThemeChangedNotification")
	def.AddObserver(delegate, selector, "AppleColorPreferencesChangedNotification")

	ns.App().SetActivationPolicy(ns.ActivationPolicyRegular)
}

func platformBeep() {
	ns.Beep()
}

func platformIsDarkModeTrackingPossible() bool {
	return true
}

func platformIsDarkModeEnabled() bool {
	return strings.Contains(strings.ToLower(ns.StandardUserDefaults().StringForKey("AppleInterfaceStyle")), "dark")
}

func platformDoubleClickInterval() time.Duration {
	return ns.DoubleClickInterval()
}
