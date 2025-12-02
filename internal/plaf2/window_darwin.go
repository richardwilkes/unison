package plaf2

import (
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/mac"
)

type platformWindow struct {
	nativeWindow   mac.Window
	nsCursorHidden bool
}

func (w *Window) adjustToCursorChange() { // formerly plafAdjustToCursorChange
	if w.cursorInContentArea() {
		w.updateCursorImage()
	}
}

func (w *Window) updateCursorImage() { // formerly _plafUpdateCursorImage
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

func (w *Window) cursorInContentArea() bool { // formerly _plafCursorInContentArea
	view := w.platformWindow.nativeWindow.ContentView()
	return view.MouseInRect(w.platformWindow.nativeWindow.MouseLocationOutsideOfEventStream(), view.Frame())
}

func (w *Window) CursorPosition() geom.Point { // formerly plafGetCursorPos
	loc := w.platformWindow.nativeWindow.MouseLocationOutsideOfEventStream()
	frame := w.platformWindow.nativeWindow.ContentView().Frame()
	return geom.NewPoint(loc.X, frame.Height-loc.Y)
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

func (w *Window) destroy() { // formerly _plafDestroyWindow
	w.platformWindow.nativeWindow.OrderOut()
	/* TODO
	if (window->context.destroy) {
		window->context.destroy(window);
	}
	*/
	delegate := w.platformWindow.nativeWindow.Delegate()
	w.platformWindow.nativeWindow.SetDelegate(0)
	delegate.Release()
	w.platformWindow.nativeWindow.ContentView().Release()
	w.platformWindow.nativeWindow.Close()
	w.platformWindow.nativeWindow = 0
	/* TODO
	plafPollEvents();
	*/
}
