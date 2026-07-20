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
	"github.com/richardwilkes/toolbox/v2/geom"
	"github.com/richardwilkes/unison/enums/mod"
)

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
