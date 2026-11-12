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
	"runtime"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// TestAssetName pins the names down against what upack actually produces. These are a contract with the published
// releases rather than an internal detail: getting one wrong means the update silently reports that no build exists for
// the user's platform.
func TestAssetName(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	for _, one := range []struct {
		goos   string
		goarch string
		want   string
	}{
		{xos.MacOS, archAMD64, "example-1.3.0-macos-amd64.dmg"},
		{xos.MacOS, archARM64, "example-1.3.0-macos-arm64.dmg"},
		{xos.LinuxOS, archAMD64, "example-1.3.0-linux-amd64.tgz"},
		{xos.LinuxOS, archARM64, "example-1.3.0-linux-arm64.tgz"},
		{xos.WindowsOS, archAMD64, "example-1.3.0-windows-amd64.zip"},
		{xos.WindowsOS, archARM64, "example-1.3.0-windows-arm64.zip"},
	} {
		name, err := u.assetName(testNextVersion, one.goos, one.goarch)
		c.NoError(err, "%s/%s", one.goos, one.goarch)
		c.Equal(one.want, name, "%s/%s", one.goos, one.goarch)
	}
}

// TestAssetNameRejectsUnbuiltPlatforms verifies that a platform with no published build is an error rather than a
// plausible-looking name that would 404 partway through an update.
func TestAssetNameRejectsUnbuiltPlatforms(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	for _, one := range []struct {
		goos   string
		goarch string
	}{
		{"freebsd", archAMD64},
		{"js", "wasm"},
		{xos.LinuxOS, "386"},
		{xos.LinuxOS, "riscv64"},
		{xos.MacOS, "ppc64"},
		{"", ""},
	} {
		_, err := u.assetName(testNextVersion, one.goos, one.goarch)
		c.HasError(err, "%s/%s", one.goos, one.goarch)
	}
	_, err := u.assetName("", xos.LinuxOS, archAMD64)
	c.HasError(err, "an empty version must be rejected")
}

// TestPayloadName verifies what each archive is expected to hold: a bundle on macOS, a bare executable elsewhere.
func TestPayloadName(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	for goos, want := range map[string]string{
		xos.MacOS:     testBundle,
		xos.LinuxOS:   testCmd,
		xos.WindowsOS: testCmd + ".exe",
	} {
		name, err := u.payloadName(goos)
		c.NoError(err, goos)
		c.Equal(want, name, goos)
	}
	_, err := u.payloadName("plan9")
	c.HasError(err)
}

// TestSelectAsset verifies that the distribution for a platform is found by exact name among everything the release
// published, and that a release missing that build reports it as missing rather than resolving to some other file.
func TestSelectAsset(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	assets := []Asset{
		{Name: "language.i18n", URL: "https://example.com/i18n"},
		{Name: "example-1.3.0-linux-amd64.tgz", URL: "https://example.com/linux-amd64"},
		{Name: "example-1.3.0-linux-arm64.tgz", URL: "https://example.com/linux-arm64"},
		{Name: "example-1.3.0-macos-arm64.dmg", URL: "https://example.com/macos-arm64"},
	}

	asset, err := u.selectAsset(testNextVersion, xos.LinuxOS, archARM64, assets)
	c.NoError(err)
	c.Equal("https://example.com/linux-arm64", asset.URL)

	asset, err = u.selectAsset(testNextVersion, xos.MacOS, archARM64, assets)
	c.NoError(err)
	c.Equal("https://example.com/macos-arm64", asset.URL)

	// The Intel macOS build is absent from this release, which must be reported rather than falling back to the arm64
	// disk image that happens to be there.
	_, err = u.selectAsset(testNextVersion, xos.MacOS, archAMD64, assets)
	c.HasError(err)

	_, err = u.selectAsset(testNextVersion, xos.WindowsOS, archAMD64, assets)
	c.HasError(err)

	_, err = u.selectAsset(testNextVersion, xos.LinuxOS, archAMD64, nil)
	c.HasError(err, "a release with no assets must not match")

	// Names come from the GitHub API rather than a filesystem, so matching does not depend on case.
	asset, err = u.selectAsset(testNextVersion, xos.LinuxOS, archAMD64,
		[]Asset{{Name: "EXAMPLE-1.3.0-LINUX-AMD64.TGZ", URL: "u"}})
	c.NoError(err)
	c.Equal("u", asset.URL)
}

// TestUpdateArchPrefersAPublishedBuild verifies the Rosetta fallback ordering. On anything but a translated macOS
// process this is just runtime.GOARCH; the interesting case is that even when the native build would be preferred, an
// absent one must not be selected.
func TestUpdateArchPrefersAPublishedBuild(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	// No arm64 asset is published here, so whatever the translation state, the result is this build's own architecture.
	c.Equal(runtime.GOARCH, u.updateArch([]Asset{{Name: "example-1.3.0-macos-amd64.dmg"}}, testNextVersion))
}
