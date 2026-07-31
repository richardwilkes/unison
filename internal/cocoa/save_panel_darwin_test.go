// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package cocoa

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

func TestSavePanelNameField(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewSavePanel()
			if p == 0 {
				t.Error("NewSavePanel returned 0")
				return
			}
			defer p.Release()
			for _, want := range []string{"report.txt", "unison-漢字.txt", ""} {
				p.SetInitialFileName(want)
				if got := p.InitialFileName(); got != want {
					t.Errorf("InitialFileName = %q, want %q", got, want)
				}
			}
		})
	})
}

// TestSavePanelRelease covers the Release method the root dialogs depend on to free their panels (before it existed,
// every NewSaveDialog leaked its panel for the life of the process and these tests worked around the gap with raw
// Release(objc.ID(p)) calls). Release must drop exactly one reference: a panel kept alive by an extra retain has to
// remain fully usable afterward.
func TestSavePanelRelease(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewSavePanel()
			if p == 0 {
				t.Error("NewSavePanel returned 0")
				return
			}
			Retain(objc.ID(p)) // hold a second reference so the panel survives the Release under test
			defer Release(objc.ID(p))
			p.Release()
			const want = "release-probe.txt"
			p.SetInitialFileName(want)
			if got := p.InitialFileName(); got != want {
				t.Errorf("after a balanced Release, InitialFileName = %q, want %q", got, want)
			}
		})
	})
}

func TestSavePanelDirectoryURL(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewSavePanel()
			if p == 0 {
				t.Error("NewSavePanel returned 0")
				return
			}
			defer p.Release()
			dirURL := NewFileURL("/Library/")
			p.SetDirectoryURL(dirURL)
			dirURL.Release()
			abs := p.DirectoryURL().AbsoluteString()
			if parsed, err := url.Parse(abs); err != nil || parsed.Path != "/Library/" {
				t.Errorf("directory URL = %q (parse err %v), want path /Library/", abs, err)
			}
		})
	})
}

func TestSavePanelAllowedFileTypes(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewSavePanel()
			if p == 0 {
				t.Error("NewSavePanel returned 0")
				return
			}
			defer p.Release()
			allowed := p.AllowedFileTypes()
			if objc.ID(allowed) != 0 {
				t.Errorf("AllowedFileTypes = %#x before any set, want 0", objc.ID(allowed))
			}
			allowed.Release()
			types := NewArrayFromStringSlice([]string{"pdf"})
			p.SetAllowedFileTypes(types)
			types.Release()
			allowed = p.AllowedFileTypes()
			got := allowed.ArrayOfStringToStringSlice()
			allowed.Release()
			if want := []string{"pdf"}; !slices.Equal(got, want) {
				t.Errorf("AllowedFileTypes = %q, want %q", got, want)
			}
			p.SetAllowedFileTypes(0)
			if cleared := p.AllowedFileTypes(); objc.ID(cleared) != 0 {
				t.Errorf("AllowedFileTypes = %#x after clearing, want 0", objc.ID(cleared))
				cleared.Release()
			}
		})
	})
}

func TestSavePanelRunModalCancel(t *testing.T) {
	requirePanelService(t)
	runOnMain(func() {
		WithPool(func() {
			p := NewSavePanel()
			if p == 0 {
				t.Error("NewSavePanel returned 0")
				return
			}
			defer p.Release()
			dirURL := NewFileURL("/Library/")
			p.SetDirectoryURL(dirURL)
			dirURL.Release()
			p.SetInitialFileName("unison-mac-test.txt")
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
			// The panel composes its URL from the directory and name field once it has been presented (before the
			// first presentation it still reports its defaults — verified against compiled Objective-C, so it is
			// AppKit behavior, not a port difference). This is the value the root dialog's Path parses.
			abs := p.URL().AbsoluteString()
			if parsed, err := url.Parse(abs); err != nil || parsed.Path != "/Library/unison-mac-test.txt" {
				t.Errorf("URL = %q (parse err %v), want path /Library/unison-mac-test.txt", abs, err)
			}
		})
	})
}
