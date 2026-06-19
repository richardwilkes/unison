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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/check"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

// DefaultCheckBoxTheme holds the default CheckBoxTheme values for CheckBoxes. Modifying this data will not alter
// existing CheckBoxes, but will alter any CheckBoxes created in the future.
var DefaultCheckBoxTheme = CheckBoxTheme{
	RadioButtonTheme: DefaultRadioButtonTheme,
	CornerRadius:     geom.NewUniformSize(4),
}

// CheckBoxTheme holds theming data for a CheckBox.
type CheckBoxTheme struct {
	RadioButtonTheme
	CornerRadius geom.Size
}

// CheckBox represents a clickable checkbox with an optional label.
type CheckBox struct {
	CheckBoxTheme
	checkRadioBase
	State check.Enum
}

// NewCheckBox creates a new checkbox.
func NewCheckBox() *CheckBox {
	var c CheckBox
	c.Self = &c
	c.CheckBoxTheme = DefaultCheckBoxTheme
	c.baseTheme = &c.RadioButtonTheme
	c.commonInit()
	c.updateState = func() {
		if c.State == check.On {
			c.State = check.Off
		} else {
			c.State = check.On
		}
	}
	c.drawMark = c.drawCheck
	return &c
}

func (c *CheckBox) drawCheck(canvas *Canvas, rect geom.Rect, thickness float32, fg, bg, edge Ink) {
	DrawRoundedRectBase(canvas, rect, c.CornerRadius, thickness, bg, edge)
	rect = rect.Inset(geom.NewUniformInsets(0.5))
	if c.State == check.Off {
		return
	}
	paint := fg.Paint(canvas, rect, paintstyle.Stroke)
	defer paint.Dispose()
	paint.SetStrokeWidth(2)
	if !c.Enabled() {
		paint.SetColorFilter(Grayscale30Filter())
	}
	if c.State == check.On {
		path := NewPath()
		defer path.Dispose()
		path.MoveTo(geom.NewPoint(rect.X+rect.Width*0.25, rect.Y+rect.Height*0.55))
		path.LineTo(geom.NewPoint(rect.X+rect.Width*0.45, rect.Y+rect.Height*0.7))
		path.LineTo(geom.NewPoint(rect.X+rect.Width*0.75, rect.Y+rect.Height*0.3))
		canvas.DrawPath(path, paint)
	} else {
		canvas.DrawLine(rect.Point.Add(geom.NewPoint(rect.Width*0.25, rect.Height*0.5)),
			rect.Point.Add(geom.NewPoint(rect.Width*0.7, rect.Height*0.5)), paint)
	}
}
