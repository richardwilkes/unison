// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

//go:build !darwin

package updater

import (
	"os"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// relaunch starts the newly installed application. Linux and Windows install a single executable, so there is nothing
// to do but run it.
//
// It is started with no arguments: reopening whatever files the user had open would mean handing paths recorded before
// the quit to a process that starts a minute later, and passing a stale path would be worse than passing none.
//
// It takes the update's log only to match the macOS version, which records the output of `open` there. The
// application's own output is discarded rather than added to that log: the log is in the staging directory, which the
// application removes once it has started, and on Windows its handle to the log would prevent that while it ran.
func relaunch(t *target, _ string) error {
	if err := spawnDetached(t.Path, nil, "", os.TempDir()); err != nil {
		return errs.NewWithCause("unable to start the updated application", err)
	}
	return nil
}
