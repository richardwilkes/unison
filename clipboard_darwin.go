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
	"github.com/richardwilkes/unison/internal/mac"
)

func apiClipboardAvailableDataTypes() []string {
	return mac.PasteboardGeneral().AvailableDataTypes()
}

func apiClipboardHasDataType(dataType *uti.DataType) bool {
	return mac.PasteboardGeneral().HasDataType(dataType)
}

func apiClipboardGetData(dataType *uti.DataType) []byte {
	return mac.PasteboardGeneral().Bytes(dataType)
}

func apiClipboardSetData(data ...ClipboardData) {
	pb := mac.PasteboardGeneral()
	pb.Clear()
	all := make([]mac.PasteboardItem, 0, len(data))
	for _, one := range data {
		item := mac.NewPasteboardItem()
		item.SetData(one.DataType, one.Data)
		if uti.UTF8PlainText.ConformsTo(one.DataType) {
			item.SetString(string(one.Data))
		}
		all = append(all, item)
	}
	pb.WriteItems(all...)
}
