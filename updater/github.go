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
	"encoding/json/v2"
	"net/http"
	"slices"
	"strings"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xio"
	"github.com/richardwilkes/toolbox/v2/xstrings"
)

// Release holds information about a single release published on GitHub.
type Release struct {
	// Version is the release's tag without its leading "v".
	Version string
	// Notes is the release's description, in Markdown.
	Notes string
	// ZipBallURL is where GitHub serves an archive of the repository's source at this release.
	ZipBallURL string
	// Assets are the files attached to the release, or nil if it has none.
	Assets []Asset
}

// FetchReleases returns the releases published in the GitHub repository owner/repo whose versions are at least
// currentVersion, newest first; an empty currentVersion returns them all. Only tags of the form "v<version>" count as
// releases, and drafts and pre-releases never do. Versions are compared in natural order, except that one with a
// pre-release suffix, such as 1.2.0-rc1, ranks below the same version without it. The release for currentVersion itself
// is dropped when a newer one exists. skip, when not nil, is called for each candidate, and returning true leaves it
// out. The result is nil when no release qualifies, or when owner or repo is empty.
//
// client makes the request; nil uses a new http.Client. token, when not empty, is presented as a bearer token, which
// raises GitHub's rate limit and gives access to a private repository.
func FetchReleases(ctx context.Context, client *http.Client, owner, repo, token, currentVersion string, skip func(version, notes string) bool) ([]Release, error) {
	if owner == "" || repo == "" {
		return nil, nil
	}
	if client == nil {
		client = &http.Client{}
	}
	uri := "https://api.github.com/repos/" + owner + "/" + repo + "/releases"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, http.NoBody)
	if err != nil {
		return nil, errs.NewWithCause("unable to create request for "+uri, err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rsp, err := client.Do(req)
	if err != nil {
		return nil, errs.NewWithCause("unable to connect to "+uri, err)
	}
	defer xio.DiscardAndCloseIgnoringErrors(rsp.Body)
	if rsp.StatusCode < 200 || rsp.StatusCode > 299 {
		return nil, errs.New("unexpected response code from " + uri + " -> " + rsp.Status)
	}
	var releases []struct {
		TagName    string `json:"tag_name"`
		Body       string `json:"body"`
		ZipBallURL string `json:"zipball_url"`
		Assets     []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			APIURL string `json:"url"`
			Digest string `json:"digest"`
			Size   int64  `json:"size"`
		} `json:"assets"`
		Draft      bool `json:"draft"`
		Prerelease bool `json:"prerelease"`
	}
	if err = json.UnmarshalRead(rsp.Body, &releases); err != nil {
		return nil, errs.NewWithCause("unable to decode response from GitHub API "+uri, err)
	}
	var versions []Release
	for _, one := range releases {
		if one.Draft || one.Prerelease {
			continue
		}
		tag, ok := strings.CutPrefix(one.TagName, "v")
		if !ok {
			continue
		}
		version := strings.TrimSpace(tag)
		if version == "" || compareVersions(version, currentVersion) < 0 {
			continue
		}
		if skip != nil && skip(version, one.Body) {
			continue
		}
		var assets []Asset
		if len(one.Assets) != 0 {
			assets = make([]Asset, len(one.Assets))
			for i, asset := range one.Assets {
				// GitHub only began publishing digests in 2025, so older assets carry none. Anything other than a
				// SHA-256 is treated as none at all, since that is the only kind the download can check.
				var sha string
				if digest, found := strings.CutPrefix(asset.Digest, "sha256:"); found {
					sha = digest
				}
				assets[i] = Asset{
					Name:   asset.Name,
					URL:    asset.URL,
					apiURL: asset.APIURL,
					SHA256: sha,
					Size:   asset.Size,
				}
			}
		}
		versions = append(versions, Release{
			Version:    version,
			Notes:      one.Body,
			ZipBallURL: one.ZipBallURL,
			Assets:     assets,
		})
	}
	slices.SortFunc(versions, func(a, b Release) int { return compareVersions(b.Version, a.Version) })
	if len(versions) > 1 && versions[len(versions)-1].Version == currentVersion {
		versions = versions[:len(versions)-1]
	}
	return versions, nil
}

// compareVersions returns -1, 0 or 1 as version a is older than, the same as or newer than b. Natural order decides,
// except that a pre-release, whose version has a "-" suffix, is older than the same version without one.
func compareVersions(a, b string) int {
	aCore, aPre, aIsPre := strings.Cut(a, "-")
	bCore, bPre, bIsPre := strings.Cut(b, "-")
	if result := xstrings.NaturalCmp(aCore, bCore, true); result != 0 {
		return result
	}
	switch {
	case aIsPre == bIsPre:
		return xstrings.NaturalCmp(aPre, bPre, true)
	case aIsPre:
		return -1
	default:
		return 1
	}
}
