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
	"sync/atomic"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// recordingPanel stands in for the cocoa panel handles so the release-on-collect plumbing can be exercised without
// the AppKit panel service, which is unavailable on headless CI and only safely used from the main thread.
type recordingPanel struct {
	released *atomic.Bool
}

func (p recordingPanel) Release() {
	p.released.Store(true)
}

// TestReleasePanelOnCleanupReleasesPanel verifies the mechanism apiNewOpenDialog/apiNewSaveDialog use to free their
// NSOpenPanel/NSSavePanel: once the dialog wrapper becomes unreachable, the panel's Release must run — and run via
// the UI-thread task queue, never directly on the runtime's cleanup goroutine, since AppKit objects must be released
// on the UI thread. Before this, every dialog leaked its panel for the life of the process, because the
// OpenDialog/SaveDialog interfaces have no dispose method. This test mutates the global task queue and therefore must
// not call t.Parallel.
func TestReleasePanelOnCleanupReleasesPanel(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	withRecoveryCallback(t, func(err error) { c.NoError(err) })

	released := &atomic.Bool{}
	// The owner must be big enough to stay out of the runtime's tiny allocator, which batches sub-16-byte pointer-free
	// objects and can delay their cleanups until unrelated neighbors die.
	owner := new([32]byte)
	releasePanelOnCleanup(owner, recordingPanel{released: released})
	owner = nil //nolint:wastedassign,ineffassign // drops the last strong reference so the cleanup can run

	// Wait for the cleanup to enqueue the release as a UI task. Nothing drains the queue yet, so observing the panel
	// still unreleased once a task is pending proves the release is marshaled rather than run on the cleanup
	// goroutine.
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if length, head := taskQueueState(); length-head > 0 {
			break
		}
		runtime.GC()
		time.Sleep(10 * time.Millisecond)
	}
	length, head := taskQueueState()
	if length-head == 0 {
		t.Fatal("collecting the dialog wrapper never enqueued a UI task to release its panel")
	}
	c.False(released.Load(), "the panel must not be released before the UI task queue runs the release")

	drainTasks()
	c.True(released.Load(), "draining the UI task queue should release the collected dialog's panel")
}
