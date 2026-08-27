// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// This file is the headless testing example from README.md, verbatim: everything below the package clause is what the
// README shows, and TestHeadlessReadmeExampleIsCurrent fails if the two ever differ. Being compiled and run here is
// what keeps the example honest. Edit the example here and copy it into the README, not the other way around, so that
// gofumpt keeps it formatted.

package unison_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
	"github.com/richardwilkes/unison/enums/align"
)

func TestButton(t *testing.T) {
	var button *unison.Button
	clicks := 0
	screen, err := unison.StartHeadless(unison.HeadlessConfig{Width: 400, Height: 300},
		unison.StartupFinishedCallback(func() {
			button = unison.NewButton()
			button.SetTitle("Press Me")
			button.ClickCallback = func() { clicks++ }
			button.SetLayoutData(&unison.FlexLayoutData{HAlign: align.Fill, VAlign: align.Fill, HGrab: true, VGrab: true})
			wnd, wndErr := unison.NewWindow("example")
			if wndErr != nil {
				t.Error(wndErr)
				return
			}
			wnd.Content().SetLayout(&unison.FlexLayout{Columns: 1})
			wnd.Content().AddChild(button)
			wnd.SetContentRect(geom.NewRect(20, 20, 200, 80))
			wnd.ToFront()
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Stop)

	screen.Click(screen.PanelCenter(button))
	var count int
	screen.Do(func() { count = clicks })
	if count != 1 {
		t.Errorf("expected 1 click, got %d", count)
	}

	f, err := os.Create(filepath.Join(t.TempDir(), "button.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(f, screen.Capture()); err != nil { // Capture() returns an *image.NRGBA of the whole screen
		t.Error(err)
	}
	if err = f.Close(); err != nil {
		t.Error(err)
	}
}
