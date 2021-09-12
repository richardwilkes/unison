// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

// Point https://developer.apple.com/documentation/foundation/nspoint?language=objc
type Point struct {
	X float64
	Y float64
}

// MakePoint https://developer.apple.com/documentation/foundation/1391309-nsmakepoint?language=objc
func MakePoint(x, y float64) Point {
	return Point{X: x, Y: y}
}

// Size https://developer.apple.com/documentation/foundation/nssize?language=objc
type Size struct {
	Width  float64
	Height float64
}

// MakeSize https://developer.apple.com/documentation/foundation/1391295-nsmakesize?language=objc
func MakeSize(w, h float64) Size {
	return Size{Width: w, Height: h}
}

// Rect https://developer.apple.com/documentation/foundation/nsrect?language=objc
type Rect struct {
	Origin Point
	Size   Size
}

// MakeRect https://developer.apple.com/documentation/foundation/1391329-nsmakerect?language=objc
func MakeRect(x, y, w, h float64) Rect {
	return Rect{Origin: MakePoint(x, y), Size: MakeSize(w, h)}
}
