package plaf2

import (
	"slices"
)

var windowList []*Window

type WindowConfig struct {
	Share            *Window
	Title            string
	Undecorated      bool
	NotResizable     bool
	Floating         bool
	Transparent      bool
	MousePassThrough bool
}

type Window struct {
	plWnd        platformWindow
	plGctx       platformGraphicsContext
	cursor       *Cursor
	cursorHidden bool
}

func NewWindow(cfg *WindowConfig) *Window {
	if cfg == nil {
		cfg = &WindowConfig{}
	}
	w := newWindow(cfg)
	if w == nil {
		return nil
	}
	windowList = append(windowList, w)
	return w
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
	w.plGctx.MakeCurrent()
	wndWithCurrentCtx = w
}

func (w *Window) swapBuffers() {
	w.plGctx.SwapBuffers()
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
