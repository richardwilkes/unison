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
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
)

// serveBytes stands up a server handing back the given payload, and returns an Asset describing it truthfully.
func serveBytes(t *testing.T, payload []byte) (*httptest.Server, Asset) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := w.Write(payload); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(payload)
	return srv, Asset{
		Name:   "payload.tgz",
		URL:    srv.URL,
		SHA256: hex.EncodeToString(sum[:]),
		Size:   int64(len(payload)),
	}
}

// The happy path, including that progress is reported and ends at the full size. With a token, the file is asked of
// the GitHub API, which is how a private repository's file is fetched with one.
func TestDownload(t *testing.T) {
	c := check.New(t)
	payload := []byte("a plausible archive of some length")
	_, public := serveBytes(t, payload)
	const token = "secret"
	private := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api" || r.Header.Get("Authorization") != "Bearer "+token ||
			r.Header.Get("Accept") != "application/octet-stream" {
			http.NotFound(w, r)
			return
		}
		if _, err := w.Write(payload); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(private.Close)
	privateAsset := public
	privateAsset.URL = private.URL + "/browser"
	privateAsset.apiURL = private.URL + "/api"
	for _, one := range []struct {
		name  string
		token string
		asset Asset
	}{
		{name: "a public repository", asset: public},
		{name: "a private repository", asset: privateAsset, token: token},
	} {
		dst := filepath.Join(t.TempDir(), "asset.tgz")
		var last int64
		var calls int
		c.NoError(download(t.Context(), nil, one.asset, one.token, dst, func(read int64) {
			calls++
			last = read
		}), one.name)

		got, err := os.ReadFile(dst)
		c.NoError(err, one.name)
		c.Equal(payload, got, one.name)
		c.True(calls > 0, "%s: progress must be reported", one.name)
		c.Equal(int64(len(payload)), last, "%s: progress must finish at the full size", one.name)
	}
}

// A body that is not what GitHub published must never reach the disk as if it were: one that fails the integrity check
// is refused, as is one of the wrong length in either direction, since a short body is a truncated transfer and a long
// one means the response is not the asset that was described. An asset with no checksum cannot be verified, so it is
// refused too; preflight already refuses these, so reaching here would mean a caller bypassed it.
func TestDownloadRejectsAWrongLength(t *testing.T) {
	c := check.New(t)
	payload := []byte("0123456789")

	for name, mutate := range map[string]func(*Asset){
		"claims more than it sends":   func(a *Asset) { a.Size = int64(len(payload)) + 5 },
		"claims less than it sends":   func(a *Asset) { a.Size = int64(len(payload)) - 5 },
		"claims a different checksum": func(a *Asset) { a.SHA256 = hex.EncodeToString(make([]byte, sha256.Size)) },
		"claims no checksum":          func(a *Asset) { a.SHA256 = "" },
	} {
		_, asset := serveBytes(t, payload)
		mutate(&asset)
		dst := filepath.Join(t.TempDir(), "asset.tgz")
		c.HasError(download(t.Context(), nil, asset, "", dst, nil), name)
		c.False(exists(dst), "%s left a file behind", name)
	}
}

// An HTTP error must be reported as one, rather than the error page being written out and handed to the extractor as
// though it were an archive.
func TestDownloadRejectsAnErrorResponse(t *testing.T) {
	c := check.New(t)
	for _, code := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
			if _, err := w.Write([]byte("<html>nope</html>")); err != nil {
				t.Error(err)
			}
		}))
		asset := Asset{Name: "asset.tgz", URL: srv.URL, SHA256: testDigest, Size: 17}
		dst := filepath.Join(t.TempDir(), "asset.tgz")
		err := download(t.Context(), srv.Client(), asset, "", dst, nil)
		srv.Close()
		c.HasError(err, "status %d", code)
		c.Contains(err.Error(), strconv.Itoa(code), "status %d", code)
		c.False(exists(dst), "status %d left a file behind", code)
	}
}

// The Cancel button must actually stop the transfer and clean up, rather than leaving a partial file that a later run
// might find.
func TestDownloadStopsWhenCanceled(t *testing.T) {
	c := check.New(t)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("the beginning of the file")); err != nil {
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(t.Context())
	asset := Asset{Name: "asset.tgz", URL: srv.URL, SHA256: testDigest, Size: 1 << 20}
	dst := filepath.Join(t.TempDir(), "asset.tgz")

	done := make(chan error, 1)
	go func() {
		done <- download(ctx, srv.Client(), asset, "", dst, func(int64) { cancel() })
	}()

	select {
	case err := <-done:
		c.HasError(err)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the canceled download to return")
	}
	c.False(exists(dst), "a canceled download must not leave a partial file behind")
}

// The destination is created exclusively, so two updates racing in the same staging directory cannot interleave into
// one file.
func TestDownloadRefusesAnExistingDestination(t *testing.T) {
	c := check.New(t)
	_, asset := serveBytes(t, []byte("content"))
	dst := filepath.Join(t.TempDir(), "asset.tgz")
	write(t, dst, "already here")
	c.HasError(download(t.Context(), nil, asset, "", dst, nil))
	c.Equal("already here", read(t, dst), "the existing file must be left alone")
}
