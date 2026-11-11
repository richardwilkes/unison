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
	"net/http"
	"runtime"
	"strings"

	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison"
)

// Config describes the application an Updater keeps up to date. The names in it have to match the ones upack gave the
// published releases.
type Config struct {
	// Icon is shown in the dialog that offers an update.
	Icon unison.Drawable
	// IconInk is the ink Icon is drawn with.
	IconInk unison.Ink
	// SkipRelease, when set, is called with each release at or above Version; returning true leaves that release out.
	SkipRelease func(version, notes string) bool
	// FormatVersion, when set, turns a version into the form shown to the user. Asset names and version comparisons
	// always use the raw version.
	FormatVersion func(version string) string
	// LastSeenVersion, when set, returns the newest version the user has already been told about.
	LastSeenVersion func() string
	// SetLastSeenVersion, when set, records the newest version the user has been told about.
	SetLastSeenVersion func(version string)
	// ChecksDisabled, when set, reports whether the user has turned off automatic checks for updates, which selects the
	// status shown before any check has run.
	ChecksDisabled func() bool
	// StateChanged, when set, is called on the UI thread after every change to the update status.
	StateChanged func()
	// ErrorHandler, when set, reports an error to the user. nil uses unison.ErrorDialogWithError.
	ErrorHandler func(title string, err error)
	// HTTPClient makes every request. nil uses a new http.Client.
	HTTPClient *http.Client
	// AppName is the application's name as shown to the user. "" uses xos.AppName.
	AppName string
	// CmdName is the executable's name, without the ".exe" Windows adds, and the start of every release asset's name.
	// "" uses xos.AppCmdName, minus any ".exe".
	CmdName string
	// BundleName is the name of the macOS application bundle. "" uses CmdName + ".app".
	BundleName string
	// Version is the running application's version. "" uses xos.AppVersion.
	Version string
	// HomebrewCask is the name of the Homebrew cask that installs the application on macOS, whose installation is left
	// for Homebrew to update. "" skips that check.
	HomebrewCask string
	// GitHubOwner is the account that owns the repository the releases are published in.
	GitHubOwner string
	// GitHubRepo is the repository the releases are published in.
	GitHubRepo string
	// GitHubToken, when set, is presented to the GitHub API as a bearer token.
	GitHubToken string
	// DownloadPageURL is where the user can download a release by hand. "" offers no download page.
	DownloadPageURL string
	// VersionArgs make the application print its version and exit without opening a window. A staged release is run
	// with them, to prove it starts on this machine, before it replaces the installation. nil uses {"-v"}.
	VersionArgs []string
	// HandoffPort is the port the application's single-instance handoff listens on. The helper waits for it to be
	// released before replacing the installation. 0 waits for the application's process to exit instead.
	HandoffPort int
}

// Updater finds newer releases of the running application on GitHub, tells the user about them, and replaces the
// application with one. Unless noted otherwise, its methods must be called on the UI thread.
type Updater struct {
	// The functions below are what tests substitute. New sets each to the real thing.
	fetchFn        func(ctx context.Context) ([]Release, error)
	invokeFn       func(f func())
	planFn         func(toVersion string, assets []Asset) (*plan, error)
	stageFn        func(ctx context.Context, p *plan, progress func(fraction float64), phaseFn func(phase)) (*staged, error)
	spawnFn        func(s *state, statePath string) error
	relaunchFn     func(t *target, logPath string) error
	quitFn         func()
	atExitFn       func(f func()) int
	cancelAtExitFn func(id int)
	warnFn         func(primary, detail string)
	// stateFile, when set, replaces the state file's usual location.
	stateFile string
	// lastSeenVersion stands in for the application's record of it when Config has no hooks for one.
	lastSeenVersion string
	cfg             Config
	pending         pendingUpdate
	check           checkState
}

// New returns an Updater for the application cfg describes. cfg is copied, with the defaults its fields describe filled
// in; a nil cfg takes every default. The defaults are read from xos when New is called, so set its values first.
func New(cfg *Config) *Updater {
	u := &Updater{
		invokeFn:       unison.InvokeTask,
		relaunchFn:     relaunch,
		quitFn:         unison.AttemptQuit,
		atExitFn:       xos.RunAtExit,
		cancelAtExitFn: xos.CancelRunAtExit,
		warnFn:         unison.WarningDialogWithMessage,
	}
	u.fetchFn = u.fetchReleases
	u.planFn = u.currentPreflight
	u.stageFn = u.stage
	u.spawnFn = u.spawnFinisher
	if cfg != nil {
		u.cfg = *cfg
	}
	if u.cfg.AppName == "" {
		u.cfg.AppName = xos.AppName
	}
	if u.cfg.CmdName == "" {
		u.cfg.CmdName = xos.AppCmdName
		if runtime.GOOS == xos.WindowsOS && strings.HasSuffix(strings.ToLower(u.cfg.CmdName), ".exe") {
			u.cfg.CmdName = u.cfg.CmdName[:len(u.cfg.CmdName)-len(".exe")]
		}
	}
	if u.cfg.BundleName == "" {
		u.cfg.BundleName = u.cfg.CmdName + ".app"
	}
	if u.cfg.Version == "" {
		u.cfg.Version = xos.AppVersion
	}
	if u.cfg.VersionArgs == nil {
		u.cfg.VersionArgs = []string{"-v"}
	}
	if u.cfg.HTTPClient == nil {
		u.cfg.HTTPClient = &http.Client{}
	}
	return u
}
