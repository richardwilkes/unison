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
	"fmt"

	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

type demoRow struct {
	index     int
	subIndex  int
	children  []unison.TableRowData
	container bool
	open      bool
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
		label := unison.NewLabel()
		label.SetBorder(unison.NewEmptyBorder(geom32.Insets{Top: 5, Left: 0, Bottom: 5, Right: 5}))
		label.Text = fmt.Sprintf("Row %d", d.index+1)
		return label
	case 1:
		check := unison.NewCheckBox()
		check.Font = unison.LabelFont
		if d.subIndex != 0 {
			check.Text = fmt.Sprintf("Sub-Row %d", d.subIndex)
		}
		check.SetBorder(unison.NewEmptyBorder(geom32.NewUniformInsets(5)))
		return check
	default:
		jot.Fatal(1, "column index out of bounds")
		return nil
	}
}

func (d *demoRow) IsOpen() bool {
	return d.open
}

func (d *demoRow) SetOpen(open bool) {
	d.open = open
}
