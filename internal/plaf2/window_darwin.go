package plaf2

import (
	"log/slog"
	"slices"

	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/mac"
)

type platformWindow struct {
	wnd            mac.Window
	view           mac.View
	nsCursorHidden bool
}

func findWindowByNSWindow(macWnd mac.Window) *Window {
	if i := slices.IndexFunc(windowList, func(w *Window) bool {
		return w.plWnd.wnd == macWnd
	}); i != -1 {
		return windowList[i]
	}
	return nil
}

func initWindowCallbacks() {
	mac.WindowKeyPressedCallback = func(macWnd mac.Window, ch rune, key int, mods uint) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.keyPressed(ch, keyCodes[key], translateFlags(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key pressed callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowKeyReleasedCallback = func(macWnd mac.Window, key int, mods uint) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.keyReleased(keyCodes[key], translateFlags(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window key released callback for unknown window", "window", macWnd)
		}
	}
	// mac.WindowInputFlagsCallback = func(macWnd mac.Window, keyCode int, mods uint) {
	// 	if w := findWindowByNSWindow(macWnd); w != nil {
	// 		flags := translateFlags(mac.EventModifierFlags(mods))
	// 		key := keyCodes[keyCode]
	// 		var keyFlag ModifierKeys
	// 		switch key {
	// 		case KeyLeftShift:
	// 		case KeyRightShift:
	// 			keyFlag = ModKeyShift
	// 		case KeyLeftControl:
	// 		case KeyRightControl:
	// 			keyFlag = ModKeyControl
	// 		case KeyLeftAlt:
	// 		case KeyRightAlt:
	// 			keyFlag = ModKeyAlt
	// 		case KeyLeftSuper:
	// 		case KeyRightSuper:
	// 			keyFlag = ModKeySuper
	// 		case KeyCapsLock:
	// 			keyFlag = ModKeyCapsLock
	// 		}
	// 		pressed := false
	// 		if (keyFlag&flags) != 0 && !w.pressedKeys[key] {
	// 			pressed = true
	// 		}
	// 		w.inputKey(key, keyCode, pressed, flags)
	// 	} else {
	// 		slog.Warn("received window input flags callback for unknown window", "window", macWnd)
	// 	}
	// }
	mac.WindowShouldCloseCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.RequestClose()
		} else {
			slog.Warn("received window should close callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidResizeCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.plGctx.ctx.Update()
			maximized := w.plWnd.wnd.Zoomed()
			if w.maximized != maximized {
				w.maximized = maximized
				if w.MaximizeCallback != nil {
					w.MaximizeCallback(maximized)
				}
			}
			r := w.plWnd.view.Frame()
			if r.Width != w.width || r.Height != w.height {
				w.width = r.Width
				w.height = r.Height
				if w.SizeCallback != nil {
					w.SizeCallback()
				}
			}
		} else {
			slog.Warn("received window did resize callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidMoveCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.plGctx.ctx.Update()
			if w.PosCallback != nil {
				w.PosCallback()
			}
		} else {
			slog.Warn("received window did move callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMinimizeCallback = func(macWnd mac.Window, minimized bool) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.MinimizeCallback != nil {
				w.MinimizeCallback(minimized)
			}
		} else {
			slog.Warn("received window minimize callback for unknown window", "window", macWnd, "minimized", minimized)
		}
	}
	mac.WindowDidBecomeKeyCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.notifyOfFocusChange(true)
			if w.cursorInContentArea() {
				w.updateCursorImage()
			}
		} else {
			slog.Warn("received window did become key callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowDidResignKeyCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.notifyOfFocusChange(false)
		} else {
			slog.Warn("received window did resign key callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowCursorUpdateCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.updateCursorImage()
		} else {
			slog.Warn("received window cursor update callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseEnterCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.updateCursorImage()
			if w.MouseEnterCallback != nil {
				w.MouseEnterCallback()
			}
		} else {
			slog.Warn("received window mouse enter callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseExitCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.updateCursorImage()
			if w.MouseExitCallback != nil {
				w.MouseExitCallback()
			}
		} else {
			slog.Warn("received window mouse exit callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseMovedCallback = func(macWnd mac.Window, pt geom.Point) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.mouseMoved(pt)
		} else {
			slog.Warn("received window mouse moved callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowScrollCallback = func(macWnd mac.Window, deltaX, deltaY float32, pixels bool) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.ScrollCallback != nil {
				w.ScrollCallback(geom.NewPoint(deltaX, deltaY), pixels)
			}
		} else {
			slog.Warn("received window scroll callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowMouseClickCallback = func(macWnd mac.Window, button int, pressed bool, mods uint) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.mouseClick(button, pressed, translateFlags(mac.EventModifierFlags(mods)))
		} else {
			slog.Warn("received window mouse click callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowUpdateLayerCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			w.plGctx.ctx.Update()
			if w.DrawCallback != nil {
				w.DrawCallback()
			}
		} else {
			slog.Warn("received window update layer callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowRedrawCallback = func(macWnd mac.Window) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.DrawCallback != nil {
				w.DrawCallback()
			}
		} else {
			slog.Warn("received window draw rect callback for unknown window", "window", macWnd)
		}
	}
	mac.WindowScaleCallback = func(macWnd mac.Window, scale geom.Point) {
		// This will be called once before the window is finished initializing, so just ignore any unknown windows here.
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.ScaleCallback != nil {
				w.ScaleCallback(scale)
			}
		}
	}
	mac.WindowDropCallback = func(macWnd mac.Window, filePaths []string) {
		if w := findWindowByNSWindow(macWnd); w != nil {
			if w.DropCallback != nil {
				w.DropCallback(filePaths)
			}
		} else {
			slog.Warn("received window drop callback for unknown window", "window", macWnd)
		}
	}
}

func newWindow(cfg *WindowConfig) *Window {
	styleMask := mac.WindowStyleMaskMiniaturizable
	if cfg.Undecorated {
		styleMask |= mac.WindowStyleMaskBorderless
	} else {
		styleMask |= mac.WindowStyleMaskTitled | mac.WindowStyleMaskClosable
		if !cfg.NotResizable {
			styleMask |= mac.WindowStyleMaskResizable
		}
	}
	w := mac.NewWindow(geom.NewRect(0, 0, 1, 1), styleMask, true, true)
	if w == 0 {
		return nil
	}
	if cfg.NotResizable {
		w.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenNone)
	} else {
		w.SetCollectionBehavior(mac.WindowCollectionBehaviorFullScreenPrimary | mac.WindowCollectionBehaviorManaged)
	}
	if cfg.Floating {
		w.SetLevel(mac.WindowLevelFloating)
	}
	v := mac.NewView(w)
	if cfg.Transparent {
		w.SetTransparent()
	}
	w.SetContentView(v)
	w.MakeFirstResponder(v)
	if cfg.Title != "" {
		w.SetTitle(cfg.Title)
	}
	delegate := mac.NewWindowDelegate(w)
	w.SetDelegate(delegate)
	w.SetAcceptsMouseMovedEvents(true)
	w.SetRestorable(false)
	w.SetTabbingMode(mac.WindowTabbingModeDisallowed)
	wnd := &Window{
		plWnd: platformWindow{
			wnd:  w,
			view: v,
		},
	}
	wnd.createOpenGLContext(cfg.Share, cfg.Transparent)
	return wnd
}

func (w *Window) NativeWindow() mac.Window {
	return w.plWnd.wnd
}

func (w *Window) NativeView() mac.View {
	return w.plWnd.view
}

func (w *Window) SetTitle(title string) {
	w.plWnd.wnd.SetTitle(title)
}

func (w *Window) Frame() geom.Rect {
	frame := w.plWnd.wnd.Frame()
	frame.Y = transformCocoaY(frame.Bottom())
	return frame
}

func (w *Window) ContentRect() geom.Rect {
	r := w.plWnd.wnd.ContentRectForFrameRect(w.plWnd.wnd.Frame())
	r.Y = transformCocoaY(r.Bottom())
	return r
}

func (w *Window) SetContentRect(rect geom.Rect) {
	rect.Y = transformCocoaY(rect.Bottom())
	w.plWnd.wnd.SetFrame(w.plWnd.wnd.FrameRectForContentRect(rect))
}

func (w *Window) adjustToCursorChange() { // formerly plafAdjustToCursorChange
	if w.cursorInContentArea() {
		w.updateCursorImage()
	}
}

func (w *Window) updateCursor() { // formerly _plafUpdateCursor
	if w.Focused() {
		if w.cursorInContentArea() {
			w.updateCursorImage()
		}
	}
}

func (w *Window) updateCursorImage() { // formerly _plafUpdateCursorImage
	if w.cursorHidden {
		if !w.plWnd.nsCursorHidden {
			mac.HideCursor()
			w.plWnd.nsCursorHidden = true
		}
	} else {
		if w.plWnd.nsCursorHidden {
			mac.ShowCursor()
			w.plWnd.nsCursorHidden = false
		}
		if w.cursor != nil {
			w.cursor.plCursor.Set()
		} else {
			mac.ArrowCursor().Set()
		}
	}
}

func (w *Window) cursorInContentArea() bool { // formerly _plafCursorInContentArea
	return w.plWnd.view.MouseInRect(w.plWnd.wnd.MouseLocationOutsideOfEventStream(), w.plWnd.view.Frame())
}

func (w *Window) CursorPosition() geom.Point { // formerly plafGetCursorPos
	loc := w.plWnd.wnd.MouseLocationOutsideOfEventStream()
	frame := w.plWnd.view.Frame()
	return geom.NewPoint(loc.X, frame.Height-loc.Y)
}

func (w *Window) ContentScale() geom.Point {
	return w.plWnd.view.BackingScale()
}

func (w *Window) Minimize() {
	if !w.plWnd.wnd.Miniaturized() {
		w.plWnd.wnd.Miniaturize()
	}
}

func (w *Window) Maximize() {
	if !w.plWnd.wnd.Zoomed() {
		w.plWnd.wnd.Zoom()
	}
}

func (w *Window) Focused() bool { // formerly plafIsWindowFocused
	return w.plWnd.wnd.Focused()
}

func (w *Window) Focus() {
	mac.ActivateIgnoringOtherApps()
	w.plWnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) Visible() bool {
	return w.plWnd.wnd.Visible()
}

func (w *Window) Show() {
	w.plWnd.wnd.MakeKeyAndOrderFront()
}

func (w *Window) Hide() {
	w.plWnd.wnd.OrderOut()
}

func (w *Window) Resizable() bool {
	return w.plWnd.wnd.StyleMask()&mac.WindowStyleMaskResizable != 0
}

func (w *Window) destroy() { // formerly _plafDestroyWindow
	w.plWnd.wnd.OrderOut()
	w.plGctx.destroy()
	delegate := w.plWnd.wnd.Delegate()
	w.plWnd.wnd.SetDelegate(0)
	delegate.Release()
	w.plWnd.view.Release()
	w.plWnd.wnd.Close()
	w.plWnd.wnd = 0
	PollEvents()
}
