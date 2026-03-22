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
)

// Writer provides methods for writing data to a buffer in the format used by the X11 protocol, and sending the buffer
// contents to a destination writer.
type Writer struct {
	byteOrder binary.ByteOrder
	buffer    []byte
}

// NewWriter creates a new Writer that uses Little Endian byte ordering with the specified initial capacity in its
// buffer.
func NewWriter(initialCapacity int) *Writer {
	return NewWriterWithByteOrder(binary.LittleEndian, initialCapacity)
}

// NewWriterWithByteOrder creates a new Writer with the specified byte ordering and initial capacity in its buffer.
func NewWriterWithByteOrder(byteOrder binary.ByteOrder, initialCapacity int) *Writer {
	if initialCapacity <= 0 {
		initialCapacity = 1024
	}
	return &Writer{
		byteOrder: byteOrder,
		buffer:    make([]byte, 0, initialCapacity),
	}
}

// Send the current buffer contents to the destination writer, then reset the buffer contents to empty.
func (w *Writer) Send(dst io.Writer) error {
	_, err := dst.Write(w.buffer)
	w.buffer = w.buffer[:0]
	return err
}

// Zero emits 'count' bytes with a value of 0 into the buffer.
func (w *Writer) Zero(count int) {
	if count > 0 {
		w.buffer = append(w.buffer, make([]byte, count)...)
	}
}

// ZeroTo4ByteAlignment emits enough zero bytes to align the buffer to a 4-byte boundary.
func (w *Writer) ZeroTo4ByteAlignment() {
	w.Zero(pad4(len(w.buffer)) - len(w.buffer))
}

// Bytes appends the specified byte slice to the buffer.
func (w *Writer) Bytes(v []byte) {
	w.buffer = append(w.buffer, v...)
}

// String appends the specified string to the buffer.
func (w *Writer) String(s string) {
	w.buffer = append(w.buffer, s...)
}

// Bool appends a single byte to the buffer with a value of 1 if the specified boolean is true, or 0 if it is false.
func (w *Writer) Bool(v bool) {
	var b byte
	if v {
		b = 1
	}
	w.buffer = append(w.buffer, b)
}

// Byte appends a single byte to the buffer.
func (w *Writer) Byte(v byte) {
	w.buffer = append(w.buffer, v)
}

// Int8 appends a single int8 value to the buffer.
func (w *Writer) Int8(v int8) {
	w.buffer = append(w.buffer, byte(v))
}

// Atom appends a uint32 value representing an Atom to the buffer using the Writer's byte order.
func (w *Writer) Atom(v Atom) {
	w.Uint32(uint32(v))
}

// WindowID appends a uint32 value representing a WindowID to the buffer using the Writer's byte order.
func (w *Writer) WindowID(v WindowID) {
	w.Uint32(uint32(v))
}

// VisualID appends a uint32 value representing a VisualID to the buffer using the Writer's byte order.
func (w *Writer) VisualID(v VisualID) {
	w.Uint32(uint32(v))
}

// Int16 appends an int16 value to the buffer using the Writer's byte order.
func (w *Writer) Int16(v int16) {
	var buf [2]byte
	w.byteOrder.PutUint16(buf[:], uint16(v))
	w.buffer = append(w.buffer, buf[:]...)
}

// Uint16 appends a uint16 value to the buffer using the Writer's byte order.
func (w *Writer) Uint16(v uint16) {
	var buf [2]byte
	w.byteOrder.PutUint16(buf[:], v)
	w.buffer = append(w.buffer, buf[:]...)
}

// Uint32 appends a uint32 value to the buffer using the Writer's byte order.
func (w *Writer) Uint32(v uint32) {
	var buf [4]byte
	w.byteOrder.PutUint32(buf[:], v)
	w.buffer = append(w.buffer, buf[:]...)
}

func pad4(size int) int {
	return (size + 3) & ^3
}
