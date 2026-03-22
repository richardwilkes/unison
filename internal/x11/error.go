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
	"fmt"
)

// Error represents an X server error.
type Error struct {
	Name        string
	Value       uint32
	Sequence    uint16
	MinorOpcode uint16
	MajorOpcode byte
	Code        byte
}

// NewError reads an Error from the specified Reader and returns it.
func NewError(c *Conn, r *Reader) *Error {
	var e Error
	r.Skip(1)
	e.Code = r.Byte()
	e.Sequence = r.Uint16()
	e.Value = r.Uint32()
	e.MinorOpcode = r.Uint16()
	e.MajorOpcode = r.Byte()
	r.Skip(21)
	c.errorCodeLock.RLock()
	name, ok := c.errorCodeMap[e.Code]
	c.errorCodeLock.RUnlock()
	if ok {
		e.Name = name + " error"
	} else {
		e.Name = "unknown error"
	}
	return &e
}

func (e *Error) String() string {
	return e.Name
}

func (e *Error) Error() string {
	return fmt.Sprintf("X11 error code %d: %s", e.Code, e.Name)
}
