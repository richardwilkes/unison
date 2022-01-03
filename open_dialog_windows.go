// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unsafe"

	"github.com/richardwilkes/toolbox/i18n"
	"github.com/richardwilkes/unison/internal/w32"
)

type winOpenDialog struct {
	fileCommon
}

func platformNewOpenDialog() OpenDialog {
	d := &winOpenDialog{}
	d.initialize()
	return d
}

func (d *winOpenDialog) RunModal() bool {
	var fileNameBuffer [64 * 1024]uint16
	filter := createExtensionFilter(d.extensions)
	initialDir := utf16.Encode([]rune(d.initialDir + "\x00"))
	ofn := w32.OpenFileName{
		Size:        uint32(unsafe.Sizeof(w32.OpenFileName{})),
		FileName:    uintptr(unsafe.Pointer(&fileNameBuffer[0])),
		MaxFileName: uint32(len(fileNameBuffer)),
		Filter:      uintptr(unsafe.Pointer(&filter[0])),
		FilterIndex: 1,
		InitialDir:  uintptr(unsafe.Pointer(&initialDir[0])),
		Flags:       w32.OFNExplorer | w32.OFNPathMustExist | w32.OFNFileMustExist,
	}
	if d.allowMultipleSelection {
		ofn.Flags |= w32.OFNAllowMultiSelect
	}
	if !d.resolvesAliases {
		ofn.Flags |= w32.OFNNoDereferenceLinks
	}
	d.paths = nil
	if !w32.GetOpenFileName(&ofn) {
		return false
	}
	start := 0
	for i := range fileNameBuffer {
		if fileNameBuffer[i] == 0 {
			if start == i {
				break
			}
			d.paths = append(d.paths, string(utf16.Decode(fileNameBuffer[start:i])))
			start = i + 1
		}
	}
	switch len(d.paths) {
	case 0:
		return false
	case 1:
		lastWorkingDir = filepath.Dir(d.paths[0])
	default:
		if d.allowMultipleSelection {
			paths := make([]string, len(d.paths)-1)
			for i, p := range d.paths {
				if i != 0 {
					paths[i-1] = filepath.Join(d.paths[0], p)
				}
			}
			d.paths = paths
			lastWorkingDir = d.paths[0]
		} else {
			paths := make([]string, 1)
			paths[0] = d.paths[1]
			d.paths = paths
			lastWorkingDir = filepath.Dir(d.paths[0])
		}
	}
	return true
}

func createExtensionFilter(extensions []string) []uint16 {
	if len(extensions) == 0 {
		extensions = []string{"*"}
	}
	var buffer strings.Builder
	for _, ext := range extensions {
		if ext == "*" {
			buffer.WriteString(i18n.Text("All Files"))
			buffer.WriteByte(0)
			buffer.WriteString("*.*")
			buffer.WriteByte(0)
		} else {
			buffer.WriteString(ext)
			buffer.WriteString(i18n.Text(" Files"))
			buffer.WriteByte(0)
			buffer.WriteString("*.")
			buffer.WriteString(ext)
			buffer.WriteByte(0)
		}
	}
	buffer.WriteByte(0)
	return utf16.Encode([]rune(buffer.String()))
}
