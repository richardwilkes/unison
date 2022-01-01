// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

import (
	"github.com/progrium/macdriver/objc"
)

// ActivationPolicy https://developer.apple.com/documentation/appkit/nsapplicationactivationpolicy?language=objc
type ActivationPolicy uint

// https://developer.apple.com/documentation/appkit/nsapplicationactivationpolicy?language=objc
const (
	ActivationPolicyRegular ActivationPolicy = iota
	ActivationPolicyAccessory
	ActivationPolicyProhibited
)

var applicationClass = objc.Get("NSApplication")

// Application https://developer.apple.com/documentation/appkit/nsapplication?language=objc
type Application struct {
	objc.Object
}

// App https://developer.apple.com/documentation/appkit/nsapp?language=objc
func App() Application {
	return Application{Object: applicationClass.Send("sharedApplication")}
}

// GetDelegate https://developer.apple.com/documentation/appkit/nsapplication/1428705-delegate?language=objc
func (a Application) GetDelegate() objc.Object {
	return a.Send("delegate")
}

// SetDelegate https://developer.apple.com/documentation/appkit/nsapplication/1428705-delegate?language=objc
func (a Application) SetDelegate(delegate objc.Object) {
	a.Send("setDelegate:", delegate)
}

// HideOtherApplications https://developer.apple.com/documentation/appkit/nsapplication/1428746-hideotherapplications?language=objc
func (a Application) HideOtherApplications() {
	a.Send("hideOtherApplications:", nil)
}

// UnhideAllApplications https://developer.apple.com/documentation/appkit/nsapplication/1428737-unhideallapplications?language=objc
func (a Application) UnhideAllApplications() {
	a.Send("unhideAllApplications:", nil)
}

// SetActivationPolicy https://developer.apple.com/documentation/appkit/nsapplication/1428621-setactivationpolicy?language=objc
func (a Application) SetActivationPolicy(policy ActivationPolicy) {
	a.Send("setActivationPolicy:", policy)
}

// SetMainMenu https://developer.apple.com/documentation/appkit/nsapplication/1428634-mainmenu?language=objc
func (a Application) SetMainMenu(menu Menu) {
	a.Send("setMainMenu:", menu)
}

// SetServicesMenu https://developer.apple.com/documentation/appkit/nsapplication/1428608-servicesmenu?language=objc
func (a Application) SetServicesMenu(menu Menu) {
	a.Send("setServicesMenu:", menu)
}

// SetWindowsMenu https://developer.apple.com/documentation/appkit/nsapplication/1428547-windowsmenu?language=objc
func (a Application) SetWindowsMenu(menu Menu) {
	a.Send("setWindowsMenu:", menu)
}

// SetHelpMenu https://developer.apple.com/documentation/appkit/nsapplication/1428644-helpmenu?language=objc
func (a Application) SetHelpMenu(menu Menu) {
	a.Send("setHelpMenu:", menu)
}
