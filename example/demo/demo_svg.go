// Copyright (c) 2021-2024 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package demo

import (
	_ "embed"
	"fmt"

	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
	"github.com/richardwilkes/unison/enums/paintstyle"
)

var (
	//go:embed resources/tiger.svg
	tiger      string
	tigerSVG   *unison.SVG
	svgCounter int
)

// NewDemoSVGWindow creates and displays our demo SVG window.
func NewDemoSVGWindow(where unison.Point) (*unison.Window, error) {
	// Create the window
	svgCounter++
	wnd, err := unison.NewWindow(fmt.Sprintf("SVG #%d", svgCounter))
	if err != nil {
		return nil, err
	}

	// Install our menus
	installDefaultMenus(wnd)

	content := wnd.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1})

	// Create the svg content
	var svg *unison.SVG
	if svg, err = getTigerSVG(); err != nil {
		return nil, err
	}
	panel := unison.NewPanel()
	panel.SetLayoutData(&unison.FlexLayoutData{
		MinSize: unison.NewSize(50, 50),
		HSpan:   1,
		VSpan:   1,
		HAlign:  align.Fill,
		VAlign:  align.Fill,
		HGrab:   true,
		VGrab:   true,
	})
	panel.DrawCallback = func(gc *unison.Canvas, dirty unison.Rect) {
		r := panel.ContentRect(false)
		gc.DrawRect(dirty, unison.ThemeSurface.Dark.Paint(gc, dirty, paintstyle.Fill))
		ratio := svg.AspectRatio()
		w := r.Width
		h := w / ratio
		if h > r.Height {
			h = r.Height
			w = h * ratio
		}
		svg.DrawInRect(gc, unison.NewRect((r.Width-w)/2, (r.Height-h)/2, w, h), nil, nil)
	}
	content.AddChild(panel)

	wnd.SetFrameRect(unison.Rect{Point: where, Size: unison.NewSize(400, 400)})
	wnd.ToFront()

	return wnd, nil
}

func getTigerSVG() (*unison.SVG, error) {
	if tigerSVG == nil {
		var err error
		if tigerSVG, err = unison.NewSVGFromContentString(tiger); err != nil {
			return nil, err
		}
	}
	return tigerSVG, nil
}
