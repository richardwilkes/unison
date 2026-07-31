// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"os/exec"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/errs"
)

// TestX11RunDialogModalAbortsWhenWindowCreationFails is the regression test for the open and save dialogs' kdialog and
// zenity paths having only logged a NewWindow failure and then dereferenced the nil window it returned — a guaranteed
// panic in exactly the degraded environments where those external fallback dialogs run. A failing WindowOption forces
// NewWindow to fail the same way here; the dialog must report cancellation and never start the external process.
func TestX11RunDialogModalAbortsWhenWindowCreationFails(t *testing.T) {
	c := check.New(t)

	ran := false
	failOption := func(_ *Window) error { return errs.New("window creation failed") }
	ok := x11RunDialogModal(exec.Command("true"), "\n",
		func(_ *Window, _ *exec.Cmd, _ string) { ran = true }, failOption)
	c.False(ok)
	c.False(ran)
}
