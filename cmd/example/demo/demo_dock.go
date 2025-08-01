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
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/side"
)

var dockCounter int

// NewDemoDockWindow creates and displays our demo dock window.
func NewDemoDockWindow(where geom.Point) (*unison.Window, error) {
	// Create the window
	dockCounter++
	wnd, err := unison.NewWindow(fmt.Sprintf("Dock #%d", dockCounter))
	if err != nil {
		return nil, err
	}

	// Install our menus
	installDefaultMenus(wnd)

	content := wnd.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1})

	// Create the dock
	dock := unison.NewDock()
	yellowDockable := NewDockablePanel("Yellow", "", unison.Yellow)
	dock.DockTo(yellowDockable, nil, side.Left)
	dock.DockTo(NewDockablePanel("Green", "", unison.Green), unison.Ancestor[*unison.DockContainer](yellowDockable), side.Right)
	blueDockable := NewDockablePanel("Blue with a tooltip", "I've got a tooltip!", unison.Blue)
	dock.DockTo(blueDockable, nil, side.Bottom)
	unison.Ancestor[*unison.DockContainer](blueDockable).Stack(NewDockablePanel("Orange", "", unison.Orange), -1)
	dock.SetLayoutData(&unison.FlexLayoutData{
		HSpan:  1,
		VSpan:  1,
		HAlign: align.Fill,
		VAlign: align.Fill,
		HGrab:  true,
		VGrab:  true,
	})
	content.AddChild(dock)

	// Pack our window to fit its content, then set its location on the display and make it visible.
	wnd.Pack()
	rect := wnd.FrameRect()
	rect.Point = where
	wnd.SetFrameRect(rect)
	wnd.ToFront()

	return wnd, nil
}
