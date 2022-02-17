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
	"fmt"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

const topLevelRowsToMake = 100

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
	table.HierarchyColumnIndex = 1
	table.ColumnSizes = make([]unison.ColumnSize, 3)
	for i := range table.ColumnSizes {
		table.ColumnSizes[i].Minimum = 20
		table.ColumnSizes[i].Maximum = 10000
	}
	_, checkColSize, _ := unison.NewCheckBox().Sizes(geom32.Size{})
	table.ColumnSizes[0].Minimum = checkColSize.Width
	table.ColumnSizes[0].Maximum = checkColSize.Width
	rows := make([]unison.TableRowData, topLevelRowsToMake)
	for i := range rows {
		row := &demoRow{
			text:  fmt.Sprintf("Row %d", i+1),
			text2: fmt.Sprintf("Some longer content for Row %d", i+1),
		}
		if i%10 == 3 {
			if i == 3 {
				row.doubleHeight = true
			}
			row.container = true
			row.open = true
			row.children = make([]unison.TableRowData, 5)
			for j := range row.children {
				child := &demoRow{text: fmt.Sprintf("Sub-Row %d", j+1)}
				row.children[j] = child
				if j < 2 {
					child.container = true
					child.open = true
					child.children = make([]unison.TableRowData, 2)
					for k := range child.children {
						child.children[k] = &demoRow{text: fmt.Sprintf("Sub-Sub-Row %d", k+1)}
					}
				}
			}
		}
		rows[i] = row
	}
	table.SetTopLevelRows(rows)
	table.SizeColumnsToFit(true)

	header := unison.NewTableHeader(table,
		unison.NewTableColumnHeader("", ""),
		unison.NewTableColumnHeader("First", ""),
		unison.NewTableColumnHeader("Second", ""),
	)
	content.AddChild(header)

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
