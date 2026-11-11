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
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/richardwilkes/toolbox/v2/check"
	"github.com/richardwilkes/toolbox/v2/xos"
	"github.com/richardwilkes/unison"
)

// The identity the tests give the application. It is deliberately not that of any real application, so that nothing
// here can pass only because it matches a literal written into the package.
const (
	testApp    = "Example"
	testCmd    = "example"
	testBundle = "Example.app"
	// testVersion is the running version.
	testVersion = "1.2.3"
	// testNextVersion is the release being updated to.
	testNextVersion = "1.3.0"
	// testDigest stands in for a SHA-256 digest wherever its value does not matter.
	testDigest = "abc"
)

// newTestUpdater returns an Updater for the application the tests describe.
func newTestUpdater(t *testing.T) *Updater {
	t.Helper()
	return New(&Config{AppName: testApp, CmdName: testCmd, BundleName: testBundle, Version: testVersion})
}

// startHeadless starts a headless session for the tests that put up a dialog, and shuts it down when the test ends. The
// session is told not to quit when its last window closes, since the dialogs under test are the only windows it has. A
// session owns most of unison's mutable globals while it runs, so those tests must not call t.Parallel.
func startHeadless(t *testing.T) *unison.HeadlessScreen {
	t.Helper()
	screen, err := unison.StartHeadless(unison.HeadlessConfig{Width: 1024, Height: 768},
		unison.QuitAfterLastWindowClosedCallback(func() bool { return false }))
	if err != nil {
		t.Fatalf("unable to start headless session: %v", err)
	}
	t.Cleanup(screen.Stop)
	return screen
}

// modalDialog returns the dialog that is up, or nil if the focused window is not one.
func modalDialog(screen *unison.HeadlessScreen) *unison.Dialog {
	var dialog *unison.Dialog
	screen.Do(func() {
		if wnd := screen.FocusedWindow(); wnd != nil {
			dialog, _ = wnd.ClientData()[unison.DialogClientDataKey].(*unison.Dialog) //nolint:errcheck // nil is the answer
		}
	})
	return dialog
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// listing returns every path under dir, relative to it, so that two moments can be compared.
func listing(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		paths = append(paths, rel)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return paths
}

// Every field New fills in has a default an application can rely on leaving unset, and a value an application sets is
// kept. The bundle name follows the command name actually in use, and the default command name drops the ".exe" Windows
// reports, since every name derived from it adds its own extension.
func TestNewFillsInDefaults(t *testing.T) {
	c := check.New(t)
	defaultCmd := xos.AppCmdName
	if runtime.GOOS == xos.WindowsOS && strings.HasSuffix(strings.ToLower(defaultCmd), ".exe") {
		defaultCmd = defaultCmd[:len(defaultCmd)-len(".exe")]
	}
	defaults := &Config{
		AppName:     xos.AppName,
		CmdName:     defaultCmd,
		BundleName:  defaultCmd + ".app",
		Version:     xos.AppVersion,
		VersionArgs: []string{"-v"},
	}
	client := &http.Client{}
	for _, one := range []struct {
		in   *Config
		want *Config
		name string
	}{
		{name: "no config", in: nil, want: defaults},
		{name: "an empty config", in: &Config{}, want: defaults},
		{
			name: "only a command name",
			in:   &Config{CmdName: testCmd},
			want: &Config{
				AppName:     xos.AppName,
				CmdName:     testCmd,
				BundleName:  testCmd + ".app",
				Version:     xos.AppVersion,
				VersionArgs: []string{"-v"},
			},
		},
		{
			name: "everything set",
			in: &Config{
				AppName:     testApp,
				CmdName:     testCmd,
				BundleName:  "Other.app",
				Version:     testVersion,
				VersionArgs: []string{"--version"},
				HTTPClient:  client,
			},
			want: &Config{
				AppName:     testApp,
				CmdName:     testCmd,
				BundleName:  "Other.app",
				Version:     testVersion,
				VersionArgs: []string{"--version"},
				HTTPClient:  client,
			},
		},
	} {
		got := New(one.in).cfg
		c.NotNil(got.HTTPClient, one.name)
		if one.want.HTTPClient == nil {
			got.HTTPClient = nil
		}
		c.Equal(*one.want, got, one.name)
	}
}
