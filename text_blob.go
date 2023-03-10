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
	runtime.SetFinalizer(tb, func(obj *TextBlob) {
		ReleaseOnUIThread(func() {
			skia.TextBlobUnref(obj.blob)
		})
	})
	return tb
}
