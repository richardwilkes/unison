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
// apiFinalFinishStartup. Before the fix, it held macPendingFilesLock while invoking the user's openFilesCallback; if
// that callback pumped events (e.g. via RunModal) and AppKit delivered another open-files request during the nested
// loop, macOpenFilesRequested re-locked the same non-reentrant mutex on the same thread and deadlocked. The callback
// here simulates that by calling macOpenFilesRequested directly from within openFilesCallback, and the test runs
// apiFinalFinishStartup on a separate goroutine so a regression shows up as a timeout failure rather than hanging the
// test binary. It also verifies the ordering guarantees: files buffered before startup completes are delivered
// synchronously and exactly once, while requests arriving after the flag flips are routed through the task queue.
// This test mutates global state and therefore must not call t.Parallel.
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
		apiFinalFinishStartup()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("apiFinalFinishStartup deadlocked on a reentrant open-files request")
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
