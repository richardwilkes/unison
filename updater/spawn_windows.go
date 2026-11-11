// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// detach configures the command to outlive the process starting it.
//
// DETACHED_PROCESS gives the child no console at all, which is what both the helper and the relaunched application
// want: neither has anything to say to a terminal, and inheriting one would flash a console window on screen.
// CREATE_NEW_PROCESS_GROUP keeps a Ctrl-C aimed at whatever console started this from reaching the child.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}
