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
	"path/filepath"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// The helper is recognized in every form the flag package accepts, so an application that also parses its own command
// line with that package sees the same thing this does. A flag with no path is not a request to finish anything, since
// there is no state to finish it from.
func TestFinishArgRecognizesTheHelperFlag(t *testing.T) {
	c := check.New(t)
	const path = "/data/update-state.json"
	for _, one := range []struct {
		name   string
		want   string
		args   []string
		wantOK bool
	}{
		{name: "one dash", args: []string{testCmd, "-finish-update", path}, want: path, wantOK: true},
		{name: "two dashes", args: []string{testCmd, "--finish-update", path}, want: path, wantOK: true},
		{name: "one dash and =", args: []string{testCmd, "-finish-update=" + path}, want: path, wantOK: true},
		{name: "two dashes and =", args: []string{testCmd, "--finish-update=" + path}, want: path, wantOK: true},
		{
			name:   "among other arguments",
			args:   []string{testCmd, "-x", "--finish-update", path, "file"},
			want:   path,
			wantOK: true,
		},
		{name: "absent", args: []string{testCmd, "-x", path}},
		{name: "no arguments"},
		{name: "no path", args: []string{testCmd, "--finish-update"}},
		{name: "an empty path", args: []string{testCmd, "--finish-update="}},
		{name: "three dashes", args: []string{testCmd, "---finish-update", path}},
		{name: "not a flag", args: []string{testCmd, "finish-update", path}},
	} {
		got, ok := finishArg(one.args)
		c.Equal(one.wantOK, ok, one.name)
		c.Equal(one.want, got, one.name)
	}
}

// The helper only ever applies an update the application staged. A missing state, or one that has already been applied
// or has failed, is refused without being touched, so a stray or repeated helper cannot rewrite what the newly
// installed application will report.
func TestFinishRefusesWithoutAStagedUpdate(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)

	c.HasError(u.Finish(filepath.Join(t.TempDir(), stateName)), "a missing state must be refused")

	for _, st := range []status{statusApplied, statusFailed} {
		path := filepath.Join(t.TempDir(), stateName)
		c.NoError((&state{Status: st, Reason: reasonRelaunchFailed}).save(path))
		c.HasError(u.Finish(path), "a %s state must be refused", st)
		got, err := loadState(path)
		c.NoError(err)
		c.Equal(&state{Schema: stateSchema, Status: st, Reason: reasonRelaunchFailed}, got,
			"refusing a %s state must leave it as it was", st)
	}
}
