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
	"net/url"
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// newCursorTestWindow returns a minimal Window suitable for exercising cursor updates without a live windowing system.
// The window deliberately reports itself as invalid so MouseLocation returns the origin instead of querying the
// platform, and updateCursor skips the platform cursor-image call after recording the new cursor.
func newCursorTestWindow() *Window {
	w := &Window{}
	w.root = newRootPanel(w)
	return w
}

func TestUpdateCursorConvertsRootPointForCallbackOwner(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0, 20, 200, 200))
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(30, 40, 100, 100))
	content.AddChild(child)
	var got []geom.Point
	child.UpdateCursorCallback = func(where geom.Point) *Cursor {
		got = append(got, where)
		return &Cursor{}
	}
	// Root point (50,70) is content-local (50,50) and child-local (20,10).
	w.updateCursor(child.AsPanel(), geom.NewPoint(50, 70))
	c.Equal([]geom.Point{geom.NewPoint(20, 10)}, got)
}

func TestUpdateCursorWalksUpToAncestorWithCallback(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0, 20, 200, 200))
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(30, 40, 100, 100))
	content.AddChild(child)
	var got []geom.Point
	content.UpdateCursorCallback = func(where geom.Point) *Cursor {
		got = append(got, where)
		return &Cursor{}
	}
	// The leaf has no callback, so updateCursor must walk up to the content panel and convert the root point into that
	// panel's coordinates, not the leaf's.
	w.updateCursor(child.AsPanel(), geom.NewPoint(50, 70))
	c.Equal([]geom.Point{geom.NewPoint(50, 50)}, got)
}

func TestUpdateCursorNowDeliversTargetLocalPointExactlyOnce(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	// The invalid test window reports the mouse at the origin, so the target panel is given a negative origin — as a
	// scrolled panel would have — to make its offset from root non-zero. A double conversion would report (50,30)
	// instead of the correct (25,15).
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(-25, -15, 100, 100))
	content.AddChild(child)
	var got []geom.Point
	child.UpdateCursorCallback = func(where geom.Point) *Cursor {
		got = append(got, where)
		return &Cursor{}
	}
	w.UpdateCursorNow()
	c.Equal([]geom.Point{geom.NewPoint(25, 15)}, got)
}

// dropTargetTestDragInfo is a minimal drag.Info for exercising drop-target resolution without a live drag session.
type dropTargetTestDragInfo struct{}

func (d *dropTargetTestDragInfo) SourceDragOpMask() drag.Op { return drag.Copy }
func (d *dropTargetTestDragInfo) DataTypes() []string       { return nil }
func (d *dropTargetTestDragInfo) HasString() bool           { return false }
func (d *dropTargetTestDragInfo) HasFilePaths() bool        { return false }
func (d *dropTargetTestDragInfo) HasURLs() bool             { return false }
func (d *dropTargetTestDragInfo) HasDataType(_ string) bool { return false }
func (d *dropTargetTestDragInfo) Text() string              { return "" }
func (d *dropTargetTestDragInfo) FilePaths() []string       { return nil }
func (d *dropTargetTestDragInfo) URLs() []*url.URL          { return nil }
func (d *dropTargetTestDragInfo) Data(_ string) []byte      { return nil }

// newDropTargetTestWindow returns a minimal Window suitable for exercising drop-target resolution without a live
// windowing system, along with a child panel covering the left half of the content area.
func newDropTargetTestWindow() (w *Window, child *Panel) {
	w = &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.root = newRootPanel(w)
	w.root.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	w.root.contentPanel.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	child = NewPanel()
	child.SetFrameRect(geom.NewRect(0, 0, 100, 200))
	w.root.contentPanel.AddChild(child)
	return w, child
}

// TestFindDropTargetWithNilCanAcceptDropCallback is the regression test for the documented contract that a panel with
// a DropCallback and no CanAcceptDropCallback is treated as a drop candidate.
func TestFindDropTargetWithNilCanAcceptDropCallback(t *testing.T) {
	c := check.New(t)
	w, child := newDropTargetTestWindow()
	child.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
	target := w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100))
	c.NotNil(target)
	c.True(child.Is(target), "the panel with only a DropCallback must be selected as the drop target")
}

// TestFindDropTargetHonorsCanAcceptDropCallback verifies that an explicit CanAcceptDropCallback still governs
// candidacy: true selects the panel, false declines it so the search continues up the parent hierarchy.
func TestFindDropTargetHonorsCanAcceptDropCallback(t *testing.T) {
	c := check.New(t)
	w, child := newDropTargetTestWindow()
	child.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
	child.CanAcceptDropCallback = func(_ drag.Info) bool { return true }
	target := w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100))
	c.NotNil(target)
	c.True(child.Is(target))

	// Declining must pass the drag to an enclosing candidate — here the content panel, which relies on the
	// nil-callback default.
	child.CanAcceptDropCallback = func(_ drag.Info) bool { return false }
	content := w.root.contentPanel.AsPanel()
	content.DropCallback = func(_ drag.Info, _ geom.Point, _ mod.Modifiers) bool { return true }
	target = w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100))
	c.NotNil(target)
	c.True(content.Is(target), "a declined panel must yield to an enclosing panel with a DropCallback")
}

// TestFindDropTargetWithoutDropCallback verifies that panels without a DropCallback are never candidates, regardless
// of the CanAcceptDropCallback default.
func TestFindDropTargetWithoutDropCallback(t *testing.T) {
	c := check.New(t)
	w, _ := newDropTargetTestWindow()
	c.Nil(w.findDropTarget(&dropTargetTestDragInfo{}, geom.NewPoint(50, 100)))
}

// These tests exercise the bookkeeping behind ToFront() with hand-built windows that have no platform backing, so
// they must never let a request reach the platform: flushPendingFront is only ever called for a window that it will
// decline to activate. The empty event requestFront posts is harmless, since every backend ignores it before Start()
// has run.

// saveFrontState restores the package-level window state the tests below alter once the test is over.
func saveFrontState(t *testing.T) {
	t.Helper()
	savedList := windowList
	savedPending := pendingFrontWindow
	t.Cleanup(func() {
		windowList = savedList
		pendingFrontWindow = savedPending
	})
}

// TestFrontRequestsCoalesce verifies that requests made within a pass collapse into the most recent one, which is what
// lets a menu closing (which fronts the window underneath) followed by a dialog opening activate only the dialog.
func TestFrontRequestsCoalesce(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	b := newModalInputTestWindow()
	pendingFrontWindow = nil
	requestFront(a)
	c.True(pendingFrontWindow == a)
	requestFront(b)
	c.True(pendingFrontWindow == b, "the most recent request should replace the earlier one")
	requestFront(b)
	c.True(pendingFrontWindow == b)
}

// TestFlushPendingFrontDropsDisposedWindow verifies that a request outstanding for a window that has since been
// disposed of is discarded rather than performed. Reaching the platform for a hand-built window would panic, which is
// the assertion.
func TestFlushPendingFrontDropsDisposedWindow(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	pendingFrontWindow = nil
	requestFront(a)
	a.valid = false
	flushPendingFront()
	c.Nil(pendingFrontWindow, "the request should have been cleared")
	flushPendingFront() // nothing pending: must be a no-op
	c.Nil(pendingFrontWindow)
}

// TestFlushPendingFrontHonorsKeepHidden verifies that a window marked keepHidden is never activated, matching the
// check ToFront() itself makes, since the mark may be applied after the request was made.
func TestFlushPendingFrontHonorsKeepHidden(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	pendingFrontWindow = nil
	requestFront(a)
	a.keepHidden = true
	flushPendingFront()
	c.Nil(pendingFrontWindow)
}

// TestActiveWindowPrefersPendingFront verifies that a window whose activation is still outstanding is reported as the
// active one, so that anything positioned or parented relative to the active window during the same pass — a dialog
// centering itself, for instance — uses the window that is about to hold the focus rather than the one that still
// does. A transient window is never reported, matching the treatment of focused transient windows.
func TestActiveWindowPrefersPendingFront(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	a := newModalInputTestWindow()
	b := newModalInputTestWindow()
	a.focused = true
	windowList = []*Window{a, b}
	pendingFrontWindow = nil
	c.True(ActiveWindow() == a)
	pendingFrontWindow = b
	c.True(ActiveWindow() == b, "the window about to be activated should be reported as active")
	b.transient = true
	c.True(ActiveWindow() == a, "a transient window is never the active window")
	b.transient = false
	b.valid = false
	c.True(ActiveWindow() == a, "a disposed window is never the active window")
}

// newModalInputTestWindow returns a minimal Window suitable for exercising input routing without a live windowing
// system. The window reports itself as valid, but has no platform resources, so tests must only drive code paths that
// check validity before touching platform APIs.
func newModalInputTestWindow() *Window {
	w := &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.valid = true
	w.root = newRootPanel(w)
	return w
}

// pushTestModal installs the given window as the top of the modal stack for the duration of the test, restoring the
// previous stack when the test completes.
func pushTestModal(t *testing.T, w *Window) {
	t.Helper()
	saved := modalStack
	modalStack = append(append([]*Window{}, saved...), w)
	t.Cleanup(func() { modalStack = saved })
}

func TestKeyPressedOnBlockedWindowRoutesToModal(t *testing.T) {
	c := check.New(t)
	blocked := newModalInputTestWindow()
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	var blockedKeys, modalKeys []KeyCode
	blocked.KeyDownCallback = func(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
		blockedKeys = append(blockedKeys, keyCode)
		return true
	}
	modal.KeyDownCallback = func(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
		modalKeys = append(modalKeys, keyCode)
		return true
	}
	blocked.keyPressed(KeyA, 0)
	c.Equal(0, len(blockedKeys))
	c.Equal([]KeyCode{KeyA}, modalKeys)
	c.False(blocked.pressedKeys[KeyA])
	c.True(modal.pressedKeys[KeyA])
}

func TestKeyPressedOnTopModalIsProcessed(t *testing.T) {
	c := check.New(t)
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	var keys []KeyCode
	modal.KeyDownCallback = func(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
		keys = append(keys, keyCode)
		return true
	}
	modal.keyPressed(KeyA, 0)
	c.Equal([]KeyCode{KeyA}, keys)
	c.True(modal.pressedKeys[KeyA])
}

func TestRuneTypedOnBlockedWindowRoutesToModal(t *testing.T) {
	c := check.New(t)
	blocked := newModalInputTestWindow()
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	var blockedRunes, modalRunes []rune
	blocked.RuneTypedCallback = func(ch rune) bool {
		blockedRunes = append(blockedRunes, ch)
		return true
	}
	modal.RuneTypedCallback = func(ch rune) bool {
		modalRunes = append(modalRunes, ch)
		return true
	}
	blocked.runeTyped('x')
	c.Equal(0, len(blockedRunes))
	c.Equal([]rune{'x'}, modalRunes)
}

func TestKeyReleasedOnBlockedWindowRoutesToModalAndClearsLocalState(t *testing.T) {
	c := check.New(t)
	blocked := newModalInputTestWindow()
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	var blockedKeys, modalKeys []KeyCode
	blocked.KeyUpCallback = func(keyCode KeyCode, _ mod.Modifiers) bool {
		blockedKeys = append(blockedKeys, keyCode)
		return true
	}
	modal.KeyUpCallback = func(keyCode KeyCode, _ mod.Modifiers) bool {
		modalKeys = append(modalKeys, keyCode)
		return true
	}
	// Simulate a key that was pressed in each window before checking release routing. The blocked window's entry must
	// be cleared even though delivery is routed to the modal, so releases synthesized by lostFocus cannot leave stale
	// pressed key state behind.
	blocked.pressedKeys[KeyA] = true
	modal.pressedKeys[KeyA] = true
	blocked.keyReleased(KeyA, 0)
	c.Equal(0, len(blockedKeys))
	c.Equal([]KeyCode{KeyA}, modalKeys)
	c.False(blocked.pressedKeys[KeyA])
	c.False(modal.pressedKeys[KeyA])
}

func TestKeyReleasedForwardedToModalWithoutMatchingDownIsNotDispatched(t *testing.T) {
	c := check.New(t)
	blocked := newModalInputTestWindow()
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	var modalKeys []KeyCode
	modal.KeyUpCallback = func(keyCode KeyCode, _ mod.Modifiers) bool {
		modalKeys = append(modalKeys, keyCode)
		return true
	}
	// The key went down in the blocked window before the modal opened, so the modal never saw the press. The release
	// (here as synthesized by lostFocus) must clear the blocked window's pressed state without delivering a KeyUp to
	// the modal for a key it never saw pressed.
	blocked.pressedKeys[KeyA] = true
	blocked.keyReleased(KeyA, 0)
	c.Equal(0, len(modalKeys))
	c.False(blocked.pressedKeys[KeyA])
}

func TestKeyReleasedWithoutMatchingDownIsIgnored(t *testing.T) {
	c := check.New(t)
	w := newModalInputTestWindow()
	ups := 0
	w.KeyUpCallback = func(_ KeyCode, _ mod.Modifiers) bool {
		ups++
		return true
	}
	w.keyReleased(KeyA, 0)
	c.Equal(0, ups)
}

// TestMouseUpWhileBlockedByModalClearsDragState is the regression test for the stuck-drag bug: when a modal opens
// while a mouse button is held (e.g. a mouse-down handler calls RunModal, or a background task raises a dialog
// mid-drag), the release arrives while the window is blocked. Delivery must be suppressed, but the pressed-button
// bookkeeping must still run — otherwise mouseMovedOrDragged, which has no modal gate, keeps feeding drag events to
// lastMouseDownPanel for the modal's entire lifetime.
func TestMouseUpWhileBlockedByModalClearsDragState(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	content := w.root.contentPanel
	ups := 0
	drags := 0
	content.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
	content.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		ups++
		return true
	}
	content.MouseDragCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		drags++
		return true
	}
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonLeft, 0)
	c.True(w.inMouseDown)
	c.NotNil(w.lastMouseDownPanel)

	// A modal opens while the button is held, then lostFocus synthesizes the release while the window is blocked.
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	w.synthesizeMouseUp()
	c.Equal(0, ups, "the release must not be delivered while blocked by a modal")
	c.False(w.inMouseDown)
	c.Equal(0, len(w.pressedButtons))
	c.Nil(w.lastMouseDownPanel)

	// With the state clean, subsequent motion over the blocked window must not be treated as a drag.
	w.mouseMovedOrDragged(geom.NewPoint(20, 20), 0)
	c.Equal(0, drags)
}

func TestMouseWheelOnBlockedWindowIsStillDelivered(t *testing.T) {
	c := check.New(t)
	blocked := newModalInputTestWindow()
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	blockedCount := 0
	modalCount := 0
	blocked.MouseWheelCallback = func(_, _ geom.Point, _ mod.Modifiers) bool {
		blockedCount++
		return true
	}
	modal.MouseWheelCallback = func(_, _ geom.Point, _ mod.Modifiers) bool {
		modalCount++
		return true
	}
	// Wheel events arrive for the window under the cursor, not the focused window, and scrolling only adjusts the
	// view, so a window blocked by a modal still processes them itself rather than having them gated or rerouted.
	blocked.mouseWheel(geom.NewPoint(10, 10), geom.NewPoint(0, 1), 0)
	c.Equal(1, blockedCount)
	c.Equal(0, modalCount)
}

func TestMouseEventsOnBlockedWindowAreIgnored(t *testing.T) {
	c := check.New(t)
	blocked := newModalInputTestWindow()
	modal := newModalInputTestWindow()
	pushTestModal(t, modal)
	blockedDowns, modalDowns, blockedUps, modalUps := 0, 0, 0, 0
	blocked.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
		blockedDowns++
		return true
	}
	modal.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
		modalDowns++
		return true
	}
	blocked.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		blockedUps++
		return true
	}
	modal.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		modalUps++
		return true
	}
	// Mouse events are positional, so unlike keyboard events they must not be rerouted to the modal: the coordinates
	// are in the blocked window's space and would land on arbitrary panels in the modal. The blocked window must also
	// not record any pressed-button state of its own, and the modal's state must be left untouched.
	where := geom.NewPoint(10, 10)
	blocked.mouseDown(where, ButtonLeft, 0)
	blocked.mouseUp(where, ButtonLeft, 0)
	c.Equal(0, blockedDowns)
	c.Equal(0, blockedUps)
	c.Equal(0, modalDowns)
	c.Equal(0, modalUps)
	c.False(blocked.inMouseDown)
	c.Equal(0, len(blocked.pressedButtons))
	c.False(modal.inMouseDown)
	c.Equal(0, len(modal.pressedButtons))
}

func TestKeyEventsProcessedNormallyWithoutModal(t *testing.T) {
	c := check.New(t)
	w := newModalInputTestWindow()
	var downKeys, upKeys []KeyCode
	var runes []rune
	w.KeyDownCallback = func(keyCode KeyCode, _ mod.Modifiers, _ bool) bool {
		downKeys = append(downKeys, keyCode)
		return true
	}
	w.RuneTypedCallback = func(ch rune) bool {
		runes = append(runes, ch)
		return true
	}
	w.KeyUpCallback = func(keyCode KeyCode, _ mod.Modifiers) bool {
		upKeys = append(upKeys, keyCode)
		return true
	}
	w.keyPressed(KeyA, 0)
	w.runeTyped('a')
	w.keyReleased(KeyA, 0)
	c.Equal([]KeyCode{KeyA}, downKeys)
	c.Equal([]rune{'a'}, runes)
	c.Equal([]KeyCode{KeyA}, upKeys)
}

// newMouseButtonTestWindow returns a minimal Window suitable for exercising mouse button bookkeeping without a live
// windowing system. The window deliberately reports itself as invalid so MouseLocation returns the origin instead of
// querying the platform, and cursor updates skip the platform cursor-image call. The content panel is given a frame
// covering the window and a cursor callback so the post-release cursor update never has to fall back to the
// platform-backed ArrowCursor. The window claims the focus so mouse downs are dispatched to panels.
func newMouseButtonTestWindow() *Window {
	w := &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.root = newRootPanel(w)
	w.root.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	w.root.contentPanel.SetFrameRect(geom.NewRect(0, 0, 200, 200))
	w.root.contentPanel.UpdateCursorCallback = func(_ geom.Point) *Cursor { return &Cursor{} }
	w.focused = true
	return w
}

// TestSecondButtonReleaseIsDelivered is the regression test for multi-button presses: with two buttons down, the first
// release must not end the whole mouse-down interaction, and the second release must still reach the panel's
// MouseUpCallback.
func TestSecondButtonReleaseIsDelivered(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	content := w.root.contentPanel
	var downs, ups []int
	content.MouseDownCallback = func(_ geom.Point, button, _ int, _ mod.Modifiers) bool {
		downs = append(downs, button)
		return true
	}
	content.MouseUpCallback = func(_ geom.Point, button int, _ mod.Modifiers) bool {
		ups = append(ups, button)
		return true
	}
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseDown(pt, ButtonRight, 0)
	c.Equal([]int{ButtonLeft, ButtonRight}, downs)
	c.True(w.inMouseDown)
	c.True(w.pressedButtons[ButtonLeft])
	c.True(w.pressedButtons[ButtonRight])

	w.mouseUp(pt, ButtonLeft, 0)
	c.Equal([]int{ButtonLeft}, ups)
	c.True(w.inMouseDown, "the interaction must continue while another button remains down")
	c.NotNil(w.lastMouseDownPanel, "the drag target must be retained until the last button is released")

	w.mouseUp(pt, ButtonRight, 0)
	c.Equal([]int{ButtonLeft, ButtonRight}, ups)
	c.False(w.inMouseDown)
	c.Equal(0, len(w.pressedButtons), "released buttons must be removed from the pressed set, not just marked false")
	c.Nil(w.lastMouseDownPanel)
}

// TestMouseUpWithoutMatchingDownIsIgnored verifies that a release with no matching press — e.g. a click begun before
// the window existed — is dropped rather than dispatched.
func TestMouseUpWithoutMatchingDownIsIgnored(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	ups := 0
	w.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		ups++
		return true
	}
	w.mouseUp(geom.NewPoint(10, 10), ButtonLeft, 0)
	c.Equal(0, ups)
}

// TestSynthesizeMouseUpReleasesAllPressedButtons verifies that the synthetic releases used on focus loss and at drag
// start cover every pressed button and leave the pressed set empty.
func TestSynthesizeMouseUpReleasesAllPressedButtons(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	var ups []int
	w.MouseUpCallback = func(_ geom.Point, button int, _ mod.Modifiers) bool {
		ups = append(ups, button)
		return true
	}
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseDown(pt, ButtonRight, 0)
	w.synthesizeMouseUp()
	c.Equal(2, len(ups))
	c.True(slices.Contains(ups, ButtonLeft))
	c.True(slices.Contains(ups, ButtonRight))
	c.False(w.inMouseDown)
	c.Equal(0, len(w.pressedButtons))
}

// TestCancelPressesForModalEndsPressWithoutDelivery verifies the cancel-mode step RunModal performs before its loop:
// a press in progress in another window is ended, but the synthesized release is not delivered to the panels, since a
// button under the pointer would otherwise fire its click again and recurse into RunModal.
func TestCancelPressesForModalEndsPressWithoutDelivery(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	savedList := windowList
	windowList = []*Window{w}
	t.Cleanup(func() { windowList = savedList })
	content := w.root.contentPanel
	content.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
	ups := 0
	content.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		ups++
		return true
	}
	w.mouseDown(geom.NewPoint(10, 10), ButtonLeft, 0)
	c.True(w.inMouseDown)
	c.NotNil(w.lastMouseDownPanel)

	modal := newMouseButtonTestWindow()
	pushTestModal(t, modal)
	cancelPressesForModal(modal)
	c.Equal(0, ups, "the synthesized release must not reach the panels")
	c.False(w.inMouseDown)
	c.Equal(0, len(w.pressedButtons))
	c.Nil(w.lastMouseDownPanel)
	c.Equal(0, len(modal.pressedButtons), "the modal window itself is left alone")
}

// TestUpdateTooltipPassesAlignedAvoidRect verifies that updateTooltip hands UpdateTooltipCallback — and, through it,
// the tooltip sequencer — a pixel-aligned rect to avoid. geom.Rect.Align has a value receiver, so calling it without
// keeping its result left the avoid rect at whatever fractional position the panel happened to occupy, placing
// tooltips at fractional pixel positions.
func TestUpdateTooltipPassesAlignedAvoidRect(t *testing.T) {
	c := check.New(t)
	w := newCursorTestWindow()
	content := w.root.contentPanel
	content.SetFrameRect(geom.NewRect(0.25, 20.75, 200, 200))
	child := NewPanel()
	child.SetFrameRect(geom.NewRect(30.5, 40.25, 100.5, 100.25))
	content.AddChild(child)

	var got geom.Rect
	called := 0
	child.UpdateTooltipCallback = func(_ geom.Point, avoid geom.Rect) geom.Rect {
		called++
		got = avoid
		return avoid
	}
	w.updateTooltip(child.AsPanel(), geom.NewPoint(50, 70))
	c.Equal(1, called)

	unaligned := child.RectToRoot(child.ContentRect(true))
	c.NotEqual(unaligned, unaligned.Align(), "the panel must sit at a fractional position for this test to mean anything")
	c.Equal(unaligned.Align(), got)
}

// TestModifierKeyDownKeepsTooltip verifies that pressing a modifier on its own leaves a showing tooltip in place, while
// any other key still clears it.
func TestModifierKeyDownKeepsTooltip(t *testing.T) {
	c := check.New(t)
	w := newModalInputTestWindow()
	tip := NewPanel()
	modifiers := []KeyCode{
		KeyLShift, KeyRShift, KeyLControl, KeyRControl, KeyLOption, KeyROption, KeyLCommand, KeyRCommand,
	}
	for _, key := range modifiers {
		w.root.setTooltip(tip)
		w.keyPressed(key, mod.Shift|mod.Command)
		w.keyReleased(key, 0)
		c.True(w.root.tooltipPanel == tip, key.String())
	}
	w.keyPressed(KeyS, mod.Shift|mod.Command)
	c.True(w.root.tooltipPanel == nil)
}

// swapRedrawSet installs a fresh, empty redrawSet for the duration of the test, restoring the previous set when the
// test completes, so these tests neither see nor disturb redraw state left behind by other tests in the package.
func swapRedrawSet(t *testing.T) {
	t.Helper()
	saved := redrawSet
	redrawSet = make(map[*Window]struct{})
	t.Cleanup(func() { redrawSet = saved })
}

// preventQuitOnLastWindowClosed keeps Dispose from invoking quitting() (which would terminate the test process) when
// the window list is empty, restoring the previous callback when the test completes.
func preventQuitOnLastWindowClosed(t *testing.T) {
	t.Helper()
	saved := quitAfterLastWindowClosedCallback
	quitAfterLastWindowClosedCallback = func() bool { return false }
	t.Cleanup(func() { quitAfterLastWindowClosedCallback = saved })
}

// newRedrawTestWindow returns a minimal Window suitable for exercising the redraw bookkeeping without a live windowing
// system. The window reports itself as valid, but has no platform resources, so tests must only drive code paths that
// check validity before touching platform APIs.
func newRedrawTestWindow() *Window {
	w := &Window{
		wnd:            &apiWindow{},
		glCtx:          &apiGLContext{},
		surface:        &surface{},
		pressedKeys:    make(map[KeyCode]bool),
		pressedButtons: make(map[int]bool),
	}
	w.valid = true
	w.root = newRootPanel(w)
	return w
}

// TestCloseRemovesPendingRedraw covers the leak where a window disposed while a redraw was pending was retained in
// redrawSet forever: e.g. a click handler that marks panels for redraw and then calls AttemptClose(). The valid flag is
// cleared before closing so Dispose skips the platform teardown, which cannot run in a headless test; the redrawSet
// removal it performs is unconditional and is what this test verifies.
func TestCloseRemovesPendingRedraw(t *testing.T) {
	c := check.New(t)
	preventQuitOnLastWindowClosed(t)
	w := newRedrawTestWindow()
	swapRedrawSet(t)
	w.MarkForRedraw()
	_, pending := redrawSet[w]
	c.True(pending, "window should have a pending redraw after MarkForRedraw")
	w.valid = false
	c.True(w.AttemptClose(), "AttemptClose should succeed without an AllowCloseCallback")
	_, pending = redrawSet[w]
	c.False(pending, "disposing a window must remove its pending redraw request")
}

// TestFinishProcessingEventsDropsDisposedWindows verifies that the redraw pass discards windows that were disposed
// while a redraw was pending, rather than re-queuing them on every pass forever.
func TestFinishProcessingEventsDropsDisposedWindows(t *testing.T) {
	c := check.New(t)
	w := newRedrawTestWindow()
	swapRedrawSet(t)
	w.MarkForRedraw()
	w.valid = false // Simulate disposal without platform teardown.
	finishProcessingEvents()
	_, pending := redrawSet[w]
	c.False(pending, "the redraw pass must not re-queue a disposed window")
	c.Equal(0, len(redrawSet))
}

// TestMarkForRedrawIgnoresDisposedWindow verifies that a disposed window can never re-enter redrawSet, since it can
// never be drawn again.
func TestMarkForRedrawIgnoresDisposedWindow(t *testing.T) {
	c := check.New(t)
	w := newRedrawTestWindow()
	w.valid = false
	swapRedrawSet(t)
	w.MarkForRedraw()
	c.Equal(0, len(redrawSet), "MarkForRedraw on a disposed window must be a no-op")
}

func newFocusablePanel() *Panel {
	p := NewPanel()
	p.SetFocusable(true)
	return p
}

// TestCollectFocusablesSkipsHiddenSubtrees verifies that tab traversal does not descend into hidden containers, since
// focusable descendants of a hidden ancestor are not visible even though their own Hidden flag is false. This mirrors
// the dock container case, where non-current dockables are hidden at the container level only.
func TestCollectFocusablesSkipsHiddenSubtrees(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	visible := newFocusablePanel()
	root.AddChild(visible)
	hiddenContainer := NewPanel()
	hiddenContainer.Hidden = true
	buried := newFocusablePanel()
	hiddenContainer.AddChild(buried)
	root.AddChild(hiddenContainer)
	match, focusables := collectFocusables(root, visible, nil)
	c.Equal(1, len(focusables))
	c.True(focusables[0].Is(visible))
	c.Equal(0, match)

	// Unhiding the container makes its focusable descendant reachable again.
	hiddenContainer.Hidden = false
	match, focusables = collectFocusables(root, buried, nil)
	c.Equal(2, len(focusables))
	c.True(focusables[1].Is(buried))
	c.Equal(1, match)
}

// TestCollectFocusablesSkipsDirectlyHiddenPanels verifies that a hidden focusable panel is excluded even when its
// parent is visible.
func TestCollectFocusablesSkipsDirectlyHiddenPanels(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	hidden := newFocusablePanel()
	hidden.Hidden = true
	root.AddChild(hidden)
	visible := newFocusablePanel()
	root.AddChild(visible)
	match, focusables := collectFocusables(root, visible, nil)
	c.Equal(1, len(focusables))
	c.True(focusables[0].Is(visible))
	c.Equal(0, match)
}

// TestFirstFocusableChildSkipsHiddenSubtrees verifies that FirstFocusableChild neither returns a focusable panel
// buried in a hidden subtree nor a directly hidden focusable panel.
func TestFirstFocusableChildSkipsHiddenSubtrees(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	hiddenContainer := NewPanel()
	hiddenContainer.Hidden = true
	buried := newFocusablePanel()
	hiddenContainer.AddChild(buried)
	root.AddChild(hiddenContainer)
	c.Nil(root.FirstFocusableChild())
	visible := newFocusablePanel()
	root.AddChild(visible)
	c.True(root.FirstFocusableChild().Is(visible))
	hiddenContainer.Hidden = false
	c.True(root.FirstFocusableChild().Is(buried))
}

// TestLastFocusableChildSkipsHiddenSubtrees verifies that LastFocusableChild neither returns a focusable panel buried
// in a hidden subtree nor a directly hidden focusable panel.
func TestLastFocusableChildSkipsHiddenSubtrees(t *testing.T) {
	c := check.New(t)
	root := NewPanel()
	visible := newFocusablePanel()
	root.AddChild(visible)
	hiddenContainer := NewPanel()
	hiddenContainer.Hidden = true
	buried := newFocusablePanel()
	hiddenContainer.AddChild(buried)
	root.AddChild(hiddenContainer)
	c.True(root.LastFocusableChild().Is(visible))
	hiddenContainer.Hidden = false
	c.True(root.LastFocusableChild().Is(buried))
}
