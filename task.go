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
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

var (
	taskQueueLock sync.Mutex
	taskQueue     []func()
	taskQueueHead int
)

// InvokeTask calls a function on the UI thread. The function is put into the system event queue and will be run at the
// next opportunity.
func InvokeTask(f func()) {
	taskQueueLock.Lock()
	taskQueue = append(taskQueue, f)
	taskQueueLock.Unlock()
	apiPostEmptyEvent()
}

// InvokeTaskAfter schedules a function to be run on the UI thread after waiting for the specified duration.
func InvokeTaskAfter(f func(), after time.Duration) {
	time.AfterFunc(after, func() { InvokeTask(f) })
}

// InvokeTaskAndWait calls a function on the UI thread and does not return until it has run. Use it when the calling
// goroutine may not proceed until the work is done, such as an exit function that must not let the process terminate
// before the UI state it reads has been written out.
//
// The function is called directly, on the calling goroutine, if the caller is the UI thread or if Start() has not yet
// reached its event loop, since a wait for that loop would never end in either case. A panic inside the function is
// recovered and reported the same way it is for InvokeTask, rather than delivered to the caller.
//
// The wait is unbounded, because the UI thread may legitimately be busy for an arbitrary length of time, such as when
// a modal dialog waits for an answer from the user. Do not call this from work that the UI thread itself waits on,
// since that deadlocks.
func InvokeTaskAndWait(f func()) {
	if f == nil {
		return
	}
	if !platformInited.Load() || onUIThread() {
		SafeCall(f)
		return
	}
	done := make(chan struct{})
	InvokeTask(func() {
		defer close(done)
		f()
	})
	<-done
}

func processNextTask() {
	var f func()
	needsPost := false
	taskQueueLock.Lock()
	if taskQueueHead < len(taskQueue) {
		f = taskQueue[taskQueueHead]
		taskQueue[taskQueueHead] = nil // release the closure for GC
		taskQueueHead++
		if taskQueueHead == len(taskQueue) {
			// Fully drained: reset to reuse the backing array.
			taskQueue = taskQueue[:0]
			taskQueueHead = 0
		} else {
			needsPost = true
			// If the dead prefix has grown large relative to the live tail, compact it to the front so the
			// backing array doesn't grow without bound when the queue is never fully drained.
			if taskQueueHead >= 1024 && taskQueueHead > len(taskQueue)-taskQueueHead {
				n := copy(taskQueue, taskQueue[taskQueueHead:])
				clear(taskQueue[n:]) // drop references in the vacated tail
				taskQueue = taskQueue[:n]
				taskQueueHead = 0
			}
		}
	}
	taskQueueLock.Unlock()
	if f != nil {
		SafeCall(f)
		if needsPost {
			apiPostEmptyEvent()
		}
	}
}

// SafeCall uses xos.SafeCall() and the callback, if any, set with the RecoveryCallback startup option to safely call a
// function. If 'f' is nil, this function returns immediately.
func SafeCall(f func()) {
	if f == nil {
		return
	}
	xos.SafeCall(f, func(err error) {
		if recoveryCallback != nil {
			// Note that it is not necessary to wrap this callback inside another xos.SafeCall, as xos.PanicRecovery
			// (called from xos.SafeCall) does that for us.
			recoveryCallback(err)
		} else {
			errs.Log(err)
		}
	})
}
