// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

func apiClipboardGetText() string {
	return x11Conn.GetClipboardText()
}

func apiClipboardSetText(text string) {
	x11Conn.SetClipboardText(text)
}

func apiClipboardGetBytes(dataType string) []byte {
	// TODO: Implement
	return nil
}

func apiClipboardSetBytes(dataType string, data []byte) {
	// TODO: Implement
}
