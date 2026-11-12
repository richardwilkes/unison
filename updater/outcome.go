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
	"github.com/richardwilkes/toolbox/v2/i18n"
	"github.com/richardwilkes/toolbox/v2/xstrings"
)

// ReportOutcome settles up after an update that was applied while the application was not running, clearing away what
// it left behind and telling the user if anything went wrong. Success is silent: the version the application now
// reports is confirmation enough. Call it once at startup, before anything else looks at the installation directory,
// and only in the instance that owns the installation: an application with a single-instance handoff must first have
// established that this process is the primary one.
func (u *Updater) ReportOutcome() {
	if primary, detail, ok := u.outcomeWarning(u.reportAndSweep()); ok {
		u.warnFn(primary, detail)
	}
}

// outcomeWarning returns what to tell the user about how an update ended, and whether there is anything to tell.
func (u *Updater) outcomeWarning(out *outcome) (primary, detail string, ok bool) {
	if out == nil || out.Reason == reasonNone {
		return "", "", false
	}
	if out.Applied {
		// Installed; only a later step, most likely the relaunch, failed. That isn't worth interrupting a user who is
		// already running the new version.
		return "", "", false
	}
	return i18n.Text("%s was not updated", u.cfg.AppName), xstrings.Wrap("", u.outcomeMessage(out.Reason), 100), true
}

// outcomeMessage explains why a prepared update was not applied. The caller wraps the result to its dialog's width.
func (u *Updater) outcomeMessage(r reason) string {
	switch r {
	case reasonPredecessorRunning:
		return i18n.Text("A copy of %s was still running when the update was to be installed, so nothing was changed. Try the update again, and wait for %s to restart on its own.",
			u.cfg.AppName, u.cfg.AppName)
	case reasonSwapFailed:
		return i18n.Text("The update could not be installed, so the previous version was kept. Downloading it from the %s web site and installing it by hand will work.",
			u.cfg.AppName)
	case reasonVersionMismatch:
		return i18n.Text("The update reported success but a different version is running. The previous version has been kept.")
	case reasonAbandoned:
		return i18n.Text("The update was prepared but never installed, so nothing was changed. Try the update again, or download it from the %s web site and install it by hand.",
			u.cfg.AppName)
	default:
		return i18n.Text("The update could not be installed. Downloading it from the %s web site and installing it by hand will work.",
			u.cfg.AppName)
	}
}
