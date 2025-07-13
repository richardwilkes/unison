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

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison"
)

// IDs for the actions
const (
	NewMenuID = unison.UserBaseID + iota
	NewWindowActionID
	NewTableWindowActionID
	NewDockWindowActionID
	NewMarkdownWindowActionID
	NewSVGWindowActionID
	ShowColorsWindowActionID
	OpenActionID
)

var (
	// NewWindowAction opens a new demo window when triggered.
	NewWindowAction *unison.Action
	// NewTableWindowAction opens a new demo table window when triggered.
	NewTableWindowAction *unison.Action
	// NewDockWindowAction opens a new demo dock window when triggered.
	NewDockWindowAction *unison.Action
	// NewMarkdownWindowAction opens a new demo markdown window when triggered.
	NewMarkdownWindowAction *unison.Action
	// NewSVGWindowAction opens a new demo SVG window when triggered.
	NewSVGWindowAction *unison.Action
	// ShowColorsWindowAction opens a demo colors window when triggered.
	ShowColorsWindowAction *unison.Action
	// OpenAction presents a file open dialog and then prints any selected files onto the console.
	OpenAction *unison.Action
)

func init() {
	NewWindowAction = &unison.Action{
		ID:         NewWindowActionID,
		Title:      "New Demo Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyN, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			if _, err := NewDemoWindow(initialWindowLocation()); err != nil {
				errs.Log(err)
			}
		},
	}

	NewTableWindowAction = &unison.Action{
		ID:         NewTableWindowActionID,
		Title:      "New Demo Table Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyT, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			if _, err := NewDemoTableWindow(initialWindowLocation()); err != nil {
				errs.Log(err)
			}
		},
	}

	NewDockWindowAction = &unison.Action{
		ID:         NewDockWindowActionID,
		Title:      "New Demo Dock Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyD, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			if _, err := NewDemoDockWindow(initialWindowLocation()); err != nil {
				errs.Log(err)
			}
		},
	}

	NewMarkdownWindowAction = &unison.Action{
		ID:         NewMarkdownWindowActionID,
		Title:      "New Demo Markdown Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyK, Modifiers: unison.ShiftModifier | unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			if _, err := NewDemoMarkdownWindow(initialWindowLocation()); err != nil {
				errs.Log(err)
			}
		},
	}

	NewSVGWindowAction = &unison.Action{
		ID:         NewSVGWindowActionID,
		Title:      "New Demo SVG Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyS, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			if _, err := NewDemoSVGWindow(initialWindowLocation()); err != nil {
				errs.Log(err)
			}
		},
	}

	ShowColorsWindowAction = &unison.Action{
		ID:         ShowColorsWindowActionID,
		Title:      "Show Colors",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyK, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			if _, err := NewDemoColorsWindow(initialWindowLocation()); err != nil {
				errs.Log(err)
			}
		},
	}

	OpenAction = &unison.Action{
		ID:         OpenActionID,
		Title:      "Open…",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyO, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			open := unison.NewOpenDialog()
			open.SetAllowsMultipleSelection(true)
			if open.RunModal() {
				fmt.Println("Paths selected:")
				for i, p := range open.Paths() {
					fmt.Printf("%d: %s\n", i, p)
				}
			}
		},
	}
}

func initialWindowLocation() geom.Point {
	// Try to position the new window to the right of the currently active window
	var pt geom.Point
	if w := unison.ActiveWindow(); w != nil {
		r := w.FrameRect()
		pt.X = r.X + r.Width
		pt.Y = r.Y
	}
	return pt
}
