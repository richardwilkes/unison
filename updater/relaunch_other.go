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
func relaunch(t *target, logPath string) error {
	if err := spawnDetached(t.Path, nil, logPath, os.TempDir()); err != nil {
		return errs.NewWithCause("unable to start the updated application", err)
	}
	return nil
}
