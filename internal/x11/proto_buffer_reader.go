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
	"bytes"
	"encoding/binary"
	"io"

	"github.com/richardwilkes/toolbox/v2/errs"
)

const attemptToReadPastEndOfBufferErr = "attempt to read past end of buffer"

type protoReader interface {
	protoRead(*protoBufferReader)
}

type protoBufferReader struct {
	byteOrder binary.ByteOrder
	buffer    []byte
	pos       int
}

func newProtoBufferReader(buffer []byte) *protoBufferReader {
	return newProtoBufferReaderWithOrder(binary.LittleEndian, buffer)
}

func newProtoBufferReaderWithOrder(byteOrder binary.ByteOrder, buffer []byte) *protoBufferReader {
	return &protoBufferReader{
		byteOrder: byteOrder,
		buffer:    buffer,
	}
}

func (x *protoBufferReader) load(r io.Reader) error {
	x.pos = 0
	if _, err := io.ReadFull(r, x.buffer); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func (x *protoBufferReader) append(byteCount int, r io.Reader) error {
	i := len(x.buffer)
	buffer := make([]byte, i+byteCount)
	copy(buffer, x.buffer)
	if _, err := io.ReadFull(r, x.buffer[i:]); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

func (x *protoBufferReader) seek(index int) {
	x.pos = max(index, 0)
}

func (x *protoBufferReader) len() int {
	if x.pos >= len(x.buffer) {
		return 0
	}
	return len(x.buffer) - x.pos
}

func (x *protoBufferReader) skip(count int) {
	x.pos += count
}

func (x *protoBufferReader) skipTo4ByteAlignment() {
	x.pos += pad4(x.pos)
}

func (x *protoBufferReader) bytes(count int) []byte {
	buffer := make([]byte, count)
	x.intoBytes(buffer)
	return buffer
}

func (x *protoBufferReader) sizePrefixedBytes() []byte {
	return x.bytes(int(x.uint16()))
}

func (x *protoBufferReader) string(count int) string {
	return string(x.bytes(count))
}

func (x *protoBufferReader) sizePrefixedString() string {
	return string(x.bytes(int(x.uint16())))
}

func (x *protoBufferReader) zeroedString(count int) string {
	buffer := x.bytes(count)
	if i := bytes.IndexByte(buffer, 0); i != -1 {
		buffer = buffer[:i]
	}
	return string(buffer)
}

func (x *protoBufferReader) intoBytes(buffer []byte) {
	defer func() { x.pos += len(buffer) }()
	if x.pos+len(buffer) > len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return
	}
	copy(buffer, x.buffer[x.pos:])
}

func (x *protoBufferReader) bool() bool {
	return x.byte() != 0
}

func (x *protoBufferReader) byte() byte {
	defer func() { x.pos++ }()
	if x.pos >= len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return 0
	}
	return x.buffer[x.pos]
}

func (x *protoBufferReader) uint16() uint16 {
	defer func() { x.pos += 2 }()
	if x.pos+2 > len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return 0
	}
	return x.byteOrder.Uint16(x.buffer[x.pos:])
}

func (x *protoBufferReader) uint32() uint32 {
	defer func() { x.pos += 4 }()
	if x.pos+4 > len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return 0
	}
	return x.byteOrder.Uint32(x.buffer[x.pos:])
}

func readProtoList[T protoReader](count int, x *protoBufferReader) []T {
	list := make([]T, count)
	for i := range count {
		list[i].protoRead(x)
	}
	x.skipTo4ByteAlignment()
	return list
}
