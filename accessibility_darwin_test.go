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

	// An action for a window that does not exist is dropped rather than queued.
	resetTaskQueue()
	cocoa.AccessibilityActionCallback(macUnknownWindow, accessibility.ActionRequest{Action: accessibility.Press})
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
