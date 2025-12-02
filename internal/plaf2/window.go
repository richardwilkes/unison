package plaf2

import "slices"

var windowList []*Window

type Window struct {
	platformWindow platformWindow
	cursor         *Cursor
	cursorHidden   bool
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
