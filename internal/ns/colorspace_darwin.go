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

var colorSpaceClass = objc.Get("NSColorSpace")

// ColorSpace https://developer.apple.com/documentation/appkit/nscolorspace?language=objc
type ColorSpace struct {
	objc.Object
}

// DeviceRGBColorSpace https://developer.apple.com/documentation/appkit/nscolorspace/1412066-devicergbcolorspace?language=objc
func DeviceRGBColorSpace() ColorSpace {
	return ColorSpace{Object: colorSpaceClass.Send("deviceRGBColorSpace")}
}
