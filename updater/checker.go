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
	"sync"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/i18n"
)

// checkTimeout bounds a request for the list of releases.
const checkTimeout = 5 * time.Minute

// checkState holds what is known about available updates. A visible check (asked for by the user or run at launch)
// blanks what is known and sets updating, so that whatever shows the status shows a check under way. A quiet check (run
// by a repeating schedule) leaves the previous answer on display until it has a better one, so a failed or unchanged
// background check never takes away an update the user was already told about.
type checkState struct {
	result   string
	releases []Release
	lock     sync.RWMutex
	seq      int  // bumped by every visible check, so a quiet result that arrives late can be recognized as stale
	updating bool // a visible check is in flight
	quiet    bool // a quiet check is in flight
}

// Check starts a visible check for updates: Status reports a check under way, and finding a release the user has not
// yet been shown opens the dialog Notify shows. With alwaysNotify, that dialog opens for any release found, which is
// what a user who asks for a check expects. A development build has no release to compare against, so it reports that
// instead of checking.
func (u *Updater) Check(alwaysNotify bool) {
	if IsDevVersion(u.cfg.Version) {
		u.setResult(u.devVersionText())
		u.stateChanged()
		return
	}
	if !u.beginVisible() {
		return
	}
	u.stateChanged()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
		defer cancel()
		releases, err := u.fetchFn(ctx)
		if err != nil {
			u.setResult(u.unableToAccessText())
			errs.Log(err)
			u.stateChanged()
			return
		}
		if len(releases) == 0 || releases[0].Version == u.cfg.Version {
			u.setResult(u.noUpdatesText())
			u.stateChanged()
			return
		}
		u.setReleases(releases)
		u.stateChanged()
		u.invokeFn(func() {
			if alwaysNotify || shouldNotify(releases[0].Version, u.lastSeen()) {
				u.Notify()
			}
		})
	}()
}

// CheckQuietly checks for updates on behalf of a repeating schedule. It never opens a dialog, and a failed check leaves
// what is known in place. It does nothing while another check is running.
func (u *Updater) CheckQuietly() {
	if IsDevVersion(u.cfg.Version) {
		// Development versions never look for updates; saying so, as the visible check does, beats a status claiming
		// that a check is still to come.
		u.setResult(u.devVersionText())
		u.stateChanged()
		return
	}
	seq, ok := u.beginQuiet()
	if !ok {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
		defer cancel()
		releases, err := u.fetchFn(ctx)
		if err != nil {
			errs.Log(err)
		}
		u.finishQuiet(seq, releases, err)
		u.stateChanged()
	}()
}

// Status returns what is currently known: a title that says it in a few words, the releases newer than the running
// version, newest first, or nil if no update is known, and whether a visible check is under way. The title is never
// blank; until a check records something, it says why: a quiet check is under way, the checks are off, or none has run
// yet. It may be called from any goroutine.
func (u *Updater) Status() (title string, releases []Release, updating bool) {
	u.check.lock.RLock()
	defer u.check.lock.RUnlock()
	if u.check.result == "" {
		return u.uncheckedTitleLocked(), u.check.releases, u.check.updating
	}
	return u.check.result, u.check.releases, u.check.updating
}

// Checking returns true while a check of either kind is in flight, unlike Status, which reports only a visible check as
// updating. Use it to avoid offering a check while one is running. It may be called from any goroutine.
func (u *Updater) Checking() bool {
	u.check.lock.RLock()
	defer u.check.lock.RUnlock()
	return u.check.updating || u.check.quiet
}

// shouldNotify reports whether a release warrants interrupting the user with the dialog. The last seen version is
// recorded by Notify just before it goes modal, so the dialog opens the first time a release is found, while later
// checks that turn up a release the user has already declined leave it to Status to say so.
func shouldNotify(version, lastSeen string) bool {
	return version != lastSeen
}

// beginVisible marks the start of a visible check, returning false if one is already running. Bumping seq makes any
// quiet check still in flight discard its result.
func (u *Updater) beginVisible() bool {
	u.check.lock.Lock()
	defer u.check.lock.Unlock()
	if u.check.updating {
		return false
	}
	u.check.result = u.checkingText()
	u.check.releases = nil
	u.check.updating = true
	u.check.seq++
	return true
}

// uncheckedTitleLocked returns the title for the state in which no check has recorded anything. The lock must already
// be held.
func (u *Updater) uncheckedTitleLocked() string {
	switch {
	case u.check.quiet:
		return u.checkingText()
	case u.cfg.ChecksDisabled != nil && u.cfg.ChecksDisabled():
		return i18n.Text("Automatic %s update checks are off", u.cfg.AppName)
	default:
		return i18n.Text("No %s update check has run yet", u.cfg.AppName)
	}
}

// setResult records a check outcome that has no releases to offer and marks any visible check as finished.
func (u *Updater) setResult(str string) {
	u.check.lock.Lock()
	u.check.result = str
	u.check.updating = false
	u.check.lock.Unlock()
}

// setReleases records the releases a visible check found.
func (u *Updater) setReleases(releases []Release) {
	u.check.lock.Lock()
	u.setReleasesLocked(releases)
	u.check.lock.Unlock()
}

// setReleasesLocked records the releases a check found. The lock must already be held.
func (u *Updater) setReleasesLocked(releases []Release) {
	u.check.result = i18n.Text("%s %s is available!", u.cfg.AppName, u.formatVersion(releases[0].Version))
	u.check.releases = releases
	u.check.updating = false
}

// beginQuiet claims the right to run a quiet check, returning the sequence number to hand back to finishQuiet. It
// refuses while a visible check is running, since that check's answer is the one the user is waiting on, and while
// another quiet check is already in flight.
func (u *Updater) beginQuiet() (seq int, ok bool) {
	u.check.lock.Lock()
	defer u.check.lock.Unlock()
	if u.check.updating || u.check.quiet {
		return 0, false
	}
	u.check.quiet = true
	return u.check.seq, true
}

// finishQuiet records the outcome of a quiet check. A result whose sequence number no longer matches is discarded,
// since a visible check has started since. A failed check (the caller logs the error) leaves what is known untouched,
// so a network hiccup can't erase an update the user was already told about; when nothing was known, the failure is
// recorded so that the status says the site couldn't be reached rather than that no check has run.
func (u *Updater) finishQuiet(seq int, releases []Release, err error) {
	u.check.lock.Lock()
	defer u.check.lock.Unlock()
	u.check.quiet = false
	if seq != u.check.seq {
		return
	}
	if err != nil {
		if u.check.result == "" {
			u.check.result = u.unableToAccessText()
		}
		return
	}
	if len(releases) == 0 || releases[0].Version == u.cfg.Version {
		u.check.result = u.noUpdatesText()
		u.check.releases = nil
		return
	}
	u.setReleasesLocked(releases)
}

// fetchReleases retrieves the releases newer than the running version. With nothing newer, it may hold just the running
// version's own release.
func (u *Updater) fetchReleases(ctx context.Context) ([]Release, error) {
	return FetchReleases(ctx, u.cfg.HTTPClient, u.cfg.GitHubOwner, u.cfg.GitHubRepo, u.cfg.GitHubToken, u.cfg.Version,
		u.cfg.SkipRelease)
}

// stateChanged tells the application, on the UI thread, that what Status reports has changed.
func (u *Updater) stateChanged() {
	if u.cfg.StateChanged != nil {
		u.invokeFn(u.cfg.StateChanged)
	}
}

// formatVersion returns the version in the form shown to the user.
func (u *Updater) formatVersion(version string) string {
	if u.cfg.FormatVersion != nil {
		return u.cfg.FormatVersion(version)
	}
	return version
}

func (u *Updater) lastSeen() string {
	if u.cfg.LastSeenVersion != nil {
		return u.cfg.LastSeenVersion()
	}
	return u.lastSeenVersion
}

func (u *Updater) setLastSeen(version string) {
	u.lastSeenVersion = version
	if u.cfg.SetLastSeenVersion != nil {
		u.cfg.SetLastSeenVersion(version)
	}
}

func (u *Updater) noUpdatesText() string {
	return i18n.Text("No %s updates are available", u.cfg.AppName)
}

func (u *Updater) checkingText() string {
	return i18n.Text("Checking for %s updates…", u.cfg.AppName)
}

func (u *Updater) unableToAccessText() string {
	return i18n.Text("Unable to access the %s update site", u.cfg.AppName)
}

func (u *Updater) devVersionText() string {
	return i18n.Text("Development versions don't look for %s updates", u.cfg.AppName)
}
