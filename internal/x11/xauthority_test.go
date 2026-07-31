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
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// fakeAddrConn is a net.Conn stand-in whose only working method is RemoteAddr, which is all readAuthority needs.
type fakeAddrConn struct {
	net.Conn
	remote net.Addr
}

func (f *fakeAddrConn) RemoteAddr() net.Addr {
	return f.remote
}

// xauthEntry serializes one Xauthority file entry: a big-endian uint16 family followed by size-prefixed address,
// display, name, and data fields.
func xauthEntry(family uint16, addr []byte, display, name string, data []byte) []byte {
	var buf bytes.Buffer
	field := func(b []byte) {
		var size [2]byte
		binary.BigEndian.PutUint16(size[:], uint16(len(b)))
		buf.Write(size[:])
		buf.Write(b)
	}
	var fam [2]byte
	binary.BigEndian.PutUint16(fam[:], family)
	buf.Write(fam[:])
	field(addr)
	field([]byte(display))
	field([]byte(name))
	field(data)
	return buf.Bytes()
}

// writeXauthorityFile writes the given entries to a temporary Xauthority file and points XAUTHORITY at it for the
// duration of the test.
func writeXauthorityFile(t *testing.T, entries ...[]byte) {
	t.Helper()
	fileName := filepath.Join(t.TempDir(), "Xauthority")
	if err := os.WriteFile(fileName, bytes.Join(entries, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XAUTHORITY", fileName)
}

// TestXauthAddressMatches exercises the per-family Xauthority address matching, including the FamilyInternet and
// FamilyInternet6 binary-IP forms that remote TCP displays store their cookies under.
func TestXauthAddressMatches(t *testing.T) {
	c := check.New(t)
	v4 := net.ParseIP("192.168.1.10")
	v6 := net.ParseIP("fd00::1234")
	// Wild entries match any connection.
	c.True(xauthAddressMatches(xauthFamilyWild, "ignored", "myhost", nil))
	c.True(xauthAddressMatches(xauthFamilyWild, "", "myhost", v4))
	// Local entries match on the hostname string.
	c.True(xauthAddressMatches(xauthFamilyLocal, "myhost", "myhost", nil))
	c.False(xauthAddressMatches(xauthFamilyLocal, "otherhost", "myhost", nil))
	// Internet entries match on the connection's remote IPv4 address, in binary form.
	c.True(xauthAddressMatches(xauthFamilyInternet, string(v4.To4()), "myhost", v4))
	c.False(xauthAddressMatches(xauthFamilyInternet, string(net.ParseIP("10.0.0.1").To4()), "myhost", v4))
	c.False(xauthAddressMatches(xauthFamilyInternet, string(v4.To4()), "myhost", nil), "local connections have no remote IP")
	c.False(xauthAddressMatches(xauthFamilyInternet, string(v4.To4()), "myhost", v6))
	// Internet6 entries match on the connection's remote IPv6 address, in binary form.
	c.True(xauthAddressMatches(xauthFamilyInternet6, string(v6.To16()), "myhost", v6))
	c.False(xauthAddressMatches(xauthFamilyInternet6, string(net.ParseIP("fd00::5678").To16()), "myhost", v6))
	c.False(xauthAddressMatches(xauthFamilyInternet6, string(v6.To16()), "myhost", v4), "an IPv4 connection must not match an IPv6 entry")
	c.False(xauthAddressMatches(xauthFamilyInternet6, string(v6.To16()), "myhost", nil))
}

// TestReadAuthorityMatchesInternetFamily is the regression test for remote TCP displays being unable to authenticate:
// their Xauthority entries are stored as FamilyInternet/FamilyInternet6 with a binary IP address, which the scan
// previously could never match (it only accepted FamilyWild and FamilyLocal), so any remote server requiring
// MIT-MAGIC-COOKIE-1 refused the connection.
func TestReadAuthorityMatchesInternetFamily(t *testing.T) {
	c := check.New(t)
	const authName = "MIT-MAGIC-COOKIE-1"
	cookie := []byte{0xde, 0xad, 0xbe, 0xef, 1, 2, 3, 4}
	writeXauthorityFile(t,
		xauthEntry(xauthFamilyLocal, []byte("otherhost"), "0", authName, []byte("wrong-local")),
		xauthEntry(xauthFamilyInternet, net.ParseIP("10.0.0.1").To4(), "0", authName, []byte("wrong-address")),
		xauthEntry(xauthFamilyInternet, net.ParseIP("192.168.1.10").To4(), "1", authName, []byte("wrong-display")),
		xauthEntry(xauthFamilyInternet, net.ParseIP("192.168.1.10").To4(), "0", authName, cookie),
	)
	conn := &Conn{
		conn:    &fakeAddrConn{remote: &net.TCPAddr{IP: net.ParseIP("192.168.1.10"), Port: 6000}},
		display: "0",
	}
	name, data := conn.readAuthority("192.168.1.10")
	c.Equal(authName, name)
	c.Equal(cookie, data)
	// A local (unix socket) connection has no remote IP, so none of the Internet entries may match it.
	local := &Conn{display: "0"}
	name, data = local.readAuthority("myhost")
	c.Equal("", name)
	c.Equal(0, len(data))
}

// TestReadAuthorityInternet6Entry verifies that IPv6 TCP connections match FamilyInternet6 entries and never the
// same-address FamilyInternet form.
func TestReadAuthorityInternet6Entry(t *testing.T) {
	c := check.New(t)
	const authName = "MIT-MAGIC-COOKIE-1"
	cookie := []byte{9, 8, 7, 6}
	ip := net.ParseIP("fd00::1234")
	writeXauthorityFile(t,
		xauthEntry(xauthFamilyInternet, ip.To16()[:4], "0", authName, []byte("wrong-family")),
		xauthEntry(xauthFamilyInternet6, ip.To16(), "0", authName, cookie),
	)
	conn := &Conn{
		conn:    &fakeAddrConn{remote: &net.TCPAddr{IP: ip, Port: 6000}},
		display: "0",
	}
	name, data := conn.readAuthority("fd00::1234")
	c.Equal(authName, name)
	c.Equal(cookie, data)
}

// TestReadAuthorityStillMatchesLocalAndWild verifies that the pre-existing FamilyLocal and FamilyWild matching still
// works after the Internet-family support was added.
func TestReadAuthorityStillMatchesLocalAndWild(t *testing.T) {
	c := check.New(t)
	const authName = "MIT-MAGIC-COOKIE-1"
	localCookie := []byte{1, 1, 2, 2}
	wildCookie := []byte{3, 3, 4, 4}
	writeXauthorityFile(t,
		xauthEntry(xauthFamilyLocal, []byte("myhost"), "0", authName, localCookie),
		xauthEntry(xauthFamilyWild, nil, "", authName, wildCookie),
	)
	conn := &Conn{display: "0"}
	name, data := conn.readAuthority("myhost")
	c.Equal(authName, name)
	c.Equal(localCookie, data)
	name, data = conn.readAuthority("someotherhost")
	c.Equal(authName, name)
	c.Equal(wildCookie, data)
}
