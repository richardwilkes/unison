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
	"unsafe"

	"github.com/progrium/macdriver/objc"
)

var dataClass = objc.Get("NSData")

// Data https://developer.apple.com/documentation/foundation/nsdata?language=objc
type Data struct {
	objc.Object
}

// NewData https://developer.apple.com/documentation/foundation/nsdata/1547231-datawithbytes?language=objc
func NewData(buffer []byte) Data {
	return Data{Object: dataClass.Send("dataWithBytes:length:", uintptr(unsafe.Pointer(&buffer[0])), len(buffer))}
}

// Length https://developer.apple.com/documentation/foundation/nsdata/1416769-length?language=objc
func (d Data) Length() uint {
	return uint(d.Send("length").Uint())
}

// Bytes https://developer.apple.com/documentation/foundation/nsdata/1411450-getbytes?language=objc
func (d Data) Bytes(buffer []byte) {
	d.Send("getBytes:length:", uintptr(unsafe.Pointer(&buffer[0])), len(buffer))
}
