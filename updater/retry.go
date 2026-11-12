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
	"time"
)

// stopRetry is what an operation returns from retry when a failure is final. retry unwraps it and returns the cause at
// once.
type stopRetry struct {
	cause error
}

func (s stopRetry) Error() string { return s.cause.Error() }

func (s stopRetry) Unwrap() error { return s.cause }

// stopRetrying wraps err so that retry gives up immediately and returns err rather than trying again.
func stopRetrying(err error) error {
	return stopRetry{cause: err}
}

// retry calls op until it succeeds, making at most attempts calls and sleeping for delay between them. The error from
// the last attempt is returned when none succeeds. An error op wrapped with stopRetrying is returned right away,
// unwrapped. op is always called at least once, whatever attempts says.
func retry(attempts int, delay time.Duration, op func() error) error {
	attempts = max(attempts, 1)
	var err error
	for i := range attempts {
		if err = op(); err == nil {
			return nil
		}
		if stop, ok := errors.AsType[stopRetry](err); ok {
			return stop.cause
		}
		if i < attempts-1 {
			time.Sleep(delay)
		}
	}
	return err
}
