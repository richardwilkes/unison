package plaf2

import "slices"

var windowList []*Window

type Window struct {
	platformWindow platformWindow
	cursor         *Cursor
	cursorHidden   bool
}

// HideCursor hides the cursor.
func (w *Window) HideCursor() {
	if !w.cursorHidden {
		w.cursorHidden = true
		w.updateCursor()
	}
}

// ShowCursor shows the cursor.
func (w *Window) ShowCursor() {
	if w.cursorHidden {
		w.cursorHidden = false
		w.updateCursor()
	}
}

func (w *Window) Destroy() { // formerly plafDestroyWindow
	if w == nil {
		return
	}
	/* TODO
	if (window == _plaf.wndWithCurrentCtx) {
		plafMakeContextCurrent(NULL);
	}
	*/
	w.destroy()
	windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
}
