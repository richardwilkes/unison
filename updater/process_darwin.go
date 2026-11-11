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
	"strconv"

	"golang.org/x/sys/unix"
)

// processStartID returns a value that tells the process with identifier pid apart from any other given that identifier,
// or an empty string if that cannot be established. It is when the system recorded the process starting, which setting
// the clock afterwards does not change.
func processStartID(pid int) string {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(info.Proc.P_starttime.Nano(), 10)
}
