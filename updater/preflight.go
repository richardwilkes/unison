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
	"runtime"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// blocker identifies why this installation cannot update itself. These are checked before anything is downloaded, so
// the user learns an automatic update is not possible without first waiting for the download.
type blocker string

const (
	// blockerDevBuild means this is a development build, which has no release to update from.
	blockerDevBuild blocker = "dev-build"
	// blockerRenamedExecutable means the executable is not named what the distribution names it, so replacing it would
	// not produce a working installation; on Linux the desktop integration keys off the name.
	blockerRenamedExecutable blocker = "renamed-executable"
	// blockerNotABundle means macOS is running a bare executable rather than an installed application bundle.
	blockerNotABundle blocker = "not-a-bundle"
	// blockerTranslocated means macOS is running a read-only copy made by App Translocation, typically because the
	// application was launched straight from the disk image.
	blockerTranslocated blocker = "translocated"
	// blockerPackageManaged means the installation belongs to a package manager, which should be the one to update it.
	blockerPackageManaged blocker = "package-managed"
	// blockerHomebrew means the installation came from the Homebrew cask.
	blockerHomebrew blocker = "homebrew"
	// blockerReadOnly means the installation cannot be written to. This is the ordinary outcome for an application in a
	// location that needs administrator rights.
	blockerReadOnly blocker = "read-only"
	// blockerNoAsset means the release has no build for this operating system and processor.
	blockerNoAsset blocker = "no-asset"
	// blockerNoDigest means the release's build for this platform carries no checksum, so it cannot be verified.
	blockerNoDigest blocker = "no-digest"
)

// unavailable reports that an automatic update is not possible, and why. Detail carries the technical specifics for the
// log; the Blocker is what the caller turns into something to show the user.
type unavailable struct {
	Blocker blocker
	Detail  string
}

func (e *unavailable) Error() string {
	if e.Detail != "" {
		return string(e.Blocker) + ": " + e.Detail
	}
	return string(e.Blocker)
}

// plan is a checked, ready-to-run update.
type plan struct {
	FromVersion string
	ToVersion   string
	Target      target
	Asset       Asset
}

// packageManagedPrefixes are locations owned by a system package manager. Replacing a file under one of these would
// leave the package database describing something that is no longer there, and the next package operation would undo
// the update or refuse to proceed.
var packageManagedPrefixes = []string{
	"/usr/", "/opt/", "/snap/", "/nix/store/", "/var/lib/flatpak/", "/app/",
}

// sandboxEnvVars are set by the packaging systems that run an application from a managed, usually read-only, image.
var sandboxEnvVars = []string{"SNAP", "FLATPAK_ID", "APPIMAGE", "APPDIR", "container"}

// caskroomRoots are where Homebrew records cask installations, on Apple silicon and Intel respectively. A variable so
// that a test can point it somewhere it controls.
var caskroomRoots = []string{"/opt/homebrew/Caskroom", "/usr/local/Caskroom"}

// caskInstallPath returns where the Homebrew cask puts the application. A cask moves the bundle into /Applications
// exactly as a person dragging it there would, so the installed copy carries no marking of its own; the only evidence
// is the Caskroom entry alongside it.
func (u *Updater) caskInstallPath() string {
	return "/Applications/" + u.cfg.BundleName
}

// checkHomebrew refuses to replace an installation Homebrew is managing. Updating it behind Homebrew's back would leave
// `brew` still describing the old version, free to reinstall it over the new one on the next upgrade.
//
// Both the location and a Caskroom entry are required: the Caskroom existing only means the cask is installed
// somewhere, and a second copy the user keeps elsewhere is theirs to update.
func (u *Updater) checkHomebrew(targetPath string) error {
	if u.cfg.HomebrewCask == "" || targetPath != u.caskInstallPath() {
		return nil
	}
	for _, root := range caskroomRoots {
		caskroom := filepath.Join(root, u.cfg.HomebrewCask)
		if _, err := os.Stat(caskroom); err == nil {
			return &unavailable{Blocker: blockerHomebrew, Detail: caskroom + " manages " + targetPath}
		}
	}
	return nil
}

// preflight decides whether the running installation can replace itself with the given release, and returns the plan to
// do it. Every check here is local and immediate: nothing is downloaded until this succeeds. The inputs are parameters
// rather than globals so that each refusal can be reached from a test.
func (u *Updater) preflight(exePath, goos, goarch, fromVersion, toVersion string, assets []Asset, lookupEnv func(string) (string, bool)) (*plan, error) {
	if IsDevVersion(fromVersion) {
		return nil, &unavailable{Blocker: blockerDevBuild, Detail: "version is " + fromVersion}
	}
	t, err := resolveTarget(exePath, goos)
	if err != nil {
		return nil, err
	}
	if err = u.checkInstallation(&t, exePath, goos, lookupEnv); err != nil {
		return nil, err
	}
	asset, err := u.selectAsset(toVersion, goos, goarch, assets)
	if err != nil {
		return nil, &unavailable{Blocker: blockerNoAsset, Detail: err.Error()}
	}
	if asset.SHA256 == "" {
		// GitHub only began publishing asset checksums in 2025, so this cannot happen for any release new enough to be
		// worth installing. Refused rather than downgraded to an unverified install, since an update that silently
		// skips its integrity check is worse than no automatic update at all.
		return nil, &unavailable{Blocker: blockerNoDigest, Detail: asset.Name + " has no checksum"}
	}
	return &plan{Target: t, Asset: asset, FromVersion: fromVersion, ToVersion: toVersion}, nil
}

// checkInstallation verifies that this installation is one that may replace itself.
func (u *Updater) checkInstallation(t *target, exePath, goos string, lookupEnv func(string) (string, bool)) error {
	wantExe := u.cfg.CmdName
	if goos == xos.WindowsOS {
		wantExe += ".exe"
	}
	if !strings.EqualFold(filepath.Base(exePath), wantExe) {
		return &unavailable{Blocker: blockerRenamedExecutable, Detail: filepath.Base(exePath) + " is not " + wantExe}
	}
	if goos == xos.MacOS {
		// App Translocation runs the application from a randomized read-only copy, which happens whenever a quarantined
		// application is launched without first being moved out of the disk image it arrived in. That copy's containing
		// directory can be writable, so the write probe below would happily "update" something thrown away the moment
		// the application quits.
		if strings.Contains(exePath, "/AppTranslocation/") {
			return &unavailable{Blocker: blockerTranslocated, Detail: exePath}
		}
		if t.Kind != kindBundle {
			return &unavailable{Blocker: blockerNotABundle, Detail: exePath}
		}
		if err := u.checkHomebrew(t.Path); err != nil {
			return err
		}
	}
	if goos == xos.LinuxOS {
		for _, name := range sandboxEnvVars {
			if value, ok := lookupEnv(name); ok && value != "" {
				return &unavailable{Blocker: blockerPackageManaged, Detail: name + " is set"}
			}
		}
	}
	if goos != xos.WindowsOS {
		normalized := filepath.ToSlash(t.Path)
		for _, prefix := range packageManagedPrefixes {
			if strings.HasPrefix(normalized, prefix) {
				return &unavailable{Blocker: blockerPackageManaged, Detail: t.Path + " is under " + prefix}
			}
		}
	}
	if err := u.probeWritable(t.Parent); err != nil {
		return &unavailable{Blocker: blockerReadOnly, Detail: err.Error()}
	}
	return nil
}

// probeWritable verifies that the directory can be written to, by writing to it. Checking permission bits is not
// equivalent: it does not account for access control lists, for a read-only mount -- which is what running straight
// from a disk image looks like -- or for Windows, where the bits carry almost no information at all. The probe is named
// like a staging directory, so the startup sweep clears one that a crash leaves behind.
func (u *Updater) probeWritable(dir string) error {
	probe, err := os.MkdirTemp(dir, u.workDirPrefix()+"probe-")
	if err != nil {
		return errs.Wrap(err)
	}
	if err = os.Remove(probe); err != nil {
		return errs.Wrap(err)
	}
	return nil
}

// currentPreflight runs preflight for the running application against the given release.
func (u *Updater) currentPreflight(toVersion string, assets []Asset) (*plan, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	if exePath, err = filepath.EvalSymlinks(exePath); err != nil {
		return nil, errs.Wrap(err)
	}
	if exePath, err = filepath.Abs(exePath); err != nil {
		return nil, errs.Wrap(err)
	}
	return u.preflight(exePath, runtime.GOOS, u.updateArch(assets, toVersion), u.cfg.Version, toVersion, assets,
		os.LookupEnv)
}
