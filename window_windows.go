// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"image"
	"math"
	"runtime"
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
	blankCursor  w32.HCURSOR
)

type apiWindow struct {
	wnd           windows.HWND
	bigIcon       w32.HICON
	smallIcon     w32.HICON
	highSurrogate uint16
	maximized     bool
	minimized     bool
	mouseTracked  bool
}

func findWindowByHWND(wnd windows.HWND) *Window {
	for _, w := range windowList {
		if w.wnd.wnd == wnd {
			return w
		}
	}
	return nil
}

func (w *Window) apiInit() error {
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
			ClassName: &className[0],
			Icon: w32.HICON(w32.LoadImageW(0, w32.MakeIntResourceW(w32.IDI_APPLICATION), w32.IMAGE_ICON, 0, 0,
				w32.LR_DEFAULT_SIZE|w32.LR_SHARED)),
		})
		if mainWndClass == 0 {
			return errs.New("unable to register window class")
		}
	}
	w.wnd.wnd = w32.CreateWindowExW(exStyle, wndProcClassName, w.title, style, 0, 0, 1, 1, 0, 0, mainInstance, 0)
	if w.wnd.wnd == 0 {
		return errs.New("unable to create window")
	}
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_DROPFILES, w32.MSGFLT_ALLOW, nil)
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYDATA, w32.MSGFLT_ALLOW, nil)
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYGLOBALDATA, w32.MSGFLT_ALLOW, nil)
	w32.DragAcceptFiles(w.wnd.wnd, true)
	w.updateFramebufferTransparency()
	var rect w32.RECT
	w32.GetClientRect(w.wnd.wnd, &rect)
	w.lastWidth = float32(rect.Right - rect.Left)
	w.lastHeight = float32(rect.Bottom - rect.Top)
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
				w.wnd.highSurrogate = uint16(wParam)
			case uMsg == w32.WM_SYSCHAR:
				w.wnd.highSurrogate = 0
			default:
				var r rune
				if wParam >= 0xDC00 && wParam <= 0xDFFF {
					if w.wnd.highSurrogate != 0 {
						r = (rune(w.wnd.highSurrogate) - 0xD800) << 10
						r += (rune(wParam) & 0xFFFF) - 0xDC00
						r += 0x10000
					}
				} else {
					r = rune(wParam) & 0xFFFF
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
			scanCode := int(((lParam >> 16) & (w32.KF_EXTENDED | 0xFF)))
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
				if (lParam>>16)&w32.KF_EXTENDED != 0 {
					key = KeyRControl
				} else {
					var next w32.MSG
					when := w32.GetMessageTime()
					if w32.PeekMessageW(&next, 0, 0, 0, w32.PM_NOREMOVE) {
						if (next.Message == w32.WM_KEYDOWN || next.Message == w32.WM_SYSKEYDOWN || next.Message == w32.WM_KEYUP || next.Message == w32.WM_SYSKEYUP) && next.WParam == w32.VK_MENU && next.Time == when && ((next.LParam>>16)&w32.KF_EXTENDED) != 0 {
							break
						}
					}
					key = KeyLControl
				}
			} else if wParam == w32.VK_PROCESSKEY {
				break
			}
			mods := w.CurrentKeyModifiers()
			pressed := (lParam>>16)&w32.KF_UP == 0
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
			pressed := uMsg == w32.WM_LBUTTONDOWN || uMsg == w32.WM_RBUTTONDOWN || uMsg == w32.WM_MBUTTONDOWN ||
				uMsg == w32.WM_XBUTTONDOWN
			w.nativeMouseClick(button, geom.NewPoint(float32(lParam&0xFFFF), float32((lParam>>16)&0xFFFF)), pressed,
				w.CurrentKeyModifiers())
			if uMsg == w32.WM_XBUTTONDOWN || uMsg == w32.WM_XBUTTONUP {
				return 1
			}
			return 0
		case w32.WM_MOUSEMOVE:
			w.handleWindowsMouseMove(geom.NewPoint(float32(lParam&0xFFFF), float32((lParam>>16)&0xFFFF)))
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
				w.lastWidth = width
				w.lastHeight = height
				w.resized()
			}
			return 0
		case w32.WM_MOVE:
			w.moved()
			return 0
		case w32.WM_SIZING:
			return 1
		case w32.WM_GETMINMAXINFO:
			var frame w32.RECT
			style := w.windowStyle()
			exStyle := w.windowExStyle()
			if w32IsWindows10BuildOrGreater(w32.Windows10AnniversaryUpdateBuild) {
				w32.AdjustWindowRectExForDpi(&frame, style, false, exStyle, w32.GetDpiForWindow(w.wnd.wnd))
			} else {
				w32.AdjustWindowRectEx(&frame, style, false, exStyle)
			}
			minimum, maximum := w.minMaxContentSize()
			scale := w.apiBackingScale()
			minimum = minimum.MulPt(scale).Ceil()
			maximum = maximum.MulPt(scale).Ceil()
			frame.Left = int32(float32(frame.Left))
			frame.Right = int32(float32(frame.Right))
			frame.Top = int32(float32(frame.Top))
			frame.Bottom = int32(float32(frame.Bottom))
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
			w.updateFramebufferTransparency()
			return 0
		case w32.WM_GETDPISCALEDSIZE:
			if w32IsWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
				var src, dst w32.RECT
				style := w.windowStyle()
				exStyle := w.windowExStyle()
				curDPI := w32.GetDpiForWindow(w.wnd.wnd)
				newDPI := uint32(wParam & 0xFFFF)
				w32.AdjustWindowRectExForDpi(&src, style, false, exStyle, curDPI)
				w32.AdjustWindowRectExForDpi(&dst, style, false, exStyle, newDPI)
				size := (*w32.SIZE)(unsafe.Pointer(lParam)) //nolint:govet // Required to access data
				if curDPI != newDPI {
					scale := float32(newDPI) / float32(curDPI)
					size.CX = int32(float32(size.CX) * scale)
					size.CY = int32(float32(size.CY) * scale)
				}
				size.CX += (dst.Right - dst.Left) - (src.Right - src.Left)
				size.CY += (dst.Bottom - dst.Top) - (src.Bottom - src.Top)
				return 1
			}
		case w32.WM_DPICHANGED:
			if w32IsWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
				rect := (*w32.RECT)(unsafe.Pointer(lParam)) //nolint:govet // Required to access data
				w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOP, rect.Left, rect.Top, rect.Right-rect.Left,
					rect.Bottom-rect.Top, w32.SWP_NOZORDER|w32.SWP_NOACTIVATE)
			}
			if w.ContentScaleCallback != nil {
				scale := float32((wParam>>16)&0xFFFF) / 96
				w.ContentScaleCallback(geom.NewPoint(scale, scale))
			}
		case w32.WM_SETCURSOR:
			if lParam&0xFFFF == w32.HTCLIENT {
				w.apiUpdateCursorImage()
				return 1
			}
		case w32.WM_DROPFILES:
			drop := w32.HDROP(wParam)
			count := w32.DragQueryFileCount(drop)
			paths := make([]string, count)
			pt, _ := w32.DragQueryPoint(drop)
			w.handleWindowsMouseMove(geom.NewPoint(float32(pt.X), float32(pt.Y)))
			for i := range count {
				paths[i] = w32.DragQueryFileW(drop, i)
			}
			w.fileDrop(paths)
			w32.DragFinish(drop)
			return 0
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

func (w *Window) handleWindowsMouseMove(pt geom.Point) {
	if !w.wnd.mouseTracked {
		var evt w32.TRACKMOUSEEVENT
		evt.Flags = w32.TME_LEAVE
		evt.HwndTrack = w.wnd.wnd
		w32.TrackMouseEvent(&evt)
		w.wnd.mouseTracked = true
		w.apiUpdateCursorImage()
		w.mouseEnter(pt, w.lastKeyModifiers)
	}
	w.nativeMouseMoved(pt)
}

func (w *Window) apiSetTitle(title string) {
	w32.SetWindowTextW(w.wnd.wnd, title)
}

func (w *Window) apiSetTitleIcons(images []*image.NRGBA) {
	var big, small w32.HICON
	if len(images) > 0 {
		cxIcon := w32.GetSystemMetrics(w32.SM_CXICON)
		cyIcon := w32.GetSystemMetrics(w32.SM_CYICON)
		cxSmIcon := w32.GetSystemMetrics(w32.SM_CXSMICON)
		cySmIcon := w32.GetSystemMetrics(w32.SM_CYSMICON)
		big = w32CreateIconFromImage(chooseBestImage(images, cxIcon, cyIcon), 0, 0, true)
		small = w32CreateIconFromImage(chooseBestImage(images, cxSmIcon, cySmIcon), 0, 0, true)
	} else {
		big = w32.HICON(w32.GetClassLongPtrW(w.wnd.wnd, w32.GCLP_HICON))
		small = w32.HICON(w32.GetClassLongPtrW(w.wnd.wnd, w32.GCLP_HICONSM))
	}
	w32.SendMessageW(w.wnd.wnd, w32.WM_SETICON, w32.ICON_BIG, w32.LPARAM(big))
	w32.SendMessageW(w.wnd.wnd, w32.WM_SETICON, w32.ICON_SMALL, w32.LPARAM(small))
	if w.wnd.bigIcon != 0 {
		w32.DestroyIcon(w.wnd.bigIcon)
	}
	if w.wnd.smallIcon != 0 {
		w32.DestroyIcon(w.wnd.smallIcon)
	}
	w.wnd.bigIcon = big
	w.wnd.smallIcon = small
}

func chooseBestImage(images []*image.NRGBA, width, height int) *image.NRGBA {
	var closest *image.NRGBA
	leastDiff := math.MaxInt32
	wh := width * height
	for _, image := range images {
		currDiff := image.Rect.Dx()*image.Rect.Dy() - wh
		if currDiff < 0 {
			currDiff = -currDiff
		}
		if currDiff < leastDiff {
			closest = image
			leastDiff = currDiff
		}
	}
	return closest
}

func (w *Window) apiDisplay() *Display {
	var rect w32.RECT
	w32.GetWindowRect(w.wnd.wnd, &rect)
	pt := geom.NewPoint(float32(rect.Left), float32(rect.Top))
	for _, d := range AllDisplays() {
		if pt.In(d.Usable) {
			return d
		}
	}
	return monitorInfo(w32.MonitorFromWindow(w.wnd.wnd, w32.MONITOR_DEFAULTTOPRIMARY))
}

func (w *Window) apiFrameRect() geom.Rect {
	var rect w32.RECT
	w32.GetWindowRect(w.wnd.wnd, &rect)
	r := rectFromW32Rect(rect)
	r.Size = r.Size.DivPt(w.apiBackingScale())
	return r.Align()
}

func (w *Window) apiFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	return contentRect.Inset(w.frameInsets().Mul(-1)).Align()
}

func (w *Window) frameInsets() geom.Insets {
	var rect w32.RECT
	style := w.windowStyle()
	exStyle := w.windowExStyle()
	if w32IsWindows10BuildOrGreater(w32.Windows10AnniversaryUpdateBuild) {
		w32.AdjustWindowRectExForDpi(&rect, style, false, exStyle, w32.GetDpiForWindow(w.wnd.wnd))
	} else {
		w32.AdjustWindowRectEx(&rect, style, false, exStyle)
	}
	r := rectFromW32Rect(rect)
	scale := w.apiBackingScale()
	r.Point = r.Point.DivPt(scale)
	r.Size = r.Size.DivPt(scale)
	r = r.Align()
	return geom.NewInsets(-r.Y, -r.X, r.Bottom(), r.Right())
}

func (w *Window) apiEnsureOnDisplay() {
	var r w32.RECT
	w32.GetWindowRect(w.wnd.wnd, &r)
	frameRect := rectFromW32Rect(r)
	d := w.apiDisplay()
	revisedRect := d.FitRectOnto(frameRect)
	if frameRect != revisedRect {
		revisedRect.Size = revisedRect.Size.DivPt(d.Scale)
		w.SetFrameRect(revisedRect.Align())
	}
}

func (w *Window) apiContentRect() geom.Rect {
	var rect w32.RECT
	w32.GetClientRect(w.wnd.wnd, &rect)
	var pt w32.POINT
	w32.ClientToScreen(w.wnd.wnd, &pt)
	r := geom.NewRect(float32(pt.X), float32(pt.Y), float32(rect.Right), float32(rect.Bottom))
	r.Size = r.Size.DivPt(w.apiBackingScale())
	return r.Align()
}

func (w *Window) apiContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	return frameRect.Inset(w.frameInsets()).Align()
}

func (w *Window) apiSetContentRect(rect geom.Rect) {
	rect = rect.Inset(w.frameInsets().Mul(-1))
	rect.Size = rect.Size.MulPt(w.apiBackingScale())
	rect = rect.Align()
	w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOP, int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height),
		w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_NOOWNERZORDER)
}

func (w *Window) apiCurrentKeyModifiers() Modifiers {
	var mods Modifiers
	if w32.GetKeyState(w32.VK_SHIFT)&0x8000 != 0 {
		mods |= ShiftModifier
	}
	if w32.GetKeyState(w32.VK_CONTROL)&0x8000 != 0 {
		mods |= ControlModifier
	}
	if w32.GetKeyState(w32.VK_MENU)&0x8000 != 0 {
		mods |= OptionModifier
	}
	if w32.GetKeyState(w32.VK_LWIN)&0x8000 != 0 || w32.GetKeyState(w32.VK_RWIN)&0x8000 != 0 {
		mods |= CommandModifier
	}
	if w32.GetKeyState(w32.VK_CAPITAL)&0x0001 != 0 {
		mods |= CapsLockModifier
	}
	if w32.GetKeyState(w32.VK_NUMLOCK)&0x0001 != 0 {
		mods |= NumLockModifier
	}
	return mods
}

func (w *Window) apiUpdateCursorImage() {
	switch {
	case w.cursorHidden:
		if blankCursor == 0 {
			var data [1]byte
			blankCursor = w32.CreateCursor(w32.HINSTANCE(w32.GetModuleHandleW("")), 0, 0, 1, 1, data[:], data[:])
		}
		w32.SetCursor(blankCursor)
	case w.cursor != nil:
		w32.SetCursor(w.cursor.cursor.cursor)
	default:
		w32.SetCursor(ArrowCursor().cursor.cursor)
	}
}

func (w *Window) apiCursorInContentArea() bool {
	var pos w32.POINT
	if !w32.GetCursorPos(&pos) {
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

func (w *Window) apiCursorPosition() geom.Point {
	var pos w32.POINT
	if w32.GetCursorPos(&pos) {
		w32.ScreenToClient(w.wnd.wnd, &pos)
		return geom.NewPoint(float32(pos.X), float32(pos.Y))
	}
	return geom.NewPoint(0, 0)
}

func (w *Window) apiBackingScale() geom.Point {
	dpi := w32.GetDpiForWindow(w.wnd.wnd)
	return geom.NewPoint(float32(dpi)/96.0, float32(dpi)/96.0)
}

func (w *Window) apiMinimize() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_MINIMIZE)
}

func (w *Window) apiMaximize() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_MAXIMIZE)
}

func (w *Window) apiAcquireFocus() {
	w32.BringWindowToTop(w.wnd.wnd)
	w32.SetForegroundWindow(w.wnd.wnd)
	w32.SetFocus(w.wnd.wnd)
}

func (w *Window) apiVisible() bool {
	return windows.IsWindowVisible(w.wnd.wnd)
}

func (w *Window) apiShow() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_SHOWNA)
}

func (w *Window) apiHide() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_HIDE)
}

func (w *Window) apiDestroy() {
	w.glCtx.apiDestroy()
	if w.wnd.wnd != 0 {
		w32.DestroyWindow(w.wnd.wnd)
		w.wnd.wnd = 0
	}
	if w.wnd.bigIcon != 0 {
		w32.DestroyIcon(w.wnd.bigIcon)
		w.wnd.bigIcon = 0
	}
	if w.wnd.smallIcon != 0 {
		w32.DestroyIcon(w.wnd.smallIcon)
		w.wnd.smallIcon = 0
	}
}

func (w *Window) apiConvertRawMouse(where geom.Point) geom.Point {
	if w.IsValid() {
		scale := w.apiBackingScale()
		where.X /= scale.X
		where.Y /= scale.Y
	}
	return where
}

func (w *Window) updateFramebufferTransparency() {
	if w.transparent {
		region := w32.CreateRectRgn(0, 0, -1, -1)
		bb := w32.DWM_BLURBEHIND{
			Flags:   w32.DWM_BB_ENABLE | w32.DWM_BB_BLURREGION,
			Enable:  1,
			RgnBlur: region,
		}
		w32.DwmEnableBlurBehindWindow(w.wnd.wnd, &bb)
		w32.DeleteObject(w32.HGDIOBJ(region))
	}
}
