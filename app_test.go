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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison/enums/thememode"
)

// TestThemeModeConcurrentAccess verifies that CurrentThemeMode may be called from arbitrary goroutines while the UI
// thread changes the mode via SetThemeMode. The platform theme monitors (e.g. the Windows registry watcher goroutine)
// read the mode this way, so the access must be race-free: prior to currentThemeMode becoming atomic, this test failed
// under the race detector. This test shares global state and therefore must not call t.Parallel.
func TestThemeModeConcurrentAccess(t *testing.T) {
	c := check.New(t)
	resetTaskQueue() // SetThemeMode queues a ThemeChanged task; keep the shared queue clean for other tests.
	prev := CurrentThemeMode()
	t.Cleanup(func() {
		SetThemeMode(prev)
		resetTaskQueue()
	})

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					mode := CurrentThemeMode()
					c.True(mode == thememode.Auto || mode == thememode.Dark || mode == thememode.Light,
						"unexpected theme mode")
				}
			}
		}()
	}
	for i := range 1000 {
		if i%2 == 0 {
			SetThemeMode(thememode.Dark)
		} else {
			SetThemeMode(thememode.Light)
		}
	}
	close(stop)
	wg.Wait()
	c.Equal(thememode.Light, CurrentThemeMode())
}
