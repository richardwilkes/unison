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
	"encoding/binary"
	"fmt"
	"image"
	"log/slog"
	"math"
	"net/url"
	"os"
	"slices"
	"strings"
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

type apiWindow struct {
	dndInfo           *x11DragInfo
	id                x11.WindowID
	parent            x11.WindowID
	colorMap          x11.ColorMapID
	dndSource         x11.WindowID
	dndVersion        uint32
	lastX             float32
	lastY             float32
	borderTop         uint32
	borderLeft        uint32
	borderBottom      uint32
	borderRight       uint32
	borderValid       bool
	minimized         bool
	maximized         bool
	cursorWasHidden   bool
	awaitingConfigure bool
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
	w.apiSetWMClass()
	w.apiSetTitle(w.title)
	x11Conn.Flush()
	return nil
}

// apiSetWMClass sets the WM_CLASS property (the ICCCM instance and class names) on the window. Desktop environments use
// this to associate the window with its launcher/.desktop file (for the taskbar/dock icon, application grouping, etc.),
// so the class name is set to the application identifier, which is what a .desktop file's StartupWMClass entry should
// match.
func (w *Window) apiSetWMClass() {
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

func (w *Window) apiSetTitle(title string) {
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMName, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(title))
	x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMIconName, x11Conn.Atoms.UTF8String, 8, x11.PropModeReplace,
		[]byte(title))
	x11Conn.Flush()
}

func (w *Window) apiSetTitleIcons(images []*image.NRGBA) {
	if len(images) == 0 {
		x11Conn.DeleteProperty(w.wnd.id, x11Conn.Atoms.NetWMIcon)
	} else {
		size := 0
		for _, img := range images {
			size += 8 + img.Rect.Dy()*img.Rect.Dx()*4
		}
		data := make([]byte, size)
		offset := 0
		for _, img := range images {
			w := img.Rect.Dx()
			h := img.Rect.Dy()
			d := data[offset:]
			offset += 8 + w*h*4
			binary.LittleEndian.PutUint32(d, uint32(w))
			binary.LittleEndian.PutUint32(d[4:], uint32(h))
			pix := d[8:]
			for y := range h {
				row := y * img.Stride
				for x := range w {
					i := row + (x * 4)
					a := uint16(img.Pix[i+3])
					pix[i] = uint8((uint16(img.Pix[i+2]) * a) / 0xff)
					pix[i+1] = uint8((uint16(img.Pix[i+1]) * a) / 0xff)
					pix[i+2] = uint8((uint16(img.Pix[i]) * a) / 0xff)
					pix[i+3] = img.Pix[i+3]
				}
			}
		}
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.NetWMIcon, x11.AtomCardinal, 32, x11.PropModeReplace, data)
	}
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

func (w *Window) apiCurrentKeyModifiers() mod.Modifiers {
	return x11CurrentKeyModifiers()
}

func (w *Window) apiUpdateCursorImage() {
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

func (w *Window) apiCursorInContentArea() bool {
	qpr := x11Conn.QueryPointer(w.wnd.id)
	if qpr == nil {
		return false
	}
	return w.x11ConvertRawMouse(geom.NewPoint(float32(qpr.RootX), float32(qpr.RootY))).In(w.apiContentRect())
}

func (w *Window) apiCursorPosition() geom.Point {
	qpr := x11Conn.QueryPointer(w.wnd.id)
	if qpr == nil {
		return geom.Point{}
	}
	return w.x11ConvertRawMouse(geom.NewPoint(float32(qpr.WinX), float32(qpr.WinY)))
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
	if w.wnd.minimized {
		x11Conn.DeiconifyWindow(w.wnd.id)
	} else {
		x11Conn.IconifyWindow(w.wnd.id)
	}
}

func (w *Window) apiMaximize() {
	if w.wnd.maximized {
		x11Conn.DemaximizeWindow(w.wnd.id)
	} else {
		x11Conn.MaximizeWindow(w.wnd.id)
	}
}

func (w *Window) apiAcquireFocusAndBringToFront() {
	x11Conn.ConfigureWindow(w.wnd.id, x11.ConfigureWindowMaskStackMode, &x11.ConfigureWindowRequest{
		StackMode: x11.StackModeAbove,
	})
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
	// Draw the window now that it is visible. The filtered wait above discards the nil wake-up events posted by
	// MarkForRedraw, so a redraw queued while the window was being prepared would otherwise be lost. When a window is
	// shown from within an event handler (such as a modal dialog raised while handling the window manager's close
	// request), the event loop would then block in WaitEvents with nothing left to wake it, leaving the window blank
	// and the application unresponsive. Drawing here guarantees the freshly shown window is painted regardless of
	// event ordering, and also covers the case where the X server never sends an Expose.
	w.draw()
}

func (w *Window) apiHide() {
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

func (w *Window) apiStartDrag(img *Image, origin geom.Point, opMask drag.Op, data ...drag.Data) {
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
	if !x11Conn.GrabPointer(w.wnd.id, x11.EventMaskButtonPress|x11.EventMaskButtonRelease|x11.EventMaskPointerMotion,
		0) {
		slog.Warn("unable to grab the pointer to start a drag")
		return
	}
	defer x11Conn.UngrabPointer()
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
			// Coalesce any additional queued motion events into this one
			for {
				next := x11Conn.PollEvents(func(pe x11.Event) bool {
					_, ok := pe.(*x11.MotionNotifyEvent)
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
			x11Conn.RespondToSelectionRequest(ev)
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

func (w *Window) apiUpdateRegisteredDragTypes(types []*uti.DataType) {
	if len(types) == 0 {
		x11Conn.DeleteProperty(w.wnd.id, x11Conn.Atoms.DnDAware)
	} else {
		buf := x11.NewWriter(4)
		buf.Atom(x11.DnDVersion)
		x11Conn.ChangeProperty(w.wnd.id, x11Conn.Atoms.DnDAware, x11.AtomAtom, 32, x11.PropModeReplace, buf.Retrieve())
	}
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
	apiPollEvents()
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
		if t, l, b, r, ok := x11Conn.GetWindowBorderWidths(w.wnd.id); ok {
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
	if format == 32 && actualType == x11.AtomCardinal && len(value) >= 8 {
		r := x11.NewReader(value)
		w.wnd.borderLeft = r.Uint32()
		w.wnd.borderRight = r.Uint32()
		w.wnd.borderTop = r.Uint32()
		w.wnd.borderBottom = r.Uint32()
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
	switch ev := e.(type) {
	case *x11.ErrorEvent:
		errs.Log(ev.Error)
	case *x11.ReparentNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			w.wnd.parent = ev.Parent
		}
	case *x11.KeyPressEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			if key, ok := rawScanCodeToKeyCodeMap[uint16(ev.Detail)]; ok {
				mods := x11TranslateModifierState(ev.State)
				w.keyPressed(key, mods)
				if mods&(mod.Control|mod.Option|mod.Command) == 0 {
					keySym := x11ScanCodeToKeySym(uint16(ev.Detail), mods)
					if ch := x11KeySymToUnicode(keySym); ch != utf8.RuneError {
						w.runeTyped(ch)
					}
				}
			}
		}
	case *x11.KeyReleaseEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			if key, ok := rawScanCodeToKeyCodeMap[uint16(ev.Detail)]; ok {
				mods := x11TranslateModifierState(ev.State)
				w.keyReleased(key, mods)
			}
		}
	case *x11.ButtonPressEvent:
		if w := x11FindWindow(ev.Event); w != nil {
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
		if w := x11FindWindow(ev.Event); w != nil {
			mods := x11TranslateModifierState(ev.State)
			where := w.x11ConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY)))
			switch ev.Detail {
			case 1:
				w.mouseUp(where, ButtonLeft, mods)
			case 2:
				w.mouseUp(where, ButtonMiddle, mods)
			case 3:
				w.mouseUp(where, ButtonRight, mods)
			case 4, 5, 6, 7:
			default:
				w.mouseUp(where, int(ev.Detail-5), mods)
			}
		}
	case *x11.EnterNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.apiUpdateCursorImage()
			w.mouseEnter(w.x11ConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY))),
				x11TranslateModifierState(ev.State))
		}
	case *x11.LeaveNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.apiUpdateCursorImage()
			w.mouseExit()
		}
	case *x11.MotionNotifyEvent:
		if w := x11FindWindow(ev.Event); w != nil {
			w.mouseMovedOrDragged(w.x11ConvertRawMouse(geom.NewPoint(float32(ev.EventX), float32(ev.EventY))),
				x11TranslateModifierState(ev.State))
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
				y := int16((ev.Data32[2]) & 0xffff)
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
		x11Conn.RespondToSelectionRequest(ev)
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
				minimized := r.Uint32() == x11.StateIconic
				if minimized != w.wnd.minimized {
					w.wnd.minimized = minimized
					if w.MinimizedCallback != nil {
						SafeCall(func() { w.MinimizedCallback(minimized) })
					}
				}
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
				if maximized != w.wnd.maximized {
					w.wnd.maximized = maximized
					if w.MaximizedCallback != nil {
						SafeCall(func() { w.MaximizedCallback(maximized) })
					}
				}
			}
		}
	}
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
	data, ok := x11Conn.DnDSelectionBytes(target, d.timestamp)
	if !ok {
		data = nil
	}
	if d.cache == nil {
		d.cache = make(map[x11.Atom][]byte)
	}
	d.cache[target] = data
	return data
}
