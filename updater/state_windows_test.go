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
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"golang.org/x/sys/windows"
)

// stateHold is how long these tests keep the state file from the code under test, which is far less than that code
// waits for it.
const stateHold = 100 * time.Millisecond

// The application records the helper it has just started while that helper is reading the state, and Windows refuses to
// replace a file that is open. A save that gave up at once would have the application stop the helper, and the update
// with it.
func TestStateSaveWaitsForAReader(t *testing.T) {
	c := check.New(t)
	path := filepath.Join(t.TempDir(), stateName)
	s := &state{Status: statusStaged}
	c.NoError(s.save(path))
	// Opened as loadState opens it.
	f, err := os.Open(path)
	c.NoError(err)
	closed := make(chan error, 1)
	go func() {
		time.Sleep(stateHold)
		closed <- f.Close()
	}()

	s.HelperPID = 4343
	c.NoError(s.save(path))

	c.NoError(<-closed)
	got, err := loadState(path)
	c.NoError(err)
	c.Equal(s, got)
}

// The helper may read the state at the moment the application that started it is replacing the file, and Windows
// refuses to open a file for as long as that takes. A helper that gave up at once would exit without applying the
// update.
func TestLoadStateWaitsForAWriter(t *testing.T) {
	c := check.New(t)
	path := filepath.Join(t.TempDir(), stateName)
	want := &state{Status: statusStaged, HelperPID: 4343}
	c.NoError(want.save(path))
	name, err := windows.UTF16PtrFromString(path)
	c.NoError(err)
	// Shared with nobody, as the file is while it is being replaced.
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL, 0)
	c.NoError(err)
	closed := make(chan error, 1)
	go func() {
		time.Sleep(stateHold)
		closed <- windows.CloseHandle(handle)
	}()

	got, err := loadState(path)
	c.NoError(err)
	c.Equal(want, got)

	c.NoError(<-closed)
}
