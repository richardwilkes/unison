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
	text      string
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
		return unison.NewCheckBox()
	case 1:
		label := unison.NewLabel()
		label.Text = d.text
		return label
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
