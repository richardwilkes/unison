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
	"strconv"
	"strings"

	"github.com/richardwilkes/unison/accessibility"
)

// This file is the return path: what happens when an assistive technology asks a node to do something. The platform
// adapter hands the request to the root package, which marshals it onto the UI thread — the adapters never wait for a
// result, since the UI thread may be inside a modal loop or a drag — and it arrives here.

// performAccessibilityAction carries out a request from an assistive technology. UI thread only.
//
// The node is resolved through the registry the most recently published tree was built with, so a request naming a node
// that has since gone away is refused rather than acted on. The panel that described the node is asked first, through
// its Accessibility.ActionCallback and then through AccessibilityActor, and only if neither handled the request does the
// default behavior for the action apply. Returns true if the request was carried out.
//
// A request that was carried out is followed at once by a fresh description of the window, laid out first so that
// whatever the request scrolled or moved is where it now is. An assistive technology reads what it asked for straight
// after asking — the frame of the row it just selected, say — and a description that waited for the next redraw would
// hand it the old answer, which VoiceOver then keeps: it does not read a frame again merely because a scroll moved it.
func (w *Window) performAccessibilityAction(req accessibility.ActionRequest) bool {
	if !w.dispatchAccessibilityAction(req) {
		return false
	}
	if accessibilityActive.Load() && w.IsValid() && w.root != nil && w.ax != nil {
		w.ValidateLayout()
		w.publishAccessibilityNow()
	}
	return true
}

// dispatchAccessibilityAction finds the panel a request is aimed at and asks it, then the defaults, to carry the request
// out. See performAccessibilityAction.
func (w *Window) dispatchAccessibilityAction(req accessibility.ActionRequest) bool {
	if !w.IsValid() || w.ax == nil {
		return false
	}
	target, ok := w.ax.targets[req.Node]
	if !ok || target.panel == nil || target.panel.Window() != w {
		return false
	}
	// The widget is told which of its virtual children the request is aimed at, which is the only way it can tell one
	// row of a table from another.
	req.Key = target.key
	return target.panel.axDispatchAction(req, target.key != nil)
}

// axDispatchAction asks a panel to carry out a request: through its Accessibility.ActionCallback, then through
// AccessibilityActor, and then — unless the request is aimed at one of the panel's virtual children, which the defaults
// cannot act on since a virtual child is not a panel — through the default behavior for the action.
func (p *Panel) axDispatchAction(req accessibility.ActionRequest, virtual bool) bool {
	handled := false
	if p.Accessibility.ActionCallback != nil {
		SafeCall(func() { handled = p.Accessibility.ActionCallback(req) })
		if handled {
			return true
		}
	}
	if actor, ok := p.Self.(AccessibilityActor); ok {
		SafeCall(func() { handled = actor.PerformAccessibilityAction(req) })
		if handled {
			return true
		}
	}
	if virtual {
		return false
	}
	switch req.Action {
	case accessibility.Focus:
		p.RequestFocus()
		p.ScrollIntoView()
		return true
	case accessibility.ScrollIntoView:
		p.ScrollIntoView()
		return true
	case accessibility.Press:
		return p.axSynthesizeClick()
	default:
		return false
	}
}

// axPanelAtPath returns the panel at the given position beneath root, where the position is child indexes from the
// root down, joined with dots, as axCellPanelKey.Path holds them. An empty path is the root itself; a path that leads
// nowhere yields nil.
func axPanelAtPath(root *Panel, path string) *Panel {
	p := root
	if path == "" {
		return p
	}
	for _, part := range strings.Split(path, ".") {
		index, err := strconv.Atoi(part)
		if err != nil || p == nil {
			return nil
		}
		children := p.Children()
		if index < 0 || index >= len(children) {
			return nil
		}
		p = children[index]
	}
	return p
}

// axSynthesizeClick presses and releases the mouse at the center of the panel, which is how a panel that has no notion
// of being activated other than being clicked on is activated. The focus moves first, if the panel can hold it, so that
// the sequence is the one a person's click would have produced. Reports false if the panel has no click to synthesize.
func (p *Panel) axSynthesizeClick() bool {
	if p.MouseDownCallback == nil || p.MouseUpCallback == nil {
		return false
	}
	if p.Focusable() {
		p.RequestFocus()
	}
	where := p.ContentRect(true).Center()
	SafeCall(func() { p.MouseDownCallback(where, ButtonLeft, 1, 0) })
	SafeCall(func() { p.MouseUpCallback(where, ButtonLeft, 0) })
	return true
}
