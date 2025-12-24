package plaf2

import (
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
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
	SizeCallback        func()
	PosCallback         func()
	MinimizeCallback    func(minimized bool)
	MaximizeCallback    func(maximized bool)
	FocusCallback       func(focused bool)
	KeyCallback         func(key, code int, pressed, repeated bool, mods ModifierKeys)
	MouseMovedCallback  func(pt geom.Point)
	MouseButtonCallback func(button int, pressed bool, mods ModifierKeys)
	DrawCallback        func()
	plWnd               platformWindow
	plGctx              platformGraphicsContext
	cursor              *Cursor
	pressedKeys         map[int]bool
	pressedButtons      map[int]bool
	width               float32
	height              float32
	cursorHidden        bool
	maximized           bool
}

func NewWindow(cfg *WindowConfig) *Window {
	if cfg == nil {
		cfg = &WindowConfig{}
	}
	w := newWindow(cfg)
	if w == nil {
		return nil
	}
	w.pressedKeys = make(map[int]bool)
	w.pressedButtons = make(map[int]bool)
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

func (w *Window) notifyOfFocusChange(focused bool) { // formerly _plafNotifyOfFocusChange
	if w.FocusCallback != nil {
		w.FocusCallback(focused)
	}
	if !focused {
		if len(w.pressedKeys) != 0 {
			keys := make([]int, 0, len(w.pressedKeys))
			for key := range w.pressedKeys {
				keys = append(keys, key)
			}
			for _, key := range keys {
				w.inputKey(key, scanCodes[key], false, 0)
			}
		}
		if len(w.pressedButtons) != 0 {
			buttons := make([]int, 0, len(w.pressedButtons))
			for button := range w.pressedButtons {
				buttons = append(buttons, button)
			}
			for _, button := range buttons {
				w.mouseClick(button, false, 0)
			}
		}
	}
}

func (w *Window) inputKey(key, code int, pressed bool, mods ModifierKeys) { // formerly _plafInputKey
	previous := w.pressedKeys[key]
	if !pressed && !previous {
		return
	}
	repeated := pressed && previous
	if pressed {
		w.pressedKeys[key] = true
	} else {
		delete(w.pressedKeys, key)
	}
	if w.KeyCallback != nil {
		w.KeyCallback(key, code, pressed, repeated, mods)
	}
}

func (w *Window) mouseMoved(pt geom.Point) { // formerly _plafInputCursorPos
	if w.MouseMovedCallback != nil {
		w.MouseMovedCallback(pt)
	}
}

func (w *Window) mouseClick(button int, pressed bool, mods ModifierKeys) { // formerly _plafInputMouseClick
	w.pressedButtons[button] = pressed
	if w.MouseButtonCallback != nil {
		w.MouseButtonCallback(button, pressed, mods)
	}
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
