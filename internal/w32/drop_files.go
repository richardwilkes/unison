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
	"unicode/utf16"
)

// DROPFILES https://learn.microsoft.com/en-us/windows/win32/api/shlobj_core/ns-shlobj_core-dropfiles
type DROPFILES struct {
	PFiles uint32
	PtX    int32
	PtY    int32
	FNC    uint32 // BOOL: non-client area flag
	FWide  uint32 // BOOL: TRUE = Unicode wide chars
}

// Byte offsets of the DROPFILES fields read below, plus the total header size. The struct has no padding (five 4-byte
// fields), so these match unsafe.Offsetof/Sizeof on every platform.
const (
	dropFilesPFilesOffset = 0
	dropFilesFWideOffset  = 16
	dropFilesHeaderSize   = 20
)

// ParseDropFiles extracts file paths from a raw CF_HDROP HGLOBAL buffer. The buffer originates in another process's
// data object, so nothing about it can be trusted: the header is read field-by-field and the file list is decoded
// without reinterpreting the byte slice, ensuring a malformed or hostile PFiles offset can neither panic nor produce a
// misaligned pointer.
func ParseDropFiles(data []byte) []string {
	if len(data) < dropFilesHeaderSize {
		return nil
	}
	if binary.LittleEndian.Uint32(data[dropFilesFWideOffset:]) == 0 {
		return nil // Only support Unicode (FWide=TRUE)
	}
	offset := int(binary.LittleEndian.Uint32(data[dropFilesPFilesOffset:]))
	if offset < 0 || offset >= len(data) {
		return nil
	}
	// The file list is a double-null-terminated list of wide strings. Drop any odd trailing byte so only complete
	// UTF-16 code units are decoded.
	remaining := data[offset:]
	u16 := make([]uint16, len(remaining)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(remaining[i*2:])
	}
	var paths []string
	start := 0
	for i, v := range u16 {
		if v == 0 {
			if i == start {
				break // Double null: end of list
			}
			paths = append(paths, string(utf16.Decode(u16[start:i])))
			start = i + 1
		}
	}
	return paths
}
