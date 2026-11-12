// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import (
	"errors"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// neverSleep is a delay no test can afford to wait out. Passing it proves a path does not sleep at all: if it did, the
// test would hang until the harness killed it.
const neverSleep = time.Hour

func TestRetryStopsAtTheFirstSuccess(t *testing.T) {
	c := check.New(t)
	calls := 0
	err := retry(5, 0, func() error {
		calls++
		if calls < 3 {
			return errors.New("not yet")
		}
		return nil
	})
	c.NoError(err)
	c.Equal(3, calls)
}

// The common case, on the platforms where the first attempt always works, must cost nothing.
func TestRetryDoesNotSleepAfterAnImmediateSuccess(t *testing.T) {
	c := check.New(t)
	calls := 0
	c.NoError(retry(5, neverSleep, func() error {
		calls++
		return nil
	}))
	c.Equal(1, calls)
}

// The error reported must be the final attempt's, which is the one describing the state things were left in.
func TestRetryReturnsTheLastErrorWhenEveryAttemptFails(t *testing.T) {
	c := check.New(t)
	calls := 0
	failures := []error{errors.New("first"), errors.New("second"), errors.New("third")}
	err := retry(len(failures), 0, func() error {
		calls++
		return failures[calls-1]
	})
	c.Equal(len(failures), calls)
	c.True(errors.Is(err, failures[len(failures)-1]), "the last attempt's error should be the one returned")
}

// The escape hatch the atomic exchange depends on: a refusal or an unsupported operation must come back immediately and
// unwrapped, not after the full run of sleeps.
func TestRetryGivesUpAtOnceWhenTold(t *testing.T) {
	c := check.New(t)
	cause := errors.New("refused")
	calls := 0
	err := retry(5, neverSleep, func() error {
		calls++
		return stopRetrying(cause)
	})
	c.Equal(1, calls)
	c.Equal(cause, err, "the cause should come back as given, not wrapped")
}

// Guards against a zero or negative attempt count turning into a silent success that never ran the operation at all.
func TestRetryAlwaysMakesOneAttempt(t *testing.T) {
	c := check.New(t)
	for _, attempts := range []int{0, -1} {
		calls := 0
		err := retry(attempts, neverSleep, func() error {
			calls++
			return errors.New("failed")
		})
		c.HasError(err)
		c.Equal(1, calls)
	}
}
