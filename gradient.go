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
	"fmt"

	"github.com/richardwilkes/toolbox/xmath/geom"
)

var _ Ink = &Gradient{}

// Stop provides information about the color and position of one 'color stop' in a gradient.
type Stop struct {
	Color    ColorProvider
	Location float32
}

func (s Stop) String() string {
	return fmt.Sprintf("%v:%v", s.Color.GetColor(), s.Location)
}

// Gradient defines a smooth transition between colors across an area. Start and End should hold values from 0 to 1.
// These will be be used to set a relative starting and ending position for the gradient. If StartRadius and EndRadius
// are both greater than 0, then the gradient will be a radial one instead of a linear one.
type Gradient struct {
	Start       Point
	StartRadius float32
	End         Point
	EndRadius   float32
	Stops       []Stop
}

// NewHorizontalEvenlySpacedGradient creates a new gradient with the specified colors evenly spread across the whole
// range.
func NewHorizontalEvenlySpacedGradient(colors ...ColorProvider) *Gradient {
	return NewEvenlySpacedGradient(Point{}, Point{X: 1}, 0, 0, colors...)
}

// NewVerticalEvenlySpacedGradient creates a new gradient with the specified colors evenly spread across the whole
// range.
func NewVerticalEvenlySpacedGradient(colors ...ColorProvider) *Gradient {
	return NewEvenlySpacedGradient(Point{}, Point{Y: 1}, 0, 0, colors...)
}

// NewEvenlySpacedGradient creates a new gradient with the specified colors evenly spread across the whole range. start
// and end should hold values from 0 to 1, representing the percentage position within the area that will be filled.
func NewEvenlySpacedGradient(start, end Point, startRadius, endRadius float32, colors ...ColorProvider) *Gradient {
	gradient := &Gradient{
		Start:       start,
		StartRadius: startRadius,
		End:         end,
		EndRadius:   endRadius,
		Stops:       make([]Stop, len(colors)),
	}
	switch len(colors) {
	case 0:
	case 1:
		gradient.Stops[0].Color = colors[0]
	case 2:
		gradient.Stops[0].Color = colors[0]
		gradient.Stops[1].Color = colors[1]
		gradient.Stops[1].Location = 1
	default:
		step := 1 / float32(len(colors)-1)
		var location float32
		for i, color := range colors {
			gradient.Stops[i].Color = color
			gradient.Stops[i].Location = location
			if i < len(colors)-1 {
				location += step
			} else {
				location = 1
			}
		}
	}
	return gradient
}

// Paint returns a Paint for this Gradient.
func (g *Gradient) Paint(_ *Canvas, rect Rect, style PaintStyle) *Paint {
	paint := NewPaint()
	paint.SetStyle(style)
	paint.SetColor(Black)
	colors := make([]Color, len(g.Stops))
	colorPos := make([]float32, len(g.Stops))
	for i := range g.Stops {
		colors[i] = g.Stops[i].Color.GetColor()
		colorPos[i] = g.Stops[i].Location
	}
	start := Point{
		X: rect.X + rect.Width*g.Start.X,
		Y: rect.Y + rect.Height*g.Start.Y,
	}
	end := Point{
		X: rect.X + rect.Width*g.End.X,
		Y: rect.Y + rect.Height*g.End.Y,
	}
	var shader *Shader
	if g.StartRadius > 0 && g.EndRadius > 0 {
		shader = New2PtConicalGradientShader(start, end, g.StartRadius, g.EndRadius, colors, colorPos, TileModeClamp,
			geom.NewIdentityMatrix2D[float32]())
	} else {
		shader = NewLinearGradientShader(start, end, colors, colorPos, TileModeClamp,
			geom.NewIdentityMatrix2D[float32]())
	}
	paint.SetShader(shader)
	return paint
}

// Reversed creates a copy of the current Gradient and inverts the locations of each color stop in that copy.
func (g *Gradient) Reversed() *Gradient {
	other := *g
	other.Stops = make([]Stop, len(g.Stops))
	for i, stop := range g.Stops {
		stop.Location = 1 - stop.Location
		other.Stops[i] = stop
	}
	return &other
}
