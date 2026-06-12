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
	"net/url"
	"slices"
	"strings"
	"syscall"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"golang.org/x/sys/windows"
)

var _ drag.Info = &dragInfo{}

// DragTargetWindow is the interface that the drag target implementation requires from a window.
type DragTargetWindow interface {
	HWND() windows.HWND
	ConvertRawMousePoint(where geom.Point) geom.Point
	DragEntered(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op
	DragUpdated(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op
	DragExited()
	Drop(di drag.Info, where geom.Point, mods mod.Modifiers) bool
}

// DROPFILES https://learn.microsoft.com/en-us/windows/win32/api/shlobj_core/ns-shlobj_core-dropfiles
type DROPFILES struct {
	PFiles uint32
	PtX    int32
	PtY    int32
	FNC    uint32 // BOOL: non-client area flag
	FWide  uint32 // BOOL: TRUE = Unicode wide chars
}

// dragInfo implements drag.Info using a Windows IDataObject.
type dragInfo struct {
	obj    *IDataObject
	opMask drag.Op
}

func (d *dragInfo) SourceDragOpMask() drag.Op {
	return d.opMask
}

func (d *dragInfo) DataTypes() []string {
	enum := d.obj.EnumFormatEtc(DataDirGet)
	if enum == nil {
		return nil
	}
	defer enum.Release()
	var result []string
	batch := make([]FORMATETC, 16)
	for {
		n := enum.Next(batch)
		if n == 0 {
			break
		}
		for _, fe := range batch[:n] {
			name := ReverseDataType(ClipboardFormat(fe.CfFormat))
			if name != "" && !slices.Contains(result, name) {
				result = append(result, name)
			}
		}
	}
	return result
}

func (d *dragInfo) HasString() bool {
	return d.hasFormat(CFUnicodeText)
}

func (d *dragInfo) HasFilePaths() bool {
	return d.hasFormat(CFHDrop)
}

func (d *dragInfo) HasURLs() bool {
	return d.hasFormat(CFHDrop) || d.hasDataType(uti.URL.UTI)
}

func (d *dragInfo) HasDataType(dataType string) bool {
	cf := LookupDataType(dataType)
	if cf == CFNone {
		return false
	}
	return d.hasFormat(cf)
}

func (d *dragInfo) Text() string {
	data := d.getFormatData(CFUnicodeText)
	if len(data) < 2 {
		return ""
	}
	// Strip null terminator(s)
	u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&data[0])), len(data)/2)
	end := len(u16)
	for end > 0 && u16[end-1] == 0 {
		end--
	}
	return windows.UTF16ToString(u16[:end])
}

func (d *dragInfo) FilePaths() []string {
	data := d.getFormatData(CFHDrop)
	return ParseDropFiles(data)
}

func (d *dragInfo) URLs() []*url.URL {
	var result []*url.URL
	// File paths from CF_HDROP as file:// URLs
	for _, path := range d.FilePaths() {
		// Convert backslashes and build file:// URL
		urlPath := "/" + strings.ReplaceAll(path, "\\", "/")
		u, err := url.Parse("file://" + urlPath)
		if err == nil {
			result = append(result, u)
		}
	}
	// Generic URL data type
	if raw := d.Data(uti.URL.UTI); len(raw) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if u, err := url.Parse(line); err == nil {
				result = append(result, u)
			}
		}
	}
	return result
}

func (d *dragInfo) Data(dataType string) []byte {
	cf := LookupDataType(dataType)
	if cf == CFNone {
		return nil
	}
	raw := d.getFormatData(cf)
	if len(raw) == 0 {
		return nil
	}
	if uti.UTF8PlainText.ConformsTo(uti.ByUTI(dataType)) {
		// Convert from UTF-16LE to UTF-8
		if len(raw) < 2 {
			return nil
		}
		u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&raw[0])), len(raw)/2)
		end := len(u16)
		for end > 0 && u16[end-1] == 0 {
			end--
		}
		return []byte(windows.UTF16ToString(u16[:end]))
	}
	return raw
}

func (d *dragInfo) hasFormat(cf ClipboardFormat) bool {
	fe := FORMATETC{
		CfFormat: uint16(cf),
		DwAspect: DVAspectContent,
		Lindex:   -1,
		Tymed:    TyMedHGlobal,
	}
	return d.obj.QueryGetData(&fe)
}

func (d *dragInfo) hasDataType(utiStr string) bool {
	cf := LookupDataType(utiStr)
	if cf == CFNone {
		return false
	}
	return d.hasFormat(cf)
}

func (d *dragInfo) getFormatData(cf ClipboardFormat) []byte {
	fe := FORMATETC{
		CfFormat: uint16(cf),
		DwAspect: DVAspectContent,
		Lindex:   -1,
		Tymed:    TyMedHGlobal,
	}
	stg, ok := d.obj.GetData(&fe)
	if !ok {
		return nil
	}
	defer ReleaseStgMedium(&stg)
	if stg.Tymed != TyMedHGlobal || stg.Data == 0 {
		return nil
	}
	h := syscall.Handle(stg.Data)
	buf := GlobalLock(h)
	if buf == 0 {
		return nil
	}
	defer GlobalUnlock(h)
	size := GlobalSize(h)
	if size == 0 {
		return nil
	}
	data := make([]byte, size)
	copy(data, unsafe.Slice(xruntime.PtrFromUintptr[byte](buf), size))
	return data
}

// ParseDropFiles extracts file paths from a raw CF_HDROP HGLOBAL buffer.
func ParseDropFiles(data []byte) []string {
	if len(data) < int(unsafe.Sizeof(DROPFILES{})) {
		return nil
	}
	df := (*DROPFILES)(unsafe.Pointer(&data[0]))
	if df.FWide == 0 {
		return nil // Only support Unicode (FWide=TRUE)
	}
	offset := int(df.PFiles)
	if offset >= len(data) {
		return nil
	}
	// The file list is a double-null-terminated list of wide strings.
	remaining := data[offset:]
	if len(remaining)%2 != 0 {
		remaining = remaining[:len(remaining)-1]
	}
	u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&remaining[0])), len(remaining)/2)
	var paths []string
	start := 0
	for i, v := range u16 {
		if v == 0 {
			if i == start {
				break // Double null: end of list
			}
			paths = append(paths, windows.UTF16ToString(u16[start:i]))
			start = i + 1
		}
	}
	return paths
}
