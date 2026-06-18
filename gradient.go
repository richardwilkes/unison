// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/tilemode"
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

// Stops is a slice of Stop values.
type Stops []Stop

// NewEvenlySpacedGradientStopsForColors creates a slice of Stops with the specified colors evenly spread across the
// whole range. The first Stop will have a Location of 0, the last Stop will have a Location of 1, and any Stops in
// between will be evenly spaced between those two values.
func NewEvenlySpacedGradientStopsForColors(colors ...ColorProvider) Stops {
	if len(colors) == 0 {
		return nil
	}
	stops := make(Stops, len(colors))
	switch len(colors) {
	case 1:
		stops[0].Color = colors[0]
	case 2:
		stops[0].Color = colors[0]
		stops[1].Color = colors[1]
		stops[1].Location = 1
	default:
		step := 1 / float32(len(colors)-1)
		var location float32
		for i, color := range colors {
			stops[i].Color = color
			stops[i].Location = location
			if i < len(colors)-1 {
				location += step
			} else {
				location = 1
			}
		}
	}
	return stops
}

// Reverse inverts the locations of each stop, then sorts them.
func (s Stops) Reverse() {
	for i := range s {
		s[i].Location = 1 - s[i].Location
	}
	s.Sort()
}

// Sort the stops by their location.
func (s Stops) Sort() {
	slices.SortStableFunc(s, func(a, b Stop) int {
		return cmp.Compare(a.Location, b.Location)
	})
}

// StartEnd holds a start and end value.
type StartEnd struct {
	Start float32
	End   float32
}

// Gradient defines a smooth transition between colors across an area.
type Gradient struct {
	Stops     Stops
	StartPt   geom.Point  // Values in the range 0 to 1; used as the center for radial and sweep gradients
	EndPt     geom.Point  // Values in the range 0 to 1; unused by radial and sweep gradients
	Radius    StartEnd    // Values in pixels; unused by linear and sweep gradients; .End unused by radial gradients
	Angle     StartEnd    // Values in degrees; unused by linear, radial, and conical gradients
	Transform geom.Matrix // An empty matrix is treated as an identity matrix
	Kind      gradienttype.Enum
	TileMode  tilemode.Enum
}

// Clone creates a copy of this Gradient.
func (g *Gradient) Clone() *Gradient {
	clone := *g
	clone.Stops = make(Stops, len(g.Stops))
	copy(clone.Stops, g.Stops)
	return &clone
}

// Paint returns a Paint for this Gradient.
func (g *Gradient) Paint(_ *Canvas, rect geom.Rect, style paintstyle.Enum) *Paint {
	p := NewPaint()
	p.SetStyle(style)
	switch len(g.Stops) {
	case 0:
		p.SetColor(Black)
		return p
	case 1:
		p.SetColor(g.Stops[0].Color.GetColor())
		return p
	}
	c := make([]Color, len(g.Stops))
	locs := make([]float32, len(g.Stops))
	for i := range g.Stops {
		c[i] = g.Stops[i].Color.GetColor()
		locs[i] = g.Stops[i].Location
	}
	if g.Transform == (geom.Matrix{}) {
		g.Transform = geom.NewIdentityMatrix()
	}
	var shader *Shader
	switch g.Kind {
	case gradienttype.Linear:
		start := geom.NewPoint(rect.X+rect.Width*g.StartPt.X, rect.Y+rect.Height*g.StartPt.Y)
		end := geom.NewPoint(rect.X+rect.Width*g.EndPt.X, rect.Y+rect.Height*g.EndPt.Y)
		shader = NewLinearGradientShader(start, end, c, locs, g.TileMode, g.Transform)
	case gradienttype.Radial:
		center := geom.NewPoint(rect.X+rect.Width*g.StartPt.X, rect.Y+rect.Height*g.StartPt.Y)
		shader = NewRadialGradientShader(center, g.Radius.Start, c, locs, g.TileMode, g.Transform)
	case gradienttype.Sweep:
		center := geom.NewPoint(rect.X+rect.Width*g.StartPt.X, rect.Y+rect.Height*g.StartPt.Y)
		shader = NewSweepGradientShader(center, g.Angle.Start, g.Angle.End, c, locs, g.TileMode, g.Transform)
	case gradienttype.Conical:
		start := geom.NewPoint(rect.X+rect.Width*g.StartPt.X, rect.Y+rect.Height*g.StartPt.Y)
		end := geom.NewPoint(rect.X+rect.Width*g.EndPt.X, rect.Y+rect.Height*g.EndPt.Y)
		shader = New2PtConicalGradientShader(start, end, g.Radius.Start, g.Radius.End, c, locs, g.TileMode, g.Transform)
	default:
		panic(fmt.Sprintf("unknown gradient type: %v", g.Kind))
	}
	p.SetShader(shader)
	shader.Dispose()
	return p
}
