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

	"github.com/richardwilkes/toolbox/v2/check"
)

// Everything the helper needs must survive being written and read back. The file is the only channel between the
// application that stages an update and the process that applies it, so a field lost here is one the helper silently
// does without. Saving over an existing state must replace it atomically, leaving no temporary file behind: readers are
// separate processes, and a truncated file is indistinguishable from a stale one.
func TestStateRoundTrip(t *testing.T) {
	c := check.New(t)
	dir := t.TempDir()
	path := filepath.Join(dir, stateName)
	want := &state{
		Status:      statusStaged,
		Reason:      reasonNone,
		FromVersion: testVersion,
		ToVersion:   testNextVersion,
		Target:      "/Applications/Example.app",
		Exec:        "/Applications/Example.app/Contents/MacOS/example",
		Payload:     "/Applications/.example-update-abc/Example.app",
		Backup:      "/Applications/.Example.app.old-123",
		WorkDir:     "/Applications/.example-update-abc",
		Helper:      "/Applications/.example-update-abc/helper",
		LogPath:     "/Applications/.example-update-abc/update.log",
		HandoffPort: 13322,
		ParentPID:   4242,
		Bundle:      true,
	}
	c.NoError(want.save(path))

	got, err := loadState(path)
	c.NoError(err)
	c.NotNil(got)
	c.Equal(stateSchema, got.Schema, "save must stamp the schema")
	want.Schema = stateSchema
	c.Equal(want, got)

	c.NoError((&state{Status: statusApplied, Target: "/two"}).save(path))
	got, err = loadState(path)
	c.NoError(err)
	c.Equal(statusApplied, got.Status)
	c.Equal("/two", got.Target)
	entries, err := os.ReadDir(dir)
	c.NoError(err)
	c.Equal(1, len(entries), "the atomic write must not leave a temporary file behind")
}

// A build that predates this package being generic stages updates that a newer build applies and reports on, so the
// newer build must read the file exactly as the older one wrote it: every field name, and every status and reason
// value.
func TestLoadStateReadsTheShippedLayout(t *testing.T) {
	c := check.New(t)
	path := filepath.Join(t.TempDir(), stateName)
	write(t, path, `{
		"schema": 1,
		"status": "failed",
		"reason": "swap-failed",
		"detail": "access denied",
		"from_version": "5.45.2",
		"to_version": "5.46.0",
		"target": "/Applications/GCS.app",
		"exec": "/Applications/GCS.app/Contents/MacOS/gcs",
		"payload": "/Applications/.gcs-update-abc/GCS.app",
		"backup": "/Applications/.GCS.app.old-123",
		"work_dir": "/Applications/.gcs-update-abc",
		"helper": "/Applications/.gcs-update-abc/helper/Contents/MacOS/gcs",
		"log_path": "/Applications/.gcs-update-abc/update.log",
		"handoff_port": 13322,
		"parent_pid": 4242,
		"bundle": true
	}`)
	got, err := loadState(path)
	c.NoError(err)
	c.Equal(&state{
		Schema:      1,
		Status:      statusFailed,
		Reason:      reasonSwapFailed,
		Detail:      "access denied",
		FromVersion: "5.45.2",
		ToVersion:   "5.46.0",
		Target:      "/Applications/GCS.app",
		Exec:        "/Applications/GCS.app/Contents/MacOS/gcs",
		Payload:     "/Applications/.gcs-update-abc/GCS.app",
		Backup:      "/Applications/.GCS.app.old-123",
		WorkDir:     "/Applications/.gcs-update-abc",
		Helper:      "/Applications/.gcs-update-abc/helper/Contents/MacOS/gcs",
		LogPath:     "/Applications/.gcs-update-abc/update.log",
		HandoffPort: 13322,
		ParentPID:   4242,
		Bundle:      true,
	}, got)

	for value, want := range map[string]string{
		"staged":                    string(statusStaged),
		"applied":                   string(statusApplied),
		"failed":                    string(statusFailed),
		"predecessor-still-running": string(reasonPredecessorRunning),
		"swap-failed":               string(reasonSwapFailed),
		"relaunch-failed":           string(reasonRelaunchFailed),
		"version-mismatch":          string(reasonVersionMismatch),
		"abandoned":                 string(reasonAbandoned),
	} {
		c.Equal(value, want, "a shipped value has changed")
	}
	c.Equal("", string(reasonNone))
}

// Having no update in progress is the ordinary case rather than an error; every launch takes this path.
func TestLoadStateWithNoFile(t *testing.T) {
	c := check.New(t)
	s, err := loadState(filepath.Join(t.TempDir(), "does-not-exist.json"))
	c.NoError(err)
	c.Nil(s)
}

// A corrupt file, or one written by a build using a layout this one does not know, must be refused rather than acted
// on: acting on a half-understood state could mean deleting the wrong directory.
func TestLoadStateRejectsUnusableFiles(t *testing.T) {
	c := check.New(t)
	for name, content := range map[string]string{
		"truncated":     `{"schema":1,"status":"sta`,
		"not json":      `this is not json at all`,
		"empty":         ``,
		"future schema": `{"schema":99,"status":"staged","target":"/Applications/Example.app"}`,
		"no schema":     `{"status":"staged","target":"/Applications/Example.app"}`,
	} {
		path := filepath.Join(t.TempDir(), stateName)
		c.NoError(os.WriteFile(path, []byte(content), 0o600), name)
		s, err := loadState(path)
		c.HasError(err, name)
		c.Nil(s, name)
	}
}

// Forward tolerance: a state written by a newer build carrying unknown fields must still be readable, or an update that
// installs a newer build could leave the newly installed application unable to report on its own installation.
func TestLoadStateIgnoresUnknownFields(t *testing.T) {
	c := check.New(t)
	path := filepath.Join(t.TempDir(), stateName)
	c.NoError(os.WriteFile(path, []byte(`{
		"schema": 1,
		"status": "applied",
		"target": "/Applications/Example.app",
		"to_version": "1.3.0",
		"something_added_later": {"nested": [1, 2, 3]},
		"another_new_field": "value"
	}`), 0o600))
	s, err := loadState(path)
	c.NoError(err)
	c.NotNil(s)
	c.Equal(statusApplied, s.Status)
	c.Equal(testNextVersion, s.ToVersion)
	c.Equal("/Applications/Example.app", s.Target)
}

// The helper must be able to reconstruct what it is replacing without re-deriving it from its own executable path,
// which would be wrong, since the helper runs from the staging directory rather than from the installation. The
// executable it relaunches must never be the bundle directory itself: a state written before the executable path was
// recorded has to yield the conventional location inside the bundle, named for the configured command.
func TestTargetInfoExecIsNeverTheBundle(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	bundle := hostPath("/Applications/" + testBundle)
	wantExec := hostPath("/Applications/" + testBundle + "/Contents/MacOS/" + testCmd)

	legacy := u.targetInfo(&state{Target: bundle, Bundle: true})
	c.Equal(target{Path: bundle, Parent: hostPath("/Applications"), Exec: wantExec, Kind: kindBundle}, legacy)

	recorded := u.targetInfo(&state{Target: bundle, Exec: wantExec, Bundle: true})
	c.Equal(target{Path: bundle, Parent: hostPath("/Applications"), Exec: wantExec, Kind: kindBundle}, recorded)

	exePath := hostPath("/home/someone/bin/" + testCmd)
	exe := u.targetInfo(&state{Target: exePath})
	c.Equal(target{Path: exePath, Parent: hostPath("/home/someone/bin"), Exec: exePath, Kind: kindExecutable}, exe)
}
