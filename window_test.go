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
	"github.com/richardwilkes/unison/accessibility"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/enums/mod"
)

// ownerTitle is the title the tests give a window that owns popup windows.
const ownerTitle = "owner"

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

// TestWindowMouseUpCallbackStoppingReleaseEndsThePress verifies that a release the window's own MouseUpCallback
// consumes still ends the press, so that the window does not go on acting as if one were in progress: panel exits are
// skipped and tooltips kept while a press is held (see mouseExit and forgetTooltip), and the next release would go to
// the panel of a press long over.
func TestWindowMouseUpCallbackStoppingReleaseEndsThePress(t *testing.T) {
	c := check.New(t)
	w := newMouseButtonTestWindow()
	content := w.root.contentPanel
	content.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
	ups := 0
	content.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool {
		ups++
		return true
	}
	w.MouseUpCallback = func(_ geom.Point, _ int, _ mod.Modifiers) bool { return true }
	pt := geom.NewPoint(10, 10)
	w.mouseDown(pt, ButtonLeft, 0)
	c.NotNil(w.lastMouseDownPanel)
	w.mouseUp(pt, ButtonLeft, 0)
	c.Equal(0, ups, "the window's callback consumed the release")
	c.Nil(w.lastMouseDownPanel, "but the press is over all the same")
	w.mouseDown(pt, ButtonLeft, 0)
	w.mouseDown(pt, ButtonRight, 0)
	w.mouseUp(pt, ButtonLeft, 0)
	c.NotNil(w.lastMouseDownPanel, "a gesture with a button still down goes on")
	w.mouseUp(pt, ButtonRight, 0)
	c.Nil(w.lastMouseDownPanel)
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
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	screen.MouseMove(screen.PanelCenter(panel), mod.None)
	showing := func() bool {
		var result bool
		c.True(screen.Do(func() { result = owner.tooltipShowing() }))
		return result
	}
	c.True(showing())
	modifiers := []KeyCode{
		KeyLShift, KeyRShift, KeyLControl, KeyRControl, KeyLOption, KeyROption, KeyLCommand, KeyRCommand,
	}
	for _, key := range modifiers {
		screen.KeyDown(key, mod.Shift|mod.Command)
		screen.KeyUp(key, 0)
		c.True(showing(), key.String())
	}
	screen.KeyDown(KeyS, mod.Shift|mod.Command)
	c.False(showing())
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// newTooltipTestPanel returns a panel at frame, for a content panel with no layout, whose tooltip is tipSize and shows
// at once.
func newTooltipTestPanel(frame geom.Rect, tipSize geom.Size) *Panel {
	p := NewPanel()
	p.SetFrameRect(frame)
	tip := NewTooltipBase()
	tip.SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) { return tipSize, tipSize, tipSize })
	p.Tooltip = tip
	p.TooltipImmediate = true
	return p
}

// startTooltipTest starts a 400x300 session holding an owner window, shown and focused at (20, 20, 200, 150), whose
// content has no layout and holds the panels build returns, at the frames they were given.
func startTooltipTest(t *testing.T, build func() []*Panel) (screen *HeadlessScreen, owner *Window) {
	t.Helper()
	screen = startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			if owner = newHeadlessTestWindow(t, ownerTitle, geom.NewRect(20, 20, 200, 150)); owner == nil {
				return
			}
			content := owner.Content()
			content.SetLayout(nil)
			for _, p := range build() {
				content.AddChild(p)
			}
			owner.ToFront()
		}))
	if owner == nil {
		t.FailNow()
	}
	return screen, owner
}

// TestTooltipShowsInPopupWindow verifies that a tooltip is shown in a popup window of the tooltip kind belonging to the
// window it is shown for, placed just below the panel in screen space and so free to extend past that window's edges.
func TestTooltipShowsInPopupWindow(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(150, 130, 50, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	var tw, twOwner *Window
	var valid, visible, isTooltip, cpuOnly, transient, listed bool
	var kind WindowKind
	var rect, ownerRect geom.Rect
	var content, tip *Panel
	c.True(screen.Do(func() { tw = owner.tooltipWindow }))
	c.Nil(tw, "the tooltip window is created only when a tooltip is first shown")
	screen.MouseMove(screen.PanelCenter(panel), mod.None)
	c.True(screen.Do(func() {
		tw = owner.tooltipWindow
		valid = tw.IsValid()
		visible = tw.IsVisible()
		if !valid {
			return
		}
		twOwner = tw.owner
		isTooltip = tw.isTooltipWindow()
		kind = tw.kind
		cpuOnly = tw.cpuOnly
		transient = tw.transient
		listed = slices.Contains(Windows(), tw)
		rect = tw.ContentRect()
		ownerRect = owner.ContentRect()
		content = owner.tooltipPanel()
		tip = panel.Tooltip
	}))
	c.True(valid)
	c.True(visible)
	c.True(twOwner == owner)
	c.True(isTooltip)
	c.Equal(WindowKindTooltip, kind)
	c.True(cpuOnly)
	c.True(transient)
	c.False(listed, "Windows() leaves the tooltip window out")
	c.Equal(geom.NewRect(170, 171, 60, 20), rect, "placed one unit below the panel, in screen space")
	c.True(rect.Right() > ownerRect.Right() && rect.Bottom() > ownerRect.Bottom(),
		"the tooltip extends past the owner's edges rather than being squeezed into it")
	c.True(content == tip)
	c.True(screen.TooltipWindow(owner) == tw)
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipWindowIsReusedAcrossShows verifies that a window keeps one tooltip window for every tooltip it shows: one
// replacing a tooltip that is showing takes its place at once, without waiting out the delay or the window being hidden
// in between, and clearing the tooltip hides the window rather than disposing of it.
func TestTooltipWindowIsReusedAcrossShows(t *testing.T) {
	c := check.New(t)
	var first, second *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		first = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		second = newTooltipTestPanel(geom.NewRect(50, 10, 40, 20), geom.NewSize(70, 30))
		second.TooltipImmediate = false
		return []*Panel{first, second}
	})
	type state struct {
		tw      *Window
		tip     *Panel
		rect    geom.Rect
		visible bool
	}
	look := func() state {
		var st state
		st.tw = owner.tooltipWindow
		st.tip = owner.tooltipPanel()
		st.visible = st.tw.IsVisible()
		if st.tw != nil {
			st.rect = st.tw.ContentRect()
		}
		return st
	}
	read := func() state {
		var st state
		c.True(screen.Do(func() { st = look() }))
		return st
	}
	var firstTip, secondTip *Panel
	c.True(screen.Do(func() { firstTip, secondTip = first.Tooltip, second.Tooltip }))
	screen.MouseMove(screen.PanelCenter(first), mod.None)
	one := read()
	c.NotNil(one.tw)
	c.True(one.visible)
	c.True(one.tip == firstTip)
	c.Equal(geom.NewRect(30, 51, 60, 20), one.rect)
	// The second tooltip is not immediate, so showing it arms its dismissal. The move, the look at the tooltip and the
	// clear are made in one visit to the UI thread, which the dismissal cannot interrupt however slow the runner.
	at := screen.PanelCenter(second)
	var two state
	var valid bool
	var visibleWhilePlaced []bool
	c.True(screen.Do(func() {
		// The second tooltip differs from the first in size and position, so placing it moves and resizes the window.
		record := func() { visibleWhilePlaced = append(visibleWhilePlaced, one.tw.IsVisible()) }
		one.tw.MovedCallback = record
		one.tw.ResizedCallback = record
		screen.pointerMoved(at, mod.None)
		one.tw.MovedCallback = nil
		one.tw.ResizedCallback = nil
		two = look()
		owner.ClearTooltip()
		valid = one.tw.IsValid()
	}))
	c.True(two.tw == one.tw, "the same window shows the next tooltip")
	c.True(len(visibleWhilePlaced) != 0, "placing the next tooltip should have moved and resized the window")
	c.False(slices.Contains(visibleWhilePlaced, false), "the window is not hidden while the next tooltip is placed")
	c.True(two.visible, "and a tooltip replacing one that was showing is shown at once")
	c.True(two.tip == secondTip)
	c.Equal(geom.NewRect(70, 51, 70, 30), two.rect, "placed below the panel it now belongs to")
	cleared := read()
	c.True(cleared.tw == one.tw, "clearing the tooltip keeps its window")
	c.True(valid)
	c.False(cleared.visible, "but hides it")
	c.Nil(cleared.tip)
	c.Nil(screen.TooltipWindow(owner))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHiddenTooltipWindowLeavesRedrawSet verifies that a tooltip window is not left among the windows waiting to be
// drawn once its tooltip has been taken down, with accessibility support active, which marks a window for its hiding
// as well: a hidden window in that set is asked whether it is visible on every pass of the event loop for as long as
// it stays there, which for the tooltip window would be the life of its owner.
func TestHiddenTooltipWindowLeavesRedrawSet(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	c.NotNil(screen.AccessibilityTree(owner), "accessibility support is active for the rest of the test")
	screen.MouseMove(screen.PanelCenter(panel), mod.None)
	tw := screen.TooltipWindow(owner)
	c.NotNil(tw)
	c.Nil(screen.WindowAt(geom.NewPoint(390, 290)), "the point left for is outside every window")
	screen.MouseMove(geom.NewPoint(390, 290), mod.None)
	c.Nil(screen.TooltipWindow(owner))
	var active, valid, pending bool
	c.True(screen.Do(func() {
		active = accessibilityActive.Load()
		valid = tw.IsValid()
		_, pending = redrawSet[tw]
	}))
	c.True(active)
	c.True(valid, "the tooltip window is kept for the next tooltip")
	c.False(pending, "but is not left waiting to be drawn")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipWaitsOutTheDelay verifies that a tooltip that is not immediate, hovered while none is showing, is armed to
// show once the delay runs out rather than shown at once.
func TestTooltipWaitsOutTheDelay(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		panel.TooltipImmediate = false
		return []*Panel{panel}
	})
	// The move and the look are made in one visit to the UI thread, which the delay cannot interrupt however slow the
	// runner.
	at := screen.PanelCenter(panel)
	var showing, armed bool
	c.True(screen.Do(func() {
		screen.pointerMoved(at, mod.None)
		showing = owner.tooltipShowing()
		armed = owner.lastTooltip == panel.Tooltip
		owner.ClearTooltip()
	}))
	c.False(showing, "the tooltip waits out the delay")
	c.True(armed, "but is armed to show once it has")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// showTestTooltip shows panel's tooltip for owner as the delay running out does, without the pointer having to be over
// the panel.
func showTestTooltip(owner *Window, panel *Panel) {
	owner.tooltipSequence++
	owner.lastTooltip = panel.Tooltip
	avoid := panel.RectToRoot(panel.ContentRect(true))
	(&tooltipSequencer{window: owner, avoid: avoid, sequence: owner.tooltipSequence}).show()
}

// TestTooltipClearsOnOwnerChanges verifies that moving, resizing, hiding or minimizing a window takes down the tooltip
// it is showing, and that disposing of it disposes of its tooltip window first.
func TestTooltipClearsOnOwnerChanges(t *testing.T) {
	for _, one := range []struct {
		change func(owner *Window)
		name   string
	}{
		{name: "move", change: func(owner *Window) { owner.SetContentRect(geom.NewRect(30, 30, 200, 150)) }},
		{name: "resize", change: func(owner *Window) { owner.SetContentRect(geom.NewRect(20, 20, 210, 160)) }},
		{name: "hide", change: func(owner *Window) { owner.Hide() }},
		{name: "minimize", change: func(owner *Window) { owner.Minimize() }},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var panel *Panel
			screen, owner := startTooltipTest(t, func() []*Panel {
				panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
				return []*Panel{panel}
			})
			// Another window takes the focus and the tooltip is shown directly, so that it is not the focus moving away
			// or the pointer leaving, both of which take a tooltip down as well, that does it here.
			c.True(screen.Do(func() { newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100)).ToFront() }))
			var tw *Window
			var showing, valid, visible bool
			c.True(screen.Do(func() {
				owner.showTooltip(panel.Tooltip, geom.NewRect(30, 51, 60, 20))
				tw = owner.tooltipWindow
				showing = owner.tooltipShowing() && !owner.Focused()
			}))
			c.True(showing)
			c.True(screen.Do(func() {
				one.change(owner)
				showing = owner.tooltipShowing()
				valid = tw.IsValid()
				visible = tw.IsVisible()
			}))
			c.False(showing)
			c.True(valid, "the tooltip window is kept for the next tooltip")
			c.False(visible)
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
	t.Run("dispose", func(t *testing.T) {
		c := check.New(t)
		var panel *Panel
		screen, owner := startTooltipTest(t, func() []*Panel {
			panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
			return []*Panel{panel}
		})
		var order []string
		var tw, after *Window
		var valid, listed bool
		c.True(screen.Do(func() {
			// Keeps the session alive once the owner is gone.
			newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100))
			showTestTooltip(owner, panel)
			tw = owner.tooltipWindow
			if tw == nil {
				return
			}
			tw.WillCloseCallback = func() { order = append(order, "tooltip") }
			owner.WillCloseCallback = func() { order = append(order, ownerTitle) }
			owner.Dispose()
			valid = tw.IsValid()
			listed = slices.Contains(windowList, tw)
			after = owner.tooltipWindow
		}))
		c.NotNil(tw)
		c.Equal([]string{"tooltip", ownerTitle}, order)
		c.False(valid)
		c.False(listed)
		c.Nil(after, "the owner lets go of its disposed tooltip window")
		c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
	})
}

// TestSuggestionsHiddenBeforeTheWindowGoes verifies that Hide and Minimize hide the window's field suggestions
// themselves, before the window goes, rather than leaving that to the focus loss that follows, which X11 reports only
// later.
func TestSuggestionsHiddenBeforeTheWindowGoes(t *testing.T) {
	for _, one := range []struct {
		change func(owner *Window)
		gone   func(owner *Window) bool
		name   string
	}{
		{
			name:   "hidden",
			change: func(owner *Window) { owner.Hide() },
			gone:   func(owner *Window) bool { return owner.hidden },
		},
		{
			name:   "minimized",
			change: func(owner *Window) { owner.Minimize() },
			gone:   func(owner *Window) bool { return owner.minimized },
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			screen, owner, _, _ := startSuggestionsTest(t)
			host := &fakeSuggestionsHost{shown: true}
			var hidden, goneAtHide bool
			host.onHide = func() { goneAtHide = one.gone(owner) }
			c.True(screen.Do(func() {
				owner.suggestions = host
				one.change(owner)
				hidden = !host.shown
			}))
			c.True(hidden)
			c.False(goneAtHide, "hidden by the window itself, before it went")
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
}

// TestSuggestionsHideOnOwnerChanges verifies that moving, resizing, hiding or minimizing a window, or another window
// coming to the front, hides the field suggestions it is showing, disposing of their popup window, and that disposing
// of it disposes of that window too.
func TestSuggestionsHideOnOwnerChanges(t *testing.T) {
	for _, one := range []struct {
		change func(t *testing.T, owner *Window)
		name   string
	}{
		{name: "moved", change: func(_ *testing.T, owner *Window) {
			owner.SetContentRect(geom.NewRect(30, 30, 200, 150))
		}},
		{name: "resized", change: func(_ *testing.T, owner *Window) {
			owner.SetContentRect(geom.NewRect(20, 20, 210, 160))
		}},
		{name: "hidden", change: func(_ *testing.T, owner *Window) { owner.Hide() }},
		{name: "minimized", change: func(_ *testing.T, owner *Window) { owner.Minimize() }},
		{name: "another in front", change: func(t *testing.T, _ *Window) {
			if other := newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100)); other != nil {
				other.ToFront()
			}
		}},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			screen, owner, _, s := startSuggestionsTest(t)
			var wnd *Window
			c.True(screen.Do(func() {
				s.SetRows(suggestionsTestRows)
				wnd = s.wnd
			}))
			c.True(screen.SuggestionsWindow(owner) == wnd)
			c.True(screen.Do(func() { one.change(t, owner) }))
			var showing, valid, listed bool
			var host fieldSuggestionsHost
			c.True(screen.Do(func() {
				showing = s.Showing()
				host = owner.suggestions
				valid = wnd.IsValid()
				listed = slices.Contains(windowList, wnd)
			}))
			c.False(showing)
			c.Nil(host)
			c.False(valid, "the popup window is disposed of")
			c.False(listed)
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
	t.Run("dispose", func(t *testing.T) {
		c := check.New(t)
		screen, owner, _, s := startSuggestionsTest(t)
		var wnd, after *Window
		var host fieldSuggestionsHost
		var valid, listed, showing bool
		c.True(screen.Do(func() {
			// Keeps the session alive once the owner is gone.
			newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100))
			s.SetRows(suggestionsTestRows)
			wnd = s.wnd
			owner.Dispose()
			valid = wnd.IsValid()
			listed = slices.Contains(windowList, wnd)
			after = s.wnd
			host = owner.suggestions
			showing = s.Showing()
		}))
		c.NotNil(wnd)
		c.False(valid)
		c.False(listed)
		c.Nil(after, "the suggestions let go of their disposed window")
		c.Nil(host)
		c.False(showing)
		c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
	})
}

// TestTooltipWindowClosedOnItsOwn verifies that a tooltip window closed by something other than the toolkit, as a
// WM_CLOSE from another program would close it, takes its owner's tooltip with it: the owner's description loses the
// tooltip, and the pointer moving on within the same panel shows the tooltip again, in a new window.
func TestTooltipWindowClosedOnItsOwn(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	over := screen.PanelCenter(panel)
	screen.MouseMove(over, mod.None)
	tw := screen.TooltipWindow(owner)
	c.NotNil(tw)
	c.NotNil(screen.AccessibilityTree(owner))
	var tipID accessibility.NodeID
	c.True(screen.Do(func() { tipID = panel.Tooltip.Accessibility.id }))
	c.NotEqual(accessibility.NodeID(0), tipID, "the tooltip should have been described")
	screen.AccessibilityEvents(owner)
	var after *Window
	var lastIsNil bool
	c.True(screen.Do(func() {
		tw.requestClose()
		after = owner.tooltipWindow
		lastIsNil = owner.lastTooltip == nil
	}))
	c.Nil(after)
	c.True(lastIsNil, "the tooltip asked for is forgotten, so moving on within the panel arms it again")
	removed := false
	for _, e := range screen.AccessibilityEvents(owner) {
		if e.Kind == accessibility.NodeRemoved && e.Node == tipID {
			removed = true
		}
	}
	c.True(removed, "the tooltip should have been removed from the owner's description")
	screen.MouseMove(over.Add(geom.NewPoint(1, 0)), mod.None)
	next := screen.TooltipWindow(owner)
	c.NotNil(next)
	c.True(next != tw, "in a new window")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipShowsOnceWindowGainsFocus verifies that the tooltip of a panel hovered while its window was not focused,
// which is neither shown nor remembered then, shows once the window gains the focus with the pointer still over the
// panel.
func TestTooltipShowsOnceWindowGainsFocus(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	c.True(screen.Do(func() { newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100)).ToFront() }))
	screen.MouseMove(screen.PanelCenter(panel), mod.None)
	var focused bool
	c.True(screen.Do(func() { focused = owner.Focused() }))
	c.False(focused)
	c.Nil(screen.TooltipWindow(owner), "no tooltip shows in a window without the focus")
	var remembered *Panel
	c.True(screen.Do(func() { remembered = owner.lastTooltip }))
	c.Nil(remembered, "nor is one remembered, which would keep it from showing later")
	c.True(screen.Do(func() { owner.ToFront() }))
	c.NotNil(screen.TooltipWindow(owner), "the tooltip shows once the window has the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipComesBackWithTheFocus verifies that a tooltip taken down by the window losing the focus is forgotten, so
// that it shows again when the focus returns with the pointer still over the same panel, as after switching to another
// application and back without moving the pointer.
func TestTooltipComesBackWithTheFocus(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	screen.MouseMove(screen.PanelCenter(panel), mod.None)
	c.NotNil(screen.TooltipWindow(owner))
	var other *Window
	var lastIsNil bool
	c.True(screen.Do(func() {
		other = newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100))
		other.ToFront()
	}))
	c.Nil(screen.TooltipWindow(owner), "losing the focus takes the tooltip down")
	c.True(screen.Do(func() { lastIsNil = owner.lastTooltip == nil }))
	c.True(lastIsNil, "and forgets it")
	c.True(screen.Do(func() { owner.ToFront() }))
	c.NotNil(screen.TooltipWindow(owner), "the tooltip of the panel under the pointer shows again with the focus")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHideAndMinimizeForgetTooltip verifies that hiding or minimizing a window forgets the tooltip asked for, whether
// or not the platform reports the pointer leaving the window for it, as AppKit is not known to: the pointer is left
// outside the window here, so that the headless backend has no exit to synthesize and the forgetting is the window's
// own.
func TestHideAndMinimizeForgetTooltip(t *testing.T) {
	for _, one := range []struct {
		away func(w *Window)
		name string
	}{
		{name: "hide", away: func(w *Window) { w.Hide() }},
		{name: "minimize", away: func(w *Window) { w.Minimize() }},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var panel *Panel
			screen, owner := startTooltipTest(t, func() []*Panel {
				panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
				return []*Panel{panel}
			})
			c.Nil(screen.WindowAt(geom.NewPoint(390, 290)), "the pointer is left outside every window")
			screen.MouseMove(geom.NewPoint(390, 290), mod.None)
			var lastIsNil bool
			c.True(screen.Do(func() {
				// As a platform that reports no exit for the window going would leave it.
				owner.lastTooltip = panel.Tooltip
				one.away(owner)
				lastIsNil = owner.lastTooltip == nil
			}))
			c.True(lastIsNil, "the tooltip asked for is forgotten")
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
}

// TestClickToFocusKeepsTooltipDown verifies that a press that gives a window the focus leaves the tooltip of the panel
// it lands on down, on its release and as the pointer moves on within the panel, as a press in a window that had the
// focus does.
func TestClickToFocusKeepsTooltipDown(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
		return []*Panel{panel}
	})
	c.True(screen.Do(func() { newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100)).ToFront() }))
	over := screen.PanelCenter(panel)
	screen.MouseMove(over, mod.None)
	c.Nil(screen.TooltipWindow(owner))
	screen.MouseDown(over, ButtonLeft, mod.None)
	var focused bool
	c.True(screen.Do(func() { focused = owner.Focused() }))
	c.True(focused, "the press gives the window the focus")
	c.Nil(screen.TooltipWindow(owner), "no tooltip shows for the press")
	screen.MouseUp(over, ButtonLeft, mod.None)
	c.Nil(screen.TooltipWindow(owner), "nor for its release")
	screen.MouseMove(over.Add(geom.NewPoint(1, 0)), mod.None)
	c.Nil(screen.TooltipWindow(owner), "nor as the pointer moves on within the panel")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestTooltipReArmsAfterOwnerMovesOrResizes verifies that a tooltip a move or resize of its window took down comes back
// as the pointer moves on within the same panel, though not on the release of a press during which the window changed.
func TestTooltipReArmsAfterOwnerMovesOrResizes(t *testing.T) {
	for _, one := range []struct {
		name string
		rect geom.Rect
	}{
		{name: "move", rect: geom.NewRect(30, 30, 200, 150)},
		{name: "resize", rect: geom.NewRect(20, 20, 210, 160)},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			var panel *Panel
			screen, owner := startTooltipTest(t, func() []*Panel {
				panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
				panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
				return []*Panel{panel}
			})
			screen.MouseMove(screen.PanelCenter(panel), mod.None)
			c.NotNil(screen.TooltipWindow(owner))
			var remembered *Panel
			c.True(screen.Do(func() {
				owner.SetContentRect(one.rect)
				remembered = owner.lastTooltip
			}))
			c.Nil(screen.TooltipWindow(owner), "the change takes the tooltip down")
			c.Nil(remembered, "and forgets it")
			over := screen.PanelCenter(panel)
			screen.MouseMove(over, mod.None)
			c.NotNil(screen.TooltipWindow(owner), "moving on within the panel shows it again")
			screen.MouseDown(over, ButtonLeft, mod.None)
			c.True(screen.Do(func() { owner.SetContentRect(geom.NewRect(20, 20, 200, 150)) }))
			screen.MouseUp(screen.PanelCenter(panel), ButtonLeft, mod.None)
			c.Nil(screen.TooltipWindow(owner), "but not on the release of a press")
			c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
		})
	}
}

// TestNoTooltipWhileMenusOpen verifies that no tooltip is shown while a menu is open, since its window would sit above
// the menus: one showing is taken down when a menu opens, hovering arms none, and one whose delay runs out is not
// shown. Once the menu closes, hovering arms the tooltip again.
func TestNoTooltipWhileMenusOpen(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	over := screen.PanelCenter(panel)
	screen.MouseMove(over, mod.None)
	c.NotNil(screen.TooltipWindow(owner))
	var open int
	var showing, lastIsNil bool
	c.True(screen.Do(func() {
		var chosen []string
		newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One").Popup(geom.NewRect(150, 10, 0, 0), -1)
		open = len(owner.root.openMenuPanels)
		showing = owner.tooltipShowing()
	}))
	c.Equal(1, open)
	c.False(showing, "opening a menu takes the tooltip down")
	screen.MouseMove(over.Add(geom.NewPoint(1, 0)), mod.None)
	var delayedShowing bool
	c.True(screen.Do(func() {
		showing = owner.tooltipShowing()
		lastIsNil = owner.lastTooltip == nil
		showTestTooltip(owner, panel)
		delayedShowing = owner.tooltipShowing()
		owner.lastTooltip = nil
	}))
	c.False(showing, "hovering shows no tooltip while the menu is open")
	c.True(lastIsNil)
	c.False(delayedShowing, "nor does a delay running out")
	screen.KeyPress(KeyEscape, mod.None)
	c.True(screen.Do(func() { open = len(owner.root.openMenuPanels) }))
	c.Equal(0, open)
	screen.MouseMove(over, mod.None)
	c.NotNil(screen.TooltipWindow(owner), "the tooltip is shown again once the menu has closed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestNoTooltipWhileAnotherWindowsMenusOpen verifies that no tooltip is shown in a window while another window's menus
// are open, since its tooltip window would sit above them: a focusable transient window takes the focus here, which
// leaves the menus of the window it came from open, as it does on Windows, where no press outside them closes them.
func TestNoTooltipWhileAnotherWindowsMenusOpen(t *testing.T) {
	c := check.New(t)
	screen, owner := startTooltipTest(t, func() []*Panel { return nil })
	var transient *Window
	var panel *Panel
	c.True(screen.Do(func() {
		var chosen []string
		newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One").Popup(geom.NewRect(10, 10, 0, 0), -1)
		var err error
		if transient, err = NewWindow("transient", TransientWindowOption()); err != nil {
			t.Errorf("unable to create transient window: %v", err)
			return
		}
		transient.SetContentRect(geom.NewRect(300, 20, 90, 100))
		content := transient.Content()
		content.SetLayout(nil)
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		content.AddChild(panel)
		transient.Show()
		transient.ToFront()
	}))
	var open int
	var focused bool
	c.True(screen.Do(func() {
		open = len(owner.root.openMenuPanels)
		focused = transient.Focused()
	}))
	c.Equal(1, open, "the transient window taking the focus leaves the menu open")
	c.True(focused)
	over := screen.PanelCenter(panel)
	screen.MouseMove(over, mod.None)
	c.Nil(screen.TooltipWindow(transient), "no tooltip shows while another window's menu is open")
	var lastIsNil bool
	c.True(screen.Do(func() { lastIsNil = transient.lastTooltip == nil }))
	c.True(lastIsNil, "nor is one remembered")
	c.True(screen.Do(func() { owner.root.closeMenuStackStoppingAt(nil) }))
	screen.MouseMove(over.Add(geom.NewPoint(1, 0)), mod.None)
	c.NotNil(screen.TooltipWindow(transient), "the tooltip is shown once the menu has closed")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPointerLeavingWindowClearsTooltip verifies that the tooltip goes when the pointer leaves the window, and that
// coming back to the same panel shows it again.
func TestPointerLeavingWindowClearsTooltip(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		return []*Panel{panel}
	})
	over := screen.PanelCenter(panel)
	screen.MouseMove(over, mod.None)
	c.NotNil(screen.TooltipWindow(owner))
	c.Nil(screen.WindowAt(geom.NewPoint(390, 290)), "the point left for is outside every window")
	screen.MouseMove(geom.NewPoint(390, 290), mod.None)
	var showing, lastIsNil bool
	c.True(screen.Do(func() {
		showing = owner.tooltipShowing()
		lastIsNil = owner.lastTooltip == nil
	}))
	c.False(showing)
	c.True(lastIsNil, "the tooltip asked for is forgotten, so coming back arms it again")
	screen.MouseMove(over, mod.None)
	c.NotNil(screen.TooltipWindow(owner))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestPointerLeftDuringPressKeepsTooltipDown verifies that the pointer leaving the window while a press is in progress,
// as macOS and X11 report for a press dragged out of the window, does not arm the pressed panel's tooltip again:
// neither coming back over the panel during the press nor the release shows it.
func TestPointerLeftDuringPressKeepsTooltipDown(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(10, 10, 40, 20), geom.NewSize(60, 20))
		panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool { return true }
		return []*Panel{panel}
	})
	over := screen.PanelCenter(panel)
	screen.MouseMove(over, mod.None)
	c.NotNil(screen.TooltipWindow(owner))
	var showing bool
	screen.MouseDown(over, ButtonLeft, mod.None)
	c.True(screen.Do(func() {
		// The press dragged out of the window and back over the panel.
		owner.pointerLeft()
		owner.mouseEnter(windowLocal(owner, over), mod.None)
		showing = owner.tooltipShowing()
	}))
	c.False(showing, "coming back over the panel during the press shows no tooltip")
	screen.MouseUp(over, ButtonLeft, mod.None)
	c.Nil(screen.TooltipWindow(owner), "nor does the release")
	screen.MouseDown(over, ButtonLeft, mod.None)
	c.True(screen.Do(func() { owner.pointerLeft() }))
	screen.MouseUp(over, ButtonLeft, mod.None)
	c.Nil(screen.TooltipWindow(owner), "nor a release with the pointer not having come back first")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestHoverOverTooltipWindowPassesThrough verifies that the pointer passes through a tooltip window to the window
// beneath it, as the platforms make it do: moving over the part of the owner the tooltip covers leaves the owner
// hovered and the tooltip showing, and a press there reaches the panel beneath.
func TestHoverOverTooltipWindowPassesThrough(t *testing.T) {
	c := check.New(t)
	var panel *Panel
	presses := 0
	screen, owner := startTooltipTest(t, func() []*Panel {
		panel = newTooltipTestPanel(geom.NewRect(0, 0, 200, 150), geom.NewSize(100, 30))
		// The tooltip goes below this part of the panel, and so over the rest of it.
		panel.UpdateTooltipCallback = func(_ geom.Point, _ geom.Rect) geom.Rect { return geom.NewRect(10, 10, 20, 20) }
		panel.MouseDownCallback = func(_ geom.Point, _, _ int, _ mod.Modifiers) bool {
			presses++
			return true
		}
		return []*Panel{panel}
	})
	screen.MouseMove(geom.NewPoint(40, 40), mod.None)
	tw := screen.TooltipWindow(owner)
	c.NotNil(tw)
	var rect geom.Rect
	c.True(screen.Do(func() { rect = tw.ContentRect() }))
	c.Equal(geom.NewRect(30, 51, 100, 30), rect)
	under := geom.NewPoint(60, 65)
	c.True(under.In(rect))
	c.True(screen.WindowAt(under) == owner, "the tooltip window is passed over")
	screen.MouseMove(under, mod.None)
	var hovered *Panel
	c.True(screen.Do(func() { hovered = owner.lastMouseOverPanel }))
	c.True(hovered == panel, "the owner keeps the hover")
	c.True(screen.TooltipWindow(owner) == tw, "and the tooltip stays")
	screen.Click(under)
	var pressed int
	c.True(screen.Do(func() { pressed = presses }))
	c.Equal(1, pressed, "a press over the tooltip reaches the panel beneath it")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
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

// startPopupTest starts a session holding an owner window, shown and focused, and a popup menu window it owns, shown
// over it.
func startPopupTest(t *testing.T) (screen *HeadlessScreen, owner, popup *Window) {
	t.Helper()
	screen = startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			owner = newHeadlessTestWindow(t, ownerTitle, geom.NewRect(20, 20, 200, 150))
			owner.ToFront()
			var err error
			if popup, err = newPopupWindow(owner, "popup", WindowKindMenu); err != nil {
				t.Errorf("unable to create popup window: %v", err)
				return
			}
			popup.SetContentRect(geom.NewRect(100, 100, 80, 60))
			popup.Show()
		}))
	if owner == nil || popup == nil {
		t.FailNow()
	}
	return screen, owner, popup
}

// TestNewPopupWindow verifies what newPopupWindow sets up: a CPU-rendered, transient, undecorated menu or tooltip
// window that belongs to the top-level window even when created from another popup, whose size is not limited by its
// content, and which Windows and WindowCount leave out.
func TestNewPopupWindow(t *testing.T) {
	c := check.New(t)
	screen, owner, popup := startPopupTest(t)
	var sub, orphan, tip *Window
	var subErr, orphanErr, tipErr error
	var windows []*Window
	var count, listed, listedAfterOrphan int
	var size geom.Size
	var popupOwner, popupMenuOwner, ownerMenuOwner, subOwner, tipOwner *Window
	var popupIsPopup, ownerIsPopup, cpuOnly, surfaceCPUOnly, transient, undecorated, notResizable bool
	var popupIsTooltip, tipIsTooltip, tipIsPopup, tipCPUOnly, tipTransient bool
	var kind, tipKind WindowKind
	var titleIcons int
	c.True(screen.Do(func() {
		sub, subErr = newPopupWindow(popup, "sub", WindowKindMenu)
		tip, tipErr = newPopupWindow(popup, "tip", WindowKindTooltip)
		listed = len(windowList)
		orphan, orphanErr = newPopupWindow(nil, "orphan", WindowKindMenu)
		listedAfterOrphan = len(windowList)
		windows = Windows()
		count = WindowCount()
		popup.SetContent(NewPanel())
		popup.Content().SetSizer(func(_ geom.Size) (minSize, prefSize, maxSize geom.Size) {
			return geom.NewSize(50, 50), geom.NewSize(50, 50), geom.NewSize(50, 50)
		})
		popup.SetContentRect(geom.NewRect(100, 100, 300, 10))
		size = popup.ContentRect().Size
		popupOwner = popup.owner
		popupIsPopup = popup.isPopup()
		ownerIsPopup = owner.isPopup()
		popupMenuOwner = popup.menuOwner()
		ownerMenuOwner = owner.menuOwner()
		cpuOnly = popup.cpuOnly
		surfaceCPUOnly = popup.surface.cpuOnly
		kind = popup.kind
		transient = popup.transient
		undecorated = popup.undecorated
		notResizable = popup.notResizable
		titleIcons = len(popup.titleIcons)
		if sub != nil {
			subOwner = sub.owner
		}
		popupIsTooltip = popup.isTooltipWindow()
		if tip != nil {
			tipOwner = tip.owner
			tipKind = tip.kind
			tipIsTooltip = tip.isTooltipWindow()
			tipIsPopup = tip.isPopup()
			tipCPUOnly = tip.cpuOnly
			tipTransient = tip.transient
		}
	}))
	c.True(popupOwner == owner)
	c.True(popupIsPopup)
	c.False(ownerIsPopup)
	c.True(popupMenuOwner == owner)
	c.True(ownerMenuOwner == owner)
	c.True((*Window)(nil).menuOwner() == nil)
	c.False((*Window)(nil).isPopup())
	c.True(cpuOnly)
	c.True(surfaceCPUOnly, "the surface should have been told to stay on the CPU")
	c.Equal(WindowKindMenu, kind)
	c.True(transient)
	c.True(undecorated)
	c.True(notResizable)
	c.Equal(0, titleIcons)
	c.NoError(subErr)
	c.True(subOwner == owner, "a popup opened from a popup should belong to the top-level window")
	c.HasError(orphanErr)
	c.True(orphan == nil)
	c.Equal(listed, listedAfterOrphan, "a refused popup should not join the window list")
	c.Equal(4, listed)
	c.Equal([]*Window{owner}, windows, "popups should not be listed")
	c.Equal(1, count, "popups should not be counted")
	c.Equal(geom.NewSize(300, 10), size, "a popup's size should not be limited by its content")
	c.NoError(tipErr)
	c.True(tipOwner == owner, "a tooltip window belongs to the top-level window too")
	c.Equal(WindowKindTooltip, tipKind)
	c.True(tipIsTooltip)
	c.True(tipIsPopup)
	c.True(tipCPUOnly)
	c.True(tipTransient)
	c.False(popupIsTooltip, "a menu's popup window is not a tooltip window")
	c.False((&Window{kind: WindowKindTooltip}).isTooltipWindow(),
		"an application's own window of the tooltip kind is not one of the toolkit's tooltip windows")
}

// TestFirstTopLevelWindowSkipsPopups verifies that the window a closing window hands the focus to, and the one the
// Windows clipboard code borrows, is never a popup menu window.
func TestFirstTopLevelWindowSkipsPopups(t *testing.T) {
	c := check.New(t)
	saveFrontState(t)
	owner := newModalInputTestWindow()
	popup := newModalInputTestWindow()
	popup.owner = owner
	windowList = []*Window{popup, owner}
	c.True(firstTopLevelWindow() == owner)
	windowList = []*Window{popup}
	c.True(firstTopLevelWindow() == nil)
	windowList = nil
	c.True(firstTopLevelWindow() == nil)
}

// TestPopupToFrontIsRefused verifies that a popup menu window never takes the focus or becomes the active window, even
// when asked to with ToFront, and that ToFront does not show it either.
func TestPopupToFrontIsRefused(t *testing.T) {
	c := check.New(t)
	screen, owner, popup := startPopupTest(t)
	var active, pending *Window
	var visible bool
	c.True(screen.Do(func() {
		popup.Hide()
		popup.ToFront()
		visible = popup.IsVisible()
		pending = pendingFrontWindow
	}))
	c.False(visible, "ToFront should not show a popup")
	c.True(pending != popup, "ToFront should not ask for a popup to be activated")
	var focused bool
	c.True(screen.Do(func() {
		popup.Show()
		popup.ToFront()
		active = ActiveWindow()
		visible = popup.IsVisible()
		focused = popup.Focused()
	}))
	c.True(visible)
	c.True(active == owner, "the owner should remain the active window")
	c.True(screen.FocusedWindow() == owner, "the owner should keep the focus")
	c.False(focused)
}

// TestPopupFollowsOwnerThroughModal verifies that a popup menu window is blocked by a modal window exactly when its
// owner is, and that a key event reaching a popup is handed to its owner.
func TestPopupFollowsOwnerThroughModal(t *testing.T) {
	c := check.New(t)
	owner := newModalInputTestWindow()
	popup := newModalInputTestWindow()
	popup.owner = owner
	other := newModalInputTestWindow()
	c.True(popup.okToProcess())
	pushTestModal(t, owner)
	c.True(popup.okToProcess(), "a popup should be usable while its owner is the top modal window")
	c.False(other.okToProcess())
	pushTestModal(t, other)
	c.False(popup.okToProcess(), "a popup should be blocked while its owner is")
	modalStack = modalStack[:len(modalStack)-1]
	var events []string
	owner.KeyDownCallback = func(_ KeyCode, _ mod.Modifiers, _ bool) bool {
		events = append(events, "down")
		return true
	}
	owner.RuneTypedCallback = func(_ rune) bool {
		events = append(events, "rune")
		return true
	}
	owner.KeyUpCallback = func(_ KeyCode, _ mod.Modifiers) bool {
		events = append(events, "up")
		return true
	}
	popup.KeyDownCallback = func(_ KeyCode, _ mod.Modifiers, _ bool) bool {
		events = append(events, "popup")
		return true
	}
	popup.keyPressed(KeyA, 0)
	popup.runeTyped('a')
	popup.keyReleased(KeyA, 0)
	c.Equal([]string{"down", "rune", "up"}, events)
	c.Equal(0, len(popup.pressedKeys))
	c.Equal(0, len(owner.pressedKeys))
}

// TestDisposePopupWindow verifies that disposing of a popup menu window leaves its owner valid and focused and takes
// the popup out of the window list, which Windows never listed it in.
func TestDisposePopupWindow(t *testing.T) {
	c := check.New(t)
	screen, owner, popup := startPopupTest(t)
	var before, after int
	var windows []*Window
	var inList, ownerValid, popupValid bool
	c.True(screen.Do(func() {
		before = len(windowList)
		popup.Dispose()
		after = len(windowList)
		inList = slices.Contains(windowList, popup)
		windows = Windows()
		ownerValid = owner.IsValid()
		popupValid = popup.IsValid()
	}))
	c.Equal(2, before)
	c.Equal(1, after)
	c.False(inList)
	c.False(popupValid)
	c.True(ownerValid)
	c.Equal([]*Window{owner}, windows)
	c.True(screen.FocusedWindow() == owner)
	c.True(screen.WindowAt(geom.NewPoint(110, 110)) == owner, "the owner should be what is under the popup's old spot")
}

// TestDisposeOwnerDisposesPopupsFirst verifies that disposing of a window closes its menu stack and disposes of the
// popup menu windows it owns, the one showing the open menu and one that is in no menu stack, before itself, leaving
// the stack empty and the menu closed.
func TestDisposeOwnerDisposesPopupsFirst(t *testing.T) {
	c := check.New(t)
	screen, owner, popup := startPopupTest(t)
	var order []string
	var other, menuPopup *Window
	var windows []*Window
	var listed, open int
	var popupValid, ownerValid, menuPopupValid, menuOpen bool
	c.True(screen.Do(func() {
		// Keeps the session alive once the owner is gone.
		other = newHeadlessTestWindow(t, "other", geom.NewRect(250, 20, 100, 100))
		var chosen []string
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One")
		m.Popup(geom.NewRect(10, 10, 0, 0), -1)
		if m.popupPanel == nil {
			return
		}
		menuPopup = m.popupPanel.Window()
		menuPopup.WillCloseCallback = func() { order = append(order, "menu") }
		popup.WillCloseCallback = func() { order = append(order, "popup") }
		owner.WillCloseCallback = func() { order = append(order, ownerTitle) }
		owner.Dispose()
		windows = Windows()
		listed = len(windowList)
		open = len(owner.root.openMenuPanels)
		menuOpen = m.popupPanel != nil
		popupValid = popup.IsValid()
		menuPopupValid = menuPopup.IsValid()
		ownerValid = owner.IsValid()
	}))
	c.NotNil(menuPopup)
	c.Equal([]string{"menu", "popup", ownerTitle}, order)
	c.Equal(0, open, "the menu stack is closed")
	c.False(menuOpen, "and the menu knows it")
	c.False(menuPopupValid)
	c.False(popupValid)
	c.False(ownerValid)
	c.Equal([]*Window{other}, windows)
	c.Equal(1, listed)
}

// TestPopupDrawDescribesOwner verifies that drawing a popup menu window describes its owner to an assistive technology,
// since the owner's tree is where the menu lives, and never builds a tree for the popup itself.
func TestPopupDrawDescribesOwner(t *testing.T) {
	c := check.New(t)
	screen, owner, popup := startPopupTest(t)
	screen.EnableAccessibility()
	var before, after, hiddenBefore, hiddenAfter, popupGeneration uint64
	var popupAX *windowAccessibility
	c.True(screen.Do(func() {
		before = owner.axGeneration
		popup.draw()
		after = owner.axGeneration
		owner.Hide()
		hiddenBefore = owner.axGeneration
		popup.draw()
		hiddenAfter = owner.axGeneration
		popupAX = popup.ax
		popupGeneration = popup.axGeneration
	}))
	c.True(after > before, "drawing the popup should have described the owner")
	c.Equal(hiddenBefore, hiddenAfter, "a hidden owner should not be described")
	c.True(popupAX == nil, "a popup should never get accessibility state of its own")
	c.Equal(uint64(0), popupGeneration)
}

// TestUsesGLRenderingSkipsCPUOnlyWindows verifies that a CPU-only window never draws through OpenGL, even while the
// rest of the process does.
func TestUsesGLRenderingSkipsCPUOnlyWindows(t *testing.T) {
	c := check.New(t)
	saved := cpuRenderingActive
	t.Cleanup(func() { cpuRenderingActive = saved })
	cpuRenderingActive = false
	w := &Window{surface: &surface{}}
	c.True(w.usesGLRendering())
	w.cpuOnly = true
	c.False(w.usesGLRendering())
}

// TestHideAndMinimizeCloseTheMenuStack verifies that hiding or minimizing a window closes the menus open for it and
// disposes of their popup windows.
func TestHideAndMinimizeCloseTheMenuStack(t *testing.T) {
	c := check.New(t)
	screen, owner, _ := startPopupTest(t)
	for _, which := range []string{"Hide", "Minimize"} {
		var popup *Window
		var open int
		var popupValid bool
		c.True(screen.Do(func() {
			var chosen []string
			m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One")
			m.Popup(geom.NewRect(10, 10, 0, 0), -1)
			if m.popupPanel != nil {
				popup = m.popupPanel.Window()
			}
			if which == "Hide" {
				owner.Hide()
			} else {
				owner.Minimize()
			}
			open = len(owner.root.openMenuPanels)
			popupValid = popup.IsValid()
			if which == "Minimize" {
				owner.Minimize()
			}
			owner.ToFront()
		}))
		c.NotNil(popup, which)
		c.Equal(0, open, which)
		c.False(popupValid, which)
	}
}

// TestWindowMenuListsNoPopup verifies that the Window menu lists the open windows but none of the popup menu windows.
func TestWindowMenuListsNoPopup(t *testing.T) {
	c := check.New(t)
	screen, _, _ := startPopupTest(t)
	var titles []string
	var separators int
	c.True(screen.Do(func() {
		m := NewInWindowMenuFactory().NewMenu(WindowMenuID, "Window", nil)
		windowListUpdater(m)
		for i := range m.Count() {
			if mi := m.ItemAtIndex(i); mi.IsSeparator() {
				separators++
			} else {
				titles = append(titles, mi.Title())
			}
		}
	}))
	c.Equal(1, separators)
	c.Equal([]string{ownerTitle}, titles)
}

// TestMoveOverPopupRestoresOwnersHiddenCursor verifies that the mouse moving over a popup menu window shows the cursor
// that typing in the owner hid, as a move over the owner would.
func TestMoveOverPopupRestoresOwnersHiddenCursor(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var center geom.Point
	var hidden, opened bool
	c.True(screen.Do(func() {
		owner.HideCursorUntilMouseMoves()
		m := newTestMenu(&inWindowMenuFactory{}, UserBaseID, &chosen, "One", "Two")
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if opened = m.popupPanel != nil; opened {
			center = screen.PanelCenter(m.items[1].panel)
		}
		hidden = owner.cursorHidden
	}))
	c.True(opened)
	c.True(hidden, "the test needs the owner's cursor hidden with the menu open")
	screen.MouseMove(center, 0)
	var stillHidden, untilMoves bool
	c.True(screen.Do(func() { stillHidden, untilMoves = owner.cursorHidden, owner.cursorHiddenUntilMouseMoves }))
	c.False(untilMoves, "a move over the popup counts as the move the owner was waiting for")
	c.False(stillHidden)
}

// TestDisposePopupGoesThroughTheMenuStack verifies that a popup menu window disposed of directly, as a platform's close
// request does it, is closed through its owner's menu stack: the menu it shows and those above it are closed, the
// stack and the menus no longer hold the windows, and the menu can be opened again.
func TestDisposePopupGoesThroughTheMenuStack(t *testing.T) {
	c := check.New(t)
	screen, owner := startMenuTest(t, geom.NewSize(500, 400), geom.NewRect(50, 50, 300, 200))
	var chosen []string
	var m, sub *menu
	var parent, child *Window
	c.True(screen.Do(func() {
		f := &inWindowMenuFactory{}
		m = newTestMenu(f, UserBaseID, &chosen, "One")
		sub = newTestMenu(f, UserBaseID+100, &chosen, "Deep")
		m.InsertMenu(-1, sub)
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		if m.popupPanel != nil {
			parent = m.popupPanel.Window()
			sub.titleItem.showSubMenu()
			if sub.popupPanel != nil {
				child = sub.popupPanel.Window()
			}
		}
	}))
	c.NotNil(parent)
	c.NotNil(child)
	if parent == nil || child == nil {
		return
	}
	var open int
	var parentValid, childValid, parentOpen, subOpen bool
	look := func() {
		open = len(owner.root.openMenuPanels)
		parentValid, childValid = parent.IsValid(), child.IsValid()
		parentOpen, subOpen = m.popupPanel != nil, sub.popupPanel != nil
	}
	c.True(screen.Do(func() {
		child.Dispose()
		look()
	}))
	c.Equal(1, open, "disposing of the sub-menu's popup closes only the sub-menu")
	c.True(parentValid)
	c.False(childValid)
	c.True(parentOpen)
	c.False(subOpen)
	c.True(screen.Do(func() {
		sub.titleItem.showSubMenu()
		if sub.popupPanel != nil {
			child = sub.popupPanel.Window()
		}
		parent.Dispose()
		look()
	}))
	c.Equal(0, open, "disposing of the parent's popup closes it and the sub-menu above it")
	c.False(parentValid)
	c.False(childValid)
	c.False(parentOpen)
	c.False(subOpen)
	var reopened bool
	c.True(screen.Do(func() {
		m.Popup(geom.NewRect(20, 20, 0, 0), -1)
		reopened = m.popupPanel != nil
		m.closeMenuStack()
	}))
	c.True(reopened, "the menu opens again, since nothing still says it is open")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// TestFirstVisibleTopLevelWindowSkipsHiddenWindows verifies that the window a closing window hands the focus to is
// never one the application hid, which ToFront would show, nor a popup menu window, though it may be a minimized one,
// which ToFront restores, while the first top-level window the Windows clipboard code borrows may still be a hidden one.
func TestFirstVisibleTopLevelWindowSkipsHiddenWindows(t *testing.T) {
	c := check.New(t)
	var hidden, minimized, visible *Window
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			hidden = newHeadlessTestWindow(t, "hidden", geom.NewRect(10, 10, 100, 100))
			minimized = newHeadlessTestWindow(t, "minimized", geom.NewRect(120, 10, 100, 100))
			visible = newHeadlessTestWindow(t, "visible", geom.NewRect(230, 10, 100, 100))
		}))
	c.NotNil(hidden)
	var popup *Window
	var first, firstVisible, firstMinimized, hiddenWhileMinimized, shownAgain, noneVisible, noWindows *Window
	c.True(screen.Do(func() {
		var err error
		if popup, err = newPopupWindow(visible, "popup", WindowKindMenu); err != nil {
			t.Errorf("unable to create popup window: %v", err)
			return
		}
		hidden.Hide()
		minimized.Minimize()
		saved := windowList
		defer func() { windowList = saved }()
		windowList = []*Window{popup, hidden, visible, minimized}
		first = firstTopLevelWindow()
		firstVisible = firstVisibleTopLevelWindow()
		windowList = []*Window{popup, hidden, minimized, visible}
		firstMinimized = firstVisibleTopLevelWindow()
		minimized.Hide()
		hiddenWhileMinimized = firstVisibleTopLevelWindow()
		minimized.Show()
		shownAgain = firstVisibleTopLevelWindow()
		windowList = []*Window{popup, hidden}
		noneVisible = firstVisibleTopLevelWindow()
		windowList = nil
		noWindows = firstVisibleTopLevelWindow()
	}))
	c.True(first == hidden, "any top-level window will do for the clipboard")
	c.True(firstVisible == visible, "but only a visible one may be given the focus")
	c.True(firstMinimized == minimized, "or a minimized one, which is restored")
	c.True(hiddenWhileMinimized == visible, "but not one the application hid while it was minimized")
	c.True(shownAgain == minimized, "until it is shown again")
	c.True(noneVisible == nil)
	c.True(noWindows == nil)
}

// TestModalCenterFrame verifies where a window is placed to sit over another: centered horizontally and a third of the
// way down, whether the math is done in place or on a display whose space is the same as the window-rect space.
func TestModalCenterFrame(t *testing.T) {
	c := check.New(t)
	within := geom.NewRect(100, 100, 800, 600)
	frame := geom.NewRect(0, 0, 400, 300)
	c.Equal(geom.NewRect(300, 200, 400, 300), modalCenterFrame(nil, within, frame))
	d := &Display{Usable: geom.NewRect(0, 0, 1920, 1080), headless: true}
	c.Equal(geom.NewRect(300, 200, 400, 300), modalCenterFrame(d, within, frame))
}

// TestSetFocusFromLostFocusCallback verifies that a LostFocusCallback that moves the focus itself runs once and has its
// move stand, whether the focus was being moved to another panel or removed: the nested call runs no callback for a
// panel already losing the focus, and the outer one leaves the focus where the callback put it. The callback is
// guarded, so that a regression cannot run it without bound and overflow the stack.
func TestSetFocusFromLostFocusCallback(t *testing.T) {
	c := check.New(t)
	var a, b, d *Field
	var wnd *Window
	lost, gained := 0, 0
	screen := startHeadlessTest(t, HeadlessConfig{Width: 400, Height: 300},
		StartupFinishedCallback(func() {
			a, b, d = NewField(), NewField(), NewField()
			a.LostFocusCallback = func() {
				lost++
				if lost < 5 {
					wnd.SetFocus(d)
				}
			}
			d.GainedFocusCallback = func() { gained++ }
			if wnd = newHeadlessTestWindow(t, "focus", geom.NewRect(20, 20, 200, 150)); wnd != nil {
				content := wnd.Content()
				content.SetLayout(&FlexLayout{Columns: 1})
				content.AddChild(a)
				content.AddChild(b)
				content.AddChild(d)
				wnd.ToFront()
				wnd.SetFocus(a)
			}
		}))
	c.NotNil(wnd)
	var focus *Panel
	c.True(screen.Do(func() {
		wnd.SetFocus(b)
		focus = wnd.CurrentFocus()
	}))
	c.Equal(1, lost, "the callback runs once")
	c.Equal(1, gained)
	c.True(focus == d.AsPanel(), "the callback's move stands")
	c.True(screen.Do(func() {
		lost, gained = 0, 0
		wnd.SetFocus(a)
		wnd.SetFocus(nil)
		focus = wnd.CurrentFocus()
	}))
	c.Equal(1, lost, "the callback runs once when the focus is being removed")
	c.Equal(1, gained)
	c.True(focus == d.AsPanel(), "and its move stands then too")
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
