// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import "strings"

const (
	// backupSuffix separates the displaced installation's original name from the unique suffix that follows it. See
	// target.backupPath for why the resulting name is shaped the way it is.
	backupSuffix = ".old-"
	// helperName is the copy of the running application that finishes the update after that application exits. It is a
	// copy rather than the installed file so that, once the application has quit, nothing under the installation
	// directory is in use by any of the application's processes, which is what makes the swap and the cleanup
	// unconditionally safe -- notably on Windows, where a running image cannot be deleted. It names a file everywhere
	// except macOS, where it names a copy of the whole bundle; see stageHelper. It is the base name only: Windows
	// requires an ".exe" extension on top of it before the copy can be started at all; see helperFileName.
	helperName = "helper"
	// stateName is the record shared between the application that stages an update, the helper that applies it, and the
	// newly installed application that reports and cleans up after it.
	stateName = "update-state.json"
	// logName is the helper's log. The helper has no UI, so this and the application log are the only places its
	// failures can surface.
	logName = "update.log"
)

// workDirPrefix names the staging directory. It is created inside the installation's directory, not in the system
// temporary directory, so that moving the staged copy into place is a rename within one filesystem -- atomic and
// instant -- rather than a copy across two. The leading dot keeps it out of ordinary directory listings on macOS and
// Linux; Windows Explorer shows it.
func (u *Updater) workDirPrefix() string {
	return "." + u.cfg.CmdName + "-update-"
}

// downloadDirPrefix names the directory in the system temporary directory that the release is downloaded into.
func (u *Updater) downloadDirPrefix() string {
	return u.cfg.CmdName + "-download-"
}

// isLeftover reports whether name, an entry in the directory holding the installation whose base name is targetBase, is
// something an update to that installation leaves behind: a staging directory, or a displaced installation. The startup
// sweep removes these once they are old enough. Displaced copies of any other application sharing the directory, such
// as /Applications, are not this one's to remove.
func (u *Updater) isLeftover(name, targetBase string) bool {
	return strings.HasPrefix(name, u.workDirPrefix()) || strings.HasPrefix(name, "."+targetBase+backupSuffix)
}
