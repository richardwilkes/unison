// Copyright (c) 2021-2025 by Richard A. Wilkes. All rights reserved.
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
	releaseOnce  sync.Once
	releaseQueue = make(chan func(), 1024)
)

// ReleaseOnUIThread will add f to the release queue and eventually call it on the UI thread.
func ReleaseOnUIThread(f func()) {
	releaseOnce.Do(func() {
		go processReleaseQueue()
	})
	releaseQueue <- f
}

func processReleaseQueue() {
	const minListAlloc = 1 << 12
	var allocation [25]int
	var pos int
	list := make([]func(), 0, minListAlloc)
	for {
		// Collect up the current set of release functions to execute
		timer := time.NewTimer(time.Second / 5)
	inner:
		for {
			select {
			case f := <-releaseQueue:
				list = append(list, f)
				for len(releaseQueue) > 0 {
					list = append(list, <-releaseQueue)
				}
			case <-timer.C:
				break inner
			}
		}
		timer.Stop()

		// If we have any, pass them off to the UI thread for execution
		if len(list) > 0 && glfwInited.Load() {
			funcs := list
			InvokeTask(func() {
				for _, f := range funcs {
					xos.SafeCall(f, nil)
				}
			})
			allocation[pos] = len(list)
			pos++
			if pos >= len(allocation) {
				pos = 0
			}
			peak := 0
			for _, amt := range allocation {
				if amt > peak {
					peak = amt
				}
			}
			if peak < minListAlloc {
				peak = minListAlloc
			}
			list = make([]func(), 0, peak)
		}
	}
}
