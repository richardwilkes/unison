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
	"log/slog"

	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// axReadersFollowFocus reports whether this platform's screen readers start from the keyboard focus. VoiceOver's cursor
// is its own: it moves through a window's content from the window itself, whether or not anything inside holds the
// focus, so a panel that takes the focus only for an assistive technology's sake — see Panel.axTakesFocus — has no
// reason to take it here, and a window in which nothing holds the focus is left as it is.
const axReadersFollowFocus = false

// The macOS side of accessibility support: this file connects the snapshots the root package publishes to the
// NSAccessibility adapter in internal/cocoa, and connects the requests that come back from an assistive technology to
// the UI thread.
//
// Nothing here runs until an assistive technology queries a window's content view. That query reaches
// macAccessibilityActivate, which is the only thing on this platform that turns snapshot building on; before it happens
// no window has an adapter and no tree has been built. Once it has happened the window stays described for the rest of
// its life: macOS gives no notification that the last assistive technology has gone away, so there is nothing to
// deactivate on.

// macInitAccessibilityCallbacks installs the three callbacks the Cocoa accessibility adapter reaches the root package
// through. It is called once, from nativeLateInit, and installs nothing but function pointers: no snapshot is built and
// no adapter is created until an assistive technology asks something.
func macInitAccessibilityCallbacks() {
	cocoa.AccessibilityActivateCallback = macAccessibilityActivate
	cocoa.AccessibilityActionCallback = func(macWnd cocoa.Window, req accessibility.ActionRequest) {
		// Finding the window is itself UI-thread work: the search is over windowList, which every window opening and
		// closing rewrites there. AppKit delivers accessibility callbacks on the main thread, which is the UI thread,
		// so this is the ordinary case; a call from anywhere else hands the whole thing over rather than reading that
		// list where it stands.
		if !onUIThread() {
			InvokeTask(func() { macPerformAccessibilityAction(macWnd, req) })
			return
		}
		w := macFindWindow(macWnd)
		if w == nil {
			slog.Warn("received accessibility action callback for unknown window", "window", macWnd)
			return
		}
		// A request that merely moves the focus, the selection or the view is carried out on the spot: the adapter is
		// called on the main thread from within the run loop, so this is no different from handling a key press — and
		// VoiceOver reads the result straight after asking, so the frame of the row it just selected or scrolled to has
		// to be the scrolled one by then. Anything that activates something is queued instead, since it may run
		// application code that opens a modal dialog, and VoiceOver must not be left waiting for that.
		if w.axActionRunsInline(req) {
			w.performAccessibilityAction(req)
			return
		}
		InvokeTask(func() { w.performAccessibilityAction(req) })
	}
	cocoa.AccessibilityActionsCallback = func(macWnd cocoa.Window, reqs []accessibility.ActionRequest) {
		// The adapter hands over a set only for setting which rows are selected, whose requests are all carried out
		// inline, so there is no per-request choice to make here: they are carried out together, and the window is
		// described once at the end rather than once per row. The thread check is the one the single-request callback
		// makes, and for the same reason.
		if !onUIThread() {
			InvokeTask(func() { macPerformAccessibilityActions(macWnd, reqs) })
			return
		}
		macPerformAccessibilityActions(macWnd, reqs)
	}
}

// macPerformAccessibilityAction finds the window a request names and carries the request out. UI thread only; it is
// what a request that arrived on some other thread is handed to the UI thread as.
func macPerformAccessibilityAction(macWnd cocoa.Window, req accessibility.ActionRequest) {
	w := macFindWindow(macWnd)
	if w == nil {
		slog.Warn("received accessibility action callback for unknown window", "window", macWnd)
		return
	}
	w.performAccessibilityAction(req)
}

// macPerformAccessibilityActions finds the window a set of requests names and carries the whole set out as one. UI
// thread only.
func macPerformAccessibilityActions(macWnd cocoa.Window, reqs []accessibility.ActionRequest) {
	w := macFindWindow(macWnd)
	if w == nil {
		slog.Warn("received accessibility action callback for unknown window", "window", macWnd)
		return
	}
	w.performAccessibilityActions(reqs)
}

// axActionIsNavigation reports whether an action only moves the focus, the selection or the view — the requests an
// assistive technology makes as it travels through a window, whose effect it expects to read back immediately — as
// opposed to one that activates something, which may run arbitrary application code, up to and including a modal
// dialog, and so must never be carried out while the assistive technology is waiting for an answer.
func axActionIsNavigation(action accessibility.Action) bool {
	switch action {
	case accessibility.Focus, accessibility.ScrollIntoView, accessibility.Select, accessibility.AddToSelection,
		accessibility.RemoveFromSelection, accessibility.Expand, accessibility.Collapse, accessibility.SetTextSelection:
		return true
	default:
		return false
	}
}

// axActionRunsInline reports whether a request may be carried out while the assistive technology that made it waits
// for the answer: the navigation requests (see axActionIsNavigation), and setting or stepping a scroll bar, which is
// how VoiceOver scrolls something into view before it reads where that something is. Everything else may run arbitrary
// application code and is queued instead.
//
// Carrying a request out on the spot lays the window out again and publishes a fresh snapshot before the adapter has
// returned to AppKit, so the adapter is re-entered from inside its own accessibility callback. That is deliberate — it
// is the only way the answer VoiceOver reads back describes the state its request produced — and internal/cocoa is
// written for it: an element whose node leaves the tree during such a publish is autoreleased rather than released
// outright, so it outlives the AppKit call that is standing on it.
func (w *Window) axActionRunsInline(req accessibility.ActionRequest) bool {
	if axActionIsNavigation(req.Action) {
		return true
	}
	switch req.Action {
	case accessibility.SetValue, accessibility.Increment, accessibility.Decrement:
		if w.ax != nil && w.ax.last != nil {
			if n := w.ax.last.Node(req.Node); n != nil && n.Role == role.ScrollBar {
				return true
			}
		}
	default:
	}
	return false
}

// macAccessibilityActivate answers the first accessibility query a window's content view receives. Snapshot building is
// turned on for the whole application and this window is described immediately, synchronously, since the query that
// caused this has nothing else to be answered from. It reports whether the window now has an adapter to answer with.
func macAccessibilityActivate(macWnd cocoa.Window) bool {
	w := macFindWindow(macWnd)
	if w == nil || !w.IsValid() || w.root == nil {
		return false
	}
	if !activateAccessibility() {
		return false
	}
	if w.ax == nil {
		w.ax = &windowAccessibility{}
	}
	// Laid out first, since a query that arrives while a layout invalidation is still pending would otherwise be
	// answered from the frames the panels had before it — the very first bounds VoiceOver is given, and the ones it
	// draws its cursor from. Window.performAccessibilityAction lays the window out ahead of its publish for the same
	// reason.
	w.ValidateLayout()
	w.publishAccessibilityNow()
	return w.wnd.ax != nil
}

// nativeAccessibilityPublish hands a freshly built snapshot, and the events describing how it differs from the one
// before it, to the platform's assistive-technology adapter.
//
// An adapter that could not be created is not asked for again. The only way that happens is the accessibility element
// class failing to register, which is a property of the process rather than of this window or this moment, so every
// later attempt would fail in the same way — and a window being described runs through here as often as every
// axPublishThrottle, which would turn one failure into a steady stream of them.
func (w *Window) nativeAccessibilityPublish(tree *accessibility.Tree, events []accessibility.Event) {
	if w.wnd.view == 0 || w.ax == nil || w.ax.adapterFailed {
		return
	}
	if w.wnd.ax == nil {
		if w.wnd.ax = cocoa.NewAXAdapter(w.wnd.view); w.wnd.ax == nil {
			w.ax.adapterFailed = true
			return
		}
	}
	w.wnd.ax.Publish(tree, events)
}

// nativeAccessibilityGeometryChanged is a no-op on macOS: the adapter reports frames by converting each node's
// window-local bounds through the content view and the window every time it is asked, so a window that has moved,
// resized or changed backing scale needs nothing recomputed.
func (*Window) nativeAccessibilityGeometryChanged() {
}

// nativeAccessibilityShutdown releases everything the adapter holds for this window.
func (w *Window) nativeAccessibilityShutdown() {
	if w.wnd.ax != nil {
		w.wnd.ax.Shutdown()
		w.wnd.ax = nil
	}
}

// nativeAccessibilityWindowHidden keeps the adapter of a window that has been hidden or minimized. AppKit lists the
// application's windows itself, so a window that is off the screen drops out of what an assistive technology sees
// without anything being withdrawn, and keeping the elements means VoiceOver finds the ones it already knows, its
// cursor included, when the window is shown again.
func (*Window) nativeAccessibilityWindowHidden() bool {
	return false
}

// nativeAccessibilityEnabledChanged turns support back on for an application the environment forced it on for, and
// otherwise has nothing to do on this platform: support starts again on the next query a content view receives, which
// activation refuses or allows as it stands at the time, and stopping has already shut every adapter down.
//
// An application that AccessibilityEnvKey forced support on for has no such query to wait for, since a content view is
// asked only when an assistive technology is actually there, so without this turning support off and back on would
// leave it describing nothing at all. Linux restores a forced-on application the same way, through linuxA11yStatusInit,
// and the promise SetAccessibilityEnabled makes is that the three platforms end up where they started.
func nativeAccessibilityEnabledChanged(enabled bool) {
	if enabled && accessibilityEnv.Load() > 0 {
		activateAccessibility()
	}
}

// nativeAccessibilityAnnounce asks the platform's assistive technology to speak text.
func nativeAccessibilityAnnounce(text string) {
	cocoa.AXAnnounce(text)
}
