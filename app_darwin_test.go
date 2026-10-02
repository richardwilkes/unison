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
)

// TestFinalFinishStartupSurvivesReentrantOpenFiles is the regression test for a self-deadlock in
// nativeFinalFinishStartup. Before the fix, it held macPendingFilesLock while invoking the user's openFilesCallback; if
// that callback pumped events (e.g. via RunModal) and AppKit delivered another open-files request during the nested
// loop, macOpenFilesRequested re-locked the same non-reentrant mutex on the same thread and deadlocked. The callback
// here simulates that by calling macOpenFilesRequested directly from within openFilesCallback, and the test runs
// nativeFinalFinishStartup on a separate goroutine so a regression shows up as a timeout failure rather than hanging
// the test binary. It also verifies the ordering guarantees: files buffered before startup completes are delivered
// synchronously and exactly once, while requests arriving after the flag flips are routed through the task queue. This
// test mutates global state and therefore must not call t.Parallel.
func TestFinalFinishStartupSurvivesReentrantOpenFiles(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	withRecoveryCallback(t, func(err error) { c.NoError(err) })

	savedCallback := openFilesCallback
	macPendingFilesLock.Lock()
	savedPending := macPendingFilesToOpen
	savedMayIssue := macMayIssueFileOpens
	macPendingFilesToOpen = []string{"a", "b"}
	macMayIssueFileOpens = false
	macPendingFilesLock.Unlock()
	t.Cleanup(func() {
		openFilesCallback = savedCallback
		macPendingFilesLock.Lock()
		macPendingFilesToOpen = savedPending
		macMayIssueFileOpens = savedMayIssue
		macPendingFilesLock.Unlock()
	})

	var received [][]string
	openFilesCallback = func(paths []string) {
		received = append(received, paths)
		if len(received) == 1 {
			// Simulate AppKit delivering another open-files request while the callback pumps a nested event loop.
			macOpenFilesRequested([]string{"c"})
		}
	}

	done := make(chan struct{})
	go func() {
		nativeFinalFinishStartup()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("nativeFinalFinishStartup deadlocked on a reentrant open-files request")
	}

	// The buffered files must have been delivered synchronously and the buffer cleared.
	c.Equal(1, len(received))
	c.Equal([]string{"a", "b"}, received[0])
	macPendingFilesLock.Lock()
	c.True(macMayIssueFileOpens)
	c.Equal(0, len(macPendingFilesToOpen))
	macPendingFilesLock.Unlock()

	// The reentrant request arrived after startup completed, so it must have been queued as a task rather than
	// buffered or invoked inline; draining the queue delivers it.
	length, head := taskQueueState()
	c.Equal(1, length)
	c.Equal(0, head)
	processNextTask()
	c.Equal(2, len(received))
	c.Equal([]string{"c"}, received[1])
}

// TestMacFilesRequestedAtLaunchReturnsCopy verifies that the snapshot FilesRequestedAtLaunch returns on macOS is a copy
// of the buffered open-files requests that leaves the buffer intact, so that a later Start still delivers them to the
// OpenFilesCallback: changing the copy must not reach the buffer, and requests buffered afterward must not reach the
// copy. This test mutates global state and therefore must not call t.Parallel.
func TestMacFilesRequestedAtLaunchReturnsCopy(t *testing.T) {
	c := check.New(t)
	macPendingFilesLock.Lock()
	savedPending := macPendingFilesToOpen
	savedMayIssue := macMayIssueFileOpens
	macPendingFilesToOpen = nil
	macMayIssueFileOpens = false
	macPendingFilesLock.Unlock()
	t.Cleanup(func() {
		macPendingFilesLock.Lock()
		macPendingFilesToOpen = savedPending
		macMayIssueFileOpens = savedMayIssue
		macPendingFilesLock.Unlock()
	})

	c.Nil(macFilesRequestedAtLaunch(), "nothing has been requested yet")

	macOpenFilesRequested([]string{"/a", "/b"})
	files := macFilesRequestedAtLaunch()
	c.Equal([]string{"/a", "/b"}, files)
	files[0] = "/changed"
	macPendingFilesLock.Lock()
	c.Equal([]string{"/a", "/b"}, macPendingFilesToOpen, "changing the copy must not change the buffer")
	macPendingFilesLock.Unlock()

	macOpenFilesRequested([]string{"/c"})
	c.Equal([]string{"/changed", "/b"}, files, "a later request must not change the copy")
	c.Equal([]string{"/a", "/b", "/c"}, macFilesRequestedAtLaunch(), "taking a copy must not drain the buffer")
	macPendingFilesLock.Lock()
	c.Equal([]string{"/a", "/b", "/c"}, macPendingFilesToOpen)
	macPendingFilesLock.Unlock()
}

// TestAPIWithAutoreleasePool verifies the wrapper finishProcessingEvents brackets its work with runs the function it
// is given (inside a real autorelease pool on macOS, so autoreleased objects created by tasks and draws are
// reclaimed each pass instead of accumulating until process exit).
func TestAPIWithAutoreleasePool(t *testing.T) {
	c := check.New(t)
	ran := false
	nativeWithAutoreleasePool(func() { ran = true })
	c.True(ran)
}
