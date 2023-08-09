// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"

	"github.com/richardwilkes/toolbox/xmath/geom"
	"github.com/richardwilkes/unison/internal/skia"
)

// PathEffect1DStyle holds the 1D path effect.
type PathEffect1DStyle byte

// Possible values for PathEffect1DStyle.
const (
	TranslatePathEffect PathEffect1DStyle = iota
	RotatePathEffect
	MorphPathEffect
)

// TrimMode holds the type of trim.
type TrimMode byte

// Possible values for TrimMode.
const (
	NormalTrim TrimMode = iota
	InvertedTrim
)

// PathEffect affects the geometry of a drawing primitive before it is transformed by the canvas' matrix and drawn.
type PathEffect struct {
	effect skia.PathEffect
}

func newPathEffect(effect skia.PathEffect) *PathEffect {
	if effect == nil {
		return nil
	}
	e := &PathEffect{effect: effect}
	runtime.SetFinalizer(e, func(obj *PathEffect) {
		ReleaseOnUIThread(func() {
			skia.PathEffectUnref(obj.effect)
		})
	})
	return e
}

func (e *PathEffect) effectOrNil() skia.PathEffect {
	if e == nil {
		return nil
	}
	return e.effect
}

// NewComposePathEffect creates a new PathEffect that combines two PathEffects.
func NewComposePathEffect(outer, inner *PathEffect) *PathEffect {
	return newPathEffect(skia.PathEffectCreateCompose(outer.effect, inner.effect))
}

// NewSumPathEffect creates a new sum PathEffect.
func NewSumPathEffect(first, second *PathEffect) *PathEffect {
	return newPathEffect(skia.PathEffectCreateSum(first.effect, second.effect))
}

// NewDiscretePathEffect creates a new discrete PathEffect.
func NewDiscretePathEffect(segLength, deviation float32, seedAssist uint32) *PathEffect {
	return newPathEffect(skia.PathEffectCreateDiscrete(segLength, deviation, seedAssist))
}

// NewCornerPathEffect creates a new corner PathEffect.
func NewCornerPathEffect(radius float32) *PathEffect {
	return newPathEffect(skia.PathEffectCreateCorner(radius))
}

// New1dPathPathEffect creates a new 1D path PathEffect.
func New1dPathPathEffect(path *Path, advance, phase float32, style PathEffect1DStyle) *PathEffect {
	return newPathEffect(skia.PathEffectCreate1dPath(path.path, advance, phase, skia.PathEffect1DStyle(style)))
}

// New2dLinePathEffect creates a new 2D line PathEffect.
func New2dLinePathEffect(width float32, matrix *geom.Matrix2D32) *PathEffect {
	return newPathEffect(skia.PathEffectCreate2dLine(width, skia.Matrix2DtoMatrix(matrix)))
}

// New2dPathEffect creates a new 2D PathEffect.
func New2dPathEffect(matrix *geom.Matrix2D32, path *Path) *PathEffect {
	return newPathEffect(skia.PathEffectCreate2dPath(skia.Matrix2DtoMatrix(matrix), path.path))
}

// NewDashPathEffect creates a new dash PathEffect.
func NewDashPathEffect(intervals []float32, phase float32) *PathEffect {
	return newPathEffect(skia.PathEffectCreateDash(intervals, phase))
}

// NewTrimPathEffect creates a new trim PathEffect.
func NewTrimPathEffect(start, stop float32, mode TrimMode) *PathEffect {
	return newPathEffect(skia.PathEffectCreateTrim(start, stop, skia.TrimMode(mode)))
}

var dashEffect *PathEffect

// DashEffect returns a 4-4 dash effect.
func DashEffect() *PathEffect {
	if dashEffect == nil {
		dashEffect = NewDashPathEffect([]float32{4, 4}, 0)
	}
	return dashEffect
}
