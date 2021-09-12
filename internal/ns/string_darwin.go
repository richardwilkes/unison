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
	"reflect"
	"unsafe"

	"github.com/progrium/macdriver/objc"
)

// utf8StringEncoding https://developer.apple.com/documentation/foundation/1497293-string_encodings/nsutf8stringencoding?language=occ
const utf8StringEncoding = 4

var stringClass = objc.Get("NSString")

// String https://developer.apple.com/documentation/foundation/nsstring?language=occ
type String struct {
	objc.Object
}

// StringFromString returns a String created by copying the data from the given string.
func StringFromString(str string) String {
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&str))
	return String{Object: stringClass.Alloc().Send("initWithBytes:length:encoding:", hdr.Data, hdr.Len,
		utf8StringEncoding)}
}
