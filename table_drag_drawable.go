// Copyright ©2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"fmt"

	"github.com/richardwilkes/unison/enums/paintstyle"
)

type dragDrawable struct {
	label *Label
}

// NewTableDragDrawable creates a new drawable for a table row drag.
func NewTableDragDrawable[T TableRowConstraint[T]](data *TableDragData[T], svg *SVG, singularName, pluralName string) Drawable {
	label := NewLabel()
	label.DrawCallback = func(gc *Canvas, rect Rect) {
		r := rect.Inset(NewUniformInsets(1))
		corner := r.Height / 2
		gc.SaveWithOpacity(0.7)
		gc.DrawRoundedRect(r, corner, corner, data.Table.SelectionInk.Paint(gc, r, paintstyle.Fill))
		gc.DrawRoundedRect(r, corner, corner, data.Table.OnSelectionInk.Paint(gc, r, paintstyle.Stroke))
		gc.Restore()
		label.DefaultDraw(gc, rect)
	}
	label.OnBackgroundInk = data.Table.OnSelectionInk
	label.SetBorder(NewEmptyBorder(Insets{
		Top:    4,
		Left:   label.Font.LineHeight(),
		Bottom: 4,
		Right:  label.Font.LineHeight(),
	}))
	if count := CountTableRows(data.Rows); count == 1 {
		label.SetPlainText(fmt.Sprintf("1 %s", singularName))
	} else {
		label.SetPlainText(fmt.Sprintf("%d %s", count, pluralName))
	}
	if svg != nil {
		baseline := label.Font.Baseline()
		label.Drawable = &DrawableSVG{
			SVG:  svg,
			Size: Size{Width: baseline, Height: baseline},
		}
	}
	_, pref, _ := label.Sizes(Size{})
	label.SetFrameRect(Rect{Size: pref})
	return &dragDrawable{label: label}
}

func (d *dragDrawable) LogicalSize() Size {
	return d.label.FrameRect().Size
}

func (d *dragDrawable) DrawInRect(canvas *Canvas, rect Rect, _ *SamplingOptions, _ *Paint) {
	d.label.Draw(canvas, rect)
}
