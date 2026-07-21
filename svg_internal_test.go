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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
)

// TestSVGPolylineTwoPoints verifies that a minimal, legal two-point polyline (and polygon) produces geometry, since the
// handler previously required more than two points and silently dropped the shape.
func TestSVGPolylineTwoPoints(t *testing.T) {
	c := check.New(t)
	svg, err := NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><polyline points="1,2 9,8"/></svg>`)
	c.NoError(err)
	c.Equal(1, len(svg.paths))
	c.Equal(geom.NewRect(1, 2, 8, 6), svg.paths[0].path.ComputeTightBounds())

	svg, err = NewSVGFromContentString(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><polygon points="1,2 9,8"/></svg>`)
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
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20"><path d="M0 0 L1E1 1e1 L2E+1 1e-0 Z"/></svg>`)
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
