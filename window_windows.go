// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"runtime"
	"slices"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

const wndProcClassName = "Unison"

var (
	mainWndClass w32.ATOM
	mainInstance w32.HINSTANCE
)

type platformWindow struct {
	wnd           windows.HWND
	highSurrogate rune
	maximized     bool
	minimized     bool
	mouseTracked  bool
}

func findWindowByHWND(wnd windows.HWND) *Window {
	if i := slices.IndexFunc(windowList, func(w *Window) bool {
		return w.wnd.wnd == wnd
	}); i != -1 {
		return windowList[i]
	}
	return nil
}

func (w *Window) initNativeWindow(cfg *WindowConfig) error {
	style := w.windowStyle()
	exStyle := w.windowExStyle()
	if mainWndClass == 0 {
		mainInstance = w32.HINSTANCE(w32.GetModuleHandleW(""))
		className, err := windows.UTF16FromString(wndProcClassName)
		if err != nil {
			return errs.New("unable to create window class name")
		}
		defer runtime.KeepAlive(className)
		mainWndClass = w32.RegisterClassExW(&w32.WNDCLASSEX{
			Style:     w32.CS_HREDRAW | w32.CS_VREDRAW | w32.CS_OWNDC,
			WndProc:   windows.NewCallbackCDecl(wndProc),
			Instance:  mainInstance,
			Cursor:    ArrowCursor().cursor.cursor,
			ClassName: &className[0],
			Icon: w32.HICON(w32.LoadImageW(0, w32.MakeIntResourceW(w32.IDI_APPLICATION), w32.IMAGE_ICON, 0, 0,
				w32.LR_DEFAULT_SIZE|w32.LR_SHARED)),
		})
		if mainWndClass == 0 {
			return errs.New("unable to register window class")
		}
	}
	var frameX, frameY, frameWidth, frameHeight int32
	rect := w32.RECT{
		Left:   0,
		Top:    0,
		Right:  1,
		Bottom: 1,
	}
	w32.AdjustWindowRectEx(&rect, style, false, exStyle)
	frameX = rect.Left
	frameY = rect.Top
	frameWidth = rect.Right - rect.Left
	frameHeight = rect.Bottom - rect.Top
	w.wnd.wnd = w32.CreateWindowExW(exStyle, wndProcClassName, w.title, style, frameX, frameY, frameWidth, frameHeight,
		0, 0, mainInstance, 0)
	if w.wnd.wnd == 0 {
		return errs.New("unable to create window")
	}
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_DROPFILES, w32.MSGFLT_ALLOW, nil)
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYDATA, w32.MSGFLT_ALLOW, nil)
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYGLOBALDATA, w32.MSGFLT_ALLOW, nil)
	if isWindows10BuildOrGreater(w32.Windows10AnniversaryUpdateBuild) {
		rect = w32.RECT{
			Left:   0,
			Top:    0,
			Right:  1,
			Bottom: 1,
		}
		w32.AdjustWindowRectExForDpi(&rect, style, false, exStyle, w32.GetDpiForWindow(w.wnd.wnd))
	}
	var wp w32.WINDOWPLACEMENT
	w32.GetWindowPlacement(w.wnd.wnd, &wp)
	dx := wp.NormalPosition.Left - rect.Left
	dy := wp.NormalPosition.Top - rect.Top
	rect.Left += dx
	rect.Top += dy
	rect.Right += dx
	rect.Bottom += dy
	wp.NormalPosition = rect
	wp.ShowCmd = w32.SW_HIDE
	w32.SetWindowPlacement(w.wnd.wnd, &wp)
	w32.DragAcceptFiles(w.wnd.wnd, true)
	w32.GetClientRect(w.wnd.wnd, &rect)
	w.lastWidth = float32(rect.Right - rect.Left)
	w.lastHeight = float32(rect.Bottom - rect.Top)

	if err := w.glCtx.create(w, cfg.Share, cfg.Transparent); err != nil {
		return err
	}

	// TODO: Implement mouse passthrough
	// if (wndconfig->mousePassthrough) {
	// 	_plafSetWindowMousePassthrough(window, true);
	// }

	return nil
}

func wndProc(hWnd windows.HWND, uMsg uint32, wParam w32.WPARAM, lParam w32.LPARAM) uintptr {
	if w := findWindowByHWND(hWnd); w != nil {
		switch uMsg {
		case w32.WM_SETFOCUS:
			w.gainedFocus()
			return 0
		case w32.WM_KILLFOCUS:
			w.lostFocus()
			return 0
		case w32.WM_CLOSE:
			w.nativeRequestClose()
			return 0
		case w32.WM_CHAR, w32.WM_SYSCHAR:
			switch {
			case wParam >= 0xD800 && wParam <= 0xDBFF:
				w.wnd.highSurrogate = rune(wParam)
			case uMsg == w32.WM_SYSCHAR:
				w.wnd.highSurrogate = 0
			default:
				var r rune
				if wParam >= 0xDC00 && wParam <= 0xDFFF {
					if w.wnd.highSurrogate != 0 {
						r = (w.wnd.highSurrogate-0xD800)<<10 + (rune(wParam) - 0xDC00) + 0x10000
					}
				} else {
					r = rune(wParam)
				}
				w.wnd.highSurrogate = 0
				w.runeTyped(r)
			}
			return 0
		case w32.WM_UNICHAR:
			if wParam == w32.UNICODE_NOCHAR {
				return 1
			}
			w.runeTyped(rune(wParam))
			return 0
		case w32.WM_KEYDOWN,
			w32.WM_SYSKEYDOWN,
			w32.WM_KEYUP,
			w32.WM_SYSKEYUP:
			// TODO: Standard ways of inputting accented characters (ALT-0201 for accented capital E, for example) aren't working right now
			scanCode := int(((lParam >> 16) & 0xFFFF) & (w32.KF_EXTENDED | 0xFF))
			switch scanCode {
			case 0:
				scanCode = int(w32.MapVirtualKeyW(uint32(wParam), w32.MAPVK_VK_TO_VSC))
			case 0x54: // Alt+PrtScn
				scanCode = 0x137
			case 0x146: // Ctrl+Pause
				scanCode = 0x45
			case 0x136: // CJK IME right shift
				scanCode = 0x36
			}
			key := rawScanCodeToKeyCodeMap[scanCode]
			if wParam == w32.VK_CONTROL {
				if ((lParam>>16)&0xFFFF)&w32.KF_EXTENDED != 0 {
					key = KeyRControl
				} else {
					var next w32.MSG
					when := w32.GetMessageTime()
					if w32.PeekMessageW(&next, 0, 0, 0, w32.PM_NOREMOVE) {
						if (next.Message == w32.WM_KEYDOWN || next.Message == w32.WM_SYSKEYDOWN || next.Message == w32.WM_KEYUP || next.Message == w32.WM_SYSKEYUP) && next.WParam == w32.VK_MENU && next.Time == when && ((next.LParam>>16)&0xFFFF)&w32.KF_EXTENDED != 0 {
							break
						}
					}
					key = KeyLControl
				}
			} else if wParam == w32.VK_PROCESSKEY {
				break
			}
			mods := w.CurrentKeyModifiers()
			pressed := ((lParam>>16)&0xFFFF)&w32.KF_UP != 0
			switch {
			case !pressed && wParam == w32.VK_SHIFT:
				w.keyReleased(KeyLShift, mods)
				w.keyReleased(KeyRShift, mods)
			case wParam == w32.VK_SNAPSHOT:
				w.keyPressed(KeyPrintScreen, mods)
				w.keyReleased(KeyPrintScreen, mods)
			case pressed:
				w.keyPressed(key, mods)
			default:
				w.keyReleased(key, mods)
			}
		case w32.WM_LBUTTONDOWN,
			w32.WM_RBUTTONDOWN,
			w32.WM_MBUTTONDOWN,
			w32.WM_XBUTTONDOWN,
			w32.WM_LBUTTONUP,
			w32.WM_RBUTTONUP,
			w32.WM_MBUTTONUP,
			w32.WM_XBUTTONUP:
			var button int
			switch uMsg {
			case w32.WM_LBUTTONDOWN, w32.WM_LBUTTONUP:
				button = ButtonLeft
			case w32.WM_RBUTTONDOWN, w32.WM_RBUTTONUP:
				button = ButtonRight
			case w32.WM_MBUTTONDOWN, w32.WM_MBUTTONUP:
				button = ButtonMiddle
			default:
				if (wParam>>16)&0xFFFF == w32.XBUTTON1 {
					button = ButtonMiddle + 1
				} else {
					button = ButtonMiddle + 2
				}
			}
			pressed := uMsg == w32.WM_LBUTTONDOWN || uMsg == w32.WM_RBUTTONDOWN || uMsg == w32.WM_MBUTTONDOWN || uMsg == w32.WM_XBUTTONDOWN
			w.nativeMouseClick(button, pressed, w.CurrentKeyModifiers())
			if uMsg == w32.WM_XBUTTONDOWN || uMsg == w32.WM_XBUTTONUP {
				return 1
			}
			return 0
		case w32.WM_MOUSEMOVE:
			pt := geom.NewPoint(float32(lParam&0xFFFF), float32((lParam>>16)&0xFFFF))
			if !w.wnd.mouseTracked {
				var evt w32.TRACKMOUSEEVENT
				evt.Flags = w32.TME_LEAVE
				evt.HwndTrack = w.wnd.wnd
				w32.TrackMouseEvent(&evt)
				w.wnd.mouseTracked = true
				w.updateCursorImage()
				w.mouseEnter(pt, w.lastKeyModifiers)
			}
			w.nativeMouseMoved(pt)
			return 0
		case w32.WM_MOUSELEAVE:
			w.wnd.mouseTracked = false
			w.mouseExit()
			return 0
		case w32.WM_MOUSEWHEEL:
			w.nativeMouseWheel(geom.NewPoint(0, float32(int16((wParam>>16)&0xFFFF))/float32(w32.WHEEL_DELTA)), true)
			return 0
		case w32.WM_MOUSEHWHEEL:
			w.nativeMouseWheel(geom.NewPoint(float32(int16((wParam>>16)&0xFFFF))/float32(w32.WHEEL_DELTA), 0), true)
			return 0
		case w32.WM_SIZE:
			minimized := wParam == w32.SIZE_MINIMIZED
			if w.wnd.minimized != minimized {
				w.wnd.minimized = minimized
				if w.MinimizedCallback != nil {
					w.MinimizedCallback(minimized)
				}
			}
			maximized := wParam == w32.SIZE_MAXIMIZED || (w.wnd.maximized && wParam != w32.SIZE_RESTORED)
			if w.wnd.maximized != maximized {
				w.wnd.maximized = maximized
				if w.MaximizeCallback != nil {
					w.MaximizeCallback(maximized)
				}
			}
			width := float32(lParam & 0xFFFF)
			height := float32((lParam >> 16) & 0xFFFF)
			if width != w.lastWidth || height != w.lastHeight {
				w.resized()
				w.lastWidth = width
				w.lastHeight = height
			}
			return 0
		case w32.WM_MOVE:
			if w.MovedCallback != nil {
				w.MovedCallback()
			}
			return 0
		case w32.WM_SIZING:
			return 1
		case w32.WM_GETMINMAXINFO:
			var frame w32.RECT
			style := w.windowStyle()
			exStyle := w.windowExStyle()
			if isWindows10BuildOrGreater(w32.Windows10AnniversaryUpdateBuild) {
				w32.AdjustWindowRectExForDpi(&frame, style, false, exStyle, w32.GetDpiForWindow(w.wnd.wnd))
			} else {
				w32.AdjustWindowRectEx(&frame, style, false, exStyle)
			}
			minimum, maximum := w.minMaxContentSize()
			mmi := (*w32.MINMAXINFO)(unsafe.Pointer(lParam)) //nolint:govet // No other choice
			mmi.MinTrackSize.X = int32(minimum.Width) + frame.Right - frame.Left
			mmi.MinTrackSize.Y = int32(minimum.Height) + frame.Bottom - frame.Top
			mmi.MaxTrackSize.X = int32(maximum.Width) + frame.Right - frame.Left
			mmi.MaxTrackSize.Y = int32(maximum.Height) + frame.Bottom - frame.Top
			return 0
		case w32.WM_PAINT:
			w.draw()
		case w32.WM_ERASEBKGND:
			return 1
		case w32.WM_NCACTIVATE,
			w32.WM_NCPAINT:
			if w.undecorated {
				return 1
			}
		case w32.WM_DWMCOMPOSITIONCHANGED,
			w32.WM_DWMCOLORIZATIONCOLORCHANGED:
			// TODO: IMPLEMENT!
		case w32.WM_GETDPISCALEDSIZE:
			// TODO: IMPLEMENT!
		case w32.WM_DPICHANGED:
			// TODO: IMPLEMENT!
		case w32.WM_SETCURSOR:
			if lParam&0xFFFF == w32.HTCLIENT {
				w.updateCursorImage()
				return 1
			}
		case w32.WM_DROPFILES:
			// TODO: IMPLEMENT!
		}
	}
	return w32.DefWindowProcW(hWnd, uMsg, wParam, lParam)
}

func (w *Window) windowStyle() uint32 {
	var style uint32
	style = w32.WS_CLIPSIBLINGS | w32.WS_CLIPCHILDREN | w32.WS_SYSMENU | w32.WS_MINIMIZEBOX
	if w.undecorated {
		style |= w32.WS_POPUP
	} else {
		style |= w32.WS_CAPTION
		if !w.notResizable {
			style |= w32.WS_MAXIMIZEBOX | w32.WS_THICKFRAME
		}
	}
	return style
}

func (w *Window) windowExStyle() uint32 {
	var style uint32
	style = w32.WS_EX_APPWINDOW
	if w.floating {
		style |= w32.WS_EX_TOPMOST
	}
	return style
}

func (w *Window) setTitle(title string) {
	w32.SetWindowTextW(w.wnd.wnd, title)
}

func (w *Window) frameRect() geom.Rect {
	if w.IsValid() {
		var rect w32.RECT
		w32.GetClientRect(w.wnd.wnd, &rect)
		rect.Right -= rect.Left
		rect.Bottom -= rect.Top
		rect.Left = 0
		rect.Top = 0
		width := rect.Right
		height := rect.Bottom
		if isWindows10BuildOrGreater(w32.Windows10AnniversaryUpdateBuild) {
			if !w32.AdjustWindowRectExForDpi(&rect, w.windowStyle(), false, w.windowExStyle(), w32.GetDpiForWindow(w.wnd.wnd)) {
				return geom.NewRect(1, 1, 2, 2)
			}
		} else {
			if !w32.AdjustWindowRectEx(&rect, w.windowStyle(), false, w.windowExStyle()) {
				return geom.NewRect(1, 1, 2, 2)
			}
		}
		return geom.NewRect(float32(-rect.Left), float32(-rect.Top), float32(rect.Right-width), float32(rect.Bottom-height))
	}
	return geom.NewRect(1, 1, 2, 2)
}

// ContentRect returns the boundaries in display coordinates of the window's content area.
func (w *Window) ContentRect() geom.Rect {
	if w.IsValid() {
		var pt w32.POINT
		w32.ClientToScreen(w.wnd.wnd, &pt)
		var rect w32.RECT
		w32.GetClientRect(w.wnd.wnd, &rect)
		r := geom.NewRect(float32(pt.X), float32(pt.Y), float32(rect.Right-rect.Left), float32(rect.Bottom-rect.Top))
		scale := w.backingScale()
		r.Point = r.Point.DivPt(scale)
		r.Size = r.Size.DivPt(scale)
		return r
	}
	return geom.NewRect(0, 0, 1, 1)
}

// SetContentRect sets the boundaries of the frame of this window by converting the content rect into a suitable frame
// rect and then applying it to the window.
func (w *Window) SetContentRect(rect geom.Rect) {
	if w.IsValid() {
		rect = w.adjustContentRectForMinMax(rect)
		scale := w.backingScale()
		rect.Point = rect.Point.MulPt(scale)
		rect.Size = rect.Size.MulPt(scale)
		w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOP, int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height),
			w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_NOOWNERZORDER)
	}
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	// TODO: Is this right?
	return w.LastKeyModifiers()
}

func (w *Window) adjustToCursorChange() {
	if w.cursorInContentArea() {
		w.updateCursorImage()
	}
}

func (w *Window) updateCursorImage() {
	switch {
	case w.cursorHidden:
		// TODO: This might need to be an actual blank cursor
		w32.SetCursor(0)
	case w.cursor != nil:
		w32.SetCursor(w.cursor.cursor.cursor)
	default:
		w32.SetCursor(ArrowCursor().cursor.cursor)
	}
}

func (w *Window) cursorInContentArea() bool {
	var pos w32.POINT
	if !w32.GetCursorPos(&pos) {
		return false
	}
	if w32.WindowFromPoint(pos) != w.wnd.wnd {
		return false
	}
	var area w32.RECT
	w32.GetClientRect(w.wnd.wnd, &area)
	var topLeft w32.POINT
	topLeft.X = area.Left
	topLeft.Y = area.Top
	w32.ClientToScreen(w.wnd.wnd, &topLeft)
	var bottomRight w32.POINT
	bottomRight.X = area.Right
	bottomRight.Y = area.Bottom
	w32.ClientToScreen(w.wnd.wnd, &bottomRight)
	return pos.X >= topLeft.X && pos.X <= bottomRight.X && pos.Y >= topLeft.Y && pos.Y <= bottomRight.Y
}

func (w *Window) cursorPosition() geom.Point {
	var pos w32.POINT
	if w32.GetCursorPos(&pos) {
		w32.ScreenToClient(w.wnd.wnd, &pos)
		return geom.NewPoint(float32(pos.X), float32(pos.Y))
	}
	return geom.NewPoint(0, 0)
}

func (w *Window) backingScale() geom.Point {
	dpi := w32.GetDpiForWindow(w.wnd.wnd)
	return geom.NewPoint(float32(dpi)/96.0, float32(dpi)/96.0)
}

func (w *Window) minimize() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_MINIMIZE)
}

func (w *Window) maximize() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_MAXIMIZE)
}

func (w *Window) acquireFocus() {
	w32.BringWindowToTop(w.wnd.wnd)
	w32.SetForegroundWindow(w.wnd.wnd)
	w32.SetFocus(w.wnd.wnd)
}

func (w *Window) visible() bool {
	return w32.IsWindowVisible(w.wnd.wnd)
}

func (w *Window) show() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_SHOWNA)
}

func (w *Window) hide() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_HIDE)
}

func (w *Window) nativeDestroy() {
	w.glCtx.destroy()
	if w.wnd.wnd != 0 {
		w32.DestroyWindow(w.wnd.wnd)
		w.wnd.wnd = 0
	}
}

func (w *Window) convertRawMouseLocationForPlatform(where geom.Point) geom.Point {
	if w.IsValid() {
		scale := w.backingScale()
		where.X /= scale.X
		where.Y /= scale.Y
	}
	return where
}
