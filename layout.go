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
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/toolbox/xmath/mathf32"
)

const (
	// DefaultMaxSize is the default size that should be used for a maximum dimension if the target has no real
	// preference and can be expanded beyond its preferred size. This is intentionally not something very large to allow
	// basic math operations an opportunity to succeed when laying out panels. It is perfectly acceptable to use a
	// larger value than this, however, if that makes sense for your specific target.
	DefaultMaxSize = 10000
	// StdHSpacing is the typical spacing between columns.
	StdHSpacing = 8
	// StdVSpacing is the typical spacing between rows.
	StdVSpacing = 4
)

// Alignment constants.
const (
	StartAlignment Alignment = iota
	MiddleAlignment
	EndAlignment
	FillAlignment
)

// Alignment specifies how to align an object within its available space.
type Alignment uint8

// Side constants.
const (
	TopSide Side = iota
	LeftSide
	BottomSide
	RightSide
)

// Side specifies which side an object should be on.
type Side uint8

// Horizontal returns true if the side is to the left or right.
func (s Side) Horizontal() bool {
	return s == LeftSide || s == RightSide
}

// Vertical returns true if the side is to the top or bottom.
func (s Side) Vertical() bool {
	return s == TopSide || s == BottomSide
}

// Sizer returns minimum, preferred, and maximum sizes. The hint will contain
// values other than zero for a dimension that has already been determined.
type Sizer func(hint geom32.Size) (min, pref, max geom32.Size)

// Layout defines methods that all layouts must provide.
type Layout interface {
	LayoutSizes(target Layoutable, hint geom32.Size) (min, pref, max geom32.Size)
	PerformLayout(target Layoutable)
}

// Layoutable defines the methods an object that wants to participate in
// layout must implement.
type Layoutable interface {
	SetLayout(layout Layout)
	LayoutData() interface{}
	SetLayoutData(data interface{})
	Sizes(hint geom32.Size) (min, pref, max geom32.Size)
	Border() Border
	FrameRect() geom32.Rect
	SetFrameRect(rect geom32.Rect)
	ChildrenForLayout() []Layoutable
}

// MaxSize returns the size that is at least as large as DefaultMaxSize in
// both dimensions, but larger if the size that is passed in is larger.
func MaxSize(size geom32.Size) geom32.Size {
	return geom32.Size{
		Width:  mathf32.Max(DefaultMaxSize, size.Width),
		Height: mathf32.Max(DefaultMaxSize, size.Height),
	}
}
