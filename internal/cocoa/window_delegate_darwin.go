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

// Callbacks invoked by the window delegates created by NewWindowDelegate. They are invoked on the main thread from
// within the event loop.
var (
	// WindowShouldCloseCallback is invoked when the user asks a window to close (e.g. via its close button). If set,
	// the delegate reports NO to AppKit, making the callback responsible for actually closing the window; if nil, the
	// window closes normally.
	WindowShouldCloseCallback func(Window)
	// WindowDidResizeCallback is invoked after a window has been resized.
	WindowDidResizeCallback func(Window)
	// WindowDidMoveCallback is invoked after a window has been moved.
	WindowDidMoveCallback func(Window)
	// WindowMinimizeCallback is invoked after a window has been miniaturized (true) or deminiaturized (false).
	WindowMinimizeCallback func(Window, bool)
	// WindowDidBecomeKeyCallback is invoked after a window becomes the key window.
	WindowDidBecomeKeyCallback func(Window)
	// WindowDidResignKeyCallback is invoked after a window resigns key window status.
	WindowDidResignKeyCallback func(Window)
)

// WindowDelegate is a handle to a macWindowDelegate instance, which routes a window's NSWindowDelegate messages to
// the callback funcs above. NewWindowDelegate returns an owned (+1) reference; balance it with Release.
type WindowDelegate objc.ID

var (
	windowDelegateClassOnce sync.Once
	windowDelegateClass     objc.Class
	windowDelegateClassErr  error
)

// windowFromNotification returns the window a delegate notification refers to. The old Objective-C delegate captured
// its window in an ivar at init time, but every NSWindowDelegate notification carries that same window as the
// notification object (and windowShouldClose:'s sender is the window itself), so the window is derived from each
// message instead of stored.
func windowFromNotification(notification objc.ID) Window {
	return Window(notification.Send(Sel("object")))
}

// registerWindowDelegateClass registers the macWindowDelegate Objective-C class. Registration is process-global and
// can only happen once per class name, so it is guarded by windowDelegateClassOnce; instances are created per
// NewWindowDelegate call.
func registerWindowDelegateClass() {
	LoadAppKit()
	var protocols []*objc.Protocol
	if p := objc.GetProtocol("NSWindowDelegate"); p != nil {
		protocols = append(protocols, p)
	}
	cls, err := objc.RegisterClass("macWindowDelegate", Cls("NSObject"), protocols, nil, []objc.MethodDef{
		{
			Cmd: Sel("windowShouldClose:"),
			Fn: func(_ objc.ID, _ objc.SEL, sender objc.ID) bool {
				if WindowShouldCloseCallback == nil {
					return true
				}
				WindowShouldCloseCallback(Window(sender))
				return false
			},
		},
		{
			Cmd: Sel("windowDidResize:"),
			Fn: func(_ objc.ID, _ objc.SEL, notification objc.ID) {
				if WindowDidResizeCallback != nil {
					WindowDidResizeCallback(windowFromNotification(notification))
				}
			},
		},
		{
			Cmd: Sel("windowDidMove:"),
			Fn: func(_ objc.ID, _ objc.SEL, notification objc.ID) {
				if WindowDidMoveCallback != nil {
					WindowDidMoveCallback(windowFromNotification(notification))
				}
			},
		},
		{
			Cmd: Sel("windowDidMiniaturize:"),
			Fn: func(_ objc.ID, _ objc.SEL, notification objc.ID) {
				if WindowMinimizeCallback != nil {
					WindowMinimizeCallback(windowFromNotification(notification), true)
				}
			},
		},
		{
			Cmd: Sel("windowDidDeminiaturize:"),
			Fn: func(_ objc.ID, _ objc.SEL, notification objc.ID) {
				if WindowMinimizeCallback != nil {
					WindowMinimizeCallback(windowFromNotification(notification), false)
				}
			},
		},
		{
			Cmd: Sel("windowDidBecomeKey:"),
			Fn: func(_ objc.ID, _ objc.SEL, notification objc.ID) {
				if WindowDidBecomeKeyCallback != nil {
					WindowDidBecomeKeyCallback(windowFromNotification(notification))
				}
			},
		},
		{
			Cmd: Sel("windowDidResignKey:"),
			Fn: func(_ objc.ID, _ objc.SEL, notification objc.ID) {
				if WindowDidResignKeyCallback != nil {
					WindowDidResignKeyCallback(windowFromNotification(notification))
				}
			},
		},
	})
	if err != nil {
		windowDelegateClassErr = errs.NewWithCause("NewWindowDelegate: unable to register window delegate class", err)
		return
	}
	windowDelegateClass = cls
}

// NewWindowDelegate returns a new window delegate, or 0 if the delegate could not be created.
func NewWindowDelegate() WindowDelegate {
	windowDelegateClassOnce.Do(registerWindowDelegateClass)
	if windowDelegateClassErr != nil {
		errs.Log(windowDelegateClassErr)
		return 0
	}
	return WindowDelegate(objc.ID(windowDelegateClass).Send(Sel("new")))
}

// Release releases the delegate.
func (d WindowDelegate) Release() {
	Release(objc.ID(d))
}
