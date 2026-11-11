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
	"net"
	"strconv"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
)

const (
	// waitTimeout bounds how long the helper will wait for the application to exit before giving up. Giving up means
	// abandoning the update with nothing touched, so a generous bound costs only a delayed update, while too short a
	// one abandons updates whenever quitting takes a while.
	waitTimeout = 90 * time.Second
	// waitPollInterval is how often the port or process is checked. The wait is normally over within a second or two of
	// the application exiting, so this is short enough to be imperceptible.
	waitPollInterval = 50 * time.Millisecond
	// pidGraceInterval is how long to wait before believing that a departed process left the port to someone else. A
	// socket outlives its process by a moment, so checking immediately would mistake that for another instance.
	pidGraceInterval = 2 * time.Second
)

// waitForPredecessor blocks until the application that staged this update has exited.
//
// When the application has a single-instance handoff port, it waits on that port rather than on the process, because
// that is the resource that governs what happens next: a new instance started while the port is still held hands its
// arguments to whoever holds it and exits immediately, so relaunching too early would look exactly like the update
// having done nothing. Being able to bind the port answers "can the replacement become the primary instance?" directly,
// and unlike watching a process identifier it cannot be fooled by that identifier being reused.
//
// The listener is closed as soon as it is obtained rather than held through the swap. Holding it would make any copy of
// the application the user launches meanwhile connect to the helper and stall on a handoff that will never be answered.
// Releasing it leaves a sub-second window in which something else could take the port, which the relaunched instance
// has to recover from on its own, by retrying the bind.
//
// With no port, it waits on the process identifier alone; see waitForProcess.
func waitForPredecessor(pid, port int, appName string) error {
	if port <= 0 {
		return waitForProcess(pid, appName)
	}
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(waitTimeout)
	started := time.Now()
	for time.Now().Before(deadline) {
		listener, err := net.Listen("tcp4", address)
		if err == nil {
			xio.CloseIgnoringErrors(listener)
			return nil
		}
		// A port held by something else entirely -- a second installation of the application, say -- would otherwise
		// stall this until the timeout. Once the process being waited for is definitively gone, the port is no longer
		// evidence about it.
		if time.Since(started) > pidGraceInterval && !processExists(pid) {
			slog.Warn("the application has exited but its port is held by something else; continuing", "port", port)
			return nil
		}
		time.Sleep(waitPollInterval)
	}
	return errs.Newf("the running copy of %s did not exit within %s", appName, waitTimeout)
}

// waitForProcess blocks until the process with the given identifier has exited. A zero or negative identifier means the
// state file carried no process to watch, so there is nothing to wait for. Unlike the port, the identifier can be
// reused by an unrelated process once the application has exited, which can only make this wait longer, never end
// early.
func waitForProcess(pid int, appName string) error {
	if pid <= 0 {
		return nil
	}
	deadline := time.Now().Add(waitTimeout)
	for time.Now().Before(deadline) {
		if !processIsAlive(pid) {
			return nil
		}
		time.Sleep(waitPollInterval)
	}
	return errs.Newf("the running copy of %s did not exit within %s", appName, waitTimeout)
}

// processExists reports whether a process with the given identifier is still around. A zero or negative identifier
// means the state file carried no process to watch, so it reports true and the port alone decides.
//
// This is only ever a tiebreaker for a port held by something other than what is being waited for, so erring towards
// "still running" is the safe direction: it means waiting longer, never swapping too early.
func processExists(pid int) bool {
	if pid <= 0 {
		return true
	}
	return processIsAlive(pid)
}
