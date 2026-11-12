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
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/richardwilkes/toolbox/v2/errs"
	"golang.org/x/sys/unix"
)

// detachAttempts and detachDelay bound the retries when unmounting the disk image. Spotlight indexes a newly mounted
// volume and briefly holds it, so the first detach routinely fails on a volume nothing is actually using.
const (
	detachAttempts = 4
	detachDelay    = 500 * time.Millisecond
)

// attachAttempts and attachDelay bound the retries when mounting the disk image. Only the failure hdiutil reports as
// attachBusy is retried: it means another process holds a lock on the image, which is usually gone a moment later.
// diskutil mounts an image regardless of such a lock.
const (
	attachAttempts = 5
	attachDelay    = time.Second
	attachBusy     = "Resource temporarily unavailable"
)

// commandTimeout bounds each external tool invocation. None of these should take more than a few seconds, and a hung
// one would otherwise leave the update dialog spinning forever.
const commandTimeout = 2 * time.Minute

// attachWait bounds how long attachDMG waits for the disk image to mount. A variable only so tests can shorten it.
var attachWait = commandTimeout

// stagePayload mounts the downloaded disk image and copies the application bundle out of it.
func (u *Updater) stagePayload(ctx context.Context, archivePath, dstPath string) (err error) {
	mountPoint, err := os.MkdirTemp(filepath.Dir(archivePath), "mnt-")
	if err != nil {
		return errs.Wrap(err)
	}
	// Neither Cancel nor the update's deadline stops the attach; see attachDMG.
	if err = attachDMG(archivePath, mountPoint); err != nil {
		return err
	}
	defer detachDMG(mountPoint)

	src := filepath.Join(mountPoint, u.cfg.BundleName)
	fi, err := os.Stat(src)
	if err != nil || !fi.IsDir() {
		return errs.New("the downloaded disk image does not hold " + u.cfg.BundleName)
	}
	// ditto rather than a hand-rolled copy: it is the only thing that reliably carries extended attributes, access
	// control lists and BSD flags across, and a bundle that loses those no longer verifies against its signature.
	if err = run(ctx, "/usr/bin/ditto", src, dstPath); err != nil {
		return errs.NewWithCause("unable to unpack the downloaded application", err)
	}
	// Nothing here sets the quarantine attribute -- a download made by this process is not marked the way a browser's
	// would be -- but clearing it costs nothing and covers an image that arrived carrying one.
	if err = run(ctx, "/usr/bin/xattr", "-dr", "com.apple.quarantine", dstPath); err != nil {
		slog.Warn("unable to clear the quarantine attribute", "error", err)
	}
	return nil
}

// stageHelper places a copy of the running application in the staging directory and returns the executable to run. See
// stage for why the helper is a copy at all.
//
// The whole bundle is copied, not just the executable inside it. That executable's own signature records the hashes of
// Contents/Info.plist and of the sealed resource directory, so on its own it carries a signature that can never be
// satisfied: macOS kills the process at exec, before a line of this program runs, and nothing reports it -- the helper
// starts as the application quits, and a process the kernel kills writes no log.
//
// For the same reason the copy has to keep the executable's name and its position within the bundle. Only the name of
// the bundle directory itself is free, and it deliberately does not end in ".app", so that Launch Services does not
// register a second application with the same bundle identifier sitting in the staging directory.
//
// ditto is used for the reason given in stagePayload. For a target that is a bare executable rather than a bundle, this
// copies just that file, as the other platforms do.
func stageHelper(ctx context.Context, t *target, workDir string) (string, error) {
	dst := filepath.Join(workDir, helperName)
	if err := run(ctx, "/usr/bin/ditto", t.Path, dst); err != nil {
		return "", errs.NewWithCause("unable to prepare the update helper", err)
	}
	return t.execWithin(dst), nil
}

// diskutilMajorVersion is the first major version of macOS on which the disk image is attached and detached with
// diskutil rather than hdiutil, whose attach and detach print deprecation warnings there pointing to diskutil image
// attach and diskutil eject. diskutil gained its image verb only in macOS 13, and its use here has been verified only
// from this version on, so earlier versions keep hdiutil, which works on all of them.
const diskutilMajorVersion = 27

// useDiskutil reports whether the running macOS is diskutilMajorVersion or later. A version that cannot be read keeps
// hdiutil.
func useDiskutil() bool {
	version, err := unix.Sysctl("kern.osproductversion")
	if err != nil {
		return false
	}
	major, _, _ := strings.Cut(version, ".")
	n, err := strconv.Atoi(major)
	return err == nil && n >= diskutilMajorVersion
}

// attachDMG mounts the disk image at an explicit mount point.
//
// The mount point is given rather than discovered because upack names the volume after the application and its version,
// and a user who already has that disk image open pushes ours to the same name with a number appended. Naming it
// removes both the guess and the need to parse the attach's plist output, which Go has no standard reader for.
//
// -noverify skips re-checksumming the image, which the SHA-256 check during the download already covered far more
// strictly, and which on a 30MB image is otherwise several seconds of doing it twice. diskutil has no such option and
// needs none, since it does not re-checksum the image.
//
// Nothing stops the attach once it has started: hdiutil stopped part-way leaves the attach to finish without it,
// mounting the image a moment later with nothing left to detach it. attachWait therefore bounds only how long this
// waits, and an attach still running then is detached in the background once it finishes.
func attachDMG(dmgPath, mountPoint string) error {
	tool, args := "/usr/bin/hdiutil", []string{
		"attach", "-nobrowse", "-readonly", "-noverify", "-noautoopen", "-mountpoint", mountPoint, dmgPath,
	}
	if useDiskutil() {
		tool, args = "/usr/sbin/diskutil", []string{
			"image", "attach", "--readOnly", "--nobrowse", "--mountPoint", mountPoint, dmgPath,
		}
	}
	done := make(chan error, 1)
	go func() {
		done <- retry(attachAttempts, attachDelay, func() error {
			out, err := exec.Command(tool, args...).CombinedOutput()
			if err == nil {
				return nil
			}
			err = commandFailure(tool, out, err)
			if !bytes.Contains(out, []byte(attachBusy)) {
				return stopRetrying(err)
			}
			return err
		})
	}()
	timer := time.NewTimer(attachWait)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			return errs.NewWithCause("unable to open the downloaded disk image", err)
		}
		return nil
	case <-timer.C:
		go func() {
			if <-done == nil {
				detachDMG(mountPoint)
			}
		}()
		return errs.Newf("unable to open the downloaded disk image: %s did not finish within %s",
			filepath.Base(tool), attachWait)
	}
}

// detachDMG unmounts the disk image, retrying before resorting to force. A volume that will not unmount is not a reason
// to fail an update that has otherwise succeeded, so this only ever logs.
func detachDMG(mountPoint string) {
	tool, args, forceArgs := "/usr/bin/hdiutil", []string{"detach", "-quiet", mountPoint},
		[]string{"detach", "-force", "-quiet", mountPoint}
	if useDiskutil() {
		tool, args, forceArgs = "/usr/sbin/diskutil", []string{"eject", mountPoint}, []string{"eject", "force", mountPoint}
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	if retry(detachAttempts, detachDelay, func() error { return run(ctx, tool, args...) }) == nil {
		return
	}
	if err := run(ctx, tool, forceArgs...); err != nil {
		slog.Warn("unable to unmount the downloaded disk image", "mountPoint", mountPoint, "error", err)
	}
}

// verifyPayload establishes that the downloaded application is intact and came from the same developer as the copy
// already installed.
//
// Nothing else will check: a download made by this process is not marked with the quarantine attribute the way a
// browser's would be, so Gatekeeper never examines the result. What follows is the only verification that happens.
func verifyPayload(ctx context.Context, payloadPath, installedPath string) error {
	// A cryptographic check of the bundle against its own signature. Offline, and independent of any system policy.
	if err := run(ctx, "/usr/bin/codesign", "--verify", "--strict", "--deep", payloadPath); err != nil {
		return errs.NewWithCause("the downloaded application is not correctly signed", err)
	}
	// The property that actually matters: the replacement is signed by the same developer as the application the user
	// already trusts. Comparing against the installed copy rather than a name written into this source means the check
	// keeps working if the signing identity ever changes.
	wantTeam, err := teamIdentifier(ctx, installedPath)
	if err != nil {
		return err
	}
	gotTeam, err := teamIdentifier(ctx, payloadPath)
	if err != nil {
		return err
	}
	if wantTeam != gotTeam {
		return errs.New("the downloaded application is signed by a different developer")
	}
	// Whether Gatekeeper would admit this bundle is worth knowing but cannot be a condition. The notarization ticket is
	// stapled to the disk image, not to the application inside it, so a bundle copied out has none and the assessment
	// has to reach Apple -- it fails offline on a perfectly good copy. It also consults user policy, so it approves
	// everything on a machine with Gatekeeper turned off.
	if err = run(ctx, "/usr/sbin/spctl", "--assess", "--type", "execute", payloadPath); err != nil {
		slog.Warn("the downloaded application did not pass a Gatekeeper assessment; continuing on the strength of "+
			"its signature", "error", err)
	}
	return nil
}

// teamIdentifier returns the Apple developer team the bundle is signed by.
func teamIdentifier(ctx context.Context, path string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	// codesign writes its description to standard error.
	out, err := exec.CommandContext(cctx, "/usr/bin/codesign", "-dv", path).CombinedOutput()
	if err != nil {
		return "", errs.NewWithCause("unable to inspect the signature of "+filepath.Base(path), err)
	}
	for line := range strings.Lines(string(out)) {
		if team, ok := strings.CutPrefix(strings.TrimSpace(line), "TeamIdentifier="); ok {
			team = strings.TrimSpace(team)
			if team == "" || team == "not set" {
				return "", errs.New(filepath.Base(path) + " is not signed by an identified developer")
			}
			return team, nil
		}
	}
	return "", errs.New("unable to determine who signed " + filepath.Base(path))
}

// run executes a command, discarding its output unless it fails, in which case the output becomes the error's detail.
func run(ctx context.Context, name string, args ...string) error {
	cctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	out, err := exec.CommandContext(cctx, name, args...).CombinedOutput()
	if err != nil {
		return commandFailure(name, out, err)
	}
	return nil
}

// commandFailure describes the failure of the command name, with its output as the detail.
func commandFailure(name string, out []byte, err error) error {
	detail := strings.TrimSpace(string(out))
	if detail == "" {
		return errs.NewWithCause(filepath.Base(name)+" failed", err)
	}
	return errs.NewWithCause(filepath.Base(name)+" failed: "+detail, err)
}
