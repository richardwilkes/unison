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
	"log/slog"
	"time"

	"golang.org/x/sys/windows"
)

// processIsAlive reports whether the process is still running.
//
// Windows has no signal 0, and a handle can still be opened for a process that has exited but whose handle has not yet
// been released, so neither os.FindProcess nor sending a signal answers this. Waiting on the handle with a zero timeout
// does: the object becomes signaled at the moment the process terminates.
func processIsAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		// The process is gone, or belongs to someone this one may not touch. Either way there is nothing to wait for.
		return false
	}
	defer func() {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			slog.Warn("unable to close a process handle", "error", closeErr)
		}
	}()
	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		// Its state cannot be established, so report it as running: the caller uses this only to decide whether to stop
		// waiting early, and waiting longer is always the safe answer.
		return true
	}
	return event == uint32(windows.WAIT_TIMEOUT)
}

// processStartTime returns when the process started, and whether that could be established.
func processStartTime(pid int) (time.Time, bool) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return time.Time{}, false
	}
	defer func() {
		if closeErr := windows.CloseHandle(handle); closeErr != nil {
			slog.Warn("unable to close a process handle", "error", closeErr)
		}
	}()
	var created, exited, kernel, user windows.Filetime
	if err = windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, created.Nanoseconds()), true
}
