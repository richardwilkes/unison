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
	"os"

	"github.com/richardwilkes/toolbox/v2/errs"
)

const attemptToReadPastEndOfBufferErr = "attempt to read past end of buffer"

type Readable interface {
	Read(*XReader)
}

type XReader struct {
	byteOrder binary.ByteOrder
	buffer    []byte
	pos       int
}

func NewXReader(byteOrder binary.ByteOrder, buffer []byte) *XReader {
	return &XReader{
		byteOrder: byteOrder,
		buffer:    buffer,
	}
}

func NewXReaderWithLoad(byteOrder binary.ByteOrder, size int, r io.Reader) (*XReader, error) {
	x := NewXReader(byteOrder, make([]byte, size))
	err := x.Load(r)
	return x, err
}

func NewXReaderWithFile(byteOrder binary.ByteOrder, filePath string) (*XReader, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return NewXReader(byteOrder, fileData), nil
}

// Load the underlying buffer from the provided reader and reset the position to 0.
func (x *XReader) Load(r io.Reader) error {
	x.pos = 0
	if _, err := io.ReadFull(r, x.buffer); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// Len returns the number of bytes of the unread portion of the buffer.
func (x *XReader) Len() int {
	if x.pos >= len(x.buffer) {
		return 0
	}
	return len(x.buffer) - x.pos
}

func (x *XReader) Skip(count int) {
	x.pos += count
}

func (x *XReader) SkipTo4ByteAlignment() {
	x.pos += (x.pos + 3) & ^3
}

func (x *XReader) Bytes(count int) []byte {
	buffer := make([]byte, count)
	x.IntoBytes(buffer)
	return buffer
}

func (x *XReader) SizePrefixedBytes() []byte {
	return x.Bytes(int(x.Uint16()))
}

func (x *XReader) String(count int) string {
	return string(x.Bytes(count))
}

func (x *XReader) SizePrefixedString() string {
	return string(x.Bytes(int(x.Uint16())))
}

func (x *XReader) ZeroedString(count int) string {
	buffer := x.Bytes(count)
	if i := bytes.IndexByte(buffer, 0); i != -1 {
		buffer = buffer[:i]
	}
	return string(buffer)
}

func (x *XReader) IntoBytes(buffer []byte) {
	defer func() { x.pos += len(buffer) }()
	if x.pos+len(buffer) > len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return
	}
	copy(buffer, x.buffer[x.pos:])
}

func (x *XReader) Bool() bool {
	return x.Byte() != 0
}

func (x *XReader) Byte() byte {
	defer func() { x.pos++ }()
	if x.pos >= len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return 0
	}
	return x.buffer[x.pos]
}

func (x *XReader) Uint16() uint16 {
	defer func() { x.pos += 2 }()
	if x.pos+2 > len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return 0
	}
	return x.byteOrder.Uint16(x.buffer[x.pos:])
}

func (x *XReader) Uint32() uint32 {
	defer func() { x.pos += 4 }()
	if x.pos+4 > len(x.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr))
		return 0
	}
	return x.byteOrder.Uint32(x.buffer[x.pos:])
}

func ReadList[T Readable](count int, x *XReader) []T {
	list := make([]T, count)
	for i := range count {
		list[i].Read(x)
	}
	x.SkipTo4ByteAlignment()
	return list
}
