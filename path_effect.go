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

	skpatheffect "github.com/richardwilkes/canvas/patheffect"
	"github.com/richardwilkes/canvas/stroke"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/patheffect"
	"github.com/richardwilkes/unison/enums/trimmode"
)

// PathEffect affects the geometry of a drawing primitive before it is transformed by the canvas' matrix and drawn.
type PathEffect struct {
	effect stroke.PathEffect
}

func newPathEffect(effect stroke.PathEffect) *PathEffect {
	if effect == nil {
		return nil
	}
	return &PathEffect{effect: effect}
}

func (e *PathEffect) effectOrNil() stroke.PathEffect {
	if e == nil {
		return nil
	}
	return e.effect
}

// NewComposePathEffect creates a new PathEffect that combines two PathEffects.
func NewComposePathEffect(outer, inner *PathEffect) *PathEffect {
	return newPathEffect(skpatheffect.MakeCompose(outer.effect, inner.effect))
}

// NewSumPathEffect creates a new sum PathEffect.
func NewSumPathEffect(first, second *PathEffect) *PathEffect {
	return newPathEffect(skpatheffect.MakeSum(first.effect, second.effect))
}

// NewDiscretePathEffect creates a new discrete PathEffect.
func NewDiscretePathEffect(segLength, deviation float32, seedAssist uint32) *PathEffect {
	return newPathEffect(skpatheffect.MakeDiscrete(segLength, deviation, seedAssist))
}

// NewCornerPathEffect creates a new corner PathEffect.
func NewCornerPathEffect(radius float32) *PathEffect {
	return newPathEffect(skpatheffect.MakeCorner(radius))
}

// New1dPathPathEffect creates a new 1D path PathEffect.
func New1dPathPathEffect(path *Path, advance, phase float32, style patheffect.Enum) *PathEffect {
	return newPathEffect(skpatheffect.MakePath1D(path.path, advance, phase, skpatheffect.Path1DStyle(style)))
}

// New2dLinePathEffect creates a new 2D line PathEffect.
func New2dLinePathEffect(width float32, matrix geom.Matrix) *PathEffect {
	return newPathEffect(skpatheffect.MakeLine2D(width, toCanvasMatrixPtr(matrix)))
}

// New2dPathEffect creates a new 2D PathEffect.
func New2dPathEffect(matrix geom.Matrix, path *Path) *PathEffect {
	return newPathEffect(skpatheffect.MakePath2D(toCanvasMatrixPtr(matrix), path.path))
}

// NewDashPathEffect creates a new dash PathEffect.
func NewDashPathEffect(intervals []float32, phase float32) *PathEffect {
	return newPathEffect(skpatheffect.MakeDash(intervals, phase))
}

// NewTrimPathEffect creates a new trim PathEffect.
func NewTrimPathEffect(start, stop float32, mode trimmode.Enum) *PathEffect {
	return newPathEffect(skpatheffect.MakeTrim(start, stop, skpatheffect.TrimMode(mode)))
}

var (
	dashEffect     *PathEffect
	dashEffectOnce sync.Once
)

// DashEffect returns a 4-4 dash effect.
func DashEffect() *PathEffect {
	dashEffectOnce.Do(func() {
		dashEffect = NewDashPathEffect([]float32{4, 4}, 0)
	})
	return dashEffect
}
