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

// Asking the owner of the CLIPBOARD selection for anything is a filtered wait on its reply, which may swallow the
// wake-up a pending redraw is riding on, so each of the calls below repairs that bookkeeping on its way out. Setting
// the clipboard needs no such repair: it claims the selection and stores the data, and waits for nothing. See
// x11FilteredWaitDone.

func nativeClipboardAvailableDataTypes() []string {
	defer x11FilteredWaitDone()
	return x11Conn.ClipboardDataTypes()
}

func nativeClipboardHasDataType(dataType *uti.DataType) bool {
	defer x11FilteredWaitDone()
	return slices.Contains(x11Conn.ClipboardDataTypes(), dataType.UTI)
}

func nativeClipboardGetData(dataType *uti.DataType) []byte {
	defer x11FilteredWaitDone()
	return x11Conn.GetClipboardBytes(dataType.UTI)
}

func nativeClipboardSetData(data ...drag.Data) {
	x11Conn.SetClipboardData(data...)
}
