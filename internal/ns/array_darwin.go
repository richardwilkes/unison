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

// Array https://developer.apple.com/documentation/foundation/nsarray?language=objc
type Array struct {
	objc.Object
}

// Count https://developer.apple.com/documentation/foundation/nsarray/1409982-count?language=objc
func (a Array) Count() int {
	return int(a.Send("count").Uint())
}

// ObjectAtIndex https://developer.apple.com/documentation/foundation/nsarray/1417555-objectatindex?language=objc
func (a Array) ObjectAtIndex(index int) objc.Object {
	return a.Send("objectAtIndex:", uint(index))
}

// StringAtIndex returns the String at the specified index. No check is made to verify the object is actually a String.
func (a Array) StringAtIndex(index int) String {
	return String{Object: a.ObjectAtIndex(index)}
}
