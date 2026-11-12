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
	"os"
	"path/filepath"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
)

const (
	// renameAttempts and renameDelay bound the retries around each rename. They exist for Windows, where antivirus
	// scanners and shell extensions routinely take a brief handle on a freshly written executable and a rename fails
	// with an access error that is gone a moment later. They are harmless elsewhere, where the first attempt succeeds.
	renameAttempts = 10
	renameDelay    = 200 * time.Millisecond
)

// renameFunc and exchangeFunc are the operations swap is built from. They are variables so a test can make a chosen
// step fail and check that the recovery actually restores the installation -- otherwise an unreachable path, and the
// one that matters most, since getting it wrong leaves the user with no application at all.
//
// Substituting exchangeFunc also makes the two-rename fallback reachable on macOS, where the atomic exchange would
// normally be taken. That fallback is not test-only there: it is what runs on any filesystem whose driver does not
// implement the exchange.
var (
	renameFunc   = os.Rename
	exchangeFunc = exchange
)

// swap replaces the installation at targetPath with the staged copy at payload, moving what was there to backup.
//
// The installation is either the old one or the new one at every moment a crash could occur. On macOS this is normally
// a single atomic exchange, so there is no moment at all where the application is missing. Otherwise it is two renames,
// leaving a window of microseconds between them; a crash inside that window leaves the backup intact, which the startup
// repair recovers from.
//
// If the second rename fails, the first is undone before returning, so a failure leaves the installation as it was.
func swap(targetPath, payload, backup string) error {
	if err := validateSwap(targetPath, payload, backup); err != nil {
		return err
	}
	if swapped, err := exchangeFunc(payload, targetPath, backup); swapped || err != nil {
		return err
	}
	if err := renameWithRetry(targetPath, backup); err != nil {
		// Nothing has moved, so there is nothing to undo.
		return errs.NewWithCause("unable to move the existing version aside", err)
	}
	if err := renameWithRetry(payload, targetPath); err != nil {
		if restoreErr := renameWithRetry(backup, targetPath); restoreErr != nil {
			// The destination name is free and the source exists, so this should not be reachable. If it ever is, the
			// installation is missing and only the startup repair can put it back, which is why the backup's location
			// is recorded before any of this begins.
			errs.Log(errs.NewWithCause("unable to restore the existing version after a failed update", restoreErr),
				"backup", backup, "target", targetPath)
		}
		return errs.NewWithCause("unable to put the new version in place", err)
	}
	return nil
}

// validateSwap checks the preconditions that make the renames below safe, before any of them are attempted.
func validateSwap(targetPath, payload, backup string) error {
	if targetPath == "" || payload == "" || backup == "" {
		return errs.New("the update is missing the paths it needs")
	}
	if filepath.Dir(targetPath) != filepath.Dir(backup) {
		// A backup in another directory could be on another filesystem, which would turn the move into a copy: no
		// longer atomic, and no longer something that can be undone by renaming it back.
		return errs.New("the backup must be beside what it replaces")
	}
	if _, err := os.Lstat(backup); err == nil {
		// The installation is about to be renamed to this path, and on most systems that would replace whatever is
		// already there without a word, destroying it if it happened to be a backup from an earlier update. The name
		// carries a timestamp, so reaching this means something is wrong; refusing costs one update, while being wrong
		// here costs the fallback.
		return errs.New("something is already where the previous version would be moved to")
	}
	payloadInfo, err := os.Lstat(payload)
	if err != nil {
		return errs.NewWithCause("the prepared update is missing", err)
	}
	targetInfo, err := os.Lstat(targetPath)
	if err != nil {
		return errs.NewWithCause("the installed version is missing", err)
	}
	if payloadInfo.IsDir() != targetInfo.IsDir() {
		// A bundle replacing a bare executable, or the reverse, means something upstream resolved the wrong thing.
		return errs.New("the prepared update does not match what it would replace")
	}
	return nil
}

// renameWithRetry renames a path, retrying briefly before giving up. See renameAttempts for why.
func renameWithRetry(from, to string) error {
	return errs.Wrap(retry(renameAttempts, renameDelay, func() error { return renameFunc(from, to) }))
}
