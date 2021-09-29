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

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

var tableCounter int

// NewDemoTableWindow creates and displays our demo table window.
func NewDemoTableWindow(where geom32.Point) (*unison.Window, error) {
	// Create the window
	tableCounter++
	wnd, err := unison.NewWindow(fmt.Sprintf("Table #%d", tableCounter))
	if err != nil {
		return nil, err
	}

	// Install our menus
	installDefaultMenus(wnd)

	content := wnd.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1})

	// Create the table
	table := unison.NewTable()
	table.ColumnWidths = make([]float32, 2)
	rows := make([]unison.TableRowData, 10000)
	for i := range rows {
		row := &demoRow{index: i}
		if i%10 == 3 {
			row.container = true
			row.open = true
			row.children = make([]unison.TableRowData, 5)
			for j := range row.children {
				row.children[j] = &demoRow{
					index:    i,
					subIndex: j + 1,
				}
			}
		}
		rows[i] = row
	}
	table.SetTopLevelRows(rows)
	table.SizeColumnsToFit()

	// Create a scroll panel and place a table panel inside it
	scrollArea := unison.NewScrollPanel()
	scrollArea.SetContent(table, unison.FillBehavior)
	scrollArea.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: unison.FillAlignment,
		VAlign: unison.FillAlignment,
		HGrab:  true,
		VGrab:  true,
	})
	content.AddChild(scrollArea)

	// Pack our window to fit its content, then set its location on the display and make it visible.
	wnd.Pack()
	rect := wnd.FrameRect()
	rect.Point = where
	wnd.SetFrameRect(rect)
	wnd.ToFront()

	return wnd, nil
}
