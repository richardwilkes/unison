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
	"context"
	"encoding/binary"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// Conn represents a connection to an X server.
type Conn struct {
	conn          net.Conn
	SetupInfo     SetupInfo
	DefaultScreen int
}

type connectionInfo struct {
	envDisplay string
	socket     string
	protocol   string
	host       string
	display    string
	screen     string
	displayNum int
}

func OpenDisplay() (*Conn, error) {
	var c Conn
	info, err := c.parseDisplayEnv()
	if err != nil {
		return nil, err
	}
	if err = c.connect(info); err != nil {
		return nil, err
	}
	if err = c.authenticate(info); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Conn) parseDisplayEnv() (*connectionInfo, error) {
	const invalidDisplayErr = "invalid DISPLAY environment variable: "
	var info connectionInfo
	info.envDisplay = os.Getenv("DISPLAY")
	colon := strings.LastIndex(info.envDisplay, ":")
	if colon < 0 {
		return nil, errs.New(invalidDisplayErr + info.envDisplay)
	}
	if info.envDisplay[0] == '/' {
		info.socket = info.envDisplay[0:colon]
	} else {
		if slash := strings.LastIndex(info.envDisplay, "/"); slash >= 0 {
			info.protocol = info.envDisplay[0:slash]
			info.host = info.envDisplay[slash+1 : colon]
		} else {
			info.host = info.envDisplay[0:colon]
		}
	}
	id := info.envDisplay[colon+1:]
	if id == "" {
		return nil, errs.New(invalidDisplayErr + info.envDisplay)
	}
	dot := strings.LastIndex(info.envDisplay, ".")
	if dot < 0 {
		info.display = info.envDisplay[0:]
	} else {
		info.display = info.envDisplay[0:dot]
		if info.screen = info.envDisplay[dot+1:]; info.screen != "" {
			var err error
			if c.DefaultScreen, err = strconv.Atoi(info.screen); err != nil {
				return nil, errs.New(invalidDisplayErr + info.envDisplay)
			}
		}
	}
	var err error
	if info.displayNum, err = strconv.Atoi(info.display); err != nil || info.displayNum < 0 {
		return nil, errs.New(invalidDisplayErr + info.envDisplay)
	}
	if info.host == "" || info.host == "localhost" {
		if info.host, err = os.Hostname(); err != nil {
			return nil, errs.NewWithCause("cannot determine hostname", err)
		}
	}
	return &info, nil
}

func (c *Conn) connect(info *connectionInfo) error {
	var err error
	switch {
	case info.socket != "":
		c.conn, err = net.Dial("unix", info.socket+":"+info.display)
	case info.host != "" && info.host != "unix":
		if info.protocol == "" {
			info.protocol = "tcp"
		}
		c.conn, err = net.Dial(info.protocol, info.host+":"+strconv.Itoa(6000+info.displayNum))
	default:
		c.conn, err = net.Dial("unix", "/tmp/.X11-unix/X"+info.display)
	}
	if err != nil {
		return errs.NewWithCause("unable to connect to X server with DISPLAY "+info.envDisplay, err)
	}
	return nil
}

func (c *Conn) authenticate(info *connectionInfo) error {
	authName, authData, err := c.readAuthority(info)
	if err != nil {
		errs.LogWithLevel(context.Background(), slog.LevelWarn, slog.Default(), err)
	} else if authName != "MIT-MAGIC-COOKIE-1" || len(authData) != 16 {
		return errs.New("unsupported auth protocol: " + authName)
	}
	w := NewXWriter(binary.LittleEndian, 18+len(authName)+len(authData))
	w.Byte(0x6C) // Use little endian
	w.Zero(1)
	w.Uint16(11) // Major version
	w.Uint16(0)  // Minor version
	w.Uint16(uint16(len(authName)))
	w.Uint16(uint16(len(authData)))
	w.Zero(2)
	w.String(authName)
	w.ZeroTo4ByteAlignment()
	w.Bytes(authData)
	w.ZeroTo4ByteAlignment()
	if err = w.Send(c.conn); err != nil {
		return errs.NewWithCause("failed to send authentication data", err)
	}
	r, err := NewXReaderWithLoad(binary.LittleEndian, 8, c.conn)
	if err != nil {
		return errs.NewWithCause("failed to read authentication response header", err)
	}
	code := r.Byte()
	reasonLen := r.Byte()
	major := r.Uint16()
	minor := r.Uint16()
	dataLen := r.Uint16() * 4
	if major != 11 || minor != 0 {
		return errs.Newf("unsupported X protocol version: %d.%d", major, minor)
	}
	if r, err = NewXReaderWithLoad(binary.LittleEndian, int(dataLen), c.conn); err != nil {
		return errs.NewWithCause("failed to read authentication response data", err)
	}
	switch code {
	case 0:
		return errs.New("authentication refused: " + r.String(int(reasonLen)))
	case 1:
		c.SetupInfo.Read(r)
		return nil
	case 2:
		return errs.New("further authentication required: " + r.ZeroedString(int(dataLen)))
	default:
		return errs.Newf("unexpected response code: %d", code)
	}
}

func (c *Conn) readAuthority(info *connectionInfo) (name string, data []byte, err error) {
	fileName := os.Getenv("XAUTHORITY")
	if fileName == "" {
		if fileName = os.Getenv("HOME"); fileName == "" {
			return "", nil, errs.New("cannot determine Xauthority file location")
		}
		fileName += "/.Xauthority"
	}
	r, err := NewXReaderWithFile(binary.BigEndian, fileName)
	if err != nil {
		return "", nil, errs.NewWithCause("failed to load Xauthority file", err)
	}
	for r.Len() != 0 {
		family := r.Uint16()
		addr := r.SizePrefixedString()
		disp := r.SizePrefixedString()
		name = r.SizePrefixedString()
		data = r.SizePrefixedBytes()
		if ((family == 65535) || (family == 256 && addr == info.host)) &&
			((disp == "") || (disp == info.display)) {
			return name, data, nil
		}
	}
	return "", nil, errs.NewWithCause("failed to find target in Xauthority file", err)
}

func pad4(n int) int {
	return (n + 3) & ^3
}
