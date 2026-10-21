// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package secrets

import "github.com/richardwilkes/unison/internal/dbus"

// newPlatformStore returns a store backed by the freedesktop Secret Service, reached over the session bus connection
// that the whole process shares.
func newPlatformStore() Store {
	return newSecretServiceStore(dbus.Session)
}
