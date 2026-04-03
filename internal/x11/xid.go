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

type xid struct {
	lock sync.Mutex
	base uint32
	inc  uint32
	max  uint32
	last uint32
}

func (x *xid) init(r *Reader) {
	x.base = r.Uint32()
	x.max = r.Uint32()
	x.inc = x.max & -x.max
}

func (x *xid) next(c *Conn) (uint32, error) {
	x.lock.Lock()
	defer x.lock.Unlock()
	switch {
	case x.last < x.max-x.inc+1:
		x.last += x.inc
	case c.ExtMisc.Available():
		startID, count, err := c.ExtMisc.GetXIDRange()
		if err != nil {
			return 0, err
		}
		x.last = startID
		x.max = startID + (count-1)*x.inc
	default:
		return 0, errs.New("no more IDs available")
	}
	return x.last | x.base, nil
}
