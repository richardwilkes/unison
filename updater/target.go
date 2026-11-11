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
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// kind distinguishes what an update replaces.
type kind byte

const (
	// kindExecutable means the target is the executable file itself, as on Linux and Windows.
	kindExecutable kind = iota
	// kindBundle means the target is a macOS application bundle directory.
	kindBundle
)

// target describes what an update replaces and what to run afterwards.
type target struct {
	// Path is what gets swapped: the .app directory on macOS, the executable elsewhere.
	Path string
	// Parent is the directory holding Path. Staging happens here, so the swap is a rename within one filesystem.
	Parent string
	// Exec is the executable to run, which is inside Path for a bundle and equal to it otherwise.
	Exec string
	Kind kind
}

// resolveTarget determines what an update running from exePath would replace. exePath is expected to already be
// absolute and symlink-free. This reports what *would* be replaced; whether replacing it is allowed is preflight's job.
//
// goos is a parameter rather than being read from runtime.GOOS so that the rules for every platform can be exercised
// from any host; callers outside of tests should use currentTarget.
func resolveTarget(exePath, goos string) (target, error) {
	if exePath == "" {
		return target{}, errs.New("an executable path is required")
	}
	if goos != xos.MacOS {
		return target{Path: exePath, Parent: filepath.Dir(exePath), Exec: exePath, Kind: kindExecutable}, nil
	}
	// On macOS the executable normally lives at <name>.app/Contents/MacOS/<name>, and it is the bundle as a whole that
	// has to be replaced, since swapping just the executable would leave the bundle's signature broken. Walk up looking
	// for the enclosing bundle rather than assuming a fixed depth.
	dir := filepath.Dir(exePath)
	for {
		if strings.HasSuffix(dir, ".app") {
			return target{Path: dir, Parent: filepath.Dir(dir), Exec: exePath, Kind: kindBundle}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached the filesystem root without finding a bundle: a bare executable, not an installed application.
			return target{Path: exePath, Parent: filepath.Dir(exePath), Exec: exePath, Kind: kindExecutable}, nil
		}
		dir = parent
	}
}

// currentTarget determines what an update to the running application would replace. Symlinks are resolved first, so
// that a symlink on the user's path is left alone and the real installation is what gets replaced.
func currentTarget() (target, error) {
	exePath, err := os.Executable()
	if err != nil {
		return target{}, errs.Wrap(err)
	}
	if exePath, err = filepath.EvalSymlinks(exePath); err != nil {
		return target{}, errs.Wrap(err)
	}
	if exePath, err = filepath.Abs(exePath); err != nil {
		return target{}, errs.Wrap(err)
	}
	return resolveTarget(exePath, runtime.GOOS)
}

// backupPath returns the path to move the existing installation aside to. The unique suffix keeps a leftover from an
// earlier attempt from colliding, and the shape of the name is what lets the startup sweep recognize these.
//
// Two properties of the name matter on macOS: it no longer ends in ".app", so a bundle moved aside is not a second live
// Launch Services registration of the application's bundle identifier that document associations could resolve to, and
// it starts with a dot, so the Finder does not show the user a mysterious extra item mid-update.
func (t *target) backupPath(unique string) string {
	return filepath.Join(t.Parent, "."+filepath.Base(t.Path)+backupSuffix+unique)
}

// freeBackupPath returns a backup path that nothing currently occupies. The timestamp alone makes a collision all but
// impossible, but the swap renames the installation onto this path, which on most systems silently destroys whatever is
// there -- possibly the user's only other copy of the application.
func (t *target) freeBackupPath() string {
	base := strconv.FormatInt(time.Now().UnixNano(), 36)
	path := t.backupPath(base)
	for i := 1; ; i++ {
		if _, err := os.Lstat(path); err != nil {
			return path
		}
		path = t.backupPath(base + "-" + strconv.Itoa(i))
	}
}

// payloadPath returns where the replacement lives inside a staging directory.
func (t *target) payloadPath(workDir string) string {
	return filepath.Join(workDir, filepath.Base(t.Path))
}

// execWithin returns the executable to run for an installation rooted at path, which may be a staging copy rather than
// the installed one. This is what makes it possible to smoke-test the replacement before committing to it.
func (t *target) execWithin(path string) string {
	if t.Kind == kindBundle {
		rel, err := filepath.Rel(t.Path, t.Exec)
		if err != nil {
			return filepath.Join(path, "Contents", "MacOS", filepath.Base(t.Exec))
		}
		return filepath.Join(path, rel)
	}
	return path
}
