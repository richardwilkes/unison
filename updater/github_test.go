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
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
)

// redirectingTransport sends requests to a test server rather than to the GitHub API host baked into FetchReleases.
type redirectingTransport struct {
	host string
}

func (t *redirectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	redirected := req.Clone(req.Context())
	redirected.URL.Scheme = "http"
	redirected.URL.Host = t.host
	redirected.Host = ""
	return http.DefaultTransport.RoundTrip(redirected)
}

// releasesServer serves body as the GitHub API's list of releases and records the requests it receives.
type releasesServer struct {
	client        *http.Client
	path          atomic.Pointer[string]
	authorization atomic.Pointer[string]
	requests      atomic.Int64
}

func newReleasesServer(t *testing.T, body string) *releasesServer {
	t.Helper()
	s := &releasesServer{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		path := r.URL.Path
		s.path.Store(&path)
		authorization := r.Header.Get("Authorization")
		s.authorization.Store(&authorization)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	s.client = &http.Client{Transport: &redirectingTransport{host: srv.Listener.Addr().String()}}
	return s
}

// An updater downloads a packaged distribution directly, so an asset's name, URLs and size have to survive the decode,
// along with its digest stripped of the algorithm GitHub prefixes it with. Assets that predate GitHub's digests, or
// carry a digest of some other kind, have no SHA-256 rather than a bogus one, and a release with no assets has none
// rather than an empty list.
func TestFetchReleasesDecodesAssets(t *testing.T) {
	c := check.New(t)
	s := newReleasesServer(t, `[
		{
			"tag_name": "v2.0.0",
			"body": "Notes",
			"zipball_url": "https://example.com/zipball",
			"assets": [
				{
					"name": "example-2.0.0-macos-arm64.dmg",
					"browser_download_url": "https://example.com/example-2.0.0-macos-arm64.dmg",
					"url": "https://api.example.com/assets/1",
					"digest": "sha256:abc123",
					"size": 29234206
				},
				{"name": "old.tgz", "browser_download_url": "https://example.com/old.tgz", "size": 10},
				{"name": "other.tgz", "browser_download_url": "https://example.com/other.tgz", "digest": "md5:def456"}
			]
		},
		{"tag_name": "v1.5.0", "body": "", "zipball_url": "", "assets": []},
		{"tag_name": "v1.4.0", "body": "", "zipball_url": ""}
	]`)
	releases, err := FetchReleases(t.Context(), s.client, "someone", "repo", "", "", nil)
	c.NoError(err)
	c.Equal([]Release{
		{
			Version:    "2.0.0",
			Notes:      "Notes",
			ZipBallURL: "https://example.com/zipball",
			Assets: []Asset{
				{
					Name:   "example-2.0.0-macos-arm64.dmg",
					URL:    "https://example.com/example-2.0.0-macos-arm64.dmg",
					apiURL: "https://api.example.com/assets/1",
					SHA256: "abc123",
					Size:   29234206,
				},
				{Name: "old.tgz", URL: "https://example.com/old.tgz", Size: 10},
				{Name: "other.tgz", URL: "https://example.com/other.tgz"},
			},
		},
		{Version: "1.5.0"},
		{Version: "1.4.0"},
	}, releases)
}

// The list comes from the named repository, and a token travels as a bearer token while its absence sends no
// authorization at all, since GitHub answers a malformed header with a 401 that reads like a bad token. With no
// repository to ask, nothing is asked.
func TestFetchReleasesRequest(t *testing.T) {
	c := check.New(t)
	for _, token := range []string{"", "secret"} {
		s := newReleasesServer(t, `[]`)
		_, err := FetchReleases(t.Context(), s.client, "someone", "repo", token, testVersion, nil)
		c.NoError(err, "token %q", token)
		c.Equal("/repos/someone/repo/releases", *s.path.Load(), "token %q", token)
		want := ""
		if token != "" {
			want = "Bearer " + token
		}
		c.Equal(want, *s.authorization.Load(), "token %q", token)
	}

	s := newReleasesServer(t, `[]`)
	for _, ownerAndRepo := range [][2]string{{"", "repo"}, {"someone", ""}} {
		releases, err := FetchReleases(t.Context(), s.client, ownerAndRepo[0], ownerAndRepo[1], "", testVersion, nil)
		c.NoError(err)
		c.Nil(releases)
	}
	c.Equal(int64(0), s.requests.Load(), "no request may be made without an owner and a repository")
}

// Only "v" tags at or above the running version count, newest first, and minus any the caller skips. Drafts and
// pre-releases never count, and a version with a pre-release suffix ranks below the same version without one, so that
// someone running a release candidate is offered the final release. The running version's own release is kept only
// when nothing newer is, and nothing qualifying is nil.
func TestFetchReleasesSelectsNewerVersions(t *testing.T) {
	c := check.New(t)
	const body = `[
		{"tag_name": "v1.2.3", "body": "current"},
		{"tag_name": "v1.11.0", "body": "draft", "draft": true},
		{"tag_name": "v1.10.0-rc1", "body": "candidate", "prerelease": true},
		{"tag_name": "v1.10.0", "body": "ten"},
		{"tag_name": "v1.9.0", "body": "skip me"},
		{"tag_name": "v1.4.0-beta", "body": "candidate not marked as a pre-release"},
		{"tag_name": "v1.3.0", "body": "three"},
		{"tag_name": "v1.1.0", "body": "older"},
		{"tag_name": "1.20.0", "body": "no v"},
		{"tag_name": "release-1.30.0", "body": "not a version tag"},
		{"tag_name": "v", "body": "no version"},
		{"tag_name": "v  ", "body": "blank version"},
		{"tag_name": "v 1.4.0 ", "body": "padded"}
	]`
	const ten, candidate, nine, four, fourBeta = "1.10.0", "1.10.0-rc1", "1.9.0", "1.4.0", "1.4.0-beta"
	skipNotes := func(_, notes string) bool { return notes == "skip me" }
	for _, one := range []struct {
		skip    func(version, notes string) bool
		name    string
		current string
		want    []string
	}{
		{name: "newer releases", current: testVersion, want: []string{ten, nine, four, fourBeta, testNextVersion}},
		{
			name:    "skipped releases",
			current: testVersion,
			skip:    skipNotes,
			want:    []string{ten, four, fourBeta, testNextVersion},
		},
		{name: "only the current release", current: ten, want: []string{ten}},
		{name: "a release candidate", current: candidate, want: []string{ten}},
		{name: "nothing at or above", current: "2.0.0"},
		{
			name:    "no current version",
			current: "",
			want:    []string{ten, nine, four, fourBeta, testNextVersion, testVersion, "1.1.0"},
		},
	} {
		s := newReleasesServer(t, body)
		releases, err := FetchReleases(t.Context(), s.client, "someone", "repo", "", one.current, one.skip)
		c.NoError(err, one.name)
		var got []string
		for _, release := range releases {
			got = append(got, release.Version)
		}
		c.Equal(one.want, got, one.name)
		if one.want == nil {
			c.Nil(releases, one.name)
		}
	}
}
