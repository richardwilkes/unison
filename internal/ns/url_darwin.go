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

var urlClass = objc.Get("NSURL")

// URL https://developer.apple.com/documentation/foundation/nsurl?language=objc
type URL struct {
	objc.Object
}

// NewFileURL https://developer.apple.com/documentation/foundation/nsurl/1414650-fileurlwithpath?language=objc
func NewFileURL(str string) URL {
	s := StringFromString(str)
	defer s.Release()
	return URL{Object: urlClass.Send("fileURLWithPath:", s)}
}

// AbsoluteString https://developer.apple.com/documentation/foundation/nsurl/1409868-absolutestring?language=objc
func (u URL) AbsoluteString() string {
	return u.Send("absoluteURL").String()
}
