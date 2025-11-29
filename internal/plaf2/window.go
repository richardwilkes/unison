package plaf2

import "slices"

var windowList []*Window

type Window struct {
	cursor *Cursor
}

func (w *Window) adjustToCursorChange() {
	/* TODO
	plafAdjustToCursorChange(window);
	*/
}

func (w *Window) Destroy() {
	if w == nil {
		return
	}
	/* TODO
	   	if (window == _plaf.wndWithCurrentCtx) {
	   		plafMakeContextCurrent(NULL);
	   	}
	   _plafDestroyWindow(window);
	*/
	windowList = slices.DeleteFunc(windowList, func(wnd *Window) bool { return wnd == w })
}
