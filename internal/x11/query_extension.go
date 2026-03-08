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
func QueryExtension(c *Conn, name string) (*QueryExtensionReply, error) {
	req := newRequest(c, true, true, &QueryExtensionReply{})
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
	return req.Reply()
}

// QueryExtensionReply holds the result of a call to QueryExtension().
type QueryExtensionReply struct {
	Present     bool
	MajorOpcode byte
	FirstEvent  byte
	FirstError  byte
}

func (q *QueryExtensionReply) protoRead(r *Reader) {
	r.Skip(8)
	q.Present = r.Bool()
	q.MajorOpcode = r.Byte()
	q.FirstEvent = r.Byte()
	q.FirstError = r.Byte()
}
