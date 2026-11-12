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
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// Only an update that left the user on the old version is worth interrupting them for, including one that was staged
// and never applied, since the application quit for it and never came back. Success is silent, as is an update that
// was installed but then failed to restart: the user is already running the new version.
func TestOutcomeWarningOnlyForAnUpdateThatWasNotApplied(t *testing.T) {
	u := newTestUpdater(t)
	for _, tc := range []struct {
		out      *outcome
		name     string
		wantWarn bool
	}{
		{name: "nothing to report"},
		{name: "applied", out: &outcome{Applied: true}},
		{name: "applied but not restarted", out: &outcome{Applied: true, Reason: reasonRelaunchFailed}},
		{name: "staged and abandoned", out: &outcome{Reason: reasonAbandoned}, wantWarn: true},
		{name: "the application never quit", out: &outcome{Reason: reasonPredecessorRunning}, wantWarn: true},
		{name: "the swap failed", out: &outcome{Reason: reasonSwapFailed}, wantWarn: true},
		{name: "the wrong version is running", out: &outcome{Reason: reasonVersionMismatch}, wantWarn: true},
		{name: "a reason from a newer build", out: &outcome{Reason: "not-yet-invented"}, wantWarn: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			primary, detail, ok := u.outcomeWarning(tc.out)
			c.Equal(tc.wantWarn, ok)
			if tc.wantWarn {
				c.Contains(primary, testApp)
				c.NotEqual("", detail)
			}
		})
	}
}
