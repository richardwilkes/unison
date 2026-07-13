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
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

const wndProcClassName = "Unison"

var (
	w32MainWndClass w32.ATOM
	w32MainInstance w32.HINSTANCE
	w32BlankCursor  w32.HCURSOR
)

type apiWindow struct {
	dropTarget    *w32.DropTarget
	wnd           windows.HWND
	bigIcon       w32.HICON
	smallIcon     w32.HICON
	highSurrogate uint16
	mouseTracked  bool
}

func w32FindWindowByHWND(wnd windows.HWND) *Window {
	for _, w := range windowList {
		if w.wnd.wnd == wnd {
			return w
		}
	}
	return nil
}

// w32OwnerHWND returns the native handle of the active window, or failing that, the frontmost window, for use as the
// owner of native modal dialogs. Providing an owner causes the system to position the dialog relative to it, ensuring
// it appears on the same display as the window that spawned it; without one, the system chooses the location itself,
// typically placing the dialog on the primary display.
func w32OwnerHWND() windows.HWND {
	w := ActiveWindow()
	if w == nil {
		w = FrontmostWindow()
	}
	if w != nil && w.IsValid() {
		return w.wnd.wnd
	}
	return 0
}

func (w *Window) apiInit() error {
	style := w.w32WindowStyle()
	exStyle := w.w32WindowExStyle()
	if w32MainWndClass == 0 {
		w32MainInstance = w32.HINSTANCE(w32.GetModuleHandleW(""))
		className, err := windows.UTF16FromString(wndProcClassName)
		if err != nil {
			return errs.New("unable to create window class name")
		}
		defer runtime.KeepAlive(className)
		w32MainWndClass = w32.RegisterClassExW(&w32.WNDCLASSEX{
			Style:     w32.CS_HREDRAW | w32.CS_VREDRAW | w32.CS_OWNDC,
			WndProc:   windows.NewCallbackCDecl(w32WndProc),
			Instance:  w32MainInstance,
			ClassName: &className[0],
			Icon: w32.HICON(w32.LoadImageW(0, w32.MakeIntResourceW(w32.IDI_APPLICATION), w32.IMAGE_ICON, 0, 0,
				w32.LR_DEFAULT_SIZE|w32.LR_SHARED)),
		})
		if w32MainWndClass == 0 {
			return errs.New("unable to register window class")
		}
	}
	w.wnd.wnd = w32.CreateWindowExW(exStyle, wndProcClassName, w.title, style, 0, 0, 1, 1, 0, 0, w32MainInstance, 0)
	if w.wnd.wnd == 0 {
		return errs.New("unable to create window")
	}
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYDATA, w32.MSGFLT_ALLOW, nil)
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYGLOBALDATA, w32.MSGFLT_ALLOW, nil)
	w.w32UpdateFramebufferTransparency()
	var rect w32.RECT
	w32.GetClientRect(w.wnd.wnd, &rect)
	w.lastWidth = float32(rect.Right - rect.Left)
	w.lastHeight = float32(rect.Bottom - rect.Top)
	return nil
}

func w32WndProc(hWnd windows.HWND, uMsg uint32, wParam w32.WPARAM, lParam w32.LPARAM) uintptr {
	if w := w32FindWindowByHWND(hWnd); w != nil {
		switch uMsg {
		case w32.WM_SETFOCUS:
			w.gainedFocus()
			return 0
		case w32.WM_KILLFOCUS:
			w.lostFocus()
			return 0
		case w32.WM_CLOSE:
			w.requestClose()
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
			scanCode := uint16(((lParam >> 16) & (w32.KF_EXTENDED | 0xFF)))
			switch scanCode {
			case 0:
				scanCode = uint16(w32.MapVirtualKeyW(uint32(wParam), w32.MAPVK_VK_TO_VSC))
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
			w32.WM_XBUTTONDOWN:
			var button int
			switch uMsg {
			case w32.WM_LBUTTONDOWN:
				button = ButtonLeft
			case w32.WM_RBUTTONDOWN:
				button = ButtonRight
			case w32.WM_MBUTTONDOWN:
				button = ButtonMiddle
			default:
				if (wParam>>16)&0xFFFF == w32.XBUTTON1 {
					button = ButtonMiddle + 1
				} else {
					button = ButtonMiddle + 2
				}
			}
			w.mouseDown(w.w32ConvertRawMouse(geom.NewPoint(float32(lParam&0xFFFF), float32((lParam>>16)&0xFFFF))),
				button, w.CurrentKeyModifiers())
			if uMsg == w32.WM_XBUTTONDOWN {
				return 1
			}
			return 0
		case w32.WM_LBUTTONUP,
			w32.WM_RBUTTONUP,
			w32.WM_MBUTTONUP,
			w32.WM_XBUTTONUP:
			var button int
			switch uMsg {
			case w32.WM_LBUTTONUP:
				button = ButtonLeft
			case w32.WM_RBUTTONUP:
				button = ButtonRight
			case w32.WM_MBUTTONUP:
				button = ButtonMiddle
			default:
				if (wParam>>16)&0xFFFF == w32.XBUTTON1 {
					button = ButtonMiddle + 1
				} else {
					button = ButtonMiddle + 2
				}
			}
			w.mouseUp(w.w32ConvertRawMouse(geom.NewPoint(float32(lParam&0xFFFF), float32((lParam>>16)&0xFFFF))),
				button, w.CurrentKeyModifiers())
			if uMsg == w32.WM_XBUTTONUP {
				return 1
			}
			return 0
		case w32.WM_MOUSEMOVE:
			w.w32HandleMouseMove(geom.NewPoint(float32(lParam&0xFFFF), float32((lParam>>16)&0xFFFF)))
			return 0
		case w32.WM_MOUSELEAVE:
			w.wnd.mouseTracked = false
			w.mouseExit()
			return 0
		case w32.WM_MOUSEWHEEL:
			var pos w32.POINT
			pos.X = int32(int16(lParam & 0xFFFF))
			pos.Y = int32(int16((lParam >> 16) & 0xFFFF))
			w32.ScreenToClient(w.wnd.wnd, &pos)
			w.mouseWheel(w.w32ConvertRawMouse(geom.NewPoint(float32(pos.X), float32(pos.Y))),
				geom.NewPoint(0, float32(int16((wParam>>16)&0xFFFF))/float32(w32.WHEEL_DELTA)), w.CurrentKeyModifiers())
			return 0
		case w32.WM_MOUSEHWHEEL:
			var pos w32.POINT
			pos.X = int32(int16(lParam & 0xFFFF))
			pos.Y = int32(int16((lParam >> 16) & 0xFFFF))
			w32.ScreenToClient(w.wnd.wnd, &pos)
			w.mouseWheel(w.w32ConvertRawMouse(geom.NewPoint(float32(pos.X), float32(pos.Y))),
				geom.NewPoint(float32(int16((wParam>>16)&0xFFFF))/float32(w32.WHEEL_DELTA), 0), w.CurrentKeyModifiers())
			return 0
		case w32.WM_SIZE:
			minimized := wParam == w32.SIZE_MINIMIZED
			if w.minimized != minimized {
				w.minimized = minimized
				if w.MinimizedCallback != nil {
					SafeCall(func() { w.MinimizedCallback(minimized) })
				}
			}
			maximized := wParam == w32.SIZE_MAXIMIZED || (w.maximized && wParam != w32.SIZE_RESTORED)
			if w.maximized != maximized {
				w.maximized = maximized
				if w.MaximizedCallback != nil {
					SafeCall(func() { w.MaximizedCallback(maximized) })
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
			style := w.w32WindowStyle()
			exStyle := w.w32WindowExStyle()
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
			mmi := xruntime.PtrFromUintptr[w32.MINMAXINFO](lParam)
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
			w.w32UpdateFramebufferTransparency()
			return 0
		case w32.WM_GETDPISCALEDSIZE:
			if w32IsWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
				var src, dst w32.RECT
				style := w.w32WindowStyle()
				exStyle := w.w32WindowExStyle()
				curDPI := w32.GetDpiForWindow(w.wnd.wnd)
				newDPI := uint32(wParam & 0xFFFF)
				w32.AdjustWindowRectExForDpi(&src, style, false, exStyle, curDPI)
				w32.AdjustWindowRectExForDpi(&dst, style, false, exStyle, newDPI)
				size := xruntime.PtrFromUintptr[w32.SIZE](lParam)
				// The incoming size is the current window (outer) size at the current DPI. Strip the current frame to
				// recover the client size, scale just the client area to the new DPI so the logical content size is
				// preserved, then add the frame computed for the new DPI. Scaling the client area rather than the whole
				// window rect keeps the snapped size exact.
				clientCX := size.CX - (src.Right - src.Left)
				clientCY := size.CY - (src.Bottom - src.Top)
				if curDPI != 0 && curDPI != newDPI {
					scale := float32(newDPI) / float32(curDPI)
					clientCX = int32(float32(clientCX) * scale)
					clientCY = int32(float32(clientCY) * scale)
				}
				size.CX = clientCX + (dst.Right - dst.Left)
				size.CY = clientCY + (dst.Bottom - dst.Top)
				return 1
			}
		case w32.WM_DPICHANGED:
			if w32IsWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
				rect := xruntime.PtrFromUintptr[w32.RECT](lParam)
				w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOP, rect.Left, rect.Top, rect.Right-rect.Left,
					rect.Bottom-rect.Top, w32.SWP_NOZORDER|w32.SWP_NOACTIVATE)
			}
			if w.ContentScaleCallback != nil {
				scale := float32((wParam>>16)&0xFFFF) / 96
				SafeCall(func() { w.ContentScaleCallback(geom.NewPoint(scale, scale)) })
			}
			// Custom cursors are rasterized per monitor DPI, so refresh to the size that matches the new scale.
			w.adjustToCursorChange()
		case w32.WM_SETCURSOR:
			if lParam&0xFFFF == w32.HTCLIENT {
				w.apiUpdateCursorImage()
				return 1
			}
		}
	}
	return w32.DefWindowProcW(hWnd, uMsg, wParam, lParam)
}

func (w *Window) w32WindowStyle() uint32 {
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

func (w *Window) w32WindowExStyle() uint32 {
	var style uint32
	style = w32.WS_EX_APPWINDOW
	if w.floating {
		style |= w32.WS_EX_TOPMOST
	}
	return style
}

func (w *Window) w32HandleMouseMove(pt geom.Point) {
	pt = w.w32ConvertRawMouse(pt)
	mods := w.CurrentKeyModifiers()
	if !w.wnd.mouseTracked {
		var evt w32.TRACKMOUSEEVENT
		evt.Flags = w32.TME_LEAVE
		evt.HwndTrack = w.wnd.wnd
		w32.TrackMouseEvent(&evt)
		w.wnd.mouseTracked = true
		w.apiUpdateCursorImage()
		w.mouseEnter(pt, mods)
	}
	w.mouseMovedOrDragged(pt, mods)
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
		big = w32CreateIconFromImage(w32ChooseBestImage(images, cxIcon, cyIcon), 0, 0, true)
		small = w32CreateIconFromImage(w32ChooseBestImage(images, cxSmIcon, cySmIcon), 0, 0, true)
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

func w32ChooseBestImage(images []*image.NRGBA, width, height int) *image.NRGBA {
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
	return contentRect.Inset(w.w32FrameInsets().Mul(-1)).Align()
}

func (w *Window) w32FrameInsets() geom.Insets {
	var rect w32.RECT
	style := w.w32WindowStyle()
	exStyle := w.w32WindowExStyle()
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
	return frameRect.Inset(w.w32FrameInsets()).Align()
}

func (w *Window) apiSetContentRect(rect geom.Rect) {
	rect = rect.Inset(w.w32FrameInsets().Mul(-1))
	rect.Size = rect.Size.MulPt(w.apiBackingScale())
	rect = rect.Align()
	w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOP, int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height),
		w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_NOOWNERZORDER)
}

func (w *Window) apiCurrentKeyModifiers() mod.Modifiers {
	var mods mod.Modifiers
	if w32.GetKeyState(w32.VK_SHIFT)&0x8000 != 0 {
		mods |= mod.Shift
	}
	if w32.GetKeyState(w32.VK_CONTROL)&0x8000 != 0 {
		mods |= mod.Control
	}
	if w32.GetKeyState(w32.VK_MENU)&0x8000 != 0 {
		mods |= mod.Option
	}
	if w32.GetKeyState(w32.VK_LWIN)&0x8000 != 0 || w32.GetKeyState(w32.VK_RWIN)&0x8000 != 0 {
		mods |= mod.Command
	}
	if w32.GetKeyState(w32.VK_CAPITAL)&0x0001 != 0 {
		mods |= mod.CapsLock
	}
	if w32.GetKeyState(w32.VK_NUMLOCK)&0x0001 != 0 {
		mods |= mod.NumLock
	}
	return mods
}

func (w *Window) apiUpdateCursorImage() {
	switch {
	case w.cursorHidden:
		if w32BlankCursor == 0 {
			var data [1]byte
			w32BlankCursor = w32.CreateCursor(w32.HINSTANCE(w32.GetModuleHandleW("")), 0, 0, 1, 1, data[:], data[:])
		}
		w32.SetCursor(w32BlankCursor)
	case w.cursor != nil:
		w.w32SetCursor(w.cursor.cursor)
	default:
		w.w32SetCursor(ArrowCursor().cursor)
	}
}

// w32SetCursor applies the native cursor sized for this window's current monitor DPI. Setting a null cursor would hide
// it entirely, so a failed handle creation leaves the existing cursor in place.
func (w *Window) w32SetCursor(c *w32Cursor) {
	if h := c.handle(w.apiBackingScale().X); h != 0 {
		w32.SetCursor(h)
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
		return w.w32ConvertRawMouse(geom.NewPoint(float32(pos.X), float32(pos.Y)))
	}
	return geom.NewPoint(0, 0)
}

func (w *Window) apiBackingScale() geom.Point {
	dpi := w32.GetDpiForWindow(w.wnd.wnd)
	return geom.NewPoint(float32(dpi)/96.0, float32(dpi)/96.0)
}

func (w *Window) apiMinimize() {
	if w.minimized {
		w32.ShowWindow(w.wnd.wnd, w32.SW_RESTORE)
	} else {
		w32.ShowWindow(w.wnd.wnd, w32.SW_MINIMIZE)
	}
}

func (w *Window) apiMaximize() {
	if w.maximized {
		w32.ShowWindow(w.wnd.wnd, w32.SW_RESTORE)
	} else {
		w32.ShowWindow(w.wnd.wnd, w32.SW_MAXIMIZE)
	}
}

func (w *Window) apiAcquireFocusAndBringToFront() {
	// Windows only permits a process to call SetForegroundWindow when it already owns the foreground window. When the
	// app is launched from a command line, the console (a different process) owns the foreground, so the call is denied
	// and the window merely flashes in the taskbar. Temporarily attaching our input thread to the thread that currently
	// owns the foreground window lifts that restriction long enough to bring our window to the front.
	foreground := w32.GetForegroundWindow()
	ourThread := windows.GetCurrentThreadId()
	foregroundThread := w32.GetWindowThreadProcessId(foreground)
	attached := foreground != 0 && foregroundThread != 0 && foregroundThread != ourThread
	if attached {
		attached = w32.AttachThreadInput(ourThread, foregroundThread, true)
	}
	w32.BringWindowToTop(w.wnd.wnd)
	w32.SetForegroundWindow(w.wnd.wnd)
	w32.SetFocus(w.wnd.wnd)
	if attached {
		w32.AttachThreadInput(ourThread, foregroundThread, false)
	}
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

func (w *Window) apiStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
	dataObj := w32.NewDataObject(data, opMask)
	defer dataObj.Release()
	dropSrc := w32.NewDropSource()
	defer dropSrc.Release()
	w.w32InitDragImage(img, origin, dataObj)
	okEffects := uintptr(w32.OpMaskToDropEffect(opMask))
	var effect uint32
	// Windows runs a modal message loop inside DoDragDrop that swallows mouse wheel messages, so install a hook to
	// keep the scroll wheel working while the drag is in progress, matching the behavior on the other platforms.
	w32.InstallDragScrollHook(w.w32HandleDragScroll)
	defer w32.RemoveDragScrollHook()
	w32.DoDragDrop(unsafe.Pointer(dataObj), unsafe.Pointer(dropSrc), okEffects, &effect)

	w.dragSourceFinished()
}

// w32HandleDragScroll forwards a mouse wheel event received during a drag-and-drop operation to the normal wheel
// handling, allowing the view under the cursor to scroll. The drag may have moved over a different top-level window
// than the one that initiated it, so the event is routed to whichever window the cursor is currently over (the
// active drop target), falling back to the source window when the cursor is not over any drop target. pt is in
// screen coordinates.
func (w *Window) w32HandleDragScroll(pt w32.POINT, horizontal bool, delta float32) {
	target := w
	dt := w32.ActiveDropTarget()
	if dt != nil {
		if tw := w32FindWindowByHWND(dt.HWND()); tw != nil {
			target = tw
		}
	}
	mods := target.CurrentKeyModifiers()
	client := pt
	w32.ScreenToClient(target.wnd.wnd, &client)
	where := target.w32ConvertRawMouse(geom.NewPoint(float32(client.X), float32(client.Y)))
	var d geom.Point
	if horizontal {
		d = geom.NewPoint(delta, 0)
	} else {
		d = geom.NewPoint(0, delta)
	}
	target.mouseWheel(where, d, mods)
	// Scrolling moves the content beneath the stationary cursor, so recompute the drop feedback; Windows will not
	// deliver a DragOver of its own until the mouse actually moves.
	if dt != nil {
		dt.RefreshDragOver(pt, mods)
	}
}

// w32InitDragImage stores a drag image into the data object via the shell's drag source helper so that the drag
// image is rendered during the drag. Failures are logged or ignored, since the drag itself still works without an
// image.
func (w *Window) w32InitDragImage(img *Image, originInRoot geom.Point, dataObj *w32.DataObject) {
	if img == nil {
		return
	}
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return
	}
	scale := w.apiBackingScale()
	size := img.LogicalSize().MulPt(scale).Ceil()
	width := int(size.Width)
	height := int(size.Height)
	if width < 1 || height < 1 {
		return
	}
	if nrgba.Rect.Dx() != width || nrgba.Rect.Dy() != height {
		dstRect := image.Rect(0, 0, width, height)
		dst := image.NewNRGBA(dstRect)
		draw.CatmullRom.Scale(dst, dstRect, nrgba, nrgba.Bounds(), draw.Over, nil)
		nrgba = dst
	}
	dc := w32.GetDC(0)
	if dc == 0 {
		return
	}
	defer w32.ReleaseDC(0, dc)
	var ppvBits *byte
	bmp := w32.CreateDIBSection(dc, &w32.BITMAPV5HEADER{
		BV5Width:       int32(width),
		BV5Height:      int32(-height),
		BV5Planes:      1,
		BV5BitCount:    32,
		BV5Compression: w32.BI_BITFIELDS,
		BV5RedMask:     0x00ff0000,
		BV5GreenMask:   0x0000ff00,
		BV5BlueMask:    0x000000ff,
		BV5AlphaMask:   0xff000000,
	}, w32.DIB_RGB_COLORS, &ppvBits, 0, 0)
	if bmp == 0 {
		return
	}
	// The shell renders the drag image as a layered window, which requires premultiplied BGRA pixels.
	target := unsafe.Slice(ppvBits, len(nrgba.Pix))
	for i := 0; i < len(nrgba.Pix); i += 4 {
		a := uint32(nrgba.Pix[i+3])
		target[i] = byte(uint32(nrgba.Pix[i+2]) * a / 255)
		target[i+1] = byte(uint32(nrgba.Pix[i+1]) * a / 255)
		target[i+2] = byte(uint32(nrgba.Pix[i]) * a / 255)
		target[i+3] = byte(a)
	}
	var cursor w32.POINT
	w32.GetCursorPos(&cursor)
	w32.ScreenToClient(w.wnd.wnd, &cursor)
	mouse := w.w32ConvertRawMouse(geom.NewPoint(float32(cursor.X), float32(cursor.Y)))
	offset := w32.POINT{
		X: min(max(int32((mouse.X-originInRoot.X)*scale.X), 0), int32(width-1)),
		Y: min(max(int32((mouse.Y-originInRoot.Y)*scale.Y), 0), int32(height-1)),
	}
	if !w32.InitializeDragImage(dataObj, bmp, w32.SIZE{CX: int32(width), CY: int32(height)}, offset) {
		w32.DeleteObject(w32.HGDIOBJ(bmp))
	}
}

func (w *Window) apiUpdateRegisteredDragTypes(types []*uti.DataType) {
	if w.wnd.dropTarget != nil {
		w32.RevokeDragDrop(w.wnd.wnd)
		w.wnd.dropTarget.Revoke()
		w.wnd.dropTarget = nil
	}
	if len(types) != 0 {
		dt := w32.NewDropTarget(w32DragTargetWindowProxy{w: w})
		if r := w32.RegisterDragDrop(w.wnd.wnd, dt); r != 0 {
			errs.Log(errs.Newf("RegisterDragDrop failed: 0x%X", r))
			dt.Revoke()
			return
		}
		w.wnd.dropTarget = dt
	}
}

func (w *Window) apiDestroy() {
	w.glCtx.apiDestroy()
	if w.wnd.dropTarget != nil {
		w32.RevokeDragDrop(w.wnd.wnd)
		w.wnd.dropTarget.Revoke()
		w.wnd.dropTarget = nil
	}
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

func (w *Window) w32ConvertRawMouse(where geom.Point) geom.Point {
	if w.IsValid() {
		scale := w.apiBackingScale()
		where.X /= scale.X
		where.Y /= scale.Y
	}
	return where
}

func (w *Window) w32UpdateFramebufferTransparency() {
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

type w32DragTargetWindowProxy struct {
	w *Window
}

func (p w32DragTargetWindowProxy) HWND() windows.HWND {
	return p.w.wnd.wnd
}

func (p w32DragTargetWindowProxy) ConvertRawMousePoint(where geom.Point) geom.Point {
	return p.w.w32ConvertRawMouse(where)
}

func (p w32DragTargetWindowProxy) DragEntered(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op {
	return p.w.dragEntered(di, where, mods)
}

func (p w32DragTargetWindowProxy) DragUpdated(di drag.Info, where geom.Point, mods mod.Modifiers) drag.Op {
	return p.w.dragUpdate(di, where, mods)
}

func (p w32DragTargetWindowProxy) DragExited() {
	p.w.dragExit()
}

func (p w32DragTargetWindowProxy) Drop(di drag.Info, where geom.Point, mods mod.Modifiers) bool {
	return p.w.drop(di, where, mods)
}
