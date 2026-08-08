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
	"github.com/richardwilkes/unison/enums/blendmode"
)

// TestBlendColorFilterNoOpsAreNil documents the premise of TestComposeColorFilterWithNilOperands: this package's own
// constructors legitimately hand back a nil *ColorFilter for filters that would do nothing.
func TestBlendColorFilterNoOpsAreNil(t *testing.T) {
	c := check.New(t)
	c.Nil(NewBlendColorFilter(Red, blendmode.Dst))
	c.Nil(NewBlendColorFilter(Color(0), blendmode.SrcOver))
	c.NotNil(NewBlendColorFilter(Red, blendmode.SrcOver))
}

// TestComposeColorFilterWithNilOperands verifies that composing with a nil *ColorFilter yields the other filter rather
// than panicking with a nil pointer dereference.
func TestComposeColorFilterWithNilOperands(t *testing.T) {
	c := check.New(t)
	real1 := NewBlendColorFilter(Red, blendmode.SrcOver)
	c.NotNil(real1)
	real2 := NewLumaColorFilter()
	c.NotNil(real2)
	noop := NewBlendColorFilter(Red, blendmode.Dst)
	c.Nil(noop)

	// A nil operand on either side yields the other filter's contents.
	var composed *ColorFilter
	c.NotPanics(func() { composed = NewComposeColorFilter(noop, real1) })
	c.NotNil(composed)
	c.True(composed.filter == real1.filter)

	c.NotPanics(func() { composed = NewComposeColorFilter(real1, noop) })
	c.NotNil(composed)
	c.True(composed.filter == real1.filter)

	// Two nil operands compose to nil rather than to a filter wrapping nothing.
	c.NotPanics(func() { composed = NewComposeColorFilter(noop, noop) })
	c.Nil(composed)

	// Two real operands still compose into a distinct filter.
	c.NotPanics(func() { composed = NewComposeColorFilter(real1, real2) })
	c.NotNil(composed)
	c.True(composed.filter != real1.filter)
	c.True(composed.filter != real2.filter)
}
