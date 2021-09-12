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

var mutableArrayClass = objc.Get("NSMutableArray")

// MutableArray https://developer.apple.com/documentation/foundation/nsmutablearray?language=objc
type MutableArray struct {
	Array
}

// MutableArrayWithCapacity https://developer.apple.com/documentation/foundation/nsmutablearray/1460057-arraywithcapacity?language=objc
func MutableArrayWithCapacity(capacity int) MutableArray {
	return MutableArray{Array: Array{Object: mutableArrayClass.Send("arrayWithCapacity:", uint(capacity))}}
}

// AddObject https://developer.apple.com/documentation/foundation/nsmutablearray/1411274-addobject?language=objc
func (a MutableArray) AddObject(obj objc.Object) {
	a.Send("addObject:", obj)
}
