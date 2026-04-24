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
	"fmt"
	"image"
	"log/slog"
	"os"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/internal/x11"
)

type apiWindow struct {
	id       x11.WindowID
	parent   x11.WindowID
	colorMap x11.ColorMapID
	lastX    float32
	lastY    float32
}

func x11FindWindow(id x11.WindowID) *Window {
	for _, w := range windowList {
		if w.wnd.id == id {
			return w
		}
	}
	return nil
}

func (w *Window) apiInit() error {
	if err := w.glCtx.x11PrepareWindow(w); err != nil {
		return err
	}
	w.wnd.parent = x11Conn.RootWindow()
	visual := x11Conn.DefaultVisual()
	depth := x11Conn.DefaultDepth()
	if w.glCtx.visual != 0 {
		visual = w.glCtx.visual
		depth = w.glCtx.depth
	}
	if w.wnd.colorMap = x11Conn.CreateColormap(visual, w.wnd.parent, false); w.wnd.colorMap == 0 {
		return errs.New("failed to create X11 color map for window")
	}
	if w.wnd.id = x11Conn.CreateWindow(w.wnd.parent, 0, 0, 1, 1, 0, x11.WindowClassInputOutput,
		depth, visual, x11.WindowMaskBorderPixel|x11.WindowMaskColorMap|x11.WindowMaskEventMask,
		&x11.WindowCreationAttributes{
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
		}); w.wnd.id == 0 {
		x11Conn.FreeColormap(w.wnd.colorMap)
		w.wnd.colorMap = 0
		return errs.New("failed to create X11 window")
	}
	if w.undecorated {
		w.x11SetDecorated(false)
	}
	if w.floating {
		buf := x11.NewWriter(4)
		buf.Atom(x11Conn.Atoms.NetStateAbove)
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetState, x11.AtomAtom, 32, x11.PropModeReplace,
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

	buf = x11.NewWriter(4)
	buf.Atom(5)
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.DnDAware, x11.AtomAtom, 32, x11.PropModeReplace, buf.Retrieve())

	w.apiSetTitle(w.title)
	x11Conn.Flush()
	return nil
}

func (w *Window) apiSetTitle(title string) {
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMName, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(title))
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMIconName, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(title))
	x11Conn.Flush()
}

func (w *Window) apiSetTitleIcons(_images []*image.NRGBA) {
	// TODO: Need implementation
	x11Conn.Flush()
}

func (w *Window) apiDisplay() *Display {
	return BestDisplayForRect(w.apiFrameRect())
}

func (w *Window) apiFrameRect() geom.Rect {
	return w.apiFrameRectForContentRect(w.apiContentRect())
}

func (w *Window) apiFrameRectForContentRect(contentRect geom.Rect) geom.Rect {
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

func (w *Window) apiEnsureOnDisplay() {
	frameRect := w.apiFrameRect()
	revisedRect := w.apiDisplay().FitRectOnto(frameRect)
	if frameRect != revisedRect {
		w.SetFrameRect(revisedRect)
	}
}

func (w *Window) apiContentRect() geom.Rect {
	info, err := x11Conn.GetGeometry(x11.DrawableID(w.wnd.id))
	if err != nil {
		errs.Log(err)
		return geom.Rect{}
	}
	root := x11Conn.RootWindow()
	if info.Root != root {
		var x, y int16
		if x, y, _, _, err = x11Conn.TranslateCoordinates(w.wnd.id, root, info.X, info.Y); err != nil {
			errs.Log(err)
		} else {
			info.X = x
			info.Y = y
		}
	}
	r := geom.NewRect(float32(info.X), float32(info.Y), float32(info.Width), float32(info.Height))
	scale := w.BackingScale()
	r.X /= scale.X
	r.Y /= scale.Y
	r.Width /= scale.X
	r.Height /= scale.Y
	return r
}

func (w *Window) apiContentRectForFrameRect(frameRect geom.Rect) geom.Rect {
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

func (w *Window) apiSetContentRect(rect geom.Rect) {
	scale := w.BackingScale()
	rect.X *= scale.X
	rect.Y *= scale.Y
	rect.Width *= scale.X
	rect.Height *= scale.Y
	var req x11.ConfigureWindowRequest
	var mask x11.ConfigureWindowValueMask
	if w.lastContentRect.X != rect.X {
		req.X = int16(rect.X)
		mask |= x11.ConfigureWindowMaskX
	}
	if w.lastContentRect.Y != rect.Y {
		req.Y = int16(rect.Y)
		mask |= x11.ConfigureWindowMaskY
	}
	if w.lastContentRect.Width != rect.Width {
		req.Width = uint16(rect.Width)
		mask |= x11.ConfigureWindowMaskWidth
	}
	if w.lastContentRect.Height != rect.Height {
		req.Height = uint16(rect.Height)
		mask |= x11.ConfigureWindowMaskHeight
	}
	if mask == 0 {
		return
	}
	if w.notResizable {
		var sizeHints x11.WindowSizeHints
		sizeHints.Flags |= x11.WSHMPMinSize | x11.WSHMPMaxSize
		sizeHints.MinWidth = uint32(rect.Width)
		sizeHints.MinHeight = uint32(rect.Height)
		sizeHints.MaxWidth = uint32(rect.Width)
		sizeHints.MaxHeight = uint32(rect.Height)
		sizeHints.Flags |= x11.WSHMPPosition | x11.WSHMPWinGravity
		sizeHints.WinGravity = x11.StaticGravity
		x11Conn.SetSizeHints(w.wnd.id, x11.AtomWMNormalHints, &sizeHints)
	}
	x11Conn.ConfigureWindow(w.wnd.id, mask, &req)
	x11Conn.Flush()
}

func (w *Window) apiConvertRawMouse(where geom.Point) geom.Point {
	return where.DivPt(w.BackingScale())
}

func (w *Window) apiCurrentKeyModifiers() Modifiers {
	// TODO: Need to fix implementation
	return w.LastKeyModifiers()
}

func (w *Window) apiUpdateCursorImage() {
	// TODO: Need implementation
	switch {
	case w.cursorHidden:
		// TODO: Need to test this and cursor showing once text input works
		x11Conn.ExtXFixes.HideCursor(w.wnd.id)
	case w.cursor != nil:
		x11Conn.ChangeWindowAttributes(w.wnd.id, x11.WindowMaskCursor, &x11.WindowCreationAttributes{
			Cursor: w.cursor.cursor.cursor,
		})
	default:
		x11Conn.ChangeWindowAttributes(w.wnd.id, x11.WindowMaskCursor, &x11.WindowCreationAttributes{})
	}
}

func (w *Window) apiCursorInContentArea() bool {
	qpr := x11Conn.QueryPointer(w.wnd.id)
	if qpr == nil {
		return false
	}
	return w.apiConvertRawMouse(geom.NewPoint(float32(qpr.RootX), float32(qpr.RootY))).In(w.apiContentRect())
}

func (w *Window) apiCursorPosition() geom.Point {
	qpr := x11Conn.QueryPointer(w.wnd.id)
	if qpr == nil {
		return geom.Point{}
	}
	return w.apiConvertRawMouse(geom.NewPoint(float32(qpr.WinX), float32(qpr.WinY)))
}

func (w *Window) apiBackingScale() geom.Point {
	scale, err := x11Conn.ContentScale()
	if err != nil {
		errs.Log(err)
		return geom.NewPoint(1, 1)
	}
	return geom.NewPoint(scale, scale)
}

func (w *Window) apiMinimize() {
	// TODO: Need implementation
}

func (w *Window) apiMaximize() {
	// TODO: Need implementation
}

func (w *Window) apiAcquireFocus() {
	x11Conn.FocusWindow(w.wnd.id)
	x11Conn.Flush()
}

func (w *Window) apiVisible() bool {
	return x11Conn.IsWindowVisible(w.wnd.id)
}

func (w *Window) apiShow() {
	if w.apiVisible() {
		return
	}
	x11Conn.MapWindow(w.wnd.id)
	x11Conn.WaitEvents(func(e x11.Event) bool {
		ev, ok := e.(*x11.VisibilityNotifyEvent)
		return ok && ev.Window == w.wnd.id
	})
}

func (w *Window) apiHide() {
	x11Conn.UnmapWindow(w.wnd.id)
	x11Conn.Flush()
}

func (w *Window) apiDestroy() {
	w.glCtx.apiDestroy()
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
	return x11Conn.GetWindowBorderWidths(w.wnd.id)
}

func x11ProcessEvent(e x11.Event) {
	if xreflect.IsNil(e) {
		return
	}
	switch ev := e.(type) {
	case *x11.ErrorEvent:
		errs.Log(ev.Error)
	case *x11.ReparentNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			w.wnd.parent = ev.Parent
		}
	case *x11.KeyPressEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("KeyPressEvent", "event", ev)
			// TODO: Implement
		}
	case *x11.KeyReleaseEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("KeyReleaseEvent", "event", ev)
			// TODO: Implement
		}
	case *x11.ButtonPressEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			mods := x11TranslateModifierState(ev.State)
			where := w.apiConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY)))
			switch ev.Detail {
			case 1:
				w.mouseDown(where, ButtonLeft, mods)
			case 2:
				w.mouseDown(where, ButtonRight, mods)
			case 3:
				w.mouseDown(where, ButtonMiddle, mods)
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
		if w := x11FindWindow(ev.Event); w != nil {
			mods := x11TranslateModifierState(ev.State)
			where := w.apiConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY)))
			switch ev.Detail {
			case 1:
				w.mouseUp(where, ButtonLeft, mods)
			case 2:
				w.mouseUp(where, ButtonRight, mods)
			case 3:
				w.mouseUp(where, ButtonMiddle, mods)
			case 4, 5, 6, 7:
			default:
				w.mouseUp(where, int(ev.Detail-5), mods)
			}
		}
	case *x11.EnterNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.apiUpdateCursorImage()
			w.mouseEnter(w.apiConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY))),
				x11TranslateModifierState(ev.State))
		}
	case *x11.LeaveNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.apiUpdateCursorImage()
			w.mouseExit()
		}
	case *x11.MotionNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.mouseMovedOrDragged(w.apiConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY))),
				x11TranslateModifierState(ev.State))
		}
	case *x11.ConfigureNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			if float32(ev.Width) != w.lastWidth || float32(ev.Height) != w.lastHeight {
				w.lastWidth = float32(ev.Width)
				w.lastHeight = float32(ev.Height)
				w.resized()
			}
			x := ev.X
			y := ev.Y
			if w.wnd.parent != x11Conn.RootWindow() {
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
					x11Conn.RespondToPing()
				default:
					slog.Info(fmt.Sprintf("ClientMessageEvent with unhandled protocol: %d", ev.Data32[0]))
				}
			case x11Conn.Atoms.DnDEnter:
			// TODO: Implement
			case x11Conn.Atoms.DnDDrop:
			// TODO: Implement
			case x11Conn.Atoms.DnDPosition:
			// TODO: Implement
			default:
				slog.Info(fmt.Sprintf("ClientMessageEvent with unhandled type: %d", ev.Type))
				return
			}
		}
	case *x11.SelectionNotifyEvent:
		slog.Info("SelectionNotifyEvent", "event", ev)
		// TODO: Implement
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
			case x11Conn.Atoms.WMState:
				// TODO: Implement
			case x11Conn.Atoms.NetState:
				// TODO: Implement
			}
		}
	}
}
