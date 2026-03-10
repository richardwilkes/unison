// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// QueryExtension returns information about the specified extension.
func QueryExtension(c *Conn, name string) (present bool, majorOpcode, firstEvent, firstError byte, err error) {
	req := newRequest(c, true, true, func(r *Reader) {
		r.Skip(8)
		present = r.Bool()
		majorOpcode = r.Byte()
		firstEvent = r.Byte()
		firstError = r.Byte()
	})
	size := 8 + pad4(len(name)) - len(name)
	w := NewWriter(size)
	w.Byte(98)
	w.Zero(1)
	w.Uint16(uint16(size / 4))
	w.Uint16(uint16(len(name)))
	w.Zero(2)
	w.String(name)
	w.ZeroTo4ByteAlignment()
	c.newRequest(w, req)
	if err = req.Reply(); err != nil {
		return false, 0, 0, 0, err
	}
	return present, majorOpcode, firstEvent, firstError, nil
}
