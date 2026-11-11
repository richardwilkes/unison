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
	"path/filepath"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// hostPath rewrites a path written with forward slashes into the host's own separator form.
//
// resolveTarget takes goos as a parameter so the rules for every platform can be exercised from any host, but the path
// handling underneath it is path/filepath, which is compiled for the host rather than for the goos being exercised. The
// rules are what these tests are about, so separators are normalized on both sides of every comparison; otherwise the
// macOS cases would fail on a Windows host purely over backslashes.
func hostPath(path string) string {
	return filepath.FromSlash(path)
}

// An update running from inside an application bundle must replace the bundle rather than the executable within it.
// Swapping just the executable would leave a bundle whose code signature no longer matches its contents, which macOS
// kills on sight. The search walks up rather than assuming a fixed depth, so something nested deeper inside a bundle
// still resolves to the nearest enclosing bundle.
func TestResolveTargetOnMacOSFindsTheBundle(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		exePath string
		bundle  string
		parent  string
	}{
		{
			hostPath("/Applications/" + testBundle + "/Contents/MacOS/" + testCmd),
			hostPath("/Applications/" + testBundle),
			hostPath("/Applications"),
		},
		{
			hostPath("/Applications/" + testBundle + "/Contents/Frameworks/Inner.app/Contents/MacOS/tool"),
			hostPath("/Applications/" + testBundle + "/Contents/Frameworks/Inner.app"),
			hostPath("/Applications/" + testBundle + "/Contents/Frameworks"),
		},
	} {
		tgt, err := resolveTarget(one.exePath, xos.MacOS)
		c.NoError(err, one.exePath)
		c.Equal(kindBundle, tgt.Kind, one.exePath)
		c.Equal(one.bundle, tgt.Path, "the nearest enclosing bundle is the one found for %s", one.exePath)
		c.Equal(one.parent, tgt.Parent, one.exePath)
		c.Equal(one.exePath, tgt.Exec, one.exePath)
	}
}

// Linux and Windows replace the single executable, which is all their archives contain. On macOS, a bare executable --
// a development build run straight from the build directory -- resolves to the executable itself too; preflight is what
// refuses to update it, while resolution just reports what it sees.
func TestResolveTargetElsewhereIsTheExecutable(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		exePath string
		goos    string
		parent  string
	}{
		{hostPath("/home/someone/bin/" + testCmd), xos.LinuxOS, hostPath("/home/someone/bin")},
		{hostPath("/usr/local/bin/" + testCmd), xos.LinuxOS, hostPath("/usr/local/bin")},
		{`C:\Apps\Example\` + testCmd + `.exe`, xos.WindowsOS, `C:\Apps\Example`},
		{hostPath("/Users/someone/code/example/" + testCmd), xos.MacOS, hostPath("/Users/someone/code/example")},
	} {
		tgt, err := resolveTarget(one.exePath, one.goos)
		c.NoError(err, one.exePath)
		c.Equal(kindExecutable, tgt.Kind, one.exePath)
		c.Equal(one.exePath, tgt.Path, one.exePath)
		c.Equal(one.exePath, tgt.Exec, one.exePath)
		// The Windows path is the one case hostPath cannot normalize, since a host using forward slashes does not see
		// its backslashes as separators at all. Its parent is only meaningful on a Windows host.
		if one.goos != xos.WindowsOS || filepath.Separator == '\\' {
			c.Equal(one.parent, tgt.Parent, one.exePath)
		}
	}
}

// A failed os.Executable must not produce a target rooted at the current directory.
func TestResolveTargetRejectsAnEmptyPath(t *testing.T) {
	c := check.New(t)
	_, err := resolveTarget("", xos.LinuxOS)
	c.HasError(err)
	_, err = resolveTarget("", xos.MacOS)
	c.HasError(err)
}

// The property that keeps a displaced macOS installation from confusing Launch Services: the name it is moved aside to
// must no longer end in ".app", or /Applications briefly holds two complete bundles both claiming the application's
// bundle identifier. On every platform the backup must be a sibling of what it replaces, since landing anywhere else
// would make the swap a cross-device copy rather than a rename.
func TestBackupPathIsNotABundle(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		exePath string
		goos    string
		parent  string
	}{
		{hostPath("/Applications/" + testBundle + "/Contents/MacOS/" + testCmd), xos.MacOS, hostPath("/Applications")},
		{hostPath("/home/someone/bin/" + testCmd), xos.LinuxOS, hostPath("/home/someone/bin")},
	} {
		tgt, err := resolveTarget(one.exePath, one.goos)
		c.NoError(err, one.exePath)
		backup := tgt.backupPath("123")
		c.False(strings.HasSuffix(backup, ".app"), "got %s", backup)
		c.Equal(one.parent, filepath.Dir(backup), "the backup must be a sibling, so the move is a rename")
		c.NotEqual(tgt.Path, backup)
		c.True(strings.HasPrefix(filepath.Base(backup), "."), "the backup should be hidden; got %s", backup)
		c.NotEqual(backup, tgt.backupPath("456"), "the unique suffix must actually vary")
	}
}

// The name chosen for the displaced installation must be one nothing occupies, which is what keeps the swap's refusal
// to overwrite an existing backup from being reachable in practice.
func TestFreeBackupPathAvoidsWhatIsAlreadyThere(t *testing.T) {
	c := check.New(t)
	dir := t.TempDir()
	tgt := target{Path: filepath.Join(dir, testCmd), Parent: dir, Kind: kindExecutable}

	first := tgt.freeBackupPath()
	write(t, first, "taken")
	second := tgt.freeBackupPath()

	c.NotEqual(first, second, "a path already in use must not be chosen again")
	c.False(exists(second))
	c.Equal(dir, filepath.Dir(second), "the backup must still be a sibling of what it replaces")
}

// The staged replacement keeps the name of what it replaces, directly inside the staging directory.
func TestPayloadPath(t *testing.T) {
	c := check.New(t)
	bundle, err := resolveTarget(hostPath("/Applications/"+testBundle+"/Contents/MacOS/"+testCmd), xos.MacOS)
	c.NoError(err)
	workDir := hostPath("/Applications/.example-update-x")
	c.Equal(hostPath("/Applications/.example-update-x/"+testBundle), bundle.payloadPath(workDir))
}

// execWithin locates the executable inside the installed or a staged bundle rather than returning the bundle, since
// that is what the smoke test and the Launch Services fallback in relaunch run. When the recorded executable cannot be
// placed relative to the bundle, it falls back to the conventional location, keeping the executable's own name. For a
// bare executable, it returns the path it is given.
func TestExecWithin(t *testing.T) {
	c := check.New(t)
	bundle := hostPath("/Applications/" + testBundle)
	exe := hostPath("/Applications/" + testBundle + "/Contents/MacOS/" + testCmd)
	staged := hostPath("/tmp/staged/" + testBundle)
	stagedExe := hostPath("/tmp/staged/" + testBundle + "/Contents/MacOS/" + testCmd)
	for _, one := range []struct {
		name string
		path string
		want string
		tgt  target
	}{
		{
			name: "the installed bundle",
			tgt:  target{Path: bundle, Exec: exe, Kind: kindBundle},
			path: bundle,
			want: exe,
		},
		{
			name: "a staged bundle",
			tgt:  target{Path: bundle, Exec: exe, Kind: kindBundle},
			path: staged,
			want: stagedExe,
		},
		{
			name: "an executable that cannot be placed",
			tgt:  target{Path: bundle, Exec: "relative/" + testCmd, Kind: kindBundle},
			path: staged,
			want: stagedExe,
		},
		{
			name: "a bare executable",
			tgt:  target{Path: hostPath("/home/someone/bin/" + testCmd), Kind: kindExecutable},
			path: hostPath("/tmp/staged/" + testCmd),
			want: hostPath("/tmp/staged/" + testCmd),
		},
	} {
		c.Equal(one.want, one.tgt.execWithin(one.path), one.name)
	}
}
