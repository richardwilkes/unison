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
	"time"

	"golang.org/x/sys/unix"
)

// clockTicksPerSecond is the unit of the times in /proc/<pid>/stat, which the kernel fixes at 100 on every architecture
// Go supports.
const clockTicksPerSecond = 100

// processStartTime returns when the process started, and whether that could be established.
func processStartTime(pid int) (time.Time, bool) {
	data, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return time.Time{}, false
	}
	// The command name, the second field, is in parentheses and may hold spaces and parentheses of its own, so the
	// fields are counted from the last ")". The start time is the 22nd field, in clock ticks after boot.
	i := bytes.LastIndexByte(data, ')')
	if i < 0 {
		return time.Time{}, false
	}
	fields := strings.Fields(string(data[i+1:]))
	const startTimeIndex = 22 - 3
	if len(fields) <= startTimeIndex {
		return time.Time{}, false
	}
	ticks, err := strconv.ParseInt(fields[startTimeIndex], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	var sinceBoot unix.Timespec
	if err = unix.ClockGettime(unix.CLOCK_BOOTTIME, &sinceBoot); err != nil {
		return time.Time{}, false
	}
	age := time.Duration(sinceBoot.Nano()) - time.Duration(ticks)*(time.Second/clockTicksPerSecond)
	return time.Now().Add(-age), true
}
