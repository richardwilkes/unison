// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// ClipboardGetText returns text from the clipboard.
func ClipboardGetText() string {
	return apiClipboardGetText()
}

// ClipboardSetText sets text onto the clipboard, replacing the previous content.
func ClipboardSetText(text string) {
	apiClipboardSetText(text)
}

// ClipboardGetBytes returns the data associated with the specified type on the clipboard.
func ClipboardGetBytes(dataType string) []byte {
	return apiClipboardGetBytes(dataType)
}

// ClipboardSetBytes sets data onto the clipboard, replacing the previous content.
func ClipboardSetBytes(dataType string, data []byte) {
	apiClipboardSetBytes(dataType, data)
}
