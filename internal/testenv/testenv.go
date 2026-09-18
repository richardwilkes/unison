// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

// Package testenv describes the environment the tests are running in, so that a test which cannot be trusted there can
// step aside rather than fail for reasons that have nothing to do with the code under test.
package testenv

import (
	"runtime"
	"testing"
)

// SkipTimingSensitive skips the calling test on macOS Intel. A test that depends on the wall clock, or on the scheduler
// treating its goroutines fairly, needs a machine that keeps up, and the macOS Intel CI runner does not: it is slow
// enough that such tests have failed there repeatedly with nothing wrong, and each round of loosening their deadlines
// only postponed the next failure. The tests still run on every other platform, which is where a real regression shows
// up just as well. The decision is made from the architecture rather than by asking whether CI is running, so a test
// run on an Intel Mac behaves the same everywhere.
func SkipTimingSensitive(t testing.TB) {
	t.Helper()
	if runtime.GOOS == "darwin" && runtime.GOARCH == "amd64" {
		t.Skip("timing-sensitive tests are not run on macOS Intel, whose CI runner is too slow for them to be reliable")
	}
}
