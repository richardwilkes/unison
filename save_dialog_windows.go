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
	"unicode/utf16"
	"unsafe"

	"github.com/richardwilkes/unison/internal/w32"
)

type winSaveDialog struct {
	fileCommon
}

func platformNewSaveDialog() SaveDialog {
	d := &winSaveDialog{}
	d.initialize()
	return d
}

func (d *winSaveDialog) RunModal() bool {
	var fileNameBuffer [64 * 1024]uint16
	filter := createExtensionFilter(d.extensions)
	initialDir := utf16.Encode([]rune(d.initialDir + "\x00"))
	d.paths = nil
	if w32.GetSaveFileName(&w32.OpenFileName{
		Size:        uint32(unsafe.Sizeof(w32.OpenFileName{})),
		FileName:    uintptr(unsafe.Pointer(&fileNameBuffer[0])),
		MaxFileName: uint32(len(fileNameBuffer)),
		Filter:      uintptr(unsafe.Pointer(&filter[0])),
		FilterIndex: 1,
		InitialDir:  uintptr(unsafe.Pointer(&initialDir[0])),
		Flags:       w32.OFNExplorer | w32.OFNPathMustExist | w32.OFNNoTestFileCreate | w32.OFNOverwritePrompt,
	}) {
		return false
	}
	for i := range fileNameBuffer {
		if fileNameBuffer[i] == 0 {
			d.paths = append(d.paths, string(utf16.Decode(fileNameBuffer[:i])))
			break
		}
	}
	if len(d.paths) != 0 {
		lastWorkingDir = filepath.Dir(d.paths[0])
	}
	return true
}
