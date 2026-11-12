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
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
)

// buildStandIn compiles src into a stand-in for the real executable, and returns its path.
func buildStandIn(t *testing.T, src string) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("the Go toolchain is needed to build the stand-in executable")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	// The stand-in needs only the standard library, so it asks for an old Go version -- the go on the PATH need not be
	// the one running this test -- and GOTOOLCHAIN=local keeps that go from downloading a toolchain. GOWORK=off keeps a
	// go.work inherited from the environment, which cannot list this temporary module, from failing the build.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module standin\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(dir, testCmd)
	if runtime.GOOS == xos.WindowsOS {
		exePath += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exePath, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("unable to build the stand-in executable: %v\n%s", err, out)
	}
	return exePath
}

// A build that runs and reports the expected version is accepted. The version comparison must tolerate the "~" a build
// made from a modified checkout appends and the build number xflag's --version adds, which would otherwise be read as
// the wrong version entirely, and an executable exiting cleanly without printing anything, as a Windows GUI build does,
// is accepted too. The build is asked for its version with the arguments the application configured, which may not be
// the default.
func TestVerifyRunsAcceptsAWorkingBuild(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		name string
		src  string
		args []string
	}{
		{name: "the version", src: `package main

import "fmt"

func main() { fmt.Println("1.3.0") }
`},
		{name: "the version with the modified-build marker", src: `package main

import "fmt"

func main() { fmt.Println("1.3.0~") }
`},
		{name: "the version with a build number", src: `package main

import "fmt"

func main() { fmt.Println("1.3.0-42~") }
`},
		{name: "nothing", src: `package main

func main() {}
`},
		{name: "the version when asked with --version", args: []string{"--version"}, src: `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] != "--version" {
		fmt.Fprintln(os.Stderr, "unexpected arguments:", os.Args[1:])
		os.Exit(2)
	}
	fmt.Println("1.3.0")
}
`},
	} {
		u := New(&Config{CmdName: testCmd, Version: testVersion, VersionArgs: one.args})
		c.NoError(u.verifyRuns(t.Context(), buildStandIn(t, one.src), testNextVersion), "a build printing %s", one.name)
	}
}

// The case this check exists for: a downloaded release that cannot start on this machine -- wrong processor, a system
// library too old, a corrupted extraction -- must be caught while the installation is still untouched, rather than at
// the launch that follows the swap, where there is nothing left to fall back to.
func TestVerifyRunsRejectsABuildThatFails(t *testing.T) {
	c := check.New(t)
	exePath := buildStandIn(t, `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "error while loading shared libraries: libc.so.6: version not found")
	os.Exit(1)
}
`)
	err := newTestUpdater(t).verifyRuns(t.Context(), exePath, testNextVersion)
	c.HasError(err)
	c.Contains(err.Error(), "will not run on this system")
	c.Contains(err.Error(), "libc.so.6", "the reason the executable gave must survive into the error")
}

// An executable that starts but is not the release that was downloaded must be refused; it would mean the wrong asset
// had been fetched or unpacked. A version that merely begins with the wanted one is a different version.
func TestVerifyRunsRejectsTheWrongVersion(t *testing.T) {
	c := check.New(t)
	for _, one := range []struct {
		reported string
		want     string
	}{
		{reported: "1.0.0", want: testNextVersion},
		{reported: "1.3.10", want: "1.3.1"},
	} {
		exePath := buildStandIn(t, `package main

import "fmt"

func main() { fmt.Println("`+one.reported+`") }
`)
		err := newTestUpdater(t).verifyRuns(t.Context(), exePath, one.want)
		c.HasError(err, "%s reported for %s", one.reported, one.want)
		if err != nil {
			c.Contains(err.Error(), one.reported)
		}
	}
}

// The case a corrupted download produces: a file that exists and is marked executable but is not a program at all.
func TestVerifyRunsRejectsSomethingUnrunnable(t *testing.T) {
	c := check.New(t)
	u := newTestUpdater(t)
	exePath := filepath.Join(t.TempDir(), testCmd)
	if runtime.GOOS == xos.WindowsOS {
		// Without it, os/exec rejects the name before the loader ever sees the file.
		exePath += ".exe"
	}
	write(t, exePath, "this is definitely not an executable")
	c.NoError(os.Chmod(exePath, executableModePerm)) //nolint:gosec // It has to look executable to be tried
	c.HasError(u.verifyRuns(t.Context(), exePath, testNextVersion))

	c.HasError(u.verifyRuns(t.Context(), filepath.Join(t.TempDir(), "does-not-exist"), testNextVersion))
}
