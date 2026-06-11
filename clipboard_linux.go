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
	"slices"

	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

func apiClipboardAvailableDataTypes() []string {
	return x11Conn.ClipboardDataTypes()
}

func apiClipboardHasDataType(dataType *uti.DataType) bool {
	return slices.Contains(x11Conn.ClipboardDataTypes(), dataType.UTI)
}

func apiClipboardGetData(dataType *uti.DataType) []byte {
	return x11Conn.GetClipboardBytes(dataType.UTI)
}

func apiClipboardSetData(data ...drag.Data) {
	x11Conn.SetClipboardData(data...)
}
