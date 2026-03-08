// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// GetInputFocus returns the current focus state.
func GetInputFocus(c *Conn) (*GetInputFocusReply, error) {
	cook := newCookie(c, true, true, &GetInputFocusReply{})
	c.newRequest(getInputFocusRequest(), cook)
	return cook.Reply()
}

func getInputFocusRequest() *protoBufferWriter {
	w := newProtoBufferWriter(4)
	w.byte(43)
	w.zero(1)
	w.uint16(1)
	return w
}

// GetInputFocusReply represents the data returned from a GetInputFocus request.
type GetInputFocusReply struct {
	Focus    WindowID
	Sequence uint16
	RevertTo byte
}

func (reply *GetInputFocusReply) protoRead(r *protoBufferReader) {
	r.skip(1)
	reply.RevertTo = r.byte()
	reply.Sequence = r.uint16()
	r.skip(4)
	reply.Focus = WindowID(r.uint32())
}
