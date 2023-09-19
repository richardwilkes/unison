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
	"github.com/richardwilkes/toolbox/xmath/geom"
	"github.com/richardwilkes/toolbox/xmath/geom/poly"
)

// Point is an alias for geom.Point[float32], for convenience.
type Point = geom.Point[float32]

// NewPoint creates a new Point.
func NewPoint(x, y float32) Point {
	return geom.NewPoint(x, y)
}

// Size is an alias for geom.Size[float32], for convenience.
type Size = geom.Size[float32]

// NewSize creates a new Size.
func NewSize(width, height float32) Size {
	return geom.NewSize(width, height)
}

// Rect is an alias for geom.Rect[float32], for convenience.
type Rect = geom.Rect[float32]

// NewRect creates a new Rect.
func NewRect(x, y, width, height float32) Rect {
	return geom.NewRect(x, y, width, height)
}

// Insets is an alias for geom.Insets[float32], for convenience.
type Insets = geom.Insets[float32]

// NewUniformInsets creates a new Insets whose edges all have the same value.
func NewUniformInsets(amount float32) Insets {
	return geom.NewUniformInsets(amount)
}

// NewSymmetricInsets returns an Insets whose edges match their opposite edge.
func NewSymmetricInsets(h, v float32) Insets {
	return geom.NewSymmetricInsets(h, v)
}

// NewHorizontalInsets creates a new Insets whose left and right edges have the specified value.
func NewHorizontalInsets(amount float32) Insets {
	return geom.NewHorizontalInsets(amount)
}

// NewVerticalInsets creates a new Insets whose top and bottom edges have the specified value.
func NewVerticalInsets(amount float32) Insets {
	return geom.NewVerticalInsets(amount)
}

// StdInsets returns insets preset to the standard spacing.
func StdInsets() Insets {
	return Insets{
		Top:    StdVSpacing,
		Left:   StdHSpacing,
		Bottom: StdVSpacing,
		Right:  StdHSpacing,
	}
}

// Matrix is an alias for geom.Matrix2D[float32], for convenience.
type Matrix = geom.Matrix[float32]

// NewIdentityMatrix creates a new identity transformation 2D matrix.
func NewIdentityMatrix() Matrix {
	return geom.NewIdentityMatrix[float32]()
}

// NewTranslationMatrix creates a new 2D matrix that translates by 'tx' and 'ty'.
func NewTranslationMatrix(tx, ty float32) Matrix {
	return geom.NewTranslationMatrix(tx, ty)
}

// NewScaleMatrix creates a new 2D matrix that scales by 'sx' and 'sy'.
func NewScaleMatrix(sx, sy float32) Matrix {
	return geom.NewScaleMatrix(sx, sy)
}

// NewRotationMatrix creates a new 2D matrix that rotates by 'radians'. Positive values are clockwise.
func NewRotationMatrix(radians float32) Matrix {
	return geom.NewRotationMatrix(radians)
}

// NewRotationByDegreesMatrix creates a new 2D matrix that rotates by 'degrees'. Positive values are clockwise.
func NewRotationByDegreesMatrix(degrees float32) Matrix {
	return geom.NewRotationByDegreesMatrix(degrees)
}

// Polygon is an alias for poly.Polygon[float32], for convenience.
type Polygon = poly.Polygon[float32]

// Contour is an alias for poly.Contour[float32], for convenience.
type Contour = poly.Contour[float32]
