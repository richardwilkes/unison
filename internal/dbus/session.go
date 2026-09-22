// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package dbus

import (
	"os"
	"strconv"
	"sync"
)

// sessionState holds the one connection to the desktop session bus that the whole process shares.
var sessionState struct {
	conn     *Conn
	err      error
	lock     sync.Mutex
	resolved bool
}

// Session returns the connection to the desktop session bus, making it on the first call and returning the same one
// afterwards. Failure is cached too, so asking again on a machine that has no session bus, which includes every machine
// that is not running a Linux desktop, costs nothing after the first attempt.
//
// The connection is never reconnected. Use [Conn.OnDisconnect] to learn that the session bus has gone away; there is
// nothing useful to be done about it while the process runs, since everything the session bus was being used for has
// gone away with it.
func Session() (*Conn, error) {
	sessionState.lock.Lock()
	defer sessionState.lock.Unlock()
	if !sessionState.resolved {
		sessionState.resolved = true
		sessionState.conn, sessionState.err = Dial(sessionAddress())
	}
	return sessionState.conn, sessionState.err
}

// SetSessionForTest makes [Session] return the given connection and error without contacting anything, and returns a
// function that puts back whatever was there before. It exists so that the packages built on top of this one can be
// tested against a fake bus, and should only ever be called from a test.
func SetSessionForTest(conn *Conn, err error) (restore func()) {
	sessionState.lock.Lock()
	priorResolved, priorConn, priorErr := sessionState.resolved, sessionState.conn, sessionState.err
	sessionState.resolved = true
	sessionState.conn = conn
	sessionState.err = err
	sessionState.lock.Unlock()
	return func() {
		sessionState.lock.Lock()
		sessionState.resolved, sessionState.conn, sessionState.err = priorResolved, priorConn, priorErr
		sessionState.lock.Unlock()
	}
}

// sessionAddress returns the address of the desktop session bus: what the environment says, if it says anything, and
// otherwise the well-known socket in the user's runtime directory, which is where every current implementation puts it.
func sessionAddress() string {
	if addr := os.Getenv("DBUS_SESSION_BUS_ADDRESS"); addr != "" {
		return addr
	}
	return "unix:path=" + escapeAddressValue(runtimeBusPath())
}

// runtimeBusPath returns the path of the session bus socket in the user's runtime directory.
func runtimeBusPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return dir + "/bus"
	}
	return "/run/user/" + strconv.Itoa(os.Getuid()) + "/bus"
}
