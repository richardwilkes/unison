// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// OpenBrowser asks the operating system to open url in the browser. Use it rather than xos.OpenBrowser, since a
// headless session records the request for HeadlessScreen.OpenedURLs() instead of launching a browser. May be called
// from any goroutine.
func OpenBrowser(url string) error {
	return apiOpenBrowser(url)
}
