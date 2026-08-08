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
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/gradienttype"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// TestNewEvenlySpacedGradientStopsForColors verifies the documented contract: the first stop is at 0, the last is at
// exactly 1, and the ones in between increase evenly. Before this, the last stop kept the float32 accumulation of the
// step (e.g. 1.0000001 for 7 colors, 0.99999994 for others) rather than an exact 1, which a subsequent Reverse could
// then turn into a slightly negative location.
func TestNewEvenlySpacedGradientStopsForColors(t *testing.T) {
	c := check.New(t)
	c.Nil(unison.NewEvenlySpacedGradientStopsForColors(), "no colors should produce no stops")

	// A single color has no second endpoint to span to, so its lone stop stays at 0.
	single := unison.NewEvenlySpacedGradientStopsForColors(unison.Red)
	c.Equal(1, len(single))
	c.Equal(float32(0), single[0].Location)

	// Enough colors to cover the counts whose step does not accumulate back to exactly 1 in float32 (11 and 12 do
	// drift, reaching 1.0000001).
	colors := []unison.ColorProvider{
		unison.Red, unison.Green, unison.Blue, unison.Yellow, unison.Cyan, unison.Magenta, unison.Orange,
		unison.White, unison.Black, unison.Gray, unison.Teal, unison.Navy, unison.Olive, unison.Purple,
	}
	for count := 2; count <= len(colors); count++ {
		stops := unison.NewEvenlySpacedGradientStopsForColors(colors[:count]...)
		c.Equal(count, len(stops), "count %d", count)
		c.Equal(float32(0), stops[0].Location, "count %d: the first stop must be at 0", count)
		c.Equal(float32(1), stops[count-1].Location, "count %d: the last stop must be at exactly 1", count)
		for i := 1; i < count; i++ {
			c.True(stops[i].Location > stops[i-1].Location, "count %d: stop %d must come after stop %d", count, i, i-1)
		}
		stops.Reverse()
		c.Equal(float32(0), stops[0].Location, "count %d: the first stop after Reverse must be at 0", count)
		c.Equal(float32(1), stops[count-1].Location, "count %d: the last stop after Reverse must be at exactly 1",
			count)
	}
}

func TestGradientPaintDoesNotMutateGradient(t *testing.T) {
	c := check.New(t)
	rect := geom.NewRect(0, 0, 100, 50)
	for _, kind := range []gradienttype.Enum{
		gradienttype.Linear,
		gradienttype.Radial,
		gradienttype.Sweep,
		gradienttype.Conical,
	} {
		g := &unison.Gradient{
			Stops:  unison.NewEvenlySpacedGradientStopsForColors(unison.Red, unison.Blue),
			EndPt:  geom.NewPoint(1, 1),
			Radius: unison.StartEnd{Start: 1, End: 10},
			Angle:  unison.StartEnd{End: 360},
			Kind:   kind,
		}
		before := *g
		p := g.Paint(nil, rect, paintstyle.Fill)
		c.NotNil(p, "kind %v", kind)
		c.Equal(before, *g, "Paint must not mutate the Gradient (kind %v)", kind)
		c.Equal(geom.Matrix{}, g.Transform, "Transform must remain the zero matrix (kind %v)", kind)
	}
}

func TestGradientPaintConcurrentUse(t *testing.T) {
	// Gradients are commonly shared theme state; concurrent Paint calls on the same Gradient must be safe. Run with
	// -race to catch regressions that write to the receiver during painting.
	g := &unison.Gradient{
		Stops: unison.NewEvenlySpacedGradientStopsForColors(unison.Red, unison.Green, unison.Blue),
		EndPt: geom.NewPoint(1, 1),
		Kind:  gradienttype.Linear,
	}
	rect := geom.NewRect(0, 0, 64, 64)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				g.Paint(nil, rect, paintstyle.Fill)
			}
		}()
	}
	wg.Wait()
	check.New(t).Equal(geom.Matrix{}, g.Transform)
}
