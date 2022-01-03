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
	"sync"
)

var attachConsoleOnce sync.Once

// AttachConsole attempts to fix Windows console output, which is generally broken thanks to their oddball and
// unnecessary split between graphical and command-line applications. This attempts to fix that by attaching the console
// so that a graphical application can also be used as a command-line application without having to build two variants.
// May be called more than once. Subsequent calls do nothing. Does nothing on non-Windows platforms.
func AttachConsole() {
	attachConsoleOnce.Do(attachConsole)
}
