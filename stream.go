// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"io"

	"github.com/richardwilkes/toolbox/errs"
	"github.com/richardwilkes/unison/internal/skia"
)

// Stream provides a way to use different streams for the same purpose. These streams currently only exist for Skia's
// PDF support, which requires a stream to write to.
type Stream interface {
	asWStream() skia.WStream
}

// MemoryStream provides a way to write data to a buffer. This exists for PDF output, but can be used for other things.
type MemoryStream struct {
	memory skia.DynamicMemoryWStream
}

// NewMemoryStream creates a new stream that writes to memory.
func NewMemoryStream() *MemoryStream {
	return &MemoryStream{memory: skia.DynamicMemoryWStreamNew()}
}

func (s *MemoryStream) asWStream() skia.WStream {
	return skia.DynamicMemoryWStreamAsWStream(s.memory)
}

func (s *MemoryStream) Write(data []byte) (n int, err error) {
	current := s.BytesWritten()
	if !skia.DynamicMemoryWStreamWrite(s.memory, data) {
		err = io.ErrShortWrite
	}
	return s.BytesWritten() - current, err
}

// BytesWritten returns the number of bytes written so far.
func (s *MemoryStream) BytesWritten() int {
	return skia.DynamicMemoryWStreamBytesWritten(s.memory)
}

// Bytes returns the bytes that have been written.
func (s *MemoryStream) Bytes() []byte {
	buffer := make([]byte, s.BytesWritten())
	skia.DynamicMemoryWStreamRead(s.memory, buffer)
	return buffer
}

// Close the stream. Further writes should not be done.
func (s *MemoryStream) Close() {
	skia.DynamicMemoryWStreamDelete(s.memory)
}

// FileStream provides a way to write data to a file. This exists for PDF output, but can be used for other things.
type FileStream struct {
	file skia.FileWStream
}

// NewFileStream creates a new stream that writes to a file.
func NewFileStream(filePath string) (*FileStream, error) {
	s := &FileStream{file: skia.FileWStreamNew(filePath)}
	if s.file == nil {
		return nil, errs.New("unable to create file stream at " + filePath)
	}
	return s, nil
}

func (s *FileStream) asWStream() skia.WStream {
	return skia.FileWStreamAsWStream(s.file)
}

func (s *FileStream) Write(data []byte) (n int, err error) {
	current := s.BytesWritten()
	if !skia.FileWStreamWrite(s.file, data) {
		err = io.ErrShortWrite
	}
	return s.BytesWritten() - current, err
}

// BytesWritten returns the number of bytes written so far.
func (s *FileStream) BytesWritten() int {
	return skia.FileWStreamBytesWritten(s.file)
}

// Flush the stream to disk. Note that the underlying skia code does not return any errors from this operation, yet
// there is the potential for that to occur, since any buffered but not written bytes may not be able to be written. If
// this is a concern, use a MemoryStream instead and use Go code to write the result.
func (s *FileStream) Flush() {
	skia.FileWStreamFlush(s.file)
}

// Close the stream. Further writes should not be done. Note that the underlying skia code does not return any errors
// from this operation, yet there is the potential for that to occur, since any buffered but not written bytes may not
// be able to be written. If this is a concern, use a MemoryStream instead and use Go code to write the result.
func (s *FileStream) Close() {
	skia.FileWStreamDelete(s.file)
}
