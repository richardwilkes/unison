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
	"runtime"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison/internal/skia"
)

// TextBuilder provides Text construction.
type TextBuilder struct {
	builder skia.TextBlobBuilder
}

// NewTextBuilder creates a new NewTextBuilder.
func NewTextBuilder() *TextBuilder {
	b := &TextBuilder{builder: skia.TextBlobBuilderNew()}
	runtime.SetFinalizer(b, func(obj *TextBuilder) {
		ReleaseOnUIThread(func() {
			skia.TextBlobBuilderDelete(obj.builder)
		})
	})
	return b
}

// Text returns a newly-created Text from the contents of this TextBuilder. If the builder is empty, this will return
// nil.
func (b *TextBuilder) Text() *Text {
	return newText(skia.TextBlobBuilderMake(b.builder))
}

// AllocRun allocates a text run.
func (b *TextBuilder) AllocRun(font Font, glyphs []uint16, x, y float32) {
	skia.TextBlobBuilderAllocRun(b.builder, font.skiaFont(), glyphs, x, y)
}

// AllocRunPos allocates a text run.
func (b *TextBuilder) AllocRunPos(font Font, glyphs []uint16, pos []geom32.Point) {
	skia.TextBlobBuilderAllocRunPos(b.builder, font.skiaFont(), glyphs, pos)
}

// AllocRunPosH allocates a text run.
func (b *TextBuilder) AllocRunPosH(font Font, glyphs []uint16, pos []float32, y float32) {
	skia.TextBlobBuilderAllocRunPosH(b.builder, font.skiaFont(), glyphs, pos, y)
}

func (b *TextBuilder) AllocTextRun(font Font, text string, x, y float32) {
	face := font.Face()
	glyphs := font.Glyphs(text)
	runes := []rune(text)
	start := 0
	i := 0
	for i < len(glyphs) {
		if glyphs[i] == 0 {
			if i > start {
				glyphRun := glyphs[start:i]
				b.AllocRun(font, glyphRun, x, y)
				starts := font.GlyphStarts(glyphRun)
				x = starts[len(starts)-1]
			}
			altFace := face.FallbackForCharacter(runes[i])
			glyphRun := glyphs[i : i+1]
			b.AllocRun(altFace.Font(font.Size()), glyphRun, x, y)
			starts := font.GlyphStarts(glyphRun)
			x = starts[len(starts)-1]
			start = i + 1
		}
		i++
	}
	if start < len(glyphs)-1 {
		glyphRun := glyphs[start:]
		b.AllocRun(font, glyphRun, x, y)
	}
}
