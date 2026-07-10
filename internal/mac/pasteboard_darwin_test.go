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
	"bytes"
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/uti"
)

// testDataType is an unregistered custom type: the pasteboard APIs only read the UTI string, so registration with
// the uti package is unnecessary.
var testDataType = &uti.DataType{UTI: "com.trollworks.unison.test-data"}

// newUniquePasteboard returns a uniquely named pasteboard cast to the Pasteboard handle type, so the Pasteboard
// methods can be exercised without touching the user's clipboard. It must be called on the main thread (from within
// a runOnMain closure), and the returned cleanup — which releases the pasteboard's server-side resources — must run
// there too.
func newUniquePasteboard(t *testing.T) (Pasteboard, func()) {
	t.Helper()
	var pb objc.ID
	WithPool(func() {
		pb = objc.ID(Cls("NSPasteboard")).Send(Sel("pasteboardWithUniqueName"))
	})
	if pb == 0 {
		t.Fatal("pasteboardWithUniqueName returned nil")
	}
	return Pasteboard(pb), func() { pb.Send(Sel("releaseGlobally")) }
}

func TestPasteboardGeneral(t *testing.T) {
	runOnMain(func() {
		first := PasteboardGeneral()
		second := PasteboardGeneral()
		if first == 0 {
			t.Error("PasteboardGeneral returned 0")
		}
		if first != second {
			t.Errorf("PasteboardGeneral not a singleton: %#x vs %#x", first, second)
		}
		if direct := objc.ID(Cls("NSPasteboard")).Send(Sel("generalPasteboard")); objc.ID(first) != direct {
			t.Errorf("PasteboardGeneral = %#x, want AppKit's generalPasteboard %#x", first, direct)
		}
	})
}

func TestPasteboardWriteAndReadBack(t *testing.T) {
	runOnMain(func() {
		pb, cleanup := newUniquePasteboard(t)
		defer cleanup()
		const text = "pasteboard test 日本語"
		payload := []byte{0x00, 0x01, 0xfe, 0xff, 'u', 'n', 'i', 's', 'o', 'n'}
		pb.Clear()
		item := NewPasteboardItem()
		item.SetData(testDataType, payload)
		item.SetString(text)
		pb.WriteItems(item)

		types := pb.AvailableDataTypes()
		if !slices.Contains(types, testDataType.UTI) {
			t.Errorf("AvailableDataTypes = %v, missing %q", types, testDataType.UTI)
		}
		if !slices.Contains(types, uti.UTF8PlainText.UTI) {
			t.Errorf("AvailableDataTypes = %v, missing %q", types, uti.UTF8PlainText.UTI)
		}

		if !pb.HasDataType(testDataType) {
			t.Errorf("HasDataType(%q) = false, want true", testDataType.UTI)
		}
		if !pb.HasDataType(uti.UTF8PlainText) {
			t.Errorf("HasDataType(%q) = false, want true", uti.UTF8PlainText.UTI)
		}
		if pb.HasDataType(uti.PDF) {
			t.Errorf("HasDataType(%q) = true, want false", uti.PDF.UTI)
		}

		if got := pb.Bytes(testDataType); !bytes.Equal(got, payload) {
			t.Errorf("Bytes(%q) = %v, want %v", testDataType.UTI, got, payload)
		}
		if got := pb.Bytes(uti.UTF8PlainText); string(got) != text {
			t.Errorf("Bytes(%q) = %q, want %q", uti.UTF8PlainText.UTI, got, text)
		}
		if got := pb.Bytes(uti.PDF); got != nil {
			t.Errorf("Bytes(%q) = %v, want nil", uti.PDF.UTI, got)
		}

		// SetString must have written through AppKit's own string channel, not just raw bytes.
		var readBack string
		WithPool(func() {
			readBack = GoStringFromNSString(objc.ID(pb).Send(Sel("stringForType:"), pasteboardTypeString()))
		})
		if readBack != text {
			t.Errorf("stringForType readback = %q, want %q", readBack, text)
		}
	})
}

func TestPasteboardMultipleItemsAndClear(t *testing.T) {
	runOnMain(func() {
		pb, cleanup := newUniquePasteboard(t)
		defer cleanup()
		otherType := &uti.DataType{UTI: "com.trollworks.unison.test-data-2"}
		pb.Clear()
		first := NewPasteboardItem()
		first.SetData(testDataType, []byte{1, 2, 3})
		second := NewPasteboardItem()
		second.SetData(otherType, []byte{4, 5, 6})
		pb.WriteItems(first, second)
		if !pb.HasDataType(testDataType) || !pb.HasDataType(otherType) {
			t.Error("multi-item write did not expose both data types")
		}

		// Zero-length data is representable on the pasteboard, but Bytes reports it as nil, matching the old bridge.
		empty := &uti.DataType{UTI: "com.trollworks.unison.test-empty"}
		emptyItem := NewPasteboardItem()
		emptyItem.SetData(empty, nil)
		pb.WriteItems(emptyItem)
		if !pb.HasDataType(empty) {
			t.Errorf("HasDataType(%q) = false after writing empty data, want true", empty.UTI)
		}
		if got := pb.Bytes(empty); got != nil {
			t.Errorf("Bytes(%q) = %v, want nil for empty data", empty.UTI, got)
		}

		// WriteItems with no items must be a no-op, and Clear must remove everything.
		pb.WriteItems()
		pb.Clear()
		if types := pb.AvailableDataTypes(); len(types) != 0 {
			t.Errorf("AvailableDataTypes after Clear = %v, want none", types)
		}
		if pb.HasDataType(testDataType) {
			t.Errorf("HasDataType(%q) = true after Clear, want false", testDataType.UTI)
		}
	})
}
