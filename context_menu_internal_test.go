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
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/toolbox/v2/tid"
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/enums/mod"
	"github.com/richardwilkes/unison/enums/role"
	"github.com/richardwilkes/unison/enums/side"
)

// activateForContextMenu makes w the active window, with others behind it, since Panel.ShowContextMenu shows nothing
// for a panel in any other window. The window list is restored when the test ends.
func activateForContextMenu(t *testing.T, w *Window, others ...*Window) {
	t.Helper()
	saveFrontState(t)
	windowList = append([]*Window{w}, others...)
	pendingFrontWindow = nil
}

// newContextMenuTestWindow returns the active window, whose content panel's menu callback counts its calls in asked,
// records the position in where and returns nil, so nothing pops up.
func newContextMenuTestWindow(t *testing.T, asked *int, where *geom.Point) *Window {
	t.Helper()
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	w.root.contentPanel.ContextMenuCallback = func(pt geom.Point) Menu {
		*asked++
		*where = pt
		return nil
	}
	return w
}

// The kinds of mouse event a cmMouseLog records.
const (
	cmDown = "down"
	cmDrag = "drag"
	cmUp   = "up"
)

// cmMouseLog records the mouse events delivered to a panel, in order.
type cmMouseLog struct {
	events []cmMouseEvent
}

// cmMouseEvent is one recorded mouse event; count is set only for a press.
type cmMouseEvent struct {
	kind   string
	where  geom.Point
	button int
	count  int
	mods   mod.Modifiers
}

// logMouse has p claim every press and records every mouse event delivered to it.
func (l *cmMouseLog) logMouse(p *Panel) {
	p.MouseDownCallback = func(where geom.Point, button, count int, mods mod.Modifiers) bool {
		l.events = append(l.events, cmMouseEvent{kind: cmDown, where: where, button: button, count: count, mods: mods})
		return true
	}
	p.MouseDragCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
		l.events = append(l.events, cmMouseEvent{kind: cmDrag, where: where, button: button, mods: mods})
		return true
	}
	p.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
		l.events = append(l.events, cmMouseEvent{kind: cmUp, where: where, button: button, mods: mods})
		return true
	}
}

func (l *cmMouseLog) kinds() []string {
	kinds := make([]string, 0, len(l.events))
	for _, e := range l.events {
		kinds = append(kinds, e.kind)
	}
	return kinds
}

func (l *cmMouseLog) buttons() []int {
	buttons := make([]int, 0, len(l.events))
	for _, e := range l.events {
		buttons = append(buttons, e.button)
	}
	return buttons
}

// cmPressHandler implements ContextMenuPressHandler, recording the presses it is told of and answering with want.
type cmPressHandler struct {
	Panel
	presses  int
	lastAt   geom.Point
	lastMods mod.Modifiers
	want     bool
}

func newCMPressHandler(want bool) *cmPressHandler {
	p := &cmPressHandler{want: want}
	p.Self = p
	return p
}

// ContextMenuPressed implements ContextMenuPressHandler.
func (p *cmPressHandler) ContextMenuPressed(where geom.Point, mods mod.Modifiers) bool {
	p.presses++
	p.lastAt = where
	p.lastMods = mods
	return p.want
}

// newPressHandlerTestWindow returns the active window holding owner, a cmPressHandler at (20, 30) whose menu callback
// counts its calls in asked and returns nil, and whose mouse events go to log.
func newPressHandlerTestWindow(t *testing.T, want bool,
	asked *int,
) (w *Window, owner *cmPressHandler, log *cmMouseLog) {
	t.Helper()
	w = newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	owner = newCMPressHandler(want)
	owner.SetFrameRect(geom.NewRect(20, 30, 100, 100))
	owner.ContextMenuCallback = func(_ geom.Point) Menu {
		*asked++
		return nil
	}
	w.root.contentPanel.AddChild(owner)
	log = &cmMouseLog{}
	log.logMouse(owner.AsPanel())
	return w, owner, log
}

// TestContextMenuOpensOnTheRightButtonRelease verifies that a right-click asks for the menu at the release position
// without delivering any of its events to panels, though the window's own callbacks are still told of them, and that a
// right press on a panel with no menu is delivered normally.
func TestContextMenuOpensOnTheRightButtonRelease(t *testing.T) {
	c := check.New(t)
	var asked int
	var where geom.Point
	w := newContextMenuTestWindow(t, &asked, &where)
	var log cmMouseLog
	log.logMouse(w.root.contentPanel)
	var windowEvents []string
	w.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
		windowEvents = append(windowEvents, cmDown)
		return false
	}
	w.MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		windowEvents = append(windowEvents, cmDrag)
		return false
	}
	w.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		windowEvents = append(windowEvents, cmUp)
		return false
	}
	_, drift := DragGestureParameters()
	w.mouseDown(geom.NewPoint(10, 10), ButtonRight, 0)
	c.Equal(0, len(log.events), "the press is the window's")
	c.Nil(w.lastMouseDownPanel, "and no panel is sent what follows it")
	c.Equal(0, asked, "the menu waits for the release")
	w.mouseDrag(geom.NewPoint(10+drift, 10), ButtonRight, 0)
	c.Equal(0, len(log.events), "nor is a drag within the threshold delivered")
	w.mouseUp(geom.NewPoint(12, 14), ButtonRight, 0)
	c.Equal(0, len(log.events), "nor the release")
	c.Equal([]string{cmDown, cmDrag, cmUp}, windowEvents, "the window's own callbacks are told of all three")
	c.Equal(1, asked)
	c.Equal(geom.NewPoint(12, 14), where, "the menu is asked for where the button was released")
	c.Nil(w.contextMenuPanel)
	c.False(w.rightPressTaken)

	w.root.contentPanel.ContextMenuCallback = nil
	w.lastButtonTime = time.Time{}
	w.mouseDown(geom.NewPoint(10, 10), ButtonRight, 0)
	w.mouseDrag(geom.NewPoint(11, 10), ButtonRight, 0)
	w.mouseUp(geom.NewPoint(11, 10), ButtonRight, 0)
	c.Equal([]string{cmDown, cmDrag, cmUp}, log.kinds(), "a right press with no menu is delivered as any other")
	c.Equal(1, asked)
}

// TestContextMenuPressFocusesTheOwner verifies that a right press focuses the menu's owner when it is focusable and
// otherwise leaves the focus where it was, the press being taken for the menu either way.
func TestContextMenuPressFocusesTheOwner(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	other := NewPanel()
	other.SetFrameRect(geom.NewRect(0, 0, 20, 20))
	other.SetFocusable(true)
	owner := NewPanel()
	owner.SetFrameRect(geom.NewRect(50, 50, 50, 50))
	var asked int
	owner.ContextMenuCallback = func(_ geom.Point) Menu {
		asked++
		return nil
	}
	w.root.contentPanel.AddChild(other)
	w.root.contentPanel.AddChild(owner)
	other.RequestFocus()
	c.True(w.focus == other, "the test needs the focus to start out elsewhere")

	w.mouseDown(geom.NewPoint(60, 60), ButtonRight, 0)
	c.True(w.contextMenuPanel == owner, "the press is taken for the owner's menu")
	c.True(w.focus == other, "an owner that cannot hold the focus leaves it where it was")
	w.mouseUp(geom.NewPoint(60, 60), ButtonRight, 0)
	c.Equal(1, asked, "and is asked for its menu on the release")
	c.True(w.focus == other)

	owner.SetFocusable(true)
	w.lastButtonTime = time.Time{}
	w.mouseDown(geom.NewPoint(60, 60), ButtonRight, 0)
	c.True(w.contextMenuPanel == owner)
	c.True(w.focus == owner, "an owner that can hold the focus takes it on the press")
	w.mouseUp(geom.NewPoint(60, 60), ButtonRight, 0)
	c.Equal(2, asked)
}

// TestContextMenuPressHandlerGetsThePressBackOnADrag verifies that the handler is told of each right press, and that
// answering true has a press that becomes a drag delivered as made, followed by its drags and release, with no menu.
func TestContextMenuPressHandlerGetsThePressBackOnADrag(t *testing.T) {
	c := check.New(t)
	var asked int
	w, owner, log := newPressHandlerTestWindow(t, true, &asked)
	_, drift := DragGestureParameters()
	press := geom.NewPoint(40, 50)

	// A right-click first, so that the press that becomes a drag has a click count of two.
	w.mouseDown(press, ButtonRight, mod.Option)
	c.Equal(1, owner.presses, "the handler is told of the press")
	c.Equal(geom.NewPoint(20, 20), owner.lastAt, "where it was made, in the owner's space")
	c.Equal(mod.Option, owner.lastMods, "and with which modifiers")
	w.mouseDrag(geom.NewPoint(press.X+drift, press.Y), ButtonRight, mod.Option)
	w.mouseUp(geom.NewPoint(press.X+drift, press.Y), ButtonRight, mod.Option)
	c.Equal(1, asked, "a drift within the threshold is still a right-click")
	c.Equal(0, len(log.events), "and delivers nothing, even to a handler that would take a drag")

	w.mouseDown(press, ButtonRight, mod.Shift)
	c.Equal(2, w.lastButtonCount)
	c.Equal(2, owner.presses, "the handler is told of every press")
	c.Equal(mod.Shift, owner.lastMods)
	c.Equal(0, len(log.events), "the press is not delivered while it may still be a right-click")
	dragTo := geom.NewPoint(press.X, press.Y+drift*2)
	w.mouseDrag(dragTo, ButtonRight, mod.Shift)
	c.Equal([]string{cmDown, cmDrag}, log.kinds(), "a drag hands the press back, followed by the move that made it one")
	if len(log.events) == 2 {
		down := log.events[0]
		c.Equal(geom.NewPoint(20, 20), down.where, "the press is delivered where it was made")
		c.Equal(ButtonRight, down.button)
		c.Equal(2, down.count, "with its click count")
		c.Equal(mod.Shift, down.mods, "and its modifiers")
		c.Equal(geom.NewPoint(20, 20+drift*2), log.events[1].where)
	}
	c.Nil(w.contextMenuPanel, "the drag is no longer a right-click")
	w.mouseDrag(geom.NewPoint(press.X, press.Y+drift*3), ButtonRight, mod.Shift)
	w.mouseUp(geom.NewPoint(press.X, press.Y+drift*3), ButtonRight, mod.Shift)
	c.Equal([]string{cmDown, cmDrag, cmDrag, cmUp}, log.kinds(), "the drags and the release follow as for any press")
	c.Equal(1, asked, "and no menu opens")
	c.Nil(w.lastMouseDownPanel)
}

// TestContextMenuPressHandlerDeclinesADrag verifies that a right-drag delivers nothing and opens no menu when the
// handler answers false.
func TestContextMenuPressHandlerDeclinesADrag(t *testing.T) {
	c := check.New(t)
	var asked int
	w, owner, log := newPressHandlerTestWindow(t, false, &asked)
	_, drift := DragGestureParameters()
	w.mouseDown(geom.NewPoint(40, 50), ButtonRight, 0)
	c.Equal(1, owner.presses, "the handler is still told of the press")
	w.mouseDrag(geom.NewPoint(40+drift*2, 50), ButtonRight, 0)
	w.mouseDrag(geom.NewPoint(40+drift*3, 50), ButtonRight, 0)
	w.mouseUp(geom.NewPoint(40+drift*3, 50), ButtonRight, 0)
	c.Equal(0, len(log.events), "nothing is delivered")
	c.Equal(0, asked, "and no menu opens")
}

// TestContextMenuNotOpenedByARightDrag verifies that a right press moved beyond the drag threshold opens no menu, while
// one moved within it is still a click.
func TestContextMenuNotOpenedByARightDrag(t *testing.T) {
	c := check.New(t)
	var asked int
	var where geom.Point
	w := newContextMenuTestWindow(t, &asked, &where)
	_, drift := DragGestureParameters()
	w.mouseDown(geom.NewPoint(10, 10), ButtonRight, 0)
	w.mouseDrag(geom.NewPoint(10+drift, 10), ButtonRight, 0)
	c.NotNil(w.contextMenuPanel, "a move within the drag threshold is still a click")
	w.mouseDrag(geom.NewPoint(10+drift*2, 10), ButtonRight, 0)
	c.Nil(w.contextMenuPanel, "a move beyond it is a drag")
	w.mouseUp(geom.NewPoint(10+drift*2, 10), ButtonRight, 0)
	c.Equal(0, asked, "a right-drag must not open the menu on its release")
}

// TestContextMenuNotOpenedBySynthesizedRelease verifies that a synthesized release opens no menu, including when the
// press handler itself ends the press.
func TestContextMenuNotOpenedBySynthesizedRelease(t *testing.T) {
	c := check.New(t)
	var asked int
	var where geom.Point
	w := newContextMenuTestWindow(t, &asked, &where)
	w.mouseDown(geom.NewPoint(10, 10), ButtonRight, 0)
	c.NotNil(w.contextMenuPanel, "a right press records whose menu its release is for")
	w.synthesizeMouseUp()
	c.Equal(0, asked, "a synthesized release must not open the menu")
	c.Nil(w.contextMenuPanel)
	c.False(w.inMouseDown)
	c.False(w.rightPressTaken)

	var handlerAsked int
	hw, owner, _ := newPressHandlerTestWindow(t, true, &handlerAsked)
	owner.Self = &cmEndingPressHandler{cmPressHandler: owner}
	hw.mouseDown(geom.NewPoint(40, 50), ButtonRight, 0)
	c.Nil(hw.contextMenuPanel, "a press the handler ended leaves nothing waiting")
	c.False(hw.inMouseDown)
	hw.mouseUp(geom.NewPoint(40, 50), ButtonRight, 0)
	c.Equal(0, handlerAsked)
}

// cmEndingPressHandler ends the press it is told of, as a modal opening from within the handler would.
type cmEndingPressHandler struct {
	*cmPressHandler
}

// ContextMenuPressed implements ContextMenuPressHandler.
func (p *cmEndingPressHandler) ContextMenuPressed(where geom.Point, mods mod.Modifiers) bool {
	p.cmPressHandler.ContextMenuPressed(where, mods)
	p.Window().synthesizeMouseUp()
	return true
}

// TestContextMenuNotOpenedWhenAnotherButtonIntervenes verifies that pressing another button with the right one, in
// either order, opens no menu and leaves the other's gesture intact, and a right press made second leaves no trace.
func TestContextMenuNotOpenedWhenAnotherButtonIntervenes(t *testing.T) {
	c := check.New(t)
	var asked int
	w, owner, log := newPressHandlerTestWindow(t, true, &asked)
	var windowDowns, windowDrags, windowUps []int
	w.MouseDownCallback = func(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
		windowDowns = append(windowDowns, button)
		return false
	}
	w.MouseDragCallback = func(_ geom.Point, button int, _ mod.Modifiers) bool {
		windowDrags = append(windowDrags, button)
		return false
	}
	w.MouseUpCallback = func(_ geom.Point, button int, _ mod.Modifiers) bool {
		windowUps = append(windowUps, button)
		return false
	}
	pt := geom.NewPoint(40, 50)
	near := geom.NewPoint(pt.X+3, pt.Y+3)
	far := geom.NewPoint(pt.X+20, pt.Y)
	farther := geom.NewPoint(pt.X+30, pt.Y)
	leftOnly := []int{ButtonLeft, ButtonLeft, ButtonLeft}

	// The right button, then the left, with the left let go first.
	w.mouseDown(pt, ButtonRight, 0)
	c.Equal(1, owner.presses)
	w.mouseDown(pt, ButtonLeft, 0)
	c.Nil(w.contextMenuPanel, "another press ends the right-click")
	c.Equal([]string{cmDown}, log.kinds(), "and is delivered as any other")
	w.mouseMovedOrDragged(far, 0)
	w.mouseUp(far, ButtonLeft, 0)
	c.Nil(w.lastMouseDownPanel, "the left release ends the left gesture, though the right button is still down")
	w.mouseMovedOrDragged(farther, 0)
	w.mouseUp(farther, ButtonRight, 0)
	c.Equal([]string{cmDown, cmDrag, cmUp}, log.kinds(),
		"nothing follows the left release, and the right release is not delivered, since its press was not")
	c.Equal(leftOnly, log.buttons(), "the left gesture comes with the left button throughout")
	c.Equal([]int{ButtonRight, ButtonLeft}, windowDowns, "the window's own callback is told of both presses")
	c.Equal([]int{ButtonLeft, ButtonLeft}, windowDrags, "and of both drags, with the button of the gesture under way")
	c.Equal([]int{ButtonLeft, ButtonRight}, windowUps, "and of both releases")
	c.Equal(0, asked)

	// The right button, then the left, with the right let go first.
	log.events, windowDowns, windowDrags, windowUps = nil, nil, nil, nil
	w.lastButtonTime = time.Time{}
	w.mouseDown(pt, ButtonRight, 0)
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseUp(pt, ButtonRight, 0)
	c.NotNil(w.lastMouseDownPanel, "the right release does not end the left gesture")
	w.mouseMovedOrDragged(far, 0)
	w.mouseUp(far, ButtonLeft, 0)
	c.Equal([]string{cmDown, cmDrag, cmUp}, log.kinds())
	c.Equal(leftOnly, log.buttons(), "nor does it change the button the left gesture's drags come with")
	c.Equal([]int{ButtonLeft}, windowDrags, "which the window's own callback is told of too")
	c.Nil(w.lastMouseDownPanel)
	c.Equal(0, asked)

	// The left button, as the second click of a double-click, then the right, with the left let go first.
	w.lastButtonTime = time.Time{}
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseUp(pt, ButtonLeft, 0)
	log.events, windowDowns, windowDrags, windowUps = nil, nil, nil, nil
	w.mouseDown(pt, ButtonLeft, 0)
	claimed := w.lastMouseDownPanel
	c.NotNil(claimed)
	c.Equal(2, w.lastButtonCount)
	when := w.lastButtonTime
	w.mouseDown(near, ButtonRight, 0)
	c.Equal(2, owner.presses, "a right press made while another button is down tells the handler nothing")
	c.Nil(w.contextMenuPanel, "and records nothing")
	c.True(w.lastMouseDownPanel == claimed, "the gesture under way keeps the panel that claimed it")
	c.Equal(ButtonLeft, w.lastButton, "and its button")
	c.Equal(2, w.lastButtonCount, "and its click count")
	c.Equal(pt, w.firstButtonLocation, "and where it began")
	c.True(w.lastButtonTime.Equal(when), "and when")
	c.Equal([]int{ButtonLeft}, windowDowns, "the window's own callback is not told of the right press")
	w.mouseMovedOrDragged(far, 0)
	w.mouseUp(far, ButtonLeft, 0)
	c.False(w.inMouseDown, "the left release ends the gesture, though the right button is still held")
	c.Nil(w.lastMouseDownPanel)
	w.mouseMovedOrDragged(farther, 0)
	w.mouseUp(farther, ButtonRight, 0)
	c.Equal([]string{cmDown, cmDrag, cmUp}, log.kinds(),
		"the left gesture is delivered whole, and nothing follows its release")
	c.Equal(leftOnly, log.buttons())
	if len(log.events) != 0 {
		c.Equal(2, log.events[0].count, "with the click count its press was given")
	}
	c.Equal([]int{ButtonLeft}, windowDrags,
		"the window's own callback is told of the drag, and not of the move that followed the release")
	c.Equal([]int{ButtonLeft}, windowUps, "nor is the window's own callback told of the right release")
	c.Equal(0, asked)

	// The left button, then the right, with the right let go first.
	log.events, windowDowns, windowDrags, windowUps = nil, nil, nil, nil
	w.lastButtonTime = time.Time{}
	w.mouseDown(pt, ButtonLeft, 0)
	claimed = w.lastMouseDownPanel
	w.mouseDown(near, ButtonRight, 0)
	w.mouseMovedOrDragged(far, 0)
	w.mouseUp(far, ButtonRight, 0)
	c.True(w.lastMouseDownPanel == claimed, "the right release neither ends the gesture")
	c.Equal(ButtonLeft, w.lastButton, "nor changes its button")
	w.mouseMovedOrDragged(farther, 0)
	w.mouseUp(farther, ButtonLeft, 0)
	c.Equal([]string{cmDown, cmDrag, cmDrag, cmUp}, log.kinds(), "the left gesture is delivered whole")
	c.Equal([]int{ButtonLeft, ButtonLeft, ButtonLeft, ButtonLeft}, log.buttons())
	c.Equal([]int{ButtonLeft}, windowDowns)
	c.Equal([]int{ButtonLeft, ButtonLeft}, windowDrags)
	c.Equal([]int{ButtonLeft}, windowUps)
	c.Nil(w.lastMouseDownPanel)
	c.Nil(w.contextMenuPanel)
	c.Equal(0, asked, "and nothing is left for the last release to open")
}

// TestContextMenuDoubleRightClick verifies that each press of a double right-click asks for the menu on its release
// without being delivered, and that a release stopped by the window's MouseUpCallback opens no menu.
func TestContextMenuDoubleRightClick(t *testing.T) {
	c := check.New(t)
	var asked int
	var where geom.Point
	w := newContextMenuTestWindow(t, &asked, &where)
	var log cmMouseLog
	log.logMouse(w.root.contentPanel)
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonRight, 0)
	w.mouseUp(pt, ButtonRight, 0)
	c.Equal(1, asked)
	w.mouseDown(pt, ButtonRight, 0)
	c.Equal(2, w.lastButtonCount)
	c.Equal(0, len(log.events), "the second press of a double-click is not delivered either")
	w.mouseUp(pt, ButtonRight, 0)
	c.Equal(2, asked, "and its release asks for the menu again")
	c.Equal(0, len(log.events))

	// Long enough ago that the next press starts a run of its own.
	w.lastButtonTime = time.Time{}
	w.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool { return true }
	w.mouseDown(pt, ButtonRight, 0)
	c.Equal(1, w.lastButtonCount)
	w.mouseUp(pt, ButtonRight, 0)
	c.Equal(2, asked, "a release the window's own callback stopped opens nothing")
	c.Nil(w.contextMenuPanel)
	c.False(w.rightPressTaken)
}

// TestIsContextMenuKey verifies that only the Menu key alone and shift+F10 ask for a contextual menu, whatever the
// state of caps lock and num lock.
func TestIsContextMenuKey(t *testing.T) {
	c := check.New(t)
	c.True(isContextMenuKey(KeyMenu, 0))
	c.True(isContextMenuKey(KeyMenu, mod.CapsLock|mod.NumLock))
	c.False(isContextMenuKey(KeyMenu, mod.Shift))
	c.False(isContextMenuKey(KeyMenu, mod.Control))
	c.True(isContextMenuKey(KeyF10, mod.Shift))
	c.True(isContextMenuKey(KeyF10, mod.Shift|mod.CapsLock))
	c.False(isContextMenuKey(KeyF10, 0))
	c.False(isContextMenuKey(KeyF10, mod.Shift|mod.Control))
	c.False(isContextMenuKey(KeyF10, mod.Option))
	c.False(isContextMenuKey(KeyF9, mod.Shift))
}

// TestOffersContextMenu verifies that a panel offers its menu only when it is enabled, has a callback and is not
// withholding, and that a menu of the panel around it is never its own.
func TestOffersContextMenu(t *testing.T) {
	c := check.New(t)
	outer := NewPanel()
	inner := newCMWithholdingPanel()
	outer.AddChild(inner)
	c.False(inner.offersContextMenu(), "nothing has a menu")
	outer.ContextMenuCallback = func(geom.Point) Menu { return nil }
	c.True(outer.offersContextMenu(), "a panel with a callback offers its menu")
	c.False(inner.offersContextMenu(), "but a menu of the panel around it is not the panel's own")
	inner.ContextMenuCallback = func(geom.Point) Menu { return nil }
	c.True(inner.offersContextMenu())
	inner.SetEnabled(false)
	c.False(inner.offersContextMenu(), "a disabled panel offers nothing")
	inner.SetEnabled(true)
	inner.withhold = true
	c.False(inner.offersContextMenu(), "nor does one withholding its menu")
	inner.withhold = false
	c.True(inner.offersContextMenu())
}

// cmWithholdingPanel is a panel with a menu it may withhold.
type cmWithholdingPanel struct {
	Panel
	withhold bool
}

func newCMWithholdingPanel() *cmWithholdingPanel {
	p := &cmWithholdingPanel{}
	p.Self = p
	return p
}

// WithholdsContextMenu implements ContextMenuWithholder.
func (p *cmWithholdingPanel) WithholdsContextMenu() bool {
	return p.withhold
}

// TestContextMenuSlowRightClick verifies that a right-click held longer than IsDragGesture allows still opens the menu,
// since only the pointer's drift makes a right press a drag.
func TestContextMenuSlowRightClick(t *testing.T) {
	c := check.New(t)
	var asked int
	var where geom.Point
	w := newContextMenuTestWindow(t, &asked, &where)
	_, drift := DragGestureParameters()
	pt := geom.NewPoint(10, 10)
	within := geom.NewPoint(pt.X+drift, pt.Y)
	w.mouseDown(pt, ButtonRight, 0)
	w.lastButtonTime = time.Now().Add(-time.Second)
	c.True(w.IsDragGesture(within), "the test needs the button held long enough for IsDragGesture to see a drag")
	w.mouseMovedOrDragged(within, 0)
	c.NotNil(w.contextMenuPanel, "a move within the drag threshold is still a click, however long the button is held")
	w.mouseUp(within, ButtonRight, 0)
	c.Equal(1, asked, "a slow right-click opens the menu")
	c.Equal(within, where)
}

// TestContextMenuDragNotHandedBackToAChangedOwner verifies that a right press taken for a menu, which the owner asked
// to have back should it become a drag, is delivered to no panel once it does when the owner has been removed, disabled
// or moved from under the press in the meantime, since the press would then reach a panel that was never offered it.
func TestContextMenuDragNotHandedBackToAChangedOwner(t *testing.T) {
	for _, one := range []struct {
		change func(owner *cmPressHandler)
		name   string
	}{
		{name: "disabled", change: func(owner *cmPressHandler) { owner.SetEnabled(false) }},
		{name: "removed", change: func(owner *cmPressHandler) { owner.RemoveFromParent() }},
		{name: "moved", change: func(owner *cmPressHandler) { owner.SetFrameRect(geom.NewRect(150, 150, 40, 40)) }},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var asked int
			w, owner, log := newPressHandlerTestWindow(t, true, &asked)
			var parentLog cmMouseLog
			parentLog.logMouse(w.root.contentPanel)
			_, drift := DragGestureParameters()
			pt := geom.NewPoint(40, 50)
			far := geom.NewPoint(pt.X+drift*2, pt.Y)
			w.mouseDown(pt, ButtonRight, 0)
			c.True(w.contextMenuPanel == owner.AsPanel(), "the press is for the owner's menu")
			one.change(owner)
			w.mouseDrag(far, ButtonRight, 0)
			c.Nil(w.contextMenuPanel, "the drag ends the right-click")
			c.Nil(w.lastMouseDownPanel, "and hands the press to no panel")
			w.mouseUp(far, ButtonRight, 0)
			c.Equal(0, len(log.events), "the owner is sent nothing")
			c.Equal(0, len(parentLog.events), "and neither is the panel the press now lands on")
			c.Equal(0, asked)
		})
	}
}

// TestContextMenuNotOpenedForAChangedOwner verifies that a right-click opens nothing when its owner is disabled,
// removed, or moved into another window between the press and the release.
func TestContextMenuNotOpenedForAChangedOwner(t *testing.T) {
	for _, one := range []struct {
		change func(w *Window, owner *cmPressHandler)
		name   string
	}{
		{name: "disabled", change: func(_ *Window, owner *cmPressHandler) { owner.SetEnabled(false) }},
		{name: "removed", change: func(_ *Window, owner *cmPressHandler) { owner.RemoveFromParent() }},
		{
			name: "moved",
			change: func(w *Window, owner *cmPressHandler) {
				// Into the now-active window, under the release point: only mouseUp's window check stops the menu.
				other := newMouseButtonTestWindow()
				other.root.contentPanel.AddChild(owner)
				windowList = []*Window{other, w}
			},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var asked int
			w, owner, _ := newPressHandlerTestWindow(t, false, &asked)
			pt := geom.NewPoint(40, 50)
			w.mouseDown(pt, ButtonRight, 0)
			c.True(w.contextMenuPanel == owner.AsPanel(), "the press is for the owner's menu")
			one.change(w, owner)
			w.mouseUp(pt, ButtonRight, 0)
			c.Equal(0, asked, "the release must not open the menu")
			c.Nil(w.contextMenuPanel)
		})
	}
}

// newContainedPressHandlerTestWindow returns the active window holding container, a panel at (10, 10) of 150 by 150,
// which in turn holds owner, a cmPressHandler at (10, 20) of 100 by 100 within the container, whose menu callback
// counts its calls in asked and returns nil, and whose mouse events go to log. A press at (40, 50) lands within the
// owner.
func newContainedPressHandlerTestWindow(t *testing.T, want bool,
	asked *int,
) (w *Window, container *Panel, owner *cmPressHandler, log *cmMouseLog) {
	t.Helper()
	w = newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	container = NewPanel()
	container.SetFrameRect(geom.NewRect(10, 10, 150, 150))
	w.root.contentPanel.AddChild(container)
	owner = newCMPressHandler(want)
	owner.SetFrameRect(geom.NewRect(10, 20, 100, 100))
	owner.ContextMenuCallback = func(_ geom.Point) Menu {
		*asked++
		return nil
	}
	container.AddChild(owner)
	log = &cmMouseLog{}
	log.logMouse(owner.AsPanel())
	return w, container, owner, log
}

// cmContainerChanges are the ways a container between a menu's owner and the panel the window finds under a position
// can stop showing the owner there while the owner's own frame still holds the position: being hidden, and shrinking
// so that the position falls outside it.
var cmContainerChanges = []struct {
	change func(container *Panel)
	name   string
}{
	{name: "hidden", change: func(container *Panel) { container.Hidden = true }},
	{name: "shrunk", change: func(container *Panel) { container.SetFrameRect(geom.NewRect(10, 10, 25, 25)) }},
}

// TestContextMenuDragNotHandedBackThroughAChangedContainer verifies that a right press taken for a menu, which the
// owner asked to have back should it become a drag, is delivered to no panel once it does when a container between the
// owner and the panel under the press was hidden or shrunk from under the press in the meantime, though the owner's own
// frame still holds the press and the owner is still enabled.
func TestContextMenuDragNotHandedBackThroughAChangedContainer(t *testing.T) {
	for _, one := range cmContainerChanges {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var asked int
			w, container, owner, log := newContainedPressHandlerTestWindow(t, true, &asked)
			var parentLog cmMouseLog
			parentLog.logMouse(w.root.contentPanel)
			_, drift := DragGestureParameters()
			pt := geom.NewPoint(40, 50)
			far := geom.NewPoint(pt.X+drift*2, pt.Y)
			w.mouseDown(pt, ButtonRight, 0)
			c.True(w.contextMenuPanel == owner.AsPanel(), "the press is for the owner's menu")
			one.change(container)
			c.True(owner.Enabled(), "the test needs the owner itself enabled")
			c.True(owner.PointFromRoot(pt).In(owner.ContentRect(true)), "with its own frame still holding the press")
			c.True(w.root.PanelAt(pt) == w.root.contentPanel, "and the window finding the container's parent under it")
			w.mouseDrag(far, ButtonRight, 0)
			c.Nil(w.contextMenuPanel, "the drag ends the right-click")
			c.Nil(w.lastMouseDownPanel, "and hands the press to no panel")
			w.mouseUp(far, ButtonRight, 0)
			c.Equal(0, len(log.events), "the owner is sent nothing")
			c.Equal(0, len(parentLog.events), "and neither is the panel the press now lands on")
			c.Equal(0, asked)
		})
	}
}

// TestContextMenuNotOpenedThroughAChangedContainer verifies that a right-click opens nothing when a container between
// its owner and the panel the window finds under the release was hidden or shrunk from under the release between the
// press and the release, though the owner's own frame still holds the release and the owner is still enabled.
func TestContextMenuNotOpenedThroughAChangedContainer(t *testing.T) {
	for _, one := range cmContainerChanges {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var asked int
			w, container, owner, _ := newContainedPressHandlerTestWindow(t, false, &asked)
			pt := geom.NewPoint(40, 50)
			w.mouseDown(pt, ButtonRight, 0)
			c.True(w.contextMenuPanel == owner.AsPanel(), "the press is for the owner's menu")
			one.change(container)
			c.True(owner.Enabled(), "the test needs the owner itself enabled")
			c.True(owner.PointFromRoot(pt).In(owner.ContentRect(true)), "with its own frame still holding the release")
			w.mouseUp(pt, ButtonRight, 0)
			c.Equal(0, asked, "the release must not open the menu")
			c.Nil(w.contextMenuPanel)
		})
	}
}

// cmCountingMenu is a one-item in-window menu that counts its popups. When native, Popup shows nothing, so that, as
// with a native menu, nothing is left open afterward.
type cmCountingMenu struct {
	cmInnerMenu
	popups *int
	native bool
}

// cmInnerMenu lets cmCountingMenu embed a Menu without the embedded field's name hiding the Menu.Menu method.
type cmInnerMenu = Menu

func newCMCountingMenu(popups *int, native bool) *cmCountingMenu {
	f := NewInWindowMenuFactory()
	m := f.NewMenu(PopupMenuTemporaryBaseID|ContextMenuIDFlag, "", nil)
	m.InsertItem(-1, f.NewItem(PopupMenuTemporaryBaseID+1, "Item", KeyBinding{}, nil, func(MenuItem) {}))
	return &cmCountingMenu{cmInnerMenu: m, popups: popups, native: native}
}

// Popup implements Menu.
func (m *cmCountingMenu) Popup(where geom.Rect, itemIndex int) {
	*m.popups++
	if !m.native {
		m.cmInnerMenu.Popup(where, itemIndex)
	}
}

// newKeyMenuTestWindow returns the active window holding a focused panel whose menu is a cmCountingMenu and whose key
// callbacks count into keysDown and keysUp.
func newKeyMenuTestWindow(t *testing.T, popups, keysDown, keysUp *int, native bool) *Window {
	t.Helper()
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	owner := NewPanel()
	owner.SetFrameRect(geom.NewRect(10, 10, 50, 50))
	owner.SetFocusable(true)
	owner.ContextMenuCallback = func(geom.Point) Menu { return newCMCountingMenu(popups, native) }
	owner.KeyDownCallback = func(KeyCode, mod.Modifiers, bool) bool {
		*keysDown++
		return true
	}
	owner.KeyUpCallback = func(KeyCode, mod.Modifiers) bool {
		*keysUp++
		return true
	}
	w.root.contentPanel.AddChild(owner)
	owner.RequestFocus()
	return w
}

// TestContextMenuKeyAfterANativeMenu verifies that, although a native menu's tracking loop swallows the chord's key up,
// the next press still opens the menu rather than counting as a repeat, and a late key up is not delivered.
func TestContextMenuKeyAfterANativeMenu(t *testing.T) {
	c := check.New(t)
	var popups, keysDown, keysUp int
	w := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, true)
	c.NotNil(w.CurrentFocus())
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(1, popups)
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(2, popups, "shift+F10 opens the menu again, though the window never heard the key go up")
	w.keyPressed(KeyMenu, 0)
	w.keyPressed(KeyMenu, 0)
	c.Equal(4, popups, "and so does the Menu key")
	w.keyReleased(KeyMenu, 0)
	c.Equal(0, keysDown, "the panel is sent none of the key downs")
	c.Equal(0, keysUp, "nor a key up that arrives late after all")
	c.Equal(0, len(w.pressedKeys))
}

// TestContextMenuKeyHeldOverAnInWindowMenu verifies that a held Menu key opens an in-window menu only once, even after
// the menu closes, and that the next press after the key up opens it again.
func TestContextMenuKeyHeldOverAnInWindowMenu(t *testing.T) {
	c := check.New(t)
	var popups, keysDown, keysUp int
	w := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, false)
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, popups)
	c.Equal(1, len(w.root.openMenuPanels), "the in-window menu is still open")
	w.root.closeMenuStackStoppingAt(nil)
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, popups, "a repeat does not open the menu again")
	c.Equal(1, keysDown, "and is passed on to the panel holding the focus as one")
	w.keyReleased(KeyMenu, 0)
	w.keyPressed(KeyMenu, 0)
	c.Equal(2, popups, "the next press after the key was let go opens it again")
	w.root.closeMenuStackStoppingAt(nil)
}

// TestShowContextMenuOnlyInTheActiveWindow verifies that a menu is shown, and the callback asked, only for a panel in
// the active window, and that the Menu key in a background window goes to the focused panel instead.
func TestShowContextMenuOnlyInTheActiveWindow(t *testing.T) {
	c := check.New(t)
	front := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	activateForContextMenu(t, front, back)
	var asked, popups int
	callback := func(geom.Point) Menu {
		asked++
		return newCMCountingMenu(&popups, true)
	}
	inFront := front.root.contentPanel
	inBack := back.root.contentPanel
	inFront.ContextMenuCallback = callback
	inBack.ContextMenuCallback = callback
	pt := geom.NewPoint(10, 10)
	c.True(inFront.ShowContextMenu(pt), "a panel in the active window shows its menu")
	c.Equal(1, asked)
	c.Equal(1, popups)
	c.False(inBack.ShowContextMenu(pt), "a panel in a background window shows nothing")
	c.Equal(1, asked, "and its callback is not asked for a menu")

	var keys int
	focusable := NewPanel()
	focusable.SetFrameRect(geom.NewRect(50, 50, 20, 20))
	focusable.SetFocusable(true)
	focusable.ContextMenuCallback = callback
	focusable.KeyDownCallback = func(KeyCode, mod.Modifiers, bool) bool {
		keys++
		return true
	}
	inBack.AddChild(focusable)
	focusable.RequestFocus()
	back.keyPressed(KeyMenu, 0)
	c.Equal(1, asked, "the Menu key in a background window asks for no menu")
	c.Equal(1, keys, "and is passed on to the panel holding the focus")

	front.transient = true
	c.True(ActiveWindow() == back, "the test needs the transient window passed over")
	c.False(inFront.ShowContextMenu(pt), "a panel in a transient window shows nothing")
	var log cmMouseLog
	log.logMouse(inFront)
	front.mouseDown(pt, ButtonRight, 0)
	c.Nil(front.contextMenuPanel, "a right press in a transient window is not taken for a menu that could not open")
	front.mouseUp(pt, ButtonRight, 0)
	c.Equal(1, asked, "so a right-click there asks for no menu")
	c.Equal([]string{cmDown, cmUp}, log.kinds(), "and is delivered as an ordinary press")

	// The Menu key with nothing holding the focus opens nothing and reaches nothing.
	back.SetFocus(nil)
	c.Nil(back.CurrentFocus(), "the test needs nothing focused")
	back.keyPressed(KeyMenu, 0)
	c.Equal(1, asked, "the Menu key with no focus asks for no menu")
	c.Equal(1, keys, "and reaches no panel")

	front.transient = false
	front.focused = false
	back.focused = false
	c.Nil(ActiveWindow(), "the test needs no window active")
	c.False(inFront.ShowContextMenu(pt), "no panel shows its menu while no window is active")
	c.Equal(1, asked)
	c.Equal(1, popups)
}

// TestContextMenuWindowCallbackNeverCalled verifies that a Window's own ContextMenuCallback, which it has by embedding
// InputCallbacks, is never called: a right-click on its content panel, the Menu key while the content panel holds the
// focus and an assistive technology's request for the content panel's menu all find no menu.
func TestContextMenuWindowCallbackNeverCalled(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var asked int
	w.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return newCMCountingMenu(&asked, true)
	}
	content := w.root.contentPanel
	content.SetFocusable(true)
	var log cmMouseLog
	log.logMouse(content)
	var keys int
	content.KeyDownCallback = func(KeyCode, mod.Modifiers, bool) bool {
		keys++
		return true
	}
	content.RequestFocus()
	c.True(w.CurrentFocus() == content)
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonRight, 0)
	c.Nil(w.contextMenuPanel, "a right press on the content panel is not taken for the window's menu")
	w.mouseUp(pt, ButtonRight, 0)
	c.Equal([]string{cmDown, cmUp}, log.kinds(), "and is delivered as an ordinary press")
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, keys, "the Menu key goes on to the content panel")
	c.False(content.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu}, false),
		"an assistive technology's request for the content panel's menu is refused")
	c.Equal(0, asked, "the window's own callback is never asked")
}

// TestContextMenuAccessibilityRefusedWhereItCannotOpen verifies that an assistive technology's menu request for a
// disabled panel, or for a panel, list row or table row or cell in a background window, is refused without asking the
// callback or moving the focus or selection, and is carried out once the window is active.
func TestContextMenuAccessibilityRefusedWhereItCannotOpen(t *testing.T) {
	c := check.New(t)
	front := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	activateForContextMenu(t, front, back)
	var asked, popups int
	callback := func(geom.Point) Menu {
		asked++
		return newCMCountingMenu(&popups, true)
	}
	addFocusable := func(w *Window, rect geom.Rect) *Panel {
		p := NewPanel()
		p.SetFrameRect(rect)
		p.SetFocusable(true)
		w.root.contentPanel.AddChild(p)
		return p
	}
	holder := addFocusable(back, geom.NewRect(0, 0, 20, 20))
	target := addFocusable(back, geom.NewRect(30, 0, 20, 20))
	target.ContextMenuCallback = callback
	list := NewList[string]()
	list.Append("Zero", "One", "Two")
	list.SetFrameRect(geom.NewRect(0, 30, 100, 60))
	list.ContextMenuCallback = callback
	list.Select(false, 0)
	var listChanges int
	list.NewSelectionCallback = func() { listChanges++ }
	back.root.contentPanel.AddChild(list)
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows([]*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0), newFocusCellRow("c", 0)})
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}, {ID: 2, Current: 50}}
	table.SetFrameRect(geom.NewRect(0, 100, 250, 60))
	back.root.contentPanel.AddChild(table)
	table.SyncToModel()
	table.SelectByIndex(0)
	table.ContextMenuCallback = callback
	var tableChanges int
	table.SelectionChangedCallback = func() { tableChanges++ }
	holder.RequestFocus()
	c.True(back.focus == holder, "the test needs the focus to start out elsewhere")
	c.True(ActiveWindow() == front, "the test needs the panels' window in the background")

	ask := func(key any) accessibility.ActionRequest {
		return accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: key}
	}
	c.False(target.axDispatchAction(ask(nil), false), "a panel in a background window refuses to open its menu")
	c.False(list.axDispatchAction(ask(2), true), "and so does a row of a list there")
	c.False(table.axDispatchAction(ask(tid.TID("c")), true), "and a row of a table")
	c.False(table.axDispatchAction(ask(accessibility.CellKey{Row: tid.TID("c"), Col: 1}), true), "and one of its cells")
	c.Equal(0, asked, "no callback is asked for a menu")
	c.True(back.focus == holder, "and the focus is left where it was")
	c.True(list.Selection.State(0) && !list.Selection.State(2), "the list's selection is left as it was")
	c.Equal(0, listChanges, "and the application is not told it changed")
	c.True(table.IsRowSelected(0) && !table.IsRowSelected(2), "the table's selection is left as it was")
	c.Equal(-1, table.LeadColumnIndex(), "as is its cell cursor")
	c.Equal(0, tableChanges, "and the application is not told it changed")

	frontHolder := addFocusable(front, geom.NewRect(0, 0, 20, 20))
	disabled := addFocusable(front, geom.NewRect(30, 0, 20, 20))
	disabled.ContextMenuCallback = callback
	disabled.SetEnabled(false)
	frontHolder.RequestFocus()
	c.False(disabled.axDispatchAction(ask(nil), false), "a disabled panel refuses to open its menu")
	c.Equal(0, asked, "and its callback is not asked for one")
	c.True(front.focus == frontHolder)

	activateForContextMenu(t, back, front)
	c.True(target.axDispatchAction(ask(nil), false), "the panel opens its menu once its window is the active one")
	c.Equal(1, asked)
	c.True(back.focus == target, "taking the focus first")
	c.True(list.axDispatchAction(ask(2), true), "and so does the list's row")
	c.True(list.Selection.State(2), "selecting it")
	c.Equal(1, listChanges)
	c.True(back.focus == list.AsPanel())
	c.True(table.axDispatchAction(ask(accessibility.CellKey{Row: tid.TID("c"), Col: 1}), true), "and the table's cell")
	c.True(table.IsRowSelected(2), "selecting its row")
	c.Equal(1, table.LeadColumnIndex())
	c.Equal(1, tableChanges)
	c.True(back.focus == table.AsPanel())
	c.Equal(3, asked)
	c.Equal(3, popups)
}

// TestContextMenuNotOpenedWhenAnotherButtonIntervenesOverATableCell verifies that a right press made while another
// button is down, over a widget with a menu in a Table cell, changes nothing and leaves the gesture under way intact.
func TestContextMenuNotOpenedWhenAnotherButtonIntervenesOverATableCell(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	rows := []*focusCellRow{newFocusCellRow("a", 1), newFocusCellRow("b", 1), newFocusCellRow("c", 1)}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 80}, {ID: 1, Current: 60}, {ID: 2, Current: 40}}
	table.SetFrameRect(geom.NewRect(0, 0, 200, 120))
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	// The first column's cells stand in for a Field: focusable, claiming a press, and offering a menu of their own.
	var asked int
	for _, row := range rows {
		row.ColumnCell(0, 0, nil, nil, false, false, false).AsPanel().ContextMenuCallback = func(geom.Point) Menu {
			asked++
			return nil
		}
	}
	var kinds []string
	var buttons []int
	down, drag, up := table.MouseDownCallback, table.MouseDragCallback, table.MouseUpCallback
	table.MouseDownCallback = func(where geom.Point, button, count int, mods mod.Modifiers) bool {
		kinds = append(kinds, cmDown)
		buttons = append(buttons, button)
		return down(where, button, count, mods)
	}
	table.MouseDragCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
		kinds = append(kinds, cmDrag)
		buttons = append(buttons, button)
		return drag(where, button, mods)
	}
	table.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
		kinds = append(kinds, cmUp)
		buttons = append(buttons, button)
		return up(where, button, mods)
	}
	button := NewButton()
	button.SetTitle("Press")
	button.SetFrameRect(geom.NewRect(0, 130, 100, 40))
	var clicks int
	button.ClickCallback = func() { clicks++ }
	w.root.contentPanel.AddChild(button)
	// A release updates the cursor, and the table's and button's defaults would create a platform cursor; with them
	// nil, the content panel's stand-in answers.
	table.UpdateCursorCallback = nil
	button.UpdateCursorCallback = nil
	w.SetFocus(table)
	c.True(w.CurrentFocus() == table.AsPanel())

	// The table is at the content's origin, so its coordinates are the window's.
	plain := table.CellFrame(0, 2).Center()
	field := table.CellFrame(1, 0).Center()
	far := geom.NewPoint(plain.X+20, plain.Y)
	w.mouseDown(plain, ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == table.AsPanel(), "the left press is the table's")
	c.True(table.IsRowSelected(0) && !table.IsRowSelected(1), "and selects the row it landed on")
	w.mouseDown(field, ButtonRight, 0)
	c.Nil(w.contextMenuPanel,
		"a right press over a widget with a menu made while the left button is down asks for no menu")
	c.False(w.pressedButtons[ButtonRight], "and is not counted among the buttons down")
	c.Equal(ButtonLeft, w.lastButton)
	c.True(w.lastMouseDownPanel == table.AsPanel(), "the left gesture keeps the table")
	c.True(w.CurrentFocus() == table.AsPanel(), "the widget in the cell is not given the focus")
	c.True(rows[1].cells[0].Parent() == nil, "and its cell is not kept attached")
	c.True(table.IsRowSelected(0) && !table.IsRowSelected(1), "the selection is left as the left press made it")
	c.Equal(-1, table.LeadColumnIndex(), "and so is the cell cursor")
	w.mouseMovedOrDragged(far, 0)
	w.mouseUp(far, ButtonLeft, 0)
	w.mouseUp(far, ButtonRight, 0)
	c.Equal([]string{cmDown, cmDrag, cmUp}, kinds, "the left gesture is delivered whole, and nothing else is")
	c.Equal([]int{ButtonLeft, ButtonLeft, ButtonLeft}, buttons, "with the left button throughout")
	c.Equal(0, asked, "and no menu is asked for")
	c.True(w.CurrentFocus() == table.AsPanel())

	// A press on the button, interrupted by a right press over the field, still clicks the button.
	kinds, buttons = nil, nil
	w.lastButtonTime = time.Time{}
	onButton := geom.NewPoint(50, 150)
	w.mouseDown(onButton, ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == button.AsPanel())
	w.mouseDown(field, ButtonRight, 0)
	c.True(w.lastMouseDownPanel == button.AsPanel(), "the right press leaves the button its press")
	w.mouseUp(onButton, ButtonLeft, 0)
	c.Equal(1, clicks, "and the button is clicked on the left release")
	w.mouseUp(field, ButtonRight, 0)
	c.Equal(0, len(kinds), "the table is sent nothing")
	c.Equal(0, asked)
	c.True(w.CurrentFocus() == table.AsPanel())

	// A right-click on the widget in a disabled table never consults the table for the widget's menu: the press is
	// delivered to nothing, as any press on a disabled panel is, and the widget is neither asked nor focused.
	kinds, buttons = nil, nil
	w.lastButtonTime = time.Time{}
	w.SetFocus(button)
	table.SetEnabled(false)
	w.mouseDown(field, ButtonRight, 0)
	c.Nil(w.contextMenuPanel, "a right press on a widget in a disabled table is not taken for a menu")
	c.Nil(w.lastMouseDownPanel, "nor claimed by anything")
	c.True(w.CurrentFocus() == button.AsPanel(), "the widget in the cell is not given the focus")
	c.True(rows[1].cells[0].Parent() == nil, "and its cell is not kept attached")
	w.mouseUp(field, ButtonRight, 0)
	c.Equal(0, len(kinds), "the table is sent nothing")
	c.Equal(0, asked, "and the widget is not asked for its menu")
	table.SetEnabled(true)

	// A request for the widget's menu made while a left press is held on a selected row has the release delivered
	// before the widget's row is selected, so that the release's narrowing of the selection to the pressed row does not
	// undo the selection of the widget's row.
	w.SetFocus(table)
	table.SelectByIndex(0)
	w.mouseDown(plain, ButtonLeft, 0)
	c.True(table.IsRowSelected(0))
	c.True(w.inMouseDown)
	key := axCellPanelKey{Cell: accessibility.CellKey{Row: tid.TID("b"), Col: 0}, Path: ""}
	c.False(table.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: key}, true),
		"the widget's callback offers nothing, so nothing is shown")
	c.Equal(1, asked, "though the widget was asked")
	c.False(w.inMouseDown, "the press was ended first")
	c.True(table.IsRowSelected(1) && !table.IsRowSelected(0),
		"and the widget's row is the selection, the release having been delivered before it was selected")
	c.True(w.CurrentFocus() == rows[1].cells[0], "the widget holds the focus")
}

// TestContextMenuRowRequestAfterTheReleaseChangesTheRows verifies that a request for a table row's menu made while a
// button is held looks the row up again once the release has been delivered, since the release may change the rows: a
// row it removed is refused, and one it moved is still the one selected.
func TestContextMenuRowRequestAfterTheReleaseChangesTheRows(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	rows := []*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0), newFocusCellRow("c", 0)}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 80}, {ID: 1, Current: 60}, {ID: 2, Current: 40}}
	table.SetFrameRect(geom.NewRect(0, 0, 200, 120))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	var asked int
	table.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	// A panel whose release drops the first row.
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(0, 130, 100, 40))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	holder.MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool {
		model.SetRootRows(rows[1:])
		table.SyncToModel()
		return true
	}
	w.root.contentPanel.AddChild(holder)
	onHolder := geom.NewPoint(50, 150)
	ask := func(id string, col int) accessibility.ActionRequest {
		return accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: accessibility.CellKey{
			Row: tid.TID(id), Col: col,
		}}
	}

	w.mouseDown(onHolder, ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == holder)
	c.False(table.axDispatchAction(ask("a", 1), true), "a row the release removed is refused")
	c.False(w.inMouseDown, "the press was ended")
	c.Equal(2, table.LastRowIndex()+1, "and the rows are those the release left")
	c.Equal(0, asked, "the table is not asked for a menu")
	c.False(table.HasSelection(), "and nothing is selected")

	model.SetRootRows(rows)
	table.SyncToModel()
	w.lastButtonTime = time.Time{}
	w.mouseDown(onHolder, ButtonLeft, 0)
	c.False(table.axDispatchAction(ask("c", 1), true), "the callback returns nil, so no menu is shown")
	c.Equal(1, asked, "but a row the release moved is found again: the table is asked for its menu")
	c.Equal(2, table.LastRowIndex()+1)
	c.True(table.IsRowSelected(1), "and the row asked for is selected, at the index the release left it at")
	c.Equal(1, table.LeadColumnIndex())
}

// TestContextMenuKeyWhileAButtonIsHeld verifies that the chord or an assistive technology's request made while a button
// is held ends the press with a release before the menu opens, so no later move drags or hands a press back, and that
// the release is delivered outside every panel, so that a button held down is let go of without being clicked.
func TestContextMenuKeyWhileAButtonIsHeld(t *testing.T) {
	c := check.New(t)
	var popups, keysDown, keysUp int
	w := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, true)
	owner := w.CurrentFocus()
	c.NotNil(owner)
	log := &cmMouseLog{}
	log.logMouse(owner)
	pt := geom.NewPoint(20, 20)
	far := geom.NewPoint(pt.X+30, pt.Y)

	// A left press in progress when shift+F10 is pressed.
	w.mouseDown(pt, ButtonLeft, 0)
	c.Equal([]string{cmDown}, log.kinds())
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(1, popups, "the chord opens the menu")
	c.Equal([]string{cmDown, cmUp}, log.kinds(),
		"once the press has been ended, with a release to the panel that claimed it")
	if len(log.events) == 2 {
		c.Equal(owner.PointFromRoot(offPanelPoint), log.events[1].where, "delivered outside every panel")
	}
	c.False(w.inMouseDown, "and no button is left down")
	c.Equal(0, len(w.pressedButtons))
	w.mouseMovedOrDragged(far, 0)
	c.Equal([]string{cmDown, cmUp}, log.kinds(), "so a later move is not a drag")
	c.Equal(0, keysDown, "the owner is sent neither the chord")
	w.keyReleased(KeyF10, mod.Shift)
	c.Equal(0, keysUp, "nor its key up")

	// A right press taken for the menu, still waiting for its release, when the Menu key is pressed.
	log.events = nil
	w.mouseDown(pt, ButtonRight, 0)
	c.True(w.contextMenuPanel == owner, "the right press is taken for the menu")
	c.Equal(0, len(log.events), "and delivered to no panel")
	w.keyPressed(KeyMenu, 0)
	c.Equal(2, popups, "the Menu key opens the menu")
	c.Nil(w.contextMenuPanel, "and the right-click that was waiting is forgotten")
	c.False(w.inMouseDown)
	c.False(w.rightPressTaken)
	c.Equal(0, len(log.events), "the release of a press that was never delivered is not delivered either")
	w.mouseMovedOrDragged(far, 0)
	c.Equal(0, len(log.events), "and a later move does not hand the press back")
	c.Equal(2, popups, "nor open the menu again")

	// The same for an assistive technology's request.
	log.events = nil
	w.mouseDown(pt, ButtonLeft, 0)
	c.True(owner.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu}, false))
	c.Equal(3, popups)
	c.Equal([]string{cmDown, cmUp}, log.kinds(), "the press is ended before the menu opens")
	c.False(w.inMouseDown)
	w.mouseMovedOrDragged(far, 0)
	c.Equal([]string{cmDown, cmUp}, log.kinds())
	c.Equal(0, keysDown)
	c.Equal(0, keysUp)

	// A button held down when the chord is pressed is let go of, drawing itself released, but not clicked, since the
	// person never released it over the button.
	button := NewButton()
	button.SetTitle("Press")
	button.SetFrameRect(geom.NewRect(100, 100, 80, 40))
	button.UpdateCursorCallback = nil
	var clicks int
	button.ClickCallback = func() { clicks++ }
	w.root.contentPanel.AddChild(button)
	w.mouseDown(geom.NewPoint(140, 120), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == button.AsPanel(), "the test needs the button to hold the press")
	c.True(button.Pressed, "and to be drawn pressed")
	w.keyPressed(KeyMenu, 0)
	c.Equal(4, popups, "the chord opens the focused panel's menu")
	c.False(w.inMouseDown, "the button's press was ended")
	c.False(button.Pressed, "and the button is no longer drawn pressed")
	c.Equal(0, clicks, "but it was not clicked")
}

// TestContextMenuRowRequestEndsAHeldPress verifies that an assistive technology's request for the menu of a table row
// or a list row made while a button is held ends the press before the menu opens, as a request for a panel's menu does.
func TestContextMenuRowRequestEndsAHeldPress(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var asked, popups int
	callback := func(geom.Point) Menu {
		asked++
		return newCMCountingMenu(&popups, true)
	}
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(0, 0, 20, 20))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	w.root.contentPanel.AddChild(holder)
	list := NewList[string]()
	list.Append("Zero", "One", "Two")
	list.SetFrameRect(geom.NewRect(0, 30, 100, 60))
	list.ContextMenuCallback = callback
	list.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(list)
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows([]*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0)})
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
	table.SetFrameRect(geom.NewRect(0, 100, 200, 60))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	table.ContextMenuCallback = callback
	onHolder := geom.NewPoint(10, 10)

	w.mouseDown(onHolder, ButtonLeft, 0)
	c.True(w.inMouseDown)
	c.True(list.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: 1}, true))
	c.Equal(1, popups, "the list row's menu opens")
	c.False(w.inMouseDown, "once the held press has been ended")
	c.Equal(0, len(w.pressedButtons))

	w.lastButtonTime = time.Time{}
	w.mouseDown(onHolder, ButtonLeft, 0)
	c.True(w.inMouseDown)
	c.True(table.axDispatchAction(accessibility.ActionRequest{
		Action: accessibility.ShowContextMenu, Key: tid.TID("b"),
	}, true))
	c.Equal(2, popups, "the table row's menu opens")
	c.False(w.inMouseDown, "once the held press has been ended")
	c.Equal(0, len(w.pressedButtons))
	c.Equal(2, asked)
}

// TestContextMenuKeyUpNotSentToTheOwner verifies that the owner of a menu opened by the chord is sent no key up for the
// chord or for the keys that drive and close an in-window menu, since it was sent none of their key downs.
func TestContextMenuKeyUpNotSentToTheOwner(t *testing.T) {
	c := check.New(t)
	var popups, keysDown, keysUp int
	w := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, false)
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, popups)
	c.Equal(1, len(w.root.openMenuPanels), "the in-window menu is open")
	w.keyReleased(KeyMenu, 0)
	c.Equal(0, keysUp, "the key up of the chord is not sent to the owner")
	w.keyPressed(KeyDown, 0)
	w.keyReleased(KeyDown, 0)
	c.Equal(1, len(w.root.openMenuPanels), "the menu takes the keys that drive it")
	w.keyPressed(KeyEscape, 0)
	c.Equal(0, len(w.root.openMenuPanels), "and Escape closes it")
	w.keyReleased(KeyEscape, 0)
	c.Equal(0, keysDown, "the owner saw none of the key downs")
	c.Equal(0, keysUp, "and is sent none of the key ups, that of the key that closed the menu included")

	// A chord whose menu was closed some other way while the key was still down.
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(2, popups)
	w.root.closeMenuStackStoppingAt(nil)
	w.keyReleased(KeyF10, mod.Shift)
	c.Equal(0, keysUp, "the key up of the chord is not sent to the owner then either")
	c.Equal(0, keysDown)
}

// cmWithholdingTable and cmWithholdingList stand in for a Table and a List whose widget type implements
// ContextMenuWithholder, which the tests install as the widget's Self.
type cmWithholdingTable struct {
	*Table[*focusCellRow]
	withhold bool
}

// WithholdsContextMenu implements ContextMenuWithholder.
func (t *cmWithholdingTable) WithholdsContextMenu() bool { return t.withhold }

type cmWithholdingList struct {
	*List[string]
	withhold bool
}

// WithholdsContextMenu implements ContextMenuWithholder.
func (l *cmWithholdingList) WithholdsContextMenu() bool { return l.withhold }

// TestContextMenuWithholdingRows verifies that the rows of a Table or a List whose type withholds its menu do not offer
// the menu to an assistive technology, refuse a request made against a description from before the withholding began,
// and take a right-click as an ordinary press, until the widget stops withholding.
func TestContextMenuWithholdingRows(t *testing.T) {
	c := check.New(t)
	var table *cmWithholdingTable
	var list *cmWithholdingList
	var tableAsked, listAsked int
	var wnd *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 800, Height: 600},
		StartupFinishedCallback(func() {
			model := &SimpleTableModel[*focusCellRow]{}
			model.SetRootRows([]*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0)})
			tbl := NewTable[*focusCellRow](model)
			tbl.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
			table = &cmWithholdingTable{Table: tbl}
			tbl.Self = table
			tbl.SyncToModel()
			tbl.ContextMenuCallback = func(geom.Point) Menu {
				tableAsked++
				return nil
			}
			lst := NewList[string]()
			lst.Append("Zero", "One")
			list = &cmWithholdingList{List: lst}
			lst.Self = list
			lst.ContextMenuCallback = func(geom.Point) Menu {
				listAsked++
				return nil
			}
			content := NewPanel()
			content.SetLayout(&FlexLayout{Columns: 1})
			content.AddChild(tbl)
			content.AddChild(lst)
			wnd = axNewTestWindow(t, "withholding rows", geom.NewRect(10, 10, 500, 400), content)
			if wnd != nil {
				wnd.ToFront()
			}
		}))
	c.NotNil(wnd)
	rowsOf := func(p Paneler) []*accessibility.Node {
		tree := screen.AccessibilityTree(wnd)
		node := screen.AccessibilityNodeFor(p)
		c.NotNil(node)
		var rows []*accessibility.Node
		if node != nil {
			for _, id := range node.Children {
				if child := tree.Node(id); child != nil && (child.Role == role.Row || child.Role == role.ListItem) {
					rows = append(rows, child)
				}
			}
		}
		return rows
	}
	asked := func() (tbl, lst int) {
		screen.Do(func() { tbl, lst = tableAsked, listAsked })
		return tbl, lst
	}
	rowPoint := func(p *Panel, rect geom.Rect) geom.Point {
		var pt geom.Point
		screen.Do(func() {
			pt = p.PointToRoot(geom.NewPoint(rect.X+10, rect.CenterY())).Add(wnd.ContentRect().Point)
		})
		return pt
	}

	// Described while offering the menu, then withholding it: a request against that description is refused.
	tableRows := rowsOf(table.Table)
	listRows := rowsOf(list.List)
	c.Equal(2, len(tableRows))
	c.Equal(2, len(listRows))
	for _, row := range append(tableRows, listRows...) {
		c.True(row.Actions.Has(accessibility.ShowContextMenu), "a row of a widget offering its menu offers it")
	}
	screen.Do(func() {
		table.withhold = true
		list.withhold = true
	})
	for _, row := range append(tableRows, listRows...) {
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   row.ID,
			Action: accessibility.ShowContextMenu,
		}), "a request against a description from before the widget withheld its menu is refused")
	}
	for _, row := range append(rowsOf(table.Table), rowsOf(list.List)...) {
		c.False(row.Actions.Has(accessibility.ShowContextMenu),
			"a row of a widget withholding its menu does not offer it")
	}
	tbl, lst := asked()
	c.Equal(0, tbl, "the table is not asked for its menu")
	c.Equal(0, lst, "nor is the list")
	var tableSelection, listSelection int
	screen.Do(func() {
		tableSelection = table.SelectionCount()
		listSelection = list.Selection.Count()
	})
	c.Equal(0, tableSelection, "and the table's selection is left alone")
	c.Equal(0, listSelection, "as is the list's")
	var tableRect, listRect geom.Rect
	screen.Do(func() {
		tableRect = table.RowFrame(1)
		listRect = list.RowRect(1)
	})
	screen.ClickWith(rowPoint(table.AsPanel(), tableRect), ButtonRight, 0)
	screen.ClickWith(rowPoint(list.AsPanel(), listRect), ButtonRight, 0)
	tbl, lst = asked()
	c.Equal(0, tbl, "a right-click on a row of the table asks for no menu")
	c.Equal(0, lst, "nor does one on a row of the list")
	var tableSelected, listSelected bool
	screen.Do(func() {
		tableSelected = table.IsRowSelected(1)
		listSelected = list.Selection.State(1)
	})
	c.True(tableSelected, "the press reached the table as an ordinary one, selecting the row")
	c.True(listSelected, "and the list")

	screen.Do(func() {
		table.withhold = false
		list.withhold = false
	})
	tableRows = rowsOf(table.Table)
	listRows = rowsOf(list.List)
	c.Equal(2, len(tableRows))
	c.Equal(2, len(listRows))
	for _, row := range append(tableRows, listRows...) {
		c.True(row.Actions.Has(accessibility.ShowContextMenu), "once no longer withheld, each row offers the menu")
		c.False(screen.PerformAccessibilityAction(accessibility.ActionRequest{
			Node:   row.ID,
			Action: accessibility.ShowContextMenu,
		}), "the callback offers nothing, so nothing is shown")
	}
	tbl, lst = asked()
	c.Equal(2, tbl, "but the table is asked for its menu")
	c.Equal(2, lst, "and so is the list")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestShowContextMenuEndsAHeldPress verifies that Panel.ShowContextMenu, called from application code while a mouse
// button is held, ends the press before the menu opens: the panel holding it is sent its release outside every panel,
// so a button drawing itself pressed lets go without being clicked, and no button is left down.
func TestShowContextMenuEndsAHeldPress(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var popups int
	owner := NewPanel()
	owner.SetFrameRect(geom.NewRect(10, 10, 50, 50))
	owner.ContextMenuCallback = func(geom.Point) Menu { return newCMCountingMenu(&popups, true) }
	w.root.contentPanel.AddChild(owner)
	button := NewButton()
	button.SetTitle("Press")
	button.SetFrameRect(geom.NewRect(100, 100, 80, 40))
	button.UpdateCursorCallback = nil
	var clicks int
	button.ClickCallback = func() { clicks++ }
	var releases []geom.Point
	up := button.MouseUpCallback
	button.MouseUpCallback = func(where geom.Point, b int, mods mod.Modifiers) bool {
		releases = append(releases, where)
		return up(where, b, mods)
	}
	w.root.contentPanel.AddChild(button)

	w.mouseDown(geom.NewPoint(140, 120), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == button.AsPanel(), "the test needs the button to hold the press")
	c.True(button.Pressed)
	c.True(owner.ShowContextMenu(geom.NewPoint(5, 5)), "the menu is shown")
	c.Equal(1, popups)
	c.Equal([]geom.Point{button.PointFromRoot(offPanelPoint)}, releases,
		"the button was sent its release first, outside every panel")
	c.False(button.Pressed, "so it is no longer drawn pressed")
	c.Equal(0, clicks, "but was not clicked")
	c.False(w.inMouseDown, "and no button is left down")
	c.Equal(0, len(w.pressedButtons))
	c.Nil(w.lastMouseDownPanel)
}

// TestShowContextMenuFromAMouseDownCallback verifies that a panel whose MouseDownCallback opens its menu through
// Panel.ShowContextMenu, which ends the press it is handling, is not recorded as holding that press afterwards: the
// release was already spent, so recording it would leave the window believing a press was in progress, withholding
// every MouseExitCallback until the next press.
func TestShowContextMenuFromAMouseDownCallback(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var popups int
	// At the origin, which is where the test window reports the pointer.
	owner := NewPanel()
	owner.SetFrameRect(geom.NewRect(0, 0, 50, 50))
	owner.ContextMenuCallback = func(geom.Point) Menu { return newCMCountingMenu(&popups, true) }
	owner.MouseDownCallback = func(where geom.Point, button, _ int, _ mod.Modifiers) bool {
		if button == ButtonLeft {
			owner.ShowContextMenu(where)
		}
		return true
	}
	var exits int
	owner.MouseExitCallback = func() bool {
		exits++
		return false
	}
	w.root.contentPanel.AddChild(owner)

	pt := geom.NewPoint(10, 10)
	w.mouseMove(pt, 0)
	c.True(w.lastMouseOverPanel == owner, "the test needs the pointer over the panel")
	w.mouseDown(pt, ButtonLeft, 0)
	c.Equal(1, popups, "the callback opened the menu")
	c.False(w.inMouseDown, "which ended the press")
	c.Nil(w.lastMouseDownPanel, "so the panel is not recorded as holding a press that is over")
	w.mouseUp(pt, ButtonLeft, 0)
	c.Nil(w.lastMouseDownPanel, "the real release, already spent, changes nothing")
	c.Equal(0, exits)
	w.mouseMove(geom.NewPoint(100, 100), 0)
	c.Equal(1, exits, "and the pointer leaving the panel is delivered, since no press is in progress")
	c.True(w.lastMouseOverPanel == w.root.contentPanel)
}

// TestContextMenuRangeRequestWhileAPressIsHeld verifies that an assistive technology's request for a Field's menu that
// names a range, made while a press on the field is held, has the release delivered before the caret is placed on the
// range, so that a release which moves the selection does not undo the placement.
func TestContextMenuRangeRequestWhileAPressIsHeld(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	field := NewField()
	field.SetText("hello world")
	field.SetFrameRect(geom.NewRect(0, 0, 150, 30))
	field.UpdateCursorCallback = nil
	var popups int
	field.ContextMenuCallback = func(geom.Point) Menu { return newCMCountingMenu(&popups, true) }
	// The release moves the selection, as a drag's release might.
	up := field.MouseUpCallback
	field.MouseUpCallback = func(where geom.Point, button int, mods mod.Modifiers) bool {
		field.SetSelection(0, 2)
		return up(where, button, mods)
	}
	w.root.contentPanel.AddChild(field)

	w.mouseDown(geom.NewPoint(20, 15), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == field.AsPanel(), "the test needs the field to hold the press")
	c.True(field.axDispatchAction(accessibility.ActionRequest{
		Action: accessibility.ShowContextMenu, Start: 3, End: 3,
	}, false))
	c.Equal(1, popups, "the menu opens")
	start, end := field.Selection()
	c.Equal(3, start, "at the position asked for, the release having been delivered before the caret was placed")
	c.Equal(3, end)
	c.False(w.inMouseDown)
}

// TestContextMenuOfferEndedByTheRelease verifies the checks made again after the release that ends a held press, since
// a MouseUpCallback may end the offer: Panel.ShowContextMenu shows nothing for a panel the release disabled, removed,
// made withhold its menu or left in a window that is no longer the active one; the default handling of an assistive
// technology's request refuses before it moves the focus; and the Menu key asks whatever holds the focus afterwards.
func TestContextMenuOfferEndedByTheRelease(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	back.focused = false
	activateForContextMenu(t, w, back)
	var asked, popups int
	callback := func(geom.Point) Menu {
		asked++
		return newCMCountingMenu(&popups, true)
	}
	owner := newCMWithholdingPanel()
	owner.SetFrameRect(geom.NewRect(10, 10, 50, 50))
	owner.SetFocusable(true)
	owner.ContextMenuCallback = callback
	w.root.contentPanel.AddChild(owner)
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(100, 100, 50, 50))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	w.root.contentPanel.AddChild(holder)
	onHolder := geom.NewPoint(120, 120)
	onOwner := geom.NewPoint(20, 20)
	hold := func(release func()) {
		holder.MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool {
			release()
			return true
		}
		w.lastButtonTime = time.Time{}
		w.mouseDown(onHolder, ButtonLeft, 0)
		c.True(w.lastMouseDownPanel == holder, "the test needs the holder to hold the press")
	}

	hold(func() { owner.SetEnabled(false) })
	c.False(owner.ShowContextMenu(onOwner), "a panel the release disabled shows nothing")
	c.Equal(0, asked, "and is not asked")
	c.False(w.inMouseDown, "though the press was ended")
	owner.SetEnabled(true)

	hold(func() { owner.RemoveFromParent() })
	c.False(owner.ShowContextMenu(onOwner), "a panel the release removed shows nothing")
	c.Equal(0, asked)
	w.root.contentPanel.AddChild(owner)

	hold(func() { owner.withhold = true })
	c.False(owner.ShowContextMenu(onOwner), "a panel the release made withhold its menu shows nothing")
	c.Equal(0, asked)
	owner.withhold = false

	hold(func() {
		w.focused = false
		back.focused = true
	})
	c.False(owner.ShowContextMenu(onOwner), "a panel whose window the release left behind shows nothing")
	c.True(ActiveWindow() == back, "the test needs the release to have made another window the active one")
	c.Equal(0, asked, "since the menu would open in the other window")
	back.focused = false
	w.focused = true

	hold(func() {})
	c.True(owner.ShowContextMenu(onOwner), "a release that changes nothing leaves the menu to open")
	c.Equal(1, asked)
	c.Equal(1, popups)

	// The default handling of an assistive technology's request refuses before it moves the focus.
	w.SetFocus(holder)
	holder.SetFocusable(true)
	w.SetFocus(holder)
	c.True(w.CurrentFocus() == holder, "the test needs the focus elsewhere")
	hold(func() { owner.withhold = true })
	c.False(owner.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu}, false),
		"a request for the menu of a panel the release made withhold it is refused")
	c.Equal(1, asked, "without asking it")
	c.True(w.CurrentFocus() == holder, "and without moving the focus to it")
	owner.withhold = false

	// The Menu key asks whatever holds the focus once the release has been delivered.
	other := NewPanel()
	other.SetFrameRect(geom.NewRect(10, 100, 50, 50))
	other.SetFocusable(true)
	var keys int
	other.KeyDownCallback = func(KeyCode, mod.Modifiers, bool) bool {
		keys++
		return true
	}
	w.root.contentPanel.AddChild(other)
	w.SetFocus(owner)
	hold(func() { w.SetFocus(other) })
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, asked, "the panel that held the focus at the key press is not asked once the release moved it")
	c.Equal(1, keys, "and the key goes on to the panel that holds it, which has no menu")
	w.keyReleased(KeyMenu, 0)
	var otherAsked int
	other.ContextMenuCallback = func(geom.Point) Menu {
		otherAsked++
		return newCMCountingMenu(&popups, true)
	}
	w.SetFocus(owner)
	hold(func() { w.SetFocus(other) })
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, asked)
	c.Equal(1, otherAsked, "the panel the release moved the focus to is the one asked for its menu")
	c.Equal(1, keys, "and not sent the key")
	w.keyReleased(KeyMenu, 0)
}

// TestContextMenuRowRequestAfterTheReleaseEndsTheOffer verifies that an assistive technology's request for the menu of
// a List row or a Table row made while a button is held is refused when the release removes the row or the widget's
// menu, or makes another window the active one, since the release is delivered before the row is readied and may
// change any of them.
func TestContextMenuRowRequestAfterTheReleaseEndsTheOffer(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	back.focused = false
	activateForContextMenu(t, w, back)
	var asked int
	callback := func(geom.Point) Menu {
		asked++
		return nil
	}
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(0, 0, 20, 20))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	w.root.contentPanel.AddChild(holder)
	list := NewList[string]()
	list.Append("Zero", "One", "Two")
	list.SetFrameRect(geom.NewRect(0, 30, 100, 60))
	list.ContextMenuCallback = callback
	list.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(list)
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows([]*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0)})
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
	table.SetFrameRect(geom.NewRect(0, 100, 200, 60))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	table.ContextMenuCallback = callback
	onHolder := geom.NewPoint(10, 10)
	hold := func(release func()) {
		holder.MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool {
			release()
			return true
		}
		w.lastButtonTime = time.Time{}
		w.mouseDown(onHolder, ButtonLeft, 0)
		c.True(w.inMouseDown)
	}

	hold(func() { list.RemoveRange(1, 2) })
	c.False(list.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: 2}, true),
		"a list row the release removed is refused")
	c.Equal(1, list.Count(), "the test needs the release to have removed the row")
	c.Equal(0, asked, "the list is not asked for its menu")
	c.Equal(0, list.Selection.Count(), "and nothing is selected")
	c.False(w.inMouseDown, "though the press was ended")
	list.Append("One", "Two")

	hold(func() { list.ContextMenuCallback = nil })
	c.False(list.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: 2}, true),
		"a list whose menu the release removed refuses")
	c.Equal(0, asked)
	c.Equal(0, list.Selection.Count())
	list.ContextMenuCallback = callback

	hold(func() { table.ContextMenuCallback = nil })
	c.False(table.axDispatchAction(accessibility.ActionRequest{
		Action: accessibility.ShowContextMenu, Key: tid.TID("b"),
	}, true), "a table whose menu the release removed refuses")
	c.Equal(0, asked)
	c.False(table.HasSelection(), "and nothing is selected")
	table.ContextMenuCallback = callback

	// A release that makes another window the active one, where the menu would then open: refused before the row is
	// selected or the focus taken.
	holder.SetFocusable(true)
	w.SetFocus(holder)
	c.True(w.CurrentFocus() == holder, "the test needs the focus elsewhere")
	toBack := func() {
		w.focused = false
		back.focused = true
	}
	hold(toBack)
	c.False(list.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: 2}, true),
		"a list whose window the release left behind refuses")
	c.True(ActiveWindow() == back, "the test needs the release to have made another window the active one")
	c.Equal(0, asked, "since the menu would open in the other window")
	c.Equal(0, list.Selection.Count(), "with nothing selected")
	c.True(w.CurrentFocus() == holder, "and the focus left where it was")
	back.focused = false
	w.focused = true
	hold(toBack)
	c.False(table.axDispatchAction(accessibility.ActionRequest{
		Action: accessibility.ShowContextMenu, Key: tid.TID("b"),
	}, true), "a table whose window the release left behind refuses")
	c.True(ActiveWindow() == back)
	c.Equal(0, asked)
	c.False(table.HasSelection(), "with nothing selected")
	c.True(w.CurrentFocus() == holder, "and the focus left where it was")
	back.focused = false
	w.focused = true
	holder.SetFocusable(false)
	w.SetFocus(nil)

	hold(func() {})
	c.False(list.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: 2}, true),
		"the callback offers nothing, so nothing is shown")
	c.Equal(1, asked, "but a release that changes nothing leaves the list to be asked")
	c.True(list.Selection.State(2))
}

// TestContextMenuChordedRightPressInATransientWindow verifies that a right press made while another button is held over
// a panel with a menu is ignored only in the active window: in a transient window, where no menu could open, it is
// delivered as any other press.
func TestContextMenuChordedRightPressInATransientWindow(t *testing.T) {
	c := check.New(t)
	front := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	activateForContextMenu(t, back, front)
	front.transient = true
	c.True(ActiveWindow() == back, "the test needs the transient window passed over")
	var asked int
	owner := front.root.contentPanel
	owner.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	var log cmMouseLog
	log.logMouse(owner)
	pt := geom.NewPoint(10, 10)
	front.mouseDown(pt, ButtonLeft, 0)
	front.mouseDown(pt, ButtonRight, 0)
	c.True(front.pressedButtons[ButtonRight], "the right press is counted as down")
	c.Equal([]int{ButtonLeft, ButtonRight}, log.buttons(), "and delivered")
	front.mouseUp(pt, ButtonRight, 0)
	front.mouseUp(pt, ButtonLeft, 0)
	c.Equal([]string{cmDown, cmDown, cmUp, cmUp}, log.kinds(), "as is its release")
	c.Equal(0, asked, "and no menu is asked for")
	c.False(front.inMouseDown)
}

// TestContextMenuReleaseWhileBlockedByAModal verifies that a left release arriving while the window is blocked by a
// modal, with a taken right press still held, ends the gesture: the taken press is part of no gesture, so the panel
// holding the left press is let go of rather than being fed drags for the life of the modal.
func TestContextMenuReleaseWhileBlockedByAModal(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var asked int
	owner := NewPanel()
	owner.SetFrameRect(geom.NewRect(10, 10, 50, 50))
	owner.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	w.root.contentPanel.AddChild(owner)
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(100, 100, 50, 50))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	w.root.contentPanel.AddChild(holder)

	w.mouseDown(geom.NewPoint(20, 20), ButtonRight, 0)
	c.True(w.contextMenuPanel == owner, "the test needs the right press taken for the menu")
	w.mouseDown(geom.NewPoint(120, 120), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == holder, "and the holder to hold a left press")
	c.True(w.rightPressTaken)
	pushTestModal(t, newModalInputTestWindow())
	c.False(w.okToProcess(), "the test needs the window blocked")
	w.mouseUp(geom.NewPoint(120, 120), ButtonLeft, 0)
	c.Nil(w.lastMouseDownPanel, "the left release ends the gesture though the taken right press is still down")
	c.True(w.inMouseDown)
	w.mouseUp(geom.NewPoint(20, 20), ButtonRight, 0)
	c.False(w.inMouseDown)
	c.Equal(0, asked, "no menu opens in a blocked window")
}

// TestContextMenuPressHandedBackToACellWidget verifies that a ContextMenuPressHandler in a Table cell that asks for its
// press back is given it when the right press becomes a drag, although the window finds only the table under the press:
// the press is delivered through the table, as any press on a cell widget is, and the drag follows.
func TestContextMenuPressHandedBackToACellWidget(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	rows := []*focusCellRow{newFocusCellRow("a", 1), newFocusCellRow("b", 1)}
	handler := newCMPressHandler(true)
	handler.SetFocusable(true)
	var asked int
	handler.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	log := &cmMouseLog{}
	log.logMouse(handler.AsPanel())
	// Seeded before the row is first asked for the cell, so that the row hands the handler back for it.
	rows[1].cells[0] = handler.AsPanel()
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 80}, {ID: 1, Current: 60}, {ID: 2, Current: 40}}
	table.SetFrameRect(geom.NewRect(0, 0, 200, 120))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()

	press := table.CellFrame(1, 0).Center()
	w.mouseDown(press, ButtonRight, 0)
	c.True(w.contextMenuPanel == handler.AsPanel(), "the right press is taken for the widget's menu")
	c.Equal(1, handler.presses, "and the widget is told of it")
	c.True(w.CurrentFocus() == handler.AsPanel(), "the widget holds the focus")
	c.True(handler.Parent() == table.AsPanel(), "with its cell attached to the table")
	c.Equal(0, len(log.events), "and no mouse event has reached it")
	_, drift := DragGestureParameters()
	far := geom.NewPoint(press.X+drift*2+2, press.Y)
	w.mouseMovedOrDragged(far, 0)
	c.Nil(w.contextMenuPanel, "a drag ends the right-click")
	c.Equal([]string{cmDown, cmDrag}, log.kinds(),
		"and the press is handed back to the widget, with the move as its first drag")
	c.Equal([]int{ButtonRight, ButtonRight}, log.buttons())
	if len(log.events) != 0 {
		c.Equal(handler.PointFromRoot(press), log.events[0].where, "delivered where it was made")
	}
	w.mouseUp(far, ButtonRight, 0)
	c.Equal([]string{cmDown, cmDrag, cmUp}, log.kinds(), "as is the release")
	c.Equal(0, asked, "and no menu opens")
}

// TestContextMenuReleaseUpdatesTheHover verifies that the release endPressesForContextMenu delivers outside every panel
// brings the hover state up to date at the pointer rather than where the release was delivered: a pointer still over
// the panel it was over keeps that panel as the one under it, while one dragged to another panel since the press has
// the panel it left told so and the cursor of the panel it is over shown. Both presses are declined, since a panel
// holding a press is not told the pointer left while it holds it, which would hide where the release was delivered.
func TestContextMenuReleaseUpdatesTheHover(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var asked int
	owner := NewPanel()
	owner.SetFrameRect(geom.NewRect(100, 100, 50, 50))
	owner.SetFocusable(true)
	owner.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	w.root.contentPanel.AddChild(owner)
	owner.RequestFocus()
	// At the origin, which is where the test window reports the pointer.
	hovered := NewPanel()
	hovered.SetFrameRect(geom.NewRect(0, 0, 50, 50))
	sentinel := &Cursor{}
	hovered.UpdateCursorCallback = func(geom.Point) *Cursor { return sentinel }
	var hoveredExits int
	hovered.MouseExitCallback = func() bool {
		hoveredExits++
		return false
	}
	w.root.contentPanel.AddChild(hovered)
	other := NewPanel()
	other.SetFrameRect(geom.NewRect(60, 0, 50, 50))
	var otherExits int
	other.MouseExitCallback = func() bool {
		otherExits++
		return false
	}
	w.root.contentPanel.AddChild(other)

	// The pointer stays over the panel it was over.
	onHovered := geom.NewPoint(10, 10)
	w.mouseMove(onHovered, 0)
	c.True(w.lastMouseOverPanel == hovered, "the test needs the pointer over the panel")
	c.True(w.cursor == sentinel, "with its cursor shown")
	w.mouseDown(onHovered, ButtonLeft, 0)
	c.Nil(w.lastMouseDownPanel, "the test needs the press declined")
	c.True(w.inMouseDown)
	c.False(owner.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu}, false),
		"the callback offers nothing, so nothing is shown")
	c.Equal(1, asked, "but the request asks the panel for its menu")
	c.False(w.inMouseDown, "having ended the press")
	c.Equal(0, hoveredExits, "the panel under the pointer is not told the pointer left")
	c.True(w.lastMouseOverPanel == hovered, "and is still the one under it")
	c.True(w.cursor == sentinel, "with its cursor still the one shown")

	// The pointer was dragged from one panel to another, which no move reported, and the release ends the press over
	// the second: the first is told the pointer left, and the cursor is the second's.
	onOther := geom.NewPoint(70, 10)
	w.mouseMove(onOther, 0)
	c.True(w.lastMouseOverPanel == other, "the test needs the pointer over the other panel")
	c.Equal(1, hoveredExits)
	c.True(w.cursor != sentinel, "and its cursor shown")
	w.mouseDown(onOther, ButtonLeft, 0)
	c.Nil(w.lastMouseDownPanel, "the test needs the press declined")
	c.True(w.inMouseDown)
	w.mouseMovedOrDragged(onHovered, 0)
	c.True(w.lastMouseOverPanel == other, "a drag leaves the hover where the press began")
	c.True(w.cursor != sentinel, "and the cursor as it was")
	w.keyPressed(KeyMenu, 0)
	c.Equal(2, asked)
	c.False(w.inMouseDown)
	c.Equal(1, otherExits, "the panel the pointer was dragged off is told it left")
	c.Nil(w.lastMouseOverPanel)
	c.True(w.cursor == sentinel, "and the cursor is that of the panel the pointer is over")
	w.keyReleased(KeyMenu, 0)
	w.mouseMove(onHovered, 0)
	c.True(w.lastMouseOverPanel == hovered, "and the next move enters the panel the pointer is over")
}

// cmClosableDockable is a testDockable whose tab shows a close button.
type cmClosableDockable struct {
	testDockable
	closeAttempts int
}

func newCMClosableDockable(title string) *cmClosableDockable {
	d := &cmClosableDockable{}
	d.title = title
	d.Self = d
	return d
}

// MayAttemptClose implements TabCloser.
func (d *cmClosableDockable) MayAttemptClose() bool { return true }

// AttemptClose implements TabCloser, counting the attempts and closing nothing.
func (d *cmClosableDockable) AttemptClose() bool {
	d.closeAttempts++
	return false
}

// newDockTabTestWindow returns the active window holding a dock container of two closable dockables, laid out, with the
// tabs of the container.
func newDockTabTestWindow(t *testing.T) (w *Window, dc *DockContainer, tabs []*dockTab) {
	t.Helper()
	w = newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	dock, dc := newTestDockContainer(newCMClosableDockable("First"), newCMClosableDockable("Second"))
	w.root.contentPanel.AddChild(dock)
	// The dock is laid out on its own, since the content panel's layout would size it to its preferred size.
	dock.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	dock.ValidateLayout()
	tabs, _ = dc.header.partition()
	if len(tabs) != 2 {
		t.Fatalf("the test needs two tabs, got %d", len(tabs))
	}
	// A release updates the cursor, and the dock's and tabs' defaults would create a platform cursor; with them nil,
	// the content panel's stand-in answers.
	dock.UpdateCursorCallback = nil
	for _, tab := range tabs {
		tab.UpdateCursorCallback = nil
		if tab.button != nil {
			tab.button.UpdateCursorCallback = nil
		}
	}
	return w, dc, tabs
}

// TestDockTabCloseButtonChordedRightPress verifies that a right press made while the left button is held on a tab's
// close button is claimed on behalf of the press already held, so that the left release still reaches the button and it
// does not stay drawn pressed, whichever button is released first, while the right release cannot click it; a right
// press alone is still declined.
func TestDockTabCloseButtonChordedRightPress(t *testing.T) {
	c := check.New(t)
	w, _, tabs := newDockTabTestWindow(t)
	tab := tabs[1]
	c.NotNil(tab.button, "the test needs a close button")
	button := tab.button
	var clicks int
	button.ClickCallback = func() { clicks++ }
	button.ClickAnimationTime = 0
	onClose := button.RectToRoot(button.ContentRect(true)).Center()
	c.True(w.root.PanelAt(onClose) == button.AsPanel(), "the test needs the point over the close button")

	for _, first := range []int{ButtonRight, ButtonLeft} {
		second := ButtonLeft
		if first == ButtonLeft {
			second = ButtonRight
		}
		w.lastButtonTime = time.Time{}
		w.mouseDown(onClose, ButtonLeft, 0)
		c.True(w.lastMouseDownPanel == button.AsPanel(), "the left press is the button's")
		c.True(button.Pressed, "which draws itself pressed")
		w.mouseDown(onClose, ButtonRight, 0)
		c.True(w.lastMouseDownPanel == button.AsPanel(),
			"a right press while the button holds a press is claimed on its behalf")
		c.True(button.Pressed)
		w.mouseUp(onClose, first, 0)
		if first == ButtonRight {
			c.True(button.Pressed, "the right release does not let go of the button while the left is held")
			c.Equal(0, clicks, "nor does it click")
		} else {
			c.False(button.Pressed, "the left release lets go of the button")
			c.Equal(1, clicks, "and clicks it, the pointer being over it")
		}
		w.mouseUp(onClose, second, 0)
		c.False(button.Pressed, "the button is let go of once every button is up, button %d released first", first)
		c.Equal(1, clicks, "and clicked once, by the left release")
		c.False(w.inMouseDown)
		c.Nil(w.lastMouseDownPanel)
		clicks = 0
	}

	w.lastButtonTime = time.Time{}
	w.mouseDown(onClose, ButtonRight, 0)
	c.False(w.lastMouseDownPanel == button.AsPanel(), "a right press alone is declined by the button")
	c.False(button.Pressed)
	w.mouseUp(onClose, ButtonRight, 0)
	c.Equal(0, clicks, "and its release cannot click")
}

// TestDockTabTitleFollowsTheTab verifies that a tab's title offers exactly the menu the tab offers: none once the tab
// is disabled or its callback cleared, and the tab's replacement callback when an application installs one, asked at
// the position in the tab's coordinates.
func TestDockTabTitleFollowsTheTab(t *testing.T) {
	c := check.New(t)
	_, dc, tabs := newDockTabTestWindow(t)
	tab := tabs[0]
	title := tab.title
	c.True(tab.offersContextMenu(), "the test needs a tab among several, offering its menu")
	c.True(title.offersContextMenu(), "the title offers the tab's menu")

	tab.SetEnabled(false)
	c.False(title.offersContextMenu(), "a disabled tab's title offers nothing")
	tab.SetEnabled(true)
	c.True(title.offersContextMenu())

	saved := tab.ContextMenuCallback
	tab.ContextMenuCallback = nil
	c.False(tab.offersContextMenu())
	c.True(title.WithholdsContextMenu(), "the title of a tab whose callback was cleared withholds its menu")
	c.False(title.offersContextMenu())
	c.Nil(title.ContextMenuCallback(geom.NewPoint(3, 4)), "and builds nothing")

	var asked int
	var at geom.Point
	tab.ContextMenuCallback = func(where geom.Point) Menu {
		asked++
		at = where
		return nil
	}
	c.True(title.offersContextMenu(), "the title offers a menu again once the tab has a callback")
	c.Nil(title.ContextMenuCallback(geom.NewPoint(3, 4)))
	c.Equal(1, asked, "the tab's replacement callback is the one asked")
	c.Equal(title.PointTo(geom.NewPoint(3, 4), tab.AsPanel()), at, "at the position in the tab's coordinates")
	c.NotEqual(geom.NewPoint(3, 4), at, "which is not the title's own")
	tab.ContextMenuCallback = saved

	dc.Close(tabs[1].dockable)
	c.True(tab.WithholdsContextMenu(), "the test needs the tab alone, withholding its menu")
	c.True(title.WithholdsContextMenu(), "and its title withholds with it")
	c.False(title.offersContextMenu())
}

// cmPanickingWithholder is a panel whose ContextMenuWithholder panics.
type cmPanickingWithholder struct {
	Panel
}

func newCMPanickingWithholder() *cmPanickingWithholder {
	p := &cmPanickingWithholder{}
	p.Self = p
	return p
}

// WithholdsContextMenu implements ContextMenuWithholder, badly.
func (p *cmPanickingWithholder) WithholdsContextMenu() bool {
	panic("withholder")
}

// TestContextMenuWithholderPanics verifies that a ContextMenuWithholder that panics is taken as not withholding, and
// that the panic does not escape into whatever asked whether the panel offers a menu.
func TestContextMenuWithholderPanics(t *testing.T) {
	c := check.New(t)
	p := newCMPanickingWithholder()
	c.False(p.offersContextMenu(), "a panel without a callback offers nothing")
	p.ContextMenuCallback = func(geom.Point) Menu { return nil }
	c.True(p.offersContextMenu(), "a withholder that panics is taken as not withholding")
}

// TestContextMenuCellWidgetRequestAfterTheReleaseChangesTheRows verifies that an assistive technology's request for
// the menu of a widget in a table cell, made while a button is held, looks the row up again once the release has been
// delivered, since the release may change the rows: the widget of the row asked for is the one asked for its menu,
// wherever the release moved the row, and a row the release removed is refused.
func TestContextMenuCellWidgetRequestAfterTheReleaseChangesTheRows(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	rows := []*focusCellRow{newFocusCellRow("a", 1), newFocusCellRow("b", 1), newFocusCellRow("c", 1)}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 80}, {ID: 1, Current: 60}, {ID: 2, Current: 40}}
	table.SetFrameRect(geom.NewRect(0, 0, 200, 120))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	asked := make(map[tid.TID]int)
	for _, row := range rows {
		cell := row.ColumnCell(0, 0, nil, nil, false, false, false).AsPanel()
		cell.ContextMenuCallback = func(geom.Point) Menu {
			asked[row.id]++
			return nil
		}
	}
	// A panel whose release moves the last row to the front.
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(0, 130, 100, 40))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	holder.MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool {
		model.SetRootRows([]*focusCellRow{rows[2], rows[0], rows[1]})
		table.SyncToModel()
		return true
	}
	w.root.contentPanel.AddChild(holder)
	onHolder := geom.NewPoint(50, 150)
	ask := func(id string) accessibility.ActionRequest {
		return accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: axCellPanelKey{
			Cell: accessibility.CellKey{Row: tid.TID(id), Col: 0},
		}}
	}

	w.mouseDown(onHolder, ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == holder)
	c.False(table.axDispatchAction(ask("c"), true), "the callback returns nil, so no menu is shown")
	c.False(w.inMouseDown, "the press was ended")
	c.Equal(0, table.RowToIndex(rows[2]), "and the row asked for moved")
	c.Equal(map[tid.TID]int{"c": 1}, asked, "the widget of the row asked for is the one asked for its menu")
	c.True(w.CurrentFocus() == rows[2].cells[0], "and holds the focus")
	row, col := table.FocusedCell()
	c.Equal(0, row, "with its cell adopted at the index the release left the row at")
	c.Equal(0, col)
	c.True(table.IsRowSelected(0), "and its row selected")
	c.Equal(1, table.SelectionCount())

	// A release that removes the row asked for.
	w.SetFocus(nil)
	model.SetRootRows(rows)
	table.SyncToModel()
	holder.MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool {
		model.SetRootRows(rows[:2])
		table.SyncToModel()
		return true
	}
	w.lastButtonTime = time.Time{}
	w.mouseDown(onHolder, ButtonLeft, 0)
	c.False(table.axDispatchAction(ask("c"), true), "a row the release removed is refused")
	c.False(w.inMouseDown)
	c.Equal(2, table.LastRowIndex()+1, "the test needs the release to have removed the row")
	c.Equal(map[tid.TID]int{"c": 1}, asked, "and no widget is asked")
	c.Nil(w.CurrentFocus(), "nor given the focus")
	row, _ = table.FocusedCell()
	c.Equal(-1, row)
}

// TestContextMenuChordDuringASliderOrDividerDrag verifies that the Menu key or shift+F10 pressed while a slider's thumb
// or a dock divider is being dragged opens the menu and ends the drag where it was, since the release that ends the
// press is delivered outside every panel and must not be taken as a final position.
func TestContextMenuChordDuringASliderOrDividerDrag(t *testing.T) {
	c := check.New(t)
	var popups, keysDown, keysUp int
	w := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, true)
	slider := NewSlider(0, 100, 0)
	slider.SetFrameRect(geom.NewRect(0, 100, 200, 20))
	slider.UpdateCursorCallback = nil
	slider.ContextMenuCallback = func(geom.Point) Menu { return newCMCountingMenu(&popups, true) }
	w.root.contentPanel.AddChild(slider)
	w.mouseDown(geom.NewPoint(40, 110), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == slider.AsPanel(), "the test needs the slider to hold the press")
	c.True(slider.pressed)
	w.mouseDrag(geom.NewPoint(120, 110), ButtonLeft, 0)
	dragged := slider.Value()
	c.True(dragged > 40 && dragged < 80, "the test needs the drag to have moved the thumb, not to %v", dragged)
	w.keyPressed(KeyMenu, 0)
	c.Equal(1, popups, "the Menu key opens the menu")
	c.False(w.inMouseDown, "the press was ended")
	c.False(slider.pressed, "and the slider is no longer pressed")
	c.Equal(dragged, slider.Value(), "with the value where the drag left it")
	w.keyReleased(KeyMenu, 0)

	dock := NewDock()
	first := newCMClosableDockable("First")
	second := newCMClosableDockable("Second")
	dock.DockTo(first, nil, side.Left)
	dock.DockTo(second, Ancestor[*DockContainer](first), side.Right)
	dock.SetFrameRect(geom.NewRect(0, 130, 200, 60))
	dock.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(dock)
	dock.ValidateLayout()
	layout := dock.RootDockLayout()
	c.True(layout.Full(), "the test needs the two dockables side by side, with a divider between them")
	c.True(layout.Horizontal)
	before := layout.DividerPosition()
	c.True(before > 0)
	onDivider := geom.NewPoint(before+dock.DockDividerSize()/2, 160)
	w.lastButtonTime = time.Time{}
	w.mouseDown(onDivider, ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == dock.AsPanel(), "the test needs the dock to hold the press")
	c.NotNil(dock.dividerDragLayout, "for the divider")
	w.mouseDrag(onDivider.Add(geom.NewPoint(10, 0)), ButtonLeft, 0)
	moved := layout.DividerPosition()
	c.Equal(before+10, moved, "the test needs the drag to have moved the divider")
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(2, popups, "shift+F10 opens the menu")
	c.False(w.inMouseDown, "the press was ended")
	c.Nil(dock.dividerDragLayout, "and the drag with it")
	c.Equal(moved, layout.DividerPosition(), "with the divider where the drag left it")
	w.keyReleased(KeyF10, mod.Shift)
}

// TestContextMenuChordKeyRecorded verifies that the window records the key of the chord that opened an in-window
// contextual menu until its key up arrives, which the Windows platform code relies on to keep such an F10 from
// DefWindowProc, and records nothing for a native menu, whose tracking loop swallows the key up, or for a chord that
// opened nothing.
func TestContextMenuChordKeyRecorded(t *testing.T) {
	c := check.New(t)
	var popups, keysDown, keysUp int
	w := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, false)
	owner := w.CurrentFocus()
	c.NotNil(owner)
	c.Equal(KeyNone, w.contextMenuChordKey)
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(1, popups)
	c.Equal(KeyF10, w.contextMenuChordKey, "the F10 of the chord that opened the menu is recorded")
	w.keyReleased(KeyF10, mod.Shift)
	c.Equal(KeyNone, w.contextMenuChordKey, "until its key up")
	w.keyPressed(KeyEscape, 0)
	w.keyReleased(KeyEscape, 0)
	w.keyPressed(KeyMenu, 0)
	c.Equal(2, popups)
	c.Equal(KeyMenu, w.contextMenuChordKey, "and so is the Menu key")
	w.keyReleased(KeyLShift, 0)
	c.Equal(KeyMenu, w.contextMenuChordKey, "which another key's key up leaves alone")
	w.keyReleased(KeyMenu, 0)
	c.Equal(KeyNone, w.contextMenuChordKey)
	w.keyPressed(KeyEscape, 0)
	w.keyReleased(KeyEscape, 0)

	callback := owner.ContextMenuCallback
	owner.ContextMenuCallback = func(geom.Point) Menu { return nil }
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(2, popups, "the test needs the chord to open nothing")
	c.Equal(KeyNone, w.contextMenuChordKey, "a chord that opened nothing is not recorded")
	w.keyReleased(KeyF10, mod.Shift)
	owner.ContextMenuCallback = callback

	native := newKeyMenuTestWindow(t, &popups, &keysDown, &keysUp, true)
	native.keyPressed(KeyF10, mod.Shift)
	c.Equal(3, popups)
	c.Equal(KeyNone, native.contextMenuChordKey, "a chord that opened a native menu is not recorded")
}

// TestContextMenuActivationChangeWithoutAFocusEvent verifies that a window that stops being the active one without
// losing the focus itself is described again, so that its nodes stop offering accessibility.ShowContextMenu, which a
// request would refuse: a transient window that takes the focus is passed over by ActiveWindow(), so the window behind
// it stays the active one, and only the transient window and the window the focus then goes on to hear of the change.
func TestContextMenuActivationChangeWithoutAFocusEvent(t *testing.T) {
	c := check.New(t)
	var a, b, transient *Window
	var owner *Panel
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			a = newHeadlessTestWindow(t, "a", geom.NewRect(0, 0, 200, 150))
			b = newHeadlessTestWindow(t, "b", geom.NewRect(200, 150, 200, 150))
			if a == nil || b == nil {
				return
			}
			owner = NewPanel()
			owner.SetFocusable(true)
			owner.SetFrameRect(geom.NewRect(10, 10, 50, 50))
			owner.ContextMenuCallback = func(geom.Point) Menu { return nil }
			a.Content().AddChild(owner)
			a.ToFront()
		}))
	c.NotNil(a)
	c.NotNil(b)
	c.NotNil(owner)
	offered := func() bool {
		var offers bool
		screen.Do(func() {
			if node := a.ax.last.Node(axIDFor(owner)); node != nil {
				offers = node.Actions.Has(accessibility.ShowContextMenu)
			}
		})
		return offers
	}
	c.NotNil(screen.AccessibilityTree(a))
	var active *Window
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == a, "the test needs the first window active")
	c.True(offered(), "whose panel then offers its menu")

	c.True(screen.Do(func() {
		var err error
		if transient, err = NewWindow("transient", TransientWindowOption()); err != nil {
			t.Errorf("unable to create transient window: %v", err)
			return
		}
		transient.SetContentRect(geom.NewRect(50, 50, 100, 60))
		transient.ToFront()
	}))
	c.NotNil(transient)
	c.True(screen.FocusedWindow() == transient, "the test needs the transient window to have taken the focus")
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == a, "which leaves the first window the active one")
	c.True(offered(), "and its panel offering its menu")

	// The application is deactivated while the transient window holds the focus, and activated again with the focus
	// going back to it: the first window becomes the active one again without gaining any focus itself, which only the
	// transient window gaining the focus can tell it.
	c.True(screen.Do(func() { screen.setFocus(nil) }))
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == nil, "with no window focused there is no active window")
	c.False(offered(), "so the panel no longer offers its menu")
	c.True(screen.Do(func() { screen.setFocus(transient) }))
	c.True(screen.FocusedWindow() == transient)
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == a, "the focus back on the transient window makes the first window the active one again")
	c.True(offered(), "and its panel offers its menu again, though the window itself gained no focus")

	c.True(screen.Do(func() { b.ToFront() }))
	c.True(screen.FocusedWindow() == b, "the test needs the second window to have taken the focus")
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == b, "which makes it the active one")
	c.False(offered(), "so the first window's panel no longer offers its menu, though the window lost no focus")

	c.True(screen.Do(func() { transient.ToFront() }))
	c.True(screen.FocusedWindow() == transient)
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == b, "a transient window taking the focus leaves the second window the active one")
	c.False(offered())
	c.True(screen.Do(func() { transient.Dispose() }))
	c.True(screen.Do(func() { a.ToFront() }))
	screen.Do(func() { active = ActiveWindow() })
	c.True(active == a)
	c.True(offered(), "and the panel offers its menu again once its window is the active one")
}

// TestContextMenuChordDuringAHeldPressKeepsTheSelection verifies that the Menu key or shift+F10 pressed while a left
// press is held on one of several selected rows of a List or a Table opens the menu with the whole selection kept: the
// release that ends the press arrives outside the widget, so it is not the click that would narrow the selection to
// the pressed row, which a real click still does.
func TestContextMenuChordDuringAHeldPressKeepsTheSelection(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	var listSelected, tableSelected int
	list := NewList[string]()
	list.Append("Zero", "One", "Two", "Three")
	list.SetAllowMultipleSelection(true)
	list.SetFrameRect(geom.NewRect(0, 0, 100, 80))
	list.UpdateCursorCallback = nil
	list.ContextMenuCallback = func(geom.Point) Menu {
		listSelected = list.Selection.Count()
		return nil
	}
	w.root.contentPanel.AddChild(list)
	list.Select(false, 0, 1, 2)
	c.Equal(3, list.Selection.Count())
	onListRow := func(row int) geom.Point {
		rect := list.RowRect(row)
		return geom.NewPoint(rect.X+5, rect.CenterY())
	}
	w.mouseDown(onListRow(1), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == list.AsPanel(), "the test needs the list to hold the press")
	c.Equal(3, list.Selection.Count(), "a press on a selected row keeps the selection until the release")
	w.keyPressed(KeyMenu, 0)
	c.Equal(3, listSelected, "the Menu key ending the press does not narrow the selection to the pressed row")
	c.Equal(3, list.Selection.Count())
	c.False(w.inMouseDown, "though the press was ended")
	w.keyReleased(KeyMenu, 0)
	w.lastButtonTime = time.Time{}
	w.mouseDown(onListRow(1), ButtonLeft, 0)
	w.mouseUp(onListRow(1), ButtonLeft, 0)
	c.Equal(1, list.Selection.Count(), "a click on a selected row narrows the selection to it")
	c.True(list.Selection.State(1))

	rows := []*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0), newFocusCellRow("c", 0)}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 80}, {ID: 1, Current: 60}, {ID: 2, Current: 40}}
	table.SetFrameRect(geom.NewRect(0, 100, 200, 90))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	table.ContextMenuCallback = func(geom.Point) Menu {
		tableSelected = table.SelectionCount()
		return nil
	}
	table.SelectAll()
	c.Equal(3, table.SelectionCount())
	onTableRow := func(row int) geom.Point {
		rect := table.RowFrame(row)
		return geom.NewPoint(rect.X+5, rect.CenterY()+100)
	}
	w.lastButtonTime = time.Time{}
	w.mouseDown(onTableRow(1), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == table.AsPanel(), "the test needs the table to hold the press")
	c.Equal(3, table.SelectionCount(), "a press on a selected row keeps the selection until the release")
	w.keyPressed(KeyF10, mod.Shift)
	c.Equal(3, tableSelected, "shift+F10 ending the press does not narrow the selection to the pressed row")
	c.Equal(3, table.SelectionCount())
	c.False(w.inMouseDown, "though the press was ended")
	w.keyReleased(KeyF10, mod.Shift)
	w.lastButtonTime = time.Time{}
	w.mouseDown(onTableRow(1), ButtonLeft, 0)
	w.mouseUp(onTableRow(1), ButtonLeft, 0)
	c.Equal(1, table.SelectionCount(), "a click on a selected row narrows the selection to it")
	c.True(table.IsRowSelected(1))
}

// TestContextMenuRowRequestAfterTheFocusMoveEndsTheOffer verifies the checks made again after a List or a Table takes
// the focus for an assistive technology's request for a row's menu, and after the row is selected for it, since the
// panel losing the focus or the selection callback may end the offer: the request is refused, without the widget being
// asked, when the row is gone, the menu was taken away, the widget was disabled or another window was made the active
// one, with the selection left alone where the focus move ended the offer and the cell cursor left alone where the
// selection callback did.
func TestContextMenuRowRequestAfterTheFocusMoveEndsTheOffer(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	back.focused = false
	activateForContextMenu(t, w, back)
	var asked, listChanges, tableChanges int
	callback := func(geom.Point) Menu {
		asked++
		return nil
	}
	// A focusable panel elsewhere in the window, standing in for a field that commits on losing the focus.
	holder := NewPanel()
	holder.SetFocusable(true)
	holder.SetFrameRect(geom.NewRect(0, 0, 20, 20))
	w.root.contentPanel.AddChild(holder)
	list := NewList[string]()
	list.Append("Zero", "One", "Two")
	list.SetFrameRect(geom.NewRect(0, 30, 100, 60))
	list.ContextMenuCallback = callback
	list.UpdateCursorCallback = nil
	list.NewSelectionCallback = func() { listChanges++ }
	w.root.contentPanel.AddChild(list)
	rows := []*focusCellRow{newFocusCellRow("a", 1), newFocusCellRow("b", 1)}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
	table.SetFrameRect(geom.NewRect(0, 100, 200, 60))
	table.UpdateCursorCallback = nil
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	table.ContextMenuCallback = callback
	table.SelectionChangedCallback = func() { tableChanges++ }
	askList := func() bool {
		return list.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: 2}, true)
	}
	askTable := func() bool {
		return table.axDispatchAction(accessibility.ActionRequest{
			Action: accessibility.ShowContextMenu, Key: tid.TID("b"),
		}, true)
	}
	focusHolder := func(lost func()) {
		holder.LostFocusCallback = lost
		w.SetFocus(holder)
		c.True(w.CurrentFocus() == holder, "the test needs the holder focused")
	}
	toBack := func() {
		w.focused = false
		back.focused = true
	}
	toFront := func() {
		back.focused = false
		w.focused = true
	}

	focusHolder(func() { list.RemoveRange(1, 2) })
	c.False(askList(), "a list row that the focus move removed is refused")
	c.Equal(1, list.Count(), "the test needs the focus move to have removed the row")
	c.Equal(0, asked, "without the list being asked")
	c.Equal(0, listChanges, "and nothing selected")
	c.Equal(0, list.Selection.Count())
	list.Append("One", "Two")

	focusHolder(func() { list.ContextMenuCallback = nil })
	c.False(askList(), "a list whose menu the focus move took away refuses")
	c.Equal(0, asked)
	c.Equal(0, listChanges)
	c.Equal(0, list.Selection.Count())
	list.ContextMenuCallback = callback

	focusHolder(toBack)
	c.False(askList(), "a list whose window the focus move left behind refuses")
	c.True(ActiveWindow() == back, "the test needs the focus move to have made another window the active one")
	c.Equal(0, asked)
	c.Equal(0, listChanges)
	c.Equal(0, list.Selection.Count())
	toFront()

	w.SetFocus(nil)
	list.NewSelectionCallback = func() {
		listChanges++
		list.ContextMenuCallback = nil
	}
	c.False(askList(), "a list whose menu the selection change took away refuses")
	c.Equal(1, listChanges, "once the row was selected")
	c.Equal(0, asked, "without the list being asked")
	list.ContextMenuCallback = callback
	list.Selection.Reset()
	list.NewSelectionCallback = func() {
		listChanges++
		list.RemoveRange(1, 2)
	}
	c.False(askList(), "a list row that the selection change removed is refused")
	c.Equal(1, list.Count(), "the test needs the selection change to have removed the row")
	c.Equal(2, listChanges, "once the row was selected")
	c.Equal(0, asked, "without the list being asked for a menu over a row it no longer has")
	list.Append("One", "Two")
	list.NewSelectionCallback = func() { listChanges++ }

	// The table: the cell holding the focus commits as the table takes the focus from it.
	table.SelectByIndex(0)
	c.True(table.FocusCell(0, 0))
	cell := rows[0].cells[0]
	c.True(w.CurrentFocus() == cell, "the test needs the first row's cell focused")
	lost := cell.LostFocusCallback
	commit := func(then func()) {
		tableChanges = 0
		c.True(table.FocusCell(0, 0))
		c.True(w.CurrentFocus() == cell, "the test needs the first row's cell focused")
		cell.LostFocusCallback = func() {
			lost()
			then()
		}
	}
	untouched := func(what string) {
		c.Equal(0, asked, "%s: the table is not asked", what)
		c.Equal(0, tableChanges, "%s: and the selection is left alone", what)
		c.True(table.IsRowSelected(0), what)
		c.False(table.IsRowSelected(1), what)
	}

	commit(func() { table.ContextMenuCallback = nil })
	c.False(askTable(), "a table whose menu the commit took away refuses")
	untouched("menu taken away")
	table.ContextMenuCallback = callback

	commit(func() { table.SetEnabled(false) })
	c.False(askTable(), "a table the commit disabled refuses")
	untouched("disabled")
	table.SetEnabled(true)

	commit(toBack)
	c.False(askTable(), "a table whose window the commit left behind refuses")
	c.True(ActiveWindow() == back, "the test needs the commit to have made another window the active one")
	untouched("window left behind")
	toFront()

	commit(func() {
		model.SetRootRows(rows[:1])
		table.SyncToModel()
	})
	c.False(askTable(), "a table row that the commit removed is refused")
	c.Equal(1, table.LastRowIndex()+1, "the test needs the commit to have removed the row")
	c.Equal(0, asked, "without the table being asked")
	c.Equal(0, tableChanges, "and the selection left alone")
	model.SetRootRows(rows)
	table.SyncToModel()
	cell.LostFocusCallback = lost

	// A SelectionChangedCallback that ends the offer: refused once the row is selected, with the cell cursor left
	// where it was.
	w.SetFocus(table)
	c.True(table.SetLeadCell(0, 1))
	c.Equal(1, table.LeadColumnIndex())
	tableChanges = 0
	table.SelectionChangedCallback = func() {
		tableChanges++
		table.ContextMenuCallback = nil
	}
	c.False(table.axDispatchAction(accessibility.ActionRequest{
		Action: accessibility.ShowContextMenu, Key: accessibility.CellKey{Row: tid.TID("b"), Col: 0},
	}, true), "a table whose menu the selection change took away refuses")
	c.Equal(1, tableChanges, "once the row was selected")
	c.True(table.IsRowSelected(1))
	c.Equal(0, asked, "without the table being asked")
	c.Equal(1, table.LeadColumnIndex(), "and the cell cursor left where it was")
}

// TestContextMenuCellRequestRefusedLeavesThePress verifies that an assistive technology's request for the menu of a
// widget in a table cell that is going to be refused (the widget offers no menu, cannot take the focus or is gone
// from the cell) changes nothing, leaving a mouse press still held in progress, where a request that is carried out
// ends the press before the menu opens.
func TestContextMenuCellRequestRefusedLeavesThePress(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	activateForContextMenu(t, w)
	rows := []*focusCellRow{newFocusCellRow("a", 1)}
	model := &SimpleTableModel[*focusCellRow]{}
	model.SetRootRows(rows)
	table := NewTable[*focusCellRow](model)
	table.Columns = []ColumnInfo{{ID: 0, Current: 100}, {ID: 1, Current: 100}}
	table.SetFrameRect(geom.NewRect(0, 0, 200, 60))
	table.UpdateCursorCallback = nil
	// The table has a menu of its own, so that the last check can tell it is never asked for it in the widget's place.
	var tableAsked int
	table.ContextMenuCallback = func(geom.Point) Menu {
		tableAsked++
		return nil
	}
	w.root.contentPanel.AddChild(table)
	table.SyncToModel()
	holder := NewPanel()
	holder.SetFrameRect(geom.NewRect(0, 100, 100, 40))
	holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
	w.root.contentPanel.AddChild(holder)
	ask := func(path string) bool {
		return table.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu, Key: axCellPanelKey{
			Cell: accessibility.CellKey{Row: tid.TID("a"), Col: 0}, Path: path,
		}}, true)
	}
	cell := table.cell(0, 0)
	var asked int

	w.mouseDown(geom.NewPoint(50, 120), ButtonLeft, 0)
	c.True(w.lastMouseDownPanel == holder, "the test needs the holder to hold the press")
	c.False(ask(""), "a widget with no menu is refused")
	c.True(w.inMouseDown, "with the press left alone")
	c.True(w.lastMouseDownPanel == holder)
	c.Nil(w.CurrentFocus(), "and the focus left where it was")

	cell.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	cell.SetFocusable(false)
	c.False(ask(""), "a widget that cannot take the focus is refused")
	c.True(w.inMouseDown, "with the press left alone")
	c.Equal(0, asked, "and the widget not asked")
	cell.SetFocusable(true)

	c.False(ask("3"), "a panel gone from the cell is refused")
	c.True(w.inMouseDown, "with the press left alone")
	c.Equal(0, asked)

	c.False(ask(""), "the callback offers nothing, so nothing is shown")
	c.Equal(1, asked, "but the widget was asked")
	c.False(w.inMouseDown, "and the press was ended first")
	c.True(w.CurrentFocus() == cell, "with the widget given the focus")
	c.Equal(0, tableAsked, "the table is never asked for its own menu in the widget's place")
}

// TestContextMenuKeyInATransientWindowLeavesThePress verifies that the Menu key or shift+F10 in a window other than the
// active one, a transient window, where no menu could open, is an ordinary key: the panel holding the focus is not
// asked for its menu, and a press held on it is left in progress rather than ended for a menu that would then be
// refused.
func TestContextMenuKeyInATransientWindowLeavesThePress(t *testing.T) {
	c := check.New(t)
	front := newMouseButtonTestWindow()
	back := newMouseButtonTestWindow()
	activateForContextMenu(t, back, front)
	front.transient = true
	c.True(ActiveWindow() == back, "the test needs the transient window passed over")
	var asked, keysDown int
	owner := NewPanel()
	owner.SetFocusable(true)
	owner.SetFrameRect(geom.NewRect(10, 10, 50, 50))
	owner.ContextMenuCallback = func(geom.Point) Menu {
		asked++
		return nil
	}
	owner.KeyDownCallback = func(KeyCode, mod.Modifiers, bool) bool {
		keysDown++
		return true
	}
	front.root.contentPanel.AddChild(owner)
	var log cmMouseLog
	log.logMouse(owner)
	front.SetFocus(owner)
	c.True(front.CurrentFocus() == owner, "the test needs the panel focused")
	pt := geom.NewPoint(20, 20)
	front.mouseDown(pt, ButtonLeft, 0)
	c.True(front.lastMouseDownPanel == owner, "the test needs the panel to hold the press")
	front.keyPressed(KeyMenu, 0)
	c.Equal(0, asked, "no menu can open in a transient window, so the panel is not asked")
	c.Equal(1, keysDown, "and the key goes to the panel as any other")
	c.True(front.inMouseDown, "with the press left in progress")
	c.Equal([]string{cmDown}, log.kinds(), "no release having been delivered")
	front.keyReleased(KeyMenu, 0)
	front.keyPressed(KeyF10, mod.Shift)
	c.Equal(0, asked)
	c.Equal(2, keysDown)
	c.True(front.inMouseDown)
	front.keyReleased(KeyF10, mod.Shift)
	front.mouseUp(pt, ButtonLeft, 0)
	c.Equal([]string{cmDown, cmUp}, log.kinds(), "the release is delivered when it comes")
	c.False(front.inMouseDown)
}

// TestContextMenuFieldBehindATransientWindow verifies that a Field that is the active window's focus has its default
// menu although a transient window holds the platform focus, where Field.Focused reports it unfocused: the menu's
// commands act on the active window's focus, which the field is, and the field is offered the menu to an assistive
// technology on the same terms, so a request for it opens the menu in the field's window.
func TestContextMenuFieldBehindATransientWindow(t *testing.T) {
	c := check.New(t)
	var a, transient *Window
	var field *Field
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			a = newHeadlessTestWindow(t, "a", geom.NewRect(0, 0, 200, 150))
			if a == nil {
				return
			}
			field = NewField()
			field.SetText("hello")
			field.SetFrameRect(geom.NewRect(10, 10, 100, 24))
			a.Content().AddChild(field)
			a.ToFront()
			field.RequestFocus()
		}))
	c.NotNil(a)
	c.NotNil(field)
	var focused, offered, shown bool
	c.True(screen.Do(func() {
		focused = field.Focused()
		offered = axMayShowContextMenu(field.AsPanel())
	}))
	c.True(focused, "the test needs the field focused")
	c.True(offered, "and offered its menu")

	c.True(screen.Do(func() {
		var err error
		if transient, err = NewWindow("transient", TransientWindowOption()); err != nil {
			t.Errorf("unable to create transient window: %v", err)
			return
		}
		transient.SetContentRect(geom.NewRect(50, 50, 100, 60))
		transient.ToFront()
	}))
	c.NotNil(transient)
	c.True(screen.FocusedWindow() == transient, "the test needs the transient window to have taken the focus")
	var active *Window
	var menu Menu
	c.True(screen.Do(func() {
		active = ActiveWindow()
		focused = field.Focused()
		offered = axMayShowContextMenu(field.AsPanel())
		menu = field.DefaultContextMenu(geom.Point{})
	}))
	c.True(active == a, "which leaves the field's window the active one")
	c.False(focused, "where Focused reports the field unfocused while the transient window holds the focus")
	c.True(offered, "the field is still offered its menu, being the active window's focus")
	c.NotNil(menu, "and its default menu is built")
	var menus int
	c.True(screen.Do(func() {
		if menu != nil {
			menu.Dispose()
		}
		shown = field.axDispatchAction(accessibility.ActionRequest{Action: accessibility.ShowContextMenu}, false)
		menus = len(a.root.openMenuPanels)
	}))
	c.True(shown, "so a request for the menu opens it")
	c.Equal(1, menus, "in the field's window")
	c.True(screen.Do(func() { a.keyPressed(KeyEscape, 0) }))
	c.True(screen.Do(func() { menus = len(a.root.openMenuPanels) }))
	c.Equal(0, menus, "Escape closes it")
}

// TestContextMenuFocusedCellRequestFollowsTheCellRule verifies that an assistive technology's request for the
// contextual menu of a panel of the cell holding the keyboard focus, which is sent to the panel itself since it is
// described under its own id, is carried out only when the table would carry it out for a panel of any other cell,
// whatever was last published: refused for a panel that cannot take the focus and holds nothing that can, and for any
// panel while the table is disabled. A request about to be refused leaves a mouse press still held alone, where one
// that is carried out ends it first, and one whose release disables the table is refused after all.
func TestContextMenuFocusedCellRequestFollowsTheCellRule(t *testing.T) {
	c := check.New(t)
	var table *Table[*focusCellRow]
	var field *Field
	var label, holder *Label
	var wrapper *Panel
	var wnd *Window
	var fieldAsked, labelAsked int
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			rows := []*focusCellRow{newFocusCellRow("a", 0), newFocusCellRow("b", 0)}
			field = NewField()
			field.SetText("abc")
			field.ContextMenuCallback = func(geom.Point) Menu {
				fieldAsked++
				return nil
			}
			label = NewLabel()
			label.SetTitle("beside")
			label.ContextMenuCallback = func(geom.Point) Menu {
				labelAsked++
				return nil
			}
			wrapper = NewPanel()
			wrapper.SetLayout(&FlexLayout{Columns: 2, HSpacing: 4})
			wrapper.AddChild(field)
			wrapper.AddChild(label)
			// Seeded before the row is first asked for the cell, so that the row hands the wrapper back for it.
			rows[1].cells[0] = wrapper
			model := &SimpleTableModel[*focusCellRow]{}
			model.SetRootRows(rows)
			table = NewTable[*focusCellRow](model)
			table.Columns = []ColumnInfo{{ID: 0, Current: 160}, {ID: 1, Current: 60}, {ID: 2, Current: 40}}
			table.ContextMenuCallback = func(geom.Point) Menu { return nil }
			table.SyncToModel()
			holder = NewLabel()
			holder.SetTitle("holder")
			holder.MouseDownCallback = func(geom.Point, int, int, mod.Modifiers) bool { return true }
			column := NewPanel()
			column.SetLayout(&FlexLayout{Columns: 1, VSpacing: StdVSpacing})
			column.AddChild(table)
			column.AddChild(holder)
			wnd = axNewTestWindow(t, "focused cell", geom.NewRect(10, 10, 300, 280), column)
			if wnd != nil {
				// A window that was merely shown holds no focus, and no menu opens in any but the active window.
				wnd.ToFront()
			}
		}))
	c.NotNil(table)
	c.NotNil(wnd)
	if table == nil || wnd == nil {
		return
	}
	var focused, attached bool
	screen.Do(func() {
		table.FocusCell(1, 0)
		focused = field.Focused()
		attached = wrapper.Parent() == table.AsPanel()
	})
	c.True(focused, "the test needs the field focused")
	c.True(attached, "with its cell attached to the table")
	screen.AccessibilityTree(wnd)
	fieldNode := screen.AccessibilityNodeFor(field)
	labelNode := screen.AccessibilityNodeFor(label)
	c.NotNil(fieldNode, "the field is described under its own id")
	c.NotNil(labelNode, "and so is the label beside it")
	if fieldNode == nil || labelNode == nil {
		return
	}
	c.True(fieldNode.Actions.Has(accessibility.ShowContextMenu), "the field in the focused cell offers its menu")
	c.False(labelNode.Actions.Has(accessibility.ShowContextMenu),
		"the label, which cannot take the focus, does not offer its own, as it would not in any other cell")
	ask := func(node *accessibility.Node) (handled, inMouseDown bool) {
		screen.Do(func() {
			handled = wnd.performAccessibilityAction(accessibility.ActionRequest{
				Node:   node.ID,
				Action: accessibility.ShowContextMenu,
			})
			inMouseDown = wnd.inMouseDown
		})
		return handled, inMouseDown
	}

	// The request the window itself is handed, past the check an assistive technology's adapter makes against what
	// was published, is refused for the label, as the table refuses it for a label in any other cell.
	handled, _ := ask(labelNode)
	c.False(handled, "a request for the label's menu is refused")
	c.Equal(0, labelAsked, "and the label is not asked")
	handled, _ = ask(fieldNode)
	c.False(handled, "the field's callback offers nothing, so nothing is shown")
	c.Equal(1, fieldAsked, "but the request asks the field for its menu")
	screen.Do(func() { focused = field.Focused() })
	c.True(focused, "which keeps the focus")

	// A request for the field in the focused cell of a table that has been disabled since the window was described is
	// refused, as it is for a field in any other cell, and leaves a press still held alone.
	holderPoint := screen.PanelPoint(holder, holder.ContentRect(false).Center())
	screen.Do(func() { table.SetEnabled(false) })
	screen.MouseDown(holderPoint, ButtonLeft, mod.None)
	var pressed bool
	screen.Do(func() { pressed = wnd.lastMouseDownPanel == holder.AsPanel() })
	c.True(pressed, "the test needs the holder to hold the press")
	handled, inMouseDown := ask(fieldNode)
	c.False(handled, "a request for the field's menu is refused while the table is disabled")
	c.Equal(1, fieldAsked, "without the field being asked")
	c.True(inMouseDown, "and the press is left alone")
	screen.MouseUp(holderPoint, ButtonLeft, mod.None)

	// The release that ends a press for the menu disables the table: the request is refused after all, since what the
	// release did is the same as what a table disabled before the request was made.
	screen.Do(func() {
		table.SetEnabled(true)
		holder.MouseUpCallback = func(geom.Point, int, mod.Modifiers) bool {
			table.SetEnabled(false)
			return true
		}
	})
	screen.MouseDown(holderPoint, ButtonLeft, mod.None)
	screen.Do(func() { pressed = wnd.lastMouseDownPanel == holder.AsPanel() })
	c.True(pressed, "the test needs the holder to hold the press")
	handled, inMouseDown = ask(fieldNode)
	c.False(handled, "a request whose release disabled the table is refused")
	c.Equal(1, fieldAsked, "without the field being asked")
	c.False(inMouseDown, "though the press was ended for it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
