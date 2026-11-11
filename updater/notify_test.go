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
	"github.com/richardwilkes/unison"
)

// testDownloadPage is where the tests say a release can be downloaded by hand.
const testDownloadPage = "https://example.com/download"

// TestNotifyOffersOnlyWhatIsPossible verifies the buttons the dialog has: Install & Restart only when this installation
// can replace itself, and Download Page only when the application has one. The dialog records the release as seen, and
// the Download Page button opens that page.
func TestNotifyOffersOnlyWhatIsPossible(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t)
	for _, tc := range []struct {
		planErr      error
		name         string
		downloadPage string
		wantInstall  bool
	}{
		{name: "installable, with a download page", downloadPage: testDownloadPage, wantInstall: true},
		{name: "installable, without a download page", wantInstall: true},
		{
			name:         "not installable",
			downloadPage: testDownloadPage,
			planErr:      &unavailable{Blocker: blockerReadOnly},
		},
	} {
		var seen []string
		u := New(&Config{
			AppName:            testApp,
			CmdName:            testCmd,
			Version:            testVersion,
			DownloadPageURL:    tc.downloadPage,
			SetLastSeenVersion: func(version string) { seen = append(seen, version) },
		})
		u.planFn = func(string, []Asset) (*plan, error) {
			if tc.planErr != nil {
				return nil, tc.planErr
			}
			return &plan{ToVersion: testNextVersion}, nil
		}
		u.setReleases([]Release{{Version: testNextVersion, Notes: "- Something new"}})
		c.True(screen.Post(u.Notify))
		screen.Sync()
		dialog := modalDialog(screen)
		if dialog == nil {
			t.Fatalf("%s: the dialog did not open", tc.name)
		}
		var cancel, download, install *unison.Button
		c.True(screen.Do(func() {
			cancel = dialog.Button(unison.ModalResponseCancel)
			download = dialog.Button(downloadPageResponse)
			install = dialog.Button(unison.ModalResponseOK)
		}))
		c.NotNil(cancel, "%s", tc.name)
		c.Equal(tc.wantInstall, install != nil, "%s", tc.name)
		c.Equal(tc.downloadPage != "", download != nil, "%s", tc.name)
		c.Equal([]string{testNextVersion}, seen, "%s", tc.name)

		screen.OpenedURLs() // Forgets anything opened before the click
		if download != nil {
			screen.Click(screen.PanelCenter(download))
			c.Equal([]string{testDownloadPage}, screen.OpenedURLs(), "%s", tc.name)
		} else {
			screen.Click(screen.PanelCenter(cancel))
		}
		c.Nil(modalDialog(screen), "%s: the dialog must close", tc.name)
	}
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// With no update known there is nothing to show, so no dialog opens.
func TestNotifyDoesNothingWithoutAnUpdate(t *testing.T) {
	c := check.New(t)
	screen := startHeadless(t)
	u := newTestUpdater(t)
	// Posted rather than run with Do, which would never return were a dialog to open.
	c.True(screen.Post(u.Notify))
	screen.Sync()
	c.Nil(modalDialog(screen))
	c.Equal(0, len(screen.Errors()), "nothing should have panicked: %v", screen.Errors())
}

// Each reason an installation cannot update itself has its own explanation, and the one for Homebrew names the cask the
// application says it is installed by.
func TestBlockerMessages(t *testing.T) {
	c := check.New(t)
	u := New(&Config{AppName: testApp, CmdName: testCmd, Version: testVersion, HomebrewCask: "example-cask"})
	fallback := u.blockerMessage("not-yet-invented")
	seen := map[string]blocker{}
	for _, b := range []blocker{
		blockerDevBuild, blockerRenamedExecutable, blockerNotABundle, blockerTranslocated, blockerPackageManaged,
		blockerHomebrew, blockerReadOnly, blockerNoAsset, blockerNoDigest,
	} {
		msg := u.blockerMessage(b)
		c.NotEqual(fallback, msg, "%s must have its own message", b)
		other, dup := seen[msg]
		c.False(dup, "%s and %s share a message", b, other)
		seen[msg] = b
	}
	c.Contains(u.blockerMessage(blockerHomebrew), "brew upgrade --cask example-cask")
}
