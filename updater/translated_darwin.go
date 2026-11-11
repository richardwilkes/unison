// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package updater

import "golang.org/x/sys/unix"

// runningTranslated reports whether this process is an Intel binary being translated by Rosetta on Apple silicon. The
// sysctl is absent on Intel hardware, which reads the same as "not translated".
func runningTranslated() bool {
	translated, err := unix.SysctlUint32("sysctl.proc_translated")
	return err == nil && translated == 1
}
