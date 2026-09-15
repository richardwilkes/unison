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
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	// maxAuthLineLength is the longest authentication line that will be accepted. The specification's limit is 16384,
	// but nothing we exchange comes close, so a smaller limit keeps a broken peer from making us allocate.
	maxAuthLineLength = 4096
	// maxAuthLines is the most lines that will be read before giving up, so that a peer cannot keep us in the handshake
	// forever.
	maxAuthLines = 8
)

// authenticate performs the client side of the D-Bus SASL handshake, which precedes every message on a connection. Only
// the EXTERNAL mechanism is used, since it is the only one the session and accessibility buses need: the credentials
// travel with the Unix socket itself and the user id in the AUTH command merely confirms them. Unix file descriptor
// passing is never negotiated, as nothing here supports it.
//
// Lines are read through r, which must be the only reader of the underlying connection so that none of the message
// stream that follows BEGIN is buffered away and lost. Writes go to w, which is normally the same connection.
func authenticate(w io.Writer, r *bufio.Reader) error {
	// The leading zero byte identifies the start of the handshake and is not part of any line.
	uid := hex.EncodeToString([]byte(strconv.Itoa(os.Getuid())))
	if _, err := io.WriteString(w, "\x00AUTH EXTERNAL "+uid+"\r\n"); err != nil {
		return fmt.Errorf("dbus: unable to start authentication: %w", err)
	}
	for range maxAuthLines {
		line, err := readAuthLine(r)
		if err != nil {
			return err
		}
		command, rest, _ := strings.Cut(line, " ")
		switch command {
		case "OK":
			// rest holds the server's GUID, which is of no use to us.
			if _, err = io.WriteString(w, "BEGIN\r\n"); err != nil {
				return fmt.Errorf("dbus: unable to complete authentication: %w", err)
			}
			return nil
		case "DATA":
			// The server is asking for the identity that the AUTH command already carried, so an empty response tells
			// it to use the credentials of the socket.
			if _, err = io.WriteString(w, "DATA\r\n"); err != nil {
				return fmt.Errorf("dbus: unable to continue authentication: %w", err)
			}
		case "REJECTED":
			return fmt.Errorf("dbus: authentication was rejected; the server offers %q", rest)
		case "ERROR":
			return fmt.Errorf("dbus: authentication failed: %s", strings.TrimSpace(rest))
		default:
			return fmt.Errorf("dbus: unexpected authentication response %q", line)
		}
	}
	return errors.New("dbus: authentication did not complete")
}

// readAuthLine reads one CRLF terminated line, consuming exactly up to and including the newline.
func readAuthLine(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errors.New("dbus: the server closed the connection during authentication")
			}
			return "", fmt.Errorf("dbus: unable to read the authentication response: %w", err)
		}
		if b == '\n' {
			return strings.TrimSuffix(sb.String(), "\r"), nil
		}
		if sb.Len() >= maxAuthLineLength {
			return "", errors.New("dbus: the authentication response is too long")
		}
		sb.WriteByte(b)
	}
}
