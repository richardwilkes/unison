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
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// ExtMisc provides access to the XC-MISC extension.
type ExtMisc struct {
	conn  *Conn
	query *QueryExtensionReply
	lock  sync.RWMutex
}

// Available returns true if the extension is available on the server. If this returns false, no other methods on this
// object may be called.
func (e *ExtMisc) Available() bool {
	e.lock.RLock()
	q := e.query
	e.lock.RUnlock()
	if q == nil {
		q = e.conn.hasExtension("XC-MISC")
		e.lock.Lock()
		e.query = q
		e.lock.Unlock()
	}
	return q.Present
}

// GetXIDRange requests a range of unused resource IDs from the server.
func (e *ExtMisc) GetXIDRange() (*GetXIDRangeReply, error) {
	cook := newCookie(e.conn, true, true, &GetXIDRangeReply{})
	w := newProtoBufferWriter(4)
	w.byte(e.query.MajorOpcode)
	w.byte(1)
	w.uint16(1)
	e.conn.newRequest(w, cook)
	reply, err := cook.Reply()
	if err != nil {
		err = errs.Wrap(err)
	} else if reply.Count == 0 || (reply.StartID == 0 && reply.Count == 1) {
		err = errs.New("no more IDs available")
	}
	return reply, err
}

// GetXIDRangeReply holds the resource ID range data.
type GetXIDRangeReply struct {
	StartID uint32
	Count   uint32
}

func (reply *GetXIDRangeReply) protoRead(r *protoBufferReader) {
	r.skip(8)
	reply.StartID = r.uint32()
	reply.Count = r.uint32()
}
