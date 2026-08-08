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
	"github.com/richardwilkes/unison"
)

// TestNewShaderMaskFilterNilShader verifies that a nil *Shader yields a nil MaskFilter rather than a panic.
// Paint.Shader() returns nil for a paint with no shader, so handing that result straight to NewShaderMaskFilter — the
// obvious way to reuse a paint's shader as a mask — must work like the rest of the API family, which tolerates nil.
func TestNewShaderMaskFilterNilShader(t *testing.T) {
	c := check.New(t)

	var filter *unison.MaskFilter
	c.NotPanics(func() { filter = unison.NewShaderMaskFilter(unison.NewPaint().Shader()) })
	c.Nil(filter, "a nil shader should produce no mask filter")
	c.NotPanics(func() { filter = unison.NewShaderMaskFilter(nil) })
	c.Nil(filter, "a nil shader should produce no mask filter")

	// The resulting nil must remain usable with the rest of the API, which also tolerates nil.
	p := unison.NewPaint()
	c.NotPanics(func() { p.SetMaskFilter(filter) })
	c.Nil(p.MaskFilter())

	c.NotNil(unison.NewShaderMaskFilter(unison.NewColorShader(unison.Red)), "a real shader should still make a filter")
}
