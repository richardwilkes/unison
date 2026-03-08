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
	cook := newCookie(c, true, true, &QueryExtensionReply{})
	size := 8 + pad4(len(name))
	w := newProtoBufferWriter(size)
	w.byte(98)
	w.zero(1)
	w.uint16(uint16(size / 4))
	w.uint16(uint16(len(name)))
	w.zero(2)
	w.string(name)
	w.zeroTo4ByteAlignment()
	c.newRequest(w, cook)
	return cook.Reply()
}

// QueryExtensionReply holds the result of a call to QueryExtension().
type QueryExtensionReply struct {
	Present     bool
	MajorOpcode byte
	FirstEvent  byte
	FirstError  byte
}

func (reply *QueryExtensionReply) protoRead(r *protoBufferReader) {
	r.skip(8)
	reply.Present = r.bool()
	reply.MajorOpcode = r.byte()
	reply.FirstEvent = r.byte()
	reply.FirstError = r.byte()
}
