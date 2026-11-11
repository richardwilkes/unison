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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison/internal/testenv"
)

// signedBundle builds a directory shaped like an application bundle around a real Mach-O and signs it, then returns
// what an update to it would replace.
//
// An ad-hoc signature is enough: it seals Contents/Info.plist and the resource directory exactly as the Developer ID
// signature on a released build does, and it is those seals that decide whether a copy of the executable can run. That
// makes what follows reproducible on any Mac, with no signing identity and no network.
func signedBundle(t *testing.T, dir string) target {
	t.Helper()
	for _, tool := range []string{"/usr/bin/codesign", "/usr/bin/ditto"} {
		if _, err := os.Stat(tool); err != nil {
			t.Skipf("%s is needed for this test", tool)
		}
	}
	bundle := filepath.Join(dir, testBundle)
	exePath := filepath.Join(bundle, "Contents", "MacOS", testCmd)
	// Any real Mach-O will do, and one that is on every macOS installation avoids building one here. A plain copy of
	// the bytes suffices, since this only stands in for an executable and the signature that matters is applied below.
	data, err := os.ReadFile("/bin/echo")
	if err != nil {
		t.Skipf("unable to prepare an executable for this test: %v", err)
	}
	if err = os.MkdirAll(filepath.Dir(exePath), 0o755); err != nil {
		t.Fatal(err)
	}
	//nolint:gosec // Copies a system executable into the test's own temporary directory
	if err = os.WriteFile(exePath, data, executableModePerm); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(bundle, "Contents", "Info.plist"), `<?xml version="1.0" encoding="UTF-8"?>`+
		`<plist version="1.0"><dict><key>CFBundleExecutable</key><string>`+testCmd+`</string>`+
		`<key>CFBundleIdentifier</key><string>com.example.updater.test</string></dict></plist>`)
	write(t, filepath.Join(bundle, "Contents", "Resources", "app.icns"), "icon data")
	// --options runtime matches how upack signs a released application, since the hardened runtime is part of what
	// makes macOS insist the signature actually verifies.
	out, err := exec.Command("/usr/bin/codesign", "-s", "-", "-f", "--options", "runtime", bundle).CombinedOutput()
	if err != nil {
		t.Skipf("unable to sign a bundle for this test: %v\n%s", err, out)
	}

	tgt, err := resolveTarget(exePath, xos.MacOS)
	if err != nil {
		t.Fatal(err)
	}
	return tgt
}

// verifySignature asks the same tool the operating system consults whether the code at path would be allowed to run.
func verifySignature(ctx context.Context, path string) error {
	return run(ctx, "/usr/bin/codesign", "--verify", "--strict", path)
}

// A regression test for an update that could never be applied: the helper was once a bare copy of the bundle's main
// executable, which the kernel kills at exec since its signature seals files outside it (see stageHelper), so the
// update silently never happened. Staging the helper must also touch nothing outside the staging directory, so an abort
// here can cost the user nothing.
func TestStageHelperCopiesEnoughOfTheBundleToRun(t *testing.T) {
	c := check.New(t)
	installDir := t.TempDir()
	tgt := signedBundle(t, installDir)
	before := listing(t, installDir)
	workDir := t.TempDir()

	helper, err := stageHelper(t.Context(), &tgt, workDir)
	c.NoError(err)

	c.Equal(before, listing(t, installDir), "staging the helper must leave the installation alone")
	c.True(strings.HasPrefix(helper, workDir+string(filepath.Separator)),
		"the helper must be staged rather than run from the installation being replaced")
	c.NoError(verifySignature(t.Context(), helper),
		"the staged helper does not satisfy its own signature, so macOS will refuse to run it")

	// The same executable copied out on its own, the way it once was, must fail -- otherwise the check above proves
	// nothing. ditto rather than a plain copy, to show that this is not about losing extended attributes on the way.
	bare := filepath.Join(t.TempDir(), helperName)
	c.NoError(run(t.Context(), "/usr/bin/ditto", tgt.Exec, bare))
	c.HasError(verifySignature(t.Context(), bare),
		"a bare copy of the executable is supposed to fail; if it no longer does, this test has stopped guarding anything")

	// The executable has to keep its name and its position within the bundle, since both are part of what is checked.
	c.Equal(filepath.Join(workDir, helperName, "Contents", "MacOS", testCmd), helper)
	c.Equal("icon data", read(t, filepath.Join(workDir, helperName, "Contents", "Resources", "app.icns")),
		"everything the signature seals has to come across, not just the executable and the plist")
	// Only the name of the copied bundle is free, and it must not end in ".app", or Launch Services would index a
	// second application sitting in the staging directory.
	c.False(strings.HasSuffix(helperName, ".app"),
		"the staged copy must not look like a second installed application")
}

// makeDMG builds a real disk image holding a directory shaped like an application bundle, whose executable holds app,
// so the mount, copy and unmount sequence is exercised against the tools it actually uses. The image is given the same
// style of volume name upack produces, since that name is exactly what the mount deliberately does not rely on.
func makeDMG(t *testing.T, volumeName, app string) string {
	t.Helper()
	for _, tool := range []string{"/usr/bin/hdiutil", "/usr/bin/ditto"} {
		if _, err := os.Stat(tool); err != nil {
			t.Skipf("%s is needed for this test", tool)
		}
	}
	src := filepath.Join(t.TempDir(), "src")
	write(t, filepath.Join(src, testBundle, "Contents", "MacOS", testCmd), app)
	write(t, filepath.Join(src, testBundle, "Contents", "Info.plist"), "<plist/>")
	write(t, filepath.Join(src, testBundle, "Contents", "Resources", "app.icns"), "icon data")

	dmg := filepath.Join(t.TempDir(), "test.dmg")
	cmd := exec.Command("/usr/bin/hdiutil", "create", "-quiet", "-srcfolder", src, "-volname", volumeName,
		"-format", "UDZO", "-ov", dmg)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("unable to build a disk image for this test: %v\n%s", err, out)
	}
	return dmg
}

// The macOS unpacking path against a real disk image: mounting it, copying the bundle out, and unmounting again. None
// of that can be exercised any other way, and getting the hdiutil invocation wrong would only show up when a user tried
// to update. It is repeated with the volume name already in use, the case the explicit mount point exists for: upack
// names every volume "<name> v<version>", so a user who already has that disk image open would push a second mount to a
// different, unpredictable path, and anything that guessed the path from the volume name would copy from the wrong
// place or fail outright.
func TestStagePayloadFromADiskImage(t *testing.T) {
	if testing.Short() {
		t.Skip("mounting a disk image is slow")
	}
	c := check.New(t)
	u := newTestUpdater(t)
	volumeName := testApp + " v" + testNextVersion
	for _, occupied := range []bool{false, true} {
		if occupied {
			// Mount another image under the contested name where a user opening it would, and keep it there for the
			// rest of the test. Its application differs, so a copy taken from it cannot pass for the right one.
			occupying := makeDMG(t, volumeName, "another application")
			out, err := exec.Command("/usr/bin/hdiutil", "attach", "-nobrowse", "-readonly", "-noverify", "-noautoopen",
				occupying).Output()
			if err != nil {
				t.Fatalf("unable to attach the occupying disk image: %v", err)
			}
			device := strings.Fields(string(out))[0]
			t.Cleanup(func() {
				c.NoError(run(context.Background(), "/usr/bin/hdiutil", "detach", "-force", "-quiet", device))
			})
			occupyingExe := filepath.Join("/Volumes/"+volumeName, testBundle, "Contents", "MacOS", testCmd)
			c.Equal("another application", read(t, occupyingExe),
				"the occupying image must hold the volume name's usual mount point")
		}

		dmg := makeDMG(t, volumeName, "the application")
		dst := filepath.Join(t.TempDir(), testBundle)

		c.NoError(u.stagePayload(t.Context(), dmg, dst), "volume name in use: %v", occupied)

		c.Equal("the application", read(t, filepath.Join(dst, "Contents", "MacOS", testCmd)),
			"the whole bundle must be copied out, not just its top level")
		c.Equal("icon data", read(t, filepath.Join(dst, "Contents", "Resources", "app.icns")))

		fi, err := os.Stat(filepath.Join(dst, "Contents", "MacOS", testCmd))
		c.NoError(err)
		c.True(fi.Mode().IsRegular())

		// A stray mount would accumulate a volume per update attempt, and would keep the downloaded file alive after
		// the directory holding it was removed.
		out, err := exec.Command("/usr/bin/hdiutil", "info").CombinedOutput()
		c.NoError(err)
		c.NotContains(string(out), dmg, "the disk image was left mounted")
	}
}

// imageAttached reports whether hdiutil lists the disk image at path as attached.
func imageAttached(t *testing.T, path string) bool {
	t.Helper()
	out, err := exec.Command("/usr/bin/hdiutil", "info").CombinedOutput()
	if err != nil {
		t.Errorf("unable to list the attached disk images: %v\n%s", err, out)
		return false
	}
	return strings.Contains(string(out), path)
}

// Stopping the update while the disk image is being attached, by Cancel or the update's deadline, must not leave it
// mounted. hdiutil stopped part-way leaves the attach to finish without it, mounting the image a moment later.
func TestStagePayloadLeavesNothingMountedWhenCanceled(t *testing.T) {
	if testing.Short() {
		t.Skip("mounting a disk image is slow")
	}
	testenv.SkipTimingSensitive(t)
	c := check.New(t)
	dmg := makeDMG(t, testApp+" v"+testNextVersion, "the application")
	t.Cleanup(func() {
		if imageAttached(t, dmg) {
			//nolint:errcheck // The pattern is valid
			mountPoints, _ := filepath.Glob(filepath.Join(filepath.Dir(dmg), "mnt-*"))
			for _, mountPoint := range mountPoints {
				detachDMG(mountPoint)
			}
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	// Canceled as soon as the image is attached, which is while hdiutil is still mounting it.
	polling := make(chan struct{})
	go func() {
		defer close(polling)
		for ctx.Err() == nil {
			if imageAttached(t, dmg) {
				cancel()
			}
		}
	}()

	//nolint:errcheck // Either outcome will do
	_ = newTestUpdater(t).stagePayload(ctx, dmg, filepath.Join(t.TempDir(), testBundle))
	cancel()
	<-polling

	time.Sleep(time.Second) // Long enough for a mount hdiutil left unfinished to appear
	c.False(imageAttached(t, dmg), "the disk image was left mounted")
}

// A disk image that is readable but does not hold what was expected must be refused, rather than producing an empty or
// partial installation.
func TestStagePayloadRejectsAnImageWithoutTheApplication(t *testing.T) {
	if testing.Short() {
		t.Skip("mounting a disk image is slow")
	}
	c := check.New(t)
	src := filepath.Join(t.TempDir(), "src")
	write(t, filepath.Join(src, "SomethingElse.app", "Contents", "MacOS", "other"), "not what was asked for")
	dmg := filepath.Join(t.TempDir(), "test.dmg")
	if out, err := exec.Command("/usr/bin/hdiutil", "create", "-quiet", "-srcfolder", src, "-volname", "Other",
		"-format", "UDZO", "-ov", dmg).CombinedOutput(); err != nil {
		t.Skipf("unable to build a disk image for this test: %v\n%s", err, out)
	}

	dst := filepath.Join(t.TempDir(), testBundle)
	c.HasError(newTestUpdater(t).stagePayload(t.Context(), dmg, dst))
	c.False(exists(dst))
}

// A corrupted download must be reported rather than producing a confusing failure further along.
func TestStagePayloadRejectsSomethingThatIsNotADiskImage(t *testing.T) {
	c := check.New(t)
	dmg := filepath.Join(t.TempDir(), "test.dmg")
	write(t, dmg, "this is not a disk image")
	c.HasError(newTestUpdater(t).stagePayload(t.Context(), dmg, filepath.Join(t.TempDir(), testBundle)))
}
