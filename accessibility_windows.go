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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/internal/w32"
)

// axReadersFollowFocus reports that this platform's screen readers start from the keyboard focus, so that a panel which
// takes the focus only for an assistive technology's sake takes it here; see Panel.axTakesFocus. Narrator keeps its
// cursor on the focused element and, from a window in which nothing holds the focus, its scan mode, heading and link
// navigation all stay on the window's own element, while its item navigation alone reaches the content — and only by
// stepping through the title bar's buttons and the menu bar first. From any element inside the window, all of them
// move through the rest of it freely.
const axReadersFollowFocus = true

// The Windows side of accessibility support: this file connects the snapshots the root package publishes to the UI
// Automation provider in internal/w32, and connects the requests that come back from an assistive technology to the UI
// thread.
//
// Nothing here ordinarily runs until something sends a window a WM_GETOBJECT asking for UiaRootObjectId, which nothing
// but UI Automation does. That message reaches w32HandleGetObject in window_windows.go, and until it arrives no window
// has an adapter and no tree has been built. The one other way in is AccessibilityEnvKey: an application it forced
// support on for has activateAccessibility called during finishStartup, so every window's first draw reaches
// nativeAccessibilityPublish and builds an adapter with no WM_GETOBJECT ever sent, which is the path
// nativeAccessibilityEnabledChanged below and uiautomationcore_windows.go's lazy provider registration both count on.
// Once a window has an adapter it stays described for the rest of its life: UI Automation gives no notification that
// the last client has gone away, so there is nothing to deactivate on. What it does give is UiaClientsAreListening,
// which the adapter consults before raising anything, so a window that goes on being described after every client has
// gone costs the snapshots and nothing more.
//
// Everything here runs on the UI thread except the action callback, which UI Automation delivers on whichever thread it
// pleases. That one hands the work to the UI thread and returns at once, so nothing outside this file ever touches a
// live panel from another thread.

// w32AccessibilityAdapter returns the window's UI Automation adapter, creating it — and turning snapshot building on
// for the whole application — when nothing has asked about this window before. It reports nil when accessibility
// support has been refused by the environment or by the NoAccessibility startup option, and when the window has nothing
// to describe.
//
// The adapter itself is created by the publish this performs rather than here, since the adapter cannot exist without
// the snapshot it answers from. Both paths that reach it — a WM_GETOBJECT arriving for a window that has no adapter,
// and the ordinary after-draw publish of a window that gained one when some other window was asked about — therefore
// end up in nativeAccessibilityPublish with the first tree in hand.
func (w *Window) w32AccessibilityAdapter() *w32.UIAWindow {
	// The flag makes this re-entrant safe. Creating the adapter raises the events a window appearing for the first time
	// raises, and UI Automation answers an event by asking about what it names, so a second request for this window can
	// arrive before the first has finished being answered. Such a request is told there is no provider yet, which costs
	// it the fragment it asked for and nothing else, rather than starting the whole activation over again.
	if w.wnd.uia != nil || w.wnd.uiaActivating {
		return w.wnd.uia
	}
	// A window with no root panel — a bare Window something assembled itself — has nothing to describe.
	if !w.IsValid() || w.root == nil || !activateAccessibility() {
		return nil
	}
	if w.ax == nil {
		w.ax = &windowAccessibility{}
	}
	w.wnd.uiaActivating = true
	// Laid out first, since a message that arrives while a layout invalidation is still pending would otherwise be
	// answered from the frames the panels had before it — the very first bounds this client is given, and the ones it
	// draws its highlight from. Window.performAccessibilityAction lays the window out ahead of its publish for the same
	// reason.
	w.ValidateLayout()
	// Synchronously and without regard to the publish throttle: whatever asked has nothing else to be answered from.
	// This runs on the UI thread — WM_GETOBJECT is delivered to the thread that owns the window, even when UI
	// Automation sent it from another one — so walking the live panel hierarchy here is safe, whether the message
	// arrived from the main event loop or from a nested one put up by a modal dialog or a drag.
	w.publishAccessibilityNow()
	w.wnd.uiaActivating = false
	return w.wnd.uia
}

// nativeAccessibilityPublish hands a freshly built snapshot, and the events describing how it differs from the one
// before it, to the platform's assistive-technology adapter.
func (w *Window) nativeAccessibilityPublish(tree *accessibility.Tree, events []accessibility.Event) {
	if w.wnd.wnd == 0 || tree == nil {
		return
	}
	if w.wnd.uia == nil {
		// Creating the adapter is itself the window's first publish: it is handed the snapshot and raises what a window
		// being described for the first time raises, so the events that describe the step from no tree at all to this
		// one have nowhere to go and are not passed on.
		w.wnd.uia = w32.NewUIAWindow(w32.UIAConfig{
			Action: w.w32AccessibilityAction,
			HWND:   w.wnd.wnd,
		}, tree, w.w32AccessibilityGeometry())
		return
	}
	// Refreshed with every publish as well as on every move, resize and scale change, since a window that was moved
	// while no snapshot was being built has a stale origin and would point a screen reader's highlight at the wrong
	// place.
	w.wnd.uia.SetGeometry(w.w32AccessibilityGeometry())
	w.wnd.uia.Publish(tree, events)
}

// nativeAccessibilityGeometryChanged tells the adapter that the window has moved, resized or changed backing scale, so
// that the screen coordinates it reports are recomputed.
func (w *Window) nativeAccessibilityGeometryChanged() {
	if w.wnd.uia != nil {
		w.wnd.uia.SetGeometry(w.w32AccessibilityGeometry())
	}
}

// nativeAccessibilityShutdown releases everything the adapter holds for this window.
func (w *Window) nativeAccessibilityShutdown() {
	if w.wnd.uia != nil {
		// Before the window handle goes away: the adapter withdraws the provider it gave UI Automation for that handle,
		// and raises the event that says the window has closed, both of which name it.
		w.wnd.uia.Destroy()
		w.wnd.uia = nil
	}
}

// nativeAccessibilityWindowHidden keeps the provider of a window that has been hidden or minimized. UI Automation
// enumerates the top-level windows itself: a hidden one is not offered to a client and a minimized one reports that
// state through its window pattern, so nothing has to be withdrawn, and keeping the provider means a client finds the
// elements it already holds when the window comes back.
func (*Window) nativeAccessibilityWindowHidden() bool {
	return false
}

// nativeAccessibilityEnabledChanged turns support back on for an application the environment forced it on for, and
// otherwise has nothing to do on this platform: support starts again on the next WM_GETOBJECT asking for the UI
// Automation root, which activation refuses or allows as it stands at the time, and stopping has already shut every
// adapter down and withdrawn its providers.
//
// An application that AccessibilityEnvKey forced support on for has no such message to wait for, since UI Automation
// sends one only when a client of its own is there, so without this turning support off and back on would leave it
// describing nothing at all. Linux restores a forced-on application the same way, through linuxA11yStatusInit, and the
// promise SetAccessibilityEnabled makes is that the three platforms end up where they started.
func nativeAccessibilityEnabledChanged(enabled bool) {
	if enabled && accessibilityEnv.Load() > 0 {
		activateAccessibility()
	}
}

// nativeAccessibilityAnnounce asks the platform's assistive technology to speak text.
func nativeAccessibilityAnnounce(text string) {
	if w := w32AnnouncementWindow(); w != nil {
		w.wnd.uia.Announce(text)
	}
}

// w32AnnouncementWindow returns the window whose fragment an announcement is raised on. UI Automation has no notion of
// an application-wide announcement: it is an event on an element, so it has to come from some window's fragment root.
// The active window is the right one, since it is what the user is working in, but an announcement about something that
// finished in the background may well arrive while no window of ours is active, so any window with an adapter will do
// rather than none. It reports nil when no window has one, which is what an announcement made before anything has asked
// about a window looks like.
func w32AnnouncementWindow() *Window {
	if w := ActiveWindow(); w != nil && w.wnd.uia != nil {
		return w
	}
	for _, w := range windowList {
		if w.wnd.uia != nil {
			return w
		}
	}
	return nil
}

// w32AccessibilityAction receives one request an assistive technology has made of a node in this window.
//
// UI Automation calls providers on whichever thread it likes, so this may be running on any of them, and it must not
// wait on the UI thread: that thread may be inside a modal loop or a drag-and-drop loop, and a client that is kept
// waiting decides the application has stopped responding. The request is therefore queued and returned from at once.
// The assistive technology has already been told, optimistically, that the request will be carried out, and learns what
// actually happened from the events the next snapshot produces.
func (w *Window) w32AccessibilityAction(request accessibility.ActionRequest) {
	InvokeTask(func() { w.performAccessibilityAction(request) })
}

// w32AccessibilityGeometry returns where the window's content area sits on the screen, in physical pixels, along with
// the backing scale the window-local logical bounds in a snapshot must be multiplied by to reach that space.
//
// This is the platform's own answer rather than Window.accessibilityGeometry, which multiplies the content rect's
// origin by the scale: window and display rects on this platform already keep their origin in the raw global pixel
// space (see w32ApplyFrameInsets), so scaling it again would place a window on a 2x display at twice its actual
// distance from the top-left of the virtual screen. ClientToScreen gives the client area's origin in exactly the space
// UI Automation works in.
func (w *Window) w32AccessibilityGeometry() w32.UIAGeometry {
	var origin w32.POINT
	w32.ClientToScreen(w.wnd.wnd, &origin)
	return w32AccessibilityGeometryFor(origin, w.nativeBackingScale())
}

// w32AccessibilityGeometryFor turns the screen position of a window's client origin and the window's backing scale into
// the geometry the adapter converts node bounds with. It is separate from w32AccessibilityGeometry so that the
// conversion can be exercised without a window.
func w32AccessibilityGeometryFor(origin w32.POINT, scale geom.Point) w32.UIAGeometry {
	return w32.UIAGeometry{
		Origin: geom.NewPoint(float32(origin.X), float32(origin.Y)),
		Scale:  scale,
	}
}
