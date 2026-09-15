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
// A hidden window is the one place it has to pick one platform's behavior over another's. Everything recorded for a
// window that has gone off the screen is dropped here, which is what AT-SPI calls for and therefore what Linux does;
// macOS and Windows both keep what they built, since there the system lists the application's windows itself and a
// hidden one simply drops out of that list. Withdrawing is the answer with something to assert on — a test can watch
// the description go and come back, where keeping it is the absence of any change — but a test written against it is
// pinning what Linux does rather than what all three platforms do. See Window.apiAccessibilityWindowHidden.
//
// A session starts with accessibility support inactive, exactly as an application whose platform has no assistive
// technology running does, so a test that never asks for it pays nothing and can assert as much. EnableAccessibility
// and AccessibilityTree are the two ways of asking.

// headlessMaxAXEvents bounds how many published events one window holds for a test that has not read them. A real
// adapter turns each batch into platform notifications as it arrives and keeps nothing, so this backlog exists only
// here; a test that enables accessibility support and then runs a long interaction without reading the events must not
// be able to grow it for the life of the session. The newest are the ones kept, since they describe what the test is
// about to assert on.
const headlessMaxAXEvents = 4096

// headlessMaxAXAnnouncements bounds how many announcements the session holds for a test that has not read them, for the
// same reason headlessMaxAXEvents bounds the events: a real adapter hands each one to the platform and keeps nothing,
// so a session that announces in a loop and never calls Announcements() must not be able to grow this for the life of
// the session. The newest are the ones kept.
const headlessMaxAXAnnouncements = 4096

// accessibilityPublish adopts the tree as what this window currently looks like to an assistive technology and records
// the events describing how it got there.
func (hw *headlessWindow) accessibilityPublish(tree *accessibility.Tree, events []accessibility.Event) {
	hw.axTree = tree
	hw.axEvents = append(hw.axEvents, events...)
	if extra := len(hw.axEvents) - headlessMaxAXEvents; extra > 0 {
		hw.axEvents = append(hw.axEvents[:0], hw.axEvents[extra:]...)
	}
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
	if extra := len(s.announcements) - headlessMaxAXAnnouncements; extra > 0 {
		s.announcements = append(s.announcements[:0], s.announcements[extra:]...)
	}
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
	described := false
	if !s.Do(func() {
		if !activateAccessibility() || !w.IsValid() {
			return
		}
		w.publishAccessibility()
		described = true
	}) || !described {
		return nil
	}
	// Read in a second visit to the UI thread rather than alongside the publish above, because Do waits for everything
	// that publish set in motion only after the closure has returned: turning accessibility support on marks every
	// window for redraw, and each of those redraws describes its window again, so a tree taken inside the closure would
	// already have been superseded by the time this returned — and would disagree with what AccessibilityNodeFor,
	// which reads the window's own record of the last publish, answered for the very same node.
	var tree *accessibility.Tree
	s.run(func() {
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
//
// Only the most recent headlessMaxAXEvents are kept, so a test that runs a long interaction before asking may find the
// beginning of it gone. Ask between the steps that matter rather than once at the end.
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
//
// An action the node does not advertise in the tree it was last published in is refused without being asked of the
// window at all, which is what every platform adapter does: each of them reads the action set of the node it is
// answering for and passes the request on only if what is being asked for is in it — see internal/atspi.Component,
// w32AccessibilityProvider and the Cocoa element's setAccessibilityFocused:. A test that could reach past that would be
// exercising a path no assistive technology can take, and would go on passing after the widget stopped offering the
// action at all.
func (s *HeadlessScreen) PerformAccessibilityAction(req accessibility.ActionRequest) bool {
	handled := false
	s.Do(func() {
		for _, w := range windowList {
			if w.ax == nil {
				continue
			}
			if _, ok := w.ax.targets[req.Node]; !ok {
				continue
			}
			if node := w.ax.last.Node(req.Node); node == nil || !node.Actions.Has(req.Action) {
				// The node has left the published tree, or does not offer this. The search goes on for the same reason
				// it does below: another window may describe the node and offer it.
				continue
			}
			if handled = w.performAccessibilityAction(req); handled {
				return
			}
			// The window that last described the node refused to act on it, which is what a window says about a panel
			// that has since been reparented into another one and that it has not been described without since. The
			// search goes on rather than dropping the request on the floor.
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
		// The id is this panel's only when the panel is the one it was handed out to. An id that arrived by having
		// another panel's AccessibilityInfo assigned onto this one describes that other panel, and answering with the
		// node for it would be a quiet lie about the panel that was asked about. The builder replaces such an id the
		// next time the panel is described, but a panel that is never described — one that is hidden, or hides itself
		// with role.None — keeps it. See axIDFor.
		if panel.Accessibility.id == 0 || panel.Accessibility.owner != panel {
			return
		}
		if w := panel.Window(); w != nil && w.ax != nil {
			node = w.ax.last.Node(panel.Accessibility.id)
		}
	})
	return node
}
