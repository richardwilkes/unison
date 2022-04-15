// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

var _ Border = &CompoundBorder{}

// CompoundBorder provides stacking of borders together.
type CompoundBorder struct {
	borders []Border
}

// NewCompoundBorder creates a border that contains other borders. The first one will be drawn in the outermost
// position, with each successive one moving further into the interior.
func NewCompoundBorder(borders ...Border) *CompoundBorder {
	return &CompoundBorder{borders: borders}
}

// Insets returns the insets describing the space the border occupies on each side.
func (b *CompoundBorder) Insets() Insets {
	insets := Insets{}
	for _, one := range b.borders {
		insets.Add(one.Insets())
	}
	return insets
}

// Draw the border into rect.
func (b *CompoundBorder) Draw(canvas *Canvas, rect Rect) {
	for _, one := range b.borders {
		canvas.Save()
		one.Draw(canvas, rect)
		canvas.Restore()
		rect.Inset(one.Insets())
	}
}
