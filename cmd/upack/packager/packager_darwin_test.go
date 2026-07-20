// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package packager

import (
	"bytes"
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConformsTo = "public.data"

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireWellFormedXML(t *testing.T, data []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		if _, err := dec.Token(); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			t.Fatalf("not well-formed XML: %v", err)
		}
	}
}

func TestWritePlistEscaping(t *testing.T) {
	cfg := &Config{
		FullName:        "Spoken <Name> & Co",
		ExecutableName:  "testapp",
		AppIcon:         "app.png",
		CopyrightHolder: "Smith & Jones <LLC>",
		CopyrightYears:  "2026",
		FileInfo: []*FileData{
			{
				Name:       "Doc & <Type>",
				Icon:       "doc.png",
				Role:       "Viewer",
				Rank:       rankOwner,
				UTI:        "com.example.doc",
				ConformsTo: []string{testConformsTo},
				Extensions: []string{"doc"},
				MimeTypes:  []string{"application/x-doc"},
			},
			{
				Name:       "Import & Type",
				Icon:       "imp.png",
				Role:       "Editor",
				Rank:       "Alternate",
				UTI:        "com.example.imp",
				ConformsTo: []string{testConformsTo},
				Extensions: []string{"imp"},
				MimeTypes:  []string{"application/x-imp"},
			},
		},
		Mac: MacOnlyOpts{AppID: "com.example.testapp", CategoryUTI: "public.app-category.utilities"},
	}
	cfg.prepare("1.0.0")
	target := filepath.Join(t.TempDir(), "Info.plist")
	if err := writePlist(cfg, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	requireWellFormedXML(t, data)
	s := string(data)
	for _, want := range []string{
		"Smith &amp; Jones &lt;LLC&gt;",
		"Spoken &lt;Name&gt; &amp; Co",
		"Doc &amp; &lt;Type&gt;",
		"Import &amp; Type",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("plist does not contain %q", want)
		}
	}
}

func TestCreateAppPackageOwnerIcons(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestPNG(t, "app.png")
	writeTestPNG(t, "doc.png")
	writeTestPNG(t, "viewer.png")
	if err := os.WriteFile("testapp", []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		FullName:        "Test App",
		ExecutableName:  "testapp",
		AppIcon:         "app.png",
		CopyrightHolder: "Tester",
		CopyrightYears:  "2026",
		FileInfo: []*FileData{
			// Rank Owner with a non-Editor role: the plist references this type's .icns, so it must be generated.
			{
				Name:       "Doc",
				Icon:       "doc.png",
				Role:       "Viewer",
				Rank:       rankOwner,
				UTI:        "com.example.doc",
				ConformsTo: []string{testConformsTo},
				Extensions: []string{"doc"},
				MimeTypes:  []string{"application/x-doc"},
			},
			// Editor role without Owner rank: the plist never references an icon for it, so none should be written.
			{
				Name:       "View",
				Icon:       "viewer.png",
				Role:       "Editor",
				Rank:       "Alternate",
				UTI:        "com.example.view",
				ConformsTo: []string{testConformsTo},
				Extensions: []string{"view"},
				MimeTypes:  []string{"application/x-view"},
			},
		},
		Mac: MacOnlyOpts{AppID: "com.example.testapp"},
	}
	cfg.prepare("1.0.0")
	if err := createAppPackage(cfg); err != nil {
		t.Fatal(err)
	}
	resDir := filepath.Join("testapp.app", "Contents", "Resources")
	for _, want := range []string{"app.icns", "doc.icns"} {
		if _, err := os.Stat(filepath.Join(resDir, want)); err != nil {
			t.Errorf("missing %s: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(resDir, "viewer.icns")); err == nil {
		t.Error("viewer.icns was generated for a non-Owner file type that the plist never references")
	}
	if _, err := os.Stat(filepath.Join("testapp.app", "Contents", "MacOS", "testapp")); err != nil {
		t.Errorf("missing executable: %v", err)
	}
	data, err := os.ReadFile(filepath.Join("testapp.app", "Contents", "Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	requireWellFormedXML(t, data)
	if !strings.Contains(string(data), "doc.icns") {
		t.Error("plist does not reference doc.icns")
	}
}
