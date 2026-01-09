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
	wnd windows.HWND
}

func isWindows10BuildOrGreater(build uint32) bool {
	cond := w32.VerSetConditionMask(0, w32.VER_MAJORVERSION, w32.VER_GREATER_EQUAL)
	cond = w32.VerSetConditionMask(cond, w32.VER_MINORVERSION, w32.VER_GREATER_EQUAL)
	cond = w32.VerSetConditionMask(cond, w32.VER_BUILDNUMBER, w32.VER_GREATER_EQUAL)
	return w32.RtlVerifyVersionInfo(&w32.OSVERSIONINFOEXW{
		MajorVersion: 10,
		MinorVersion: 0,
		BuildNumber:  build,
	}, w32.VER_MAJORVERSION|w32.VER_MINORVERSION|w32.VER_BUILDNUMBER, cond) == 0
}

func findWindowByHWND(wnd windows.HWND) *Window {
	if i := slices.IndexFunc(windowList, func(w *Window) bool {
		return w.wnd.wnd == wnd
	}); i != -1 {
		return windowList[i]
	}
	return nil
}

func (w *Window) initNativeWindow(cfg *WindowConfig) bool {
	style := w.windowStyle()
	exStyle := w.windowExStyle()
	if mainWndClass == 0 {
		mainInstance = w32.HINSTANCE(w32.GetModuleHandleW(""))
		className, err := windows.UTF16FromString(wndProcClassName)
		if err != nil {
			return false
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
			return false
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
		return false
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
	return true
}

func wndProc(hWnd windows.HWND, uMsg uint32, wParam w32.WPARAM, lParam w32.LPARAM) uintptr {
	// TODO: IMPLEMENT!
	return 0
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

func (w *Window) frameRect() geom.Rect {
	if w.IsValid() {
		left, top, right, bottom := w.wnd.GetFrameSize()
		r := geom.NewRect(float32(left), float32(top), float32(right-left), float32(bottom-top))
		sx, sy := w.wnd.GetContentScale()
		r.X /= sx
		r.Y /= sy
		r.Width /= sx
		r.Height /= sy
		return r
	}
	return geom.NewRect(0, 0, 1, 1)
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

func (w *Window) backingScale() geom.Point {
	dpi := w32.GetDpiForWindow(w.wnd.wnd)
	return geom.NewPoint(float32(dpi)/96.0, float32(dpi)/96.0)
}

func (w *Window) convertRawMouseLocationForPlatform(where geom.Point) geom.Point {
	if w.IsValid() {
		scale := w.backingScale()
		where.X /= scale.X
		where.Y /= scale.Y
	}
	return where
}

// CurrentKeyModifiers returns the current key modifiers, which is usually the same as calling .LastKeyModifiers(),
// however, on platforms that are using native menus, this will also capture modifier changes that occurred while the
// menu is being displayed.
func (w *Window) CurrentKeyModifiers() Modifiers {
	return w.LastKeyModifiers()
}
