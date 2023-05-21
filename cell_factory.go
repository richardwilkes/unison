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
)

var _ CellFactory = &DefaultCellFactory{}

// CellFactory defines methods all cell factories must implement.
type CellFactory interface {
	// CellHeight returns the height to use for the cells. A value less than 1 indicates that each cell's height may be
	// different.
	CellHeight() float32

	// CreateCell creates a new cell.
	CreateCell(owner Paneler, element any, row int, foreground, background Ink, selected, focused bool) Paneler
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
func (f *DefaultCellFactory) CreateCell(_ Paneler, element any, _ int, foreground, _ Ink, _, _ bool) Paneler {
	txtLabel := NewLabel()
	txtLabel.SetBorder(NewEmptyBorder(Insets{Top: StdVSpacing, Left: StdHSpacing, Bottom: StdVSpacing, Right: StdHSpacing}))
	txtLabel.Text = fmt.Sprintf("%v", element)
	txtLabel.Font = FieldFont
	txtLabel.OnBackgroundInk = foreground
	return txtLabel
}
