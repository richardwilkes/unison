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
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// tgzEntry describes one file to place into a test archive.
type tgzEntry struct {
	name     string
	body     []byte
	typeflag byte
}

// makeTGZ builds a gzip-compressed tar in memory. The single-regular-file case has the same shape as what upack's
// packager_linux.go emits, so the happy-path test exercises the real archive shape.
func makeTGZ(t *testing.T, entries ...tgzEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		flag := e.typeflag
		if flag == 0 {
			flag = tar.TypeReg
		}
		if err := tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Size:     int64(len(e.body)),
			Mode:     0o755,
			ModTime:  time.Unix(0, 0),
			Typeflag: flag,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// makeZip builds a zip archive on disk, matching what upack's packager_windows.go emits.
func makeZip(t *testing.T, names []string, bodies [][]byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "archive.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for i, name := range names {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Unix(0, 0)}
		hdr.SetMode(0o755)
		var w interface{ Write([]byte) (int, error) }
		if w, err = zw.CreateHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(bodies[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// umask077Env, when set, marks a run of this test binary that TestExtractSingleTGZ started under a umask of 077.
const umask077Env = "UPDATER_TEST_UMASK_077"

// TestExtractSingleTGZ verifies the happy path against the archive shape the packager actually produces, including that
// the result is executable under a umask of 077, which alone would make it 0700.
func TestExtractSingleTGZ(t *testing.T) {
	c := check.New(t)
	if runtime.GOOS != xos.WindowsOS && os.Getenv(umask077Env) == "" {
		// The umask cannot be set from Go on every platform, so a copy of this test runs under a shell that sets it.
		t.Setenv(umask077Env, "1")
		exe, err := os.Executable()
		c.NoError(err)
		out, err := exec.Command("/bin/sh", "-c", `umask 077 && exec "$0" -test.run '^TestExtractSingleTGZ$'`,
			exe).CombinedOutput()
		c.NoError(err, "under a umask of 077:\n%s", out)
	}
	dst := filepath.Join(t.TempDir(), testCmd)
	c.NoError(extractSingleTGZ(bytes.NewReader(makeTGZ(t, tgzEntry{name: testCmd, body: []byte("\x7fELF binary")})),
		testCmd, dst))
	data, err := os.ReadFile(dst)
	c.NoError(err)
	c.Equal([]byte("\x7fELF binary"), data)
	if runtime.GOOS != xos.WindowsOS {
		fi, statErr := os.Stat(dst)
		c.NoError(statErr)
		c.Equal(os.FileMode(executableModePerm), fi.Mode().Perm(),
			"the extracted file must be executable whatever the umask")
	}
}

// TestExtractSingleTGZRejectsBadArchives verifies that anything other than exactly the one expected file, or that file
// expanding past maxPayloadSize, is refused without leaving a partial file behind. Requiring an exact name with no
// directory component is what makes path traversal structurally impossible here, so the traversal cases below are
// rejected by name matching rather than by a separate filter.
func TestExtractSingleTGZRejectsBadArchives(t *testing.T) {
	c := check.New(t)
	savedMax := maxPayloadSize
	maxPayloadSize = 1024
	t.Cleanup(func() { maxPayloadSize = savedMax })
	for _, one := range []struct {
		name    string
		archive []byte
	}{
		{"empty archive", makeTGZ(t)},
		{"wrong name", makeTGZ(t, tgzEntry{name: "something-else", body: []byte("x")})},
		{"path traversal", makeTGZ(t, tgzEntry{name: "../../" + testCmd, body: []byte("x")})},
		{"absolute path", makeTGZ(t, tgzEntry{name: "/etc/cron.d/" + testCmd, body: []byte("x")})},
		{"nested path", makeTGZ(t, tgzEntry{name: "dir/" + testCmd, body: []byte("x")})},
		{"a directory", makeTGZ(t, tgzEntry{name: testCmd, typeflag: tar.TypeDir})},
		{"a symlink", makeTGZ(t, tgzEntry{name: testCmd, typeflag: tar.TypeSymlink})},
		{"two files", makeTGZ(t, tgzEntry{name: testCmd, body: []byte("x")}, tgzEntry{name: "extra", body: []byte("y")})},
		{"empty file", makeTGZ(t, tgzEntry{name: testCmd})},
		{"not gzip", []byte("this is not a gzip stream at all")},
		{"an over-large archive", makeTGZ(t, tgzEntry{name: testCmd, body: make([]byte, maxPayloadSize+1)})},
	} {
		dst := filepath.Join(t.TempDir(), testCmd)
		err := extractSingleTGZ(bytes.NewReader(one.archive), testCmd, dst)
		c.HasError(err, one.name)
		_, statErr := os.Stat(dst)
		c.True(os.IsNotExist(statErr), "%s left a partial file behind", one.name)
	}
}

// TestExtractSingleZip verifies the Windows archive path. It is pure Go, so it is exercised on every platform rather
// than only where it ships.
func TestExtractSingleZip(t *testing.T) {
	c := check.New(t)
	exe := testCmd + ".exe"
	archive := makeZip(t, []string{exe}, [][]byte{[]byte("MZ binary")})
	dst := filepath.Join(t.TempDir(), exe)
	c.NoError(extractSingleZip(archive, exe, dst))
	data, err := os.ReadFile(dst)
	c.NoError(err)
	c.Equal([]byte("MZ binary"), data)
}

// TestExtractSingleZipRejectsBadArchives mirrors the tar cases for the Windows archive format.
func TestExtractSingleZipRejectsBadArchives(t *testing.T) {
	c := check.New(t)
	exe := testCmd + ".exe"
	for _, one := range []struct {
		name   string
		names  []string
		bodies [][]byte
	}{
		{"empty archive", nil, nil},
		{"wrong name", []string{"other.exe"}, [][]byte{[]byte("x")}},
		{"path traversal", []string{"../../" + exe}, [][]byte{[]byte("x")}},
		{"nested path", []string{"dir/" + exe}, [][]byte{[]byte("x")}},
		{"two files", []string{exe, "extra"}, [][]byte{[]byte("x"), []byte("y")}},
		{"empty file", []string{exe}, [][]byte{{}}},
	} {
		archive := makeZip(t, one.names, one.bodies)
		dst := filepath.Join(t.TempDir(), exe)
		c.HasError(extractSingleZip(archive, exe, dst), one.name)
		_, statErr := os.Stat(dst)
		c.True(os.IsNotExist(statErr), "%s left a partial file behind", one.name)
	}

	c.HasError(extractSingleZip(filepath.Join(t.TempDir(), "missing.zip"), exe, filepath.Join(t.TempDir(), exe)),
		"a missing archive must be an error")
}
