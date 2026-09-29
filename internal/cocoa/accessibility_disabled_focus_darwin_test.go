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
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/role"
)

// TestAXDisabledNodeOfferingTheFocus proves that whether a disabled node may be given the keyboard focus is decided by
// its action set alone. A disabled node that offers the focus is a control the application asked to have read (see
// unison.SetFocusForReading): it says it is not enabled, says it holds the focus, lets AXFocused be set and passes the
// request on. One that does not offer the focus refuses, and nothing is asked of the application for it.
func TestAXDisabledNodeOfferingTheFocus(t *testing.T) {
	defer func() { AccessibilityActionCallback = nil }()
	runOnMain(func() {
		const (
			rootID accessibility.NodeID = iota + 1
			readableID
			unreachableID
		)
		tree := &accessibility.Tree{
			Nodes: map[accessibility.NodeID]*accessibility.Node{
				rootID: {
					ID: rootID, Children: []accessibility.NodeID{readableID, unreachableID}, Role: role.Window,
					Name: "Test Window", Bounds: geom.NewRect(0, 0, 320, 240), Focused: true,
				},
				readableID: {
					ID: readableID, Parent: rootID, Role: role.Button, Name: "Apply",
					Bounds: geom.NewRect(10, 20, 80, 24), Disabled: true, Focusable: true, Focused: true,
					Actions: accessibility.ActionSet(0).With(accessibility.ScrollIntoView, accessibility.Focus),
				},
				unreachableID: {
					ID: unreachableID, Parent: rootID, Role: role.Button, Name: "Revert",
					Bounds: geom.NewRect(10, 50, 80, 24), Disabled: true,
					Actions: accessibility.ActionSet(0).With(accessibility.ScrollIntoView),
				},
			},
			Root:       rootID,
			Focus:      readableID,
			Generation: 1,
		}
		v, a, cleanup := newAXAdapterWithTree(t, tree)
		defer cleanup()
		if a == nil {
			return
		}
		var requests []accessibility.ActionRequest
		AccessibilityActionCallback = func(_ Window, req accessibility.ActionRequest) {
			requests = append(requests, req)
		}
		WithPool(func() {
			children := axTestChildren(t, v, 2)
			readable, unreachable := children[0], children[1]
			for _, one := range []struct {
				name      string
				element   objc.ID
				reachable bool
			}{
				{name: "the disabled button that offers the focus", element: readable, reachable: true},
				{name: "the disabled button that does not", element: unreachable},
			} {
				if objc.Send[bool](one.element, Sel("isAccessibilityEnabled")) {
					t.Errorf("%s says it is enabled", one.name)
				}
				if got := objc.Send[bool](one.element, Sel("isAccessibilityFocused")); got != one.reachable {
					t.Errorf("%s: isAccessibilityFocused = %v, want %v", one.name, got, one.reachable)
				}
				if got := axSelectorAllowed(one.element, "setAccessibilityFocused:"); got != one.reachable {
					t.Errorf("%s allows setAccessibilityFocused: = %v, want %v", one.name, got, one.reachable)
				}
				if got := axAttributeSettable(one.element, "AXFocused"); got != one.reachable {
					t.Errorf("%s reports AXFocused settable = %v, want %v", one.name, got, one.reachable)
				}
				if axSelectorAllowed(one.element, "accessibilityPerformPress") {
					t.Errorf("%s allows accessibilityPerformPress", one.name)
				}
				one.element.Send(Sel("setAccessibilityFocused:"), true)
			}
		})
		want := []accessibility.ActionRequest{{Node: readableID, Action: accessibility.Focus}}
		if !slices.Equal(requests, want) {
			t.Errorf("asking for the focus passed on %v, want %v", requests, want)
		}
	})
}
