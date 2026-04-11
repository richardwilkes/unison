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
	"log/slog"
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// Opcodes for XC-MISC requests.
const (
	XCMiscGetVersionOpCode = iota
	XCMiscGetXIDRangeOpCode
	XCMiscGetXIDListOpCode
)

// ExtMisc provides access to the XC-MISC extension. Note that only those calls that I need have been implemented.
type ExtMisc struct {
	conn *Conn
	lock sync.RWMutex
	extensionInfo
}

// Available determines if the extension is available on the server. No other methods on this object may be called if
// false is returned for available.
func (e *ExtMisc) Available() (available bool, majorVersion, minorVersion uint32) {
	e.lock.RLock()
	info := e.extensionInfo
	e.lock.RUnlock()
	if !info.checked {
		info = e.conn.hasExtension("XC-MISC")
		w := NewWriter(8)
		w.Byte(info.majorOpcode)
		w.Byte(XCMiscGetVersionOpCode)
		w.Uint16(2)
		w.Uint32(1) // Major version max
		w.Uint32(1) // Minor version max
		if err := e.conn.sendNewRequest(newReplyRequest("XCMiscGetVersion", w, func(r *Reader) {
			r.Skip(8)
			info.majorVersion = uint32(r.Uint16())
			info.minorVersion = uint32(r.Uint16())
			r.Skip(20)
		})); err != nil {
			slog.Error("failed to get XC-MISC version", "error", err)
		}
		e.lock.Lock()
		e.extensionInfo = info
		e.lock.Unlock()
	}
	return info.present, info.majorVersion, info.minorVersion
}

// GetXIDRange requests a range of unused resource IDs from the server.
func (e *ExtMisc) GetXIDRange() (startID, count uint32, err error) {
	w := NewWriter(4)
	w.Byte(e.majorOpcode)
	w.Byte(XCMiscGetXIDRangeOpCode)
	w.Uint16(1)
	err = e.conn.sendNewRequest(newReplyRequest("XCMiscGetXIDRange", w, func(r *Reader) {
		r.Skip(8)
		startID = r.Uint32()
		count = r.Uint32()
		r.Skip(16)
	}))
	if err == nil && (count == 0 || (startID == 0 && count == 1)) {
		err = errs.New("no more IDs available")
	}
	return startID, count, err
}
