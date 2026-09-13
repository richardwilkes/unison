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
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// authPeer stands in for a server during the SASL handshake, replying with a canned script and recording what the
// client sent.
type authPeer struct {
	responses *strings.Reader
	writeErr  error
	written   bytes.Buffer
}

func (p *authPeer) Read(data []byte) (int, error) {
	return p.responses.Read(data)
}

func (p *authPeer) Write(data []byte) (int, error) {
	if p.writeErr != nil {
		return 0, p.writeErr
	}
	return p.written.Write(data)
}

func newAuthPeer(script string) *authPeer {
	return &authPeer{responses: strings.NewReader(script)}
}

// authStart is what the client always sends first: the zero byte that opens the handshake, then the EXTERNAL mechanism
// with the hex encoded user id.
func authStart() string {
	return "\x00AUTH EXTERNAL " + hex.EncodeToString([]byte(strconv.Itoa(os.Getuid()))) + "\r\n"
}

func TestAuthenticateOK(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	peer := newAuthPeer("OK 1234567890abcdef1234567890abcdef\r\n")
	c.NoError(authenticate(peer, bufio.NewReader(peer)))
	c.Equal(authStart()+"BEGIN\r\n", peer.written.String())
}

func TestAuthenticateOKWithoutCarriageReturn(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	peer := newAuthPeer("OK abc\n")
	c.NoError(authenticate(peer, bufio.NewReader(peer)))
	c.Equal(authStart()+"BEGIN\r\n", peer.written.String())
}

func TestAuthenticateWithDataChallenge(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	peer := newAuthPeer("DATA\r\nOK abc\r\n")
	c.NoError(authenticate(peer, bufio.NewReader(peer)))
	c.Equal(authStart()+"DATA\r\nBEGIN\r\n", peer.written.String())
}

func TestAuthenticateLeavesTheMessageStreamAlone(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	peer := newAuthPeer("OK abc\r\nMESSAGES")
	in := bufio.NewReader(peer)
	c.NoError(authenticate(peer, in))
	rest, err := io.ReadAll(in)
	c.NoError(err)
	c.Equal("MESSAGES", string(rest))
}

func TestAuthenticateFailures(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	for i, one := range []struct {
		script  string
		message string
	}{
		{script: "REJECTED EXTERNAL DBUS_COOKIE_SHA1\r\n", message: `rejected; the server offers "EXTERNAL`},
		{script: "ERROR not right now\r\n", message: "authentication failed: not right now"},
		{script: "AGREE_UNIX_FD\r\n", message: `unexpected authentication response "AGREE_UNIX_FD"`},
		{script: "", message: "closed the connection during authentication"},
		{script: "OK abc", message: "closed the connection during authentication"},
		{script: strings.Repeat("x", maxAuthLineLength+1), message: "response is too long"},
		{script: strings.Repeat("DATA\r\n", maxAuthLines+1), message: "did not complete"},
	} {
		peer := newAuthPeer(one.script)
		err := authenticate(peer, bufio.NewReader(peer))
		c.HasError(err, "case %d", i)
		c.Contains(err.Error(), one.message, "case %d", i)
	}
}

func TestAuthenticateWriteFailure(t *testing.T) {
	t.Parallel()
	c := check.New(t)
	peer := newAuthPeer("OK abc\r\n")
	peer.writeErr = errors.New("no")
	err := authenticate(peer, bufio.NewReader(peer))
	c.HasError(err)
	c.Contains(err.Error(), "unable to start authentication")
}
