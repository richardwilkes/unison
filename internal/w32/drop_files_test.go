// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/richardwilkes/toolbox/v2/check"
)

// makeDropFiles builds a CF_HDROP buffer: a DROPFILES header whose PFiles points at a double-null-terminated list of
// UTF-16LE strings.
func makeDropFiles(pFiles uint32, fWide uint32, paths ...string) []byte {
	buf := make([]byte, dropFilesHeaderSize)
	binary.LittleEndian.PutUint32(buf[dropFilesPFilesOffset:], pFiles)
	binary.LittleEndian.PutUint32(buf[dropFilesFWideOffset:], fWide)
	for _, p := range paths {
		for _, u := range utf16.Encode([]rune(p)) {
			buf = binary.LittleEndian.AppendUint16(buf, u)
		}
		buf = binary.LittleEndian.AppendUint16(buf, 0)
	}
	return binary.LittleEndian.AppendUint16(buf, 0)
}

func TestParseDropFiles(t *testing.T) {
	c := check.New(t)
	c.Equal([]string{`C:\Users\demo\file.txt`},
		ParseDropFiles(makeDropFiles(dropFilesHeaderSize, 1, `C:\Users\demo\file.txt`)))
	c.Equal([]string{`C:\one.txt`, `D:\two two.png`, `E:\séveń.md`},
		ParseDropFiles(makeDropFiles(dropFilesHeaderSize, 1, `C:\one.txt`, `D:\two two.png`, `E:\séveń.md`)))
	// An empty list is just the terminating null.
	c.Nil(ParseDropFiles(makeDropFiles(dropFilesHeaderSize, 1)))
}

// TestParseDropFilesMalformed feeds ParseDropFiles the kinds of hostile or corrupt buffers another process can hand
// over via IDataObject. None of these may panic: FilePaths() runs inside the IDropTarget::Drop COM callback, where a Go
// panic cannot cleanly unwind through the NewCallback boundary and would crash the process.
func TestParseDropFilesMalformed(t *testing.T) {
	c := check.New(t)
	// Buffer smaller than the header.
	c.Nil(ParseDropFiles(nil))
	c.Nil(ParseDropFiles(make([]byte, dropFilesHeaderSize-1)))
	// ANSI (FWide=FALSE) lists are unsupported.
	c.Nil(ParseDropFiles(makeDropFiles(dropFilesHeaderSize, 0, `C:\one.txt`)))
	// PFiles at or past the end of the buffer.
	valid := makeDropFiles(dropFilesHeaderSize, 1, `C:\one.txt`)
	c.Nil(ParseDropFiles(makeDropFiles(uint32(len(valid)), 1, `C:\one.txt`)))
	c.Nil(ParseDropFiles(makeDropFiles(0xFFFFFFFF, 1, `C:\one.txt`)))
	// PFiles == len(data)-1: the lone trailing byte is an incomplete UTF-16 unit, leaving nothing to decode. This
	// used to panic with an index out of range when the odd-length trim emptied the slice before &remaining[0].
	c.Nil(ParseDropFiles(makeDropFiles(uint32(len(valid)-1), 1, `C:\one.txt`)))
	// An odd PFiles shifts the list off its 2-byte grid; decoding must still walk it without panicking. The
	// misaligned units no longer match the expected text, but the walk terminates at the buffer's end.
	c.NotPanics(func() { ParseDropFiles(makeDropFiles(dropFilesHeaderSize+1, 1, `C:\one.txt`)) })
	// A list missing its double-null terminator yields only the complete strings.
	trailingGarbage := append(makeDropFiles(dropFilesHeaderSize, 1, `C:\one.txt`), 'X', 0, 'Y')
	c.Equal([]string{`C:\one.txt`}, ParseDropFiles(trailingGarbage))
	// PFiles pointing inside the header is bogus but must parse without panicking.
	c.NotPanics(func() { ParseDropFiles(makeDropFiles(4, 1, `C:\one.txt`)) })
}
