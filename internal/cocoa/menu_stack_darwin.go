// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"sync"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/errs"
)

// The NSEventMask bits for the presses a menu stack monitor watches, from NSEvent.h.
const (
	nsEventMaskLeftMouseDown  uint64 = 1 << 1
	nsEventMaskRightMouseDown uint64 = 1 << 3
	nsEventMaskOtherMouseDown uint64 = 1 << 25
	nsEventMaskMouseDown             = nsEventMaskLeftMouseDown | nsEventMaskRightMouseDown | nsEventMaskOtherMouseDown
)

// nsWorkspaceActiveSpaceDidChangeNotification is the name of NSWorkspace's notification that the active Space changed.
const nsWorkspaceActiveSpaceDidChangeNotification = "NSWorkspaceActiveSpaceDidChangeNotification"

// nsMenuDidBeginTrackingNotification is the name of NSMenu's notification that a menu has begun tracking, which the
// application's own menu bar posts as one of its menus opens.
const nsMenuDidBeginTrackingNotification = "NSMenuDidBeginTrackingNotification"

var (
	menuStackObserverClassOnce sync.Once
	menuStackObserverClass     objc.Class
	menuStackObserverClassErr  error
	menuStackObserver          objc.ID
	menuStackLocalMonitor      objc.ID
	menuStackGlobalMonitor     objc.ID
	menuStackLocalBlock        objc.Block
	menuStackGlobalBlock       objc.Block
	menuStackCallback          func(w Window)
	menuStackInCallback        bool
	menuStackRemovePending     bool
)

// InstallMenuStackMonitor arranges for f to be called on every mouse button press until RemoveMenuStackMonitor is
// called, with the window of this application the press was made in, or 0 for one made in none of them, which the
// global event monitor reports: in another application's window, which activates that application and so takes the
// key status from the window whose menus are open, or on the Dock, a menu bar extra or Notification Center, none of
// which do. A press on the application's own menu bar reaches neither monitor, since menu tracking consumes it before
// it is dispatched through sendEvent:, so f is called with 0 as a menu of the bar begins tracking instead, which also
// covers a menu opened from the keyboard or by an assistive technology. f is also called with 0 when the active Space
// changes. A further install replaces f. Main thread only.
func InstallMenuStackMonitor(f func(w Window)) {
	menuStackCallback = f
	if menuStackLocalMonitor != 0 {
		return
	}
	menuStackLocalBlock = objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		menuStackMonitorFired(Window(event.Send(Sel("window"))))
		return event
	})
	menuStackLocalMonitor = objc.ID(Cls("NSEvent")).Send(Sel("addLocalMonitorForEventsMatchingMask:handler:"),
		nsEventMaskMouseDown, menuStackLocalBlock)
	menuStackGlobalBlock = objc.NewBlock(func(_ objc.Block, _ objc.ID) { menuStackMonitorFired(0) })
	menuStackGlobalMonitor = objc.ID(Cls("NSEvent")).Send(Sel("addGlobalMonitorForEventsMatchingMask:handler:"),
		nsEventMaskMouseDown, menuStackGlobalBlock)
	menuStackObserverClassOnce.Do(registerMenuStackObserverClass)
	if menuStackObserverClassErr != nil {
		errs.Log(menuStackObserverClassErr)
		return
	}
	WithPool(func() {
		if menuStackObserver == 0 {
			menuStackObserver = objc.ID(menuStackObserverClass).Send(Sel("new"))
		}
		workspaceNotificationCenter().Send(Sel("addObserver:selector:name:object:"), menuStackObserver,
			Sel("activeSpaceDidChange:"), NSStringFromGo(nsWorkspaceActiveSpaceDidChangeNotification), 0)
		defaultNotificationCenter().Send(Sel("addObserver:selector:name:object:"), menuStackObserver,
			Sel("menuDidBeginTracking:"), NSStringFromGo(nsMenuDidBeginTrackingNotification), 0)
	})
}

// RemoveMenuStackMonitor undoes InstallMenuStackMonitor. Called from within the function the monitor is calling, it
// takes effect on the next pass of the main run loop rather than at once, since the block being run must not be
// released from inside its own run, which lasts until the event monitor's call into it returns, and a monitor
// installed again before then stays. Main thread only.
func RemoveMenuStackMonitor() {
	menuStackCallback = nil
	if menuStackInCallback {
		menuStackRemovePending = true
		return
	}
	removeMenuStackMonitorNow()
}

// removeMenuStackMonitorNow takes the monitors and observers down.
func removeMenuStackMonitorNow() {
	if menuStackLocalMonitor == 0 {
		return
	}
	events := objc.ID(Cls("NSEvent"))
	events.Send(Sel("removeMonitor:"), menuStackLocalMonitor)
	events.Send(Sel("removeMonitor:"), menuStackGlobalMonitor)
	menuStackLocalMonitor, menuStackGlobalMonitor = 0, 0
	menuStackLocalBlock.Release()
	menuStackGlobalBlock.Release()
	menuStackLocalBlock, menuStackGlobalBlock = 0, 0
	if menuStackObserver != 0 {
		WithPool(func() {
			workspaceNotificationCenter().Send(Sel("removeObserver:name:object:"), menuStackObserver,
				NSStringFromGo(nsWorkspaceActiveSpaceDidChangeNotification), 0)
			defaultNotificationCenter().Send(Sel("removeObserver:name:object:"), menuStackObserver,
				NSStringFromGo(nsMenuDidBeginTrackingNotification), 0)
		})
	}
}

// menuStackMonitorFired hands a press in w, or 0, to the installed function, if any, and arranges for a removal the
// function asked for to be carried out on the next pass of the main run loop, by which time the block that may be
// running now has returned, unless the function installed another in the meantime. Without the observer to perform
// that later, which is only the case when its class could not be registered, the removal is carried out at once.
func menuStackMonitorFired(w Window) {
	if menuStackCallback == nil {
		return
	}
	menuStackInCallback = true
	menuStackCallback(w)
	menuStackInCallback = false
	if menuStackRemovePending {
		menuStackRemovePending = false
		if menuStackCallback == nil {
			if menuStackObserver != 0 {
				menuStackObserver.Send(Sel("performSelectorOnMainThread:withObject:waitUntilDone:"),
					Sel("removeMonitorLater:"), 0, false)
			} else {
				removeMenuStackMonitorNow()
			}
		}
	}
}

// registerMenuStackObserverClass registers the class whose instance observes the active Space changing and a menu of
// the application's menu bar beginning to track, and carries out a removal deferred by menuStackMonitorFired, unless
// a monitor has been installed again since.
func registerMenuStackObserverClass() {
	menuStackObserverClass, menuStackObserverClassErr = objc.RegisterClass("macMenuStackObserver", Cls("NSObject"),
		nil, nil, []objc.MethodDef{{
			Cmd: Sel("activeSpaceDidChange:"),
			Fn:  func(_ objc.ID, _ objc.SEL, _ objc.ID) { menuStackMonitorFired(0) },
		}, {
			Cmd: Sel("menuDidBeginTracking:"),
			Fn:  func(_ objc.ID, _ objc.SEL, _ objc.ID) { menuStackMonitorFired(0) },
		}, {
			Cmd: Sel("removeMonitorLater:"),
			Fn: func(_ objc.ID, _ objc.SEL, _ objc.ID) {
				if menuStackCallback == nil {
					removeMenuStackMonitorNow()
				}
			},
		}})
	if menuStackObserverClassErr != nil {
		menuStackObserverClassErr = errs.NewWithCause("unable to register macMenuStackObserver class",
			menuStackObserverClassErr)
	}
}

// workspaceNotificationCenter returns the notification center of the shared NSWorkspace.
func workspaceNotificationCenter() objc.ID {
	return objc.ID(Cls("NSWorkspace")).Send(Sel("sharedWorkspace")).Send(Sel("notificationCenter"))
}

// defaultNotificationCenter returns the process's default NSNotificationCenter.
func defaultNotificationCenter() objc.ID {
	return objc.ID(Cls("NSNotificationCenter")).Send(Sel("defaultCenter"))
}
