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
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
)

func TestDragOpConversions(t *testing.T) {
	cases := []struct {
		unison drag.Op
		native DragOp
	}{
		{unison: 0, native: DragOpNone},
		{unison: drag.Copy, native: DragOpCopy},
		{unison: drag.Move, native: DragOpMove},
		{unison: drag.Copy | drag.Move, native: DragOpCopy | DragOpMove},
	}
	for _, c := range cases {
		if got := DragOpFromUnison(c.unison); got != c.native {
			t.Errorf("DragOpFromUnison(%d) = %d, want %d", c.unison, got, c.native)
		}
		if got := c.native.ToUnisonDragOp(); got != c.unison {
			t.Errorf("ToUnisonDragOp(%d) = %d, want %d", c.native, got, c.unison)
		}
	}
	// Bits AppKit can set that unison does not model (e.g. NSDragOperationLink = 2) must be dropped.
	if got := DragOp(2).ToUnisonDragOp(); got != 0 {
		t.Errorf("ToUnisonDragOp(link) = %d, want 0", got)
	}
}

// newDragInfoWithPasteboard returns a DragInfo whose fake sender answers draggingPasteboard with a fresh uniquely
// named pasteboard, plus the pasteboard for populating it. Like newUniquePasteboard, it must be called on the main
// thread (from within a runOnMain closure), and the returned cleanup must run there too.
func newDragInfoWithPasteboard(t *testing.T) (DragInfo, Pasteboard, func()) {
	t.Helper()
	pb, pbCleanup := newUniquePasteboard(t)
	info := testDragInfo(t)
	testDragPasteboard = objc.ID(pb)
	return DragInfo(info), pb, func() {
		testDragPasteboard = 0
		Release(info)
		pbCleanup()
	}
}

func TestDragInfoSourceDragOpMask(t *testing.T) {
	runOnMain(func() {
		d, _, cleanup := newDragInfoWithPasteboard(t)
		defer cleanup()
		for _, c := range []struct {
			mask uint64
			want drag.Op
		}{
			{mask: uint64(DragOpCopy | DragOpMove), want: drag.Copy | drag.Move},
			{mask: uint64(DragOpCopy), want: drag.Copy},
			{mask: 0, want: 0},
		} {
			testDragSourceMask = c.mask
			if got := d.SourceDragOpMask(); got != c.want {
				t.Errorf("SourceDragOpMask with native mask %d = %d, want %d", c.mask, got, c.want)
			}
		}
	})
}

func TestDragInfoStringAndData(t *testing.T) {
	runOnMain(func() {
		d, pb, cleanup := newDragInfoWithPasteboard(t)
		defer cleanup()
		const text = "dragged text 中文"
		payload := []byte{9, 8, 7, 0, 1}
		pb.Clear()
		item := NewPasteboardItem()
		item.SetString(text)
		item.SetData(testDataType, payload)
		pb.WriteItems(item)

		types := d.DataTypes()
		if !slices.Contains(types, uti.UTF8PlainText.UTI) || !slices.Contains(types, testDataType.UTI) {
			t.Errorf("DataTypes = %v, want both %q and %q", types, uti.UTF8PlainText.UTI, testDataType.UTI)
		}
		if !d.HasString() {
			t.Error("HasString = false, want true")
		}
		if got := d.Text(); got != text {
			t.Errorf("Text = %q, want %q", got, text)
		}
		if !d.HasDataType(testDataType.UTI) {
			t.Errorf("HasDataType(%q) = false, want true", testDataType.UTI)
		}
		if d.HasDataType(uti.PDF.UTI) {
			t.Errorf("HasDataType(%q) = true, want false", uti.PDF.UTI)
		}
		if got := d.Data(testDataType.UTI); !bytes.Equal(got, payload) {
			t.Errorf("Data(%q) = %v, want %v", testDataType.UTI, got, payload)
		}
		if got := d.Data(uti.PDF.UTI); got != nil {
			t.Errorf("Data(%q) = %v, want nil", uti.PDF.UTI, got)
		}

		// No URLs of any kind are present.
		if d.HasFilePaths() {
			t.Error("HasFilePaths = true, want false")
		}
		if got := d.FilePaths(); len(got) != 0 {
			t.Errorf("FilePaths = %v, want none", got)
		}
		if d.HasURLs() {
			t.Error("HasURLs = true, want false")
		}
		if got := d.URLs(); len(got) != 0 {
			t.Errorf("URLs = %v, want none", got)
		}
	})
}

func TestDragInfoFilePathsAndURLs(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "dragged.txt")
	if err := os.WriteFile(filePath, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	runOnMain(func() {
		d, pb, cleanup := newDragInfoWithPasteboard(t)
		defer cleanup()

		// Record what AppKit itself reports for the URL being written, so the assertions do not depend on how macOS
		// canonicalizes paths (e.g. /var vs /private/var, NFD normalization).
		var wantFSRep string
		WithPool(func() {
			pb.Clear()
			fileURL := NSURLFromFilePath(filePath)
			wantFSRep = GoStringFromCString(objc.Send[*byte](fileURL, Sel("fileSystemRepresentation")))
			objc.ID(pb).Send(Sel("writeObjects:"), NSArrayFromIDs(fileURL))
		})

		if !d.HasFilePaths() {
			t.Error("HasFilePaths = false, want true")
		}
		if got := d.FilePaths(); len(got) != 1 || got[0] != wantFSRep {
			t.Errorf("FilePaths = %v, want [%q]", got, wantFSRep)
		}
		// A file URL is still a URL.
		if !d.HasURLs() {
			t.Error("HasURLs = false, want true")
		}

		// Non-file URLs: not file paths, but URLs. Matching the old bridge, URLs() strips everything but the path.
		WithPool(func() {
			pb.Clear()
			webURL := objc.ID(Cls("NSURL")).Send(Sel("URLWithString:"),
				NSStringFromGo("https://example.com/some/path"))
			objc.ID(pb).Send(Sel("writeObjects:"), NSArrayFromIDs(webURL))
		})
		if d.HasFilePaths() {
			t.Error("HasFilePaths = true for a non-file URL, want false")
		}
		if got := d.FilePaths(); len(got) != 0 {
			t.Errorf("FilePaths = %v for a non-file URL, want none", got)
		}
		if !d.HasURLs() {
			t.Error("HasURLs = false, want true")
		}
		urls := d.URLs()
		if len(urls) != 1 {
			t.Errorf("URLs returned %d entries, want 1", len(urls))
		} else if urls[0].Path != "/some/path" || urls[0].Scheme != "" || urls[0].Host != "" {
			t.Errorf("URLs[0] = %#v, want the old bridge's path-only form /some/path", urls[0])
		}
	})
}
