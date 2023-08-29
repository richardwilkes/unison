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

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/unison"
)

// IDs for the actions
const (
	NewMenuID = unison.UserBaseID + iota
	NewWindowActionID
	NewTableWindowActionID
	NewDockWindowActionID
	OpenActionID
)

var (
	// NewWindowAction opens a new demo window when triggered.
	NewWindowAction *unison.Action
	// NewTableWindowAction opens a new demo table window when triggered.
	NewTableWindowAction *unison.Action
	// NewDockWindowAction opens a new demo dock window when triggered.
	NewDockWindowAction *unison.Action
	// OpenAction presents a file open dialog and then prints any selected files onto the console.
	OpenAction *unison.Action
)

func init() {
	NewWindowAction = &unison.Action{
		ID:         NewWindowActionID,
		Title:      "New Demo Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyN, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			// Try to position the new window to the right of the currently active window
			var pt unison.Point
			if w := unison.ActiveWindow(); w != nil {
				r := w.FrameRect()
				pt.X = r.X + r.Width
				pt.Y = r.Y
			}
			if _, err := NewDemoWindow(pt); err != nil {
				errs.Log(err)
			}
		},
	}

	NewTableWindowAction = &unison.Action{
		ID:         NewTableWindowActionID,
		Title:      "New Demo Table Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyT, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			// Try to position the new window to the right of the currently active window
			var pt unison.Point
			if w := unison.ActiveWindow(); w != nil {
				r := w.FrameRect()
				pt.X = r.X + r.Width
				pt.Y = r.Y
			}
			if _, err := NewDemoTableWindow(pt); err != nil {
				errs.Log(err)
			}
		},
	}

	NewDockWindowAction = &unison.Action{
		ID:         NewDockWindowActionID,
		Title:      "New Demo Dock Window",
		KeyBinding: unison.KeyBinding{KeyCode: unison.KeyD, Modifiers: unison.OSMenuCmdModifier()},
		ExecuteCallback: func(_ *unison.Action, _ any) {
			// Try to position the new window to the right of the currently active window
			var pt unison.Point
			if w := unison.ActiveWindow(); w != nil {
				r := w.FrameRect()
				pt.X = r.X + r.Width
				pt.Y = r.Y
			}
			if _, err := NewDemoDockWindow(pt); err != nil {
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
