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
	"debug/pe"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func solidImage(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	return img
}

func TestMakeManifest(t *testing.T) {
	s := string(makeManifest("My <App> & friends"))
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		`My &lt;App&gt; &amp; friends`, // description is HTML-escaped
		`permonitorv2,system`,
		`level="asInvoker"`,
		`<supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("manifest missing %q in:\n%s", want, s)
		}
	}
	if got := strings.Count(s, "<supportedOS"); got != 1 {
		t.Errorf("expected 1 supportedOS entry, got %d", got)
	}
}

func TestVersionInfo(t *testing.T) {
	vi := versionInfo{
		companyName:  "Acme",
		productName:  "Example",
		shortVersion: "4.5.6",
		fullVersion:  "4.5.6.0",
	}
	b := vi.bytes()
	if len(b) == 0 {
		t.Fatal("version bytes are empty")
	}
	// The leading uint16 is the node length, which spans the whole structure.
	if got := int(b[0]) | int(b[1])<<8; got != len(b) {
		t.Errorf("version node length = %d, want %d", got, len(b))
	}

	// setVersionInfo produces exactly one en-US RT_VERSION resource.
	rs := &resourceSet{}
	rs.setVersionInfo(&vi)
	re := rs.types[idNum(rtVersion)].resources[idNum(1)]
	if len(re.data) != 1 {
		t.Fatalf("expected 1 language, got %d", len(re.data))
	}
	if _, ok := re.data[lcidDefault]; !ok {
		t.Error("version resource is not at lcidDefault")
	}
}

func sampleResourceSet(t *testing.T) *resourceSet {
	t.Helper()
	rs := &resourceSet{}
	rs.setManifest("Example app")
	rs.setVersionInfo(&versionInfo{
		productName:  "Example",
		shortVersion: "1.0.0",
		fullVersion:  "1.0.0.0",
	})
	ic := &winIcon{}
	if err := ic.addImage(solidImage(48, 48)); err != nil {
		t.Fatal(err)
	}
	if err := rs.setIcon(idName("APP"), ic); err != nil {
		t.Fatal(err)
	}
	return rs
}

func TestWriteObject(t *testing.T) {
	// A single resource set is reused across architectures, exactly as
	// prepareBinary does, so this also guards that reuse stays deterministic.
	rs := sampleResourceSet(t)
	write := func(arch sysoArch) []byte {
		var buf bytes.Buffer
		if err := rs.writeCOFF(&buf, arch); err != nil {
			t.Fatalf("%s: %v", arch.goarch, err)
		}
		return buf.Bytes()
	}

	var rsrc []byte
	for _, arch := range sysoArches {
		data := write(arch)
		if len(data) == 0 {
			t.Fatalf("%s: empty object", arch.goarch)
		}
		if !bytes.Equal(data, write(arch)) {
			t.Errorf("%s: output is not deterministic", arch.goarch)
		}

		// The object must be a valid COFF object that debug/pe can parse, with
		// the expected machine type and .rsrc section.
		f, err := pe.NewFile(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: debug/pe could not parse the object: %v", arch.goarch, err)
		}
		if f.Machine != arch.machine {
			t.Errorf("%s: machine = 0x%x, want 0x%x", arch.goarch, f.Machine, arch.machine)
		}
		sec := f.Section(".rsrc")
		if sec == nil {
			t.Fatalf("%s: missing .rsrc section", arch.goarch)
		}

		// The resource data is architecture-independent; only the COFF wrapper
		// (machine type and relocation type) differs between targets.
		secData, err := sec.Data()
		if err != nil {
			t.Fatal(err)
		}
		if rsrc == nil {
			rsrc = secData
		} else if !bytes.Equal(rsrc, secData) {
			t.Errorf("%s: .rsrc content differs across architectures", arch.goarch)
		}
	}
}

func TestAddWindowsIconsRejectsOwnerWithoutExtensions(t *testing.T) {
	iconPath := filepath.Join(t.TempDir(), "app.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, solidImage(32, 32)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(iconPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	rs := &resourceSet{}
	err := rs.addWindowsIcons(iconPath, []*FileData{{Name: "Doc", Icon: iconPath, Rank: rankOwner}})
	if err == nil {
		t.Fatal("expected an error for an Owner file_info entry with no extensions, not a panic or success")
	}
	if !strings.Contains(err.Error(), "no extensions") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetIconRejectsBadName(t *testing.T) {
	ic := &winIcon{}
	if err := ic.addImage(solidImage(16, 16)); err != nil {
		t.Fatal(err)
	}
	rs := &resourceSet{}
	if err := rs.setIcon(idName(""), ic); err == nil {
		t.Error("expected error for empty icon name")
	}
}

func TestAddImageTooBig(t *testing.T) {
	ic := &winIcon{}
	if err := ic.addImage(solidImage(257, 257)); err == nil {
		t.Error("expected error for oversized image")
	}
}
