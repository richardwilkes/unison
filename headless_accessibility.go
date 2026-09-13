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
	"github.com/richardwilkes/toolbox/v2/xreflect"
	"github.com/richardwilkes/unison/accessibility"
)

// This file is the headless stand-in for a platform assistive-technology adapter, together with the driver methods a
// test drives it through. The adapter does what the real ones do — it keeps the most recently published tree and
// accumulates the events it was told about — and nothing else, so what a test sees is exactly what a screen reader
// would have been handed.
//
// A session starts with accessibility support inactive, exactly as an application whose platform has no assistive
// technology running does, so a test that never asks for it pays nothing and can assert as much. EnableAccessibility
// and AccessibilityTree are the two ways of asking.

// accessibilityPublish adopts the tree as what this window currently looks like to an assistive technology and records
// the events describing how it got there.
func (hw *headlessWindow) accessibilityPublish(tree *accessibility.Tree, events []accessibility.Event) {
	hw.axTree = tree
	hw.axEvents = append(hw.axEvents, events...)
}

// accessibilityGeometryChanged does nothing. The platforms that need it recompute the screen coordinates they hand out
// from Window.accessibilityGeometry; a headless window's tree is read in its own window-local coordinates, so there is
// nothing to recompute.
func (hw *headlessWindow) accessibilityGeometryChanged() {
}

// accessibilityShutdown drops everything recorded for this window, as a real adapter releases the elements it was
// answering from.
func (hw *headlessWindow) accessibilityShutdown() {
	hw.axTree = nil
	hw.axEvents = nil
}

// accessibilityAnnounce records text as something that would have been spoken.
func (s *headlessState) accessibilityAnnounce(text string) {
	s.announcements = append(s.announcements, text)
}

// EnableAccessibility turns on accessibility support for the session, as the arrival of a screen reader would, and
// waits for every window to be described. Until this or AccessibilityTree has been called, no snapshot is ever built,
// which is what a test asserting that an inactive session costs nothing relies on.
//
// It does nothing if the NoAccessibility startup option, or AccessibilityEnvKey set to a false value, has refused
// accessibility support for the application under test.
func (s *HeadlessScreen) EnableAccessibility() {
	s.Do(func() { activateAccessibility() })
}

// AccessibilityTree returns the most recent description of the window, as a platform adapter would have been handed it,
// turning accessibility support on first if it is not on already and then rebuilding the description so that what comes
// back reflects the window as it is now. Returns nil if the session has ended, the window is not one of this session's,
// or accessibility support has been refused.
func (s *HeadlessScreen) AccessibilityTree(w *Window) *accessibility.Tree {
	var tree *accessibility.Tree
	s.Do(func() {
		if !activateAccessibility() || !w.IsValid() {
			return
		}
		w.publishAccessibility()
		if hw := headlessWindowFor(w); hw != nil {
			tree = hw.axTree
		}
	})
	return tree
}

// AccessibilityEvents returns the events published for the window since the last time it was asked, and forgets them,
// so a test asserts on what happened between two points rather than on everything that ever has. Enable accessibility
// support first, with EnableAccessibility or AccessibilityTree; a window nothing has ever been published for has no
// events.
func (s *HeadlessScreen) AccessibilityEvents(w *Window) []accessibility.Event {
	var events []accessibility.Event
	s.Do(func() {
		if hw := headlessWindowFor(w); hw != nil {
			events = hw.axEvents
			hw.axEvents = nil
		}
	})
	return events
}

// Announcements returns the text passed to AnnounceForAccessibility since the last time it was asked, and forgets it.
func (s *HeadlessScreen) Announcements() []string {
	var announcements []string
	s.Do(func() {
		announcements = s.announcements
		s.announcements = nil
	})
	return announcements
}

// PerformAccessibilityAction asks a node to do something, exactly as an assistive technology would. The window holding
// the node is found from what was most recently published, so the request must name a node from a tree this session has
// handed out. Returns true if the request was carried out.
func (s *HeadlessScreen) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	handled := false
	s.Do(func() {
		for _, w := range windowList {
			if w.ax == nil {
				continue
			}
			if _, ok := w.ax.targets[req.Node]; ok {
				handled = w.performAccessibilityAction(req)
				return
			}
		}
	})
	return handled
}

// AccessibilityNodeFor returns the node describing a panel in the most recently published description of its window, or
// nil if there is not one — because accessibility support is off, because the panel is hidden or hides itself with
// role.None, or because it does not belong to a window at all. Ask for the window's tree first, so that what comes back
// describes the panel as it is now.
func (s *HeadlessScreen) AccessibilityNodeFor(p Paneler) *accessibility.Node {
	var node *accessibility.Node
	s.Do(func() {
		if xreflect.IsNil(p) {
			return
		}
		panel := p.AsPanel()
		if panel == nil {
			return
		}
		if w := panel.Window(); w != nil && w.ax != nil {
			node = w.ax.last.Node(panel.Accessibility.id)
		}
	})
	return node
}
