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
	"github.com/richardwilkes/toolbox/cmdline"
	"github.com/richardwilkes/toolbox/log/jot"
	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison"
)

var aboutWindow *unison.Window

// ShowAboutWindow displays the about window, creating it if necessary.
func ShowAboutWindow(item unison.MenuItem) {
	// Is our about window already open?
	if aboutWindow == nil {

		// Nope, so create it.
		var err error
		aboutWindow, err = unison.NewWindow(item.Title(), unison.NotResizableWindowOption())
		if err != nil {
			jot.Error(err)
			return
		}

		// Clear our global when the window closes
		aboutWindow.WillCloseCallback = func() { aboutWindow = nil }

		// Put some empty space around the edges of our window and apply a single column layout.
		content := aboutWindow.Content()
		content.SetBorder(unison.NewEmptyBorder(geom32.NewUniformInsets(24)))
		content.SetLayout(&unison.FlexLayout{
			Columns:  1,
			HSpacing: unison.StdHSpacing,
			VSpacing: unison.StdVSpacing,
		})

		// Put the name of the app in a large font at the top
		title := unison.NewLabel()
		title.Text = cmdline.AppName
		title.Font = unison.EmphasizedSystemFont.ResolvedFont().Face().Font(24)
		title.SetLayoutData(&unison.FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: unison.MiddleAlignment,
			VAlign: unison.MiddleAlignment,
			HGrab:  true,
		})
		content.AddChild(title)

		// Put a description below the title, line 1
		desc := unison.NewLabel()
		desc.Text = "A simple app to demonstrate"
		desc.Font = unison.LabelFont.ResolvedFont().Face().Font(14)
		desc.SetLayoutData(&unison.FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: unison.MiddleAlignment,
			VAlign: unison.MiddleAlignment,
			HGrab:  true,
		})
		content.AddChild(desc)

		// Put a description below the title, line 2
		desc = unison.NewLabel()
		desc.Text = "the capabilities of Unison"
		desc.Font = unison.LabelFont.ResolvedFont().Face().Font(14)
		desc.SetLayoutData(&unison.FlexLayoutData{
			HSpan:  1,
			VSpan:  1,
			HAlign: unison.MiddleAlignment,
			VAlign: unison.MiddleAlignment,
			HGrab:  true,
		})
		content.AddChild(desc)

		// Pack our window to fit its content, then center it on the main display.
		aboutWindow.Pack()
		wndFrame := aboutWindow.FrameRect()
		frame := unison.PrimaryDisplay().Usable
		frame.Y += (frame.Height - wndFrame.Height) / 3
		frame.Height = wndFrame.Height
		frame.X += (frame.Width - wndFrame.Width) / 2
		frame.Width = wndFrame.Width
		frame.Align()
		aboutWindow.SetFrameRect(frame)
	}

	// Make it visible and in the front.
	aboutWindow.ToFront()
}
