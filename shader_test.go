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
	"github.com/richardwilkes/unison/enums/blendmode"
)

// TestNewBlendShaderNilOperands verifies that a nil *Shader operand yields a nil Shader rather than a panic. A nil
// *Shader is a legitimate value in this API family — Paint.Shader() returns nil when no shader is set and the package's
// own constructors can return nil — so the operands must go through the same nil guard SetShader uses.
func TestNewBlendShaderNilOperands(t *testing.T) {
	c := check.New(t)

	solid := unison.NewColorShader(unison.Red)
	c.NotNil(solid)
	for i, in := range []struct{ dst, src *unison.Shader }{
		{dst: nil, src: nil},
		{dst: solid, src: nil},
		{dst: nil, src: solid},
		{dst: unison.NewPaint().Shader(), src: unison.NewPaint().Shader()},
	} {
		var shader *unison.Shader
		c.NotPanics(func() { shader = unison.NewBlendShader(blendmode.SrcOver, in.dst, in.src) }, "case %d", i)
		c.Nil(shader, "case %d: a nil operand should produce no shader", i)
	}

	c.NotNil(unison.NewBlendShader(blendmode.SrcOver, solid, unison.NewColorShader(unison.Blue)),
		"two real shaders should still blend")
}
