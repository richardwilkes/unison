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
	"bytes"
	"os"
	"strconv"
	"strings"
)

// processStartID returns a value that tells the process with identifier pid apart from any other given that identifier,
// or an empty string if that cannot be established. It is the boot the process started in and how long after that boot
// it started, neither of which setting the clock changes.
func processStartID(pid int) string {
	bootID, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return ""
	}
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return ""
	}
	// The command name, the second field, is in parentheses and may hold spaces and parentheses of its own, so the
	// fields are counted from the last ")". The start time is the 22nd field, in clock ticks after boot.
	i := bytes.LastIndexByte(data, ')')
	if i < 0 {
		return ""
	}
	fields := strings.Fields(string(data[i+1:]))
	const startTimeIndex = 22 - 3
	if len(fields) <= startTimeIndex {
		return ""
	}
	if _, err = strconv.ParseUint(fields[startTimeIndex], 10, 64); err != nil {
		return ""
	}
	return strings.TrimSpace(string(bootID)) + "/" + fields[startTimeIndex]
}
