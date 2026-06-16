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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/xio"
)

// This file implements just enough of the D-Bus protocol (https://dbus.freedesktop.org/doc/dbus-specification.html) to
// query and watch the XDG Desktop Portal's "color-scheme" appearance setting. It deliberately avoids pulling in a
// full D-Bus dependency. All encoding is little-endian, which matches every platform Unison runs on.

const (
	dbusTypeMethodReturn = 2
	dbusTypeSignal       = 4

	dbusFieldPath        = 1
	dbusFieldInterface   = 2
	dbusFieldMember      = 3
	dbusFieldDestination = 6
	dbusFieldSignature   = 8

	dbusColorSchemeNamespace = "org.freedesktop.appearance"
	dbusColorSchemeKey       = "color-scheme"

	dbusReadTimeout   = 5 * time.Second
	dbusMaxMessageLen = 1 << 20 // Our messages are tiny; reject anything absurd to avoid huge allocations.
)

// ReadColorScheme queries the XDG Desktop Portal for the current color scheme preference. It returns the raw value
// (0 = no preference, 1 = prefer dark, 2 = prefer light) and whether the query succeeded. A false result means the
// portal or the setting is unavailable.
func ReadColorScheme() (value uint32, ok bool) {
	c, err := dialDBus()
	if err != nil {
		return 0, false
	}
	defer c.close()
	if err = c.hello(); err != nil {
		return 0, false
	}
	var body dbusBuf
	body.str(dbusColorSchemeNamespace)
	body.str(dbusColorSchemeKey)
	if err = c.send(opMethodCall, "org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Settings", "Read", "ss", body.b); err != nil {
		return 0, false
	}
	msg, err := c.receiveReply()
	if err != nil || msg.typ != dbusTypeMethodReturn {
		return 0, false
	}
	r := dbusReader{data: msg.body}
	return r.variantUint32()
}

// WatchColorScheme subscribes to XDG Desktop Portal "SettingChanged" signals and invokes onChange with the new
// color-scheme value whenever it changes. It returns immediately; watching continues in a background goroutine that
// exits silently if the portal is unavailable or the connection drops.
func WatchColorScheme(onChange func(value uint32)) {
	go func() {
		c, err := dialDBus()
		if err != nil {
			return
		}
		defer c.close()
		if err = c.hello(); err != nil {
			return
		}
		var body dbusBuf
		body.str("type='signal',interface='org.freedesktop.portal.Settings',member='SettingChanged'")
		if err = c.send(opMethodCall, "org.freedesktop.DBus", "/org/freedesktop/DBus",
			"org.freedesktop.DBus", "AddMatch", "s", body.b); err != nil {
			return
		}
		if _, err = c.receiveReply(); err != nil {
			return
		}
		for {
			msg, readErr := c.readMessage(0) // Block indefinitely; signals arrive only when the user changes settings.
			if readErr != nil {
				return
			}
			if msg.typ != dbusTypeSignal || msg.member != "SettingChanged" {
				continue
			}
			r := dbusReader{data: msg.body}
			namespace, ok := r.str()
			if !ok || namespace != dbusColorSchemeNamespace {
				continue
			}
			key, ok := r.str()
			if !ok || key != dbusColorSchemeKey {
				continue
			}
			if value, valueOK := r.variantUint32(); valueOK {
				onChange(value)
			}
		}
	}()
}

const opMethodCall = 1

// dbusConn is a minimal authenticated connection to the session bus.
type dbusConn struct {
	conn   net.Conn
	serial uint32
}

func dialDBus() (*dbusConn, error) {
	path, err := dbusSocketPath(os.Getenv("DBUS_SESSION_BUS_ADDRESS"))
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("unix", path, dbusReadTimeout)
	if err != nil {
		return nil, err
	}
	c := &dbusConn{conn: conn}
	if err = c.authenticate(); err != nil {
		xio.CloseIgnoringErrors(conn)
		return nil, err
	}
	return c, nil
}

func (c *dbusConn) close() {
	xio.CloseIgnoringErrors(c.conn)
}

// setReadDeadline applies a read deadline, ignoring the error, which is not meaningful for our use.
func (c *dbusConn) setReadDeadline(t time.Time) {
	_ = c.conn.SetReadDeadline(t) //nolint:errcheck // deadline errors on a live connection are not actionable here
}

// dbusSocketPath extracts the Unix socket path from a D-Bus address, falling back to the well-known location.
func dbusSocketPath(addr string) (string, error) {
	if addr == "" {
		if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
			return xdg + "/bus", nil
		}
		return "/run/user/" + strconv.Itoa(os.Getuid()) + "/bus", nil
	}
	for entry := range strings.SplitSeq(addr, ";") {
		rest, isUnix := strings.CutPrefix(entry, "unix:")
		if !isUnix {
			continue
		}
		for kv := range strings.SplitSeq(rest, ",") {
			if p, found := strings.CutPrefix(kv, "path="); found {
				return p, nil
			}
			if p, found := strings.CutPrefix(kv, "abstract="); found {
				return "@" + p, nil // Leading "@" selects the Linux abstract socket namespace.
			}
		}
	}
	return "", errors.New("no usable unix D-Bus address")
}

// authenticate performs SASL EXTERNAL authentication, identifying via the process UID.
func (c *dbusConn) authenticate() error {
	uidHex := hex.EncodeToString([]byte(strconv.Itoa(os.Getuid())))
	if _, err := c.conn.Write([]byte("\x00AUTH EXTERNAL " + uidHex + "\r\n")); err != nil {
		return err
	}
	line, err := c.readLine()
	if err != nil {
		return err
	}
	if !strings.HasPrefix(line, "OK ") {
		return errors.New("D-Bus authentication rejected: " + line)
	}
	_, err = c.conn.Write([]byte("BEGIN\r\n"))
	return err
}

// readLine reads a single CRLF-terminated line, consuming exactly up to (and including) the newline so it does not
// over-read into the binary message stream that follows BEGIN.
func (c *dbusConn) readLine() (string, error) {
	c.setReadDeadline(time.Now().Add(dbusReadTimeout))
	defer c.setReadDeadline(time.Time{})
	var sb strings.Builder
	b := make([]byte, 1)
	for {
		if _, err := io.ReadFull(c.conn, b); err != nil {
			return "", err
		}
		if b[0] == '\n' {
			return strings.TrimSuffix(sb.String(), "\r"), nil
		}
		sb.WriteByte(b[0])
		if sb.Len() > 4096 {
			return "", errors.New("D-Bus authentication line too long")
		}
	}
}

// hello performs the mandatory org.freedesktop.DBus.Hello handshake.
func (c *dbusConn) hello() error {
	if err := c.send(opMethodCall, "org.freedesktop.DBus", "/org/freedesktop/DBus",
		"org.freedesktop.DBus", "Hello", "", nil); err != nil {
		return err
	}
	_, err := c.receiveReply()
	return err
}

// send marshals and writes a method call. body must already be encoded to match signature.
func (c *dbusConn) send(typ byte, dest, path, iface, member, signature string, body []byte) error {
	c.serial++

	// Header fields are an array of (byte, variant) structs, written at message offset 16, which is 8-aligned, so the
	// buffer-relative alignment used here matches the absolute alignment the protocol requires.
	var f dbusBuf
	dbusField(&f, dbusFieldPath, 'o', path)
	if iface != "" {
		dbusField(&f, dbusFieldInterface, 's', iface)
	}
	dbusField(&f, dbusFieldMember, 's', member)
	if dest != "" {
		dbusField(&f, dbusFieldDestination, 's', dest)
	}
	if signature != "" {
		dbusField(&f, dbusFieldSignature, 'g', signature)
	}

	var m dbusBuf
	m.byte('l') // Little-endian byte order.
	m.byte(typ)
	m.byte(0) // Flags.
	m.byte(1) // Protocol version.
	m.b = binary.LittleEndian.AppendUint32(m.b, uint32(len(body)))
	m.b = binary.LittleEndian.AppendUint32(m.b, c.serial)
	m.b = binary.LittleEndian.AppendUint32(m.b, uint32(len(f.b)))
	m.b = append(m.b, f.b...)
	m.align(8) // The body begins on an 8-byte boundary.
	m.b = append(m.b, body...)

	_, err := c.conn.Write(m.b)
	return err
}

// dbusMessage is a decoded incoming message; only the fields we need are retained.
type dbusMessage struct {
	member string
	body   []byte
	typ    byte
}

// receiveReply reads messages until a non-signal (method return or error) arrives.
func (c *dbusConn) receiveReply() (*dbusMessage, error) {
	for {
		msg, err := c.readMessage(dbusReadTimeout)
		if err != nil {
			return nil, err
		}
		if msg.typ != dbusTypeSignal {
			return msg, nil
		}
	}
}

// readMessage reads and decodes one message. A timeout of 0 blocks indefinitely.
func (c *dbusConn) readMessage(timeout time.Duration) (*dbusMessage, error) {
	if timeout > 0 {
		c.setReadDeadline(time.Now().Add(timeout))
		defer c.setReadDeadline(time.Time{})
	} else {
		c.setReadDeadline(time.Time{})
	}

	fixed := make([]byte, 12)
	if _, err := io.ReadFull(c.conn, fixed); err != nil {
		return nil, err
	}
	if fixed[0] != 'l' {
		return nil, errors.New("unsupported D-Bus byte order")
	}
	bodyLen := binary.LittleEndian.Uint32(fixed[4:8])

	var arrayLen [4]byte
	if _, err := io.ReadFull(c.conn, arrayLen[:]); err != nil {
		return nil, err
	}
	fieldsLen := binary.LittleEndian.Uint32(arrayLen[:])
	if fieldsLen > dbusMaxMessageLen || bodyLen > dbusMaxMessageLen {
		return nil, errors.New("D-Bus message too large")
	}

	fields := make([]byte, fieldsLen)
	if _, err := io.ReadFull(c.conn, fields); err != nil {
		return nil, err
	}
	// The body starts on an 8-byte boundary; the fixed header (12) plus the array length (4) is already 8-aligned, so
	// only the fields array length determines the padding.
	if pad := (8 - (fieldsLen % 8)) % 8; pad > 0 {
		if _, err := io.ReadFull(c.conn, make([]byte, pad)); err != nil {
			return nil, err
		}
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(c.conn, body); err != nil {
		return nil, err
	}
	return &dbusMessage{typ: fixed[1], member: dbusParseMember(fields), body: body}, nil
}

// dbusParseMember walks the header field array and returns the MEMBER field, if present.
func dbusParseMember(fields []byte) string {
	r := dbusReader{data: fields}
	var member string
	for r.remaining() > 0 {
		r.align(8) // Each (byte, variant) struct is 8-aligned.
		code, ok := r.byte()
		if !ok {
			break
		}
		sig, ok := r.sig()
		if !ok {
			break
		}
		switch sig {
		case "s", "o":
			v, vok := r.str()
			if !vok {
				return member
			}
			if code == dbusFieldMember {
				member = v
			}
		case "g":
			if _, gok := r.sig(); !gok {
				return member
			}
		case "u":
			if _, uok := r.uint32(); !uok {
				return member
			}
		default:
			return member // Unknown field type; we cannot safely skip it.
		}
	}
	return member
}

// dbusBuf accumulates a little-endian D-Bus message body or header with the protocol's alignment rules.
type dbusBuf struct {
	b []byte
}

func (w *dbusBuf) align(a int) {
	for len(w.b)%a != 0 {
		w.b = append(w.b, 0)
	}
}

func (w *dbusBuf) byte(v byte) {
	w.b = append(w.b, v)
}

func (w *dbusBuf) u32(v uint32) {
	w.align(4)
	w.b = binary.LittleEndian.AppendUint32(w.b, v)
}

// str writes a STRING/OBJECT_PATH: a 4-byte length, the bytes, and a trailing NUL.
func (w *dbusBuf) str(s string) {
	w.u32(uint32(len(s)))
	w.b = append(w.b, s...)
	w.b = append(w.b, 0)
}

// sig writes a SIGNATURE: a single-byte length, the bytes, and a trailing NUL.
func (w *dbusBuf) sig(s string) {
	w.b = append(w.b, byte(len(s)))
	w.b = append(w.b, s...)
	w.b = append(w.b, 0)
}

// dbusField writes one header field as a (byte code, variant value) struct.
func dbusField(f *dbusBuf, code, typ byte, val string) {
	f.align(8)
	f.byte(code)
	f.sig(string(typ)) // The variant's type signature.
	switch typ {
	case 'o', 's':
		f.str(val)
	case 'g':
		f.sig(val)
	}
}

// dbusReader decodes a little-endian D-Bus body. It is positioned at an 8-aligned offset, so buffer-relative
// alignment matches the protocol's absolute alignment.
type dbusReader struct {
	data []byte
	pos  int
}

func (r *dbusReader) remaining() int {
	return len(r.data) - r.pos
}

func (r *dbusReader) align(a int) {
	for r.pos%a != 0 && r.pos < len(r.data) {
		r.pos++
	}
}

func (r *dbusReader) byte() (byte, bool) {
	if r.remaining() < 1 {
		return 0, false
	}
	v := r.data[r.pos]
	r.pos++
	return v, true
}

func (r *dbusReader) uint32() (uint32, bool) {
	r.align(4)
	if r.remaining() < 4 {
		return 0, false
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, true
}

// sig reads a SIGNATURE (single-byte length, bytes, NUL).
func (r *dbusReader) sig() (string, bool) {
	n, ok := r.byte()
	if !ok || r.remaining() <= int(n) {
		return "", false
	}
	s := string(r.data[r.pos : r.pos+int(n)])
	r.pos += int(n) + 1
	return s, true
}

// str reads a STRING/OBJECT_PATH (4-byte length, bytes, NUL).
func (r *dbusReader) str() (string, bool) {
	n, ok := r.uint32()
	if !ok || r.remaining() <= int(n) {
		return "", false
	}
	s := string(r.data[r.pos : r.pos+int(n)])
	r.pos += int(n) + 1
	return s, true
}

// variantUint32 reads a variant expected to ultimately contain a uint32, transparently unwrapping the nested variant
// that org.freedesktop.portal.Settings.Read is known to return.
func (r *dbusReader) variantUint32() (uint32, bool) {
	sig, ok := r.sig()
	if !ok {
		return 0, false
	}
	switch sig {
	case "u":
		return r.uint32()
	case "v":
		return r.variantUint32()
	default:
		return 0, false
	}
}
