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
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// forgetSession puts the session connection back to the state it is in before anyone has asked for it. The caller must
// already have arranged for the previous state to be restored.
func forgetSession() {
	sessionState.lock.Lock()
	sessionState.resolved = false
	sessionState.conn = nil
	sessionState.err = nil
	sessionState.lock.Unlock()
}

// These tests are not parallel, since they share the process-wide session connection and the environment.

func TestSetSessionForTest(t *testing.T) {
	c := check.New(t)
	b := newFakeBus(t)
	restore := SetSessionForTest(b.client, nil)
	conn, err := Session()
	c.NoError(err)
	c.Equal(b.client, conn)

	failure := errors.New("no bus here")
	inner := SetSessionForTest(nil, failure)
	conn, err = Session()
	c.Nil(conn)
	c.True(errors.Is(err, failure))
	inner()

	conn, err = Session()
	c.NoError(err)
	c.Equal(b.client, conn)
	restore()
}

func TestSessionCachesItsFailure(t *testing.T) {
	c := check.New(t)
	t.Cleanup(SetSessionForTest(nil, nil))
	forgetSession()
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/unison-test-bus")
	conn, first := Session()
	c.Nil(conn)
	c.HasError(first)
	c.Contains(first.Error(), "unison-test-bus")

	// The second attempt returns the cached failure without looking at the environment again.
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/nonexistent/somewhere-else")
	conn, second := Session()
	c.Nil(conn)
	c.HasError(second)
	c.Equal(first.Error(), second.Error())
}

func TestSessionAddress(t *testing.T) {
	c := check.New(t)
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/whatever")
	c.Equal("unix:path=/tmp/whatever", sessionAddress())

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/4242")
	c.Equal("unix:path=/run/user/4242/bus", sessionAddress())
	targets, err := parseAddress(sessionAddress())
	c.NoError(err)
	c.Equal([]transport{{network: unixNetwork, address: "/run/user/4242/bus"}}, targets)

	t.Setenv("XDG_RUNTIME_DIR", "")
	c.Equal("unix:path=/run/user/"+strconv.Itoa(os.Getuid())+"/bus", sessionAddress())

	// A path that needs escaping survives the round trip.
	t.Setenv("XDG_RUNTIME_DIR", "/tmp/odd dir")
	c.Equal("unix:path=/tmp/odd%20dir/bus", sessionAddress())
	targets, err = parseAddress(sessionAddress())
	c.NoError(err)
	c.Equal([]transport{{network: unixNetwork, address: "/tmp/odd dir/bus"}}, targets)
}
