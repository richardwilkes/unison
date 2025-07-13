// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var (
	_ Ink = &IndirectInk{}
	_ Ink = &ColorFilteredInk{}
)

// Ink holds a color, pattern, or gradient to draw with.
type Ink interface {
	Paint(canvas *Canvas, rect geom.Rect, style paintstyle.Enum) *Paint
}

// IndirectInk an ink that references another ink.
type IndirectInk struct {
	Target Ink
}

// Paint implements Ink.
func (i *IndirectInk) Paint(canvas *Canvas, rect geom.Rect, style paintstyle.Enum) *Paint {
	return i.Target.Paint(canvas, rect, style)
}

// ColorFilteredInk holds an ink and a color filter to apply to the ink.
type ColorFilteredInk struct {
	OriginalInk Ink
	ColorFilter *ColorFilter
}

// Paint implements Ink.
func (c *ColorFilteredInk) Paint(canvas *Canvas, rect geom.Rect, style paintstyle.Enum) *Paint {
	paint := c.OriginalInk.Paint(canvas, rect, style)
	if c.ColorFilter != nil {
		paint.SetColorFilter(c.ColorFilter)
	}
	return paint
}
