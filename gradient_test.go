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
