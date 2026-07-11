// Copyright (c) 2021-2026 by Richard A. Wilkes. All rights reserved.
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, version 2.0. If a copy of the MPL was not distributed with
// this file, You can obtain one at http://mozilla.org/MPL/2.0/.
//
// This Source Code Form is "Incompatible With Secondary Licenses", as
// defined by the Mozilla Public License, version 2.0.

package unison

import (
	"github.com/richardwilkes/toolbox/v2/uti"
	"github.com/richardwilkes/unison/drag"
	"github.com/richardwilkes/unison/internal/cocoa"
)

func apiClipboardAvailableDataTypes() []string {
	return cocoa.PasteboardGeneral().AvailableDataTypes()
}

func apiClipboardHasDataType(dataType *uti.DataType) bool {
	return cocoa.PasteboardGeneral().HasDataType(dataType)
}

func apiClipboardGetData(dataType *uti.DataType) []byte {
	return cocoa.PasteboardGeneral().Bytes(dataType)
}

func apiClipboardSetData(data ...drag.Data) {
	pb := cocoa.PasteboardGeneral()
	pb.Clear()
	all := make([]cocoa.PasteboardItem, 0, len(data))
	for _, one := range data {
		item := cocoa.NewPasteboardItem()
		item.SetData(one.Type, one.Data)
		if uti.UTF8PlainText.ConformsTo(one.Type) {
			item.SetString(string(one.Data))
		}
		all = append(all, item)
	}
	pb.WriteItems(all...)
}
