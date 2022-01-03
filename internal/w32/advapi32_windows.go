// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package w32

import (
	"syscall"

	"golang.org/x/sys/windows/registry"
)

var (
	advapi32                    = syscall.NewLazyDLL("advapi32.dll")
	regNotifyChangeKeyValueProc = advapi32.NewProc("RegNotifyChangeKeyValue")
)

type RegNotifyMask int

// Mask values for RegNotifyMask
const (
	RegNotifyChangeName RegNotifyMask = 1 << iota
	RegNotifyChangeAttributes
	RegNotifyChangeLastSet
	RegNotifyChangeSecurity
	RegNotifyThreadAgnostic RegNotifyMask = 1 << 28
)

func boolParam(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// RegNotifyChangeKeyValue https://docs.microsoft.com/en-us/windows/win32/api/winreg/nf-winreg-regnotifychangekeyvalue
func RegNotifyChangeKeyValue(key registry.Key, watchSubTree bool, notifyFilter RegNotifyMask, event syscall.Handle, async bool) int {
	result, _, _ := regNotifyChangeKeyValueProc.Call(uintptr(key), boolParam(watchSubTree), uintptr(notifyFilter),
		uintptr(event), boolParam(async))
	return int(result)
}
