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
	"slices"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// resetTaskQueue clears the package-level task queue so each test starts from a known state. These tests share global
// state and therefore must not call t.Parallel.
func resetTaskQueue() {
	taskQueueLock.Lock()
	taskQueue = nil
	taskQueueHead = 0
	taskQueueLock.Unlock()
}

func taskQueueState() (length, head int) {
	taskQueueLock.Lock()
	defer taskQueueLock.Unlock()
	return len(taskQueue), taskQueueHead
}

// withRecoveryCallback installs f as the recovery callback used by SafeCallWithRecovery (and thus processNextTask) for
// the duration of the test, restoring the previous one afterward. These tests share global state and therefore must not
// call t.Parallel.
func withRecoveryCallback(t *testing.T, f func(error)) {
	t.Helper()
	prev := recoveryCallback
	recoveryCallback = f
	t.Cleanup(func() { recoveryCallback = prev })
}

func TestProcessNextTaskRunsInFIFOOrder(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	withRecoveryCallback(t, func(err error) { c.NoError(err) })

	const count = 5
	var order []int
	for i := range count {
		InvokeTask(func() { order = append(order, i) })
	}
	length, head := taskQueueState()
	c.Equal(count, length)
	c.Equal(0, head)

	for range count {
		processNextTask()
	}

	// A fully drained queue must reset so the backing array is reused rather than growing without bound.
	length, head = taskQueueState()
	c.Equal(0, length)
	c.Equal(0, head)
	c.True(slices.Equal(order, []int{0, 1, 2, 3, 4}))

	// Draining an empty queue is a no-op and must not panic or alter state.
	processNextTask()
	length, head = taskQueueState()
	c.Equal(0, length)
	c.Equal(0, head)
}

func TestProcessNextTaskRecoversFromPanic(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()

	var recovered error
	withRecoveryCallback(t, func(err error) { recovered = err })

	ran := false
	InvokeTask(func() { panic("boom") })
	InvokeTask(func() { ran = true })

	processNextTask()
	c.NotNil(recovered)

	// A panic in one task must not corrupt the queue; the following task still runs.
	recovered = nil
	processNextTask()
	c.Nil(recovered)
	c.True(ran)
	length, head := taskQueueState()
	c.Equal(0, length)
	c.Equal(0, head)
}

func TestProcessNextTaskCompactsDeadPrefix(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	withRecoveryCallback(t, func(err error) { c.NoError(err) })

	// Enough tasks that, partway through draining, the dead prefix exceeds both the 1024 threshold and the live
	// tail, triggering compaction.
	const count = 3000
	var order []int
	for i := range count {
		InvokeTask(func() { order = append(order, i) })
	}

	compacted := false
	for range count {
		processNextTask()
		// Compaction is the only path that resets head to 0 while tasks remain queued; a full drain resets both
		// length and head to 0.
		if length, head := taskQueueState(); head == 0 && length > 0 {
			compacted = true
		}
	}
	c.True(compacted)

	length, head := taskQueueState()
	c.Equal(0, length)
	c.Equal(0, head)

	want := make([]int, count)
	for i := range want {
		want[i] = i
	}
	c.True(slices.Equal(order, want))
}
