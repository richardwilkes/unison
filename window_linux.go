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
	"bytes"
	"fmt"
	"image"
	"log/slog"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/internal/x11"
	"golang.org/x/image/draw"
)

type nativeWindow struct {
	dndInfo *x11DragInfo
	// frameColorScheme is the path _KDE_NET_WM_COLOR_SCHEME was last set to, or empty if it is not set.
	frameColorScheme  string
	id                x11.WindowID
	parent            x11.WindowID
	colorMap          x11.ColorMapID
	gc                x11.GCID // lazily created; only used by the CPU-rendering presentation path
	dndSource         x11.WindowID
	dndVersion        uint32
	lastX             float32
	lastY             float32
	borderTop         uint32
	borderLeft        uint32
	borderBottom      uint32
	borderRight       uint32
	depth             byte
	borderValid       bool
	cursorWasHidden   bool
	awaitingConfigure bool
	frameDark         bool
	frameThemeSet     bool
	// menuGrab is true while this window holds the pointer grab taken for its open menu stack; see
	// nativeMenuStackChanged.
	menuGrab bool
	// pressGrab is true while this window holds the pointer grab that stands in for the implicit grab of a press made
	// while a menu grab was in effect; see nativeMenuStackChanged.
	pressGrab bool
	// pointerInside is true while the pointer is in the window, as its crossing events last reported; see
	// Window.x11PointerCrossed.
	pointerInside bool
}

func x11FindWindow(id x11.WindowID) *Window {
	for _, w := range windowList {
		if w.wnd.id == id {
			return w
		}
	}
	return nil
}

func (w *Window) nativeInit() error {
	if !cpuRenderingActive && !w.cpuOnly {
		if err := w.glCtx.x11PrepareWindow(w); err != nil {
			fallbackToCPURendering(err)
		}
	}
	w.wnd.parent = x11Conn.RootWindow()
	visual := x11Conn.DefaultVisual()
	depth := x11Conn.DefaultDepth()
	switch {
	case w.glCtx.hasVisual:
		visual = w.glCtx.visual
		depth = w.glCtx.depth
	case (cpuRenderingActive || w.cpuOnly) && w.transparent:
		// Without GLX to supply an alpha-capable framebuffer configuration, a transparent window needs a 32-bit ARGB
		// visual so the CPU-rendered pixels keep their alpha channel.
		if argb := x11FindARGBVisual(); argb != 0 {
			visual = argb
			depth = 32
		}
	}
	w.wnd.depth = depth
	if w.wnd.colorMap = x11Conn.CreateColormap(visual, w.wnd.parent, false); w.wnd.colorMap == 0 {
		return errs.New("failed to create X11 color map for window")
	}
	mask := x11.WindowMaskBorderPixel | x11.WindowMaskColorMap | x11.WindowMaskEventMask
	attrs := &x11.WindowCreationAttributes{
		ColorMap: w.wnd.colorMap,
		EventMask: x11.EventMaskStructureNotify |
			x11.EventMaskKeyPress |
			x11.EventMaskKeyRelease |
			x11.EventMaskPointerMotion |
			x11.EventMaskButtonPress |
			x11.EventMaskButtonRelease |
			x11.EventMaskExposure |
			x11.EventMaskFocusChange |
			x11.EventMaskVisibilityChange |
			x11.EventMaskEnterWindow |
			x11.EventMaskLeaveWindow |
			x11.EventMaskPropertyChange,
	}
	popup := w.isPopup()
	if popup {
		// The window manager never frames, places or focuses an override-redirect window. SaveUnder lets a server that
		// supports it restore what the menu covered without asking the windows beneath to repaint.
		mask |= x11.WindowMaskOverrideRedirect | x11.WindowMaskSaveUnder
		attrs.OverrideRedirect = true
		attrs.SaveUnder = true
	}
	if w.wnd.id = x11Conn.CreateWindow(w.wnd.parent, 0, 0, 1, 1, 0, x11.WindowClassInputOutput, depth, visual, mask,
		attrs); w.wnd.id == 0 {
		x11Conn.FreeColormap(w.wnd.colorMap)
		w.wnd.colorMap = 0
		return errs.New("failed to create X11 window")
	}
	if popup {
		w.x11SetPopupProperties()
		return nil
	}
	if w.undecorated {
		w.x11SetDecorated(false)
	}
	if w.floating {
		buf := x11.NewWriter(4)
		buf.Atom(x11Conn.Atoms.NetWMStateAbove)
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMState, x11.AtomAtom, 32, x11.PropModeReplace,
			buf.Retrieve())
	}

	buf := x11.NewWriter(8)
	buf.Atom(x11Conn.Atoms.WMDeleteWindow)
	buf.Atom(x11Conn.Atoms.NetWMPing)
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.WMProtocols, x11.AtomAtom, 32, x11.PropModeReplace, buf.Retrieve())

	buf = x11.NewWriter(4)
	buf.Uint32(uint32(os.Getpid()))
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.WMPID, x11.AtomCardinal, 32, x11.PropModeReplace, buf.Retrieve())

	var kind x11.Atom
	switch w.kind {
	case WindowKindDialog:
		kind = x11Conn.Atoms.NetWMWindowTypeDialog
	case WindowKindMenu:
		kind = x11Conn.Atoms.NetWMWindowTypeMenu
	case WindowKindTooltip:
		kind = x11Conn.Atoms.NetWMWindowTypeTooltip
	default:
		kind = x11Conn.Atoms.NetWMWindowTypeNormal
	}
	buf = x11.NewWriter(4)
	buf.Atom(kind)
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMWindowType, x11.AtomAtom, 32, x11.PropModeReplace,
		buf.Retrieve())

	// Associate dialog windows with the window that spawned them via WM_TRANSIENT_FOR, the ICCCM/EWMH-correct hint that
	// tells the window manager which top-level a dialog belongs to, letting it center and stack the dialog over its
	// parent (and, under XWayland, treat it as a child rather than an independent toplevel). The owner is the currently
	// active window, but since focus notifications can be delayed (e.g. immediately after a menu window closes, the
	// underlying window's focus event may not have arrived yet), fall back to the frontmost window so the dialog is
	// still tied to the window it was raised from rather than being left unowned, which typically causes the window
	// manager to place it on the primary display instead of the display the user is working on.
	if w.kind == WindowKindDialog {
		owner := ActiveWindow()
		if owner == nil {
			owner = FrontmostWindow()
		}
		if owner != nil && owner != w && owner.wnd.id != 0 {
			buf = x11.NewWriter(4)
			buf.WindowID(owner.wnd.id)
			x11Conn.ChangeProperty(w.wnd.id, x11.AtomWMTransientFor, x11.AtomWindow, 32, x11.PropModeReplace,
				buf.Retrieve())
		}
	}

	buf = x11.NewWriter(36)
	buf.Uint32(2) // StateHint
	buf.Zero(4)
	buf.Uint32(1) // NormalState
	buf.Zero(24)
	x11Conn.ChangeProperty(w.wnd.id, x11.AtomWMHints, x11.AtomWMHints, 32, x11.PropModeReplace, buf.Retrieve())

	var sizeHints x11.WindowSizeHints
	if w.notResizable {
		sizeHints.Flags |= x11.WSHMPMinSize | x11.WSHMPMaxSize
		sizeHints.MinWidth = 1
		sizeHints.MinHeight = 1
		sizeHints.MaxWidth = 1
		sizeHints.MaxHeight = 1
	}
	sizeHints.Flags |= x11.WSHMPPosition | x11.WSHMPWinGravity
	sizeHints.WinGravity = x11.StaticGravity
	x11Conn.SetSizeHints(w.wnd.id, x11.AtomWMNormalHints, &sizeHints)
	w.nativeSetWMClass()
	w.nativeSetTitle(w.title)
	w.nativeUpdateFrameTheme()
	x11Conn.Flush()
	return nil
}

// x11SetPopupProperties sets the properties of a popup menu window. Those only a window manager reads (WM_PROTOCOLS,
// WM_HINTS, the size hints, the title and the Motif decoration hints) are left off, since none manages the window, but
// compositors read its type, and WM_CLASS and the PID tie it to the application.
func (w *Window) x11SetPopupProperties() {
	buf := x11.NewWriter(4)
	buf.Uint32(uint32(os.Getpid()))
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.WMPID, x11.AtomCardinal, 32, x11.PropModeReplace, buf.Retrieve())
	buf = x11.NewWriter(4)
	buf.Atom(x11Conn.Atoms.NetWMWindowTypePopupMenu)
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMWindowType, x11.AtomAtom, 32, x11.PropModeReplace,
		buf.Retrieve())
	// KWin's fading popups effect fades an unmanaged window out over 600 ms as it closes, which this skips. Its 150 ms
	// fade-in remains.
	buf = x11.NewWriter(4)
	buf.Uint32(1)
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.KDENetWMSkipCloseAnimation, x11.AtomCardinal, 32,
		x11.PropModeReplace, buf.Retrieve())
	w.nativeSetWMClass()
	x11Conn.Flush()
}

// nativeSetWMClass sets the WM_CLASS property (the ICCCM instance and class names) on the window. Desktop environments
// use this to associate the window with its launcher/.desktop file (for the taskbar/dock icon, application grouping,
// etc.), so the class name is set to the application identifier, which is what a .desktop file's StartupWMClass entry
// should match.
func (w *Window) nativeSetWMClass() {
	x11Conn.ChangeProperty(w.wnd.id, x11.AtomWMClass, x11.AtomString, 8, x11.PropModeReplace, wmClassData())
}

// wmClassData builds the WM_CLASS property payload: the ICCCM instance and class names as a pair of null-terminated
// Latin-1 strings. The instance name is the application's command name and the class name is the application
// identifier, which is what a .desktop file's StartupWMClass entry should match.
func wmClassData() []byte {
	instance := xos.AppCmdName
	if instance == "" {
		instance = xos.AppName
	}
	class := xos.AppIdentifier
	if class == "" {
		class = instance
	}
	data := make([]byte, 0, len(instance)+len(class)+2)
	data = append(data, instance...)
	data = append(data, 0)
	data = append(data, class...)
	data = append(data, 0)
	return data
}

func (w *Window) nativeSetTitle(title string) {
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMName, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(title))
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMIconName, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(title))
	x11Conn.Flush()
}

func (w *Window) nativeSetTitleIcons(images []*image.NRGBA) {
	if len(images) == 0 {
		x11Conn.DeleteProperty(w.wnd.id, x11Conn.Atoms.NetWMIcon)
	} else {
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMIcon, x11.AtomCardinal, 32, x11.PropModeReplace,
			x11.IconData(images))
	}
	x11Conn.Flush()
}

func (w *Window) nativeDisplay() *Display {
	return BestDisplayForRect(w.nativeFrameRect())
}

func (w *Window) nativeFrameRect() geom.Rect {
	return w.nativeFrameRectForContentRect(w.nativeContentRect())
}

func (w *Window) nativeFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
	if !w.undecorated {
		top, left, bottom, right := w.x11Border()
		scale := w.BackingScale()
		contentRect.X -= float32(left) / scale.X
		contentRect.Y -= float32(top) / scale.Y
		contentRect.Width += float32(left+right) / scale.X
		contentRect.Height += float32(top+bottom) / scale.Y
	}
	return contentRect
}

func (w *Window) nativeEnsureOnDisplay() {
	frameRect := w.nativeFrameRect()
	revisedRect := w.nativeDisplay().FitRectOnto(frameRect)
	if frameRect != revisedRect {
		w.SetFrameRect(revisedRect)
	}
}

func (w *Window) nativeContentRect() geom.Rect {
	if w.wnd.awaitingConfigure {
		return w.lastContentRect
	}
	info, err := x11Conn.GetGeometry(x11.DrawableID(w.wnd.id))
	if err != nil {
		errs.Log(err)
		return geom.Rect{}
	}
	root := x11Conn.RootWindow()
	var x, y int16
	if x, y, _, _, err = x11Conn.TranslateCoordinates(w.wnd.id, root, 0, 0); err != nil {
		errs.Log(err)
	} else {
		info.X = x
		info.Y = y
	}
	r := geom.NewRect(float32(info.X), float32(info.Y), float32(info.Width), float32(info.Height))
	scale := w.BackingScale()
	r.X /= scale.X
	r.Y /= scale.Y
	r.Width /= scale.X
	r.Height /= scale.Y
	return r
}

func (w *Window) nativePointToScreen(pt geom.Point) geom.Point {
	return w.nativeContentRect().Point.Add(pt)
}

func (w *Window) nativePointFromScreen(pt geom.Point) geom.Point {
	return pt.Sub(w.nativeContentRect().Point)
}

func (w *Window) nativeContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
	if !w.undecorated {
		top, left, bottom, right := w.x11Border()
		scale := w.BackingScale()
		frameRect.X += float32(left) / scale.X
		frameRect.Y += float32(top) / scale.Y
		frameRect.Width -= float32(left+right) / scale.X
		frameRect.Height -= float32(top+bottom) / scale.Y
	}
	return frameRect
}

func (w *Window) nativeSetContentRect(rect geom.Rect) {
	scale := w.BackingScale()
	rect.X *= scale.X
	rect.Y *= scale.Y
	rect.Width *= scale.X
	rect.Height *= scale.Y
	// Size hints are read only by a window manager, which leaves an override-redirect popup menu window alone.
	if !w.isPopup() {
		sizeHints := x11.WindowSizeHints{
			Flags:      x11.WSHMUSPosition | x11.WSHMPPosition | x11.WSHMUSSize | x11.WSHMPSize | x11.WSHMPWinGravity,
			X:          int32(rect.X),
			Y:          int32(rect.Y),
			Width:      uint32(rect.Width),
			Height:     uint32(rect.Height),
			WinGravity: x11.StaticGravity,
		}
		if w.notResizable {
			sizeHints.Flags |= x11.WSHMPMinSize | x11.WSHMPMaxSize
			sizeHints.MinWidth = uint32(rect.Width)
			sizeHints.MinHeight = uint32(rect.Height)
			sizeHints.MaxWidth = uint32(rect.Width)
			sizeHints.MaxHeight = uint32(rect.Height)
		}
		x11Conn.SetSizeHints(w.wnd.id, x11.AtomWMNormalHints, &sizeHints)
	}
	x11Conn.ConfigureWindow(w.wnd.id, x11.ConfigureWindowMaskX|x11.ConfigureWindowMaskY|
		x11.ConfigureWindowMaskWidth|x11.ConfigureWindowMaskHeight, &x11.ConfigureWindowRequest{
		X:      int16(rect.X),
		Y:      int16(rect.Y),
		Width:  uint16(rect.Width),
		Height: uint16(rect.Height),
	})
	w.wnd.awaitingConfigure = true
	x11Conn.Flush()
}

func (w *Window) x11ConvertRawMouse(where geom.Point) geom.Point {
	return where.DivPt(w.BackingScale())
}

func (w *Window) nativeCurrentKeyModifiers() mod.Modifiers {
	return x11CurrentKeyModifiers()
}

func (w *Window) nativeUpdateCursorImage() {
	switch {
	case w.cursorHidden:
		if !w.wnd.cursorWasHidden {
			w.wnd.cursorWasHidden = true
			x11Conn.ExtXFixes.HideCursor(w.wnd.id)
		}
	case w.cursor != nil:
		if w.wnd.cursorWasHidden {
			w.wnd.cursorWasHidden = false
			x11Conn.ExtXFixes.ShowCursor(w.wnd.id)
		}
		x11Conn.ChangeWindowAttributes(w.wnd.id, x11.WindowMaskCursor, &x11.WindowCreationAttributes{
			Cursor: w.cursor.cursor,
		})
	default:
		if w.wnd.cursorWasHidden {
			w.wnd.cursorWasHidden = false
			x11Conn.ExtXFixes.ShowCursor(w.wnd.id)
		}
		x11Conn.ChangeWindowAttributes(w.wnd.id, x11.WindowMaskCursor, &x11.WindowCreationAttributes{})
	}
}

func (w *Window) nativeCursorInContentArea() bool {
	qpr := x11Conn.QueryPointer(w.wnd.id)
	if qpr == nil {
		return false
	}
	return w.x11ConvertRawMouse(geom.NewPoint(float32(qpr.RootX), float32(qpr.RootY))).In(w.nativeContentRect())
}

func (w *Window) nativeCursorPosition() geom.Point {
	qpr := x11Conn.QueryPointer(w.wnd.id)
	if qpr == nil {
		return geom.Point{}
	}
	return w.x11ConvertRawMouse(geom.NewPoint(float32(qpr.WinX), float32(qpr.WinY)))
}

func (w *Window) nativeBackingScale() geom.Point {
	scale, err := x11Conn.ContentScale()
	if err != nil {
		errs.Log(err)
		return geom.NewPoint(1, 1)
	}
	return geom.NewPoint(scale, scale)
}

func (w *Window) nativeMinimize() {
	if w.minimized {
		x11Conn.DeiconifyWindow(w.wnd.id)
	} else {
		x11Conn.IconifyWindow(w.wnd.id)
	}
}

func (w *Window) nativeMaximize() {
	if w.maximized {
		x11Conn.DemaximizeWindow(w.wnd.id)
	} else {
		x11Conn.MaximizeWindow(w.wnd.id)
	}
}

// x11SetMinimized records that the window manager has iconified the window, or restored it, which is what a taskbar,
// switching workspaces and "show desktop" all do — and what Window.Minimize asks for, since all it can do is send
// WM_CHANGE_STATE and wait. Every one of them arrives here as the PropertyNotify the window manager sends when it sets
// WM_STATE, so this is where the application learns that the state has actually changed rather than where it was asked
// for; the only cost of the overlap is the second, harmless axMarkForPublish a Window.Minimize leads to.
func (w *Window) x11SetMinimized(minimized bool) {
	if minimized != w.minimized {
		w.minimized = minimized
		// A window that is no longer on the screen has to be taken out of what an assistive technology has been told,
		// and the event loop does that for the windows it finds it cannot draw, exactly as it does for Window.Minimize;
		// a window that has come back is described afresh the same way. Without this, only a window that held the
		// keyboard focus would be withdrawn — by the FocusOut that follows it — and a background window a person cannot
		// see would go on being listed as showing and visible. See Window.axMarkForPublish.
		w.axMarkForPublish()
		if w.MinimizedCallback != nil {
			SafeCall(func() { w.MinimizedCallback(minimized) })
		}
	}
}

func (w *Window) x11SetMaximized(maximized bool) {
	if maximized != w.maximized {
		w.maximized = maximized
		if w.MaximizedCallback != nil {
			SafeCall(func() { w.MaximizedCallback(maximized) })
		}
	}
}

// nativeAcquireFocusAndBringToFront raises the window and gives it the input focus.
func (w *Window) nativeAcquireFocusAndBringToFront() {
	x11Conn.ConfigureWindow(w.wnd.id, x11.ConfigureWindowMaskStackMode, &x11.ConfigureWindowRequest{
		StackMode: x11.StackModeAbove,
	})
	x11Conn.FocusWindow(w.wnd.id)
	x11Conn.Flush()
}

// nativeCancelMouseCapture releases the grab that stands in for a press's implicit grab, if this window holds one (see
// nativeMenuStackChanged). Otherwise the only hold a press installs is the server's implicit grab, which the server
// itself releases when the last button comes up. The explicit GrabPointer used for the source side of drag & drop
// belongs to that loop, which releases it on its own way out, and a menu grab lasts as long as the menus do.
func (w *Window) nativeCancelMouseCapture() {
	w.x11EndPressGrab()
}

// x11MenuGrabEventMask is the set of pointer events the grab taken for an open menu stack reports to the grab window.
// Crossings are left out, since with owner events on they would report the pointer crossing other clients' windows.
const x11MenuGrabEventMask = x11.EventMaskButtonPress | x11.EventMaskButtonRelease | x11.EventMaskPointerMotion

// nativeMenuStackChanged grabs the pointer for the window's menu stack when it opens and releases the grab when it
// empties. Owner events are on, so the pointer events over the application's own windows are reported to them as
// usual, and only those over other clients' windows (the desktop, other applications, the owner's own frame) come to
// the owner, relative to it; x11MenuGrabInterceptsPress turns a press there into the menus closing. Without the grab, a
// click on the desktop, a panel or any other window that does not take the focus would leave the menus open, since no
// focus change follows it, and a focus-follows-mouse window manager, which sees no pointer crossings during a grab,
// would move the focus away as the pointer crossed a menu.
//
// A press anywhere but on a menu closes the menus first (in rootPanel.preMouseDown, or in x11ProcessEvent for one
// outside our windows), and the menus hold nothing a drag starts from, so a drag starts while the grab is held only
// from a press that was already down when a menu was opened from the keyboard. The drag's grab replaces this one and
// the drag's end releases it, so nativeStartDrag records the replacement and takes this grab again afterwards, if the
// menus are still open. A drag begun by a press that closed the menus replaces the grab held for that press (see
// below). A menu opened from the keyboard during a drag takes no grab, since the drag loop holds the pointer and its
// release would end this grab too; x11RestoreMenuGrab gives it one once the drag is over.
//
// A press made while the grab is held forms no implicit grab of its own. When the stack empties with such a press
// still down in a window other than a popup menu window, the pointer is grabbed for that window instead, without owner
// events, as the implicit grab would have been, and x11ProcessEvent releases it when the last button comes up.
// Otherwise a release made over another client's window would never arrive and the press would be left stuck down.
func (w *Window) nativeMenuStackChanged(open bool) {
	if open {
		if w.wnd.menuGrab || w.wnd.id == 0 || dragSource != nil {
			return
		}
		w.x11TakeMenuGrab()
		return
	}
	if !w.wnd.menuGrab {
		return
	}
	w.wnd.menuGrab = false
	var holder *Window
	for _, one := range windowList {
		if one.IsValid() && one.inMouseDown && !one.isPopup() && one.wnd.id != 0 {
			holder = one
			break
		}
	}
	if holder != nil && x11Conn.GrabPointer(holder.wnd.id, false,
		x11MenuGrabEventMask|x11.EventMaskEnterWindow|x11.EventMaskLeaveWindow, 0) {
		holder.wnd.pressGrab = true
	} else {
		x11Conn.UngrabPointer()
	}
	x11Conn.Flush()
}

// x11TakeMenuGrab grabs the pointer for the window's open menu stack; see nativeMenuStackChanged, which decides when.
func (w *Window) x11TakeMenuGrab() {
	// The grab shows the arrow wherever the pointer is. Without a cursor of its own, X would show the owner's current
	// cursor everywhere outside the owner, menus included, which after a right-click on a text field is an I-beam.
	var cursor x11.CursorID
	if arrow := ArrowCursor(); arrow != nil {
		cursor = arrow.cursor
	}
	if x11Conn.GrabPointer(w.wnd.id, true, x11MenuGrabEventMask, cursor) {
		// The new grab replaced any held for a press, whose events x11RoutePointerEvent now sends where they belong,
		// and any held for another window's menu stack, since the server holds one pointer grab per client.
		x11ForgetPressGrabs()
		x11ForgetMenuGrabs()
		w.wnd.menuGrab = true
	}
	x11Conn.Flush()
}

// x11ForgetMenuGrabs records that no window holds the pointer grab for its menu stack, for when another grab has just
// replaced it.
func x11ForgetMenuGrabs() {
	for _, w := range windowList {
		if w.wnd != nil {
			w.wnd.menuGrab = false
		}
	}
}

// x11DragReplacedMenuGrab records that the grab a drag has just taken replaced the one held for an open menu stack, if
// one was held, and returns the window that held it, for x11RestoreMenuGrab once the drag is over.
func x11DragReplacedMenuGrab() *Window {
	owner := x11MenuGrabOwner()
	if owner != nil {
		owner.wnd.menuGrab = false
	}
	return owner
}

// x11RestoreMenuGrab takes the grab for the menu stack of owner, the window whose grab the drag replaced, if its menus
// are still open, or else for that of any other top-level window with menus open, once the drag has released its own
// grab: a menu opened from the keyboard during the drag took none (see nativeMenuStackChanged), and the drag is the
// last chance to give it one. It is called before the drag is forgotten, so it does not go through
// nativeMenuStackChanged, which takes no grab during a drag. A nil owner is one that held no grab.
func x11RestoreMenuGrab(owner *Window) {
	if !owner.x11MenuGrabWanted() {
		owner = nil
		for _, w := range windowList {
			if w.x11MenuGrabWanted() {
				owner = w
				break
			}
		}
	}
	if owner != nil {
		owner.x11TakeMenuGrab()
	}
}

// x11MenuGrabWanted returns true if the window has menus open and holds no grab for them.
func (w *Window) x11MenuGrabWanted() bool {
	return w.IsValid() && !w.isPopup() && w.wnd.id != 0 && !w.wnd.menuGrab && w.root != nil &&
		len(w.root.openMenuPanels) != 0
}

// x11EndPressGrab releases the grab the window holds in place of a press's implicit grab, if any.
func (w *Window) x11EndPressGrab() {
	if w.wnd.pressGrab {
		w.wnd.pressGrab = false
		x11Conn.UngrabPointer()
		x11Conn.Flush()
	}
}

// x11ForgetPressGrabs records that no window holds a grab in place of a press's implicit grab, for when another grab
// has just replaced it.
func x11ForgetPressGrabs() {
	for _, w := range windowList {
		if w.wnd != nil {
			w.wnd.pressGrab = false
		}
	}
}

// x11MenuGrabOwner returns the window holding the pointer grab for its open menu stack, or nil if none does.
func x11MenuGrabOwner() *Window {
	for _, w := range windowList {
		if w.wnd != nil && w.wnd.menuGrab {
			return w
		}
	}
	return nil
}

// x11PressHolder returns a valid window of ours with a press in progress, preferring a popup menu window, or nil if
// there is none.
func x11PressHolder() *Window {
	var found *Window
	for _, w := range windowList {
		if w.IsValid() && w.inMouseDown {
			if w.isPopup() {
				return w
			}
			if found == nil {
				found = w
			}
		}
	}
	return found
}

// nativeUpdateFrameTheme asks the window manager to draw the frame (the title bar and its buttons) to match the current
// dark mode state. The window manager owns the frame on Linux, so all a client can do is set a property on the window
// that asks for one. No single property is honored by every window manager, so both of the ones in use are set, and
// each window manager ignores the one it does not know. See x11UpdateGTKThemeVariant and x11UpdateKDEColorScheme.
func (w *Window) nativeUpdateFrameTheme() {
	if w.undecorated || w.wnd.id == 0 {
		return // No window-manager-drawn frame to theme.
	}
	dark := IsDarkModeEnabled()
	sentGTK := w.x11UpdateGTKThemeVariant(dark)
	sentKDE := w.x11UpdateKDEColorScheme(dark)
	if sentGTK || sentKDE {
		x11Conn.Flush()
	}
}

// x11UpdateGTKThemeVariant sets the _GTK_THEME_VARIANT property that GTK sets on its own windows. Window managers that
// draw their frames with the GTK theme, such as Cinnamon's Muffin and GNOME's Mutter prior to version 44, draw the frame
// with the variant of the theme that the property names. Mutter 44 and later ignore the property and draw every frame to
// match the desktop's color scheme instead, which is what the application follows too unless SetThemeMode() has chosen
// otherwise.
//
// Light is named explicitly rather than expressed by removing the property, since a window manager gives a window
// without it the desktop's preferred variant, which is the dark one when the desktop prefers dark. A theme without a
// "light" variant falls back to itself, which is the light look for all but the themes that are dark to begin with.
// It returns true if a request was sent.
func (w *Window) x11UpdateGTKThemeVariant(dark bool) bool {
	if w.wnd.frameThemeSet && w.wnd.frameDark == dark {
		return false
	}
	w.wnd.frameThemeSet = true
	w.wnd.frameDark = dark
	variant := "light"
	if dark {
		variant = "dark"
	}
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.GTKThemeVariant, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(variant))
	return true
}

// x11UpdateKDEColorScheme sets the _KDE_NET_WM_COLOR_SCHEME property that KDE's own applications set on their windows
// to the path of a color scheme file, and that KWin draws the frame of a window that carries it from. Without it, a
// frame takes the colors of the desktop's own color scheme, which already match the application whenever its dark mode
// state is the desktop's. So the property is set only when SetThemeMode() has chosen the other state, or when the
// desktop's state cannot be determined, and is removed again once it is no longer needed. That keeps the frame in the
// scheme the user chose for as long as it matches, rather than replacing it with a stock one. The scheme named is
// Breeze's own light or dark one, and if that is not installed, the frame is left to the desktop's scheme. It returns
// true if a request was sent.
func (w *Window) x11UpdateKDEColorScheme(dark bool) bool {
	var scheme string
	if !nativeIsColorModeTrackingPossible() || dark != nativeIsDarkModeEnabled() {
		light, darkScheme := x11KDEColorSchemes()
		scheme = light
		if dark {
			scheme = darkScheme
		}
	}
	if scheme == w.wnd.frameColorScheme {
		return false
	}
	w.wnd.frameColorScheme = scheme
	if scheme == "" {
		x11Conn.DeleteProperty(w.wnd.id, x11Conn.Atoms.KDENetWMColorScheme)
	} else {
		// KWin asks for the property as a STRING, and is given nothing for one of any other type.
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.KDENetWMColorScheme, x11.AtomString, 8, x11.PropModeReplace,
			[]byte(scheme))
	}
	return true
}

// x11KDEColorSchemes returns the paths of Breeze's light and dark color schemes, found where KDE looks for color
// schemes, which is the color-schemes directory within each of the XDG data directories. Either is empty if it is not
// installed. They are looked up the first time they are needed, and not again.
var x11KDEColorSchemes = sync.OnceValues(func() (light, dark string) {
	dirs := linuxXDGDataDirs()
	return linuxFindDataFile(dirs, "color-schemes/BreezeLight.colors"),
		linuxFindDataFile(dirs, "color-schemes/BreezeDark.colors")
})

// linuxXDGDataDirs returns the XDG data directories, in the order a file is looked for in them: the user's own
// ($XDG_DATA_HOME) and then the system's ($XDG_DATA_DIRS), with the defaults the XDG Base Directory Specification gives
// for each that is not set. A relative path is ignored, as the specification requires.
func linuxXDGDataDirs() []string {
	system := os.Getenv("XDG_DATA_DIRS")
	if system == "" {
		system = "/usr/local/share:/usr/share"
	}
	dirs := append([]string{xos.AppDataDir(false)}, strings.Split(system, ":")...)
	return slices.DeleteFunc(dirs, func(dir string) bool { return !filepath.IsAbs(dir) })
}

// linuxFindDataFile returns the path of the named file within the first of dirs that holds a readable one, or an empty
// string if none does.
func linuxFindDataFile(dirs []string, name string) string {
	for _, dir := range dirs {
		if p := filepath.Join(dir, name); xos.FileIsReadable(p) {
			return p
		}
	}
	return ""
}

func (w *Window) nativeVisible() bool {
	return x11Conn.IsWindowVisible(w.wnd.id)
}

// x11FilteredWaitDone repairs the redraw wake-up bookkeeping after a call that may have blocked in one of
// internal/x11's filtered waits. It is called on the UI thread, from every place in this package that can reach such a
// wait: showing a window, answering a selection request, asking the window manager for a window's frame widths, and
// each clipboard or drag & drop transfer.
//
// A filtered wait takes the event it is after out of the queue and leaves everything else in it, but the wake-up
// Window.MarkForRedraw posts is not an event: x11.Conn.PostEmptyEvent sends a bare token, which a filtered wait
// consumes and then loops on without returning. The pending redraw the token was posted for is left with nothing
// coming to wake the event loop for it, while redrawWakePending goes on saying that this pass has already asked, so
// every further request of the pass posts nothing either — and a redraw marked after that pass's draw loop, from the
// focus callbacks that ToFront's activation provokes, say, leaves the window unpainted until an unrelated event
// happens to arrive.
//
// Posting another token whenever a redraw is still pending is what puts that right. A spare wake-up, for a wait that
// swallowed nothing, costs one more pass of the loop that finds the redraw already done. With nothing pending there is
// no token worth replacing, so the flag is merely cleared, leaving the rest of the pass free to post for itself.
func x11FilteredWaitDone() {
	if !redrawWakePending {
		return
	}
	if len(redrawSet) == 0 {
		redrawWakePending = false
		return
	}
	apiPostEmptyEvent()
}

// x11ShowTimeout bounds how long nativeShow waits for the window manager to actually map a newly shown window.
// MapWindow is intercepted by the window manager (SubstructureRedirect), so a hung window manager — or one that keeps
// the window unmapped, such as by assigning it to a non-current desktop — may never produce a VisibilityNotify. Since
// filtered waits ignore PostEmptyEvent wake-ups, an unbounded wait here would hang the whole application permanently.
const x11ShowTimeout = 2 * time.Second

func (w *Window) nativeShow() {
	if w.nativeVisible() {
		return
	}
	// The window manager does not intercept the map of an override-redirect popup menu window, and the X server itself
	// sends the VisibilityNotify, so the wait below ends at once for one.
	x11Conn.MapWindow(w.wnd.id)
	x11Conn.WaitForWindowVisibility(w.wnd.id, x11ShowTimeout)
	x11FilteredWaitDone()
	// Draw the window now that it is visible. The filtered wait above discards the nil wake-up events posted by
	// MarkForRedraw; x11FilteredWaitDone puts a token back, but the redraw it asks for does not happen until the event
	// loop comes round again. When a window is shown from within an event handler (such as a modal dialog raised while
	// handling the window manager's close request), the window would stand blank until then. Drawing here guarantees
	// the freshly shown window is painted regardless of event ordering, and also covers the case where the X server
	// never sends an Expose.
	w.draw()
}

func (w *Window) nativeHide() {
	x11Conn.UnmapWindow(w.wnd.id)
	x11Conn.Flush()
}

// x11DnDFinishedTimeout is the maximum amount of time to wait for the drop target to send XdndFinished after a drop.
const x11DnDFinishedTimeout = 2 * time.Second

// x11DnDStatusTimeout is the maximum amount of time to wait for the drop target to answer the final XdndPosition when
// the mouse is released before the answer has arrived.
const x11DnDStatusTimeout = 250 * time.Millisecond

// x11DnDContinuousInterval is the interval between synthesized XdndPosition messages so that drop targets keep
// receiving updates while the mouse is stationary.
const x11DnDContinuousInterval = 50 * time.Millisecond

func (w *Window) nativeStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
	defer w.dragSourceFinished()
	x11Conn.SetDnDData(data...)
	// Publish the full set of actions we permit so that targets can choose among them
	buf := x11.NewWriter(8)
	if opMask&drag.Copy != 0 {
		buf.Atom(x11Conn.Atoms.DnDActionCopy)
	}
	if opMask&drag.Move != 0 {
		buf.Atom(x11Conn.Atoms.DnDActionMove)
	}
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.DnDActionList, x11.AtomAtom, 32, x11.PropModeReplace, buf.Retrieve())
	if !x11Conn.GrabPointer(w.wnd.id, false,
		x11.EventMaskButtonPress|x11.EventMaskButtonRelease|x11.EventMaskPointerMotion, 0) {
		slog.Warn("unable to grab the pointer to start a drag")
		return
	}
	// This grab replaced any held for a press or for an open menu stack (see nativeMenuStackChanged), and the drag's
	// release ends it; the menu grab is taken again afterwards.
	x11ForgetPressGrabs()
	menuOwner := x11DragReplacedMenuGrab()
	defer func() {
		x11Conn.UngrabPointer()
		x11RestoreMenuGrab(menuOwner)
	}()
	imgWnd := w.newX11DragImageWindow(img, origin)
	defer imgWnd.dispose()
	w.x11DragLoop(x11DnDActionForOp(opMask), imgWnd)
}

// x11DragLoop runs the source side of an XDND drag, processing events until the drag completes. Events unrelated to
// the drag are dispatched normally so that the windows of this application continue to function, which also allows
// them to act as the drop target.
func (w *Window) x11DragLoop(suggestedAction x11.Atom, imgWnd *x11DragImageWindow) {
	const (
		dragStateTracking = iota
		dragStateAwaitFinalStatus
		dragStateAwaitFinished
	)
	targets := x11Conn.DnDTargets()
	awareCache := make(map[x11.WindowID]uint32)
	state := dragStateTracking
	var deadline time.Time
	var curTarget x11.WindowID
	var curLocal *Window
	var curVersion uint32
	var awaitingStatus, hasPending, accepted bool
	var pendingX, pendingY int16
	var lastRootX, lastRootY int16
	var timestamp uint32

	sendPosition := func(rootX, rootY int16) {
		if curTarget == 0 {
			return
		}
		if awaitingStatus {
			pendingX = rootX
			pendingY = rootY
			hasPending = true
			return
		}
		awaitingStatus = true
		x11Conn.SendDnDPosition(w.wnd.id, curTarget, rootX, rootY, timestamp, suggestedAction)
		x11Conn.Flush()
	}

	updateTarget := func(rootX, rootY int16) {
		lastRootX = rootX
		lastRootY = rootY
		target, version, local := x11FindDnDAwareWindow(rootX, rootY, awareCache, imgWnd.windowID())
		curLocal = local
		if target != curTarget {
			if curTarget != 0 {
				x11Conn.SendDnDLeave(w.wnd.id, curTarget)
			}
			curTarget = target
			curVersion = min(version, x11.DnDVersion)
			awaitingStatus = false
			hasPending = false
			accepted = false
			if curTarget != 0 {
				x11Conn.SendDnDEnter(w.wnd.id, curTarget, curVersion, targets)
			}
			x11Conn.Flush()
		}
		sendPosition(rootX, rootY)
	}

	leaveAndStop := func() {
		if curTarget != 0 {
			x11Conn.SendDnDLeave(w.wnd.id, curTarget)
			x11Conn.Flush()
		}
	}

	// dropOrStop either sends XdndDrop and transitions to waiting for XdndFinished, or ends the drag. It returns true
	// when the loop should exit.
	dropOrStop := func() bool {
		if !accepted {
			leaveAndStop()
			return true
		}
		x11Conn.SendDnDDrop(w.wnd.id, curTarget, timestamp)
		x11Conn.Flush()
		state = dragStateAwaitFinished
		deadline = time.Now().Add(x11DnDFinishedTimeout)
		return false
	}

	// handleEvent processes a single event, returning true when the drag is complete and the loop should exit.
	handleEvent := func(e x11.Event) (done bool) {
		switch ev := e.(type) {
		case nil:
		case *x11.MotionNotifyEvent:
			if state != dragStateTracking {
				return false
			}
			// Coalesce any additional queued motion events into this one, but not past a queued ButtonRelease: motion
			// behind the release happened after the button came up, so folding it in here would send a final
			// XdndPosition for a position the pointer only reached after the drop was committed, dropping at the
			// wrong spot — or on a different target entirely — whenever events have backed up.
			for {
				next := x11Conn.PollEventsBefore(func(pe x11.Event) bool {
					_, ok := pe.(*x11.MotionNotifyEvent)
					return ok
				}, func(pe x11.Event) bool {
					_, ok := pe.(*x11.ButtonReleaseEvent)
					return ok
				})
				m, ok := next.(*x11.MotionNotifyEvent)
				if !ok {
					break
				}
				ev = m
			}
			timestamp = ev.Time
			imgWnd.moveTo(ev.RootX, ev.RootY)
			updateTarget(ev.RootX, ev.RootY)
		case *x11.ButtonPressEvent:
			// Mouse wheel scrolls arrive as presses of buttons 4-7. Deliver them to the window of this application
			// that is under the pointer so that scrolling continues to work during the drag, then refresh the drop
			// target's position, since the content beneath the pointer may have scrolled. Other button presses are
			// ignored while the drag is in progress.
			if state == dragStateTracking && ev.Detail >= 4 && ev.Detail <= 7 && curLocal != nil {
				x, y, _, _, err := x11Conn.TranslateCoordinates(x11Conn.RootWindow(), curLocal.wnd.id, ev.RootX,
					ev.RootY)
				if err != nil {
					errs.Log(err)
					return false
				}
				var delta geom.Point
				switch ev.Detail {
				case 4:
					delta = geom.NewPoint(0, 1)
				case 5:
					delta = geom.NewPoint(0, -1)
				case 6:
					delta = geom.NewPoint(1, 0)
				case 7:
					delta = geom.NewPoint(-1, 0)
				}
				curLocal.mouseWheel(curLocal.x11ConvertRawMouse(geom.NewPoint(float32(x), float32(y))), delta,
					x11TranslateModifierState(ev.State))
				sendPosition(ev.RootX, ev.RootY)
			}
		case *x11.ButtonReleaseEvent:
			if state != dragStateTracking || (ev.Detail >= 4 && ev.Detail <= 7) {
				return false
			}
			timestamp = ev.Time
			x11Conn.UngrabPointer()
			imgWnd.dispose()
			switch {
			case curTarget == 0:
				return true
			case awaitingStatus:
				// The final position hasn't been answered yet, so wait for its XdndStatus before deciding
				state = dragStateAwaitFinalStatus
				deadline = time.Now().Add(x11DnDStatusTimeout)
			default:
				return dropOrStop()
			}
		case *x11.KeyPressEvent:
			if state == dragStateTracking && rawScanCodeToKeyCodeMap[uint16(ev.Detail)] == KeyEscape {
				leaveAndStop()
				return true
			}
			x11ProcessEvent(e)
		case *x11.ClientMessageEvent:
			switch ev.Type {
			case x11Conn.Atoms.DnDStatus:
				if x11.WindowID(ev.Data32[0]) != curTarget {
					return false
				}
				awaitingStatus = false
				accepted = ev.Data32[1]&1 != 0
				switch state {
				case dragStateTracking:
					if hasPending {
						hasPending = false
						sendPosition(pendingX, pendingY)
					}
				case dragStateAwaitFinalStatus:
					return dropOrStop()
				}
			case x11Conn.Atoms.DnDFinished:
				if state == dragStateAwaitFinished && x11.WindowID(ev.Data32[0]) == curTarget {
					return true
				}
			default:
				x11ProcessEvent(e)
			}
		case *x11.SelectionRequestEvent:
			// An INCR transfer answers the requestor from a filtered wait, which may swallow a pending redraw's
			// wake-up. See x11FilteredWaitDone.
			x11Conn.RespondToSelectionRequest(ev)
			x11FilteredWaitDone()
		case *x11.SelectionClearEvent:
			if ev.Selection == x11Conn.Atoms.DnDSelection {
				// Something else claimed the drag selection, so abort the drag
				if state == dragStateTracking {
					leaveAndStop()
				}
				return true
			}
		default:
			x11ProcessEvent(e)
		}
		return false
	}

	if qpr := x11Conn.QueryPointer(w.wnd.id); qpr != nil {
		imgWnd.moveTo(qpr.RootX, qpr.RootY)
		updateTarget(qpr.RootX, qpr.RootY)
	}
	for {
		var e x11.Event
		if state == dragStateTracking {
			// Wake periodically so a fresh XdndPosition can be sent even when the pointer is stationary, keeping
			// drop targets updated continuously.
			e = x11Conn.WaitEventsUntil(nil, x11DnDContinuousInterval)
			if xreflect.IsNil(e) && curTarget != 0 {
				sendPosition(lastRootX, lastRootY)
			}
		} else {
			remaining := time.Until(deadline)
			if remaining > 0 {
				e = x11Conn.WaitEventsUntil(nil, remaining)
			}
			// A nil event is just a wake-up call, so only stop if the deadline has passed
			if xreflect.IsNil(e) && !time.Now().Before(deadline) {
				// The target took too long to respond, so give up on it
				if state == dragStateAwaitFinalStatus {
					leaveAndStop()
				}
				return
			}
		}
		// Process this event plus any others that are already available before flushing UI tasks and redraws, so that
		// event processing isn't throttled to the redraw rate.
		for {
			if handleEvent(e) {
				return
			}
			if e = x11Conn.PollEvents(nil); xreflect.IsNil(e) {
				break
			}
		}
		finishProcessingEvents()
	}
}

// x11FindDnDAwareWindow returns the deepest window beneath the given root coordinates that has the XdndAware property
// set, along with the XDND protocol version it advertises. The awareness lookups are stored in the provided cache to
// avoid repeated queries while the pointer moves. The skip window (the drag image) is never descended into. Also
// returns the window of this application that is beneath the coordinates, if any.
func x11FindDnDAwareWindow(rootX, rootY int16, awareCache map[x11.WindowID]uint32, skip x11.WindowID) (target x11.WindowID, version uint32, local *Window) {
	root := x11Conn.RootWindow()
	cur := root
	for {
		v, ok := awareCache[cur]
		if !ok {
			v = 0
			format, actualType, values, _, err := x11Conn.GetProperty(cur, x11Conn.Atoms.DnDAware, x11.AtomAtom, 0, 1,
				false)
			if err == nil && format == 32 && actualType == x11.AtomAtom && len(values) >= 4 {
				v = x11.NewReader(values).Uint32()
			}
			awareCache[cur] = v
		}
		if v != 0 {
			target = cur
			version = v
		}
		if w := x11FindWindow(cur); w != nil {
			local = w
		}
		_, _, _, child, err := x11Conn.TranslateCoordinates(root, cur, rootX, rootY)
		if err != nil {
			errs.Log(err)
			break
		}
		if child == 0 || child == skip {
			break
		}
		cur = child
	}
	return target, version, local
}

func x11DnDActionForOp(op drag.Op) x11.Atom {
	switch {
	case op&drag.Copy != 0:
		return x11Conn.Atoms.DnDActionCopy
	case op&drag.Move != 0:
		return x11Conn.Atoms.DnDActionMove
	default:
		return x11.AtomNone
	}
}

func x11OpForDnDAction(action x11.Atom) drag.Op {
	switch action {
	case x11Conn.Atoms.DnDActionCopy:
		return drag.Copy
	case x11Conn.Atoms.DnDActionMove:
		return drag.Move
	default:
		return drag.None
	}
}

// x11DragImageWindow is an override-redirect window that displays the drag image and follows the pointer during a
// drag. Its input shape is empty so that it does not interfere with locating the window under the pointer.
type x11DragImageWindow struct {
	id       x11.WindowID
	colorMap x11.ColorMapID
	offsetX  int16
	offsetY  int16
}

// newX11DragImageWindow creates the drag image window, positioned so that the image's offset from the pointer matches
// the offset of originInRoot from the current mouse location. Failures are logged or ignored, since the drag itself
// still works without an image; nil may be returned and is safe to use.
func (w *Window) newX11DragImageWindow(img *Image, originInRoot geom.Point) *x11DragImageWindow {
	if img == nil {
		return nil
	}
	nrgba, err := img.ToNRGBA()
	if err != nil {
		errs.Log(err)
		return nil
	}
	scale := w.BackingScale()
	size := img.LogicalSize().MulPt(scale).Ceil()
	width := int(size.Width)
	height := int(size.Height)
	if width < 1 || height < 1 {
		return nil
	}
	if nrgba.Rect.Dx() != width || nrgba.Rect.Dy() != height {
		dstRect := image.Rect(0, 0, width, height)
		dst := image.NewNRGBA(dstRect)
		draw.CatmullRom.Scale(dst, dstRect, nrgba, nrgba.Bounds(), draw.Over, nil)
		nrgba = dst
	}
	visual := x11FindARGBVisual()
	if visual == 0 {
		slog.Warn("unable to find a 32-bit visual for the drag image")
		return nil
	}
	root := x11Conn.RootWindow()
	pm := x11Conn.CreatePixMap(x11.DrawableID(root), 32, uint16(width), uint16(height))
	if pm == 0 {
		return nil
	}
	defer x11Conn.FreePixMap(pm)
	gc := x11Conn.CreateGC(x11.DrawableID(pm), 0, nil)
	if gc == 0 {
		return nil
	}
	x11Conn.PutImage(x11.DrawableID(pm), gc, 0, 0, nrgba)
	x11Conn.FreeGC(gc)
	// Anchor the image to the drag's origin point by translating it into screen coordinates and then capturing its
	// offset from the pointer, so that the image maintains that offset as it follows the pointer.
	d := &x11DragImageWindow{}
	var x, y int16
	origin := originInRoot.MulPt(scale)
	rootX, rootY, _, _, err := x11Conn.TranslateCoordinates(w.wnd.id, root, int16(origin.X), int16(origin.Y))
	if err != nil {
		errs.Log(err)
	} else {
		x = rootX
		y = rootY
	}
	if qpr := x11Conn.QueryPointer(w.wnd.id); qpr != nil {
		if err != nil {
			x = qpr.RootX
			y = qpr.RootY
		} else {
			d.offsetX = qpr.RootX - x
			d.offsetY = qpr.RootY - y
		}
	}
	if d.colorMap = x11Conn.CreateColormap(visual, root, false); d.colorMap == 0 {
		return nil
	}
	if d.id = x11Conn.CreateWindow(root, x, y, uint16(width), uint16(height), 0, x11.WindowClassInputOutput, 32,
		visual, x11.WindowMaskBackPixMap|x11.WindowMaskBorderPixel|x11.WindowMaskOverrideRedirect|x11.WindowMaskColorMap,
		&x11.WindowCreationAttributes{
			BackgroundPixMap: pm,
			OverrideRedirect: true,
			ColorMap:         d.colorMap,
		}); d.id == 0 {
		x11Conn.FreeColormap(d.colorMap)
		return nil
	}
	if region := x11Conn.ExtXFixes.CreateRegion(); region != 0 {
		x11Conn.ExtXFixes.SetWindowShapeRegion(d.id, x11.ShapeKindInput, region)
		x11Conn.ExtXFixes.DestroyRegion(region)
	}
	x11Conn.MapWindow(d.id)
	x11Conn.Flush()
	return d
}

func (d *x11DragImageWindow) windowID() x11.WindowID {
	if d == nil {
		return 0
	}
	return d.id
}

func (d *x11DragImageWindow) moveTo(rootX, rootY int16) {
	if d == nil || d.id == 0 {
		return
	}
	x11Conn.ConfigureWindow(d.id, x11.ConfigureWindowMaskX|x11.ConfigureWindowMaskY, &x11.ConfigureWindowRequest{
		X: rootX - d.offsetX,
		Y: rootY - d.offsetY,
	})
	x11Conn.Flush()
}

func (d *x11DragImageWindow) dispose() {
	if d == nil {
		return
	}
	if d.id != 0 {
		x11Conn.UnmapWindow(d.id)
		x11Conn.DestroyWindow(d.id)
		d.id = 0
	}
	if d.colorMap != 0 {
		x11Conn.FreeColormap(d.colorMap)
		d.colorMap = 0
	}
	x11Conn.Flush()
}

// x11FindARGBVisual returns a 32-bit TrueColor visual suitable for rendering an image with alpha, or 0 if none exists.
func x11FindARGBVisual() x11.VisualID {
	const trueColorClass = 4
	for _, depth := range x11Conn.Roots[x11Conn.DefaultScreen].AllowedDepths {
		if depth.Depth != 32 {
			continue
		}
		for _, v := range depth.Visuals {
			if v.Class == trueColorClass && v.RedMask == 0x00ff0000 && v.GreenMask == 0x0000ff00 &&
				v.BlueMask == 0x000000ff {
				return v.VisualID
			}
		}
	}
	return 0
}

func (w *Window) nativeUpdateRegisteredDragTypes(types []*uti.DataType) {
	if len(types) == 0 {
		x11Conn.DeleteProperty(w.wnd.id, x11Conn.Atoms.DnDAware)
	} else {
		buf := x11.NewWriter(4)
		buf.Atom(x11.DnDVersion)
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.DnDAware, x11.AtomAtom, 32, x11.PropModeReplace, buf.Retrieve())
	}
	x11Conn.Flush()
}

func (w *Window) nativeDestroy() {
	// The window is identified on the accessibility bus by its X11 window id, so it has to be taken out of the
	// accessibility tree while that id is still valid, which is here rather than after the id has been given back to
	// the X server below. Window.destroy has already done it for every path that reaches this function today, and doing
	// it again is a map lookup that finds nothing, so this is what keeps that true of any path added later.
	w.nativeAccessibilityShutdown()
	// The server would end a grab on the window once it is unmapped, but not clear the flags that say it is held.
	if w.wnd.menuGrab {
		w.wnd.menuGrab = false
		x11Conn.UngrabPointer()
	}
	w.x11EndPressGrab()
	w.glCtx.nativeDestroy()
	if w.wnd.gc != 0 {
		x11Conn.FreeGC(w.wnd.gc)
		w.wnd.gc = 0
	}
	if w.wnd.id != 0 {
		x11Conn.UnmapWindow(w.wnd.id)
		x11Conn.DestroyWindow(w.wnd.id)
		w.wnd.id = 0
	}
	if w.wnd.colorMap != 0 {
		x11Conn.FreeColormap(w.wnd.colorMap)
		w.wnd.colorMap = 0
	}
	x11Conn.Flush()
	// Nothing handles the next queued event here: that would do so from inside the caller, which is often an event
	// handler, such as the press that closed a popup menu, with its release queued behind it. The event loop gets to
	// it once the handler returns.
}

func (w *Window) x11SetDecorated(decorated bool) {
	buf := x11.NewWriter(20)
	buf.Uint32(x11.MWMHintsDecorations)
	buf.Uint32(0)
	if decorated {
		buf.Uint32(1)
	} else {
		buf.Uint32(0)
	}
	buf.Zero(8)
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.MotifWMHints, x11Conn.Atoms.MotifWMHints, 32, x11.PropModeReplace,
		buf.Retrieve())
}

func (w *Window) x11Border() (top, left, bottom, right uint32) {
	if w.undecorated {
		return 0, 0, 0, 0
	}
	if !w.wnd.borderValid {
		// The widths come from a filtered wait on the window manager's reply, which may swallow a pending redraw's
		// wake-up. See x11FilteredWaitDone.
		t, l, b, r, ok := x11Conn.GetWindowBorderWidths(w.wnd.id)
		x11FilteredWaitDone()
		if ok {
			w.wnd.borderTop, w.wnd.borderLeft, w.wnd.borderBottom, w.wnd.borderRight = t, l, b, r
			w.wnd.borderValid = true
		}
	}
	return w.wnd.borderTop, w.wnd.borderLeft, w.wnd.borderBottom, w.wnd.borderRight
}

// x11RefreshFrameExtents updates the cached window border widths from the current _NET_FRAME_EXTENTS property. Called
// when the window manager reports new frame extents (e.g. once decorations are applied after mapping, or when they
// change such as on maximize).
func (w *Window) x11RefreshFrameExtents() {
	format, actualType, value, _, err := x11Conn.GetProperty(w.wnd.id, x11Conn.Atoms.NetFrameExtents, x11.AtomCardinal,
		0, 32, false)
	if err != nil {
		errs.Log(err)
		return
	}
	if top, left, bottom, right, ok := x11.ParseFrameExtents(format, actualType, value); ok {
		w.wnd.borderTop, w.wnd.borderLeft, w.wnd.borderBottom, w.wnd.borderRight = top, left, bottom, right
		w.wnd.borderValid = true
	}
}

func x11ProcessEvent(e x11.Event) {
	if xreflect.IsNil(e) {
		return
	}
	// The XSETTINGS manager is not one of our windows, so handle its property changes (used for dark-mode tracking on
	// desktops without the XDG portal) before the per-window dispatch below.
	if pne, ok := e.(*x11.PropertyNotifyEvent); ok && pne.State == x11.PropertyNewValue {
		if w := x11Conn.XSettingsManagerWindow(); w != 0 && pne.Window == w {
			linuxXSettingsChanged()
			return
		}
	}
	// Likewise, the MANAGER ClientMessage announcing a new XSETTINGS manager (e.g. after the settings daemon restarts)
	// arrives on the root window, so it too must be handled before the per-window dispatch.
	if cme, ok := e.(*x11.ClientMessageEvent); ok {
		if handled, changed := x11Conn.XSettingsHandleManagerMessage(cme); handled {
			if changed && linuxRecomputeDarkMode() {
				ThemeChanged()
			}
			return
		}
	}
	switch ev := e.(type) {
	case *x11.ErrorEvent:
		errs.Log(ev.Error)
	case *x11.ReparentNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			w.wnd.parent = ev.Parent
		}
	case *x11.KeyPressEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			mods := x11TranslateModifierState(ev.State)
			w.keyPressed(rawScanCodeToKeyCodeMap[uint16(ev.Detail)], mods)
			if mods&(mod.Control|mod.Option|mod.Command) == 0 {
				keySym := x11ScanCodeToKeySym(uint16(ev.Detail), mods, ev.State&x11Mod5Mask != 0)
				if ch := x11KeySymToUnicode(keySym); ch != utf8.RuneError {
					w.runeTyped(ch)
				}
			}
		}
	case *x11.KeyReleaseEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.keyReleased(rawScanCodeToKeyCodeMap[uint16(ev.Detail)], x11TranslateModifierState(ev.State))
		}
	case *x11.MappingNotifyEvent:
		if ev.Request != x11.MappingPointer {
			nativeFillKeyCodes()
		}
	case *x11.ButtonPressEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			if intercept, closeStack := x11MenuGrabInterceptsPress(w, ev, x11WindowUnder); intercept {
				if owner := x11MenuGrabOwner(); closeStack && owner != nil {
					owner.root.closeMenuStackStoppingAt(nil)
				}
				return
			}
			mods := x11TranslateModifierState(ev.State)
			where := w.x11ConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY)))
			switch ev.Detail {
			case 1:
				w.mouseDown(where, ButtonLeft, mods)
			case 2:
				w.mouseDown(where, ButtonMiddle, mods)
			case 3:
				w.mouseDown(where, ButtonRight, mods)
			case 4:
				w.mouseWheel(where, geom.NewPoint(0, 1), mods)
			case 5:
				w.mouseWheel(where, geom.NewPoint(0, -1), mods)
			case 6:
				w.mouseWheel(where, geom.NewPoint(1, 0), mods)
			case 7:
				w.mouseWheel(where, geom.NewPoint(-1, 0), mods)
			default:
				w.mouseDown(where, int(ev.Detail-5), mods)
			}
		}
	case *x11.ButtonReleaseEvent:
		button := x11ButtonFromDetail(ev.Detail)
		if target, where := x11RoutePointerEvent(x11FindWindow(ev.Event), &ev.InputEvent, button); target != nil &&
			button >= 0 {
			target.mouseUp(where, button, x11TranslateModifierState(ev.State))
		}
		x11EndPressGrabs(ev)
	case *x11.EnterNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.x11PointerCrossed(true, ev.Mode)
			if !x11IgnoreEnter(w, ev) {
				w.nativeUpdateCursorImage()
				w.mouseEnter(w.x11ConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY))),
					x11TranslateModifierState(ev.State))
			}
		}
	case *x11.LeaveNotifyEvent:
		// Every leave is kept, NotifyGrab ones included: one may come from another client's grab, such as a window
		// manager's move, which really does take the pointer away, and a spurious one only costs a hover state that the
		// next motion restores. The crossings a grab's release reports (NotifyUngrab) say where the pointer really is.
		if w := x11FindWindow(ev.Event); w != nil {
			w.x11PointerCrossed(false, ev.Mode)
			w.nativeUpdateCursorImage()
			w.mouseExit()
		}
	case *x11.MotionNotifyEvent:
		if target, where := x11RoutePointerEvent(x11FindWindow(ev.Event), &ev.InputEvent, -1); target != nil {
			target.mouseMovedOrDragged(where, x11TranslateModifierState(ev.State))
		}
	case *x11.ConfigureNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			w.wnd.awaitingConfigure = false
			if float32(ev.Width) != w.lastWidth || float32(ev.Height) != w.lastHeight {
				w.lastWidth = float32(ev.Width)
				w.lastHeight = float32(ev.Height)
				w.resized()
				// X11 does not guarantee an Expose after a resize (and the resize may arrive after the initial Expose
				// has already been handled), so explicitly mark the window for redraw to ensure its content is painted
				// at the new size.
				w.MarkForRedraw()
			}
			x := ev.X
			y := ev.Y
			// The position a ConfigureNotify the X server generated carries is relative to the window's parent, which
			// under a reparenting window manager is the frame it put around the window rather than the root, so it has
			// to be translated. A synthetic one — which is what the window manager sends when it has moved a window
			// without resizing it, since the window's position within its frame did not change — already carries the
			// position relative to the root and must not be (ICCCM 4.2.3), or the frame's origin would be added to it a
			// second time. Windows are created with a zero border width, so either way this is the content origin
			// itself, in the X server's pixels.
			if !x11.Synthetic(ev) && w.wnd.parent != x11Conn.RootWindow() {
				var err error
				if x, y, _, _, err = x11Conn.TranslateCoordinates(w.wnd.parent, x11Conn.RootWindow(), x, y); err != nil {
					errs.Log(err)
					return
				}
			}
			if float32(x) != w.wnd.lastX || float32(y) != w.wnd.lastY {
				w.wnd.lastX = float32(x)
				w.wnd.lastY = float32(y)
				w.moved()
			}
			// A window that has moved or been resized reports its contents somewhere else on the screen. A resize is
			// marked for redraw above, so the next snapshot would carry the new geometry with it, but a move goes
			// through w.moved() and never marks anything, so nothing else would tell the adapter that every node in the
			// window is now somewhere else. The origin worked out above is handed over rather than asked for again,
			// which is what keeps an assistive technology from costing two X round trips per event of every resize and
			// every drag of a title bar.
			w.x11RefreshAccessibilityGeometry(geom.NewPoint(float32(x), float32(y)))
		}
	case *x11.ClientMessageEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			switch ev.Type {
			case x11.AtomNone:
				return
			case x11Conn.Atoms.WMProtocols:
				switch x11.Atom(ev.Data32[0]) {
				case x11.AtomNone:
					return
				case x11Conn.Atoms.WMDeleteWindow:
					w.requestClose()
				case x11Conn.Atoms.NetWMPing:
					x11Conn.RespondToPing(ev)
				default:
					slog.Info(fmt.Sprintf("ClientMessageEvent with unhandled protocol: %d", ev.Data32[0]))
				}
			case x11Conn.Atoms.DnDEnter:
				w.wnd.dndSource = x11.WindowID(ev.Data32[0])
				w.wnd.dndVersion = ev.Data32[1] >> 24
				di := &x11DragInfo{}
				if ev.Data32[1]&1 == 1 {
					_, _, values, _, err := x11Conn.GetProperty(w.wnd.dndSource, x11Conn.Atoms.DnDTypeList,
						x11.AtomAtom, 0, math.MaxUint32, false)
					if err != nil {
						errs.Log(err)
						return
					}
					r := x11.NewReader(values)
					for r.Remaining() > 3 {
						di.targets = append(di.targets, r.Atom())
					}
				} else {
					for i := 2; i <= 4; i++ {
						if a := x11.Atom(ev.Data32[i]); a != x11.AtomNone {
							di.targets = append(di.targets, a)
						}
					}
				}
				for _, target := range di.targets {
					di.dataTypes = append(di.dataTypes, x11Conn.DataTypeForTarget(target))
				}
				// Use the source's full set of allowed actions, if published. Otherwise, the suggested actions from
				// the position messages will be accumulated as they arrive.
				format, actualType, values, _, err := x11Conn.GetProperty(w.wnd.dndSource,
					x11Conn.Atoms.DnDActionList, x11.AtomAtom, 0, math.MaxUint32, false)
				if err == nil && format == 32 && actualType == x11.AtomAtom {
					r := x11.NewReader(values)
					for r.Remaining() > 3 {
						di.opMask |= x11OpForDnDAction(r.Atom())
					}
					di.hasActionList = di.opMask != drag.None
				}
				w.wnd.dndInfo = di
			case x11Conn.Atoms.DnDPosition:
				src := x11.WindowID(ev.Data32[0])
				di := w.wnd.dndInfo
				if di == nil {
					x11Conn.SendDnDStatus(src, w.wnd.id, false, x11.AtomNone)
					x11Conn.Flush()
					return
				}
				if !di.hasActionList {
					di.opMask |= x11OpForDnDAction(x11.Atom(ev.Data32[4]))
				}
				di.timestamp = ev.Data32[3]
				x := int16((ev.Data32[2] >> 16) & 0xffff)
				y := int16(ev.Data32[2] & 0xffff)
				var err error
				if x, y, _, _, err = x11Conn.TranslateCoordinates(x11Conn.RootWindow(), w.wnd.id, x, y); err != nil {
					errs.Log(err)
					return
				}
				di.lastWhere = w.x11ConvertRawMouse(geom.NewPoint(float32(x), float32(y)))
				op := w.dragUpdate(di, di.lastWhere, w.CurrentKeyModifiers())
				x11Conn.SendDnDStatus(src, w.wnd.id, op != drag.None, x11DnDActionForOp(op))
				x11Conn.Flush()
			case x11Conn.Atoms.DnDLeave:
				w.dragExit()
				w.wnd.dndInfo = nil
			case x11Conn.Atoms.DnDDrop:
				src := x11.WindowID(ev.Data32[0])
				handled := false
				if di := w.wnd.dndInfo; di != nil {
					di.timestamp = ev.Data32[2]
					handled = w.drop(di, di.lastWhere, w.CurrentKeyModifiers())
				}
				if w.wnd.dndVersion > 1 {
					x11Conn.SendDnDFinished(src, w.wnd.id, handled, x11DnDActionForOp(w.lastDragOp))
					x11Conn.Flush()
				}
				w.wnd.dndInfo = nil
			case x11Conn.Atoms.DnDStatus, x11Conn.Atoms.DnDFinished:
				// These are handled by the drag loop while a drag is in progress. One may still arrive after the drag
				// loop has given up waiting for it, so just ignore it.
			default:
				slog.Info(fmt.Sprintf("ClientMessageEvent with unhandled type: %d", ev.Type))
				return
			}
		}
	case *x11.SelectionRequestEvent:
		// An INCR transfer answers the requestor from a filtered wait, which may swallow a pending redraw's wake-up.
		// See x11FilteredWaitDone.
		x11Conn.RespondToSelectionRequest(ev)
		x11FilteredWaitDone()
	case *x11.FocusInEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			if ev.Mode == x11.NotifyGrab || ev.Mode == x11.NotifyUngrab {
				return
			}
			w.gainedFocus()
		}
	case *x11.FocusOutEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			if ev.Mode == x11.NotifyGrab || ev.Mode == x11.NotifyUngrab {
				return
			}
			w.lostFocus()
		}
	case *x11.ExposeEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			w.draw()
		}
	case *x11.PropertyNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			if ev.State != x11.PropertyNewValue {
				return
			}
			switch ev.Atom {
			case x11Conn.Atoms.NetFrameExtents:
				w.x11RefreshFrameExtents()
			case x11Conn.Atoms.WMState:
				format, actualType, values, _, err := x11Conn.GetProperty(w.wnd.id, x11Conn.Atoms.WMState,
					x11Conn.Atoms.WMState, 0, math.MaxUint32, false)
				if err != nil {
					errs.Log(err)
					return
				}
				if format != 32 || actualType != x11Conn.Atoms.WMState || len(values) < 8 {
					errs.Log(errs.New("unexpected response"))
					return
				}
				r := x11.NewReader(values)
				w.x11SetMinimized(r.Uint32() == x11.StateIconic)
			case x11Conn.Atoms.NetWMState:
				format, actualType, values, _, err := x11Conn.GetProperty(w.wnd.id, x11Conn.Atoms.NetWMState,
					x11.AtomAtom, 0, math.MaxUint32, false)
				if err != nil {
					errs.Log(err)
					return
				}
				if format != 32 || actualType != x11.AtomAtom {
					errs.Log(errs.New("unexpected response"))
					return
				}
				maximized := false
				r := x11.NewReader(values)
				for range len(values) / 4 {
					a := r.Atom()
					if a == x11Conn.Atoms.NetWMStateMaximizedHorz || a == x11Conn.Atoms.NetWMStateMaximizedVert {
						maximized = true
						break
					}
				}
				w.x11SetMaximized(maximized)
			}
		}
	}
}

// x11MenuGrabInterceptsPress decides what becomes of a press X delivered to w while one of our windows holds the
// pointer grab for its open menus (see nativeMenuStackChanged). intercept is true if the press is kept from being
// dispatched, and closeStack is true if it also closes the menus. With owner events on, a press outside every window of
// ours comes to the owner, so one there is outside when it lies outside the owner's content, or within it where under,
// given the root coordinates, finds another client's window on top. Such a press is intercepted, and closes the menus
// unless it is of a wheel (buttons 4 through 7). A press on another of our top-level windows is intercepted and closes
// the menus too, since the grab kept the window manager from activating that window, but a wheel there scrolls it. A
// popup menu window's own presses are left to it.
func x11MenuGrabInterceptsPress(w *Window, ev *x11.ButtonPressEvent,
	under func(rootX, rootY int16) *Window,
) (intercept, closeStack bool) {
	owner := x11MenuGrabOwner()
	if owner == nil || w.isPopup() {
		return false, false
	}
	closeStack = ev.Detail < 4 || ev.Detail > 7
	if w != owner {
		return closeStack, closeStack
	}
	if x11InContent(w, ev.EventX, ev.EventY) && under(ev.RootX, ev.RootY) == w {
		return false, false
	}
	return true, closeStack
}

// x11WindowUnder returns our window that is topmost at the given root coordinates, or nil if another client's window
// (a window manager's frame included) is. Like x11FindDnDAwareWindow, it descends from the root to the deepest window
// containing the point.
func x11WindowUnder(rootX, rootY int16) *Window {
	root := x11Conn.RootWindow()
	var found *Window
	for cur := root; ; {
		_, _, _, child, err := x11Conn.TranslateCoordinates(root, cur, rootX, rootY)
		if err != nil {
			errs.Log(err)
			return nil
		}
		if child == 0 {
			return found
		}
		cur = child
		found = x11FindWindow(cur)
	}
}

// x11InContent returns true if a position relative to w, in the X server's pixels, lies within w's content.
func x11InContent(w *Window, x, y int16) bool {
	return x >= 0 && y >= 0 && float32(x) < w.lastWidth && float32(y) < w.lastHeight
}

// x11PointerCrossed records that the pointer entered or left the window, from an EnterNotify or LeaveNotify with the
// given mode. One a grab reports as it takes effect (NotifyGrab) enters the grab window wherever the pointer really
// is, so it says nothing about where the pointer is and is not recorded; a leave of that mode is, since another
// client's grab really does take the pointer away, and the crossing the grab's release reports (NotifyUngrab) puts it
// back.
func (w *Window) x11PointerCrossed(entered bool, mode byte) {
	if !entered || mode != x11.NotifyGrab {
		w.wnd.pointerInside = entered
	}
}

// x11PointerTarget returns the window a motion or button release X delivered to w belongs to, or nil if it is to be
// dropped. inside is true if the event lies within w's content, and button is the toolkit's button a release is for, or
// -1 for a motion (see x11ButtonFromDetail). While a menu grab is held, a press forms no implicit grab, so X reports
// such an event to whichever of our windows is under the pointer, or to the owner when the pointer is over another
// client's window, rather than to the window holding the press. A release therefore goes to the window recording its
// button as down, and a motion to the window holding a press, preferring a popup menu window, as the implicit grab
// would have sent them; one with no press in progress that reaches the owner from outside it, or from within its bounds
// while the pointer has left it for another client's window on top (see x11PointerCrossed), is over another client's
// window, and is dropped. w is nil when X reported the event to a window that was gone by the time it was read, such as
// a popup menu window closed by events queued ahead of the event: while a menu grab is held, the event still goes where
// that window would have sent it, since no implicit grab delivers a release to the window holding the press.
func x11PointerTarget(w *Window, inside bool, button int) *Window {
	owner := x11MenuGrabOwner()
	if owner == nil {
		return w
	}
	if holder := x11ButtonHolder(button); holder != nil {
		return holder
	}
	if w != nil && w.inMouseDown {
		return w
	}
	if holder := x11PressHolder(); holder != nil {
		return holder
	}
	if w == nil || (w == owner && (!inside || !w.wnd.pointerInside)) {
		return nil
	}
	return w
}

// x11ButtonHolder returns a valid window of ours recording button as down, preferring a popup menu window, or nil if
// there is none or button is -1.
func x11ButtonHolder(button int) *Window {
	if button < 0 {
		return nil
	}
	var found *Window
	for _, w := range windowList {
		if w.IsValid() && w.pressedButtons[button] {
			if w.isPopup() {
				return w
			}
			if found == nil {
				found = w
			}
		}
	}
	return found
}

// x11RoutePointerEvent returns the window a motion or button release X delivered to w, which is nil for a window that
// is gone, goes to (see x11PointerTarget) and the event's position within it, or nil if the event is dropped. button is
// the toolkit's button a release is for, or -1 for a motion.
func x11RoutePointerEvent(w *Window, ev *x11.InputEvent, button int) (target *Window, where geom.Point) {
	inside := w != nil && x11InContent(w, ev.EventX, ev.EventY)
	if target = x11PointerTarget(w, inside, button); target == nil {
		return nil, geom.Point{}
	}
	x, y := x11PointerEventPosition(w, target, ev)
	where = target.x11ConvertRawMouse(geom.NewPoint(float32(x), float32(y)))
	return target, where
}

// x11PointerEventPosition returns the position of a pointer event X delivered to w, in the X server's pixels, within
// target, which is w itself or another window of ours; w is nil for a window that is gone. The other window's origin on
// the root is known from its last ConfigureNotify, so no round trip to the server is needed for an event rerouted on
// every motion.
func x11PointerEventPosition(w, target *Window, ev *x11.InputEvent) (x, y int16) {
	if target == w {
		return ev.EventX, ev.EventY
	}
	return ev.RootX - int16(target.wnd.lastX), ev.RootY - int16(target.wnd.lastY)
}

// x11EndPressGrabs releases each grab held in place of a press's implicit grab (see nativeMenuStackChanged) once its
// window has no press in progress, or once the release ev leaves no button down. In the latter case, a press the
// window still records is ended too: its release went to a window that was gone by the time it was read, such as a
// popup menu window closed by the press itself.
func x11EndPressGrabs(ev *x11.ButtonReleaseEvent) {
	for _, w := range slices.Clone(windowList) {
		if !w.wnd.pressGrab || (w.inMouseDown && (x11OtherButtonsDown(ev) || w.x11OtherButtonsHeld(ev.Detail))) {
			continue
		}
		w.x11EndPressGrab()
		if w.inMouseDown {
			w.synthesizeMouseUp()
		}
	}
}

// x11OtherButtonsDown returns true if the state a ButtonRelease carries, which is from before the release, has one of
// the first five buttons down other than the one released. The core protocol reports no state for higher buttons.
func x11OtherButtonsDown(ev *x11.ButtonReleaseEvent) bool {
	held := ev.State & 0x1f00 // Button1Mask through Button5Mask
	if ev.Detail >= 1 && ev.Detail <= 5 {
		held &^= 0x80 << ev.Detail // The released button's own mask
	}
	return held != 0
}

// x11OtherButtonsHeld returns true if the window records a button down other than the one a release with the given
// detail is for. The core protocol carries no state for buttons above the fifth, so a held back or forward button is
// known only from the window's own record.
func (w *Window) x11OtherButtonsHeld(detail byte) bool {
	released := x11ButtonFromDetail(detail)
	for button := range w.pressedButtons {
		if button != released {
			return true
		}
	}
	return false
}

// x11ButtonFromDetail returns the toolkit's button for the detail of a ButtonPress or ButtonRelease, or -1 for the
// wheel, which x11ProcessEvent reports as scrolling rather than as a button.
func x11ButtonFromDetail(detail byte) int {
	switch detail {
	case 1:
		return ButtonLeft
	case 2:
		return ButtonMiddle
	case 3:
		return ButtonRight
	case 4, 5, 6, 7:
		return -1
	default:
		return int(detail - 5)
	}
}

// x11IgnoreEnter returns true if an EnterNotify X delivered to w is to be ignored. One a grab reports as it takes
// effect (NotifyGrab) enters the grab window wherever the pointer really is, which for a menu grab is often outside
// the owner. While a menu grab is held, one entering a window other than the one holding a press is ignored too, as the
// press's implicit grab would have kept it from being reported; see x11PointerTarget.
func x11IgnoreEnter(w *Window, ev *x11.EnterNotifyEvent) bool {
	return ev.Mode == x11.NotifyGrab || x11PointerTarget(w, true, -1) != w
}

func x11TranslateModifierState(state uint16) mod.Modifiers {
	var m mod.Modifiers
	if state&0x0001 != 0 {
		m |= mod.Shift
	}
	if state&0x0002 != 0 {
		m |= mod.CapsLock
	}
	if state&0x0004 != 0 {
		m |= mod.Control
	}
	if state&0x0008 != 0 {
		m |= mod.Option
	}
	if state&0x0010 != 0 { // Mod2 is NumLock on essentially all X11 configurations
		m |= mod.NumLock
	}
	if state&0x0040 != 0 { // Mod4 is the Super/Windows key on essentially all X11 configurations
		m |= mod.Command
	}
	return m
}

func x11ParseURIList(uriList []byte) []*url.URL {
	var urls []*url.URL
	for line := range strings.SplitSeq(string(bytes.ReplaceAll(uriList, []byte{'\r', '\n'}, []byte{'\n'})), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line[0] == '#' {
			continue
		}
		u, err := url.Parse(line)
		if err != nil {
			errs.Log(err)
			continue
		}
		urls = append(urls, u)
	}
	return urls
}

var _ drag.Info = &x11DragInfo{}

// x11DragInfo implements drag.Info for an incoming XDND drag. The drag data is fetched lazily by converting the
// XdndSelection and is cached, since the data for a given drag cannot change once the drag has begun.
type x11DragInfo struct {
	cache         map[x11.Atom][]byte
	targets       []x11.Atom
	dataTypes     []string
	lastWhere     geom.Point
	timestamp     uint32
	opMask        drag.Op
	hasActionList bool
}

func (d *x11DragInfo) SourceDragOpMask() drag.Op {
	if d.opMask == drag.None {
		return drag.Copy
	}
	return d.opMask
}

func (d *x11DragInfo) DataTypes() []string {
	result := make([]string, 0, len(d.dataTypes))
	for _, dataType := range d.dataTypes {
		if dataType != "" && !slices.Contains(result, dataType) {
			result = append(result, dataType)
		}
	}
	return result
}

// targetFor returns the offered target to use for the given data type, or AtomNone if the data type is not present in
// the drag.
func (d *x11DragInfo) targetFor(dataType string) x11.Atom {
	for _, target := range x11Conn.TargetsForDataType(dataType) {
		if slices.Contains(d.targets, target) {
			return target
		}
	}
	for i, dt := range d.dataTypes {
		if dt == dataType {
			return d.targets[i]
		}
	}
	return x11.AtomNone
}

func (d *x11DragInfo) HasString() bool {
	return d.HasDataType(uti.UTF8PlainText.UTI)
}

func (d *x11DragInfo) HasFilePaths() bool {
	return slices.Contains(d.targets, x11Conn.Atoms.TextURIList)
}

func (d *x11DragInfo) HasURLs() bool {
	return slices.Contains(d.targets, x11Conn.Atoms.TextURIList) || d.HasDataType(uti.URL.UTI)
}

func (d *x11DragInfo) HasDataType(dataType string) bool {
	return d.targetFor(dataType) != x11.AtomNone
}

func (d *x11DragInfo) Text() string {
	return string(d.Data(uti.UTF8PlainText.UTI))
}

func (d *x11DragInfo) FilePaths() []string {
	var paths []string
	for _, u := range x11ParseURIList(d.fetch(x11Conn.Atoms.TextURIList)) {
		if u.Scheme == "file" || u.Scheme == "" {
			paths = append(paths, u.Path)
		}
	}
	return paths
}

func (d *x11DragInfo) URLs() []*url.URL {
	if urls := x11ParseURIList(d.fetch(x11Conn.Atoms.TextURIList)); len(urls) != 0 {
		return urls
	}
	return x11ParseURIList(d.Data(uti.URL.UTI))
}

func (d *x11DragInfo) Data(dataType string) []byte {
	return d.fetch(d.targetFor(dataType))
}

func (d *x11DragInfo) fetch(target x11.Atom) []byte {
	if target == x11.AtomNone {
		return nil
	}
	if data, ok := d.cache[target]; ok {
		return data
	}
	// A transfer from another application runs as a filtered wait, which may swallow a pending redraw's wake-up. See
	// x11FilteredWaitDone.
	data, ok := x11Conn.DnDSelectionBytes(target, d.timestamp)
	x11FilteredWaitDone()
	if !ok {
		data = nil
	}
	if d.cache == nil {
		d.cache = make(map[x11.Atom][]byte)
	}
	d.cache[target] = data
	return data
}
