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

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/internal/cocoa"
)

// recordingPanel stands in for the cocoa panel handles so the release-on-collect plumbing can be exercised without
// the AppKit panel service, which is unavailable on headless CI and only safely used from the main thread.
type recordingPanel struct {
	released *atomic.Bool
}

func (p recordingPanel) Release() {
	p.released.Store(true)
}

// TestSetAllowedFileTypesOwnership is the regression test for the allowed-file-types leak: SetAllowedExtensions used
// to pass the owned array from cocoa.NewArrayFromStringSlice inline to SetAllowedFileTypes and never release it,
// leaking one NSArray plus its NSStrings per call. The fake setter stands in for the panel property's own
// copy/retain, so after the helper returns, that single reference must be the only one left — the helper's creating
// (+1) reference has to have been dropped.
func TestSetAllowedFileTypesOwnership(t *testing.T) {
	c := check.New(t)
	var captured cocoa.Array
	setAllowedFileTypes(func(a cocoa.Array) {
		captured = a
		cocoa.Retain(objc.ID(a))
	}, []string{"png", "jpg"})
	c.Equal([]string{"png", "jpg"}, captured.ArrayOfStringToStringSlice())
	c.Equal(uint64(1), objc.Send[uint64](objc.ID(captured), cocoa.Sel("retainCount")),
		"the helper must release its own reference to the array once the setter has run")
	captured.Release()

	// An empty list must clear the property with a nil handle rather than allocating an empty array.
	var cleared []cocoa.Array
	setAllowedFileTypes(func(a cocoa.Array) { cleared = append(cleared, a) }, nil)
	c.Equal(1, len(cleared))
	c.Equal(cocoa.Array(0), cleared[0])
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
