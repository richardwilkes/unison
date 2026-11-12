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

// exchange reports that no atomic two-path exchange is available, leaving the caller to use the two-rename form.
//
// Linux has renameat2 with RENAME_EXCHANGE, which would serve, but it is refused by several filesystems in common use
// and needs a runtime fallback anyway; Windows has no equivalent at all. What is replaced on both platforms is a single
// file rather than a directory tree, so the two-rename form is straightforward, and its window -- the microseconds
// between moving the old file aside and moving the new one in -- is covered by the startup repair.
func exchange(_, _, _ string) (bool, error) {
	return false, nil
}
