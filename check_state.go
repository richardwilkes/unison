// Copyright ©2021-2022 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

// Possible values for CheckState.
const (
	OffCheckState CheckState = iota
	OnCheckState
	MixedCheckState
)

// CheckState represents the current state of something like a check box or mark.
type CheckState uint8

// CheckStateFromBool returns the equivalent CheckState.
func CheckStateFromBool(b bool) CheckState {
	if b {
		return OnCheckState
	}
	return OffCheckState
}
