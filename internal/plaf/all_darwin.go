// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package plaf

//#include "platform.h"
import "C"

import (
	"github.com/richardwilkes/unison/internal/mac"
)

//export goAppOpenURLsCallback
func goAppOpenURLsCallback(a C.CFArrayRef) {
	if OpenFilesCallback != nil {
		if urls := mac.Array(a).ArrayOfURLToStringSlice(); len(urls) > 0 {
			OpenFilesCallback(urls)
		}
	}
}
