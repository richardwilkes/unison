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
	"sync"
	"unsafe"

	"github.com/ebitengine/purego/objc"
	"github.com/richardwilkes/toolbox/v2/uti"
)

type (
	// Pasteboard is a handle to an NSPasteboard. The only instance unison uses is the general pasteboard, a
	// process-lifetime singleton, so Pasteboard values are never owned or released.
	Pasteboard objc.ID
	// PasteboardItem is a handle to an NSPasteboardItem. NewPasteboardItem returns an owned (+1) reference whose
	// ownership transfers to whatever the item is handed to: Pasteboard.WriteItems releases the written items, and a
	// drag's NSDraggingItem takes over the reference (see View.BeginDraggingSession). An NSPasteboardItem can only
	// ever be attached to a single pasteboard, so an item handle must not be reused after being handed off.
	PasteboardItem objc.ID
)

// pasteboardTypeString returns AppKit's NSPasteboardTypeString constant, resolved once.
var pasteboardTypeString = sync.OnceValue(func() objc.ID {
	return NSStringConstant("AppKit", "NSPasteboardTypeString")
})

// pasteboardContainsType returns true if the pasteboard's current types include dataType (an NSString). Callers must
// provide the autorelease pool.
func pasteboardContainsType(pb, dataType objc.ID) bool {
	return objc.Send[bool](pb.Send(Sel("types")), Sel("containsObject:"), dataType)
}

// PasteboardGeneral returns the general pasteboard.
func PasteboardGeneral() Pasteboard {
	return Pasteboard(objc.ID(Cls("NSPasteboard")).Send(Sel("generalPasteboard")))
}

// AvailableDataTypes returns the data types currently available on the pasteboard.
func (p Pasteboard) AvailableDataTypes() (types []string) {
	WithPool(func() {
		ids := IDsFromNSArray(objc.ID(p).Send(Sel("types")))
		types = make([]string, 0, len(ids))
		for _, s := range ids {
			types = append(types, GoStringFromNSString(s))
		}
	})
	return types
}

// HasDataType returns true if the pasteboard currently holds data of the given type.
func (p Pasteboard) HasDataType(dataType *uti.DataType) (has bool) {
	WithPool(func() {
		has = pasteboardContainsType(objc.ID(p), NSStringFromGo(dataType.UTI))
	})
	return has
}

// Bytes returns the data of the given type currently on the pasteboard, if any.
func (p Pasteboard) Bytes(dataType *uti.DataType) (data []byte) {
	WithPool(func() {
		dt := NSStringFromGo(dataType.UTI)
		if pasteboardContainsType(objc.ID(p), dt) {
			data = GoBytesFromNSData(objc.ID(p).Send(Sel("dataForType:"), dt))
		}
	})
	return data
}

// Clear clears the pasteboard's contents.
func (p Pasteboard) Clear() {
	objc.ID(p).Send(Sel("clearContents"))
}

// WriteItems writes the given items to the pasteboard, transferring ownership of the caller's references: the
// pasteboard retains the items and the owned references returned by NewPasteboardItem are released here. The item
// handles must not be used afterward.
func (p Pasteboard) WriteItems(items ...PasteboardItem) {
	if len(items) == 0 {
		return
	}
	WithPool(func() {
		ids := make([]objc.ID, len(items))
		for i, item := range items {
			ids[i] = objc.ID(item)
		}
		objc.ID(p).Send(Sel("writeObjects:"), NSArrayFromIDs(ids...))
		for _, id := range ids {
			Release(id)
		}
	})
}

// NewPasteboardItem returns a new owned (+1) pasteboard item.
func NewPasteboardItem() PasteboardItem {
	return PasteboardItem(objc.ID(Cls("NSPasteboardItem")).Send(Sel("alloc")).Send(Sel("init")))
}

// SetString sets the item's string content (of type NSPasteboardTypeString).
func (i PasteboardItem) SetString(s string) {
	WithPool(func() {
		objc.ID(i).Send(Sel("setString:forType:"), NSStringFromGo(s), pasteboardTypeString())
	})
}

// SetData sets the item's data for the given type.
func (i PasteboardItem) SetData(dataType *uti.DataType, data []byte) {
	WithPool(func() {
		var ptr unsafe.Pointer
		if len(data) != 0 {
			ptr = unsafe.Pointer(&data[0])
		}
		// dataWithBytes:length: copies the buffer during the call, so passing Go memory is safe here.
		nsData := objc.ID(Cls("NSData")).Send(Sel("dataWithBytes:length:"), ptr, uint64(len(data)))
		objc.ID(i).Send(Sel("setData:forType:"), nsData, NSStringFromGo(dataType.UTI))
	})
}
