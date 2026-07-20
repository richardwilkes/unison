// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xfilepath"
)

// rankOwner is the LSHandlerRank value that marks a file type as owned (and therefore exported) by the application.
const rankOwner = "Owner"

// FileData holds information about a file type.
type FileData struct {
	Name       string   `yaml:"name"`
	Icon       string   `yaml:"icon"`
	IconName   string   `yaml:"-"`
	Role       string   `yaml:"role"`
	Rank       string   `yaml:"rank"`
	UTI        string   `yaml:"uti"`
	ConformsTo []string `yaml:"conforms_to"`
	Extensions []string `yaml:"extensions"`
	MimeTypes  []string `yaml:"mime_types"`
}

// MacCodeSigning holds information about code signing for macOS.
//
// Notarization uses the keychain profile named by Credentials. That profile may either already exist (created
// beforehand via "xcrun notarytool store-credentials") or be created automatically at packaging time from an App
// Store Connect API key by supplying APIKey, APIKeyID, and APIKeyIssuer. Each of the three API key values falls back
// to an environment variable (NOTARY_API_KEY, NOTARY_API_KEY_ID, NOTARY_API_KEY_ISSUER, respectively) when left empty,
// so secrets need not be committed to the configuration file.
type MacCodeSigning struct {
	Identity     string   `yaml:"identity"`
	Credentials  string   `yaml:"credentials"`
	APIKey       string   `yaml:"api_key"`        // Path to the App Store Connect API key (.p8) file
	APIKeyID     string   `yaml:"api_key_id"`     // The App Store Connect API key ID
	APIKeyIssuer string   `yaml:"api_key_issuer"` // The App Store Connect API key issuer ID
	Options      []string `yaml:"options"`
}

// MacOnlyOpts holds options for macOS.
type MacOnlyOpts struct {
	FinderAppName             string         `yaml:"finder_app_name"`
	AppID                     string         `yaml:"app_id"`
	MinimumSystemVersionAMD64 string         `yaml:"minimum_system_version_amd64"`
	MinimumSystemVersionARM64 string         `yaml:"minimum_system_version_arm64"`
	CategoryUTI               string         `yaml:"category_uti"`
	CodeSigning               MacCodeSigning `yaml:"code_signing"`
}

// Config holds the configuration for the packager.
type Config struct {
	version         string
	FullName        string      `yaml:"full_name"`
	ExecutableName  string      `yaml:"executable_name"`
	AppIcon         string      `yaml:"app_icon"`
	Description     string      `yaml:"description"`
	CopyrightHolder string      `yaml:"copyright_holder"`
	CopyrightYears  string      `yaml:"copyright_years"`
	Trademarks      string      `yaml:"trademarks"`
	FileInfo        []*FileData `yaml:"file_info"`
	Mac             MacOnlyOpts `yaml:"mac"`
}

func (c *Config) prepare(version string) {
	c.version = version
	c.ExecutableName = xfilepath.BaseName(c.ExecutableName)
}

// validate rejects configurations that would otherwise fail (or panic) deep inside platform-specific packaging code.
func (c *Config) validate() error {
	for _, fi := range c.FileInfo {
		if fi.Rank == rankOwner && len(fi.Extensions) == 0 {
			return errs.Newf("file_info entry %q has rank Owner but no extensions", fi.Name)
		}
	}
	return nil
}

func (c *Config) finderAppName() string { //nolint:unused // This is used only on some platforms
	if c.Mac.FinderAppName != "" {
		return c.Mac.FinderAppName
	}
	return c.ExecutableName
}

func (c *Config) shortAppVersion() string { //nolint:unused // This is used only on some platforms
	shortVersion := strings.TrimSuffix(c.version, ".0")
	if strings.IndexByte(shortVersion, '.') == -1 {
		return c.version
	}
	return shortVersion
}

func (c *Config) macMinSysVersion() string { //nolint:unused // This is used only on some platforms
	if runtime.GOARCH == "arm64" {
		return c.Mac.MinimumSystemVersionARM64
	}
	return c.Mac.MinimumSystemVersionAMD64
}

func (c *Config) copyright() string { //nolint:unused // This is used only on some platforms
	return fmt.Sprintf("©%s by %s", c.CopyrightYears, c.CopyrightHolder)
}
