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

func processNextTask(recoveryHandler func(error)) {
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
		xos.SafeCall(f, recoveryHandler)
		if needsPost {
			apiPostEmptyEvent()
		}
	}
}
