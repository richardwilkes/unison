// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/unison"
)

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

func (d *demoRow) ColumnCell(index int) unison.Paneler {
	switch index {
	case 0:
		if d.checkbox == nil {
			d.checkbox = unison.NewCheckBox()
		}
		return d.checkbox
	case 1:
		label := unison.NewLabel()
		label.Text = d.text
		if !d.doubleHeight {
			return label
		}
		wrapper := unison.NewPanel()
		wrapper.SetLayout(&unison.FlexLayout{Columns: 1})
		wrapper.AddChild(label)
		label = unison.NewLabel()
		label.Text = "A little note…"
		desc := unison.LabelFont.ResolvedFont().Descriptor()
		desc.Size -= 2
		label.Font = desc.Font()
		wrapper.AddChild(label)
		return wrapper
	case 2:
		label := unison.NewLabel()
		label.Text = d.text2
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
