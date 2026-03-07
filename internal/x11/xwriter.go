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
	"encoding/binary"
	"io"
	"slices"
)

type XWriter struct {
	byteOrder binary.ByteOrder
	buffer    []byte
}

func NewXWriter(byteOrder binary.ByteOrder, initialCapacity int) *XWriter {
	if initialCapacity <= 0 {
		initialCapacity = 1024
	}
	return &XWriter{
		byteOrder: byteOrder,
		buffer:    make([]byte, 0, initialCapacity),
	}
}

func (x *XWriter) Send(w io.Writer) error {
	_, err := w.Write(x.buffer)
	x.buffer = x.buffer[:0]
	return err
}

func (x *XWriter) Zero(count int) {
	if count > 0 {
		x.buffer = append(x.buffer, make([]byte, count)...)
	}
}

func (x *XWriter) ZeroTo4ByteAlignment() {
	x.Zero((len(x.buffer) + 3) & ^3)
}

func (x *XWriter) Bytes(v []byte) {
	x.buffer = append(x.buffer, v...)
}

func (x *XWriter) SizePrefixedBytes(v []byte) {
	x.ensureCapacity(2 + len(v))
	x.Uint16(uint16(len(v)))
	x.Bytes(v)
}

func (x *XWriter) String(s string) {
	x.buffer = append(x.buffer, []byte(s)...)
}

func (x *XWriter) SizePrefixedString(s string) {
	x.ensureCapacity(2 + len(s))
	x.Uint16(uint16(len(s)))
	x.String(s)
}

func (x *XWriter) Bool(v bool) {
	var b byte
	if v {
		b = 1
	}
	x.buffer = append(x.buffer, b)
}

func (x *XWriter) Byte(v byte) {
	x.buffer = append(x.buffer, v)
}

func (x *XWriter) Uint16(v uint16) {
	x.ensureCapacity(2)
	x.byteOrder.PutUint16(x.buffer, v)
}

func (x *XWriter) Uint32(v uint32) {
	x.ensureCapacity(4)
	x.byteOrder.PutUint32(x.buffer, v)
}

func (x *XWriter) ensureCapacity(extra int) {
	if extra -= cap(x.buffer) - len(x.buffer); extra > 0 {
		// Grow no more than 1K at a time, unless asked for more
		x.buffer = slices.Grow(x.buffer, len(x.buffer)+max(extra, min(len(x.buffer), 1024)))
	}
}
