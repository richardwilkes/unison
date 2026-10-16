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
	"slices"
	"strings"
	"unsafe"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xruntime"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/internal/pixconv"
	"github.com/richardwilkes/unison/internal/w32"
	"github.com/richardwilkes/unison/internal/w32/uia"
	"golang.org/x/image/draw"
	"golang.org/x/sys/windows"
)

const wndProcClassName = "Unison"

// w32WndClass is a window class the toolkit's windows are created from, registered on first use.
type w32WndClass struct {
	name  string
	atom  w32.ATOM
	style uint32
	icon  bool
}

var (
	// w32MainWndClass is the class of every window but the popup windows. The OpenGL probe windows use it too.
	w32MainWndClass = w32WndClass{
		name:  wndProcClassName,
		style: w32.CS_HREDRAW | w32.CS_VREDRAW | w32.CS_OWNDC,
		icon:  true,
	}
	// w32PopupWndClass is the class of the popup windows, a menu's or a tooltip's: no icon, and the drop shadow the
	// system's menus and tooltips have.
	w32PopupWndClass = w32WndClass{
		name:  "UnisonPopup",
		style: w32.CS_HREDRAW | w32.CS_VREDRAW | w32.CS_OWNDC | w32.CS_DROPSHADOW,
	}
	w32MainInstance    w32.HINSTANCE
	w32WndProcCallback uintptr
	w32BlankCursor     w32.HCURSOR
)

type nativeWindow struct {
	dropTarget *w32.DropTarget
	// uia is the UI Automation adapter serving this window, or nil until an assistive technology has asked about it.
	uia *uia.Window
	// The surface nativePresentCPUPixels draws through: a DIB section selected into a memory DC, whose pixels are
	// written directly and then blitted to the window. See w32EnsurePresentSurface for why it works this way.
	presentPixels []uint32
	presentDC     w32.HDC
	presentBmp    w32.HBITMAP
	presentPrev   w32.HGDIOBJ
	presentWidth  int
	presentHeight int
	wnd           windows.HWND
	bigIcon       w32.HICON
	smallIcon     w32.HICON
	highSurrogate uint16
	mouseTracked  bool
	mouseCaptured bool
	frameDark     bool
	frameThemeSet bool
	// uiaActivating reports that the window is in the middle of building the first snapshot its adapter will be created
	// with. See w32AccessibilityAdapter.
	uiaActivating bool
	// uiaAnsweredForOwner reports that this popup window has answered a WM_GETOBJECT with its owner's fragment root, so
	// its handle has to be withdrawn from UI Automation when it is destroyed. See w32HandleGetObject.
	uiaAnsweredForOwner bool
}

// w32CaptureOp describes how a mouse button message should change the mouse capture state.
type w32CaptureOp int

const (
	w32CaptureKeep w32CaptureOp = iota
	w32CaptureAcquire
	w32CaptureRelease
)

// w32MouseCaptureTransition decides how a button message should change the mouse capture state. The first button press
// acquires the capture, which is then held until the release of the last button, whose up message carries MK_* flags
// showing that no buttons remain down. When the capture has already been lost (e.g. taken by OLE's drag-drop loop),
// releases make no change, so a capture held elsewhere is never stolen or dropped.
func w32MouseCaptureTransition(down, captured bool, wParam w32.WPARAM) w32CaptureOp {
	switch {
	case down && !captured:
		return w32CaptureAcquire
	case !down && captured && !w32AnyMouseButtonDown(wParam):
		return w32CaptureRelease
	default:
		return w32CaptureKeep
	}
}

// w32AnyMouseButtonDown reports whether the MK_* flags in a mouse message's wParam indicate at least one mouse button
// is currently held down.
func w32AnyMouseButtonDown(wParam w32.WPARAM) bool {
	return wParam&(w32.MK_LBUTTON|w32.MK_RBUTTON|w32.MK_MBUTTON|w32.MK_XBUTTON1|w32.MK_XBUTTON2) != 0
}

// w32MouseMessagePoint returns the client-area position packed into a mouse message's lParam. The coordinates must be
// sign-extended from 16 bits: while the mouse is captured, positions outside the client area are delivered, and those
// above or to the left of it are negative.
func w32MouseMessagePoint(lParam w32.LPARAM) geom.Point {
	return geom.NewPoint(float32(int16(lParam&0xFFFF)), float32(int16((lParam>>16)&0xFFFF)))
}

// w32KeyEvent is one logical key transition produced by translating a WM_KEYDOWN/WM_KEYUP message.
type w32KeyEvent struct {
	key     KeyCode
	pressed bool
}

// w32KeyEvents translates a key message into the logical key events to dispatch. A shift release fans out to both
// shift keys, since when both are held Windows delivers a release only for the last one let go. Physical keyboards
// deliver only WM_KEYUP for PrintScreen, so a full press/release pair is synthesized from the release; injected input
// (SendInput, remote-desktop stacks, automation) delivers both messages, so the KEYDOWN must produce nothing or every
// PrintScreen would trigger any bound action twice.
func w32KeyEvents(key KeyCode, wParam w32.WPARAM, pressed bool) []w32KeyEvent {
	switch {
	case !pressed && wParam == w32.VK_SHIFT:
		return []w32KeyEvent{{key: KeyLShift}, {key: KeyRShift}}
	case wParam == w32.VK_SNAPSHOT:
		if pressed {
			return nil
		}
		return []w32KeyEvent{{key: KeyPrintScreen, pressed: true}, {key: KeyPrintScreen}}
	default:
		return []w32KeyEvent{{key: key, pressed: pressed}}
	}
}

// w32RuneFromCharMessage translates the wParam of a WM_CHAR/WM_SYSCHAR message into the rune it produces, using and
// updating the high surrogate carried over from a preceding message. ok is false when the message yields no character:
// a high surrogate (which only completes once its low surrogate arrives), a WM_SYSCHAR (which is an accelerator rather
// than typed text), or a low surrogate with no stored high surrogate, which is a malformed pair that must be discarded
// rather than reported as U+0000.
func w32RuneFromCharMessage(highSurrogate *uint16, sysChar bool, wParam w32.WPARAM) (r rune, ok bool) {
	switch {
	case wParam >= 0xD800 && wParam <= 0xDBFF:
		*highSurrogate = uint16(wParam)
		return 0, false
	case sysChar:
		*highSurrogate = 0
		return 0, false
	case wParam >= 0xDC00 && wParam <= 0xDFFF:
		high := *highSurrogate
		*highSurrogate = 0
		if high == 0 {
			return 0, false
		}
		return 0x10000 + (rune(high)-0xD800)<<10 + ((rune(wParam) & 0xFFFF) - 0xDC00), true
	default:
		*highSurrogate = 0
		return rune(wParam) & 0xFFFF, true
	}
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

// w32Class returns the class the window is created from.
func (w *Window) w32Class() *w32WndClass {
	if w.isPopup() {
		return &w32PopupWndClass
	}
	return &w32MainWndClass
}

// register registers the class unless that has been done already. Every class shares the one window procedure.
func (c *w32WndClass) register() error {
	if c.atom != 0 {
		return nil
	}
	if w32WndProcCallback == 0 {
		w32MainInstance = w32.HINSTANCE(w32.GetModuleHandleW(""))
		w32WndProcCallback = windows.NewCallback(w32WndProc)
	}
	className, err := windows.UTF16FromString(c.name)
	if err != nil {
		return errs.New("unable to create window class name")
	}
	defer runtime.KeepAlive(className)
	wc := w32.WNDCLASSEX{
		Style:     c.style,
		WndProc:   w32WndProcCallback,
		Instance:  w32MainInstance,
		ClassName: &className[0],
	}
	if c.icon {
		wc.Icon = w32.HICON(w32.LoadImageW(0, w32.MakeIntResourceW(w32.IDI_APPLICATION), w32.IMAGE_ICON, 0, 0,
			w32.LR_DEFAULT_SIZE|w32.LR_SHARED))
	}
	if c.atom = w32.RegisterClassExW(&wc); c.atom == 0 {
		return errs.New("unable to register window class")
	}
	return nil
}

func (w *Window) nativeInit() error {
	style := w.w32WindowStyle()
	exStyle := w.w32WindowExStyle()
	class := w.w32Class()
	if err := class.register(); err != nil {
		return err
	}
	var owner windows.HWND
	if w.isPopup() {
		// An owned window stays above its owner and is hidden and destroyed along with it.
		owner = w.owner.wnd.wnd
	}
	w.wnd.wnd = w32.CreateWindowExW(exStyle, class.name, w.w32CreationTitle(), style, 0, 0, 1, 1, owner, 0,
		w32MainInstance, 0)
	if w.wnd.wnd == 0 {
		return errs.New("unable to create window")
	}
	if w.isTooltipWindow() {
		// A layered window is never displayed until SetLayeredWindowAttributes or UpdateLayeredWindow has been called
		// for it, so this has to come before the first show, and a window it fails for could never be shown. With
		// LWA_ALPHA the system keeps a redirection surface for the window, so it is still drawn through its DC, as
		// nativePresentCPUPixels does.
		if !w32.SetLayeredWindowAttributes(w.wnd.wnd, 0, 255, w32.LWA_ALPHA) {
			return errs.New("unable to set the layered window attributes")
		}
	}
	if w.isPopup() {
		// DWM fades a window in as it is shown and out as it is hidden. Turning that off before the first show makes a
		// menu or a tooltip appear and disappear at once. A failure leaves the fades in place, which is harmless.
		disabled := int32(1)
		w32.DwmSetWindowAttribute(w.wnd.wnd, w32.DWMWA_TRANSITIONS_FORCEDISABLED, unsafe.Pointer(&disabled),
			uint32(unsafe.Sizeof(disabled)))
	}
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYDATA, w32.MSGFLT_ALLOW, nil)
	w32.ChangeWindowMessageFilterEx(w.wnd.wnd, w32.WM_COPYGLOBALDATA, w32.MSGFLT_ALLOW, nil)
	w.w32UpdateFramebufferTransparency()
	w.nativeUpdateFrameTheme()
	var rect w32.RECT
	w32.GetClientRect(w.wnd.wnd, &rect)
	w.lastWidth = float32(rect.Right - rect.Left)
	w.lastHeight = float32(rect.Bottom - rect.Top)
	return nil
}

// w32CreationTitle returns the title the window is created with: none for a popup window, whose title is never shown,
// and otherwise the window's own, as w32WindowTitle makes it.
func (w *Window) w32CreationTitle() string {
	if w.isPopup() {
		return ""
	}
	return w32WindowTitle(w.title)
}

func w32WndProc(hWnd windows.HWND, uMsg uint32, wParam w32.WPARAM, lParam w32.LPARAM) uintptr {
	if w := w32FindWindowByHWND(hWnd); w != nil {
		switch uMsg {
		case w32.WM_SETFOCUS:
			// The toolkit's focus stays with a popup window's owner, should the popup ever get the native focus.
			if !w.isPopup() {
				w.gainedFocus()
			}
			return 0
		case w32.WM_KILLFOCUS:
			if !w.isPopup() {
				w.lostFocus()
			}
			return 0
		case w32.WM_MOUSEACTIVATE:
			if result, handled := w.w32MouseActivate(); handled {
				return result
			}
		case w32.WM_SHOWWINDOW:
			if w.w32KeepHidden(lParam) {
				return 0
			}
		case w32.WM_NCLBUTTONDOWN, w32.WM_NCRBUTTONDOWN, w32.WM_NCMBUTTONDOWN, w32.WM_NCXBUTTONDOWN:
			// Left to DefWindowProc afterwards, so a window drag or a caption button works as usual.
			w.w32NonClientMouseDown()
		case w32.WM_CLOSE:
			w.requestClose()
			return 0
		case w32.WM_CHAR, w32.WM_SYSCHAR:
			if r, ok := w32RuneFromCharMessage(&w.wnd.highSurrogate, uMsg == w32.WM_SYSCHAR, wParam); ok {
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
			// An F10 whose key down opened a contextual menu (shift+F10, the chord) is kept from DefWindowProc, its key
			// up included: DefWindowProc notes an F10 press, shifted or not, and on its release sends WM_SYSCOMMAND
			// SC_KEYMENU, whose keyboard loop would take the Down, Enter or Escape meant for the menu that just opened.
			// The key up clears the record as it is delivered, so it is read before.
			chordF10 := wParam == w32.VK_F10 && w.contextMenuChordKey == KeyF10
			for _, e := range w32KeyEvents(key, wParam, pressed) {
				if e.pressed {
					w.keyPressed(e.key, mods)
				} else {
					w.keyReleased(e.key, mods)
				}
			}
			if chordF10 || (wParam == w32.VK_F10 && w.contextMenuChordKey == KeyF10) {
				return 0
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
			target := w
			pos := w32MouseMessagePoint(lParam)
			// Capture the mouse on the first button press so that moves and the eventual release keep being delivered
			// while the cursor is outside the client area. Without capture, Windows stops sending mouse messages once
			// the cursor leaves the window, so a release that happens outside is never seen (leaving stuck button
			// state) and drag-past-edge auto-scroll cannot work.
			if w32MouseCaptureTransition(true, w.wnd.mouseCaptured, wParam) == w32CaptureAcquire {
				w32.SetCapture(hWnd)
				w.wnd.mouseCaptured = true
			} else {
				// A further press while the capture is held is delivered here wherever the pointer is, so one over a
				// popup menu window of ours, or over the owner of the popup holding the capture, is handed on to it, as
				// the owner-events grab does on X11, rather than taken for a press outside every menu.
				target, pos = w.w32CaptureTarget(true, button, pos, w32.WindowFromPoint)
				w.w32HandOverCapture(target)
			}
			target.mouseDown(target.w32ConvertRawMouse(pos), button, target.CurrentKeyModifiers())
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
			// Release the capture taken on button-down once the last button has been released. This is done before
			// the release is delivered rather than after, since the handler may run a nested event loop — a modal
			// dialog put up from a mouse up does exactly that — in which further mouse input must be routed normally
			// rather than to this window. The flag is cleared first, so the WM_CAPTURECHANGED that ReleaseCapture
			// sends is not mistaken for another window taking the capture away.
			released := w32MouseCaptureTransition(false, w.wnd.mouseCaptured, wParam) == w32CaptureRelease
			if released {
				w.wnd.mouseCaptured = false
				w32.ReleaseCapture()
			}
			// The release of a press that was handed on to a popup menu window goes to that popup too.
			target, pos := w.w32CaptureTarget(false, button, w32MouseMessagePoint(lParam), w32.WindowFromPoint)
			target.mouseUp(target.w32ConvertRawMouse(pos), button, target.CurrentKeyModifiers())
			if released && w.w32LeaveUnreported() && !w.nativeCursorInContentArea() {
				w.pointerLeft()
			}
			if uMsg == w32.WM_XBUTTONUP {
				return 1
			}
			return 0
		case w32.WM_MOUSEMOVE:
			w.w32HandleMouseMove(w32MouseMessagePoint(lParam))
			return 0
		case w32.WM_MOUSELEAVE:
			w.wnd.mouseTracked = false
			// The pointer has left the window itself, which takes down its tooltip as well. Moving onto the tooltip
			// window does not count, since hit testing passes over it. Crossing events are suppressed while the mouse
			// is captured (see w32HandleMouseMove); one consumed here is reported by the release that gives up the
			// capture, or by the capture being taken away, instead; see w32LeaveUnreported.
			if !w.wnd.mouseCaptured {
				w.pointerLeft()
			}
			return 0
		case w32.WM_CAPTURECHANGED:
			if windows.HWND(lParam) != hWnd && w.wnd.mouseCaptured {
				w.w32CaptureLost(w.nativeCursorInContentArea())
			}
			return 0
		case w32.WM_MOUSEWHEEL, w32.WM_MOUSEHWHEEL:
			var pos w32.POINT
			pos.X = int32(int16(lParam & 0xFFFF))
			pos.Y = int32(int16((lParam >> 16) & 0xFFFF))
			target := w.w32PopupUnder(pos, w32.WindowFromPoint)
			w32.ScreenToClient(target.wnd.wnd, &pos)
			amount := float32(int16((wParam>>16)&0xFFFF)) / float32(w32.WHEEL_DELTA)
			delta := geom.NewPoint(0, amount)
			if uMsg == w32.WM_MOUSEHWHEEL {
				delta = geom.NewPoint(amount, 0)
			}
			target.mouseWheel(target.w32ConvertRawMouse(geom.NewPoint(float32(pos.X), float32(pos.Y))), delta,
				target.CurrentKeyModifiers())
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
			w.w32RefreshAccessibilityGeometry()
			return 0
		case w32.WM_MOVE:
			w.moved()
			w.w32RefreshAccessibilityGeometry()
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
			scale := w.nativeBackingScale()
			minimum = minimum.MulPt(scale).Ceil()
			maximum = maximum.MulPt(scale).Ceil()
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
			// The suggested rect scales the window's current size by the change in DPI. A popup window was sized for
			// the DPI of the display it was placed on (see nativeSetContentRect), so for one the suggestion would scale
			// it a second time. Keeping its pixel size changes its logical size, with no WM_SIZE to lay the content out
			// for it or WM_PAINT to draw it at the new scale, so both are done here.
			if w.isPopup() {
				w.resized()
				w.MarkForRedraw()
			} else if w32IsWindows10BuildOrGreater(w32.Windows10CreatorsUpdateBuild) {
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
			w.w32RefreshAccessibilityGeometry()
		case w32.WM_SETCURSOR:
			if lParam&0xFFFF == w32.HTCLIENT {
				w.nativeUpdateCursorImage()
				return 1
			}
		case w32.WM_GETOBJECT:
			if result, handled := w.w32HandleGetObject(wParam, lParam); handled {
				return result
			}
		}
	}
	return w32.DefWindowProcW(hWnd, uMsg, wParam, lParam)
}

// w32HandleGetObject answers the message an assistive technology asks a window to describe itself through, reporting
// whether it answered at all: a window that did not must let DefWindowProc have the message, which is what tells the
// caller that nothing here implements the interface it asked for.
//
// Only UI Automation's request is answered. WM_GETOBJECT carries an object identifier in its lParam, and everything
// else that arrives in one — the Microsoft Active Accessibility interfaces for the window frame, the client area, the
// caret, the scroll bars — belongs to something this toolkit does not implement, so the whole of the cost to a window
// that nothing has ever asked about is this one comparison. Answering the wrong identifier would be worse than useless:
// telling a caller that asked for MSAA about a UI Automation provider makes it build a second, empty view of the window
// alongside the real one.
//
// The first UI Automation request for a window is what turns accessibility support on for the application, builds the
// window's first snapshot and creates its adapter, all of it before this returns, since the request has nothing else to
// be answered from.
//
// A popup window answers with its owner's fragment root. The nodes of its menu or tooltip live in the owner's tree, in
// the owner's coordinates and marked as overlays, so UI Automation's ElementFromPoint over a popup menu window
// hit-tests the owner's tree with the owner's geometry and finds the item under the point. Nothing detects UI
// Automation rejecting a root hosted by another window; should that turn out to happen, a popup's request would have to
// be left to DefWindowProc instead.
func (w *Window) w32HandleGetObject(wParam w32.WPARAM, lParam w32.LPARAM) (result uintptr, handled bool) {
	adapter := w.w32GetObjectAdapter(lParam)
	if adapter == nil {
		return 0, false
	}
	return uintptr(uia.ReturnRawElementProvider(w.wnd.wnd, wParam, lParam, adapter.RootUnknown())), true
}

// w32GetObjectAdapter returns the adapter whose fragment root answers a WM_GETOBJECT carrying lParam, or nil if the
// request is left to DefWindowProc; see w32HandleGetObject. A popup window that answers records that it did, so that
// nativeAccessibilityShutdown withdraws its handle.
func (w *Window) w32GetObjectAdapter(lParam w32.LPARAM) *uia.Window {
	if int32(lParam) != uia.RootObjectId {
		return nil
	}
	adapter := w.menuOwner().w32AccessibilityAdapter()
	if adapter == nil {
		return nil
	}
	if w.isPopup() {
		w.wnd.uiaAnsweredForOwner = true
	}
	return adapter
}

// w32CaptureLost handles another window taking the capture away from this one (e.g. OLE's drag-drop loop or a system
// menu/size-move loop). Any button release that happens while it holds the capture will never be delivered here, so the
// releases are synthesized now to avoid stuck button state: this window's own, and those of any popup menu window that
// a press made while this window held the capture was handed on to (see w32CaptureTarget), whose release would have
// come through this window too. They are delivered at offPanelPoint, as endPressesForContextMenu's are, so that the
// panel holding a press does not count its release as a click. A WM_MOUSELEAVE consumed while the capture was held is
// reported now when the cursor is outside the client area, which cursorInContentArea says, as it is by a release that
// gives up the capture; see w32LeaveUnreported.
func (w *Window) w32CaptureLost(cursorInContentArea bool) {
	w.wnd.mouseCaptured = false
	w.synthesizeMouseUpAt(offPanelPoint)
	for _, other := range slices.Clone(windowList) {
		if other.isPopup() && other.IsValid() && other.inMouseDown {
			other.synthesizeMouseUpAt(offPanelPoint)
		}
	}
	if w.w32LeaveUnreported() && !cursorInContentArea {
		w.pointerLeft()
	}
}

// w32NonClientMouseDown handles a press on the window's title bar or frame. It lands outside every menu, so the
// window's open menus close at once rather than only once the press has moved the window.
func (w *Window) w32NonClientMouseDown() {
	if w.root != nil {
		w.root.closeMenuStackStoppingAt(nil)
	}
}

// w32PopupUnder returns the popup menu window of ours at screen position pt, or w's owner when w is a popup menu window
// and the owner is there, or else w, for a mouse message Windows delivered to w although the pointer is over that
// window: a wheel, which goes to the focus window unless Windows is set to scroll inactive windows under the pointer,
// and a popup menu window is never the focus window, or a press made while w holds the capture. A tooltip window takes
// no input and is never returned; WindowFromPoint passes over it anyway. under finds the window at pt; it is
// w32.WindowFromPoint outside the tests.
func (w *Window) w32PopupUnder(pt w32.POINT, under func(w32.POINT) windows.HWND) *Window {
	if target := w32FindWindowByHWND(under(pt)); target != nil &&
		((target.isPopup() && !target.isTooltipWindow()) || target == w.owner) && target.IsValid() {
		return target
	}
	return w
}

// w32HandOverCapture gives target the capture w holds when w is a popup menu window handing a press on to its owner.
// The press closes the menus, w with them, so the owner would otherwise see its release only over its client area. w's
// own presses end, as when any other window takes the capture.
func (w *Window) w32HandOverCapture(target *Window) {
	if target == w || target.isPopup() {
		return
	}
	w.w32CaptureLost(w.nativeCursorInContentArea())
	w32.SetCapture(target.wnd.wnd)
	target.wnd.mouseCaptured = true
}

// w32CaptureTarget returns the window a button message Windows delivered to w is for, along with the message's
// position pos, which is in w's client area, converted into the target's: for a press made while w holds the capture,
// the popup menu window of ours, or w's owner, under the pointer (see w32PopupUnder), and for a release, the popup menu
// window recording button as down, which was handed the press. Either is w itself when there is no such window. under
// finds the window at a screen position; it is w32.WindowFromPoint outside the tests.
func (w *Window) w32CaptureTarget(pressed bool, button int, pos geom.Point,
	under func(w32.POINT) windows.HWND,
) (target *Window, where geom.Point) {
	target = w
	screen := w32.POINT{X: int32(pos.X), Y: int32(pos.Y)}
	if pressed {
		w32.ClientToScreen(w.wnd.wnd, &screen)
		target = w.w32PopupUnder(screen, under)
	} else {
		for _, other := range windowList {
			if other.isPopup() && other.IsValid() && other.pressedButtons[button] {
				target = other
				break
			}
		}
	}
	if target == w {
		return w, pos
	}
	if !pressed {
		w32.ClientToScreen(w.wnd.wnd, &screen)
	}
	w32.ScreenToClient(target.wnd.wnd, &screen)
	return target, geom.NewPoint(float32(screen.X), float32(screen.Y))
}

// w32LeaveUnreported reports whether the window, having just given up or lost the capture, has a pointer exit it was
// never told of: the WM_MOUSELEAVE of a press dragged out of the window arrives while the mouse is captured and is
// consumed rather than delivered (see WM_MOUSELEAVE in wndProc), and once the press has ended outside the window
// nothing reports the leave again, so the window reports it itself (see Window.pointerLeft). Tracking still armed
// means the pointer never left, and the capture held again, by the release's handler having run a nested event loop,
// say, means the press is not over yet.
func (w *Window) w32LeaveUnreported() bool {
	return w.IsValid() && !w.wnd.mouseCaptured && !w.wnd.mouseTracked
}

// w32MouseActivate answers WM_MOUSEACTIVATE, reporting whether it answered at all. A click on a popup window, a menu's
// or a tooltip's, must not activate it, since that would take the activation, and with it the keys that drive the
// menus, from its owner; a tooltip window, being transparent to hit testing, never receives one in any case. Every
// other window leaves the message to DefWindowProc.
func (w *Window) w32MouseActivate() (result uintptr, handled bool) {
	if w.isPopup() {
		return w32.MA_NOACTIVATE, true
	}
	return 0, false
}

// w32KeepHidden reports whether a WM_SHOWWINDOW with the given lParam is to be answered without DefWindowProc, so that
// the window stays hidden: the one that shows an owner's popups again as the owner is restored, for a tooltip window
// with no tooltip showing in it. Windows hides the popups of an owner being minimized, and marks them to be shown on
// restore, before the owner learns of the minimize and takes its tooltip down, and hiding a window that is already
// hidden does not clear that mark.
func (w *Window) w32KeepHidden(lParam w32.LPARAM) bool {
	return lParam == w32.SW_PARENTOPENING && w.isTooltipWindow() &&
		(w.owner.tooltipWindow != w || !w.owner.tooltipShowing())
}

// w32RefreshAccessibilityGeometry tells the window's assistive-technology adapter, if it has one, that the screen
// position, size or scale of its content area has changed. A window that is not being described pays one nil check.
func (w *Window) w32RefreshAccessibilityGeometry() {
	if w.ax != nil {
		w.apiAccessibilityGeometryChanged()
	}
}

func (w *Window) w32WindowStyle() uint32 {
	if w.isPopup() {
		return w32.WS_POPUP | w32.WS_CLIPSIBLINGS | w32.WS_CLIPCHILDREN
	}
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
	if w.isPopup() {
		// Never activated, kept off the taskbar and out of Alt+Tab, and topmost, as the other platforms' menu levels
		// are, so that a popup is not hidden by a floating window of the application or another program's
		// always-on-top window; being owned keeps it above its owner, but above nothing else.
		style := uint32(w32.WS_EX_NOACTIVATE | w32.WS_EX_TOOLWINDOW | w32.WS_EX_TOPMOST)
		if w.isTooltipWindow() {
			// Layered and transparent, as comctl32 styles its own tooltips: hit testing, WindowFromPoint's and every
			// other process's included, passes over such a window, so the pointer and clicks go to whatever is
			// beneath it. Answering WM_NCHITTEST with HTTRANSPARENT would not do, since that only passes them on to
			// windows of the same thread.
			style |= w32.WS_EX_LAYERED | w32.WS_EX_TRANSPARENT
		}
		return style
	}
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
	// While the mouse is captured, moves are delivered even when the cursor is outside the client area, where arming
	// TrackMouseEvent would post an immediate WM_MOUSELEAVE and cause enter/exit churn on every move. Crossing events
	// are therefore suppressed for the duration of the capture; normal tracking resumes on the first move after the
	// capture is released.
	if !w.wnd.mouseTracked && !w.wnd.mouseCaptured {
		var evt w32.TRACKMOUSEEVENT
		evt.Flags = w32.TME_LEAVE
		evt.HwndTrack = w.wnd.wnd
		w32.TrackMouseEvent(&evt)
		w.wnd.mouseTracked = true
		w.nativeUpdateCursorImage()
		w.mouseEnter(pt, mods)
	}
	w.mouseMovedOrDragged(pt, mods)
}

func (w *Window) nativeSetTitle(title string) {
	w32.SetWindowTextW(w.wnd.wnd, w32WindowTitle(title))
}

// w32WindowTitle returns title cut at its first NUL, if it has one, since a title is a C string to Windows and
// CreateWindowExW and SetWindowTextW refuse one holding a NUL, which would otherwise cost a window its creation or
// leave its title unchanged.
func w32WindowTitle(title string) string {
	title, _, _ = strings.Cut(title, "\x00")
	return title
}

func (w *Window) nativeSetTitleIcons(images []*image.NRGBA) {
	var big, small w32.HICON
	owned := false
	if len(images) > 0 {
		cxIcon := w32.GetSystemMetrics(w32.SM_CXICON)
		cyIcon := w32.GetSystemMetrics(w32.SM_CYICON)
		cxSmIcon := w32.GetSystemMetrics(w32.SM_CXSMICON)
		cySmIcon := w32.GetSystemMetrics(w32.SM_CYSMICON)
		big = w32CreateIconFromImage(w32ChooseBestImage(images, cxIcon, cyIcon), 0, 0, true)
		small = w32CreateIconFromImage(w32ChooseBestImage(images, cxSmIcon, cySmIcon), 0, 0, true)
		owned = true
	} else {
		// Revert to the class icons. These handles are shared and owned by the system, so they must never be passed to
		// DestroyIcon; bigIcon/smallIcon only ever record icons created by this window.
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
	if owned {
		w.wnd.bigIcon = big
		w.wnd.smallIcon = small
	} else {
		w.wnd.bigIcon = 0
		w.wnd.smallIcon = 0
	}
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

func (w *Window) nativeDisplay() *Display {
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

func (w *Window) nativeFrameRect() geom.Rect {
	var rect w32.RECT
	w32.GetWindowRect(w.wnd.wnd, &rect)
	r := rectFromW32Rect(rect)
	r.Size = r.Size.DivPt(w.nativeBackingScale())
	return r.Align()
}

func (w *Window) nativeFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	return w32ApplyFrameInsets(contentRect, w.w32FrameInsets(), w.nativeBackingScale(), false).Align()
}

// w32FrameInsets returns the window's frame insets in raw screen pixels.
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
	return geom.NewInsets(-r.Y, -r.X, r.Bottom(), r.Right())
}

// w32ApplyFrameInsets converts between a window's frame rect and its content rect. Window rects on this platform keep
// their origin in the raw global pixel space that display rects use, while their size is in logical, 1x-scale units
// (see Display.nativeUsableInWindowUnits), so the insets — which are raw pixels — cannot simply be handed to
// Rect.Inset: the position must move by the raw amount while the size changes by the scaled-down amount. Getting this
// wrong makes a ContentRect/SetContentRect round trip (e.g. Pack) creep by insets×(scale−1) pixels on every call. Pass
// true for remove to go from a frame rect to a content rect, false for the reverse.
func w32ApplyFrameInsets(r geom.Rect, insets geom.Insets, scale geom.Point, remove bool) geom.Rect {
	if !remove {
		insets = insets.Mul(-1)
	}
	r.X += insets.Left
	r.Y += insets.Top
	r.Width -= insets.Width() / scale.X
	r.Height -= insets.Height() / scale.Y
	return r
}

func (w *Window) nativeEnsureOnDisplay() {
	var r w32.RECT
	w32.GetWindowRect(w.wnd.wnd, &r)
	frameRect := rectFromW32Rect(r)
	d := w.nativeDisplay()
	revisedRect := d.FitRectOnto(frameRect)
	if frameRect != revisedRect {
		revisedRect.Size = revisedRect.Size.DivPt(d.Scale)
		w.SetFrameRect(revisedRect.Align())
	}
}

func (w *Window) nativeContentRect() geom.Rect {
	var rect w32.RECT
	w32.GetClientRect(w.wnd.wnd, &rect)
	var pt w32.POINT
	w32.ClientToScreen(w.wnd.wnd, &pt)
	r := geom.NewRect(float32(pt.X), float32(pt.Y), float32(rect.Right), float32(rect.Bottom))
	r.Size = r.Size.DivPt(w.nativeBackingScale())
	return r.Align()
}

// nativePointToScreen adds pt, scaled to raw pixels, to the client origin, which window rects keep in raw pixels.
func (w *Window) nativePointToScreen(pt geom.Point) geom.Point {
	return w.w32ClientOrigin().Add(pt.MulPt(w.nativeBackingScale()))
}

// nativePointFromScreen is the inverse of nativePointToScreen.
func (w *Window) nativePointFromScreen(pt geom.Point) geom.Point {
	return pt.Sub(w.w32ClientOrigin()).DivPt(w.nativeBackingScale())
}

// w32ClientOrigin returns the screen position, in raw pixels, of the top-left corner of the window's client area.
func (w *Window) w32ClientOrigin() geom.Point {
	var pt w32.POINT
	w32.ClientToScreen(w.wnd.wnd, &pt)
	return geom.NewPoint(float32(pt.X), float32(pt.Y))
}

func (w *Window) nativeContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	return w32ApplyFrameInsets(frameRect, w.w32FrameInsets(), w.nativeBackingScale(), true).Align()
}

func (w *Window) nativeSetContentRect(rect geom.Rect) {
	scale := w.nativeBackingScale()
	if w.isPopup() {
		// A popup window is placed from a rect fitted to the display it lands on, in that display's scale (see
		// menu.ensureOnDisplay and tooltipWindowRect), so its size is converted with that display's scale rather than
		// its own, which is still that of the display it was created or last placed on. The DPI change Windows reports
		// for the move is left alone for the same reason; see WM_DPICHANGED.
		scale = w32PopupContentScale(rect.Point, AllDisplays(), scale)
	}
	rect = w32ApplyFrameInsets(rect, w.w32FrameInsets(), scale, false)
	rect.Size = rect.Size.MulPt(scale)
	rect = rect.Align()
	w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOP, int32(rect.X), int32(rect.Y), int32(rect.Width), int32(rect.Height),
		w32.SWP_NOACTIVATE|w32.SWP_NOZORDER|w32.SWP_NOOWNERZORDER)
}

// w32PopupContentScale returns the scale a popup window's content rect placed at pt, in the raw pixel space display
// rects use, is sized with: that of the display whose frame holds pt, or own, the window's own scale, when none does.
func w32PopupContentScale(pt geom.Point, displays []*Display, own geom.Point) geom.Point {
	for _, d := range displays {
		if pt.In(d.Frame) {
			return d.Scale
		}
	}
	return own
}

func (w *Window) nativeCurrentKeyModifiers() mod.Modifiers {
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

func (w *Window) nativeUpdateCursorImage() {
	switch {
	case w.cursorHidden:
		if w32BlankCursor == 0 {
			w32BlankCursor = w32CreateBlankCursor()
		}
		// A zero handle here means the creation failed; SetCursor(NULL) also removes the cursor from the screen, so
		// the fallback still hides it.
		w32.SetCursor(w32BlankCursor)
	case w.cursor != nil:
		w.w32SetCursor(w.cursor.cursor)
	default:
		w.w32SetCursor(ArrowCursor().cursor)
	}
}

// w32CreateBlankCursor creates a fully transparent cursor at the system cursor size. CreateCursor documents that size,
// not an arbitrary one, as what it accepts.
func w32CreateBlankCursor() w32.HCURSOR {
	width := w32.GetSystemMetrics(w32.SM_CXCURSOR)
	height := w32.GetSystemMetrics(w32.SM_CYCURSOR)
	if width < 1 || height < 1 {
		width = 32
		height = 32
	}
	andMask, xorMask := w32BlankCursorMasks(width, height)
	return w32.CreateCursor(w32.HINSTANCE(w32.GetModuleHandleW("")), 0, 0, uint32(width), uint32(height), andMask,
		xorMask)
}

// w32BlankCursorMasks builds the AND and XOR masks for a fully transparent monochrome cursor. Win32 combines the two
// per-pixel as (screen AND andMask) XOR xorMask, so transparency requires an AND bit of 1 with an XOR bit of 0; the
// all-zero masks that a zeroed buffer yields instead paint opaque black. Each scanline of a monochrome mask is padded
// out to a WORD boundary, so the buffers are sized from that stride rather than from the pixel width.
func w32BlankCursorMasks(width, height int) (andMask, xorMask []byte) {
	stride := ((width + 15) / 16) * 2
	andMask = make([]byte, stride*height)
	for i := range andMask {
		andMask[i] = 0xFF
	}
	return andMask, make([]byte, stride*height)
}

// w32SetCursor applies the native cursor sized for this window's current monitor DPI. Setting a null cursor would hide
// it entirely, so a nil (destroyed) cursor or a failed handle creation leaves the existing cursor in place.
func (w *Window) w32SetCursor(c *w32Cursor) {
	if c == nil {
		return
	}
	if h := c.handle(w.nativeBackingScale().X); h != 0 {
		w32.SetCursor(h)
	}
}

func (w *Window) nativeCursorInContentArea() bool {
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
	return w32InClientArea(pos, topLeft, bottomRight)
}

// w32InClientArea returns true if screen position pos lies in the client area whose corners, in screen coordinates, are
// topLeft and bottomRight. bottomRight is the first pixel outside the area, as a RECT's right and bottom edges are.
func w32InClientArea(pos, topLeft, bottomRight w32.POINT) bool {
	return pos.X >= topLeft.X && pos.X < bottomRight.X && pos.Y >= topLeft.Y && pos.Y < bottomRight.Y
}

func (w *Window) nativeCursorPosition() geom.Point {
	var pos w32.POINT
	if w32.GetCursorPos(&pos) {
		w32.ScreenToClient(w.wnd.wnd, &pos)
		return w.w32ConvertRawMouse(geom.NewPoint(float32(pos.X), float32(pos.Y)))
	}
	return geom.NewPoint(0, 0)
}

func (w *Window) nativeBackingScale() geom.Point {
	dpi := w.w32DPI()
	return geom.NewPoint(float32(dpi)/96.0, float32(dpi)/96.0)
}

// w32DPI returns the effective DPI for the window. GetDpiForWindow only exists as of the Windows 10 Anniversary Update
// and its lazy proc would panic when called on an older build, so builds before that — which the SetProcessDpiAwareness
// startup fallback still supports — instead query the DPI of the monitor hosting the window via shcore, present since
// Windows 8.1.
func (w *Window) w32DPI() uint32 {
	return w32WindowDPI(w32IsWindows10BuildOrGreater(w32.Windows10AnniversaryUpdateBuild),
		func() uint32 { return w32.GetDpiForWindow(w.wnd.wnd) },
		func() uint32 {
			dpi, _ := w32.GetDpiForMonitor(w32.MonitorFromWindow(w.wnd.wnd, w32.MONITOR_DEFAULTTONEAREST),
				w32.MDT_EFFECTIVE_DPI)
			return dpi
		})
}

// w32WindowDPI picks which DPI query to use based on whether GetDpiForWindow is available, invoking only the chosen
// one, since calling the other could panic on a proc the running Windows build does not export. A query that fails and
// reports zero is mapped to the default 96, because callers turn the result into scale factors that are divided by.
func w32WindowDPI(haveGetDpiForWindow bool, getDpiForWindow, getDpiForMonitor func() uint32) uint32 {
	var dpi uint32
	if haveGetDpiForWindow {
		dpi = getDpiForWindow()
	} else {
		dpi = getDpiForMonitor()
	}
	if dpi == 0 {
		return 96
	}
	return dpi
}

func (w *Window) nativeMinimize() {
	if w.minimized {
		w32.ShowWindow(w.wnd.wnd, w32.SW_RESTORE)
	} else {
		w32.ShowWindow(w.wnd.wnd, w32.SW_MINIMIZE)
	}
}

func (w *Window) nativeMaximize() {
	if w.maximized {
		w32.ShowWindow(w.wnd.wnd, w32.SW_RESTORE)
	} else {
		w32.ShowWindow(w.wnd.wnd, w32.SW_MAXIMIZE)
	}
}

func (w *Window) nativeAcquireFocusAndBringToFront() {
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

// nativeCancelMouseCapture gives up the capture taken on button-down, as DefWindowProc does for WM_CANCELMODE. The flag
// is cleared first, so the WM_CAPTURECHANGED that ReleaseCapture sends is not mistaken for another window taking the
// capture away.
func (w *Window) nativeCancelMouseCapture() {
	if !w.wnd.mouseCaptured {
		return
	}
	w.wnd.mouseCaptured = false
	w32.ReleaseCapture()
}

// nativeMenuStackChanged is a no-op: a platform may grab the pointer while a menu stack is open, but this one need not.
func (w *Window) nativeMenuStackChanged(_ bool) {
}

// nativeUpdateFrameTheme asks DWM to draw the system frame (the title bar and its buttons) to match the current dark
// mode state. DWM uses a light frame for every window that does not explicitly ask for a dark one, so without this a
// dark-themed application keeps a light title bar.
func (w *Window) nativeUpdateFrameTheme() {
	if w.undecorated || w.wnd.wnd == 0 {
		return // No system-drawn frame to theme.
	}
	dark := IsDarkModeEnabled()
	if w.wnd.frameThemeSet && w.wnd.frameDark == dark {
		return
	}
	// Recorded before the call is attempted: a failure means the attribute is unsupported on this version of
	// Windows, so there is nothing to retry until the state itself changes.
	w.wnd.frameThemeSet = true
	w.wnd.frameDark = dark
	attr, enabled := w32FrameThemeAttribute(dark, w32IsWindows10BuildOrGreater)
	if !w32.DwmSetWindowAttribute(w.wnd.wnd, attr, unsafe.Pointer(&enabled), uint32(unsafe.Sizeof(enabled))) {
		return
	}
	if w.nativeVisible() {
		// Windows 10 does not repaint the frame on its own when the attribute changes on a visible window.
		w32.SetWindowPos(w.wnd.wnd, 0, 0, 0, 0, 0,
			w32.SWP_NOMOVE|w32.SWP_NOSIZE|w32.SWP_NOZORDER|w32.SWP_NOACTIVATE|w32.SWP_FRAMECHANGED)
	}
}

// w32FrameThemeAttribute works out the DwmSetWindowAttribute call that asks for a dark (or light) frame: the attribute
// index and the BOOL to store in it. The dark mode attribute was renumbered from 19 to 20 in Windows 10 build 18985,
// so the index depends on isBuildOrGreater, which reports whether the running system is at least a given build of
// Windows 10. It is kept apart from the call itself so the choice can be tested without a window.
func w32FrameThemeAttribute(dark bool, isBuildOrGreater func(build uint32) bool) (attr uint32, enabled int32) {
	attr = w32.DWMWA_USE_IMMERSIVE_DARK_MODE
	if !isBuildOrGreater(w32.Windows10ImmersiveDarkModeBuild) {
		attr = w32.DWMWA_USE_IMMERSIVE_DARK_MODE_PRE_20H1
	}
	if dark {
		enabled = 1
	}
	return attr, enabled
}

func (w *Window) nativeVisible() bool {
	return windows.IsWindowVisible(w.wnd.wnd)
}

func (w *Window) nativeShow() {
	if w.isPopup() {
		// Raised to the top of the topmost band as it is shown, which ShowWindow does not promise for a window it does
		// not activate. The tooltip window is reused, merely hidden between tooltips, so a topmost window shown since,
		// such as a floating window of the application, would otherwise be free to cover it. Raising a popup that is
		// already showing does no harm. The owner is left where it is in the z-order.
		w32.SetWindowPos(w.wnd.wnd, w32.HWND_TOPMOST, 0, 0, 0, 0,
			w32.SWP_NOMOVE|w32.SWP_NOSIZE|w32.SWP_NOACTIVATE|w32.SWP_NOOWNERZORDER|w32.SWP_SHOWWINDOW)
		return
	}
	w32.ShowWindow(w.wnd.wnd, w32.SW_SHOWNA)
}

func (w *Window) nativeHide() {
	w32.ShowWindow(w.wnd.wnd, w32.SW_HIDE)
}

func (w *Window) nativeStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
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
	scale := w.nativeBackingScale()
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
	// The shell renders the drag image as a layered window, which requires premultiplied BGRA pixels. The DIB holds
	// exactly width*height*4 bytes with no stride padding, so the conversion runs a row at a time through PixOffset:
	// ToNRGBA and the rescale above both hand back packed, zero-origin pixels, but a source whose stride is wider than
	// its rows would otherwise walk past the end of the DIB. The DIB's pixel store is written in place, never replaced.
	rowBytes := width * 4
	target := unsafe.Slice(ppvBits, height*rowBytes)
	for y := range height {
		si := nrgba.PixOffset(nrgba.Rect.Min.X, nrgba.Rect.Min.Y+y)
		di := y * rowBytes
		pixconv.PremulBGRA(target[di:di+rowBytes], nrgba.Pix[si:si+rowBytes])
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

func (w *Window) nativeUpdateRegisteredDragTypes(types []*uti.DataType) {
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

func (w *Window) nativeDestroy() {
	w.glCtx.nativeDestroy()
	// The adapter has handed UI Automation a provider for this window handle and raises the event that says the window
	// has closed, so it has to be shut down while the handle is still valid. Window.destroy has normally done this
	// already; a window torn down by any other path has not.
	w.nativeAccessibilityShutdown()
	w.w32DisposePresentSurface()
	if w.wnd.dropTarget != nil {
		w32.RevokeDragDrop(w.wnd.wnd)
		w.wnd.dropTarget.Revoke()
		w.wnd.dropTarget = nil
	}
	// Given up before the window goes, so that the capture release destroying it would cause is not taken for another
	// window taking the capture away and answered with releases to the panels of a window that is already disposed of.
	// A popup menu window closed by a key while a press is held on one of its items is destroyed holding the capture.
	w.nativeCancelMouseCapture()
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
		scale := w.nativeBackingScale()
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
