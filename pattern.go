// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/toolbox/xmath/geom32"

var _ Ink = &Pattern{}

// Pattern holds the information necessary to draw an image in a pattern.
type Pattern struct {
	Image           *Image
	Offset          geom32.Point
	Scale           geom32.Point
	TileModeX       TileMode
	TileModeY       TileMode
	SamplingOptions SamplingOptions
}

// Paint returns a Paint for this Pattern.
func (p *Pattern) Paint(canvas *Canvas, _ geom32.Rect, style PaintStyle) *Paint {
	paint := NewPaint()
	paint.SetStyle(style)
	scale := p.Scale
	if scale.X <= 0 {
		scale.X = 1
	}
	if scale.Y <= 0 {
		scale.Y = 1
	}
	imgScale := p.Image.Scale()
	paint.SetColor(Black)
	paint.SetShader(NewImageShader(canvas, p.Image, p.TileModeX, p.TileModeY, &p.SamplingOptions,
		&geom32.Matrix2D{
			ScaleX: scale.X * imgScale,
			ScaleY: scale.Y * imgScale,
			TransX: p.Offset.X,
			TransY: p.Offset.Y,
		}))
	return paint
}
