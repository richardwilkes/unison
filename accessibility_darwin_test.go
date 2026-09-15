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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// macUnknownWindow is a window handle no unison window can have, for exercising the callbacks' unknown-window paths
// without going anywhere near AppKit. Zero would not do: a headless window's native handle is zero, so macFindWindow
// would match one.
const macUnknownWindow cocoa.Window = 0xdead0000

// macTestWindow is a window handle for a hand-built window put into the window list, so that the paths which begin by
// finding the window a request names can be followed past that point without AppKit having created anything.
const macTestWindow cocoa.Window = 0xdead0001

// macSaveAccessibilityState puts the process-wide accessibility flags, and the window list that activation walks, back
// the way the test found them. The tests below turn snapshot building on and off, which nothing else in the package
// expects to find changed, and hand-build windows that no event loop knows about.
func macSaveAccessibilityState(t *testing.T) {
	t.Helper()
	savedActive := accessibilityActive.Load()
	savedEnv := accessibilityEnv.Load()
	savedRefused := noAccessibility.Load()
	savedWindows := windowList
	windowList = nil
	accessibilityActive.Store(false)
	accessibilityEnv.Store(0)
	noAccessibility.Store(false)
	t.Cleanup(func() {
		accessibilityActive.Store(savedActive)
		accessibilityEnv.Store(savedEnv)
		noAccessibility.Store(savedRefused)
		windowList = savedWindows
	})
}

// TestMacAccessibilityCallbacksInstalled proves the two Cocoa accessibility callbacks are wired up, which is what
// nativeLateInit does at startup, and that both refuse a window they cannot resolve rather than acting on the wrong
// one. Everything the adapter itself does needs a real screen and is tested in internal/cocoa. This test mutates
// global state and therefore must not call t.Parallel.
func TestMacAccessibilityCallbacksInstalled(t *testing.T) {
	c := check.New(t)
	savedActivate := cocoa.AccessibilityActivateCallback
	savedAction := cocoa.AccessibilityActionCallback
	t.Cleanup(func() {
		cocoa.AccessibilityActivateCallback = savedActivate
		cocoa.AccessibilityActionCallback = savedAction
	})
	cocoa.AccessibilityActivateCallback = nil
	cocoa.AccessibilityActionCallback = nil

	macInitAccessibilityCallbacks()
	c.NotNil(cocoa.AccessibilityActivateCallback)
	c.NotNil(cocoa.AccessibilityActionCallback)

	// A window that does not exist activates nothing, so nothing is built and no adapter appears.
	before := axSnapshotCount
	c.False(cocoa.AccessibilityActivateCallback(macUnknownWindow))
	c.Equal(before, axSnapshotCount)
	c.False(IsAccessibilityActive())

	// A request that arrived somewhere other than the user interface thread is handed to it in full, since finding the
	// window it names is a read of the window list and only that thread may make one. It is dropped there.
	resetTaskQueue()
	cocoa.AccessibilityActionCallback(macUnknownWindow, accessibility.ActionRequest{Action: accessibility.Press})
	length, _ := taskQueueState()
	c.Equal(1, length, "a request that arrived off the user interface thread must be handed to it")
	processNextTask()
	length, _ = taskQueueState()
	c.Equal(0, length, "and must be dropped there, since no window answers to it")

	// On the user interface thread the window is looked up on the spot, and a request naming one that does not exist is
	// dropped without anything being queued at all.
	withUIThreadIdentity(t)
	resetTaskQueue()
	cocoa.AccessibilityActionCallback(macUnknownWindow, accessibility.ActionRequest{Action: accessibility.Press})
	length, _ = taskQueueState()
	c.Equal(0, length, "a request naming a window that does not exist must not have been queued")
}

// TestMacAxActionRunsInline proves which requests are carried out while the assistive technology that made them waits,
// which is what makes this platform re-enter its own adapter: only a request that moves the focus, the selection or the
// view, plus setting or stepping a scroll bar, since VoiceOver reads the result of those straight after asking.
// Anything else may run application code — a press can open a modal dialog — and so is queued instead. This test
// mutates no global state, but it is the contract the adapter's deferred element release exists for.
func TestMacAxActionRunsInline(t *testing.T) {
	c := check.New(t)
	const (
		scrollBarID accessibility.NodeID = 1
		buttonID    accessibility.NodeID = 2
	)
	w := &Window{ax: &windowAccessibility{last: &accessibility.Tree{
		Nodes: map[accessibility.NodeID]*accessibility.Node{
			scrollBarID: {ID: scrollBarID, Role: role.ScrollBar},
			buttonID:    {ID: buttonID, Role: role.Button},
		},
		Root: buttonID,
	}}}
	for _, action := range []accessibility.Action{
		accessibility.Focus, accessibility.ScrollIntoView, accessibility.Select, accessibility.AddToSelection,
		accessibility.RemoveFromSelection, accessibility.Expand, accessibility.Collapse, accessibility.SetTextSelection,
	} {
		c.True(axActionIsNavigation(action), action)
		c.True(w.axActionRunsInline(accessibility.ActionRequest{Node: buttonID, Action: action}), action)
	}
	// Stepping or setting a value is inline for a scroll bar, which is how VoiceOver brings something into view before
	// reading where it is, and queued for everything else, since setting a value runs application code.
	for _, action := range []accessibility.Action{
		accessibility.SetValue, accessibility.Increment, accessibility.Decrement,
	} {
		c.False(axActionIsNavigation(action), action)
		c.True(w.axActionRunsInline(accessibility.ActionRequest{Node: scrollBarID, Action: action}), action)
		c.False(w.axActionRunsInline(accessibility.ActionRequest{Node: buttonID, Action: action}), action)
	}
	for _, action := range []accessibility.Action{
		accessibility.Press, accessibility.Toggle, accessibility.ShowContextMenu, accessibility.ReplaceText,
	} {
		c.False(axActionIsNavigation(action), action)
		c.False(w.axActionRunsInline(accessibility.ActionRequest{Node: scrollBarID, Action: action}), action)
	}
	// A window that has published nothing yet has no node to judge a scroll bar by, so nothing runs inline but the
	// navigation requests.
	bare := &Window{}
	c.False(bare.axActionRunsInline(accessibility.ActionRequest{Node: scrollBarID, Action: accessibility.SetValue}))
	c.True(bare.axActionRunsInline(accessibility.ActionRequest{Node: scrollBarID, Action: accessibility.Focus}))
}

// TestMacAccessibilityHooksAreInertWithoutAView covers what every path from the root package to the adapter must do
// for a window that has no content view: nothing at all. Both halves of that case are real. The environment override
// turns snapshot building on during startup, so a window reaches the publish path before AppKit has given it a view,
// and a window that is being destroyed reaches it with the view already gone. Everything the adapter itself does needs
// a real view and is tested in internal/cocoa. This test mutates global state and therefore must not call t.Parallel.
func TestMacAccessibilityHooksAreInertWithoutAView(t *testing.T) {
	c := check.New(t)
	w := &Window{wnd: &apiWindow{}, ax: &windowAccessibility{}}
	w.nativeAccessibilityPublish(&accessibility.Tree{}, nil)
	c.Nil(w.wnd.ax, "no adapter may be created for a window with no view")
	c.False(w.ax.adapterFailed, "and nothing may be latched as having failed, since nothing was attempted")
	c.NotPanics(w.nativeAccessibilityGeometryChanged)
	c.NotPanics(w.nativeAccessibilityShutdown)
	c.Nil(w.wnd.ax)
	c.False(w.nativeAccessibilityWindowHidden(),
		"this platform keeps everything it built for a window that has been hidden")

	// A window nothing has ever asked about has no accessibility state at all, which the publish path refuses before it
	// looks at the view.
	bare := &Window{wnd: &apiWindow{}}
	bare.nativeAccessibilityPublish(&accessibility.Tree{}, nil)
	c.Nil(bare.wnd.ax)
}

// TestMacAccessibilityAdapterFailureIsLatched covers the flag that takes a window out of the description for good. The
// only way an adapter fails to be created is the accessibility element class failing to register, which is a property
// of the process rather than of this window or this moment, so every later attempt would fail the same way — and a
// window being described runs through the publish path as often as every axPublishThrottle, which would turn one
// failure into a steady stream of them. This test mutates global state and therefore must not call t.Parallel.
func TestMacAccessibilityAdapterFailureIsLatched(t *testing.T) {
	c := check.New(t)
	macSaveAccessibilityState(t)
	accessibilityActive.Store(true)
	snapshots := axSnapshotCount
	w := &Window{wnd: &apiWindow{}, ax: &windowAccessibility{adapterFailed: true}}
	w.publishAccessibilityNow()
	c.Equal(snapshots, axSnapshotCount, "a window with nothing to publish to must not be described at all")
	c.True(w.ax.lastPublish.IsZero(), "nor counted as having been published")
	// The check is made again on the way to the adapter, so a publish that reached it by some other route stops there.
	w.nativeAccessibilityPublish(&accessibility.Tree{}, nil)
	c.Nil(w.wnd.ax)
}

// TestMacAccessibilityEnabledChangedRestoresAForcedApplication covers the one thing this platform has to do when
// SetAccessibilityEnabled lifts a refusal. macOS says nothing about an assistive technology arriving — a query to a
// window's content view is the only signal there is — so an application that AccessibilityEnvKey forced support on for
// has nothing to wait for, and without this, turning support off and back on would leave it describing nothing for the
// rest of its life. This test mutates global state and therefore must not call t.Parallel.
func TestMacAccessibilityEnabledChangedRestoresAForcedApplication(t *testing.T) {
	c := check.New(t)
	macSaveAccessibilityState(t)

	// Nothing forced support on, so there is nothing to restore: the next query a content view receives turns it back
	// on, exactly as at startup.
	nativeAccessibilityEnabledChanged(true)
	c.False(IsAccessibilityActive())

	// Forced on by the environment, and so restored on the spot.
	accessibilityEnv.Store(1)
	nativeAccessibilityEnabledChanged(true)
	c.True(IsAccessibilityActive())

	// Being told that support has been refused never turns anything on: the refusal has already shut every adapter
	// down, and activation would refuse it anyway.
	accessibilityActive.Store(false)
	nativeAccessibilityEnabledChanged(false)
	c.False(IsAccessibilityActive())
}

// TestMacAccessibilityActivateRefusesAWindowWithNothingToDescribe covers the guards the first accessibility query a
// content view receives passes through. A window that is not valid, or that has no root panel — a bare Window
// something assembled itself — has nothing to describe, and answering such a query must not turn snapshot building on
// for the whole application. The answer for a window that does have something to describe needs AppKit, since it
// depends on an adapter being created, and is tested in internal/cocoa. This test mutates global state and therefore
// must not call t.Parallel.
func TestMacAccessibilityActivateRefusesAWindowWithNothingToDescribe(t *testing.T) {
	c := check.New(t)
	macSaveAccessibilityState(t)
	snapshots := axSnapshotCount
	w := &Window{wnd: &apiWindow{}}
	w.wnd.wnd = macTestWindow
	windowList = append(windowList, w)

	c.False(macAccessibilityActivate(macTestWindow), "a window that is not valid has nothing to describe")
	w.valid = true
	c.False(macAccessibilityActivate(macTestWindow), "nor has one with no root panel")
	c.False(IsAccessibilityActive(), "and neither may have turned snapshot building on")
	c.Equal(snapshots, axSnapshotCount, "no snapshot may have been built")
	c.Nil(w.ax, "and no state may have been created for the window")
}
