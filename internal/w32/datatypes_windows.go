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
	"sync"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/uti"
)

var (
	dataTypeMapLock sync.RWMutex
	dataTypeMap     = map[string]ClipboardFormat{
		uti.UTF8PlainText.UTI: CFUnicodeText,
	}
	reverseDataTypeMap = map[ClipboardFormat]string{
		CFUnicodeText: uti.UTF8PlainText.UTI,
	}
)

// LookupDataType returns the ClipboardFormat for a UTI string, registering it if needed.
func LookupDataType(dataType string) ClipboardFormat {
	dataTypeMapLock.RLock()
	f, ok := dataTypeMap[dataType]
	dataTypeMapLock.RUnlock()
	if ok {
		return f
	}
	if f = RegisterClipboardFormatW(dataType); f == CFNone {
		errs.Log(errs.Newf("unable to register clipboard format %q", dataType))
		return CFNone
	}
	dataTypeMapLock.Lock()
	dataTypeMap[dataType] = f
	reverseDataTypeMap[f] = dataType
	dataTypeMapLock.Unlock()
	return f
}

// ReverseDataType returns the UTI string for a ClipboardFormat, or "".
func ReverseDataType(cf ClipboardFormat) string {
	dataTypeMapLock.RLock()
	name, ok := reverseDataTypeMap[cf]
	dataTypeMapLock.RUnlock()
	if ok {
		return name
	}
	if name = GetClipboardFormatNameW(cf); name != "" {
		return name
	}
	return ""
}
