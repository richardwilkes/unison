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
	"github.com/richardwilkes/unison/internal/skia"
)

var _ Font = &IndirectFont{}

// IndirectFont holds a Font that references another font.
type IndirectFont struct {
	Font Font
}

// Face implements Font.
func (f *IndirectFont) Face() *FontFace {
	return f.Font.Face()
}

// Size implements Font.
func (f *IndirectFont) Size() float32 {
	return f.Font.Size()
}

// Metrics implements Font.
func (f *IndirectFont) Metrics() FontMetrics {
	return f.Font.Metrics()
}

// Baseline implements Font.
func (f *IndirectFont) Baseline() float32 {
	return f.Font.Baseline()
}

// LineHeight implements Font.
func (f *IndirectFont) LineHeight() float32 {
	return f.Font.LineHeight()
}

// Width implements Font.
func (f *IndirectFont) Width(str string) float32 {
	return f.Font.Width(str)
}

// Extents implements Font.
func (f *IndirectFont) Extents(str string) geom32.Size {
	return f.Font.Extents(str)
}

// Glyphs implements Font.
func (f *IndirectFont) Glyphs(text string) []uint16 {
	return f.Font.Glyphs(text)
}

// IndexForPosition implements Font.
func (f *IndirectFont) IndexForPosition(x float32, str string) int {
	return f.Font.IndexForPosition(x, str)
}

// PositionForIndex implements Font.
func (f *IndirectFont) PositionForIndex(index int, str string) float32 {
	return f.Font.PositionForIndex(index, str)
}

// Descriptor implements Font.
func (f *IndirectFont) Descriptor() FontDescriptor {
	return f.Font.Descriptor()
}

func (f *IndirectFont) skiaFont() skia.Font {
	return f.Font.skiaFont()
}
