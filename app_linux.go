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
	"log/slog"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/internal/x11"
)

var x11Conn *x11.Conn

func apiBeginStartup() error {
	var err error
	if x11Conn, err = x11.NewConn(); err != nil {
		return err
	}
	apiFillKeyCodes()
	return nil
}

func apiLateInit() {
}

func apiFinalFinishStartup() {
}

func apiTerminate() error {
	if x11Conn != nil {
		x11Conn.Close()
		x11Conn = nil
	}
	return nil
}

func apiBeep() {
	x11Conn.Bell(0)
}

func apiIsColorModeTrackingPossible() bool {
	return false
}

func apiIsDarkModeEnabled() bool {
	return false
}

func apiDoubleClickInterval() time.Duration {
	return 500 * time.Millisecond
}

func apiPollEvents() {
	x11ProcessEvent(x11Conn.PollEvents(nil))
}

func apiWaitEvents() {
	x11ProcessEvent(x11Conn.WaitEvents(nil))
}

func apiPostEmptyEvent() {
	x11Conn.PostEmptyEvent()
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
		if w := x11FindWindow(ev.Child); w != nil {
			mods := x11TranslateState(ev.State)
			switch ev.Detail {
			case 1:
				w.nativeMouseClick(ButtonLeft, true, mods)
			case 2:
				w.nativeMouseClick(ButtonRight, true, mods)
			case 3:
				w.nativeMouseClick(ButtonMiddle, true, mods)
			case 4:
				w.nativeMouseWheel(geom.NewPoint(0, 1), false)
			case 5:
				w.nativeMouseWheel(geom.NewPoint(0, -1), false)
			case 6:
				w.nativeMouseWheel(geom.NewPoint(1, 0), false)
			case 7:
				w.nativeMouseWheel(geom.NewPoint(-1, 0), false)
			default:
				w.nativeMouseClick(int(ev.Detail-5), true, mods)
			}
		}
	case *x11.ButtonReleaseEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			mods := x11TranslateState(ev.State)
			switch ev.Detail {
			case 1:
				w.nativeMouseClick(ButtonLeft, false, mods)
			case 2:
				w.nativeMouseClick(ButtonRight, false, mods)
			case 3:
				w.nativeMouseClick(ButtonMiddle, false, mods)
			case 4, 5, 6, 7:
			default:
				w.nativeMouseClick(int(ev.Detail-5), false, mods)
			}
		}
	case *x11.EnterNotifyEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			w.apiUpdateCursorImage()
			w.mouseEnter(geom.NewPoint(float32(ev.EventX), float32(ev.EventY)), w.lastKeyModifiers)
		}
	case *x11.LeaveNotifyEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			w.apiUpdateCursorImage()
			w.mouseExit()
		}
	case *x11.MotionNotifyEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			w.nativeMouseMoved(geom.NewPoint(float32(ev.EventX), float32(ev.EventY)))
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
					w.nativeRequestClose()
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
			// TODO: Old code for Linux cleared its internal flags for key and button pressed states
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

func x11TranslateState(state uint16) Modifiers {
	return Modifiers(state) & AllModifiers
}
