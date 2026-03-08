// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package x11

// GetProperty returns information about the specified property.
func GetProperty(c *Conn, window WindowID, property, propertyType Atom, offset, length uint32, remove bool) (*GetPropertyReply, error) {
	req := newRequest(c, true, true, &GetPropertyReply{})
	w := NewWriter(24)
	w.Byte(20)
	w.Bool(remove)
	w.Uint16(6)
	w.Uint32(uint32(window))
	w.Uint32(uint32(property))
	w.Uint32(uint32(propertyType))
	w.Uint32(offset)
	w.Uint32(length)
	c.newRequest(w, req)
	return req.Reply()
}

// GetPropertyReply holds the result of a call to GetProperty().
type GetPropertyReply struct {
	Value      []byte
	BytesAfter uint32
	Format     byte
	Type       Atom
}

func (g *GetPropertyReply) protoRead(r *Reader) {
	r.Skip(1)
	g.Format = r.Byte()
	r.Skip(6)
	g.Type = Atom(r.Uint32())
	g.BytesAfter = r.Uint32()
	lengthInFormatUnits := r.Uint32()
	r.Skip(12)
	if g.Format != 0 {
		g.Value = r.Bytes(int(lengthInFormatUnits * uint32(g.Format/8)))
		r.Skip(pad4(len(g.Value)))
	}
}
