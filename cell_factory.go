// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/xmath/geom32"
)

var _ CellFactory = &DefaultCellFactory{}

// CellFactory defines methods all cell factories must implement.
type CellFactory interface {
	// CellHeight returns the height to use for the cells. A value less than 1 indicates that each cell's height may be
	// different.
	CellHeight() float32

	// CreateCell creates a new cell for 'owner' using 'element' as the content. 'index' indicates which row the element
	// came from. 'selected' indicates the cell should be created in its selected state. 'focused' indicates the cell
	// should be created in its focused state.
	CreateCell(owner Paneler, element interface{}, index int, foreground Ink, selected, focused bool) *Panel
}

// DefaultCellFactory provides a simple implementation of a CellFactory that uses Labels for its cells.
type DefaultCellFactory struct {
	Height float32
}

// CellHeight implements CellFactory.
func (f *DefaultCellFactory) CellHeight() float32 {
	return f.Height
}

// CreateCell implements CellFactory.
func (f *DefaultCellFactory) CreateCell(owner Paneler, element interface{}, index int, foreground Ink, selected, focused bool) *Panel {
	txtLabel := NewLabel()
	txtLabel.Text = fmt.Sprintf("%v", element)
	txtLabel.SetBorder(NewEmptyBorder(geom32.Insets{Top: 2, Left: 4, Bottom: 2, Right: 4}))
	txtLabel.Font = FieldFont
	txtLabel.Ink = foreground
	return txtLabel.AsPanel()
}
