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
	"runtime"

	"github.com/richardwilkes/toolbox/xmath/geom32"
	"github.com/richardwilkes/unison/internal/skia"
)

// Text holds text with formatting.
type Text struct {
	text skia.TextBlob
}

// NewText creates a new Text with the given Font.
func NewText(text string, font *Font) *Text {
	return newText(skia.TextBlobMakeFromText(text, font.font))
}

func newText(text skia.TextBlob) *Text {
	if text == nil {
		return nil
	}
	f := &Text{text: text}
	runtime.SetFinalizer(f, func(obj *Text) {
		ReleaseOnUIThread(func() {
			skia.TextBlobUnref(obj.text)
		})
	})
	return f
}

// Bounds returns a conservative bounding box. The returned rect may be larger than the bounds of all glyphs in runs.
func (t *Text) Bounds() geom32.Rect {
	return skia.TextBlobGetBounds(t.text).ToRect()
}

// Intercepts returns the number of intervals that intersect with start and end along the advance. For each glyph
// intercepted, two values will be returned: its left and right edge. paint may be nil.
func (t *Text) Intercepts(start, end float32, paint *Paint) []float32 {
	p := paint.paintOrNil()
	count := skia.TextBlobGetIntercepts(t.text, p, start, end, nil)
	if count == 0 {
		return nil
	}
	intervals := make([]float32, count)
	skia.TextBlobGetIntercepts(t.text, p, start, end, intervals)
	return intervals
}
