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
	"github.com/richardwilkes/unison/enums/behavior"
)

//go:embed sample_markdown.md
var sampleMarkdown string

var markdownCounter int

// NewDemoMarkdownWindow creates and displays our demo markdown window.
func NewDemoMarkdownWindow(where unison.Point) (*unison.Window, error) {
	// Create the window
	markdownCounter++
	wnd, err := unison.NewWindow(fmt.Sprintf("Markdown #%d", markdownCounter))
	if err != nil {
		return nil, err
	}

	// Install our menus
	installDefaultMenus(wnd)

	content := wnd.Content()
	content.SetLayout(&unison.FlexLayout{Columns: 1})
	content.SetBorder(unison.NewEmptyBorder(unison.NewSymmetricInsets(unison.StdHSpacing, unison.StdVSpacing)))

	// Create the markdown view
	markdown := unison.NewMarkdown(true)
	markdown.SetContent(sampleMarkdown, 0)

	// Create a scroll panel and place a table panel inside it
	scrollArea := unison.NewScrollPanel()
	scrollArea.SetContent(markdown, behavior.Fill, behavior.Fill)
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
