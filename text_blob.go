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
	"runtime"
	"sync"

	"github.com/richardwilkes/unison/internal/skia"
)

// TextBlob represents runs of text for a font, that may be drawn on a Canvas.
type TextBlob struct {
	blob        skia.TextBlob
	cleanup     runtime.Cleanup
	disposeOnce sync.Once
}

func newTextBlob(textBlob skia.TextBlob) *TextBlob {
	if textBlob == nil {
		return nil
	}
	tb := &TextBlob{blob: textBlob}
	tb.cleanup = runtime.AddCleanup(tb, func(sb skia.TextBlob) {
		ReleaseOnUIThread(func() {
			skia.TextBlobUnref(sb)
		})
	}, tb.blob)
	return tb
}

// Dispose releases the native resource. Use this if you wish to force cleanup earlier than a gc run would normally
// trigger it.
func (tb *TextBlob) Dispose() {
	if tb == nil {
		return
	}
	tb.disposeOnce.Do(func() {
		tb.cleanup.Stop()
		if tb.blob != nil {
			skia.TextBlobUnref(tb.blob)
			tb.blob = nil
		}
	})
}
