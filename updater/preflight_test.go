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
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// noEnv stands in for a process with none of the sandbox markers set.
func noEnv(string) (string, bool) { return "", false }

// allAssets is a release publishing every build, so a test exercises the platform checks rather than a missing asset.
func allAssets(u *Updater, version string) []Asset {
	var assets []Asset
	for _, goos := range []string{xos.MacOS, xos.LinuxOS, xos.WindowsOS} {
		for _, goarch := range []string{archAMD64, archARM64} {
			name, err := u.assetName(version, goos, goarch)
			if err != nil {
				continue
			}
			assets = append(assets, Asset{Name: name, URL: "https://example.com/" + name, SHA256: testDigest, Size: 100})
		}
	}
	return assets
}

// blockerOf extracts the blocker from a refusal, failing the test if the error is not one.
func blockerOf(t *testing.T, err error) blocker {
	t.Helper()
	var refusal *unavailable
	if !errors.As(err, &refusal) {
		t.Fatalf("expected an *unavailable, got %v", err)
	}
	return refusal.Blocker
}

// installedExecutable writes a bare executable named as the distribution names it into a new directory, and returns its
// path.
func installedExecutable(t *testing.T) string {
	t.Helper()
	exePath := filepath.Join(t.TempDir(), testCmd)
	write(t, exePath, "binary")
	return exePath
}

// The case that must work: an ordinary installation in a directory the user can write to, with the matching build
// published.
func TestPreflightAcceptsAWritableInstallation(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := installedExecutable(t)

	p, err := u.preflight(exePath, xos.LinuxOS, archAMD64, testVersion, testNextVersion, allAssets(u, testNextVersion),
		noEnv)
	c.NoError(err)
	c.NotNil(p)
	c.Equal(testNextVersion, p.ToVersion)
	c.Equal(testVersion, p.FromVersion)
	c.Equal("example-1.3.0-linux-amd64.tgz", p.Asset.Name)
	c.Equal(exePath, p.Target.Path)
}

// A build with no release behind it, whether unstamped ("0.0") or stamped with a VCS pseudo-version, must never try to
// update.
func TestPreflightRefusesADevBuild(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := installedExecutable(t)

	for _, version := range []string{
		"0.0",
		"",
		"5.48.1-0.20260826001723-41ebfa03e3e8",
		"5.48.1-0.20260826001723-41ebfa03e3e8+dirty",
	} {
		_, err := u.preflight(exePath, xos.LinuxOS, archAMD64, version, testNextVersion,
			allAssets(u, testNextVersion), noEnv)
		c.HasError(err, version)
		c.Equal(blockerDevBuild, blockerOf(t, err), version)
	}
}

// A copy the user renamed is left alone. On Linux the desktop integration only installs itself when the executable has
// the name the distribution gave it, so replacing a differently-named file would produce an installation that half
// works.
func TestPreflightRefusesARenamedExecutable(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := filepath.Join(t.TempDir(), testCmd+"-"+testVersion)
	write(t, exePath, "binary")

	_, err := u.preflight(exePath, xos.LinuxOS, archAMD64, testVersion, testNextVersion, allAssets(u, testNextVersion),
		noEnv)
	c.HasError(err)
	c.Equal(blockerRenamedExecutable, blockerOf(t, err))
}

// An installation belonging to a package manager is left for that package manager to update; replacing it would leave
// the package database describing a file that is no longer there.
func TestPreflightRefusesPackageManagedLocations(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	for _, dir := range []string{
		"/usr/bin", "/usr/local/bin", "/opt/example", "/snap/example/current/bin", "/nix/store/abc123-example/bin",
		"/var/lib/flatpak/app/com.example.App/current", "/app/bin",
	} {
		_, err := u.preflight(filepath.Join(dir, testCmd), xos.LinuxOS, archAMD64, testVersion, testNextVersion,
			allAssets(u, testNextVersion), noEnv)
		c.HasError(err, dir)
		c.Equal(blockerPackageManaged, blockerOf(t, err), dir)
	}
}

// The packaging systems that run the application from a managed image do not necessarily install under a recognizable
// path, so they are detected by their environment variables instead.
func TestPreflightRefusesSandboxedRuntimes(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := installedExecutable(t)

	for _, name := range sandboxEnvVars {
		env := func(lookup string) (string, bool) {
			if lookup == name {
				return "set", true
			}
			return "", false
		}
		_, err := u.preflight(exePath, xos.LinuxOS, archAMD64, testVersion, testNextVersion,
			allAssets(u, testNextVersion), env)
		c.HasError(err, name)
		c.Equal(blockerPackageManaged, blockerOf(t, err), name)
	}
}

// The check that keeps an update from being downloaded only to discover at the last moment that it cannot be installed.
func TestPreflightRefusesAnUnwritableInstallation(t *testing.T) {
	if runtime.GOOS == xos.WindowsOS {
		t.Skip("directory modes do not govern writability on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write to a read-only directory")
	}
	c := check.New(t)
	u := newTestUpdater(t)
	dir := filepath.Join(t.TempDir(), "readonly")
	c.NoError(os.Mkdir(dir, 0o755))
	exePath := filepath.Join(dir, testCmd)
	write(t, exePath, "binary")
	c.NoError(os.Chmod(dir, 0o500))                //nolint:gosec // Read-only is the point of the test
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) //nolint:errcheck,gosec // Best-effort cleanup of a test directory

	_, err := u.preflight(exePath, xos.LinuxOS, archAMD64, testVersion, testNextVersion, allAssets(u, testNextVersion),
		noEnv)
	c.HasError(err)
	c.Equal(blockerReadOnly, blockerOf(t, err))
}

// The App Translocation copy's parent directory can be writable, so the write probe alone would happily "update" a copy
// that vanishes when the application quits.
func TestPreflightRefusesATranslocatedApp(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := "/private/var/folders/xy/T/AppTranslocation/1234-ABCD/d/" + testBundle + "/Contents/MacOS/" + testCmd
	_, err := u.preflight(exePath, xos.MacOS, archARM64, testVersion, testNextVersion, allAssets(u, testNextVersion),
		noEnv)
	c.HasError(err)
	c.Equal(blockerTranslocated, blockerOf(t, err))
}

// macOS only updates a real installed bundle; replacing a bare executable would not produce something the Finder or
// Launch Services could run.
func TestPreflightRefusesABareExecutableOnMacOS(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := installedExecutable(t)

	_, err := u.preflight(exePath, xos.MacOS, archARM64, testVersion, testNextVersion, allAssets(u, testNextVersion),
		noEnv)
	c.HasError(err)
	c.Equal(blockerNotABundle, blockerOf(t, err))
}

// A release that did not publish a build for this platform is reported as such, rather than some other platform's file
// being downloaded.
func TestPreflightRefusesAMissingAsset(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := installedExecutable(t)

	_, err := u.preflight(exePath, xos.LinuxOS, archAMD64, testVersion, testNextVersion, []Asset{
		{Name: "example-1.3.0-macos-arm64.dmg", SHA256: testDigest},
	}, noEnv)
	c.HasError(err)
	c.Equal(blockerNoAsset, blockerOf(t, err))
}

// A release whose asset carries no checksum is refused outright rather than installed without verification.
func TestPreflightRefusesAnUnverifiableAsset(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := installedExecutable(t)

	_, err := u.preflight(exePath, xos.LinuxOS, archAMD64, testVersion, testNextVersion, []Asset{
		{Name: "example-1.3.0-linux-amd64.tgz", URL: "https://example.com/x", Size: 100},
	}, noEnv)
	c.HasError(err)
	c.Equal(blockerNoDigest, blockerOf(t, err))
}

// Every part of the Homebrew rule. The cask moves the bundle into /Applications exactly as a person would, so the
// installed copy is indistinguishable on its own: it is refused only when a Caskroom entry for the application's own
// cask exists under either Homebrew prefix. A second copy the user keeps elsewhere is theirs, and refusing to update
// that would be wrong. An application that names no cask is never refused.
func TestCheckHomebrewOnlyRefusesTheManagedCopy(t *testing.T) {
	c := check.New(t)
	saved := caskroomRoots
	caskroomRoots = []string{filepath.Join(t.TempDir(), "silicon"), filepath.Join(t.TempDir(), "intel")}
	t.Cleanup(func() { caskroomRoots = saved })
	managed := "/Applications/" + testBundle
	for _, one := range []struct {
		name    string
		cask    string
		entry   string
		path    string
		root    int
		refused bool
	}{
		{name: "no Caskroom entry", cask: testCmd, path: managed},
		{name: "entry on Apple silicon", cask: testCmd, entry: testCmd, path: managed, refused: true},
		{name: "entry on Intel", cask: testCmd, entry: testCmd, root: 1, path: managed, refused: true},
		{name: "another cask's entry", cask: testCmd, entry: "other", path: managed},
		{
			name:  "a copy in the user's folder",
			cask:  testCmd,
			entry: testCmd,
			path:  "/Users/someone/Applications/" + testBundle,
		},
		{name: "a copy on another volume", cask: testCmd, entry: testCmd, path: "/Volumes/Data/" + testBundle},
		{name: "no cask", entry: testCmd, path: managed},
	} {
		u := newTestUpdater(t)
		u.cfg.HomebrewCask = one.cask
		var entry string
		if one.entry != "" {
			entry = filepath.Join(caskroomRoots[one.root], one.entry)
			c.NoError(os.MkdirAll(entry, 0o755), one.name)
		}
		err := u.checkHomebrew(one.path)
		if one.refused {
			c.HasError(err, one.name)
			c.Equal(blockerHomebrew, blockerOf(t, err), one.name)
		} else {
			c.NoError(err, one.name)
		}
		if entry != "" {
			c.NoError(os.RemoveAll(entry), one.name)
		}
	}
}

// The probe runs every time an update is offered, so one that leaked a directory each time would slowly fill the user's
// installation directory.
func TestProbeWritableCleansUpAfterItself(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	dir := t.TempDir()
	for range 5 {
		c.NoError(u.probeWritable(dir))
	}
	entries, err := os.ReadDir(dir)
	c.NoError(err)
	c.Equal(0, len(entries), "the write probe left something behind")
}
