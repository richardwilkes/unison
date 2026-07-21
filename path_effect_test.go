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
)

func TestDashEffectConcurrentInit(t *testing.T) {
	const goroutines = 16
	results := make([]*PathEffect, goroutines)
	var start, done sync.WaitGroup
	start.Add(1)
	done.Add(goroutines)
	for i := range goroutines {
		go func() {
			defer done.Done()
			start.Wait()
			results[i] = DashEffect()
		}()
	}
	start.Done()
	done.Wait()
	c := check.New(t)
	c.NotNil(results[0])
	for i := 1; i < goroutines; i++ {
		c.Equal(results[0], results[i])
	}
}
