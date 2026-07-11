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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestArrayFromStringSlice(t *testing.T) {
	runOnMain(func() {
		WithPool(func() {
			want := []string{"png", "jpg", "漢字"}
			a := NewArrayFromStringSlice(want)
			defer a.Release()
			if got := a.Count(); got != len(want) {
				t.Errorf("Count = %d, want %d", got, len(want))
			}
			if got := a.StringAtIndex(2).String(); got != "漢字" {
				t.Errorf("StringAtIndex(2) = %q, want %q", got, "漢字")
			}
			if got := a.ArrayOfStringToStringSlice(); !slices.Equal(got, want) {
				t.Errorf("ArrayOfStringToStringSlice = %q, want %q", got, want)
			}
			empty := NewArrayFromStringSlice(nil)
			defer empty.Release()
			if got := empty.Count(); got != 0 {
				t.Errorf("Count of empty array = %d, want 0", got)
			}
			// Nil handles are safe no-ops yielding zero values (the CF-based cgo bridge crashed on these).
			if got := Array(0).Count(); got != 0 {
				t.Errorf("Count of nil array = %d, want 0", got)
			}
			if got := Array(0).ArrayOfStringToStringSlice(); len(got) != 0 {
				t.Errorf("ArrayOfStringToStringSlice of nil array = %q, want empty", got)
			}
			Array(0).Release()
		})
	})
}

func TestArrayOfURLToStringSlice(t *testing.T) {
	runOnMain(func() {
		WithPool(func() {
			a := objc.ID(Cls("NSMutableArray")).Send(Sel("alloc")).Send(Sel("init"))
			defer Release(a)
			u1 := NewFileURL("/tmp/foo bar.txt")
			a.Send(Sel("addObject:"), objc.ID(u1))
			u1.Release()
			u2 := NewFileURL("/tmp/some dir/")
			a.Send(Sel("addObject:"), objc.ID(u2))
			u2.Release()
			// A non-file URL exercises the documented quirk carried over from the cgo bridge: scheme and host are
			// discarded and only the (percent-decoded) path survives.
			web := objc.ID(Cls("NSURL")).Send(Sel("URLWithString:"), NSStringFromGo("https://example.com/a/b.txt"))
			a.Send(Sel("addObject:"), web)
			got := Array(a).ArrayOfURLToStringSlice()
			want := []string{"/tmp/foo bar.txt", "/tmp/some dir/", "/a/b.txt"}
			if !slices.Equal(got, want) {
				t.Errorf("ArrayOfURLToStringSlice = %q, want %q", got, want)
			}
		})
	})
}

func TestStringWrapperRoundTrip(t *testing.T) {
	runOnMain(func() {
		for _, want := range []string{"", "hello", "漢字テスト"} { //nolint:goconst // Explicit strings are more readable
			s := NewString(want)
			got := s.String()
			s.Release()
			if got != want {
				t.Errorf("round trip of %q produced %q", want, got)
			}
		}
		if got := String(0).String(); got != "" {
			t.Errorf("String of nil handle = %q, want empty", got)
		}
		String(0).Release()
	})
}

func TestNewFileURL(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "foo bar漢字.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	runOnMain(func() {
		WithPool(func() {
			// An existing plain file must not be treated as a directory.
			u := NewFileURL(filePath)
			abs := u.AbsoluteString()
			u.Release()
			if !strings.HasPrefix(abs, "file://") {
				t.Errorf("AbsoluteString = %q, want file:// prefix", abs)
			}
			if strings.HasSuffix(abs, "/") {
				t.Errorf("AbsoluteString = %q for a file, want no trailing slash", abs)
			}
			parsed, err := url.Parse(abs)
			switch {
			case err != nil:
				t.Errorf("unable to parse %q: %v", abs, err)
			case parsed.Path != filePath:
				t.Errorf("parsed path = %q, want %q", parsed.Path, filePath)
			}
			// An existing directory named without a trailing separator is detected via the file system.
			u = NewFileURL(dir)
			abs = u.AbsoluteString()
			u.Release()
			if !strings.HasSuffix(abs, "/") {
				t.Errorf("AbsoluteString = %q for a directory, want trailing slash", abs)
			}
			// A nonexistent path with a trailing separator is treated as a directory...
			u = NewFileURL("/no/such/dir/")
			abs = u.AbsoluteString()
			u.Release()
			if abs != "file:///no/such/dir/" {
				t.Errorf("AbsoluteString = %q, want %q", abs, "file:///no/such/dir/")
			}
			// ...and a nonexistent plain path as a file.
			u = NewFileURL("/no/such/file.txt")
			abs = u.AbsoluteString()
			u.Release()
			if abs != "file:///no/such/file.txt" {
				t.Errorf("AbsoluteString = %q, want %q", abs, "file:///no/such/file.txt")
			}
			if got := URL(0).AbsoluteString(); got != "" {
				t.Errorf("AbsoluteString of nil handle = %q, want empty", got)
			}
			URL(0).Release()
		})
	})
}
