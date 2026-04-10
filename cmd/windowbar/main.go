// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package main

import (
	"fmt"
	"log/slog"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xflag"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xslog"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
)

func main() {
	xos.AppName = "Window Bar Test"
	xos.AppCmdName = "windowbar"
	xos.CopyrightStartYear = "2021"
	xos.CopyrightHolder = "Richard A. Wilkes"
	xos.AppIdentifier = "com.trollworks.unison.windowbar"
	xflag.SetUsage(nil, "Window placement test for Linux title bar positioning", "")

	logCfg := xslog.Config{Console: true}
	logCfg.AddFlags()
	xflag.Parse()

	unison.Start(unison.StartupFinishedCallback(func() {
		display := unison.PrimaryDisplay()
		if display == nil {
			xos.ExitWithMsg("no primary display detected")
		}
		wnd, err := unison.NewWindow("Window Bar Test")
		xos.ExitIfErr(err)

		content := wnd.Content()
		content.SetBorder(unison.NewEmptyBorder(geom.NewUniformInsets(12)))
		label := unison.NewLabel()
		label.SetTitle("Move/close this window after checking whether the title bar is visible")
		label.SetLayoutData(align.Middle)
		content.AddChild(label)

		wnd.Pack()
		frame := wnd.FrameRect()
		frame.Point = display.Usable.Point
		wnd.SetFrameRect(frame)
		wnd.ToFront()

		slog.Info("window placement",
			"display_frame", display.Frame,
			"display_usable", display.Usable,
			"requested_frame", frame,
			"actual_content", wnd.ContentRect(),
			"actual_frame", wnd.FrameRect(),
			"frame_insets", fmt.Sprintf("%v", wnd.FrameRect().Point.Sub(wnd.ContentRect().Point)),
			"visible", wnd.IsVisible(),
		)
	})) // Never returns
}
