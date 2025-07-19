// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/behavior"
)

const topLevelRowsToMake = 100

var tableCounter int

// NewDemoTableWindow creates and displays our demo table window.
func NewDemoTableWindow(where geom.Point) (*unison.Window, error) {
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
	table := unison.NewTable(&unison.SimpleTableModel[*demoRow]{})
	table.HierarchyColumnID = 1
	table.Columns = make([]unison.ColumnInfo, 4)
	for i := range table.Columns {
		table.Columns[i].ID = i
		table.Columns[i].Minimum = 20
		table.Columns[i].Maximum = 10000
	}
	_, checkColSize, _ := unison.NewCheckBox().Sizes(geom.Size{})
	table.Columns[0].Minimum = checkColSize.Width
	table.Columns[0].Maximum = checkColSize.Width
	rows := make([]*demoRow, topLevelRowsToMake)
	for i := range rows {
		row := &demoRow{
			table: table,
			id:    tid.MustNewTID('a'),
			text:  fmt.Sprintf("Row %d", i+1),
			text2: fmt.Sprintf("Some longer content for Row %d", i+1),
		}
		if i%10 == 3 {
			if i == 3 {
				row.doubleHeight = true
			}
			row.container = true
			row.open = true
			row.children = make([]*demoRow, 5)
			for j := range row.children {
				child := &demoRow{
					table:  table,
					parent: row,
					id:     tid.MustNewTID('a'),
					text:   fmt.Sprintf("Sub Row %d", j+1),
				}
				row.children[j] = child
				if j < 2 {
					child.container = true
					child.open = true
					child.children = make([]*demoRow, 2)
					for k := range child.children {
						child.children[k] = &demoRow{
							table:  table,
							parent: child,
							id:     tid.MustNewTID('a'),
							text:   fmt.Sprintf("Sub Sub Row %d", k+1),
						}
					}
				}
			}
		}
		rows[i] = row
	}
	table.SetRootRows(rows)
	table.SizeColumnsToFit(true)
	table.InstallDragSupport(nil, "demoRow", "Row", "Rows")
	unison.InstallDropSupport[*demoRow, any](table, "demoRow",
		func(from, to *unison.Table[*demoRow]) bool { return from == to }, nil, nil)

	header := unison.NewTableHeader(table,
		unison.NewTableColumnHeader[*demoRow]("", "", nil),
		unison.NewTableColumnHeader[*demoRow]("First", "", nil),
		unison.NewTableColumnHeader[*demoRow]("Second", "", nil),
		unison.NewTableColumnHeader[*demoRow]("xyz", "", nil),
	)
	header.SetLayoutData(&unison.FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
	})

	// Create a scroll panel and place a table panel inside it
	scrollArea := unison.NewScrollPanel()
	scrollArea.SetContent(table, behavior.Fill, behavior.Fill)
	scrollArea.SetColumnHeader(header)
	scrollArea.SetLayoutData(&unison.FlexLayoutData{
		HAlign: align.Fill,
		VAlign: align.Fill,
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
