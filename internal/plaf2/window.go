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
	KeyPressedCallback  func(ch rune, key int, mods ModifierKeys, repeated bool)
	KeyReleasedCallback func(key int, mods ModifierKeys)
	MouseEnterCallback  func()
	MouseExitCallback   func()
	MouseMovedCallback  func(pt geom.Point)
	MouseButtonCallback func(button int, pressed bool, mods ModifierKeys)
	ScrollCallback      func(delta geom.Point, pixels bool)
	ScaleCallback       func(scale geom.Point)
	DrawCallback        func()
	DropCallback        func(filePaths []string)
	CloseCallback       func()
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

func (w *Window) MakeOpenGLContextCurrent() {
	w.plGctx.MakeCurrent()
	wndWithCurrentCtx = w
}

func (w *Window) SwapBuffers() {
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
				w.keyReleased(key, 0)
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

func (w *Window) keyPressed(ch rune, key int, mods ModifierKeys) {
	repeated := w.pressedKeys[key]
	w.pressedKeys[key] = true
	if w.KeyPressedCallback != nil {
		w.KeyPressedCallback(ch, key, mods, repeated)
	}
}

func (w *Window) keyReleased(key int, mods ModifierKeys) {
	delete(w.pressedKeys, key)
	if w.KeyReleasedCallback != nil {
		w.KeyReleasedCallback(key, mods)
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

func (w *Window) SetCursor(c *Cursor) {
	if c == nil {
		w.cursor = nil
	} else {
		w.cursor = c
	}
	w.adjustToCursorChange()
}

func (w *Window) RequestClose() {
	if w.CloseCallback != nil {
		w.CloseCallback()
	} else {
		w.Destroy()
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
