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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/gradienttype"
)

func TestSVGViewBoxAndSizes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		content       string
		size          geom.Size
		suggestedSize geom.Size
		aspectRatio   float32
	}{
		{
			name:          "viewBox only",
			content:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 16"><path d="M0 0 L10 0 L10 10 Z"/></svg>`,
			size:          geom.NewSize(24, 16),
			suggestedSize: geom.NewSize(24, 16), // falls back to viewBox when width/height absent
			aspectRatio:   1.5,
		},
		{
			name:          "viewBox with width and height",
			content:       `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50" width="200" height="100"><path d="M0 0 L1 1 Z"/></svg>`,
			size:          geom.NewSize(100, 50), // Size reports the viewBox, not the suggested size
			suggestedSize: geom.NewSize(200, 100),
			aspectRatio:   2,
		},
		{
			name:          "width and height without viewBox",
			content:       `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20"><path d="M0 0 L1 1 Z"/></svg>`,
			size:          geom.NewSize(40, 20), // viewBox derived from width/height
			suggestedSize: geom.NewSize(40, 20),
			aspectRatio:   2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			svg, err := NewSVGFromContentString(tc.content)
			c.NoError(err)
			c.NotNil(svg)
			c.Equal(tc.size, svg.Size())
			c.Equal(tc.suggestedSize, svg.SuggestedSize())
			c.Equal(tc.aspectRatio, svg.AspectRatio())
		})
	}
}

func TestSVGShapeCounting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		paths   int
	}{
		{
			name:    "single path",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M0 0 L10 10 Z"/></svg>`,
			paths:   1,
		},
		{
			name: "multiple shapes",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">` +
				`<path d="M0 0 L10 10 Z"/>` +
				`<rect x="1" y="1" width="4" height="4"/>` +
				`<circle cx="5" cy="5" r="2"/>` +
				`</svg>`,
			paths: 3,
		},
		{
			name:    "no shapes",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"></svg>`,
			paths:   0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			svg, err := NewSVGFromContentString(tc.content)
			c.NoError(err)
			c.Equal(tc.paths, len(svg.paths))
		})
	}
}

func TestSVGParseErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "not xml", content: "this is not svg"},
		{name: "viewBox wrong arity", content: `<svg viewBox="0 0 10"><path d="M0 0 Z"/></svg>`},
		{name: "malformed path command", content: `<svg viewBox="0 0 10 10"><path d="Q junk"/></svg>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			svg, err := NewSVGFromContentString(tc.content)
			c.HasError(err)
			c.Nil(svg)
		})
	}
}

func TestSVGGradientDefaults(t *testing.T) {
	const stops = `<stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/>`
	near := func(c check.Checker, expected, actual float32, what string) {
		c.True(xmath.Abs(expected-actual) < 0.0001, "%s: expected %v, got %v", what, expected, actual)
	}
	for _, tc := range []struct {
		name        string
		gradient    string
		kind        gradienttype.Enum
		start       geom.Point
		end         geom.Point
		radiusStart float32
		radiusEnd   float32
	}{
		{
			// cx, cy and r default to 50%; fx and fy default to cx and cy; fr defaults to 0, yielding a plain radial
			// gradient centered in the box, not a conical gradient.
			name:        "radial with all defaults",
			gradient:    `<radialGradient id="g">` + stops + `</radialGradient>`,
			kind:        gradienttype.Radial,
			start:       geom.NewPoint(0.5, 0.5),
			end:         geom.NewPoint(0.5, 0.5),
			radiusStart: 50,
		},
		{
			name:        "radial with focal point matching center",
			gradient:    `<radialGradient id="g" fx="50%" fy="50%">` + stops + `</radialGradient>`,
			kind:        gradienttype.Radial,
			start:       geom.NewPoint(0.5, 0.5),
			end:         geom.NewPoint(0.5, 0.5),
			radiusStart: 50,
		},
		{
			// An offset focal point without fr produces a two-point conical gradient with a zero start radius.
			name:        "radial with offset focal point",
			gradient:    `<radialGradient id="g" fx="30%" fy="40%">` + stops + `</radialGradient>`,
			kind:        gradienttype.Conical,
			start:       geom.NewPoint(0.3, 0.4),
			end:         geom.NewPoint(0.5, 0.5),
			radiusStart: 0,
			radiusEnd:   50,
		},
		{
			// fy defaults to cy when only fx is given.
			name:        "radial with fx only",
			gradient:    `<radialGradient id="g" fx="30%">` + stops + `</radialGradient>`,
			kind:        gradienttype.Conical,
			start:       geom.NewPoint(0.3, 0.5),
			end:         geom.NewPoint(0.5, 0.5),
			radiusStart: 0,
			radiusEnd:   50,
		},
		{
			name:        "radial with focal radius",
			gradient:    `<radialGradient id="g" fr="10%">` + stops + `</radialGradient>`,
			kind:        gradienttype.Conical,
			start:       geom.NewPoint(0.5, 0.5),
			end:         geom.NewPoint(0.5, 0.5),
			radiusStart: 10,
			radiusEnd:   50,
		},
		{
			// x1, y1 and y2 default to 0%, x2 to 100%, yielding a horizontal left-to-right gradient.
			name:     "linear with defaults",
			gradient: `<linearGradient id="g">` + stops + `</linearGradient>`,
			kind:     gradienttype.Linear,
			start:    geom.NewPoint(0, 0),
			end:      geom.NewPoint(1, 0),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
				`<defs>` + tc.gradient + `</defs>` +
				`<rect width="100" height="100" fill="url(#g)"/></svg>`)
			c.NoError(err)
			c.NotNil(svg)
			c.Equal(1, len(svg.paths))
			grad, ok := svg.paths[0].fillInk.(*Gradient)
			c.True(ok, "fillInk should be a *Gradient, got %T", svg.paths[0].fillInk)
			c.Equal(tc.kind, grad.Kind)
			near(c, tc.start.X, grad.StartPt.X, "StartPt.X")
			near(c, tc.start.Y, grad.StartPt.Y, "StartPt.Y")
			near(c, tc.end.X, grad.EndPt.X, "EndPt.X")
			near(c, tc.end.Y, grad.EndPt.Y, "EndPt.Y")
			near(c, tc.radiusStart, grad.Radius.Start, "Radius.Start")
			near(c, tc.radiusEnd, grad.Radius.End, "Radius.End")
		})
	}
}

func TestSVGOffsetToCenterWithinScaledSize(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M0 0 L10 10 Z"/></svg>`)
	c.NoError(err)

	// A square SVG scaled to fit a wider rectangle: the scale is limited by height, so the image is centered
	// horizontally and flush vertically.
	c.Equal(geom.NewPoint(20, 0), svg.OffsetToCenterWithinScaledSize(geom.NewSize(60, 20)))

	// Same size means no scaling and no offset.
	c.Equal(geom.NewPoint(0, 0), svg.OffsetToCenterWithinScaledSize(geom.NewSize(10, 10)))

	// Taller-than-wide target: scale limited by width, centered vertically.
	c.Equal(geom.NewPoint(0, 15), svg.OffsetToCenterWithinScaledSize(geom.NewSize(10, 40)))
}
