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
// adapter hands the request to the root package and it arrives here.
//
// On Linux and Windows it arrives by way of the UI thread's task queue: the adapter is called on a thread of its
// platform's choosing and must not wait for a result, since the UI thread may be inside a modal loop or a drag, so the
// assistive technology is told optimistically that the request will be carried out and learns what actually happened
// from the events the next snapshot produces. macOS is the exception, and deliberately so. AppKit delivers
// accessibility callbacks on the main thread, which is the UI thread, and VoiceOver reads the result back the moment it
// has asked — the frame of the row it just scrolled to has to be the scrolled one by then — so a request that only
// moves the focus, the selection or the view is carried out on the spot, inside the callback. See axActionRunsInline in
// accessibility_darwin.go for exactly which those are; everything that may run application code is queued there too.

// axDisabledActions is every action a disabled node may still be asked to perform. Only scrolling into view survives:
// it acts on the node's ancestors rather than on the node, and an assistive technology moving through a window has to
// be able to bring what it is describing into view whether or not the control there can be used. Everything else is
// refused, exactly as the mouse and key paths refuse a disabled panel — Window.mouseDown, Window.mouseUp and
// Window.keyDown all gate on Panel.Enabled — so that an assistive technology cannot activate, change or type into what
// a person cannot. The snapshot builder narrows a disabled node's advertised actions to this same set, so that nothing
// is offered that would then be refused.
var axDisabledActions = accessibility.ActionSet(0).With(accessibility.ScrollIntoView)

// performAccessibilityAction carries out a request from an assistive technology. UI thread only.
//
// The node is resolved through the registry the most recently published tree was built with, so a request naming a node
// that has since gone away is refused rather than acted on. The panel that described the node is asked first, through
// its Accessibility.ActionCallback and then through AccessibilityActor, and only if neither handled the request does
// the default behavior for the action apply. Returns true if the request was carried out.
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

// performAccessibilityActions carries out a set of requests that an assistive technology made as one, describing the
// window once at the end rather than once per request. UI thread only. Returns true if any of them was carried out.
//
// Only one thing an assistive technology asks for names more than one node: which rows of a table, outline or list are
// selected. The schema has no request that replaces a selection wholesale, so the macOS adapter turns one of those into
// a Select followed by an AddToSelection apiece (see axSelectRows in internal/cocoa) and hands the set over together.
// Carried out one at a time through performAccessibilityAction, each would lay the window out, build a complete
// snapshot, diff it and post the notifications for it, so a set naming k rows would cost k snapshots, k selection
// notifications and k intermediate selections an assistive technology may read — from inside the single callback it is
// waiting on. The requests themselves are exactly the ones performAccessibilityAction carries out, gate for gate: each
// goes through dispatchAccessibilityAction, which refuses a window a modal is blocking, a node that has left the tree
// and a disabled one. One that is refused leaves the rest alone, since each row of the set stands on its own.
func (w *Window) performAccessibilityActions(reqs []accessibility.ActionRequest) bool {
	carried := false
	for _, req := range reqs {
		if w.dispatchAccessibilityAction(req) {
			carried = true
		}
	}
	if carried && accessibilityActive.Load() && w.IsValid() && w.root != nil && w.ax != nil {
		w.ValidateLayout()
		w.publishAccessibilityNow()
	}
	return carried
}

// dispatchAccessibilityAction finds the panel a request is aimed at and asks it, then the defaults, to carry the
// request out. See performAccessibilityAction.
func (w *Window) dispatchAccessibilityAction(req accessibility.ActionRequest) bool {
	if !w.IsValid() || w.ax == nil {
		return false
	}
	if !w.okToProcess() {
		// A window another window's modal is blocking refuses requests, as Window.mouseDown and Window.mouseUp refuse
		// mouse events. It is still described — an assistive technology is shown what is on the screen, and only the
		// top modal window's root says it is modal — but pressing something or moving the focus in a window the person
		// cannot touch would run application callbacks that are entitled to assume the modal is still in front.
		return false
	}
	target, ok := w.ax.targets[req.Node]
	if !ok || target.panel == nil || target.panel.Window() != w {
		return false
	}
	if node := w.ax.last.Node(req.Node); node != nil && node.Disabled && !axDisabledActions.Has(req.Action) {
		// The node was published as disabled, which for a virtual child such as a row of a table, or for a panel that
		// reports a state of its own through Accessibility.Callback, is the only place that is known: the panel those
		// belong to may itself be perfectly enabled. See axDisabledActions.
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
	if !p.Enabled() && !axDisabledActions.Has(req.Action) {
		// Gated here rather than in each widget so that it holds for every Accessibility.ActionCallback, every
		// AccessibilityActor implementation and every default behavior alike, including the requests aimed at a
		// virtual child of a disabled panel, which only this panel could have carried out. See axDisabledActions.
		return false
	}
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
		// Window.SetFocus does not refuse a target that cannot hold the focus: it hands the focus to that target's
		// first focusable child instead, or clears it entirely. Either would leave the focus somewhere other than the
		// node the request named while reporting that the request was carried out, so a panel that cannot take the
		// focus is refused outright, and what actually happened is checked afterwards in case the window declined it
		// for some other reason.
		if !p.Focusable() {
			return false
		}
		p.RequestFocus()
		if wnd := p.Window(); wnd == nil || !p.Is(wnd.CurrentFocus()) {
			return false
		}
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
//
// A disabled panel has none: Window.mouseDown and Window.mouseUp pass over a panel that is not enabled, so synthesizing
// a click here would do what a real one could not. axDispatchAction has already refused such a request, but the check
// is repeated here because this is what actually calls the callbacks.
//
// A panel that declines the press has none either. Window.mouseDown remembers which panel to deliver the release to
// only when the press was accepted — a panel that returns false is letting the press go to its parent instead, and
// never sees the matching release — so a release sent here regardless would be something no real click could produce.
// The request is reported as not carried out, which is the truth: the press went nowhere.
//
// A panel whose click pops a menu up has none when the menu would not land where the panel is. Such a widget refuses
// the request itself, and refusing is exactly what brings the default behavior into play, so without this the widget's
// own refusal would be undone by the click synthesized on its behalf. See axMenuOpener and axMayPopupMenu.
func (p *Panel) axSynthesizeClick() bool {
	if p.MouseDownCallback == nil || p.MouseUpCallback == nil || !p.Enabled() {
		return false
	}
	if opener, ok := p.Self.(axMenuOpener); ok && opener.axClickOpensMenu() && !axMayPopupMenu(p) {
		return false
	}
	if p.Focusable() {
		p.RequestFocus()
	}
	where := p.ContentRect(true).Center()
	pressed := false
	SafeCall(func() { pressed = p.MouseDownCallback(where, ButtonLeft, 1, 0) })
	if !pressed {
		return false
	}
	SafeCall(func() { p.MouseUpCallback(where, ButtonLeft, 0) })
	return true
}

// axMayPopupMenu reports whether a menu popped up on behalf of this panel would land where the panel is. Every request
// from an assistive technology that opens a menu has to be refused when it would not.
//
// A menu does not open inside the window of the widget that opened it: menu.createPopup inserts the popup panel into
// ActiveWindow()'s root (menu.go), and macMenu.Popup pops the native menu up over ActiveWindow()'s native window
// (menu_darwin.go). Both look that window up for themselves, so the question is simply whether the window they are
// going to find is this panel's. A request aimed at a widget in any other window would put that widget's menu up in
// whatever window is active, at coordinates translated from the one the widget is in, and a request made while no
// window is active would quietly do nothing at all while reporting that it had been carried out.
//
// Asking ActiveWindow() is what the window's own focus flag cannot do. A transient window — one that takes input
// without ever becoming the active one, such as a menu or a tooltip window — may perfectly well hold the focus, and
// ActiveWindow() still passes over it and answers with the next window that is not transient, so a menu opened on
// behalf of a widget in one goes up somewhere else entirely. Window.mouseDown does let a press through to such a
// window, and that clause is deliberately not repeated here, because the two answer different questions: there, whether
// this window may take input at all; here, where the menu that input would open is going to land.
//
// Window.dispatchAccessibilityAction cannot make this check for every action, since most of them act on the panel
// itself and are perfectly reasonable to ask of a background window; only the ones that open a menu are placed
// somewhere else entirely. Those are also narrowed out of what a node advertises, so that nothing is offered that would
// then be refused; see axSnapshot.narrowMenuActions.
func axMayPopupMenu(p *Panel) bool {
	wnd := p.Window()
	return wnd != nil && wnd == ActiveWindow()
}

// axMenuActions is implemented by a widget that advertises actions which would open a menu somewhere other than within
// its own window: a Field and a Markdown, both of which offer a contextual menu, and a PopupMenu, whose list of choices
// is one. The snapshot builder asks each such widget which of the actions it has just been described with are those,
// and takes them away when a menu opened on the widget's behalf would not land where the widget is. See
// axSnapshot.narrowMenuActions and axMayPopupMenu.
type axMenuActions interface {
	// axMenuOpeningActions returns the subset of the node's actions that would open a menu, given what the widget has
	// just been described as. An action that is carried out, or accepted as already done, whatever window the widget is
	// in is not one of them.
	axMenuOpeningActions(node *accessibility.Node) accessibility.ActionSet
}

// axMenuOpener is implemented by a widget whose click pops a menu up rather than doing something within its own window.
// The distinction matters only to axSynthesizeClick, which stands in for a click on a widget that has nothing else to
// be activated by: a click on one of these builds its menu somewhere other than where the widget is, so it is subject
// to axMayPopupMenu exactly as the widget's own handling of the request is.
type axMenuOpener interface {
	// axClickOpensMenu reports whether clicking this widget now would pop a menu up.
	axClickOpensMenu() bool
}
