// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var _ Border = &EmptyBorder{}

// EmptyBorder provides and empty border with the specified insets.
type EmptyBorder struct {
	insets geom32.Insets
}

// NewEmptyBorder creates a new empty border with the specified insets.
func NewEmptyBorder(insets geom32.Insets) *EmptyBorder {
	return &EmptyBorder{insets: insets}
}

// Insets returns the insets describing the space the border occupies on each side.
func (b *EmptyBorder) Insets() geom32.Insets {
	return b.insets
}

// Draw the border into rect.
func (b *EmptyBorder) Draw(_ *Canvas, _ geom32.Rect) {
}
