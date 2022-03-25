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
	"github.com/richardwilkes/toolbox/xmath/geom"
	"github.com/richardwilkes/unison"
)

var _ unison.TableRowData = &demoRow{}

type demoRow struct {
	table        *unison.Table
	parent       unison.TableRowData
	text         string
	text2        string
	children     []unison.TableRowData
	checkbox     *unison.CheckBox
	container    bool
	open         bool
	doubleHeight bool
}

func (d *demoRow) ParentRow() unison.TableRowData {
	return d.parent
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
	case 3:
		return ""
	default:
		return ""
	}
}

func (d *demoRow) ColumnCell(row, col int, selected bool) unison.Paneler {
	switch col {
	case 0:
		if d.checkbox == nil {
			d.checkbox = unison.NewCheckBox()
		}
		return d.checkbox
	case 1:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, d.text, unison.LabelFont, width, selected)
		if d.doubleHeight {
			addWrappedText(wrapper, "A little note…", unison.LabelFont.Face().Font(unison.LabelFont.Size()-1), width, selected)
		}
		wrapper.UpdateTooltipCallback = func(where geom.Point[float32], suggestedAvoidInRoot geom.Rect[float32]) geom.Rect[float32] {
			wrapper.Tooltip = unison.NewTooltipWithText("A tooltip for the cell")
			return wrapper.RectToRoot(wrapper.ContentRect(true))
		}
		return wrapper
	case 2:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, d.text2, unison.LabelFont, width, selected)
		return wrapper
	case 3:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, "xyz", unison.LabelFont, width, selected)
		return wrapper
	default:
		jot.Errorf("column index out of range (0-2): %d", col)
		return unison.NewLabel()
	}
}

func addWrappedText(parent *unison.Panel, text string, font unison.Font, width float32, selected bool) {
	decoration := &unison.TextDecoration{Font: font}
	var lines []*unison.Text
	if width > 0 {
		lines = unison.NewTextWrappedLines(text, decoration, width)
	} else {
		lines = unison.NewTextLines(text, decoration)
	}
	for _, line := range lines {
		label := unison.NewLabel()
		label.Text = line.String()
		label.Font = font
		if selected {
			label.LabelTheme.OnBackgroundInk = unison.OnSelectionColor
		}
		parent.AddChild(label)
	}
}

func (d *demoRow) IsOpen() bool {
	return d.open
}

func (d *demoRow) SetOpen(open bool) {
	d.open = open
}
