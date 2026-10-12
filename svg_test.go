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
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/richardwilkes/canvas/raster"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/paintstyle"
	"github.com/richardwilkes/unison/enums/tilemode"
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
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path d="M0 0 L10 10 Z"/></svg>`,
	)
	c.NoError(err)

	// A square SVG scaled to fit a wider rectangle: the scale is limited by height, so the image is centered
	// horizontally and flush vertically.
	c.Equal(geom.NewPoint(20, 0), svg.OffsetToCenterWithinScaledSize(geom.NewSize(60, 20)))

	// Same size means no scaling and no offset.
	c.Equal(geom.NewPoint(0, 0), svg.OffsetToCenterWithinScaledSize(geom.NewSize(10, 10)))

	// Taller-than-wide target: scale limited by width, centered vertically.
	c.Equal(geom.NewPoint(0, 15), svg.OffsetToCenterWithinScaledSize(geom.NewSize(10, 40)))
}

// TestSVGPolylineTwoPoints verifies that a minimal, legal two-point polyline (and polygon) produces geometry, since the
// handler previously required more than two points and silently dropped the shape.
func TestSVGPolylineTwoPoints(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><polyline points="1,2 9,8"/></svg>`,
	)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.Equal(geom.NewRect(1, 2, 8, 6), svg.paths[0].path.ComputeTightBounds())

	svg, err = NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><polygon points="1,2 9,8"/></svg>`,
	)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.Equal(geom.NewRect(1, 2, 8, 6), svg.paths[0].path.ComputeTightBounds())
}

// TestSVGGradientForwardReference verifies that a fill or stroke may reference a gradient defined later in the
// document, which is legal SVG and previously failed the entire parse.
func TestSVGGradientForwardReference(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<rect x="0" y="0" width="10" height="10" fill="url(#grad)"/>
<linearGradient id="grad"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></linearGradient>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	g, ok := svg.paths[0].fillInk.(*Gradient)
	c.True(ok, "fill should resolve to a gradient, got %T", svg.paths[0].fillInk)
	c.Equal(2, len(g.Stops))

	// A backward reference must keep working.
	svg, err = NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<linearGradient id="grad"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></linearGradient>
<rect x="0" y="0" width="10" height="10" fill="url(#grad)"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	_, ok = svg.paths[0].fillInk.(*Gradient)
	c.True(ok, "fill should resolve to a gradient, got %T", svg.paths[0].fillInk)

	// A reference to an id that never appears must still fail.
	_, err = NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<rect x="0" y="0" width="10" height="10" fill="url(#missing)"/>
</svg>`)
	c.HasError(err)
}

// TestSVGPathUppercaseExponent verifies that path data accepts numbers with uppercase exponents (and exponents with an
// explicit sign), which the number scanner previously mis-parsed.
func TestSVGPathUppercaseExponent(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path d="M0 0 L1E1 1e1 L2E+1 1e-0 Z"/></svg>`,
	)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.Equal(geom.NewRect(0, 0, 20, 10), svg.paths[0].path.ComputeTightBounds())
}

// TestSVGTransformListOrder verifies that a multi-op transform attribute composes left-to-right as the spec requires,
// since the parser previously composed the ops in reverse order. For "translate(10,0) rotate(90)" the rotation must be
// applied to geometry first: the unit rect maps to (9,0)-(10,1), not the reversed order's (-1,10)-(0,11).
func TestSVGTransformListOrder(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
<rect x="0" y="0" width="1" height="1" transform="translate(10,0) rotate(90)"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	bounds := svg.paths[0].path.ComputeTightBounds()
	c.True(rectsNearlyEqual(geom.NewRect(9, 0, 1, 1), bounds), "got %v", bounds)

	svg, err = NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
<rect x="0" y="0" width="1" height="1" transform="translate(5,5) scale(2)"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	bounds = svg.paths[0].path.ComputeTightBounds()
	c.True(rectsNearlyEqual(geom.NewRect(5, 5, 2, 2), bounds), "got %v", bounds)
}

// TestSVGTransformListCommaSeparated verifies that the comma-wsp separator the spec permits between the operations of
// a transform list parses. The parser splits on ")" and used to hand the leftover leading comma to the operation name
// matcher, so a single comma anywhere in the list rejected the entire document.
func TestSVGTransformListCommaSeparated(t *testing.T) {
	c := check.New(t)
	for _, transform := range []string{
		"translate(5,5), scale(2)",
		"translate(5,5),scale(2)",
		"translate(5,5) , scale(2)",
		"translate(5 5)  scale(2)",
	} {
		svg, err := NewSVGFromContentString(fmt.Sprintf(
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
<rect x="0" y="0" width="1" height="1" transform="%s"/>
</svg>`, transform,
		))
		c.NoError(err, "transform %q", transform)
		if err != nil {
			continue
		}
		c.Equal(1, len(svg.paths), "transform %q", transform)
		bounds := svg.paths[0].path.ComputeTightBounds()
		c.True(rectsNearlyEqual(geom.NewRect(5, 5, 2, 2), bounds), "transform %q got %v", transform, bounds)
	}

	// A genuinely unknown operation must still be rejected.
	_, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
<rect x="0" y="0" width="1" height="1" transform="translate(5,5) bogus(2)"/>
</svg>`)
	c.HasError(err)
}

// rectsNearlyEqual compares rects with a small tolerance, since transforms like rotate(90) introduce float32 rounding
// noise (e.g. cos 90° is not exactly zero).
func rectsNearlyEqual(a, b geom.Rect) bool {
	const tolerance = 1e-5
	near := func(x, y float32) bool { return math.Abs(float64(x-y)) < tolerance }
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.Width, b.Width) && near(a.Height, b.Height)
}

// TestSVGCompactArcFlags verifies that arc commands whose single-digit flags abut the following number without a
// separator (the compact form emitted by svgo and similar minifiers) parse identically to the fully separated form,
// since the number tokenizer previously consumed the run as one number and rejected the whole document.
func TestSVGCompactArcFlags(t *testing.T) {
	c := check.New(t)
	const doc = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path d="%s"/></svg>`
	for _, one := range []struct{ compact, expanded string }{
		{"M0 0a4 4 0 014 4z", "M0 0a4 4 0 0 1 4 4z"},
		{"M0 0a4 4 0 104 4z", "M0 0a4 4 0 1 0 4 4z"},
		{"M0 0a4 4 0 11-4 4z", "M0 0a4 4 0 1 1 -4 4z"},
		{"M0 0a4 4 0 014 4 4 4 0 014 4z", "M0 0a4 4 0 0 1 4 4 4 4 0 0 1 4 4z"},
		{"M2 2A4 4 0 01.5.5z", "M2 2A4 4 0 0 1 0.5 0.5z"},
	} {
		compact, err := NewSVGFromContentString(fmt.Sprintf(doc, one.compact))
		c.NoError(err, "compact form %q", one.compact)
		expanded, err := NewSVGFromContentString(fmt.Sprintf(doc, one.expanded))
		c.NoError(err, "expanded form %q", one.expanded)
		c.Equal(1, len(compact.paths), "compact form %q", one.compact)
		c.Equal(1, len(expanded.paths), "expanded form %q", one.expanded)
		c.Equal(expanded.paths[0].path.ComputeTightBounds(), compact.paths[0].path.ComputeTightBounds(),
			"compact form %q", one.compact)
	}
}

// TestSVGObjectBoundingBoxGradientFractions verifies that objectBoundingBox gradient coordinates written as plain
// numbers are treated as fractions of the bounding box, equivalent to percentages, since they were previously passed
// through as user-space pixels and collapsed the gradient into a sliver.
func TestSVGObjectBoundingBoxGradientFractions(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<linearGradient id="grad" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></linearGradient>
<rect x="2" y="2" width="6" height="6" fill="url(#grad)"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	g, ok := svg.paths[0].fillInk.(*Gradient)
	c.True(ok, "fill should resolve to a gradient, got %T", svg.paths[0].fillInk)
	// The gradient must span the shape's bounding box vertically: from the box's top edge to its bottom edge, both
	// expressed as fractions of the viewBox.
	c.Equal(geom.NewPoint(0.2, 0.2), g.StartPt)
	c.Equal(geom.NewPoint(0.2, 0.8), g.EndPt)

	// A fraction and the equivalent percentage must resolve identically, for both linear and radial gradients.
	fromFractions, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<radialGradient id="grad" cx="0.5" cy="0.5" r="0.25"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></radialGradient>
<rect x="2" y="2" width="6" height="6" fill="url(#grad)"/>
</svg>`)
	c.NoError(err)
	fromPercents, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<radialGradient id="grad" cx="50%" cy="50%" r="25%"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></radialGradient>
<rect x="2" y="2" width="6" height="6" fill="url(#grad)"/>
</svg>`)
	c.NoError(err)
	gf, ok := fromFractions.paths[0].fillInk.(*Gradient)
	c.True(ok, "fill should resolve to a gradient, got %T", fromFractions.paths[0].fillInk)
	gp, ok := fromPercents.paths[0].fillInk.(*Gradient)
	c.True(ok, "fill should resolve to a gradient, got %T", fromPercents.paths[0].fillInk)
	c.Equal(gp.StartPt, gf.StartPt)
	c.Equal(gp.EndPt, gf.EndPt)
	c.Equal(gp.Radius, gf.Radius)
}

// TestSVGLinearGradientDefaultDirection verifies that a linearGradient with no coordinate attributes is horizontal, per
// the spec defaults x1=0% y1=0% x2=100% y2=0%, since y2 was previously seeded as 100% and rendered diagonally.
func TestSVGLinearGradientDefaultDirection(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<linearGradient id="grad"><stop offset="0" stop-color="#ff0000"/><stop offset="1" stop-color="#0000ff"/></linearGradient>
<rect x="0" y="0" width="10" height="10" fill="url(#grad)"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	g, ok := svg.paths[0].fillInk.(*Gradient)
	c.True(ok, "fill should resolve to a gradient, got %T", svg.paths[0].fillInk)
	c.Equal(geom.NewPoint(0, 0), g.StartPt)
	c.Equal(geom.NewPoint(1, 0), g.EndPt)
}

// TestSVGMaskMultiplePathsUnion verifies that multiple shapes within one mask reveal the union of their areas, since
// they were previously intersected — which for disjoint shapes produced an empty mask that was silently dropped.
func TestSVGMaskMultiplePathsUnion(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<mask id="m"><rect x="0" y="0" width="4" height="4" fill="#fff"/><rect x="6" y="6" width="3" height="3" fill="#fff"/></mask>
<rect x="0" y="0" width="10" height="10" mask="url(#m)"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.NotNil(svg.paths[0].mask)
	c.Equal(geom.NewRect(0, 0, 9, 9), svg.paths[0].mask.ComputeTightBounds())

	// Distinct mask references reached through nesting must still intersect: a group mask and an element mask each
	// clip, so the element is limited to the overlap of the two.
	svg, err = NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<mask id="a"><rect x="0" y="0" width="6" height="6" fill="#fff"/></mask>
<mask id="b"><rect x="4" y="4" width="6" height="6" fill="#fff"/></mask>
<g mask="url(#a)"><rect x="0" y="0" width="10" height="10" mask="url(#b)"/></g>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.NotNil(svg.paths[0].mask)
	c.Equal(geom.NewRect(4, 4, 2, 2), svg.paths[0].mask.ComputeTightBounds())
}

// TestSVGUseOfDefNestedInGroup verifies that an id-bearing element nested inside an id-bearing group within defs can be
// used without corrupting the style stack. Previously the def list was split at the nested id, leaving the group's endg
// sentinel in the wrong def entry, so each use popped a style frame that was never pushed — corrupting inherited styles
// and panicking on the second use.
func TestSVGUseOfDefNestedInGroup(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<defs><g id="outer" fill="#ff0000"><rect id="inner" x="1" y="1" width="2" height="2"/></g></defs>
<use href="#inner"/>
<use href="#inner" x="4"/>
<use href="#outer" y="4"/>
<rect x="0" y="0" width="1" height="1"/>
</svg>`)
	c.NoError(err)
	c.Equal(4, len(svg.paths))
	// Using the nested rect directly draws it without the group's fill.
	c.Equal(geom.NewRect(1, 1, 2, 2), svg.paths[0].path.ComputeTightBounds())
	c.Equal(Black, svg.paths[0].fillInk)
	c.Equal(geom.NewRect(5, 1, 2, 2), svg.paths[1].path.ComputeTightBounds())
	// Using the group draws the nested rect with the group's fill.
	c.Equal(geom.NewRect(1, 5, 2, 2), svg.paths[2].path.ComputeTightBounds())
	c.Equal(Red, svg.paths[2].fillInk)
	// The trailing rect must still see the document's default style, proving the stack survived the uses intact.
	c.Equal(geom.NewRect(0, 0, 1, 1), svg.paths[3].path.ComputeTightBounds())
	c.Equal(Black, svg.paths[3].fillInk)
}

// TestSVGNestedUseKeepsOuterOffset verifies that a use reached through another use's def combines both x/y offsets and
// that the offsets are restored afterward. Previously the inner use replaced the outer offset and then reset it to
// zero.
func TestSVGNestedUseKeepsOuterOffset(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40">
<defs><rect id="leaf" width="2" height="2"/><use id="mid" href="#leaf" x="3" y="3"/></defs>
<use href="#mid" x="10" y="20"/>
<rect x="0" y="0" width="1" height="1"/>
</svg>`)
	c.NoError(err)
	c.Equal(2, len(svg.paths))
	c.Equal(geom.NewRect(13, 23, 2, 2), svg.paths[0].path.ComputeTightBounds())
	c.Equal(geom.NewRect(0, 0, 1, 1), svg.paths[1].path.ComputeTightBounds())
}

// TestSVGRecursiveUseIsRejected verifies that a def which reaches itself, directly or through other defs, is reported
// as an error rather than recursing until the stack overflows, which would be a fatal, unrecoverable process death.
func TestSVGRecursiveUseIsRejected(t *testing.T) {
	c := check.New(t)

	// Direct self-reference.
	_, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<defs><g id="a"><use href="#a"/></g></defs>
<use href="#a"/>
</svg>`)
	c.HasError(err)
	c.Contains(err.Error(), "recursive")

	// Mutual reference through a second def.
	_, err = NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<defs><g id="a"><use href="#b"/></g><g id="b"><use href="#a"/></g></defs>
<use href="#a"/>
</svg>`)
	c.HasError(err)
	c.Contains(err.Error(), "recursive")

	// A chain of distinct ids can't cycle, but can still nest arbitrarily deep, so it is capped too.
	var buf strings.Builder
	buf.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><defs><rect id="u0" width="2" height="2"/>`)
	depth := svgMaxUseDepth + 8
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&buf, `<g id="u%d"><use href="#u%d"/></g>`, i, i-1)
	}
	fmt.Fprintf(&buf, `</defs><use href="#u%d"/></svg>`, depth)
	_, err = NewSVGFromContentString(buf.String())
	c.HasError(err)
	c.Contains(err.Error(), "nest")
}

// TestSVGRepeatedNonRecursiveUseStillWorks verifies that the recursion guard tracks only the defs currently being
// expanded, so the same def may be used repeatedly, both in sequence and from sibling positions within another def.
func TestSVGRepeatedNonRecursiveUseStillWorks(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40">
<defs><rect id="leaf" width="2" height="2"/><g id="pair"><use href="#leaf"/><use href="#leaf" x="5"/></g></defs>
<use href="#pair"/>
<use href="#pair" y="10"/>
<use href="#leaf" x="20" y="20"/>
</svg>`)
	c.NoError(err)
	c.Equal(5, len(svg.paths))
	c.Equal(geom.NewRect(0, 0, 2, 2), svg.paths[0].path.ComputeTightBounds())
	c.Equal(geom.NewRect(5, 0, 2, 2), svg.paths[1].path.ComputeTightBounds())
	c.Equal(geom.NewRect(0, 10, 2, 2), svg.paths[2].path.ComputeTightBounds())
	c.Equal(geom.NewRect(5, 10, 2, 2), svg.paths[3].path.ComputeTightBounds())
	c.Equal(geom.NewRect(20, 20, 2, 2), svg.paths[4].path.ComputeTightBounds())
}

// TestSVGPercentStrokeWidthUsesDiagonal verifies that a percentage stroke-width resolves against the normalized
// diagonal the spec requires for lengths that lie along neither axis. It was previously resolved against the viewport
// width, giving the wrong width on any non-square viewBox.
func TestSVGPercentStrokeWidthUsesDiagonal(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">
<path d="M0 0 L10 10" stroke="#000000" stroke-width="10%"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	// sqrt(100² + 50²) / sqrt(2) is 79.0569..., a tenth of which is the resolved width. The viewport width would
	// have given 10.
	nearlyEqual(c, xmath.Sqrt(100*100+50*50)/xmath.Sqrt(2)/10, svg.paths[0].strokeWidth)

	// On a square viewBox the diagonal reduces to the width, so both readings agree.
	svg, err = NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<path d="M0 0 L10 10" stroke="#000000" stroke-width="10%"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	nearlyEqual(c, 10, svg.paths[0].strokeWidth)
}

// TestSVGClosePathAfterClosePath verifies that a closepath following drawing commands that were themselves issued
// after an earlier closepath closes the subpath those commands implicitly restarted. The parser marked itself as being
// in a subpath only on moveto, so the second Z was silently dropped and left that subpath open.
func TestSVGClosePathAfterClosePath(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30"><path d="M0 0 L10 0 L10 10 Z L20 20 Z"/></svg>`,
	)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.True(svg.paths[0].path.path.IsLastContourClosed(), "the second subpath should be closed")

	// The restarted subpath begins at the closed subpath's initial point, so the result must match the form that
	// names that point explicitly.
	explicit, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30"><path d="M0 0 L10 0 L10 10 Z M0 0 L20 20 Z"/></svg>`,
	)
	c.NoError(err)
	c.Equal(1, len(explicit.paths))
	c.Equal(explicit.paths[0].path.path.CountVerbs(), svg.paths[0].path.path.CountVerbs())
	c.Equal(explicit.paths[0].path.ComputeTightBounds(), svg.paths[0].path.ComputeTightBounds())

	// A closepath with nothing drawn since the previous one must remain a no-op.
	svg, err = NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 30 30"><path d="M0 0 L10 0 L10 10 Z Z"/></svg>`,
	)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.Equal(4, svg.paths[0].path.path.CountVerbs())
	c.Equal(geom.NewRect(0, 0, 10, 10), svg.paths[0].path.ComputeTightBounds())
}

// TestSVGUseOffsetIsOutsideDefTransform verifies that a use element's x/y offset is applied outside the referenced
// def's own transform, matching the translate(x,y) wrapper the spec defines. The offset was previously added to the
// raw coordinates, so a def carrying a scale or rotation transformed the offset along with the geometry.
func TestSVGUseOffsetIsOutsideDefTransform(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<defs><rect id="r" width="10" height="10" transform="scale(2)"/></defs>
<use href="#r" x="5" y="5"/>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	// scale(2) makes the rect 20x20 and the offset is then added unscaled. Scaling the offset too would place it at
	// (10, 10).
	bounds := svg.paths[0].path.ComputeTightBounds()
	c.True(rectsNearlyEqual(geom.NewRect(5, 5, 20, 20), bounds), "got %v", bounds)

	// The same thing spelled out as the group the spec says a use is equivalent to.
	equivalent, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<g transform="translate(5,5)"><rect width="10" height="10" transform="scale(2)"/></g>
</svg>`)
	c.NoError(err)
	c.Equal(1, len(equivalent.paths))
	c.Equal(equivalent.paths[0].path.ComputeTightBounds(), bounds)
}

// TestSVGUseForwardReference verifies that a use element may reference a def declared later in the document, which is
// legal SVG that previously aborted the whole parse, and that the postponed expansion still lands at the point in the
// drawing order where the use appeared.
func TestSVGUseForwardReference(t *testing.T) {
	c := check.New(t)
	// Parsing must succeed for any of the assertions below to mean anything, so stop the test rather than
	// dereferencing a nil result.
	parse := func(content string) *SVG {
		t.Helper()
		svg, err := NewSVGFromContentString(content)
		c.NoError(err)
		if svg == nil {
			t.FailNow()
		}
		return svg
	}

	svg := parse(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<use href="#r" x="5" y="5"/>
<rect x="50" y="50" width="4" height="4"/>
<defs><rect id="r" width="10" height="10"/></defs>
</svg>`)
	c.Equal(2, len(svg.paths))
	c.Equal(geom.NewRect(5, 5, 10, 10), svg.paths[0].path.ComputeTightBounds())
	c.Equal(geom.NewRect(50, 50, 4, 4), svg.paths[1].path.ComputeTightBounds())

	// The style inherited at the point of use must survive the postponement.
	svg = parse(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<g fill="#ff0000"><use href="#r"/></g>
<defs><rect id="r" width="10" height="10"/></defs>
</svg>`)
	c.Equal(1, len(svg.paths))
	c.Equal(Red, svg.paths[0].fillInk)

	// A postponed def that reaches a def of its own must resolve too.
	svg = parse(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<use href="#outer"/>
<defs><g id="outer"><use href="#leaf" x="3"/></g><rect id="leaf" width="2" height="2"/></defs>
</svg>`)
	c.Equal(1, len(svg.paths))
	c.Equal(geom.NewRect(3, 0, 2, 2), svg.paths[0].path.ComputeTightBounds())

	// A postponed use inside a mask must still contribute to that mask.
	svg = parse(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20">
<mask id="m"><use href="#box"/></mask>
<rect x="0" y="0" width="20" height="20" mask="url(#m)"/>
<defs><rect id="box" x="2" y="2" width="6" height="6" fill="#ffffff"/></defs>
</svg>`)
	c.Equal(1, len(svg.paths))
	c.NotNil(svg.paths[0].mask)
	c.Equal(geom.NewRect(2, 2, 6, 6), svg.paths[0].mask.ComputeTightBounds())

	// An id that never appears anywhere must still be rejected.
	_, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
<use href="#missing"/>
</svg>`)
	c.HasError(err)
}

// TestSVGUseWithMultiShapeDef verifies that a use element referencing a def containing multiple shapes draws all of
// them, since each shape's geometry was previously discarded when the next one reset the working path.
func TestSVGUseWithMultiShapeDef(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">
<defs><g id="both"><path d="M0 0h4v4h-4z"/><path d="M6 6h3v3h-3z"/></g></defs>
<use href="#both"/>
</svg>`)
	c.NoError(err)
	c.Equal(2, len(svg.paths))
	c.Equal(geom.NewRect(0, 0, 4, 4), svg.paths[0].path.ComputeTightBounds())
	c.Equal(geom.NewRect(6, 6, 3, 3), svg.paths[1].path.ComputeTightBounds())
}

// inkColBounds returns the leftmost and rightmost columns containing any non-transparent pixel within the row range
// [minY, maxY), or (-1, -1) if the region is empty.
func inkColBounds(pix *raster.Pixmap, minY, maxY int32) (left, right int32) {
	left = -1
	right = -1
	for x := int32(0); x < pix.Width; x++ {
		for y := minY; y < maxY; y++ {
			if pix.Pix[int(y)*int(pix.RowPixels)+int(x)]>>24 != 0 {
				if left == -1 {
					left = x
				}
				right = x
				break
			}
		}
	}
	return left, right
}

// checkInkBounds asserts that the ink drawn into pix exactly covers rect, allowing one pixel of slack on each edge for
// antialiasing.
func checkInkBounds(c check.Checker, pix *raster.Pixmap, rect geom.Rect) {
	top, bottom := inkRowBounds(pix, 0, pix.Width)
	left, right := inkColBounds(pix, 0, pix.Height)
	c.NotEqual(int32(-1), top, "expected some ink to be drawn")
	within1 := func(expected, actual int32, what string) {
		diff := expected - actual
		if diff < 0 {
			diff = -diff
		}
		c.True(diff <= 1, "%s: expected %d, got %d", what, expected, actual)
	}
	within1(int32(rect.Y), top, "top")
	within1(int32(rect.Bottom())-1, bottom, "bottom")
	within1(int32(rect.X), left, "left")
	within1(int32(rect.Right())-1, right, "right")
}

func TestSVGDrawInRectStretchesToExactRect(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
	}{
		{
			// A square SVG whose single path covers the entire viewBox.
			name:    "origin viewBox",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`,
		},
		{
			// The same shape, but with a viewBox whose origin is not (0, 0), to verify the origin translation composes
			// with the stretch correctly.
			name:    "offset viewBox",
			content: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="5 -3 10 10"><rect x="5" y="-3" width="10" height="10"/></svg>`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			svg, err := NewSVGFromContentString(tc.content)
			c.NoError(err)

			// Draw into a rect much wider than the SVG's aspect ratio. The content must be stretched non-uniformly to
			// fill exactly this rect, with no leftover centering shift pushing it outside.
			cv, pix := newPixmapCanvas(64, 32)
			rect := geom.NewRect(8, 4, 40, 16)
			svg.DrawInRect(cv, rect, nil, Black.Paint(cv, rect, paintstyle.Fill))
			checkInkBounds(c, pix, rect)
		})
	}
}

func TestSVGDrawInRectPreservingAspectRatioCenters(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`,
	)
	c.NoError(err)

	// A square SVG drawn into a wide rect must be scaled uniformly to the rect height and centered horizontally.
	cv, pix := newPixmapCanvas(64, 24)
	rect := geom.NewRect(0, 0, 60, 20)
	svg.DrawInRectPreservingAspectRatio(cv, rect, nil, Black.Paint(cv, rect, paintstyle.Fill))
	checkInkBounds(c, pix, geom.NewRect(20, 0, 20, 20))
}

// svgTestPixel is an exact, unpremultiplied pixel value read back from a rasterized SVG.
type svgTestPixel struct {
	r, g, b, a uint8
}

var (
	svgTestBlack = svgTestPixel{a: 255}
	svgTestWhite = svgTestPixel{r: 255, g: 255, b: 255, a: 255}
	svgTestRed   = svgTestPixel{r: 255, a: 255}
	svgTestGreen = svgTestPixel{g: 128, a: 255}
)

// svgTestReplacements maps the ink-substitution convention's source colors onto two colors that share no channel value
// with either the originals or with each other, so an anti-aliased blend of the replacements can never accidentally
// reproduce a pure black or pure white pixel.
var svgTestReplacements = map[Color]Ink{Black: Red, White: Green}

// svgRasterizePixels rasterizes the SVG into a square image of the given size entirely on the CPU and returns the set
// of exact pixel values it contains. Working with the set of values present, rather than counts or positions, keeps the
// assertions insensitive to anti-aliasing.
func svgRasterizePixels(c check.Checker, svg *SVG, size int, replacements map[Color]Ink) map[svgTestPixel]bool {
	img, err := NewImageFromDrawing(size, size, 72, func(canvas *Canvas) {
		svg.DrawInRectWithReplacementInks(canvas, geom.NewRect(0, 0, float32(size), float32(size)), nil, replacements)
	})
	c.NoError(err)
	c.NotNil(img)
	defer img.Dispose()
	nrgba, err := img.ToNRGBA()
	c.NoError(err)
	present := make(map[svgTestPixel]bool)
	for y := range size {
		row := nrgba.Pix[y*nrgba.Stride : y*nrgba.Stride+size*4]
		for x := range size {
			present[svgTestPixel{r: row[x*4], g: row[x*4+1], b: row[x*4+2], a: row[x*4+3]}] = true
		}
	}
	return present
}

// svgReplacementTestContent is a single path carrying a black fill and a white stroke, so that the independent lookup
// of the two inks can be observed on one path. The stroke is wide enough that both it and the remaining interior fill
// contain fully covered pixels at the size used below.
const svgReplacementTestContent = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">` +
	`<path fill="#000" stroke="#fff" stroke-width="16" d="M16 16h32v32H16z"/></svg>`

func TestSVGReplaceInk(t *testing.T) {
	c := check.New(t)

	c.Nil(svgReplaceInk(nil, svgTestReplacements), "a nil ink has nothing to replace")
	c.Equal(Ink(Black), svgReplaceInk(Black, nil), "a nil map replaces nothing")
	c.Equal(Ink(Black), svgReplaceInk(Black, map[Color]Ink{}), "an empty map replaces nothing")
	c.Equal(Ink(Red), svgReplaceInk(Black, svgTestReplacements), "a matching color is replaced")
	c.Equal(Ink(Green), svgReplaceInk(White, svgTestReplacements), "each color is looked up independently")
	c.Equal(Ink(Blue), svgReplaceInk(Blue, svgTestReplacements), "an unlisted color is left alone")

	// Inks that aren't plain colors, such as gradients, can't be map keys and must pass through untouched.
	gradient := &Gradient{
		Stops:    NewEvenlySpacedGradientStopsForColors(Black, White),
		EndPt:    geom.NewPoint(1, 1),
		Kind:     gradienttype.Linear,
		TileMode: tilemode.Clamp,
	}
	c.Equal(Ink(gradient), svgReplaceInk(gradient, svgTestReplacements), "a gradient is not a plain color")
}

func TestSVGDrawInRectWithReplacementInks(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(svgReplacementTestContent)
	c.NoError(err)

	const size = 64

	// Control: with no replacements, the source colors must appear untouched.
	present := svgRasterizePixels(c, svg, size, nil)
	c.True(present[svgTestBlack], "the unreplaced fill must produce pure black pixels")
	c.True(present[svgTestWhite], "the unreplaced stroke must produce pure white pixels")

	// With replacements, both inks are substituted and neither original survives. Anti-aliased edges blend the
	// replacement colors, so they can never reproduce the originals either.
	present = svgRasterizePixels(c, svg, size, svgTestReplacements)
	c.True(present[svgTestRed], "the black fill must be replaced by red")
	c.True(present[svgTestGreen], "the white stroke must be replaced by green")
	c.False(present[svgTestBlack], "no pure black pixel may survive the replacement")
	c.False(present[svgTestWhite], "no pure white pixel may survive the replacement")
}

// TestSVGCursorAssetsFollowInkConvention verifies that every cursor asset is drawn purely with pure black and pure
// white, the two colors the cursor foreground and background substitution relies on. Anything painted with some other
// color, or with an implicit fill that the convention doesn't cover, would leave pure black or pure white pixels behind
// after substituting both, or would fail to produce the substituted colors at all.
func TestSVGCursorAssetsFollowInkConvention(t *testing.T) {
	for name, svg := range map[string]*SVG{
		"arrow":                 CursorArrowSVG,
		"hand_closed":           CursorHandClosedSVG,
		"hand_open":             CursorHandOpenSVG,
		"hand_pointing":         CursorHandPointingSVG,
		"move":                  CursorMoveSVG,
		"resize_horizontal":     CursorResizeHorizontalSVG,
		"resize_left_diagonal":  CursorResizeLeftDiagonalSVG,
		"resize_right_diagonal": CursorResizeRightDiagonalSVG,
		"resize_vertical":       CursorResizeVerticalSVG,
		"text":                  CursorTextSVG,
	} {
		t.Run(name, func(t *testing.T) {
			c := check.New(t)

			// Rasterize at the assets' native viewBox size so that even the thinnest stroke has fully covered pixels.
			const size = 256

			present := svgRasterizePixels(c, svg, size, nil)
			c.True(present[svgTestBlack], "the foreground must be drawn with pure black")
			c.True(present[svgTestWhite], "the background must be drawn with pure white")

			present = svgRasterizePixels(c, svg, size, svgTestReplacements)
			c.True(present[svgTestRed], "the foreground must pick up the black replacement")
			c.True(present[svgTestGreen], "the background must pick up the white replacement")
			c.False(present[svgTestBlack], "no pure black pixel may survive the replacement")
			c.False(present[svgTestWhite], "no pure white pixel may survive the replacement")
		})
	}
}
