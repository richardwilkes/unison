// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/tilemode"
)

var _ Ink = &Pattern{}

// Pattern holds the information necessary to draw an image in a pattern.
type Pattern struct {
	Image           *Image
	Offset          geom.Point
	Scale           geom.Point
	TileModeX       tilemode.Enum
	TileModeY       tilemode.Enum
	SamplingOptions SamplingOptions
}

// Paint returns a Paint for this Pattern.
func (p *Pattern) Paint(canvas *Canvas, _ geom.Rect, style paintstyle.Enum) *Paint {
	paint := NewPaint()
	paint.SetStyle(style)
	scale := p.Scale
	if scale.X <= 0 {
		scale.X = 1
	}
	if scale.Y <= 0 {
		scale.Y = 1
	}
	scale = scale.MulPt(p.Image.Scale())
	paint.SetColor(Black)
	paint.SetShader(NewImageShader(canvas, p.Image, p.TileModeX, p.TileModeY, &p.SamplingOptions,
		geom.Matrix{
			ScaleX: scale.X,
			ScaleY: scale.Y,
			TransX: p.Offset.X,
			TransY: p.Offset.Y,
		}))
	return paint
}
