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
