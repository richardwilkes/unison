// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/toolbox/fatal"
	"github.com/richardwilkes/toolbox/tid"
	"github.com/richardwilkes/unison"
)

var _ unison.TableRowData[*demoRow] = &demoRow{}

type demoRow struct {
	table        *unison.Table[*demoRow]
	parent       *demoRow
	checkbox     *unison.CheckBox
	text         string
	text2        string
	id           tid.TID
	children     []*demoRow
	container    bool
	open         bool
	doubleHeight bool
}

func (d *demoRow) CloneForTarget(target unison.Paneler, newParent *demoRow) *demoRow {
	table, ok := target.(*unison.Table[*demoRow])
	if !ok {
		fatal.IfErr(errs.New("invalid target"))
	}
	clone := *d
	clone.table = table
	clone.parent = newParent
	clone.id = tid.MustNewTID('a')
	return &clone
}

func (d *demoRow) ID() tid.TID {
	return d.id
}

func (d *demoRow) Parent() *demoRow {
	return d.parent
}

func (d *demoRow) SetParent(parent *demoRow) {
	d.parent = parent
}

func (d *demoRow) CanHaveChildren() bool {
	return d.container
}

func (d *demoRow) Children() []*demoRow {
	return d.children
}

func (d *demoRow) SetChildren(children []*demoRow) {
	d.children = children
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

func (d *demoRow) ColumnCell(row, col int, foreground, _ unison.Ink, _, _, _ bool) unison.Paneler {
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
		addWrappedText(wrapper, d.text, foreground, unison.LabelFont, width)
		if d.doubleHeight {
			addWrappedText(wrapper, "A little note…", foreground,
				unison.LabelFont.Face().Font(unison.LabelFont.Size()-1), width)
		}
		wrapper.UpdateTooltipCallback = func(_ unison.Point, _ unison.Rect) unison.Rect {
			wrapper.Tooltip = unison.NewTooltipWithText("A tooltip for the cell")
			return wrapper.RectToRoot(wrapper.ContentRect(true))
		}
		return wrapper
	case 2:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, d.text2, foreground, unison.LabelFont, width)
		return wrapper
	case 3:
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		width := d.table.CellWidth(row, col)
		addWrappedText(wrapper, "xyz", foreground, unison.LabelFont, width)
		return wrapper
	default:
		errs.Log(errs.New("column index out of range (0-2)"), "column", col)
		return unison.NewLabel()
	}
}

func addWrappedText(parent *unison.Panel, text string, ink unison.Ink, font unison.Font, width float32) {
	decoration := &unison.TextDecoration{Font: font}
	var lines []*unison.Text
	if width > 0 {
		lines = unison.NewTextWrappedLines(text, decoration, width)
	} else {
		lines = unison.NewTextLines(text, decoration)
	}
	for _, line := range lines {
		label := unison.NewLabel()
		label.Font = font
		label.LabelTheme.OnBackgroundInk = ink
		label.SetTitle(line.String())
		parent.AddChild(label)
	}
}

func (d *demoRow) IsOpen() bool {
	return d.open
}

func (d *demoRow) SetOpen(open bool) {
	d.open = open
}
