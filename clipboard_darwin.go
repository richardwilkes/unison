// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import "github.com/richardwilkes/unison/internal/mac"

func apiClipboardGetText() string {
	return mac.PasteboardString()
}

func apiClipboardSetText(text string) {
	mac.SetPasteboardString(text)
}

func apiClipboardGetBytes(dataType string) []byte {
	return mac.PasteboardBytes(dataType)
}

func apiClipboardSetBytes(dataType string, data []byte) {
	mac.SetPasteboardBytes(dataType, data)
}
