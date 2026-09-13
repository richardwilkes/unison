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
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// The values that the address tests use over and over.
const (
	unixNetwork = "unix"
	unixBusPath = "/tmp/bus"
)

func TestParseAddress(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for i, one := range []struct {
		in   string
		want []transport
	}{
		{in: "unix:path=" + unixBusPath, want: []transport{{network: unixNetwork, address: unixBusPath}}},
		{in: "unix:abstract=/tmp/dbus-AbCd", want: []transport{{network: unixNetwork, address: "@/tmp/dbus-AbCd"}}},
		{in: "unix:guid=ff00,path=" + unixBusPath, want: []transport{{network: unixNetwork, address: unixBusPath}}},
		{in: "unix:path=/tmp/x%20y", want: []transport{{network: unixNetwork, address: "/tmp/x y"}}},
		{in: "unix:path=%2ftmp%2Fbus", want: []transport{{network: unixNetwork, address: unixBusPath}}},
		{
			in: "unix:path=/tmp/one;unix:abstract=two",
			want: []transport{
				{network: unixNetwork, address: "/tmp/one"},
				{network: unixNetwork, address: "@two"},
			},
		},
		{ // An alternative that cannot be used is skipped as long as a usable one remains
			in:   "tcp:host=localhost,port=1234;unix:path=" + unixBusPath,
			want: []transport{{network: unixNetwork, address: unixBusPath}},
		},
		{in: "  ;unix:path=" + unixBusPath + ";  ", want: []transport{{network: unixNetwork, address: unixBusPath}}},
	} {
		got, err := parseAddress(one.in)
		c.NoError(err, "case %d: %s", i, one.in)
		c.Equal(one.want, got, "case %d: %s", i, one.in)
	}
}

func TestParseAddressErrors(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for i, one := range []struct {
		in      string
		message string
	}{
		{in: "", message: "contains no address"},
		{in: ";;", message: "contains no address"},
		{in: "unix", message: "does not name a transport"},
		{in: "tcp:host=localhost,port=1234", message: `the "tcp" transport`},
		{in: "unixexec:path=/bin/true", message: `the "unixexec" transport`},
		{in: "nonce-tcp:host=localhost", message: `the "nonce-tcp" transport`},
		{in: "unix:path", message: "malformed key/value pair"},
		{in: "unix:guid=ff00", message: "has no path, abstract or runtime key"},
		{in: "unix:path=/a,abstract=b", message: "more than one socket"},
		{in: "unix:runtime=no", message: "unsupported runtime value"},
		{in: "unix:path=%zz", message: "invalid escape"},
		{in: "unix:path=/tmp/%2", message: "incomplete escape"},
	} {
		got, err := parseAddress(one.in)
		c.Nil(got, "case %d: %s", i, one.in)
		c.HasError(err, "case %d: %s", i, one.in)
		c.Contains(err.Error(), one.message, "case %d: %s", i, one.in)
	}
}

func TestParseAddressRuntime(t *testing.T) { // Not parallel, since it sets the environment
	c := check.New(t)
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/4242")
	got, err := parseAddress("unix:runtime=yes")
	c.NoError(err)
	c.Equal([]transport{{network: unixNetwork, address: "/run/user/4242/bus"}}, got)
}

func TestEscapeAddressValue(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for _, one := range []struct {
		in   string
		want string
	}{
		{in: "/run/user/1000/bus", want: "/run/user/1000/bus"},
		{in: "/tmp/a-b_c.d", want: "/tmp/a-b_c.d"},
		{in: "/tmp/x y", want: "/tmp/x%20y"},
		{in: "a,b;c=d", want: "a%2cb%3bc%3dd"},
	} {
		escaped := escapeAddressValue(one.in)
		c.Equal(one.want, escaped, one.in)
		unescaped, err := unescapeAddressValue(escaped)
		c.NoError(err, one.in)
		c.Equal(one.in, unescaped, one.in)
	}
	c.Equal("unix:path=/tmp/a%20b", "unix:path="+escapeAddressValue("/tmp/a b"))
	c.NotContains(escapeAddressValue(strings.Repeat("ü", 2)), "ü")
}

func TestParseAddressTrimsEachAlternative(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	// A space after a separator must not turn the alternative that follows it into an unknown transport, and the
	// newline that a shell leaves on DBUS_SESSION_BUS_ADDRESS must not end up inside a socket path.
	got, err := parseAddress("unix:path=" + unixBusPath + "; unix:path=/tmp/other\n")
	c.NoError(err)
	c.Equal([]transport{
		{network: unixNetwork, address: unixBusPath},
		{network: unixNetwork, address: "/tmp/other"},
	}, got)
	got, err = parseAddress("\tunix:path=" + unixBusPath + "  \n")
	c.NoError(err)
	c.Equal([]transport{{network: unixNetwork, address: unixBusPath}}, got)
}
