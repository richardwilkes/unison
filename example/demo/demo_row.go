// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"strconv"

	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

var _ unison.TableRowData = &demoRow{}

type demoRow struct {
	text         string
	text2        string
	children     []unison.TableRowData
	checkbox     *unison.CheckBox
	container    bool
	open         bool
	doubleHeight bool
}

func (d *demoRow) CanHaveChildRows() bool {
	return d.container
}

func (d *demoRow) ChildRows() []unison.TableRowData {
	return d.children
}

func (d *demoRow) CellDataForSort(index int) string {
	switch index {
	case 0:
		if d.checkbox == nil {
			d.checkbox = unison.NewCheckBox()
		}
		return strconv.Itoa(int(d.checkbox.State))
	case 1:
		return d.text
	case 2:
		return d.text2
	default:
		return ""
	}
}

func (d *demoRow) ColumnCell(index int, selected bool) unison.Paneler {
	switch index {
	case 0:
		if d.checkbox == nil {
			d.checkbox = unison.NewCheckBox()
		}
		return d.checkbox
	case 1:
		label := unison.NewLabel()
		label.Text = d.text
		if selected {
			label.OnBackgroundInk = unison.OnSelectionColor
		}
		if !d.doubleHeight {
			return label
		}
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		wrapper.AddChild(label)
		subLabel := unison.NewLabel()
		subLabel.Text = "A little note…"
		if selected {
			subLabel.OnBackgroundInk = unison.OnSelectionColor
		}
		desc := unison.LabelFont.ResolvedFont().Descriptor()
		desc.Size -= 2
		subLabel.Font = desc.Font()
		wrapper.AddChild(subLabel)
		wrapper.UpdateTooltipCallback = func(where geom32.Point, suggestedAvoid geom32.Rect) geom32.Rect {
			wrapper.Tooltip = unison.NewTooltipWithText("A tooltip for the cell")
			avoid := label.FrameRect()
			avoid.Union(subLabel.FrameRect())
			return wrapper.RectToRoot(avoid)
		}
		return wrapper
	case 2:
		label := unison.NewLabel()
		label.Text = d.text2
		if selected {
			label.OnBackgroundInk = unison.OnSelectionColor
		}
		return label
	default:
		jot.Errorf("column index out of range (0-2): %d", index)
		return unison.NewLabel()
	}
}

func (d *demoRow) IsOpen() bool {
	return d.open
}

func (d *demoRow) SetOpen(open bool) {
	d.open = open
}
