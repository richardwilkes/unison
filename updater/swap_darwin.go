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
	"log/slog"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/sys/unix"
)

// exchange swaps the staged copy and the installed one in a single atomic operation. It returns false if the exchange
// is not available on this filesystem, leaving the caller to fall back to the two-rename form.
//
// macOS has a rename that exchanges two existing paths rather than replacing one with the other. That removes the
// window the two-rename form leaves -- there is no instant at which the application is not at its usual path -- and it
// sidesteps the fact that an ordinary rename cannot replace a non-empty directory, which is what a bundle is.
//
// It also does the work of the backup for free: after the exchange, the previous version is sitting at the staged
// copy's path, so it only has to be moved to where the record says the backup lives.
func exchange(payload, targetPath, backup string) (bool, error) {
	err := retry(renameAttempts, renameDelay, func() error {
		err := unix.RenamexNp(payload, targetPath, unix.RENAME_SWAP)
		if exchangeUnsupported(err) || exchangeRefused(err) {
			// Neither will change on another try.
			return stopRetrying(err)
		}
		return err
	})
	switch {
	case err == nil:
		slog.Info("replacing the installed version", "target", targetPath, "payload", payload, "backup", backup)
		// The previous version is now where the staged copy was. Move it to the recorded backup path so the startup
		// sweep can find it, and so it is not removed along with the staging directory.
		if renameErr := renameWithRetry(payload, backup); renameErr != nil {
			// The update itself succeeded; only the tidying did not. Carry on rather than undoing a good installation
			// over a misplaced backup.
			slog.Warn("unable to move the previous version aside after the update", "error", renameErr)
		}
		return true, nil
	case exchangeUnsupported(err):
		// Not an error worth reporting: the two-rename form works everywhere.
		return false, nil
	case exchangeRefused(err):
		// Refused rather than transiently unavailable, most likely because macOS wants explicit permission for one
		// application to modify another. Retrying cannot help, and there is no way to ask from here.
		return false, errs.NewWithCause("not allowed to replace the installed application", err)
	default:
		slog.Warn("unable to exchange the installed application atomically; falling back", "error", err)
		return false, nil
	}
}

// exchangeUnsupported reports whether the error means the filesystem does not implement the exchange.
func exchangeUnsupported(err error) bool {
	return errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL)
}

// exchangeRefused reports whether the error means the exchange was denied, rather than merely unavailable for the
// moment.
func exchangeRefused(err error) bool {
	return errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES)
}
