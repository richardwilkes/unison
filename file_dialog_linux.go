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
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// x11RunDialogModal runs cmd (an external kdialog/zenity file dialog process) via runCmd while a hidden window runs a
// modal event loop, blocking input to this app's windows until the process finishes. It reports whether the modal loop
// ended with ModalResponseOK. The window exists only to run that loop, so it is never shown; showing it off-screen
// instead does not work under Wayland, which ignores client-requested window positions and places it on-screen as a
// tiny "phantom" window. If the window cannot be created — likely in exactly the degraded environments where these
// external fallback dialogs are used — the dialog is treated as canceled and cmd is never started. The options are
// passed through to NewWindow, letting tests force that failure path.
func x11RunDialogModal(cmd *exec.Cmd, splitOn string, runCmd func(*Window, *exec.Cmd, string), options ...WindowOption) bool {
	wnd, err := NewWindow("", options...)
	if err != nil {
		errs.Log(err, "cmd", cmd.String())
		return false
	}
	wnd.keepHidden = true
	InvokeTaskAfter(func() { go runCmd(wnd, cmd, splitOn) }, time.Millisecond)
	return wnd.RunModal() == ModalResponseOK
}
