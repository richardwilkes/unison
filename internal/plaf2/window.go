package plaf2

import "slices"

var windowList []*Window

type Window struct {
	platformWindow      platformWindow
	platformGraphicsCtx platformGraphicsContext
	cursor              *Cursor
	cursorHidden        bool
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

func (w *Window) makeOpenGLContextCurrent() {
	w.platformGraphicsCtx.MakeCurrent()
	wndWithCurrentCtx = w
}

func (w *Window) swapBuffers() {
	w.platformGraphicsCtx.SwapBuffers()
}

func (w *Window) Destroy() { // formerly plafDestroyWindow
	if w == nil {
		return
	}
	if w == wndWithCurrentCtx {
		ClearOpenGLCurrentContext()
	}
	w.destroy()
	windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
}
