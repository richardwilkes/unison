// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package secrets

import (
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// TestUseStoreNilSelectsThePlatformStore verifies that UseStore(nil) puts the platform store in use over the store
// installed before it. The platform store is only compared, never called.
func TestUseStoreNilSelectsThePlatformStore(t *testing.T) {
	c := check.New(t)
	memory := NewMemoryStore()
	restoreMemory := UseStore(memory)
	defer restoreMemory()
	c.True(current() == memory, "the memory store should be in use")
	restoreNil := UseStore(nil)
	c.True(current() == platformStore(), "a nil store should select the platform store")
	restoreNil()
	c.True(current() == memory, "restoring should bring back the memory store")
}
