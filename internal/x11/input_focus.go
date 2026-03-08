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
	req := newRequest(c, true, true, &GetInputFocusReply{})
	c.newRequest(getInputFocusRequest(), req)
	return req.Reply()
}

func getInputFocusRequest() *Writer {
	w := NewWriter(4)
	w.Byte(43)
	w.Zero(1)
	w.Uint16(1)
	return w
}

// GetInputFocusReply represents the data returned from a GetInputFocus request.
type GetInputFocusReply struct {
	Focus    WindowID
	Sequence uint16
	RevertTo byte
}

func (g *GetInputFocusReply) protoRead(r *Reader) {
	r.Skip(1)
	g.RevertTo = r.Byte()
	g.Sequence = r.Uint16()
	r.Skip(4)
	g.Focus = WindowID(r.Uint32())
}
