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

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xmath"
)

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
