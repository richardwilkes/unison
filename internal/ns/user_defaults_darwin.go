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

var userDefaultsClass = objc.Get("NSUserDefaults")

// UserDefaults https://developer.apple.com/documentation/foundation/nsuserdefaults/
type UserDefaults struct {
	objc.Object
}

// StandardUserDefaults https://developer.apple.com/documentation/foundation/nsuserdefaults/1416603-standarduserdefaults
func StandardUserDefaults() UserDefaults {
	return UserDefaults{Object: userDefaultsClass.Send("standardUserDefaults")}
}

// StringForKey https://developer.apple.com/documentation/foundation/nsuserdefaults/1416700-stringforkey
func (u UserDefaults) StringForKey(key string) string {
	keyString := StringFromString(key)
	defer keyString.Release()
	return u.Send("stringForKey:", keyString).String()
}
