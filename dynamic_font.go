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

var _ Font = &DynamicFont{}

// DynamicFont holds a Font that can be dynamically adjusted.
type DynamicFont struct {
	Resolver func() FontDescriptor
	desc     FontDescriptor
	font     Font
}

func (f *DynamicFont) resolvedFont() Font {
	if desc := f.Resolver(); desc != f.desc {
		f.desc = desc
		f.font = desc.Font()
	}
	return f.font
}

// Face implements Font.
func (f *DynamicFont) Face() *FontFace {
	return f.resolvedFont().Face()
}

// Size implements Font.
func (f *DynamicFont) Size() float32 {
	return f.resolvedFont().Size()
}

// Metrics implements Font.
func (f *DynamicFont) Metrics() FontMetrics {
	return f.resolvedFont().Metrics()
}

// Baseline implements Font.
func (f *DynamicFont) Baseline() float32 {
	return f.resolvedFont().Baseline()
}

// LineHeight implements Font.
func (f *DynamicFont) LineHeight() float32 {
	return f.resolvedFont().LineHeight()
}

// Width implements Font.
func (f *DynamicFont) Width(str string) float32 {
	return f.resolvedFont().Width(str)
}

// Extents implements Font.
func (f *DynamicFont) Extents(str string) geom32.Size {
	return f.resolvedFont().Extents(str)
}

// Glyphs implements Font.
func (f *DynamicFont) Glyphs(text string) []uint16 {
	return f.resolvedFont().Glyphs(text)
}

// IndexForPosition implements Font.
func (f *DynamicFont) IndexForPosition(x float32, str string) int {
	return f.resolvedFont().IndexForPosition(x, str)
}

// PositionForIndex implements Font.
func (f *DynamicFont) PositionForIndex(index int, str string) float32 {
	return f.resolvedFont().PositionForIndex(index, str)
}

// Descriptor implements Font.
func (f *DynamicFont) Descriptor() FontDescriptor {
	return f.resolvedFont().Descriptor()
}

func (f *DynamicFont) skiaFont() skia.Font {
	return f.resolvedFont().skiaFont()
}
