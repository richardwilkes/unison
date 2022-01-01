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
	"time"

	"github.com/progrium/macdriver/objc"
)

var eventClass = objc.Get("NSEvent")

// DoubleClickInterval https://developer.apple.com/documentation/appkit/nsevent/1528384-doubleclickinterval?language=objc
func DoubleClickInterval() time.Duration {
	return time.Duration(eventClass.Send("doubleClickInterval").Float()*1000) * time.Millisecond
}
