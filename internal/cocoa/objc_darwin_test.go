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
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

//nolint:goconst // Explicit strings are more readable

func TestNSStringRoundTrip(t *testing.T) {
	WithPool(func() {
		for _, s := range []string{"", "hello", "héllo wörld", "漢字テスト", "emoji 🎉👍", "line\nbreak"} { //nolint:goconst // Explicit strings are more readable
			if got := GoStringFromNSString(NSStringFromGo(s)); got != s {
				t.Errorf("round trip of %q produced %q", s, got)
			}
		}
		if got := GoStringFromNSString(0); got != "" {
			t.Errorf("nil NSString produced %q", got)
		}
	})
}

func TestNewNSString(t *testing.T) {
	for _, s := range []string{"", "hello", "héllo wörld", "漢字テスト", "emoji 🎉👍", "line\nbreak"} {
		// Deliberately created outside any autorelease pool — that is NewNSString's reason for existing.
		str := NewNSString(s)
		WithPool(func() {
			if got := GoStringFromNSString(str); got != s {
				t.Errorf("round trip of %q produced %q", s, got)
			}
		})
		Release(str)
	}
}

// TestNSStringInvalidUTF8 proves the NSString constructors never return nil for invalid UTF-8. NSString's own
// UTF-8 decoders reject bad sequences with a nil result, which would flow into AppKit calls (e.g. setString:forType:
// during a clipboard write) and raise NSInvalidArgumentException; instead, the helpers substitute the Unicode
// replacement character.
func TestNSStringInvalidUTF8(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
	}{
		{in: "bad\x80seq", want: "bad�seq"},
		{in: "\xff", want: "�"},
		{in: "truncated\xe2\x82", want: "truncated�"},
		{in: "fine", want: "fine"},
	} {
		str := NewNSString(c.in)
		if str == 0 {
			t.Errorf("NewNSString(%q) returned nil", c.in)
			continue
		}
		WithPool(func() {
			if got := GoStringFromNSString(str); got != c.want {
				t.Errorf("NewNSString(%q) round-tripped to %q, want %q", c.in, got, c.want)
			}
		})
		Release(str)
		WithPool(func() {
			auto := NSStringFromGo(c.in)
			if auto == 0 {
				t.Errorf("NSStringFromGo(%q) returned nil", c.in)
				return
			}
			if got := GoStringFromNSString(auto); got != c.want {
				t.Errorf("NSStringFromGo(%q) round-tripped to %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestNSArrayHelpers(t *testing.T) {
	WithPool(func() {
		if count := NSArrayCount(NSArrayFromIDs()); count != 0 {
			t.Errorf("empty array has count %d", count)
		}
		if ids := IDsFromNSArray(NSArrayFromIDs()); ids != nil {
			t.Errorf("empty array produced slice %v", ids)
		}
		want := []string{"one", "two", "three"}
		in := make([]objc.ID, len(want))
		for i, s := range want {
			in[i] = NSStringFromGo(s)
		}
		array := NSArrayFromIDs(in...)
		if count := NSArrayCount(array); count != uint64(len(want)) {
			t.Fatalf("count = %d, want %d", count, len(want))
		}
		for i, s := range want {
			if got := GoStringFromNSString(NSArrayObjectAt(array, uint64(i))); got != s {
				t.Errorf("element %d = %q, want %q", i, got, s)
			}
		}
		for i, id := range IDsFromNSArray(array) {
			if got := GoStringFromNSString(id); got != want[i] {
				t.Errorf("slice element %d = %q, want %q", i, got, want[i])
			}
		}
	})
}

func TestNSNumberRoundTrip(t *testing.T) {
	WithPool(func() {
		for _, v := range []int64{0, 1, -1, 42, -9223372036854775808, 9223372036854775807} {
			if got := Int64FromNSNumber(NSNumberFromInt64(v)); got != v {
				t.Errorf("int64 round trip of %d produced %d", v, got)
			}
		}
		for _, v := range []float64{0, 1.5, -2.25, 1e300} {
			if got := Float64FromNSNumber(NSNumberFromFloat64(v)); got != v {
				t.Errorf("float64 round trip of %v produced %v", v, got)
			}
		}
		if Int64FromNSNumber(0) != 0 || Float64FromNSNumber(0) != 0 {
			t.Error("nil NSNumber did not produce zero")
		}
	})
}

// TestNSValueRoundTrip proves the NSValue helpers the accessibility adapter hands rects, ranges and points to AppKit
// with, including the nil-tolerance of the readers.
func TestNSValueRoundTrip(t *testing.T) {
	WithPool(func() {
		for _, r := range []NSRect{
			{},
			{Origin: NSPoint{X: 1.5, Y: -2.25}, Size: NSSize{Width: 320, Height: 240}},
			{Origin: NSPoint{X: -1e6, Y: 1e6}, Size: NSSize{Width: 0.5, Height: 0.25}},
		} {
			if got := RectFromNSValue(NSValueFromRect(r)); got != r {
				t.Errorf("rect round trip of %+v produced %+v", r, got)
			}
		}
		for _, r := range []NSRange{{}, {Location: 6, Length: 5}, {Location: nsNotFound, Length: 0}} {
			if got := RangeFromNSValue(NSValueFromRange(r)); got != r {
				t.Errorf("range round trip of %+v produced %+v", r, got)
			}
		}
		pt := NSPoint{X: 3.5, Y: -4.5}
		if got := objc.Send[NSPoint](NSValueFromPoint(pt), Sel("pointValue")); got != pt {
			t.Errorf("point round trip of %+v produced %+v", pt, got)
		}
		if got := RectFromNSValue(0); got != (NSRect{}) {
			t.Errorf("RectFromNSValue(0) = %+v, want the zero rect", got)
		}
		if got := RangeFromNSValue(0); got != (NSRange{}) {
			t.Errorf("RangeFromNSValue(0) = %+v, want the zero range", got)
		}
	})
}

// TestNSDictionaryFromPairs proves the userInfo builder: alternating keys and values, an empty dictionary rather than
// nil for no pairs, and a dangling key dropped rather than paired with nil.
func TestNSDictionaryFromPairs(t *testing.T) {
	WithPool(func() {
		empty := NSDictionaryFromPairs()
		if empty == 0 {
			t.Fatal("NSDictionaryFromPairs() returned nil")
		}
		if got := objc.Send[uint64](empty, Sel("count")); got != 0 {
			t.Errorf("empty dictionary count = %d, want 0", got)
		}
		dict := NSDictionaryFromPairs(
			NSStringFromGo("one"), NSStringFromGo("first"),
			NSStringFromGo("two"), NSNumberFromInt64(2),
			NSStringFromGo("dangling"),
		)
		if got := objc.Send[uint64](dict, Sel("count")); got != 2 {
			t.Errorf("dictionary count = %d, want 2 (the dangling key must be dropped)", got)
		}
		if got := GoStringFromNSString(dict.Send(Sel("objectForKey:"), NSStringFromGo("one"))); got != "first" {
			t.Errorf("dictionary[one] = %q, want %q", got, "first")
		}
		if got := Int64FromNSNumber(dict.Send(Sel("objectForKey:"), NSStringFromGo("two"))); got != 2 {
			t.Errorf("dictionary[two] = %d, want 2", got)
		}
	})
}

// TestAppKitString proves the cached constant lookup returns AppKit's own object and keeps returning the same one.
func TestAppKitString(t *testing.T) {
	first := AppKitString("NSAccessibilityGroupRole")
	if first == 0 {
		t.Fatal("AppKitString returned nil for NSAccessibilityGroupRole")
	}
	if second := AppKitString("NSAccessibilityGroupRole"); second != first {
		t.Errorf("AppKitString returned %#x then %#x for the same symbol", first, second)
	}
	WithPool(func() {
		if got := GoStringFromNSString(first); got != "AXGroup" {
			t.Errorf("NSAccessibilityGroupRole = %q, want AXGroup", got)
		}
	})
}

func TestNSURLFilePathRoundTrip(t *testing.T) {
	WithPool(func() {
		// Note: only normalization-stable characters here (ASCII, CJK). macOS decomposes path strings to NFD, so
		// e.g. a precomposed "ï" would round-trip to "i"+U+0308 — same path, different Go string.
		for _, p := range []string{"/tmp/file.txt", "/tmp/dir with spaces/file two.txt", "/tmp/漢字/テスト.txt", "/"} {
			if got := FilePathFromNSURL(NSURLFromFilePath(p)); got != p {
				t.Errorf("round trip of %q produced %q", p, got)
			}
		}
		if got := FilePathFromNSURL(0); got != "" {
			t.Errorf("nil NSURL produced %q", got)
		}
	})
}

func TestRetainReleaseAndPools(t *testing.T) {
	// Nested pools plus an explicit retain/autorelease cycle; failure mode is a crash, not a bad value.
	WithPool(func() {
		str := objc.ID(Cls("NSString")).Send(Sel("alloc")).Send(Sel("initWithUTF8String:"), "retained") // +1
		Retain(str)                                                                                     // +2
		WithPool(func() {
			Autorelease(str) // back to +1 when the inner pool pops
		})
		if got := GoStringFromNSString(str); got != "retained" {
			t.Errorf("string after inner pool = %q", got)
		}
		Release(str) // balances the init; str is dead after this
		Retain(0)    // nil sends must be no-ops
		Release(0)
		Autorelease(0)
	})
}

// TestStructMsgSend permanently guards the struct-passing message-send paths the cgo-free bridge depends on
// (NSRect is 32 bytes: HFA in registers on arm64, hidden-pointer return + stret dispatch on amd64).
func TestStructMsgSend(t *testing.T) {
	WithPool(func() {
		rectIn := NSRect{Origin: NSPoint{X: 12.5, Y: -3.25}, Size: NSSize{Width: 640, Height: 480}}
		if rectOut := objc.Send[NSRect](objc.ID(Cls("NSValue")).Send(Sel("valueWithRect:"), rectIn),
			Sel("rectValue")); rectOut != rectIn {
			t.Errorf("NSRect round trip produced %+v, want %+v", rectOut, rectIn)
		}
		ptIn := NSPoint{X: 1.5, Y: 2.5}
		if ptOut := objc.Send[NSPoint](objc.ID(Cls("NSValue")).Send(Sel("valueWithPoint:"), ptIn),
			Sel("pointValue")); ptOut != ptIn {
			t.Errorf("NSPoint round trip produced %+v, want %+v", ptOut, ptIn)
		}
		r := objc.Send[NSRange](NSStringFromGo("hello world"), Sel("rangeOfString:"), NSStringFromGo("world"))
		if r.Location != 6 || r.Length != 5 {
			t.Errorf("rangeOfString: produced %+v, want {6 5}", r)
		}
	})
}

func TestGoStringFromCString(t *testing.T) {
	if got := GoStringFromCString(nil); got != "" {
		t.Errorf("GoStringFromCString(nil) = %q, want \"\"", got)
	}
	buf := []byte("hello\x00trailing garbage")
	if got := GoStringFromCString(&buf[0]); got != "hello" {
		t.Errorf("GoStringFromCString = %q, want %q", got, "hello")
	}
	empty := []byte{0}
	if got := GoStringFromCString(&empty[0]); got != "" {
		t.Errorf("GoStringFromCString(empty) = %q, want \"\"", got)
	}
	WithPool(func() {
		// A real Objective-C-sourced C string, the way DragInfo.FilePaths uses it.
		p := objc.Send[*byte](NSURLFromFilePath("/tmp/some file.txt"), Sel("fileSystemRepresentation"))
		if got := GoStringFromCString(p); got != "/tmp/some file.txt" {
			t.Errorf("fileSystemRepresentation via GoStringFromCString = %q", got)
		}
	})
}

func TestGoBytesFromNSData(t *testing.T) {
	if got := GoBytesFromNSData(0); got != nil {
		t.Errorf("GoBytesFromNSData(0) = %v, want nil", got)
	}
	WithPool(func() {
		payload := []byte{0, 1, 2, 253, 254, 255}
		data := objc.ID(Cls("NSData")).Send(Sel("dataWithBytes:length:"),
			unsafe.Pointer(&payload[0]), uint64(len(payload)))
		if got := GoBytesFromNSData(data); !bytes.Equal(got, payload) {
			t.Errorf("GoBytesFromNSData = %v, want %v", got, payload)
		}
		if got := GoBytesFromNSData(objc.ID(Cls("NSData")).Send(Sel("data"))); got != nil {
			t.Errorf("GoBytesFromNSData(empty) = %v, want nil", got)
		}
	})
}
