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
	"github.com/richardwilkes/unison/internal/w32"
)

// TestW32MouseCaptureTransition is the regression test for the missing SetCapture/ReleaseCapture handling: without
// capture, Windows stops delivering mouse-move once the cursor leaves the client area during a drag and never delivers
// a button-up that happens outside, leaving stuck button state. The window procedure drives SetCapture/ReleaseCapture
// from this transition function, so it must acquire on the first press, hold across additional presses and partial
// releases, and release only when the last button goes up.
func TestW32MouseCaptureTransition(t *testing.T) {
	c := check.New(t)
	for i, one := range []struct {
		expected w32CaptureOp
		wParam   w32.WPARAM
		down     bool
		captured bool
	}{
		// First button press acquires the capture. The wParam of a down message already includes the flag of the
		// button just pressed.
		{expected: w32CaptureAcquire, down: true, captured: false, wParam: w32.MK_LBUTTON},
		// A second button pressed while the capture is held changes nothing.
		{expected: w32CaptureKeep, down: true, captured: true, wParam: w32.MK_LBUTTON | w32.MK_RBUTTON},
		// Releasing one button while another remains down keeps the capture. The wParam of an up message no longer
		// includes the flag of the button just released.
		{expected: w32CaptureKeep, down: false, captured: true, wParam: w32.MK_RBUTTON},
		// Releasing the last button releases the capture.
		{expected: w32CaptureRelease, down: false, captured: true, wParam: 0},
		// Every button type holds the capture on its own.
		{expected: w32CaptureKeep, down: false, captured: true, wParam: w32.MK_LBUTTON},
		{expected: w32CaptureKeep, down: false, captured: true, wParam: w32.MK_MBUTTON},
		{expected: w32CaptureKeep, down: false, captured: true, wParam: w32.MK_XBUTTON1},
		{expected: w32CaptureKeep, down: false, captured: true, wParam: w32.MK_XBUTTON2},
		// A release arriving after the capture was lost (e.g. taken by OLE's drag-drop loop) must not call
		// ReleaseCapture, since that would drop a capture now legitimately held elsewhere.
		{expected: w32CaptureKeep, down: false, captured: false, wParam: 0},
		// A press while the capture is somehow already held does not re-acquire.
		{expected: w32CaptureKeep, down: true, captured: true, wParam: w32.MK_LBUTTON},
	} {
		c.Equal(one.expected, w32MouseCaptureTransition(one.down, one.captured, one.wParam), "case %d", i)
	}
}

// TestW32AnyMouseButtonDown verifies that only the five mouse button MK_* flags count as a held button. Modifier key
// flags (MK_SHIFT is 0x0004, MK_CONTROL is 0x0008) and the XBUTTON identifier in the high word of an XBUTTON message's
// wParam must be ignored, since treating those as held buttons would keep the capture forever.
func TestW32AnyMouseButtonDown(t *testing.T) {
	c := check.New(t)
	c.False(w32AnyMouseButtonDown(0))
	c.True(w32AnyMouseButtonDown(w32.MK_LBUTTON))
	c.True(w32AnyMouseButtonDown(w32.MK_RBUTTON))
	c.True(w32AnyMouseButtonDown(w32.MK_MBUTTON))
	c.True(w32AnyMouseButtonDown(w32.MK_XBUTTON1))
	c.True(w32AnyMouseButtonDown(w32.MK_XBUTTON2))
	c.True(w32AnyMouseButtonDown(w32.MK_LBUTTON | w32.MK_XBUTTON2))
	c.False(w32AnyMouseButtonDown(0x0004 | 0x0008))                // MK_SHIFT | MK_CONTROL
	c.False(w32AnyMouseButtonDown(w32.WPARAM(w32.XBUTTON2) << 16)) // high word of WM_XBUTTONUP
	c.True(w32AnyMouseButtonDown(w32.WPARAM(w32.XBUTTON1)<<16 | 0x04 | w32.MK_MBUTTON))
}

// TestW32MouseMessagePoint verifies that the client coordinates packed into a mouse message's lParam are sign-extended
// from 16 bits. While the mouse is captured, positions outside the client area are delivered, and those above or to
// the left of it are negative; the previous unsigned decode turned a drag to (-5, -1) into (65531, 65535).
func TestW32MouseMessagePoint(t *testing.T) {
	c := check.New(t)
	pack := func(x, y int16) w32.LPARAM {
		return w32.LPARAM(uint32(uint16(y))<<16 | uint32(uint16(x)))
	}
	c.Equal(geom.NewPoint(0, 0), w32MouseMessagePoint(pack(0, 0)))
	c.Equal(geom.NewPoint(200, 100), w32MouseMessagePoint(pack(200, 100)))
	c.Equal(geom.NewPoint(-5, -1), w32MouseMessagePoint(pack(-5, -1)))
	c.Equal(geom.NewPoint(-32768, 32767), w32MouseMessagePoint(pack(-32768, 32767)))
	// Bits above the low 32 must not leak in: lParam is 64 bits wide on 64-bit Windows and the system does not
	// guarantee the upper half is zero. (The conversion truncates the high bits away on a 32-bit build, where the
	// case degenerates to the plain negative-coordinate check above.)
	hiBits := ^uint64(0)
	hiBits <<= 32
	c.Equal(geom.NewPoint(-5, -1), w32MouseMessagePoint(w32.LPARAM(hiBits)|pack(-5, -1)))
}
