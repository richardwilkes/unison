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
)

func rectPath(bounds geom.Rect) *unison.Path {
	p := unison.NewPath()
	p.Rect(bounds)
	return p
}

// TestPathBooleanOps exercises the four boolean path operators, which share a single applyOp helper. Two 10x10 squares
// overlap in a 5x5 region; each operator's result is probed with Contains to confirm it selects the right region.
func TestPathBooleanOps(t *testing.T) {
	c := check.New(t)

	a := geom.NewRect(0, 0, 10, 10)
	b := geom.NewRect(5, 5, 10, 10)
	inAOnly := geom.NewPoint(2, 2)   // inside A, outside B
	inBOnly := geom.NewPoint(12, 12) // inside B, outside A
	inBoth := geom.NewPoint(7, 7)    // inside the overlap

	// Union: covers both squares.
	p := rectPath(a)
	c.True(p.Union(rectPath(b)))
	c.True(p.Contains(inAOnly))
	c.True(p.Contains(inBOnly))
	c.True(p.Contains(inBoth))

	// Intersect: only the overlap survives.
	p = rectPath(a)
	c.True(p.Intersect(rectPath(b)))
	c.False(p.Contains(inAOnly))
	c.False(p.Contains(inBOnly))
	c.True(p.Contains(inBoth))

	// Subtract: A minus the overlap.
	p = rectPath(a)
	c.True(p.Subtract(rectPath(b)))
	c.True(p.Contains(inAOnly))
	c.False(p.Contains(inBOnly))
	c.False(p.Contains(inBoth))

	// Xor: both squares minus the overlap.
	p = rectPath(a)
	c.True(p.Xor(rectPath(b)))
	c.True(p.Contains(inAOnly))
	c.True(p.Contains(inBOnly))
	c.False(p.Contains(inBoth))
}

// TestPaintFillPathWithCull confirms both the nil and non-nil cullRect paths of FillPathWithCull produce a usable
// result. The non-nil path flows through toSkRectPtr, which replaced the previously open-coded conversion.
func TestPaintFillPathWithCull(t *testing.T) {
	c := check.New(t)

	paint := unison.NewPaint()
	paint.SetStyle(paintstyle.Stroke)
	paint.SetStrokeWidth(2)

	src := rectPath(geom.NewRect(0, 0, 20, 20))

	result, _ := paint.FillPathWithCull(src, nil, 1)
	c.False(result.Empty())

	cull := geom.NewRect(0, 0, 20, 20)
	culled, _ := paint.FillPathWithCull(src, &cull, 1)
	c.False(culled.Empty())
}
