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
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/unison"
)

// checkWait is a generous bound on the wait for a check's goroutine, so that a broken check fails the test rather than
// hanging it.
const checkWait = 10 * time.Second

// pendingRelease is a release newer than the version the tests run as.
var pendingRelease = []Release{{Version: testNextVersion}}

// awaitSignal waits for a check's goroutine to report in, failing rather than hanging if it never does.
func awaitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(checkWait):
		t.Fatal("timed out waiting for the check")
	}
}

// TestQuietCheckFailureKeepsAKnownUpdate verifies what a quiet check records: a failure keeps a known update and
// reports the unreachable site only when nothing was known, finding nothing clears a release no longer on offer (such
// as one the user has since installed), and a release found is recorded as a visible check would, since whatever shows
// the status can't tell which check produced it. A later success replaces any of these.
func TestQuietCheckFailureKeepsAKnownUpdate(t *testing.T) {
	seeded := newTestUpdater(t)
	seeded.setReleases(pendingRelease)
	availableTitle, _, _ := seeded.Status()
	check.New(t).Contains(availableTitle, testNextVersion, "the title must name the release it is about")
	failure := errors.New("update site unreachable")
	for _, tc := range []struct {
		err       error
		name      string
		wantTitle string
		known     []Release
		found     []Release
		want      []Release
	}{
		{
			name:      "failure keeps a known update",
			known:     pendingRelease,
			err:       failure,
			wantTitle: availableTitle,
			want:      pendingRelease,
		},
		{
			name:      "failure with nothing known is reported",
			err:       failure,
			wantTitle: seeded.unableToAccessText(),
		},
		{
			name:      "finding nothing clears a stale release",
			known:     pendingRelease,
			wantTitle: seeded.noUpdatesText(),
		},
		{
			name:      "finding only the running version clears a stale release",
			known:     pendingRelease,
			found:     []Release{{Version: testVersion}},
			wantTitle: seeded.noUpdatesText(),
		},
		{
			name:      "finding an update records it",
			found:     pendingRelease,
			wantTitle: availableTitle,
			want:      pendingRelease,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			u := newTestUpdater(t)
			if tc.known != nil {
				u.setReleases(tc.known)
			}
			seq, ok := u.beginQuiet()
			c.True(ok)
			u.finishQuiet(seq, tc.found, tc.err)
			title, releases, updating := u.Status()
			c.Equal(tc.wantTitle, title)
			c.Equal(tc.want, releases)
			c.False(updating)
			c.False(u.check.quiet, "the quiet check must no longer be marked as in flight")

			seq, ok = u.beginQuiet()
			c.True(ok)
			u.finishQuiet(seq, nil, nil)
			title, _, _ = u.Status()
			c.Equal(u.noUpdatesText(), title, "a later success must replace the result")
		})
	}
}

// TestQuietResultIsDiscardedAfterAVisibleCheckStarts verifies that a late quiet result can't overwrite the "Checking…"
// state of a visible check the user started.
func TestQuietResultIsDiscardedAfterAVisibleCheckStarts(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	seq, ok := u.beginQuiet()
	c.True(ok)
	c.True(u.beginVisible(), "a visible check must be able to start while a quiet one is in flight")
	checkingTitle, _, _ := u.Status()
	u.finishQuiet(seq, pendingRelease, nil)
	title, releases, updating := u.Status()
	c.Equal(checkingTitle, title, "the stale quiet result must not replace the visible check's state")
	c.Nil(releases)
	c.True(updating, "the visible check must still be marked as running")
	c.False(u.check.quiet, "the quiet check must no longer be marked as in flight")
}

func TestBeginQuietRefusesWhileAnotherCheckRuns(t *testing.T) {
	c := check.New(t)
	visible := newTestUpdater(t)
	c.True(visible.beginVisible())
	_, ok := visible.beginQuiet()
	c.False(ok, "a quiet check must not start while a visible check is running")

	quiet := newTestUpdater(t)
	_, ok = quiet.beginQuiet()
	c.True(ok)
	_, ok = quiet.beginQuiet()
	c.False(ok, "a second quiet check must not start while the first is in flight")
}

// TestUncheckedUpdaterReportsAStatus verifies that the title before any result is never blank and follows the
// application's setting as it is now, not as it was when the updater was made.
func TestUncheckedUpdaterReportsAStatus(t *testing.T) {
	c := check.New(t)
	disabled := true
	u := newTestUpdater(t)
	u.cfg.ChecksDisabled = func() bool { return disabled }
	title, releases, updating := u.Status()
	c.NotEqual("", title, "an updater that has never checked must still report a status")
	c.Contains(title, "off", "with the checks off, the title must say so")
	c.Nil(releases)
	c.False(updating)

	disabled = false
	title, _, _ = u.Status()
	c.NotEqual("", title)
	c.True(!strings.Contains(title, "off"), "with the checks on, the title must not claim they are off: %q", title)

	seq, ok := u.beginQuiet()
	c.True(ok)
	title, _, _ = u.Status()
	c.Equal(u.checkingText(), title, "while a quiet check runs with nothing known, the title must say so")
	u.finishQuiet(seq, nil, nil)
	title, _, _ = u.Status()
	c.Equal(u.noUpdatesText(), title)
}

// TestCheckingCoversBothKindsOfCheck verifies that Checking reports both kinds of check while Status reports only a
// visible one as updating: an application's check command reads Checking, while whatever shows the status reads Status
// and must keep showing a known update during a quiet check.
func TestCheckingCoversBothKindsOfCheck(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	u.setReleases(pendingRelease)
	c.False(u.Checking(), "nothing is in flight to begin with")

	seq, ok := u.beginQuiet()
	c.True(ok)
	c.True(u.Checking(), "a quiet check must count as a check in flight")
	_, releases, updating := u.Status()
	c.False(updating, "a quiet check must not report as a visible one")
	c.Equal(1, len(releases), "a quiet check must leave the known update on display")
	u.finishQuiet(seq, pendingRelease, nil)
	c.False(u.Checking(), "a finished quiet check must no longer count")

	c.True(u.beginVisible())
	c.True(u.Checking(), "a visible check must count as a check in flight")
	_, _, updating = u.Status()
	c.True(updating)
	u.setResult(u.noUpdatesText())
	c.False(u.Checking(), "a finished visible check must no longer count")
}

// TestVisibleCheckRecordsWhatItFinds verifies the status a visible check leaves behind, and that the application is
// told each time the status changes: once as the check starts and once as it ends.
func TestVisibleCheckRecordsWhatItFinds(t *testing.T) {
	u := newTestUpdater(t)
	u.setReleases(pendingRelease)
	availableTitle, _, _ := u.Status()
	for _, tc := range []struct {
		err       error
		name      string
		wantTitle string
		found     []Release
		want      []Release
	}{
		{name: "an unreachable site", err: errors.New("update site unreachable"), wantTitle: u.unableToAccessText()},
		{name: "no releases", wantTitle: u.noUpdatesText()},
		{name: "only the running version", found: []Release{{Version: testVersion}}, wantTitle: u.noUpdatesText()},
		{name: "a newer release", found: pendingRelease, wantTitle: availableTitle, want: pendingRelease},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := check.New(t)
			changes := make(chan struct{}, 4)
			one := newTestUpdater(t)
			one.cfg.StateChanged = func() { changes <- struct{}{} }
			one.invokeFn = func(f func()) { f() }
			one.fetchFn = func(context.Context) ([]Release, error) { return tc.found, tc.err }
			// A release already shown is not announced again, which keeps this test clear of the dialog.
			one.lastSeenVersion = testNextVersion
			one.Check(false)
			awaitSignal(t, changes)
			awaitSignal(t, changes)
			title, releases, updating := one.Status()
			c.Equal(tc.wantTitle, title)
			c.Equal(tc.want, releases)
			c.False(updating)
		})
	}
}

// A development build has no release to compare against, so either kind of check says so rather than asking the update
// site.
func TestCheckOnADevelopmentBuildDoesNotFetch(t *testing.T) {
	for _, one := range []struct {
		check func(u *Updater)
		name  string
	}{
		{name: "a visible check", check: func(u *Updater) { u.Check(true) }},
		{name: "a quiet check", check: (*Updater).CheckQuietly},
	} {
		t.Run(one.name, func(t *testing.T) {
			c := check.New(t)
			changes := 0
			u := New(&Config{AppName: testApp, CmdName: testCmd, Version: "0.0", StateChanged: func() { changes++ }})
			u.invokeFn = func(f func()) { f() }
			u.fetchFn = func(context.Context) ([]Release, error) {
				t.Error("a development build must not ask for releases")
				return nil, nil
			}
			one.check(u)
			title, releases, updating := u.Status()
			c.Equal(u.devVersionText(), title)
			c.Nil(releases)
			c.False(updating)
			c.False(u.Checking())
			c.Equal(1, changes, "the application must be told the status changed")
		})
	}
}

// TestCheckNotifiesOnlyOfAnUnseenRelease verifies when a visible check interrupts the user: the first time a release is
// found, and whenever the caller says to always notify, but not for a release the user has already been shown.
func TestCheckNotifiesOnlyOfAnUnseenRelease(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t)
	u := newTestUpdater(t)
	u.fetchFn = func(context.Context) ([]Release, error) { return pendingRelease, nil }
	u.planFn = func(string, []Asset) (*plan, error) { return nil, &unavailable{Blocker: blockerReadOnly} }
	// With no StateChanged callback, the one task a successful check posts is the one that decides whether to notify.
	posted := make(chan struct{}, 1)
	u.invokeFn = func(f func()) {
		unison.InvokeTask(f)
		posted <- struct{}{}
	}
	for _, one := range []struct {
		name         string
		alwaysNotify bool
		wantDialog   bool
	}{
		{name: "a release not yet seen", wantDialog: true},
		{name: "a release already seen"},
		{name: "a release already seen, when asked to always notify", alwaysNotify: true, wantDialog: true},
	} {
		c.True(screen.Do(func() { u.Check(one.alwaysNotify) }))
		awaitSignal(t, posted)
		screen.Sync()
		dialog := modalDialog(screen)
		c.Equal(one.wantDialog, dialog != nil, "%s", one.name)
		if dialog != nil {
			screen.KeyPress(unison.KeyEscape, 0)
			c.Nil(modalDialog(screen), "escape must dismiss the dialog")
		}
		var seen string
		c.True(screen.Do(func() { seen = u.lastSeen() }))
		c.Equal(testNextVersion, seen, "%s", one.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}
