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
		} else {
			slog.Info("ReparentNotifyEvent for unknown window")
		}
	case *x11.KeyPressEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("KeyPressEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("KeyPressEvent for unknown window")
		}
	case *x11.KeyReleaseEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("KeyReleaseEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("KeyReleaseEvent for unknown window")
		}
	case *x11.ButtonPressEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("ButtonPressEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("ButtonPressEvent for unknown window")
		}
	case *x11.ButtonReleaseEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("ButtonReleaseEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("ButtonReleaseEvent for unknown window")
		}
	case *x11.EnterNotifyEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("EnterNotifyEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("EnterNotifyEvent for unknown window")
		}
	case *x11.LeaveNotifyEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("LeaveNotifyEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("LeaveNotifyEvent for unknown window")
		}
	case *x11.MotionNotifyEvent:
		if w := x11FindWindow(ev.Child); w != nil {
			slog.Info("MotionNotifyEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("MotionNotifyEvent for unknown window")
		}
	case *x11.ConfigureNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("ConfigureNotifyEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("ConfigureNotifyEvent for unknown window")
		}
	case *x11.ClientMessageEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("ClientMessageEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("ClientMessageEvent for unknown window")
		}
	case *x11.SelectionNotifyEvent:
		slog.Info("SelectionNotifyEvent", "event", ev)
		// TODO: Implement
	case *x11.FocusInEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("FocusInEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("FocusInEvent for unknown window")
		}
	case *x11.FocusOutEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("FocusOutEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("FocusOutEvent for unknown window")
		}
	case *x11.ExposeEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("ExposeEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("ExposeEvent for unknown window")
		}
	case *x11.PropertyNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("PropertyNotifyEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("PropertyNotifyEvent for unknown window")
		}
	case *x11.DestroyNotifyEvent:
		if w := x11FindWindow(ev.Window); w != nil {
			slog.Info("DestroyNotifyEvent", "event", ev)
			// TODO: Implement
		} else {
			slog.Info("DestroyNotifyEvent for unknown window")
		}
	default:
		slog.Info(fmt.Sprintf("Unknown event: %T", ev))
	}
}
