// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestParseDisplayEnv verifies DISPLAY parsing, in particular that the screen separator dot is only recognized after
// the colon, since hostnames routinely contain dots (e.g. DISPLAY=myhost.example.com:0), and that the display field
// always ends up holding just the display number, which readAuthority compares against Xauthority entries.
func TestParseDisplayEnv(t *testing.T) {
	cases := []struct {
		env           string
		socket        string
		protocol      string
		host          string
		display       string
		screen        string
		displayNum    int
		defaultScreen int
		wantErr       bool
	}{
		{env: ":0", display: "0"},
		{env: ":1", display: "1", displayNum: 1},
		{env: ":0.1", display: "0", screen: "1", defaultScreen: 1},
		{env: ":2.0", display: "2", screen: "0", displayNum: 2},
		{env: ":0.", display: "0"},
		{env: "myhost:0", host: "myhost", display: "0"},
		{env: "myhost.example.com:0", host: "myhost.example.com", display: "0"},
		{env: "myhost.example.com:2.1", host: "myhost.example.com", display: "2", screen: "1", displayNum: 2, defaultScreen: 1},
		{env: "localhost:10.0", host: "localhost", display: "10", screen: "0", displayNum: 10},
		{env: "tcp/myhost:1", protocol: "tcp", host: "myhost", display: "1", displayNum: 1},
		{env: "tcp/myhost.example.com:1", protocol: "tcp", host: "myhost.example.com", display: "1", displayNum: 1},
		{env: "/run/user/1000/x11-display:0", socket: "/run/user/1000/x11-display", display: "0"},
		{env: "", wantErr: true},
		{env: "myhost", wantErr: true},
		{env: ":", wantErr: true},
		{env: ":abc", wantErr: true},
		{env: ":0.x", wantErr: true},
		{env: ":-1", wantErr: true},
		{env: ":0.-1", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.env, func(t *testing.T) {
			c := check.New(t)
			t.Setenv("DISPLAY", tc.env)
			var conn Conn
			err := conn.parseDisplayEnv()
			if tc.wantErr {
				c.HasError(err)
				return
			}
			c.NoError(err)
			c.Equal(tc.socket, conn.socket)
			c.Equal(tc.protocol, conn.protocol)
			c.Equal(tc.host, conn.host)
			c.Equal(tc.display, conn.display)
			c.Equal(tc.screen, conn.screen)
			c.Equal(tc.displayNum, conn.displayNum)
			c.Equal(tc.defaultScreen, conn.DefaultScreen)
		})
	}
}
