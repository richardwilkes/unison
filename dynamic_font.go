// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
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

// RuneToGlyph implements Font.
func (f *DynamicFont) RuneToGlyph(r rune) uint16 {
	return f.resolvedFont().RuneToGlyph(r)
}

// RunesToGlyphs implements Font.
func (f *DynamicFont) RunesToGlyphs(r []rune) []uint16 {
	return f.resolvedFont().RunesToGlyphs(r)
}

// GlyphWidth implements Font.
func (f *DynamicFont) GlyphWidth(glyph uint16) float32 {
	return f.resolvedFont().GlyphWidth(glyph)
}

// GlyphWidths implements Font.
func (f *DynamicFont) GlyphWidths(glyphs []uint16) []float32 {
	return f.resolvedFont().GlyphWidths(glyphs)
}

// SimpleWidth implements Font.
func (f *DynamicFont) SimpleWidth(str string) float32 {
	return f.resolvedFont().SimpleWidth(str)
}

// Descriptor implements Font.
func (f *DynamicFont) Descriptor() FontDescriptor {
	return f.resolvedFont().Descriptor()
}

// TextBlobPosH implements Font.
func (f *DynamicFont) TextBlobPosH(glyphs []uint16, positions []float32, y float32) *TextBlob {
	return f.resolvedFont().TextBlobPosH(glyphs, positions, y)
}

func (f *DynamicFont) skiaFont() skia.Font {
	return f.resolvedFont().skiaFont()
}
