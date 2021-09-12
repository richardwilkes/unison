// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

import "github.com/progrium/macdriver/objc"

var runningApplicationClass = objc.Get("NSRunningApplication")

// RunningApplication https://developer.apple.com/documentation/appkit/nsrunningapplication?language=objc
type RunningApplication struct {
	objc.Object
}

// CurrentApplication https://developer.apple.com/documentation/appkit/nsrunningapplication/1533604-currentapplication?language=objc
func CurrentApplication() RunningApplication {
	return RunningApplication{Object: runningApplicationClass.Send("currentApplication")}
}

// Hide https://developer.apple.com/documentation/appkit/nsrunningapplication/1526608-hide?language=objc
func (a RunningApplication) Hide() {
	a.Send("hide")
}
