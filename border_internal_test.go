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
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// lineInkOf returns the ink of the line in a border made by newDefaultFieldBorder, looking through an accessoryBorder,
// or nil if the border is not shaped that way.
func lineInkOf(b Border) Ink {
	if ab, ok := b.(*accessoryBorder); ok {
		b = ab.Border
	}
	if cb, ok := b.(*CompoundBorder); ok && len(cb.borders) != 0 {
		if lb, ok2 := cb.borders[0].(*LineBorder); ok2 {
			return lb.ink
		}
	}
	return nil
}

// TestFieldBorderInkFollowsFieldValidity verifies that fieldBorderInk paints with the field's ErrorInk only while the
// field is invalid.
func TestFieldBorderInkFollowsFieldValidity(t *testing.T) {
	c := check.New(t)
	f := NewField()
	ink := &fieldBorderInk{normal: ThemeSurfaceEdge, field: f}
	rect := geom.NewRect(0, 0, 10, 10)
	c.Equal(ThemeSurfaceEdge.GetColor(), ink.Paint(nil, rect, paintstyle.Fill).Color())

	f.ValidateCallback = func() bool { return false }
	f.Validate()
	c.Equal(ThemeError.GetColor(), ink.Paint(nil, rect, paintstyle.Fill).Color())

	// The error ink is read at paint time, so an override made after the border was built is honored.
	f.ErrorInk = Red
	c.Equal(Red, ink.Paint(nil, rect, paintstyle.Fill).Color())

	f.ValidateCallback = nil
	f.Validate()
	c.Equal(ThemeSurfaceEdge.GetColor(), ink.Paint(nil, rect, paintstyle.Fill).Color())
}

// TestInstallDefaultFieldBorderIsErrorAwareOnlyForFields verifies that InstallDefaultFieldBorder wraps the line ink in
// a fieldBorderInk for a field, in both focus states, and leaves it plain for any other panel.
func TestInstallDefaultFieldBorderIsErrorAwareOnlyForFields(t *testing.T) {
	c := check.New(t)
	f := NewField()
	ink, ok := lineInkOf(f.Border()).(*fieldBorderInk)
	c.True(ok, "a field's unfocused border should be error-aware")
	if ok {
		c.True(ink.field == f)
		c.True(ink.normal == ThemeSurfaceEdge)
	}
	f.GainedFocusCallback()
	ink, ok = lineInkOf(f.Border()).(*fieldBorderInk)
	c.True(ok, "a field's focused border should be error-aware")
	if ok {
		c.True(ink.field == f)
		c.True(ink.normal == ThemeFocus)
	}
	f.LostFocusCallback()
	ink, ok = lineInkOf(f.Border()).(*fieldBorderInk)
	c.True(ok, "a field's unfocused border should be error-aware after focus leaves")
	if ok {
		c.True(ink.normal == ThemeSurfaceEdge)
	}

	p := NewPanel()
	InstallDefaultFieldBorder(p, p)
	c.True(lineInkOf(p.Border()) == ThemeSurfaceEdge, "a non-field panel should get the plain unfocused ink")
	p.GainedFocusCallback()
	c.True(lineInkOf(p.Border()) == ThemeFocus, "a non-field panel should get the plain focused ink")
	p.LostFocusCallback()
	c.True(lineInkOf(p.Border()) == ThemeSurfaceEdge, "a non-field panel should get the plain ink back")

	c.True(lineInkOf(NewDefaultFieldBorder(false)) == ThemeSurfaceEdge, "NewDefaultFieldBorder should use a plain ink")
}

// TestInstallDefaultFieldBorderResolvesEmbeddedField verifies that the field is found in a type that embeds it, both
// directly and through Self.
func TestInstallDefaultFieldBorderResolvesEmbeddedField(t *testing.T) {
	c := check.New(t)
	f := NewNumericField(5, 0, 100, strconv.Itoa, strconv.Atoi, nil)
	ink, ok := lineInkOf(f.Border()).(*fieldBorderInk)
	c.True(ok, "the border NewNumericField reinstalls should be error-aware")
	if ok {
		c.True(ink.field == f.Field)
	}
	c.True(fieldOf(f.AsPanel()) == f.Field, "the field should be found through Self")
	c.True(fieldOf(NewPanel()) == nil)
}
