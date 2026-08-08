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
	"sync"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestComposeAndSumPathEffectNilOperands verifies that a nil *PathEffect operand collapses to the other operand (or to
// nil when both are nil) rather than panicking. The package's own constructors return nil for inputs that produce no
// effect (e.g. a zero corner radius) and Paint.PathEffect() returns nil when none is set, so composing those results is
// an ordinary thing for callers to do.
func TestComposeAndSumPathEffectNilOperands(t *testing.T) {
	c := check.New(t)

	dash := NewDashPathEffect([]float32{4, 4}, 0)
	c.NotNil(dash)
	corner := NewCornerPathEffect(4)
	c.NotNil(corner)
	c.Nil(NewCornerPathEffect(0), "a zero radius should produce no effect, the nil these constructors must tolerate")

	for i, in := range []struct {
		first  *PathEffect
		second *PathEffect
		want   *PathEffect
	}{
		{first: nil, second: nil, want: nil},
		{first: dash, second: nil, want: dash},
		{first: nil, second: dash, want: dash},
		{first: NewPaint().PathEffect(), second: corner, want: corner},
	} {
		var composed, summed *PathEffect
		c.NotPanics(func() { composed = NewComposePathEffect(in.first, in.second) }, "compose case %d", i)
		c.NotPanics(func() { summed = NewSumPathEffect(in.first, in.second) }, "sum case %d", i)
		if in.want == nil {
			c.Nil(composed, "compose case %d: two nil operands should produce no effect", i)
			c.Nil(summed, "sum case %d: two nil operands should produce no effect", i)
			continue
		}
		c.Equal(in.want.effect, composed.effect, "compose case %d: should collapse to the non-nil operand", i)
		c.Equal(in.want.effect, summed.effect, "sum case %d: should collapse to the non-nil operand", i)
	}

	c.NotNil(NewComposePathEffect(dash, corner), "two real effects should still compose")
	c.NotNil(NewSumPathEffect(dash, corner), "two real effects should still sum")
}

func TestDashEffectConcurrentInit(t *testing.T) {
	const goroutines = 16
	results := make([]*PathEffect, goroutines)
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(goroutines)
	for i := range goroutines {
		go func() {
			defer done.Done()
			start.Wait()
			results[i] = DashEffect()
		}()
	}
	start.Done()
	done.Wait()
	c := check.New(t)
	c.NotNil(results[0])
	for i := 1; i < goroutines; i++ {
		c.Equal(results[0], results[i])
	}
}
