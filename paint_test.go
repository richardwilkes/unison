// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison_test

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/tilemode"
)

func TestPaintEquivalent(t *testing.T) {
	c := check.New(t)

	// nil handling
	var nilPaint *unison.Paint
	c.True(nilPaint.Equivalent(nil))
	c.False(nilPaint.Equivalent(unison.NewPaint()))
	c.False(unison.NewPaint().Equivalent(nil))

	// Two independently-constructed default paints are equivalent (content, not pointer, identity).
	c.True(unison.NewPaint().Equivalent(unison.NewPaint()))

	// A paint is equivalent to itself and to a clone of itself.
	p := unison.NewPaint()
	p.SetColor(unison.Red)
	p.SetStrokeWidth(2)
	c.True(p.Equivalent(p))
	c.True(p.Clone().Equivalent(p))
	c.True(p.Equivalent(p.Clone()))

	// Differing in a single scalar field makes them not equivalent.
	q := p.Clone()
	q.SetColor(unison.Blue)
	c.False(p.Equivalent(q))

	q = p.Clone()
	q.SetStrokeWidth(3)
	c.False(p.Equivalent(q))

	q = p.Clone()
	q.SetStyle(paintstyle.Stroke)
	c.False(p.Equivalent(q))
}

func TestPaintEquivalentGradientShaders(t *testing.T) {
	c := check.New(t)

	// Gradient shaders carry non-comparable state (a []Color of stops); comparing two paints that hold
	// different gradient shaders must neither panic nor report them as equivalent.
	makeGradientPaint := func(colors ...unison.Color) *unison.Paint {
		p := unison.NewPaint()
		p.SetShader(unison.NewLinearGradientShader(geom.Point{}, geom.Point{X: 10, Y: 10}, colors, nil,
			tilemode.Clamp, geom.NewIdentityMatrix()))
		return p
	}

	p := makeGradientPaint(unison.Red, unison.Blue)
	q := makeGradientPaint(unison.Green, unison.White)
	c.False(p.Equivalent(q)) // must not panic

	// A paint sharing the very same shader instance (via Clone) is still equivalent.
	c.True(p.Clone().Equivalent(p))

	// A gradient-bearing paint is not equivalent to a plain default paint.
	c.False(p.Equivalent(unison.NewPaint()))
}
