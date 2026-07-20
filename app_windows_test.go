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
	"runtime"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/w32"
	"golang.org/x/sys/windows"
)

// TestStuckModifierTableUsesVirtualKeyCodes verifies that the stuck-modifier-release hack queries GetKeyState with
// virtual-key codes. Prior to the fix, the table held the keys' scan codes (0x2A, 0x36, 0x15B, 0x15C), so
// GetKeyState(0x2A) actually tested VK_PRINT and GetKeyState(0x36) tested the '6' key, causing a spurious keyReleased
// to be synthesized on the next event-loop pass while Shift or a Win key was genuinely held.
func TestStuckModifierTableUsesVirtualKeyCodes(t *testing.T) {
	c := check.New(t)
	expected := map[KeyCode]int{
		KeyLShift:   0xA0, // VK_LSHIFT
		KeyRShift:   0xA1, // VK_RSHIFT
		KeyLCommand: 0x5B, // VK_LWIN
		KeyRCommand: 0x5C, // VK_RWIN
	}
	c.Equal(len(expected), len(w32StuckModifierKeys))
	for _, k := range w32StuckModifierKeys {
		c.Equal(expected[k.key], k.virtualKey)
	}
}

// TestCollectStuckModifiers verifies the decision logic: a key is only reported as stuck when the window believes it
// is pressed but the OS reports it as up.
func TestCollectStuckModifiers(t *testing.T) {
	c := check.New(t)

	// A fake GetKeyState that reports keys in the down set as pressed. The old bug queried scan codes, so simulating
	// "left Shift is physically held" by reporting only VK_LSHIFT as down distinguishes correct from broken lookups:
	// the buggy code queried 0x2A instead, saw "up", and synthesized a phantom release.
	fakeKeyState := func(down ...int) func(int) uint16 {
		return func(virtualKey int) uint16 {
			for _, d := range down {
				if d == virtualKey {
					return 0x8000
				}
			}
			return 0
		}
	}

	// Left Shift genuinely held: pressed in the window and reported down by the OS. Nothing should be released.
	pressed := map[KeyCode]bool{KeyLShift: true}
	c.Equal(0, len(w32CollectStuckModifiers(pressed, fakeKeyState(0xA0))))

	// Regression for the original finding: with the same physical state, a lookup keyed by the old scan code (0x2A)
	// finds nothing down and would flag left Shift as stuck.
	c.Equal([]KeyCode{KeyLShift}, w32CollectStuckModifiers(pressed, fakeKeyState(0x2A)))

	// Genuinely stuck: the window thinks right Shift and the right Win key are held, but the OS says everything is up.
	pressed = map[KeyCode]bool{KeyRShift: true, KeyRCommand: true}
	c.Equal([]KeyCode{KeyRShift, KeyRCommand}, w32CollectStuckModifiers(pressed, fakeKeyState()))

	// Keys the window never saw pressed are ignored no matter what the OS reports.
	c.Equal(0, len(w32CollectStuckModifiers(map[KeyCode]bool{}, fakeKeyState())))
}

// TestPostEmptyEventWakesMainThreadWithoutWindows verifies that apiPostEmptyEvent, called from another goroutine with
// no windows open, delivers WM_NULL to the main (UI) thread's message queue. Prior to the fix, it read windowList —
// UI-thread-only state — from arbitrary goroutines, a data race, and with an empty windowList it fell back to
// PostMessageW(0, WM_NULL), which posts to the *calling* thread's queue, so the main loop blocked in WaitMessage was
// never woken. The test locks the current goroutine to its OS thread, designates that thread as the main thread, and
// confirms the wakeup message arrives on it. This test shares global state and therefore must not call t.Parallel.
func TestPostEmptyEventWakesMainThreadWithoutWindows(t *testing.T) {
	c := check.New(t)
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Ensure this thread has a message queue (the first PeekMessageW call creates it) and drain anything pending so a
	// stray message cannot satisfy the check below. Note that a min/max filter of WM_NULL cannot be used here or
	// below, since a 0/0 filter range means "all messages" to PeekMessageW.
	var msg w32.MSG
	for w32.PeekMessageW(&msg, 0, 0, 0, w32.PM_REMOVE) {
	}

	prevInited := platformInited.Load()
	prevThreadID := w32MainThreadID.Load()
	platformInited.Store(true)
	w32MainThreadID.Store(windows.GetCurrentThreadId())
	defer func() {
		platformInited.Store(prevInited)
		w32MainThreadID.Store(prevThreadID)
	}()

	// The finding is specifically about the no-window case, which used to post to the wrong thread's queue.
	c.Equal(0, len(windowList))

	done := make(chan struct{})
	go func() {
		// This goroutine necessarily runs on a different OS thread, since the test goroutine has this one locked.
		apiPostEmptyEvent()
		close(done)
	}()
	<-done

	received := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		if !w32.PeekMessageW(&msg, 0, 0, 0, w32.PM_REMOVE) {
			time.Sleep(time.Millisecond)
			continue
		}
		if msg.Hwnd == 0 && msg.Message == w32.WM_NULL {
			received = true
			break
		}
	}
	c.True(received, "WM_NULL never arrived on the designated main thread's queue")
}
