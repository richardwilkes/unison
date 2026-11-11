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
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

const (
	archAMD64 = "amd64"
	archARM64 = "arm64"
)

// Asset is a downloadable file attached to a release.
type Asset struct {
	// Name is the file's name.
	Name string
	// URL is where the file is downloaded from without a token.
	URL string
	// apiURL is the GitHub API's address for the file, from which a token can fetch a private repository's file.
	apiURL string
	// SHA256 is the hex-encoded SHA-256 digest GitHub published for the file, or "" if it published none.
	SHA256 string
	// Size is the file's length in bytes.
	Size int64
}

// platform describes the names and formats the packager produces for a GOOS.
type platform struct {
	label string
	ext   string
}

// platforms mirrors the naming in upack's packager, whose packager_darwin.go, packager_linux.go and packager_windows.go
// each build "<exe>-<version>-<label>-<arch><ext>".
var platforms = map[string]platform{
	xos.MacOS:     {label: "macos", ext: ".dmg"},
	xos.LinuxOS:   {label: "linux", ext: ".tgz"},
	xos.WindowsOS: {label: "windows", ext: ".zip"},
}

// assetName returns the name of the release asset built for the given platform. version must be the raw release
// version, e.g. "5.46.0", not one Config.FormatVersion has reshaped for display, which would not match.
func (u *Updater) assetName(version, goos, goarch string) (string, error) {
	p, ok := platforms[goos]
	if !ok {
		return "", errs.Newf("no %s distribution is built for %s", u.cfg.AppName, goos)
	}
	if goarch != archAMD64 && goarch != archARM64 {
		return "", errs.Newf("no %s distribution is built for %s/%s", u.cfg.AppName, goos, goarch)
	}
	if version == "" {
		return "", errs.New("a version is required")
	}
	return u.cfg.CmdName + "-" + version + "-" + p.label + "-" + goarch + p.ext, nil
}

// payloadName returns the name of the single item the platform's archive holds: the application bundle on macOS, the
// executable elsewhere.
func (u *Updater) payloadName(goos string) (string, error) {
	switch goos {
	case xos.MacOS:
		return u.cfg.BundleName, nil
	case xos.LinuxOS:
		return u.cfg.CmdName, nil
	case xos.WindowsOS:
		return u.cfg.CmdName + ".exe", nil
	default:
		return "", errs.Newf("no %s distribution is built for %s", u.cfg.AppName, goos)
	}
}

// selectAsset returns the asset holding the distribution for the given platform. Matching is by exact (though
// case-insensitive) name rather than by pattern, so a release missing the build for this platform is reported as
// missing instead of resolving to some other platform's file.
func (u *Updater) selectAsset(version, goos, goarch string, assets []Asset) (Asset, error) {
	name, err := u.assetName(version, goos, goarch)
	if err != nil {
		return Asset{}, err
	}
	for _, one := range assets {
		if strings.EqualFold(one.Name, name) {
			return one, nil
		}
	}
	return Asset{}, errs.Newf("release %s has no %s", version, name)
}

// updateArch returns the architecture to update to. It is normally the one this build was compiled for, but an amd64
// build running under Rosetta on Apple silicon returns arm64: leaving it as amd64 would keep such a user on the
// translated build forever.
func (u *Updater) updateArch(assets []Asset, version string) string {
	arch := runtime.GOARCH
	if runtime.GOOS != xos.MacOS || arch != archAMD64 || !runningTranslated() {
		return arch
	}
	// Prefer the native build only if it was published; otherwise stay on the one known to exist.
	if _, err := u.selectAsset(version, xos.MacOS, archARM64, assets); err != nil {
		return arch
	}
	return archARM64
}
