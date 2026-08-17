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
	"sync/atomic"
	"testing"
	"time"

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

// TestInvokeTaskAndWaitMarshalsAndWaits verifies the reason the call exists: the work runs on the UI thread and the
// caller stays blocked until it has. This test mutates global state and therefore must not call t.Parallel.
func TestInvokeTaskAndWaitMarshalsAndWaits(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	withUIThreadIdentity(t)

	uiID := currentGoroutineID()
	returned := make(chan struct{})
	var ranOn atomic.Uint64
	var ranBeforeReturn atomic.Bool
	go func() {
		InvokeTaskAndWait(func() {
			ranOn.Store(currentGoroutineID())
			select {
			case <-returned:
			default:
				ranBeforeReturn.Store(true)
			}
		})
		close(returned)
	}()

	// The function must have been handed to the task queue rather than called in place, and the caller must still be
	// blocked while the task sits there unserviced.
	waitForQueuedTask(t)
	c.Equal(uint64(0), ranOn.Load(), "the function must not run on the calling goroutine")
	select {
	case <-returned:
		t.Fatal("InvokeTaskAndWait returned without waiting for the function to run")
	case <-time.After(50 * time.Millisecond):
	}

	pumpTasksUntil(t, returned)
	c.Equal(uiID, ranOn.Load(), "the function must run on the UI thread")
	c.True(ranBeforeReturn.Load(), "InvokeTaskAndWait must not return before the function has run")
}

// TestInvokeTaskAndWaitRunsDirectlyOnTheUIThread verifies that the UI thread does not wait on itself, which would
// never end. This test mutates global state and therefore must not call t.Parallel.
func TestInvokeTaskAndWaitRunsDirectlyOnTheUIThread(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	withUIThreadIdentity(t)

	ranOn := uint64(0)
	InvokeTaskAndWait(func() { ranOn = currentGoroutineID() })
	c.Equal(currentGoroutineID(), ranOn, "the UI thread must call the function itself")
	length, head := taskQueueState()
	c.Equal(0, length-head, "nothing may be left on the queue")
}

// TestInvokeTaskAndWaitRunsDirectlyBeforeTheEventLoop verifies that a caller is not blocked forever by work handed
// over before Start() reaches its event loop, since nothing would ever drain the queue. This test mutates global state
// and therefore must not call t.Parallel.
func TestInvokeTaskAndWaitRunsDirectlyBeforeTheEventLoop(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	savedInited := platformInited.Load()
	platformInited.Store(false)
	t.Cleanup(func() { platformInited.Store(savedInited) })

	ran := make(chan uint64, 1)
	go func() {
		var ranOn uint64
		InvokeTaskAndWait(func() { ranOn = currentGoroutineID() })
		ran <- ranOn
	}()
	select {
	case ranOn := <-ran:
		c.True(ranOn != 0, "the function must be called rather than queued for an event loop that is not running")
	case <-time.After(5 * time.Second):
		t.Fatal("InvokeTaskAndWait blocked before the event loop was running")
	}
	length, head := taskQueueState()
	c.Equal(0, length-head, "nothing may be left on the queue")
}

// TestInvokeTaskAndWaitReturnsWhenTheFunctionPanics verifies that a panic is handled the way InvokeTask handles one and
// still releases the caller. This test mutates global state and therefore must not call t.Parallel.
func TestInvokeTaskAndWaitReturnsWhenTheFunctionPanics(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	var recovered atomic.Bool
	withRecoveryCallback(t, func(_ error) { recovered.Store(true) })
	withUIThreadIdentity(t)

	// From another goroutine, the panic happens on the UI thread while the caller waits for it.
	returned := make(chan struct{})
	go func() {
		InvokeTaskAndWait(func() { panic("boom") })
		close(returned)
	}()
	waitForQueuedTask(t)
	pumpTasksUntil(t, returned)
	c.True(recovered.Load(), "the panic must be reported through the recovery callback")

	// On the UI thread itself, the panic must not reach the caller either.
	recovered.Store(false)
	c.NotPanics(func() { InvokeTaskAndWait(func() { panic("boom") }) })
	c.True(recovered.Load(), "the panic must be reported through the recovery callback")
}

// TestInvokeTaskAndWaitIgnoresANilFunction verifies that a nil function is a no-op, as it is for SafeCall, rather than
// a panic or a task that outlives the call. This test mutates global state and therefore must not call t.Parallel.
func TestInvokeTaskAndWaitIgnoresANilFunction(t *testing.T) {
	c := check.New(t)
	resetTaskQueue()
	t.Cleanup(resetTaskQueue)
	withRecoveryCallback(t, func(err error) { c.NoError(err) })
	withUIThreadIdentity(t)

	c.NotPanics(func() { InvokeTaskAndWait(nil) })
	length, head := taskQueueState()
	c.Equal(0, length-head, "nothing may be left on the queue")
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
