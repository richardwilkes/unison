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

// Reader provides methods for reading data from a buffer in the format used by the X11 protocol.
type Reader struct {
	byteOrder binary.ByteOrder
	buffer    []byte
	pos       int
}

// NewReader creates a new Reader that uses Little Endian byte ordering with the specified buffer.
func NewReader(buffer []byte) *Reader {
	return NewReaderWithByteOrder(binary.LittleEndian, buffer)
}

// NewReaderWithByteOrder creates a new Reader with the specified byte order and buffer.
func NewReaderWithByteOrder(byteOrder binary.ByteOrder, buffer []byte) *Reader {
	return &Reader{
		byteOrder: byteOrder,
		buffer:    buffer,
	}
}

// Load reads data from the specified reader into the Reader's buffer, replacing any existing buffer contents, and
// resets the position to the start of the buffer. Note that the Reader expects the input data to completely fill the
// buffer, so the caller should ensure that the buffer is appropriately sized before calling Load.
func (r *Reader) Load(in io.Reader) error {
	r.pos = 0
	if _, err := io.ReadFull(in, r.buffer); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// Append reads data from the specified reader and appends it to the Reader's buffer, increasing the buffer size as
// needed, and leaves the position unchanged.
func (r *Reader) Append(byteCount int, in io.Reader) error {
	i := len(r.buffer)
	buffer := make([]byte, i+byteCount)
	copy(buffer, r.buffer)
	if _, err := io.ReadFull(in, buffer[i:]); err != nil {
		return errs.Wrap(err)
	}
	r.buffer = buffer
	return nil
}

// Seek sets the position to the specified index, or to 0 if the index is negative. Note that seeking past the end of
// the buffer is allowed, but will cause subsequent read operations to log an error and return zero values until the
// position is moved back within the buffer bounds.
func (r *Reader) Seek(index int) {
	r.pos = max(index, 0)
}

// SeekRelative advances the position by the specified amount, or moves it back if the amount is negative, but does not
// allow the position to become negative. Note that seeking past the end of the buffer is allowed, but will cause
// subsequent read operations to log an error and return zero values until the position is moved back within the buffer
// bounds.
func (r *Reader) SeekRelative(amount int) {
	r.pos += amount
	if r.pos < 0 {
		r.pos = 0
	}
}

// Remaining returns the number of bytes remaining in the buffer from the current position to the end of the buffer.
func (r *Reader) Remaining() int {
	if r.pos >= len(r.buffer) {
		return 0
	}
	return len(r.buffer) - r.pos
}

// Skip advances the position by the specified number of bytes, without reading any data. Note that skipping past the
// end of the buffer is allowed, but will cause subsequent read operations to log an error and return zero values until
// the position is moved back within the buffer bounds.
func (r *Reader) Skip(count int) {
	r.pos += count
}

// SkipTo4ByteAlignment advances the position to the next 4-byte boundary, without reading any data. Note that skipping
// past the end of the buffer is allowed, but will cause subsequent read operations to log an error and return zero
// values until the position is moved back within the buffer bounds.
func (r *Reader) SkipTo4ByteAlignment() {
	r.pos = pad4(r.pos)
}

// Bytes reads the specified number of bytes from the buffer at the current position, advances the position by that
// number of bytes, and returns the read bytes as a new slice. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and a slice will be returned containing the bytes that could be read with
// the remaining space filled with zeroes.
func (r *Reader) Bytes(count int) []byte {
	buffer := make([]byte, count)
	r.IntoBytes(buffer)
	return buffer
}

// SizePrefixedBytes reads a uint16 from the buffer at the current position to determine the byte count, then reads that
// number of bytes from the buffer, advances the position by the total number of bytes read (2 for the uint16 plus
// the byte count), and returns the read bytes as a new slice. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and a slice will be returned containing the bytes that could be read with
// the remaining space filled with zeroes.
func (r *Reader) SizePrefixedBytes() []byte {
	return r.Bytes(int(r.Uint16()))
}

// String reads the specified number of bytes from the buffer at the current position, advances the position by that
// number of bytes, and returns the read bytes as a string. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and a string will be returned containing the bytes that could be read with
// the remaining space filled with zeroes.
func (r *Reader) String(count int) string {
	return string(r.Bytes(count))
}

// SizePrefixedString reads a uint16 from the buffer at the current position to determine the byte count, then reads
// that number of bytes from the buffer, advances the position by the total number of bytes read (2 for the uint16 plus
// the byte count), and returns the read bytes as a string. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and a string will be returned containing the bytes that could be read with
// the remaining space filled with zeroes.
func (r *Reader) SizePrefixedString() string {
	return string(r.Bytes(int(r.Uint16())))
}

// ZeroedString reads the specified number of bytes from the buffer at the current position, advances the position by
// that number of bytes, and returns the read bytes as a string, excluding any trailing zero bytes. Note that if the
// read operation attempts to read past the end of the buffer, an error will be logged and a string will be returned
// containing the bytes that could be read.
func (r *Reader) ZeroedString(count int) string {
	buffer := r.Bytes(count)
	if i := bytes.IndexByte(buffer, 0); i != -1 {
		buffer = buffer[:i]
	}
	return string(buffer)
}

// IntoBytes reads bytes from the buffer at the current position into the specified byte slice, and advances the
// position by the length of the byte slice. Note that if the read operation attempts to read past the end of the
// buffer, an error will be logged and only the bytes that could be read will be copied into the byte slice.
func (r *Reader) IntoBytes(buffer []byte) {
	if len(buffer) == 0 {
		return
	}
	defer func() { r.pos += len(buffer) }()
	if r.pos < len(r.buffer) {
		copy(buffer, r.buffer[r.pos:])
	}
	if r.pos+len(buffer) > len(r.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr), "pos", r.pos, "length", len(buffer), "bufferLength", len(r.buffer))
	}
}

// Bool reads a single byte from the buffer at the current position, advances the position by one byte, and returns true
// if the read byte is non-zero, or false if the read byte is zero. Note that if the read operation attempts to read
// past the end of the buffer, an error will be logged and false will be returned.
func (r *Reader) Bool() bool {
	return r.Byte() != 0
}

// Byte reads a single byte from the buffer at the current position, advances the position by one byte, and returns the
// read byte. Note that if the read operation attempts to read past the end of the buffer, an error will be logged and
// zero will be returned.
func (r *Reader) Byte() byte {
	defer func() { r.pos++ }()
	if r.pos >= len(r.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr), "pos", r.pos, "length", 1, "bufferLength", len(r.buffer))
		return 0
	}
	return r.buffer[r.pos]
}

// Int16 reads two bytes from the buffer at the current position, advances the position by two bytes, and returns the
// read bytes as an int16 value using the Reader's byte order. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and zero will be returned.
func (r *Reader) Int16() int16 {
	return int16(r.Uint16())
}

// Int32 reads four bytes from the buffer at the current position, advances the position by four bytes, and returns the
// read bytes as an int32 value using the Reader's byte order. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and zero will be returned.
func (r *Reader) Int32() int32 {
	return int32(r.Uint32())
}

// Uint16 reads two bytes from the buffer at the current position, advances the position by two bytes, and returns the
// read bytes as a uint16 value using the Reader's byte order. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and zero will be returned.
func (r *Reader) Uint16() uint16 {
	defer func() { r.pos += 2 }()
	if r.pos+2 > len(r.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr), "pos", r.pos, "length", 2, "bufferLength", len(r.buffer))
		return 0
	}
	return r.byteOrder.Uint16(r.buffer[r.pos:])
}

// Uint32 reads four bytes from the buffer at the current position, advances the position by four bytes, and returns the
// read bytes as a uint32 value using the Reader's byte order. Note that if the read operation attempts to read past the
// end of the buffer, an error will be logged and zero will be returned.
func (r *Reader) Uint32() uint32 {
	defer func() { r.pos += 4 }()
	if r.pos+4 > len(r.buffer) {
		errs.Log(errs.New(attemptToReadPastEndOfBufferErr), "pos", r.pos, "length", 4, "bufferLength", len(r.buffer))
		return 0
	}
	return r.byteOrder.Uint32(r.buffer[r.pos:])
}

// Uint32Slice reads the specified number of uint32 values from the buffer at the current position, advances the
// position by the total number of bytes read for all values, and returns the read values as a slice.
func (r *Reader) Uint32Slice(count int) []uint32 {
	list := make([]uint32, count)
	for i := range list {
		list[i] = r.Uint32()
	}
	return list
}

// Atom is a convenience method that called Uint32() and converts the result to an Atom type.
func (r *Reader) Atom() Atom {
	return Atom(r.Uint32())
}

// ColorMapID is a convenience method that called Uint32() and converts the result to a ColorMapID type.
func (r *Reader) ColorMapID() ColorMapID {
	return ColorMapID(r.Uint32())
}

// DrawableID is a convenience method that called Uint32() and converts the result to a DrawableID type.
func (r *Reader) DrawableID() DrawableID {
	return DrawableID(r.Uint32())
}

// VisualID is a convenience method that called Uint32() and converts the result to a VisualID type.
func (r *Reader) VisualID() VisualID {
	return VisualID(r.Uint32())
}

// WindowID is a convenience method that called Uint32() and converts the result to a WindowID type.
func (r *Reader) WindowID() WindowID {
	return WindowID(r.Uint32())
}

// ReadList reads the specified number of objects from the buffer at the current position, advances the position by the
// total number of bytes read for all objects, and returns the read objects as a slice.
func (r *Reader) ReadList[T any](count int, readFunc func(*Reader) T) []T {
	list := make([]T, count)
	for i := range list {
		list[i] = readFunc(r)
	}
	r.SkipTo4ByteAlignment()
	return list
}
