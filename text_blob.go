// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"

	"github.com/richardwilkes/unison/internal/skia"
)

// TextBlob represents runs of text for a font, that may be drawn on a Canvas.
type TextBlob struct {
	blob skia.TextBlob
}

func newTextBlob(textBlob skia.TextBlob) *TextBlob {
	if textBlob == nil {
		return nil
	}
	tb := &TextBlob{blob: textBlob}
	runtime.AddCleanup(tb, func(sb skia.TextBlob) {
		ReleaseOnUIThread(func() {
			skia.TextBlobUnref(sb)
		})
	}, tb.blob)
	return tb
}
