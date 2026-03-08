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

type protoBufferWriter struct {
	byteOrder binary.ByteOrder
	buffer    []byte
}

func newProtoBufferWriter(initialCapacity int) *protoBufferWriter {
	return newProtoBufferWriterWithOrder(binary.LittleEndian, initialCapacity)
}

func newProtoBufferWriterWithOrder(byteOrder binary.ByteOrder, initialCapacity int) *protoBufferWriter {
	if initialCapacity <= 0 {
		initialCapacity = 1024
	}
	return &protoBufferWriter{
		byteOrder: byteOrder,
		buffer:    make([]byte, 0, initialCapacity),
	}
}

func (x *protoBufferWriter) send(w io.Writer) error {
	_, err := w.Write(x.buffer)
	x.buffer = x.buffer[:0]
	return err
}

func (x *protoBufferWriter) zero(count int) {
	if count > 0 {
		x.buffer = append(x.buffer, make([]byte, count)...)
	}
}

func (x *protoBufferWriter) zeroTo4ByteAlignment() {
	x.zero(pad4(len(x.buffer)))
}

func (x *protoBufferWriter) bytes(v []byte) {
	x.buffer = append(x.buffer, v...)
}

func (x *protoBufferWriter) sizePrefixedBytes(v []byte) {
	x.ensureCapacity(2 + len(v))
	x.uint16(uint16(len(v)))
	x.bytes(v)
}

func (x *protoBufferWriter) string(s string) {
	x.buffer = append(x.buffer, []byte(s)...)
}

func (x *protoBufferWriter) sizePrefixedString(s string) {
	x.ensureCapacity(2 + len(s))
	x.uint16(uint16(len(s)))
	x.string(s)
}

func (x *protoBufferWriter) bool(v bool) {
	var b byte
	if v {
		b = 1
	}
	x.buffer = append(x.buffer, b)
}

func (x *protoBufferWriter) byte(v byte) {
	x.buffer = append(x.buffer, v)
}

func (x *protoBufferWriter) uint16(v uint16) {
	x.ensureCapacity(2)
	x.byteOrder.PutUint16(x.buffer, v)
}

func (x *protoBufferWriter) uint32(v uint32) {
	x.ensureCapacity(4)
	x.byteOrder.PutUint32(x.buffer, v)
}

func (x *protoBufferWriter) ensureCapacity(extra int) {
	if extra -= cap(x.buffer) - len(x.buffer); extra > 0 {
		// Grow no more than 1K at a time, unless asked for more
		x.buffer = slices.Grow(x.buffer, len(x.buffer)+max(extra, min(len(x.buffer), 1024)))
	}
}

func pad4(size int) int {
	return (size + 3) & ^3
}
