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

var colorClass = objc.Get("NSColor")

// Color https://developer.apple.com/documentation/appkit/nscolor?language=objc
type Color struct {
	objc.Object
}

// ControlAccentColor https://developer.apple.com/documentation/appkit/nscolor/3000782-controlaccentcolor?language=objc
func ControlAccentColor() Color {
	return Color{Object: colorClass.Send("controlAccentColor")}
}

// ColorUsingColorSpace https://developer.apple.com/documentation/appkit/nscolor/1527379-colorusingcolorspace/
func (c Color) ColorUsingColorSpace(cs ColorSpace) Color {
	return Color{Object: c.Send("colorUsingColorSpace:", cs)}
}

// GetRedGreenBlueAlpha https://developer.apple.com/documentation/appkit/nscolor/1527848-getred?language=objc
func (c Color) GetRedGreenBlueAlpha() (r, g, b, a float64) {
	c.Send("getRed:green:blue:alpha:", &r, &g, &b, &a)
	return
}
