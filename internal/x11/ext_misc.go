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
	"github.com/richardwilkes/toolbox/v2/errs"
)

//nolint:unused // All available opcodes are defined here, even if not all are used by my code.
const (
	xmOpGetVersion = iota
	xmOpGetXIDRange
	xmOpGetXIDList
)

// ExtMisc provides access to the XC-MISC extension. Note that only those calls that I need have been implemented.
type ExtMisc struct {
	conn *Conn
	extensionInfo
}

func newExtMisc(conn *Conn) *ExtMisc {
	info := conn.hasExtension16("XC-MISC", 1, 1)
	return &ExtMisc{
		conn:          conn,
		extensionInfo: info,
	}
}

// GetXIDRange requests a range of unused resource IDs from the server.
func (e *ExtMisc) GetXIDRange() (startID, count uint32, err error) {
	w := NewWriter(4)
	w.Byte(e.majorOpcode)
	w.Byte(xmOpGetXIDRange)
	w.Uint16(1)
	err = e.conn.sendNewRequest(newReplyRequest(w, func(r *Reader) {
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
