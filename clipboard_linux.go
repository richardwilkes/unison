// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

func apiClipboardAvailableDataTypes() []string {
	// TODO: Implement
	return nil
}

func apiClipboardHasDataType(dataType *uti.DataType) bool {
	// TODO: Implement
	return false
}

func apiClipboardGetData(dataType *uti.DataType) []byte {
	return x11Conn.GetClipboardBytes(dataType.UTI)
}

func apiClipboardSetData(data ...drag.Data) {
	// TODO: Implement
}

func apiClipboardGetText() string {
	// TODO: Remove once the four functions above have been implemented
	return x11Conn.GetClipboardText()
}

func apiClipboardSetText(text string) {
	// TODO: Remove once the four functions above have been implemented
	x11Conn.SetClipboardText(text)
}

func apiClipboardSetBytes(dataType string, data []byte) {
	// TODO: Remove once the four functions above have been implemented
	x11Conn.SetClipboardBytes(dataType, data)
}
