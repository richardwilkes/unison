// Copyright ©2021 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package ns

import "github.com/progrium/macdriver/objc"

var pasteboardItemClass = objc.Get("NSPasteboardItem")

// PasteboardItem https://developer.apple.com/documentation/appkit/nspasteboarditem?language=objc
type PasteboardItem struct {
	objc.Object
}

// NewPasteboardItem https://developer.apple.com/documentation/appkit/nspasteboarditem?language=objc
func NewPasteboardItem() PasteboardItem {
	return PasteboardItem{Object: pasteboardItemClass.Alloc().Init()}
}

// DataForType https://developer.apple.com/documentation/appkit/nspasteboarditem/1508496-datafortype?language=objc
func (p PasteboardItem) DataForType(pbType string) Data {
	str := StringFromString(pbType)
	defer str.Release()
	return Data{Object: p.Send("dataForType:", str)}
}

// Types https://developer.apple.com/documentation/appkit/nspasteboarditem/1508499-types?language=objc
func (p PasteboardItem) Types() Array {
	return Array{Object: p.Send("types")}
}

// SetDataForType https://developer.apple.com/documentation/appkit/nspasteboarditem/1508501-setdata?language=objc
func (p PasteboardItem) SetDataForType(data Data, pbType string) {
	str := StringFromString(pbType)
	defer str.Release()
	p.Send("setData:forType:", data, str)
}
