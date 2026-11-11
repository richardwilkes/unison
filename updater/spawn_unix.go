// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build !windows

package updater

import (
	"os/exec"
	"syscall"
)

// detach configures the command to outlive the process starting it.
//
// Setsid makes the child a session leader, so it has no controlling terminal and a signal aimed at this process's group
// -- a hangup when a terminal closes, or the interrupt from a Ctrl-C -- cannot reach it.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
