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
	"log/slog"
	"os"

	"github.com/richardwilkes/toolbox/v2/errs"
)

// relaunch starts the newly installed application.
//
// A bundle is started through `open` rather than by running the executable inside it, which hands the launch to Launch
// Services and so gives the application a Dock icon, foreground activation, and its own place in the session. A process
// started directly gets none of those, and inherits the helper's identity for privacy permissions, so the user could be
// asked again for access they had already granted.
//
// The application is named by absolute path rather than by bundle identifier, so Launch Services cannot resolve the
// request to some other copy on the machine.
//
// Only the output of `open` is appended to logPath, the update's log, where it is the only record of why Launch
// Services refused to start the application; `open` exits at once and does not pass its streams on, so it cannot hold
// the log open. The application's own output is discarded, since the log is in the staging directory the application
// removes once it has started.
func relaunch(t *target, logPath string) error {
	if t.Kind != kindBundle {
		_, err := spawnDetached(t.Path, nil, "", os.TempDir())
		return err
	}
	if _, err := spawnDetached("/usr/bin/open", []string{t.Path}, logPath, os.TempDir()); err != nil {
		// The executable inside the bundle gets the application running, just without a proper foreground activation,
		// which is better than leaving the user staring at nothing.
		slog.Warn("unable to start the updated application through Launch Services; starting it directly",
			"error", err)
		if _, fallbackErr := spawnDetached(t.execWithin(t.Path), nil, "", os.TempDir()); fallbackErr != nil {
			return errs.NewWithCause("unable to start the updated application", fallbackErr)
		}
	}
	return nil
}
