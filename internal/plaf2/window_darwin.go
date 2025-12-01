package plaf2

import "github.com/richardwilkes/unison/internal/mac"

type platformWindow struct {
	nativeWindow   mac.Window
	nsCursorHidden bool
}

func (w *Window) adjustToCursorChange() {
	if w.cursorInContentArea() {
		w.updateCursorImage()
	}
}

func (w *Window) updateCursorImage() {
	if w.cursorHidden {
		w.hideCursor()
	} else {
		w.showCursor()
		if w.cursor != nil {
			w.cursor.nativeCursor.Set()
		} else {
			mac.ArrowCursor().Set()
		}
	}
}

func (w *Window) cursorInContentArea() bool {
	view := w.platformWindow.nativeWindow.ContentView()
	return view.MouseInRect(w.platformWindow.nativeWindow.MouseLocationOutsideOfEventStream(), view.Frame())
}

func (w *Window) hideCursor() {
	if !w.platformWindow.nsCursorHidden {
		mac.HideCursor()
		w.platformWindow.nsCursorHidden = true
	}
}

func (w *Window) showCursor() {
	if w.platformWindow.nsCursorHidden {
		mac.ShowCursor()
		w.platformWindow.nsCursorHidden = false
	}
}
