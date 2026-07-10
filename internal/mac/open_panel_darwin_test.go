// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package mac

import (
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

// panelServiceAvailable reports whether this environment can create NSOpenPanel/NSSavePanel instances. On modern
// macOS both panels are backed by the remote view service com.apple.appkit.xpc.openAndSavePanelService, and on some
// headless CI VMs that service cannot start: +[NSOpenPanel openPanel] then blocks for the ~60s XPC timeout before
// returning nil (observed on the GitHub macos-26-intel runners, while the arm64 runners create panels fine, so a
// plain headless check would wrongly skip machines where panels work). The probe is a raw msgSend independent of
// NewOpenPanel's Retain/WithPool handling, so a genuine regression in the ported code still fails — not skips —
// wherever panels actually work; the result is cached so a broken environment pays the ~60s stall once for the whole
// suite instead of once per test. NSOpenPanel subclasses NSSavePanel and both are served by the same XPC service, so
// the single probe covers both panel kinds. Must be called only from the test goroutine (it submits work through
// runOnMain; calling it from within a runOnMain closure would deadlock).
var panelServiceAvailable = sync.OnceValue(func() bool {
	available := false
	runOnMain(func() {
		WithPool(func() {
			available = objc.ID(Cls("NSOpenPanel")).Send(Sel("openPanel")) != 0
		})
	})
	return available
})

// requirePanelService skips the test when the environment cannot create open/save panels (headless CI without a
// working panel XPC service — see panelServiceAvailable). It must be called from the test goroutine, before
// runOnMain: t.Skip calls runtime.Goexit and must never run inside a runOnMain closure.
func requirePanelService(t *testing.T) {
	t.Helper()
	if !panelServiceAvailable() {
		t.Skip("open/save panel service unavailable in this environment (headless CI)")
	}
}

// nsModalPanelRunLoopModes returns an autoreleased NSArray holding NSModalPanelRunLoopMode, the mode timers must be
// scheduled in for them to fire inside a panel's runModal session. Must be called on the main thread inside a pool.
func nsModalPanelRunLoopModes() objc.ID {
	return NSArrayFromIDs(NSStringConstant("AppKit", "NSModalPanelRunLoopMode"))
}

// cancelModalAfter arranges for panel to receive cancel: after the given delay once its modal session is running,
// plus an abortModal backstop at 10 seconds so a wedged modal session fails the test instead of hanging the whole
// suite (the main-thread pump is blocked while runModal runs). Returns a cleanup func that must be called after
// runModal returns to cancel whichever delayed performs have not fired.
func cancelModalAfter(panel objc.ID, delay float64) (cleanup func()) {
	modes := nsModalPanelRunLoopModes()
	panel.Send(Sel("performSelector:withObject:afterDelay:inModes:"), Sel("cancel:"), objc.ID(0), delay, modes)
	nsApp := objc.ID(Cls("NSApplication")).Send(Sel("sharedApplication"))
	nsApp.Send(Sel("performSelector:withObject:afterDelay:inModes:"), Sel("abortModal"), objc.ID(0), 10.0, modes)
	return func() {
		objc.ID(Cls("NSObject")).Send(Sel("cancelPreviousPerformRequestsWithTarget:"), panel)
		objc.ID(Cls("NSObject")).Send(Sel("cancelPreviousPerformRequestsWithTarget:"), nsApp)
	}
}

func TestOpenPanelBoolAccessors(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewOpenPanel()
			if p == 0 {
				t.Error("NewOpenPanel returned 0")
				return
			}
			defer Release(objc.ID(p))
			checks := []struct {
				get  func() bool
				set  func(bool)
				name string
			}{
				{name: "canChooseFiles", get: p.CanChooseFiles, set: p.SetCanChooseFiles},
				{name: "canChooseDirectories", get: p.CanChooseDirectories, set: p.SetCanChooseDirectories},
				{name: "resolvesAliases", get: p.ResolvesAliases, set: p.SetResolvesAliases},
				{name: "allowsMultipleSelection", get: p.AllowsMultipleSelection, set: p.SetAllowsMultipleSelection},
			}
			for _, c := range checks {
				for _, v := range []bool{true, false, true} {
					c.set(v)
					if got := c.get(); got != v {
						t.Errorf("%s: set %v, read back %v", c.name, v, got)
					}
				}
			}
		})
	})
}

func TestOpenPanelDirectoryURL(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewOpenPanel()
			if p == 0 {
				t.Error("NewOpenPanel returned 0")
				return
			}
			defer Release(objc.ID(p))
			dirURL := NewFileURL("/Library/")
			p.SetDirectoryURL(dirURL)
			dirURL.Release()
			abs := p.DirectoryURL().AbsoluteString()
			parsed, err := url.Parse(abs)
			switch {
			case err != nil:
				t.Errorf("unable to parse %q: %v", abs, err)
			case strings.TrimSuffix(parsed.Path, "/") != "/Library":
				t.Errorf("directory URL = %q, want path /Library", abs)
			}
		})
	})
}

func TestOpenPanelAllowedFileTypes(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewOpenPanel()
			if p == 0 {
				t.Error("NewOpenPanel returned 0")
				return
			}
			defer Release(objc.ID(p))
			// With no types set the handle is 0, and using it the way the root dialog's AllowedExtensions does must
			// be safe and yield an empty slice.
			allowed := p.AllowedFileTypes()
			if objc.ID(allowed) != 0 {
				t.Errorf("AllowedFileTypes = %#x before any set, want 0", objc.ID(allowed))
			}
			if got := allowed.ArrayOfStringToStringSlice(); len(got) != 0 {
				t.Errorf("ArrayOfStringToStringSlice = %q before any set, want empty", got)
			}
			allowed.Release()
			// Root-dialog shape: pass an owned array. The property copies it, so releasing ours afterwards must not
			// disturb the panel (the root dialogs actually leak theirs; that is their pre-existing behavior).
			types := NewArrayFromStringSlice([]string{"png", "jpg"})
			p.SetAllowedFileTypes(types)
			types.Release()
			want := []string{"png", "jpg"}
			// Read back twice, releasing each result: proves the returned reference is owned as documented and the
			// caller's Release does not over-release the panel's copy.
			for i := range 2 {
				allowed = p.AllowedFileTypes()
				got := allowed.ArrayOfStringToStringSlice()
				allowed.Release()
				if !slices.Equal(got, want) {
					t.Errorf("read %d: AllowedFileTypes = %q, want %q", i, got, want)
				}
			}
			// Clearing with 0, as the root dialog does for an empty extension list.
			p.SetAllowedFileTypes(0)
			if got := p.AllowedFileTypes(); objc.ID(got) != 0 {
				t.Errorf("AllowedFileTypes = %#x after clearing, want 0", objc.ID(got))
				got.Release()
			}
		})
	})
}

func TestOpenPanelURLsEmpty(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewOpenPanel()
			if p == 0 {
				t.Error("NewOpenPanel returned 0")
				return
			}
			defer Release(objc.ID(p))
			// Before the panel has run, the selection is empty. URLs returns a borrowed reference, so no Release —
			// same shape as the root dialog's Paths.
			if got := p.URLs().ArrayOfURLToStringSlice(); len(got) != 0 {
				t.Errorf("URLs = %q before running the panel, want empty", got)
			}
		})
	})
}

func TestOpenPanelRunModalCancel(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewOpenPanel()
			if p == 0 {
				t.Error("NewOpenPanel returned 0")
				return
			}
			defer Release(objc.ID(p))
			cleanup := cancelModalAfter(objc.ID(p), 0.3)
			start := time.Now()
			ok := p.RunModal()
			elapsed := time.Since(start)
			cleanup()
			if ok {
				t.Error("RunModal reported OK for a canceled panel")
			}
			if elapsed > 5*time.Second {
				t.Errorf("runModal took %v; it likely returned via the abortModal backstop, not cancel:", elapsed)
			}
		})
	})
}
